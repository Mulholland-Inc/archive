package main

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"
	"time"

	"github.com/Mulholland-Inc/archive/connect"
	"github.com/google/uuid"
)

// streamOf is connector id's stream name.
func streamOf(t *testing.T, id, name string) *connect.Stream {
	t.Helper()
	for _, s := range connect.Get(id).Streams {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("%s has no stream %s", id, name)
	return nil
}

// A synced Gmail message, Notion page and Gusto company, department, employee, payroll and contractor payment
// land as rows of mail, docs and payroll, with the rows that belong to them, and link what they name whichever
// came first. The people they name are one person by their address, whichever connection names them. Each
// member sees the rows of the connections they see: their own mail, and the archive's Gusto while it is not
// narrowed to a group they are not in; a person only through a row they see. A message deleted there, a page a
// full listing no longer lists, and a connection disconnected take their rows with them, and the people only
// they named.
func TestSyncedRecordsLandInTheirTables(t *testing.T) {
	t.Parallel()
	in := newInst(t, "ARCHIVE_MCP_GUSTO=http://127.0.0.1:1/mcp", "ARCHIVE_MCP_NOTION=http://127.0.0.1:1/mcp")
	in.must("user", "add", "alice@example.com")
	in.must("user", "add", "bob@example.com", "-role", "editor")
	srv := in.serve()
	alice := decode[Token](t, in.must("token", "create")).Secret
	bob := decode[Token](t, in.must("-as", "bob@example.com", "token", "create")).Secret
	a := hubOn(t, in, nil)
	ctx := t.Context()
	connection := func(email, provider string, integration bool) uuid.UUID {
		id := uuid.New()
		in.psql("archive", "INSERT INTO archive.connection (id, member, provider, account, token, sync_at, integration) SELECT '"+id.String()+"', id, '"+provider+
			"', email, '\\x00', NULL, "+map[bool]string{true: "true", false: "false"}[integration]+" FROM archive.member WHERE email = '"+email+"'")
		return id
	}
	mailA, mailB := connection("alice@example.com", "gmail", false), connection("bob@example.com", "gmail", false)
	gusto, notion := connection("alice@example.com", "gusto", true), connection("alice@example.com", "notion", false)
	save := func(conn uuid.UUID, provider, stream string, page *connect.Page) {
		t.Helper()
		if err := a.sys(ctx, func(ctx context.Context) error {
			q := a.st.db(ctx)
			now, err := q.TxNow(ctx)
			if err != nil {
				return err
			}
			c, err := q.Connection(ctx, conn)
			if err != nil {
				return err
			}
			return syncSave(ctx, &syncDB{q: q, db: a.st.Q(ctx), id: conn, acc: c.Access, now: now}, streamOf(t, provider, stream), page, &syncStream{})
		}); err != nil {
			t.Fatalf("saving %s %s: %v", provider, stream, err)
		}
	}
	msg := func(id, from, to string) connect.Record {
		return connect.Record{ID: id, Data: map[string]any{"id": id, "threadId": "t-" + id, "labelIds": []string{"INBOX"}, "snippet": "About pay",
			"subject": "Pay " + id, "from": from, "to": to, "cc": "", "date": "Mon, 02 Nov 2026 10:00:00 +0100", "text": "Hello", "internalDate": "1793613600000",
			"attachments": []map[string]any{{"filename": "a.pdf", "mimeType": "application/pdf", "size": 1200}}}}
	}
	save(mailA, "gmail", "messages", &connect.Page{Records: []connect.Record{
		msg("m1", `"Ann Lee" <Ann.Lee@Acme.example>`, "alice@example.com, Zed <zed@elsewhere.example>"),
	}})
	save(mailB, "gmail", "messages", &connect.Page{Records: []connect.Record{msg("m2", "bob@example.com", "carl@example.net")}})
	save(notion, "notion", "private_pages", &connect.Page{Full: true, Records: []connect.Record{
		{ID: "n1", Data: map[string]any{"type": "page", "title": "Plans", "url": "https://www.notion.so/Plans-n1", "icon": "📝"}},
		{ID: "n2", Data: map[string]any{"type": "database", "title": "Tasks", "url": "https://www.notion.so/Tasks-n2"}},
	}})
	save(gusto, "gusto", "employees", &connect.Page{Records: []connect.Record{{ID: "e1", Data: map[string]any{"uuid": "e1", "company_uuid": "co1",
		"first_name": "Ann", "last_name": "Lee", "email": "ann@home.example", "work_email": "ann.lee@acme.example", "terminated": false, "department_uuid": "d1",
		"jobs":           []any{map[string]any{"uuid": "j1", "title": "Engineer", "rate": "120000.00", "payment_unit": "Year", "hire_date": "2019-03-01", "primary": true}},
		"home_addresses": []any{map[string]any{"street_1": "1 Main St", "city": "Denver", "state": "CO", "zip": "80202", "active": true}}}}}})
	save(gusto, "gusto", "company", &connect.Page{Records: []connect.Record{{ID: "co1", Data: map[string]any{"uuid": "co1", "name": "Acme", "ein": "123456789",
		"entity_type": "LLC", "primary_payroll_admin": map[string]any{"first_name": "Ann", "last_name": "Lee", "email": "ann.lee@acme.example"}}}}})
	save(gusto, "gusto", "departments", &connect.Page{Records: []connect.Record{{ID: "d1", Data: map[string]any{"uuid": "d1", "company_uuid": "co1", "title": "Engineering",
		"employees": []any{map[string]any{"uuid": "e1"}}}}}})
	save(gusto, "gusto", "payrolls", &connect.Page{Records: []connect.Record{{ID: "p1", Data: map[string]any{"payroll_uuid": "p1", "company_uuid": "co1",
		"processed": true, "check_date": "2026-10-30", "pay_period": map[string]any{"start_date": "2026-10-12", "end_date": "2026-10-25"},
		"totals": map[string]any{"gross_pay": "5000.00", "net_pay": "3600.10", "employee_taxes": "900.00", "employer_taxes": "400.00"},
		"employee_compensations": []any{map[string]any{"employee_uuid": "e1", "gross_pay": "5000.00", "net_pay": "3600.10", "payment_method": "Direct Deposit",
			"fixed_compensations": []any{map[string]any{"name": "Salary", "amount": "5000.00"}},
			"deductions":          []any{map[string]any{"name": "401k", "amount": "499.90"}}, "taxes": []any{map[string]any{"name": "Federal", "employer": false, "amount": "900.00"}}}}}}}})
	save(gusto, "gusto", "contractor_payments", &connect.Page{Records: []connect.Record{{ID: "cp1", Data: map[string]any{"uuid": "cp1", "company_uuid": "co1",
		"contractor_uuid": "c1", "date": "2026-10-15", "wage_total": "800.00", "reimbursement": "20.00", "payment_method": "Check"}}}})

	want := func(what, sql, want string) {
		t.Helper()
		if got := in.psql("archive", sql); got != want {
			t.Errorf("%s: %q, want %q", what, got, want)
		}
	}
	want("the mail", "SELECT string_agg(subject || ' ' || thread || ' ' || text || ' ' || sent_at || ' ' || labels::text, ', ' ORDER BY subject) FROM archive.mail_message",
		"Pay m1 t-m1 Hello 2026-11-02 09:00:00+00 {INBOX}, Pay m2 t-m2 Hello 2026-11-02 09:00:00+00 {INBOX}")
	want("who the mail names", `SELECT string_agg(m.subject || ' ' || p.role || ' ' || k.email || ' ' || p.name, ', ' ORDER BY m.subject, p.role, k.email)
		FROM archive.mail_party p JOIN archive.mail_message m ON m.id = p.message JOIN archive.known_person k ON k.id = p.person`,
		"Pay m1 from ann.lee@acme.example Ann Lee, Pay m1 to alice@example.com , Pay m1 to zed@elsewhere.example Zed, Pay m2 from bob@example.com , Pay m2 to carl@example.net")
	want("the files attached", "SELECT string_agg(filename || ' ' || mime || ' ' || size, ', ') FROM archive.mail_attachment", "a.pdf application/pdf 1200, a.pdf application/pdf 1200")
	want("the pages", "SELECT string_agg(kind || ' ' || title || ' ' || url || ' ' || icon, ', ' ORDER BY title) FROM archive.doc_page",
		"page Plans https://www.notion.so/Plans-n1 📝, database Tasks https://www.notion.so/Tasks-n2")
	want("the company, its EIN and its admin", `SELECT c.name || ' ' || c.entity_type || ' ' || o.ein || ' ' || k.email || ' ' || c.admin_name
		FROM archive.payroll_company c JOIN archive.known_org o ON o.id = c.org JOIN archive.known_person k ON k.id = c.admin`, "Acme LLC 12-3456789 ann.lee@acme.example Ann Lee")
	want("the employee, linked to the company and department written after them", `SELECT w.kind || ' ' || w.name || ' ' || w.email || ' ' || w.personal_email || ' ' ||
		c.name || ' ' || d.title || ' ' || w.title || ' ' || w.hired_on || ' ' || w.active || ' ' || w.pay_rate::text || ' ' || w.pay_per
		FROM archive.payroll_worker w JOIN archive.payroll_company c ON c.id = w.company JOIN archive.payroll_department d ON d.id = w.department`,
		"employee Ann Lee ann.lee@acme.example ann@home.example Acme Engineering Engineer 2019-03-01 true (12000000,USD) Year")
	want("their job, home and department", `SELECT (SELECT j.title || ' ' || j.primary_job FROM archive.payroll_job j) || ' ' || (SELECT a.kind || ' ' || a.city FROM archive.payroll_address a)
		|| ' ' || (SELECT d.title || ' ' || w.name FROM archive.payroll_department_member m JOIN archive.payroll_department d ON d.id = m.department JOIN archive.payroll_worker w ON w.id = m.worker)`,
		"Engineer true home Denver Engineering Ann Lee")
	want("the payroll", `SELECT r.check_date || ' ' || r.period_start || ' ' || r.processed || ' ' || r.gross_pay::text || ' ' || r.net_pay::text || ' ' || r.employer_taxes::text || ' ' || c.name
		FROM archive.payroll_run r JOIN archive.payroll_company c ON c.id = r.company`, "2026-10-30 2026-10-12 true (500000,USD) (360010,USD) (40000,USD) Acme")
	want("what Ann was paid in it", `SELECT p.gross_pay::text || ' ' || p.net_pay::text || ' ' || p.deductions::text || ' ' || p.employee_taxes::text || ' ' || w.name || ' ' || p.payment_method
		FROM archive.payroll_pay p JOIN archive.payroll_worker w ON w.id = p.worker`, "(500000,USD) (360010,USD) (49990,USD) (90000,USD) Ann Lee Direct Deposit")
	want("line by line", "SELECT string_agg(kind || ' ' || name || ' ' || amount::text, ', ' ORDER BY kind) FROM archive.payroll_pay_line",
		"deduction 401k (49990,USD), earning Salary (500000,USD), tax Federal (90000,USD)")
	want("the contractor payment, its contractor not synced", "SELECT paid_on || ' ' || wage_total::text || ' ' || worker_source || ' ' || (worker IS NULL) || ' ' || payment_method FROM archive.payroll_contractor_payment",
		"2026-10-15 (80000,USD) c1 true Check")
	want("one person for an address two connections name", "SELECT count(*) FROM archive.known_person WHERE email = 'ann.lee@acme.example'", "1")
	want("the person of the employee is the sender of the mail", `SELECT count(DISTINCT k.id) FROM archive.known_person k JOIN archive.payroll_worker w ON w.person = k.id
		JOIN archive.mail_party p ON p.person = k.id`, "1")

	sees := func(tok, sql string) string {
		t.Helper()
		_, out := e2eRun(t, srv.url+"/api", tok, "const rows = await archive.query(`"+sql+"`); return rows.map((r) => Object.values(r).join(' ')).join(', ')")
		if out.Error != "" {
			t.Fatalf("%s: %s", sql, out.Error)
		}
		var s string
		json.Unmarshal(out.Result, &s)
		return s
	}
	people := "SELECT email FROM known_person ORDER BY email"
	if got := sees(alice, people); got != "alice@example.com, ann.lee@acme.example, zed@elsewhere.example" {
		t.Errorf("alice sees the people %q", got)
	}
	if got := sees(bob, people); got != "ann.lee@acme.example, bob@example.com, carl@example.net" {
		t.Errorf("bob sees the people %q", got)
	}
	if got := sees(bob, "SELECT subject FROM mail_message") + " | " + sees(bob, "SELECT count(*) FROM mail_party") + " | " + sees(bob, "SELECT count(*) FROM mail_attachment") +
		" | " + sees(bob, "SELECT title FROM doc_page"); got != "Pay m2 | 2 | 1 | " {
		t.Errorf("bob sees the mail and pages %q", got)
	}
	gustoRows := "SELECT (SELECT count(*) FROM payroll_worker) || ' ' || (SELECT count(*) FROM payroll_job) || ' ' || (SELECT count(*) FROM payroll_run) || ' ' || (SELECT count(*) FROM payroll_pay_line) || ' ' || (SELECT count(*) FROM known_org)"
	if got := sees(bob, gustoRows); got != "1 1 1 3 1" {
		t.Errorf("bob sees of the archive's Gusto %q", got)
	}
	if st, _ := call(t, "PUT", srv.url+"/connect/"+gusto.String()+"/access", alice, map[string]string{"access": "hr"}, nil); st != 204 {
		t.Fatalf("narrowing Gusto to hr: %d", st)
	}
	if got := sees(bob, gustoRows); got != "0 0 0 0 0" {
		t.Errorf("bob sees of the archive's Gusto narrowed to hr %q", got)
	}
	if got := sees(bob, people); got != "bob@example.com, carl@example.net" {
		t.Errorf("bob sees the people once Gusto is narrowed to hr: %q", got)
	}
	if got := sees(alice, gustoRows); got != "1 1 1 3 1" {
		t.Errorf("the owner sees %s of Gusto narrowed to hr", got)
	}

	// A message deleted there takes its rows, and the people only it named.
	save(mailA, "gmail", "messages", &connect.Page{Records: []connect.Record{{ID: "m1", Deleted: true}}})
	want("after a message is deleted", "SELECT (SELECT count(*) FROM archive.mail_message) || ' ' || (SELECT count(*) FROM archive.mail_party) || ' ' || (SELECT string_agg(email, ' ' ORDER BY email) FROM archive.known_person)",
		"1 2 ann.lee@acme.example bob@example.com carl@example.net")

	// A full listing that lists a page as the same keeps it as it is, and deletes the one it no longer lists.
	in.psql("archive", "UPDATE archive.doc_page SET title = 'Plans, kept' WHERE source_id = 'n1'")
	save(notion, "notion", "private_pages", &connect.Page{Full: true, Records: []connect.Record{{ID: "n1", Same: true}}})
	want("after a full listing", "SELECT string_agg(source_id || ' ' || title, ', ') FROM archive.doc_page", "n1 Plans, kept")

	// Disconnecting Gusto takes every row of it, and the organization and people only it knew.
	if st, _ := call(t, "DELETE", srv.url+"/connect/"+gusto.String(), alice, nil, nil); st != 204 {
		t.Fatalf("disconnecting Gusto: %d", st)
	}
	want("after Gusto is disconnected", `SELECT (SELECT count(*) FROM archive.payroll_company) + (SELECT count(*) FROM archive.payroll_worker) + (SELECT count(*) FROM archive.payroll_job)
		+ (SELECT count(*) FROM archive.payroll_run) + (SELECT count(*) FROM archive.payroll_pay_line) + (SELECT count(*) FROM archive.payroll_contractor_payment)
		+ (SELECT count(*) FROM archive.known_org) || ' ' || (SELECT string_agg(email, ' ' ORDER BY email) FROM archive.known_person)`,
		"0 bob@example.com carl@example.net")
}

// A stream pulled every so often is not due again until that long after it last caught up, unless a listing
// of it is under way; others are due on every ask. A stream whose rows are in tables of before is due, and
// starts from scratch.
func TestAStreamPulledEverySoOftenWaits(t *testing.T) {
	now := time.Now()
	asked, before, hourAgo, dayAgo := now, now.Add(-time.Minute), now.Add(-time.Hour), now.Add(-25*time.Hour)
	streams := []*connect.Stream{{Name: "lists"}, {Name: "setup", Every: 24 * time.Hour}}
	st := syncState{"lists": {Done: &asked, Rows: syncRowsVersion}, "setup": {Done: &before, Last: &hourAgo, Rows: syncRowsVersion}}
	if s := syncPick(streams, st, asked); s != nil {
		t.Errorf("a stream that caught up an hour ago was picked: %s", s.Name)
	}
	st["setup"].Last = &dayAgo
	if s := syncPick(streams, st, asked); s == nil || s.Name != "setup" {
		t.Errorf("a stream that caught up a day ago was not picked: %v", s)
	}
	st["setup"].Last, st["setup"].More = &hourAgo, true
	if s := syncPick(streams, st, asked); s == nil || s.Name != "setup" {
		t.Errorf("a stream whose listing is under way was not picked: %v", s)
	}
	old := &syncStream{Cursor: `{"h":"9"}`, Done: &asked, Named: &hourAgo}
	st = syncState{"lists": old, "setup": {Done: &asked, Last: &hourAgo, Rows: syncRowsVersion}}
	if s := syncPick(streams, st, asked); s == nil || s.Name != "lists" {
		t.Errorf("a stream of tables before was not picked: %v", s)
	}
	if f := old.fresh(); f.Cursor != "" || f.Done != nil || f.Named != &hourAgo {
		t.Errorf("a stream of tables before starts from %+v", f)
	}
}

// A sync asks the provider again what the account is called, a day after it last did, and keeps the name, unless
// another of the member's own connections to it is called that.
func TestASyncNamesItsAccountAgainADayOn(t *testing.T) {
	t.Parallel()
	f := newFakeMCP(t)
	f.gustoTools("Acme")
	in := newInst(t, "ARCHIVE_MCP_GUSTO="+f.srv.URL+"/mcp")
	in.must("user", "add", "alice@example.com")
	srv := in.serve()
	ws := in.psql("archive", "SELECT id FROM archive.workspace ORDER BY created_at LIMIT 1")
	alice := decode[Token](t, in.must("token", "create")).Secret
	_, hdr := call(t, "GET", srv.url+"/connect/gusto?workspace="+ws, alice, nil, nil)
	loc, _ := url.Parse(hdr.Get("Location"))
	if st, hdr := call(t, "GET", srv.url+"/connect/gusto/callback?"+url.Values{"state": {loc.Query().Get("state")}, "code": {"alice@example.com"}}.Encode(), alice, nil, nil); st != 303 {
		t.Fatalf("connecting Gusto: %d %s", st, hdr.Get("Location"))
	}
	id := uuid.MustParse(in.psql("archive", "SELECT id FROM archive.connection WHERE provider = 'gusto'"))
	account := func() string {
		return in.psql("archive", "SELECT account FROM archive.connection WHERE id = '"+id.String()+"'")
	}
	if got := account(); got != "Acme" {
		t.Fatalf("connected as %q", got)
	}
	a := hubOn(t, in, map[string]string{"ARCHIVE_MCP_GUSTO": f.srv.URL + "/mcp"})
	// syncAgain asks for the company stream again, last named at named.
	syncAgain := func(named string) {
		t.Helper()
		in.psql("archive", `UPDATE archive.connection SET cursor = jsonb_set(cursor, '{company}', jsonb_build_object('rows', 2, 'named', '`+named+`'::text)),
			asked_at = now(), sync_at = now() WHERE id = '`+id.String()+`'`)
		syncPages(t, a, id, 1)
	}
	f.do(func() { f.companies = []string{"Acme", "Acme Labs"} })
	syncPages(t, a, id, 1)
	if got := account(); got != "Acme, Acme Labs" {
		t.Errorf("the first sync named it %q", got)
	}
	if n := in.psql("archive", "SELECT string_agg(source_id, ' ') FROM archive.payroll_company WHERE connection = '"+id.String()+"'"); n != "co0" {
		t.Errorf("the first page wrote the companies %q", n)
	}
	f.do(func() { f.companies = []string{"Acme Co"} })
	syncAgain(time.Now().Add(-time.Hour).UTC().Format(time.RFC3339))
	if got := account(); got != "Acme, Acme Labs" {
		t.Errorf("a sync within the day named it %q", got)
	}
	syncAgain(time.Now().Add(-25 * time.Hour).UTC().Format(time.RFC3339))
	if got := account(); got != "Acme Co" {
		t.Errorf("a sync a day on named it %q", got)
	}
	in.psql("archive", "INSERT INTO archive.connection (member, provider, account, token, sync_at) SELECT id, 'gusto', 'Acme Two', '\\x00', NULL FROM archive.member")
	f.do(func() { f.companies = []string{"Acme Two"} })
	syncAgain(time.Now().Add(-25 * time.Hour).UTC().Format(time.RFC3339))
	if got := account(); got != "Acme Co" {
		t.Errorf("a sync named it %q, as another of the member's own connections is", got)
	}
}

// Every stream of every connector writes the table it names, with the rows that belong to its rows, from a
// record with everything any mapping reads: a column a mapping names that its table has not fails.
func TestEveryStreamWritesItsTable(t *testing.T) {
	t.Parallel()
	in := newInst(t)
	in.must("user", "add", "alice@example.com")
	a := hubOn(t, in, nil)
	ctx := t.Context()
	all := map[string]any{"uuid": "x1", "company_uuid": "co1", "employee_uuid": "e1", "contractor_uuid": "c1", "first_name": "Ann", "last_name": "Lee",
		"email": "ann@acme.example", "work_email": "ann.lee@acme.example", "date_of_birth": "1990-01-01", "start_date": "2026-01-01", "end_date": "2026-01-14",
		"check_date": "2026-01-16", "date": "2026-01-16", "title": "Plans", "url": "https://example.com/x", "type": "page", "icon": map[string]any{"emoji": "📝"},
		"name": "Acme", "ein": "12-3456789", "wage_type": "Hourly", "hourly_rate": "80.00", "frequency": "Biweekly", "anchor_pay_date": "2026-01-16",
		"day_1": 15.0, "hours": "8.0", "wage_total": "800.00", "due_date": "2026-02-01", "priority": "high", "status": "approved", "request_type": "vacation",
		"payroll_deadline": "2026-01-14T23:00:00Z", "processed": true, "selection_options": []any{"a", "b"},
		"from": "Ann <ann@acme.example>", "to": "bob@example.com", "subject": "Hi", "text": "Hello", "labelIds": []any{"INBOX"},
		"internalDate": "1793613600000", "attachments": []any{map[string]any{"filename": "a.pdf", "mimeType": "application/pdf", "size": 10.0}},
		"primary_payroll_admin": map[string]any{"email": "ann@acme.example"}, "primary_signatory": map[string]any{"email": "bob@example.com"},
		"jobs": []any{map[string]any{"uuid": "j1", "title": "Engineer", "rate": "1.00", "primary": true,
			"compensations": []any{map[string]any{"rate": "1.00", "effective_date": "2026-01-01"}}}},
		"home_addresses": []any{map[string]any{"street_1": "1 Main St"}}, "work_addresses": []any{map[string]any{"street_1": "2 Main St", "location_uuid": "l1"}},
		"address": map[string]any{"street_1": "3 Main St"}, "employment_history": []any{map[string]any{"hire_date": "2026-01-01"}},
		"custom_fields": []any{map[string]any{"name": "Shirt", "value": "M", "company_custom_field_id": "f1"}},
		"terminations":  []any{map[string]any{"effective_date": "2026-03-01"}}, "terminated": true,
		"employees": []any{map[string]any{"uuid": "e1", "employee_uuid": "e1", "pay_schedule_uuid": "s1"}}, "contractors": []any{map[string]any{"uuid": "c1"}},
		"departments": []any{map[string]any{"department_uuid": "d1", "pay_schedule_uuid": "s1"}},
		"days":        map[string]any{"2026-01-05": "8.000", "2026-01-06": "4.000"}, "employee": map[string]any{"uuid": "e1", "full_name": "Ann Lee"},
		"pay_period": map[string]any{"start_date": "2026-01-01", "end_date": "2026-01-14", "pay_schedule_uuid": "s1"},
		"payroll":    map[string]any{"payroll_uuid": "p1", "check_date": "2026-01-16", "processed": true},
		"totals":     map[string]any{"gross_pay": "1.00", "amount": "1.00"},
		"balances":   []any{map[string]any{"policy_uuid": "v1", "policy_name": "Vacation", "balance": "40.0"}},
		"employee_compensations": []any{map[string]any{"employee_uuid": "e1", "gross_pay": "1.00",
			"fixed_compensations":  []any{map[string]any{"name": "Bonus", "amount": "1.00"}},
			"hourly_compensations": []any{map[string]any{"name": "Regular", "hours": "8", "amount": "1.00"}},
			"paid_time_off":        []any{map[string]any{"name": "Vacation", "hours": "8"}},
			"deductions":           []any{map[string]any{"name": "401k", "amount": "1.00"}},
			"benefits":             []any{map[string]any{"name": "Dental", "employee_deduction": "1.00", "company_contribution": "2.00"}},
			"taxes":                []any{map[string]any{"name": "FICA", "employer": true, "amount": "1.00"}}}},
	}
	for _, c := range connect.All() {
		if len(c.Streams) == 0 {
			continue
		}
		id := uuid.New()
		in.psql("archive", "INSERT INTO archive.connection (id, member, provider, account, token, sync_at) SELECT '"+id.String()+"', id, '"+c.ID+"', email, '\\x00', NULL FROM archive.member")
		for _, s := range c.Streams {
			if err := a.sys(ctx, func(ctx context.Context) error {
				q := a.st.db(ctx)
				now, err := q.TxNow(ctx)
				if err != nil {
					return err
				}
				return syncSave(ctx, &syncDB{q: q, db: a.st.Q(ctx), id: id, now: now}, s, &connect.Page{Records: []connect.Record{{ID: "x1", Data: all}}}, &syncStream{})
			}); err != nil {
				t.Errorf("%s %s: %v", c.ID, s.Name, err)
			}
			if n := in.psql("archive", "SELECT count(*) FROM archive."+s.Table+" WHERE connection = '"+id.String()+"' AND stream = '"+s.Name+"'"); n != "1" {
				t.Errorf("%s %s wrote %s rows of %s", c.ID, s.Name, n, s.Table)
			}
		}
	}
	for name, tb := range syncSchema {
		if tb.parent == "" {
			continue
		}
		if n := in.psql("archive", "SELECT count(*) FROM archive."+name); n == "0" {
			t.Errorf("no stream wrote a row of %s", name)
		}
	}
}
