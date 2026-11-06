package connect

import (
	"context"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// A connector signs in with OAuth 1.0a when OAuth1 is its request-token URL, like Evernote's: Archive asks for a
// request token, the person allows it at Endpoint.AuthURL, and Archive trades it and the verifier for an access token
// at Endpoint.TokenURL. That token lasts until it expires and never refreshes, so it is kept as the refresh token is.
// The request token's secret is between the two legs: the callback URL is fixed before it is known, so it rides in a
// sealed cookie, not the state.

// RequestToken is the first leg: a request token and its secret, to come back to callback.
func (p *Provider) RequestToken(ctx context.Context, callback string) (token, secret string, err error) {
	v, err := p.oauth1(ctx, p.at(p.OAuth1), "", "", url.Values{"oauth_callback": {callback}})
	if err == nil && v.Get("oauth_token") == "" {
		err = fmt.Errorf("%s sent no request token", p.Name)
	}
	if err != nil {
		return "", "", fmt.Errorf("%s could not start signing in: %w", p.Name, err)
	}
	return v.Get("oauth_token"), v.Get("oauth_token_secret"), nil
}

// AuthURL1 is where the person allows the request token.
func (p *Provider) AuthURL1(token string) string {
	u, _ := url.Parse(p.endpoint.AuthURL)
	q := u.Query()
	q.Set("oauth_token", token)
	u.RawQuery = q.Encode()
	return u.String()
}

// Exchange1 is the second leg, from the callback's query: the access token, with the response's Keep values as Meta.
func (p *Provider) Exchange1(ctx context.Context, q url.Values, token, secret string) (*Grant, error) {
	if q.Get("oauth_token") != token {
		return nil, fmt.Errorf("%s was not connected: it came back for another sign-in; start again", p.Name)
	}
	if q.Get("oauth_verifier") == "" {
		return nil, fmt.Errorf("%s was not connected: you did not allow Archive", p.Name)
	}
	v, err := p.oauth1(ctx, p.endpoint.TokenURL, token, secret, url.Values{"oauth_verifier": {q.Get("oauth_verifier")}})
	if err != nil {
		return nil, fmt.Errorf("%s was not connected: %w", p.Name, err)
	}
	g := &Grant{Refresh: v.Get("oauth_token"), Scopes: append([]string{}, p.scopes...), Meta: map[string]string{}}
	if g.Refresh == "" {
		return nil, fmt.Errorf("%s sent no access token; connect it again", p.Name)
	}
	for _, k := range p.Keep {
		if g.Meta[k] = v.Get(k); g.Meta[k] == "" {
			return nil, fmt.Errorf("%s was not connected: it came back without %s", p.Name, k)
		}
		if vals, ok := p.Allow[k]; ok && !slices.Contains(vals, g.Meta[k]) {
			return nil, fmt.Errorf("%s was not connected: it came back with %s %s, which Archive does not know", p.Name, k, g.Meta[k])
		}
	}
	return p.named(ctx, &oauth2.Token{AccessToken: g.Refresh}, g)
}

// oauth1 sends a request signed with HMAC-SHA1, its oauth_ parameters in the query, and reads the form it answers.
func (p *Provider) oauth1(ctx context.Context, u, token, secret string, extra url.Values) (url.Values, error) {
	pu, err := url.Parse(u)
	if err != nil {
		return nil, err
	}
	q := pu.Query()
	for k, vs := range extra {
		q[k] = vs
	}
	q.Set("oauth_consumer_key", p.clientID)
	q.Set("oauth_nonce", nonce())
	q.Set("oauth_signature_method", "HMAC-SHA1")
	q.Set("oauth_timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	q.Set("oauth_version", "1.0")
	if token != "" {
		q.Set("oauth_token", token)
	}
	pu.RawQuery = ""
	q.Set("oauth_signature", oauth1Sign("GET", pu.String(), q, p.secret, secret))
	pu.RawQuery = oauth1Query(q)
	req, err := http.NewRequestWithContext(ctx, "GET", pu.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, unreached{fmt.Errorf("%s could not be reached: %w", p.Name, err)}
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, refusal(p, resp)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return nil, err
	}
	return url.ParseQuery(strings.TrimSpace(string(b)))
}

func oauth1Sign(method, base string, q url.Values, consumerSecret, tokenSecret string) string {
	m := hmac.New(sha1.New, []byte(oauth1Esc(consumerSecret)+"&"+oauth1Esc(tokenSecret)))
	m.Write([]byte(method + "&" + oauth1Esc(base) + "&" + oauth1Esc(oauth1Query(q))))
	return base64.StdEncoding.EncodeToString(m.Sum(nil))
}

// oauth1Query is q's pairs escaped as RFC 5849 says and sorted, as a signature base string and a query both want.
func oauth1Query(q url.Values) string {
	var pairs []string
	for k, vs := range q {
		for _, v := range vs {
			pairs = append(pairs, oauth1Esc(k)+"="+oauth1Esc(v))
		}
	}
	slices.Sort(pairs)
	return strings.Join(pairs, "&")
}

func oauth1Esc(s string) string { return strings.ReplaceAll(url.QueryEscape(s), "+", "%20") }

// SealSecret seals a request token's secret for a cookie, bound to its token, so it opens only for that sign-in.
func SealSecret(aead cipher.AEAD, token, secret string) string {
	return base64.RawURLEncoding.EncodeToString(seal(aead, []byte(secret), "oauth1 "+token))
}

func OpenSecret(aead cipher.AEAD, token, sealed string) (string, bool) {
	ct, err := base64.RawURLEncoding.DecodeString(sealed)
	if err != nil {
		return "", false
	}
	b, err := open(aead, ct, "oauth1 "+token)
	return string(b), err == nil
}
