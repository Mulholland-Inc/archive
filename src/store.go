package main

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"cmp"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Mulholland-Inc/archive/generated"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/stripe/pg-schema-diff/pkg/diff"
	"github.com/stripe/pg-schema-diff/pkg/sqldb"
	"github.com/stripe/pg-schema-diff/pkg/tempdb"
	"github.com/ulikunitz/xz"
)

//go:embed archive.sql
var storeSQLFile string

var storeSchema, _, _ = strings.Cut(storeSQLFile, "\n-- name:")

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
    IF NOT has_database_privilege('{p}_authenticator', current_database(), 'CONNECT') THEN
        EXECUTE format('GRANT CONNECT ON DATABASE %I TO {p}_authenticator', current_database());
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
	ddl      *sql.DB
	cfg      *pgx.ConnConfig
	prefix   string
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
	Deferred   bool              `json:"deferred,omitempty"`
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

func OpenStore(ctx context.Context, pooled, direct *pgx.ConnConfig, prefix, password string) (*pgStore, error) {
	cfg := direct.Copy()
	cfg.RuntimeParams["search_path"] = "public"
	db := stdlib.OpenDB(*cfg.Copy())
	db.SetMaxOpenConns(storeDDLConns)
	db.SetConnMaxIdleTime(storeIdle)
	s := &pgStore{cfg: cfg, prefix: prefix, ddl: db}
	var err error
	if s.sys, err = storePool(ctx, pooled, storeSysConns, nil); err == nil {
		err = s.bootstrap(ctx, password)
	}
	if err == nil {
		auth := prefix + "_authenticator"
		s.app, err = storePool(ctx, pooled, storeAppConns, func(c *pgx.ConnConfig) { c.User, c.Password = auth, password })
	}
	if err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

const (
	storeAppConns = 8
	storeSysConns = 4
	storeDDLConns = 2
	storeIdle     = time.Minute
)

func storePool(ctx context.Context, base *pgx.ConnConfig, conns int32, fn func(*pgx.ConnConfig)) (*pgxpool.Pool, error) {
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
	return pgxpool.NewWithConfig(ctx, cfg)
}

func (s *pgStore) bootstrap(ctx context.Context, password string) error {
	if _, err := s.sys.Exec(ctx, s.expand(storeRoles)); err != nil {
		return err
	}
	if _, err := s.sys.Exec(ctx, "ALTER ROLE "+s.prefix+"_authenticator PASSWORD '"+password+"'"); err != nil {
		return err
	}
	if err := pgx.BeginFunc(ctx, s.sys, func(tx pgx.Tx) error {
		err := generated.New(tx).AdvisoryXactLock(ctx, storeLock)
		if err == nil {
			_, err = tx.Exec(ctx, storeDropOld)
		}
		return err
	}); err != nil {
		return err
	}
	rev, _ := generated.New(s.sys).Revision(ctx)
	if strings.HasSuffix(rev, ":"+s.hash()) {
		return nil
	}
	p, err := s.Apply(ctx, nil, nil, false, false)
	if err != nil && p != nil && len(p.blocked()) > 0 {
		fmt.Fprintf(os.Stderr, "archive: archive.sql not applied at startup, it would delete data:\n  %s\nrun archive schema pull dir, then archive schema apply dir -allow-drop\n", strings.Join(p.blocked(), "\n  "))
		return nil
	}
	return err
}

const storeDropOld = `DO $$ BEGIN
    IF to_regclass('archive.source') IS NOT NULL THEN
        DELETE FROM archive.secret WHERE table_name = 'source';
        DROP TABLE archive.source;
    END IF;
    DROP TABLE IF EXISTS archive.schema_file;
    DROP TABLE IF EXISTS archive.view;
END $$`

func (s *pgStore) expand(sql string) string { return strings.ReplaceAll(sql, "{p}", s.prefix) }

func (s *pgStore) hash() string {
	h := sha256.Sum256([]byte(s.expand(storeSchema)))
	return hex.EncodeToString(h[:])
}

const storeMinTimeout = time.Minute

var storeFileRe = regexp.MustCompile(`^[a-z0-9_][a-z0-9_.-]*\.sql$`)

var (
	storeSubjectRe = regexp.MustCompile(`("(?:[^"]|"")+"|[a-z_][a-z0-9_$]*)\.("(?:[^"]|"")+"|[a-z_][a-z0-9_$]*)`)
	storeValidRe   = regexp.MustCompile(`^ALTER TABLE \S+ VALIDATE CONSTRAINT `)
)

func storeUnquote(id string) string {
	if strings.HasPrefix(id, `"`) {
		return strings.ReplaceAll(id[1:len(id)-1], `""`, `"`)
	}
	return id
}

func (s *pgStore) withTemp(ctx context.Context, fn func(f tempdb.Factory, tmp *sql.DB) error) error {
	f, err := tempdb.NewOnInstanceFactory(ctx, s.openDB, tempdb.WithRootDatabase(s.cfg.Database))
	if err != nil {
		return err
	}
	defer f.Close()
	tmp, err := f.Create(ctx)
	if err != nil {
		return err
	}
	defer tmp.Close(context.WithoutCancel(ctx))
	if _, err := tmp.ConnPool.ExecContext(ctx, s.expand(storeSchema)); err != nil {
		return fmt.Errorf("archive.sql: %w", err)
	}
	return fn(f, tmp.ConnPool)
}

func (s *pgStore) SchemaFiles(ctx context.Context) (map[string]string, error) {
	var plan diff.Plan
	if err := s.withTemp(ctx, func(f tempdb.Factory, tmp *sql.DB) (err error) {
		plan, err = diff.Generate(ctx, diff.DBSchemaSource(tmp), diff.DBSchemaSource(s.ddl), diff.WithTempDbFactory(f), diff.WithDoNotValidatePlan(),
			diff.WithIncludeSchemas("archive", "public"), diff.WithNoConcurrentIndexOps(), diff.WithDisableCheckFunctionBodies())
		return err
	}); err != nil {
		return nil, fmt.Errorf("schema: %w", err)
	}
	owner := ` OWNER TO ` + pgIdent(s.cfg.User)
	grant := regexp.MustCompile(`^GRANT (SELECT|INSERT|UPDATE|DELETE) ON \S+ TO ` + regexp.QuoteMeta(pgIdent(s.prefix+"_user")) + `$`)
	var ddl, subject []string
	tables, views := map[string]bool{}, map[string]bool{}
	for _, st := range plan.Statements {
		m := storeSubjectRe.FindStringSubmatch(st.DDL)
		if m == nil || storeUnquote(m[1]) != "public" || strings.HasSuffix(st.DDL, owner) || grant.MatchString(st.DDL) || storeValidRe.MatchString(st.DDL) {
			continue
		}
		name := storeUnquote(m[2])
		if strings.HasPrefix(st.DDL, "CREATE TABLE ") {
			tables[name] = true
		}
		if strings.HasPrefix(st.DDL, "CREATE VIEW ") {
			views[name] = true
		}
		ddl = append(ddl, strings.TrimSuffix(strings.ReplaceAll(strings.ReplaceAll(st.DDL, ` COLLATE "pg_catalog"."default"`, ""), " NOT VALID", ""), ";"))
		subject = append(subject, name)
	}
	file := map[string]string{}
	for name := range tables {
		file[name] = name
	}
	for name := range views {
		file[name] = name
	}
	for i, name := range subject {
		if file[name] != "" {
			continue
		}
		file[name] = name
		used := regexp.MustCompile(`(^|[^a-z0-9_$])` + regexp.QuoteMeta(name) + `($|[^a-z0-9_$])`)
		for j, d := range ddl {
			if j != i && tables[subject[j]] && used.MatchString(d) {
				file[name] = subject[j]
				break
			}
		}
	}
	out := map[string]string{}
	for i, d := range ddl {
		out[file[subject[i]]+".sql"] += d + ";\n"
	}
	return out, nil
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

func (s *pgStore) openDB(_ context.Context, name string) (*sql.DB, error) {
	c := s.cfg.Copy()
	c.Database = name
	c.RuntimeParams["statement_timeout"] = "0"
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
	if err := generated.New(storeSQL{conn}).AdvisoryLock(ctx, storeLock); err != nil {
		return nil, err
	}
	defer generated.New(storeSQL{conn}).AdvisoryUnlock(context.WithoutCancel(ctx), storeLock)
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	check := files != nil
	if files == nil {
		if files, err = s.SchemaFiles(ctx); err != nil {
			return nil, err
		}
	}
	if err := storeSet(ctx, tx, "check_function_bodies", "off"); err != nil {
		return nil, err
	}
	var src sqldb.Queryable = s.ddl
	if len(renames) > 0 {
		if err := storeSet(ctx, tx, "lock_timeout", "10s"); err != nil {
			return nil, err
		}
		for i, r := range renames {
			if _, err := tx.ExecContext(ctx, r); err != nil {
				return nil, fmt.Errorf("renames[%d]: %w", i, err)
			}
		}
		src = tx
	}
	p, err := s.plan(ctx, src, files, allowDrop, check)
	if err != nil || dryRun {
		return storeWithRenames(p, renames), err
	}
	if b := p.blocked(); len(b) > 0 {
		return storeWithRenames(p, renames), fmt.Errorf("the plan deletes data (%s); apply it with allow_drop: %w", strings.Join(b, "; "), ErrConflict)
	}
	for _, st := range p.Statements {
		if err := storeSet(ctx, tx, "statement_timeout", storeMS(max(st.timeout, storeMinTimeout)), "lock_timeout", storeMS(st.lock)); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, st.DDL); err != nil {
			return p, fmt.Errorf("schema: %s: %w", st.DDL, err)
		}
	}
	if err := storeSet(ctx, tx, "statement_timeout", "0"); err != nil {
		return nil, err
	}
	if err := generated.New(storeSQL{tx}).InsertRevision(ctx, s.hash()); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	p.Applied = true
	return storeWithRenames(p, renames), nil
}

func storeSet(ctx context.Context, tx *sql.Tx, kv ...string) error {
	for i := 0; i < len(kv); i += 2 {
		if err := generated.New(storeSQL{tx}).SetLocal(ctx, kv[i], kv[i+1]); err != nil {
			return err
		}
	}
	return nil
}

type storeSQL struct {
	x interface {
		ExecContext(context.Context, string, ...any) (sql.Result, error)
		QueryRowContext(context.Context, string, ...any) *sql.Row
	}
}

func (s storeSQL) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	_, err := s.x.ExecContext(ctx, query, args...)
	return pgconn.CommandTag{}, err
}

