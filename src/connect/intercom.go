package connect

import (
	"time"

	"golang.org/x/oauth2"
)

// Intercom: a workspace's conversations, tickets, contacts and companies, read, and when asked conversations and
// tickets answered, closed, opened, snoozed and assigned, and contacts added and changed. Its app, in Intercom's
// Developer Hub, lists <API>/connect/intercom/callback as its redirect URL, reads API version 2.14, and has the
// permissions Read and list users and companies, Write users and companies, Read conversations, Write conversations,
// Read tickets, Write tickets and Read admins; an app is not offered to other workspaces until Intercom reviews it.
// Its tokens never expire and come without a refresh token, so the token is kept, and disconnecting uninstalls the
// app. api.intercom.io takes a workspace of any region, and the sign-in asks one hosted in Europe or Australia to
// pick its region.
//
// It syncs companies into crm_company, contacts, users and leads alike, into crm_contact, tickets into work_item and
// conversations into chat_message, each by its id, a conversation as one message with its whole thread as its text.
// Companies are listed whole every sync; the rest the first time and a week after, and searched in between for what
// changed since the newest change the sync before read. It pushes nothing yet: Intercom signs its webhooks with
// HMAC-SHA1.
func init() {
	Register(&Connector{
		ID: "intercom", Name: "Intercom", Integration: true, Lasts: true,
		Info: Info{By: "Intercom", Description: "Read conversations, tickets, contacts and companies, and answer or update them when you ask", Category: CatSupport, Icon: IconInbox,
			Scopes: []string{"Read conversations, tickets, contacts, companies and teammates", "Reply to, close, snooze and assign conversations and tickets, and add or change contacts, when you ask"}},
		API:      "https://api.intercom.io",
		Endpoint: oauth2.Endpoint{AuthURL: "https://app.intercom.com/oauth", TokenURL: "https://api.intercom.io/auth/eagle/token", AuthStyle: oauth2.AuthStyleInParams},
		RevokeBy: &Req{Method: POST, URL: "https://api.intercom.io/auth/uninstall", Header: map[string][]string{"Authorization": {"Bearer {refresh}"}}},
		Who:      &Who{Req: Req{URL: "/me"}, Names: []Path{"app/name", "email"}},
		Actions: map[string]*Action{
			"search": Call[intercomFind, intercomFound](Plan{
				Doc: "Find contacts, conversations or tickets with Intercom's search query, a filter like { field: \"email\", operator: \"=\", value: \"jane@example.com\" } " +
					"or { operator: \"AND\", value: [{ field: \"state\", operator: \"=\", value: \"open\" }, { field: \"updated_at\", operator: \">\", value: 1767225600 }] }, " +
					"its operators =, !=, IN, NIN, <, >, ~ (contains), !~, ^ (starts with) and $ (ends with), and its times Unix seconds: { results, total, next } (pass next as page for more)",
				Clamp: map[string][2]int{"limit": {25, 150}},
				Enum:  map[string][]string{"type": intercomSearched},
				Steps: []Step{{As: "found", Req: Req{Method: POST, Read: true, URL: "/{type}/search",
					Body: Obj{"query": "{query}", "pagination": Obj{"per_page": "{limit}", "starting_after": "{page}"}}}}},
				Out: Obj{"results": "{found/data;found/conversations;found/tickets|[]}", "total": "{found/total_count|0}", "next": "{found/pages/next/starting_after|}"},
			}),
			"get": Call[intercomOne, map[string]any](Plan{
				Doc:   "One contact, company, conversation or ticket by id; a conversation with its thread, up to its last 500 parts, in plain text",
				Enum:  map[string][]string{"type": intercomTypes},
				Steps: []Step{{Req: Req{URL: "/{type}/{id}", Query: Q{"display_as": "plaintext"}}}},
			}),
			"admins": Call[struct{}, []intercomAdmin](Plan{
				Doc:   "The workspace's teammates, whom a conversation is assigned to: [{ id, name, email, away_mode_enabled }]",
				Steps: []Step{{As: "got", Req: Req{URL: "/admins"}}},
				Out:   "{got/admins|[]}",
			}),
			"reply": Call[intercomSay, map[string]any](Plan{
				Doc: "Add to a conversation or ticket as you: an internal note, which only teammates see, unless public, a reply the customer gets in the " +
					"Messenger or by email. Returns the conversation or ticket",
				Enum: map[string][]string{"type": {"conversations", "tickets"}},
				Steps: []Step{
					intercomMe,
					{If: "public", As: "kind", In: "comment"},
					{Unless: "public", As: "kind", In: "note"},
					{Req: Req{Method: POST, URL: "/{type}/{id}/reply", Body: Obj{"message_type": "{kind}", "type": "admin", "admin_id": "{me/id}", "body": "{body}"}}},
				},
			}),
			"update": Call[intercomMove, map[string]any](Plan{
				Doc: "Close, open, snooze or assign a conversation as you, maybe with a note: { id, action: \"close\" }, { id, action: \"snoozed\", snoozed_until } " +
					"or { id, action: \"assignment\", assignee_id } (an admin's id, from admins, or 0 for nobody). Returns the conversation",
				Enum: map[string][]string{"action": {"close", "open", "snoozed", "assignment"}},
				Steps: []Step{
					{Only: Test{At: "action", Is: "assignment"}, Unless: "assignee_id", Fail: "an assignment needs assignee_id"},
					{Only: Test{At: "action", Is: "snoozed"}, Unless: "snoozed_until", Fail: "snoozing needs snoozed_until"},
					intercomMe,
					{Req: Req{Method: POST, URL: "/conversations/{id}/parts", Body: Obj{"message_type": "{action}", "type": "admin", "admin_id": "{me/id}",
						"assignee_id": "{assignee_id}", "snoozed_until": "{snoozed_until}", "body": "{body}"}}},
				},
			}),
			"contact": Call[intercomContact, map[string]any](Plan{
				Doc:  "Add a contact, a user or a lead, or with id change one: only what is given changes, custom_attributes by their names. Returns the contact",
				Enum: map[string][]string{"role": {"user", "lead"}},
				Steps: []Step{
					{If: "id", Req: Req{Method: PUT, URL: "/contacts/{id}", Body: intercomPerson}},
					{Unless: "id", Req: Req{Method: POST, URL: "/contacts", Body: intercomPerson}},
				},
			}),
		},
		Sync: []*Sync{
			{Name: "companies", Gap: time.Second, Req: Req{Method: POST, Read: true, URL: "/companies/list", Query: Q{"per_page": "60", "page": "{page}"}},
				Items: "data", Page: Offset{From: 1, Size: 60, Step: 1},
				Doc: "Every company, with its name, website, industry and size, by its id: listed whole every sync, 60 a page, which deletes those gone.",
				To: Map{Table: "crm_company",
					Cols: map[string]SQL{"title": "data->>'name'", "domain": "archive.sync_domain(data->>'website')", "industry": "data->>'industry'",
						"stage": "data#>>'{plan,name}'", "employees": "archive.sync_num(data->>'size')", "made_at": intercomMade, "edited_at": intercomEdited},
					Orgs: map[string]SQL{"org": "data->>'website'"}}},
			{Name: "contacts", Gap: time.Second, Req: Req{URL: "/contacts", Query: Q{"per_page": "150", "starting_after": "{page}"}}, Items: "data", Page: intercomPage,
				Vars: Obj{"size": 150}, Change: SinceTime{Stamp: "updated_at", Req: intercomSearch},
				Doc: "Every contact, users and leads alike, with their name, email, phone, role, place and first company, by its id: listed whole the first time " +
					"and a week after, 150 a page, which deletes those gone, and in between searched for those changed since the sync before.",
				To: Map{Table: "crm_contact",
					Cols: map[string]SQL{"first_name": `regexp_replace(btrim(data->>'name'), ' [^ ]*$', '')`, "last_name": `substring(btrim(data->>'name') from ' ([^ ]*)$')`,
						"email": "archive.sync_email(data->>'email')", "phone": "data->>'phone'", "stage": "data->>'role'", "company_source": "data#>>'{companies,data,0,id}'",
						"city": "data#>>'{location,city}'", "state": "data#>>'{location,region}'", "country": "data#>>'{location,country}'", "made_at": intercomMade, "edited_at": intercomEdited},
					People: map[string]SQL{"person": "data->>'email'"},
					Refs:   map[string]SQL{"company": "data#>>'{companies,data,0,id}'"}}},
			// The teammate a ticket is assigned to, which a ticket names only by id, looked up once a page for each.
			{Name: "tickets", Gap: time.Second, Req: intercomSearch, Items: "tickets", Page: intercomPage,
				Vars: Obj{"since": "0", "size": 100}, Change: SinceTime{Stamp: "updated_at", Req: intercomSearch},
				Hydrate: []Hydrate{Fetch{If: Test{At: "admin_assignee_id", Re: "^[1-9]"}, Once: "{admin_assignee_id}", Soft: true, Req: Req{URL: "/admins/{admin_assignee_id}"}, As: "assignee"}},
				Doc: "Every ticket, with its title, type, state, assignee and description, cut at 64 KB, by its id: listed whole the first time and a week after, " +
					"100 a page, which deletes those gone, and in between searched for those changed since the sync before.",
				To: Map{Table: "work_item",
					Cols: map[string]SQL{"title": "data#>>'{ticket_attributes,_default_title_}'", "type": "coalesce(data#>>'{ticket_type,name}', 'ticket')",
						"status":        "coalesce(data#>>'{ticket_state,internal_label}', data#>>'{ticket_state,category}')",
						"assignee_name": "data#>>'{assignee,name}'", "done_on": "CASE WHEN data#>>'{ticket_state,category}' = 'resolved' THEN (" + intercomEdited + ")::date END",
						"description": "archive.sync_clip(data#>>'{ticket_attributes,_default_description_}', 65536)", "made_at": intercomMade, "edited_at": intercomEdited},
					People: map[string]SQL{"assignee": "data#>>'{assignee,email}'"}}},
			// A conversation's thread, in plain text, read once for each conversation that changed.
			{Name: "conversations", Gap: time.Second, Req: Req{URL: "/conversations", Query: Q{"per_page": "50", "starting_after": "{page}"}}, Items: "conversations", Page: intercomPage,
				Vars: Obj{"size": 50}, Change: SinceTime{Stamp: "updated_at", Req: intercomSearch}, Memo: "{updated_at}",
				Set:     Obj{"kind": "chat"},
				Hydrate: []Hydrate{Fetch{Par: 4, Soft: true, Req: Req{URL: "/conversations/{id}", Query: Q{"display_as": "plaintext"}}, As: "full"}},
				Doc: "Every conversation, as one message: its title, who started it, and its thread, each part with who wrote it, cut at 64 KB, by its id: listed " +
					"whole the first time and a week after, 50 a page, which deletes those gone, and in between searched for those changed since the sync before.",
				To: Map{Table: "chat_message",
					Cols: map[string]SQL{"channel": "coalesce(nullif(data->>'title', ''), 'Intercom')", "kind": "data->>'kind'", "thread": "data->>'id'",
						"sender_name": "data#>>'{source,author,name}'",
						"text": `archive.sync_clip(concat_ws(E'\n\n', nullif(concat_ws(': ', data#>>'{source,author,name}', coalesce(data#>>'{full,source,body}', data#>>'{source,body}')), ''),
							(SELECT string_agg(concat_ws(': ', p#>>'{author,name}', p->>'body'), E'\n\n' ORDER BY p->>'created_at')
							FROM archive.sync_list(data#>'{full,conversation_parts,conversation_parts}') p WHERE nullif(p->>'body', '') IS NOT NULL)), 65536)`,
						"sent_at": intercomMade, "edited_at": intercomEdited},
					People: map[string]SQL{"sender": "data#>>'{source,author,email}'"}}},
		},
	})
}

