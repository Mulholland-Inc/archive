package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Mulholland-Inc/archive/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := start(ctx, os.Args[1:])
	cancel()
	if err != nil && !errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(os.Stderr, "archive:", err)
		os.Exit(1)
	}
}

func start(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("want serve, migrate, secrets or places")
	}
	switch args[0] {
	case "serve":
		fs := flag.NewFlagSet("archive serve", flag.ContinueOnError)
		addr := fs.String("addr", ":8080", "listen address")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		h, err := Open(ctx)
		if err != nil {
			return err
		}
		defer h.Close()
		return serve(ctx, h, *addr)
	case "migrate":
		h, err := Open(ctx)
		if err != nil {
			return err
		}
		defer h.Close()
		return h.migrateAll(ctx)
	case "secrets":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		return enc.Encode(connect.Secrets())
	case "places":
		return connect.WritePlaces(ctx, "connect/places.json.gz")
	}
	return fmt.Errorf("unknown command %q: want serve, migrate, secrets or places", args[0])
}

func serve(ctx context.Context, h *Hub, addr string) error {
	_, port, _ := net.SplitHostPort(addr)
	if err := h.urls(cmp.Or(port, "80")); err != nil {
		return err
	}
	si, err := newSignin(h)
	if err != nil {
		return err
	}
	if _, err := h.q().FirstAdmin(ctx); errors.Is(err, pgx.ErrNoRows) {
		fmt.Fprintln(os.Stderr, "archive: no users yet, so nobody can sign in")
	} else if err != nil {
		return err
	}
	if h.localApp() {
		fmt.Fprintf(os.Stderr, "archive: %s is on this machine, so requests from this machine are signed in as the first admin\n", h.app)
	} else if len(si.configs) == 0 {
		fmt.Fprintln(os.Stderr, "archive: no sign-in provider is set, so nobody can sign in to the web app; set ARCHIVE_GOOGLE, ARCHIVE_MICROSOFT, ARCHIVE_SLACK or ARCHIVE_GITHUB")
	}
	if err := h.wakeAll(ctx); err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.Handle("/api/", h.httpAuth((*Archive).handler))
	tus := h.httpAuth(func(a *Archive) http.Handler { return http.HandlerFunc(a.tus) })
	mux.Handle("/api/files/tus", tus)
	mux.Handle("/api/files/tus/", tus)
	mux.HandleFunc("GET /files/{workspace}/{id}", h.inWorkspace((*Archive).fileSigned))
	mux.HandleFunc("POST /hooks/run/{workspace}/{token}", h.inWorkspace((*Archive).workflowHook))
	mux.Handle("/auth/", si.handler())
	h.connectRoutes(mux)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	h.mcpRoutes(mux, false)
	mux.HandleFunc("POST /tasks", h.tasksHTTP)
	mux.HandleFunc("POST /push/{id}", h.pushHTTP)
	mux.HandleFunc("GET /push/{id}", h.pushHTTP)
	mux.HandleFunc("GET /device/attest", h.device(false, h.deviceChallenge))
	mux.HandleFunc("POST /device/attest", h.device(false, h.deviceAttest))
	mux.HandleFunc("POST /device/files", h.device(false, h.deviceSave))
	mux.HandleFunc("POST /device/ask", h.device(true, h.deviceAsk))
	mux.HandleFunc("PUT /device/originals", h.device(true, h.deviceOriginal))
	mux.HandleFunc("POST /device/{stream}", h.device(true, h.deviceRecords))
	mux.HandleFunc("POST /email", h.emailHTTP)
	mux.HandleFunc("GET /live", h.liveHTTP)
	mux.HandleFunc("GET /auth/live", h.liveCheckHTTP)
	own := http.NewServeMux()
	h.mcpRoutes(own, true)
	api, mcpOwn := h.cors(mux), mcpCORS(own)
	hs := &http.Server{Addr: addr, ReadHeaderTimeout: 10 * time.Second, Handler: h.live.after(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'")
		if h.mcpHost(r) {
			mcpOwn.ServeHTTP(w, r)
		} else {
			api.ServeHTTP(w, r)
		}
	}))}
	errc := make(chan error, 1)
	go func() { errc <- hs.ListenAndServe() }()
	fmt.Fprintf(os.Stderr, "archive: serving the API on %s/api and MCP on %s, for the web app on %s\n", h.api, h.mcpAt(), h.app)
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return hs.Shutdown(sctx)
	}
}

