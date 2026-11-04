package connect

// Stripe, through its hosted MCP server: customers, payments, invoices, subscriptions and the rest of an
// account, as the person who connects allows. Its authorization server registers any client, but only as a
// public one, so Archive's client there has no secret and proves itself with PKCE alone. An account's admins
// can turn MCP access off. get_stripe_account_info names the account.
//
// It syncs nothing until its customers, payments and invoices have tables in archive.sql; its actions are the
// server's tools.
func init() {
	Register(&Connector{ID: "stripe", Name: "Stripe", Integration: true, MCP: "https://mcp.stripe.com", Who: &Who{Tool: "get_stripe_account_info"},
		Info: Info{By: "Stripe", Description: "Look up customers, payments, invoices and subscriptions", Category: CatFinance, Icon: IconCard, Scopes: []string{"Use your Stripe account as you allow"}}})
}
