package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/websocket"
)

// liveEdge is a stand-in for the app Worker's /live/announce: it keeps every announce it is sent.
type liveEdge struct {
	mu   sync.Mutex
	got  []liveAnnounce
	auth []string
}

func (e *liveEdge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var a liveAnnounce
	json.NewDecoder(r.Body).Decode(&a)
	e.mu.Lock()
	e.got = append(e.got, a)
	e.auth = append(e.auth, r.URL.Path+" "+r.Header.Get("Authorization"))
	e.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

// tables are the tables announced since the last call, and drains them.
func (e *liveEdge) tables() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	var out []string
	for _, a := range e.got {
		for _, c := range a.Changes {
			out = append(out, c.Table)
		}
	}
	e.got = nil
	return out
}

func TestLiveAnnouncesWhatCommittedAndNothingRolledBack(t *testing.T) {
	t.Parallel()
	edge := &liveEdge{}
	hs := httptest.NewServer(edge)
	t.Cleanup(hs.Close)
	in := newInst(t, "ARCHIVE_LIVE_KEY=k3y", "ARCHIVE_LIVE_URL="+hs.URL)
	in.must("user", "add", "alice@example.com")
	in.push(e2eSchema)
	srv := in.serve()
	api := srv.url + "/api"
	tok := decode[Token](t, in.must("token", "create")).Secret
	edge.tables()

	// A request's announces are sent before it ends, so the edge has them when the answer arrives.
	if st, out := e2eRun(t, api, tok, `await archive.create("company", { name: "Acme" }); return 1`); st != 200 || out.Error != "" {
		t.Fatalf("create: %d %v", st, out)
	}
	if got := edge.tables(); !slices.Contains(got, "company") {
		t.Fatalf("a committed create announced %v, want company", got)
	}
	edge.mu.Lock()
	auth := slices.Clone(edge.auth)
	edge.mu.Unlock()
	if !slices.Contains(auth, "/live/announce Bearer k3y") {
		t.Errorf("announces went to %v, want /live/announce with the key", auth)
	}

	// archive.test undoes what it wrote with a savepoint, and a run that throws rolls back: neither moved
	// anything.
	if st, out := e2eRun(t, api, tok, `return await archive.test("await archive.create('company', { name: 'Draft' }); return 1")`); st != 200 || out.Error != "" {
		t.Fatalf("test: %d %v", st, out)
	}
	e2eRun(t, api, tok, `await archive.create("person", { name: "Ann" }); throw new Error("nope")`)
	time.Sleep(3 * liveWindow)
	if got := edge.tables(); slices.Contains(got, "company") || slices.Contains(got, "person") {
		t.Fatalf("rolled-back writes announced %v", got)
	}

	// Archive's own tables carry the row's id; the main database's say so from Go.
	st, f := e2eUpload(t, api+"/files", tok, "a.txt", []byte("hello"))
	if st != 201 {
		t.Fatalf("upload: %d", st)
	}
	call(t, "POST", srv.url+"/auth/tokens", tok, map[string]string{"name": "second"}, nil)
	time.Sleep(3 * liveWindow)
	edge.mu.Lock()
	var ids []string
	for _, a := range edge.got {
		for _, c := range a.Changes {
			ids = append(ids, c.Table+" "+c.ID)
		}
	}
	edge.mu.Unlock()
	if !slices.Contains(ids, "archive.file "+f.ID.String()) || !slices.ContainsFunc(ids, func(s string) bool { return strings.HasPrefix(s, "archive.token ") }) {
		t.Errorf("announced %v, want the file by id and the new token", ids)
	}
	edge.tables()

	// A migration announces the new revision, and the table it makes is live from then on.
	in.migrate(e2eNote)
	if got := edge.tables(); !slices.Contains(got, "archive.revision") {
		t.Errorf("a migration announced %v, want archive.revision", got)
	}
	if st, out := e2eRun(t, api, tok, `await archive.create("note", { title: "hi" }); return 1`); st != 200 || out.Error != "" {
		t.Fatalf("create note: %d %v", st, out)
	}
	if got := edge.tables(); !slices.Contains(got, "note") {
		t.Errorf("a write to a new table announced %v, want note", got)
	}
	if strings.Contains(in.must("schema", "show", "note"), "archive_moved") {
		t.Errorf("schema show shows the live trigger")
	}

	// A workspace's look moves it as a rename does.
	if st, _ := call(t, "PATCH", srv.url+"/auth/workspace", tok, map[string]any{"icon": "oak", "color": 2}, nil); st != 200 {
		t.Fatalf("set the look: %d", st)
	}
	if got := edge.tables(); !slices.Contains(got, "archive.workspace") {
		t.Errorf("a new look announced %v, want archive.workspace", got)
	}
}

func TestLiveSocketsHearTheirWorkspace(t *testing.T) {
	t.Parallel()
	in := newInst(t)
	in.must("user", "add", "alice@example.com")
	in.push(e2eSchema)
	srv := in.serve()
	api := srv.url + "/api"
	tok := decode[Token](t, in.must("token", "create")).Secret

	dial := func(token, workspace string) (*websocket.Conn, error) {
		cfg, _ := websocket.NewConfig(strings.Replace(srv.url, "http", "ws", 1)+"/live?workspace="+workspace, "http://localhost:5173")
		cfg.Header = http.Header{"Authorization": {"Bearer " + token}, "X-Forwarded-For": {"203.0.113.7"}}
		return websocket.DialConfig(cfg)
	}
	if _, err := dial("arc_nope", ""); err == nil {
		t.Fatal("a socket with a bad token was let in")
	}
	ws, err := dial(tok, "")
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	frame := func() map[string]any {
		t.Helper()
		ws.SetReadDeadline(time.Now().Add(5 * time.Second))
		var s string
		if err := websocket.Message.Receive(ws, &s); err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		json.Unmarshal([]byte(s), &m)
		return m
	}
	if f := frame(); f["event"] != "ready" || f["workspace"] == "" {
		t.Fatalf("first frame %v, want ready", f)
	}
	e2eRun(t, api, tok, `await archive.create("company", { name: "Acme" }); return 1`)
	f := frame()
	b, _ := json.Marshal(f)
	if f["event"] != "change" || !strings.Contains(string(b), `"table":"company"`) {
		t.Fatalf("after a create: %s", b)
	}
	websocket.Message.Send(ws, "ping")
	ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	var pong string
	if err := websocket.Message.Receive(ws, &pong); err != nil || pong != "pong" {
		t.Fatalf("ping: %q %v", pong, err)
	}

	// Another process on this machine, the CLI, has its writes heard through the serving one.
	resp, err := http.Post(srv.url+"/live/announce", "application/json", strings.NewReader(`{"workspace":"`+frameWorkspace(in)+`","changes":[{"table":"person"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("local announce: %s", resp.Status)
	}
	if b, _ := json.Marshal(frame()); !strings.Contains(string(b), `"table":"person"`) {
		t.Fatalf("after a local announce: %s", b)
	}
}

func frameWorkspace(in *e2eInst) string {
	return strings.TrimSpace(in.psql("archive", "SELECT id FROM archive.workspace ORDER BY created_at LIMIT 1"))
}
