package connect

import (
	"time"

	"golang.org/x/oauth2"
)

// Shopify, a store's admin through its GraphQL Admin API. It signs in as Archive's own Shopify app: a person names
// their store's myshopify.com address, and Shopify asks its owner to install the app there, with OAuth's
// authorization code grant on the store's own host, whose callback Shopify signs with an hmac of its query. The
// token it trades the code for expires in an hour and comes with a refresh token, which a refresh replaces, as
// Shopify has every public app's since April 2026. Disconnecting uninstalls the app.
//
// Before Shopify's sign-in, a store pasted the Admin API token of a custom app it made in its own admin, which
// Shopify stopped making new ones of on January 1, 2026: shopify_token below still connects with one, and a store
// connected so before keeps working as this one, its Was.
//
// It syncs the store's orders, with their lines, into commerce_order and commerce_line, its products into
// commerce_product, and its customers into crm_contact, the person of their email. Each is listed whole the
// first time and a week after, which deletes what is gone, and in between searched for what changed since the
// sync before. Shopify pushes to POST /push/shopify the topics its app's configuration subscribes: app/uninstalled
// starts a sync, which finds the grant gone, and the privacy topics it requires of every app forget what they name:
// customers/redact a customer and their orders, shop/redact the store, 48 hours after it uninstalled the app; a
// customers/data_request is only heard, as the store has all Archive keeps of the customer in its archive.
//
// Its actions read with any GraphQL query, a mutation refused, and write only through the actions that say so:
// a product, its variants' prices, a stock count, draft orders, an order's note and tags, tags of anything that
// has them, and a customer.
func init() {
	Register(&Connector{
		ID: "shopify", Name: "Shopify", Integration: true,
		Info: Info{By: "Shopify", Description: "Read the store’s orders, products and customers, and change them when asked", Category: CatCommerce, Icon: IconCard,
			Scopes: []string{"Read orders, products, customers and inventory", "Change products, prices, stock, draft orders, orders’ notes and tags, and customers when asked"}},
		// The app's client, whose host is only where its variable names it: each store signs in on its own.
		Host:     "https://admin.shopify.com",
		Endpoint: oauth2.Endpoint{AuthURL: "https://{meta/shop}/admin/oauth/authorize", TokenURL: "https://{meta/shop}/admin/oauth/access_token", AuthStyle: oauth2.AuthStyleInParams},
		// write_ scopes have the read_ ones in them, and Shopify grants only those. Orders over 60 days old need
		// read_all_orders, which Shopify approves an app for apart: asked for once the variable has ?all_orders=1.
		Scopes: []string{"write_products", "write_inventory", "write_orders", "write_draft_orders", "write_customers"},
		Extra:  map[string][]string{"all_orders": {"read_all_orders"}},
		Comma:  true,
		Trades: map[string]string{"expiring": "1"},
		Ask: &Key{Name: "shop", Label: "Store address", Help: "Your store's myshopify.com address, like acme.myshopify.com",
			Find: `^(?:https?://)?([a-z0-9][a-z0-9-]*)(?:\.myshopify\.com)?/?(?:admin\b.*)?$`, As: "{it}.myshopify.com"},
		Signed: "hmac",
		Was:    "shopify_token",
		Options: "?all_orders=1 asks for read_all_orders too, for orders over 60 days old, once Shopify has approved the app for it. " +
			"The app's configuration subscribes app/uninstalled and the privacy topics customers/data_request, customers/redact and shop/redact to <API>/push/shopify",
		Sign: map[string]string{"X-Shopify-Access-Token": "{token}"},
		// Uninstalling the app ends its grant.
		RevokeBy: &Req{Method: DELETE, URL: "https://{meta/shop}/admin/api_permissions/current.json", Header: map[string][]string{"X-Shopify-Access-Token": {"{access}"}}},
		API:      shopifyAPI,
		Let:      []Step{{As: "shop", In: "{meta/shop}"}},
		Errors:   shopifyErrors,
		Who:      shopifyWho,
		Listen: &Listen{
			Signs: []Sign{{Header: "X-Shopify-Hmac-Sha256", Of: "{body}"}},
			Key:   "{meta/shop}", Lower: true,
			Keys: []Path{"shop_domain;myshopify_domain"},
			Forget: []Forget{
				{When: "X-Shopify-Topic", Is: "customers/data_request"},
				{When: "X-Shopify-Topic", Is: "customers/redact", Of: []Forgot{
					{Stream: "customers", At: "customer/id", ID: "gid://shopify/Customer/{it}"},
					{Stream: "orders", At: "orders_to_redact", ID: "gid://shopify/Order/{it}"}}},
				{When: "X-Shopify-Topic", Is: "shop/redact", All: true},
			},
		},
		Actions: shopifyActions(),
		Sync:    shopifySync(),
	})

	// The token of a custom app a store made in its own admin before January 1, 2026, pasted in with the store's
	// address, which lasts until the store uninstalls the app. It does what that app's scopes let it.
	Register(&Connector{
		ID: "shopify_token", Name: "Shopify (app token)", Integration: true,
		Info: Info{By: "Shopify", Description: "Read the store’s orders, products and customers with a custom app the store made before 2026", Category: CatCommerce, Icon: IconCard,
			Scopes: []string{"Read orders, products and customers, and change them as the store’s app may, with its token"}},
		Keys: []Key{
			{Name: "shop", Label: "Store address", Help: "Your store's myshopify.com address, like acme.myshopify.com"},
			{Name: "token", Label: "Admin API access token", Secret: true,
				Help: "In your Shopify admin, Settings, Apps and sales channels, Develop apps: the custom app's Admin API access token, which starts with shpat_. " +
					"Shopify makes no new such apps since 2026; connect Shopify itself instead"},
		},
		Sign: map[string]string{"X-Shopify-Access-Token": "{key/token}"},
		API:  shopifyAPI,
		Let: []Step{
			{As: "shop", In: "{key/shop}", Find: `^(?:[^/]*://)?([^/]*)`},
			{Only: Test{At: "shop", Re: `^[^.]*$`}, As: "shop", In: "{shop}.myshopify.com"},
		},
		Errors:  shopifyErrors,
		Who:     shopifyWho,
		Actions: shopifyActions(),
		Sync:    shopifySync(),
	})
}

