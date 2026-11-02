package main

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/Mulholland-Inc/archive/generated"
	"github.com/jackc/pgx/v5"
)

// pgObject is one object of a schema as Postgres renders it from pg_catalog: a table, view, sequence, type,
// domain or function, or a column, constraint, index, trigger or policy of one, or the schema itself.
type pgObject struct {
	Kind, Schema string
	Of           string // the table, view, type or domain it is part of
	Name         string // a function's has its arguments
	ID           string // its oid in its database, which a rename keeps
	Def          string // what defines it, as Postgres renders it: a column's type, default and NOT NULL, an index's CREATE INDEX
	Type         string // a column's type; a constraint's kind
	Comment, ACL string
	RLS, Force   bool
	Num          int      // a column's number
	File         int64    // a table's file, which a rewrite replaces
	Items        []string // an enum's labels; a column's or domain's default and whether it is NOT NULL
	Users        []string // the phTables and phViews of its schema that use a type, domain, function or sequence
}

// pgSchema is a snapshot of schemas, by kind and qualified name.
type pgSchema map[string]*pgObject

func (o *pgObject) key() string { return o.Kind + " " + o.qname() }

func (o *pgObject) qname() string {
	if o.Of != "" {
		return o.Schema + "." + o.Of + "." + o.Name
	}
	return o.Schema + "." + o.Name
}

// def is what a diff compares: its definition, comment and grants.
func (o *pgObject) def() string {
	parts := []string{o.Def}
	if o.RLS {
		parts = append(parts, "ENABLE ROW LEVEL SECURITY")
	}
	if o.Force {
		parts = append(parts, "FORCE ROW LEVEL SECURITY")
	}
	if o.Comment != "" {
		parts = append(parts, "COMMENT "+pgLiteral(o.Comment))
	}
	if o.ACL != "" {
		parts = append(parts, "GRANT "+o.ACL)
	}
	return strings.TrimSpace(strings.Join(slices.DeleteFunc(parts, func(s string) bool { return s == "" }), "\n"))
}

