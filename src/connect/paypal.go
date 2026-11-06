package connect

import "time"

// PayPal, a business account through PayPal's REST API: its transactions, invoices and disputes, read, and when asked
// an invoice drafted and sent. PayPal's sign-in for other apps, Log in with PayPal, only says who someone is, and its
// partner onboarding is for platforms PayPal approves to take payments for others, so this connects as a REST app the
// business makes itself: at developer.paypal.com, Apps & Credentials, Live, an app with Transaction search and
// Invoicing ticked, whose client ID and secret an owner pastes in. Each client trades them for an access token, which
// lasts nine hours, and signs every request with it; the keys last until the business deletes the app or its secret.
// Nothing here moves money.
//
// It syncs the account's transactions into commerce_payment: PayPal searches no more than 31 days at once, three years
// back, and a transaction shows up to three hours after it happens, so the first sync reads three years, a month at a
// time, and each sync after the last 31 days again. A listing deletes no transaction. Its invoices, into
// commerce_invoice, and disputes, into commerce_dispute, are listed whole each sync. It pushes nothing: PayPal's
// webhooks are each app's own.
func init() {
	Register(&Connector{
		ID: "paypal", Name: "PayPal", Integration: true,
		Info: Info{By: "PayPal", Description: "Read your transactions, invoices and disputes, and draft and send invoices when you ask", Category: CatFinance, Icon: IconCard,
			Scopes: []string{"Read transactions, invoices and disputes with the business’s app keys", "Draft and send invoices when you ask"}},
		Keys: []Key{
			{Name: "id", Label: "Client ID", Help: "At developer.paypal.com, Apps & Credentials, Live: make an app of type Merchant, tick Transaction search and Invoicing under its features, and copy its Client ID"},
			{Name: "secret", Label: "Secret", Secret: true, Help: "The same app's Secret key, under its Client ID"},
		},
		API: "https://api-m.paypal.com",
		Let: []Step{
			{As: "basic", In: "{key/id}:{key/secret}", Codec: Basic},
			{As: "auth", Req: Req{Method: POST, Read: true, URL: "https://api-m.paypal.com/v1/oauth2/token", Header: map[string][]string{"Authorization": {"{basic}"}},
				Form: Q{"grant_type": "client_credentials"}}},
		},
		Sign: map[string]string{"Authorization": "Bearer {let/auth/access_token}"},
		// The account's balances name it by its PayPal account id, and need Transaction search, as its sync does.
		Who: &Who{Req: Req{URL: "/v1/reporting/balances"}, Names: []Path{"account_id"}},
		Actions: map[string]*Action{
			"transactions": Call[paypalSearch, paypalFound](Plan{
				Doc: "Transactions between two times no more than 31 days apart, within the last three years, maybe only one by its id: " +
					"{ transactions: [{ transaction_info, payer_info, cart_info, shipping_info }], pages } (pass page for more). " +
					"A transaction shows up to three hours after it happens",
				Clamp: map[string][2]int{"limit": {100, 500}},
				Steps: []Step{{As: "got", Req: Req{URL: "/v1/reporting/transactions", Query: Q{"start_date": "{from}", "end_date": "{to}", "transaction_id": "{id}",
					"fields": "all", "page_size": "{limit}", "page": "{page}"}}}},
				Out: Obj{"transactions": "{got/transaction_details|[]}", "pages": "{got/total_pages|1}"},
			}),
			"balances": Call[struct{}, any](Plan{
				Doc:   "The account's balances now, a currency each: { balances: [{ currency, total_balance, available_balance, withheld_balance }], account_id }",
				Steps: []Step{{Req: Req{URL: "/v1/reporting/balances"}}},
			}),
			"invoices": Call[paypalInvoices, paypalBilled](Plan{
				Doc:   "Invoices, maybe only those to an address, in some statuses, or dated between two days: { invoices: [Invoice], pages } (pass page for more)",
				Clamp: map[string][2]int{"limit": {20, 100}},
				Steps: []Step{
					{If: "since", As: "dates/start", In: "{since}"},
					{If: "until", As: "dates/end", In: "{until}"},
					{As: "got", Req: Req{Method: POST, Read: true, URL: "/v2/invoicing/search-invoices", Query: Q{"page": "{page}", "page_size": "{limit}", "total_required": "true"},
						Body: Obj{"recipient_email": "{email}", "status": "{status}", "invoice_number": "{number}", "invoice_date_range": "{dates}"}}},
				},
				Out: Obj{"invoices": "{got/items|[]}", "pages": "{got/total_pages|1}"},
			}),
			"invoice": Call[paypalOne, map[string]any](Plan{
				Doc:   "One invoice by its id, like INV2-XXXX-XXXX-XXXX-XXXX, with its items, payments and refunds",
				Steps: []Step{{Req: Req{URL: "/v2/invoicing/invoices/{id}"}}},
			}),
			"disputes": Call[paypalDisputes, paypalDisputed](Plan{
				Doc:   "Disputes, newest first, maybe only in some states, like REQUIRED_ACTION or UNDER_PAYPAL_REVIEW: { disputes: [{ dispute_id, reason, status, dispute_amount }] } (pass page for more)",
				Clamp: map[string][2]int{"limit": {10, 50}},
				Steps: []Step{{As: "got", Req: Req{URL: "/v1/customer/disputes", Query: Q{"dispute_state": "{state}", "page_size": "{limit}", "page": "{page}"}}}},
				Out:   Obj{"disputes": "{got/items|[]}"},
			}),
			"dispute": Call[paypalOne, map[string]any](Plan{
				Doc:   "One dispute by its id, like PP-D-12345, with the transactions it is over, its messages, offers and evidence",
				Steps: []Step{{Req: Req{URL: "/v1/customer/disputes/{id}"}}},
			}),
			"draft": Call[paypalDraft, map[string]any](Plan{
				Doc: "Draft an invoice to an address for some items, maybe due on a day: the Invoice, a DRAFT that PayPal sends nobody until send. " +
					"Amounts are decimal strings, like 12.50",
				Steps: []Step{
					{Unless: "items", Fail: paypalItems},
					{Each: "items", As: "lines", In: Obj{"name": "{name}", "description": "{description}", "quantity": "{quantity|1}",
						"unit_amount": Obj{"currency_code": "{currency|USD}", "value": "{amount}"}}},
					{If: "due", As: "terms/due_date", In: "{due}"},
					{Req: Req{Method: POST, URL: "/v2/invoicing/invoices", Header: map[string][]string{"Prefer": {"return=representation"}},
						Body: Obj{"detail": Obj{"currency_code": "{currency|USD}", "note": "{note}", "payment_term": "{terms}"},
							"primary_recipients": []Obj{{"billing_info": Obj{"email_address": "{email}", "name": Obj{"given_name": "{first}", "surname": "{last}"}}}},
							"items":              "{lines}"}}},
				},
			}),
			"send": Call[paypalOne, paypalSent](Plan{
				Doc:   "Send a draft invoice, by its id, to its recipient by email, who pays it there, with a copy to the business: { id }",
				Steps: []Step{{Req: Req{Method: POST, URL: "/v2/invoicing/invoices/{id}/send", Body: Obj{"send_to_recipient": true, "send_to_invoicer": true}}}},
				Out:   Obj{"id": "{id}"},
			}),
		},
		Sync: []*Sync{
			{Name: "transactions", Gap: time.Second, Keep: true, Items: "transaction_details", Absent: "account_number", ID: "transaction_info/transaction_id", Updated: "transaction_info/transaction_updated_date",
				Parts: &Parts{Span: &Span{First: Shift{-3, 0, 1}, Then: Shift{0, 0, -31}, Step: Shift{0, 1, 0}}, ID: "{start}", Vals: Obj{"start": "{start}", "end": "{end}"}},
				Req: Req{URL: "/v1/reporting/transactions", Query: Q{"start_date": "{in/start}T00:00:00Z", "end_date": "{in/end}T23:59:59Z",
					"fields": "transaction_info,payer_info,cart_info", "page_size": "500", "page": "{page}"}},
				Page: Offset{From: 1, Size: 500, Step: 1},
				Deny: []Deny{{Status: []int{403}, Why: "the app may not search transactions; tick Transaction search among its features: {message}"}},
				Doc: "Every transaction of the last three years, by its id: what it is, by PayPal's event code, its amount, fee and status, and who paid. " +
					"Three years the first time, a month at a time, and then the last 31 days each sync; none is deleted.",
				To: Map{Table: "commerce_payment",
					Cols: map[string]SQL{"kind": "data#>>'{transaction_info,transaction_event_code}'", "status": "data#>>'{transaction_info,transaction_status}'",
						"amount": paypalMoney + "transaction_amount" + paypalMoneyOf, "fee": paypalMoney + "fee_amount" + paypalMoneyOf, "tip": paypalMoney + "tip_amount" + paypalMoneyOf,
						"method": "coalesce(nullif(data#>>'{transaction_info,instrument_type}', ''), data#>>'{transaction_info,payment_method_type}')",
						"customer_name": `coalesce(nullif(data#>>'{payer_info,payer_name,alternate_full_name}', ''),
							nullif(btrim(concat_ws(' ', data#>>'{payer_info,payer_name,given_name}', data#>>'{payer_info,payer_name,surname}')), ''))`,
						"email":     "archive.sync_email(data#>>'{payer_info,email_address}')",
						"reference": "data#>>'{transaction_info,invoice_id}'",
						"note":      "nullif(concat_ws(E'\\n\\n', nullif(btrim(data#>>'{transaction_info,transaction_subject}'), ''), nullif(btrim(data#>>'{transaction_info,transaction_note}'), '')), '')",
						"paid_at":   "archive.sync_at(data#>>'{transaction_info,transaction_initiation_date}')",
						"edited_at": "archive.sync_at(data#>>'{transaction_info,transaction_updated_date}')"},
					People: map[string]SQL{"customer": "data#>>'{payer_info,email_address}'"}}},
			{Name: "invoices", Gap: time.Second, Items: "items", Absent: "links", Updated: "detail/metadata/last_update_time",
				Req:  Req{URL: "/v2/invoicing/invoices", Query: Q{"page_size": "100", "page": "{page}"}},
				Page: Offset{From: 1, Size: 100, Step: 1},
				Deny: []Deny{{Status: []int{403}, Why: "the app may not read invoices; tick Invoicing among its features: {message}"}},
				Doc: "Every invoice, with who it is to, its total, what was paid and is due, and when, by its id: listed whole each sync, 100 a page, " +
					"which deletes those gone.",
				To: Map{Table: "commerce_invoice",
					Cols: map[string]SQL{"number": "data#>>'{detail,invoice_number}'", "title": "data#>>'{detail,reference}'", "status": "data->>'status'",
						"customer_name": `coalesce(nullif(data#>>'{primary_recipients,0,billing_info,name,full_name}', ''),
							nullif(btrim(concat_ws(' ', data#>>'{primary_recipients,0,billing_info,name,given_name}', data#>>'{primary_recipients,0,billing_info,name,surname}')), ''),
							data#>>'{primary_recipients,0,billing_info,business_name}')`,
						"email":     "archive.sync_email(data#>>'{primary_recipients,0,billing_info,email_address}')",
						"total":     "archive.sync_money(data#>>'{amount,value}', data#>>'{amount,currency_code}')",
						"paid":      "archive.sync_money(data#>>'{payments,paid_amount,value}', data#>>'{payments,paid_amount,currency_code}')",
						"due":       "archive.sync_money(data#>>'{due_amount,value}', data#>>'{due_amount,currency_code}')",
						"issued_on": "archive.sync_day(data#>>'{detail,invoice_date}')", "due_on": "archive.sync_day(data#>>'{detail,payment_term,due_date}')",
						"note": "data#>>'{detail,note}'", "url": "data#>>'{detail,metadata,recipient_view_url}'",
						"made_at": "archive.sync_at(data#>>'{detail,metadata,create_time}')", "edited_at": "archive.sync_at(data#>>'{detail,metadata,last_update_time}')"},
					People: map[string]SQL{"customer": "data#>>'{primary_recipients,0,billing_info,email_address}'"}}},
			// A listing pages no further than page 50, so an account's 2,500 newest disputes.
			{Name: "disputes", Gap: time.Second, Items: "items", Absent: "links", ID: "dispute_id", Updated: "update_time",
				Req:  Req{URL: "/v1/customer/disputes", Query: Q{"page_size": "50", "page": "{page}"}},
				Page: Offset{From: 1, Size: 50, Step: 1},
				Deny: []Deny{{Status: []int{403}, Why: "the app may not read disputes: {message}"}},
				Doc: "Every dispute a buyer opened, with its reason, status, stage, amount and when the business has to answer by, by its id: listed " +
					"whole each sync, 50 a page, up to the 2,500 newest, which deletes those gone.",
				To: Map{Table: "commerce_dispute",
					Cols: map[string]SQL{"reason": "data->>'reason'", "status": "data->>'status'", "stage": "data->>'dispute_life_cycle_stage'",
						"amount":     "archive.sync_money(data#>>'{dispute_amount,value}', data#>>'{dispute_amount,currency_code}')",
						"respond_by": "archive.sync_at(data->>'seller_response_due_date')",
						"made_at":    "archive.sync_at(data->>'create_time')", "edited_at": "archive.sync_at(data->>'update_time')"}}},
		},
	})
}

