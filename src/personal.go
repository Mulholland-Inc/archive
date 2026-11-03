package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Mulholland-Inc/archive/connect"
	"github.com/Mulholland-Inc/archive/generated"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// The personal store: a person's own connections, like their Gmail, Outlook, Google Drive and Notion, and what
// they sync, kept once in the main database, not in any workspace, so connecting in one workspace connects them
// in every workspace the person is in, and disconnecting in one disconnects them in all. personal.sql is its
// schema. The integrations, a workspace's own accounts like its QuickBooks or Gusto, stay in the workspace.
//
// Hub.self is the store as code sees it: an Archive whose database is the main one, and whose workspace is the
// nil one, which no workspace is. Its connections sync, refresh, watch and act as a workspace's do
// (connection.go), with its syncs due as the nil workspace's work, and its watches the nil workspace's.
//
// A person reads it from whichever workspace they are in, as themselves (personal.sql): list and
// get of mail_message, mail_party, mail_attachment and doc_page read the store; of connection and known_person,
// both the store and the workspace, whose integrations have their own; and archive.query reads the store when
// it names one of the store's own tables, and the workspace otherwise. Postgres keeps them apart: a query of
// both cannot be, and fails saying so.

// mainPrefix starts the names of the main database's roles, the personal store's: the same for every process of
// one Archive, and another for each main database on a server, since roles are the server's.
func (h *Hub) mainPrefix() string {
	sum := sha256.Sum256([]byte(h.pg.Database))
	return "main_" + hex.EncodeToString(sum[:5])
}

const mainRoles = `DO $$ BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '{p}_authenticator') THEN
        CREATE ROLE "{p}_authenticator" LOGIN NOINHERIT;
    END IF;
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '{p}_personal') THEN
        CREATE ROLE "{p}_personal" NOLOGIN;
    END IF;
    GRANT "{p}_personal" TO "{p}_authenticator";
    IF NOT has_database_privilege('{p}_authenticator', current_database(), 'CONNECT') THEN
        EXECUTE format('GRANT CONNECT ON DATABASE %I TO "{p}_authenticator"', current_database());
    END IF;
END $$`

// mainPassword is the password of the personal store's authenticator, from the server's key.
func (h *Hub) mainPassword() string { return authPassword(secretDerive(h.key, "main")) }

// mainRolesMake makes the personal store's roles and sets its authenticator's password, before main.sql and
// personal.sql grant them anything; the caller holds mainLock.
func (h *Hub) mainRolesMake(ctx context.Context, q generated.DBTX) error {
	if _, err := q.Exec(ctx, strings.ReplaceAll(mainRoles, "{p}", h.mainPrefix())); err != nil {
		return fmt.Errorf("the main database's roles: %w", err)
	}
	_, err := q.Exec(ctx, "ALTER ROLE "+pgIdent(h.mainPrefix()+"_authenticator")+" PASSWORD '"+h.mainPassword()+"'")
	return err
}

// openSelf opens the personal store once the main database is personal.sql, keeps its personal_key, and asks for
// the syncs that are due.
func (h *Hub) openSelf(ctx context.Context) error {
	p := h.mainPrefix()
	app, err := storePool(ctx, h.pg, storeAppConns, nil, func(c *pgx.ConnConfig) { c.User, c.Password = p+"_authenticator", h.mainPassword() })
	if err != nil {
		return err
	}
	h.self = &Archive{hub: h, ws: generated.ArchiveWorkspace{ID: uuid.Nil, Name: "personal"}, st: &pgStore{app: app, sys: h.main, cfg: h.pg.Copy(), prefix: p},
		hints: h.hints, prefix: p, aead: h.aead, personal: true, slots: make(chan struct{}, workspaceRuns)}
	if _, err := h.main.Exec(ctx, "INSERT INTO archive.personal_key (key) VALUES ($1) ON CONFLICT (id) DO UPDATE SET key = EXCLUDED.key WHERE archive.personal_key.key <> EXCLUDED.key",
		h.selfKey()); err != nil {
		return err
	}
	next, err := h.self.nextDue(ctx, "sync")
	if err == nil && next != nil {
		err = h.self.soon(ctx, "sync", *next)
	}
	return err
}

