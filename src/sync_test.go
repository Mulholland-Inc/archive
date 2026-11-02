package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Mulholland-Inc/archive/connect"
	"github.com/Mulholland-Inc/archive/generated"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// hubOn is the first workspace of a hub of the test's own on in's database, with the connectors env sets.
// Syncs are off in it, as in serve (TestMain), so only syncPages pulls a page.
func hubOn(t *testing.T, in *e2eInst, env map[string]string) *Archive {
	t.Helper()
	ctx := t.Context()
	h, err := openHub(ctx, "host="+in.dir+" user=archive dbname=archive", filepath.Join(in.dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	if h.connect, err = connect.Load(func(k string) string { return env[k] }); err != nil {
		t.Fatal(err)
	}
	a, release, err := h.acquire(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	return a
}

// syncPages pulls n pages of connection id's sync, each as a task would once it was due.
func syncPages(t *testing.T, a *Archive, id uuid.UUID, n int) {
	t.Helper()
	ctx := t.Context()
	for range n {
		var c generated.ArchiveConnection
		if err := a.st.System(ctx, func(ctx context.Context) (err error) {
			c, err = a.st.db(ctx).Connection(ctx, id)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		if err := a.syncPage(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
}

// syncSummary is how many messages and pages connection id has, whether it caught up, and its error, like
// "50 false ".
func syncSummary(in *e2eInst, id uuid.UUID) string {
	return in.psql("archive", `SELECT ((SELECT count(*) FROM archive.mail_message r WHERE r.connection = c.id) + (SELECT count(*) FROM archive.doc_page r WHERE r.connection = c.id))
		|| ' ' || (c.synced_at IS NOT NULL) || ' ' || c.error FROM archive.connection c WHERE c.id = '`+id.String()+`'`)
}

// gmailPush posts a Pub/Sub push to srv's /push/gmail for the mailbox email, signed by the Google stand-in
// with claims, as the tasks account's push subscription would sign it unless they say otherwise.
func gmailPush(t *testing.T, in *e2eInst, srv *e2eServer, email string, claims map[string]any) int {
	t.Helper()
	aud := "http://localhost:" + srv.url[strings.LastIndex(srv.url, ":")+1:] + "/push/gmail"
	c := map[string]any{"aud": aud, "sub": "1", "email": in.getenv("K_SERVICE") + "-tasks@e2e.iam.gserviceaccount.com", "email_verified": true}
	maps.Copy(c, claims)
	tok := ""
	if c["email"] != "" {
		var err error
		if tok, err = e2eCloudTasks.token(c); err != nil {
			t.Fatal(err)
		}
	}
	data := base64.StdEncoding.EncodeToString([]byte(`{"emailAddress":"` + email + `","historyId":"9"}`))
	st, _ := call(t, "POST", srv.url+"/push/gmail", tok, map[string]any{"message": map[string]any{"data": data, "messageId": "1"}, "subscription": "s"}, nil)
	return st
}

// gmailSyncs checks, in a hub of its own on in, that alice's Gmail id, just connected, syncs into mail_message a page
// at a time, one page of a connection at a time: the mailbox whole the first time, 50 messages a page, from
// where a failed page left off, with nothing more due once it has caught up. It watches the mailbox once, and
// a push signed by the tasks account asks for the history since, with a message added, one relabeled and one
// deleted; one signed otherwise is refused. A history that says there is more from where it began fails rather
// than read forever; one Gmail no longer has lists the mailbox whole again. A watch near its end is renewed by
// the next page, unless another is renewing it. Only alice reads its messages; reading them or opening the
// workspace while they are stale asks for a sync; Sync now does; a refused grant stops it, and pushes do not
// start it again.
func gmailSyncs(t *testing.T, in *e2eInst, srv *e2eServer, g *fakeGoogle, id uuid.UUID, alice, bob string) {
	t.Helper()
	ctx := t.Context()
	conn := "'" + id.String() + "'"
	if due := in.psql("archive", "SELECT sync_at <= now() FROM archive.connection WHERE id = "+conn); due != "t" {
		t.Errorf("connecting did not make its sync due: %q", due)
	}
	if !slices.ContainsFunc(e2eCloudTasks.tasks(), func(tk hubTask) bool {
		return strings.HasPrefix(tk.Name, in.queue()+"/") && strings.Contains(string(tk.HTTPRequest.Body), `"work":"syncs"`)
	}) {
		t.Error("connecting queued no sync")
	}
	g.do(func() {
		for i := range 60 {
			g.box[fmt.Sprintf("a%02d", i)] = fmt.Sprintf("Mail %d", i)
		}
	})
	a := hubOn(t, in, map[string]string{"ARCHIVE_GOOGLE": strings.TrimPrefix(g.env(), "ARCHIVE_GOOGLE="), "ARCHIVE_GMAIL_TOPIC": "projects/e2e/topics/gmail"})
	a.hub.api = "https://api.archive.test"
	want := func(what, state string) {
		t.Helper()
		if got := syncSummary(in, id); got != strings.TrimSpace(state) {
			t.Fatalf("%s: %q, want %q", what, got, state)
		}
	}
	field := func(sql string) string {
		return in.psql("archive", "SELECT "+sql+" FROM archive.connection WHERE id = "+conn)
	}
	pushed := func(what string) {
		t.Helper()
		if st := gmailPush(t, in, srv, "Alice@Gmail.com", nil); st != 202 {
			t.Fatalf("%s: a push: %d", what, st)
		}
	}

	// One page of a connection at a time: a second lease finds none due.
	var leased generated.ArchiveConnection
	claim := func() error {
		return a.st.System(ctx, func(ctx context.Context) (err error) {
			leased, err = a.st.db(ctx).ClaimSync(ctx, time.Now().Add(syncLease))
			return err
		})
	}
	if err := claim(); err != nil || leased.ID != id {
		t.Fatalf("leasing the first page: %v", err)
	}
	first := leased
	if err := claim(); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("a second lease of a connection whose page is under way: %v", err)
	}
	if err := a.syncPage(ctx, first); err != nil {
		t.Fatal(err)
	}
	want("the first page", "50 false ")
	if got := field("lease_at IS NULL"); got != "t" {
		t.Errorf("a page done keeps its lease")
	}
	g.do(func() {
		if !slices.Equal(g.watched, []string{"projects/e2e/topics/gmail"}) {
			t.Errorf("the mailbox was watched with %q", g.watched)
		}
	})
	if w := in.psql("archive", "SELECT key || ' ' || (expires_at > now() + interval '6 days') FROM archive.watch WHERE connection = "+conn); w != "alice@gmail.com true" {
		t.Errorf("the watch kept: %q", w)
	}

	cursor := field("cursor->'messages'->>'cursor'")
	g.do(func() { g.fail = 1 })
	syncPages(t, a, id, 1)
	want("a page that failed", "50 false syncing messages: Gmail refused: 500 Internal Server Error: Backend Error")
	if later := field("sync_at > now() + interval '50 seconds'"); later != "t" {
		t.Errorf("a page that failed is retried at once")
	}
	if got := field("cursor->'messages'->>'cursor'"); got != cursor {
		t.Errorf("a page that failed moved the cursor from %s to %s", cursor, got)
	}
	syncPages(t, a, id, 1)
	want("the page after, from where it failed", "60 true ")
	if text := in.psql("archive", "SELECT subject || ': ' || text FROM archive.mail_message WHERE source_id = 'a07'"); text != "Mail 7: About Mail 7" {
		t.Errorf("a message: %q", text)
	}
	if idle := field("sync_at IS NULL"); idle != "t" {
		t.Fatalf("a sync that caught up is still due: nothing may wake the service until something asks")
	}

	g.do(func() {
		g.mail("a60", "Mail 60")
		g.mail("a01", "Mail 1, labeled")
		g.unmail("a02")
	})
	syncPages(t, a, id, 1)
	want("a page nothing asked for", "60 true ")
	for what, claims := range map[string]map[string]any{
		"unsigned": {"email": ""}, "for another URL": {"aud": "http://localhost:1/push/gmail"}, "by another account": {"email": "eve@e2e.iam.gserviceaccount.com"},
	} {
		if st := gmailPush(t, in, srv, "alice@gmail.com", claims); st != 401 && st != 403 {
			t.Errorf("a push %s: %d", what, st)
		}
	}
	if idle := field("sync_at IS NULL"); idle != "t" {
		t.Errorf("a forged push asked for a sync")
	}
	pushed("a change")
	if due := field("sync_at <= now() AND asked_at > synced_at"); due != "t" {
		t.Fatalf("a push did not ask for a sync: %q", due)
	}
	syncPages(t, a, id, 1)
	want("the history since", "60 true ")
	if got := in.psql("archive", "SELECT string_agg(source_id || ' ' || subject, ', ' ORDER BY source_id) FROM archive.mail_message WHERE source_id IN ('a01', 'a02', 'a60')"); got != "a01 Mail 1, labeled, a60 Mail 60" {
		t.Errorf("after the history: %q", got)
	}

	g.do(func() { g.stuck = true })
	pushed("a history that never ends")
	syncPages(t, a, id, 2)
	if got := field("error"); !strings.Contains(got, "said there was more, but from where this page began") {
		t.Errorf("a page that would go round forever: %q", got)
	}
	g.do(func() { g.stuck = false })
	syncPages(t, a, id, 1)
	want("once it moves on again", "60 true ")

	// A stream that keeps failing backs off syncTries times, then waits for the next ask, which tries again.
	g.do(func() { g.fail = syncTries })
	pushed("a Gmail that keeps failing")
	syncPages(t, a, id, syncTries)
	if got := field("(sync_at IS NULL) || ' ' || (error LIKE 'syncing messages: Gmail refused: 500%')"); got != "true true" {
		t.Errorf("a stream failing %d times in a row: %q", syncTries, got)
	}
	pushed("Gmail back")
	syncPages(t, a, id, 1)
	want("asked again once Gmail is back", "60 true ")

	g.do(func() {
		g.hist++
		g.oldest = g.hist
		delete(g.box, "a03")
	})
	pushed("a history Gmail no longer has")
	syncPages(t, a, id, 3)
	want("listed whole again", "59 true ")
	g.do(func() {
		if len(g.watched) != 1 {
			t.Errorf("a watch far from its end was renewed: %q", g.watched)
		}
	})

	// A watch near its end is renewed by the next page, unless another page holds it.
	in.psql("archive", "UPDATE archive.watch SET expires_at = now() + interval '1 hour' WHERE connection = "+conn)
	var c generated.ArchiveConnection
	a.st.System(ctx, func(ctx context.Context) (err error) { c, err = a.st.db(ctx).Connection(ctx, id); return err })
	cl, err := a.connClient(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := a.hub.connect.Get("gmail")
	tx, err := a.hub.main.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := generated.New(tx).TryLock(ctx, "watch "+id.String()); !ok || err != nil {
		t.Fatalf("holding the watch: %v", err)
	}
	a.syncWatch(ctx, c, p, cl)
	tx.Rollback(ctx)
	g.do(func() {
		if len(g.watched) != 1 {
			t.Errorf("a watch another page holds was renewed too: %q", g.watched)
		}
	})
	a.syncWatch(ctx, c, p, cl)
	cl.Close()
	g.do(func() {
		if len(g.watched) != 2 {
			t.Errorf("a watch near its end was not renewed: %q", g.watched)
		}
	})
	if w := in.psql("archive", "SELECT count(*) || ' ' || bool_and(expires_at > now() + interval '6 days') FROM archive.watch WHERE connection = "+conn); w != "1 true" {
		t.Errorf("the renewed watch: %q", w)
	}

	for tok, n := range map[string]string{alice: "59", bob: "0"} {
		if st, out := e2eRun(t, srv.url+"/api", tok, `const [r] = await archive.list("mail_message", { where: { source_id: "a01" } });
			const [{ n }] = await archive.query("SELECT count(*)::int AS n FROM mail_message WHERE stream = 'messages'");
			return n + " " + (r ? r.subject : "none")`); st != 200 || out.Error != "" || !strings.HasPrefix(string(out.Result), `"`+n+" ") {
			t.Errorf("reading the messages, %s want %s: %d %+v", tok[:6], n, st, out)
		}
	}
	var cs Connections
	call(t, "GET", srv.url+"/connect", alice, nil, &cs)
	if len(cs.Connections) != 1 || cs.Connections[0].Items != 59 || cs.Connections[0].SyncedAt == nil || cs.Connections[0].SyncAt != nil {
		t.Errorf("the connection's sync, as listed: %+v", cs.Connections)
	}

	// Reading mail, or opening the workspace, asks for a stale sync, and nothing else does.
	stale := func() {
		in.psql("archive", "UPDATE archive.connection SET synced_at = now() - interval '10 minutes' WHERE id = "+conn)
	}
	stale()
	for _, code := range []string{`return await archive.query("SELECT 1 AS n")`, `return await archive.list("workflow")`} {
		e2eRun(t, srv.url+"/api", alice, code)
	}
	call(t, "GET", srv.url+"/connect", alice, nil, nil)
	if idle := field("sync_at IS NULL"); idle != "t" {
		t.Errorf("reading something other than what syncs asked for a sync")
	}
	e2eRun(t, srv.url+"/api", bob, `return await archive.query("SELECT count(*) FROM mail_message")`)
	if idle := field("sync_at IS NULL"); idle != "t" {
		t.Errorf("bob reading mail asked for alice's sync")
	}
	e2eRun(t, srv.url+"/api", alice, `return await archive.query("SELECT count(*) FROM mail_message")`)
	if due := field("sync_at <= now()"); due != "t" {
		t.Errorf("alice reading mail did not ask for her stale sync")
	}
	syncPages(t, a, id, 1)
	stale()
	if st, _ := call(t, "GET", srv.url+"/auth/me", alice, nil, nil); st != 200 {
		t.Fatalf("opening the workspace: %d", st)
	}
	if due := field("sync_at <= now()"); due != "t" {
		t.Errorf("opening the workspace did not ask for a stale sync")
	}
	syncPages(t, a, id, 1)

	if st, _ := call(t, "POST", srv.url+"/connect/"+id.String()+"/sync", bob, nil, nil); st != 404 {
		t.Errorf("bob asks for a sync of alice's Gmail: %d", st)
	}
	if st, _ := call(t, "POST", srv.url+"/connect/"+id.String()+"/sync", alice, nil, nil); st != 204 {
		t.Errorf("alice asks for a sync: %d", st)
	}
	if due := field("sync_at <= now() AND cursor->'messages'->'at' IS NULL"); due != "t" {
		t.Errorf("Sync now did not make it due: %q", due)
	}

	var held map[string]string
	g.do(func() { held, g.refresh = g.refresh, map[string]string{} })
	a.conns.forget(id)
	syncPages(t, a, id, 1)
	if got := field("(sync_at IS NULL) || ' ' || revoked || ' ' || error"); !strings.HasPrefix(got, "true true Gmail refused to let Archive keep reading alice@gmail.com") {
		t.Errorf("a refused grant does not stop the sync: %q", got)
	}
	pushed("a refused grant")
	if idle := field("sync_at IS NULL"); idle != "t" {
		t.Errorf("a push started a sync whose grant was refused")
	}
	g.do(func() { g.refresh = held })
}

// gmailPushesAfterDisconnect checks that a push for a mailbox disconnected since starts nothing, and that a
// watch Archive kept of a connection gone is let go when one comes.
func gmailPushesAfterDisconnect(t *testing.T, in *e2eInst, srv *e2eServer, g *fakeGoogle) {
	t.Helper()
	g.do(func() {
		if g.stops != 1 {
			t.Errorf("disconnecting stopped the watch %d times", g.stops)
		}
	})
	if n := in.psql("archive", "SELECT count(*) FROM archive.watch"); n != "0" {
		t.Errorf("disconnecting left %s watches", n)
	}
	if st := gmailPush(t, in, srv, "alice@gmail.com", nil); st != 202 {
		t.Errorf("a push for a mailbox disconnected: %d", st)
	}
	ws := in.psql("archive", "SELECT id FROM archive.workspace ORDER BY created_at LIMIT 1")
	in.psql("archive", "INSERT INTO archive.watch (provider, key, workspace, connection) VALUES ('gmail', 'alice@gmail.com', '"+ws+"', '"+uuid.NewString()+"')")
	if st := gmailPush(t, in, srv, "alice@gmail.com", nil); st != 202 {
		t.Errorf("a push for a connection gone: %d", st)
	}
	if n := in.psql("archive", "SELECT count(*) FROM archive.watch"); n != "0" {
		t.Errorf("a push for a connection gone kept its watch")
	}
	if n := in.psql("archive", "SELECT count(*) FROM archive.connection"); n != "0" {
		t.Errorf("a push brought a connection back")
	}
}

// notionSyncs checks, in a hub of its own on in, that alice's Notion id syncs into doc_page with its server's
// tools: its private pages listed whole, two a page as the server pages them, from where a failed page left
// off, with its shared pages skipped as the server has no tool for them; and listed whole again, adding a
// page, changing one and deleting the one no longer listed, once something asks.
func notionSyncs(t *testing.T, in *e2eInst, f *fakeMCP, id uuid.UUID) {
	t.Helper()
	f.do(func() {
		for i := 1; i <= 5; i++ {
			f.pages[fmt.Sprintf("%032x", i)] = fmt.Sprintf("Page %d", i)
		}
	})
	a := hubOn(t, in, map[string]string{"ARCHIVE_MCP_NOTION": f.srv.URL + "/mcp"})
	want := func(what, state string) {
		t.Helper()
		if got := syncSummary(in, id); got != strings.TrimSpace(state) {
			t.Fatalf("%s: %q, want %q", what, got, state)
		}
	}
	syncPages(t, a, id, 2)
	want("the first page, and the shared pages skipped", "2 false ")
	if skip := in.psql("archive", "SELECT cursor->'shared_pages'->>'skip' FROM archive.connection WHERE id = '"+id.String()+"'"); skip != "Notion's MCP server has no tool notion-list-shared-pages" {
		t.Errorf("the shared pages: %q", skip)
	}
	f.do(func() { f.fail = true })
	syncPages(t, a, id, 1)
	want("a page that failed", "2 false syncing private_pages: Notion said: notion-list-private-pages: the list is down")
	syncPages(t, a, id, 2)
	want("the pages after", "5 true ")
	if err := func() error { _, err := a.syncWant(t.Context(), id); return err }(); err != nil {
		t.Fatal(err)
	}
	f.do(func() {
		f.pages[fmt.Sprintf("%032x", 6)] = "Page 6"
		f.pages[fmt.Sprintf("%032x", 1)] = "Page 1, renamed"
		delete(f.pages, fmt.Sprintf("%032x", 2))
	})
	syncPages(t, a, id, 4)
	want("listed whole again, and the shared pages looked for again", "5 true ")
	if got := in.psql("archive", "SELECT string_agg(right(source_id, 1) || ' ' || title, ', ' ORDER BY source_id) FROM archive.doc_page"); got != "1 Page 1, renamed, 3 Page 3, 4 Page 4, 5 Page 5, 6 Page 6" {
		t.Errorf("after listing whole again: %q", got)
	}
}

// notionListTool adds f's notion-list-private-pages to tools: its pages by id, two a page with a cursor.
func notionListTool(f *fakeMCP, tools *mcp.Server) {
	type listIn struct {
		Limit  int    `json:"limit,omitempty"`
		Cursor string `json:"cursor,omitempty"`
	}
	type listOut struct {
		Pages      []map[string]string `json:"pages"`
		NextCursor string              `json:"nextCursor,omitempty"`
	}
	mcp.AddTool(tools, &mcp.Tool{Name: "notion-list-private-pages", Description: "Your private pages."},
		func(ctx context.Context, req *mcp.CallToolRequest, in listIn) (*mcp.CallToolResult, listOut, error) {
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.fail {
				f.fail = false
				return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "the list is down"}}}, listOut{}, nil
			}
			ids := slices.Sorted(func(yield func(string) bool) {
				for id := range f.pages {
					if !yield(id) {
						return
					}
				}
			})
			from := slices.Index(ids, in.Cursor) + 1
			out := listOut{Pages: []map[string]string{}}
			for _, id := range ids[from:min(from+2, len(ids))] {
				out.Pages = append(out.Pages, map[string]string{"type": "page", "title": f.pages[id], "url": "https://www.notion.so/" + strings.ReplaceAll(f.pages[id], " ", "-") + "-" + id})
			}
			if from+2 < len(ids) {
				out.NextCursor = ids[from+1]
			}
			return nil, out, nil
		})
}

// A record's NUL characters, which jsonb refuses, are left out, and nothing else is.
func TestSyncJSONLeavesOutNULs(t *testing.T) {
	for in, want := range map[string]string{"a\x00b": `{"a\\b":["ab"]}`, `\u0000`: `{"a\\b":["\\u0000"]}`} {
		b, err := syncJSON(map[string]any{"a\\b": []any{in}})
		if err != nil || string(b) != want {
			t.Errorf("%q: %s %v, want %s", in, b, err, want)
		}
	}
}

// asanaHandshakes checks that /push/asana keeps the secret of a webhook's handshake by the key in its URL, for
// a connection to Asana alone, answering with it; refuses a second handshake and an event not signed with it;
// asks for the connection's sync on a signed event; and lets the watch go once one finds its connection gone.
func asanaHandshakes(t *testing.T, in *e2eInst, srv *e2eServer) {
	t.Helper()
	ws := in.psql("archive", "SELECT id FROM archive.workspace ORDER BY created_at LIMIT 1")
	conn := uuid.NewString()
	in.psql("archive", "INSERT INTO archive.connection (id, member, provider, account, token, sync_at) SELECT '"+conn+"', id, 'asana', 'a@b', '\\x00', NULL FROM archive.member ORDER BY email LIMIT 1")
	stranger := srv.url + "/push/asana?" + url.Values{"w": {ws}, "c": {uuid.NewString()}, "k": {"c/p1/other"}}.Encode()
	u := srv.url + "/push/asana?" + url.Values{"w": {ws}, "c": {conn}, "k": {"c/p1/n0nce"}}.Encode()
	postTo := func(u, body string, hdr map[string]string) (int, http.Header) {
		t.Helper()
		req, _ := http.NewRequest("POST", u, strings.NewReader(body))
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode, resp.Header
	}
	post := func(body string, hdr map[string]string) (int, http.Header) { return postTo(u, body, hdr) }
	if st, _ := postTo(stranger, "", map[string]string{"X-Hook-Secret": "x"}); st != 401 {
		t.Errorf("a handshake for no connection: %d", st)
	}
	if st, h := post("", map[string]string{"X-Hook-Secret": "s3cret"}); st != 200 || h.Get("X-Hook-Secret") != "s3cret" {
		t.Fatalf("the handshake: %d %v", st, h)
	}
	if kept := in.psql("archive", "SELECT count(*) FROM archive.watch WHERE provider = 'asana' AND key = 'c/p1/n0nce' AND position('s3cret'::bytea IN secret) = 0"); kept != "1" {
		t.Errorf("the handshake's secret is not kept sealed: %s", kept)
	}
	if st, _ := post("", map[string]string{"X-Hook-Secret": "evil"}); st != 401 {
		t.Errorf("a second handshake: %d", st)
	}
	body := `{"events":[{"action":"changed"}]}`
	m := hmac.New(sha256.New, []byte("evil"))
	m.Write([]byte(body))
	if st, _ := post(body, map[string]string{"X-Hook-Signature": hex.EncodeToString(m.Sum(nil))}); st != 401 {
		t.Errorf("an event signed with another secret: %d", st)
	}
	m = hmac.New(sha256.New, []byte("s3cret"))
	m.Write([]byte(body))
	if st, _ := post(body, map[string]string{"X-Hook-Signature": hex.EncodeToString(m.Sum(nil))}); st != 202 {
		t.Errorf("a signed event: %d", st)
	}
	if due := in.psql("archive", "SELECT sync_at <= now() FROM archive.connection WHERE id = '"+conn+"'"); due != "t" {
		t.Errorf("a signed event did not ask for its connection's sync: %q", due)
	}
	in.psql("archive", "DELETE FROM archive.connection WHERE id = '"+conn+"'")
	if st, _ := post(body, map[string]string{"X-Hook-Signature": hex.EncodeToString(m.Sum(nil))}); st != 202 {
		t.Errorf("a signed event for a connection gone: %d", st)
	}
	if n := in.psql("archive", "SELECT count(*) FROM archive.watch WHERE provider = 'asana'"); n != "0" {
		t.Errorf("a signed event for a connection gone kept its watch")
	}
}

// An integration is the archive's: only an owner connects, narrows, syncs and disconnects it, and the archive
// has one; every member sees it and what it syncs, while an owner has not narrowed it to a group they are not
// in; and editors and owners in that group act with it. Connections of an integration from before there were
// integrations become the archive's, the first of each.
func TestIntegrationsAreTheArchives(t *testing.T) {
	t.Parallel()
	f := newFakeMCP(t)
	in := newInst(t, "ARCHIVE_MCP_LINEAR="+f.srv.URL+"/mcp")
	in.must("user", "add", "alice@example.com")
	in.must("user", "add", "bob@example.com", "-role", "editor")
	in.must("user", "add", "carol@example.com", "-role", "viewer")
	carol := in.psql("archive", "SELECT id FROM archive.member WHERE email = 'carol@example.com'")
	in.must("set", "member", carol, `{"groups":["finance"]}`)
	srv := in.serve()
	ws := in.psql("archive", "SELECT id FROM archive.workspace ORDER BY created_at LIMIT 1")
	alice := decode[Token](t, in.must("token", "create")).Secret
	bob := decode[Token](t, in.must("-as", "bob@example.com", "token", "create")).Secret
	carolTok := decode[Token](t, in.must("-as", "carol@example.com", "token", "create")).Secret

	st, hdr := call(t, "GET", srv.url+"/connect/linear?workspace="+ws, bob, nil, nil)
	if to := hdr.Get("Location"); st != 303 || !strings.Contains(to, "/archive/integrations/linear?") || !strings.Contains(to, "only+an+owner+connects+Linear") {
		t.Errorf("an editor connects the archive's Linear: %d %s", st, to)
	}
	st, hdr = call(t, "GET", srv.url+"/connect/linear?workspace="+ws, alice, nil, nil)
	loc, _ := url.Parse(hdr.Get("Location"))
	if st != 302 || loc.Path != "/authorize" {
		t.Fatalf("an owner starts connecting Linear: %d %s", st, hdr.Get("Location"))
	}
	st, hdr = call(t, "GET", srv.url+"/connect/linear/callback?"+url.Values{"state": {loc.Query().Get("state")}, "code": {"alice@example.com"}}.Encode(), alice, nil, nil)
	if to := hdr.Get("Location"); st != 303 || strings.Contains(to, "error") || !strings.Contains(to, "/archive/integrations/linear?") {
		t.Fatalf("alice connects the archive's Linear: %d %s", st, to)
	}
	var cs Connections
	call(t, "GET", srv.url+"/connect", bob, nil, &cs)
	if len(cs.Connections) != 1 || !cs.Connections[0].Integration || cs.Connections[0].ConnectedBy != "alice@example.com" ||
		!slices.Contains(cs.Connectors, Connector{ID: "linear", Name: "Linear", Ready: true, Integration: true}) {
		t.Fatalf("bob's view of the archive's connections: %+v", cs)
	}
	id := cs.Connections[0].ID.String()
	for _, req := range [][2]string{{"DELETE", "/connect/" + id}, {"POST", "/connect/" + id + "/sync"}, {"PUT", "/connect/" + id + "/access"}} {
		if st, _ := call(t, req[0], srv.url+req[1], bob, map[string]string{"access": ""}, nil); st != 403 {
			t.Errorf("an editor %s %s: %d", req[0], req[1], st)
		}
	}
	in.psql("archive", "INSERT INTO archive.doc_page (connection, stream, source_id) VALUES ('"+id+"', 'documents', 'i1'), ('"+id+"', 'documents', 'i2')")
	sees := func(tok string) string {
		t.Helper()
		_, out := e2eRun(t, srv.url+"/api", tok, `const [{ n }] = await archive.query("SELECT count(*)::int AS n FROM doc_page"); return n + " " + (await archive.list("connection")).length`)
		return string(out.Result)
	}
	acts := func(tok string) string {
		t.Helper()
		_, out := e2eRun(t, srv.url+"/api", tok, `return await archive.action("linear.notion-search", { query: "plan" }).then(() => "acts", (e) => e.code)`)
		return string(out.Result)
	}
	for who, tok := range map[string]string{"alice": alice, "bob": bob, "carol": carolTok} {
		if got := sees(tok); got != `"2 1"` {
			t.Errorf("%s sees %s of the archive's Linear, want all of it", who, got)
		}
	}
	if got := acts(bob) + acts(carolTok); got != `"acts""forbidden"` {
		t.Errorf("an editor and a viewer act with the archive's Linear: %s", got)
	}

	if st, _ := call(t, "PUT", srv.url+"/connect/"+id+"/access", alice, map[string]string{"access": "finance"}, nil); st != 204 {
		t.Fatalf("alice narrows Linear to finance: %d", st)
	}
	for who, want := range map[string]string{alice: `"2 1"`, bob: `"0 0"`, carolTok: `"2 1"`} {
		if got := sees(who); got != want {
			t.Errorf("%s sees %s of Linear narrowed to finance, want %s", who[:6], got, want)
		}
	}
	if got := acts(bob); got != `"forbidden"` {
		t.Errorf("an editor out of finance acts with Linear narrowed to it: %s", got)
	}
	in.psql("archive", "INSERT INTO archive.doc_page (connection, stream, source_id, access) VALUES ('"+id+"', 'documents', 'i3', 'finance')")
	if st, _ := call(t, "PUT", srv.url+"/connect/"+id+"/access", alice, map[string]string{"access": "Fin ance"}, nil); st != 400 {
		t.Errorf("a group that is no group: %d", st)
	}

	// Connecting another Linear account while the archive has one is refused.
	a := hubOn(t, in, nil)
	member := uuid.MustParse(in.psql("archive", "SELECT id FROM archive.member WHERE email = 'alice@example.com'"))
	if _, err := a.connectionSave(t.Context(), member, connect.Get("linear"), &connect.Grant{Account: "Other", Refresh: "rt"}); !errors.Is(err, ErrConflict) ||
		!strings.Contains(err.Error(), "this archive's Linear is Linear; disconnect it before connecting Other") {
		t.Errorf("a second Linear account: %v", err)
	}
	if st, _ := call(t, "DELETE", srv.url+"/connect/"+id, alice, nil, nil); st != 204 {
		t.Fatalf("alice disconnects Linear: %d", st)
	}
	if n := in.psql("archive", "SELECT count(*) FROM archive.doc_page"); n != "0" {
		t.Errorf("disconnecting the archive's Linear left %s pages", n)
	}

	// Two members' Stripe from before integrations: the first becomes the archive's; the other stays its own.
	in.psql("archive", `INSERT INTO archive.connection (member, provider, account, token, created_at)
		SELECT id, 'stripe', email, '\x00', CASE WHEN email = 'bob@example.com' THEN now() - interval '1 day' ELSE now() END FROM archive.member WHERE email IN ('alice@example.com', 'bob@example.com');
		INSERT INTO archive.connection (member, provider, account, token) SELECT id, 'gmail', email, '\x00' FROM archive.member WHERE email = 'carol@example.com'`)
	if err := a.connectionUpgrade(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := in.psql("archive", "SELECT string_agg(provider || ' ' || account || ' ' || integration, ', ' ORDER BY provider, account) FROM archive.connection"); got != "gmail carol@example.com false, stripe alice@example.com false, stripe bob@example.com true" {
		t.Errorf("after the upgrade: %s", got)
	}
}
