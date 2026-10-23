package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

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
	storeVersion = `SELECT coalesce(max(to_version), 0) FROM changes WHERE status IN ('applied', 'reverted')`
	storePutType = `
		INSERT INTO types (name, extends, def, updated_at) VALUES ($1, $2, $3, now())
		ON CONFLICT (name) DO UPDATE SET extends = excluded.extends, def = excluded.def, updated_at = now()`
)

func (s *pgStore) CurrentSpec(ctx context.Context) (*Spec, error) {
	q := s.Q(ctx)
	sp := &Spec{Types: map[string]*Type{}}
	if err := q.QueryRow(ctx, storeVersion).Scan(&sp.Version); err != nil {
		return nil, err
	}
	rows, _ := q.Query(ctx, `SELECT name, extends, def, builtin, updated_at IS NOT NULL FROM types`)
	var name, extends string
	var def []byte
	var builtin, edited bool
	_, err := pgx.ForEachRow(rows, []any{&name, &extends, &def, &builtin, &edited}, func() error {
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

func (s *pgStore) ApplyChange(ctx context.Context, next *Spec, c, undo *Change) error {
	return pgx.BeginFunc(ctx, s.Q(ctx), func(tx pgx.Tx) error {
		var ver int
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(7272)`); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, storeVersion).Scan(&ver); err != nil {
			return err
		}
		if ver != c.FromVer {
			return fmt.Errorf("change %s is based on spec v%d, current is v%d: %w", c.ID, c.FromVer, ver, ErrConflict)
		}
		for _, op := range c.Ops {
			t, ok := next.Types[op.Type]
			if !ok {
				if _, err := tx.Exec(ctx, `DELETE FROM types WHERE name = $1`, op.Type); err != nil {
					return err
				}
				continue
			}
			def := *t
			def.Extends = ""
			if _, err := tx.Exec(ctx, storePutType, op.Type, t.Extends, &def); err != nil {
				return err
			}
		}
		if err := storePutChange(ctx, tx, undo); err != nil {
			return err
		}
		return storePutChange(ctx, tx, c)
	})
}

const (
	storeInsertRaw = `
		INSERT INTO raw (source, external_id, hash, fetched_at, payload, space)
		VALUES ($1, $2, $3, $4, coalesce($5::jsonb, 'null'), $6)
		ON CONFLICT (space, source, external_id, hash) DO NOTHING
		RETURNING id, fetched_at`
	storeFindRaw = `SELECT id, fetched_at FROM raw WHERE source = $1 AND external_id = $2 AND hash = $3 AND space = $4`
)

func (s *pgStore) PutRaw(ctx context.Context, r *Raw) (bool, error) {
	if r.Hash == "" {
		sum := sha256.Sum256(r.Payload)
		r.Hash = hex.EncodeToString(sum[:])
	}
	if r.FetchedAt.IsZero() {
		r.FetchedAt = time.Now()
	}
	q := s.Q(ctx)
	err := q.QueryRow(ctx, storeInsertRaw, r.Source, r.ExternalID, r.Hash, r.FetchedAt, r.Payload, r.Space).Scan(&r.ID, &r.FetchedAt)
	if !errors.Is(err, pgx.ErrNoRows) {
		return err == nil, err
	}
	return false, q.QueryRow(ctx, storeFindRaw, r.Source, r.ExternalID, r.Hash, r.Space).Scan(&r.ID, &r.FetchedAt)
}

func (s *pgStore) GetRaw(ctx context.Context, id uuid.UUID) (*Raw, error) {
	return storeOne[Raw](ctx, s, `SELECT * FROM raw WHERE id = $1`, id)
}

const (
	storeCreateRecord = `
		INSERT INTO records (id, type, spec_version, data, space) VALUES ($1, $2, $3, coalesce($4::jsonb, '{}'), $5)
		RETURNING created_at, updated_at`
	storeUpdateRecord = `
		UPDATE records SET type = $2, spec_version = $3, data = coalesce($4::jsonb, '{}'), updated_at = now()
		WHERE id = $1 RETURNING created_at, updated_at`

	storeListRecords = `
		SELECT * FROM records
		WHERE (cardinality($1::text[]) = 0 OR type = ANY($1))
		  AND data @> $2::jsonb AND id > $3`
)

func (s *pgStore) CreateRecord(ctx context.Context, r *Record) error {
	if r.ID == uuid.Nil {
		r.ID = coreNewID()
	}
	return s.Q(ctx).QueryRow(ctx, storeCreateRecord, r.ID, r.Type, r.SpecVersion, r.Data, r.Space).Scan(&r.CreatedAt, &r.UpdatedAt)
}

func (s *pgStore) GetRecord(ctx context.Context, id uuid.UUID) (*Record, error) {
	return storeOne[Record](ctx, s, `SELECT * FROM records WHERE id = $1`, id)
}

func (s *pgStore) UpdateRecord(ctx context.Context, r *Record) error {
	err := s.Q(ctx).QueryRow(ctx, storeUpdateRecord, r.ID, r.Type, r.SpecVersion, r.Data).Scan(&r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (s *pgStore) DeleteRecord(ctx context.Context, id uuid.UUID) error {
	tag, err := s.Q(ctx).Exec(ctx, `DELETE FROM records WHERE id = $1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (s *pgStore) ListRecords(ctx context.Context, f Filter) ([]*Record, error) {
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
	sql, args := storeListRecords, []any{types, where, f.After, limit}
	for _, k := range specKeys(f.Keys) {
		args = append(args, f.Keys[k])
		sql += fmt.Sprintf(" AND data ->> %s = $%d", storeLiteral(k), len(args))
	}
	if f.Space != uuid.Nil {
		args = append(args, f.Space)
		sql += fmt.Sprintf(" AND space = $%d", len(args))
	}
	return storeAll[Record](ctx, s, sql+" ORDER BY id LIMIT $4", args...)
}

func storeLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

const storeAddProvenance = `
	INSERT INTO provenance (record_id, field_path, raw_ids, method, confidence, original, space, file_id, page, source_id)
	VALUES ($1, $2, coalesce($3, '{}'::uuid[]), $4, $5, $6, (SELECT space FROM records WHERE id = $1), $7, $8, $9) RETURNING id, space`

func (s *pgStore) AddProvenance(ctx context.Context, ps []*Provenance) error {
	return pgx.BeginFunc(ctx, s.Q(ctx), func(tx pgx.Tx) error {
		for _, p := range ps {
			err := tx.QueryRow(ctx, storeAddProvenance, p.RecordID, p.FieldPath, p.RawIDs, p.Method, p.Confidence, p.Original, p.FileID, p.Page, p.SourceID).Scan(&p.ID, &p.Space)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *pgStore) ListProvenance(ctx context.Context, recordID uuid.UUID) ([]*Provenance, error) {
	return storeAll[Provenance](ctx, s, `SELECT * FROM provenance WHERE record_id = $1 ORDER BY id`, recordID)
}

func (s *pgStore) SetLinks(ctx context.Context, r *Record, ls []*Link) error {
	q := s.Q(ctx)
	if _, err := q.Exec(ctx, `DELETE FROM links WHERE from_id = $1`, r.ID); err != nil {
		return err
	}
	for _, l := range ls {
		err := q.QueryRow(ctx, `INSERT INTO links (from_id, to_id, kind, space) VALUES ($1, $2, $3, $4) RETURNING id`,
			r.ID, l.ToID, l.Kind, r.Space).Scan(&l.ID)
		if err != nil {
			return err
		}
		l.FromID, l.Space = r.ID, r.Space
	}
	return nil
}

func (s *pgStore) ListLinks(ctx context.Context, id uuid.UUID) ([]*Link, error) {
	return storeAll[Link](ctx, s, `SELECT l.*, coalesce(r.type, '') AS from_type FROM links l LEFT JOIN records r ON r.id = l.from_id WHERE l.from_id = $1 OR l.to_id = $1 ORDER BY l.id`, id)
}

const storePutChangeSQL = `
	INSERT INTO changes (id, proposed_by, status, ops, from_version, to_version)
	VALUES (coalesce($1, uuidv7()), $2, $3, $4, $5, $6)
	ON CONFLICT (id) DO UPDATE SET
		proposed_by = excluded.proposed_by, status = excluded.status, ops = excluded.ops,
		from_version = excluded.from_version, to_version = excluded.to_version
	RETURNING id, created_at`

func (s *pgStore) PutChange(ctx context.Context, c *Change) error {
	return storePutChange(ctx, s.Q(ctx), c)
}

func storePutChange(ctx context.Context, q storeQ, c *Change) error {
	var id *uuid.UUID
	if c.ID != uuid.Nil {
		id = &c.ID
	}
	ops, err := json.Marshal(c.Ops)
	if err != nil {
		return err
	}
	return q.QueryRow(ctx, storePutChangeSQL, id, c.ProposedBy, c.Status, ops, c.FromVer, c.ToVer).Scan(&c.ID, &c.CreatedAt)
}

func (s *pgStore) GetChange(ctx context.Context, id uuid.UUID) (*Change, error) {
	return storeOne[Change](ctx, s, `
		SELECT id, proposed_by, status, ops, from_version AS from_ver, to_version AS to_ver, created_at
		FROM changes WHERE id = $1`, id)
}
