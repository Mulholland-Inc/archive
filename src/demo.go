package archive

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

//go:embed demo.json
var demoJSON []byte

type demoSet struct {
	Types   map[string]json.RawMessage `json:"types"`
	Spaces  []string                   `json:"spaces"`
	Records []struct {
		Ref   string         `json:"ref"`
		Type  string         `json:"type"`
		Space string         `json:"space"`
		Data  map[string]any `json:"data"`
	} `json:"records"`
	Proposals []struct {
		Type   string `json:"type"`
		Space  string `json:"space"`
		Label  string `json:"label"`
		Source string `json:"source"`
		Fields []struct {
			Path       string  `json:"path"`
			Value      any     `json:"value"`
			Confidence float64 `json:"confidence"`
		} `json:"fields"`
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
	for _, r := range d.Records {
		data, _ := demoResolve(r.Data, now, ids).(map[string]any)
		rec, err := a.Create(ctx, r.Space, r.Type, data)
		if err != nil {
			return nil, fmt.Errorf("demo %s %s: %w", r.Type, r.Ref, err)
		}
		ids[r.Ref] = rec.ID.String()
		counts[r.Type]++
	}

	spec := a.Spec()
	for _, p := range d.Proposals {
		fields, err := spec.Fields(p.Type)
		if err != nil {
			return nil, err
		}
		res := &IntakeResult{Type: p.Type}
		accepted, complete := map[string]any{}, true
		for _, name := range specKeys(fields) {
			fr := FieldResult{Path: name, Route: "drop"}
			for _, f := range p.Fields {
				if f.Path != name {
					continue
				}
				raw := demoResolve(f.Value, now, ids)
				fr.Original, fr.Confidence, fr.RawScore = fmt.Sprint(raw), f.Confidence, f.Confidence
				if v, err := spec.NormalizeField(fields[name], raw, a.hints); err == nil {
					fr.Value, fr.Route = v, intakeRoute(f.Confidence, a.intake.cfg.Accept, a.intake.cfg.Review)
				}
			}
			if fr.Route == "accept" {
				accepted[name] = fr.Value
			} else if fields[name].Required {
				complete = false
			}
			res.Fields = append(res.Fields, fr)
		}
		payload, _ := json.Marshal(map[string]any{"name": p.Label})
		raw, err := coreDo(a, ctx, func(ctx context.Context) (*Raw, error) {
			sid, err := a.coreSpace(ctx, p.Space)
			if err != nil {
				return nil, err
			}
			raw := &Raw{Source: p.Source, ExternalID: "demo/" + p.Label, Payload: payload, Space: sid}
			if _, err := a.st.PutRaw(ctx, raw); err != nil {
				return nil, err
			}
			if !complete || len(accepted) == 0 {
				return raw, nil
			}
			var prov []*Provenance
			for _, f := range res.Fields {
				if f.Route == "accept" {
					prov = append(prov, &Provenance{FieldPath: f.Path, RawIDs: []uuid.UUID{raw.ID}, Method: "intake:demo", Confidence: f.Confidence, Original: f.Original})
				}
			}
			rec := &Record{Type: p.Type, SpecVersion: spec.Version, Space: sid}
			if err := a.coreSave(ctx, rec, accepted, prov); err != nil {
				return nil, err
			}
			res.RecordID = rec.ID
			return raw, nil
		})
		if err != nil {
			return nil, fmt.Errorf("demo review %s: %w", p.Label, err)
		}
		res.RawID = raw.ID
		if err := a.propose(ctx, raw, res); err != nil {
			return nil, err
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
