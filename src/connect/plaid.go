package connect

import "time"

func init() {
	Register(&Connector{
		ID: "plaid", Name: "Plaid", Integration: true, Env: "ARCHIVE_PLAID", Host: "https://production.plaid.com",
		Options: "<client id>:<secret> as its server key, or @sandbox.plaid.com with the Sandbox secret",
		Info: Info{By: "Plaid", Description: "Read bank, brokerage, retirement and loan accounts: balances, transactions, holdings and loan terms, Zelle and Venmo payments among them", Category: CatFinance, Icon: IconCard,
			Scopes: []string{"Read one login’s accounts, balances and transactions", "Read its investment holdings and trades, and its card, student loan and mortgage terms, where the institution has them", "Never trade or move money"}},
		API:  "https://production.plaid.com",
		Keys: []Key{{Name: "public_token", Label: "Bank", Secret: true}},
		Link: &Link{
			Start: Req{Method: POST, Read: true, URL: "/link/token/create", Body: Obj{"client_name": "Archive", "language": "en", "country_codes": []string{"US"},
				"user": Obj{"client_user_id": "{user}"}, "products": []string{"transactions"}, "optional_products": []string{"investments", "liabilities"}, "transactions": Obj{"days_requested": 730},
				"webhook": "{hook}", "redirect_uri": "{back}"}},
			Token: "link_token",
			Swap:  Req{Method: POST, Read: true, URL: "/item/public_token/exchange", Body: Obj{"public_token": "{key/public_token}"}},
			Keep:  map[string]Path{"access_token": "access_token", "item_id": "item_id"},
			Meta:  []string{"item_id"},
		},
		Sign:     map[string]string{"PLAID-CLIENT-ID": "{client/id}", "PLAID-SECRET": "{client/secret}", "Plaid-Version": "2020-09-14"},
		Who:      &Who{Req: Req{Method: POST, Read: true, URL: "/item/get", Body: plaidLogin}, Names: []Path{"item/institution_name", "item/institution_id"}},
		Answer:   plaidSecurities,
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
			"holdings": Call[struct{}, []map[string]any](Plan{
				Doc: "What the login's investment accounts hold, as Plaid last refreshed it: [{ account_id, quantity, institution_price, institution_value, cost_basis, " +
					"iso_currency_code, security: { name, ticker_symbol, cusip, isin, type } }]",
				Steps: []Step{{As: "got", Req: Req{Method: POST, Read: true, URL: "/investments/holdings/get", Body: plaidLogin}}},
				Out:   "{got/holdings|[]}",
			}),
			"liabilities": Call[struct{}, map[string]any](Plan{
				Doc: "The terms of the login's credit cards, student loans and mortgages: { credit: [{ account_id, aprs, minimum_payment_amount, next_payment_due_date, " +
					"last_statement_balance, ... }], student: [...], mortgage: [...] }",
				Steps: []Step{{As: "got", Req: Req{Method: POST, Read: true, URL: "/liabilities/get", Body: plaidLogin}}},
				Out:   "{got/liabilities|{}}",
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
			{Name: "holdings", Gap: time.Second, Req: Req{Method: POST, Read: true, URL: "/investments/holdings/get", Body: plaidLogin}, Items: "holdings",
				Key: []Reads{{Of: "{account_id}/{security_id}"}}, Deny: plaidLacks,
				Doc: "What each investment account holds, by its account_id and security_id, with the security's name, ticker and type, quantity, price, value " +
					"and cost basis as Plaid last refreshed them: listed whole each sync, so one sold is deleted.",
				To: Map{Table: "investment_holding",
					Cols: map[string]SQL{"security": "data#>>'{security,name}'", "ticker": "data#>>'{security,ticker_symbol}'", "cusip": "data#>>'{security,cusip}'",
						"isin": "data#>>'{security,isin}'", "kind": "data#>>'{security,type}'", "quantity": "archive.sync_num(data->>'quantity')",
						"price": plaidMoney("institution_price"), "priced_on": "archive.sync_day(coalesce(data->>'institution_price_as_of', data->>'institution_price_datetime'))",
						"value": plaidMoney("institution_value"), "cost_basis": plaidMoney("cost_basis")},
					Refs: map[string]SQL{"account": "data->>'account_id'"}}},
			{Name: "investment_transactions", Gap: time.Second, Parts: plaidYears, Keep: true, Items: "investment_transactions", ID: "investment_transaction_id",
				Page: Offset{Size: 500, Step: 500}, Deny: plaidLacks,
				Req: Req{Method: POST, Read: true, URL: "/investments/transactions/get", Body: Obj{"access_token": "{key/access_token}", "start_date": "{in/start}", "end_date": "{in/end}",
					"options": Obj{"count": 500, "offset": "{#page|0}"}}},
				Doc: "Every buy, sell, dividend, fee, contribution and transfer of the investment accounts, by its investment_transaction_id: a year at a time " +
					"from two years back the first time, and after that the last 30 days each sync, 500 a page, which deletes nothing.",
				To: Map{Table: "investment_transaction",
					Cols: map[string]SQL{"traded_on": "archive.sync_day(data->>'date')", "kind": "data->>'type'", "subtype": "data->>'subtype'", "description": "data->>'name'",
						"security": "data#>>'{security,name}'", "ticker": "data#>>'{security,ticker_symbol}'", "quantity": "archive.sync_num(data->>'quantity')",
						"price": plaidMoney("price"), "fees": plaidMoney("fees"),
						"amount": "CASE WHEN coalesce(data->>'unofficial_currency_code', '') = '' THEN archive.sync_money((-archive.sync_num(data->>'amount'))::text, coalesce(nullif(data->>'iso_currency_code', ''), 'USD')) END"},
					Refs: map[string]SQL{"account": "data->>'account_id'"}}},
			{Name: "liabilities", Gap: time.Second, Req: Req{Method: POST, Read: true, URL: "/liabilities/get", Body: plaidLogin},
				Items: "liabilities/credit+liabilities/mortgage+liabilities/student", ID: "account_id", Deny: plaidLacks,
				Doc: "The terms of each credit card, mortgage and student loan, by its account_id: its rate, minimum payment and due date, last payment and statement, " +
					"and a loan's origination, maturity, escrow and what was paid this year: listed whole each sync.",
				To: Map{Table: "bank_liability",
					Cols: map[string]SQL{"kind": "CASE WHEN data ? 'aprs' THEN 'credit' WHEN data ? 'escrow_balance' OR data ? 'property_address' THEN 'mortgage' ELSE 'student' END",
						"loan_name": "coalesce(nullif(data->>'loan_name', ''), data->>'loan_type_description')", "status": "data#>>'{loan_status,type}'",
						"interest_rate": "coalesce(archive.sync_num(data->>'interest_rate_percentage'), archive.sync_num(data#>>'{interest_rate,percentage}'), " +
							"(SELECT archive.sync_num(a->>'apr_percentage') FROM archive.sync_list(data->'aprs') a ORDER BY a->>'apr_type' <> 'purchase_apr' LIMIT 1))",
						"minimum_payment": "coalesce(archive.sync_money(data->>'minimum_payment_amount', 'USD'), archive.sync_money(data->>'next_monthly_payment', 'USD'))",
						"next_due_on":     "archive.sync_day(data->>'next_payment_due_date')", "last_payment": "archive.sync_money(data->>'last_payment_amount', 'USD')",
						"last_paid_on": "archive.sync_day(data->>'last_payment_date')", "statement_balance": "archive.sync_money(data->>'last_statement_balance', 'USD')",
						"statement_on": "archive.sync_day(data->>'last_statement_issue_date')", "overdue": "archive.sync_bool(data->'is_overdue')",
						"past_due": "archive.sync_money(data->>'past_due_amount', 'USD')", "originated_on": "archive.sync_day(data->>'origination_date')",
						"original_principal": "archive.sync_money(data->>'origination_principal_amount', 'USD')",
						"matures_on":         "coalesce(archive.sync_day(data->>'maturity_date'), archive.sync_day(data->>'expected_payoff_date'))",
						"escrow_balance":     "archive.sync_money(data->>'escrow_balance', 'USD')", "interest_paid_ytd": "archive.sync_money(data->>'ytd_interest_paid', 'USD')",
						"principal_paid_ytd": "archive.sync_money(data->>'ytd_principal_paid', 'USD')"},
					Refs: map[string]SQL{"account": "data->>'account_id'"}}},
		},
	})
}

const plaidBalanceCurrency = "coalesce(nullif(data#>>'{balances,iso_currency_code}', ''), 'USD')"

var (
	plaidLogin = Obj{"access_token": "{key/access_token}"}
	plaidYears = &Parts{Span: &Span{First: Shift{-2, 0, 0}, Step: Shift{1, 0, 0}, Then: Shift{0, 0, -30}}, ID: "{start}", Vals: Obj{"start": "{start}", "end": "{end}"}}
	plaidLacks = []Deny{{Status: []int{400}, Says: []string{"not supported", "no investment", "no liabilit", "consent"}, Why: "Plaid has no {stream} for this login: {message}"}}
)

// plaidMoney is a field of an investment answer as money, or nothing when Plaid prices it in a currency with no ISO code, like a coin.
func plaidMoney(field string) SQL {
	return SQL("CASE WHEN coalesce(data->>'unofficial_currency_code', '') = '' THEN archive.sync_money(data->>'" + field + "', coalesce(nullif(data->>'iso_currency_code', ''), 'USD')) END")
}

// plaidSecurities puts each holding's and investment transaction's security into it, from the securities beside them.
var plaidSecurities = &Codec{"plaid_securities", func(v any) (any, error) {
	m, _ := v.(map[string]any)
	secs, _ := m["securities"].([]any)
	if secs == nil {
		return v, nil
	}
	by := map[string]any{}
	for _, s := range secs {
		by[text(pick(s, "security_id"))] = s
	}
	for _, at := range []string{"holdings", "investment_transactions"} {
		items, _ := m[at].([]any)
		for _, it := range items {
			if x, ok := it.(map[string]any); ok {
				if s, ok := by[text(x["security_id"])]; ok {
					x["security"] = s
				}
			}
		}
	}
	return v, nil
}}

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
