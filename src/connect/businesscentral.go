package connect

import (
	"time"

	"golang.org/x/oauth2"
)

func init() {
	Register(&Connector{
		ID: "businesscentral", Name: "Business Central", Integration: true, Env: "ARCHIVE_OUTLOOK",
		Info: Info{By: "Microsoft", Description: "Read your companies’ customers, vendors, items, sales, purchases and ledger, and add or update sales quotes and customers when you ask",
			Category: CatFinance, Icon: IconLedger,
			Scopes: []string{"Read every company of your production environments", "Add and change sales quotes and customers when you ask; never post or delete"}},
		API: bcAPI, Endpoint: bcEndpoint, Scopes: bcScopes, Params: bcParams,
		Who:     &Who{Req: Req{URL: "/environments/v1.1"}, Each: "value", Where: Test{At: "type", Is: "Production"}, Names: []Path{"name"}},
		Actions: bcActions("Production"),
		Sync:    bcSyncs(bcEnvironments("Production")),
	})

	Register(&Connector{
		ID: "businesscentralsandbox", Name: "Business Central sandbox", Integration: true, Env: "ARCHIVE_OUTLOOK",
		Info: Info{By: "Microsoft", Description: "Read your sandbox companies’ customers, vendors, items, sales, purchases and ledger, and add or update sales quotes and customers when you ask",
			Category: CatFinance, Icon: IconLedger,
			Scopes: []string{"Read every company of your sandbox environments", "Add and change sales quotes and customers when you ask; never post or delete"}},
		API: bcAPI, Endpoint: bcEndpoint, Scopes: bcScopes, Params: bcParams,
		Who:     &Who{Req: Req{URL: "/environments/v1.1"}, Each: "value", Where: Test{At: "type", Is: "Sandbox"}, Names: []Path{"name"}},
		Actions: bcActions("Sandbox"),
		Sync:    bcSyncs(bcEnvironments("Sandbox")),
	})

	Register(&Connector{
		ID: "businesscentralonprem", Name: "Business Central on premises", Integration: true,
		Info: Info{By: "Microsoft", Description: "Read your companies’ customers, vendors, items, sales, purchases and ledger from your own server, and add or update sales quotes and customers when you ask",
			Category: CatFinance, Icon: IconLedger,
			Scopes: []string{"Read every company of the server instance, as the user may", "Add and change sales quotes and customers when you ask; never post or delete"}},
		Keys: []Key{
			{Name: "url", Label: "API address",
				Help: "The address of your Business Central server instance's API web services, reachable from the internet over https, like https://bc.example.com:7048/BC/api/v2.0"},
			{Name: "user", Label: "User name", Help: "The Business Central user Archive reads as, like ARCHIVE; give it the permission sets D365 READ, and D365 SALES DOC, EDIT and D365 CUSTOMER, EDIT to write"},
			{Name: "key", Label: "Web service access key", Secret: true,
				Help: "On that user's card in Business Central, Web Service Access Key: make one with no expiry date and copy it. The server's credential type must be NavUserPassword"},
		},
		Basic: [2]string{"user", "key"},
		API:   "{+base}/api/v2.0",
		Let: []Step{
			{As: "base", In: "{key/url}", Find: `^(https://[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*\.[A-Za-z]{2,}(?::[0-9]{1,5})?(?:/[A-Za-z0-9_.-]+)*?)/*(?:/api/v2\.0)?/*$`},
			{Unless: "base", Fail: "Business Central's address is https://<server>:<port>/<instance>/api/v2.0, at a host name the internet reaches, not {key/url}"},
		},
		Who:     &Who{Req: Req{URL: "/companies"}, Each: "value", Names: []Path{"displayName", "name"}},
		Actions: bcActions(""),
		Sync: bcSyncs(&Parts{Req: Req{URL: "/companies", Header: bcPrefer}, Items: "value", Page: bcPage,
			Vals: Obj{"at": "/companies({id})", "company": "{displayName;name}", "environment": "on premises"}}),
	})
}

const (
	bcAPI          = "https://api.businesscentral.dynamics.com"
	bcCurrency SQL = "coalesce(nullif(data->>'currencyCode', ''), 'USD')"
	bcEdited   SQL = "archive.sync_at(data->>'lastModifiedDateTime')"
	bcDoc          = ", by company and id: listed whole the first time and a week after, 1000 a page, which deletes those gone, and in between those changed since the sync before."
	bcZero         = "00000000-0000-0000-0000-000000000000"
)

