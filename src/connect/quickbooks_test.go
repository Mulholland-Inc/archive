package connect

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// QuickBooks keeps the company its callback names, reads whose it is with it, and queries it in the minor
// version the actions read; its token endpoint takes the client as Basic auth.
func TestQuickBooksKeepsTheCompanyOfTheCallback(t *testing.T) {
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/oauth2/v1/tokens/bearer":
			if id, secret, _ := r.BasicAuth(); id != "app" || secret != "shh" {
				w.WriteHeader(401)
				return
			}
			w.Write([]byte(`{"access_token":"at","refresh_token":"rt","token_type":"bearer","expires_in":3600}`))
		case r.Header.Get("Authorization") != "Bearer at":
			w.WriteHeader(401)
		case r.URL.Path == "/v3/company/9130/companyinfo/9130":
			w.Write([]byte(`{"CompanyInfo":{"CompanyName":"Acme Ltd"}}`))
		case r.URL.Path == "/v3/company/9130/query":
			queries = append(queries, r.URL.Query().Get("query")+" v"+r.URL.Query().Get("minorversion"))
			w.Write([]byte(`{"QueryResponse":{"Invoice":[{"Id":"1","TotalAmt":10}]}}`))
		default:
			w.WriteHeader(404)
			w.Write([]byte(`{"Fault":{"Error":[{"Message":"no","Detail":"Object Not Found"}]}}`))
		}
	}))
	defer srv.Close()
	p, err := load(t, map[string]string{"ARCHIVE_QUICKBOOKS": strings.Replace(srv.URL, "://", "://app:shh@", 1) + "?sandbox=1"}).Get("quickbooks")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exchange(t.Context(), "https://api.example.com/cb", url.Values{"code": {"c"}}, "v"); err == nil || !strings.Contains(err.Error(), "came back without realmId") {
		t.Errorf("a callback without the company: %v", err)
	}
	g, err := p.Exchange(t.Context(), "https://api.example.com/cb", url.Values{"code": {"c"}, "realmId": {"9130"}}, "v")
	if err != nil || g.Account != "Acme Ltd" || g.Meta["realmId"] != "9130" || g.Refresh != "rt" {
		t.Fatalf("connecting QuickBooks: %+v %v", g, err)
	}
	tok, err := p.Refresh(t.Context(), g.Refresh)
	if err != nil {
		t.Fatal(err)
	}
	c := p.Open(t.Context(), tok, g.Meta)
	out, err := p.Actions["invoices"].Do(c, json.RawMessage(`{"customer":"O'Brien","unpaid":true}`))
	if b, _ := json.Marshal(out); err != nil || string(b) != `[{"Id":"1","TotalAmt":10}]` ||
		len(queries) != 1 || queries[0] != `SELECT * FROM Invoice WHERE CustomerRef = 'O\'Brien' AND Balance > '0' ORDERBY TxnDate DESC MAXRESULTS 100 v75` {
		t.Errorf("unpaid invoices: %s %v %q", b, err, queries)
	}
	c.Meta = map[string]string{"realmId": "1"}
	if _, err := p.Actions["company"].Do(c, json.RawMessage(`{}`)); err == nil || err.Error() != "QuickBooks refused: 404 Not Found: Object Not Found" {
		t.Errorf("another company: %v", err)
	}
}
