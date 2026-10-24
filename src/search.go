package archive

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	searchK       = 50
	searchRRF     = 60
	embedJobs     = 16
	embedBatch    = 64
	embedChunk    = 1000
	embedChunks   = 32
	searchIndexRe = `^records_f_[0-9a-f]{12}_`
)

var searchSlugRe = regexp.MustCompile(`[^a-z0-9]+`)

type SearchFilter struct {
	Type   string
	Space  string
	Source string
	From   string
	To     string
	Limit  int
}

type SearchHit struct {
	ID         uuid.UUID       `json:"id"`
	Kind       string          `json:"kind"`
	Type       string          `json:"type,omitempty"`
	Source     string          `json:"source,omitempty"`
	Space      uuid.UUID       `json:"space"`
	Date       time.Time       `json:"date"`
	Data       json.RawMessage `json:"data,omitempty"`
	Snippet    string          `json:"snippet,omitempty"`
	Score      float64         `json:"score"`
	TextRank   int             `json:"text_rank,omitempty"`
	VectorRank int             `json:"vector_rank,omitempty"`
}

type searchKey struct {
	id   uuid.UUID
	kind string
}

const searchRecordWhere = `(cardinality($2::text[]) = 0 OR r.type = ANY($2))
	AND ($3::uuid IS NULL OR r.space = $3)
	AND ($5::timestamptz IS NULL OR r.created_at >= $5) AND ($6::timestamptz IS NULL OR r.created_at < $6)
	AND ($4 = '' OR EXISTS (SELECT FROM provenance p JOIN raw s ON s.id = ANY(p.raw_ids) WHERE p.record_id = r.id AND s.source = $4)
		OR EXISTS (SELECT FROM raw s WHERE s.source = $4 AND jsonb_path_exists(s.payload, '$.**.file_id ? (@ == $id)', jsonb_build_object('id', r.id::text))))`

const searchText = `
	WITH q AS (SELECT search_query($1) AS q),
	hit AS (
		SELECT r.id, 'record' AS kind, ts_rank(search_record(r.type, r.data), q.q, 1) AS rank
		FROM records r, q WHERE search_record(r.type, r.data) @@ q.q
		UNION ALL
		SELECT f.id, 'record', ts_rank(search_file(f.text), q.q, 1) FROM files f, q WHERE search_file(f.text) @@ q.q
		UNION ALL
		SELECT w.id, 'raw', ts_rank(search_raw(w.payload), q.q, 1) FROM raw w, q
		WHERE w.source <> 'file' AND search_raw(w.payload) @@ q.q
			AND NOT EXISTS (SELECT FROM provenance p WHERE p.raw_ids @> ARRAY[w.id])
	)
	SELECT h.id, h.kind FROM hit h
	LEFT JOIN records r ON h.kind = 'record' AND r.id = h.id
	LEFT JOIN raw w ON h.kind = 'raw' AND w.id = h.id
	WHERE r.id IS NOT NULL AND ` + searchRecordWhere + `
		OR w.id IS NOT NULL AND cardinality($2::text[]) = 0 AND ($3::uuid IS NULL OR w.space = $3) AND ($4 = '' OR w.source = $4)
			AND ($5::timestamptz IS NULL OR w.fetched_at >= $5) AND ($6::timestamptz IS NULL OR w.fetched_at < $6)
	GROUP BY h.id, h.kind ORDER BY sum(h.rank) DESC, h.id LIMIT $7`

const searchVector = `
	SELECT id, 'record' FROM (
		SELECT e.record_id AS id, e.embedding <=> $1::vector AS d FROM embeddings e JOIN records r ON r.id = e.record_id
		WHERE ` + searchRecordWhere + `
		ORDER BY e.embedding <=> $1::vector LIMIT $7 * 4
	) s GROUP BY id ORDER BY min(d), id LIMIT $7`

