package archive

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func (in *e2eInst) writeFile(name, content string) string {
	p := filepath.Join(in.dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		in.t.Fatal(err)
	}
	return p
}

// A restricted record is read only by its space's admins and by the user who made it, through
// every surface: the API, links, provenance, search, MCP, files and Postgres itself. A type can
// say its records start restricted, as a family archive does for people who may be living.
func TestRestrictedRecordsHideFromEveryoneButAdminsAndTheirMaker(t *testing.T) {
	in := newInst(t, "ARCHIVE_ADMIN_EMAIL=alice@example.com")
	srv := in.serve()
	api := srv.url + "/api"
	in.must("user", "add", "bob@example.com", "-name", "Bob")
	in.must("user", "add", "carol@example.com", "-name", "Carol")
	spec := `{"types": {
	  "relative": {"restricted": true, "fields": {"name": {"type": "text", "required": true}}},
	  "letter": {"fields": {"title": {"type": "text", "required": true}, "about": {"type": "reference", "to": "relative"}}}}}`
	in.must("spec", "push", in.writeFile("spec.json", spec))
	in.must("space", "add", "family")
	in.must("space", "grant", "family", "bob@example.com", "viewer")
	in.must("space", "grant", "family", "carol@example.com", "editor")
	alice := decode[Token](t, in.must("token", "create", "-full")).Secret
	bob := decode[Token](t, in.must("-as", "bob@example.com", "token", "create", "-full")).Secret
	carol := decode[Token](t, in.must("-as", "carol@example.com", "token", "create", "-full")).Secret

	var living, dead, letter, tobin e2eRecord
	st, _ := call(t, "POST", api+"/relative?space=family", alice, map[string]any{"name": "Juniper Quist"}, &living)
	wantStatus(t, st, 201, "alice adds a living relative")
	call(t, "POST", api+"/relative?space=family", alice, map[string]any{"name": "Mordant Quist"}, &dead)
	st, _ = call(t, "PUT", api+"/relative/"+dead.ID+"/restricted", alice, map[string]any{"restricted": false}, &dead)
	wantStatus(t, st, 200, "the admin opens a record")
	call(t, "POST", api+"/letter?space=family", alice, map[string]any{"title": "A letter from Gloamshire", "about": living.ID}, &letter)
	st, _ = call(t, "POST", api+"/relative?space=family", carol, map[string]any{"name": "Tobin Quist"}, &tobin)
	wantStatus(t, st, 201, "an editor adds a relative")
	st, f := e2eUpload(t, api+"/files?space=family", alice, "juniper.txt", []byte("Juniper's school report"))
	wantStatus(t, st, 201, "upload")
	st, _ = call(t, "PUT", api+"/file/"+f.ID.String()+"/restricted", alice, map[string]any{"restricted": true}, nil)
	wantStatus(t, st, 200, "the admin restricts a file")

	names := func(tok string) string {
		var rs []e2eRecord
		call(t, "GET", api+"/relative", tok, nil, &rs)
		return names(rs)
	}

	t.Run("NewRecordsOfARestrictedTypeStartRestricted", func(t *testing.T) {
		if living.Data == nil || !strings.Contains(in.on(t).psql("archive", "SELECT restricted FROM records WHERE id = '"+living.ID+"'"), "t") {
			t.Fatalf("a new relative is not restricted")
		}
	})

	t.Run("AViewerSeesOnlyWhatIsOpen", func(t *testing.T) {
		if got := names(bob); got != "Mordant Quist" {
			t.Fatalf("bob lists %q", got)
		}
		st, _ := call(t, "GET", api+"/relative/"+living.ID, bob, nil, nil)
		wantStatus(t, st, 404, "bob reads a restricted relative")
		st, _ = call(t, "GET", api+"/relative/"+living.ID+"/provenance", bob, nil, nil)
		if st == 200 {
			t.Errorf("bob reads a restricted relative's provenance")
		}
		var links []map[string]any
		call(t, "GET", api+"/letter/"+letter.ID+"/links", bob, nil, &links)
		if len(links) != 0 {
			t.Errorf("bob sees links to a restricted relative: %v", links)
		}
		if hits := e2eSearch(t, api, bob, "Juniper"); len(hits) != 0 {
			t.Errorf("bob finds %s", e2eTitles(hits))
		}
		if hits := e2eSearch(t, api, alice, "Juniper"); len(hits) == 0 {
			t.Errorf("alice finds nothing, so search proves nothing")
		}
		if len(links) == 0 {
			call(t, "GET", api+"/letter/"+letter.ID+"/links", alice, nil, &links)
			if len(links) == 0 {
				t.Errorf("alice sees no links either, so links prove nothing")
			}
		}
		if got, _ := mcpCall(t, srv.url, bob, "query", map[string]any{"type": "relative"}); strings.Contains(got, "Juniper") || strings.Contains(got, "Tobin") {
			t.Errorf("bob over mcp: %s", got)
		}
	})

	t.Run("AViewerCannotReadARestrictedFile", func(t *testing.T) {
		for _, part := range []string{"", "/content", "/thumbnail", "/text"} {
			resp, _ := e2eGet(t, api+"/files/"+f.ID.String()+part, bob)
			if resp.StatusCode == 200 {
				t.Errorf("bob reads the restricted file's %q", part)
			}
			if part != "/thumbnail" {
				if resp, _ := e2eGet(t, api+"/files/"+f.ID.String()+part, alice); resp.StatusCode != 200 {
					t.Errorf("alice cannot read the file's %q either: %d", part, resp.StatusCode)
				}
			}
		}
	})

	t.Run("AnEditorSeesWhatTheyMadeButNotOthers", func(t *testing.T) {
		if got := names(carol); got != "Mordant Quist,Tobin Quist" {
			t.Fatalf("carol lists %q", got)
		}
		st, _ := call(t, "PATCH", api+"/relative/"+living.ID, carol, map[string]any{"name": "Changed"}, nil)
		wantStatus(t, st, 404, "carol edits a restricted relative she did not add")
		st, _ = call(t, "PUT", api+"/relative/"+tobin.ID+"/restricted", carol, map[string]any{"restricted": false}, nil)
		wantStatus(t, st, 403, "an editor opens a record")
	})

	t.Run("TheAdminSeesEverything", func(t *testing.T) {
		if got := names(alice); got != "Juniper Quist,Mordant Quist,Tobin Quist" {
			t.Fatalf("alice lists %q", got)
		}
	})

	t.Run("PostgresItselfHidesThem", func(t *testing.T) {
		in := in.on(t)
		role := in.psql("archive", "SELECT role FROM users WHERE email = 'bob@example.com'")
		in.psql("archive", "ALTER ROLE "+role+" LOGIN")
		defer in.psql("archive", "ALTER ROLE "+role+" NOLOGIN")
		in.psql("archive", "INSERT INTO proposals (type, fields, confidence, status, space) SELECT 'relative', '[]', 1, 'review', id FROM spaces WHERE name = 'family'")
		for what, sql := range map[string]string{
			"records":    "SELECT count(*) FROM records WHERE restricted",
			"files":      "SELECT count(*) FROM files WHERE name = 'juniper.txt'",
			"provenance": "SELECT count(*) FROM provenance p JOIN records r ON r.id = p.record_id WHERE r.data->>'name' <> 'Mordant Quist' AND r.type = 'relative'",
			"proposals":  "SELECT count(*) FROM proposals WHERE type = 'relative'",
			"links":      "SELECT count(*) FROM links WHERE kind = 'about'",
		} {
			if n := in.psql(role, sql); n != "0" {
				t.Errorf("psql as bob sees %s restricted %s", n, what)
			}
			if n := in.psql("archive", sql); n == "0" {
				t.Errorf("the test proves nothing about %s: there are none to hide", what)
			}
		}
		if n := in.psql(role, "SELECT count(*) FROM records r WHERE r.type = 'relative'"); n != "1" {
			t.Errorf("psql as bob sees %s relatives, want 1", n)
		}
	})
}
