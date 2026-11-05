package main

import (
	"cmp"
	"context"
	"crypto/cipher"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
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
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/text/currency"
	"golang.org/x/text/language"
)

type Archive struct {
	hub    *Hub
	ws     generated.ArchiveWorkspace
	st     *pgStore
	hints  Hints
	prefix string
	aead   cipher.AEAD
	slots  chan struct{}

	blobs   blobStore
	linkKey []byte
	reading sync.Mutex

	mu  sync.RWMutex
	cat *Catalog

	hmu  sync.Mutex
	api  http.Handler
	mcph http.Handler
	mcps map[uuid.UUID]*mcp.Server

	conns archiveConns
}

func (a *Archive) as(ctx context.Context, fn func(context.Context) error) error {
	ac, err := signedIn(ctx)
	if err != nil {
		return err
	}
	if t, ok := ctx.Value(storeTxKey{}).(storeTx); ok && t.role == ac.Role {
		return fn(ctx)
	}
	return a.st.As(ctx, ac.Role, func(ctx context.Context) error {
		if err := fn(ctx); err != nil {
			return err
		}
		tx, err := a.st.db(ctx).TxWrote(ctx)
		if tx.Members {
			storeAfter(ctx, func() {
				if err := a.syncMembers(ctx, true); err != nil {
					fmt.Fprintf(os.Stderr, "archive: workspace %s members: %v\n", a.ws.ID, err)
				}
			})
		}
		if tx.Wrote {
			storeAfter(ctx, func() { a.wakeTables(context.WithValue(context.WithoutCancel(ctx), storeTxKey{}, nil)) })
		}
		return err
	})
}

func (a *Archive) wakeTables(ctx context.Context) {
	var n int64
	err := a.sys(ctx, func(ctx context.Context) (err error) {
		n, err = a.st.db(ctx).WakeTableWorkflows(ctx)
		return err
	})
	if err == nil && n > 0 {
		err = a.soon(ctx, "workflow", time.Now())
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "archive: workspace %s table workflows: %v\n", a.ws.ID, err)
	}
}

func coreDo[T any](a *Archive, ctx context.Context, fn func(context.Context) (T, error)) (T, error) {
	var out T
	err := a.as(ctx, func(ctx context.Context) (err error) {
		out, err = fn(ctx)
		return err
	})
	return out, err
}

func (a *Archive) Close() { a.st.Close() }

func (a *Archive) Catalog() *Catalog {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.cat
}

func (a *Archive) catalog(ctx context.Context) *Catalog {
	if c := a.st.txCatalog(ctx); c != nil {
		return c
	}
	return a.Catalog()
}

func (a *Archive) reload(ctx context.Context) error {
	c, err := a.st.Catalog(ctx)
	if err == nil {
		a.mu.Lock()
		a.cat = c
		a.mu.Unlock()
	}
	return err
}

func (a *Archive) List(ctx context.Context, typ string, where map[string]any, limit int, after uuid.UUID) ([]map[string]any, error) {
	cat := a.catalog(ctx)
	tb, err := cat.table(typ)
	if err != nil {
		return nil, err
	}
	verr := &ValidationError{Type: typ}
	norm := map[string]any{}
	for _, k := range specKeys(where) {
		col := tb.col(k)
		switch {
		case col == nil || !col.writable():
			verr.Errors = append(verr.Errors, FieldError{Path: k, Message: "unknown field"})
		case cat.secret(col) != nil:
			verr.Errors = append(verr.Errors, FieldError{Path: k, Message: "secret fields cannot be filtered"})
		default:
			if v, ok := cat.clean(k, col, where[k], a.hints, &verr.Errors); ok {
				norm[k] = v
			}
		}
	}
	if err := verr.err(); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	return coreDo(a, ctx, func(ctx context.Context) ([]map[string]any, error) {
		if err := a.pgCheck(ctx, tb, norm); err != nil {
			return nil, err
		}
		q := `SELECT ` + tb.rowSQL() + ` FROM ` + tb.ident() + ` t, jsonb_populate_record(NULL::` + tb.ident() + `, $1) w WHERE t.id > $2`
		args := []any{norm, after}
		for _, k := range specKeys(norm) {
			q += " AND t." + pgIdent(k) + " = w." + pgIdent(k)
		}
		args = append(args, limit)
		rs, err := storeRows(ctx, a.st, q+fmt.Sprintf(" ORDER BY t.id LIMIT $%d", len(args)), args...)
		if tb.isWorkflow() {
			a.workflowRows(ctx, rs...)
		}
		return rs, err
	})
}