// pgReadSQL reads every object of the schemas in $1, each as kind, schema, of, name, id, def, type, comment, acl,
// rls, force, num, file and items. {acl x} is the grants in x, sorted; {ext x} leaves out what an extension owns.
var pgReadSQL = pgMacro(pgMacro(`
SELECT 'schema', n.nspname::text, '', n.nspname::text, 'n' || n.oid, '', '', coalesce(obj_description(n.oid, 'pg_namespace'), ''),
    {acl coalesce(n.nspacl, acldefault('n'::"char", n.nspowner))}, false, false, 0, 0::int8, '{}'::text[] FROM pg_namespace n WHERE n.nspname = ANY($1)
UNION ALL
SELECT 'default acl', n.nspname::text, '', pg_get_userbyid(d.defaclrole) || ' ' || d.defaclobjtype::text, 'd' || d.oid, '', d.defaclobjtype::text, '',
    {acl d.defaclacl}, false, false, 0, 0, '{}' FROM pg_default_acl d JOIN pg_namespace n ON n.oid = d.defaclnamespace WHERE n.nspname = ANY($1)
UNION ALL
SELECT CASE c.relkind WHEN 'v' THEN 'view' WHEN 'S' THEN 'sequence' ELSE 'table' END, n.nspname::text, '', c.relname::text, 'c' || c.oid,
    CASE c.relkind
        WHEN 'v' THEN concat('WITH (' || array_to_string(c.reloptions, ', ') || ') ', 'AS' || chr(10), rtrim(pg_get_viewdef(c.oid, true), ';'))
        WHEN 'S' THEN (SELECT format('AS %s START WITH %s INCREMENT BY %s MINVALUE %s MAXVALUE %s CACHE %s', format_type(q.seqtypid, NULL),
            q.seqstart, q.seqincrement, q.seqmin, q.seqmax, q.seqcache) || CASE WHEN q.seqcycle THEN ' CYCLE' ELSE '' END FROM pg_sequence q WHERE q.seqrelid = c.oid)
        ELSE concat_ws(' ', (SELECT 'PARTITION OF ' || i.inhparent::regclass || ' ' || pg_get_expr(c.relpartbound, c.oid) FROM pg_inherits i WHERE i.inhrelid = c.oid AND c.relispartition),
            (SELECT 'INHERITS (' || string_agg(i.inhparent::regclass::text, ', ' ORDER BY i.inhseqno) || ')' FROM pg_inherits i WHERE i.inhrelid = c.oid AND NOT c.relispartition),
            'PARTITION BY ' || pg_get_partkeydef(c.oid), 'WITH (' || array_to_string(c.reloptions, ', ') || ')') END,
    '', coalesce(obj_description(c.oid, 'pg_class'), ''), {acl coalesce(c.relacl, acldefault((CASE c.relkind WHEN 'S' THEN 's' ELSE 'r' END)::"char", c.relowner))}, c.relrowsecurity, c.relforcerowsecurity, 0, c.relfilenode::int8, '{}'
FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = ANY($1) AND c.relkind IN ('r', 'p', 'v', 'S') AND {ext c.oid}
    AND NOT EXISTS (SELECT FROM pg_depend d WHERE d.classid = 'pg_class'::regclass AND d.objid = c.oid AND d.deptype = 'i')
UNION ALL
SELECT CASE c.relkind WHEN 'c' THEN 'attribute' ELSE 'column' END, n.nspname::text, c.relname::text, a.attname::text, 'c' || c.oid || '.' || a.attnum,
    concat_ws(' ', format_type(a.atttypid, a.atttypmod),
        'COLLATE ' || (SELECT quote_ident(l.collname) FROM pg_collation l WHERE l.oid = a.attcollation AND a.attcollation <> t.typcollation),
        CASE WHEN a.attgenerated <> '' THEN 'GENERATED ALWAYS AS (' || pg_get_expr(d.adbin, d.adrelid) || CASE a.attgenerated WHEN 's' THEN ') STORED' ELSE ') VIRTUAL' END
            ELSE 'DEFAULT ' || pg_get_expr(d.adbin, d.adrelid) END,
        CASE a.attidentity WHEN 'a' THEN 'GENERATED ALWAYS AS IDENTITY' WHEN 'd' THEN 'GENERATED BY DEFAULT AS IDENTITY' END,
        CASE WHEN a.attnotnull THEN 'NOT NULL' END),
    format_type(a.atttypid, a.atttypmod), coalesce(col_description(c.oid, a.attnum), ''), {acl a.attacl}, false, false, a.attnum::int, 0,
    ARRAY[coalesce(pg_get_expr(d.adbin, d.adrelid), ''), a.attnotnull::text, a.attgenerated::text]
FROM pg_attribute a JOIN pg_class c ON c.oid = a.attrelid JOIN pg_namespace n ON n.oid = c.relnamespace JOIN pg_type t ON t.oid = a.atttypid
    LEFT JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
WHERE n.nspname = ANY($1) AND c.relkind IN ('r', 'p', 'c') AND a.attnum > 0 AND NOT a.attisdropped AND a.attislocal AND {ext c.oid}
UNION ALL
SELECT 'constraint', n.nspname::text, coalesce(c.relname, t.typname)::text, k.conname::text, 'k' || k.oid, pg_get_constraintdef(k.oid), k.contype::text,
    coalesce(obj_description(k.oid, 'pg_constraint'), ''), '', false, false, (k.contype <> 'p')::int + (k.contype = 'f')::int, 0, '{}'
FROM pg_constraint k JOIN pg_namespace n ON n.oid = k.connamespace LEFT JOIN pg_class c ON c.oid = k.conrelid LEFT JOIN pg_type t ON t.oid = k.contypid
WHERE n.nspname = ANY($1) AND k.contype NOT IN ('n', 't') AND k.conparentid = 0 AND k.conislocal
UNION ALL
SELECT 'index', n.nspname::text, c.relname::text, i.relname::text, 'c' || i.oid, pg_get_indexdef(i.oid), '', coalesce(obj_description(i.oid, 'pg_class'), ''), '', false, false, 0, 0, '{}'
FROM pg_index x JOIN pg_class i ON i.oid = x.indexrelid JOIN pg_class c ON c.oid = x.indrelid JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = ANY($1) AND NOT i.relispartition AND NOT EXISTS (SELECT FROM pg_constraint k WHERE k.conindid = i.oid AND k.conrelid = c.oid)
UNION ALL
SELECT 'trigger', n.nspname::text, c.relname::text, g.tgname::text, 'g' || g.oid, pg_get_triggerdef(g.oid) || CASE g.tgenabled
        WHEN 'D' THEN format(E';\nALTER TABLE %s DISABLE TRIGGER %I', g.tgrelid::regclass, g.tgname)
        WHEN 'R' THEN format(E';\nALTER TABLE %s ENABLE REPLICA TRIGGER %I', g.tgrelid::regclass, g.tgname)
        WHEN 'A' THEN format(E';\nALTER TABLE %s ENABLE ALWAYS TRIGGER %I', g.tgrelid::regclass, g.tgname) ELSE '' END,
    '', coalesce(obj_description(g.oid, 'pg_trigger'), ''), '', false, false, 0, 0, '{}'
FROM pg_trigger g JOIN pg_class c ON c.oid = g.tgrelid JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = ANY($1) AND NOT g.tgisinternal AND g.tgparentid = 0 AND NOT (n.nspname = 'public' AND g.tgname = 'archive_moved')
UNION ALL
SELECT 'policy', n.nspname::text, c.relname::text, p.polname::text, 'o' || p.oid, concat_ws(' ', CASE WHEN NOT p.polpermissive THEN 'AS RESTRICTIVE' END,
        CASE p.polcmd WHEN 'r' THEN 'FOR SELECT' WHEN 'a' THEN 'FOR INSERT' WHEN 'w' THEN 'FOR UPDATE' WHEN 'd' THEN 'FOR DELETE' END,
        'TO ' || (SELECT string_agg(CASE WHEN r = 0 THEN 'PUBLIC' ELSE quote_ident(pg_get_userbyid(r)) END, ', ' ORDER BY pg_get_userbyid(r)) FROM unnest(p.polroles) r WHERE p.polroles <> '{0}'),
        'USING (' || pg_get_expr(p.polqual, p.polrelid) || ')', 'WITH CHECK (' || pg_get_expr(p.polwithcheck, p.polrelid) || ')'),
    '', coalesce(obj_description(p.oid, 'pg_policy'), ''), '', false, false, 0, 0, '{}'
FROM pg_policy p JOIN pg_class c ON c.oid = p.polrelid JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname = ANY($1)
UNION ALL
SELECT CASE t.typtype WHEN 'd' THEN 'domain' ELSE 'type' END, n.nspname::text, '', t.typname::text, 't' || t.oid, CASE t.typtype
        WHEN 'e' THEN 'ENUM (' || coalesce((SELECT string_agg(quote_literal(e.enumlabel), ', ' ORDER BY e.enumsortorder) FROM pg_enum e WHERE e.enumtypid = t.oid), '') || ')'
        WHEN 'r' THEN (SELECT 'RANGE (SUBTYPE = ' || format_type(r.rngsubtype, NULL) || ')' FROM pg_range r WHERE r.rngtypid = t.oid)
        WHEN 'd' THEN concat_ws(' ', format_type(t.typbasetype, t.typtypmod),
            'COLLATE ' || (SELECT quote_ident(l.collname) FROM pg_collation l JOIN pg_type b ON b.oid = t.typbasetype WHERE l.oid = t.typcollation AND t.typcollation <> b.typcollation),
            'DEFAULT ' || pg_get_expr(t.typdefaultbin, 0), CASE WHEN t.typnotnull THEN 'NOT NULL' END)
        ELSE '' END,
    CASE t.typtype WHEN 'd' THEN format_type(t.typbasetype, t.typtypmod) ELSE '' END, coalesce(obj_description(t.oid, 'pg_type'), ''), {acl coalesce(t.typacl, acldefault('T'::"char", t.typowner))}, false, false, 0, 0,
    CASE t.typtype WHEN 'e' THEN ARRAY(SELECT e.enumlabel::text FROM pg_enum e WHERE e.enumtypid = t.oid ORDER BY e.enumsortorder)
        WHEN 'd' THEN ARRAY[coalesce(pg_get_expr(t.typdefaultbin, 0), ''), t.typnotnull::text] ELSE '{}' END
FROM pg_type t JOIN pg_namespace n ON n.oid = t.typnamespace LEFT JOIN pg_class c ON c.oid = t.typrelid
WHERE n.nspname = ANY($1) AND (t.typtype IN ('e', 'd', 'r') OR t.typtype = 'c' AND c.relkind = 'c') AND {ext t.oid}
UNION ALL
SELECT 'function', n.nspname::text, '', p.proname || '(' || pg_get_function_identity_arguments(p.oid) || ')', 'p' || p.oid, rtrim(pg_get_functiondef(p.oid), E'\n'),
    '', coalesce(obj_description(p.oid, 'pg_proc'), ''), {acl coalesce(p.proacl, acldefault('f'::"char", p.proowner))}, false, false, 0, 0, '{}'
FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace WHERE n.nspname = ANY($1) AND p.prokind = 'f' AND {ext p.oid}`,
	"acl", "coalesce((SELECT string_agg(a::text, ', ' ORDER BY a::text COLLATE \"C\") FROM unnest($1) a), '')"),
	"ext", "NOT EXISTS (SELECT FROM pg_depend x WHERE x.objid = $1 AND x.deptype = 'e')")

