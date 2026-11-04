package connect

import (
	"time"

	"golang.org/x/oauth2"
)

// HubSpot's CRM: contacts, companies and deals, read, and when asked made, changed, linked and archived (to
// HubSpot's recycling bin, which keeps them 90 days). Its app, in a HubSpot developer project, lists
// <API>/connect/hubspot/callback among its redirect URLs, and exactly the Scopes here as required. Its webhooks are
// set up once on the app, to <API>/push/hubspot, for the creation, deletion and property changes of contacts,
// companies and deals; a push is an array of events, each with the portalId of its account, the hub_id of its token.
func init() {
	Register(&Connector{
		ID: "hubspot", Name: "HubSpot", Integration: true,
		Info: Info{By: "HubSpot", Description: "Read contacts, companies and deals, and add or update them when you ask", Category: CatCRM, Icon: IconUsers,
			Scopes: []string{"Read contacts, companies and deals", "Add, change, link and archive them when you ask"}},
		API: "https://api.hubapi.com",
		Endpoint: oauth2.Endpoint{AuthURL: "https://app.hubspot.com/oauth/authorize", TokenURL: "https://api.hubspot.com/oauth/v3/token",
			AuthStyle: oauth2.AuthStyleInParams},
		RevokeBy: &Req{Method: POST, URL: "https://api.hubapi.com/oauth/v3/token/revoke",
			Form: Q{"client_id": "{client/id}", "client_secret": "{client/secret}", "token": "{refresh}", "token_type_hint": "refresh_token"}},
		Scopes: []string{"oauth", "crm.objects.contacts.read", "crm.objects.companies.read", "crm.objects.deals.read",
			"crm.objects.contacts.write", "crm.objects.companies.write", "crm.objects.deals.write"},
		Who: &Who{Req: hubspotIntrospect, Names: []Path{"hub_domain", "user"}, Lower: true},
		Actions: map[string]*Action{
			"search": Call[hubspotFind, hubspotFound](Plan{
				Doc:   "Find contacts, companies or deals by words in their default text properties (names, emails, domains, phone numbers), newest first (pass next as page for more)",
				Clamp: map[string][2]int{"limit": {25, 100}},
				Enum:  map[string][]string{"type": hubspotTypes},
				Steps: []Step{{As: "found", Req: Req{Method: POST, Read: true, URL: "/crm/v3/objects/{type}/search",
					Body: Obj{"limit": "{limit}", "query": "{query}", "properties": "{properties}", "after": "{page}",
						"sorts": []Obj{{"propertyName": "createdate", "direction": "DESCENDING"}}}}}},
				Out: Obj{"results": "{found/results|[]}", "next": "{found/paging/next/after|}"},
			}),
			"get": Call[hubspotOne, hubspotRecord](Plan{
				Doc:  "One contact, company or deal by id, with the properties asked for and the ids of the records it is associated with",
				Enum: map[string][]string{"type": hubspotTypes},
				Steps: []Step{{Req: Req{URL: "/crm/v3/objects/{type}/{id}",
					Query: Q{"properties": "{properties,}", "associations": "{associations,}"}}}},
			}),
			"create": Call[hubspotNew, hubspotRecord](Plan{
				Doc:   "Add a contact, company or deal with its properties, like { type: \"contacts\", properties: { email, firstname, lastname } } or { type: \"deals\", properties: { dealname, amount, dealstage } }: the record",
				Enum:  map[string][]string{"type": hubspotTypes},
				Steps: []Step{{Req: Req{Method: POST, URL: "/crm/v3/objects/{type}", Body: Obj{"properties": "{properties}"}}}},
			}),
			"update": Call[hubspotEdit, hubspotRecord](Plan{
				Doc:   "Change a contact, company or deal: only the properties given change, and an empty string clears one. Returns the record",
				Enum:  map[string][]string{"type": hubspotTypes},
				Steps: []Step{{Req: Req{Method: PATCH, URL: "/crm/v3/objects/{type}/{id}", Body: Obj{"properties": "{properties}"}}}},
			}),
			"link": Call[hubspotLink, hubspotLinked](Plan{
				Doc:   "Associate two records with HubSpot's default label, like a contact with its company or a deal with a contact: { from, to }",
				Enum:  map[string][]string{"type": hubspotTypes, "toType": hubspotTypes},
				Steps: []Step{{Req: Req{Method: PUT, URL: "/crm/v4/objects/{type}/{id}/associations/default/{toType}/{toId}"}}},
				Out:   Obj{"from": "{id}", "to": "{toId}"},
			}),
			"archive": Call[hubspotOne, hubspotArchived](Plan{
				Doc:   "Archive a contact, company or deal: HubSpot moves it to its recycling bin, where it can be restored for 90 days. { id }",
				Enum:  map[string][]string{"type": hubspotTypes},
				Steps: []Step{{Req: Req{Method: DELETE, URL: "/crm/v3/objects/{type}/{id}"}}},
				Out:   Obj{"id": "{id}"},
			}),
		},
		Listen: &Listen{
			Watch: hubspotIntrospect, Key: "{answer/hub_id}", Keys: []Path{"*/portalId"},
			Signs: []Sign{
				{Header: "X-HubSpot-Signature-v3", Of: "{method}{url}{body}{header/X-HubSpot-Request-Timestamp}", Plain: hubspotPlain, Fresh: "X-HubSpot-Request-Timestamp"},
				{Header: "X-HubSpot-Signature", When: "X-HubSpot-Signature-Version", Is: "v1", Digest: SHA256, Enc: Hex, Of: "{secret}{body}"},
				{Header: "X-HubSpot-Signature", When: "X-HubSpot-Signature-Version", Is: "v2", Digest: SHA256, Enc: Hex, Of: "{secret}{method}{url}{body}", Plain: hubspotPlain},
			},
		},
		Sync: []*Sync{
			{Name: "companies", Gap: time.Second, Req: hubspotList, Items: "results", Updated: "updatedAt", Page: hubspotPage, Change: hubspotSearch,
				Vars: Obj{"changed": "hs_lastmodifieddate", "with": "", "props": []string{"name", "domain", "website", "industry", "lifecyclestage", "phone", "city", "state", "country", "numberofemployees", "annualrevenue", "description", "createdate", "hs_lastmodifieddate"}},
				Doc:  "Every one of HubSpot's companies, by id: listed whole the first time and a week after, 100 a page, which deletes those HubSpot no longer has, and searched in between for those changed since the sync before.",
				To: Map{Table: "crm_company",
					Cols: map[string]SQL{"title": "data#>>'{properties,name}'", "domain": hubspotDomain, "industry": "data#>>'{properties,industry}'", "stage": "data#>>'{properties,lifecyclestage}'",
						"phone": "data#>>'{properties,phone}'", "city": "data#>>'{properties,city}'", "state": "data#>>'{properties,state}'", "country": "data#>>'{properties,country}'",
						"employees": "archive.sync_num(data#>>'{properties,numberofemployees}')", "revenue": "archive.sync_money(data#>>'{properties,annualrevenue}', 'USD')",
						"description": "archive.sync_clip(data#>>'{properties,description}', 65536)", "made_at": "archive.sync_at(data#>>'{properties,createdate}')",
						"edited_at": "archive.sync_at(coalesce(nullif(data#>>'{properties,hs_lastmodifieddate}', ''), data->>'updatedAt'))"},
					Orgs: map[string]SQL{"org": hubspotDomain}}},
			{Name: "contacts", Gap: time.Second, Req: hubspotList, Items: "results", Updated: "updatedAt", Page: hubspotPage, Change: hubspotSearch,
				Vars: Obj{"changed": "lastmodifieddate", "with": "", "props": []string{"firstname", "lastname", "email", "phone", "jobtitle", "company", "associatedcompanyid", "lifecyclestage", "hs_lead_status", "city", "state", "country", "createdate", "lastmodifieddate"}},
				Doc:  "Every one of HubSpot's contacts, by id: listed whole the first time and a week after, 100 a page, which deletes those HubSpot no longer has, and searched in between for those changed since the sync before.",
				To: Map{Table: "crm_contact",
					Cols: map[string]SQL{"first_name": "data#>>'{properties,firstname}'", "last_name": "data#>>'{properties,lastname}'", "email": "archive.sync_email(data#>>'{properties,email}')",
						"phone": "data#>>'{properties,phone}'", "job_title": "data#>>'{properties,jobtitle}'", "company_name": "data#>>'{properties,company}'",
						"stage": "data#>>'{properties,lifecyclestage}'", "lead_status": "data#>>'{properties,hs_lead_status}'", "city": "data#>>'{properties,city}'",
						"state": "data#>>'{properties,state}'", "country": "data#>>'{properties,country}'", "made_at": "archive.sync_at(data#>>'{properties,createdate}')",
						"edited_at": "archive.sync_at(coalesce(nullif(data#>>'{properties,lastmodifieddate}', ''), data->>'updatedAt'))"},
					People: map[string]SQL{"person": "data#>>'{properties,email}'"},
					Refs:   map[string]SQL{"company": "data#>>'{properties,associatedcompanyid}'"}}},
			{Name: "deals", Gap: time.Second, Req: hubspotList, Items: "results", Updated: "updatedAt", Page: hubspotPage, Change: hubspotSearch,
				Vars: Obj{"changed": "hs_lastmodifieddate", "with": "companies", "props": []string{"dealname", "amount", "deal_currency_code", "pipeline", "dealstage", "closedate", "hs_is_closed", "hs_is_closed_won", "description", "createdate", "hs_lastmodifieddate"}},
				Doc:  "Every one of HubSpot's deals, by id, with its pipeline and stage by name and the company it is with: listed whole the first time and a week after, 100 a page, which deletes those HubSpot no longer has, and searched in between for those changed since the sync before.",
				Hydrate: []Hydrate{
					Batch{Req: Req{Method: POST, Read: true, URL: "/crm/v4/associations/deals/companies/batch/read", Body: Obj{"inputs": "{inputs}"}},
						As: "association", Results: "results", Key: "from/id", DeltaOnly: true},
					Lookup{Req: Req{URL: "/crm/v3/pipelines/deals"}, As: "pipelines", Soft: true},
				},
				To: Map{Table: "crm_deal",
					Cols: map[string]SQL{"title": "data#>>'{properties,dealname}'",
						"amount": "archive.sync_money(data#>>'{properties,amount}', coalesce(nullif(data#>>'{properties,deal_currency_code}', ''), 'USD'))",
						"pipeline": `CASE WHEN data->'pipelines' IS NOT NULL THEN coalesce((SELECT p->>'label' FROM jsonb_array_elements(data#>'{pipelines,results}') p
							WHERE p->>'id' = data#>>'{properties,pipeline}' LIMIT 1), data#>>'{properties,pipeline}') END`,
						"stage": `CASE WHEN data->'pipelines' IS NOT NULL THEN coalesce((SELECT s->>'label' FROM jsonb_array_elements(data#>'{pipelines,results}') p, jsonb_array_elements(p->'stages') s
							WHERE p->>'id' = data#>>'{properties,pipeline}' AND s->>'id' = data#>>'{properties,dealstage}' LIMIT 1), data#>>'{properties,dealstage}') END`,
						"closed":      "CASE data#>>'{properties,hs_is_closed}' WHEN 'true' THEN true WHEN 'false' THEN false END",
						"won":         "CASE data#>>'{properties,hs_is_closed_won}' WHEN 'true' THEN true WHEN 'false' THEN false END",
						"close_on":    "archive.sync_day(data#>>'{properties,closedate}')",
						"description": "archive.sync_clip(data#>>'{properties,description}', 65536)", "made_at": "archive.sync_at(data#>>'{properties,createdate}')",
						"edited_at": "archive.sync_at(coalesce(nullif(data#>>'{properties,hs_lastmodifieddate}', ''), data->>'updatedAt'))"},
					Refs: map[string]SQL{"company": "coalesce(nullif(data#>>'{association,to,0,toObjectId}', ''), data#>>'{associations,companies,results,0,id}')"}}},
		},
	})
}

