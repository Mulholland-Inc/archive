package archive

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/oauth2"
	"golang.org/x/time/rate"
)

type Source struct {
	ID        uuid.UUID       `json:"id"`
	Kind      string          `json:"kind"`
	Name      string          `json:"name"`
	Space     uuid.UUID       `json:"space"`
	Owner     string          `json:"-"`
	CreatedBy string          `json:"created_by"`
	Type      string          `json:"type,omitempty"`
	Mode      string          `json:"mode"`
	Schedule  string          `json:"schedule,omitempty"`
	Rate      float64         `json:"rate"`
	Config    json.RawMessage `json:"config"`
	State     json.RawMessage `json:"state"`
	Status    string          `json:"status"`
	Error     string          `json:"error,omitempty"`
	Failures  int             `json:"failures"`
	LastRunAt *time.Time      `json:"last_run_at,omitempty"`
	NextRunAt *time.Time      `json:"next_run_at,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

type SourceInput struct {
	Kind        string         `json:"kind"`
	Name        string         `json:"name"`
	Space       string         `json:"space"`
	Type        string         `json:"type"`
	Mode        string         `json:"mode"`
	Schedule    string         `json:"schedule"`
	Rate        float64        `json:"rate"`
	Config      map[string]any `json:"config"`
	Credentials map[string]any `json:"credentials"`
}

type SyncResult struct {
	Source  uuid.UUID `json:"source"`
	Fetched int       `json:"fetched"`
	New     int       `json:"new"`
	Mapped  int       `json:"mapped"`
	Status  string    `json:"status"`
	Error   string    `json:"error,omitempty"`
}

type connector struct {
	check func(ctx context.Context, a *Archive, src *Source, creds map[string]any) error
	sync  func(ctx context.Context, r *syncRun) error
}

var connectors = map[string]connector{
	"gmail":    {check: checkGoogle, sync: syncGmail},
	"drive":    {check: checkGoogle, sync: syncDrive},
	"imap":     {check: checkIMAP, sync: syncIMAP},
	"folder":   {check: checkFolder, sync: syncFolder},
	"external": {check: checkExternal, sync: syncExternal},
}

var googleScopes = map[string][]string{
	"gmail": {"https://www.googleapis.com/auth/gmail.readonly"},
	"drive": {"https://www.googleapis.com/auth/drive.readonly"},
}

const (
	sourceCols    = `id, kind, name, space, owner, created_by, type, mode, schedule, rate, config, state, status, error, failures, last_run_at, next_run_at, created_at`
	sourceLock    = 7274
	sourceRetries = 3
	sourceBackoff = 500 * time.Millisecond
	sourceRetry   = 30 * time.Second
	sourceSecret  = "source"
)

var sourceNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,63}$`)

func sourceInvalid(path, msg string) error {
	return &ValidationError{Type: "source", Errors: []FieldError{{Path: path, Message: msg}}}
}

