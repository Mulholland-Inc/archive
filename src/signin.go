package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

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
		// Cookies are shared between tabs. Bind queued work to the account the
		// client observed, using this request's authenticated actor, not a
		// separate /auth/me preflight. This precondition grants no permissions.
		if values, present := r.Header["Archive-Expected-User"]; present {
			var expected uuid.UUID
			if len(values) == 1 {
				expected, err = uuid.Parse(values[0])
			}
			if len(values) != 1 || err != nil || expected == uuid.Nil {
				httpWrite(w, 0, nil, fmt.Errorf("Archive-Expected-User: want one user UUID: %w", httpErrBadRequest))
				return
			}
			if expected != ac.UserID {
				httpWrite(w, http.StatusPreconditionFailed, map[string]string{"error": "signed-in account changed"}, nil)
				return
			}
		}
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
	// A product that shows a sign-in link as a page of its own, so a mail scanner that follows the
	// link does not spend it, uses the link when that page's button is pressed.
	mux.HandleFunc("POST /auth/magic/use", func(w http.ResponseWriter, r *http.Request) {
		if token, ok := s.posted(w, r); ok {
			s.browser(w, r, func() (*User, error) { return a.UseMagicLink(r.Context(), token) })
		}
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
	authed("POST /auth/signout/everywhere", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		http.SetCookie(w, s.cookie(signinCookie, "", "/", -1))
		return http.StatusNoContent, nil, a.SignOutEverywhere(r.Context())
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
	authed("POST /auth/spaces/{space}/invites", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		var body struct {
			Email string `json:"email"`
			Name  string `json:"name"`
			Role  string `json:"role"`
		}
		if err := httpDecode(r, &body); err != nil {
			return 0, nil, err
		}
		inv, secret, err := a.Invite(r.Context(), r.PathValue("space"), body.Email, body.Name, body.Role)
		if err != nil {
			return 0, nil, err
		}
		// the link goes back to whoever invited, to send as they like
		inv.Link = s.base.String() + "/auth/invite?token=" + url.QueryEscape(secret)
		return http.StatusCreated, inv, nil
	})
	// Opening an invite only shows it; a press of its button uses it, so a mail scanner that
	// follows links does not spend it. A client that is not a browser reads what it is for.
	mux.HandleFunc("GET /auth/invite", func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		inv, err := a.PeekInvite(r.Context(), token)
		w.Header().Set("Cache-Control", "no-store")
		if err != nil || !strings.Contains(r.Header.Get("Accept"), "text/html") {
			httpWrite(w, http.StatusOK, inv, err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Referrer-Policy", "no-referrer")
		as := "a " + inv.Role
		if inv.Role == "admin" || inv.Role == "editor" || inv.Role == "owner" {
			as = "an " + inv.Role
		}
		inviteTmpl.Execute(w, map[string]any{"Invite": inv, "Token": token, "As": as})
	})
	mux.HandleFunc("POST /auth/invite", func(w http.ResponseWriter, r *http.Request) {
		if token, ok := s.posted(w, r); ok {
			s.browser(w, r, func() (*User, error) { return a.AcceptInvite(r.Context(), token) })
		}
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

// posted is the token a press of a link's button sends, as a form or as JSON. A press that signs a
// browser in must come from this archive's own pages or from no browser at all (an app, a script),
// so a page elsewhere cannot sign it into an account of that page's choosing; posted answers 403
// itself when not.
func (s *signin) posted(w http.ResponseWriter, r *http.Request) (string, bool) {
	if err := http.NewCrossOriginProtection().Check(r); err != nil {
		httpWrite(w, 0, nil, fmt.Errorf("that came from another site: %w", ErrForbidden))
		return "", false
	}
	if token := r.PostFormValue("token"); token != "" {
		return token, true
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := httpDecode(r, &body); err != nil {
		httpWrite(w, 0, nil, err)
		return "", false
	}
	return body.Token, true
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
	u, err := s.a.userBy(ctx, `id = $1`, id)
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
		_, err := s.a.st.Q(ctx).Exec(ctx, `UPDATE "user" SET signin = $2 WHERE id = $1`, pu.u.ID, b)
		return err
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
	u, err := s.a.userBy(r.Context(), `signin->'oidc' @> $1::jsonb`, string(lb))
	if errors.Is(err, ErrNotFound) && claims.Email != "" && (claims.EmailVerified || claims.DomainOwner) {
		if email, e := authEmail(claims.Email); e == nil {
			u, err = s.a.userBy(r.Context(), `email = $1`, email)
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
