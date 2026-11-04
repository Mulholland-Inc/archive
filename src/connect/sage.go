package connect

import (
	"time"

	"golang.org/x/oauth2"
)

// Sage Accounting, the small-business books Sage once called Business Cloud Accounting, through its v3.1 API,
// which reads businesses in the UK, Ireland and Canada. It reads ARCHIVE_SAGE, the keys of an app from
// developerselfservice.sageone.com, whose callback URLs list <API>/connect/sage/callback. Sage has only two
// scopes, readonly and full_access, so writing anything takes full_access. Its access token lasts 5 minutes,
// and its refresh token 31 days, replaced at every refresh.
//
// A sign-in reaches every business its user has, and a call reads the one X-Business names. The connection keeps
// the user's lead business when it was made, with its base currency, which amounts that name none are in; a
// person connects another by making it their lead and connecting again. Sage lets an app 1,296,000 calls a day and
// 150 at once.
//
// It syncs the books into the ledger tables: the chart of accounts, contacts, products and services, and every
// kind of transaction with its lines, a stream each. Each is listed whole the first time, 200 a page, and after
// that only what was made or changed since the sync before, as updated_or_created_since asks; it is listed whole
// again a week after it last was, which deletes what Sage no longer has, as a void is kept but a draft deleted is
// not. Sage pushes nothing.
func init() {
	Register(&Connector{
		ID: "sage", Name: "Sage", Integration: true,
		Info: Info{By: "Sage", Description: "Read your business’s invoices, contacts and books, and add, change or void contacts and invoices when you ask",
			Category: CatFinance, Icon: IconLedger, Scopes: []string{"Read and change one business’s accounting", "Archive only adds, changes and voids contacts and invoices, when you ask"}},
		API: "https://api.accounting.sage.com/v3.1",
		Let: []Step{
			{Unless: "meta/business", As: "lead", Req: Req{URL: "https://api.accounting.sage.com/v3.1/businesses/lead"}},
			{Unless: "meta/business", As: "meta/business", In: "{lead/id}"},
			{Unless: "meta/business", Fail: "Sage named no business of this sign-in"},
			{Unless: "meta/currency", As: "money", Req: Req{URL: "https://api.accounting.sage.com/v3.1/financial_settings", Header: map[string][]string{"X-Business": {"{meta/business}"}}}},
			{Unless: "meta/currency", As: "meta/currency", In: "{money/base_currency/id}"},
		},
		Sign: map[string]string{"X-Business": "{meta/business}"},
		Endpoint: oauth2.Endpoint{AuthURL: "https://www.sageone.com/oauth2/auth/central?filter=apiv3.1", TokenURL: "https://oauth.accounting.sage.com/token",
			AuthStyle: oauth2.AuthStyleInParams},
		RevokeBy: &Req{Method: POST, URL: "https://oauth.accounting.sage.com/revoke", Form: Q{"client_id": "{client/id}", "client_secret": "{client/secret}", "token": "{refresh}"}},
		Scopes:   []string{"full_access"},
		Who:      &Who{Req: Req{URL: "/businesses/lead"}, Names: []Path{"name", "displayed_as"}},
		Actions: map[string]*Action{
			"business": Call[struct{}, any](Plan{
				Doc:   "The business connected: its name, address, telephone, website and subscriptions",
				Steps: []Step{{Req: Req{URL: "/businesses/{meta/business}"}}},
			}),
			"list": Call[sageFind, any](Plan{
				Doc: "List records of a type with every attribute, maybe only some, like { type: \"sales_invoices\", status: \"UNPAID\", from: \"2026-01-01\" }: " +
					"{ $items: [...], $total, $page, $next } (pass page + 1 for more). search finds words in a name or reference, status is an id like DRAFT, UNPAID, PART_PAID, PAID or VOID",
				Clamp: map[string][2]int{"limit": {50, 200}, "page": {1, 100000}},
				Enum:  map[string][]string{"type": sageTypes},
				Steps: []Step{{Req: Req{URL: "/{type}", Query: Q{"attributes": "all", "search": "{search}", "updated_or_created_since": "{since}", "from_date": "{from}", "to_date": "{to}",
					"contact_id": "{contact}", "status_id": "{status}", "page": "{page}", "items_per_page": "{limit}"}}}},
			}),
			"get": Call[sageOne, any](Plan{
				Doc:   "One record of a type by its id, with every attribute: an invoice with its lines and the payments allocated to it",
				Enum:  map[string][]string{"type": sageTypes},
				Steps: []Step{{Req: Req{URL: "/{type}/{id}", Query: Q{"attributes": "all"}}}},
			}),
			"trial_balance": Call[sageSpan, any](Plan{
				Doc:   "The trial balance between two days: each ledger account with its opening and closing balance, debits and credits",
				Steps: []Step{{Req: Req{URL: "/trial_balance", Query: Q{"from_date": "{from}", "to_date": "{to}"}}}},
			}),
			"contact": Call[sageContactIn, any](Plan{
				Doc: "Add a contact, or change one by its id: only the fields given change. Like { contact: { name, contact_type_ids: [\"CUSTOMER\"], email, main_address: { address_line_1, city, postal_code } } }. " +
					"Returns the contact",
				Steps: []Step{
					{If: "id", As: "got", Req: Req{Method: PUT, URL: "/contacts/{id}", Body: Obj{"contact": "{contact}"}}},
					{Unless: "id", As: "got", Req: Req{Method: POST, URL: "/contacts", Header: sageOnce, Body: Obj{"contact": "{contact}"}}},
				},
				Out: "{got}",
			}),
			"invoice": Call[sageInvoiceIn, any](Plan{
				Doc: "Add a sales invoice, or change one by its id. Like { invoice: { contact_id, date, due_date, invoice_lines: [{ description, ledger_account_id, quantity, unit_price, tax_rate_id }] } }, " +
					"the ledger account and tax rate by their ids from list. Returns the invoice",
				Steps: []Step{
					{If: "id", As: "got", Req: Req{Method: PUT, URL: "/sales_invoices/{id}", Body: Obj{"sales_invoice": "{invoice}"}}},
					{Unless: "id", As: "got", Req: Req{Method: POST, URL: "/sales_invoices", Header: sageOnce, Body: Obj{"sales_invoice": "{invoice}"}}},
				},
				Out: "{got}",
			}),
			"void": Call[sageVoid, sageVoided](Plan{
				Doc:  "Void an invoice or credit note, sales or purchase, with a reason: Sage keeps it, VOID, at no amount. A draft is refused, as Sage would delete it. { id }",
				Enum: map[string][]string{"type": {"sales_invoices", "sales_credit_notes", "purchase_invoices", "purchase_credit_notes"}},
				Steps: []Step{
					{As: "got", Req: Req{URL: "/{type}/{id}"}},
					{Only: Test{At: "got/status/id", Is: "DRAFT"}, Fail: "It is a draft, which Sage deletes rather than voids; it was left alone"},
					{Req: Req{Method: DELETE, URL: "/{type}/{id}", Query: Q{"void_reason": "{reason}"}}},
				},
				Out: Obj{"id": "{id}"},
			}),
		},
		Sync: []*Sync{
			{Name: "ledger_accounts", Gap: time.Second, Req: sageList, Items: "$items", Page: sagePage, Change: sageChange, Set: sageSet, Gone: sageGone, Deny: sageDeny,
				Vars: Obj{"entity": "ledger_accounts"},
				Doc:  "The chart of accounts" + sageDoc,
				To: Map{Table: "ledger_account",
					Cols: map[string]SQL{"number": "data->>'nominal_code'", "title": "data->>'name'", "full_name": "data->>'display_name'", "type": "data#>>'{ledger_account_type,displayed_as}'",
						"subtype": "data#>>'{ledger_account_classification,displayed_as}'",
						"class":   "CASE upper(data#>>'{ledger_account_group,id}') WHEN 'INCOME' THEN 'Revenue' WHEN 'SALES' THEN 'Revenue' ELSE initcap(data#>>'{ledger_account_group,id}') END",
						"active":  "archive.sync_bool(data->'included_in_chart')", "edited_at": sageEditedAt}}},
			{Name: "contacts", Gap: time.Second, Req: sageList, Items: "$items", Page: sagePage, Change: sageChange, Set: sageSet, Gone: sageGone, Deny: sageDeny,
				Vars: Obj{"entity": "contacts", "balance": "true"},
				Doc:  "Every customer and supplier, with what they owe or are owed" + sageDoc,
				// A contact is a vendor when it is only a supplier, and else a customer; its email and phone are its own or its main
				// person's, and its address its main one, on one line.
				To: Map{Table: "ledger_contact",
					Cols: map[string]SQL{"kind": sageKind, "title": "coalesce(nullif(data->>'name', ''), data->>'displayed_as')",
						"email": "archive.sync_email(coalesce(nullif(data->>'email', ''), data#>>'{main_contact_person,email}'))",
						"phone": "coalesce(nullif(data#>>'{main_contact_person,telephone}', ''), nullif(data#>>'{main_contact_person,mobile}', ''))",
						"address": `nullif(concat_ws(', ', nullif(btrim(data#>>'{main_address,address_line_1}'), ''), nullif(btrim(data#>>'{main_address,address_line_2}'), ''),
							nullif(btrim(data#>>'{main_address,city}'), ''), nullif(btrim(data#>>'{main_address,region}'), ''), nullif(btrim(data#>>'{main_address,postal_code}'), ''),
							nullif(btrim(data#>>'{main_address,country,displayed_as}'), '')), '')`,
						"balance": "archive.sync_money(data->>'balance', " + sageCurrency + ")", "active": "archive.sync_bool(data->'is_active')", "edited_at": sageEditedAt},
					People: map[string]SQL{"person": "coalesce(nullif(data->>'email', ''), data#>>'{main_contact_person,email}')"}}},
			{Name: "products", Gap: time.Second, Req: sageList, Items: "$items", Page: sagePage, Change: sageChange, Set: sageSet, Gone: sageGone, Deny: sageDeny, To: sageItem,
				Vars: Obj{"entity": "products", "kind": "Product"}, Doc: "Every product" + sageDoc},
			{Name: "services", Gap: time.Second, Req: sageList, Items: "$items", Page: sagePage, Change: sageChange, Set: sageSet, Gone: sageGone, Deny: sageDeny, To: sageItem,
				Vars: Obj{"entity": "services", "kind": "Service"}, Doc: "Every service" + sageDoc},
			{Name: "sales_invoices", Gap: time.Second, Req: sageList, Items: "$items", Page: sagePage, Change: sageChange, Set: sageSet, Gone: sageGone, Deny: sageDeny, To: sageEntry,
				Vars: Obj{"entity": "sales_invoices", "kind": "Invoice"}, Doc: "Every sales invoice" + sageDoc},
			{Name: "sales_credit_notes", Gap: time.Second, Req: sageList, Items: "$items", Page: sagePage, Change: sageChange, Set: sageSet, Gone: sageGone, Deny: sageDeny, To: sageEntry,
				Vars: Obj{"entity": "sales_credit_notes", "kind": "CreditNote"}, Doc: "Every sales credit note" + sageDoc},
			{Name: "sales_quotes", Gap: time.Second, Req: sageList, Items: "$items", Page: sagePage, Change: sageChange, Set: sageSet, Gone: sageGone, Deny: sageDeny, To: sageEntry,
				Vars: Obj{"entity": "sales_quotes", "kind": "Quote"}, Doc: "Every quote" + sageDoc},
			{Name: "sales_estimates", Gap: time.Second, Req: sageList, Items: "$items", Page: sagePage, Change: sageChange, Set: sageSet, Gone: sageGone, Deny: sageDeny, To: sageEntry,
				Vars: Obj{"entity": "sales_estimates", "kind": "Estimate"}, Doc: "Every estimate" + sageDoc},
			{Name: "purchase_invoices", Gap: time.Second, Req: sageList, Items: "$items", Page: sagePage, Change: sageChange, Set: sageSet, Gone: sageGone, Deny: sageDeny, To: sageEntry,
				Vars: Obj{"entity": "purchase_invoices", "kind": "Bill"}, Doc: "Every purchase invoice, a bill" + sageDoc},
			{Name: "purchase_credit_notes", Gap: time.Second, Req: sageList, Items: "$items", Page: sagePage, Change: sageChange, Set: sageSet, Gone: sageGone, Deny: sageDeny, To: sageEntry,
				Vars: Obj{"entity": "purchase_credit_notes", "kind": "BillCredit"}, Doc: "Every purchase credit note" + sageDoc},
			{Name: "contact_payments", Gap: time.Second, Req: sageList, Items: "$items", Page: sagePage, Change: sageChange, Set: sageSet, Gone: sageGone, Deny: sageDeny, To: sageEntry,
				Vars: Obj{"entity": "contact_payments", "kind": "Payment"}, Doc: "Every payment from a customer or to a supplier, and their refunds" + sageDoc},
			{Name: "other_payments", Gap: time.Second, Req: sageList, Items: "$items", Page: sagePage, Change: sageChange, Set: sageSet, Gone: sageGone, Deny: sageDeny, To: sageEntry,
				Vars: Obj{"entity": "other_payments", "kind": "OtherPayment"}, Doc: "Every money spent or received that is not a contact's invoice's" + sageDoc},
			{Name: "bank_transfers", Gap: time.Second, Req: sageList, Items: "$items", Page: sagePage, Change: sageChange, Set: sageSet, Gone: sageGone, Deny: sageDeny, To: sageEntry,
				Vars: Obj{"entity": "bank_transfers", "kind": "Transfer"}, Doc: "Every transfer between bank accounts" + sageDoc},
			{Name: "journals", Gap: time.Second, Req: sageList, Items: "$items", Page: sagePage, Change: sageChange, Set: sageSet, Gone: sageGone, Deny: sageDeny, To: sageEntry,
				Vars: Obj{"entity": "journals", "kind": "Journal"}, Doc: "Every journal" + sageDoc},
		},
	})
}

