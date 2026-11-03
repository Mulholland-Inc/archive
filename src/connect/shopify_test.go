package connect

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
)

// shopifyFake is a store whose app has scopes, and whose every connection lists two pages of nodes, 1 and 2
// then 3, by after and endCursor as the Admin API pages them. The queries it was asked are in asked.
func shopifyFake(t *testing.T, scopes []string, asked *[]string) *fake {
	return newFake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || !strings.HasPrefix(r.URL.Path, "/admin/api/") || !strings.HasSuffix(r.URL.Path, "/graphql.json") {
			t.Errorf("asked %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("X-Shopify-Access-Token") != "shpat_test" {
			t.Errorf("asked with token %q", r.Header.Get("X-Shopify-Access-Token"))
		}
		var in struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		json.NewDecoder(r.Body).Decode(&in)
		if strings.Contains(in.Query, "currentAppInstallation") {
			var hs []any
			for _, s := range scopes {
				hs = append(hs, map[string]any{"handle": s})
			}
			writeJSON(w, map[string]any{"data": map[string]any{"currentAppInstallation": map[string]any{"accessScopes": hs}}})
			return
		}
		field, _, _ := strings.Cut(strings.TrimSpace(strings.SplitN(in.Query, "{", 2)[1]), "(")
		if q, ok := in.Variables["query"].(string); ok {
			*asked = append(*asked, q)
		} else {
			*asked = append(*asked, "")
		}
		node := func(n string) map[string]any {
			return map[string]any{"id": "gid://shopify/" + field + "/" + n, "updatedAt": "2026-10-01T10:00:00Z"}
		}
		nodes, info := []any{node("1"), node("2")}, map[string]any{"hasNextPage": true, "endCursor": "c1"}
		if in.Variables["after"] == "c1" {
			nodes, info = []any{node("3")}, map[string]any{"hasNextPage": false, "endCursor": "c2"}
		}
		writeJSON(w, map[string]any{"data": map[string]any{field: map[string]any{"nodes": nodes, "pageInfo": info}}})
	})
}

func shopifyOpen(t *testing.T, f *fake) *Client {
	return f.open(t, "shopify", "", map[string]string{"shop": f.host(), "token": "shpat_test"}, nil)
}

// A store's products are listed whole, a page at a time by endCursor, each page Full, and the cursor after
// says when the listing began.
func TestShopifyProductsListingIsFull(t *testing.T) {
	var asked []string
	f := shopifyFake(t, nil, &asked)
	pages, cursor := pull(t, shopifyOpen(t, f), stream(t, "shopify", "products"), "")
	if got, want := ids(pages...), []string{"gid://shopify/products/1", "gid://shopify/products/2", "gid://shopify/products/3"}; !slices.Equal(got, want) {
		t.Errorf("listed %v, want %v", got, want)
	}
	for i, p := range pages {
		if !p.Full {
			t.Errorf("page %d of a products listing is not Full", i+1)
		}
	}
	if !slices.Equal(asked, []string{"", ""}) {
		t.Errorf("a listing searched %q", asked)
	}
	if !strings.Contains(cursor, `"listed"`) || strings.Contains(cursor, `"after"`) {
		t.Errorf("after the listing the cursor is %s", cursor)
	}
}

// An app with read_all_orders lists every order, and its listing is Full.
func TestShopifyOrdersWithAllOrdersIsFull(t *testing.T) {
	var asked []string
	f := shopifyFake(t, []string{"read_orders", "read_all_orders"}, &asked)
	pages, _ := pull(t, shopifyOpen(t, f), stream(t, "shopify", "orders"), "")
	if len(ids(pages...)) != 3 {
		t.Errorf("listed %v, want 3 orders", ids(pages...))
	}
	for i, p := range pages {
		if !p.Full {
			t.Errorf("page %d of an all-orders listing is not Full", i+1)
		}
	}
}

// Without read_all_orders Shopify lists only the last 60 days of orders, so that listing is not Full and
// deletes none of the older ones. Before #290 it was Full, and every order older than 60 days was deleted.
func TestShopifyOrdersWithoutAllOrdersDeletesNothing(t *testing.T) {
	var asked []string
	f := shopifyFake(t, []string{"read_orders", "read_products"}, &asked)
	pages, _ := pull(t, shopifyOpen(t, f), stream(t, "shopify", "orders"), "")
	if len(ids(pages...)) != 3 {
		t.Errorf("listed %v, want 3 orders", ids(pages...))
	}
	for i, p := range pages {
		if p.Full {
			t.Errorf("page %d of a 60-day orders listing is Full, so it deletes the older orders", i+1)
		}
	}
}

// Between listings a sync searches for what was updated since the one before, which is not Full, and keeps
// when the last listing was.
func TestShopifyDeltaSearchesSince(t *testing.T) {
	var asked []string
	f := shopifyFake(t, nil, &asked)
	since := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	listed := time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339)
	pages, cursor := pull(t, shopifyOpen(t, f), stream(t, "shopify", "products"), cursorFor(shopifyCursor{Since: since, Listed: listed}))
	if len(ids(pages...)) != 3 {
		t.Errorf("found %v, want 3 products", ids(pages...))
	}
	for i, p := range pages {
		if p.Full {
			t.Errorf("page %d of a delta is Full", i+1)
		}
	}
	want := "updated_at:>'" + since + "'"
	if !slices.Equal(asked, []string{want, want}) {
		t.Errorf("searched %q, want %q on each page", asked, want)
	}
	if !strings.Contains(cursor, `"listed":"`+listed+`"`) {
		t.Errorf("after the delta the cursor is %s, want listed kept at %s", cursor, listed)
	}
}
