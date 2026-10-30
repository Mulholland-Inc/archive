package main

import (
	"cmp"
	"context"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Mulholland-Inc/archive/generated"
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
	stop   func() error
	one    string

	mu   sync.Mutex
	open map[string]*hubEntry
}

type hubEntry struct {
	ready chan struct{}
	a     *Archive
	err   error
	users int
	used  time.Time
}

type Workspace struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Plan      string    `json:"plan,omitempty"`
	CreatedAt time.Time `json:"created_at,omitzero"`
}

const (
	hubIdle       = 10 * time.Minute
	hubSweep      = time.Minute
	hubWorkers    = 4
	hubTick       = time.Second
	hubBatch      = 16
	hubFileDelay  = 10 * time.Second
	hubBusy       = 10 * time.Second
	workspaceRuns = 8
	workspaceMax  = 64
	signupTTL     = 15 * time.Minute
)

var workspaceShortRe = regexp.MustCompile(`^[a-z0-9]{1,16}$`)

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
	prefix := coreEnv("ARCHIVE_INSTANCE", "a")
	if !corePrefixRe.MatchString(prefix) {
		return nil, fmt.Errorf("ARCHIVE_INSTANCE %q: want 1-16 lowercase letters and digits, starting with a letter", prefix)
	}
	dsn, stop := os.Getenv("DATABASE_URL"), func() error { return nil }
	if dsn != "" && strings.TrimSpace(os.Getenv("ARCHIVE_SECRET_KEY")) == "" {
		return nil, errors.New("DATABASE_URL needs ARCHIVE_SECRET_KEY: every instance on that database must share one 32-byte key, like the output of openssl rand -hex 32")
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
	if dsn == "" {
		if dsn = pgLive(dir); dsn == "" {
			if err := BackupBeforeUpgrade(ctx, dir, cliVersion); err != nil {
				return nil, fmt.Errorf("backup: %w", err)
			}
			if dsn, stop, err = StartPostgres(ctx, dir); err != nil {
				return nil, fmt.Errorf("start postgres: %w", err)
			}
		}
	}
	pooled, err := pgx.ParseConfig(dsn)
	if err != nil {
		stop()
		return nil, fmt.Errorf("DATABASE_URL: %w", err)
	}
	direct := pooled
	if d := os.Getenv("DATABASE_DIRECT_URL"); d != "" {
		if direct, err = pgx.ParseConfig(d); err != nil {
			stop()
			return nil, fmt.Errorf("DATABASE_DIRECT_URL: %w", err)
		}
	}
	h := &Hub{key: key, aead: aead, pooled: pooled, direct: direct, blobs: blobs, hints: coreHints(os.Getenv("ARCHIVE_LOCALE")),
		stop: stop, open: map[string]*hubEntry{}}
	h.urls(coreEnv("ARCHIVE_URL", "http://localhost:8080"))
	if h.main, err = storePool(ctx, pooled, storeSysConns, nil); err != nil {
		stop()
		return nil, fmt.Errorf("open store: %w", err)
	}
	if err := h.register(ctx, prefix); err != nil {
		h.Close()
		return nil, fmt.Errorf("open store: %w", err)
	}
	return h, nil
}

func (h *Hub) urls(app string) {
	h.app = strings.TrimRight(app, "/")
	h.api = strings.TrimRight(cmp.Or(os.Getenv("ARCHIVE_API_URL"), h.app), "/")
}

func storeUndefined(err error) bool {
	var pe *pgconn.PgError
	return errors.As(err, &pe) && (pe.Code == "42P01" || pe.Code == "3F000")
}

