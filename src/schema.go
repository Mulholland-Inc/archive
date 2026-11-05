package main

import (
	"cmp"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Mulholland-Inc/archive/generated"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The schema: the Catalog is the tables and types a workspace has, read from Postgres, with what each column
// takes and how a value is cleaned for it. Migrate changes it with DDL, dry run first; pgPlan, at the end,
// diffs archive.sql against the database and plans the DDL that brings Archive's own schema up to date.
type Catalog struct {
	Tables map[string]*Table  `json:"tables"`
	Types  map[string]*PgType `json:"types"`
	stamp  string
}

type Table struct {
	Name        string    `json:"name"`
	Schema      string    `json:"schema"`
	Kind        string    `json:"kind"`
	Description string    `json:"description"`
	Layout      string    `json:"layout,omitempty"`
	By          string    `json:"by,omitempty"`
	RLS         bool      `json:"rls"`
	Force       bool      `json:"force"`
	Invoker     bool      `json:"security_invoker,omitempty"`
	Policy      bool      `json:"policy"`
	Object      bool      `json:"object"`
	Partitioned bool      `json:"partitioned,omitempty"`
	Columns     []*Column `json:"columns"`
	meta        [][2]string
	internal    []string
}

type Column struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Schema      string `json:"schema"`
	Reg         string `json:"reg"`
	List        bool   `json:"list"`
	NotNull     bool   `json:"not_null"`
	Default     bool   `json:"default"`
	Generated   bool   `json:"generated"`
	PK          bool   `json:"pk"`
	Ref         string `json:"ref"`
}

type PgType struct {
	Name        string    `json:"name"`
	Schema      string    `json:"schema"`
	Kind        string    `json:"kind"`
	Description string    `json:"description"`
	Base        string    `json:"base"`
	BaseSchema  string    `json:"base_schema"`
	Enum        []string  `json:"enum"`
	Attrs       []*Column `json:"attrs"`
}

type Hints struct {
	Locale   string
	Country  string
	Currency string
	Zone     string
}

type Candidate struct {
	Value      any     `json:"value"`
	Confidence float64 `json:"confidence"`
}

type FieldError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
	// Line and Column place a compile error in a workflow's source.
	Line   int `json:"line,omitempty"`
	Column int `json:"column,omitempty"`
}

type ValidationError struct {
	Type   string       `json:"type"`
	Errors []FieldError `json:"errors"`
}

func (e *ValidationError) Error() string {
	if len(e.Errors) == 0 {
		return "invalid " + e.Type
	}
	return "invalid " + e.Type + ": " + e.Errors[0].Path + ": " + e.Errors[0].Message
}

func (e *ValidationError) err() error {
	if len(e.Errors) > 0 {
		return e
	}
	return nil
}

var (
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrForbidden    = errors.New("forbidden")
	ErrUnauthorized = errors.New("unauthorized")
	ErrReadOnly     = errors.New("read only")
	ErrBusy         = errors.New("busy")
)

func catalogParse(b []byte) (*Catalog, error) {
	c := &Catalog{}
	if err := json.Unmarshal(b, c); err != nil {
		return nil, fmt.Errorf("catalog: %w", err)
	}
	for _, t := range c.Tables {
		t.Description, t.meta = catalogComment(t.Description)
		for _, kv := range t.meta {
			switch kv[0] {
			case "layout":
				t.Layout = kv[1]
			case "by":
				t.By = kv[1]
			}
		}
		if t.Kind == "view" && t.Layout == "" {
			t.Layout = "table"
		}
		if t.isWorkflow() {
			catalogWorkflow(t)
		}
		if t.Schema == "archive" && t.Name == "connection" {
			catalogConnection(t)
		}
		if t.synced() {
			t.internal = []string{"seen_at"}
			t.Columns = slices.DeleteFunc(t.Columns, func(c *Column) bool { return c.Name == "seen_at" })
		}
		// These of Archive's tables are seen by whose they are, not by a group: the access every object has
		// says nothing of them.
		if t.Schema == "archive" && slices.Contains([]string{"member", "group", "workflow", "workflow_run", "turn", "memory", "known_person", "known_org"}, t.Name) {
			t.internal = append(t.internal, "access")
			t.Columns = slices.DeleteFunc(t.Columns, func(c *Column) bool { return c.Name == "access" })
		}
	}
	return c, nil
}

// catalogWorkflow shows archive.workflow as a person writes it: name, source, on and access, with whose it is,
// its next time and webhook URL to read. What Archive keeps of it (who it runs as, its trigger and cursor) stays
// out of list and get, and its webhook secret becomes its URL (workflowRows).
func catalogWorkflow(t *Table) {
	internal := []string{"user_id", "role", "trigger", "hook", "cursor_at", "cursor_id"}
	t.Columns = slices.DeleteFunc(t.Columns, func(c *Column) bool { return slices.Contains(internal, c.Name) })
	t.internal = slices.DeleteFunc(internal, func(n string) bool { return n == "hook" })
	for _, n := range []string{"next_at", "member"} {
		if c := t.col(n); c != nil {
			c.Generated = true
		}
	}
	t.Columns = append(t.Columns, &Column{Name: "url", Type: "text", Schema: "pg_catalog", Reg: "text", Generated: true,
		Description: "Its webhook URL, when on is { webhook: true }: anyone with it can POST to it, so give it only to the service that calls it"})
}

