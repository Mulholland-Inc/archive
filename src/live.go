package main

// Live data: what a committed write moved, told to every browser looking at that workspace, so it re-reads.
//
// Where it starts. Every table a workspace writes has a trigger, archive.moved (archive.sql), that sends what
// it moved with pg_notify on a channel named for the writing session, archive_moved_<pid>. Every pooled
// connection listens on its own channel only (livePool), so Postgres hands each session back exactly what it
// committed: a notification is delivered when its transaction commits, never for one that rolls back, and is
// dropped with a rolled-back savepoint (archive.test). It arrives on the same connection, inside the COMMIT
// it follows, in the goroutine that committed. So every write path (archive.create, files, chats, workflows,
// members, migrations, background work) is live with no code of its own, and a new table is too: Migrate
// gives every public table the trigger (schemaMoved). The rows of archive.user, archive.token and
// archive.workspace live in the main database, are shared by every workspace and are written by the Hub
// outside any workspace's store, so those few writes say so themselves (liveUser, Archive.moved).
//
// What it carries. A change names the table that moved ("invoice", or "archive.chat" for Archive's own) and,
// for Archive's own rows, the row's id. Never row data: a browser re-reads through the API, so row-level
// security still decides what it sees.
//
// Where it goes. Changes are held per workspace for liveWindow and sent together, and every request sends
// what it moved before it ends (liveHub.after), since Cloud Run takes the CPU away once a request is done.
//   - With ARCHIVE_LIVE_KEY, to the edge: one POST to the app Worker's /live/announce, which relays it to
//     the workspace's Durable Object and on to every socket there (apps/web/worker.js). The browser's
//     socket is held at the edge, never here.
//   - Without it, archive serve is its own edge: it holds the sockets in process at /live, with the same
//     frames, and another archive process on this machine (the CLI) announces to it. On Cloud Run without a
//     key live data is off, rather than holding sockets that keep instances awake.

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/net/websocket"
)

const (
	// liveWindow is how long changes are held to be sent together.
	liveWindow = 100 * time.Millisecond
	// liveIDs is how many ids of one table a send names; past it, the table alone says it moved.
	liveIDs = 20
	// liveSettle bounds how long a request waits for its announces after its answer.
	liveSettle = 2 * time.Second
	// liveSockets bounds the sockets one workspace holds in process, as the Durable Object does.
	liveSockets = 512
	// liveFrame is the largest frame a client may send; a client says nothing yet but ping.
	liveFrame = 1024
)

// liveChange is one thing that moved: a table, and for Archive's own tables the row.
type liveChange struct {
	Table string `json:"table"`
	ID    string `json:"id,omitempty"`
}

// liveAnnounce is what one send tells a workspace: what moved, and the users whose sockets must close
// because they left it.
type liveAnnounce struct {
	Workspace string       `json:"workspace"`
	Changes   []liveChange `json:"changes,omitempty"`
	Drop      []string     `json:"drop,omitempty"`
}

type livePending struct {
	changes map[liveChange]bool
	drop    map[string]bool
}

type liveHub struct {
	// to is where announces go: the edge's /live/announce, or a local archive serve's; key authenticates
	// them to the edge. serve says this process holds the sockets itself.
	to, key string
	serve   bool
	client  *http.Client

	mu      sync.Mutex
	pending map[string]*livePending
	timer   *time.Timer
	sending sync.WaitGroup

	smu   sync.Mutex
	socks map[string]map[*liveSocket]bool
}

// newLive is the hub of a process that does not serve: it announces to the edge with a key, to an archive
// serve on this machine without one, and nowhere else.
func newLive(api string, local bool) *liveHub {
	l := &liveHub{key: strings.TrimSpace(os.Getenv("ARCHIVE_LIVE_KEY")), client: &http.Client{Timeout: 5 * time.Second},
		pending: map[string]*livePending{}, socks: map[string]map[*liveSocket]bool{}}
	switch {
	case l.key != "":
		l.to = strings.TrimRight(coreEnv("ARCHIVE_LIVE_URL", api), "/") + "/live/announce"
	case local:
		l.to, l.client.Timeout = api+"/live/announce", time.Second
	}
	return l
}

