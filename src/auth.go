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
	ID         uuid.UUID       `json:"id"`
	Email      string          `json:"email"`
	Name       string          `json:"name"`
	Workspaces []uuid.UUID     `json:"-"`
	Signin     json.RawMessage `json:"-"`
	CreatedAt  time.Time       `json:"created_at"`
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
	return a.prefix + "_" + kind + base64.RawURLEncoding.EncodeToString(id[:])
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

func (a *Archive) createSpace(ctx context.Context, name string, home *uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	if !authSpaceRe.MatchString(name) {
		return id, &ValidationError{Type: "space", Errors: []FieldError{{Path: "name", Message: "want 1-32 lowercase letters, digits or _"}}}
	}
	b := make([]byte, 9)
	rand.Read(b)
	base := a.prefix + "_s" + base64.RawURLEncoding.EncodeToString(b)
	id, err := a.st.db(ctx).CreateSpace(ctx, name, base, home)
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

func (a *Archive) AddUser(ctx context.Context, email, name string, admin bool) (*User, error) {
	email, err := authEmail(email)
	if err != nil {
		return nil, err
	}
	hq := a.hub.q()
	var u *User
	err = a.sys(ctx, func(ctx context.Context) error {
		q := a.st.db(ctx)
		if err := q.AdvisoryXactLock(ctx, 7273); err != nil {
			return err
		}
		first, err := hq.NoMembers(ctx, a.ws.ID)
		if err != nil {
			return err
		}
		if !first {
			ac, err := authUser(ctx)
			if err != nil {
				return err
			}
			if !a.has(ctx, ac.UserRole, a.prefix+"_admin") {
				return fmt.Errorf("only admins of this workspace add people: %w", ErrForbidden)
			}
		}
		r, err := hq.JoinWorkspace(ctx, email, strings.TrimSpace(name), a.ws.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%s is in this workspace already: %w", email, ErrConflict)
		}
		if err != nil {
			return err
		}
		u = (*User)(&r)
		var bases []string
		if first {
			spaces, _ := q.SpaceRoles(ctx)
			for _, sp := range spaces {
				bases = append(bases, sp.Role)
			}
		}
		home, err := q.HomeSpace(ctx, &u.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			home, err = a.createSpace(ctx, a.freeSpaceName(ctx, email), &u.ID)
		}
		if err != nil {
			return err
		}
		role := a.roleID("u", u.ID)
		if err := a.createRole(ctx, role); err != nil {
			return err
		}
		homeRole, err := q.SpaceRoleByID(ctx, home)
		if err != nil {
			return err
		}
		for _, base := range append(bases, homeRole) {
			if err := a.setLevel(ctx, role, base, authLevel("owner")); err != nil {
				return err
			}
		}
		if admin || first {
			return a.exec(ctx, authGrant(a.prefix+"_admin", role))
		}
		return nil
	})
	if err != nil && u != nil {
		hq.LeaveWorkspace(context.WithoutCancel(ctx), a.ws.ID, u.ID)
	}
	return u, err
}

func (a *Archive) RemoveUser(ctx context.Context, email string) error {
	ac, err := authUser(ctx)
	if err != nil {
		return err
	}
	u, err := a.member(ctx, email)
	if err != nil {
		return err
	}
	hq, role := a.hub.q(), a.roleID("u", u.ID)
	return a.sys(ctx, func(ctx context.Context) error {
		q := a.st.db(ctx)
		if !a.has(ctx, ac.UserRole, a.prefix+"_admin") {
			return fmt.Errorf("only admins of this workspace remove people: %w", ErrForbidden)
		}
		spaces, err := q.SpaceRoles(ctx)
		if err != nil {
			return err
		}
		for _, sp := range spaces {
			if (sp.Home == nil || *sp.Home != u.ID) && a.level(ctx, role, sp.Role, true) == authLevel("owner") {
				if owners, _ := q.CountOwners(ctx, sp.Role+"_owner", a.prefix+"_u"); owners <= 1 {
					return fmt.Errorf("%s is the last owner of %s: %w", u.Email, sp.Name, ErrConflict)
				}
			}
		}
		roles, err := hq.DeleteWorkspaceTokens(ctx, u.ID, &a.ws.ID)
		if err != nil {
			return err
		}
		for _, r := range append(roles, role) {
			if r != "" {
				if err := a.exec(ctx, authDrop(r)); err != nil {
					return err
				}
			}
		}
		if err := q.ForgetUser(ctx, u.ID); err != nil {
			return err
		}
		return hq.LeaveWorkspace(ctx, a.ws.ID, u.ID)
	})
}

func userBy[T any](ctx context.Context, h *Hub, get func(*generated.Queries, context.Context, T) (generated.ArchiveUser, error), arg T) (*User, error) {
	r, err := get(h.q(), ctx, arg)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return (*User)(&r), err
}

func (a *Archive) member(ctx context.Context, email string) (*User, error) {
	email, err := authEmail(email)
	if err != nil {
		return nil, err
	}
	r, err := a.hub.q().WorkspaceUser(ctx, email, a.ws.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("user %s: %w", email, ErrNotFound)
	}
	return (*User)(&r), err
}

func (a *Archive) userActor(u *User) *Actor {
	role := a.roleID("u", u.ID)
	return &Actor{UserID: u.ID, Email: u.Email, Name: u.Name, Role: role, UserRole: role}
}

func (a *Archive) CLIActor(ctx context.Context, email string) (*Actor, error) {
	if email == "" {
		u, err := userBy(ctx, a.hub, (*generated.Queries).FirstUser, a.ws.ID)
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return a.userActor(u), nil
	}
	u, err := a.member(ctx, email)
	if err != nil {
		return nil, err
	}
	return a.userActor(u), nil
}

func (h *Hub) newToken(ctx context.Context, userID uuid.UUID, kind string, ttl time.Duration) (string, error) {
	secret := authNewSecret()
	return secret, h.q().CreateLoginToken(ctx, generated.CreateLoginTokenParams{UserID: userID, Kind: kind, Hash: authHash(secret), Ttl: ttl.String()})
}

func (h *Hub) useMagicLink(ctx context.Context, secret string) (*User, error) {
	id, err := h.q().UseMagicLink(ctx, authHash(secret))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("sign-in link is invalid, used or expired: %w", ErrUnauthorized)
	}
	if err != nil {
		return nil, err
	}
	return userBy(ctx, h, (*generated.Queries).UserByID, id)
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
		if sp.ID, err = a.createSpace(ctx, name, nil); err != nil {
			return err
		}
		base, err := a.st.db(ctx).SpaceRoleByID(ctx, sp.ID)
		if err != nil {
			return err
		}
		return a.setLevel(ctx, ac.UserRole, base, authLevel("owner"))
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
		if sp.Home != nil {
			return fmt.Errorf("%s is a personal space: %w", name, ErrConflict)
		}
		if err := q.DeleteSpaceSecrets(ctx, id); err != nil {
			return err
		}
		if err := a.exec(ctx, "GRANT "+authIdent(ac.UserRole)+" TO CURRENT_USER", "SET LOCAL ROLE "+authIdent(ac.UserRole)); err != nil {
			return err
		}
		for _, t := range a.Catalog().deleteOrder() {
			if _, err := a.st.Q(ctx).Exec(ctx, `DELETE FROM `+pgIdent("public", t)+` WHERE space = $1`, id); err != nil {
				return err
			}
		}
		if err := a.exec(ctx, "RESET ROLE", "REVOKE "+authIdent(ac.UserRole)+" FROM CURRENT_USER"); err != nil {
			return err
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
		users, err := a.hub.q().WorkspaceUsers(ctx, a.ws.ID)
		for _, r := range users {
			if l := a.level(ctx, a.roleID("u", r.ID), base, true); l > 0 {
				out = append(out, Member{Email: r.Email, Name: r.Name, Level: authLevels[l-1]})
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
	u, err := a.member(ctx, email)
	if err != nil {
		return err
	}
	return a.sys(ctx, func(ctx context.Context) error {
		q := a.st.db(ctx)
		base, err := a.spaceRole(ctx, space)
		if err != nil {
			return err
		}
		role := a.roleID("u", u.ID)
		cur := a.level(ctx, role, base, true)
		need := "admin"
		if max(lvl, cur) == authLevel("owner") {
			need = "owner"
		}
		if !a.has(ctx, ac.UserRole, base+"_"+need) {
			return fmt.Errorf("changing this membership needs %s in %s: %w", need, space, ErrForbidden)
		}
		if cur == authLevel("owner") && lvl != cur {
			if owners, _ := q.CountOwners(ctx, base+"_owner", a.prefix+"_u"); owners <= 1 {
				return fmt.Errorf("%s is the last owner of %s: %w", u.Email, space, ErrConflict)
			}
		}
		if err := a.setLevel(ctx, role, base, lvl); err != nil {
			return err
		}
		troles, err := a.hub.q().TokenRoles(ctx, u.ID, &a.ws.ID)
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
	hq := a.hub.q()
	return t, a.sys(ctx, func(ctx context.Context) error {
		if full {
			r, err := hq.CreateUserToken(ctx, generated.CreateUserTokenParams{ID: t.ID, UserID: ac.UserID, Workspace: &a.ws.ID, Name: t.Name, Hash: authHash(t.Secret), ExpiresAt: exp})
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
		if err := a.createRole(ctx, role); err != nil {
			return err
		}
		for n, lv := range t.Spaces {
			if err := a.setLevel(ctx, role, all[n], authLevel(lv)); err != nil {
				return err
			}
		}
		r, err := hq.CreateAPIToken(ctx, generated.CreateAPITokenParams{ID: t.ID, UserID: ac.UserID, Workspace: &a.ws.ID, Name: t.Name, Role: &role, Hash: authHash(t.Secret), ExpiresAt: exp})
		t.CreatedAt, t.ExpiresAt = r.CreatedAt, r.ExpiresAt
		return err
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
		rows, err := a.hub.q().Tokens(ctx, ac.UserID, &a.ws.ID)
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
		role, err := a.hub.q().RevokeToken(ctx, id, ac.UserID, &a.ws.ID)
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
	u, err := a.member(ctx, email)
	if err != nil {
		return err
	}
	return a.sys(ctx, func(ctx context.Context) error {
		if !a.has(ctx, ac.UserRole, a.prefix+"_admin") {
			return fmt.Errorf("only admins of this workspace grant secret types: %w", ErrForbidden)
		}
		role, group := a.roleID("u", u.ID), a.prefix+"_secret_"+typ
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
	signinTTL      = 15 * time.Minute
)

type signin struct {
	h      *Hub
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

type signinState struct {
	Nonce    string `json:"n"`
	Verifier string `json:"v"`
	Bind     string `json:"b"`
	Exp      int64  `json:"x"`
}

type signinUser struct {
	u *User
	m signinMethods
}

func (p *signinUser) WebAuthnID() []byte                         { return p.u.ID[:] }
func (p *signinUser) WebAuthnName() string                       { return p.u.Email }
func (p *signinUser) WebAuthnDisplayName() string                { return cmp.Or(p.u.Name, p.u.Email) }
func (p *signinUser) WebAuthnCredentials() []webauthn.Credential { return p.m.Passkeys }

func newSignin(h *Hub) (*signin, error) {
	u, err := url.Parse(h.app)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("ARCHIVE_URL %q is not a URL", h.app)
	}
	host := u.Hostname()
	s := &signin{h: h, secure: host != "localhost" && host != "127.0.0.1" && host != "::1",
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

func (s *signin) handler() http.Handler {
	h, mux := s.h, http.NewServeMux()
	open := func(pattern string, f func(w http.ResponseWriter, r *http.Request) (int, any, error)) {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			status, v, err := f(w, r)
			httpWrite(w, status, v, err)
		})
	}
	authed := func(pattern string, f func(w http.ResponseWriter, r *http.Request) (int, any, error)) {
		mux.Handle(pattern, h.httpAuth(func(*Archive) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				status, v, err := f(w, r)
				httpWrite(w, status, v, err)
			})
		}))
	}
	ok := func(v any, err error) (int, any, error) { return http.StatusOK, v, err }

	open("POST /auth/magic", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		var body struct {
			Email string `json:"email"`
		}
		if err := httpDecode(r, &body); err != nil {
			return 0, nil, err
		}
		email, err := authEmail(body.Email)
		if err != nil {
			return 0, nil, err
		}
		u, err := userBy(r.Context(), h, (*generated.Queries).UserByEmail, email)
		if err == nil {
			var secret string
			if secret, err = h.newToken(r.Context(), u.ID, "magic", signinTTL); err == nil {
				err = s.mail(email, "sign-in", "Sign in to Archive", "Open this link within 15 minutes to sign in to Archive:", h.app+"/auth/magic?token="+url.QueryEscape(secret))
			}
		}
		if err != nil && !errors.Is(err, ErrNotFound) {
			return 0, nil, err
		}
		return http.StatusAccepted, map[string]string{"status": "if that address has an account, a sign-in link is on its way"}, nil
	})
	mux.HandleFunc("GET /auth/magic", func(w http.ResponseWriter, r *http.Request) {
		s.browser(w, r, func() (*User, string, error) {
			u, err := h.useMagicLink(r.Context(), r.URL.Query().Get("token"))
			return u, "", err
		})
	})
	open("GET /auth/methods", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		providers := append([]string{}, slices.Sorted(maps.Keys(s.oidc))...)
		return ok(map[string]any{"providers": providers, "mail": os.Getenv("ARCHIVE_SMTP_URL") != "", "signup": h.signupOpen(r.Context())}, nil)
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
		a, release, err := h.acquire(r.Context(), h.one)
		if err != nil {
			return 0, nil, err
		}
		defer release()
		u, err := a.AddUser(r.Context(), body.Email, body.Name, true)
		if err != nil {
			s.mu.Lock()
			s.setup = body.Token
			s.mu.Unlock()
			return 0, nil, err
		}
		return ok(s.signedIn(w, r, u, h.one))
	})
	open("POST /auth/signup", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		var body struct {
			Email string `json:"email"`
			Name  string `json:"name"`
		}
		if err := httpDecode(r, &body); err != nil {
			return 0, nil, err
		}
		if !h.signupOpen(r.Context()) {
			return 0, nil, fmt.Errorf("signup is closed on this Archive; ask an admin for a workspace: %w", ErrForbidden)
		}
		email, err := authEmail(body.Email)
		if err != nil {
			return 0, nil, err
		}
		name, err := workspaceName(body.Name)
		if err != nil {
			return 0, nil, err
		}
		link := h.app + "/auth/signup?token=" + url.QueryEscape(h.signupToken(email, name))
		if err := s.mail(email, "signup", "Create your Archive workspace", "Open this link within 15 minutes to create "+name+":", link); err != nil {
			return 0, nil, err
		}
		return http.StatusAccepted, map[string]string{"status": "a link to create the workspace is on its way"}, nil
	})
	mux.HandleFunc("GET /auth/signup", func(w http.ResponseWriter, r *http.Request) {
		s.browser(w, r, func() (*User, string, error) {
			c, err := h.signupClaim(r.URL.Query().Get("token"))
			if err != nil {
				return nil, "", err
			}
			u, err := h.signup(r.Context(), c)
			return u, c.ID, err
		})
	})
	open("POST /auth/signout", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		s.setSession(w, "")
		if c, err := r.Cookie(signinCookie); err == nil {
			return http.StatusNoContent, nil, h.q().SignOut(r.Context(), authHash(c.Value))
		}
		return http.StatusNoContent, nil, nil
	})
	authed("GET /auth/me", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		a, ac := archiveFrom(r.Context()), ActorFrom(r.Context())
		sp, err := a.Spaces(r.Context())
		if err != nil {
			return 0, nil, err
		}
		var home uuid.UUID
		a.sys(r.Context(), func(ctx context.Context) error {
			home, _ = a.st.db(ctx).HomeSpace(ctx, &ac.UserID)
			return nil
		})
		rows, err := h.q().UserWorkspaces(r.Context(), ac.UserID)
		all := []Workspace{}
		for _, x := range rows {
			if r.Header.Get("Authorization") == "" || x.ID == a.ws.ID {
				all = append(all, Workspace{ID: x.ID.String(), Name: x.Name})
			}
		}
		return ok(map[string]any{"actor": ac, "spaces": sp, "home": home, "workspace": a.ws.ID.String(), "workspaces": all}, err)
	})
	authed("GET /auth/spaces", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		return ok(archiveFrom(r.Context()).Spaces(r.Context()))
	})
	authed("POST /auth/spaces", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		var body struct {
			Name string `json:"name"`
		}
		if err := httpDecode(r, &body); err != nil {
			return 0, nil, err
		}
		sp, err := archiveFrom(r.Context()).AddSpace(r.Context(), body.Name)
		return http.StatusCreated, sp, err
	})
	authed("DELETE /auth/spaces/{space}", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		return http.StatusNoContent, nil, archiveFrom(r.Context()).DeleteSpace(r.Context(), r.PathValue("space"))
	})
	authed("GET /auth/spaces/{space}/members", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		return ok(archiveFrom(r.Context()).Members(r.Context(), r.PathValue("space")))
	})
	authed("POST /auth/spaces/{space}/members", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		var body struct {
			Email string `json:"email"`
			Role  string `json:"role"`
		}
		if err := httpDecode(r, &body); err != nil {
			return 0, nil, err
		}
		a := archiveFrom(r.Context())
		if err := a.SetMember(r.Context(), r.PathValue("space"), body.Email, body.Role); err != nil {
			return 0, nil, err
		}
		return ok(a.Members(r.Context(), r.PathValue("space")))
	})
	authed("GET /auth/tokens", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		return ok(archiveFrom(r.Context()).Tokens(r.Context()))
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
		t, err := archiveFrom(r.Context()).CreateToken(r.Context(), body.Name, body.Spaces, ttl, body.Full)
		return http.StatusCreated, t, err
	})
	authed("DELETE /auth/tokens/{id}", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		id, err := httpID(r)
		if err != nil {
			return 0, nil, err
		}
		return http.StatusNoContent, nil, archiveFrom(r.Context()).RevokeToken(r.Context(), id)
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
		return ok(s.signedIn(w, r, found.u, ""))
	})

	mux.HandleFunc("GET /auth/oidc/{provider}", func(w http.ResponseWriter, r *http.Request) {
		p, err := s.provider(r.PathValue("provider"))
		if err != nil {
			httpWrite(w, 0, nil, err)
			return
		}
		bind := authNewSecret()
		st := signinState{Nonce: authNewSecret(), Verifier: oauth2.GenerateVerifier(), Bind: authHash(bind), Exp: time.Now().Add(10 * time.Minute).Unix()}
		http.SetCookie(w, s.cookie(signinOIDC, bind, "/auth/oidc/", 600))
		http.Redirect(w, r, p.cfg.AuthCodeURL(s.seal(st), oidc.Nonce(st.Nonce), oauth2.S256ChallengeOption(st.Verifier)), http.StatusFound)
	})
	mux.HandleFunc("GET /auth/oidc/{provider}/callback", func(w http.ResponseWriter, r *http.Request) {
		st, token, err := s.oidcCallback(r)
		if err != nil {
			httpWrite(w, 0, nil, err)
			return
		}
		q := url.Values{"b": {st.Bind}, "token": {token}, "sig": {h.sign("oidc", st.Bind+"."+token)}}
		http.Redirect(w, r, h.app+"/auth/oidc/done?"+q.Encode(), http.StatusSeeOther)
	})
	mux.HandleFunc("GET /auth/oidc/done", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		c, err := r.Cookie(signinOIDC)
		http.SetCookie(w, s.cookie(signinOIDC, "", "/auth/oidc/", -1))
		b, tok := q.Get("b"), q.Get("token")
		if err != nil || tok == "" || authHash(c.Value) != b || !hmac.Equal([]byte(q.Get("sig")), []byte(h.sign("oidc", b+"."+tok))) {
			httpWrite(w, 0, nil, fmt.Errorf("sign-in state does not match; start again: %w", ErrUnauthorized))
			return
		}
		s.browser(w, r, func() (*User, string, error) {
			u, err := h.useMagicLink(r.Context(), tok)
			return u, "", err
		})
	})
	h.workspaceRoutes(authed)
	return mux
}

