package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ProposedField struct {
	Path       string  `json:"path"`
	Value      any     `json:"value"`
	Original   string  `json:"original,omitempty"`
	Confidence float64 `json:"confidence"`
	Route      string  `json:"route"`
	Page       int     `json:"page,omitempty"`
	Secret     bool    `json:"secret,omitempty"`
}

type Proposal struct {
	ID         uuid.UUID       `json:"id"`
	RawID      *uuid.UUID      `json:"raw_id,omitempty"`
	Type       string          `json:"type"`
	Source     string          `json:"source"`
	Label      string          `json:"label"`
	FileID     *uuid.UUID      `json:"file_id,omitempty"`
	Method     string          `json:"method"`
	Fields     []ProposedField `json:"fields"`
	Confidence float64         `json:"confidence"`
	Status     string          `json:"status"`
	RecordID   *uuid.UUID      `json:"record_id,omitempty"`
	DecidedBy  string          `json:"decided_by,omitempty"`
	DecidedAt  *time.Time      `json:"decided_at,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
	Space      uuid.UUID       `json:"space"`
}

type ReviewFilter struct {
	Status string
	Type   string
	Space  string
	Limit  int
	After  uuid.UUID
}

type ReviewOutcome struct {
	Done   []*Proposal   `json:"done"`
	Failed []ReviewError `json:"failed"`
}

type ReviewError struct {
	ID    uuid.UUID `json:"id"`
	Error string    `json:"error"`
}

const proposalCols = `id, raw_id, type, source, label, file_id, method, fields, confidence, status, record_id, decided_by, decided_at, created_at, space`

var reviewStatuses = []string{"review", "accepted", "approved", "rejected"}

func (a *Archive) ingest(ctx context.Context, raw *Raw, typ string) (*IntakeResult, error) {
	res, err := a.intake.Ingest(ctx, raw, typ)
	if err != nil {
		return nil, err
	}
	return res, a.propose(ctx, raw, res)
}

func (a *Archive) propose(ctx context.Context, raw *Raw, res *IntakeResult) error {
	var payload struct {
		Name    string            `json:"name"`
		Text    string            `json:"text"`
		File    *uuid.UUID        `json:"file"`
		Pages   []string          `json:"pages"`
		Headers map[string]string `json:"headers"`
	}
	json.Unmarshal(raw.Payload, &payload)
	spec := a.Spec()
	p := &Proposal{RawID: &raw.ID, Type: res.Type, Source: raw.Source, FileID: payload.File, Method: "intake:" + a.intake.cfg.Model, Space: raw.Space, Confidence: 1}
	p.Label = reviewLabel(payload.Name, payload.Headers["Subject"], payload.Text)
	review, usable := false, false
	for _, f := range res.Fields {
		pf := ProposedField{Path: f.Path, Value: f.Value, Original: f.Original, Confidence: f.Confidence, Route: f.Route, Page: intakePage(payload.Pages, f.Original, f.Value)}
		if spec.secretField(res.Type, f.Path) {
			pf.Secret, pf.Original = true, ""
			if f.Route != "accept" || res.RecordID == uuid.Nil {
				pf.Value = nil
			}
		}
		if pf.Value != nil {
			usable = true
			p.Confidence = min(p.Confidence, f.Confidence)
		}
		if f.Route == "review" || f.Route == "accept" && res.RecordID == uuid.Nil {
			review = true
		}
		p.Fields = append(p.Fields, pf)
	}
	switch {
	case res.RecordID != uuid.Nil && !review:
		p.Status = "accepted"
	case usable:
		p.Status = "review"
	default:
		p.Status, p.DecidedBy, p.Confidence = "rejected", "intake", 0
	}
	if res.RecordID != uuid.Nil {
		p.RecordID = &res.RecordID
	}
	return a.as(ctx, func(ctx context.Context) error {
		_, err := storeOne[Proposal](ctx, a.st, `INSERT INTO proposals (raw_id, type, source, label, file_id, method, fields, confidence, status, record_id, decided_by, space)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) RETURNING `+proposalCols,
			p.RawID, p.Type, p.Source, p.Label, p.FileID, p.Method, p.Fields, p.Confidence, p.Status, p.RecordID, p.DecidedBy, p.Space)
		return err
	})
}

func reviewLabel(name, subject, text string) string {
	for _, s := range []string{name, subject, strings.TrimSpace(text)} {
		if s = strings.TrimSpace(strings.SplitN(s, "\n", 2)[0]); s != "" {
			r := []rune(s)
			if len(r) > 80 {
				return string(r[:79]) + "…"
			}
			return s
		}
	}
	return ""
}

func (a *Archive) Proposals(ctx context.Context, f ReviewFilter) ([]*Proposal, error) {
	if f.Status != "" && !slices.Contains(reviewStatuses, f.Status) {
		return nil, &ValidationError{Type: "review", Errors: []FieldError{{Path: "status", Message: "want " + strings.Join(reviewStatuses, ", ")}}}
	}
	if f.Limit <= 0 || f.Limit > 1000 {
		f.Limit = 100
	}
	return coreDo(a, ctx, func(ctx context.Context) ([]*Proposal, error) {
		var space *uuid.UUID
		if f.Space != "" {
			id, err := a.coreSpace(ctx, f.Space)
			if err != nil {
				return nil, err
			}
			space = &id
		}
		out, err := storeAll[Proposal](ctx, a.st, `SELECT `+proposalCols+` FROM proposals
			WHERE ($1 = '' OR status = $1) AND ($2 = '' OR type = $2) AND ($3::uuid IS NULL OR space = $3) AND ($4::uuid IS NULL OR id > $4)
			ORDER BY id LIMIT $5`, f.Status, f.Type, space, reviewAfter(f.After), f.Limit)
		if out == nil {
			out = []*Proposal{}
		}
		return out, err
	})
}

func reviewAfter(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}

func (a *Archive) ProposalCounts(ctx context.Context, space string) (map[string]int, error) {
	return coreDo(a, ctx, func(ctx context.Context) (map[string]int, error) {
		var sid *uuid.UUID
		if space != "" {
			id, err := a.coreSpace(ctx, space)
			if err != nil {
				return nil, err
			}
			sid = &id
		}
		out := map[string]int{}
		for _, s := range reviewStatuses {
			out[s] = 0
		}
		rows, err := a.st.Q(ctx).Query(ctx, `SELECT status, count(*) FROM proposals WHERE $1::uuid IS NULL OR space = $1 GROUP BY status`, sid)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var s string
			var n int
			if err := rows.Scan(&s, &n); err != nil {
				return nil, err
			}
			out[s] = n
		}
		return out, rows.Err()
	})
}

func (a *Archive) GetProposal(ctx context.Context, id uuid.UUID) (*Proposal, error) {
	return coreDo(a, ctx, func(ctx context.Context) (*Proposal, error) {
		p, err := storeOne[Proposal](ctx, a.st, `SELECT `+proposalCols+` FROM proposals WHERE id = $1`, id)
		if errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("proposal %s: %w", id, ErrNotFound)
		}
		return p, err
	})
}

func (a *Archive) Approve(ctx context.Context, ids []uuid.UUID, edits map[uuid.UUID]map[string]any) (*ReviewOutcome, error) {
	return a.decide(ctx, ids, func(ctx context.Context, p *Proposal) error {
		r, err := a.approve(ctx, p, edits[p.ID])
		if err != nil {
			return err
		}
		p.RecordID, p.Status = &r.ID, "approved"
		return nil
	})
}

func (a *Archive) Reject(ctx context.Context, ids []uuid.UUID) (*ReviewOutcome, error) {
	return a.decide(ctx, ids, func(ctx context.Context, p *Proposal) error {
		p.Status = "rejected"
		return nil
	})
}

func (a *Archive) decide(ctx context.Context, ids []uuid.UUID, fn func(context.Context, *Proposal) error) (*ReviewOutcome, error) {
	if ActorFrom(ctx) == nil {
		return nil, fmt.Errorf("not signed in: %w", ErrUnauthorized)
	}
	out := &ReviewOutcome{Done: []*Proposal{}, Failed: []ReviewError{}}
	for _, id := range ids {
		p, err := coreDo(a, ctx, func(ctx context.Context) (*Proposal, error) {
			p, err := a.GetProposal(ctx, id)
			if err != nil {
				return nil, err
			}
			if p.Status == "approved" || p.Status == "rejected" {
				return nil, fmt.Errorf("already %s: %w", p.Status, ErrConflict)
			}
			if err := fn(ctx, p); err != nil {
				return nil, err
			}
			p, err = storeOne[Proposal](ctx, a.st, `UPDATE proposals SET status = $2, record_id = $3, decided_by = $4, decided_at = now()
				WHERE id = $1 RETURNING `+proposalCols, p.ID, p.Status, p.RecordID, ActorFrom(ctx).Label)
			if errors.Is(err, ErrNotFound) {
				return nil, fmt.Errorf("you cannot review in this space: %w", ErrForbidden)
			}
			return p, err
		})
		if err != nil {
			out.Failed = append(out.Failed, ReviewError{ID: id, Error: err.Error()})
			continue
		}
		out.Done = append(out.Done, p)
	}
	return out, nil
}

func (a *Archive) approve(ctx context.Context, p *Proposal, edits map[string]any) (*Record, error) {
	spec := a.Spec()
	if err := coreHasType(spec, p.Type); err != nil {
		return nil, err
	}
	var r *Record
	data := map[string]any{}
	if p.RecordID != nil {
		var err error
		if r, err = a.st.GetRecord(ctx, *p.RecordID); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(r.Data, &data); err != nil {
			return nil, err
		}
	}
	reviewer := "review:" + ActorFrom(ctx).Label
	var prov []*Provenance
	for _, f := range p.Fields {
		if _, edited := edits[f.Path]; edited || f.Value == nil || f.Route == "drop" || r != nil && f.Route == "accept" {
			continue
		}
		data[f.Path] = f.Value
		method, conf := reviewer, 1.0
		if f.Route == "accept" {
			method, conf = p.Method, f.Confidence
		}
		prov = append(prov, &Provenance{FieldPath: f.Path, RawIDs: reviewRaw(p), FileID: p.FileID, Page: f.Page, Method: method, Confidence: conf, Original: f.Original})
	}
	for k, v := range edits {
		if v == nil {
			delete(data, k)
			continue
		}
		data[k] = v
		prov = append(prov, &Provenance{FieldPath: k, Method: "manual", Confidence: 1})
	}
	out, _, err := a.coreCheck(ctx, spec, p.Type, data)
	if err != nil {
		return nil, err
	}
	if r == nil {
		r = &Record{Type: p.Type, SpecVersion: spec.Version, Space: p.Space}
		return r, a.coreSave(ctx, r, out, prov)
	}
	if r, err = a.coreUpdate(ctx, r, out, nil); err != nil {
		return nil, err
	}
	for _, pv := range prov {
		pv.RecordID = r.ID
	}
	if len(prov) == 0 {
		return r, nil
	}
	return r, a.st.AddProvenance(ctx, prov)
}

func reviewRaw(p *Proposal) []uuid.UUID {
	if p.RawID == nil {
		return nil
	}
	return []uuid.UUID{*p.RawID}
}

func cliReview(ctx context.Context, a *Archive, pos []string, f ReviewFilter, after string, out func(any, error) error) error {
	var ids []uuid.UUID
	for _, s := range pos[1:] {
		id, err := uuid.Parse(s)
		if err != nil {
			return fmt.Errorf("id %q: %w", s, err)
		}
		ids = append(ids, id)
	}
	switch pos[0] {
	case "list":
		if after != "" {
			id, err := uuid.Parse(after)
			if err != nil {
				return fmt.Errorf("-after: %w", err)
			}
			f.After = id
		}
		return out(a.Proposals(ctx, f))
	case "counts":
		return out(a.ProposalCounts(ctx, f.Space))
	case "approve", "reject":
		if len(ids) == 0 {
			return cliErrUsage
		}
		decide := a.Reject
		if pos[0] == "approve" {
			decide = func(ctx context.Context, ids []uuid.UUID) (*ReviewOutcome, error) { return a.Approve(ctx, ids, nil) }
		}
		res, err := decide(ctx, ids)
		if err == nil && len(res.Failed) > 0 {
			err = fmt.Errorf("%d of %d failed: %s", len(res.Failed), len(ids), res.Failed[0].Error)
		}
		if err != nil {
			return err
		}
		return out(res, nil)
	}
	return cliErrUsage
}
