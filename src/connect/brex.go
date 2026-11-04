package connect

import (
	"time"

	"golang.org/x/oauth2"
)

// Brex, the corporate card and spend platform, read only through its API. It reads ARCHIVE_BREX, the OAuth client
// Brex issues a partner, whose redirect URIs, which Brex sets, list <API>/connect/brex/callback; Brex takes the client
// in the form, gives a refresh token for offline_access and a new one with every refresh, and only an account admin
// connects a company, for everyone. Its staging, on two hosts of its own, is not a sandbox, so this reads production
// alone.
//
// It syncs the company's departments, people and cards, every expense into spend_transaction with its receipts, card
// purchases and reimbursements alike, and its vendors into ledger_contact. Brex's expenses are what its people spend,
// with a memo, category and receipts, but not the card; its card transactions, which name the card and nothing more,
// the get action reads. It has no API for bills, whose payments are expenses too. Expenses are listed whole the first
// time and a week after, which deletes those gone, and in between by what was updated since the sync before; the rest,
// which are few, are listed whole each sync. Its scopes only read, and so do its actions: nothing here moves money,
// issues or changes a card, or approves spend, and cards.pan, which shows a card's number, is not asked for. Brex has
// webhooks, which Archive does not register yet: connecting, asking and reading start a sync.
func init() {
	Register(&Connector{
		ID: "brex", Name: "Brex", Integration: true,
		Info: Info{By: "Brex", Description: "Read card spend, reimbursements and vendors", Category: CatFinance, Icon: IconCard,
			Scopes: []string{"Read expenses, receipts, cards, people and departments", "Read card transactions, accounts, budgets and vendors"}},
		API: "https://api.brex.com",
		Endpoint: oauth2.Endpoint{AuthURL: "https://accounts-api.brex.com/oauth2/default/v1/authorize", TokenURL: "https://accounts-api.brex.com/oauth2/default/v1/token",
			AuthStyle: oauth2.AuthStyleInParams},
		RevokeBy: &Req{Method: POST, URL: "https://accounts-api.brex.com/oauth2/default/v1/revoke",
			Form: Q{"client_id": "{client/id}", "client_secret": "{client/secret}", "token": "{refresh}", "token_type_hint": "refresh_token"}},
		Scopes: []string{"openid", "offline_access", "companies.readonly", "users.readonly", "departments.readonly", "locations.readonly", "cards.readonly",
			"expenses.card.readonly", "transactions.card.readonly", "accounts.card.readonly", "budgets.readonly", "vendors.readonly"},
		Who: &Who{Req: Req{URL: "/v2/company"}, Names: []Path{"legal_name", "id"}},
		Actions: map[string]*Action{
			"get": Call[brexGet, any](Plan{
				Doc: "Read any of Brex's GET endpoints the connection's scopes allow, like { path: \"/v2/transactions/card/primary\", query: \"posted_at_start=2026-01-01T00:00:00Z\" }, " +
					"{ path: \"/v1/expenses\", query: \"expense_type[]=REIMBURSEMENT&expand[]=receipts.download_uris\" }, { path: \"/v2/accounts/card\" }, { path: \"/v2/budgets\" } " +
					"or { path: \"/v2/users/me\" }: its answer, { items, next_cursor } for a list, money as { amount in cents, currency } (pass cursor=<next_cursor> in query for more). It only reads",
				Steps: []Step{{Req: Req{URL: "{+path}?{+query|}"}}},
			}),
		},
		Sync: []*Sync{
			{Name: "departments", Gap: time.Second, Req: Req{URL: "/v2/departments", Query: brexQuery}, Items: "items", Page: brexPage,
				Doc: "Every department, by id, listed whole each sync, 100 a page, so one gone is deleted.",
				To:  Map{Table: "spend_department", Cols: map[string]SQL{"title": "data->>'name'"}}},
			{Name: "users", Gap: time.Second, Req: Req{URL: "/v2/users", Query: brexQuery}, Items: "items", Page: brexPage,
				Doc: "Everyone of the company, by id, with their status, department and manager, listed whole each sync, 100 a page, so one gone is deleted.",
				To: Map{Table: "spend_user",
					Cols: map[string]SQL{"first_name": "data->>'first_name'", "last_name": "data->>'last_name'", "email": "archive.sync_email(data->>'email')",
						"status": "data->>'status'", "employee_id": "data->>'remote_display_id'"},
					People: map[string]SQL{"person": "data->>'email'"},
					Refs:   map[string]SQL{"department": "data->>'department_id'", "manager": "data->>'manager_id'"}}},
			{Name: "cards", Gap: time.Second, Req: Req{URL: "/v2/cards", Query: brexQuery}, Items: "items", Page: brexPage,
				Doc: "Every card, physical and virtual, by id, with its holder, its last four digits, when it expires and a vendor card's own limit, listed whole each sync, " +
					"100 a page, so one gone is deleted.",
				To: Map{Table: "spend_card",
					Cols: map[string]SQL{"title": "data->>'card_name'", "kind": "lower(data->>'card_type')", "last_four": "data->>'last_four'", "status": "data->>'status'",
						"expires":      "CASE WHEN data#>>'{expiration_date,month}' ~ '^[0-9]{1,2}$' AND data#>>'{expiration_date,year}' ~ '^[0-9]{2,4}$' THEN lpad(data#>>'{expiration_date,month}', 2, '0') || '/' || right(data#>>'{expiration_date,year}', 2) END",
						"spend_limit":  brexLimit,
						"limit_period": "data#>>'{spend_controls,spend_duration}'", "made_at": "archive.sync_at(data->>'created_at')"},
					Refs: map[string]SQL{"holder": "data#>>'{owner,user_id}'"}}},
			{Name: "expenses", Gap: time.Second, Req: Req{URL: "/v1/expenses", Query: brexExpenses}, Items: "items", Updated: "updated_at", Page: brexPage,
				Vars:   Obj{"expand": []string{"user", "department", "merchant"}},
				Change: SinceTime{Req: Req{URL: "/v1/expenses", Query: Q{"limit": "100", "cursor": "{page}", "expand[]": "{expand}", "updated_at_start": "{since}"}}},
				Doc: "Every expense, card, reimbursement or bill payment, by id, with its merchant, amount, status, memo, category, spender, department and receipts: listed " +
					"whole the first time and a week after, 100 a page, which deletes those gone, and in between those updated since the sync before.",
				To: Map{Table: "spend_transaction",
					Cols: map[string]SQL{"kind": "coalesce(lower(nullif(data->>'expense_type', '')), 'card')", "merchant": "data#>>'{merchant,raw_descriptor}'", "mcc": "data#>>'{merchant,mcc}'",
						"category": "data->>'category'", "amount": brexAmount, "original_amount": brexOriginal,
						"spent_at": "archive.sync_at(data->>'purchased_at')", "posted_at": "archive.sync_at(data->>'payment_posted_at')",
						"status": "coalesce(nullif(data->>'payment_status', ''), data->>'status')", "memo": "data->>'memo'",
						"spender_name":    "nullif(btrim(concat_ws(' ', data#>>'{user,first_name}', data#>>'{user,last_name}')), '')",
						"department_name": "data#>>'{department,name}'", "url": "data->>'dashboard_url'", "edited_at": "archive.sync_at(data->>'updated_at')"},
					Refs: map[string]SQL{"spender": "data->>'user_id'", "department": "data->>'department_id'"},
					Children: []Kids{{Table: "spend_receipt", From: "archive.sync_list(data->'receipts') WITH ORDINALITY r(x, i)",
						Cols: map[string]SQL{"receipt_source": "x->>'id'", "position": "i"}}}}},
			{Name: "vendors", Gap: time.Second, Req: Req{URL: "/v1/vendors", Query: brexQuery}, Items: "items", Page: brexPage,
				Doc: "Every vendor the company pays, by id, with its email and phone, listed whole each sync, 100 a page, so one gone is deleted.",
				To: Map{Table: "ledger_contact",
					Cols:   map[string]SQL{"kind": "'vendor'", "title": "data->>'company_name'", "email": "archive.sync_email(data->>'email')", "phone": "data->>'phone'"},
					People: map[string]SQL{"person": "data->>'email'"}}},
		},
	})
}

