package connect

import (
	"cmp"
	"maps"
	"strconv"
	"strings"
	"time"
)

// Gusto, through its hosted MCP server: everything its read tools show of the companies an admin who connects
// it reaches, their people and their payrolls. Its authorization server publishes a registration endpoint (RFC
// 7591) and takes PKCE with S256 and a client secret, so Archive registers its own client there like Notion's;
// Gusto's docs ask that only trusted clients connect, but name no approval. Archive asks for read scopes only,
// so its actions only read.
//
// One sign-in can reach several companies, and every tool but get_token_info acts on the one company_uuid
// names, or on one employee. get_token_info lists the companies, and names the account by them. Each stream
// walks what it reads, one listing of them all: each company, each year since a year before the company joined
// Gusto (a payroll list takes at most a year), or each employee, the terminated too. None lists by when a
// record changed, so each sync lists everything again, and what it no longer lists is deleted. Left out are
// the tools that read one record by its uuid, which the lists already hold, the onboarding and payroll-blocker
// tools, time records, which only third-party time tracking keeps, and the earnings summary, which payrolls
// add up to.
func init() {
	Register(&Connector{ID: "gusto", Name: "Gusto", Integration: true, MCP: "https://mcp.api.gusto.com",
		Scopes: []string{"public", "companies:read", "employees:read", "contractors:read", "departments:read", "jobs:read", "compensations:read",
			"employments:read", "pay_schedules:read", "payrolls:read", "time_off_requests:read"},
		Account: gustoAccount,
		Streams: []*Stream{
			{Name: "company", Pull: gustoEach(gustoCompanyArgs, gustoCompany),
				Doc: "Each company, as get_company answers: its name, entity type, EIN, contact, locations and compensation types, by its uuid."},
			gustoList("employees", "list_employees", "Every employee, the terminated too, with every compensation, home address and custom field",
				gustoStatuses, map[string]any{"include": "all_compensations,all_home_addresses,custom_fields"}, nil),
			gustoList("contractors", "get_contractors", "Every contractor, the terminated too", gustoStatuses, nil, nil),
			gustoList("departments", "get_departments", "Every department, with who is in it", gustoCompanyArgs, nil, nil),
			gustoList("locations", "get_locations", "Every location", gustoCompanyArgs, nil, nil),
			gustoList("pay_schedules", "get_pay_schedules", "Every pay schedule", gustoCompanyArgs, nil, nil),
			{Name: "pay_schedule_assignments", Pull: gustoEach(gustoCompanyArgs, gustoOne("list_pay_schedule_assignments")),
				Doc: "How each company assigns people to pay schedules, as list_pay_schedule_assignments answers, by the company's uuid."},
			{Name: "earning_types", Pull: gustoEach(gustoCompanyArgs, gustoEarningTypes),
				Doc: "Every earning type of each company, default and custom, as list_earning_types answers, by uuid."},
			gustoList("custom_fields", "list_custom_fields_schema", "Every custom field each company defines", gustoCompanyArgs, nil, nil),
			gustoList("time_off_requests", "get_time_off_requests", "Every time-off request", gustoCompanyArgs, nil, nil),
			gustoList("time_off_balances", "get_time_off_balances", "Each employee's time-off balances, per policy", gustoCompanyArgs, nil,
				func(at, it map[string]any) string { return str(it, "employee_uuid") }),
			{Name: "payrolls", Gap: time.Second, Pull: gustoEach(gustoYears, gustoPayrolls),
				Doc: "Every payroll, processed or not, of every type, with its totals and taxes, and once processed what get_payroll adds: each person's " +
					"earnings, taxes, deductions and net pay. Listed a year at a time, 10 a page, by uuid."},
			gustoList("pay_periods", "list_pay_periods", "Every pay period and the payroll it is paid in, a year at a time", gustoYears, nil,
				func(at, it map[string]any) string { return str(it, "pay_schedule_uuid") + "/" + str(it, "start_date") }),
			gustoList("contractor_payments", "get_contractor_payments", "Every contractor payment, a year at a time", gustoYears, nil, nil),
			gustoList("contractor_payment_groups", "get_contractor_payment_groups", "Every batch of contractor payments, a year at a time", gustoYears, nil, nil),
			gustoList("employment_history", "list_employee_employment_history", "Each employee's employments, by the employee's uuid and hire date", gustoEmployees, nil,
				func(at, it map[string]any) string { return str(at, "employee_uuid") + "/" + str(it, "hire_date") }),
			gustoList("terminations", "list_employee_terminations", "Each employee's terminations", gustoEmployees, nil, nil),
			gustoList("work_addresses", "get_employee_work_addresses", "Each employee's work addresses, past ones too", gustoEmployees, nil, nil),
			gustoList("tasks", "get_tasks", "Each company's open to-dos in Gusto, like payroll blockers and documents to sign", gustoCompanyArgs, nil, nil),
		}})
}

