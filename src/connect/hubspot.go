package connect

import (
	"cmp"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// HubSpot's CRM, read only: contacts, companies and deals. It reads ARCHIVE_HUBSPOT, an app of a HubSpot
// developer account whose redirect URLs list <API>/connect/hubspot/callback and whose required scopes are
// exactly the Scopes here. HubSpot's own v3 token endpoint takes the client in the form, and says whose
// token it is when asked with the client's secret.
//
// It syncs companies, contacts and deals into the crm tables: a company as the organization of its web domain, a
// contact as the person of their email, and a deal with its pipeline and stage by name and the company it is
// with. Each is listed whole the first time and a week after it last was, which deletes what HubSpot no longer
// has, and in between searched for what changed since the sync before, as its pushes say.
func init() {
	Register(&Connector{
		ID: "hubspot", Name: "HubSpot", Integration: true, Env: "ARCHIVE_HUBSPOT", Host: "https://app.hubspot.com",
		Endpoint: oauth2.Endpoint{AuthURL: "https://app.hubspot.com/oauth/authorize", TokenURL: "https://api.hubspot.com/oauth/v3/token",
			AuthStyle: oauth2.AuthStyleInParams},
		RevokeWith: func(rv *Revocation) error {
			id, secret := rv.Client()
			return rv.Form("https://api.hubapi.com/oauth/v3/token/revoke", nil,
				url.Values{"client_id": {id}, "client_secret": {secret}, "token": {rv.Refresh}, "token_type_hint": {"refresh_token"}})
		},
		Scopes: []string{"oauth", "crm.objects.contacts.read", "crm.objects.companies.read", "crm.objects.deals.read"},
		Account: func(c *Client) (string, error) {
			id, secret := c.Credentials()
			var t struct {
				User      string `json:"user"`
				HubDomain string `json:"hub_domain"`
			}
			err := c.Do("POST", "https://api.hubspot.com/oauth/v3/token/introspect",
				url.Values{"client_id": {id}, "client_secret": {secret}, "token_type_hint": {"access_token"}, "access_token": {c.Token()}}, &t)
			return cmp.Or(t.HubDomain, strings.ToLower(t.User)), err
		},
		Actions: map[string]*Action{
			"search": Act("Find contacts, companies or deals by words in their default text properties (names, emails, domains, phone numbers), "+
				"newest first: { results: [{ id, properties, createdAt, updatedAt }], next } (pass next as page for more)",
				func(c *Client, in struct {
					Type       string   `json:"type" jsonschema:"contacts, companies or deals"`
					Query      string   `json:"query,omitempty" jsonschema:"Words to find; every record when left out"`
					Properties []string `json:"properties,omitempty" jsonschema:"The properties to return, like firstname, email, amount or dealstage; HubSpot's defaults when left out"`
					Limit      int      `json:"limit,omitempty" jsonschema:"At most this many, 1 to 100; 25 when left out"`
					Page       string   `json:"page,omitempty" jsonschema:"The next of the search before, for the records after it"`
				}) (any, error) {
					if err := hubspotType(in.Type); err != nil {
						return nil, err
					}
					body := map[string]any{"limit": clamp(in.Limit, 25, 100), "sorts": []map[string]string{{"propertyName": "createdate", "direction": "DESCENDING"}}}
					if in.Query != "" {
						body["query"] = in.Query
					}
					if len(in.Properties) > 0 {
						body["properties"] = in.Properties
					}
					if in.Page != "" {
						body["after"] = in.Page
					}
					var out struct {
						Results []any `json:"results"`
						Paging  struct {
							Next struct {
								After string `json:"after"`
							} `json:"next"`
						} `json:"paging"`
					}
					out.Results = []any{}
					err := c.Do("POST", hubspotAPI+"/crm/v3/objects/"+in.Type+"/search", body, &out)
					return map[string]any{"results": out.Results, "next": out.Paging.Next.After}, err
				}),
			"get": Act("One contact, company or deal by id, with the properties asked for and the ids of the records it is associated with: "+
				"{ id, properties, associations, createdAt, updatedAt }",
				func(c *Client, in struct {
					Type         string   `json:"type" jsonschema:"contacts, companies or deals"`
					ID           string   `json:"id" jsonschema:"The record's id"`
					Properties   []string `json:"properties,omitempty" jsonschema:"The properties to return; HubSpot's defaults when left out"`
					Associations []string `json:"associations,omitempty" jsonschema:"The types of the records associated with it to list, like companies or deals"`
				}) (any, error) {
					if err := hubspotType(in.Type); err != nil {
						return nil, err
					}
					q := url.Values{}
					if len(in.Properties) > 0 {
						q.Set("properties", strings.Join(in.Properties, ","))
					}
					if len(in.Associations) > 0 {
						q.Set("associations", strings.Join(in.Associations, ","))
					}
					var out any
					err := c.Get(hubspotAPI+"/crm/v3/objects/"+in.Type+"/"+url.PathEscape(in.ID)+"?"+q.Encode(), &out)
					return out, err
				}),
		},
		Push: hubspotPush,
		Streams: []*Stream{
			hubspotStream("companies", "companies", "hs_lastmodifieddate", "crm_company", hubspotCompanyRows,
				"name,domain,website,industry,lifecyclestage,phone,city,state,country,numberofemployees,annualrevenue,description,createdate,hs_lastmodifieddate"),
			hubspotStream("contacts", "contacts", "lastmodifieddate", "crm_contact", hubspotContactRows,
				"firstname,lastname,email,phone,jobtitle,company,associatedcompanyid,lifecyclestage,hs_lead_status,city,state,country,createdate,lastmodifieddate"),
			hubspotStream("deals", "deals", "hs_lastmodifieddate", "crm_deal", hubspotDealRows,
				"dealname,amount,deal_currency_code,pipeline,dealstage,closedate,hs_is_closed,hs_is_closed_won,description,createdate,hs_lastmodifieddate"),
		},
	})
}

// hubspotRelist is how long after a full listing the next lists whole again, to see what was deleted.
const hubspotRelist = 7 * 24 * time.Hour

// hubspotSearchMost is how far a search pages: HubSpot answers no more than 10,000 of one, so a delta bigger
// than that lists whole instead.
const hubspotSearchMost = 10000

// hubspotCursor is where a stream is: Since when the sync its records are as of began, in Unix milliseconds,
// Listed when its last full listing began, and while a sync is under way, Mode list or search, Began when it
// began and After the next page.
type hubspotCursor struct {
	Since  int64  `json:"since,omitempty"`
	Listed int64  `json:"listed,omitempty"`
	Mode   string `json:"mode,omitempty"`
	Began  int64  `json:"began,omitempty"`
	After  string `json:"after,omitempty"`
	// Mark is where a search pages from: the last change it read, so a record that changes while it pages, and
	// moves to the end, moves no other out of reach.
	Mark int64 `json:"mark,omitempty"`
}

// hubspotStream is the stream of the object type obj, with props, into table, its delta searched by changed,
// the property of when a record last changed.
func hubspotStream(name, obj, changed, table string, rows func(map[string]any) *Row, props string) *Stream {
	return &Stream{Name: name, Gap: time.Second, Table: table, Rows: rows,
		Doc: "Every one of HubSpot's " + obj + ", by id, with " + strings.ReplaceAll(props, ",", ", ") + ": listed whole the first time and a week " +
			"after, 100 a page, which deletes those HubSpot no longer has, and searched in between for those changed since the sync before.",
		Pull: func(c *Client, cursor string) (*Page, error) {
			var cur hubspotCursor
			if err := cursorOf(cursor, &cur); err != nil {
				return nil, err
			}
			now := time.Now().Add(-time.Minute).UnixMilli()
			if cur.Mode == "" {
				cur.Mode, cur.Began, cur.After, cur.Mark = "search", now, "", 0
				if cur.Since == 0 || time.Since(time.UnixMilli(cur.Listed)) > hubspotRelist {
					cur.Mode = "list"
				}
			}
			var res struct {
				Results []map[string]any `json:"results"`
				Paging  struct {
					Next struct {
						After string `json:"after"`
					} `json:"next"`
				} `json:"paging"`
			}
			page := &Page{Full: cur.Mode == "list"}
			if cur.Mode == "list" {
				q := url.Values{"limit": {"100"}, "properties": {props}}
				if obj == "deals" {
					q.Set("associations", "companies")
				}
				if cur.After != "" {
					q.Set("after", cur.After)
				}
				if err := c.Get(hubspotAPI+"/crm/v3/objects/"+obj+"?"+q.Encode(), &res); err != nil {
					return nil, err
				}
			} else {
				if n, _ := strconv.Atoi(cur.After); n >= hubspotSearchMost-100 {
					cur = hubspotCursor{Since: cur.Since, Listed: cur.Listed, Mode: "list", Began: cur.Began}
					page.More, page.Cursor = true, cursorFor(cur)
					return page, nil
				}
				mark := max(cur.Mark, cur.Since)
				body := map[string]any{"limit": 100, "properties": strings.Split(props, ","),
					"filterGroups": []map[string]any{{"filters": []map[string]any{{"propertyName": changed, "operator": "GTE", "value": strconv.FormatInt(mark, 10)}}}},
					"sorts":        []map[string]string{{"propertyName": changed, "direction": "ASCENDING"}}}
				if cur.After != "" {
					body["after"] = cur.After
				}
				if err := c.Do("POST", hubspotAPI+"/crm/v3/objects/"+obj+"/search", body, &res); err != nil {
					return nil, err
				}
				if obj == "deals" {
					if err := hubspotDealCompanies(c, res.Results); err != nil {
						return nil, err
					}
				}
			}
			if obj == "deals" {
				if err := hubspotStages(c, res.Results); err != nil {
					return nil, err
				}
			}
			for _, m := range res.Results {
				page.Records = append(page.Records, Record{ID: str(m, "id"), Data: m, Updated: stamp(str(m, "updatedAt"))})
			}
			if res.Paging.Next.After != "" {
				cur.After, page.More = res.Paging.Next.After, true
				// A search goes on from the last change it read, unless every record of the page changed at once,
				// and then by offset, as before.
				if n := len(res.Results); cur.Mode == "search" && n > 0 {
					if last := stamp(str(obj(res.Results[n-1], "properties"), changed)).UnixMilli(); last > max(cur.Mark, cur.Since) {
						cur.Mark, cur.After = last, ""
					}
				}
			} else {
				if cur.Mode == "list" {
					cur.Listed = cur.Began
				}
				cur = hubspotCursor{Since: cur.Began, Listed: cur.Listed}
			}
			page.Cursor = cursorFor(cur)
			return page, nil
		}}
}

// hubspotDealCompanies adds to each deal the id of the first company it is associated with, as a search does
// not say, from one batch read of the associations of them all.
func hubspotDealCompanies(c *Client, deals []map[string]any) error {
	if len(deals) == 0 {
		return nil
	}
	inputs := make([]map[string]string, len(deals))
	for i, d := range deals {
		inputs[i] = map[string]string{"id": str(d, "id")}
	}
	var res struct {
		Results []struct {
			From struct {
				ID string `json:"id"`
			} `json:"from"`
			To []struct {
				ID hubspotID `json:"toObjectId"`
			} `json:"to"`
		} `json:"results"`
	}
	if err := c.Do("POST", hubspotAPI+"/crm/v4/associations/deals/companies/batch/read", map[string]any{"inputs": inputs}, &res); err != nil {
		return err
	}
	company := map[string]string{}
	for _, r := range res.Results {
		if len(r.To) > 0 {
			company[r.From.ID] = string(r.To[0].ID)
		}
	}
	for _, d := range deals {
		d["companyId"] = company[str(d, "id")]
	}
	return nil
}

// hubspotID is an id HubSpot sends as a number or a string, as its digits.
type hubspotID string

func (n *hubspotID) UnmarshalJSON(b []byte) error {
	*n = hubspotID(strings.Trim(string(b), `"`))
	return nil
}

// hubspotStages adds to each deal the names of its pipeline and stage, which it names by id. A HubSpot that
// will not list its pipelines leaves the names out.
func hubspotStages(c *Client, deals []map[string]any) error {
	var res struct {
		Results []struct {
			ID     string `json:"id"`
			Label  string `json:"label"`
			Stages []struct {
				ID    string `json:"id"`
				Label string `json:"label"`
			} `json:"stages"`
		} `json:"results"`
	}
	if err := c.Get(hubspotAPI+"/crm/v3/pipelines/deals", &res); err != nil {
		if Busy(err) {
			return err
		}
		return nil
	}
	pipes, stages := map[string]string{}, map[string]string{}
	for _, p := range res.Results {
		pipes[p.ID] = p.Label
		for _, st := range p.Stages {
			stages[p.ID+"/"+st.ID] = st.Label
		}
	}
	for _, d := range deals {
		ps := obj(d, "properties")
		d["pipelineLabel"] = cmp.Or(pipes[str(ps, "pipeline")], str(ps, "pipeline"))
		d["stageLabel"] = cmp.Or(stages[str(ps, "pipeline")+"/"+str(ps, "dealstage")], str(ps, "dealstage"))
	}
	return nil
}

// hubspotBool is a HubSpot property that is true or false, as it sends them as text.
func hubspotBool(ps map[string]any, k string) any {
	switch str(ps, k) {
	case "true":
		return true
	case "false":
		return false
	}
	return nil
}

// hubspotCompanyRows is a company, the organization of its domain, or else of its website's.
func hubspotCompanyRows(m map[string]any) *Row {
	ps := obj(m, "properties")
	domain := cmp.Or(Domain(str(ps, "domain")), Domain(str(ps, "website")))
	return rowOf(map[string]any{"title": str(ps, "name"), "domain": domain, "industry": str(ps, "industry"), "stage": str(ps, "lifecyclestage"),
		"phone": str(ps, "phone"), "city": str(ps, "city"), "state": str(ps, "state"), "country": str(ps, "country"),
		"employees": decimal(ps["numberofemployees"]), "revenue": money(ps["annualrevenue"], "USD"), "description": clip(str(ps, "description"), maxText),
		"made_at": instant(str(ps, "createdate")), "edited_at": instant(cmp.Or(str(ps, "hs_lastmodifieddate"), str(m, "updatedAt")))}).org("org", domain)
}

// hubspotContactRows is a contact, the person of their email, with the company HubSpot holds them to be with.
func hubspotContactRows(m map[string]any) *Row {
	ps := obj(m, "properties")
	email := Email(str(ps, "email"))
	return rowOf(map[string]any{"first_name": str(ps, "firstname"), "last_name": str(ps, "lastname"), "email": email, "phone": str(ps, "phone"),
		"job_title": str(ps, "jobtitle"), "company_name": str(ps, "company"), "stage": str(ps, "lifecyclestage"), "lead_status": str(ps, "hs_lead_status"),
		"city": str(ps, "city"), "state": str(ps, "state"), "country": str(ps, "country"), "made_at": instant(str(ps, "createdate")),
		"edited_at": instant(cmp.Or(str(ps, "lastmodifieddate"), str(m, "updatedAt")))}).
		person("person", email).ref("company", str(ps, "associatedcompanyid"))
}

// hubspotDealRows is a deal with its pipeline and stage by name, and the company it is with: the one a search
// found, or the first a listing names.
func hubspotDealRows(m map[string]any) *Row {
	ps := obj(m, "properties")
	company := str(m, "companyId")
	if company == "" {
		if cs := objects(obj(obj(m, "associations"), "companies")["results"], ""); len(cs) > 0 {
			company = str(cs[0], "id")
		}
	}
	return rowOf(map[string]any{"title": str(ps, "dealname"), "amount": money(ps["amount"], cmp.Or(str(ps, "deal_currency_code"), "USD")),
		"pipeline": str(m, "pipelineLabel"), "stage": str(m, "stageLabel"), "closed": hubspotBool(ps, "hs_is_closed"), "won": hubspotBool(ps, "hs_is_closed_won"),
		"close_on": day(str(ps, "closedate")), "description": clip(str(ps, "description"), maxText), "made_at": instant(str(ps, "createdate")),
		"edited_at": instant(cmp.Or(str(ps, "hs_lastmodifieddate"), str(m, "updatedAt")))}).ref("company", company)
}

const hubspotAPI = "https://api.hubapi.com"

func hubspotType(t string) error {
	if t != "contacts" && t != "companies" && t != "deals" {
		return fmt.Errorf("type is contacts, companies or deals, not %q", t)
	}
	return nil
}

// hubspotPush is the HubSpot app's webhooks, set up once on the app: its target URL <API>/push/hubspot, and
// subscriptions to the creation, deletion and property changes of contacts, companies and deals. A push is an
// array of events, each with the portalId of the account it is for, the hub_id of the account's token.
// X-HubSpot-Signature-v3 is the base64 HMAC-SHA256, with the app's client secret, of the method, the URL, the
// body and X-HubSpot-Request-Timestamp, which must be under 5 minutes old; an app that still signs v1 or v2,
// the hex SHA-256 of the secret and the body, or of the secret, method, URL and body, is checked that way.
var hubspotPush = &Push{
	Watch: func(c *Client, h Hook, old []Watch) ([]Watch, error) {
		id, secret := c.Credentials()
		var t struct {
			HubID int64 `json:"hub_id"`
		}
		if err := c.Do("POST", "https://api.hubspot.com/oauth/v3/token/introspect",
			url.Values{"client_id": {id}, "client_secret": {secret}, "token_type_hint": {"access_token"}, "access_token": {c.Token()}}, &t); err != nil {
			return nil, err
		}
		if t.HubID == 0 {
			return nil, fmt.Errorf("HubSpot named no hub of the token")
		}
		return []Watch{{Key: strconv.FormatInt(t.HubID, 10)}}, nil
	},
	Hear: func(p *Provider, r *http.Request, at string, body []byte, secret func(string) string) (*Heard, error) {
		key := p.secret
		uri := strings.NewReplacer("%3A", ":", "%2F", "/", "%3F", "?", "%40", "@", "%21", "!", "%24", "$", "%27", "'", "%28", "(", "%29", ")",
			"%2A", "*", "%2C", ",", "%3B", ";").Replace(at)
		switch v := r.Header.Get("X-HubSpot-Signature-Version"); {
		case r.Header.Get("X-HubSpot-Signature-v3") != "":
			ts := r.Header.Get("X-HubSpot-Request-Timestamp")
			ms, err := strconv.ParseInt(ts, 10, 64)
			if err != nil || time.Since(time.UnixMilli(ms)).Abs() > 5*time.Minute {
				return nil, forged("X-HubSpot-Request-Timestamp %q", ts)
			}
			if !same(r.Header.Get("X-HubSpot-Signature-v3"), b64MAC(key, r.Method, uri, string(body), ts)) {
				return nil, forged("X-HubSpot-Signature-v3")
			}
		case v == "v1" && same(r.Header.Get("X-HubSpot-Signature"), sha256Hex(key+string(body))):
		case v == "v2" && same(r.Header.Get("X-HubSpot-Signature"), sha256Hex(key+r.Method+uri+string(body))):
		default:
			return nil, forged("X-HubSpot-Signature")
		}
		var events []struct {
			PortalID int64 `json:"portalId"`
		}
		if err := readJSON(body, &events); err != nil {
			return nil, err
		}
		var keys []string
		for _, e := range events {
			keys = append(keys, strconv.FormatInt(e.PortalID, 10))
		}
		return &Heard{Keys: keysOf(keys...)}, nil
	}}

func sha256Hex(s string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(s))) }
