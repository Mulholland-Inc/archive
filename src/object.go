package main

// Objects: every row has a name. Each table of the public schema inherits archive.object (archive.sql), which
// gives it id, name, access, created_at and updated_at, and each of Archive's own tables the catalog shows does
// too, named as it is written (archive.object_name). name is what every screen titles a row by, and every link
// to it, so no screen falls back on a column it guesses or on the row's id. Check holds every table and view
// to it, schemaDerive gives a new table what it lacks of it, and objectUpgrade brings a workspace's tables and
// views from before it there, once, when the workspace opens.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/Mulholland-Inc/archive/generated"
	"github.com/jackc/pgx/v5"
)

// objectText says whether c can be the name archive.object gives a table: plain text, written, not computed.
func objectText(c *Column) bool {
	return c.Schema == "pg_catalog" && c.Type == "text" && !c.List && !c.Generated
}

// objectFits says whether t can inherit archive.object once schemaDerive added what it lacks: each column it
// has of object's is of object's type, NOT NULL where object's is.
func objectFits(t *Table) bool {
	for _, w := range []struct {
		name, typ string
		notNull   bool
	}{{"id", "uuid", true}, {"name", "text", false}, {"access", "text", false}, {"created_at", "timestamptz", true}, {"updated_at", "timestamptz", true}} {
		c := t.col(w.name)
		if c != nil && (c.Schema != "pg_catalog" || c.Type != w.typ || c.List || c.Generated || w.notNull && !c.NotNull) {
			return false
		}
	}
	return true
}

// objectPreferred are the columns a table's name is filled from when it had none, the first it has: a person's
// first and last name, then what usually names a row.
var objectPreferred = []string{"title", "full_name", "display_name", "label", "subject", "company_name", "company", "number", "code", "email", "slug"}

// objectSources are the columns, or fields of one, that a name made of t's row is made of: first and last name,
// a column objectPreferred names, an archive.name's full name, or else its first text column. None when it has
// no text to name a row by.
func objectSources(cat *Catalog, t *Table) []string {
	usable := func(c *Column) bool {
		return c != nil && !c.List && c.Name != "name" && c.Name != "access" && c.Name != "search" && !strings.HasSuffix(c.Name, "_source") &&
			cat.texty(c.Schema, c.Type) && cat.secret(c) == nil
	}
	if usable(t.col("first_name")) && usable(t.col("last_name")) {
		return []string{"first_name", "last_name"}
	}
	for _, n := range objectPreferred {
		if usable(t.col(n)) {
			return []string{n}
		}
	}
	for _, c := range t.Columns {
		if c.Schema == "archive" && c.Type == "name" && !c.List {
			return []string{c.Name + ".full"}
		}
	}
	for _, c := range t.Columns {
		if usable(c) {
			return []string{c.Name}
		}
	}
	return nil
}

// objectMade is the SQL that makes a name of a row of the table aliased t from sources, as name_follows does.
func objectMade(sources []string) string {
	parts := make([]string, len(sources))
	for i, s := range sources {
		col, field, ok := strings.Cut(s, ".")
		parts[i] = "nullif(btrim(t." + pgIdent(col) + "::text), '')"
		if ok {
			parts[i] = "nullif(btrim((t." + pgIdent(col) + ")." + pgIdent(field) + "::text), '')"
		}
	}
	return "coalesce(nullif(concat_ws(' ', " + strings.Join(parts, ", ") + "), ''), '')"
}

// objectFree is the first of base, base_2, base_3, ... that t has no column of.
func objectFree(t *Table, base string) string {
	name := base
	for i := 2; t.col(name) != nil; i++ {
		name = fmt.Sprintf("%s_%d", base, i)
	}
	return name
}

