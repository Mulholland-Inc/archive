package main

import (
	"context"
	"crypto/cipher"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Mulholland-Inc/archive/generated"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

	mu  sync.RWMutex
	cat *Catalog
	wmu sync.Mutex

	hmu  sync.Mutex
	api  http.Handler
	mcph http.Handler
}

func (a *Archive) as(ctx context.Context, fn func(context.Context) error) error {
	ac := ActorFrom(ctx)
	if ac == nil {
		return fmt.Errorf("not signed in: %w", ErrUnauthorized)
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

func (a *Archive) Close() error {
	a.st.Close()
	return nil
}

func (a *Archive) Catalog() *Catalog {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.cat
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
	cat := a.Catalog()
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
		q := `SELECT ` + tb.rowSQL("t") + ` FROM ` + tb.ident() + ` t, jsonb_populate_record(NULL::` + tb.ident() + `, $1) w WHERE t.id > $2`
		args := []any{norm, after}
		for _, k := range specKeys(norm) {
			q += " AND t." + pgIdent(k) + " = w." + pgIdent(k)
		}
		args = append(args, limit)
		return storeRows(ctx, a.st, q+fmt.Sprintf(" ORDER BY t.id LIMIT $%d", len(args)), args...)
	})
}

func (a *Archive) Get(ctx context.Context, typ string, id uuid.UUID) (map[string]any, error) {
	tb, err := a.Catalog().table(typ)
	if err != nil {
		return nil, err
	}
	return coreDo(a, ctx, func(ctx context.Context) (map[string]any, error) { return a.coreGet(ctx, tb, id) })
}

func (a *Archive) coreGet(ctx context.Context, tb *Table, id uuid.UUID) (map[string]any, error) {
	rs, err := storeRows(ctx, a.st, `SELECT `+tb.rowSQL("t")+` FROM `+tb.ident()+` t WHERE t.id = $1`, id)
	if err != nil {
		return nil, err
	}
	if len(rs) == 0 {
		return nil, fmt.Errorf("%s %s: %w", tb.Name, id, ErrNotFound)
	}
	return rs[0], nil
}

func (a *Archive) Create(ctx context.Context, typ string, data map[string]any) (map[string]any, error) {
	tb, err := a.Catalog().writable(typ)
	if err != nil {
		return nil, err
	}
	return coreDo(a, ctx, func(ctx context.Context) (map[string]any, error) { return a.coreCreate(ctx, tb, data) })
}

func (a *Archive) coreCreate(ctx context.Context, tb *Table, data map[string]any) (map[string]any, error) {
	out, seal, err := a.prepare(ctx, tb, data, true, nil)
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
	rs, err := storeRows(ctx, a.st, q+` RETURNING `+tb.rowSQL("t"), out)
	if err != nil {
		return nil, err
	}
	row := rs[0]
	return row, a.secretsPut(ctx, tb, row, seal)
}

