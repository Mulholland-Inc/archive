package connect

import (
	"time"

	"golang.org/x/oauth2"
)

// Jira Cloud, through its REST API v3, as an OAuth 2.0 (3LO) app of Atlassian's developer console: its
// callback URL <API>/connect/jira/callback, its permissions the Jira API with the classic Scopes here, and its
// distribution sharing on, so other sites can consent to it. Atlassian's tokens are for every site the person let
// the app into, and its API is per site, at api.atlassian.com/ex/jira/<cloud id>: the first site
// accessible-resources lists is the one connected. The refresh token is replaced on every refresh, and lapses
// after 90 days unused. Atlassian has no endpoint that revokes a grant: the person removes the app under their
// Atlassian account's Connected apps.
//
// It syncs every issue the person can see into work_item, its key and summary as its title, its description's
// words and a link to it on the site. Its webhooks, made per app with a JQL filter and gone after 30 days, are not
// asked for yet: connecting, asking and reading start a sync.
func init() {
	Register(&Connector{
		ID: "jira", Name: "Jira", Integration: true,
		Info: Info{By: "Atlassian", Description: "Read projects and issues, and add, change, move and comment on issues when you ask", Category: CatProjects, Icon: IconList,
			Scopes: []string{"Read projects, issues, comments and people of your Jira site", "Add, change, move and comment on issues when you ask"}},
		// The site's API, by the cloud id of the first site the token is for.
		API: "https://api.atlassian.com/ex/jira/{cloud}/rest/api/3",
		Let: []Step{
			{As: "sites", Req: jiraSites},
			{As: "cloud", In: "{sites/0/id}"},
			{Unless: "cloud", Fail: "Jira named no site this connection may read; connect it again and choose your site"},
		},
		Endpoint: oauth2.Endpoint{AuthURL: "https://auth.atlassian.com/authorize", TokenURL: "https://auth.atlassian.com/oauth/token",
			AuthStyle: oauth2.AuthStyleInParams},
		Params: map[string]string{"audience": "api.atlassian.com", "prompt": "consent"},
		Scopes: []string{"read:jira-work", "write:jira-work", "read:jira-user", "offline_access"},
		Who:    &Who{Req: jiraSites, Names: []Path{"0/url", "0/name"}, Lower: true},
		Actions: map[string]*Action{
			"search": Call[jiraFind, jiraFound](Plan{
				Doc: "Find issues with JQL, like { jql: \"project = OPS AND statusCategory != Done ORDER BY updated DESC\" }, which has to name something to " +
					"narrow it by, like a project, assignee or date: { issues: [{ id, key, fields: { summary, status, issuetype, assignee, duedate, updated, ... } }], next } " +
					"(pass next as page for more)",
				Clamp: map[string][2]int{"limit": {50, 100}},
				Steps: []Step{{As: "got", Req: Req{URL: "/search/jql", Query: Q{"jql": "{jql}", "fields": "{fields,|" + jiraFields + "}", "maxResults": "{limit}", "nextPageToken": "{page}"}}}},
				Out:   Obj{"issues": "{got/issues|[]}", "next": "{got/nextPageToken|}"},
			}),
			"issue": Call[jiraOne, jiraIssue](Plan{
				Doc: "One issue with its fields and its 50 latest comments, newest first: { issue: { id, key, fields: { summary, description, status, assignee, ... } }, " +
					"comments: [{ id, author, body, created }] }; a description and a comment's body are Atlassian Document Format, its words under text",
				Steps: []Step{
					{As: "issue", Req: Req{URL: "/issue/{key}"}},
					{As: "comments", Req: Req{URL: "/issue/{key}/comment", Query: Q{"orderBy": "-created", "maxResults": "50"}}},
				},
				Out: Obj{"issue": "{issue}", "comments": "{comments/comments|[]}"},
			}),
			"projects": Call[jiraProjects, jiraFoundProjects](Plan{
				Doc:   "The projects you can see, maybe only those whose name or key has some words: { projects: [{ id, key, name, projectTypeKey }], total } (pass how many you have as page for more)",
				Clamp: map[string][2]int{"limit": {50, 100}, "page": {0, 1 << 30}},
				Steps: []Step{{As: "got", Req: Req{URL: "/project/search", Query: Q{"query": "{query}", "maxResults": "{limit}", "startAt": "{page}"}}}},
				Out:   Obj{"projects": "{got/values|[]}", "total": "{got/total|0}"},
			}),
			"users": Call[jiraPeople, []map[string]any](Plan{
				Doc:   "People of the site whose name or email has some words, for the accountId an issue's assignee is: [{ accountId, displayName, emailAddress, active }]",
				Steps: []Step{{Req: Req{URL: "/user/search", Query: Q{"query": "{query}", "maxResults": "20"}}}},
			}),
			"transitions": Call[jiraOne, []map[string]any](Plan{
				Doc:   "Where an issue can move from its status: [{ id, name, to: { name, statusCategory } }], each id one for transition",
				Steps: []Step{{As: "got", Req: Req{URL: "/issue/{key}/transitions"}}},
				Out:   "{got/transitions|[]}",
			}),
			"create": Call[jiraNew, jiraMade](Plan{
				Doc: "Add an issue to a project, like { project: \"OPS\", type: \"Task\", summary: \"Renew the lease\" }: { id, key, self }",
				Steps: []Step{
					{Unless: "project", Fail: jiraNeeds}, {Unless: "summary", Fail: jiraNeeds},
					jiraDoc, jiraAssignee, jiraParent,
					{Req: Req{Method: POST, URL: "/issue", Body: Obj{"fields": Obj{"project": Obj{"key": "{project}"}, "issuetype": Obj{"name": "{type|Task}"}, "summary": "{summary}",
						"description": "{doc}", "assignee": "{assignee_of}", "duedate": "{due}", "labels": "{labels}", "parent": "{parent_of}"}}}},
				},
			}),
			"update": Call[jiraEdit, jiraDone](Plan{
				Doc:   "Change an issue: only what you give changes, and labels, when given, replace its labels. { key }",
				Steps: []Step{jiraDoc, jiraAssignee, {Req: Req{Method: PUT, URL: "/issue/{key}", Body: Obj{"fields": jiraChange}}}},
				Out:   Obj{"key": "{key}"},
			}),
			"transition": Call[jiraMove, jiraDone](Plan{
				Doc:   "Move an issue to another status by a transition's id, from transitions: { key }",
				Steps: []Step{{Req: Req{Method: POST, URL: "/issue/{key}/transitions", Body: Obj{"transition": Obj{"id": "{transition}"}}}}},
				Out:   Obj{"key": "{key}"},
			}),
			"comment": Call[jiraSay, map[string]any](Plan{
				Doc:   "Comment on an issue, as you, in plain text: the comment, { id, author, body, created }",
				Steps: []Step{{Req: Req{Method: POST, URL: "/issue/{key}/comment", Body: Obj{"body": jiraComment}}}},
			}),
		},
		Sync: []*Sync{
			{Name: "issues", Gap: time.Second, Req: Req{URL: "/search/jql", Query: Q{"jql": "project is not EMPTY ORDER BY id ASC", "fields": jiraFields + ",description", "maxResults": "100", "nextPageToken": "{page}"}},
				Items: "issues", Updated: "fields/updated", Page: Cursor{At: "nextPageToken"}, Change: NewestFirst{Query: Q{"jql": "project is not EMPTY ORDER BY updated DESC"}},
				// The site's address, for each issue's link, which the API's answers do not give.
				Hydrate: []Hydrate{Lookup{Req: jiraSites, As: "sites", Soft: true}},
				Doc: "Every issue of every project you can see, by its id, with its key and summary, type, status, assignee, dates and the words of its description, cut at 64 KB: " +
					"listed whole the first time and a week after, 100 a page, which deletes those gone, and in between those updated since the sync before, newest first.",
				To: Map{Table: "work_item",
					Cols: map[string]SQL{"title": "concat_ws(' ', data->>'key', data#>>'{fields,summary}')", "type": "data#>>'{fields,issuetype,name}'", "status": "data#>>'{fields,status,name}'",
						"assignee_name": "data#>>'{fields,assignee,displayName}'", "due_on": "archive.sync_day(data#>>'{fields,duedate}')",
						"done_on":     "archive.sync_day(data#>>'{fields,resolutiondate}')",
						"description": `archive.sync_clip((SELECT string_agg(t #>> '{}', ' ') FROM jsonb_path_query(data#>'{fields,description}', 'strict $.**.text') t), 65536)`,
						"url":         "nullif(rtrim(data#>>'{sites,0,url}', '/'), '') || '/browse/' || (data->>'key')",
						"made_at":     "archive.sync_at(data#>>'{fields,created}')", "edited_at": "archive.sync_at(data#>>'{fields,updated}')"},
					People: map[string]SQL{"assignee": "data#>>'{fields,assignee,emailAddress}'"}}},
		},
	})
}

