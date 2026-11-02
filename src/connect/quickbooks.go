package connect

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// QuickBooks Online, through Intuit's accounting API. It reads ARCHIVE_QUICKBOOKS, an app's production keys
// from developer.intuit.com, whose redirect URIs list <API>/connect/quickbooks/callback; ?sandbox=1 on the
// variable, with the app's development keys, reads sandbox companies instead. Intuit has no read-only scope,
// so com.intuit.quickbooks.accounting is all of it, and the actions only read. The callback names the
// company, realmId, which every call needs; a person connects one company at a time, and connects again for
// another. Intuit replaces the refresh token about once a day.
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
		Push:    quickbooksPush,
		Streams: qboStreams(),
	})
}

// qboEntities are the entities QuickBooks streams, one stream each, named for it in lowercase.
var qboEntities = []string{"Account", "Bill", "BillPayment", "CreditMemo", "Customer", "Deposit", "Employee", "Estimate", "Invoice",
	"Item", "JournalEntry", "Payment", "Purchase", "PurchaseOrder", "SalesReceipt", "Vendor"}

// qboStreams are a stream of each of qboEntities: listed whole the first time, 1,000 a page by Id, and then
// its change data capture since, which says what was deleted too, but reaches back only 30 days, so a sync
// stopped for longer lists it whole again.
func qboStreams() []*Stream {
	var out []*Stream
	for _, e := range qboEntities {
		out = append(out, &Stream{Name: strings.ToLower(e), Pull: func(c *Client, cursor string) (*Page, error) { return qboPull(c, e, cursor) },
			Doc: "Every " + e + " of the company, as QuickBooks has it: listed whole the first time, 1,000 a page, and then its change data capture " +
				"since, deletions too; a sync stopped for over 30 days, as far back as that reaches, lists it whole again."})
	}
	return out
}

// qboCursor is where an entity's stream is: Since is when its listing began, or its last change data capture,
// less a minute, and Start the listing's next STARTPOSITION.
type qboCursor struct {
	Since string `json:"since"`
	Start int    `json:"start,omitempty"`
}

const qboPage = 1000

func qboPull(c *Client, entity, cursor string) (*Page, error) {
	var cur qboCursor
	if err := cursorOf(cursor, &cur); err != nil {
		return nil, err
	}
	now := time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)
	since, _ := time.Parse(time.RFC3339, cur.Since)
	if cur.Since == "" || cur.Start == 0 && time.Since(since) > 29*24*time.Hour {
		cur = qboCursor{Since: now, Start: 1}
	}
	if cur.Start > 0 {
		var out struct {
			QueryResponse map[string]any
		}
		q := fmt.Sprintf("SELECT * FROM %s ORDERBY Id STARTPOSITION %d MAXRESULTS %d", entity, cur.Start, qboPage)
		if err := c.Get(qbo(c, "/query?query="+url.QueryEscape(q)), &out); err != nil {
			return nil, err
		}
		rows := qboRecords(out.QueryResponse[entity])
		page := &Page{Full: true, More: len(rows) == qboPage, Records: rows}
		if page.More {
			cur.Start += qboPage
		} else {
			cur.Start = 0
		}
		page.Cursor = cursorFor(cur)
		return page, nil
	}
	var out struct {
		CDCResponse []struct {
			QueryResponse []map[string]any
		}
	}
	if err := c.Get(qbo(c, "/cdc?entities="+entity+"&changedSince="+url.QueryEscape(cur.Since)), &out); err != nil {
		return nil, err
	}
	page := &Page{}
	for _, r := range out.CDCResponse {
		for _, qr := range r.QueryResponse {
			page.Records = append(page.Records, qboRecords(qr[entity])...)
		}
	}
	cur.Since = now
	// Change data capture answers at most 1,000 of an entity; with that many, the next starts from the last.
	if len(page.Records) >= qboPage {
		last := since
		for _, r := range page.Records {
			if r.Updated.After(last) {
				last = r.Updated
			}
		}
		cur.Since, page.More = last.UTC().Format(time.RFC3339), true
	}
	page.Cursor = cursorFor(cur)
	return page, nil
}

// qboRecords are the entities rows, as QuickBooks answers them; one whose status is Deleted is deleted.
func qboRecords(rows any) []Record {
	var out []Record
	for _, r := range asList(rows) {
		m, _ := r.(map[string]any)
		if m == nil || str(m, "Id") == "" {
			continue
		}
		meta, _ := m["MetaData"].(map[string]any)
		out = append(out, Record{ID: str(m, "Id"), Data: m, Updated: stamp(str(meta, "LastUpdatedTime")), Deleted: m["status"] == "Deleted"})
	}
	return out
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
