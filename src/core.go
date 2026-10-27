package main

import (
	"context"
	"crypto/cipher"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/text/currency"
	"golang.org/x/text/language"
)

type Archive struct {
	st     *pgStore
	intake *Intake
	hints  Hints
	stop   func() error
	prefix string
	base   string
	aead   cipher.AEAD

	blobs    blobStore
	uploads  string
	linkKey  []byte
	wake     chan struct{}
	tusLocks sync.Map

	mu   sync.RWMutex
	spec *Spec
	wmu  sync.Mutex
}

var corePrefixRe = regexp.MustCompile(`^[a-z][a-z0-9]{0,15}$`)

func Open(ctx context.Context) (*Archive, error) {
	dir := coreEnv("ARCHIVE_DATA", "./data")
	prefix := coreEnv("ARCHIVE_INSTANCE", "a")
	if !corePrefixRe.MatchString(prefix) {
		return nil, fmt.Errorf("ARCHIVE_INSTANCE %q: want 1-16 lowercase letters and digits, starting with a letter", prefix)
	}
	key, err := secretLoadKey(dir)
	if err != nil {
		return nil, fmt.Errorf("secret key: %w", err)
	}
	aead, err := secretAEAD(key)
	if err != nil {
		return nil, err
	}
	blobs, err := blobOpen(dir)
	if err != nil {
		return nil, err
	}
	uploads := filepath.Join(dir, "uploads")
	if err := os.MkdirAll(uploads, 0o700); err != nil {
		return nil, err
	}
	dsn, stop := os.Getenv("DATABASE_URL"), func() error { return nil }
	if dsn == "" {
		if dsn = pgLive(dir); dsn == "" {
			if err := BackupBeforeUpgrade(ctx, dir, cliVersion); err != nil {
				return nil, fmt.Errorf("backup: %w", err)
			}
			if dsn, stop, err = StartPostgres(ctx, dir); err != nil {
				return nil, fmt.Errorf("start postgres: %w", err)
			}
		}
	}
	st, err := OpenStore(ctx, dsn, prefix, authPassword(key))
	if err != nil {
		stop()
		return nil, fmt.Errorf("open store: %w", err)
	}
	a, err := newArchive(ctx, st, IntakeConfig{
		Endpoint: coreEnv("ARCHIVE_LLM_URL", "http://localhost:11434/v1"),
		Model:    os.Getenv("ARCHIVE_LLM_MODEL"),
		APIKey:   os.Getenv("ARCHIVE_LLM_KEY"),
		Hints:    coreHints(os.Getenv("ARCHIVE_LOCALE")),
		Vision:   os.Getenv("ARCHIVE_VISION_MODEL"),
		Whisper:  os.Getenv("ARCHIVE_WHISPER_MODEL"),
	})
	if err == nil {
		a.prefix, a.aead, a.stop = prefix, aead, stop
		a.blobs, a.uploads, a.linkKey = blobs, uploads, secretDerive(key, "files")
		err = a.searchIndexes(ctx, a.Spec())
	}
	if err != nil {
		st.Close()
		stop()
		return nil, err
	}
	return a, nil
}

