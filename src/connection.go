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
	"math/rand/v2"
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

func (h *Hub) connector(id string) (*connect.Provider, error) {
	p, err := h.connect.Get(id)
	if err != nil {
		return nil, fmt.Errorf("%v: %w", err, ErrNotFound)
	}
	return p, nil
}

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

func (h *Hub) forgetClient(ctx context.Context, p *connect.Provider) {
	if sealed, err := h.q().OAuthClient(ctx, p.ID); err == nil {
		if r, err := connect.OpenClient(h.aead, p.ID, sealed); err == nil && r.ClientID == p.ClientID() {
			h.q().DeleteOAuthClient(ctx, p.ID, sealed)
		}
	}
}

func (h *Hub) connectRedirect(id string) string { return h.api + "/connect/" + id + "/callback" }

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
	authed("POST /connect/{id}/keys", func(a *Archive, w http.ResponseWriter, r *http.Request) {
		var body struct {
			Keys map[string]string `json:"keys"`
		}
		err := httpDecode(r, &body)
		if err == nil {
			err = a.connectKeys(r.Context(), r.PathValue("id"), body.Keys)
		}
		httpWrite(w, http.StatusNoContent, nil, err)
	})
	authed("POST /connect/{id}/link", func(a *Archive, w http.ResponseWriter, r *http.Request) {
		tok, err := a.connectLink(r.Context(), r.PathValue("id"))
		httpWrite(w, http.StatusOK, map[string]string{"token": tok}, err)
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
	if err = a.mayConnect(r.Context(), p.Connector); err == nil && len(p.Keys) > 0 {
		err = fmt.Errorf("%s connects with keys, not by signing in", p.Name)
	}
	if err == nil {
		p, err = a.hub.provider(r.Context(), p.ID, true)
	}
	if err == nil && !p.Ready() {
		err = fmt.Errorf("%s is not set up on this server; it needs %s", p.Name, p.Env)
	}
	var asked map[string]string
	if err == nil && p.Ask != nil {
		asked, err = p.AskOf(r.URL.Query().Get(p.Ask.Name))
	}
	st := connect.State{Provider: r.PathValue("id"), Workspace: a.ws.ID.String(), User: ac.UserID, Member: ac.member,
		Verifier: oauth2.GenerateVerifier(), Exp: time.Now().Add(connect.TTL).Unix(), Meta: asked}
	var to string
	if err == nil {
		to, err = p.AuthURL(a.hub.connectRedirect(p.ID), connect.SealState(a.hub.aead, st), st.Verifier, asked)
	}
	if err != nil {
		http.Redirect(w, r, a.hub.app+connectBack(r.PathValue("id"))+"?"+url.Values{"workspace": {a.ws.ID.String()}, "error": {err.Error()}}.Encode(), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, to, http.StatusFound)
}

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
		g, err = p.Exchange(r.Context(), a.hub.connectRedirect(p.ID), r.URL.Query(), st.Verifier, st.Meta)
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

func (a *Archive) connectKeys(ctx context.Context, id string, keys map[string]string) error {
	ac, err := authUser(ctx)
	if err != nil {
		return err
	}
	p, err := a.hub.connector(id)
	if err != nil {
		return err
	}
	if len(p.Keys) == 0 {
		return fmt.Errorf("%s connects by signing in, not with keys: %w", p.Name, httpErrBadRequest)
	}
	if err := a.mayConnect(ctx, p.Connector); err != nil {
		return err
	}
	g, err := p.KeyGrant(ctx, keys)
	if err != nil {
		return fmt.Errorf("%w: %w", err, httpErrBadRequest)
	}
	if _, err = a.connectionSave(ctx, ac.member, p.Connector, g); err != nil && p.Link != nil {
		p.RevokeToken(ctx, g.Refresh, g.Meta)
	}
	return err
}

func (a *Archive) connectLink(ctx context.Context, id string) (string, error) {
	ac, err := authUser(ctx)
	if err != nil {
		return "", err
	}
	p, err := a.hub.connector(id)
	if err != nil {
		return "", err
	}
	if p.Link == nil {
		return "", fmt.Errorf("%s has no widget to connect with: %w", p.Name, httpErrBadRequest)
	}
	if err := a.mayConnect(ctx, p.Connector); err != nil {
		return "", err
	}
	if p.Integration {
		err = a.sys(ctx, func(ctx context.Context) error {
			had, err := a.st.db(ctx).IntegrationFor(ctx, p.ID)
			if err == nil {
				return fmt.Errorf("this archive's %s is %s; disconnect it before connecting another: %w", p.Name, had.Account, ErrConflict)
			}
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		})
		if err != nil {
			return "", err
		}
	}
	return p.LinkToken(ctx, ac.member.String(), a.hub.api+"/push/"+p.ID, a.hub.app+connectBack(p.ID))
}

func connectBack(id string) string {
	if c := connect.Get(id); c != nil && c.Integration {
		return "/settings/integrations/" + id
	}
	return "/settings/connectors/" + id
}

func (a *Archive) level(ctx context.Context) (string, *uuid.UUID, error) {
	ac, err := signedIn(ctx)
	if err != nil {
		return "", nil, err
	}
	m, err := a.memberOf(ctx, ac.Email)
	if err != nil {
		return "", nil, err
	}
	if ac.Role != ac.UserRole {
		return "viewer", m.Group, nil
	}
	return m.Role, m.Group, nil
}

func mayManage(lvl string) bool {
	return lvl == "manager" || lvl == "engineer" || lvl == "owner"
}

func (a *Archive) mayConnect(ctx context.Context, c *connect.Connector) error {
	if _, err := authUser(ctx); err != nil || !c.Integration {
		return err
	}
	if lvl, _, err := a.level(ctx); err != nil || !mayManage(lvl) {
		return cmp.Or(err, fmt.Errorf("only managers, engineers and the owner connect %s, for the whole archive: %w", c.Name, ErrForbidden))
	}
	return nil
}

func mayUse(c generated.ArchiveConnection, lvl string, group *uuid.UUID) bool {
	return mayManage(lvl) || lvl == "editor" && (c.Access == nil || group != nil && *group == *c.Access)
}

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
		if _, err := a.syncWant(ctx, id); err != nil {
			fmt.Fprintf(os.Stderr, "archive: workspace %s: queueing a sync: %v\n", a.ws.ID, err)
		}
	}
	return id, err
}

