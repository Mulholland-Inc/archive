package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Mulholland-Inc/archive/connect"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

const gmailScope = "https://www.googleapis.com/auth/gmail.readonly"

// fakeGoogle stands in for Google's token, revoke and Gmail profile endpoints. A code is the mailbox address
// it connects, or noscope:<address> for a consent that left gmail.readonly unticked.
type fakeGoogle struct {
	srv *httptest.Server

	mu       sync.Mutex
	n        int
	refresh  map[string]string // live refresh tokens, to their mailbox
	access   map[string]string // access tokens, to their mailbox
	rotate   bool              // a refresh replaces the refresh token
	verifier string            // the last PKCE verifier a code came with
	searched string            // the last search, q and maxResults
	revoked  []string

	// A mailbox to sync: its messages, by id to their subject; its historyId, and the history of each id
	// added, changed or deleted after the first that history still reaches. fail is how many Gmail API
	// requests from now fail with a 500.
	box     map[string]string
	hist    int
	history []fakeChange
	oldest  int
	fail    int
	// stuck makes history.list answer that there is more, from the page it was asked for. watched are the
	// topics users.watch was asked for, and stops how many times users.stop was called.
	stuck   bool
	watched []string
	stops   int
}

// fakeChange is a change of the mailbox's history: its historyId, the message, and what happened to it, as
// the key history.list lists it under.
type fakeChange struct {
	h        int
	id, kind string
}

// mail adds message id to the mailbox, or relabels it with a new subject, with its history.
func (g *fakeGoogle) mail(id, subject string) {
	kind := "messagesAdded"
	if _, had := g.box[id]; had {
		kind = "labelsAdded"
	}
	g.hist++
	g.box[id] = subject
	g.history = append(g.history, fakeChange{h: g.hist, id: id, kind: kind})
}

// unmail deletes message id from the mailbox, with its history.
func (g *fakeGoogle) unmail(id string) {
	g.hist++
	delete(g.box, id)
	g.history = append(g.history, fakeChange{h: g.hist, id: id, kind: "messagesDeleted"})
}