func (h *Hub) register(ctx context.Context, prefix string) error {
	q := h.q()
	row, err := q.MainWorkspace(ctx)
	fresh := errors.Is(err, pgx.ErrNoRows) || storeUndefined(err)
	if err != nil && !fresh {
		return err
	}
	key := h.key
	if fresh {
		db, err := q.CurrentDatabase(ctx)
		if err != nil {
			return err
		}
		row = generated.ArchiveWorkspace{Short: workspaceShort(), Name: "Archive", Database: db, Prefix: prefix}
	} else if key, err = h.unwrap(row); err != nil {
		return err
	}
	a, err := h.openArchive(ctx, row, key)
	if err != nil {
		return err
	}
	if fresh {
		r, err := q.CreateWorkspace(ctx, generated.CreateWorkspaceParams{Short: row.Short, Name: row.Name, Database: row.Database, Prefix: row.Prefix, Key: h.wrap(row.Short, key)})
		if errors.Is(err, pgx.ErrNoRows) {
			r, err = q.MainWorkspace(ctx)
		}
		if err != nil {
			a.Close()
			return err
		}
		a.ws = r
	}
	h.one = a.ws.Short
	ready := make(chan struct{})
	close(ready)
	h.open[h.one] = &hubEntry{ready: ready, a: a, used: time.Now()}
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
	if h.stop != nil {
		return h.stop()
	}
	return nil
}

func (h *Hub) q() *generated.Queries { return generated.New(h.main) }

func workspaceShort() string {
	const abc = "abcdefghijkmnpqrstuvwxyz23456789"
	b := make([]byte, 8)
	rand.Read(b)
	for i := range b {
		b[i] = abc[int(b[i])%len(abc)]
	}
	return string(b)
}

func (h *Hub) wrap(short string, key []byte) []byte {
	nonce := make([]byte, h.aead.NonceSize())
	rand.Read(nonce)
	return h.aead.Seal(nonce, nonce, key, []byte("workspace "+short))
}

func (h *Hub) unwrap(row generated.ArchiveWorkspace) ([]byte, error) {
	n := h.aead.NonceSize()
	if len(row.Key) < n {
		return nil, fmt.Errorf("workspace %s: its data key is corrupt", row.Short)
	}
	k, err := h.aead.Open(nil, row.Key[:n], row.Key[n:], []byte("workspace "+row.Short))
	if err != nil {
		return nil, fmt.Errorf("workspace %s: its data key does not open with ARCHIVE_SECRET_KEY: %w", row.Short, err)
	}
	return k, nil
}

func (h *Hub) openArchive(ctx context.Context, row generated.ArchiveWorkspace, key []byte) (*Archive, error) {
	pooled, direct := h.pooled.Copy(), h.direct.Copy()
	pooled.Database, direct.Database = row.Database, row.Database
	st, err := OpenStore(ctx, pooled, direct, row.Prefix, authPassword(key))
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
	return &Archive{hub: h, ws: row, st: st, cat: cat, hints: h.hints, prefix: row.Prefix, aead: aead,
		blobs: blobPrefixed(h.blobs, row.Blobs), linkKey: secretDerive(key, "files"), slots: make(chan struct{}, h.concurrent(ctx, row))}, nil
}

func (h *Hub) acquire(ctx context.Context, short string) (*Archive, func(), error) {
	short = cmp.Or(short, h.one)
	if !workspaceShortRe.MatchString(short) {
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
		h.mu.Unlock()
		select {
		case <-e.ready:
		case <-ctx.Done():
			h.done(e)
			return nil, nil, ctx.Err()
		}
	}
	if e.err != nil {
		h.done(e)
		return nil, nil, e.err
	}
	var once sync.Once
	return e.a, func() { once.Do(func() { h.done(e) }) }, nil
}

func (h *Hub) done(e *hubEntry) {
	h.mu.Lock()
	e.users--
	e.used = time.Now()
	h.mu.Unlock()
}

func (h *Hub) load(ctx context.Context, short string) (*Archive, error) {
	row, err := h.q().WorkspaceByShort(ctx, short)
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

func (h *Hub) sweep() {
	h.mu.Lock()
	var idle []*Archive
	for k, e := range h.open {
		select {
		case <-e.ready:
		default:
			continue
		}
		if k != h.one && e.users == 0 && e.a != nil && time.Since(e.used) > hubIdle {
			idle = append(idle, e.a)
			delete(h.open, k)
		}
	}
	h.mu.Unlock()
	for _, a := range idle {
		a.Close()
	}
}

func (h *Hub) janitor(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	tick := time.NewTicker(hubSweep)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			h.sweep()
		}
	}
}

