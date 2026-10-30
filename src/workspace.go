package main

import (
	"bytes"
	"cmp"
	"context"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Mulholland-Inc/archive/generated"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Hub struct {
	key    []byte
	aead   cipher.AEAD
	pooled *pgx.ConnConfig
	direct *pgx.ConnConfig
	main   *pgxpool.Pool
	blobs  blobStore
	hints  Hints
	app    string
	api    string
	one    string
	tasks  *hubTasks

	mu   sync.Mutex
	open map[string]*hubEntry
}

type hubEntry struct {
	ready chan struct{}
	a     *Archive
	err   error
	users int
	idle  *time.Timer
}

type hubTasks struct {
	queue, account, base string
	gcp                  *gcpToken

	mu     sync.Mutex
	verify *oidc.IDTokenVerifier
}

type gcpToken struct {
	meta  string
	mu    sync.Mutex
	token string
	exp   time.Time
}

func gcpMetadata() *gcpToken {
	return &gcpToken{meta: "http://" + coreEnv("GCE_METADATA_HOST", "metadata.google.internal")}
}

type Workspace struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at,omitzero"`
}

const (
	hubIdle       = 10 * time.Minute
	hubWorkers    = 4
	hubBatch      = 16
	hubTaskMax    = 29 * 24 * time.Hour
	hubFileDelay  = 10 * time.Second
	hubBusy       = 10 * time.Second
	workspaceRuns = 8
	workspaceMax  = 64
)

type archiveKey struct{}

func archiveFrom(ctx context.Context) *Archive {
	a, _ := ctx.Value(archiveKey{}).(*Archive)
	return a
}

func withArchive(ctx context.Context, a *Archive) context.Context {
	return context.WithValue(ctx, archiveKey{}, a)
}

func Open(ctx context.Context) (*Hub, error) {
	dir := coreEnv("ARCHIVE_DATA", "./data")
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return nil, errors.New("DATABASE_URL is required: a Postgres 18 role with CREATEROLE and CREATEDB, like the one docker compose up -d postgres starts")
	}
	var tasks *hubTasks
	if q := os.Getenv("ARCHIVE_TASKS_QUEUE"); q != "" {
		tasks = &hubTasks{queue: q, account: os.Getenv("ARCHIVE_TASKS_SERVICE_ACCOUNT"), base: coreEnv("ARCHIVE_TASKS_URL", "https://cloudtasks.googleapis.com"),
			gcp: gcpMetadata()}
		if tasks.account == "" {
			return nil, errors.New("ARCHIVE_TASKS_QUEUE needs ARCHIVE_TASKS_SERVICE_ACCOUNT, the service account whose OIDC token Cloud Tasks sends to /tasks")
		}
	}
	key, err := secretLoadKey(dir)
	if err != nil {
		return nil, fmt.Errorf("secret key: %w", err)
	}
	aead, err := secretAEAD(key)
	if err != nil {
		return nil, err
	}
	blobs, err := blobOpen(dir)
	if err != nil {
		return nil, err
	}
	pooled, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("DATABASE_URL: %w", err)
	}
	direct := pooled
	if d := os.Getenv("DATABASE_DIRECT_URL"); d != "" {
		if direct, err = pgx.ParseConfig(d); err != nil {
			return nil, fmt.Errorf("DATABASE_DIRECT_URL: %w", err)
		}
	}
	h := &Hub{key: key, aead: aead, pooled: pooled, direct: direct, blobs: blobs, hints: coreHints(os.Getenv("ARCHIVE_LOCALE")),
		tasks: tasks, open: map[string]*hubEntry{}}
	h.urls("8080")
	if h.main, err = storePool(ctx, pooled, storeSysConns, nil); err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	if err := h.register(ctx); err != nil {
		h.Close()
		return nil, fmt.Errorf("open store: %w", err)
	}
	return h, nil
}

func (h *Hub) urls(port string) {
	h.app = strings.TrimRight(coreEnv("ARCHIVE_URL", "http://localhost:3000"), "/")
	h.api = strings.TrimRight(coreEnv("ARCHIVE_API_URL", "http://localhost:"+port), "/")
}

func storeUndefined(err error) bool {
	var pe *pgconn.PgError
	return errors.As(err, &pe) && (pe.Code == "42P01" || pe.Code == "3F000")
}

