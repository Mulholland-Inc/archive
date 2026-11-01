package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// pgTestStore is a store on a Postgres of its own, with the roles of every archive.sql there has been.
func pgTestStore(t *testing.T) *pgStore {
	t.Helper()
	in := newInst(t)
	cfg, err := pgx.ParseConfig("host=" + in.dir + " user=archive dbname=postgres")
	if err != nil {
		t.Fatal(err)
	}
	s := &pgStore{cfg: cfg, prefix: "x"}
	if s.sys, err = storePool(t.Context(), cfg, 4, nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.sys.Close)
	if _, err := s.sys.Exec(t.Context(), s.expand(storeRoles)+";\nCREATE ROLE x_user NOLOGIN"); err != nil {
		t.Fatal(err)
	}
	return s
}

// pgTestDB makes the database name with sqls applied, one transaction each, and returns a pool on it.
func pgTestDB(t *testing.T, s *pgStore, name string, sqls ...string) *pgxpool.Pool {
	t.Helper()
	if _, err := s.sys.Exec(t.Context(), "CREATE DATABASE "+pgIdent(name)); err != nil {
		t.Fatal(err)
	}
	cfg := s.cfg.Copy()
	cfg.Database = name
	db, err := storePool(t.Context(), cfg, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	for _, sql := range sqls {
		if err := pgx.BeginFunc(t.Context(), db, func(tx pgx.Tx) error {
			_, err := tx.Exec(t.Context(), "SET LOCAL check_function_bodies = off;\n"+s.expand(sql))
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

// pgTestSnap is Archive's part of the schema of db.
func pgTestSnap(t *testing.T, db *pgxpool.Pool) pgSchema {
	t.Helper()
	var sch pgSchema
	if err := pgx.BeginFunc(t.Context(), db, func(tx pgx.Tx) (err error) {
		if _, err = tx.Exec(t.Context(), "SET LOCAL search_path = ''"); err == nil {
			sch, err = pgRead(t.Context(), tx, "archive", "public")
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return sch.ours()
}

// pgTestMigrate runs the plan to want in db, dropping what it would lose.
func pgTestMigrate(t *testing.T, db *pgxpool.Pool, want pgSchema) {
	t.Helper()
	if err := pgx.BeginFunc(t.Context(), db, func(tx pgx.Tx) error {
		_, err := pgMigrate(t.Context(), tx, want, true)
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

func pgTestSame(t *testing.T, what string, got, want pgSchema) {
	t.Helper()
	if changes, _ := pgCompare(got, want); len(changes) > 0 {
		for _, c := range changes[:min(len(changes), 10)] {
			t.Errorf("%s: %s %s %s:\n  got  %s\n  want %s", what, c.Action, c.Kind, c.Name, c.Before, c.After)
		}
	}
}

func pgTestVersions(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("testdata/archive_sql/*.sql")
	if err != nil || len(files) < 2 {
		t.Fatal(files, err)
	}
	slices.Sort(files)
	var out []string
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, string(b))
	}
	if out[len(out)-1] != storeSchema {
		t.Fatalf("%s is not src/archive.sql: add the new version to testdata/archive_sql", files[len(files)-1])
	}
	return out
}

// Every archive.sql there has been migrates to the next, and the first to the last, as applying the next
// fresh would make it.
func TestArchiveSQLMigratesFromEveryVersion(t *testing.T) {
	t.Parallel()
	vs := pgTestVersions(t)
	s := pgTestStore(t)
	ctx := t.Context()
	fresh := make([]pgSchema, len(vs))
	for i, v := range vs {
		var err error
		if fresh[i], err = s.scratch(ctx, s.expand(v)); err != nil {
			t.Fatalf("version %d: %v", i+1, err)
		}
	}
	for i := range len(vs) - 1 {
		db := pgTestDB(t, s, fmt.Sprintf("v%d", i+1), vs[i])
		pgTestMigrate(t, db, fresh[i+1])
		pgTestSame(t, fmt.Sprintf("version %d to the next", i+1), pgTestSnap(t, db), fresh[i+1])
	}
	db := pgTestDB(t, s, "first", vs[0])
	pgTestMigrate(t, db, fresh[len(vs)-1])
	pgTestSame(t, "the first version to the last", pgTestSnap(t, db), fresh[len(vs)-1])
	if stmts, lost, err := pgPlan(pgTestSnap(t, db), fresh[len(vs)-1], false); len(stmts) > 0 || len(lost) > 0 || err != nil {
		t.Errorf("a second plan: %q %q %v", stmts, lost, err)
	}
	var n int
	if err := s.sys.QueryRow(ctx, "SELECT count(*) FROM pg_database WHERE datname LIKE 'archive_scratch_%'").Scan(&n); err != nil || n != 0 {
		t.Errorf("%d scratch databases left: %v", n, err)
	}
}

// The plan alters domains and types in place, which user tables use, refuses to lose data, and leaves no
// scratch database behind, even when archive.sql fails.
func TestPlanAltersTypesInPlaceAndKeepsData(t *testing.T) {
	t.Parallel()
	s := pgTestStore(t)
	ctx := t.Context()
	db := pgTestDB(t, s, "live", `CREATE SCHEMA archive;
CREATE DOMAIN archive.code AS text CONSTRAINT code_len CHECK (length(VALUE) < 10);
CREATE TYPE archive.pair AS (a text, b integer);
CREATE TYPE archive.mood AS ENUM ('good', 'bad');
CREATE TABLE archive.thing (id integer PRIMARY KEY, code archive.code, p archive.pair, m archive.mood, n integer, note varchar(10));
CREATE TABLE public.mine (code archive.code, p archive.pair, m archive.mood);
INSERT INTO archive.thing VALUES (1, 'x', ('a', 1), 'good', 1, 'n');
INSERT INTO public.mine VALUES ('y', ('b', 2), 'bad');`)
	want := func(sql string) pgSchema {
		t.Helper()
		sch, err := s.scratch(ctx, sql)
		if err != nil {
			t.Fatal(err)
		}
		return sch
	}
	next := want(`CREATE SCHEMA archive;
CREATE DOMAIN archive.code AS text DEFAULT 'none' NOT NULL CONSTRAINT code_short CHECK (length(VALUE) < 20);
CREATE TYPE archive.pair AS (a text, b integer, c date);
CREATE TYPE archive.mood AS ENUM ('great', 'good', 'meh', 'bad');
CREATE TABLE archive.thing (id integer PRIMARY KEY, code archive.code, p archive.pair, m archive.mood, n integer, note varchar(20));`)
	stmts, lost, err := pgPlan(pgTestSnap(t, db), next, false)
	if err != nil || len(lost) > 0 || strings.Join(stmts, ";\n") != `ALTER DOMAIN "archive"."code" DROP CONSTRAINT "code_len";
ALTER TYPE "archive"."pair" ADD ATTRIBUTE "c" date;
ALTER DOMAIN "archive"."code" SET DEFAULT 'none'::text;
ALTER DOMAIN "archive"."code" SET NOT NULL;
ALTER TYPE "archive"."mood" ADD VALUE 'great' BEFORE 'good';
ALTER TYPE "archive"."mood" ADD VALUE 'meh' AFTER 'good';
ALTER DOMAIN "archive"."code" ADD CONSTRAINT "code_short" CHECK ((length(VALUE) < 20));
ALTER TABLE "archive"."thing" ALTER COLUMN "note" TYPE character varying(20)` {
		t.Fatalf("plan: %v %v\n%s", err, lost, strings.Join(stmts, ";\n"))
	}
	pgTestMigrate(t, db, next)
	pgTestSame(t, "altered in place", pgTestSnap(t, db), next)
	var row string
	if err := db.QueryRow(ctx, "SELECT format('%s %s %s', (SELECT row(code, p, m)::text FROM archive.thing), (SELECT row(code, p, m)::text FROM public.mine), (SELECT count(*) FROM pg_type WHERE typname = 'code'))").Scan(&row); err != nil || row != `(x,"(a,1,)",good) (y,"(b,2,)",bad) 1` {
		t.Errorf("rows after altering their types in place: %q %v", row, err)
	}

	lossy := want(`CREATE SCHEMA archive;
CREATE DOMAIN archive.code AS text DEFAULT 'none' NOT NULL CONSTRAINT code_short CHECK (length(VALUE) < 20);
CREATE TYPE archive.pair AS (a text, c date);
CREATE TYPE archive.mood AS ENUM ('great', 'good', 'meh', 'bad');
CREATE TABLE archive.thing (id integer PRIMARY KEY, code archive.code, p archive.pair, m archive.mood, note integer);`)
	if err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		lost, err := pgMigrate(ctx, tx, lossy, false)
		if err != nil || fmt.Sprint(lost) != "[attribute archive.pair.b column archive.thing.n column archive.thing.note from character varying(20) to integer]" {
			t.Errorf("data loss: %v %v", lost, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	pgTestSame(t, "after a refused plan", pgTestSnap(t, db), next)

	for sql, wantErr := range map[string]string{
		`CREATE SCHEMA archive; CREATE DOMAIN archive.code AS text; CREATE TYPE archive.pair AS (a text, b integer, c date);
CREATE TYPE archive.mood AS ENUM ('good', 'great', 'meh', 'bad'); CREATE TABLE archive.thing (id integer PRIMARY KEY, code archive.code, p archive.pair, m archive.mood, n integer, note varchar(20));`: "type archive.mood: removing or reordering the values of an enum is not supported",
		`CREATE SCHEMA archive; CREATE DOMAIN archive.code AS integer; CREATE TYPE archive.pair AS (a text, b integer, c date);
CREATE TYPE archive.mood AS ENUM ('great', 'good', 'meh', 'bad'); CREATE TABLE archive.thing (id integer PRIMARY KEY, code archive.code, p archive.pair, m archive.mood, n integer, note varchar(20));`: "domain archive.code: changing its type from text to integer is not supported",
	} {
		if _, _, err := pgPlan(pgTestSnap(t, db), want(sql), false); err == nil || !strings.Contains(err.Error(), wantErr) {
			t.Errorf("plan error %v, want %q", err, wantErr)
		}
	}

	if _, err := s.scratch(ctx, "CREATE SCHEMA archive; CREATE TABLE archive.broken ("); err == nil {
		t.Error("a broken archive.sql made a snapshot")
	}
	var n int
	if err := s.sys.QueryRow(ctx, "SELECT count(*) FROM pg_database WHERE datname LIKE 'archive_scratch_%'").Scan(&n); err != nil || n != 0 {
		t.Errorf("%d scratch databases left: %v", n, err)
	}
}
