package connect

// Sentry, through its hosted MCP server: organizations, projects, issues, events and traces. Its authorization
// server registers any client (RFC 7591) and asks for the scopes its metadata lists. whoami names the user.
func init() {
	Register(&Connector{ID: "sentry", Name: "Sentry", MCP: "https://mcp.sentry.dev/mcp", Account: mcpAccount("whoami")})
}