func (s *signin) browser(w http.ResponseWriter, r *http.Request, user func() (*User, string, error)) {
	u, short, err := user()
	var v any
	if err == nil {
		v, err = s.signedIn(w, r, u, short)
	}
	if err == nil && strings.Contains(r.Header.Get("Accept"), "text/html") {
		to := "/"
		if short != "" {
			to += "?workspace=" + url.QueryEscape(short)
		}
		http.Redirect(w, r, to, http.StatusSeeOther)
		return
	}
	httpWrite(w, http.StatusOK, v, err)
}

func (s *signin) cookie(name, value, path string, maxAge int) *http.Cookie {
	return &http.Cookie{Name: name, Value: value, Path: path, MaxAge: maxAge, HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteLaxMode}
}

func (s *signin) setSession(w http.ResponseWriter, secret string) {
	age := 30 * 24 * 3600
	if secret == "" {
		age = -1
	}
	http.SetCookie(w, s.cookie(signinCookie, secret, "/", age))
}

func (s *signin) signedIn(w http.ResponseWriter, r *http.Request, u *User, short string) (any, error) {
	secret, err := s.h.newToken(r.Context(), u.ID, "session", 30*24*time.Hour)
	if err != nil {
		return nil, err
	}
	s.setSession(w, secret)
	return struct {
		User      *User  `json:"user"`
		Workspace string `json:"workspace,omitempty"`
	}{u, short}, nil
}

