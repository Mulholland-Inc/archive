package connect

import "time"

// BILL (Bill.com), a business's payables and receivables, through its v3 API. It is a key connector with a key of
// the server's own: ARCHIVE_BILL is https://<developer key>@gateway.prod.bill.com, the developer key BILL's Partner
// & Developer Success team issues Mulholland as an app partner; with gateway.stage.bill.com and the sandbox's key
// it reads sandbox organizations instead. An owner pastes in their organization's ID and an AP & AR sync token, made
// in BILL at Settings, Sync & Integrations, Tokens, whose name signs in as the username and whose value as the
// password. BILL has no OAuth for its API.
//
// A sync token opens a session that can never move money: BILL refuses it every payment, void, cancel and charge,
// approving bills, and sending or mailing invoices. So Archive only reads, and, when asked, adds, changes, archives
// and restores bills, invoices, vendors and customers; it has no action that pays or sends anything.
//
// Every call needs a session, which POST /v3/login opens with the token and the developer key, and the devKey and
// sessionId headers carry. A session lasts until 48 hours idle, and BILL allows a developer key 200 logins an hour,
// so a session is kept for the account for six hours, or until a 401 says it is gone. BILL allows a developer key
// 20,000 calls an hour and three at once per organization, and past either says "Max number of ...", which waits.
//
// It syncs the chart of accounts, vendors, customers, bills, invoices, payments sent and payments received into the
// ledger tables. Each is listed whole the first time and a week after, 100 a page, which deletes what is gone,
// and in between read newest change first down to the sync before. BILL keeps what is archived, and lists it,
// marked archived. Its webhooks need a subscription per organization, which it does not make yet.
func init() {
	Register(&Connector{
		ID: "bill", Name: "BILL", Integration: true, Env: "ARCHIVE_BILL", Host: "https://gateway.prod.bill.com",
		Info: Info{By: "BILL", Description: "Read bills, invoices, vendors, customers and payments, and add or change bills and invoices when you ask", Category: CatFinance, Icon: IconLedger,
			Scopes: []string{"Read payables, receivables and payments with the organization’s sync token", "Add, change and archive bills, invoices, vendors and customers when you ask; never pay or send"}},
		API: "https://gateway.prod.bill.com/connect/v3",
		Keys: []Key{
			{Name: "organization_id", Label: "Organization ID", Help: "In BILL, Settings, Sync & Integrations, Manage Developer Keys: the organization ID at the bottom of the page, which starts with 008"},
			{Name: "token_name", Label: "Sync token name", Help: "In BILL, Settings, Sync & Integrations, Tokens: make a new token, and enter the name you gave it"},
			{Name: "token", Label: "Sync token", Secret: true, Help: "The token BILL made on the same page"},
		},
		Let: []Step{{As: "session", Req: Req{Method: POST, Read: true, URL: "https://gateway.prod.bill.com/connect/v3/login",
			Body: Obj{"username": "{key/token_name}", "password": "{key/token}", "organizationId": "{key/organization_id}", "devKey": "{client/id}"}}}},
		LetFor: 6 * time.Hour,
		Sign:   map[string]string{"devKey": "{client/id}", "sessionId": "{let/session/sessionId}"},
		Busy:   []string{"Max number of"},
		Who:    &Who{Req: Req{URL: "/login/organizations"}, Names: []Path{"results/organizations/0/name", "results/organizations/0/id"}},
		Actions: map[string]*Action{
			"get": Call[billGet, any](Plan{
				Doc: "Read any of BILL's v3 GET endpoints, with its filters, sort and max, like { path: \"/bills\", query: \"filters=paymentStatus:eq:UNPAID,dueDate:lt:2026-12-01&sort=dueDate:asc&max=100\" }, " +
					"{ path: \"/invoices/<id>\" }, { path: \"/vendors\", query: \"filters=name:sw:\\\"Acme\\\"\" } or { path: \"/payments\" }: its answer, { results, nextPage } for a list " +
					"(pass page=<nextPage>&max=100, without filters or sort, for more). Lists include archived records, with archived: true",
				Steps: []Step{{Req: Req{URL: "{+path}?{+query|}"}}},
			}),
			"create": Call[billNew, map[string]any](Plan{
				Doc: "Add a bill, invoice, vendor or customer with BILL's fields, like { type: \"bills\", fields: { vendorId, invoice: { invoiceNumber, invoiceDate }, dueDate, " +
					"billLineItems: [{ amount, description }] } }, { type: \"invoices\", fields: { customer: { id }, invoiceNumber, invoiceDate, dueDate, invoiceLineItems: [{ description, price, quantity }] } }, " +
					"{ type: \"vendors\", fields: { name, email, address } } or { type: \"customers\", fields: { name, email } }: the record. Nothing is paid or sent",
				Enum:  map[string][]string{"type": billTypes},
				Steps: []Step{billUnsent, {Req: Req{Method: POST, URL: "/{type}", Body: "{fields}"}}},
			}),
			"update": Call[billEdit, map[string]any](Plan{
				Doc:   "Change a bill, invoice, vendor or customer: only the fields given change, and a line item given with its id changes that line. Returns the record",
				Enum:  map[string][]string{"type": billTypes},
				Steps: []Step{billUnsent, {Req: Req{Method: PATCH, URL: "/{type}/{id}", Body: "{fields}"}}},
			}),
			"archive": Call[billOne, map[string]any](Plan{
				Doc:   "Archive a bill, invoice, vendor or customer, which BILL keeps and restore brings back: the record, archived",
				Enum:  map[string][]string{"type": billTypes},
				Steps: []Step{{Req: Req{Method: POST, URL: "/{type}/{id}/archive"}}},
			}),
			"restore": Call[billOne, map[string]any](Plan{
				Doc:   "Restore an archived bill, invoice, vendor or customer: the record",
				Enum:  map[string][]string{"type": billTypes},
				Steps: []Step{{Req: Req{Method: POST, URL: "/{type}/{id}/restore"}}},
			}),
		},
		Sync: []*Sync{
			{Name: "accounts", Gap: time.Second, Req: billList, Items: "results", Updated: "updatedTime", Page: billPage, Change: billChange,
				Vars: Obj{"what": "classifications/chart-of-accounts"},
				Doc:  "The chart of accounts" + billDoc,
				To: Map{Table: "ledger_account",
					Cols: map[string]SQL{"number": "data#>>'{account,number}'", "title": "data->>'name'", "type": "data#>>'{account,type}'", "active": billActive, "edited_at": billEdited},
					Refs: map[string]SQL{"parent": "data->>'parentId'"}}},
			{Name: "vendors", Gap: time.Second, Req: billList, Items: "results", Updated: "updatedTime", Page: billPage, Change: billChange,
				Vars: Obj{"what": "vendors"},
				Doc:  "Every vendor, with its contact details, address and what is owed it" + billDoc,
				To: Map{Table: "ledger_contact",
					Cols: map[string]SQL{"kind": "'vendor'", "title": "data->>'name'", "company": "data#>>'{additionalInfo,companyName}'", "email": "archive.sync_email(data->>'email')", "phone": "data->>'phone'",
						"address": billAddress("address"), "balance": "archive.sync_money(data#>>'{balance,amount}', coalesce(nullif(data->>'billCurrency', ''), 'USD'))", "active": billActive, "edited_at": billEdited},
					People: map[string]SQL{"person": "data->>'email'"}}},
			{Name: "customers", Gap: time.Second, Req: billList, Items: "results", Updated: "updatedTime", Page: billPage, Change: billChange,
				Vars: Obj{"what": "customers"},
				Doc:  "Every customer, with its contact details, billing address and what it owes" + billDoc,
				To: Map{Table: "ledger_contact",
					Cols: map[string]SQL{"kind": "'customer'", "title": "data->>'name'", "company": "data->>'companyName'", "email": "archive.sync_email(data->>'email')", "phone": "data->>'phone'",
						"address": billAddress("billingAddress"), "balance": "archive.sync_money(data#>>'{balance,amount}', coalesce(nullif(data->>'invoiceCurrency', ''), 'USD'))", "active": billActive, "edited_at": billEdited},
					People: map[string]SQL{"person": "data->>'email'"}}},
			// A bill is dated by the vendor's invoice it is for, and its status is where paying it is, or that it is archived.
			{Name: "bills", Gap: time.Second, Req: billList, Items: "results", Updated: "updatedTime", Page: billPage, Change: billChange,
				Vars: Obj{"what": "bills"},
				Doc:  "Every bill, with its vendor, invoice number and date, due date, amounts, payment and approval status, and lines" + billDoc,
				To: Map{Table: "ledger_entry",
					Cols: map[string]SQL{"kind": "'Bill'", "number": "data#>>'{invoice,invoiceNumber}'", "txn_on": "archive.sync_day(data#>>'{invoice,invoiceDate}')", "due_on": "archive.sync_day(data->>'dueDate')",
						"contact_name": "data->>'vendorName'", "total": "archive.sync_money(data->>'amount', " + billCurrency + ")", "balance": "archive.sync_money(data->>'dueAmount', " + billCurrency + ")",
						"memo": "data->>'description'", "status": "CASE WHEN data->>'archived' = 'true' THEN 'ARCHIVED' ELSE concat_ws(', ', nullif(data->>'paymentStatus', ''), 'approval ' || nullif(data->>'approvalStatus', '')) END",
						"edited_at": billEdited},
					Refs: map[string]SQL{"contact": "data->>'vendorId'", "account": "data->>'payFromChartOfAccountId'"},
					Children: []Kids{{Table: "ledger_line", From: "archive.sync_list(data->'billLineItems') WITH ORDINALITY l(x, i)",
						Cols: map[string]SQL{"position": "i", "description": "x->>'description'", "amount": "archive.sync_money(x->>'amount', " + billCurrency + ")",
							"quantity": "archive.sync_num(x->>'quantity')", "unit_price": "archive.sync_num(x->>'price')"},
						Refs: map[string]SQL{"account": "x#>>'{classifications,chartOfAccountId}'", "item": "x#>>'{classifications,itemId}'"}}}}},
			// An invoice's line has a price and a quantity, which make its amount; BILL invoices in US dollars.
			{Name: "invoices", Gap: time.Second, Req: billList, Items: "results", Updated: "updatedTime", Page: billPage, Change: billChange,
				Vars: Obj{"what": "invoices"},
				Doc:  "Every invoice, with its customer, number, dates, amounts, status and lines" + billDoc,
				To: Map{Table: "ledger_entry",
					Cols: map[string]SQL{"kind": "'Invoice'", "number": "data->>'invoiceNumber'", "txn_on": "archive.sync_day(data->>'invoiceDate')", "due_on": "archive.sync_day(data->>'dueDate')",
						"contact_name": "data->>'customerName'", "total": "archive.sync_money(data->>'totalAmount', 'USD')", "balance": "archive.sync_money(data->>'dueAmount', 'USD')",
						"status": "CASE WHEN data->>'archived' = 'true' THEN 'ARCHIVED' ELSE data->>'status' END", "edited_at": billEdited},
					Refs: map[string]SQL{"contact": "data->>'customerId'", "account": "data->>'payToChartOfAccountId'"},
					Children: []Kids{{Table: "ledger_line", From: "archive.sync_list(data->'invoiceLineItems') WITH ORDINALITY l(x, i)",
						Cols: map[string]SQL{"position": "i", "description": "x->>'description'",
							"amount":   "archive.sync_money((archive.sync_num(x->>'price') * coalesce(archive.sync_num(x->>'quantity'), 1))::text, 'USD')",
							"quantity": "archive.sync_num(x->>'quantity')", "unit_price": "archive.sync_num(x->>'price')"},
						Refs: map[string]SQL{"account": "x#>>'{classifications,chartOfAccountId}'", "item": "x#>>'{classifications,itemId}'"}}}}},
			{Name: "payments", Gap: time.Second, Req: billList, Items: "results", Updated: "updatedTime", Page: billPage, Change: billChange,
				Vars: Obj{"what": "payments"},
				Doc:  "Every payment sent to a vendor, through BILL or recorded in it, with its process date, amount, confirmation number and status" + billDoc,
				To: Map{Table: "ledger_entry",
					Cols: map[string]SQL{"kind": "'BillPayment'", "number": "coalesce(nullif(data->>'confirmationNumber', ''), data->>'transactionNumber')", "txn_on": "archive.sync_day(data->>'processDate')",
						"contact_name": "data->>'vendorName'", "total": "archive.sync_money(data->>'amount', 'USD')", "memo": "data->>'description'",
						"status": "coalesce(nullif(data->>'singleStatus', ''), data->>'status')", "edited_at": billEdited},
					Refs: map[string]SQL{"contact": "data->>'vendorId'"}}},
			{Name: "received_payments", Gap: time.Second, Req: billList, Items: "results", Updated: "updatedTime", Page: billPage, Change: billChange,
				Vars: Obj{"what": "receivable-payments"},
				Doc:  "Every payment received from a customer, through BILL or recorded in it, with its date, amount, reference number and status" + billDoc,
				To: Map{Table: "ledger_entry",
					Cols: map[string]SQL{"kind": "'Payment'", "number": "data->>'referenceNumber'", "txn_on": "archive.sync_day(data->>'paymentDate')",
						"total": "archive.sync_money(data->>'amount', 'USD')", "memo": "data->>'description'", "status": "data->>'status'", "edited_at": billEdited},
					Refs: map[string]SQL{"contact": "data->>'customerId'"}}},
		},
	})
}

