package main

import (
	"cmp"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Mulholland-Inc/archive/generated"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed archive.sql
var storeSQLFile string

var storeSchema, _, _ = strings.Cut(storeSQLFile, "\n-- name:")

const storeRoles = `DO $$ BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '{p}_authenticator') THEN
        CREATE ROLE "{p}_authenticator" LOGIN NOINHERIT;
    END IF;
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '{p}_viewer') THEN
        CREATE ROLE "{p}_viewer" NOLOGIN;
    END IF;
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '{p}_editor') THEN
        CREATE ROLE "{p}_editor" NOLOGIN IN ROLE "{p}_viewer";
    END IF;
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '{p}_owner') THEN
        CREATE ROLE "{p}_owner" NOLOGIN IN ROLE "{p}_editor";
    END IF;
    IF NOT EXISTS (SELECT FROM pg_db_role_setting s JOIN pg_database d ON d.oid = s.setdatabase WHERE d.datname = current_database() AND s.setrole = 0) THEN
        EXECUTE format('ALTER DATABASE %I SET search_path = archive, public', current_database());
        EXECUTE format('ALTER DATABASE %I SET timezone = %L', current_database(), 'UTC');
    END IF;
    IF NOT has_database_privilege('{p}_authenticator', current_database(), 'CONNECT') THEN
        EXECUTE format('GRANT CONNECT ON DATABASE %I TO "{p}_authenticator"', current_database());
    END IF;
END $$`

const storeLock = 7271

type storeQ interface {
	generated.DBTX
	Begin(ctx context.Context) (pgx.Tx, error)
}

type storeTxKey struct{}

type storeTx struct {
	tx    pgx.Tx
	role  string
	after *[]func()
	sch   *schemaTx
}

func storeAfter(ctx context.Context, fn func()) {
	if t, ok := ctx.Value(storeTxKey{}).(storeTx); ok && t.after != nil {
		*t.after = append(*t.after, fn)
		return
	}
	fn()
}

func storeCommitted(after *[]func(), err error) error {
	if err == nil {
		for _, fn := range *after {
			fn()
		}
	}
	return err
}

type pgStore struct {
	app, sys *pgxpool.Pool
	cfg      *pgx.ConnConfig
	prefix   string
}

func OpenStore(ctx context.Context, pg *pgx.ConnConfig, prefix, password string) (*pgStore, error) {
	cfg := pg.Copy()
	cfg.RuntimeParams["search_path"] = "public"
	s := &pgStore{cfg: cfg, prefix: prefix}
	var err error
	if s.sys, err = storePool(ctx, pg, storeSysConns, nil); err == nil {
		err = s.bootstrap(ctx, password)
	}
	if err == nil {
		auth := prefix + "_authenticator"
		s.app, err = storePool(ctx, pg, storeAppConns, func(c *pgx.ConnConfig) { c.User, c.Password = auth, password })
	}
	if err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

const (
	storeAppConns = 8
	storeSysConns = 4
	storeIdle     = time.Minute
)

func storePool(ctx context.Context, base *pgx.ConnConfig, conns int32, fn func(*pgx.ConnConfig)) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig("")
	if err != nil {
		return nil, err
	}
	cfg.ConnConfig = base.Copy()
	cfg.ConnConfig.RuntimeParams["search_path"] = "archive, public"
	cfg.ConnConfig.RuntimeParams["timezone"] = "UTC"
	cfg.MaxConns, cfg.MinConns, cfg.MaxConnIdleTime = conns, 0, storeIdle
	if fn != nil {
		fn(cfg.ConnConfig)
	}
	return pgxpool.NewWithConfig(ctx, cfg)
}

// bootstrap makes the store's roles and sets the authenticator's password. Processes opening the same store at
// once would race on those roles' catalog rows, so it holds the store lock while it does.
func (s *pgStore) bootstrap(ctx context.Context, password string) error {
	tx, err := s.sys.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	if err := generated.New(tx).AdvisoryXactLock(ctx, storeLock); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, s.expand(storeRoles)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "ALTER ROLE "+pgIdent(s.prefix+"_authenticator")+" PASSWORD '"+password+"'"); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	rev, _ := generated.New(s.sys).Revision(ctx)
	if strings.HasSuffix(rev, ":"+s.hash()) {
		return nil
	}
	return s.migrate(ctx)
}

func (s *pgStore) expand(sql string) string { return strings.ReplaceAll(sql, "{p}", s.prefix) }

func (s *pgStore) hash() string {
	h := sha256.Sum256([]byte(s.expand(storeSchema)))
	return hex.EncodeToString(h[:])
}

func storeUnquote(id string) string {
	if strings.HasPrefix(id, `"`) {
		return strings.ReplaceAll(id[1:len(id)-1], `""`, `"`)
	}
	return id
}

var (
	storeDollarRe = regexp.MustCompile(`^\$([A-Za-z_][A-Za-z0-9_]*)?\$`)
	storeIdentRe  = regexp.MustCompile(`[A-Za-z0-9_$]`)
)

