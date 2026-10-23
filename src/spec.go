package main

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
)

const (
	specReference = "reference"
	specEnum      = "enum"
)

func ParseSpec(b []byte) (*Spec, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	s := &Spec{}
	if err := dec.Decode(s); err != nil {
		return nil, fmt.Errorf("spec: %w", err)
	}
	specFix(s)
	return s, nil
}

func (s *Spec) JSON() ([]byte, error) {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

func specFix(s *Spec) {
	if s.Types == nil {
		s.Types = map[string]*Type{}
	}
	for name, t := range s.Types {
		if t == nil {
			t = &Type{}
			s.Types[name] = t
		}
		t.Name = name
		if t.Fields == nil {
			t.Fields = map[string]*Field{}
		}
	}
}

func (s *Spec) resolve(typ string) (*Type, error) {
	var chain []*Type
	for name := typ; name != ""; {
		t, ok := s.Types[name]
		if !ok {
			return nil, fmt.Errorf("unknown type %q: %w", name, ErrNotFound)
		}
		if len(chain) > len(s.Types) {
			return nil, fmt.Errorf("type %q: cyclic extends", typ)
		}
		chain = append(chain, t)
		name = t.Extends
	}
	r := *chain[0]
	r.Fields = map[string]*Field{}
	for i := len(chain) - 1; i >= 0; i-- {
		maps.Copy(r.Fields, chain[i].Fields)
	}
	for _, t := range chain[1:] {
		if r.Schema == nil {
			r.Schema = t.Schema
		}
		if r.Normalize == nil {
			r.Normalize = t.Normalize
		}
		r.Category, r.Sensitivity = cmp.Or(r.Category, t.Category), cmp.Or(r.Sensitivity, t.Sensitivity)
		r.Parse, r.Check, r.Format = cmp.Or(r.Parse, t.Parse), cmp.Or(r.Check, t.Check), cmp.Or(r.Format, t.Format)
	}
	return &r, nil
}

func (s *Spec) Check() error {
	ve := &ValidationError{Type: "spec"}
	bad := func(path, format string, a ...any) {
		ve.Errors = append(ve.Errors, FieldError{Path: path, Message: fmt.Sprintf(format, a...)})
	}
	for _, name := range specKeys(s.Types) {
		t := s.Types[name]
		r, rerr := s.resolve(name)
		if _, ok := s.Types[t.Extends]; t.Extends != "" && !ok {
			bad(name, "extends unknown type %q", t.Extends)
		} else if rerr != nil {
			bad(name, "cyclic extends")
		} else if r.Schema != nil && len(r.Fields) > 0 {
			bad(name, "has both a schema and fields")
		}
		if t.Schema != nil {
			if err := specSchemaCheck(t.Schema); err != nil {
				bad(name, "schema: %v", err)
			}
		}
		if !slices.Contains([]Sensitivity{"", Public, Personal, Secret}, t.Sensitivity) {
			bad(name, "unknown sensitivity %q", t.Sensitivity)
		}
		for _, n := range t.Normalize {
			if semTransforms[n] == nil {
				bad(name, "unknown transform %q", n)
			}
		}
		if t.Parse != "" && semParsers[t.Parse] == nil {
			bad(name, "unknown parser %q", t.Parse)
		}
		if t.Check != "" && semChecks[t.Check] == nil {
			bad(name, "unknown check %q", t.Check)
		}
		if t.Format != "" && semFormats[t.Format] == nil {
			bad(name, "unknown formatter %q", t.Format)
		}
		var parent map[string]*Field
		if t.Extends != "" && rerr == nil {
			parent, _ = s.Fields(t.Extends)
		}
		for _, fname := range specKeys(t.Fields) {
			f, path := t.Fields[fname], name+"."+fname
			if f == nil {
				bad(path, "empty field")
				continue
			}
			if _, ok := s.Types[f.Type]; !ok {
				bad(path, "unknown type %q", f.Type)
			} else if f.Type == specReference && s.Types[f.To] == nil {
				bad(path, "reference to unknown type %q", f.To)
			} else if f.Type == specEnum && len(f.Enum) == 0 {
				bad(path, "enum has no values")
			}
			if p := parent[fname]; p != nil {
				if p.Type != f.Type {
					bad(path, "redefines inherited %s as %s", p.Type, f.Type)
				} else if f.Type == specReference && p.To != f.To && !slices.Contains(s.Subtypes(p.To), f.To) {
					bad(path, "narrows reference to %q, not a subtype of %q", f.To, p.To)
				}
			}
		}
		if rerr == nil && s.specContains(name, name, map[string]bool{}) {
			bad(name, "contains itself; use a reference")
		}
	}
	if len(ve.Errors) == 0 {
		for _, name := range specKeys(s.Types) {
			for i, ex := range s.Types[name].Examples {
				if err := s.Validate(name, ex); err != nil {
					bad(fmt.Sprintf("%s.examples[%d]", name, i), "%v", err)
				}
			}
		}
	}
	if len(ve.Errors) > 0 {
		return ve
	}
	return nil
}

func (s *Spec) specContains(typ, target string, seen map[string]bool) bool {
	fields, _ := s.Fields(typ)
	for _, f := range fields {
		if f.Type == target {
			return true
		}
		if !seen[f.Type] {
			seen[f.Type] = true
			if s.specContains(f.Type, target, seen) {
				return true
			}
		}
	}
	return false
}

func (s *Spec) Fields(typ string) (map[string]*Field, error) {
	t, err := s.resolve(typ)
	if err != nil {
		return nil, err
	}
	return t.Fields, nil
}

func (s *Spec) Subtypes(typ string) []string {
	if _, ok := s.Types[typ]; !ok {
		return nil
	}
	in := map[string]bool{typ: true}
	for grew := true; grew; {
		grew = false
		for name, t := range s.Types {
			if !in[name] && in[t.Extends] {
				in[name], grew = true, true
			}
		}
	}
	return specKeys(in)
}

func (s *Spec) Normalize(typ string, in map[string]any, h Hints) (out map[string]any, originals map[string]string, err error) {
	t, err := s.resolve(typ)
	if err != nil {
		return nil, nil, err
	}
	if t.Schema != nil {
		return nil, nil, fmt.Errorf("%s is a scalar type, not a record type", typ)
	}
	ve := &ValidationError{Type: typ}
	out, _ = s.specObject(t, "", in, &h, &ve.Errors)
	if len(ve.Errors) > 0 {
		return nil, nil, ve
	}
	originals = map[string]string{}
	for k, v := range out {
		if !reflect.DeepEqual(in[k], v) {
			originals[k] = specString(in[k])
		}
	}
	return out, originals, nil
}

func (s *Spec) Validate(typ string, v any) error {
	t, err := s.resolve(typ)
	if err != nil {
		return err
	}
	ve := &ValidationError{Type: typ}
	s.specValue(t, "", v, nil, &ve.Errors)
	if len(ve.Errors) > 0 {
		return ve
	}
	return nil
}

func (s *Spec) Candidates(typ string, in any, h Hints) ([]Candidate, error) {
	t, err := s.resolve(typ)
	if err != nil {
		return nil, err
	}
	return s.specCandidates(t, in, h)
}

func (s *Spec) NormalizeField(f *Field, v any, h Hints) (any, error) {
	t, err := s.resolve(f.Type)
	if err != nil {
		return nil, err
	}
	var errs []FieldError
	nv, ok := s.specOne(t, f, "", v, &h, &errs)
	if !ok {
		return nil, errors.New(errs[0].Message)
	}
	return nv, nil
}

func (s *Spec) Format(typ string, v any, locale string) (out string) {
	t, err := s.resolve(typ)
	if err != nil || t.Format == "" || s.specValid(t, v) != nil {
		return semPlain(v)
	}
	defer func() {
		if recover() != nil {
			out = semPlain(v)
		}
	}()
	return semFormats[t.Format](v, locale)
}

func (s *Spec) specValue(t *Type, path string, v any, h *Hints, errs *[]FieldError) (any, bool) {
	if _, ok := secretRef(v); ok && t.Sensitivity == Secret {
		return v, true
	}
	if h != nil && (t.Parse != "" || t.Schema != nil) {
		c, err := s.specCandidates(t, v, *h)
		if err != nil {
			specBad(errs, path, err)
			return nil, false
		}
		return c[0].Value, true
	}
	if t.Schema == nil {
		m, ok := v.(map[string]any)
		if !ok {
			specBad(errs, path, errors.New("must be an object"))
			return nil, false
		}
		return s.specObject(t, path, m, h, errs)
	}
	if err := specScalar(t, v); err != nil {
		specBad(errs, path, err)
		return nil, false
	}
	return v, true
}

func specScalar(t *Type, v any) error {
	if err := specSchemaValid(t.Schema, v); err != nil {
		if t.Schema["type"] == "object" {
			return err
		}
		return fmt.Errorf("not a valid %s", t.Name)
	}
	if str, ok := v.(string); ok && len(t.Normalize) > 0 {
		c, err := specTransform(t, str)
		if err != nil {
			return err
		}
		if c != str {
			return fmt.Errorf("not canonical, want %q", c)
		}
	}
	if t.Check != "" {
		return semChecks[t.Check](v)
	}
	return nil
}

func specTransform(t *Type, s string) (string, error) {
	var err error
	for _, n := range t.Normalize {
		if s, err = semTransforms[n](s); err != nil {
			return "", err
		}
	}
	return s, nil
}

func (s *Spec) specValid(t *Type, v any) error {
	var errs []FieldError
	if s.specValue(t, "", v, nil, &errs); len(errs) == 0 {
		return nil
	}
	if errs[0].Path == "" {
		return errors.New(errs[0].Message)
	}
	return errors.New(errs[0].Path + ": " + errs[0].Message)
}

func (s *Spec) specCandidates(t *Type, in any, h Hints) ([]Candidate, error) {
	if s.specValid(t, in) == nil {
		return []Candidate{{Value: in, Confidence: 1}}, nil
	}
	if t.Schema == nil && t.Parse == "" {
		m, ok := in.(map[string]any)
		if !ok {
			return nil, errors.New("must be an object")
		}
		var errs []FieldError
		out, _ := s.specObject(t, "", m, &h, &errs)
		if len(errs) > 0 {
			return nil, &ValidationError{Type: t.Name, Errors: errs}
		}
		return []Candidate{{Value: out, Confidence: 1}}, nil
	}
	v := in
	if num, ok := semNumStr(in); ok && t.Parse == "" {
		v = num
	}
	if str, ok := v.(string); ok {
		str, err := specTransform(t, strings.TrimSpace(str))
		if err != nil {
			return nil, err
		}
		v = str
	}
	cands := []Candidate{{Value: v, Confidence: 1}}
	if t.Parse != "" {
		var err error
		if cands, err = semParsers[t.Parse](v, h); err != nil {
			return nil, err
		}
	}
	var out []Candidate
	var first error
	seen := map[string]int{}
	for _, c := range cands {
		if err := s.specValid(t, c.Value); err != nil {
			first = cmp.Or(first, err)
			continue
		}
		b, _ := json.Marshal(c.Value)
		if i, dup := seen[string(b)]; dup {
			out[i].Confidence = max(out[i].Confidence, c.Confidence)
			continue
		}
		seen[string(b)] = len(out)
		out = append(out, c)
	}
	if len(out) == 0 {
		return nil, cmp.Or(first, fmt.Errorf("not a valid %s", t.Name))
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Confidence > out[j].Confidence })
	return out, nil
}

