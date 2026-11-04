package connect

import "time"

// Stripe, through its hosted MCP server: customers, payments, invoices, subscriptions and the rest of an
// account, as the person who connects allows. Its authorization server registers any client, but only as a
// public one, so Archive's client there has no secret and proves itself with PKCE alone. An account's admins
// can turn MCP access off. get_stripe_account_info names the account.
//
// It syncs each live account the person allowed, a sandbox left out: its customers into ledger_contact and its invoices
// and charges into ledger_entry, through stripe_api_read and Stripe's list methods, 100 a page, newest first. Those say
// nothing of what changed when, so each is listed at most an hour apart. A listing deletes nothing, so what is deleted
// there stays here. Its actions are the server's tools.
func init() {
	Register(&Connector{ID: "stripe", Name: "Stripe", Integration: true, MCP: "https://mcp.stripe.com", Who: &Who{Tool: "get_stripe_account_info"}, Busy: ToolBusy,
		Info: Info{By: "Stripe", Description: "Read customers, invoices and charges, and look up payments and subscriptions", Category: CatFinance, Icon: IconCard, Scopes: []string{"Use your Stripe account as you allow"}},
		Sync: []*Sync{
			{Name: "customers", Gap: time.Second, Every: time.Hour, Keep: true, Parts: stripeAccounts, Req: stripeList("GetCustomers"), Items: "data", ID: "id", Page: stripePage,
				Doc: "Every customer of each live account, by id, with their name, email, phone, address and balance: listed at most an hour apart, 100 a page. A listing deletes nothing, so one deleted in Stripe stays here.",
				To: Map{Table: "ledger_contact",
					Cols: map[string]SQL{"kind": "'customer'", "title": "data->>'name'", "email": "archive.sync_email(data->>'email')", "phone": "data->>'phone'",
						"address": `nullif(concat_ws(', ', nullif(btrim(data#>>'{address,line1}'), ''), nullif(btrim(data#>>'{address,line2}'), ''), nullif(btrim(data#>>'{address,city}'), ''),
							nullif(btrim(data#>>'{address,state}'), ''), nullif(btrim(data#>>'{address,postal_code}'), ''), nullif(btrim(data#>>'{address,country}'), '')), '')`,
						"balance": "CASE WHEN data->>'currency' IS NOT NULL THEN " + stripeMoney("data", "balance") + " END"},
					People: map[string]SQL{"person": "data->>'email'"}}},
			{Name: "invoices", Gap: time.Second, Every: time.Hour, Keep: true, Parts: stripeAccounts, Req: stripeList("GetInvoices"), Items: "data", ID: "id", Page: stripePage,
				Doc: "Every invoice of each live account, by id, with its customer, number, dates, total, what is left to pay, status and up to its first 10 lines, as a list shows them: listed at most an hour apart, 100 a page. A listing deletes nothing, so one deleted in Stripe stays here.",
				To: Map{Table: "ledger_entry",
					Cols: map[string]SQL{"kind": "'Invoice'", "number": "data->>'number'", "txn_on": stripeDay("created"), "due_on": stripeDay("due_date"), "contact_name": "data->>'customer_name'",
						"total": stripeMoney("data", "total"), "balance": stripeMoney("data", "amount_remaining"), "memo": "data->>'description'", "status": "data->>'status'"},
					Refs: map[string]SQL{"contact": "data->>'customer'"},
					Children: []Kids{{Table: "ledger_line", From: "archive.sync_list(data#>'{lines,data}') WITH ORDINALITY l(x, i)",
						Cols: map[string]SQL{"position": "i", "description": "x->>'description'", "amount": stripeMoney("x", "amount"), "quantity": "archive.sync_num(x->>'quantity')"}}}}},
			{Name: "charges", Gap: time.Second, Every: time.Hour, Keep: true, Parts: stripeAccounts, Req: stripeList("GetCharges"), Items: "data", ID: "id", Page: stripePage,
				Doc: "Every charge of each live account, by id, as a Payment, with its customer, amount, description and status: listed at most an hour apart, 100 a page. A listing deletes nothing.",
				To: Map{Table: "ledger_entry",
					Cols: map[string]SQL{"kind": "'Payment'", "number": "data->>'receipt_number'", "txn_on": stripeDay("created"), "contact_name": "data#>>'{billing_details,name}'",
						"total": stripeMoney("data", "amount"), "memo": "data->>'description'", "status": "data->>'status'"},
					Refs: map[string]SQL{"contact": "data->>'customer'"}}},
		}})
}

var (
	// The next page starts after the last record of this one.
	stripePage = Cursor{At: "data/-1/id", More: "has_more"}
	// The accounts the person allowed; every other tool names one, and says whether it is live.
	stripeAccounts = &Parts{Req: Req{Tool: "list_available_accounts_or_orgs"}, Items: "accounts", ID: "{stripe_context}", Skip: []Test{{At: "livemode", Is: "false"}}}
)

// stripeList is a list method of Stripe's API, by its operation id, for the account.
func stripeList(op string) Req {
	return Req{Tool: "stripe_api_read", Body: Obj{"stripe_api_operation_id": op, "stripe_context": "{part}", "livemode": true, "parameters": Obj{"limit": 100, "starting_after": "{page}"}}}
}

// stripeMoney is the amount at key of of, already in the currency's minor units, as sync_money takes major ones.
func stripeMoney(of, key string) SQL {
	return SQL("archive.sync_money((archive.sync_num(" + of + "->>'" + key + "') / 100)::text, upper(" + of + "->>'currency'))")
}

// stripeDay is the date, in UTC, of the Unix seconds at key.
func stripeDay(key string) SQL {
	return SQL("(to_timestamp(archive.sync_num(data->>'" + key + "')) AT TIME ZONE 'UTC')::date")
}