const (
	sageDoc = ", with every attribute: listed whole the first time and a week after, 200 a page, which deletes those Sage no longer has, " +
		"and in between what was made or changed since the sync before."
	sageEditedAt SQL = "archive.sync_at(data->>'updated_at')"
	// What amounts are in: the record's currency, or the business's.
	sageCurrency     = "coalesce(nullif(data#>>'{currency,id}', ''), data->>'base')"
	sageKind     SQL = `CASE WHEN EXISTS (SELECT FROM archive.sync_list(data->'contact_types') t WHERE t->>'id' = 'VENDOR')
		AND NOT EXISTS (SELECT FROM archive.sync_list(data->'contact_types') t WHERE t->>'id' = 'CUSTOMER') THEN 'vendor' ELSE 'customer' END`
)

var (
	sageTypes = []string{"contacts", "sales_invoices", "sales_credit_notes", "sales_quotes", "sales_estimates", "purchase_invoices", "purchase_credit_notes",
		"contact_payments", "other_payments", "bank_accounts", "bank_transfers", "journals", "ledger_accounts", "products", "services", "tax_rates"}
	// A new record names itself, so that Sage makes one of it however often it is sent.
	sageOnce = map[string][]string{"Idempotency-Key": {"{nonce}"}}
	sageSet  = Obj{"kind": "{kind}", "base": "{meta/currency}"}
	sageList = Req{URL: "/{entity}", Query: Q{"attributes": "all", "items_per_page": "200", "page": "{page}", "show_balance": "{balance|}"}}
	sagePage = Offset{From: 1, Size: 200, Step: 1}
	// Sage reads updated_or_created_since as on or after it.
	sageChange = SinceTime{Req: Req{URL: "/{entity}", Query: Q{"attributes": "all", "items_per_page": "200", "page": "{page}", "show_balance": "{balance|}", "updated_or_created_since": "{since}"}}}
	sageGone   = []Test{{At: "deleted_at"}}
	// A business whose plan or region does not have a kind of record, or a user whose role does not reach it, is refused it.
	sageDeny = []Deny{{Status: []int{403, 404}, Why: "Sage does not list {entity} for this business: {message}"}}
	// A product or service as an item: its description names it, and its first price is what it sells for.
	sageItem = Map{Table: "ledger_item",
		Cols: map[string]SQL{"title": "coalesce(nullif(data->>'description', ''), data->>'displayed_as')", "sku": "data->>'item_code'", "type": "data->>'kind'", "description": "data->>'notes'",
			"price": "archive.sync_money(coalesce(data#>>'{sales_prices,0,price}', data#>>'{sales_rates,0,rate}'), data->>'base')", "cost": "archive.sync_money(data->>'cost_price', data->>'base')",
			"active": "archive.sync_bool(data->'active')", "edited_at": sageEditedAt},
		Refs: map[string]SQL{"income_account": "data#>>'{sales_ledger_account,id}'", "expense_account": "data#>>'{purchase_ledger_account,id}'"}}
	// A transaction as an entry of the books, with its lines; a journal's line is a debit or a credit. A payment to a
	// supplier is a bill's, and another payment spends unless it is a receipt. Who it is with is its contact; the bank
	// account it pays from or into is not one of the ledger's accounts, so it is not linked.
	sageEntry = Map{Table: "ledger_entry",
		Cols: map[string]SQL{"kind": `CASE WHEN data->>'kind' = 'Payment' AND data#>>'{transaction_type,id}' LIKE 'VENDOR%' THEN 'BillPayment'
				WHEN data->>'kind' = 'OtherPayment' THEN CASE WHEN data#>>'{transaction_type,id}' = 'OTHER_RECEIPT' THEN 'Receive' ELSE 'Spend' END ELSE data->>'kind' END`,
			"number":       "coalesce(nullif(data->>'invoice_number', ''), nullif(data->>'credit_note_number', ''), nullif(data->>'quote_number', ''), nullif(data->>'estimate_number', ''), data->>'vendor_reference')",
			"txn_on":       "archive.sync_day(data->>'date')",
			"due_on":       "archive.sync_day(coalesce(data->>'due_date', data->>'expiry_date'))",
			"contact_name": "coalesce(nullif(data->>'contact_name', ''), data#>>'{contact,displayed_as}')",
			"total":        "archive.sync_money(coalesce(data->>'total_amount', data->>'total', data->>'amount'), " + sageCurrency + ")",
			"balance":      "archive.sync_money(data->>'outstanding_amount', " + sageCurrency + ")",
			"memo":         `concat_ws(E'\n\n', nullif(btrim(data->>'reference'), ''), nullif(btrim(data->>'notes'), ''), nullif(btrim(data->>'description'), ''))`,
			"status":       "data#>>'{status,displayed_as}'",
			"edited_at":    sageEditedAt},
		Refs: map[string]SQL{"contact": "data#>>'{contact,id}'"},
		Children: []Kids{{Table: "ledger_line",
			From: "archive.sync_list(coalesce(data->'invoice_lines', data->'credit_note_lines', data->'quote_lines', data->'estimate_lines', data->'journal_lines', data->'payment_lines')) WITH ORDINALITY l(x, i)",
			Cols: map[string]SQL{"position": "i", "description": "coalesce(nullif(x->>'description', ''), x->>'details')",
				"amount":     "archive.sync_money(coalesce(x->>'net_amount', CASE WHEN archive.sync_num(x->>'debit') > 0 THEN x->>'debit' ELSE x->>'credit' END), " + sageCurrency + ")",
				"posting":    "CASE WHEN x ? 'debit' THEN CASE WHEN archive.sync_num(x->>'debit') > 0 THEN 'Debit' ELSE 'Credit' END END",
				"quantity":   "archive.sync_num(x->>'quantity')",
				"unit_price": "archive.sync_num(x->>'unit_price')"},
			Refs: map[string]SQL{"account": "x#>>'{ledger_account,id}'", "item": "coalesce(x#>>'{product,id}', x#>>'{service,id}')"}}}}
)

