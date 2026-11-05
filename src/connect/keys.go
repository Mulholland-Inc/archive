package connect

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

const mintLasts = 50 * time.Minute

type Key struct {
	Name   string `json:"name"`
	Label  string `json:"label"`
	Help   string `json:"help,omitempty"`
	Link   string `json:"link,omitempty"`
	Secret bool   `json:"secret,omitempty"`
	Find   string `json:"-"`
	As     string `json:"-"`
}

type Link struct {
	Start Req
	Token Path
	Swap  Req
	Keep  map[string]Path
	Meta  []string
}

func (l *Link) check() error {
	if err := l.Start.check("start", "client", "option", "user", "hook", "back"); err != nil {
		return err
	}
	if l.Token == "" || len(l.Keep) == 0 {
		return errors.New("link: needs Token and Keep")
	}
	return l.Swap.check("swap", "client", "option", "key")
}

func (p *Provider) LinkToken(ctx context.Context, user, hook, back string) (string, error) {
	if p.Link == nil {
		return "", fmt.Errorf("%s has no widget to connect with", p.Name)
	}
	c := p.openKeys(ctx, nil, nil)
	defer c.Close()
	sc := c.scope()
	sc["user"], sc["hook"], sc["back"] = user, hook, back
	ans, err := c.req(p.Link.Start, sc)
	if err != nil {
		return "", err
	}
	tok := text(pick(ans, p.Link.Token))
	if tok == "" {
		return "", fmt.Errorf("%s answered without %s", p.Name, p.Link.Token)
	}
	return tok, nil
}

func (p *Provider) swap(ctx context.Context, keys map[string]string) (kept, meta map[string]string, err error) {
	c := p.openKeys(ctx, keys, nil)
	defer c.Close()
	ans, err := c.req(p.Link.Swap, c.scope())
	if err != nil {
		return nil, nil, err
	}
	kept, meta = map[string]string{}, map[string]string{}
	for name, at := range p.Link.Keep {
		if kept[name] = text(pick(ans, at)); kept[name] == "" {
			return nil, nil, fmt.Errorf("%s answered without %s", p.Name, at)
		}
		if slices.Contains(p.Link.Meta, name) {
			meta[name] = kept[name]
		}
	}
	return kept, meta, nil
}

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
	if p.Link != nil {
		var err error
		if kept, meta, err = p.swap(ctx, kept); err != nil {
			return nil, fmt.Errorf("%s could not be connected: %w", p.Name, err)
		}
	}
	open, err := kept, error(nil)
	if p.Trade != nil || p.Mint != nil {
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

func (p *Provider) trade(ctx context.Context, keys map[string]string) (map[string]string, time.Time, error) {
	c := &Client{ctx: ctx, p: p, http: client, keys: keys}
	sc := c.scope()
	if p.Mint != nil {
		if _, err := runSteps(c, p.Mint, sc); err != nil {
			return nil, time.Time{}, err
		}
		out := maps.Clone(keys)
		for k, v := range sc["key"].(map[string]any) {
			out[k] = text(v)
		}
		return out, time.Now().Add(mintLasts), nil
	}
	sc["basic"] = basic(keys["client_id"], keys["client_secret"])
	ans, err := c.req(*p.Trade, sc)
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

func keysFrom(s string) map[string]string {
	keys := map[string]string{}
	json.Unmarshal([]byte(s), &keys)
	return keys
}

func (p *Provider) openKeys(ctx context.Context, keys, meta map[string]string) *Client {
	c := &Client{ctx: ctx, p: p, keys: keys, Meta: meta}
	var rt http.RoundTripper = &keyTransport{c: c}
	if p.Pace > 0 {
		rt = &paced{base: rt, p: pacerOf(p.ID, p.Pace)}
	}
	c.http = &http.Client{Timeout: client.Timeout, Transport: rt}
	return c
}

func (c *Client) Key(name string) string { return c.keys[name] }

type keyTransport struct{ c *Client }

func (t *keyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	if t.c.p.Authorize != nil {
		t.c.p.Authorize(r, t.c)
	}
	return http.DefaultTransport.RoundTrip(r)
}