func coreAccess(v any) *string {
	if s, ok := v.(string); ok && s != "" {
		return &s
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
	tb, err := a.Catalog().writable(typ)
	if err != nil {
		return nil, err
	}
	return coreDo(a, ctx, func(ctx context.Context) (map[string]any, error) {
		old, err := a.coreGet(ctx, tb, id)
		if err != nil {
			return nil, err
		}
		out, seal, err := a.prepare(ctx, tb, data, full, old)
		if err != nil {
			return nil, err
		}
		q := `UPDATE ` + tb.ident() + ` AS t SET updated_at = now()`
		for _, c := range specKeys(out) {
			q += ", " + pgIdent(c) + " = r." + pgIdent(c)
		}
		rs, err := storeRows(ctx, a.st, q+` FROM jsonb_populate_record(NULL::`+tb.ident()+`, $2) r WHERE t.id = $1 RETURNING `+tb.rowSQL("t"), id, out)
		if err != nil {
			return nil, err
		}
		if len(rs) == 0 {
			return nil, fmt.Errorf("you cannot change this row: %w", ErrForbidden)
		}
		return rs[0], a.secretsPut(ctx, tb, rs[0], seal)
	})
}

func (a *Archive) prepare(ctx context.Context, tb *Table, data map[string]any, full bool, old map[string]any) (map[string]any, map[string]any, error) {
	cat := a.Catalog()
	in, keep := map[string]any{}, map[string]any{}
	for k, v := range data {
		if str, ok := v.(string); ok && old != nil && old[k] == str && tb.col(k) != nil && cat.secret(tb.col(k)) != nil {
			keep[k] = v
			continue
		}
		in[k] = v
	}
	out, err := cat.Normalize(tb, in, a.hints, full)
	if err != nil {
		return nil, nil, err
	}
	if err := a.refCheck(ctx, tb, out); err != nil {
		return nil, nil, err
	}
	if err := a.pgCheck(ctx, tb, out); err != nil {
		return nil, nil, err
	}
	seal := map[string]any{}
	for k, v := range out {
		if s := cat.secret(tb.col(k)); s != nil {
			seal[k] = v
			if v != nil {
				out[k] = secretMask(s, v)
			}
		}
	}
	for k, v := range keep {
		out[k] = v
		delete(seal, k)
	}
	return out, seal, nil
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
	cat := a.Catalog()
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
	tb, err := a.Catalog().writable(typ)
	if err != nil {
		return err
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
		return a.st.db(ctx).ClearSecrets(ctx, tb.Name, id, nil)
	})
}

func (a *Archive) SchemaFiles(ctx context.Context) (map[string]string, error) {
	if ActorFrom(ctx) == nil {
		return nil, fmt.Errorf("not signed in: %w", ErrUnauthorized)
	}
	return a.st.SchemaFiles(ctx)
}

func (a *Archive) schemaAdmin(ctx context.Context) error {
	ac := ActorFrom(ctx)
	if ac == nil {
		return fmt.Errorf("not signed in: %w", ErrUnauthorized)
	}
	if !a.owner(ctx, ac.Role) {
		return fmt.Errorf("only owners change the schema: %w", ErrForbidden)
	}
	return nil
}

func (a *Archive) ApplySchema(ctx context.Context, files map[string]string, renames []string, allowDrop, dryRun bool) (*SchemaPlan, error) {
	if !dryRun || len(renames) > 0 {
		if err := a.schemaAdmin(ctx); err != nil {
			return nil, err
		}
	}
	a.wmu.Lock()
	defer a.wmu.Unlock()
	p, err := a.st.Apply(ctx, files, renames, allowDrop, dryRun)
	if err != nil || dryRun {
		return p, err
	}
	return p, a.reload(ctx)
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
		'size', f.size, 'sha256', f.hash, 'access', f.access, 'created_at', f.created_at) AS data, ts_rank(f.search, q.q, 1) AS rank,
		CASE WHEN f.text <> '' THEN ts_headline('simple', left(f.text, 100000), q.q, 'MaxFragments=1, MaxWords=30, MinWords=10, StartSel=**, StopSel=**') ELSE '' END AS snippet
	FROM archive.file f, q WHERE f.search @@ q.q`

func (a *Archive) Search(ctx context.Context, query string, f SearchFilter) ([]*SearchHit, error) {
	if strings.TrimSpace(query) == "" {
		return nil, &ValidationError{Type: "search", Errors: []FieldError{{Path: "q", Message: "empty query"}}}
	}
	cat := a.Catalog()
	var parts []string
	for _, name := range cat.names() {
		tb := cat.Tables[name]
		if tb.Schema == "public" && (f.Type == name || f.Type == "" && tb.Kind == "table") {
			parts = append(parts, `SELECT 'entity' AS kind, `+pgLiteral(name)+` AS type, t.id, t.created_at, `+tb.rowSQL("t")+` AS data, ts_rank(t.search, q.q, 1) AS rank, '' AS snippet
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
