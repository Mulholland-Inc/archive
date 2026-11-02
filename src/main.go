package main

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Mulholland-Inc/archive/generated"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var cliVersion = "0.1.0"

const cliUsage = `archive keeps all your stuff in one place, in Postgres tables you define.

Usage:
  archive [-as email] [-workspace uuid] <command> ...
                                     every command acts as a user (default the first owner) of a
                                     workspace (default the first one)

  archive serve [-addr :8080]        serve the HTTP API (/api), sign-in (/auth) and MCP (/mcp) for the web app
  archive mcp                        serve MCP over stdio
  archive types                      print the tables, their columns and the built-in types
  archive run <file.ts|->            run TypeScript against the archive SDK in one transaction,
                                     as MCP's run tool does, and print {result, logs, error}
  archive run -docs                  print the SDK documentation, with an interface per table

  archive schema show [name]         print the DDL of every table and view, or of one, generated from
                                     the database
  archive schema migrate <file.sql|dir|-> [-confirm hash]
                                     dry-run DDL in one transaction and print what it runs, what it
                                     changes, what data it drops and a confirm; -confirm with that
                                     confirm runs it. Archive adds to each table the id, access,
                                     created_at, updated_at, search and row-level security it leaves
                                     out. A file runs once, by its name; a dir runs its *.sql files
                                     that have not run, in name order
  archive schema revisions           list the migrations, newest first

  archive file add <path>... [-access group]
                                     store files and extract their metadata and text; with -access
                                     only that group and the owners see them
  archive file get <id> [-o path|-]  print the file's metadata, or with -o write its content
  archive file list [-limit n] [-after id]

  archive list <table> [-limit n] [-after id] [-reveal] [column=value ...]
  archive get <table> <id> [-reveal] -reveal shows secret values, to owners
  archive create <table> [json]      json from the argument or stdin; its access column says which
                                     group sees the row, null for the whole workspace
  archive set <table> <id> [json]    set some columns; null clears a column
  archive update <table> <id> [json] replace the whole row
  archive delete <table> <id>
  archive search <query> [-type t] [-from date] [-to date] [-limit n]
                                     find rows and files by their words

  archive user add <email> [-name n] [-role viewer|editor|owner]
                                     add a row to the member table, which invites them: their first
                                     sign-in with a provider under that verified email links them,
                                     and one sign-in reaches all of their workspaces; the first one
                                     is an owner. Change roles and groups with archive set member
  archive user remove <email>        delete their member row, which takes them out of the workspace
                                     with their tokens and role; the last owner stays
  archive token create [-name n] [-expires 720h] [-viewer]
                                     a token acts as you; -viewer makes it read only
  archive token list
  archive token revoke <id>

  archive workspace add <name> <owner email> [-type t] [-icon i] [-color 0-7]
                                     create a workspace, with its own database, for someone else;
                                     -type is personal (the default), family or business, and
                                     -icon and -color are how the web app shows it
  archive workspace list
  archive workspace delete <uuid>    drop a workspace: its database and its roles

Environment:
  DATABASE_URL       Postgres 18, required, as a role with CREATEROLE and CREATEDB, like the one
                     docker compose up -d starts; several instances can share one when they
                     share ARCHIVE_SECRET_KEY and ARCHIVE_BLOB_URL; every other workspace gets its own
                     database on the same server
  ARCHIVE_SECRET_KEY 32-byte key (hex or base64) for secret values and the Postgres role password,
                     the same on every instance; unset uses ./data/secret.key, for one instance
  ARCHIVE_URL        public base URL of the web app, which runs on its own as static files,
                     for passkeys and where sign-in returns (default http://localhost:5173); the API,
                     sign-in, MCP, webhooks, file links and OAuth redirects are on the same URL with
                     app. made api., like https://api.archive.dev for https://app.archive.dev, or on
                     http://localhost:<port> while it is http://localhost, when requests from this
                     machine are signed in as the first owner
  ARCHIVE_BLOB_URL   gs://bucket/prefix for GCS as the service account of the Google Cloud metadata server;
                     unset stores files in ./data/blobs
  ARCHIVE_GOOGLE     https://<client id>:<client secret>@accounts.google.com
  ARCHIVE_MICROSOFT  https://<client id>:<client secret>@login.microsoftonline.com
  ARCHIVE_SLACK      https://<client id>:<client secret>@slack.com
  ARCHIVE_GITHUB     https://<client id>:<client secret>@github.com
                     sign-in providers, with the id and secret URL-escaped; each shows on the sign-in
                     page when it is set, and its redirect URL is
                     <API>/auth/oidc/<google|microsoft|slack|github>/callback. Only invited people sign
                     in, matched on an email the provider says is verified
  ARCHIVE_LIVE_KEY   key the API announces live changes with to the app Worker on the API's /live/announce
                     (apps/web/worker.js), which tells every open web app what moved; unset, serve holds
                     the web app's /live sockets itself and other archive commands on this machine tell
                     it; ARCHIVE_LIVE_URL sends the announces to another address, like wrangler dev's
  ANTHROPIC_API_KEY  key for the agent behind POST /api/chat, which can only call the run tool, in
                     every workspace; ANTHROPIC_BASE_URL points it at another
                     Messages API endpoint

On Cloud Run, the Cloud Tasks queue projects/<project>/locations/<region>/queues/<service> calls
POST <API>/tasks at each time workflows or files are due, so serve can scale to zero, with the OIDC
token of <service>-tasks@<project>.iam.gserviceaccount.com; /tasks refuses any other. Anywhere else,
serve runs that work itself when it is due, and picks up what was due while it was stopped when it starts.
`