var (
	bcEndpoint = oauth2.Endpoint{AuthURL: "https://login.microsoftonline.com/organizations/oauth2/v2.0/authorize",
		TokenURL: "https://login.microsoftonline.com/organizations/oauth2/v2.0/token", AuthStyle: oauth2.AuthStyleInParams}
	bcScopes  = []string{"offline_access", bcAPI + "/Financials.ReadWrite.All"}
	bcParams  = map[string]string{"prompt": "select_account"}
	bcPage    = NextURL{At: "@odata.nextLink"}
	bcPrefer  = map[string][]string{"Prefer": {"odata.maxpagesize=1000"}}
	bcIfMatch = map[string][]string{"If-Match": {"*"}}
)

func bcEnvironments(kind string) *Parts {
	return &Parts{Req: Req{URL: "/environments/v1.1"}, Items: "value",
		Skip: []Test{{At: "applicationFamily", Not: "BusinessCentral"}, {At: "type", Not: kind}},
		Under: &Parts{Req: Req{URL: "/v2.0/{up/name}/api/v2.0/companies", Header: bcPrefer}, Items: "value", Page: bcPage, ID: "{up/name}/{id}",
			Vals: Obj{"at": "/v2.0/{up/name}/api/v2.0/companies({id})", "company": "{displayName;name}", "environment": "{up/name}"}}}
}

func bcRef(of, field string) SQL {
	return SQL("data->>'company' || '/' || nullif(nullif(" + of + "->>'" + field + "', ''), '" + bcZero + "')")
}

func bcDay(field string) SQL {
	return SQL("archive.sync_day(nullif(data->>'" + field + "', '0001-01-01'))")
}

func bcEnum(at string) SQL {
	return SQL("nullif(btrim(replace(replace(replace(" + at + ", '_x0020_', ' '), '_x002D_', '-'), '_x0026_', '&')), '')")
}

type bcDocument struct {
	name, kind, entity, lines, on, due, with string
}

var bcDocuments = []bcDocument{
	{"sales_quotes", "SalesQuote", "salesQuotes", "salesQuoteLines", "documentDate", "validUntilDate", "customer"},
	{"sales_orders", "SalesOrder", "salesOrders", "salesOrderLines", "orderDate", "requestedDeliveryDate", "customer"},
	{"sales_invoices", "SalesInvoice", "salesInvoices", "salesInvoiceLines", "invoiceDate", "dueDate", "customer"},
	{"purchase_orders", "PurchaseOrder", "purchaseOrders", "purchaseOrderLines", "orderDate", "requestedReceiptDate", "vendor"},
	{"purchase_invoices", "PurchaseInvoice", "purchaseInvoices", "purchaseInvoiceLines", "invoiceDate", "dueDate", "vendor"},
}

func bcStream(name, entity, expand string, parts *Parts) *Sync {
	q := Q{}
	if expand != "" {
		q["$expand"] = expand
	}
	since := Q{"$filter": "lastModifiedDateTime gt {since}"}
	for k, v := range q {
		since[k] = v
	}
	return &Sync{Name: name, Gap: time.Second, Parts: parts, Req: Req{URL: "{+in/at}/" + entity, Query: q, Header: bcPrefer}, Items: "value",
		Key: []Reads{{Of: "{part}/{id}"}}, Updated: "lastModifiedDateTime", Page: bcPage,
		Change: SinceTime{Req: Req{URL: "{+in/at}/" + entity, Query: since, Header: bcPrefer}},
		Set:    Obj{"company": "{part}", "company_name": "{in/company}", "environment": "{in/environment}"},
		Pass:   []int{403}}
}