func newArchive(ctx context.Context, st *pgStore, cfg IntakeConfig) (*Archive, error) {
	var spec *Spec
	err := st.System(ctx, func(ctx context.Context) (err error) {
		spec, err = st.CurrentSpec(ctx)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("load spec: %w", err)
	}
	if err := spec.Check(); err != nil {
		return nil, fmt.Errorf("load spec: %w", err)
	}
	a := &Archive{st: st, spec: spec, hints: cfg.Hints, wake: make(chan struct{}, 1), base: strings.TrimRight(coreEnv("ARCHIVE_URL", "http://localhost:8080"), "/")}
	a.intake = NewIntake(cfg, a.Spec, a.coreSave)
	return a, nil
}

func (a *Archive) as(ctx context.Context, fn func(context.Context) error) error {
	ac := ActorFrom(ctx)
	if ac == nil {
		return fmt.Errorf("not signed in: %w", ErrUnauthorized)
	}
	return a.st.As(ctx, ac.Role, fn)
}

func coreDo[T any](a *Archive, ctx context.Context, fn func(context.Context) (T, error)) (T, error) {
	var out T
	err := a.as(ctx, func(ctx context.Context) (err error) {
		out, err = fn(ctx)
		return err
	})
	return out, err
}

func (a *Archive) Close() error {
	a.st.Close()
	if a.stop != nil {
		return a.stop()
	}
	return nil
}

func (a *Archive) Spec() *Spec {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.spec
}

func (a *Archive) setSpec(s *Spec) {
	a.mu.Lock()
	a.spec = s
	a.mu.Unlock()
}

func (a *Archive) List(ctx context.Context, typ, space string, where map[string]any, limit int, after uuid.UUID, proposed bool) ([]*Entity, error) {
	spec := a.Spec()
	if err := coreHasType(spec, typ); err != nil {
		return nil, err
	}
	fields, err := spec.Fields(typ)
	if err != nil {
		return nil, err
	}
	norm, keys := map[string]any{}, map[string]string{}
	verr := &ValidationError{Type: typ}
	for k, v := range where {
		f, ok := fields[k]
		if !ok {
			verr.Errors = append(verr.Errors, FieldError{Path: k, Message: "unknown field"})
			continue
		}
		if norm[k], err = spec.NormalizeField(f, v, a.hints); err != nil {
			verr.Errors = append(verr.Errors, FieldError{Path: k, Message: err.Error()})
		}
		if s, ok := norm[k].(string); ok && f.Indexed {
			keys[k] = s
		}
	}
	if len(verr.Errors) > 0 {
		return nil, verr
	}
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	return coreDo(a, ctx, func(ctx context.Context) ([]*Entity, error) {
		f := Filter{Types: spec.Subtypes(typ), Where: norm, Keys: keys, Limit: limit, After: after, Proposed: proposed}
		if space != "" {
			if f.Space, err = a.coreSpace(ctx, space); err != nil {
				return nil, err
			}
		}
		return a.st.ListEntities(ctx, f)
	})
}

func (a *Archive) Get(ctx context.Context, typ string, id uuid.UUID, proposed bool) (*Entity, error) {
	spec := a.Spec()
	if err := coreHasType(spec, typ); err != nil {
		return nil, err
	}
	return coreDo(a, ctx, func(ctx context.Context) (*Entity, error) {
		e, err := a.st.GetEntity(ctx, id)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(spec.Subtypes(typ), e.Type) || e.Proposed && !proposed {
			return nil, fmt.Errorf("%s %s: %w", typ, id, ErrNotFound)
		}
		return e, nil
	})
}

func (a *Archive) Create(ctx context.Context, space, typ string, data map[string]any) (*Entity, error) {
	spec := a.Spec()
	if err := coreHasType(spec, typ); err != nil {
		return nil, err
	}
	return coreDo(a, ctx, func(ctx context.Context) (*Entity, error) {
		sid, err := a.coreSpace(ctx, space)
		if err != nil {
			return nil, err
		}
		out, err := a.coreCheck(ctx, spec, typ, data)
		if err != nil {
			return nil, err
		}
		e := &Entity{Type: typ, Space: sid}
		return e, a.coreSave(ctx, e, out)
	})
}

func (a *Archive) coreSpace(ctx context.Context, name string) (uuid.UUID, error) {
	var id uuid.UUID
	q, args := `SELECT id FROM space WHERE name = $1`, []any{name}
	if name == "" {
		home := uuid.Nil
		if ac := ActorFrom(ctx); ac != nil {
			home = ac.Home
		}
		q, args = `SELECT id FROM space WHERE id = ANY(my_spaces('editor')) ORDER BY id = $1 DESC, id LIMIT 1`, []any{home}
	}
	err := a.st.Q(ctx).QueryRow(ctx, q, args...).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		if name == "" {
			return id, fmt.Errorf("no space you can write to: %w", ErrForbidden)
		}
		return id, fmt.Errorf("space %q: %w", name, ErrNotFound)
	}
	return id, err
}

