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

//go:embed demo.json
var demoJSON []byte

type demoSet struct {
	Types    map[string]json.RawMessage `json:"types"`
	Spaces   []string                   `json:"spaces"`
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
	for _, name := range specKeys(d.Types) {
		if a.Spec().Types[name] != nil {
			return nil, fmt.Errorf("demo data needs an archive without a %s type: %w", name, ErrConflict)
		}
	}
	b, err := json.Marshal(map[string]any{"types": d.Types})
	if err != nil {
		return nil, err
	}
	if _, err := a.ApplySpec(ctx, b, false); err != nil {
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
		rec, err := a.Create(ctx, r.Space, r.Type, data)
		if err != nil {
			return nil, fmt.Errorf("demo %s %s: %w", r.Type, r.Ref, err)
		}
		ids[r.Ref] = rec.ID.String()
		counts[r.Type]++
	}

	spec := a.Spec()
	for i, p := range d.Proposals {
		fields, err := spec.Fields(p.Type)
		if err != nil {
			return nil, err
		}
		data := map[string]any{}
		for k, v := range p.Data {
			if f := fields[k]; f != nil {
				if nv, err := spec.NormalizeField(f, demoResolve(v, now, ids), a.hints); err == nil {
					data[k] = nv
				}
			}
		}
		_, err = coreDo(a, ctx, func(ctx context.Context) (*Entity, error) {
			sid, err := a.coreSpace(ctx, p.Space)
			if err != nil {
				return nil, err
			}
			e := &Entity{Type: p.Type, Space: sid, Proposed: true}
			return e, a.coreSave(ctx, e, data)
		})
		if err != nil {
			return nil, fmt.Errorf("demo review %s %d: %w", p.Type, i, err)
		}
		counts["review"]++
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