func (a *Archive) AddSource(ctx context.Context, in SourceInput) (*Source, string, error) {
	c, ok := connectors[in.Kind]
	if !ok {
		return nil, "", sourceInvalid("kind", "want one of "+strings.Join(slices.Sorted(maps.Keys(connectors)), ", "))
	}
	if in.Kind == "external" || in.Kind == "folder" {
		if err := a.adminOnly(ctx, "adds "+in.Kind+" sources, which run programs or read the server's disk"); err != nil {
			return nil, "", err
		}
	}
	if !sourceNameRe.MatchString(in.Name) {
		return nil, "", sourceInvalid("name", "want 1-64 lowercase letters, digits, _, . or -")
	}
	if in.Mode == "" {
		in.Mode = "mirror"
	}
	if in.Mode != "mirror" {
		return nil, "", sourceInvalid("mode", "only mirror is supported; federate is not built yet")
	}
	if err := a.sourceCheck(in.Type, in.Schedule, in.Rate); err != nil {
		return nil, "", err
	}
	if in.Config == nil {
		in.Config = map[string]any{}
	}
	if in.Credentials == nil {
		in.Credentials = map[string]any{}
	}
	src := &Source{ID: coreNewID(), Kind: in.Kind, Name: in.Name, Type: in.Type, Mode: in.Mode, Schedule: in.Schedule,
		Rate: in.Rate, Status: "new"}
	if src.Rate == 0 {
		src.Rate = 5
	}
	var err error
	if src.Config, err = json.Marshal(in.Config); err != nil {
		return nil, "", err
	}
	if err := c.check(ctx, a, src, in.Credentials); err != nil {
		return nil, "", err
	}
	var authURL string
	if scopes := googleScopes[in.Kind]; scopes != nil && in.Credentials["refresh_token"] == nil {
		redirect := a.base + "/api/sources/oauth/callback"
		cfg, err := googleOAuth(redirect, scopes)
		if err != nil {
			return nil, "", err
		}
		state, verifier := src.ID.String()+"."+authNewSecret(), oauth2.GenerateVerifier()
		in.Credentials = map[string]any{"oauth_state": state, "oauth_verifier": verifier, "redirect_uri": redirect}
		authURL = cfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent"), oauth2.S256ChallengeOption(verifier))
		src.Status = "needs_auth"
	}
	var next *time.Time
	if src.Schedule != "" && src.Status != "needs_auth" {
		now := time.Now()
		next = &now
	}
	return src, authURL, sourceErr(a.as(ctx, func(ctx context.Context) error {
		ac := ActorFrom(ctx)
		sid, err := a.coreSpace(ctx, in.Space)
		if err != nil {
			return err
		}
		s, err := storeOne[Source](ctx, a.st, `
			INSERT INTO sources (id, kind, name, space, owner, created_by, type, mode, schedule, rate, config, status, next_run_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13) RETURNING `+sourceCols,
			src.ID, src.Kind, src.Name, sid, ac.Role, ac.Label, src.Type, src.Mode, src.Schedule, src.Rate, src.Config, src.Status, next)
		if err != nil {
			return err
		}
		*src = *s
		if len(in.Credentials) == 0 {
			return nil
		}
		return a.sourceSeal(ctx, src, in.Credentials)
	}))
}

func (a *Archive) sourceCheck(typ, schedule string, rate float64) error {
	if schedule != "" {
		if d, err := time.ParseDuration(schedule); err != nil || d < time.Second {
			return sourceInvalid("schedule", "want a duration of at least 1s, like 15m or 6h")
		}
	}
	if rate < 0 {
		return sourceInvalid("rate", "want requests per second, 0 for the default")
	}
	if typ != "" {
		return coreHasType(a.Spec(), typ)
	}
	return nil
}

type SourcePatch struct {
	Type     *string  `json:"type"`
	Schedule *string  `json:"schedule"`
	Rate     *float64 `json:"rate"`
}

func (a *Archive) UpdateSource(ctx context.Context, ref string, p SourcePatch) (*Source, error) {
	return coreDo(a, ctx, func(ctx context.Context) (*Source, error) {
		s, err := a.sourceFind(ctx, ref)
		if err != nil {
			return nil, err
		}
		typ, schedule, rate := s.Type, s.Schedule, s.Rate
		if p.Type != nil {
			typ = *p.Type
		}
		if p.Schedule != nil {
			schedule = *p.Schedule
		}
		if p.Rate != nil {
			rate = *p.Rate
		}
		if err := a.sourceCheck(typ, schedule, rate); err != nil {
			return nil, err
		}
		if rate == 0 {
			rate = 5
		}
		out, err := storeOne[Source](ctx, a.st, `UPDATE sources SET type = $2, rate = $3, schedule = $4,
			next_run_at = CASE WHEN $4 = '' OR status = 'needs_auth' THEN NULL WHEN $4 = schedule THEN next_run_at ELSE now() END
			WHERE id = $1 RETURNING `+sourceCols, s.ID, typ, rate, schedule)
		if errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("you cannot change sources in this space: %w", ErrForbidden)
		}
		return out, err
	})
}

func sourceErr(err error) error {
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == "23505" {
		return fmt.Errorf("a source with that name already exists in this space: %w", ErrConflict)
	}
	return err
}

func (a *Archive) sourceSeal(ctx context.Context, src *Source, creds map[string]any) error {
	b, err := json.Marshal(creds)
	if err != nil {
		return err
	}
	id := coreNewID()
	_, err = a.st.Q(ctx).Exec(ctx, `INSERT INTO secrets (id, source_id, type, value, space) VALUES ($1, $2, $3, $4, $5)`,
		id, src.ID, sourceSecret, a.secretBox(id, sourceSecret, b), src.Space)
	return err
}

