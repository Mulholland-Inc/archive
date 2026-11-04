package connect

// Otter.ai, through its hosted MCP server: the meetings Otter recorded that the person can see, searched with their
// overviews, and each one's transcript. Its authorization server registers any client (RFC 7591) and asks for
// profile:read and conversations:read. Otter's REST API is for Enterprise workspaces only, with keys an admin
// makes, so this reads through the server any plan signs in to. No tool is documented to name the user, so the
// account is Otter.ai.
//
// It syncs nothing until its tools' answers are documented; its actions are the server's tools.
func init() {
	Register(&Connector{ID: "otter", Name: "Otter.ai", MCP: "https://mcp.otter.ai/mcp",
		Info: Info{By: "Otter.ai", Description: "Search your recorded meetings and read their transcripts", Category: CatCalendar, Icon: IconNotebook,
			Scopes: []string{"Read your profile and the meetings shared with you"}}})
}
