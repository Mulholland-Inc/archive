package connect

import (
	"bytes"
	"cmp"
	"context"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
	"golang.org/x/oauth2"
)

type Connector struct {
	ID, Name    string
	Integration bool
	Pace        time.Duration
	Env, Host   string
	Hosts       []string
	Endpoint    oauth2.Endpoint
	Revoke      string
	RevokeWith  func(rv *Revocation) error
	Lasts       bool
	Scopes      []string
	Optional    []string
	Extra       map[string][]string
	Comma       bool
	Params      map[string]string
	Trades      map[string]string
	Ask         *Key
	Signed      string
	Was         string
	Keep        []string
	Allow       map[string][]string
	TokenType   string
	MCP         string
	Account     func(c *Client) (string, error)
	Actions     map[string]*Action
	Streams     []*Stream
	Push        *Push
	Keys        []Key
	Authorize   func(req *http.Request, c *Client)
	Trade       *Req
	Mint        []Step
	Link        *Link
	Info        Info
	Options     string
	API         string
	Sign        map[string]string
	Basic       [2]string
	Pin         Q
	Who         *Who
	RevokeBy    *Req
	Listen      *Listen
	Sync        []*Sync
	Let         []Step
	Errors      *Errors
	Answer      *Codec
	Busy        []string
	Trace       string
}

type Errors struct {
	At, Code Path
	Status   int
	Busy     []string
}

func (c *Connector) MCPEnv() string { return "ARCHIVE_MCP_" + strings.ToUpper(c.ID) }

var registry = map[string]*Connector{}

func Register(c *Connector) {
	if registry[c.ID] != nil {
		panic("connect: two connectors are " + c.ID)
	}
	if err := c.compile(); err != nil {
		panic("connect: " + c.ID + ": " + err.Error())
	}
	if len(c.Sync) == 0 {
		panic("connect: " + c.ID + " has no stream")
	}
	if len(c.Actions) == 0 && c.MCP == "" {
		panic("connect: " + c.ID + " has no action")
	}
	registry[c.ID] = c
}

func Get(id string) *Connector { return registry[id] }

