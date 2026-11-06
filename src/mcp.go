package main

import (
	"cmp"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Mulholland-Inc/archive/generated"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

const (
	mcpAccessTTL  = time.Hour
	mcpRefreshTTL = 90 * 24 * time.Hour
	mcpAskTTL     = 15 * time.Minute
	mcpRedirects  = 10
	mcpNoName     = "MCP client"
)

var (
	mcpPKCE   = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)
	mcpScheme = regexp.MustCompile(`^[a-z][a-z0-9+.-]*$`)
	mcpUnsafe = []string{"javascript", "data", "file", "vbscript", "about", "blob", "filesystem", "ftp", "ws", "wss", "mailto", "tel", "sms"}
)

type mcpAsk struct {
	Client    uuid.UUID `json:"c"`
	Redirect  string    `json:"r"`
	Challenge string    `json:"p"`
	State     string    `json:"s"`
	Exp       int64     `json:"x"`
}

func (h *Hub) mcpHost(r *http.Request) bool {
	if h.mcpURL == h.api {
		return false
	}
	u, err := url.Parse(h.mcpURL)
	host := r.Host
	if hn, _, err := net.SplitHostPort(host); err == nil {
		host = hn
	}
	return err == nil && strings.EqualFold(host, u.Hostname())
}

func (h *Hub) mcpBase(r *http.Request) string {
	if h.mcpHost(r) {
		return h.mcpURL
	}
	return h.api
}

func (h *Hub) mcpAt() string {
	if h.mcpURL == h.api {
		return h.api + "/mcp"
	}
	return h.mcpURL
}

func (h *Hub) mcpRoutes(mux *http.ServeMux, own bool) {
	switch {
	case own:
		mux.HandleFunc("/{$}", h.mcp)
		mux.HandleFunc("GET /.well-known/oauth-protected-resource", h.mcpResource)
		mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	case h.mcpURL == h.api:
		mux.HandleFunc("/mcp", h.mcp)
		mux.HandleFunc("GET /.well-known/oauth-protected-resource/mcp", h.mcpResource)
	default:
		return
	}
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", h.mcpServerMeta)
	mux.HandleFunc("GET /oauth/authorize", h.mcpAuthorize)
	mux.HandleFunc("POST /oauth/token", h.mcpToken)
	mux.HandleFunc("POST /oauth/register", h.mcpRegister)
	mux.HandleFunc("POST /oauth/revoke", h.mcpRevoke)
}

func mcpCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") == "" {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Expose-Headers", "WWW-Authenticate, Mcp-Session-Id")
		if r.Method == "OPTIONS" && r.Header.Get("Access-Control-Request-Method") != "" {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Mcp-Protocol-Version, Mcp-Session-Id, Last-Event-ID, Archive-Workspace")
			w.Header().Set("Access-Control-Max-Age", "7200")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Hub) mcpRefuse(w http.ResponseWriter, r *http.Request, err error) {
	status, v := httpError(err)
	if status == http.StatusUnauthorized {
		bad := ""
		if r.Header.Get("Authorization") != "" {
			bad = `error="invalid_token", `
		}
		path := strings.TrimSuffix(r.URL.Path, "/")
		w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer %sresource_metadata="%s/.well-known/oauth-protected-resource%s"`, bad, h.mcpBase(r), path))
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func mcpJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func mcpFail(w http.ResponseWriter, status int, code, desc string) {
	if status == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", `Basic realm="archive"`)
	}
	mcpJSON(w, status, map[string]string{"error": code, "error_description": desc})
}

func (h *Hub) mcpResource(w http.ResponseWriter, r *http.Request) {
	mcpJSON(w, http.StatusOK, &oauthex.ProtectedResourceMetadata{
		Resource:               h.mcpBase(r) + strings.TrimPrefix(r.URL.Path, "/.well-known/oauth-protected-resource"),
		AuthorizationServers:   []string{h.mcpURL},
		BearerMethodsSupported: []string{"header"},
		ResourceName:           "Archive",
		ResourceDocumentation:  h.app,
	})
}

func (h *Hub) mcpServerMeta(w http.ResponseWriter, r *http.Request) {
	auth := []string{"none", "client_secret_post", "client_secret_basic"}
	mcpJSON(w, http.StatusOK, map[string]any{
		"issuer":                                         h.mcpURL,
		"authorization_endpoint":                         h.mcpURL + "/oauth/authorize",
		"token_endpoint":                                 h.mcpURL + "/oauth/token",
		"registration_endpoint":                          h.mcpURL + "/oauth/register",
		"revocation_endpoint":                            h.mcpURL + "/oauth/revoke",
		"response_types_supported":                       []string{"code"},
		"response_modes_supported":                       []string{"query"},
		"grant_types_supported":                          []string{"authorization_code", "refresh_token"},
		"token_endpoint_auth_methods_supported":          auth,
		"revocation_endpoint_auth_methods_supported":     auth,
		"code_challenge_methods_supported":               []string{"S256"},
		"authorization_response_iss_parameter_supported": true,
		"service_documentation":                          h.app,
	})
}

func (h *Hub) mcpResourceOK(resource string) bool {
	resource = strings.TrimSuffix(resource, "/")
	return resource == "" || resource == h.mcpAt()
}

func mcpRedirectValid(s string) bool {
	u, err := url.Parse(s)
	if err != nil || len(s) > 2000 || strings.Contains(s, "#") || u.User != nil || !mcpScheme.MatchString(u.Scheme) {
		return false
	}
	switch {
	case u.Scheme == "https":
		return u.Host != "" && u.Hostname() != ""
	case u.Scheme == "http":
		return localHost(u.Hostname())
	case slices.Contains(mcpUnsafe, u.Scheme):
		return false
	}
	return u.Host != "" || u.Path != "" || u.Opaque != ""
}

func mcpRedirectMatch(registered []string, given string) bool {
	if !mcpRedirectValid(given) {
		return false
	}
	if slices.Contains(registered, given) {
		return true
	}
	g, _ := url.Parse(given)
	if g.Scheme != "http" {
		return false
	}
	for _, s := range registered {
		u, err := url.Parse(s)
		if err == nil && u.Scheme == "http" && u.Hostname() == g.Hostname() && u.EscapedPath() == g.EscapedPath() && u.RawQuery == g.RawQuery {
			return true
		}
	}
	return false
}

func (h *Hub) mcpSendTo(redirect string, params map[string]string) string {
	u, _ := url.Parse(redirect)
	q := u.Query()
	for k, v := range params {
		if v != "" {
			q.Set(k, v)
		}
	}
	q.Set("iss", h.mcpURL)
	u.RawQuery = q.Encode()
	return u.String()
}

func (h *Hub) mcpSeal(ask mcpAsk) string {
	b, _ := json.Marshal(ask)
	nonce := make([]byte, h.aead.NonceSize())
	rand.Read(nonce)
	return base64.RawURLEncoding.EncodeToString(h.aead.Seal(nonce, nonce, b, []byte("mcp authorize")))
}

var mcpAskStale = fmt.Errorf("this sign-in link is old or broken; go back to the app you came from and connect again: %w", httpErrBadRequest)

func (h *Hub) mcpUnseal(s string) (*mcpAsk, error) {
	ct, err := base64.RawURLEncoding.DecodeString(s)
	n := h.aead.NonceSize()
	if err != nil || len(ct) < n {
		return nil, mcpAskStale
	}
	b, err := h.aead.Open(nil, ct[:n], ct[n:], []byte("mcp authorize"))
	var ask mcpAsk
	if err != nil || json.Unmarshal(b, &ask) != nil || time.Now().Unix() > ask.Exp {
		return nil, mcpAskStale
	}
	return &ask, nil
}

func (h *Hub) mcpClient(r *http.Request, id string) (*generated.ArchiveMcpClient, error) {
	cid, err := uuid.Parse(id)
	if err != nil {
		return nil, ErrNotFound
	}
	c, err := h.q().MCPClient(r.Context(), cid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

func (h *Hub) mcpAuthorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	screen := func(msg string) {
		http.Redirect(w, r, h.app+"/authorize?error="+url.QueryEscape(msg), http.StatusFound)
	}
	c, err := h.mcpClient(r, q.Get("client_id"))
	if errors.Is(err, ErrNotFound) {
		screen("The app that sent you here isn’t registered with this Archive. Go back to it and connect again.")
		return
	} else if err != nil {
		httpWrite(w, 0, nil, err)
		return
	}
	redirect := q.Get("redirect_uri")
	if redirect == "" && len(c.RedirectUris) == 1 {
		redirect = c.RedirectUris[0]
	}
	if !mcpRedirectMatch(c.RedirectUris, redirect) {
		screen("The app that sent you here asked to go back to an address it didn’t register. Nothing was shared.")
		return
	}
	back := func(code, desc string) {
		http.Redirect(w, r, h.mcpSendTo(redirect, map[string]string{"error": code, "error_description": desc, "state": q.Get("state")}), http.StatusFound)
	}
	switch {
	case q.Get("response_type") != "code":
		back("unsupported_response_type", "response_type must be code")
	case q.Get("code_challenge_method") != "S256" || !mcpPKCE.MatchString(q.Get("code_challenge")):
		back("invalid_request", "PKCE is required: send code_challenge with code_challenge_method S256")
	case !h.mcpResourceOK(q.Get("resource")):
		back("invalid_target", "resource must be this Archive's MCP, "+h.mcpURL)
	default:
		ask := mcpAsk{Client: c.ID, Redirect: redirect, Challenge: q.Get("code_challenge"), State: q.Get("state"), Exp: time.Now().Add(mcpAskTTL).Unix()}
		http.Redirect(w, r, h.app+"/authorize?request="+h.mcpSeal(ask), http.StatusFound)
	}
}

func (h *Hub) mcpClientAuth(r *http.Request) (*generated.ArchiveMcpClient, bool) {
	id, secret := r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	if u, p, ok := r.BasicAuth(); ok {
		u, _ = url.QueryUnescape(u)
		p, _ = url.QueryUnescape(p)
		if id != "" && id != u {
			return nil, false
		}
		id, secret = u, p
	}
	c, err := h.mcpClient(r, id)
	if err != nil {
		return nil, false
	}
	if c.Secret != nil && subtle.ConstantTimeCompare([]byte(authHash(secret)), []byte(*c.Secret)) != 1 {
		return nil, false
	}
	return c, true
}

func mcpForm(w http.ResponseWriter, r *http.Request) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		mcpFail(w, http.StatusBadRequest, "invalid_request", "want an application/x-www-form-urlencoded body")
		return false
	}
	return true
}

func (h *Hub) mcpToken(w http.ResponseWriter, r *http.Request) {
	if !mcpForm(w, r) {
		return
	}
	c, ok := h.mcpClientAuth(r)
	if !ok {
		mcpFail(w, http.StatusUnauthorized, "invalid_client", "unknown client, or a wrong client secret")
		return
	}
	f, ctx := r.PostForm, r.Context()
	if !h.mcpResourceOK(f.Get("resource")) {
		mcpFail(w, http.StatusBadRequest, "invalid_target", "resource must be this Archive's MCP, "+h.mcpURL)
		return
	}
	access, refresh := authNewSecret(), authNewSecret()
	exp, rexp := time.Now().Add(mcpAccessTTL).UTC(), time.Now().Add(mcpRefreshTTL).UTC()
	var ws *uuid.UUID
	switch f.Get("grant_type") {
	case "authorization_code":
		id, hash := coreNewID(), authHash(f.Get("code"))
		code, err := h.q().UseMCPCode(ctx, &id, hash)
		if errors.Is(err, pgx.ErrNoRows) {
			if tok, err := h.q().UsedMCPCode(ctx, hash); err == nil && tok != nil {
				if ws, err := h.q().DeleteMCPToken(ctx, *tok); err == nil && ws != nil {
					h.live.moved(ws.String(), liveChange{Table: "archive.token"})
				}
			}
			mcpFail(w, http.StatusBadRequest, "invalid_grant", "the code is unknown, used or expired")
			return
		} else if err != nil {
			mcpFail(w, http.StatusInternalServerError, "server_error", err.Error())
			return
		}
		sum := sha256.Sum256([]byte(f.Get("code_verifier")))
		switch {
		case code.Client != c.ID:
			mcpFail(w, http.StatusBadRequest, "invalid_grant", "the code was issued to another client")
			return
		case f.Get("redirect_uri") != code.RedirectUri:
			mcpFail(w, http.StatusBadRequest, "invalid_grant", "redirect_uri is not the one the code was issued for")
			return
		case !mcpPKCE.MatchString(f.Get("code_verifier")) ||
			subtle.ConstantTimeCompare([]byte(base64.RawURLEncoding.EncodeToString(sum[:])), []byte(code.Challenge)) != 1:
			mcpFail(w, http.StatusBadRequest, "invalid_grant", "code_verifier does not match the code_challenge")
			return
		}
		if err := h.q().CreateMCPToken(ctx, generated.CreateMCPTokenParams{ID: id, UserID: code.UserID, Workspace: &code.Workspace, Name: cmp.Or(c.Name, mcpNoName),
			Hash: authHash(access), ExpiresAt: &exp, Client: &c.ID, Refresh: new(authHash(refresh)), RefreshExpiresAt: &rexp}); err != nil {
			mcpFail(w, http.StatusInternalServerError, "server_error", err.Error())
			return
		}
		ws = &code.Workspace
	case "refresh_token":
		row, err := h.q().RefreshMCPToken(ctx, generated.RefreshMCPTokenParams{Hash: authHash(access), ExpiresAt: &exp, Next: new(authHash(refresh)),
			RefreshExpiresAt: &rexp, Refresh: new(authHash(f.Get("refresh_token"))), Client: &c.ID})
		if errors.Is(err, pgx.ErrNoRows) {
			mcpFail(w, http.StatusBadRequest, "invalid_grant", "the refresh token is unknown, revoked, used or expired")
			return
		} else if err != nil {
			mcpFail(w, http.StatusInternalServerError, "server_error", err.Error())
			return
		}
		ws = row.Workspace
	default:
		mcpFail(w, http.StatusBadRequest, "unsupported_grant_type", "grant_type must be authorization_code or refresh_token")
		return
	}
	if ws != nil {
		h.live.moved(ws.String(), liveChange{Table: "archive.token"})
	}
	w.Header().Set("Pragma", "no-cache")
	mcpJSON(w, http.StatusOK, map[string]any{"access_token": access, "token_type": "Bearer", "expires_in": int(mcpAccessTTL.Seconds()), "refresh_token": refresh})
}

func (h *Hub) mcpRevoke(w http.ResponseWriter, r *http.Request) {
	if !mcpForm(w, r) {
		return
	}
	c, ok := h.mcpClientAuth(r)
	if !ok {
		mcpFail(w, http.StatusUnauthorized, "invalid_client", "unknown client, or a wrong client secret")
		return
	}
	row, err := h.q().RevokeMCPToken(r.Context(), &c.ID, authHash(r.PostForm.Get("token")))
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		mcpFail(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	if err == nil && row.Workspace != nil {
		h.live.moved(row.Workspace.String(), liveChange{Table: "archive.token"})
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
}

func (h *Hub) mcpRegister(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RedirectURIs []string `json:"redirect_uris"`
		ClientName   string   `json:"client_name"`
		AuthMethod   string   `json:"token_endpoint_auth_method"`
		GrantTypes   []string `json:"grant_types"`
		Responses    []string `json:"response_types"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&body); err != nil {
		mcpFail(w, http.StatusBadRequest, "invalid_client_metadata", "want a JSON body")
		return
	}
	if len(body.RedirectURIs) == 0 || len(body.RedirectURIs) > mcpRedirects {
		mcpFail(w, http.StatusBadRequest, "invalid_redirect_uri", fmt.Sprintf("want 1 to %d redirect_uris", mcpRedirects))
		return
	}
	for _, u := range body.RedirectURIs {
		if !mcpRedirectValid(u) {
			mcpFail(w, http.StatusBadRequest, "invalid_redirect_uri", fmt.Sprintf("%q: want https, http to localhost or 127.0.0.1, or an app's own scheme, with no fragment", u))
			return
		}
	}
	method := body.AuthMethod
	if method == "" {
		method = "client_secret_basic"
		if slices.Contains(body.GrantTypes, "authorization_code") || len(body.GrantTypes) == 0 {
			method = "none"
		}
	}
	switch {
	case !slices.Contains([]string{"none", "client_secret_post", "client_secret_basic"}, method):
		mcpFail(w, http.StatusBadRequest, "invalid_client_metadata", "token_endpoint_auth_method must be none, client_secret_post or client_secret_basic")
		return
	case slices.ContainsFunc(body.GrantTypes, func(g string) bool { return g != "authorization_code" && g != "refresh_token" }):
		mcpFail(w, http.StatusBadRequest, "invalid_client_metadata", "grant_types may be authorization_code and refresh_token")
		return
	case slices.ContainsFunc(body.Responses, func(t string) bool { return t != "code" }):
		mcpFail(w, http.StatusBadRequest, "invalid_client_metadata", "response_types may only be code")
		return
	}
	name := strings.Join(strings.Fields(body.ClientName), " ")
	if n := []rune(name); len(n) > 80 {
		name = string(n[:80])
	}
	var secret string
	var hash *string
	if method != "none" {
		secret = authNewSecret()
		hash = new(authHash(secret))
	}
	row, err := h.q().CreateMCPClient(r.Context(), name, body.RedirectURIs, hash)
	if err != nil {
		mcpFail(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	out := map[string]any{"client_id": row.ID.String(), "client_id_issued_at": row.CreatedAt.Unix(), "redirect_uris": body.RedirectURIs,
		"grant_types": []string{"authorization_code", "refresh_token"}, "response_types": []string{"code"}, "token_endpoint_auth_method": method}
	if name != "" {
		out["client_name"] = name
	}
	if secret != "" {
		out["client_secret"], out["client_secret_expires_at"] = secret, 0
	}
	mcpJSON(w, http.StatusCreated, out)
}

func (h *Hub) mcpUser(r *http.Request) (*generated.AuthenticateRow, error) {
	if r.Header.Get("Authorization") != "" {
		return nil, fmt.Errorf("let an app in from the browser you are signed in with, not with a token: %w", ErrForbidden)
	}
	if c, err := r.Cookie(signinCookie); err == nil && c.Value != "" {
		t, err := h.q().Authenticate(r.Context(), authHash(c.Value), []string{"session"})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("signed out: %w", ErrUnauthorized)
		}
		return &t, err
	}
	if !h.local(r) {
		return nil, fmt.Errorf("signed out: %w", ErrUnauthorized)
	}
	u, err := h.q().FirstAdmin(r.Context())
	if err != nil {
		return nil, fmt.Errorf("signed out: %w", ErrUnauthorized)
	}
	t := &generated.AuthenticateRow{Kind: "session", UserID: u.ID, Email: u.Email, UserName: u.Name}
	for _, w := range u.Workspaces {
		t.Workspaces = append(t.Workspaces, w.String())
	}
	return t, nil
}

func (h *Hub) mcpRoutesSignin(open func(string, func(http.ResponseWriter, *http.Request) (int, any, error))) {
	ok := func(v any, err error) (int, any, error) { return http.StatusOK, v, err }
	open("GET /auth/oauth/request", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		ask, err := h.mcpUnseal(r.URL.Query().Get("request"))
		if err != nil {
			return 0, nil, err
		}
		c, err := h.mcpClient(r, ask.Client.String())
		if err != nil {
			return 0, nil, mcpAskStale
		}
		out := map[string]any{"client": map[string]string{"name": c.Name}, "user": nil, "workspaces": []Workspace{}}
		t, err := h.mcpUser(r)
		if errors.Is(err, ErrUnauthorized) {
			return ok(out, nil)
		} else if err != nil {
			return 0, nil, err
		}
		rows, err := h.q().UserWorkspaces(r.Context(), t.UserID)
		all := []Workspace{}
		for _, x := range rows {
			all = append(all, workspaceOut(x))
		}
		out["user"], out["workspaces"] = map[string]string{"email": t.Email, "name": t.UserName}, all
		return ok(out, err)
	})
	open("POST /auth/oauth/answer", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		if r.Header.Get("Origin") != httpOrigin(h.app) {
			return 0, nil, fmt.Errorf("answer from %s: %w", h.app, ErrForbidden)
		}
		var body struct {
			Request   string `json:"request"`
			Workspace string `json:"workspace"`
			Allow     bool   `json:"allow"`
		}
		if err := httpDecode(r, &body); err != nil {
			return 0, nil, err
		}
		ask, err := h.mcpUnseal(body.Request)
		if err != nil {
			return 0, nil, err
		}
		c, err := h.mcpClient(r, ask.Client.String())
		if err != nil || !mcpRedirectMatch(c.RedirectUris, ask.Redirect) {
			return 0, nil, mcpAskStale
		}
		t, err := h.mcpUser(r)
		if err != nil {
			return 0, nil, err
		}
		if !body.Allow {
			return ok(map[string]string{"redirect": h.mcpSendTo(ask.Redirect, map[string]string{"error": "access_denied", "error_description": "you said no", "state": ask.State})}, nil)
		}
		_, _, release, err := h.enter(r.Context(), *t, body.Workspace)
		if err != nil {
			return 0, nil, err
		}
		release()
		ws, _ := uuid.Parse(body.Workspace)
		code := authNewSecret()
		if err := h.q().CreateMCPCode(r.Context(), generated.CreateMCPCodeParams{Hash: authHash(code), Client: c.ID, UserID: t.UserID, Workspace: ws,
			RedirectUri: ask.Redirect, Challenge: ask.Challenge}); err != nil {
			return 0, nil, err
		}
		return ok(map[string]string{"redirect": h.mcpSendTo(ask.Redirect, map[string]string{"code": code, "state": ask.State})}, nil)
	})
}
