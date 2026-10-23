package main

import (
	"cmp"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

// OAuth 2.1 for MCP clients and apps, as the MCP authorization spec asks: protected resource and
// authorization server metadata, dynamic client registration, the authorization code flow with
// PKCE (S256 only), refresh tokens that rotate, and revocation. Clients are public, so none has a
// secret. A grant is an ordinary API token over the user's spaces at their levels, named after the
// client, so it shows in the token list and is revoked like any other; its secret is the access
// token, replaced on every refresh.

const (
	oauthCodeTTL   = 5 * time.Minute
	oauthAccessTTL = time.Hour
)

func (s *signin) oauthRoutes(mux *http.ServeMux) {
	base := s.base.String()
	prm := func(w http.ResponseWriter, r *http.Request) {
		oauthJSON(w, http.StatusOK, &oauthex.ProtectedResourceMetadata{Resource: base + "/mcp", AuthorizationServers: []string{base},
			BearerMethodsSupported: []string{"header"}, ResourceName: "Archive"})
	}
	mux.HandleFunc("GET /.well-known/oauth-protected-resource", prm)
	mux.HandleFunc("GET /.well-known/oauth-protected-resource/mcp", prm)
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		oauthJSON(w, http.StatusOK, &oauthex.AuthServerMeta{
			Issuer: base, AuthorizationEndpoint: base + "/oauth/authorize", TokenEndpoint: base + "/oauth/token",
			RegistrationEndpoint: base + "/oauth/register", RevocationEndpoint: base + "/oauth/revoke",
			ResponseTypesSupported: []string{"code"}, GrantTypesSupported: []string{"authorization_code", "refresh_token"},
			TokenEndpointAuthMethodsSupported: []string{"none"}, CodeChallengeMethodsSupported: []string{"S256"},
			RevocationEndpointAuthMethodsSupported:     []string{"none"},
			AuthorizationResponseIssParameterSupported: true,
		})
	})
	mux.HandleFunc("POST /oauth/register", s.oauthRegister)
	mux.HandleFunc("GET /oauth/authorize", s.oauthAuthorizePage)
	mux.HandleFunc("POST /oauth/authorize", s.oauthAuthorize)
	mux.HandleFunc("POST /oauth/signin", s.oauthSignin)
	mux.HandleFunc("POST /oauth/token", s.oauthToken)
	mux.HandleFunc("POST /oauth/revoke", s.oauthRevoke)
}

func oauthJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func oauthError(w http.ResponseWriter, status int, code, desc string) {
	oauthJSON(w, status, map[string]string{"error": code, "error_description": desc})
}

// oauthRedirectOK allows https, http on a loopback address (a native app's local listener) and a
// private-use scheme with a dot in it, such as com.example.app:/callback (RFC 8252); nothing that
// would run in a page, such as javascript: or data:.
func oauthRedirectOK(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Fragment != "" || u.Scheme == "" {
		return false
	}
	switch u.Scheme {
	case "https":
		return u.Host != ""
	case "http":
		ip := net.ParseIP(u.Hostname())
		return u.Hostname() == "localhost" || ip != nil && ip.IsLoopback()
	}
	return strings.Contains(u.Scheme, ".")
}

func (s *signin) oauthRegister(w http.ResponseWriter, r *http.Request) {
	var meta oauthex.ClientRegistrationMetadata
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&meta); err != nil {
		oauthError(w, 400, "invalid_client_metadata", "want a JSON client registration")
		return
	}
	if n := len(meta.RedirectURIs); n == 0 || n > 10 {
		oauthError(w, 400, "invalid_redirect_uri", "want 1 to 10 redirect_uris")
		return
	}
	for _, u := range meta.RedirectURIs {
		if !oauthRedirectOK(u) {
			oauthError(w, 400, "invalid_redirect_uri", "a redirect_uri must be https, http on loopback, or a private-use scheme: "+u)
			return
		}
	}
	name := strings.TrimSpace(meta.ClientName)
	if name == "" {
		name = "An assistant"
	}
	if len(name) > 64 {
		name = name[:64]
	}
	id := authNewSecret()[:32]
	err := s.a.sys(r.Context(), func(ctx context.Context) error {
		_, err := s.a.st.Q(ctx).Exec(ctx, `INSERT INTO oauth_clients (id, name, redirect_uris) VALUES ($1, $2, $3)`, id, name, meta.RedirectURIs)
		return err
	})
	if err != nil {
		oauthError(w, 500, "server_error", "could not register the client")
		return
	}
	oauthJSON(w, http.StatusCreated, map[string]any{
		"client_id": id, "client_name": name, "redirect_uris": meta.RedirectURIs, "client_id_issued_at": time.Now().Unix(),
		"token_endpoint_auth_method": "none", "grant_types": []string{"authorization_code", "refresh_token"}, "response_types": []string{"code"},
	})
}