const (
	paypalItems = "an invoice needs items, each with a name and an amount"
	// A transaction's money: paypalMoney, which of transaction_info's, then paypalMoneyOf.
	paypalMoney   SQL = "archive.sync_money(data#>>'{transaction_info,"
	paypalMoneyOf SQL = ",value}', data#>>'{transaction_info,transaction_amount,currency_code}')"
)

type (
	paypalSearch struct {
		From  string `json:"from" jsonschema:"The first time, RFC 3339 with seconds, like 2026-10-01T00:00:00Z; within the last three years"`
		To    string `json:"to" jsonschema:"The last time, no more than 31 days after from"`
		ID    string `json:"id,omitempty" jsonschema:"Only this transaction, by its id"`
		Limit int    `json:"limit,omitempty" jsonschema:"At most this many a page, 1 to 500; 100 when left out"`
		Page  int    `json:"page,omitempty" jsonschema:"Which page, from 1"`
	}
	paypalFound struct {
		Transactions []map[string]any `json:"transactions"`
		Pages        int              `json:"pages" jsonschema:"How many pages there are"`
	}
	paypalInvoices struct {
		Email  string   `json:"email,omitempty" jsonschema:"Only those to this address"`
		Status []string `json:"status,omitempty" jsonschema:"Only those in these statuses, like DRAFT, SENT, UNPAID, PAID or CANCELLED"`
		Number string   `json:"number,omitempty" jsonschema:"Only the one of this invoice number"`
		Since  string   `json:"since,omitempty" jsonschema:"Only those dated on or after this day, YYYY-MM-DD; needs until"`
		Until  string   `json:"until,omitempty" jsonschema:"Only those dated on or before this day, YYYY-MM-DD; needs since"`
		Limit  int      `json:"limit,omitempty" jsonschema:"At most this many a page, 1 to 100; 20 when left out"`
		Page   int      `json:"page,omitempty" jsonschema:"Which page, from 1"`
	}
	paypalBilled struct {
		Invoices []map[string]any `json:"invoices"`
		Pages    int              `json:"pages" jsonschema:"How many pages there are"`
	}
	paypalDisputes struct {
		State string `json:"state,omitempty" jsonschema:"Only those in these states, comma separated: REQUIRED_ACTION, REQUIRED_OTHER_PARTY_ACTION, UNDER_PAYPAL_REVIEW, RESOLVED or OPEN_INQUIRIES"`
		Limit int    `json:"limit,omitempty" jsonschema:"At most this many a page, 1 to 50; 10 when left out"`
		Page  int    `json:"page,omitempty" jsonschema:"Which page, from 1 to 50"`
	}
	paypalDisputed struct {
		Disputes []map[string]any `json:"disputes"`
	}
	paypalOne struct {
		ID string `json:"id" jsonschema:"Its id"`
	}
	paypalDraft struct {
		Email    string        `json:"email" jsonschema:"Who it is to, by their address; PayPal emails them when it is sent"`
		First    string        `json:"first,omitempty" jsonschema:"Their first name"`
		Last     string        `json:"last,omitempty" jsonschema:"Their last name"`
		Items    []paypalDraws `json:"items" jsonschema:"What it is for"`
		Currency string        `json:"currency,omitempty" jsonschema:"Its currency, like USD; USD when left out"`
		Due      string        `json:"due,omitempty" jsonschema:"The day it is due, YYYY-MM-DD; due on receipt when left out"`
		Note     string        `json:"note,omitempty" jsonschema:"What it says to the recipient"`
	}
	paypalDraws struct {
		Name        string `json:"name" jsonschema:"What it is"`
		Description string `json:"description,omitempty" jsonschema:"More of what it is"`
		Quantity    string `json:"quantity,omitempty" jsonschema:"How many, as a decimal string; 1 when left out"`
		Amount      string `json:"amount" jsonschema:"What one costs, as a decimal string, like 12.50"`
	}
	paypalSent struct {
		ID string `json:"id"`
	}
)