func All() []*Connector {
	out := make([]*Connector, 0, len(registry))
	for _, c := range registry {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

type Provider struct {
	*Connector
	clientID, secret, base string
	opts                   url.Values
	endpoint               oauth2.Endpoint
	revoke                 string
	scopes                 []string
	server, resource       string
	pushTo                 string
}

type Providers map[string]*Provider

func Load(getenv func(string) string) (Providers, error) {
	out := Providers{}
	for _, c := range All() {
		if len(c.Keys) > 0 {
			if p, ok := keyProvider(c, getenv); ok {
				out[c.ID] = p
			}
			continue
		}
		if c.MCP != "" {
			out[c.ID] = &Provider{Connector: c, server: cmp.Or(strings.TrimSpace(getenv(c.MCPEnv())), c.MCP)}
			continue
		}
		raw := strings.TrimSpace(getenv(c.Env))
		if raw == "" {
			if c.Was != "" {
				out[c.ID] = &Provider{Connector: c}
			}
			continue
		}
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" || u.Scheme != "https" && u.Scheme != "http" {
			return nil, fmt.Errorf("%s: want %s, with the client id and secret URL-escaped", c.Env, strings.Replace(c.Host, "://", "://<client id>:<client secret>@", 1))
		}
		secret, _ := u.User.Password()
		if id := u.User.Username(); id != "" && secret != "" {
			p := &Provider{Connector: c, clientID: id, secret: secret, base: strings.TrimRight(u.Scheme+"://"+u.Host+u.Path, "/"), opts: u.Query(), scopes: c.Scopes}
			if u.Query().Get("push") != "" {
				p.scopes = slices.Concat(c.Scopes, c.Optional)
			}
			for _, opt := range slices.Sorted(maps.Keys(c.Extra)) {
				if u.Query().Get(opt) != "" {
					p.scopes = slices.Concat(p.scopes, c.Extra[opt])
				}
			}
			p.endpoint = oauth2.Endpoint{AuthURL: p.at(c.Endpoint.AuthURL), TokenURL: p.at(c.Endpoint.TokenURL), AuthStyle: c.Endpoint.AuthStyle}
			if c.Push != nil && c.Push.Env != "" {
				p.pushTo = strings.TrimSpace(getenv(c.Push.Env))
			}
			if c.Revoke != "" {
				p.revoke = p.at(c.Revoke)
			}
			out[c.ID] = p
		}
	}
	return out, nil
}

func (ps Providers) Get(id string) (*Provider, error) {
	if p := ps[id]; p != nil {
		return p, nil
	}
	if c := registry[id]; c != nil {
		return nil, fmt.Errorf("%s is not set up on this server; it needs %s", c.Name, c.Env)
	}
	return nil, fmt.Errorf("there is no connector %q", id)
}

var client = &http.Client{Timeout: 30 * time.Second}

func (p *Provider) at(u string) string {
	pu, err := url.Parse(u)
	if p.base == "" || p.base == p.Host || err != nil || len(p.Hosts) > 0 && !slices.Contains(p.Hosts, pu.Scheme+"://"+pu.Host) {
		return u
	}
	return p.base + pu.RequestURI()
}

func (p *Provider) config(redirect string) *oauth2.Config {
	return &oauth2.Config{ClientID: p.clientID, ClientSecret: p.secret, RedirectURL: redirect, Scopes: p.scopes, Endpoint: p.endpoint}
}

func (p *Provider) configOf(redirect string, meta map[string]string) (*oauth2.Config, error) {
	cfg := p.config(redirect)
	if !strings.Contains(cfg.Endpoint.TokenURL, "{") {
		return cfg, nil
	}
	u, err := expand(cfg.Endpoint.TokenURL, p.metaScope(meta))
	if err != nil {
		return nil, fmt.Errorf("%s: its token endpoint: %w", p.Name, err)
	}
	cfg.Endpoint.TokenURL = u
	return cfg, nil
}

func (p *Provider) metaScope(meta map[string]string) map[string]any {
	m, opts := map[string]any{}, map[string]any{}
	for k, v := range meta {
		m[k] = v
	}
	for k := range p.opts {
		opts[k] = p.opts.Get(k)
	}
	return map[string]any{"meta": m, "option": opts}
}

func (p *Provider) AuthURL(redirect, state, verifier string, meta map[string]string) (string, error) {
	opts := []oauth2.AuthCodeOption{oauth2.S256ChallengeOption(verifier)}
	for k, v := range p.Params {
		opts = append(opts, oauth2.SetAuthURLParam(k, v))
	}
	if p.resource != "" {
		opts = append(opts, oauth2.SetAuthURLParam("resource", p.resource))
	}
	if p.Comma {
		opts = append(opts, oauth2.SetAuthURLParam("scope", strings.Join(p.scopes, ",")))
	}
	cfg := p.config(redirect)
	if strings.Contains(cfg.Endpoint.AuthURL, "{") {
		u, err := expand(cfg.Endpoint.AuthURL, p.metaScope(meta))
		if err != nil {
			return "", fmt.Errorf("%s: where it signs in: %w", p.Name, err)
		}
		cfg.Endpoint.AuthURL = u
	}
	return cfg.AuthCodeURL(state, opts...), nil
}

func (p *Provider) AskOf(v string) (map[string]string, error) {
	k := p.Ask
	if k == nil {
		return nil, nil
	}
	v = strings.ToLower(strings.TrimSpace(v))
	if k.Find != "" {
		f := reOf(k.Find).FindStringSubmatch(v)
		if f == nil {
			return nil, fmt.Errorf("%s needs %s: %s", p.Name, k.Label, k.Help)
		}
		v = f[1]
	}
	if k.As != "" {
		x, _ := fillStr(k.As, map[string]any{"it": v})
		v = text(x)
	}
	if v == "" {
		return nil, fmt.Errorf("%s needs %s", p.Name, k.Label)
	}
	return map[string]string{k.Name: v}, nil
}

func (p *Provider) signed(q url.Values) bool {
	got := q.Get(p.Signed)
	for _, esc := range []func(string) string{func(s string) string { return s }, url.QueryEscape} {
		var pairs []string
		for _, k := range slices.Sorted(maps.Keys(q)) {
			if k != p.Signed {
				pairs = append(pairs, esc(k)+"="+esc(strings.Join(q[k], ",")))
			}
		}
		m := hmac.New(sha256.New, []byte(p.secret))
		m.Write([]byte(strings.Join(pairs, "&")))
		if same(got, hex.EncodeToString(m.Sum(nil))) {
			return true
		}
	}
	return false
}

type Grant struct {
	Account, Refresh string
	Scopes           []string
	Meta             map[string]string
}

func (p *Provider) Exchange(ctx context.Context, redirect string, q url.Values, verifier string, asked map[string]string) (*Grant, error) {
	if e := q.Get("error"); e != "" {
		return nil, fmt.Errorf("%s was not connected: %s", p.Name, cmp.Or(q.Get("error_description"), e))
	}
	if p.Signed != "" && !p.signed(q) {
		return nil, fmt.Errorf("%s was not connected: it came back without %s's signature", p.Name, p.Name)
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, client)
	opts := []oauth2.AuthCodeOption{oauth2.VerifierOption(verifier)}
	if p.resource != "" {
		opts = append(opts, oauth2.SetAuthURLParam("resource", p.resource))
	}
	for k, v := range p.Trades {
		opts = append(opts, oauth2.SetAuthURLParam(k, v))
	}
	meta := map[string]string{}
	if p.Ask != nil {
		k := p.Ask.Name
		if asked[k] == "" || (p.Signed != "" || q.Has(k)) && !strings.EqualFold(q.Get(k), asked[k]) {
			return nil, fmt.Errorf("%s was not connected: it came back for %s %q, not %q", p.Name, k, q.Get(k), asked[k])
		}
		meta[k] = asked[k]
	}
	for _, k := range p.Keep {
		if v := q.Get(k); v != "" {
			meta[k] = v
		}
		if vals, ok := p.Allow[k]; ok && meta[k] != "" && !slices.Contains(vals, meta[k]) {
			return nil, fmt.Errorf("%s was not connected: it came back with %s %s, which Archive does not know", p.Name, k, meta[k])
		}
	}
	cfg, err := p.configOf(redirect, meta)
	if err != nil {
		return nil, err
	}
	tok, err := cfg.Exchange(ctx, q.Get("code"), opts...)
	if err != nil {
		return nil, fmt.Errorf("%s was not connected: %s", p.Name, Reason(err))
	}
	for _, k := range p.Keep {
		extra, _ := tok.Extra(k).(string)
		if meta[k] = cmp.Or(meta[k], extra); meta[k] == "" {
			return nil, fmt.Errorf("%s was not connected: it came back without %s", p.Name, k)
		}
	}
	g := &Grant{Refresh: tok.RefreshToken, Scopes: p.scopes, Meta: meta}
	if p.Lasts {
		g.Refresh = cmp.Or(g.Refresh, tok.AccessToken)
	}
	if s, _ := tok.Extra("scope").(string); s != "" {
		g.Scopes = strings.FieldsFunc(s, func(r rune) bool { return r == ',' || unicode.IsSpace(r) })
	}
	switch {
	case g.Refresh == "":
		return nil, fmt.Errorf("%s sent no refresh token, so Archive could not keep reading it; connect it again", p.Name)
	case p.MCP == "" && slices.ContainsFunc(p.Scopes, func(s string) bool { return !granted(g.Scopes, s) }):
		return nil, fmt.Errorf("%s was connected without all the access Archive asked for; connect it again and allow it all", p.Name)
	}
	c := p.Open(ctx, tok, g.Meta)
	defer c.Close()
	if p.Account != nil {
		g.Account, err = p.Account(c)
	}
	if err == nil && g.Account == "" && p.MCP == "" {
		err = errors.New("it named no account")
	}
	switch {
	case err != nil && p.MCP == "":
		return nil, fmt.Errorf("%s did not say whose account it is: %v", p.Name, err)
	case err != nil:
		fmt.Fprintf(os.Stderr, "archive: %s did not say whose account it is, so it is named %s: %v\n", p.Name, p.Name, err)
		g.Account = p.Name
	case g.Account == "":
		g.Account = p.Name
	}
	return g, nil
}

func granted(scopes []string, want string) bool {
	return want == "offline_access" || strings.HasPrefix(want, "launch/") || slices.ContainsFunc(scopes, func(s string) bool {
		return strings.EqualFold(s, want) || strings.HasSuffix(strings.ToLower(s), "/"+strings.ToLower(want))
	})
}

func (p *Provider) Refresh(ctx context.Context, rt string, meta map[string]string) (*oauth2.Token, error) {
	if len(p.Keys) > 0 && p.Trade == nil && p.Mint == nil || p.Lasts || p.was(rt) != nil {
		return &oauth2.Token{AccessToken: rt, RefreshToken: rt}, nil
	}
	if len(p.Keys) > 0 {
		keys, end, err := p.trade(ctx, keysFrom(rt))
		if err != nil {
			return nil, err
		}
		b, _ := json.Marshal(keys)
		return &oauth2.Token{AccessToken: string(b), RefreshToken: rt, Expiry: end}, nil
	}
	cfg, err := p.configOf("", meta)
	if err != nil {
		return nil, err
	}
	tok, err := cfg.TokenSource(context.WithValue(ctx, oauth2.HTTPClient, client), &oauth2.Token{RefreshToken: rt}).Token()
	if err != nil {
		return nil, err
	}
	tok.RefreshToken = cmp.Or(tok.RefreshToken, rt)
	return tok, nil
}

func (p *Provider) RevokeToken(ctx context.Context, rt string, meta map[string]string) error {
	rv := &Revocation{Refresh: rt, Meta: meta, ctx: ctx, p: p}
	switch {
	case p.was(rt) != nil:
		return nil
	case p.RevokeWith != nil:
		return rv.p.RevokeWith(rv)
	case len(p.Keys) > 0:
		return nil
	case p.revoke == "":
		return nil
	}

	form, hdr := url.Values{"token": {rt}, "token_type_hint": {"refresh_token"}}, http.Header{}
	switch {
	case p.MCP != "" && p.endpoint.AuthStyle == oauth2.AuthStyleInHeader:
		hdr.Set("Authorization", basic(p.clientID, p.secret))
	case p.MCP != "":
		form.Set("client_id", p.clientID)
		if p.secret != "" {
			form.Set("client_secret", p.secret)
		}
	default:
		form.Del("token_type_hint")
	}
	return rv.send("POST", p.revoke, hdr, form)
}

func basic(id, secret string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(url.QueryEscape(id)+":"+url.QueryEscape(secret)))
}

