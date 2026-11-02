package main

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Mulholland-Inc/archive/connect"
	"github.com/Mulholland-Inc/archive/generated"
	"github.com/google/uuid"
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

// syncSummary is connection id's live and deleted records, whether it caught up, and its error, like
// "50+0 false ".
func syncSummary(in *e2eInst, id uuid.UUID) string {
	return in.psql("archive", `SELECT (SELECT count(*) FILTER (WHERE NOT r.deleted) || '+' || count(*) FILTER (WHERE r.deleted) FROM archive.record r WHERE r.connection = c.id)
		|| ' ' || (c.synced_at IS NOT NULL) || ' ' || c.error FROM archive.connection c WHERE c.id = '`+id.String()+`'`)
}

// gmailSyncs checks, in a hub of its own on in, that alice's Gmail id, just connected, syncs into record a page
// at a time: the mailbox whole the first time, 50 messages a page, from where a failed page left off; then
// its history since, with a message added, one relabeled and one deleted; whole again once Gmail no longer
// has the history, deleting what it no longer lists. Only alice reads them, Sync now makes it due, and a
// refused grant stops it.
func gmailSyncs(t *testing.T, in *e2eInst, srv *e2eServer, g *fakeGoogle, id uuid.UUID, alice, bob string) {
	t.Helper()
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
	a := hubOn(t, in, map[string]string{"ARCHIVE_GOOGLE": strings.TrimPrefix(g.env(), "ARCHIVE_GOOGLE=")})
	want := func(what, state string) {
		t.Helper()
		if got := syncSummary(in, id); got != strings.TrimSpace(state) {
			t.Fatalf("%s: %q, want %q", what, got, state)
		}
	}
	syncPages(t, a, id, 1)
	want("the first page", "50+0 false ")
	g.do(func() { g.fail = 1 })
	syncPages(t, a, id, 1)
	want("a page that failed", "50+0 false syncing messages: Gmail refused: 500 Internal Server Error: Backend Error")
	if later := in.psql("archive", "SELECT sync_at > now() + interval '50 seconds' FROM archive.connection WHERE id = "+conn); later != "t" {
		t.Errorf("a page that failed is retried at once")
	}
	syncPages(t, a, id, 1)
	want("the page after, from where it failed", "60+0 true ")
	if text := in.psql("archive", "SELECT data->>'subject' || ': ' || (data->>'text') FROM archive.record WHERE source_id = 'a07'"); text != "Mail 7: About Mail 7" {
		t.Errorf("a message's record: %q", text)
	}

	g.do(func() {
		g.mail("a60", "Mail 60")
		g.mail("a01", "Mail 1, labeled")
		g.unmail("a02")
	})
	syncPages(t, a, id, 1)
	want("the history since", "60+1 true ")
	if got := in.psql("archive", "SELECT string_agg(source_id || ' ' || (data->>'subject') || ' ' || deleted, ', ' ORDER BY source_id) FROM archive.record WHERE source_id IN ('a01', 'a02', 'a60')"); got != "a01 Mail 1, labeled false, a02 Mail 2 true, a60 Mail 60 false" {
		t.Errorf("after the history: %q", got)
	}

	g.do(func() {
		g.oldest = g.hist + 1
		delete(g.box, "a03")
	})
	syncPages(t, a, id, 3)
	want("listed whole again", "59+2 true ")

	for tok, n := range map[string]string{alice: "59", bob: "0"} {
		if st, out := e2eRun(t, srv.url+"/api", tok, `const [r] = await archive.list("record", { where: { source_id: "a01" } });
			const [{ n }] = await archive.query("SELECT count(*)::int AS n FROM record WHERE stream = 'messages' AND NOT deleted");
			return n + " " + (r ? r.data.subject : "none")`); st != 200 || out.Error != "" || !strings.HasPrefix(string(out.Result), `"`+n+" ") {
			t.Errorf("reading the records, %s want %s: %d %+v", tok[:6], n, st, out)
		}
	}
	var cs Connections
	call(t, "GET", srv.url+"/connect", alice, nil, &cs)
	if len(cs.Connections) != 1 || cs.Connections[0].Records != 59 || cs.Connections[0].SyncedAt == nil || cs.Connections[0].SyncAt == nil {
		t.Errorf("the connection's sync, as listed: %+v", cs.Connections)
	}

	if st, _ := call(t, "POST", srv.url+"/connect/"+id.String()+"/sync", bob, nil, nil); st != 404 {
		t.Errorf("bob asks for a sync of alice's Gmail: %d", st)
	}
	if st, _ := call(t, "POST", srv.url+"/connect/"+id.String()+"/sync", alice, nil, nil); st != 204 {
		t.Errorf("alice asks for a sync: %d", st)
	}
	if due := in.psql("archive", `SELECT sync_at <= now() AND cursor->'messages'->'at' IS NULL FROM archive.connection WHERE id = `+conn); due != "t" {
		t.Errorf("Sync now did not make it due: %q", due)
	}

	var held map[string]string
	g.do(func() { held, g.refresh = g.refresh, map[string]string{} })
	a.conns.forget(id)
	syncPages(t, a, id, 1)
	if got := in.psql("archive", "SELECT (sync_at IS NULL) || ' ' || error FROM archive.connection WHERE id = "+conn); !strings.HasPrefix(got, "true Gmail refused to let Archive keep reading alice@gmail.com") {
		t.Errorf("a refused grant does not stop the sync: %q", got)
	}
	g.do(func() { g.refresh = held })
}

// notionSyncs checks, in a hub of its own on in, that alice's Notion id syncs into record with its server's
// tools: its private pages listed whole, two a page as the server pages them, from where a failed page left
// off, with its shared pages skipped as the server has no tool for them; and listed whole again, adding a
// page, changing one and deleting the one no longer listed.
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
	want("the first page, and the shared pages skipped", "2+0 false ")
	if skip := in.psql("archive", "SELECT cursor->'shared_pages'->>'skip' FROM archive.connection WHERE id = '"+id.String()+"'"); skip != "Notion's MCP server has no tool notion-list-shared-pages" {
		t.Errorf("the shared pages: %q", skip)
	}
	f.do(func() { f.fail = true })
	syncPages(t, a, id, 1)
	want("a page that failed", "2+0 false syncing private_pages: Notion said: notion-list-private-pages: the list is down")
	syncPages(t, a, id, 2)
	want("the pages after", "5+0 true ")
	f.do(func() {
		f.pages[fmt.Sprintf("%032x", 6)] = "Page 6"
		f.pages[fmt.Sprintf("%032x", 1)] = "Page 1, renamed"
		delete(f.pages, fmt.Sprintf("%032x", 2))
	})
	syncPages(t, a, id, 3)
	want("listed whole again", "5+1 true ")
	if got := in.psql("archive", "SELECT string_agg(right(source_id, 1) || ' ' || (data->>'title') || ' ' || deleted, ', ' ORDER BY source_id) FROM archive.record"); got != "1 Page 1, renamed false, 2 Page 2 true, 3 Page 3 false, 4 Page 4 false, 5 Page 5 false, 6 Page 6 false" {
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
