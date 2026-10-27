package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

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

const searchFile = `SELECT 'file' AS kind, 'file' AS type, f.id, f.space, f.created_at, jsonb_build_object('id', f.id, 'name', f.name, 'mime', f.mime,
		'size', f.size, 'sha256', f.hash, 'space', f.space, 'created_at', f.created_at) AS data, ts_rank(f.search, q.q, 1) AS rank,
		CASE WHEN f.text <> '' THEN ts_headline('simple', left(f.text, 100000), q.q, 'MaxFragments=1, MaxWords=30, MinWords=10, StartSel=**, StopSel=**') ELSE '' END AS snippet
	FROM archive.file f, q WHERE f.search @@ q.q`

func (a *Archive) Search(ctx context.Context, query string, f SearchFilter) ([]*SearchHit, error) {
	if strings.TrimSpace(query) == "" {
		return nil, &ValidationError{Type: "search", Errors: []FieldError{{Path: "q", Message: "empty query"}}}
	}
	cat := a.Catalog()
	var parts []string
	for _, name := range cat.names() {
		if f.Type == "" || f.Type == name {
			tb := cat.Tables[name]
			parts = append(parts, `SELECT 'entity' AS kind, `+pgLiteral(name)+` AS type, t.id, t.space, t.created_at, `+tb.rowSQL("t")+` AS data, ts_rank(t.search, q.q, 1) AS rank, '' AS snippet
				FROM `+tb.ident()+` t, q WHERE t.search @@ q.q`)
		}
	}
	if f.Type == "" || f.Type == "file" {
		parts = append(parts, searchFile)
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("type %q: %w", f.Type, ErrNotFound)
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
	sql := `WITH q AS (SELECT search_query($1) AS q)
		SELECT h.kind, h.type, h.id, h.space, h.created_at, h.data, h.rank, h.snippet FROM (` + strings.Join(parts, "\nUNION ALL\n") + `) h
		WHERE ($2::uuid IS NULL OR h.space = $2) AND ($3::timestamptz IS NULL OR h.created_at >= $3) AND ($4::timestamptz IS NULL OR h.created_at < $4)
		ORDER BY h.rank DESC, h.id LIMIT $5`
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
		rows, _ := q.Query(ctx, sql, query, space, from, to, limit)
		out := []*SearchHit{}
		var h SearchHit
		_, err := pgx.ForEachRow(rows, []any{&h.Kind, &h.Type, &h.ID, &h.Space, &h.Date, &h.Data, &h.Score, &h.Snippet}, func() error {
			c := h
			c.Data = append(json.RawMessage(nil), h.Data...)
			out = append(out, &c)
			return nil
		})
		return out, err
	})
}

func pgLiteral(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

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
