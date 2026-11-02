package connect

import "time"

// Stripe, through its hosted MCP server: customers, payments, invoices, subscriptions and the rest of an
// account, as the person who connects allows. Its authorization server registers any client, but only as a
// public one, so Archive's client there has no secret and proves itself with PKCE alone. An account's admins
// can turn MCP access off. get_stripe_account_info names the account.
//
// Its server reads the API with one tool, stripe_api_read, whose input Stripe does not document: the streams
// call it as Stripe's earlier stripe_api_execute was, with the method's operation id and its parameters, and
// are skipped when the schema the server lists has no stripe_api_operation_id. Stripe's objects have no time
// of their last change to list by, so each is listed whole every 6 hours, 100 a page, and one gone is deleted.
func init() {
	Register(&Connector{ID: "stripe", Name: "Stripe", MCP: "https://mcp.stripe.com", Account: mcpAccount("get_stripe_account_info"),
		Streams: []*Stream{
			stripeList("customers", "GetCustomers", nil),
			stripeList("products", "GetProducts", nil),
			stripeList("prices", "GetPrices", nil),
			stripeList("subscriptions", "GetSubscriptions", map[string]any{"status": "all"}),
			stripeList("invoices", "GetInvoices", nil),
		}})
}

func stripeList(name, op string, params map[string]any) *Stream {
	if params == nil {
		params = map[string]any{}
	}
	l := &mcpList{tool: "stripe_api_read", args: map[string]any{"stripe_api_operation_id": op, "parameters": params}, in: "parameters",
		limit: "limit", max: 100, after: "starting_after", items: "data", relist: 6 * time.Hour}
	return l.stream(name, "Every one of the account's "+name+", as Stripe's API lists them with "+op+", by id, 100 a page, listed whole every 6 hours.", 6*time.Hour)
}