var cliErrUsage = errors.New("usage")

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := cliRun(ctx, os.Args[1:], os.Stdin, os.Stdout)
	cancel()
	switch {
	case err == nil, errors.Is(err, flag.ErrHelp):
	case errors.Is(err, cliErrUsage):
		fmt.Fprint(os.Stderr, cliUsage)
		os.Exit(1)
	default:
		cliPrintError(os.Stderr, err)
		os.Exit(1)
	}
}

func cliPrintError(w io.Writer, err error) {
	var ve *ValidationError
	if errors.As(err, &ve) {
		fmt.Fprintf(w, "invalid %s:\n", ve.Type)
		for _, e := range ve.Errors {
			fmt.Fprintf(w, "  %s: %s\n", e.Path, e.Message)
		}
		return
	}
	fmt.Fprintln(w, "archive:", err)
}

func cliRun(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) == 0 {
		return cliErrUsage
	}
	cmd, args := args[0], args[1:]
	switch cmd {
	case "help", "-h", "-help", "--help":
		fmt.Fprint(stdout, cliUsage)
		return nil
	case "version":
		fmt.Fprintln(stdout, cliVersion)
		return nil
	}

	fs := flag.NewFlagSet("archive "+cmd, flag.ContinueOnError)
	fs.Usage = func() { fmt.Fprint(fs.Output(), cliUsage) }
	addr := fs.String("addr", ":8080", "listen address (serve)")
	confirm := fs.String("confirm", "", "run the migration whose dry run printed this confirm (schema migrate)")
	typ := fs.String("type", "", "only this table, or file (search); personal, family or business (workspace add)")
	icon := fs.String("icon", "", "one of the web app's icons, like chest or cottage (workspace add)")
	color := fs.String("color", "", "one of the web app's tints, 0 to 7 (workspace add)")
	limit := fs.Int("limit", 100, "page size (list)")
	after := fs.String("after", "", "list objects after this id (list)")
	as := fs.String("as", "", "act as this user")
	name := fs.String("name", "", "name (user add, token create)")
	role := fs.String("role", "", "viewer, editor or owner (user add)")
	viewer := fs.Bool("viewer", false, "read-only token (token create)")
	access := fs.String("access", "", "group that sees the files (file add)")
	expires := fs.Duration("expires", 0, "token lifetime (token create)")
	reveal := fs.Bool("reveal", false, "show secret values, to owners (list, get)")
	output := fs.String("o", "", "write the file content here, - for stdout (file get)")
	docs := fs.Bool("docs", false, "print the SDK documentation (run)")
	workspace := fs.String("workspace", "", "act in this workspace")
	from := fs.String("from", "", "only items added on or after this date (search)")
	to := fs.String("to", "", "only items added up to this date (search)")
	for (cmd == "-as" || cmd == "-workspace") && len(args) > 1 {
		if cmd == "-as" {
			*as = args[0]
		} else {
			*workspace = args[0]
		}
		cmd, args = args[1], args[2:]
	}
	pos, err := cliParse(fs, args)
	if err != nil {
		return err
	}
	if cmd == "run" && *docs {
		pos = append(pos, "")
	}
	if err := cliCheckCommand(cmd, pos); err != nil {
		return err
	}

	h, err := Open(ctx)
	if err != nil {
		return err
	}
	defer h.Close()
	a, release, err := h.acquire(ctx, *workspace)
	if err != nil {
		return err
	}
	defer release()
	var ac *Actor
	if cmd != "serve" {
		if ac, err = a.CLIActor(ctx, *as); err != nil {
			return err
		}
	}
	if ac == nil && cmd != "serve" && cmd != "types" && cmd != "user" && (cmd != "schema" || pos[0] != "show") {
		return errors.New("no users yet: add the first owner with archive user add <email>")
	}
	ctx = withArchive(WithActor(ctx, ac), a)
	out := func(v any, err error) error {
		if err != nil {
			return err
		}
		return cliJSON(stdout, v)
	}

	switch cmd {
	case "serve":
		return cliServe(ctx, h, a, *addr)
	case "mcp":
		return mcpServer(a).Run(ctx, &mcp.StdioTransport{})
	case "types":
		return cliJSON(stdout, a.Catalog())
	case "run":
		if *docs {
			_, err := io.WriteString(stdout, a.RunDocs(ctx))
			return err
		}
		var code []byte
		if pos[0] == "-" {
			code, err = io.ReadAll(stdin)
		} else {
			code, err = os.ReadFile(pos[0])
		}
		if err != nil {
			return err
		}
		res, err := a.Run(ctx, string(code))
		if err != nil {
			return err
		}
		if err := cliJSON(stdout, res); err != nil {
			return err
		}
		if res.Error != "" {
			return errors.New(res.Error)
		}
		return nil
	case "schema":
		return cliSchema(ctx, a, pos, *confirm, stdin, stdout, out)
	case "file":
		return cliFile(ctx, a, pos, *access, *output, *limit, *after, stdout, out)
	case "user", "token":
		return cliAuth(ctx, a, cmd, pos, *name, *role, *viewer, *expires, out)
	case "workspace":
		var look WorkspaceLook
		if *typ != "" {
			look.Type = typ
		}
		if *icon != "" {
			look.Icon = icon
		}
		if *color != "" {
			c, err := strconv.ParseInt(*color, 10, 16)
			if err != nil {
				return fmt.Errorf("-color %q: want a tint from 0 to 7", *color)
			}
			look.Color = new(int16(c))
		}
		return cliWorkspace(ctx, h, cmd, pos, look, out)
	case "list":
		where := map[string]any{}
		for _, kv := range pos[1:] {
			k, v, ok := strings.Cut(kv, "=")
			if !ok {
				return fmt.Errorf("filter %q: want field=value", kv)
			}
			where[k] = v
		}
		var aid uuid.UUID
		if *after != "" {
			if aid, err = uuid.Parse(*after); err != nil {
				return fmt.Errorf("-after: %w", err)
			}
		}
		rs, err := a.List(ctx, pos[0], where, *limit, aid)
		if err == nil && *reveal {
			err = a.Reveal(ctx, pos[0], rs...)
		}
		return out(rs, err)
	case "search":
		f := SearchFilter{Type: *typ, From: *from, To: *to, Limit: *limit}
		return out(a.Search(ctx, strings.Join(pos, " "), f))
	case "create":
		data, err := cliData(pos[1:], stdin)
		if err != nil {
			return err
		}
		return out(a.Create(ctx, pos[0], data))
	}

	id, err := uuid.Parse(pos[1])
	if err != nil {
		return fmt.Errorf("id %q: %w", pos[1], err)
	}
	switch cmd {
	case "get":
		r, err := a.Get(ctx, pos[0], id)
		if err == nil && *reveal {
			err = a.Reveal(ctx, pos[0], r)
		}
		return out(r, err)
	case "delete":
		return a.Delete(ctx, pos[0], id)
	}
	data, err := cliData(pos[2:], stdin)
	if err != nil {
		return err
	}
	if cmd == "set" {
		return out(a.Patch(ctx, pos[0], id, data))
	}
	return out(a.Replace(ctx, pos[0], id, data))
}