// archiveOf is the workspace ws, or the personal store for the nil one, as work and pushes name them.
func (h *Hub) archiveOf(ctx context.Context, ws uuid.UUID) (*Archive, func(), error) {
	if ws == uuid.Nil {
		return h.self, func() {}, nil
	}
	return h.acquire(ctx, ws.String())
}

// selfKey is personal_key, the secret archive.person is signed with.
func (h *Hub) selfKey() string { return hex.EncodeToString(secretDerive(h.key, "personal")) }

// selfWho is archive.person for a read of person's own: their id, signed.
func (h *Hub) selfWho(person uuid.UUID) string {
	sum := sha256.Sum256([]byte(h.selfKey() + person.String()))
	return person.String() + "." + hex.EncodeToString(sum[:])
}

// connPersonal says whether provider is a personal connector, whose connections are in the personal store.
func connPersonal(provider string) bool {
	c := connect.Get(provider)
	return c != nil && !c.Integration
}

// personalTables are the tables only the personal store has rows of; personalBoth those the store and every
// workspace have rows of, the store a person's own and a workspace its integrations'.
var (
	personalTables = []string{"mail_message", "mail_party", "mail_attachment", "doc_page"}
	personalBoth   = []string{"connection", "known_person"}
)

// personalWhere is where the rows of tb are: "self" in the personal store, "both" there and in the workspace,
// or "" in the workspace alone.
func personalWhere(tb *Table) string {
	switch {
	case tb.Schema != "archive":
		return ""
	case slices.Contains(personalTables, tb.Name):
		return "self"
	case slices.Contains(personalBoth, tb.Name):
		return "both"
	}
	return ""
}

// personalQueryRe finds a query that names one of the personal store's own tables.
var personalQueryRe = regexp.MustCompile(`(?i)\b(` + strings.Join(personalTables, "|") + `)\b`)

// selfDo runs fn as the caller in the personal store, as coreDo runs it as their member in the workspace.
func selfDo[T any](a *Archive, ctx context.Context, fn func(context.Context) (T, error)) (T, error) {
	var out T
	ac, err := signedIn(ctx)
	if err != nil {
		return out, err
	}
	s, role, who := a.hub.self.st, a.hub.mainPrefix()+"_personal", a.hub.selfWho(ac.UserID)
	if t, ok := ctx.Value(storeTxKey{}).(storeTx); ok && t.role == role+" "+who {
		return fn(ctx)
	}
	after := &[]func(){}
	err = storeCommitted(after, storeErr(pgx.BeginFunc(ctx, s.app, func(tx pgx.Tx) error {
		q := generated.New(tx)
		if err := q.SetLocal(ctx, "role", role); err != nil {
			return err
		}
		if err := q.SetLocal(ctx, "archive.person", who); err != nil {
			return err
		}
		var err error
		out, err = fn(context.WithValue(ctx, storeTxKey{}, storeTx{tx, role + " " + who, after, &schemaTx{}}))
		return err
	})))
	return out, err
}

// personalMixed is the error of a query of the personal store that names a table only a workspace has.
func personalMixed(err error) error {
	var pe *pgconn.PgError
	if !errors.As(err, &pe) || pe.Code != "42P01" && (pe.Code != "42501" || !strings.HasPrefix(pe.Message, "permission denied for ")) {
		return nil
	}
	return &actionError{kind: errActionInvalid, msg: fmt.Sprintf("archive.query: %s: %s are your own, kept once for every archive you are in "+
		"and apart from this archive's tables, so one query reads either them or this archive's tables, never both; query each on its own "+
		"and join them in the script", pe.Message, strings.Join(personalTables, ", "))}
}

