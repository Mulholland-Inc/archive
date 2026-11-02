package main

// Pushes: POST /push/<connector> is where a connector's provider tells Archive that an account changed
// (connect/push.go), which asks for that connection's sync and nothing more: the sync reads what changed from
// its cursor. A push names a watch by its key; archive.watch, in the main database, says which workspace and
// connection each is, and holds the secret its pushes prove themselves with. A push for a watch Archive no
// longer has, as of an account disconnected since, is answered as heard and starts nothing, so the provider
// does not send it again; one not signed as its provider signs is refused.
//
// A page of a connection's sync keeps its watches (syncWatch): it asks the provider for them the first time,
// and renews one once it nears its end. Nothing renews them on a timer: a watch of an account that nothing
// syncs lapses, the pushes it would have sent are missed, and the next sync, whatever asks for it, reads all
// they would have said from its cursor and watches again. Disconnecting ends them.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/Mulholland-Inc/archive/connect"
	"github.com/Mulholland-Inc/archive/generated"
	"github.com/google/uuid"
)

// pushMax is the most of a push Archive reads.
const pushMax = 1 << 20

func (h *Hub) pushHTTP(w http.ResponseWriter, r *http.Request) {
	p, err := h.connector(r.PathValue("id"))
	var push *connect.Push
	if err == nil {
		push, _ = p.PushOf()
	}
	if push == nil {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()
	body, err := io.ReadAll(io.LimitReader(r.Body, pushMax+1))
	switch {
	case err != nil:
		httpWrite(w, 0, nil, err)
		return
	case len(body) > pushMax:
		httpWrite(w, 0, nil, fmt.Errorf("a push of over %d bytes: %w", pushMax, httpErrBadRequest))
		return
	case push.OIDC && h.tasks == nil:
		http.NotFound(w, r)
		return
	case push.OIDC:
		if err := h.googleCaller(r, "/push/"+p.ID); err != nil {
			httpWrite(w, 0, nil, err)
			return
		}
	}
	heard, err := push.Hear(p, r, h.api+r.URL.RequestURI(), body, func(key string) string { return h.watchSecret(ctx, p.ID, key) })
	if err != nil {
		fmt.Fprintf(os.Stderr, "archive: a push to %s: %v\n", p.ID, err)
		if errors.Is(err, connect.ErrForged) {
			err = fmt.Errorf("%v: %w", err, ErrUnauthorized)
		} else {
			err = fmt.Errorf("%v: %w", err, httpErrBadRequest)
		}
		httpWrite(w, 0, nil, err)
		return
	}
	for key, secret := range heard.Keep {
		ws, err1 := uuid.Parse(r.URL.Query().Get("w"))
		conn, err2 := uuid.Parse(r.URL.Query().Get("c"))
		if err1 != nil || err2 != nil || !h.pushConnection(ctx, ws, conn, p.ID) {
			httpWrite(w, 0, nil, fmt.Errorf("a handshake for no connection of %s: %w", p.Name, ErrUnauthorized))
			return
		}
		if _, err := h.q().KeepWatchSecret(ctx, generated.KeepWatchSecretParams{Provider: p.ID, Key: key, Workspace: ws, Connection: conn,
			Secret: connect.SealWatch(h.aead, p.ID, key, secret)}); err != nil {
			httpWrite(w, 0, nil, err)
			return
		}
	}
	for _, key := range heard.Keys {
		rows, err := h.q().WatchesFor(ctx, p.ID, key)
		for _, row := range rows {
			err = errors.Join(err, h.pushAsk(ctx, row.Workspace, row.Connection))
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "archive: a push to %s for %s: %v\n", p.ID, key, err)
			httpWrite(w, 0, nil, err)
			return
		}
	}
	for k, v := range heard.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(cmpOr(heard.Status, http.StatusAccepted))
	io.WriteString(w, heard.Body)
}

func cmpOr(a, b int) int {
	if a != 0 {
		return a
	}
	return b
}

// pushAsk asks for the sync of connection conn of workspace ws, which a push named; a connection or workspace
// gone since takes its watches with it.
func (h *Hub) pushAsk(ctx context.Context, ws, conn uuid.UUID) error {
	a, release, err := h.acquire(ctx, ws.String())
	if errors.Is(err, ErrNotFound) {
		return h.q().DropWatches(context.WithoutCancel(ctx), conn, []string{})
	}
	if err != nil {
		return err
	}
	defer release()
	found, err := a.syncWant(ctx, conn)
	if err == nil && !found {
		return h.q().DropWatches(context.WithoutCancel(ctx), conn, []string{})
	}
	return err
}

// pushConnection says whether workspace ws has connection conn, to connector id: a handshake is kept only
// for one.
func (h *Hub) pushConnection(ctx context.Context, ws, conn uuid.UUID, id string) bool {
	a, release, err := h.acquire(ctx, ws.String())
	if err != nil {
		return false
	}
	defer release()
	var c generated.ArchiveConnection
	err = a.st.System(ctx, func(ctx context.Context) (err error) {
		c, err = a.st.db(ctx).Connection(ctx, conn)
		return err
	})
	return err == nil && c.Provider == id
}