func (a *Archive) Get(ctx context.Context, typ string, id uuid.UUID) (map[string]any, error) {
	tb, err := a.catalog(ctx).table(typ)
	if err != nil {
		return nil, err
	}
	return coreDo(a, ctx, func(ctx context.Context) (map[string]any, error) {
		row, err := a.coreGet(ctx, tb, id)
		if tb.isWorkflow() {
			a.workflowRows(ctx, row)
		}
		return row, err
	})
}

func (a *Archive) coreGet(ctx context.Context, tb *Table, id uuid.UUID) (map[string]any, error) {
	rs, err := storeRows(ctx, a.st, `SELECT `+tb.rowSQL()+` FROM `+tb.ident()+` t WHERE t.id = $1`, id)
	if err != nil {
		return nil, err
	}
	if len(rs) == 0 {
		return nil, fmt.Errorf("%s %s: %w", tb.Name, id, ErrNotFound)
	}
	return rs[0], nil
}

func (a *Archive) Create(ctx context.Context, typ string, data map[string]any) (map[string]any, error) {
	tb, err := a.catalog(ctx).writable(typ)
	if err != nil {
		return nil, err
	}
	if tb.isWorkflow() {
		return a.workflowSave(ctx, tb, uuid.Nil, data, true)
	}
	return coreDo(a, ctx, func(ctx context.Context) (map[string]any, error) { return a.coreCreate(ctx, tb, data) })
}

func (a *Archive) coreCreate(ctx context.Context, tb *Table, data map[string]any) (map[string]any, error) {
	out, err := a.prepare(ctx, tb, data, true)
	if err != nil {
		return nil, err
	}
	var cols, sel []string
	for _, c := range specKeys(out) {
		if out[c] == nil && tb.col(c).Default {
			continue
		}
		cols, sel = append(cols, pgIdent(c)), append(sel, "r."+pgIdent(c))
	}
	q := `INSERT INTO ` + tb.ident() + ` AS t (` + strings.Join(cols, ", ") + `) SELECT ` + strings.Join(sel, ", ") + ` FROM jsonb_populate_record(NULL::` + tb.ident() + `, $1) r`
	if len(cols) == 0 {
		q = `INSERT INTO ` + tb.ident() + ` AS t SELECT FROM jsonb_populate_record(NULL::` + tb.ident() + `, $1) r`
	}
	rs, err := storeRows(ctx, a.st, q+` RETURNING `+tb.rowSQL(), out)
	if err != nil {
		return nil, err
	}
	return rs[0], nil
}

func coreAccess(v any) *uuid.UUID {
	if s, ok := v.(string); ok {
		if id, err := uuid.Parse(s); err == nil {
			return &id
		}
	}
	return nil
}

func (a *Archive) Replace(ctx context.Context, typ string, id uuid.UUID, data map[string]any) (map[string]any, error) {
	return a.coreWrite(ctx, typ, id, data, true)
}

func (a *Archive) Patch(ctx context.Context, typ string, id uuid.UUID, data map[string]any) (map[string]any, error) {
	return a.coreWrite(ctx, typ, id, data, false)
}

