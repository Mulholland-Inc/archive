package main

// Groups: a group is a row of archive."group" (archive.sql), and a row's access is the id of the group that
// sees it, a foreign key to that row, or null for the whole workspace. Writes may name a group instead of its
// id, in access and in a member's groups, and groupNames makes the name its id.

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var groupNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_]{0,23}$`)

// groupID is the id of the group s names, by its id or its name; nil for "", the whole workspace. With make,
// a name no group has yet becomes a new group, as a write of an owner's makes it.
func (a *Archive) groupID(ctx context.Context, s string, make bool) (*uuid.UUID, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	if id, err := uuid.Parse(s); err == nil {
		return &id, nil
	}
	name := strings.ToLower(s)
	id, err := a.st.db(ctx).GroupByName(ctx, name)
	if errors.Is(err, pgx.ErrNoRows) && make && groupNameRe.MatchString(name) {
		err = a.st.Q(ctx).QueryRow(ctx, `INSERT INTO archive."group" (name) VALUES ($1) RETURNING id`, name).Scan(&id)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, &ValidationError{Type: "group", Errors: []FieldError{{Path: "access", Message: fmt.Sprintf(`no group %q; make it with archive.create("group", { name: %q })`, s, name)}}}
	}
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// groupNames makes each group a write names by its name into its id: its access, and a member's groups, where
// a name no group has yet makes the group, as setting a member's groups by name always has.
func (a *Archive) groupNames(ctx context.Context, tb *Table, in map[string]any) error {
	if s, ok := in["access"].(string); ok && tb.col("access") != nil {
		id, err := a.groupID(ctx, s, false)
		if ve := (*ValidationError)(nil); errors.As(err, &ve) {
			ve.Type = tb.Name
		}
		if err != nil {
			return err
		}
		if id == nil {
			in["access"] = nil
		} else {
			in["access"] = id.String()
		}
	}
	if tb.Schema != "archive" || tb.Name != "member" {
		return nil
	}
	list, ok := in["groups"].([]any)
	if !ok {
		return nil
	}
	out := make([]any, 0, len(list))
	for _, v := range list {
		s, ok := v.(string)
		if !ok {
			out = append(out, v)
			continue
		}
		id, err := a.groupID(ctx, s, true)
		if err != nil {
			var ve *ValidationError
			if errors.As(err, &ve) {
				ve.Type, ve.Errors[0].Path = "member", "groups"
			}
			return err
		}
		if id != nil {
			out = append(out, id.String())
		}
	}
	in["groups"] = out
	return nil
}
