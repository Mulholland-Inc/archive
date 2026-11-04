package connect

import (
	"time"

	"golang.org/x/oauth2"
)

// Ramp, the corporate card and spend platform, read only through its Developer API. It reads ARCHIVE_RAMP, an app
// of Ramp's partner Developer Console with the authorization code and refresh token grants, whose redirect URIs list
// <API>/connect/ramp/callback; Ramp takes the client as HTTP Basic, and only an admin or the business owner connects
// a business, for everyone. A refresh keeps the refresh token. Ramp's sandbox, on demo.ramp.com and
// demo-api.ramp.com, is two hosts where production has two others, so this reads production alone.
//
// It syncs the business's departments, people and cards, every card transaction and reimbursement into
// spend_transaction with their receipts, its vendors into ledger_contact and its bills into ledger_entry. Ramp lists
// transactions by when they were made and says nothing of what changed since, so they are listed a year at a time
// from eight years back the first time, and after that the last 90 days each sync, which a pending one clears within;
// those pages delete nothing. Reimbursements are listed whole the first time and a week after, and in between by
// what was updated since the sync before. The rest, which are few, are listed whole each sync. Its scopes only
// read, and so do its actions: nothing here moves money, issues or changes a card, or approves spend. Ramp has
// webhooks, which Archive does not register yet: connecting, asking and reading start a sync.
func init() {
	Register(&Connector{
		ID: "ramp", Name: "Ramp", Integration: true,
		Info: Info{By: "Ramp", Description: "Read card spend, reimbursements, bills and vendors", Category: CatFinance, Icon: IconCard,
			Scopes: []string{"Read transactions, cards, people and departments", "Read reimbursements, receipts, bills and vendors"}},
		API: "https://api.ramp.com/developer/v1",
		Endpoint: oauth2.Endpoint{AuthURL: "https://app.ramp.com/v1/authorize", TokenURL: "https://api.ramp.com/developer/v1/token",
			AuthStyle: oauth2.AuthStyleInHeader},
		RevokeBy: &Req{Method: POST, URL: "https://api.ramp.com/developer/v1/token/revoke", Header: map[string][]string{"Authorization": {"{basic}"}}, Form: Q{"token": "{refresh}"}},
		Scopes: []string{"business:read", "users:read", "departments:read", "cards:read", "transactions:read", "reimbursements:read", "receipts:read",
			"vendors:read", "bills:read"},
		Who: &Who{Req: Req{URL: "/business"}, Names: []Path{"business_name_legal", "business_name_on_card", "id"}},
		Actions: map[string]*Action{
			"get": Call[rampGet, any](Plan{
				Doc: "Read any of Ramp's Developer API GET endpoints the connection's scopes allow, like { path: \"/transactions\", query: \"from_date=2026-01-01T00:00:00Z&state=CLEARED\" }, " +
					"{ path: \"/reimbursements\" }, { path: \"/bills\", query: \"payment_status=OPEN\" }, { path: \"/receipts\", query: \"transaction_id=<id>\" } " +
					"or { path: \"/business/balance\" }: its answer, { data, page: { next } } for a list, money in cents (pass page.next as page for more). It only reads",
				Steps: []Step{
					{If: "page", Req: Req{URL: "{+page}"}},
					{Unless: "page", Req: Req{URL: "{+path}?{+query|}"}},
				},
			}),
		},
		Sync: []*Sync{
			{Name: "departments", Gap: time.Second, Req: Req{URL: "/departments", Query: rampQuery}, Items: "data", Page: rampPage,
				Doc: "Every department, by id, listed whole each sync, 100 a page, so one gone is deleted.",
				To:  Map{Table: "spend_department", Cols: map[string]SQL{"title": "data->>'name'"}}},
			{Name: "users", Gap: time.Second, Req: Req{URL: "/users", Query: rampQuery}, Items: "data", Page: rampPage,
				Doc: "Everyone of the business, by id, with their role, status, department and manager, listed whole each sync, 100 a page, so one gone is deleted.",
				To: Map{Table: "spend_user",
					Cols: map[string]SQL{"first_name": "data->>'first_name'", "last_name": "data->>'last_name'", "email": "archive.sync_email(data->>'email')",
						"role": "data->>'role'", "status": "data->>'status'", "employee_id": "data->>'employee_id'"},
					People: map[string]SQL{"person": "data->>'email'"},
					Refs:   map[string]SQL{"department": "data->>'department_id'", "manager": "data->>'manager_id'"}}},
			// Physical and virtual cards are two lists, which Ramp's API still calls beta: a business it does not show them for skips them.
			{Name: "physical_cards", Gap: time.Second, Req: Req{URL: "/cards/physical", Query: rampQuery}, Items: "data", Page: rampPage, Deny: rampBeta,
				Doc: "Every physical card, by id, with its holder, its last four digits and when it expires, listed whole each sync, 100 a page, so one gone is deleted.",
				To: Map{Table: "spend_card",
					Cols: map[string]SQL{"kind": "'physical'", "title": "data->>'display_name'", "last_four": "data->>'last_four'", "status": "data->>'state'",
						"expires":     "CASE WHEN data->>'expiration' ~ '^[0-9]{4}$' THEN substr(data->>'expiration', 1, 2) || '/' || substr(data->>'expiration', 3, 2) END",
						"holder_name": "data->>'cardholder_name'", "made_at": "archive.sync_at(data->>'created_at')"},
					Refs: map[string]SQL{"holder": "data->>'cardholder_id'"}}},
			{Name: "virtual_cards", Gap: time.Second, Req: Req{URL: "/cards/virtual", Query: rampQuery}, Items: "data", Page: rampPage, Deny: rampBeta,
				Doc: "Every virtual card, by id, with its holder and whether it is suspended, listed whole each sync, 100 a page, so one gone is deleted.",
				To: Map{Table: "spend_card",
					Cols: map[string]SQL{"kind": "'virtual'", "status": "CASE data->>'is_card_suspended' WHEN 'true' THEN 'SUSPENDED' WHEN 'false' THEN 'ACTIVE' END",
						"made_at": "archive.sync_at(data->>'created_at')"},
					Refs: map[string]SQL{"holder": "data->>'user_id'"}}},
			{Name: "transactions", Gap: time.Second, Parts: rampYears, Keep: true, Items: "data", Updated: "updated_at", Page: rampPage,
				Req: Req{URL: "/transactions", Query: Q{"page_size": "100", "from_date": "{in/start}T00:00:00Z", "to_date": "{in/end}T23:59:59Z"}},
				Doc: "Every card transaction, by id, with its merchant, amount, state, memo, card, cardholder, department and receipts: a year at a time from eight years " +
					"back the first time, and after that the last 90 days each sync, 100 a page, which deletes nothing.",
				To: Map{Table: "spend_transaction",
					Cols: map[string]SQL{"kind": "'card'", "merchant": "coalesce(nullif(data->>'merchant_name', ''), data->>'merchant_descriptor')", "mcc": "data->>'merchant_category_code'",
						"category": "data->>'sk_category_name'", "amount": rampAmount, "original_amount": rampOriginal,
						"spent_at": "archive.sync_at(data->>'user_transaction_time')", "posted_at": "archive.sync_at(data->>'settlement_date')", "status": "data->>'state'",
						"memo":            "data->>'memo'",
						"spender_name":    "nullif(btrim(concat_ws(' ', data#>>'{card_holder,first_name}', data#>>'{card_holder,last_name}')), '')",
						"department_name": "data#>>'{card_holder,department_name}'", "edited_at": "archive.sync_at(data->>'updated_at')"},
					Refs:     map[string]SQL{"spender": "data#>>'{card_holder,user_id}'", "card": "data->>'card_id'", "department": "data#>>'{card_holder,department_id}'"},
					Children: []Kids{rampReceipts}}},
			{Name: "reimbursements", Gap: time.Second, Req: Req{URL: "/reimbursements", Query: rampQuery}, Items: "data", Updated: "updated_at", Page: rampPage,
				Change: SinceTime{Req: Req{URL: "/reimbursements", Query: Q{"page_size": "100", "updated_after": "{since}"}}},
				Doc: "Every reimbursement, by id, out of pocket, mileage or per diem, with who it is for, its amount, state, memo and receipts: listed whole the first " +
					"time and a week after, 100 a page, which deletes those gone, and in between those updated since the sync before.",
				To: Map{Table: "spend_transaction",
					Cols: map[string]SQL{"kind": "'reimbursement'", "merchant": "data->>'merchant'", "category": "data->>'type'", "amount": rampReimbursed, "original_amount": rampPaid,
						"spent_at": "coalesce(archive.sync_at(data->>'transaction_date'), archive.sync_day(data->>'transaction_date')::timestamp AT TIME ZONE 'UTC')", "posted_at": "archive.sync_at(data->>'payment_processed_at')", "status": "data->>'state'",
						"memo": "data->>'memo'", "spender_name": "data->>'user_full_name'", "edited_at": "archive.sync_at(data->>'updated_at')"},
					Refs:     map[string]SQL{"spender": "data->>'user_id'"},
					Children: []Kids{rampReceipts}}},
			{Name: "vendors", Gap: time.Second, Req: Req{URL: "/vendors", Query: rampQuery}, Items: "data", Updated: "updated_at", Page: rampPage,
				Doc: "Every vendor the business pays, by id, with its legal name and address, listed whole each sync, 100 a page, so one gone is deleted.",
				To: Map{Table: "ledger_contact",
					Cols: map[string]SQL{"kind": "'vendor'", "title": "coalesce(nullif(data->>'display_name', ''), data->>'name')", "company": "data->>'name_legal'",
						"address": `concat_ws(', ', nullif(btrim(data#>>'{address,address_line_1}'), ''), nullif(btrim(data#>>'{address,address_line_2}'), ''), nullif(btrim(data#>>'{address,city}'), ''),
							nullif(btrim(data#>>'{address,state}'), ''), nullif(btrim(data#>>'{address,postal_code}'), ''), nullif(btrim(data#>>'{address,country}'), ''))`,
						"active": "archive.sync_bool(data->'is_active')", "edited_at": "archive.sync_at(data->>'updated_at')"}}},
			// A bill is open until it is paid, and what is left of it is all of it until then.
			{Name: "bills", Gap: time.Second, Req: Req{URL: "/bills", Query: rampQuery}, Items: "data", Page: rampPage,
				Doc: "Every bill, by id, with its vendor, invoice number, amount, dates, status and lines, listed whole each sync, 100 a page, so one gone is deleted.",
				To: Map{Table: "ledger_entry",
					Cols: map[string]SQL{"kind": "'Bill'", "number": "data->>'invoice_number'", "txn_on": "archive.sync_day(data->>'issued_at')", "due_on": "archive.sync_day(data->>'due_at')",
						"contact_name": "data#>>'{vendor,name}'", "total": rampBill, "balance": "CASE WHEN data->>'status' = 'PAID' THEN archive.sync_money('0', data#>>'{amount,currency_code}') ELSE " + rampBill + " END",
						"memo": "data->>'memo'", "status": "coalesce(nullif(data->>'status_summary', ''), data->>'status')"},
					Refs: map[string]SQL{"contact": "data#>>'{vendor,id}'"},
					Children: []Kids{{Table: "ledger_line", From: "archive.sync_list(data->'line_items') WITH ORDINALITY l(x, i)",
						Cols: map[string]SQL{"position": "i", "description": "x->>'memo'",
							"amount": "archive.sync_money((archive.sync_num(x#>>'{amount,amount}') / coalesce(nullif(archive.sync_num(x#>>'{amount,minor_unit_conversion_rate}'), 0), 100))::text, x#>>'{amount,currency_code}')"}}}}},
		},
	})
}