func (s *signin) mail(to, kind, subject, intro, link string) error {
	raw := os.Getenv("ARCHIVE_SMTP_URL")
	if raw == "" {
		fmt.Fprintf(os.Stderr, "archive: %s link for %s: %s\n", kind, to, link)
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
	host := s.h.app
	if p, err := url.Parse(s.h.app); err == nil {
		host = p.Hostname()
	}
	from := coreEnv("ARCHIVE_SMTP_FROM", "archive@"+host)
	msg := "From: " + from + "\r\nTo: " + to + "\r\nSubject: " + subject + "\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n" + intro + "\r\n\r\n" + link + "\r\n"
	return smtp.SendMail(u.Hostname()+":"+cmp.Or(u.Port(), "587"), auth, from, []string{to}, []byte(msg))
}

func (s *signin) user(ctx context.Context, id uuid.UUID) (*signinUser, error) {
	u, err := userBy(ctx, s.h, (*generated.Queries).UserByID, id)
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
	return s.h.q().SetSignin(ctx, pu.u.ID, b)
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
			RedirectURL: s.h.api + "/auth/oidc/" + name + "/callback", Scopes: []string{oidc.ScopeOpenID, "email", "profile"}}}
	s.providers[name] = p
	return p, nil
}

func (s *signin) seal(st signinState) string {
	b, _ := json.Marshal(st)
	nonce := make([]byte, s.h.aead.NonceSize())
	rand.Read(nonce)
	return base64.RawURLEncoding.EncodeToString(s.h.aead.Seal(nonce, nonce, b, []byte("oidc")))
}

