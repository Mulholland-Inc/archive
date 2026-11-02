package connect

// The pushes of the API connectors, each as its provider's docs have it now.

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// gmailPush is users.watch to the Pub/Sub topic ARCHIVE_GMAIL_TOPIC names, which gmail-api-push@
// system.gserviceaccount.com publishes to and a push subscription delivers to Archive, signed by the tasks
// account. A watch lasts 7 days, and Google asks that it be renewed daily: the first sync a day after renews it.
// A push's message is { emailAddress, historyId }, the mailbox it is for.
var gmailPush = &Push{Env: "ARCHIVE_GMAIL_TOPIC", OIDC: true, Renew: 6 * 24 * time.Hour,
	Watch: func(c *Client, h Hook, old []Watch) ([]Watch, error) {
		var w struct {
			Expiration string `json:"expiration"`
		}
		if err := c.Do("POST", gmailAPI+"/watch", map[string]any{"topicName": h.Target}, &w); err != nil {
			return nil, err
		}
		return []Watch{{Key: strings.ToLower(h.Account), Expires: stamp(w.Expiration)}}, nil
	},
	Unwatch: func(c *Client, w Watch) error { return c.Do("POST", gmailAPI+"/stop", nil, nil) },
	Hear: func(p *Provider, r *http.Request, at string, body []byte, secret func(string) string) (*Heard, error) {
		var env struct {
			Message struct {
				Data string `json:"data"`
			} `json:"message"`
		}
		if err := readJSON(body, &env); err != nil {
			return nil, err
		}
		data, err := base64.StdEncoding.DecodeString(env.Message.Data)
		if err != nil {
			return nil, fmt.Errorf("a push whose data is not base64: %v", err)
		}
		var m struct {
			EmailAddress string `json:"emailAddress"`
		}
		if err := readJSON(data, &m); err != nil {
			return nil, err
		}
		return &Heard{Keys: keysOf(strings.ToLower(m.EmailAddress))}, nil
	}}

// drivePush is changes.watch, a channel of Archive's own id that pushes to Archive with its token, for a week
// at most; the first sync a day after replaces it, and stops the one before. A push says its channel in
// X-Goog-Channel-ID and its token in X-Goog-Channel-Token; the sync push of a new channel says nothing changed.
var drivePush = &Push{Renew: 6 * 24 * time.Hour,
	Watch: func(c *Client, h Hook, old []Watch) ([]Watch, error) {
		var t struct {
			Token string `json:"startPageToken"`
		}
		if err := c.Get(driveAPI+"/changes/startPageToken?supportsAllDrives=true", &t); err != nil {
			return nil, err
		}
		id, token := "archive-"+nonce()[:32], nonce()
		var ch struct {
			ResourceID string `json:"resourceId"`
			Expiration string `json:"expiration"`
		}
		q := url.Values{"pageToken": {t.Token}, "supportsAllDrives": {"true"}, "includeItemsFromAllDrives": {"true"}}
		body := map[string]any{"id": id, "type": "web_hook", "address": h.URL(id), "token": token,
			"expiration": strconv.FormatInt(time.Now().Add(7*24*time.Hour-time.Hour).UnixMilli(), 10)}
		if err := c.Do("POST", driveAPI+"/changes/watch?"+q.Encode(), body, &ch); err != nil {
			return nil, err
		}
		for _, o := range old {
			c.Do("POST", driveAPI+"/channels/stop", map[string]string{"id": o.Key, "resourceId": o.Resource}, nil)
		}
		return []Watch{{Key: id, Resource: ch.ResourceID, Secret: token, Expires: stamp(ch.Expiration)}}, nil
	},
	Unwatch: func(c *Client, w Watch) error {
		return c.Do("POST", driveAPI+"/channels/stop", map[string]string{"id": w.Key, "resourceId": w.Resource}, nil)
	},
	Hear: func(p *Provider, r *http.Request, at string, body []byte, secret func(string) string) (*Heard, error) {
		id := r.Header.Get("X-Goog-Channel-ID")
		kept := secret(id)
		switch {
		case kept == "":
			return &Heard{}, nil
		case !same(kept, r.Header.Get("X-Goog-Channel-Token")):
			return nil, forged("channel %s's token", id)
		case r.Header.Get("X-Goog-Resource-State") == "sync":
			return &Heard{}, nil
		}
		return &Heard{Keys: []string{id}}, nil
	}}