const (
	// Ramp's money is in minor units, with how many make one; a transaction says how many once, beside its amounts. Its
	// amount in dollars, before entity_amount, stands in when that is missing.
	rampAmount SQL = "coalesce(archive.sync_money((archive.sync_num(data#>>'{entity_amount,value}') / coalesce(nullif(archive.sync_num(data->>'minor_unit_conversion_rate'), 0), 100))::text, data#>>'{entity_amount,currency}'), " +
		"archive.sync_money(data->>'amount', coalesce(nullif(data->>'currency_code', ''), 'USD')))"
	rampOriginal   SQL = "archive.sync_money((archive.sync_num(data#>>'{original_transaction_amount,amount}') / coalesce(nullif(archive.sync_num(data#>>'{original_transaction_amount,minor_unit_conversion_rate}'), 0), 100))::text, data#>>'{original_transaction_amount,currency_code}')"
	rampReimbursed SQL = "archive.sync_money((archive.sync_num(data#>>'{entity_amount,value}') / coalesce(nullif(archive.sync_num(data#>>'{entity_amount,minor_unit_conversion_rate}'), 0), 100))::text, data#>>'{entity_amount,currency}')"
	rampPaid       SQL = "archive.sync_money((archive.sync_num(data#>>'{merchant_amount,value}') / coalesce(nullif(archive.sync_num(data#>>'{merchant_amount,minor_unit_conversion_rate}'), 0), 100))::text, data#>>'{merchant_amount,currency}')"
	rampBill       SQL = "archive.sync_money((archive.sync_num(data#>>'{amount,amount}') / coalesce(nullif(archive.sync_num(data#>>'{amount,minor_unit_conversion_rate}'), 0), 100))::text, data#>>'{amount,currency_code}')"
)