// selfMoved tells every workspace person is in that tables of the personal store moved, so their app re-reads
// them wherever it is open, as a workspace's own writes tell it (live.go).
func (h *Hub) selfMoved(ctx context.Context, person uuid.UUID, tables ...string) {
	if !h.live.on() {
		return
	}
	u, err := h.q().UserByID(context.WithoutCancel(ctx), person)
	if err != nil {
		return
	}
	for _, ws := range u.Workspaces {
		for _, t := range tables {
			h.live.moved(ws.String(), liveChange{Table: "archive." + t})
		}
	}
}

// Moving personal connections out of the workspaces. Before the personal store, each workspace kept its
// members' own connections and what they synced, so a person who connected their Gmail in two workspaces had it
// twice. personalMove moves a workspace's into the personal store, at startup for every workspace and again
// whenever one opens: each connection becomes its person's one connection to its account, the one they already
// have when another workspace's moved first, and what it synced its rows. Of two connections to one account, the
// one that changed last keeps its token and its sync's state; their rows are one, each by its stream and its id
// there.
//
// Postgres moves nothing between databases, so Go copies, as moveFirst does: in one transaction on the workspace
// that holds the tables locked against writes, it copies each connection and its rows into the personal store in
// one transaction on the main database, and checks, before that commits, that every row is there, as itself or
// as the row of the same thing the store already had. Only once the main database has committed does the
// workspace delete what moved, and commit. Anything that does not match, a member with no user, a token that
// does not open, a table whose columns are not the store's, stops it, and both are as they were. A failure
// after the main database committed leaves the workspace as it was too, and the next try finds the copy there
// and only deletes. A workspace with no personal connections is left alone, so starting again changes nothing.

// personalBatch is how many rows of a table personalMove copies at a time.
const personalBatch = 200

// personalMoveAll moves every workspace's personal connections, saying what failed, which stays where it was.
func (h *Hub) personalMoveAll(ctx context.Context) {
	all, err := h.q().AllWorkspaces(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "archive: moving personal connections: %v\n", err)
		return
	}
	for _, ws := range all {
		h.personalMoveLogged(ctx, ws)
	}
}

func (h *Hub) personalMoveLogged(ctx context.Context, ws generated.ArchiveWorkspace) {
	if err := h.personalMove(ctx, ws); err != nil {
		fmt.Fprintf(os.Stderr, "archive: workspace %s: its members' own connections stay in it, as they were, until they move: %v\n", ws.ID, err)
	}
}

// personalMoved is a connection personalMove moved: its id in the workspace and the store, and whose it is.
type personalMoved struct {
	from, to, person uuid.UUID
}

