package connect

// Linear, through its hosted MCP server: issues, projects, cycles, comments and documents. Its authorization
// server registers any client (RFC 7591). No tool is documented to name the user, so the account is Linear.
//
// Linear documents no input schema for its tools, so each stream reads the one the server lists: list_issues,
// list_projects and list_documents are pulled for what changed since, 250 a page, when their schema takes
// updatedAt and a cursor, as third parties describe them, and listed whole each sync when not.
func init() {
	Register(&Connector{ID: "linear", Name: "Linear", Integration: true, MCP: "https://mcp.linear.app/mcp",
		Streams: []*Stream{
			linearList("teams", "list_teams", "Your teams"),
			linearList("projects", "list_projects", "Every project"),
			linearList("issues", "list_issues", "Every issue, archived ones too"),
			linearList("documents", "list_documents", "Every document"),
		}})
}

func linearList(name, tool, what string) *Stream {
	l := &mcpList{tool: tool, opts: map[string]any{"orderBy": "updatedAt", "includeArchived": true}, limit: "limit", max: 250,
		after: "after,cursor", since: "updatedAt", lastID: true}
	return l.stream(name, what+", as "+tool+" answers, by id: those updated since the pull before when the tool takes updatedAt, 250 a page; "+
		"listed whole each time when it does not, so one gone is deleted.")
}
