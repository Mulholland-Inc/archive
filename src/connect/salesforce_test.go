package connect

import (
	"net/http"
	"slices"
	"strings"
	"testing"
)

// sfFake is an org on the fake's own host, as the token's user info names it, whose query answers with
// answer, given how many queries it has had.
func sfFake(t *testing.T, answer func(n int, w http.ResponseWriter, r *http.Request)) *fake {
	var f *fake
	n := 0
	f = newFake(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/services/oauth2/userinfo":
			writeJSON(w, map[string]any{"urls": map[string]any{"rest": f.URL + "/services/data/v{version}/"}})
		case strings.HasPrefix(r.URL.Path, "/services/data/"+sfVersion+"/query"):
			n++
			answer(n, w, r)
		default:
			t.Errorf("asked %s", r.URL.Path)
			http.NotFound(w, r)
		}
	})
	return f
}

func sfAccount(id string) map[string]any {
	return map[string]any{"Id": id, "Name": "Account " + id, "SystemModstamp": "2026-10-01T10:00:00.000+0000"}
}

// A listing whose next page has expired, as a query locator does after about 15 minutes idle, starts again
// from the top, and the listing after that ends. Before #290 the expired locator's 400 was taken for an object
// the org does not let Archive read, and the stream stalled on it every sync.
func TestSalesforceExpiredLocatorStartsAgain(t *testing.T) {
	f := sfFake(t, func(n int, w http.ResponseWriter, r *http.Request) {
		switch {
		case n == 1:
			writeJSON(w, map[string]any{"records": []any{sfAccount("A")}, "nextRecordsUrl": "/services/data/" + sfVersion + "/query/01g-2000"})
		case n == 2:
			if r.URL.Path != "/services/data/"+sfVersion+"/query/01g-2000" {
				t.Errorf("the second query asked %s, want the next page", r.URL.Path)
			}
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, []any{map[string]any{"errorCode": "INVALID_QUERY_LOCATOR", "message": "invalid query locator"}})
		default:
			if r.URL.Query().Get("q") == "" {
				t.Errorf("query %d asked %s, want the listing from the top", n, r.URL)
			}
			writeJSON(w, map[string]any{"records": []any{sfAccount("A"), sfAccount("B")}})
		}
	})
	c := f.open(t, "salesforce", "", nil, nil)
	pages, cursor := pull(t, c, stream(t, "salesforce", "accounts"), "")
	if got, want := ids(pages...), []string{"A", "A", "B"}; !slices.Equal(got, want) {
		t.Errorf("listed %v, want %v", got, want)
	}
	if len(pages) != 3 {
		t.Fatalf("%d pages, want 3", len(pages))
	}
	if p := pages[1]; p.Full || len(p.Records) > 0 || !p.More {
		t.Errorf("the page that starts again is Full %v with %d records, More %v; want only More", p.Full, len(p.Records), p.More)
	}
	if !pages[2].Full {
		t.Error("the listing that starts again is not Full")
	}
	if !strings.Contains(cursor, `"since"`) || !strings.Contains(cursor, `"listed"`) || strings.Contains(cursor, `"next"`) {
		t.Errorf("after the listing the cursor is %s, want since and listed", cursor)
	}
}

// A 400 on a sync's first query is still the org refusing the object, not a locator to start again from.
func TestSalesforceRefusedObjectIsUnsupported(t *testing.T) {
	f := sfFake(t, func(n int, w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, []any{map[string]any{"errorCode": "INVALID_TYPE", "message": "sObject type 'Lead' is not supported"}})
	})
	c := f.open(t, "salesforce", "", nil, nil)
	if _, err := stream(t, "salesforce", "leads").Pull(c, ""); err == nil || !strings.Contains(err.Error(), "does not let Archive read Lead") {
		t.Errorf("a refused object is %v", err)
	}
}

// A variable naming a sandbox's login server moves only login.salesforce.com onto it; the org's API, on its
// own host, as the user info names it, stays there. Before #290 every URL moved onto test.salesforce.com,
// which serves no org's API.
func TestSalesforceSandboxMovesOnlyTheLoginHost(t *testing.T) {
	ps, err := Load(func(k string) string {
		if k == "ARCHIVE_SALESFORCE" {
			return "https://id:secret@test.salesforce.com"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := ps.Get("salesforce")
	if err != nil {
		t.Fatal(err)
	}
	for u, want := range map[string]string{
		"https://login.salesforce.com/services/oauth2/userinfo":                     "https://test.salesforce.com/services/oauth2/userinfo",
		"https://acme--dev.sandbox.my.salesforce.com/services/data/v62.0/query?q=x": "https://acme--dev.sandbox.my.salesforce.com/services/data/v62.0/query?q=x",
	} {
		if got := p.at(u); got != want {
			t.Errorf("%s is at %s, want %s", u, got, want)
		}
	}
	if got, want := p.endpoint.TokenURL, "https://test.salesforce.com/services/oauth2/token"; got != want {
		t.Errorf("token URL %s, want %s", got, want)
	}
}