var (
	// A page's next is the whole URL of the one after, its cursor in it.
	rampQuery = Q{"page_size": "100"}
	rampPage  = NextURL{At: "page/next"}
	rampBeta  = []Deny{{Status: []int{403, 404}, Why: "Ramp does not list these cards for this business: {message}"}}
	// rampYears are the years to list transactions in, from eight years back, until the stream has listed them once, and
	// after that the last 90 days.
	rampYears = &Parts{Span: &Span{First: Shift{-8, 0, 0}, Step: Shift{1, 0, 0}, Then: Shift{0, 0, -90}}, ID: "{start}", Vals: Obj{"start": "{start}", "end": "{end}"}}
	// A transaction's or reimbursement's receipts are the ids of the images attached to it.
	rampReceipts = Kids{Table: "spend_receipt", From: "archive.sync_list(data->'receipts') WITH ORDINALITY r(x, i)", Cols: map[string]SQL{"receipt_source": "x #>> '{}'", "position": "i"}}
)

type rampGet struct {
	Path  string `json:"path,omitempty" jsonschema:"The endpoint after /developer/v1, like /transactions, /cards/physical, /users, /reimbursements, /receipts, /bills or /vendors"`
	Query string `json:"query,omitempty" jsonschema:"Its query, like from_date=2026-01-01T00:00:00Z&page_size=100"`
	Page  string `json:"page,omitempty" jsonschema:"The page.next of the call before"`
}