func (h *Hub) personalMove(ctx context.Context, ws generated.ArchiveWorkspace) error {
	bg := context.WithoutCancel(ctx)
	var providers []string
	for _, c := range connect.All() {
		if !c.Integration {
			providers = append(providers, c.ID)
		}
	}
	cfg := h.pg.Copy()
	cfg.Database = h.database(ws.ID.String())
	w, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return err
	}
	defer w.Close(bg)
	var some bool
	if err := w.QueryRow(ctx, "SELECT to_regclass('archive.connection') IS NOT NULL AND EXISTS (SELECT FROM archive.connection WHERE NOT integration AND provider = ANY($1))",
		providers).Scan(&some); err != nil && !storeUndefined(err) {
		return err
	}
	if !some {
		return nil
	}
	key, err := h.unwrap(ws)
	if err != nil {
		return err
	}
	waead, err := secretAEAD(key)
	if err != nil {
		return err
	}
	start := time.Now()
	wtx, err := w.Begin(ctx)
	if err != nil {
		return err
	}
	defer wtx.Rollback(bg)
	tables := append([]string{"connection", "known_person"}, personalTables...)
	locks := make([]string, len(tables))
	for i, t := range tables {
		locks[i] = pgIdent("archive", t)
	}
	if _, err := wtx.Exec(ctx, "SET LOCAL lock_timeout = '"+moveLockTimeout+"'; SET LOCAL statement_timeout = 0; LOCK TABLE "+strings.Join(locks, ", ")+" IN EXCLUSIVE MODE"); err != nil {
		return fmt.Errorf("locking its tables: %w", err)
	}
	mtx, err := h.main.Begin(ctx)
	if err != nil {
		return err
	}
	defer mtx.Rollback(bg)
	if _, err := mtx.Exec(ctx, "SET LOCAL statement_timeout = 0; SELECT pg_advisory_xact_lock("+fmt.Sprint(mainLock)+")"); err != nil {
		return err
	}
	for _, t := range tables {
		if err := personalSameCols(ctx, wtx, mtx, t); err != nil {
			return err
		}
	}
	type row struct {
		id, member uuid.UUID
		provider   string
		account    string
		email      string
		token      []byte
		updated    time.Time
		data       map[string]any
	}
	var conns []row
	rs, _ := wtx.Query(ctx, `SELECT c.id, c.member, c.provider, c.account, coalesce(m.email::text, ''), c.token, c.updated_at, to_jsonb(c) - 'token'
        FROM archive.connection c LEFT JOIN archive.member m ON m.id = c.member WHERE NOT c.integration AND c.provider = ANY($1) ORDER BY c.id`, providers)
	var r row
	var raw []byte
	if _, err := pgx.ForEachRow(rs, []any{&r.id, &r.member, &r.provider, &r.account, &r.email, &r.token, &r.updated, &raw}, func() error {
		c := r
		if err := json.Unmarshal(raw, &c.data); err != nil {
			return err
		}
		conns = append(conns, c)
		return nil
	}); err != nil {
		return err
	}
	mq, wq := generated.New(mtx), generated.New(wtx)
	var moved []personalMoved
	var copied int64
	for _, c := range conns {
		u, err := mq.UserByEmail(ctx, c.email)
		if err != nil {
			return fmt.Errorf("connection %s is of member %s %q, who is no one who signs in: %w", c.id, c.member, c.email, err)
		}
		rt, err := connect.OpenToken(waead, c.id, c.token)
		if err != nil {
			return err
		}
		// The store's connection to the account, if it has one; one of this id is that one, as a try that copied
		// it and did not delete it left it.
		var to uuid.UUID
		var at time.Time
		err = mtx.QueryRow(ctx, "SELECT id, updated_at FROM archive.connection WHERE member = $1 AND provider = $2 AND account = $3 FOR UPDATE",
			u.ID, c.provider, c.account).Scan(&to, &at)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			var taken bool
			if err := mtx.QueryRow(ctx, "SELECT EXISTS (SELECT FROM archive.connection WHERE id = $1)", c.id).Scan(&taken); err != nil {
				return err
			}
			if taken {
				return fmt.Errorf("connection %s is in the personal store already, as another account than %s %s", c.id, c.provider, c.account)
			}
			c.data["member"], c.data["token"], c.data["lease_at"] = u.ID, `\x`+hex.EncodeToString(connect.SealToken(h.aead, c.id, rt)), nil
			if _, err := mtx.Exec(ctx, "INSERT INTO archive.connection SELECT (jsonb_populate_record(NULL::archive.connection, $1)).*", c.data); err != nil {
				return fmt.Errorf("connection %s: %w", c.id, err)
			}
			to = c.id
		case err != nil:
			return err
		case c.updated.After(at):
			// The newer of the two keeps its token and where its sync is.
			c.data["token"] = `\x` + hex.EncodeToString(connect.SealToken(h.aead, to, rt))
			if _, err := mtx.Exec(ctx, `UPDATE archive.connection t SET token = n.token, scopes = n.scopes, meta = n.meta, error = n.error, revoked = n.revoked,
                cursor = n.cursor, sync_at = n.sync_at, synced_at = n.synced_at, asked_at = n.asked_at, lease_at = NULL, updated_at = n.updated_at
                FROM jsonb_populate_record(NULL::archive.connection, $2) n WHERE t.id = $1`, to, c.data); err != nil {
				return fmt.Errorf("connection %s: %w", c.id, err)
			}
		}
		for _, t := range []string{"mail_message", "doc_page"} {
			n, err := personalCopy(ctx, wtx, mtx, t, c.id, to)
			if err != nil {
				return err
			}
			copied += n
		}
		for _, t := range []string{"mail_party", "mail_attachment"} {
			n, err := personalCopy(ctx, wtx, mtx, t, c.id, to)
			if err != nil {
				return err
			}
			copied += n
		}
		// Its watches are the store's now, or go, when the store had the account already; that one watches it.
		if to == c.id {
			_, err = mtx.Exec(ctx, "UPDATE archive.watch SET workspace = $1 WHERE connection = $2 AND workspace = $3", uuid.Nil, c.id, ws.ID)
		} else {
			_, err = mtx.Exec(ctx, "DELETE FROM archive.watch WHERE connection = $1 AND workspace = $2", c.id, ws.ID)
		}
		if err != nil {
			return err
		}
		moved = append(moved, personalMoved{from: c.id, to: to, person: u.ID})
	}
	if err := mq.DeleteUnknownPeople(ctx); err != nil {
		return err
	}
	if err := mtx.Commit(ctx); err != nil {
		return err
	}
	ids := make([]uuid.UUID, len(moved))
	for i, m := range moved {
		ids[i] = m.from
	}
	if _, err := wtx.Exec(ctx, "DELETE FROM archive.connection WHERE id = ANY($1)", ids); err != nil {
		return err
	}
	if err := wq.DeleteUnknown(ctx); err != nil {
		return err
	}
	if err := wtx.Commit(ctx); err != nil {
		return err
	}
	for _, m := range moved {
		h.selfMoved(bg, m.person, append([]string{"connection"}, personalTables...)...)
	}
	fmt.Fprintf(os.Stderr, "archive: workspace %s: moved %d of its members' own connections and %d rows they synced into the personal store, checked, in %s\n",
		ws.ID, len(moved), copied, time.Since(start).Round(time.Millisecond))
	return nil
}

