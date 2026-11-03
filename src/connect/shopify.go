package connect

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Shopify, a store's admin through its GraphQL Admin API, read only. Shopify's sign-in is per store and for apps
// it lists, so this connects as a custom app the store makes itself: in the store's admin, Settings, Apps and
// sales channels, Develop apps, an app with the Admin API scopes read_orders, read_products and
// read_customers, whose Admin API access token an owner pastes in with the store's address. The token lasts
// until the store uninstalls the app.
//
// It syncs the store's orders, with their lines, into commerce_order and commerce_line, its products into
// commerce_product, and its customers into crm_contact, the person of their email. Each is listed whole the
// first time and a week after, which deletes what is gone, and in between searched for what changed since the
// sync before. It pushes nothing yet: Shopify's webhooks are made per store and topic.
func init() {
	Register(&Connector{
		ID: "shopify", Name: "Shopify", Integration: true,
		Keys: []Key{
			{Name: "shop", Label: "Store address", Help: "Your store's myshopify.com address, like acme.myshopify.com"},
			{Name: "token", Label: "Admin API access token", Secret: true,
				Help: "In your Shopify admin, Settings, Apps and sales channels, Develop apps: make an app with the Admin API scopes read_orders, " +
					"read_all_orders, read_products and read_customers, install it, and copy its Admin API access token, which starts with shpat_"},
		},
		Authorize: func(r *http.Request, c *Client) { r.Header.Set("X-Shopify-Access-Token", c.Key("token")) },
		Account: func(c *Client) (string, error) {
			var out struct {
				Shop struct {
					Name   string `json:"name"`
					Domain string `json:"myshopifyDomain"`
				} `json:"shop"`
			}
			err := shopifyQL(c, "{ shop { name myshopifyDomain } }", nil, &out)
			return cmp.Or(out.Shop.Domain, out.Shop.Name), err
		},
		Actions: map[string]*Action{
			"graphql": Act("Run a query of Shopify's GraphQL Admin API, like { query: \"{ orders(first: 10, query: \\\"financial_status:pending\\\") "+
				"{ nodes { name totalPriceSet { shopMoney { amount } } } } }\" }: its data. Queries only; a mutation is refused",
				func(c *Client, in struct {
					Query     string         `json:"query" jsonschema:"The GraphQL query"`
					Variables map[string]any `json:"variables,omitempty" jsonschema:"Its variables"`
				}) (any, error) {
					if strings.HasPrefix(strings.TrimSpace(in.Query), "mutation") {
						return nil, errors.New("Archive reads Shopify, and runs no mutations")
					}
					var out any
					err := shopifyQL(c, in.Query, in.Variables, &out)
					return out, err
				}),
		},
		Streams: []*Stream{
			shopifyStream("products", "products", "ProductSortKeys", 100, shopifyProductFields, "commerce_product", shopifyProductRows,
				"Every product, with its vendor, type, status, tags, lowest price, stock and description, cut at 64 KB"),
			shopifyStream("customers", "customers", "CustomerSortKeys", 100, shopifyCustomerFields, "crm_contact", shopifyCustomerRows,
				"Every customer, with their name, email, phone and address"),
			shopifyStream("orders", "orders", "OrderSortKeys", 5, shopifyOrderFields, "commerce_order", shopifyOrderRows,
				"Every order, with its customer, totals, payment and shipping status, tags and note, and up to 25 of its lines; without "+
					"read_all_orders Shopify lists only the last 60 days, so a listing deletes no order"),
		},
	})
}

// shopifyVersion is the version of the Admin API the connector reads.
const shopifyVersion = "2025-10"

// shopifyURL is the GraphQL Admin API of the store the client's keys name, given as its address or a URL of it.
func shopifyURL(c *Client) string {
	shop := strings.ToLower(strings.TrimSpace(c.Key("shop")))
	if i := strings.Index(shop, "://"); i >= 0 {
		shop = shop[i+3:]
	}
	shop, _, _ = strings.Cut(shop, "/")
	if !strings.Contains(shop, ".") {
		shop += ".myshopify.com"
	}
	return "https://" + shop + "/admin/api/" + shopifyVersion + "/graphql.json"
}

