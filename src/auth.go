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
	Space     string    `json:"-"`
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

func authCreate(role string) string { return "CREATE ROLE " + authIdent(role) + " NOLOGIN" }

func authDrop(role string) string { return "DROP ROLE " + authIdent(role) }

func authGrant(group, role string) string {
	return "GRANT " + authIdent(group) + " TO " + authIdent(role)
}

func authRevoke(group, role string) string {
	return "REVOKE " + authIdent(group) + " FROM " + authIdent(role)
}

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
	ok, err := a.st.db(ctx).HasRole(ctx, role, group)
	return err == nil && ok
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
	names, _ := a.st.db(ctx).DirectRoles(ctx, role, []string{base + "_viewer", base + "_editor", base + "_admin", base + "_owner"})
	n := 0
	for _, s := range names {
		n = max(n, authLevel(strings.TrimPrefix(s, base+"_")))
	}
	return n
}

func (a *Archive) setLevel(ctx context.Context, role, base string, lvl int) error {
	for _, l := range authLevels {
		if err := a.exec(ctx, authRevoke(base+"_"+l, role)); err != nil {
			return err
		}
	}
	if lvl == 0 {
		return nil
	}
	return a.exec(ctx, authGrant(base+"_"+authLevels[lvl-1], role))
}

func (a *Archive) createSpace(ctx context.Context, name string) (uuid.UUID, error) {
	var id uuid.UUID
	if !authSpaceRe.MatchString(name) {
		return id, &ValidationError{Type: "space", Errors: []FieldError{{Path: "name", Message: "want 1-32 lowercase letters, digits or _"}}}
	}
	base := a.prefix + "_s_" + name
	id, err := a.st.db(ctx).CreateSpace(ctx, name, base)
	if err != nil {
		return id, err
	}
	for i, l := range authLevels {
		if err := a.exec(ctx, authCreate(base+"_"+l)); err != nil {
			return id, err
		}
		if i > 0 {
			if err := a.exec(ctx, authGrant(base+"_"+authLevels[i-1], base+"_"+l)); err != nil {
				return id, err
			}
		}
	}
	return id, nil
}

