package connect

import (
	"time"

	"golang.org/x/oauth2"
)

// Square, a seller's account through Square's API: its payments, orders, customers, items, invoices and disputes,
// read, and when asked a customer added and an invoice drafted and sent. Its application, in Square's Developer
// Console, lists <API>/connect/square/callback as its production redirect URL; ARCHIVE_SQUARE is its production
// application id and secret, or, on connect.squareupsandbox.com, its sandbox ones. Square asks the seller for exactly
// the Scopes here, and gives a refresh token that each refresh replaces, good for 90 days after the last, as the
// sign-in has PKCE; an access token lasts 30 days. Nothing here moves money: an invoice Square would charge a card or
// bank account on file for is not sent.
//
// It syncs each location's payments into commerce_payment and orders, with their lines, into commerce_order and
// commerce_line, and the seller's customers into crm_contact, the person of their email, and items into
// commerce_product: each listed whole the first time and a week after, which deletes what is gone, and in between
// searched for what changed since the sync before, items deleted too. Each location's invoices, into
// commerce_invoice, and the disputes, into commerce_dispute, are listed whole each sync. Square leaves a list out of
// an answer that has nothing in it, so any answer it gives is one.
//
// Its pushes are the application's webhook subscription, set up once in the Developer Console to <API>/push/square,
// for the events of payments, orders, customers, catalog, invoices and disputes, its signature key the hook option of
// ARCHIVE_SQUARE. x-square-hmacsha256-signature is the base64 HMAC-SHA256 of the URL and the body with that key, and an
// event's merchant_id is the seller it is for.
func init() {
	Register(&Connector{
		ID: "square", Name: "Square", Integration: true, Options: "?hook=<webhook signature key> for its pushes",
		Info: Info{By: "Square", Description: "Read your payments, orders, customers, items, invoices and disputes, and draft and send invoices when you ask", Category: CatCommerce, Icon: IconCard,
			Scopes: []string{"Read payments, orders, customers, items, invoices and disputes", "Add customers, and draft and send invoices, when you ask"}},
		API: "https://connect.squareup.com",
		Endpoint: oauth2.Endpoint{AuthURL: "https://connect.squareup.com/oauth2/authorize", TokenURL: "https://connect.squareup.com/oauth2/token",
			AuthStyle: oauth2.AuthStyleInParams},
		RevokeBy: &Req{Method: POST, URL: "https://connect.squareup.com/oauth2/revoke", Header: map[string][]string{"Authorization": {"Client {client/secret}"}},
			Body: Obj{"client_id": "{client/id}", "access_token": "{access}"}},
		Scopes: []string{"MERCHANT_PROFILE_READ", "PAYMENTS_READ", "ORDERS_READ", "CUSTOMERS_READ", "ITEMS_READ", "INVOICES_READ", "DISPUTES_READ",
			"CUSTOMERS_WRITE", "ORDERS_WRITE", "INVOICES_WRITE"},
		Params: map[string]string{"session": "false"},
		Who:    &Who{Req: squareMe, Names: []Path{"merchant/business_name", "merchant/id"}},
		Actions: map[string]*Action{
			"get": Call[squareGet, any](Plan{
				Doc: "Read any of Square's GET endpoints, like { path: \"/v2/orders/<id>\" }, { path: \"/v2/locations\" } or " +
					"{ path: \"/v2/catalog/list?types=ITEM\" }: Square's answer as it is. Reads only",
				Steps: []Step{{Req: Req{URL: "{+path}", Header: squareV}}},
			}),
			"payments": Call[squarePayments, squarePaid](Plan{
				Doc:   "Payments of a location, the main one when left out, newest first, maybe only between two times: { payments: [Payment], next } (pass next as page for more)",
				Clamp: map[string][2]int{"limit": {50, 100}},
				Steps: []Step{{As: "got", Req: Req{URL: "/v2/payments", Header: squareV, Query: Q{"location_id": "{location}", "begin_time": "{from|" + squareEver + "}",
					"end_time": "{to}", "sort_order": "DESC", "limit": "{limit}", "cursor": "{page}"}}}},
				Out: Obj{"payments": "{got/payments|[]}", "next": "{got/cursor|}"},
			}),
			"orders": Call[squareOrders, squareOrdered](Plan{
				Doc: "Orders of every location, or only one, newest first, maybe only in a state or a customer's: " +
					"{ orders: [Order], next } (pass next as page for more)",
				Clamp: map[string][2]int{"limit": {50, 500}},
				Steps: []Step{
					{Unless: "location", As: "all", Req: Req{URL: "/v2/locations", Header: squareV}},
					{Unless: "location", As: "locations", In: "{all/locations/*/id}"},
					{If: "location", As: "locations", In: []any{"{location}"}},
					{If: "state", As: "filter/state_filter/states", In: []any{"{state}"}},
					{If: "customer", As: "filter/customer_filter/customer_ids", In: []any{"{customer}"}},
					{As: "got", Req: Req{Method: POST, Read: true, URL: "/v2/orders/search", Header: squareV, Body: Obj{"location_ids": "{locations}", "limit": "{limit}", "cursor": "{page}",
						"query": Obj{"filter": "{filter}", "sort": Obj{"sort_field": "CREATED_AT", "sort_order": "DESC"}}}}},
				},
				Out: Obj{"orders": "{got/orders|[]}", "next": "{got/cursor|}"},
			}),
			"customers": Call[squareFind, squareFound](Plan{
				Doc:   "Customers, maybe only those whose email or phone has some text: { customers: [Customer], next } (pass next as page for more)",
				Clamp: map[string][2]int{"limit": {50, 100}},
				Steps: []Step{
					{If: "email", As: "filter/email_address/fuzzy", In: "{email}"},
					{If: "phone", As: "filter/phone_number/fuzzy", In: "{phone}"},
					{As: "got", Req: Req{Method: POST, Read: true, URL: "/v2/customers/search", Header: squareV, Body: Obj{"limit": "{limit}", "cursor": "{page}",
						"query": Obj{"filter": "{filter}", "sort": Obj{"field": "CREATED_AT", "order": "DESC"}}}}},
				},
				Out: Obj{"customers": "{got/customers|[]}", "next": "{got/cursor|}"},
			}),
			"invoices": Call[squareInvoices, squareBilled](Plan{
				Doc:   "Invoices of a location, the main one when left out, maybe only a customer's: { invoices: [Invoice], next } (pass next as page for more)",
				Clamp: map[string][2]int{"limit": {50, 200}},
				Steps: []Step{
					{Unless: "location", As: "me", Req: squareMe},
					{Unless: "location", As: "location", In: "{me/merchant/main_location_id}"},
					{If: "customer", As: "customers", In: []any{"{customer}"}},
					{As: "got", Req: Req{Method: POST, Read: true, URL: "/v2/invoices/search", Header: squareV, Body: Obj{"limit": "{limit}", "cursor": "{page}",
						"query": Obj{"filter": Obj{"location_ids": []any{"{location}"}, "customer_ids": "{customers}"}, "sort": Obj{"field": "INVOICE_SORT_DATE", "order": "DESC"}}}}},
				},
				Out: Obj{"invoices": "{got/invoices|[]}", "next": "{got/cursor|}"},
			}),
			"customer": Call[squareNew, map[string]any](Plan{
				Doc: "Add a customer, with at least one of a name, a company, an email address or a phone number: the Customer",
				Steps: []Step{{As: "got", Req: Req{Method: POST, URL: "/v2/customers", Header: squareV, Body: Obj{"idempotency_key": "{nonce}", "given_name": "{given_name}",
					"family_name": "{family_name}", "company_name": "{company_name}", "email_address": "{email_address}", "phone_number": "{phone_number}", "note": "{note}"}}}},
				Out: "{got/customer}",
			}),
			"draft": Call[squareDraft, map[string]any](Plan{
				Doc: "Draft an invoice to a customer for some lines, due on a day, with an order of them, at a location, the main one when left out: " +
					"the Invoice, a DRAFT that Square sends nobody until send. Amounts are in the smallest unit of the location's currency, like cents",
				Steps: []Step{
					{Unless: "lines", Fail: squareLines},
					{Unless: "location", As: "me", Req: squareMe},
					{Unless: "location", As: "location", In: "{me/merchant/main_location_id}"},
					{As: "at", Req: Req{URL: "/v2/locations/{location}", Header: squareV}},
					{Each: "lines", As: "items", In: Obj{"name": "{name}", "quantity": "{quantity|1}", "note": "{note}",
						"base_price_money": Obj{"amount": "{amount}", "currency": "{at/location/currency}"}}},
					{As: "order", Req: Req{Method: POST, URL: "/v2/orders", Header: squareV, Body: Obj{"idempotency_key": "{nonce}-order",
						"order": Obj{"location_id": "{location}", "customer_id": "{customer}", "line_items": "{items}"}}}},
					{As: "got", Req: Req{Method: POST, URL: "/v2/invoices", Header: squareV, Body: Obj{"idempotency_key": "{nonce}-invoice",
						"invoice": Obj{"location_id": "{location}", "order_id": "{order/order/id}", "primary_recipient": Obj{"customer_id": "{customer}"},
							"payment_requests": []Obj{{"request_type": "BALANCE", "due_date": "{due}"}}, "delivery_method": "EMAIL",
							"accepted_payment_methods": Obj{"card": true, "bank_account": true}, "title": "{title}", "description": "{description}"}}}},
				},
				Out: "{got/invoice}",
			}),
			"send": Call[squareSend, map[string]any](Plan{
				Doc: "Send a draft invoice, by its id, to its customer by email, who pays it there: the Invoice. One Square would charge a card or bank account " +
					"on file for is refused",
				Steps: []Step{
					{As: "got", Req: Req{URL: "/v2/invoices/{id}", Header: squareV}},
					{Only: Test{At: "got/invoice/payment_requests/*/automatic_payment_source", Is: "CARD_ON_FILE"}, Fail: squareCharges},
					{Only: Test{At: "got/invoice/payment_requests/*/automatic_payment_source", Is: "BANK_ON_FILE"}, Fail: squareCharges},
					{As: "sent", Req: Req{Method: POST, URL: "/v2/invoices/{id}/publish", Header: squareV, Body: Obj{"version": "{got/invoice/version}", "idempotency_key": "{nonce}"}}},
				},
				Out: "{sent/invoice}",
			}),
		},
		Listen: &Listen{Watch: squareMe, Key: "{answer/merchant/id}", Keys: []Path{"merchant_id"},
			Signs: []Sign{{Header: "x-square-hmacsha256-signature", Hook: true, Of: "{url}{body}"}}},
		Sync: []*Sync{
			{Name: "items", Gap: time.Second, Items: "objects", Absent: "*", Updated: "updated_at", Page: squarePage, Gone: []Test{{At: "is_deleted", Is: "true"}},
				Req: Req{Method: POST, Read: true, URL: "/v2/catalog/search", Header: squareV, Body: Obj{"object_types": []string{"ITEM"}, "limit": 1000, "cursor": "{page}"}},
				Change: SinceTime{Req: Req{Method: POST, Read: true, URL: "/v2/catalog/search", Header: squareV,
					Body: Obj{"object_types": []string{"ITEM"}, "include_deleted_objects": true, "begin_time": "{since}", "limit": 1000, "cursor": "{page}"}}},
				Doc: "Every item of the catalog, with its type, its lowest price and its description, cut at 64 KB, by its id: listed whole the first time and a " +
					"week after, which deletes those gone, and in between those changed since the sync before, deleted too.",
				To: Map{Table: "commerce_product",
					Cols: map[string]SQL{"title": "data#>>'{item_data,name}'", "type": "data#>>'{item_data,product_type}'",
						"status":      "CASE WHEN data#>>'{item_data,is_archived}' = 'true' THEN 'ARCHIVED' ELSE 'ACTIVE' END",
						"price":       squareLowest,
						"description": "archive.sync_clip(coalesce(data#>>'{item_data,description_plaintext}', data#>>'{item_data,description}'), 65536)",
						"url":         "data#>>'{item_data,ecom_uri}'", "edited_at": "archive.sync_at(data->>'updated_at')"}}},
			{Name: "customers", Gap: time.Second, Items: "customers", Absent: "*", Updated: "updated_at", Page: squarePage,
				Req: Req{Method: POST, Read: true, URL: "/v2/customers/search", Header: squareV, Body: Obj{"limit": 100, "cursor": "{page}",
					"query": Obj{"sort": Obj{"field": "CREATED_AT", "order": "ASC"}}}},
				Change: SinceTime{Req: Req{Method: POST, Read: true, URL: "/v2/customers/search", Header: squareV, Body: Obj{"limit": 100, "cursor": "{page}",
					"query": Obj{"filter": Obj{"updated_at": Obj{"start_at": "{since}"}}, "sort": Obj{"field": "CREATED_AT", "order": "ASC"}}}}},
				Doc: "Every customer, with their name, company, email, phone and city, by its id: listed whole the first time and a week after, which deletes " +
					"those gone, and in between those changed since the sync before.",
				To: Map{Table: "crm_contact",
					Cols: map[string]SQL{"first_name": "data->>'given_name'", "last_name": "data->>'family_name'", "email": "archive.sync_email(data->>'email_address')",
						"phone": "data->>'phone_number'", "company_name": "data->>'company_name'", "stage": "'customer'", "city": "data#>>'{address,locality}'",
						"state": "data#>>'{address,administrative_district_level_1}'", "country": "data#>>'{address,country}'",
						"made_at": "archive.sync_at(data->>'created_at')", "edited_at": "archive.sync_at(data->>'updated_at')"},
					People: map[string]SQL{"person": "data->>'email_address'"}}},
			{Name: "orders", Gap: time.Second, Parts: squareLocations, Items: "orders", Absent: "*", Updated: "updated_at", Page: squarePage,
				Req: Req{Method: POST, Read: true, URL: "/v2/orders/search", Header: squareV, Body: Obj{"location_ids": []any{"{part}"}, "limit": 500, "cursor": "{page}",
					"query": Obj{"sort": Obj{"sort_field": "CREATED_AT", "sort_order": "ASC"}}}},
				Change: SinceTime{Req: Req{Method: POST, Read: true, URL: "/v2/orders/search", Header: squareV, Body: Obj{"location_ids": []any{"{part}"}, "limit": 500, "cursor": "{page}",
					"query": Obj{"filter": Obj{"date_time_filter": Obj{"updated_at": Obj{"start_at": "{since}"}}}, "sort": Obj{"sort_field": "UPDATED_AT", "sort_order": "ASC"}}}}},
				Doc: "Every order of each location, with its totals, state, who it is for and where it ships, and its lines, by its id: listed whole the first " +
					"time and a week after, which deletes those gone, and in between those changed since the sync before. A line names the item variation it " +
					"is, not the item, so it links to no product.",
				To: Map{Table: "commerce_order",
					Cols: map[string]SQL{"number": "coalesce(nullif(data->>'reference_id', ''), data->>'ticket_name')",
						"customer_name": squareRecipient + "->>'display_name'", "email": "archive.sync_email(" + squareRecipient + "->>'email_address')",
						"total": squareMoney + "data->'total_money'" + squareMoneyOf, "tax": squareMoney + "data->'total_tax_money'" + squareMoneyOf,
						"discounts": squareMoney + "data->'total_discount_money'" + squareMoneyOf, "refunded": squareMoney + "data#>'{return_amounts,total_money}'" + squareMoneyOf,
						"paid":         "CASE WHEN jsonb_typeof(data->'tenders') = 'array' THEN 'PAID' ELSE 'UNPAID' END",
						"fulfilled":    "coalesce(data#>>'{fulfillments,0,state}', data->>'state')",
						"cancelled_at": "CASE WHEN data->>'state' = 'CANCELED' THEN archive.sync_at(data->>'updated_at') END",
						"city":         squareRecipient + "#>>'{address,locality}'", "state": squareRecipient + "#>>'{address,administrative_district_level_1}'",
						"country": squareRecipient + "#>>'{address,country}'", "placed_at": "archive.sync_at(data->>'created_at')", "edited_at": "archive.sync_at(data->>'updated_at')"},
					People: map[string]SQL{"customer": squareRecipient + "->>'email_address'"},
					Children: []Kids{{Table: "commerce_line", From: "archive.sync_list(data->'line_items') l",
						Cols: map[string]SQL{"title": "l->>'name'", "variant": "l->>'variation_name'", "quantity": "archive.sync_num(l->>'quantity')",
							"price": squareMoney + "l->'base_price_money'" + squareMoneyOf, "total": squareMoney + "l->'total_money'" + squareMoneyOf},
						Refs: map[string]SQL{"product": "l->>'catalog_object_id'"}}}}},
			{Name: "payments", Gap: time.Second, Parts: squareLocations, Items: "payments", Absent: "*", Updated: "updated_at", Page: squarePage,
				Req: Req{URL: "/v2/payments", Header: squareV, Query: Q{"location_id": "{part}", "begin_time": squareEver, "sort_order": "ASC", "limit": "100", "cursor": "{page}"}},
				Change: SinceTime{Req: Req{URL: "/v2/payments", Header: squareV, Query: Q{"location_id": "{part}", "begin_time": squareEver, "updated_at_begin_time": "{since}",
					"sort_field": "UPDATED_AT", "sort_order": "ASC", "limit": "100", "cursor": "{page}"}}},
				Doc: "Every payment each location took, with its amount, fee, tip, what of it was refunded, how it was paid and the order it paid for, by its id: " +
					"listed whole the first time and a week after, which deletes those gone, and in between those changed since the sync before.",
				To: Map{Table: "commerce_payment",
					Cols: map[string]SQL{"kind": "'payment'", "status": "data->>'status'", "amount": squareMoney + "data->'total_money'" + squareMoneyOf,
						"fee": `(SELECT jsonb_build_object('amount', sum(archive.sync_num(f#>>'{amount_money,amount}')), 'currency', min(f#>>'{amount_money,currency}'))
							FROM archive.sync_list(data->'processing_fee') f HAVING count(*) > 0)`,
						"tip": squareMoney + "data->'tip_money'" + squareMoneyOf, "refunded": squareMoney + "data->'refunded_money'" + squareMoneyOf,
						"method":        "nullif(concat_ws(' ', data->>'source_type', data#>>'{card_details,card,card_brand}', data#>>'{card_details,card,last_4}'), '')",
						"customer_name": "data#>>'{card_details,card,cardholder_name}'", "email": "archive.sync_email(data->>'buyer_email_address')",
						"reference": "data->>'reference_id'", "note": "data->>'note'", "url": "data->>'receipt_url'",
						"paid_at": "archive.sync_at(data->>'created_at')", "edited_at": "archive.sync_at(data->>'updated_at')"},
					People: map[string]SQL{"customer": "data->>'buyer_email_address'"},
					Refs:   map[string]SQL{"purchase": "data->>'order_id'"}}},
			{Name: "invoices", Gap: time.Second, Parts: squareLocations, Items: "invoices", Absent: "*", Updated: "updated_at", Page: squarePage,
				Req: Req{URL: "/v2/invoices", Header: squareV, Query: Q{"location_id": "{part}", "limit": "200", "cursor": "{page}"}},
				Doc: "Every invoice of each location, with who it is to, what it asks, what was paid and when it is due, by its id: listed whole each sync, " +
					"which deletes those gone.",
				To: Map{Table: "commerce_invoice",
					Cols: map[string]SQL{"number": "data->>'invoice_number'", "title": "data->>'title'", "status": "data->>'status'",
						"customer_name": "coalesce(nullif(btrim(concat_ws(' ', data#>>'{primary_recipient,given_name}', data#>>'{primary_recipient,family_name}')), ''), data#>>'{primary_recipient,company_name}')",
						"email":         "archive.sync_email(data#>>'{primary_recipient,email_address}')",
						"total":         squareAsked + "sum(archive.sync_num(p#>>'{computed_amount_money,amount}'))" + squareAskedOf,
						"paid":          squareAsked + "sum(coalesce(archive.sync_num(p#>>'{total_completed_amount_money,amount}'), 0))" + squareAskedOf,
						"due":           squareAsked + "sum(archive.sync_num(p#>>'{computed_amount_money,amount}')) - sum(coalesce(archive.sync_num(p#>>'{total_completed_amount_money,amount}'), 0))" + squareAskedOf,
						"issued_on":     "archive.sync_day(coalesce(data->>'sale_or_service_date', data->>'scheduled_at', data->>'created_at'))",
						"due_on":        "(SELECT min(archive.sync_day(p->>'due_date')) FROM archive.sync_list(data->'payment_requests') p)",
						"note":          "data->>'description'", "url": "data->>'public_url'",
						"made_at": "archive.sync_at(data->>'created_at')", "edited_at": "archive.sync_at(data->>'updated_at')"},
					People: map[string]SQL{"customer": "data#>>'{primary_recipient,email_address}'"},
					Refs:   map[string]SQL{"purchase": "data->>'order_id'"}}},
			{Name: "disputes", Gap: time.Second, Items: "disputes", Absent: "*", Updated: "updated_at", Page: squarePage,
				Req: Req{URL: "/v2/disputes", Header: squareV, Query: Q{"cursor": "{page}"}},
				Doc: "Every dispute a buyer opened over a payment, with its reason, state, amount and when to answer by, by its id: listed whole each sync, " +
					"which deletes those gone.",
				To: Map{Table: "commerce_dispute",
					Cols: map[string]SQL{"reason": "data->>'reason'", "status": "data->>'state'", "amount": squareMoney + "data->'amount_money'" + squareMoneyOf,
						"respond_by": "archive.sync_at(data->>'due_at')", "made_at": "archive.sync_at(coalesce(data->>'reported_at', data->>'created_at'))",
						"edited_at": "archive.sync_at(data->>'updated_at')"},
					Refs: map[string]SQL{"payment": "data#>>'{disputed_payment,payment_id}'"}}},
		},
	})
}

