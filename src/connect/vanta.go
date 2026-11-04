package connect

import "time"

// Vanta, the compliance platform, read only through its API. Vanta's partner apps cannot read a customer's
// compliance data, only an app of the customer's own can, so this connects as an app the customer makes in
// Vanta's Developer Console (Settings, Developer console, a "Manage Vanta" application), whose client id and secret
// an owner pastes in. Each is traded for a token with the scope vanta-api.all:read, which lasts an hour; Vanta ends
// the token before whenever another is made, so the app should be Archive's alone. The secret lasts until the
// customer rotates it or deletes the app. One host answers for every region.
//
// It syncs the organization's frameworks with how far along each is, its controls, its automated tests with their
// status and what to fix, its vendors and their risk, its people and their onboarding tasks, its policies and its
// documents, all into compliance, its kind saying which. Vanta says nothing of what changed when, so each is listed
// whole, 100 a page, at most an hour apart, which deletes what is gone. Vanta takes 50 requests a minute.
func init() {
	Register(&Connector{
		ID: "vanta", Name: "Vanta", Integration: true,
		Info: Info{By: "Vanta", Description: "Read frameworks, controls, tests, vendors, people, policies and documents", Category: CatCompliance, Icon: IconList,
			Scopes: []string{"Read the organization’s compliance data with an app it makes in Vanta"}},
		API:  "https://api.vanta.com/v1",
		Pace: 1250 * time.Millisecond,
		Keys: []Key{
			{Name: "client_id", Label: "Client ID", Secret: true,
				Help: "In Vanta, Settings, Developer console: create a Manage Vanta application for Archive and copy its client ID, which starts with vci_"},
			{Name: "client_secret", Label: "Client secret", Secret: true, Help: "On the same page: generate its client secret, which starts with vcs_"},
		},
		Trade: &Req{Method: POST, URL: "https://api.vanta.com/oauth/token",
			Body: Obj{"client_id": "{key/client_id}", "client_secret": "{key/client_secret}", "scope": "vanta-api.all:read", "grant_type": "client_credentials"}},
		Sign: map[string]string{"Authorization": "Bearer {key/access}"},
		Who:  &Who{Req: Req{URL: "/organization"}, Names: []Path{"displayName", "id"}},
		Actions: map[string]*Action{
			"get": Call[vantaGet, any](Plan{
				Doc: "Read any of Vanta's GET endpoints, like { path: \"/tests\", query: \"statusFilter=NEEDS_ATTENTION\" }, " +
					"{ path: \"/tests/<id>/entities\", query: \"entityStatus=FAILING\" } for what a test failed on, { path: \"/frameworks/<id>/controls\" }, " +
					"{ path: \"/people\", query: \"tasksSummaryStatusMatchesAny=OVERDUE\" }, /risk-scenarios, /vulnerabilities, /monitored-computers, /integrations " +
					"or /issues: its answer, a list as { results: { data, pageInfo: { endCursor, hasNextPage } } } (pass endCursor as pageCursor for more)",
				Steps: []Step{{Req: Req{URL: "{+path}?{+query|}"}}},
			}),
		},
		Sync: []*Sync{
			{Name: "frameworks", Gap: time.Second, Every: time.Hour, Req: Req{URL: "/frameworks", Query: vantaList(Q{"includeStatus": "true", "includeProgress": "true"})},
				Items: "results/data", Page: vantaPage,
				Doc: "Every framework, like SOC 2, with its status and how many of its controls are done, tests passing and documents ready: " +
					"listed whole at most an hour apart, 100 a page, which deletes those gone.",
				To: Map{Table: "compliance",
					Cols: map[string]SQL{"kind": "'framework'", "code": "data->>'shorthandName'", "title": "data->>'displayName'", "status": "data->>'status'",
						"passing": "archive.sync_num(data->>'numControlsCompleted')", "total": "archive.sync_num(data->>'numControlsTotal')",
						"description": "nullif(concat_ws(E'\\n', nullif(data->>'description', ''), " +
							"CASE WHEN data->>'numTestsTotal' IS NOT NULL THEN format('%s of %s tests passing; %s of %s documents ready', " +
							"data->>'numTestsPassing', data->>'numTestsTotal', data->>'numDocumentsPassing', data->>'numDocumentsTotal') END), '')"}}},
			{Name: "controls", Gap: time.Second, Every: time.Hour, Req: Req{URL: "/controls", Query: vantaList(nil)}, Items: "results/data", Updated: "modificationDate", Page: vantaPage,
				Doc: "Every control, with its code, domains, owner and description: listed whole at most an hour apart, 100 a page, which deletes those gone.",
				To: Map{Table: "compliance",
					Cols: map[string]SQL{"kind": "'control'", "code": "data->>'externalId'", "title": "data->>'name'", "category": vantaDomains,
						"owner_name": "data#>>'{owner,displayName}'", "description": "archive.sync_clip(data->>'description', 65536)",
						"made_at": "archive.sync_at(data->>'creationDate')", "edited_at": "archive.sync_at(data->>'modificationDate')"},
					People: map[string]SQL{"owner": "data#>>'{owner,emailAddress}'"}}},
			{Name: "tests", Gap: time.Second, Every: time.Hour, Req: Req{URL: "/tests", Query: vantaList(nil)}, Items: "results/data", Page: vantaPage,
				Doc: "Every automated test, with its status (NEEDS_ATTENTION is failing), category, owner, when it last ran and flipped, " +
					"when its fix is due, and what failed and how to fix it: listed whole at most an hour apart, 100 a page, which deletes those gone.",
				To: Map{Table: "compliance",
					Cols: map[string]SQL{"kind": "'test'", "title": "data->>'name'", "status": "data->>'status'", "category": "data->>'category'",
						"owner_name": "data#>>'{owner,displayName}'", "due_on": "archive.sync_day(data#>>'{remediationStatusInfo,soonestRemediateByDate}')",
						"checked_at": "archive.sync_at(data->>'lastTestRunDate')", "edited_at": "archive.sync_at(data->>'latestFlipDate')",
						"description": "archive.sync_clip(nullif(concat_ws(E'\\n\\n', nullif(data->>'description', ''), " +
							"CASE WHEN data->>'status' = 'NEEDS_ATTENTION' THEN nullif(data->>'failureDescription', '') END, nullif(data->>'remediationDescription', '')), ''), 65536)"},
					People: map[string]SQL{"owner": "data#>>'{owner,emailAddress}'"}}},
			{Name: "vendors", Gap: time.Second, Every: time.Hour, Req: Req{URL: "/vendors", Query: vantaList(nil)}, Items: "results/data", Page: vantaPage,
				Doc: "Every vendor, with its status, category, risk, website, services and security reviews: listed whole at most an hour apart, " +
					"100 a page, which deletes those gone.",
				To: Map{Table: "compliance",
					Cols: map[string]SQL{"kind": "'vendor'", "title": "data->>'name'", "status": "data->>'status'", "category": "data#>>'{category,displayName}'",
						"risk": "coalesce(nullif(data->>'residualRiskLevel', 'UNSCORED'), data->>'inherentRiskLevel')", "url": "nullif(data->>'websiteUrl', '')",
						"due_on": "archive.sync_day(data->>'nextSecurityReviewDueDate')", "checked_at": "archive.sync_at(data->>'lastSecurityReviewCompletionDate')",
						"description": "archive.sync_clip(nullif(concat_ws(E'\\n\\n', nullif(data->>'servicesProvided', ''), nullif(data->>'additionalNotes', '')), ''), 65536)"}}},
			{Name: "people", Gap: time.Second, Every: time.Hour, Req: Req{URL: "/people", Query: vantaList(nil)}, Items: "results/data", Page: vantaPage,
				Doc: "Every person, with their name, employment status and job title, and where their security tasks stand, like OVERDUE or COMPLETE, " +
					"and when due: listed whole at most an hour apart, 100 a page, which deletes those gone.",
				To: Map{Table: "compliance",
					Cols: map[string]SQL{"kind": "'person'", "title": vantaName, "owner_name": vantaName, "status": "data#>>'{tasksSummary,status}'",
						"category": "data#>>'{employment,status}'", "description": "data#>>'{employment,jobTitle}'",
						"due_on": "archive.sync_day(data#>>'{tasksSummary,dueDate}')", "checked_at": "archive.sync_at(data#>>'{tasksSummary,completionDate}')",
						"made_at": "archive.sync_at(data#>>'{employment,startDate}')"},
					People: map[string]SQL{"owner": "data->>'emailAddress'"}}},
			{Name: "policies", Gap: time.Second, Every: time.Hour, Req: Req{URL: "/policies", Query: vantaList(nil)}, Items: "results/data", Page: vantaPage,
				Doc: "Every policy, with where its latest version stands, like APPROVED, RENEW_SOON or EXPIRED, and when it was approved: listed whole " +
					"at most an hour apart, 100 a page, which deletes those gone.",
				To: Map{Table: "compliance",
					Cols: map[string]SQL{"kind": "'policy'", "title": "data->>'name'", "status": "coalesce(data#>>'{latestVersion,status}', data->>'status')",
						"checked_at": "archive.sync_at(data->>'approvedAtDate')", "description": "archive.sync_clip(data->>'description', 65536)"}}},
			{Name: "documents", Gap: time.Second, Every: time.Hour, Req: Req{URL: "/documents", Query: vantaList(nil)}, Items: "results/data", Page: vantaPage,
				Doc: "Every document Vanta asks for as evidence, with its category and whether it is uploaded, like OK or Needs document: listed whole " +
					"at most an hour apart, 100 a page, which deletes those gone. Not the files.",
				To: Map{Table: "compliance",
					Cols: map[string]SQL{"kind": "'document'", "title": "data->>'title'", "status": "data->>'uploadStatus'", "category": "data->>'category'",
						"checked_at": "archive.sync_at(data->>'uploadStatusDate')", "url": "nullif(data->>'url', '')", "description": "archive.sync_clip(data->>'description', 65536)"}}},
		},
	})
}

var vantaPage = Cursor{At: "results/pageInfo/endCursor", More: "results/pageInfo/hasNextPage"}

// vantaName is a person's name.
const vantaName SQL = "coalesce(nullif(data#>>'{name,display}', ''), nullif(btrim(concat(data#>>'{name,first}', ' ', data#>>'{name,last}')), ''), data->>'emailAddress')"

// vantaDomains are a control's domains, comma separated.
const vantaDomains SQL = "(SELECT string_agg(d, ', ') FROM jsonb_array_elements_text(CASE WHEN jsonb_typeof(data->'domains') = 'array' THEN data->'domains' ELSE '[]' END) d)"

// vantaList is a listing's query: q, 100 a page, from {page}.
func vantaList(q Q) Q {
	out := Q{"pageSize": "100", "pageCursor": "{page}"}
	for k, v := range q {
		out[k] = v
	}
	return out
}

type vantaGet struct {
	Path  string `json:"path" jsonschema:"The endpoint after /v1, like /tests, /controls/<id>, /vendors or /people"`
	Query string `json:"query,omitempty" jsonschema:"Its query, like statusFilter=NEEDS_ATTENTION&pageSize=100"`
}
