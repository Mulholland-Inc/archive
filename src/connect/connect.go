// Package connect is Archive's connectors: the services a member connects their own accounts at, like their
// Gmail, so Archive can read them for them, and the actions a script takes with those accounts.
//
// A connector is one file of this package, <id>.go, that declares a Connector and adds it with Register in
// its init. Nothing else names it. One with an API may be only data, which the engine in def.go runs. It speaks one of three protocols, and a member never sees which:
//   - An API connector names the environment variable that holds its OAuth client, its endpoints and scopes,
//     and its Actions, Go funcs over the provider's API. Load reads every connector's variable, and one
//     whose variable is unset is unavailable: listed, but not offered.
//   - An MCP connector names only its MCP server. Its OAuth client is Archive's own, which Discover and
//     Register get from the server's authorization server the first time anyone connects (below), and the
//     caller keeps; its actions are the server's tools.
//   - A key connector has no sign-in: a person pastes in what the service gave them, like an API key and the
//     address of their store, as its Keys say (keys.go). Its Authorize puts them on each request.
//
// Connecting is OAuth's authorization code flow with PKCE. AuthURL sends the browser to the provider with a
// State sealed by SealState; Exchange trades the code it comes back with for a Grant, the account and its
// refresh token, which the caller keeps sealed with SealToken; Refresh trades that for an access token, which
// Open turns into a Client for actions; RevokeToken ends it at the provider.
package connect

import (
	"bytes"
	"cmp"
	"context"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
	"golang.org/x/oauth2"
)

// Connector is a service accounts can be connected at, as its file declares it.
type Connector struct {
	ID, Name string
	// Integration says an account connected to it is the archive's, like its QuickBooks company: an owner
	// connects it for everyone, an archive has one, and its records are every member's. Without it, each member
	// connects their own, like their Gmail, and its records are theirs alone.
	Integration bool
	// Pace is the least time between two requests this server sends the provider, every account's and every
	// page's together, for a provider that refuses requests that come faster, as Gusto does; zero for none.
	Pace time.Duration
	// Env is the environment variable that holds an API connector's OAuth client, https://<client id>:<client
	// secret>@<host> with the id and secret URL-escaped, and maybe a query of options the connector reads with
	// Client.Option; Host is the provider host it names by default. When Env names another host, every URL of
	// the connector moves onto it, so another server that speaks the provider's API can stand in for it.
	Env, Host string
	// Hosts, when given, are the only hosts whose URLs move onto the one Env names, like Salesforce's login
	// server, whose sandbox is another, while each org's API stays on its own host.
	Hosts    []string
	Endpoint oauth2.Endpoint
	// Revoke is where a refresh token is revoked on disconnect, posted as token=, as RFC 7009 has it; empty
	// when the provider has nowhere. RevokeWith, when set, revokes in its place, for a provider that does it
	// another way.
	Revoke     string
	RevokeWith func(rv *Revocation) error
	Scopes     []string
	// Optional are scopes an API connector asks for too, but connects without, like those its pushes need:
	// asked for only once its variable has ?push=1, as its app may not have them yet.
	Optional []string
	// Params go on the authorization URL, like the ones that ask Google for a refresh token.
	Params map[string]string
	// Keep are parameters the provider adds to the callback that calls need later, like QuickBooks' realmId;
	// the Grant keeps them as its Meta.
	Keep []string
	// MCP is an MCP connector's server. It needs no Env, Host, Endpoint, Revoke or Scopes: its authorization
	// server has them. Scopes, when given, are asked for in place of the ones the server suggests.
	MCP string
	// Account is the account a token is for, like the mailbox address, read with c. An MCP connector's may be
	// nil, or fail: then the account is the connector's name, and a member has one connection to it.
	Account func(c *Client) (string, error)
	// Actions are an API connector's actions, by name; an MCP connector's are its server's tools.
	Actions map[string]*Action
	// Streams are what Archive keeps a copy of for each account (sync.go), pulled in this order when several
	// are due at once.
	Streams []*Stream
	// Push is how the provider tells Archive an account changed (sync.go); nil when it cannot, and then only
	// connecting, asking and reading start a sync.
	Push *Push
	// Keys are what a key connector asks a person for in place of a sign-in, and Authorize puts them on a request
	// (keys.go). Its Env, when set, is a client of the server's own its requests need too, like a developer key.
	Keys      []Key
	Authorize func(req *http.Request, c *Client)
	// Info is how it shows. Options says what else its Env's value takes or does, for the docs of that variable.
	Info    Info
	Options string
	// API is the base of the requests of a connector that is only data (def.go): its Env is ARCHIVE_<ID> unless it
	// says, and its Host that of its Endpoint. Sign are the headers of its keys' requests, Who its Account, RevokeBy
	// its RevokeWith, Listen its Push and Sync its Streams.
	API      string
	Sign     map[string]string
	Who      *Who
	RevokeBy *Req
	Listen   *Listen
	Sync     []*Sync
}

