package connect

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// gustoStandIn is a Gusto MCP server of n companies, each with people employees, two of them terminated, two
// contractors, and a regular payroll every two weeks since it joined, at the start of 2018: processed until
// today, and unprocessed for the 90 days ahead. calls counts the calls of each tool.
type gustoStandIn struct {
	mu        sync.Mutex
	calls     map[string]int
	versions  map[string]int
	payrolls  map[string][]map[string]any
	processed map[string]bool
	n, people int
}

func newGustoStandIn(n, people int) *gustoStandIn {
	g := &gustoStandIn{calls: map[string]int{}, versions: map[string]int{}, payrolls: map[string][]map[string]any{}, processed: map[string]bool{}, n: n, people: people}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	for co := range n {
		id := fmt.Sprint("co", co)
		for i, d := 0, time.Date(2018, 1, 5, 0, 0, 0, 0, time.UTC); !d.After(today.AddDate(0, 0, 90)); i, d = i+1, d.AddDate(0, 0, 14) {
			pid := fmt.Sprintf("%s-p%03d", id, i)
			g.processed[pid] = !d.After(today)
			g.payrolls[id] = append(g.payrolls[id], map[string]any{"payroll_uuid": pid, "check_date": d.Format(time.DateOnly), "company_uuid": id,
				"pay_period": map[string]any{"start_date": d.AddDate(0, 0, -18).Format(time.DateOnly), "end_date": d.AddDate(0, 0, -5).Format(time.DateOnly)},
				"totals":     map[string]any{"gross_pay": "2000.00", "net_pay": "1500.50", "employee_taxes": "300.00", "employer_taxes": "150.25"}})
		}
	}
	return g
}

func (g *gustoStandIn) count() map[string]int {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := maps.Clone(g.calls)
	clear(g.calls)
	return out
}

func total(calls map[string]int) int {
	n := 0
	for _, c := range calls {
		n += c
	}
	return n
}

