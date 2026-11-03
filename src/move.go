package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Moving the first workspace out of the main database. Before the main database held only what every workspace
// shares, the first workspace lived in it too: its tables, views, functions and rows were next to the main
// tables, all from one archive.sql. moveFirst moves them into archive_<id>, a database of its own like every
// other workspace's, once, at startup, before any workspace opens.
//
// Postgres moves nothing between databases, and Cloud SQL gives no superuser and no pg_dump, so Go copies it:
// in one transaction on the main database that holds every table of the workspace locked, so nothing writes
// them while it runs, it makes the workspace's schema again in the new database from what Postgres says it is
// (pgRead), object by object and column by column in the same order, copies every row with COPY, and sets
// every sequence where it was. Then it checks the copy: every object of both, as Postgres renders it, is the
// same (moveFingerprint), and every table has the same rows (moveRows). Only then, in the same transaction, it
// drops the workspace from the main database, brings that to main.sql and makes the workspace's owners its
// admins. Until that commits, the main database is as it
// was: a failure, or a process killed, rolls it back, and the next start drops the half-made copy and begins
// again. A start after it committed finds no workspace in the main database and does nothing.
//
// Nothing here ever drops a workspace that is real. The copy says it is half made, by a schema archive_move of
// its own, from the moment it is created until the main database has committed the move; a database archive_<id>
// that exists without it and is not empty is never dropped, and stops the move, so workspace tables that come
// back into the main database after it moved, as an older Archive started against it could bring them, stop
// the start and drop nothing.
//
// The workspace's roles are the cluster's, not a database's, so they stay as they are: its grants and policies
// name them in the copy as they did. Its files are kept by the workspace's id (blobPrefixed), not its database,
// so they stay where they are too.

const (
	moveTries       = 5
	moveLockTimeout = "20s"
)