type Connection struct {
	ID          uuid.UUID  `json:"id"`
	Provider    string     `json:"provider"`
	Account     string     `json:"account"`
	Scopes      []string   `json:"scopes"`
	Error       string     `json:"error"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	SyncAt      *time.Time `json:"sync_at"`
	SyncedAt    *time.Time `json:"synced_at"`
	Items       int64      `json:"items"`
	Integration bool       `json:"integration"`
	Access      string     `json:"access"`
	ConnectedBy string     `json:"connected_by"`
}

type Connector struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Ready       bool          `json:"ready"`
	Integration bool          `json:"integration"`
	Keys        []connect.Key `json:"keys,omitempty"`
	Ask         *connect.Key  `json:"ask,omitempty"`
	Link        bool          `json:"link,omitempty"`
	connect.Info
}

type Connections struct {
	Connectors  []Connector  `json:"connectors"`
	Connections []Connection `json:"connections"`
}

func (a *Archive) Connections(ctx context.Context) (*Connections, error) {
	ac, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	out := &Connections{Connectors: []Connector{}, Connections: []Connection{}}
	for _, c := range connect.All() {
		out.Connectors = append(out.Connectors, Connector{ID: c.ID, Name: c.Name, Ready: a.hub.connect[c.ID] != nil && a.hub.connect[c.ID].Ready(), Integration: c.Integration, Keys: c.Keys, Ask: c.Ask, Link: c.Link != nil, Info: c.Info})
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

type OwnConnection struct {
	Connection
	Workspace Workspace `json:"workspace"`
}

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

func (a *Archive) Disconnect(ctx context.Context, id uuid.UUID) error {
	c, err := a.connectionMine(ctx, id)
	if err != nil {
		return err
	}
	return a.disconnect(ctx, c)
}

func (a *Archive) disconnect(ctx context.Context, c generated.ArchiveConnection) error {
	id := c.ID
	err := a.sys(ctx, func(ctx context.Context) (err error) {
		q := a.st.db(ctx)
		if c, err = q.DeleteConnection(ctx, c.ID); err != nil {
			return err
		}
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
		var cl *connect.Client
		if push, _ := p.PushOf(); push == nil || push.Unwatch == nil {
		} else if tok, terr := p.Refresh(ctx, rt, c.Meta); terr == nil {
			cl = p.Open(ctx, tok, c.Meta)
			rt = tok.RefreshToken
		}
		if werr := a.unwatch(ctx, c, p, cl); werr != nil {
			fmt.Fprintf(os.Stderr, "archive: workspace %s: disconnected %s, but did not end all it pushed: %v\n", a.ws.ID, id, werr)
		}
		if cl != nil {
			cl.Close()
		}
		var shared bool
		if shared, err = a.grantShared(ctx, c, p); err == nil && !shared {
			err = p.RevokeToken(ctx, rt, c.Meta)
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "archive: workspace %s: disconnected %s, but did not revoke it: %v\n", a.ws.ID, id, err)
	}
	return nil
}

func (a *Archive) grantShared(ctx context.Context, c generated.ArchiveConnection, p *connect.Provider) (shared bool, err error) {
	err = a.sys(ctx, func(ctx context.Context) error {
		rows, err := a.st.db(ctx).MemberConnections(ctx, c.Member)
		for _, r := range rows {
			o := connect.Get(r.Provider)
			if o == nil || r.Member != c.Member || o.ID != p.ID && (p.Env == "" || o.Env != p.Env) {
				continue
			}
			shared = shared || r.Account == c.Account || r.Account == o.Name || c.Account == p.Name
		}
		return err
	})
	return shared, err
}

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
		if lvl, _, err := a.level(ctx); err != nil || !mayManage(lvl) {
			return c, cmp.Or(err, fmt.Errorf("only managers, engineers and the owner change the archive's %s: %w", c.Provider, ErrForbidden))
		}
	}
	return c, nil
}

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
		if tok, err = p.Refresh(ctx, rt, c.Meta); err != nil {
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

const (
	syncPageTime   = 2 * time.Minute
	syncLease      = syncPageTime + time.Minute
	syncLeaseCheck = 15 * time.Second
	syncBudget     = 2 * time.Minute
	syncRetry      = time.Minute
	syncTries      = 5
	syncBusyWait   = 15 * time.Minute
	syncMaxPages   = 100_000
	syncPoll       = time.Hour
	syncBackstop   = 24 * time.Hour
	syncRename     = 24 * time.Hour
)

var errRevoked = errors.New("revoked")

type syncStream struct {
	Cursor string     `json:"cursor,omitempty"`
	At     *time.Time `json:"at,omitempty"`
	Last   *time.Time `json:"last,omitempty"`
	More   bool       `json:"more,omitempty"`
	Again  bool       `json:"again,omitempty"`
	Since  *time.Time `json:"since,omitempty"`
	Skip   string     `json:"skip,omitempty"`
	Pages  int        `json:"pages,omitempty"`
	Fails  int        `json:"fails,omitempty"`
	Error  string     `json:"error,omitempty"`
	Def    string     `json:"def,omitempty"`
	Named  *time.Time `json:"named,omitempty"`
}

func (v *syncStream) due(s *connect.Stream, now time.Time) bool {
	return !v.current(s) || v.At == nil || !v.At.After(now)
}

func (v *syncStream) settled(s *connect.Stream, now time.Time) bool {
	return !v.due(s, now) && !v.More && !v.Again && v.Fails == 0
}

func (v *syncStream) floor(s *connect.Stream, now time.Time) time.Time {
	if v.Last != nil && v.Last.Add(s.Every).After(now) {
		return v.Last.Add(s.Every)
	}
	return now
}

func syncEvery(p *connect.Provider, s *connect.Stream) time.Duration {
	if s.Every > 0 {
		return s.Every
	}
	if push, target := p.PushOf(); push != nil && push.Watch != nil && (push.Env == "" || target != "") {
		return syncBackstop
	}
	return syncPoll
}

func syncLater(p *connect.Provider, s *connect.Stream, now time.Time) *time.Time {
	every := syncEvery(p, s)
	return ptr(now.Add(every + rand.N(every/10)))
}

func (v *syncStream) current(s *connect.Stream) bool { return v.Def == s.Version }

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

func syncPick(streams []*connect.Stream, st syncState, now time.Time) *connect.Stream {
	var pick *connect.Stream
	var at time.Time
	for _, s := range streams {
		v := st.of(s.Name)
		if !v.due(s, now) {
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

func (a *Archive) syncPage(ctx context.Context, c generated.ArchiveConnection) error {
	p, gone := a.hub.connect.Get(c.Provider)
	var streams []*connect.Stream
	if gone == nil {
		streams = p.Streams
	}
	s := syncPick(streams, syncStateOf(c.Cursor), time.Now())
	var page *connect.Page
	var err error
	var named bool
	var account string
	if s != nil {
		cur := syncStateOf(c.Cursor).of(s.Name).fresh(s)
		pctx, cancel := context.WithTimeout(ctx, syncPageTime)
		var cl *connect.Client
		if cl, err = a.connClient(pctx, c); err == nil {
			cl.Store = a.syncStore(pctx, c.ID, c.Access)
			cl.Mail = a.syncMail(pctx, c)
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
		if next != nil {
			if err := a.soon(ctx, "sync", *next); err != nil {
				fmt.Fprintf(os.Stderr, "archive: workspace %s: queueing a sync: %v\n", a.ws.ID, err)
			}
		}
	}()
	var wrote bool
	err = a.sys(context.WithoutCancel(ctx), func(ctx context.Context) error {
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
			save.SyncAt = ptr(now.Add(syncBackstop))
			next = save.SyncAt
			return q.SaveSync(ctx, save)
		case errors.Is(err, errRevoked):
			save.Revoked = true
			return q.SaveSync(ctx, save)
		}
		cur, name := &syncStream{}, ""
		if s != nil {
			cur, name = st.of(s.Name).fresh(s), s.Name
			cur.Def, st[s.Name] = s.Version, cur
		}
		var unsup *connect.Unsupported
		switch {
		case s == nil:
		case errors.As(err, &unsup):
			cur.Skip, cur.More, cur.Again, cur.Since, cur.Fails, cur.Error, cur.Pages = unsup.Reason, false, false, nil, 0, "", 0
			cur.At = syncLater(p, s, now)
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
				cur.At = ptr(now.Add(max(min(syncRetry<<min(cur.Fails-1, 16), syncBusyWait), connect.RetryAfter(err))))
				if cur.Fails < syncTries {
					cur.Error = ""
				}
			}
			if cur.Fails >= syncTries && !busy {
				cur.At, cur.Again, cur.Fails = syncLater(p, s, now), false, 0
			}
		default:
			if (c.Access == nil) != (c2.Access == nil) || c.Access != nil && *c.Access != *c2.Access {
				if _, err := a.st.Q(ctx).Exec(ctx, "UPDATE archive.file SET access = $2 WHERE connection = $1 AND access IS DISTINCT FROM $2", c.ID, c2.Access); err != nil {
					return err
				}
			}
			w := &syncDB{q: q, db: a.st.Q(ctx), id: c.ID, acc: c2.Access, now: now}
			if err := syncSave(ctx, w, s, page, cur); err != nil {
				return err
			}
			wrote = len(page.Records) > 0
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
			switch {
			case page.More:
				cur.At = ptr(now.Add(s.Gap))
			case cur.Again:
				cur.Last, cur.Pages, cur.Again = &now, 0, false
				cur.At = ptr(cur.floor(s, now.Add(s.Gap)))
			default:
				cur.Last, cur.Pages = &now, 0
				cur.At = syncLater(p, s, now)
			}
		}
		var errs []string
		settled := true
		for _, d := range streams {
			v := st.of(d.Name)
			if v.Error != "" {
				errs = append(errs, v.Error)
			}
			settled = settled && v.settled(d, now)
			at := now
			if v.current(d) && v.At != nil {
				at = *v.At
			}
			if save.SyncAt == nil || at.Before(*save.SyncAt) {
				save.SyncAt = &at
			}
		}
		for k := range st {
			if !slices.ContainsFunc(streams, func(d *connect.Stream) bool { return d.Name == k }) {
				delete(st, k)
				w := &syncDB{q: q, db: a.st.Q(ctx), id: c.ID, now: now}
				if err := w.stream(ctx, k); err != nil {
					return err
				}
			}
		}
		if settled && len(errs) == 0 {
			save.SyncedAt = &now
		}
		save.Error = strings.Join(errs, "; ")
		save.Cursor, _ = json.Marshal(st)
		next = save.SyncAt
		return q.SaveSync(ctx, save)
	})
	if err == nil && wrote {
		a.wakeTables(context.WithValue(context.WithoutCancel(ctx), storeTxKey{}, nil))
	}
	return err
}

func (v *syncStream) fresh(s *connect.Stream) *syncStream {
	if v.current(s) {
		return v
	}
	return &syncStream{Named: v.Named}
}

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
		live = append(live, syncRec{ID: r.ID, Data: b})
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
			v.At, v.Fails, v.Last, v.Again = nil, 0, nil, false
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

func (a *Archive) syncWant(ctx context.Context, id uuid.UUID) (bool, error) {
	ctx = context.WithoutCancel(ctx)
	var found bool
	var next *time.Time
	err := a.sys(ctx, func(ctx context.Context) error {
		q := a.st.db(ctx)
		c, err := q.ConnectionLocked(ctx, id)
		if err != nil {
			return err
		}
		found = true
		p, gone := a.hub.connect.Get(c.Provider)
		if gone != nil || c.Revoked {
			return nil
		}
		now, err := q.TxNow(ctx)
		if err != nil {
			return err
		}
		st := syncStateOf(c.Cursor)
		for _, s := range p.Streams {
			v := st.of(s.Name)
			if v.settled(s, now) {
				v.At = ptr(v.floor(s, now))
			} else {
				v.Again = true
			}
			at := now
			if v.current(s) && v.At != nil {
				at = *v.At
			}
			if next == nil || at.Before(*next) {
				next = &at
			}
		}
		cursor, _ := json.Marshal(st)
		return q.WantSync(ctx, cursor, next, id)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return found, nil
	}
	if err != nil || next == nil {
		return found, err
	}
	return true, a.soon(ctx, "sync", *next)
}

func (a *Archive) streamTypes(ctx context.Context) string {
	var b strings.Builder
	var conns []generated.ArchiveConnection
	if ac := ActorFrom(ctx); ac != nil {
		conns, _ = a.memberConnections(ctx, ac.member)
	}
	if len(conns) == 0 {
		return ""
	}
	b.WriteString("\n" + runDoc("", `What Archive syncs of the accounts you connected, by "<connector>.<stream>", and the table each lands in: its rows' stream column is the part after the dot`) + "interface Streams {\n")
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

func (a *Archive) syncShown(ctx context.Context, cat *Catalog) map[string]bool {
	show := map[string]bool{}
	var walk func(name string)
	walk = func(name string) {
		t := cat.Tables[name]
		if t == nil || show[name] {
			return
		}
		show[name] = true
		for child, ct := range syncSchema {
			if ct.parent == name {
				walk(child)
			}
		}
		for _, c := range t.Columns {
			walk(strings.TrimPrefix(strings.TrimPrefix(c.target(), "public."), "archive."))
		}
	}
	for _, name := range cat.names() {
		if t := cat.Tables[name]; !t.synced() || name == "file" {
			show[name] = true
		}
	}
	var providers []string
	err := a.st.System(ctx, func(ctx context.Context) (err error) {
		providers, err = a.st.db(ctx).ConnectedProviders(ctx)
		return err
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "archive: workspace %s: its connections for the docs: %v\n", a.ws.ID, err)
	}
	if len(providers) > 0 {
		walk("known_person")
		walk("known_org")
	}
	for _, id := range providers {
		if c := connect.Get(id); c != nil {
			for _, st := range c.Streams {
				walk(st.Table)
			}
		}
	}
	for _, name := range cat.names() {
		if t := cat.Tables[name]; t.Schema == "public" {
			for _, c := range t.Columns {
				walk(strings.TrimPrefix(strings.TrimPrefix(c.target(), "public."), "archive."))
			}
		}
	}
	return show
}

type syncTable struct {
	parent, of string
	links      map[string]string
}

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
	"file":                             {links: map[string]string{"parent": "file"}},
	"calendar_event":                   {links: map[string]string{"series": "calendar_event"}},
	"calendar_attendee":                {parent: "calendar_event", of: "event"},
	"crm_company":                      {},
	"crm_contact":                      {links: map[string]string{"company": "crm_company"}},
	"crm_deal":                         {links: map[string]string{"company": "crm_company"}},
	"chat_message":                     {},
	"marketing_campaign":               {},
	"work_item":                        {},
	"code_project":                     {},
	"compliance":                       {},
	"commerce_product":                 {},
	"commerce_order":                   {},
	"dental_provider":                  {},
	"dental_patient":                   {links: map[string]string{"provider": "dental_provider"}},
	"dental_appointment":               {links: map[string]string{"patient": "dental_patient", "provider": "dental_provider", "hygienist": "dental_provider"}},
	"dental_procedure":                 {links: map[string]string{"patient": "dental_patient", "appointment": "dental_appointment", "provider": "dental_provider"}},
	"dental_payment":                   {links: map[string]string{"patient": "dental_patient"}},
	"ad_metric":                        {links: map[string]string{"campaign": "marketing_campaign"}},
	"commerce_line":                    {parent: "commerce_order", of: "purchase", links: map[string]string{"product": "commerce_product"}},
	"ledger_account":                   {links: map[string]string{"parent": "ledger_account"}},
	"ledger_contact":                   {},
	"ledger_item":                      {links: map[string]string{"income_account": "ledger_account", "expense_account": "ledger_account"}},
	"ledger_entry":                     {links: map[string]string{"contact": "ledger_contact", "account": "ledger_account"}},
	"ledger_line":                      {parent: "ledger_entry", of: "entry", links: map[string]string{"account": "ledger_account", "item": "ledger_item"}},
	"bank_account":                     {},
	"bank_transaction":                 {links: map[string]string{"account": "bank_account"}},
	"spend_department":                 {},
	"spend_user":                       {links: map[string]string{"department": "spend_department", "manager": "spend_user"}},
	"spend_card":                       {links: map[string]string{"holder": "spend_user"}},
	"spend_transaction":                {links: map[string]string{"spender": "spend_user", "card": "spend_card", "department": "spend_department"}},
	"spend_receipt":                    {parent: "spend_transaction", of: "spend"},
	"warehouse_table":                  {},
	"warehouse_column":                 {parent: "warehouse_table", of: "relation"},
	"investment_holding":               {links: map[string]string{"account": "bank_account"}},
	"investment_transaction":           {links: map[string]string{"account": "bank_account"}},
	"bank_liability":                   {links: map[string]string{"account": "bank_account"}},
	"budget_category":                  {},
	"parcel":                           {},
	"parcel_event":                     {parent: "parcel", of: "parcel"},
	"vehicle":                          {},
	"vehicle_charge":                   {links: map[string]string{"vehicle": "vehicle"}},
	"home_device":                      {},
	"health_sample":                    {},
	"health_day":                       {},
	"health_sleep":                     {},
	"health_workout":                   {},
	"health_record":                    {},
	"music_playlist":                   {},
	"music_track":                      {links: map[string]string{"playlist": "music_playlist"}},
	"music_artist":                     {},
	"school_notice":                    {},
}