func (a *Archive) coreWrite(ctx context.Context, typ string, id uuid.UUID, data map[string]any, full bool) (map[string]any, error) {
	tb, err := a.catalog(ctx).writable(typ)
	if err != nil {
		return nil, err
	}
	if tb.isWorkflow() {
		return a.workflowSave(ctx, tb, id, data, full)
	}
	return coreDo(a, ctx, func(ctx context.Context) (map[string]any, error) {
		if _, err := a.coreGet(ctx, tb, id); err != nil {
			return nil, err
		}
		out, err := a.prepare(ctx, tb, data, full)
		if err != nil {
			return nil, err
		}
		q := `UPDATE ` + tb.ident() + ` AS t SET updated_at = now()`
		for _, c := range specKeys(out) {
			q += ", " + pgIdent(c) + " = r." + pgIdent(c)
		}
		rs, err := storeRows(ctx, a.st, q+` FROM jsonb_populate_record(NULL::`+tb.ident()+`, $2) r WHERE t.id = $1 RETURNING `+tb.rowSQL(), id, out)
		if err != nil {
			return nil, err
		}
		if len(rs) == 0 {
			return nil, fmt.Errorf("you cannot change this row: %w", ErrForbidden)
		}
		return rs[0], nil
	})
}

func (a *Archive) prepare(ctx context.Context, tb *Table, data map[string]any, full bool) (map[string]any, error) {
	in := maps.Clone(data)
	if err := a.groupNames(ctx, tb, in); err != nil {
		return nil, err
	}
	out, err := a.catalog(ctx).Normalize(tb, in, a.hints, full)
	if err != nil {
		return nil, err
	}
	if err := a.refCheck(ctx, tb, out); err != nil {
		return nil, err
	}
	if err := a.pgCheck(ctx, tb, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (a *Archive) refCheck(ctx context.Context, tb *Table, out map[string]any) error {
	verr := &ValidationError{Type: tb.Name}
	for _, k := range specKeys(out) {
		col := tb.col(k)
		target := col.target()
		if target == "" || out[k] == nil {
			continue
		}
		vals, path := []any{out[k]}, func(int) string { return k }
		if col.List {
			vals, _ = specList(out[k])
			path = func(i int) string { return fmt.Sprintf("%s[%d]", k, i) }
		}
		schema, name, _ := strings.Cut(target, ".")
		for i, v := range vals {
			s, _ := v.(string)
			id, err := uuid.Parse(s)
			if err != nil {
				continue
			}
			var ok bool
			if err := a.st.Q(ctx).QueryRow(ctx, `SELECT EXISTS (SELECT FROM `+pgIdent(schema, name)+` WHERE id = $1)`, id).Scan(&ok); err != nil {
				return err
			}
			if !ok {
				verr.Errors = append(verr.Errors, FieldError{Path: path(i), Message: "no " + name + " " + s})
			}
		}
	}
	return verr.err()
}

func (a *Archive) pgCheck(ctx context.Context, tb *Table, out map[string]any) error {
	cat := a.catalog(ctx)
	var ls []pgLeaf
	for _, k := range specKeys(out) {
		if col := tb.col(k); cat.secret(col) == nil {
			cat.leaves(k, col, out[k], &ls)
		}
	}
	if len(ls) == 0 {
		return nil
	}
	texts, regs := make([]string, len(ls)), make([]string, len(ls))
	for i, l := range ls {
		texts[i], regs[i] = l.text, l.reg
	}
	rows, err := a.st.db(ctx).InputErrors(ctx, texts, regs)
	if err != nil {
		return err
	}
	verr := &ValidationError{Type: tb.Name}
	for _, r := range rows {
		verr.Errors = append(verr.Errors, FieldError{Path: ls[r.I-1].path, Message: r.Message})
	}
	return verr.err()
}

func (a *Archive) Delete(ctx context.Context, typ string, id uuid.UUID) error {
	tb, err := a.catalog(ctx).writable(typ)
	if err != nil {
		return err
	}
	if tb.isWorkflow() {
		return a.workflowDelete(ctx, id)
	}
	return a.as(ctx, func(ctx context.Context) error {
		if _, err := a.coreGet(ctx, tb, id); err != nil {
			return err
		}
		tag, err := a.st.Q(ctx).Exec(ctx, `DELETE FROM `+tb.ident()+` WHERE id = $1`, id)
		if err != nil {
			return storeErr(err)
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("you cannot delete this row: %w", ErrForbidden)
		}
		return nil
	})
}

func (a *Archive) Schema(ctx context.Context, name string) (any, error) {
	if _, err := signedIn(ctx); err != nil {
		return nil, err
	}
	ddl, err := a.st.Schema(ctx)
	if err != nil {
		return nil, err
	}
	cat := a.catalog(ctx)
	out := []runTable{}
	for _, t := range runTables(cat) {
		if t.Name == name || name == "" && cat.Tables[t.Name].Schema == "public" {
			t.DDL = ddl[t.Name]
			out = append(out, t)
		}
	}
	switch {
	case name == "":
		return out, nil
	case len(out) == 1:
		return out[0], nil
	}
	return nil, fmt.Errorf("no table or view %q; there are %s: %w", name, strings.Join(cat.names(), ", "), ErrNotFound)
}

func (a *Archive) Migrate(ctx context.Context, sql, name, confirm string) (*SchemaMigration, error) {
	ac, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	if !a.has(ctx, ac.Role, a.prefix+"_manager") {
		return nil, fmt.Errorf("only managers, engineers and the owner change the schema: %w", ErrForbidden)
	}
	author := ac.Email
	if ac.TokenName != "" {
		author += " (" + ac.TokenName + ")"
	}
	var m *SchemaMigration
	err = a.as(ctx, func(ctx context.Context) (err error) {
		m, err = a.st.Migrate(ctx, sql, name, author, confirm)
		if err == nil && m.Applied {
			storeAfter(ctx, func() { a.reload(context.WithoutCancel(ctx)) })
		}
		return err
	})
	return m, err
}

func (a *Archive) SchemaRevisions(ctx context.Context, limit int) ([]SchemaRevision, error) {
	if _, err := signedIn(ctx); err != nil {
		return nil, err
	}
	return a.st.Revisions(ctx, cmp.Or(min(max(limit, 0), 100), 20))
}

func coreRowID(row map[string]any) uuid.UUID { return coreUUID(row, "id") }

func coreUUID(row map[string]any, key string) uuid.UUID {
	s, _ := row[key].(string)
	id, _ := uuid.Parse(s)
	return id
}

func coreNewID() uuid.UUID { return uuid.Must(uuid.NewV7()) }

func coreEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func coreHints(locale string) Hints {
	h := Hints{Locale: locale, Zone: time.Local.String()}
	if locale == "" {
		return h
	}
	if r, conf := language.Make(locale).Region(); conf != language.No {
		h.Country = r.String()
		if c, ok := currency.FromRegion(r); ok {
			h.Currency = c.String()
		}
	}
	return h
}

type SearchFilter struct {
	Type  string
	From  string
	To    string
	Limit int
}

type SearchHit struct {
	ID      uuid.UUID       `json:"id"`
	Kind    string          `json:"kind"`
	Type    string          `json:"type"`
	Date    time.Time       `json:"date"`
	Data    json.RawMessage `json:"data,omitempty"`
	Snippet string          `json:"snippet,omitempty"`
	Score   float64         `json:"score"`
}

const searchFile = `SELECT 'file' AS kind, 'file' AS type, f.id, f.created_at, jsonb_build_object('id', f.id, 'name', f.name, 'mime', f.mime,
		'size', f.size, 'sha256', f.hash, 'access', f.access, 'created_at', f.created_at, 'kind', f.kind, 'parent', f.parent, 'connection', f.connection, 'url', f.url, 'status', f.status, 'error', f.error, 'edited_at', f.edited_at) AS data, ts_rank(f.search, q.q, 1) AS rank,
		CASE WHEN f.text <> '' THEN ts_headline('simple', left(f.text, 100000), q.q, 'MaxFragments=1, MaxWords=30, MinWords=10, StartSel=**, StopSel=**') ELSE '' END AS snippet
	FROM archive.file f, q WHERE f.search @@ q.q`

func (a *Archive) Search(ctx context.Context, query string, f SearchFilter) ([]*SearchHit, error) {
	if strings.TrimSpace(query) == "" {
		return nil, &ValidationError{Type: "search", Errors: []FieldError{{Path: "q", Message: "empty query"}}}
	}
	cat := a.catalog(ctx)
	var parts []string
	for _, name := range cat.names() {
		tb := cat.Tables[name]
		if tb.Schema == "public" && (f.Type == name || f.Type == "" && tb.Kind == "table") {
			parts = append(parts, `SELECT 'entity' AS kind, `+pgLiteral(name)+` AS type, t.id, t.created_at, `+tb.rowSQL()+` AS data, ts_rank(t.search, q.q, 1) AS rank, '' AS snippet
				FROM `+tb.ident()+` t, q WHERE t.search @@ q.q`)
		}
	}
	if f.Type == "" || f.Type == "file" {
		parts = append(parts, searchFile)
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("type %q: %w", f.Type, ErrNotFound)
	}
	from, err := searchTime("from", f.From, false)
	if err != nil {
		return nil, err
	}
	to, err := searchTime("to", f.To, true)
	if err != nil {
		return nil, err
	}
	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	sql := `WITH q AS (SELECT search_query($1) AS q)
		SELECT h.kind, h.type, h.id, h.created_at, h.data, h.rank, h.snippet FROM (` + strings.Join(parts, "\nUNION ALL\n") + `) h
		WHERE ($2::timestamptz IS NULL OR h.created_at >= $2) AND ($3::timestamptz IS NULL OR h.created_at < $3)
		ORDER BY h.rank DESC, h.id LIMIT $4`
	return coreDo(a, ctx, func(ctx context.Context) ([]*SearchHit, error) {
		rows, _ := a.st.Q(ctx).Query(ctx, sql, query, from, to, limit)
		out := []*SearchHit{}
		var h SearchHit
		_, err := pgx.ForEachRow(rows, []any{&h.Kind, &h.Type, &h.ID, &h.Date, &h.Data, &h.Score, &h.Snippet}, func() error {
			c := h
			c.Data = append(json.RawMessage(nil), h.Data...)
			out = append(out, &c)
			return nil
		})
		return out, err
	})
}

func pgLiteral(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

func searchTime(name, s string, end bool) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return &t, nil
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return nil, &ValidationError{Type: "search", Errors: []FieldError{{Path: name, Message: "want a date like 2026-10-23 or an RFC 3339 time"}}}
	}
	if end {
		t = t.AddDate(0, 0, 1)
	}
	return &t, nil
}

//go:embed archive.sql
var storeSQLFile string

var storeSchema, _, _ = strings.Cut(storeSQLFile, "\n-- name:")

//go:embed main.sql
var mainSQLFile string

var mainSchema, _, _ = strings.Cut(mainSQLFile, "\n-- name:")

const storeRoles = `DO $$ BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '{p}_authenticator') THEN
        CREATE ROLE "{p}_authenticator" LOGIN NOINHERIT;
    END IF;
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '{p}_viewer') THEN
        CREATE ROLE "{p}_viewer" NOLOGIN;
    END IF;
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '{p}_editor') THEN
        CREATE ROLE "{p}_editor" NOLOGIN;
    END IF;
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '{p}_manager') THEN
        CREATE ROLE "{p}_manager" NOLOGIN;
    END IF;
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '{p}_engineer') THEN
        CREATE ROLE "{p}_engineer" NOLOGIN;
    END IF;
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '{p}_owner') THEN
        CREATE ROLE "{p}_owner" NOLOGIN;
    END IF;
    GRANT "{p}_viewer" TO "{p}_editor";
    GRANT "{p}_editor" TO "{p}_manager";
    GRANT "{p}_manager" TO "{p}_engineer";
    GRANT "{p}_engineer" TO "{p}_owner";
    IF NOT EXISTS (SELECT FROM pg_db_role_setting s JOIN pg_database d ON d.oid = s.setdatabase WHERE d.datname = current_database() AND s.setrole = 0) THEN
        EXECUTE format('ALTER DATABASE %I SET search_path = archive, public', current_database());
        EXECUTE format('ALTER DATABASE %I SET timezone = %L', current_database(), 'UTC');
    END IF;
    IF NOT has_database_privilege('{p}_authenticator', current_database(), 'CONNECT') THEN
        EXECUTE format('GRANT CONNECT ON DATABASE %I TO "{p}_authenticator"', current_database());
    END IF;
    IF NOT has_database_privilege('{p}_viewer', current_database(), 'CONNECT') THEN
        EXECUTE format('GRANT CONNECT ON DATABASE %I TO "{p}_viewer"', current_database());
    END IF;
END $$`

const storeLock = 7271

type storeQ interface {
	generated.DBTX
	Begin(ctx context.Context) (pgx.Tx, error)
}

type storeTxKey struct{}

type storeTx struct {
	tx    pgx.Tx
	role  string
	after *[]func()
	sch   *schemaTx
}

func storeAfter(ctx context.Context, fn func()) {
	if t, ok := ctx.Value(storeTxKey{}).(storeTx); ok && t.after != nil {
		*t.after = append(*t.after, fn)
		return
	}
	fn()
}

func storeCommitted(after *[]func(), err error) error {
	if err == nil {
		for _, fn := range *after {
			fn()
		}
	}
	return err
}

type pgStore struct {
	app, sys *pgxpool.Pool
	cfg      *pgx.ConnConfig
	prefix   string
	password string
	base     *pgx.ConnConfig
	moved    func(table, id string)
	mu       sync.Mutex
	roles    map[string]*pgxpool.Pool
}

func OpenStore(ctx context.Context, pg *pgx.ConnConfig, prefix, password string, moved func(table, id string)) (*pgStore, error) {
	cfg := pg.Copy()
	cfg.RuntimeParams["search_path"] = "public"
	s := &pgStore{cfg: cfg, prefix: prefix, password: password, base: pg, moved: moved, roles: map[string]*pgxpool.Pool{}}
	var err error
	if s.sys, err = storePool(ctx, pg, storeSysConns, moved, nil); err == nil {
		err = s.bootstrap(ctx, password)
	}
	if err == nil {
		auth := prefix + "_authenticator"
		s.app, err = storePool(ctx, pg, storeAppConns, moved, func(c *pgx.ConnConfig) { c.User, c.Password = auth, password })
	}
	if err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

const (
	storeAppConns  = 6
	storeSysConns  = 3
	storeRoleConns = 2
	storeIdle      = time.Minute
)

func storePool(ctx context.Context, base *pgx.ConnConfig, conns int32, moved func(table, id string), fn func(*pgx.ConnConfig)) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig("")
	if err != nil {
		return nil, err
	}
	cfg.ConnConfig = base.Copy()
	cfg.ConnConfig.RuntimeParams["search_path"] = "archive, public"
	cfg.ConnConfig.RuntimeParams["timezone"] = "UTC"
	cfg.MaxConns, cfg.MinConns, cfg.MaxConnIdleTime = conns, 0, storeIdle
	if fn != nil {
		fn(cfg.ConnConfig)
	}
	if moved != nil {
		livePool(cfg, moved)
	}
	return pgxpool.NewWithConfig(ctx, cfg)
}

func (s *pgStore) bootstrap(ctx context.Context, password string) error {
	tx, err := s.sys.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	if err := generated.New(tx).AdvisoryXactLock(ctx, storeLock); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, s.expand(storeRoles)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "ALTER ROLE "+pgIdent(s.prefix+"_authenticator")+" PASSWORD '"+password+"'"); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	rev, _ := generated.New(s.sys).Revision(ctx)
	if strings.HasSuffix(rev, ":"+s.hash()) {
		return nil
	}
	return s.migrate(ctx)
}

