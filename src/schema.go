package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/Mulholland-Inc/archive/generated"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// SchemaMigration is a migration's report: its own statements, then what Archive added to the tables, and what
// they change and which of those changes lose data. Without confirm it is a dry run, which changes nothing;
// applying it takes its confirm. A named migration runs once; running it again skips it.
type SchemaMigration struct {
	Applied    bool           `json:"applied"`
	Confirm    string         `json:"confirm,omitempty"`
	Revision   *uuid.UUID     `json:"revision,omitempty"`
	Name       string         `json:"name,omitempty"`
	Statements []string       `json:"statements"`
	Changes    []SchemaChange `json:"changes"`
	Drops      []SchemaChange `json:"drops"`
	Skipped    bool           `json:"skipped,omitempty"`
}

// schemaTx is the schema state of one transaction: once it migrated, its catalog is read from it, cached per
// archive.schema setting, which a rolled back savepoint undoes along with the migration.
type schemaTx struct {
	n    int
	cats map[string]*Catalog
}

type SchemaRevision struct {
	ID         uuid.UUID       `json:"id"`
	At         string          `json:"at"`
	Author     string          `json:"author"`
	Name       string          `json:"name,omitempty"`
	Statements json.RawMessage `json:"statements"`
}

// Migrate runs src, DDL on the public schema, in a savepoint of the transaction in ctx, adds to each table what
// every table has, checks the result and reports what it changes. Without confirm it rolls back; with the
// report's confirm it keeps the migration, which commits or rolls back with the transaction.
func (s *pgStore) Migrate(ctx context.Context, src, name, author, confirm string) (*SchemaMigration, error) {
	stmts := storeSplit(src)
	if err := schemaCheckSQL(stmts); err != nil {
		return nil, err
	}
	t, ok := ctx.Value(storeTxKey{}).(storeTx)
	if !ok {
		return nil, errors.New("schema: no transaction")
	}
	out := &SchemaMigration{Name: name, Statements: []string{}}
	if name != "" {
		done, err := generated.New(s.sys).RevisionNamed(ctx, &name)
		if err != nil || done {
			out.Skipped = done
			return out, err
		}
	}
	pid, err := generated.New(t.tx).BackendPID(ctx)
	if err != nil {
		return nil, err
	}
	b := make([]byte, 32)
	rand.Read(b)
	key := hex.EncodeToString(b)
	if err := generated.New(s.sys).AddDDLKey(ctx, key, pid); err != nil {
		return nil, err
	}
	defer generated.New(s.sys).DeleteDDLKey(context.WithoutCancel(ctx), key)
	sp, err := t.tx.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer sp.Rollback(context.WithoutCancel(ctx))
	q := generated.New(sp)
	if err := q.AdvisoryXactLock(ctx, storeLock); err != nil {
		return nil, err
	}
	restore, err := schemaSettings(ctx, sp, "lock_timeout", "search_path")
	if err != nil {
		return nil, err
	}
	if err := q.SetLocal(ctx, "lock_timeout", "10s"); err != nil {
		return nil, err
	}
	if err := q.SetLocal(ctx, "search_path", "public"); err != nil {
		return nil, err
	}
	before, err := pgRead(ctx, sp, "archive", "public")
	if err != nil {
		return nil, err
	}
	h := sha256.Sum256([]byte(strings.Join(stmts, ";\n") + "\x00" + s.hash() + "\x00" + before.hash()))
	if out.Confirm = hex.EncodeToString(h[:]); confirm != "" && confirm != out.Confirm {
		return nil, fmt.Errorf("the schema or this migration changed since its dry run; dry-run it again for a new confirm: %w", ErrConflict)
	}
	run := func(what, st string) error {
		if err := q.DDL(ctx, key, st); err != nil {
			if len(st) > 80 {
				st = st[:77] + "..."
			}
			return fmt.Errorf("%s (%s): %w", what, st, err)
		}
		out.Statements = append(out.Statements, st)
		return nil
	}
	cat, err := schemaCatalog(ctx, sp)
	if err != nil {
		return nil, err
	}
	derived, err := schemaDerived(ctx, sp, cat)
	if err != nil {
		return nil, err
	}
	for _, st := range schemaSearchClear(stmts, derived) {
		if err := q.DDL(ctx, key, st); err != nil {
			return nil, err
		}
	}
	derive := func(cols bool) (*Catalog, error) {
		cat, err := schemaCatalog(ctx, sp)
		if err != nil {
			return nil, err
		}
		add, err := schemaDerive(ctx, sp, cat, derived, s.prefix, cols)
		if err != nil || len(add) == 0 {
			return cat, err
		}
		for _, st := range add {
			if err := run("adding what every table has", st); err != nil {
				return nil, err
			}
		}
		return schemaCatalog(ctx, sp)
	}
	for i, st := range stmts {
		if err := run(fmt.Sprintf("statement %d", i+1), st); err != nil {
			return nil, err
		}
		// A view or a foreign key after it may use the columns every table has.
		if schemaCreateTableRe.MatchString(schemaBare(st)) {
			if _, err := derive(true); err != nil {
				return nil, err
			}
		}
	}
	if cat, err = derive(false); err != nil {
		return nil, err
	}
	after, err := pgRead(ctx, sp, "archive", "public")
	if err != nil {
		return nil, err
	}
	if ours, _ := pgCompare(before.only("archive"), after.only("archive")); len(ours) > 0 {
		return nil, fmt.Errorf("a migration changes only your own tables, views and types, not Archive's (%s %s): %w", ours[0].Kind, ours[0].Name, ErrForbidden)
	}
	if err := cat.Check(); err != nil {
		return nil, err
	}
	out.Changes, out.Drops = pgCompare(before.only("public"), after.only("public"))
	if confirm == "" {
		return out, nil
	}
	sj, _ := json.Marshal(out.Statements)
	id, err := q.DDLRevision(ctx, generated.DDLRevisionParams{Key: key, Hash: s.hash(), Author: author, Name: name, Statements: sj})
	if err != nil {
		return nil, err
	}
	out.Applied, out.Revision = true, &id
	t.sch.n++
	if err := q.SetLocal(ctx, "archive.schema", strconv.Itoa(t.sch.n)); err != nil {
		return nil, err
	}
	if err := restore(); err != nil {
		return nil, err
	}
	return out, sp.Commit(ctx)
}

