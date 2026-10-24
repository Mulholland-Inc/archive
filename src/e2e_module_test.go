package archive

import (
	"io"
	"net/http"
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