func httpOrigin(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

func (h *Hub) cors(next http.Handler) http.Handler {
	app := httpOrigin(h.app)
	split := app != httpOrigin(h.api)
	if h.localApp() && !split {
		return next
	}
	mcp := mcpCORS(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Origin")
		first, _, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
		if first == "mcp" || first == "oauth" || first == ".well-known" {
			mcp.ServeHTTP(w, r)
			return
		}
		headers := "Authorization, Content-Type, Archive-Workspace"
		switch origin := r.Header.Get("Origin"); {
		case origin == "":
		case split && origin == app && (first == "api" || first == "auth" || first == "connect"):
			w.Header().Set("Access-Control-Allow-Origin", app)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			headers = "Content-Type, Archive-Workspace, Tus-Resumable, Upload-Length, Upload-Offset, Upload-Metadata"
			w.Header().Set("Access-Control-Expose-Headers", "Location, Upload-Offset, Archive-File-Id")
		case !h.localApp():
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		if w.Header().Get("Access-Control-Allow-Origin") != "" && r.Method == "OPTIONS" && r.Header.Get("Access-Control-Request-Method") != "" {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE")
			w.Header().Set("Access-Control-Allow-Headers", headers)
			w.Header().Set("Access-Control-Max-Age", "7200")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

var httpErrBadRequest = errors.New("bad request")

func httpWrite(w http.ResponseWriter, status int, v any, err error) {
	if err != nil {
		status, v = httpError(err)
	}
	if status == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", "Bearer")
	}
	if status == http.StatusNoContent {
		w.WriteHeader(status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func httpHandle(mux *http.ServeMux, pattern string, f func(r *http.Request) (int, any, error)) {
	mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		status, v, err := f(r)
		httpWrite(w, status, v, err)
	})
}

func httpHandler(a *Archive) http.Handler {
	mux := http.NewServeMux()
	handle := func(pattern string, f func(r *http.Request) (int, any, error)) { httpHandle(mux, pattern, f) }
	ok := func(v any, err error) (int, any, error) { return http.StatusOK, v, err }

	handle("POST /api/run", func(r *http.Request) (int, any, error) {
		var body struct {
			Code string `json:"code"`
		}
		if err := httpDecode(r, &body); err != nil {
			return 0, nil, err
		}
		if body.Code == "" {
			return 0, nil, fmt.Errorf("code: want the TypeScript to run: %w", httpErrBadRequest)
		}
		return ok(a.Run(context.WithValue(r.Context(), runFreshKey{}, true), body.Code))
	})
	handle("GET /api/boot", func(r *http.Request) (int, any, error) {
		return ok(a.boot(r.Context()))
	})
	handle("GET /api/agent/list", func(r *http.Request) (int, any, error) {
		return ok(a.agentList(r.Context()))
	})
	handle("GET /api/workflow/list", func(r *http.Request) (int, any, error) {
		return ok(a.workflowList(r.Context()))
	})
	mux.HandleFunc("POST /api/turn", a.turn)
	handle("GET /api/agent/{id}/turns", func(r *http.Request) (int, any, error) {
		id, err := httpID(r)
		if err != nil {
			return 0, nil, err
		}
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		return ok(a.Turns(r.Context(), id, limit))
	})
	handle("DELETE /api/agent/{id}/turns", func(r *http.Request) (int, any, error) {
		id, err := httpID(r)
		if err != nil {
			return 0, nil, err
		}
		return http.StatusNoContent, nil, a.ClearTurns(r.Context(), id)
	})
	mux.HandleFunc("GET /api/run/docs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		io.WriteString(w, a.RunDocs(r.Context()))
	})
	handle("GET /api/types", func(r *http.Request) (int, any, error) {
		return ok(a.Catalog(), nil)
	})
	handle("GET /api/schema", func(r *http.Request) (int, any, error) {
		return ok(a.Schema(r.Context(), r.URL.Query().Get("name")))
	})
	handle("POST /api/schema/migrate", func(r *http.Request) (int, any, error) {
		var body struct {
			SQL     string `json:"sql"`
			Name    string `json:"name"`
			Confirm string `json:"confirm"`
		}
		if err := httpDecode(r, &body); err != nil {
			return 0, nil, err
		}
		return ok(a.Migrate(r.Context(), body.SQL, body.Name, body.Confirm))
	})
	handle("GET /api/schema/revisions", func(r *http.Request) (int, any, error) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		revs, err := a.SchemaRevisions(r.Context(), limit)
		return ok(map[string]any{"revisions": revs}, err)
	})

	a.fileRoutes(mux)
	handle("GET /api/search", func(r *http.Request) (int, any, error) {
		q := r.URL.Query()
		limit, _ := strconv.Atoi(q.Get("limit"))
		f := SearchFilter{Type: q.Get("type"), From: q.Get("from"), To: q.Get("to"), Limit: limit}
		return ok(a.Search(r.Context(), q.Get("q"), f))
	})

	handle("GET /api/{type}", func(r *http.Request) (int, any, error) {
		q := r.URL.Query()
		limit, _ := strconv.Atoi(q.Get("limit"))
		var after uuid.UUID
		if s := q.Get("after"); s != "" {
			var err error
			if after, err = uuid.Parse(s); err != nil {
				return 0, nil, fmt.Errorf("after: %w", httpErrBadRequest)
			}
		}
		where := map[string]any{}
		for k, v := range q {
			if k != "limit" && k != "after" {
				where[k] = v[0]
			}
		}
		return ok(a.List(r.Context(), r.PathValue("type"), where, limit, after))
	})
	handle("POST /api/{type}", func(r *http.Request) (int, any, error) {
		var data map[string]any
		if err := httpDecode(r, &data); err != nil {
			return 0, nil, err
		}
		rec, err := a.Create(r.Context(), r.PathValue("type"), data)
		return http.StatusCreated, rec, err
	})
	object := func(f func(ctx context.Context, typ string, id uuid.UUID, r *http.Request) (any, error)) func(r *http.Request) (int, any, error) {
		return func(r *http.Request) (int, any, error) {
			id, err := httpID(r)
			if err != nil {
				return 0, nil, err
			}
			return ok(f(r.Context(), r.PathValue("type"), id, r))
		}
	}
	withData := func(f func(ctx context.Context, typ string, id uuid.UUID, data map[string]any) (map[string]any, error)) func(r *http.Request) (int, any, error) {
		return object(func(ctx context.Context, typ string, id uuid.UUID, r *http.Request) (any, error) {
			var data map[string]any
			if err := httpDecode(r, &data); err != nil {
				return nil, err
			}
			return f(ctx, typ, id, data)
		})
	}
	handle("GET /api/{type}/{id}", object(func(ctx context.Context, typ string, id uuid.UUID, r *http.Request) (any, error) {
		return a.Get(ctx, typ, id)
	}))
	handle("PUT /api/{type}/{id}", withData(a.Replace))
	handle("PATCH /api/{type}/{id}", withData(a.Patch))
	handle("DELETE /api/{type}/{id}", func(r *http.Request) (int, any, error) {
		id, err := httpID(r)
		if err != nil {
			return 0, nil, err
		}
		return http.StatusNoContent, nil, a.Delete(r.Context(), r.PathValue("type"), id)
	})
	return mux
}

func httpError(err error) (int, any) {
	var ve *ValidationError
	status := http.StatusInternalServerError
	switch {
	case errors.As(err, &ve):
		return http.StatusUnprocessableEntity, ve
	case errors.Is(err, ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, ErrConflict):
		status = http.StatusConflict
	case errors.Is(err, httpErrBadRequest):
		status = http.StatusBadRequest
	case errors.Is(err, ErrUnauthorized):
		status = http.StatusUnauthorized
	case errors.Is(err, ErrForbidden):
		status = http.StatusForbidden
	case errors.Is(err, ErrReadOnly):
		status = http.StatusMethodNotAllowed
	case errors.Is(err, ErrBusy):
		status = http.StatusTooManyRequests
	}
	return status, map[string]string{"error": err.Error()}
}

func httpDecode(r *http.Request, v any) error {
	if err := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 10<<20)).Decode(v); err != nil {
		return fmt.Errorf("body: %v: %w", err, httpErrBadRequest)
	}
	return nil
}

func httpID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return uuid.Nil, fmt.Errorf("id %q: %w", r.PathValue("id"), httpErrBadRequest)
	}
	return id, nil
}

