package connect

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

func init() {
	Register(&Connector{
		ID: "gemini", Name: "Gemini",
		Info: Info{By: "Gemini", Description: "Read your Gemini exchange balances, staking, trades, deposits and withdrawals", Category: CatFinance, Icon: IconCard,
			Scopes: []string{"Read balances, staking, trades and transfers with a read-only Auditor key", "Never trade, send or move money"}},
		API: "https://api.gemini.com",
		Keys: []Key{
			{Name: "key", Label: "API key", Secret: true,
				Help: "In Gemini, Settings, API, Create API key: scope Primary, role Auditor only, and leave “Uses a time based nonce” off. Its key starts with account-"},
			{Name: "secret", Label: "API secret", Secret: true, Help: "The secret Gemini shows once, beside the key"},
		},
		Authorize: geminiSign,
		Answer:    geminiPaged,
		Who:       &Who{Req: Req{Method: POST, Read: true, URL: "/v1/balances"}, Names: []Path{"accountName"}},
		Actions: map[string]*Action{
			"balances": Call[struct{}, []map[string]any](Plan{
				Doc:   "What the exchange account holds of each currency, with its worth in US dollars: [{ currency, amount, available, amountNotional, availableNotional }]",
				Steps: []Step{{As: "got", Req: Req{Method: POST, Read: true, URL: "/v1/notionalbalances/usd"}}},
				Out:   "{got/items|[]}",
			}),
			"trades": Call[geminiFind, []map[string]any](Plan{
				Doc: "Your trades of one pair, like btcusd, on or after a time, oldest of them first, up to 500: [{ tid, price, amount, fee_amount, fee_currency, type, timestamp, ... }] " +
					"(pass the last one's timestamp + 1 as since for more)",
				Steps: []Step{{As: "got", Req: Req{Method: POST, Read: true, URL: "/v1/mytrades", Body: Obj{"symbol": "{symbol}", "limit_trades": 500, "timestamp": "{#since|0}"}}}},
				Out:   "{got/items|[]}",
			}),
		},
		Sync: []*Sync{
			{Name: "balances", Gap: time.Second, Req: Req{Method: POST, Read: true, URL: "/v1/notionalbalances/usd"}, Items: "items", ID: "currency",
				Skip: []Test{{At: "amount", Re: `^-?0*(\.0*)?$`}},
				Doc:  "What the exchange account holds of each currency, by the currency, with its worth in US dollars: listed whole each sync, so one sold off is deleted.",
				To: Map{Table: "investment_holding",
					Cols: map[string]SQL{"security": "data->>'currency'", "ticker": "data->>'currency'",
						"kind":     "CASE WHEN data->>'currency' IN ('USD', 'EUR', 'GBP', 'CAD', 'SGD', 'AUD', 'HKD') THEN 'cash' ELSE 'cryptocurrency' END",
						"quantity": "archive.sync_num(data->>'amount')", "value": "archive.sync_money(data->>'amountNotional', 'USD')"}}},
			{Name: "staking", Gap: time.Second, Req: Req{Method: POST, Read: true, URL: "/v1/balances/staking"}, Items: "items", Key: []Reads{{Of: "staking/{currency}"}},
				Skip: []Test{{At: "balance", Re: `^-?0*(\.0*)?$`}}, Deny: geminiDeny,
				Doc: "What is staked of each currency, by the currency: listed whole each sync, so one unstaked is deleted.",
				To: Map{Table: "investment_holding",
					Cols: map[string]SQL{"security": "(data->>'currency') || ' staked'", "ticker": "data->>'currency'", "kind": "'cryptocurrency'",
						"quantity": "archive.sync_num(coalesce(data->>'balance', data->>'amount'))"}}},
			{Name: "trades", Gap: time.Second, Parts: geminiPairs, Items: "items", ID: "tid", Page: geminiPage,
				Req: Req{Method: POST, Read: true, URL: "/v1/mytrades", Body: Obj{"symbol": "{part}", "limit_trades": 500, "timestamp": "{#page|0}"}},
				Set: Obj{"symbol": "{part}"},
				Doc: "Every trade of each currency held now against US dollars, like btcusd, by its tid, with its price, amount and fee: listed whole each sync, oldest first, " +
					"500 a page and a pair at a time.",
				To: Map{Table: "investment_transaction",
					Cols: map[string]SQL{"traded_on": "archive.sync_at(data->>'timestampms')::date", "kind": "lower(data->>'type')", "description": "upper(data->>'symbol')",
						"security": geminiCoin, "ticker": geminiCoin,
						"quantity": "CASE WHEN data->>'type' = 'Sell' THEN -archive.sync_num(data->>'amount') ELSE archive.sync_num(data->>'amount') END",
						"price":    "archive.sync_money(data->>'price', 'USD')",
						"amount":   "archive.sync_money((CASE WHEN data->>'type' = 'Sell' THEN 1 ELSE -1 END * archive.sync_num(data->>'price') * archive.sync_num(data->>'amount'))::text, 'USD')",
						"fees":     "CASE WHEN data->>'fee_currency' ~ '^[A-Z]{3}$' THEN archive.sync_money(data->>'fee_amount', data->>'fee_currency') END"}}},
			{Name: "transfers", Gap: time.Second, Items: "items", ID: "eid", Page: geminiPage,
				Req: Req{Method: POST, Read: true, URL: "/v2/transfers", Body: Obj{"limit_transfers": 50, "timestamp": "{#page|0}"}},
				Doc: "Every deposit and withdrawal, by its eid, with its currency, amount and status: listed whole each sync, oldest first, 50 a page.",
				To: Map{Table: "investment_transaction",
					Cols: map[string]SQL{"traded_on": "archive.sync_at(data->>'timestampms')::date", "kind": "lower(data->>'type')", "subtype": "lower(data->>'status')",
						"description": "coalesce(nullif(data->>'purpose', ''), data->>'method')", "security": "data->>'currency'", "ticker": "data->>'currency'",
						"quantity": "CASE WHEN data->>'type' = 'Withdrawal' THEN -archive.sync_num(data->>'amount') ELSE archive.sync_num(data->>'amount') END",
						"amount":   "CASE WHEN data->>'currency' = 'USD' THEN archive.sync_money((CASE WHEN data->>'type' = 'Withdrawal' THEN -1 ELSE 1 END * archive.sync_num(data->>'amount'))::text, 'USD') END"}}},
		},
	})
}

