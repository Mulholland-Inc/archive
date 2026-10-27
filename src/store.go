package main

import (
	"cmp"
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/stripe/pg-schema-diff/pkg/diff"
	"github.com/stripe/pg-schema-diff/pkg/sqldb"
	"github.com/stripe/pg-schema-diff/pkg/tempdb"
)

//go:embed archive.sql
var storeSchema string

const storeRoles = `DO $$ BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '{p}_authenticator') THEN
        CREATE ROLE {p}_authenticator LOGIN NOINHERIT;
    END IF;
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '{p}_user') THEN
        CREATE ROLE {p}_user NOLOGIN;
    END IF;
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '{p}_admin') THEN
        CREATE ROLE {p}_admin NOLOGIN;
    END IF;
END $$`

const (
	storeLock     = 7271
	storeRevision = `SELECT coalesce((SELECT id::text || ':' || hash FROM archive.revision ORDER BY created_at DESC, id DESC LIMIT 1), '')`
)

type storeQ interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

type storeTxKey struct{}

type storeTx struct {
	tx   pgx.Tx
	role string
}

type pgStore struct {
	app, sys *pgxpool.Pool
	ddl      *sql.DB
	cfg      *pgx.ConnConfig
	prefix   string
	dir      string
}

type SchemaStatement struct {
	DDL     string                 `json:"ddl"`
	Hazards []diff.MigrationHazard `json:"hazards,omitempty"`
	Blocked bool                   `json:"blocked,omitempty"`
	timeout time.Duration
	lock    time.Duration
}

type SchemaPlan struct {
	Statements []SchemaStatement `json:"statements"`
	Applied    bool              `json:"applied"`
}

func (p *SchemaPlan) blocked() []string {
	var out []string
	for _, s := range p.Statements {
		if s.Blocked {
			out = append(out, s.DDL)
		}
	}
	return out
}

func OpenStore(ctx context.Context, dsn, prefix, password, dir string) (*pgStore, error) {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.RuntimeParams["search_path"] = "public"
	s := &pgStore{cfg: cfg, prefix: prefix, dir: filepath.Join(dir, "schema"), ddl: stdlib.OpenDB(*cfg.Copy())}
	if s.sys, err = storePool(ctx, dsn, nil); err == nil {
		err = s.bootstrap(ctx, password)
	}
	if err == nil {
		auth := prefix + "_authenticator"
		s.app, err = storePool(ctx, dsn, func(c *pgx.ConnConfig) { c.User, c.Password = auth, password })
	}
	if err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

func storePool(ctx context.Context, dsn string, fn func(*pgx.ConnConfig)) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = "archive, public"
	cfg.ConnConfig.RuntimeParams["timezone"] = "UTC"
	if fn != nil {
		fn(cfg.ConnConfig)
	}
	return pgxpool.NewWithConfig(ctx, cfg)
}

func (s *pgStore) bootstrap(ctx context.Context, password string) error {
	if _, err := s.sys.Exec(ctx, s.expand(storeRoles)); err != nil {
		return err
	}
	if _, err := s.sys.Exec(ctx, "ALTER ROLE "+s.prefix+"_authenticator PASSWORD '"+password+"'"); err != nil {
		return err
	}
	files, err := s.SchemaFiles()
	if err != nil {
		return err
	}
	var rev string
	s.sys.QueryRow(ctx, storeRevision).Scan(&rev)
	if strings.HasSuffix(rev, ":"+s.hash(files)) {
		return nil
	}
	p, err := s.Apply(ctx, files, nil, false, false)
	if err != nil && p != nil && len(p.blocked()) > 0 {
		fmt.Fprintf(os.Stderr, "archive: schema not applied at startup, it would delete data:\n  %s\nrun archive schema apply -allow-drop, or change %s\n", strings.Join(p.blocked(), "\n  "), s.dir)
		return nil
	}
	return err
}

func (s *pgStore) expand(sql string) string { return strings.ReplaceAll(sql, "{p}", s.prefix) }

func (s *pgStore) hash(files map[string]string) string {
	h := sha256.New()
	h.Write([]byte(s.expand(storeSchema)))
	for _, name := range specKeys(files) {
		fmt.Fprintf(h, "\x00%s\x00%s", name, files[name])
	}
	return hex.EncodeToString(h.Sum(nil))
}

const storeMinTimeout = time.Minute

var storeFileRe = regexp.MustCompile(`^[a-z0-9_][a-z0-9_.-]*\.sql$`)

func (s *pgStore) SchemaFiles() (map[string]string, error) {
	out := map[string]string{}
	paths, err := filepath.Glob(filepath.Join(s.dir, "*.sql"))
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		out[filepath.Base(p)] = string(b)
	}
	return out, err
}

func storeCheckFiles(files map[string]string) error {
	ve := &ValidationError{Type: "schema"}
	for _, name := range specKeys(files) {
		if !storeFileRe.MatchString(name) {
			ve.Errors = append(ve.Errors, FieldError{Path: name, Message: "want a file name like house.sql: lowercase letters, digits, _, . or -"})
		}
	}
	if len(ve.Errors) > 0 {
		return ve
	}
	return nil
}