func cliCheckCommand(cmd string, pos []string) error {
	n, ok := map[string]int{
		"serve": 0, "mcp": 0, "types": 0, "run": 1, "schema": 1, "list": 1, "create": 1,
		"get": 2, "delete": 2, "set": 2, "update": 2,
		"user": 2, "token": 1, "file": 1, "search": 1, "workspace": 1,
	}[cmd]
	if !ok {
		return fmt.Errorf("unknown command %q (see archive help)", cmd)
	}
	if len(pos) < n {
		return cliErrUsage
	}
	return nil
}

func cliAuth(ctx context.Context, a *Archive, cmd string, pos []string, name, role string, viewer bool, expires time.Duration, out func(any, error) error) error {
	switch cmd + " " + pos[0] {
	case "user add":
		return out(a.AddUser(ctx, pos[1], name, role))
	case "user remove":
		return a.RemoveUser(ctx, pos[1])
	case "token create":
		return out(a.CreateToken(ctx, name, expires, viewer))
	case "token list":
		return out(a.Tokens(ctx))
	case "token revoke":
		if len(pos) > 1 {
			id, err := uuid.Parse(pos[1])
			if err != nil {
				return err
			}
			return a.RevokeToken(ctx, id)
		}
	}
	return cliErrUsage
}

func cliWorkspace(ctx context.Context, h *Hub, cmd string, pos []string, look WorkspaceLook, out func(any, error) error) error {
	switch cmd + " " + pos[0] {
	case "workspace add":
		if len(pos) > 2 {
			return out(h.AddWorkspace(ctx, pos[1], pos[2], look))
		}
	case "workspace list":
		return out(h.Workspaces(ctx))
	case "workspace delete":
		if len(pos) > 1 {
			return h.DeleteWorkspace(ctx, pos[1])
		}
	}
	return cliErrUsage
}

