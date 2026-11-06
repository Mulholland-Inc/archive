package connect

import (
	"strings"
	"time"
)

// Monarch's only official API is its MCP server, which signs a person in with OAuth and registers Archive itself
// (Dynamic Client Registration), so there are no app keys. Archive asks only for mcp:read.
// Its tools are listed only to a signed-in person, so each sync names the tool names it may have, first one listed wins,
// and passes arguments under the names they may take, which toolReq keeps to those the tool's schema has.
func init() {
	Register(&Connector{
		ID: "monarch", Name: "Monarch", MCP: "https://api.monarch.com/mcp", Scopes: []string{"mcp:read"}, Pace: 500 * time.Millisecond, Busy: ToolBusy,
		Info: Info{By: "Monarch", Description: "Read your Monarch Money: accounts and balances, transactions, categories, budgets and investment holdings", Category: CatFinance, Icon: IconLedger,
			Scopes: []string{"Read your accounts, balances, transactions, categories, budgets and holdings", "Never change anything in Monarch or move money"}},
		Who: &Who{Tool: "get_me,get_user,get_profile,get_household,whoami"},
		Sync: []*Sync{
			{Name: "accounts", Gap: time.Second, Req: Req{Tool: "get_accounts,list_accounts,accounts"}, Items: "**", Key: monarchKey,
				Doc: "Every account Monarch tracks, hidden and closed ones too, by its id, with its institution, type and current balance: listed whole each sync, so one removed is deleted.",
				To: Map{Table: "bank_account",
					Cols: map[string]SQL{"title": monarchText("displayName", "display_name", "name"), "official_name": monarchText("institution,name", "institution_name", "institution"),
						"mask": monarchText("mask", "account_mask"), "type": monarchType, "subtype": monarchText("subtype,display", "subtype,name", "subtype"),
						"balance":      monarchMoney("currentBalance", "current_balance", "displayBalance", "display_balance", "balance"),
						"available":    monarchMoney("availableBalance", "available_balance"),
						"credit_limit": monarchMoney("limit", "creditLimit", "credit_limit")}}},
			{Name: "transactions", Gap: time.Second, Parts: monarchYears, Keep: true, Items: "**", Key: monarchKey, Page: monarchPage,
				Req: Req{Tool: "get_transactions,list_transactions,search_transactions", Body: Obj{"start_date": "{in/start}", "end_date": "{in/end}", "startDate": "{in/start}", "endDate": "{in/end}"}},
				Doc: "Every transaction of every account, by its id, with its merchant, category, notes and whether it is pending: a year at a time from ten years back the first time, " +
					"and after that the last 30 days each sync, 100 a page. A listing deletes nothing, so one deleted in Monarch stays here.",
				To: Map{Table: "bank_transaction",
					Cols: map[string]SQL{"posted_on": "archive.sync_day(" + monarchText("date") + ")", "amount": monarchMoney("amount"),
						"description": monarchText("plaidName", "plaid_name", "originalStatement", "original_statement", "originalName", "original_name"),
						"merchant":    monarchText("merchant,name", "merchant_name", "merchant"), "counterparty": monarchText("merchant,name", "merchant_name", "merchant"),
						"category": monarchText("category,name", "category_name", "category"), "detail": monarchText("category,group,name", "category_group", "group"),
						"pending":        "archive.sync_bool(coalesce(data->'pending', data->'isPending', data->'is_pending'))",
						"account_source": monarchText("account,id", "account_id", "accountId")},
					Refs: map[string]SQL{"account": monarchText("account,id", "account_id", "accountId")}}},
			{Name: "categories", Every: monarchDaily, Gap: time.Second, Req: Req{Tool: "get_transaction_categories,get_categories,list_categories"}, Items: "**", Key: monarchKey,
				Doc: "Every category transactions are sorted into, by its id, with its group and whether it is hidden: listed whole each day, so one removed is deleted.",
				To: Map{Table: "budget_category",
					Cols: map[string]SQL{"budget": "'Monarch'", "group_name": monarchText("group,name", "group_name", "group"), "title": monarchText("name", "title"),
						"hidden": "archive.sync_bool(coalesce(data->'isHidden', data->'is_hidden', data->'hidden'))"}}},
			{Name: "budget", Every: monarchDaily, Gap: time.Second, Parts: monarchMonth, Keep: true, Items: "**", Key: []Reads{{Of: "{category/id;category_id;categoryId;id}/{in/start}"}}, Set: Obj{"month": "{in/start}"},
				Req: Req{Tool: "get_budgets,get_budget,list_budgets", Body: Obj{"start_date": "{in/start}", "end_date": "{in/end}", "startDate": "{in/start}", "endDate": "{in/end}", "month": "{in/start}"}},
				Doc: "The budget of last month, this month and next, by category and month: what is planned, what was spent or came in, and what is left: listed each day, a row a category a month, and kept once the month is past.",
				To: Map{Table: "budget_category",
					Cols: map[string]SQL{"budget": "'Monarch ' || left(data->>'month', 7)", "group_name": monarchText("category,group,name", "group_name", "group"),
						"title":     monarchText("category,name", "category_name", "name"),
						"budgeted":  monarchMoney("plannedCashFlowAmount", "planned_cash_flow_amount", "planned", "budgeted", "monthlyAmounts,0,plannedCashFlowAmount"),
						"activity":  monarchMoney("actualAmount", "actual_amount", "actual", "spent", "monthlyAmounts,0,actualAmount"),
						"available": monarchMoney("remainingAmount", "remaining_amount", "remaining", "available", "monthlyAmounts,0,remainingAmount")}}},
			{Name: "holdings", Gap: time.Second, Req: Req{Tool: "get_account_holdings,get_holdings,get_investments,get_portfolio"}, Items: "**",
				Key: []Reads{{Of: "{id;security/id}"}},
				Doc: "What each investment account holds, by its id, with the security's name, ticker and type, quantity, price and value: listed whole each sync, so one sold is deleted.",
				To: Map{Table: "investment_holding",
					Cols: map[string]SQL{"security": monarchText("security,name", "name", "security_name"), "ticker": monarchText("security,ticker", "ticker", "symbol"),
						"kind": monarchText("security,typeDisplay", "security,type", "type", "kind"), "quantity": "archive.sync_num(" + monarchText("quantity", "shares") + ")",
						"price": monarchMoney("security,currentPrice", "price", "current_price"), "value": monarchMoney("totalValue", "total_value", "value"),
						"cost_basis":     monarchMoney("basis", "costBasis", "cost_basis"),
						"account_source": monarchText("account,id", "account_id", "accountId")},
					Refs: map[string]SQL{"account": monarchText("account,id", "account_id", "accountId")}}},
		},
	})
}

