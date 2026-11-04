package connect

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Key connectors are the services that let an app in with what a person pastes, not with a sign-in: an API key
// and the token of an app made in their account, like Karbon's, or an app's token and the address of the store
// it is for, like Shopify's. Its Keys say what to ask for; connecting checks them by asking the service whose
// account they are (Account), and keeps them as an OAuth connector keeps its refresh token, sealed with the
// workspace key, so they are never shown again. A key that is not Secret, like a store's address, is kept as the
// connection's Meta too. A key never refreshes and is never revoked by Archive: the person ends it where they
// made it, and connects again with a new one. Keys that are an OAuth client of the account's own, like Vanta's,
// are traded for an access token whenever the last has run out (Trade); the token is never kept.

// Key is something a key connector asks for.
type Key struct {
	// Name is its name in the connector's code and in the API, Label what the form says, and Help where to find
	// it, for the form.
	Name  string `json:"name"`
	Label string `json:"label"`
	Help  string `json:"help,omitempty"`
	// Secret says it is a secret: the form hides it, and it is kept only sealed.
	Secret bool `json:"secret,omitempty"`
}

// keyProvider is the key connector c, available unless it names an Env this server does not set.
func keyProvider(c *Connector, getenv func(string) string) (*Provider, bool) {
	p := &Provider{Connector: c}
	if c.Env == "" {
		return p, true
	}
	raw := strings.TrimSpace(getenv(c.Env))
	if raw == "" {
		return nil, false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return nil, false
	}
	p.clientID, p.opts = u.User.Username(), u.Query()
	p.secret, _ = u.User.Password()
	p.base = strings.TrimRight(u.Scheme+"://"+u.Host+u.Path, "/")
	return p, true
}

// KeyGrant is the account the keys are for, to keep, once the service says whose they are. Every key is
// needed, cut of the space around it.
func (p *Provider) KeyGrant(ctx context.Context, keys map[string]string) (*Grant, error) {
	if len(p.Keys) == 0 {
		return nil, fmt.Errorf("%s connects by signing in, not with keys", p.Name)
	}
	kept, meta := map[string]string{}, map[string]string{}
	for _, k := range p.Keys {
		v := strings.TrimSpace(keys[k.Name])
		if v == "" {
			return nil, fmt.Errorf("%s needs %s", p.Name, k.Label)
		}
		kept[k.Name] = v
		if !k.Secret {
			meta[k.Name] = v
		}
	}
	// Keys that are a client are tried by trading them, which refuses them with a 400 or 401.
	open, err := kept, error(nil)
	if p.Trade != nil {
		if open, _, err = p.trade(ctx, kept); err != nil {
			var r *Refusal
			if errors.As(err, &r) && r.Status/100 == 4 {
				return nil, fmt.Errorf("%s did not take those keys: %s", p.Name, r.Message)
			}
			return nil, fmt.Errorf("%s could not be connected with those keys: %w", p.Name, err)
		}
	}
	c := p.openKeys(ctx, open, meta)
	defer c.Close()
	account := p.Name
	if p.Account != nil {
		a, err := p.Account(c)
		if err != nil {
			var r *Refusal
			if errors.As(err, &r) && (r.Status == http.StatusUnauthorized || r.Status == http.StatusForbidden) {
				return nil, fmt.Errorf("%s did not take those keys: %s", p.Name, r.Message)
			}
			return nil, fmt.Errorf("%s could not be connected with those keys: %w", p.Name, err)
		}
		account = a
	}
	b, _ := json.Marshal(kept)
	return &Grant{Account: account, Refresh: string(b), Meta: meta}, nil
}

// trade is keys with the access token their Trade answers with, as access, and when that token ends: zero when
// it does not say. The request goes unsigned, with the keys as {key/...}.
func (p *Provider) trade(ctx context.Context, keys map[string]string) (map[string]string, time.Time, error) {
	c := &Client{ctx: ctx, p: p, http: client, keys: keys}
	ans, err := c.req(*p.Trade, c.scope())
	if err != nil {
		return nil, time.Time{}, err
	}
	out := maps.Clone(keys)
	if out["access"] = text(pick(ans, "access_token")); out["access"] == "" {
		return nil, time.Time{}, fmt.Errorf("%s gave no access token for those keys", p.Name)
	}
	var end time.Time
	if s := atoi(text(pick(ans, "expires_in"))); s > 0 {
		end = time.Now().Add(time.Duration(s) * time.Second)
	}
	return out, end, nil
}

// keysFrom are the keys a grant kept, as KeyGrant wrote them.
func keysFrom(s string) map[string]string {
	keys := map[string]string{}
	json.Unmarshal([]byte(s), &keys)
	return keys
}

// openKeys is a Client of the account keys are for, whose requests Authorize signs.
func (p *Provider) openKeys(ctx context.Context, keys, meta map[string]string) *Client {
	c := &Client{ctx: ctx, p: p, keys: keys, Meta: meta}
	var rt http.RoundTripper = &keyTransport{c: c}
	if p.Pace > 0 {
		rt = &paced{base: rt, p: pacerOf(p.ID, p.Pace)}
	}
	c.http = &http.Client{Timeout: client.Timeout, Transport: rt}
	return c
}

// Key is the key name of a key connector's account, like its store's address.
func (c *Client) Key(name string) string { return c.keys[name] }

// keyTransport signs each request with its connector's Authorize.
type keyTransport struct{ c *Client }

func (t *keyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	if t.c.p.Authorize != nil {
		t.c.p.Authorize(r, t.c)
	}
	return http.DefaultTransport.RoundTrip(r)
}