type oauthRequest struct {
	ClientID, ClientName, RedirectURI, State, Challenge, Resource string
}

// oauthParse checks an authorization request. Until the client and its redirect_uri are known to
// match, errors are shown on the page rather than sent anywhere (redirect is false).
func (s *signin) oauthParse(ctx context.Context, v url.Values) (q *oauthRequest, redirect bool, code, desc string) {
	q = &oauthRequest{ClientID: v.Get("client_id"), RedirectURI: v.Get("redirect_uri"), State: v.Get("state"),
		Challenge: v.Get("code_challenge"), Resource: v.Get("resource")}
	var uris []string
	err := s.a.sys(ctx, func(ctx context.Context) error {
		return s.a.st.Q(ctx).QueryRow(ctx, `SELECT name, redirect_uris FROM oauth_clients WHERE id = $1`, q.ClientID).Scan(&q.ClientName, &uris)
	})
	switch {
	case err != nil:
		return q, false, "invalid_client", "This app is not registered with this archive."
	case !slices.Contains(uris, q.RedirectURI):
		return q, false, "invalid_request", "This app asked to be sent back somewhere it did not register."
	case v.Get("response_type") != "code":
		return q, true, "unsupported_response_type", "only code"
	case q.Challenge == "" || v.Get("code_challenge_method") != "S256":
		return q, true, "invalid_request", "PKCE with S256 is required"
	case q.Resource != "" && strings.TrimRight(q.Resource, "/") != s.base.String()+"/mcp" && strings.TrimRight(q.Resource, "/") != s.base.String():
		return q, true, "invalid_target", "this server is " + s.base.String() + "/mcp"
	}
	return q, true, "", ""
}

func (s *signin) oauthBack(w http.ResponseWriter, r *http.Request, q *oauthRequest, params url.Values) {
	u, _ := url.Parse(q.RedirectURI)
	v := u.Query()
	for k, vs := range params {
		v[k] = vs
	}
	if q.State != "" {
		v.Set("state", q.State)
	}
	v.Set("iss", s.base.String())
	u.RawQuery = v.Encode()
	http.Redirect(w, r, u.String(), http.StatusSeeOther)
}