func (s *pgStore) writeFiles(files map[string]string) error {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	old, err := s.SchemaFiles()
	if err != nil {
		return err
	}
	for name := range old {
		if _, ok := files[name]; !ok {
			if err := os.Remove(filepath.Join(s.dir, name)); err != nil {
				return err
			}
		}
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(s.dir, name), []byte(body), 0o600); err != nil {
			return err
		}
	}
	return nil
}

func (s *pgStore) openDB(_ context.Context, name string) (*sql.DB, error) {
	c := s.cfg.Copy()
	c.Database = name
	return stdlib.OpenDB(*c), nil
}

func (s *pgStore) Apply(ctx context.Context, files map[string]string, renames []string, allowDrop, dryRun bool) (*SchemaPlan, error) {
	if err := storeCheckFiles(files); err != nil {
		return nil, err
	}
	if err := storeCheckRenames(renames); err != nil {
		return nil, err
	}
	conn, err := s.ddl.Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, storeLock); err != nil {
		return nil, err
	}
	defer conn.ExecContext(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1)`, storeLock)
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SET LOCAL check_function_bodies = off`); err != nil {
		return nil, err
	}
	var src sqldb.Queryable = s.ddl
	if len(renames) > 0 {
		if _, err := tx.ExecContext(ctx, `SET LOCAL lock_timeout = '10s'`); err != nil {
			return nil, err
		}
		for i, r := range renames {
			if _, err := tx.ExecContext(ctx, r); err != nil {
				return nil, fmt.Errorf("renames[%d]: %w", i, err)
			}
		}
		src = tx
	}
	p, err := s.plan(ctx, src, files, allowDrop)
	if err != nil || dryRun {
		return storeWithRenames(p, renames), err
	}
	if b := p.blocked(); len(b) > 0 {
		return storeWithRenames(p, renames), fmt.Errorf("the plan deletes data (%s); apply it with allow_drop: %w", strings.Join(b, "; "), ErrConflict)
	}
	for _, st := range p.Statements {
		set := fmt.Sprintf(`SET LOCAL statement_timeout = %d; SET LOCAL lock_timeout = %d`, max(st.timeout, storeMinTimeout).Milliseconds(), st.lock.Milliseconds())
		if _, err := tx.ExecContext(ctx, set); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, st.DDL); err != nil {
			return p, fmt.Errorf("schema: %s: %w", st.DDL, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `SET LOCAL statement_timeout = 0; INSERT INTO archive.revision (hash) VALUES ('`+s.hash(files)+`')`); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	p.Applied = true
	return storeWithRenames(p, renames), nil
}

func storeWithRenames(p *SchemaPlan, renames []string) *SchemaPlan {
	if p == nil || len(renames) == 0 {
		return p
	}
	out := make([]SchemaStatement, 0, len(renames)+len(p.Statements))
	for _, r := range renames {
		out = append(out, SchemaStatement{DDL: strings.TrimSuffix(strings.TrimSpace(r), ";")})
	}
	p.Statements = append(out, p.Statements...)
	return p
}

var storeRenameRe = regexp.MustCompile(`(?is)^ALTER\s+(TABLE|TYPE|DOMAIN|INDEX)\s+(IF\s+EXISTS\s+)?(ONLY\s+)?((public|"public")\.)?("[a-z_][a-z0-9_]*"|[a-z_][a-z0-9_]*)\s+RENAME\s+[^;]+$`)

func storeCheckRenames(renames []string) error {
	ve := &ValidationError{Type: "schema"}
	for i, r := range renames {
		if !storeRenameRe.MatchString(strings.TrimSuffix(strings.TrimSpace(r), ";")) {
			ve.Errors = append(ve.Errors, FieldError{Path: fmt.Sprintf("renames[%d]", i), Message: "want one ALTER TABLE|TYPE|DOMAIN|INDEX <name> RENAME ... statement on a public object"})
		}
	}
	if len(ve.Errors) > 0 {
		return ve
	}
	return nil
}

