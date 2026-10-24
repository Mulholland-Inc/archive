package archive

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A product built on the engine registers a module and ships as one binary. The toy module uses
// every hook, and each one works through a real server: its type pack is applied by itself on
// start, its semantic type parses and checks, its connector syncs, its MCP tool and HTTP route run
// as the caller, and its pages are served and listed for the sidebar.
func TestAModuleAddsTypesConnectorsToolsRoutesAndPages(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "toymodule")
	if out, err := exec.Command("go", "build", "-o", bin, "./testdata/toymodule").CombinedOutput(); err != nil {
		t.Fatalf("build the toy module: %v\n%s", err, out)
	}
	in := newInst(t, "ARCHIVE_ADMIN_EMAIL=alice@example.com")
	in.bin = bin
	srv := in.serve()
	api := srv.url + "/api"
	tok := decode[Token](t, in.must("token", "create", "-full")).Secret

	t.Run("ThePackIsAppliedOnStartAndItsTypeParses", func(t *testing.T) {
		var n e2eRecord
		st, _ := call(t, "POST", api+"/note", tok, map[string]any{"text": "hello"}, &n)
		wantStatus(t, st, 201, "create a note")
		if n.Data["text"] != "HELLO" {
			t.Fatalf("note = %v", n.Data)
		}
	})

	t.Run("RestartingChangesNothing", func(t *testing.T) {
		before := in.on(t).must("spec", "pull")
		srv.stop()
		srv = in.serve()
		api = srv.url + "/api"
		if after := in.on(t).must("spec", "pull"); after != before {
			t.Errorf("the pack changed the spec again on restart")
		}
		var cs []Change
		call(t, "GET", api+"/changes", tok, nil, &cs)
		if len(cs) != 1 {
			t.Errorf("%d changes, want the pack's one", len(cs))
		}
	})

	t.Run("ItsConnectorSyncs", func(t *testing.T) {
		var s Source
		st, _ := call(t, "POST", api+"/sources", tok, map[string]any{"kind": "counter", "name": "counter"}, &s)
		wantStatus(t, st, 201, "add a counter source")
		var res SyncResult
		call(t, "POST", api+"/sources/"+s.ID.String()+"/run", tok, nil, &res)
		if res.New != 3 {
			t.Fatalf("sync: %+v", res)
		}
	})

	t.Run("ItsToolAndRouteRunAsTheCaller", func(t *testing.T) {
		if out, isErr := mcpCall(t, srv.url, tok, "toy_whoami", nil); isErr || !strings.Contains(out, "alice@example.com") {
			t.Errorf("mcp: %v %s", isErr, out)
		}
		var me map[string]string
		st, _ := call(t, "GET", api+"/toy/whoami", tok, nil, &me)
		if st != 200 || me["email"] != "alice@example.com" {
			t.Errorf("route: %d %v", st, me)
		}
		st, _ = call(t, "GET", api+"/toy/whoami", "", nil, nil)
		wantStatus(t, st, 401, "the route without a caller")
	})

	t.Run("ItsPagesAreServedAndListed", func(t *testing.T) {
		resp, err := http.Get(srv.url + "/toy/")
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 || !strings.Contains(string(b), "The toy module's page") {
			t.Fatalf("app page: %d %s", resp.StatusCode, b)
		}
		var apps []map[string]string
		call(t, "GET", api+"/apps", tok, nil, &apps)
		if len(apps) != 1 || apps[0]["path"] != "/toy/" || apps[0]["title"] != "Toy" {
			t.Errorf("apps: %v", apps)
		}
		// an address of the app's own, with no file behind it, loads the app; a missing file is still 404
		for path, want := range map[string]int{"/toy/some/route": 200, "/toy/missing.js": 404} {
			resp, err := http.Get(srv.url + path)
			if err != nil {
				t.Fatal(err)
			}
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != want || want == 200 && !strings.Contains(string(b), "The toy module's page") {
				t.Errorf("%s: %d", path, resp.StatusCode)
			}
		}
	})

	t.Run("ItsCommandImportsGrantsAndTheirClientsRefreshAsBefore", func(t *testing.T) {
		hash := func(s string) string { return authHash(s) }
		grants := fmt.Sprintf(`[
		  {"client_id": "elsewhere-public", "client_name": "An assistant", "redirect_uris": ["https://example.com/cb"], "email": "alice@example.com", "refresh_hash": %q},
		  {"client_id": "elsewhere-secret", "client_name": "A server", "redirect_uris": ["https://example.com/cb"], "auth_method": "client_secret_basic", "secret_hash": %q, "email": "alice@example.com", "refresh_hash": %q},
		  {"client_id": "elsewhere-public", "client_name": "Nobody here", "redirect_uris": ["https://example.com/cb"], "email": "nobody@example.com", "refresh_hash": %q}]`,
			hash("refresh-1"), hash("the-secret"), hash("refresh-2"), hash("refresh-3"))
		out, errs, err := in.run(grants, "import-grants")
		if err != nil || strings.TrimSpace(out) != "imported 2" {
			t.Fatalf("import-grants: %v %q %s", err, out, errs)
		}
		if out, _, _ := in.run(grants, "import-grants"); strings.TrimSpace(out) != "imported 0" {
			t.Errorf("importing again: %q", out)
		}
		refresh := func(client, secret, token string) int {
			form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {token}}
			if secret == "" {
				form.Set("client_id", client)
			}
			req, _ := http.NewRequest("POST", srv.url+"/oauth/token", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			if secret != "" {
				req.SetBasicAuth(client, secret)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			return resp.StatusCode
		}
		for _, c := range []struct {
			client, secret, token string
			want                  int
		}{
			{"elsewhere-public", "", "refresh-1", 200},
			{"elsewhere-secret", "", "refresh-2", 401},
			{"elsewhere-secret", "wrong", "refresh-2", 401},
			{"elsewhere-secret", "the-secret", "refresh-2", 200},
			{"elsewhere-public", "", "refresh-3", 400},
		} {
			if st := refresh(c.client, c.secret, c.token); st != c.want {
				t.Errorf("refresh %s with secret %q: %d, want %d", c.client, c.secret, st, c.want)
			}
		}
	})

	t.Run("AGuardedTypeIsWrittenByTheModuleOrAnAdminOnly", func(t *testing.T) {
		in.must("space", "add", "shared")
		in.must("user", "add", "bob@example.com")
		in.must("space", "grant", "shared", "bob@example.com", "editor")
		bob := decode[Token](t, in.must("-as", "bob@example.com", "token", "create")).Secret
		alice := decode[Token](t, in.must("token", "create", "-full")).Secret
		var v e2eRecord
		if st, _ := call(t, "POST", api+"/vow?space=shared", bob, map[string]any{"text": "mine"}, &v); st != 403 {
			t.Errorf("an editor writes a guarded type through the API: %d", st)
		}
		if out, isErr := mcpCall(t, srv.url, bob, "upsert", map[string]any{"type": "vow", "space": "shared", "data": map[string]any{"text": "mine"}}); !isErr {
			t.Errorf("an editor writes a guarded type through MCP: %s", out)
		}
		if st, _ := call(t, "POST", api+"/toy/vow?space=shared", bob, nil, &v); st != 201 {
			t.Fatalf("the module writes it as the editor: %d", st)
		}
		if st, _ := call(t, "PATCH", api+"/vow/"+v.ID, bob, map[string]any{"text": "changed"}, nil); st != 403 {
			t.Errorf("an editor changes a guarded record through the API: %d", st)
		}
		if st, _ := call(t, "DELETE", api+"/vow/"+v.ID, bob, nil, nil); st != 403 {
			t.Errorf("an editor deletes a guarded record through the API: %d", st)
		}
		if st, _ := call(t, "PATCH", api+"/vow/"+v.ID, alice, map[string]any{"text": "changed"}, nil); st != 200 {
			t.Errorf("the space's admin changes it through the API: %d", st)
		}
		// an agent drafts through the module's tools and never writes a guarded type itself, the admin's included
		if out, isErr := mcpCall(t, srv.url, alice, "upsert", map[string]any{"type": "vow", "id": v.ID, "data": map[string]any{"text": "the agent's"}}); !isErr {
			t.Errorf("the admin's agent writes a guarded type through MCP: %s", out)
		}
		if out, isErr := mcpCall(t, srv.url, alice, "delete", map[string]any{"type": "vow", "id": v.ID}); !isErr {
			t.Errorf("the admin's agent deletes a guarded record through MCP: %s", out)
		}
		if out, isErr := mcpCall(t, srv.url, alice, "upsert", map[string]any{"type": "note", "space": "shared", "data": map[string]any{"text": "fine"}}); isErr {
			t.Errorf("the admin's agent writes a type nobody guards: %s", out)
		}
	})

	t.Run("ItsPageIsTheFrontDoor", func(t *testing.T) {
		// a visitor not signed in sees the app's own page at /; someone signed in is sent into the app
		noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		resp, err := noRedirect.Get(srv.url + "/")
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 || !strings.Contains(string(b), "The toy module's page") {
			t.Errorf("/ for a visitor: %d %s", resp.StatusCode, b)
		}
		req, _ := http.NewRequest("GET", srv.url+"/", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err = noRedirect.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 303 || resp.Header.Get("Location") != "/toy/" {
			t.Errorf("/ for a member: %d %q", resp.StatusCode, resp.Header.Get("Location"))
		}
		// the dashboard's other pages are where they were
		resp, _ = http.Get(srv.url + "/records")
		b, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
		if strings.Contains(string(b), "The toy module's page") {
			t.Errorf("the dashboard's /records is the toy's page")
		}
	})

	t.Run("LlmsTxtSaysHowAnAgentConnectsAndEveryToolThereIs", func(t *testing.T) {
		resp, err := http.Get(srv.url + "/llms.txt")
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		txt := string(b)
		if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/plain") {
			t.Fatalf("llms.txt: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
		}
		for _, want := range []string{"# Toy Archive", "> A toy archive, for the engine's tests.", "/mcp, over Streamable HTTP", "/.well-known/oauth-authorization-server", "An access token lasts an hour;", "- `describe`: Describe the ontology", "- `toy_whoami`: ", "- `query`: "} {
			if !strings.Contains(txt, want) {
				t.Errorf("llms.txt lacks %q:\n%s", want, txt)
			}
		}
	})

	t.Run("ItsBrandDressesTheDashboard", func(t *testing.T) {
		var b map[string]string
		call(t, "GET", srv.url+"/api/brand", "", nil, &b)
		if b["name"] != "Toy Archive" || !strings.Contains(b["logo"], "<circle") {
			t.Errorf("brand: %v", b)
		}
		for path, want := range map[string]string{"/brand.css": "--brand: rgb(1, 2, 3)", "/brand/toy.woff2": "toy font bytes", "/favicon.svg": `r="5"`} {
			resp, err := http.Get(srv.url + path)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != 200 || !strings.Contains(string(body), want) {
				t.Errorf("%s: %d %q", path, resp.StatusCode, body)
			}
		}
	})

	t.Run("TheEngineAloneHasNoneOfIt", func(t *testing.T) {
		plain := newInst(t, "ARCHIVE_ADMIN_EMAIL=alice@example.com")
		srv := plain.serve()
		tok := decode[Token](t, plain.must("token", "create", "-full")).Secret
		st, _ := call(t, "POST", srv.url+"/api/note", tok, map[string]any{"text": "hello"}, nil)
		if st == 201 {
			t.Errorf("the engine alone made a note")
		}
		var apps []map[string]string
		call(t, "GET", srv.url+"/api/apps", tok, nil, &apps)
		if len(apps) != 0 {
			t.Errorf("apps: %v", apps)
		}
		var b map[string]string
		call(t, "GET", srv.url+"/api/brand", "", nil, &b)
		if b["name"] != "Archive" || b["logo"] != "" {
			t.Errorf("the engine alone wears a brand: %v", b)
		}
		resp, err := http.Get(srv.url + "/brand.css")
		if err != nil {
			t.Fatal(err)
		}
		css, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 || len(css) != 0 {
			t.Errorf("the engine alone serves brand.css: %d %q", resp.StatusCode, css)
		}
	})
}