const (
	billDoc = ", archived ones too, by its id: listed whole the first time and a week after, 100 a page, which deletes those gone, " +
		"and in between those changed since the sync before, newest first."
	billActive   SQL = "data->>'archived' IS DISTINCT FROM 'true'"
	billEdited   SQL = "archive.sync_at(data->>'updatedTime')"
	billCurrency     = "coalesce(nullif(data->>'currency', ''), 'USD')"
)

var (
	billTypes = []string{"bills", "invoices", "vendors", "customers"}
	// nextPage names the page after, of the same search.
	billList   = Req{URL: "/{+what}", Query: Q{"max": "100", "page": "{page}"}}
	billPage   = Cursor{At: "nextPage"}
	billChange = NewestFirst{Query: Q{"sort": "updatedTime:desc"}}
	// billUnsent refuses to have BILL email an invoice, which a sync token may not do anyway.
	billUnsent = Step{If: "fields/processingOptions/sendEmail", Fail: "Archive does not send invoices; send it in BILL"}
)

// billAddress is the address at key on one line.
func billAddress(key string) SQL {
	a := "data->'" + key + "'"
	return SQL("concat_ws(', ', nullif(btrim(" + a + "->>'line1'), ''), nullif(btrim(" + a + "->>'line2'), ''), nullif(btrim(" + a + "->>'city'), ''), " +
		"nullif(btrim(" + a + "->>'stateOrProvince'), ''), nullif(btrim(" + a + "->>'zipOrPostalCode'), ''), nullif(btrim(" + a + "->>'country'), ''))")
}

type (
	billGet struct {
		Path  string `json:"path" jsonschema:"The endpoint after /v3, like /bills, /invoices, /vendors, /customers, /payments, /receivable-payments, /vendor-credits or /classifications/chart-of-accounts"`
		Query string `json:"query,omitempty" jsonschema:"Its query, like filters=vendorId:eq:<id>&sort=dueDate:desc&max=100, or page=<nextPage>&max=100"`
	}
	billNew struct {
		Type   string         `json:"type" jsonschema:"bills, invoices, vendors or customers"`
		Fields map[string]any `json:"fields" jsonschema:"Its fields as BILL names them, like vendorId, invoice, dueDate and billLineItems for a bill"`
	}
	billEdit struct {
		Type   string         `json:"type" jsonschema:"bills, invoices, vendors or customers"`
		ID     string         `json:"id" jsonschema:"The record's id"`
		Fields map[string]any `json:"fields" jsonschema:"The fields to change, as BILL names them"`
	}
	billOne struct {
		Type string `json:"type" jsonschema:"bills, invoices, vendors or customers"`
		ID   string `json:"id" jsonschema:"The record's id"`
	}
)