const monarchDaily = 24 * time.Hour

var (
	monarchKey   = []Reads{{Of: "{id}"}}
	monarchPage  = ToolPage{Limit: "limit,page_size,count", Max: 100, After: "cursor,after", Offset: "offset"}
	monarchYears = &Parts{Span: &Span{First: Shift{-10, 0, 0}, Step: Shift{1, 0, 0}, Then: Shift{0, 0, -30}}, ID: "{start}", Vals: Obj{"start": "{start}", "end": "{end}"}}
	monarchMonth = &Parts{Span: &Span{Month: true, Last: Shift{0, 1, 0}, First: Shift{0, -2, 0}, Step: Shift{0, 1, 0}}, ID: "{start}", Vals: Obj{"start": "{start}", "end": "{end}"}}

	monarchType = SQL("CASE lower(" + monarchText("type,name", "type") + ") WHEN 'depository' THEN 'depository' WHEN 'credit' THEN 'credit' WHEN 'loan' THEN 'loan' " +
		"WHEN 'brokerage' THEN 'investment' WHEN 'investment' THEN 'investment' WHEN 'cryptocurrency' THEN 'crypto' WHEN 'crypto' THEN 'crypto' ELSE 'other' END")
)

// monarchText is the first of a record's fields that has a value, each a comma path like account,id, for the names Monarch's tools may give.
func monarchText(fields ...string) SQL {
	alts := make([]string, len(fields))
	for i, f := range fields {
		if strings.Contains(f, ",") {
			alts[i] = "nullif(data#>>'{" + f + "}', '')"
		} else {
			alts[i] = "CASE jsonb_typeof(data->'" + f + "') WHEN 'object' THEN nullif(data#>>'{" + f + ",name}', '') ELSE nullif(data->>'" + f + "', '') END"
		}
	}
	if len(alts) == 1 {
		return SQL(alts[0])
	}
	return SQL("coalesce(" + strings.Join(alts, ", ") + ")")
}

// monarchMoney is the first of a record's money fields that has a value; Monarch keeps money in the household's currency, US dollars.
func monarchMoney(fields ...string) SQL {
	return SQL("archive.sync_money(" + string(monarchText(fields...)) + ", coalesce(nullif(data->>'currency', ''), 'USD'))")
}