// outlookPush is a Graph subscription to every message of the mailbox, me/messages, with a clientState of
// Archive's own, for 10,020 minutes, under the 10,080 Outlook allows; the first sync a day after renews it with
// a PATCH, or makes another when Graph no longer has it. Graph checks the URL first by posting a
// validationToken, which Archive answers as it came, in plain text.
var outlookPush = &Push{Renew: 6 * 24 * time.Hour,
	Watch: func(c *Client, h Hook, old []Watch) ([]Watch, error) {
		until := time.Now().Add(10020 * time.Minute).UTC()
		for _, o := range old {
			var s struct {
				Expiration string `json:"expirationDateTime"`
			}
			err := c.Do("PATCH", graphAPI+"/subscriptions/"+url.PathEscape(o.Key), map[string]string{"expirationDateTime": until.Format(time.RFC3339)}, &s)
			if err == nil {
				o.Expires = stamp(s.Expiration)
				return []Watch{o}, nil
			}
			if r, ok := err.(*Refusal); !ok || r.Status != http.StatusNotFound {
				return nil, err
			}
		}
		state := nonce()[:32]
		var s struct {
			ID         string `json:"id"`
			Expiration string `json:"expirationDateTime"`
		}
		if err := c.Do("POST", graphAPI+"/subscriptions", map[string]string{"changeType": "created,updated,deleted", "notificationUrl": h.URL(""),
			"resource": "me/messages", "expirationDateTime": until.Format(time.RFC3339), "clientState": state}, &s); err != nil {
			return nil, err
		}
		return []Watch{{Key: s.ID, Secret: state, Expires: stamp(s.Expiration)}}, nil
	},
	Unwatch: func(c *Client, w Watch) error {
		return c.Do("DELETE", graphAPI+"/subscriptions/"+url.PathEscape(w.Key), nil, nil)
	},
	Hear: func(p *Provider, r *http.Request, at string, body []byte, secret func(string) string) (*Heard, error) {
		if v := r.URL.Query().Get("validationToken"); v != "" {
			return &Heard{Status: http.StatusOK, Header: http.Header{"Content-Type": {"text/plain; charset=utf-8"}}, Body: v}, nil
		}
		var n struct {
			Value []struct {
				SubscriptionID string `json:"subscriptionId"`
				ClientState    string `json:"clientState"`
			} `json:"value"`
		}
		if err := readJSON(body, &n); err != nil {
			return nil, err
		}
		var keys []string
		for _, v := range n.Value {
			switch kept := secret(v.SubscriptionID); {
			case kept == "":
			case !same(kept, v.ClientState):
				return nil, forged("subscription %s's clientState", v.SubscriptionID)
			default:
				keys = append(keys, v.SubscriptionID)
			}
		}
		return &Heard{Keys: keysOf(keys...)}, nil
	}}

// githubPush is the GitHub App's webhook, set up once on the app: its URL <API>/push/github, its secret the hook
// option of ARCHIVE_GITHUB_APP, and its events Issues, Pull request and Repository. A push names the
// installation it is for, and an account is the installations its token reaches, read again on a sync a day
// after. X-Hub-Signature-256 is sha256= and the HMAC-SHA256 of the body with the secret, in hex.
var githubPush = &Push{Renew: 24 * time.Hour,
	Watch: func(c *Client, h Hook, old []Watch) ([]Watch, error) {
		var out []Watch
		for page := 1; ; page++ {
			var in struct {
				Installations []struct {
					ID int64 `json:"id"`
				} `json:"installations"`
			}
			if err := c.Get(fmt.Sprintf("%s/user/installations?per_page=100&page=%d", githubAPI, page), &in); err != nil {
				return nil, err
			}
			for _, i := range in.Installations {
				out = append(out, Watch{Key: strconv.FormatInt(i.ID, 10), Expires: time.Now().Add(48 * time.Hour)})
			}
			if len(in.Installations) < 100 {
				return out, nil
			}
		}
	},
	Hear: func(p *Provider, r *http.Request, at string, body []byte, secret func(string) string) (*Heard, error) {
		if p.HookSecret() == "" || !same(r.Header.Get("X-Hub-Signature-256"), "sha256="+hexMAC(p.HookSecret(), string(body))) {
			return nil, forged("X-Hub-Signature-256")
		}
		var e struct {
			Installation struct {
				ID int64 `json:"id"`
			} `json:"installation"`
		}
		if err := readJSON(body, &e); err != nil {
			return nil, err
		}
		if e.Installation.ID == 0 {
			return &Heard{}, nil
		}
		return &Heard{Keys: []string{strconv.FormatInt(e.Installation.ID, 10)}}, nil
	}}