func (s *pgStore) expand(sql string) string { return strings.ReplaceAll(sql, "{p}", s.prefix) }

func (s *pgStore) hash() string {
	h := sha256.Sum256([]byte(s.expand(storeSchema)))
	return hex.EncodeToString(h[:])
}

func storeUnquote(id string) string {
	if strings.HasPrefix(id, `"`) {
		return strings.ReplaceAll(id[1:len(id)-1], `""`, `"`)
	}
	return id
}

var (
	storeDollarRe = regexp.MustCompile(`^\$([A-Za-z_][A-Za-z0-9_]*)?\$`)
	storeIdentRe  = regexp.MustCompile(`[A-Za-z0-9_$]`)
)

func storeSkip(src string, i int) (int, bool) {
	switch c := src[i]; {
	case c == '-' && strings.HasPrefix(src[i:], "--"):
		if n := strings.IndexByte(src[i:], '\n'); n >= 0 {
			return i + n, true
		}
		return len(src), true
	case c == '/' && strings.HasPrefix(src[i:], "/*"):
		for d := 0; i < len(src); i++ {
			if strings.HasPrefix(src[i:], "/*") {
				d, i = d+1, i+1
			} else if strings.HasPrefix(src[i:], "*/") {
				if d, i = d-1, i+1; d == 0 {
					return i, true
				}
			}
		}
		return len(src), true
	case c == '\'' || c == '"':
		esc := c == '\'' && i > 0 && (src[i-1] == 'E' || src[i-1] == 'e') && (i == 1 || !storeIdentRe.MatchString(src[i-2:i-1]))
		for i++; i < len(src) && src[i] != c; i++ {
			if esc && src[i] == '\\' {
				i++
			}
		}
		return i, false
	case c == '$':
		if m := storeDollarRe.FindString(src[i:]); m != "" && (i == 0 || !storeIdentRe.MatchString(src[i-1:i])) {
			if n := strings.Index(src[i+len(m):], m); n >= 0 {
				return i + len(m) + n + len(m) - 1, false
			}
			return len(src), false
		}
	}
	return i, false
}

