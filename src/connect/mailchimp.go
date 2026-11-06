package connect

import (
	"time"

	"golang.org/x/oauth2"
)

func init() {
	Register(&Connector{
		ID: "mailchimp", Name: "Mailchimp", Integration: true, Lasts: true,
		Info: Info{By: "Intuit Mailchimp", Description: "Read audiences, members and campaigns, and add or tag members when you ask", Category: CatMarketing, Icon: IconMail,
			Scopes: []string{"Read audiences, members, tags, campaigns and reports", "Add, update and tag members when you ask"}},
		Hosts: []string{"https://login.mailchimp.com"},
		API:   "{+host}/3.0",
		Let: []Step{
			{As: "who", Req: Req{URL: "https://login.mailchimp.com/oauth2/metadata"}},
			{Unless: "who/api_endpoint", Fail: "Mailchimp did not name the account's API"},
			{As: "host", In: "{who/api_endpoint}", Find: `^(https://[^/]+)`},
		},
		Endpoint: oauth2.Endpoint{AuthURL: "https://login.mailchimp.com/oauth2/authorize", TokenURL: "https://login.mailchimp.com/oauth2/token",
			AuthStyle: oauth2.AuthStyleInParams},
		Who: &Who{Req: Req{URL: "/", Query: Q{"fields": "account_name,email"}}, Names: []Path{"account_name", "email"}},
		Actions: map[string]*Action{
			"get": Call[mailchimpGet, any](Plan{
				Doc: "Read any of Mailchimp's GET endpoints, like { path: \"/lists\" }, { path: \"/lists/<id>/members/ann@example.com\" }, " +
					"{ path: \"/lists/<id>/tag-search\" }, { path: \"/lists/<id>/segments\", query: \"type=static\" } (tags and their counts), " +
					"{ path: \"/reports/<campaign id>\" } or { path: \"/search-members\", query: \"query=ann\" }: its JSON answer. Lists come 10 at a time; " +
					"pass count=1000 and offset for more, and fields= or exclude_fields= to trim them",
				Steps: []Step{{Req: Req{URL: "{+path}?{+query|}"}}},
			}),
			"save": Call[mailchimpMember, any](Plan{
				Doc: "Add a member to an audience, or update one already there, by email: { audience, email, status, fields: { FNAME, LNAME } }. " +
					"status is only for someone new (subscribed only with their permission; pending emails them to confirm); it never changes " +
					"the status of someone already there. Returns the member",
				Enum: map[string][]string{"status": {"subscribed", "pending", "unsubscribed", "transactional"}},
				Steps: []Step{{Req: Req{Method: PUT, URL: "/lists/{audience}/members/{email}",
					Body: Obj{"email_address": "{email}", "status_if_new": "{status}", "merge_fields": "{fields}"}}}},
			}),
			"tag": Call[mailchimpTag, mailchimpTagged](Plan{
				Doc: "Add tags to a member, or take them off, by email: { audience, email, tags: [{ name: \"VIP\", status: \"active\" }] }, status " +
					"active to add and inactive to take off; a new name makes the tag. Automations that start on a tag do not start",
				Steps: []Step{{Req: Req{Method: POST, URL: "/lists/{audience}/members/{email}/tags", Body: Obj{"tags": "{tags}", "is_syncing": true}}}},
				Out:   Obj{"email": "{email}", "tags": "{tags}"},
			}),
		},
		Sync: []*Sync{
			{Name: "members", Gap: time.Second, Req: Req{URL: "/lists/{part}/members", Query: mailchimpMembers}, Items: "members", Updated: "last_changed", Page: mailchimpPage,
				Key:   []Reads{{Of: "{list_id}/{id}"}},
				Parts: &Parts{Req: Req{URL: "/lists", Query: Q{"count": "1000", "offset": "{page}", "fields": "lists.id,total_items"}}, Items: "lists", Page: Offset{Size: 1000, Step: 1000}},
				Change: SinceTime{Req: Req{URL: "/lists/{part}/members",
					Query: Q{"count": "1000", "offset": "{page}", "exclude_fields": "members._links,_links", "since_last_changed": "{since}", "sort_field": "last_changed", "sort_dir": "ASC"}}},
				Gone: []Test{{At: "status", Is: "archived"}},
				Doc: "Every member of every audience, by its audience and id, with their email, name, phone, company, address and status: listed whole " +
					"the first time and a week after, 1000 a page, which deletes those gone, and in between those changed since the sync before.",
				To: Map{Table: "crm_contact",
					Cols: map[string]SQL{"first_name": "nullif(data#>>'{merge_fields,FNAME}', '')", "last_name": "nullif(data#>>'{merge_fields,LNAME}', '')",
						"email": "archive.sync_email(data->>'email_address')", "phone": "coalesce(nullif(data#>>'{merge_fields,PHONE}', ''), nullif(data->>'sms_phone_number', ''))",
						"company_name": "nullif(data#>>'{merge_fields,COMPANY}', '')", "stage": "data->>'status'",
						"city": "nullif(data#>>'{merge_fields,ADDRESS,city}', '')", "state": "nullif(data#>>'{merge_fields,ADDRESS,state}', '')",
						"country":   "coalesce(nullif(data#>>'{merge_fields,ADDRESS,country}', ''), nullif(data#>>'{location,country_code}', ''))",
						"made_at":   "archive.sync_at(coalesce(nullif(data->>'timestamp_signup', ''), data->>'timestamp_opt'))",
						"edited_at": "archive.sync_at(data->>'last_changed')"},
					People: map[string]SQL{"person": "data->>'email_address'"}}},
			{Name: "campaigns", Gap: time.Second, Req: Req{URL: "/campaigns", Query: Q{"count": "1000", "offset": "{page}", "sort_field": "create_time", "sort_dir": "ASC",
				"fields": "campaigns.id,campaigns.type,campaigns.status,campaigns.create_time,campaigns.send_time,campaigns.settings.title,campaigns.settings.subject_line,total_items"}},
				Items: "campaigns", Page: Offset{Size: 1000, Step: 1000},
				Doc: "Every campaign, by its id, with its name, status and when it was made and sent: listed whole each sync, so one deleted is deleted.",
				To: Map{Table: "marketing_campaign",
					Cols: map[string]SQL{"title": "coalesce(nullif(data#>>'{settings,title}', ''), data#>>'{settings,subject_line}')", "channel": "'email'",
						"status": "data->>'status'", "archived": "data->>'status' = 'archived'",
						"scheduled_at": "CASE WHEN data->>'status' = 'schedule' THEN archive.sync_at(data->>'send_time') END",
						"sent_at":      "CASE WHEN data->>'status' IN ('sending', 'sent') THEN archive.sync_at(data->>'send_time') END",
						"made_at":      "archive.sync_at(data->>'create_time')"}}},
		},
	})
}

