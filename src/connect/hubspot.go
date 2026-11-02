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
		Push:    hubspotPush,
		Streams: []*Stream{hubspotStream("contacts"), hubspotStream("companies"), hubspotStream("deals")},
	})
}

// hubspotProps are the properties a stream keeps of each type, with its last change: lastmodifieddate for a
// contact, hs_lastmodifieddate for the rest, as HubSpot's search has them.
var hubspotProps = map[string][]string{
	"contacts":  {"firstname", "lastname", "email", "phone", "company", "jobtitle", "lifecyclestage", "createdate", "lastmodifieddate"},
	"companies": {"name", "domain", "industry", "phone", "city", "country", "numberofemployees", "createdate", "hs_lastmodifieddate"},
	"deals":     {"dealname", "amount", "dealstage", "pipeline", "closedate", "hubspot_owner_id", "createdate", "hs_lastmodifieddate"},
}

// hubspotCursor is where a stream is: Since is the last change, in Unix milliseconds, a delta searches from,
// After the next page of the search, and Max the latest change it has seen.
type hubspotCursor struct {
	Since string `json:"since,omitempty"`
	After string `json:"after,omitempty"`
	Max   int64  `json:"max,omitempty"`
}

// hubspotStream is the records of typ, by a search sorted by their last change, 100 a page: all of them the
// first time, and then those changed since. A search pages at most 10,000 deep, so one that would go further
// searches again from the last change it read.
func hubspotStream(typ string) *Stream {
	return &Stream{Name: typ, Doc: "Every one of your " + typ + " with " + strings.Join(hubspotProps[typ], ", ") +
		": { id, properties, createdAt, updatedAt }, by a search sorted by their last change, 100 a page, all the first time and then those changed since. " +
		"The search does not see deleted ones, so they stay.",
		Pull: func(c *Client, cursor string) (*Page, error) {
			var cur hubspotCursor
			if err := cursorOf(cursor, &cur); err != nil {
				return nil, err
			}
			prop := hubspotProps[typ][len(hubspotProps[typ])-1]
			body := map[string]any{"limit": 100, "properties": hubspotProps[typ], "sorts": []map[string]string{{"propertyName": prop, "direction": "ASCENDING"}}}
			if cur.Since != "" {
				body["filterGroups"] = []any{map[string]any{"filters": []any{map[string]string{"propertyName": prop, "operator": "GTE", "value": cur.Since}}}}
			}
			if cur.After != "" {
				body["after"] = cur.After
			}
			var out struct {
				Results []map[string]any `json:"results"`
				Paging  struct {
					Next struct {
						After string `json:"after"`
					} `json:"next"`
				} `json:"paging"`
			}
			if err := c.Do("POST", hubspotAPI+"/crm/v3/objects/"+typ+"/search", body, &out); err != nil {
				return nil, err
			}
			page := &Page{}
			for _, r := range out.Results {
				at := stamp(str(r, "updatedAt"))
				cur.Max = max(cur.Max, at.UnixMilli())
				page.Records = append(page.Records, Record{ID: str(r, "id"), Data: r, Updated: at})
			}
			deep, _ := strconv.Atoi(out.Paging.Next.After)
			switch {
			case out.Paging.Next.After == "":
				if cur.Max > 0 {
					cur.Since = strconv.FormatInt(cur.Max, 10)
				}
				cur.After = ""
			case deep >= 9900:
				cur.Since, cur.After, page.More = strconv.FormatInt(cur.Max, 10), "", true
			default:
				cur.After, page.More = out.Paging.Next.After, true
			}
			page.Cursor = cursorFor(cur)
			return page, nil
		}}
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