const (
	// shopifyAPI is the store, in the version of the Admin API the connector reads.
	shopifyAPI = "https://{+shop}/admin/api/2026-10"

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
	shopifyDraft = "id name status invoiceUrl email note tags totalPriceSet { " + shopifyMoney + " } " +
		"lineItems(first: 50) { nodes { title quantity variant { id } originalUnitPriceSet { " + shopifyMoney + " } } }"
	shopifyFailed = "field message"
)

var (
	shopifyList   = Req{Method: POST, Read: true, URL: "/graphql.json", Body: Obj{"query": "{what}", "variables": Obj{"first": "{first}", "after": "{page}"}}}
	shopifyChange = SinceTime{Req: Req{Method: POST, Read: true, URL: "/graphql.json",
		Body: Obj{"query": "{since_query}", "variables": Obj{"first": "{first}", "after": "{page}", "query": "updated_at:>'{since}'"}}}}
	shopifyDeny = []Deny{{Status: []int{400}, Says: []string{"access denied"}, Why: "the store's app may not read {field}; give it the scope read_{field}"}}
	// Shopify answers errors with a 200, and running out of the store's query budget is one the sync tries again later.
	shopifyErrors = &Errors{At: "errors/0/message", Code: "errors/0/extensions/code", Busy: []string{"THROTTLED"}}
	shopifyWho    = &Who{Req: Req{Method: POST, Read: true, URL: "/graphql.json", Body: Obj{"query": "{ shop { name myshopifyDomain } }"}}, Names: []Path{"data/shop/myshopifyDomain", "data/shop/name"}}
)

// shopifyMutation is the steps of an action that sends mutation, named name, with vars, failing with the first
// of the userErrors Shopify answers it with in a 200.
func shopifyMutation(name, mutation string, vars Obj) []Step {
	return []Step{
		{As: "got", Req: Req{Method: POST, URL: "/graphql.json", Body: Obj{"query": mutation, "variables": vars}}},
		{Only: Test{At: Path("got/data/" + name + "/userErrors/0/message")}, Fail: "Shopify did not take it: {got/data/" + name + "/userErrors/0/message}"},
	}
}

