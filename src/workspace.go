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

	"github.com/Mulholland-Inc/archive/connect"
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
	migrating bool
	key       []byte
	aead      cipher.AEAD
	pg        *pgx.ConnConfig
	main      *pgxpool.Pool
	blobs     blobStore
	hints     Hints
	app       string
	api       string
	mcpURL    string
	tasks     *hubTasks
	live      *liveHub
	connect   connect.Providers

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
	queue, account string
	base           string
	meta           *gcpToken

	mu     sync.Mutex
	verify *oidc.IDTokenVerifier
}

type hubTask struct {
	Name         string    `json:"name"`
	ScheduleTime time.Time `json:"scheduleTime"`
	HTTPRequest  struct {
		HTTPMethod string            `json:"httpMethod"`
		URL        string            `json:"url"`
		Headers    map[string]string `json:"headers"`
		Body       []byte            `json:"body"`
		OIDCToken  struct {
			ServiceAccountEmail string `json:"serviceAccountEmail"`
			Audience            string `json:"audience"`
		} `json:"oidcToken"`
	} `json:"httpRequest"`
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
	Type      string    `json:"type"`
	Icon      *string   `json:"icon"`
	Color     *int16    `json:"color"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at,omitzero"`
	Unread    bool      `json:"unread,omitempty"`
}

type WorkspaceLook struct {
	Type  *string `json:"type"`
	Icon  *string `json:"icon"`
	Color *int16  `json:"color"`
}

const (
	hubIdle       = 10 * time.Minute
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
	return openHub(ctx, os.Getenv("DATABASE_URL"), "./data")
}

func openHub(ctx context.Context, dsn, dir string) (*Hub, error) {
	if dsn == "" {
		return nil, errors.New("DATABASE_URL is required: a Postgres 18 role with CREATEROLE and CREATEDB, like the one docker compose up -d starts")
	}
	tasks, err := hubTasksOpen()
	if err != nil {
		return nil, err
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
	pg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("DATABASE_URL: %w", err)
	}
	conns, err := connect.Load(os.Getenv)
	if err != nil {
		return nil, err
	}
	if err := syncCheck(); err != nil {
		return nil, err
	}
	h := &Hub{key: key, aead: aead, pg: pg, blobs: blobs, hints: coreHints("en-US"), tasks: tasks, connect: conns, open: map[string]*hubEntry{}}
	if err := h.urls("8080"); err != nil {
		return nil, err
	}
	h.live = newLive(h.api)
	if h.main, err = storePool(ctx, pg, storeSysConns, nil, nil); err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	return h, nil
}

func (h *Hub) urls(port string) error {
	h.app = strings.TrimRight(coreEnv("ARCHIVE_URL", "http://localhost:5173"), "/")
	u, err := url.Parse(h.app)
	switch {
	case err != nil || u.Host == "":
		return fmt.Errorf("ARCHIVE_URL %q is not a URL", h.app)
	case h.localApp():
		h.api = "http://localhost:" + port
		h.mcpURL = h.api
	case strings.HasPrefix(u.Host, "app."):
		h.api = u.Scheme + "://api." + strings.TrimPrefix(u.Host, "app.")
		h.mcpURL = u.Scheme + "://mcp." + strings.TrimPrefix(u.Host, "app.")
	default:
		return fmt.Errorf("ARCHIVE_URL %q: want https://app.<domain>, whose API is https://api.<domain>, or http://localhost:<port> in development", h.app)
	}
	return nil
}

const mainLock = 7274

func (h *Hub) migrateAll(ctx context.Context) error {
	h.migrating = true
	if err := h.migrate(ctx); err != nil {
		return fmt.Errorf("main database: %w", err)
	}
	rows, err := h.q().AllWorkspaces(ctx)
	if err != nil {
		return err
	}
	var errs []error
	for _, row := range rows {
		a, err := h.load(ctx, row.ID.String())
		if err != nil {
			errs = append(errs, err)
			continue
		}
		a.Close()
	}
	return errors.Join(errs...)
}