type Revocation struct {
	Refresh string
	Meta    map[string]string
	ctx     context.Context
	p       *Provider
}

func (rv *Revocation) Client() (id, secret string) { return rv.p.clientID, rv.p.secret }

func (rv *Revocation) Access() (string, error) {
	tok, err := rv.p.Refresh(rv.ctx, rv.Refresh, rv.Meta)
	if err != nil {
		return "", err
	}
	return tok.AccessToken, nil
}

func (rv *Revocation) Form(u string, hdr http.Header, v url.Values) error {
	return rv.send("POST", rv.p.at(u), hdr, v)
}

func (rv *Revocation) Send(method, u string, hdr http.Header, body any) error {
	return rv.send(method, rv.p.at(u), hdr, body)
}

func (rv *Revocation) send(method, u string, hdr http.Header, body any) error {
	req, err := request(rv.ctx, method, u, body)
	if err != nil {
		return err
	}
	for k, v := range hdr {
		req.Header[k] = v
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return refusal(rv.p, resp)
	}
	return nil
}

func Reason(err error) string {
	var re *oauth2.RetrieveError
	if errors.As(err, &re) && re.ErrorCode != "" {
		return strings.TrimSuffix(re.ErrorCode+": "+re.ErrorDescription, ": ")
	}
	return err.Error()
}