func storeSplit(src string) []string {
	var out []string
	start := 0
	for i := 0; i < len(src); i++ {
		if src[i] == ';' {
			if st := strings.TrimSpace(src[start:i]); st != "" {
				out = append(out, st)
			}
			start = i + 1
			continue
		}
		i, _ = storeSkip(src, i)
	}
	if st := strings.TrimSpace(src[start:]); st != "" {
		out = append(out, st)
	}
	return out
}

func (s *pgStore) Catalog(ctx context.Context) (*Catalog, error) {
	q := generated.New(s.sys)
	b, err := q.Catalog(ctx)
	if err != nil {
		return nil, err
	}
	rev, err := q.Revision(ctx)
	if err != nil {
		return nil, err
	}
	c, err := catalogParse(b)
	if c != nil {
		c.stamp = rev
	}
	return c, err
}

func (s *pgStore) Close() {
	s.mu.Lock()
	for _, p := range s.roles {
		p.Close()
	}
	s.mu.Unlock()
	if s.app != nil {
		s.app.Close()
	}
	if s.sys != nil {
		s.sys.Close()
	}
}

func (s *pgStore) Q(ctx context.Context) storeQ {
	if t, ok := ctx.Value(storeTxKey{}).(storeTx); ok {
		return t.tx
	}
	return s.app
}