func (s *Spec) specObject(t *Type, path string, m map[string]any, h *Hints, errs *[]FieldError) (map[string]any, bool) {
	n := len(*errs)
	for _, k := range specKeys(m) {
		if _, ok := t.Fields[k]; !ok && m[k] != nil {
			specBad(errs, specJoin(path, k), errors.New("unknown field"))
		}
	}
	out := map[string]any{}
	for _, name := range specKeys(t.Fields) {
		f, v, p := t.Fields[name], m[name], specJoin(path, name)
		if v == nil {
			if f.Required {
				specBad(errs, p, errors.New("required"))
			}
			continue
		}
		if nv, ok := s.specField(p, f, v, h, errs); ok {
			out[name] = nv
		}
	}
	if len(*errs) == n && t.Check != "" {
		if err := semChecks[t.Check](out); err != nil {
			specBad(errs, path, err)
		}
	}
	return out, len(*errs) == n
}

func (s *Spec) specField(path string, f *Field, v any, h *Hints, errs *[]FieldError) (any, bool) {
	t, err := s.resolve(f.Type)
	if err != nil {
		specBad(errs, path, err)
		return nil, false
	}
	if !f.List {
		return s.specOne(t, f, path, v, h, errs)
	}
	items, ok := specList(v)
	if !ok {
		if h == nil {
			specBad(errs, path, errors.New("must be a list"))
			return nil, false
		}
		items = []any{v}
	}
	list, good := make([]any, 0, len(items)), true
	for i, item := range items {
		nv, ok := s.specOne(t, f, fmt.Sprintf("%s[%d]", path, i), item, h, errs)
		list, good = append(list, nv), good && ok
	}
	return list, good
}