var (
	mailchimpMembers = Q{"count": "1000", "offset": "{page}", "exclude_fields": "members._links,_links"}
	mailchimpPage    = Offset{Size: 1000, Step: 950}
)

type (
	mailchimpGet struct {
		Path  string `json:"path" jsonschema:"The endpoint after /3.0, like /lists, /campaigns or /reports/<campaign id>"`
		Query string `json:"query,omitempty" jsonschema:"Its query, like count=100&offset=100 or status=subscribed"`
	}
	mailchimpMember struct {
		Audience string         `json:"audience" jsonschema:"The audience's id, from get /lists"`
		Email    string         `json:"email" jsonschema:"The member's email address"`
		Status   string         `json:"status,omitempty" jsonschema:"For someone new: subscribed (only with their permission), pending (Mailchimp emails them to confirm), unsubscribed or transactional; needed when they are not there yet"`
		Fields   map[string]any `json:"fields,omitempty" jsonschema:"Merge fields to set by tag, like FNAME, LNAME or PHONE"`
	}
	mailchimpTag struct {
		Audience string         `json:"audience" jsonschema:"The audience's id, from get /lists"`
		Email    string         `json:"email" jsonschema:"The member's email address"`
		Tags     []mailchimpTie `json:"tags" jsonschema:"The tags to add or take off"`
	}
	mailchimpTie struct {
		Name   string `json:"name" jsonschema:"The tag's name"`
		Status string `json:"status" jsonschema:"active to add it, inactive to take it off"`
	}
	mailchimpTagged struct {
		Email string         `json:"email"`
		Tags  []mailchimpTie `json:"tags"`
	}
)
