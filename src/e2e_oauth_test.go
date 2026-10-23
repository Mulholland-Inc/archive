package archive

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

// e2eConsent plays the person's browser on the consent page: it opens the authorization URL with
// the session cookie, presses Allow, and returns where the server sends them back to.
func e2eConsent(t *testing.T, authURL string, session *http.Cookie, allow string) *url.URL {
	t.Helper()
	nc := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, _ := http.NewRequest("GET", authURL, nil)
	req.AddCookie(session)
	resp, err := nc.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	page, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	field := func(name string) string {
		m := regexp.MustCompile(`name="` + name + `" value="([^"]*)"`).FindSubmatch(page)
		if m == nil {
			t.Fatalf("no %s on the consent page:\n%s", name, page)
		}
		return strings.ReplaceAll(string(m[1]), "&amp;", "&")
	}
	base, _ := url.Parse(authURL)
	form := url.Values{"query": {field("query")}, "csrf": {field("csrf")}, "allow": {allow}}
	req, _ = http.NewRequest("POST", base.Scheme+"://"+base.Host+"/oauth/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(session)
	if resp, err = nc.Do(req); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	back, err := url.Parse(resp.Header.Get("Location"))
	if resp.StatusCode != http.StatusSeeOther || err != nil {
		t.Fatalf("consent: %d %s", resp.StatusCode, resp.Header.Get("Location"))
	}
	return back
}

// An MCP client that knows nothing but the server's address connects the way the MCP spec says,
// with the SDK's own OAuth client: it finds the metadata from the 401, registers itself, sends the
// person to allow it, trades the code for tokens with PKCE, and calls a tool as them.
func TestAnMCPClientConnectsWithOAuthAsTheMCPSpecSays(t *testing.T) {
	in := newInst(t, "ARCHIVE_ADMIN_EMAIL=alice@example.com")
	srv := in.serve()
	in.must("spec", "push", in.writeSpec())
	alice := e2eSignIn(t, srv, "alice@example.com")
	in.must("create", "company", `{"name": "Gloamshire Mills"}`)

	var authURL string
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	h, err := auth.NewAuthorizationCodeHandler(&auth.AuthorizationCodeHandlerConfig{
		RedirectURL: "http://127.0.0.1:9/callback",
		DynamicClientRegistrationConfig: &auth.DynamicClientRegistrationConfig{Metadata: &oauthex.ClientRegistrationMetadata{
			ClientName: "Claude", RedirectURIs: []string{"http://127.0.0.1:9/callback"}}},
		RequestRefreshToken: true,
		AuthorizationCodeFetcher: func(ctx context.Context, args *auth.AuthorizationArgs) (*auth.AuthorizationResult, error) {
			authURL = args.URL
			back := e2eConsent(t, args.URL, alice, "yes")
			return &auth.AuthorizationResult{Code: back.Query().Get("code"), State: back.Query().Get("state"), Iss: back.Query().Get("iss")}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	c := mcp.NewClient(&mcp.Implementation{Name: "e2e", Version: "0"}, nil)
	// the address the server knows itself by, ARCHIVE_URL's default, as a real client would use it
	public := strings.Replace(srv.url, "127.0.0.1", "localhost", 1)
	sess, err := c.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: public + "/mcp", OAuthHandler: h}, nil)
	if err != nil {
		t.Fatalf("connect with oauth: %v\n%s", err, srv.log)
	}
	res, err := sess.CallTool(ctx, &mcp.CallToolParams{Name: "query", Arguments: map[string]any{"type": "company"}})
	if err != nil || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "Gloamshire Mills") {
		t.Fatalf("tool as alice: %v %+v", err, res)
	}
	sess.Close()

	ts, _ := h.TokenSource(ctx)
	tok, err := ts.Token()
	if err != nil || tok.RefreshToken == "" {
		t.Fatalf("tokens: %v %+v", err, tok)
	}
	api := srv.url + "/api"
	if st, _ := call(t, "GET", api+"/company", tok.AccessToken, nil, nil); st != 200 {
		t.Fatalf("the access token on the API: %d", st)
	}
	if !strings.Contains(in.must("token", "list"), `"Claude"`) {
		t.Errorf("the grant is not in alice's tokens: %s", in.must("token", "list"))
	}

	post := func(path string, v url.Values) (int, map[string]any) {
		var out map[string]any
		resp, err := http.PostForm(srv.url+path, v)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		json.Unmarshal(b, &out)
		return resp.StatusCode, out
	}
	clientID := regexp.MustCompile(`client_id=([^&]+)`).FindStringSubmatch(authURL)

	t.Run("RefreshRotatesBothTokens", func(t *testing.T) {
		st, out := post("/oauth/token", url.Values{"grant_type": {"refresh_token"}, "refresh_token": {tok.RefreshToken}, "client_id": {clientID[1]}})
		if st != 200 || out["access_token"] == nil {
			t.Fatalf("refresh: %d %v", st, out)
		}
		if st, _ := call(t, "GET", api+"/company", tok.AccessToken, nil, nil); st != 401 {
			t.Errorf("the old access token still works: %d", st)
		}
		if st, _ := call(t, "GET", api+"/company", out["access_token"].(string), nil, nil); st != 200 {
			t.Errorf("the new access token: %d", st)
		}
		if st, _ := post("/oauth/token", url.Values{"grant_type": {"refresh_token"}, "refresh_token": {tok.RefreshToken}, "client_id": {clientID[1]}}); st != 400 {
			t.Errorf("the old refresh token again: %d", st)
		}
		st, _ = post("/oauth/revoke", url.Values{"token": {out["refresh_token"].(string)}})
		wantStatus(t, st, 200, "revoke")
		if st, _ := call(t, "GET", api+"/company", out["access_token"].(string), nil, nil); st != 401 {
			t.Errorf("a revoked grant's access token still works: %d", st)
		}
	})

	t.Run("RefusesWhatTheSpecRefuses", func(t *testing.T) {
		resp, _ := http.Post(srv.url+"/oauth/register", "application/json", strings.NewReader(`{"redirect_uris": ["javascript:alert(1)"]}`))
		wantStatus(t, resp.StatusCode, 400, "a javascript: redirect")
		resp.Body.Close()
		bad := strings.Replace(authURL, "redirect_uri=", "redirect_uri=https%3A%2F%2Fevil.example%2F&x=", 1)
		nc := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		if resp, _ := nc.Get(bad); resp.StatusCode != 400 || resp.Header.Get("Location") != "" {
			t.Errorf("an unregistered redirect_uri: %d %s", resp.StatusCode, resp.Header.Get("Location"))
		}
		back := e2eConsent(t, authURL, alice, "yes")
		code := back.Query().Get("code")
		st, out := post("/oauth/token", url.Values{"grant_type": {"authorization_code"}, "code": {code}, "client_id": {clientID[1]},
			"redirect_uri": {"http://127.0.0.1:9/callback"}, "code_verifier": {"not-the-verifier-that-made-the-challenge-0000"}})
		if st != 400 || out["error"] != "invalid_grant" {
			t.Errorf("a wrong code_verifier: %d %v", st, out)
		}
		st, _ = post("/oauth/token", url.Values{"grant_type": {"authorization_code"}, "code": {code}, "client_id": {clientID[1]}, "redirect_uri": {"http://127.0.0.1:9/callback"}})
		wantStatus(t, st, 400, "a code used twice")
		if back := e2eConsent(t, authURL, alice, "no"); back.Query().Get("error") != "access_denied" {
			t.Errorf("deny: %s", back)
		}
		form := url.Values{"query": {strings.SplitN(authURL, "?", 2)[1]}, "allow": {"yes"}}
		req, _ := http.NewRequest("POST", srv.url+"/oauth/authorize", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(alice)
		if resp, _ := nc.Do(req); resp.StatusCode != 403 {
			t.Errorf("consent without the form's csrf: %d", resp.StatusCode)
		}
	})

	t.Run("SomeoneSignedOutIsAskedToSignInAndComesBack", func(t *testing.T) {
		resp, _ := http.Get(authURL)
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if !strings.Contains(string(b), `action="/oauth/signin"`) || strings.Contains(string(b), "<script") {
			t.Fatalf("signed out: %s", b)
		}
	})
}