// objectUpgrade brings the tables and views of a workspace from before every row had a name to archive.object,
// once, when it opens. A table gets a name: the one it has, made text and NOT NULL, or else a new one, filled
// from objectSources and kept made of them by archive.name_follows; a name that cannot be text, like an
// archive.name, keeps its values under another column, which the new name follows. Then it inherits object.
// A view gets a name column: the name of the row of the tables it reads whose id is its row's id. Each table
// and view is its own step, so one that cannot change is left as it was, and said, while the rest move on;
// what it ran is kept as a revision by archive.
func (a *Archive) objectUpgrade(ctx context.Context) error {
	var ran []string
	err := a.st.System(ctx, func(ctx context.Context) error {
		tx := ctx.Value(storeTxKey{}).(storeTx).tx
		q := generated.New(tx)
		if err := q.AdvisoryXactLock(ctx, storeLock); err != nil {
			return err
		}
		if err := q.SetLocal(ctx, "search_path", "public"); err != nil {
			return err
		}
		cat, err := schemaCatalog(ctx, tx)
		if err != nil {
			return err
		}
		derived, err := schemaDerived(ctx, tx, cat)
		if err != nil {
			return err
		}
		step := func(what string, fn func(run func(string) error) error) error {
			sp, err := tx.Begin(ctx)
			if err != nil {
				return err
			}
			var did []string
			run := func(st string) error {
				if _, err := sp.Exec(ctx, st); err != nil {
					return fmt.Errorf("%s: %w", st, err)
				}
				did = append(did, st)
				return nil
			}
			if err := fn(run); err != nil {
				fmt.Fprintf(os.Stderr, "archive: workspace %s: %s stays as it was: %v\n", a.ws.ID, what, err)
				return sp.Rollback(ctx)
			}
			ran = append(ran, did...)
			return sp.Commit(ctx)
		}
		for _, name := range cat.names() {
			t := cat.Tables[name]
			if t.Schema != "public" || t.Kind != "table" || t.Object || t.Partitioned && t.col("name") != nil {
				continue
			}
			// With what its name is filled from, or failing that with an empty name, which the app shows as
			// untitled: a table nothing can name is still a table every screen can show.
			for _, fill := range []bool{true, false} {
				if err := step("table "+name, func(run func(string) error) error {
					return objectAttach(ctx, tx, cat, t, derived[name], fill, run)
				}); err != nil {
					return err
				}
				if cur, err := schemaCatalog(ctx, tx); err != nil || cur.Tables[name] == nil || cur.Tables[name].Object || t.Partitioned {
					break
				}
			}
		}
		// One view at a time, oldest first, so a view of a view reads its name once that one has it.
		tried := map[string]bool{}
		for {
			views, err := objectViews(ctx, tx)
			if err != nil {
				return err
			}
			i := slices.IndexFunc(views, func(v objectView) bool { return !tried[v.name] })
			if i < 0 {
				break
			}
			v := views[i]
			tried[v.name] = true
			if err := step("view "+v.name, func(run func(string) error) error { return run(v.ddl) }); err != nil {
				return err
			}
		}
		if len(ran) == 0 {
			return nil
		}
		sj, _ := json.Marshal(ran)
		_, err = tx.Exec(ctx, "INSERT INTO archive.revision (hash, author, statements) VALUES ($1, 'archive', $2)", a.st.hash(), sj)
		return err
	})
	if err != nil || len(ran) == 0 {
		return err
	}
	return a.reload(ctx)
}

// objectAttach gives the public table t its name and makes it inherit archive.object, by running each
// statement. With fill it fills a new name from what the row says, and keeps it so.
func objectAttach(ctx context.Context, tx pgx.Tx, cat *Catalog, t *Table, derived, fill bool, run func(string) error) error {
	id := pgIdent("public", t.Name)
	sources := objectSources(cat, t)
	name := t.col("name")
	if name != nil && !objectText(name) {
		switch {
		case name.Schema == "pg_catalog" && (name.Type == "varchar" || name.Type == "bpchar") && !name.List && !name.Generated:
			if err := run("ALTER TABLE " + id + " ALTER COLUMN name TYPE text"); err != nil {
				return err
			}
		default:
			// A name that cannot be text keeps its values under another column, which the new one is made of.
			to := objectFree(t, "name_value")
			if name.Schema == "archive" && name.Type == "name" && !name.List {
				to = objectFree(t, "full_name")
			}
			if err := run("ALTER TABLE " + id + " RENAME COLUMN name TO " + pgIdent(to)); err != nil {
				return err
			}
			sources = []string{to}
			if name.Schema == "archive" && name.Type == "name" && !name.List {
				sources = []string{to + ".full"}
			}
			name = nil
		}
	}
	if !fill {
		sources = nil
	}
	// The rows are filled as their table's owner, past the row-level security that FORCE holds it to too.
	unforce := func() error { return run("ALTER TABLE " + id + " NO FORCE ROW LEVEL SECURITY") }
	reforce := func() error { return run("ALTER TABLE " + id + " FORCE ROW LEVEL SECURITY") }
	if name == nil {
		if err := run("ALTER TABLE " + id + " ADD COLUMN name text NOT NULL DEFAULT ''"); err != nil {
			return err
		}
		if len(sources) > 0 {
			if err := unforce(); err != nil {
				return err
			}
			if err := run("UPDATE " + id + " t SET name = " + objectMade(sources)); err != nil {
				return err
			}
			if t.Force {
				if err := reforce(); err != nil {
					return err
				}
			}
			args := make([]string, len(sources))
			for i, s := range sources {
				args[i] = pgLiteral(s)
			}
			if err := run("CREATE TRIGGER name_follows BEFORE INSERT OR UPDATE ON " + id + " FOR EACH ROW EXECUTE FUNCTION archive.name_follows(" + strings.Join(args, ", ") + ")"); err != nil {
				return err
			}
		}
	} else if !name.NotNull {
		if !name.Default {
			if err := run("ALTER TABLE " + id + " ALTER COLUMN name SET DEFAULT ''"); err != nil {
				return err
			}
		}
		if err := unforce(); err != nil {
			return err
		}
		made := "''"
		if len(sources) > 0 {
			made = objectMade(sources)
		}
		if err := run("UPDATE " + id + " t SET name = " + made + " WHERE name IS NULL"); err != nil {
			return err
		}
		if t.Force {
			if err := reforce(); err != nil {
				return err
			}
		}
		if err := run("ALTER TABLE " + id + " ALTER COLUMN name SET NOT NULL"); err != nil {
			return err
		}
	}
	if t.Partitioned {
		return nil
	}
	if err := run("ALTER TABLE " + id + " INHERIT archive.object"); err != nil {
		return err
	}
	if !derived {
		return nil
	}
	// A search over its text columns covers its name too.
	cur, err := schemaCatalog(ctx, tx)
	if err != nil {
		return err
	}
	if nt := cur.Tables[t.Name]; nt != nil {
		return run(schemaSetSearch(t.Name, schemaSearchCols(cur, nt)))
	}
	return nil
}