func (s *pgStore) db(ctx context.Context) *generated.Queries { return generated.New(s.Q(ctx)) }

func (s *pgStore) As(ctx context.Context, role string, fn func(context.Context) error) error {
	if t, ok := ctx.Value(storeTxKey{}).(storeTx); ok && t.role == role {
		return fn(ctx)
	}
	pool, err := s.rolePool(ctx, role)
	if err != nil {
		return err
	}
	after := &[]func(){}
	return storeCommitted(after, storeErr(pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if err := generated.New(tx).SetLocal(ctx, "role", role); err != nil {
			return err
		}
		return fn(context.WithValue(ctx, storeTxKey{}, storeTx{tx, role, after, &schemaTx{}}))
	})))
}

func (s *pgStore) rolePool(ctx context.Context, role string) (*pgxpool.Pool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p := s.roles[role]; p != nil {
		return p, nil
	}
	if !strings.HasPrefix(role, s.prefix+"_") {
		return nil, fmt.Errorf("role %s is not this workspace's: %w", role, ErrForbidden)
	}
	password := s.rolePassword(role)
	err := pgx.BeginFunc(ctx, s.sys, func(tx pgx.Tx) error {
		if err := generated.New(tx).AdvisoryXactLock(ctx, storeLock); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, "ALTER ROLE "+pgIdent(role)+" LOGIN PASSWORD '"+password+"'")
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("logging in as %s: %w", role, err)
	}
	p, err := storePool(ctx, s.base, storeRoleConns, s.moved, func(c *pgx.ConnConfig) { c.User, c.Password = role, password })
	if err != nil {
		return nil, err
	}
	s.roles[role] = p
	return p, nil
}