func (h *Hub) migrate(ctx context.Context) error {
	conn, err := h.main.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	q := generated.New(conn)
	if err := q.AdvisoryLock(ctx, mainLock); err != nil {
		return err
	}
	defer q.AdvisoryUnlock(context.WithoutCancel(ctx), mainLock)
	at, err := q.MainRevision(ctx)
	if err != nil && !storeUndefined(err) {
		return err
	}
	if at != mainHash() {
		want, err := pgScratch(ctx, h.main, h.pg, mainSchema)
		if err != nil {
			return fmt.Errorf("main.sql: %w", err)
		}
		if err := pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error { return mainMigrate(ctx, tx, want) }); err != nil {
			return err
		}
	}
	fresh, err := q.NoUsers(ctx)
	if err != nil || !fresh {
		return err
	}
	_, err = h.createWorkspace(ctx, "Archive", "", WorkspaceLook{})
	return err
}

func mainMigrate(ctx context.Context, tx pgx.Tx, want pgSchema) error {
	lost, err := pgMigrate(ctx, tx, want)
	if err != nil {
		return fmt.Errorf("main.sql: %w", err)
	}
	if len(lost) > 0 {
		return fmt.Errorf("main.sql not applied, it would delete data:\n  %s", strings.Join(lost, "\n  "))
	}
	return generated.New(tx).InsertMainRevision(ctx, mainHash())
}

func mainHash() string {
	h := sha256.Sum256([]byte(mainSchema))
	return hex.EncodeToString(h[:])
}

func storeUndefined(err error) bool {
	var pe *pgconn.PgError
	return errors.As(err, &pe) && (pe.Code == "42P01" || pe.Code == "3F000")
}