// oauthCSRF ties the consent form to the session that saw it; a page on another site cannot read
// the session cookie, so it cannot make the field.
func oauthCSRF(r *http.Request) string {
	c, err := r.Cookie(signinCookie)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256([]byte("oauth-consent:" + c.Value))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (s *signin) oauthPage(w http.ResponseWriter, status int, data map[string]any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", webCSP)
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	oauthTmpl.Execute(w, data)
}

func (s *signin) oauthAuthorizePage(w http.ResponseWriter, r *http.Request) {
	q, redirect, code, desc := s.oauthParse(r.Context(), r.URL.Query())
	if code != "" {
		if redirect {
			s.oauthBack(w, r, q, url.Values{"error": {code}, "error_description": {desc}})
			return
		}
		s.oauthPage(w, http.StatusBadRequest, map[string]any{"Error": desc})
		return
	}
	ac, err := s.a.httpActor(r)
	if err != nil || ac.TokenID != uuid.Nil {
		s.oauthPage(w, http.StatusOK, map[string]any{"SignIn": true, "Client": q.ClientName, "Next": r.URL.RequestURI()})
		return
	}
	s.oauthPage(w, http.StatusOK, map[string]any{"Client": q.ClientName, "Who": cmp.Or(ac.Name, ac.Email), "Query": r.URL.RawQuery, "CSRF": oauthCSRF(r)})
}

// oauthSignin mails a sign-in link that comes back to the consent page, for someone who is not
// signed in when an app asks.
func (s *signin) oauthSignin(w http.ResponseWriter, r *http.Request) {
	next := r.PostFormValue("next")
	if !strings.HasPrefix(next, "/oauth/authorize?") {
		s.oauthPage(w, http.StatusBadRequest, map[string]any{"Error": "That sign-in did not come from an app asking for access."})
		return
	}
	secret, u, err := s.a.MagicLink(r.Context(), r.PostFormValue("email"))
	if err == nil {
		err = s.mail(u.Email, s.base.String()+"/auth/magic?token="+url.QueryEscape(secret)+"&next="+url.QueryEscape(next))
	}
	if err != nil && !errors.Is(err, ErrNotFound) {
		s.oauthPage(w, http.StatusBadRequest, map[string]any{"Error": err.Error()})
		return
	}
	s.oauthPage(w, http.StatusOK, map[string]any{"Sent": true})
}

func (s *signin) oauthAuthorize(w http.ResponseWriter, r *http.Request) {
	ac, err := s.a.httpActor(r)
	if err != nil || ac.TokenID != uuid.Nil || r.PostFormValue("csrf") == "" ||
		subtle.ConstantTimeCompare([]byte(r.PostFormValue("csrf")), []byte(oauthCSRF(r))) != 1 {
		s.oauthPage(w, http.StatusForbidden, map[string]any{"Error": "Open the app's request again and sign in to allow it."})
		return
	}
	v, _ := url.ParseQuery(r.PostFormValue("query"))
	q, redirect, code, desc := s.oauthParse(r.Context(), v)
	if code != "" {
		if redirect {
			s.oauthBack(w, r, q, url.Values{"error": {code}, "error_description": {desc}})
			return
		}
		s.oauthPage(w, http.StatusBadRequest, map[string]any{"Error": desc})
		return
	}
	if r.PostFormValue("allow") != "yes" {
		s.oauthBack(w, r, q, url.Values{"error": {"access_denied"}})
		return
	}
	secret := authNewSecret()
	err = s.a.sys(r.Context(), func(ctx context.Context) error {
		_, err := s.a.st.Q(ctx).Exec(ctx, `INSERT INTO oauth_codes (hash, client_id, user_id, redirect_uri, challenge, expires_at)
			VALUES ($1, $2, $3, $4, $5, now() + $6::interval)`, authHash(secret), q.ClientID, ac.UserID, q.RedirectURI, q.Challenge, oauthCodeTTL.String())
		return err
	})
	if err != nil {
		s.oauthBack(w, r, q, url.Values{"error": {"server_error"}})
		return
	}
	s.oauthBack(w, r, q, url.Values{"code": {secret}})
}

func (s *signin) oauthToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	clientID := r.PostFormValue("client_id")
	switch r.PostFormValue("grant_type") {
	case "authorization_code":
		var uid uuid.UUID
		var redirectURI, challenge, clientName string
		err := s.a.sys(ctx, func(ctx context.Context) error {
			return s.a.st.Q(ctx).QueryRow(ctx, `DELETE FROM oauth_codes c USING oauth_clients k WHERE k.id = c.client_id
				AND c.hash = $1 AND c.expires_at > now() RETURNING c.user_id, c.client_id, c.redirect_uri, c.challenge, k.name`,
				authHash(r.PostFormValue("code"))).Scan(&uid, &clientID, &redirectURI, &challenge, &clientName)
		})
		sum := sha256.Sum256([]byte(r.PostFormValue("code_verifier")))
		switch {
		case err != nil:
			oauthError(w, 400, "invalid_grant", "the code is invalid, used or expired")
			return
		case clientID != r.PostFormValue("client_id") || redirectURI != r.PostFormValue("redirect_uri"):
			oauthError(w, 400, "invalid_grant", "the code was issued to another client or redirect_uri")
			return
		case subtle.ConstantTimeCompare([]byte(base64.RawURLEncoding.EncodeToString(sum[:])), []byte(challenge)) != 1:
			oauthError(w, 400, "invalid_grant", "code_verifier does not match the code_challenge")
			return
		}
		u, err := s.a.userBy(ctx, `id = $1`, uid)
		if err != nil {
			oauthError(w, 400, "invalid_grant", "the user is gone")
			return
		}
		uctx := WithActor(ctx, &Actor{UserID: u.ID, Email: u.Email, Name: u.Name, Role: u.Role, UserRole: u.Role, Home: u.Home})
		t, err := s.a.CreateToken(uctx, clientName, nil, oauthAccessTTL, false)
		if err != nil {
			oauthError(w, 500, "server_error", "could not issue a token")
			return
		}
		refresh := authNewSecret()
		err = s.a.sys(ctx, func(ctx context.Context) error {
			_, err := s.a.st.Q(ctx).Exec(ctx, `INSERT INTO oauth_grants (token_id, client_id, refresh_hash) VALUES ($1, $2, $3)`, t.ID, clientID, authHash(refresh))
			return err
		})
		if err != nil {
			oauthError(w, 500, "server_error", "could not issue a token")
			return
		}
		oauthJSON(w, http.StatusOK, map[string]any{"access_token": t.Secret, "token_type": "Bearer",
			"expires_in": int(oauthAccessTTL.Seconds()), "refresh_token": refresh})
	case "refresh_token":
		access, refresh := authNewSecret(), authNewSecret()
		err := s.a.sys(ctx, func(ctx context.Context) error {
			var tid uuid.UUID
			err := s.a.st.Q(ctx).QueryRow(ctx, `UPDATE oauth_grants SET refresh_hash = $3 WHERE refresh_hash = $1 AND client_id = $2 RETURNING token_id`,
				authHash(r.PostFormValue("refresh_token")), clientID, authHash(refresh)).Scan(&tid)
			if err != nil {
				return err
			}
			_, err = s.a.st.Q(ctx).Exec(ctx, `UPDATE tokens SET hash = $2, expires_at = now() + $3::interval WHERE id = $1`, tid, authHash(access), oauthAccessTTL.String())
			return err
		})
		if errors.Is(err, pgx.ErrNoRows) {
			oauthError(w, 400, "invalid_grant", "the refresh token is invalid, used or revoked")
			return
		}
		if err != nil {
			oauthError(w, 500, "server_error", "could not refresh")
			return
		}
		oauthJSON(w, http.StatusOK, map[string]any{"access_token": access, "token_type": "Bearer",
			"expires_in": int(oauthAccessTTL.Seconds()), "refresh_token": refresh})
	default:
		oauthError(w, 400, "unsupported_grant_type", "want authorization_code or refresh_token")
	}
}

