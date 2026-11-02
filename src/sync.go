package main

// Syncs: Archive keeps a copy of what each connection's account has, as rows of archive.record, in the streams
// its connector declares (connect/sync.go). A sync runs only when something asks for one, and never on a
// timer, so the service can sleep: connecting, Sync now (POST /connect/<id>/sync), a push from the provider
// (push.go), or a member reading record or opening the workspace while a connection is stale (syncCatchUp).
// Asking moves the connection's asked_at on, and each stream then catches up to it: it is due until a page of
// it ends with nothing more to come after the ask, and then waits for the next.
//
// A sync runs a page at a time, as tasks like the rest of Archive's work (workspace.go): a connection's
// sync_at is when its next page is due, null once every stream has caught up, and the workspace's sync is due
// at the earliest. A page leases one connection that is due, so a connection runs one page at a time however
// many tasks or asks there are, pulls its stream most due, and in one transaction saves what it found and
// where the stream is now: a page that fails leaves the cursor where it was, and the next resumes there. A
// stream with more to come is due again its Gap later. A page that fails is the connection's error, and its
// stream backs off, a minute doubling, for syncTries pages, and then waits for the next ask. A grant the
// provider refused stops the sync until the member connects again or asks; a stream the provider cannot pull
// is skipped until the next ask. A page renews its connection's watches as they near their end (syncWatch).
//
// ARCHIVE_SYNC off keeps serve from syncing at all, though asks are kept and queued: for tests that sync by hand.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/Mulholland-Inc/archive/connect"
	"github.com/Mulholland-Inc/archive/generated"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	// syncLease is how long a leased page has before another may take its connection over; syncBudget how
	// long one task pulls pages before it queues the rest.
	syncLease  = 10 * time.Minute
	syncBudget = 2 * time.Minute
	// A failed page is retried syncRetry later, doubling, syncTries times in a row at most.
	syncRetry = time.Minute
	syncTries = 5
	// syncMaxPages is the most pages a stream reads for one ask, against a provider whose cursor goes round.
	syncMaxPages = 100_000
	// syncStale is how old a connection's last sync is before a read or an open asks for another.
	syncStale = 5 * time.Minute
)

// errRevoked is a refresh the provider refused, which stops a connection's sync.
var errRevoked = errors.New("revoked")

// syncStream is where one stream of a connection is, in its cursor. At is when its next page is due, nil
// now, while it is due; Done the ask it last caught up to; Since when the full listing under way began, which
// the records it has not read since are older than; Pages how many pages it has read for this ask; Fails how
// many in a row failed, and Error why the last did.
type syncStream struct {
	Cursor string     `json:"cursor,omitempty"`
	At     *time.Time `json:"at,omitempty"`
	Done   *time.Time `json:"done,omitempty"`
	More   bool       `json:"more,omitempty"`
	Since  *time.Time `json:"since,omitempty"`
	Skip   string     `json:"skip,omitempty"`
	Pages  int        `json:"pages,omitempty"`
	Fails  int        `json:"fails,omitempty"`
	Error  string     `json:"error,omitempty"`
}

// due says whether the stream has yet to catch up to the ask at asked.
func (v *syncStream) due(asked time.Time) bool { return v.Done == nil || v.Done.Before(asked) }

type syncState map[string]*syncStream

func syncStateOf(raw []byte) syncState {
	st := syncState{}
	json.Unmarshal(raw, &st)
	for k, v := range st {
		if v == nil {
			delete(st, k)
		}
	}
	return st
}

func (st syncState) of(name string) *syncStream {
	if st[name] == nil {
		st[name] = &syncStream{}
	}
	return st[name]
}

// syncPick is the stream most due of streams for the ask at asked: of those that have yet to catch up to it,
// the one due first, the first declared of those due at once; nil when all have.
func syncPick(streams []*connect.Stream, st syncState, asked time.Time) *connect.Stream {
	var pick *connect.Stream
	var at time.Time
	for _, s := range streams {
		v := st.of(s.Name)
		if !v.due(asked) {
			continue
		}
		var t time.Time
		if v.At != nil {
			t = *v.At
		}
		if pick == nil || t.Before(at) {
			pick, at = s, t
		}
	}
	return pick
}

