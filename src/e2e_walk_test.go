package archive

import (
	"slices"
	"strings"
	"testing"
)

// One call walks links of given kinds and directions, round after round, as a pedigree needs:
// from a person in through a parent link's child field, then out through its parent field, six
// generations up in one request. Restricted records are neither returned nor walked through.
func TestWalkingLinksGivesAPedigreeInOneCall(t *testing.T) {
	in := newInst(t, "ARCHIVE_ADMIN_EMAIL=alice@example.com")
	srv := in.serve()
	api := srv.url + "/api"
	in.must("user", "add", "bob@example.com")
	in.must("spec", "push", in.writeFile("spec.json", `{"types": {
	  "kin": {"fields": {"name": {"type": "text", "required": true}}},
	  "parent_link": {"fields": {"child": {"type": "reference", "to": "kin", "required": true}, "parent": {"type": "reference", "to": "kin", "required": true}}}}}`))
	in.must("space", "add", "family")
	in.must("space", "grant", "family", "bob@example.com", "viewer")
	alice := decode[Token](t, in.must("token", "create", "-full")).Secret
	bob := decode[Token](t, in.must("-as", "bob@example.com", "token", "create", "-full")).Secret

	ids := map[string]string{}
	add := func(name string) {
		var r e2eRecord
		st, _ := call(t, "POST", api+"/kin?space=family", alice, map[string]any{"name": name}, &r)
		wantStatus(t, st, 201, "add "+name)
		ids[name] = r.ID
	}
	parent := func(child, p string) {
		st, _ := call(t, "POST", api+"/parent_link?space=family", alice, map[string]any{"child": ids[child], "parent": ids[p]}, nil)
		wantStatus(t, st, 201, p+" is a parent of "+child)
	}
	for _, n := range []string{"Juniper", "Odo", "Hazel", "Alder", "Fenna", "Thaddeus", "Marigold", "Tobiah", "Elowen"} {
		add(n)
	}
	parent("Juniper", "Odo")
	parent("Juniper", "Hazel")
	parent("Odo", "Alder")
	parent("Odo", "Fenna")
	parent("Alder", "Thaddeus")
	parent("Alder", "Marigold")
	parent("Hazel", "Tobiah")
	parent("Hazel", "Elowen")

	type walked struct {
		Records []struct {
			ID    string         `json:"id"`
			Type  string         `json:"type"`
			Data  map[string]any `json:"data"`
			Depth int            `json:"depth"`
		} `json:"records"`
		Links []Link `json:"links"`
	}
	walk := func(tok, from, query string) (int, string) {
		var w walked
		st, _ := call(t, "GET", api+"/kin/"+ids[from]+"/walk?"+query, tok, nil, &w)
		var out []string
		for _, r := range w.Records {
			if r.Type == "kin" {
				out = append(out, r.Data["name"].(string)+"@"+string(rune('0'+r.Depth)))
			}
		}
		slices.Sort(out)
		return st, strings.Join(out, ",")
	}

	t.Run("Ancestors", func(t *testing.T) {
		st, got := walk(alice, "Juniper", "steps=in:child,out:parent&depth=6")
		wantStatus(t, st, 200, "walk")
		want := "Alder@2,Elowen@2,Fenna@2,Hazel@1,Juniper@0,Marigold@3,Odo@1,Thaddeus@3,Tobiah@2"
		if got != want {
			t.Fatalf("ancestors:\n got %s\nwant %s", got, want)
		}
		if _, got := walk(alice, "Juniper", "steps=in:child,out:parent&depth=1"); got != "Hazel@1,Juniper@0,Odo@1" {
			t.Errorf("one generation: %s", got)
		}
	})

	t.Run("Descendants", func(t *testing.T) {
		if _, got := walk(alice, "Thaddeus", "steps=in:parent,out:child&depth=6"); got != "Alder@1,Juniper@3,Odo@2,Thaddeus@0" {
			t.Fatalf("descendants: %s", got)
		}
	})

	t.Run("NeitherReturnsNorPassesThroughARestrictedRecord", func(t *testing.T) {
		st, _ := call(t, "PUT", api+"/kin/"+ids["Hazel"]+"/restricted", alice, map[string]any{"restricted": true}, nil)
		wantStatus(t, st, 200, "restrict Hazel")
		if _, got := walk(bob, "Juniper", "steps=in:child,out:parent&depth=6"); got != "Alder@2,Fenna@2,Juniper@0,Marigold@3,Odo@1,Thaddeus@3" {
			t.Fatalf("bob walks: %s", got)
		}
	})

	t.Run("RefusesWhatItCannotDo", func(t *testing.T) {
		for _, q := range []string{"steps=up:child", "steps=", "steps=in:child&depth=99"} {
			if st, _ := walk(alice, "Juniper", q); st != 422 {
				t.Errorf("%s: %d", q, st)
			}
		}
	})

	t.Run("OverMCP", func(t *testing.T) {
		out, isErr := mcpCall(t, srv.url, alice, "walk", map[string]any{"type": "kin", "id": ids["Juniper"], "steps": []string{"in:child", "out:parent"}, "depth": 2})
		if isErr || !strings.Contains(out, "Fenna") || strings.Contains(out, "Thaddeus") {
			t.Fatalf("mcp walk: %v %s", isErr, out)
		}
	})
}
