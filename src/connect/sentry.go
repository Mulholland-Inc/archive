package connect

// Sentry, through its hosted MCP server: organizations, projects, issues, events and traces. Its authorization
// server registers any client (RFC 7591) and asks for the scopes its metadata lists. whoami names the user.
//
// It syncs nothing until its projects have tables in archive.sql; its actions are the server's tools.
func init() {
	Register(&Connector{ID: "sentry", Name: "Sentry", Integration: true, MCP: "https://mcp.sentry.dev/mcp", Account: mcpAccount("whoami")})
}
