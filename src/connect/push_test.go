package connect

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

// provider is connector id with a client whose variable has opts, like ?hook=<secret>.
func provider(t *testing.T, id, opts string) *Provider {
	t.Helper()
	p, err := load(t, map[string]string{registry[id].Env: "https://app:shh@example.com" + opts}).Get(id)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// hear is what p's Hear made of a push of body with hdr to url, with the watch secrets kept.
func hear(t *testing.T, p *Provider, url, body string, hdr map[string]string, kept map[string]string) (*Heard, error) {
	t.Helper()
	r := httptest.NewRequest("POST", url, strings.NewReader(body))
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	return p.Push.Hear(p, r, url, []byte(body), func(k string) string { return kept[k] })
}

// Each provider's push is heard only as its provider signs it, and names the watches it is for; one for a
// watch Archive does not have is heard, and names none.
func TestPushesAreHeardOnlyAsTheirProviderSignsThem(t *testing.T) {
	type try struct {
		what, url, body string
		hdr             map[string]string
		keys            string // the keys heard, or "forged"
	}
	now := fmt.Sprint(time.Now().UnixMilli())
	old := fmt.Sprint(time.Now().Add(-10 * time.Minute).UnixMilli())
	hub := `[{"portalId":62515,"objectId":1,"subscriptionType":"contact.creation"},{"portalId":62515}]`
	hubURL := "https://api.archive.test/push/hubspot"
	qbo := `[{"specversion":"1.0","type":"qbo.invoice.created.v1","intuitaccountid":"9130"},{"intuitaccountid":"77"}]`
	qboOld := `{"eventNotifications":[{"realmId":"9130","dataChangeEvent":{"entities":[]}}]}`
	gh := `{"action":"opened","installation":{"id":42}}`
	asana := `{"events":[{"action":"changed"}]}`
	gmailData := base64.StdEncoding.EncodeToString([]byte(`{"emailAddress":"Alice@Gmail.com","historyId":"9"}`))
	kept := map[string]string{"ch1": "tok1", "sub1": "state1", "c/p/n": "hs"}
	for id, tries := range map[string][]try{
		"github": {
			{"signed", "/push/github", gh, map[string]string{"X-Hub-Signature-256": "sha256=" + hexMAC("gh-secret", gh)}, "42"},
			{"signed with another secret", "/push/github", gh, map[string]string{"X-Hub-Signature-256": "sha256=" + hexMAC("other", gh)}, "forged"},
			{"unsigned", "/push/github", gh, nil, "forged"},
			{"a ping, of no installation", "/push/github", `{"zen":"hi"}`, map[string]string{"X-Hub-Signature-256": "sha256=" + hexMAC("gh-secret", `{"zen":"hi"}`)}, ""},
		},
		"hubspot": {
			{"v3", hubURL, hub, map[string]string{"X-HubSpot-Signature-v3": b64MAC("shh", "POST", hubURL, hub, now), "X-HubSpot-Request-Timestamp": now}, "62515"},
			{"v3 too old", hubURL, hub, map[string]string{"X-HubSpot-Signature-v3": b64MAC("shh", "POST", hubURL, hub, old), "X-HubSpot-Request-Timestamp": old}, "forged"},
			{"v3 of another body", hubURL, hub, map[string]string{"X-HubSpot-Signature-v3": b64MAC("shh", "POST", hubURL, "[]", now), "X-HubSpot-Request-Timestamp": now}, "forged"},
			{"v1", hubURL, hub, map[string]string{"X-HubSpot-Signature": sha256Hex("shh" + hub), "X-HubSpot-Signature-Version": "v1"}, "62515"},
			{"v1 of another secret", hubURL, hub, map[string]string{"X-HubSpot-Signature": sha256Hex("x" + hub), "X-HubSpot-Signature-Version": "v1"}, "forged"},
			{"unsigned", hubURL, hub, nil, "forged"},
		},
		"quickbooks": {
			{"CloudEvents", "/push/quickbooks", qbo, map[string]string{"intuit-signature": b64MAC("verifier", qbo)}, "9130 77"},
			{"the format before", "/push/quickbooks", qboOld, map[string]string{"intuit-signature": b64MAC("verifier", qboOld)}, "9130"},
			{"signed with another token", "/push/quickbooks", qbo, map[string]string{"intuit-signature": b64MAC("other", qbo)}, "forged"},
		},
		"gdrive": {
			{"a change", "/push/gdrive", "", map[string]string{"X-Goog-Channel-ID": "ch1", "X-Goog-Channel-Token": "tok1", "X-Goog-Resource-State": "change"}, "ch1"},
			{"a new channel's sync", "/push/gdrive", "", map[string]string{"X-Goog-Channel-ID": "ch1", "X-Goog-Channel-Token": "tok1", "X-Goog-Resource-State": "sync"}, ""},
			{"another token", "/push/gdrive", "", map[string]string{"X-Goog-Channel-ID": "ch1", "X-Goog-Channel-Token": "tok2"}, "forged"},
			{"a channel Archive does not have", "/push/gdrive", "", map[string]string{"X-Goog-Channel-ID": "gone", "X-Goog-Channel-Token": "x"}, ""},
		},
		"outlook": {
			{"a change", "/push/outlook", `{"value":[{"subscriptionId":"sub1","clientState":"state1"},{"subscriptionId":"gone","clientState":"x"}]}`, nil, "sub1"},
			{"another clientState", "/push/outlook", `{"value":[{"subscriptionId":"sub1","clientState":"state2"}]}`, nil, "forged"},
		},
		"asana": {
			{"an event", "/push/asana?k=c%2Fp%2Fn", asana, map[string]string{"X-Hook-Signature": hexMAC("hs", asana)}, "c/p/n"},
			{"a heartbeat", "/push/asana?k=c%2Fp%2Fn", `{"events":[]}`, map[string]string{"X-Hook-Signature": hexMAC("hs", `{"events":[]}`)}, ""},
			{"another signature", "/push/asana?k=c%2Fp%2Fn", asana, map[string]string{"X-Hook-Signature": hexMAC("other", asana)}, "forged"},
			{"a second handshake", "/push/asana?k=c%2Fp%2Fn", "", map[string]string{"X-Hook-Secret": "evil"}, "forged"},
			{"a webhook Archive does not have", "/push/asana?k=x", asana, map[string]string{"X-Hook-Signature": hexMAC("hs", asana)}, ""},
		},
		"gmail": {
			{"a change", "/push/gmail", `{"message":{"data":"` + gmailData + `"},"subscription":"s"}`, nil, "alice@gmail.com"},
		},
	} {
		opts := map[string]string{"github": "?hook=gh-secret", "quickbooks": "?hook=verifier"}[id]
		p := provider(t, id, opts)
		for _, tr := range tries {
			h, err := hear(t, p, tr.url, tr.body, tr.hdr, kept)
			got := ""
			switch {
			case errors.Is(err, ErrForged):
				got = "forged"
			case err != nil:
				got = err.Error()
			default:
				got = strings.Join(h.Keys, " ")
			}
			if got != tr.keys {
				t.Errorf("%s, %s: %q, want %q", id, tr.what, got, tr.keys)
			}
		}
	}
	// An app without its webhook secret hears nothing.
	if _, err := hear(t, provider(t, "github", ""), "/push/github", gh, map[string]string{"X-Hub-Signature-256": "sha256=" + hexMAC("", gh)}, nil); !errors.Is(err, ErrForged) {
		t.Errorf("a GitHub app without its secret: %v", err)
	}
	// Graph's validation and Asana's handshake are answered as they ask; Asana's secret is kept by its key.
	h, err := hear(t, provider(t, "outlook", ""), "/push/outlook?validationToken=Validation%3A+abc", "", nil, nil)
	if err != nil || h.Status != 200 || h.Body != "Validation: abc" || !strings.HasPrefix(h.Header.Get("Content-Type"), "text/plain") || len(h.Keys) != 0 {
		t.Errorf("Graph's validation: %+v %v", h, err)
	}
	h, err = hear(t, provider(t, "asana", ""), "/push/asana?k=c%2Fp%2Fnew", "", map[string]string{"X-Hook-Secret": "s3"}, kept)
	if err != nil || h.Status != 200 || h.Header.Get("X-Hook-Secret") != "s3" || h.Keep["c/p/new"] != "s3" {
		t.Errorf("Asana's handshake: %+v %v", h, err)
	}
	// GitHub's documented test vector.
	if got := hexMAC("It's a Secret to Everybody", "Hello, World!"); got != "757107ea0eb2509fc211221cce984b8a37570b6d7586c22c46f4379c8b043e17" {
		t.Errorf("HMAC-SHA256: %s", got)
	}
}

// Each provider's Watch asks it as its docs have it, and renews or replaces what it had.
func TestWatchesAskTheirProviders(t *testing.T) {
	hook := Hook{ID: "conn", Account: "Alice@Gmail.com", Target: "projects/p/topics/gmail", URL: func(k string) string { return "https://api.archive.test/push/x?k=" + k }}
	reply := func(w http.ResponseWriter, v any) { json.NewEncoder(w).Encode(v) }

	g, seen := standIn(t, "gmail", func(w http.ResponseWriter, r *http.Request, body string) {
		reply(w, map[string]string{"historyId": "1", "expiration": "1767225600000"})
	})
	ws, err := g.p.Push.Watch(g, hook, nil)
	if err != nil || len(ws) != 1 || ws[0].Key != "alice@gmail.com" || !ws[0].Expires.Equal(time.UnixMilli(1767225600000)) || (*seen)[0] != `POST /gmail/v1/users/me/watch {"topicName":"projects/p/topics/gmail"}` {
		t.Errorf("Gmail's watch: %+v %v %q", ws, err, *seen)
	}

	d, seen := standIn(t, "gdrive", func(w http.ResponseWriter, r *http.Request, body string) {
		switch r.URL.Path {
		case "/drive/v3/changes/startPageToken":
			reply(w, map[string]string{"startPageToken": "s9"})
		case "/drive/v3/changes/watch":
			reply(w, map[string]string{"resourceId": "res2", "expiration": "1767225600000"})
		default:
			reply(w, map[string]string{})
		}
	})
	ws, err = d.p.Push.Watch(d, hook, []Watch{{Key: "old", Resource: "res1"}})
	if err != nil || len(ws) != 1 || ws[0].Resource != "res2" || len(ws[0].Secret) < 20 || !strings.HasPrefix(ws[0].Key, "archive-") {
		t.Fatalf("Drive's watch: %+v %v", ws, err)
	}
	if w := (*seen)[1]; !strings.Contains(w, "pageToken=s9") || !strings.Contains(w, `"address":"https://api.archive.test/push/x?k=`+ws[0].Key+`"`) ||
		!strings.Contains(w, `"type":"web_hook"`) || !strings.Contains(w, `"token":"`+ws[0].Secret+`"`) {
		t.Errorf("Drive's changes.watch: %s", w)
	}
	if stop := (*seen)[2]; stop != `POST /drive/v3/channels/stop {"id":"old","resourceId":"res1"}` {
		t.Errorf("the channel before was not stopped: %s", stop)
	}

	gone := true
	o, seen := standIn(t, "outlook", func(w http.ResponseWriter, r *http.Request, body string) {
		switch {
		case r.Method == "PATCH" && gone:
			w.WriteHeader(404)
			reply(w, map[string]any{"error": map[string]string{"code": "ResourceNotFound"}})
		case r.Method == "PATCH":
			reply(w, map[string]string{"id": "sub1", "expirationDateTime": "2026-11-09T00:00:00Z"})
		default:
			reply(w, map[string]string{"id": "sub2", "expirationDateTime": "2026-11-09T00:00:00Z"})
		}
	})
	ws, err = o.p.Push.Watch(o, hook, []Watch{{Key: "sub1", Secret: "state1"}})
	if err != nil || len(ws) != 1 || ws[0].Key != "sub2" || len(ws[0].Secret) != 32 {
		t.Fatalf("Graph's subscription made again: %+v %v", ws, err)
	}
	if c := (*seen)[1]; !strings.Contains(c, `"resource":"me/messages"`) || !strings.Contains(c, `"changeType":"created,updated,deleted"`) || !strings.Contains(c, `"clientState":"`+ws[0].Secret+`"`) {
		t.Errorf("Graph's subscription: %s", c)
	}
	gone = false
	ws, err = o.p.Push.Watch(o, hook, []Watch{{Key: "sub1", Secret: "state1"}})
	if err != nil || len(ws) != 1 || ws[0].Key != "sub1" || ws[0].Secret != "state1" || !strings.HasPrefix((*seen)[2], "PATCH /v1.0/subscriptions/sub1") {
		t.Errorf("Graph's subscription renewed: %+v %v %q", ws, err, *seen)
	}

	a, seen := standIn(t, "asana", func(w http.ResponseWriter, r *http.Request, body string) {
		switch r.URL.Path {
		case "/api/1.0/webhooks/w1":
			reply(w, map[string]any{"data": map[string]any{"active": true}})
		case "/api/1.0/webhooks/w2":
			w.WriteHeader(404)
			reply(w, map[string]any{"errors": []any{}})
		case "/api/1.0/workspaces":
			reply(w, map[string]any{"data": []any{map[string]string{"gid": "ws"}}})
		case "/api/1.0/projects":
			reply(w, map[string]any{"data": []any{map[string]string{"gid": "p1"}, map[string]string{"gid": "p2"}}})
		case "/api/1.0/webhooks":
			reply(w, map[string]any{"data": map[string]string{"gid": "w3"}})
		}
	})
	ws, err = a.p.Push.Watch(a, hook, []Watch{{Key: "conn/p1/n1", Resource: "w1"}, {Key: "conn/p2/n2", Resource: "w2"}})
	if err != nil || len(ws) != 2 || ws[0].Key != "conn/p1/n1" || ws[1].Resource != "w3" || !strings.HasPrefix(ws[1].Key, "conn/p2/") || ws[1].Key == "conn/p2/n2" {
		t.Errorf("Asana's webhooks: %+v %v", ws, err)
	}
	if !slices.ContainsFunc(*seen, func(s string) bool {
		return strings.HasPrefix(s, "POST /api/1.0/webhooks ") && strings.Contains(s, `"resource":"p2"`) && strings.Contains(s, `"target":"https://api.archive.test/push/x?k=conn/p2/`)
	}) {
		t.Errorf("Asana was not asked for p2's webhook: %q", *seen)
	}

	gh, _ := standIn(t, "github", func(w http.ResponseWriter, r *http.Request, body string) {
		reply(w, map[string]any{"total_count": 2, "installations": []any{map[string]any{"id": 7}, map[string]any{"id": 8}}})
	})
	if ws, err = gh.p.Push.Watch(gh, hook, nil); err != nil || len(ws) != 2 || ws[0].Key != "7" || ws[1].Key != "8" {
		t.Errorf("GitHub's installations: %+v %v", ws, err)
	}
	hs, seen := standIn(t, "hubspot", func(w http.ResponseWriter, r *http.Request, body string) {
		reply(w, map[string]any{"hub_id": 62515, "user": "a@b"})
	})
	if ws, err = hs.p.Push.Watch(hs, hook, nil); err != nil || len(ws) != 1 || ws[0].Key != "62515" || !strings.Contains((*seen)[0], "access_token=at") {
		t.Errorf("HubSpot's hub: %+v %v", ws, err)
	}
	q, _ := standIn(t, "quickbooks", func(w http.ResponseWriter, r *http.Request, body string) {})
	if ws, err = q.p.Push.Watch(q, hook, nil); err != nil || len(ws) != 1 || ws[0].Key != "9130" {
		t.Errorf("QuickBooks' company: %+v %v", ws, err)
	}
}
