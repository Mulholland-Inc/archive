package connect

import (
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// Zoho CRM and Zoho Books, both with ARCHIVE_ZOHO: a server-based client from Zoho's API Console
// (api-console.zoho.com) whose redirect URIs list <API>/connect/zohocrm/callback and <API>/connect/zohobooks/callback,
// with multi-DC on, so that an account of any data center connects. An owner connects one for everyone.
//
// Each data center is its own Zoho: consent starts at accounts.zoho.com, which sends a person on to theirs, and the
// callback names its accounts server, accounts-server, where their tokens come from and are revoked; the token's
// answer names their API, api_domain. Zoho asks for its scopes comma separated, and sends a refresh token only with
// access_type=offline, and again only with prompt=consent. Its APIs take a token as Zoho-oauthtoken, not Bearer.
//
// CRM syncs leads and contacts into crm_contact, as Salesforce's are, accounts into crm_company and deals into
// crm_deal; it adds and changes them when asked, and deletes nothing. Books syncs every organization of the account:
// its chart of accounts into ledger_account, its customers and vendors into ledger_contact, and its invoices, bills,
// payments both ways and journals into ledger_entry, without their lines, which only reading one has; it only reads.
// A stream is listed whole the first time and a week after, which deletes what is gone, and in between pages its
// newest changes down to the sync before; Books' chart of accounts and payments, which it cannot list by change, are
// listed whole every time. Neither pushes.
func init() {
	Register(&Connector{
		ID: "zohocrm", Name: "Zoho CRM", Integration: true, Env: "ARCHIVE_ZOHO",
		Info: Info{By: "Zoho", Description: "Read leads, contacts, accounts and deals, and add or update them when you ask", Category: CatCRM, Icon: IconUsers,
			Scopes: []string{"Read leads, contacts, accounts and deals", "Add and change them when you ask; never delete"}},
		Endpoint: zohoEndpoint, Hosts: zohoHosts, Region: "accounts-server", Regions: zohoRegions, Keep: []string{"api_domain"}, TokenType: "Zoho-oauthtoken",
		Revoke: zohoRevoke, Params: zohoParams, Let: zohoLet,
		API: "{+api}/crm/v8",
		Scopes: []string{strings.Join([]string{"ZohoCRM.org.READ", "ZohoCRM.coql.READ",
			"ZohoCRM.modules.leads.READ", "ZohoCRM.modules.contacts.READ", "ZohoCRM.modules.accounts.READ", "ZohoCRM.modules.deals.READ",
			"ZohoCRM.modules.leads.CREATE", "ZohoCRM.modules.contacts.CREATE", "ZohoCRM.modules.accounts.CREATE", "ZohoCRM.modules.deals.CREATE",
			"ZohoCRM.modules.leads.UPDATE", "ZohoCRM.modules.contacts.UPDATE", "ZohoCRM.modules.accounts.UPDATE", "ZohoCRM.modules.deals.UPDATE"}, ",")},
		Who: &Who{Req: Req{URL: "/org"}, Names: []Path{"org/0/company_name", "org/0/domain_name"}},
		Actions: map[string]*Action{
			"search": Call[zcrmFind, zcrmFound](Plan{
				Doc: "Find leads, contacts, accounts or deals by a word in any of their fields, by email or phone, or by Zoho's criteria, " +
					"like { module: \"Deals\", criteria: \"(Stage:equals:Negotiation/Review)\" }: at most 2000 in all, 200 a page",
				Clamp: map[string][2]int{"limit": {50, 200}},
				Enum:  map[string][]string{"module": zcrmModules},
				Steps: []Step{{As: "found", Req: Req{URL: "/{module}/search",
					Query: Q{"word": "{word}", "email": "{email}", "phone": "{phone}", "criteria": "{criteria}", "per_page": "{limit}", "page": "{page}"}}}},
				Out: Obj{"data": "{found/data|[]}", "more": "{found/info/more_records|false}"},
			}),
			"get": Call[zcrmOne, any](Plan{
				Doc:   "One lead, contact, account or deal by id, with every field: { data: [record] }",
				Enum:  map[string][]string{"module": zcrmModules},
				Steps: []Step{{Req: Req{URL: "/{module}/{id}"}}},
			}),
			"query": Call[zcrmQuery, any](Plan{
				Doc: "Run a COQL query, Zoho's SQL-like SELECT over one module and its lookups, like { query: \"select Deal_Name, Amount, Account_Name.Account_Name " +
					"from Deals where Stage = 'Closed Won' order by Closing_Date desc limit 100\" }: { data, info: { count, more_records } }; " +
					"a WHERE is required, at most 2000 rows a query, paged with OFFSET",
				Steps: []Step{{As: "got", Req: Req{Method: POST, Read: true, URL: "/coql", Body: Obj{"select_query": "{query}"}}}},
				Out:   Obj{"data": "{got/data|[]}", "info": "{got/info|{}}"},
			}),
			"create": Call[zcrmNew, zcrmDone](Plan{
				Doc: "Add a lead, contact, account or deal with its fields by API name, like { module: \"Leads\", record: { Last_Name, First_Name, Email, Company } } or " +
					"{ module: \"Deals\", record: { Deal_Name, Stage, Amount, Closing_Date, Account_Name: { id } } }. Leads and contacts need Last_Name, accounts " +
					"Account_Name and deals Deal_Name and Stage. Returns { id }",
				Enum: map[string][]string{"module": zcrmModules},
				Steps: []Step{
					{As: "got", Req: Req{Method: POST, URL: "/{module}", Body: Obj{"data": []any{"{record}"}}}},
					zcrmRefused,
				},
				Out: Obj{"id": "{got/data/0/details/id}"},
			}),
			"update": Call[zcrmEdit, zcrmDone](Plan{
				Doc:   "Change a lead, contact, account or deal: only the fields given change, and null clears one. Returns { id }",
				Enum:  map[string][]string{"module": zcrmModules},
				Steps: []Step{{As: "got", Req: Req{Method: PUT, URL: "/{module}/{id}", Body: Obj{"data": []any{"{record}"}}}}, zcrmRefused},
				Out:   Obj{"id": "{id}"},
			}),
		},
		Sync: []*Sync{
			{Name: "accounts", Gap: time.Second, Req: zcrmList, Items: "data", Empty: true, Updated: "Modified_Time", Page: zcrmPage, Change: zcrmChange,
				Vars: Obj{"module": "Accounts", "fields": "Account_Name,Website,Industry,Account_Type,Phone,Billing_City,Billing_State,Billing_Country,Employees,Annual_Revenue,Description,Created_Time,Modified_Time"},
				Doc:  "Every account, with Account_Name, Website, Industry, Account_Type, Phone, Billing_City, Billing_State, Billing_Country, Employees, Annual_Revenue, Description" + zcrmDoc,
				To: Map{Table: "crm_company",
					Cols: map[string]SQL{"title": "data->>'Account_Name'", "domain": "archive.sync_domain(data->>'Website')", "industry": "data->>'Industry'", "stage": "data->>'Account_Type'",
						"phone": "data->>'Phone'", "city": "data->>'Billing_City'", "state": "data->>'Billing_State'", "country": "data->>'Billing_Country'",
						"employees": "archive.sync_num(data->>'Employees')", "revenue": "archive.sync_money(data->>'Annual_Revenue', " + zcrmCurrency + ")",
						"description": "archive.sync_clip(data->>'Description', 65536)", "made_at": "archive.sync_at(data->>'Created_Time')", "edited_at": "archive.sync_at(data->>'Modified_Time')"},
					Orgs: map[string]SQL{"org": "data->>'Website'"}}},
			{Name: "contacts", Gap: time.Second, Req: zcrmList, Items: "data", Empty: true, Updated: "Modified_Time", Page: zcrmPage, Change: zcrmChange,
				Vars: Obj{"module": "Contacts", "fields": "First_Name,Last_Name,Email,Phone,Title,Account_Name,Lead_Source,Mailing_City,Mailing_State,Mailing_Country,Created_Time,Modified_Time"},
				Doc:  "Every contact, with First_Name, Last_Name, Email, Phone, Title, Account_Name, Lead_Source, Mailing_City, Mailing_State, Mailing_Country" + zcrmDoc,
				To: Map{Table: "crm_contact",
					Cols: map[string]SQL{"first_name": "data->>'First_Name'", "last_name": "data->>'Last_Name'", "email": "archive.sync_email(data->>'Email')", "phone": "data->>'Phone'",
						"job_title": "data->>'Title'", "company_name": "data#>>'{Account_Name,name}'", "stage": "'contact'", "city": "data->>'Mailing_City'",
						"state": "data->>'Mailing_State'", "country": "data->>'Mailing_Country'", "made_at": "archive.sync_at(data->>'Created_Time')", "edited_at": "archive.sync_at(data->>'Modified_Time')"},
					People: map[string]SQL{"person": "data->>'Email'"},
					Refs:   map[string]SQL{"company": "data#>>'{Account_Name,id}'"}}},
			// A lead is a contact whose stage is lead until it is converted, when Zoho lists it no more and the contact
			// it became is the one.
			{Name: "leads", Gap: time.Second, Req: zcrmList, Items: "data", Empty: true, Updated: "Modified_Time", Page: zcrmPage, Change: zcrmChange,
				Vars: Obj{"module": "Leads", "fields": "First_Name,Last_Name,Email,Phone,Designation,Company,Lead_Status,Lead_Source,City,State,Country,Created_Time,Modified_Time"},
				Doc:  "Every lead not yet converted, with First_Name, Last_Name, Email, Phone, Designation, Company, Lead_Status, Lead_Source, City, State, Country" + zcrmDoc,
				To: Map{Table: "crm_contact",
					Cols: map[string]SQL{"first_name": "data->>'First_Name'", "last_name": "data->>'Last_Name'", "email": "archive.sync_email(data->>'Email')", "phone": "data->>'Phone'",
						"job_title": "data->>'Designation'", "company_name": "data->>'Company'", "stage": "'lead'", "lead_status": "data->>'Lead_Status'", "city": "data->>'City'",
						"state": "data->>'State'", "country": "data->>'Country'", "made_at": "archive.sync_at(data->>'Created_Time')", "edited_at": "archive.sync_at(data->>'Modified_Time')"},
					People: map[string]SQL{"person": "data->>'Email'"}}},
			// Zoho's deals have no closed or won of their own: a stage that starts Closed is closed, and Closed Won won,
			// as the stages a CRM starts with are named.
			{Name: "deals", Gap: time.Second, Req: zcrmList, Items: "data", Empty: true, Updated: "Modified_Time", Page: zcrmPage, Change: zcrmChange,
				Vars: Obj{"module": "Deals", "fields": "Deal_Name,Amount,Currency,Stage,Pipeline,Closing_Date,Account_Name,Description,Created_Time,Modified_Time"},
				Doc:  "Every deal, with Deal_Name, Amount, Currency, Stage, Pipeline, Closing_Date, Account_Name, Description" + zcrmDoc,
				To: Map{Table: "crm_deal",
					Cols: map[string]SQL{"title": "data->>'Deal_Name'", "amount": "archive.sync_money(data->>'Amount', " + zcrmCurrency + ")", "stage": "data->>'Stage'",
						"pipeline": "CASE jsonb_typeof(data->'Pipeline') WHEN 'object' THEN data#>>'{Pipeline,name}' ELSE data->>'Pipeline' END",
						"closed":   "data->>'Stage' ILIKE 'closed%'", "won": "CASE WHEN data->>'Stage' ILIKE 'closed%' THEN data->>'Stage' ILIKE 'closed won%' END",
						"close_on": "archive.sync_day(data->>'Closing_Date')", "description": "archive.sync_clip(data->>'Description', 65536)",
						"made_at": "archive.sync_at(data->>'Created_Time')", "edited_at": "archive.sync_at(data->>'Modified_Time')"},
					Refs: map[string]SQL{"company": "data#>>'{Account_Name,id}'"}}},
		},
	})

	Register(&Connector{
		ID: "zohobooks", Name: "Zoho Books", Integration: true, Env: "ARCHIVE_ZOHO",
		Info: Info{By: "Zoho", Description: "Read your organizations’ invoices, bills, payments, contacts and books", Category: CatFinance, Icon: IconLedger,
			Scopes: []string{"Read your organizations’ accounting"}},
		Endpoint: zohoEndpoint, Hosts: zohoHosts, Region: "accounts-server", Regions: zohoRegions, Keep: []string{"api_domain"}, TokenType: "Zoho-oauthtoken",
		Revoke: zohoRevoke, Params: zohoParams, Let: zohoLet,
		API: "{+api}/books/v3",
		Scopes: []string{strings.Join([]string{"ZohoBooks.settings.READ", "ZohoBooks.contacts.READ", "ZohoBooks.invoices.READ", "ZohoBooks.bills.READ",
			"ZohoBooks.customerpayments.READ", "ZohoBooks.vendorpayments.READ", "ZohoBooks.accountants.READ"}, ",")},
		Who: &Who{Req: Req{URL: "/organizations"}, Each: "organizations", Names: []Path{"name"}},
		Actions: map[string]*Action{
			"organizations": Call[struct{}, any](Plan{
				Doc:   "The organizations of the account: [{ organization_id, name, currency_code, time_zone, is_default_org, ... }]",
				Steps: []Step{{As: "got", Req: Req{URL: "/organizations"}}},
				Out:   "{got/organizations|[]}",
			}),
			"get": Call[zbooksGet, any](Plan{
				Doc: "Read any of Zoho Books' GET endpoints, like { path: \"/invoices/<invoice_id>\" } for an invoice with its line_items, or " +
					"{ path: \"/invoices\", query: \"status=overdue&page=2\" } or { path: \"/reports/profitandloss\" }: its answer, " +
					"200 a page, with page_context.has_more_page. In the default organization unless given one",
				Steps: []Step{
					{Only: Test{At: "path", Re: `^/[A-Za-z0-9_/.-]*$`, Off: true}, Fail: "path is an endpoint after /books/v3, like /invoices"},
					{Unless: "organization", As: "orgs", Req: Req{URL: "/organizations"}},
					{Unless: "organization", As: "organization", In: "{orgs/organizations/0/organization_id}"},
					{Req: Req{URL: "{+path}?{+query|}", Query: Q{"organization_id": "{organization}"}}},
				},
			}),
		},
		Sync: []*Sync{
			{Name: "accounts", Gap: time.Second, Parts: zbooksOrgs, Req: Req{URL: "/chartofaccounts", Query: Q{"organization_id": "{part}", "per_page": "200", "page": "{page}", "showbalance": "true"}}, Items: "chartofaccounts", ID: "account_id", Updated: "last_modified_time", Page: zbooksPage, Set: zbooksSet,
				Doc: "The chart of accounts of each organization, with balances, the inactive too, by account_id: listed whole every time, 200 a page.",
				To: Map{Table: "ledger_account",
					Cols: map[string]SQL{"number": "data->>'account_code'", "title": "data->>'account_name'", "type": "data->>'account_type'",
						"balance": "archive.sync_money(data->>'current_balance', " + zbooksCurrency + ")", "active": "archive.sync_bool(data->'is_active')",
						"edited_at": zbooksEdited},
					Refs: map[string]SQL{"parent": "nullif(data->>'parent_account_id', '')"}}},
			{Name: "contacts", Gap: time.Second, Parts: zbooksOrgs, Req: zbooksList, Items: "contacts", ID: "contact_id", Updated: "last_modified_time", Page: zbooksPage, Change: zbooksChange, Set: zbooksSet,
				Vars: Obj{"what": "contacts"},
				Doc:  "Every customer and vendor of each organization, the inactive too, by contact_id" + zbooksDoc,
				To: Map{Table: "ledger_contact",
					Cols: map[string]SQL{"kind": "CASE data->>'contact_type' WHEN 'vendor' THEN 'vendor' ELSE 'customer' END", "title": "data->>'contact_name'",
						"company": "data->>'company_name'", "email": "archive.sync_email(data->>'email')", "phone": "coalesce(nullif(data->>'phone', ''), data->>'mobile')",
						"balance": "archive.sync_money(CASE data->>'contact_type' WHEN 'vendor' THEN data->>'outstanding_payable_amount' ELSE data->>'outstanding_receivable_amount' END, " + zbooksCurrency + ")",
						"active":  "CASE data->>'status' WHEN 'active' THEN true WHEN 'inactive' THEN false END", "edited_at": zbooksEdited},
					People: map[string]SQL{"person": "data->>'email'"}}},
			{Name: "invoices", Gap: time.Second, Parts: zbooksOrgs, Req: zbooksList, Items: "invoices", ID: "invoice_id", Updated: "last_modified_time", Page: zbooksPage, Change: zbooksChange,
				Set:  Obj{"organization_id": "{part}", "organization": "{in/name}", "org_currency": "{in/currency}", "kind": "Invoice", "entry": Obj{"number": "{invoice_number}", "on": "{date}", "total": "{total}", "with": "{customer_name}", "with_id": "{customer_id}", "left": "{balance}"}},
				Vars: Obj{"what": "invoices"},
				Doc:  "Every invoice of each organization, by invoice_id" + zbooksDoc,
				To:   zbooksEntry},
			{Name: "bills", Gap: time.Second, Parts: zbooksOrgs, Req: zbooksList, Items: "bills", ID: "bill_id", Updated: "last_modified_time", Page: zbooksPage, Change: zbooksChange,
				Set:  Obj{"organization_id": "{part}", "organization": "{in/name}", "org_currency": "{in/currency}", "kind": "Bill", "entry": Obj{"number": "{bill_number}", "on": "{date}", "total": "{total}", "with": "{vendor_name}", "with_id": "{vendor_id}", "left": "{balance}"}},
				Vars: Obj{"what": "bills"},
				Doc:  "Every bill of each organization, by bill_id" + zbooksDoc,
				To:   zbooksEntry},
			{Name: "customer_payments", Gap: time.Second, Parts: zbooksOrgs, Req: zbooksList, Items: "customerpayments", ID: "payment_id", Updated: "last_modified_time", Page: zbooksPage,
				Set:  Obj{"organization_id": "{part}", "organization": "{in/name}", "org_currency": "{in/currency}", "kind": "CustomerPayment", "entry": Obj{"number": "{payment_number}", "on": "{date}", "total": "{amount}", "with": "{customer_name}", "with_id": "{customer_id}", "left": "{unused_amount}"}},
				Vars: Obj{"what": "customerpayments"},
				Doc:  "Every payment a customer made to each organization, by payment_id: listed whole every time, 200 a page.",
				To:   zbooksEntry},
			{Name: "vendor_payments", Gap: time.Second, Parts: zbooksOrgs, Req: zbooksList, Items: "vendorpayments", ID: "payment_id", Updated: "last_modified_time", Page: zbooksPage,
				Set:  Obj{"organization_id": "{part}", "organization": "{in/name}", "org_currency": "{in/currency}", "kind": "VendorPayment", "entry": Obj{"number": "{payment_number}", "on": "{date}", "total": "{amount}", "with": "{vendor_name}", "with_id": "{vendor_id}", "left": "{balance}"}},
				Vars: Obj{"what": "vendorpayments"},
				Doc:  "Every payment each organization made to a vendor, by payment_id: listed whole every time, 200 a page.",
				To:   zbooksEntry},
			{Name: "journals", Gap: time.Second, Parts: zbooksOrgs, Req: zbooksList, Items: "journals", ID: "journal_id", Updated: "last_modified_time", Page: zbooksPage, Change: zbooksChange,
				Set:  Obj{"organization_id": "{part}", "organization": "{in/name}", "org_currency": "{in/currency}", "kind": "Journal", "entry": Obj{"number": "{entry_number}", "on": "{journal_date}", "total": "{total}"}},
				Vars: Obj{"what": "journals"},
				Doc:  "Every manual journal of each organization, by journal_id" + zbooksDoc,
				To:   zbooksEntry},
		},
	})
}

const (
	zcrmDoc = ", by its id: listed whole the first time and a week after, 200 a page, which deletes those gone, and in between those modified since the sync before, newest first."
	// A record of a CRM with one currency names none: US dollars.
	zcrmCurrency SQL = "coalesce(nullif(data->>'Currency', ''), 'USD')"
	zbooksDoc        = ": listed whole the first time and a week after, 200 a page, which deletes those gone, and in between those modified since the sync before, newest first."
	// An item without its own currency is in its organization's.
	zbooksCurrency SQL = "coalesce(nullif(data->>'currency_code', ''), nullif(data->>'org_currency', ''), 'USD')"
	zbooksEdited   SQL = "archive.sync_at(coalesce(nullif(data->>'last_modified_time', ''), data->>'created_time'))"
)

var (
	zohoEndpoint = oauth2.Endpoint{AuthURL: "https://accounts.zoho.com/oauth/v2/auth", TokenURL: "https://accounts.zoho.com/oauth/v2/token", AuthStyle: oauth2.AuthStyleInParams}
	zohoHosts    = []string{"https://accounts.zoho.com"}
	zohoRevoke   = "https://accounts.zoho.com/oauth/v2/token/revoke"
	zohoParams   = map[string]string{"access_type": "offline", "prompt": "consent"}
	// The accounts servers of Zoho's data centers, as https://accounts.zoho.com/oauth/serverinfo lists them.
	zohoRegions = []string{"https://accounts.zoho.com", "https://accounts.zoho.eu", "https://accounts.zoho.in", "https://accounts.zoho.com.au",
		"https://accounts.zoho.jp", "https://accounts.zoho.uk", "https://accounts.zoho.sg", "https://accounts.zoho.ae", "https://accounts.zoho.sa",
		"https://accounts.zohocloud.ca", "https://accounts.zohohq.in"}
	// The API is the one the token's answer named, if it is Zoho's, as a token goes nowhere else.
	zohoLet = []Step{
		{As: "api", In: "{meta/api_domain}", Find: `^(https://(?:[a-z0-9-]+\.)*zohoapis\.[a-z]+(?:\.[a-z]+)?)/?$`},
		{Unless: "api", Fail: "Zoho named no API of its own for this account: {meta/api_domain|nothing}"},
	}

	zcrmModules = []string{"Leads", "Contacts", "Accounts", "Deals"}
	zcrmList    = Req{URL: "/{module}", Query: Q{"fields": "{fields}", "per_page": "200", "page_token": "{page}"}}
	// The first 2000 records page by number, and the rest only by the token each page names.
	zcrmPage   = Cursor{At: "info/next_page_token", More: "info/more_records"}
	zcrmChange = NewestFirst{Query: Q{"sort_by": "Modified_Time", "sort_order": "desc"}, Expired: []int{400}}
	// Zoho answers a write of one record with its outcome in data, which may refuse it with a 2xx.
	zcrmRefused = Step{Only: Test{At: "got/data/0/status", Is: "error"}, Fail: "Zoho refused it: {got/data/0/code}, {got/data/0/message} {got/data/0/details}"}

	// Every organization of the account is a part, with its currency.
	zbooksOrgs = &Parts{Req: Req{URL: "/organizations"}, Items: "organizations", ID: "{organization_id}", Vals: Obj{"name": "{name}", "currency": "{currency_code}"}}
	zbooksList = Req{URL: "/{what}", Query: Q{"organization_id": "{part}", "per_page": "200", "page": "{page}"}}
	zbooksPage = Offset{From: 1, Size: 200, Step: 1}
	// A list that cannot sort by last_modified_time lists whole.
	zbooksChange = NewestFirst{Query: Q{"sort_column": "last_modified_time", "sort_order": "D"}, Expired: []int{400}}
	zbooksSet    = Obj{"organization_id": "{part}", "organization": "{in/name}", "org_currency": "{in/currency}"}
	// A transaction as an entry of the books, from what its stream's Set reads into entry: its number, date, who it is
	// with and their id, its total and what is left of it.
	zbooksEntry = Map{Table: "ledger_entry",
		Cols: map[string]SQL{"kind": "data->>'kind'", "number": "data#>>'{entry,number}'", "txn_on": "archive.sync_day(data#>>'{entry,on}')",
			"due_on": "archive.sync_day(data->>'due_date')", "contact_name": "data#>>'{entry,with}'",
			"total": "archive.sync_money(data#>>'{entry,total}', " + zbooksCurrency + ")", "balance": "archive.sync_money(data#>>'{entry,left}', " + zbooksCurrency + ")",
			"memo":   `concat_ws(E'\n\n', nullif(btrim(data->>'reference_number'), ''), nullif(btrim(data->>'notes'), ''), nullif(btrim(data->>'description'), ''))`,
			"status": "data->>'status'", "edited_at": zbooksEdited},
		Refs: map[string]SQL{"contact": "nullif(data#>>'{entry,with_id}', '')", "account": "nullif(data->>'account_id', '')"}}
)

type (
	zcrmFind struct {
		Module   string `json:"module" jsonschema:"Leads, Contacts, Accounts or Deals"`
		Word     string `json:"word,omitempty" jsonschema:"A word in any of its fields"`
		Email    string `json:"email,omitempty" jsonschema:"An email address it has"`
		Phone    string `json:"phone,omitempty" jsonschema:"A phone number it has"`
		Criteria string `json:"criteria,omitempty" jsonschema:"Zoho's criteria, like ((Last_Name:equals:Burns)and(Lead_Status:equals:Contacted)); with starts_with, equals, not_equal, greater_than or less_than"`
		Limit    int    `json:"limit,omitempty" jsonschema:"At most this many a page, 1 to 200; 50 when left out"`
		Page     int    `json:"page,omitempty" jsonschema:"The page, from 1"`
	}
	zcrmFound struct {
		Data []map[string]any `json:"data"`
		More bool             `json:"more" jsonschema:"There is another page"`
	}
	zcrmOne struct {
		Module string `json:"module" jsonschema:"Leads, Contacts, Accounts or Deals"`
		ID     string `json:"id" jsonschema:"The record's id"`
	}
	zcrmQuery struct {
		Query string `json:"query" jsonschema:"The COQL query, like select Last_Name, Email from Contacts where Mailing_City = 'Austin' limit 200"`
	}
	zcrmNew struct {
		Module string         `json:"module" jsonschema:"Leads, Contacts, Accounts or Deals"`
		Record map[string]any `json:"record" jsonschema:"Its fields by API name, like Last_Name, Email, Account_Name, Deal_Name, Stage or Amount; a lookup as { id }"`
	}
	zcrmEdit struct {
		Module string         `json:"module" jsonschema:"Leads, Contacts, Accounts or Deals"`
		ID     string         `json:"id" jsonschema:"The record's id"`
		Record map[string]any `json:"record" jsonschema:"The fields to change by API name; null clears one"`
	}
	zcrmDone struct {
		ID string `json:"id"`
	}
	zbooksGet struct {
		Path         string `json:"path" jsonschema:"The endpoint after /books/v3, like /invoices, /invoices/<invoice_id>, /bills, /contacts, /chartofaccounts or /reports/balancesheet"`
		Query        string `json:"query,omitempty" jsonschema:"Its query string, like status=unpaid&per_page=200&page=2"`
		Organization string `json:"organization,omitempty" jsonschema:"The organization_id; the account's first organization when left out"`
	}
)