// catalogConnection shows archive.connection without whose it is, its sealed refresh token (connect/connect.go)
// and its sync's cursor (connection.go).
func catalogConnection(t *Table) {
	t.internal = []string{"member", "token", "cursor"}
	t.Columns = slices.DeleteFunc(t.Columns, func(c *Column) bool { return slices.Contains(t.internal, c.Name) })
}

// synced says whether t is a table Archive syncs from connections (connection.go): one of syncSchema, or the
// people and organizations they name.
func (t *Table) synced() bool {
	_, ok := syncSchema[t.Name]
	return t.Schema == "archive" && (ok || t.Name == "known_person" || t.Name == "known_org")
}

// isWorkflow says whether t is archive.workflow, whose writes go through its write hook (workflow.go).
func (t *Table) isWorkflow() bool { return t.Schema == "archive" && t.Name == "workflow" }

var catalogMetaRe = regexp.MustCompile(`^([a-z][a-z0-9_]*):[ \t]*(\S.*?)\s*$`)

func catalogComment(s string) (string, [][2]string) {
	lines := strings.Split(s, "\n")
	var meta [][2]string
	for len(lines) > 0 {
		m := catalogMetaRe.FindStringSubmatch(lines[len(lines)-1])
		if m == nil {
			break
		}
		meta = append([][2]string{{m[1], m[2]}}, meta...)
		lines = lines[:len(lines)-1]
	}
	return strings.TrimSpace(strings.Join(lines, "\n")), meta
}

var catalogLayouts = []string{"table", "board", "calendar", "list", "map", "gallery", "chart", "timeline", "tree", "stack", "scatter", "year"}

var (
	catalogSystem   = map[string]bool{"id": true, "created_at": true, "updated_at": true}
	catalogReserved = map[string]bool{"files": true, "views": true, "search": true, "schema": true, "types": true,
		"member": true, "group": true, "user": true, "token": true, "file": true, "upload": true, "secret": true, "revision": true, "run": true,
		"workflow": true, "workflow_run": true, "connection": true, "agent": true, "turn": true, "memory": true}
	catalogNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)
	catalogRefRe  = regexp.MustCompile(`(?i)^references\s+([a-z][a-z0-9_]*)\b[\s.;:,-]*`)
)

func (c *Catalog) table(name string) (*Table, error) {
	t, ok := c.Tables[name]
	if !ok {
		return nil, fmt.Errorf("type %q: %w", name, ErrNotFound)
	}
	return t, nil
}

func (c *Catalog) writable(name string) (*Table, error) {
	t, err := c.table(name)
	if err == nil && t.Kind == "view" {
		return nil, fmt.Errorf("%s is a view, and views are read only; write to the tables it selects from: %w", name, ErrReadOnly)
	}
	if err == nil && t.Schema == "archive" && (t.Name == "turn" || t.Name == "memory") {
		return nil, fmt.Errorf("%s is read only: Archive records it as you talk with an agent: %w", t.Name, ErrReadOnly)
	}
	if err == nil && t.Schema == "archive" && t.Name == "workflow_run" {
		return nil, fmt.Errorf("workflow_run is read only: Archive records a run each time a workflow runs: %w", ErrReadOnly)
	}
	if err == nil && t.Schema == "archive" && t.Name == "connection" {
		return nil, fmt.Errorf("connection is read only: connect and disconnect your accounts in Settings, Connectors: %w", ErrReadOnly)
	}
	if err == nil && t.Schema == "archive" && t.Name == "file" {
		return nil, fmt.Errorf("a file is not written as a row: make one with archive.create(name) or archive.copy, change one with archive.edit, and delete one with archive.delete: %w", ErrReadOnly)
	}
	if err == nil && t.synced() {
		return nil, fmt.Errorf("%s is read only: Archive syncs it from your connections: %w", name, ErrReadOnly)
	}
	return t, err
}

func (c *Catalog) names() []string { return specKeys(c.Tables) }

