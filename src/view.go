package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

type View struct {
	ID        uuid.UUID         `json:"id"`
	Name      string            `json:"name"`
	Space     uuid.UUID         `json:"space"`
	Type      string            `json:"type"`
	Layout    string            `json:"layout"`
	By        string            `json:"by,omitempty" db:"by_field"`
	Where     map[string]string `json:"where" db:"filter"`
	CreatedBy string            `json:"created_by"`
	CreatedAt time.Time         `json:"created_at"`
}

type ViewInput struct {
	Name   string            `json:"name"`
	Space  string            `json:"space"`
	Type   string            `json:"type"`
	Layout string            `json:"layout"`
	By     string            `json:"by"`
	Where  map[string]string `json:"where"`
}

const viewCols = `id, name, space, type, layout, by_field, filter, created_by, created_at`

var viewLayouts = []string{"table", "board", "calendar"}

func viewInvalid(path, msg string) error {
	return &ValidationError{Type: "view", Errors: []FieldError{{Path: path, Message: msg}}}
}

func viewErr(err error) error {
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == "23505" {
		return fmt.Errorf("a view with that name already exists in this space: %w", ErrConflict)
	}
	return err
}

func (a *Archive) viewCheck(in *ViewInput) error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len(in.Name) > 100 {
		return viewInvalid("name", "want 1-100 characters")
	}
	if in.Layout == "" {
		in.Layout = "table"
	}
	if !slices.Contains(viewLayouts, in.Layout) {
		return viewInvalid("layout", "want "+strings.Join(viewLayouts, ", "))
	}
	if err := coreHasType(a.Spec(), in.Type); err != nil {
		return err
	}
	fields, err := a.Spec().Fields(in.Type)
	if err != nil {
		return err
	}
	if in.By != "" && fields[in.By] == nil {
		return viewInvalid("by", "unknown field")
	}
	for _, k := range specKeys(in.Where) {
		if fields[k] == nil {
			return viewInvalid("where."+k, "unknown field")
		}
	}
	if in.Where == nil {
		in.Where = map[string]string{}
	}
	return nil
}

func (a *Archive) Views(ctx context.Context) ([]*View, error) {
	return coreDo(a, ctx, func(ctx context.Context) ([]*View, error) {
		out, err := storeAll[View](ctx, a.st, `SELECT `+viewCols+` FROM views ORDER BY name, id`)
		if out == nil {
			out = []*View{}
		}
		return out, err
	})
}

func (a *Archive) GetView(ctx context.Context, id uuid.UUID) (*View, error) {
	return coreDo(a, ctx, func(ctx context.Context) (*View, error) {
		v, err := storeOne[View](ctx, a.st, `SELECT `+viewCols+` FROM views WHERE id = $1`, id)
		if errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("view %s: %w", id, ErrNotFound)
		}
		return v, err
	})
}

func (a *Archive) AddView(ctx context.Context, in ViewInput) (*View, error) {
	if err := a.viewCheck(&in); err != nil {
		return nil, err
	}
	v, err := coreDo(a, ctx, func(ctx context.Context) (*View, error) {
		sid, err := a.coreSpace(ctx, in.Space)
		if err != nil {
			return nil, err
		}
		return storeOne[View](ctx, a.st, `INSERT INTO views (name, space, type, layout, by_field, filter, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING `+viewCols, in.Name, sid, in.Type, in.Layout, in.By, in.Where, ActorFrom(ctx).Label)
	})
	return v, viewErr(err)
}

func (a *Archive) SetView(ctx context.Context, id uuid.UUID, in ViewInput) (*View, error) {
	if err := a.viewCheck(&in); err != nil {
		return nil, err
	}
	v, err := coreDo(a, ctx, func(ctx context.Context) (*View, error) {
		old, err := a.GetView(ctx, id)
		if err != nil {
			return nil, err
		}
		sid := old.Space
		if in.Space != "" {
			if sid, err = a.coreSpace(ctx, in.Space); err != nil {
				return nil, err
			}
		}
		v, err := storeOne[View](ctx, a.st, `UPDATE views SET name = $2, space = $3, type = $4, layout = $5, by_field = $6, filter = $7
			WHERE id = $1 RETURNING `+viewCols, id, in.Name, sid, in.Type, in.Layout, in.By, in.Where)
		if errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("you cannot change views in this space: %w", ErrForbidden)
		}
		return v, err
	})
	return v, viewErr(err)
}

func (a *Archive) RemoveView(ctx context.Context, id uuid.UUID) error {
	return a.as(ctx, func(ctx context.Context) error {
		if _, err := a.GetView(ctx, id); err != nil {
			return err
		}
		tag, err := a.st.Q(ctx).Exec(ctx, `DELETE FROM views WHERE id = $1`, id)
		if err == nil && tag.RowsAffected() == 0 {
			return fmt.Errorf("you cannot remove views in this space: %w", ErrForbidden)
		}
		return err
	})
}
