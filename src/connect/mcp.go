package connect

// An MCP connector's OAuth client is Archive's own, registered at its server's authorization server: Register
// finds that server as the MCP authorization spec has it, with the protected resource's metadata (RFC 9728)
// and then the authorization server's (RFC 8414), and registers a client there with Dynamic Client
// Registration (RFC 7591). The caller keeps the Registration, one per connector and server, and hands it to
// Registered for every later connect, refresh and revoke.

import (
	"cmp"
	"context"
	"crypto/cipher"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
	"golang.org/x/oauth2"
)

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
