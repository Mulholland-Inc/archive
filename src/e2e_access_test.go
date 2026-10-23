package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Sources that run a program or read the server's own disk are the instance admin's alone.
func TestOnlyInstanceAdminsAddSourcesThatRunProgramsOrReadTheServersDisk(t *testing.T) {
	in := newInst(t, "ARCHIVE_ADMIN_EMAIL=alice@example.com")
	srv := in.serve()
	api := srv.url + "/api"

	in.must("user", "add", "bob@example.com", "-name", "Bob")
	in.must("spec", "push", in.writeSpec())
	in.must("space", "add", "family")
	in.must("space", "grant", "family", "bob@example.com", "viewer")
	bobTok := decode[Token](t, in.must("-as", "bob@example.com", "token", "create")).Secret
	narrowTok := decode[Token](t, in.must("token", "create", "-name", "Narrow", "-space", "family:editor")).Secret
	aliceTok := decode[Token](t, in.must("token", "create", "-name", "Claude", "-full")).Secret

	pwned := filepath.Join(in.dir, "pwned")
	command := map[string]any{"kind": "external", "name": "x", "config": map[string]any{"command": []string{"sh", "-c", "touch " + pwned}}}
	folder := map[string]any{"kind": "folder", "name": "etc", "type": "invoice", "config": map[string]any{"path": "/etc"}}

	t.Run("OthersCannotAddACommandSource", func(t *testing.T) {
		for who, tok := range map[string]string{"a user": bobTok, "the admin's token without admin rights": narrowTok} {
			st, _ := call(t, "POST", api+"/sources", tok, command, nil)
			wantStatus(t, st, 403, who+" adds an external source")
			if _, err := os.Stat(pwned); err == nil {
				t.Fatalf("%s ran a program on the server", who)
			}
		}
	})

	t.Run("OthersCannotAddAFolderSource", func(t *testing.T) {
		st, _ := call(t, "POST", api+"/sources", bobTok, folder, nil)
		wantStatus(t, st, 403, "a user adds a folder source")
		if n := in.on(t).psql("archive", "SELECT count(*) FROM raw"); n != "0" {
			t.Fatalf("a user read %s files off the server's disk", n)
		}
	})

	t.Run("OthersCannotIntakeAFolderThroughMCP", func(t *testing.T) {
		out, isErr := mcpCall(t, srv.url, bobTok, "intake", map[string]any{"dir": "/etc", "type": "invoice"})
		if !isErr || !strings.Contains(out, "instance admin") {
			t.Fatalf("mcp intake of /etc as a user: %v %s", isErr, out)
		}
	})

	t.Run("TheInstanceAdminStillCan", func(t *testing.T) {
		st, _ := call(t, "POST", api+"/sources", aliceTok, folder, nil)
		wantStatus(t, st, 201, "the admin adds a folder source")
	})
}

// Changing the ontology rewrites records in every space, so it needs the instance admin, not
// merely admin of some space; every user owns their home space.
func TestOnlyInstanceAdminsChangeTheOntology(t *testing.T) {
	in := newInst(t, "ARCHIVE_ADMIN_EMAIL=alice@example.com")
	srv := in.serve()
	api := srv.url + "/api"

	in.must("user", "add", "bob@example.com", "-name", "Bob")
	in.must("spec", "push", in.writeSpec())
	aliceTok := decode[Token](t, in.must("token", "create", "-name", "Claude", "-full")).Secret
	bobTok := decode[Token](t, in.must("-as", "bob@example.com", "token", "create")).Secret
	var fam e2eRecord
	st, _ := call(t, "POST", api+"/contact", aliceTok, map[string]any{"name": "Alice", "phone": "+31612345678"}, &fam)
	wantStatus(t, st, 201, "alice creates a contact")

	drop := map[string]any{"ops": []Op{{Op: "remove_field", Type: "contact", Field: "phone"}}}
	var c Change
	st, _ = call(t, "POST", api+"/changes", bobTok, drop, &c)
	wantStatus(t, st, 201, "a user proposes a change")
	st, _ = call(t, "POST", api+"/changes/"+c.ID.String()+"/apply", bobTok, nil, nil)
	if st == 200 {
		t.Fatalf("a user applied a change that removes a field from everyone's records")
	}
	if got := in.psql("archive", "SELECT data->>'phone' FROM records WHERE type = 'contact'"); got != "+31612345678" {
		t.Fatalf("alice's contact lost its phone: %q", got)
	}

	st, _ = call(t, "POST", api+"/changes/"+c.ID.String()+"/apply", aliceTok, nil, nil)
	wantStatus(t, st, 200, "the instance admin applies it")
}
