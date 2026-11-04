package connect

import "time"

// Shopify, a store's admin through its GraphQL Admin API, read only. Shopify's sign-in is per store and for apps
// it lists, so this connects as a custom app the store makes itself: in the store's admin, Settings, Apps and
// sales channels, Develop apps, an app with the Admin API scopes read_orders, read_products and
// read_customers, whose Admin API access token an owner pastes in with the store's address. The token lasts
// until the store uninstalls the app. A GraphQL document with anything but queries in it is refused before it goes.
//
// It syncs the store's orders, with their lines, into commerce_order and commerce_line, its products into
// commerce_product, and its customers into crm_contact, the person of their email. Each is listed whole the
// first time and a week after, which deletes what is gone, and in between searched for what changed since the
// sync before. It pushes nothing yet: Shopify's webhooks are made per store and topic.
func init() {
	Register(&Connector{
		ID: "shopify", Name: "Shopify", Integration: true,
		Info: Info{By: "Shopify", Description: "Read the store’s orders, products and customers", Category: CatCommerce, Icon: IconCard, Scopes: []string{"Read orders, products and customers with the store’s app token"}},
		Keys: []Key{
			{Name: "shop", Label: "Store address", Help: "Your store's myshopify.com address, like acme.myshopify.com"},
			{Name: "token", Label: "Admin API access token", Secret: true,
				Help: "In your Shopify admin, Settings, Apps and sales channels, Develop apps: make an app with the Admin API scopes read_orders, " +
					"read_all_orders, read_products and read_customers, install it, and copy its Admin API access token, which starts with shpat_"},
		},
		Sign: map[string]string{"X-Shopify-Access-Token": "{key/token}"},
		// The store, given as its address or a URL of it, in the version of the Admin API the connector reads.
		API: "https://{+shop}/admin/api/2025-10",
		Let: []Step{
			{As: "shop", In: "{key/shop}", Find: `^(?:[^/]*://)?([^/]*)`},
			{Only: Test{At: "shop", Re: `^[^.]*$`}, As: "shop", In: "{shop}.myshopify.com"},
		},
		// Shopify answers errors with a 200, and running out of the store's query budget is one the sync tries again later.
		Errors: &Errors{At: "errors/0/message", Code: "errors/0/extensions/code", Busy: []string{"THROTTLED"}},
		Who:    &Who{Req: Req{Method: POST, Read: true, URL: "/graphql.json", Body: Obj{"query": "{ shop { name myshopifyDomain } }"}}, Names: []Path{"data/shop/myshopifyDomain", "data/shop/name"}},
		Actions: map[string]*Action{
			"graphql": Call[shopifyQuery, any](Plan{
				Doc: "Run a query of Shopify's GraphQL Admin API, like { query: \"{ orders(first: 10, query: \\\"financial_status:pending\\\") " +
					"{ nodes { name totalPriceSet { shopMoney { amount } } } } }\" }: its data. Queries only; a mutation is refused",
				Steps: []Step{
					{Codec: GraphQLReads, In: "{query}"},
					{As: "got", Req: Req{Method: POST, Read: true, URL: "/graphql.json", Body: Obj{"query": "{query}", "variables": "{variables}"}}},
				},
				Out: "{got/data}",
			}),
		},
		Sync: []*Sync{
			{Name: "products", Gap: time.Second, Req: shopifyList, Items: "data/products/nodes", Updated: "updatedAt", Page: Cursor{At: "data/products/pageInfo/endCursor", More: "data/products/pageInfo/hasNextPage"},
				Change: shopifyChange, Deny: shopifyDeny, Vars: Obj{"first": 100, "what": "query($first: Int!, $after: String) { products(first: $first, after: $after, sortKey: ID) { nodes { " + shopifyProduct + " } pageInfo { hasNextPage endCursor } } }",
					"since_query": "query($first: Int!, $after: String, $query: String) { products(first: $first, after: $after, query: $query, sortKey: UPDATED_AT) { nodes { " + shopifyProduct + " } pageInfo { hasNextPage endCursor } } }", "field": "products"},
				Doc: "Every product, with its vendor, type, status, tags, lowest price, stock and description, cut at 64 KB, by its id: listed whole the first time and a week after, which deletes those gone, and in between those updated since the sync before.",
				To: Map{Table: "commerce_product",
					Cols: map[string]SQL{"title": "data->>'title'", "vendor": "data->>'vendor'", "type": "data->>'productType'", "status": "data->>'status'", "tags": "coalesce(data->'tags', '[]'::jsonb)",
						"price":     "archive.sync_money(data#>>'{priceRangeV2,minVariantPrice,amount}', coalesce(nullif(data#>>'{priceRangeV2,minVariantPrice,currencyCode}', ''), 'USD'))",
						"inventory": "archive.sync_num(data->>'totalInventory')", "description": "archive.sync_clip(data->>'description', 65536)", "url": "data->>'onlineStoreUrl'",
						"made_at": "archive.sync_at(data->>'createdAt')", "edited_at": "archive.sync_at(data->>'updatedAt')"}}},
			{Name: "customers", Gap: time.Second, Req: shopifyList, Items: "data/customers/nodes", Updated: "updatedAt", Page: Cursor{At: "data/customers/pageInfo/endCursor", More: "data/customers/pageInfo/hasNextPage"},
				Change: shopifyChange, Deny: shopifyDeny, Vars: Obj{"first": 100, "what": "query($first: Int!, $after: String) { customers(first: $first, after: $after, sortKey: ID) { nodes { " + shopifyCustomer + " } pageInfo { hasNextPage endCursor } } }",
					"since_query": "query($first: Int!, $after: String, $query: String) { customers(first: $first, after: $after, query: $query, sortKey: UPDATED_AT) { nodes { " + shopifyCustomer + " } pageInfo { hasNextPage endCursor } } }", "field": "customers"},
				Doc: "Every customer, with their name, email, phone and address, by its id: listed whole the first time and a week after, which deletes those gone, and in between those updated since the sync before.",
				To: Map{Table: "crm_contact",
					Cols: map[string]SQL{"first_name": "data->>'firstName'", "last_name": "data->>'lastName'", "email": "archive.sync_email(data#>>'{defaultEmailAddress,emailAddress}')",
						"phone": "data#>>'{defaultPhoneNumber,phoneNumber}'", "stage": "'customer'", "city": "data#>>'{defaultAddress,city}'", "state": "data#>>'{defaultAddress,provinceCode}'",
						"country": "data#>>'{defaultAddress,countryCodeV2}'", "made_at": "archive.sync_at(data->>'createdAt')", "edited_at": "archive.sync_at(data->>'updatedAt')"},
					People: map[string]SQL{"person": "data#>>'{defaultEmailAddress,emailAddress}'"}}},
			// Without read_all_orders, Shopify lists only the last 60 days of orders: such a listing is no listing of
			// them all, and deletes nothing. A listing's every page says which scopes the app has, so that a page never asks apart.
			{Name: "orders", Gap: time.Second, Req: shopifyList, Items: "data/orders/nodes", Updated: "updatedAt", Page: Cursor{At: "data/orders/pageInfo/endCursor", More: "data/orders/pageInfo/hasNextPage"},
				Change: shopifyChange, Deny: shopifyDeny, Short: Test{At: "data/currentAppInstallation/accessScopes/*/handle", Is: "read_all_orders", Off: true},
				Vars: Obj{"first": 5, "what": "query($first: Int!, $after: String) { orders(first: $first, after: $after, sortKey: ID) { nodes { " + shopifyOrder + " } pageInfo { hasNextPage endCursor } } currentAppInstallation { accessScopes { handle } } }",
					"since_query": "query($first: Int!, $after: String, $query: String) { orders(first: $first, after: $after, query: $query, sortKey: UPDATED_AT) { nodes { " + shopifyOrder + " } pageInfo { hasNextPage endCursor } } }", "field": "orders"},
				Doc: "Every order, with its customer, totals, payment and shipping status, tags and note, and up to 25 of its lines; without " +
					"read_all_orders Shopify lists only the last 60 days, so a listing deletes no order, by its id: listed whole the first time and a week after, which deletes those gone, and in between those updated since the sync before.",
				To: Map{Table: "commerce_order",
					Cols: map[string]SQL{"number": "data->>'name'", "customer_name": "data#>>'{customer,displayName}'", "email": "archive.sync_email(data->>'email')",
						"total":     "archive.sync_money(data#>>'{totalPriceSet,shopMoney,amount}', coalesce(nullif(data#>>'{totalPriceSet,shopMoney,currencyCode}', ''), 'USD'))",
						"subtotal":  "archive.sync_money(data#>>'{subtotalPriceSet,shopMoney,amount}', coalesce(nullif(data#>>'{subtotalPriceSet,shopMoney,currencyCode}', ''), 'USD'))",
						"tax":       "archive.sync_money(data#>>'{totalTaxSet,shopMoney,amount}', coalesce(nullif(data#>>'{totalTaxSet,shopMoney,currencyCode}', ''), 'USD'))",
						"shipping":  "archive.sync_money(data#>>'{totalShippingPriceSet,shopMoney,amount}', coalesce(nullif(data#>>'{totalShippingPriceSet,shopMoney,currencyCode}', ''), 'USD'))",
						"discounts": "archive.sync_money(data#>>'{totalDiscountsSet,shopMoney,amount}', coalesce(nullif(data#>>'{totalDiscountsSet,shopMoney,currencyCode}', ''), 'USD'))",
						"refunded":  "archive.sync_money(data#>>'{totalRefundedSet,shopMoney,amount}', coalesce(nullif(data#>>'{totalRefundedSet,shopMoney,currencyCode}', ''), 'USD'))",
						"paid":      "data->>'displayFinancialStatus'", "fulfilled": "data->>'displayFulfillmentStatus'", "cancelled_at": "archive.sync_at(data->>'cancelledAt')",
						"city": "data#>>'{shippingAddress,city}'", "state": "data#>>'{shippingAddress,provinceCode}'", "country": "data#>>'{shippingAddress,countryCodeV2}'",
						"tags": "coalesce(data->'tags', '[]'::jsonb)", "note": "data->>'note'", "placed_at": "archive.sync_at(data->>'createdAt')", "edited_at": "archive.sync_at(data->>'updatedAt')"},
					People: map[string]SQL{"customer": "data->>'email'"},
					Children: []Kids{{Table: "commerce_line", From: "archive.sync_list(data#>'{lineItems,nodes}') l",
						Cols: map[string]SQL{"title": "l->>'title'", "variant": "l->>'variantTitle'", "sku": "l->>'sku'", "quantity": "archive.sync_num(l->>'quantity')",
							"price": "archive.sync_money(l#>>'{originalUnitPriceSet,shopMoney,amount}', coalesce(nullif(l#>>'{originalUnitPriceSet,shopMoney,currencyCode}', ''), 'USD'))",
							"total": "archive.sync_money(l#>>'{discountedTotalSet,shopMoney,amount}', coalesce(nullif(l#>>'{discountedTotalSet,shopMoney,currencyCode}', ''), 'USD'))"},
						Refs: map[string]SQL{"product": "l#>>'{product,id}'"}}}}},
		},
	})
}