func (h *Hub) Close() {
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
	h.live.settle(liveSettle)
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

func (h *Hub) database(id string) string { return "archive_" + id }

func (h *Hub) openArchive(ctx context.Context, row generated.ArchiveWorkspace, key []byte) (*Archive, error) {
	id := row.ID.String()
	cfg := h.pg.Copy()
	cfg.Database = h.database(id)
	st, err := OpenStore(ctx, cfg, id, authPassword(key), h.migrating, func(table, rid string) {
		if table == "archive.push" {
			h.pushed(row.ID, rid)
			return
		}
		h.live.moved(id, liveChange{Table: table, ID: rid})
	})
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
	if e.users--; e.users > 0 || e.a == nil {
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
	err = errors.Join(err, a.syncMembers(ctx, false))
	for _, kind := range []string{"workflow", "file", "sync", "pull", "blob"} {
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
	return a.hub.soon(ctx, a.ws.ID, kind, at)
}

func (h *Hub) soon(ctx context.Context, ws uuid.UUID, kind string, at time.Time) error {
	ctx = context.WithoutCancel(ctx)
	r, err := h.q().SoonDue(ctx, ws, kind, at)
	switch {
	case err != nil:
		return err
	case r.Changed:
		return h.wake(ctx, ws, kind, r.At)
	case time.Since(r.At) > workflowRetry:
		return h.wake(ctx, ws, kind, time.Now())
	}
	return nil
}

func (h *Hub) pushed(ws uuid.UUID, kind string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := h.soon(ctx, ws, kind, time.Now()); err != nil {
		fmt.Fprintf(os.Stderr, "archive: workspace %s: queueing %s: %v\n", ws, kind, err)
	}
}

func hubTasksOpen() (*hubTasks, error) {
	if os.Getenv("ARCHIVE_TASKS_URL") == "" && os.Getenv("K_SERVICE") == "" && os.Getenv("CLOUD_RUN_JOB") == "" {
		return nil, nil
	}
	raw := coreEnv("ARCHIVE_TASKS_URL", "https://cloudtasks.googleapis.com")
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.Scheme != "https" && u.Scheme != "http" {
		return nil, fmt.Errorf("ARCHIVE_TASKS_URL %q: want https://host or, for an emulator, http://host:port/projects/<p>/locations/<r>/queues/<q>", raw)
	}
	meta, queue := gcpMetadata(), strings.Trim(u.Path, "/")
	if queue == "" {
		if queue, err = meta.queue(context.Background()); err != nil {
			return nil, err
		}
	}
	p := strings.Split(queue, "/")
	if len(p) != 6 || p[0] != "projects" || p[2] != "locations" || p[4] != "queues" {
		return nil, fmt.Errorf("cloud tasks queue %q: want projects/<p>/locations/<r>/queues/<q>", queue)
	}
	t := &hubTasks{queue: queue, account: p[5] + "-tasks@" + p[1] + ".iam.gserviceaccount.com"}
	t.base = u.Scheme + "://" + u.Host
	if u.Scheme == "https" {
		t.meta = meta
	}
	return t, nil
}

func (h *Hub) wake(ctx context.Context, ws uuid.UUID, kind string, at time.Time) error {
	return h.queue(ctx, ws, kind, at, time.Now().Unix())
}

func (h *Hub) wakeClaimed(ctx context.Context, ws uuid.UUID, kind string, at time.Time) error {
	return h.queue(ctx, ws, kind, at, 0)
}

func (h *Hub) queue(ctx context.Context, ws uuid.UUID, kind string, at time.Time, asked int64) error {
	t := h.tasks
	if limit := time.Now().Add(hubTaskMax); at.After(limit) {
		at = limit
	}
	if t == nil {
		time.AfterFunc(time.Until(at), func() {
			if err := h.work(context.Background(), ws, kind); err != nil {
				fmt.Fprintf(os.Stderr, "archive: workspace %s %ss: %v\n", ws, kind, err)
			}
		})
		return nil
	}
	url := h.api + "/tasks"
	body, _ := json.Marshal(map[string]string{"workspace": ws.String(), "work": kind + "s"})
	sum := sha256.Sum256(fmt.Appendf(nil, "%s/%s/%d/%d", ws, kind, at.UnixMilli(), asked))
	var tk hubTask
	tk.Name, tk.ScheduleTime = t.queue+"/tasks/"+hex.EncodeToString(sum[:]), at.UTC()
	r := &tk.HTTPRequest
	r.HTTPMethod, r.URL, r.Headers, r.Body = "POST", url, map[string]string{"Content-Type": "application/json"}, body
	r.OIDCToken.ServiceAccountEmail, r.OIDCToken.Audience = t.account, url
	b, _ := json.Marshal(map[string]any{"task": tk})
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "POST", t.base+"/v2/"+t.queue+"/tasks", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if t.meta != nil {
		tok, err := t.meta.access(ctx)
		if err != nil {
			return fmt.Errorf("cloud tasks: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := hubHTTP.Do(req)
	if err != nil {
		return fmt.Errorf("cloud tasks: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 == 2 || resp.StatusCode == http.StatusConflict {
		return nil
	}
	var e struct {
		Error struct{ Message string } `json:"error"`
	}
	json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&e)
	return fmt.Errorf("cloud tasks: %s", cmp.Or(e.Error.Message, resp.Status))
}

func (t *gcpToken) queue(ctx context.Context) (string, error) {
	name := cmp.Or(os.Getenv("K_SERVICE"), os.Getenv("CLOUD_RUN_JOB"))
	project, err := t.get(ctx, "project/project-id")
	if err != nil {
		return "", err
	}
	region, err := t.get(ctx, "instance/region")
	if err != nil {
		return "", err
	}
	return "projects/" + project + "/locations/" + region[strings.LastIndex(region, "/")+1:] + "/queues/" + name, nil
}

func (t *gcpToken) get(ctx context.Context, path string) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", t.meta+"/computeMetadata/v1/"+path, nil)
	req.Header.Set("Metadata-Flavor", "Google")
	resp, err := hubHTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("metadata server: %w", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if v := strings.TrimSpace(string(b)); err == nil && resp.StatusCode == http.StatusOK && v != "" {
		return v, nil
	}
	return "", fmt.Errorf("metadata server: no %s (%s)", path, resp.Status)
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

func (h *Hub) taskCaller(r *http.Request) error { return h.googleCaller(r, "/tasks") }

func (h *Hub) googleCaller(r *http.Request, path string) error {
	t := h.tasks
	tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok {
		return fmt.Errorf("want the OIDC token Google sends: %w", ErrUnauthorized)
	}
	t.mu.Lock()
	if t.verify == nil {
		p, err := oidc.NewProvider(context.Background(), "https://accounts.google.com")
		if err != nil {
			t.mu.Unlock()
			return fmt.Errorf("google's signing keys: %w", err)
		}
		t.verify = p.Verifier(&oidc.Config{SkipClientIDCheck: true})
	}
	v := t.verify
	t.mu.Unlock()
	id, err := v.Verify(r.Context(), strings.TrimSpace(tok))
	if err != nil {
		return fmt.Errorf("%v: %w", err, ErrUnauthorized)
	}
	if !slices.Contains(id.Audience, h.api+path) {
		return fmt.Errorf("a token for %v, not %s: %w", id.Audience, h.api+path, ErrUnauthorized)
	}
	var c struct {
		Email    string `json:"email"`
		Verified bool   `json:"email_verified"`
	}
	if err := id.Claims(&c); err != nil || !c.Verified || c.Email != t.account {
		return fmt.Errorf("only %s calls %s: %w", t.account, path, ErrForbidden)
	}
	return nil
}

func (h *Hub) tasksHTTP(w http.ResponseWriter, r *http.Request) {
	if h.tasks == nil {
		http.NotFound(w, r)
		return
	}
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
	if !ok || kind != "workflow" && kind != "file" && kind != "sync" && kind != "pull" && kind != "blob" {
		httpWrite(w, 0, nil, fmt.Errorf("work: want workflows, files, syncs, pulls or blobs: %w", httpErrBadRequest))
		return
	}
	err := h.work(r.Context(), body.Workspace, kind)
	if err != nil {
		fmt.Fprintf(os.Stderr, "archive: workspace %s %s: %v\n", body.Workspace, body.Work, err)
	}
	httpWrite(w, http.StatusNoContent, nil, err)
}

func (h *Hub) work(ctx context.Context, ws uuid.UUID, kind string) error {
	q := h.q()
	lease := runTimeout + workflowRetry
	switch {
	case kind == "workflow":
		lease += browseTimeout
	case kind == "sync":
		lease = syncBudget + syncRetry
	case kind == "pull":
		lease = pullBudget + pullTime + time.Minute
	}
	at, err := q.ClaimDueFor(ctx, time.Now().Add(lease), ws, kind)
	if err == nil {
		return h.due(ctx, ws, kind, at)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if at, err = q.DueAt(ctx, ws, kind); err == nil && at.After(time.Now()) {
		return h.wakeClaimed(ctx, ws, kind, at)
	} else if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	return err
}

func (h *Hub) wakeAll(ctx context.Context) error {
	if h.tasks != nil {
		return nil
	}
	due, err := h.q().AllDue(ctx)
	if err != nil {
		return err
	}
	for _, d := range due {
		if err := h.wake(ctx, d.Workspace, d.Kind, d.At); err != nil {
			return err
		}
	}
	return nil
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
		var n *time.Time
		var err error
		var t time.Time
		switch kind {
		case "sync":
			t, err = q.NextSyncAt(ctx, time.Now().Add(syncLeaseCheck))
			n = &t
		case "pull":
			t, err = q.NextPull(ctx)
			n = &t
		case "blob":
			t, err = q.NextBlob(ctx)
			n = &t
		default:
			n, err = q.NextWorkflowAt(ctx)
		}
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
	start := time.Now()
	for range hubBatch {
		var ok bool
		switch kind {
		case "workflow":
			ok, err = a.workflowNext(ctx)
		case "file":
			ok, err = a.fileProcess(ctx, uuid.Nil)
		case "sync":
			ok, err = a.syncNext(ctx)
			ok = ok && time.Since(start) < syncBudget
		case "pull":
			a.pullAll(ctx)
		case "blob":
			ok, err = a.blobNext(ctx)
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

func (h *Hub) authenticate(ctx context.Context, secret, want string, kinds ...string) (*Archive, *Actor, func(), error) {
	if len(kinds) == 0 {
		kinds = []string{"api", "session"}
	}
	t, err := h.q().Authenticate(ctx, authHash(secret), kinds)
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
	u, err := h.q().FirstAdmin(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, nil, fmt.Errorf("no users yet: %w", ErrUnauthorized)
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
	if short == "" && t.Kind == "api" && len(t.Workspaces) > 1 {
		return nil, nil, nil, h.pickWorkspace(ctx, t.UserID)
	}
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
	m, err := generated.New(a.st.sys).MemberEntry(ctx, t.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		err = fmt.Errorf("you are not in this workspace: %w", ErrForbidden)
	}
	if err == nil && m.Revision != a.Catalog().stamp {
		err = a.reload(ctx)
	}
	if err != nil {
		release()
		return nil, nil, nil, err
	}
	role := a.roleID("u", m.ID)
	ac := &Actor{UserID: t.UserID, Email: t.Email, Name: t.UserName, Role: role, UserRole: role, member: m.ID}
	ac.scoped = t.Workspace != ""
	if t.Kind != "session" {
		ac.TokenName = t.Name
	}
	if t.Kind == "api" || t.Kind == "mcp" {
		ac.Role, ac.TokenID = cmp.Or(t.Role, ac.Role), t.ID
	}
	return a, ac, release, nil
}

func (h *Hub) pickWorkspace(ctx context.Context, user uuid.UUID) error {
	rows, err := h.q().UserWorkspaces(ctx, user)
	if err != nil {
		return err
	}
	names := make([]string, len(rows))
	for i, w := range rows {
		names[i] = fmt.Sprintf("%s (%s)", w.Name, w.ID)
	}
	return fmt.Errorf("this key works in several workspaces, so send Archive-Workspace with the id of one: %s: %w", strings.Join(names, ", "), httpErrBadRequest)
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
	scheme, tok, _ := strings.Cut(r.Header.Get("Authorization"), " ")
	err := fmt.Errorf("send Authorization: Bearer <token>, from OAuth or an API token: %w", ErrUnauthorized)
	var a *Archive
	var ac *Actor
	var release func()
	if strings.EqualFold(scheme, "Bearer") && strings.TrimSpace(tok) != "" {
		a, ac, release, err = h.authenticate(r.Context(), strings.TrimSpace(tok), r.Header.Get("Archive-Workspace"), "api", "mcp")
	}
	if err != nil {
		h.mcpRefuse(w, r, err)
		return
	}
	defer release()
	a.hmu.Lock()
	if a.mcph == nil {
		a.mcph = auth.RequireBearerToken(a.mcpVerify, &auth.RequireBearerTokenOptions{AllowMissingExpiration: true})(
			mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
				ac := ActorFrom(r.Context())
				a.hmu.Lock()
				defer a.hmu.Unlock()
				if a.mcps[ac.member] == nil {
					if a.mcps == nil {
						a.mcps = map[uuid.UUID]*mcp.Server{}
					}
					a.mcps[ac.member] = mcpServer(a, ac)
				}
				return a.mcps[ac.member]
			},
				&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, PropagateRequestCancellation: true}))
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

func (a *Archive) workspaceAdmin(ctx context.Context) (*Actor, error) {
	ac, err := authUser(ctx)
	if err != nil {
		return nil, err
	}
	if !a.has(ctx, ac.UserRole, a.prefix+"_engineer") {
		return nil, fmt.Errorf("only engineers and the owner change this workspace: %w", ErrForbidden)
	}
	return ac, nil
}

func (h *Hub) admin(ctx context.Context) error {
	ac, err := authUser(ctx)
	if err != nil {
		return err
	}
	u, err := h.q().UserByID(ctx, ac.UserID)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && !u.Admin {
		return fmt.Errorf("only admins of this Archive do this: %w", ErrForbidden)
	}
	return err
}

func workspaceOut(r generated.ArchiveWorkspace) Workspace {
	return Workspace{ID: r.ID.String(), Name: r.Name, Type: string(r.Type), Icon: r.Icon, Color: r.Color, Slug: r.Slug, CreatedAt: r.CreatedAt}
}

func (h *Hub) AddWorkspace(ctx context.Context, name, owner string, look WorkspaceLook) (*Workspace, error) {
	if err := h.admin(ctx); err != nil {
		return nil, err
	}
	if _, err := authEmail(owner); err != nil {
		return nil, err
	}
	r, err := h.createWorkspace(ctx, name, owner, look)
	if err != nil {
		return nil, err
	}
	w := workspaceOut(r)
	return &w, nil
}

func (h *Hub) DeleteWorkspace(ctx context.Context, id string) error {
	ac, err := authUser(ctx)
	if err != nil {
		return err
	}
	ws, err := uuid.Parse(id)
	if err == nil {
		_, err = h.q().WorkspaceByID(ctx, ws)
	}
	if err != nil {
		return fmt.Errorf("workspace %q: %w", id, ErrNotFound)
	}
	if ac.TokenID != uuid.Nil && archiveFrom(ctx).ws.ID != ws {
		return fmt.Errorf("a token only deletes its own workspace: %w", ErrForbidden)
	}
	a, release, err := h.acquire(ctx, id)
	if err != nil {
		return err
	}
	m, err := a.memberOf(ctx, ac.Email)
	release()
	if err != nil {
		return err
	}
	if m.Role != "owner" {
		return fmt.Errorf("only this workspace's owner deletes it: %w", ErrForbidden)
	}
	tell := h.liveMembers(ctx, ws)
	h.forget(id)
	bg := context.WithoutCancel(ctx)
	roles, err := h.q().RolesWithPrefix(bg, id+"_")
	if err != nil {
		return err
	}
	if c, cerr := pgx.ConnectConfig(bg, h.pg); cerr == nil {
		c.Exec(bg, "DROP DATABASE IF EXISTS "+pgIdent(h.database(id))+" WITH (FORCE)")
		for _, role := range roles {
			c.Exec(bg, "DROP ROLE IF EXISTS "+pgIdent(role))
		}
		c.Close(bg)
	} else {
		return cerr
	}
	if err := h.q().DeleteWorkspace(bg, ws); err != nil {
		return err
	}
	tell()
	h.live.moved(id, liveChange{Table: "archive.workspace", ID: id})
	return nil
}

func workspaceName(name string) (string, error) {
	name = strings.Join(strings.Fields(name), " ")
	if name == "" || len(name) > workspaceMax {
		return "", &ValidationError{Type: "workspace", Errors: []FieldError{{Path: "name", Message: fmt.Sprintf("want a name of 1-%d characters", workspaceMax)}}}
	}
	return name, nil
}

var workspaceIcons = []string{"aframe", "apartment", "beachhouse", "bungalow", "tent", "caravan", "cottage", "shed", "houseboat", "hut", "igloo", "logcabin", "stilts", "tipi", "treehouse", "castle", "duplex", "farmhouse", "lighthouse", "manor", "pagoda", "palace", "rowhouse", "townhouse", "windmill", "church", "daycare", "school", "farm", "greenhouse", "accounting", "architect", "bank", "datacentre", "insurance", "lawfirm", "marketing", "office", "realestate", "recruiter", "server", "skyscraper", "software", "exchange", "dental", "hospital", "clinic", "pharmacy", "vet", "bakery", "bar", "brewery", "butcher", "cafe", "diner", "fishmonger", "foodtruck", "grocery", "market", "pizzeria", "restaurant", "winery", "bookshop", "boutique", "florist", "shop", "hardware", "jeweller", "barbershop", "gym", "salon", "spa", "yoga", "gallery", "cinema", "hotel", "library", "museum", "music", "photo", "stadium", "theatre", "capitol", "courthouse", "nonprofit", "townhall", "university", "garage", "dealership", "carwash", "petrol", "construction", "factory", "silo", "depot", "mine", "warehouse", "workshop", "hangar", "airtower", "bridge", "port", "station", "oilwell", "power", "solar", "telecom", "watertower", "windfarm"}

func (l WorkspaceLook) typ() (*generated.ArchiveWorkspaceType, error) {
	ve := &ValidationError{Type: "workspace"}
	var out *generated.ArchiveWorkspaceType
	if l.Type != nil {
		switch t := generated.ArchiveWorkspaceType(*l.Type); t {
		case generated.ArchiveWorkspaceTypePersonal, generated.ArchiveWorkspaceTypeFamily, generated.ArchiveWorkspaceTypeBusiness:
			out = &t
		default:
			ve.Errors = append(ve.Errors, FieldError{Path: "type", Message: "want personal, family or business"})
		}
	}
	if l.Icon != nil && !slices.Contains(workspaceIcons, *l.Icon) {
		ve.Errors = append(ve.Errors, FieldError{Path: "icon", Message: "want one of " + strings.Join(workspaceIcons, ", ")})
	}
	if l.Color != nil && (*l.Color < 0 || *l.Color > 31) {
		ve.Errors = append(ve.Errors, FieldError{Path: "color", Message: "want a tint from 0 to 31"})
	}
	return out, ve.err()
}

func (h *Hub) createWorkspace(ctx context.Context, name, owner string, look WorkspaceLook) (generated.ArchiveWorkspace, error) {
	var row generated.ArchiveWorkspace
	id := coreNewID()
	name, err := workspaceName(name)
	if err != nil {
		return row, err
	}
	typ, err := look.typ()
	if err != nil {
		return row, err
	}
	if typ == nil {
		typ = new(generated.ArchiveWorkspaceTypePersonal)
	}
	if owner != "" {
		if owner, err = authEmail(owner); err != nil {
			return row, err
		}
	}
	short := id.String()
	key := make([]byte, 32)
	rand.Read(key)
	q := h.q()
	row, err = q.CreateWorkspace(ctx, generated.CreateWorkspaceParams{ID: id, Name: name, Key: h.wrap(short, key), Type: *typ, Icon: look.Icon, Color: look.Color})
	if errors.Is(err, pgx.ErrNoRows) {
		return row, fmt.Errorf("workspace %s already exists: %w", short, ErrConflict)
	}
	if err != nil {
		return row, storeErr(err)
	}
	ctx = context.WithoutCancel(ctx)
	fail := func(err error) (generated.ArchiveWorkspace, error) {
		h.forget(short)
		if c, cerr := pgx.ConnectConfig(ctx, h.pg); cerr == nil {
			c.Exec(ctx, "DROP DATABASE IF EXISTS "+pgIdent(h.database(short))+" WITH (FORCE)")
			c.Close(ctx)
		}
		q.DeleteWorkspace(ctx, row.ID)
		return row, fmt.Errorf("creating workspace %s: %w", name, err)
	}
	c, err := pgx.ConnectConfig(ctx, h.pg)
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
	if owner != "" {
		_, err = a.AddUser(context.Background(), owner, "", "owner")
	}
	release()
	if err != nil {
		return fail(err)
	}
	return row, nil
}

func (a *Archive) Info(ctx context.Context) (*Workspace, error) {
	if _, err := signedIn(ctx); err != nil {
		return nil, err
	}
	row, err := a.hub.q().WorkspaceByID(ctx, a.ws.ID)
	if err != nil {
		return nil, err
	}
	w := workspaceOut(row)
	return &w, nil
}

func (a *Archive) SetInfo(ctx context.Context, name *string, look WorkspaceLook) (*Workspace, error) {
	if _, err := a.workspaceAdmin(ctx); err != nil {
		return nil, err
	}
	if name != nil || look != (WorkspaceLook{}) {
		if name != nil {
			n, err := workspaceName(*name)
			if err != nil {
				return nil, err
			}
			name = &n
		}
		typ, err := look.typ()
		if err != nil {
			return nil, err
		}
		if err := a.hub.q().UpdateWorkspace(ctx, generated.UpdateWorkspaceParams{ID: a.ws.ID, Name: name, Type: typ, Icon: look.Icon, Color: look.Color}); err != nil {
			return nil, storeErr(err)
		}
		a.moved(ctx, "archive.workspace", a.ws.ID.String())
		a.hub.liveMembers(ctx, a.ws.ID)()
	}
	return a.Info(ctx)
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

func (x prefixBlobs) put(ctx context.Context, key string, r io.Reader, size int64) error {
	return x.b.put(ctx, x.p+key, r, size)
}

func (x prefixBlobs) open(ctx context.Context, key string, size int64) (io.ReadSeekCloser, error) {
	return x.b.open(ctx, x.p+key, size)
}

func (x prefixBlobs) move(ctx context.Context, from, to string) error {
	return x.b.move(ctx, x.p+from, x.p+to)
}

func (x prefixBlobs) remove(ctx context.Context, key string) error { return x.b.remove(ctx, x.p+key) }

func (h *Hub) workspaceRoutes(authed func(string, func(http.ResponseWriter, *http.Request) (int, any, error))) {
	ok := func(v any, err error) (int, any, error) { return http.StatusOK, v, err }
	authed("PATCH /auth/workspace", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		var body struct {
			Name *string `json:"name"`
			WorkspaceLook
		}
		if err := httpDecode(r, &body); err != nil {
			return 0, nil, err
		}
		return ok(archiveFrom(r.Context()).SetInfo(r.Context(), body.Name, body.WorkspaceLook))
	})
	authed("POST /auth/workspaces", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		var body struct {
			Name  string `json:"name"`
			Owner string `json:"owner"`
			WorkspaceLook
		}
		if err := httpDecode(r, &body); err != nil {
			return 0, nil, err
		}
		ws, err := h.AddWorkspace(r.Context(), body.Name, body.Owner, body.WorkspaceLook)
		return http.StatusCreated, ws, err
	})
	authed("DELETE /auth/workspaces/{id}", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		return http.StatusNoContent, nil, h.DeleteWorkspace(r.Context(), r.PathValue("id"))
	})
}

func (a *Archive) groupID(ctx context.Context, s string, make bool) (*uuid.UUID, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	if id, err := uuid.Parse(s); err == nil {
		return &id, nil
	}
	id, err := a.st.db(ctx).GroupByName(ctx, s)
	if errors.Is(err, pgx.ErrNoRows) && make {
		err = a.st.Q(ctx).QueryRow(ctx, `INSERT INTO archive."group" (name) VALUES ($1) RETURNING id`, s).Scan(&id)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, &ValidationError{Type: "group", Errors: []FieldError{{Path: "access", Message: fmt.Sprintf(`no group %q; make it with archive.create("group", { name: %q })`, s, s)}}}
	}
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func (a *Archive) groupNames(ctx context.Context, tb *Table, in map[string]any) error {
	if s, ok := in["access"].(string); ok && tb.col("access") != nil {
		id, err := a.groupID(ctx, s, false)
		if ve := (*ValidationError)(nil); errors.As(err, &ve) {
			ve.Type = tb.Name
		}
		if err != nil {
			return err
		}
		if id == nil {
			in["access"] = nil
		} else {
			in["access"] = id.String()
		}
	}
	if tb.Schema != "archive" || tb.Name != "member" {
		return nil
	}
	s, ok := in["group"].(string)
	if !ok {
		return nil
	}
	id, err := a.groupID(ctx, s, true)
	if err != nil {
		var ve *ValidationError
		if errors.As(err, &ve) {
			ve.Type, ve.Errors[0].Path = "member", "group"
		}
		return err
	}
	in["group"] = nil
	if id != nil {
		in["group"] = id.String()
	}
	return nil
}
