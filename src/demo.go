package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

var (
	//go:embed demo.json
	demoJSON []byte
	//go:embed demo.sql
	demoSQL string
)

type demoSet struct {
	Spaces   []string `json:"spaces"`
	Entities []struct {
		Ref   string         `json:"ref"`
		Type  string         `json:"type"`
		Space string         `json:"space"`
		Data  map[string]any `json:"data"`
	} `json:"entities"`
	Proposals []struct {
		Type  string         `json:"type"`
		Space string         `json:"space"`
		Data  map[string]any `json:"data"`
	} `json:"proposals"`
}

func (a *Archive) LoadDemo(ctx context.Context) (map[string]int, error) {
	if _, err := authUser(ctx); err != nil {
		return nil, err
	}
	var d demoSet
	if err := json.Unmarshal(demoJSON, &d); err != nil {
		return nil, err
	}
	for _, e := range d.Entities {
		if a.Catalog().Tables[e.Type] != nil {
			return nil, fmt.Errorf("demo data needs an archive without a %s table: %w", e.Type, ErrConflict)
		}
	}
	files, err := a.st.SchemaFiles()
	if err != nil {
		return nil, err
	}
	if _, ok := files["demo.sql"]; ok {
		return nil, fmt.Errorf("demo data needs an archive without a demo.sql schema file: %w", ErrConflict)
	}
	files["demo.sql"] = demoSQL
	if _, err := a.ApplySchema(ctx, files, nil, false, false); err != nil {
		return nil, err
	}
	have, err := a.Spaces(ctx)
	if err != nil {
		return nil, err
	}
	for _, s := range d.Spaces {
		if !slices.ContainsFunc(have, func(h SpaceInfo) bool { return h.Name == s }) {
			if _, err := a.AddSpace(ctx, s); err != nil {
				return nil, err
			}
		}
	}

	now := time.Now()
	ids := map[string]string{}
	counts := map[string]int{}
	for _, r := range d.Entities {
		data, _ := demoResolve(r.Data, now, ids).(map[string]any)
		row, err := a.Create(ctx, r.Space, r.Type, data)
		if err != nil {
			return nil, fmt.Errorf("demo %s %s: %w", r.Type, r.Ref, err)
		}
		ids[r.Ref] = coreRowID(row).String()
		counts[r.Type]++
	}
	for i, p := range d.Proposals {
		data, _ := demoResolve(p.Data, now, ids).(map[string]any)
		if _, err := a.Create(ctx, p.Space, p.Type, data); err != nil {
			return nil, fmt.Errorf("demo %s %d: %w", p.Type, i, err)
		}
		counts[p.Type]++
	}
	return counts, nil
}

func demoResolve(v any, now time.Time, ids map[string]string) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, e := range x {
			out[k] = demoResolve(e, now, ids)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = demoResolve(e, now, ids)
		}
		return out
	case string:
		if ref, ok := strings.CutPrefix(x, "@ref:"); ok {
			return ids[ref]
		}
		if days, ok := strings.CutPrefix(x, "@day:"); ok {
			n, _ := strconv.Atoi(days)
			return now.AddDate(0, 0, n).Format(time.DateOnly)
		}
		if at, ok := strings.CutPrefix(x, "@at:"); ok {
			days, clock, _ := strings.Cut(at, "T")
			n, _ := strconv.Atoi(days)
			t, _ := time.Parse("15:04", clock)
			d := now.AddDate(0, 0, n)
			return time.Date(d.Year(), d.Month(), d.Day(), t.Hour(), t.Minute(), 0, 0, now.Location()).Format(time.RFC3339)
		}
	}
	return v
}
