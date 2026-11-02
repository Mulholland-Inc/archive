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

type Person struct {
	ID     uuid.UUID `json:"id"`
	Email  string    `json:"email"`
	Name   string    `json:"name"`
	Role   string    `json:"role"`
	Groups []string  `json:"groups"`
}

type Token struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	Viewer    bool       `json:"viewer,omitempty"`
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

func (a *Archive) owner(ctx context.Context, role string) bool {
	return a.has(ctx, role, a.prefix+"_owner")
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
	var m generated.MemberByEmailRow
	err := a.st.System(ctx, func(ctx context.Context) (err error) {
		m, err = a.st.db(ctx).MemberByEmail(ctx, email)
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		err = fmt.Errorf("you are not in this workspace: %w", ErrForbidden)
	}
	return m, err
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
	return a.joined(ctx, email, name)
}

func (a *Archive) joined(ctx context.Context, email, name string) (*User, error) {
	ctx = context.WithoutCancel(ctx)
	hq, name := a.hub.q(), strings.TrimSpace(name)
	if _, err := hq.JoinWorkspace(ctx, email, name, a.ws.ID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
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

func (a *Archive) RemoveUser(ctx context.Context, email string) error {
	email, err := authEmail(email)
	if err != nil {
		return err
	}
	people, err := a.People(ctx)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(people, func(p Person) bool { return p.Email == email })
	if i < 0 {
		return fmt.Errorf("user %s: %w", email, ErrNotFound)
	}
	return a.Delete(ctx, "member", people[i].ID)
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
	for _, u := range users {
		in[u.Email] = true
	}
	for _, m := range ms {
		keep[m.Email] = true
		if !in[m.Email] {
			u, err := hq.JoinWorkspace(ctx, m.Email, "", a.ws.ID)
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

func (a *Archive) People(ctx context.Context) ([]Person, error) {
	ms, err := coreDo(a, ctx, func(ctx context.Context) ([]generated.MembersRow, error) { return a.st.db(ctx).Members(ctx) })
	if err != nil {
		return nil, err
	}
	users, err := a.hub.q().WorkspaceUsers(ctx, a.ws.ID)
	names := map[string]string{}
	for _, u := range users {
		names[u.Email] = u.Name
	}
	out := []Person{}
	for _, m := range ms {
		out = append(out, Person{ID: m.ID, Email: m.Email, Name: names[m.Email], Role: m.Role, Groups: m.Groups})
	}
	return out, err
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

func (a *Archive) CLIActor(ctx context.Context, email string) (*Actor, error) {
	if email == "" {
		u, err := userBy(ctx, a.hub, (*generated.Queries).FirstUser, a.ws.ID)
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return a.userActor(ctx, u)
	}
	email, err := authEmail(email)
	if err != nil {
		return nil, err
	}
	u, err := userBy(ctx, a.hub, (*generated.Queries).UserByEmail, email)
	if errors.Is(err, ErrNotFound) {
		return nil, fmt.Errorf("user %s: %w", email, ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	return a.userActor(ctx, u)
}

func (a *Archive) CreateToken(ctx context.Context, name string, ttl time.Duration, viewer bool) (*Token, error) {
	ac, err := authUser(ctx)
	if err != nil {
		return nil, err
	}
	t := &Token{ID: coreNewID(), Name: strings.TrimSpace(name), Viewer: viewer, Secret: authNewSecret()}
	var exp *time.Time
	if ttl > 0 {
		e := time.Now().Add(ttl).UTC()
		exp = &e
	}
	return t, a.sys(ctx, func(ctx context.Context) error {
		var role *string
		if viewer {
			r := a.roleID("t", t.ID)
			if err := a.exec(ctx, authCreate(r), authGrant(a.prefix+"_viewer", r), authGrant(a.roleID("m", ac.member), r), authGrant(r, a.prefix+"_authenticator")); err != nil {
				return err
			}
			role = &r
		}
		row, err := a.hub.q().CreateToken(ctx, generated.CreateTokenParams{ID: t.ID, UserID: ac.UserID, Workspace: &a.ws.ID, Name: t.Name, Role: role, Hash: authHash(t.Secret), ExpiresAt: exp})
		t.CreatedAt, t.ExpiresAt = row.CreatedAt, row.ExpiresAt
		a.moved(ctx, "archive.token", t.ID.String())
		return err
	})
}

func (a *Archive) Tokens(ctx context.Context) ([]*Token, error) {
	ac, err := authUser(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := a.hub.q().Tokens(ctx, ac.UserID, &a.ws.ID)
	out := []*Token{}
	for _, r := range rows {
		out = append(out, &Token{ID: r.ID, Name: r.Name, Viewer: r.Viewer, ExpiresAt: r.ExpiresAt, CreatedAt: r.CreatedAt})
	}
	return out, err
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
	signinGitHub   = "https://github.com"
)

var (
	signinRefused  = fmt.Errorf("no Archive account for this sign-in; ask an owner to add you: %w", ErrForbidden)
	signinMismatch = fmt.Errorf("sign-in state does not match; start again: %w", ErrUnauthorized)
	signinBuiltin  = []signinConfig{
		{ID: "google", Name: "Google", base: "https://accounts.google.com"},
		{ID: "microsoft", Name: "Microsoft", base: "https://login.microsoftonline.com"},
		{ID: "slack", Name: "Slack", base: "https://slack.com"},
		{ID: "github", Name: "GitHub", base: signinGitHub},
	}
)

type signin struct {
	h       *Hub
	secure  bool
	wa      *webauthn.WebAuthn
	configs []signinConfig

	mu         sync.Mutex
	ceremonies map[string]signinCeremonyData
	providers  map[string]*signinProvider
}

type signinCeremonyData struct {
	data webauthn.SessionData
	user uuid.UUID
	exp  time.Time
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
	s := &signin{h: h, secure: strings.HasPrefix(h.api, "https://"), ceremonies: map[string]signinCeremonyData{}, providers: map[string]*signinProvider{}}
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
		return ok(map[string]any{"providers": append([]signinConfig{}, s.configs...)}, nil)
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
		m, err := a.memberOf(r.Context(), ac.Email)
		if err != nil {
			return 0, nil, err
		}
		if ac.Role != ac.UserRole {
			m.Role = "viewer"
		}
		rows, err := h.q().UserWorkspaces(r.Context(), ac.UserID)
		all := []Workspace{}
		for _, x := range rows {
			if r.Header.Get("Authorization") == "" || x.ID == a.ws.ID {
				all = append(all, workspaceOut(x))
			}
		}
		return ok(map[string]any{"actor": ac, "role": m.Role, "groups": m.Groups, "workspace": a.ws.ID.String(), "workspaces": all}, err)
	})
	authed("GET /auth/users", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		return ok(archiveFrom(r.Context()).People(r.Context()))
	})
	authed("POST /auth/users", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		var body struct {
			Email string `json:"email"`
			Name  string `json:"name"`
			Role  string `json:"role"`
		}
		if err := httpDecode(r, &body); err != nil {
			return 0, nil, err
		}
		a := archiveFrom(r.Context())
		if _, err := a.AddUser(r.Context(), body.Email, body.Name, body.Role); err != nil {
			return 0, nil, err
		}
		p, err := a.People(r.Context())
		return http.StatusCreated, p, err
	})
	authed("DELETE /auth/users/{email}", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		a := archiveFrom(r.Context())
		if err := a.RemoveUser(r.Context(), r.PathValue("email")); err != nil {
			return 0, nil, err
		}
		return ok(a.People(r.Context()))
	})
	authed("GET /auth/tokens", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		return ok(archiveFrom(r.Context()).Tokens(r.Context()))
	})
	authed("POST /auth/tokens", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		var body struct {
			Name    string `json:"name"`
			Expires string `json:"expires"`
			Viewer  bool   `json:"viewer"`
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
		t, err := archiveFrom(r.Context()).CreateToken(r.Context(), body.Name, ttl, body.Viewer)
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
		return ok(s.signedIn(w, r, found.u))
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
		http.SetCookie(w, s.cookie(signinOIDC, "", "/auth/oidc/", -1))
		u, err := s.oidcCallback(r)
		if err == nil {
			_, err = s.signedIn(w, r, u)
		}
		if err != nil {
			httpWrite(w, 0, nil, err)
			return
		}
		http.Redirect(w, r, h.app+"/", http.StatusSeeOther)
	})
	h.workspaceRoutes(authed)
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
	if err := s.h.q().CreateSession(r.Context(), u.ID, authHash(secret)); err != nil {
		return nil, err
	}
	s.setSession(w, secret)
	return map[string]*User{"user": u}, nil
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
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		DomainOwner   bool   `json:"xms_edov"`
		Name          string `json:"name"`
	}
	if err := idt.Claims(&claims); err != nil {
		return nil, err
	}
	id := &signinIdentity{link: signinOIDCLink{idt.Issuer, idt.Subject}, name: claims.Name}
	if claims.EmailVerified || p.ID == "microsoft" && claims.DomainOwner {
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
	tok, err := p.cfg.Exchange(ctx, r.URL.Query().Get("code"), oauth2.VerifierOption(st.Verifier))
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

func secretMask(s *semType, v any) (out string) {
	mask := semMasks[s.Mask]
	if mask == nil {
		return "****"
	}
	defer func() {
		if recover() != nil {
			out = semPlain(v)
		}
	}()
	return mask(v)
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
	id, access := coreRowID(row), coreAccess(row["access"])
	cat := a.catalog(ctx)
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
		if err := q.CreateSecret(ctx, generated.CreateSecretParams{ID: sid, TableName: tb.Name, RowID: id, ColumnName: col, Type: typ, Value: a.secretBox(sid, typ, b), Access: access}); err != nil {
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
