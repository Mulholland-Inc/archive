package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	//go:embed schema.sql
	storeSchema string
	//go:embed seed.sql
	storeSeed string
)

type storeQ interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

type storeTxKey struct{}

type storeTx struct {
	tx   pgx.Tx
	role string
}

type pgStore struct{ app, sys *pgxpool.Pool }

func OpenStore(ctx context.Context, dsn, prefix, password string) (*pgStore, error) {
	sys, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if _, err := sys.Exec(ctx, strings.ReplaceAll(storeSchema, "{p}", prefix)+"\n"+storeSeed); err != nil {
		sys.Close()
		return nil, err
	}
	auth := prefix + "_authenticator"
	if _, err := sys.Exec(ctx, "ALTER ROLE "+auth+" PASSWORD '"+password+"'"); err != nil {
		sys.Close()
		return nil, err
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		sys.Close()
		return nil, err
	}
	cfg.ConnConfig.User, cfg.ConnConfig.Password = auth, password
	app, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		sys.Close()
		return nil, err
	}
	return &pgStore{app: app, sys: sys}, nil
}

func (s *pgStore) Close() {
	s.app.Close()
	s.sys.Close()
}

func (s *pgStore) Q(ctx context.Context) storeQ {
	if t, ok := ctx.Value(storeTxKey{}).(storeTx); ok {
		return t.tx
	}
	return s.app
}

func (s *pgStore) As(ctx context.Context, role string, fn func(context.Context) error) error {
	if t, ok := ctx.Value(storeTxKey{}).(storeTx); ok && t.role == role {
		return fn(ctx)
	}
	return storeErr(pgx.BeginFunc(ctx, s.app, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SET LOCAL ROLE "+pgx.Identifier{role}.Sanitize()); err != nil {
			return err
		}
		return fn(context.WithValue(ctx, storeTxKey{}, storeTx{tx, role}))
	}))
}

func (s *pgStore) System(ctx context.Context, fn func(context.Context) error) error {
	if t, ok := ctx.Value(storeTxKey{}).(storeTx); ok && t.role == "" {
		return fn(ctx)
	}
	return pgx.BeginFunc(ctx, s.sys, func(tx pgx.Tx) error {
		return fn(context.WithValue(ctx, storeTxKey{}, storeTx{tx, ""}))
	})
}

func storeErr(err error) error {
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == "42501" {
		return fmt.Errorf("%s: %w", pe.Message, ErrForbidden)
	}
	return err
}

func storeOne[T any](ctx context.Context, s *pgStore, sql string, args ...any) (*T, error) {
	rows, _ := s.Q(ctx).Query(ctx, sql, args...)
	v, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[T])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return v, err
}

func storeAll[T any](ctx context.Context, s *pgStore, sql string, args ...any) ([]*T, error) {
	rows, _ := s.Q(ctx).Query(ctx, sql, args...)
	return pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[T])
}

const (
	storeStamp   = `SELECT coalesce(max(updated_at)::text, '') || ':' || count(*) FROM type`
	storePutType = `
		INSERT INTO type (name, extends, def, updated_at) VALUES ($1, $2, $3, now())
		ON CONFLICT (name) DO UPDATE SET extends = excluded.extends, def = excluded.def, updated_at = now()`
)

func (s *pgStore) SpecStamp(ctx context.Context) (string, error) {
	var stamp string
	return stamp, s.Q(ctx).QueryRow(ctx, storeStamp).Scan(&stamp)
}

func (s *pgStore) CurrentSpec(ctx context.Context) (*Spec, error) {
	q := s.Q(ctx)
	sp := &Spec{Types: map[string]*Type{}}
	var err error
	if sp.stamp, err = s.SpecStamp(ctx); err != nil {
		return nil, err
	}
	rows, _ := q.Query(ctx, `SELECT name, extends, def, builtin, updated_at IS NOT NULL FROM type`)
	var name, extends string
	var def []byte
	var builtin, edited bool
	_, err = pgx.ForEachRow(rows, []any{&name, &extends, &def, &builtin, &edited}, func() error {
		t := &Type{}
		dec := json.NewDecoder(bytes.NewReader(def))
		dec.DisallowUnknownFields()
		if err := dec.Decode(t); err != nil {
			return fmt.Errorf("type %s: %w", name, err)
		}
		t.Extends, t.Builtin, t.Edited = extends, builtin, edited
		sp.Types[name] = t
		return nil
	})
	specFix(sp)
	return sp, err
}

func (s *pgStore) PutType(ctx context.Context, t *Type) error {
	def := *t
	def.Extends = ""
	_, err := s.Q(ctx).Exec(ctx, storePutType, t.Name, t.Extends, &def)
	return err
}

func (s *pgStore) DeleteType(ctx context.Context, name string) error {
	tag, err := s.Q(ctx).Exec(ctx, `DELETE FROM type WHERE name = $1`, name)
	if err == nil && tag.RowsAffected() == 0 {
		return fmt.Errorf("only ontology admins can remove types: %w", ErrForbidden)
	}
	return err
}

const (
	storeCreateEntity = `
		INSERT INTO entity (id, type, data, proposed, space) VALUES ($1, $2, coalesce($3::jsonb, '{}'), $4, $5)
		RETURNING created_at, updated_at`
	storeUpdateEntity = `
		UPDATE entity SET type = $2, data = coalesce($3::jsonb, '{}'), proposed = $4, updated_at = now()
		WHERE id = $1 RETURNING created_at, updated_at`

	storeListEntities = `
		SELECT * FROM entity
		WHERE (cardinality($1::text[]) = 0 OR type = ANY($1))
		  AND data @> $2::jsonb AND id > $3 AND proposed = $5`
)

func (s *pgStore) CreateEntity(ctx context.Context, e *Entity) error {
	if e.ID == uuid.Nil {
		e.ID = coreNewID()
	}
	return s.Q(ctx).QueryRow(ctx, storeCreateEntity, e.ID, e.Type, e.Data, e.Proposed, e.Space).Scan(&e.CreatedAt, &e.UpdatedAt)
}

func (s *pgStore) GetEntity(ctx context.Context, id uuid.UUID) (*Entity, error) {
	return storeOne[Entity](ctx, s, `SELECT * FROM entity WHERE id = $1`, id)
}

func (s *pgStore) UpdateEntity(ctx context.Context, e *Entity) error {
	err := s.Q(ctx).QueryRow(ctx, storeUpdateEntity, e.ID, e.Type, e.Data, e.Proposed).Scan(&e.CreatedAt, &e.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (s *pgStore) DeleteEntity(ctx context.Context, id uuid.UUID) error {
	tag, err := s.Q(ctx).Exec(ctx, `DELETE FROM entity WHERE id = $1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (s *pgStore) ListEntities(ctx context.Context, f Filter) ([]*Entity, error) {
	types, where, limit := f.Types, f.Where, f.Limit
	if types == nil {
		types = []string{}
	}
	if where == nil {
		where = map[string]any{}
	}
	if limit <= 0 {
		limit = 100
	}
	sql, args := storeListEntities, []any{types, where, f.After, limit, f.Proposed}
	for _, k := range specKeys(f.Keys) {
		args = append(args, f.Keys[k])
		sql += fmt.Sprintf(" AND data ->> %s = $%d", storeLiteral(k), len(args))
	}
	if f.Space != uuid.Nil {
		args = append(args, f.Space)
		sql += fmt.Sprintf(" AND space = $%d", len(args))
	}
	out, err := storeAll[Entity](ctx, s, sql+" ORDER BY id LIMIT $4", args...)
	if out == nil {
		out = []*Entity{}
	}
	return out, err
}

func storeLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