func (t *Table) col(name string) *Column {
	for _, c := range t.Columns {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func (c *Column) writable() bool {
	return !catalogSystem[c.Name] && !c.Generated && c.Type != "tsvector"
}

func (c *Column) required() bool { return c.NotNull && !c.Default && !c.Generated }

func (c *Column) target() string {
	if c.Ref != "" {
		return c.Ref
	}
	if m := catalogRefRe.FindStringSubmatch(c.Description); m != nil && c.List && c.Type == "uuid" {
		return "public." + m[1]
	}
	return ""
}

func (t *Table) hidden() string {
	out := slices.Clone(t.internal)
	for _, c := range t.Columns {
		if c.Type == "tsvector" {
			out = append(out, c.Name)
		}
	}
	return "'{" + strings.Join(out, ",") + "}'::text[]"
}

func (t *Table) ident() string { return pgIdent(t.Schema, t.Name) }

func (t *Table) rowSQL() string { return "to_jsonb(t) - " + t.hidden() }

func (c *Catalog) Check() error {
	ve := &ValidationError{Type: "schema"}
	bad := func(path, msg string) { ve.Errors = append(ve.Errors, FieldError{Path: path, Message: msg}) }
	want := []struct{ name, typ, msg string }{
		{"id", "uuid", "want id uuid PRIMARY KEY DEFAULT uuidv7()"},
		{"name", "text", "want name text NOT NULL, the title every screen shows a row and its links by, which CREATE TABLE ... INHERITS (archive.object) gives it; fill it on every create, or keep it made of other columns with CREATE TRIGGER name_follows BEFORE INSERT OR UPDATE ON <table> FOR EACH ROW EXECUTE FUNCTION archive.name_follows('first_name', 'last_name')"},
		{"access", "uuid", "want access uuid REFERENCES archive.\"group\" ON DELETE RESTRICT, the id of the group that sees the row, null for the whole workspace, which CREATE TABLE ... INHERITS (archive.object) and Archive give it; a table only one group sees has access uuid NOT NULL DEFAULT '<its id>' CHECK (access = '<its id>')"},
		{"created_at", "timestamptz", "want created_at timestamptz NOT NULL DEFAULT now()"},
		{"updated_at", "timestamptz", "want updated_at timestamptz NOT NULL DEFAULT now()"},
		{"search", "tsvector", "want a search tsvector column, like search tsvector GENERATED ALWAYS AS (archive.search_doc(name)) STORED"},
	}
	for _, name := range c.names() {
		t := c.Tables[name]
		if t.Schema != "public" {
			continue
		}
		if !catalogNameRe.MatchString(name) || catalogReserved[name] {
			bad(name, t.Kind+" names are lowercase letters, digits and _, and not one of "+strings.Join(specKeys(catalogReserved), ", "))
		}
		c.checkMeta(t, bad)
		if t.Kind == "view" {
			if !t.Invoker {
				bad(name, "want CREATE VIEW "+name+" WITH (security_invoker = true) AS SELECT ..., so row-level security applies to whoever reads it")
			}
			if col := t.col("id"); col == nil || col.Type != "uuid" || col.List {
				bad(name+".id", "want an id uuid column, like CREATE VIEW "+name+" WITH (security_invoker = true) AS SELECT * FROM <table>")
			}
			if col := t.col("name"); col == nil || col.List {
				bad(name+".name", "want a name column, the title the app shows each row by, like CREATE VIEW "+name+" WITH (security_invoker = true) AS SELECT * FROM <table>, whose name every table has, or SELECT ..., <expression> AS name")
			}
			continue
		}
		if !t.Object && !t.Partitioned {
			bad(name, "want CREATE TABLE "+name+" (<its own columns>) INHERITS (archive.object), which gives it id, name, access, created_at and updated_at; for a table that exists, ALTER TABLE "+name+" INHERIT archive.object")
		}
		for _, w := range want {
			col := t.col(w.name)
			ok := col != nil && col.Type == w.typ && !col.List
			switch w.name {
			case "id":
				ok = ok && col.PK && col.Default
			case "created_at", "updated_at":
				ok = ok && col.NotNull && col.Default
			case "name":
				ok = ok && col.NotNull
			case "access":
				ok = ok && col.Ref == "archive.group"
			}
			if !ok {
				bad(name+"."+w.name, w.msg)
			}
		}
		if !t.RLS || !t.Force || !t.Policy {
			bad(name, "want ENABLE and FORCE ROW LEVEL SECURITY and exactly one policy, CREATE POLICY access ON "+name+" USING (archive.can(access)) WITH CHECK (archive.can(access))")
		}
		for _, col := range t.Columns {
			if tg := col.target(); col.Ref == "" && tg != "" && c.Tables[strings.TrimPrefix(tg, "public.")] == nil {
				bad(name+"."+col.Name, "references unknown table "+strings.TrimPrefix(tg, "public."))
			}
		}
	}
	return ve.err()
}

func (c *Catalog) checkMeta(t *Table, bad func(path, msg string)) {
	keys, what := []string{}, "COMMENT ON TABLE "+t.Name
	if t.Kind == "view" {
		keys, what = []string{"layout", "by"}, "COMMENT ON VIEW "+t.Name
	}
	seen := map[string]bool{}
	for _, kv := range t.meta {
		k, v, path := kv[0], kv[1], t.Name+"."+kv[0]
		switch {
		case k == "icon":
			// Tables and views had icons once; an icon: line from then is ignored.
		case !slices.Contains(keys, k) && len(keys) == 0:
			bad(path, "unknown key "+k+" in the last line of "+what+"; a table's comment is only its description")
		case !slices.Contains(keys, k):
			bad(path, "unknown key "+k+" in the last lines of "+what+"; want description lines, then key: value lines with "+strings.Join(keys, ", "))
		case seen[k]:
			bad(path, k+" is set twice in "+what)
		case k == "layout" && !slices.Contains(catalogLayouts, v):
			bad(path, "unknown layout "+strconv.Quote(v)+"; want "+strings.Join(catalogLayouts, ", "))
		case k == "by" && t.col(v) == nil:
			bad(path, "no column "+strconv.Quote(v)+" in view "+t.Name+"; want one of its columns: "+strings.Join(t.colNames(), ", "))
		}
		seen[k] = true
	}
	if slices.Contains([]string{"board", "calendar", "map", "chart", "timeline", "tree", "stack", "scatter", "year"}, t.Layout) && t.By == "" {
		bad(t.Name+".by", "a "+t.Layout+" view needs a by: <column> line in "+what)
	}
	if c := t.col(t.By); t.Layout == "map" && c != nil && (c.Schema != "archive" || c.Type != "geo") {
		bad(t.Name+".by", "a map view's by: is the archive.geo column it places rows at, not "+strconv.Quote(t.By))
	}
	dates := map[string]string{"timeline": "the date column its rows start on", "year": "the date column it counts its rows on"}
	if col := t.col(t.By); dates[t.Layout] != "" && col != nil {
		s := c.sem(col.Schema, col.Type)
		dated := (col.Schema == "pg_catalog" && col.Type == "timestamp") || (s != nil && (s.Parse == "date" || s.Parse == "instant" || s.Parse == "timestamp"))
		if col.List || !dated {
			bad(t.Name+".by", "a "+t.Layout+" view's by: is "+dates[t.Layout]+", not "+strconv.Quote(t.By))
		}
	}
	if col := t.col(t.By); t.Layout == "tree" && col != nil && (col.Schema != "pg_catalog" || col.Type != "uuid" || col.List) {
		bad(t.Name+".by", "a tree view's by: is the uuid column that links each row to the row it is under, not "+strconv.Quote(t.By))
	}
	numbers := map[string]string{"stack": "the number column of each row's floor", "scatter": "the number or archive.money column its dots run across by"}
	if col := t.col(t.By); numbers[t.Layout] != "" && col != nil {
		number := col.Schema == "pg_catalog" && c.kind(col.Schema, col.Type) == "number"
		money := t.Layout == "scatter" && col.Schema == "archive" && col.Type == "money"
		if col.List || !number && !money {
			bad(t.Name+".by", "a "+t.Layout+" view's by: is "+numbers[t.Layout]+", not "+strconv.Quote(t.By))
		}
	}
}

func (t *Table) colNames() []string {
	var out []string
	for _, c := range t.Columns {
		if c.Type != "tsvector" {
			out = append(out, c.Name)
		}
	}
	return out
}

func (c *Catalog) typ(schema, name string) *PgType { return c.Types[schema+"."+name] }

func (c *Catalog) sem(schema, name string) *semType {
	switch schema {
	case "pg_catalog":
		if s, ok := semLib[semBase[name]]; ok {
			return &s
		}
		return nil
	case "archive":
		if s, ok := semLib[name]; ok {
			return &s
		}
	}
	if t := c.typ(schema, name); t != nil && t.Kind == "d" && t.Base != "" {
		return c.sem(t.BaseSchema, t.Base)
	}
	return nil
}

func (c *Catalog) kind(schema, name string) string {
	if t := c.typ(schema, name); t != nil {
		switch t.Kind {
		case "c":
			return "object"
		case "e":
			return "string"
		case "d":
			return c.kind(t.BaseSchema, t.Base)
		}
	}
	switch name {
	case "int2", "int4", "int8", "numeric", "float4", "float8":
		return "number"
	case "bool":
		return "boolean"
	case "json", "jsonb":
		return ""
	}
	return "string"
}

func (c *Catalog) secret(col *Column) *semType {
	if s := c.sem(col.Schema, col.Type); s != nil && s.Secret {
		return s
	}
	return nil
}

func (c *Catalog) Normalize(tb *Table, in map[string]any, h Hints, full bool) (map[string]any, error) {
	ve := &ValidationError{Type: tb.Name}
	out := map[string]any{}
	for _, k := range specKeys(in) {
		col := tb.col(k)
		switch {
		case col == nil:
			if in[k] != nil {
				specBad(&ve.Errors, k, errors.New("unknown field"))
			}
		case !col.writable():
		case in[k] == nil:
			if col.required() {
				specBad(&ve.Errors, k, errors.New("required"))
			}
			out[k] = nil
		default:
			if v, ok := c.clean(k, col, in[k], h, &ve.Errors); ok {
				out[k] = v
			}
		}
	}
	if full {
		for _, col := range tb.Columns {
			if _, ok := in[col.Name]; ok || !col.writable() {
				continue
			}
			if col.required() {
				specBad(&ve.Errors, col.Name, errors.New("required"))
			}
			out[col.Name] = nil
		}
	}
	if len(ve.Errors) > 0 {
		sort.SliceStable(ve.Errors, func(i, j int) bool { return ve.Errors[i].Path < ve.Errors[j].Path })
		return nil, ve
	}
	return out, nil
}

func (c *Catalog) clean(path string, col *Column, v any, h Hints, errs *[]FieldError) (any, bool) {
	if !col.List {
		return c.cleanOne(path, col.Schema, col.Type, v, h, errs)
	}
	items, ok := specList(v)
	if !ok {
		items = []any{v}
	}
	list, good := make([]any, 0, len(items)), true
	for i, item := range items {
		nv, ok := c.cleanOne(fmt.Sprintf("%s[%d]", path, i), col.Schema, col.Type, item, h, errs)
		list, good = append(list, nv), good && ok
	}
	return list, good
}

func (c *Catalog) cleanOne(path, schema, name string, v any, h Hints, errs *[]FieldError) (any, bool) {
	t := c.typ(schema, name)
	if t != nil && t.Kind == "e" {
		s, _ := v.(string)
		for _, e := range t.Enum {
			if strings.EqualFold(strings.TrimSpace(s), e) {
				return e, true
			}
		}
		specBad(errs, path, fmt.Errorf("must be one of %s", strings.Join(t.Enum, ", ")))
		return nil, false
	}
	kind := c.kind(schema, name)
	if s := c.sem(schema, name); s != nil {
		if s.Secret {
			kind = ""
		}
		nv, err := semNormalize(s, kind, name, v, h)
		if err != nil {
			specBad(errs, path, err)
			return nil, false
		}
		return nv, true
	}
	if t != nil && t.Kind == "c" {
		m, ok := v.(map[string]any)
		if !ok {
			specBad(errs, path, errors.New("must be an object"))
			return nil, false
		}
		out, good := map[string]any{}, true
		for _, k := range specKeys(m) {
			if !slices.ContainsFunc(t.Attrs, func(a *Column) bool { return a.Name == k }) && m[k] != nil {
				specBad(errs, specJoin(path, k), errors.New("unknown field"))
				good = false
			}
		}
		for _, a := range t.Attrs {
			if m[a.Name] == nil {
				continue
			}
			nv, ok := c.clean(specJoin(path, a.Name), a, m[a.Name], h, errs)
			out[a.Name], good = nv, good && ok
		}
		return out, good
	}
	if err := semKind(kind, v); err != nil {
		specBad(errs, path, err)
		return nil, false
	}
	return v, true
}

type pgLeaf struct{ path, text, reg string }

func (c *Catalog) leaves(path string, col *Column, v any, out *[]pgLeaf) {
	if v == nil {
		return
	}
	if col.List {
		items, _ := specList(v)
		for i, item := range items {
			c.leaf(fmt.Sprintf("%s[%d]", path, i), col, item, out)
		}
		return
	}
	c.leaf(path, col, v, out)
}

func (c *Catalog) leaf(path string, col *Column, v any, out *[]pgLeaf) {
	if v == nil {
		return
	}
	if t := c.typ(col.Schema, col.Type); t != nil && t.Kind == "c" {
		m, _ := v.(map[string]any)
		for _, a := range t.Attrs {
			c.leaves(specJoin(path, a.Name), a, m[a.Name], out)
		}
		return
	}
	var s string
	switch x := v.(type) {
	case string:
		s = x
	case bool:
		s = strconv.FormatBool(x)
	default:
		var ok bool
		if s, ok = semNumStr(v); !ok {
			return
		}
	}
	*out = append(*out, pgLeaf{path, s, col.Reg})
}

func semKind(kind string, v any) error {
	switch kind {
	case "string":
		if _, ok := v.(string); !ok {
			return errors.New("must be a string")
		}
	case "number":
		if _, ok := semNumStr(v); ok {
			return nil
		}
		if s, ok := v.(string); ok {
			if _, ok := semDec(s); ok {
				return nil
			}
		}
		return errors.New("must be a number")
	case "boolean":
		if _, ok := v.(bool); !ok {
			return errors.New("must be true or false")
		}
	case "object":
		if _, ok := v.(map[string]any); !ok {
			return errors.New("must be an object")
		}
	}
	return nil
}

func semValid(s *semType, kind string, v any) error {
	if err := semKind(kind, v); err != nil {
		return err
	}
	if str, ok := v.(string); ok && len(s.Normalize) > 0 {
		c, err := semTransform(s, str)
		if err != nil {
			return err
		}
		if c != str {
			return fmt.Errorf("not canonical, want %q", c)
		}
	}
	if s.Check != "" {
		return semChecks[s.Check](v)
	}
	return nil
}

func semTransform(t *semType, s string) (string, error) {
	var err error
	for _, n := range t.Normalize {
		if s, err = semTransforms[n](s); err != nil {
			return "", err
		}
	}
	return s, nil
}

func semNormalize(s *semType, kind, name string, in any, h Hints) (any, error) {
	if semValid(s, kind, in) == nil {
		return in, nil
	}
	v := in
	if num, ok := semNumStr(in); ok && s.Parse == "" {
		v = num
	}
	if str, ok := v.(string); ok {
		str, err := semTransform(s, strings.TrimSpace(str))
		if err != nil {
			return nil, err
		}
		v = str
	}
	cands := []Candidate{{Value: v, Confidence: 1}}
	if s.Parse != "" {
		var err error
		if cands, err = semParsers[s.Parse](v, h); err != nil {
			return nil, err
		}
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].Confidence > cands[j].Confidence })
	var first error
	for _, c := range cands {
		err := semValid(s, kind, c.Value)
		if err == nil {
			return c.Value, nil
		}
		first = cmp.Or(first, err)
	}
	return nil, cmp.Or(first, fmt.Errorf("not a valid %s", name))
}