// oauthRevoke ends a grant given either of its tokens, and answers 200 whatever it was given, as
// RFC 7009 asks, so it says nothing about which tokens exist.
func (s *signin) oauthRevoke(w http.ResponseWriter, r *http.Request) {
	h := authHash(r.PostFormValue("token"))
	s.a.sys(r.Context(), func(ctx context.Context) error {
		_, err := s.a.st.Q(ctx).Exec(ctx, `DELETE FROM tokens WHERE id IN (
			SELECT token_id FROM oauth_grants WHERE refresh_hash = $1
			UNION SELECT g.token_id FROM oauth_grants g JOIN tokens t ON t.id = g.token_id WHERE t.hash = $1)`, h)
		return err
	})
	w.WriteHeader(http.StatusOK)
}

var oauthTmpl = template.Must(template.New("oauth").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Archive</title>
<style>body{font:17px/1.5 system-ui,sans-serif;max-width:28rem;margin:18vh auto;padding:0 1rem;color:#1d1d1f;background:#fff}
@media (prefers-color-scheme:dark){body{color:#f5f5f7;background:#111}}button,input{font:inherit;padding:.6rem 1rem;border-radius:.6rem}
button{border:0;background:#1d1d1f;color:#fff;margin-right:.5rem}button.no{background:transparent;color:inherit;border:1px solid #888}
input{border:1px solid #888;width:100%;box-sizing:border-box;margin:.25rem 0 1rem;background:transparent;color:inherit}
@media (prefers-color-scheme:dark){button{background:#f5f5f7;color:#111}}</style></head><body>
{{if .Error}}<h1>That did not work</h1><p>{{.Error}}</p>
{{else if .Sent}}<h1>Check your email</h1><p>If that address has an account, a sign-in link is on its way. It brings you back here.</p>
{{else if .SignIn}}<h1>Sign in to Archive</h1><p>{{.Client}} is asking to use your archive. Sign in first.</p>
<form method="post" action="/oauth/signin"><input type="hidden" name="next" value="{{.Next}}">
<label>Email<input type="email" name="email" autocomplete="email" required></label><button>Email me a link</button></form>
{{else}}<h1>Allow {{.Client}}?</h1>
<p>{{.Client}} wants to use your archive as {{.Who}}. It can read and change what you can, in every space you are in, until you revoke it under your tokens.</p>
<form method="post" action="/oauth/authorize"><input type="hidden" name="query" value="{{.Query}}"><input type="hidden" name="csrf" value="{{.CSRF}}">
<button name="allow" value="yes">Allow</button><button class="no" name="allow" value="no">Deny</button></form>{{end}}
</body></html>
`))