type syncRec struct {
	ID   string
	Data json.RawMessage
}

type syncDB struct {
	q   *generated.Queries
	db  generated.DBTX
	id  uuid.UUID
	acc *uuid.UUID
	now time.Time
}

func (w *syncDB) write(ctx context.Context, s *connect.Stream, recs []syncRec) error {
	t, ok := syncSchema[s.Table]
	if !ok || t.parent != "" {
		return fmt.Errorf("%s lands in %q, which is no table of archive.sql a stream lands in", s.Name, s.Table)
	}
	rows, err := w.mapped(ctx, s, recs)
	if err != nil {
		return err
	}
	kept := recs
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
		o["stream"], o["source_id"], o["seen_at"], o["created_at"], o["updated_at"] = s.Name, kept[i].ID, w.now, w.now, w.now
		objs[i] = o
	}
	cols := syncCols(objs)
	set := []string{}
	var compare []string
	for _, c := range cols {
		if c == "connection" || c == "stream" || c == "source_id" || c == "created_at" || c == "updated_at" {
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

func (w *syncDB) mapped(ctx context.Context, s *connect.Stream, recs []syncRec) ([]*connect.Row, error) {
	data := make([]json.RawMessage, len(recs))
	for i, r := range recs {
		data[i] = r.Data
	}
	res, err := w.db.Query(ctx, s.SQL, jsonOf(data))
	if err != nil {
		return nil, fmt.Errorf("the map of %s: %w", s.Name, err)
	}
	defer res.Close()
	rows := make([]*connect.Row, 0, len(recs))
	for res.Next() {
		var b []byte
		if err := res.Scan(&b); err != nil {
			return nil, err
		}
		d := json.NewDecoder(bytes.NewReader(b))
		d.UseNumber()
		var row connect.Row
		if err := d.Decode(&row); err != nil {
			return nil, fmt.Errorf("the map of %s: %w", s.Name, err)
		}
		rows = append(rows, &row)
	}
	if err := res.Err(); err != nil {
		return nil, fmt.Errorf("the map of %s: %w", s.Name, err)
	}
	return rows, nil
}

func syncCheck() error {
	cols := map[string][]string{}
	for _, m := range syncTableRe.FindAllStringSubmatch(storeSchema, -1) {
		for _, line := range strings.Split(m[2], "\n") {
			if f := strings.Fields(line); len(f) > 1 && syncColRe.MatchString(f[0]) {
				cols[m[1]] = append(cols[m[1]], f[0])
			}
		}
	}
	has := func(table string, sets ...map[string]connect.SQL) error {
		for _, set := range sets {
			for c := range set {
				if !slices.Contains(cols[table], c) {
					return fmt.Errorf("%s has no column %s", table, c)
				}
			}
		}
		return nil
	}
	for _, c := range connect.All() {
		for _, s := range c.Sync {
			fail := func(err error) error { return fmt.Errorf("connector %s, stream %s: %w", c.ID, s.Name, err) }
			t, ok := syncSchema[s.To.Table]
			if !ok || t.parent != "" {
				return fail(fmt.Errorf("%q is no table of archive.sql a stream lands in", s.To.Table))
			}
			if err := has(s.To.Table, s.To.Cols, s.To.People, s.To.Orgs, s.To.EINs, s.To.Refs); err != nil {
				return fail(err)
			}
			for k := range s.To.Refs {
				if t.links[k] == "" {
					return fail(fmt.Errorf("%s of %s links no table", k, s.To.Table))
				}
			}
			for _, k := range s.To.Children {
				ct, ok := syncSchema[k.Table]
				if !ok || ct.parent != s.To.Table {
					return fail(fmt.Errorf("%q is no table of the rows of %s", k.Table, s.To.Table))
				}
				if err := has(k.Table, k.Cols, k.People, k.Orgs, k.Refs); err != nil {
					return fail(err)
				}
			}
		}
	}
	return nil
}

var syncTableRe = regexp.MustCompile(`(?ms)^CREATE TABLE archive\.(\w+) \((.*?)^\)`)

func syncCheckSQL(ctx context.Context, db generated.DBTX) error {
	for _, c := range connect.All() {
		for _, s := range c.Streams {
			if _, err := db.Exec(ctx, "EXPLAIN "+s.SQL, []byte("[]")); err != nil {
				return fmt.Errorf("connector %s, stream %s: its map: %w", c.ID, s.Name, err)
			}
		}
	}
	return nil
}

func (w *syncDB) object(table string, row *connect.Row, people, orgs map[string]uuid.UUID) map[string]any {
	o := map[string]any{}
	for col, v := range row.Cols {
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

func (w *syncDB) known(ctx context.Context, rows []*connect.Row) (map[string]uuid.UUID, map[string]uuid.UUID, error) {
	var emails, eins, domains []string
	for _, r := range rows {
		for _, x := range append([]connect.Row{*r}, children(r)...) {
			emails = append(emails, slices.Collect(maps.Values(x.People))...)
			for _, k := range x.Orgs {
				if strings.Contains(k, ".") {
					domains = append(domains, k)
				} else {
					eins = append(eins, k)
				}
			}
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
	if len(domains) > 0 {
		got, err := w.q.KnownDomains(ctx, domains)
		if err != nil {
			return nil, nil, err
		}
		for _, o := range got {
			orgs[o.Domain] = o.ID
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

func (w *syncDB) see(ctx context.Context, s *connect.Stream, ids []string) error {
	_, err := w.db.Exec(ctx, "UPDATE archive."+s.Table+" SET seen_at = $4 WHERE connection = $1 AND stream = $2 AND source_id = ANY($3)", w.id, s.Name, ids, w.now)
	return err
}

func (w *syncDB) delete(ctx context.Context, s *connect.Stream, ids []string) error {
	return w.gone(ctx, "DELETE FROM archive."+s.Table+" WHERE connection = $1 AND stream = $2 AND source_id = ANY($3)", w.id, s.Name, ids)
}

func (w *syncDB) unseen(ctx context.Context, s *connect.Stream, since time.Time) error {
	return w.gone(ctx, "DELETE FROM archive."+s.Table+" WHERE connection = $1 AND stream = $2 AND seen_at < $3", w.id, s.Name, since)
}

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

func (w *syncDB) gone(ctx context.Context, sql string, args ...any) error {
	tag, err := w.db.Exec(ctx, sql, args...)
	if err != nil || tag.RowsAffected() == 0 {
		return err
	}
	return w.q.DeleteUnknown(ctx)
}

func syncNarrow(ctx context.Context, db generated.DBTX, id uuid.UUID, access *uuid.UUID) error {
	for _, t := range slices.Sorted(maps.Keys(syncSchema)) {
		if _, err := db.Exec(ctx, "UPDATE archive."+t+" SET access = $2 WHERE connection = $1 AND access IS DISTINCT FROM $2", id, access); err != nil {
			return err
		}
	}
	_, err := db.Exec(ctx, "UPDATE archive.file SET access = $2 WHERE connection = $1 AND access IS DISTINCT FROM $2", id, access)
	return err
}

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
			if heard.All || len(heard.Forget) > 0 {
				err = errors.Join(err, h.pushForget(ctx, row.Workspace, row.Connection, heard))
				continue
			}
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

func (h *Hub) pushForget(ctx context.Context, ws, conn uuid.UUID, heard *connect.Heard) error {
	ctx = context.WithoutCancel(ctx)
	a, release, err := h.acquire(ctx, ws.String())
	if errors.Is(err, ErrNotFound) {
		return h.q().DropWatches(ctx, conn, []string{})
	}
	if err != nil {
		return err
	}
	defer release()
	var c generated.ArchiveConnection
	err = a.sys(ctx, func(ctx context.Context) (err error) {
		c, err = a.st.db(ctx).Connection(ctx, conn)
		return err
	})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return h.q().DropWatches(ctx, conn, []string{})
	case err != nil:
		return err
	case heard.All:
		fmt.Fprintf(os.Stderr, "archive: workspace %s: %s asked Archive to forget %s, so it is disconnected\n", ws, c.Provider, c.Account)
		return a.disconnect(ctx, c)
	}
	p := connect.Get(c.Provider)
	if p == nil {
		return nil
	}
	return a.sys(ctx, func(ctx context.Context) error {
		w := &syncDB{q: a.st.db(ctx), db: a.st.Q(ctx), id: c.ID, now: time.Now()}
		for _, s := range p.Streams {
			if ids := heard.Forget[s.Name]; len(ids) > 0 {
				if err := w.delete(ctx, s, ids); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

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

const (
	actionToolsTTL = time.Hour
	actionFailTTL  = time.Minute
)

type archiveConns struct {
	mu     sync.Mutex
	access map[uuid.UUID]*oauth2.Token
	tools  map[uuid.UUID]*actionTools
}

type actionTools struct {
	at    time.Time
	tools []*mcp.Tool
	err   error
}

func (cs *archiveConns) forget(id uuid.UUID) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	delete(cs.access, id)
	delete(cs.tools, id)
}

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

func (a *Archive) memberConnections(ctx context.Context, member uuid.UUID) ([]generated.ArchiveConnection, error) {
	var out []generated.ArchiveConnection
	err := a.st.System(ctx, func(ctx context.Context) (err error) {
		out, err = a.st.db(ctx).MemberConnections(ctx, member)
		return err
	})
	return out, err
}

func (a *Archive) syncStore(ctx context.Context, id uuid.UUID, access *uuid.UUID) func(name string, b []byte) (string, error) {
	return func(name string, b []byte) (string, error) {
		var f *File
		err := a.sys(ctx, func(ctx context.Context) error {
			var err error
			f, err = a.fileCommit(ctx, access, name, nil, &id, func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(b)), nil })
			return err
		})
		if f == nil {
			return "", err
		}
		return f.ID.String(), err
	}
}

func (a *Archive) syncMail(ctx context.Context, c generated.ArchiveConnection) func(s connect.Scan) ([]any, error) {
	return func(s connect.Scan) ([]any, error) {
		var find *regexp.Regexp
		if s.Find != "" {
			var err error
			if find, err = regexp.Compile(s.Find); err != nil {
				return nil, err
			}
		}
		type hit struct {
			subject, text, id, from, name string
			at                            *time.Time
		}
		var hits []hit
		err := a.sys(ctx, func(ctx context.Context) error {
			rows, err := a.st.Q(ctx).Query(ctx, `SELECT coalesce(m.subject, ''), coalesce(m.text, ''), m.source_id, coalesce(m.sent_at, m.received_at),
					coalesce(f.email::text, ''), coalesce(f.name, '')
				FROM archive.mail_message m JOIN archive.connection o ON o.id = m.connection
				LEFT JOIN LATERAL (SELECT k.email, nullif(p.name, '') AS name FROM archive.mail_party p JOIN archive.known_person k ON k.id = p.person
					WHERE p.message = m.id AND p.role = 'from' LIMIT 1) f ON true
				WHERE coalesce(m.received_at, m.sent_at) > now() - make_interval(days => $1) AND (m.subject ~* $2 OR m.text ~* $2)
					AND (o.integration OR o.member = $3 AND NOT $4) AND (m.access IS NULL OR m.access = $5) AND concat_ws(' ', f.email::text, f.name) ~* $6
				ORDER BY coalesce(m.received_at, m.sent_at) DESC LIMIT 2000`,
				max(s.Days, 1), cmp.Or(s.Near, "."), c.Member, c.Integration, c.Access, cmp.Or(s.From, ""))
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var h hit
				if err := rows.Scan(&h.subject, &h.text, &h.id, &h.at, &h.from, &h.name); err != nil {
					return err
				}
				hits = append(hits, h)
			}
			return rows.Err()
		})
		if err != nil {
			return nil, err
		}
		out, seen := []any{}, map[string]bool{}
		for _, h := range hits {
			o := map[string]any{"subject": h.subject, "message": h.id, "from": h.from, "from_name": h.name}
			if h.at != nil {
				o["sent_at"] = h.at.UTC().Format(time.RFC3339)
			}
			if find == nil {
				if !seen[h.id] {
					seen[h.id] = true
					o["id"], o["text"] = h.id, h.text
					out = append(out, o)
				}
				continue
			}
			for _, m := range find.FindAllStringSubmatch(h.subject+"\n"+h.text, -1) {
				n := strings.ToUpper(strings.Join(strings.Fields(m[len(m)-1]), ""))
				if !seen[n] {
					seen[n] = true
					x := maps.Clone(o)
					x["id"] = n
					out = append(out, x)
				}
			}
		}
		return out, nil
	}
}

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

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

type actionError struct {
	msg  string
	kind error
}

var errActionInvalid = errors.New("invalid")

func (e *actionError) Error() string { return e.msg }
func (e *actionError) Unwrap() error { return e.kind }

func (a *Archive) Action(ctx context.Context, name string, input any, pick string, clients map[uuid.UUID]*connect.Client, until time.Time) (any, error) {
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
		lvl, group, err := a.level(ctx)
		if err != nil {
			return nil, err
		}
		if !mayUse(mine[0], lvl, group) {
			return nil, fail(ErrForbidden, "only editors and up use the archive's %s%s", p.Name, map[bool]string{true: ", and of the group it is narrowed to", false: ""}[mine[0].Access != nil])
		}
	}
	switch {
	case len(mine) == 0 && pick != "":
		return nil, fail(ErrNotFound, "you have no %s connection %q", p.Name, pick)
	case len(mine) == 0 && p.Integration:
		return nil, fail(ErrNotFound, "this archive has not connected %s; a manager, engineer or the owner connects it in Archive settings, Integrations", p.Name)
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
	acts, err := a.actionsOf(ctx, p, c)
	if err != nil {
		return nil, actionFailed(fail, err)
	}
	do := acts[act]
	if do == nil {
		return nil, fail(ErrNotFound, "%s has no action %s; it has %s", p.Name, act, strings.Join(actionNames(acts), ", "))
	}
	if do.Writes {
		if lvl, _, err := a.level(ctx); err != nil {
			return nil, err
		} else if lvl == "viewer" {
			return nil, fail(ErrForbidden, "a read-only token cannot change anything in %s; use a full one", p.Name)
		}
	}
	if do.Input != nil {
		if err := actionCheck(name, do.Input, in); err != nil {
			return nil, err
		}
	}
	cl := clients[c.ID]
	if cl == nil {
		if cl, err = a.connClient(ctx, c); err != nil {
			return nil, actionFailed(fail, err)
		}
		clients[c.ID] = cl
	}
	cl.Files = actionFiles{a, ctx, c.ID, until}
	out, err := do.Do(cl, raw)
	if err != nil {
		return nil, actionFailed(fail, err)
	}
	if do.Asks {
		if _, err := a.syncWant(ctx, c.ID); err != nil {
			fmt.Fprintf(os.Stderr, "archive: workspace %s: asking for a sync of %s: %v\n", a.ws.ID, c.ID, err)
		}
	}
	return out, nil
}

func (a *Archive) actionsOf(ctx context.Context, p *connect.Provider, c generated.ArchiveConnection) (map[string]*connect.Action, error) {
	if p.MCP == "" {
		return p.Actions, nil
	}
	tools, err := a.connTools(ctx, c)
	if err != nil {
		return nil, err
	}
	return p.ToolActions(tools), nil
}

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

func actionNames(acts map[string]*connect.Action) []string {
	var out []string
	for n := range acts {
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
		return r.a.Action(ctx, name, input, opts.Connection, r.clients, r.clock.end)
	})
}

func (r *runner) closeClients() {
	for _, c := range r.clients {
		c.Close()
	}
}

func (a *Archive) actionTypes(ctx context.Context) string {
	var b strings.Builder
	var conns []generated.ArchiveConnection
	if ac := ActorFrom(ctx); ac != nil {
		var err error
		if conns, err = a.memberConnections(ctx, ac.member); err != nil {
			fmt.Fprintf(os.Stderr, "archive: workspace %s: the actions of %s: %v\n", a.ws.ID, ac.Email, err)
		}
		lvl, group, _ := a.level(ctx)
		conns = slices.DeleteFunc(conns, func(c generated.ArchiveConnection) bool {
			k := connect.Get(c.Provider)
			return k == nil || k.Integration != c.Integration || c.Integration && !mayUse(c, lvl, group)
		})
	}
	var yours []string
	for _, c := range conns {
		if p, err := a.hub.connector(c.Provider); err == nil {
			yours = append(yours, p.Name+" "+c.Account)
		}
	}
	intro := "Actions on the accounts you connected, by name, for archive.action (see Connected accounts)"
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
		acts, err := a.actionsOf(ctx, p, c)
		if err != nil {
			fmt.Fprintf(os.Stderr, "archive: workspace %s: the tools of %s %s: %v\n", a.ws.ID, p.Name, c.Account, err)
			continue
		}
		done[p.ID] = true
		for _, n := range actionNames(acts) {
			act := acts[n]
			actionEntry(&b, p.ID+"."+n, act.Doc, act.Input, act.Output)
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