// shopifyActions are a new copy of the actions, for each connector to compile its own.
func shopifyActions() map[string]*Action {
	return map[string]*Action{
		"graphql": Call[shopifyQuery, any](Plan{
			Doc: "Run a query of Shopify's GraphQL Admin API, like { query: \"{ orders(first: 10, query: \\\"financial_status:pending\\\") " +
				"{ nodes { name totalPriceSet { shopMoney { amount } } } } }\" }: its data. Queries only; a mutation is refused, and changes go through the actions that make them",
			Steps: []Step{
				{Codec: GraphQLReads, In: "{query}"},
				{As: "got", Req: Req{Method: POST, Read: true, URL: "/graphql.json", Body: Obj{"query": "{query}", "variables": "{variables}"}}},
			},
			Out: "{got/data}",
		}),
		"update_product": Call[shopifyChanges, any](Plan{
			Doc: "Change a product, like { id: \"gid://shopify/Product/1\", changes: { title, descriptionHtml, tags, status, vendor, productType } }, " +
				"its fields by ProductUpdateInput's names: tags replace the old. Prices are its variants', with update_variants, and stock with set_inventory",
			Steps: append([]Step{{As: "changes/id", In: "{id}"}}, shopifyMutation("productUpdate",
				"mutation($product: ProductUpdateInput!) { productUpdate(product: $product) { product { id title status tags updatedAt } userErrors { "+shopifyFailed+" } } }",
				Obj{"product": "{changes}"})...),
			Out: "{got/data/productUpdate/product}",
		}),
		"update_variants": Call[shopifyVariants, any](Plan{
			Doc: "Change variants of a product, like their price: { product_id: \"gid://shopify/Product/1\", variants: [{ id: \"gid://shopify/ProductVariant/2\", price: \"19.99\", " +
				"compareAtPrice: \"24.99\" }] }, each by ProductVariantsBulkInput's names",
			Steps: shopifyMutation("productVariantsBulkUpdate",
				"mutation($productId: ID!, $variants: [ProductVariantsBulkInput!]!) { productVariantsBulkUpdate(productId: $productId, variants: $variants) { "+
					"productVariants { id title price compareAtPrice } userErrors { "+shopifyFailed+" } } }",
				Obj{"productId": "{product_id}", "variants": "{variants}"}),
			Out: "{got/data/productVariantsBulkUpdate/productVariants}",
		}),
		"set_inventory": Call[shopifyStock, any](Plan{
			Doc: "Set how many of an item a location has, like { inventory_item_id: \"gid://shopify/InventoryItem/1\", location_id: \"gid://shopify/Location/2\", " +
				"quantity: 12, change_from_quantity: 10 }: change_from_quantity is how many it has now, as last read, so a count that changed since is not overwritten",
			Enum: map[string][]string{"name": {"available", "on_hand"}, "reason": {"correction", "cycle_count_available", "damaged", "movement_created", "movement_received", "other", "promotion", "received", "restock", "safety_stock", "shrinkage"}},
			Steps: shopifyMutation("inventorySetQuantities",
				"mutation($input: InventorySetQuantitiesInput!, $key: String!) { inventorySetQuantities(input: $input) @idempotent(key: $key) { "+
					"inventoryAdjustmentGroup { reason changes { name delta quantityAfterChange } } userErrors { "+shopifyFailed+" } } }",
				Obj{"key": "{nonce}", "input": Obj{"name": "{name|available}", "reason": "{reason|correction}",
					"quantities": []any{Obj{"inventoryItemId": "{inventory_item_id}", "locationId": "{location_id}", "quantity": "{quantity}", "changeFromQuantity": "{change_from_quantity}"}}}}),
			Out: "{got/data/inventorySetQuantities/inventoryAdjustmentGroup}",
		}),
		"create_draft_order": Call[shopifyDraftInput, any](Plan{
			Doc: "Make a draft order, like { input: { lineItems: [{ variantId: \"gid://shopify/ProductVariant/2\", quantity: 1 }], email, note, tags, " +
				"purchasingEntity: { customerId: \"gid://shopify/Customer/3\" }, shippingAddress } }, by DraftOrderInput's names: the draft, with its invoice URL. It charges nobody",
			Steps: shopifyMutation("draftOrderCreate",
				"mutation($input: DraftOrderInput!) { draftOrderCreate(input: $input) { draftOrder { "+shopifyDraft+" } userErrors { "+shopifyFailed+" } } }",
				Obj{"input": "{input}"}),
			Out: "{got/data/draftOrderCreate/draftOrder}",
		}),
		"update_draft_order": Call[shopifyDraftChanges, any](Plan{
			Doc: "Change a draft order, like { id: \"gid://shopify/DraftOrder/1\", input: { note, tags, lineItems } }, by DraftOrderInput's names: lineItems replace the old",
			Steps: shopifyMutation("draftOrderUpdate",
				"mutation($id: ID!, $input: DraftOrderInput!) { draftOrderUpdate(id: $id, input: $input) { draftOrder { "+shopifyDraft+" } userErrors { "+shopifyFailed+" } } }",
				Obj{"id": "{id}", "input": "{input}"}),
			Out: "{got/data/draftOrderUpdate/draftOrder}",
		}),
		"update_order": Call[shopifyChanges, any](Plan{
			Doc: "Change an order's note or tags, like { id: \"gid://shopify/Order/1\", changes: { note: \"Gift wrap\", tags: [\"vip\"] } }, by OrderInput's names: " +
				"tags replace the old, and add_tags adds",
			Steps: append([]Step{{As: "changes/id", In: "{id}"}}, shopifyMutation("orderUpdate",
				"mutation($input: OrderInput!) { orderUpdate(input: $input) { order { id name note tags } userErrors { "+shopifyFailed+" } } }",
				Obj{"input": "{changes}"})...),
			Out: "{got/data/orderUpdate/order}",
		}),
		"add_tags": Call[shopifyTags, any](Plan{
			Doc: "Add tags to an order, draft order, product or customer, like { id: \"gid://shopify/Order/1\", tags: [\"vip\"] }, keeping those it has",
			Steps: shopifyMutation("tagsAdd",
				"mutation($id: ID!, $tags: [String!]!) { tagsAdd(id: $id, tags: $tags) { node { id } userErrors { "+shopifyFailed+" } } }",
				Obj{"id": "{id}", "tags": "{tags}"}),
			Out: "{got/data/tagsAdd/node}",
		}),
		"update_customer": Call[shopifyChanges, any](Plan{
			Doc: "Change a customer, like { id: \"gid://shopify/Customer/1\", changes: { firstName, lastName, email, phone, note, tags } }, by CustomerInput's names: tags replace the old",
			Steps: append([]Step{{As: "changes/id", In: "{id}"}}, shopifyMutation("customerUpdate",
				"mutation($input: CustomerInput!) { customerUpdate(input: $input) { customer { id displayName note tags updatedAt } userErrors { "+shopifyFailed+" } } }",
				Obj{"input": "{changes}"})...),
			Out: "{got/data/customerUpdate/customer}",
		}),
	}
}

