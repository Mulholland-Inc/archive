package main

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/emersion/go-message/charset"
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
	Status  string    `json:"status"`
	Error   string    `json:"error,omitempty"`
}

type connector struct {
	check func(ctx context.Context, a *Archive, src *Source, creds map[string]any) error
	sync  func(ctx context.Context, r *syncRun) error
}

var connectors = map[string]connector{
	"gmail":  {check: checkGoogle, sync: syncGmail},
	"drive":  {check: checkGoogle, sync: syncDrive},
	"folder": {check: checkFolder, sync: syncFolder},
}

var googleScopes = map[string][]string{
	"gmail": {"https://www.googleapis.com/auth/gmail.readonly"},
	"drive": {"https://www.googleapis.com/auth/drive.readonly"},
}

const (
	sourceCols    = `id, kind, name, space, owner, created_by, mode, schedule, rate, config, state, status, error, failures, last_run_at, next_run_at, created_at`
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
	if !sourceNameRe.MatchString(in.Name) {
		return nil, "", sourceInvalid("name", "want 1-64 lowercase letters, digits, _, . or -")
	}
	if in.Mode == "" {
		in.Mode = "mirror"
	}
	if in.Mode != "mirror" {
		return nil, "", sourceInvalid("mode", "only mirror is supported; federate is not built yet")
	}
	if err := sourceCheck(in.Schedule, in.Rate); err != nil {
		return nil, "", err
	}
	if in.Config == nil {
		in.Config = map[string]any{}
	}
	if in.Credentials == nil {
		in.Credentials = map[string]any{}
	}
	src := &Source{ID: coreNewID(), Kind: in.Kind, Name: in.Name, Mode: in.Mode, Schedule: in.Schedule,
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
			INSERT INTO source (id, kind, name, space, owner, created_by, mode, schedule, rate, config, status, next_run_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) RETURNING `+sourceCols,
			src.ID, src.Kind, src.Name, sid, ac.Role, ac.Label, src.Mode, src.Schedule, src.Rate, src.Config, src.Status, next)
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

func sourceCheck(schedule string, rate float64) error {
	if schedule != "" {
		if d, err := time.ParseDuration(schedule); err != nil || d < time.Second {
			return sourceInvalid("schedule", "want a duration of at least 1s, like 15m or 6h")
		}
	}
	if rate < 0 {
		return sourceInvalid("rate", "want requests per second, 0 for the default")
	}
	return nil
}

type SourcePatch struct {
	Schedule *string  `json:"schedule"`
	Rate     *float64 `json:"rate"`
}

func (a *Archive) UpdateSource(ctx context.Context, ref string, p SourcePatch) (*Source, error) {
	return coreDo(a, ctx, func(ctx context.Context) (*Source, error) {
		s, err := a.sourceFind(ctx, ref)
		if err != nil {
			return nil, err
		}
		schedule, rate := s.Schedule, s.Rate
		if p.Schedule != nil {
			schedule = *p.Schedule
		}
		if p.Rate != nil {
			rate = *p.Rate
		}
		if err := sourceCheck(schedule, rate); err != nil {
			return nil, err
		}
		if rate == 0 {
			rate = 5
		}
		out, err := storeOne[Source](ctx, a.st, `UPDATE source SET rate = $2, schedule = $3,
			next_run_at = CASE WHEN $3 = '' OR status = 'needs_auth' THEN NULL WHEN $3 = schedule THEN next_run_at ELSE now() END
			WHERE id = $1 RETURNING `+sourceCols, s.ID, rate, schedule)
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
	_, err = a.st.Q(ctx).Exec(ctx, `INSERT INTO secret (id, table_name, row_id, column_name, type, value, space) VALUES ($1, 'source', $2, 'credentials', $3, $4, $5)`,
		id, src.ID, sourceSecret, a.secretBox(id, sourceSecret, b), src.Space)
	return err
}

func (a *Archive) sourceCreds(ctx context.Context, id uuid.UUID) (map[string]any, error) {
	var sid uuid.UUID
	var ct []byte
	creds := map[string]any{}
	err := a.st.Q(ctx).QueryRow(ctx, `SELECT id, value FROM secret WHERE table_name = 'source' AND row_id = $1`, id).Scan(&sid, &ct)
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
		out, err := storeAll[Source](ctx, a.st, `SELECT `+sourceCols+` FROM source ORDER BY name, id`)
		if out == nil {
			out = []*Source{}
		}
		return out, err
	})
}

func (a *Archive) sourceFind(ctx context.Context, ref string) (*Source, error) {
	rs, err := storeAll[Source](ctx, a.st, `SELECT `+sourceCols+` FROM source WHERE id::text = $1 OR name = $1`, ref)
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
		tag, err := a.st.Q(ctx).Exec(ctx, `DELETE FROM source WHERE id = $1`, s.ID)
		if err == nil && tag.RowsAffected() == 0 {
			return fmt.Errorf("you cannot remove sources in this space: %w", ErrForbidden)
		}
		if err == nil {
			_, err = a.st.Q(ctx).Exec(ctx, `SELECT secret_clear('source', $1, NULL)`, s.ID)
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
		s, err := storeOne[Source](ctx, a.st, `SELECT `+sourceCols+` FROM source WHERE id = $1`, id)
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
		if _, err := a.st.Q(ctx).Exec(ctx, `DELETE FROM secret WHERE table_name = 'source' AND row_id = $1`, id); err != nil {
			return err
		}
		if err := a.sourceSeal(ctx, s, map[string]any{"refresh_token": tok.RefreshToken}); err != nil {
			return err
		}
		src, err = storeOne[Source](ctx, a.st, `UPDATE source SET status = 'new', error = '',
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
	a     *Archive
	src   *Source
	creds map[string]any
	octx  context.Context
	limit *rate.Limiter
	http  *http.Client
	res   *SyncResult
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
		if src, err = storeOne[Source](ctx, a.st, `SELECT `+sourceCols+` FROM source WHERE id = $1`, id); err != nil {
			return err
		}
		if src.Status == "needs_auth" {
			return fmt.Errorf("source %s is waiting for authorization: %w", src.Name, ErrConflict)
		}
		if creds, err = a.sourceCreds(ctx, id); err != nil {
			return err
		}
		_, err = a.st.Q(ctx).Exec(ctx, `UPDATE source SET status = 'running' WHERE id = $1`, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	limit := rate.NewLimiter(rate.Limit(src.Rate), 1)
	r := &syncRun{a: a, src: src, creds: creds, limit: limit, http: syncClient(limit), res: &SyncResult{Source: id},
		octx: WithActor(ctx, &Actor{Role: src.Owner})}
	err = connectors[src.Kind].sync(r.octx, r)
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
		_, err := a.st.Q(ctx).Exec(ctx, `UPDATE source SET status = $2, error = $3, last_run_at = now(), next_run_at = $4,
			failures = CASE WHEN $2 = 'ok' THEN 0 ELSE failures + 1 END WHERE id = $1`, id, r.res.Status, r.res.Error, next)
		return err
	})
}

