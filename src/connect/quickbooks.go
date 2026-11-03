package connect

import (
	"cmp"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// QuickBooks Online, through Intuit's accounting API. It reads ARCHIVE_QUICKBOOKS, an app's production keys
// from developer.intuit.com, whose redirect URIs list <API>/connect/quickbooks/callback; ?sandbox=1 on the
// variable, with the app's development keys, reads sandbox companies instead. Intuit has no read-only scope,
// so com.intuit.quickbooks.accounting is all of it, and the actions only read. The callback names the
// company, realmId, which every call needs; a person connects one company at a time, and connects again for
// another. Intuit replaces the refresh token about once a day.
//
// It syncs the books into the ledger tables: the chart of accounts, customers, vendors, employees, items, and
// every kind of transaction with its lines, a stream each. Each is listed whole the first time, 1000 a page,
// the inactive too; after that Intuit's change data capture (CDC) says what changed since the last sync began,
// deleted too, as long as that was within 30 days, and a stream whose last sync is older, or which changed
// more than CDC answers at once, is listed whole again. Its pushes say which company changed.
func init() {
	Register(&Connector{
		ID: "quickbooks", Name: "QuickBooks", Integration: true, Env: "ARCHIVE_QUICKBOOKS", Host: "https://appcenter.intuit.com",
		Endpoint: oauth2.Endpoint{AuthURL: "https://appcenter.intuit.com/connect/oauth2", TokenURL: "https://oauth.platform.intuit.com/oauth2/v1/tokens/bearer",
			AuthStyle: oauth2.AuthStyleInHeader},
		RevokeWith: func(rv *Revocation) error {
			id, secret := rv.Client()
			return rv.Send("POST", "https://developer.api.intuit.com/v2/oauth2/tokens/revoke", http.Header{"Authorization": {basic(id, secret)}}, map[string]string{"token": rv.Refresh})
		},
		Scopes: []string{"com.intuit.quickbooks.accounting"},
		Keep:   []string{"realmId"},
		Account: func(c *Client) (string, error) {
			var ci struct {
				CompanyInfo struct {
					CompanyName string
				}
			}
			err := c.Get(qbo(c, "/companyinfo/"+url.PathEscape(c.Meta["realmId"])), &ci)
			return ci.CompanyInfo.CompanyName, err
		},
		Actions: map[string]*Action{
			"query": Act("Run a QuickBooks query, its SQL-like language over one entity, like { query: \"SELECT * FROM Bill WHERE Balance > '0' MAXRESULTS 100\" }: "+
				"{ QueryResponse: { <Entity>: [...], startPosition, maxResults } }. Entities include Account, Bill, Customer, Estimate, Invoice, Item, "+
				"Payment, Purchase, Vendor; no joins, and at most 1000 rows, paged with STARTPOSITION",
				func(c *Client, in struct {
					Query string `json:"query" jsonschema:"The query, like SELECT * FROM Invoice WHERE TxnDate >= '2026-01-01' ORDERBY TxnDate DESC"`
				}) (any, error) {
					var out any
					err := c.Get(qbo(c, "/query?query="+url.QueryEscape(in.Query)), &out)
					return out, err
				}),
			"company": Act("The company connected: its CompanyInfo, with CompanyName, LegalName, CompanyAddr, Country, FiscalYearStartMonth and more",
				func(c *Client, in struct{}) (any, error) {
					var out struct {
						CompanyInfo any
					}
					err := c.Get(qbo(c, "/companyinfo/"+url.PathEscape(c.Meta["realmId"])), &out)
					return out.CompanyInfo, err
				}),
			"invoices": Act("Invoices, newest first, maybe only a customer's or only unpaid ones: [Invoice] with DocNumber, TxnDate, DueDate, CustomerRef, TotalAmt, Balance, Line",
				func(c *Client, in struct {
					Customer string `json:"customer,omitempty" jsonschema:"Only this customer's, by its Id"`
					Unpaid   bool   `json:"unpaid,omitempty" jsonschema:"Only those with a balance left"`
					Since    string `json:"since,omitempty" jsonschema:"Only those dated on or after this day, YYYY-MM-DD"`
					Limit    int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 1000; 100 when left out"`
				}) (any, error) {
					var where []string
					if in.Customer != "" {
						where = append(where, "CustomerRef = "+qboQuote(in.Customer))
					}
					if in.Unpaid {
						where = append(where, "Balance > '0'")
					}
					if in.Since != "" {
						where = append(where, "TxnDate >= "+qboQuote(in.Since))
					}
					return qboList(c, "Invoice", where, "TxnDate DESC", in.Limit)
				}),
			"customers": Act("Customers by name, maybe only those whose name starts with some text: [Customer] with Id, DisplayName, CompanyName, PrimaryEmailAddr, Balance, Active",
				func(c *Client, in struct {
					Name  string `json:"name,omitempty" jsonschema:"Only those whose DisplayName starts with this"`
					Limit int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 1000; 100 when left out"`
				}) (any, error) {
					var where []string
					if in.Name != "" {
						where = append(where, "DisplayName LIKE "+qboQuote(in.Name+"%"))
					}
					return qboList(c, "Customer", where, "DisplayName", in.Limit)
				}),
		},
		Push: quickbooksPush,
		Streams: []*Stream{
			qboStream("accounts", "Account", "The chart of accounts", "ledger_account", qboAccountRows),
			qboStream("customers", "Customer", "Every customer", "ledger_contact", qboContactRows),
			qboStream("vendors", "Vendor", "Every vendor", "ledger_contact", qboContactRows),
			qboStream("employees", "Employee", "Every employee", "ledger_contact", qboContactRows),
			qboStream("items", "Item", "Every product and service", "ledger_item", qboItemRows),
			qboStream("invoices", "Invoice", "Every invoice", "ledger_entry", qboEntryRows),
			qboStream("payments", "Payment", "Every payment a customer made", "ledger_entry", qboEntryRows),
			qboStream("sales_receipts", "SalesReceipt", "Every sales receipt", "ledger_entry", qboEntryRows),
			qboStream("credit_memos", "CreditMemo", "Every credit memo", "ledger_entry", qboEntryRows),
			qboStream("refund_receipts", "RefundReceipt", "Every refund receipt", "ledger_entry", qboEntryRows),
			qboStream("estimates", "Estimate", "Every estimate", "ledger_entry", qboEntryRows),
			qboStream("bills", "Bill", "Every bill", "ledger_entry", qboEntryRows),
			qboStream("bill_payments", "BillPayment", "Every bill payment", "ledger_entry", qboEntryRows),
			qboStream("vendor_credits", "VendorCredit", "Every vendor credit", "ledger_entry", qboEntryRows),
			qboStream("purchases", "Purchase", "Every expense, check and credit card charge", "ledger_entry", qboEntryRows),
			qboStream("purchase_orders", "PurchaseOrder", "Every purchase order", "ledger_entry", qboEntryRows),
			qboStream("deposits", "Deposit", "Every bank deposit", "ledger_entry", qboEntryRows),
			qboStream("transfers", "Transfer", "Every transfer between accounts", "ledger_entry", qboEntryRows),
			qboStream("journal_entries", "JournalEntry", "Every journal entry", "ledger_entry", qboEntryRows),
		},
	})
}

// qboPage is the most a query answers, and CDC too.
const qboPage = 1000

// qboOverlap is how many records of a page a listing's next page reads again.
const qboOverlap = 50

// qboCDC is how far back CDC reaches: 30 days, less a day to spare.
const qboCDC = 29 * 24 * time.Hour

// qboNamed are the entities a query lists only the active of unless asked for the inactive too.
var qboNamed = map[string]bool{"Account": true, "Customer": true, "Vendor": true, "Employee": true, "Item": true}

// qboKeyed are the entities whose ids are only theirs, kept with the entity's name before them, like
// Customer:12, as a customer and a vendor may have one id and share a table.
var qboKeyed = map[string]bool{"Customer": true, "Vendor": true, "Employee": true}

// qboCursor is where a stream is: Since when the sync its records are as of began, which the next asks CDC
// from, and while a listing is under way, At its next start position and Began when it began.
type qboCursor struct {
	Since string `json:"since,omitempty"`
	At    int    `json:"at,omitempty"`
	Began string `json:"began,omitempty"`
}

// qboStream is a stream of entity into table, listed whole and then by CDC.
func qboStream(name, entity, what, table string, rows func(map[string]any) *Row) *Stream {
	return &Stream{Name: name, Gap: time.Second, Table: table, Rows: rows,
		Doc: what + ", as a query of " + entity + " answers, the inactive too: listed whole the first time, 1000 a page, and then what " +
			"CDC says changed since the sync before, deleted too, or listed whole again when that was over 29 days before.",
		Pull: func(c *Client, cursor string) (*Page, error) {
			var cur qboCursor
			if err := cursorOf(cursor, &cur); err != nil {
				return nil, err
			}
			now := time.Now().Add(-time.Minute).UTC()
			if since := stamp(cur.Since); cur.Began == "" && !since.IsZero() && now.Sub(since) < qboCDC {
				var v any
				q := url.Values{"entities": {entity}, "changedSince": {since.Format(time.RFC3339)}}
				if err := qboGet(c, entity, "/cdc?"+q.Encode(), &v); err != nil {
					return nil, err
				}
				items := objects(v, entity)
				// CDC answers at most 1000: more changed than that is listed whole instead.
				if len(items) < qboPage {
					page := &Page{Records: qboRecords(entity, items)}
					page.Cursor = cursorFor(qboCursor{Since: now.Format(time.RFC3339)})
					return page, nil
				}
			}
			if cur.Began == "" {
				cur = qboCursor{At: 1, Began: now.Format(time.RFC3339)}
			}
			q := "SELECT * FROM " + entity
			if qboNamed[entity] {
				q += " WHERE Active IN (true, false)"
			}
			// Pages are in the order of ids, so a record made while the listing runs comes last; a company that
			// does not sort an entity by id lists it in its own order.
			page := fmt.Sprintf(" STARTPOSITION %d MAXRESULTS %d", cur.At, qboPage)
			var v any
			err := qboGet(c, entity, "/query?query="+url.QueryEscape(q+" ORDERBY Id"+page), &v)
			if u := (*Unsupported)(nil); errors.As(err, &u) {
				// Not v, err = nil, qboGet(…): that reads into v and only then empties it, and an empty page of a
				// listing deletes every row.
				v = nil
				err = qboGet(c, entity, "/query?query="+url.QueryEscape(q+page), &v)
			}
			if err != nil {
				return nil, err
			}
			items := objects(v, entity)
			out := &Page{Full: true, Records: qboRecords(entity, items), More: len(items) >= qboPage}
			if out.More {
				// Pages overlap by qboOverlap, so a record deleted while the listing runs, which moves the ones after
				// it a place back, skips none of them; those read twice are written once.
				cur.At += len(items) - qboOverlap
			} else {
				cur = qboCursor{Since: cur.Began}
			}
			out.Cursor = cursorFor(cur)
			return out, nil
		}}
}

// qboGet reads path of the company into v. A 400 is an entity the company's plan does not have, like purchase
// orders without QuickBooks Plus, which skips its stream.
func qboGet(c *Client, entity, path string, v any) error {
	err := c.Get(qbo(c, path), v)
	if r := (*Refusal)(nil); errors.As(err, &r) && r.Status == http.StatusBadRequest {
		return unsupported("QuickBooks does not list %s for this company: %s", entity, r.Message)
	}
	return err
}

// qboRecords are entities as records, each with its entity's name as its kind; one CDC says is deleted is gone.
func qboRecords(entity string, items []map[string]any) []Record {
	out := make([]Record, 0, len(items))
	for _, it := range items {
		id := str(it, "Id")
		if id == "" {
			continue
		}
		if qboKeyed[entity] {
			id = entity + ":" + id
		}
		if strings.EqualFold(str(it, "status"), "Deleted") {
			out = append(out, Record{ID: id, Deleted: true})
			continue
		}
		it["kind"] = entity
		out = append(out, Record{ID: id, Data: it, Updated: stamp(str(obj(it, "MetaData"), "LastUpdatedTime"))})
	}
	return out
}

// qboRef is the id a reference like CustomerRef names, "" when it names none.
func qboRef(m map[string]any, k string) string { return str(obj(m, k), "value") }

// qboCurrency is the currency of an entity, US dollars when it names none, as a company with one currency does
// not.
func qboCurrency(m map[string]any) string {
	return cmp.Or(qboRef(m, "CurrencyRef"), "USD")
}

func qboEdited(m map[string]any) any { return instant(str(obj(m, "MetaData"), "LastUpdatedTime")) }

func qboAccountRows(m map[string]any) *Row {
	cur := qboCurrency(m)
	return rowOf(map[string]any{"number": str(m, "AcctNum"), "title": str(m, "Name"), "full_name": str(m, "FullyQualifiedName"),
		"type": str(m, "AccountType"), "subtype": str(m, "AccountSubType"), "class": str(m, "Classification"),
		"balance": money(m["CurrentBalanceWithSubAccounts"], cur), "active": flag(m, "Active"), "edited_at": qboEdited(m)}).
		ref("parent", qboRef(m, "ParentRef"))
}

// qboContactRows is a customer, vendor or employee as a contact, who is the person of its email, with its
// billing address, or an employee's own, on one line.
func qboContactRows(m map[string]any) *Row {
	addr := obj(m, "BillAddr")
	if len(addr) == 0 {
		addr = obj(m, "PrimaryAddr")
	}
	var parts []string
	for _, k := range []string{"Line1", "Line2", "Line3", "City", "CountrySubDivisionCode", "PostalCode", "Country"} {
		if v := strings.TrimSpace(str(addr, k)); v != "" {
			parts = append(parts, v)
		}
	}
	email := Email(str(obj(m, "PrimaryEmailAddr"), "Address"))
	return rowOf(map[string]any{"kind": strings.ToLower(str(m, "kind")), "title": str(m, "DisplayName"), "company": str(m, "CompanyName"),
		"email": email, "phone": str(obj(m, "PrimaryPhone"), "FreeFormNumber"), "address": strings.Join(parts, ", "),
		"balance": money(m["Balance"], qboCurrency(m)), "active": flag(m, "Active"), "edited_at": qboEdited(m)}).person("person", email)
}

func qboItemRows(m map[string]any) *Row {
	cur := qboCurrency(m)
	return rowOf(map[string]any{"title": str(m, "Name"), "sku": str(m, "Sku"), "type": str(m, "Type"), "description": str(m, "Description"),
		"price": money(m["UnitPrice"], cur), "cost": money(m["PurchaseCost"], cur), "quantity": decimal(m["QtyOnHand"]), "active": flag(m, "Active"),
		"edited_at": qboEdited(m)}).ref("income_account", qboRef(m, "IncomeAccountRef")).ref("expense_account", qboRef(m, "ExpenseAccountRef"))
}

// qboEntryRows is a transaction as an entry of the books, with its lines but the subtotals, which add up others.
// Who it is with is its customer, its vendor, or the entity it names, which says which; the account it pays
// from or into is the first it names of those that say so.
func qboEntryRows(m map[string]any) *Row {
	cur := qboCurrency(m)
	contact, name := "", ""
	switch {
	case qboRef(m, "CustomerRef") != "":
		contact, name = "Customer:"+qboRef(m, "CustomerRef"), str(obj(m, "CustomerRef"), "name")
	case qboRef(m, "VendorRef") != "":
		contact, name = "Vendor:"+qboRef(m, "VendorRef"), str(obj(m, "VendorRef"), "name")
	case qboRef(m, "EntityRef") != "":
		e := obj(m, "EntityRef")
		contact, name = cmp.Or(str(e, "type"), "Customer")+":"+str(e, "value"), str(e, "name")
	}
	account := cmp.Or(qboRef(m, "DepositToAccountRef"), qboRef(m, "AccountRef"), qboRef(obj(m, "CheckPayment"), "BankAccountRef"),
		qboRef(obj(m, "CreditCardPayment"), "CCAccountRef"), qboRef(m, "FromAccountRef"), qboRef(m, "APAccountRef"), qboRef(m, "ARAccountRef"))
	var memo []string
	for _, s := range []string{str(m, "PrivateNote"), str(obj(m, "CustomerMemo"), "value"), str(m, "Memo")} {
		if s = strings.TrimSpace(s); s != "" {
			memo = append(memo, s)
		}
	}
	total := m["TotalAmt"]
	if total == nil {
		total = m["Amount"]
	}
	r := rowOf(map[string]any{"kind": str(m, "kind"), "number": str(m, "DocNumber"), "txn_on": day(str(m, "TxnDate")), "due_on": day(str(m, "DueDate")),
		"contact_name": name, "total": money(total, cur), "balance": money(m["Balance"], cur), "memo": strings.Join(memo, "\n\n"),
		"status": cmp.Or(str(m, "TxnStatus"), str(m, "POStatus")), "edited_at": qboEdited(m)}).ref("contact", contact).ref("account", account)
	for i, l := range objects(m["Line"], "") {
		kind := str(l, "DetailType")
		if kind == "SubTotalLineDetail" {
			continue
		}
		d := obj(l, kind)
		pos := decimal(l["LineNum"])
		if pos == nil {
			pos = i + 1
		}
		r.child("ledger_line", rowOf(map[string]any{"position": pos, "kind": kind, "description": str(l, "Description"), "amount": money(l["Amount"], cur),
			"posting": str(d, "PostingType"), "quantity": decimal(d["Qty"]), "unit_price": decimal(d["UnitPrice"])}).
			ref("account", qboRef(d, "AccountRef")).ref("item", qboRef(d, "ItemRef")))
	}
	return r
}

// qbo is the URL of path in the company the client is for, in the minor version of the API the actions read.
func qbo(c *Client, path string) string {
	host := "https://quickbooks.api.intuit.com"
	if c.Option("sandbox") != "" {
		host = "https://sandbox-quickbooks.api.intuit.com"
	}
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	return host + "/v3/company/" + url.PathEscape(c.Meta["realmId"]) + path + sep + "minorversion=75"
}

func qboQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `\'`) + "'" }