// shopifySync is a new copy of the streams, for each connector to compile its own.
func shopifySync() []*Sync {
	return []*Sync{
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
	}
}

type (
	shopifyQuery struct {
		Query     string         `json:"query" jsonschema:"The GraphQL query"`
		Variables map[string]any `json:"variables,omitempty" jsonschema:"Its variables"`
	}
	shopifyChanges struct {
		ID      string         `json:"id" jsonschema:"Its id, like gid://shopify/Product/1"`
		Changes map[string]any `json:"changes" jsonschema:"The fields to change, by the API's names"`
	}
	shopifyVariants struct {
		ProductID string           `json:"product_id" jsonschema:"The product's id, like gid://shopify/Product/1"`
		Variants  []map[string]any `json:"variants" jsonschema:"Each variant to change: its id and the fields to change, like price and compareAtPrice"`
	}
	shopifyStock struct {
		InventoryItemID    string `json:"inventory_item_id" jsonschema:"The variant's inventory item, like gid://shopify/InventoryItem/1, its inventoryItem { id }"`
		LocationID         string `json:"location_id" jsonschema:"The location, like gid://shopify/Location/2"`
		Quantity           int    `json:"quantity" jsonschema:"How many it has from now"`
		ChangeFromQuantity int    `json:"change_from_quantity" jsonschema:"How many it has now, as last read"`
		Name               string `json:"name,omitempty" jsonschema:"Which count: available, the default, or on_hand"`
		Reason             string `json:"reason,omitempty" jsonschema:"Why, correction unless it says"`
	}
	shopifyDraftInput struct {
		Input map[string]any `json:"input" jsonschema:"The draft order, by DraftOrderInput's names"`
	}
	shopifyDraftChanges struct {
		ID    string         `json:"id" jsonschema:"The draft order's id, like gid://shopify/DraftOrder/1"`
		Input map[string]any `json:"input" jsonschema:"The fields to change, by DraftOrderInput's names"`
	}
	shopifyTags struct {
		ID   string   `json:"id" jsonschema:"Its id, like gid://shopify/Order/1"`
		Tags []string `json:"tags" jsonschema:"The tags to add"`
	}
)
