package connect

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"testing"
	"time"
)

// hubspotSearchFake is contacts searched as HubSpot searches them: those changed at or after the filter's
// value, by lastmodifieddate ascending, 100 a page from the offset after. changed is when each contact last
// changed, in Unix milliseconds, given how many searches there have been.
func hubspotSearchFake(t *testing.T, changed func(n int) map[string]int64) (*fake, *[]map[string]any) {
	var bodies []map[string]any
	n := 0
	f := newFake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/crm/v3/objects/contacts/search" {
			t.Errorf("asked %s %s, want a search", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("a search body: %v", err)
			return
		}
		bodies = append(bodies, body)
		from, _ := strconv.ParseInt(filterValue(body), 10, 64)
		n++
		at := changed(n)
		var all []string
		for id, ms := range at {
			if ms >= from {
				all = append(all, id)
			}
		}
		slices.SortFunc(all, func(a, b string) int {
			if at[a] != at[b] {
				return int(at[a] - at[b])
			}
			x, _ := strconv.Atoi(a)
			y, _ := strconv.Atoi(b)
			return x - y
		})
		after, _ := body["after"].(string)
		off, _ := strconv.Atoi(after)
		var results []any
		for i := off; i < len(all) && i < off+100; i++ {
			results = append(results, map[string]any{"id": all[i], "updatedAt": time.UnixMilli(at[all[i]]).UTC().Format(time.RFC3339Nano),
				"properties": map[string]any{"lastmodifieddate": time.UnixMilli(at[all[i]]).UTC().Format(time.RFC3339Nano)}})
		}
		res := map[string]any{"results": results}
		if off+100 < len(all) {
			res["paging"] = map[string]any{"next": map[string]any{"after": strconv.Itoa(off + 100)}}
		}
		writeJSON(w, res)
	})
	return f, &bodies
}

// A contact that changes while a search pages on moves to its end, and the rest a place back; the next page
// searches from the last change read, not from an offset, so none moves out of reach. Before #290 the second
// page was the search's offset 100, which by then began a contact later, and that one was never read.
func TestHubSpotSearchPagesFromTheLastChange(t *testing.T) {
	since := time.Now().Add(-time.Hour).UnixMilli()
	at := map[string]int64{}
	for i := 1; i <= 150; i++ {
		at[strconv.Itoa(i)] = since + int64(i)*1000
	}
	f, bodies := hubspotSearchFake(t, func(n int) map[string]int64 {
		if n == 2 {
			at["5"] = since + 500*1000 // contact 5 changes after the first page is read
		}
		return at
	})
	c := f.open(t, "hubspot", "", nil, nil)
	cursor := cursorFor(hubspotCursor{Since: since, Listed: time.Now().Add(-time.Hour).UnixMilli()})
	pages, _ := pull(t, c, stream(t, "hubspot", "contacts"), cursor)
	seen := map[string]bool{}
	for _, id := range ids(pages...) {
		seen[id] = true
	}
	for i := 1; i <= 150; i++ {
		if id := strconv.Itoa(i); !seen[id] {
			t.Errorf("contact %s was not read", id)
		}
	}
	if len(*bodies) < 2 {
		t.Fatalf("%d searches, want at least 2", len(*bodies))
	}
	second := (*bodies)[1]
	if after, ok := second["after"]; ok {
		t.Errorf("the second page searched from offset %v, want from the last change", after)
	}
	if got, want := filterValue(second), fmt.Sprint(since+100*1000); got != want {
		t.Errorf("the second page searched from %s, want %s, contact 100's change", got, want)
	}
	for _, p := range pages {
		if p.Full {
			t.Error("a search page is Full")
		}
	}
}

// filterValue is the value of a search body's first filter.
func filterValue(body map[string]any) string {
	gs, _ := body["filterGroups"].([]any)
	if len(gs) == 0 {
		return ""
	}
	g, _ := gs[0].(map[string]any)
	fs, _ := g["filters"].([]any)
	if len(fs) == 0 {
		return ""
	}
	f, _ := fs[0].(map[string]any)
	return fmt.Sprint(f["value"])
}
