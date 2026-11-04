package connect

import (
	"time"

	"golang.org/x/oauth2"
)

// QuickBooks Online, through Intuit's accounting API. It reads ARCHIVE_QUICKBOOKS, an app's production keys
// from developer.intuit.com, whose redirect URIs list <API>/connect/quickbooks/callback; ?sandbox=1 on the
// variable, with the app's development keys, reads sandbox companies instead. Intuit has no read-only scope,
// so com.intuit.quickbooks.accounting is all of it, and the actions only read. The callback names the
// company, realmId, which every call needs; a person connects one company at a time, and connects again for
// another. Intuit replaces the refresh token about once a day.
//
// It syncs the books into the ledger tables: the chart of accounts, customers, vendors, employees, items, and
// every kind of transaction with its lines, a stream each. Each is listed whole the first time, 1000 a page,
// the inactive too; after that Intuit's change data capture (CDC) says what changed since the last sync began,
// deleted too, as long as that was within 30 days, and a stream whose last sync is older, or which changed
// more than CDC answers at once, is listed whole again. Pages overlap by 50, so a record deleted while a listing
// runs, which moves the ones after it a place back, skips none of them; those read twice are written once.
//
// Its pushes are the Intuit app's webhooks, set up once for each of its environments: its endpoint
// <API>/push/quickbooks, the entities QuickBooks streams, and its verifier token the hook option of
// ARCHIVE_QUICKBOOKS. intuit-signature is the base64 HMAC-SHA256 of the body with the verifier token. A push is
// an array of CloudEvents, each with intuitaccountid, the realmId of the company it is for; one of the format
// before, { eventNotifications: [{ realmId }] }, is read too.
func init() {
	Register(&Connector{
		ID: "quickbooks", Name: "QuickBooks", Integration: true, Options: "?sandbox=1 for development keys, &hook=<webhooks verifier token> for its pushes",
		Info: Info{By: "Intuit", Description: "Read your company’s invoices, customers and books", Category: CatFinance, Icon: IconLedger, Scopes: []string{"Read one company’s accounting"}},
		API:  "https://{+host}.api.intuit.com/v3/company/{meta/realmId}",
		Let: []Step{
			{As: "host", In: "quickbooks"},
			{If: "option/sandbox", As: "host", In: "sandbox-quickbooks"},
		},
		// Intuit tags every answer with this, which its support asks for.
		Trace: "intuit_tid",
		Endpoint: oauth2.Endpoint{AuthURL: "https://appcenter.intuit.com/connect/oauth2", TokenURL: "https://oauth.platform.intuit.com/oauth2/v1/tokens/bearer",
			AuthStyle: oauth2.AuthStyleInHeader},
		RevokeBy: &Req{Method: POST, URL: "https://developer.api.intuit.com/v2/oauth2/tokens/revoke", Header: map[string][]string{"Authorization": {"{basic}"}}, Body: Obj{"token": "{refresh}"}},
		Scopes:   []string{"com.intuit.quickbooks.accounting"},
		Keep:     []string{"realmId"},
		Who:      &Who{Req: Req{URL: "/companyinfo/{meta/realmId}", Query: qboV}, Names: []Path{"CompanyInfo/CompanyName"}},
		Actions: map[string]*Action{
			"query": Call[qboQuery, any](Plan{
				Doc: "Run a QuickBooks query, its SQL-like language over one entity, like { query: \"SELECT * FROM Bill WHERE Balance > '0' MAXRESULTS 100\" }: " +
					"{ QueryResponse: { <Entity>: [...], startPosition, maxResults } }. Entities include Account, Bill, Customer, Estimate, Invoice, Item, " +
					"Payment, Purchase, Vendor; no joins, and at most 1000 rows, paged with STARTPOSITION",
				Steps: []Step{{Req: Req{URL: "/query", Query: Q{"query": "{query}", "minorversion": "75"}}}},
			}),
			"company": Call[struct{}, any](Plan{
				Doc:   "The company connected: its CompanyInfo, with CompanyName, LegalName, CompanyAddr, Country, FiscalYearStartMonth and more",
				Steps: []Step{{As: "got", Req: Req{URL: "/companyinfo/{meta/realmId}", Query: qboV}}},
				Out:   "{got/CompanyInfo}",
			}),
			"invoices": Call[qboInvoices, []map[string]any](Plan{
				Doc:   "Invoices, newest first, maybe only a customer's or only unpaid ones: [Invoice] with DocNumber, TxnDate, DueDate, CustomerRef, TotalAmt, Balance, Line",
				Clamp: map[string][2]int{"limit": {100, 1000}},
				Steps: []Step{
					{If: "customer", As: "a", In: "CustomerRef = '{'customer}'"},
					{If: "unpaid", As: "b", In: "Balance > '0'"},
					{If: "since", As: "c", In: "TxnDate >= '{'since}'"},
					{As: "all", In: []any{"{a}", "{b}", "{c}"}},
					{If: "all", As: "where", In: " WHERE {all, AND }"},
					{As: "got", Req: Req{URL: "/query", Query: Q{"query": "SELECT * FROM Invoice{where|} ORDERBY TxnDate DESC MAXRESULTS {limit}", "minorversion": "75"}}},
				},
				Out: "{got/QueryResponse/Invoice|[]}",
			}),
			"customers": Call[qboCustomers, []map[string]any](Plan{
				Doc:   "Customers by name, maybe only those whose name starts with some text: [Customer] with Id, DisplayName, CompanyName, PrimaryEmailAddr, Balance, Active",
				Clamp: map[string][2]int{"limit": {100, 1000}},
				Steps: []Step{
					{If: "name", As: "where", In: " WHERE DisplayName LIKE '{'name}%'"},
					{As: "got", Req: Req{URL: "/query", Query: Q{"query": "SELECT * FROM Customer{where|} ORDERBY DisplayName MAXRESULTS {limit}", "minorversion": "75"}}},
				},
				Out: "{got/QueryResponse/Customer|[]}",
			}),
		},
		Listen: &Listen{Key: "{meta/realmId}", Keys: []Path{"*/intuitaccountid", "eventNotifications/*/realmId"},
			Signs: []Sign{{Header: "intuit-signature", Hook: true, Of: "{body}"}}},
		Sync: []*Sync{
			{Name: "accounts", Gap: time.Second, Req: qboList, Items: qboItems, Absent: "QueryResponse", ID: "Id", Updated: qboEdited, Page: qboPage, Change: qboChange, Set: qboKind, Gone: qboGone, Deny: qboDeny,
				Vars: Obj{"entity": "Account", "active": qboActive},
				Doc:  "The chart of accounts" + qboDoc,
				To: Map{Table: "ledger_account",
					Cols: map[string]SQL{"number": "data->>'AcctNum'", "title": "data->>'Name'", "full_name": "data->>'FullyQualifiedName'", "type": "data->>'AccountType'", "subtype": "data->>'AccountSubType'",
						"class": "data->>'Classification'", "balance": "archive.sync_money(data->>'CurrentBalanceWithSubAccounts', " + qboCurrency + ")", "active": "archive.sync_bool(data->'Active')",
						"edited_at": qboEditedAt},
					Refs: map[string]SQL{"parent": "data#>>'{ParentRef,value}'"}}},
			{Name: "customers", Gap: time.Second, Req: qboList, Items: qboItems, Absent: "QueryResponse", Key: qboKeyed, Updated: qboEdited, Page: qboPage, Change: qboChange, Set: qboKind, Gone: qboGone, Deny: qboDeny, To: qboContact,
				Vars: Obj{"entity": "Customer", "active": qboActive}, Doc: "Every customer" + qboDoc},
			{Name: "vendors", Gap: time.Second, Req: qboList, Items: qboItems, Absent: "QueryResponse", Key: qboKeyed, Updated: qboEdited, Page: qboPage, Change: qboChange, Set: qboKind, Gone: qboGone, Deny: qboDeny, To: qboContact,
				Vars: Obj{"entity": "Vendor", "active": qboActive}, Doc: "Every vendor" + qboDoc},
			{Name: "employees", Gap: time.Second, Req: qboList, Items: qboItems, Absent: "QueryResponse", Key: qboKeyed, Updated: qboEdited, Page: qboPage, Change: qboChange, Set: qboKind, Gone: qboGone, Deny: qboDeny, To: qboContact,
				Vars: Obj{"entity": "Employee", "active": qboActive}, Doc: "Every employee" + qboDoc},
			{Name: "items", Gap: time.Second, Req: qboList, Items: qboItems, Absent: "QueryResponse", ID: "Id", Updated: qboEdited, Page: qboPage, Change: qboChange, Set: qboKind, Gone: qboGone, Deny: qboDeny,
				Vars: Obj{"entity": "Item", "active": qboActive},
				Doc:  "Every product and service" + qboDoc,
				To: Map{Table: "ledger_item",
					Cols: map[string]SQL{"title": "data->>'Name'", "sku": "data->>'Sku'", "type": "data->>'Type'", "description": "data->>'Description'", "price": "archive.sync_money(data->>'UnitPrice', " + qboCurrency + ")",
						"cost": "archive.sync_money(data->>'PurchaseCost', " + qboCurrency + ")", "quantity": "archive.sync_num(data->>'QtyOnHand')", "active": "archive.sync_bool(data->'Active')", "edited_at": qboEditedAt},
					Refs: map[string]SQL{"income_account": "data#>>'{IncomeAccountRef,value}'", "expense_account": "data#>>'{ExpenseAccountRef,value}'"}}},
			{Name: "invoices", Gap: time.Second, Req: qboList, Items: qboItems, Absent: "QueryResponse", ID: "Id", Updated: qboEdited, Page: qboPage, Change: qboChange, Set: qboKind, Gone: qboGone, Deny: qboDeny, To: qboEntry,
				Vars: Obj{"entity": "Invoice"}, Doc: "Every invoice" + qboDoc},
			{Name: "payments", Gap: time.Second, Req: qboList, Items: qboItems, Absent: "QueryResponse", ID: "Id", Updated: qboEdited, Page: qboPage, Change: qboChange, Set: qboKind, Gone: qboGone, Deny: qboDeny, To: qboEntry,
				Vars: Obj{"entity": "Payment"}, Doc: "Every payment a customer made" + qboDoc},
			{Name: "sales_receipts", Gap: time.Second, Req: qboList, Items: qboItems, Absent: "QueryResponse", ID: "Id", Updated: qboEdited, Page: qboPage, Change: qboChange, Set: qboKind, Gone: qboGone, Deny: qboDeny, To: qboEntry,
				Vars: Obj{"entity": "SalesReceipt"}, Doc: "Every sales receipt" + qboDoc},
			{Name: "credit_memos", Gap: time.Second, Req: qboList, Items: qboItems, Absent: "QueryResponse", ID: "Id", Updated: qboEdited, Page: qboPage, Change: qboChange, Set: qboKind, Gone: qboGone, Deny: qboDeny, To: qboEntry,
				Vars: Obj{"entity": "CreditMemo"}, Doc: "Every credit memo" + qboDoc},
			{Name: "refund_receipts", Gap: time.Second, Req: qboList, Items: qboItems, Absent: "QueryResponse", ID: "Id", Updated: qboEdited, Page: qboPage, Change: qboChange, Set: qboKind, Gone: qboGone, Deny: qboDeny, To: qboEntry,
				Vars: Obj{"entity": "RefundReceipt"}, Doc: "Every refund receipt" + qboDoc},
			{Name: "estimates", Gap: time.Second, Req: qboList, Items: qboItems, Absent: "QueryResponse", ID: "Id", Updated: qboEdited, Page: qboPage, Change: qboChange, Set: qboKind, Gone: qboGone, Deny: qboDeny, To: qboEntry,
				Vars: Obj{"entity": "Estimate"}, Doc: "Every estimate" + qboDoc},
			{Name: "bills", Gap: time.Second, Req: qboList, Items: qboItems, Absent: "QueryResponse", ID: "Id", Updated: qboEdited, Page: qboPage, Change: qboChange, Set: qboKind, Gone: qboGone, Deny: qboDeny, To: qboEntry,
				Vars: Obj{"entity": "Bill"}, Doc: "Every bill" + qboDoc},
			{Name: "bill_payments", Gap: time.Second, Req: qboList, Items: qboItems, Absent: "QueryResponse", ID: "Id", Updated: qboEdited, Page: qboPage, Change: qboChange, Set: qboKind, Gone: qboGone, Deny: qboDeny, To: qboEntry,
				Vars: Obj{"entity": "BillPayment"}, Doc: "Every bill payment" + qboDoc},
			{Name: "vendor_credits", Gap: time.Second, Req: qboList, Items: qboItems, Absent: "QueryResponse", ID: "Id", Updated: qboEdited, Page: qboPage, Change: qboChange, Set: qboKind, Gone: qboGone, Deny: qboDeny, To: qboEntry,
				Vars: Obj{"entity": "VendorCredit"}, Doc: "Every vendor credit" + qboDoc},
			{Name: "purchases", Gap: time.Second, Req: qboList, Items: qboItems, Absent: "QueryResponse", ID: "Id", Updated: qboEdited, Page: qboPage, Change: qboChange, Set: qboKind, Gone: qboGone, Deny: qboDeny, To: qboEntry,
				Vars: Obj{"entity": "Purchase"}, Doc: "Every expense, check and credit card charge" + qboDoc},
			{Name: "purchase_orders", Gap: time.Second, Req: qboList, Items: qboItems, Absent: "QueryResponse", ID: "Id", Updated: qboEdited, Page: qboPage, Change: qboChange, Set: qboKind, Gone: qboGone, Deny: qboDeny, To: qboEntry,
				Vars: Obj{"entity": "PurchaseOrder"}, Doc: "Every purchase order" + qboDoc},
			{Name: "deposits", Gap: time.Second, Req: qboList, Items: qboItems, Absent: "QueryResponse", ID: "Id", Updated: qboEdited, Page: qboPage, Change: qboChange, Set: qboKind, Gone: qboGone, Deny: qboDeny, To: qboEntry,
				Vars: Obj{"entity": "Deposit"}, Doc: "Every bank deposit" + qboDoc},
			{Name: "transfers", Gap: time.Second, Req: qboList, Items: qboItems, Absent: "QueryResponse", ID: "Id", Updated: qboEdited, Page: qboPage, Change: qboChange, Set: qboKind, Gone: qboGone, Deny: qboDeny, To: qboEntry,
				Vars: Obj{"entity": "Transfer"}, Doc: "Every transfer between accounts" + qboDoc},
			{Name: "journal_entries", Gap: time.Second, Req: qboList, Items: qboItems, Absent: "QueryResponse", ID: "Id", Updated: qboEdited, Page: qboPage, Change: qboChange, Set: qboKind, Gone: qboGone, Deny: qboDeny, To: qboEntry,
				Vars: Obj{"entity": "JournalEntry"}, Doc: "Every journal entry" + qboDoc},
		},
	})
}