var (
	schemaStmtRe   = regexp.MustCompile(`(?is)^(CREATE\s+(OR\s+REPLACE\s+)?(UNIQUE\s+)?(TABLE|VIEW|INDEX|TYPE|DOMAIN|FUNCTION|TRIGGER|SEQUENCE|POLICY)|(ALTER|DROP)\s+(TABLE|VIEW|INDEX|TYPE|DOMAIN|FUNCTION|TRIGGER|SEQUENCE|POLICY)|COMMENT\s+ON)\s`)
	schemaBannedRe = regexp.MustCompile(`(?is)\bSECURITY\s+DEFINER\b|\bOWNER\s+TO\b|\bSET\s+SCHEMA\b|^CREATE\s+TABLE\s+[^(]*\bAS\s`)
	schemaLangRe   = regexp.MustCompile(`(?i)\bLANGUAGE\s+'?([a-z_]+)`)
)

// schemaCheckSQL allows only DDL on the kinds of object a workspace has. Rows change through archive.create,
// update and delete, which run as the user, so row-level security applies.
func schemaCheckSQL(stmts []string) error {
	ve := &ValidationError{Type: "schema"}
	if len(stmts) == 0 {
		ve.Errors = append(ve.Errors, FieldError{Path: "sql", Message: "want DDL statements, like ALTER TABLE house ADD COLUMN rooms integer"})
	}
	for i, st := range stmts {
		st = schemaBare(st)
		path := fmt.Sprintf("sql[%d]", i)
		switch {
		case !schemaStmtRe.MatchString(st):
			ve.Errors = append(ve.Errors, FieldError{Path: path, Message: "want CREATE, ALTER or DROP of a TABLE, VIEW, INDEX, TYPE, DOMAIN, FUNCTION, TRIGGER, SEQUENCE or POLICY, or COMMENT ON; change rows with archive.create, archive.update and archive.delete between migrations"})
		case strings.HasPrefix(strings.ToUpper(st), "COMMENT"):
		case schemaBannedRe.MatchString(st):
			ve.Errors = append(ve.Errors, FieldError{Path: path, Message: "SECURITY DEFINER, OWNER TO, SET SCHEMA and CREATE TABLE ... AS are not allowed"})
		default:
			if m := schemaLangRe.FindStringSubmatch(st); m != nil && !strings.EqualFold(m[1], "sql") && !strings.EqualFold(m[1], "plpgsql") {
				ve.Errors = append(ve.Errors, FieldError{Path: path, Message: "functions are LANGUAGE sql or plpgsql"})
			}
		}
	}
	if len(ve.Errors) > 0 {
		return ve
	}
	return nil
}

