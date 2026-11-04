package connect

import "time"

// Plaid, which reads a bank login's accounts and transactions for apps: an owner connects the business's bank
// for everyone, through Plaid Link, Plaid's own widget, where they pick the bank and sign in to it, never to
// Archive. ARCHIVE_PLAID is https://<client id>:<secret>@production.plaid.com, the keys of Mulholland's Plaid team;
// @sandbox.plaid.com with its Sandbox secret reads Plaid's test banks instead. Every call is a POST with the
// client in its headers.
//
// Link starts with a link token for Transactions alone, two years of it, with Archive's push URL as its webhook
// and the bank's integrations page as its redirect URI, which Plaid's dashboard must list, for the banks, like
// Chase, whose sign-in is their own (OAuth) and sends the browser back there. Link hands the app a public token,
// which connecting trades for the login's access token and item id. Only reads are ever asked for: no transfer,
// payment or Auth, so Archive never sees an account number. Disconnecting removes the login at Plaid, which
// ends its access token and Plaid's monthly charge for it.
//
// It syncs the login's accounts with their balances, as Plaid last refreshed them (/accounts/get, which Plaid does
// not charge for, unlike /accounts/balance/get), listed whole each sync; and its transactions, through
// /transactions/sync: all of them the first time, 500 a page, and then from the cursor the last sync ended at,
// what was added, changed and removed since. A removed transaction is only its ids. A change while a sync pages
// (TRANSACTIONS_SYNC_MUTATION_DURING_PAGINATION, a 400) starts it again from where it began. Plaid's webhooks,
// signed with a JWS, name the item_id, and SYNC_UPDATES_AVAILABLE among them is new transactions.
//
// Its actions only read, from what Plaid already holds and does not charge for by the call: the login's state, its
// accounts, and transactions between two dates. None asks Plaid to refresh anything.
func init() {
	Register(&Connector{
		ID: "plaid", Name: "Plaid", Integration: true, Env: "ARCHIVE_PLAID", Host: "https://production.plaid.com",
		Options: "<client id>:<secret> as its server key, or @sandbox.plaid.com with the Sandbox secret",
		Info: Info{By: "Plaid", Description: "Read the business’s bank accounts, balances and transactions, Zelle and Venmo payments among them", Category: CatFinance, Icon: IconCard,
			Scopes: []string{"Read one bank login’s accounts, balances and transactions", "Never move money"}},
		API:  "https://production.plaid.com",
		Keys: []Key{{Name: "public_token", Label: "Bank", Secret: true}},
		Link: &Link{
			Start: Req{Method: POST, Read: true, URL: "/link/token/create", Body: Obj{"client_name": "Archive", "language": "en", "country_codes": []string{"US"},
				"user": Obj{"client_user_id": "{user}"}, "products": []string{"transactions"}, "transactions": Obj{"days_requested": 730},
				"webhook": "{hook}", "redirect_uri": "{back}"}},
			Token: "link_token",
			Swap:  Req{Method: POST, Read: true, URL: "/item/public_token/exchange", Body: Obj{"public_token": "{key/public_token}"}},
			Keep:  map[string]Path{"access_token": "access_token", "item_id": "item_id"},
			Meta:  []string{"item_id"},
		},
		Sign:     map[string]string{"PLAID-CLIENT-ID": "{client/id}", "PLAID-SECRET": "{client/secret}", "Plaid-Version": "2020-09-14"},
		Who:      &Who{Req: Req{Method: POST, Read: true, URL: "/item/get", Body: plaidLogin}, Names: []Path{"item/institution_name", "item/institution_id"}},
		RevokeBy: &Req{Method: POST, URL: "https://production.plaid.com/item/remove", Body: Obj{"client_id": "{client/id}", "secret": "{client/secret}", "access_token": "{key/access_token}"}},
		Listen: &Listen{Key: "{meta/item_id}", Keys: []Path{"item_id"},
			Signs: []Sign{{Header: "Plaid-Verification", Digest: SHA256, Enc: Hex, Of: "{body}", JWK: "key", Claim: "request_body_sha256",
				Key: &Req{Method: POST, Read: true, URL: "/webhook_verification_key/get", Body: Obj{"key_id": "{kid}"}}}}},
		Actions: map[string]*Action{
			"item": Call[struct{}, plaidItem](Plan{
				Doc:   "The bank login: { institution, products, consent_expires, error, status: { transactions: { last_successful_update, last_failed_update } } }",
				Steps: []Step{{As: "got", Req: Req{Method: POST, Read: true, URL: "/item/get", Body: plaidLogin}}},
				Out:   Obj{"institution": "{got/item/institution_name}", "products": "{got/item/products|[]}", "consent_expires": "{got/item/consent_expiration_time}", "error": "{got/item/error}", "status": "{got/status}"},
			}),
			"accounts": Call[struct{}, []map[string]any](Plan{
				Doc:   "The login's accounts, with their balances as Plaid last refreshed them: [{ account_id, name, official_name, mask, type, subtype, balances: { current, available, limit, iso_currency_code } }]",
				Steps: []Step{{As: "got", Req: Req{Method: POST, Read: true, URL: "/accounts/get", Body: plaidLogin}}},
				Out:   "{got/accounts|[]}",
			}),
			"transactions": Call[plaidFind, plaidFound](Plan{
				Doc: "Transactions between two dates, newest first, maybe only of some accounts, up to 500 a page: { total, transactions: [{ transaction_id, account_id, date, amount, " +
					"iso_currency_code, name, merchant_name, original_description, pending, ... }] } (a positive amount leaves the account; pass offset for more, until total)",
				Clamp: map[string][2]int{"count": {100, 500}, "offset": {0, 1 << 30}},
				Steps: []Step{{As: "got", Req: Req{Method: POST, Read: true, URL: "/transactions/get", Body: Obj{"access_token": "{key/access_token}", "start_date": "{start}", "end_date": "{end}",
					"options": Obj{"count": "{count}", "offset": "{offset}", "account_ids": "{accounts}", "include_original_description": true}}}}},
				Out: Obj{"total": "{got/total_transactions|0}", "transactions": "{got/transactions|[]}"},
			}),
		},
		Sync: []*Sync{
			{Name: "accounts", Gap: time.Second, Req: Req{Method: POST, Read: true, URL: "/accounts/get", Body: plaidLogin}, Items: "accounts", ID: "account_id",
				Doc: "Every account of the bank login, with its balances as Plaid last refreshed them, by its account_id: listed whole each sync.",
				To: Map{Table: "bank_account",
					Cols: map[string]SQL{"title": "data->>'name'", "official_name": "data->>'official_name'", "mask": "data->>'mask'", "type": "data->>'type'", "subtype": "data->>'subtype'",
						"balance": "archive.sync_money(data#>>'{balances,current}', " + plaidBalanceCurrency + ")", "available": "archive.sync_money(data#>>'{balances,available}', " + plaidBalanceCurrency + ")",
						"credit_limit": "archive.sync_money(data#>>'{balances,limit}', " + plaidBalanceCurrency + ")"}}},
			{Name: "transactions", Gap: time.Second, Req: plaidList,
				Items: "added+modified+removed", ID: "transaction_id", Page: Cursor{At: "next_cursor", More: "has_more"},
				Change: Resume{At: "next_cursor", Req: plaidDelta},
				Gone:   []Test{{At: "date", Off: true}}, Restart: []int{400},
				Doc: "Every transaction of the bank login's accounts, pending ones too, by its transaction_id: all of them the first time, 500 a page, and then what " +
					"was added, changed and removed since the cursor the last sync ended at. A pending one that posts is removed, and comes again posted.",
				To: Map{Table: "bank_transaction",
					Cols: map[string]SQL{"posted_on": "archive.sync_day(data->>'date')", "authorized_on": "archive.sync_day(data->>'authorized_date')",
						"amount":      "archive.sync_money((-archive.sync_num(data->>'amount'))::text, coalesce(nullif(data->>'iso_currency_code', ''), 'USD'))",
						"description": "coalesce(nullif(data->>'original_description', ''), data->>'name')", "merchant": "coalesce(nullif(data->>'merchant_name', ''), data->>'name')",
						"counterparty": "(SELECT string_agg(c->>'name', ', ') FROM archive.sync_list(data->'counterparties') c WHERE nullif(c->>'name', '') IS NOT NULL)",
						"channel":      "data->>'payment_channel'", "category": "data#>>'{personal_finance_category,primary}'", "detail": "data#>>'{personal_finance_category,detailed}'",
						"pending": "archive.sync_bool(data->'pending')", "check_number": "data->>'check_number'"},
					Orgs: map[string]SQL{"org": "coalesce(nullif(data->>'website', ''), (SELECT c->>'website' FROM archive.sync_list(data->'counterparties') c WHERE c->>'type' = 'merchant' LIMIT 1))"},
					Refs: map[string]SQL{"account": "data->>'account_id'"}}},
		},
	})
}

