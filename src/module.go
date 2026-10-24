package archive

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"net/http"
	"path"
	"slices"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// A Module is what a product built on the engine adds to it, such as a family archive's types,
// GEDCOM import, tools and pages. The product's own main registers it and then calls Main, so the
// engine stays generic and the product still ships as one binary in one container.
type Module struct {
	Name string
	// Pack is a spec of the module's own types. It is applied on every start, adding and changing
	// only its types, by the instance admin, so it needs no one to push it.
	Pack []byte
	// Types are semantic types by name, for the pack's types to parse, check and format with.
	Types map[string]SemanticType
	// Connectors are source kinds by name, added like gmail or imap.
	Connectors map[string]Connector
	// Tools adds MCP tools; Tool registers one that runs as the caller.
	Tools func(s *mcp.Server, a *Archive)
	// Routes adds HTTP routes; Authed wraps one that needs a signed-in caller.
	Routes func(mux *http.ServeMux, a *Archive)
	// App is a set of web pages the dashboard links to from its sidebar.
	App *App
	// Brand dresses the dashboard as the product: its name, logo, colours and type. One module
	// at most brings one; without it the dashboard looks as the engine's own.
	Brand *Brand
	// Commands adds commands to the binary, by name: each runs as the first owner on the opened
	// archive, with every module's types applied, and reads its own arguments.
	Commands map[string]func(ctx context.Context, a *Archive, args []string) error
}

// moduleCommand is a module's command of this name, or nil.
func moduleCommand(name string) func(context.Context, *Archive, []string) error {
	for _, m := range registered() {
		if run := m.Commands[name]; run != nil {
			return run
		}
	}
	return nil
}

// Brand is how a product built on the engine looks. CSS is loaded after the dashboard's own
// stylesheet, so it changes the dashboard's tokens (app.css: the palette, the roles such as
// --brand and --accent, --font-sans and --font-display) for day and for night ([data-theme]);
// Files holds what it refers to, such as fonts, served under /brand/. Logo is an SVG drawn where
// the dashboard draws its mark, in the colour around it; Icon is the browser tab's SVG.
type Brand struct {
	Name string
	// About says in a sentence what the product is, for an agent reading /llms.txt.
	About string
	Logo  string
	Icon  []byte
	CSS   []byte
	Files fs.FS
}

// SemanticType reads a value as people write it into its stored form, checks a stored value and
// shows one, as the built-in types such as date and money do.
type SemanticType struct {
	Parse  func(in any, h Hints) ([]Candidate, error)
	Check  func(v any) error
	Format func(v any, locale string) string
}

// Connector is a source kind. Check runs when a source is added; Sync fetches what is new and
// emits each item, which lands in raw and, when the source has a type, goes through intake.
type Connector struct {
	Check func(ctx context.Context, a *Archive, src *Source, creds map[string]any) error
	Sync  func(ctx context.Context, s *Sync) error
}

// Sync is one run of a source, given to a module's connector.
type Sync struct{ r *syncRun }

func (s *Sync) Source() *Source             { return s.r.src }
func (s *Sync) Credentials() map[string]any { return s.r.creds }
func (s *Sync) HTTP() *http.Client          { return s.r.http }
func (s *Sync) Archive() *Archive           { return s.r.a }
func (s *Sync) Emit(ctx context.Context, externalID string, payload map[string]any) error {
	return s.r.emit(ctx, externalID, payload)
}

// App is static web pages served under Path, with Title and Icon for the dashboard's sidebar.
type App struct {
	Path  string
	Title string
	Icon  string
	Files fs.FS
	// Front makes the app's own page the front door: a visitor who is not signed in sees it at /,
	// and someone signed in is sent into the app. One module at most sets it.
	Front bool
}

var (
	modulesMu sync.Mutex
	modules   []Module
)

// Register adds a module. Call it before Main, from the product's main.
func Register(m Module) {
	modulesMu.Lock()
	defer modulesMu.Unlock()
	if m.Name == "" {
		panic("archive: a module needs a name")
	}
	for name, t := range m.Types {
		if _, ok := semParsers[name]; ok {
			panic(fmt.Sprintf("archive: module %s: semantic type %s is already defined", m.Name, name))
		}
		if t.Parse != nil {
			semParsers[name] = t.Parse
		}
		if t.Check != nil {
			semChecks[name] = t.Check
		}
		if t.Format != nil {
			semFormats[name] = t.Format
		}
	}
	for kind, c := range m.Connectors {
		if _, ok := connectors[kind]; ok {
			panic(fmt.Sprintf("archive: module %s: connector %s is already defined", m.Name, kind))
		}
		connectors[kind] = connector{check: c.Check, sync: func(ctx context.Context, r *syncRun) error { return c.Sync(ctx, &Sync{r}) }}
	}
	if m.App != nil && (!strings.HasPrefix(m.App.Path, "/") || strings.HasPrefix(m.App.Path, "/api") || strings.HasPrefix(m.App.Path, "/auth")) {
		panic(fmt.Sprintf("archive: module %s: app path %q must start with / and not be /api or /auth", m.Name, m.App.Path))
	}
	if m.Brand != nil && slices.ContainsFunc(modules, func(o Module) bool { return o.Brand != nil }) {
		panic(fmt.Sprintf("archive: module %s: another module already brings the brand", m.Name))
	}
	modules = append(modules, m)
}