func bcSyncs(parts *Parts) []*Sync {
	address := SQL("nullif(concat_ws(', ', nullif(data->>'addressLine1', ''), nullif(data->>'addressLine2', ''), nullif(data->>'city', ''), nullif(data->>'state', ''), " +
		"nullif(data->>'postalCode', ''), nullif(data->>'country', '')), '')")
	customers := bcStream("customers", "customers", "", parts)
	customers.Doc = "Every customer of each company, with its number, name, address, phone, email, balance due and whether it is blocked" + bcDoc
	customers.To = Map{Table: "ledger_contact",
		Cols: map[string]SQL{"kind": "'customer'", "title": "data->>'displayName'", "company": "data->>'displayName'", "email": "archive.sync_email(data->>'email')",
			"phone": "data->>'phoneNumber'", "address": address, "balance": "archive.sync_money(data->>'balanceDue', " + bcCurrency + ")",
			"active": bcEnum("data->>'blocked'") + " IS NULL", "edited_at": bcEdited},
		People: map[string]SQL{"person": "data->>'email'"}}

	vendors := bcStream("vendors", "vendors", "", parts)
	vendors.Doc = "Every vendor of each company, with its number, name, address, phone, email, balance and whether it is blocked" + bcDoc
	vendors.To = Map{Table: "ledger_contact",
		Cols: map[string]SQL{"kind": "'vendor'", "title": "data->>'displayName'", "company": "data->>'displayName'", "email": "archive.sync_email(data->>'email')",
			"phone": "data->>'phoneNumber'", "address": address, "balance": "archive.sync_money(data->>'balance', " + bcCurrency + ")",
			"active": bcEnum("data->>'blocked'") + " IS NULL", "edited_at": bcEdited},
		People: map[string]SQL{"person": "data->>'email'"}}

	items := bcStream("items", "items", "", parts)
	items.Doc = "Every item of each company, with its number, name, type, category, unit price and cost, and inventory on hand" + bcDoc
	items.To = Map{Table: "ledger_item",
		Cols: map[string]SQL{"title": "data->>'displayName'", "sku": "data->>'number'", "type": bcEnum("data->>'type'"),
			"description": "concat_ws(E'\\n', nullif(data->>'displayName2', ''), nullif(data->>'itemCategoryCode', ''))",
			"price":       "archive.sync_money(data->>'unitPrice', 'USD')", "cost": "archive.sync_money(data->>'unitCost', 'USD')",
			"quantity": "archive.sync_num(data->>'inventory')", "active": "NOT coalesce(archive.sync_bool(data->'blocked'), false)", "edited_at": bcEdited}}

	accounts := bcStream("accounts", "accounts", "", parts)
	accounts.Doc = "The chart of accounts of each company, with each account's number, name, category, subcategory and net change, the blocked too" + bcDoc
	accounts.To = Map{Table: "ledger_account",
		Cols: map[string]SQL{"number": "data->>'number'", "title": "data->>'displayName'", "type": bcEnum("data->>'category'"), "subtype": "nullif(data->>'subCategory', '')",
			"class": "CASE data->>'category' WHEN 'Assets' THEN 'Asset' WHEN 'Liabilities' THEN 'Liability' WHEN 'Equity' THEN 'Equity' WHEN 'Income' THEN 'Revenue' " +
				"WHEN 'CostOfGoodsSold' THEN 'Expense' WHEN 'Expense' THEN 'Expense' END",
			"balance": "archive.sync_money(data->>'netChange', 'USD')", "active": "NOT coalesce(archive.sync_bool(data->'blocked'), false)", "edited_at": bcEdited}}

	ledger := bcStream("ledger_entries", "generalLedgerEntries", "", parts)
	ch := ledger.Change.(SinceTime)
	ch.Lasts = true
	ledger.Change = ch
	ledger.Doc = "Every general ledger entry of each company, with its posting date, document type and number, account, description and debit or credit, " +
		"by company and id: listed whole the first time, 1000 a page, and then those posted since the sync before; a posted entry never changes."
	ledger.To = Map{Table: "ledger_entry",
		Cols: map[string]SQL{"kind": "'GLEntry'", "number": "data->>'documentNumber'", "txn_on": bcDay("postingDate"),
			"total": "archive.sync_money((coalesce(archive.sync_num(data->>'debitAmount'), 0) - coalesce(archive.sync_num(data->>'creditAmount'), 0))::text, 'USD')",
			"memo":  "data->>'description'", "status": bcEnum("data->>'documentType'"), "edited_at": bcEdited},
		Refs: map[string]SQL{"account": bcRef("data", "accountId")}}

	out := []*Sync{accounts, customers, vendors, items}
	for _, d := range bcDocuments {
		s := bcStream(d.name, d.entity, d.lines, parts)
		s.Doc = "Every " + d.kind + " of each company, with its number, " + d.with + ", dates, totals, status and lines" + bcDoc
		balance := SQL("NULL")
		if d.entity == "salesInvoices" {
			balance = "archive.sync_money(data->>'remainingAmount', " + bcCurrency + ")"
		}
		s.To = Map{Table: "ledger_entry",
			Cols: map[string]SQL{"kind": SQL("'" + d.kind + "'"), "number": "data->>'number'", "txn_on": bcDay(d.on),
				"due_on": bcDay(d.due), "contact_name": SQL("data->>'" + d.with + "Name'"),
				"total": "archive.sync_money(data->>'totalAmountIncludingTax', " + bcCurrency + ")", "balance": balance,
				"memo": "nullif(data->>'externalDocumentNumber', '')", "status": bcEnum("data->>'status'"), "edited_at": bcEdited},
			Refs: map[string]SQL{"contact": bcRef("data", d.with+"Id")},
			Children: []Kids{{Table: "ledger_line", From: SQL("archive.sync_list(data->'" + d.lines + "') WITH ORDINALITY l(x, i) WHERE x->>'lineType' IS DISTINCT FROM 'Comment'"),
				Cols: map[string]SQL{"position": "i", "kind": bcEnum("x->>'lineType'"), "description": "x->>'description'",
					"amount":     "archive.sync_money(coalesce(x->>'amountExcludingTax', x->>'netAmount'), " + bcCurrency + ")",
					"quantity":   "archive.sync_num(x->>'quantity')",
					"unit_price": "archive.sync_num(coalesce(x->>'unitPrice', x->>'directUnitCost'))"},
				Refs: map[string]SQL{"item": bcRef("x", "itemId"), "account": bcRef("x", "accountId")}}}}
		out = append(out, s)
	}
	return append(out, ledger)
}

