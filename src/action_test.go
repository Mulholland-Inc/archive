package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/oauth2"
)

// fakeMCP stands in for an MCP connector's server and its authorization server, as Notion's: protected
// resource and authorization server metadata, Dynamic Client Registration, a token endpoint that replaces the
// refresh token on every refresh, revocation, and an MCP server whose tools need a token it issued. A code is
// the email of the account it connects.
type fakeMCP struct {
	srv *httptest.Server

	mu         sync.Mutex
	n          int
	registered []map[string]any  // the clients registered, as they asked
	clients    map[string]string // client id to secret
	refresh    map[string]string // live refresh tokens, to their account
	access     map[string]string // access tokens, to their account
	asked      []url.Values      // the token requests
	revoked    []url.Values      // the revocations
	pages      map[string]string // the private pages notion-list-private-pages lists, by id to title
	fail       bool              // the next list fails
}

func newFakeMCP(t *testing.T) *fakeMCP {
	f := &fakeMCP{clients: map[string]string{}, refresh: map[string]string{}, access: map[string]string{}, pages: map[string]string{}}
	tools := mcp.NewServer(&mcp.Implementation{Name: "wiki", Version: "1"}, nil)
	tools.AddTool(&mcp.Tool{Name: "notion-get-self", Description: "Who you are.", InputSchema: map[string]any{"type": "object"}},
		func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			who := f.who(req.Extra.Header.Get("Authorization"))
			b, _ := json.Marshal(map[string]any{"user": map[string]any{"name": "Alice", "email": strings.ToUpper(who[:1]) + who[1:]}, "workspace": "Acme"})
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}, nil
		})
	type searchIn struct {
		Query string `json:"query" jsonschema:"Words to find"`
		Limit int    `json:"limit,omitempty"`
	}
	type searchOut struct {
		Results []map[string]string `json:"results"`
	}
	mcp.AddTool(tools, &mcp.Tool{Name: "notion-search", Description: "Search pages.\n\nLonger text that the docs leave out."},
		func(ctx context.Context, req *mcp.CallToolRequest, in searchIn) (*mcp.CallToolResult, searchOut, error) {
			if in.Query == "boom" {
				return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "the search index is down"}}}, searchOut{}, nil
			}
			return nil, searchOut{Results: []map[string]string{{"title": "Page about " + in.Query, "by": f.who(req.Extra.Header.Get("Authorization"))}}}, nil
		})
	notionListTool(f, tools)
	served := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return tools }, nil)
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "http://" + r.Host
		if r.URL.Path == "/mcp" {
			if f.who(r.Header.Get("Authorization")) == "" {
				w.Header().Set("WWW-Authenticate", `Bearer realm="OAuth", resource_metadata="`+base+`/.well-known/oauth-protected-resource/mcp", scope="default"`)
				w.WriteHeader(401)
				return
			}
			served.ServeHTTP(w, r)
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		refuse := func(code int, e string) {
			w.WriteHeader(code)
			json.NewEncoder(w).Encode(map[string]string{"error": e})
		}
		switch r.URL.Path {
		case "/.well-known/oauth-protected-resource/mcp":
			json.NewEncoder(w).Encode(map[string]any{"resource": base + "/mcp", "authorization_servers": []string{base}, "scopes_supported": []string{"default"}})
		case "/.well-known/oauth-authorization-server":
			json.NewEncoder(w).Encode(map[string]any{"issuer": base, "authorization_endpoint": base + "/authorize", "token_endpoint": base + "/token",
				"registration_endpoint": base + "/register", "revocation_endpoint": base + "/revoke", "response_types_supported": []string{"code"},
				"code_challenge_methods_supported": []string{"S256"}, "token_endpoint_auth_methods_supported": []string{"client_secret_post", "none"},
				"grant_types_supported": []string{"authorization_code", "refresh_token"}})
		case "/register":
			var m map[string]any
			json.NewDecoder(r.Body).Decode(&m)
			f.registered = append(f.registered, m)
			f.n++
			id, secret := fmt.Sprintf("client-%d", f.n), fmt.Sprintf("secret-%d", f.n)
			f.clients[id] = secret
			m["client_id"], m["client_secret"], m["client_secret_expires_at"] = id, secret, 0
			w.WriteHeader(201)
			json.NewEncoder(w).Encode(m)
		case "/token":
			r.ParseForm()
			f.asked = append(f.asked, r.Form)
			if s, ok := f.clients[r.Form.Get("client_id")]; !ok || s != r.Form.Get("client_secret") {
				refuse(401, "invalid_client")
				return
			}
			var who string
			switch r.Form.Get("grant_type") {
			case "authorization_code":
				who = r.Form.Get("code")
			case "refresh_token":
				var ok bool
				if who, ok = f.refresh[r.Form.Get("refresh_token")]; !ok {
					refuse(400, "invalid_grant")
					return
				}
				delete(f.refresh, r.Form.Get("refresh_token"))
			}
			f.n++
			at, rt := fmt.Sprintf("at-%d", f.n), fmt.Sprintf("rt-%d", f.n)
			f.access[at], f.refresh[rt] = who, who
			json.NewEncoder(w).Encode(map[string]any{"access_token": at, "refresh_token": rt, "token_type": "Bearer", "expires_in": 3600, "scope": "default"})
		case "/revoke":
			r.ParseForm()
			f.revoked = append(f.revoked, r.Form)
			delete(f.refresh, r.Form.Get("token"))
			w.Write([]byte("{}"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeMCP) who(authorization string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.access[strings.TrimPrefix(authorization, "Bearer ")]
}

func (f *fakeMCP) do(fn func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn()
}

// An MCP connector connects with no client of its own: Archive finds the server's authorization server,
// registers itself there once, connects with PKCE for the resource, and archive.action calls the server's
// tools with the account's token, refreshed with the registered client, which the run tool's docs list for
// whoever connected it alone; its sync lists its pages with them; disconnecting revokes with that client and
// deletes the records.
func TestAnMCPConnectorRegistersConnectsAndActs(t *testing.T) {
	t.Parallel()
	f := newFakeMCP(t)
	in := newInst(t, "ARCHIVE_MCP_NOTION="+f.srv.URL+"/mcp")
	in.must("user", "add", "alice@example.com")
	in.must("user", "add", "bob@example.com")
	srv := in.serve()
	api := srv.url + "/api"
	ws := in.psql("archive", "SELECT id FROM archive.workspace ORDER BY created_at LIMIT 1")
	alice := decode[Token](t, in.must("token", "create")).Secret
	bob := decode[Token](t, in.must("-as", "bob@example.com", "token", "create")).Secret

	start := func(tok string) url.Values {
		t.Helper()
		st, hdr := call(t, "GET", srv.url+"/connect/notion?workspace="+ws, tok, nil, nil)
		loc, err := url.Parse(hdr.Get("Location"))
		if st != 302 || err != nil || loc.Host != strings.TrimPrefix(f.srv.URL, "http://") || loc.Path != "/authorize" {
			t.Fatalf("start connecting Notion: %d %s", st, hdr.Get("Location"))
		}
		return loc.Query()
	}
	q := start(alice)
	if q.Get("client_id") != "client-1" || q.Get("resource") != f.srv.URL+"/mcp" || q.Get("scope") != "default" || q.Get("code_challenge_method") != "S256" ||
		!strings.HasSuffix(q.Get("redirect_uri"), "/connect/notion/callback") {
		t.Fatalf("the consent Notion is sent to: %v", q)
	}
	if q2 := start(bob); q2.Get("client_id") != "client-1" {
		t.Errorf("a second connect registered again: %v", q2)
	}
	f.do(func() {
		if len(f.registered) != 1 || f.registered[0]["token_endpoint_auth_method"] != "client_secret_post" || f.registered[0]["client_name"] != "Archive" ||
			fmt.Sprint(f.registered[0]["redirect_uris"]) != "["+q.Get("redirect_uri")+"]" {
			t.Errorf("registered: %v", f.registered)
		}
	})
	if plain := in.psql("archive", "SELECT count(*) FROM archive.oauth_client WHERE provider = 'notion' AND position('secret-1'::bytea IN client) = 0"); plain != "1" {
		t.Errorf("the registered client is not kept sealed: %s", plain)
	}

	st, hdr := call(t, "GET", srv.url+"/connect/notion/callback?"+url.Values{"state": {q.Get("state")}, "code": {"alice@example.com"}}.Encode(), alice, nil, nil)
	if to := hdr.Get("Location"); st != 303 || strings.Contains(to, "error") || !strings.HasSuffix(to, "/connectors/notion?workspace="+ws) {
		t.Fatalf("alice connects her Notion: %d %s", st, to)
	}
	f.do(func() {
		code := f.asked[0]
		if code.Get("resource") != f.srv.URL+"/mcp" || oauth2.S256ChallengeFromVerifier(code.Get("code_verifier")) != q.Get("code_challenge") {
			t.Errorf("the code was traded without its resource or verifier: %v", code)
		}
	})
	var cs Connections
	call(t, "GET", srv.url+"/connect", alice, nil, &cs)
	if len(cs.Connections) != 1 || cs.Connections[0].Account != "alice@example.com" || cs.Connections[0].Provider != "notion" {
		t.Fatalf("alice's connections: %+v", cs)
	}

	run := func(tok, code string) RunOutput {
		t.Helper()
		st, out := e2eRun(t, api, tok, code)
		if st != 200 {
			t.Fatalf("run %s: %d %+v", code, st, out)
		}
		return out
	}
	if out := run(alice, `return await archive.action("notion.search", { query: "roadmap" })`); out.Error != "" ||
		string(out.Result) != `{"results":[{"by":"alice@example.com","title":"Page about roadmap"}]}` {
		t.Errorf("alice searches her Notion: %+v", out)
	}
	if out := run(alice, `return await archive.action("notion.search", { query: "again" }, { connection: "alice@example.com" })`); out.Error != "" {
		t.Errorf("a second search, naming the connection: %+v", out)
	}
	f.do(func() {
		if n := len(f.asked); n != 2 || f.asked[1].Get("grant_type") != "refresh_token" || f.asked[1].Get("client_id") != "client-1" {
			t.Errorf("the token requests: %v", f.asked)
		}
	})
	for code, want := range map[string]string{
		`archive.action("notion.search", {})`:                                  `invalid invalid archive.action notion.search input: `,
		`archive.action("notion.search", { query: "boom" })`:                   `undefined archive.action notion.search: Notion said: notion-search: the search index is down`,
		`archive.action("notion.edit", {})`:                                    `not_found archive.action notion.edit: Notion has no action edit; it has get-self, list-private-pages, search`,
		`archive.action("notion.search", { query: "x" }, { connection: "z" })`: `not_found archive.action notion.search: you have no Notion connection "z"`,
		`archive.action("gmail.search", { query: "x" })`:                       `not_found archive.action gmail.search: Gmail is not set up on this server; it needs ARCHIVE_GOOGLE`,
		`archive.action("search", {})`:                                         `invalid archive.action search: name is "<connector>.<action>"`,
	} {
		out := run(alice, `return await `+code+`.then(() => "worked", (e) => e.code + " " + e.message)`)
		got := ""
		json.Unmarshal(out.Result, &got)
		if !strings.HasPrefix(got, want) {
			t.Errorf("%s: %q, want %q", code, got, want)
		}
	}
	if out := run(bob, `return await archive.action("notion.search", { query: "x" }).catch((e) => e.code + " " + e.message)`); string(out.Result) != `"not_found archive.action notion.search: you have not connected Notion; connect it in Settings, Connectors"` {
		t.Errorf("bob, who has not connected Notion: %+v", out)
	}

	docs := func(tok string) string {
		t.Helper()
		req, _ := http.NewRequest("GET", api+"/run/docs", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return string(b)
	}
	mine := docs(alice)
	for _, want := range []string{`action<A extends keyof Actions>(name: A, input: Actions[A]["input"]`, "Notion alice@example.com", `"notion.search": {`,
		"/** Search pages. */", "/** Words to find. */\n            query: string;", "limit?: number;", "results: null | Record<string, string>[];"} {
		if !strings.Contains(mine, want) {
			t.Errorf("alice's docs lack %q:\n%s", want, mine[strings.Index(mine, "interface Actions"):])
		}
	}
	if theirs := docs(bob); strings.Contains(theirs, "notion.search") || !strings.Contains(theirs, "You have connected none") {
		t.Errorf("bob's docs list alice's actions")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	for tok, want := range map[string]bool{alice: true, bob: false} {
		cl := mcp.NewClient(&mcp.Implementation{Name: "e2e", Version: "1"}, nil)
		sess, err := cl.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: srv.url + "/mcp", HTTPClient: &http.Client{Transport: e2eBearer(tok)}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		tools, err := sess.ListTools(ctx, nil)
		if err != nil || len(tools.Tools) != 1 || strings.Contains(tools.Tools[0].Description, `"notion.search"`) != want {
			t.Errorf("the run tool of Archive's own MCP server lists notion.search: want %v", want)
		}
		sess.Close()
	}

	id := cs.Connections[0].ID
	notionSyncs(t, in, f, id)
	if st, _ := call(t, "DELETE", srv.url+"/connect/"+id.String(), alice, nil, nil); st != 204 {
		t.Fatalf("alice disconnects her Notion: %d", st)
	}
	if n := in.psql("archive", "SELECT count(*) FROM archive.record"); n != "0" {
		t.Errorf("disconnecting left %s records", n)
	}
	f.do(func() {
		if len(f.revoked) != 1 || !strings.HasPrefix(f.revoked[0].Get("token"), "rt-") || f.revoked[0].Get("client_id") != "client-1" || f.revoked[0].Get("client_secret") != "secret-1" {
			t.Errorf("disconnecting revoked %v", f.revoked)
		}
		if len(f.refresh) != 0 {
			t.Errorf("refresh tokens left live: %v", f.refresh)
		}
	})
}