type State struct {
	Provider  string            `json:"p"`
	Workspace string            `json:"w"`
	User      uuid.UUID         `json:"u"`
	Member    uuid.UUID         `json:"m"`
	Verifier  string            `json:"v"`
	Exp       int64             `json:"x"`
	Meta      map[string]string `json:"e,omitempty"`
}

const TTL = 10 * time.Minute

func SealState(aead cipher.AEAD, st State) string {
	b, _ := json.Marshal(st)
	return base64.RawURLEncoding.EncodeToString(seal(aead, b, "connect"))
}

func OpenState(aead cipher.AEAD, sealed string) (State, bool) {
	var st State
	ct, err := base64.RawURLEncoding.DecodeString(sealed)
	if err != nil {
		return st, false
	}
	b, err := open(aead, ct, "connect")
	return st, err == nil && json.Unmarshal(b, &st) == nil && time.Now().Unix() <= st.Exp
}

func SealToken(aead cipher.AEAD, id uuid.UUID, token string) []byte {
	return seal(aead, []byte(token), "connection "+id.String())
}

func OpenToken(aead cipher.AEAD, id uuid.UUID, sealed []byte) (string, error) {
	b, err := open(aead, sealed, "connection "+id.String())
	if err != nil {
		return "", fmt.Errorf("connection %s: its token does not open: %w", id, err)
	}
	return string(b), nil
}

func seal(aead cipher.AEAD, plain []byte, purpose string) []byte {
	nonce := make([]byte, aead.NonceSize())
	rand.Read(nonce)
	return aead.Seal(nonce, nonce, plain, []byte(purpose))
}

func open(aead cipher.AEAD, ct []byte, purpose string) ([]byte, error) {
	n := aead.NonceSize()
	if len(ct) < n {
		return nil, errors.New("too short")
	}
	return aead.Open(nil, ct[:n], ct[n:], []byte(purpose))
}

const maxBody = 5 << 20

type Client struct {
	ctx     context.Context
	p       *Provider
	http    *http.Client
	tok     *oauth2.Token
	Meta    map[string]string
	sess    *mcp.ClientSession
	tools   []*mcp.Tool
	keys    map[string]string
	apiOnce sync.Once
	apiURL  string
	apiErr  error
	mcpSeen *seen
	Files   Yours
	Store   func(name string, b []byte) (string, error)
}

