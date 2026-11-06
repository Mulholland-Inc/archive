package main

import (
	"bytes"
	"cmp"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
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
	liveWindow  = 100 * time.Millisecond
	liveIDs     = 20
	liveSettle  = 2 * time.Second
	liveSockets = 512
	liveFrame   = 1024
	liveTicket  = 5 * time.Minute
)

type liveChange struct {
	Table string `json:"table"`
	ID    string `json:"id,omitempty"`
}

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
	to, key string
	secret  []byte
	serve   bool
	client  *http.Client

	mu      sync.Mutex
	pending map[string]*livePending
	timer   *time.Timer
	sending sync.WaitGroup

	smu   sync.Mutex
	socks map[string]map[*liveSocket]bool
}

func newLive(api string, key []byte) *liveHub {
	l := &liveHub{key: strings.TrimSpace(os.Getenv("ARCHIVE_LIVE_KEY")), client: &http.Client{Timeout: 5 * time.Second},
		pending: map[string]*livePending{}, socks: map[string]map[*liveSocket]bool{}}
	if l.key != "" {
		l.to = strings.TrimRight(coreEnv("ARCHIVE_LIVE_URL", api), "/") + "/live/announce"
		l.secret = []byte(l.key)
	} else {
		l.serve = os.Getenv("K_REVISION") == ""
		l.secret = secretDerive(key, "live")
	}
	return l
}

type liveClaims struct {
	User      string `json:"user"`
	Workspace string `json:"workspace"`
	Exp       int64  `json:"exp"`
}

func (l *liveHub) sign(claims string) string {
	m := hmac.New(sha256.New, l.secret)
	m.Write([]byte("live ticket " + claims))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

func (l *liveHub) ticket(ws, user string) string {
	b, _ := json.Marshal(liveClaims{User: user, Workspace: ws, Exp: time.Now().Add(liveTicket).Unix()})
	claims := base64.RawURLEncoding.EncodeToString(b)
	return claims + "." + l.sign(claims)
}

func (l *liveHub) verify(ticket string) (liveClaims, error) {
	var c liveClaims
	claims, sig, _ := strings.Cut(ticket, ".")
	if !hmac.Equal([]byte(sig), []byte(l.sign(claims))) {
		return c, fmt.Errorf("want a ticket from /api/boot?only=live: %w", ErrUnauthorized)
	}
	b, err := base64.RawURLEncoding.DecodeString(claims)
	if err == nil {
		err = json.Unmarshal(b, &c)
	}
	if err != nil || c.Exp <= time.Now().Unix() {
		return c, fmt.Errorf("this ticket has expired; get a new one from /api/boot?only=live: %w", ErrUnauthorized)
	}
	return c, nil
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

func (l *liveHub) drop(ws, user string) {
	if !l.on() || ws == "" || user == "" {
		return
	}
	l.mu.Lock()
	l.take(ws).drop[user] = true
	l.mu.Unlock()
}

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

func (l *liveHub) waiting() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.pending) > 0
}

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
	req.Header.Set("Authorization", "Bearer "+l.key)
	resp, err := l.client.Do(req)
	if err != nil {
		return err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s answered %s", l.to, resp.Status)
	}
	return nil
}

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

func (a *Archive) moved(ctx context.Context, table, id string) {
	storeAfter(ctx, func() { a.hub.live.moved(a.ws.ID.String(), liveChange{Table: table, ID: id}) })
}

func (h *Hub) liveUser(ctx context.Context, ids ...uuid.UUID) {
	if !h.live.on() {
		return
	}
	rows, err := h.q().UsersWorkspaces(context.WithoutCancel(ctx), ids)
	if err != nil {
		return
	}
	for _, u := range rows {
		for _, ws := range u.Workspaces {
			h.live.moved(ws.String(), liveChange{Table: "archive.user", ID: u.ID.String()})
		}
	}
}

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

type liveSocket struct {
	c    *websocket.Conn
	user string
	exp  int64
	wmu  sync.Mutex
}

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
		if len(a.Changes) == 0 {
			continue
		}
		if s.exp <= time.Now().Unix() {
			s.c.Close()
			continue
		}
		s.send(frame)
	}
}

func (h *Hub) liveHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.live.serve {
		http.Error(w, "live updates are served at the edge", http.StatusNotFound)
		return
	}
	if o := r.Header.Get("Origin"); o != "" && o != httpOrigin(h.app) && o != httpOrigin(h.api) {
		http.Error(w, "this origin may not listen", http.StatusForbidden)
		return
	}
	t, err := h.live.verify(r.URL.Query().Get("ticket"))
	if err != nil {
		httpWrite(w, 0, nil, err)
		return
	}
	ws := t.Workspace
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
			s := &liveSocket{c: c, user: t.User, exp: t.Exp}
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