func (s *signin) unseal(state string) (*signinState, error) {
	bad := fmt.Errorf("sign-in state does not match; start again: %w", ErrUnauthorized)
	ct, err := base64.RawURLEncoding.DecodeString(state)
	n := s.h.aead.NonceSize()
	if err != nil || len(ct) < n {
		return nil, bad
	}
	b, err := s.h.aead.Open(nil, ct[:n], ct[n:], []byte("oidc"))
	var st signinState
	if err != nil || json.Unmarshal(b, &st) != nil || time.Now().Unix() > st.Exp {
		return nil, bad
	}
	return &st, nil
}

func (s *signin) oidcCallback(r *http.Request) (*signinState, string, error) {
	ctx := r.Context()
	p, err := s.provider(r.PathValue("provider"))
	if err != nil {
		return nil, "", err
	}
	st, err := s.unseal(r.URL.Query().Get("state"))
	if err != nil {
		return nil, "", err
	}
	tok, err := p.cfg.Exchange(ctx, r.URL.Query().Get("code"), oauth2.VerifierOption(st.Verifier))
	if err != nil {
		return nil, "", fmt.Errorf("sign-in: %v: %w", err, ErrUnauthorized)
	}
	raw, _ := tok.Extra("id_token").(string)
	idt, err := p.verifier.Verify(ctx, raw)
	if err != nil || idt.Nonce != st.Nonce {
		return nil, "", fmt.Errorf("sign-in: invalid ID token: %w", ErrUnauthorized)
	}
	if p.multi && (!strings.HasPrefix(idt.Issuer, "https://login.microsoftonline.com/") || !strings.HasSuffix(idt.Issuer, "/v2.0")) {
		return nil, "", fmt.Errorf("sign-in: unexpected issuer %s: %w", idt.Issuer, ErrUnauthorized)
	}
	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		DomainOwner   bool   `json:"xms_edov"`
	}
	if err := idt.Claims(&claims); err != nil {
		return nil, "", err
	}
	email := ""
	if claims.EmailVerified || claims.DomainOwner {
		email, _ = authEmail(claims.Email)
	}
	link := signinOIDCLink{idt.Issuer, idt.Subject}
	lb, _ := json.Marshal([]signinOIDCLink{link})
	u, err := userBy(ctx, s.h, (*generated.Queries).UserByOIDC, lb)
	if errors.Is(err, ErrNotFound) && email != "" {
		if u, err = userBy(ctx, s.h, (*generated.Queries).UserByEmail, email); err == nil {
			var pu *signinUser
			if pu, err = s.user(ctx, u.ID); err == nil {
				pu.m.OIDC = append(pu.m.OIDC, link)
				err = s.save(ctx, pu)
			}
		}
	}
	if errors.Is(err, ErrNotFound) {
		return nil, "", fmt.Errorf("no Archive account for this sign-in; ask an admin to add you: %w", ErrForbidden)
	}
	if err != nil {
		return nil, "", err
	}
	secret, err := s.h.newToken(ctx, u.ID, "magic", signinTTL)
	return st, secret, err
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