func (a *Archive) sourceCreds(ctx context.Context, id uuid.UUID) (map[string]any, error) {
	var sid uuid.UUID
	var ct []byte
	creds := map[string]any{}
	err := a.st.Q(ctx).QueryRow(ctx, `SELECT id, value FROM secrets WHERE source_id = $1`, id).Scan(&sid, &ct)
	if errors.Is(err, pgx.ErrNoRows) {
		return creds, nil
	}
	if err != nil {
		return nil, err
	}
	b, err := a.secretOpen(sid, sourceSecret, ct)
	if err != nil {
		return nil, err
	}
	return creds, json.Unmarshal(b, &creds)
}

func (a *Archive) Sources(ctx context.Context) ([]*Source, error) {
	return coreDo(a, ctx, func(ctx context.Context) ([]*Source, error) {
		out, err := storeAll[Source](ctx, a.st, `SELECT `+sourceCols+` FROM sources ORDER BY name, id`)
		if out == nil {
			out = []*Source{}
		}
		return out, err
	})
}

func (a *Archive) sourceFind(ctx context.Context, ref string) (*Source, error) {
	rs, err := storeAll[Source](ctx, a.st, `SELECT `+sourceCols+` FROM sources WHERE id::text = $1 OR name = $1`, ref)
	switch {
	case err != nil:
		return nil, err
	case len(rs) == 0:
		return nil, fmt.Errorf("source %q: %w", ref, ErrNotFound)
	case len(rs) > 1:
		return nil, fmt.Errorf("several sources are named %q; use the id: %w", ref, ErrConflict)
	}
	return rs[0], nil
}

func (a *Archive) GetSource(ctx context.Context, ref string) (*Source, error) {
	return coreDo(a, ctx, func(ctx context.Context) (*Source, error) { return a.sourceFind(ctx, ref) })
}

func (a *Archive) RemoveSource(ctx context.Context, ref string) error {
	return a.as(ctx, func(ctx context.Context) error {
		s, err := a.sourceFind(ctx, ref)
		if err != nil {
			return err
		}
		tag, err := a.st.Q(ctx).Exec(ctx, `DELETE FROM sources WHERE id = $1`, s.ID)
		if err == nil && tag.RowsAffected() == 0 {
			return fmt.Errorf("you cannot remove sources in this space: %w", ErrForbidden)
		}
		return err
	})
}

func (a *Archive) RunSource(ctx context.Context, ref string) (*SyncResult, error) {
	s, err := coreDo(a, ctx, func(ctx context.Context) (*Source, error) {
		s, err := a.sourceFind(ctx, ref)
		if err != nil {
			return nil, err
		}
		var ok bool
		if err := a.st.Q(ctx).QueryRow(ctx, `SELECT $1::uuid = ANY(my_spaces('editor'))`, s.Space).Scan(&ok); err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("you cannot sync sources in this space: %w", ErrForbidden)
		}
		return s, nil
	})
	if err != nil {
		return nil, err
	}
	return a.sync(ctx, s.ID)
}