func (h *Hub) register(ctx context.Context) error {
	q := h.q()
	row, err := q.FirstWorkspace(ctx)
	fresh := errors.Is(err, pgx.ErrNoRows) || storeUndefined(err)
	if err != nil && !fresh {
		return err
	}
	var key []byte
	if fresh {
		key = make([]byte, 32)
		rand.Read(key)
		row = generated.ArchiveWorkspace{ID: coreNewID(), Name: "Archive"}
	} else if key, err = h.unwrap(row); err != nil {
		return err
	}
	h.one = row.ID.String()
	a, err := h.openArchive(ctx, row, key)
	if err != nil {
		return err
	}
	if fresh {
		r, err := q.CreateWorkspace(ctx, row.ID, row.Name, h.wrap(h.one, key))
		if errors.Is(err, pgx.ErrNoRows) {
			r, err = q.FirstWorkspace(ctx)
		}
		if err != nil {
			a.Close()
			return err
		}
		a.ws = r
	}
	ready := make(chan struct{})
	close(ready)
	h.open[h.one] = &hubEntry{ready: ready, a: a}
	a.afterOpen(ctx)
	return nil
}

func (h *Hub) Close() error {
	h.mu.Lock()
	for k, e := range h.open {
		if e.a != nil {
			e.a.Close()
		}
		delete(h.open, k)
	}
	h.mu.Unlock()
	if h.main != nil {
		h.main.Close()
	}
	return nil
}

func (h *Hub) q() *generated.Queries { return generated.New(h.main) }

func (h *Hub) wrap(id string, key []byte) []byte {
	nonce := make([]byte, h.aead.NonceSize())
	rand.Read(nonce)
	return h.aead.Seal(nonce, nonce, key, []byte("workspace "+id))
}

func (h *Hub) unwrap(row generated.ArchiveWorkspace) ([]byte, error) {
	n := h.aead.NonceSize()
	if len(row.Key) < n {
		return nil, fmt.Errorf("workspace %s: its data key is corrupt", row.ID)
	}
	k, err := h.aead.Open(nil, row.Key[:n], row.Key[n:], []byte("workspace "+row.ID.String()))
	if err != nil {
		return nil, fmt.Errorf("workspace %s: its data key does not open with ARCHIVE_SECRET_KEY: %w", row.ID, err)
	}
	return k, nil
}

func (h *Hub) database(id string) string {
	if id == h.one {
		return h.pooled.Database
	}
	return "archive_" + id
}

func (h *Hub) openArchive(ctx context.Context, row generated.ArchiveWorkspace, key []byte) (*Archive, error) {
	id := row.ID.String()
	pooled, direct := h.pooled.Copy(), h.direct.Copy()
	pooled.Database, direct.Database = h.database(id), h.database(id)
	st, err := OpenStore(ctx, pooled, direct, id, authPassword(key))
	if err != nil {
		return nil, err
	}
	cat, err := st.Catalog(ctx)
	if err != nil {
		st.Close()
		return nil, fmt.Errorf("load catalog: %w", err)
	}
	aead, err := secretAEAD(key)
	if err != nil {
		st.Close()
		return nil, err
	}
	return &Archive{hub: h, ws: row, st: st, cat: cat, hints: h.hints, prefix: id, aead: aead,
		blobs: blobPrefixed(h.blobs, id+"/"), linkKey: secretDerive(key, "files"), slots: make(chan struct{}, workspaceRuns)}, nil
}

func (h *Hub) acquire(ctx context.Context, short string) (*Archive, func(), error) {
	short = cmp.Or(short, h.one)
	if _, err := uuid.Parse(short); err != nil {
		return nil, nil, fmt.Errorf("workspace %q: %w", short, ErrNotFound)
	}
	h.mu.Lock()
	e := h.open[short]
	if e == nil {
		e = &hubEntry{ready: make(chan struct{}), users: 1}
		h.open[short] = e
		h.mu.Unlock()
		a, err := h.load(context.WithoutCancel(ctx), short)
		h.mu.Lock()
		e.a, e.err = a, err
		if err != nil {
			delete(h.open, short)
		}
		close(e.ready)
		h.mu.Unlock()
	} else {
		e.users++
		if e.idle != nil {
			e.idle.Stop()
			e.idle = nil
		}
		h.mu.Unlock()
		select {
		case <-e.ready:
		case <-ctx.Done():
			h.done(short, e)
			return nil, nil, ctx.Err()
		}
	}
	if e.err != nil {
		h.done(short, e)
		return nil, nil, e.err
	}
	var once sync.Once
	return e.a, func() { once.Do(func() { h.done(short, e) }) }, nil
}

