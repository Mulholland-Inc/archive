package main

// Connections: the rows of archive.connection, each one of a member's own accounts that a connector of the
// connect package connected. This is the routes and the rows; connect has the connectors and OAuth.
//
// GET /connect/<id> sends the browser to the provider with a state naming the workspace, the user and their
// member, sealed with ARCHIVE_SECRET_KEY. Its callback, GET /connect/<id>/callback, checks the same person is signed in, keeps the
// grant's refresh token sealed with the workspace key, and sends the browser back to the connector in the app.
// Only Go writes the rows; a member reads their own. connectionToken is how code that reads an account (a
// sync, later) gets an access token for it.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
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

// connectStart sends the browser to consent to the connector in the path, for whoever is signed in.
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
	p, err := a.hub.connector(st.Provider)
	var g *connect.Grant
	if err == nil {
		g, err = p.Exchange(r.Context(), a.hub.connectRedirect(p.ID), r.URL.Query(), st.Verifier)
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
			Token: connect.SealToken(a.aead, id, g.Refresh), Scopes: g.Scopes})
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
	p, err := a.hub.connector(c.Provider)
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

// connectionToken is a fresh access token for the connection id, for code that reads the account it connects;
// keep it while it lasts, since each call refreshes. A refresh that fails is the connection's error until one
// works, and a refresh token the provider replaces is kept.
func (a *Archive) connectionToken(ctx context.Context, id uuid.UUID) (*oauth2.Token, error) {
	var c generated.ArchiveConnection
	err := a.st.System(ctx, func(ctx context.Context) (err error) {
		c, err = a.st.db(ctx).Connection(ctx, id)
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("connection %s: %w", id, ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	p, err := a.hub.connector(c.Provider)
	if err != nil {
		return nil, err
	}
	rt, err := connect.OpenToken(a.aead, c.ID, c.Token)
	if err != nil {
		return nil, err
	}
	tok, err := p.Refresh(ctx, rt)
	if err != nil {
		msg := fmt.Sprintf("%s refused to let Archive keep reading %s (%s); connect it again", p.Name, c.Account, connect.Reason(err))
		if serr := a.st.System(ctx, func(ctx context.Context) error { return a.st.db(ctx).SetConnectionError(ctx, id, msg) }); serr != nil {
			return nil, serr
		}
		return nil, fmt.Errorf("connection %s: %s: %w", id, msg, ErrUnauthorized)
	}
	if c.Error != "" || tok.RefreshToken != rt {
		if err := a.st.System(ctx, func(ctx context.Context) error {
			return a.st.db(ctx).SetConnectionToken(ctx, id, connect.SealToken(a.aead, id, tok.RefreshToken))
		}); err != nil {
			return nil, err
		}
	}
	return tok, nil
}
