package connect

// Stripe, through its hosted MCP server: customers, payments, invoices, subscriptions and the rest of an
// account, as the person who connects allows. Its authorization server registers any client, but only as a
// public one, so Archive's client there has no secret and proves itself with PKCE alone. An account's admins
// can turn MCP access off. get_stripe_account_info names the account.
func init() {
	Register(&Connector{ID: "stripe", Name: "Stripe", MCP: "https://mcp.stripe.com", Account: mcpAccount("get_stripe_account_info")})
}
