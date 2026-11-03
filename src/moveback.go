package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Mulholland-Inc/archive/connect"
	"github.com/Mulholland-Inc/archive/generated"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Moving each person's own connections back into an archive. For a while a person's own connections, like their
// Gmail, and what they synced were kept once in the main database, in a personal store that every workspace they
// were in read. A person's own connections are a workspace's again, connected in the archive they are in and
// seen only there. moveBack moves what the store still has into its person's oldest archive, the earliest-made
// workspace in their workspaces, at startup with the main database locked (Hub.migrate) and before main.sql
// applies; then it drops the store from the main database: its tables, functions, domains and roles.
//
// Postgres moves nothing between databases, so Go copies, as moveFirst does. For each workspace that gets
// connections, a transaction on the main database locks the store's tables against writes, and one on the
// workspace locks its tables of the same names. Each connection becomes one of its person's member there, by
// email, its token opened with ARCHIVE_SECRET_KEY and sealed again with the workspace's key, and its rows are
// copied with their ids, a message's party naming the workspace's known_person of the same address. Before
// anything commits, every table's rows of each connection are counted and hashed on both sides, and the token
// is opened again from the copy. Only once the workspace has committed does the main database give the
// connection's watches and its due sync to the workspace, delete the connection and its rows, and commit.
//
// Anything that does not match stops it before either database commits, and both stay as they were: a person
// who signs in no more, is in no archive, or has no member in their oldest; a token that does not open; a table
// whose columns differ; a connection the workspace already has to the same account. The start then fails saying
// why, as moveFirst's does. A start killed between the two commits finds the copy in the workspace and the
// connection still in the main database, which is what is real, and so makes the copy again. Once the store
// holds no connection, one transaction drops it; every start after that finds nothing of it and does nothing.
// Remove this, and its call in Hub.migrate, once no Archive has a personal store.

// moveBackSynced are the store's tables of what connections synced, each after the one its rows belong to.
var moveBackSynced = []string{"mail_message", "doc_page", "mail_party", "mail_attachment"}

// moveBackTables are the tables moveBack locks and copies, in both databases.
var moveBackTables = append([]string{"connection", "known_person"}, moveBackSynced...)

// moveBackBatch is how many rows of a table moveBack copies at a time.
const moveBackBatch = 200

// moveBackRoles are the roles reads of the store ran as, which nothing else has.
func (h *Hub) moveBackRoles() []string {
	sum := sha256.Sum256([]byte(h.pg.Database))
	p := "main_" + hex.EncodeToString(sum[:5])
	return []string{p + "_authenticator", p + "_personal"}
}