func (r *syncRun) file(ctx context.Context, name string, b []byte) (*File, error) {
	r.res.Fetched++
	f, err := coreDo(r.a, ctx, func(ctx context.Context) (*File, error) {
		return r.a.fileAdd(ctx, r.src.Space, name, bytes.NewReader(b))
	})
	if err == nil {
		r.res.New++
	}
	return f, err
}

func (r *syncRun) save(ctx context.Context, state any) error {
	b, err := json.Marshal(state)
	if err != nil {
		return err
	}
	r.src.State = b
	return r.a.st.System(ctx, func(ctx context.Context) error {
		_, err := r.a.st.Q(ctx).Exec(ctx, `UPDATE source SET state = $2 WHERE id = $1`, r.src.ID, b)
		return err
	})
}

func (a *Archive) syncLoop(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	var mu sync.Mutex
	running := map[uuid.UUID]bool{}
	wait := time.Second
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		var ids []uuid.UUID
		var next *time.Time
		err := a.st.System(ctx, func(ctx context.Context) (err error) {
			rows, _ := a.st.Q(ctx).Query(ctx, `SELECT id FROM source WHERE next_run_at <= now() AND status <> 'needs_auth'`)
			if ids, err = pgx.CollectRows(rows, pgx.RowTo[uuid.UUID]); err != nil {
				return err
			}
			return a.st.Q(ctx).QueryRow(ctx, `SELECT min(next_run_at) FROM source WHERE next_run_at > now() AND status <> 'needs_auth'`).Scan(&next)
		})
		wait = time.Second
		if next != nil {
			wait = min(wait, max(time.Until(*next), 10*time.Millisecond))
		}
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

const syncMaxBody = 32 << 20

var mailHeaders = []string{"From", "To", "Cc", "Subject", "Date", "Message-ID", "In-Reply-To", "References", "Reply-To"}

func mailText(headers map[string]string, body string) string {
	var text strings.Builder
	for _, h := range []string{"From", "To", "Cc", "Subject", "Date"} {
		if v := headers[h]; v != "" {
			text.WriteString(h + ": " + v + "\n")
		}
	}
	return text.String() + "\n" + body
}

func mailEML(headers map[string]string, body string) []byte {
	var b strings.Builder
	for _, h := range mailHeaders {
		if v := headers[h]; v != "" {
			b.WriteString(h + ": " + strings.ReplaceAll(strings.ReplaceAll(v, "\r", ""), "\n", " ") + "\r\n")
		}
	}
	b.WriteString("MIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n" + body)
	return []byte(b.String())
}

func googleAPI() string {
	return strings.TrimRight(coreEnv("ARCHIVE_GOOGLE_API_URL", "https://www.googleapis.com"), "/")
}

func checkGoogle(_ context.Context, _ *Archive, _ *Source, _ map[string]any) error {
	_, err := googleOAuth("", nil)
	return err
}

func (r *syncRun) google(ctx context.Context) (*http.Client, error) {
	cfg, err := googleOAuth("", googleScopes[r.src.Kind])
	if err != nil {
		return nil, err
	}
	rt, _ := r.creds["refresh_token"].(string)
	return cfg.Client(context.WithValue(ctx, oauth2.HTTPClient, r.http), &oauth2.Token{RefreshToken: rt}), nil
}

func syncFetch(ctx context.Context, hc *http.Client, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, syncMaxBody))
	path, _, _ := strings.Cut(u, "?")
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, fmt.Errorf("GET %s: %w", path, ErrNotFound)
	case resp.StatusCode/100 != 2:
		return nil, fmt.Errorf("GET %s: %s: %s", path, resp.Status, bytes.TrimSpace(b[:min(len(b), 300)]))
	}
	return b, err
}

