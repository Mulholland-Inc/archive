package connect

import (
	"cmp"
	"strings"
)

// Gusto, through its hosted MCP server: the companies, their people and their payrolls, as an admin who
// connects it. Its authorization server publishes a registration endpoint (RFC 7591) and takes PKCE with S256
// and a client secret, so Archive registers its own client there like Notion's; Gusto's docs ask that only
// trusted clients connect, but name no approval. Archive asks for read scopes only, so its actions only read.
//
// One sign-in can reach several companies, and every tool but get_token_info acts on the one company_uuid
// names. get_token_info lists them, and names the account by them; each stream walks them in turn, listing each
// whole, with get_company or the company's list tool, paged by number while a page comes back full. None lists
// by when a record changed, so each sync lists everything again, and what it no longer lists is deleted.
func init() {
	Register(&Connector{ID: "gusto", Name: "Gusto", Integration: true, MCP: "https://mcp.api.gusto.com",
		Scopes: []string{"public", "companies:read", "employees:read", "contractors:read", "departments:read", "jobs:read", "compensations:read",
			"employments:read", "pay_schedules:read", "payrolls:read", "time_off_requests:read"},
		Account: gustoAccount,
		Streams: []*Stream{
			gustoEach("company", "Each company, as get_company answers: its name, entity type, EIN, contact and locations, by its uuid.", gustoCompany),
			gustoList("employees", "list_employees", "Every employee"),
			gustoList("contractors", "get_contractors", "Every contractor"),
			gustoList("departments", "get_departments", "Every department"),
			gustoList("locations", "get_locations", "Every location"),
			gustoList("pay_schedules", "get_pay_schedules", "Every pay schedule"),
			gustoList("payrolls", "list_payrolls", "The payrolls list_payrolls lists when asked for no dates"),
		}})
}

// gustoID is a Gusto record's uuid, or its id.
func gustoID(it map[string]any) string { return cmp.Or(str(it, "uuid"), str(it, "id")) }

// gustoCompanies are the companies the token reaches, as get_token_info lists them: each one's uuid and name.
func gustoCompanies(c *Client) ([]map[string]any, error) {
	t, err := c.tool("get_token_info")
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
	var out []map[string]any
	for _, r := range objects(v, "accessible_resources") {
		if strings.EqualFold(str(r, "resource_type"), "company") && str(r, "uuid") != "" {
			out = append(out, r)
		}
	}
	return out, nil
}

// gustoAccount names the account by its companies.
func gustoAccount(c *Client) (string, error) {
	cos, err := gustoCompanies(c)
	if err != nil {
		return "", err
	}
	var names []string
	for _, co := range cos {
		if n := strings.TrimSpace(str(co, "name")); n != "" {
			names = append(names, n)
		}
	}
	return strings.Join(names, ", "), nil
}

// gustoCursor is where a Gusto stream is: the companies it walks, as the token reached them when its listing
// began, the one it is at, and the cursor of that one's listing.
type gustoCursor struct {
	Companies []string `json:"companies"`
	At        int      `json:"at,omitempty"`
	Cursor    string   `json:"cursor,omitempty"`
}

// gustoEach is a stream that lists each company with pull, one listing of them all: what a company's pull
// lists, from its cursor, and whether it has more.
func gustoEach(name, doc string, pull func(c *Client, company, cursor string) (*Page, error)) *Stream {
	return &Stream{Name: name, Doc: doc, Pull: func(c *Client, cursor string) (*Page, error) {
		var st gustoCursor
		if err := cursorOf(cursor, &st); err != nil {
			return nil, err
		}
		if st.Companies == nil {
			cos, err := gustoCompanies(c)
			if err != nil {
				return nil, err
			}
			st.Companies = []string{}
			for _, co := range cos {
				st.Companies = append(st.Companies, str(co, "uuid"))
			}
		}
		if st.At >= len(st.Companies) {
			return &Page{Full: true}, nil
		}
		page, err := pull(c, st.Companies[st.At], st.Cursor)
		if err != nil {
			return nil, err
		}
		page.Full = true
		switch {
		case page.More:
			st.Cursor = page.Cursor
		case st.At+1 < len(st.Companies):
			st.At, st.Cursor, page.More = st.At+1, "", true
		default:
			st = gustoCursor{}
		}
		page.Cursor = ""
		if page.More {
			page.Cursor = cursorFor(st)
		}
		return page, nil
	}}
}

func gustoList(name, tool, what string) *Stream {
	return gustoEach(name, what+" of each company, as "+tool+" answers, by uuid, 100 a page, listed whole each sync, so one gone is deleted.",
		func(c *Client, company, cursor string) (*Page, error) {
			l := &mcpList{tool: tool, args: map[string]any{"company_uuid": company}, limit: "per,limit,page_size", max: 100, page: "page",
				after: "cursor,after", key: gustoID, id: "uuid", updated: "updated_at"}
			return l.pull(c, cursor)
		})
}

func gustoCompany(c *Client, company, cursor string) (*Page, error) {
	t, err := c.tool("get_company")
	if err != nil {
		return nil, err
	}
	res, err := c.Call(t.Name, map[string]any{"company_uuid": company})
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
	page := &Page{}
	if id := cmp.Or(gustoID(m), company); m != nil {
		page.Records = []Record{{ID: id, Data: m, Updated: stamp(str(m, "updated_at"))}}
	}
	return page, nil
}