// A balance's currency, US dollars when Plaid names none.
const plaidBalanceCurrency = "coalesce(nullif(data#>>'{balances,iso_currency_code}', ''), 'USD')"

var plaidLogin = Obj{"access_token": "{key/access_token}"}

// Transactions from a cursor, with the statement line as the bank wrote it beside Plaid's cleaned-up name: none for
// the first page of a listing, the page's for the next, and the last sync's for a delta's first.
var (
	plaidList = Req{Method: POST, Read: true, URL: "/transactions/sync",
		Body: Obj{"access_token": "{key/access_token}", "cursor": "{page}", "count": 500, "options": Obj{"include_original_description": true}}}
	plaidDelta = Req{Method: POST, Read: true, URL: "/transactions/sync",
		Body: Obj{"access_token": "{key/access_token}", "cursor": "{page;token}", "count": 500, "options": Obj{"include_original_description": true}}}
)

type (
	plaidItem struct {
		Institution    string   `json:"institution,omitempty"`
		Products       []string `json:"products"`
		ConsentExpires string   `json:"consent_expires,omitempty"`
		Error          any      `json:"error,omitempty"`
		Status         any      `json:"status,omitempty"`
	}
	plaidFind struct {
		Start    string   `json:"start" jsonschema:"The first date, YYYY-MM-DD"`
		End      string   `json:"end" jsonschema:"The last date, YYYY-MM-DD"`
		Count    int      `json:"count,omitempty" jsonschema:"At most this many, 1 to 500; 100 when left out"`
		Offset   int      `json:"offset,omitempty" jsonschema:"How many to skip, for the next page"`
		Accounts []string `json:"accounts,omitempty" jsonschema:"Only these account_ids, from accounts"`
	}
	plaidFound struct {
		Total        int              `json:"total"`
		Transactions []map[string]any `json:"transactions"`
	}
)