func (p *Provider) Open(ctx context.Context, tok *oauth2.Token, meta map[string]string) *Client {
	if len(p.Keys) > 0 {
		return p.openKeys(ctx, keysFrom(tok.AccessToken), meta)
	}
	if w := p.was(tok.AccessToken); w != nil {
		return w.openKeys(ctx, keysFrom(tok.AccessToken), meta)
	}
	if p.TokenType != "" {
		t := *tok
		t.TokenType = p.TokenType
		tok = &t
	}
	c := &Client{ctx: ctx, p: p, tok: tok, Meta: meta}
	var rt http.RoundTripper = &oauth2.Transport{Source: oauth2.StaticTokenSource(tok)}
	if p.Authorize != nil {
		rt = &keyTransport{c: c}
	}
	if p.Pace > 0 {
		rt = &paced{base: rt, p: pacerOf(p.ID, p.Pace)}
	}
	c.http = &http.Client{Timeout: client.Timeout, Transport: rt}
	return c
}

func (p *Provider) was(t string) *Provider {
	if p.Was == "" || !strings.HasPrefix(t, "{") || registry[p.Was] == nil {
		return nil
	}
	return &Provider{Connector: registry[p.Was]}
}

func (c *Client) Context() context.Context { return c.ctx }

func (c *Client) Option(k string) string { return c.p.opts.Get(k) }

func (c *Client) Token() string {
	if c.tok == nil {
		return ""
	}
	return c.tok.AccessToken
}

func (c *Client) Credentials() (id, secret string) { return c.p.clientID, c.p.secret }

func (c *Client) Get(u string, v any) error { return c.Do("GET", u, nil, v) }

func (c *Client) Do(method, u string, body, v any) error { return c.Send(method, u, nil, body, v) }