func pgMacro(sql, name, repl string) string {
	return regexp.MustCompile(`\{`+name+` ([^}]+)\}`).ReplaceAllString(sql, repl)
}

// pgUsersSQL is, for each type, function or sequence, the tables and views in the schemas in $1 that use it.
const pgUsersSQL = `SELECT DISTINCT CASE d.refclassid WHEN 'pg_proc'::regclass THEN 'p' || d.refobjid WHEN 'pg_class'::regclass THEN 'c' || d.refobjid
        ELSE 't' || coalesce((SELECT y.typelem FROM pg_type y WHERE y.oid = d.refobjid AND y.typcategory = 'A' AND y.typelem <> 0), d.refobjid) END,
    r.relnamespace::regnamespace::text || '.' || r.relname
FROM pg_depend d JOIN pg_class r ON r.oid = CASE d.classid
    WHEN 'pg_class'::regclass THEN coalesce((SELECT x.indrelid FROM pg_index x WHERE x.indexrelid = d.objid), d.objid)
    WHEN 'pg_attrdef'::regclass THEN (SELECT x.adrelid FROM pg_attrdef x WHERE x.oid = d.objid)
    WHEN 'pg_constraint'::regclass THEN (SELECT x.conrelid FROM pg_constraint x WHERE x.oid = d.objid)
    WHEN 'pg_rewrite'::regclass THEN (SELECT x.ev_class FROM pg_rewrite x WHERE x.oid = d.objid)
    WHEN 'pg_trigger'::regclass THEN (SELECT x.tgrelid FROM pg_trigger x WHERE x.oid = d.objid)
    WHEN 'pg_policy'::regclass THEN (SELECT x.polrelid FROM pg_policy x WHERE x.oid = d.objid) END
WHERE d.deptype = 'n' AND d.refclassid IN ('pg_type'::regclass, 'pg_proc'::regclass, 'pg_class'::regclass) AND r.relkind IN ('r', 'p', 'v')
    AND r.relnamespace IN (SELECT oid FROM pg_namespace WHERE nspname = ANY($1))`