const (
	squareLines   = "an invoice needs lines, each with a name and an amount"
	squareCharges = "Square would charge a card or bank account on file for this invoice when it is sent, which moves money: send it from Square yourself"
	// squareEver is before Square took its first payment: a listing starts there, as it starts a year ago otherwise.
	squareEver = "2009-01-01T00:00:00Z"
	// Square's money is in the currency's smallest unit already: squareMoney, the money there, then squareMoneyOf.
	squareMoney   SQL = "(SELECT jsonb_build_object('amount', archive.sync_num(m->>'amount'), 'currency', m->>'currency') FROM (SELECT "
	squareMoneyOf SQL = " m) x WHERE archive.sync_num(m->>'amount') IS NOT NULL)"
	// An invoice's money is its payment requests': squareAsked, a sum over each p, then squareAskedOf.
	squareAsked   SQL = "(SELECT jsonb_build_object('amount', "
	squareAskedOf SQL = ", 'currency', min(p#>>'{computed_amount_money,currency}')) FROM archive.sync_list(data->'payment_requests') p HAVING count(*) > 0)"
	// An order is for who its first fulfillment goes to.
	squareRecipient SQL = "coalesce(data#>'{fulfillments,0,shipment_details,recipient}', data#>'{fulfillments,0,delivery_details,recipient}', data#>'{fulfillments,0,pickup_details,recipient}')"
	// An item's price is its cheapest variation's.
	squareLowest SQL = `(SELECT jsonb_build_object('amount', min(archive.sync_num(v#>>'{item_variation_data,price_money,amount}')), 'currency', min(v#>>'{item_variation_data,price_money,currency}'))
		FROM archive.sync_list(data#>'{item_data,variations}') v WHERE archive.sync_num(v#>>'{item_variation_data,price_money,amount}') IS NOT NULL HAVING count(*) > 0)`
)