func specBad(errs *[]FieldError, path string, err error) {
	var ve *ValidationError
	if errors.As(err, &ve) {
		for _, e := range ve.Errors {
			*errs = append(*errs, FieldError{Path: specJoin(path, e.Path), Message: e.Message})
		}
		return
	}
	*errs = append(*errs, FieldError{Path: path, Message: err.Error()})
}

func specJoin(path, name string) string {
	if path == "" || name == "" {
		return path + name
	}
	return path + "." + name
}

func specList(v any) ([]any, bool) {
	if l, ok := v.([]any); ok {
		return l, true
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice {
		return nil, false
	}
	l := make([]any, rv.Len())
	for i := range l {
		l[i] = rv.Index(i).Interface()
	}
	return l, true
}

func specKeys[V any](m map[string]V) []string {
	return slices.Sorted(maps.Keys(m))
}

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
// schemaTurnKey is the agent's turn a run belongs to, a request's id.
type schemaTurnKey struct{}

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
	// In a turn the confirm carries the turn of its dry run, so a migration that drops data is confirmed only in a
	// later turn, after the person saw the drops; the turn is in the hash, so it cannot be changed.
	turn, _ := ctx.Value(schemaTurnKey{}).(string)
	from := turn
	if confirm != "" {
		from, _, _ = strings.Cut(confirm, ".")
		if !strings.Contains(confirm, ".") {
			from = ""
		}
	}
	h := sha256.Sum256([]byte(strings.Join(stmts, ";\n") + "\x00" + s.hash() + "\x00" + before.hash() + "\x00" + from))
	if out.Confirm = hex.EncodeToString(h[:]); from != "" {
		out.Confirm = from + "." + out.Confirm
	}
	if confirm != "" && confirm != out.Confirm {
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
	if err := schemaMoved(ctx, sp, func(st string) error { return q.DDL(ctx, key, st) }); err != nil {
		return nil, err
	}
	if err := schemaFunctions(ctx, sp); err != nil {
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
	if len(out.Drops) > 0 && turn != "" && from == turn {
		names := make([]string, len(out.Drops))
		for i, d := range out.Drops {
			names[i] = d.Kind + " " + d.Name
		}
		return nil, fmt.Errorf("this migration deletes or rewrites data (%s): tell the person what it drops and confirm it only after they agree, in a later turn: %w", strings.Join(names, ", "), ErrForbidden)
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
	schemaBannedRe = regexp.MustCompile(`(?is)\bOWNER\s+TO\b|\bSET\s+SCHEMA\b|^CREATE\s+TABLE\s+[^(]*\bAS\s`)
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
			ve.Errors = append(ve.Errors, FieldError{Path: path, Message: "OWNER TO, SET SCHEMA and CREATE TABLE ... AS are not allowed"})
		}
	}
	return ve.err()
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

// schemaBare is st with each comment a space, as Postgres reads it, and no space around it.
func schemaBare(st string) string {
	var b strings.Builder
	for i := 0; i < len(st); i++ {
		end, comment := storeSkip(st, i)
		if comment {
			b.WriteByte(' ')
		} else {
			b.WriteString(st[i:min(end+1, len(st))])
		}
		i = end
	}
	return strings.TrimSpace(b.String())
}

// schemaFunctions refuses what Postgres says a migration left behind, however its SQL spelled it: a function that
// runs as its owner, or in another language than sql and plpgsql.
func schemaFunctions(ctx context.Context, q generated.DBTX) error {
	var name string
	err := q.QueryRow(ctx, `SELECT p.oid::pg_catalog.regprocedure::text FROM pg_catalog.pg_proc p
        JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace
        JOIN pg_catalog.pg_language l ON l.oid = p.prolang
        WHERE n.nspname NOT IN ('pg_catalog', 'information_schema', 'archive')
        AND (p.prosecdef OR l.lanname NOT IN ('sql', 'plpgsql')) LIMIT 1`).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("function %s: functions are LANGUAGE sql or plpgsql, without SECURITY DEFINER: %w", name, ErrForbidden)
}

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

// schemaDerive is what each public table lacks of what every table has: archive.object (object.go) and its
// id, name, access, created_at and updated_at columns, the primary key on id and the foreign key on access to
// archive."group" that INHERITS does not pass on,
// the search column and index, row-level security with the access policy, and the grants to the viewer and
// editor roles; and a derived search that no longer covers its text columns. With cols, only the columns
// and object.
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
		if c := t.col("id"); c == nil {
			add = append(add, "ADD COLUMN id uuid NOT NULL DEFAULT uuidv7()", "ADD CONSTRAINT "+pgIdent(name+"_pkey")+" PRIMARY KEY (id)")
		} else if c.Type == "uuid" && !c.List && !c.PK && !t.Partitioned {
			// INHERITS gives a table its parent's columns, not its primary key.
			add = append(add, "ADD CONSTRAINT "+pgIdent(name+"_pkey")+" PRIMARY KEY (id)")
		}
		if c := t.col("name"); c == nil {
			add = append(add, "ADD COLUMN name text NOT NULL DEFAULT ''")
		} else if !t.Object && objectText(c) && !c.NotNull {
			if !c.Default {
				add = append(add, "ALTER COLUMN name SET DEFAULT ''")
			}
			add = append(add, "ALTER COLUMN name SET NOT NULL")
		}
		access := t.col("access")
		if access == nil {
			add = append(add, "ADD COLUMN access uuid")
		}
		if !t.Object && !t.Partitioned && objectFits(t) {
			after = append(after, "ALTER TABLE "+id+" INHERIT archive.object")
		}
		// Postgres passes no foreign key on: each table's access references the group that sees its rows.
		if access == nil || access.Type == "uuid" && !access.List && access.Ref == "" {
			after = append(after, "ALTER TABLE "+id+" ADD CONSTRAINT "+pgIdent(name+"_access_fkey")+` FOREIGN KEY (access) REFERENCES archive."group"(id) ON DELETE RESTRICT`)
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

// schemaMoved gives each public table that lacks it the trigger that tells Go what a write moved
// (archive.moved), by running each statement. It is plumbing, like the row-level security every table has,
// so schema show and a migration's changes leave it out (pgRead).
func schemaMoved(ctx context.Context, q generated.DBTX, run func(string) error) error {
	rows, err := q.Query(ctx, `SELECT c.relname::text FROM pg_catalog.pg_class c WHERE c.relnamespace = 'public'::regnamespace
        AND c.relkind IN ('r', 'p') AND NOT c.relispartition
        AND NOT EXISTS (SELECT FROM pg_catalog.pg_trigger g WHERE g.tgrelid = c.oid AND g.tgname = 'archive_moved') ORDER BY 1`)
	if err != nil {
		return err
	}
	names, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	for _, name := range names {
		if err := run("CREATE OR REPLACE TRIGGER archive_moved AFTER INSERT OR UPDATE OR DELETE OR TRUNCATE ON " + pgIdent("public", name) +
			" FOR EACH STATEMENT EXECUTE FUNCTION archive.moved()"); err != nil {
			return err
		}
	}
	return nil
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
	applied := true
	err = pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		for _, st := range []string{"SET LOCAL search_path = ''", "SET LOCAL check_function_bodies = off"} {
			if _, err := tx.Exec(ctx, st); err != nil {
				return err
			}
		}
		lost, err := pgMigrate(ctx, tx, want)
		if err != nil {
			return fmt.Errorf("archive.sql: %w", err)
		}
		if len(lost) > 0 {
			fmt.Fprintf(os.Stderr, "archive: archive.sql not applied at startup, it would delete data:\n  %s\n", strings.Join(lost, "\n  "))
			applied = false
			return nil
		}
		if err := generated.New(tx).SeedAgent(ctx); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, fileInheritOnce); err != nil {
			return fmt.Errorf("files in folders: %w", err)
		}
		return generated.New(tx).InsertRevision(ctx, s.hash())
	})
	if err != nil || !applied {
		return err
	}
	return syncCheckSQL(ctx, conn)
}