func (h *Hub) done(short string, e *hubEntry) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if e.users--; e.users > 0 || short == h.one || e.a == nil {
		return
	}
	e.idle = time.AfterFunc(hubIdle, func() {
		h.mu.Lock()
		idle := h.open[short] == e && e.users == 0
		if idle {
			delete(h.open, short)
		}
		h.mu.Unlock()
		if idle {
			e.a.Close()
		}
	})
}

func (h *Hub) load(ctx context.Context, short string) (*Archive, error) {
	id, err := uuid.Parse(short)
	if err != nil {
		return nil, fmt.Errorf("workspace %q: %w", short, ErrNotFound)
	}
	row, err := h.q().WorkspaceByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("workspace %q: %w", short, ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	key, err := h.unwrap(row)
	if err != nil {
		return nil, err
	}
	a, err := h.openArchive(ctx, row, key)
	if err != nil {
		return nil, fmt.Errorf("workspace %s: %w", short, err)
	}
	a.afterOpen(ctx)
	return a, nil
}

func (h *Hub) forget(short string) {
	h.mu.Lock()
	e := h.open[short]
	delete(h.open, short)
	h.mu.Unlock()
	if e != nil && e.a != nil {
		e.a.Close()
	}
}

func (a *Archive) afterOpen(ctx context.Context) {
	err := a.st.System(ctx, func(ctx context.Context) error {
		return a.st.db(ctx).ResetProcessing(ctx)
	})
	for _, kind := range []string{"workflow", "file"} {
		next, nerr := a.nextDue(ctx, kind)
		if nerr == nil && next != nil {
			nerr = a.soon(ctx, kind, *next)
		}
		err = errors.Join(err, nerr)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "archive: workspace %s: %v\n", a.ws.ID, err)
	}
}

func (a *Archive) soon(ctx context.Context, kind string, at time.Time) error {
	ctx = context.WithoutCancel(ctx)
	r, err := a.hub.q().SoonDue(ctx, a.ws.ID, kind, at)
	switch {
	case err != nil:
		return err
	case r.Changed:
		return a.hub.wake(ctx, a.ws.ID, kind, r.At)
	case time.Since(r.At) > workflowRetry:
		return a.hub.wake(ctx, a.ws.ID, kind, time.Now())
	}
	return nil
}

func (h *Hub) wake(ctx context.Context, ws uuid.UUID, kind string, at time.Time) error {
	if h.tasks == nil {
		return h.q().NotifyDue(ctx, at.Format(time.RFC3339Nano))
	}
	t := h.tasks
	if limit := time.Now().Add(hubTaskMax); at.After(limit) {
		at = limit
	}
	tok, err := t.gcp.access(ctx)
	if err != nil {
		return err
	}
	url := h.api + "/tasks"
	body, _ := json.Marshal(map[string]string{"workspace": ws.String(), "work": kind + "s"})
	sum := sha256.Sum256(fmt.Appendf(nil, "%s/%s/%d", ws, kind, at.UnixMilli()))
	task, _ := json.Marshal(map[string]any{"task": map[string]any{
		"name":         t.queue + "/tasks/" + hex.EncodeToString(sum[:]),
		"scheduleTime": at.UTC().Format(time.RFC3339Nano),
		"httpRequest": map[string]any{"httpMethod": "POST", "url": url, "headers": map[string]string{"Content-Type": "application/json"}, "body": body,
			"oidcToken": map[string]string{"serviceAccountEmail": t.account, "audience": url}},
	}})
	req, _ := http.NewRequestWithContext(ctx, "POST", t.base+"/v2/"+t.queue+"/tasks", bytes.NewReader(task))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	resp, err := hubHTTP.Do(req)
	if err != nil {
		return fmt.Errorf("cloud tasks: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 == 2 || resp.StatusCode == http.StatusConflict {
		return nil
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<10))
	return fmt.Errorf("cloud tasks: %s: %s", resp.Status, bytes.TrimSpace(b))
}

var hubHTTP = &http.Client{Timeout: 10 * time.Second}

func (t *gcpToken) access(ctx context.Context) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.token != "" && time.Now().Before(t.exp) {
		return t.token, nil
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", t.meta+"/computeMetadata/v1/instance/service-accounts/default/token", nil)
	req.Header.Set("Metadata-Flavor", "Google")
	resp, err := hubHTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("metadata server: %w", err)
	}
	defer resp.Body.Close()
	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil || resp.StatusCode != http.StatusOK || tok.AccessToken == "" {
		return "", fmt.Errorf("metadata server: no access token (%s)", resp.Status)
	}
	t.token, t.exp = tok.AccessToken, time.Now().Add(time.Duration(tok.ExpiresIn)*time.Second-time.Minute)
	return t.token, nil
}