func (a *Archive) SourceOAuth(ctx context.Context, state, code string) (*Source, error) {
	ref, _, _ := strings.Cut(state, ".")
	id, err := uuid.Parse(ref)
	if err != nil {
		return nil, fmt.Errorf("authorization state names no source: %w", ErrUnauthorized)
	}
	var src *Source
	err = a.st.System(ctx, func(ctx context.Context) error {
		s, err := storeOne[Source](ctx, a.st, `SELECT `+sourceCols+` FROM sources WHERE id = $1`, id)
		if errors.Is(err, ErrNotFound) {
			return fmt.Errorf("authorization state names no source: %w", ErrUnauthorized)
		}
		if err != nil {
			return err
		}
		creds, err := a.sourceCreds(ctx, id)
		if err != nil {
			return err
		}
		want, _ := creds["oauth_state"].(string)
		if want == "" || subtle.ConstantTimeCompare([]byte(want), []byte(state)) != 1 {
			return fmt.Errorf("authorization state does not match; add the source again: %w", ErrUnauthorized)
		}
		redirect, _ := creds["redirect_uri"].(string)
		verifier, _ := creds["oauth_verifier"].(string)
		cfg, err := googleOAuth(redirect, googleScopes[s.Kind])
		if err != nil {
			return err
		}
		tok, err := cfg.Exchange(context.WithValue(ctx, oauth2.HTTPClient, syncClient(rate.NewLimiter(rate.Inf, 1))), code, oauth2.VerifierOption(verifier))
		if err != nil {
			return fmt.Errorf("authorization: %v: %w", err, ErrUnauthorized)
		}
		if tok.RefreshToken == "" {
			return fmt.Errorf("Google sent no refresh token; remove Archive at myaccount.google.com/permissions and add the source again: %w", ErrUnauthorized)
		}
		if _, err := a.st.Q(ctx).Exec(ctx, `DELETE FROM secrets WHERE source_id = $1`, id); err != nil {
			return err
		}
		if err := a.sourceSeal(ctx, s, map[string]any{"refresh_token": tok.RefreshToken}); err != nil {
			return err
		}
		src, err = storeOne[Source](ctx, a.st, `UPDATE sources SET status = 'new', error = '',
			next_run_at = CASE WHEN schedule <> '' THEN now() END WHERE id = $1 RETURNING `+sourceCols, id)
		return err
	})
	return src, err
}

func googleOAuth(redirect string, scopes []string) (*oauth2.Config, error) {
	id, secret := os.Getenv("ARCHIVE_GOOGLE_CLIENT_ID"), os.Getenv("ARCHIVE_GOOGLE_CLIENT_SECRET")
	if id == "" || secret == "" {
		return nil, sourceInvalid("kind", "set ARCHIVE_GOOGLE_CLIENT_ID and ARCHIVE_GOOGLE_CLIENT_SECRET to a Google OAuth client")
	}
	return &oauth2.Config{ClientID: id, ClientSecret: secret, RedirectURL: redirect, Scopes: scopes,
		Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.google.com/o/oauth2/auth",
			TokenURL: coreEnv("ARCHIVE_GOOGLE_TOKEN_URL", "https://oauth2.googleapis.com/token")}}, nil
}

type syncRun struct {
	a      *Archive
	src    *Source
	creds  map[string]any
	octx   context.Context
	limit  *rate.Limiter
	http   *http.Client
	res    *SyncResult
	intake []error
}