func (g *gustoStandIn) client(t *testing.T) *Client {
	needs := func(key string, props ...string) map[string]any {
		s := schemaOf(append(props, key)...)
		s["required"] = []any{key}
		return s
	}
	tool := func(name string, schema map[string]any, answer func(args map[string]any) any) mcpTool {
		return mcpTool{schema, func(args map[string]any) any {
			g.mu.Lock()
			defer g.mu.Unlock()
			g.calls[name]++
			return answer(args)
		}}
	}
	list := func(name string, item func(co string, i int) map[string]any, n int) mcpTool {
		return tool(name, needs("company_uuid", "page", "per"), func(args map[string]any) any {
			out := []any{}
			for i := range n {
				out = append(out, item(fmt.Sprint(args["company_uuid"]), i))
			}
			return out
		})
	}
	uuid := func(kind string) func(co string, i int) map[string]any {
		return func(co string, i int) map[string]any { return map[string]any{"uuid": fmt.Sprint(co, "-", kind, i)} }
	}
	tools := map[string]mcpTool{
		"get_token_info": tool("get_token_info", schemaOf(), func(map[string]any) any {
			var cos []any
			for co := range g.n {
				cos = append(cos, map[string]any{"resource_type": "Company", "uuid": fmt.Sprint("co", co), "name": fmt.Sprint("Acme ", co)})
			}
			return map[string]any{"accessible_resources": cos}
		}),
		"get_company": tool("get_company", needs("company_uuid"), func(args map[string]any) any {
			return map[string]any{"uuid": args["company_uuid"], "name": "Acme", "trade_name": "Acme Co", "ein": "12-3456789", "entity_type": "LLC", "join_date": "2018-01-01"}
		}),
		"list_employees": tool("list_employees", needs("company_uuid", "page", "per", "terminated", "include"), func(args map[string]any) any {
			out := []any{}
			gone := args["terminated"] == true
			for i := range g.people {
				if gone != (i < 2) {
					continue
				}
				id := fmt.Sprint(args["company_uuid"], "-e", i)
				e := map[string]any{"uuid": id, "company_uuid": args["company_uuid"], "version": fmt.Sprint("v", g.versions[id]), "first_name": "Ann",
					"last_name": fmt.Sprint("Lee ", i), "email": fmt.Sprintf("ann%d@home.example", i), "work_email": fmt.Sprintf("Ann.%s.%d@Acme.example", args["company_uuid"], i),
					"terminated": gone, "jobs": []any{map[string]any{"title": "Engineer", "rate": "120000.00", "payment_unit": "Year", "hire_date": "2019-03-01", "primary": true}}}
				if gone {
					e["terminations"] = []any{map[string]any{"effective_date": "2024-06-30"}}
				}
				out = append(out, e)
			}
			return out
		}),
		"list_employee_employment_history": tool("list_employee_employment_history", needs("employee_uuid"), func(args map[string]any) any {
			return []any{map[string]any{"hire_date": "2019-03-01"}}
		}),
		"get_employee_work_addresses": tool("get_employee_work_addresses", needs("employee_uuid"), func(args map[string]any) any {
			return []any{map[string]any{"uuid": "wa", "city": "Denver"}}
		}),
		"get_contractors": tool("get_contractors", needs("company_uuid", "page", "per", "terminated"), func(args map[string]any) any {
			if args["terminated"] == true {
				return []any{}
			}
			return []any{map[string]any{"uuid": fmt.Sprint(args["company_uuid"], "-c0"), "company_uuid": args["company_uuid"], "first_name": "Cy",
				"email": "cy@contract.example", "is_active": true, "wage_type": "Hourly", "hourly_rate": "80.00"}}
		}),
		"get_departments":   list("get_departments", uuid("d"), 2),
		"get_locations":     list("get_locations", uuid("l"), 1),
		"get_pay_schedules": list("get_pay_schedules", uuid("s"), 1),
		"list_pay_schedule_assignments": tool("list_pay_schedule_assignments", needs("company_uuid"), func(args map[string]any) any {
			return map[string]any{"type": "single"}
		}),
		"list_earning_types": tool("list_earning_types", needs("company_uuid"), func(args map[string]any) any {
			return map[string]any{"default": []any{map[string]any{"uuid": "reg"}}, "custom": []any{map[string]any{"uuid": "bonus"}}}
		}),
		"list_custom_fields_schema": list("list_custom_fields_schema", uuid("f"), 1),
		"get_time_off_requests":     list("get_time_off_requests", uuid("t"), 1),
		"get_time_off_balances":     list("get_time_off_balances", func(co string, i int) map[string]any { return map[string]any{"employee_uuid": co + "-e2"} }, 1),
		"get_tasks":                 list("get_tasks", uuid("k"), 1),
		"list_payrolls": tool("list_payrolls", needs("company_uuid", "start_date", "end_date", "page", "per", "include"), func(args map[string]any) any {
			var in []any
			for _, p := range g.payrolls[fmt.Sprint(args["company_uuid"])] {
				if d := p["check_date"].(string); d >= args["start_date"].(string) && d <= args["end_date"].(string) {
					in = append(in, merged(p, map[string]any{"processed": g.processed[p["payroll_uuid"].(string)]}))
				}
			}
			page, per := int(args["page"].(float64)), int(args["per"].(float64))
			return in[min((page-1)*per, len(in)):min(page*per, len(in))]
		}),
		"get_payroll": tool("get_payroll", needs("payroll_uuid", "company_uuid", "employee_compensations_page"), func(args map[string]any) any {
			page := args["employee_compensations_page"]
			return map[string]any{"payroll_uuid": args["payroll_uuid"], "employee_compensations": []any{map[string]any{
				"employee_uuid": fmt.Sprint(args["company_uuid"], "-e", page), "gross_pay": "1000.00", "net_pay": "750.25", "payment_method": "Direct Deposit",
				"deductions": []any{map[string]any{"name": "401k", "amount": "50.00"}, map[string]any{"name": "Dental", "amount": 12.5}},
				"taxes":      []any{map[string]any{"name": "Federal", "employer": false, "amount": "150.00"}, map[string]any{"name": "FICA", "employer": true, "amount": "76.50"}}}},
				"employee_compensations_pagination": map[string]any{"has_more": page == 1.0}}
		}),
		"list_pay_periods": tool("list_pay_periods", needs("company_uuid", "start_date", "end_date", "page", "per"), func(args map[string]any) any {
			return []any{map[string]any{"pay_schedule_uuid": "s", "start_date": args["start_date"]}}
		}),
		"get_contractor_payments": tool("get_contractor_payments", needs("company_uuid", "start_date", "end_date", "page", "per"), func(args map[string]any) any {
			return []any{map[string]any{"uuid": fmt.Sprint(args["company_uuid"], "-cp-", args["start_date"]), "contractor_uuid": fmt.Sprint(args["company_uuid"], "-c0"),
				"date": args["start_date"], "wage_total": "800.00", "reimbursement": "20.00", "payment_method": "Check"}}
		}),
		"get_contractor_payment_groups": tool("get_contractor_payment_groups", needs("company_uuid", "start_date", "end_date", "page", "per"), func(args map[string]any) any {
			return []any{}
		}),
	}
	return mcpStandIn(t, "gusto", tools)
}