func syncGet(ctx context.Context, hc *http.Client, u string, out any) error {
	b, err := syncFetch(ctx, hc, u)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

type gmailPart struct {
	MimeType string `json:"mimeType"`
	Filename string `json:"filename"`
	Headers  []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"headers"`
	Body struct {
		AttachmentID string `json:"attachmentId"`
		Size         int    `json:"size"`
		Data         string `json:"data"`
	} `json:"body"`
	Parts []gmailPart `json:"parts"`
}

type gmailMessage struct {
	ID           string    `json:"id"`
	ThreadID     string    `json:"threadId"`
	LabelIDs     []string  `json:"labelIds"`
	Snippet      string    `json:"snippet"`
	InternalDate string    `json:"internalDate"`
	Payload      gmailPart `json:"payload"`
}

func (p *gmailPart) walk(body *string, atts *[]map[string]any) {
	if p.Filename != "" {
		*atts = append(*atts, map[string]any{"filename": p.Filename, "mime": p.MimeType, "size": p.Body.Size, "attachment_id": p.Body.AttachmentID})
	} else if p.MimeType == "text/plain" && *body == "" {
		if b, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(p.Body.Data, "=")); err == nil {
			*body = string(b)
		}
	}
	for i := range p.Parts {
		p.Parts[i].walk(body, atts)
	}
}

func syncGmail(ctx context.Context, r *syncRun) error {
	hc, err := r.google(ctx)
	if err != nil {
		return err
	}
	base := googleAPI() + "/gmail/v1/users/me"
	var st struct {
		HistoryID string `json:"history_id"`
	}
	json.Unmarshal(r.src.State, &st)
	var ids []string
	next := st.HistoryID
	if st.HistoryID != "" {
		ids, next, err = gmailHistory(ctx, hc, base, st.HistoryID)
		if errors.Is(err, ErrNotFound) {
			st.HistoryID = ""
		} else if err != nil {
			return err
		}
	}
	if st.HistoryID == "" {
		var p struct {
			HistoryID string `json:"historyId"`
		}
		if err := syncGet(ctx, hc, base+"/profile", &p); err != nil {
			return err
		}
		var cfg struct {
			Query string `json:"query"`
		}
		json.Unmarshal(r.src.Config, &cfg)
		if ids, err = gmailList(ctx, hc, base, cmp.Or(cfg.Query, "newer_than:30d")); err != nil {
			return err
		}
		next = p.HistoryID
	}
	for _, id := range ids {
		var m gmailMessage
		if err := syncGet(ctx, hc, base+"/messages/"+url.PathEscape(id)+"?format=full", &m); errors.Is(err, ErrNotFound) {
			continue
		} else if err != nil {
			return err
		}
		headers := map[string]string{}
		for _, h := range m.Payload.Headers {
			for _, want := range mailHeaders {
				if strings.EqualFold(h.Name, want) && headers[want] == "" {
					headers[want] = h.Value
				}
			}
		}
		var body string
		var atts []map[string]any
		m.Payload.walk(&body, &atts)
		for _, att := range atts {
			aid, _ := att["attachment_id"].(string)
			if aid == "" {
				continue
			}
			var data struct {
				Data string `json:"data"`
			}
			if err := syncGet(ctx, hc, base+"/messages/"+url.PathEscape(m.ID)+"/attachments/"+url.PathEscape(aid), &data); err != nil {
				return err
			}
			b, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(data.Data, "="))
			if err != nil {
				return fmt.Errorf("gmail attachment %s: %w", aid, err)
			}
			name, _ := att["filename"].(string)
			if _, err := r.file(ctx, name, b); err != nil {
				return err
			}
		}
		if _, err := r.file(ctx, cmp.Or(headers["Subject"], m.ID)+".eml", mailEML(headers, cmp.Or(body, m.Snippet))); err != nil {
			return err
		}
	}
	return r.save(ctx, map[string]string{"history_id": next})
}

func gmailList(ctx context.Context, hc *http.Client, base, query string) ([]string, error) {
	var ids []string
	for page := ""; ; {
		var out struct {
			Messages []struct {
				ID string `json:"id"`
			} `json:"messages"`
			NextPageToken string `json:"nextPageToken"`
		}
		q := url.Values{"q": {query}, "maxResults": {"100"}}
		if page != "" {
			q.Set("pageToken", page)
		}
		if err := syncGet(ctx, hc, base+"/messages?"+q.Encode(), &out); err != nil {
			return nil, err
		}
		for _, m := range out.Messages {
			ids = append(ids, m.ID)
		}
		if page = out.NextPageToken; page == "" {
			slices.Reverse(ids)
			return ids, nil
		}
	}
}

func gmailHistory(ctx context.Context, hc *http.Client, base, start string) ([]string, string, error) {
	var ids []string
	seen := map[string]bool{}
	for page := ""; ; {
		var out struct {
			History []struct {
				MessagesAdded []struct {
					Message struct {
						ID string `json:"id"`
					} `json:"message"`
				} `json:"messagesAdded"`
			} `json:"history"`
			HistoryID     string `json:"historyId"`
			NextPageToken string `json:"nextPageToken"`
		}
		q := url.Values{"startHistoryId": {start}, "historyTypes": {"messageAdded"}}
		if page != "" {
			q.Set("pageToken", page)
		}
		if err := syncGet(ctx, hc, base+"/history?"+q.Encode(), &out); err != nil {
			return nil, "", err
		}
		for _, h := range out.History {
			for _, m := range h.MessagesAdded {
				if !seen[m.Message.ID] {
					seen[m.Message.ID] = true
					ids = append(ids, m.Message.ID)
				}
			}
		}
		if page = out.NextPageToken; page == "" {
			return ids, cmp.Or(out.HistoryID, start), nil
		}
	}
}

const driveFields = "id,name,mimeType,modifiedTime,createdTime,size,md5Checksum,parents,webViewLink,trashed,owners(emailAddress,displayName)"

func syncDrive(ctx context.Context, r *syncRun) error {
	hc, err := r.google(ctx)
	if err != nil {
		return err
	}
	base := googleAPI() + "/drive/v3"
	var st struct {
		PageToken string `json:"page_token"`
	}
	json.Unmarshal(r.src.State, &st)
	if st.PageToken == "" {
		var start struct {
			StartPageToken string `json:"startPageToken"`
		}
		if err := syncGet(ctx, hc, base+"/changes/startPageToken", &start); err != nil {
			return err
		}
		for page := ""; ; {
			var out struct {
				Files         []map[string]any `json:"files"`
				NextPageToken string           `json:"nextPageToken"`
			}
			q := url.Values{"q": {"trashed = false"}, "pageSize": {"100"}, "fields": {"nextPageToken,files(" + driveFields + ")"}}
			if page != "" {
				q.Set("pageToken", page)
			}
			if err := syncGet(ctx, hc, base+"/files?"+q.Encode(), &out); err != nil {
				return err
			}
			for _, f := range out.Files {
				if err := r.driveFile(ctx, hc, base, f); err != nil {
					return err
				}
			}
			if page = out.NextPageToken; page == "" {
				break
			}
		}
		return r.save(ctx, map[string]string{"page_token": start.StartPageToken})
	}
	for tok := st.PageToken; ; {
		var out struct {
			Changes []struct {
				Removed bool           `json:"removed"`
				File    map[string]any `json:"file"`
			} `json:"changes"`
			NextPageToken     string `json:"nextPageToken"`
			NewStartPageToken string `json:"newStartPageToken"`
		}
		q := url.Values{"pageToken": {tok}, "pageSize": {"100"}, "fields": {"nextPageToken,newStartPageToken,changes(removed,file(" + driveFields + "))"}}
		if err := syncGet(ctx, hc, base+"/changes?"+q.Encode(), &out); err != nil {
			return err
		}
		for _, c := range out.Changes {
			if c.Removed || c.File == nil || c.File["trashed"] == true {
				continue
			}
			if err := r.driveFile(ctx, hc, base, c.File); err != nil {
				return err
			}
		}
		tok = cmp.Or(out.NextPageToken, out.NewStartPageToken)
		if err := r.save(ctx, map[string]string{"page_token": tok}); err != nil || out.NextPageToken == "" {
			return err
		}
	}
}

func (r *syncRun) driveFile(ctx context.Context, hc *http.Client, base string, f map[string]any) error {
	id, _ := f["id"].(string)
	mt, _ := f["mimeType"].(string)
	name, _ := f["name"].(string)
	size, _ := strconv.Atoi(fmt.Sprint(f["size"]))
	var u string
	switch {
	case mt == "application/vnd.google-apps.document":
		u, name = base+"/files/"+url.PathEscape(id)+"/export?mimeType=text%2Fplain", name+".txt"
	case !strings.HasPrefix(mt, "application/vnd.google-apps.") && size <= syncMaxBody:
		u = base + "/files/" + url.PathEscape(id) + "?alt=media"
	default:
		return nil
	}
	b, err := syncFetch(ctx, hc, u)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err == nil {
		_, err = r.file(ctx, name, b)
	}
	return err
}

func checkFolder(_ context.Context, _ *Archive, src *Source, _ map[string]any) error {
	var cfg map[string]any
	json.Unmarshal(src.Config, &cfg)
	p, _ := cfg["path"].(string)
	abs, err := filepath.Abs(p)
	if fi, serr := os.Stat(abs); p == "" || err != nil || serr != nil || !fi.IsDir() {
		return sourceInvalid("config.path", "want a folder on the Archive server")
	}
	cfg["path"] = abs
	src.Config, err = json.Marshal(cfg)
	return err
}

func syncFolder(ctx context.Context, r *syncRun) error {
	var cfg struct {
		Path string `json:"path"`
	}
	json.Unmarshal(r.src.Config, &cfg)
	var st struct {
		Files map[string]string `json:"files"`
	}
	json.Unmarshal(r.src.State, &st)
	if st.Files == nil {
		st.Files = map[string]string{}
	}
	err := filepath.WalkDir(cfg.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != cfg.Path && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(cfg.Path, path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		if hash := hex.EncodeToString(sum[:]); st.Files[filepath.ToSlash(rel)] != hash {
			if _, err := r.file(ctx, d.Name(), b); err != nil {
				return err
			}
			st.Files[filepath.ToSlash(rel)] = hash
		}
		return nil
	})
	if err != nil {
		return err
	}
	return r.save(ctx, st)
}