// syncNext leases a connection whose sync is due and pulls a page of it, and says whether there was one.
func (a *Archive) syncNext(ctx context.Context) (bool, error) {
	var c generated.ArchiveConnection
	err := a.sys(ctx, func(ctx context.Context) (err error) {
		c, err = a.st.db(ctx).ClaimSync(ctx, time.Now().Add(syncLease))
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, a.syncPage(ctx, c)
}

// syncPage pulls the page of c's stream most due, whether it is due yet or not, saves it, and lets the lease go.
func (a *Archive) syncPage(ctx context.Context, c generated.ArchiveConnection) error {
	p, gone := a.hub.connect.Get(c.Provider)
	var streams []*connect.Stream
	if gone == nil {
		streams = p.Streams
	}
	s := syncPick(streams, syncStateOf(c.Cursor), c.AskedAt)
	var page *connect.Page
	var err error
	if s != nil {
		cur := syncStateOf(c.Cursor).of(s.Name)
		var cl *connect.Client
		if cl, err = a.connClient(ctx, c); err == nil {
			page, err = s.Pull(cl, cur.Cursor)
			if err == nil && page.More && page.Cursor == cur.Cursor {
				err = fmt.Errorf("%s said there was more, but from where this page began", p.Name)
			}
			if err == nil {
				a.syncWatch(ctx, c, p, cl)
			}
			cl.Close()
		}
	}
	var next *time.Time
	defer func() {
		// A page that leaves its connection due queues the next itself, as an ask that came while it held the
		// lease could not.
		if next != nil {
			if err := a.soon(ctx, "sync", *next); err != nil {
				fmt.Fprintf(os.Stderr, "archive: workspace %s: queueing a sync: %v\n", a.ws.ID, err)
			}
		}
	}()
	return a.sys(context.WithoutCancel(ctx), func(ctx context.Context) error {
		q := a.st.db(ctx)
		now, err2 := q.TxNow(ctx)
		if err2 != nil {
			return err2
		}
		c2, err2 := q.ConnectionLocked(ctx, c.ID)
		if errors.Is(err2, pgx.ErrNoRows) {
			return nil
		}
		if err2 != nil {
			return err2
		}
		st := syncStateOf(c2.Cursor)
		save := generated.SaveSyncParams{ID: c.ID, Error: c2.Error}
		save.Cursor, _ = json.Marshal(st)
		switch {
		case gone != nil:
			// A connector this server has no client for waits for one, and an ask.
			return q.SaveSync(ctx, save)
		case errors.Is(err, errRevoked):
			save.Revoked = true
			return q.SaveSync(ctx, save)
		}
		// With no stream picked, every stream has caught up to the ask, and only the connection is saved.
		cur, name := &syncStream{}, ""
		if s != nil {
			cur, name = st.of(s.Name), s.Name
		}
		var unsup *connect.Unsupported
		switch {
		case s == nil:
		case errors.As(err, &unsup):
			cur.Skip, cur.More, cur.Since, cur.Fails, cur.Error, cur.At, cur.Pages = unsup.Reason, false, nil, 0, "", nil, 0
			cur.Done = &c.AskedAt
		case err == nil && cur.Pages+1 >= syncMaxPages:
			err = fmt.Errorf("%s had more after %d pages; it starts again on the next sync", p.Name, syncMaxPages)
			cur.Cursor, cur.More, cur.Since, cur.Pages = "", false, nil, 0
			fallthrough
		case err != nil:
			cur.Fails++
			cur.Error = fmt.Sprintf("syncing %s: %v", name, err)
			cur.At = ptr(now.Add(syncRetry << min(cur.Fails-1, 16)))
			if cur.Fails >= syncTries {
				// It waits for the next ask, as one caught up does, with its error, and tries as many times then.
				cur.At, cur.Done, cur.Fails = nil, &c.AskedAt, 0
			}
		default:
			if err := a.syncSave(ctx, q, c.ID, name, page, cur, now); err != nil {
				return err
			}
			cur.Cursor, cur.More, cur.Skip, cur.Fails, cur.Error, cur.At = page.Cursor, page.More, "", 0, "", nil
			cur.Pages++
			if page.More {
				cur.At = ptr(now.Add(s.Gap))
			} else {
				cur.Done, cur.Pages = &c.AskedAt, 0
			}
		}
		// The connection is as its declared streams are: due at the first of those that have yet to catch up to
		// the latest ask, which may have come while this page ran; failing while any is; and synced once none
		// is due.
		var errs []string
		for _, d := range streams {
			v := st.of(d.Name)
			if v.Error != "" {
				errs = append(errs, v.Error)
			}
			if !v.due(c2.AskedAt) {
				continue
			}
			at := now
			if v.At != nil {
				at = *v.At
			}
			if save.SyncAt == nil || at.Before(*save.SyncAt) {
				save.SyncAt = &at
			}
		}
		for k := range st {
			if !slices.ContainsFunc(streams, func(d *connect.Stream) bool { return d.Name == k }) {
				delete(st, k)
			}
		}
		if save.SyncAt == nil && len(errs) == 0 {
			save.SyncedAt = &now
		}
		save.Error = strings.Join(errs, "; ")
		save.Cursor, _ = json.Marshal(st)
		next = save.SyncAt
		return q.SaveSync(ctx, save)
	})
}

// syncSave writes the records of page of stream to connection id, in the transaction at now; a page that ends
// a full listing deletes the records it began after and did not read.
func (a *Archive) syncSave(ctx context.Context, q *generated.Queries, id uuid.UUID, stream string, page *connect.Page, cur *syncStream, now time.Time) error {
	if page.Full && cur.Since == nil {
		cur.Since = &now
	}
	type rec struct {
		ID      string          `json:"id"`
		Data    json.RawMessage `json:"data"`
		Updated *time.Time      `json:"updated"`
	}
	live, at := []rec{}, map[string]int{}
	var gone []string
	for _, r := range page.Records {
		if r.ID == "" {
			continue
		}
		if r.Deleted {
			gone = append(gone, r.ID)
			if i, ok := at[r.ID]; ok {
				live[i].ID = ""
			}
			continue
		}
		b, err := syncJSON(r.Data)
		if err != nil {
			return fmt.Errorf("record %s of %s: %w", r.ID, stream, err)
		}
		v := rec{ID: r.ID, Data: b}
		if !r.Updated.IsZero() {
			v.Updated = &r.Updated
		}
		// A record twice in a page is as it was last.
		if i, ok := at[r.ID]; ok {
			live[i].ID = ""
		}
		at[r.ID] = len(live)
		live = append(live, v)
		gone = slices.DeleteFunc(gone, func(g string) bool { return g == r.ID })
	}
	live = slices.DeleteFunc(live, func(r rec) bool { return r.ID == "" })
	if len(live) > 0 {
		b, _ := json.Marshal(live)
		if err := q.SaveRecords(ctx, id, stream, b); err != nil {
			return err
		}
	}
	if len(gone) > 0 {
		if err := q.DeleteRecords(ctx, id, stream, gone); err != nil {
			return err
		}
	}
	switch {
	case page.Full && !page.More:
		err := q.DeleteUnseenRecords(ctx, id, stream, *cur.Since)
		cur.Since = nil
		return err
	case !page.Full:
		cur.Since = nil
	}
	return nil
}

func ptr[T any](v T) *T { return &v }

// syncJSON is v as JSON that jsonb takes: without the NUL characters it refuses, which mail has now and then.
func syncJSON(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil || !bytes.Contains(b, []byte(`\u0000`)) {
		return b, err
	}
	var back any
	json.Unmarshal(b, &back)
	var scrub func(v any) any
	scrub = func(v any) any {
		switch v := v.(type) {
		case string:
			return strings.ReplaceAll(v, "\x00", "")
		case []any:
			for i, e := range v {
				v[i] = scrub(e)
			}
		case map[string]any:
			out := make(map[string]any, len(v))
			for k, e := range v {
				out[strings.ReplaceAll(k, "\x00", "")] = scrub(e)
			}
			return out
		}
		return v
	}
	return json.Marshal(scrub(back))
}

// SyncNow asks for a sync of the caller's connection id, or an owner's of the archive's integration id, now,
// past any backoff, and queues it.
func (a *Archive) SyncNow(ctx context.Context, id uuid.UUID) error {
	if _, err := a.connectionMine(ctx, id); err != nil {
		return err
	}
	err := a.sys(ctx, func(ctx context.Context) error {
		q := a.st.db(ctx)
		c, err := q.ConnectionLocked(ctx, id)
		if err != nil {
			return err
		}
		st := syncStateOf(c.Cursor)
		for _, v := range st {
			v.At, v.Fails = nil, 0
		}
		cursor, _ := json.Marshal(st)
		return q.AskSync(ctx, id, cursor)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("connection %s: %w", id, ErrNotFound)
	}
	if err != nil {
		return err
	}
	return a.soon(ctx, "sync", time.Now())
}

// syncSoon queues the sync of a connection that is due now, as one just connected.
func (a *Archive) syncSoon(ctx context.Context) {
	if err := a.soon(ctx, "sync", time.Now()); err != nil {
		fmt.Fprintf(os.Stderr, "archive: workspace %s: queueing a sync: %v\n", a.ws.ID, err)
	}
}

// syncWant asks for a sync of connection id, as a push does: it keeps a time it is already due at, or backing
// off to, and leaves a connection whose grant was refused as it is. It says whether there is the connection.
func (a *Archive) syncWant(ctx context.Context, id uuid.UUID) (bool, error) {
	ctx = context.WithoutCancel(ctx)
	var found bool
	err := a.sys(ctx, func(ctx context.Context) error {
		q := a.st.db(ctx)
		if _, err := q.Connection(ctx, id); err != nil {
			return err
		}
		found = true
		_, err := q.WantSync(ctx, id)
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return found, nil
	}
	if err != nil {
		return found, err
	}
	return true, a.soon(ctx, "sync", time.Now())
}

// syncCatchUp asks for a sync of each of member's connections whose last is over syncStale old and that is not
// syncing, as reading record or opening the workspace does. It does not wait for them: the read reads what
// there is, and the next sees what they bring.
func (a *Archive) syncCatchUp(ctx context.Context, member uuid.UUID) {
	ctx = context.WithValue(context.WithoutCancel(ctx), storeTxKey{}, nil)
	var asked []uuid.UUID
	err := a.sys(ctx, func(ctx context.Context) (err error) {
		asked, err = a.st.db(ctx).CatchUpSyncs(ctx, member, time.Now().Add(-syncStale))
		return err
	})
	if err == nil && len(asked) > 0 {
		err = a.soon(ctx, "sync", time.Now())
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "archive: workspace %s: catching up syncs: %v\n", a.ws.ID, err)
	}
}

// streamTypes is the TypeScript the run tool's docs end with: a Streams interface of what the caller's
// connections sync into record, by "<connector>.<stream>", each with what its data holds.
func (a *Archive) streamTypes(ctx context.Context) string {
	var b strings.Builder
	var conns []generated.ArchiveConnection
	if ac := ActorFrom(ctx); ac != nil {
		conns, _ = a.memberConnections(ctx, ac.member)
	}
	b.WriteString("\n" + runDoc("", `What record holds of the accounts you connected, by "<connector>.<stream>": its stream column is the part after the dot, and its connection column the connection's id (see Connected accounts in the header)`) + "interface Streams {\n")
	done := map[string]bool{}
	for _, c := range conns {
		p, err := a.hub.connector(c.Provider)
		if err != nil || done[p.ID] {
			continue
		}
		done[p.ID] = true
		for _, s := range p.Streams {
			b.WriteString(runDoc("    ", p.Name+": "+s.Doc))
			fmt.Fprintf(&b, "    %q: any;\n", p.ID+"."+s.Name)
		}
	}
	b.WriteString("}\n")
	return b.String()
}