// gustoSync pulls each of c's streams as a sync would: every page of each that is due, from the cursor it
// ended the sync before at, which cursors keep; one pulled every so often is not due when skip says. It is
// the records each stream read.
func gustoSync(t *testing.T, c *Client, cursors map[string]string, skip func(s *Stream) bool) map[string][]Record {
	t.Helper()
	out := map[string][]Record{}
	for _, s := range c.p.Streams {
		if skip(s) {
			continue
		}
		p := &Page{Cursor: cursors[s.Name], More: true}
		for n := 0; p.More; n++ {
			if n > 1000 {
				t.Fatalf("%s pages on and on", s.Name)
			}
			var err error
			if p, err = s.Pull(c, p.Cursor); err != nil {
				t.Fatalf("%s: %v", s.Name, err)
			}
			out[s.Name] = append(out[s.Name], p.Records...)
			if strings.HasPrefix(s.Name, "pay") && s.Name != "pay_schedules" && s.Name != "pay_schedule_assignments" && p.Full {
				t.Errorf("a page of %s deletes what it does not list", s.Name)
			}
		}
		cursors[s.Name] = p.Cursor
	}
	return out
}

// Gusto reads everything once, and after that only what can have changed: the first sync lists every company's
// history, payroll detail and per-employee calls included; the next lists recent payrolls alone, reads no
// payroll's detail again and no employee's history whose version is the same, and leaves setup for a day. An
// employee whose version changes is read again, and a payroll that turns processed gets its detail then.
func TestGustoSyncsOnlyWhatChanged(t *testing.T) {
	g := newGustoStandIn(5, 11)
	c := g.client(t)
	if name, err := c.p.Account(c); err != nil || name != "Acme 0, Acme 1, Acme 2, Acme 3, Acme 4" {
		t.Errorf("the account: %q %v", name, err)
	}
	g.count()
	setup := []string{"company", "departments", "locations", "pay_schedules", "pay_schedule_assignments", "earning_types", "custom_fields", "time_off_balances", "pay_periods"}
	for _, s := range c.p.Streams {
		if slices.Contains(setup, s.Name) != (s.Every == 24*time.Hour) {
			t.Errorf("%s is pulled every %s", s.Name, s.Every)
		}
	}
	cursors := map[string]string{}
	first := gustoSync(t, c, cursors, func(*Stream) bool { return false })
	calls := g.count()
	processed := 0
	for _, p := range g.processed {
		if p {
			processed++
		}
	}
	t.Logf("a first sync: %d calls, %v", total(calls), calls)
	if calls["get_payroll"] != 2*processed {
		t.Errorf("get_payroll was called %d times for %d processed payrolls, two pages each", calls["get_payroll"], processed)
	}
	if calls["list_employee_employment_history"] != 55 || calls["get_employee_work_addresses"] != 55 {
		t.Errorf("each employee is not read once: %v", calls)
	}
	if n := len(first["payrolls"]); n != len(g.processed) {
		t.Errorf("the first sync read %d payrolls of %d", n, len(g.processed))
	}
	ids := func(rs []Record) []string {
		var out []string
		for _, r := range rs {
			out = append(out, r.ID)
		}
		return out
	}
	if got := ids(first["company"]); fmt.Sprint(got) != "[co0 co1 co2 co3 co4]" {
		t.Errorf("the companies: %v", got)
	}
	var e0 map[string]any
	for _, r := range first["employees"] {
		if r.ID == "co0-e0" {
			e0 = r.Data.(map[string]any)
		}
	}
	if fmt.Sprint(e0["employment_history"], e0["work_addresses"]) != "[map[hire_date:2019-03-01]] [map[city:Denver uuid:wa]]" {
		t.Errorf("an employee's history and addresses: %v", e0)
	}
	for _, r := range first["payrolls"] {
		if r.ID == "co0-p000" && fmt.Sprint(r.Data.(map[string]any)["employee_compensations"]) == "" {
			t.Errorf("a processed payroll without its detail: %v", r.Data)
		}
	}

	// The next sync, within the day.
	daily := func(s *Stream) bool { return s.Every > 0 }
	again := gustoSync(t, c, cursors, daily)
	calls = g.count()
	t.Logf("a repeat sync: %d calls, %v", total(calls), calls)
	for _, tool := range []string{"get_payroll", "list_employee_employment_history", "get_employee_work_addresses", "get_company", "get_token_info", "get_departments"} {
		if calls[tool] != 0 {
			t.Errorf("a repeat sync called %s %d times", tool, calls[tool])
		}
	}
	if calls["list_payrolls"] != 5 {
		t.Errorf("a repeat sync listed payrolls %d times, want the recent months of each company once", calls["list_payrolls"])
	}
	same := func(rs []Record) (n int) {
		for _, r := range rs {
			if r.Same {
				n++
			}
		}
		return n
	}
	if n := same(again["employees"]); n != 55 || len(again["employees"]) != 55 {
		t.Errorf("unchanged employees: %d the same of %d", n, len(again["employees"]))
	}
	if len(again["payrolls"]) == 0 || len(again["payrolls"]) > 5*16 {
		t.Errorf("the recent payrolls: %d", len(again["payrolls"]))
	}
	for _, r := range again["payrolls"] {
		if r.Same != g.processed[r.ID] {
			t.Errorf("payroll %s, processed %v, the same: %v", r.ID, g.processed[r.ID], r.Same)
		}
	}

	// One employee changes, and the next payroll is processed.
	var next string
	for _, p := range g.payrolls["co0"] {
		if id := p["payroll_uuid"].(string); !g.processed[id] {
			next = id
			break
		}
	}
	g.mu.Lock()
	g.versions["co0-e3"]++
	g.processed[next] = true
	g.mu.Unlock()
	again = gustoSync(t, c, cursors, daily)
	calls = g.count()
	if calls["list_employee_employment_history"] != 1 || calls["get_employee_work_addresses"] != 1 || calls["get_payroll"] != 2 {
		t.Errorf("after an employee changed and a payroll was processed: %v", calls)
	}
	for _, r := range again["employees"] {
		if r.Same != (r.ID != "co0-e3") {
			t.Errorf("employee %s the same: %v", r.ID, r.Same)
		}
	}
	for _, r := range again["payrolls"] {
		if r.ID == next && (r.Same || r.Data.(map[string]any)["employee_compensations"] == nil) {
			t.Errorf("the payroll just processed: %+v", r)
		}
	}

	// A day on, setup is read again, from the companies get_token_info lists again.
	for name, cur := range cursors {
		cursors[name] = strings.Replace(cur, `"companies_at":"`+time.Now().UTC().Format("2006"), `"companies_at":"2000`, 1)
	}
	gustoSync(t, c, cursors, func(*Stream) bool { return false })
	if calls = g.count(); calls["get_departments"] != 5 || calls["get_token_info"] == 0 || calls["get_payroll"] != 0 {
		t.Errorf("a day on: %v", calls)
	}
}

