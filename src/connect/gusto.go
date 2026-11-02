package connect

import (
	"cmp"
	"errors"
	"maps"
	"slices"
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
// names, or on one employee. get_token_info lists the companies, and names the account by them; each stream
// keeps them a day. Each stream walks what it reads, one listing of them all: each company, or each year of a
// company's history (a payroll list takes at most a year).
//
// None of its tools lists by when a record changed, and nothing it pushes reaches Archive, so a sync lists
// again, and each stream's memory (its cursor) keeps that to what can have changed:
//   - Setup, like departments and pay schedules, and what changes with payrolls alone, time-off balances and
//     pay periods, is listed a day apart (Every); setup whole, so one gone is deleted.
//   - People are listed every sync, whole, but an employee whose version is the one read before is the Same:
//     their employment history and work addresses, which take a call each, are read only for one new or
//     changed. Their terminations come with them.
//   - Payrolls, pay periods and contractor payments are listed from a year before the company joined Gusto
//     once, on the first sync, and after that only from three months back to 90 days ahead, as far as Gusto
//     lists what is coming. Those pages never delete. A processed payroll does not change, as Gusto corrects
//     one with another, off-cycle, so get_payroll reads each once, when it is first seen processed.
//
// Left out are the tools that read one record by its uuid, which the lists already hold, the onboarding and
// payroll-blocker tools, time records, which only third-party time tracking keeps, and the earnings summary,
// which payrolls add up to.
func init() {
	Register(&Connector{ID: "gusto", Name: "Gusto", Integration: true, MCP: "https://mcp.api.gusto.com", Pace: 300 * time.Millisecond,
		Scopes: []string{"public", "companies:read", "employees:read", "contractors:read", "departments:read", "jobs:read", "compensations:read",
			"employments:read", "pay_schedules:read", "payrolls:read", "time_off_requests:read"},
		Account: gustoAccount,
		Streams: []*Stream{
			{Name: "company", Every: gustoDaily, Pull: (&gustoStream{each: gustoCompanyArgs, pull: gustoCompany}).Pull, Table: "payroll_company", Rows: gustoCompanyRows,
				Doc: "Each company, as get_company answers: its name, entity type, EIN, status and admin, by its uuid; read a day apart."},
			{Name: "employees", Pull: (&gustoStream{each: gustoStatuses, pull: gustoEmployees}).Pull, Table: "payroll_worker", Rows: gustoEmployeeRows,
				Doc: "Every employee, the terminated too, as list_employees answers with their jobs, every compensation, home address and custom field, and " +
					"with their employment history and work addresses, which list_employee_employment_history and get_employee_work_addresses answer, " +
					"read again only when the employee's version changes; 100 a page, listed whole each sync, so one gone is deleted."},
			gustoList("contractors", "get_contractors", "Every contractor, the terminated too", gustoStatuses, 0, nil, "payroll_worker", gustoContractorRows),
			gustoList("departments", "get_departments", "Every department, with who is in it", gustoCompanyArgs, gustoDaily, nil, "payroll_department", gustoDepartmentRows),
			gustoList("locations", "get_locations", "Every location", gustoCompanyArgs, gustoDaily, nil, "payroll_location", gustoLocationRows),
			gustoList("pay_schedules", "get_pay_schedules", "Every pay schedule", gustoCompanyArgs, gustoDaily, nil, "payroll_schedule", gustoScheduleRows),
			{Name: "pay_schedule_assignments", Every: gustoDaily, Pull: (&gustoStream{each: gustoCompanyArgs, pull: gustoOne("list_pay_schedule_assignments")}).Pull,
				Table: "payroll_schedule_assignment", Rows: gustoAssignmentRows,
				Doc: "How each company assigns people to pay schedules, as list_pay_schedule_assignments answers, by the company's uuid; read a day apart."},
			{Name: "earning_types", Every: gustoDaily, Pull: (&gustoStream{each: gustoCompanyArgs, pull: gustoEarningTypes}).Pull,
				Table: "payroll_earning_type", Rows: gustoEarningTypeRows,
				Doc: "Every earning type of each company, default and custom, as list_earning_types answers, by uuid; read a day apart."},
			gustoList("custom_fields", "list_custom_fields_schema", "Every custom field each company defines", gustoCompanyArgs, gustoDaily, nil,
				"payroll_custom_field", gustoCustomFieldRows),
			gustoList("time_off_requests", "get_time_off_requests", "Every time-off request", gustoCompanyArgs, 0, nil, "payroll_time_off_request", gustoTimeOffRows),
			gustoList("time_off_balances", "get_time_off_balances", "Each employee's time-off balances, per policy", gustoCompanyArgs, gustoDaily,
				func(at, it map[string]any) string { return str(it, "employee_uuid") }, "payroll_time_off_balance", gustoBalanceRows),
			{Name: "payrolls", Gap: time.Second, Pull: (&gustoStream{each: gustoYears, pull: gustoPayrolls, recent: true}).Pull, Table: "payroll_run", Rows: gustoPayrollRows,
				Doc: "Every payroll, processed or not, of every type, with its totals and taxes, and once processed what get_payroll adds, read once: each " +
					"person's earnings, taxes, deductions, benefits and net pay. The first sync lists every year, 25 a page; the next ones from three " +
					"months back to 90 days ahead, by uuid, and delete nothing."},
			gustoYearList("pay_periods", "list_pay_periods", "Every pay period and the payroll it is paid in", gustoDaily,
				func(at, it map[string]any) string { return str(it, "pay_schedule_uuid") + "/" + str(it, "start_date") }, "payroll_pay_period", gustoPayPeriodRows),
			gustoYearList("contractor_payments", "get_contractor_payments", "Every contractor payment", 0, nil, "payroll_contractor_payment", gustoContractorPaymentRows),
			gustoYearList("contractor_payment_groups", "get_contractor_payment_groups", "Every batch of contractor payments", 0, nil,
				"payroll_contractor_payment_group", gustoPaymentGroupRows),
			gustoList("tasks", "get_tasks", "Each company's open to-dos in Gusto, like payroll blockers and documents to sign", gustoCompanyArgs, 0, nil,
				"payroll_task", gustoTaskRows),
		}})
}

// gustoDaily is how far apart setup, which seldom changes, is read.
const gustoDaily = 24 * time.Hour

// gustoRecent is how far back the payroll lists of a sync after the first reach.
const gustoRecent = 3 // months

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

// gustoCursor is where a Gusto stream is. A listing under way has the arguments it reads with, one set at a
// time, as they were when it began, the set it is at, and the cursor of that set's listing. The rest is the
// stream's memory, kept from one sync to the next: the companies the token reached when it last asked, for a
// day; that the stream has listed every year of its history; and what it has read in full, Seen, by id: an
// employee's version, or a payroll whose detail it read. Next is what a listing under way has seen, which
// becomes Seen when it ends, so one no longer listed is forgotten.
type gustoCursor struct {
	Each        []map[string]any  `json:"each,omitempty"`
	At          int               `json:"at,omitempty"`
	Cursor      string            `json:"cursor,omitempty"`
	Companies   []string          `json:"companies,omitempty"`
	CompaniesAt *time.Time        `json:"companies_at,omitempty"`
	History     bool              `json:"history,omitempty"`
	Seen        map[string]string `json:"seen,omitempty"`
	Next        map[string]string `json:"next,omitempty"`
}

// gustoStream is a stream that lists with pull for each set of arguments each finds when its listing begins,
// one listing of them all: whole, so one no longer listed is deleted, unless recent, whose pages delete
// nothing.
type gustoStream struct {
	each   func(c *Client, st *gustoCursor) ([]map[string]any, error)
	pull   func(c *Client, st *gustoCursor, args map[string]any, cursor string) (*Page, error)
	recent bool
}

func (g *gustoStream) Pull(c *Client, cursor string) (*Page, error) {
	var st gustoCursor
	if err := cursorOf(cursor, &st); err != nil {
		return nil, err
	}
	if st.Each == nil {
		all, err := g.each(c, &st)
		if err != nil {
			return nil, err
		}
		st.Each = append([]map[string]any{}, all...)
	}
	page := &Page{}
	if st.At < len(st.Each) {
		var err error
		if page, err = g.pull(c, &st, st.Each[st.At], st.Cursor); err != nil {
			return nil, err
		}
	}
	page.Full = !g.recent
	switch {
	case page.More:
		st.Cursor = page.Cursor
	case st.At+1 < len(st.Each):
		st.At, st.Cursor, page.More = st.At+1, "", true
	default:
		// The listing is over, and only the stream's memory goes on to the next.
		st.Each, st.At, st.Cursor = nil, 0, ""
		st.History = st.History || g.recent
		if st.Next != nil {
			st.Seen, st.Next = st.Next, nil
		}
	}
	page.Cursor = cursorFor(st)
	return page, nil
}

// gustoCos are the uuids of the companies the token reaches, as the stream last asked within a day.
func gustoCos(c *Client, st *gustoCursor) ([]string, error) {
	if st.CompaniesAt != nil && time.Since(*st.CompaniesAt) < gustoDaily {
		return st.Companies, nil
	}
	cos, err := gustoCompanies(c)
	if err != nil {
		return nil, err
	}
	st.Companies = []string{}
	for _, co := range cos {
		st.Companies = append(st.Companies, str(co, "uuid"))
	}
	st.CompaniesAt = ptrTo(time.Now().UTC())
	return st.Companies, nil
}

func ptrTo[T any](v T) *T { return &v }

// gustoCompanyArgs are the arguments that name each company.
func gustoCompanyArgs(c *Client, st *gustoCursor) ([]map[string]any, error) {
	cos, err := gustoCos(c, st)
	out := []map[string]any{}
	for _, co := range cos {
		out = append(out, map[string]any{"company_uuid": co})
	}
	return out, err
}

// gustoStatuses are each company's arguments twice: as they are, and for those no longer active, whom a list
// leaves out unless asked for.
func gustoStatuses(c *Client, st *gustoCursor) ([]map[string]any, error) {
	cos, err := gustoCompanyArgs(c, st)
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for _, co := range cos {
		out = append(out, co, map[string]any{"company_uuid": co["company_uuid"], "terminated": true})
	}
	return out, nil
}

// gustoYears are each company's years to list, as start_date and end_date, to 90 days ahead, as far as Gusto
// lists what is coming: from a year before it joined Gusto, for history brought in with it, until the stream
// has listed them once, and after that the recent months alone.
func gustoYears(c *Client, st *gustoCursor) ([]map[string]any, error) {
	cos, err := gustoCompanyArgs(c, st)
	if err != nil {
		return nil, err
	}
	last := time.Now().UTC().AddDate(0, 0, 90).Truncate(24 * time.Hour)
	out := []map[string]any{}
	for _, co := range cos {
		first := last.AddDate(0, -gustoRecent, -90)
		if !st.History {
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
			first = joined.AddDate(-1, 0, 0).Truncate(24 * time.Hour)
		}
		for start := first; !start.After(last); start = start.AddDate(1, 0, 0) {
			end := start.AddDate(1, 0, -1)
			if end.After(last) {
				end = last
			}
			out = append(out, map[string]any{"company_uuid": co["company_uuid"], "start_date": start.Format(time.DateOnly), "end_date": end.Format(time.DateOnly)})
		}
	}
	return out, nil
}

func merged(a, b map[string]any) map[string]any {
	out := maps.Clone(a)
	maps.Copy(out, b)
	return out
}

// gustoListPull is a page of tool's list for args, with opts too where the tool takes them, 100 a page; key
// is a record's id, its uuid when nil.
func gustoListPull(c *Client, tool string, per int, args, opts map[string]any, key func(args, it map[string]any) string, cursor string) (*Page, error) {
	l := &mcpList{tool: tool, args: args, opts: opts, limit: "per,limit,page_size", max: per, page: "page", after: "cursor,after",
		key: gustoID, id: "uuid", updated: "updated_at"}
	if key != nil {
		l.key = func(it map[string]any) string { return key(args, it) }
	}
	page, err := l.pull(c, cursor)
	if err != nil {
		return nil, err
	}
	// A record says the company it is of, as a list of a company's may leave out.
	for _, r := range page.Records {
		if m, ok := r.Data.(map[string]any); ok && str(m, "company_uuid") == "" && args["company_uuid"] != nil {
			m["company_uuid"] = args["company_uuid"]
		}
	}
	return page, nil
}

// gustoList is a stream of tool's list for each set of arguments each finds, listed whole every so often, so
// one gone is deleted; key is a record's id, as gustoListPull has it, and rows its records' rows.
func gustoList(name, tool, what string, each func(c *Client, st *gustoCursor) ([]map[string]any, error), every time.Duration,
	key func(args, it map[string]any) string, table string, rows func(map[string]any) *Row) *Stream {
	g := &gustoStream{each: each, pull: func(c *Client, st *gustoCursor, args map[string]any, cursor string) (*Page, error) {
		return gustoListPull(c, tool, 100, args, nil, key, cursor)
	}}
	doc := what + ", as " + tool + " answers, 100 a page, listed whole each sync, so one gone is deleted."
	if every > 0 {
		doc = what + ", as " + tool + " answers, 100 a page, listed whole a day apart, so one gone is deleted."
	}
	return &Stream{Name: name, Doc: doc, Every: every, Pull: g.Pull, Table: table, Rows: rows}
}

// gustoYearList is a stream of tool's list for each company's years (gustoYears): every year once, and the
// recent months after, in pages that delete nothing.
func gustoYearList(name, tool, what string, every time.Duration, key func(args, it map[string]any) string, table string, rows func(map[string]any) *Row) *Stream {
	g := &gustoStream{each: gustoYears, recent: true, pull: func(c *Client, st *gustoCursor, args map[string]any, cursor string) (*Page, error) {
		return gustoListPull(c, tool, 100, args, nil, key, cursor)
	}}
	doc := what + ", as " + tool + " answers, 100 a page: every year on the first sync, and from three months back to 90 days ahead after, which deletes nothing."
	if every > 0 {
		doc = strings.Replace(doc, "ahead after", "ahead a day apart after", 1)
	}
	return &Stream{Name: name, Every: every, Pull: g.Pull, Table: table, Rows: rows, Doc: doc}
}

func gustoCompany(c *Client, st *gustoCursor, args map[string]any, cursor string) (*Page, error) {
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

// gustoEmployees is a page of a company's employees. One whose version is the one read before is the Same;
// one new or changed is read with their employment history and work addresses, where the server has the tools.
func gustoEmployees(c *Client, st *gustoCursor, args map[string]any, cursor string) (*Page, error) {
	// 20 a page: each new or changed one costs two calls more, which at Gusto's pace must fit in a page's time.
	page, err := gustoListPull(c, "list_employees", 20, args, map[string]any{"include": "all_compensations,all_home_addresses,custom_fields"}, nil, cursor)
	if err != nil {
		return nil, err
	}
	if st.Next == nil {
		st.Next = map[string]string{}
	}
	for i, r := range page.Records {
		e, _ := r.Data.(map[string]any)
		version := str(e, "version")
		st.Next[r.ID] = version
		if version != "" && st.Seen[r.ID] == version {
			page.Records[i] = Record{ID: r.ID, Same: true}
			continue
		}
		for key, tool := range map[string]string{"employment_history": "list_employee_employment_history", "work_addresses": "get_employee_work_addresses"} {
			v, err := gustoCall(c, tool, map[string]any{"employee_uuid": r.ID})
			var unsup *Unsupported
			switch {
			case errors.As(err, &unsup):
			case err != nil:
				return nil, err
			default:
				e[key] = objects(v, "")
			}
		}
	}
	return page, nil
}

// gustoOne is a pull of tool's answer for a company as one record, by the company's uuid.
func gustoOne(tool string) func(c *Client, st *gustoCursor, args map[string]any, cursor string) (*Page, error) {
	return func(c *Client, st *gustoCursor, args map[string]any, cursor string) (*Page, error) {
		v, err := gustoCall(c, tool, args)
		if err != nil {
			return nil, err
		}
		return &Page{Records: []Record{{ID: str(args, "company_uuid"), Data: v}}}, nil
	}
}

// gustoEarningTypes are a company's earning types, the default ones and its own, which list_earning_types
// answers in two lists.
func gustoEarningTypes(c *Client, st *gustoCursor, args map[string]any, cursor string) (*Page, error) {
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

// gustoPayrolls are a page of a company's payrolls in a year, 25 at a time. A processed one is read with what
// get_payroll adds, every page of its employee compensations, once: after that it is the Same.
func gustoPayrolls(c *Client, st *gustoCursor, args map[string]any, cursor string) (*Page, error) {
	const per = 25
	page := max(1, atoi(cursor))
	v, err := gustoCall(c, "list_payrolls", merged(args, map[string]any{"include": "taxes,totals", "processing_statuses": "processed,unprocessed",
		"payroll_types": "regular,off_cycle,external", "page": page, "per": per}))
	if err != nil {
		return nil, err
	}
	if st.Seen == nil {
		st.Seen = map[string]string{}
	}
	items := objects(v, "")
	out := &Page{}
	for _, it := range items {
		id := cmp.Or(str(it, "payroll_uuid"), gustoID(it))
		switch {
		case id == "":
			continue
		case it["processed"] != true:
			out.Records = append(out.Records, Record{ID: id, Data: it})
			continue
		}
		if _, ok := st.Seen[id]; ok {
			out.Records = append(out.Records, Record{ID: id, Same: true})
			continue
		}
		full, err := gustoPayroll(c, str(args, "company_uuid"), id)
		if err != nil {
			return nil, err
		}
		st.Seen[id] = ""
		out.Records = append(out.Records, Record{ID: id, Data: merged(it, full)})
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

// Gusto pays in US dollars.
const gustoCurrency = "USD"

func gustoMoney(v any) Money { return money(v, gustoCurrency) }

// gustoRow is a row of a company's: of the company its company_uuid names.
func gustoRow(m map[string]any, cols map[string]any) *Row {
	return rowOf(cols).ref("company", str(m, "company_uuid"))
}

// gustoName is the full name of a person or business Gusto has.
func gustoName(m map[string]any) string {
	return cmp.Or(strings.TrimSpace(str(m, "full_name")), strings.TrimSpace(str(m, "first_name")+" "+str(m, "last_name")), str(m, "business_name"))
}

// gustoAddress is an address as a row of payroll_address, of kind home or work.
func gustoAddress(kind string, a map[string]any) *Row {
	return rowOf(map[string]any{"kind": kind, "street_1": str(a, "street_1"), "street_2": str(a, "street_2"), "city": str(a, "city"),
		"state": str(a, "state"), "zip": str(a, "zip"), "country": str(a, "country"), "active": flag(a, "active"),
		"effective_on": day(str(a, "effective_date"))})
}

// gustoCompanyRows is a company with the organization its EIN is, and its payroll admin and signatory, who are
// people by their address.
func gustoCompanyRows(m map[string]any) *Row {
	admin, signer := obj(m, "primary_payroll_admin"), obj(m, "primary_signatory")
	r := rowOf(map[string]any{"name": str(m, "name"), "trade_name": str(m, "trade_name"), "entity_type": str(m, "entity_type"),
		"status": str(m, "company_status"), "tier": str(m, "tier"), "suspended": flag(m, "is_suspended"), "joined_on": day(str(m, "join_date")),
		"admin_name": gustoName(admin), "signatory_name": gustoName(signer)}).person("admin", str(admin, "email")).person("signatory", str(signer, "email"))
	if ein := EIN(str(m, "ein")); ein != "" {
		r.Orgs["org"] = ein
	}
	return r
}

// gustoEmployeeRows is an employee as a worker: their primary job, or else their first, says their title, start
// and pay, and their last termination when they left. Their jobs, compensations, addresses, employment history
// and custom fields are rows of their own.
func gustoEmployeeRows(m map[string]any) *Row {
	r := gustoRow(m, map[string]any{"kind": "employee", "name": gustoName(m), "first_name": str(m, "first_name"), "middle_initial": str(m, "middle_initial"),
		"last_name": str(m, "last_name"), "preferred_name": str(m, "preferred_first_name"), "email": Email(cmp.Or(str(m, "work_email"), str(m, "email"))),
		"personal_email": Email(str(m, "email")), "phone": str(m, "phone"), "birthday": day(str(m, "date_of_birth")), "department_name": str(m, "department"),
		"active": m["terminated"] != true, "onboarded": flag(m, "onboarded"), "onboarding_status": str(m, "onboarding_status"),
		"payment_method": str(m, "payment_method")})
	r.person("person", cmp.Or(str(m, "work_email"), str(m, "email"))).ref("department", str(m, "department_uuid")).ref("manager", str(m, "manager_uuid"))
	jobs := objects(m["jobs"], "")
	for i, j := range jobs {
		if i == 0 || j["primary"] == true {
			r.Cols["title"], r.Cols["hired_on"], r.Cols["pay_rate"], r.Cols["pay_per"] = str(j, "title"), day(str(j, "hire_date")), gustoMoney(j["rate"]), str(j, "payment_unit")
		}
		r.child("payroll_job", rowOf(map[string]any{"source_id": gustoID(j), "title": str(j, "title"), "rate": gustoMoney(j["rate"]),
			"pay_per": str(j, "payment_unit"), "hired_on": day(str(j, "hire_date")), "primary_job": j["primary"] == true}).ref("location", str(j, "location_uuid")))
		for _, c := range objects(j["compensations"], "") {
			r.child("payroll_compensation", rowOf(map[string]any{"job_source": gustoID(j), "rate": gustoMoney(c["rate"]), "pay_per": str(c, "payment_unit"),
				"flsa_status": str(c, "flsa_status"), "effective_on": day(str(c, "effective_date"))}))
			if i == 0 || j["primary"] == true {
				r.Cols["flsa_status"] = str(c, "flsa_status")
			}
		}
		if j["primary"] == true {
			break
		}
	}
	for _, t := range objects(m["terminations"], "") {
		if d, _ := day(str(t, "effective_date")).(string); d != "" && (r.Cols["terminated_on"] == nil || d > r.Cols["terminated_on"].(string)) {
			r.Cols["terminated_on"] = d
		}
	}
	for _, a := range objects(m["home_addresses"], "") {
		r.child("payroll_address", gustoAddress("home", a))
	}
	for _, a := range objects(m["work_addresses"], "") {
		r.child("payroll_address", gustoAddress("work", a).ref("location", str(a, "location_uuid")))
	}
	for _, e := range objects(m["employment_history"], "") {
		r.child("payroll_employment", rowOf(map[string]any{"hired_on": day(str(e, "hire_date")), "terminated_on": day(str(e, "termination_date")),
			"status": str(e, "employment_status")}))
	}
	for _, f := range objects(m["custom_fields"], "") {
		r.child("payroll_custom_value", rowOf(map[string]any{"name": str(f, "name"), "value": str(f, "value")}).ref("field", str(f, "company_custom_field_id")))
	}
	return r
}

func gustoContractorRows(m map[string]any) *Row {
	r := gustoRow(m, map[string]any{"kind": "contractor", "name": gustoName(m), "first_name": str(m, "first_name"), "last_name": str(m, "last_name"),
		"business_name": str(m, "business_name"), "contractor_type": str(m, "type"), "wage_type": str(m, "wage_type"), "email": Email(str(m, "email")),
		"active": m["is_active"] != false, "hired_on": day(str(m, "start_date")), "payment_method": str(m, "payment_method")})
	r.person("person", str(m, "email")).ref("department", str(m, "department_uuid"))
	if strings.EqualFold(str(m, "wage_type"), "Hourly") {
		r.Cols["pay_rate"], r.Cols["pay_per"] = gustoMoney(m["hourly_rate"]), "Hour"
	}
	if a, ok := m["address"].(map[string]any); ok {
		r.child("payroll_address", gustoAddress("home", a))
	}
	return r
}

// gustoDepartmentRows is a department with its members, employees and contractors.
func gustoDepartmentRows(m map[string]any) *Row {
	r := gustoRow(m, map[string]any{"title": str(m, "title")})
	for _, kind := range []string{"employees", "contractors"} {
		for _, w := range objects(m[kind], "") {
			r.child("payroll_department_member", rowOf(map[string]any{}).ref("worker", gustoID(w)))
		}
	}
	return r
}

func gustoLocationRows(m map[string]any) *Row {
	return gustoRow(m, map[string]any{"street_1": str(m, "street_1"), "street_2": str(m, "street_2"), "city": str(m, "city"), "state": str(m, "state"),
		"zip": str(m, "zip"), "country": str(m, "country"), "phone": str(m, "phone_number"), "active": flag(m, "active"),
		"mailing_address": flag(m, "mailing_address"), "filing_address": flag(m, "filing_address")})
}

func gustoScheduleRows(m map[string]any) *Row {
	return gustoRow(m, map[string]any{"name": cmp.Or(str(m, "custom_name"), str(m, "name")), "frequency": str(m, "frequency"),
		"anchor_pay_date": day(str(m, "anchor_pay_date")), "anchor_period_end": day(str(m, "anchor_end_of_pay_period")),
		"day_1": decimal(m["day_1"]), "day_2": decimal(m["day_2"]), "auto_pilot": flag(m, "auto_pilot"), "active": flag(m, "active")})
}

// gustoAssignmentRows is how a company assigns pay schedules, with the schedule of each employee or department
// it names.
func gustoAssignmentRows(m map[string]any) *Row {
	r := gustoRow(m, map[string]any{"type": str(m, "type")}).ref("default_schedule", str(m, "default_pay_schedule_uuid")).
		ref("hourly_schedule", str(m, "hourly_pay_schedule_uuid")).ref("salaried_schedule", str(m, "salaried_pay_schedule_uuid"))
	for _, e := range objects(m["employees"], "") {
		r.child("payroll_schedule_member", rowOf(map[string]any{}).ref("worker", str(e, "employee_uuid")).ref("schedule", str(e, "pay_schedule_uuid")))
	}
	for _, d := range objects(m["departments"], "") {
		r.child("payroll_schedule_member", rowOf(map[string]any{}).ref("department", str(d, "department_uuid")).ref("schedule", str(d, "pay_schedule_uuid")))
	}
	return r
}

func gustoEarningTypeRows(m map[string]any) *Row {
	return gustoRow(m, map[string]any{"name": str(m, "name"), "kind": str(m, "kind")})
}

func gustoCustomFieldRows(m map[string]any) *Row {
	return gustoRow(m, map[string]any{"name": str(m, "name"), "type": str(m, "type"), "description": str(m, "description"),
		"options": strs(m["selection_options"])})
}

// gustoTimeOffRows is a time-off request: its days, each with its hours, add up to its hours, from the first
// to the last.
func gustoTimeOffRows(m map[string]any) *Row {
	emp, approver := obj(m, "employee"), obj(m, "approver")
	r := gustoRow(m, map[string]any{"policy": str(m, "request_type"), "status": str(m, "status"), "employee_note": str(m, "employee_note"),
		"employer_note": str(m, "employer_note"), "employee_name": gustoName(emp), "approver_name": gustoName(approver)}).
		ref("worker", gustoID(emp)).ref("approver", gustoID(approver))
	days := obj(m, "days")
	var first, last string
	hours := 0.0
	for _, d := range slices.Sorted(maps.Keys(days)) {
		on, _ := day(d).(string)
		if on == "" {
			continue
		}
		first, last = cmp.Or(first, on), on
		h, _ := strconv.ParseFloat(num(days[d]), 64)
		hours += h
		r.child("payroll_time_off_day", rowOf(map[string]any{"on_date": on, "hours": decimal(days[d])}))
	}
	r.Cols["starts_on"], r.Cols["ends_on"], r.Cols["hours"] = day(cmp.Or(first, str(m, "start_date"))), day(cmp.Or(last, str(m, "end_date"))), hours
	return r
}

// gustoBalanceRows is an employee's time-off balances, one a policy: what is left, accrued, used and pending.
func gustoBalanceRows(m map[string]any) *Row {
	r := gustoRow(m, map[string]any{}).ref("worker", str(m, "employee_uuid"))
	for _, p := range objects(m["balances"], "") {
		r.child("payroll_time_off_policy", rowOf(map[string]any{"name": str(p, "policy_name"), "policy_type": str(p, "policy_type"), "policy_source": str(p, "policy_uuid"),
			"hours": decimal(p["balance_hours"]), "accrued_hours": decimal(p["accrued_hours"]), "used_hours": decimal(p["used_hours"]),
			"pending_hours": decimal(p["pending_hours"])}))
	}
	return r
}

// gustoPayrollRows is a payroll with what each employee was paid in it, and every line of that: their earnings,
// deductions, benefits and taxes. A tax is the employee's unless Gusto says it is the employer's.
func gustoPayrollRows(m map[string]any) *Row {
	totals, period := obj(m, "totals"), obj(m, "pay_period")
	r := gustoRow(m, map[string]any{"period_start": day(str(period, "start_date")), "period_end": day(str(period, "end_date")),
		"check_date": day(str(m, "check_date")), "deadline": instant(str(m, "payroll_deadline")), "processed": m["processed"] == true,
		"processed_at": instant(str(m, "processed_date")), "off_cycle": m["off_cycle"] == true, "off_cycle_reason": str(m, "off_cycle_reason"),
		"external": m["external"] == true, "gross_pay": gustoMoney(totals["gross_pay"]), "net_pay": gustoMoney(totals["net_pay"]),
		"employee_taxes": gustoMoney(totals["employee_taxes"]), "employer_taxes": gustoMoney(totals["employer_taxes"]),
		"benefits": gustoMoney(totals["benefits"]), "deductions": gustoMoney(totals["deductions"]), "reimbursements": gustoMoney(totals["reimbursements"]),
		"check_amount": gustoMoney(totals["check_amount"]), "company_debit": gustoMoney(totals["company_debit"])}).
		ref("schedule", str(period, "pay_schedule_uuid"))
	for _, e := range objects(m["employee_compensations"], "") {
		worker := str(e, "employee_uuid")
		line := func(kind, name string, amount Money, hours any, employer bool) {
			r.child("payroll_pay_line", rowOf(map[string]any{"kind": kind, "name": name, "amount": amount, "hours": hours, "employer": employer}).ref("worker", worker))
		}
		var deductions, taxes, employer []Money
		hours := 0.0
		for _, x := range objects(e["fixed_compensations"], "") {
			line("earning", str(x, "name"), gustoMoney(x["amount"]), nil, false)
		}
		for _, x := range objects(e["hourly_compensations"], "") {
			line("earning", str(x, "name"), gustoMoney(x["amount"]), decimal(x["hours"]), false)
			h, _ := strconv.ParseFloat(num(x["hours"]), 64)
			hours += h
		}
		for _, x := range objects(e["paid_time_off"], "") {
			line("time_off", str(x, "name"), Money{}, decimal(x["hours"]), false)
		}
		for _, x := range objects(e["deductions"], "") {
			deductions = append(deductions, gustoMoney(x["amount"]))
			line("deduction", str(x, "name"), gustoMoney(x["amount"]), nil, false)
		}
		for _, x := range objects(e["benefits"], "") {
			line("benefit", str(x, "name"), gustoMoney(x["employee_deduction"]), nil, false)
			if c := gustoMoney(x["company_contribution"]); c.Minor != nil && *c.Minor != 0 {
				line("benefit", str(x, "name"), c, nil, true)
			}
		}
		for _, x := range objects(e["taxes"], "") {
			if x["employer"] == true {
				employer = append(employer, gustoMoney(x["amount"]))
			} else {
				taxes = append(taxes, gustoMoney(x["amount"]))
			}
			line("tax", str(x, "name"), gustoMoney(x["amount"]), nil, x["employer"] == true)
		}
		r.child("payroll_pay", rowOf(map[string]any{"gross_pay": gustoMoney(e["gross_pay"]), "net_pay": gustoMoney(e["net_pay"]),
			"check_amount": gustoMoney(e["check_amount"]), "deductions": sum(deductions...), "employee_taxes": sum(taxes...),
			"employer_taxes": sum(employer...), "hours": hours, "payment_method": str(e, "payment_method")}).ref("worker", worker))
	}
	return r
}

// gustoPayPeriodRows is a pay period, with the payroll it is paid in.
func gustoPayPeriodRows(m map[string]any) *Row {
	p := obj(m, "payroll")
	return gustoRow(m, map[string]any{"starts_on": day(str(m, "start_date")), "ends_on": day(str(m, "end_date")), "check_date": day(str(p, "check_date")),
		"deadline": instant(str(p, "payroll_deadline")), "processed": flag(p, "processed"), "payroll_type": str(p, "payroll_type")}).
		ref("schedule", str(m, "pay_schedule_uuid")).ref("run", str(p, "payroll_uuid"))
}

func gustoContractorPaymentRows(m map[string]any) *Row {
	return gustoRow(m, map[string]any{"paid_on": day(str(m, "date")), "wage": gustoMoney(m["wage"]), "bonus": gustoMoney(m["bonus"]),
		"reimbursement": gustoMoney(m["reimbursement"]), "wage_total": gustoMoney(m["wage_total"]), "hours": decimal(m["hours"]),
		"hourly_rate": gustoMoney(m["hourly_rate"]), "wage_type": str(m, "wage_type"), "payment_method": str(m, "payment_method"),
		"status": str(m, "status")}).ref("worker", str(m, "contractor_uuid"))
}

func gustoPaymentGroupRows(m map[string]any) *Row {
	t := obj(m, "totals")
	return gustoRow(m, map[string]any{"check_date": day(str(m, "check_date")), "debit_date": day(str(m, "debit_date")), "status": str(m, "status"),
		"amount": gustoMoney(t["amount"]), "debit_amount": gustoMoney(t["debit_amount"]), "wages": gustoMoney(t["wage_amount"]),
		"reimbursements": gustoMoney(t["reimbursement_amount"])})
}

func gustoTaskRows(m map[string]any) *Row {
	return gustoRow(m, map[string]any{"title": str(m, "title"), "description": str(m, "message"), "due_on": day(str(m, "due_at")),
		"priority": str(m, "priority"), "category": str(m, "category"), "status": str(m, "status"), "url": str(m, "deep_link")})
}
