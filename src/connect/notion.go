package connect

// Notion, through its hosted MCP server: search, read and write pages and databases as the person who
// connects, in the workspaces they allow. Its authorization server registers any client (RFC 7591), and
// replaces the refresh token on every refresh. notion-get-self names the user.
func init() {
	Register(&Connector{ID: "notion", Name: "Notion", MCP: "https://mcp.notion.com/mcp", Account: mcpAccount("notion-get-self")})
}
