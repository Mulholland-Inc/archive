package connect

import (
	"time"

	"golang.org/x/oauth2"
)

func init() {
	Register(&Connector{
		ID: "ynab", Name: "YNAB",
		Info: ynabInfo,
		API:  "https://api.ynab.com/v1",
		Endpoint: oauth2.Endpoint{AuthURL: "https://app.ynab.com/oauth/authorize", TokenURL: "https://app.ynab.com/oauth/token",
			AuthStyle: oauth2.AuthStyleInParams},
		Scopes:  []string{"read-only"},
		Who:     &Who{Req: Req{URL: "/user"}, Names: []Path{"data/user/id"}},
		Actions: ynabActions(),
		Sync:    ynabSyncs(),
	})
}

var ynabInfo = Info{By: "YNAB", Description: "Read your YNAB plans: accounts, transactions and what each category has budgeted, spent and left", Category: CatFinance, Icon: IconLedger,
	Scopes: []string{"Read your plans, accounts, payees, categories and transactions", "Never change your plans or move money"}}

func ynabActions() map[string]*Action {
	return map[string]*Action{
		"get": Call[ynabGet, any](Plan{
			Doc: "Read one of YNAB's GET endpoints, like { path: \"/plans\" }, { path: \"/plans/<id>/months/current\" }, { path: \"/plans/<id>/transactions\", query: \"since_date=2026-10-01\" } " +
				"or { path: \"/plans/<id>/scheduled_transactions\" }: its answer, { data }, money in milliunits (1000 is one dollar). It only reads",
			Steps: []Step{
				{Only: Test{At: "path", Re: `^/(user|plans|budgets)(/|$)`, Off: true}, Fail: "YNAB reads only /user and /plans and what is under it"},
				{Req: Req{URL: "{+path}?{+query|}"}},
			},
		}),
	}
}

func ynabSyncs() []*Sync {
	return []*Sync{
		{Name: "accounts", Gap: time.Second, Parts: ynabPlans, Req: Req{URL: "/plans/{part}/accounts"}, Items: "data/accounts",
			Gone: ynabDeleted, Set: ynabSet,
			Change: Resume{At: "data/server_knowledge", Req: Req{URL: "/plans/{part}/accounts", Query: Q{"last_knowledge_of_server": "{token}"}}},
			Doc:    "Every account of each plan, open and closed, by its id, with its balance: all of them the first time, and then those changed since, by YNAB's server_knowledge.",
			To: Map{Table: "bank_account",
				Cols: map[string]SQL{"title": "data->>'name'", "official_name": "data->>'plan'", "subtype": "data->>'type'", "type": ynabType,
					"balance": ynabMoney("balance"), "available": ynabMoney("cleared_balance")}}},
		{Name: "transactions", Gap: time.Second, Parts: ynabPlans, Req: Req{URL: "/plans/{part}/transactions"}, Items: "data/transactions",
			Gone: ynabDeleted, Set: ynabSet,
			Change: Resume{At: "data/server_knowledge", Req: Req{URL: "/plans/{part}/transactions", Query: Q{"last_knowledge_of_server": "{token}"}}},
			Doc: "Every transaction of each plan, by its id, with its payee, category, memo and whether it has cleared: all of them the first time, and then those added, " +
				"changed and deleted since, by YNAB's server_knowledge.",
			To: Map{Table: "bank_transaction",
				Cols: map[string]SQL{"posted_on": "archive.sync_day(data->>'date')", "amount": ynabMoney("amount"), "description": "data->>'memo'",
					"merchant": "data->>'payee_name'", "counterparty": "data->>'payee_name'", "category": "data->>'category_name'",
					"detail":  "CASE WHEN data->>'transfer_account_id' IS NOT NULL THEN 'transfer' WHEN jsonb_array_length(coalesce(data->'subtransactions', '[]')) > 0 THEN 'split' END",
					"pending": "data->>'cleared' = 'uncleared'"},
				Refs: map[string]SQL{"account": "data->>'account_id'"}}},
		{Name: "categories", Gap: time.Second, Parts: ynabPlans, Req: Req{URL: "/plans/{part}/months/current"}, Items: "data/month/categories",
			Gone: ynabDeleted, Set: ynabSet,
			Doc: "Every category of each plan, by its id, with what it has budgeted, spent and left this month and its goal: listed whole each sync.",
			To: Map{Table: "budget_category",
				Cols: map[string]SQL{"budget": "data->>'plan'", "group_name": "data->>'category_group_name'", "title": "data->>'name'",
					"budgeted": ynabMoney("budgeted"), "activity": ynabMoney("activity"), "available": ynabMoney("balance"), "goal": ynabMoney("goal_target"),
					"goal_due_on": "archive.sync_day(coalesce(data->>'goal_target_date', data->>'goal_target_month'))", "hidden": "archive.sync_bool(data->'hidden')", "note": "data->>'note'"}}},
	}
}

const ynabType SQL = "CASE WHEN data->>'type' IN ('checking', 'savings', 'cash') THEN 'depository' WHEN data->>'type' IN ('creditCard', 'lineOfCredit') THEN 'credit' " +
	"WHEN data->>'type' IN ('mortgage', 'autoLoan', 'studentLoan', 'personalLoan', 'medicalDebt', 'otherDebt') THEN 'loan' ELSE 'other' END"

// ynabMoney is a field YNAB gives in milliunits of the plan's currency, as money.
func ynabMoney(field string) SQL {
	return SQL("archive.sync_money((archive.sync_num(data->>'" + field + "') / 1000)::text, coalesce(nullif(data->>'currency', ''), 'USD'))")
}

var (
	ynabPlans   = &Parts{Req: Req{URL: "/plans"}, Items: "data/plans", Vals: Obj{"name": "{name}", "currency": "{currency_format/iso_code}"}}
	ynabSet     = Obj{"plan": "{in/name}", "currency": "{in/currency}"}
	ynabDeleted = []Test{{At: "deleted", Is: "true"}}
)

type ynabGet struct {
	Path  string `json:"path" jsonschema:"The endpoint, like /plans, /plans/<id>/accounts, /plans/<id>/months/current or /plans/<id>/transactions"`
	Query string `json:"query,omitempty" jsonschema:"Its query, like since_date=2026-10-01&type=uncategorized"`
}