// shopifyQL runs a GraphQL query on the store into out, its data. Shopify answers errors with a 200: they are a
// Refusal, and running out of the store's query budget is a 429, which the sync tries again later.
func shopifyQL(c *Client, query string, vars map[string]any, out any) error {
	var res struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message    string `json:"message"`
			Extensions struct {
				Code string `json:"code"`
			} `json:"extensions"`
		} `json:"errors"`
	}
	if err := c.Do("POST", shopifyURL(c), map[string]any{"query": query, "variables": vars}, &res); err != nil {
		return err
	}
	if len(res.Errors) > 0 {
		e := res.Errors[0]
		status := http.StatusBadRequest
		if e.Extensions.Code == "THROTTLED" {
			status = http.StatusTooManyRequests
		}
		return &Refusal{Provider: "Shopify", Status: status, Message: e.Message}
	}
	if len(res.Data) == 0 || string(res.Data) == "null" {
		return nil
	}
	return json.Unmarshal(res.Data, out)
}

// shopifyAllOrders says whether the store's app may read every order, not only the last 60 days'.
func shopifyAllOrders(c *Client) bool {
	var out struct {
		App struct {
			Scopes []struct {
				Handle string `json:"handle"`
			} `json:"accessScopes"`
		} `json:"currentAppInstallation"`
	}
	if shopifyQL(c, "{ currentAppInstallation { accessScopes { handle } } }", nil, &out) != nil {
		return false
	}
	for _, s := range out.App.Scopes {
		if s.Handle == "read_all_orders" {
			return true
		}
	}
	return false
}

// shopifyCursor is where a stream is: Since when the sync its records are as of began, Listed when its last
// full listing began, and while a sync is under way, Mode list or delta, Began when it began and After the
// cursor of its next page.
type shopifyCursor struct {
	Since  string `json:"since,omitempty"`
	Listed string `json:"listed,omitempty"`
	Mode   string `json:"mode,omitempty"`
	Began  string `json:"began,omitempty"`
	After  string `json:"after,omitempty"`
}

// shopifyStream is the stream of the store's connection field, size a page, each node with fields, into table.
func shopifyStream(name, field, sortKeys string, size int, fields, table string, rows func(map[string]any) *Row, what string) *Stream {
	return &Stream{Name: name, Gap: time.Second, Table: table, Rows: rows,
		Doc: what + ", by its id: listed whole the first time and a week after, which deletes those gone, and in between those updated " +
			"since the sync before.",
		Pull: func(c *Client, cursor string) (*Page, error) {
			var cur shopifyCursor
			if err := cursorOf(cursor, &cur); err != nil {
				return nil, err
			}
			now := time.Now().Add(-time.Minute).UTC()
			if cur.Mode == "" {
				cur.Mode, cur.Began, cur.After = "delta", now.Format(time.RFC3339), ""
				if cur.Since == "" || now.Sub(stamp(cur.Listed)) > 7*24*time.Hour {
					cur.Mode = "list"
				}
			}
			vars := map[string]any{"first": size}
			sort, search := "ID", ""
			if cur.Mode == "delta" {
				sort, search = "UPDATED_AT", fmt.Sprintf("updated_at:>'%s'", cur.Since)
				vars["query"] = search
			}
			if cur.After != "" {
				vars["after"] = cur.After
			}
			q := fmt.Sprintf(`query($first: Int!, $after: String, $query: String) { %s(first: $first, after: $after, query: $query, sortKey: %s) {
				nodes { %s } pageInfo { hasNextPage endCursor } } }`, field, sort, fields)
			var out map[string]struct {
				Nodes    []map[string]any `json:"nodes"`
				PageInfo struct {
					More bool   `json:"hasNextPage"`
					End  string `json:"endCursor"`
				} `json:"pageInfo"`
			}
			if err := shopifyQL(c, q, vars, &out); err != nil {
				if r := (*Refusal)(nil); errors.As(err, &r) && r.Status == http.StatusBadRequest && strings.Contains(strings.ToLower(r.Message), "access denied") {
					return nil, unsupported("the store's app may not read %s; give it the scope read_%s", field, field)
				}
				return nil, err
			}
			got := out[field]
			// Without read_all_orders, Shopify lists only the last 60 days of orders: such a listing is no listing of
			// them all, and deletes nothing.
			page := &Page{Full: cur.Mode == "list" && (field != "orders" || shopifyAllOrders(c))}
			for _, m := range got.Nodes {
				page.Records = append(page.Records, Record{ID: str(m, "id"), Data: m, Updated: stamp(str(m, "updatedAt"))})
			}
			if got.PageInfo.More && got.PageInfo.End != "" {
				cur.After, page.More = got.PageInfo.End, true
			} else {
				if cur.Mode == "list" {
					cur.Listed = cur.Began
				}
				cur = shopifyCursor{Since: cur.Began, Listed: cur.Listed}
			}
			page.Cursor = cursorFor(cur)
			return page, nil
		}}
}