var (
	// squareV is the version of Square's API the connector reads, on every request.
	squareV    = map[string][]string{"Square-Version": {"2026-09-16"}}
	squareMe   = Req{URL: "/v2/merchants/me", Header: squareV}
	squarePage = Cursor{At: "cursor"}
	// squareLocations are every location of the seller's, the inactive too, as their payments and orders stay.
	squareLocations = &Parts{Req: Req{URL: "/v2/locations", Header: squareV}, Items: "locations", Vals: Obj{"name": "{name}"}}
)

type (
	squareGet struct {
		Path string `json:"path" jsonschema:"The path and query, like /v2/orders/<id> or /v2/catalog/list?types=ITEM"`
	}
	squarePayments struct {
		Location string `json:"location,omitempty" jsonschema:"The location's id; the main location when left out"`
		From     string `json:"from,omitempty" jsonschema:"Only those made at or after this time, RFC 3339"`
		To       string `json:"to,omitempty" jsonschema:"Only those made before this time, RFC 3339"`
		Limit    int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 100; 50 when left out"`
		Page     string `json:"page,omitempty" jsonschema:"The next of the call before, for those after it"`
	}
	squarePaid struct {
		Payments []map[string]any `json:"payments"`
		Next     string           `json:"next" jsonschema:"Pass as page for more; empty when there is no more"`
	}
	squareOrders struct {
		Location string `json:"location,omitempty" jsonschema:"Only this location's, by id; every location's, up to 10, when left out"`
		State    string `json:"state,omitempty" jsonschema:"Only those in this state: OPEN, COMPLETED, CANCELED or DRAFT"`
		Customer string `json:"customer,omitempty" jsonschema:"Only this customer's, by id"`
		Limit    int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 500; 50 when left out"`
		Page     string `json:"page,omitempty" jsonschema:"The next of the call before, for those after it"`
	}
	squareOrdered struct {
		Orders []map[string]any `json:"orders"`
		Next   string           `json:"next" jsonschema:"Pass as page for more; empty when there is no more"`
	}
	squareFind struct {
		Email string `json:"email,omitempty" jsonschema:"Only those whose email has this"`
		Phone string `json:"phone,omitempty" jsonschema:"Only those whose phone number has this"`
		Limit int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 100; 50 when left out"`
		Page  string `json:"page,omitempty" jsonschema:"The next of the call before, for those after it"`
	}
	squareFound struct {
		Customers []map[string]any `json:"customers"`
		Next      string           `json:"next" jsonschema:"Pass as page for more; empty when there is no more"`
	}
	squareInvoices struct {
		Location string `json:"location,omitempty" jsonschema:"The location's id; the main location when left out"`
		Customer string `json:"customer,omitempty" jsonschema:"Only this customer's, by id"`
		Limit    int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 200; 50 when left out"`
		Page     string `json:"page,omitempty" jsonschema:"The next of the call before, for those after it"`
	}
	squareBilled struct {
		Invoices []map[string]any `json:"invoices"`
		Next     string           `json:"next" jsonschema:"Pass as page for more; empty when there is no more"`
	}
	squareNew struct {
		GivenName   string `json:"given_name,omitempty" jsonschema:"Their first name"`
		FamilyName  string `json:"family_name,omitempty" jsonschema:"Their last name"`
		CompanyName string `json:"company_name,omitempty" jsonschema:"Their company"`
		Email       string `json:"email_address,omitempty" jsonschema:"Their email address"`
		Phone       string `json:"phone_number,omitempty" jsonschema:"Their phone number"`
		Note        string `json:"note,omitempty" jsonschema:"A note on them"`
	}
	squareDraft struct {
		Customer    string        `json:"customer" jsonschema:"Who it is to, by their customer id; Square emails them when it is sent"`
		Due         string        `json:"due" jsonschema:"The day it is due, YYYY-MM-DD"`
		Lines       []squareDraws `json:"lines" jsonschema:"What it is for"`
		Title       string        `json:"title,omitempty" jsonschema:"Its title"`
		Description string        `json:"description,omitempty" jsonschema:"What it says to the customer"`
		Location    string        `json:"location,omitempty" jsonschema:"The location's id; the main location when left out"`
	}
	squareDraws struct {
		Name     string `json:"name" jsonschema:"What it is"`
		Quantity string `json:"quantity,omitempty" jsonschema:"How many, as a decimal string; 1 when left out"`
		Amount   int    `json:"amount" jsonschema:"What one costs, in the smallest unit of the currency, like cents"`
		Note     string `json:"note,omitempty" jsonschema:"A note on it"`
	}
	squareSend struct {
		ID string `json:"id" jsonschema:"The invoice's id"`
	}
)
