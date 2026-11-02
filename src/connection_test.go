package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

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
	revoked  []string
}

func newFakeGoogle(t *testing.T) *fakeGoogle {
	g := &fakeGoogle{refresh: map[string]string{}, access: map[string]string{}}
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
			json.NewEncoder(w).Encode(map[string]any{"emailAddress": mailbox, "historyId": "1"})
		case "/revoke":
			g.revoked = append(g.revoked, r.Form.Get("token"))
			delete(g.refresh, r.Form.Get("token"))
			w.Write([]byte("{}"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(g.srv.Close)
	return g
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
// connect, and disconnecting revokes it.
func TestGmailConnectsForWhoStartedItAndDisconnectsWithARevoke(t *testing.T) {
	t.Parallel()
	g := newFakeGoogle(t)
	edge := &liveEdge{}
	hs := httptest.NewServer(edge)
	t.Cleanup(hs.Close)
	in := newInst(t, g.env(), "ARCHIVE_LIVE_KEY=k3y", "ARCHIVE_LIVE_URL="+hs.URL)
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

	if st, _ := call(t, "DELETE", srv.url+"/connect/"+id.String(), bob, nil, nil); st != 404 {
		t.Errorf("bob disconnects alice's Gmail: %d", st)
	}
	if st, _ := call(t, "DELETE", srv.url+"/connect/"+id.String(), alice, nil, nil); st != 204 {
		t.Fatalf("alice disconnects her Gmail: %d", st)
	}
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
	h, err := openHub(ctx, "host="+in.dir+" user=archive dbname=archive", filepath.Join(in.dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if h.connect, err = connect.Load(func(k string) string {
		return map[string]string{"ARCHIVE_GOOGLE": strings.TrimPrefix(g.env(), "ARCHIVE_GOOGLE=")}[k]
	}); err != nil {
		t.Fatal(err)
	}
	a, release, err := h.acquire(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
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
