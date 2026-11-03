package main

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Mulholland-Inc/archive/connect"
	"github.com/Mulholland-Inc/archive/generated"
	"github.com/dop251/goja"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/oauth2"
)

// Connections: the rows of archive.connection, each one of a member's own accounts that a connector of the
// connect package connected. This is the routes and the rows; connect has the connectors and OAuth.
//
// GET /connect/<id> sends the browser to the provider with a state naming the workspace, the user and their
// member, sealed with ARCHIVE_SECRET_KEY; for an MCP connector it first registers Archive's own client at its
// authorization server, once for every workspace, and keeps it in archive.oauth_client of the main database.
// Its callback, GET /connect/<id>/callback, checks the same person is signed in, keeps the grant's refresh
// token sealed with the workspace key, and sends the browser back to the connector in the app. Only Go writes
// the rows; a member reads their own. connectionAccess is how code that uses an account (archive.action,
// below) gets an access token for it.

func (h *Hub) connector(id string) (*connect.Provider, error) {
	p, err := h.connect.Get(id)
	if err != nil {
		return nil, fmt.Errorf("%v: %w", err, ErrNotFound)
	}
	return p, nil
}

// provider is the connector id with its OAuth client: an API connector's from its variable, and an MCP
// connector's from archive.oauth_client. With register, an MCP connector without one, or without one that
// still fits, registers one now; a process that registered at the same time keeps the one that was first.
func (h *Hub) provider(ctx context.Context, id string, register bool) (*connect.Provider, error) {
	p, err := h.connector(id)
	if err != nil || p.MCP == "" {
		return p, err
	}
	redirect := h.connectRedirect(id)
	kept := func() ([]byte, *connect.Registration, error) {
		sealed, err := h.q().OAuthClient(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, nil
		}
		if err != nil {
			return nil, nil, err
		}
		r, err := connect.OpenClient(h.aead, id, sealed)
		return sealed, r, err
	}
	sealed, r, err := kept()
	switch {
	case err != nil:
		return nil, err
	case r != nil && (!register || p.Fits(r, redirect)):
		return p.Registered(r), nil
	case !register:
		return nil, fmt.Errorf("%s has no client on this server; connect it again: %w", p.Name, ErrUnauthorized)
	}
	fresh, err := p.Register(ctx, redirect)
	if err != nil {
		return nil, err
	}
	if err := h.q().SetOAuthClient(ctx, id, connect.SealClient(h.aead, id, fresh), sealed); err != nil {
		return nil, err
	}
	if _, r, err = kept(); err != nil || r == nil {
		return nil, cmp.Or(err, fmt.Errorf("%s: its client was not kept", p.Name))
	}
	return p.Registered(r), nil
}

// forgetClient drops p's client from archive.oauth_client when its authorization server no longer knows it, so
// the next connect registers again; one another process registered since stays.
func (h *Hub) forgetClient(ctx context.Context, p *connect.Provider) {
	if sealed, err := h.q().OAuthClient(ctx, p.ID); err == nil {
		if r, err := connect.OpenClient(h.aead, p.ID, sealed); err == nil && r.ClientID == p.ClientID() {
			h.q().DeleteOAuthClient(ctx, p.ID, sealed)
		}
	}
}

// connectRedirect is the provider's redirect URI for connector id.
func (h *Hub) connectRedirect(id string) string { return h.api + "/connect/" + id + "/callback" }

// connectRoutes are /connect, like sign-in's /auth/oidc: GET /connect/<id> starts connecting one and sends the
// browser to the provider, and the provider sends it back to GET /connect/<id>/callback. A navigation sends no
// Archive-Workspace header, so the start takes the workspace from ?workspace= and the callback from its state.
// GET /connect lists the connectors and your connections, GET /connect/mine your own in every archive you are
// in, DELETE /connect/<connection id> disconnects one, and
// POST /connect/<connection id>/sync asks for its sync now (below), and PUT /connect/<connection id>/access
// narrows an integration to a group. A person connects, disconnects and syncs their own connections; only an
// owner, the archive's integrations.
func (h *Hub) connectRoutes(mux *http.ServeMux) {
	authed := func(pattern string, f func(a *Archive, w http.ResponseWriter, r *http.Request)) {
		mux.Handle(pattern, h.httpAuth(func(a *Archive) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { f(a, w, r) })
		}))
	}
	authed("GET /connect", func(a *Archive, w http.ResponseWriter, r *http.Request) {
		c, err := a.Connections(r.Context())
		httpWrite(w, http.StatusOK, c, err)
	})
	authed("GET /connect/mine", func(a *Archive, w http.ResponseWriter, r *http.Request) {
		c, err := a.hub.OwnConnections(r.Context(), a)
		httpWrite(w, http.StatusOK, map[string]any{"connections": c}, err)
	})
	authed("DELETE /connect/{id}", func(a *Archive, w http.ResponseWriter, r *http.Request) {
		id, err := httpID(r)
		if err == nil {
			err = a.Disconnect(r.Context(), id)
		}
		httpWrite(w, http.StatusNoContent, nil, err)
	})
	authed("PUT /connect/{id}/access", func(a *Archive, w http.ResponseWriter, r *http.Request) {
		id, err := httpID(r)
		var body struct {
			Access string `json:"access"`
		}
		if err == nil {
			err = httpDecode(r, &body)
		}
		if err == nil {
			err = a.SetAccess(r.Context(), id, body.Access)
		}
		httpWrite(w, http.StatusNoContent, nil, err)
	})
	authed("POST /connect/{id}/sync", func(a *Archive, w http.ResponseWriter, r *http.Request) {
		id, err := httpID(r)
		if err == nil {
			err = a.SyncNow(r.Context(), id)
		}
		httpWrite(w, http.StatusNoContent, nil, err)
	})
	mux.HandleFunc("GET /connect/{id}", func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("Archive-Workspace", r.URL.Query().Get("workspace"))
		h.httpAuth(func(a *Archive) http.Handler { return http.HandlerFunc(a.connectStart) }).ServeHTTP(w, r)
	})
	mux.HandleFunc("GET /connect/{id}/callback", func(w http.ResponseWriter, r *http.Request) {
		st, ok := connect.OpenState(h.aead, r.URL.Query().Get("state"))
		if !ok || st.Provider != r.PathValue("id") {
			httpWrite(w, 0, nil, signinMismatch)
			return
		}
		r.Header.Set("Archive-Workspace", st.Workspace)
		h.httpAuth(func(a *Archive) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { a.connectFinish(w, r, st) })
		}).ServeHTTP(w, r)
	})
}

// connectStart sends the browser to consent to the connector in the path, for whoever is signed in, or back
// to the connector in the app with ?error= when an MCP connector's server could not be set up.
func (a *Archive) connectStart(w http.ResponseWriter, r *http.Request) {
	ac, err := authUser(r.Context())
	var p *connect.Provider
	if err == nil {
		p, err = a.hub.connector(r.PathValue("id"))
	}
	if err != nil {
		httpWrite(w, 0, nil, err)
		return
	}
	if err = a.mayConnect(r.Context(), p.Connector); err == nil {
		p, err = a.hub.provider(r.Context(), p.ID, true)
	}
	if err != nil {
		http.Redirect(w, r, a.hub.app+connectBack(r.PathValue("id"))+"?"+url.Values{"workspace": {a.ws.ID.String()}, "error": {err.Error()}}.Encode(), http.StatusSeeOther)
		return
	}
	st := connect.State{Provider: p.ID, Workspace: a.ws.ID.String(), User: ac.UserID, Member: ac.member,
		Verifier: oauth2.GenerateVerifier(), Exp: time.Now().Add(connect.TTL).Unix()}
	http.Redirect(w, r, p.AuthURL(a.hub.connectRedirect(p.ID), connect.SealState(a.hub.aead, st), st.Verifier), http.StatusFound)
}

