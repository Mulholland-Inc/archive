package connect

import "time"

// Sentry, through its hosted MCP server: organizations, projects, issues, events and traces. Its authorization
// server registers any client (RFC 7591) and asks for the scopes its metadata lists. whoami names the user.
//
// It syncs each organization's projects into code_project, by slug, the only thing find_projects says of one. Issues are
// not synced: search_issues answers in Markdown, at most 100 and with no next page, and Archive syncs only from JSON.
// A listing deletes nothing, so a project deleted there stays here. Its actions are the server's tools.
func init() {
	Register(&Connector{ID: "sentry", Name: "Sentry", Integration: true, MCP: "https://mcp.sentry.dev/mcp", Who: &Who{Tool: "whoami"}, Busy: ToolBusy,
		Info: Info{By: "Sentry", Description: "List projects, and look into issues, events and traces", Category: CatCode, Icon: IconBug, Scopes: []string{"Read your organization’s projects and issues"}},
		Sync: []*Sync{{Name: "projects", Gap: time.Second, Every: time.Hour, Keep: true, Parts: sentryOrgs,
			Req:   Req{Tool: "find_projects", Body: Obj{"organizationSlug": "{in/org}", "regionUrl": "{in/region}", "cursor": "{page}"}},
			Items: "projects", Key: []Reads{{Of: "{in/org}/{slug}"}}, Page: sentryPage, Set: Obj{"organization": "{in/org}"},
			Doc: "Every project of each organization, by <organization>/<slug>, from find_projects, 25 a page: listed at most an hour apart. A listing deletes nothing, so a project deleted in Sentry stays here.",
			To:  Map{Table: "code_project", Cols: map[string]SQL{"title": "data->>'slug'", "organization": "data->>'organization'"}}}}})
}

var (
	sentryPage = Cursor{At: "nextCursor", More: "hasMore"}
	sentryOrgs = &Parts{Req: Req{Tool: "find_organizations", Body: Obj{"cursor": "{page}"}}, Items: "organizations", Page: sentryPage, ID: "{slug}", Vals: Obj{"org": "{slug}", "region": "{regionUrl}"}}
)