func (h *Hub) taskCaller(r *http.Request) error {
	t := h.tasks
	tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok {
		return fmt.Errorf("want the OIDC token Cloud Tasks sends: %w", ErrUnauthorized)
	}
	t.mu.Lock()
	if t.verify == nil {
		p, err := oidc.NewProvider(context.Background(), coreEnv("ARCHIVE_GOOGLE_URL", signinBuiltin[0].base))
		if err != nil {
			t.mu.Unlock()
			return fmt.Errorf("google's signing keys: %w", err)
		}
		t.verify = p.Verifier(&oidc.Config{ClientID: h.api + "/tasks"})
	}
	v := t.verify
	t.mu.Unlock()
	id, err := v.Verify(r.Context(), strings.TrimSpace(tok))
	if err != nil {
		return fmt.Errorf("%v: %w", err, ErrUnauthorized)
	}
	var c struct {
		Email    string `json:"email"`
		Verified bool   `json:"email_verified"`
	}
	if err := id.Claims(&c); err != nil || !c.Verified || c.Email != t.account {
		return fmt.Errorf("only %s calls /tasks: %w", t.account, ErrForbidden)
	}
	return nil
}

func (h *Hub) tasksHTTP(w http.ResponseWriter, r *http.Request) {
	if err := h.taskCaller(r); err != nil {
		httpWrite(w, 0, nil, err)
		return
	}
	var body struct {
		Workspace uuid.UUID `json:"workspace"`
		Work      string    `json:"work"`
	}
	if err := httpDecode(r, &body); err != nil {
		httpWrite(w, 0, nil, err)
		return
	}
	kind, ok := strings.CutSuffix(body.Work, "s")
	if !ok || kind != "workflow" && kind != "file" {
		httpWrite(w, 0, nil, fmt.Errorf("work: want workflows or files: %w", httpErrBadRequest))
		return
	}
	ctx, q := r.Context(), h.q()
	at, err := q.ClaimDueFor(ctx, time.Now().Add(runTimeout()+workflowRetry), body.Workspace, kind)
	if err == nil {
		err = h.due(ctx, body.Workspace, kind, at)
	} else if errors.Is(err, pgx.ErrNoRows) {
		if at, err = q.DueAt(ctx, body.Workspace, kind); err == nil && at.After(time.Now()) {
			err = h.wake(ctx, body.Workspace, kind, at)
		} else if errors.Is(err, pgx.ErrNoRows) {
			err = nil
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "archive: workspace %s %s: %v\n", body.Workspace, body.Work, err)
	}
	httpWrite(w, http.StatusNoContent, nil, err)
}

func (a *Archive) nextDue(ctx context.Context, kind string) (*time.Time, error) {
	var next *time.Time
	return next, a.st.System(ctx, func(ctx context.Context) error {
		q := a.st.db(ctx)
		if kind == "file" {
			pending, err := q.HasPendingFiles(ctx)
			if pending {
				t := time.Now().Add(hubFileDelay)
				next = &t
			}
			return err
		}
		n, err := q.NextWorkflowAt(ctx)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		next = n
		return err
	})
}

func (a *Archive) scheduleWorkflows(ctx context.Context) error {
	next, err := a.nextDue(ctx, "workflow")
	if err != nil || next == nil {
		return err
	}
	return a.soon(ctx, "workflow", *next)
}

func (h *Hub) clock(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	soon, idle := make(chan time.Time), make(chan struct{})
	go h.listen(ctx, soon)
	t := time.NewTimer(time.Hour)
	t.Stop()
	var next time.Time
	arm := func(at time.Time) {
		next = at
		t.Reset(time.Until(at))
	}
	earliest := func() {
		at, err := h.q().EarliestDue(ctx)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			next = time.Time{}
			t.Stop()
		case err != nil:
			if ctx.Err() == nil {
				fmt.Fprintln(os.Stderr, "archive: background work:", err)
			}
			arm(time.Now().Add(workflowRetry))
		default:
			arm(at)
		}
	}
	busy := 0
	for {
		select {
		case <-ctx.Done():
			t.Stop()
			for ; busy > 0; busy-- {
				<-idle
			}
			return
		case at := <-soon:
			if at.IsZero() {
				earliest()
			} else if next.IsZero() || at.Before(next) {
				arm(at)
			}
		case <-t.C:
			next = time.Time{}
			for ; busy < hubWorkers; busy++ {
				go func() {
					h.drain(ctx)
					idle <- struct{}{}
				}()
			}
		case <-idle:
			busy--
			earliest()
		}
	}
}