// gustoID is a Gusto record's uuid, or its id.
func gustoID(it map[string]any) string { return cmp.Or(str(it, "uuid"), str(it, "id")) }

// gustoCall is tool's answer to args.
func gustoCall(c *Client, tool string, args map[string]any) (any, error) {
	t, err := c.tool(tool)
	if err != nil {
		return nil, err
	}
	res, err := c.Call(t.Name, args)
	if err != nil {
		return nil, err
	}
	return answer(c, t.Name, res)
}

// gustoCompanies are the companies the token reaches, as get_token_info lists them: each one's uuid and name.
func gustoCompanies(c *Client) ([]map[string]any, error) {
	v, err := gustoCall(c, "get_token_info", map[string]any{})
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

// gustoCompanyArgs are the arguments that name each company.
func gustoCompanyArgs(c *Client) ([]map[string]any, error) {
	cos, err := gustoCompanies(c)
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for _, co := range cos {
		out = append(out, map[string]any{"company_uuid": str(co, "uuid")})
	}
	return out, nil
}

// gustoStatuses are each company's arguments twice: as they are, and for those no longer active, whom a list
// leaves out unless asked for.
func gustoStatuses(c *Client) ([]map[string]any, error) {
	cos, err := gustoCompanyArgs(c)
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for _, co := range cos {
		out = append(out, co, map[string]any{"company_uuid": co["company_uuid"], "terminated": true})
	}
	return out, nil
}

// gustoYears are each company's years, as start_date and end_date: from a year before it joined Gusto, for
// history brought in with it, to 90 days ahead, as far as Gusto lists what is coming.
func gustoYears(c *Client) ([]map[string]any, error) {
	cos, err := gustoCompanyArgs(c)
	if err != nil {
		return nil, err
	}
	last := time.Now().UTC().AddDate(0, 0, 90).Truncate(24 * time.Hour)
	out := []map[string]any{}
	for _, co := range cos {
		v, err := gustoCall(c, "get_company", co)
		if err != nil {
			return nil, err
		}
		m, _ := v.(map[string]any)
		joined := stamp(str(m, "join_date"))
		if joined.IsZero() {
			joined, _ = time.Parse(time.DateOnly, clip(str(m, "join_date"), 10))
		}
		if joined.IsZero() {
			joined = last.AddDate(-10, 0, 0)
		}
		for start := joined.AddDate(-1, 0, 0).Truncate(24 * time.Hour); !start.After(last); start = start.AddDate(1, 0, 0) {
			end := start.AddDate(1, 0, -1)
			if end.After(last) {
				end = last
			}
			out = append(out, map[string]any{"company_uuid": co["company_uuid"], "start_date": start.Format(time.DateOnly), "end_date": end.Format(time.DateOnly)})
		}
	}
	return out, nil
}

// gustoEmployees are the arguments that name each employee of each company, the terminated too.
func gustoEmployees(c *Client) ([]map[string]any, error) {
	all, err := gustoStatuses(c)
	if err != nil {
		return nil, err
	}
	out, seen := []map[string]any{}, map[string]bool{}
	for _, args := range all {
		for page := 1; ; page++ {
			v, err := gustoCall(c, "list_employees", merged(args, map[string]any{"page": page, "per": 100}))
			if err != nil {
				return nil, err
			}
			es := objects(v, "")
			for _, e := range es {
				if id := str(e, "uuid"); id != "" && !seen[id] {
					seen[id] = true
					out = append(out, map[string]any{"employee_uuid": id})
				}
			}
			if len(es) < 100 {
				break
			}
		}
	}
	return out, nil
}

func merged(a, b map[string]any) map[string]any {
	out := maps.Clone(a)
	maps.Copy(out, b)
	return out
}

// gustoCursor is where a Gusto stream is: the arguments it reads with, one set at a time, as they were when its
// listing began, the set it is at, and the cursor of that set's listing.
type gustoCursor struct {
	Each   []map[string]any `json:"each"`
	At     int              `json:"at,omitempty"`
	Cursor string           `json:"cursor,omitempty"`
}

// gustoEach is a stream's Pull that lists with pull for each set of arguments each finds when its listing
// begins, one listing of them all.
func gustoEach(each func(c *Client) ([]map[string]any, error), pull func(c *Client, args map[string]any, cursor string) (*Page, error)) func(c *Client, cursor string) (*Page, error) {
	return func(c *Client, cursor string) (*Page, error) {
		var st gustoCursor
		if err := cursorOf(cursor, &st); err != nil {
			return nil, err
		}
		if st.Each == nil {
			all, err := each(c)
			if err != nil {
				return nil, err
			}
			st.Each = all
		}
		if st.At >= len(st.Each) {
			return &Page{Full: true}, nil
		}
		page, err := pull(c, st.Each[st.At], st.Cursor)
		if err != nil {
			return nil, err
		}
		page.Full = true
		switch {
		case page.More:
			st.Cursor = page.Cursor
		case st.At+1 < len(st.Each):
			st.At, st.Cursor, page.More = st.At+1, "", true
		}
		page.Cursor = ""
		if page.More {
			page.Cursor = cursorFor(st)
		}
		return page, nil
	}
}

// gustoList is a stream of tool's list for each set of arguments each finds, with opts too where the tool
// takes them, 100 a page; key is a record's id, its uuid when nil.
func gustoList(name, tool, what string, each func(c *Client) ([]map[string]any, error), opts map[string]any, key func(args, it map[string]any) string) *Stream {
	return &Stream{Name: name, Doc: what + ", as " + tool + " answers, 100 a page, listed whole each sync, so one gone is deleted.",
		Pull: gustoEach(each, func(c *Client, args map[string]any, cursor string) (*Page, error) {
			l := &mcpList{tool: tool, args: args, opts: opts, limit: "per,limit,page_size", max: 100, page: "page", after: "cursor,after",
				key: gustoID, id: "uuid", updated: "updated_at"}
			if key != nil {
				l.key = func(it map[string]any) string { return key(args, it) }
			}
			return l.pull(c, cursor)
		})}
}

func gustoCompany(c *Client, args map[string]any, cursor string) (*Page, error) {
	v, err := gustoCall(c, "get_company", args)
	if err != nil {
		return nil, err
	}
	m, _ := v.(map[string]any)
	if inner, ok := m["company"].(map[string]any); ok {
		m = inner
	}
	page := &Page{}
	if m != nil {
		page.Records = []Record{{ID: cmp.Or(gustoID(m), str(args, "company_uuid")), Data: m, Updated: stamp(str(m, "updated_at"))}}
	}
	return page, nil
}

// gustoOne is a pull of tool's answer for a company as one record, by the company's uuid.
func gustoOne(tool string) func(c *Client, args map[string]any, cursor string) (*Page, error) {
	return func(c *Client, args map[string]any, cursor string) (*Page, error) {
		v, err := gustoCall(c, tool, args)
		if err != nil {
			return nil, err
		}
		return &Page{Records: []Record{{ID: str(args, "company_uuid"), Data: v}}}, nil
	}
}

// gustoEarningTypes are a company's earning types, the default ones and its own, which list_earning_types
// answers in two lists.
func gustoEarningTypes(c *Client, args map[string]any, cursor string) (*Page, error) {
	v, err := gustoCall(c, "list_earning_types", args)
	if err != nil {
		return nil, err
	}
	page := &Page{}
	for _, kind := range []string{"default", "custom"} {
		for _, it := range objects(v, kind) {
			if id := gustoID(it); id != "" {
				page.Records = append(page.Records, Record{ID: id, Data: merged(it, map[string]any{"kind": kind})})
			}
		}
	}
	return page, nil
}

// gustoPayrolls are a page of a company's payrolls in a year, 10 at a time, each processed one with what
// get_payroll adds: every page of its employee compensations.
func gustoPayrolls(c *Client, args map[string]any, cursor string) (*Page, error) {
	const per = 10
	page := max(1, atoi(cursor))
	v, err := gustoCall(c, "list_payrolls", merged(args, map[string]any{"include": "taxes,totals", "processing_statuses": "processed,unprocessed",
		"payroll_types": "regular,off_cycle,external", "page": page, "per": per}))
	if err != nil {
		return nil, err
	}
	items := objects(v, "")
	out := &Page{}
	for _, it := range items {
		id := cmp.Or(str(it, "payroll_uuid"), gustoID(it))
		if id == "" {
			continue
		}
		data := it
		if it["processed"] == true {
			full, err := gustoPayroll(c, str(args, "company_uuid"), id)
			if err != nil {
				return nil, err
			}
			data = merged(it, full)
		}
		out.Records = append(out.Records, Record{ID: id, Data: data})
	}
	if len(items) >= per {
		out.Cursor, out.More = strconv.Itoa(page+1), true
	}
	return out, nil
}

// gustoPayroll is payroll as get_payroll answers, with its employee compensations from every page.
func gustoPayroll(c *Client, company, payroll string) (map[string]any, error) {
	var out map[string]any
	var comps []any
	for page := 1; page <= 100; page++ {
		v, err := gustoCall(c, "get_payroll", map[string]any{"company_uuid": company, "payroll_uuid": payroll,
			"employee_compensations_page": page, "employee_compensations_per": 100})
		if err != nil {
			return nil, err
		}
		m, _ := v.(map[string]any)
		if out == nil {
			out = m
		}
		got, _ := m["employee_compensations"].([]any)
		comps = append(comps, got...)
		more, _ := m["employee_compensations_pagination"].(map[string]any)
		if more["has_more"] != true || len(got) == 0 {
			break
		}
	}
	if out != nil {
		out["employee_compensations"] = comps
		delete(out, "employee_compensations_pagination")
	}
	return out, nil
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