// moveBackLeft is what of the store the main database still has.
func (h *Hub) moveBackLeft(ctx context.Context, q generated.DBTX) ([]string, error) {
	rows, err := q.Query(ctx, `SELECT x FROM unnest($1::text[]) x WHERE to_regclass(x) IS NOT NULL
UNION ALL SELECT x FROM unnest($2::text[]) x WHERE to_regprocedure(x) IS NOT NULL
UNION ALL SELECT x FROM unnest($3::text[]) x WHERE to_regtype(x) IS NOT NULL
UNION ALL SELECT 'role ' || r.rolname FROM pg_catalog.pg_roles r WHERE r.rolname = ANY($4)`,
		[]string{"archive.mail_attachment", "archive.mail_party", "archive.mail_message", "archive.doc_page", "archive.connection",
			"archive.known_person", "archive.personal_key", "archive.object"},
		[]string{"archive.me()", "archive.my_type()", "archive.object_name()"},
		[]string{"archive.email", "archive.url"}, h.moveBackRoles())
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// moveBack moves the store's connections back into their archives and drops the store, if the main database
// has any of it, trying again a few times while others hold its tables.
func (h *Hub) moveBack(ctx context.Context, conn *pgx.Conn) error {
	for try := 1; ; try++ {
		left, err := h.moveBackLeft(ctx, conn)
		if err != nil || len(left) == 0 {
			return err
		}
		if err = h.moveBackOnce(ctx, conn); err == nil {
			return nil
		}
		fmt.Fprintf(os.Stderr, "archive: MOVING PEOPLE'S OWN CONNECTIONS BACK INTO THEIR ARCHIVES FAILED; what had not moved is in the main database, as it was: %v\n", err)
		var pe *pgconn.PgError
		if try == moveTries || ctx.Err() != nil || !errors.As(err, &pe) || pe.Code != "55P03" && pe.Code != "40P01" {
			return fmt.Errorf("moving people's own connections back from the main database into their archives: %w", err)
		}
		select {
		case <-time.After(time.Duration(try) * 2 * time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (h *Hub) moveBackOnce(ctx context.Context, conn *pgx.Conn) error {
	var some bool
	if err := conn.QueryRow(ctx, "SELECT to_regclass('archive.connection') IS NOT NULL").Scan(&some); err != nil {
		return err
	}
	if some {
		// Where each goes, its person's oldest archive, for every one before any moves.
		rows, _ := conn.Query(ctx, `SELECT c.id, c.provider, c.account, coalesce(u.email, ''),
    (SELECT w.id FROM archive.workspace w WHERE w.id = ANY (u.workspaces) ORDER BY w.created_at, w.id LIMIT 1)
FROM archive.connection c LEFT JOIN archive."user" u ON u.id = c.member ORDER BY c.id`)
		to := map[uuid.UUID][]uuid.UUID{}
		var id uuid.UUID
		var provider, account, email string
		var ws *uuid.UUID
		if _, err := pgx.ForEachRow(rows, []any{&id, &provider, &account, &email, &ws}, func() error {
			switch {
			case email == "":
				return fmt.Errorf("connection %s (%s %s) is of someone who no longer signs in, so it has no archive to go to", id, provider, account)
			case ws == nil:
				return fmt.Errorf("connection %s (%s %s) is of %s, who is in no archive, so it has none to go to", id, provider, account, email)
			}
			to[*ws] = append(to[*ws], id)
			return nil
		}); err != nil {
			return err
		}
		all, err := generated.New(conn).AllWorkspaces(ctx)
		if err != nil {
			return err
		}
		for _, w := range all {
			if ids := to[w.ID]; len(ids) > 0 {
				if err := h.moveBackTo(ctx, conn, w, ids); err != nil {
					return fmt.Errorf("workspace %s: %w", w.ID, err)
				}
			}
		}
	}
	return h.moveBackDrop(ctx, conn)
}

// moveBackLock locks tables of tx against writes, for as long as a move takes.
func moveBackLock(ctx context.Context, tx pgx.Tx, tables ...string) error {
	locks := make([]string, len(tables))
	for i, t := range tables {
		locks[i] = pgIdent("archive", t)
	}
	_, err := tx.Exec(ctx, "SET LOCAL lock_timeout = '"+moveLockTimeout+"'; SET LOCAL statement_timeout = 0; SET LOCAL TimeZone = 'UTC'; LOCK TABLE "+
		strings.Join(locks, ", ")+" IN EXCLUSIVE MODE")
	return err
}

// moveBackTo moves the store's connections ids, and their rows, into the workspace ws.
func (h *Hub) moveBackTo(ctx context.Context, conn *pgx.Conn, ws generated.ArchiveWorkspace, ids []uuid.UUID) error {
	bg := context.WithoutCancel(ctx)
	start := time.Now()
	key, err := h.unwrap(ws)
	if err != nil {
		return err
	}
	waead, err := secretAEAD(key)
	if err != nil {
		return err
	}
	mtx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer mtx.Rollback(bg)
	if err := moveBackLock(ctx, mtx, moveBackTables...); err != nil {
		return fmt.Errorf("locking the main database's tables: %w", err)
	}
	cfg := h.pg.Copy()
	cfg.Database = h.database(ws.ID.String())
	w, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return err
	}
	defer w.Close(bg)
	wtx, err := w.Begin(ctx)
	if err != nil {
		return err
	}
	defer wtx.Rollback(bg)
	if err := moveBackLock(ctx, wtx, moveBackTables...); err != nil {
		return fmt.Errorf("locking its tables: %w", err)
	}
	for _, t := range moveBackTables {
		if err := moveBackCols(ctx, mtx, wtx, t); err != nil {
			return err
		}
	}
	type row struct {
		id                       uuid.UUID
		email, provider, account string
		token                    []byte
		data                     map[string]any
	}
	var conns []row
	rs, _ := mtx.Query(ctx, `SELECT c.id, u.email, c.provider, c.account, c.token, to_jsonb(c) - 'token' - 'type'
FROM archive.connection c JOIN archive."user" u ON u.id = c.member WHERE c.id = ANY($1) ORDER BY c.id`, ids)
	var r row
	var raw []byte
	if _, err := pgx.ForEachRow(rs, []any{&r.id, &r.email, &r.provider, &r.account, &r.token, &raw}, func() error {
		c := r
		var err error
		c.data, err = moveBackJSON(raw)
		conns = append(conns, c)
		return err
	}); err != nil {
		return err
	}
	var moved []uuid.UUID
	var copied int64
	for _, c := range conns {
		what := fmt.Sprintf("connection %s (%s %s of %s)", c.id, c.provider, c.account, c.email)
		var member uuid.UUID
		err := wtx.QueryRow(ctx, "SELECT id FROM archive.member WHERE email = $1", c.email).Scan(&member)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%s: %s is no member of it, though it is their oldest archive", what, c.email)
		} else if err != nil {
			return err
		}
		rt, err := connect.OpenToken(h.aead, c.id, c.token)
		if err != nil {
			return fmt.Errorf("%s: %w", what, err)
		}
		var other uuid.UUID
		err = wtx.QueryRow(ctx, "SELECT id FROM archive.connection WHERE member = $1 AND provider = $2 AND account = $3 AND id <> $4",
			member, c.provider, c.account, c.id).Scan(&other)
		if err == nil {
			return fmt.Errorf("%s: the workspace has connection %s to the same account already", what, other)
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		// A copy an earlier try committed before it stopped is made again: what the main database has is real.
		var was struct {
			member            uuid.UUID
			provider, account string
		}
		err = wtx.QueryRow(ctx, "SELECT member, provider, account FROM archive.connection WHERE id = $1", c.id).Scan(&was.member, &was.provider, &was.account)
		switch {
		case err == nil && (was.member != member || was.provider != c.provider || was.account != c.account):
			return fmt.Errorf("%s: the workspace has a connection of this id already, to %s %s", what, was.provider, was.account)
		case err == nil:
			if _, err := wtx.Exec(ctx, "DELETE FROM archive.connection WHERE id = $1", c.id); err != nil {
				return err
			}
		case !errors.Is(err, pgx.ErrNoRows):
			return err
		}
		c.data["member"], c.data["token"], c.data["lease_at"] = member, `\x`+hex.EncodeToString(connect.SealToken(waead, c.id, rt)), nil
		if _, err := wtx.Exec(ctx, "INSERT INTO archive.connection SELECT (jsonb_populate_record(NULL::archive.connection, $1)).*", c.data); err != nil {
			return fmt.Errorf("%s: %w", what, err)
		}
		for _, t := range moveBackSynced {
			n, err := moveBackCopy(ctx, mtx, wtx, t, c.id)
			if err != nil {
				return fmt.Errorf("%s: %s: %w", what, t, err)
			}
			copied += n
		}
		for _, t := range append([]string{"connection"}, moveBackSynced...) {
			a, err := moveBackSum(ctx, mtx, t, c.id)
			if err != nil {
				return err
			}
			b, err := moveBackSum(ctx, wtx, t, c.id)
			if err != nil {
				return err
			}
			if a != b {
				return fmt.Errorf("%s: %s: the main database has %s, and the copy %s", what, t, a, b)
			}
		}
		var sealed []byte
		if err := wtx.QueryRow(ctx, "SELECT token FROM archive.connection WHERE id = $1", c.id).Scan(&sealed); err != nil {
			return err
		}
		if back, err := connect.OpenToken(waead, c.id, sealed); err != nil || back != rt {
			return fmt.Errorf("%s: its token does not open the same with the workspace's key: %v", what, err)
		}
		moved = append(moved, c.id)
	}
	if err := wtx.Commit(ctx); err != nil {
		return err
	}
	// The workspace has them; only now does the main database let them go.
	if _, err := mtx.Exec(ctx, "UPDATE archive.watch SET workspace = $1 WHERE connection = ANY($2) AND workspace = $3", ws.ID, moved, uuid.Nil); err != nil {
		return err
	}
	if _, err := mtx.Exec(ctx, `INSERT INTO archive.due (workspace, kind, at) VALUES ($1, 'sync', now())
ON CONFLICT (workspace, kind) DO UPDATE SET at = least(archive.due.at, EXCLUDED.at)`, ws.ID); err != nil {
		return err
	}
	if _, err := mtx.Exec(ctx, "DELETE FROM archive.connection WHERE id = ANY($1)", moved); err != nil {
		return err
	}
	for _, t := range moveBackSynced {
		var n int64
		if err := mtx.QueryRow(ctx, "SELECT count(*) FROM "+pgIdent("archive", t)+" WHERE connection = ANY($1)", moved).Scan(&n); err != nil || n > 0 {
			return cmpErr(err, fmt.Errorf("%s still has %d rows of the connections it deleted", t, n))
		}
	}
	if err := mtx.Commit(ctx); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "archive: workspace %s: moved %d of its members' own connections and %d rows they synced back into it from the main database, checked, in %s\n",
		ws.ID, len(moved), copied, time.Since(start).Round(time.Millisecond))
	return nil
}

// moveBackCols checks that table has the same columns, by name and type, in the store and the workspace, but
// for the store's connection.type, which the workspace has no need of.
func moveBackCols(ctx context.Context, mtx, wtx pgx.Tx, table string) error {
	cols := func(tx pgx.Tx) (map[string]string, error) {
		out := map[string]string{}
		rs, _ := tx.Query(ctx, `SELECT a.attname::text, pg_catalog.format_type(a.atttypid, a.atttypmod) FROM pg_catalog.pg_attribute a
            WHERE a.attrelid = $1::regclass AND a.attnum > 0 AND NOT a.attisdropped`, "archive."+table)
		var n, t string
		_, err := pgx.ForEachRow(rs, []any{&n, &t}, func() error { out[n] = t; return nil })
		return out, err
	}
	m, err := cols(mtx)
	if err != nil {
		return err
	}
	w, err := cols(wtx)
	if err != nil {
		return err
	}
	if table == "connection" {
		delete(m, "type")
	}
	for k, t := range m {
		if w[k] != t {
			return fmt.Errorf("archive.%s has %s %s in the main database, and %q in the workspace", table, k, t, w[k])
		}
	}
	for k := range w {
		if _, ok := m[k]; !ok {
			return fmt.Errorf("archive.%s has %s in the workspace, and not in the main database", table, k)
		}
	}
	return nil
}

// moveBackRow is a row of table as moveBack compares it: a party names its person by address, since each
// database has its own known_person, and a connection leaves out what the move changes.
func moveBackRow(table string) (string, string) {
	switch table {
	case "connection":
		return "to_jsonb(t) - 'token' - 'member' - 'lease_at' - 'type'", "archive.connection t WHERE t.id = $1"
	case "mail_party":
		return "to_jsonb(t) - 'person' || jsonb_build_object('person', k.email::text)",
			"archive.mail_party t JOIN archive.known_person k ON k.id = t.person WHERE t.connection = $1"
	}
	return "to_jsonb(t)", pgIdent("archive", table) + " t WHERE t.connection = $1"
}

// moveBackSum is how many rows of table connection id has in tx, and a hash of them all.
func moveBackSum(ctx context.Context, tx pgx.Tx, table string, id uuid.UUID) (string, error) {
	sel, from := moveBackRow(table)
	var n int64
	var sum string
	err := tx.QueryRow(ctx, "SELECT count(*), md5(coalesce(string_agg(("+sel+")::text, E'\\n' ORDER BY t.id), '')) FROM "+from, id).Scan(&n, &sum)
	return fmt.Sprintf("%d rows (hash %s)", n, sum), err
}

// moveBackJSON is a row as to_jsonb has it, its numbers as they are, however large.
func moveBackJSON(raw []byte) (map[string]any, error) {
	var m map[string]any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	return m, d.Decode(&m)
}

// moveBackCopy copies the rows of table of connection id from the main database into the workspace, a batch at a
// time, each a party's person the workspace's of its address, which it adds when it has none.
func moveBackCopy(ctx context.Context, mtx, wtx pgx.Tx, table string, id uuid.UUID) (int64, error) {
	sel, from := moveBackRow(table)
	ident := pgIdent("archive", table)
	var n int64
	after := uuid.Nil
	for {
		rs, _ := mtx.Query(ctx, "SELECT "+sel+" FROM "+from+" AND t.id > $2 ORDER BY t.id LIMIT $3", id, after, moveBackBatch)
		var batch []map[string]any
		var raw []byte
		if _, err := pgx.ForEachRow(rs, []any{&raw}, func() error {
			m, err := moveBackJSON(raw)
			batch = append(batch, m)
			return err
		}); err != nil {
			return n, err
		}
		if len(batch) == 0 {
			return n, nil
		}
		after = uuid.MustParse(batch[len(batch)-1]["id"].(string))
		if table == "mail_party" {
			var emails []string
			for _, m := range batch {
				emails = append(emails, m["person"].(string))
			}
			var people []byte
			if err := mtx.QueryRow(ctx, "SELECT coalesce(jsonb_agg(to_jsonb(k)), '[]') FROM archive.known_person k WHERE k.email = ANY($1)", emails).Scan(&people); err != nil {
				return n, err
			}
			// The store's people, as they are, unless the workspace knows them by another id already.
			if _, err := wtx.Exec(ctx, "INSERT INTO archive.known_person SELECT * FROM jsonb_populate_recordset(NULL::archive.known_person, $1::jsonb) ON CONFLICT DO NOTHING",
				people); err != nil {
				return n, err
			}
			known, err := generated.New(wtx).KnownPeople(ctx, emails)
			if err != nil {
				return n, err
			}
			by := map[string]uuid.UUID{}
			for _, p := range known {
				by[p.Email] = p.ID
			}
			for _, m := range batch {
				p, ok := by[m["person"].(string)]
				if !ok {
					return n, fmt.Errorf("the workspace has no known_person %q", m["person"])
				}
				m["person"] = p
			}
		}
		b, _ := json.Marshal(batch)
		tag, err := wtx.Exec(ctx, "INSERT INTO "+ident+" SELECT (jsonb_populate_record(NULL::"+ident+", r)).* FROM jsonb_array_elements($1::jsonb) r", b)
		if err != nil {
			return n, err
		}
		if tag.RowsAffected() != int64(len(batch)) {
			return n, fmt.Errorf("%d of %d rows went in", tag.RowsAffected(), len(batch))
		}
		n += int64(len(batch))
	}
}

// moveBackDrop drops the store from the main database once it holds no connection: what is left of it is only
// the people nothing names, and the due work and watches of no workspace.
func (h *Hub) moveBackDrop(ctx context.Context, conn *pgx.Conn) error {
	return pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SET LOCAL lock_timeout = '"+moveLockTimeout+"'; SET LOCAL statement_timeout = 0"); err != nil {
			return err
		}
		var left int64
		if err := tx.QueryRow(ctx, `SELECT CASE WHEN to_regclass('archive.connection') IS NULL THEN 0 ELSE (SELECT count(*) FROM archive.connection) END
    + CASE WHEN to_regclass('archive.mail_message') IS NULL THEN 0 ELSE (SELECT count(*) FROM archive.mail_message) END
    + CASE WHEN to_regclass('archive.mail_party') IS NULL THEN 0 ELSE (SELECT count(*) FROM archive.mail_party) END
    + CASE WHEN to_regclass('archive.mail_attachment') IS NULL THEN 0 ELSE (SELECT count(*) FROM archive.mail_attachment) END
    + CASE WHEN to_regclass('archive.doc_page') IS NULL THEN 0 ELSE (SELECT count(*) FROM archive.doc_page) END`).Scan(&left); err != nil {
			return err
		}
		if left > 0 {
			return fmt.Errorf("the personal store still has %d connections and rows after they moved", left)
		}
		watches, err := tx.Exec(ctx, "DELETE FROM archive.watch WHERE workspace = $1", uuid.Nil)
		if err != nil {
			return err
		}
		dues, err := tx.Exec(ctx, "DELETE FROM archive.due d WHERE d.workspace = $1 OR NOT EXISTS (SELECT FROM archive.workspace w WHERE w.id = d.workspace)", uuid.Nil)
		if err != nil {
			return err
		}
		for _, s := range []string{
			// At once, since known_person's policy names mail_party, and mail_party known_person.
			"DROP TABLE IF EXISTS archive.mail_attachment, archive.mail_party, archive.mail_message, archive.doc_page, archive.connection, archive.known_person, archive.personal_key",
			"DROP TABLE IF EXISTS archive.object",
			"DROP FUNCTION IF EXISTS archive.me(), archive.my_type(), archive.object_name()",
			"DROP DOMAIN IF EXISTS archive.email, archive.url",
		} {
			if _, err := tx.Exec(ctx, s); err != nil {
				return err
			}
		}
		for _, role := range h.moveBackRoles() {
			var there bool
			if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = $1)", role).Scan(&there); err != nil {
				return err
			}
			if !there {
				continue
			}
			r := pgIdent(role)
			for _, s := range []string{
				"REVOKE ALL ON SCHEMA archive FROM " + r,
				"DO $$ BEGIN EXECUTE format('REVOKE ALL ON DATABASE %I FROM %I', current_database(), '" + role + "'); END $$",
				"DROP ROLE " + r,
			} {
				if _, err := tx.Exec(ctx, s); err != nil {
					return fmt.Errorf("role %s: %w", role, err)
				}
			}
		}
		if left, err := h.moveBackLeft(ctx, tx); err != nil || len(left) > 0 {
			return cmpErr(err, fmt.Errorf("the main database still has %s", strings.Join(left, ", ")))
		}
		fmt.Fprintf(os.Stderr, "archive: the main database no longer has the personal store; it dropped %d watches and %d due works of no workspace with it\n",
			watches.RowsAffected(), dues.RowsAffected())
		return nil
	})
}
