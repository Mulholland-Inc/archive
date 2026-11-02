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
// GET /connect lists the connectors and your connections, and DELETE /connect/<connection id> disconnects one.
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
	if p, err = a.hub.provider(r.Context(), p.ID, true); err != nil {
		http.Redirect(w, r, a.hub.app+"/connectors/"+r.PathValue("id")+"?"+url.Values{"workspace": {a.ws.ID.String()}, "error": {err.Error()}}.Encode(), http.StatusSeeOther)
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
	var g *connect.Grant
	if err == nil {
		g, err = p.Exchange(r.Context(), a.hub.connectRedirect(p.ID), r.URL.Query(), st.Verifier)
		if err != nil && p.MCP != "" && strings.Contains(err.Error(), "invalid_client") {
			a.hub.forgetClient(r.Context(), p)
		}
	}
	if err == nil {
		_, err = a.connectionSave(r.Context(), st.Member, p.ID, g)
	}
	if err != nil {
		back.Set("error", err.Error())
	}
	http.Redirect(w, r, a.hub.app+"/connectors/"+st.Provider+"?"+back.Encode(), http.StatusSeeOther)
}

// connectionSave keeps g for member at provider, in the row they have for its account or a new one.
func (a *Archive) connectionSave(ctx context.Context, member uuid.UUID, provider string, g *connect.Grant) (uuid.UUID, error) {
	var id uuid.UUID
	if g.Meta == nil {
		g.Meta = map[string]string{}
	}
	return id, a.sys(ctx, func(ctx context.Context) error {
		q := a.st.db(ctx)
		var err error
		if id, err = q.ConnectionFor(ctx, member, provider, g.Account); errors.Is(err, pgx.ErrNoRows) {
			id, err = coreNewID(), nil
		}
		if err != nil {
			return err
		}
		return q.SaveConnection(ctx, generated.SaveConnectionParams{ID: id, Member: member, Provider: provider, Account: g.Account,
			Token: connect.SealToken(a.aead, id, g.Refresh), Scopes: g.Scopes, Meta: g.Meta})
	})
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
}

// Connector is a service accounts can be connected at; Ready says this server has its OAuth client.
type Connector struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Ready bool   `json:"ready"`
}

type Connections struct {
	Connectors  []Connector  `json:"connectors"`
	Connections []Connection `json:"connections"`
}

// Connections are the connectors and the caller's own connected accounts.
func (a *Archive) Connections(ctx context.Context) (*Connections, error) {
	ac, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	out := &Connections{Connectors: []Connector{}, Connections: []Connection{}}
	for _, c := range connect.All() {
		out.Connectors = append(out.Connectors, Connector{ID: c.ID, Name: c.Name, Ready: a.hub.connect[c.ID] != nil})
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

// Disconnect deletes the caller's connection id, and revokes its refresh token at the provider where it can.
// A revoke that fails leaves the grant at the provider, where the person can still remove it; the connection
// is gone either way.
func (a *Archive) Disconnect(ctx context.Context, id uuid.UUID) error {
	ac, err := authUser(ctx)
	if err != nil {
		return err
	}
	var c generated.ArchiveConnection
	err = a.sys(ctx, func(ctx context.Context) (err error) {
		c, err = a.st.db(ctx).DeleteConnection(ctx, id, ac.member)
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("connection %s: %w", id, ErrNotFound)
	}
	if err != nil {
		return err
	}
	a.conns.forget(c.ID)
	p, err := a.hub.provider(ctx, c.Provider, false)
	var rt string
	if err == nil {
		rt, err = connect.OpenToken(a.aead, c.ID, c.Token)
	}
	if err == nil {
		err = p.RevokeToken(context.WithoutCancel(ctx), rt)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "archive: workspace %s: disconnected %s, but did not revoke it: %v\n", a.ws.ID, id, err)
	}
	return nil
}

// connectionToken is a fresh access token for the connection id; each call refreshes, so code that uses an
// account calls connectionAccess, which keeps one while it lasts. It refreshes holding the connection's row,
// so processes refreshing at once take turns, and each refreshes with the refresh token the one before kept:
// providers that replace the refresh token on every refresh refuse an old one, some revoking the grant. A
// refresh that fails is the connection's error until one works.
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
