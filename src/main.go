package main

import (
	"bufio"
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
  archive [-as email] <command> ...  every command acts as a user (default the first owner)

  archive serve [-addr :8080]        serve the HTTP API (/api), sign-in (/auth) and MCP (/mcp)
  archive mcp                        serve MCP over stdio
  archive types                      print the tables, their columns and the built-in types
  archive run <file.ts|->            run TypeScript against the archive SDK in one transaction,
                                     as MCP's run tool does, and print {result, logs, error}
  archive run -docs                  print the SDK documentation, with an interface per table

  archive schema plan [-allow-drop]  show the statements that bring the database to the DDL in
                                     $ARCHIVE_DATA/schema/*.sql; statements that delete data are
                                     blocked unless -allow-drop
  archive schema apply [-allow-drop] run that plan
  archive schema pull [dir]          print the DDL files, or write them into dir
  archive schema push <dir> [-allow-drop]
                                     plan and apply the *.sql files in dir, then make them the
                                     DDL files; renames are not detected, so rename a table or
                                     column with ALTER ... RENAME in psql first

  archive file add <path> [-space s] store a file and extract its metadata and text
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

  archive source add <kind> [-name n] [-space s] [-schedule 15m] [-rate n] [json|-]
                                     connect a source: gmail, drive or folder; it stores every
                                     new mail, attachment and file as a file;
                                     json is {"config": {...}, "credentials": {...}}, and
                                     credentials are stored encrypted; gmail and drive print a
                                     Google sign-in address and read the code from stdin
  archive source list
  archive source run <id|name>       sync now
  archive source remove <id|name>    stop syncing; files stay

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

Environment:
  DATABASE_URL       external Postgres, as a superuser; unset starts the bundled one
  ARCHIVE_DATA       data directory: the bundled Postgres, files and schema/*.sql (default ./data)
  ARCHIVE_PG_BIN     directory holding the Postgres binaries; unset uses those on PATH, or
                     downloads Postgres into $ARCHIVE_DATA/pgbin on first run
  ARCHIVE_LOCALE     locale that messy input is read in, like nl-NL
  ARCHIVE_RUN_TIMEOUT
                     time limit of one archive run, POST /api/run or MCP run (default 30s)
  ARCHIVE_BLOB_URL   s3://bucket/prefix?endpoint=https://host&region=r for S3, R2, MinIO or GCS;
                     unset stores files in $ARCHIVE_DATA/blobs
  ARCHIVE_BLOB_KEY, ARCHIVE_BLOB_SECRET
                     access key for ARCHIVE_BLOB_URL (default AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
  ARCHIVE_INSTANCE   prefix of every Postgres role of this instance (default a)
  ARCHIVE_SECRET_KEY 32-byte key (hex or base64) for secret values; default $ARCHIVE_DATA/secret.key
  ARCHIVE_URL        public base URL for links, cookies and passkeys (default http://localhost:<port>)
  ARCHIVE_ADMIN_EMAIL
                     first owner that serve creates when there are no users yet
  ARCHIVE_SMTP_URL   smtp://user:pass@host:587 for sign-in mail; unset prints links to the log
  ARCHIVE_SMTP_FROM  sender address for sign-in mail
  ARCHIVE_OIDC       JSON list of OIDC providers: [{"name","issuer","client_id","client_secret"}]
  ARCHIVE_GOOGLE_CLIENT_ID, ARCHIVE_GOOGLE_CLIENT_SECRET
                     Google OAuth client for the gmail and drive sources; its redirect URI is
                     $ARCHIVE_URL/api/sources/oauth/callback
  ARCHIVE_GOOGLE_API_URL, ARCHIVE_GOOGLE_TOKEN_URL
                     Google API base and token endpoint (default the real Google)
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
	schedule := fs.String("schedule", "", "sync every duration, like 15m (source add)")
	rateLimit := fs.Float64("rate", 0, "requests per second to the platform (source add)")
	output := fs.String("o", "", "write the file content here, - for stdout (file get)")
	docs := fs.Bool("docs", false, "print the SDK documentation (run)")
	from := fs.String("from", "", "only items added on or after this date (search)")
	to := fs.String("to", "", "only items added up to this date (search)")
	if cmd == "-as" && len(args) > 1 {
		*as, cmd, args = args[0], args[1], args[2:]
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

	a, err := Open(ctx)
	if err != nil {
		return err
	}
	defer a.Close()
	ac, err := a.CLIActor(ctx, *as)
	if err != nil {
		return err
	}
	if ac == nil && cmd != "serve" && cmd != "types" && cmd != "user" && (cmd != "schema" || pos[0] != "pull") {
		return errors.New("no users yet: create the first owner with archive user add <email>, or run archive serve with ARCHIVE_ADMIN_EMAIL")
	}
	ctx = WithActor(ctx, ac)
	out := func(v any, err error) error {
		if err != nil {
			return err
		}
		return cliJSON(stdout, v)
	}

	switch cmd {
	case "serve":
		return cliServe(ctx, a, *addr)
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
	case "source":
		in := SourceInput{Name: *name, Space: *space, Schedule: *schedule, Rate: *rateLimit}
		return cliSource(ctx, a, pos, in, stdin, stdout, out)
	case "user", "space", "token", "secret":
		return cliAuth(ctx, a, cmd, pos, *name, *admin, *full, *space, *expires, out)
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
		"user": 2, "space": 1, "token": 1, "secret": 3, "source": 1, "file": 1, "search": 1,
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

func cliSource(ctx context.Context, a *Archive, pos []string, in SourceInput, stdin io.Reader, stdout io.Writer, out func(any, error) error) error {
	switch {
	case pos[0] == "list":
		return out(a.Sources(ctx))
	case pos[0] == "add" && len(pos) > 1:
		if len(pos) > 2 {
			var r io.Reader = strings.NewReader(pos[2])
			if pos[2] == "-" {
				r = stdin
			}
			if err := json.NewDecoder(r).Decode(&in); err != nil {
				return fmt.Errorf("reading source JSON: %w", err)
			}
		}
		in.Kind, in.Name = pos[1], cmp.Or(in.Name, pos[1])
		s, authURL, err := a.AddSource(ctx, in)
		if err != nil || authURL == "" {
			return out(s, err)
		}
		fmt.Fprintf(os.Stderr, "Open this address to let Archive read your %s:\n\n  %s\n\nThen paste the code, or the whole address you were sent back to: ", in.Kind, authURL)
		line, _ := bufio.NewReader(stdin).ReadString('\n')
		code, state := strings.TrimSpace(line), ""
		if u, err := url.Parse(authURL); err == nil {
			state = u.Query().Get("state")
		}
		if u, err := url.Parse(code); err == nil && u.Query().Get("code") != "" {
			code, state = u.Query().Get("code"), cmp.Or(u.Query().Get("state"), state)
		}
		if code == "" {
			fmt.Fprintln(os.Stderr, "\nno code; the source waits until the browser finishes at the address above")
			return out(s, nil)
		}
		return out(a.SourceOAuth(ctx, state, code))
	case pos[0] == "run" && len(pos) > 1:
		res, err := a.RunSource(ctx, pos[1])
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
	case pos[0] == "remove" && len(pos) > 1:
		return a.RemoveSource(ctx, pos[1])
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
		fh, err := os.Open(pos[1])
		if err != nil {
			return err
		}
		defer fh.Close()
		f, err := a.AddFile(ctx, space, filepath.Base(pos[1]), fh)
		if err != nil {
			return err
		}
		return out(a.ProcessFile(ctx, f.ID))
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
	case pos[0] == "plan":
		return out(a.ApplySchema(ctx, nil, nil, allowDrop, true))
	case pos[0] == "apply":
		return out(a.ApplySchema(ctx, nil, nil, allowDrop, false))
	case pos[0] == "pull":
		files, err := a.st.SchemaFiles()
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
	case pos[0] == "push" && len(pos) > 1:
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
		return out(a.ApplySchema(ctx, files, nil, allowDrop, false))
	}
	return cliErrUsage
}

func cliServe(ctx context.Context, a *Archive, addr string) error {
	base := os.Getenv("ARCHIVE_URL")
	if base == "" {
		_, port, _ := net.SplitHostPort(addr)
		base = "http://localhost:" + cmp.Or(port, "80")
	}
	a.base = strings.TrimRight(base, "/")
	si, err := newSignin(a, base)
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
			fmt.Fprintf(os.Stderr, "archive: created the first owner %s; sign in at %s/auth/magic\n", u.Email, base)
		} else {
			si.setup = authNewSecret()
			fmt.Fprintf(os.Stderr, "archive: no users yet; create the first owner with POST %s/auth/setup {\"token\": \"%s\", \"email\": ...}\n", base, si.setup)
		}
	}
	mux := http.NewServeMux()
	mux.Handle("/api/", a.httpAuth(httpHandler(a)))
	mux.HandleFunc("GET /api/sources/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		var s *Source
		err := fmt.Errorf("authorization: %s: %w", q.Get("error"), ErrUnauthorized)
		if q.Get("error") == "" {
			s, err = a.SourceOAuth(r.Context(), q.Get("state"), q.Get("code"))
		}
		httpWrite(w, http.StatusOK, s, err)
	})
	tus := a.httpAuth(http.HandlerFunc(a.tus))
	mux.Handle("/api/files/tus", tus)
	mux.Handle("/api/files/tus/", tus)
	mux.HandleFunc("GET /files/{id}", a.fileSigned)
	mux.Handle("/auth/", si.handler())
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	srv := mcpServer(a)
	mux.Handle("/mcp", auth.RequireBearerToken(a.mcpVerify, &auth.RequireBearerTokenOptions{AllowMissingExpiration: true})(
		mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)))
	hs := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	errc := make(chan error, 1)
	go func() { errc <- hs.ListenAndServe() }()
	var wg sync.WaitGroup
	syncCtx, stopSync := context.WithCancel(ctx)
	defer wg.Wait()
	defer stopSync()
	wg.Add(1 + mediaWorkers)
	go a.syncLoop(syncCtx, &wg)
	if err := a.st.System(ctx, func(ctx context.Context) error {
		_, err := a.st.Q(ctx).Exec(ctx, `UPDATE file SET status = 'pending' WHERE status = 'processing'`)
		return err
	}); err != nil {
		return err
	}
	for range mediaWorkers {
		go a.fileLoop(syncCtx, &wg)
	}
	fmt.Fprintf(os.Stderr, "archive: serving API on %s/api, sign-in on %s/auth and MCP on %s/mcp\n", base, base, base)
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
	handle("GET /api/search", func(r *http.Request) (int, any, error) {
		q := r.URL.Query()
		limit, _ := strconv.Atoi(q.Get("limit"))
		f := SearchFilter{Type: q.Get("type"), Space: q.Get("space"), From: q.Get("from"), To: q.Get("to"), Limit: limit}
		return ok(a.Search(r.Context(), q.Get("q"), f))
	})
	handle("GET /api/sources", func(r *http.Request) (int, any, error) {
		return ok(a.Sources(r.Context()))
	})
	handle("POST /api/sources", func(r *http.Request) (int, any, error) {
		var in SourceInput
		if err := httpDecode(r, &in); err != nil {
			return 0, nil, err
		}
		s, authURL, err := a.AddSource(r.Context(), in)
		return http.StatusCreated, struct {
			*Source
			AuthURL string `json:"auth_url,omitempty"`
		}{s, authURL}, err
	})
	handle("GET /api/sources/{id}", func(r *http.Request) (int, any, error) {
		return ok(a.GetSource(r.Context(), r.PathValue("id")))
	})
	handle("PATCH /api/sources/{id}", func(r *http.Request) (int, any, error) {
		var p SourcePatch
		if err := httpDecode(r, &p); err != nil {
			return 0, nil, err
		}
		return ok(a.UpdateSource(r.Context(), r.PathValue("id"), p))
	})
	handle("POST /api/sources/{id}/run", func(r *http.Request) (int, any, error) {
		return ok(a.RunSource(r.Context(), r.PathValue("id")))
	})
	handle("DELETE /api/sources/{id}", func(r *http.Request) (int, any, error) {
		return http.StatusNoContent, nil, a.RemoveSource(r.Context(), r.PathValue("id"))
	})

	handle("GET /api/views", func(r *http.Request) (int, any, error) {
		return ok(a.Views(r.Context()))
	})
	handle("POST /api/views", func(r *http.Request) (int, any, error) {
		var in ViewInput
		if err := httpDecode(r, &in); err != nil {
			return 0, nil, err
		}
		v, err := a.AddView(r.Context(), in)
		return http.StatusCreated, v, err
	})
	handle("GET /api/views/{id}", func(r *http.Request) (int, any, error) {
		id, err := httpID(r)
		if err != nil {
			return 0, nil, err
		}
		return ok(a.GetView(r.Context(), id))
	})
	handle("PUT /api/views/{id}", func(r *http.Request) (int, any, error) {
		id, err := httpID(r)
		if err != nil {
			return 0, nil, err
		}
		var in ViewInput
		if err := httpDecode(r, &in); err != nil {
			return 0, nil, err
		}
		return ok(a.SetView(r.Context(), id, in))
	})
	handle("DELETE /api/views/{id}", func(r *http.Request) (int, any, error) {
		id, err := httpID(r)
		if err != nil {
			return 0, nil, err
		}
		return http.StatusNoContent, nil, a.RemoveView(r.Context(), id)
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
