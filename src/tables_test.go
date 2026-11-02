package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Mulholland-Inc/archive/connect"
	"github.com/Mulholland-Inc/archive/generated"
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

// A synced Gmail message, Notion page and Gusto company, employee, contractor and payroll land as rows of mail,
// docs and payroll, and the people they name are one person by their address, whichever connection names
// them. Each member sees the rows of the records they see: their own mail, and the archive's Gusto while it is
// not narrowed to a group they are not in; a person only through a row they see. A record deleted there, one a
// full listing no longer lists, and a connection disconnected take their rows with them, and the people only
// they named. Records synced before their stream had tables are written into them once.
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
			return a.syncSave(ctx, q, conn, streamOf(t, provider, stream), page, &syncStream{}, now)
		}); err != nil {
			t.Fatalf("saving %s %s: %v", provider, stream, err)
		}
	}
	msg := func(id, from, to string) connect.Record {
		return connect.Record{ID: id, Data: map[string]any{"id": id, "threadId": "t-" + id, "labelIds": []string{"INBOX"}, "snippet": "About pay",
			"subject": "Pay " + id, "from": from, "to": to, "cc": "", "date": "Mon, 02 Nov 2026 10:00:00 +0100", "text": "Hello",
			"attachments": []map[string]any{{"filename": "a.pdf"}}}}
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
		"first_name": "Ann", "last_name": "Lee", "email": "ann@home.example", "work_email": "ann.lee@acme.example", "terminated": false,
		"jobs": []any{map[string]any{"title": "Engineer", "rate": "120000.00", "payment_unit": "Year", "hire_date": "2019-03-01", "primary": true}}}}}})
	save(gusto, "gusto", "company", &connect.Page{Records: []connect.Record{{ID: "co1", Data: map[string]any{"uuid": "co1", "name": "Acme", "ein": "123456789",
		"entity_type": "LLC"}}}})
	save(gusto, "gusto", "payrolls", &connect.Page{Records: []connect.Record{{ID: "p1", Data: map[string]any{"payroll_uuid": "p1", "company_uuid": "co1",
		"processed": true, "check_date": "2026-10-30", "pay_period": map[string]any{"start_date": "2026-10-12", "end_date": "2026-10-25"},
		"totals": map[string]any{"gross_pay": "5000.00", "net_pay": "3600.10", "employee_taxes": "900.00", "employer_taxes": "400.00"},
		"employee_compensations": []any{map[string]any{"employee_uuid": "e1", "gross_pay": "5000.00", "net_pay": "3600.10", "payment_method": "Direct Deposit",
			"deductions": []any{map[string]any{"amount": "499.90"}}, "taxes": []any{map[string]any{"employer": false, "amount": "900.00"}}}}}}}})
	save(gusto, "gusto", "contractor_payments", &connect.Page{Records: []connect.Record{{ID: "cp1", Data: map[string]any{"uuid": "cp1", "contractor_uuid": "c1",
		"date": "2026-10-15", "wage_total": "800.00", "reimbursement": "20.00", "payment_method": "Check"}}}})

	want := func(what, sql, want string) {
		t.Helper()
		if got := in.psql("archive", sql); got != want {
			t.Errorf("%s: %q, want %q", what, got, want)
		}
	}
	want("the mail", "SELECT string_agg(subject || ' ' || thread || ' ' || sent_at || ' ' || labels::text || ' ' || attachments, ', ' ORDER BY subject) FROM archive.mail_message",
		"Pay m1 t-m1 2026-11-02 09:00:00+00 {INBOX} 1, Pay m2 t-m2 2026-11-02 09:00:00+00 {INBOX} 1")
	want("who the mail names", `SELECT string_agg(m.subject || ' ' || p.role || ' ' || k.email || coalesce(' ' || (p.name)."full", ''), ', ' ORDER BY m.subject, p.role, k.email)
		FROM archive.mail_party p JOIN archive.mail_message m ON m.id = p.message JOIN archive.known_person k ON k.id = p.person`,
		"Pay m1 from ann.lee@acme.example Ann Lee, Pay m1 to alice@example.com, Pay m1 to zed@elsewhere.example Zed, Pay m2 from bob@example.com, Pay m2 to carl@example.net")
	want("the pages", "SELECT string_agg(kind || ' ' || title || ' ' || url || ' ' || icon, ', ' ORDER BY title) FROM archive.doc_page",
		"page Plans https://www.notion.so/Plans-n1 📝, database Tasks https://www.notion.so/Tasks-n2")
	want("the company, linked to its EIN", "SELECT c.name || ' ' || c.entity_type || ' ' || o.ein FROM archive.payroll_company c JOIN archive.known_org o ON o.id = c.org", "Acme LLC 12-3456789")
	want("the employee, linked to their company written after them, and their pay", `SELECT w.kind || ' ' || (w.name)."full" || ' ' || w.email || ' ' || w.personal_email || ' ' || c.name || ' ' ||
		w.title || ' ' || w.hired_on || ' ' || w.active || ' ' || w.pay_rate::text || ' ' || w.pay_per FROM archive.payroll_worker w JOIN archive.payroll_company c ON c.id = w.company`,
		"employee Ann Lee ann.lee@acme.example ann@home.example Acme Engineer 2019-03-01 true (12000000,USD) Year")
	want("the payroll", `SELECT r.check_date || ' ' || r.period_start || ' ' || r.processed || ' ' || r.gross_pay::text || ' ' || r.net_pay::text || ' ' || r.employer_taxes::text || ' ' || c.name
		FROM archive.payroll_run r JOIN archive.payroll_company c ON c.id = r.company`, "2026-10-30 2026-10-12 true (500000,USD) (360010,USD) (40000,USD) Acme")
	want("the pays", `SELECT string_agg(p.worker_source || ' ' || p.paid_on || ' ' || p.gross_pay::text || ' ' || p.net_pay::text || ' ' || coalesce(p.deductions::text, '-') || ' ' ||
		(p.run IS NOT NULL) || ' ' || (p.worker IS NOT NULL) || ' ' || p.payment_method, ', ' ORDER BY p.worker_source) FROM archive.payroll_pay p`,
		"c1 2026-10-15 (80000,USD) (82000,USD) - false false Check, e1 2026-10-30 (500000,USD) (360010,USD) (49990,USD) true true Direct Deposit")
	want("one person for an address two connections name", "SELECT count(*) FROM archive.known_person WHERE email = 'ann.lee@acme.example'", "1")
	want("the person of the employee is the sender of the mail", `SELECT count(DISTINCT k.id) FROM archive.known_person k JOIN archive.payroll_worker w ON w.person = k.id
		JOIN archive.mail_party p ON p.person = k.id`, "1")

	sees := func(tok, sql string) string {
		t.Helper()
		_, out := e2eRun(t, srv.url+"/api", tok, "const rows = await archive.query(`"+sql+"`); return rows.map((r) => Object.values(r).join(' ')).join(', ')")
		var s string
		json.Unmarshal(out.Result, &s)
		if out.Error != "" {
			t.Fatalf("%s: %s", sql, out.Error)
		}
		return s
	}
	people := "SELECT email FROM known_person ORDER BY email"
	if got := sees(alice, people); got != "alice@example.com, ann.lee@acme.example, zed@elsewhere.example" {
		t.Errorf("alice sees the people %q", got)
	}
	if got := sees(bob, people); got != "ann.lee@acme.example, bob@example.com, carl@example.net" {
		t.Errorf("bob sees the people %q", got)
	}
	if got := sees(bob, "SELECT subject FROM mail_message") + " | " + sees(bob, "SELECT title FROM doc_page ORDER BY title"); got != "Pay m2 | " {
		t.Errorf("bob sees the mail and pages %q", got)
	}
	if got := sees(bob, "SELECT (SELECT count(*) FROM payroll_worker) || ' ' || (SELECT count(*) FROM payroll_run) || ' ' || (SELECT count(*) FROM payroll_pay) || ' ' || (SELECT count(*) FROM known_org)"); got != "1 1 2 1" {
		t.Errorf("bob sees of the archive's Gusto %q", got)
	}
	if st, _ := call(t, "PUT", srv.url+"/connect/"+gusto.String()+"/access", alice, map[string]string{"access": "hr"}, nil); st != 204 {
		t.Fatalf("narrowing Gusto to hr: %d", st)
	}
	if got := sees(bob, "SELECT (SELECT count(*) FROM payroll_worker) || ' ' || (SELECT count(*) FROM payroll_pay) || ' ' || (SELECT count(*) FROM known_org)"); got != "0 0 0" {
		t.Errorf("bob sees of the archive's Gusto narrowed to hr %q", got)
	}
	if got := sees(bob, people); got != "bob@example.com, carl@example.net" {
		t.Errorf("bob sees the people once Gusto is narrowed to hr: %q", got)
	}
	if got := sees(alice, "SELECT count(*) FROM payroll_worker"); got != "1" {
		t.Errorf("the owner sees %s workers of Gusto narrowed to hr", got)
	}

	// A message deleted there takes its rows, and the people only it named.
	save(mailA, "gmail", "messages", &connect.Page{Records: []connect.Record{{ID: "m1", Deleted: true}}})
	want("after a message is deleted", "SELECT (SELECT count(*) FROM archive.mail_message) || ' ' || (SELECT string_agg(email, ' ' ORDER BY email) FROM archive.known_person)",
		"1 ann.lee@acme.example bob@example.com carl@example.net")

	// A full listing that lists a page as the same keeps it, and deletes the one it no longer lists.
	in.psql("archive", "UPDATE archive.record SET data = data || '{\"title\": \"Plans, kept\"}' WHERE source_id = 'n1'")
	save(notion, "notion", "private_pages", &connect.Page{Full: true, Records: []connect.Record{{ID: "n1", Same: true}}})
	want("after a full listing", "SELECT string_agg(r.source_id || ' ' || r.deleted || ' ' || (r.data->>'title') || ' ' || (d.id IS NOT NULL), ', ' ORDER BY r.source_id) FROM archive.record r LEFT JOIN archive.doc_page d ON d.record = r.id WHERE r.stream = 'private_pages'",
		"n1 false Plans, kept true, n2 true Tasks false")

	// Disconnecting Gusto takes every row of it, and the organization and people only it knew.
	if st, _ := call(t, "DELETE", srv.url+"/connect/"+gusto.String(), alice, nil, nil); st != 204 {
		t.Fatalf("disconnecting Gusto: %d", st)
	}
	want("after Gusto is disconnected", `SELECT (SELECT count(*) FROM archive.payroll_company) + (SELECT count(*) FROM archive.payroll_worker) + (SELECT count(*) FROM archive.payroll_run)
		+ (SELECT count(*) FROM archive.payroll_pay) + (SELECT count(*) FROM archive.known_org) || ' ' || (SELECT string_agg(email, ' ' ORDER BY email) FROM archive.known_person)`,
		"0 bob@example.com carl@example.net")

	// A message synced before Gmail had tables is written into them on the next page.
	in.psql("archive", `INSERT INTO archive.record (connection, stream, source_id, data) VALUES ('`+mailA.String()+`', 'messages', 'm0',
		'{"id": "m0", "subject": "Old", "from": "old@example.com", "date": "Fri, 02 Jan 2026 10:00:00 +0000"}');
		UPDATE archive.connection SET cursor = '{"messages": {"rows": 0}}' WHERE id = '`+mailA.String()+`'`)
	var c generated.ArchiveConnection
	if err := a.sys(ctx, func(ctx context.Context) (err error) {
		c, err = a.st.db(ctx).Connection(ctx, mailA)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.syncFill(ctx, c, connect.Get("gmail").Streams, time.Minute); err != nil {
		t.Fatal(err)
	}
	want("an old message", "SELECT string_agg(m.subject || ' ' || k.email, ', ') FROM archive.mail_message m JOIN archive.mail_party p ON p.message = m.id JOIN archive.known_person k ON k.id = p.person WHERE m.subject = 'Old'",
		"Old old@example.com")
	want("the stream's cursor once it is written", "SELECT cursor->'messages'->>'rows' FROM archive.connection WHERE id = '"+mailA.String()+"'", "1")
	if strings.Contains(sees(bob, "SELECT subject FROM mail_message"), "Old") {
		t.Error("bob sees alice's old message")
	}
}

// A stream pulled every so often is not due again until that long after it last caught up, unless a listing
// of it is under way; others are due on every ask.
func TestAStreamPulledEverySoOftenWaits(t *testing.T) {
	now := time.Now()
	asked, before, hourAgo, dayAgo := now, now.Add(-time.Minute), now.Add(-time.Hour), now.Add(-25*time.Hour)
	streams := []*connect.Stream{{Name: "lists"}, {Name: "setup", Every: 24 * time.Hour}}
	st := syncState{"lists": {Done: &asked}, "setup": {Done: &before, Last: &hourAgo}}
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
}