func cliFile(ctx context.Context, a *Archive, pos []string, access, output string, limit int, after string, stdout io.Writer, out func(any, error) error) error {
	if pos[0] == "list" {
		var aid uuid.UUID
		if after != "" {
			var err error
			if aid, err = uuid.Parse(after); err != nil {
				return fmt.Errorf("-after: %w", err)
			}
		}
		return out(a.Files(ctx, limit, aid))
	}
	if len(pos) < 2 {
		return cliErrUsage
	}
	switch pos[0] {
	case "add":
		var fs []*File
		for _, p := range pos[1:] {
			fh, err := os.Open(p)
			if err != nil {
				return err
			}
			f, err := a.AddFile(ctx, access, filepath.Base(p), fh)
			fh.Close()
			if err != nil {
				return fmt.Errorf("%s: %w", p, err)
			}
			fs = append(fs, f)
		}
		if len(fs) == 1 {
			return out(fs[0], nil)
		}
		return out(fs, nil)
	case "get":
		id, err := uuid.Parse(pos[1])
		if err != nil {
			return fmt.Errorf("id %q: %w", pos[1], err)
		}
		if output == "" {
			return out(a.GetFile(ctx, id))
		}
		_, rc, err := a.OpenFile(ctx, id)
		if err != nil {
			return err
		}
		defer rc.Close()
		w := stdout
		if output != "-" {
			fh, err := os.Create(output)
			if err != nil {
				return err
			}
			defer fh.Close()
			w = fh
		}
		_, err = io.Copy(w, rc)
		return err
	}
	return cliErrUsage
}