func (h *Hub) listen(ctx context.Context, soon chan<- time.Time) {
	send := func(at time.Time) {
		select {
		case soon <- at:
		case <-ctx.Done():
		}
	}
	wait := time.Second
	for ctx.Err() == nil {
		c, err := pgx.ConnectConfig(ctx, h.direct)
		if err == nil {
			if _, err = c.Exec(ctx, "LISTEN archive_due"); err == nil {
				wait = time.Second
				send(time.Time{})
			}
			for err == nil {
				var n *pgconn.Notification
				if n, err = c.WaitForNotification(ctx); err == nil {
					at, _ := time.Parse(time.RFC3339Nano, n.Payload)
					send(at)
				}
			}
			c.Close(context.Background())
		}
		if ctx.Err() != nil {
			return
		}
		fmt.Fprintln(os.Stderr, "archive: waiting for due work:", err)
		select {
		case <-ctx.Done():
		case <-time.After(wait):
		}
		wait = min(2*wait, time.Minute)
	}
}

func (h *Hub) drain(ctx context.Context) {
	for ctx.Err() == nil {
		d, err := h.q().ClaimDue(ctx, time.Now().Add(runTimeout()+workflowRetry))
		if errors.Is(err, pgx.ErrNoRows) {
			return
		}
		if err == nil {
			err = h.due(ctx, d.ID, d.Kind, d.At)
		}
		if err != nil {
			if ctx.Err() == nil {
				fmt.Fprintln(os.Stderr, "archive: background work:", err)
			}
			return
		}
	}
}

func (h *Hub) due(ctx context.Context, ws uuid.UUID, kind string, claimed time.Time) error {
	bg := context.WithoutCancel(ctx)
	a, release, err := h.acquire(ctx, ws.String())
	if errors.Is(err, ErrNotFound) {
		return h.q().ClearDue(bg, ws, kind, claimed)
	}
	if err != nil {
		return err
	}
	defer release()
	for range hubBatch {
		var ok bool
		switch kind {
		case "workflow":
			ok, err = a.workflowNext(ctx)
		case "file":
			ok, err = a.fileProcess(ctx, uuid.Nil)
		}
		if err != nil && ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "archive: workspace %s %ss: %v\n", ws, kind, err)
		}
		if !ok || err != nil || ctx.Err() != nil {
			break
		}
	}
	next, err := a.nextDue(bg, kind)
	if err != nil {
		return err
	}
	if next == nil {
		return h.q().ClearDue(bg, ws, kind, claimed)
	}
	n, err := h.q().SetDue(bg, generated.SetDueParams{Next: *next, ID: ws, Kind: kind, Claimed: claimed})
	if err != nil || n == 0 {
		return err
	}
	return h.wake(bg, ws, kind, *next)
}

func (h *Hub) secret(r *http.Request) (string, error) {
	if hd := r.Header.Get("Authorization"); hd != "" {
		tok, ok := strings.CutPrefix(hd, "Bearer ")
		if !ok {
			return "", fmt.Errorf("want Authorization: Bearer <token>: %w", ErrUnauthorized)
		}
		return strings.TrimSpace(tok), nil
	}
	if c, err := r.Cookie(signinCookie); err == nil && c.Value != "" {
		return c.Value, nil
	}
	return "", fmt.Errorf("sign in or send a bearer token: %w", ErrUnauthorized)
}

func (h *Hub) authenticate(ctx context.Context, secret, want string) (*Archive, *Actor, func(), error) {
	t, err := h.q().Authenticate(ctx, authHash(secret))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, nil, fmt.Errorf("invalid or expired token: %w", ErrUnauthorized)
	}
	if err != nil {
		return nil, nil, nil, err
	}
	return h.enter(ctx, t, want)
}

func (h *Hub) localApp() bool {
	app, err := url.Parse(h.app)
	return err == nil && app.Scheme == "http" && localHost(app.Hostname())
}

func (h *Hub) local(r *http.Request) bool {
	if !h.localApp() || r.Header.Get("X-Forwarded-For") != "" || r.Header.Get("Forwarded") != "" {
		return false
	}
	peer, _, _ := net.SplitHostPort(r.RemoteAddr)
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
	}
	return localHost(peer) && localHost(host)
}

func localHost(host string) bool {
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return host == "localhost" || ip != nil && ip.IsLoopback()
}