// MCPEnv is the environment variable that, when set, is another server for an MCP connector than its own, one
// that stands in for it. Unlike Env, nothing needs it.
func (c *Connector) MCPEnv() string { return "ARCHIVE_MCP_" + strings.ToUpper(c.ID) }

var registry = map[string]*Connector{}

// Register adds c; each connector's file calls it from init.
func Register(c *Connector) {
	if registry[c.ID] != nil {
		panic("connect: two connectors are " + c.ID)
	}
	if err := c.compile(); err != nil {
		panic("connect: " + c.ID + ": " + err.Error())
	}
	registry[c.ID] = c
}

// Get is connector id, or nil when there is none.
func Get(id string) *Connector { return registry[id] }

// All are the connectors, by id.
func All() []*Connector {
	out := make([]*Connector, 0, len(registry))
	for _, c := range registry {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Provider is a connector with the OAuth client this server has for it: from its variable for an API
// connector, and for an MCP connector from its Registration, which Registered adds.
type Provider struct {
	*Connector
	clientID, secret, base string
	opts                   url.Values
	endpoint               oauth2.Endpoint
	revoke                 string
	scopes                 []string
	// server is an MCP connector's server, its own or one standing in; resource is what its tokens are for, as
	// RFC 8707 has the authorization server told.
	server, resource string
	// pushTo is the value of its Push's Env.
	pushTo string
}

// Providers are the available connectors, by id.
type Providers map[string]*Provider

// Load is every MCP connector, and every API connector whose client getenv has.
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

// Get is the available connector id, or why there is none.
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

// at is the provider's URL u, on the host its client names.
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

// AuthURL is where the browser goes to consent, coming back to redirect with state and a code that only
// verifier (oauth2.GenerateVerifier) trades.
func (p *Provider) AuthURL(redirect, state, verifier string) string {
	opts := []oauth2.AuthCodeOption{oauth2.S256ChallengeOption(verifier)}
	for k, v := range p.Params {
		opts = append(opts, oauth2.SetAuthURLParam(k, v))
	}
	if p.resource != "" {
		opts = append(opts, oauth2.SetAuthURLParam("resource", p.resource))
	}
	return p.config(redirect).AuthCodeURL(state, opts...)
}

// Grant is a connected account: who it is, its refresh token, the scopes the provider granted and the
// parameters of the callback the connector keeps.
type Grant struct {
	Account, Refresh string
	Scopes           []string
	Meta             map[string]string
}

// Exchange is the account the provider came back to redirect for, with the query q it came back with. It
// fails when the person refused, or left out a scope an API connector asks for.
func (p *Provider) Exchange(ctx context.Context, redirect string, q url.Values, verifier string) (*Grant, error) {
	if e := q.Get("error"); e != "" {
		return nil, fmt.Errorf("%s was not connected: %s", p.Name, cmp.Or(q.Get("error_description"), e))
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, client)
	opts := []oauth2.AuthCodeOption{oauth2.VerifierOption(verifier)}
	if p.resource != "" {
		opts = append(opts, oauth2.SetAuthURLParam("resource", p.resource))
	}
	tok, err := p.config(redirect).Exchange(ctx, q.Get("code"), opts...)
	if err != nil {
		return nil, fmt.Errorf("%s was not connected: %s", p.Name, Reason(err))
	}
	g := &Grant{Refresh: tok.RefreshToken, Scopes: p.scopes, Meta: map[string]string{}}
	if s, _ := tok.Extra("scope").(string); s != "" {
		g.Scopes = strings.Fields(s)
	}
	for _, k := range p.Keep {
		if g.Meta[k] = q.Get(k); g.Meta[k] == "" {
			return nil, fmt.Errorf("%s was not connected: it came back without %s", p.Name, k)
		}
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

// granted says whether the scopes a provider granted have want: itself, or a URL ending in it, as Microsoft
// answers Graph's. offline_access is what asks for the refresh token, which nobody lists again.
func granted(scopes []string, want string) bool {
	return want == "offline_access" || slices.ContainsFunc(scopes, func(s string) bool {
		return strings.EqualFold(s, want) || strings.HasSuffix(strings.ToLower(s), "/"+strings.ToLower(want))
	})
}

// Refresh is a fresh access token for the refresh token rt. Its RefreshToken is the one to keep from now on:
// rt, or the one the provider replaced it with.
func (p *Provider) Refresh(ctx context.Context, rt string) (*oauth2.Token, error) {
	if len(p.Keys) > 0 {
		return &oauth2.Token{AccessToken: rt, RefreshToken: rt}, nil
	}
	tok, err := p.config("").TokenSource(context.WithValue(ctx, oauth2.HTTPClient, client), &oauth2.Token{RefreshToken: rt}).Token()
	if err != nil {
		return nil, err
	}
	tok.RefreshToken = cmp.Or(tok.RefreshToken, rt)
	return tok, nil
}

// RevokeToken ends the grant of the refresh token rt at the provider, where it has a way to: RevokeWith, or
// RFC 7009's form at Revoke, which an MCP connector's authorization server also takes its client's
// credentials with.
func (p *Provider) RevokeToken(ctx context.Context, rt string) error {
	rv := &Revocation{Refresh: rt, ctx: ctx, p: p}
	switch {
	case len(p.Keys) > 0:
		return nil
	case p.RevokeWith != nil:
		return rv.p.RevokeWith(rv)
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

// basic is the Authorization header of HTTP Basic with the client id and secret, form-encoded as RFC 6749 has
// them.
func basic(id, secret string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(url.QueryEscape(id)+":"+url.QueryEscape(secret)))
}

// Revocation is a grant a connector's RevokeWith ends.
type Revocation struct {
	// Refresh is the grant's refresh token.
	Refresh string
	ctx     context.Context
	p       *Provider
}

// Client is the OAuth client's id and secret.
func (rv *Revocation) Client() (id, secret string) { return rv.p.clientID, rv.p.secret }

// Access is an access token of the grant, for a provider that revokes with one.
func (rv *Revocation) Access() (string, error) {
	tok, err := rv.p.Refresh(rv.ctx, rv.Refresh)
	if err != nil {
		return "", err
	}
	return tok.AccessToken, nil
}

// Form posts v to the provider's URL u as a form, with hdr, and fails unless it answers 2xx.
func (rv *Revocation) Form(u string, hdr http.Header, v url.Values) error {
	return rv.send("POST", rv.p.at(u), hdr, v)
}

// Send sends body to the provider's URL u, JSON unless it is a string, with hdr, and fails unless it
// answers 2xx.
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
		return refusal(rv.p.Name, resp)
	}
	return nil
}

// Reason is what a provider said when it refused, without the rest of its answer.
func Reason(err error) string {
	var re *oauth2.RetrieveError
	if errors.As(err, &re) && re.ErrorCode != "" {
		return strings.TrimSuffix(re.ErrorCode+": "+re.ErrorDescription, ": ")
	}
	return err.Error()
}

// State is what the provider hands back with the code: who started connecting, where, and the PKCE verifier.
type State struct {
	Provider  string    `json:"p"`
	Workspace string    `json:"w"`
	User      uuid.UUID `json:"u"`
	Member    uuid.UUID `json:"m"`
	Verifier  string    `json:"v"`
	Exp       int64     `json:"x"`
}

// TTL is how long a person has to consent.
const TTL = 10 * time.Minute

// SealState is st encrypted and authenticated with aead, to fit a URL.
func SealState(aead cipher.AEAD, st State) string {
	b, _ := json.Marshal(st)
	return base64.RawURLEncoding.EncodeToString(seal(aead, b, "connect"))
}

// OpenState is the state SealState sealed, while it has not expired.
func OpenState(aead cipher.AEAD, sealed string) (State, bool) {
	var st State
	ct, err := base64.RawURLEncoding.DecodeString(sealed)
	if err != nil {
		return st, false
	}
	b, err := open(aead, ct, "connect")
	return st, err == nil && json.Unmarshal(b, &st) == nil && time.Now().Unix() <= st.Exp
}

// SealToken is the refresh token of connection id, encrypted and bound to it.
func SealToken(aead cipher.AEAD, id uuid.UUID, token string) []byte {
	return seal(aead, []byte(token), "connection "+id.String())
}

// OpenToken is the refresh token SealToken sealed for connection id.
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

// maxBody is the most of a provider's answer a Client reads, like fetch's limit in a run.
const maxBody = 5 << 20

// Client is a connected account at work, for an Account or an Action: requests to the provider's API as the
// account, or calls to its MCP server's tools. Close it when done.
type Client struct {
	ctx  context.Context
	p    *Provider
	http *http.Client
	tok  *oauth2.Token
	// Meta are the callback's parameters the connector keeps, like QuickBooks' realmId.
	Meta map[string]string
	sess *mcp.ClientSession
	// tools are the MCP server's, as it listed them for a stream.
	tools []*mcp.Tool
	// keys are a key connector's, by name.
	keys map[string]string
}

// Open is a Client of the account whose access token tok is, with the Meta its grant kept.
func (p *Provider) Open(ctx context.Context, tok *oauth2.Token, meta map[string]string) *Client {
	if len(p.Keys) > 0 {
		return p.openKeys(ctx, keysFrom(tok.AccessToken), meta)
	}
	var rt http.RoundTripper = &oauth2.Transport{Source: oauth2.StaticTokenSource(tok)}
	if p.Pace > 0 {
		rt = &paced{base: rt, p: pacerOf(p.ID, p.Pace)}
	}
	hc := &http.Client{Timeout: client.Timeout, Transport: rt}
	return &Client{ctx: ctx, p: p, http: hc, tok: tok, Meta: meta}
}

// Context is what the Client's requests run in.
func (c *Client) Context() context.Context { return c.ctx }

// Option is the option k in the query of the connector's variable, like ?sandbox=1.
func (c *Client) Option(k string) string { return c.p.opts.Get(k) }

// Token is the access token, for a provider that wants it somewhere other than the Authorization header.
func (c *Client) Token() string {
	if c.tok == nil {
		return ""
	}
	return c.tok.AccessToken
}

// Credentials are the OAuth client's id and secret, for a provider that wants them with a request.
func (c *Client) Credentials() (id, secret string) { return c.p.clientID, c.p.secret }

// Get reads the provider's URL u as JSON into v.
func (c *Client) Get(u string, v any) error { return c.Do("GET", u, nil, v) }

// Do sends body to the provider's URL u, as a form when it is url.Values, as it is when a string and JSON
// otherwise, and reads the answer as JSON into v; into a *string or *[]byte it is the answer as it came. A
// 4xx or 5xx is a *Refusal.
func (c *Client) Do(method, u string, body, v any) error { return c.Send(method, u, nil, body, v) }

// Send is Do with the headers hdr too.
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
		return fmt.Errorf("%s could not be reached: %w", c.p.Name, errors.Unwrap(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return refusal(c.p.Name, resp)
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

// Refusal is a provider answering a request with a 4xx or 5xx, or a tool's result marked as an error, with
// no Status.
type Refusal struct {
	Provider string
	Status   int
	// Message is what it said why, as it said it.
	Message string
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

func refusal(provider string, resp *http.Response) *Refusal {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	return &Refusal{Provider: provider, Status: resp.StatusCode, Message: reasonIn(b)}
}

// reasonIn is the message in a provider's error body: the first of the keys providers put it under, at any
// depth, like Google's error.message, QuickBooks' Fault.Error[0].Detail or OAuth's error_description; or the
// body itself, cut short.
func reasonIn(b []byte) string {
	var v any
	if json.Unmarshal(b, &v) != nil {
		s := strings.Join(strings.Fields(string(b)), " ")
		if len(s) > 300 {
			s = s[:300] + "..."
		}
		return s
	}
	for _, k := range []string{"message", "error_description", "Detail", "Message", "detail", "error"} {
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

// Action is something a script does with a connected account of an API connector, as
// archive.action("<connector>.<name>", input). Its input is checked against Input before Do sees it.
type Action struct {
	// Doc says what it does and returns, for the run tool's docs.
	Doc    string
	Input  *jsonschema.Schema
	Output any
	Do     func(c *Client, in json.RawMessage) (any, error)
	// Writes says it changes something at the provider, like sending a message, which a viewer-only token
	// may not.
	Writes bool
	plan   *Plan
}

// ToolActions are the actions of an MCP connector: its server's tools, each by its name without the connector's id
// before it. A tool writes unless the server says it only reads.
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

// Write is Act for an action that Writes.
func Write[In any](doc string, do func(c *Client, in In) (any, error)) *Action {
	a := Act(doc, do)
	a.Writes = true
	return a
}

// Act is an Action whose input is In, a struct: its fields' json names are the input's keys, required unless
// omitempty, and their jsonschema tags describe them.
func Act[In any](doc string, do func(c *Client, in In) (any, error)) *Action {
	s, err := jsonschema.For[In](nil)
	if err != nil {
		panic(err)
	}
	return &Action{Doc: doc, Input: s, Do: func(c *Client, raw json.RawMessage) (any, error) {
		var in In
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, err
		}
		return do(c, in)
	}}
}

// An MCP connector's OAuth client is Archive's own, registered at its server's authorization server: Register
// finds that server as the MCP authorization spec has it, with the protected resource's metadata (RFC 9728)
// and then the authorization server's (RFC 8414), and registers a client there with Dynamic Client
// Registration (RFC 7591). The caller keeps the Registration, one per connector and server, and hands it to
// Registered for every later connect, refresh and revoke.

// Registration is the client Archive registered at an MCP connector's authorization server, with what it
// needs of that server's metadata.
type Registration struct {
	// Server is the MCP server it is for, and Redirect the redirect URI it registered: a change of either
	// registers again.
	Server   string `json:"server"`
	Redirect string `json:"redirect"`
	// Resource is the server's resource identifier, which goes with every authorization and code (RFC 8707).
	Resource string `json:"resource"`
	AuthURL  string `json:"auth_url"`
	TokenURL string `json:"token_url"`
	Revoke   string `json:"revoke,omitempty"`
	ClientID string `json:"client_id"`
	Secret   string `json:"secret,omitempty"`
	// Method is how the client authenticates at the token endpoint: client_secret_basic, client_secret_post
	// or none, PKCE alone.
	Method string   `json:"method"`
	Scopes []string `json:"scopes,omitempty"`
	// Expires is when the secret expires; zero when it does not.
	Expires time.Time `json:"expires,omitzero"`
}

// Fits says whether r is still the client for p and redirect.
func (p *Provider) Fits(r *Registration, redirect string) bool {
	return r != nil && r.Server == p.server && r.Redirect == redirect && (r.Expires.IsZero() || time.Until(r.Expires) > TTL)
}

// Registered is p with the client r.
func (p *Provider) Registered(r *Registration) *Provider {
	q := *p
	q.clientID, q.secret, q.scopes, q.resource, q.revoke = r.ClientID, r.Secret, r.Scopes, r.Resource, r.Revoke
	q.endpoint = oauth2.Endpoint{AuthURL: r.AuthURL, TokenURL: r.TokenURL, AuthStyle: oauth2.AuthStyleInParams}
	if r.Method == "client_secret_basic" {
		q.endpoint.AuthStyle = oauth2.AuthStyleInHeader
	}
	return &q
}

// Register finds the authorization server of p's MCP server and registers Archive there as a client that
// comes back to redirect. It fails, saying so, when the server does not let clients register themselves.
func (p *Provider) Register(ctx context.Context, redirect string) (*Registration, error) {
	fail := func(format string, a ...any) error {
		return fmt.Errorf("%s could not be set up: %s", p.Name, fmt.Sprintf(format, a...))
	}
	su, err := url.Parse(p.server)
	if err != nil {
		return nil, fail("%v", err)
	}
	origin := su.Scheme + "://" + su.Host
	// The server's 401 names its metadata and the scopes to ask for; without one, the well-known places do.
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
	// A server without that metadata is its own authorization server, as the MCP spec once had it.
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
	// A secret where the server takes one, as the confidential client Archive is; PKCE alone where it does not.
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

// SealClient is r encrypted and bound to connector id, to keep.
func SealClient(aead cipher.AEAD, id string, r *Registration) []byte {
	b, _ := json.Marshal(r)
	return seal(aead, b, "client "+id)
}

// OpenClient is the Registration SealClient sealed for connector id.
func OpenClient(aead cipher.AEAD, id string, sealed []byte) (*Registration, error) {
	b, err := open(aead, sealed, "client "+id)
	if err != nil {
		return nil, fmt.Errorf("the client of %s does not open: %w", id, err)
	}
	var r Registration
	return &r, json.Unmarshal(b, &r)
}

// session is the Client's session with the MCP server, opened on first use.
func (c *Client) session() (*mcp.ClientSession, error) {
	if c.sess != nil {
		return c.sess, nil
	}
	if c.p.server == "" {
		return nil, fmt.Errorf("%s has no MCP server", c.p.Name)
	}
	cl := mcp.NewClient(&mcp.Implementation{Name: "archive", Title: "Archive", Version: "1"}, nil)
	s, err := cl.Connect(c.ctx, &mcp.StreamableClientTransport{Endpoint: c.p.server, HTTPClient: c.http, MaxRetries: -1, DisableStandaloneSSE: true}, nil)
	if err != nil {
		return nil, fmt.Errorf("%s's MCP server could not be reached: %w", c.p.Name, err)
	}
	c.sess = s
	return s, nil
}

// Tools are the tools of the MCP server, every page of them.
func (c *Client) Tools() ([]*mcp.Tool, error) {
	s, err := c.session()
	if err != nil {
		return nil, err
	}
	var out []*mcp.Tool
	for t, err := range s.Tools(c.ctx, nil) {
		if err != nil {
			return nil, fmt.Errorf("%s did not list its tools: %w", c.p.Name, err)
		}
		out = append(out, t)
	}
	return out, nil
}

// Call calls the MCP server's tool name with args. A result the tool marks as an error is an error, with
// what the tool said.
func (c *Client) Call(name string, args any) (*mcp.CallToolResult, error) {
	s, err := c.session()
	if err != nil {
		return nil, err
	}
	res, err := s.CallTool(c.ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", c.p.Name, name, err)
	}
	if res.IsError {
		return nil, &Refusal{Provider: c.p.Name, Message: name + ": " + Text(res)}
	}
	return res, nil
}

// pacer spaces the requests this server sends one provider at least pace apart, and holds them all back when
// the provider says it had too many: until its Retry-After, or pacerRest when it names none.
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

// slot is when the next request may go, which it reserves.
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

// paced is a transport that sends each request in its pacer's next slot.
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
		rest := pacerRest
		if s, e := strconv.Atoi(res.Header.Get("Retry-After")); e == nil && s > 0 {
			rest = time.Duration(s) * time.Second
		}
		t.p.rest(rest)
	}
	return res, err
}

// Busy says err is a provider that turned a call away for now, by rate limit or by being out of reach, rather
// than one that will fail again: a sync keeps trying such a page, later and later, and never gives it up.
func Busy(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	for _, w := range []string{"Too Many Requests", " 429", "could not be reached", " 502", " 503", " 504", "Service Unavailable", "Bad Gateway", "Gateway Timeout",
		"Quota exceeded", "rateLimitExceeded", "userRateLimitExceeded", "Rate Limit Exceeded"} {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

// Text is the text of a tool's result, its text parts on lines of their own.
func Text(res *mcp.CallToolResult) string {
	var parts []string
	for _, c := range res.Content {
		if t, ok := c.(*mcp.TextContent); ok {
			parts = append(parts, t.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// Close ends the Client's MCP session, if it opened one.
func (c *Client) Close() {
	if c.sess != nil {
		c.sess.Close()
		c.sess = nil
	}
}

// mcpAccount is an MCP connector's Account that calls tool, without arguments, and reads its answer: the
// first email, then the first name, in its JSON; or its first line.
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

// ClientID is the OAuth client's id.
func (p *Provider) ClientID() string { return p.clientID }