func (s *Spec) specOne(t *Type, f *Field, path string, v any, h *Hints, errs *[]FieldError) (any, bool) {
	nv, ok := s.specValue(t, path, v, h, errs)
	if !ok || len(f.Enum) == 0 {
		return nv, ok
	}
	str, _ := nv.(string)
	for _, e := range f.Enum {
		if str == e || (h != nil && strings.EqualFold(str, e)) {
			return e, true
		}
	}
	specBad(errs, path, fmt.Errorf("must be one of %s", strings.Join(f.Enum, ", ")))
	return nil, false
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

func specString(v any) string {
	if str, ok := v.(string); ok {
		return str
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}

func (s *Spec) JSONSchema(typ string) (map[string]any, error) {
	t, err := s.resolve(typ)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if t.Schema != nil {
		out = specClone(t.Schema).(map[string]any)
	} else {
		props, names := map[string]any{}, specKeys(t.Fields)
		for _, name := range names {
			f := t.Fields[name]
			p, err := s.JSONSchema(f.Type)
			if err != nil {
				return nil, fmt.Errorf("%s.%s: %w", typ, name, err)
			}
			if len(f.Enum) > 0 {
				p["enum"] = slices.Clone(f.Enum)
			}
			if f.List {
				p = map[string]any{"type": "array", "items": p}
			}
			if !f.Required {
				p = map[string]any{"anyOf": []any{p, map[string]any{"type": "null"}}}
			}
			if f.Description != "" {
				p["description"] = f.Description
			}
			props[name] = p
		}
		out = map[string]any{"type": "object", "properties": props, "required": names, "additionalProperties": false}
	}
	if t.Description != "" {
		out["description"] = t.Description
	}
	return out, nil
}

var specSchemaKeys = map[string]bool{"type": true, "properties": true, "required": true, "additionalProperties": true,
	"enum": true, "items": true, "anyOf": true, "pattern": true, "format": true, "description": true}

func specSchemaCheck(sc map[string]any) error {
	var subs []any
	for _, k := range specKeys(sc) {
		if !specSchemaKeys[k] {
			return fmt.Errorf("keyword %q is not supported", k)
		}
		switch v := sc[k].(type) {
		case map[string]any:
			if k == "properties" {
				subs = append(subs, slices.Collect(maps.Values(v))...)
			} else {
				subs = append(subs, v)
			}
		case []any:
			if k == "anyOf" {
				subs = append(subs, v...)
			}
		}
	}
	if p, ok := sc["pattern"].(string); ok {
		if _, err := specRegexp(p); err != nil {
			return err
		}
	}
	if sc["type"] == "object" {
		props, _ := sc["properties"].(map[string]any)
		req, _ := sc["required"].([]any)
		if len(req) != len(props) || sc["additionalProperties"] != false {
			return errors.New("an object must require every property and forbid others")
		}
	}
	for _, sub := range subs {
		m, ok := sub.(map[string]any)
		if !ok {
			return errors.New("subschema must be an object")
		}
		if err := specSchemaCheck(m); err != nil {
			return err
		}
	}
	return nil
}

func specSchemaValid(sc map[string]any, v any) error {
	if alts, ok := sc["anyOf"].([]any); ok {
		var first error
		for _, a := range alts {
			m, _ := a.(map[string]any)
			err := specSchemaValid(m, v)
			if err == nil {
				return nil
			}
			first = cmp.Or(first, err)
		}
		return first
	}
	if enum, ok := sc["enum"].([]any); ok && !slices.ContainsFunc(enum, func(e any) bool { return reflect.DeepEqual(e, v) }) {
		return errors.New("not one of the allowed values")
	}
	switch sc["type"] {
	case "string":
		str, ok := v.(string)
		if !ok {
			return errors.New("must be a string")
		}
		if p, _ := sc["pattern"].(string); p != "" {
			if re, err := specRegexp(p); err != nil || !re.MatchString(str) {
				return errors.New("has the wrong format")
			}
		}
	case "number":
		if _, ok := v.(float64); !ok {
			return errors.New("must be a number")
		}
	case "integer":
		if f, ok := v.(float64); !ok || f != math.Trunc(f) || math.Abs(f) > 1<<53 {
			return errors.New("must be an integer within ±2^53")
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			return errors.New("must be true or false")
		}
	case "null":
		if v != nil {
			return errors.New("must be null")
		}
	case "array":
		l, ok := v.([]any)
		if !ok {
			return errors.New("must be a list")
		}
		items, _ := sc["items"].(map[string]any)
		for i, e := range l {
			if err := specSchemaValid(items, e); err != nil {
				return fmt.Errorf("[%d]: %w", i, err)
			}
		}
	case "object":
		m, ok := v.(map[string]any)
		if !ok {
			return errors.New("must be an object")
		}
		props, _ := sc["properties"].(map[string]any)
		for _, k := range specKeys(m) {
			if _, ok := props[k]; !ok {
				return fmt.Errorf("unknown field %q", k)
			}
		}
		for _, k := range specKeys(props) {
			p, _ := props[k].(map[string]any)
			if err := specSchemaValid(p, m[k]); err != nil {
				return fmt.Errorf("%s: %w", k, err)
			}
		}
	}
	return nil
}

var specRegexps sync.Map

func specRegexp(p string) (*regexp.Regexp, error) {
	if re, ok := specRegexps.Load(p); ok {
		return re.(*regexp.Regexp), nil
	}
	re, err := regexp.Compile(p)
	if err != nil {
		return nil, fmt.Errorf("bad pattern %q: %w", p, err)
	}
	specRegexps.Store(p, re)
	return re, nil
}

func specClone(v any) any {
	switch x := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(x))
		for k, e := range x {
			m[k] = specClone(e)
		}
		return m
	case []any:
		a := make([]any, len(x))
		for i, e := range x {
			a[i] = specClone(e)
		}
		return a
	}
	return v
}

