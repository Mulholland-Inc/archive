package connect

import (
	"time"

	"golang.org/x/oauth2"
)

// Zendesk Support: tickets, the people who ask and answer them, and their organizations, read, and when asked
// tickets made, changed and commented on. Zendesk signs in at each account's own subdomain, which an owner gives
// first, and lets an app used by many accounts in only with a global OAuth client: a client made in Mulholland's
// own Zendesk, with the identifier zdg-archive, its redirect URL <API>/connect/zendesk/callback, and that Zendesk
// converts once it approves it, allowing exactly the Scopes here. Its tokens last 30 minutes, and their refresh
// tokens 30 days, replaced on every refresh; disconnecting revokes the token in use, which ends the grant.
//
// It syncs organizations into crm_company, users, agents and end users alike, into crm_contact, and tickets into
// work_item, each by its id. Organizations are listed whole every sync, as Zendesk sorts them no other way; users
// and tickets are listed whole the first time and a week after, and in between read newest change first down to
// the sync before. Comments are not synced but read with ticket. It pushes nothing yet: Zendesk's webhooks are made
// per account, with a trigger for each kind of change.
func init() {
	Register(&Connector{
		ID: "zendesk", Name: "Zendesk", Integration: true, Host: "https://zendesk.com",
		Info: Info{By: "Zendesk", Description: "Read tickets, users and organizations, and update or answer tickets when you ask", Category: CatSupport, Icon: IconInbox,
			Scopes: []string{"Read tickets, users and organizations", "Make, change and comment on tickets when you ask"}},
		Ask: &Key{Name: "subdomain", Label: "Zendesk subdomain", Help: "The name before .zendesk.com in your Zendesk’s address: acme, for acme.zendesk.com",
			Find: `^([a-z0-9][a-z0-9-]*)$`},
		API: "https://{meta/subdomain}.zendesk.com/api/v2",
		Endpoint: oauth2.Endpoint{AuthURL: "https://{meta/subdomain}.zendesk.com/oauth/authorizations/new", TokenURL: "https://{meta/subdomain}.zendesk.com/oauth/tokens",
			AuthStyle: oauth2.AuthStyleInParams},
		RevokeBy: &Req{Method: DELETE, URL: "https://{meta/subdomain}.zendesk.com/api/v2/oauth/tokens/current.json", Header: map[string][]string{"Authorization": {"Bearer {access}"}}},
		Scopes:   []string{"tickets:read", "tickets:write", "users:read", "organizations:read"},
		Who:      &Who{Req: Req{URL: "/users/me.json"}, Names: []Path{"user/email"}, Lower: true},
		Actions: map[string]*Action{
			"search": Call[zendeskFind, zendeskFound](Plan{
				Doc: "Find tickets, users or organizations with Zendesk's search syntax, like { query: \"type:ticket status<solved requester:jane@example.com\" } " +
					"or { query: \"type:user acme\" }, best match first: { results: [{ result_type, id, ... }], count, next } (pass next as page for more)",
				Clamp: map[string][2]int{"limit": {25, 100}},
				Steps: []Step{
					{If: "page", As: "got", Req: Req{URL: "{+page}"}},
					{Unless: "page", As: "got", Req: Req{URL: "/search.json", Query: Q{"query": "{query}", "per_page": "{limit}"}}},
				},
				Out: Obj{"results": "{got/results|[]}", "count": "{got/count|0}", "next": "{got/next_page|}"},
			}),
			"ticket": Call[zendeskOne, zendeskFull](Plan{
				Doc: "One ticket with its comments, oldest first, public replies and internal notes alike: { ticket: { id, subject, status, priority, type, " +
					"requester_id, assignee_id, organization_id, tags, due_at, ... }, comments: [{ id, author_id, body, public, created_at }] }",
				Steps: []Step{
					{As: "ticket", Req: Req{URL: "/tickets/{id}.json"}},
					{As: "comments", Req: Req{URL: "/tickets/{id}/comments.json"}, Page: NextURL{At: "next_page"}, Items: "comments"},
				},
				Out: Obj{"ticket": "{ticket/ticket}", "comments": "{comments|[]}"},
			}),
			"create": Call[zendeskNew, zendeskTicket](Plan{
				Doc: "Make a ticket, its first comment its description, for a requester by email, who is added as a user when Zendesk has none: " +
					"like { subject, body, requester: { email, name } }. Zendesk's triggers may email the requester, as for any new ticket. Returns the ticket",
				Enum: map[string][]string{"priority": zendeskPriorities, "type": zendeskTypes},
				Steps: []Step{{As: "got", Req: Req{Method: POST, URL: "/tickets.json", Body: Obj{"ticket": Obj{"subject": "{subject}",
					"comment": Obj{"body": "{body}"}, "requester": "{requester}", "priority": "{priority}", "type": "{type}",
					"assignee_email": "{assignee_email}", "tags": "{tags}"}}}}},
				Out: "{got/ticket}",
			}),
			"update": Call[zendeskEdit, zendeskTicket](Plan{
				Doc:  "Change a ticket's status, priority, type, assignee or tags: only what is given changes, and tags replace the ticket's. Returns the ticket",
				Enum: map[string][]string{"status": zendeskStatuses, "priority": zendeskPriorities, "type": zendeskTypes},
				Steps: []Step{{As: "got", Req: Req{Method: PUT, URL: "/tickets/{id}.json", Body: Obj{"ticket": Obj{"status": "{status}", "priority": "{priority}",
					"type": "{type}", "assignee_email": "{assignee_email}", "tags": "{tags}"}}}}},
				Out: "{got/ticket}",
			}),
			"comment": Call[zendeskSay, zendeskTicket](Plan{
				Doc: "Add a comment to a ticket: an internal note, which only agents see, unless public, a reply Zendesk emails to the requester and " +
					"whoever is copied. With status, the ticket moves to it too, like solved. Returns the ticket",
				Enum: map[string][]string{"status": zendeskStatuses},
				Steps: []Step{{As: "got", Req: Req{Method: PUT, URL: "/tickets/{id}.json", Body: Obj{"ticket": Obj{"status": "{status}",
					"comment": Obj{"body": "{body}", "public": "{public|false}"}}}}}},
				Out: "{got/ticket}",
			}),
		},
		Sync: []*Sync{
			{Name: "organizations", Gap: time.Second, Req: Req{URL: "/organizations.json", Query: zendeskPage}, Items: "organizations", Updated: "updated_at", Page: zendeskCursor,
				Doc: "Every organization, with its name, first domain, notes and details, by its id: listed whole every sync, 100 a page, which deletes those gone.",
				To: Map{Table: "crm_company",
					Cols: map[string]SQL{"title": "data->>'name'", "domain": zendeskDomain, "description": "archive.sync_clip(concat_ws(E'\\n\\n', nullif(data->>'details', ''), nullif(data->>'notes', '')), 65536)",
						"made_at": "archive.sync_at(data->>'created_at')", "edited_at": "archive.sync_at(data->>'updated_at')"},
					Orgs: map[string]SQL{"org": zendeskDomain}}},
			{Name: "users", Gap: time.Second, Req: Req{URL: "/users.json", Query: zendeskPage}, Items: "users", Updated: "updated_at", Page: zendeskCursor, Change: zendeskChange,
				Gone: []Test{{At: "active", Is: "false"}},
				Doc: "Every user, end users, agents and admins alike, with their name, email, phone, role and organization, by its id: listed whole the first time " +
					"and a week after, 100 a page, which deletes those gone, and in between those changed since the sync before, newest first.",
				To: Map{Table: "crm_contact",
					Cols: map[string]SQL{"first_name": `regexp_replace(btrim(data->>'name'), ' [^ ]*$', '')`, "last_name": `substring(btrim(data->>'name') from ' ([^ ]*)$')`,
						"email": "archive.sync_email(data->>'email')", "phone": "data->>'phone'", "stage": "data->>'role'", "company_source": "data->>'organization_id'",
						"made_at": "archive.sync_at(data->>'created_at')", "edited_at": "archive.sync_at(data->>'updated_at')"},
					People: map[string]SQL{"person": "data->>'email'"},
					Refs:   map[string]SQL{"company": "data->>'organization_id'"}}},
			// The address of the agent a ticket is assigned to, which a ticket does not say, looked up once a page for each.
			{Name: "tickets", Gap: 2 * time.Second, Req: Req{URL: "/tickets.json", Query: zendeskPage}, Items: "tickets", Updated: "updated_at", Page: zendeskCursor, Change: zendeskChange,
				Gone:    []Test{{At: "status", Is: "deleted"}},
				Hydrate: []Hydrate{Fetch{If: Test{At: "assignee_id"}, Once: "{assignee_id}", Soft: true, Req: Req{URL: "/users/{assignee_id}.json"}, As: "assignee"}},
				Doc: "Every ticket, with its subject, type, status, priority, organization, assignee, due date and description, its first comment, cut at 64 KB, " +
					"by its id: listed whole the first time and a week after, 100 a page, which deletes those gone, and in between those changed since the sync before, newest first.",
				To: Map{Table: "work_item",
					Cols: map[string]SQL{"title": "data->>'subject'", "type": "coalesce(data->>'type', 'ticket')", "status": "data->>'status'",
						"client_source": "data->>'organization_id'", "assignee_name": "data#>>'{assignee,user,name}'", "due_on": "archive.sync_day(data->>'due_at')",
						"done_on":     "CASE WHEN data->>'status' IN ('solved', 'closed') THEN archive.sync_day(data->>'updated_at') END",
						"description": "archive.sync_clip(data->>'description', 65536)", "url": `regexp_replace(data->>'url', '/api/v2/tickets/([0-9]+)\.json$', '/agent/tickets/\1')`,
						"made_at": "archive.sync_at(data->>'created_at')", "edited_at": "archive.sync_at(data->>'updated_at')"},
					People: map[string]SQL{"assignee": "data#>>'{assignee,user,email}'"}}},
		},
	})
}

