package main

import (
	"cmp"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	authLevels  = []string{"viewer", "editor", "admin", "owner"}
	authSpaceRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_]{0,31}$`)
	authTables  = "records, raw, provenance, links, secrets"
)

type Actor struct {
	UserID    uuid.UUID `json:"user_id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	TokenID   uuid.UUID `json:"token_id,omitzero"`
	TokenName string    `json:"token_name,omitempty"`
	Label     string    `json:"acting_as"`
	Role      string    `json:"-"`
	UserRole  string    `json:"-"`
	Home      uuid.UUID `json:"-"`
	Session   uuid.UUID `json:"-"`
}

func (ac *Actor) label() string {
	who := cmp.Or(ac.Name, ac.Email)
	if ac.TokenID == uuid.Nil {
		return who
	}
	return cmp.Or(ac.TokenName, "token "+ac.TokenID.String()[:8]) + ", on behalf of " + who
}

type actorKey struct{}

func WithActor(ctx context.Context, ac *Actor) context.Context {
	if ac != nil {
		ac.Label = ac.label()
	}
	return context.WithValue(ctx, actorKey{}, ac)
}

func ActorFrom(ctx context.Context) *Actor {
	ac, _ := ctx.Value(actorKey{}).(*Actor)
	return ac
}

type User struct {
	ID        uuid.UUID       `json:"id"`
	Email     string          `json:"email"`
	Name      string          `json:"name"`
	Role      string          `json:"role"`
	Home      uuid.UUID       `json:"home"`
	Signin    json.RawMessage `json:"-"`
	CreatedAt time.Time       `json:"created_at"`
}

type SpaceInfo struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Level string    `json:"level"`
}

type Member struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Level string `json:"level"`
}

