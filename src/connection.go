package main

// Connections: the rows of archive.connection, each one of a member's own accounts that a connector of the
// connect package connected. This is the routes and the rows; connect has the connectors and OAuth.
//
// GET /connect/<id> sends the browser to the provider with a state naming the workspace, the user and their
// member, sealed with ARCHIVE_SECRET_KEY; for an MCP connector it first registers Archive's own client at its
// authorization server, once for every workspace, and keeps it in archive.oauth_client of the main database.
// Its callback, GET /connect/<id>/callback, checks the same person is signed in, keeps the grant's refresh
// token sealed with the workspace key, and sends the browser back to the connector in the app. Only Go writes
// the rows; a member reads their own. connectionAccess is how code that uses an account (archive.action,
// action.go) gets an access token for it.

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Mulholland-Inc/archive/connect"
	"github.com/Mulholland-Inc/archive/generated"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/oauth2"
)

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
// GET /connect lists the connectors and your connections, DELETE /connect/<connection id> disconnects one, and
// POST /connect/<connection id>/sync asks for its sync now (sync.go), and PUT /connect/<connection id>/access
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
func (a *Archive) level(ctx context.Context) (string, []string, error) {
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
// owners. Everyone who sees its records reads them; acting on the archive's account writes, as an editor does.
func mayUse(c generated.ArchiveConnection, lvl string, groups []string) bool {
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
	// Records how many of its records are not deleted.
	SyncAt   *time.Time `json:"sync_at"`
	SyncedAt *time.Time `json:"synced_at"`
	Records  int64      `json:"records"`
	// Integration says it is the archive's; Access is the group it is narrowed to, "" for everyone; and
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

// Disconnect deletes the caller's connection id, or, for an owner, the archive's integration id, with its
// records, and revokes its refresh token at the provider where it can. A revoke that fails leaves the grant at
// the provider, where the person can still remove it; the connection is gone either way.
func (a *Archive) Disconnect(ctx context.Context, id uuid.UUID) error {
	c, err := a.connectionMine(ctx, id)
	if err != nil {
		return err
	}
	err = a.sys(ctx, func(ctx context.Context) (err error) {
		c, err = a.st.db(ctx).DeleteConnection(ctx, c.ID)
		return err
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

// SetAccess narrows the archive's integration id, and its records, to the members of group, or opens them to
// everyone with "", as an owner does.
func (a *Archive) SetAccess(ctx context.Context, id uuid.UUID, group string) error {
	c, err := a.connectionMine(ctx, id)
	switch {
	case err != nil:
		return err
	case !c.Integration:
		return fmt.Errorf("only an integration is narrowed to a group; a personal connection is its owner's alone: %w", httpErrBadRequest)
	case group != "" && !connectionGroupRe.MatchString(group):
		return fmt.Errorf("access: want a group of 1-24 lowercase letters, digits or _, like finance: %w", httpErrBadRequest)
	}
	var access *string
	if group != "" {
		access = &group
	}
	return a.sys(ctx, func(ctx context.Context) error { return a.st.db(ctx).SetConnectionAccess(ctx, access, id) })
}

var connectionGroupRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_]{0,23}$`)

// connectionUpgrade makes the first connection of each integration from before there were integrations the
// archive's; any other stays its member's own, as it was.
func (a *Archive) connectionUpgrade(ctx context.Context) error {
	var ids []string
	for _, c := range connect.All() {
		if c.Integration {
			ids = append(ids, c.ID)
		}
	}
	return a.sys(ctx, func(ctx context.Context) error {
		_, err := a.st.db(ctx).UpgradeIntegrations(ctx, ids)
		return err
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