var (
	schemaCreateTableRe = regexp.MustCompile(`(?is)^CREATE\s+TABLE\s`)
	schemaAlterRe       = regexp.MustCompile(`(?is)^ALTER\s+TABLE\s+(?:IF\s+EXISTS\s+)?(?:ONLY\s+)?(?:(?:public|"public")\.)?("(?:[^"]|"")+"|[a-z_][a-z0-9_$]*)\s(.*)$`)
	schemaRewriteRe     = regexp.MustCompile(`(?is)\bDROP\b|\bTYPE\b`)
	schemaSearchRe      = regexp.MustCompile(`^archive\.search_doc\(VARIADIC (?:ARRAY\[(.*)\]|'\{\}'::text\[\])\)$`)
	schemaItemRe        = regexp.MustCompile(`^\(?"?([a-z_][a-z0-9_]*)"?\)?(::text)?$`)
)

// schemaSearch is the derived search expression of each public table that has one, and the columns it covers.
func schemaSearch(ctx context.Context, tx pgx.Tx) (map[string][]string, error) {
	rows, err := tx.Query(ctx, `SELECT c.relname::text, pg_catalog.pg_get_expr(d.adbin, d.adrelid) FROM pg_catalog.pg_attrdef d
        JOIN pg_catalog.pg_attribute a ON a.attrelid = d.adrelid AND a.attnum = d.adnum JOIN pg_catalog.pg_class c ON c.oid = d.adrelid
        WHERE c.relnamespace = 'public'::regnamespace AND a.attname = 'search' AND a.attgenerated = 's'`)
	if err != nil {
		return nil, err
	}
	out := map[string][]string{}
	var table, expr string
	_, err = pgx.ForEachRow(rows, []any{&table, &expr}, func() error {
		m := schemaSearchRe.FindStringSubmatch(expr)
		if m == nil {
			return nil
		}
		cols := []string{}
		if m[1] != "" {
			for _, it := range strings.Split(m[1], ", ") {
				im := schemaItemRe.FindStringSubmatch(it)
				if im == nil {
					return nil
				}
				cols = append(cols, im[1])
			}
		}
		slices.Sort(cols)
		out[table] = cols
		return nil
	})
	return out, err
}

// schemaDerived are the tables whose search is the one schemaDerive adds, over their text columns.
func schemaDerived(ctx context.Context, tx pgx.Tx, cat *Catalog) (map[string]bool, error) {
	search, err := schemaSearch(ctx, tx)
	out := map[string]bool{}
	for name, cols := range search {
		if t := cat.Tables[name]; t != nil && t.Schema == "public" && slices.Equal(cols, schemaSearchCols(cat, t)) {
			out[name] = true
		}
	}
	return out, err
}