// hubspotPush is the HubSpot app's webhooks, set up once on the app: its target URL <API>/push/hubspot, and
// subscriptions to the creation, deletion and property changes of contacts, companies and deals. A push is an
// array of events, each with the portalId of the account it is for, the hub_id of the account's token.
// X-HubSpot-Signature-v3 is the base64 HMAC-SHA256, with the app's client secret, of the method, the URL, the
// body and X-HubSpot-Request-Timestamp, which must be under 5 minutes old; an app that still signs v1 or v2,
// the hex SHA-256 of the secret and the body, or of the secret, method, URL and body, is checked that way.
var hubspotPush = &Push{
	Watch: func(c *Client, h Hook, old []Watch) ([]Watch, error) {
		id, secret := c.Credentials()
		var t struct {
			HubID int64 `json:"hub_id"`
		}
		if err := c.Do("POST", "https://api.hubspot.com/oauth/v3/token/introspect",
			url.Values{"client_id": {id}, "client_secret": {secret}, "token_type_hint": {"access_token"}, "access_token": {c.Token()}}, &t); err != nil {
			return nil, err
		}
		if t.HubID == 0 {
			return nil, fmt.Errorf("HubSpot named no hub of the token")
		}
		return []Watch{{Key: strconv.FormatInt(t.HubID, 10)}}, nil
	},
	Hear: func(p *Provider, r *http.Request, at string, body []byte, secret func(string) string) (*Heard, error) {
		key := p.secret
		uri := strings.NewReplacer("%3A", ":", "%2F", "/", "%3F", "?", "%40", "@", "%21", "!", "%24", "$", "%27", "'", "%28", "(", "%29", ")",
			"%2A", "*", "%2C", ",", "%3B", ";").Replace(at)
		switch v := r.Header.Get("X-HubSpot-Signature-Version"); {
		case r.Header.Get("X-HubSpot-Signature-v3") != "":
			ts := r.Header.Get("X-HubSpot-Request-Timestamp")
			ms, err := strconv.ParseInt(ts, 10, 64)
			if err != nil || time.Since(time.UnixMilli(ms)).Abs() > 5*time.Minute {
				return nil, forged("X-HubSpot-Request-Timestamp %q", ts)
			}
			if !same(r.Header.Get("X-HubSpot-Signature-v3"), b64MAC(key, r.Method, uri, string(body), ts)) {
				return nil, forged("X-HubSpot-Signature-v3")
			}
		case v == "v1" && same(r.Header.Get("X-HubSpot-Signature"), sha256Hex(key+string(body))):
		case v == "v2" && same(r.Header.Get("X-HubSpot-Signature"), sha256Hex(key+r.Method+uri+string(body))):
		default:
			return nil, forged("X-HubSpot-Signature")
		}
		var events []struct {
			PortalID int64 `json:"portalId"`
		}
		if err := readJSON(body, &events); err != nil {
			return nil, err
		}
		var keys []string
		for _, e := range events {
			keys = append(keys, strconv.FormatInt(e.PortalID, 10))
		}
		return &Heard{Keys: keysOf(keys...)}, nil
	}}

// quickbooksPush is the Intuit app's webhooks, set up once for each of its environments: its endpoint
// <API>/push/quickbooks, the entities QuickBooks streams, and its verifier token the hook option of
// ARCHIVE_QUICKBOOKS. intuit-signature is the base64 HMAC-SHA256 of the body with the verifier token. A push is
// an array of CloudEvents, each with intuitaccountid, the realmId of the company it is for; one of the format
// before, { eventNotifications: [{ realmId }] }, is read too.
var quickbooksPush = &Push{
	Watch: func(c *Client, h Hook, old []Watch) ([]Watch, error) {
		return []Watch{{Key: c.Meta["realmId"]}}, nil
	},
	Hear: func(p *Provider, r *http.Request, at string, body []byte, secret func(string) string) (*Heard, error) {
		if p.HookSecret() == "" || !same(r.Header.Get("intuit-signature"), b64MAC(p.HookSecret(), string(body))) {
			return nil, forged("intuit-signature")
		}
		var keys []string
		if t := strings.TrimSpace(string(body)); strings.HasPrefix(t, "[") {
			var events []struct {
				Account string `json:"intuitaccountid"`
			}
			if err := readJSON(body, &events); err != nil {
				return nil, err
			}
			for _, e := range events {
				keys = append(keys, e.Account)
			}
		} else {
			var n struct {
				Notifications []struct {
					RealmID string `json:"realmId"`
				} `json:"eventNotifications"`
			}
			if err := readJSON(body, &n); err != nil {
				return nil, err
			}
			for _, e := range n.Notifications {
				keys = append(keys, e.RealmID)
			}
		}
		return &Heard{Keys: keysOf(keys...)}, nil
	}}