func (a *Archive) sync(ctx context.Context, id uuid.UUID) (*SyncResult, error) {
	conn, err := a.st.sys.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	var locked bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1, hashtext($2))`, sourceLock, id.String()).Scan(&locked); err != nil {
		return nil, err
	}
	if !locked {
		return nil, fmt.Errorf("source %s is already syncing: %w", id, ErrConflict)
	}
	defer conn.Exec(context.Background(), `SELECT pg_advisory_unlock($1, hashtext($2))`, sourceLock, id.String())

	var src *Source
	var creds map[string]any
	err = a.st.System(ctx, func(ctx context.Context) (err error) {
		if src, err = storeOne[Source](ctx, a.st, `SELECT `+sourceCols+` FROM sources WHERE id = $1`, id); err != nil {
			return err
		}
		if src.Status == "needs_auth" {
			return fmt.Errorf("source %s is waiting for authorization: %w", src.Name, ErrConflict)
		}
		if creds, err = a.sourceCreds(ctx, id); err != nil {
			return err
		}
		_, err = a.st.Q(ctx).Exec(ctx, `UPDATE sources SET status = 'running' WHERE id = $1`, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	limit := rate.NewLimiter(rate.Limit(src.Rate), 1)
	r := &syncRun{a: a, src: src, creds: creds, limit: limit, http: syncClient(limit), res: &SyncResult{Source: id},
		octx: WithActor(ctx, &Actor{Role: src.Owner})}
	err = errors.Join(append([]error{connectors[src.Kind].sync(r.octx, r)}, r.intake...)...)
	r.res.Status = "ok"
	if err != nil {
		r.res.Status, r.res.Error = "error", err.Error()
	}
	var next *time.Time
	if d, perr := time.ParseDuration(src.Schedule); perr == nil {
		if err != nil {
			d = min(d, sourceRetry<<min(src.Failures, 10))
		}
		t := time.Now().Add(d)
		next = &t
	}
	return r.res, a.st.System(context.WithoutCancel(ctx), func(ctx context.Context) error {
		_, err := a.st.Q(ctx).Exec(ctx, `UPDATE sources SET status = $2, error = $3, last_run_at = now(), next_run_at = $4,
			failures = CASE WHEN $2 = 'ok' THEN 0 ELSE failures + 1 END WHERE id = $1`, id, r.res.Status, r.res.Error, next)
		return err
	})
}

func (r *syncRun) rawSource() string { return sourceRaw(r.src) }

func sourceRaw(src *Source) string {
	if src.Kind == "folder" {
		var cfg struct {
			Path string `json:"path"`
		}
		json.Unmarshal(src.Config, &cfg)
		return "folder:" + cfg.Path
	}
	return src.Kind + ":" + src.ID.String()
}

func (r *syncRun) emit(ctx context.Context, externalID string, payload map[string]any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	raw := &Raw{Source: r.rawSource(), ExternalID: externalID, Payload: b, Space: r.src.Space}
	r.res.Fetched++
	var inserted bool
	if err := r.a.as(ctx, func(ctx context.Context) (err error) {
		inserted, err = r.a.st.PutRaw(ctx, raw)
		return err
	}); err != nil || !inserted {
		return err
	}
	r.res.New++
	r.ingest(ctx, raw)
	return nil
}

func (r *syncRun) ingest(ctx context.Context, raw *Raw) {
	if r.src.Type == "" {
		return
	}
	res, err := r.a.ingest(ctx, raw, r.src.Type)
	if err != nil {
		r.intake = append(r.intake, fmt.Errorf("intake %s: %w", raw.ExternalID, err))
		return
	}
	if res.RecordID != uuid.Nil {
		r.res.Mapped++
	}
}

func (r *syncRun) save(ctx context.Context, state any) error {
	b, err := json.Marshal(state)
	if err != nil {
		return err
	}
	r.src.State = b
	return r.a.st.System(ctx, func(ctx context.Context) error {
		_, err := r.a.st.Q(ctx).Exec(ctx, `UPDATE sources SET state = $2 WHERE id = $1`, r.src.ID, b)
		return err
	})
}

func (a *Archive) syncLoop(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	var mu sync.Mutex
	running := map[uuid.UUID]bool{}
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		var ids []uuid.UUID
		err := a.st.System(ctx, func(ctx context.Context) (err error) {
			rows, _ := a.st.Q(ctx).Query(ctx, `SELECT id FROM sources WHERE next_run_at <= now() AND status <> 'needs_auth'`)
			ids, err = pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
			return err
		})
		if err != nil && ctx.Err() == nil {
			fmt.Fprintln(os.Stderr, "archive: sync:", err)
		}
		for _, id := range ids {
			mu.Lock()
			busy := running[id]
			running[id] = true
			mu.Unlock()
			if busy {
				continue
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				res, err := a.sync(ctx, id)
				if err == nil && res.Error != "" {
					err = errors.New(res.Error)
				}
				if err != nil && !errors.Is(err, ErrConflict) && ctx.Err() == nil {
					fmt.Fprintf(os.Stderr, "archive: sync %s: %v\n", id, err)
				}
				mu.Lock()
				delete(running, id)
				mu.Unlock()
			}()
		}
	}
}

type syncTransport struct{ limit *rate.Limiter }

func syncClient(limit *rate.Limiter) *http.Client {
	return &http.Client{Transport: &syncTransport{limit}, Timeout: 5 * time.Minute}
}

func (t *syncTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	for i := 0; ; i++ {
		if err := t.limit.Wait(ctx); err != nil {
			return nil, err
		}
		r := req
		if i > 0 && req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			r = req.Clone(ctx)
			r.Body = body
		}
		resp, err := http.DefaultTransport.RoundTrip(r)
		retry := err != nil || resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		if !retry || i == sourceRetries || ctx.Err() != nil || (req.Body != nil && req.GetBody == nil) {
			return resp, err
		}
		wait := sourceBackoff << i
		if resp != nil {
			if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil {
				wait = max(wait, min(time.Duration(s)*time.Second, time.Minute))
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
}