func (h *Hub) upgradeAll(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	rows, err := h.q().AllWorkspaces(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "archive: upgrading workspaces:", err)
		return
	}
	for _, row := range rows {
		if ctx.Err() != nil {
			return
		}
		h.mu.Lock()
		_, open := h.open[row.Short]
		h.mu.Unlock()
		if open {
			continue
		}
		key, err := h.unwrap(row)
		if err == nil {
			var a *Archive
			if a, err = h.openArchive(ctx, row, key); err == nil {
				a.afterOpen(ctx)
				a.Close()
			}
		}
		if err != nil && ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "archive: upgrading workspace %s: %v\n", row.Short, err)
		}
	}
}

func (a *Archive) afterOpen(ctx context.Context) {
	err := errors.Join(a.syncMembers(ctx), a.st.System(ctx, func(ctx context.Context) error {
		return a.st.db(ctx).ResetProcessing(ctx)
	}), a.storedBytes(ctx))
	for _, kind := range []string{"workflow", "file"} {
		next, nerr := a.nextDue(ctx, kind)
		if nerr == nil && next != nil {
			nerr = a.soon(ctx, kind, *next)
		}
		err = errors.Join(err, nerr)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "archive: workspace %s: %v\n", a.ws.Short, err)
	}
}

func (a *Archive) syncMembers(ctx context.Context) error {
	var emails []string
	if err := a.st.System(ctx, func(ctx context.Context) (err error) {
		emails, err = a.st.db(ctx).UserEmails(ctx)
		return err
	}); err != nil {
		return err
	}
	if emails == nil {
		emails = []string{}
	}
	q := a.hub.q()
	if err := q.KeepMembers(ctx, a.ws.ID, emails); err != nil {
		return err
	}
	return q.AddMembers(ctx, emails, a.ws.ID)
}