// serving makes this process hold the sockets, unless the edge does or it runs on Cloud Run, which names
// every instance's revision in K_REVISION: a socket held there keeps an instance awake.
func (l *liveHub) serving() {
	if l.key == "" {
		l.to, l.serve = "", os.Getenv("K_REVISION") == ""
	}
}

func (l *liveHub) on() bool { return l != nil && (l.serve || l.to != "") }

func (l *liveHub) take(ws string) *livePending {
	p := l.pending[ws]
	if p == nil {
		p = &livePending{changes: map[liveChange]bool{}, drop: map[string]bool{}}
		l.pending[ws] = p
	}
	if l.timer == nil {
		l.timer = time.AfterFunc(liveWindow, l.flush)
	}
	return p
}

// moved holds a change to send with the next flush. Past liveIDs ids of one table, the table alone stands
// for them all.
func (l *liveHub) moved(ws string, c liveChange) {
	if !l.on() || ws == "" || c.Table == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	p := l.take(ws)
	if p.changes[liveChange{Table: c.Table}] {
		return
	}
	if c.ID != "" {
		n := 0
		for k := range p.changes {
			if k.Table == c.Table {
				n++
			}
		}
		if n >= liveIDs {
			for k := range p.changes {
				if k.Table == c.Table {
					delete(p.changes, k)
				}
			}
			c.ID = ""
		}
	}
	p.changes[c] = true
}

// drop closes a user's sockets on a workspace they left.
func (l *liveHub) drop(ws, user string) {
	if !l.on() || ws == "" || user == "" {
		return
	}
	l.mu.Lock()
	l.take(ws).drop[user] = true
	l.mu.Unlock()
}

// flush sends everything held, one announce per workspace.
func (l *liveHub) flush() {
	l.mu.Lock()
	pending := l.pending
	l.pending = map[string]*livePending{}
	if l.timer != nil {
		l.timer.Stop()
		l.timer = nil
	}
	for ws, p := range pending {
		a := liveAnnounce{Workspace: ws}
		for c := range p.changes {
			a.Changes = append(a.Changes, c)
		}
		slices.SortFunc(a.Changes, func(x, y liveChange) int { return cmp.Or(cmp.Compare(x.Table, y.Table), cmp.Compare(x.ID, y.ID)) })
		for u := range p.drop {
			a.Drop = append(a.Drop, u)
		}
		slices.Sort(a.Drop)
		l.sending.Add(1)
		go func() {
			defer l.sending.Done()
			if err := l.send(a); err != nil {
				fmt.Fprintf(os.Stderr, "archive: workspace %s live: %v\n", ws, err)
			}
		}()
	}
	l.mu.Unlock()
}

// waiting says whether anything is held.
func (l *liveHub) waiting() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.pending) > 0
}

// settle sends what is held and waits, up to d, until every send is done.
func (l *liveHub) settle(d time.Duration) {
	if l == nil {
		return
	}
	l.flush()
	done := make(chan struct{})
	go func() { l.sending.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(d):
	}
}

// after sends what a request moved before the request ends, its answer first.
func (l *liveHub) after(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		if r.URL.Path == "/live" || !l.waiting() {
			return
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		l.settle(liveSettle)
	})
}

