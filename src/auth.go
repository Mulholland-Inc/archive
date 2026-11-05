package main

import (
	"cmp"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
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
	"golang.org/x/oauth2/github"
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
	member    uuid.UUID
	scoped    bool
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
	Admin      bool            `json:"admin"`
	Unread     []uuid.UUID     `json:"-"`
	Totp       []byte          `json:"-"`
	TotpStep   int64           `json:"-"`
	TotpFails  int32           `json:"-"`
	TotpUntil  *time.Time      `json:"-"`
}

type Token struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	Viewer    bool       `json:"viewer,omitempty"`
	App       bool       `json:"app,omitempty"`
	Workspace string     `json:"workspace,omitempty"`
	Where     string     `json:"workspace_name,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	Secret    string     `json:"token,omitempty"`
}

func authIdent(s string) string { return pgx.Identifier{s}.Sanitize() }

func authCreate(role string) string { return "CREATE ROLE " + authIdent(role) + " NOLOGIN" }

func authDrop(role string) string { return "DROP ROLE IF EXISTS " + authIdent(role) }

func authGrant(group, role string) string {
	return "GRANT " + authIdent(group) + " TO " + authIdent(role)
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
	var ok bool
	a.sys(ctx, func(ctx context.Context) error {
		r, err := a.st.db(ctx).HasRole(ctx, role, group)
		ok = err == nil && r
		return nil
	})
	return ok
}

func signedIn(ctx context.Context) (*Actor, error) {
	if ac := ActorFrom(ctx); ac != nil {
		return ac, nil
	}
	return nil, fmt.Errorf("not signed in: %w", ErrUnauthorized)
}

func authUser(ctx context.Context) (*Actor, error) {
	ac, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	if ac.Role != ac.UserRole {
		return nil, fmt.Errorf("a viewer-only token cannot manage tokens or workspaces: %w", ErrForbidden)
	}
	return ac, nil
}

func (a *Archive) memberOf(ctx context.Context, email string) (generated.MemberByEmailRow, error) {
	q := generated.New(a.st.sys)
	if t, ok := ctx.Value(storeTxKey{}).(storeTx); ok && t.role == "" {
		q = generated.New(t.tx)
	}
	m, err := q.MemberByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return m, fmt.Errorf("you are not in this workspace: %w", ErrForbidden)
	}
	return m, storeErr(err)
}

func (a *Archive) AddUser(ctx context.Context, email, name, role string) (*User, error) {
	email, err := authEmail(email)
	if err != nil {
		return nil, err
	}
	data := map[string]any{"email": email, "role": cmp.Or(role, "viewer")}
	var first bool
	err = a.sys(ctx, func(ctx context.Context) error {
		q := a.st.db(ctx)
		if err := q.AdvisoryXactLock(ctx, 7273); err != nil {
			return err
		}
		if first, err = q.NoMembers(ctx); err != nil || !first {
			return err
		}
		tb, err := a.Catalog().table("member")
		if err != nil {
			return err
		}
		data["role"] = "owner"
		_, err = a.coreCreate(ctx, tb, data)
		return err
	})
	if err == nil && !first {
		_, err = a.Create(ctx, "member", data)
	}
	if err != nil {
		return nil, err
	}
	return a.joined(ctx, email, name, first)
}

func (a *Archive) joined(ctx context.Context, email, name string, first bool) (*User, error) {
	ctx = context.WithoutCancel(ctx)
	hq, name := a.hub.q(), strings.TrimSpace(name)
	if r, err := hq.JoinWorkspace(ctx, email, name, a.ws.ID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	} else if err == nil && first {
		if err := hq.AdminIfNone(ctx, r.ID); err != nil {
			return nil, err
		}
	}
	u, err := userBy(ctx, a.hub, (*generated.Queries).UserByEmail, email)
	if err == nil && name != "" && u.Name == "" {
		if err = hq.NameUser(ctx, u.ID, name); err == nil {
			u.Name = name
		}
	}
	if err == nil {
		a.hub.liveUser(ctx, u.ID)
	}
	return u, err
}

func (a *Archive) syncMembers(ctx context.Context, removals bool) error {
	ctx = context.WithValue(context.WithoutCancel(ctx), storeTxKey{}, nil)
	hq := a.hub.q()
	var ms []generated.MembersRow
	if err := a.st.System(ctx, func(ctx context.Context) (err error) {
		ms, err = a.st.db(ctx).Members(ctx)
		return err
	}); err != nil {
		return err
	}
	users, err := hq.WorkspaceUsers(ctx, a.ws.ID)
	if err != nil {
		return err
	}
	in, keep := map[string]bool{}, map[string]bool{}
	var emails, names []string
	for _, u := range users {
		in[u.Email] = true
		emails, names = append(emails, u.Email), append(names, u.Name)
	}
	if err := a.st.System(ctx, func(ctx context.Context) error {
		return a.st.db(ctx).NameMembers(ctx, emails, names)
	}); err != nil {
		return err
	}
	for _, m := range ms {
		keep[m.Email] = true
		if !in[m.Email] {
			name := m.Name
			if name == m.Email {
				name = ""
			}
			u, err := hq.JoinWorkspace(ctx, m.Email, name, a.ws.ID)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			if err == nil {
				a.hub.liveUser(ctx, u.ID)
			}
		}
	}
	for _, u := range users {
		if !removals || keep[u.Email] {
			continue
		}
		roles, err := hq.DeleteWorkspaceTokens(ctx, u.ID, &a.ws.ID)
		if err != nil {
			return err
		}
		if err := a.st.System(ctx, func(ctx context.Context) error {
			for _, r := range roles {
				if r != "" {
					if err := a.exec(ctx, authDrop(r)); err != nil {
						return err
					}
				}
			}
			return a.st.db(ctx).ForgetUser(ctx, u.ID)
		}); err != nil {
			return err
		}
		if err := hq.LeaveWorkspace(ctx, a.ws.ID, u.ID); err != nil {
			return err
		}
		a.hub.live.drop(a.ws.ID.String(), u.ID.String())
		a.hub.live.moved(a.ws.ID.String(), liveChange{Table: "archive.token"})
		a.hub.liveUser(ctx, u.ID)
	}
	return nil
}

func userBy[T any](ctx context.Context, h *Hub, get func(*generated.Queries, context.Context, T) (generated.ArchiveUser, error), arg T) (*User, error) {
	r, err := get(h.q(), ctx, arg)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return (*User)(&r), err
}

func (a *Archive) userActor(ctx context.Context, u *User) (*Actor, error) {
	m, err := a.memberOf(ctx, u.Email)
	if err != nil {
		return nil, err
	}
	role := a.roleID("u", m.ID)
	return &Actor{UserID: u.ID, Email: u.Email, Name: u.Name, Role: role, UserRole: role, member: m.ID}, nil
}

func (a *Archive) mayKey(ctx context.Context, email string) error {
	m, err := a.memberOf(ctx, email)
	if err == nil && m.Role != "engineer" && m.Role != "owner" {
		err = fmt.Errorf("only engineers and the owner have this workspace's API keys: %w", ErrForbidden)
	}
	return err
}

func (a *Archive) mayKeyOf(ctx context.Context, ac *Actor, id uuid.UUID) error {
	ws, err := a.hub.q().KeyWorkspace(ctx, id, ac.UserID)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && ws == nil {
		return nil
	}
	if err != nil {
		return err
	}
	if *ws == a.ws.ID {
		return a.mayKey(ctx, ac.Email)
	}
	other, release, err := a.hub.acquire(ctx, ws.String())
	if err != nil {
		return err
	}
	defer release()
	return other.mayKey(ctx, ac.Email)
}

func (a *Archive) CreateToken(ctx context.Context, name string, ttl time.Duration, viewer, personal bool) (*Token, error) {
	ac, err := authUser(ctx)
	if err != nil {
		return nil, err
	}
	if personal && viewer {
		return nil, fmt.Errorf("personal: a viewer-only token belongs to one workspace: %w", httpErrBadRequest)
	}
	if personal && ac.scoped {
		return nil, fmt.Errorf("a key for one workspace cannot make a personal key: %w", ErrForbidden)
	}
	if !personal {
		if err := a.mayKey(ctx, ac.Email); err != nil {
			return nil, err
		}
	}
	t := &Token{ID: coreNewID(), Name: strings.TrimSpace(name), Viewer: viewer, Secret: authNewSecret()}
	var exp *time.Time
	if ttl > 0 {
		e := time.Now().Add(ttl).UTC()
		exp = &e
	}
	return t, a.sys(ctx, func(ctx context.Context) error {
		var role *string
		ws := &a.ws.ID
		if personal {
			ws = nil
		}
		if viewer {
			r := a.roleID("t", t.ID)
			if err := a.exec(ctx, authCreate(r), authGrant(a.prefix+"_viewer", r), authGrant(a.roleID("m", ac.member), r)); err != nil {
				return err
			}
			role = &r
		}
		row, err := a.hub.q().CreateToken(ctx, generated.CreateTokenParams{ID: t.ID, UserID: ac.UserID, Workspace: ws, Name: t.Name, Role: role, Hash: authHash(t.Secret), Sealed: a.sealToken(t.ID, t.Secret), ExpiresAt: exp})
		t.CreatedAt, t.ExpiresAt = row.CreatedAt, row.ExpiresAt
		a.moved(ctx, "archive.token", t.ID.String())
		return err
	})
}

func (a *Archive) sealToken(id uuid.UUID, secret string) []byte {
	nonce := make([]byte, a.hub.aead.NonceSize())
	rand.Read(nonce)
	return a.hub.aead.Seal(nonce, nonce, []byte(secret), []byte("token "+id.String()))
}

func (a *Archive) TokenSecret(ctx context.Context, id uuid.UUID) (string, error) {
	ac, err := authUser(ctx)
	if err != nil {
		return "", err
	}
	if err := a.mayKeyOf(ctx, ac, id); err != nil {
		return "", err
	}
	sealed, err := a.hub.q().TokenSealed(ctx, generated.TokenSealedParams{ID: id, UserID: ac.UserID, Scoped: ac.scoped, Workspace: &a.ws.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	n := a.hub.aead.NonceSize()
	if len(sealed) < n {
		return "", ErrNotFound
	}
	plain, err := a.hub.aead.Open(nil, sealed[:n], sealed[n:], []byte("token "+id.String()))
	if err != nil {
		return "", fmt.Errorf("token %s does not open: %w", id, err)
	}
	return string(plain), nil
}

func (a *Archive) Tokens(ctx context.Context) ([]*Token, error) {
	ac, err := authUser(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := a.hub.q().Tokens(ctx, ac.UserID, ac.scoped, &a.ws.ID)
	out := []*Token{}
	for _, r := range rows {
		t := &Token{ID: r.ID, Name: r.Name, Viewer: r.Viewer, App: r.App, Workspace: r.Workspace, Where: r.WorkspaceName, ExpiresAt: r.ExpiresAt, CreatedAt: r.CreatedAt}
		if r.App {
			t.ExpiresAt = r.RefreshExpiresAt
		}
		out = append(out, t)
	}
	return out, err
}

func (a *Archive) RevokeToken(ctx context.Context, id uuid.UUID) error {
	ac, err := authUser(ctx)
	if err != nil {
		return err
	}
	if err := a.mayKeyOf(ctx, ac, id); err != nil {
		return err
	}
	return a.sys(ctx, func(ctx context.Context) error {
		role, err := a.hub.q().RevokeToken(ctx, generated.RevokeTokenParams{ID: id, UserID: ac.UserID, Scoped: ac.scoped, Workspace: &a.ws.ID})
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("token %s: %w", id, ErrNotFound)
		}
		if err == nil {
			a.moved(ctx, "archive.token", id.String())
		}
		if err != nil || role == "" {
			return err
		}
		return a.exec(ctx, authDrop(role))
	})
}

const (
	signinCookie   = "archive_session"
	signinCeremony = "archive_webauthn"
	signinOIDC     = "archive_oidc"
	signinTotp     = "archive_totp"
	signinTotpLink = "archive_totp_link"
	signinGitHub   = "https://github.com"
	signinApple    = "https://appleid.apple.com"
)

var (
	signinRefused  = fmt.Errorf("no Archive account for this sign-in; ask a workspace's owner to add you: %w", ErrForbidden)
	signinMismatch = fmt.Errorf("sign-in state does not match; start again: %w", ErrUnauthorized)
	totpWrong      = fmt.Errorf("Wrong code: %w", httpErrBadRequest)
	totpLocked     = fmt.Errorf("Too many tries. Try again later: %w", ErrBusy)
	signinBuiltin  = []signinConfig{
		{ID: "google", Name: "Google", base: "https://accounts.google.com"},
		{ID: "microsoft", Name: "Microsoft", base: "https://login.microsoftonline.com"},
		{ID: "slack", Name: "Slack", base: "https://slack.com"},
		{ID: "github", Name: "GitHub", base: signinGitHub},
		{ID: "apple", Name: "Apple", base: signinApple},
	}
)

type signin struct {
	h       *Hub
	secure  bool
	wa      *webauthn.WebAuthn
	configs []signinConfig

	mu        sync.Mutex
	providers map[string]*signinProvider
}

type signinTotpSetup struct {
	Secret []byte    `json:"s"`
	Exp    time.Time `json:"x"`
}

type signinCeremonyData struct {
	Data webauthn.SessionData `json:"data"`
	User uuid.UUID            `json:"user"`
	Exp  time.Time            `json:"exp"`
}

type signinConfig struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	base     string
	clientID string
	secret   string
}

type signinProvider struct {
	signinConfig
	cfg      oauth2.Config
	verifier *oidc.IDTokenVerifier
	apple    *signinAppleKey
}

// signinAppleKey signs the client secret Sign in with Apple wants instead of a fixed one: a JWT from the team's key, good for an hour.
type signinAppleKey struct {
	team, id string
	key      *ecdsa.PrivateKey
}

type signinMethods struct {
	Passkeys []webauthn.Credential `json:"passkeys,omitempty"`
	OIDC     []signinOIDCLink      `json:"oidc,omitempty"`
}

type signinOIDCLink struct {
	Issuer  string `json:"iss"`
	Subject string `json:"sub"`
}

type signinIdentity struct {
	link  signinOIDCLink
	email string
	name  string
}

type signinState struct {
	Nonce    string `json:"n"`
	Verifier string `json:"v"`
	Bind     string `json:"b"`
	Exp      int64  `json:"x"`
	Next     string `json:"t,omitempty"`
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
	s := &signin{h: h, secure: strings.HasPrefix(h.api, "https://"), providers: map[string]*signinProvider{}}
	if s.wa, err = webauthn.New(&webauthn.Config{RPID: u.Hostname(), RPDisplayName: "Archive", RPOrigins: []string{u.Scheme + "://" + u.Host}}); err != nil {
		return nil, fmt.Errorf("passkeys: %w", err)
	}
	for _, c := range signinBuiltin {
		env := "ARCHIVE_" + strings.ToUpper(c.ID)
		raw := strings.TrimSpace(os.Getenv(env))
		if raw == "" {
			continue
		}
		p, err := url.Parse(raw)
		if err != nil || p.Host == "" || p.Scheme != "https" && p.Scheme != "http" {
			return nil, fmt.Errorf("%s: want %s, with the client id and secret URL-escaped", env, strings.Replace(c.base, "://", "://<client id>:<client secret>@", 1))
		}
		c.clientID = p.User.Username()
		c.secret, _ = p.User.Password()
		c.base = strings.TrimRight(p.Scheme+"://"+p.Host+p.Path, "/")
		if c.ID == "apple" {
			if _, err := signinAppleKeyOf(c.secret); err != nil {
				return nil, fmt.Errorf("%s: want https://<services id>:<team id>.<key id>.<the .p8 key, base64>@appleid.apple.com, URL-escaped: %v", env, err)
			}
		}
		if c.clientID != "" && c.secret != "" {
			s.configs = append(s.configs, c)
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

	open("GET /auth/methods", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		_, err := s.pending(r)
		return ok(map[string]any{"providers": append([]signinConfig{}, s.configs...), "code": err == nil}, nil)
	})
	open("POST /auth/signout", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		s.setSession(w, "")
		http.SetCookie(w, s.cookie(signinTotp, "", "/auth/", -1))
		var errs []error
		for _, name := range []string{signinCookie, signinTotp} {
			if c, err := r.Cookie(name); err == nil {
				errs = append(errs, h.q().SignOut(r.Context(), authHash(c.Value)))
			}
		}
		return http.StatusNoContent, nil, errors.Join(errs...)
	})
	open("POST /auth/totp/verify", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		if err := s.sameOrigin(r); err != nil {
			return 0, nil, err
		}
		t, err := s.pending(r)
		if err != nil {
			return 0, nil, err
		}
		if len(t.Workspaces) == 0 {
			return 0, nil, signinRefused
		}
		var body struct {
			Code string `json:"code"`
		}
		if err := httpDecode(r, &body); err != nil {
			return 0, nil, err
		}
		step, err := s.totpCheck(r.Context(), t.UserID, body.Code)
		if err != nil {
			return 0, nil, err
		}
		if n, err := h.q().TotpUse(r.Context(), step, t.UserID); err != nil {
			return 0, nil, err
		} else if n == 0 {
			return 0, nil, totpWrong
		}
		c, err := r.Cookie(signinTotp)
		if err != nil {
			return 0, nil, signinMismatch
		}
		secret := authNewSecret()
		if n, err := h.q().TotpSession(r.Context(), authHash(secret), authHash(c.Value)); err != nil {
			return 0, nil, err
		} else if n == 0 {
			return 0, nil, signinMismatch
		}
		http.SetCookie(w, s.cookie(signinTotp, "", "/auth/", -1))
		s.setSession(w, secret)
		u, err := userBy(r.Context(), h, (*generated.Queries).UserByID, t.UserID)
		return ok(map[string]*User{"user": u}, err)
	})
	authed("GET /auth/me", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		a, ac := archiveFrom(r.Context()), ActorFrom(r.Context())
		m, err := a.memberOf(r.Context(), ac.Email)
		if err != nil {
			return 0, nil, err
		}
		if ac.Role != ac.UserRole {
			m.Role = "viewer"
		}
		rows, err := h.q().UserWorkspaces(r.Context(), ac.UserID)
		if err != nil {
			return 0, nil, err
		}
		u, err := h.q().UserByID(r.Context(), ac.UserID)
		if err != nil {
			return 0, nil, err
		}
		all := []Workspace{}
		for _, x := range rows {
			if !ac.scoped || x.ID == a.ws.ID {
				w := workspaceOut(x)
				w.Unread = slices.Contains(u.Unread, x.ID)
				all = append(all, w)
			}
		}
		admin := u.Admin && ac.Role == ac.UserRole
		return ok(map[string]any{"actor": ac, "role": m.Role, "group": m.Group, "workspace": a.ws.ID.String(), "workspaces": all, "admin": admin}, err)
	})
	authed("DELETE /auth/unread", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		return http.StatusNoContent, nil, h.q().ClearUnread(r.Context(), archiveFrom(r.Context()).ws.ID, ActorFrom(r.Context()).UserID)
	})
	authed("GET /auth/tokens", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		return ok(archiveFrom(r.Context()).Tokens(r.Context()))
	})
	authed("POST /auth/tokens", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		var body struct {
			Name     string `json:"name"`
			Expires  string `json:"expires"`
			Viewer   bool   `json:"viewer"`
			Personal bool   `json:"personal"`
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
		t, err := archiveFrom(r.Context()).CreateToken(r.Context(), body.Name, ttl, body.Viewer, body.Personal)
		return http.StatusCreated, t, err
	})
	authed("GET /auth/tokens/{id}/secret", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		id, err := httpID(r)
		if err != nil {
			return 0, nil, err
		}
		secret, err := archiveFrom(r.Context()).TokenSecret(r.Context(), id)
		return http.StatusOK, map[string]string{"secret": secret}, err
	})
	authed("DELETE /auth/tokens/{id}", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		id, err := httpID(r)
		if err != nil {
			return 0, nil, err
		}
		return http.StatusNoContent, nil, archiveFrom(r.Context()).RevokeToken(r.Context(), id)
	})

	browser := func(pattern string, f func(w http.ResponseWriter, r *http.Request, ac *Actor) (int, any, error)) {
		authed(pattern, func(w http.ResponseWriter, r *http.Request) (int, any, error) {
			ac := ActorFrom(r.Context())
			if ac.TokenID != uuid.Nil {
				return 0, nil, fmt.Errorf("two-factor changes need a signed-in browser, not a token: %w", ErrForbidden)
			}
			if r.Method != http.MethodGet {
				if err := s.sameOrigin(r); err != nil {
					return 0, nil, err
				}
			}
			return f(w, r, ac)
		})
	}
	browser("GET /auth/totp", func(w http.ResponseWriter, r *http.Request, ac *Actor) (int, any, error) {
		u, err := userBy(r.Context(), h, (*generated.Queries).UserByID, ac.UserID)
		if err != nil {
			return 0, nil, err
		}
		return ok(map[string]bool{"on": u.Totp != nil}, nil)
	})
	browser("POST /auth/totp/setup", func(w http.ResponseWriter, r *http.Request, ac *Actor) (int, any, error) {
		u, err := userBy(r.Context(), h, (*generated.Queries).UserByID, ac.UserID)
		if err != nil {
			return 0, nil, err
		}
		if u.Totp != nil {
			return 0, nil, fmt.Errorf("An app is linked already: %w", ErrConflict)
		}
		secret := make([]byte, 20)
		rand.Read(secret)
		b, _ := json.Marshal(signinTotpSetup{secret, time.Now().Add(10 * time.Minute)})
		sealed := s.totpSeal("totp link "+ac.UserID.String(), b)
		http.SetCookie(w, s.cookie(signinTotpLink, base64.RawURLEncoding.EncodeToString(sealed), "/auth/totp/", 600))
		key := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret)
		link := url.URL{Scheme: "otpauth", Host: "totp", Path: "/Archive:" + ac.Email, RawQuery: url.Values{"secret": {key}, "issuer": {"Archive"}}.Encode()}
		return ok(map[string]string{"key": key, "uri": link.String()}, nil)
	})
	browser("POST /auth/totp/enable", func(w http.ResponseWriter, r *http.Request, ac *Actor) (int, any, error) {
		var body struct {
			Code string `json:"code"`
		}
		if err := httpDecode(r, &body); err != nil {
			return 0, nil, err
		}
		c, err := r.Cookie(signinTotpLink)
		if err != nil {
			return 0, nil, fmt.Errorf("Start again: %w", httpErrBadRequest)
		}
		sealed, err := base64.RawURLEncoding.DecodeString(c.Value)
		var setup signinTotpSetup
		if err == nil {
			var b []byte
			if b, err = s.totpOpen("totp link "+ac.UserID.String(), sealed); err == nil {
				err = json.Unmarshal(b, &setup)
			}
		}
		if err != nil || time.Now().After(setup.Exp) {
			return 0, nil, fmt.Errorf("Start again: %w", httpErrBadRequest)
		}
		step := totpStep(setup.Secret, body.Code)
		if step == 0 {
			return 0, nil, totpWrong
		}
		keep, _ := h.secret(r)
		n, err := h.q().EnableTotp(r.Context(), generated.EnableTotpParams{ID: ac.UserID, Totp: s.totpSeal("totp "+ac.UserID.String(), setup.Secret), Step: step, Keep: authHash(keep)})
		if err != nil {
			return 0, nil, err
		}
		if n == 0 {
			return 0, nil, fmt.Errorf("An app is linked already: %w", ErrConflict)
		}
		http.SetCookie(w, s.cookie(signinTotpLink, "", "/auth/totp/", -1))
		return ok(map[string]bool{"on": true}, nil)
	})
	browser("POST /auth/totp/disable", func(w http.ResponseWriter, r *http.Request, ac *Actor) (int, any, error) {
		var body struct {
			Code string `json:"code"`
		}
		if err := httpDecode(r, &body); err != nil {
			return 0, nil, err
		}
		step, err := s.totpCheck(r.Context(), ac.UserID, body.Code)
		if err != nil {
			return 0, nil, err
		}
		n, err := h.q().DisableTotp(r.Context(), ac.UserID, step)
		if err != nil {
			return 0, nil, err
		}
		if n == 0 {
			return 0, nil, totpWrong
		}
		return ok(map[string]bool{"on": false}, nil)
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
		if err != nil || cer.User != ac.UserID {
			return 0, nil, cmp.Or(err, fmt.Errorf("passkey ceremony belongs to another user: %w", ErrForbidden))
		}
		pu, err := s.user(r.Context(), ac.UserID)
		if err != nil {
			return 0, nil, err
		}
		cred, err := s.wa.FinishRegistration(pu, cer.Data, r)
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
		}, cer.Data, r)
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
		bind := authNewSecret()
		st := signinState{Nonce: authNewSecret(), Verifier: oauth2.GenerateVerifier(), Bind: authHash(bind), Exp: time.Now().Add(10 * time.Minute).Unix(), Next: signinNext(r.URL.Query().Get("next"))}
		http.SetCookie(w, s.cookie(signinOIDC, bind, "/auth/oidc/", 600))
		opts := []oauth2.AuthCodeOption{oidc.Nonce(st.Nonce), oauth2.S256ChallengeOption(st.Verifier)}
		if p.apple != nil {
			// Apple sends the name and email only to a form post, and takes no PKCE.
			opts = []oauth2.AuthCodeOption{oidc.Nonce(st.Nonce), oauth2.SetAuthURLParam("response_mode", "form_post")}
		}
		http.Redirect(w, r, p.cfg.AuthCodeURL(s.seal(st), opts...), http.StatusFound)
	})
	// Apple posts the code back from its own site, which carries no Lax cookie; a redirect to the GET callback carries it again.
	mux.HandleFunc("POST /auth/oidc/{provider}/callback", func(w http.ResponseWriter, r *http.Request) {
		q := url.Values{}
		for _, k := range []string{"code", "state", "error"} {
			if v := r.PostFormValue(k); v != "" {
				q.Set(k, v)
			}
		}
		http.Redirect(w, r, s.h.api+"/auth/oidc/"+url.PathEscape(r.PathValue("provider"))+"/callback?"+q.Encode(), http.StatusSeeOther)
	})
	mux.HandleFunc("GET /auth/oidc/{provider}/callback", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, s.cookie(signinOIDC, "", "/auth/oidc/", -1))
		u, err := s.oidcCallback(r)
		next := "/"
		if st, _ := s.unseal(r.URL.Query().Get("state")); st != nil && st.Next != "" {
			next = st.Next
		}
		if err == nil {
			_, err = s.signedIn(w, r, u)
		}
		if err != nil {
			httpWrite(w, 0, nil, err)
			return
		}
		http.Redirect(w, r, h.app+next, http.StatusSeeOther)
	})
	h.workspaceRoutes(authed)
	h.mcpRoutesSignin(open)
	return mux
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

func (s *signin) signedIn(w http.ResponseWriter, r *http.Request, u *User) (any, error) {
	if len(u.Workspaces) == 0 {
		return nil, signinRefused
	}
	secret := authNewSecret()
	if u.Totp != nil {
		if err := s.h.q().CreateTotpToken(r.Context(), u.ID, authHash(secret)); err != nil {
			return nil, err
		}
		http.SetCookie(w, s.cookie(signinTotp, secret, "/auth/", 300))
		return map[string]bool{"code": true}, nil
	}
	if err := s.h.q().CreateSession(r.Context(), u.ID, authHash(secret)); err != nil {
		return nil, err
	}
	s.setSession(w, secret)
	return map[string]*User{"user": u}, nil
}

func (s *signin) sameOrigin(r *http.Request) error {
	if r.Header.Get("Origin") != httpOrigin(s.h.app) {
		return fmt.Errorf("send this from %s: %w", s.h.app, ErrForbidden)
	}
	return nil
}

func (s *signin) pending(r *http.Request) (generated.AuthenticateRow, error) {
	c, err := r.Cookie(signinTotp)
	if err != nil {
		return generated.AuthenticateRow{}, signinMismatch
	}
	t, err := s.h.q().Authenticate(r.Context(), authHash(c.Value), []string{"totp"})
	if errors.Is(err, pgx.ErrNoRows) {
		return t, signinMismatch
	}
	return t, err
}

func (s *signin) totpSeal(purpose string, plain []byte) []byte {
	nonce := make([]byte, s.h.aead.NonceSize())
	rand.Read(nonce)
	return s.h.aead.Seal(nonce, nonce, plain, []byte(purpose))
}

func (s *signin) totpOpen(purpose string, sealed []byte) ([]byte, error) {
	n := s.h.aead.NonceSize()
	if len(sealed) < n {
		return nil, errors.New("sealed value is too short")
	}
	return s.h.aead.Open(nil, sealed[:n], sealed[n:], []byte(purpose))
}

func (s *signin) totpCheck(ctx context.Context, id uuid.UUID, code string) (int64, error) {
	sealed, err := s.h.q().TotpTry(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, totpLocked
	}
	if err != nil {
		return 0, err
	}
	secret, err := s.totpOpen("totp "+id.String(), sealed)
	if err != nil {
		return 0, fmt.Errorf("two-factor secret of %s does not open with ARCHIVE_SECRET_KEY: %w", id, err)
	}
	step := totpStep(secret, code)
	if step == 0 {
		return 0, totpWrong
	}
	return step, nil
}

func totpCode(secret []byte, step int64) string {
	m := hmac.New(sha1.New, secret)
	m.Write(binary.BigEndian.AppendUint64(nil, uint64(step)))
	sum := m.Sum(nil)
	at := sum[len(sum)-1] & 15
	return fmt.Sprintf("%06d", binary.BigEndian.Uint32(sum[at:at+4])&0x7fffffff%1000000)
}

func totpStep(secret []byte, code string) int64 {
	now, found := time.Now().Unix()/30, int64(0)
	for step := now - 1; step <= now+1; step++ {
		if subtle.ConstantTimeCompare([]byte(totpCode(secret, step)), []byte(code)) == 1 {
			found = step
		}
	}
	return found
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
	b, _ := json.Marshal(signinCeremonyData{data, user, time.Now().Add(5 * time.Minute)})
	nonce := make([]byte, s.h.aead.NonceSize())
	rand.Read(nonce)
	sealed := s.h.aead.Seal(nonce, nonce, b, []byte(signinCeremony))
	http.SetCookie(w, s.cookie(signinCeremony, base64.RawURLEncoding.EncodeToString(sealed), "/auth/passkey/", 300))
}

func (s *signin) finish(w http.ResponseWriter, r *http.Request) (signinCeremonyData, error) {
	var cer signinCeremonyData
	c, err := r.Cookie(signinCeremony)
	if err != nil {
		return cer, fmt.Errorf("no passkey ceremony in progress: %w", httpErrBadRequest)
	}
	http.SetCookie(w, s.cookie(signinCeremony, "", "/auth/passkey/", -1))
	sealed, err := base64.RawURLEncoding.DecodeString(c.Value)
	n := s.h.aead.NonceSize()
	if err != nil || len(sealed) < n {
		return cer, fmt.Errorf("passkey ceremony expired: %w", httpErrBadRequest)
	}
	b, err := s.h.aead.Open(nil, sealed[:n], sealed[n:], []byte(signinCeremony))
	if err != nil || json.Unmarshal(b, &cer) != nil || time.Now().After(cer.Exp) {
		return cer, fmt.Errorf("passkey ceremony expired: %w", httpErrBadRequest)
	}
	return cer, nil
}

func (s *signin) provider(id string) (*signinProvider, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p := s.providers[id]; p != nil {
		return p, nil
	}
	i := slices.IndexFunc(s.configs, func(c signinConfig) bool { return c.ID == id })
	if i < 0 {
		return nil, fmt.Errorf("sign-in provider %q: %w", id, ErrNotFound)
	}
	c := s.configs[i]
	p := &signinProvider{signinConfig: c, cfg: oauth2.Config{ClientID: c.clientID, ClientSecret: c.secret,
		RedirectURL: s.h.api + "/auth/oidc/" + id + "/callback", Scopes: []string{oidc.ScopeOpenID, "email", "profile"}}}
	if id == "apple" {
		key, err := signinAppleKeyOf(c.secret)
		if err != nil {
			return nil, err
		}
		p.apple, p.cfg.Scopes = key, []string{"name", "email"}
	}
	if id == "github" {
		p.cfg.Endpoint, p.cfg.Scopes = github.Endpoint, []string{"read:user", "user:email"}
		if c.base != signinGitHub {
			p.cfg.Endpoint = oauth2.Endpoint{AuthURL: c.base + "/login/oauth/authorize", TokenURL: c.base + "/login/oauth/access_token"}
		}
	} else {
		ctx, issuer := context.Background(), c.base
		if id == "microsoft" {
			issuer += "/common/v2.0"
			ctx = oidc.InsecureIssuerURLContext(ctx, issuer)
		}
		op, err := oidc.NewProvider(ctx, issuer)
		if err != nil {
			return nil, fmt.Errorf("sign-in provider %s: %w", c.Name, err)
		}
		p.cfg.Endpoint = op.Endpoint()
		if p.apple != nil {
			p.cfg.Endpoint.AuthStyle = oauth2.AuthStyleInParams
		}
		p.verifier = op.Verifier(&oidc.Config{ClientID: c.clientID, SkipIssuerCheck: id == "microsoft"})
	}
	s.providers[id] = p
	return p, nil
}

func (s *signin) seal(st signinState) string {
	b, _ := json.Marshal(st)
	nonce := make([]byte, s.h.aead.NonceSize())
	rand.Read(nonce)
	return base64.RawURLEncoding.EncodeToString(s.h.aead.Seal(nonce, nonce, b, []byte("oidc")))
}

func (s *signin) unseal(state string) (*signinState, error) {
	ct, err := base64.RawURLEncoding.DecodeString(state)
	n := s.h.aead.NonceSize()
	if err != nil || len(ct) < n {
		return nil, signinMismatch
	}
	b, err := s.h.aead.Open(nil, ct[:n], ct[n:], []byte("oidc"))
	var st signinState
	if err != nil || json.Unmarshal(b, &st) != nil || time.Now().Unix() > st.Exp {
		return nil, signinMismatch
	}
	return &st, nil
}

func (s *signin) identify(ctx context.Context, p *signinProvider, tok *oauth2.Token, nonce string) (*signinIdentity, error) {
	if p.verifier == nil {
		return s.github(ctx, p, tok)
	}
	raw, _ := tok.Extra("id_token").(string)
	idt, err := p.verifier.Verify(ctx, raw)
	if err != nil || idt.Nonce != nonce {
		return nil, fmt.Errorf("sign-in: invalid ID token: %w", ErrUnauthorized)
	}
	if p.ID == "microsoft" && (!strings.HasPrefix(idt.Issuer, p.base+"/") || !strings.HasSuffix(idt.Issuer, "/v2.0")) {
		return nil, fmt.Errorf("sign-in: unexpected issuer %s: %w", idt.Issuer, ErrUnauthorized)
	}
	var claims struct {
		Email         string          `json:"email"`
		EmailVerified json.RawMessage `json:"email_verified"`
		DomainOwner   bool            `json:"xms_edov"`
		Name          string          `json:"name"`
	}
	if err := idt.Claims(&claims); err != nil {
		return nil, err
	}
	id := &signinIdentity{link: signinOIDCLink{idt.Issuer, idt.Subject}, name: claims.Name}
	// Apple says email_verified as the string "true".
	verified := string(claims.EmailVerified) == "true" || string(claims.EmailVerified) == `"true"`
	if verified || p.ID == "microsoft" && claims.DomainOwner {
		id.email = claims.Email
	}
	return id, nil
}

func (s *signin) github(ctx context.Context, p *signinProvider, tok *oauth2.Token) (*signinIdentity, error) {
	api := p.base
	if api == signinGitHub {
		api = "https://api.github.com"
	}
	c := p.cfg.Client(ctx, tok)
	get := func(path string, v any) error {
		req, err := http.NewRequestWithContext(ctx, "GET", api+path, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		resp, err := c.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("GitHub %s: %s", path, resp.Status)
		}
		return json.NewDecoder(resp.Body).Decode(v)
	}
	var user struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	err := get("/user", &user)
	if err == nil && user.ID == 0 {
		err = errors.New("GitHub sent no user id")
	}
	if err == nil {
		err = get("/user/emails", &emails)
	}
	if err != nil {
		return nil, fmt.Errorf("sign-in: %v: %w", err, ErrUnauthorized)
	}
	id := &signinIdentity{link: signinOIDCLink{p.base, strconv.FormatInt(user.ID, 10)}, name: user.Name}
	for _, e := range emails {
		if e.Primary && e.Verified {
			id.email = e.Email
		}
	}
	return id, nil
}

func (s *signin) oidcCallback(r *http.Request) (*User, error) {
	ctx := r.Context()
	p, err := s.provider(r.PathValue("provider"))
	if err != nil {
		return nil, err
	}
	st, err := s.unseal(r.URL.Query().Get("state"))
	if err != nil {
		return nil, err
	}
	if c, err := r.Cookie(signinOIDC); err != nil || authHash(c.Value) != st.Bind {
		return nil, signinMismatch
	}
	cfg, opts := p.cfg, []oauth2.AuthCodeOption{oauth2.VerifierOption(st.Verifier)}
	if p.apple != nil {
		if cfg.ClientSecret, err = p.apple.secret(p.clientID); err != nil {
			return nil, err
		}
		opts = nil
	}
	tok, err := cfg.Exchange(ctx, r.URL.Query().Get("code"), opts...)
	if err != nil {
		return nil, fmt.Errorf("sign-in: %v: %w", err, ErrUnauthorized)
	}
	id, err := s.identify(ctx, p, tok, st.Nonce)
	if err != nil {
		return nil, err
	}
	email, _ := authEmail(id.email)
	lb, _ := json.Marshal([]signinOIDCLink{id.link})
	u, err := userBy(ctx, s.h, (*generated.Queries).UserByOIDC, lb)
	if errors.Is(err, ErrNotFound) && email != "" {
		if u, err = userBy(ctx, s.h, (*generated.Queries).UserByEmail, email); err == nil && len(u.Workspaces) > 0 {
			var pu *signinUser
			if pu, err = s.user(ctx, u.ID); err == nil {
				pu.m.OIDC = append(pu.m.OIDC, id.link)
				err = s.save(ctx, pu)
			}
			if name := strings.Join(strings.Fields(id.name), " "); err == nil && name != "" {
				err = s.h.q().NameUser(ctx, u.ID, name)
			}
		}
	}
	if err == nil && len(u.Workspaces) == 0 {
		err = ErrNotFound
	}
	if errors.Is(err, ErrNotFound) {
		return nil, signinRefused
	}
	return u, err
}

func signinAppleKeyOf(secret string) (*signinAppleKey, error) {
	team, rest, _ := strings.Cut(secret, ".")
	id, raw, _ := strings.Cut(rest, ".")
	b, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("the key is not base64: %v", err)
	}
	if block, _ := pem.Decode(b); block != nil {
		b = block.Bytes
	}
	k, err := x509.ParsePKCS8PrivateKey(b)
	ec, ok := k.(*ecdsa.PrivateKey)
	if err != nil || !ok || team == "" || id == "" {
		return nil, errors.New("want the team id, the key id and the key Apple gave as a .p8")
	}
	return &signinAppleKey{team: team, id: id, key: ec}, nil
}

func (k *signinAppleKey) secret(clientID string) (string, error) {
	now := time.Now()
	head, _ := json.Marshal(map[string]string{"alg": "ES256", "kid": k.id})
	body, _ := json.Marshal(map[string]any{"iss": k.team, "iat": now.Unix(), "exp": now.Add(time.Hour).Unix(), "aud": signinApple, "sub": clientID})
	signed := base64.RawURLEncoding.EncodeToString(head) + "." + base64.RawURLEncoding.EncodeToString(body)
	sum := sha256.Sum256([]byte(signed))
	r, s, err := ecdsa.Sign(rand.Reader, k.key, sum[:])
	if err != nil {
		return "", err
	}
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return signed + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func signinNext(next string) string {
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.ContainsAny(next, "\\\r\n") || len(next) > 4096 {
		return ""
	}
	return next
}

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
		f, ferr := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if ferr == nil {
			_, ferr = f.WriteString(hex.EncodeToString(key) + "\n")
			if cerr := f.Close(); ferr == nil {
				ferr = cerr
			}
			return key, ferr
		}
		if !errors.Is(ferr, fs.ErrExist) {
			return nil, ferr
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
