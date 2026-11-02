package connect

// A connector's Push is how its provider tells Archive that an account changed, so that its sync runs then and
// only then: nothing in Archive asks a provider on a timer. A push only says which account; what changed is
// the streams' delta from their cursors. So a push that is lost, or one missed while a watch had lapsed, loses
// nothing: the next sync, whatever starts it, reads everything since the last.
//
// A provider pushes one of two ways. Some push for every account of the app, as its dashboard is set up once
// (GitHub, HubSpot, QuickBooks): their Watch only names the keys their pushes carry for an account, like its
// QuickBooks realmId, and Hear checks a push with the app's secret. Others push for an account only once
// Archive asks them to, to Archive's URL (Gmail, Drive, Outlook, Asana), until the watch Expires: the first
// sync of the account asks, and a sync that runs once a watch is within Renew of expiring renews it. Nothing
// renews one on a timer; an account nobody touches lets its watch lapse, and the next sync, on a read or an
// open, asks again.

import (
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// Push is how a connector's provider tells Archive an account changed.
type Push struct {
	// Watch asks the provider to push the account's changes, renewing or replacing old, the watches it has, and
	// is the watches there are now; an app's pushes need only their keys. Watch with h.URL empty is not asked.
	Watch func(c *Client, h Hook, old []Watch) ([]Watch, error)
	// Unwatch ends w, when the account disconnects; nil when there is nothing to end.
	Unwatch func(c *Client, w Watch) error
	// Renew is how long before a watch expires the sync after renews it.
	Renew time.Duration
	// Env is the variable that names where the provider pushes, like Gmail's Pub/Sub topic; Watch reads it as
	// Hook.Target, and is not asked without it.
	Env string
	// OIDC says the provider pushes through Google Pub/Sub, whose push subscription signs each with the OIDC
	// token of Archive's tasks account, which Archive checks before Hear.
	OIDC bool
	// Hear reads a push the provider sent to the URL at, checking it is the provider's, as the keys of the
	// watches it is for: secret is the secret Archive kept with the watch of key, "" when there is none.
	Hear func(p *Provider, r *http.Request, at string, body []byte, secret func(key string) string) (*Heard, error)
}

// Hook is where a Watch asks for pushes.
type Hook struct {
	// ID is the connection's, which starts the keys that must be its own; Account its account.
	ID, Account string
	// URL is where the provider pushes for the watch of key: Archive's, with the key in its query.
	URL func(key string) string
	// Target is the value of the Push's Env.
	Target string
}

// Watch is one thing a provider pushes for an account: Key is what its pushes name it by, Resource what the
// provider calls it, to end it, Secret what its pushes prove themselves with, and Expires when it ends on its
// own, zero for never.
type Watch struct {
	Key, Resource, Secret string
	Expires               time.Time
}

// Heard is what a push said: the keys of the watches it is for, and what to answer, 202 Accepted when Status
// is 0. Keep are secrets a handshake handed over, by the key in the push's URL, to check its pushes with.
type Heard struct {
	Keys   []string
	Status int
	Header http.Header
	Body   string
	Keep   map[string]string
}

// ErrForged is a push that is not the provider's: unsigned, or signed with another secret.
var ErrForged = errors.New("not the provider's")

func forged(format string, a ...any) error {
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, a...), ErrForged)
}

// PushOf is the push of the connector this server has a client for, and where it pushes; nil when it has none.
func (p *Provider) PushOf() (*Push, string) { return p.Push, p.pushTo }

// HookSecret is the app's webhook secret, the hook option of the connector's variable, like ?hook=<secret>.
func (p *Provider) HookSecret() string { return p.opts.Get("hook") }

// mac is the HMAC-SHA256 of parts with key.
func mac(key string, parts ...string) []byte {
	m := hmac.New(sha256.New, []byte(key))
	for _, p := range parts {
		m.Write([]byte(p))
	}
	return m.Sum(nil)
}

// same says whether a and b are the same, in constant time; never when either is empty.
func same(a, b string) bool {
	return a != "" && b != "" && hmac.Equal([]byte(a), []byte(b))
}

// nonce is a random secret, URL-safe.
func nonce() string {
	b := make([]byte, 24)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func hexMAC(key string, parts ...string) string { return hex.EncodeToString(mac(key, parts...)) }
func b64MAC(key string, parts ...string) string {
	return base64.StdEncoding.EncodeToString(mac(key, parts...))
}

// keysOf are the distinct non-empty keys.
func keysOf(keys ...string) []string {
	var out []string
	seen := map[string]bool{}
	for _, k := range keys {
		if k != "" && !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	return out
}

func readJSON(body []byte, v any) error {
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("a push that is not JSON: %v", err)
	}
	return nil
}

// SealWatch is the secret of watch key of connector id, encrypted and bound to it, to keep.
func SealWatch(aead cipher.AEAD, id, key, secret string) []byte {
	return seal(aead, []byte(secret), "watch "+id+" "+key)
}

// OpenWatch is the secret SealWatch sealed for watch key of connector id.
func OpenWatch(aead cipher.AEAD, id, key string, sealed []byte) (string, error) {
	b, err := open(aead, sealed, "watch "+id+" "+key)
	return string(b), err
}