type (
	sageFind struct {
		Type    string `json:"type" jsonschema:"contacts, sales_invoices, sales_credit_notes, sales_quotes, sales_estimates, purchase_invoices, purchase_credit_notes, contact_payments, other_payments, bank_accounts, bank_transfers, journals, ledger_accounts, products, services or tax_rates"`
		Search  string `json:"search,omitempty" jsonschema:"Words in a name, reference or item code"`
		Since   string `json:"since,omitempty" jsonschema:"Only those made or changed on or after this time, like 2026-10-01T00:00:00Z"`
		From    string `json:"from,omitempty" jsonschema:"Only those dated on or after this day, YYYY-MM-DD"`
		To      string `json:"to,omitempty" jsonschema:"Only those dated on or before this day, YYYY-MM-DD"`
		Contact string `json:"contact,omitempty" jsonschema:"Only this contact's, by its id"`
		Status  string `json:"status,omitempty" jsonschema:"Only those of this status id, like UNPAID"`
		Page    int    `json:"page,omitempty" jsonschema:"Which page, from 1; 1 when left out"`
		Limit   int    `json:"limit,omitempty" jsonschema:"At most this many a page, 1 to 200; 50 when left out"`
	}
	sageOne struct {
		Type string `json:"type" jsonschema:"One of the types list takes"`
		ID   string `json:"id" jsonschema:"Its id"`
	}
	sageSpan struct {
		From string `json:"from" jsonschema:"The first day, YYYY-MM-DD"`
		To   string `json:"to" jsonschema:"The last day, YYYY-MM-DD"`
	}
	sageContactIn struct {
		ID      string         `json:"id,omitempty" jsonschema:"The contact's id, to change it; a new one when left out"`
		Contact map[string]any `json:"contact" jsonschema:"The contact as Sage has it, with name and contact_type_ids, like CUSTOMER or VENDOR, to add one"`
	}
	sageInvoiceIn struct {
		ID      string         `json:"id,omitempty" jsonschema:"The sales invoice's id, to change it; a new one when left out"`
		Invoice map[string]any `json:"invoice" jsonschema:"The sales invoice as Sage has it, with contact_id, date and invoice_lines to add one"`
	}
	sageVoid struct {
		Type   string `json:"type" jsonschema:"sales_invoices, sales_credit_notes, purchase_invoices or purchase_credit_notes"`
		ID     string `json:"id" jsonschema:"Its id"`
		Reason string `json:"reason" jsonschema:"Why it is voided, which Sage keeps with it"`
	}
	sageVoided struct {
		ID string `json:"id"`
	}
)
