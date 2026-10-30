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
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var cliVersion = "0.1.0"

const cliUsage = `archive keeps all your stuff in one place, in Postgres tables you define.

Usage:
  archive [-as email] [-workspace id] <command> ...
                                     every command acts as a user (default the first owner) of a
                                     workspace (default the first one)

  archive serve [-addr :8080]        serve the HTTP API (/api), sign-in (/auth) and MCP (/mcp)
  archive mcp                        serve MCP over stdio
  archive types                      print the tables, their columns and the built-in types
  archive run <file.ts|->            run TypeScript against the archive SDK in one transaction,
                                     as MCP's run tool does, and print {result, logs, error}
  archive run -docs                  print the SDK documentation, with an interface per table

  archive schema pull [dir]          print DDL generated from the database, one <name>.sql file
                                     per table or view, or write those files into dir; layout and
                                     comments outside DDL are not kept, COMMENT ON is
  archive schema plan <dir> [-allow-drop]
                                     show the statements that bring the database to the *.sql
                                     files in dir, the whole schema: a table in no file is dropped;
                                     statements that delete data are blocked unless -allow-drop
  archive schema apply <dir> [-allow-drop]
                                     run that plan; renames are not detected, so rename a table
                                     or column with ALTER ... RENAME in psql first

  archive file add <path>... [-space s]
                                     store files and extract their metadata and text
  archive file get <id> [-o path|-]  print the file's metadata, or with -o write its content
  archive file list [-limit n] [-after id]

  archive list <table> [-limit n] [-after id] [-reveal] [column=value ...]
  archive get <table> <id> [-reveal] -reveal shows secret values you are granted
  archive create <table> [-space s] [json]
                                     json from the argument or stdin; default space is your own
  archive set <table> <id> [json]    set some columns; null clears a column
  archive update <table> <id> [json] replace the whole row
  archive delete <table> <id>
  archive search <query> [-type t] [-space s] [-from date] [-to date] [-limit n]
                                     find rows and files by their words

  archive user add <email> [-name n] [-admin]
  archive space add <name>
  archive space list
  archive space remove <space>       delete the space and everything in it; needs owner
  archive space members <space>
  archive space grant <space> <email> <owner|admin|editor|viewer|none>
  archive token create [-name n] [-space s:role,...] [-expires 720h] [-full]
                                     -full makes a token with all your rights, which can also
                                     manage spaces, members and tokens, as Terraform needs
  archive token list
  archive token revoke <id>
  archive secret grant|revoke <email> <type>

  archive workspace add <name> <owner email>
                                     create a workspace, with its own database, for someone else
  archive workspace list
  archive signup on|off              let anyone create a workspace from the sign-in screen
  archive plan list
  archive plan set <id> [json]       a plan and its limits per month: {"name", "price" (a Stripe
                                     price id), "runs", "workflow_runs", "ai_tokens",
                                     "stored_bytes", "concurrent"}; null is no limit
  archive plan default <id|none>     the plan new workspaces start on

Environment:
  DATABASE_URL       external Postgres, as a superuser; unset starts the bundled one; several
                     instances can share one when they share ARCHIVE_SECRET_KEY and ARCHIVE_BLOB_URL;
                     every other workspace gets its own database on the same server
  DATABASE_DIRECT_URL
                     the same Postgres without a pooler in between, for schema changes, which
                     hold a session lock (default DATABASE_URL)
  ARCHIVE_DATA       data directory: the bundled Postgres, its secret.key and files (default ./data)
  ARCHIVE_PG_BIN     directory holding the Postgres binaries; unset uses those on PATH, or
                     downloads Postgres into $ARCHIVE_DATA/pgbin on first run
  ARCHIVE_LOCALE     locale that messy input is read in, like nl-NL
  ANTHROPIC_API_KEY  key for the agent behind POST /api/chat, which can only call the run tool;
                     ANTHROPIC_BASE_URL points it at another Messages API endpoint
  ARCHIVE_CHAT_MODEL model of that agent (default claude-haiku-5-5)
  ARCHIVE_RUN_TIMEOUT
                     time limit of one archive run, POST /api/run or MCP run (default 30s)
  ARCHIVE_BLOB_URL   s3://bucket/prefix?endpoint=https://host&region=r for S3, R2, MinIO or GCS;
                     unset stores files in $ARCHIVE_DATA/blobs
  ARCHIVE_BLOB_KEY, ARCHIVE_BLOB_SECRET
                     access key for ARCHIVE_BLOB_URL (default AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
  ARCHIVE_INSTANCE   prefix of every Postgres role of this instance (default a)
  ARCHIVE_SECRET_KEY 32-byte key (hex or base64) for secret values and the Postgres role password;
                     required with DATABASE_URL, the same on every instance; unset with the
                     bundled Postgres uses $ARCHIVE_DATA/secret.key
  ARCHIVE_URL        public base URL of the web app and sign-in, for sign-in links, cookies and
                     passkeys (default http://localhost:<port>)
  ARCHIVE_API_URL    public base URL of the API, MCP, webhooks, file links, Stripe and every
                     OAuth redirect URL (default ARCHIVE_URL)
  ARCHIVE_ADMIN_EMAIL
                     first owner that serve creates when there are no users yet
  ARCHIVE_SMTP_URL   smtp://user:pass@host:587 for sign-in mail; unset prints links to the log
  ARCHIVE_SMTP_FROM  sender address for sign-in mail
  ARCHIVE_OIDC       JSON list of OIDC providers: [{"name","issuer","client_id","client_secret"}]
  ARCHIVE_STRIPE_KEY Stripe secret key; set, plans have limits and Stripe Checkout sells them
  ARCHIVE_STRIPE_WEBHOOK_SECRET
                     signing secret of the Stripe webhook at ARCHIVE_API_URL/stripe
  ARCHIVE_STRIPE_URL Stripe API base URL (default https://api.stripe.com)
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
	allowDrop := fs.Bool("allow-drop", false, "apply statements that delete data (schema)")
	typ := fs.String("type", "", "only this table, or file (search)")
	limit := fs.Int("limit", 100, "page size (list)")
	after := fs.String("after", "", "list objects after this id (list)")
	as := fs.String("as", "", "act as this user")
	name := fs.String("name", "", "name (user add, token create)")
	admin := fs.Bool("admin", false, "instance admin (user add)")
	full := fs.Bool("full", false, "token with all your rights (token create)")
	space := fs.String("space", "", "space (create, file add); space:role,... (token create)")
	expires := fs.Duration("expires", 0, "token lifetime (token create)")
	reveal := fs.Bool("reveal", false, "show granted secret values (list, get)")
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
	ac, err := a.CLIActor(ctx, *as)
	if err != nil {
		return err
	}
	if ac == nil && cmd != "serve" && cmd != "types" && cmd != "user" && (cmd != "schema" || pos[0] != "pull") {
		return errors.New("no users yet: create the first owner with archive user add <email>, or run archive serve with ARCHIVE_ADMIN_EMAIL")
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
		return cliSchema(ctx, a, pos, *allowDrop, stdout, out)
	case "file":
		return cliFile(ctx, a, pos, *space, *output, *limit, *after, stdout, out)
	case "user", "space", "token", "secret":
		return cliAuth(ctx, a, cmd, pos, *name, *admin, *full, *space, *expires, out)
	case "workspace", "signup", "plan":
		return cliWorkspace(ctx, h, cmd, pos, stdin, out)
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
		rs, err := a.List(ctx, pos[0], "", where, *limit, aid)
		if err == nil && *reveal {
			err = a.Reveal(ctx, pos[0], rs...)
		}
		return out(rs, err)
	case "search":
		f := SearchFilter{Type: *typ, Space: *space, From: *from, To: *to, Limit: *limit}
		return out(a.Search(ctx, strings.Join(pos, " "), f))
	case "create":
		data, err := cliData(pos[1:], stdin)
		if err != nil {
			return err
		}
		return out(a.Create(ctx, *space, pos[0], data))
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
		"user": 2, "space": 1, "token": 1, "secret": 3, "file": 1, "search": 1, "workspace": 1, "signup": 1, "plan": 1,
	}[cmd]
	if !ok {
		return fmt.Errorf("unknown command %q (see archive help)", cmd)
	}
	if len(pos) < n {
		return cliErrUsage
	}
	return nil
}

func cliAuth(ctx context.Context, a *Archive, cmd string, pos []string, name string, admin, full bool, space string, expires time.Duration, out func(any, error) error) error {
	switch cmd + " " + pos[0] {
	case "user add":
		return out(a.AddUser(ctx, pos[1], name, admin))
	case "space add":
		if len(pos) > 1 {
			return out(a.AddSpace(ctx, pos[1]))
		}
	case "space list":
		return out(a.Spaces(ctx))
	case "space remove":
		if len(pos) > 1 {
			if err := a.DeleteSpace(ctx, pos[1]); err != nil {
				return err
			}
			return out(a.Spaces(ctx))
		}
	case "space members":
		if len(pos) > 1 {
			return out(a.Members(ctx, pos[1]))
		}
	case "space grant":
		if len(pos) > 3 {
			if err := a.SetMember(ctx, pos[1], pos[2], pos[3]); err != nil {
				return err
			}
			return out(a.Members(ctx, pos[1]))
		}
	case "token create":
		spaces := map[string]string{}
		for _, sr := range strings.Split(space, ",") {
			if sr == "" {
				continue
			}
			n, r, ok := strings.Cut(sr, ":")
			if !ok {
				return fmt.Errorf("-space %q: want space:role", sr)
			}
			spaces[n] = r
		}
		return out(a.CreateToken(ctx, name, spaces, expires, full))
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
	case "secret grant", "secret revoke":
		return a.GrantSecret(ctx, pos[1], pos[2], pos[0] == "grant")
	}
	return cliErrUsage
}

func cliWorkspace(ctx context.Context, h *Hub, cmd string, pos []string, stdin io.Reader, out func(any, error) error) error {
	switch cmd + " " + pos[0] {
	case "workspace add":
		if len(pos) > 2 {
			return out(h.AddWorkspace(ctx, pos[1], pos[2]))
		}
	case "workspace list":
		return out(h.Workspaces(ctx))
	case "signup on", "signup off":
		on := pos[0] == "on"
		return out(h.SetSettings(ctx, &on, nil))
	case "plan list":
		return out(h.Plans(ctx))
	case "plan set":
		if len(pos) > 1 {
			data, err := cliData(pos[2:], stdin)
			if err != nil {
				return err
			}
			var p Plan
			b, _ := json.Marshal(data)
			if err := json.Unmarshal(b, &p); err != nil {
				return fmt.Errorf("plan: %w", err)
			}
			p.ID = pos[1]
			return out(h.SetPlan(ctx, p))
		}
	case "plan default":
		if len(pos) > 1 {
			id := pos[1]
			if id == "none" {
				id = ""
			}
			return out(h.SetSettings(ctx, nil, &id))
		}
	}
	return cliErrUsage
}

func cliFile(ctx context.Context, a *Archive, pos []string, space, output string, limit int, after string, stdout io.Writer, out func(any, error) error) error {
	if pos[0] == "list" {
		var aid uuid.UUID
		if after != "" {
			var err error
			if aid, err = uuid.Parse(after); err != nil {
				return fmt.Errorf("-after: %w", err)
			}
		}
		return out(a.Files(ctx, "", limit, aid))
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
			f, err := a.AddFile(ctx, space, filepath.Base(p), fh)
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

func cliSchema(ctx context.Context, a *Archive, pos []string, allowDrop bool, stdout io.Writer, out func(any, error) error) error {
	switch {
	case pos[0] == "pull":
		files, err := a.st.SchemaFiles(ctx)
		if err != nil {
			return err
		}
		if len(pos) > 1 {
			if err := os.MkdirAll(pos[1], 0o755); err != nil {
				return err
			}
			for name, body := range files {
				if err := os.WriteFile(filepath.Join(pos[1], name), []byte(body), 0o644); err != nil {
					return err
				}
			}
			return nil
		}
		for _, name := range specKeys(files) {
			fmt.Fprintf(stdout, "-- %s\n%s\n", name, strings.TrimRight(files[name], "\n"))
		}
		return nil
	case (pos[0] == "plan" || pos[0] == "apply") && len(pos) > 1:
		paths, err := filepath.Glob(filepath.Join(pos[1], "*.sql"))
		if err != nil {
			return err
		}
		files := map[string]string{}
		for _, p := range paths {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			files[filepath.Base(p)] = string(b)
		}
		return out(a.ApplySchema(ctx, files, nil, allowDrop, pos[0] == "plan"))
	}
	return cliErrUsage
}

func cliServe(ctx context.Context, h *Hub, a *Archive, addr string) error {
	if os.Getenv("ARCHIVE_URL") == "" {
		_, port, _ := net.SplitHostPort(addr)
		h.urls("http://localhost:" + cmp.Or(port, "80"))
	}
	si, err := newSignin(h)
	if err != nil {
		return err
	}
	if n, err := a.userCount(ctx); err != nil {
		return err
	} else if n == 0 {
		if email := os.Getenv("ARCHIVE_ADMIN_EMAIL"); email != "" {
			u, err := a.AddUser(ctx, email, "", true)
			if err != nil {
				return fmt.Errorf("ARCHIVE_ADMIN_EMAIL: %w", err)
			}
			fmt.Fprintf(os.Stderr, "archive: created the first owner %s; sign in at %s/auth/magic\n", u.Email, h.app)
		} else {
			si.setup = authNewSecret()
			fmt.Fprintf(os.Stderr, "archive: no users yet; create the first owner with POST %s/auth/setup {\"token\": \"%s\", \"email\": ...}\n", h.app, si.setup)
		}
	}
	mux := http.NewServeMux()
	mux.Handle("/api/", h.httpAuth((*Archive).handler))
	tus := h.httpAuth(func(a *Archive) http.Handler { return http.HandlerFunc(a.tus) })
	mux.Handle("/api/files/tus", tus)
	mux.Handle("/api/files/tus/", tus)
	mux.HandleFunc("GET /files/{id}", h.inWorkspace((*Archive).fileSigned))
	mux.HandleFunc("GET /files/{workspace}/{id}", h.inWorkspace((*Archive).fileSigned))
	mux.HandleFunc("POST /hooks/run/{token}", h.inWorkspace((*Archive).workflowHook))
	mux.HandleFunc("POST /hooks/run/{workspace}/{token}", h.inWorkspace((*Archive).workflowHook))
	mux.HandleFunc("POST /stripe", h.stripe)
	mux.Handle("/auth/", si.handler())
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("/mcp", h.mcp)
	var wg sync.WaitGroup
	bg, stop := context.WithCancel(ctx)
	defer wg.Wait()
	defer stop()
	wg.Add(hubWorkers + 2)
	for range hubWorkers {
		go h.work(bg, &wg)
	}
	go h.janitor(bg, &wg)
	go h.upgradeAll(bg, &wg)
	hs := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	errc := make(chan error, 1)
	go func() { errc <- hs.ListenAndServe() }()
	fmt.Fprintf(os.Stderr, "archive: serving the API on %s/api, MCP on %s/mcp and sign-in on %s/auth\n", h.api, h.api, h.app)
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return hs.Shutdown(sctx)
	}
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
		files, err := a.SchemaFiles(r.Context())
		return ok(map[string]any{"files": files}, err)
	})
	schema := func(dryRun bool) func(r *http.Request) (int, any, error) {
		return func(r *http.Request) (int, any, error) {
			var body struct {
				Files     map[string]string `json:"files"`
				Renames   []string          `json:"renames"`
				AllowDrop bool              `json:"allow_drop"`
			}
			if err := httpDecode(r, &body); err != nil {
				return 0, nil, err
			}
			return ok(a.ApplySchema(r.Context(), body.Files, body.Renames, body.AllowDrop, dryRun))
		}
	}
	handle("POST /api/schema/plan", schema(true))
	handle("POST /api/schema/apply", schema(false))

	a.fileRoutes(mux)
	a.workflowRoutes(mux)
	handle("GET /api/search", func(r *http.Request) (int, any, error) {
		q := r.URL.Query()
		limit, _ := strconv.Atoi(q.Get("limit"))
		f := SearchFilter{Type: q.Get("type"), Space: q.Get("space"), From: q.Get("from"), To: q.Get("to"), Limit: limit}
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
			if k != "limit" && k != "after" && k != "reveal" && k != "space" {
				where[k] = v[0]
			}
		}
		rs, err := a.List(r.Context(), r.PathValue("type"), q.Get("space"), where, limit, after)
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
		rec, err := a.Create(r.Context(), r.URL.Query().Get("space"), r.PathValue("type"), data)
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
	case errors.Is(err, ErrLimit):
		status = http.StatusPaymentRequired
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

func (a *Archive) mcpVerify(ctx context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
	if secretWorkspace(token) != "" && secretWorkspace(token) != a.ws.Short {
		return nil, fmt.Errorf("%w: this token belongs to another workspace", auth.ErrInvalidToken)
	}
	ac, err := a.Authenticate(ctx, token)
	if errors.Is(err, ErrUnauthorized) {
		return nil, fmt.Errorf("%w: %v", auth.ErrInvalidToken, err)
	}
	if err != nil {
		return nil, err
	}
	return &auth.TokenInfo{UserID: ac.UserID.String(), Extra: map[string]any{"actor": ac}}, nil
}