// connectFinish keeps the account the provider came back for, when the person who started is the one signed
// in, and sends the browser to the connector in the app, with ?error= when it could not.
func (a *Archive) connectFinish(w http.ResponseWriter, r *http.Request, st connect.State) {
	ac, err := authUser(r.Context())
	if err == nil && (ac.UserID != st.User || ac.member != st.Member) {
		err = signinMismatch
	}
	if err != nil {
		httpWrite(w, 0, nil, err)
		return
	}
	back := url.Values{"workspace": {st.Workspace}}
	p, err := a.hub.provider(r.Context(), st.Provider, false)
	if err == nil {
		err = a.mayConnect(r.Context(), p.Connector)
	}
	var g *connect.Grant
	if err == nil {
		g, err = p.Exchange(r.Context(), a.hub.connectRedirect(p.ID), r.URL.Query(), st.Verifier)
		if err != nil && p.MCP != "" && strings.Contains(err.Error(), "invalid_client") {
			a.hub.forgetClient(r.Context(), p)
		}
	}
	if err == nil {
		_, err = a.connectionSave(r.Context(), st.Member, p.Connector, g)
	}
	if err != nil {
		back.Set("error", err.Error())
	}
	http.Redirect(w, r, a.hub.app+connectBack(st.Provider)+"?"+back.Encode(), http.StatusSeeOther)
}

// connectBack is where the app shows connector id: the archive's integrations in its settings, and a person's
// own connectors in theirs.
func connectBack(id string) string {
	if c := connect.Get(id); c != nil && c.Integration {
		return "/archive/integrations/" + id
	}
	return "/connectors/" + id
}

// level is what the caller may do in the archive: their member's level, viewer through a viewer-only token,
// and their groups.
func (a *Archive) level(ctx context.Context) (string, []uuid.UUID, error) {
	ac, err := signedIn(ctx)
	if err != nil {
		return "", nil, err
	}
	m, err := a.memberOf(ctx, ac.Email)
	if err != nil {
		return "", nil, err
	}
	if ac.Role != ac.UserRole {
		return "viewer", m.Groups, nil
	}
	return m.Role, m.Groups, nil
}

// mayConnect says whether the caller may connect, disconnect or ask for a sync of c: anyone signed in with a
// full token connects their own; only an owner connects an integration, for the archive.
func (a *Archive) mayConnect(ctx context.Context, c *connect.Connector) error {
	if _, err := authUser(ctx); err != nil || !c.Integration {
		return err
	}
	if lvl, _, err := a.level(ctx); err != nil || lvl != "owner" {
		return cmp.Or(err, fmt.Errorf("only an owner connects %s, for the whole archive: %w", c.Name, ErrForbidden))
	}
	return nil
}

// mayUse says whether a member at level lvl in groups may use the archive's integration c, with
// archive.action: editors and owners, and of an integration narrowed to a group, only its members and the
// owners. Everyone who sees its rows reads them; acting on the archive's account writes, as an editor does.
func mayUse(c generated.ArchiveConnection, lvl string, groups []uuid.UUID) bool {
	return lvl == "owner" || lvl == "editor" && (c.Access == nil || slices.Contains(groups, *c.Access))
}

// connectionSave keeps g for member at provider: in the row they have for its account, or the archive's for an
// integration, or a new one; and queues its sync, which is due now. An integration connected to another
// account is disconnected first.
func (a *Archive) connectionSave(ctx context.Context, member uuid.UUID, c *connect.Connector, g *connect.Grant) (uuid.UUID, error) {
	var id uuid.UUID
	if g.Meta == nil {
		g.Meta = map[string]string{}
	}
	err := a.sys(ctx, func(ctx context.Context) error {
		q := a.st.db(ctx)
		var err error
		if c.Integration {
			var had generated.IntegrationForRow
			if had, err = q.IntegrationFor(ctx, c.ID); err == nil && had.Account != g.Account {
				return fmt.Errorf("this archive's %s is %s; disconnect it before connecting %s: %w", c.Name, had.Account, g.Account, ErrConflict)
			}
			id = had.ID
		} else {
			id, err = q.ConnectionFor(ctx, member, c.ID, g.Account)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			id, err = coreNewID(), nil
		}
		if err != nil {
			return err
		}
		return q.SaveConnection(ctx, generated.SaveConnectionParams{ID: id, Member: member, Provider: c.ID, Account: g.Account,
			Token: connect.SealToken(a.aead, id, g.Refresh), Scopes: g.Scopes, Meta: g.Meta, Integration: c.Integration})
	})
	if err == nil {
		a.syncSoon(ctx)
	}
	return id, err
}