// fileInheritOnce gives what is in a folder the folder's access, top down, for rows from before file_check did; two
// that then differ by nothing but access keep both, one renamed. Delete it once production has run it.
const fileInheritOnce = `DO $$
DECLARE
    r record;
BEGIN
    FOR r IN
        WITH RECURSIVE tree AS (
            SELECT id, 0 AS depth FROM archive.file WHERE parent IS NULL AND connection IS NULL AND kind = 'folder'
            UNION ALL
            SELECT f.id, t.depth + 1 FROM archive.file f JOIN tree t ON f.parent = t.id WHERE f.connection IS NULL
        )
        SELECT id FROM tree WHERE depth > 0 ORDER BY depth
    LOOP
        BEGIN
            UPDATE archive.file c SET access = p.access FROM archive.file p WHERE c.id = r.id AND p.id = c.parent AND c.access IS DISTINCT FROM p.access;
        EXCEPTION WHEN unique_violation THEN
            UPDATE archive.file c SET name = 'Copy ' || left(c.id::text, 4) || ' ' || c.name, access = p.access FROM archive.file p WHERE c.id = r.id AND p.id = c.parent;
        END;
    END LOOP;
END $$`

// scratch is the snapshot of Archive's schema that sql makes in a new database, which it drops after.
func (s *pgStore) scratch(ctx context.Context, sql string) (pgSchema, error) {
	return pgScratch(ctx, s.sys, s.cfg, sql)
}

