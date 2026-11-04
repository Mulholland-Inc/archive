package connect

// Notion, through its hosted MCP server: search, read and write pages and databases as the person who
// connects, in the workspaces they allow. Its authorization server registers any client (RFC 7591), and
// replaces the refresh token on every refresh. notion-get-self names the user.
//
// Its tools list no page by when it was edited: notion-search has no cursor and stops at 50, and sorts and
// filters by edit time only on Business and Enterprise plans. So the streams are its lists of the sidebar's
// pages, which page with a cursor but have no edit time either, listed whole each sync; a page is its
// title, URL and icon, not its content, which notion-fetch reads.
func init() {
	Register(&Connector{ID: "notion", Name: "Notion", MCP: "https://mcp.notion.com/mcp", Who: &Who{Tool: "notion-get-self"}, Busy: ToolBusy,
		Info: Info{By: "Notion", Description: "Search, read and write the pages and databases you allow", Category: CatDocs, Icon: IconNotebook, Scopes: []string{"Read and write pages you allow"}},
		Sync: []*Sync{
			{Name: "private_pages", Req: Req{Tool: "notion-list-private-pages"}, Items: "**", Key: notionKey, Page: notionPage, To: notionPages,
				Doc: "The pages of your Private section, by the id in their URL: { type, url, title, icon }, from notion-list-private-pages, 200 a page, listed whole each sync, so one gone is deleted; Notion says nothing of when a page was edited."},
			{Name: "shared_pages", Req: Req{Tool: "notion-list-shared-pages"}, Items: "**", Key: notionKey, Page: notionPage, To: notionPages,
				Doc: "The pages shared with you, by the id in their URL: { type, url, title, icon }, from notion-list-shared-pages, 200 a page, listed whole each sync, so one gone is deleted; Notion says nothing of when a page was edited."},
		}})
}

var (
	notionPage  = ToolPage{Limit: "limit", Max: 200, After: "cursor"}
	notionKey   = []Reads{{Of: "{id}"}, {Of: "{url}", Find: `([0-9a-f]{32})(?:[?#].*)?$`}, {Of: "{url}"}}
	notionPages = Map{Table: "doc_page",
		Cols: map[string]SQL{"kind": "coalesce(nullif(data->>'type', ''), 'page')", "title": "data->>'title'", "url": "data->>'url'",
			"icon":      "CASE jsonb_typeof(data->'icon') WHEN 'object' THEN coalesce(nullif(data#>>'{icon,emoji}', ''), data#>>'{icon,url}') ELSE data->>'icon' END",
			"edited_at": "archive.sync_at(data->>'last_edited_time')"}}
)