// Connection is one of a member's connected accounts, as the API shows it: never its token.
type Connection struct {
	ID        uuid.UUID `json:"id"`
	Provider  string    `json:"provider"`
	Account   string    `json:"account"`
	Scopes    []string  `json:"scopes"`
	Error     string    `json:"error"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// SyncAt is when its next sync page is due, nil while it does not sync; SyncedAt when it last caught up;
	// Items how many things of it Archive has, the rows of its streams' tables.
	SyncAt   *time.Time `json:"sync_at"`
	SyncedAt *time.Time `json:"synced_at"`
	Items    int64      `json:"items"`
	// Integration says it is the archive's; Access is the id of the group it is narrowed to, "" for everyone; and
	// ConnectedBy who connected it last.
	Integration bool   `json:"integration"`
	Access      string `json:"access"`
	ConnectedBy string `json:"connected_by"`
}

// Connector is a service accounts can be connected at; Ready says this server has its OAuth client, and
// Integration that an account of it is the archive's, not a person's.
type Connector struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Ready       bool   `json:"ready"`
	Integration bool   `json:"integration"`
}

type Connections struct {
	Connectors  []Connector  `json:"connectors"`
	Connections []Connection `json:"connections"`
}

// Connections are the connectors, the caller's own connected accounts and the archive's integrations.
func (a *Archive) Connections(ctx context.Context) (*Connections, error) {
	ac, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	out := &Connections{Connectors: []Connector{}, Connections: []Connection{}}
	for _, c := range connect.All() {
		out.Connectors = append(out.Connectors, Connector{ID: c.ID, Name: c.Name, Ready: a.hub.connect[c.ID] != nil, Integration: c.Integration})
	}
	var rows []generated.ConnectionsRow
	err = a.st.System(ctx, func(ctx context.Context) (err error) {
		rows, err = a.st.db(ctx).Connections(ctx, ac.member)
		return err
	})
	for _, r := range rows {
		out.Connections = append(out.Connections, Connection(r))
	}
	return out, err
}

// OwnConnection is one of a person's own connections, with the archive it is connected in.
type OwnConnection struct {
	Connection
	Workspace Workspace `json:"workspace"`
}

// OwnConnections are the caller's own connections in every archive they are in, so they see them all in one
// place: each read in its own archive, as them, where it stays, and is synced and disconnected there. A token
// reaches one archive, and reads only its own there.
func (h *Hub) OwnConnections(ctx context.Context, here *Archive) ([]OwnConnection, error) {
	ac, err := authUser(ctx)
	if err != nil {
		return nil, err
	}
	all := []uuid.UUID{here.ws.ID}
	if ac.TokenID == uuid.Nil && ac.TokenName == "" {
		u, err := h.q().UserByID(ctx, ac.UserID)
		if err != nil {
			return nil, err
		}
		all = u.Workspaces
	}
	out := []OwnConnection{}
	for _, ws := range all {
		a, release, err := h.acquire(ctx, ws.String())
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		rows, err := a.ownConnections(ctx, ac)
		release()
		if errors.Is(err, ErrForbidden) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("workspace %s: %w", ws, err)
		}
		for _, r := range rows {
			out = append(out, OwnConnection{Connection: Connection(r), Workspace: workspaceOut(a.ws)})
		}
	}
	slices.SortStableFunc(out, func(x, y OwnConnection) int {
		return cmp.Or(strings.Compare(x.Provider, y.Provider), strings.Compare(x.Account, y.Account), strings.Compare(x.Workspace.Name, y.Workspace.Name))
	})
	return out, nil
}

// ownConnections are the person ac's own connections in a, read as their member there.
func (a *Archive) ownConnections(ctx context.Context, ac *Actor) ([]generated.ConnectionsRow, error) {
	me, err := a.userActor(ctx, &User{ID: ac.UserID, Email: ac.Email, Name: ac.Name})
	if err != nil {
		return nil, err
	}
	rows, err := coreDo(a, withArchive(WithActor(ctx, me), a), func(ctx context.Context) ([]generated.ConnectionsRow, error) {
		return a.st.db(ctx).Connections(ctx, me.member)
	})
	return slices.DeleteFunc(rows, func(r generated.ConnectionsRow) bool { return r.Integration }), err
}

// Disconnect deletes the caller's connection id, or, for an owner, the archive's integration id, with its
// rows, and revokes its refresh token at the provider where it can. A revoke that fails leaves the grant at
// the provider, where the person can still remove it; the connection is gone either way.
func (a *Archive) Disconnect(ctx context.Context, id uuid.UUID) error {
	c, err := a.connectionMine(ctx, id)
	if err != nil {
		return err
	}
	err = a.sys(ctx, func(ctx context.Context) (err error) {
		q := a.st.db(ctx)
		if c, err = q.DeleteConnection(ctx, c.ID); err != nil {
			return err
		}
		// Its rows went with it; the people and organizations only they named go too.
		return q.DeleteUnknown(ctx)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("connection %s: %w", id, ErrNotFound)
	}
	if err != nil {
		return err
	}
	a.conns.forget(c.ID)
	ctx = context.WithoutCancel(ctx)
	p, err := a.hub.provider(ctx, c.Provider, false)
	var rt string
	if err == nil {
		rt, err = connect.OpenToken(a.aead, c.ID, c.Token)
	}
	if err == nil {
		// Its watches end first, while its grant still lets Archive end them.
		var cl *connect.Client
		if push, _ := p.PushOf(); push == nil || push.Unwatch == nil {
		} else if tok, terr := p.Refresh(ctx, rt); terr == nil {
			cl = p.Open(ctx, tok, c.Meta)
			rt = tok.RefreshToken
		}
		if werr := a.unwatch(ctx, c, p, cl); werr != nil {
			fmt.Fprintf(os.Stderr, "archive: workspace %s: disconnected %s, but did not end all it pushed: %v\n", a.ws.ID, id, werr)
		}
		if cl != nil {
			cl.Close()
		}
		err = p.RevokeToken(ctx, rt)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "archive: workspace %s: disconnected %s, but did not revoke it: %v\n", a.ws.ID, id, err)
	}
	return nil
}

// connectionMine is the connection id, when the caller may connect, disconnect and sync it: their own, or the
// archive's integration for an owner; not found for anyone else.
func (a *Archive) connectionMine(ctx context.Context, id uuid.UUID) (generated.ArchiveConnection, error) {
	ac, err := authUser(ctx)
	if err != nil {
		return generated.ArchiveConnection{}, err
	}
	var c generated.ArchiveConnection
	err = a.st.System(ctx, func(ctx context.Context) (err error) {
		c, err = a.st.db(ctx).Connection(ctx, id)
		return err
	})
	switch {
	case errors.Is(err, pgx.ErrNoRows), err == nil && !c.Integration && c.Member != ac.member:
		return c, fmt.Errorf("connection %s: %w", id, ErrNotFound)
	case err != nil:
		return c, err
	case c.Integration:
		if lvl, _, err := a.level(ctx); err != nil || lvl != "owner" {
			return c, cmp.Or(err, fmt.Errorf("only an owner changes the archive's %s: %w", c.Provider, ErrForbidden))
		}
	}
	return c, nil
}

// SetAccess narrows the archive's integration id, and its rows, to the members of group, by its id or its
// name, or opens them to everyone with "", as an owner does.
func (a *Archive) SetAccess(ctx context.Context, id uuid.UUID, group string) error {
	c, err := a.connectionMine(ctx, id)
	switch {
	case err != nil:
		return err
	case !c.Integration:
		return fmt.Errorf("only an integration is narrowed to a group; a personal connection is its owner's alone: %w", httpErrBadRequest)
	}
	return a.sys(ctx, func(ctx context.Context) error {
		access, err := a.groupID(ctx, group, false)
		if err != nil {
			return err
		}
		if err := a.st.db(ctx).SetConnectionAccess(ctx, access, id); err != nil {
			return err
		}
		return syncNarrow(ctx, a.st.Q(ctx), id, access)
	})
}

// connectionToken is a fresh access token for the connection id; each call refreshes, so code that uses an
// account calls connectionAccess, which keeps one while it lasts. It refreshes holding the connection's row,
// so processes refreshing at once take turns, and each refreshes with the refresh token the one before kept:
// providers that replace the refresh token on every refresh refuse an old one, some revoking the grant. A
// refresh that fails is the connection's error until one works; one the provider refused is errRevoked too,
// which stops its sync.
func (a *Archive) connectionToken(ctx context.Context, id uuid.UUID) (*oauth2.Token, error) {
	var tok *oauth2.Token
	var refused error
	err := a.st.System(ctx, func(ctx context.Context) error {
		q := a.st.db(ctx)
		c, err := q.ConnectionLocked(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("connection %s: %w", id, ErrNotFound)
		}
		if err != nil {
			return err
		}
		p, err := a.hub.provider(ctx, c.Provider, false)
		if err != nil {
			return err
		}
		rt, err := connect.OpenToken(a.aead, c.ID, c.Token)
		if err != nil {
			return err
		}
		if tok, err = p.Refresh(ctx, rt); err != nil {
			msg := fmt.Sprintf("%s refused to let Archive keep reading %s (%s); connect it again", p.Name, c.Account, connect.Reason(err))
			refused = fmt.Errorf("connection %s: %s: %w", id, msg, ErrUnauthorized)
			if re := (*oauth2.RetrieveError)(nil); errors.As(err, &re) && re.Response != nil && re.Response.StatusCode/100 == 4 {
				refused = fmt.Errorf("%w%w", refused, errRevoked)
			}
			return q.SetConnectionError(ctx, id, msg)
		}
		if c.Error != "" || tok.RefreshToken != rt {
			return q.SetConnectionToken(ctx, id, connect.SealToken(a.aead, id, tok.RefreshToken))
		}
		return nil
	})
	if err == nil {
		err = refused
	}
	if err != nil {
		return nil, err
	}
	return tok, nil
}

// Syncs: Archive keeps a copy of what each connection's account has, as rows of the tables of its domain
// (below), in the streams its connector declares (connect/sync.go). A sync runs only when something asks for
// one, and never on a timer, so the service can sleep: connecting, Sync now (POST /connect/<id>/sync), a push from the provider
// (below), or a member reading what it syncs or opening the workspace while a connection is stale
// (syncCatchUp).
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

const (
	// syncPageTime is the most a page may take, and syncLease how long a leased page has before another may take
	// its connection over: a little longer, so a page never outlives its lease, and short, so an instance that
	// stops mid-page, as a deploy stops it, holds its connection up for minutes at most. syncLeaseCheck is how
	// soon a task looks again at a connection another task's page holds, which wakes the queue itself once done.
	// syncBudget is how long one task pulls pages before it queues the rest.
	syncPageTime   = 2 * time.Minute
	syncLease      = syncPageTime + time.Minute
	syncLeaseCheck = 15 * time.Second
	syncBudget     = 2 * time.Minute
	// A failed page is retried syncRetry later, doubling, syncTries times in a row at most.
	syncRetry = time.Minute
	syncTries = 5
	// syncBusyWait is the longest a page a busy provider turned away waits before it is tried again.
	syncBusyWait = 15 * time.Minute
	// syncMaxPages is the most pages a stream reads for one ask, against a provider whose cursor goes round.
	syncMaxPages = 100_000
	// syncStale is how old a connection's last sync is before a read or an open asks for another.
	syncStale = 5 * time.Minute
	// syncRename is how often a sync asks the provider again what its connection's account is called, as an
	// account can change after it connects, like the companies a Gusto sign-in reaches.
	syncRename = 24 * time.Hour
)

// errRevoked is a refresh the provider refused, which stops a connection's sync.
var errRevoked = errors.New("revoked")

// syncStream is where one stream of a connection is, in its cursor. At is when its next page is due, nil
// now, while it is due; Done the ask it last caught up to, and Last when; Since when the full listing under
// way began, which the rows it has not read since are older than; Pages how many pages it has read for this
// ask; Fails how many in a row failed, and Error why the last did. Rows is the version of Archive's tables its
// rows are in (syncRowsVersion): a stream of an older one is due, and starts from scratch. Named, kept on a
// connector's first stream, is when the account's name was last asked for (syncRename).
type syncStream struct {
	Cursor string     `json:"cursor,omitempty"`
	At     *time.Time `json:"at,omitempty"`
	Done   *time.Time `json:"done,omitempty"`
	Last   *time.Time `json:"last,omitempty"`
	More   bool       `json:"more,omitempty"`
	Since  *time.Time `json:"since,omitempty"`
	Skip   string     `json:"skip,omitempty"`
	Pages  int        `json:"pages,omitempty"`
	Fails  int        `json:"fails,omitempty"`
	Error  string     `json:"error,omitempty"`
	Rows   int        `json:"rows,omitempty"`
	Named  *time.Time `json:"named,omitempty"`
}

// due says whether the stream has yet to catch up to the ask at asked: one that pulls only every so often
// counts as caught up while it last did within every of the ask, unless a listing of it is under way. One whose
// rows are in older tables is due until it starts again.
func (v *syncStream) due(asked time.Time, every time.Duration) bool {
	if v.Rows < syncRowsVersion {
		return true
	}
	if v.Done != nil && !v.Done.Before(asked) {
		return false
	}
	return every <= 0 || v.Last == nil || v.More || !asked.Before(v.Last.Add(every))
}

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
		if !v.due(asked, s.Every) {
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
	// The first stream starting a listing asks again what the account is called, at most every syncRename.
	var named bool
	var account string
	if s != nil {
		cur := syncStateOf(c.Cursor).of(s.Name).fresh()
		pctx, cancel := context.WithTimeout(ctx, syncPageTime)
		var cl *connect.Client
		if cl, err = a.connClient(pctx, c); err == nil {
			page, err = s.Pull(cl, cur.Cursor)
			if err == nil && page.More && page.Cursor == cur.Cursor {
				err = fmt.Errorf("%s said there was more, but from where this page began", p.Name)
			}
			if err == nil {
				a.syncWatch(ctx, c, p, cl)
			}
			if err == nil && p.Account != nil && s == streams[0] && !cur.More && (cur.Named == nil || time.Since(*cur.Named) >= syncRename) {
				named = true
				name, err2 := p.Account(cl)
				if err2 != nil {
					fmt.Fprintf(os.Stderr, "archive: workspace %s: naming %s %s: %v\n", a.ws.ID, p.Name, c.Account, err2)
				}
				account = strings.TrimSpace(name)
			}
			cl.Close()
		}
		if errors.Is(err, context.DeadlineExceeded) {
			err = fmt.Errorf("a page of %s took more than %s: %w", s.Name, syncPageTime, err)
		}
		cancel()
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
			// A stream of older tables starts again from here, whatever this page did.
			cur, name = st.of(s.Name).fresh(), s.Name
			cur.Rows, st[s.Name] = syncRowsVersion, cur
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
			busy := connect.Busy(err)
			if busy {
				// A provider that is busy or out of reach is tried again, at most syncBusyWait apart, for as long
				// as it takes; only a page that fails some other way waits for the next ask.
				cur.At = ptr(now.Add(min(syncRetry<<min(cur.Fails-1, 16), syncBusyWait)))
			}
			if cur.Fails >= syncTries && !busy {
				// It waits for the next ask, as one caught up does, with its error, and tries as many times then.
				cur.At, cur.Done, cur.Fails = nil, &c.AskedAt, 0
			}
		default:
			w := &syncDB{q: q, db: a.st.Q(ctx), id: c.ID, acc: c2.Access, now: now}
			if err := syncSave(ctx, w, s, page, cur); err != nil {
				return err
			}
			if named {
				cur.Named = &now
				if account != "" {
					if err := q.RenameConnection(ctx, c.ID, account); err != nil {
						return err
					}
				}
			}
			cur.Cursor, cur.More, cur.Skip, cur.Fails, cur.Error, cur.At = page.Cursor, page.More, "", 0, "", nil
			cur.Pages++
			if page.More {
				cur.At = ptr(now.Add(s.Gap))
			} else {
				cur.Done, cur.Last, cur.Pages = &c.AskedAt, &now, 0
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
			if !v.due(c2.AskedAt, d.Every) {
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
		// A stream the connector no longer declares is gone, and so are its rows.
		for k := range st {
			if !slices.ContainsFunc(streams, func(d *connect.Stream) bool { return d.Name == k }) {
				delete(st, k)
				w := &syncDB{q: q, db: a.st.Q(ctx), id: c.ID, now: now}
				if err := w.stream(ctx, k); err != nil {
					return err
				}
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

// fresh is v, or where a stream whose rows are in older tables starts again: from scratch, its cursor and memory
// forgotten, with only when the account was named kept.
func (v *syncStream) fresh() *syncStream {
	if v.Rows >= syncRowsVersion {
		return v
	}
	return &syncStream{Named: v.Named}
}

// syncSave writes the records of page of stream s into its table with w, in the transaction at w.now; a record
// the page says is the Same is only seen. A page that ends a full listing deletes the rows it began after and
// did not read.
func syncSave(ctx context.Context, w *syncDB, s *connect.Stream, page *connect.Page, cur *syncStream) error {
	if page.Full && cur.Since == nil {
		cur.Since = &w.now
	}
	live, at := []syncRec{}, map[string]int{}
	var gone, same []string
	for _, r := range page.Records {
		if r.ID == "" {
			continue
		}
		// A record twice in a page is as it was last.
		if i, ok := at[r.ID]; ok {
			live[i].ID = ""
			delete(at, r.ID)
		}
		gone = slices.DeleteFunc(gone, func(g string) bool { return g == r.ID })
		same = slices.DeleteFunc(same, func(g string) bool { return g == r.ID })
		switch {
		case r.Deleted:
			gone = append(gone, r.ID)
			continue
		case r.Same:
			same = append(same, r.ID)
			continue
		}
		b, err := syncJSON(r.Data)
		if err != nil {
			return fmt.Errorf("record %s of %s: %w", r.ID, s.Name, err)
		}
		at[r.ID] = len(live)
		live = append(live, syncRec{ID: r.ID, Data: b, Updated: r.Updated})
	}
	live = slices.DeleteFunc(live, func(r syncRec) bool { return r.ID == "" })
	if len(live) > 0 {
		if err := w.write(ctx, s, live); err != nil {
			return err
		}
	}
	if len(same) > 0 {
		if err := w.see(ctx, s, same); err != nil {
			return err
		}
	}
	if len(gone) > 0 {
		if err := w.delete(ctx, s, gone); err != nil {
			return err
		}
	}
	switch {
	case page.Full && !page.More:
		err := w.unseen(ctx, s, *cur.Since)
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
		// Sync now pulls every stream, those that pull only every so often too.
		for _, v := range st {
			v.At, v.Fails, v.Last = nil, 0, nil
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
// syncing, as reading what connections sync or opening the workspace does. It does not wait for them: the read reads what
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
// connections sync, by "<connector>.<stream>", each with what it pulls and the table it lands in.
func (a *Archive) streamTypes(ctx context.Context) string {
	var b strings.Builder
	var conns []generated.ArchiveConnection
	if ac := ActorFrom(ctx); ac != nil {
		conns, _ = a.memberConnections(ctx, ac.member)
	}
	b.WriteString("\n" + runDoc("", `What Archive syncs of the accounts you connected, by "<connector>.<stream>", and the table each lands in: its rows' stream column is the part after the dot, and their connection column the connection's id (see Connected accounts in the header)`) + "interface Streams {\n")
	done := map[string]bool{}
	for _, c := range conns {
		p, err := a.hub.connector(c.Provider)
		if err != nil || done[p.ID] {
			continue
		}
		done[p.ID] = true
		for _, s := range p.Streams {
			b.WriteString(runDoc("    ", p.Name+": "+s.Doc))
			fmt.Fprintf(&b, "    %q: %q;\n", p.ID+"."+s.Name, s.Table)
		}
	}
	b.WriteString("}\n")
	return b.String()
}

// Archive's tables of what connections sync (archive.sql): each stream's records land as rows of the table it
// names, as its Rows maps them (connect/sync.go), with the rows that belong to each, like a message's
// parties. syncSave writes them a page at a time, in the transaction it saves the stream's cursor in: a row
// for each source_id of each stream of each connection, written again, with all its own rows, when it is read
// again, only seen when the stream says it is the Same, and deleted when it is gone there, and with the people
// and organizations nothing names any more.
//
// Every row has its connection and its access, the connection's, so row-level security sees it as it sees the
// connection. A column that names another row of the connection keeps that row's id there in <column>_source,
// and links once both are there, whichever came first (syncLink).

// syncRowsVersion is the version of the tables: a stream whose rows are of one before syncs from scratch, its
// cursor and memory forgotten, so its rows are written whole again. Changing a table or a mapping so that the
// rows already synced should change counts it up.
const syncRowsVersion = 2

// syncTable is what syncSave needs to know of a table of archive.sql: for a row that belongs to a row of another
// table, that table, its parent, and the column that names it; and its columns that link another row of the
// connection, by the table of that row.
type syncTable struct {
	parent, of string
	links      map[string]string
}

// syncSchema are the tables of archive.sql that syncs write.
var syncSchema = map[string]syncTable{
	"mail_message":                     {},
	"mail_party":                       {parent: "mail_message", of: "message"},
	"mail_attachment":                  {parent: "mail_message", of: "message"},
	"doc_page":                         {},
	"payroll_company":                  {},
	"payroll_department":               {links: map[string]string{"company": "payroll_company"}},
	"payroll_location":                 {links: map[string]string{"company": "payroll_company"}},
	"payroll_schedule":                 {links: map[string]string{"company": "payroll_company"}},
	"payroll_worker":                   {links: map[string]string{"company": "payroll_company", "department": "payroll_department", "manager": "payroll_worker"}},
	"payroll_job":                      {parent: "payroll_worker", of: "worker", links: map[string]string{"location": "payroll_location"}},
	"payroll_compensation":             {parent: "payroll_worker", of: "worker"},
	"payroll_address":                  {parent: "payroll_worker", of: "worker", links: map[string]string{"location": "payroll_location"}},
	"payroll_employment":               {parent: "payroll_worker", of: "worker"},
	"payroll_custom_field":             {links: map[string]string{"company": "payroll_company"}},
	"payroll_custom_value":             {parent: "payroll_worker", of: "worker", links: map[string]string{"field": "payroll_custom_field"}},
	"payroll_department_member":        {parent: "payroll_department", of: "department", links: map[string]string{"worker": "payroll_worker"}},
	"payroll_schedule_assignment":      {links: map[string]string{"company": "payroll_company", "default_schedule": "payroll_schedule", "hourly_schedule": "payroll_schedule", "salaried_schedule": "payroll_schedule"}},
	"payroll_schedule_member":          {parent: "payroll_schedule_assignment", of: "assignment", links: map[string]string{"worker": "payroll_worker", "department": "payroll_department", "schedule": "payroll_schedule"}},
	"payroll_earning_type":             {links: map[string]string{"company": "payroll_company"}},
	"payroll_time_off_request":         {links: map[string]string{"company": "payroll_company", "worker": "payroll_worker", "approver": "payroll_worker"}},
	"payroll_time_off_day":             {parent: "payroll_time_off_request", of: "request"},
	"payroll_time_off_balance":         {links: map[string]string{"company": "payroll_company", "worker": "payroll_worker"}},
	"payroll_time_off_policy":          {parent: "payroll_time_off_balance", of: "balance"},
	"payroll_run":                      {links: map[string]string{"company": "payroll_company", "schedule": "payroll_schedule"}},
	"payroll_pay":                      {parent: "payroll_run", of: "run", links: map[string]string{"worker": "payroll_worker"}},
	"payroll_pay_line":                 {parent: "payroll_run", of: "run", links: map[string]string{"worker": "payroll_worker"}},
	"payroll_pay_period":               {links: map[string]string{"company": "payroll_company", "schedule": "payroll_schedule", "run": "payroll_run"}},
	"payroll_contractor_payment":       {links: map[string]string{"company": "payroll_company", "worker": "payroll_worker"}},
	"payroll_contractor_payment_group": {links: map[string]string{"company": "payroll_company"}},
	"payroll_task":                     {links: map[string]string{"company": "payroll_company"}},
	"drive_file":                       {links: map[string]string{"folder": "drive_file"}},
	"calendar_event":                   {links: map[string]string{"series": "calendar_event"}},
	"calendar_attendee":                {parent: "calendar_event", of: "event"},
	"ledger_account":                   {links: map[string]string{"parent": "ledger_account"}},
	"ledger_contact":                   {},
	"ledger_item":                      {links: map[string]string{"income_account": "ledger_account", "expense_account": "ledger_account"}},
	"ledger_entry":                     {links: map[string]string{"contact": "ledger_contact", "account": "ledger_account"}},
	"ledger_line":                      {parent: "ledger_entry", of: "entry", links: map[string]string{"account": "ledger_account", "item": "ledger_item"}},
}

// syncRec is a record of a page to write, its data as JSON.
type syncRec struct {
	ID      string
	Data    json.RawMessage
	Updated time.Time
}

// syncDB is the transaction syncSave writes in: its queries, and the statements below.
type syncDB struct {
	q   *generated.Queries
	db  generated.DBTX
	id  uuid.UUID
	acc *uuid.UUID
	now time.Time
}

// write writes recs of stream s into its table, with the rows that belong to each, and links what they name.
// A record that is no row there is deleted.
func (w *syncDB) write(ctx context.Context, s *connect.Stream, recs []syncRec) error {
	t, ok := syncSchema[s.Table]
	if !ok || t.parent != "" {
		return fmt.Errorf("%s lands in %q, which is no table of archive.sql a stream lands in", s.Name, s.Table)
	}
	rows := make([]*connect.Row, 0, len(recs))
	var kept []syncRec
	var none []string
	for _, r := range recs {
		if row := s.RowOf(r.Data); row != nil {
			rows, kept = append(rows, row), append(kept, r)
		} else {
			none = append(none, r.ID)
		}
	}
	if len(none) > 0 {
		if err := w.delete(ctx, s, none); err != nil {
			return err
		}
	}
	if len(rows) == 0 {
		return nil
	}
	people, orgs, err := w.known(ctx, rows)
	if err != nil {
		return err
	}
	objs := make([]map[string]any, len(rows))
	for i, row := range rows {
		o := w.object(s.Table, row, people, orgs)
		o["stream"], o["source_id"], o["seen_at"], o["updated_at"] = s.Name, kept[i].ID, w.now, w.now
		if !kept[i].Updated.IsZero() {
			o["updated_at"] = kept[i].Updated
		}
		objs[i] = o
	}
	cols := syncCols(objs)
	set := []string{}
	var compare []string
	for _, c := range cols {
		if c == "connection" || c == "stream" || c == "source_id" || c == "updated_at" {
			continue
		}
		set = append(set, c+" = EXCLUDED."+c)
		if c != "seen_at" && c != "access" && t.links[c] == "" {
			compare = append(compare, c)
		}
	}
	tbl := "archive." + s.Table
	sql := "INSERT INTO " + tbl + " AS t (" + strings.Join(cols, ", ") + ") SELECT " + strings.Join(cols, ", ") +
		" FROM jsonb_populate_recordset(NULL::" + tbl + ", $1) ON CONFLICT (connection, stream, source_id) DO UPDATE SET " + strings.Join(set, ", ") +
		", updated_at = CASE WHEN (" + syncTuple("t", compare) + ") IS DISTINCT FROM (" + syncTuple("EXCLUDED", compare) + ") THEN EXCLUDED.updated_at ELSE t.updated_at END" +
		" RETURNING id, source_id"
	res, err := w.db.Query(ctx, sql, jsonOf(objs))
	if err != nil {
		return fmt.Errorf("%s: %w", s.Table, err)
	}
	ids := map[string]uuid.UUID{}
	for res.Next() {
		var id uuid.UUID
		var src string
		if err := res.Scan(&id, &src); err != nil {
			res.Close()
			return err
		}
		ids[src] = id
	}
	res.Close()
	if err := res.Err(); err != nil {
		return fmt.Errorf("%s: %w", s.Table, err)
	}
	// The rows that belong to them are theirs alone: those of before go.
	written := []string{s.Table}
	parents := slices.Collect(maps.Values(ids))
	kids := map[string][]map[string]any{}
	for i, row := range rows {
		for _, c := range row.Children {
			ct, ok := syncSchema[c.Table]
			if !ok || ct.parent != s.Table {
				return fmt.Errorf("%s: %q is no table of the rows of %s", s.Name, c.Table, s.Table)
			}
			o := w.object(c.Table, &c.Row, people, orgs)
			o[ct.of] = ids[kept[i].ID]
			kids[c.Table] = append(kids[c.Table], o)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(syncSchema)) {
		ct := syncSchema[name]
		if ct.parent != s.Table {
			continue
		}
		written = append(written, name)
		if _, err := w.db.Exec(ctx, "DELETE FROM archive."+name+" WHERE "+ct.of+" = ANY($1)", parents); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if len(kids[name]) == 0 {
			continue
		}
		cols := syncCols(kids[name])
		if _, err := w.db.Exec(ctx, "INSERT INTO archive."+name+" ("+strings.Join(cols, ", ")+") SELECT "+strings.Join(cols, ", ")+
			" FROM jsonb_populate_recordset(NULL::archive."+name+", $1)", jsonOf(kids[name])); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return w.link(ctx, written)
}

// object is the columns of a row of table: cols, its people and organizations by their ids, each link by its
// <column>_source, which links it once both are there, and its connection and access.
func (w *syncDB) object(table string, row *connect.Row, people, orgs map[string]uuid.UUID) map[string]any {
	o := map[string]any{}
	for col, v := range row.Cols {
		// A value a provider left empty is none: '' is no email, date or amount, and their domains refuse it.
		if v == "" {
			v = nil
		}
		o[col] = v
	}
	for col, e := range row.People {
		o[col] = people[e]
	}
	for col, e := range row.Orgs {
		o[col] = orgs[e]
	}
	for col := range syncSchema[table].links {
		o[col], o[col+"_source"] = nil, row.Refs[col]
	}
	o["connection"], o["access"] = w.id, w.acc
	return o
}

// known are the people and organizations rows name, by address and EIN, each added unless there is one.
func (w *syncDB) known(ctx context.Context, rows []*connect.Row) (map[string]uuid.UUID, map[string]uuid.UUID, error) {
	var emails, eins []string
	for _, r := range rows {
		for _, x := range append([]connect.Row{*r}, children(r)...) {
			emails = append(emails, slices.Collect(maps.Values(x.People))...)
			eins = append(eins, slices.Collect(maps.Values(x.Orgs))...)
		}
	}
	people, orgs := map[string]uuid.UUID{}, map[string]uuid.UUID{}
	if len(emails) > 0 {
		got, err := w.q.KnownPeople(ctx, emails)
		if err != nil {
			return nil, nil, err
		}
		for _, p := range got {
			people[p.Email] = p.ID
		}
	}
	if len(eins) > 0 {
		got, err := w.q.KnownOrgs(ctx, eins)
		if err != nil {
			return nil, nil, err
		}
		for _, o := range got {
			orgs[o.Ein] = o.ID
		}
	}
	return people, orgs, nil
}

func children(r *connect.Row) []connect.Row {
	out := make([]connect.Row, len(r.Children))
	for i, c := range r.Children {
		out[i] = c.Row
	}
	return out
}

// link links the rows of the connection that name a row of the tables written, or are of them, to the rows
// whose ids there they keep.
func (w *syncDB) link(ctx context.Context, written []string) error {
	for _, name := range slices.Sorted(maps.Keys(syncSchema)) {
		for _, col := range slices.Sorted(maps.Keys(syncSchema[name].links)) {
			to := syncSchema[name].links[col]
			if !slices.Contains(written, name) && !slices.Contains(written, to) {
				continue
			}
			if _, err := w.db.Exec(ctx, "UPDATE archive."+name+" x SET "+col+" = p.id FROM archive."+to+" p WHERE x.connection = $1 AND p.connection = $1"+
				" AND p.source_id = x."+col+"_source AND x."+col+" IS DISTINCT FROM p.id", w.id); err != nil {
				return fmt.Errorf("linking %s.%s: %w", name, col, err)
			}
		}
	}
	return nil
}

// see marks the rows of ids of stream s seen, as the Same.
func (w *syncDB) see(ctx context.Context, s *connect.Stream, ids []string) error {
	_, err := w.db.Exec(ctx, "UPDATE archive."+s.Table+" SET seen_at = $4 WHERE connection = $1 AND stream = $2 AND source_id = ANY($3)", w.id, s.Name, ids, w.now)
	return err
}

// delete deletes the rows of ids of stream s, gone there, with the rows that belong to them.
func (w *syncDB) delete(ctx context.Context, s *connect.Stream, ids []string) error {
	return w.gone(ctx, "DELETE FROM archive."+s.Table+" WHERE connection = $1 AND stream = $2 AND source_id = ANY($3)", w.id, s.Name, ids)
}

// unseen deletes the rows of stream s that a full listing begun at since did not read.
func (w *syncDB) unseen(ctx context.Context, s *connect.Stream, since time.Time) error {
	return w.gone(ctx, "DELETE FROM archive."+s.Table+" WHERE connection = $1 AND stream = $2 AND seen_at < $3", w.id, s.Name, since)
}

// stream deletes the rows of a stream the connector no longer declares, in whichever table they are.
func (w *syncDB) stream(ctx context.Context, name string) error {
	for _, t := range slices.Sorted(maps.Keys(syncSchema)) {
		if syncSchema[t].parent != "" {
			continue
		}
		if err := w.gone(ctx, "DELETE FROM archive."+t+" WHERE connection = $1 AND stream = $2", w.id, name); err != nil {
			return err
		}
	}
	return nil
}

// gone runs a delete, and deletes the people and organizations nothing names any more once it deleted any.
func (w *syncDB) gone(ctx context.Context, sql string, args ...any) error {
	tag, err := w.db.Exec(ctx, sql, args...)
	if err != nil || tag.RowsAffected() == 0 {
		return err
	}
	return w.q.DeleteUnknown(ctx)
}

// syncNarrow gives every row connection id synced the access its connection now has.
func syncNarrow(ctx context.Context, db generated.DBTX, id uuid.UUID, access *uuid.UUID) error {
	for _, t := range slices.Sorted(maps.Keys(syncSchema)) {
		if _, err := db.Exec(ctx, "UPDATE archive."+t+" SET access = $2 WHERE connection = $1 AND access IS DISTINCT FROM $2", id, access); err != nil {
			return err
		}
	}
	return nil
}

// syncCols are the columns of objs, all of each's keys, in order; a row without one has it null. A key that is
// no column name is left out, so nothing a mapping says becomes SQL; one that is no column of the table fails.
func syncCols(objs []map[string]any) []string {
	var cols []string
	for _, o := range objs {
		for k := range o {
			if syncColRe.MatchString(k) && !slices.Contains(cols, k) {
				cols = append(cols, k)
			}
		}
	}
	slices.Sort(cols)
	return cols
}

var syncColRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// syncTuple is cols of alias as a row, to compare.
func syncTuple(alias string, cols []string) string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = alias + "." + c
	}
	return "ROW(" + strings.Join(out, ", ") + ")"
}

func jsonOf(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// Pushes: POST /push/<connector> is where a connector's provider tells Archive that an account changed
// (connect/sync.go), which asks for that connection's sync and nothing more: the sync reads what changed from
// its cursor. A push names a watch by its key; archive.watch, in the main database, says which workspace and
// connection each is, and holds the secret its pushes prove themselves with. A push for a watch Archive no
// longer has, as of an account disconnected since, is answered as heard and starts nothing, so the provider
// does not send it again; one not signed as its provider signs is refused.
//
// A page of a connection's sync keeps its watches (syncWatch): it asks the provider for them the first time,
// and renews one once it nears its end. Nothing renews them on a timer: a watch of an account that nothing
// syncs lapses, the pushes it would have sent are missed, and the next sync, whatever asks for it, reads all
// they would have said from its cursor and watches again. Disconnecting ends them.

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
	w.WriteHeader(cmp.Or(heard.Status, http.StatusAccepted))
	io.WriteString(w, heard.Body)
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

// syncOnRead catches up the caller's syncs, as reading what they sync does.
func (a *Archive) syncOnRead(ctx context.Context) {
	if ac := ActorFrom(ctx); ac != nil && ac.member != uuid.Nil {
		a.syncCatchUp(ctx, ac.member)
	}
}

// Actions: archive.action(name, input, opts) in a run, which does something with one of the caller's own
// connections. name is "<connector>.<action>". An API connector's actions are the Go funcs of its file in the
// connect package; an MCP connector's are its server's tools, called with tools/call, each named for its tool
// without the connector's own prefix (notion-search is notion.search). The caller's connection to the
// connector is the one used, or the one opts.connection names, by account or id, when they have several.
//
// actionTypes is the TypeScript the run tool's docs end with: an Actions interface of the actions of the
// caller's connections, with their input and output types from the actions' JSON Schemas, which
// archive.action's signature in sdk.d.ts reads, as archive.list reads Tables. A workspace keeps, between
// runs, each connection's access token while it lasts and each MCP connection's tools for an hour.

const (
	actionToolsTTL = time.Hour
	actionFailTTL  = time.Minute
)

// archiveConns is what a workspace keeps of its connections between runs.
type archiveConns struct {
	mu     sync.Mutex
	access map[uuid.UUID]*oauth2.Token
	tools  map[uuid.UUID]*actionTools
}

// actionTools are an MCP connection's tools as its server last listed them, or why it could not.
type actionTools struct {
	at    time.Time
	tools []*mcp.Tool
	err   error
}

// forget drops what is kept of connection id, as when it is disconnected.
func (cs *archiveConns) forget(id uuid.UUID) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	delete(cs.access, id)
	delete(cs.tools, id)
}

// connectionAccess is an access token of the connection id, the one kept while it has a minute left, or a
// fresh one.
func (a *Archive) connectionAccess(ctx context.Context, id uuid.UUID) (*oauth2.Token, error) {
	a.conns.mu.Lock()
	tok := a.conns.access[id]
	a.conns.mu.Unlock()
	if tok != nil && (tok.Expiry.IsZero() || time.Until(tok.Expiry) > time.Minute) {
		return tok, nil
	}
	tok, err := a.connectionToken(ctx, id)
	if err != nil {
		return nil, err
	}
	a.conns.mu.Lock()
	if a.conns.access == nil {
		a.conns.access = map[uuid.UUID]*oauth2.Token{}
	}
	a.conns.access[id] = tok
	a.conns.mu.Unlock()
	return tok, nil
}

// memberConnections are the personal connections of member and the archive's integrations, by connector and
// account.
func (a *Archive) memberConnections(ctx context.Context, member uuid.UUID) ([]generated.ArchiveConnection, error) {
	var out []generated.ArchiveConnection
	err := a.st.System(ctx, func(ctx context.Context) (err error) {
		out, err = a.st.db(ctx).MemberConnections(ctx, member)
		return err
	})
	return out, err
}

// connClient is a client of the connection c.
func (a *Archive) connClient(ctx context.Context, c generated.ArchiveConnection) (*connect.Client, error) {
	p, err := a.hub.provider(ctx, c.Provider, false)
	if err != nil {
		return nil, err
	}
	tok, err := a.connectionAccess(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	return p.Open(ctx, tok, c.Meta), nil
}

// connTools are the tools of the MCP connection c, kept for an hour, and a failure to list them for a minute.
func (a *Archive) connTools(ctx context.Context, c generated.ArchiveConnection) ([]*mcp.Tool, error) {
	a.conns.mu.Lock()
	t := a.conns.tools[c.ID]
	a.conns.mu.Unlock()
	if t != nil && (t.err == nil && time.Since(t.at) < actionToolsTTL || t.err != nil && time.Since(t.at) < actionFailTTL) {
		return t.tools, t.err
	}
	t = &actionTools{at: time.Now()}
	cl, err := a.connClient(ctx, c)
	if err == nil {
		t.tools, err = cl.Tools()
		cl.Close()
	}
	t.err = err
	a.conns.mu.Lock()
	if a.conns.tools == nil {
		a.conns.tools = map[uuid.UUID]*actionTools{}
	}
	a.conns.tools[c.ID] = t
	a.conns.mu.Unlock()
	return t.tools, t.err
}

// actionName is the action of an MCP connector's tool: its name, without the connector's id before it.
func actionName(p *connect.Provider, tool string) string {
	for _, sep := range []string{"-", "_", "."} {
		if s, ok := strings.CutPrefix(tool, p.ID+sep); ok && s != "" {
			return s
		}
	}
	return tool
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// actionError is a failed action, as the script reads it: its message, and what kind of failure, for its
// code.
type actionError struct {
	msg  string
	kind error
}

// errActionInvalid is the kind of an action asked for wrong, or that the service said was, as code invalid.
var errActionInvalid = errors.New("invalid")

func (e *actionError) Error() string { return e.msg }
func (e *actionError) Unwrap() error { return e.kind }

// Action does the action name with input on one of the caller's connections: the one pick names, by account
// or id, or their only one to its connector. clients are the connections' clients this run opened, which it
// closes when it ends.
func (a *Archive) Action(ctx context.Context, name string, input any, pick string, clients map[uuid.UUID]*connect.Client) (any, error) {
	ac, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	fail := func(kind error, format string, args ...any) error {
		return &actionError{msg: "archive.action " + name + ": " + fmt.Sprintf(format, args...), kind: kind}
	}
	id, act, ok := strings.Cut(name, ".")
	if !ok || act == "" {
		return nil, fail(errActionInvalid, `name is "<connector>.<action>", like "gmail.search"`)
	}
	p, err := a.hub.connect.Get(id)
	if err != nil {
		return nil, fail(ErrNotFound, "%v", err)
	}
	all, err := a.memberConnections(ctx, ac.member)
	if err != nil {
		return nil, err
	}
	var mine []generated.ArchiveConnection
	for _, c := range all {
		if c.Provider == id && c.Integration == p.Integration && (pick == "" || pick == c.Account || pick == c.ID.String()) {
			mine = append(mine, c)
		}
	}
	if p.Integration && len(mine) > 0 {
		lvl, groups, err := a.level(ctx)
		if err != nil {
			return nil, err
		}
		if !mayUse(mine[0], lvl, groups) {
			return nil, fail(ErrForbidden, "only editors and owners use the archive's %s%s", p.Name, map[bool]string{true: ", and of the group it is narrowed to", false: ""}[mine[0].Access != nil])
		}
	}
	switch {
	case len(mine) == 0 && pick != "":
		return nil, fail(ErrNotFound, "you have no %s connection %q", p.Name, pick)
	case len(mine) == 0 && p.Integration:
		return nil, fail(ErrNotFound, "this archive has not connected %s; an owner connects it in Archive settings, Integrations", p.Name)
	case len(mine) == 0:
		return nil, fail(ErrNotFound, "you have not connected %s; connect it in Settings, Connectors", p.Name)
	case len(mine) > 1:
		accounts := make([]string, len(mine))
		for i, c := range mine {
			accounts[i] = strconv.Quote(c.Account)
		}
		return nil, fail(errActionInvalid, "you have %d %s connections, so pass { connection } with one of %s", len(mine), p.Name, strings.Join(accounts, ", "))
	}
	c := mine[0]
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	if string(raw) == "null" {
		raw = []byte("{}")
	}
	var in any
	json.Unmarshal(raw, &in)
	cl := clients[c.ID]
	if cl == nil {
		if cl, err = a.connClient(ctx, c); err != nil {
			return nil, actionFailed(fail, err)
		}
		clients[c.ID] = cl
	}
	if p.MCP == "" {
		do := p.Actions[act]
		if do == nil {
			return nil, fail(ErrNotFound, "%s has no action %s; it has %s", p.Name, act, strings.Join(actionNames(p), ", "))
		}
		if err := actionCheck(name, do.Input, in); err != nil {
			return nil, err
		}
		out, err := do.Do(cl, raw)
		if err != nil {
			return nil, actionFailed(fail, err)
		}
		return out, nil
	}
	tools, err := a.connTools(ctx, c)
	if err != nil {
		return nil, actionFailed(fail, err)
	}
	var tool *mcp.Tool
	var names []string
	for _, t := range tools {
		if actionName(p, t.Name) == act {
			tool = t
		}
		names = append(names, actionName(p, t.Name))
	}
	if tool == nil {
		return nil, fail(ErrNotFound, "%s has no action %s; it has %s", p.Name, act, strings.Join(names, ", "))
	}
	if s := actionSchema(tool.InputSchema); s != nil {
		if err := actionCheck(name, s, in); err != nil {
			return nil, err
		}
	}
	res, err := cl.Call(tool.Name, in)
	if err != nil {
		return nil, actionFailed(fail, err)
	}
	return actionResult(res), nil
}

// actionFailed is err as the action's failure, with the code its kind has: a provider's 404 is not_found, a
// refused grant is unauthorized, and so on.
func actionFailed(fail func(error, string, ...any) error, err error) error {
	var kind error
	var rf *connect.Refusal
	switch {
	case errors.As(err, &rf) && rf.Status == 401 || errors.Is(err, ErrUnauthorized):
		kind = ErrUnauthorized
	case errors.As(err, &rf) && rf.Status == 403:
		kind = ErrForbidden
	case errors.As(err, &rf) && rf.Status == 404 || errors.Is(err, ErrNotFound):
		kind = ErrNotFound
	case errors.As(err, &rf) && rf.Status == 409:
		kind = ErrConflict
	case errors.As(err, &rf) && rf.Status == 400:
		kind = errActionInvalid
	}
	return fail(kind, "%s", strings.TrimPrefix(err.Error(), "connection "))
}

// actionCheck checks input against the action's schema s, as an invalid error naming the action.
func actionCheck(name string, s *jsonschema.Schema, input any) error {
	rs, err := s.Resolve(nil)
	if err != nil {
		return nil
	}
	if err := rs.Validate(input); err != nil {
		msg := err.Error()
		if _, after, ok := strings.Cut(msg, "validating root: "); ok {
			msg = after
		}
		return &ValidationError{Type: "archive.action " + name + " input", Errors: []FieldError{{Message: msg}}}
	}
	return nil
}

// actionSchema is an MCP tool's schema, nil when it does not read as one.
func actionSchema(v any) *jsonschema.Schema {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var s jsonschema.Schema
	if json.Unmarshal(b, &s) != nil {
		return nil
	}
	return &s
}

// actionResult is what a tool's result is in the script: its structured content, or its text, parsed when
// it is JSON.
func actionResult(res *mcp.CallToolResult) any {
	if res.StructuredContent != nil {
		return res.StructuredContent
	}
	text := connect.Text(res)
	var v any
	if json.Unmarshal([]byte(text), &v) == nil {
		return v
	}
	return text
}

func actionNames(p *connect.Provider) []string {
	var out []string
	for n := range p.Actions {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func (r *runner) actionSDK(o *goja.Object) {
	r.def(o, "action", func(ctx context.Context, call goja.FunctionCall) (any, error) {
		name, err := r.str(call, 0, "action", "name")
		if err != nil {
			return nil, err
		}
		var input any
		if err := r.arg(call, 1, "action", "input", &input); err != nil {
			return nil, err
		}
		var opts struct {
			Connection string `json:"connection"`
		}
		if err := r.arg(call, 2, "action", "opts", &opts); err != nil {
			return nil, err
		}
		if r.clients == nil {
			r.clients = map[uuid.UUID]*connect.Client{}
		}
		return r.a.Action(ctx, name, input, opts.Connection, r.clients)
	})
}

// closeClients ends the sessions the run's actions opened.
func (r *runner) closeClients() {
	for _, c := range r.clients {
		c.Close()
	}
}

// actionTypes is the TypeScript of the actions of the caller's connections: the Actions interface, from
// action name to its input and output. A connector the caller has several connections to has its actions
// once, from the first whose server answers.
func (a *Archive) actionTypes(ctx context.Context) string {
	var b strings.Builder
	var conns []generated.ArchiveConnection
	if ac := ActorFrom(ctx); ac != nil {
		var err error
		if conns, err = a.memberConnections(ctx, ac.member); err != nil {
			fmt.Fprintf(os.Stderr, "archive: workspace %s: the actions of %s: %v\n", a.ws.ID, ac.Email, err)
		}
		lvl, groups, _ := a.level(ctx)
		conns = slices.DeleteFunc(conns, func(c generated.ArchiveConnection) bool {
			k := connect.Get(c.Provider)
			return k == nil || k.Integration != c.Integration || c.Integration && !mayUse(c, lvl, groups)
		})
	}
	var yours []string
	for _, c := range conns {
		if p, err := a.hub.connector(c.Provider); err == nil {
			yours = append(yours, p.Name+" "+c.Account)
		}
	}
	intro := "Actions on the accounts you connected, by name, for archive.action (see Connected accounts in the header)"
	if len(yours) == 0 {
		intro += ". You have connected none"
	} else {
		intro += ": " + strings.Join(yours, ", ")
	}
	b.WriteString("\n" + runDoc("", intro) + "interface Actions {\n")
	done := map[string]bool{}
	for _, c := range conns {
		p, err := a.hub.connector(c.Provider)
		if err != nil || done[p.ID] {
			continue
		}
		if p.MCP == "" {
			done[p.ID] = true
			for _, n := range actionNames(p) {
				act := p.Actions[n]
				actionEntry(&b, p.ID+"."+n, act.Doc, act.Input, nil)
			}
			continue
		}
		tools, err := a.connTools(ctx, c)
		if err != nil {
			fmt.Fprintf(os.Stderr, "archive: workspace %s: the tools of %s %s: %v\n", a.ws.ID, p.Name, c.Account, err)
			continue
		}
		done[p.ID] = true
		slices.SortFunc(tools, func(x, y *mcp.Tool) int { return strings.Compare(x.Name, y.Name) })
		for _, t := range tools {
			doc, _, _ := strings.Cut(strings.TrimSpace(t.Description), "\n\n")
			if len(doc) > 600 {
				doc = doc[:600] + "..."
			}
			actionEntry(&b, p.ID+"."+actionName(p, t.Name), doc, t.InputSchema, t.OutputSchema)
		}
	}
	b.WriteString("}\n")
	return b.String()
}

func actionEntry(b *strings.Builder, name, doc string, input, output any) {
	b.WriteString(runDoc("    ", strings.Join(strings.Fields(doc), " ")))
	out := "any"
	if output != nil {
		out = schemaTS(output, "        ", 0)
	}
	fmt.Fprintf(b, "    %s: {\n        input: %s;\n        output: %s;\n    };\n", strconv.Quote(name), schemaTS(input, "        ", 0), out)
}

// schemaTS is the TypeScript type of the JSON Schema v, as far as one says: types, enums, unions, objects
// with their properties' descriptions, arrays and maps, and any for the rest, like a $ref.
func schemaTS(v any, indent string, depth int) string {
	var m map[string]any
	if b, err := json.Marshal(v); err != nil || json.Unmarshal(b, &m) != nil || m == nil || depth > 6 {
		return "any"
	}
	if e, ok := m["enum"].([]any); ok && len(e) > 0 {
		var parts []string
		for _, x := range e {
			j, _ := json.Marshal(x)
			parts = append(parts, string(j))
		}
		return strings.Join(parts, " | ")
	}
	if c, ok := m["const"]; ok {
		j, _ := json.Marshal(c)
		return string(j)
	}
	for _, k := range []string{"anyOf", "oneOf"} {
		if alts, ok := m[k].([]any); ok && len(alts) > 0 {
			var parts []string
			for _, alt := range alts {
				parts = append(parts, schemaTS(alt, indent, depth+1))
			}
			return strings.Join(parts, " | ")
		}
	}
	var types []string
	switch t := m["type"].(type) {
	case string:
		types = []string{t}
	case []any:
		for _, x := range t {
			if s, ok := x.(string); ok {
				types = append(types, s)
			}
		}
	}
	if len(types) == 0 && m["properties"] != nil {
		types = []string{"object"}
	}
	if len(types) == 0 {
		return "any"
	}
	var parts []string
	for _, t := range types {
		switch t {
		case "string", "boolean", "null":
			parts = append(parts, t)
		case "number", "integer":
			parts = append(parts, "number")
		case "array":
			item := schemaTS(m["items"], indent, depth+1)
			if strings.Contains(item, "|") {
				item = "(" + item + ")"
			}
			parts = append(parts, item+"[]")
		case "object":
			parts = append(parts, schemaObject(m, indent, depth))
		default:
			parts = append(parts, "any")
		}
	}
	return strings.Join(parts, " | ")
}

func schemaObject(m map[string]any, indent string, depth int) string {
	props, _ := m["properties"].(map[string]any)
	if len(props) == 0 {
		if ap, ok := m["additionalProperties"].(map[string]any); ok && len(ap) > 0 {
			return "Record<string, " + schemaTS(ap, indent, depth+1) + ">"
		}
		if m["additionalProperties"] == false {
			return "{}"
		}
		return "Record<string, any>"
	}
	var required []string
	if rs, ok := m["required"].([]any); ok {
		for _, r := range rs {
			if s, ok := r.(string); ok {
				required = append(required, s)
			}
		}
	}
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		ri, rj := slices.Contains(required, keys[i]), slices.Contains(required, keys[j])
		if ri != rj {
			return ri
		}
		return keys[i] < keys[j]
	})
	inner := indent + "    "
	var b strings.Builder
	b.WriteString("{\n")
	for _, k := range keys {
		p, _ := props[k].(map[string]any)
		desc, _ := p["description"].(string)
		b.WriteString(runDoc(inner, strings.Join(strings.Fields(desc), " ")))
		opt := "?"
		if slices.Contains(required, k) {
			opt = ""
		}
		fmt.Fprintf(&b, "%s%s%s: %s;\n", inner, runProp(k), opt, schemaTS(p, inner, depth+1))
	}
	b.WriteString(indent + "}")
	return b.String()
}