// pgScratch is the snapshot of the schema that sql makes in a new database on sys's server, which it drops after.
func pgScratch(ctx context.Context, sys *pgxpool.Pool, base *pgx.ConnConfig, sql string) (pgSchema, error) {
	b := make([]byte, 8)
	rand.Read(b)
	name := "archive_scratch_" + hex.EncodeToString(b)
	if _, err := sys.Exec(ctx, "CREATE DATABASE "+pgIdent(name)); err != nil {
		return nil, err
	}
	defer sys.Exec(context.WithoutCancel(ctx), "DROP DATABASE IF EXISTS "+pgIdent(name)+" WITH (FORCE)")
	cfg := base.Copy()
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
// what and runs nothing.
func pgMigrate(ctx context.Context, tx pgx.Tx, want pgSchema) ([]string, error) {
	for _, st := range []string{"SET LOCAL search_path = ''", "SET LOCAL check_function_bodies = off", "SET LOCAL lock_timeout = '1min'", "SET LOCAL statement_timeout = 0"} {
		if _, err := tx.Exec(ctx, st); err != nil {
			return nil, err
		}
	}
	live, err := pgRead(ctx, tx, "archive", "public")
	if err != nil {
		return nil, err
	}
	stmts, lost, err := pgPlan(live.ours(), want)
	if err != nil || len(lost) > 0 {
		return lost, err
	}
	for _, st := range stmts {
		if _, err := tx.Exec(ctx, st); err != nil {
			return nil, fmt.Errorf("%s: %w", st, err)
		}
	}
	return lost, nil
}

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
	phChildren
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

// pgDropped are the tables and columns archive.sql deletes on purpose, data and all, by kind and qualified
// name, like "table archive.record": pgPlan drops them as it finds them gone from archive.sql, or a column of
// another type there, which it makes anew, where it refuses to drop anything else that has data. Each stays listed, so a database that still has one drops it whenever
// it opens next.
var pgDropped = map[string]bool{
	// What connections synced is in the tables of its domain now, and syncs started from scratch to fill them.
	"table archive.record": true,
	// Synced files are rows of archive.file now, and syncs start from scratch to fill them.
	"table archive.drive_file": true,
	// Agents replaced chats, and nothing of them was kept.
	"table archive.chat": true,
	// An agent's avatar became a number, and every agent started over on the first.
	"column archive.agent.avatar": true,
}

// pgPlan is the DDL that brings live to want, both read with an empty search_path, for Archive's own schema: it
// does what archive.sql needs and refuses the rest. Types and domains are altered, never dropped and created,
// since user tables use them. lost is what would lose data; the plan leaves it out, unless pgDropped names it.
func pgPlan(live, want pgSchema) (stmts, lost []string, err error) {
	var ph [phases][]string
	add := func(p int, st string) { ph[p] = append(ph[p], st) }
	fail := func(o *pgObject, what string) {
		if err == nil {
			err = fmt.Errorf("%s %s: %s is not supported; change archive.sql another way, or teach pgPlan in src/schema.go", o.Kind, o.qname(), what)
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
				// A table that inherits another is made once the tables it inherits are.
				p := phase[n.Kind]
				if parents, _ := pgInherits(n.Def); n.Kind == "table" && len(parents) > 0 {
					p = phChildren
				}
				add(p, strings.SplitN(want.render(n), ";\n", 2)[0])
				if n.RLS {
					add(p, "ALTER TABLE "+pgIdent(n.Schema, n.Name)+" ENABLE ROW LEVEL SECURITY")
				}
				if n.Force {
					add(p, "ALTER TABLE "+pgIdent(n.Schema, n.Name)+" FORCE ROW LEVEL SECURITY")
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
				if !pgDropped[o.key()] {
					lost = append(lost, o.Kind+" "+o.qname())
				}
				if pgDropped[o.key()] {
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
						// A table begins or stops inheriting once its columns are its parent's: after the tables.
						was, rest := pgInherits(o.Def)
						now, nrest := pgInherits(n.Def)
						if rest != nrest {
							fail(n, "changing it from "+o.Def+" to "+n.Def)
						}
						for _, p := range now {
							if !slices.Contains(was, p) {
								add(phKeys, "ALTER TABLE "+pgIdent(n.Schema, n.Name)+" INHERIT "+p)
							}
						}
						for _, p := range was {
							if !slices.Contains(now, p) {
								add(phKeys, "ALTER TABLE "+pgIdent(n.Schema, n.Name)+" NO INHERIT "+p)
							}
						}
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
					pgAlter(want, o, n, add, fail, &lost)
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

var pgInheritsRe = regexp.MustCompile(`INHERITS \(([^)]*)\)\s*`)

// pgInherits is the parents a table's definition names, and the rest of it.
func pgInherits(def string) ([]string, string) {
	m := pgInheritsRe.FindStringSubmatch(def)
	if m == nil {
		return nil, strings.TrimSpace(def)
	}
	return strings.Split(m[1], ", "), strings.TrimSpace(pgInheritsRe.ReplaceAllString(def, ""))
}

// pgAlter alters a domain, column or attribute o into n: its type, when no data is lost, default and NOT NULL.
// A column made NOT NULL with a default takes its default where it was null first.
func pgAlter(want pgSchema, o, n *pgObject, add func(int, string), fail func(*pgObject, string), lost *[]string) {
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
		case pgDropped[n.key()]:
			add(p, want.alter(n)+" DROP COLUMN "+pgIdent(n.Name))
			add(p, want.alter(n)+" ADD COLUMN "+pgIdent(n.Name)+" "+n.Def)
			return
		default:
			*lost = append(*lost, n.Kind+" "+n.qname()+" from "+o.Type+" to "+n.Type)
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
		if n.Kind == "column" && n.Items[1] == "true" && n.Items[0] != "" && (len(n.Items) < 3 || n.Items[2] == "") {
			add(p, "UPDATE "+pgIdent(n.Schema, n.Of)+" SET "+pgIdent(n.Name)+" = "+n.Items[0]+" WHERE "+pgIdent(n.Name)+" IS NULL")
		}
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
