// Package connect is Archive's connectors: the services a member connects their own accounts at, like their
// Gmail, so Archive can read them for them.
//
// A connector is one file of this package, <id>.go, that declares a Connector and adds it with Register in
// its init: the environment variable that holds its OAuth client, its endpoints and scopes, and how it reads
// whose account a token is for. Nothing else names it. Load reads every connector's variable, and one whose
// variable is unset is unavailable: listed, but not offered.
//
// Connecting is OAuth's authorization code flow with PKCE. AuthURL sends the browser to the provider with a
// State sealed by SealState; Exchange trades the code it comes back with for a Grant, the account and its
// refresh token, which the caller keeps sealed with SealToken; Refresh trades that for an access token;
// Revoke ends it at the provider.
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
	// Env is the environment variable that holds the OAuth client, https://<client id>:<client secret>@<host>
	// with the id and secret URL-escaped; Host is the provider host it names by default. When Env names
	// another host, every URL of the connector moves onto it: a stand-in for the provider, as in the tests.
	Env, Host string
	Endpoint  oauth2.Endpoint
	// Revoke is where a refresh token is revoked on disconnect, posted as token=, as RFC 7009 has it; empty
	// when the provider has nowhere.
	Revoke string
	Scopes []string
	// Params go on the authorization URL, like the ones that ask Google for a refresh token.
	Params map[string]string
	// Account is the account a token is for, like the mailbox address. get reads a URL of the provider as
	// JSON into v, with the token.
	Account func(get func(url string, v any) error) (string, error)
}

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

// Provider is a connector with the OAuth client this server has for it.
type Provider struct {
	*Connector
	clientID, secret, base string
}

// Providers are the available connectors, by id.
type Providers map[string]*Provider

// Load is every connector whose client getenv has.
func Load(getenv func(string) string) (Providers, error) {
	out := Providers{}
	for _, c := range All() {
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
			out[c.ID] = &Provider{Connector: c, clientID: id, secret: secret, base: strings.TrimRight(u.Scheme+"://"+u.Host+u.Path, "/")}
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
	if p.base == p.Host || err != nil {
		return u
	}
	return p.base + pu.RequestURI()
}

func (p *Provider) config(redirect string) *oauth2.Config {
	return &oauth2.Config{ClientID: p.clientID, ClientSecret: p.secret, RedirectURL: redirect, Scopes: p.Scopes,
		Endpoint: oauth2.Endpoint{AuthURL: p.at(p.Endpoint.AuthURL), TokenURL: p.at(p.Endpoint.TokenURL), AuthStyle: p.Endpoint.AuthStyle}}
}

// AuthURL is where the browser goes to consent, coming back to redirect with state and a code that only
// verifier (oauth2.GenerateVerifier) trades.
func (p *Provider) AuthURL(redirect, state, verifier string) string {
	opts := []oauth2.AuthCodeOption{oauth2.S256ChallengeOption(verifier)}
	for k, v := range p.Params {
		opts = append(opts, oauth2.SetAuthURLParam(k, v))
	}
	return p.config(redirect).AuthCodeURL(state, opts...)
}

// Grant is a connected account: who it is, its refresh token and the scopes the provider granted.
type Grant struct {
	Account, Refresh string
	Scopes           []string
}

// Exchange is the account the provider came back to redirect for, with the query q it came back with. It
// fails when the person refused, or left out a scope.
func (p *Provider) Exchange(ctx context.Context, redirect string, q url.Values, verifier string) (*Grant, error) {
	if e := q.Get("error"); e != "" {
		return nil, fmt.Errorf("%s was not connected: %s", p.Name, cmp.Or(q.Get("error_description"), e))
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, client)
	cfg := p.config(redirect)
	tok, err := cfg.Exchange(ctx, q.Get("code"), oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("%s was not connected: %s", p.Name, Reason(err))
	}
	g := &Grant{Refresh: tok.RefreshToken, Scopes: p.Scopes}
	if s, _ := tok.Extra("scope").(string); s != "" {
		g.Scopes = strings.Fields(s)
	}
	switch {
	case g.Refresh == "":
		return nil, fmt.Errorf("%s sent no refresh token, so Archive could not keep reading it; connect it again", p.Name)
	case slices.ContainsFunc(p.Scopes, func(s string) bool { return !slices.Contains(g.Scopes, s) }):
		return nil, fmt.Errorf("%s was connected without all the access Archive asked for; connect it again and allow it all", p.Name)
	}
	c := cfg.Client(ctx, tok)
	g.Account, err = p.Account(func(u string, v any) error {
		req, err := http.NewRequestWithContext(ctx, "GET", p.at(u), nil)
		if err != nil {
			return err
		}
		resp, err := c.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("%s %s", req.URL.Path, resp.Status)
		}
		return json.NewDecoder(resp.Body).Decode(v)
	})
	if err == nil && g.Account == "" {
		err = errors.New("it named no account")
	}
	if err != nil {
		return nil, fmt.Errorf("%s did not say whose account it is: %v", p.Name, err)
	}
	return g, nil
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

// RevokeToken ends the grant of token at the provider, where it has a way to.
func (p *Provider) RevokeToken(ctx context.Context, token string) error {
	if p.Revoke == "" {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, "POST", p.at(p.Revoke), strings.NewReader(url.Values{"token": {token}}.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s revoke: %s", p.Name, resp.Status)
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
