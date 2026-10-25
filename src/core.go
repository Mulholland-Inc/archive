package archive

import (
	"context"
	"crypto/cipher"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
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

	embedOff  string
	iterative bool
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
			if err := BackupBeforeUpgrade(ctx, dir, backupBuild()); err != nil {
				return nil, fmt.Errorf("backup: %w", err)
			}
			if dsn, stop, err = StartPostgres(ctx, dir); err != nil {
				return nil, fmt.Errorf("start postgres: %w", err)
			}
		}
	}
	dim, err := strconv.Atoi(coreEnv("ARCHIVE_EMBED_DIM", "768"))
	if err != nil || dim < 1 || dim > 16000 {
		stop()
		return nil, fmt.Errorf("ARCHIVE_EMBED_DIM %q: want a number of dimensions from 1 to 16000", os.Getenv("ARCHIVE_EMBED_DIM"))
	}
	st, err := OpenStore(ctx, dsn, prefix, authPassword(key))
	if err != nil {
		stop()
		return nil, fmt.Errorf("open store: %w", err)
	}
	var temp *float64
	if v := os.Getenv("ARCHIVE_LLM_TEMPERATURE"); v != "" {
		t, err := strconv.ParseFloat(v, 64)
		if err != nil || t < 0 || t > 2 {
			stop()
			return nil, fmt.Errorf("ARCHIVE_LLM_TEMPERATURE %q: want a number from 0 to 2", v)
		}
		temp = &t
	}
	a, err := newArchive(ctx, st, IntakeConfig{
		Endpoint: coreEnv("ARCHIVE_LLM_URL", "http://localhost:11434/v1"),
		Model:    os.Getenv("ARCHIVE_LLM_MODEL"),
		APIKey:   os.Getenv("ARCHIVE_LLM_KEY"),
		Labels:   os.Getenv("ARCHIVE_LABELS"),
		Hints:    coreHints(os.Getenv("ARCHIVE_LOCALE")),
		Vision:   os.Getenv("ARCHIVE_VISION_MODEL"),
		Whisper:  os.Getenv("ARCHIVE_WHISPER_MODEL"),
		Embed:    os.Getenv("ARCHIVE_EMBED_MODEL"),
		EmbedDim: dim,

		Temperature: temp,
	})
	if err == nil {
		a.prefix, a.aead, a.stop = prefix, aead, stop
		a.blobs, a.uploads, a.linkKey = blobs, uploads, secretDerive(key, "files")
		err = a.migrateSpaces(ctx)
	}
	if err == nil {
		err = a.searchIndexes(ctx, a.Spec())
	}
	if err == nil {
		err = a.embedSetup(ctx)
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
	a.intake = NewIntake(cfg, a.Spec, a.intakeSave)
	return a, nil
}

func (a *Archive) as(ctx context.Context, fn func(context.Context) error) error {
	ac := ActorFrom(ctx)
	if ac == nil {
		return fmt.Errorf("not signed in: %w", ErrUnauthorized)
	}
	return a.st.As(ctx, ac.Role, fn)
}

type outsideKey struct{}

// outside marks a request that came through the engine's own API, not a module's code: a guarded
// type is written there only by an admin of its space.
func outside(ctx context.Context) context.Context { return context.WithValue(ctx, outsideKey{}, "api") }

// byAgent marks a request that came through the engine's own MCP tools: an agent drafts a guarded
// type through the module's tools, for a person to decide, and never writes it itself.
func byAgent(ctx context.Context) context.Context {
	return context.WithValue(ctx, outsideKey{}, "agent")
}