// asanaPush is a webhook on each project of the account that is not archived, pushing to Archive's URL with a
// key of the connection's, the project and a nonce, which only Asana learns. Asana creates one only once its
// handshake, posted to that URL with X-Hook-Secret, is answered with the same header; Archive keeps that secret
// by the key, and a push after is signed with it: X-Hook-Signature, the hex HMAC-SHA256 of the body. Asana
// deletes a webhook that fails for a day, so the first sync a week after checks each is still there. A push
// without events is Asana's heartbeat, and starts nothing.
var asanaPush = &Push{Renew: 6 * 24 * time.Hour,
	Watch: func(c *Client, h Hook, old []Watch) ([]Watch, error) {
		have := map[string]Watch{}
		for _, o := range old {
			var w struct {
				Data struct {
					Active bool `json:"active"`
				} `json:"data"`
			}
			if err := c.Get(asanaAPI+"/webhooks/"+url.PathEscape(o.Resource)+"?opt_fields=active", &w); err == nil && w.Data.Active {
				o.Expires = time.Now().Add(7 * 24 * time.Hour)
				have[asanaProject(o.Key)] = o
			}
		}
		ws, err := asanaAll(c, "/workspaces?opt_fields=name")
		if err != nil {
			return nil, err
		}
		var out []Watch
		for _, w := range ws {
			ps, err := asanaAll(c, "/projects?archived=false&opt_fields=name&workspace="+url.QueryEscape(str(w, "gid")))
			if err != nil {
				return nil, err
			}
			for _, p := range ps {
				gid := str(p, "gid")
				if o, ok := have[gid]; ok {
					out = append(out, o)
					continue
				}
				key := h.ID + "/" + gid + "/" + nonce()[:16]
				var made struct {
					Data struct {
						GID string `json:"gid"`
					} `json:"data"`
				}
				if err := c.Do("POST", asanaAPI+"/webhooks", map[string]any{"data": map[string]string{"resource": gid, "target": h.URL(key)}}, &made); err != nil {
					return out, err
				}
				out = append(out, Watch{Key: key, Resource: made.Data.GID, Expires: time.Now().Add(7 * 24 * time.Hour)})
			}
		}
		return out, nil
	},
	Unwatch: func(c *Client, w Watch) error {
		return c.Do("DELETE", asanaAPI+"/webhooks/"+url.PathEscape(w.Resource), nil, nil)
	},
	Hear: func(p *Provider, r *http.Request, at string, body []byte, secret func(string) string) (*Heard, error) {
		key := r.URL.Query().Get("k")
		if s := r.Header.Get("X-Hook-Secret"); s != "" {
			if key == "" || secret(key) != "" {
				return nil, forged("a handshake for a webhook that has one")
			}
			return &Heard{Status: http.StatusOK, Header: http.Header{"X-Hook-Secret": {s}}, Keep: map[string]string{key: s}}, nil
		}
		kept := secret(key)
		if kept == "" {
			return &Heard{}, nil
		}
		if !same(r.Header.Get("X-Hook-Signature"), hexMAC(kept, string(body))) {
			return nil, forged("X-Hook-Signature")
		}
		var e struct {
			Events []any `json:"events"`
		}
		if err := readJSON(body, &e); err != nil {
			return nil, err
		}
		if len(e.Events) == 0 {
			return &Heard{}, nil
		}
		return &Heard{Keys: []string{key}}, nil
	}}

// asanaProject is the project of an Asana watch's key, <connection>/<project>/<nonce>.
func asanaProject(key string) string {
	parts := strings.Split(key, "/")
	if len(parts) != 3 {
		return ""
	}
	return parts[1]
}

func sha256Hex(s string) string { return fmt.Sprintf("%x", sha256Sum([]byte(s))) }