const (
	shopifyMoney         = "shopMoney { amount currencyCode }"
	shopifyProductFields = "id title vendor productType status tags createdAt updatedAt onlineStoreUrl description totalInventory " +
		"priceRangeV2 { minVariantPrice { amount currencyCode } }"
	shopifyCustomerFields = "id firstName lastName displayName createdAt updatedAt tags defaultEmailAddress { emailAddress } " +
		"defaultPhoneNumber { phoneNumber } defaultAddress { city provinceCode countryCodeV2 }"
	shopifyOrderFields = "id name email createdAt updatedAt cancelledAt displayFinancialStatus displayFulfillmentStatus tags note " +
		"customer { displayName } shippingAddress { city provinceCode countryCodeV2 } " +
		"totalPriceSet { " + shopifyMoney + " } subtotalPriceSet { " + shopifyMoney + " } totalTaxSet { " + shopifyMoney + " } " +
		"totalShippingPriceSet { " + shopifyMoney + " } totalDiscountsSet { " + shopifyMoney + " } totalRefundedSet { " + shopifyMoney + " } " +
		"lineItems(first: 25) { nodes { title variantTitle sku quantity product { id } originalUnitPriceSet { " + shopifyMoney + " } " +
		"discountedTotalSet { " + shopifyMoney + " } } }"
)

// shopifyAmount is a MoneyBag's amount in the shop's currency, or a MoneyV2's.
func shopifyAmount(v map[string]any) Money {
	if m, ok := v["shopMoney"].(map[string]any); ok {
		v = m
	}
	return money(v["amount"], cmp.Or(str(v, "currencyCode"), "USD"))
}

func shopifyProductRows(m map[string]any) *Row {
	return rowOf(map[string]any{"title": str(m, "title"), "vendor": str(m, "vendor"), "type": str(m, "productType"), "status": str(m, "status"),
		"tags": strs(m["tags"]), "price": shopifyAmount(obj(obj(m, "priceRangeV2"), "minVariantPrice")), "inventory": decimal(m["totalInventory"]),
		"description": clip(str(m, "description"), maxText), "url": str(m, "onlineStoreUrl"), "made_at": instant(str(m, "createdAt")),
		"edited_at": instant(str(m, "updatedAt"))})
}

// shopifyCustomerRows is a customer as a contact, the person of their email.
func shopifyCustomerRows(m map[string]any) *Row {
	email := Email(str(obj(m, "defaultEmailAddress"), "emailAddress"))
	addr := obj(m, "defaultAddress")
	return rowOf(map[string]any{"first_name": str(m, "firstName"), "last_name": str(m, "lastName"), "email": email,
		"phone": str(obj(m, "defaultPhoneNumber"), "phoneNumber"), "stage": "customer", "city": str(addr, "city"), "state": str(addr, "provinceCode"),
		"country": str(addr, "countryCodeV2"), "made_at": instant(str(m, "createdAt")), "edited_at": instant(str(m, "updatedAt"))}).person("person", email)
}

// shopifyOrderRows is an order, placed by the person of its email, with its lines, each the product it is of.
func shopifyOrderRows(m map[string]any) *Row {
	email := Email(str(m, "email"))
	ship := obj(m, "shippingAddress")
	r := rowOf(map[string]any{"number": str(m, "name"), "customer_name": str(obj(m, "customer"), "displayName"), "email": email,
		"total": shopifyAmount(obj(m, "totalPriceSet")), "subtotal": shopifyAmount(obj(m, "subtotalPriceSet")), "tax": shopifyAmount(obj(m, "totalTaxSet")),
		"shipping": shopifyAmount(obj(m, "totalShippingPriceSet")), "discounts": shopifyAmount(obj(m, "totalDiscountsSet")),
		"refunded": shopifyAmount(obj(m, "totalRefundedSet")), "paid": str(m, "displayFinancialStatus"), "fulfilled": str(m, "displayFulfillmentStatus"),
		"cancelled_at": instant(str(m, "cancelledAt")), "city": str(ship, "city"), "state": str(ship, "provinceCode"), "country": str(ship, "countryCodeV2"),
		"tags": strs(m["tags"]), "note": str(m, "note"), "placed_at": instant(str(m, "createdAt")), "edited_at": instant(str(m, "updatedAt"))}).
		person("customer", email)
	for _, l := range objects(obj(m, "lineItems")["nodes"], "") {
		r.child("commerce_line", rowOf(map[string]any{"title": str(l, "title"), "variant": str(l, "variantTitle"), "sku": str(l, "sku"),
			"quantity": decimal(l["quantity"]), "price": shopifyAmount(obj(l, "originalUnitPriceSet")), "total": shopifyAmount(obj(l, "discountedTotalSet"))}).
			ref("product", str(obj(l, "product"), "id")))
	}
	return r
}