// personalSameCols checks that table has the same columns, by name and type, in the workspace and the store.
func personalSameCols(ctx context.Context, wtx, mtx pgx.Tx, table string) error {
	cols := func(tx pgx.Tx) (map[string]string, error) {
		out := map[string]string{}
		rs, _ := tx.Query(ctx, `SELECT a.attname::text, format_type(a.atttypid, a.atttypmod) FROM pg_catalog.pg_attribute a
            WHERE a.attrelid = $1::regclass AND a.attnum > 0 AND NOT a.attisdropped`, "archive."+table)
		var n, t string
		_, err := pgx.ForEachRow(rs, []any{&n, &t}, func() error { out[n] = t; return nil })
		return out, err
	}
	a, err := cols(wtx)
	if err != nil {
		return err
	}
	b, err := cols(mtx)
	if err != nil {
		return err
	}
	for k, t := range a {
		if b[k] != t {
			return fmt.Errorf("archive.%s has %s %s in the workspace, and %q in the personal store", table, k, t, b[k])
		}
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			return fmt.Errorf("archive.%s has %s in the personal store, and not in the workspace", table, k)
		}
	}
	return nil
}

// personalCopy copies the rows of table of the workspace's connection from into the store's connection to, a
// batch at a time, and checks each batch is there: a row of a table a stream lands in by its stream and id
// there, and a row that belongs to one, like a message's party, by its own id when the row it belongs to was
// copied as itself; when the store had that one already, as the same thing of another workspace, its own
// parties are the ones it keeps. A party's person is the store's of the same address.
func personalCopy(ctx context.Context, wtx, mtx pgx.Tx, table string, from, to uuid.UUID) (int64, error) {
	t := syncSchema[table]
	sel := "SELECT to_jsonb(t) FROM archive." + table + " t WHERE t.connection = $1 AND t.id > $2 ORDER BY t.id LIMIT $3"
	if table == "mail_party" {
		sel = "SELECT to_jsonb(t) || jsonb_build_object('person_email', k.email) FROM archive.mail_party t JOIN archive.known_person k ON k.id = t.person" +
			" WHERE t.connection = $1 AND t.id > $2 ORDER BY t.id LIMIT $3"
	}
	ident := pgIdent("archive", table)
	var n int64
	after := uuid.Nil
	for {
		rs, _ := wtx.Query(ctx, sel, from, after, personalBatch)
		var batch []map[string]any
		var raw []byte
		if _, err := pgx.ForEachRow(rs, []any{&raw}, func() error {
			var m map[string]any
			if err := json.Unmarshal(raw, &m); err != nil {
				return err
			}
			batch = append(batch, m)
			return nil
		}); err != nil {
			return n, fmt.Errorf("%s: %w", table, err)
		}
		if len(batch) == 0 {
			return n, nil
		}
		after = uuid.MustParse(batch[len(batch)-1]["id"].(string))
		if table == "mail_party" {
			var emails []string
			for _, m := range batch {
				emails = append(emails, m["person_email"].(string))
			}
			people, err := generated.New(mtx).KnownPeople(ctx, emails)
			if err != nil {
				return n, err
			}
			by := map[string]uuid.UUID{}
			for _, p := range people {
				by[p.Email] = p.ID
			}
			for _, m := range batch {
				m["person"] = by[m["person_email"].(string)]
			}
		}
		for _, m := range batch {
			m["connection"] = to
		}
		b, _ := json.Marshal(batch)
		var missing int64
		if t.parent == "" {
			if _, err := mtx.Exec(ctx, "INSERT INTO "+ident+" SELECT (jsonb_populate_record(NULL::"+ident+", r)).* FROM jsonb_array_elements($1::jsonb) r ON CONFLICT DO NOTHING", b); err != nil {
				return n, fmt.Errorf("%s: %w", table, err)
			}
			err := mtx.QueryRow(ctx, "SELECT count(*) FROM jsonb_array_elements($1::jsonb) r WHERE NOT EXISTS (SELECT FROM "+ident+
				" x WHERE x.connection = $2 AND x.stream = r->>'stream' AND x.source_id = r->>'source_id')", b, to).Scan(&missing)
			if err != nil {
				return n, err
			}
		} else {
			of := pgIdent(t.of)
			if _, err := mtx.Exec(ctx, "INSERT INTO "+ident+" SELECT (jsonb_populate_record(NULL::"+ident+", r)).* FROM jsonb_array_elements($1::jsonb) r"+
				" WHERE EXISTS (SELECT FROM archive."+t.parent+" p WHERE p.id = (r->>'"+t.of+"')::uuid AND p.connection = $2) ON CONFLICT DO NOTHING", b, to); err != nil {
				return n, fmt.Errorf("%s: %w", table, err)
			}
			err := mtx.QueryRow(ctx, "SELECT count(*) FROM jsonb_array_elements($1::jsonb) r WHERE EXISTS (SELECT FROM archive."+t.parent+
				" p WHERE p.id = (r->>'"+t.of+"')::uuid AND p.connection = $2) AND NOT EXISTS (SELECT FROM "+ident+" x WHERE x.id = (r->>'id')::uuid AND x."+of+
				" = (r->>'"+t.of+"')::uuid AND x.connection = $2)", b, to).Scan(&missing)
			if err != nil {
				return n, err
			}
		}
		if missing > 0 {
			return n, fmt.Errorf("%d rows of %s of connection %s are not in the personal store after copying them", missing, table, from)
		}
		n += int64(len(batch))
	}
}