const (
	jiraFields = "summary,status,issuetype,project,assignee,reporter,priority,labels,parent,duedate,created,updated,resolutiondate"
	jiraNeeds  = "an issue needs a project and a summary"
)

var (
	// jiraSites are the sites the token is for: [{ id, name, url, scopes }], the id the cloud id of the site's API.
	jiraSites    = Req{URL: "https://api.atlassian.com/oauth/token/accessible-resources"}
	jiraDoc      = Step{If: "description", As: "doc", In: Obj{"type": "doc", "version": 1, "content": []Obj{{"type": "paragraph", "content": []Obj{{"type": "text", "text": "{description}"}}}}}}
	jiraAssignee = Step{If: "assignee", As: "assignee_of", In: Obj{"accountId": "{assignee}"}}
	jiraParent   = Step{If: "parent", As: "parent_of", In: Obj{"key": "{parent}"}}
	// jiraComment is the text in Atlassian Document Format, the one paragraph.
	jiraComment = Obj{"type": "doc", "version": 1, "content": []Obj{{"type": "paragraph", "content": []Obj{{"type": "text", "text": "{text}"}}}}}
	jiraChange  = Obj{"summary": "{summary}", "description": "{doc}", "assignee": "{assignee_of}", "duedate": "{due}", "labels": "{labels}"}
)