// pgRead snapshots the schemas as q sees them; Postgres leaves the names on q's search_path unqualified.
func pgRead(ctx context.Context, q generated.DBTX, schemas ...string) (pgSchema, error) {
	var db string
	if err := q.QueryRow(ctx, "SELECT oid::text FROM pg_catalog.pg_database WHERE datname = current_database()").Scan(&db); err != nil {
		return nil, err
	}
	users := map[string][]string{}
	rows, err := q.Query(ctx, pgUsersSQL, schemas)
	if err != nil {
		return nil, err
	}
	var id, rel string
	if _, err := pgx.ForEachRow(rows, []any{&id, &rel}, func() error { users[id] = append(users[id], rel); return nil }); err != nil {
		return nil, err
	}
	s := pgSchema{}
	if rows, err = q.Query(ctx, pgReadSQL, schemas); err != nil {
		return nil, err
	}
	var o pgObject
	_, err = pgx.ForEachRow(rows, []any{&o.Kind, &o.Schema, &o.Of, &o.Name, &o.ID, &o.Def, &o.Type, &o.Comment, &o.ACL, &o.RLS, &o.Force, &o.Num, &o.File, &o.Items}, func() error {
		n := o
		n.Users, n.ID = users[o.ID], db+":"+o.ID
		slices.Sort(n.Users)
		s[n.key()] = &n
		return nil
	})
	return s, err
}

// only is the part of s in the schema.
func (s pgSchema) only(schema string) pgSchema {
	return maps.Collect(func(yield func(string, *pgObject) bool) {
		for k, o := range s {
			if o.Schema == schema && !yield(k, o) {
				return
			}
		}
	})
}

// hash is a fingerprint of s, the same in any database with the same schemas.
func (s pgSchema) hash() string {
	h := sha256.New()
	for _, k := range slices.Sorted(maps.Keys(s)) {
		fmt.Fprintf(h, "%s\x00%s\x00", k, s[k].def())
	}
	return hex.EncodeToString(h.Sum(nil))
}

// SchemaChange is an object a migration creates, alters or drops, with its definition before and after.
type SchemaChange struct {
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Action string `json:"action"`
	From   string `json:"from,omitempty"`
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
}

var pgParents = map[string]bool{"table": true, "view": true, "type": true, "domain": true}

// pgCompare is what changed from a to b, leaving out the parts of what is created or dropped as a whole, and
// of that what loses data: a dropped table, column or attribute, and a column whose type changed in a table
// that was rewritten. Objects match by oid within a database, so a rename is a change, and otherwise by name.
func pgCompare(a, b pgSchema) (changes, phDrops []SchemaChange) {
	changes, phDrops = []SchemaChange{}, []SchemaChange{}
	byID, ids := map[string]*pgObject{}, map[string]bool{}
	for _, o := range b {
		byID[o.ID] = o
	}
	for _, o := range a {
		ids[o.ID] = true
	}
	match, matched := map[*pgObject]*pgObject{}, map[*pgObject]bool{}
	for k, o := range a {
		n := byID[o.ID]
		if n == nil || n.Kind != o.Kind {
			if n = b[k]; n != nil && ids[n.ID] && n.ID != o.ID {
				n = nil
			}
		}
		if n != nil {
			match[o], matched[n] = n, true
		}
	}
	whole := map[string]bool{}
	for _, o := range a {
		if match[o] == nil && pgParents[o.Kind] {
			whole[o.Schema+"."+o.Name] = true
		}
	}
	for _, o := range b {
		if !matched[o] && pgParents[o.Kind] {
			whole[o.Schema+"."+o.Name] = true
		}
	}
	name := func(o *pgObject) string { return strings.TrimPrefix(o.qname(), "public.") }
	for _, k := range slices.Sorted(maps.Keys(a)) {
		o, n := a[k], match[a[k]]
		var c SchemaChange
		switch {
		case n == nil && (o.Of == "" || !whole[o.Schema+"."+o.Of]):
			c = SchemaChange{Kind: o.Kind, Name: name(o), Action: "drop", Before: a.render(o)}
			if o.Kind == "table" || (o.Kind == "column" || o.Kind == "attribute") {
				phDrops = append(phDrops, c)
			}
		case n != nil && (o.def() != n.def() || o.Name != n.Name || o.Schema != n.Schema):
			c = SchemaChange{Kind: n.Kind, Name: name(n), Action: "alter", Before: o.def(), After: n.def()}
			if o.Name != n.Name || o.Schema != n.Schema {
				c.From = name(o)
			}
			if c.Before == c.After {
				c.Before, c.After = "", ""
			}
			if o.Kind == "column" && o.Type != n.Type {
				if t, nt := a["table "+o.Schema+"."+o.Of], b["table "+n.Schema+"."+n.Of]; t != nil && nt != nil && t.File != nt.File {
					phDrops = append(phDrops, c)
				}
			}
		default:
			continue
		}
		changes = append(changes, c)
	}
	for _, k := range slices.Sorted(maps.Keys(b)) {
		if o := b[k]; !matched[o] && (o.Of == "" || !whole[o.Schema+"."+o.Of]) {
			changes = append(changes, SchemaChange{Kind: o.Kind, Name: name(o), Action: "create", After: b.render(o)})
		}
	}
	slices.SortStableFunc(changes, func(x, y SchemaChange) int { return cmp.Compare(x.Name, y.Name) })
	return changes, phDrops
}