// schemaSearchClear empties the derived search of each table whose columns stmts drop or retype, since
// Postgres keeps a column a generated column uses; schemaDerive sets it again after.
func schemaSearchClear(stmts []string, derived map[string]bool) []string {
	var out []string
	for _, st := range stmts {
		m := schemaAlterRe.FindStringSubmatch(schemaBare(st))
		if m == nil || !schemaRewriteRe.MatchString(m[2]) {
			continue
		}
		name := m[1]
		if strings.HasPrefix(name, `"`) {
			name = storeUnquote(name)
		} else {
			name = strings.ToLower(name)
		}
		if derived[name] {
			out = append(out, schemaSetSearch(name, nil))
		}
	}
	return slices.Compact(out)
}

var schemaLeadRe = regexp.MustCompile(`^(\s+|--[^\n]*(\n|$)|/\*(?s:.*?)\*/)+`)

// schemaBare is st without the comments and space before it.
func schemaBare(st string) string { return schemaLeadRe.ReplaceAllString(st, "") }

func schemaSetSearch(table string, cols []string) string {
	return "ALTER TABLE " + pgIdent("public", table) + " ALTER COLUMN search SET EXPRESSION AS (archive.search_doc(VARIADIC " + schemaSearchArray(cols) + "))"
}

func schemaSearchArray(cols []string) string {
	if len(cols) == 0 {
		return "'{}'::text[]"
	}
	parts := make([]string, len(cols))
	for i, c := range cols {
		parts[i] = pgIdent(c) + "::text"
	}
	return "ARRAY[" + strings.Join(parts, ", ") + "]"
}

// schemaSearchCols are the columns a derived search column covers: the text ones that are not secret, by name.
func schemaSearchCols(cat *Catalog, t *Table) []string {
	out := []string{}
	for _, c := range t.Columns {
		if c.List || c.Generated || c.Name == "access" || c.Name == "search" || !cat.texty(c.Schema, c.Type) || cat.secret(c) != nil {
			continue
		}
		out = append(out, c.Name)
	}
	slices.Sort(out)
	return out
}

func (c *Catalog) texty(schema, name string) bool {
	for range 8 {
		if schema == "pg_catalog" {
			return name == "text" || name == "varchar" || name == "bpchar"
		}
		t := c.typ(schema, name)
		if t == nil || t.Kind != "d" {
			return false
		}
		schema, name = t.BaseSchema, t.Base
	}
	return false
}

