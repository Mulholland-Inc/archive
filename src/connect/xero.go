package connect

import (
	"time"

	"golang.org/x/oauth2"
)

// Xero, through its accounting API. It reads ARCHIVE_XERO, the keys of a web app from developer.xero.com, whose
// redirect URIs list <API>/connect/xero/callback. An app made since March 2026 takes only Xero's granular scopes,
// so it asks for each kind of record it reads, writing only contacts and invoices; offline_access is what asks for
// a refresh token, which lasts 60 days and is replaced at every refresh.
//
// Every call names the organisation it is for, in the xero-tenant-id header. A sign-in can choose more than one,
// so the connection keeps the first organisation the sign-in that made it chose, as its access token's
// authentication_event_id names them to /connections, with the organisation's base currency, which amounts that
// name none are in; a person connects again for another. Xero lets a tenant 60 calls a minute and 5,000 a day.
//
// It syncs the books into the ledger tables: the chart of accounts and items, listed whole each sync, as Xero
// deletes those outright; and contacts and every kind of transaction with its lines, a stream each, listed
// whole the first time, 1,000 a page or Xero's 100 where it takes no other, and after that only what changed
// since the sync before, as If-Modified-Since asks. Xero keeps what is deleted, as DELETED, which such a change
// says; a contact is archived rather than deleted. Pages are in the order of ids, so one made while a listing
// runs, which a page then lists twice or not at all, is in the change after.
//
// Its pushes are the Xero app's webhooks, set up once for it: its endpoint <API>/push/xero, contact and invoice
// events, and its webhooks key the hook option of ARCHIVE_XERO. x-xero-signature is the base64 HMAC-SHA256 of the
// body with that key, and each event names the organisation it is for at tenantId. Xero proves the endpoint before
// it sends anything, with pushes signed right and wrong, which Archive answers 200 and 401.
func init() {
	Register(&Connector{
		ID: "xero", Name: "Xero", Integration: true, Options: "?hook=<webhooks key> for its pushes",
		Info: Info{By: "Xero", Description: "Read your organisation’s invoices, contacts, books and reports, and add or change contacts and invoices when you ask",
			Category: CatFinance, Icon: IconLedger, Scopes: []string{"Read one organisation’s accounting and reports", "Add, change and void contacts and invoices when you ask"}},
		API: "https://api.xero.com/api.xro/2.0",
		Let: []Step{
			{Unless: "meta/tenant", As: "auth", In: "{token}", Codec: Claims},
			{Unless: "meta/tenant", As: "found", Req: Req{URL: "https://api.xero.com/connections", Query: Q{"authEventId": "{auth/authentication_event_id}"}}},
			{Unless: "meta/tenant", As: "orgs", Each: "found", Only: Test{At: "tenantType", Is: "ORGANISATION"}, In: "{tenantId}"},
			{Unless: "meta/tenant", As: "meta/tenant", In: "{orgs/0}"},
			{Unless: "meta/tenant", Fail: "Xero was connected to no organisation; connect it again and choose one"},
			{Unless: "meta/currency", As: "org", Req: Req{URL: "https://api.xero.com/api.xro/2.0/Organisation", Header: map[string][]string{"xero-tenant-id": {"{meta/tenant}"}}}},
			{Unless: "meta/currency", As: "meta/currency", In: "{org/Organisations/0/BaseCurrency}"},
		},
		Sign: map[string]string{"Authorization": "Bearer {token}", "xero-tenant-id": "{meta/tenant}"},
		// Xero tags every answer with this, which its support asks for.
		Trace: "Xero-Correlation-Id",
		Endpoint: oauth2.Endpoint{AuthURL: "https://login.xero.com/identity/connect/authorize", TokenURL: "https://identity.xero.com/connect/token",
			AuthStyle: oauth2.AuthStyleInHeader},
		RevokeBy: &Req{Method: POST, URL: "https://identity.xero.com/connect/revocation", Header: map[string][]string{"Authorization": {"{basic}"}}, Form: Q{"token": "{refresh}"}},
		Scopes: []string{"offline_access", "accounting.settings.read", "accounting.contacts", "accounting.invoices", "accounting.payments.read",
			"accounting.banktransactions.read", "accounting.manualjournals.read", "accounting.reports.profitandloss.read",
			"accounting.reports.balancesheet.read", "accounting.reports.trialbalance.read", "accounting.reports.aged.read"},
		Who: &Who{Req: Req{URL: "/Organisation"}, Names: []Path{"Organisations/0/Name"}},
		Actions: map[string]*Action{
			"organisation": Call[struct{}, any](Plan{
				Doc:   "The organisation connected: its Organisation, with Name, LegalName, BaseCurrency, CountryCode, FinancialYearEndDay and FinancialYearEndMonth, TaxNumber and more",
				Steps: []Step{{As: "got", Req: Req{URL: "/Organisation"}}},
				Out:   "{got/Organisations/0}",
			}),
			"list": Call[xeroFind, any](Plan{
				Doc: "List records of a type, maybe only those a where filter passes, like { type: \"Invoices\", where: \"Type==\\\"ACCREC\\\" AND Status==\\\"AUTHORISED\\\" AND AmountDue>0\", order: \"DueDate\" }: " +
					"{ <type>: [...], pagination }. A where compares fields with ==, !=, <, >, AND and OR, a date like Date>=DateTime(2026,01,01) and an id like " +
					"Contact.ContactID==guid(\"…\"); Invoices, Contacts and BankTransactions list 100 a page with their lines, as page asks, and Accounts and Items all at once",
				Clamp: map[string][2]int{"page": {1, 100000}},
				Enum:  map[string][]string{"type": xeroTypes},
				Steps: []Step{{Req: Req{URL: "/{type}", Query: Q{"where": "{where}", "order": "{order}", "page": "{page}", "includeArchived": "{archived|}"}}}},
			}),
			"get": Call[xeroOne, any](Plan{
				Doc:   "One record of a type by its id, like { type: \"Invoices\", id }, or an invoice by its number: { <type>: [record] }, an invoice with its lines, payments and credit notes, a contact with its balances",
				Enum:  map[string][]string{"type": xeroTypes},
				Steps: []Step{{Req: Req{URL: "/{type}/{id}"}}},
			}),
			"report": Call[xeroReport, any](Plan{
				Doc: "A report as Xero lays it out: { Reports: [{ ReportName, ReportDate, Rows }] }, each row a Header, Section or Row of Cells. ProfitAndLoss takes fromDate and toDate, " +
					"BalanceSheet and TrialBalance a date, and with periods and timeframe (MONTH, QUARTER or YEAR) ProfitAndLoss and BalanceSheet compare as many periods before; " +
					"AgedReceivablesByContact and AgedPayablesByContact take a contactId and a date",
				Enum:  map[string][]string{"report": {"ProfitAndLoss", "BalanceSheet", "TrialBalance", "AgedReceivablesByContact", "AgedPayablesByContact"}},
				Steps: []Step{{Req: Req{URL: "/Reports/{report}", Query: Q{"date": "{date}", "fromDate": "{fromDate}", "toDate": "{toDate}", "periods": "{periods}", "timeframe": "{timeframe}", "contactId": "{contactId}"}}}},
			}),
			"contact": Call[xeroContactIn, any](Plan{
				Doc: "Add a contact, or change one when it has its ContactID: only the fields given change. Like { contact: { Name, FirstName, LastName, EmailAddress, IsCustomer: true } }, " +
					"or { contact: { ContactID, ContactStatus: \"ARCHIVED\" } } to archive one, which ACTIVE undoes. Returns the contact",
				Steps: []Step{{As: "got", Req: Req{Method: POST, URL: "/Contacts", Header: xeroOnce, Body: Obj{"Contacts": []any{"{contact}"}}}}},
				Out:   "{got/Contacts/0}",
			}),
			"invoice": Call[xeroInvoiceIn, any](Plan{
				Doc: "Add an invoice (Type ACCREC) or a bill (ACCPAY), or change one when it has its InvoiceID. Like { invoice: { Type: \"ACCREC\", Contact: { ContactID }, Date, DueDate, " +
					"LineItems: [{ Description, Quantity, UnitAmount, AccountCode }] } }; it is a DRAFT unless Status says SUBMITTED or AUTHORISED. Returns the invoice",
				Steps: []Step{{As: "got", Req: Req{Method: POST, URL: "/Invoices", Header: xeroOnce, Body: Obj{"Invoices": []any{"{invoice}"}}}}},
				Out:   "{got/Invoices/0}",
			}),
			"void": Call[xeroVoid, any](Plan{
				Doc:   "Void an approved invoice or bill that has no payments: Xero keeps it, VOIDED, at no amount, and it cannot be approved again. Returns the invoice",
				Steps: []Step{{As: "got", Req: Req{Method: POST, URL: "/Invoices/{id}", Header: xeroOnce, Body: Obj{"Invoices": []Obj{{"InvoiceID": "{id}", "Status": "VOIDED"}}}}}},
				Out:   "{got/Invoices/0}",
			}),
		},
		Listen: &Listen{Key: "{meta/tenant}", Keys: []Path{"events/*/tenantId"}, Signs: []Sign{{Header: "x-xero-signature", Hook: true, Of: "{body}"}}},
		Sync: []*Sync{
			{Name: "accounts", Gap: time.Second, Req: Req{URL: "/Accounts"}, Items: "Accounts", ID: "AccountID", Set: xeroSet,
				Doc: "The chart of accounts, the archived too, listed whole each sync.",
				To: Map{Table: "ledger_account",
					Cols: map[string]SQL{"number": "data->>'Code'", "title": "data->>'Name'", "type": "data->>'Type'", "subtype": "data->>'BankAccountType'", "class": "initcap(data->>'Class')",
						"active": "data->>'Status' = 'ACTIVE'", "edited_at": xeroEditedAt}}},
			{Name: "items", Gap: time.Second, Req: Req{URL: "/Items"}, Items: "Items", ID: "ItemID", Set: xeroSet,
				Doc: "Every product and service, listed whole each sync.",
				// Xero names the accounts of an item by code, not id, so they are not linked.
				To: Map{Table: "ledger_item",
					Cols: map[string]SQL{"title": "coalesce(nullif(data->>'Name', ''), data->>'Code')", "sku": "data->>'Code'",
						"type": "CASE WHEN data->'IsTrackedAsInventory' = 'true' THEN 'Inventory' ELSE 'NonInventory' END", "description": "data->>'Description'",
						"price": "archive.sync_money(data#>>'{SalesDetails,UnitPrice}', data->>'base')", "cost": "archive.sync_money(data#>>'{PurchaseDetails,UnitPrice}', data->>'base')",
						"quantity": "CASE WHEN data->'IsTrackedAsInventory' = 'true' THEN archive.sync_num(data->>'QuantityOnHand') END", "edited_at": xeroEditedAt}}},
			{Name: "contacts", Gap: time.Second, Req: xeroList, Items: "{entity}", ID: "ContactID", Page: xeroPage, Change: xeroChange, Set: xeroSet,
				Vars: Obj{"entity": "Contacts", "id": "ContactID", "size": "1000", "archived": "true"},
				Doc:  "Every contact, the archived too" + xeroDoc,
				To:   xeroContact},
			{Name: "invoices", Gap: time.Second, Req: xeroList, Items: "{entity}", ID: "InvoiceID", Page: xeroPage, Change: xeroChange, Set: xeroSet, Gone: xeroGone, To: xeroEntry,
				Vars: Obj{"entity": "Invoices", "id": "InvoiceID", "size": "1000"}, Doc: "Every invoice and bill" + xeroDoc},
			{Name: "credit_notes", Gap: time.Second, Req: xeroList, Items: "{entity}", ID: "CreditNoteID", Page: xeroPage, Change: xeroChange, Set: xeroSet, Gone: xeroGone, To: xeroEntry,
				Vars: Obj{"entity": "CreditNotes", "id": "CreditNoteID", "size": "1000"}, Doc: "Every credit note, to a customer or from a supplier" + xeroDoc},
			{Name: "payments", Gap: time.Second, Req: xeroList, Items: "{entity}", ID: "PaymentID", Page: xeroPage, Change: xeroChange, Set: xeroSet, Gone: xeroGone, To: xeroEntry,
				Vars: Obj{"entity": "Payments", "id": "PaymentID", "size": "1000", "kind": "Payment"}, Doc: "Every payment of an invoice, bill or credit note" + xeroDoc},
			{Name: "bank_transactions", Gap: time.Second, Req: xeroList, Items: "{entity}", ID: "BankTransactionID", Page: xeroPage, Change: xeroChange, Set: xeroSet, Gone: xeroGone, To: xeroEntry,
				Vars: Obj{"entity": "BankTransactions", "id": "BankTransactionID", "size": "1000"}, Doc: "Every money spent or received that is not an invoice's payment" + xeroDoc},
			{Name: "bank_transfers", Gap: time.Second, Req: Req{URL: "/BankTransfers"}, Items: "BankTransfers", ID: "BankTransferID", Set: xeroSet, To: xeroEntry,
				Change: SinceTime{Req: Req{URL: "/BankTransfers", Header: xeroSince}},
				Vars:   Obj{"kind": "Transfer"},
				Doc:    "Every transfer between bank accounts, all at once: listed whole the first time and a week after, which deletes those Xero no longer has, and in between what changed since the sync before."},
			{Name: "manual_journals", Gap: time.Second, Req: xeroList, Items: "{entity}", ID: "ManualJournalID", Page: xeroPage, Change: xeroChange, Set: xeroSet, Gone: xeroGone, To: xeroEntry,
				Vars: Obj{"entity": "ManualJournals", "id": "ManualJournalID", "size": "1000", "kind": "ManualJournal"}, Doc: "Every manual journal" + xeroDoc},
			{Name: "purchase_orders", Gap: time.Second, Req: xeroList, Items: "{entity}", ID: "PurchaseOrderID", Page: xeroPage100, Change: xeroChange, Set: xeroSet, Gone: xeroGone, To: xeroEntry,
				Vars: Obj{"entity": "PurchaseOrders", "id": "PurchaseOrderID", "kind": "PurchaseOrder"}, Doc: "Every purchase order" + xeroDoc100},
			{Name: "quotes", Gap: time.Second, Req: xeroList, Items: "{entity}", ID: "QuoteID", Page: xeroPage100, Change: xeroChange, Set: xeroSet, Gone: xeroGone, To: xeroEntry,
				Vars: Obj{"entity": "Quotes", "id": "QuoteID", "kind": "Quote"}, Doc: "Every quote" + xeroDoc100},
		},
	})
}