// Intercom's times are Unix seconds.
const (
	intercomMade   SQL = "CASE WHEN data->>'created_at' ~ '^[0-9]{9,11}$' THEN to_timestamp((data->>'created_at')::numeric) END"
	intercomEdited SQL = "CASE WHEN data->>'updated_at' ~ '^[0-9]{9,11}$' THEN to_timestamp((data->>'updated_at')::numeric) END"
)

var (
	intercomTypes    = []string{"contacts", "companies", "conversations", "tickets"}
	intercomSearched = []string{"contacts", "conversations", "tickets"}
	intercomPage     = Cursor{At: "pages/next/starting_after"}
	intercomMe       = Step{As: "me", Req: Req{URL: "/me"}}
	// A search of the stream for what changed after {since}, in Unix seconds, {size} a page.
	intercomSearch = Req{Method: POST, Read: true, URL: "/{stream}/search", Body: Obj{"query": Obj{"field": "updated_at", "operator": ">", "value": "{since}"},
		"pagination": Obj{"per_page": "{size}", "starting_after": "{page}"}}}
	intercomPerson = Obj{"role": "{role}", "email": "{email}", "name": "{name}", "phone": "{phone}", "external_id": "{external_id}", "custom_attributes": "{custom_attributes}"}
)

type (
	intercomFind struct {
		Type  string         `json:"type" jsonschema:"contacts, conversations or tickets"`
		Query map[string]any `json:"query" jsonschema:"The filter: { field, operator, value }, or { operator: AND or OR, value: [filters] }"`
		Limit int            `json:"limit,omitempty" jsonschema:"At most this many a page, 1 to 150; 25 when left out"`
		Page  string         `json:"page,omitempty" jsonschema:"The next of the search before, for the results after it"`
	}
	intercomFound struct {
		Results []map[string]any `json:"results"`
		Total   int              `json:"total"`
		Next    string           `json:"next" jsonschema:"Pass as page for more; empty when there is no more"`
	}
	intercomOne struct {
		Type string `json:"type" jsonschema:"contacts, companies, conversations or tickets"`
		ID   string `json:"id" jsonschema:"Its id"`
	}
	intercomAdmin struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
		Away  bool   `json:"away_mode_enabled,omitempty"`
	}
	intercomSay struct {
		Type   string `json:"type" jsonschema:"conversations or tickets"`
		ID     string `json:"id" jsonschema:"The conversation's or ticket's id"`
		Body   string `json:"body" jsonschema:"What to say; it may be HTML"`
		Public bool   `json:"public,omitempty" jsonschema:"true to reply to the customer; an internal note when left out"`
	}
	intercomMove struct {
		ID           string `json:"id" jsonschema:"The conversation's id"`
		Action       string `json:"action" jsonschema:"close, open, snoozed or assignment"`
		AssigneeID   string `json:"assignee_id,omitempty" jsonschema:"For an assignment: the admin's id, from admins, or 0 for nobody"`
		SnoozedUntil int64  `json:"snoozed_until,omitempty" jsonschema:"For snoozed: until when, in Unix seconds"`
		Body         string `json:"body,omitempty" jsonschema:"A note to go with it"`
	}
	intercomContact struct {
		ID               string         `json:"id,omitempty" jsonschema:"The contact's id, to change it; a new contact when left out"`
		Role             string         `json:"role,omitempty" jsonschema:"user or lead, for a new contact"`
		Email            string         `json:"email,omitempty" jsonschema:"Their email"`
		Name             string         `json:"name,omitempty" jsonschema:"Their name"`
		Phone            string         `json:"phone,omitempty" jsonschema:"Their phone number"`
		ExternalID       string         `json:"external_id,omitempty" jsonschema:"Their id in your own systems"`
		CustomAttributes map[string]any `json:"custom_attributes,omitempty" jsonschema:"Custom attributes by name"`
	}
)