func (s *pgStore) plan(ctx context.Context, src sqldb.Queryable, files map[string]string, allowDrop bool) (*SchemaPlan, error) {
	f, err := tempdb.NewOnInstanceFactory(ctx, s.openDB, tempdb.WithRootDatabase(s.cfg.Database))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tmp, err := f.Create(ctx)
	if err != nil {
		return nil, err
	}
	defer tmp.Close(context.WithoutCancel(ctx))
	if _, err := tmp.ConnPool.ExecContext(ctx, s.expand(storeSchema)); err != nil {
		return nil, fmt.Errorf("archive.sql: %w", err)
	}
	if err := storeLoad(ctx, tmp.ConnPool, files); err != nil {
		return nil, err
	}
	var b []byte
	if err := tmp.ConnPool.QueryRowContext(ctx, catalogSQL).Scan(&b); err != nil {
		return nil, err
	}
	cat, err := catalogParse(b)
	if err != nil {
		return nil, err
	}
	if err := cat.Check(); err != nil {
		return nil, err
	}
	plan, err := diff.Generate(ctx, diff.DBSchemaSource(src), diff.DBSchemaSource(tmp.ConnPool), diff.WithTempDbFactory(f),
		diff.WithIncludeSchemas("archive", "public"), diff.WithNoConcurrentIndexOps(), diff.WithDisableCheckFunctionBodies())
	if err != nil {
		return nil, fmt.Errorf("schema plan: %w", err)
	}
	out := &SchemaPlan{Statements: []SchemaStatement{}}
	for _, st := range plan.Statements {
		drop := slices.ContainsFunc(st.Hazards, func(h diff.MigrationHazard) bool { return h.Type == diff.MigrationHazardTypeDeletesData })
		out.Statements = append(out.Statements, SchemaStatement{DDL: st.DDL, Hazards: st.Hazards, Blocked: drop && !allowDrop, timeout: st.Timeout, lock: st.LockTimeout})
	}
	return out, nil
}

func storeLoad(ctx context.Context, db *sql.DB, files map[string]string) error {
	left := specKeys(files)
	errs := map[string]error{}
	for progress := true; progress && len(left) > 0; {
		progress = false
		var next []string
		for _, name := range left {
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, files[name]); err == nil {
				err = tx.Commit()
			}
			if err != nil {
				tx.Rollback()
				errs[name] = err
				next = append(next, name)
				continue
			}
			progress = true
		}
		left = next
	}
	if len(left) == 0 {
		return nil
	}
	ve := &ValidationError{Type: "schema"}
	for _, name := range left {
		ve.Errors = append(ve.Errors, FieldError{Path: name, Message: errs[name].Error()})
	}
	return ve
}

func (s *pgStore) Catalog(ctx context.Context) (*Catalog, error) {
	var b []byte
	var rev string
	if err := s.sys.QueryRow(ctx, catalogSQL).Scan(&b); err != nil {
		return nil, err
	}
	if err := s.sys.QueryRow(ctx, storeRevision).Scan(&rev); err != nil {
		return nil, err
	}
	c, err := catalogParse(b)
	if c != nil {
		c.stamp = rev
	}
	return c, err
}

func (s *pgStore) Close() {
	if s.app != nil {
		s.app.Close()
	}
	if s.sys != nil {
		s.sys.Close()
	}
	s.ddl.Close()
}

func (s *pgStore) Q(ctx context.Context) storeQ {
	if t, ok := ctx.Value(storeTxKey{}).(storeTx); ok {
		return t.tx
	}
	return s.app
}

func (s *pgStore) As(ctx context.Context, role string, fn func(context.Context) error) error {
	if t, ok := ctx.Value(storeTxKey{}).(storeTx); ok && t.role == role {
		return fn(ctx)
	}
	return storeErr(pgx.BeginFunc(ctx, s.app, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SET LOCAL ROLE "+pgx.Identifier{role}.Sanitize()); err != nil {
			return err
		}
		return fn(context.WithValue(ctx, storeTxKey{}, storeTx{tx, role}))
	}))
}

func (s *pgStore) System(ctx context.Context, fn func(context.Context) error) error {
	if t, ok := ctx.Value(storeTxKey{}).(storeTx); ok && t.role == "" {
		return fn(ctx)
	}
	return storeErr(pgx.BeginFunc(ctx, s.sys, func(tx pgx.Tx) error {
		return fn(context.WithValue(ctx, storeTxKey{}, storeTx{tx, ""}))
	}))
}

func storeErr(err error) error {
	var pe *pgconn.PgError
	if !errors.As(err, &pe) {
		return err
	}
	switch {
	case pe.Code == "42501":
		return fmt.Errorf("%s: %w", pe.Message, ErrForbidden)
	case pe.Code == "23503":
		return fmt.Errorf("%s: %w", cmp.Or(pe.Detail, pe.Message), ErrConflict)
	case pe.Code == "23502" && pe.ColumnName != "":
		return &ValidationError{Type: pe.TableName, Errors: []FieldError{{Path: pe.ColumnName, Message: "required"}}}
	case pe.Code == "23514" || pe.Code == "23502" || strings.HasPrefix(pe.Code, "22"):
		return &ValidationError{Type: cmp.Or(pe.TableName, pe.DataTypeName), Errors: []FieldError{{Path: pe.ColumnName, Message: pe.Message}}}
	}
	return err
}

func pgIdent(parts ...string) string { return pgx.Identifier(parts).Sanitize() }

func storeOne[T any](ctx context.Context, s *pgStore, sql string, args ...any) (*T, error) {
	rows, _ := s.Q(ctx).Query(ctx, sql, args...)
	v, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[T])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return v, err
}

func storeAll[T any](ctx context.Context, s *pgStore, sql string, args ...any) ([]*T, error) {
	rows, _ := s.Q(ctx).Query(ctx, sql, args...)
	return pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[T])
}

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
