package connect

import (
	"time"

	"golang.org/x/oauth2"
)

func init() {
	Register(&Connector{
		ID: "netsuite", Name: "NetSuite", Integration: true,
		Info: Info{By: "Oracle", Description: "Read your company’s invoices, bills, customers and books", Category: CatFinance, Icon: IconLedger,
			Scopes: []string{"Read one account’s records, as the role you sign in with may see them"}},
		Hosts: []string{"https://system.netsuite.com"},
		API:   nsHost + "/services/rest",
		Endpoint: oauth2.Endpoint{AuthURL: "https://system.netsuite.com/app/login/oauth2/authorize.nl", TokenURL: nsHost + "/services/rest/auth/oauth2/v1/token",
			AuthStyle: oauth2.AuthStyleInHeader},
		RevokeBy: &Req{Method: POST, URL: nsHost + "/services/rest/auth/oauth2/v1/revoke", Header: map[string][]string{"Authorization": {"{basic}"}}, Form: Q{"token": "{refresh}"}},
		Scopes:   []string{"rest_webservices"},
		Keep:     []string{"company"},
		Who: &Who{Req: Req{Method: POST, Read: true, URL: nsQL, Header: nsTransient,
			Body: Obj{"q": "SELECT s.name FROM subsidiary s WHERE s.parent IS NULL UNION ALL SELECT '{'meta/company}' AS name FROM dual"},
			Or:   &Req{Method: POST, Read: true, URL: nsQL, Header: nsTransient, Body: Obj{"q": "SELECT '{'meta/company}' AS name FROM dual"}}},
			Names: []Path{"items/0/name"}},
		Actions: map[string]*Action{
			"query": Call[nsQuery, any](Plan{
				Doc: "Run a SuiteQL query, NetSuite's SQL, like { query: \"SELECT id, tranid, BUILTIN.DF(entity) AS customer, foreigntotal FROM transaction WHERE type = 'CustInvc' " +
					"AND foreignamountremaining > 0 ORDER BY trandate DESC\" }: { items: [...], count, hasMore, offset, totalResults }. Tables include account, customer, vendor, " +
					"employee, item, transaction (type CustInvc, VendBill, CustPymt, VendPymt, Journal and more) and transactionline; BUILTIN.DF(field) is a field's display " +
					"text. At most 1000 rows a page, paged with offset, a multiple of limit, and 100,000 in all; SELECT only",
				Clamp: map[string][2]int{"limit": {100, 1000}},
				Steps: []Step{{Req: Req{Method: POST, Read: true, URL: nsQL, Query: Q{"limit": "{limit}", "offset": "{offset|0}"}, Header: nsTransient,
					Body: Obj{"q": "{query}"}}}},
			}),
			"record": Call[nsRecord, any](Plan{
				Doc:   "One record by its type and internal id, through the REST record API, like { type: \"invoice\", id: \"1234\" }: the record, its sublists as links",
				Steps: []Step{{Req: Req{URL: "/record/v1/{type}/{id}"}}},
			}),
		},
		Sync: []*Sync{
			{Name: "accounts", Gap: time.Second, Req: nsList, Items: "items", ID: "id", Page: nsPage, Deny: nsDeny,
				Vars: Obj{"select": "t.id, t.acctnumber, t.acctname, t.fullname, t.accttype, BUILTIN.DF(t.accttype) AS typename, t.parent, t.isinactive", "from": "account t"},
				Doc:  "The chart of accounts, the inactive too, by its internal id: listed whole each sync, 1000 a page.",
				To: Map{Table: "ledger_account",
					Cols: map[string]SQL{"number": "data->>'acctnumber'", "title": "data->>'acctname'", "full_name": "data->>'fullname'", "type": "data->>'typename'",
						"class": `CASE WHEN data->>'accttype' IN ('Bank', 'AcctRec', 'OthCurrAsset', 'FixedAsset', 'OthAsset', 'DeferExpense', 'UnbilledRec') THEN 'Asset'
							WHEN data->>'accttype' IN ('AcctPay', 'CredCard', 'OthCurrLiab', 'LongTermLiab', 'DeferRevenue') THEN 'Liability' WHEN data->>'accttype' = 'Equity' THEN 'Equity'
							WHEN data->>'accttype' IN ('Income', 'OthIncome') THEN 'Revenue' WHEN data->>'accttype' IN ('COGS', 'Expense', 'OthExpense') THEN 'Expense' END`,
						"active": nsActive},
					Refs: map[string]SQL{"parent": "data->>'parent'"}}},
			{Name: "customers", Gap: time.Second, Req: nsList, Items: "items", ID: "id", Page: nsPage, Deny: nsDeny, Change: nsChange, Set: nsKind, To: nsContact,
				Vars: Obj{"kind": "customer", "select": "t.id, t.entityid, t.companyname, t.email, t.phone, t.isinactive, " + nsStamp, "from": "customer t"},
				Doc:  "Every customer" + nsDoc},
			{Name: "vendors", Gap: time.Second, Req: nsList, Items: "items", ID: "id", Page: nsPage, Deny: nsDeny, Change: nsChange, Set: nsKind, To: nsContact,
				Vars: Obj{"kind": "vendor", "select": "t.id, t.entityid, t.companyname, t.email, t.phone, t.isinactive, " + nsStamp, "from": "vendor t"},
				Doc:  "Every vendor" + nsDoc},
			{Name: "employees", Gap: time.Second, Req: nsList, Items: "items", ID: "id", Page: nsPage, Deny: nsDeny, Change: nsChange, Set: nsKind, To: nsContact,
				Vars: Obj{"kind": "employee", "select": "t.id, t.entityid, t.email, t.phone, t.isinactive, " + nsStamp, "from": "employee t"},
				Doc:  "Every employee the role may see" + nsDoc},
			{Name: "items", Gap: time.Second, Req: nsList, Items: "items", ID: "id", Page: nsPage, Deny: nsDeny, Change: nsChange,
				Vars: Obj{"select": "t.id, t.itemid, t.displayname, BUILTIN.DF(t.itemtype) AS typename, t.description, t.incomeaccount, t.expenseaccount, t.isinactive, " + nsStamp, "from": "item t"},
				Doc:  "Every product and service" + nsDoc,
				To: Map{Table: "ledger_item",
					Cols: map[string]SQL{"title": "coalesce(nullif(data->>'displayname', ''), data->>'itemid')", "sku": "data->>'itemid'", "type": "data->>'typename'",
						"description": "data->>'description'", "active": nsActive, "edited_at": nsEditedAt},
					Refs: map[string]SQL{"income_account": "data->>'incomeaccount'", "expense_account": "data->>'expenseaccount'"}}},
			{Name: "transactions", Gap: time.Second, Req: nsList, Items: "items", ID: "id", Page: nsPage, Deny: nsDeny, Change: nsChange, Memo: "{stamp}",
				Vars: Obj{"select": "t.id, t.tranid, t.type, BUILTIN.DF(t.type) AS typename, TO_CHAR(t.trandate, 'YYYY-MM-DD') AS trandate, " +
					"TO_CHAR(t.duedate, 'YYYY-MM-DD') AS duedate, t.entity, BUILTIN.DF(t.entity) AS entityname, t.foreigntotal, t.foreignamountremaining, " +
					"c.symbol AS currency, t.memo, BUILTIN.DF(t.status) AS statusname, " + nsStamp,
					"from":  "transaction t LEFT JOIN currency c ON c.id = t.currency",
					"where": " AND t.type IN ('CustInvc', 'CustPymt', 'CustCred', 'CustRfnd', 'CustDep', 'CashSale', 'CashRfnd', 'Estimate', 'VendBill', 'VendPymt', 'VendCred', 'PurchOrd', 'Check', 'CardChrg', 'CardRfnd', 'ExpRept', 'Deposit', 'Transfer', 'Journal')"},
				Hydrate: []Hydrate{Fetch{Par: 2, As: "lines", Items: "items", Req: Req{Method: POST, Read: true, URL: nsQL, Query: Q{"limit": "1000"}, Header: nsTransient,
					Body: Obj{"q": "SELECT tl.id, tl.linesequencenumber, tl.memo, tl.foreignamount, tl.quantity, tl.rate, tl.item, tl.expenseaccount FROM transactionline tl " +
						"WHERE tl.transaction = {id} AND tl.mainline = 'F' AND tl.taxline = 'F' ORDER BY tl.linesequencenumber"}}}},
				Doc: "Every invoice, payment, credit memo, refund, deposit, cash sale, estimate, bill, bill payment, vendor credit, purchase order, check, card charge, expense report, " +
					"transfer and journal entry, with up to 1000 of its lines but its main and tax lines" + nsDoc,
				To: Map{Table: "ledger_entry",
					Cols: map[string]SQL{"kind": "coalesce(data->>'typename', data->>'type')", "number": "data->>'tranid'", "txn_on": "archive.sync_day(data->>'trandate')",
						"due_on": "archive.sync_day(data->>'duedate')", "contact_name": "data->>'entityname'",
						"total": "archive.sync_money(data->>'foreigntotal', " + nsCurrency + ")", "balance": "archive.sync_money(data->>'foreignamountremaining', " + nsCurrency + ")",
						"memo": "data->>'memo'", "status": "data->>'statusname'", "edited_at": nsEditedAt},
					Refs: map[string]SQL{"contact": "data->>'entity'"},
					Children: []Kids{{Table: "ledger_line", From: "archive.sync_list(data->'lines') WITH ORDINALITY l(x, i)",
						Cols: map[string]SQL{"position": "coalesce(archive.sync_num(x->>'linesequencenumber'), i)", "kind": "CASE WHEN nullif(x->>'item', '') IS NOT NULL THEN 'Item' ELSE 'Account' END",
							"description": "x->>'memo'", "amount": "archive.sync_money(ltrim(x->>'foreignamount', '-'), " + nsCurrency + ")",
							"posting":  "CASE WHEN data->>'type' = 'Journal' THEN CASE WHEN archive.sync_num(x->>'foreignamount') < 0 THEN 'Credit' ELSE 'Debit' END END",
							"quantity": "abs(archive.sync_num(x->>'quantity'))", "unit_price": "archive.sync_num(x->>'rate')"},
						Refs: map[string]SQL{"account": "x->>'expenseaccount'", "item": "x->>'item'"}}}}},
		},
	})
}