func bcActions(kind string) map[string]*Action {
	at, need := "/companies({company})", []Step(nil)
	companies := []Step{{Req: Req{URL: "/companies", Header: bcPrefer}, Page: bcPage, Items: "value"}}
	if kind != "" {
		at = "/v2.0/{environment}/api/v2.0/companies({company})"
		need = []Step{{Unless: "environment", Fail: "environment is needed: an environment's name from companies, like " + kind}}
		companies = []Step{
			{As: "envs", Req: Req{URL: "/environments/v1.1"}},
			{As: "found", Each: "envs/value", Only: Test{At: "type", Is: kind}, Onto: "companies", Req: Req{URL: "/v2.0/{name}/api/v2.0/companies", Header: bcPrefer}, Page: bcPage, Items: "value"},
		}
	}
	steps := func(s ...Step) []Step { return append(append([]Step{}, need...), s...) }
	where, list := "the environment's name and the company's id from companies",
		"The "+kind+" environments of the tenant, each with its companies: [{ name, type, companies: [{ id, name, displayName, systemVersion }] }]"
	if kind == "" {
		where, list = "the company's id from companies", "The companies of the server instance: [{ id, name, displayName, systemVersion }]"
	}
	return map[string]*Action{
		"companies": Call[struct{}, any](Plan{
			Doc:   list,
			Steps: companies,
		}),
		"get": Call[bcGet, any](Plan{
			Doc: "Read any of a company's API v2.0 endpoints, by " + where + ", like { path: \"salesOrders\", query: \"$filter=status eq 'Open'&$expand=salesOrderLines\" }, " +
				"{ path: \"customers(<id>)\" }, { path: \"salesInvoices(<id>)/salesInvoiceLines\" } or { path: \"agedAccountsReceivables\" }: its answer, with value and " +
				"@odata.nextLink when it is a list, which $top and $skip page",
			Steps: steps(
				Step{Only: Test{At: "path", Re: `^[A-Za-z][A-Za-z0-9_]*(?:\([A-Za-z0-9'-]*\))?(?:/[A-Za-z][A-Za-z0-9_]*(?:\([A-Za-z0-9'-]*\))?)*$`, Off: true},
					Fail: "path is an endpoint of the company, like salesOrders or customers(<id>)"},
				Step{Req: Req{URL: at + "/{+path}?{+query|}"}},
			),
		}),
		"create_customer": Call[bcNewCustomer, any](Plan{
			Doc: "Add a customer to a company, by " + where + ", with its fields, like { customer: { displayName, type: \"Company\", email, phoneNumber, addressLine1, " +
				"city, state, country, postalCode, currencyCode } }; Business Central gives it its number unless you do. Returns the customer, with its id and number",
			Steps: steps(Step{Req: Req{Method: POST, URL: at + "/customers", Body: "{customer}"}}),
		}),
		"update_customer": Call[bcEditCustomer, any](Plan{
			Doc:   "Change a customer of a company, by " + where + " and the customer's id: only the fields given change. Returns the customer",
			Steps: steps(Step{Req: Req{Method: PATCH, URL: at + "/customers({id})", Header: bcIfMatch, Body: "{changes}"}}),
		}),
		"create_quote": Call[bcNewQuote, any](Plan{
			Doc: "Add a sales quote to a company, by " + where + ", like { quote: { customerNumber: \"10000\", documentDate, validUntilDate, externalDocumentNumber, " +
				"salesQuoteLines: [{ lineType: \"Item\", lineObjectNumber: \"1896-S\", quantity: 4, unitPrice }] } }. Returns the quote, with its id and number; it stays a draft " +
				"until someone sends or converts it in Business Central",
			Steps: steps(Step{Req: Req{Method: POST, URL: at + "/salesQuotes", Body: "{quote}"}}),
		}),
		"update_quote": Call[bcEditQuote, any](Plan{
			Doc:   "Change a sales quote of a company, by " + where + " and the quote's id: only the fields given change, like { changes: { validUntilDate, externalDocumentNumber } }. Returns the quote",
			Steps: steps(Step{Req: Req{Method: PATCH, URL: at + "/salesQuotes({id})", Header: bcIfMatch, Body: "{changes}"}}),
		}),
		"add_quote_line": Call[bcQuoteLine, any](Plan{
			Doc:   "Add a line to a sales quote of a company, by " + where + " and the quote's id, like { line: { lineType: \"Item\", lineObjectNumber: \"1896-S\", quantity: 2 } }. Returns the line",
			Steps: steps(Step{Req: Req{Method: POST, URL: at + "/salesQuotes({quote})/salesQuoteLines", Body: "{line}"}}),
		}),
	}
}

