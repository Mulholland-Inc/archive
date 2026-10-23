package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// A field can cite the record that backs it, such as a birth date and the baptism register it came
// from, with the words as written and the page. A citation is read only by those who can read
// both the record and its source.
func TestAFieldCitesTheRecordThatBacksIt(t *testing.T) {
	in := newInst(t, "ARCHIVE_ADMIN_EMAIL=alice@example.com")
	srv := in.serve()
	api := srv.url + "/api"
	in.must("user", "add", "bob@example.com")
	in.must("spec", "push", in.writeFile("spec.json", `{"types": {
	  "ancestor": {"fields": {"name": {"type": "text", "required": true}, "born": {"type": "fuzzy_date"}}},
	  "register": {"fields": {"title": {"type": "text", "required": true}}}}}`))
	in.must("space", "add", "family")
	in.must("space", "grant", "family", "bob@example.com", "viewer")
	alice := decode[Token](t, in.must("token", "create", "-full")).Secret
	bob := decode[Token](t, in.must("-as", "bob@example.com", "token", "create", "-full")).Secret

	var person, register, diary e2eRecord
	call(t, "POST", api+"/ancestor?space=family", alice, map[string]any{"name": "Mordant Quist", "born": "abt 1832"}, &person)
	call(t, "POST", api+"/register?space=family", alice, map[string]any{"title": "Gloamshire baptisms 1830-1840"}, &register)
	call(t, "POST", api+"/register?space=family", alice, map[string]any{"title": "Hesper's diary"}, &diary)
	call(t, "PUT", api+"/register/"+diary.ID+"/restricted", alice, map[string]any{"restricted": true}, nil)

	cite := func(tok string, body any) (int, []Provenance) {
		var raw json.RawMessage
		st, _ := call(t, "POST", api+"/ancestor/"+person.ID+"/provenance", tok, body, &raw)
		var ps []Provenance
		if st == 201 {
			json.Unmarshal(raw, &ps)
		}
		return st, ps
	}
	st, ps := cite(alice, map[string]any{"citations": []map[string]any{
		{"field": "born", "source": register.ID, "original": "bapt. 3 March 1832, son of Thaddeus", "page": 12},
		{"field": "born", "source": diary.ID, "original": "born the winter before the flood"},
	}})
	wantStatus(t, st, 201, "alice cites two sources")
	var got []string
	for _, p := range ps {
		if p.Method == "cite" && p.SourceID != nil {
			got = append(got, p.SourceID.String()+"|"+p.Original)
		}
	}
	if len(got) != 2 || !strings.Contains(strings.Join(got, ","), register.ID+"|bapt. 3 March 1832") {
		t.Fatalf("citations: %v", got)
	}

	t.Run("AViewerSeesOnlyCitationsOfWhatTheyCanRead", func(t *testing.T) {
		var ps []Provenance
		call(t, "GET", api+"/ancestor/"+person.ID+"/provenance", bob, nil, &ps)
		var srcs []string
		for _, p := range ps {
			if p.SourceID != nil {
				srcs = append(srcs, p.SourceID.String())
			}
		}
		if len(srcs) != 1 || srcs[0] != register.ID {
			t.Fatalf("bob sees citations of %v", srcs)
		}
	})

	t.Run("OnlyAnEditorCitesAndOnlyWhatTheyCanRead", func(t *testing.T) {
		st, _ := cite(bob, map[string]any{"citations": []map[string]any{{"field": "born", "source": register.ID}}})
		wantStatus(t, st, 403, "a viewer cites")
		in.must("space", "grant", "family", "bob@example.com", "editor")
		st, _ = cite(bob, map[string]any{"citations": []map[string]any{{"field": "born", "source": diary.ID}}})
		wantStatus(t, st, 422, "an editor cites a source they cannot read")
	})

	t.Run("ACitationNamesAFieldTheTypeHas", func(t *testing.T) {
		st, _ := cite(alice, map[string]any{"citations": []map[string]any{{"field": "shoe_size", "source": register.ID}}})
		wantStatus(t, st, 422, "an unknown field")
		st, _ = cite(alice, map[string]any{"citations": []map[string]any{{"field": "born"}}})
		wantStatus(t, st, 422, "no source")
	})

	t.Run("OverMCP", func(t *testing.T) {
		out, isErr := mcpCall(t, srv.url, alice, "cite", map[string]any{"type": "ancestor", "id": person.ID, "citations": []map[string]any{{"field": "name", "source": register.ID, "original": "Mordant, son of Thaddeus Quist"}}})
		if isErr || !strings.Contains(out, "Mordant, son of Thaddeus Quist") {
			t.Fatalf("mcp cite: %v %s", isErr, out)
		}
	})

	t.Run("DeletingTheSourceKeepsTheWordsAndDropsTheLink", func(t *testing.T) {
		st, _ := call(t, "DELETE", api+"/register/"+register.ID, alice, nil, nil)
		wantStatus(t, st, 204, "delete the register")
		var ps []Provenance
		call(t, "GET", api+"/ancestor/"+person.ID+"/provenance", alice, nil, &ps)
		var kept bool
		for _, p := range ps {
			if p.Original == "bapt. 3 March 1832, son of Thaddeus" {
				kept = p.SourceID == nil
			}
		}
		if !kept {
			t.Errorf("after deleting the source: %+v", ps)
		}
	})
}
