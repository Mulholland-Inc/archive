package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

type ReviewFilter struct {
	Type  string
	Space string
	Limit int
	After uuid.UUID
}

type ReviewOutcome struct {
	Done   []*Entity     `json:"done"`
	Failed []ReviewError `json:"failed"`
}

type ReviewError struct {
	ID    uuid.UUID `json:"id"`
	Error string    `json:"error"`
}

func (a *Archive) Proposed(ctx context.Context, f ReviewFilter) ([]*Entity, error) {
	spec := a.Spec()
	var types []string
	if f.Type != "" {
		if err := coreHasType(spec, f.Type); err != nil {
			return nil, err
		}
		types = spec.Subtypes(f.Type)
	}
	if f.Limit <= 0 || f.Limit > 1000 {
		f.Limit = 100
	}
	return coreDo(a, ctx, func(ctx context.Context) ([]*Entity, error) {
		lf := Filter{Types: types, Proposed: true, Limit: f.Limit, After: f.After}
		if f.Space != "" {
			id, err := a.coreSpace(ctx, f.Space)
			if err != nil {
				return nil, err
			}
			lf.Space = id
		}
		return a.st.ListEntities(ctx, lf)
	})
}

func (a *Archive) ProposedCount(ctx context.Context, space string) (map[string]int, error) {
	return coreDo(a, ctx, func(ctx context.Context) (map[string]int, error) {
		var sid *uuid.UUID
		if space != "" {
			id, err := a.coreSpace(ctx, space)
			if err != nil {
				return nil, err
			}
			sid = &id
		}
		var n int
		err := a.st.Q(ctx).QueryRow(ctx, `SELECT count(*) FROM entity WHERE proposed AND ($1::uuid IS NULL OR space = $1)`, sid).Scan(&n)
		return map[string]int{"review": n}, err
	})
}

func (a *Archive) Approve(ctx context.Context, ids []uuid.UUID, edits map[uuid.UUID]map[string]any) (*ReviewOutcome, error) {
	return a.decide(ctx, ids, func(ctx context.Context, e *Entity) (*Entity, error) {
		e.Proposed = false
		return a.corePatch(ctx, e, edits[e.ID])
	})
}

func (a *Archive) Reject(ctx context.Context, ids []uuid.UUID) (*ReviewOutcome, error) {
	return a.decide(ctx, ids, func(ctx context.Context, e *Entity) (*Entity, error) {
		if err := a.st.DeleteEntity(ctx, e.ID); errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("you cannot review in this space: %w", ErrForbidden)
		} else if err != nil {
			return nil, err
		}
		return e, nil
	})
}

func (a *Archive) decide(ctx context.Context, ids []uuid.UUID, fn func(context.Context, *Entity) (*Entity, error)) (*ReviewOutcome, error) {
	if ActorFrom(ctx) == nil {
		return nil, fmt.Errorf("not signed in: %w", ErrUnauthorized)
	}
	out := &ReviewOutcome{Done: []*Entity{}, Failed: []ReviewError{}}
	for _, id := range ids {
		e, err := coreDo(a, ctx, func(ctx context.Context) (*Entity, error) {
			e, err := a.st.GetEntity(ctx, id)
			if errors.Is(err, ErrNotFound) {
				return nil, fmt.Errorf("entity %s: %w", id, ErrNotFound)
			}
			if err != nil {
				return nil, err
			}
			if !e.Proposed {
				return nil, fmt.Errorf("entity %s is not waiting for review: %w", id, ErrConflict)
			}
			return fn(ctx, e)
		})
		if err != nil {
			out.Failed = append(out.Failed, ReviewError{ID: id, Error: err.Error()})
			continue
		}
		out.Done = append(out.Done, e)
	}
	return out, nil
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
		return out(a.Proposed(ctx, f))
	case "counts":
		return out(a.ProposedCount(ctx, f.Space))
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