func DiffSpecs(a, b *Spec) []Op {
	var ops []Op
	for _, name := range specKeys(a.Types) {
		if _, ok := b.Types[name]; !ok && !a.Types[name].Builtin {
			ops = append(ops, Op{Op: "remove_type", Type: name})
		}
	}
	for _, name := range specKeys(b.Types) {
		bt := b.Types[name]
		at, ok := a.Types[name]
		if !ok {
			ops = append(ops, Op{Op: "add_type", Type: name, Def: bt})
			continue
		}
		if !specSame(specLevel(at), specLevel(bt)) {
			ops = append(ops, Op{Op: "change_type", Type: name, Def: specLevel(bt)})
		}
		for _, f := range specKeys(at.Fields) {
			if _, ok := bt.Fields[f]; !ok {
				ops = append(ops, Op{Op: "remove_field", Type: name, Field: f})
			}
		}
		for _, f := range specKeys(bt.Fields) {
			af, ok := at.Fields[f]
			switch {
			case !ok:
				ops = append(ops, Op{Op: "add_field", Type: name, Field: f, Def: bt.Fields[f]})
			case !reflect.DeepEqual(af, bt.Fields[f]):
				ops = append(ops, Op{Op: "change_field", Type: name, Field: f, Def: bt.Fields[f]})
			}
		}
	}
	return ops
}

