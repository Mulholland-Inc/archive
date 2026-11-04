package connect

import "time"

// Karbon, the practice-management platform of accounting firms, through its API v3. It has no sign-in for apps: a
// firm's admin registers an API application with Karbon, and once Karbon approves it, Settings, Connected Apps, API
// Applications shows its Authorization token, a GUID, and its AccessKey, a JWT, which an owner pastes in to connect
// the firm for everyone. Karbon's webhooks need a subscription per kind, which it does not make yet: connecting,
// asking and reading start a sync.
func init() {
	Register(&Connector{
		ID: "karbon", Name: "Karbon", Integration: true,
		Info: Info{By: "Karbon", Description: "Read the firm’s clients, people and work items", Category: CatPractice, Icon: IconList,
			Scopes: []string{"Read contacts, organizations and work with the firm’s API keys"}},
		API: "https://api.karbonhq.com/v3",
		Keys: []Key{
			{Name: "token", Label: "Authorization token", Secret: true,
				Help: "In Karbon, Settings, Connected Apps, API Applications, your application: the Bearer token, a code like 60a39912-382f-40a2-…"},
			{Name: "access_key", Label: "AccessKey", Secret: true, Help: "On the same page: the AccessKey, which starts with eyJ"},
		},
		Sign: map[string]string{"Authorization": "Bearer {key/token}", "AccessKey": "{key/access_key}"},
		Who:  &Who{Req: Req{URL: "/TenantSettings"}, Names: []Path{"TenantName", "Name"}},
		Actions: map[string]*Action{
			"get": Call[karbonGet, any](Plan{
				Doc: "Read any of Karbon's GET endpoints, with OData's $filter, $orderby, $top and $expand, like { path: \"/WorkItems\", " +
					"query: \"$filter=WorkStatus eq 'In Progress'\" } or { path: \"/Organizations/<key>\", query: \"$expand=Contacts\" }: its answer, " +
					"{ value, @odata.nextLink } for a list (pass @odata.nextLink as page for more)",
				Steps: []Step{
					{If: "page", Req: Req{URL: "{+page}"}},
					{Unless: "page", Req: Req{URL: "{+path}?{+query|}"}},
				},
			}),
		},
		Sync: []*Sync{
			{Name: "organizations", Gap: time.Second, Req: Req{URL: "/Organizations", Query: karbonTop}, Items: "value", ID: "OrganizationKey", Updated: "LastModifiedDateTime",
				Page: karbonPage, Change: karbonChange,
				Doc: "Every organization, client or not, with its name, website, email, phone and contact type, by its OrganizationKey: listed whole the first time " +
					"and a week after, 100 a page, which deletes those gone, and in between those changed since the sync before, newest first.",
				To: Map{Table: "crm_company",
					Cols: map[string]SQL{"title": "data->>'FullName'", "domain": "archive.sync_domain(data->>'Website')", "stage": "data->>'ContactType'",
						"phone": "data->>'PhoneNumber'", "edited_at": "archive.sync_at(data->>'LastModifiedDateTime')"},
					Orgs: map[string]SQL{"org": "data->>'Website'"}}},
			{Name: "contacts", Gap: time.Second, Req: Req{URL: "/Contacts", Query: karbonTop}, Items: "value", ID: "ContactKey", Updated: "LastModifiedDateTime",
				Page: karbonPage, Change: karbonChange,
				Doc: "Every person, client or not, with their name, email, phone and contact type, by its ContactKey: listed whole the first time " +
					"and a week after, 100 a page, which deletes those gone, and in between those changed since the sync before, newest first.",
				To: Map{Table: "crm_contact",
					Cols: map[string]SQL{
						"first_name": `coalesce(nullif(data->>'FirstName', ''), regexp_replace(coalesce(nullif(btrim(data->>'FullName'), ''), btrim(concat(data->>'FirstName', ' ', data->>'LastName'))), ' [^ ]*$', ''))`,
						"last_name":  `coalesce(nullif(data->>'LastName', ''), substring(coalesce(nullif(btrim(data->>'FullName'), ''), btrim(concat(data->>'FirstName', ' ', data->>'LastName'))) from ' ([^ ]*)$'))`,
						"email":      "archive.sync_email(data->>'EmailAddress')", "phone": "data->>'PhoneNumber'", "stage": "data->>'ContactType'",
						"edited_at": "archive.sync_at(data->>'LastModifiedDateTime')"},
					People: map[string]SQL{"person": "data->>'EmailAddress'"}}},
			{Name: "work_items", Gap: time.Second, Req: Req{URL: "/WorkItems", Query: karbonTop}, Items: "value", ID: "WorkItemKey", Updated: "LastModifiedDateTime",
				Page: karbonPage, Change: karbonChange,
				Doc: "Every work item, with its title, type, status, client, assignee and dates, by its WorkItemKey: listed whole the first time " +
					"and a week after, 100 a page, which deletes those gone, and in between those changed since the sync before, newest first.",
				To: Map{Table: "work_item",
					Cols: map[string]SQL{"title": "data->>'Title'", "type": "data->>'WorkType'", "status": "coalesce(nullif(data->>'WorkStatus', ''), data->>'PrimaryStatus')",
						"client_name": "data->>'ClientName'", "client_source": "data->>'ClientKey'", "assignee_name": "data->>'AssigneeName'",
						"starts_on": "archive.sync_day(data->>'StartDate')", "due_on": "archive.sync_day(data->>'DueDate')", "done_on": "archive.sync_day(data->>'CompletedDate')",
						"description": "archive.sync_clip(data->>'Description', 65536)", "edited_at": "archive.sync_at(data->>'LastModifiedDateTime')"},
					People: map[string]SQL{"assignee": "data->>'AssigneeEmailAddress'"}}},
		},
	})
}

var (
	karbonTop    = Q{"$top": "100"}
	karbonPage   = NextURL{At: "@odata.nextLink"}
	karbonChange = NewestFirst{Query: Q{"$orderby": "LastModifiedDateTime desc"}, Expired: []int{400}}
)

type karbonGet struct {
	Path  string `json:"path,omitempty" jsonschema:"The endpoint after /v3, like /Contacts, /Organizations, /WorkItems, /Users or /Notes"`
	Query string `json:"query,omitempty" jsonschema:"Its OData query, like $filter=contains(FullName,'Smith')&$top=50"`
	Page  string `json:"page,omitempty" jsonschema:"The @odata.nextLink of the call before"`
}