const (
	xeroDoc    = ", with its lines: listed whole the first time, 1,000 a page, and then what changed since the sync before, deleted too."
	xeroDoc100 = ", with its lines: listed whole the first time, 100 a page, and then what changed since the sync before, deleted too."
	// Xero writes a time as /Date(<Unix milliseconds>+0000)/, and a day too, but for the ISO day most records have
	// beside it.
	xeroEditedAt SQL = `archive.sync_at(substring(data->>'UpdatedDateUTC' from '^/Date\((\d+)'))`
	// What amounts are in: the record's currency, or the organisation's.
	xeroCurrency = "coalesce(nullif(data->>'CurrencyCode', ''), data->>'base')"
)

var (
	xeroTypes = []string{"Invoices", "CreditNotes", "Payments", "BankTransactions", "BankTransfers", "ManualJournals", "PurchaseOrders", "Quotes",
		"Contacts", "Items", "Accounts", "Overpayments", "Prepayments"}
	// A write names itself, so that Xero makes one record of it however often it is sent.
	xeroOnce = map[string][]string{"Idempotency-Key": {"{nonce}"}}
	xeroSet  = Obj{"kind": "{kind}", "base": "{meta/currency}"}
	// Pages are in the order of ids, which never change; an entity that cannot be ordered so lists in Xero's order.
	xeroList = Req{URL: "/{entity}", Query: Q{"page": "{page}", "pageSize": "{size|}", "order": "{id} ASC", "includeArchived": "{archived|}"},
		Or: &Req{URL: "/{entity}", Query: Q{"page": "{page}", "pageSize": "{size|}", "includeArchived": "{archived|}"}}}
	xeroPage    = Offset{From: 1, Size: 1000, Step: 1}
	xeroPage100 = Offset{From: 1, Size: 100, Step: 1}
	xeroSince   = map[string][]string{"If-Modified-Since": {"{since}"}}
	xeroChange  = SinceTime{Req: Req{URL: "/{entity}", Query: Q{"page": "{page}", "pageSize": "{size|}", "includeArchived": "{archived|}"}, Header: xeroSince}, Lasts: true}
	xeroGone    = []Test{{At: "Status", Is: "DELETED"}}
	// A contact is a vendor when it is only a supplier, and else a customer, which is what it owes, or a vendor is
	// owed, now; its phone is its default one, and its address its postal one, or its street one, on one line.
	xeroContact = Map{Table: "ledger_contact",
		Cols: map[string]SQL{"kind": xeroKind, "title": "data->>'Name'", "email": "archive.sync_email(data->>'EmailAddress')",
			"phone": `(SELECT nullif(concat_ws(' ', nullif(p->>'PhoneCountryCode', ''), nullif(p->>'PhoneAreaCode', ''), nullif(p->>'PhoneNumber', '')), '')
				FROM archive.sync_list(data->'Phones') p WHERE p->>'PhoneType' = 'DEFAULT' AND nullif(p->>'PhoneNumber', '') IS NOT NULL LIMIT 1)`,
			"address": `(SELECT nullif(concat_ws(', ', nullif(btrim(a->>'AddressLine1'), ''), nullif(btrim(a->>'AddressLine2'), ''), nullif(btrim(a->>'AddressLine3'), ''),
				nullif(btrim(a->>'AddressLine4'), ''), nullif(btrim(a->>'City'), ''), nullif(btrim(a->>'Region'), ''), nullif(btrim(a->>'PostalCode'), ''), nullif(btrim(a->>'Country'), '')), '')
				FROM archive.sync_list(data->'Addresses') a WHERE nullif(btrim(concat(a->>'AddressLine1', a->>'City', a->>'PostalCode')), '') IS NOT NULL ORDER BY a->>'AddressType' = 'POBOX' DESC LIMIT 1)`,
			"balance": "archive.sync_money(CASE " + xeroKind + " WHEN 'vendor' THEN data#>>'{Balances,AccountsPayable,Outstanding}' ELSE data#>>'{Balances,AccountsReceivable,Outstanding}' END, " +
				"coalesce(nullif(data->>'DefaultCurrency', ''), data->>'base'))",
			"active": "data->>'ContactStatus' = 'ACTIVE'", "edited_at": xeroEditedAt},
		People: map[string]SQL{"person": "data->>'EmailAddress'"}}
	// A transaction as an entry of the books, with its lines, or a manual journal's, whose positive amounts are
	// debits and add up to its total. An invoice or credit note is a customer's or a supplier's by its Type, and a
	// bank transaction spends or receives; who it is with is its contact, or its invoice's or credit note's for a
	// payment, and the account it pays from or into its bank account, or a payment's account.
	xeroEntry = Map{Table: "ledger_entry",
		Cols: map[string]SQL{"kind": `CASE WHEN data->>'Type' = 'ACCREC' THEN 'Invoice' WHEN data->>'Type' = 'ACCPAY' THEN 'Bill' WHEN data->>'Type' = 'ACCRECCREDIT' THEN 'CreditNote'
				WHEN data->>'Type' = 'ACCPAYCREDIT' THEN 'BillCredit' WHEN data->>'Type' LIKE 'SPEND%' THEN 'Spend' WHEN data->>'Type' LIKE 'RECEIVE%' THEN 'Receive' ELSE data->>'kind' END`,
			"number": "coalesce(nullif(data->>'InvoiceNumber', ''), nullif(data->>'CreditNoteNumber', ''), nullif(data->>'PurchaseOrderNumber', ''), data->>'QuoteNumber')",
			"txn_on": xeroOn, "due_on": xeroDue,
			"contact_name": "coalesce(data#>>'{Contact,Name}', data#>>'{Invoice,Contact,Name}', data#>>'{CreditNote,Contact,Name}')",
			"total": `archive.sync_money(coalesce(data->>'Total', data->>'Amount', (SELECT sum(archive.sync_num(j->>'LineAmount'))::text FROM archive.sync_list(data->'JournalLines') j
				WHERE archive.sync_num(j->>'LineAmount') > 0)), ` + xeroCurrency + ")",
			"balance":   "archive.sync_money(coalesce(data->>'AmountDue', data->>'RemainingCredit'), " + xeroCurrency + ")",
			"memo":      `concat_ws(E'\n\n', nullif(btrim(data->>'Reference'), ''), nullif(btrim(data->>'Narration'), ''), nullif(btrim(data->>'Title'), ''), nullif(btrim(data->>'Summary'), ''))`,
			"status":    "data->>'Status'",
			"edited_at": `coalesce(` + xeroEditedAt + `, archive.sync_at(substring(data->>'CreatedDateUTC' from '^/Date\((\d+)')))`},
		Refs: map[string]SQL{
			"contact": "coalesce(data#>>'{Contact,ContactID}', data#>>'{Invoice,Contact,ContactID}', data#>>'{CreditNote,Contact,ContactID}')",
			"account": "coalesce(data#>>'{BankAccount,AccountID}', data#>>'{Account,AccountID}', data#>>'{FromBankAccount,AccountID}')"},
		Children: []Kids{{Table: "ledger_line", From: "archive.sync_list(coalesce(data->'LineItems', data->'JournalLines')) WITH ORDINALITY l(x, i)",
			Cols: map[string]SQL{"position": "i", "description": "x->>'Description'",
				"amount":     "archive.sync_money(x->>'LineAmount', " + xeroCurrency + ")",
				"posting":    "CASE WHEN data ? 'JournalLines' THEN CASE WHEN archive.sync_num(x->>'LineAmount') < 0 THEN 'Credit' ELSE 'Debit' END END",
				"quantity":   "archive.sync_num(x->>'Quantity')",
				"unit_price": "archive.sync_num(x->>'UnitAmount')"},
			Refs: map[string]SQL{"account": "x->>'AccountID'", "item": "x#>>'{Item,ItemID}'"}}}}
)