func (a *Archive) soon(ctx context.Context, kind string, at time.Time) error {
	return a.hub.q().SoonDue(context.WithoutCancel(ctx), a.ws.ID, kind, at)
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

func (h *Hub) work(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	tick := time.NewTicker(hubTick)
	defer tick.Stop()
	for {
		for {
			ok, err := h.next(ctx)
			if err != nil && ctx.Err() == nil {
				fmt.Fprintln(os.Stderr, "archive: background work:", err)
			}
			if !ok || err != nil {
				break
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

func (h *Hub) next(ctx context.Context) (bool, error) {
	d, err := h.q().ClaimDue(ctx, time.Now().Add(runTimeout()+workflowRetry))
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	bg := context.WithoutCancel(ctx)
	a, release, err := h.acquire(ctx, d.Short)
	if errors.Is(err, ErrNotFound) {
		return true, h.q().ClearDue(bg, d.Short, d.Kind, d.At)
	}
	if err != nil {
		return false, err
	}
	defer release()
	for range hubBatch {
		var ok bool
		switch d.Kind {
		case "workflow":
			ok, err = a.workflowNext(ctx)
		case "file":
			ok, err = a.fileProcess(ctx, uuid.Nil)
		}
		if err != nil && ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "archive: workspace %s %ss: %v\n", d.Short, d.Kind, err)
		}
		if !ok || err != nil || ctx.Err() != nil {
			break
		}
	}
	next, err := a.nextDue(bg, d.Kind)
	if err != nil {
		return true, err
	}
	if next == nil {
		return true, h.q().ClearDue(bg, d.Short, d.Kind, d.At)
	}
	return true, h.q().SetDue(bg, generated.SetDueParams{Next: *next, Short: d.Short, Kind: d.Kind, Claimed: d.At})
}

const workspaceSecretLen = 43

func secretWorkspace(s string) string {
	rest, ok := strings.CutPrefix(s, "arc_")
	if !ok || len(rest) == workspaceSecretLen {
		return ""
	}
	ws, _, ok := strings.Cut(rest, "_")
	if !ok || !workspaceShortRe.MatchString(ws) {
		return ""
	}
	return ws
}

func (h *Hub) of(secret string) string { return cmp.Or(secretWorkspace(secret), h.one) }

func (a *Archive) newSecret() string {
	b := make([]byte, 32)
	rand.Read(b)
	return "arc_" + a.ws.Short + "_" + base64.RawURLEncoding.EncodeToString(b)
}

func (h *Hub) sessions(r *http.Request) []string {
	c, err := r.Cookie(signinCookie)
	if err != nil {
		return nil
	}
	return strings.FieldsFunc(c.Value, func(r rune) bool { return r == '.' })
}

func (h *Hub) secret(r *http.Request) (string, error) {
	if hd := r.Header.Get("Authorization"); hd != "" {
		tok, ok := strings.CutPrefix(hd, "Bearer ")
		if !ok {
			return "", fmt.Errorf("want Authorization: Bearer <token>: %w", ErrUnauthorized)
		}
		return strings.TrimSpace(tok), nil
	}
	ss := h.sessions(r)
	if len(ss) == 0 {
		return "", fmt.Errorf("sign in or send a bearer token: %w", ErrUnauthorized)
	}
	want := r.Header.Get("Archive-Workspace")
	if want == "" {
		return ss[0], nil
	}
	for _, s := range ss {
		if h.of(s) == want {
			return s, nil
		}
	}
	return "", fmt.Errorf("not signed in to workspace %s: %w", want, ErrUnauthorized)
}

func (h *Hub) authenticate(ctx context.Context, secret string) (*Archive, *Actor, func(), error) {
	a, release, err := h.acquire(ctx, h.of(secret))
	if errors.Is(err, ErrNotFound) {
		err = fmt.Errorf("invalid or expired token: %w", ErrUnauthorized)
	}
	if err != nil {
		return nil, nil, nil, err
	}
	ac, err := a.Authenticate(ctx, secret)
	if err != nil {
		release()
		return nil, nil, nil, err
	}
	return a, ac, release, nil
}

func (h *Hub) httpAuth(next func(a *Archive) http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secret, err := h.secret(r)
		if err != nil {
			httpWrite(w, 0, nil, err)
			return
		}
		a, ac, release, err := h.authenticate(r.Context(), secret)
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
	a, release, err := h.acquire(r.Context(), h.of(strings.TrimSpace(tok)))
	if errors.Is(err, ErrNotFound) {
		w.Header().Set("WWW-Authenticate", "Bearer")
		http.Error(w, "invalid or expired token", http.StatusUnauthorized)
		return
	}
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
	hd.ServeHTTP(w, r)
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
	if a == nil || a.ws.Short != h.one || !a.isAdmin(ctx, ac) {
		return fmt.Errorf("only instance admins, the admins of the first workspace, do this: %w", ErrForbidden)
	}
	return nil
}

func (h *Hub) setting(ctx context.Context, key string, v any) (bool, error) {
	b, err := h.q().Setting(ctx, key)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal(b, v)
}

func (h *Hub) signupOpen(ctx context.Context) bool {
	var on bool
	h.setting(ctx, "signup", &on)
	return on
}

func (h *Hub) defaultPlan(ctx context.Context) *string {
	var p string
	if ok, err := h.setting(ctx, "plan", &p); !ok || err != nil || p == "" {
		return nil
	}
	return &p
}

type Settings struct {
	Signup bool   `json:"signup"`
	Plan   string `json:"plan"`
}

func (h *Hub) Settings(ctx context.Context) (*Settings, error) {
	if err := h.admin(ctx); err != nil {
		return nil, err
	}
	s := &Settings{Signup: h.signupOpen(ctx)}
	if p := h.defaultPlan(ctx); p != nil {
		s.Plan = *p
	}
	return s, nil
}

func (h *Hub) SetSettings(ctx context.Context, signup *bool, plan *string) (*Settings, error) {
	if err := h.admin(ctx); err != nil {
		return nil, err
	}
	if plan != nil && *plan != "" {
		if _, err := h.q().PlanByID(ctx, *plan); errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("plan %q: %w", *plan, ErrNotFound)
		} else if err != nil {
			return nil, err
		}
	}
	set := func(k string, v any) error {
		b, _ := json.Marshal(v)
		return h.q().SetSetting(ctx, k, b)
	}
	if signup != nil {
		if err := set("signup", *signup); err != nil {
			return nil, err
		}
	}
	if plan != nil {
		if err := set("plan", *plan); err != nil {
			return nil, err
		}
	}
	return h.Settings(ctx)
}

func workspaceOut(r generated.ArchiveWorkspace) Workspace {
	w := Workspace{ID: r.Short, Name: r.Name, CreatedAt: r.CreatedAt}
	if r.Plan != nil {
		w.Plan = *r.Plan
	}
	return w
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
	r, err := h.createWorkspace(ctx, "", name, owner)
	if err != nil {
		return nil, err
	}
	w := workspaceOut(r)
	return &w, nil
}

func workspaceName(name string) (string, error) {
	name = strings.Join(strings.Fields(name), " ")
	if name == "" || len(name) > workspaceMax {
		return "", &ValidationError{Type: "workspace", Errors: []FieldError{{Path: "name", Message: fmt.Sprintf("want a name of 1-%d characters", workspaceMax)}}}
	}
	return name, nil
}

func (h *Hub) createWorkspace(ctx context.Context, short, name, owner string) (generated.ArchiveWorkspace, error) {
	var row generated.ArchiveWorkspace
	name, err := workspaceName(name)
	if err != nil {
		return row, err
	}
	if owner, err = authEmail(owner); err != nil {
		return row, err
	}
	short = cmp.Or(short, workspaceShort())
	key := make([]byte, 32)
	rand.Read(key)
	q := h.q()
	row, err = q.CreateWorkspace(ctx, generated.CreateWorkspaceParams{Short: short, Name: name, Database: "archive_" + short, Prefix: "w" + short,
		Key: h.wrap(short, key), Blobs: short + "/", Plan: h.defaultPlan(ctx)})
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
			c.Exec(ctx, "DROP DATABASE IF EXISTS "+pgIdent(row.Database)+" WITH (FORCE)")
			c.Close(ctx)
		}
		q.DeleteWorkspace(ctx, row.ID)
		return row, fmt.Errorf("creating workspace %s: %w", name, err)
	}
	c, err := pgx.ConnectConfig(ctx, h.direct)
	if err != nil {
		return fail(err)
	}
	_, err = c.Exec(ctx, "CREATE DATABASE "+pgIdent(row.Database))
	if err == nil {
		_, err = c.Exec(ctx, "REVOKE CONNECT ON DATABASE "+pgIdent(row.Database)+" FROM PUBLIC")
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

type signupClaim struct {
	Email string `json:"e"`
	Name  string `json:"n"`
	Short string `json:"w"`
	Exp   int64  `json:"x"`
}

func (h *Hub) sign(purpose, payload string) string {
	m := hmac.New(sha256.New, secretDerive(h.key, purpose))
	m.Write([]byte(payload))
	return hex.EncodeToString(m.Sum(nil))
}

func (h *Hub) signupToken(email, name string) string {
	b, _ := json.Marshal(signupClaim{Email: email, Name: name, Short: workspaceShort(), Exp: time.Now().Add(signupTTL).Unix()})
	p := base64.RawURLEncoding.EncodeToString(b)
	return p + "." + h.sign("signup", p)
}

func (h *Hub) signupClaim(token string) (*signupClaim, error) {
	p, sig, _ := strings.Cut(token, ".")
	bad := fmt.Errorf("this link to create a workspace is invalid or expired: %w", ErrUnauthorized)
	if !hmac.Equal([]byte(sig), []byte(h.sign("signup", p))) {
		return nil, bad
	}
	b, err := base64.RawURLEncoding.DecodeString(p)
	var c signupClaim
	if err != nil || json.Unmarshal(b, &c) != nil || time.Now().Unix() > c.Exp {
		return nil, bad
	}
	return &c, nil
}

func (h *Hub) signup(ctx context.Context, c *signupClaim) (*Archive, func(), *User, error) {
	if !h.signupOpen(ctx) {
		return nil, nil, nil, fmt.Errorf("signup is closed on this Archive: %w", ErrForbidden)
	}
	if _, err := h.createWorkspace(ctx, c.Short, c.Name, c.Email); errors.Is(err, ErrConflict) {
		return nil, nil, nil, fmt.Errorf("this link was used already; sign in instead: %w", ErrConflict)
	} else if err != nil {
		return nil, nil, nil, err
	}
	a, release, err := h.acquire(ctx, c.Short)
	if err != nil {
		return nil, nil, nil, err
	}
	u, err := userBy(ctx, a, (*generated.Queries).UserByEmail, c.Email)
	if err != nil {
		release()
		return nil, nil, nil, err
	}
	return a, release, u, nil
}

func (h *Hub) names(ctx context.Context, shorts []string) []Workspace {
	rows, _ := h.q().WorkspaceNames(ctx, shorts)
	by := map[string]string{}
	for _, r := range rows {
		by[r.Short] = r.Name
	}
	out := []Workspace{}
	for _, s := range shorts {
		if n, ok := by[s]; ok {
			out = append(out, Workspace{ID: s, Name: n})
		}
	}
	return out
}

type WorkspaceInfo struct {
	Workspace
	Usage        generated.MonthUsageRow `json:"usage"`
	Limits       *Plan                   `json:"limits,omitempty"`
	AnthropicKey bool                    `json:"anthropic_key"`
	Billing      bool                    `json:"billing"`
}

func (a *Archive) Info(ctx context.Context) (*WorkspaceInfo, error) {
	if ActorFrom(ctx) == nil {
		return nil, fmt.Errorf("not signed in: %w", ErrUnauthorized)
	}
	q := a.hub.q()
	row, err := q.WorkspaceByShort(ctx, a.ws.Short)
	if err != nil {
		return nil, err
	}
	u, err := q.MonthUsage(ctx, row.ID)
	if err != nil {
		return nil, err
	}
	info := &WorkspaceInfo{Workspace: workspaceOut(row), Usage: u, AnthropicKey: len(row.AnthropicKey) > 0, Billing: billingOn()}
	if row.Plan != nil {
		if p, err := q.PlanByID(ctx, *row.Plan); err == nil {
			info.Limits = planOut(p)
		}
	}
	return info, nil
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

func (a *Archive) anthropicKey(ctx context.Context) (string, bool) {
	if row, err := a.hub.q().WorkspaceByShort(ctx, a.ws.Short); err == nil && len(row.AnthropicKey) > 0 {
		if b, err := a.secretOpen(a.ws.ID, "anthropic", row.AnthropicKey); err == nil {
			return string(b), true
		}
	}
	return os.Getenv("ANTHROPIC_API_KEY"), false
}

func (h *Hub) concurrent(ctx context.Context, row generated.ArchiveWorkspace) int {
	if billingOn() && row.Plan != nil {
		if p, err := h.q().PlanByID(ctx, *row.Plan); err == nil && p.Concurrent != nil && *p.Concurrent > 0 {
			return int(*p.Concurrent)
		}
	}
	return workspaceRuns
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

func (a *Archive) count(ctx context.Context, runs, workflowRuns, aiTokens int64) {
	if err := a.hub.q().AddUsage(context.WithoutCancel(ctx), generated.AddUsageParams{Workspace: a.ws.ID, Runs: runs, WorkflowRuns: workflowRuns, AiTokens: aiTokens}); err != nil {
		fmt.Fprintf(os.Stderr, "archive: workspace %s usage: %v\n", a.ws.Short, err)
	}
}

func (a *Archive) storedBytes(ctx context.Context) error {
	var n int64
	if err := a.st.System(ctx, func(ctx context.Context) (err error) {
		n, err = a.st.db(ctx).StoredBytes(ctx)
		return err
	}); err != nil {
		return err
	}
	return a.hub.q().SetStoredBytes(context.WithoutCancel(ctx), a.ws.ID, n)
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
	authed("GET /auth/settings", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		return ok(h.Settings(r.Context()))
	})
	authed("PATCH /auth/settings", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		var body struct {
			Signup *bool   `json:"signup"`
			Plan   *string `json:"plan"`
		}
		if err := httpDecode(r, &body); err != nil {
			return 0, nil, err
		}
		return ok(h.SetSettings(r.Context(), body.Signup, body.Plan))
	})
	authed("GET /auth/plans", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		return ok(h.Plans(r.Context()))
	})
	authed("PUT /auth/plans/{id}", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		var p Plan
		if err := httpDecode(r, &p); err != nil {
			return 0, nil, err
		}
		p.ID = r.PathValue("id")
		return ok(h.SetPlan(r.Context(), p))
	})
	authed("POST /auth/billing/checkout", func(w http.ResponseWriter, r *http.Request) (int, any, error) {
		var body struct {
			Plan string `json:"plan"`
		}
		if err := httpDecode(r, &body); err != nil {
			return 0, nil, err
		}
		u, err := archiveFrom(r.Context()).Checkout(r.Context(), body.Plan)
		return ok(map[string]string{"url": u}, err)
	})
}
