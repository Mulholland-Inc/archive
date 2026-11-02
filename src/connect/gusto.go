package connect

import "cmp"

// Gusto, through its hosted MCP server: the company, its people and its payrolls, as an admin who connects
// it. Its authorization server publishes a registration endpoint (RFC 7591) and takes PKCE with S256 and a
// client secret, so Archive registers its own client there like Notion's; Gusto's docs ask that only trusted
// clients connect, but name no approval. Archive asks for read scopes only, so its actions only read.
// get_company names the account, by the company's name.
//
// Its list tools have no input schema in Gusto's docs; each stream reads the one the server lists, and pages by
// number while a page comes back full. None lists by when a record changed, so each is listed whole on each
// sync, and what it no longer lists is deleted.
func init() {
	Register(&Connector{ID: "gusto", Name: "Gusto", MCP: "https://mcp.api.gusto.com",
		Scopes: []string{"public", "companies:read", "employees:read", "contractors:read", "departments:read", "jobs:read", "compensations:read",
			"employments:read", "pay_schedules:read", "payrolls:read", "time_off_requests:read"},
		Account: mcpName("get_company", "name", "trade_name", "legal_name"),
		Streams: []*Stream{
			{Name: "company", Pull: gustoCompany, Doc: "The company, as get_company answers: its name, entity type, EIN, contact and locations, by its uuid."},
			gustoList("employees", "list_employees", "Every employee"),
			gustoList("contractors", "list_contractors", "Every contractor"),
			gustoList("departments", "list_departments", "Every department"),
			gustoList("locations", "list_locations", "Every location"),
			gustoList("pay_schedules", "list_pay_schedules", "Every pay schedule"),
			gustoList("payrolls", "list_payrolls", "The payrolls list_payrolls lists when asked for no dates"),
		}})
}

// gustoID is a Gusto record's uuid, or its id.
func gustoID(it map[string]any) string { return cmp.Or(str(it, "uuid"), str(it, "id")) }

func gustoList(name, tool, what string) *Stream {
	l := &mcpList{tool: tool, limit: "per,limit,page_size", max: 100, page: "page", after: "cursor,after", key: gustoID, id: "uuid", updated: "updated_at"}
	return l.stream(name, what+", as "+tool+" answers, by uuid, 100 a page, listed whole each sync, so one gone is deleted.")
}

func gustoCompany(c *Client, cursor string) (*Page, error) {
	t, err := c.tool("get_company")
	if err != nil {
		return nil, err
	}
	res, err := c.Call(t.Name, map[string]any{})
	if err != nil {
		return nil, err
	}
	v, err := answer(c, t.Name, res)
	if err != nil {
		return nil, err
	}
	m, _ := v.(map[string]any)
	if inner, ok := m["company"].(map[string]any); ok {
		m = inner
	}
	page := &Page{Full: true}
	if id := gustoID(m); id != "" {
		page.Records = []Record{{ID: id, Data: m, Updated: stamp(str(m, "updated_at"))}}
	}
	return page, nil
}