func (s storeSQL) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.ErrUnsupported
}

func (s storeSQL) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	return s.x.QueryRowContext(ctx, query, args...)
}

func storeMS(d time.Duration) string { return strconv.FormatInt(d.Milliseconds(), 10) }

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

func (s *pgStore) plan(ctx context.Context, src sqldb.Queryable, files map[string]string, allowDrop, check bool) (*SchemaPlan, error) {
	var plan diff.Plan
	if err := s.withTemp(ctx, func(f tempdb.Factory, tmp *sql.DB) error {
		if err := storeLoad(ctx, tmp, files); err != nil {
			return err
		}
		b, err := generated.New(storeSQL{tmp}).Catalog(ctx)
		if err != nil {
			return err
		}
		cat, err := catalogParse(b)
		if err != nil {
			return err
		}
		if err := cat.Check(); err != nil && check {
			return err
		}
		plan, err = diff.Generate(ctx, diff.DBSchemaSource(src), diff.DBSchemaSource(tmp), diff.WithTempDbFactory(f),
			diff.WithIncludeSchemas("archive", "public"), diff.WithNoConcurrentIndexOps(), diff.WithDisableCheckFunctionBodies())
		if err != nil {
			return fmt.Errorf("schema plan: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	out := &SchemaPlan{Statements: []SchemaStatement{}}
	for _, st := range plan.Statements {
		drop := slices.ContainsFunc(st.Hazards, func(h diff.MigrationHazard) bool { return h.Type == diff.MigrationHazardTypeDeletesData })
		out.Statements = append(out.Statements, SchemaStatement{DDL: st.DDL, Hazards: st.Hazards, Blocked: drop && !allowDrop, timeout: st.Timeout, lock: st.LockTimeout})
	}
	return out, nil
}

type storeUnit struct{ file, sql string }

func storeLoad(ctx context.Context, db *sql.DB, files map[string]string) error {
	var units []storeUnit
	for _, name := range specKeys(files) {
		units = append(units, storeUnit{name, files[name]})
	}
	left, errs, err := storeTry(ctx, db, units)
	if err != nil || len(left) == 0 {
		return err
	}
	var stmts []storeUnit
	for _, u := range left {
		for _, st := range storeSplit(u.sql) {
			stmts = append(stmts, storeUnit{u.file, st})
		}
	}
	if left, errs, err = storeTry(ctx, db, stmts); err != nil || len(left) == 0 {
		return err
	}
	ve := &ValidationError{Type: "schema"}
	for _, u := range left {
		if !slices.ContainsFunc(ve.Errors, func(e FieldError) bool { return e.Path == u.file }) {
			ve.Errors = append(ve.Errors, FieldError{Path: u.file, Message: errs[u.file].Error()})
		}
	}
	return ve
}

func storeTry(ctx context.Context, db *sql.DB, left []storeUnit) ([]storeUnit, map[string]error, error) {
	var errs map[string]error
	for progress := true; progress && len(left) > 0; {
		progress, errs = false, map[string]error{}
		var next []storeUnit
		for _, u := range left {
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				return nil, nil, err
			}
			if _, err = tx.ExecContext(ctx, u.sql); err == nil {
				err = tx.Commit()
			}
			if err != nil {
				tx.Rollback()
				if errs[u.file] == nil {
					errs[u.file] = err
				}
				next = append(next, u)
				continue
			}
			progress = true
		}
		left = next
	}
	return left, errs, nil
}

var (
	storeDollarRe = regexp.MustCompile(`^\$([A-Za-z_][A-Za-z0-9_]*)?\$`)
	storeIdentRe  = regexp.MustCompile(`[A-Za-z0-9_$]`)
)

func storeSplit(src string) []string {
	var out []string
	start := 0
	for i := 0; i < len(src); i++ {
		switch c := src[i]; {
		case c == '-' && strings.HasPrefix(src[i:], "--"):
			if n := strings.IndexByte(src[i:], '\n'); n >= 0 {
				i += n
			} else {
				i = len(src)
			}
		case c == '/' && strings.HasPrefix(src[i:], "/*"):
			for d := 0; i < len(src); i++ {
				if strings.HasPrefix(src[i:], "/*") {
					d, i = d+1, i+1
				} else if strings.HasPrefix(src[i:], "*/") {
					if d, i = d-1, i+1; d == 0 {
						break
					}
				}
			}
		case c == '\'' || c == '"':
			esc := c == '\'' && i > 0 && (src[i-1] == 'E' || src[i-1] == 'e')
			for i++; i < len(src) && src[i] != c; i++ {
				if esc && src[i] == '\\' {
					i++
				}
			}
		case c == '$':
			if m := storeDollarRe.FindString(src[i:]); m != "" && (i == 0 || !storeIdentRe.MatchString(src[i-1:i])) {
				if n := strings.Index(src[i+len(m):], m); n >= 0 {
					i += len(m) + n + len(m) - 1
				} else {
					i = len(src)
				}
			}
		case c == ';':
			if st := strings.TrimSpace(src[start:i]); st != "" {
				out = append(out, st)
			}
			start = i + 1
		}
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

func (s *pgStore) db(ctx context.Context) *generated.Queries { return generated.New(s.Q(ctx)) }

func (s *pgStore) As(ctx context.Context, role, space string, fn func(context.Context) error) error {
	if t, ok := ctx.Value(storeTxKey{}).(storeTx); ok && t.role == role {
		return fn(ctx)
	}
	after := &[]func(){}
	return storeCommitted(after, storeErr(pgx.BeginFunc(ctx, s.app, func(tx pgx.Tx) error {
		if err := generated.New(tx).SetLocal(ctx, "role", role); err != nil {
			return err
		}
		if space != "" {
			if err := generated.New(tx).SetLocal(ctx, "archive.space", space); err != nil {
				return err
			}
		}
		return fn(context.WithValue(ctx, storeTxKey{}, storeTx{tx, role, after}))
	})))
}

func (s *pgStore) System(ctx context.Context, fn func(context.Context) error) error {
	if t, ok := ctx.Value(storeTxKey{}).(storeTx); ok && t.role == "" {
		return fn(ctx)
	}
	after := &[]func(){}
	return storeCommitted(after, storeErr(pgx.BeginFunc(ctx, s.sys, func(tx pgx.Tx) error {
		return fn(context.WithValue(ctx, storeTxKey{}, storeTx{tx, "", after}))
	})))
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

const pgSocketMax = 103

func StartPostgres(ctx context.Context, dataDir string) (dsn string, stop func() error, err error) {
	dataDir, err = filepath.Abs(dataDir)
	if err != nil {
		return "", nil, err
	}
	pgDir := filepath.Join(dataDir, "pg")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return "", nil, err
	}

	if _, err := os.Stat(filepath.Join(pgDir, "PG_VERSION")); errors.Is(err, os.ErrNotExist) {
		initdb, err := pgBin(ctx, dataDir, "initdb")
		if err != nil {
			return "", nil, err
		}
		cmd := exec.CommandContext(ctx, initdb, "-D", pgDir, "-U", "archive",
			"--auth-local=trust", "--auth-host=reject", "-E", "UTF8", "--locale=C")
		cmd.Env = pgEnv()
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", nil, fmt.Errorf("initdb: %w\n%s", err, out)
		}
	}

	sockDir, err := pgSocketDir(dataDir)
	if err != nil {
		return "", nil, err
	}
	if err := pgClearStale(pgDir); err != nil {
		return "", nil, err
	}

	postgres, err := pgBin(ctx, dataDir, "postgres")
	if err != nil {
		return "", nil, err
	}
	logf, err := os.OpenFile(filepath.Join(dataDir, "postgres.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return "", nil, err
	}

	cmd := exec.Command(postgres, "-D", pgDir, "-c", "listen_addresses=", "-k", sockDir)
	cmd.Stdout, cmd.Stderr, cmd.Env = logf, logf, pgEnv()
	if err := cmd.Start(); err != nil {
		logf.Close()
		return "", nil, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait(); logf.Close() }()

	stop = func() error {
		_ = cmd.Process.Signal(syscall.SIGINT)
		select {
		case err := <-done:
			done <- err
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				return nil
			}
			return err
		case <-time.After(time.Minute):
			_ = cmd.Process.Kill()
			return errors.New("postgres did not stop within a minute; killed")
		}
	}

	host := pgQuote(sockDir)
	if err := pgWaitReady(ctx, "host="+host+" user=archive dbname=postgres", done, dataDir); err != nil {
		_ = stop()
		return "", nil, err
	}
	if err := pgEnsureDB(ctx, "host="+host+" user=archive dbname=postgres"); err != nil {
		_ = stop()
		return "", nil, err
	}
	return "host=" + host + " user=archive dbname=archive", stop, nil
}

func pgLive(dataDir string) string {
	b, err := os.ReadFile(filepath.Join(dataDir, "pg", "postmaster.pid"))
	if err != nil {
		return ""
	}
	lines := strings.Split(string(b), "\n")
	if len(lines) < 5 {
		return ""
	}
	pid, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	sock, port := strings.TrimSpace(lines[4]), strings.TrimSpace(lines[3])
	if err != nil || pid <= 0 || sock == "" || syscall.Kill(pid, 0) != nil || !pgIsPostgres(pid) {
		return ""
	}
	return "host=" + pgQuote(sock) + " port=" + port + " user=archive dbname=archive"
}

func pgEnv() []string { return append(os.Environ(), "LC_ALL=C") }

func pgBin(ctx context.Context, dataDir, name string) (string, error) {
	if dir := os.Getenv("ARCHIVE_PG_BIN"); dir != "" {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("%s not found in $ARCHIVE_PG_BIN: %w", name, err)
		}
		return p, nil
	}
	cached := filepath.Join(dataDir, "pgbin", pgVersion, "bin", name)
	if _, err := os.Stat(cached); err == nil {
		return cached, nil
	}
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	if err := pgDownload(ctx, filepath.Join(dataDir, "pgbin")); err != nil {
		return "", fmt.Errorf("%s not found on PATH or in $ARCHIVE_PG_BIN, and downloading Postgres failed: %w", name, err)
	}
	return cached, nil
}

const pgVersion = "18.6.0"

var pgSums = map[string]string{
	"darwin/arm64": "8f6ed6f38195b50a40712ada8b2a12060b6193c1a6977a6eb5cff053bb5a1b89",
	"darwin/amd64": "690941008aac9ee360820fde9bb7870a4f3dcac5e9e2f9e45e42333a436ad0f9",
	"linux/amd64":  "008ee189eaa7ca2b58bd01dd950291e638b998392cf982297acd4779ec898ac1",
	"linux/arm64":  "5f54e016723afa6213925a8acb939bac52944b1d348f65c7a20f0cd2928c31be",
}

func pgDownload(ctx context.Context, root string) error {
	sum, ok := pgSums[runtime.GOOS+"/"+runtime.GOARCH]
	if !ok {
		return fmt.Errorf("no Postgres build for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	artifact := "embedded-postgres-binaries-" + runtime.GOOS + "-" + map[string]string{"amd64": "amd64", "arm64": "arm64v8"}[runtime.GOARCH]
	url := "https://repo1.maven.org/maven2/io/zonky/test/postgres/" + artifact + "/" + pgVersion + "/" + artifact + "-" + pgVersion + ".jar"
	fmt.Fprintf(os.Stderr, "archive: no Postgres found; downloading Postgres %s into %s\n", pgVersion, root)

	jar, err := os.CreateTemp(root, "download-*.jar")
	if err != nil {
		return err
	}
	defer os.Remove(jar.Name())
	defer jar.Close()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(jar, h), resp.Body)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != sum {
		return fmt.Errorf("%s has sha256 %s, want %s", url, got, sum)
	}

	zr, err := zip.NewReader(jar, n)
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(root, "extract-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	for _, f := range zr.File {
		if !strings.HasSuffix(f.Name, ".txz") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		err = pgUntar(rc, tmp)
		rc.Close()
		if err != nil {
			return err
		}
		return os.Rename(tmp, filepath.Join(root, pgVersion))
	}
	return fmt.Errorf("%s holds no .txz", url)
}

func pgUntar(r io.Reader, dir string) error {
	xr, err := xz.NewReader(bufio.NewReader(r))
	if err != nil {
		return err
	}
	tr := tar.NewReader(xr)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if !filepath.IsLocal(h.Name) {
			return fmt.Errorf("unsafe path %q in the Postgres archive", h.Name)
		}
		p := filepath.Join(dir, h.Name)
		switch h.Typeflag {
		case tar.TypeDir:
			err = os.MkdirAll(p, 0o755)
		case tar.TypeSymlink:
			if !filepath.IsLocal(filepath.Join(filepath.Dir(h.Name), h.Linkname)) {
				return fmt.Errorf("unsafe link %q -> %q in the Postgres archive", h.Name, h.Linkname)
			}
			if err = os.MkdirAll(filepath.Dir(p), 0o755); err == nil {
				err = os.Symlink(h.Linkname, p)
			}
		case tar.TypeReg:
			err = pgWrite(p, tr, h.FileInfo().Mode().Perm())
		}
		if err != nil {
			return err
		}
	}
}

func pgWrite(p string, r io.Reader, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func pgSocketDir(dataDir string) (string, error) {
	const sock = "/.s.PGSQL.5432"
	if len(dataDir)+len(sock) <= pgSocketMax {
		return dataDir, nil
	}
	sum := sha256.Sum256([]byte(dataDir))
	name := "archive-pg-" + hex.EncodeToString(sum[:6])
	dir := filepath.Join(os.TempDir(), name)
	if len(dir)+len(sock) > pgSocketMax {
		dir = filepath.Join("/tmp", name)
	}
	return dir, os.MkdirAll(dir, 0o700)
}

func pgClearStale(pgDir string) error {
	pidFile := filepath.Join(pgDir, "postmaster.pid")
	b, err := os.ReadFile(pidFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	line, _, _ := bytes.Cut(b, []byte("\n"))
	pid, err := strconv.Atoi(strings.TrimSpace(string(line)))
	if err == nil && pid > 0 && syscall.Kill(pid, 0) == nil && pgIsPostgres(pid) {
		return fmt.Errorf("postgres (pid %d) is already running on %s", pid, pgDir)
	}
	return os.Remove(pidFile)
}

func pgIsPostgres(pid int) bool {
	if b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/comm"); err == nil {
		return strings.HasPrefix(string(b), "postgres")
	}
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=").Output()
	if err != nil {
		return len(out) > 0 || !errors.As(err, new(*exec.ExitError))
	}
	return strings.Contains(string(out), "postgres")
}

func pgWaitReady(ctx context.Context, dsn string, done chan error, dataDir string) error {
	deadline := time.Now().Add(time.Minute)
	for {
		c, err := pgx.Connect(ctx, dsn)
		if err == nil {
			return c.Close(ctx)
		}
		select {
		case err := <-done:
			done <- err
			return fmt.Errorf("postgres exited during startup (%v); see %s", err, filepath.Join(dataDir, "postgres.log"))
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("postgres not ready after a minute: %w", err)
		}
	}
}

func pgEnsureDB(ctx context.Context, dsn string) error {
	c, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return err
	}
	defer c.Close(ctx)
	q := generated.New(c)
	ok, err := q.DatabaseExists(ctx)
	if err == nil && !ok {
		err = q.CreateDatabase(ctx)
	}
	return err
}

func pgQuote(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'"
}

const backupKeep = 3

func BackupBeforeUpgrade(ctx context.Context, dataDir, version string) error {
	verFile := filepath.Join(dataDir, "version")
	old, err := os.ReadFile(verFile)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	oldVer := strings.TrimSpace(string(old))
	if oldVer == version {
		return nil
	}
	pgDir := filepath.Join(dataDir, "pg")
	if _, err := os.Stat(filepath.Join(pgDir, "PG_VERSION")); err == nil {
		if oldVer == "" {
			oldVer = "unknown"
		}
		dir := filepath.Join(dataDir, "backups")
		name := time.Now().UTC().Format("20060102T150405Z") + "-" + backupSafe(oldVer) + ".tar.gz"
		if err := backupTar(ctx, pgDir, dir, name); err != nil {
			return fmt.Errorf("backup before upgrade from %s: %w", oldVer, err)
		}
		if err := backupPrune(dir, backupKeep); err != nil {
			return err
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}
	tmp := verFile + ".tmp"
	if err := os.WriteFile(tmp, []byte(version+"\n"), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, verFile)
}

func backupTar(ctx context.Context, src, dir, name string) (err error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".partial-*")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			f.Close()
			os.Remove(f.Name())
		}
	}()
	zw := gzip.NewWriter(f)
	tw := tar.NewWriter(zw)
	err = filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		var link string
		switch {
		case info.Mode()&fs.ModeSymlink != 0:
			if link, err = os.Readlink(path); err != nil {
				return err
			}
		case !info.Mode().IsRegular() && !info.IsDir():
			return nil
		}
		hdr, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(filepath.Dir(src), path)
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if info.IsDir() {
			hdr.Name += "/"
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		r, err := os.Open(path)
		if err != nil {
			return err
		}
		defer r.Close()
		_, err = io.CopyN(tw, r, info.Size())
		return err
	})
	if err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(dir, name))
}

func backupPrune(dir string, keep int) error {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var names []string
	for _, e := range ents {
		if !e.IsDir() && !strings.HasPrefix(e.Name(), ".") && strings.HasSuffix(e.Name(), ".tar.gz") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for len(names) > keep {
		if err := os.Remove(filepath.Join(dir, names[0])); err != nil {
			return err
		}
		names = names[1:]
	}
	return nil
}

func backupSafe(v string) string {
	var b bytes.Buffer
	for _, r := range v {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_', r == '+':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}