func specLevel(t *Type) *Type {
	c := *t
	c.Fields = nil
	return &c
}

func specSame(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return bytes.Equal(x, y)
}

func (s *Spec) Apply(ops []Op) (*Spec, error) {
	n := &Spec{Version: s.Version + 1, Types: maps.Clone(s.Types)}
	for i, op := range ops {
		if err := n.specApply(op); err != nil {
			return nil, fmt.Errorf("op %d (%s %s): %w", i, op.Op, op.Type, err)
		}
	}
	if err := n.Check(); err != nil {
		return nil, err
	}
	return n, nil
}

func (s *Spec) specApply(op Op) error {
	t, exists := s.Types[op.Type]
	if op.Op == "add_type" {
		if exists {
			return fmt.Errorf("type exists")
		}
		nt := &Type{}
		if op.Def != nil {
			if err := specCopy(op.Def, nt); err != nil {
				return err
			}
		}
		nt.Name, nt.Edited = op.Type, true
		if nt.Fields == nil {
			nt.Fields = map[string]*Field{}
		}
		s.Types[op.Type] = nt
		return nil
	}
	if !exists {
		return fmt.Errorf("unknown type")
	}
	switch op.Op {
	case "remove_type":
		if t.Builtin {
			return fmt.Errorf("built-in types cannot be removed")
		}
		delete(s.Types, op.Type)
		return nil
	case "change_type":
		nt := &Type{}
		if op.Def == nil {
			return fmt.Errorf("def is required")
		}
		if err := specCopy(op.Def, nt); err != nil {
			return err
		}
		if len(nt.Fields) > 0 {
			return fmt.Errorf("change_type cannot change fields; use field ops")
		}
		nt.Name, nt.Builtin, nt.Edited, nt.Fields = t.Name, t.Builtin, true, t.Fields
		s.Types[op.Type] = nt
		return nil
	}
	c := *t
	c.Fields, c.Edited = maps.Clone(t.Fields), true
	s.Types[op.Type] = &c
	_, has := c.Fields[op.Field]
	switch op.Op {
	case "add_field", "change_field":
		if op.Op == "add_field" && has {
			return fmt.Errorf("field %q exists", op.Field)
		}
		if op.Op == "change_field" && !has {
			return fmt.Errorf("unknown field %q", op.Field)
		}
		if op.Field == "" || op.Def == nil {
			return fmt.Errorf("field and def are required")
		}
		f := &Field{}
		if err := specCopy(op.Def, f); err != nil {
			return err
		}
		c.Fields[op.Field] = f
	case "remove_field":
		if !has {
			return fmt.Errorf("unknown field %q", op.Field)
		}
		delete(c.Fields, op.Field)
	case "rename_field":
		if !has {
			return fmt.Errorf("unknown field %q", op.Field)
		}
		if _, taken := c.Fields[op.To]; taken || op.To == "" {
			return fmt.Errorf("cannot rename to %q", op.To)
		}
		c.Fields[op.To] = c.Fields[op.Field]
		delete(c.Fields, op.Field)
	default:
		return fmt.Errorf("unknown op")
	}
	return nil
}

