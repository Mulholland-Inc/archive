package connect

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/oauth2"
)

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
}

// Open is a Client of the account whose access token tok is, with the Meta its grant kept.
func (p *Provider) Open(ctx context.Context, tok *oauth2.Token, meta map[string]string) *Client {
	hc := &http.Client{Timeout: client.Timeout, Transport: &oauth2.Transport{Source: oauth2.StaticTokenSource(tok)}}
	return &Client{ctx: ctx, p: p, http: hc, tok: tok, Meta: meta}
}

// Context is what the Client's requests run in.
func (c *Client) Context() context.Context { return c.ctx }

// Option is the option k in the query of the connector's variable, like ?sandbox=1.
func (c *Client) Option(k string) string { return c.p.opts.Get(k) }

// Token is the access token, for a provider that wants it somewhere other than the Authorization header.
func (c *Client) Token() string { return c.tok.AccessToken }

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
	Doc   string
	Input *jsonschema.Schema
	Do    func(c *Client, in json.RawMessage) (any, error)
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
