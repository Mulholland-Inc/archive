// Package connect is Archive's connectors: the services a member connects their own accounts at, like their
// Gmail, so Archive can read them for them, and the actions a script takes with those accounts.
//
// A connector is one file of this package, <id>.go, that declares a Connector and adds it with Register in
// its init. Nothing else names it. It speaks one of two protocols, and a member never sees which:
//   - An API connector names the environment variable that holds its OAuth client, its endpoints and scopes,
//     and its Actions, Go funcs over the provider's API. Load reads every connector's variable, and one
//     whose variable is unset is unavailable: listed, but not offered.
//   - An MCP connector names only its MCP server. Its OAuth client is Archive's own, which Discover and
//     Register get from the server's authorization server the first time anyone connects (mcp.go), and the
//     caller keeps; its actions are the server's tools.
//
// Connecting is OAuth's authorization code flow with PKCE. AuthURL sends the browser to the provider with a
// State sealed by SealState; Exchange trades the code it comes back with for a Grant, the account and its
// refresh token, which the caller keeps sealed with SealToken; Refresh trades that for an access token, which
// Open turns into a Client for actions; RevokeToken ends it at the provider.
package connect

import (
	"cmp"
	"context"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

// Connector is a service accounts can be connected at, as its file declares it.
type Connector struct {
	ID, Name string
	// Env is the environment variable that holds an API connector's OAuth client, https://<client id>:<client
	// secret>@<host> with the id and secret URL-escaped, and maybe a query of options the connector reads with
	// Client.Option; Host is the provider host it names by default. When Env names another host, every URL of
	// the connector moves onto it: a stand-in for the provider, as in the tests.
	Env, Host string
	Endpoint  oauth2.Endpoint
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
	// Push is how the provider tells Archive an account changed (push.go); nil when it cannot, and then only
	// connecting, asking and reading start a sync.
	Push *Push
}

// MCPEnv is the environment variable that, when set, is another server for an MCP connector than its own: a
// stand-in, as in the tests. Unlike Env, nothing needs it.
func (c *Connector) MCPEnv() string { return "ARCHIVE_MCP_" + strings.ToUpper(c.ID) }

var registry = map[string]*Connector{}

// Register adds c; each connector's file calls it from init.
func Register(c *Connector) {
	if registry[c.ID] != nil {
		panic("connect: two connectors are " + c.ID)
	}
	registry[c.ID] = c
}

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
	// server is an MCP connector's server, its own or a stand-in; resource is what its tokens are for, as
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
	if p.base == "" || p.base == p.Host || err != nil {
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
	case err != nil && p.MCP != "":
		fmt.Fprintf(os.Stderr, "archive: %s did not say whose account it is, so it is named %s: %v\n", p.Name, p.Name, err)
		fallthrough
	case g.Account == "":
		g.Account = p.Name
	case err != nil:
		return nil, fmt.Errorf("%s did not say whose account it is: %v", p.Name, err)
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