func cliParse(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if args = fs.Args(); len(args) == 0 {
			return pos, nil
		}
		pos, args = append(pos, args[0]), args[1:]
	}
}

func cliData(args []string, stdin io.Reader) (map[string]any, error) {
	var r io.Reader = stdin
	if len(args) > 0 && args[0] != "-" {
		r = strings.NewReader(args[0])
	}
	var data map[string]any
	if err := json.NewDecoder(r).Decode(&data); err != nil {
		return nil, fmt.Errorf("reading object JSON: %w", err)
	}
	return data, nil
}

func cliJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func cliSchema(ctx context.Context, a *Archive, pos []string, confirm string, stdin io.Reader, stdout io.Writer, out func(any, error) error) error {
	switch {
	case pos[0] == "show":
		ddl, err := a.st.Schema(ctx)
		if err != nil {
			return err
		}
		names := specKeys(ddl)
		if len(pos) > 1 {
			if ddl[pos[1]] == "" {
				return fmt.Errorf("no table, view, type or function %q; there are %s: %w", pos[1], strings.Join(names, ", "), ErrNotFound)
			}
			names = pos[1:2]
		}
		for _, name := range names {
			fmt.Fprintf(stdout, "-- %s\n%s\n", name, strings.TrimRight(ddl[name], "\n"))
		}
		return nil
	case pos[0] == "revisions":
		return out(a.SchemaRevisions(ctx, 100))
	case pos[0] == "migrate" && len(pos) > 1 && pos[1] == "-":
		b, err := io.ReadAll(stdin)
		if err != nil {
			return err
		}
		ms, err := cliMigrate(ctx, a, [][2]string{{string(b), ""}}, confirm)
		if err != nil {
			return err
		}
		return out(ms[0], nil)
	case pos[0] == "migrate" && len(pos) > 1:
		paths := []string{pos[1]}
		if st, err := os.Stat(pos[1]); err != nil {
			return err
		} else if st.IsDir() {
			if paths, err = filepath.Glob(filepath.Join(pos[1], "*.sql")); err != nil {
				return err
			}
			slices.Sort(paths)
		}
		var srcs [][2]string
		for _, p := range paths {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			srcs = append(srcs, [2]string{string(b), filepath.Base(p)})
		}
		return out(cliMigrate(ctx, a, srcs, confirm))
	}
	return cliErrUsage
}

// cliMigrate runs the migrations, each named or not, one after the other in one transaction, so each sees the
// ones before, and rolls them all back unless confirm is theirs: one migration's own, or a hash of several.
func cliMigrate(ctx context.Context, a *Archive, srcs [][2]string, confirm string) ([]*SchemaMigration, error) {
	ms, sums, sum := []*SchemaMigration{}, []string{}, ""
	errDry := errors.New("dry run")
	err := a.as(ctx, func(ctx context.Context) error {
		for _, src := range srcs {
			m, err := a.Migrate(ctx, src[0], src[1], "")
			if err == nil && !m.Skipped {
				sums = append(sums, m.Confirm)
				m, err = a.Migrate(ctx, src[0], src[1], m.Confirm)
			}
			if err != nil && src[1] != "" {
				return fmt.Errorf("%s: %w", src[1], err)
			} else if err != nil {
				return err
			}
			ms = append(ms, m)
		}
		if sum = strings.Join(sums, ""); len(sums) > 1 {
			h := sha256.Sum256([]byte(strings.Join(sums, "\n")))
			sum = hex.EncodeToString(h[:])
		}
		if sum == "" || sum == confirm {
			return nil
		}
		return errDry
	})
	if !errors.Is(err, errDry) {
		return ms, err
	}
	if confirm != "" {
		return nil, fmt.Errorf("the schema or the migrations changed since their dry run; dry-run them again for a new confirm: %w", ErrConflict)
	}
	for _, m := range ms {
		if !m.Skipped {
			m.Applied, m.Revision, m.Confirm = false, nil, sum
		}
	}
	return ms, nil
}