type (
	bcGet struct {
		Environment string `json:"environment,omitempty" jsonschema:"The environment's name, like Production, from companies; on premises leaves it out"`
		Company     string `json:"company" jsonschema:"The company's id, from companies"`
		Path        string `json:"path" jsonschema:"The endpoint after the company, like salesOrders, customers(<id>) or salesInvoices(<id>)/salesInvoiceLines"`
		Query       string `json:"query,omitempty" jsonschema:"Its OData query string, like $filter=number eq 'SO1001'&$expand=salesOrderLines&$top=50"`
	}
	bcNewCustomer struct {
		Environment string         `json:"environment,omitempty" jsonschema:"The environment's name, like Production, from companies; on premises leaves it out"`
		Company     string         `json:"company" jsonschema:"The company's id, from companies"`
		Customer    map[string]any `json:"customer" jsonschema:"Its fields by API name, like displayName, type, email, phoneNumber, addressLine1, city, state, country, postalCode"`
	}
	bcEditCustomer struct {
		Environment string         `json:"environment,omitempty" jsonschema:"The environment's name, like Production, from companies; on premises leaves it out"`
		Company     string         `json:"company" jsonschema:"The company's id, from companies"`
		ID          string         `json:"id" jsonschema:"The customer's id"`
		Changes     map[string]any `json:"changes" jsonschema:"The fields to change by API name"`
	}
	bcNewQuote struct {
		Environment string         `json:"environment,omitempty" jsonschema:"The environment's name, like Production, from companies; on premises leaves it out"`
		Company     string         `json:"company" jsonschema:"The company's id, from companies"`
		Quote       map[string]any `json:"quote" jsonschema:"Its fields by API name, like customerNumber or customerId, documentDate, validUntilDate, and salesQuoteLines"`
	}
	bcEditQuote struct {
		Environment string         `json:"environment,omitempty" jsonschema:"The environment's name, like Production, from companies; on premises leaves it out"`
		Company     string         `json:"company" jsonschema:"The company's id, from companies"`
		ID          string         `json:"id" jsonschema:"The quote's id"`
		Changes     map[string]any `json:"changes" jsonschema:"The fields to change by API name"`
	}
	bcQuoteLine struct {
		Environment string         `json:"environment,omitempty" jsonschema:"The environment's name, like Production, from companies; on premises leaves it out"`
		Company     string         `json:"company" jsonschema:"The company's id, from companies"`
		Quote       string         `json:"quote" jsonschema:"The quote's id"`
		Line        map[string]any `json:"line" jsonschema:"Its fields by API name, like lineType, lineObjectNumber or itemId, description, quantity, unitPrice"`
	}
)