func (a *Archive) coreSave(ctx context.Context, e *Entity, data map[string]any) error {
	return a.as(ctx, func(ctx context.Context) error {
		rows, err := a.seal(a.Spec(), e.Type, data, nil)
		if err != nil {
			return err
		}
		if e.Data, err = json.Marshal(data); err != nil {
			return err
		}
		if err := a.st.CreateEntity(ctx, e); err != nil {
			return err
		}
		return a.secretsPut(ctx, e, rows)
	})
}

func (a *Archive) Replace(ctx context.Context, typ string, id uuid.UUID, data map[string]any) (*Entity, error) {
	return coreDo(a, ctx, func(ctx context.Context) (*Entity, error) {
		e, err := a.Get(ctx, typ, id, true)
		if err != nil {
			return nil, err
		}
		return a.coreUpdate(ctx, e, data)
	})
}

func (a *Archive) Patch(ctx context.Context, typ string, id uuid.UUID, patch map[string]any) (*Entity, error) {
	return coreDo(a, ctx, func(ctx context.Context) (*Entity, error) {
		e, err := a.Get(ctx, typ, id, true)
		if err != nil {
			return nil, err
		}
		return a.corePatch(ctx, e, patch)
	})
}

func (a *Archive) corePatch(ctx context.Context, e *Entity, patch map[string]any) (*Entity, error) {
	data := map[string]any{}
	if err := json.Unmarshal(e.Data, &data); err != nil {
		return nil, err
	}
	for k, v := range patch {
		if v == nil {
			delete(data, k)
		} else {
			data[k] = v
		}
	}
	return a.coreUpdate(ctx, e, data)
}

func (a *Archive) coreUpdate(ctx context.Context, e *Entity, data map[string]any) (*Entity, error) {
	spec := a.Spec()
	var writable bool
	if err := a.st.Q(ctx).QueryRow(ctx, `SELECT $1::uuid = ANY(my_spaces('editor'))`, e.Space).Scan(&writable); err != nil {
		return nil, err
	}
	if !writable {
		return nil, fmt.Errorf("you cannot change objects in this space: %w", ErrForbidden)
	}
	out, err := a.coreCheck(ctx, spec, e.Type, data)
	if err != nil {
		return nil, err
	}
	old := map[string]any{}
	if err := json.Unmarshal(e.Data, &old); err != nil {
		return nil, err
	}
	rows, err := a.seal(spec, e.Type, out, old)
	if err != nil {
		return nil, err
	}
	if err := a.secretsPut(ctx, e, rows); err != nil {
		return nil, err
	}
	if e.Data, err = json.Marshal(out); err != nil {
		return nil, err
	}
	if err := a.st.UpdateEntity(ctx, e); errors.Is(err, ErrNotFound) {
		return nil, fmt.Errorf("you cannot change objects in this space: %w", ErrForbidden)
	} else if err != nil {
		return nil, err
	}
	return e, nil
}

func (a *Archive) Delete(ctx context.Context, typ string, id uuid.UUID) error {
	return a.as(ctx, func(ctx context.Context) error {
		if _, err := a.Get(ctx, typ, id, true); err != nil {
			return err
		}
		if err := a.st.DeleteEntity(ctx, id); errors.Is(err, ErrNotFound) {
			return fmt.Errorf("you cannot delete objects in this space: %w", ErrForbidden)
		} else {
			return err
		}
	})
}

func (a *Archive) Links(ctx context.Context, typ string, id uuid.UUID) ([]*Link, error) {
	return coreDo(a, ctx, func(ctx context.Context) ([]*Link, error) {
		e, err := a.Get(ctx, typ, id, true)
		if err != nil {
			return nil, err
		}
		spec, out := a.Spec(), []*Link{}
		refs := coreRefs(spec, e)
		var to []uuid.UUID
		for _, k := range specKeys(refs) {
			to = append(to, refs[k]...)
		}
		rows, _ := a.st.Q(ctx).Query(ctx, `SELECT id FROM entity WHERE id = ANY($1)`, to)
		seen, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
		if err != nil {
			return nil, err
		}
		for _, k := range specKeys(refs) {
			for _, t := range refs[k] {
				if slices.Contains(seen, t) {
					out = append(out, &Link{FromID: e.ID, ToID: t, Kind: k, FromType: e.Type})
				}
			}
		}
		path := `$.*[*] ? (@ == "` + e.ID.String() + `")`
		in, err := storeAll[Entity](ctx, a.st, `SELECT * FROM entity WHERE id <> $1 AND NOT proposed AND data @? $2::jsonpath ORDER BY id LIMIT 1000`, e.ID, path)
		if err != nil {
			return nil, err
		}
		for _, f := range in {
			refs := coreRefs(spec, f)
			for _, k := range specKeys(refs) {
				if slices.Contains(refs[k], e.ID) {
					out = append(out, &Link{FromID: f.ID, ToID: e.ID, Kind: k, FromType: f.Type})
				}
			}
		}
		return out, nil
	})
}