func (s *pgStore) rolePassword(role string) string {
	sum := sha256.Sum256([]byte(s.password + "\x00" + role))
	return hex.EncodeToString(sum[:])
}

func (s *pgStore) System(ctx context.Context, fn func(context.Context) error) error {
	if t, ok := ctx.Value(storeTxKey{}).(storeTx); ok && t.role == "" {
		return fn(ctx)
	}
	after := &[]func(){}
	return storeCommitted(after, storeErr(pgx.BeginFunc(ctx, s.sys, func(tx pgx.Tx) error {
		return fn(context.WithValue(ctx, storeTxKey{}, storeTx{tx, "", after, &schemaTx{}}))
	})))
}

func storeErr(err error) error {
	var pe *pgconn.PgError
	if !errors.As(err, &pe) {
		return err
	}
	switch {
	case pe.Code == "42501" && (strings.HasPrefix(pe.Message, "must be owner of ") || strings.HasPrefix(pe.Message, "permission denied for schema ")):
		return fmt.Errorf("%s; change the schema with archive.migrate: %w", pe.Message, ErrForbidden)
	case pe.Code == "42501":
		return fmt.Errorf("%s: %w", pe.Message, ErrForbidden)
	case pe.Code == "23503" || pe.Code == "23001" || pe.Code == "23505":
		return fmt.Errorf("%s: %w", cmp.Or(pe.Detail, pe.Message), ErrConflict)
	case pe.Code == "23502" && pe.ColumnName != "":
		return &ValidationError{Type: pe.TableName, Errors: []FieldError{{Path: pe.ColumnName, Message: "required"}}}
	case pe.Code == "23514" || pe.Code == "23502" || strings.HasPrefix(pe.Code, "22"):
		return &ValidationError{Type: cmp.Or(pe.TableName, pe.DataTypeName), Errors: []FieldError{{Path: pe.ColumnName, Message: pe.Message}}}
	}
	return err
}

func pgIdent(parts ...string) string { return pgx.Identifier(parts).Sanitize() }

func storeRows(ctx context.Context, s *pgStore, sql string, args ...any) ([]map[string]any, error) {
	rows, _ := s.Q(ctx).Query(ctx, sql, args...)
	out := []map[string]any{}
	var b []byte
	_, err := pgx.ForEachRow(rows, []any{&b}, func() error {
		r, err := rowDecode(b)
		out = append(out, r)
		return err
	})
	return out, err
}

func rowDecode(b []byte) (map[string]any, error) {
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.UseNumber()
	var m map[string]any
	return m, dec.Decode(&m)
}