const (
	qboDoc = ", as a query answers, the inactive too: listed whole the first time, 1000 a page, and then what " +
		"CDC says changed since the sync before, deleted too, or listed whole again when that was over 29 days before."
	qboItems  Path = "QueryResponse/{entity}"
	qboEdited Path = "MetaData/LastUpdatedTime"
	// A query lists only the active of these unless asked for the inactive too.
	qboActive = " WHERE Active IN (true, false)"
	// A company with one currency names none: US dollars.
	qboCurrency     = "coalesce(nullif(data#>>'{CurrencyRef,value}', ''), 'USD')"
	qboEditedAt SQL = "archive.sync_at(data#>>'{MetaData,LastUpdatedTime}')"
)

var (
	qboV = Q{"minorversion": "75"}
	// Pages are in the order of ids, so a record made while the listing runs comes last; a company that does not sort an
	// entity by id lists it in its own order.
	qboList = Req{URL: "/query", Query: Q{"minorversion": "75", "query": "SELECT * FROM {entity}{active|} ORDERBY Id STARTPOSITION {page} MAXRESULTS 1000"},
		Or: &Req{URL: "/query", Query: Q{"minorversion": "75", "query": "SELECT * FROM {entity}{active|} STARTPOSITION {page} MAXRESULTS 1000"}}}
	qboPage   = Offset{From: 1, Size: 1000, Step: 950}
	qboChange = SinceTime{Req: Req{URL: "/cdc", Query: Q{"entities": "{entity}", "changedSince": "{since}", "minorversion": "75"}},
		Items: "CDCResponse/*/QueryResponse/*/{entity}/*", Cap: 1000, Lasts: true, Within: 29 * 24 * time.Hour}
	qboKind = Obj{"kind": "{entity}"}
	qboGone = []Test{{At: "status", Is: "Deleted"}}
	// A 400 is an entity the company's plan does not have, like purchase orders without QuickBooks Plus.
	qboDeny = []Deny{{Status: []int{400}, Why: "QuickBooks does not list {entity} for this company: {message}"}}
	// A customer's, vendor's or employee's id is only theirs, kept with the entity's name before it, as they share a table.
	qboKeyed = []Reads{{Of: "{entity}:{Id}"}}
	// A customer, vendor or employee as a contact, who is the person of its email, with its billing address, or an
	// employee's own, on one line.
	qboContact = Map{Table: "ledger_contact",
		Cols: map[string]SQL{"kind": "lower(data->>'kind')", "title": "data->>'DisplayName'", "company": "data->>'CompanyName'", "email": "archive.sync_email(data#>>'{PrimaryEmailAddr,Address}')",
			"phone": "data#>>'{PrimaryPhone,FreeFormNumber}'",
			"address": `(SELECT concat_ws(', ', nullif(btrim(a->>'Line1'), ''), nullif(btrim(a->>'Line2'), ''), nullif(btrim(a->>'Line3'), ''), nullif(btrim(a->>'City'), ''),
				nullif(btrim(a->>'CountrySubDivisionCode'), ''), nullif(btrim(a->>'PostalCode'), ''), nullif(btrim(a->>'Country'), ''))
				FROM (SELECT coalesce(nullif(data->'BillAddr', '{}'::jsonb), data->'PrimaryAddr') a) x)`,
			"balance": "archive.sync_money(data->>'Balance', " + qboCurrency + ")", "active": "archive.sync_bool(data->'Active')", "edited_at": qboEditedAt},
		People: map[string]SQL{"person": "data#>>'{PrimaryEmailAddr,Address}'"}}
	// A transaction as an entry of the books, with its lines but the subtotals, which add up others. Who it is with is
	// its customer, its vendor, or the entity it names, which says which; the account it pays from or into is the first
	// it names of those that say so.
	qboEntry = Map{Table: "ledger_entry",
		Cols: map[string]SQL{"kind": "data->>'kind'", "number": "data->>'DocNumber'", "txn_on": "archive.sync_day(data->>'TxnDate')", "due_on": "archive.sync_day(data->>'DueDate')",
			"contact_name": `CASE WHEN nullif(data#>>'{CustomerRef,value}', '') IS NOT NULL THEN data#>>'{CustomerRef,name}' WHEN nullif(data#>>'{VendorRef,value}', '') IS NOT NULL THEN data#>>'{VendorRef,name}' ELSE data#>>'{EntityRef,name}' END`,
			"total":        "archive.sync_money(coalesce(data->>'TotalAmt', data->>'Amount'), " + qboCurrency + ")", "balance": "archive.sync_money(data->>'Balance', " + qboCurrency + ")",
			"memo":      `concat_ws(E'\n\n', nullif(btrim(data->>'PrivateNote'), ''), nullif(btrim(data#>>'{CustomerMemo,value}'), ''), nullif(btrim(data->>'Memo'), ''))`,
			"status":    "coalesce(nullif(data->>'TxnStatus', ''), data->>'POStatus')",
			"edited_at": qboEditedAt},
		Refs: map[string]SQL{
			"contact": `coalesce('Customer:' || nullif(data#>>'{CustomerRef,value}', ''), 'Vendor:' || nullif(data#>>'{VendorRef,value}', ''), coalesce(nullif(data#>>'{EntityRef,type}', ''), 'Customer') || ':' || nullif(data#>>'{EntityRef,value}', ''))`,
			"account": `coalesce(nullif(data#>>'{DepositToAccountRef,value}', ''), nullif(data#>>'{AccountRef,value}', ''), nullif(data#>>'{CheckPayment,BankAccountRef,value}', ''),
				nullif(data#>>'{CreditCardPayment,CCAccountRef,value}', ''), nullif(data#>>'{FromAccountRef,value}', ''), nullif(data#>>'{APAccountRef,value}', ''), data#>>'{ARAccountRef,value}')`},
		Children: []Kids{{Table: "ledger_line", From: "jsonb_array_elements(data->'Line') WITH ORDINALITY l(x, i) WHERE x->>'DetailType' IS DISTINCT FROM 'SubTotalLineDetail'",
			Cols: map[string]SQL{"position": "coalesce(archive.sync_num(x->>'LineNum'), i)", "kind": "x->>'DetailType'", "description": "x->>'Description'",
				"amount":     "archive.sync_money(x->>'Amount', " + qboCurrency + ")",
				"posting":    "x->(x->>'DetailType')->>'PostingType'",
				"quantity":   "archive.sync_num(x->(x->>'DetailType')->>'Qty')",
				"unit_price": "archive.sync_num(x->(x->>'DetailType')->>'UnitPrice')"},
			Refs: map[string]SQL{"account": "x->(x->>'DetailType')#>>'{AccountRef,value}'", "item": "x->(x->>'DetailType')#>>'{ItemRef,value}'"}}}}
)

type (
	qboQuery struct {
		Query string `json:"query" jsonschema:"The query, like SELECT * FROM Invoice WHERE TxnDate >= '2026-01-01' ORDERBY TxnDate DESC"`
	}
	qboInvoices struct {
		Customer string `json:"customer,omitempty" jsonschema:"Only this customer's, by its Id"`
		Unpaid   bool   `json:"unpaid,omitempty" jsonschema:"Only those with a balance left"`
		Since    string `json:"since,omitempty" jsonschema:"Only those dated on or after this day, YYYY-MM-DD"`
		Limit    int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 1000; 100 when left out"`
	}
	qboCustomers struct {
		Name  string `json:"name,omitempty" jsonschema:"Only those whose DisplayName starts with this"`
		Limit int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 1000; 100 when left out"`
	}
)