const (
	shopifyMoney   = "shopMoney { amount currencyCode }"
	shopifyProduct = "id title vendor productType status tags createdAt updatedAt onlineStoreUrl description totalInventory " +
		"priceRangeV2 { minVariantPrice { amount currencyCode } }"
	shopifyCustomer = "id firstName lastName displayName createdAt updatedAt tags defaultEmailAddress { emailAddress } " +
		"defaultPhoneNumber { phoneNumber } defaultAddress { city provinceCode countryCodeV2 }"
	shopifyOrder = "id name email createdAt updatedAt cancelledAt displayFinancialStatus displayFulfillmentStatus tags note " +
		"customer { displayName } shippingAddress { city provinceCode countryCodeV2 } " +
		"totalPriceSet { " + shopifyMoney + " } subtotalPriceSet { " + shopifyMoney + " } totalTaxSet { " + shopifyMoney + " } " +
		"totalShippingPriceSet { " + shopifyMoney + " } totalDiscountsSet { " + shopifyMoney + " } totalRefundedSet { " + shopifyMoney + " } " +
		"lineItems(first: 25) { nodes { title variantTitle sku quantity product { id } originalUnitPriceSet { " + shopifyMoney + " } " +
		"discountedTotalSet { " + shopifyMoney + " } } }"
)

var (
	shopifyList   = Req{Method: POST, Read: true, URL: "/graphql.json", Body: Obj{"query": "{what}", "variables": Obj{"first": "{first}", "after": "{page}"}}}
	shopifyChange = SinceTime{Req: Req{Method: POST, Read: true, URL: "/graphql.json",
		Body: Obj{"query": "{since_query}", "variables": Obj{"first": "{first}", "after": "{page}", "query": "updated_at:>'{since}'"}}}}
	shopifyDeny = []Deny{{Status: []int{400}, Says: []string{"access denied"}, Why: "the store's app may not read {field}; give it the scope read_{field}"}}
)

type shopifyQuery struct {
	Query     string         `json:"query" jsonschema:"The GraphQL query"`
	Variables map[string]any `json:"variables,omitempty" jsonschema:"Its variables"`
}
