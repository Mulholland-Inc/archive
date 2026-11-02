package connect

import (
	"cmp"
	"errors"
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
	Register(&Connector{ID: "gusto", Name: "Gusto", Integration: true, MCP: "https://mcp.api.gusto.com",
		Scopes: []string{"public", "companies:read", "employees:read", "contractors:read", "departments:read", "jobs:read", "compensations:read",
			"employments:read", "pay_schedules:read", "payrolls:read", "time_off_requests:read"},
		Account: gustoAccount,
		Streams: []*Stream{
			{Name: "company", Every: gustoDaily, Pull: (&gustoStream{each: gustoCompanyArgs, pull: gustoCompany}).Pull, Rows: gustoCompanyRows,
				Doc: "Each company, as get_company answers: its name, entity type, EIN, contact, locations and compensation types, by its uuid; read a day apart."},
			{Name: "employees", Pull: (&gustoStream{each: gustoStatuses, pull: gustoEmployees}).Pull, Rows: gustoEmployeeRows,
				Doc: "Every employee, the terminated too, as list_employees answers with every compensation, home address and custom field, and with " +
					"employment_history and work_addresses, which list_employee_employment_history and get_employee_work_addresses answer, read again " +
					"only when the employee's version changes; 100 a page, listed whole each sync, so one gone is deleted."},
			gustoList("contractors", "get_contractors", "Every contractor, the terminated too", gustoStatuses, 0, nil, gustoContractorRows),
			gustoList("departments", "get_departments", "Every department, with who is in it", gustoCompanyArgs, gustoDaily, nil, nil),
			gustoList("locations", "get_locations", "Every location", gustoCompanyArgs, gustoDaily, nil, nil),
			gustoList("pay_schedules", "get_pay_schedules", "Every pay schedule", gustoCompanyArgs, gustoDaily, nil, nil),
			{Name: "pay_schedule_assignments", Every: gustoDaily, Pull: (&gustoStream{each: gustoCompanyArgs, pull: gustoOne("list_pay_schedule_assignments")}).Pull,
				Doc: "How each company assigns people to pay schedules, as list_pay_schedule_assignments answers, by the company's uuid; read a day apart."},
			{Name: "earning_types", Every: gustoDaily, Pull: (&gustoStream{each: gustoCompanyArgs, pull: gustoEarningTypes}).Pull,
				Doc: "Every earning type of each company, default and custom, as list_earning_types answers, by uuid; read a day apart."},
			gustoList("custom_fields", "list_custom_fields_schema", "Every custom field each company defines", gustoCompanyArgs, gustoDaily, nil, nil),
			gustoList("time_off_requests", "get_time_off_requests", "Every time-off request", gustoCompanyArgs, 0, nil, nil),
			gustoList("time_off_balances", "get_time_off_balances", "Each employee's time-off balances, per policy", gustoCompanyArgs, gustoDaily,
				func(at, it map[string]any) string { return str(it, "employee_uuid") }, nil),
			{Name: "payrolls", Gap: time.Second, Pull: (&gustoStream{each: gustoYears, pull: gustoPayrolls, recent: true}).Pull, Rows: gustoPayrollRows,
				Doc: "Every payroll, processed or not, of every type, with its totals and taxes, and once processed what get_payroll adds, read once: each " +
					"person's earnings, taxes, deductions and net pay. The first sync lists every year, 25 a page; the next ones from three months back " +
					"to 90 days ahead, by uuid, and delete nothing."},
			gustoYearList("pay_periods", "list_pay_periods", "Every pay period and the payroll it is paid in", gustoDaily,
				func(at, it map[string]any) string { return str(it, "pay_schedule_uuid") + "/" + str(it, "start_date") }, nil),
			gustoYearList("contractor_payments", "get_contractor_payments", "Every contractor payment", 0, nil, gustoContractorPaymentRows),
			gustoYearList("contractor_payment_groups", "get_contractor_payment_groups", "Every batch of contractor payments", 0, nil, nil),
			gustoList("tasks", "get_tasks", "Each company's open to-dos in Gusto, like payroll blockers and documents to sign", gustoCompanyArgs, 0, nil, nil),
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
func gustoListPull(c *Client, tool string, args, opts map[string]any, key func(args, it map[string]any) string, cursor string) (*Page, error) {
	l := &mcpList{tool: tool, args: args, opts: opts, limit: "per,limit,page_size", max: 100, page: "page", after: "cursor,after",
		key: gustoID, id: "uuid", updated: "updated_at"}
	if key != nil {
		l.key = func(it map[string]any) string { return key(args, it) }
	}
	return l.pull(c, cursor)
}

// gustoList is a stream of tool's list for each set of arguments each finds, listed whole every so often, so
// one gone is deleted; key is a record's id, as gustoListPull has it, and rows its records' rows.
func gustoList(name, tool, what string, each func(c *Client, st *gustoCursor) ([]map[string]any, error), every time.Duration,
	key func(args, it map[string]any) string, rows func(map[string]any) *Rows) *Stream {
	g := &gustoStream{each: each, pull: func(c *Client, st *gustoCursor, args map[string]any, cursor string) (*Page, error) {
		return gustoListPull(c, tool, args, nil, key, cursor)
	}}
	doc := what + ", as " + tool + " answers, 100 a page, listed whole each sync, so one gone is deleted."
	if every > 0 {
		doc = what + ", as " + tool + " answers, 100 a page, listed whole a day apart, so one gone is deleted."
	}
	return &Stream{Name: name, Doc: doc, Every: every, Pull: g.Pull, Rows: rows}
}

// gustoYearList is a stream of tool's list for each company's years (gustoYears): every year once, and the
// recent months after, in pages that delete nothing.
func gustoYearList(name, tool, what string, every time.Duration, key func(args, it map[string]any) string, rows func(map[string]any) *Rows) *Stream {
	g := &gustoStream{each: gustoYears, recent: true, pull: func(c *Client, st *gustoCursor, args map[string]any, cursor string) (*Page, error) {
		return gustoListPull(c, tool, args, nil, key, cursor)
	}}
	doc := what + ", as " + tool + " answers, 100 a page: every year on the first sync, and from three months back to 90 days ahead after, which deletes nothing."
	if every > 0 {
		doc = strings.Replace(doc, "ahead after", "ahead a day apart after", 1)
	}
	return &Stream{Name: name, Every: every, Pull: g.Pull, Rows: rows, Doc: doc}
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
	page, err := gustoListPull(c, "list_employees", args, map[string]any{"include": "all_compensations,all_home_addresses,custom_fields"}, nil, cursor)
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

func gustoCompanyRows(m map[string]any) *Rows {
	return &Rows{Company: &PayrollCompany{Name: str(m, "name"), TradeName: str(m, "trade_name"), EntityType: str(m, "entity_type"), EIN: str(m, "ein")}}
}

// gustoEmployeeRows is an employee as a worker: their primary job, or else their first, says their title, start
// and pay, and their last termination when they left.
func gustoEmployeeRows(m map[string]any) *Rows {
	w := &PayrollWorker{Kind: "employee", Company: str(m, "company_uuid"), Given: str(m, "first_name"), Family: str(m, "last_name"),
		Email: cmp.Or(str(m, "work_email"), str(m, "email")), PersonalEmail: str(m, "email"), Active: m["terminated"] != true}
	jobs := objects(m["jobs"], "")
	for i, j := range jobs {
		if i == 0 || j["primary"] == true {
			w.Title, w.Hired, w.Rate, w.Per = str(j, "title"), day(str(j, "hire_date")), money(j["rate"], gustoCurrency), str(j, "payment_unit")
			if j["primary"] == true {
				break
			}
		}
	}
	if w.Hired == "" {
		w.Hired = day(str(m, "hired_at"))
	}
	if !w.Active {
		for _, t := range objects(m["terminations"], "") {
			if d := day(str(t, "effective_date")); d > w.Terminated {
				w.Terminated = d
			}
		}
	}
	return &Rows{Worker: w}
}

func gustoContractorRows(m map[string]any) *Rows {
	w := &PayrollWorker{Kind: "contractor", Company: str(m, "company_uuid"), Given: str(m, "first_name"), Family: str(m, "last_name"),
		Business: str(m, "business_name"), Email: str(m, "email"), Active: m["is_active"] != false, Hired: day(str(m, "start_date"))}
	if strings.EqualFold(str(m, "wage_type"), "Hourly") {
		w.Rate, w.Per = money(m["hourly_rate"], gustoCurrency), "Hour"
	}
	return &Rows{Worker: w}
}

// gustoPayrollRows is a payroll with what each employee was paid in it: their taxes are those Gusto says the
// employee pays, and the rest the company's.
func gustoPayrollRows(m map[string]any) *Rows {
	totals, _ := m["totals"].(map[string]any)
	period, _ := m["pay_period"].(map[string]any)
	r := &Rows{Run: &PayrollRun{Company: str(m, "company_uuid"), Start: day(str(period, "start_date")), End: day(str(period, "end_date")),
		Check: day(str(m, "check_date")), Processed: m["processed"] == true, OffCycle: m["off_cycle"] == true, External: m["external"] == true,
		Gross: money(totals["gross_pay"], gustoCurrency), Net: money(totals["net_pay"], gustoCurrency),
		Taxes: money(totals["employee_taxes"], gustoCurrency), EmployerTaxes: money(totals["employer_taxes"], gustoCurrency)}}
	for _, e := range objects(m["employee_compensations"], "") {
		p := PayrollPay{Worker: str(e, "employee_uuid"), Paid: r.Run.Check, Method: str(e, "payment_method"),
			Gross: money(e["gross_pay"], gustoCurrency), Net: money(e["net_pay"], gustoCurrency)}
		var deductions, taxes, employer []Money
		for _, d := range objects(e["deductions"], "") {
			deductions = append(deductions, money(d["amount"], gustoCurrency))
		}
		for _, t := range objects(e["taxes"], "") {
			if t["employer"] == true {
				employer = append(employer, money(t["amount"], gustoCurrency))
			} else {
				taxes = append(taxes, money(t["amount"], gustoCurrency))
			}
		}
		p.Deductions, p.Taxes, p.EmployerTaxes = sum(deductions...), sum(taxes...), sum(employer...)
		r.Pays = append(r.Pays, p)
	}
	return r
}

// gustoContractorPaymentRows is a contractor payment as a pay of its own: its wage total, and what it paid
// with reimbursements on top.
func gustoContractorPaymentRows(m map[string]any) *Rows {
	gross := money(m["wage_total"], gustoCurrency)
	return &Rows{Pays: []PayrollPay{{Worker: str(m, "contractor_uuid"), Paid: day(str(m, "date")), Method: str(m, "payment_method"),
		Gross: gross, Net: sum(gross, money(m["reimbursement"], gustoCurrency))}}}
}