type objectView struct{ name, ddl string }

// objectViewsSQL is each public view without a name column, or with the empty one objectViews gave a view whose
// tables had no name it could read, oldest first, so a view of a view comes after it: its definition, options,
// whether it has an id and a name, and the tables and views it reads, of the workspace or Archive's own, that
// have an id and a name.
const objectViewsSQL = `SELECT v.relname::text, rtrim(pg_get_viewdef(v.oid, true), ';'), coalesce(array_to_string(v.reloptions, ', '), ''),
    EXISTS (SELECT FROM pg_attribute a WHERE a.attrelid = v.oid AND a.attname = 'id' AND NOT a.attisdropped),
    EXISTS (SELECT FROM pg_attribute a WHERE a.attrelid = v.oid AND a.attname = 'name' AND NOT a.attisdropped),
    coalesce((SELECT array_agg(DISTINCT format('%I.%I', r.relnamespace::regnamespace::text, r.relname)) FROM pg_rewrite w JOIN pg_depend d ON d.classid = 'pg_rewrite'::regclass AND d.objid = w.oid
        JOIN pg_class r ON r.oid = d.refobjid AND d.refclassid = 'pg_class'::regclass
        WHERE w.ev_class = v.oid AND r.oid <> v.oid AND r.relnamespace IN ('public'::regnamespace, 'archive'::regnamespace) AND r.relkind IN ('r', 'p', 'v')
          AND EXISTS (SELECT FROM pg_attribute a WHERE a.attrelid = r.oid AND a.attname = 'id' AND NOT a.attisdropped)
          AND EXISTS (SELECT FROM pg_attribute a WHERE a.attrelid = r.oid AND a.attname = 'name' AND NOT a.attisdropped)), '{}')
FROM pg_class v WHERE v.relnamespace = 'public'::regnamespace AND v.relkind = 'v'
ORDER BY v.oid`

// objectEmpty is where a view objectViews gave an empty name says so, after its other columns, and the end of
// the definition it wrapped.
var objectEmpty = regexp.MustCompile(`(?s)^\s*SELECT .*?,\s*''::text AS name\s+FROM \((.*)\)\s*q$`)

// objectViews are the statements that give each view without a name column one, after its other columns: the
// name of the row of a table or view it reads whose id is the view's row's id, or an empty one when there is none.
// A view given an empty one gets the name it reads when it reads one now.
func objectViews(ctx context.Context, tx pgx.Tx) ([]objectView, error) {
	rows, err := tx.Query(ctx, objectViewsSQL)
	if err != nil {
		return nil, err
	}
	var out []objectView
	var name, def, opts string
	var hasID, hasName bool
	var reads []string
	_, err = pgx.ForEachRow(rows, []any{&name, &def, &opts, &hasID, &hasName, &reads}, func() error {
		if hasName {
			m := objectEmpty.FindStringSubmatch(def)
			if m == nil || !hasID || len(reads) == 0 {
				return nil
			}
			def = m[1]
		}
		expr := "''::text"
		if hasID && len(reads) > 0 {
			parts := make([]string, len(reads))
			for i, r := range reads {
				parts[i] = "(SELECT o.name::text FROM " + r + " o WHERE o.id = q.id)"
			}
			expr = "coalesce(" + strings.Join(parts, ", ") + ", ''::text)"
		}
		with := ""
		if opts != "" {
			with = " WITH (" + opts + ")"
		}
		out = append(out, objectView{name, "CREATE OR REPLACE VIEW " + pgIdent("public", name) + with + " AS\nSELECT q.*, " + expr + " AS name FROM (\n" + def + "\n) q"})
		return nil
	})
	return out, err
}