const (
	hubspotDomain SQL = "coalesce(archive.sync_domain(data#>>'{properties,domain}'), archive.sync_domain(data#>>'{properties,website}'))"
	hubspotPlain      = ":/?@!$'()*,;"
)

var (
	hubspotTypes      = []string{"contacts", "companies", "deals"}
	hubspotIntrospect = Req{Method: POST, Read: true, URL: "https://api.hubspot.com/oauth/v3/token/introspect",
		Form: Q{"client_id": "{client/id}", "client_secret": "{client/secret}", "token_type_hint": "access_token", "access_token": "{token}"}}
	hubspotList = Req{URL: "/crm/v3/objects/{stream}", Query: Q{"limit": "100", "properties": "{props,}", "associations": "{with}", "after": "{page}"}}
	hubspotPage = Cursor{At: "paging/next/after"}
	// A search reaches no further than 10,000 results, so a delta bigger than that lists whole. It goes on from the
	// last change it read, so a record that changes while it pages, and moves to the end, moves no other out of reach.
	hubspotSearch = SinceTime{Ms: true, Mark: "properties/{changed}", Most: 9900,
		Req: Req{Method: POST, Read: true, URL: "/crm/v3/objects/{stream}/search", Body: Obj{"limit": 100, "properties": "{props}", "after": "{page}",
			"filterGroups": []Obj{{"filters": []Obj{{"propertyName": "{changed}", "operator": "GTE", "value": "{since}"}}}},
			"sorts":        []Obj{{"propertyName": "{changed}", "direction": "ASCENDING"}}}}}
)