func qboList(c *Client, entity string, where []string, order string, limit int) (any, error) {
	q := "SELECT * FROM " + entity
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += fmt.Sprintf(" ORDERBY %s MAXRESULTS %d", order, clamp(limit, 100, 1000))
	var out struct {
		QueryResponse map[string]any
	}
	if err := c.Get(qbo(c, "/query?query="+url.QueryEscape(q)), &out); err != nil {
		return nil, err
	}
	if rows, ok := out.QueryResponse[entity]; ok {
		return rows, nil
	}
	return []any{}, nil
}

// quickbooksPush is the Intuit app's webhooks, set up once for each of its environments: its endpoint
// <API>/push/quickbooks, the entities QuickBooks streams, and its verifier token the hook option of
// ARCHIVE_QUICKBOOKS. intuit-signature is the base64 HMAC-SHA256 of the body with the verifier token. A push is
// an array of CloudEvents, each with intuitaccountid, the realmId of the company it is for; one of the format
// before, { eventNotifications: [{ realmId }] }, is read too.
var quickbooksPush = &Push{
	Watch: func(c *Client, h Hook, old []Watch) ([]Watch, error) {
		return []Watch{{Key: c.Meta["realmId"]}}, nil
	},
	Hear: func(p *Provider, r *http.Request, at string, body []byte, secret func(string) string) (*Heard, error) {
		if p.HookSecret() == "" || !same(r.Header.Get("intuit-signature"), b64MAC(p.HookSecret(), string(body))) {
			return nil, forged("intuit-signature")
		}
		var keys []string
		if t := strings.TrimSpace(string(body)); strings.HasPrefix(t, "[") {
			var events []struct {
				Account string `json:"intuitaccountid"`
			}
			if err := readJSON(body, &events); err != nil {
				return nil, err
			}
			for _, e := range events {
				keys = append(keys, e.Account)
			}
		} else {
			var n struct {
				Notifications []struct {
					RealmID string `json:"realmId"`
				} `json:"eventNotifications"`
			}
			if err := readJSON(body, &n); err != nil {
				return nil, err
			}
			for _, e := range n.Notifications {
				keys = append(keys, e.RealmID)
			}
		}
		return &Heard{Keys: keysOf(keys...)}, nil
	}}
