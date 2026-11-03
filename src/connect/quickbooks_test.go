package connect

import (
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// qboFake is a company whose customers are ids, listed by query in pages as QuickBooks lists them; orderBy says
// whether it sorts by id or, like some companies, answers ORDERBY Id with a 400.
func qboFake(t *testing.T, customers func() []int, orderBy bool) *fake {
	at := regexp.MustCompile(`STARTPOSITION (\d+) MAXRESULTS (\d+)`)
	return newFake(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v3/company/42/") {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query().Get("query")
		if strings.Contains(q, "ORDERBY Id") && !orderBy {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]any{"Fault": map[string]any{"Error": []any{map[string]any{"Message": "ORDERBY not supported"}}}})
			return
		}
		m := at.FindStringSubmatch(q)
		if m == nil {
			t.Errorf("a query without a page: %s", q)
			return
		}
		start, _ := strconv.Atoi(m[1])
		size, _ := strconv.Atoi(m[2])
		all := customers()
		var page []any
		for i := start - 1; i >= 0 && i < len(all) && len(page) < size; i++ {
			page = append(page, map[string]any{"Id": strconv.Itoa(all[i]), "DisplayName": fmt.Sprint("Customer ", all[i]),
				"MetaData": map[string]any{"LastUpdatedTime": "2026-10-01T10:00:00-07:00"}})
		}
		writeJSON(w, map[string]any{"QueryResponse": map[string]any{"Customer": page}})
	})
}

func span(from, to int) []int {
	var out []int
	for i := from; i <= to; i++ {
		out = append(out, i)
	}
	return out
}

// A company that will not sort by id is listed in its own order, and that listing keeps what it read. Before
// #290 the fallback emptied the page after reading it, and a full listing of nothing deleted every row.
func TestQuickBooksFallbackKeepsWhatItReads(t *testing.T) {
	f := qboFake(t, func() []int { return span(1, 3) }, false)
	c := f.open(t, "quickbooks", "", nil, map[string]string{"realmId": "42"})
	pages, _ := pull(t, c, stream(t, "quickbooks", "customers"), "")
	if got, want := ids(pages...), []string{"Customer:1", "Customer:2", "Customer:3"}; !slices.Equal(got, want) {
		t.Errorf("listed %v, want %v", got, want)
	}
	for _, p := range pages {
		if !p.Full {
			t.Error("a listing's page is not Full")
		}
	}
}

// A customer deleted while a listing pages on moves the ones after it a place back; pages overlap so none is
// skipped, which a full listing would delete.
func TestQuickBooksListingSkipsNoneWhenOneIsDeleted(t *testing.T) {
	all := span(1, 1500)
	read := 0
	f := qboFake(t, func() []int {
		read++
		if read == 2 {
			all = slices.Delete(all, 9, 10) // customer 10 goes after the first page is read
		}
		return all
	}, true)
	c := f.open(t, "quickbooks", "", nil, map[string]string{"realmId": "42"})
	pages, cursor := pull(t, c, stream(t, "quickbooks", "customers"), "")
	seen := map[string]bool{}
	for _, id := range ids(pages...) {
		seen[id] = true
	}
	for _, n := range all {
		if id := fmt.Sprint("Customer:", n); !seen[id] {
			t.Errorf("%s was not listed, so the listing would delete it", id)
		}
	}
	if len(pages) != 2 {
		t.Errorf("%d pages, want 2", len(pages))
	}
	if !strings.Contains(cursor, `"since"`) || strings.Contains(cursor, `"at"`) {
		t.Errorf("after the listing the cursor is %s, want only since", cursor)
	}
}

// After a listing, the next sync asks CDC what changed since, and a customer it says is deleted is a deleted
// record, in a page that deletes nothing else.
func TestQuickBooksChangesAfterAListing(t *testing.T) {
	f := newFake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v3/company/42/cdc" {
			t.Errorf("asked %s, want CDC", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("entities") != "Customer" || r.URL.Query().Get("changedSince") == "" {
			t.Errorf("CDC asked with %v", r.URL.Query())
		}
		writeJSON(w, map[string]any{"CDCResponse": []any{map[string]any{"QueryResponse": []any{map[string]any{"Customer": []any{
			map[string]any{"Id": "7", "DisplayName": "Renamed"},
			map[string]any{"Id": "8", "status": "Deleted"},
		}}}}}})
	})
	c := f.open(t, "quickbooks", "", nil, map[string]string{"realmId": "42"})
	since := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	p, err := stream(t, "quickbooks", "customers").Pull(c, cursorFor(qboCursor{Since: since}))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := ids(p), []string{"Customer:7", "-Customer:8"}; !slices.Equal(got, want) {
		t.Errorf("changes %v, want %v", got, want)
	}
	if p.Full || p.More {
		t.Errorf("a CDC page is Full %v, More %v; want neither", p.Full, p.More)
	}
}

// The sandbox option reads sandbox companies, on Intuit's sandbox host, which the fake stands in for too.
func TestQuickBooksSandboxHost(t *testing.T) {
	f := qboFake(t, func() []int { return span(1, 1) }, true)
	c := f.open(t, "quickbooks", "sandbox=1", nil, map[string]string{"realmId": "42"})
	if u := qbo(c, "/query"); !strings.HasPrefix(u, "https://sandbox-quickbooks.api.intuit.com/") {
		t.Errorf("sandbox URL %s", u)
	}
	pull(t, c, stream(t, "quickbooks", "customers"), "")
	if len(f.requests()) == 0 {
		t.Error("the fake was not asked")
	}
}