var pgOrder = map[string]int{"attribute": 1, "column": 1, "constraint": 2, "index": 3, "policy": 4, "trigger": 5}

// render is o as DDL, with its parts: a table with its columns, constraints, indexes, policies and triggers.
func (s pgSchema) render(o *pgObject) string {
	var parts []*pgObject
	for _, p := range s {
		if p.Of == o.Name && p.Schema == o.Schema && pgParents[o.Kind] {
			parts = append(parts, p)
		}
	}
	slices.SortFunc(parts, func(x, y *pgObject) int {
		return cmp.Or(cmp.Compare(pgOrder[x.Kind], pgOrder[y.Kind]), cmp.Compare(x.Num, y.Num), cmp.Compare(x.Name, y.Name))
	})
	q := pgIdent(o.Schema, o.Name)
	var out, cols, comments []string
	for _, p := range append([]*pgObject{o}, parts...) {
		if p.Comment != "" {
			comments = append(comments, "COMMENT ON "+s.target(p)+" IS "+pgLiteral(p.Comment))
		}
		switch {
		case p.Kind == "column" || p.Kind == "attribute":
			cols = append(cols, pgIdent(p.Name)+" "+p.Def)
		case p.Kind == "constraint" && o.Kind == "domain":
			cols = append(cols, "CONSTRAINT "+pgIdent(p.Name)+" "+p.Def)
		case p != o:
			out = append(out, s.create(p))
		}
	}
	var head string
	switch o.Kind {
	case "table":
		head = "CREATE TABLE " + q
		if len(cols) > 0 || !strings.HasPrefix(o.Def, "PARTITION OF") {
			head += " (\n\t" + strings.Join(cols, ",\n\t") + "\n)"
		}
		head = strings.TrimSpace(head + " " + o.Def)
		var rls []string
		if o.RLS {
			rls = append(rls, "ALTER TABLE "+q+" ENABLE ROW LEVEL SECURITY")
		}
		if o.Force {
			rls = append(rls, "ALTER TABLE "+q+" FORCE ROW LEVEL SECURITY")
		}
		out = append(rls, out...)
	case "view", "sequence":
		head = "CREATE " + strings.ToUpper(o.Kind) + " " + q + " " + o.Def
	case "type":
		if o.Def == "" {
			head = "CREATE TYPE " + q + " AS (\n\t" + strings.Join(cols, ",\n\t") + "\n)"
		} else {
			head = "CREATE TYPE " + q + " AS " + o.Def
		}
	case "domain":
		head = "CREATE DOMAIN " + q + " AS " + strings.Join(append([]string{o.Def}, cols...), "\n\t")
	case "function":
		head = o.Def
	case "schema", "default acl":
		return ""
	default:
		return o.def()
	}
	return strings.Join(append(append([]string{head}, comments...), out...), ";\n") + ";\n"
}

// create is the statement that creates o, a constraint, index, policy or trigger.
func (s pgSchema) create(o *pgObject) string {
	switch o.Kind {
	case "constraint":
		if s["domain "+o.Schema+"."+o.Of] != nil {
			return "ALTER DOMAIN " + pgIdent(o.Schema, o.Of) + " ADD CONSTRAINT " + pgIdent(o.Name) + " " + o.Def
		}
		return "ALTER TABLE " + pgIdent(o.Schema, o.Of) + " ADD CONSTRAINT " + pgIdent(o.Name) + " " + o.Def
	case "policy":
		return strings.TrimSpace("CREATE POLICY " + pgIdent(o.Name) + " ON " + pgIdent(o.Schema, o.Of) + " " + o.Def)
	}
	return o.Def
}