func (h *Hub) localUser(ctx context.Context, want string) (*Archive, *Actor, func(), error) {
	one, err := uuid.Parse(h.one)
	if err != nil {
		return nil, nil, nil, err
	}
	u, err := h.q().FirstUser(ctx, one)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, nil, fmt.Errorf("no users yet; add the first owner with archive user add <email>: %w", ErrUnauthorized)
	}
	if err != nil {
		return nil, nil, nil, err
	}
	t := generated.AuthenticateRow{Kind: "session", UserID: u.ID, Email: u.Email, UserName: u.Name}
	for _, w := range u.Workspaces {
		t.Workspaces = append(t.Workspaces, w.String())
	}
	return h.enter(ctx, t, want)
}

func (h *Hub) enter(ctx context.Context, t generated.AuthenticateRow, want string) (*Archive, *Actor, func(), error) {
	short := cmp.Or(t.Workspace, want)
	if short == "" && len(t.Workspaces) > 0 {
		short = t.Workspaces[0]
	}
	if want != "" && want != short {
		return nil, nil, nil, fmt.Errorf("this token belongs to another workspace: %w", ErrForbidden)
	}
	if !slices.Contains(t.Workspaces, short) {
		return nil, nil, nil, fmt.Errorf("you are not in this workspace: %w", ErrForbidden)
	}
	a, release, err := h.acquire(ctx, short)
	if err != nil {
		return nil, nil, nil, err
	}
	stamp, err := generated.New(a.st.sys).Revision(ctx)
	if err == nil && stamp != a.Catalog().stamp {
		err = a.reload(ctx)
	}
	if err != nil {
		release()
		return nil, nil, nil, err
	}
	ac := a.userActor(&User{ID: t.UserID, Email: t.Email, Name: t.UserName})
	if t.Kind != "session" {
		ac.TokenName = t.Name
	}
	if t.Kind == "api" {
		ac.Role, ac.TokenID = t.Role, t.ID
	}
	return a, ac, release, nil
}

func (h *Hub) httpAuth(next func(a *Archive) http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var a *Archive
		var ac *Actor
		var release func()
		secret, err := h.secret(r)
		if err == nil {
			a, ac, release, err = h.authenticate(r.Context(), secret, r.Header.Get("Archive-Workspace"))
		}
		if errors.Is(err, ErrUnauthorized) && r.Header.Get("Authorization") == "" && h.local(r) {
			a, ac, release, err = h.localUser(r.Context(), r.Header.Get("Archive-Workspace"))
		}
		if err != nil {
			httpWrite(w, 0, nil, err)
			return
		}
		defer release()
		ac.Space = r.Header.Get("Archive-Space")
		next(a).ServeHTTP(w, r.WithContext(withArchive(WithActor(r.Context(), ac), a)))
	})
}

func (h *Hub) inWorkspace(f func(a *Archive, w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, release, err := h.acquire(r.Context(), r.PathValue("workspace"))
		if errors.Is(err, ErrNotFound) {
			httpWrite(w, http.StatusNotFound, map[string]string{"error": "no workspace here"}, nil)
			return
		}
		if err != nil {
			httpWrite(w, 0, nil, err)
			return
		}
		defer release()
		f(a, w, r.WithContext(withArchive(r.Context(), a)))
	}
}

func (h *Hub) mcp(w http.ResponseWriter, r *http.Request) {
	tok, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	a, ac, release, err := h.authenticate(r.Context(), strings.TrimSpace(tok), "")
	if err != nil {
		httpWrite(w, 0, nil, err)
		return
	}
	defer release()
	a.hmu.Lock()
	if a.mcph == nil {
		srv := mcpServer(a)
		a.mcph = auth.RequireBearerToken(a.mcpVerify, &auth.RequireBearerTokenOptions{AllowMissingExpiration: true})(
			mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil))
	}
	hd := a.mcph
	a.hmu.Unlock()
	hd.ServeHTTP(w, r.WithContext(WithActor(r.Context(), ac)))
}

func (a *Archive) handler() http.Handler {
	a.hmu.Lock()
	defer a.hmu.Unlock()
	if a.api == nil {
		a.api = httpHandler(a)
	}
	return a.api
}

func (a *Archive) isAdmin(ctx context.Context, ac *Actor) bool {
	var ok bool
	a.sys(ctx, func(ctx context.Context) error {
		ok = a.has(ctx, ac.UserRole, a.prefix+"_admin")
		return nil
	})
	return ok
}

func (a *Archive) workspaceAdmin(ctx context.Context) (*Actor, error) {
	ac, err := authUser(ctx)
	if err != nil {
		return nil, err
	}
	if !a.isAdmin(ctx, ac) {
		return nil, fmt.Errorf("only admins of this workspace change it: %w", ErrForbidden)
	}
	return ac, nil
}