func (a *Archive) createRole(ctx context.Context, role string) error {
	return a.exec(ctx, authCreate(role), authGrant(a.prefix+"_user", role), authGrant(role, a.prefix+"_authenticator"))
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
		if taken, _ := a.st.db(ctx).SpaceTaken(ctx, name); !taken {
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
	var n int64
	err := a.sys(ctx, func(ctx context.Context) (err error) {
		n, err = a.st.db(ctx).CountUsers(ctx)
		return err
	})
	return int(n), err
}

func (a *Archive) AddUser(ctx context.Context, email, name string, admin bool) (*User, error) {
	email, err := authEmail(email)
	if err != nil {
		return nil, err
	}
	u := &User{ID: coreNewID(), Email: email, Name: strings.TrimSpace(name)}
	u.Role = a.roleID("u", u.ID)
	return u, a.sys(ctx, func(ctx context.Context) error {
		q := a.st.db(ctx)
		if err := q.AdvisoryXactLock(ctx, 7273); err != nil {
			return err
		}
		first, err := q.NoUsers(ctx)
		if err != nil {
			return err
		}
		if !first {
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
			spaces, _ := q.SpaceRoles(ctx)
			for _, sp := range spaces {
				orphans = append(orphans, sp.Role)
			}
		}
		home, err := a.createSpace(ctx, a.freeSpaceName(ctx, email))
		if err != nil {
			return err
		}
		u.Home = home
		u.CreatedAt, err = q.CreateUser(ctx, CreateUserParams{ID: u.ID, Email: u.Email, Name: u.Name, Role: u.Role, Home: home})
		if err != nil {
			return err
		}
		if err := a.createRole(ctx, u.Role); err != nil {
			return err
		}
		homeRole, err := q.SpaceRoleByID(ctx, home)
		if err != nil {
			return err
		}
		for _, base := range append(orphans, homeRole) {
			if err := a.setLevel(ctx, u.Role, base, authLevel("owner")); err != nil {
				return err
			}
		}
		if admin || first {
			return a.exec(ctx, authGrant(a.prefix+"_admin", u.Role))
		}
		return nil
	})
}

func userBy[T any](ctx context.Context, a *Archive, get func(*Queries, context.Context, T) (ArchiveUser, error), arg T) (*User, error) {
	var u *User
	return u, a.sys(ctx, func(ctx context.Context) error {
		r, err := get(a.st.db(ctx), ctx, arg)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		u = (*User)(&r)
		return err
	})
}

func (a *Archive) userActor(u *User) *Actor {
	return &Actor{UserID: u.ID, Email: u.Email, Name: u.Name, Role: u.Role, UserRole: u.Role, Home: u.Home}
}

func (a *Archive) CLIActor(ctx context.Context, email string) (*Actor, error) {
	if email == "" {
		u, err := userBy(ctx, a, (*Queries).FirstUser, 1)
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
	u, err := userBy(ctx, a, (*Queries).UserByEmail, email)
	if err != nil {
		return nil, fmt.Errorf("user %s: %w", email, err)
	}
	return a.userActor(u), nil
}

func (a *Archive) Authenticate(ctx context.Context, secret string) (*Actor, error) {
	var t AuthenticateRow
	err := a.sys(ctx, func(ctx context.Context) (err error) {
		t, err = a.st.db(ctx).Authenticate(ctx, authHash(secret))
		return err
	})
	if err == nil && t.Stamp != a.Catalog().stamp {
		err = a.reload(ctx)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("invalid or expired token: %w", ErrUnauthorized)
	}
	if err != nil {
		return nil, err
	}
	ac := &Actor{UserID: t.UserID, Email: t.Email, Name: t.UserName, TokenName: t.Name, Role: t.UserRole, UserRole: t.UserRole, Home: t.Home}
	switch t.Kind {
	case "api":
		ac.Role, ac.TokenID = t.Role, t.ID
	case "session":
		ac.Session, ac.TokenName = t.ID, ""
	}
	return ac, nil
}

func (a *Archive) newToken(ctx context.Context, userID uuid.UUID, kind string, ttl time.Duration) (string, error) {
	secret := authNewSecret()
	return secret, a.sys(ctx, func(ctx context.Context) error {
		return a.st.db(ctx).CreateLoginToken(ctx, CreateLoginTokenParams{UserID: userID, Kind: kind, Hash: authHash(secret), Ttl: ttl.String()})
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
	u, err := userBy(ctx, a, (*Queries).UserByEmail, email)
	if err != nil {
		return "", nil, err
	}
	s, err := a.newToken(ctx, u.ID, "magic", 15*time.Minute)
	return s, u, err
}

func (a *Archive) UseMagicLink(ctx context.Context, secret string) (*User, error) {
	var id uuid.UUID
	err := a.sys(ctx, func(ctx context.Context) (err error) {
		id, err = a.st.db(ctx).UseMagicLink(ctx, authHash(secret))
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("sign-in link is invalid, used or expired: %w", ErrUnauthorized)
	}
	if err != nil {
		return nil, err
	}
	return userBy(ctx, a, (*Queries).UserByID, id)
}

func (a *Archive) SignOut(ctx context.Context) error {
	ac := ActorFrom(ctx)
	if ac == nil || ac.Session == uuid.Nil {
		return nil
	}
	return a.sys(ctx, func(ctx context.Context) error {
		return a.st.db(ctx).DeleteToken(ctx, ac.Session)
	})
}

func (a *Archive) Spaces(ctx context.Context) ([]SpaceInfo, error) {
	return coreDo(a, ctx, func(ctx context.Context) ([]SpaceInfo, error) {
		rows, err := a.st.db(ctx).Spaces(ctx)
		out := make([]SpaceInfo, len(rows))
		for i, r := range rows {
			out[i] = SpaceInfo(r)
		}
		return out, err
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
		q := a.st.db(ctx)
		sp, err := q.SpaceByName(ctx, name)
		if err != nil {
			return fmt.Errorf("space %q: %w", name, ErrNotFound)
		}
		id, base := sp.ID, sp.Role
		if !a.has(ctx, ac.UserRole, base+"_owner") {
			return fmt.Errorf("deleting %s needs owner: %w", name, ErrForbidden)
		}
		if home, err := q.HomeOwner(ctx, id); err == nil {
			return fmt.Errorf("%s is the personal space of %s: %w", name, home, ErrConflict)
		}
		if err := q.DeleteSpaceSecrets(ctx, id); err != nil {
			return err
		}
		for _, t := range a.Catalog().deleteOrder() {
			if _, err := a.st.Q(ctx).Exec(ctx, `DELETE FROM `+pgIdent("public", t)+` WHERE space = $1`, id); err != nil {
				return err
			}
		}
		if err := q.DeleteSpaceFiles(ctx, id); err != nil {
			return err
		}
		if err := q.DeleteSpace(ctx, id); err != nil {
			return err
		}
		for i := len(authLevels) - 1; i >= 0; i-- {
			if err := a.exec(ctx, authDrop(base+"_"+authLevels[i])); err != nil {
				return err
			}
		}
		return nil
	})
}

func (a *Archive) spaceRoles(ctx context.Context) (map[string]string, error) {
	rows, err := a.st.db(ctx).SpaceRoles(ctx)
	all := map[string]string{}
	for _, r := range rows {
		all[r.Name] = r.Role
	}
	return all, err
}

func (a *Archive) spaceRole(ctx context.Context, name string) (string, error) {
	role, err := a.st.db(ctx).SpaceRole(ctx, name)
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
		users, err := a.st.db(ctx).Members(ctx)
		for _, r := range users {
			m := Member(r)
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
		q := a.st.db(ctx)
		base, err := a.spaceRole(ctx, space)
		if err != nil {
			return err
		}
		u, err := q.UserRole(ctx, email)
		if err != nil {
			return fmt.Errorf("user %s: %w", email, ErrNotFound)
		}
		uid, role := u.ID, u.Role
		cur := a.level(ctx, role, base, true)
		need := "admin"
		if max(lvl, cur) == authLevel("owner") {
			need = "owner"
		}
		if !a.has(ctx, ac.UserRole, base+"_"+need) {
			return fmt.Errorf("changing this membership needs %s in %s: %w", need, space, ErrForbidden)
		}
		if cur == authLevel("owner") && lvl != cur {
			if owners, _ := q.CountOwners(ctx, base+"_owner"); owners <= 1 {
				return fmt.Errorf("%s is the last owner of %s: %w", email, space, ErrConflict)
			}
		}
		if err := a.setLevel(ctx, role, base, lvl); err != nil {
			return err
		}
		troles, err := q.TokenRoles(ctx, uid)
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
		q := a.st.db(ctx)
		if full {
			r, err := q.CreateUserToken(ctx, CreateUserTokenParams{ID: t.ID, UserID: ac.UserID, Name: t.Name, Hash: authHash(t.Secret), ExpiresAt: exp})
			t.CreatedAt, t.ExpiresAt = r.CreatedAt, r.ExpiresAt
			return err
		}
		all, err := a.spaceRoles(ctx)
		if err != nil {
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
		r, err := q.CreateAPIToken(ctx, CreateAPITokenParams{ID: t.ID, UserID: ac.UserID, Name: t.Name, Role: &role, Hash: authHash(t.Secret), ExpiresAt: exp})
		if err != nil {
			return err
		}
		t.CreatedAt, t.ExpiresAt = r.CreatedAt, r.ExpiresAt
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
		all, err := a.spaceRoles(ctx)
		if err != nil {
			return err
		}
		rows, err := a.st.db(ctx).Tokens(ctx, ac.UserID)
		for _, r := range rows {
			c := &Token{ID: r.ID, Name: r.Name, Spaces: map[string]string{}, Full: r.Full, ExpiresAt: r.ExpiresAt, CreatedAt: r.CreatedAt}
			for n, base := range all {
				if l := a.level(ctx, r.Role, base, true); l > 0 {
					c.Spaces[n] = authLevels[l-1]
				}
			}
			out = append(out, c)
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
		role, err := a.st.db(ctx).RevokeToken(ctx, id, ac.UserID)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("token %s: %w", id, ErrNotFound)
		}
		if err != nil || role == "" {
			return err
		}
		return a.exec(ctx, authDrop(role))
	})
}

func (a *Archive) GrantSecret(ctx context.Context, email, typ string, grant bool) error {
	ac, err := authUser(ctx)
	if err != nil {
		return err
	}
	if !semLib[typ].Secret {
		return fmt.Errorf("%s is not a secret type: %w", typ, ErrNotFound)
	}
	if email, err = authEmail(email); err != nil {
		return err
	}
	return a.sys(ctx, func(ctx context.Context) error {
		if !a.has(ctx, ac.UserRole, a.prefix+"_admin") {
			return fmt.Errorf("only instance admins grant secret types: %w", ErrForbidden)
		}
		u, err := a.st.db(ctx).UserRole(ctx, email)
		if err != nil {
			return fmt.Errorf("user %s: %w", email, ErrNotFound)
		}
		role, group := u.Role, a.prefix+"_secret_"+typ
		if exists, _ := a.st.db(ctx).RoleExists(ctx, group); !exists {
			if err := a.exec(ctx, authCreate(group)); err != nil {
				return err
			}
		}
		if grant {
			return a.exec(ctx, authGrant(group, role))
		}
		return a.exec(ctx, authRevoke(group, role))
	})
}
