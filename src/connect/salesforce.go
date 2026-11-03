package connect

import (
	"cmp"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// Salesforce's CRM, through its REST API: an org's accounts, contacts, leads and opportunities. It reads
// ARCHIVE_SALESFORCE, an External Client App (or Connected App) whose callback URL is
// <API>/connect/salesforce/callback, with the OAuth scopes api and refresh_token, and PKCE on;
// https://<id>:<secret>@test.salesforce.com in place of login.salesforce.com connects sandboxes. An owner
// connects the org for everyone. Each org has its own host, which the user info of a token names.
//
// It syncs into the crm tables as HubSpot does: an account as a company, the organization of its website's
// domain, a contact or an open lead as a contact, the person of their email, and an opportunity as a deal. Each
// is listed whole the first time and a week after, which deletes what is gone for good, and in between
// queryAll reads what changed since the sync before, deleted too, while the recycle bin still has it, about
// two weeks; a sync older than that lists whole again. It pushes nothing: Salesforce's change events need a
// streaming client, not a webhook.
func init() {
	Register(&Connector{
		ID: "salesforce", Name: "Salesforce", Integration: true, Env: "ARCHIVE_SALESFORCE", Host: "https://login.salesforce.com",
		Hosts: []string{"https://login.salesforce.com"},
		Endpoint: oauth2.Endpoint{AuthURL: "https://login.salesforce.com/services/oauth2/authorize",
			TokenURL: "https://login.salesforce.com/services/oauth2/token", AuthStyle: oauth2.AuthStyleInParams},
		Revoke: "https://login.salesforce.com/services/oauth2/revoke",
		Scopes: []string{"api", "refresh_token"},
		Params: map[string]string{"prompt": "login consent"},
		Account: func(c *Client) (string, error) {
			base, err := sfBase(c)
			if err != nil {
				return "", err
			}
			var out struct {
				Records []struct {
					Name string `json:"Name"`
				} `json:"records"`
			}
			if err := c.Get(base+"/query?q="+url.QueryEscape("SELECT Name FROM Organization"), &out); err != nil {
				return "", err
			}
			if len(out.Records) == 0 {
				return "", errors.New("Salesforce named no organization")
			}
			return out.Records[0].Name, nil
		},
		Actions: map[string]*Action{
			"query": Act("Run a SOQL query, like { query: \"SELECT Id, Name, StageName, Amount FROM Opportunity WHERE IsClosed = false\" }: "+
				"{ totalSize, done, records, nextRecordsUrl } (pass nextRecordsUrl as page for more)",
				func(c *Client, in struct {
					Query string `json:"query,omitempty" jsonschema:"The SOQL query"`
					Page  string `json:"page,omitempty" jsonschema:"The nextRecordsUrl of the query before"`
				}) (any, error) {
					base, err := sfBase(c)
					if err != nil {
						return nil, err
					}
					u := base + "/query?q=" + url.QueryEscape(in.Query)
					if in.Page != "" {
						if !strings.HasPrefix(in.Page, "/services/data/") {
							return nil, errors.New("page is the nextRecordsUrl of a query before")
						}
						u = sfHost(base) + in.Page
					}
					var out any
					err = c.Get(u, &out)
					return out, err
				}),
			"search": Act("Find records by words in any field, with SOSL, like { query: \"FIND {Acme} IN ALL FIELDS RETURNING Account(Id, Name), Contact(Id, Name, Email)\" }: "+
				"{ searchRecords: [{ attributes, Id, ... }] }",
				func(c *Client, in struct {
					Query string `json:"query" jsonschema:"The SOSL search"`
				}) (any, error) {
					base, err := sfBase(c)
					if err != nil {
						return nil, err
					}
					var out any
					err = c.Get(base+"/search?q="+url.QueryEscape(in.Query), &out)
					return out, err
				}),
		},
		Streams: []*Stream{
			sfStream("accounts", "Account", "Name, Website, Industry, Type, Phone, BillingCity, BillingState, BillingCountry, NumberOfEmployees, "+
				"AnnualRevenue, Description, CreatedDate", "crm_company", sfAccountRows),
			sfStream("contacts", "Contact", "FirstName, LastName, Email, Phone, Title, AccountId, Account.Name, MailingCity, MailingState, "+
				"MailingCountry, CreatedDate", "crm_contact", sfContactRows),
			sfStream("leads", "Lead", "FirstName, LastName, Email, Phone, Title, Company, Status, City, State, Country, IsConverted, CreatedDate",
				"crm_contact", sfLeadRows),
			sfStream("opportunities", "Opportunity", "Name, Amount, StageName, CloseDate, IsClosed, IsWon, AccountId, Description, CreatedDate",
				"crm_deal", sfOpportunityRows),
		},
	})
}

// sfVersion is the version of the REST API the connector reads.
const sfVersion = "v62.0"

// sfBase is the REST API of the org a client's token is for, like https://acme.my.salesforce.com/services/data/v62.0,
// as the token's user info names it.
func sfBase(c *Client) (string, error) {
	var ui struct {
		URLs struct {
			REST string `json:"rest"`
		} `json:"urls"`
	}
	if err := c.Get("https://login.salesforce.com/services/oauth2/userinfo", &ui); err != nil {
		return "", err
	}
	if ui.URLs.REST == "" {
		return "", errors.New("Salesforce did not name the org's API")
	}
	return sfHost(ui.URLs.REST) + "/services/data/" + sfVersion, nil
}

// sfHost is the scheme and host of u.
func sfHost(u string) string {
	if p, err := url.Parse(u); err == nil {
		return p.Scheme + "://" + p.Host
	}
	return u
}

// sfRelist is how long after a full listing the next lists whole again, to see what is gone for good.
const sfRelist = 7 * 24 * time.Hour

// sfBin is how far back queryAll still sees deleted records, a day short of the recycle bin's 15.
const sfBin = 14 * 24 * time.Hour

// sfCursor is where a stream is: Since when the sync its records are as of began, Listed when its last full
// listing began, and while a sync is under way, Mode list or delta, Began when it began and Next the
// nextRecordsUrl of its query.
type sfCursor struct {
	Since  string `json:"since,omitempty"`
	Listed string `json:"listed,omitempty"`
	Mode   string `json:"mode,omitempty"`
	Began  string `json:"began,omitempty"`
	Next   string `json:"next,omitempty"`
}

// sfStream is the stream of the object obj, with fields, into table.
func sfStream(name, obj, fields, table string, rows func(map[string]any) *Row) *Stream {
	return &Stream{Name: name, Gap: time.Second, Table: table, Rows: rows,
		Doc: "Every " + obj + ", by its Id, with " + fields + ": listed whole the first time and a week after, 2000 a page, which deletes " +
			"those gone for good, and in between what queryAll says changed since the sync before, deleted too.",
		Pull: func(c *Client, cursor string) (*Page, error) {
			var cur sfCursor
			if err := cursorOf(cursor, &cur); err != nil {
				return nil, err
			}
			base, err := sfBase(c)
			if err != nil {
				return nil, err
			}
			now := time.Now().Add(-time.Minute).UTC()
			if cur.Mode == "" {
				cur.Mode, cur.Began, cur.Next = "delta", now.Format(time.RFC3339), ""
				if since := stamp(cur.Since); since.IsZero() || now.Sub(since) > sfBin || now.Sub(stamp(cur.Listed)) > sfRelist {
					cur.Mode = "list"
				}
			}
			u := sfHost(base) + cur.Next
			switch {
			case cur.Next != "":
			case cur.Mode == "list":
				u = base + "/query?q=" + url.QueryEscape("SELECT Id, SystemModstamp, "+fields+" FROM "+obj)
			default:
				u = base + "/queryAll?q=" + url.QueryEscape("SELECT Id, SystemModstamp, IsDeleted, "+fields+" FROM "+obj+" WHERE SystemModstamp > "+cur.Since)
			}
			var res struct {
				Records []map[string]any `json:"records"`
				Next    string           `json:"nextRecordsUrl"`
			}
			if err := c.Get(u, &res); err != nil {
				// A query's next page lasts about 15 minutes idle; one gone starts the sync again.
				if r := (*Refusal)(nil); errors.As(err, &r) && cur.Next != "" && (r.Status == http.StatusBadRequest || r.Status == http.StatusNotFound) {
					return &Page{More: true, Cursor: cursorFor(sfCursor{Since: cur.Since, Listed: cur.Listed})}, nil
				}
				if r := (*Refusal)(nil); errors.As(err, &r) && r.Status == http.StatusBadRequest {
					return nil, unsupported("Salesforce does not let Archive read %s in this org: %s", obj, r.Message)
				}
				return nil, err
			}
			page := &Page{Full: cur.Mode == "list"}
			for _, m := range res.Records {
				id := str(m, "Id")
				if m["IsDeleted"] == true {
					page.Records = append(page.Records, Record{ID: id, Deleted: true})
					continue
				}
				page.Records = append(page.Records, Record{ID: id, Data: m, Updated: stamp(sfTime(str(m, "SystemModstamp")))})
			}
			if res.Next != "" {
				cur.Next, page.More = res.Next, true
			} else {
				if cur.Mode == "list" {
					cur.Listed = cur.Began
				}
				cur = sfCursor{Since: cur.Began, Listed: cur.Listed}
			}
			page.Cursor = cursorFor(cur)
			return page, nil
		}}
}

// sfTime is a Salesforce time, like 2026-11-03T15:04:05.000+0000, as RFC 3339.
func sfTime(s string) string {
	if t, err := time.Parse("2006-01-02T15:04:05.000-0700", s); err == nil {
		return t.UTC().Format(time.RFC3339)
	}
	return s
}

func sfEdited(m map[string]any) any { return instant(sfTime(str(m, "SystemModstamp"))) }

// sfAccountRows is an account as a company, the organization of its website's domain.
func sfAccountRows(m map[string]any) *Row {
	domain := Domain(str(m, "Website"))
	return rowOf(map[string]any{"title": str(m, "Name"), "domain": domain, "industry": str(m, "Industry"), "stage": str(m, "Type"),
		"phone": str(m, "Phone"), "city": str(m, "BillingCity"), "state": str(m, "BillingState"), "country": str(m, "BillingCountry"),
		"employees": decimal(m["NumberOfEmployees"]), "revenue": money(m["AnnualRevenue"], "USD"), "description": clip(str(m, "Description"), maxText),
		"made_at": instant(sfTime(str(m, "CreatedDate"))), "edited_at": sfEdited(m)}).org("org", domain)
}

// sfContactRows is a contact, the person of their email, with their account as their company.
func sfContactRows(m map[string]any) *Row {
	email := Email(str(m, "Email"))
	return rowOf(map[string]any{"first_name": str(m, "FirstName"), "last_name": str(m, "LastName"), "email": email, "phone": str(m, "Phone"),
		"job_title": str(m, "Title"), "company_name": str(obj(m, "Account"), "Name"), "stage": "contact", "city": str(m, "MailingCity"),
		"state": str(m, "MailingState"), "country": str(m, "MailingCountry"), "made_at": instant(sfTime(str(m, "CreatedDate"))), "edited_at": sfEdited(m)}).
		person("person", email).ref("company", str(m, "AccountId"))
}

// sfLeadRows is a lead as a contact whose stage is lead, until it is converted into a contact, which is then
// the contact instead.
func sfLeadRows(m map[string]any) *Row {
	if m["IsConverted"] == true {
		return nil
	}
	email := Email(str(m, "Email"))
	return rowOf(map[string]any{"first_name": str(m, "FirstName"), "last_name": str(m, "LastName"), "email": email, "phone": str(m, "Phone"),
		"job_title": str(m, "Title"), "company_name": str(m, "Company"), "stage": "lead", "lead_status": str(m, "Status"), "city": str(m, "City"),
		"state": str(m, "State"), "country": str(m, "Country"), "made_at": instant(sfTime(str(m, "CreatedDate"))), "edited_at": sfEdited(m)}).
		person("person", email)
}

// sfOpportunityRows is an opportunity as a deal with its account as its company.
func sfOpportunityRows(m map[string]any) *Row {
	return rowOf(map[string]any{"title": str(m, "Name"), "amount": money(m["Amount"], cmp.Or(str(m, "CurrencyIsoCode"), "USD")), "stage": str(m, "StageName"),
		"closed": flag(m, "IsClosed"), "won": flag(m, "IsWon"), "close_on": day(str(m, "CloseDate")), "description": clip(str(m, "Description"), maxText),
		"made_at": instant(sfTime(str(m, "CreatedDate"))), "edited_at": sfEdited(m)}).ref("company", str(m, "AccountId"))
}
