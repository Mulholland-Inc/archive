package connect

import (
	"time"

	"golang.org/x/oauth2"
)

func init() {
	Register(&Connector{
		ID: "salesforce", Name: "Salesforce", Integration: true, Env: "ARCHIVE_SALESFORCE",
		Info:  Info{By: "Salesforce", Description: "Read accounts, contacts, leads and opportunities", Category: CatCRM, Icon: IconUsers, Scopes: []string{"Read your org’s records through the API"}},
		Hosts: []string{"https://login.salesforce.com"},
		API:   "{+host}",
		Let: []Step{
			{As: "who", Req: Req{URL: "https://login.salesforce.com/services/oauth2/userinfo"}},
			{Unless: "who/urls/rest", Fail: "Salesforce did not name the org's API"},
			{As: "host", In: "{who/urls/rest}", Find: `^(https?://[^/]+)`},
		},
		Endpoint: oauth2.Endpoint{AuthURL: "https://login.salesforce.com/services/oauth2/authorize",
			TokenURL: "https://login.salesforce.com/services/oauth2/token", AuthStyle: oauth2.AuthStyleInParams},
		Revoke: "https://login.salesforce.com/services/oauth2/revoke",
		Scopes: []string{"api", "refresh_token"},
		Params: map[string]string{"prompt": "login consent"},
		Who:    &Who{Req: Req{URL: sfData + "/query", Query: Q{"q": "SELECT Name FROM Organization"}}, Names: []Path{"records/0/Name"}},
		Actions: map[string]*Action{
			"query": Call[sfQuery, any](Plan{
				Doc: "Run a SOQL query, like { query: \"SELECT Id, Name, StageName, Amount FROM Opportunity WHERE IsClosed = false\" }: " +
					"{ totalSize, done, records, nextRecordsUrl } (pass nextRecordsUrl as page for more)",
				Steps: []Step{
					{If: "page", Only: Test{At: "page", Re: "^/services/data/", Off: true}, Fail: "page is the nextRecordsUrl of a query before"},
					{If: "page", Req: Req{URL: "{+page}"}},
					{Unless: "page", Req: Req{URL: sfData + "/query", Query: Q{"q": "{query}"}}},
				},
			}),
			"search": Call[sfSearch, any](Plan{
				Doc: "Find records by words in any field, with SOSL, like { query: \"FIND {Acme} IN ALL FIELDS RETURNING Account(Id, Name), Contact(Id, Name, Email)\" }: " +
					"{ searchRecords: [{ attributes, Id, ... }] }",
				Steps: []Step{{Req: Req{URL: sfData + "/search", Query: Q{"q": "{query}"}}}},
			}),
		},
		Sync: []*Sync{
			{Name: "accounts", Gap: time.Second, Req: sfList, Items: "records", ID: "Id", Updated: "SystemModstamp", Page: sfPage, Change: sfChange, Gone: sfGone, Deny: sfDeny, Restart: sfRestart,
				Vars: Obj{"object": "Account", "fields": "Name, Website, Industry, Type, Phone, BillingCity, BillingState, BillingCountry, NumberOfEmployees, AnnualRevenue, Description, CreatedDate"},
				Doc:  "Every Account, by its Id, with Name, Website, Industry, Type, Phone, BillingCity, BillingState, BillingCountry, NumberOfEmployees, AnnualRevenue, Description, CreatedDate" + sfDoc,
				To: Map{Table: "crm_company",
					Cols: map[string]SQL{"title": "data->>'Name'", "domain": "archive.sync_domain(data->>'Website')", "industry": "data->>'Industry'", "stage": "data->>'Type'",
						"phone": "data->>'Phone'", "city": "data->>'BillingCity'", "state": "data->>'BillingState'", "country": "data->>'BillingCountry'",
						"employees": "archive.sync_num(data->>'NumberOfEmployees')", "revenue": "archive.sync_money(data->>'AnnualRevenue', 'USD')",
						"description": "archive.sync_clip(data->>'Description', 65536)", "made_at": "archive.sync_at(data->>'CreatedDate')", "edited_at": "archive.sync_at(data->>'SystemModstamp')"},
					Orgs: map[string]SQL{"org": "data->>'Website'"}}},
			{Name: "contacts", Gap: time.Second, Req: sfList, Items: "records", ID: "Id", Updated: "SystemModstamp", Page: sfPage, Change: sfChange, Gone: sfGone, Deny: sfDeny, Restart: sfRestart,
				Vars: Obj{"object": "Contact", "fields": "FirstName, LastName, Email, Phone, Title, AccountId, Account.Name, MailingCity, MailingState, MailingCountry, CreatedDate"},
				Doc:  "Every Contact, by its Id, with FirstName, LastName, Email, Phone, Title, AccountId, Account.Name, MailingCity, MailingState, MailingCountry, CreatedDate" + sfDoc,
				To: Map{Table: "crm_contact",
					Cols: map[string]SQL{"first_name": "data->>'FirstName'", "last_name": "data->>'LastName'", "email": "archive.sync_email(data->>'Email')", "phone": "data->>'Phone'",
						"job_title": "data->>'Title'", "company_name": "data#>>'{Account,Name}'", "stage": "'contact'", "city": "data->>'MailingCity'",
						"state": "data->>'MailingState'", "country": "data->>'MailingCountry'", "made_at": "archive.sync_at(data->>'CreatedDate')", "edited_at": "archive.sync_at(data->>'SystemModstamp')"},
					People: map[string]SQL{"person": "data->>'Email'"},
					Refs:   map[string]SQL{"company": "data->>'AccountId'"}}},
			{Name: "leads", Gap: time.Second, Req: sfList, Items: "records", ID: "Id", Updated: "SystemModstamp", Page: sfPage, Change: sfChange, Deny: sfDeny, Restart: sfRestart,
				Gone: []Test{{At: "IsDeleted", Is: "true"}, {At: "IsConverted", Is: "true"}},
				Vars: Obj{"object": "Lead", "fields": "FirstName, LastName, Email, Phone, Title, Company, Status, City, State, Country, IsConverted, CreatedDate"},
				Doc:  "Every Lead, by its Id, with FirstName, LastName, Email, Phone, Title, Company, Status, City, State, Country, IsConverted, CreatedDate" + sfDoc,
				To: Map{Table: "crm_contact",
					Cols: map[string]SQL{"first_name": "data->>'FirstName'", "last_name": "data->>'LastName'", "email": "archive.sync_email(data->>'Email')", "phone": "data->>'Phone'",
						"job_title": "data->>'Title'", "company_name": "data->>'Company'", "stage": "'lead'", "lead_status": "data->>'Status'", "city": "data->>'City'",
						"state": "data->>'State'", "country": "data->>'Country'", "made_at": "archive.sync_at(data->>'CreatedDate')", "edited_at": "archive.sync_at(data->>'SystemModstamp')"},
					People: map[string]SQL{"person": "data->>'Email'"}}},
			{Name: "opportunities", Gap: time.Second, Req: sfList, Items: "records", ID: "Id", Updated: "SystemModstamp", Page: sfPage, Change: sfChange, Gone: sfGone, Deny: sfDeny, Restart: sfRestart,
				Vars: Obj{"object": "Opportunity", "fields": "Name, Amount, StageName, CloseDate, IsClosed, IsWon, AccountId, Description, CreatedDate"},
				Doc:  "Every Opportunity, by its Id, with Name, Amount, StageName, CloseDate, IsClosed, IsWon, AccountId, Description, CreatedDate" + sfDoc,
				To: Map{Table: "crm_deal",
					Cols: map[string]SQL{"title": "data->>'Name'", "amount": "archive.sync_money(data->>'Amount', coalesce(nullif(data->>'CurrencyIsoCode', ''), 'USD'))", "stage": "data->>'StageName'",
						"closed": "archive.sync_bool(data->'IsClosed')", "won": "archive.sync_bool(data->'IsWon')", "close_on": "archive.sync_day(data->>'CloseDate')",
						"description": "archive.sync_clip(data->>'Description', 65536)", "made_at": "archive.sync_at(data->>'CreatedDate')", "edited_at": "archive.sync_at(data->>'SystemModstamp')"},
					Refs: map[string]SQL{"company": "data->>'AccountId'"}}},
		},
	})
}

const (
	sfData = "/services/data/v62.0"
	sfDoc  = ": listed whole the first time and a week after, 2000 a page, which deletes those gone for good, and in between what queryAll says changed since the sync before, deleted too."
)

var (
	sfList    = Req{URL: sfData + "/query", Query: Q{"q": "SELECT Id, SystemModstamp, {fields} FROM {object}"}}
	sfPage    = NextURL{At: "nextRecordsUrl"}
	sfChange  = SinceTime{Req: Req{URL: sfData + "/queryAll", Query: Q{"q": "SELECT Id, SystemModstamp, IsDeleted, {fields} FROM {object} WHERE SystemModstamp > {since}"}}}
	sfGone    = []Test{{At: "IsDeleted", Is: "true"}}
	sfDeny    = []Deny{{Status: []int{400}, Why: "Salesforce does not let Archive read {object} in this org: {message}"}}
	sfRestart = []int{400, 404}
)

type (
	sfQuery struct {
		Query string `json:"query,omitempty" jsonschema:"The SOQL query"`
		Page  string `json:"page,omitempty" jsonschema:"The nextRecordsUrl of the query before"`
	}
	sfSearch struct {
		Query string `json:"query" jsonschema:"The SOSL search"`
	}
)