// Brex's money is { amount, currency }, the amount in cents.
const (
	brexAmount   SQL = "archive.sync_money((archive.sync_num(data#>>'{billing_amount,amount}') / 100)::text, data#>>'{billing_amount,currency}')"
	brexOriginal SQL = "archive.sync_money((archive.sync_num(data#>>'{original_amount,amount}') / 100)::text, data#>>'{original_amount,currency}')"
	brexLimit    SQL = "archive.sync_money((archive.sync_num(data#>>'{spend_controls,spend_limit,amount}') / 100)::text, data#>>'{spend_controls,spend_limit,currency}')"
)

var (
	brexQuery = Q{"limit": "100", "cursor": "{page}"}
	// An expense names its spender, department and merchant by id unless asked for them.
	brexExpenses = Q{"limit": "100", "cursor": "{page}", "expand[]": "{expand}"}
	brexPage     = Cursor{At: "next_cursor"}
)

type brexGet struct {
	Path  string `json:"path" jsonschema:"The endpoint, like /v2/transactions/card/primary, /v1/expenses, /v2/cards, /v2/users, /v2/accounts/card or /v1/vendors"`
	Query string `json:"query,omitempty" jsonschema:"Its query, like posted_at_start=2026-01-01T00:00:00Z&limit=100 or cursor=<next_cursor>"`
}