func (c *Client) Send(method, u string, hdr http.Header, body, v any) error {
	req, err := request(c.ctx, method, c.p.at(u), body)
	if err != nil {
		return err
	}
	for k, vs := range hdr {
		req.Header[k] = vs
	}
	resp, err := c.http.Do(req)
	if err != nil {
		var re *oauth2.RetrieveError
		if errors.As(err, &re) {
			return err
		}
		return unreached{fmt.Errorf("%s could not be reached: %w", c.p.Name, errors.Unwrap(err))}
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return refusal(c.p, resp)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	switch {
	case err != nil:
		return err
	case len(b) > maxBody:
		return fmt.Errorf("%s answered with over %d MB; ask for less", c.p.Name, maxBody>>20)
	}
	switch v := v.(type) {
	case nil:
		return nil
	case *string:
		*v = string(b)
		return nil
	case *[]byte:
		*v = b
		return nil
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s answered with what is not JSON: %v", c.p.Name, err)
	}
	return nil
}

func request(ctx context.Context, method, u string, body any) (*http.Request, error) {
	var r io.Reader
	ct := ""
	switch b := body.(type) {
	case nil:
	case url.Values:
		r, ct = strings.NewReader(b.Encode()), "application/x-www-form-urlencoded"
	case string:
		r = strings.NewReader(b)
	default:
		j, err := json.Marshal(b)
		if err != nil {
			return nil, err
		}
		r, ct = bytes.NewReader(j), "application/json"
	}
	req, err := http.NewRequestWithContext(ctx, method, u, r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	return req, nil
}

type Refusal struct {
	Provider string
	Status   int
	Message  string
	Busy     bool
	Retry    time.Duration
}

func (r *Refusal) Error() string {
	if r.Status == 0 {
		return r.Provider + " said: " + r.Message
	}
	s := fmt.Sprintf("%s refused: %d %s", r.Provider, r.Status, http.StatusText(r.Status))
	if r.Message != "" {
		s += ": " + r.Message
	}
	return s
}

func refusal(p *Provider, resp *http.Response) *Refusal {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	msg := reasonIn(b)
	f := &Refusal{Provider: p.Name, Status: resp.StatusCode, Message: msg, Retry: retryAfter(resp.Header), Busy: busyStatus(resp.StatusCode) || p.busySays(msg)}
	if p.Trace != "" {
		if id := resp.Header.Get(p.Trace); id != "" {
			f.Message = strings.TrimPrefix(msg+" ("+p.Trace+" "+id+")", " ")
		}
	}
	return f
}

func busyStatus(s int) bool {
	return s == http.StatusTooManyRequests || s == http.StatusBadGateway || s == http.StatusServiceUnavailable || s == http.StatusGatewayTimeout
}

func (c *Connector) busySays(msg string) bool {
	return slices.ContainsFunc(c.Busy, func(s string) bool { return strings.Contains(msg, s) })
}

func retryAfter(h http.Header) time.Duration {
	v := h.Get("Retry-After")
	if v == "" {
		return 0
	}
	if s, err := strconv.Atoi(v); err == nil && s > 0 {
		return time.Duration(s) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		return max(time.Until(t), 0)
	}
	return 0
}

func reasonIn(b []byte) string {
	var v any
	if json.Unmarshal(b, &v) != nil {
		s := strings.Join(strings.Fields(string(b)), " ")
		if len(s) > 300 {
			s = s[:300] + "..."
		}
		return s
	}
	for _, k := range []string{"message", "error_message", "error_description", "error_summary", "Detail", "Message", "detail", "error"} {
		if s := find(v, k); s != "" {
			return s
		}
	}
	return ""
}

func find(v any, key string) string {
	switch v := v.(type) {
	case map[string]any:
		if s, ok := v[key].(string); ok && s != "" {
			return s
		}
		for _, e := range v {
			if s := find(e, key); s != "" {
				return s
			}
		}
	case []any:
		for _, e := range v {
			if s := find(e, key); s != "" {
				return s
			}
		}
	}
	return ""
}

type Action struct {
	Doc    string
	Input  *jsonschema.Schema
	Output any
	Do     func(c *Client, in json.RawMessage) (any, error)
	Writes bool
	Asks   bool
	plan   *Plan
}

func (p *Provider) ToolActions(tools []*mcp.Tool) map[string]*Action {
	out := make(map[string]*Action, len(tools))
	for _, t := range tools {
		doc, _, _ := strings.Cut(strings.TrimSpace(t.Description), "\n\n")
		if len(doc) > 600 {
			doc = doc[:600] + "..."
		}
		a := &Action{Doc: doc, Output: t.OutputSchema, Writes: t.Annotations == nil || !t.Annotations.ReadOnlyHint}
		var s jsonschema.Schema
		if b, err := json.Marshal(t.InputSchema); err == nil && json.Unmarshal(b, &s) == nil {
			a.Input = &s
		}
		a.Do = func(c *Client, raw json.RawMessage) (any, error) {
			var in any
			json.Unmarshal(raw, &in)
			res, err := c.Call(t.Name, in)
			if err != nil {
				return nil, err
			}
			if res.StructuredContent != nil {
				return res.StructuredContent, nil
			}
			var v any
			if json.Unmarshal([]byte(Text(res)), &v) == nil {
				return v, nil
			}
			return Text(res), nil
		}
		name := t.Name
		for _, sep := range []string{"-", "_", "."} {
			if s, ok := strings.CutPrefix(t.Name, p.ID+sep); ok && s != "" {
				name = s
				break
			}
		}
		out[name] = a
	}
	return out
}

type Registration struct {
	Server   string    `json:"server"`
	Redirect string    `json:"redirect"`
	Resource string    `json:"resource"`
	AuthURL  string    `json:"auth_url"`
	TokenURL string    `json:"token_url"`
	Revoke   string    `json:"revoke,omitempty"`
	ClientID string    `json:"client_id"`
	Secret   string    `json:"secret,omitempty"`
	Method   string    `json:"method"`
	Scopes   []string  `json:"scopes,omitempty"`
	Expires  time.Time `json:"expires,omitzero"`
}

func (p *Provider) Fits(r *Registration, redirect string) bool {
	return r != nil && r.Server == p.server && r.Redirect == redirect && (r.Expires.IsZero() || time.Until(r.Expires) > TTL)
}

func (p *Provider) Registered(r *Registration) *Provider {
	q := *p
	q.clientID, q.secret, q.scopes, q.resource, q.revoke = r.ClientID, r.Secret, r.Scopes, r.Resource, r.Revoke
	q.endpoint = oauth2.Endpoint{AuthURL: r.AuthURL, TokenURL: r.TokenURL, AuthStyle: oauth2.AuthStyleInParams}
	if r.Method == "client_secret_basic" {
		q.endpoint.AuthStyle = oauth2.AuthStyleInHeader
	}
	return &q
}

func (p *Provider) Register(ctx context.Context, redirect string) (*Registration, error) {
	fail := func(format string, a ...any) error {
		return fmt.Errorf("%s could not be set up: %s", p.Name, fmt.Sprintf(format, a...))
	}
	su, err := url.Parse(p.server)
	if err != nil {
		return nil, fail("%v", err)
	}
	origin := su.Scheme + "://" + su.Host
	var metaURLs []string
	challenged := ""
	if req, err := http.NewRequestWithContext(ctx, "GET", p.server, nil); err == nil {
		req.Header.Set("Accept", "application/json, text/event-stream")
		if resp, err := client.Do(req); err == nil {
			resp.Body.Close()
			cs, _ := oauthex.ParseWWWAuthenticate(resp.Header.Values("WWW-Authenticate"))
			for _, c := range cs {
				if c.Scheme == "bearer" && c.Params["resource_metadata"] != "" {
					metaURLs = append(metaURLs, c.Params["resource_metadata"])
					challenged = c.Params["scope"]
				}
			}
		}
	}
	if path := strings.TrimRight(su.Path, "/"); path != "" {
		metaURLs = append(metaURLs, origin+"/.well-known/oauth-protected-resource"+path)
	}
	metaURLs = append(metaURLs, origin+"/.well-known/oauth-protected-resource")
	var prm *oauthex.ProtectedResourceMetadata
	for _, u := range metaURLs {
		if prm, err = oauthex.GetProtectedResourceMetadata(ctx, u, p.server, client); err == nil {
			break
		}
	}
	issuer, resource, scopes := origin, p.server, strings.Fields(challenged)
	if prm != nil {
		if len(prm.AuthorizationServers) > 0 {
			issuer = prm.AuthorizationServers[0]
		}
		resource = prm.Resource
		if len(scopes) == 0 {
			scopes = prm.ScopesSupported
		}
	}
	if len(p.Scopes) > 0 {
		scopes = p.Scopes
	}
	asm, err := auth.GetAuthServerMetadata(ctx, issuer, client)
	switch {
	case err != nil:
		return nil, fail("its authorization server %s: %v", issuer, err)
	case asm == nil:
		return nil, fail("its authorization server %s publishes no metadata", issuer)
	case asm.RegistrationEndpoint == "":
		return nil, fail("its authorization server %s does not let Archive register itself (no Dynamic Client Registration)", issuer)
	case !slices.Contains(asm.CodeChallengeMethodsSupported, "S256"):
		return nil, fail("its authorization server %s does not take PKCE with S256", issuer)
	}
	method := "none"
	for _, m := range []string{"client_secret_post", "client_secret_basic"} {
		if slices.Contains(asm.TokenEndpointAuthMethodsSupported, m) {
			method = m
			break
		}
	}
	reg, err := oauthex.RegisterClient(ctx, asm.RegistrationEndpoint, &oauthex.ClientRegistrationMetadata{
		ClientName: "Archive", ClientURI: "https://archive.dev", RedirectURIs: []string{redirect}, TokenEndpointAuthMethod: method,
		GrantTypes: []string{"authorization_code", "refresh_token"}, ResponseTypes: []string{"code"}, Scope: strings.Join(scopes, " "),
	}, client)
	if err != nil {
		var re *oauthex.ClientRegistrationError
		if errors.As(err, &re) {
			return nil, fail("it refused to register Archive: %s", strings.TrimSuffix(re.ErrorCode+": "+re.ErrorDescription, ": "))
		}
		return nil, fail("registering Archive: %v", err)
	}
	return &Registration{Server: p.server, Redirect: redirect, Resource: resource, AuthURL: asm.AuthorizationEndpoint, TokenURL: asm.TokenEndpoint,
		Revoke: asm.RevocationEndpoint, ClientID: reg.ClientID, Secret: reg.ClientSecret, Method: cmp.Or(reg.TokenEndpointAuthMethod, method),
		Scopes: scopes, Expires: reg.ClientSecretExpiresAt}, nil
}

func SealClient(aead cipher.AEAD, id string, r *Registration) []byte {
	b, _ := json.Marshal(r)
	return seal(aead, b, "client "+id)
}

func OpenClient(aead cipher.AEAD, id string, sealed []byte) (*Registration, error) {
	b, err := open(aead, sealed, "client "+id)
	if err != nil {
		return nil, fmt.Errorf("the client of %s does not open: %w", id, err)
	}
	var r Registration
	return &r, json.Unmarshal(b, &r)
}

func (c *Client) session() (*mcp.ClientSession, error) {
	if c.sess != nil {
		return c.sess, nil
	}
	if c.p.server == "" {
		return nil, fmt.Errorf("%s has no MCP server", c.p.Name)
	}
	cl := mcp.NewClient(&mcp.Implementation{Name: "archive", Title: "Archive", Version: "1"}, nil)
	c.mcpSeen = &seen{base: c.http.Transport}
	hc := *c.http
	hc.Transport = c.mcpSeen
	s, err := cl.Connect(c.ctx, &mcp.StreamableClientTransport{Endpoint: c.p.server, HTTPClient: &hc, MaxRetries: -1, DisableStandaloneSSE: true}, nil)
	if err != nil {
		return nil, unreached{fmt.Errorf("%s's MCP server could not be reached: %w", c.p.Name, err)}
	}
	c.sess = s
	return s, nil
}

func (c *Client) Tools() ([]*mcp.Tool, error) {
	s, err := c.session()
	if err != nil {
		return nil, err
	}
	var out []*mcp.Tool
	for t, err := range s.Tools(c.ctx, nil) {
		if err != nil {
			return nil, c.transportErr(fmt.Errorf("%s did not list its tools: %w", c.p.Name, err))
		}
		out = append(out, t)
	}
	return out, nil
}

func (c *Client) Call(name string, args any) (*mcp.CallToolResult, error) {
	s, err := c.session()
	if err != nil {
		return nil, err
	}
	res, err := s.CallTool(c.ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return nil, c.transportErr(fmt.Errorf("%s %s: %w", c.p.Name, name, err))
	}
	if res.IsError {
		return nil, &Refusal{Provider: c.p.Name, Message: name + ": " + Text(res), Busy: c.p.busySays(Text(res))}
	}
	return res, nil
}

type seen struct {
	base http.RoundTripper
	mu   sync.Mutex
	last *Refusal
}

func (t *seen) RoundTrip(r *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	res, err := base.RoundTrip(r)
	if err == nil && busyStatus(res.StatusCode) {
		t.mu.Lock()
		t.last = &Refusal{Status: res.StatusCode, Busy: true, Retry: retryAfter(res.Header)}
		t.mu.Unlock()
	}
	return res, err
}

func (c *Client) transportErr(err error) error {
	if c.mcpSeen == nil {
		return err
	}
	c.mcpSeen.mu.Lock()
	defer c.mcpSeen.mu.Unlock()
	if f := c.mcpSeen.last; f != nil {
		c.mcpSeen.last = nil
		return &Refusal{Provider: c.p.Name, Status: f.Status, Message: err.Error(), Busy: true, Retry: f.Retry}
	}
	return err
}

type pacer struct {
	mu   sync.Mutex
	pace time.Duration
	next time.Time
}

const pacerRest = 30 * time.Second

var pacers sync.Map

func pacerOf(id string, pace time.Duration) *pacer {
	p, _ := pacers.LoadOrStore(id, &pacer{pace: pace})
	return p.(*pacer)
}

func (p *pacer) slot() time.Time {
	p.mu.Lock()
	defer p.mu.Unlock()
	at := time.Now()
	if p.next.After(at) {
		at = p.next
	}
	p.next = at.Add(p.pace)
	return at
}

func (p *pacer) rest(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if until := time.Now().Add(d); until.After(p.next) {
		p.next = until
	}
}

type paced struct {
	base http.RoundTripper
	p    *pacer
}

func (t *paced) RoundTrip(r *http.Request) (*http.Response, error) {
	if wait := time.Until(t.p.slot()); wait > 0 {
		select {
		case <-time.After(wait):
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
	}
	res, err := t.base.RoundTrip(r)
	if err == nil && res.StatusCode == http.StatusTooManyRequests {
		t.p.rest(cmp.Or(retryAfter(res.Header), pacerRest))
	}
	return res, err
}

type unreached struct{ error }

func (u unreached) Unwrap() error { return u.error }

func Busy(err error) bool {
	if err == nil {
		return false
	}
	if r := (*Refusal)(nil); errors.As(err, &r) {
		return r.Busy
	}
	if re := (*oauth2.RetrieveError)(nil); errors.As(err, &re) && re.Response != nil {
		return busyStatus(re.Response.StatusCode)
	}
	return errors.As(err, new(unreached))
}

func RetryAfter(err error) time.Duration {
	if r := (*Refusal)(nil); errors.As(err, &r) {
		return r.Retry
	}
	return 0
}

func Text(res *mcp.CallToolResult) string {
	var parts []string
	for _, c := range res.Content {
		if t, ok := c.(*mcp.TextContent); ok {
			parts = append(parts, t.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func (c *Client) Close() {
	if c.sess != nil {
		c.sess.Close()
		c.sess = nil
	}
}

func mcpAccount(tool string) func(c *Client) (string, error) {
	return func(c *Client) (string, error) {
		res, err := c.Call(tool, map[string]any{})
		if err != nil {
			return "", err
		}
		v := res.StructuredContent
		text := Text(res)
		if v == nil {
			json.Unmarshal([]byte(text), &v)
		}
		for _, k := range []string{"email", "emailAddress", "name", "display_name", "displayName"} {
			if s := strings.TrimSpace(find(v, k)); s != "" && strings.Contains(s, "@") {
				return strings.ToLower(s), nil
			} else if s != "" {
				return s, nil
			}
		}
		line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
		if line = strings.TrimSpace(line); line == "" || len(line) > 120 {
			return "", fmt.Errorf("%s did not name an account", tool)
		}
		return line, nil
	}
}

func (p *Provider) ClientID() string { return p.clientID }

func (p *Provider) Ready() bool { return p.MCP != "" || len(p.Keys) > 0 || p.clientID != "" }