type Token struct {
	ID        uuid.UUID         `json:"id"`
	Name      string            `json:"name"`
	Spaces    map[string]string `json:"spaces"`
	Full      bool              `json:"full,omitempty"`
	ExpiresAt *time.Time        `json:"expires_at,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	Secret    string            `json:"token,omitempty"`
}

func authLevel(s string) int { return slices.Index(authLevels, s) + 1 }

func authIdent(s string) string { return pgx.Identifier{s}.Sanitize() }

func authHash(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func authNewSecret() string {
	b := make([]byte, 32)
	rand.Read(b)
	return "arc_" + base64.RawURLEncoding.EncodeToString(b)
}

func authEmail(s string) (string, error) {
	a, err := mail.ParseAddress(strings.TrimSpace(s))
	if err != nil {
		return "", &ValidationError{Type: "user", Errors: []FieldError{{Path: "email", Message: "not an email address"}}}
	}
	return strings.ToLower(a.Address), nil
}

func (a *Archive) roleID(kind string, id uuid.UUID) string {
	return a.prefix + "_" + kind + "_" + strings.ReplaceAll(id.String(), "-", "")
}

func (a *Archive) exec(ctx context.Context, sqls ...string) error {
	for _, s := range sqls {
		if _, err := a.st.Q(ctx).Exec(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

func (a *Archive) sys(ctx context.Context, fn func(context.Context) error) error {
	err := a.st.System(ctx, fn)
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == "23505" {
		return fmt.Errorf("%s: %w", pe.Detail, ErrConflict)
	}
	return err
}

func (a *Archive) has(ctx context.Context, role, group string) bool {
	var ok bool
	err := a.st.Q(ctx).QueryRow(ctx, `SELECT EXISTS (SELECT FROM pg_roles WHERE rolname = $2) AND pg_has_role($1::name, $2::name, 'USAGE')`, role, group).Scan(&ok)
	return err == nil && ok
}

// adminOnly refuses unless the caller is the instance admin, as ontology_admin() in schema.sql
// decides. It guards what reaches past one space: the server's own disk and programs.
func (a *Archive) adminOnly(ctx context.Context, what string) error {
	var ok bool
	if err := a.as(ctx, func(ctx context.Context) error {
		return a.st.Q(ctx).QueryRow(ctx, `SELECT ontology_admin()`).Scan(&ok)
	}); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("only the instance admin %s: %w", what, ErrForbidden)
	}
	return nil
}

func (a *Archive) level(ctx context.Context, role, base string, direct bool) int {
	if !direct {
		for i := len(authLevels); i > 0; i-- {
			if a.has(ctx, role, base+"_"+authLevels[i-1]) {
				return i
			}
		}
		return 0
	}
	var names []string
	rows, _ := a.st.Q(ctx).Query(ctx, `
		SELECT r.rolname FROM pg_auth_members m JOIN pg_roles r ON r.oid = m.roleid JOIN pg_roles u ON u.oid = m.member
		WHERE u.rolname = $1 AND r.rolname = ANY($2)`, role, []string{base + "_viewer", base + "_editor", base + "_admin", base + "_owner"})
	names, _ = pgx.CollectRows(rows, pgx.RowTo[string])
	n := 0
	for _, s := range names {
		n = max(n, authLevel(strings.TrimPrefix(s, base+"_")))
	}
	return n
}

func (a *Archive) setLevel(ctx context.Context, role, base string, lvl int) error {
	for _, l := range authLevels {
		if err := a.exec(ctx, "REVOKE "+authIdent(base+"_"+l)+" FROM "+authIdent(role)); err != nil {
			return err
		}
	}
	if lvl == 0 {
		return nil
	}
	return a.exec(ctx, "GRANT "+authIdent(base+"_"+authLevels[lvl-1])+" TO "+authIdent(role))
}

func (a *Archive) createSpace(ctx context.Context, name string) (uuid.UUID, error) {
	var id uuid.UUID
	if !authSpaceRe.MatchString(name) {
		return id, &ValidationError{Type: "space", Errors: []FieldError{{Path: "name", Message: "want 1-32 lowercase letters, digits or _"}}}
	}
	base := a.prefix + "_s_" + name
	if err := a.st.Q(ctx).QueryRow(ctx, `INSERT INTO spaces (name, role) VALUES ($1, $2) RETURNING id`, name, base).Scan(&id); err != nil {
		return id, err
	}
	for i, l := range authLevels {
		if err := a.exec(ctx, "CREATE ROLE "+authIdent(base+"_"+l)+" NOLOGIN"); err != nil {
			return id, err
		}
		if i > 0 {
			if err := a.exec(ctx, "GRANT "+authIdent(base+"_"+authLevels[i-1])+" TO "+authIdent(base+"_"+l)); err != nil {
				return id, err
			}
		}
	}
	return id, a.exec(ctx,
		"GRANT SELECT ON "+authTables+" TO "+authIdent(base+"_viewer"),
		"GRANT INSERT, UPDATE, DELETE ON "+authTables+" TO "+authIdent(base+"_editor"))
}

func (a *Archive) createRole(ctx context.Context, role string) error {
	return a.exec(ctx,
		"CREATE ROLE "+authIdent(role)+" NOLOGIN",
		"GRANT "+authIdent(a.prefix+"_user")+" TO "+authIdent(role),
		"GRANT "+authIdent(role)+" TO "+authIdent(a.prefix+"_authenticator"))
}

func (a *Archive) freeSpaceName(ctx context.Context, email string) string {
	base := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return '_'
	}, email)
	base = strings.TrimLeft(base, "_")
	if base == "" {
		base = "user"
	}
	base = base[:min(len(base), 28)]
	name := base
	for i := 2; ; i++ {
		var taken bool
		a.st.Q(ctx).QueryRow(ctx, `SELECT EXISTS (SELECT FROM spaces WHERE name = $1)`, name).Scan(&taken)
		if !taken {
			return name
		}
		name = fmt.Sprintf("%s_%d", base, i)
	}
}

func authUser(ctx context.Context) (*Actor, error) {
	ac := ActorFrom(ctx)
	if ac == nil {
		return nil, fmt.Errorf("not signed in: %w", ErrUnauthorized)
	}
	if ac.TokenID != uuid.Nil {
		return nil, fmt.Errorf("API tokens cannot manage users, spaces, tokens or grants: %w", ErrForbidden)
	}
	return ac, nil
}

func (a *Archive) userCount(ctx context.Context) (int, error) {
	var n int
	return n, a.sys(ctx, func(ctx context.Context) error {
		return a.st.Q(ctx).QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n)
	})
}

func (a *Archive) AddUser(ctx context.Context, email, name string, admin bool) (*User, error) {
	return a.addUser(ctx, email, name, admin, true)
}

// addUser makes a user with a home space they own. checked says whether the caller must be the
// instance admin, which every caller but an accepted invite must be, once there is a first user.
func (a *Archive) addUser(ctx context.Context, email, name string, admin, checked bool) (*User, error) {
	email, err := authEmail(email)
	if err != nil {
		return nil, err
	}
	u := &User{ID: coreNewID(), Email: email, Name: strings.TrimSpace(name)}
	u.Role = a.roleID("u", u.ID)
	return u, a.sys(ctx, func(ctx context.Context) error {
		q := a.st.Q(ctx)
		if _, err := q.Exec(ctx, `SELECT pg_advisory_xact_lock(7273)`); err != nil {
			return err
		}
		var first bool
		if err := q.QueryRow(ctx, `SELECT NOT EXISTS (SELECT FROM users)`).Scan(&first); err != nil {
			return err
		}
		if !first && checked {
			ac, err := authUser(ctx)
			if err != nil {
				return err
			}
			if !a.has(ctx, ac.UserRole, a.prefix+"_admin") {
				return fmt.Errorf("only instance admins add users: %w", ErrForbidden)
			}
		}
		var orphans []string
		if first {
			rows, _ := q.Query(ctx, `SELECT role FROM spaces`)
			orphans, _ = pgx.CollectRows(rows, pgx.RowTo[string])
		}
		home, err := a.createSpace(ctx, a.freeSpaceName(ctx, email))
		if err != nil {
			return err
		}
		u.Home = home
		err = q.QueryRow(ctx, `INSERT INTO users (id, email, name, role, home) VALUES ($1, $2, $3, $4, $5) RETURNING created_at`,
			u.ID, u.Email, u.Name, u.Role, home).Scan(&u.CreatedAt)
		if err != nil {
			return err
		}
		if err := a.createRole(ctx, u.Role); err != nil {
			return err
		}
		var homeRole string
		if err := q.QueryRow(ctx, `SELECT role FROM spaces WHERE id = $1`, home).Scan(&homeRole); err != nil {
			return err
		}
		for _, base := range append(orphans, homeRole) {
			if err := a.setLevel(ctx, u.Role, base, authLevel("owner")); err != nil {
				return err
			}
		}
		if admin || first {
			return a.exec(ctx, "GRANT "+authIdent(a.prefix+"_admin")+" TO "+authIdent(u.Role))
		}
		return nil
	})
}

func (a *Archive) userBy(ctx context.Context, where string, arg any) (*User, error) {
	var u *User
	return u, a.sys(ctx, func(ctx context.Context) (err error) {
		u, err = storeOne[User](ctx, a.st, `SELECT id, email, name, role, home, signin, created_at FROM users WHERE `+where, arg)
		return err
	})
}

func (a *Archive) userActor(u *User) *Actor {
	return &Actor{UserID: u.ID, Email: u.Email, Name: u.Name, Role: u.Role, UserRole: u.Role, Home: u.Home}
}

func (a *Archive) CLIActor(ctx context.Context, email string) (*Actor, error) {
	if email == "" {
		u, err := a.userBy(ctx, `true ORDER BY created_at, id LIMIT $1`, 1)
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return a.userActor(u), nil
	}
	email, err := authEmail(email)
	if err != nil {
		return nil, err
	}
	u, err := a.userBy(ctx, `email = $1`, email)
	if err != nil {
		return nil, fmt.Errorf("user %s: %w", email, err)
	}
	return a.userActor(u), nil
}

func (a *Archive) Authenticate(ctx context.Context, secret string) (*Actor, error) {
	ac := &Actor{}
	var tid uuid.UUID
	var kind, trole string
	var ver int
	err := a.sys(ctx, func(ctx context.Context) error {
		return a.st.Q(ctx).QueryRow(ctx, `
			SELECT t.id, t.kind, t.name, coalesce(t.role, ''), u.id, u.email, u.name, u.role, u.home, (`+storeVersion+`)
			FROM tokens t JOIN users u ON u.id = t.user_id
			WHERE t.hash = $1 AND t.kind IN ('api', 'session', 'user') AND (t.expires_at IS NULL OR t.expires_at > now())`,
			authHash(secret)).Scan(&tid, &kind, &ac.TokenName, &trole, &ac.UserID, &ac.Email, &ac.Name, &ac.UserRole, &ac.Home, &ver)
	})
	if err == nil && ver != a.Spec().Version {
		err = a.reloadSpec(ctx)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("invalid or expired token: %w", ErrUnauthorized)
	}
	if err != nil {
		return nil, err
	}
	ac.Role = ac.UserRole
	switch kind {
	case "api":
		ac.Role, ac.TokenID = trole, tid
	case "session":
		ac.Session, ac.TokenName = tid, ""
	}
	return ac, nil
}

func (a *Archive) reloadSpec(ctx context.Context) error {
	return a.sys(ctx, func(ctx context.Context) error {
		spec, err := a.st.CurrentSpec(ctx)
		if err == nil {
			a.setSpec(spec)
		}
		return err
	})
}

func (a *Archive) newToken(ctx context.Context, userID uuid.UUID, kind string, ttl time.Duration) (string, error) {
	secret := authNewSecret()
	return secret, a.sys(ctx, func(ctx context.Context) error {
		_, err := a.st.Q(ctx).Exec(ctx, `INSERT INTO tokens (user_id, kind, hash, expires_at) VALUES ($1, $2, $3, now() + $4::interval)`,
			userID, kind, authHash(secret), ttl.String())
		return err
	})
}

func (a *Archive) Session(ctx context.Context, userID uuid.UUID) (string, error) {
	return a.newToken(ctx, userID, "session", 30*24*time.Hour)
}

func (a *Archive) MagicLink(ctx context.Context, email string) (string, *User, error) {
	email, err := authEmail(email)
	if err != nil {
		return "", nil, err
	}
	u, err := a.userBy(ctx, `email = $1`, email)
	if err != nil {
		return "", nil, err
	}
	s, err := a.newToken(ctx, u.ID, "magic", 15*time.Minute)
	return s, u, err
}

func (a *Archive) UseMagicLink(ctx context.Context, secret string) (*User, error) {
	var id uuid.UUID
	err := a.sys(ctx, func(ctx context.Context) error {
		return a.st.Q(ctx).QueryRow(ctx, `DELETE FROM tokens WHERE hash = $1 AND kind = 'magic' AND expires_at > now() RETURNING user_id`,
			authHash(secret)).Scan(&id)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("sign-in link is invalid, used or expired: %w", ErrUnauthorized)
	}
	if err != nil {
		return nil, err
	}
	return a.userBy(ctx, `id = $1`, id)
}

func (a *Archive) SignOut(ctx context.Context) error {
	ac := ActorFrom(ctx)
	if ac == nil || ac.Session == uuid.Nil {
		return nil
	}
	return a.sys(ctx, func(ctx context.Context) error {
		_, err := a.st.Q(ctx).Exec(ctx, `DELETE FROM tokens WHERE id = $1`, ac.Session)
		return err
	})
}

func (a *Archive) Spaces(ctx context.Context) ([]SpaceInfo, error) {
	return coreDo(a, ctx, func(ctx context.Context) ([]SpaceInfo, error) {
		rows, _ := a.st.Q(ctx).Query(ctx, `
			SELECT id, name, CASE
				WHEN pg_has_role(current_user, (role || '_owner')::name, 'USAGE') THEN 'owner'
				WHEN pg_has_role(current_user, (role || '_admin')::name, 'USAGE') THEN 'admin'
				WHEN pg_has_role(current_user, (role || '_editor')::name, 'USAGE') THEN 'editor'
				ELSE 'viewer' END AS level
			FROM spaces ORDER BY name`)
		return pgx.CollectRows(rows, pgx.RowToStructByName[SpaceInfo])
	})
}

func (a *Archive) AddSpace(ctx context.Context, name string) (*SpaceInfo, error) {
	ac, err := authUser(ctx)
	if err != nil {
		return nil, err
	}
	sp := &SpaceInfo{Name: name, Level: "owner"}
	return sp, a.sys(ctx, func(ctx context.Context) error {
		if sp.ID, err = a.createSpace(ctx, name); err != nil {
			return err
		}
		return a.setLevel(ctx, ac.UserRole, a.prefix+"_s_"+name, authLevel("owner"))
	})
}

func (a *Archive) DeleteSpace(ctx context.Context, name string) error {
	ac, err := authUser(ctx)
	if err != nil {
		return err
	}
	if _, err := coreDo(a, ctx, func(ctx context.Context) (string, error) { return a.spaceRole(ctx, name) }); err != nil {
		return err
	}
	return a.sys(ctx, func(ctx context.Context) error {
		q := a.st.Q(ctx)
		var id uuid.UUID
		var base string
		if err := q.QueryRow(ctx, `SELECT id, role FROM spaces WHERE name = $1`, name).Scan(&id, &base); err != nil {
			return fmt.Errorf("space %q: %w", name, ErrNotFound)
		}
		if !a.has(ctx, ac.UserRole, base+"_owner") {
			return fmt.Errorf("deleting %s needs owner: %w", name, ErrForbidden)
		}
		var home string
		if q.QueryRow(ctx, `SELECT email FROM users WHERE home = $1`, id).Scan(&home) == nil {
			return fmt.Errorf("%s is the personal space of %s: %w", name, home, ErrConflict)
		}
		for _, t := range []string{"proposals", "views", "sources", "secrets", "links", "provenance", "records", "raw"} {
			if _, err := q.Exec(ctx, `DELETE FROM `+t+` WHERE space = $1`, id); err != nil {
				return err
			}
		}
		if _, err := q.Exec(ctx, `DELETE FROM spaces WHERE id = $1`, id); err != nil {
			return err
		}
		for i := len(authLevels) - 1; i >= 0; i-- {
			if err := a.exec(ctx, "REVOKE ALL ON "+authTables+" FROM "+authIdent(base+"_"+authLevels[i]), "DROP ROLE "+authIdent(base+"_"+authLevels[i])); err != nil {
				return err
			}
		}
		return nil
	})
}

func (a *Archive) spaceRole(ctx context.Context, name string) (string, error) {
	var role string
	err := a.st.Q(ctx).QueryRow(ctx, `SELECT role FROM spaces WHERE name = $1`, name).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("space %q: %w", name, ErrNotFound)
	}
	return role, err
}

func (a *Archive) Members(ctx context.Context, space string) ([]Member, error) {
	if _, err := coreDo(a, ctx, func(ctx context.Context) (string, error) { return a.spaceRole(ctx, space) }); err != nil {
		return nil, err
	}
	var out []Member
	return out, a.sys(ctx, func(ctx context.Context) error {
		base, err := a.spaceRole(ctx, space)
		if err != nil {
			return err
		}
		rows, _ := a.st.Q(ctx).Query(ctx, `SELECT email, name, role AS level FROM users ORDER BY email`)
		users, err := pgx.CollectRows(rows, pgx.RowToStructByName[Member])
		for _, m := range users {
			if l := a.level(ctx, m.Level, base, true); l > 0 {
				m.Level = authLevels[l-1]
				out = append(out, m)
			}
		}
		return err
	})
}

func (a *Archive) SetMember(ctx context.Context, space, email, level string) error {
	ac, err := authUser(ctx)
	if err != nil {
		return err
	}
	lvl := authLevel(level)
	if lvl == 0 && level != "none" {
		return &ValidationError{Type: "member", Errors: []FieldError{{Path: "role", Message: "want owner, admin, editor, viewer or none"}}}
	}
	if email, err = authEmail(email); err != nil {
		return err
	}
	return a.sys(ctx, func(ctx context.Context) error {
		q := a.st.Q(ctx)
		base, err := a.spaceRole(ctx, space)
		if err != nil {
			return err
		}
		var uid uuid.UUID
		var role string
		if err := q.QueryRow(ctx, `SELECT id, role FROM users WHERE email = $1`, email).Scan(&uid, &role); err != nil {
			return fmt.Errorf("user %s: %w", email, ErrNotFound)
		}
		cur := a.level(ctx, role, base, true)
		need := "admin"
		if max(lvl, cur) == authLevel("owner") {
			need = "owner"
		}
		if !a.has(ctx, ac.UserRole, base+"_"+need) {
			return fmt.Errorf("changing this membership needs %s in %s: %w", need, space, ErrForbidden)
		}
		if cur == authLevel("owner") && lvl != cur {
			var owners int
			q.QueryRow(ctx, `SELECT count(*) FROM pg_auth_members m JOIN pg_roles r ON r.oid = m.roleid
				JOIN pg_roles u ON u.oid = m.member JOIN users ON users.role = u.rolname WHERE r.rolname = $1`, base+"_owner").Scan(&owners)
			if owners <= 1 {
				return fmt.Errorf("%s is the last owner of %s: %w", email, space, ErrConflict)
			}
		}
		if err := a.setLevel(ctx, role, base, lvl); err != nil {
			return err
		}
		rows, _ := q.Query(ctx, `SELECT role FROM tokens WHERE user_id = $1 AND role IS NOT NULL`, uid)
		troles, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			return err
		}
		for _, tr := range troles {
			if a.level(ctx, tr, base, true) > lvl {
				if err := a.setLevel(ctx, tr, base, lvl); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (a *Archive) CreateToken(ctx context.Context, name string, spaces map[string]string, ttl time.Duration, full bool) (*Token, error) {
	ac, err := authUser(ctx)
	if err != nil {
		return nil, err
	}
	if full && len(spaces) > 0 {
		return nil, &ValidationError{Type: "token", Errors: []FieldError{{Path: "spaces", Message: "a full token has all your rights, so it takes no spaces"}}}
	}
	t := &Token{ID: coreNewID(), Name: strings.TrimSpace(name), Spaces: map[string]string{}, Full: full, Secret: authNewSecret()}
	role := a.roleID("t", t.ID)
	var exp *time.Time
	if ttl > 0 {
		e := time.Now().Add(ttl).UTC()
		exp = &e
	}
	return t, a.sys(ctx, func(ctx context.Context) error {
		q := a.st.Q(ctx)
		if full {
			return q.QueryRow(ctx, `INSERT INTO tokens (id, user_id, kind, name, hash, expires_at) VALUES ($1, $2, 'user', $3, $4, $5) RETURNING created_at, expires_at`,
				t.ID, ac.UserID, t.Name, authHash(t.Secret), exp).Scan(&t.CreatedAt, &t.ExpiresAt)
		}
		rows, _ := q.Query(ctx, `SELECT name, role FROM spaces ORDER BY name`)
		all := map[string]string{}
		var sn, sr string
		if _, err := pgx.ForEachRow(rows, []any{&sn, &sr}, func() error { all[sn] = sr; return nil }); err != nil {
			return err
		}
		if len(spaces) == 0 {
			spaces = map[string]string{}
			for n, base := range all {
				if l := a.level(ctx, ac.UserRole, base, false); l > 0 {
					spaces[n] = authLevels[l-1]
				}
			}
		}
		for n, lv := range spaces {
			base, ok := all[n]
			if !ok {
				return fmt.Errorf("space %q: %w", n, ErrNotFound)
			}
			l := authLevel(lv)
			if l == 0 {
				return &ValidationError{Type: "token", Errors: []FieldError{{Path: "spaces." + n, Message: "want owner, admin, editor or viewer"}}}
			}
			if a.level(ctx, ac.UserRole, base, false) < l {
				return fmt.Errorf("you are not %s in %s, so the token cannot be: %w", lv, n, ErrForbidden)
			}
			t.Spaces[n] = lv
		}
		if len(t.Spaces) == 0 {
			return &ValidationError{Type: "token", Errors: []FieldError{{Path: "spaces", Message: "a token needs at least one space"}}}
		}
		err := q.QueryRow(ctx, `INSERT INTO tokens (id, user_id, kind, name, role, hash, expires_at) VALUES ($1, $2, 'api', $3, $4, $5, $6) RETURNING created_at, expires_at`,
			t.ID, ac.UserID, t.Name, role, authHash(t.Secret), exp).Scan(&t.CreatedAt, &t.ExpiresAt)
		if err != nil {
			return err
		}
		if err := a.createRole(ctx, role); err != nil {
			return err
		}
		for n, lv := range t.Spaces {
			if err := a.setLevel(ctx, role, all[n], authLevel(lv)); err != nil {
				return err
			}
		}
		return nil
	})
}

func (a *Archive) Tokens(ctx context.Context) ([]*Token, error) {
	ac, err := authUser(ctx)
	if err != nil {
		return nil, err
	}
	out := []*Token{}
	return out, a.sys(ctx, func(ctx context.Context) error {
		q := a.st.Q(ctx)
		rows, _ := q.Query(ctx, `SELECT name, role FROM spaces`)
		all := map[string]string{}
		var sn, sr string
		if _, err := pgx.ForEachRow(rows, []any{&sn, &sr}, func() error { all[sn] = sr; return nil }); err != nil {
			return err
		}
		rows, _ = q.Query(ctx, `SELECT id, name, coalesce(role, ''), kind = 'user', expires_at, created_at FROM tokens
			WHERE user_id = $1 AND kind IN ('api', 'user') ORDER BY id`, ac.UserID)
		var roles []string
		var role string
		t := &Token{}
		_, err := pgx.ForEachRow(rows, []any{&t.ID, &t.Name, &role, &t.Full, &t.ExpiresAt, &t.CreatedAt}, func() error {
			c := *t
			out, roles = append(out, &c), append(roles, role)
			return nil
		})
		for i, c := range out {
			c.Spaces = map[string]string{}
			for n, base := range all {
				if l := a.level(ctx, roles[i], base, true); l > 0 {
					c.Spaces[n] = authLevels[l-1]
				}
			}
		}
		return err
	})
}

func (a *Archive) RevokeToken(ctx context.Context, id uuid.UUID) error {
	ac, err := authUser(ctx)
	if err != nil {
		return err
	}
	return a.sys(ctx, func(ctx context.Context) error {
		var role string
		err := a.st.Q(ctx).QueryRow(ctx, `DELETE FROM tokens WHERE id = $1 AND user_id = $2 AND kind IN ('api', 'user') RETURNING coalesce(role, '')`, id, ac.UserID).Scan(&role)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("token %s: %w", id, ErrNotFound)
		}
		if err != nil || role == "" {
			return err
		}
		return a.exec(ctx, "DROP ROLE "+authIdent(role))
	})
}

func (a *Archive) GrantSecret(ctx context.Context, email, typ string, grant bool) error {
	ac, err := authUser(ctx)
	if err != nil {
		return err
	}
	if t := a.Spec().Types[typ]; t == nil || t.Sensitivity != Secret || a.Spec().secretRoot(typ) != typ {
		return fmt.Errorf("%s is not a secret type that holds its own grants: %w", typ, ErrNotFound)
	}
	if email, err = authEmail(email); err != nil {
		return err
	}
	return a.sys(ctx, func(ctx context.Context) error {
		if !a.has(ctx, ac.UserRole, a.prefix+"_admin") {
			return fmt.Errorf("only instance admins grant secret types: %w", ErrForbidden)
		}
		var role string
		if err := a.st.Q(ctx).QueryRow(ctx, `SELECT role FROM users WHERE email = $1`, email).Scan(&role); err != nil {
			return fmt.Errorf("user %s: %w", email, ErrNotFound)
		}
		group := a.prefix + "_secret_" + typ
		var exists bool
		a.st.Q(ctx).QueryRow(ctx, `SELECT EXISTS (SELECT FROM pg_roles WHERE rolname = $1)`, group).Scan(&exists)
		if !exists {
			if err := a.exec(ctx, "CREATE ROLE "+authIdent(group)+" NOLOGIN"); err != nil {
				return err
			}
		}
		if grant {
			return a.exec(ctx, "GRANT "+authIdent(group)+" TO "+authIdent(role))
		}
		return a.exec(ctx, "REVOKE "+authIdent(group)+" FROM "+authIdent(role))
	})
}

func (a *Archive) migrateSpaces(ctx context.Context) error {
	tables := []string{"records", "raw", "provenance", "links"}
	return a.sys(ctx, func(ctx context.Context) error {
		q := a.st.Q(ctx)
		var nullable bool
		err := q.QueryRow(ctx, `SELECT EXISTS (SELECT FROM information_schema.columns WHERE table_schema = current_schema()
			AND table_name = ANY($1) AND column_name = 'space' AND is_nullable = 'YES')`, tables).Scan(&nullable)
		if err != nil || !nullable {
			return err
		}
		var orphans bool
		if err := q.QueryRow(ctx, `SELECT EXISTS (SELECT FROM records WHERE space IS NULL) OR EXISTS (SELECT FROM raw WHERE space IS NULL)
			OR EXISTS (SELECT FROM provenance WHERE space IS NULL) OR EXISTS (SELECT FROM links WHERE space IS NULL)`).Scan(&orphans); err != nil {
			return err
		}
		if orphans {
			id, err := a.createSpace(ctx, a.freeSpaceName(ctx, "default"))
			if err != nil {
				return err
			}
			for _, t := range tables {
				if _, err := q.Exec(ctx, "UPDATE "+t+" SET space = $1 WHERE space IS NULL", id); err != nil {
					return err
				}
			}
			spec := a.Spec()
			f := Filter{Limit: 500}
			for {
				rs, err := a.st.ListRecords(ctx, f)
				if err != nil {
					return err
				}
				for _, r := range rs {
					data := map[string]any{}
					if json.Unmarshal(r.Data, &data) != nil || spec.Types[r.Type] == nil {
						continue
					}
					changed, err := a.sealRecord(ctx, spec, r, data)
					if err != nil {
						return err
					}
					if changed {
						if r.Data, err = json.Marshal(data); err != nil {
							return err
						}
						if err := a.st.UpdateRecord(ctx, r); err != nil {
							return err
						}
					}
				}
				if len(rs) < f.Limit {
					break
				}
				f.After = rs[len(rs)-1].ID
			}
		}
		for _, t := range tables {
			if _, err := q.Exec(ctx, "ALTER TABLE "+t+" ALTER COLUMN space SET NOT NULL"); err != nil {
				return err
			}
		}
		return nil
	})
}
