package connect

// Linear, through its hosted MCP server: issues, projects, cycles, comments and documents. Its authorization
// server registers any client (RFC 7591). No tool is documented to name the user, so the account is Linear.
//
// It syncs nothing until its issues and projects have tables in archive.sql; its actions are the server's tools.
func init() {
	Register(&Connector{ID: "linear", Name: "Linear", Integration: true, MCP: "https://mcp.linear.app/mcp",
		Info: Info{By: "Linear", Description: "Search and work with issues, projects and cycles", Category: CatProjects, Icon: IconList, Scopes: []string{"Read and write issues as you"}}})
}
