package connect

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/oauth2"
)

// QuickBooks Online, through Intuit's accounting API. It reads ARCHIVE_QUICKBOOKS, an app's production keys
// from developer.intuit.com, whose redirect URIs list <API>/connect/quickbooks/callback; ?sandbox=1 on the
// variable, with the app's development keys, reads sandbox companies instead. Intuit has no read-only scope,
// so com.intuit.quickbooks.accounting is all of it, and the actions only read. The callback names the
// company, realmId, which every call needs; a person connects one company at a time, and connects again for
// another. Intuit replaces the refresh token about once a day.
// It syncs nothing until its entities are mapped into the tables of archive.sql; its push waits for that too.
func init() {
	Register(&Connector{
		ID: "quickbooks", Name: "QuickBooks", Integration: true, Env: "ARCHIVE_QUICKBOOKS", Host: "https://appcenter.intuit.com",
		Endpoint: oauth2.Endpoint{AuthURL: "https://appcenter.intuit.com/connect/oauth2", TokenURL: "https://oauth.platform.intuit.com/oauth2/v1/tokens/bearer",
			AuthStyle: oauth2.AuthStyleInHeader},
		RevokeWith: func(rv *Revocation) error {
			id, secret := rv.Client()
			return rv.Send("POST", "https://developer.api.intuit.com/v2/oauth2/tokens/revoke", http.Header{"Authorization": {basic(id, secret)}}, map[string]string{"token": rv.Refresh})
		},
		Scopes: []string{"com.intuit.quickbooks.accounting"},
		Keep:   []string{"realmId"},
		Account: func(c *Client) (string, error) {
			var ci struct {
				CompanyInfo struct {
					CompanyName string
				}
			}
			err := c.Get(qbo(c, "/companyinfo/"+url.PathEscape(c.Meta["realmId"])), &ci)
			return ci.CompanyInfo.CompanyName, err
		},
		Actions: map[string]*Action{
			"query": Act("Run a QuickBooks query, its SQL-like language over one entity, like { query: \"SELECT * FROM Bill WHERE Balance > '0' MAXRESULTS 100\" }: "+
				"{ QueryResponse: { <Entity>: [...], startPosition, maxResults } }. Entities include Account, Bill, Customer, Estimate, Invoice, Item, "+
				"Payment, Purchase, Vendor; no joins, and at most 1000 rows, paged with STARTPOSITION",
				func(c *Client, in struct {
					Query string `json:"query" jsonschema:"The query, like SELECT * FROM Invoice WHERE TxnDate >= '2026-01-01' ORDERBY TxnDate DESC"`
				}) (any, error) {
					var out any
					err := c.Get(qbo(c, "/query?query="+url.QueryEscape(in.Query)), &out)
					return out, err
				}),
			"company": Act("The company connected: its CompanyInfo, with CompanyName, LegalName, CompanyAddr, Country, FiscalYearStartMonth and more",
				func(c *Client, in struct{}) (any, error) {
					var out struct {
						CompanyInfo any
					}
					err := c.Get(qbo(c, "/companyinfo/"+url.PathEscape(c.Meta["realmId"])), &out)
					return out.CompanyInfo, err
				}),
			"invoices": Act("Invoices, newest first, maybe only a customer's or only unpaid ones: [Invoice] with DocNumber, TxnDate, DueDate, CustomerRef, TotalAmt, Balance, Line",
				func(c *Client, in struct {
					Customer string `json:"customer,omitempty" jsonschema:"Only this customer's, by its Id"`
					Unpaid   bool   `json:"unpaid,omitempty" jsonschema:"Only those with a balance left"`
					Since    string `json:"since,omitempty" jsonschema:"Only those dated on or after this day, YYYY-MM-DD"`
					Limit    int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 1000; 100 when left out"`
				}) (any, error) {
					var where []string
					if in.Customer != "" {
						where = append(where, "CustomerRef = "+qboQuote(in.Customer))
					}
					if in.Unpaid {
						where = append(where, "Balance > '0'")
					}
					if in.Since != "" {
						where = append(where, "TxnDate >= "+qboQuote(in.Since))
					}
					return qboList(c, "Invoice", where, "TxnDate DESC", in.Limit)
				}),
			"customers": Act("Customers by name, maybe only those whose name starts with some text: [Customer] with Id, DisplayName, CompanyName, PrimaryEmailAddr, Balance, Active",
				func(c *Client, in struct {
					Name  string `json:"name,omitempty" jsonschema:"Only those whose DisplayName starts with this"`
					Limit int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 1000; 100 when left out"`
				}) (any, error) {
					var where []string
					if in.Name != "" {
						where = append(where, "DisplayName LIKE "+qboQuote(in.Name+"%"))
					}
					return qboList(c, "Customer", where, "DisplayName", in.Limit)
				}),
		},
		Push: quickbooksPush,
	})
}

// qbo is the URL of path in the company the client is for, in the minor version of the API the actions read.
func qbo(c *Client, path string) string {
	host := "https://quickbooks.api.intuit.com"
	if c.Option("sandbox") != "" {
		host = "https://sandbox-quickbooks.api.intuit.com"
	}
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	return host + "/v3/company/" + url.PathEscape(c.Meta["realmId"]) + path + sep + "minorversion=75"
}

func qboQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `\'`) + "'" }

func qboList(c *Client, entity string, where []string, order string, limit int) (any, error) {
	q := "SELECT * FROM " + entity
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += fmt.Sprintf(" ORDERBY %s MAXRESULTS %d", order, clamp(limit, 100, 1000))
	var out struct {
		QueryResponse map[string]any
	}
	if err := c.Get(qbo(c, "/query?query="+url.QueryEscape(q)), &out); err != nil {
		return nil, err
	}
	if rows, ok := out.QueryResponse[entity]; ok {
		return rows, nil
	}
	return []any{}, nil
}

// quickbooksPush is the Intuit app's webhooks, set up once for each of its environments: its endpoint
// <API>/push/quickbooks, the entities QuickBooks streams, and its verifier token the hook option of
// ARCHIVE_QUICKBOOKS. intuit-signature is the base64 HMAC-SHA256 of the body with the verifier token. A push is
// an array of CloudEvents, each with intuitaccountid, the realmId of the company it is for; one of the format
// before, { eventNotifications: [{ realmId }] }, is read too.
var quickbooksPush = &Push{
	Watch: func(c *Client, h Hook, old []Watch) ([]Watch, error) {
		return []Watch{{Key: c.Meta["realmId"]}}, nil
	},
	Hear: func(p *Provider, r *http.Request, at string, body []byte, secret func(string) string) (*Heard, error) {
		if p.HookSecret() == "" || !same(r.Header.Get("intuit-signature"), b64MAC(p.HookSecret(), string(body))) {
			return nil, forged("intuit-signature")
		}
		var keys []string
		if t := strings.TrimSpace(string(body)); strings.HasPrefix(t, "[") {
			var events []struct {
				Account string `json:"intuitaccountid"`
			}
			if err := readJSON(body, &events); err != nil {
				return nil, err
			}
			for _, e := range events {
				keys = append(keys, e.Account)
			}
		} else {
			var n struct {
				Notifications []struct {
					RealmID string `json:"realmId"`
				} `json:"eventNotifications"`
			}
			if err := readJSON(body, &n); err != nil {
				return nil, err
			}
			for _, e := range n.Notifications {
				keys = append(keys, e.RealmID)
			}
		}
		return &Heard{Keys: keysOf(keys...)}, nil
	}}