// watchSecret is the secret Archive keeps for watch key of connector id, or "" when it has none.
func (h *Hub) watchSecret(ctx context.Context, id, key string) string {
	sealed, err := h.q().WatchSecret(ctx, id, key)
	if err != nil {
		return ""
	}
	s, err := connect.OpenWatch(h.aead, id, key, sealed)
	if err != nil {
		return ""
	}
	return s
}

// syncWatch asks c's provider, through cl, to push what changes, when Archive has no watch of it yet or one is
// within its Push's Renew of ending. Only one page of a connection at a time does, and a failure is only logged:
// the account still syncs whenever something asks. A provider pushes only to an https API.
func (a *Archive) syncWatch(ctx context.Context, c generated.ArchiveConnection, p *connect.Provider, cl *connect.Client) {
	push, target := p.PushOf()
	if push == nil || push.Watch == nil || push.Env != "" && target == "" || !strings.HasPrefix(a.hub.api, "https://") {
		return
	}
	if err := a.watch(context.WithoutCancel(ctx), c, p, push, target, cl); err != nil {
		fmt.Fprintf(os.Stderr, "archive: workspace %s: watching %s %s: %v\n", a.ws.ID, p.Name, c.Account, err)
	}
}

func (a *Archive) watch(ctx context.Context, c generated.ArchiveConnection, p *connect.Provider, push *connect.Push, target string, cl *connect.Client) error {
	tx, err := a.hub.main.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := generated.New(tx)
	// The lock is the transaction's: a page that finds another renewing leaves it be.
	if ok, err := q.TryLock(ctx, "watch "+c.ID.String()); err != nil || !ok {
		return err
	}
	rows, err := q.ConnectionWatches(ctx, c.ID)
	if err != nil {
		return err
	}
	if len(rows) > 0 && !slices.ContainsFunc(rows, func(w generated.ArchiveWatch) bool {
		return w.ExpiresAt != nil && time.Until(*w.ExpiresAt) < push.Renew
	}) {
		return nil
	}
	old := make([]connect.Watch, len(rows))
	for i, w := range rows {
		old[i] = connect.Watch{Key: w.Key, Resource: w.Resource}
		if w.ExpiresAt != nil {
			old[i].Expires = *w.ExpiresAt
		}
		if w.Secret != nil {
			old[i].Secret, _ = connect.OpenWatch(a.hub.aead, p.ID, w.Key, w.Secret)
		}
	}
	hook := connect.Hook{ID: c.ID.String(), Account: c.Account, Target: target, URL: func(key string) string {
		return a.hub.api + "/push/" + p.ID + "?" + url.Values{"w": {a.ws.ID.String()}, "c": {c.ID.String()}, "k": {key}}.Encode()
	}}
	got, werr := push.Watch(cl, hook, old)
	// What it made is kept even when it then failed, so that it can be ended; what it no longer has is let go
	// only when it did not.
	keys := []string{}
	for _, w := range got {
		save := generated.SaveWatchParams{Provider: p.ID, Key: w.Key, Workspace: a.ws.ID, Connection: c.ID, Resource: w.Resource}
		if w.Secret != "" {
			save.Secret = connect.SealWatch(a.hub.aead, p.ID, w.Key, w.Secret)
		}
		if !w.Expires.IsZero() {
			save.ExpiresAt = &w.Expires
		}
		if err := q.SaveWatch(ctx, save); err != nil {
			return err
		}
		keys = append(keys, w.Key)
	}
	if werr == nil {
		if err := q.DropWatches(ctx, c.ID, keys); err != nil {
			return err
		}
	}
	return errors.Join(werr, tx.Commit(ctx))
}

// unwatch ends the watches of connection c, as it disconnects, through cl when it has one, and forgets them.
func (a *Archive) unwatch(ctx context.Context, c generated.ArchiveConnection, p *connect.Provider, cl *connect.Client) error {
	rows, err := a.hub.q().ConnectionWatches(ctx, c.ID)
	if err != nil || len(rows) == 0 {
		return err
	}
	var errs []error
	if push, _ := p.PushOf(); push != nil && push.Unwatch != nil && cl != nil {
		for _, w := range rows {
			errs = append(errs, push.Unwatch(cl, connect.Watch{Key: w.Key, Resource: w.Resource}))
		}
	}
	return errors.Join(append(errs, a.hub.q().DropWatches(ctx, c.ID, []string{}))...)
}

// syncOnRead catches up the caller's syncs, as reading record does.
func (a *Archive) syncOnRead(ctx context.Context) {
	if ac := ActorFrom(ctx); ac != nil && ac.member != uuid.Nil {
		a.syncCatchUp(ctx, ac.member)
	}
}