func coreRefs(spec *Spec, e *Entity) map[string][]uuid.UUID {
	out := map[string][]uuid.UUID{}
	fields, err := spec.Fields(e.Type)
	data := map[string]any{}
	if err != nil || json.Unmarshal(e.Data, &data) != nil {
		return out
	}
	for name, f := range fields {
		if f.Type != specReference || data[name] == nil {
			continue
		}
		vals := []any{data[name]}
		if l, ok := data[name].([]any); ok {
			vals = l
		}
		for _, v := range vals {
			s, _ := v.(string)
			if id, err := uuid.Parse(s); err == nil {
				out[name] = append(out[name], id)
			}
		}
	}
	return out
}

func (a *Archive) coreCheck(ctx context.Context, spec *Spec, typ string, data map[string]any) (map[string]any, error) {
	out, err := spec.Normalize(typ, data, a.hints)
	if err != nil {
		return nil, err
	}
	fields, err := spec.Fields(typ)
	if err != nil {
		return nil, err
	}
	verr := &ValidationError{Type: typ}
	for name, f := range fields {
		if f.Type != specReference || out[name] == nil {
			continue
		}
		vals := []any{out[name]}
		if l, ok := out[name].([]any); ok {
			vals = l
		}
		for i, v := range vals {
			path := name
			if f.List {
				path = fmt.Sprintf("%s[%d]", name, i)
			}
			if msg := a.coreCheckRef(ctx, spec, f.To, v); msg != "" {
				verr.Errors = append(verr.Errors, FieldError{Path: path, Message: msg})
			}
		}
	}
	if len(verr.Errors) > 0 {
		slices.SortFunc(verr.Errors, func(x, y FieldError) int { return strings.Compare(x.Path, y.Path) })
		return nil, verr
	}
	return out, nil
}

func (a *Archive) coreCheckRef(ctx context.Context, spec *Spec, to string, v any) string {
	s, _ := v.(string)
	id, err := uuid.Parse(s)
	if err != nil {
		return "reference is not a uuid"
	}
	e, err := a.st.GetEntity(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return "no object " + s
	}
	if err != nil {
		return err.Error()
	}
	if !slices.Contains(spec.Subtypes(to), e.Type) {
		return fmt.Sprintf("%s is a %s, want %s", s, e.Type, to)
	}
	return ""
}

type Description struct {
	Spec    *Spec                `json:"spec"`
	Samples map[string][]*Entity `json:"samples,omitempty"`
}

func (a *Archive) Describe(ctx context.Context, samples int) (*Description, error) {
	spec := a.Spec()
	d := &Description{Spec: spec}
	if samples <= 0 {
		return d, nil
	}
	d.Samples = map[string][]*Entity{}
	for name, t := range spec.Types {
		if t.Builtin || t.Schema != nil {
			continue
		}
		es, err := a.Sample(ctx, name, samples)
		if err != nil {
			return nil, err
		}
		d.Samples[name] = es
	}
	return d, nil
}

func (a *Archive) Sample(ctx context.Context, typ string, n int) ([]*Entity, error) {
	if err := coreHasType(a.Spec(), typ); err != nil {
		return nil, err
	}
	if n <= 0 || n > 100 {
		n = 5
	}
	return coreDo(a, ctx, func(ctx context.Context) ([]*Entity, error) {
		return a.st.ListEntities(ctx, Filter{Types: []string{typ}, Limit: n})
	})
}

