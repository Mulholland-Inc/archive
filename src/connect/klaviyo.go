package connect

import (
	"time"

	"golang.org/x/oauth2"
)

func init() {
	Register(&Connector{
		ID: "klaviyo", Name: "Klaviyo", Integration: true,
		Info: Info{By: "Klaviyo", Description: "Read profiles, lists and campaigns", Category: CatMarketing, Icon: IconMail, Scopes: []string{"Read profiles, lists, campaigns and metrics"}},
		API:  "https://a.klaviyo.com/api",
		Endpoint: oauth2.Endpoint{AuthURL: "https://www.klaviyo.com/oauth/authorize", TokenURL: "https://a.klaviyo.com/oauth/token",
			AuthStyle: oauth2.AuthStyleInHeader},
		RevokeBy: &Req{Method: POST, URL: "https://a.klaviyo.com/oauth/revoke", Header: map[string][]string{"Authorization": {"{basic}"}},
			Form: Q{"token": "{refresh}", "token_type_hint": "refresh_token"}},
		Scopes: []string{"accounts:read", "profiles:read", "lists:read", "campaigns:read", "metrics:read"},
		Who:    &Who{Req: Req{URL: "/accounts", Header: klaviyoHead}, Names: []Path{"data/0/attributes/contact_information/organization_name", "data/0/id"}},
		Actions: map[string]*Action{
			"get": Call[klaviyoGet, any](Plan{
				Doc: "Read any of Klaviyo's GET endpoints, like { path: \"/lists\" }, { path: \"/profiles\", query: \"filter=equals(email,'ann@example.com')\" } " +
					"or { path: \"/campaigns/<id>/campaign-messages\" }: its JSON:API answer, { data, links, included } (pass links.next as page for more)",
				Steps: []Step{
					{If: "page", Req: Req{URL: "{+page}", Header: klaviyoHead}},
					{Unless: "page", Req: Req{URL: "{+path}?{+query|}", Header: klaviyoHead}},
				},
			}),
		},
		Sync: []*Sync{
			{Name: "profiles", Gap: time.Second, Req: Req{URL: "/profiles", Header: klaviyoHead, Query: Q{"page[size]": "100"}}, Items: "data", Updated: "attributes/updated", Page: klaviyoPage,
				Change: SinceTime{Req: Req{URL: "/profiles", Header: klaviyoHead, Query: Q{"page[size]": "100", "filter": "greater-than(updated,{since})", "sort": "updated"}}},
				Doc: "Every profile, by its id, with its email, phone, name, organization, title and location: listed whole the first time and a " +
					"week after, 100 a page, which deletes those gone, and in between those updated since the sync before.",
				To: Map{Table: "crm_contact",
					Cols: map[string]SQL{"first_name": "data#>>'{attributes,first_name}'", "last_name": "data#>>'{attributes,last_name}'", "email": "archive.sync_email(data#>>'{attributes,email}')",
						"phone": "data#>>'{attributes,phone_number}'", "job_title": "data#>>'{attributes,title}'", "company_name": "data#>>'{attributes,organization}'", "stage": "'subscriber'",
						"city": "data#>>'{attributes,location,city}'", "state": "data#>>'{attributes,location,region}'", "country": "data#>>'{attributes,location,country}'",
						"made_at": "archive.sync_at(data#>>'{attributes,created}')", "edited_at": "archive.sync_at(data#>>'{attributes,updated}')"},
					People: map[string]SQL{"person": "data#>>'{attributes,email}'"}}},
			{Name: "campaigns", Gap: time.Second, Req: Req{URL: "/campaigns", Header: klaviyoHead, Query: Q{"filter": "equals(messages.channel,'{part}')"}}, Items: "data", Updated: "attributes/updated_at", Page: klaviyoPage,
				Parts: &Parts{Fixed: []Obj{{"id": "email"}, {"id": "sms"}}},
				Set:   Obj{"channel": "{part}"},
				Doc: "Every email and SMS campaign, by its id, with its name, status and when it was scheduled and sent: listed whole each sync, " +
					"so one deleted is deleted.",
				To: Map{Table: "marketing_campaign",
					Cols: map[string]SQL{"title": "data#>>'{attributes,name}'", "channel": "data->>'channel'", "status": "data#>>'{attributes,status}'", "archived": "archive.sync_bool(data#>'{attributes,archived}')",
						"scheduled_at": "archive.sync_at(data#>>'{attributes,scheduled_at}')", "sent_at": "archive.sync_at(data#>>'{attributes,send_time}')",
						"made_at": "archive.sync_at(data#>>'{attributes,created_at}')", "edited_at": "archive.sync_at(data#>>'{attributes,updated_at}')"}}},
		},
	})
}

var (
	klaviyoHead = map[string][]string{"Revision": {"2025-07-15"}, "Accept": {"application/vnd.api+json"}}
	klaviyoPage = NextURL{At: "links/next"}
)

type klaviyoGet struct {
	Path  string `json:"path,omitempty" jsonschema:"The endpoint after /api, like /lists or /metrics"`
	Query string `json:"query,omitempty" jsonschema:"Its query, like filter=... or page[size]=50"`
	Page  string `json:"page,omitempty" jsonschema:"The links.next of the call before"`
}