func (l *liveHub) send(a liveAnnounce) error {
	if l.serve {
		l.relay(a)
		return nil
	}
	if l.to == "" {
		return nil
	}
	b, _ := json.Marshal(a)
	req, err := http.NewRequest(http.MethodPost, l.to, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if l.key != "" {
		req.Header.Set("Authorization", "Bearer "+l.key)
	}
	resp, err := l.client.Do(req)
	if err != nil {
		if l.key == "" {
			return nil // no archive serve on this machine to tell
		}
		return err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s answered %s", l.to, resp.Status)
	}
	return nil
}

// livePool makes every connection of a workspace's pool listen on its own channel and tell moved what it
// committed (see the top of this file).
func livePool(cfg *pgxpool.Config, moved func(table, id string)) {
	cfg.AfterConnect = func(ctx context.Context, c *pgx.Conn) error {
		_, err := c.Exec(ctx, "LISTEN "+pgIdent("archive_moved_"+strconv.FormatUint(uint64(c.PgConn().PID()), 10)))
		return err
	}
	cfg.ConnConfig.OnNotification = func(pc *pgconn.PgConn, n *pgconn.Notification) {
		if n.PID != pc.PID() || !strings.HasPrefix(n.Channel, "archive_moved_") {
			return
		}
		table, id, _ := strings.Cut(n.Payload, " ")
		moved(table, id)
	}
}

// moved tells the workspace a row of the main database moved: a token, the workspace itself.
func (a *Archive) moved(ctx context.Context, table, id string) {
	storeAfter(ctx, func() { a.hub.live.moved(a.ws.ID.String(), liveChange{Table: table, ID: id}) })
}

// liveUser tells every workspace a user is in that their account moved, so their app re-reads who they are
// and which workspaces they have, wherever it is open.
func (h *Hub) liveUser(ctx context.Context, ids ...uuid.UUID) {
	if !h.live.on() {
		return
	}
	for _, id := range ids {
		u, err := h.q().UserByID(context.WithoutCancel(ctx), id)
		if err != nil {
			continue
		}
		for _, ws := range u.Workspaces {
			h.live.moved(ws.String(), liveChange{Table: "archive.user", ID: id.String()})
		}
	}
}

// liveMembers is how to tell every member of a workspace that their account moved: the workspace was renamed
// or deleted, which the list of workspaces each of them has shows. It finds them now and tells them when
// called, so a deletion can be told once it is done.
func (h *Hub) liveMembers(ctx context.Context, ws uuid.UUID) func() {
	if !h.live.on() {
		return func() {}
	}
	users, _ := h.q().WorkspaceUsers(context.WithoutCancel(ctx), ws)
	ids := make([]uuid.UUID, len(users))
	for i, u := range users {
		ids[i] = u.ID
	}
	return func() { h.liveUser(ctx, ids...) }
}

// liveTables gives tables made before archive.moved existed its trigger, once per workspace, when it opens.
func (a *Archive) liveTables(ctx context.Context) error {
	return a.st.System(ctx, func(ctx context.Context) error {
		q := a.st.db(ctx)
		if err := q.AdvisoryXactLock(ctx, storeLock); err != nil {
			return err
		}
		tx := a.st.Q(ctx)
		return schemaMoved(ctx, tx, func(st string) error {
			_, err := tx.Exec(ctx, st)
			return err
		})
	})
}

// The sockets archive serve holds itself, when no edge holds them. The frames are the Durable Object's:
// {"event":"ready","workspace":...} once, then {"event":"change","changes":[{table, id}]} per send; a client
// may send "ping" and is answered "pong". A user who leaves the workspace has their sockets closed.

type liveSocket struct {
	c    *websocket.Conn
	user string
	wmu  sync.Mutex
}

// send writes one frame: a string as it is, anything else as JSON. A write that cannot finish in 10 seconds
// fails, and the deadline is cleared after, so it never outlives the write.
func (s *liveSocket) send(v any) error {
	msg, ok := v.(string)
	if !ok {
		b, _ := json.Marshal(v)
		msg = string(b)
	}
	s.wmu.Lock()
	defer s.wmu.Unlock()
	s.c.SetWriteDeadline(time.Now().Add(10 * time.Second))
	defer s.c.SetWriteDeadline(time.Time{})
	return websocket.Message.Send(s.c, msg)
}

func (l *liveHub) relay(a liveAnnounce) {
	l.smu.Lock()
	socks := make([]*liveSocket, 0, len(l.socks[a.Workspace]))
	for s := range l.socks[a.Workspace] {
		socks = append(socks, s)
	}
	l.smu.Unlock()
	frame := map[string]any{"event": "change", "changes": a.Changes}
	for _, s := range socks {
		if slices.Contains(a.Drop, s.user) {
			s.c.Close()
			continue
		}
		if len(a.Changes) > 0 {
			s.send(frame)
		}
	}
}

// liveAnnounceHTTP takes an announce from another archive process on this machine, the CLI.
func (h *Hub) liveAnnounceHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.live.serve || !h.local(r) {
		http.NotFound(w, r)
		return
	}
	var a liveAnnounce
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&a); err != nil || a.Workspace == "" {
		http.Error(w, "want {workspace, changes}", http.StatusBadRequest)
		return
	}
	for _, c := range a.Changes {
		h.live.moved(a.Workspace, c)
	}
	for _, u := range a.Drop {
		h.live.drop(a.Workspace, u)
	}
	w.WriteHeader(http.StatusNoContent)
}