// moveFirst moves the first workspace out of the main database, if it is still there, trying again a few times
// while others hold its tables, as a revision that is still serving does during a deploy.
func (h *Hub) moveFirst(ctx context.Context, conn *pgx.Conn) error {
	for try := 1; ; try++ {
		var there bool
		if err := conn.QueryRow(ctx, "SELECT to_regclass('archive.member') IS NOT NULL").Scan(&there); err != nil || !there {
			return err
		}
		err := h.moveOnce(ctx, conn)
		if err == nil {
			return nil
		}
		// A commit whose answer was lost may have gone through.
		if conn.QueryRow(ctx, "SELECT to_regclass('archive.member') IS NOT NULL").Scan(&there) == nil && !there {
			return nil
		}
		fmt.Fprintf(os.Stderr, "archive: moving the first workspace out of the main database failed, and the main database is as it was: %v\n", err)
		var pe *pgconn.PgError
		if try == moveTries || ctx.Err() != nil || !errors.As(err, &pe) || pe.Code != "55P03" && pe.Code != "40P01" {
			return fmt.Errorf("moving the first workspace out of the main database: %w", err)
		}
		select {
		case <-time.After(time.Duration(try) * 2 * time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// moveMark is the schema that says a workspace's database is a copy still being made (moveOnce).
const moveMark = "archive_move"

// moveLeftover says whether the database db, if there is one, may be dropped to make the copy again: it may
// when it is a copy a failed move left, which says so (moveMark), or holds nothing at all, as one made just
// before a move stopped is. Any other is a workspace, which is never dropped.
func (h *Hub) moveLeftover(ctx context.Context, db string) error {
	var exists bool
	if err := h.main.QueryRow(ctx, "SELECT EXISTS (SELECT FROM pg_catalog.pg_database WHERE datname = $1)", db).Scan(&exists); err != nil || !exists {
		return err
	}
	cfg := h.pg.Copy()
	cfg.Database = db
	c, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("%s is there already, and Archive cannot look inside: %w", db, err)
	}
	defer c.Close(context.WithoutCancel(ctx))
	var marked bool
	var objects int
	if err := c.QueryRow(ctx, `SELECT to_regnamespace($1) IS NOT NULL, (SELECT count(*) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname IN ('archive', 'public'))`, moveMark).Scan(&marked, &objects); err != nil {
		return err
	}
	if !marked && objects > 0 {
		return fmt.Errorf("%s is there already, with %d tables and other objects, and is not a copy a failed move left: Archive drops nothing; "+
			"look at it, and at why the main database still has the workspace", db, objects)
	}
	return nil
}

// moveRel is a table of the workspace, with its columns in their order.
type moveRel struct {
	oid             uint32
	schema, name    string
	kind, persist   string
	partition       bool
	parents         []uint32
	children        []*moveRel
	def             string // what follows its columns in CREATE TABLE, as pgRead has it
	cols            []moveCol
	rls, force      bool
	rows, generated []string
}

type moveCol struct {
	name, def, dflt string
	typ, coll       string // its type, and COLLATE and its collation when that is not the type's
	expr, gen       string // a generated column's expression and kind, s or v
	notNull         bool
	special         bool // generated or an identity
	generated       bool
	local           bool
	inherited       int
}

// add is the definition a column is added with to a table being made one column at a time. A generated column
// starts as nothing, since what it is made of may come after it, and is set once every column is there (set).
func (c moveCol) add() string {
	if !c.generated {
		return c.def
	}
	return strings.TrimSpace(c.typ+" "+c.coll) + " GENERATED ALWAYS AS (NULL::" + c.typ + ") " + map[string]string{"s": "STORED", "v": "VIRTUAL"}[c.gen]
}

func (c moveCol) set(table string) []string {
	if !c.generated {
		return nil
	}
	out := []string{"ALTER TABLE ONLY " + table + " ALTER COLUMN " + pgIdent(c.name) + " SET EXPRESSION AS (" + c.expr + ")"}
	if c.notNull {
		out = append(out, "ALTER TABLE ONLY "+table+" ALTER COLUMN "+pgIdent(c.name)+" SET NOT NULL")
	}
	return out
}

func (r *moveRel) ident() string { return pgIdent(r.schema, r.name) }

func (r *moveRel) col(name string) *moveCol {
	for i := range r.cols {
		if r.cols[i].name == name {
			return &r.cols[i]
		}
	}
	return nil
}

func (h *Hub) moveOnce(ctx context.Context, conn *pgx.Conn) (err error) {
	bg := context.WithoutCancel(ctx)
	var ws uuid.UUID
	if err := conn.QueryRow(ctx, "SELECT id FROM archive.workspace ORDER BY created_at, id LIMIT 1").Scan(&ws); err != nil {
		return fmt.Errorf("the first workspace: %w", err)
	}
	id := ws.String()
	db := h.database(id)
	want, err := pgScratch(ctx, h.main, h.pg, mainSchema)
	if err != nil {
		return fmt.Errorf("main.sql: %w", err)
	}
	start := time.Now()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(bg)
	made := false
	defer func() {
		// The copy goes with a failure, unless the main database has moved on without the workspace, as it does
		// when a commit went through but its answer was lost.
		var there bool
		if err == nil || !made || h.main.QueryRow(bg, "SELECT to_regclass('archive.member') IS NOT NULL").Scan(&there) != nil || !there {
			return
		}
		if h.moveLeftover(bg, db) == nil {
			h.main.Exec(bg, "DROP DATABASE IF EXISTS "+pgIdent(db)+" WITH (FORCE)")
		}
	}()
	if err := moveSettings(ctx, tx); err != nil {
		return err
	}
	for _, st := range []string{"SET LOCAL lock_timeout = '" + moveLockTimeout + "'", "SET LOCAL statement_timeout = 0", "SELECT pg_advisory_xact_lock(" + fmt.Sprint(storeLock) + ")"} {
		if _, err := tx.Exec(ctx, st); err != nil {
			return err
		}
	}
	rels, err := moveRels(ctx, tx)
	if err != nil {
		return err
	}
	var locks []string
	for _, r := range rels {
		locks = append(locks, r.ident())
	}
	if len(locks) > 0 {
		if _, err := tx.Exec(ctx, "LOCK TABLE "+strings.Join(locks, ", ")+" IN ACCESS EXCLUSIVE MODE"); err != nil {
			return fmt.Errorf("locking the workspace's tables: %w", err)
		}
	}
	src, err := pgRead(ctx, tx, "archive", "public")
	if err != nil {
		return err
	}
	src = moveOwn(src)
	before, err := moveFingerprint(ctx, tx)
	if err != nil {
		return err
	}
	mainRows, err := moveMainRows(ctx, tx)
	if err != nil {
		return err
	}
	// Every row is read, past the row-level security the tables force on their owner too.
	if err := moveNoForce(ctx, tx, rels); err != nil {
		return err
	}

	// The copy, made again from nothing: a copy a failed move left is dropped first, and anything else stops it.
	if err := h.moveLeftover(ctx, db); err != nil {
		return err
	}
	if _, err := h.main.Exec(ctx, "DROP DATABASE IF EXISTS "+pgIdent(db)+" WITH (FORCE)"); err != nil {
		return err
	}
	if _, err := h.main.Exec(ctx, "CREATE DATABASE "+pgIdent(db)); err != nil {
		return err
	}
	made = true
	cfg := h.pg.Copy()
	cfg.Database = db
	t, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return err
	}
	defer t.Close(bg)
	// It says it is half made, committed before anything else goes in, until the main database has moved on.
	if _, err := t.Exec(ctx, "CREATE SCHEMA "+moveMark+"; COMMENT ON SCHEMA "+moveMark+" IS "+
		pgLiteral("A copy of workspace "+id+" that Archive is still making; it may drop it")); err != nil {
		return err
	}
	if err := moveDatabase(ctx, tx, h, db, id); err != nil {
		return err
	}
	rows, err := moveCopy(ctx, tx, t, src, rels)
	if err != nil {
		return err
	}

	// The check: the copy is the workspace, object for object and row for row.
	var after map[string]string
	if err := pgx.BeginFunc(ctx, t, func(tt pgx.Tx) error {
		if err := moveSettings(ctx, tt); err != nil {
			return err
		}
		after, err = moveFingerprint(ctx, tt)
		return err
	}); err != nil {
		return err
	}
	if diff := moveDiff(before, after); len(diff) > 0 {
		return fmt.Errorf("the copy in %s differs from the workspace:\n  %s", db, strings.Join(diff, "\n  "))
	}
	had, err := moveRows(ctx, tx, rels)
	if err != nil {
		return err
	}
	has, err := moveCheckRows(ctx, t, rels)
	if err != nil {
		return err
	}
	for _, k := range slices.Sorted(maps.Keys(had)) {
		if had[k] != has[k] {
			return fmt.Errorf("the copy of %s in %s has %s, not %s", k, db, has[k], had[k])
		}
	}

	// The switch: the main database lets go of the workspace and becomes main.sql.
	var owners []string
	if err := tx.QueryRow(ctx, "SELECT coalesce(array_agg(email::text ORDER BY email), '{}') FROM archive.member WHERE role = 'owner'").Scan(&owners); err != nil {
		return err
	}
	if err := moveDrop(ctx, tx, src); err != nil {
		return fmt.Errorf("dropping the workspace from the main database: %w", err)
	}
	left, err := moveMainRows(ctx, tx)
	if err != nil {
		return err
	}
	if !maps.Equal(left, mainRows) {
		return fmt.Errorf("dropping the workspace changed the main tables, from %v to %v", mainRows, left)
	}
	if err := mainMigrate(ctx, tx, want); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE archive."user" SET admin = true WHERE email = ANY($1)`, owners)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		fmt.Fprintf(os.Stderr, "archive: the first workspace's owners %v have no users, so this Archive has no admin; add one with archive admin add <email>\n", owners)
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	// The copy is the workspace now, so it stops saying it is being made.
	if _, err := t.Exec(bg, "DROP SCHEMA "+moveMark); err != nil {
		fmt.Fprintf(os.Stderr, "archive: %s moved, but its schema %s, which says it was being made, stays; drop it by hand: %v\n", db, moveMark, err)
	}
	fmt.Fprintf(os.Stderr, "archive: moved the first workspace %s out of the main database into %s, %d tables and %d rows, checked, in %s; its owners %v are this Archive's admins\n",
		id, db, len(had), rows, time.Since(start).Round(time.Millisecond), owners)
	return nil
}

func cmpErr(err, or error) error {
	if err != nil {
		return err
	}
	return or
}

// moveSettings makes values read and written as text the same on both sides of a copy, and names every object
// with its schema.
func moveSettings(ctx context.Context, q pgx.Tx) error {
	for _, st := range []string{"search_path = ''", "DateStyle = 'ISO, MDY'", "IntervalStyle = 'postgres'", "TimeZone = 'UTC'", "extra_float_digits = 1",
		"bytea_output = 'hex'", "check_function_bodies = off"} {
		if _, err := q.Exec(ctx, "SET LOCAL "+st); err != nil {
			return err
		}
	}
	return nil
}

// moveOwn is the part of s that is the workspace's: all of it but the main tables and types, and what is theirs.
func moveOwn(s pgSchema) pgSchema {
	return maps.Collect(func(yield func(string, *pgObject) bool) {
		for k, o := range s {
			if o.Schema == "archive" && (mainNames[o.Name] && o.Of == "" || mainNames[o.Of]) {
				continue
			}
			if !yield(k, o) {
				return
			}
		}
	})
}

// moveRels are the workspace's tables, with their columns, parents and children.
func moveRels(ctx context.Context, tx pgx.Tx) ([]*moveRel, error) {
	rows, err := tx.Query(ctx, `SELECT c.oid, n.nspname::text, c.relname::text, c.relkind::text, c.relpersistence::text, c.relispartition,
            coalesce((SELECT array_agg(i.inhparent ORDER BY i.inhseqno) FROM pg_catalog.pg_inherits i WHERE i.inhrelid = c.oid), '{}'),
            concat_ws(' ', (SELECT 'PARTITION OF ' || i.inhparent::regclass || ' ' || pg_catalog.pg_get_expr(c.relpartbound, c.oid) FROM pg_catalog.pg_inherits i WHERE i.inhrelid = c.oid AND c.relispartition),
                'PARTITION BY ' || pg_catalog.pg_get_partkeydef(c.oid), 'WITH (' || array_to_string(c.reloptions, ', ') || ')'),
            c.relrowsecurity, c.relforcerowsecurity
        FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname IN ('archive', 'public') AND c.relkind IN ('r', 'p')
            AND NOT EXISTS (SELECT FROM pg_catalog.pg_depend x WHERE x.objid = c.oid AND x.deptype = 'e')
        ORDER BY n.nspname, c.relname`)
	if err != nil {
		return nil, err
	}
	var out []*moveRel
	byOID := map[uint32]*moveRel{}
	var r moveRel
	if _, err := pgx.ForEachRow(rows, []any{&r.oid, &r.schema, &r.name, &r.kind, &r.persist, &r.partition, &r.parents, &r.def, &r.rls, &r.force}, func() error {
		if r.schema == "archive" && mainNames[r.name] {
			return nil
		}
		c := r
		out = append(out, &c)
		byOID[c.oid] = &c
		return nil
	}); err != nil {
		return nil, err
	}
	rows, err = tx.Query(ctx, `SELECT a.attrelid, a.attname::text,
            concat_ws(' ', pg_catalog.format_type(a.atttypid, a.atttypmod),
                'COLLATE ' || (SELECT pg_catalog.quote_ident(l.collname) FROM pg_catalog.pg_collation l WHERE l.oid = a.attcollation AND a.attcollation <> t.typcollation),
                CASE WHEN a.attgenerated <> '' THEN 'GENERATED ALWAYS AS (' || pg_catalog.pg_get_expr(d.adbin, d.adrelid) || CASE a.attgenerated WHEN 's' THEN ') STORED' ELSE ') VIRTUAL' END
                    ELSE 'DEFAULT ' || pg_catalog.pg_get_expr(d.adbin, d.adrelid) END,
                CASE a.attidentity WHEN 'a' THEN 'GENERATED ALWAYS AS IDENTITY' WHEN 'd' THEN 'GENERATED BY DEFAULT AS IDENTITY' END,
                CASE WHEN a.attnotnull THEN 'NOT NULL' END),
            CASE WHEN a.attgenerated = '' THEN coalesce(pg_catalog.pg_get_expr(d.adbin, d.adrelid), '') ELSE '' END,
            pg_catalog.format_type(a.atttypid, a.atttypmod),
            coalesce('COLLATE ' || (SELECT pg_catalog.quote_ident(l.collname) FROM pg_catalog.pg_collation l WHERE l.oid = a.attcollation AND a.attcollation <> t.typcollation), ''),
            CASE WHEN a.attgenerated <> '' THEN pg_catalog.pg_get_expr(d.adbin, d.adrelid) ELSE '' END, a.attgenerated::text,
            a.attnotnull, a.attgenerated <> '' OR a.attidentity <> '', a.attgenerated <> '', a.attislocal, a.attinhcount
        FROM pg_catalog.pg_attribute a JOIN pg_catalog.pg_type t ON t.oid = a.atttypid
            LEFT JOIN pg_catalog.pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
        WHERE a.attrelid = ANY($1) AND a.attnum > 0 AND NOT a.attisdropped
        ORDER BY a.attrelid, a.attnum`, slices.Collect(maps.Keys(byOID)))
	if err != nil {
		return nil, err
	}
	var rel uint32
	var c moveCol
	if _, err := pgx.ForEachRow(rows, []any{&rel, &c.name, &c.def, &c.dflt, &c.typ, &c.coll, &c.expr, &c.gen, &c.notNull, &c.special, &c.generated, &c.local, &c.inherited}, func() error {
		r := byOID[rel]
		r.cols = append(r.cols, c)
		if c.generated {
			r.generated = append(r.generated, c.name)
		} else {
			r.rows = append(r.rows, pgIdent(c.name))
		}
		return nil
	}); err != nil {
		return nil, err
	}
	for _, r := range out {
		for _, p := range r.parents {
			if byOID[p] == nil {
				return nil, fmt.Errorf("%s inherits a table that is not the workspace's", r.ident())
			}
			byOID[p].children = append(byOID[p].children, r)
		}
	}
	return out, nil
}

// moveUnit is DDL that runs as a whole or not at all.
type moveUnit struct {
	name  string
	stmts []string
}

// moveRun runs every unit in tx, each in a savepoint, over and over until all have: one that needs another, like
// a view of a view or a function a default calls, runs once that one has. A round in which none runs is an
// error.
func moveRun(ctx context.Context, tx pgx.Tx, units []moveUnit) error {
	left, err := moveTry(ctx, tx, units)
	if len(left) > 0 {
		return fmt.Errorf("%s: %w", left[0].name, err)
	}
	return nil
}

// moveTry is moveRun that stops at a round in which none runs, with what is left and why the first of it did not.
func moveTry(ctx context.Context, tx pgx.Tx, units []moveUnit) ([]moveUnit, error) {
	for len(units) > 0 {
		var left []moveUnit
		var last error
		for _, u := range units {
			err := pgx.BeginFunc(ctx, tx, func(sp pgx.Tx) error {
				for _, st := range u.stmts {
					if _, err := sp.Exec(ctx, st); err != nil {
						return fmt.Errorf("%s: %w", st, err)
					}
				}
				return nil
			})
			if err != nil {
				left, last = append(left, u), err
			}
		}
		if len(left) == len(units) {
			return left, last
		}
		units = left
	}
	return nil, nil
}

// moveCopy makes the workspace again in t, the new database, from src and rels, and copies every row of rels
// from tx into it, which is how many it copied.
func moveCopy(ctx context.Context, tx pgx.Tx, t *pgx.Conn, src pgSchema, rels []*moveRel) (int64, error) {
	tt, err := t.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tt.Rollback(context.WithoutCancel(ctx))
	if err := moveSettings(ctx, tt); err != nil {
		return 0, err
	}
	keys := slices.Sorted(maps.Keys(src))
	objs := func(kinds ...string) []*pgObject {
		var out []*pgObject
		for _, k := range keys {
			if o := src[k]; slices.Contains(kinds, o.Kind) && !(o.Kind == "constraint" && src["domain "+o.Schema+"."+o.Of] != nil) {
				out = append(out, o)
			}
		}
		return out
	}
	// Before the rows: the schemas, types, functions, sequences, tables and views.
	pre := []moveUnit{}
	if src["schema archive.archive"] != nil {
		pre = append(pre, moveUnit{"schema archive", []string{"CREATE SCHEMA archive"}})
	}
	for _, o := range objs("type", "domain", "function", "sequence") {
		st := o.Def
		if o.Kind != "function" {
			st = src.render(o)
		}
		pre = append(pre, moveUnit{o.key(), []string{st}})
	}
	pre = append(pre, moveTables(rels)...)
	for _, o := range objs("view") {
		pre = append(pre, moveUnit{o.key(), []string{src.render(o)}})
	}
	// What cannot be made before the constraints, like a view that groups by a primary key, is made after them.
	later, err := moveTry(ctx, tt, pre)
	if i := slices.IndexFunc(later, func(u moveUnit) bool { return strings.HasPrefix(u.name, "table ") }); i >= 0 {
		return 0, fmt.Errorf("%s: %w", later[i].name, err)
	}

	// The rows, table by table, as text, which every type reads back as it was written.
	var n int64
	for _, r := range rels {
		if r.kind != "r" || len(r.rows) == 0 {
			continue
		}
		cols := strings.Join(r.rows, ", ")
		pr, pw := io.Pipe()
		got := make(chan error, 1)
		var sent pgconn.CommandTag
		go func() {
			var err error
			sent, err = tx.Conn().PgConn().CopyTo(ctx, pw, "COPY (SELECT "+cols+" FROM ONLY "+r.ident()+") TO STDOUT")
			pw.CloseWithError(err)
			got <- err
		}()
		tag, err := tt.Conn().PgConn().CopyFrom(ctx, pr, "COPY "+r.ident()+" ("+cols+") FROM STDIN")
		pr.CloseWithError(cmpErr(err, io.EOF))
		if err := cmpErr(<-got, err); err != nil {
			return 0, fmt.Errorf("copying %s: %w", r.ident(), err)
		}
		if tag.RowsAffected() != sent.RowsAffected() {
			return 0, fmt.Errorf("copying %s: %d rows went in of %d", r.ident(), tag.RowsAffected(), sent.RowsAffected())
		}
		n += tag.RowsAffected()
	}

	// After them: what checks, indexes or acts on rows, and what sequences are at.
	post := later
	for _, o := range objs("constraint", "index", "trigger", "policy") {
		post = append(post, moveUnit{o.key(), []string{src.create(o)}})
	}
	moved, err := moveTriggers(ctx, tx)
	if err != nil {
		return 0, err
	}
	for _, st := range moved {
		post = append(post, moveUnit{st, []string{st}})
	}
	for _, r := range rels {
		if r.rls {
			post = append(post, moveUnit{r.ident(), []string{"ALTER TABLE " + r.ident() + " ENABLE ROW LEVEL SECURITY"}})
		}
		if r.force {
			post = append(post, moveUnit{r.ident(), []string{"ALTER TABLE " + r.ident() + " FORCE ROW LEVEL SECURITY"}})
		}
	}
	seqs, err := moveSequences(ctx, tx)
	if err != nil {
		return 0, err
	}
	post = append(post, seqs...)
	for _, k := range keys {
		if o := src[k]; o.Comment != "" && o.Kind != "default acl" {
			post = append(post, moveUnit{k, []string{"COMMENT ON " + src.target(o) + " IS " + pgLiteral(o.Comment)}})
		}
	}
	if err := moveRun(ctx, tt, post); err != nil {
		return 0, err
	}

	// Last, the grants, from what Postgres gave each object as it was made to what it had, to each role in the
	// order it had them, which is the order Postgres keeps them in.
	got, err := pgRead(ctx, tt, "archive", "public")
	if err != nil {
		return 0, err
	}
	order, err := moveGrantOrder(ctx, tx)
	if err != nil {
		return 0, err
	}
	var grants []moveUnit
	for _, k := range keys {
		o := src[k]
		was := ""
		if g := got[k]; g != nil {
			was = g.ACL
		}
		if was == o.ACL {
			continue
		}
		target := ""
		if o.Kind != "default acl" {
			target = src.target(o)
		}
		sts, err := pgGrants(target, o, was, o.ACL)
		if err != nil {
			return 0, fmt.Errorf("%s: %w", k, err)
		}
		at := func(st string) int {
			if !strings.Contains(st, "GRANT ") || strings.Contains(st, "REVOKE ") {
				return -1
			}
			return slices.Index(order[k], storeUnquote(st[strings.LastIndex(st, " TO ")+4:]))
		}
		slices.SortStableFunc(sts, func(a, b string) int { return at(a) - at(b) })
		grants = append(grants, moveUnit{k, sts})
	}
	if err := moveRun(ctx, tt, grants); err != nil {
		return 0, err
	}
	return n, tt.Commit(ctx)
}

// moveGrantOrder is, for each object with grants of its own, the roles they are to, in their order, by the
// object's key in pgRead.
func moveGrantOrder(ctx context.Context, tx pgx.Tx) (map[string][]string, error) {
	rows, err := tx.Query(ctx, `WITH acl AS (
        SELECT CASE c.relkind WHEN 'v' THEN 'view' WHEN 'S' THEN 'sequence' ELSE 'table' END || ' ' || n.nspname || '.' || c.relname AS k, c.relacl AS a
        FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname IN ('archive', 'public') AND c.relacl IS NOT NULL
        UNION ALL SELECT 'column ' || n.nspname || '.' || c.relname || '.' || a.attname, a.attacl FROM pg_catalog.pg_attribute a JOIN pg_catalog.pg_class c ON c.oid = a.attrelid
            JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname IN ('archive', 'public') AND a.attacl IS NOT NULL
        UNION ALL SELECT 'function ' || n.nspname || '.' || p.proname || '(' || pg_catalog.pg_get_function_identity_arguments(p.oid) || ')', p.proacl
            FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace WHERE n.nspname IN ('archive', 'public') AND p.proacl IS NOT NULL
        UNION ALL SELECT CASE t.typtype WHEN 'd' THEN 'domain' ELSE 'type' END || ' ' || n.nspname || '.' || t.typname, t.typacl
            FROM pg_catalog.pg_type t JOIN pg_catalog.pg_namespace n ON n.oid = t.typnamespace WHERE n.nspname IN ('archive', 'public') AND t.typacl IS NOT NULL
        UNION ALL SELECT 'schema ' || n.nspname || '.' || n.nspname, n.nspacl FROM pg_catalog.pg_namespace n WHERE n.nspname IN ('archive', 'public') AND n.nspacl IS NOT NULL
        UNION ALL SELECT 'default acl ' || n.nspname || '.' || pg_catalog.pg_get_userbyid(d.defaclrole) || ' ' || d.defaclobjtype::text, d.defaclacl
            FROM pg_catalog.pg_default_acl d JOIN pg_catalog.pg_namespace n ON n.oid = d.defaclnamespace WHERE n.nspname IN ('archive', 'public'))
    SELECT acl.k, ARRAY(SELECT CASE WHEN x.grantee = 0 THEN 'PUBLIC' ELSE pg_catalog.pg_get_userbyid(x.grantee)::text END
        FROM pg_catalog.aclexplode(acl.a) WITH ORDINALITY x(grantor, grantee, privilege_type, is_grantable, i) GROUP BY x.grantee ORDER BY min(x.i))
    FROM acl`)
	if err != nil {
		return nil, err
	}
	out := map[string][]string{}
	var k string
	var who []string
	_, err = pgx.ForEachRow(rows, []any{&k, &who}, func() error { out[k] = who; return nil })
	return out, err
}

// moveTables makes the workspace's tables with their columns in the order they have. A table that inherits
// another, as each of the workspace's inherits archive.object, gets its columns as it once did, and in the
// same order: its own ones it adds itself, and the ones only its parent has come with the parent, as the
// parent gains them one at a time. A family that cannot be made so is made the plain way, CREATE TABLE ...
// INHERITS, and the check says how it came out.
func moveTables(rels []*moveRel) []moveUnit {
	var out []moveUnit
	table := func(r *moveRel) string {
		return "CREATE " + map[bool]string{true: "UNLOGGED ", false: ""}[r.persist == "u"] + "TABLE " + r.ident()
	}
	plain := func(r *moveRel) moveUnit {
		var cols []string
		for _, c := range r.cols {
			if c.local {
				cols = append(cols, pgIdent(c.name)+" "+c.def)
			}
		}
		st := table(r)
		if len(cols) > 0 || !r.partition {
			st += " (\n\t" + strings.Join(cols, ",\n\t") + "\n)"
		}
		def := r.def
		if !r.partition && len(r.parents) > 0 {
			var parents []string
			for _, p := range r.parents {
				for _, x := range rels {
					if x.oid == p {
						parents = append(parents, x.ident())
					}
				}
			}
			def = strings.TrimSpace("INHERITS (" + strings.Join(parents, ", ") + ") " + def)
		}
		return moveUnit{"table " + r.ident(), []string{strings.TrimSpace(st + " " + def)}}
	}
	family := func(p *moveRel, kids []*moveRel) (moveUnit, bool) {
		u := moveUnit{name: "table " + p.ident(), stmts: []string{strings.TrimSpace(table(p) + " () " + p.def)}}
		var set []string
		at := map[*moveRel]int{}
		add := func(k *moveRel) {
			for ; at[k] < len(k.cols) && k.cols[at[k]].local; at[k]++ {
				c := k.cols[at[k]]
				u.stmts = append(u.stmts, "ALTER TABLE "+k.ident()+" ADD COLUMN "+pgIdent(c.name)+" "+c.add())
				set = append(set, c.set(k.ident())...)
			}
		}
		for _, k := range kids {
			u.stmts = append(u.stmts, strings.TrimSpace(table(k)+" () "+k.def), "ALTER TABLE "+k.ident()+" INHERIT "+p.ident())
		}
		for _, pc := range p.cols {
			for _, k := range kids {
				add(k)
			}
			u.stmts = append(u.stmts, "ALTER TABLE "+p.ident()+" ADD COLUMN "+pgIdent(pc.name)+" "+pc.def)
			for _, k := range kids {
				switch kc := k.col(pc.name); {
				case kc == nil || kc.special || pc.special:
					return u, false
				case at[k] < len(k.cols) && k.cols[at[k]].name == pc.name && !kc.local:
					at[k]++
					if kc.dflt != pc.dflt && kc.dflt == "" {
						u.stmts = append(u.stmts, "ALTER TABLE ONLY "+k.ident()+" ALTER COLUMN "+pgIdent(kc.name)+" DROP DEFAULT")
					} else if kc.dflt != pc.dflt {
						u.stmts = append(u.stmts, "ALTER TABLE ONLY "+k.ident()+" ALTER COLUMN "+pgIdent(kc.name)+" SET DEFAULT "+kc.dflt)
					}
					if kc.notNull && !pc.notNull {
						u.stmts = append(u.stmts, "ALTER TABLE ONLY "+k.ident()+" ALTER COLUMN "+pgIdent(kc.name)+" SET NOT NULL")
					}
				case !slices.ContainsFunc(k.cols[:at[k]], func(c moveCol) bool { return c.name == pc.name }):
					return u, false
				}
			}
		}
		for _, k := range kids {
			if add(k); at[k] < len(k.cols) {
				return u, false
			}
		}
		u.stmts = append(u.stmts, set...)
		return u, true
	}
	done := map[*moveRel]bool{}
	for _, r := range rels {
		// A family is a table with no parent and the children that have only it, are plain tables and have no
		// children of their own; its other children are made after it.
		kids := slices.DeleteFunc(slices.Clone(r.children), func(k *moveRel) bool {
			return len(k.parents) != 1 || k.partition || len(k.children) > 0
		})
		if len(kids) == 0 || len(r.parents) > 0 || r.kind != "r" {
			continue
		}
		if u, ok := family(r, kids); ok {
			out = append(out, u)
			done[r] = true
			for _, k := range kids {
				done[k] = true
			}
		}
	}
	for _, r := range rels {
		if !done[r] {
			out = append(out, plain(r))
		}
	}
	return out
}

// moveTriggers are the statements that make again the triggers pgRead leaves out: archive_moved on each table
// of the public schema.
func moveTriggers(ctx context.Context, tx pgx.Tx) ([]string, error) {
	rows, err := tx.Query(ctx, `SELECT pg_catalog.pg_get_triggerdef(g.oid) FROM pg_catalog.pg_trigger g JOIN pg_catalog.pg_class c ON c.oid = g.tgrelid
        WHERE c.relnamespace = 'public'::regnamespace AND g.tgname = 'archive_moved' AND NOT g.tgisinternal AND g.tgparentid = 0 ORDER BY 1`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// moveSequences sets each sequence of the workspace, its own and its identity columns', to where it is, and
// ties it to the column it belongs to.
func moveSequences(ctx context.Context, tx pgx.Tx) ([]moveUnit, error) {
	rows, err := tx.Query(ctx, `SELECT pg_catalog.quote_ident(n.nspname) || '.' || pg_catalog.quote_ident(c.relname), coalesce(d.deptype::text, ''),
            coalesce((SELECT pg_catalog.quote_ident(rn.nspname) || '.' || pg_catalog.quote_ident(r.relname) FROM pg_catalog.pg_class r
                JOIN pg_catalog.pg_namespace rn ON rn.oid = r.relnamespace WHERE r.oid = d.refobjid), ''),
            coalesce((SELECT a.attname::text FROM pg_catalog.pg_attribute a WHERE a.attrelid = d.refobjid AND a.attnum = d.refobjsubid), ''),
            pg_catalog.pg_sequence_last_value(c.oid)::text
        FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
            LEFT JOIN pg_catalog.pg_depend d ON d.classid = 'pg_catalog.pg_class'::regclass AND d.objid = c.oid AND d.refclassid = 'pg_catalog.pg_class'::regclass AND d.deptype IN ('a', 'i')
        WHERE n.nspname IN ('archive', 'public') AND c.relkind = 'S' ORDER BY 1`)
	if err != nil {
		return nil, err
	}
	var out []moveUnit
	var seq, dep, rel, col string
	var last *string
	_, err = pgx.ForEachRow(rows, []any{&seq, &dep, &rel, &col, &last}, func() error {
		var sts []string
		target := pgLiteral(seq)
		if dep == "i" {
			// An identity's sequence is made with its column, under a name of Postgres's choosing.
			target = "pg_catalog.pg_get_serial_sequence(" + pgLiteral(rel) + ", " + pgLiteral(col) + ")"
		} else if dep == "a" {
			sts = append(sts, "ALTER SEQUENCE "+seq+" OWNED BY "+rel+"."+pgIdent(col))
		}
		if last != nil {
			sts = append(sts, "SELECT pg_catalog.setval("+target+", "+*last+", true)")
		}
		if len(sts) > 0 {
			out = append(out, moveUnit{seq, sts})
		}
		return nil
	})
	return out, err
}

// moveDatabase gives the new database what the main database's own settings were, and lets the workspace's
// authenticator alone connect to it, as createWorkspace does; the main database no longer lets it in.
func moveDatabase(ctx context.Context, tx pgx.Tx, h *Hub, db, id string) error {
	var main string
	var settings []string
	if err := tx.QueryRow(ctx, `SELECT current_database()::text, coalesce((SELECT s.setconfig FROM pg_catalog.pg_db_role_setting s
        JOIN pg_catalog.pg_database d ON d.oid = s.setdatabase WHERE d.datname = current_database() AND s.setrole = 0), '{}')`).Scan(&main, &settings); err != nil {
		return err
	}
	sts := []string{"REVOKE CONNECT ON DATABASE " + pgIdent(db) + " FROM PUBLIC"}
	for _, s := range settings {
		// search_path is a list, kept as it is written; any other one value.
		k, v, _ := strings.Cut(s, "=")
		if k != "search_path" {
			v = pgLiteral(v)
		}
		sts = append(sts, "ALTER DATABASE "+pgIdent(db)+" SET "+pgIdent(k)+" = "+v)
	}
	var auth bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = $1)", id+"_authenticator").Scan(&auth); err != nil {
		return err
	}
	if auth {
		sts = append(sts, "GRANT CONNECT ON DATABASE "+pgIdent(db)+" TO "+pgIdent(id+"_authenticator"))
	}
	for _, st := range sts {
		if _, err := h.main.Exec(ctx, st); err != nil {
			return fmt.Errorf("%s: %w", st, err)
		}
	}
	if auth {
		if _, err := tx.Exec(ctx, "REVOKE CONNECT ON DATABASE "+pgIdent(main)+" FROM "+pgIdent(id+"_authenticator")); err != nil {
			return err
		}
	}
	return nil
}

// moveNoForce lets tx, the tables' owner, read every row of them, past the row-level security each forces on
// its owner too; only tx sees it, and it is undone when tx ends without committing.
func moveNoForce(ctx context.Context, tx pgx.Tx, rels []*moveRel) error {
	for _, r := range rels {
		if r.force {
			if _, err := tx.Exec(ctx, "ALTER TABLE "+r.ident()+" NO FORCE ROW LEVEL SECURITY"); err != nil {
				return err
			}
		}
	}
	return nil
}

// moveRows is how many rows each table of rels has in tx, and a sum of their hashes, which any changed row changes.
func moveRows(ctx context.Context, tx pgx.Tx, rels []*moveRel) (map[string]string, error) {
	out := map[string]string{}
	for _, r := range rels {
		if r.kind != "r" {
			continue
		}
		var n int64
		var sum string
		if err := tx.QueryRow(ctx, "SELECT count(*), coalesce(sum(pg_catalog.hashtextextended(t::text, 0)), 0)::text FROM ONLY "+r.ident()+" t").Scan(&n, &sum); err != nil {
			return nil, err
		}
		out[r.schema+"."+r.name] = fmt.Sprintf("%d rows (hash %s)", n, sum)
	}
	return out, nil
}

// moveCheckRows is moveRows of the copy, read in a transaction that is rolled back.
func moveCheckRows(ctx context.Context, t *pgx.Conn, rels []*moveRel) (map[string]string, error) {
	tx, err := t.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	if err := moveSettings(ctx, tx); err != nil {
		return nil, err
	}
	if err := moveNoForce(ctx, tx, rels); err != nil {
		return nil, err
	}
	return moveRows(ctx, tx, rels)
}

// moveMainRows is how many rows each main table has and what its columns are, which dropping the workspace
// must not change.
func moveMainRows(ctx context.Context, tx pgx.Tx) (map[string]string, error) {
	out := map[string]string{}
	for _, name := range slices.Sorted(maps.Keys(mainNames)) {
		var cols *string
		if err := tx.QueryRow(ctx, `SELECT string_agg(a.attname || ' ' || pg_catalog.format_type(a.atttypid, a.atttypmod), ', ' ORDER BY a.attnum)
            FROM pg_catalog.pg_attribute a WHERE a.attrelid = to_regclass($1) AND a.attnum > 0 AND NOT a.attisdropped`, pgIdent("archive", name)).Scan(&cols); err != nil {
			return nil, err
		}
		if cols == nil {
			continue
		}
		var n int64
		if err := tx.QueryRow(ctx, "SELECT count(*) FROM ONLY "+pgIdent("archive", name)).Scan(&n); err != nil {
			return nil, err
		}
		out[name] = fmt.Sprintf("%d rows of %s", n, *cols)
	}
	return out, nil
}

// moveDrop drops the workspace from the main database: everything of the public schema, and what of the archive
// schema main.sql does not have. What is left of it, like a default privilege, main.sql's migration drops.
func moveDrop(ctx context.Context, tx pgx.Tx, src pgSchema) error {
	order := map[string]int{"view": 0, "table": 1, "sequence": 2, "function": 3, "domain": 4, "type": 4}
	objs := slices.Collect(maps.Values(src))
	slices.SortFunc(objs, func(a, b *pgObject) int {
		return strings.Compare(fmt.Sprint(order[a.Kind], a.key()), fmt.Sprint(order[b.Kind], b.key()))
	})
	for _, o := range objs {
		if _, ok := order[o.Kind]; !ok {
			continue
		}
		if _, err := tx.Exec(ctx, "DROP "+strings.Replace(src.target(o), " ", " IF EXISTS ", 1)+" CASCADE"); err != nil {
			return err
		}
	}
	var left []string
	rows, err := tx.Query(ctx, `SELECT c.relname::text FROM pg_catalog.pg_class c WHERE c.relnamespace = 'public'::regnamespace
        UNION ALL SELECT p.proname::text FROM pg_catalog.pg_proc p WHERE p.pronamespace = 'public'::regnamespace
        UNION ALL SELECT t.typname::text FROM pg_catalog.pg_type t WHERE t.typnamespace = 'public'::regnamespace`)
	if err != nil {
		return err
	}
	if left, err = pgx.CollectRows(rows, pgx.RowTo[string]); err != nil {
		return err
	}
	if len(left) > 0 {
		return fmt.Errorf("the public schema still has %s", strings.Join(left, ", "))
	}
	return nil
}

// moveDiff is what differs between two fingerprints, the first few of it.
func moveDiff(a, b map[string]string) []string {
	var out []string
	for _, k := range slices.Sorted(maps.Keys(a)) {
		if b[k] != a[k] {
			out = append(out, fmt.Sprintf("%s: %q, in the copy %q", k, a[k], b[k]))
		}
	}
	for _, k := range slices.Sorted(maps.Keys(b)) {
		if _, ok := a[k]; !ok {
			out = append(out, fmt.Sprintf("%s: only in the copy, %q", k, b[k]))
		}
	}
	if len(out) > 20 {
		out = append(out[:20], fmt.Sprintf("and %d more", len(out)-20))
	}
	return out
}

// moveFingerprint is every object of the archive and public schemas as Postgres renders it, without anything
// that names it by its oid, so a copy has the same one: each schema, table, view, sequence and where it is,
// column in its place, constraint, index, trigger, policy, rule, type, function, default privilege and grant,
// with its owner and comment. What else a schema could hold it counts. It leaves out the main tables and types.
func moveFingerprint(ctx context.Context, q pgx.Tx) (map[string]string, error) {
	rows, err := q.Query(ctx, moveFingerprintSQL)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	var kind, schema, rel, name, def string
	_, err = pgx.ForEachRow(rows, []any{&kind, &schema, &rel, &name, &def}, func() error {
		if schema == "archive" && (mainNames[rel] || kind == "type" && mainNames[name]) {
			return nil
		}
		out[kind+" "+schema+"."+rel+"."+name] = def
		return nil
	})
	return out, err
}

// {acl x} is the grants in x, sorted, with the owner's when x is null, as Postgres means it.
var moveFingerprintSQL = pgMacro(`WITH ns AS (SELECT oid, nspname::text AS nspname FROM pg_catalog.pg_namespace WHERE nspname IN ('archive', 'public')),
ext AS (SELECT objid FROM pg_catalog.pg_depend WHERE deptype = 'e')
SELECT 'schema', n.nspname, '', '', concat_ws(' | ', pg_catalog.pg_get_userbyid(s.nspowner), {acl coalesce(s.nspacl, pg_catalog.acldefault('n', s.nspowner))}, pg_catalog.obj_description(s.oid, 'pg_namespace'))
FROM ns n JOIN pg_catalog.pg_namespace s ON s.oid = n.oid
UNION ALL
SELECT 'relation', n.nspname, c.relname::text, '', concat_ws(' | ', c.relkind, c.relpersistence, pg_catalog.pg_get_userbyid(c.relowner), {acl coalesce(c.relacl, pg_catalog.acldefault(CASE c.relkind WHEN 'S' THEN 's' ELSE 'r' END::"char", c.relowner))},
    c.reloptions::text, c.relrowsecurity, c.relforcerowsecurity, c.relreplident, c.relispartition, pg_catalog.obj_description(c.oid, 'pg_class'),
    (SELECT string_agg(i.inhparent::regclass::text, ', ' ORDER BY i.inhseqno) FROM pg_catalog.pg_inherits i WHERE i.inhrelid = c.oid),
    pg_catalog.pg_get_expr(c.relpartbound, c.oid), CASE WHEN c.relkind = 'p' THEN pg_catalog.pg_get_partkeydef(c.oid) END,
    CASE WHEN c.relkind IN ('v', 'm') THEN pg_catalog.pg_get_viewdef(c.oid) END,
    (SELECT concat_ws(' ', q.seqtypid::regtype, q.seqstart, q.seqincrement, q.seqmin, q.seqmax, q.seqcache, q.seqcycle, pg_catalog.pg_sequence_last_value(c.oid))
        FROM pg_catalog.pg_sequence q WHERE q.seqrelid = c.oid),
    (SELECT d.refobjid::regclass::text || '.' || a.attname || ' ' || d.deptype::text FROM pg_catalog.pg_depend d JOIN pg_catalog.pg_attribute a ON a.attrelid = d.refobjid
        AND a.attnum = d.refobjsubid WHERE d.classid = 'pg_catalog.pg_class'::regclass AND d.objid = c.oid AND d.refclassid = 'pg_catalog.pg_class'::regclass AND d.deptype IN ('a', 'i')))
FROM pg_catalog.pg_class c JOIN ns n ON n.oid = c.relnamespace WHERE c.relkind NOT IN ('i', 'I', 't') AND c.oid NOT IN (SELECT objid FROM ext)
UNION ALL
SELECT 'column', n.nspname, c.relname::text, a.attname::text, concat_ws(' | ', row_number() OVER (PARTITION BY a.attrelid ORDER BY a.attnum),
    pg_catalog.format_type(a.atttypid, a.atttypmod), a.attcollation::regcollation::text, a.attnotnull, pg_catalog.pg_get_expr(d.adbin, d.adrelid), a.attgenerated, a.attidentity,
    a.attislocal, a.attinhcount, {acl a.attacl}, pg_catalog.col_description(c.oid, a.attnum))
FROM pg_catalog.pg_attribute a JOIN pg_catalog.pg_class c ON c.oid = a.attrelid JOIN ns n ON n.oid = c.relnamespace
    LEFT JOIN pg_catalog.pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
WHERE c.relkind NOT IN ('i', 'I', 't', 'S') AND a.attnum > 0 AND NOT a.attisdropped AND c.oid NOT IN (SELECT objid FROM ext)
UNION ALL
SELECT 'constraint', n.nspname, coalesce(c.relname, t.typname)::text, k.conname::text, concat_ws(' | ', k.contype, pg_catalog.pg_get_constraintdef(k.oid), k.conislocal,
    k.coninhcount, k.convalidated, k.connoinherit, k.condeferrable, k.condeferred, pg_catalog.obj_description(k.oid, 'pg_constraint'))
FROM pg_catalog.pg_constraint k JOIN ns n ON n.oid = k.connamespace LEFT JOIN pg_catalog.pg_class c ON c.oid = k.conrelid LEFT JOIN pg_catalog.pg_type t ON t.oid = k.contypid
WHERE k.contype <> 'n'
UNION ALL
SELECT 'index', n.nspname, c.relname::text, i.relname::text, concat_ws(' | ', pg_catalog.pg_get_indexdef(i.oid), x.indisvalid, pg_catalog.obj_description(i.oid, 'pg_class'))
FROM pg_catalog.pg_index x JOIN pg_catalog.pg_class i ON i.oid = x.indexrelid JOIN pg_catalog.pg_class c ON c.oid = x.indrelid JOIN ns n ON n.oid = c.relnamespace
WHERE c.relkind <> 't'
UNION ALL
SELECT 'trigger', n.nspname, c.relname::text, g.tgname::text, concat_ws(' | ', pg_catalog.pg_get_triggerdef(g.oid), g.tgenabled, pg_catalog.obj_description(g.oid, 'pg_trigger'))
FROM pg_catalog.pg_trigger g JOIN pg_catalog.pg_class c ON c.oid = g.tgrelid JOIN ns n ON n.oid = c.relnamespace WHERE NOT g.tgisinternal
UNION ALL
SELECT 'policy', n.nspname, c.relname::text, p.polname::text, concat_ws(' | ', p.polcmd, p.polpermissive,
    (SELECT string_agg(CASE WHEN r = 0 THEN 'PUBLIC' ELSE pg_catalog.pg_get_userbyid(r)::text END, ', ' ORDER BY 1) FROM unnest(p.polroles) r),
    pg_catalog.pg_get_expr(p.polqual, p.polrelid), pg_catalog.pg_get_expr(p.polwithcheck, p.polrelid), pg_catalog.obj_description(p.oid, 'pg_policy'))
FROM pg_catalog.pg_policy p JOIN pg_catalog.pg_class c ON c.oid = p.polrelid JOIN ns n ON n.oid = c.relnamespace
UNION ALL
SELECT 'rule', n.nspname, c.relname::text, r.rulename::text, pg_catalog.pg_get_ruledef(r.oid)
FROM pg_catalog.pg_rewrite r JOIN pg_catalog.pg_class c ON c.oid = r.ev_class JOIN ns n ON n.oid = c.relnamespace WHERE r.rulename <> '_RETURN'
UNION ALL
SELECT 'type', n.nspname, '', t.typname::text, concat_ws(' | ', t.typtype, pg_catalog.pg_get_userbyid(t.typowner), {acl coalesce(t.typacl, pg_catalog.acldefault('T', t.typowner))}, pg_catalog.obj_description(t.oid, 'pg_type'),
    (SELECT string_agg(e.enumlabel, ', ' ORDER BY e.enumsortorder) FROM pg_catalog.pg_enum e WHERE e.enumtypid = t.oid),
    CASE WHEN t.typtype = 'd' THEN pg_catalog.format_type(t.typbasetype, t.typtypmod) END, pg_catalog.pg_get_expr(t.typdefaultbin, 0), t.typnotnull,
    (SELECT r.rngsubtype::regtype::text FROM pg_catalog.pg_range r WHERE r.rngtypid = t.oid))
FROM pg_catalog.pg_type t JOIN ns n ON n.oid = t.typnamespace
WHERE (t.typrelid = 0 OR (SELECT c.relkind FROM pg_catalog.pg_class c WHERE c.oid = t.typrelid) = 'c')
    AND NOT EXISTS (SELECT FROM pg_catalog.pg_type e WHERE e.typarray = t.oid) AND t.oid NOT IN (SELECT objid FROM ext)
UNION ALL
SELECT 'function', n.nspname, '', p.proname || '(' || pg_catalog.pg_get_function_identity_arguments(p.oid) || ')', concat_ws(' | ', p.prokind,
    pg_catalog.pg_get_userbyid(p.proowner), {acl coalesce(p.proacl, pg_catalog.acldefault('f', p.proowner))}, pg_catalog.obj_description(p.oid, 'pg_proc'), CASE WHEN p.prokind <> 'a' THEN pg_catalog.pg_get_functiondef(p.oid) END)
FROM pg_catalog.pg_proc p JOIN ns n ON n.oid = p.pronamespace WHERE p.oid NOT IN (SELECT objid FROM ext)
UNION ALL
SELECT 'default acl', n.nspname, '', pg_catalog.pg_get_userbyid(d.defaclrole) || ' ' || d.defaclobjtype::text, {acl d.defaclacl}
FROM pg_catalog.pg_default_acl d JOIN ns n ON n.oid = d.defaclnamespace
UNION ALL
SELECT 'statistics', n.nspname, '', s.stxname::text, pg_catalog.pg_get_statisticsobjdef(s.oid)
FROM pg_catalog.pg_statistic_ext s JOIN ns n ON n.oid = s.stxnamespace
UNION ALL
SELECT 'other', n.nspname, x.catalog, '', count(*)::text FROM ns n CROSS JOIN LATERAL (
    SELECT 'collation' AS catalog FROM pg_catalog.pg_collation WHERE collnamespace = n.oid
    UNION ALL SELECT 'operator' FROM pg_catalog.pg_operator WHERE oprnamespace = n.oid
    UNION ALL SELECT 'operator class' FROM pg_catalog.pg_opclass WHERE opcnamespace = n.oid
    UNION ALL SELECT 'operator family' FROM pg_catalog.pg_opfamily WHERE opfnamespace = n.oid
    UNION ALL SELECT 'conversion' FROM pg_catalog.pg_conversion WHERE connamespace = n.oid
    UNION ALL SELECT 'text search' FROM pg_catalog.pg_ts_config WHERE cfgnamespace = n.oid
    UNION ALL SELECT 'text search' FROM pg_catalog.pg_ts_dict WHERE dictnamespace = n.oid) x GROUP BY n.nspname, x.catalog
UNION ALL
SELECT 'extension', '', '', e.extname::text, e.extversion FROM pg_catalog.pg_extension e`,
	"acl", "(SELECT string_agg(acl_item::text, ',' ORDER BY acl_item::text) FROM unnest($1) acl_item)")
