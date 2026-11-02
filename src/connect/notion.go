package connect

import (
	"regexp"
)

// Notion, through its hosted MCP server: search, read and write pages and databases as the person who
// connects, in the workspaces they allow. Its authorization server registers any client (RFC 7591), and
// replaces the refresh token on every refresh. notion-get-self names the user.
//
// Its tools list no page by when it was edited: notion-search has no cursor and stops at 50, and sorts and
// filters by edit time only on Business and Enterprise plans. So the streams are its lists of the sidebar's
// pages, which page with a cursor but have no edit time either, listed whole each sync; a page is its
// title, URL and icon, not its content, which notion-fetch reads.
func init() {
	Register(&Connector{ID: "notion", Name: "Notion", MCP: "https://mcp.notion.com/mcp", Account: mcpAccount("notion-get-self"),
		Streams: []*Stream{
			notionPages("private_pages", "notion-list-private-pages", "The pages of your Private section"),
			notionPages("shared_pages", "notion-list-shared-pages", "The pages shared with you"),
		}})
}

func notionPages(name, tool, what string) *Stream {
	l := &mcpList{tool: tool, limit: "limit", max: 200, after: "cursor", key: notionID}
	return l.stream(name, what+", by the id in their URL: { type, url, title, icon }, from "+tool+", 200 a page, listed whole each sync, "+
		"so one gone is deleted; Notion says nothing of when a page was edited.")
}

var notionIDRe = regexp.MustCompile(`([0-9a-f]{32})(?:[?#].*)?$`)

// notionID is the id of a page in its URL, or the URL.
func notionID(it map[string]any) string {
	if id := str(it, "id"); id != "" {
		return id
	}
	u := str(it, "url")
	if m := notionIDRe.FindStringSubmatch(u); m != nil {
		return m[1]
	}
	return u
}