// target names o after COMMENT ON, GRANT ... ON and DROP.
func (s pgSchema) target(o *pgObject) string {
	rel := pgIdent(o.Schema, o.Of)
	switch o.Kind {
	case "column", "attribute":
		return "COLUMN " + rel + "." + pgIdent(o.Name)
	case "constraint":
		if s["domain "+o.Schema+"."+o.Of] != nil {
			return "CONSTRAINT " + pgIdent(o.Name) + " ON DOMAIN " + rel
		}
		return "CONSTRAINT " + pgIdent(o.Name) + " ON " + rel
	case "trigger", "policy":
		return strings.ToUpper(o.Kind) + " " + pgIdent(o.Name) + " ON " + rel
	case "function":
		name, args, _ := strings.Cut(o.Name, "(")
		return "FUNCTION " + pgIdent(o.Schema, name) + "(" + args
	case "schema":
		return "SCHEMA " + pgIdent(o.Name)
	}
	return strings.ToUpper(o.Kind) + " " + pgIdent(o.Schema, o.Name)
}

// ddl is the DDL of the schema by table or view, each with the types, domains, functions and sequences that
// only it uses, of the tables, or of the views when no table does; the others are on their own.
func (s pgSchema) ddl(schema string) map[string]string {
	out := map[string]string{}
	rank := map[string]int{"domain": 0, "type": 0, "sequence": 1, "function": 2, "table": 3, "view": 3}
	objs := slices.Collect(maps.Values(s))
	slices.SortFunc(objs, func(x, y *pgObject) int {
		return cmp.Or(cmp.Compare(rank[x.Kind], rank[y.Kind]), cmp.Compare(x.Name, y.Name))
	})
	for _, o := range objs {
		if _, ok := rank[o.Kind]; !ok || o.Schema != schema {
			continue
		}
		owner, _, _ := strings.Cut(o.Name, "(")
		users := slices.DeleteFunc(slices.Clone(o.Users), func(u string) bool { return s["table "+u] == nil })
		if len(users) == 0 {
			users = o.Users
		}
		if o.Kind != "table" && o.Kind != "view" && len(users) == 1 {
			if t, ok := strings.CutPrefix(users[0], schema+"."); ok {
				owner = t
			}
		}
		out[owner] += s.render(o)
	}
	return out
}

// The phases of a plan, in the order it runs them.
const (
	phSchemas = iota
	phDrops
	phTypes
	phFunctions
	phDomains
	phTables
	phViews
	phKeys
	phIndexes
	phFkeys
	phTriggers
	phPolicies
	phRemoves
	phUnfunctions
	phUntypes
	phGrants
	phases
)