type (
	hubspotFind struct {
		Type       string   `json:"type" jsonschema:"contacts, companies or deals"`
		Query      string   `json:"query,omitempty" jsonschema:"Words to find; every record when left out"`
		Properties []string `json:"properties,omitempty" jsonschema:"The properties to return, like firstname, email, amount or dealstage; HubSpot's defaults when left out"`
		Limit      int      `json:"limit,omitempty" jsonschema:"At most this many, 1 to 100; 25 when left out"`
		Page       string   `json:"page,omitempty" jsonschema:"The next of the search before, for the records after it"`
	}
	hubspotOne struct {
		Type         string   `json:"type" jsonschema:"contacts, companies or deals"`
		ID           string   `json:"id" jsonschema:"The record's id"`
		Properties   []string `json:"properties,omitempty" jsonschema:"The properties to return; HubSpot's defaults when left out"`
		Associations []string `json:"associations,omitempty" jsonschema:"The types of the records associated with it to list, like companies or deals"`
	}
	hubspotNew struct {
		Type       string         `json:"type" jsonschema:"contacts, companies or deals"`
		Properties map[string]any `json:"properties" jsonschema:"Its properties by internal name, like email, firstname, name, domain, dealname, amount, dealstage or closedate"`
	}
	hubspotEdit struct {
		Type       string         `json:"type" jsonschema:"contacts, companies or deals"`
		ID         string         `json:"id" jsonschema:"The record's id"`
		Properties map[string]any `json:"properties" jsonschema:"The properties to change by internal name; an empty string clears one"`
	}
	hubspotLink struct {
		Type   string `json:"type" jsonschema:"The first record's type: contacts, companies or deals"`
		ID     string `json:"id" jsonschema:"The first record's id"`
		ToType string `json:"toType" jsonschema:"The other record's type: contacts, companies or deals"`
		ToID   string `json:"toId" jsonschema:"The other record's id"`
	}
	hubspotLinked struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	hubspotArchived struct {
		ID string `json:"id"`
	}
	hubspotRecord struct {
		ID           string         `json:"id"`
		Properties   map[string]any `json:"properties"`
		Associations map[string]any `json:"associations,omitempty"`
		CreatedAt    string         `json:"createdAt"`
		UpdatedAt    string         `json:"updatedAt"`
	}
	hubspotFound struct {
		Results []hubspotRecord `json:"results"`
		Next    string          `json:"next" jsonschema:"Pass as page for more; empty when there is no more"`
	}
)
