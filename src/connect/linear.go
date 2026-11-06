package connect

import "time"

func init() {
	Register(&Connector{ID: "linear", Name: "Linear", Integration: true, MCP: "https://mcp.linear.app/mcp", Busy: ToolBusy,
		Info: Info{By: "Linear", Description: "Read issues, and search and work with issues, projects and cycles", Category: CatProjects, Icon: IconList, Scopes: []string{"Read and write issues as you"}},
		Sync: []*Sync{{Name: "issues", Gap: time.Second, Req: Req{Tool: "list_issues", Body: Obj{"orderBy": "createdAt", "includeArchived": true}},
			Items: "**", Key: []Reads{{Of: "{id}"}, {Of: "{identifier}"}}, Updated: "updatedAt", Page: ToolPage{Limit: "limit", Max: 250, After: "cursor,after"}, Keep: true,
			Change: SinceTime{Req: Req{Tool: "list_issues", Body: Obj{"orderBy": "updatedAt", "includeArchived": true, "updatedAt": "{since}"}}},
			Doc: "Every issue, archived ones too, with its title, status, team, assignee and dates, from list_issues, 250 a page: listed whole the first time and a week after, " +
				"and in between those updated since the sync before. A listing deletes nothing, so an issue deleted in Linear stays here.",
			To: Map{Table: "work_item",
				Cols: map[string]SQL{"title": "data->>'title'", "type": linearName("team"), "status": "coalesce(" + linearName("status") + ", " + linearName("state") + ")",
					"assignee_name": linearName("assignee"), "due_on": "archive.sync_day(data->>'dueDate')", "done_on": "archive.sync_at(data->>'completedAt')::date",
					"description": "archive.sync_clip(data->>'description', 65536)", "url": "data->>'url'",
					"made_at": "archive.sync_at(data->>'createdAt')", "edited_at": "archive.sync_at(data->>'updatedAt')"},
				People: map[string]SQL{"assignee": "data#>>'{assignee,email}'"}}}}})
}

func linearName(key string) SQL {
	return SQL("CASE jsonb_typeof(data->'" + key + "') WHEN 'object' THEN data#>>'{" + key + ",name}' ELSE data->>'" + key + "' END")
}