func (a *Archive) Search(ctx context.Context, query string, f SearchFilter) ([]*SearchHit, error) {
	if strings.TrimSpace(query) == "" {
		return nil, &ValidationError{Type: "search", Errors: []FieldError{{Path: "q", Message: "empty query"}}}
	}
	spec := a.Spec()
	types := []string{}
	if f.Type != "" {
		if err := coreHasType(spec, f.Type); err != nil {
			return nil, err
		}
		types = spec.Subtypes(f.Type)
	}
	from, err := searchTime("from", f.From, false)
	if err != nil {
		return nil, err
	}
	to, err := searchTime("to", f.To, true)
	if err != nil {
		return nil, err
	}
	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	k := max(searchK, limit)
	var vec string
	if a.embedOff == "" {
		vs, err := a.embedTexts(ctx, []string{query})
		if err != nil {
			fmt.Fprintln(os.Stderr, "archive: search without embeddings:", err)
		} else {
			vec = vs[0]
		}
	}
	return coreDo(a, ctx, func(ctx context.Context) ([]*SearchHit, error) {
		q := a.st.Q(ctx)
		var space any
		if f.Space != "" {
			var id uuid.UUID
			if err := q.QueryRow(ctx, `SELECT id FROM spaces WHERE name = $1`, f.Space).Scan(&id); errors.Is(err, pgx.ErrNoRows) {
				return nil, fmt.Errorf("space %q: %w", f.Space, ErrNotFound)
			} else if err != nil {
				return nil, err
			}
			space = id
		}
		source := ""
		if f.Source != "" {
			src, err := a.sourceFind(ctx, f.Source)
			if err != nil {
				return nil, err
			}
			source = sourceRaw(src)
		}
		args := []any{query, types, space, source, from, to, k}
		lists := [][]searchKey{}
		for _, sql := range []string{searchText, searchVector} {
			if sql == searchVector {
				if vec == "" {
					break
				}
				if a.iterative {
					if _, err := q.Exec(ctx, `SET LOCAL hnsw.iterative_scan = relaxed_order`); err != nil {
						return nil, err
					}
				}
				args[0] = vec
			}
			rows, _ := q.Query(ctx, sql, args...)
			var l []searchKey
			var key searchKey
			if _, err := pgx.ForEachRow(rows, []any{&key.id, &key.kind}, func() error { l = append(l, key); return nil }); err != nil {
				return nil, err
			}
			lists = append(lists, l)
		}
		return a.searchFuse(ctx, query, lists, limit)
	})
}

func (a *Archive) searchFuse(ctx context.Context, query string, lists [][]searchKey, limit int) ([]*SearchHit, error) {
	hits := map[searchKey]*SearchHit{}
	var order []*SearchHit
	for li, l := range lists {
		for i, key := range l {
			h := hits[key]
			if h == nil {
				h = &SearchHit{ID: key.id, Kind: key.kind}
				hits[key] = h
				order = append(order, h)
			}
			h.Score += 1 / float64(searchRRF+i+1)
			if li == 0 {
				h.TextRank = i + 1
			} else {
				h.VectorRank = i + 1
			}
		}
	}
	slices.SortStableFunc(order, func(x, y *SearchHit) int { return cmp.Compare(y.Score, x.Score) })
	order = order[:min(len(order), limit)]
	var recs, raws []uuid.UUID
	for _, h := range order {
		if h.Kind == "raw" {
			raws = append(raws, h.ID)
		} else {
			recs = append(recs, h.ID)
		}
	}
	q := a.st.Q(ctx)
	const headline = `'MaxFragments=1, MaxWords=30, MinWords=10, StartSel=**, StopSel=**'`
	rows, _ := q.Query(ctx, `SELECT r.id, r.type, r.space, r.created_at, r.data,
		coalesce((SELECT ts_headline('simple', left(f.text, 100000), search_query($2), `+headline+`) FROM files f WHERE f.id = r.id AND f.text <> ''), '')
		FROM records r WHERE r.id = ANY($1)`, recs, query)
	var h SearchHit
	scan := func() error {
		if p := hits[searchKey{h.ID, h.Kind}]; p != nil {
			p.Type, p.Source, p.Space, p.Date, p.Data, p.Snippet = h.Type, h.Source, h.Space, h.Date, h.Data, h.Snippet
		}
		return nil
	}
	h.Kind = "record"
	if _, err := pgx.ForEachRow(rows, []any{&h.ID, &h.Type, &h.Space, &h.Date, &h.Data, &h.Snippet}, scan); err != nil {
		return nil, err
	}
	rows, _ = q.Query(ctx, `SELECT id, source, space, fetched_at,
		ts_headline('simple', left(coalesce(payload ->> 'text', ''), 100000), search_query($2), `+headline+`)
		FROM raw WHERE id = ANY($1)`, raws, query)
	h = SearchHit{Kind: "raw"}
	if _, err := pgx.ForEachRow(rows, []any{&h.ID, &h.Source, &h.Space, &h.Date, &h.Snippet}, scan); err != nil {
		return nil, err
	}
	out := []*SearchHit{}
	for _, h := range order {
		if !h.Date.IsZero() {
			out = append(out, h)
		}
	}
	return out, nil
}