// schemaDerive is what each public table lacks of what every table has: the id, access, created_at,
// updated_at and search columns, the search index, row-level security with the access policy, and the
// grants to the viewer and editor roles; and a derived search that no longer covers its text columns.
// With cols, only the columns.
func schemaDerive(ctx context.Context, tx pgx.Tx, cat *Catalog, derived map[string]bool, prefix string, cols bool) ([]string, error) {
	policed := map[string]bool{}
	rows, err := tx.Query(ctx, `SELECT c.relname::text FROM pg_catalog.pg_class c WHERE c.relnamespace = 'public'::regnamespace
        AND c.relkind IN ('r', 'p') AND EXISTS (SELECT FROM pg_catalog.pg_policy p WHERE p.polrelid = c.oid)`)
	if err != nil {
		return nil, err
	}
	var name string
	if _, err := pgx.ForEachRow(rows, []any{&name}, func() error { policed[name] = true; return nil }); err != nil {
		return nil, err
	}
	search, err := schemaSearch(ctx, tx)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, name := range slices.Sorted(maps.Keys(cat.Tables)) {
		t := cat.Tables[name]
		if t.Schema != "public" || t.Kind != "table" {
			continue
		}
		id := pgIdent("public", name)
		var add, after []string
		if t.col("id") == nil {
			add = append(add, "ADD COLUMN id uuid NOT NULL DEFAULT uuidv7()", "ADD CONSTRAINT "+pgIdent(name+"_pkey")+" PRIMARY KEY (id)")
		}
		if t.col("access") == nil {
			add = append(add, "ADD COLUMN access text")
		}
		for _, c := range []string{"created_at", "updated_at"} {
			if t.col(c) == nil {
				add = append(add, "ADD COLUMN "+c+" timestamptz NOT NULL DEFAULT now()")
			}
		}
		want := schemaSearchCols(cat, t)
		if t.col("search") == nil {
			add = append(add, "ADD COLUMN search tsvector GENERATED ALWAYS AS (archive.search_doc(VARIADIC "+schemaSearchArray(want)+")) STORED")
			after = append(after, "CREATE INDEX "+pgIdent(name+"_search")+" ON "+id+" USING gin (search)")
			derived[name] = true
		} else if got, ok := search[name]; ok && derived[name] && !slices.Equal(got, want) {
			after = append(after, schemaSetSearch(name, want))
		}
		if len(add) > 0 {
			out = append(out, "ALTER TABLE "+id+" "+strings.Join(add, ", "))
		}
		out = append(out, after...)
		if cols {
			continue
		}
		if !t.RLS {
			out = append(out, "ALTER TABLE "+id+" ENABLE ROW LEVEL SECURITY")
		}
		if !t.Force {
			out = append(out, "ALTER TABLE "+id+" FORCE ROW LEVEL SECURITY")
		}
		if !policed[name] {
			out = append(out, "CREATE POLICY access ON "+id+" USING (archive.can(access)) WITH CHECK (archive.can(access))")
		}
	}
	if cols {
		return out, nil
	}
	rows, err = tx.Query(ctx, `SELECT c.relname::text, NOT has_table_privilege($1, c.oid, 'SELECT'), NOT has_table_privilege($2, c.oid, 'INSERT')
        FROM pg_catalog.pg_class c WHERE c.relnamespace = 'public'::regnamespace AND c.relkind IN ('r', 'p', 'v') ORDER BY 1`, prefix+"_viewer", prefix+"_editor")
	if err != nil {
		return nil, err
	}
	var read, write bool
	_, err = pgx.ForEachRow(rows, []any{&name, &read, &write}, func() error {
		if read {
			out = append(out, "GRANT SELECT ON "+pgIdent("public", name)+" TO "+pgIdent(prefix+"_viewer"))
		}
		if write {
			out = append(out, "GRANT INSERT, UPDATE, DELETE ON "+pgIdent("public", name)+" TO "+pgIdent(prefix+"_editor"))
		}
		return nil
	})
	return out, err
}

// schemaSettings reads the settings, for the returned func to set them back.
func schemaSettings(ctx context.Context, tx pgx.Tx, names ...string) (func() error, error) {
	vals := make([]string, len(names))
	for i, n := range names {
		if err := tx.QueryRow(ctx, "SELECT current_setting($1)", n).Scan(&vals[i]); err != nil {
			return nil, err
		}
	}
	return func() error {
		for i, n := range names {
			if err := generated.New(tx).SetLocal(ctx, n, vals[i]); err != nil {
				return err
			}
		}
		return nil
	}, nil
}

// txCatalog is the catalog of the transaction in ctx when it migrated, otherwise nil.
func (s *pgStore) txCatalog(ctx context.Context) *Catalog {
	t, ok := ctx.Value(storeTxKey{}).(storeTx)
	if !ok || t.sch == nil || t.sch.n == 0 {
		return nil
	}
	var gen string
	if err := t.tx.QueryRow(ctx, "SELECT current_setting('archive.schema', true)").Scan(&gen); err != nil || gen == "" {
		return nil
	}
	if c := t.sch.cats[gen]; c != nil {
		return c
	}
	c, err := schemaCatalog(ctx, t.tx)
	if err != nil {
		return nil
	}
	if t.sch.cats == nil {
		t.sch.cats = map[string]*Catalog{}
	}
	t.sch.cats[gen] = c
	return c
}

func schemaCatalog(ctx context.Context, q generated.DBTX) (*Catalog, error) {
	b, err := generated.New(q).Catalog(ctx)
	if err != nil {
		return nil, err
	}
	return catalogParse(b)
}

// Schema is the DDL of the public schema as the transaction in ctx sees it, by table or view: each with its
// constraints, indexes, policies, triggers and comments, and the types, functions and sequences only it uses.
func (s *pgStore) Schema(ctx context.Context) (map[string]string, error) {
	var b interface {
		Begin(context.Context) (pgx.Tx, error)
	} = s.sys
	if t, ok := ctx.Value(storeTxKey{}).(storeTx); ok {
		b = t.tx
	}
	tx, err := b.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	if err := generated.New(tx).SetLocal(ctx, "search_path", "public"); err != nil {
		return nil, err
	}
	sch, err := pgRead(ctx, tx, "public")
	if err != nil {
		return nil, err
	}
	return sch.ddl("public"), nil
}