func specCopy(src, dst any) error {
	b, err := json.Marshal(src)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("bad def: %w", err)
	}
	return nil
}

func (s *Spec) eachSecret(typ string, data map[string]any, fn func(path string, t *Type, v any) (any, error)) error {
	t, err := s.resolve(typ)
	if err != nil {
		return err
	}
	return s.specSecrets(t, "", data, fn)
}

func (s *Spec) specSecrets(t *Type, path string, data map[string]any, fn func(path string, t *Type, v any) (any, error)) error {
	for _, name := range specKeys(t.Fields) {
		f, v := t.Fields[name], data[name]
		ft, err := s.resolve(f.Type)
		if v == nil || err != nil || (ft.Sensitivity != Secret && ft.Schema != nil) {
			continue
		}
		items, list := []any{v}, f.List
		if list {
			items, _ = v.([]any)
		}
		for i, item := range items {
			p := specJoin(path, name)
			if list {
				p = fmt.Sprintf("%s[%d]", p, i)
			}
			if ft.Sensitivity == Secret {
				if items[i], err = fn(p, ft, item); err != nil {
					return err
				}
			} else if m, ok := item.(map[string]any); ok {
				if err := s.specSecrets(ft, p, m, fn); err != nil {
					return err
				}
			}
		}
		if !list {
			data[name] = items[0]
		}
	}
	return nil
}

func (s *Spec) secretField(typ, field string) bool {
	fields, err := s.Fields(typ)
	if err != nil || fields[field] == nil {
		return false
	}
	return s.specHoldsSecret(fields[field].Type, map[string]bool{})
}

func (s *Spec) specHoldsSecret(typ string, seen map[string]bool) bool {
	t, err := s.resolve(typ)
	if err != nil || seen[typ] {
		return false
	}
	seen[typ] = true
	if t.Sensitivity == Secret {
		return true
	}
	for _, f := range t.Fields {
		if s.specHoldsSecret(f.Type, seen) {
			return true
		}
	}
	return false
}

func (s *Spec) secretRoot(typ string) string {
	root := typ
	for n, i := typ, 0; s.Types[n] != nil && i <= len(s.Types); n, i = s.Types[n].Extends, i+1 {
		if s.Types[n].Sensitivity == Secret {
			root = n
		}
	}
	return root
}

func specKeys[V any](m map[string]V) []string {
	return slices.Sorted(maps.Keys(m))
}