const geminiCoin SQL = "upper(regexp_replace(data->>'symbol', 'usd$', ''))"

var (
	geminiPage  = Cursor{At: "after", More: "more"}
	geminiPairs = &Parts{Req: Req{Method: POST, Read: true, URL: "/v1/balances"}, Items: "items", Skip: []Test{{At: "currency", Is: "USD"}}, ID: "{-currency}usd"}
	geminiDeny  = []Deny{{Status: []int{400, 403}, Why: "Gemini does not let this key read {stream}: {message}"}}
)

type geminiFind struct {
	Symbol string `json:"symbol" jsonschema:"The pair, like btcusd or ethusd"`
	Since  int64  `json:"since,omitempty" jsonschema:"Only trades on or after this time, in Unix seconds"`
}

var geminiNonce atomic.Int64

// geminiSign moves a request's JSON body into Gemini's signed payload: the body, the path and a nonce that only grows, in base64,
// signed with the key's secret by HMAC-SHA384. The request itself goes with no body.
func geminiSign(r *http.Request, c *Client) {
	payload := map[string]any{}
	if r.Body != nil {
		b, _ := io.ReadAll(r.Body)
		r.Body.Close()
		json.Unmarshal(b, &payload)
	}
	n := time.Now().UnixMilli()
	for {
		last := geminiNonce.Load()
		if n <= last {
			n = last + 1
		}
		if geminiNonce.CompareAndSwap(last, n) {
			break
		}
	}
	payload["request"], payload["nonce"] = r.URL.Path, strconv.FormatInt(n, 10)
	j, _ := json.Marshal(payload)
	enc := base64.StdEncoding.EncodeToString(j)
	mac := hmac.New(sha512.New384, []byte(c.keys["secret"]))
	mac.Write([]byte(enc))
	r.Body, r.ContentLength, r.GetBody = http.NoBody, 0, nil
	r.Header.Set("Content-Type", "text/plain")
	r.Header.Set("Cache-Control", "no-cache")
	r.Header.Set("X-GEMINI-APIKEY", c.keys["key"])
	r.Header.Set("X-GEMINI-PAYLOAD", enc)
	r.Header.Set("X-GEMINI-SIGNATURE", hex.EncodeToString(mac.Sum(nil)))
}

// geminiPaged wraps Gemini's list answers as { items, after, more }: after is a second past the newest timestamp in the list,
// where the next page of trades or transfers starts, and more is whether the list had any.
var geminiPaged = &Codec{"gemini_paged", func(v any) (any, error) {
	list, ok := v.([]any)
	if !ok {
		return v, nil
	}
	var newest int64
	for _, it := range list {
		ms, _ := strconv.ParseInt(text(pick(it, "timestampms")), 10, 64)
		newest = max(newest, ms)
	}
	out := map[string]any{"items": list, "more": newest > 0}
	if newest > 0 {
		out["after"] = strconv.FormatInt(newest/1000+1, 10)
	}
	return out, nil
}}