// guard refuses a write of a guarded type from outside: always to an agent, and to anyone else who
// is not an admin of the space.
func (a *Archive) guard(ctx context.Context, typ string, space uuid.UUID) error {
	from := ctx.Value(outsideKey{})
	if from == nil || !a.Spec().Types[typ].Guarded {
		return nil
	}
	if from == "agent" {
		return fmt.Errorf("an agent does not write %s here; the product's own tools draft it for a person to decide: %w", typ, ErrForbidden)
	}
	var admin bool
	if err := a.st.Q(ctx).QueryRow(ctx, `SELECT $1::uuid = ANY(my_spaces('admin'))`, space).Scan(&admin); err != nil {
		return err
	}
	if !admin {
		return fmt.Errorf("only an admin of the space writes %s here; the product writes it through its own pages: %w", typ, ErrForbidden)
	}
	return nil
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

func (a *Archive) List(ctx context.Context, typ, space string, where map[string]any, limit int, after uuid.UUID) ([]*Record, error) {
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
	return coreDo(a, ctx, func(ctx context.Context) ([]*Record, error) {
		f := Filter{Types: spec.Subtypes(typ), Where: norm, Keys: keys, Limit: limit, After: after}
		if space != "" {
			if f.Space, err = a.coreSpace(ctx, space); err != nil {
				return nil, err
			}
		}
		return a.st.ListRecords(ctx, f)
	})
}

func (a *Archive) Get(ctx context.Context, typ string, id uuid.UUID) (*Record, error) {
	spec := a.Spec()
	if err := coreHasType(spec, typ); err != nil {
		return nil, err
	}
	return coreDo(a, ctx, func(ctx context.Context) (*Record, error) {
		r, err := a.st.GetRecord(ctx, id)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(spec.Subtypes(typ), r.Type) {
			return nil, fmt.Errorf("%s %s: %w", typ, id, ErrNotFound)
		}
		return r, nil
	})
}

func (a *Archive) Create(ctx context.Context, space, typ string, data map[string]any) (*Record, error) {
	spec := a.Spec()
	if err := coreHasType(spec, typ); err != nil {
		return nil, err
	}
	return coreDo(a, ctx, func(ctx context.Context) (*Record, error) {
		sid, err := a.coreSpace(ctx, space)
		if err != nil {
			return nil, err
		}
		if err := a.guard(ctx, typ, sid); err != nil {
			return nil, err
		}
		out, originals, err := a.coreCheck(ctx, spec, typ, data)
		if err != nil {
			return nil, err
		}
		r := &Record{Type: typ, SpecVersion: spec.Version, Space: sid}
		if err := a.coreSave(ctx, r, out, nil); err != nil {
			return nil, err
		}
		return r, a.coreProvenance(ctx, spec, r, data, originals)
	})
}

func (a *Archive) coreSpace(ctx context.Context, name string) (uuid.UUID, error) {
	var id uuid.UUID
	q, args := `SELECT id FROM spaces WHERE name = $1`, []any{name}
	if name == "" {
		home := uuid.Nil
		if ac := ActorFrom(ctx); ac != nil {
			home = ac.Home
		}
		q, args = `SELECT id FROM spaces WHERE id = ANY(my_spaces('editor')) ORDER BY id = $1 DESC, id LIMIT 1`, []any{home}
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

// intakeSave saves what intake accepted through the same checks as any other write, references
// included, so a model cannot store what a person could not.
func (a *Archive) intakeSave(ctx context.Context, r *Record, data map[string]any, prov []*Provenance) error {
	var out map[string]any
	if err := a.as(ctx, func(ctx context.Context) (err error) {
		out, _, err = a.coreCheck(ctx, a.Spec(), r.Type, data)
		return err
	}); err != nil {
		return err
	}
	// in place, since coreSave seals secrets in the map it is given and intake shows the caller that
	clear(data)
	maps.Copy(data, out)
	return a.coreSave(ctx, r, data, prov)
}

func (a *Archive) coreSave(ctx context.Context, r *Record, data map[string]any, prov []*Provenance) error {
	return a.as(ctx, func(ctx context.Context) error {
		spec := a.Spec()
		rows, err := a.seal(spec, r.Type, data, nil)
		if err != nil {
			return err
		}
		if r.Data, err = json.Marshal(data); err != nil {
			return err
		}
		if err := a.st.CreateRecord(ctx, r); err != nil {
			return err
		}
		if err := a.secretsPut(ctx, r, rows); err != nil {
			return err
		}
		if err := a.coreLinks(ctx, spec, r, data); err != nil {
			return err
		}
		for _, p := range prov {
			p.RecordID = r.ID
		}
		if len(prov) == 0 {
			return nil
		}
		return a.st.AddProvenance(ctx, prov)
	})
}

func (a *Archive) coreLinks(ctx context.Context, spec *Spec, r *Record, data map[string]any) error {
	fields, err := spec.Fields(r.Type)
	if err != nil {
		return err
	}
	var ls []*Link
	for _, name := range specKeys(fields) {
		if fields[name].Type != specReference || data[name] == nil {
			continue
		}
		vals := []any{data[name]}
		if l, ok := data[name].([]any); ok {
			vals = l
		}
		for _, v := range vals {
			s, _ := v.(string)
			if id, err := uuid.Parse(s); err == nil {
				ls = append(ls, &Link{ToID: id, Kind: name})
			}
		}
	}
	return a.st.SetLinks(ctx, r, ls)
}

func (a *Archive) Replace(ctx context.Context, typ string, id uuid.UUID, data map[string]any) (*Record, error) {
	return coreDo(a, ctx, func(ctx context.Context) (*Record, error) {
		if err := a.coreLock(ctx, id); err != nil {
			return nil, err
		}
		r, err := a.Get(ctx, typ, id)
		if err != nil {
			return nil, err
		}
		if err := a.guard(ctx, typ, r.Space); err != nil {
			return nil, err
		}
		return a.coreUpdate(ctx, r, data, data)
	})
}

// coreLock holds one object until the transaction ends, so a read-merge-write such as Patch reads
// what the last writer committed instead of overwriting it. An advisory lock rather than FOR UPDATE,
// which a viewer may not take and would turn a clean 403 into a permission error.
func (a *Archive) coreLock(ctx context.Context, id uuid.UUID) error {
	_, err := a.st.Q(ctx).Exec(ctx, `SELECT pg_advisory_xact_lock(7274, hashtext($1::text))`, id)
	return err
}

func (a *Archive) Patch(ctx context.Context, typ string, id uuid.UUID, patch map[string]any) (*Record, error) {
	return a.PatchIf(ctx, typ, id, patch, nil)
}

// PatchIf patches an object as Patch does, if check, given the object as it stands under the
// object's lock, returns nil; otherwise nothing is written and check's error is returned as it
// is. Two writers that each check what they read, such as a revision, cannot both pass on the
// same one: the second waits for the first and checks what it wrote.
func (a *Archive) PatchIf(ctx context.Context, typ string, id uuid.UUID, patch map[string]any, check func(*Record) error) (*Record, error) {
	return coreDo(a, ctx, func(ctx context.Context) (*Record, error) {
		if err := a.coreLock(ctx, id); err != nil {
			return nil, err
		}
		r, err := a.Get(ctx, typ, id)
		if err != nil {
			return nil, err
		}
		if err := a.guard(ctx, typ, r.Space); err != nil {
			return nil, err
		}
		if check != nil {
			if err := check(r); err != nil {
				return nil, err
			}
		}
		data := map[string]any{}
		if err := json.Unmarshal(r.Data, &data); err != nil {
			return nil, err
		}
		for k, v := range patch {
			if v == nil {
				delete(data, k)
			} else {
				data[k] = v
			}
		}
		return a.coreUpdate(ctx, r, data, patch)
	})
}

func (a *Archive) coreUpdate(ctx context.Context, r *Record, data, changed map[string]any) (*Record, error) {
	spec := a.Spec()
	var writable bool
	if err := a.st.Q(ctx).QueryRow(ctx, `SELECT $1::uuid = ANY(my_spaces('editor'))`, r.Space).Scan(&writable); err != nil {
		return nil, err
	}
	if !writable {
		return nil, fmt.Errorf("you cannot change objects in this space: %w", ErrForbidden)
	}
	out, originals, err := a.coreCheck(ctx, spec, r.Type, data)
	if err != nil {
		return nil, err
	}
	old := map[string]any{}
	if err := json.Unmarshal(r.Data, &old); err != nil {
		return nil, err
	}
	rows, err := a.seal(spec, r.Type, out, old)
	if err != nil {
		return nil, err
	}
	if err := a.secretsPut(ctx, r, rows); err != nil {
		return nil, err
	}
	if r.Data, err = json.Marshal(out); err != nil {
		return nil, err
	}
	r.SpecVersion = spec.Version
	if err := a.st.UpdateRecord(ctx, r); errors.Is(err, ErrNotFound) {
		return nil, fmt.Errorf("you cannot change objects in this space: %w", ErrForbidden)
	} else if err != nil {
		return nil, err
	}
	if err := a.coreLinks(ctx, spec, r, out); err != nil {
		return nil, err
	}
	return r, a.coreProvenance(ctx, spec, r, changed, originals)
}

func (a *Archive) Delete(ctx context.Context, typ string, id uuid.UUID) error {
	return a.as(ctx, func(ctx context.Context) error {
		r, err := a.Get(ctx, typ, id)
		if err != nil {
			return err
		}
		if err := a.guard(ctx, typ, r.Space); err != nil {
			return err
		}
		if err := a.st.DeleteRecord(ctx, id); errors.Is(err, ErrNotFound) {
			return fmt.Errorf("you cannot delete objects in this space: %w", ErrForbidden)
		} else {
			return err
		}
	})
}

// Restrict hides an object from everyone in its space but the space's admins and owners and the
// user who made it, or opens it again. Only an admin of the space may; schema.sql's
// records_restrict trigger and the records policy hold that, here and in psql alike.
func (a *Archive) Restrict(ctx context.Context, typ string, id uuid.UUID, restricted bool) (*Record, error) {
	return coreDo(a, ctx, func(ctx context.Context) (*Record, error) {
		if _, err := a.Get(ctx, typ, id); err != nil {
			return nil, err
		}
		if _, err := a.st.Q(ctx).Exec(ctx, `UPDATE records SET restricted = $2 WHERE id = $1`, id, restricted); err != nil {
			return nil, storeErr(err)
		}
		return a.Get(ctx, typ, id)
	})
}

func (a *Archive) Provenance(ctx context.Context, typ string, id uuid.UUID) ([]*Provenance, error) {
	return coreDo(a, ctx, func(ctx context.Context) ([]*Provenance, error) {
		if _, err := a.Get(ctx, typ, id); err != nil {
			return nil, err
		}
		return a.st.ListProvenance(ctx, id)
	})
}

// Citation says which record backs a field of another: the source, the words as written there,
// the page, and how sure the person citing it is (1 unless they say).
type Citation struct {
	Field      string   `json:"field" jsonschema:"the field the source backs, or empty for the whole object"`
	Source     string   `json:"source" jsonschema:"id of the record that is the evidence, such as a register, letter or certificate"`
	Original   string   `json:"original,omitempty" jsonschema:"the words as written in the source"`
	Page       int      `json:"page,omitempty" jsonschema:"page in the source, if it has pages"`
	Confidence *float64 `json:"confidence,omitempty" jsonschema:"0 to 1, how sure the citation is; 1 when left out"`
}

// Cite records what backs an object's fields. It needs editor in the object's space, a field the
// type has, and a source the caller can read; the provenance policy then shows each citation
// only to those who can read both ends.
func (a *Archive) Cite(ctx context.Context, typ string, id uuid.UUID, cs []Citation) ([]*Provenance, error) {
	return coreDo(a, ctx, func(ctx context.Context) ([]*Provenance, error) {
		r, err := a.Get(ctx, typ, id)
		if err != nil {
			return nil, err
		}
		if err := a.guard(ctx, typ, r.Space); err != nil {
			return nil, err
		}
		var writable bool
		if err := a.st.Q(ctx).QueryRow(ctx, `SELECT $1::uuid = ANY(my_spaces('editor'))`, r.Space).Scan(&writable); err != nil {
			return nil, err
		}
		if !writable {
			return nil, fmt.Errorf("you cannot cite in this space: %w", ErrForbidden)
		}
		spec := a.Spec()
		fields, err := spec.Fields(r.Type)
		if err != nil {
			return nil, err
		}
		verr := &ValidationError{Type: "citation"}
		var ps []*Provenance
		for i, c := range cs {
			at := fmt.Sprintf("citations[%d].", i)
			if _, ok := fields[c.Field]; c.Field != "" && !ok {
				verr.Errors = append(verr.Errors, FieldError{Path: at + "field", Message: r.Type + " has no field " + c.Field})
			}
			sid, err := uuid.Parse(c.Source)
			if err == nil {
				_, err = a.st.GetRecord(ctx, sid)
			}
			if err != nil {
				verr.Errors = append(verr.Errors, FieldError{Path: at + "source", Message: "want the id of a record you can read"})
				continue
			}
			conf := 1.0
			if c.Confidence != nil {
				conf = *c.Confidence
			}
			if conf < 0 || conf > 1 {
				verr.Errors = append(verr.Errors, FieldError{Path: at + "confidence", Message: "want 0 to 1"})
			}
			p := &Provenance{RecordID: r.ID, FieldPath: c.Field, Method: "cite", Confidence: conf, Original: c.Original, Page: c.Page, SourceID: &sid}
			if spec.secretField(r.Type, c.Field) {
				p.Original = ""
			}
			ps = append(ps, p)
		}
		if len(cs) == 0 {
			verr.Errors = append(verr.Errors, FieldError{Path: "citations", Message: "want at least one"})
		}
		if len(verr.Errors) > 0 {
			return nil, verr
		}
		if err := a.st.AddProvenance(ctx, ps); err != nil {
			return nil, err
		}
		return a.st.ListProvenance(ctx, r.ID)
	})
}

func (a *Archive) Links(ctx context.Context, typ string, id uuid.UUID) ([]*Link, error) {
	return coreDo(a, ctx, func(ctx context.Context) ([]*Link, error) {
		if _, err := a.Get(ctx, typ, id); err != nil {
			return nil, err
		}
		ls, err := a.st.ListLinks(ctx, id)
		if ls == nil {
			ls = []*Link{}
		}
		return ls, err
	})
}

func (a *Archive) coreCheck(ctx context.Context, spec *Spec, typ string, data map[string]any) (map[string]any, map[string]string, error) {
	out, originals, err := spec.Normalize(typ, data, a.hints)
	if err != nil {
		return nil, nil, err
	}
	fields, err := spec.Fields(typ)
	if err != nil {
		return nil, nil, err
	}
	verr := &ValidationError{Type: typ}
	for name, f := range fields {
		if f.Type != "reference" || out[name] == nil {
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
		return nil, nil, verr
	}
	return out, originals, nil
}

func (a *Archive) coreCheckRef(ctx context.Context, spec *Spec, to string, v any) string {
	s, _ := v.(string)
	id, err := uuid.Parse(s)
	if err != nil {
		return "reference is not a uuid"
	}
	r, err := a.st.GetRecord(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return "no object " + s
	}
	if err != nil {
		return err.Error()
	}
	if !slices.Contains(spec.Subtypes(to), r.Type) {
		return fmt.Sprintf("%s is a %s, want %s", s, r.Type, to)
	}
	return ""
}

func (a *Archive) coreProvenance(ctx context.Context, spec *Spec, r *Record, fields map[string]any, originals map[string]string) error {
	var ps []*Provenance
	for _, k := range slices.Sorted(maps.Keys(fields)) {
		if fields[k] == nil {
			continue
		}
		p := &Provenance{ID: coreNewID(), RecordID: r.ID, FieldPath: k, Method: "manual", Confidence: 1, Original: originals[k]}
		if spec.secretField(r.Type, k) {
			p.Original = ""
		}
		ps = append(ps, p)
	}
	if len(ps) == 0 {
		return nil
	}
	return a.st.AddProvenance(ctx, ps)
}

type Description struct {
	Spec    *Spec                `json:"spec"`
	Samples map[string][]*Record `json:"samples,omitempty"`
}

func (a *Archive) Describe(ctx context.Context, samples int) (*Description, error) {
	spec := a.Spec()
	d := &Description{Spec: spec}
	if samples <= 0 {
		return d, nil
	}
	d.Samples = map[string][]*Record{}
	for name, t := range spec.Types {
		if t.Builtin || t.Schema != nil {
			continue
		}
		rs, err := a.Sample(ctx, name, samples)
		if err != nil {
			return nil, err
		}
		d.Samples[name] = rs
	}
	return d, nil
}

func (a *Archive) Sample(ctx context.Context, typ string, n int) ([]*Record, error) {
	if err := coreHasType(a.Spec(), typ); err != nil {
		return nil, err
	}
	if n <= 0 || n > 100 {
		n = 5
	}
	return coreDo(a, ctx, func(ctx context.Context) ([]*Record, error) {
		return a.st.ListRecords(ctx, Filter{Types: []string{typ}, Limit: n})
	})
}

func (a *Archive) Propose(ctx context.Context, ops []Op) (*Change, error) {
	spec := a.Spec()
	next, err := coreNext(spec, ops)
	if err != nil {
		return nil, err
	}
	by := ""
	if ac := ActorFrom(ctx); ac != nil {
		by = ac.Label
	}
	c := &Change{ID: coreNewID(), ProposedBy: by, Status: "proposed", Ops: ops,
		FromVer: spec.Version, ToVer: next.Version, CreatedAt: time.Now().UTC()}
	return c, a.as(ctx, func(ctx context.Context) error { return a.st.PutChange(ctx, c) })
}

func coreNext(spec *Spec, ops []Op) (*Spec, error) {
	if len(ops) == 0 {
		return nil, &ValidationError{Type: "change", Errors: []FieldError{{Path: "ops", Message: "no ops"}}}
	}
	return spec.Apply(ops)
}

type Preview struct {
	Change   *Change         `json:"change"`
	Touched  int             `json:"touched"`
	Failing  int             `json:"failing"`
	Failures []PreviewRecord `json:"failures,omitempty"`
	Samples  []PreviewRecord `json:"samples,omitempty"`
}

type PreviewRecord struct {
	ID     uuid.UUID      `json:"id"`
	Type   string         `json:"type"`
	Before map[string]any `json:"before,omitempty"`
	After  map[string]any `json:"after,omitempty"`
	Errors []FieldError   `json:"errors,omitempty"`
}

const (
	corePreviewSamples  = 3
	corePreviewFailures = 20
)

func (a *Archive) Preview(ctx context.Context, id uuid.UUID) (*Preview, error) {
	return coreDo(a, ctx, func(ctx context.Context) (*Preview, error) { return a.corePreview(ctx, id) })
}

func (a *Archive) corePreview(ctx context.Context, id uuid.UUID) (*Preview, error) {
	c, spec, next, err := a.coreProposed(ctx, id)
	if err != nil {
		return nil, err
	}
	p := &Preview{Change: c}
	err = a.coreEach(ctx, spec, next, c.Ops, func(r *Record, before, after map[string]any) error {
		p.Touched++
		pr := PreviewRecord{ID: r.ID, Type: r.Type, Before: before, After: after}
		if err := coreValidate(next, r.Type, after); err != nil {
			p.Failing++
			if len(p.Failures) < corePreviewFailures {
				p.Failures = append(p.Failures, PreviewRecord{ID: r.ID, Type: r.Type, Errors: coreFieldErrors(err)})
			}
		}
		if len(p.Samples) < corePreviewSamples {
			p.Samples = append(p.Samples, pr)
		}
		return nil
	})
	return p, err
}

func (a *Archive) Apply(ctx context.Context, id uuid.UUID) (*Change, error) {
	a.wmu.Lock()
	defer a.wmu.Unlock()
	var c *Change
	var spec, next *Spec
	err := a.as(ctx, func(ctx context.Context) (err error) {
		if c, spec, next, err = a.coreProposed(ctx, id); err != nil {
			return err
		}
		undo := &Change{ID: coreUndoID(c.ID), ProposedBy: "revert:" + c.ID.String(), Status: "proposed",
			Ops: DiffSpecs(next, spec), FromVer: next.Version, ToVer: next.Version + 1, CreatedAt: time.Now().UTC()}
		c.Status = "applied"
		return a.st.ApplyChange(ctx, next, c, undo)
	})
	if err != nil {
		return nil, err
	}
	a.setSpec(next)
	err = a.st.System(ctx, func(ctx context.Context) error {
		return a.coreEach(ctx, spec, next, c.Ops, func(r *Record, before, after map[string]any) error {
			if next.Types[r.Type] == nil {
				return nil
			}
			changed, err := a.sealRecord(ctx, next, r, after)
			if err != nil {
				return err
			}
			changed = changed || !specSame(before, after)
			valid := coreValidate(next, r.Type, after) == nil
			if !changed && !valid {
				return nil
			}
			if valid {
				r.SpecVersion = next.Version
			}
			if changed {
				b, err := json.Marshal(after)
				if err != nil {
					return err
				}
				r.Data = b
			}
			return a.st.UpdateRecord(ctx, r)
		})
	})
	if err == nil {
		err = a.searchIndexes(ctx, next)
	}
	return c, err
}

func (a *Archive) Revert(ctx context.Context, id uuid.UUID) (*Change, error) {
	c, err := coreDo(a, ctx, func(ctx context.Context) (*Change, error) { return a.st.GetChange(ctx, id) })
	if err != nil {
		return nil, err
	}
	if c.Status != "applied" {
		return nil, fmt.Errorf("change %s is %s, not applied: %w", id, c.Status, ErrConflict)
	}
	undo, err := a.Apply(ctx, coreUndoID(id))
	if err != nil {
		return nil, fmt.Errorf("revert %s: %w", id, err)
	}
	c.Status = "reverted"
	if err := a.as(ctx, func(ctx context.Context) error { return a.st.PutChange(ctx, c) }); err != nil {
		return nil, err
	}
	return undo, nil
}

func (a *Archive) Changes(ctx context.Context) ([]*Change, error) {
	return coreDo(a, ctx, func(ctx context.Context) ([]*Change, error) {
		out, err := storeAll[Change](ctx, a.st, `SELECT id, proposed_by, status, ops, from_version AS from_ver, to_version AS to_ver, created_at
			FROM changes WHERE status <> 'proposed' OR proposed_by NOT LIKE 'revert:%' ORDER BY created_at DESC, id DESC LIMIT 200`)
		if out == nil {
			out = []*Change{}
		}
		return out, err
	})
}

func (a *Archive) ApplySpec(ctx context.Context, b []byte, dryRun bool) (*Preview, error) {
	want, err := ParseSpec(b)
	if err != nil {
		return nil, err
	}
	ops := DiffSpecs(a.Spec(), want)
	if len(ops) == 0 {
		return nil, nil
	}
	c, err := a.Propose(ctx, ops)
	if err != nil {
		return nil, err
	}
	p, err := a.Preview(ctx, c.ID)
	if err != nil || dryRun {
		return p, err
	}
	p.Change, err = a.Apply(ctx, c.ID)
	return p, err
}

func (a *Archive) coreProposed(ctx context.Context, id uuid.UUID) (*Change, *Spec, *Spec, error) {
	c, err := a.st.GetChange(ctx, id)
	if err != nil {
		return nil, nil, nil, err
	}
	if c.Status != "proposed" {
		return nil, nil, nil, fmt.Errorf("change %s is %s, not proposed: %w", id, c.Status, ErrConflict)
	}
	spec, err := a.st.CurrentSpec(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	a.setSpec(spec)
	if spec.Version != c.FromVer {
		return nil, nil, nil, fmt.Errorf("change %s is based on spec v%d, current is v%d; propose it again: %w", id, c.FromVer, spec.Version, ErrConflict)
	}
	next, err := coreNext(spec, c.Ops)
	return c, spec, next, err
}

func coreTouched(old, next *Spec, ops []Op) []string {
	in := map[string]bool{}
	for _, op := range ops {
		for _, s := range []*Spec{old, next} {
			if _, ok := s.Types[op.Type]; ok {
				for _, t := range s.Subtypes(op.Type) {
					in[t] = true
				}
			}
		}
	}
	for grew := true; grew; {
		grew = false
		for name := range next.Types {
			fields, _ := next.Fields(name)
			for _, f := range fields {
				if in[f.Type] && !in[name] {
					in[name], grew = true, true
				}
			}
		}
	}
	return specKeys(in)
}

func (a *Archive) coreEach(ctx context.Context, old, next *Spec, ops []Op, fn func(r *Record, before, after map[string]any) error) error {
	types := coreTouched(old, next, ops)
	if len(types) == 0 {
		return nil
	}
	f := Filter{Types: types, Limit: 500}
	for {
		rs, err := a.st.ListRecords(ctx, f)
		if err != nil {
			return err
		}
		for _, r := range rs {
			before := map[string]any{}
			if err := json.Unmarshal(r.Data, &before); err != nil {
				return fmt.Errorf("record %s: %w", r.ID, err)
			}
			if err := fn(r, before, coreMigrate(old, r.Type, before, ops)); err != nil {
				return err
			}
		}
		if len(rs) < f.Limit {
			return nil
		}
		f.After = rs[len(rs)-1].ID
	}
}

func coreMigrate(old *Spec, typ string, data map[string]any, ops []Op) map[string]any {
	out := maps.Clone(data)
	for _, op := range ops {
		if op.Type != typ && !slices.Contains(old.Subtypes(op.Type), typ) {
			continue
		}
		switch op.Op {
		case "rename_field":
			if v, ok := out[op.Field]; ok {
				delete(out, op.Field)
				out[op.To] = v
			}
		case "remove_field":
			delete(out, op.Field)
		}
	}
	return out
}

func coreValidate(s *Spec, typ string, data map[string]any) error {
	if _, ok := s.Types[typ]; !ok {
		return &ValidationError{Type: typ, Errors: []FieldError{{Path: "", Message: "type " + typ + " no longer exists"}}}
	}
	return s.Validate(typ, data)
}

func (a *Archive) IntakeDir(ctx context.Context, space, dir, typ string) ([]*IntakeResult, error) {
	if err := a.adminOnly(ctx, "reads folders on the server"); err != nil {
		return nil, err
	}
	if err := coreHasType(a.Spec(), typ); err != nil {
		return nil, err
	}
	raws, err := coreDo(a, ctx, func(ctx context.Context) ([]*Raw, error) {
		sid, err := a.coreSpace(ctx, space)
		if err != nil {
			return nil, err
		}
		return ScanFolder(ctx, a.st, dir, sid)
	})
	if err != nil {
		return nil, err
	}
	var out []*IntakeResult
	var errs []error
	for _, raw := range raws {
		res, err := a.ingest(ctx, raw, typ)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", raw.ExternalID, err))
			continue
		}
		out = append(out, res)
	}
	return out, errors.Join(errs...)
}

func coreFieldErrors(err error) []FieldError {
	var ve *ValidationError
	if errors.As(err, &ve) {
		return ve.Errors
	}
	return []FieldError{{Message: err.Error()}}
}

func coreHasType(s *Spec, typ string) error {
	t, err := s.resolve(typ)
	if err != nil {
		return fmt.Errorf("type %q: %w", typ, ErrNotFound)
	}
	if t.Schema != nil {
		return fmt.Errorf("%s is a scalar type, not a record type: %w", typ, ErrNotFound)
	}
	return nil
}

func coreNewID() uuid.UUID { return uuid.Must(uuid.NewV7()) }

func coreUndoID(id uuid.UUID) uuid.UUID { return uuid.NewSHA1(id, []byte("undo")) }

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