func cliServe(ctx context.Context, h *Hub, a *Archive, addr string) error {
	_, port, _ := net.SplitHostPort(addr)
	if err := h.urls(cmp.Or(port, "80")); err != nil {
		return err
	}
	si, err := newSignin(h)
	if err != nil {
		return err
	}
	if none, err := generated.New(a.st.sys).NoMembers(ctx); err != nil {
		return err
	} else if none {
		fmt.Fprintln(os.Stderr, "archive: no users yet; add the first owner with archive user add <email>")
	}
	if h.localApp() {
		fmt.Fprintf(os.Stderr, "archive: %s is on this machine, so requests from this machine are signed in as the first owner\n", h.app)
	} else if len(si.configs) == 0 {
		fmt.Fprintln(os.Stderr, "archive: no sign-in provider is set, so nobody can sign in to the web app; set ARCHIVE_GOOGLE, ARCHIVE_MICROSOFT, ARCHIVE_SLACK or ARCHIVE_GITHUB (see archive help)")
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
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("/mcp", h.mcp)
	mux.HandleFunc("POST /tasks", h.tasksHTTP)
	h.live.serving()
	mux.HandleFunc("GET /live", h.liveHTTP)
	mux.HandleFunc("POST /live/announce", h.liveAnnounceHTTP)
	mux.HandleFunc("GET /auth/live", h.liveCheckHTTP)
	hs := &http.Server{Addr: addr, Handler: h.cors(h.live.after(mux)), ReadHeaderTimeout: 10 * time.Second}
	errc := make(chan error, 1)
	go func() { errc <- hs.ListenAndServe() }()
	fmt.Fprintf(os.Stderr, "archive: serving the API on %s/api and MCP on %s/mcp, for the web app on %s\n", h.api, h.api, h.app)
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
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Origin")
		first, _, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
		headers := "Authorization, Content-Type, Archive-Workspace"
		switch origin := r.Header.Get("Origin"); {
		case origin == "":
		case split && origin == app && (first == "api" || first == "auth"):
			w.Header().Set("Access-Control-Allow-Origin", app)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			headers = "Content-Type, Archive-Workspace"
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
		return ok(a.Run(r.Context(), body.Code))
	})
	mux.HandleFunc("POST /api/chat", a.chat)
	handle("GET /api/chats", func(r *http.Request) (int, any, error) {
		return ok(a.Chats(r.Context()))
	})
	handle("GET /api/chats/{id}", func(r *http.Request) (int, any, error) {
		id, err := httpID(r)
		if err != nil {
			return 0, nil, err
		}
		return ok(a.Chat(r.Context(), id))
	})
	handle("PATCH /api/chats/{id}", func(r *http.Request) (int, any, error) {
		id, err := httpID(r)
		if err != nil {
			return 0, nil, err
		}
		var body struct {
			Title string `json:"title"`
		}
		if err := httpDecode(r, &body); err != nil {
			return 0, nil, err
		}
		return ok(a.RenameChat(r.Context(), id, body.Title))
	})
	handle("DELETE /api/chats/{id}", func(r *http.Request) (int, any, error) {
		id, err := httpID(r)
		if err != nil {
			return 0, nil, err
		}
		return http.StatusNoContent, nil, a.DeleteChat(r.Context(), id)
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
	a.workflowRoutes(mux)
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
			if k != "limit" && k != "after" && k != "reveal" {
				where[k] = v[0]
			}
		}
		rs, err := a.List(r.Context(), r.PathValue("type"), where, limit, after)
		if err == nil && q.Get("reveal") == "true" {
			err = a.Reveal(r.Context(), r.PathValue("type"), rs...)
		}
		return ok(rs, err)
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
		rec, err := a.Get(ctx, typ, id)
		if err == nil && r.URL.Query().Get("reveal") == "true" {
			err = a.Reveal(ctx, typ, rec)
		}
		return rec, err
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

func mcpServer(a *Archive) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "archive", Title: "Archive", Version: cliVersion}, nil)
	var mu sync.Mutex
	var seen *Catalog
	tool := func(ctx context.Context) {
		mu.Lock()
		defer mu.Unlock()
		docs := a.RunDocs(ctx)
		if a.Catalog() == seen {
			return
		}
		seen = a.Catalog()
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
