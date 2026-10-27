package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const searchIndexRe = `^entity_f_[0-9a-f]{12}_`

var searchSlugRe = regexp.MustCompile(`[^a-z0-9]+`)

type SearchFilter struct {
	Type  string
	Space string
	From  string
	To    string
	Limit int
}

type SearchHit struct {
	ID      uuid.UUID       `json:"id"`
	Kind    string          `json:"kind"`
	Type    string          `json:"type"`
	Space   uuid.UUID       `json:"space"`
	Date    time.Time       `json:"date"`
	Data    json.RawMessage `json:"data,omitempty"`
	Snippet string          `json:"snippet,omitempty"`
	Score   float64         `json:"score"`
}

const searchSQL = `
	WITH q AS (SELECT search_query($1) AS q),
	hit AS (
		SELECT e.id, ts_rank(search_entity(e.type, e.data), q.q, 1) AS rank
		FROM entity e, q WHERE search_entity(e.type, e.data) @@ q.q
		UNION ALL
		SELECT f.id, ts_rank(search_file(f.text), q.q, 1) FROM file f, q WHERE search_file(f.text) @@ q.q
	)
	SELECT e.id, e.type, e.space, e.created_at, e.data, sum(h.rank),
		coalesce((SELECT ts_headline('simple', left(f.text, 100000), search_query($1),
			'MaxFragments=1, MaxWords=30, MinWords=10, StartSel=**, StopSel=**') FROM file f WHERE f.id = e.id AND f.text <> ''), '')
	FROM hit h JOIN entity e ON e.id = h.id
	WHERE NOT e.proposed AND (cardinality($2::text[]) = 0 OR e.type = ANY($2))
		AND ($3::uuid IS NULL OR e.space = $3)
		AND ($4::timestamptz IS NULL OR e.created_at >= $4) AND ($5::timestamptz IS NULL OR e.created_at < $5)
	GROUP BY e.id ORDER BY sum(h.rank) DESC, e.id LIMIT $6`

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
	return coreDo(a, ctx, func(ctx context.Context) ([]*SearchHit, error) {
		q := a.st.Q(ctx)
		var space any
		if f.Space != "" {
			var id uuid.UUID
			if err := q.QueryRow(ctx, `SELECT id FROM space WHERE name = $1`, f.Space).Scan(&id); errors.Is(err, pgx.ErrNoRows) {
				return nil, fmt.Errorf("space %q: %w", f.Space, ErrNotFound)
			} else if err != nil {
				return nil, err
			}
			space = id
		}
		rows, _ := q.Query(ctx, searchSQL, query, types, space, from, to, limit)
		out := []*SearchHit{}
		var h SearchHit
		_, err := pgx.ForEachRow(rows, []any{&h.ID, &h.Type, &h.Space, &h.Date, &h.Data, &h.Score, &h.Snippet}, func() error {
			c := h
			c.Kind = "entity"
			out = append(out, &c)
			return nil
		})
		return out, err
	})
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
			idx := ("entity_f_" + hex.EncodeToString(sum[:6]) + "_" + slug)[:min(63, 22+len(slug))]
			want[idx] = "CREATE INDEX IF NOT EXISTS " + pgx.Identifier{idx}.Sanitize() + " ON entity ((data ->> " + storeLiteral(field) + "))"
		}
	}
	return a.st.System(ctx, func(ctx context.Context) error {
		q := a.st.Q(ctx)
		if _, err := q.Exec(ctx, `SELECT pg_advisory_xact_lock(7273)`); err != nil {
			return err
		}
		rows, _ := q.Query(ctx, `SELECT indexname FROM pg_indexes WHERE schemaname = current_schema() AND tablename = 'entity' AND indexname ~ $1`, searchIndexRe)
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