// Revisions are the migrations, newest first.
func (s *pgStore) Revisions(ctx context.Context, limit int) ([]SchemaRevision, error) {
	rows, err := generated.New(s.sys).Revisions(ctx, int32(limit))
	out := []SchemaRevision{}
	for _, r := range rows {
		out = append(out, SchemaRevision{ID: r.ID, At: r.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"), Author: r.Author, Name: r.Name, Statements: r.Statements})
	}
	return out, err
}

// migrate brings the archive schema to archive.sql at startup: it applies the file to a scratch database and
// runs what makes the live schema the same. It never deletes data: when it would, it says so and leaves the
// schema as it is.
func (s *pgStore) migrate(ctx context.Context) error {
	conn, err := s.sys.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	q := generated.New(conn)
	if err := q.AdvisoryLock(ctx, storeLock); err != nil {
		return err
	}
	defer q.AdvisoryUnlock(context.WithoutCancel(ctx), storeLock)
	want, err := s.scratch(ctx, s.expand(storeSchema))
	if err != nil {
		return fmt.Errorf("archive.sql: %w", err)
	}
	return pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		lost, err := pgMigrate(ctx, tx, want, false)
		if err != nil {
			return fmt.Errorf("archive.sql: %w", err)
		}
		if len(lost) > 0 {
			fmt.Fprintf(os.Stderr, "archive: archive.sql not applied at startup, it would delete data:\n  %s\n", strings.Join(lost, "\n  "))
			return nil
		}
		return generated.New(tx).InsertRevision(ctx, s.hash())
	})
}

// scratch is the snapshot of Archive's schema that sql makes in a new database, which it drops after.
func (s *pgStore) scratch(ctx context.Context, sql string) (pgSchema, error) {
	b := make([]byte, 8)
	rand.Read(b)
	name := "archive_scratch_" + hex.EncodeToString(b)
	if _, err := s.sys.Exec(ctx, "CREATE DATABASE "+pgIdent(name)); err != nil {
		return nil, err
	}
	defer s.sys.Exec(context.WithoutCancel(ctx), "DROP DATABASE IF EXISTS "+pgIdent(name)+" WITH (FORCE)")
	cfg := s.cfg.Copy()
	cfg.Database = name
	c, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	defer c.Close(context.WithoutCancel(ctx))
	var sch pgSchema
	return sch, pgx.BeginFunc(ctx, c, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SET LOCAL check_function_bodies = off"); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, sql); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "SET LOCAL search_path = ''"); err != nil {
			return err
		}
		sch, err = pgRead(ctx, tx, "archive", "public")
		sch = sch.ours()
		return err
	})
}

// pgMigrate runs in tx what brings Archive's schema to want, unless that would lose data: then it returns
// what and runs nothing. With drop, which only tests use, it runs it all.
func pgMigrate(ctx context.Context, tx pgx.Tx, want pgSchema, drop bool) ([]string, error) {
	for _, st := range []string{"SET LOCAL search_path = ''", "SET LOCAL check_function_bodies = off", "SET LOCAL lock_timeout = '1min'", "SET LOCAL statement_timeout = 0"} {
		if _, err := tx.Exec(ctx, st); err != nil {
			return nil, err
		}
	}
	live, err := pgRead(ctx, tx, "archive", "public")
	if err != nil {
		return nil, err
	}
	stmts, lost, err := pgPlan(live.ours(), want, drop)
	if err != nil || len(lost) > 0 && !drop {
		return lost, err
	}
	for _, st := range stmts {
		if _, err := tx.Exec(ctx, st); err != nil {
			return nil, fmt.Errorf("%s: %w", st, err)
		}
	}
	return lost, nil
}