type mcpRunInput struct {
	Code string `json:"code" jsonschema:"TypeScript to run inside Archive against the archive SDK in the tool description"`
}

func mcpServer(a *Archive, ac *Actor) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "archive", Title: "Archive", Version: "0.1.0"}, nil)
	var mu sync.Mutex
	var seen string
	tool := func(ctx context.Context) {
		mu.Lock()
		defer mu.Unlock()
		if ActorFrom(ctx) == nil && ac != nil {
			ctx = WithActor(ctx, ac)
		}
		docs := a.RunDocs(ctx)
		if docs == seen {
			return
		}
		seen = docs
		mcp.AddTool(s, &mcp.Tool{Name: "run", Title: "Run TypeScript in Archive", Description: docs}, func(ctx context.Context, req *mcp.CallToolRequest, in mcpRunInput) (*mcp.CallToolResult, any, error) {
			if req.Extra != nil && req.Extra.TokenInfo != nil {
				if ac, ok := req.Extra.TokenInfo.Extra["actor"].(*Actor); ok {
					ctx = WithActor(ctx, ac)
				}
			}
			out, err := a.Run(ctx, in.Code)
			if err != nil {
				return nil, nil, err
			}
			b, err := json.Marshal(out)
			if err != nil {
				return nil, nil, err
			}
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}, IsError: out.Error != ""}, nil, nil
		})
	}
	tool(context.Background())
	s.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if method == "tools/list" {
				tool(ctx)
			}
			return next(ctx, method, req)
		}
	})
	return s
}

func (a *Archive) mcpVerify(_ context.Context, _ string, r *http.Request) (*auth.TokenInfo, error) {
	ac := ActorFrom(r.Context())
	if ac == nil {
		return nil, auth.ErrInvalidToken
	}
	return &auth.TokenInfo{UserID: ac.UserID.String(), Extra: map[string]any{"actor": ac}}, nil
}
