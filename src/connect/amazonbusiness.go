package connect

import (
	"strings"
	"time"

	"golang.org/x/oauth2"
)

func init() {
	Register(&Connector{
		ID: "amazon_business", Name: "Amazon Business", Integration: true,
		Info: Info{By: "Amazon", Description: "Read the business's Amazon Business orders: every item, who bought it, what it cost and where it stands", Category: CatCommerce, Icon: IconCard,
			Scopes: []string{"Read the business's orders, their items and their buyers through Amazon Business Analytics"}},
		API:  "https://na.business-api.amazon.com",
		Pace: 500 * time.Millisecond,
		Endpoint: oauth2.Endpoint{AuthURL: "https://www.amazon.com/b2b/abws/oauth?applicationId={option/app}", TokenURL: "https://api.amazon.com/auth/o2/token",
			AuthStyle: oauth2.AuthStyleInParams},
		Options: "?app=<the app's applicationId in Solution Provider Portal, amzn1.sp.solution.…>; the client is the app's LWA credentials, and the app holds the Amazon Business Analytics role. North America's marketplaces",
		Sign:    map[string]string{"x-amz-access-token": "{token}"},
		Errors:  &Errors{At: "errors/0/message", Code: "errors/0/code"},
		Actions: map[string]*Action{
			"get": Call[abGet, any](Plan{
				Doc: "Read one of the Reporting API's GET endpoints, like { path: \"/reports/2025-06-09/orderReports\", " +
					"query: \"orderStartDate=2026-01-01T00:00:00Z&orderEndDate=2026-11-01T00:00:00Z\" }, orderLineItemReports with orderIds=<id>, " +
					"shipmentReports or shipmentLineItemReports: its answer, 100 at a time; pass nextPageToken=<its nextPageToken> for more. " +
					"A range spans at most 366 days and ends no later than now",
				Steps: []Step{{Only: Test{At: "path", Re: `^/reports/`, Off: true}, Fail: "Amazon Business reads only the Reporting API, under /reports/"},
					{Req: Req{URL: "{+path}?{+query|}"}}},
			}),
		},
		Sync: []*Sync{{Name: "orders", Gap: time.Second, Keep: true, Items: "ordersReport", Key: []Reads{{Of: "{orderMetadata/orderId}"}},
			Parts: &Parts{Span: &Span{Last: Shift{0, 0, -1}, First: Shift{-5, 0, 0}, Step: Shift{1, 0, 0}, Then: Shift{0, 0, -30}}, ID: "{start}",
				Vals: Obj{"start": "{start}", "after": "{after}"}},
			Req: Req{URL: "/reports/2025-06-09/orderReports",
				Query: Q{"orderStartDate": "{in/start}T00:00:00Z", "orderEndDate": "{in/after}T00:00:00Z", "nextPageToken": "{page}"}},
			Page: Cursor{At: "nextPageToken"},
			Hydrate: []Hydrate{Fetch{Par: 2, Soft: true, As: "lines", Req: Req{URL: "/reports/2025-06-09/orderLineItemReports",
				Query: Q{"orderIds": "{orderMetadata/orderId}", "orderStartDate": "{in/start}T00:00:00Z", "orderEndDate": "{in/after}T00:00:00Z"}}}},
			Doc: "Every order of the business's Amazon Business account, by its order id, with its buyer, group, purchase order number, status, " +
				"subtotal, tax, shipping, promotions and total, and each item with its ASIN, brand, condition, quantity, price, total and seller: " +
				"a year at a time from five years back the first time, then the last 30 days each sync, up to yesterday; one Amazon stops listing is kept.",
			To: Map{Table: "commerce_order",
				Cols: map[string]SQL{"number": "data#>>'{orderMetadata,orderId}'", "customer_name": "nullif(btrim(data#>>'{buyingCustomer,name}'), '')",
					"email": "archive.sync_email(btrim(data#>>'{buyingCustomer,email}'))",
					"total": abCharge("data", "NET_TOTAL"), "subtotal": abCharge("data", "SUBTOTAL"), "tax": abCharge("data", "TAX"),
					"shipping": abCharge("data", "SHIPPINGANDHANDLING"), "discounts": abCharge("data", "PROMOTION"),
					"fulfilled": "data->>'orderStatus'", "country": "data#>>'{orderMetadata,region}'", "placed_at": "archive.sync_at(data#>>'{orderMetadata,orderDate}')",
					"tags": "coalesce((SELECT jsonb_agg(t) FROM unnest(ARRAY[data#>>'{buyerGroup,name}']) t WHERE t <> ''), '[]'::jsonb)",
					"note": "CASE WHEN nullif(data->>'purchaseOrderNumber', '') IS NOT NULL THEN 'PO ' || (data->>'purchaseOrderNumber') END"},
				People: map[string]SQL{"customer": "btrim(data#>>'{buyingCustomer,email}')"},
				Children: []Kids{{Table: "commerce_line", From: "archive.sync_list(data#>'{lines,orderLineItemsReport}') l",
					Cols: map[string]SQL{"title": "l#>>'{productDetails,title}'", "sku": "l#>>'{productDetails,asin}'", "product_source": "l#>>'{productDetails,asin}'",
						"variant":  "nullif(concat_ws(', ', l#>>'{productDetails,brandName}', l#>>'{productDetails,productCondition}'), '')",
						"quantity": "archive.sync_num(l->>'quantity')", "price": "archive.sync_money(l#>>'{unitPrice,amount}', coalesce(l#>>'{unitPrice,currencyCode}', 'USD'))",
						"total": abCharge("l", "NET_TOTAL")}}}}}},
	})
}

// abCharge is the amount of one of an order's or a line's charges, like NET_TOTAL, matching SHIPPING_AND_HANDLING too, as a promotion's is negative.
func abCharge(of, kind string) SQL {
	return SQL("(SELECT archive.sync_money(abs((c#>>'{amount,amount}')::numeric)::text, coalesce(c#>>'{amount,currencyCode}', 'USD')) FROM archive.sync_list(" + of +
		"->'charges') c WHERE replace(c->>'type', '_', '') = '" + strings.ReplaceAll(kind, "_", "") + "' LIMIT 1)")
}

type abGet struct {
	Path  string `json:"path" jsonschema:"The endpoint, like /reports/2025-06-09/orderReports"`
	Query string `json:"query,omitempty" jsonschema:"Its query string, like orderStartDate=2026-01-01T00:00:00Z&orderEndDate=2026-11-01T00:00:00Z"`
}
