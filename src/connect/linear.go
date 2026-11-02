package connect

// Linear, through its hosted MCP server: issues, projects, cycles, comments and documents. Its authorization
// server registers any client (RFC 7591). No tool is documented to name the user, so the account is Linear.
func init() {
	Register(&Connector{ID: "linear", Name: "Linear", MCP: "https://mcp.linear.app/mcp"})
}