func searchTime(name, s string, end bool) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return &t, nil
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return nil, &ValidationError{Type: "search", Errors: []FieldError{{Path: name, Message: "want a date like 2026-10-23 or an RFC 3339 time"}}}
	}
	if end {
		t = t.AddDate(0, 0, 1)
	}
	return &t, nil
}

func (a *Archive) searchIndexes(ctx context.Context, spec *Spec) error {
	want := map[string]string{}
	for _, name := range specKeys(spec.Types) {
		for _, field := range specKeys(spec.Types[name].Fields) {
			if !spec.Types[name].Fields[field].Indexed {
				continue
			}
			sum := sha256.Sum256([]byte(field))
			slug := strings.Trim(searchSlugRe.ReplaceAllString(strings.ToLower(field), "_"), "_")
			idx := ("records_f_" + hex.EncodeToString(sum[:6]) + "_" + slug)[:min(63, 23+len(slug))]
			want[idx] = "CREATE INDEX IF NOT EXISTS " + pgx.Identifier{idx}.Sanitize() + " ON records ((data ->> " + storeLiteral(field) + "))"
		}
	}
	return a.st.System(ctx, func(ctx context.Context) error {
		q := a.st.Q(ctx)
		if _, err := q.Exec(ctx, `SELECT pg_advisory_xact_lock(7273)`); err != nil {
			return err
		}
		rows, _ := q.Query(ctx, `SELECT indexname FROM pg_indexes WHERE schemaname = current_schema() AND tablename = 'records' AND indexname ~ $1`, searchIndexRe)
		have, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			return err
		}
		for _, idx := range have {
			if want[idx] == "" {
				if _, err := q.Exec(ctx, "DROP INDEX "+pgx.Identifier{idx}.Sanitize()); err != nil {
					return err
				}
			}
		}
		for _, idx := range specKeys(want) {
			if !slices.Contains(have, idx) {
				if _, err := q.Exec(ctx, want[idx]); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (a *Archive) embedSetup(ctx context.Context) error {
	cfg := a.intake.cfg
	if cfg.Embed == "" {
		a.embedOff = "ARCHIVE_EMBED_MODEL is not set"
		return nil
	}
	return a.st.System(ctx, func(ctx context.Context) error {
		q := a.st.Q(ctx)
		if _, err := q.Exec(ctx, `SELECT pg_advisory_xact_lock(7274)`); err != nil {
			return err
		}
		var available bool
		if err := q.QueryRow(ctx, `SELECT EXISTS (SELECT FROM pg_available_extensions WHERE name = 'vector')`).Scan(&available); err != nil {
			return err
		}
		if !available {
			a.embedOff = "pgvector is not installed"
			return nil
		}
		if err := pgx.BeginFunc(ctx, q, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS vector`)
			return err
		}); err != nil {
			a.embedOff = "pgvector: " + err.Error()
			return nil
		}
		var ver string
		var dim *int
		if err := q.QueryRow(ctx, `SELECT extversion, (SELECT atttypmod FROM pg_attribute WHERE attrelid = to_regclass('embeddings') AND attname = 'embedding')
			FROM pg_extension WHERE extname = 'vector'`).Scan(&ver, &dim); err != nil {
			return err
		}
		var major, minor int
		fmt.Sscanf(ver, "%d.%d", &major, &minor)
		a.iterative = major > 0 || minor >= 8
		if dim != nil && *dim != cfg.EmbedDim {
			if _, err := q.Exec(ctx, `DROP TABLE embeddings`); err != nil {
				return err
			}
		}
		_, err := q.Exec(ctx, strings.ReplaceAll(`
			CREATE TABLE IF NOT EXISTS embeddings (
				record_id uuid NOT NULL REFERENCES records ON DELETE CASCADE,
				chunk     int NOT NULL,
				model     text NOT NULL,
				embedding vector(`+strconv.Itoa(cfg.EmbedDim)+`) NOT NULL,
				space     uuid NOT NULL REFERENCES spaces,
				PRIMARY KEY (record_id, chunk)
			);
			CREATE INDEX IF NOT EXISTS embeddings_hnsw_idx ON embeddings USING hnsw (embedding vector_cosine_ops);
			CREATE INDEX IF NOT EXISTS embeddings_space_idx ON embeddings (space);
			ALTER TABLE embeddings ENABLE ROW LEVEL SECURITY, FORCE ROW LEVEL SECURITY;
			DROP POLICY IF EXISTS viewer ON embeddings;
			CREATE POLICY viewer ON embeddings FOR SELECT USING (space = ANY ((SELECT my_spaces())::uuid[]));
			GRANT SELECT ON embeddings TO {p}_user;
			DELETE FROM embeddings WHERE model <> `+storeLiteral(cfg.Embed)+`;
			INSERT INTO embed_queue (record_id) SELECT id FROM records r WHERE NOT EXISTS (SELECT FROM embeddings e WHERE e.record_id = r.id)
				ON CONFLICT DO NOTHING`, "{p}", a.prefix))
		return err
	})
}

type embedJob struct {
	id, space   uuid.UUID
	at          time.Time
	title, body string
	chunks      []string
}

func (a *Archive) embedLoop(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		for a.embedNext(ctx) {
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

func (a *Archive) embedNext(ctx context.Context) bool {
	var jobs []*embedJob
	err := a.st.System(ctx, func(ctx context.Context) error {
		rows, _ := a.st.Q(ctx).Query(ctx, `
			SELECT q.record_id, r.space, q.queued_at, coalesce(f.name, r.type),
				CASE WHEN f.id IS NULL THEN search_text(r.data) ELSE left(f.text, $1) END
			FROM embed_queue q JOIN records r ON r.id = q.record_id LEFT JOIN files f ON f.id = r.id
			WHERE q.retry_at <= now() AND coalesce(f.status, '') NOT IN ('pending', 'processing')
			ORDER BY q.queued_at LIMIT $2`, embedChunk*embedChunks*2, embedJobs)
		var j embedJob
		_, err := pgx.ForEachRow(rows, []any{&j.id, &j.space, &j.at, &j.title, &j.body}, func() error {
			c := j
			jobs = append(jobs, &c)
			return nil
		})
		return err
	})
	if err != nil || len(jobs) == 0 {
		if err != nil && ctx.Err() == nil {
			fmt.Fprintln(os.Stderr, "archive: embeddings:", err)
		}
		return false
	}
	var inputs []string
	ids := make([]uuid.UUID, len(jobs))
	for i, j := range jobs {
		ids[i] = j.id
		j.chunks = embedSplit(j.title, j.body)
		inputs = append(inputs, j.chunks...)
	}
	vecs, err := a.embedTexts(ctx, inputs)
	if err == nil {
		err = a.st.System(ctx, func(ctx context.Context) error {
			q := a.st.Q(ctx)
			for _, j := range jobs {
				if _, err := q.Exec(ctx, `DELETE FROM embeddings WHERE record_id = $1`, j.id); err != nil {
					return err
				}
				for i := range j.chunks {
					if _, err := q.Exec(ctx, `INSERT INTO embeddings (record_id, chunk, model, embedding, space)
						SELECT $1, $2, $3, $4::vector, $5 WHERE EXISTS (SELECT FROM records WHERE id = $1)`,
						j.id, i, a.intake.cfg.Embed, vecs[0], j.space); err != nil {
						return err
					}
					vecs = vecs[1:]
				}
				if _, err := q.Exec(ctx, `DELETE FROM embed_queue WHERE record_id = $1 AND queued_at = $2`, j.id, j.at); err != nil {
					return err
				}
			}
			return nil
		})
	}
	if err != nil {
		if ctx.Err() == nil {
			fmt.Fprintln(os.Stderr, "archive: embeddings:", err)
		}
		a.st.System(context.WithoutCancel(ctx), func(ctx context.Context) error {
			_, err := a.st.Q(ctx).Exec(ctx, `UPDATE embed_queue SET tries = tries + 1, error = $2,
				retry_at = now() + least(3600, 5 * 2 ^ least(tries, 10)) * interval '1 second' WHERE record_id = ANY($1)`, ids, err.Error())
			return err
		})
		return false
	}
	return true
}

func embedSplit(title, body string) []string {
	rs := []rune(strings.Join(strings.Fields(body), " "))
	if len(rs) == 0 {
		return []string{title}
	}
	var out []string
	for len(rs) > 0 && len(out) < embedChunks {
		n := min(len(rs), embedChunk)
		for i := n - 1; n < len(rs) && i > embedChunk/2; i-- {
			if rs[i] == ' ' {
				n = i
				break
			}
		}
		out = append(out, title+"\n"+string(rs[:n]))
		rs = []rune(strings.TrimLeft(string(rs[n:]), " "))
	}
	return out
}

func (a *Archive) embedTexts(ctx context.Context, inputs []string) ([]string, error) {
	cfg := a.intake.cfg
	var out []string
	for len(inputs) > 0 {
		batch := inputs[:min(len(inputs), embedBatch)]
		inputs = inputs[len(batch):]
		body, err := json.Marshal(map[string]any{"model": cfg.Embed, "input": batch})
		if err != nil {
			return nil, err
		}
		var res struct {
			Data []struct {
				Index     int       `json:"index"`
				Embedding []float64 `json:"embedding"`
			} `json:"data"`
		}
		if err := a.mediaPost(ctx, "/embeddings", "application/json", body, &res); err != nil {
			return nil, fmt.Errorf("embeddings: %w", err)
		}
		vecs := make([]string, len(batch))
		for _, d := range res.Data {
			if d.Index < 0 || d.Index >= len(batch) {
				return nil, fmt.Errorf("embeddings: index %d out of range", d.Index)
			}
			if len(d.Embedding) != cfg.EmbedDim {
				return nil, fmt.Errorf("embeddings: %s returns %d dimensions, ARCHIVE_EMBED_DIM is %d", cfg.Embed, len(d.Embedding), cfg.EmbedDim)
			}
			parts := make([]string, len(d.Embedding))
			for i, v := range d.Embedding {
				parts[i] = strconv.FormatFloat(v, 'g', -1, 32)
			}
			vecs[d.Index] = "[" + strings.Join(parts, ",") + "]"
		}
		if slices.Contains(vecs, "") {
			return nil, fmt.Errorf("embeddings: got %d of %d vectors", len(res.Data), len(batch))
		}
		out = append(out, vecs...)
	}
	return out, nil
}