// pgPlan is the DDL that brings live to want, both read with an empty search_path, for Archive's own schema: it
// does what archive.sql needs and refuses the rest. Types and domains are altered, never dropped and created,
// since user tables use them. lost is what would lose data; the plan leaves it out, unless drop, for tests.
func pgPlan(live, want pgSchema, drop bool) (stmts, lost []string, err error) {
	var ph [phases][]string
	add := func(p int, st string) { ph[p] = append(ph[p], st) }
	fail := func(o *pgObject, what string) {
		if err == nil {
			err = fmt.Errorf("%s %s: %s is not supported; change archive.sql another way, or teach pgPlan in src/pgschema.go", o.Kind, o.qname(), what)
		}
	}
	// whole is whether o is part of a table or type that s has and other does not.
	whole := func(s, other pgSchema, o *pgObject) bool {
		for _, k := range []string{"table ", "type "} {
			if o.Of != "" && s[k+o.Schema+"."+o.Of] != nil && other[k+o.Schema+"."+o.Of] == nil {
				return true
			}
		}
		return false
	}
	phase := map[string]int{"type": phTypes, "domain": phTypes, "function": phFunctions, "table": phTables, "column": phTables, "attribute": phTypes,
		"view": phViews, "index": phIndexes, "trigger": phTriggers, "policy": phPolicies, "schema": phSchemas}
	part := func(n *pgObject) int {
		switch {
		case n.Kind == "constraint" && want["domain "+n.Schema+"."+n.Of] != nil:
			return phDomains
		case n.Kind == "constraint" && n.Type == "f":
			return phFkeys
		case n.Kind == "constraint":
			return phKeys
		}
		return phase[n.Kind]
	}
	all := maps.Clone(live)
	maps.Copy(all, want)
	for _, k := range slices.Sorted(maps.Keys(all)) {
		o, n := live[k], want[k]
		switch {
		case o == nil:
			switch n.Kind {
			case "schema":
				add(phSchemas, "CREATE SCHEMA "+pgIdent(n.Name))
			case "table", "type":
				add(phase[n.Kind], strings.SplitN(want.render(n), ";\n", 2)[0])
				if n.RLS {
					add(phTables, "ALTER TABLE "+pgIdent(n.Schema, n.Name)+" ENABLE ROW LEVEL SECURITY")
				}
				if n.Force {
					add(phTables, "ALTER TABLE "+pgIdent(n.Schema, n.Name)+" FORCE ROW LEVEL SECURITY")
				}
			case "domain":
				add(phTypes, "CREATE DOMAIN "+pgIdent(n.Schema, n.Name)+" AS "+n.Def)
			case "function":
				add(phFunctions, n.Def)
			case "view":
				add(phViews, "CREATE VIEW "+pgIdent(n.Schema, n.Name)+" "+n.Def)
			case "column", "attribute":
				if !whole(want, live, n) {
					add(phase[n.Kind], want.alter(n)+" ADD "+strings.ToUpper(n.Kind)+" "+pgIdent(n.Name)+" "+n.Def)
				}
			case "constraint", "index", "trigger", "policy":
				add(part(n), want.create(n))
			case "default acl":
			default:
				fail(n, "creating it")
			}
			if n.Comment != "" {
				add(phGrants, "COMMENT ON "+want.target(n)+" IS "+pgLiteral(n.Comment))
			}
			if n.ACL != "" {
				st, e := pgGrants(want.target(n), n, "", n.ACL)
				if e != nil {
					fail(n, e.Error())
				}
				ph[phGrants] = append(ph[phGrants], st...)
			}
		case n == nil && whole(live, want, o):
		case n == nil:
			switch o.Kind {
			case "table", "column", "attribute":
				lost = append(lost, o.Kind+" "+o.qname())
				if drop {
					add(map[bool]int{true: phRemoves, false: phTables}[o.Kind == "table"], live.drop(o)+" CASCADE")
				}
			case "constraint", "trigger", "policy", "index", "view":
				add(phDrops, live.drop(o))
			case "function":
				add(phUnfunctions, live.drop(o))
			case "type", "domain":
				add(phUntypes, live.drop(o))
			case "default acl":
				st, e := pgGrants("", o, o.ACL, "")
				if e != nil {
					fail(o, e.Error())
				}
				ph[phGrants] = append(ph[phGrants], st...)
			default:
				fail(o, "dropping it")
			}
		default:
			if o.Def != n.Def || !slices.Equal(o.Items, n.Items) || o.Type != n.Type || o.RLS != n.RLS || o.Force != n.Force {
				switch n.Kind {
				case "function":
					add(phFunctions, n.Def)
				case "view":
					add(phViews, "CREATE OR REPLACE VIEW "+pgIdent(n.Schema, n.Name)+" "+n.Def)
				case "constraint", "index", "trigger", "policy":
					add(phDrops, live.drop(o))
					add(part(n), want.create(n))
				case "table":
					if o.Def != n.Def {
						fail(n, "changing it from "+o.Def+" to "+n.Def)
					}
					if o.RLS != n.RLS {
						add(phTables, "ALTER TABLE "+pgIdent(n.Schema, n.Name)+map[bool]string{true: " ENABLE", false: " DISABLE"}[n.RLS]+" ROW LEVEL SECURITY")
					}
					if o.Force != n.Force {
						add(phTables, "ALTER TABLE "+pgIdent(n.Schema, n.Name)+map[bool]string{true: " FORCE", false: " NO FORCE"}[n.Force]+" ROW LEVEL SECURITY")
					}
				case "type":
					if !strings.HasPrefix(n.Def, "ENUM") || !strings.HasPrefix(o.Def, "ENUM") {
						fail(n, "changing it from "+o.Def+" to "+n.Def)
						break
					}
					i := 0
					for j, l := range n.Items {
						if i < len(o.Items) && o.Items[i] == l {
							i++
						} else if !slices.Contains(o.Items, l) {
							st := "ALTER TYPE " + pgIdent(n.Schema, n.Name) + " ADD VALUE " + pgLiteral(l)
							if j > 0 {
								st += " AFTER " + pgLiteral(n.Items[j-1])
							} else if len(n.Items) > 1 {
								st += " BEFORE " + pgLiteral(n.Items[1])
							}
							add(phTypes, st)
						}
					}
					if i < len(o.Items) {
						fail(n, "removing or reordering the values of an enum")
					}
				case "domain", "column", "attribute":
					pgAlter(want, o, n, drop, add, fail, &lost)
				case "schema", "default acl":
				default:
					fail(n, "changing it")
				}
			}
			if o.Comment != n.Comment && n.Comment == "" {
				add(phGrants, "COMMENT ON "+want.target(n)+" IS NULL")
			} else if o.Comment != n.Comment {
				add(phGrants, "COMMENT ON "+want.target(n)+" IS "+pgLiteral(n.Comment))
			}
			if o.ACL != n.ACL {
				st, e := pgGrants(want.target(n), n, o.ACL, n.ACL)
				if e != nil {
					fail(n, e.Error())
				}
				ph[phGrants] = append(ph[phGrants], st...)
			}
		}
	}
	for _, p := range ph {
		stmts = append(stmts, p...)
	}
	return stmts, lost, err
}

