package main

// Groups: a group is a row of archive."group" (archive.sql), and a row's access is the id of the group that
// sees it, a foreign key to that row, or null for the whole workspace. Writes may name a group instead of its
// id, in access and in a member's groups, and groupNames makes the name its id. A workspace from before
// groups were rows, when access was a group's name, is brought to them once, as it opens (groupUpgrade).

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var groupNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_]{0,23}$`)

// groupID is the id of the group s names, by its id or its name; nil for "", the whole workspace. With make,
// a name no group has yet becomes a new group, as a write of an owner's makes it.
func (a *Archive) groupID(ctx context.Context, s string, make bool) (*uuid.UUID, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	if id, err := uuid.Parse(s); err == nil {
		return &id, nil
	}
	name := strings.ToLower(s)
	id, err := a.st.db(ctx).GroupByName(ctx, name)
	if errors.Is(err, pgx.ErrNoRows) && make && groupNameRe.MatchString(name) {
		err = a.st.Q(ctx).QueryRow(ctx, `INSERT INTO archive."group" (name) VALUES ($1) RETURNING id`, name).Scan(&id)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, &ValidationError{Type: "group", Errors: []FieldError{{Path: "access", Message: fmt.Sprintf(`no group %q; make it with archive.create("group", { name: %q })`, s, name)}}}
	}
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// groupNames makes each group a write names by its name into its id: its access, and a member's groups, where
// a name no group has yet makes the group, as setting a member's groups by name always has.
func (a *Archive) groupNames(ctx context.Context, tb *Table, in map[string]any) error {
	if s, ok := in["access"].(string); ok && tb.col("access") != nil {
		id, err := a.groupID(ctx, s, false)
		if ve := (*ValidationError)(nil); errors.As(err, &ve) {
			ve.Type = tb.Name
		}
		if err != nil {
			return err
		}
		if id == nil {
			in["access"] = nil
		} else {
			in["access"] = id.String()
		}
	}
	if tb.Schema != "archive" || tb.Name != "member" {
		return nil
	}
	list, ok := in["groups"].([]any)
	if !ok {
		return nil
	}
	out := make([]any, 0, len(list))
	for _, v := range list {
		s, ok := v.(string)
		if !ok {
			out = append(out, v)
			continue
		}
		id, err := a.groupID(ctx, s, true)
		if err != nil {
			var ve *ValidationError
			if errors.As(err, &ve) {
				ve.Type, ve.Errors[0].Path = "member", "groups"
			}
			return err
		}
		if id != nil {
			out = append(out, id.String())
		}
	}
	in["groups"] = out
	return nil
}

// groupUpgrade brings a workspace whose access is a group's name to groups as rows, before archive.sql is
// applied (pgStore.migrate): each name any access, any member's groups, or any default or CHECK of an access
// column says becomes a group, one a name it cannot be named by a name like it; each access column becomes
// the uuid of its group, with the foreign key; member.groups the ids; and each group's role is named by its id,
// held by the same members. A table inherits archive.object by a column of the same type, so each one stops
// inheriting it while its column changes, and inherits it again after. Each table is its own step: one that
// cannot change stays as it was, apart from object, and said, with no policy, so no one sees its rows until it
// is fixed, while the rest move on. It does nothing to a workspace that has groups as rows.
func groupUpgrade(ctx context.Context, tx pgx.Tx, schema string, prefix string) ([]string, error) {
	var fresh bool
	if err := tx.QueryRow(ctx, "SELECT to_regclass('archive.object') IS NULL").Scan(&fresh); err != nil || fresh {
		return nil, err
	}
	var tables []string
	rows, err := tx.Query(ctx, `SELECT c.oid::regclass::text FROM pg_catalog.pg_class c JOIN pg_catalog.pg_attribute a ON a.attrelid = c.oid
        WHERE c.relnamespace IN ('archive'::regnamespace, 'public'::regnamespace) AND c.relkind IN ('r', 'p') AND NOT c.relispartition
          AND a.attname = 'access' AND NOT a.attisdropped AND a.atttypid = 'text'::regtype AND c.oid <> 'archive.object'::regclass
        ORDER BY 1`)
	if err != nil {
		return nil, err
	}
	if tables, err = pgx.CollectRows(rows, pgx.RowTo[string]); err != nil {
		return nil, err
	}
	var old bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT FROM pg_catalog.pg_attribute WHERE attrelid = to_regclass('archive.member') AND attname = 'groups'
        AND atttypid = 'text[]'::regtype)`).Scan(&old); err != nil {
		return nil, err
	}
	if len(tables) == 0 && !old {
		return nil, nil
	}
	var ran []string
	run := func(q pgx.Tx, st string) error {
		if _, err := q.Exec(ctx, st); err != nil {
			return fmt.Errorf("%s: %w", st, err)
		}
		ran = append(ran, st)
		return nil
	}
	// object's access is text until every table of a workspace from before groups has been brought to them; a
	// table that could not be is all that is left on a later open, and object stays as it is then.
	var objectText, missing bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT FROM pg_catalog.pg_attribute WHERE attrelid = 'archive.object'::regclass AND attname = 'access'
        AND atttypid = 'text'::regtype), to_regclass('archive."group"') IS NULL`).Scan(&objectText, &missing); err != nil {
		return nil, err
	}
	if missing {
		if err := run(tx, `CREATE TABLE archive."group" (id uuid PRIMARY KEY DEFAULT uuidv7(), name text NOT NULL, description text NOT NULL DEFAULT '',
        created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE (name)) INHERITS (archive.object)`); err != nil {
			return nil, err
		}
		if objectText {
			tables = append(tables, `archive."group"`)
		}
	}
	live, err := pgRead(ctx, tx, "archive", "public")
	if err != nil {
		return nil, err
	}

	// Every name: what the rows say, as each table's owner past the row-level security FORCE holds it to.
	names := map[string]bool{}
	lit := regexp.MustCompile(`'((?:[^']|'')*)'::text`)
	defaults := map[string]string{}
	for _, t := range tables {
		sp, err := tx.Begin(ctx)
		if err != nil {
			return nil, err
		}
		_, err = sp.Exec(ctx, "ALTER TABLE "+t+" NO FORCE ROW LEVEL SECURITY")
		var vals []string
		if err == nil {
			var rows pgx.Rows
			if rows, err = sp.Query(ctx, "SELECT DISTINCT access FROM "+t+" WHERE access IS NOT NULL"); err == nil {
				vals, err = pgx.CollectRows(rows, pgx.RowTo[string])
			}
		}
		if rerr := sp.Rollback(ctx); err != nil || rerr != nil {
			return nil, fmt.Errorf("%s: %w", t, errors.Join(err, rerr))
		}
		for _, v := range vals {
			names[v] = true
		}
		var def string
		if err := tx.QueryRow(ctx, `SELECT coalesce((SELECT pg_get_expr(d.adbin, d.adrelid) FROM pg_catalog.pg_attrdef d JOIN pg_catalog.pg_attribute a
            ON a.attrelid = d.adrelid AND a.attnum = d.adnum WHERE d.adrelid = $1::regclass AND a.attname = 'access'), '')`, t).Scan(&def); err != nil {
			return nil, err
		}
		if def != "" {
			defaults[t] = def
			for _, m := range lit.FindAllStringSubmatch(def, -1) {
				names[strings.ReplaceAll(m[1], "''", "'")] = true
			}
		}
	}
	for _, o := range live {
		if o.Kind == "constraint" && o.Type == "c" && strings.Contains(o.Def, "access") {
			for _, m := range lit.FindAllStringSubmatch(o.Def, -1) {
				names[strings.ReplaceAll(m[1], "''", "'")] = true
			}
		}
	}
	if old {
		rows, err := tx.Query(ctx, "SELECT DISTINCT g FROM archive.member, unnest(groups::text[]) g")
		if err != nil {
			return nil, err
		}
		gs, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			return nil, err
		}
		for _, g := range gs {
			names[g] = true
		}
	}

	// Each name its group: the one by that name, or a new one, named by it when it can be, or else like it.
	ids := map[string]uuid.UUID{}
	rows, err = tx.Query(ctx, `SELECT name, id FROM archive."group"`)
	if err != nil {
		return nil, err
	}
	taken := map[string]bool{}
	var n string
	var id uuid.UUID
	if _, err := pgx.ForEachRow(rows, []any{&n, &id}, func() error { ids[n], taken[n] = id, true; return nil }); err != nil {
		return nil, err
	}
	for _, v := range slices.Sorted(maps.Keys(names)) {
		if _, ok := ids[v]; ok {
			continue
		}
		name := groupNameLike(v, taken)
		taken[name] = true
		if err := tx.QueryRow(ctx, `INSERT INTO archive."group" (name) VALUES ($1) RETURNING id`, name).Scan(&id); err != nil {
			return nil, err
		}
		ran = append(ran, fmt.Sprintf(`INSERT INTO archive."group" (name) VALUES (%s)`, pgLiteral(name)))
		ids[v] = id
	}
	for _, st := range []string{
		"CREATE TEMP TABLE group_upgrade (old text PRIMARY KEY, id uuid NOT NULL) ON COMMIT DROP",
		"CREATE FUNCTION pg_temp.group_upgrade_id(text) RETURNS uuid LANGUAGE sql STABLE AS 'SELECT id FROM pg_temp.group_upgrade WHERE old = $1'",
		"CREATE FUNCTION pg_temp.group_upgrade_ids(text[]) RETURNS uuid[] LANGUAGE sql STABLE AS 'SELECT coalesce(array_agg(u.id ORDER BY u.id), ''{}'') FROM unnest($1) g JOIN pg_temp.group_upgrade u ON u.old = g'",
	} {
		if _, err := tx.Exec(ctx, st); err != nil {
			return nil, err
		}
	}
	defer tx.Exec(context.WithoutCancel(ctx), "DROP FUNCTION IF EXISTS pg_temp.group_upgrade_id(text), pg_temp.group_upgrade_ids(text[])")
	for old, id := range ids {
		if _, err := tx.Exec(ctx, "INSERT INTO pg_temp.group_upgrade VALUES ($1, $2)", old, id); err != nil {
			return nil, err
		}
	}
	uuidOf := func(src string) string {
		return lit.ReplaceAllStringFunc(src, func(m string) string {
			v := strings.ReplaceAll(lit.FindStringSubmatch(m)[1], "''", "'")
			if id, ok := ids[v]; ok {
				return "'" + id.String() + "'::uuid"
			}
			return m
		})
	}

	// archive.can of a uuid, which the policies call, from archive.sql itself, so archive.sql finds it as it is.
	for _, fn := range []string{"CREATE FUNCTION archive.group_role(", "CREATE FUNCTION archive.can("} {
		st := schemaStatement(schema, fn)
		if st == "" {
			return nil, fmt.Errorf("archive.sql has no %s", fn)
		}
		if err := run(tx, strings.Replace(st, "CREATE FUNCTION", "CREATE OR REPLACE FUNCTION", 1)); err != nil {
			return nil, err
		}
	}

	// The views that read an access column, and the views that read those, go while the columns change.
	rows, err = tx.Query(ctx, `WITH RECURSIVE v AS (
            SELECT DISTINCT r.ev_class AS oid FROM pg_catalog.pg_depend d JOIN pg_catalog.pg_rewrite r ON r.oid = d.objid
            JOIN pg_catalog.pg_attribute a ON a.attrelid = d.refobjid AND a.attnum = d.refobjsubid
            WHERE d.classid = 'pg_rewrite'::regclass AND d.refclassid = 'pg_class'::regclass AND a.attname = 'access' AND r.ev_class <> d.refobjid
              AND d.refobjid = ANY ($1::regclass[])
            UNION
            SELECT r.ev_class FROM v JOIN pg_catalog.pg_depend d ON d.refobjid = v.oid AND d.classid = 'pg_rewrite'::regclass
            JOIN pg_catalog.pg_rewrite r ON r.oid = d.objid WHERE r.ev_class <> v.oid)
        SELECT 'view ' || c.relnamespace::regnamespace::text || '.' || c.relname FROM v JOIN pg_catalog.pg_class c ON c.oid = v.oid ORDER BY c.oid`, tables)
	if err != nil {
		return nil, err
	}
	views, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	for _, k := range slices.Backward(views) {
		if o := live[k]; o != nil {
			if err := run(tx, "DROP VIEW IF EXISTS "+pgIdent(o.Schema, o.Name)+" CASCADE"); err != nil {
				return nil, err
			}
		}
	}

	// Each table: its policies, the CHECKs and default of its access, and object go; its access becomes the
	// uuid of its group, with the foreign key; and they come back, of the uuid.
	var detached, failed []string
	for _, t := range tables {
		var keys []string
		var object bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT FROM pg_catalog.pg_inherits WHERE inhrelid = $1::regclass AND inhparent = 'archive.object'::regclass)`, t).Scan(&object); err != nil {
			return nil, err
		}
		var schemaName, tableName string
		if err := tx.QueryRow(ctx, `SELECT relnamespace::regnamespace::text, relname::text FROM pg_catalog.pg_class WHERE oid = $1::regclass`, t).Scan(&schemaName, &tableName); err != nil {
			return nil, err
		}
		for k, o := range live {
			if o.Schema == schemaName && o.Of == tableName && (o.Kind == "policy" || o.Kind == "constraint" && o.Type == "c" && strings.Contains(o.Def, "access")) {
				keys = append(keys, k)
			}
		}
		slices.Sort(keys)
		def := defaults[t]
		mine := ran
		sp, err := tx.Begin(ctx)
		if err != nil {
			return nil, err
		}
		step := func() error {
			for _, k := range keys {
				if err := run(sp, live.drop(live[k])); err != nil {
					return err
				}
			}
			if def != "" {
				if err := run(sp, "ALTER TABLE "+t+" ALTER COLUMN access DROP DEFAULT"); err != nil {
					return err
				}
			}
			if object {
				if err := run(sp, "ALTER TABLE "+t+" NO INHERIT archive.object"); err != nil {
					return err
				}
			}
			if err := run(sp, "ALTER TABLE "+t+" ALTER COLUMN access TYPE uuid USING pg_temp.group_upgrade_id(access)"); err != nil {
				return err
			}
			if def != "" {
				if err := run(sp, "ALTER TABLE "+t+" ALTER COLUMN access SET DEFAULT "+uuidOf(def)); err != nil {
					return err
				}
			}
			if t != "archive.upload" && t != "archive.secret" {
				if err := run(sp, "ALTER TABLE "+t+" ADD CONSTRAINT "+pgIdent(tableName+"_access_fkey")+` FOREIGN KEY (access) REFERENCES archive."group"(id) ON DELETE RESTRICT`); err != nil {
					return err
				}
			}
			for _, k := range keys {
				o := *live[k]
				o.Def = uuidOf(o.Def)
				if err := run(sp, live.create(&o)); err != nil {
					return err
				}
			}
			return nil
		}
		if err := step(); err != nil {
			ran = mine
			if rerr := sp.Rollback(ctx); rerr != nil {
				return nil, rerr
			}
			fmt.Fprintf(os.Stderr, "archive: %s keeps its access as text, and no one sees its rows until it is fixed: %v\n", t, err)
			failed = append(failed, t)
			// It stops inheriting object, so object's access can change, and its policies go with archive.can of text.
			if sp, err = tx.Begin(ctx); err != nil {
				return nil, err
			}
			for _, k := range keys {
				if live[k].Kind == "policy" {
					if err := run(sp, live.drop(live[k])); err != nil {
						return nil, err
					}
				}
			}
			if object {
				if err := run(sp, "ALTER TABLE "+t+" NO INHERIT archive.object"); err != nil {
					return nil, err
				}
			}
			if err := sp.Commit(ctx); err != nil {
				return nil, err
			}
			continue
		}
		if err := sp.Commit(ctx); err != nil {
			return nil, err
		}
		if object {
			detached = append(detached, t)
		}
	}
	if objectText {
		if err := run(tx, "ALTER TABLE ONLY archive.object ALTER COLUMN access TYPE uuid USING NULL::uuid"); err != nil {
			return nil, err
		}
	}
	for _, t := range detached {
		if err := run(tx, "ALTER TABLE "+t+" INHERIT archive.object"); err != nil {
			return nil, err
		}
	}
	if err := run(tx, "DROP FUNCTION IF EXISTS archive.can(text)"); err != nil {
		return nil, err
	}

	// The views, oldest first, each its own step: one that cannot read the uuid is said and stays gone.
	for _, k := range views {
		o := live[k]
		if o == nil {
			continue
		}
		mine := ran
		err := pgx.BeginFunc(ctx, tx, func(sp pgx.Tx) error {
			if err := run(sp, "CREATE VIEW "+pgIdent(o.Schema, o.Name)+" "+uuidOf(o.Def)); err != nil {
				return err
			}
			if o.Comment != "" {
				if err := run(sp, "COMMENT ON VIEW "+pgIdent(o.Schema, o.Name)+" IS "+pgLiteral(o.Comment)); err != nil {
					return err
				}
			}
			grants, err := pgGrants(live.target(o), o, "", o.ACL)
			for _, st := range grants {
				err = errors.Join(err, run(sp, st))
			}
			return err
		})
		if err != nil {
			ran = mine
			fmt.Fprintf(os.Stderr, "archive: view %s.%s is gone: it could not be made again over access as a group's id: %v\n", o.Schema, o.Name, err)
		}
	}

	// The members' groups, and the roles: by id, held by the same members, and none by name.
	if old {
		if err := run(tx, "ALTER TABLE archive.member ALTER COLUMN groups DROP DEFAULT, ALTER COLUMN groups TYPE uuid[] USING pg_temp.group_upgrade_ids(groups), ALTER COLUMN groups SET DEFAULT '{}'"); err != nil {
			return nil, err
		}
	}
	for v := range ids {
		if groupNameRe.MatchString(v) {
			if err := run(tx, "DROP ROLE IF EXISTS "+pgIdent(prefix+"_g_"+v)); err != nil {
				return nil, err
			}
		}
	}
	if err := run(tx, `DO $$ DECLARE g uuid; m record; BEGIN
            FOR g IN SELECT id FROM archive."group" LOOP
                IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = archive.group_role(g)) THEN
                    EXECUTE format('CREATE ROLE %I NOLOGIN', archive.group_role(g));
                END IF;
            END LOOP;
            FOR m IN SELECT x.id, g2 FROM archive.member x, unnest(x.groups) g2 LOOP
                EXECUTE format('GRANT %I TO %I', archive.group_role(m.g2), `+pgLiteral(prefix+"_m")+` || translate(rtrim(encode(uuid_send(m.id), 'base64'), '='), '+/', '-_'));
            END LOOP;
        END $$`); err != nil {
		return nil, err
	}
	if len(failed) > 0 {
		ran = append(ran, "-- "+strings.Join(failed, ", ")+" kept access as text")
	}
	return ran, nil
}

// groupNameLike is a group name for v that no group in taken has: v when it is one, or else v made one,
// lowercase letters, digits and _, with a number when that is taken.
func groupNameLike(v string, taken map[string]bool) string {
	if groupNameRe.MatchString(v) && !taken[v] {
		return v
	}
	b := strings.Trim(regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(strings.ToLower(v), "_"), "_")
	if b == "" {
		b = "group"
	}
	if len(b) > 20 {
		b = b[:20]
	}
	name := b
	for i := 2; taken[name]; i++ {
		name = fmt.Sprintf("%s_%d", b, i)
	}
	return name
}

// schemaStatement is the statement of sql that starts with head: through the $$; that closes a function's
// body, or else through its ;.
func schemaStatement(sql, head string) string {
	i := strings.Index(sql, head)
	if i < 0 {
		return ""
	}
	rest := sql[i:]
	if j := strings.Index(rest, "$$"); j >= 0 && !strings.Contains(rest[:j], ";") {
		if k := strings.Index(rest[j+2:], "$$;"); k >= 0 {
			return rest[:j+2+k+2]
		}
	}
	if k := strings.Index(rest, ";"); k >= 0 {
		return rest[:k]
	}
	return ""
}
