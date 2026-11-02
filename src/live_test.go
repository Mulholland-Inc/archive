package main

import (
	"encoding/json"
	"io"
	"net/http"
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
