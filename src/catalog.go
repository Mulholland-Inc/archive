package main

import (
	"cmp"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

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
	Icon        string    `json:"icon"`
	Layout      string    `json:"layout,omitempty"`
	By          string    `json:"by,omitempty"`
	RLS         bool      `json:"rls"`
	Force       bool      `json:"force"`
	Invoker     bool      `json:"security_invoker,omitempty"`
	Policy      bool      `json:"policy"`
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
			case "icon":
				t.Icon = kv[1]
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
	}
	return c, nil
}

// catalogWorkflow shows archive.workflow as a person writes it: name, source and on, with its next time and
// webhook URL to read. What Archive keeps of it (who it runs as, its trigger and cursor) stays out of list and
// get, and its webhook secret becomes its URL (workflowRows).
func catalogWorkflow(t *Table) {
	internal := []string{"user_id", "member", "role", "trigger", "hook", "cursor_at", "cursor_id"}
	t.Columns = slices.DeleteFunc(t.Columns, func(c *Column) bool { return slices.Contains(internal, c.Name) })
	t.internal = slices.DeleteFunc(internal, func(n string) bool { return n == "hook" })
	if c := t.col("next_at"); c != nil {
		c.Generated = true
	}
	t.Columns = append(t.Columns, &Column{Name: "url", Type: "text", Schema: "pg_catalog", Reg: "text", Generated: true,
		Description: "Its webhook URL, when on is { webhook: true }: anyone with it can POST to it, so give it only to the service that calls it"})
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

//go:embed icons.json
var catalogIconsJSON []byte

var catalogIcons = func() map[string]bool {
	var names []string
	json.Unmarshal(catalogIconsJSON, &names)
	out := map[string]bool{}
	for _, n := range names {
		out[n] = true
	}
	return out
}()

var catalogLayouts = []string{"table", "board", "calendar"}

var (
	catalogSystem   = map[string]bool{"id": true, "created_at": true, "updated_at": true}
	catalogReserved = map[string]bool{"files": true, "views": true, "search": true, "schema": true, "types": true,
		"member": true, "user": true, "token": true, "file": true, "upload": true, "secret": true, "revision": true, "run": true,
		"workflow": true, "workflow_run": true}
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
	if err == nil && t.Schema == "archive" && t.Name == "workflow_run" {
		return nil, fmt.Errorf("workflow_run is read only: Archive records a run each time a workflow runs: %w", ErrReadOnly)
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
		{"access", "text", "want access text, null for rows the whole workspace sees, like access text, or access text NOT NULL DEFAULT 'hr' CHECK (access = 'hr') for a table only the hr group sees"},
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
			continue
		}
		for _, w := range want {
			col := t.col(w.name)
			ok := col != nil && col.Type == w.typ && !col.List
			switch w.name {
			case "id":
				ok = ok && col.PK && col.Default
			case "created_at", "updated_at":
				ok = ok && col.NotNull && col.Default
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
	keys, what := []string{"icon"}, "COMMENT ON TABLE "+t.Name
	if t.Kind == "view" {
		keys, what = []string{"icon", "layout", "by"}, "COMMENT ON VIEW "+t.Name
	}
	seen := map[string]bool{}
	for _, kv := range t.meta {
		k, v, path := kv[0], kv[1], t.Name+"."+kv[0]
		switch {
		case !slices.Contains(keys, k):
			bad(path, "unknown key "+k+" in the last lines of "+what+"; want description lines, then key: value lines with "+strings.Join(keys, ", "))
		case seen[k]:
			bad(path, k+" is set twice in "+what)
		case k == "icon" && !catalogIcons[v]:
			bad(path, "unknown icon "+strconv.Quote(v)+"; want a Lucide icon name in kebab-case, like user, credit-card or square-check (see lucide.dev/icons)")
		case k == "layout" && !slices.Contains(catalogLayouts, v):
			bad(path, "unknown layout "+strconv.Quote(v)+"; want "+strings.Join(catalogLayouts, ", "))
		case k == "by" && t.col(v) == nil:
			bad(path, "no column "+strconv.Quote(v)+" in view "+t.Name+"; want one of its columns: "+strings.Join(t.colNames(), ", "))
		}
		seen[k] = true
	}
	if (t.Layout == "board" || t.Layout == "calendar") && t.By == "" {
		bad(t.Name+".by", "a "+t.Layout+" view needs a by: <column> line in "+what)
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