const zendeskDomain SQL = "archive.sync_domain(data#>>'{domain_names,0}')"

var (
	zendeskPage   = Q{"page[size]": "100", "page[after]": "{page}"}
	zendeskCursor = Cursor{At: "meta/after_cursor", More: "meta/has_more"}
	// A listing that will not sort so lists whole.
	zendeskChange     = NewestFirst{Query: Q{"sort": "-updated_at"}, Expired: []int{400}}
	zendeskStatuses   = []string{"new", "open", "pending", "hold", "solved", "closed"}
	zendeskPriorities = []string{"low", "normal", "high", "urgent"}
	zendeskTypes      = []string{"question", "incident", "problem", "task"}
)

type (
	zendeskFind struct {
		Query string `json:"query,omitempty" jsonschema:"Words and Zendesk search terms, like type:ticket status:open, type:user role:end-user, or organization:acme"`
		Limit int    `json:"limit,omitempty" jsonschema:"At most this many a page, 1 to 100; 25 when left out"`
		Page  string `json:"page,omitempty" jsonschema:"The next of the search before, for the results after it"`
	}
	zendeskFound struct {
		Results []map[string]any `json:"results"`
		Count   int              `json:"count"`
		Next    string           `json:"next" jsonschema:"Pass as page for more; empty when there is no more"`
	}
	zendeskOne struct {
		ID string `json:"id" jsonschema:"The ticket's id, its number"`
	}
	zendeskFull struct {
		Ticket   map[string]any   `json:"ticket"`
		Comments []map[string]any `json:"comments"`
	}
	zendeskPerson struct {
		Email string `json:"email" jsonschema:"Their email address"`
		Name  string `json:"name,omitempty" jsonschema:"Their name, for someone Zendesk does not have yet"`
	}
	zendeskNew struct {
		Subject       string        `json:"subject" jsonschema:"What it is about"`
		Body          string        `json:"body" jsonschema:"Its first comment, the description, in plain text"`
		Requester     zendeskPerson `json:"requester" jsonschema:"Who it is for"`
		Priority      string        `json:"priority,omitempty" jsonschema:"low, normal, high or urgent"`
		Type          string        `json:"type,omitempty" jsonschema:"question, incident, problem or task"`
		AssigneeEmail string        `json:"assignee_email,omitempty" jsonschema:"The email of the agent to assign it to"`
		Tags          []string      `json:"tags,omitempty" jsonschema:"Its tags"`
	}
	zendeskEdit struct {
		ID            string   `json:"id" jsonschema:"The ticket's id"`
		Status        string   `json:"status,omitempty" jsonschema:"new, open, pending, hold, solved or closed"`
		Priority      string   `json:"priority,omitempty" jsonschema:"low, normal, high or urgent"`
		Type          string   `json:"type,omitempty" jsonschema:"question, incident, problem or task"`
		AssigneeEmail string   `json:"assignee_email,omitempty" jsonschema:"The email of the agent to assign it to"`
		Tags          []string `json:"tags,omitempty" jsonschema:"Its tags, in place of those it has"`
	}
	zendeskSay struct {
		ID     string `json:"id" jsonschema:"The ticket's id"`
		Body   string `json:"body" jsonschema:"The comment, in plain text"`
		Public bool   `json:"public,omitempty" jsonschema:"true to reply to the requester, by email; an internal note when left out"`
		Status string `json:"status,omitempty" jsonschema:"What the ticket moves to with it, like pending or solved"`
	}
	zendeskTicket struct {
		ID         int    `json:"id"`
		Subject    string `json:"subject"`
		Status     string `json:"status"`
		Priority   string `json:"priority,omitempty"`
		Type       string `json:"type,omitempty"`
		AssigneeID int    `json:"assignee_id,omitempty"`
		UpdatedAt  string `json:"updated_at"`
	}
)