const (
	nsHost = "https://{-meta/company}.suitetalk.api.netsuite.com"
	nsDoc  = ", the inactive too, by its internal id: listed whole the first time and a week after, 1000 a page, which deletes those gone, and in between " +
		"those whose lastmodifieddate is on or after the newest read before."
	nsStamp        = "TO_CHAR(t.lastmodifieddate, 'YYYY-MM-DD HH24:MI:SS') AS stamp"
	nsActive   SQL = "data->>'isinactive' IS DISTINCT FROM 'T'"
	nsEditedAt SQL = "archive.sync_clock(data->>'stamp')"
	nsCurrency     = "coalesce(nullif(data->>'currency', ''), 'USD')"
	nsQL           = "/query/v1/suiteql"
	nsAfter        = "SELECT {select} FROM {from} WHERE t.id > {page|0}{where|}"
	nsOrder        = " ORDER BY t.id"
	nsSince        = " AND t.lastmodifieddate >= TO_TIMESTAMP('{'since}', 'YYYY-MM-DD HH24:MI:SS')"
)

var (
	nsTransient = map[string][]string{"Prefer": {"transient"}}
	nsList      = Req{Method: POST, Read: true, URL: nsQL, Query: Q{"limit": "1000"}, Header: nsTransient, Body: Obj{"q": nsAfter + nsOrder}}
	nsPage      = Cursor{At: "items/999/id", More: "hasMore"}
	nsChange    = SinceTime{Req: Req{Method: POST, Read: true, URL: nsQL, Query: Q{"limit": "1000"}, Header: nsTransient,
		Body: Obj{"q": nsAfter + nsSince + nsOrder}}, Stamp: "stamp"}
	nsKind    = Obj{"kind": "{kind}"}
	nsDeny    = []Deny{{Status: []int{400, 403}, Says: []string{"not found", "permission"}, Why: "NetSuite does not let the connected role read these: {message}"}}
	nsContact = Map{Table: "ledger_contact",
		Cols: map[string]SQL{"kind": "data->>'kind'", "title": "data->>'entityid'", "company": "data->>'companyname'", "email": "archive.sync_email(data->>'email')",
			"phone": "data->>'phone'", "active": nsActive, "edited_at": nsEditedAt},
		People: map[string]SQL{"person": "data->>'email'"}}
)

type (
	nsQuery struct {
		Query  string `json:"query" jsonschema:"The SuiteQL query, like SELECT id, entityid, email FROM customer WHERE isinactive = 'F'"`
		Limit  int    `json:"limit,omitempty" jsonschema:"At most this many rows, 1 to 1000; 100 when left out"`
		Offset int    `json:"offset,omitempty" jsonschema:"How many rows to skip, a multiple of limit, for the next page"`
	}
	nsRecord struct {
		Type string `json:"type" jsonschema:"The record type, like invoice, customer, vendorBill or journalEntry"`
		ID   string `json:"id" jsonschema:"Its internal id"`
	}
)