type (
	jiraFind struct {
		JQL    string   `json:"jql" jsonschema:"The JQL, like assignee = currentUser() AND resolution = Unresolved ORDER BY duedate"`
		Fields []string `json:"fields,omitempty" jsonschema:"The fields to return, like summary, status, description or a custom field's id; the usual ones when left out"`
		Limit  int      `json:"limit,omitempty" jsonschema:"At most this many, 1 to 100; 50 when left out"`
		Page   string   `json:"page,omitempty" jsonschema:"The next of the search before, for the issues after it"`
	}
	jiraOne struct {
		Key string `json:"key" jsonschema:"The issue's key, like OPS-12, or its id"`
	}
	jiraProjects struct {
		Query string `json:"query,omitempty" jsonschema:"Words in the project's name or key"`
		Limit int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 100; 50 when left out"`
		Page  int    `json:"page,omitempty" jsonschema:"How many to skip, for those after them"`
	}
	jiraPeople struct {
		Query string `json:"query" jsonschema:"Words in their name or email address"`
	}
	jiraNew struct {
		Project     string   `json:"project" jsonschema:"The project's key, like OPS"`
		Type        string   `json:"type,omitempty" jsonschema:"The issue type's name, like Task, Bug, Story or Epic; Task when left out"`
		Summary     string   `json:"summary" jsonschema:"Its title"`
		Description string   `json:"description,omitempty" jsonschema:"What it is about, in plain text"`
		Assignee    string   `json:"assignee,omitempty" jsonschema:"The accountId of who it is for, from users"`
		Due         string   `json:"due,omitempty" jsonschema:"When it is due, like 2026-11-30"`
		Labels      []string `json:"labels,omitempty" jsonschema:"Its labels, without spaces"`
		Parent      string   `json:"parent,omitempty" jsonschema:"The key of the epic or issue it belongs under, for a subtask or a child of an epic"`
	}
	jiraEdit struct {
		Key         string   `json:"key" jsonschema:"The issue's key, like OPS-12"`
		Summary     string   `json:"summary,omitempty" jsonschema:"Its new title"`
		Description string   `json:"description,omitempty" jsonschema:"Its new description, in plain text, in place of the one it has"`
		Assignee    string   `json:"assignee,omitempty" jsonschema:"The accountId of who it is for now, from users"`
		Due         string   `json:"due,omitempty" jsonschema:"When it is due now, like 2026-11-30"`
		Labels      []string `json:"labels,omitempty" jsonschema:"All its labels from now on"`
	}
	jiraMove struct {
		Key        string `json:"key" jsonschema:"The issue's key, like OPS-12"`
		Transition string `json:"transition" jsonschema:"The transition's id, from transitions"`
	}
	jiraSay struct {
		Key  string `json:"key" jsonschema:"The issue's key, like OPS-12"`
		Text string `json:"text" jsonschema:"The comment, in plain text"`
	}
	jiraFound struct {
		Issues []map[string]any `json:"issues"`
		Next   string           `json:"next" jsonschema:"Pass as page for more; empty when there is no more"`
	}
	jiraFoundProjects struct {
		Projects []map[string]any `json:"projects"`
		Total    int              `json:"total" jsonschema:"How many there are in all"`
	}
	jiraIssue struct {
		Issue    map[string]any   `json:"issue"`
		Comments []map[string]any `json:"comments"`
	}
	jiraMade struct {
		ID   string `json:"id"`
		Key  string `json:"key"`
		Self string `json:"self"`
	}
	jiraDone struct {
		Key string `json:"key"`
	}
)