// liveReader is who may listen to a workspace: the session's user, checked as every API request is, by the
// cookie (or a bearer token) and the workspace named in ?workspace= or Archive-Workspace, since a browser's
// WebSocket can send no header of its own.
func (h *Hub) liveReader(r *http.Request) (ws, user string, err error) {
	want := cmp.Or(r.URL.Query().Get("workspace"), r.Header.Get("Archive-Workspace"))
	var a *Archive
	var ac *Actor
	var release func()
	secret, err := h.secret(r)
	if err == nil {
		a, ac, release, err = h.authenticate(r.Context(), secret, want)
	}
	if errors.Is(err, ErrUnauthorized) && r.Header.Get("Authorization") == "" && h.local(r) {
		a, ac, release, err = h.localUser(r.Context(), want)
	}
	if err != nil {
		return "", "", err
	}
	release()
	return a.ws.ID.String(), ac.UserID.String(), nil
}

// liveCheckHTTP answers the edge Worker, which asks before it lets a browser's socket through to a
// workspace's Durable Object, with the browser's cookie: whether this session may read the workspace, and as
// whom.
func (h *Hub) liveCheckHTTP(w http.ResponseWriter, r *http.Request) {
	ws, user, err := h.liveReader(r)
	httpWrite(w, http.StatusOK, map[string]string{"workspace": ws, "user": user}, err)
}

// liveHTTP is the socket at /live, when this process holds the sockets.
func (h *Hub) liveHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.live.serve {
		http.Error(w, "live updates are served at the edge", http.StatusNotFound)
		return
	}
	if o := r.Header.Get("Origin"); o != "" && o != httpOrigin(h.app) && o != httpOrigin(h.api) {
		http.Error(w, "this origin may not listen", http.StatusForbidden)
		return
	}
	ws, user, err := h.liveReader(r)
	if err != nil {
		httpWrite(w, 0, nil, err)
		return
	}
	h.live.smu.Lock()
	full := len(h.live.socks[ws]) >= liveSockets
	h.live.smu.Unlock()
	if full {
		http.Error(w, "too many listeners", http.StatusTooManyRequests)
		return
	}
	websocket.Server{
		Handshake: func(*websocket.Config, *http.Request) error { return nil },
		Handler: func(c *websocket.Conn) {
			c.MaxPayloadBytes = liveFrame
			s := &liveSocket{c: c, user: user}
			h.live.smu.Lock()
			if h.live.socks[ws] == nil {
				h.live.socks[ws] = map[*liveSocket]bool{}
			}
			h.live.socks[ws][s] = true
			h.live.smu.Unlock()
			defer func() {
				h.live.smu.Lock()
				delete(h.live.socks[ws], s)
				if len(h.live.socks[ws]) == 0 {
					delete(h.live.socks, ws)
				}
				h.live.smu.Unlock()
				c.Close()
			}()
			if s.send(map[string]any{"event": "ready", "workspace": ws}) != nil {
				return
			}
			for {
				var msg string
				if err := websocket.Message.Receive(c, &msg); err != nil {
					return
				}
				if msg == "ping" {
					s.send("pong")
				}
			}
		},
	}.ServeHTTP(w, r)
}