// pgAlter alters a domain, column or attribute o into n: its type, when no data is lost, default and NOT NULL.
func pgAlter(want pgSchema, o, n *pgObject, drop bool, add func(int, string), fail func(*pgObject, string), lost *[]string) {
	p, alter := phTables, want.alter(n)+" ALTER "+strings.ToUpper(n.Kind)+" "+pgIdent(n.Name)
	if n.Kind == "domain" {
		p, alter = phTypes, "ALTER DOMAIN "+pgIdent(n.Schema, n.Name)
	}
	if n.Kind == "attribute" {
		p = phTypes
	}
	if o.Type != n.Type {
		base, _, _ := strings.Cut(o.Type, "(")
		nbase, _, _ := strings.Cut(n.Type, "(")
		switch {
		case n.Kind == "domain":
			fail(n, "changing its type from "+o.Type+" to "+n.Type)
		case base == nbase || n.Type == "text" && (base == "character varying" || base == "character"):
			add(p, alter+" TYPE "+n.Type)
		default:
			*lost = append(*lost, n.Kind+" "+n.qname()+" from "+o.Type+" to "+n.Type)
			if drop {
				add(p, alter+" TYPE "+n.Type+" USING "+pgIdent(n.Name)+"::"+n.Type)
			}
		}
	}
	if n.Kind == "attribute" {
		return
	}
	switch {
	case o.Items[0] == n.Items[0]:
	case len(n.Items) > 2 && (n.Items[2] != "" || o.Items[2] != ""):
		if n.Items[2] != o.Items[2] {
			fail(n, "changing whether it is generated")
		}
		add(p, alter+" SET EXPRESSION AS ("+n.Items[0]+")")
	case n.Items[0] == "":
		add(p, alter+" DROP DEFAULT")
	default:
		add(p, alter+" SET DEFAULT "+n.Items[0])
	}
	if o.Items[1] != n.Items[1] {
		add(p, alter+map[string]string{"true": " SET", "false": " DROP"}[n.Items[1]]+" NOT NULL")
	}
}

// alter starts the ALTER of what o is part of: a table, type or domain.
func (s pgSchema) alter(o *pgObject) string {
	for _, k := range []string{"domain", "type"} {
		if s[k+" "+o.Schema+"."+o.Of] != nil {
			return "ALTER " + strings.ToUpper(k) + " " + pgIdent(o.Schema, o.Of)
		}
	}
	return "ALTER TABLE " + pgIdent(o.Schema, o.Of)
}

// drop is the statement that drops o.
func (s pgSchema) drop(o *pgObject) string {
	switch o.Kind {
	case "column", "attribute", "constraint":
		return s.alter(o) + " DROP " + strings.ToUpper(o.Kind) + " " + pgIdent(o.Name)
	}
	return "DROP " + s.target(o)
}

var pgPrivileges = map[byte]string{'r': "SELECT", 'a': "INSERT", 'w': "UPDATE", 'd': "DELETE", 'D': "TRUNCATE", 'x': "REFERENCES",
	't': "TRIGGER", 'm': "MAINTAIN", 'X': "EXECUTE", 'U': "USAGE", 'C': "CREATE", 'c': "CONNECT", 'T': "TEMPORARY"}

// phGrants changes the grants on target, or o's default privileges, from was to now, sorted aclitems like
// role=arw/owner.
func pgGrants(target string, o *pgObject, was, now string) ([]string, error) {
	items := func(acl string) map[string]string {
		out := map[string]string{}
		for _, it := range strings.Split(acl, ", ") {
			if who, privs, ok := strings.Cut(it, "="); ok {
				privs, _, _ = strings.Cut(privs, "/")
				out[cmp.Or(strings.Trim(who, `"`), "PUBLIC")] = privs
			}
		}
		return out
	}
	old, cur := items(was), items(now)
	on, prefix := " ON "+strings.TrimPrefix(strings.TrimPrefix(target, "VIEW "), "TABLE "), ""
	if o.Kind == "default acl" {
		role, kind, _ := strings.Cut(o.Name, " ")
		on = " ON " + map[string]string{"r": "TABLES", "S": "SEQUENCES", "f": "FUNCTIONS", "T": "TYPES", "n": "SCHEMAS"}[kind]
		prefix = "ALTER DEFAULT PRIVILEGES FOR ROLE " + pgIdent(role) + " IN SCHEMA " + pgIdent(o.Schema) + " "
	}
	var out []string
	for _, who := range slices.Sorted(maps.Keys(old)) {
		if old[who] != cur[who] {
			out = append(out, prefix+"REVOKE ALL"+on+" FROM "+pgRole(who))
		}
	}
	for _, who := range slices.Sorted(maps.Keys(cur)) {
		if old[who] == cur[who] {
			continue
		}
		var privs []string
		for i := range len(cur[who]) {
			p, ok := pgPrivileges[cur[who][i]]
			if !ok || i+1 < len(cur[who]) && cur[who][i+1] == '*' {
				return nil, fmt.Errorf("granting %q", cur[who])
			}
			privs = append(privs, p)
		}
		out = append(out, prefix+"GRANT "+strings.Join(privs, ", ")+on+" TO "+pgRole(who))
	}
	return out, nil
}

func pgRole(who string) string {
	if who == "PUBLIC" {
		return who
	}
	return pgIdent(who)
}

// ours is the part of s that archive.sql makes: the archive schema, and the default privileges on public.
func (s pgSchema) ours() pgSchema {
	return maps.Collect(func(yield func(string, *pgObject) bool) {
		for k, o := range s {
			if (o.Schema == "archive" || o.Kind == "default acl") && !yield(k, o) {
				return
			}
		}
	})
}
