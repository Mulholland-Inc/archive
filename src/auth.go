package main

import (
	"cmp"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Mulholland-Inc/archive/generated"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/oauth2"
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
		u.CreatedAt, err = q.CreateUser(ctx, generated.CreateUserParams{ID: u.ID, Email: u.Email, Name: u.Name, Role: u.Role, Home: home})
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

func userBy[T any](ctx context.Context, a *Archive, get func(*generated.Queries, context.Context, T) (generated.ArchiveUser, error), arg T) (*User, error) {
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
		u, err := userBy(ctx, a, (*generated.Queries).FirstUser, 1)
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
	u, err := userBy(ctx, a, (*generated.Queries).UserByEmail, email)
	if err != nil {
		return nil, fmt.Errorf("user %s: %w", email, err)
	}
	return a.userActor(u), nil
}

func (a *Archive) Authenticate(ctx context.Context, secret string) (*Actor, error) {
	var t generated.AuthenticateRow
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
		return a.st.db(ctx).CreateLoginToken(ctx, generated.CreateLoginTokenParams{UserID: userID, Kind: kind, Hash: authHash(secret), Ttl: ttl.String()})
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
	u, err := userBy(ctx, a, (*generated.Queries).UserByEmail, email)
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
	return userBy(ctx, a, (*generated.Queries).UserByID, id)
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
			r, err := q.CreateUserToken(ctx, generated.CreateUserTokenParams{ID: t.ID, UserID: ac.UserID, Name: t.Name, Hash: authHash(t.Secret), ExpiresAt: exp})
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
		r, err := q.CreateAPIToken(ctx, generated.CreateAPITokenParams{ID: t.ID, UserID: ac.UserID, Name: t.Name, Role: &role, Hash: authHash(t.Secret), ExpiresAt: exp})
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

const (
	signinCookie   = "archive_session"
	signinCeremony = "archive_webauthn"
	signinOIDC     = "archive_oidc"
)

type signin struct {
	a      *Archive
	base   *url.URL
	secure bool
	wa     *webauthn.WebAuthn
	oidc   map[string]signinOIDCConfig

	mu         sync.Mutex
	setup      string
	ceremonies map[string]signinCeremonyData
	providers  map[string]*signinProvider
}

type signinCeremonyData struct {
	data webauthn.SessionData
	user uuid.UUID
	exp  time.Time
}

type signinOIDCConfig struct {
	Name         string `json:"name"`
	Issuer       string `json:"issuer"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type signinProvider struct {
	cfg      oauth2.Config
	verifier *oidc.IDTokenVerifier
	multi    bool
}

type signinMethods struct {
	Passkeys []webauthn.Credential `json:"passkeys,omitempty"`
	OIDC     []signinOIDCLink      `json:"oidc,omitempty"`
}

type signinOIDCLink struct {
	Issuer  string `json:"iss"`
	Subject string `json:"sub"`
}

type signinUser struct {
	u *User
	m signinMethods
}

func (p *signinUser) WebAuthnID() []byte                         { return p.u.ID[:] }
func (p *signinUser) WebAuthnName() string                       { return p.u.Email }
func (p *signinUser) WebAuthnDisplayName() string                { return cmp.Or(p.u.Name, p.u.Email) }
func (p *signinUser) WebAuthnCredentials() []webauthn.Credential { return p.m.Passkeys }

func newSignin(a *Archive, base string) (*signin, error) {
	u, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("ARCHIVE_URL %q is not a URL", base)
	}
	host := u.Hostname()
	s := &signin{a: a, base: u, secure: host != "localhost" && host != "127.0.0.1" && host != "::1",
		oidc: map[string]signinOIDCConfig{}, ceremonies: map[string]signinCeremonyData{}, providers: map[string]*signinProvider{}}
	if s.wa, err = webauthn.New(&webauthn.Config{RPID: host, RPDisplayName: "Archive", RPOrigins: []string{u.Scheme + "://" + u.Host}}); err != nil {
		return nil, fmt.Errorf("passkeys: %w", err)
	}
	if raw := os.Getenv("ARCHIVE_OIDC"); raw != "" {
		var list []signinOIDCConfig
		if err := json.Unmarshal([]byte(raw), &list); err != nil {
			return nil, fmt.Errorf("ARCHIVE_OIDC: %w", err)
		}
		for _, c := range list {
			if c.Name == "" || c.Issuer == "" || c.ClientID == "" {
				return nil, errors.New("ARCHIVE_OIDC: every provider needs name, issuer and client_id")
			}
			s.oidc[c.Name] = c
		}
	}
	return s, nil
}

func (a *Archive) httpActor(r *http.Request) (*Actor, error) {
	if h := r.Header.Get("Authorization"); h != "" {
		tok, ok := strings.CutPrefix(h, "Bearer ")
		if !ok {
			return nil, fmt.Errorf("want Authorization: Bearer <token>: %w", ErrUnauthorized)
		}
		return a.Authenticate(r.Context(), strings.TrimSpace(tok))
	}
	if c, err := r.Cookie(signinCookie); err == nil {
		return a.Authenticate(r.Context(), c.Value)
	}
	return nil, fmt.Errorf("sign in or send a bearer token: %w", ErrUnauthorized)
}

func (a *Archive) httpAuth(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ac, err := a.httpActor(r)
		if err != nil {
			httpWrite(w, 0, nil, err)
			return
		}
		ac.Space = r.Header.Get("Archive-Space")
		h.ServeHTTP(w, r.WithContext(WithActor(r.Context(), ac)))
	})
}

func (s *signin) handler() http.Handler {
	a, mux := s.a, http.NewServeMux()
	open := func(pattern string, f func(w http.ResponseWriter, r *http.Request) (int, any, error)) {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			status, v, err := f(w, r)
			httpWrite(w, status, v, err)
		})
	}
	authed := func(pattern string, f func(w http.ResponseWriter, r *http.Request) (int, any, error)) {
		mux.Handle(pattern, a.httpAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			status, v, err := f(w, r)
			httpWrite(w, status, v, err)
		})))
	}
	ok := func(v any, err error) (int, any, error) { return http.StatusOK, v, err }

	open("POST /auth/magic", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		var body struct {
			Email string `json:"email"`
		}
		if err := httpDecode(r, &body); err != nil {
			return 0, nil, err
		}
		secret, u, err := a.MagicLink(r.Context(), body.Email)
		if err == nil {
			err = s.mail(u.Email, s.base.String()+"/auth/magic?token="+url.QueryEscape(secret))
		}
		if err != nil && !errors.Is(err, ErrNotFound) {
			return 0, nil, err
		}
		return http.StatusAccepted, map[string]string{"status": "if that address has an account, a sign-in link is on its way"}, nil
	})
	mux.HandleFunc("GET /auth/magic", func(w http.ResponseWriter, r *http.Request) {
		s.browser(w, r, func() (*User, error) { return a.UseMagicLink(r.Context(), r.URL.Query().Get("token")) })
	})
	open("GET /auth/methods", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		providers := append([]string{}, slices.Sorted(maps.Keys(s.oidc))...)
		return ok(map[string]any{"providers": providers, "mail": os.Getenv("ARCHIVE_SMTP_URL") != ""}, nil)
	})
	open("POST /auth/setup", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		var body struct {
			Token string `json:"token"`
			Email string `json:"email"`
			Name  string `json:"name"`
		}
		if err := httpDecode(r, &body); err != nil {
			return 0, nil, err
		}
		body.Token = cmp.Or(body.Token, r.URL.Query().Get("token"))
		s.mu.Lock()
		valid := s.setup != "" && body.Token == s.setup
		if valid {
			s.setup = ""
		}
		s.mu.Unlock()
		if !valid {
			return 0, nil, fmt.Errorf("setup link is invalid or used: %w", ErrUnauthorized)
		}
		u, err := a.AddUser(r.Context(), body.Email, body.Name, true)
		if err != nil {
			s.mu.Lock()
			s.setup = body.Token
			s.mu.Unlock()
			return 0, nil, err
		}
		return ok(s.signedIn(w, r, u))
	})
	authed("POST /auth/signout", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		http.SetCookie(w, s.cookie(signinCookie, "", "/", -1))
		return http.StatusNoContent, nil, a.SignOut(r.Context())
	})
	authed("GET /auth/me", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		sp, err := a.Spaces(r.Context())
		return ok(map[string]any{"actor": ActorFrom(r.Context()), "spaces": sp, "home": ActorFrom(r.Context()).Home}, err)
	})
	authed("GET /auth/spaces", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		return ok(a.Spaces(r.Context()))
	})
	authed("POST /auth/spaces", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		var body struct {
			Name string `json:"name"`
		}
		if err := httpDecode(r, &body); err != nil {
			return 0, nil, err
		}
		sp, err := a.AddSpace(r.Context(), body.Name)
		return http.StatusCreated, sp, err
	})
	authed("DELETE /auth/spaces/{space}", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		return http.StatusNoContent, nil, a.DeleteSpace(r.Context(), r.PathValue("space"))
	})
	authed("GET /auth/spaces/{space}/members", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		return ok(a.Members(r.Context(), r.PathValue("space")))
	})
	authed("POST /auth/spaces/{space}/members", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		var body struct {
			Email string `json:"email"`
			Role  string `json:"role"`
		}
		if err := httpDecode(r, &body); err != nil {
			return 0, nil, err
		}
		if err := a.SetMember(r.Context(), r.PathValue("space"), body.Email, body.Role); err != nil {
			return 0, nil, err
		}
		return ok(a.Members(r.Context(), r.PathValue("space")))
	})
	authed("GET /auth/tokens", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		return ok(a.Tokens(r.Context()))
	})
	authed("POST /auth/tokens", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		var body struct {
			Name    string            `json:"name"`
			Spaces  map[string]string `json:"spaces"`
			Expires string            `json:"expires"`
			Full    bool              `json:"full"`
		}
		if err := httpDecode(r, &body); err != nil {
			return 0, nil, err
		}
		var ttl time.Duration
		if body.Expires != "" {
			var err error
			if ttl, err = time.ParseDuration(body.Expires); err != nil {
				return 0, nil, fmt.Errorf("expires: %v: %w", err, httpErrBadRequest)
			}
		}
		t, err := a.CreateToken(r.Context(), body.Name, body.Spaces, ttl, body.Full)
		return http.StatusCreated, t, err
	})
	authed("DELETE /auth/tokens/{id}", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		id, err := httpID(r)
		if err != nil {
			return 0, nil, err
		}
		return http.StatusNoContent, nil, a.RevokeToken(r.Context(), id)
	})

	authed("POST /auth/passkey/register", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		ac, err := authUser(r.Context())
		if err != nil {
			return 0, nil, err
		}
		pu, err := s.user(r.Context(), ac.UserID)
		if err != nil {
			return 0, nil, err
		}
		var exclude []protocol.CredentialDescriptor
		for _, c := range pu.m.Passkeys {
			exclude = append(exclude, c.Descriptor())
		}
		opts, data, err := s.wa.BeginRegistration(pu, webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired),
			webauthn.WithExclusions(exclude))
		if err != nil {
			return 0, nil, err
		}
		s.begin(w, *data, ac.UserID)
		return ok(opts, nil)
	})
	authed("POST /auth/passkey/register/finish", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		ac, err := authUser(r.Context())
		if err != nil {
			return 0, nil, err
		}
		cer, err := s.finish(w, r)
		if err != nil || cer.user != ac.UserID {
			return 0, nil, cmp.Or(err, fmt.Errorf("passkey ceremony belongs to another user: %w", ErrForbidden))
		}
		pu, err := s.user(r.Context(), ac.UserID)
		if err != nil {
			return 0, nil, err
		}
		cred, err := s.wa.FinishRegistration(pu, cer.data, r)
		if err != nil {
			return 0, nil, fmt.Errorf("passkey: %v: %w", err, httpErrBadRequest)
		}
		pu.m.Passkeys = append(pu.m.Passkeys, *cred)
		return ok(map[string]int{"passkeys": len(pu.m.Passkeys)}, s.save(r.Context(), pu))
	})
	open("POST /auth/passkey/login", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		opts, data, err := s.wa.BeginDiscoverableLogin()
		if err != nil {
			return 0, nil, err
		}
		s.begin(w, *data, uuid.Nil)
		return ok(opts, nil)
	})
	open("POST /auth/passkey/login/finish", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		cer, err := s.finish(w, r)
		if err != nil {
			return 0, nil, err
		}
		var found *signinUser
		_, cred, err := s.wa.FinishPasskeyLogin(func(_, handle []byte) (webauthn.User, error) {
			id, err := uuid.FromBytes(handle)
			if err != nil {
				return nil, err
			}
			found, err = s.user(r.Context(), id)
			return found, err
		}, cer.data, r)
		if err != nil {
			return 0, nil, fmt.Errorf("passkey: %v: %w", err, ErrUnauthorized)
		}
		for i, c := range found.m.Passkeys {
			if string(c.ID) == string(cred.ID) {
				found.m.Passkeys[i] = *cred
			}
		}
		if err := s.save(r.Context(), found); err != nil {
			return 0, nil, err
		}
		return ok(s.signedIn(w, r, found.u))
	})

	mux.HandleFunc("GET /auth/oidc/{provider}", func(w http.ResponseWriter, r *http.Request) {
		p, err := s.provider(r.PathValue("provider"))
		if err != nil {
			httpWrite(w, 0, nil, err)
			return
		}
		state, nonce, verifier := authNewSecret(), authNewSecret(), oauth2.GenerateVerifier()
		c := s.cookie(signinOIDC, state+"."+nonce+"."+verifier, "/auth/oidc/", 600)
		http.SetCookie(w, c)
		http.Redirect(w, r, p.cfg.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier)), http.StatusFound)
	})
	mux.HandleFunc("GET /auth/oidc/{provider}/callback", func(w http.ResponseWriter, r *http.Request) {
		s.browser(w, r, func() (*User, error) { return s.oidcCallback(w, r) })
	})
	return mux
}

func (s *signin) browser(w http.ResponseWriter, r *http.Request, user func() (*User, error)) {
	u, err := user()
	var v any
	if err == nil {
		v, err = s.signedIn(w, r, u)
	}
	if err == nil && strings.Contains(r.Header.Get("Accept"), "text/html") {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	httpWrite(w, http.StatusOK, v, err)
}

func (s *signin) cookie(name, value, path string, maxAge int) *http.Cookie {
	return &http.Cookie{Name: name, Value: value, Path: path, MaxAge: maxAge, HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteLaxMode}
}

func (s *signin) signedIn(w http.ResponseWriter, r *http.Request, u *User) (any, error) {
	secret, err := s.a.Session(r.Context(), u.ID)
	if err != nil {
		return nil, err
	}
	http.SetCookie(w, s.cookie(signinCookie, secret, "/", 30*24*3600))
	return map[string]any{"user": u}, nil
}

func (s *signin) mail(to, link string) error {
	raw := os.Getenv("ARCHIVE_SMTP_URL")
	if raw == "" {
		fmt.Fprintf(os.Stderr, "archive: sign-in link for %s: %s\n", to, link)
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return errors.New("ARCHIVE_SMTP_URL must look like smtp://user:pass@host:587")
	}
	var auth smtp.Auth
	if u.User != nil {
		pw, _ := u.User.Password()
		auth = smtp.PlainAuth("", u.User.Username(), pw, u.Hostname())
	}
	from := coreEnv("ARCHIVE_SMTP_FROM", "archive@"+s.base.Hostname())
	msg := "From: " + from + "\r\nTo: " + to + "\r\nSubject: Sign in to Archive\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n" +
		"Open this link within 15 minutes to sign in to Archive:\r\n\r\n" + link + "\r\n"
	return smtp.SendMail(u.Hostname()+":"+cmp.Or(u.Port(), "587"), auth, from, []string{to}, []byte(msg))
}

func (s *signin) user(ctx context.Context, id uuid.UUID) (*signinUser, error) {
	u, err := userBy(ctx, s.a, (*generated.Queries).UserByID, id)
	if err != nil {
		return nil, err
	}
	pu := &signinUser{u: u}
	if len(u.Signin) > 0 {
		if err := json.Unmarshal(u.Signin, &pu.m); err != nil {
			return nil, err
		}
	}
	return pu, nil
}

func (s *signin) save(ctx context.Context, pu *signinUser) error {
	b, err := json.Marshal(pu.m)
	if err != nil {
		return err
	}
	return s.a.sys(ctx, func(ctx context.Context) error {
		return s.a.st.db(ctx).SetSignin(ctx, pu.u.ID, b)
	})
}

func (s *signin) begin(w http.ResponseWriter, data webauthn.SessionData, user uuid.UUID) {
	key := authNewSecret()
	s.mu.Lock()
	for k, c := range s.ceremonies {
		if time.Now().After(c.exp) {
			delete(s.ceremonies, k)
		}
	}
	s.ceremonies[key] = signinCeremonyData{data, user, time.Now().Add(5 * time.Minute)}
	s.mu.Unlock()
	http.SetCookie(w, s.cookie(signinCeremony, key, "/auth/passkey/", 300))
}

func (s *signin) finish(w http.ResponseWriter, r *http.Request) (signinCeremonyData, error) {
	c, err := r.Cookie(signinCeremony)
	if err != nil {
		return signinCeremonyData{}, fmt.Errorf("no passkey ceremony in progress: %w", httpErrBadRequest)
	}
	http.SetCookie(w, s.cookie(signinCeremony, "", "/auth/passkey/", -1))
	s.mu.Lock()
	cer, ok := s.ceremonies[c.Value]
	delete(s.ceremonies, c.Value)
	s.mu.Unlock()
	if !ok || time.Now().After(cer.exp) {
		return cer, fmt.Errorf("passkey ceremony expired: %w", httpErrBadRequest)
	}
	return cer, nil
}

func signinMultiTenant(issuer string) bool {
	for _, t := range []string{"common", "organizations", "consumers"} {
		if strings.HasPrefix(issuer, "https://login.microsoftonline.com/"+t+"/") {
			return true
		}
	}
	return false
}

func (s *signin) provider(name string) (*signinProvider, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p := s.providers[name]; p != nil {
		return p, nil
	}
	c, ok := s.oidc[name]
	if !ok {
		return nil, fmt.Errorf("sign-in provider %q: %w", name, ErrNotFound)
	}
	ctx, multi := context.Background(), signinMultiTenant(c.Issuer)
	if multi {
		ctx = oidc.InsecureIssuerURLContext(ctx, c.Issuer)
	}
	op, err := oidc.NewProvider(ctx, c.Issuer)
	if err != nil {
		return nil, fmt.Errorf("sign-in provider %s: %w", name, err)
	}
	p := &signinProvider{multi: multi, verifier: op.Verifier(&oidc.Config{ClientID: c.ClientID, SkipIssuerCheck: multi}),
		cfg: oauth2.Config{ClientID: c.ClientID, ClientSecret: c.ClientSecret, Endpoint: op.Endpoint(),
			RedirectURL: s.base.String() + "/auth/oidc/" + name + "/callback", Scopes: []string{oidc.ScopeOpenID, "email", "profile"}}}
	s.providers[name] = p
	return p, nil
}

func (s *signin) oidcCallback(w http.ResponseWriter, r *http.Request) (*User, error) {
	p, err := s.provider(r.PathValue("provider"))
	if err != nil {
		return nil, err
	}
	c, err := r.Cookie(signinOIDC)
	http.SetCookie(w, s.cookie(signinOIDC, "", "/auth/oidc/", -1))
	parts := []string{}
	if err == nil {
		parts = strings.Split(c.Value, ".")
	}
	if len(parts) != 3 || r.URL.Query().Get("state") != parts[0] {
		return nil, fmt.Errorf("sign-in state does not match; start again: %w", ErrUnauthorized)
	}
	tok, err := p.cfg.Exchange(r.Context(), r.URL.Query().Get("code"), oauth2.VerifierOption(parts[2]))
	if err != nil {
		return nil, fmt.Errorf("sign-in: %v: %w", err, ErrUnauthorized)
	}
	raw, _ := tok.Extra("id_token").(string)
	idt, err := p.verifier.Verify(r.Context(), raw)
	if err != nil || idt.Nonce != parts[1] {
		return nil, fmt.Errorf("sign-in: invalid ID token: %w", ErrUnauthorized)
	}
	if p.multi && (!strings.HasPrefix(idt.Issuer, "https://login.microsoftonline.com/") || !strings.HasSuffix(idt.Issuer, "/v2.0")) {
		return nil, fmt.Errorf("sign-in: unexpected issuer %s: %w", idt.Issuer, ErrUnauthorized)
	}
	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		DomainOwner   bool   `json:"xms_edov"`
	}
	if err := idt.Claims(&claims); err != nil {
		return nil, err
	}
	link := signinOIDCLink{idt.Issuer, idt.Subject}
	lb, _ := json.Marshal([]signinOIDCLink{link})
	u, err := userBy(r.Context(), s.a, (*generated.Queries).UserByOIDC, lb)
	if errors.Is(err, ErrNotFound) && claims.Email != "" && (claims.EmailVerified || claims.DomainOwner) {
		if email, e := authEmail(claims.Email); e == nil {
			u, err = userBy(r.Context(), s.a, (*generated.Queries).UserByEmail, email)
		}
		if err == nil {
			pu, err := s.user(r.Context(), u.ID)
			if err != nil {
				return nil, err
			}
			if !slices.Contains(pu.m.OIDC, link) {
				pu.m.OIDC = append(pu.m.OIDC, link)
			}
			return u, s.save(r.Context(), pu)
		}
	}
	if errors.Is(err, ErrNotFound) {
		return nil, fmt.Errorf("no Archive account for this sign-in; ask an admin to add you: %w", ErrForbidden)
	}
	return u, err
}

var secretMasks = map[string]bool{"card": true, "mask": true, "mask_ssn": true, "national": true, "passport": true}

func secretLoadKey(dir string) ([]byte, error) {
	if s := strings.TrimSpace(os.Getenv("ARCHIVE_SECRET_KEY")); s != "" {
		for _, dec := range []func(string) ([]byte, error){hex.DecodeString, base64.StdEncoding.DecodeString, base64.RawURLEncoding.DecodeString} {
			if k, err := dec(s); err == nil && len(k) == 32 {
				return k, nil
			}
		}
		return nil, errors.New("ARCHIVE_SECRET_KEY must be 32 bytes, hex or base64")
	}
	path := filepath.Join(dir, "secret.key")
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, err
		}
		key := make([]byte, 32)
		rand.Read(key)
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			_, err = f.WriteString(hex.EncodeToString(key) + "\n")
			if cerr := f.Close(); err == nil {
				err = cerr
			}
			return key, err
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, err
		}
		b, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, err
	}
	k, err := hex.DecodeString(strings.TrimSpace(string(b)))
	if err != nil || len(k) != 32 {
		return nil, fmt.Errorf("%s must hold 32 hex-encoded bytes", path)
	}
	return k, nil
}

func secretAEAD(key []byte) (cipher.AEAD, error) {
	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(c)
}

func secretDerive(key []byte, purpose string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(purpose))
	return m.Sum(nil)
}

func authPassword(key []byte) string { return hex.EncodeToString(secretDerive(key, "authenticator")) }

func secretMask(s *semType, v any) string {
	if secretMasks[s.Format] {
		return semFormat(s, v, "")
	}
	return "****"
}

func (a *Archive) secretBox(id uuid.UUID, typ string, plain []byte) []byte {
	nonce := make([]byte, a.aead.NonceSize())
	rand.Read(nonce)
	return a.aead.Seal(nonce, nonce, plain, []byte(id.String()+typ))
}

func (a *Archive) secretOpen(id uuid.UUID, typ string, ct []byte) ([]byte, error) {
	n := a.aead.NonceSize()
	if len(ct) < n {
		return nil, fmt.Errorf("secret %s is corrupt", id)
	}
	b, err := a.aead.Open(nil, ct[:n], ct[n:], []byte(id.String()+typ))
	if err != nil {
		return nil, fmt.Errorf("secret %s: %w", id, err)
	}
	return b, nil
}

func (a *Archive) secretsPut(ctx context.Context, tb *Table, row map[string]any, seal map[string]any) error {
	id, space := coreRowID(row), coreUUID(row, "space")
	cat := a.Catalog()
	for _, col := range specKeys(seal) {
		q := a.st.db(ctx)
		if err := q.ClearSecrets(ctx, tb.Name, id, &col); err != nil {
			return err
		}
		if seal[col] == nil {
			continue
		}
		b, err := json.Marshal(seal[col])
		if err != nil {
			return err
		}
		sid, typ := coreNewID(), tb.col(col).Type
		if cat.secret(tb.col(col)) == nil {
			return fmt.Errorf("%s.%s is not a secret column", tb.Name, col)
		}
		if err := q.CreateSecret(ctx, generated.CreateSecretParams{ID: sid, TableName: tb.Name, RowID: id, ColumnName: col, Type: typ, Value: a.secretBox(sid, typ, b), Space: space}); err != nil {
			return err
		}
	}
	return nil
}

func (a *Archive) Reveal(ctx context.Context, typ string, rows ...map[string]any) error {
	if len(rows) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(rows))
	byID := map[uuid.UUID]map[string]any{}
	for i, r := range rows {
		ids[i] = coreRowID(r)
		byID[ids[i]] = r
	}
	return a.as(ctx, func(ctx context.Context) error {
		rows, err := a.st.db(ctx).Secrets(ctx, typ, ids)
		for _, r := range rows {
			b, err := a.secretOpen(r.ID, r.Type, r.Value)
			if err != nil {
				return err
			}
			var v any
			if err := json.Unmarshal(b, &v); err != nil {
				return err
			}
			byID[r.RowID][r.ColumnName] = v
		}
		return err
	})
}