func brandOf() *Brand {
	for _, m := range registered() {
		if m.Brand != nil {
			return m.Brand
		}
	}
	return nil
}

func registered() []Module {
	modulesMu.Lock()
	defer modulesMu.Unlock()
	return slices.Clone(modules)
}

// Tool adds an MCP tool that runs as the person or token calling it, with the same error handling
// as the engine's own tools.
func Tool[In any](s *mcp.Server, name, desc string, f func(context.Context, In) (any, error)) {
	mcpTool(s, name, desc, f)
}

// Authed wraps an HTTP handler so it runs as the signed-in caller (ActorFrom gives them), and
// answers 401 to anyone else.
func (a *Archive) Authed(h http.Handler) http.Handler { return a.httpAuth(h) }

// WriteJSON answers with v as JSON, or with err as the engine's error JSON and status.
func WriteJSON(w http.ResponseWriter, status int, v any, err error) { httpWrite(w, status, v, err) }

// applyPacks brings every module's types into the ontology, as the first instance admin, changing
// only the pack's own types. With no admin yet it waits for the next start.
func (a *Archive) applyPacks(ctx context.Context) error {
	ms := registered()
	if !slices.ContainsFunc(ms, func(m Module) bool { return len(m.Pack) > 0 }) {
		return nil
	}
	u, err := a.userBy(ctx, `pg_has_role(role::name, $1::name, 'USAGE') ORDER BY created_at, id LIMIT 1`, a.prefix+"_admin")
	if err != nil {
		return nil
	}
	ctx = WithActor(ctx, a.userActor(u))
	for _, m := range ms {
		if len(m.Pack) == 0 {
			continue
		}
		pack, err := ParseSpec(m.Pack)
		if err != nil {
			return fmt.Errorf("module %s: pack: %w", m.Name, err)
		}
		cur := a.Spec()
		want := &Spec{Version: cur.Version, Types: maps.Clone(cur.Types)}
		maps.Copy(want.Types, pack.Types)
		b, err := json.Marshal(want)
		if err != nil {
			return err
		}
		if _, err := a.ApplySpec(ctx, b, false); err != nil {
			return fmt.Errorf("module %s: pack: %w", m.Name, err)
		}
	}
	return nil
}

// moduleRoutes mounts every module's routes and apps, and lists the apps for the sidebar.
func (a *Archive) moduleRoutes(mux *http.ServeMux) {
	var apps []map[string]string
	for _, m := range registered() {
		if m.Routes != nil {
			m.Routes(mux, a)
		}
		if m.App != nil {
			p := strings.TrimRight(m.App.Path, "/")
			files := appFiles(m.App.Files)
			mux.Handle(p+"/", http.StripPrefix(p, files))
			if m.App.Front {
				mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
					if _, err := a.httpActor(r); err == nil {
						http.Redirect(w, r, p+"/", http.StatusSeeOther)
						return
					}
					w.Header().Set("Cache-Control", "no-store")
					files.ServeHTTP(w, r)
				})
			}
			apps = append(apps, map[string]string{"path": p + "/", "title": m.App.Title, "icon": m.App.Icon})
		}
	}
	// the brand, public, since the sign-in page wears it too; empty when the engine is alone
	b := brandOf()
	mux.HandleFunc("GET /brand.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		if b != nil {
			w.Write(b.CSS)
		}
	})
	mux.HandleFunc("GET /api/brand", func(w http.ResponseWriter, r *http.Request) {
		v := map[string]string{"name": "Archive"}
		if b != nil {
			v["name"], v["logo"] = cmp.Or(b.Name, "Archive"), b.Logo
		}
		httpWrite(w, http.StatusOK, v, nil)
	})
	if b != nil && b.Files != nil {
		mux.Handle("GET /brand/", http.StripPrefix("/brand", http.FileServerFS(b.Files)))
	}
	if b != nil && len(b.Icon) > 0 {
		mux.HandleFunc("GET /favicon.svg", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/svg+xml")
			w.Write(b.Icon)
		})
	}
	mux.Handle("GET /api/apps", a.httpAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpWrite(w, http.StatusOK, append([]map[string]string{}, apps...), nil)
	})))
}

// appFiles serves a module's pages, and its index.html for an address that names no file, so a
// single-page app's own routes (/family/tree) load the app and it draws them.
func appFiles(files fs.FS) http.Handler {
	fsrv := http.FileServerFS(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name != "" && !strings.HasSuffix(name, "/") {
			if _, err := fs.Stat(files, name); err != nil && !strings.Contains(path.Base(name), ".") {
				r2 := r.Clone(r.Context())
				r2.URL.Path = "/"
				fsrv.ServeHTTP(w, r2)
				return
			}
		}
		fsrv.ServeHTTP(w, r)
	})
}

func moduleTools(s *mcp.Server, a *Archive) {
	for _, m := range registered() {
		if m.Tools != nil {
			m.Tools(s, a)
		}
	}
}