func newFakeGoogle(t *testing.T) *fakeGoogle {
	g := &fakeGoogle{refresh: map[string]string{}, access: map[string]string{}, box: map[string]string{}, hist: 1}
	g.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.mu.Lock()
		defer g.mu.Unlock()
		r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		refuse := func(code int, e string) {
			w.WriteHeader(code)
			json.NewEncoder(w).Encode(map[string]string{"error": e, "error_description": "Token has been expired or revoked."})
		}
		switch r.URL.Path {
		case "/token":
			if r.Form.Get("client_id") != "web" || r.Form.Get("client_secret") != "shh" {
				refuse(401, "invalid_client")
				return
			}
			g.n++
			at := fmt.Sprintf("at-%d", g.n)
			out := map[string]any{"access_token": at, "token_type": "Bearer", "expires_in": 3600}
			var mailbox string
			switch r.Form.Get("grant_type") {
			case "authorization_code":
				scope := gmailScope
				mailbox = r.Form.Get("code")
				if m, ok := strings.CutPrefix(mailbox, "noscope:"); ok {
					mailbox, scope = m, "openid"
				}
				g.verifier = r.Form.Get("code_verifier")
				rt := fmt.Sprintf("rt-%d", g.n)
				g.refresh[rt] = mailbox
				out["refresh_token"], out["scope"] = rt, scope
			case "refresh_token":
				rt := r.Form.Get("refresh_token")
				var ok bool
				if mailbox, ok = g.refresh[rt]; !ok {
					refuse(400, "invalid_grant")
					return
				}
				if g.rotate {
					delete(g.refresh, rt)
					next := fmt.Sprintf("rt-%d", g.n)
					g.refresh[next] = mailbox
					out["refresh_token"] = next
				}
			default:
				refuse(400, "unsupported_grant_type")
				return
			}
			g.access[at] = mailbox
			json.NewEncoder(w).Encode(out)
		case "/gmail/v1/users/me/profile":
			mailbox, ok := g.access[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")]
			if !ok {
				refuse(401, "invalid_token")
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"emailAddress": mailbox, "historyId": fmt.Sprint(g.hist)})
		case "/gmail/v1/users/me/history":
			if g.serveMailbox(w, r) {
				return
			}
			start, _ := strconv.Atoi(r.Form.Get("startHistoryId"))
			if start < g.oldest {
				w.WriteHeader(404)
				w.Write([]byte(`{"error":{"code":404,"message":"Requested entity was not found."}}`))
				return
			}
			if g.stuck {
				json.NewEncoder(w).Encode(map[string]any{"history": []any{}, "historyId": fmt.Sprint(g.hist), "nextPageToken": "same"})
				return
			}
			var hs []map[string]any
			for _, c := range g.history {
				if c.h > start {
					hs = append(hs, map[string]any{"id": fmt.Sprint(c.h), c.kind: []any{map[string]any{"message": map[string]string{"id": c.id}}}})
				}
			}
			json.NewEncoder(w).Encode(map[string]any{"history": hs, "historyId": fmt.Sprint(g.hist)})
		case "/gmail/v1/users/me/messages", "/gmail/v1/users/me/messages/m1", "/gmail/v1/users/me/messages/gone":
			if _, ok := g.access[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")]; !ok {
				refuse(401, "invalid_token")
				return
			}
			switch {
			case strings.HasSuffix(r.URL.Path, "/gone"):
				w.WriteHeader(404)
				w.Write([]byte(`{"error":{"code":404,"message":"Requested entity was not found.","status":"NOT_FOUND"}}`))
			case r.Form.Get("q") == "" && strings.HasSuffix(r.URL.Path, "/messages"):
				if g.serveMailbox(w, r) {
					return
				}
				ids := slices.Sorted(maps.Keys(g.box))
				from, _ := strconv.Atoi(r.Form.Get("pageToken"))
				n, _ := strconv.Atoi(r.Form.Get("maxResults"))
				out := map[string]any{"messages": []any{}}
				for _, id := range ids[from:min(from+n, len(ids))] {
					out["messages"] = append(out["messages"].([]any), map[string]string{"id": id, "threadId": "t-" + id})
				}
				if from+n < len(ids) {
					out["nextPageToken"] = fmt.Sprint(from + n)
				}
				json.NewEncoder(w).Encode(out)
			case strings.HasSuffix(r.URL.Path, "/messages"):
				g.searched = r.Form.Get("q") + " " + r.Form.Get("maxResults")
				json.NewEncoder(w).Encode(map[string]any{"messages": []map[string]string{{"id": "m1", "threadId": "t1"}}, "nextPageToken": "p2"})
			default:
				part := func(mime, name, text string) map[string]any {
					return map[string]any{"mimeType": mime, "filename": name, "body": map[string]any{"size": len(text), "data": base64.URLEncoding.EncodeToString([]byte(text))}}
				}
				json.NewEncoder(w).Encode(map[string]any{"id": "m1", "threadId": "t1", "labelIds": []string{"INBOX"}, "snippet": "Hello",
					"payload": map[string]any{"mimeType": "multipart/mixed", "headers": []map[string]string{{"name": "From", "value": "Ann <ann@example.com>"}, {"name": "Subject", "value": "Hi"}},
						"parts": []any{part("text/plain", "", "Hello Alice"), part("application/pdf", "a.pdf", "%PDF")}}})
			}
		case "/gmail/v1/users/me/watch", "/gmail/v1/users/me/stop":
			if g.serveMailbox(w, r) {
				return
			}
			if strings.HasSuffix(r.URL.Path, "/stop") {
				g.stops++
				w.Write([]byte("{}"))
				return
			}
			var body struct {
				TopicName string `json:"topicName"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			g.watched = append(g.watched, body.TopicName)
			json.NewEncoder(w).Encode(map[string]string{"historyId": fmt.Sprint(g.hist), "expiration": fmt.Sprint(time.Now().Add(7 * 24 * time.Hour).UnixMilli())})
		case "/revoke":
			g.revoked = append(g.revoked, r.Form.Get("token"))
			delete(g.refresh, r.Form.Get("token"))
			w.Write([]byte("{}"))
		default:
			id, ok := strings.CutPrefix(r.URL.Path, "/gmail/v1/users/me/messages/")
			if !ok || g.serveMailbox(w, r) {
				http.NotFound(w, r)
				return
			}
			subject, ok := g.box[id]
			if !ok {
				w.WriteHeader(404)
				w.Write([]byte(`{"error":{"code":404,"message":"Requested entity was not found."}}`))
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"id": id, "threadId": "t-" + id, "labelIds": []string{"INBOX"}, "snippet": subject, "internalDate": "1767225600000",
				"payload": map[string]any{"mimeType": "text/html", "headers": []map[string]string{{"name": "Subject", "value": subject}},
					"body": map[string]any{"data": base64.URLEncoding.EncodeToString([]byte("<p>About " + subject + "</p>"))}}})
		}
	}))
	t.Cleanup(g.srv.Close)
	return g
}

// serveMailbox answers a request of the mailbox it cannot: one without a token the fake issued, or one of
// the failures asked for.
func (g *fakeGoogle) serveMailbox(w http.ResponseWriter, r *http.Request) bool {
	switch _, ok := g.access[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")]; {
	case !ok:
		w.WriteHeader(401)
		w.Write([]byte(`{"error":{"code":401,"message":"Invalid Credentials"}}`))
	case g.fail > 0:
		g.fail--
		w.WriteHeader(500)
		w.Write([]byte(`{"error":{"code":500,"message":"Backend Error"}}`))
	default:
		return false
	}
	return true
}

func (g *fakeGoogle) env() string {
	return "ARCHIVE_GOOGLE=" + strings.Replace(g.srv.URL, "://", "://web:shh@", 1)
}

func (g *fakeGoogle) do(fn func()) {
	g.mu.Lock()
	defer g.mu.Unlock()
	fn()
}

// Connecting Gmail keeps the mailbox for whoever started it, shows it only to them, renews it on a second
// connect, and syncs it into record; disconnecting revokes it and deletes the records.
func TestGmailConnectsForWhoStartedItAndDisconnectsWithARevoke(t *testing.T) {
	t.Parallel()
	g := newFakeGoogle(t)
	edge := &liveEdge{}
	hs := httptest.NewServer(edge)
	t.Cleanup(hs.Close)
	in := newInst(t, g.env(), "ARCHIVE_LIVE_KEY=k3y", "ARCHIVE_LIVE_URL="+hs.URL, "ARCHIVE_ASANA=https://app:shh@app.asana.com")
	in.must("user", "add", "alice@example.com")
	in.must("user", "add", "bob@example.com")
	srv := in.serve()
	ws := in.psql("archive", "SELECT id FROM archive.workspace ORDER BY created_at LIMIT 1")
	alice := decode[Token](t, in.must("token", "create")).Secret
	bob := decode[Token](t, in.must("-as", "bob@example.com", "token", "create")).Secret
	view := decode[Token](t, in.must("token", "create", "-viewer")).Secret

	start := func(tok string) url.Values {
		t.Helper()
		st, hdr := call(t, "GET", srv.url+"/connect/gmail?workspace="+ws, tok, nil, nil)
		loc, err := url.Parse(hdr.Get("Location"))
		if st != 302 || err != nil || loc.Host != strings.TrimPrefix(g.srv.URL, "http://") || loc.Path != "/o/oauth2/v2/auth" {
			t.Fatalf("start connecting Gmail: %d %s", st, hdr.Get("Location"))
		}
		return loc.Query()
	}
	finish := func(tok string, q url.Values) (int, string) {
		t.Helper()
		st, hdr := call(t, "GET", srv.url+"/connect/gmail/callback?"+q.Encode(), tok, nil, nil)
		return st, hdr.Get("Location")
	}
	list := func(tok string) Connections {
		t.Helper()
		var out Connections
		if st, _ := call(t, "GET", srv.url+"/connect", tok, nil, &out); st != 200 {
			t.Fatalf("list connections: %d", st)
		}
		return out
	}
	back := "/connectors/gmail?workspace=" + ws

	q := start(alice)
	if q.Get("access_type") != "offline" || q.Get("prompt") != "consent" || q.Get("scope") != gmailScope || q.Get("client_id") != "web" ||
		q.Get("code_challenge_method") != "S256" || !strings.HasSuffix(q.Get("redirect_uri"), ":"+strings.Split(srv.url, ":")[2]+"/connect/gmail/callback") || q.Get("state") == "" {
		t.Fatalf("the consent Gmail is sent to: %v", q)
	}
	if st, _ := call(t, "GET", srv.url+"/connect/gmail?workspace="+ws, view, nil, nil); st != 403 {
		t.Errorf("a viewer-only token connects: %d", st)
	}
	if st, _ := call(t, "GET", srv.url+"/connect/gmail", "", nil, nil); st != 401 {
		t.Errorf("connecting without signing in: %d", st)
	}
	if st, _ := call(t, "GET", srv.url+"/connect/outlook", alice, nil, nil); st != 404 {
		t.Errorf("a connector there is none of: %d", st)
	}
	if st, _ := finish(bob, url.Values{"state": {q.Get("state")}, "code": {"bob@gmail.com"}}); st != 401 {
		t.Errorf("bob finishes what alice started: %d", st)
	}
	if st, _ := finish(alice, url.Values{"state": {"forged"}, "code": {"alice@gmail.com"}}); st != 401 {
		t.Errorf("a forged state: %d", st)
	}
	edge.tables()
	st, to := finish(alice, url.Values{"state": {q.Get("state")}, "code": {"Alice@Gmail.com"}})
	if st != 303 || !strings.HasSuffix(to, back) {
		t.Fatalf("alice connects her Gmail: %d %s", st, to)
	}
	g.do(func() {
		if oauth2.S256ChallengeFromVerifier(g.verifier) != q.Get("code_challenge") {
			t.Errorf("the code came without the PKCE verifier of its challenge")
		}
	})
	if got := edge.tables(); !slices.Contains(got, "archive.connection") {
		t.Errorf("connecting announced %v, want archive.connection", got)
	}

	cs := list(alice)
	if len(cs.Connections) != 1 || cs.Connections[0].Account != "alice@gmail.com" || cs.Connections[0].Provider != "gmail" ||
		!slices.Equal(cs.Connections[0].Scopes, []string{gmailScope}) || cs.Connections[0].Error != "" ||
		!slices.Contains(cs.Connectors, Connector{ID: "gmail", Name: "Gmail", Ready: true}) {
		t.Fatalf("alice's connections: %+v", cs)
	}
	id := cs.Connections[0].ID
	if got := list(bob); len(got.Connections) != 0 {
		t.Errorf("bob sees alice's connection: %+v", got.Connections)
	}
	var rows []row
	if st, _ := call(t, "GET", srv.url+"/api/connection", alice, nil, &rows); st != 200 || len(rows) != 1 || rows[0]["account"] != "alice@gmail.com" || rows[0]["token"] != nil || rows[0]["member"] != nil {
		t.Errorf("connection read as a table shows no token: %d %v", st, rows)
	}
	if st, _ := call(t, "GET", srv.url+"/api/connection", bob, nil, &rows); st != 200 || len(rows) != 0 {
		t.Errorf("bob reads alice's connection as a table: %v", rows)
	}
	if st, _ := call(t, "POST", srv.url+"/api/connection", alice, map[string]any{"provider": "gmail", "account": "x"}, nil); st != 405 {
		t.Errorf("a connection written as a row: %d", st)
	}
	if st, out := e2eRun(t, srv.url+"/api", bob, `return await archive.query("SELECT count(*)::int AS n FROM connection")`); st != 200 || string(out.Result) != `[{"n":0}]` {
		t.Errorf("bob counts connections with SQL: %d %+v", st, out)
	}
	if st, out := e2eRun(t, srv.url+"/api", alice, `return await archive.delete("connection", `+fmt.Sprintf("%q", id)+`).then(() => "deleted", (e) => e.message)`); st != 200 || !strings.Contains(string(out.Result), "read only") {
		t.Errorf("a script deletes a connection: %d %+v", st, out)
	}
	if plain := in.psql("archive", "SELECT count(*) FROM archive.connection WHERE position('rt-'::bytea IN token) > 0"); plain != "0" {
		t.Errorf("%s refresh tokens are kept in the clear", plain)
	}

	// Connecting the same mailbox again renews its row; a consent without gmail.readonly, or one cancelled, keeps nothing.
	if st, to := finish(alice, url.Values{"state": {start(alice).Get("state")}, "code": {"alice@gmail.com"}}); st != 303 || strings.Contains(to, "error") {
		t.Errorf("alice connects her Gmail again: %d %s", st, to)
	}
	if st, to := finish(alice, url.Values{"state": {start(alice).Get("state")}, "code": {"noscope:other@gmail.com"}}); st != 303 || !strings.Contains(to, "error=Gmail+was+connected+without+all+the+access") {
		t.Errorf("a consent without gmail.readonly: %d %s", st, to)
	}
	if st, to := finish(alice, url.Values{"state": {start(alice).Get("state")}, "error": {"access_denied"}}); st != 303 || !strings.Contains(to, "error=Gmail+was+not+connected%3A+access_denied") {
		t.Errorf("a cancelled consent: %d %s", st, to)
	}
	if cs := list(alice); len(cs.Connections) != 1 || cs.Connections[0].ID != id {
		t.Fatalf("after connecting again: %+v", cs.Connections)
	}
	if n := in.psql("archive", "SELECT count(*) FROM archive.connection"); n != "1" {
		t.Fatalf("%s connections, want 1", n)
	}

	connectionRefreshes(t, in, g, id)
	if cs := list(alice); len(cs.Connections) != 1 || cs.Connections[0].Error != "" {
		t.Errorf("after a refresh that worked again: %+v", cs.Connections)
	}

	// archive.action reads her mail with the Gmail API, as her.
	if st, out := e2eRun(t, srv.url+"/api", alice, `const found = await archive.action("gmail.search", { query: "from:ann", limit: 5 });
		const m = await archive.action("gmail.message", { id: found.messages[0].id });
		const gone = await archive.action("gmail.message", { id: "gone" }).catch((e) => e.code + " " + e.message);
		const bad = await archive.action("gmail.search", { query: "x", limt: 5 }).catch((e) => e.code + " " + e.message);
		return { found, text: m.text, attachments: m.attachments, gone, bad };`); st != 200 || out.Error != "" ||
		string(out.Result) != `{"found":{"messages":[{"from":"Ann \u003cann@example.com\u003e","id":"m1","labelIds":["INBOX"],"snippet":"Hello","subject":"Hi","threadId":"t1"}],"next":"p2"},`+
			`"text":"Hello Alice","attachments":[{"filename":"a.pdf","mimeType":"application/pdf","size":4}],`+
			`"gone":"not_found archive.action gmail.message: Gmail refused: 404 Not Found: Requested entity was not found.",`+
			`"bad":"invalid invalid archive.action gmail.search input: unexpected additional properties [\"limt\"]"}` {
		t.Errorf("alice reads her Gmail with actions: %d %+v", st, out)
	}
	g.do(func() {
		if g.searched != "from:ann 5" {
			t.Errorf("searched Gmail for %q", g.searched)
		}
	})

	gmailSyncs(t, in, srv, g, id, alice, bob)

	if st, _ := call(t, "DELETE", srv.url+"/connect/"+id.String(), bob, nil, nil); st != 404 {
		t.Errorf("bob disconnects alice's Gmail: %d", st)
	}
	if st, _ := call(t, "DELETE", srv.url+"/connect/"+id.String(), alice, nil, nil); st != 204 {
		t.Fatalf("alice disconnects her Gmail: %d", st)
	}
	if n := in.psql("archive", "SELECT count(*) FROM archive.record"); n != "0" {
		t.Errorf("disconnecting left %s records", n)
	}
	gmailPushesAfterDisconnect(t, in, srv, g)
	asanaHandshakes(t, in, srv)
	if cs := list(alice); len(cs.Connections) != 0 {
		t.Errorf("a disconnected Gmail is still listed: %+v", cs.Connections)
	}
	g.do(func() {
		if len(g.revoked) != 1 || !strings.HasPrefix(g.revoked[0], "rt-") {
			t.Errorf("disconnecting revoked %v, want the renewed refresh token", g.revoked)
		}
	})
	if st, _ := call(t, "DELETE", srv.url+"/connect/"+id.String(), alice, nil, nil); st != 404 {
		t.Errorf("disconnecting twice: %d", st)
	}
}

// connectionRefreshes checks, in a hub of its own on in, that connectionToken refreshes with the kept refresh
// token, keeps a replaced one, and records a refused refresh as the connection's error until one works again.
func connectionRefreshes(t *testing.T, in *e2eInst, g *fakeGoogle, id uuid.UUID) {
	t.Helper()
	ctx := t.Context()
	a := hubOn(t, in, map[string]string{"ARCHIVE_GOOGLE": strings.TrimPrefix(g.env(), "ARCHIVE_GOOGLE=")})
	kept := func() (string, string) {
		t.Helper()
		var token []byte
		var e string
		if err := a.st.sys.QueryRow(ctx, "SELECT token, error FROM archive.connection WHERE id = $1", id).Scan(&token, &e); err != nil {
			t.Fatal(err)
		}
		rt, err := connect.OpenToken(a.aead, id, token)
		if err != nil {
			t.Fatal(err)
		}
		return rt, e
	}

	first, _ := kept()
	tok, err := a.connectionToken(ctx, id)
	if err != nil || !strings.HasPrefix(tok.AccessToken, "at-") || !tok.Valid() {
		t.Fatalf("a fresh access token: %+v %v", tok, err)
	}
	if rt, e := kept(); rt != first || e != "" {
		t.Errorf("after a refresh that kept its refresh token: %q %q", rt, e)
	}

	g.do(func() { g.rotate = true })
	if _, err := a.connectionToken(ctx, id); err != nil {
		t.Fatal(err)
	}
	g.do(func() { g.rotate = false })
	rt, _ := kept()
	if rt == first || !strings.HasPrefix(rt, "rt-") {
		t.Fatalf("the replaced refresh token was not kept: %q", rt)
	}
	if _, err := a.connectionToken(ctx, id); err != nil {
		t.Errorf("a refresh with the replaced refresh token: %v", err)
	}

	g.do(func() { delete(g.refresh, rt) })
	if _, err := a.connectionToken(ctx, id); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("a revoked grant refreshes: %v", err)
	}
	if _, e := kept(); !strings.Contains(e, "Gmail refused to let Archive keep reading alice@gmail.com (invalid_grant: Token has been expired or revoked.); connect it again") {
		t.Errorf("a refused refresh is recorded as: %q", e)
	}
	g.do(func() { g.refresh[rt] = "alice@gmail.com" })
	if _, err := a.connectionToken(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, e := kept(); e != "" {
		t.Errorf("a refresh that works again left the error: %q", e)
	}
	if _, err := a.connectionToken(ctx, coreNewID()); !errors.Is(err, ErrNotFound) {
		t.Errorf("a token for a connection there is none of: %v", err)
	}
}