func (h *Hub) admin(ctx context.Context) error {
	ac, err := authUser(ctx)
	if err != nil {
		return err
	}
	a := archiveFrom(ctx)
	if a == nil || a.ws.ID.String() != h.one || !a.isAdmin(ctx, ac) {
		return fmt.Errorf("only instance admins, the admins of the first workspace, do this: %w", ErrForbidden)
	}
	return nil
}

func workspaceOut(r generated.ArchiveWorkspace) Workspace {
	return Workspace{ID: r.ID.String(), Name: r.Name, CreatedAt: r.CreatedAt}
}

func (h *Hub) Workspaces(ctx context.Context) ([]Workspace, error) {
	if err := h.admin(ctx); err != nil {
		return nil, err
	}
	rows, err := h.q().AllWorkspaces(ctx)
	out := []Workspace{}
	for _, r := range rows {
		out = append(out, workspaceOut(r))
	}
	return out, err
}

func (h *Hub) AddWorkspace(ctx context.Context, name, owner string) (*Workspace, error) {
	if err := h.admin(ctx); err != nil {
		return nil, err
	}
	r, err := h.createWorkspace(ctx, name, owner)
	if err != nil {
		return nil, err
	}
	w := workspaceOut(r)
	return &w, nil
}

func (h *Hub) DeleteWorkspace(ctx context.Context, id string) error {
	if err := h.admin(ctx); err != nil {
		return err
	}
	ws, err := uuid.Parse(id)
	if err == nil {
		_, err = h.q().WorkspaceByID(ctx, ws)
	}
	if err != nil {
		return fmt.Errorf("workspace %q: %w", id, ErrNotFound)
	}
	if id == h.one {
		return fmt.Errorf("the first workspace holds the instance admins and cannot be deleted: %w", ErrConflict)
	}
	h.forget(id)
	bg := context.WithoutCancel(ctx)
	roles, err := h.q().RolesWithPrefix(bg, id+"_")
	if err != nil {
		return err
	}
	if c, cerr := pgx.ConnectConfig(bg, h.direct); cerr == nil {
		c.Exec(bg, "DROP DATABASE IF EXISTS "+pgIdent(h.database(id))+" WITH (FORCE)")
		for _, role := range roles {
			c.Exec(bg, "DROP ROLE IF EXISTS "+pgIdent(role))
		}
		c.Close(bg)
	} else {
		return cerr
	}
	return h.q().DeleteWorkspace(bg, ws)
}

func workspaceName(name string) (string, error) {
	name = strings.Join(strings.Fields(name), " ")
	if name == "" || len(name) > workspaceMax {
		return "", &ValidationError{Type: "workspace", Errors: []FieldError{{Path: "name", Message: fmt.Sprintf("want a name of 1-%d characters", workspaceMax)}}}
	}
	return name, nil
}

func (h *Hub) createWorkspace(ctx context.Context, name, owner string) (generated.ArchiveWorkspace, error) {
	var row generated.ArchiveWorkspace
	id := coreNewID()
	name, err := workspaceName(name)
	if err != nil {
		return row, err
	}
	if owner, err = authEmail(owner); err != nil {
		return row, err
	}
	short := id.String()
	key := make([]byte, 32)
	rand.Read(key)
	q := h.q()
	row, err = q.CreateWorkspace(ctx, id, name, h.wrap(short, key))
	if errors.Is(err, pgx.ErrNoRows) {
		return row, fmt.Errorf("workspace %s already exists: %w", short, ErrConflict)
	}
	if err != nil {
		return row, err
	}
	ctx = context.WithoutCancel(ctx)
	fail := func(err error) (generated.ArchiveWorkspace, error) {
		h.forget(short)
		if c, cerr := pgx.ConnectConfig(ctx, h.direct); cerr == nil {
			c.Exec(ctx, "DROP DATABASE IF EXISTS "+pgIdent(h.database(short))+" WITH (FORCE)")
			c.Close(ctx)
		}
		q.DeleteWorkspace(ctx, row.ID)
		return row, fmt.Errorf("creating workspace %s: %w", name, err)
	}
	c, err := pgx.ConnectConfig(ctx, h.direct)
	if err != nil {
		return fail(err)
	}
	_, err = c.Exec(ctx, "CREATE DATABASE "+pgIdent(h.database(short)))
	if err == nil {
		_, err = c.Exec(ctx, "REVOKE CONNECT ON DATABASE "+pgIdent(h.database(short))+" FROM PUBLIC")
	}
	c.Close(ctx)
	if err != nil {
		return fail(err)
	}
	a, release, err := h.acquire(ctx, short)
	if err != nil {
		return fail(err)
	}
	_, err = a.AddUser(context.Background(), owner, "", true)
	release()
	if err != nil {
		return fail(err)
	}
	return row, nil
}