const (
	// xeroKind is a contact's kind.
	xeroKind SQL = "CASE WHEN data->'IsSupplier' = 'true' AND data->'IsCustomer' IS DISTINCT FROM 'true' THEN 'vendor' ELSE 'customer' END"
	// A day is the ISO string beside it, or else its own /Date(…)/ in UTC.
	xeroOn  SQL = `coalesce(archive.sync_day(data->>'DateString'), (archive.sync_at(substring(data->>'Date' from '^/Date\((\d+)')) AT TIME ZONE 'UTC')::date)`
	xeroDue SQL = `coalesce(archive.sync_day(data->>'DueDateString'), archive.sync_day(data->>'ExpiryDateString'), archive.sync_day(data->>'DeliveryDateString'),
		(archive.sync_at(substring(coalesce(data->>'DueDate', data->>'ExpiryDate', data->>'DeliveryDate') from '^/Date\((\d+)')) AT TIME ZONE 'UTC')::date)`
)

type (
	xeroFind struct {
		Type     string `json:"type" jsonschema:"Invoices, CreditNotes, Payments, BankTransactions, BankTransfers, ManualJournals, PurchaseOrders, Quotes, Contacts, Items, Accounts, Overpayments or Prepayments"`
		Where    string `json:"where,omitempty" jsonschema:"Only those it passes, like Status==\"AUTHORISED\" AND Date>=DateTime(2026,01,01)"`
		Order    string `json:"order,omitempty" jsonschema:"A field to order by, and DESC for the other way, like Date DESC"`
		Page     int    `json:"page,omitempty" jsonschema:"Which page, from 1; 1 when left out"`
		Archived bool   `json:"archived,omitempty" jsonschema:"Contacts that are archived too"`
	}
	xeroOne struct {
		Type string `json:"type" jsonschema:"Invoices, CreditNotes, Payments, BankTransactions, BankTransfers, ManualJournals, PurchaseOrders, Quotes, Contacts, Items, Accounts, Overpayments or Prepayments"`
		ID   string `json:"id" jsonschema:"Its id, or an invoice's InvoiceNumber or an item's Code"`
	}
	xeroReport struct {
		Report    string `json:"report" jsonschema:"ProfitAndLoss, BalanceSheet, TrialBalance, AgedReceivablesByContact or AgedPayablesByContact"`
		Date      string `json:"date,omitempty" jsonschema:"The day it is at, YYYY-MM-DD; today when left out"`
		FromDate  string `json:"fromDate,omitempty" jsonschema:"The first day of a profit and loss, YYYY-MM-DD"`
		ToDate    string `json:"toDate,omitempty" jsonschema:"The last day of a profit and loss, YYYY-MM-DD"`
		Periods   int    `json:"periods,omitempty" jsonschema:"How many periods before to compare, 1 to 11"`
		Timeframe string `json:"timeframe,omitempty" jsonschema:"MONTH, QUARTER or YEAR, the length of a period"`
		ContactID string `json:"contactId,omitempty" jsonschema:"The contact of an aged report, by its ContactID"`
	}
	xeroContactIn struct {
		Contact map[string]any `json:"contact" jsonschema:"The contact as Xero has it, like { Name, EmailAddress }, with its ContactID to change one"`
	}
	xeroInvoiceIn struct {
		Invoice map[string]any `json:"invoice" jsonschema:"The invoice as Xero has it, with Type, Contact and LineItems, and its InvoiceID to change one"`
	}
	xeroVoid struct {
		ID string `json:"id" jsonschema:"The invoice's InvoiceID"`
	}
)