func storeSplit(src string) []string {
	var out []string
	start := 0
	for i := 0; i < len(src); i++ {
		switch c := src[i]; {
		case c == '-' && strings.HasPrefix(src[i:], "--"):
			if n := strings.IndexByte(src[i:], '\n'); n >= 0 {
				i += n
			} else {
				i = len(src)
			}
		case c == '/' && strings.HasPrefix(src[i:], "/*"):
			for d := 0; i < len(src); i++ {
				if strings.HasPrefix(src[i:], "/*") {
					d, i = d+1, i+1
				} else if strings.HasPrefix(src[i:], "*/") {
					if d, i = d-1, i+1; d == 0 {
						break
					}
				}
			}
		case c == '\'' || c == '"':
			esc := c == '\'' && i > 0 && (src[i-1] == 'E' || src[i-1] == 'e')
			for i++; i < len(src) && src[i] != c; i++ {
				if esc && src[i] == '\\' {
					i++
				}
			}
		case c == '$':
			if m := storeDollarRe.FindString(src[i:]); m != "" && (i == 0 || !storeIdentRe.MatchString(src[i-1:i])) {
				if n := strings.Index(src[i+len(m):], m); n >= 0 {
					i += len(m) + n + len(m) - 1
				} else {
					i = len(src)
				}
			}
		case c == ';':
			if st := strings.TrimSpace(src[start:i]); st != "" {
				out = append(out, st)
			}
			start = i + 1
		}
	}
	if st := strings.TrimSpace(src[start:]); st != "" {
		out = append(out, st)
	}
	return out
}

func (s *pgStore) Catalog(ctx context.Context) (*Catalog, error) {
	q := generated.New(s.sys)
	b, err := q.Catalog(ctx)
	if err != nil {
		return nil, err
	}
	rev, err := q.Revision(ctx)
	if err != nil {
		return nil, err
	}
	c, err := catalogParse(b)
	if c != nil {
		c.stamp = rev
	}
	return c, err
}

func (s *pgStore) Close() {
	if s.app != nil {
		s.app.Close()
	}
	if s.sys != nil {
		s.sys.Close()
	}
}

func (s *pgStore) Q(ctx context.Context) storeQ {
	if t, ok := ctx.Value(storeTxKey{}).(storeTx); ok {
		return t.tx
	}
	return s.app
}

func (s *pgStore) db(ctx context.Context) *generated.Queries { return generated.New(s.Q(ctx)) }

func (s *pgStore) As(ctx context.Context, role string, fn func(context.Context) error) error {
	if t, ok := ctx.Value(storeTxKey{}).(storeTx); ok && t.role == role {
		return fn(ctx)
	}
	after := &[]func(){}
	return storeCommitted(after, storeErr(pgx.BeginFunc(ctx, s.app, func(tx pgx.Tx) error {
		if err := generated.New(tx).SetLocal(ctx, "role", role); err != nil {
			return err
		}
		return fn(context.WithValue(ctx, storeTxKey{}, storeTx{tx, role, after, &schemaTx{}}))
	})))
}

func (s *pgStore) System(ctx context.Context, fn func(context.Context) error) error {
	if t, ok := ctx.Value(storeTxKey{}).(storeTx); ok && t.role == "" {
		return fn(ctx)
	}
	after := &[]func(){}
	return storeCommitted(after, storeErr(pgx.BeginFunc(ctx, s.sys, func(tx pgx.Tx) error {
		return fn(context.WithValue(ctx, storeTxKey{}, storeTx{tx, "", after, &schemaTx{}}))
	})))
}

func storeErr(err error) error {
	var pe *pgconn.PgError
	if !errors.As(err, &pe) {
		return err
	}
	switch {
	case pe.Code == "42501" && (strings.HasPrefix(pe.Message, "must be owner of ") || strings.HasPrefix(pe.Message, "permission denied for schema ")):
		return fmt.Errorf("%s; change the schema with archive.migrate: %w", pe.Message, ErrForbidden)
	case pe.Code == "42501":
		return fmt.Errorf("%s: %w", pe.Message, ErrForbidden)
	case pe.Code == "23503" || pe.Code == "23001" || pe.Code == "23505":
		return fmt.Errorf("%s: %w", cmp.Or(pe.Detail, pe.Message), ErrConflict)
	case pe.Code == "23502" && pe.ColumnName != "":
		return &ValidationError{Type: pe.TableName, Errors: []FieldError{{Path: pe.ColumnName, Message: "required"}}}
	case pe.Code == "23514" || pe.Code == "23502" || strings.HasPrefix(pe.Code, "22"):
		return &ValidationError{Type: cmp.Or(pe.TableName, pe.DataTypeName), Errors: []FieldError{{Path: pe.ColumnName, Message: pe.Message}}}
	}
	return err
}

func pgIdent(parts ...string) string { return pgx.Identifier(parts).Sanitize() }

func storeRows(ctx context.Context, s *pgStore, sql string, args ...any) ([]map[string]any, error) {
	rows, _ := s.Q(ctx).Query(ctx, sql, args...)
	out := []map[string]any{}
	var b []byte
	_, err := pgx.ForEachRow(rows, []any{&b}, func() error {
		r, err := rowDecode(b)
		out = append(out, r)
		return err
	})
	return out, err
}

func rowDecode(b []byte) (map[string]any, error) {
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.UseNumber()
	var m map[string]any
	return m, dec.Decode(&m)
}