type WorkspaceInfo struct {
	Workspace
	AnthropicKey bool `json:"anthropic_key"`
}

func (a *Archive) Info(ctx context.Context) (*WorkspaceInfo, error) {
	if ActorFrom(ctx) == nil {
		return nil, fmt.Errorf("not signed in: %w", ErrUnauthorized)
	}
	row, err := a.hub.q().WorkspaceByID(ctx, a.ws.ID)
	if err != nil {
		return nil, err
	}
	return &WorkspaceInfo{Workspace: workspaceOut(row), AnthropicKey: len(row.AnthropicKey) > 0}, nil
}

func (a *Archive) SetInfo(ctx context.Context, name, anthropicKey *string) (*WorkspaceInfo, error) {
	if _, err := a.workspaceAdmin(ctx); err != nil {
		return nil, err
	}
	q := a.hub.q()
	if name != nil {
		n, err := workspaceName(*name)
		if err != nil {
			return nil, err
		}
		if err := q.RenameWorkspace(ctx, a.ws.ID, n); err != nil {
			return nil, err
		}
	}
	if anthropicKey != nil {
		var sealed []byte
		if k := strings.TrimSpace(*anthropicKey); k != "" {
			sealed = a.secretBox(a.ws.ID, "anthropic", []byte(k))
		}
		if err := q.SetWorkspaceAnthropicKey(ctx, a.ws.ID, sealed); err != nil {
			return nil, err
		}
	}
	return a.Info(ctx)
}

func (a *Archive) anthropicKey(ctx context.Context) string {
	if row, err := a.hub.q().WorkspaceByID(ctx, a.ws.ID); err == nil && len(row.AnthropicKey) > 0 {
		if b, err := a.secretOpen(a.ws.ID, "anthropic", row.AnthropicKey); err == nil {
			return string(b)
		}
	}
	return os.Getenv("ANTHROPIC_API_KEY")
}

func (a *Archive) slot(ctx context.Context) (func(), error) {
	select {
	case a.slots <- struct{}{}:
		return func() { <-a.slots }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(hubBusy):
		return nil, fmt.Errorf("this workspace already runs %d scripts at once; try again shortly: %w", cap(a.slots), ErrBusy)
	}
}

type prefixBlobs struct {
	b blobStore
	p string
}

func blobPrefixed(b blobStore, p string) blobStore {
	if p == "" {
		return b
	}
	return prefixBlobs{b, p}
}

func (x prefixBlobs) put(ctx context.Context, key, sum string, r io.Reader, size int64) error {
	return x.b.put(ctx, x.p+key, sum, r, size)
}

func (x prefixBlobs) open(ctx context.Context, key string, size int64) (io.ReadSeekCloser, error) {
	return x.b.open(ctx, x.p+key, size)
}

func (x prefixBlobs) remove(ctx context.Context, key string) error { return x.b.remove(ctx, x.p+key) }

func (h *Hub) workspaceRoutes(authed func(string, func(http.ResponseWriter, *http.Request) (int, any, error))) {
	ok := func(v any, err error) (int, any, error) { return http.StatusOK, v, err }
	authed("GET /auth/workspace", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		return ok(archiveFrom(r.Context()).Info(r.Context()))
	})
	authed("PATCH /auth/workspace", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		var body struct {
			Name         *string `json:"name"`
			AnthropicKey *string `json:"anthropic_key"`
		}
		if err := httpDecode(r, &body); err != nil {
			return 0, nil, err
		}
		return ok(archiveFrom(r.Context()).SetInfo(r.Context(), body.Name, body.AnthropicKey))
	})
	authed("GET /auth/workspaces", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		return ok(h.Workspaces(r.Context()))
	})
	authed("POST /auth/workspaces", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		var body struct {
			Name  string `json:"name"`
			Owner string `json:"owner"`
		}
		if err := httpDecode(r, &body); err != nil {
			return 0, nil, err
		}
		ws, err := h.AddWorkspace(r.Context(), body.Name, body.Owner)
		return http.StatusCreated, ws, err
	})
	authed("DELETE /auth/workspaces/{id}", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		return http.StatusNoContent, nil, h.DeleteWorkspace(r.Context(), r.PathValue("id"))
	})
}
