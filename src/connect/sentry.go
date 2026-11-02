package connect

// Sentry, through its hosted MCP server: organizations, projects, issues, events and traces. Its authorization
// server registers any client (RFC 7591) and asks for the scopes its metadata lists. whoami names the user.
//
// Its organizations and projects page with a cursor, and are listed whole each sync. Its issues are not
// synced: search_issues answers in prose, has no cursor, stops at 100 and filters only by when an issue was
// last seen, not changed.
func init() {
	Register(&Connector{ID: "sentry", Name: "Sentry", Integration: true, MCP: "https://mcp.sentry.dev/mcp", Account: mcpAccount("whoami"),
		Streams: []*Stream{
			(&mcpList{tool: "find_organizations", after: "cursor", items: "organizations", id: "slug"}).stream("organizations",
				"Your organizations, { slug, webUrl, regionUrl }, by slug, from find_organizations, listed whole each sync."),
			{Name: "projects", Pull: sentryProjects,
				Doc: "The projects of each organization, by <organization>/<slug>, from find_projects, listed whole each sync."},
		}})
}

// sentryCursor is where the projects stream is: the organizations this round has left, the first under way
// at Cursor.
type sentryCursor struct {
	Orgs   []map[string]any `json:"orgs,omitempty"`
	Cursor string           `json:"cursor,omitempty"`
}

func sentryProjects(c *Client, cursor string) (*Page, error) {
	var cur sentryCursor
	if err := cursorOf(cursor, &cur); err != nil {
		return nil, err
	}
	if len(cur.Orgs) == 0 {
		for after := ""; ; {
			args := map[string]any{}
			if after != "" {
				args["cursor"] = after
			}
			orgs, next, err := sentryFind(c, "find_organizations", "organizations", args)
			if err != nil {
				return nil, err
			}
			for _, o := range orgs {
				cur.Orgs = append(cur.Orgs, map[string]any{"slug": o["slug"], "regionUrl": o["regionUrl"]})
			}
			if next == "" || next == after {
				break
			}
			after = next
		}
		if len(cur.Orgs) == 0 {
			return &Page{Full: true}, nil
		}
	}
	org := cur.Orgs[0]
	args := map[string]any{"organizationSlug": org["slug"]}
	if r, _ := org["regionUrl"].(string); r != "" {
		args["regionUrl"] = r
	}
	if cur.Cursor != "" {
		args["cursor"] = cur.Cursor
	}
	ps, next, err := sentryFind(c, "find_projects", "projects", args)
	if err != nil {
		return nil, err
	}
	page := &Page{Full: true}
	for _, p := range ps {
		p["organization"] = org["slug"]
		page.Records = append(page.Records, Record{ID: str(org, "slug") + "/" + str(p, "slug"), Data: p})
	}
	if next != "" && next != cur.Cursor {
		cur.Cursor = next
	} else {
		cur = sentryCursor{Orgs: cur.Orgs[1:]}
	}
	if page.More = len(cur.Orgs) > 0; page.More {
		page.Cursor = cursorFor(cur)
	}
	return page, nil
}

// sentryFind is a page of the find tool's items, and the cursor of the next.
func sentryFind(c *Client, tool, items string, args map[string]any) ([]map[string]any, string, error) {
	t, err := c.tool(tool)
	if err != nil {
		return nil, "", err
	}
	res, err := c.Call(t.Name, args)
	if err != nil {
		return nil, "", err
	}
	v, err := answer(c, tool, res)
	if err != nil {
		return nil, "", err
	}
	got := objects(v, items)
	return got, nextCursor(v, got, "slug"), nil
}