// Gusto's records are rows of payroll: a company with its EIN, employees and contractors as workers by their
// work email, a payroll with what each was paid, and a contractor payment as a pay of its own.
func TestGustoRows(t *testing.T) {
	g := newGustoStandIn(1, 3)
	c := g.client(t)
	recs := gustoSync(t, c, map[string]string{}, func(*Stream) bool { return false })
	rows := func(stream, id string) *Rows {
		for _, s := range c.p.Streams {
			for _, r := range recs[stream] {
				if s.Name == stream && r.ID == id {
					return s.Rows(r.Data.(map[string]any))
				}
			}
		}
		t.Fatalf("no %s %s", stream, id)
		return nil
	}
	if co := rows("company", "co0").Company; *co != (PayrollCompany{Name: "Acme", TradeName: "Acme Co", EntityType: "LLC", EIN: "12-3456789"}) {
		t.Errorf("the company: %+v", co)
	}
	w := rows("employees", "co0-e0").Worker
	if w.Kind != "employee" || w.Company != "co0" || w.Email != "Ann.co0.0@Acme.example" || w.PersonalEmail != "ann0@home.example" || w.Active ||
		w.Terminated != "2024-06-30" || w.Hired != "2019-03-01" || w.Title != "Engineer" || *w.Rate.Minor != 12000000 || w.Per != "Year" {
		t.Errorf("a terminated employee: %+v", w)
	}
	if w := rows("contractors", "co0-c0").Worker; w.Kind != "contractor" || !w.Active || *w.Rate.Minor != 8000 || w.Per != "Hour" {
		t.Errorf("a contractor: %+v", w)
	}
	r := rows("payrolls", "co0-p000")
	if r.Run.Check != "2018-01-05" || r.Run.Start != "2017-12-18" || !r.Run.Processed || *r.Run.Net.Minor != 150050 || *r.Run.EmployerTaxes.Minor != 15025 ||
		len(r.Pays) != 2 || r.Pays[0].Worker != "co0-e1" || *r.Pays[0].Deductions.Minor != 6250 || *r.Pays[0].Taxes.Minor != 15000 ||
		*r.Pays[0].EmployerTaxes.Minor != 7650 || r.Pays[0].Paid != "2018-01-05" {
		t.Errorf("a payroll: %+v %+v", r.Run, r.Pays)
	}
	var cp string
	for _, x := range recs["contractor_payments"] {
		cp = x.ID
	}
	if p := rows("contractor_payments", cp).Pays; len(p) != 1 || p[0].Worker != "co0-c0" || *p[0].Gross.Minor != 80000 || *p[0].Net.Minor != 82000 {
		t.Errorf("a contractor payment: %+v", p)
	}
}
