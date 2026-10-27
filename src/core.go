package main

import (
	"context"
	"crypto/cipher"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/text/currency"
	"golang.org/x/text/language"
)

type Archive struct {
	st     *pgStore
	hints  Hints
	stop   func() error
	prefix string
	base   string
	aead   cipher.AEAD

	blobs    blobStore
	uploads  string
	linkKey  []byte
	wake     chan struct{}
	tusLocks sync.Map
	pages    sync.Map

	mu  sync.RWMutex
	cat *Catalog
	wmu sync.Mutex
}

var corePrefixRe = regexp.MustCompile(`^[a-z][a-z0-9]{0,15}$`)

func Open(ctx context.Context) (*Archive, error) {
	dir := coreEnv("ARCHIVE_DATA", "./data")
	prefix := coreEnv("ARCHIVE_INSTANCE", "a")
	if !corePrefixRe.MatchString(prefix) {
		return nil, fmt.Errorf("ARCHIVE_INSTANCE %q: want 1-16 lowercase letters and digits, starting with a letter", prefix)
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
	uploads := filepath.Join(dir, "uploads")
	if err := os.MkdirAll(uploads, 0o700); err != nil {
		return nil, err
	}
	dsn, stop := os.Getenv("DATABASE_URL"), func() error { return nil }
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
	st, err := OpenStore(ctx, dsn, prefix, authPassword(key), dir)
	if err != nil {
		stop()
		return nil, fmt.Errorf("open store: %w", err)
	}
	cat, err := st.Catalog(ctx)
	if err != nil {
		st.Close()
		stop()
		return nil, fmt.Errorf("load catalog: %w", err)
	}
	return &Archive{st: st, cat: cat, hints: coreHints(os.Getenv("ARCHIVE_LOCALE")), wake: make(chan struct{}, 1),
		base: strings.TrimRight(coreEnv("ARCHIVE_URL", "http://localhost:8080"), "/"), prefix: prefix, aead: aead, stop: stop,
		blobs: blobs, uploads: uploads, linkKey: secretDerive(key, "files")}, nil
}

func (a *Archive) as(ctx context.Context, fn func(context.Context) error) error {
	ac := ActorFrom(ctx)
	if ac == nil {
		return fmt.Errorf("not signed in: %w", ErrUnauthorized)
	}
	return a.st.As(ctx, ac.Role, fn)
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
	if a.stop != nil {
		return a.stop()
	}
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

func (a *Archive) List(ctx context.Context, typ, space string, where map[string]any, limit int, after uuid.UUID) ([]map[string]any, error) {
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
	if len(verr.Errors) > 0 {
		return nil, verr
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
		if space != "" {
			sid, err := a.coreSpace(ctx, space)
			if err != nil {
				return nil, err
			}
			args = append(args, sid)
			q += fmt.Sprintf(" AND t.space = $%d", len(args))
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

func (a *Archive) Create(ctx context.Context, space, typ string, data map[string]any) (map[string]any, error) {
	tb, err := a.Catalog().table(typ)
	if err != nil {
		return nil, err
	}
	return coreDo(a, ctx, func(ctx context.Context) (map[string]any, error) {
		sid, err := a.coreSpace(ctx, space)
		if err != nil {
			return nil, err
		}
		return a.coreCreate(ctx, tb, sid, data)
	})
}

func (a *Archive) coreCreate(ctx context.Context, tb *Table, sid uuid.UUID, data map[string]any) (map[string]any, error) {
	out, seal, err := a.prepare(ctx, tb, data, true, nil)
	if err != nil {
		return nil, err
	}
	cols := specKeys(out)
	q := `INSERT INTO ` + tb.ident() + ` AS t (space`
	sel := ` SELECT $2`
	for _, c := range cols {
		if out[c] == nil && tb.col(c).Default {
			continue
		}
		q += ", " + pgIdent(c)
		sel += ", r." + pgIdent(c)
	}
	rs, err := storeRows(ctx, a.st, q+")"+sel+` FROM jsonb_populate_record(NULL::`+tb.ident()+`, $1) r RETURNING `+tb.rowSQL("t"), out, sid)
	if err != nil {
		return nil, err
	}
	row := rs[0]
	return row, a.secretsPut(ctx, tb, row, seal)
}

func (a *Archive) coreSpace(ctx context.Context, name string) (uuid.UUID, error) {
	var id uuid.UUID
	q, args := `SELECT id FROM space WHERE name = $1`, []any{name}
	if name == "" {
		home := uuid.Nil
		if ac := ActorFrom(ctx); ac != nil {
			home = ac.Home
		}
		q, args = `SELECT id FROM space WHERE id = ANY(my_spaces('editor')) ORDER BY id = $1 DESC, id LIMIT 1`, []any{home}
	}
	err := a.st.Q(ctx).QueryRow(ctx, q, args...).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		if name == "" {
			return id, fmt.Errorf("no space you can write to: %w", ErrForbidden)
		}
		return id, fmt.Errorf("space %q: %w", name, ErrNotFound)
	}
	return id, err
}

func (a *Archive) Replace(ctx context.Context, typ string, id uuid.UUID, data map[string]any) (map[string]any, error) {
	return a.coreWrite(ctx, typ, id, data, true)
}

func (a *Archive) Patch(ctx context.Context, typ string, id uuid.UUID, data map[string]any) (map[string]any, error) {
	return a.coreWrite(ctx, typ, id, data, false)
}

func (a *Archive) coreWrite(ctx context.Context, typ string, id uuid.UUID, data map[string]any, full bool) (map[string]any, error) {
	tb, err := a.Catalog().table(typ)
	if err != nil {
		return nil, err
	}
	return coreDo(a, ctx, func(ctx context.Context) (map[string]any, error) {
		old, err := a.coreGet(ctx, tb, id)
		if err != nil {
			return nil, err
		}
		var writable bool
		if err := a.st.Q(ctx).QueryRow(ctx, `SELECT $1::uuid = ANY(my_spaces('editor'))`, old["space"]).Scan(&writable); err != nil {
			return nil, err
		}
		if !writable {
			return nil, fmt.Errorf("you cannot change objects in this space: %w", ErrForbidden)
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
			return nil, fmt.Errorf("you cannot change objects in this space: %w", ErrForbidden)
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
		if target == "" || target == "archive.space" || out[k] == nil {
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
	if len(verr.Errors) > 0 {
		return verr
	}
	return nil
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
	rows, _ := a.st.Q(ctx).Query(ctx, `SELECT u.i, e.message FROM unnest($1::text[], $2::text[]) WITH ORDINALITY u(v, t, i),
		pg_input_error_info(u.v, u.t) e WHERE e.message IS NOT NULL ORDER BY u.i`, texts, regs)
	verr := &ValidationError{Type: tb.Name}
	var i int
	var msg string
	_, err := pgx.ForEachRow(rows, []any{&i, &msg}, func() error {
		verr.Errors = append(verr.Errors, FieldError{Path: ls[i-1].path, Message: msg})
		return nil
	})
	if err != nil {
		return err
	}
	if len(verr.Errors) > 0 {
		return verr
	}
	return nil
}

func (a *Archive) Delete(ctx context.Context, typ string, id uuid.UUID) error {
	tb, err := a.Catalog().table(typ)
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
			return fmt.Errorf("you cannot delete objects in this space: %w", ErrForbidden)
		}
		_, err = a.st.Q(ctx).Exec(ctx, `SELECT secret_clear($1, $2, NULL)`, tb.Name, id)
		return err
	})
}

func (a *Archive) SchemaFiles(ctx context.Context) (map[string]string, error) {
	if ActorFrom(ctx) == nil {
		return nil, fmt.Errorf("not signed in: %w", ErrUnauthorized)
	}
	return a.st.SchemaFiles()
}

func (a *Archive) ApplySchema(ctx context.Context, files map[string]string, renames []string, allowDrop, dryRun bool) (*SchemaPlan, error) {
	if files == nil {
		var err error
		if files, err = a.st.SchemaFiles(); err != nil {
			return nil, err
		}
	}
	admin, err := coreDo(a, ctx, func(ctx context.Context) (bool, error) {
		var ok bool
		return ok, a.st.Q(ctx).QueryRow(ctx, `SELECT ontology_admin()`).Scan(&ok)
	})
	if err != nil {
		return nil, err
	}
	if !admin && (!dryRun || len(renames) > 0) {
		return nil, fmt.Errorf("only ontology admins change the schema: %w", ErrForbidden)
	}
	a.wmu.Lock()
	defer a.wmu.Unlock()
	p, err := a.st.Apply(ctx, files, renames, allowDrop, dryRun)
	if err != nil || dryRun {
		return p, err
	}
	if err := a.st.writeFiles(files); err != nil {
		return p, err
	}
	return p, a.reload(ctx)
}

func coreRowID(row map[string]any) uuid.UUID {
	s, _ := row["id"].(string)
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
