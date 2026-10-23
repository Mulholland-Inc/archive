package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const (
	walkMaxDepth   = 25
	walkMaxRecords = 5000
)

// WalkRecord is a record a walk reached, with the round it was reached in: the root is 0, its
// parents 1, their parents 2.
type WalkRecord struct {
	*Record
	Depth int `json:"depth"`
}

type Walk struct {
	Records []*WalkRecord `json:"records"`
	Links   []*Link       `json:"links"`
}

// Walk follows links from one object, a round of steps at a time, for up to depth rounds. A step
// is "out:field", from a record to what its reference field points at, or "in:field", from a
// record to those whose field points at it. A pedigree is in:child,out:parent through parent
// links, six rounds deep, in one call. It reads as the caller, so what they cannot see is
// neither returned nor walked through, and a record reached twice is walked from once.
func (a *Archive) Walk(ctx context.Context, typ string, id uuid.UUID, steps []string, depth int) (*Walk, error) {
	verr := &ValidationError{Type: "walk"}
	if len(steps) == 0 {
		verr.Errors = append(verr.Errors, FieldError{Path: "steps", Message: "want steps such as in:child,out:parent"})
	}
	type step struct {
		in   bool
		kind string
	}
	var ss []step
	for i, s := range steps {
		dir, kind, ok := strings.Cut(strings.TrimSpace(s), ":")
		if !ok || kind == "" || (dir != "in" && dir != "out") {
			verr.Errors = append(verr.Errors, FieldError{Path: fmt.Sprintf("steps[%d]", i), Message: "want in:field or out:field"})
		}
		ss = append(ss, step{dir == "in", kind})
	}
	if depth < 1 || depth > walkMaxDepth {
		verr.Errors = append(verr.Errors, FieldError{Path: "depth", Message: fmt.Sprintf("want 1 to %d", walkMaxDepth)})
	}
	if len(verr.Errors) > 0 {
		return nil, verr
	}
	return coreDo(a, ctx, func(ctx context.Context) (*Walk, error) {
		root, err := a.Get(ctx, typ, id)
		if err != nil {
			return nil, err
		}
		depthOf := map[uuid.UUID]int{root.ID: 0}
		order := []uuid.UUID{root.ID}
		frontier := []uuid.UUID{root.ID}
		w := &Walk{Links: []*Link{}}
		for d := 1; d <= depth && len(frontier) > 0; d++ {
			for _, s := range ss {
				col, other := "from_id", "to_id"
				if s.in {
					col, other = "to_id", "from_id"
				}
				ls, err := storeAll[Link](ctx, a.st, `SELECT l.*, '' AS from_type FROM links l WHERE l.kind = $1 AND l.`+col+` = ANY($2) ORDER BY l.id`, s.kind, frontier)
				if err != nil {
					return nil, err
				}
				var next []uuid.UUID
				for _, l := range ls {
					w.Links = append(w.Links, l)
					n := l.ToID
					if other == "from_id" {
						n = l.FromID
					}
					if _, seen := depthOf[n]; seen {
						continue
					}
					depthOf[n] = d
					order = append(order, n)
					next = append(next, n)
				}
				if len(order) > walkMaxRecords {
					return nil, &ValidationError{Type: "walk", Errors: []FieldError{{Path: "depth", Message: fmt.Sprintf("reaches more than %d records; walk less deep", walkMaxRecords)}}}
				}
				frontier = next
			}
		}
		rs, err := storeAll[Record](ctx, a.st, `SELECT * FROM records WHERE id = ANY($1)`, order)
		if err != nil {
			return nil, err
		}
		byID := map[uuid.UUID]*Record{}
		for _, r := range rs {
			byID[r.ID] = r
		}
		for _, id := range order {
			if r := byID[id]; r != nil {
				w.Records = append(w.Records, &WalkRecord{Record: r, Depth: depthOf[id]})
			}
		}
		return w, nil
	})
}