type SpecChange struct {
	Defined []string `json:"defined,omitempty"`
	Removed []string `json:"removed,omitempty"`
}

func (a *Archive) DefineType(ctx context.Context, name string, def *Type) error {
	if def == nil {
		def = &Type{}
	}
	_, err := a.coreTypes(ctx, map[string]*Type{name: def}, false)
	return err
}

func (a *Archive) DeleteType(ctx context.Context, name string) error {
	_, err := a.coreTypes(ctx, map[string]*Type{name: nil}, false)
	return err
}

func (a *Archive) ApplySpec(ctx context.Context, b []byte, dryRun bool) (*SpecChange, error) {
	want, err := ParseSpec(b)
	if err != nil {
		return nil, err
	}
	cur, defs := a.Spec(), map[string]*Type{}
	for name, t := range cur.Types {
		if _, ok := want.Types[name]; !ok && !t.Builtin {
			defs[name] = nil
		}
	}
	for name, t := range want.Types {
		if c := cur.Types[name]; c == nil || !specSame(c, t) {
			defs[name] = t
		}
	}
	if len(defs) == 0 {
		return nil, nil
	}
	return a.coreTypes(ctx, defs, dryRun)
}

func (a *Archive) coreTypes(ctx context.Context, defs map[string]*Type, dryRun bool) (*SpecChange, error) {
	a.wmu.Lock()
	defer a.wmu.Unlock()
	c := &SpecChange{}
	var next *Spec
	err := a.as(ctx, func(ctx context.Context) error {
		cur, err := a.st.CurrentSpec(ctx)
		if err != nil {
			return err
		}
		if next, err = cur.with(defs); err != nil || dryRun {
			return err
		}
		for _, name := range specKeys(defs) {
			if defs[name] == nil {
				c.Removed = append(c.Removed, name)
				if err := a.st.DeleteType(ctx, name); err != nil {
					return err
				}
				continue
			}
			c.Defined = append(c.Defined, name)
			if err := a.st.PutType(ctx, next.Types[name]); err != nil {
				return err
			}
		}
		next.stamp, err = a.st.SpecStamp(ctx)
		return err
	})
	if err != nil || dryRun {
		if dryRun && err == nil {
			for _, name := range specKeys(defs) {
				if defs[name] == nil {
					c.Removed = append(c.Removed, name)
				} else {
					c.Defined = append(c.Defined, name)
				}
			}
		}
		return c, err
	}
	a.setSpec(next)
	return c, a.searchIndexes(ctx, next)
}

func (a *Archive) IntakeDir(ctx context.Context, space, dir, typ string) ([]*IntakeResult, error) {
	if err := coreHasType(a.Spec(), typ); err != nil {
		return nil, err
	}
	items, err := coreDo(a, ctx, func(ctx context.Context) ([]*Item, error) {
		sid, err := a.coreSpace(ctx, space)
		if err != nil {
			return nil, err
		}
		return ScanFolder(ctx, dir, sid, nil)
	})
	if err != nil {
		return nil, err
	}
	var out []*IntakeResult
	var errs []error
	for _, it := range items {
		res, err := a.intake.Ingest(ctx, it, typ)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", it.ExternalID, err))
			continue
		}
		out = append(out, res)
	}
	return out, errors.Join(errs...)
}

func coreHasType(s *Spec, typ string) error {
	t, err := s.resolve(typ)
	if err != nil {
		return fmt.Errorf("type %q: %w", typ, ErrNotFound)
	}
	if t.Schema != nil {
		return fmt.Errorf("%s is a scalar type, not an entity type: %w", typ, ErrNotFound)
	}
	return nil
}

func coreNewID() uuid.UUID { return uuid.Must(uuid.NewV7()) }

func coreEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func coreHints(locale string) Hints {
	h := Hints{Locale: locale, Zone: time.Local.String()}
	if locale == "" {
		return h
	}
	if r, conf := language.Make(locale).Region(); conf != language.No {
		h.Country = r.String()
		if c, ok := currency.FromRegion(r); ok {
			h.Currency = c.String()
		}
	}
	return h
}
