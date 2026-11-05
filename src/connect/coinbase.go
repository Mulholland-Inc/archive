package connect

import (
	"time"

	"golang.org/x/oauth2"
)

func init() {
	Register(&Connector{
		ID: "coinbase", Name: "Coinbase",
		Info: Info{By: "Coinbase", Description: "Read your crypto and cash wallets, what they hold and every send, receive, buy and sell", Category: CatFinance, Icon: IconCard,
			Scopes: []string{"Read your wallets and their balances", "Read your transactions", "Never trade, send or move money"}},
		API: "https://api.coinbase.com",
		Endpoint: oauth2.Endpoint{AuthURL: "https://login.coinbase.com/oauth2/auth", TokenURL: "https://login.coinbase.com/oauth2/token",
			AuthStyle: oauth2.AuthStyleInParams},
		RevokeBy: &Req{Method: POST, URL: "https://login.coinbase.com/oauth2/revoke", Form: Q{"client_id": "{client/id}", "client_secret": "{client/secret}", "token": "{access}"}},
		Scopes:   []string{"wallet:user:read", "wallet:accounts:read", "wallet:transactions:read", "offline_access"},
		Comma:    true,
		Params:   map[string]string{"account": "all"},
		Sign:     map[string]string{"Authorization": "Bearer {token}", "CB-VERSION": cbVersion},
		Who:      &Who{Req: Req{URL: "/v2/user"}, Names: []Path{"data/name", "data/username", "data/id"}},
		Actions: map[string]*Action{
			"get": Call[cbGet, any](Plan{
				Doc: "Read one of Coinbase's v2 GET endpoints the connection's scopes allow, like { path: \"/v2/accounts\" }, { path: \"/v2/accounts/<id>/transactions\" } " +
					"or { path: \"/v2/accounts/<id>/transactions/<id>\" }: its answer, { data, pagination: { next_uri } } for a list (pass pagination.next_uri as path for more). It only reads",
				Steps: []Step{
					{Only: Test{At: "path", Re: `^/v2/(user|accounts)(/|\?|$)`, Off: true}, Fail: "Coinbase reads only /v2/user and /v2/accounts and what is under it"},
					{Req: Req{URL: "{+path}?{+query|}"}},
				},
			}),
		},
		Sync: []*Sync{
			{Name: "wallets", Gap: time.Second, Req: Req{URL: "/v2/accounts", Query: cbAll}, Items: "data", Page: cbPage,
				Doc: "Every wallet, a coin's or a currency's, by its id, with its balance and that balance's worth in your own currency: listed whole each sync, 100 a page, so one gone is deleted.",
				To: Map{Table: "bank_account",
					Cols: map[string]SQL{"title": "data->>'name'", "type": "CASE WHEN data->>'type' = 'fiat' THEN 'depository' ELSE 'crypto' END", "subtype": cbCoin,
						"balance": cbWorth}}},
			{Name: "holdings", Gap: time.Second, Req: Req{URL: "/v2/accounts", Query: cbAll}, Items: "data", Page: cbPage,
				Skip: []Test{{At: "balance/amount", Re: `^-?0*(\.0*)?$`}},
				Doc:  "What each wallet that is not empty holds, by the wallet's id: the coin or currency, how much, and its worth in your own currency: listed whole each sync, so one emptied is deleted.",
				To: Map{Table: "investment_holding",
					Cols: map[string]SQL{"security": "coalesce(data#>>'{currency,name}', " + cbCoin + ")", "ticker": cbCoin,
						"kind": "CASE WHEN data->>'type' = 'fiat' THEN 'cash' ELSE 'cryptocurrency' END", "quantity": "archive.sync_num(data#>>'{balance,amount}')",
						"value": cbWorth, "priced_on": "archive.sync_day(data->>'updated_at')"},
					Refs: map[string]SQL{"account": "data->>'id'"}}},
			{Name: "transactions", Gap: time.Second, Parts: cbWallets, Req: Req{URL: "/v2/accounts/{part}/transactions", Query: cbAll}, Items: "data", Page: cbPage,
				Updated: "created_at", Change: NewestFirst{}, Set: Obj{"account_id": "{part}"},
				Doc: "Every transaction of each wallet, by its id: a buy, sell, conversion, send, receive, deposit, withdrawal, reward or fee, with how much of the coin and its worth then: " +
					"listed whole the first time and a week after, 100 a page and a wallet at a time, which deletes those gone, and in between the newest until one already read.",
				To: Map{Table: "investment_transaction",
					Cols: map[string]SQL{"traded_on": "archive.sync_day(data->>'created_at')", "kind": "data->>'type'", "subtype": "data->>'status'",
						"description": "nullif(btrim(concat_ws(' ', data#>>'{details,title}', data#>>'{details,subtitle}')), '')", "security": "data#>>'{amount,currency}'",
						"ticker": "data#>>'{amount,currency}'", "quantity": "archive.sync_num(data#>>'{amount,amount}')",
						"amount": "CASE WHEN data#>>'{native_amount,currency}' ~ '^[A-Z]{3}$' THEN archive.sync_money(data#>>'{native_amount,amount}', data#>>'{native_amount,currency}') END",
						"fees":   "CASE WHEN data#>>'{network,transaction_fee,currency}' ~ '^[A-Z]{3}$' AND data#>>'{network,transaction_fee,currency}' = data#>>'{native_amount,currency}' THEN archive.sync_money(data#>>'{network,transaction_fee,amount}', data#>>'{network,transaction_fee,currency}') END"},
					Refs: map[string]SQL{"account": "data->>'account_id'"}}},
		},
	})
}

const (
	cbVersion = "2024-10-01"
	// cbCoin is a wallet's coin or currency code, an object in newer versions of the API and a string in older ones.
	cbCoin SQL = "coalesce(data#>>'{currency,code}', data->>'currency')"
	// cbWorth is a wallet's balance in the person's own currency, or a fiat wallet's own balance.
	cbWorth SQL = "coalesce(CASE WHEN data#>>'{native_balance,currency}' ~ '^[A-Z]{3}$' THEN archive.sync_money(data#>>'{native_balance,amount}', data#>>'{native_balance,currency}') END, " +
		"CASE WHEN data->>'type' = 'fiat' AND data#>>'{balance,currency}' ~ '^[A-Z]{3}$' THEN archive.sync_money(data#>>'{balance,amount}', data#>>'{balance,currency}') END)"
)

var (
	cbAll     = Q{"limit": "100"}
	cbPage    = NextURL{At: "pagination/next_uri"}
	cbWallets = &Parts{Req: Req{URL: "/v2/accounts", Query: cbAll}, Items: "data", Page: cbPage}
)

type cbGet struct {
	Path  string `json:"path" jsonschema:"The endpoint, like /v2/accounts, /v2/accounts/<id>/transactions or a pagination.next_uri"`
	Query string `json:"query,omitempty" jsonschema:"Its query, like limit=100&order=asc"`
}
