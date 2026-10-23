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
	"text/tabwriter"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var cliVersion = "0.1.0"

const cliUsage = `archive keeps all your stuff in one place, in types you define.

Usage:
  archive [-as email] <command> ...  every command acts as a user (default the first owner)

  archive serve [-addr :8080]        serve the HTTP API (/api), sign-in (/auth) and MCP (/mcp)
  archive mcp                        serve MCP over stdio
  archive types                      list every type, built-in and user-defined

  archive spec pull [-all] [file]    write the user types and edited built-ins as JSON
                                     (default stdout); -all adds every built-in type
  archive spec push [-dry-run] <file>
                                     preview the diff against the ontology, then apply it;
                                     built-in types the file leaves out are kept

  archive intake <dir|file-id> -type <type> [-space s]
                                     read every new file in dir, or one stored file, into
                                     objects of type

  archive file add <path> [-space s] [-type t]
                                     store a file, extract its metadata and text, and with
                                     -type also read it into an object of type t
  archive file get <id> [-o path|-]  print the file's metadata, or with -o write its content
  archive file list [-limit n] [-after id]

  archive list <type> [-limit n] [-after id] [-reveal] [field=value ...]
  archive get <type> <id> [-reveal]  -reveal shows secret values you are granted
  archive create <type> [-space s] [json]
                                     json from the argument or stdin; default space is your own
  archive set <type> <id> [json]     set some fields; null clears a field
  archive update <type> <id> [json]  replace the whole object
  archive delete <type> <id>
  archive provenance <type> <id>
  archive links <type> <id>
  archive search <query> [-type t] [-space s] [-source src] [-from date] [-to date] [-limit n]
                                     find objects, files and unmapped synced items by their
                                     words and, with ARCHIVE_EMBED_MODEL, by meaning

  archive source add <kind> [-name n] [-space s] [-type t] [-schedule 15m] [-rate n] [json|-]
                                     connect a source: gmail, drive, imap, folder or external;
                                     json is {"config": {...}, "credentials": {...}}, and
                                     credentials are stored encrypted; gmail and drive print a
                                     Google sign-in address and read the code from stdin
  archive source list
  archive source run <id|name>       sync now: new items land in raw, then intake maps them
                                     into the source's type
  archive source remove <id|name>    stop syncing; raw data and objects stay

  archive demo                       load example types, spaces, records and review items
                                     into an archive that has none of the demo types yet

  archive review list [-status s] [-type t] [-space s] [-limit n] [-after id]
                                     what intake read: review (waiting for you), accepted
                                     (saved automatically), approved or rejected
  archive review counts [-space s]
  archive review approve <id> ...    save the proposed values as records
  archive review reject <id> ...

  archive backup -o <file|->          the whole archive in one file, while it serves or not: the
                                     database, the secret key and the stored files; keep it off
                                     this machine, like docker exec archive archive backup -o - > a.tgz
  archive restore <file|->           load a backup into an empty $ARCHIVE_DATA, with the server
                                     stopped; then start it as usual

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
  DATABASE_URL       external Postgres; unset starts the bundled one
  ARCHIVE_DATA       data directory for the bundled Postgres (default ./data)
  ARCHIVE_PG_BIN     directory holding the Postgres binaries; unset uses those on PATH, or
                     downloads Postgres into $ARCHIVE_DATA/pgbin on first run (without pgvector)
  ARCHIVE_LLM_URL    OpenAI-compatible endpoint (default http://localhost:11434/v1)
  ARCHIVE_LLM_MODEL  model for intake
  ARCHIVE_LLM_KEY    API key for the endpoint, if it needs one
  ARCHIVE_VISION_MODEL
                     vision model on the endpoint for OCR of photos and scans; unset skips OCR
  ARCHIVE_WHISPER_MODEL
                     speech-to-text model on the endpoint for audio and video; unset skips it
  ARCHIVE_EMBED_MODEL
                     embedding model on the endpoint for semantic search (needs pgvector);
                     unset leaves search to full text only
  ARCHIVE_EMBED_DIM  dimensions of ARCHIVE_EMBED_MODEL's vectors (default 768)
  ARCHIVE_BLOB_URL   s3://bucket/prefix?endpoint=https://host&region=r for S3, R2, MinIO or GCS;
                     unset stores files in $ARCHIVE_DATA/blobs
  ARCHIVE_BLOB_KEY, ARCHIVE_BLOB_SECRET
                     access key for ARCHIVE_BLOB_URL (default AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
  ARCHIVE_LABELS     labeled examples (JSON) for calibrating intake confidence
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

// cliBackup writes a backup to path, or stdout for -, and only puts a file at path once the backup
// is whole.
func cliBackup(ctx context.Context, path string, stdout io.Writer) error {
	dir := coreEnv("ARCHIVE_DATA", "./data")
	if path == "-" {
		return Backup(ctx, dir, stdout)
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".partial-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	err = Backup(ctx, dir, f)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
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
	case "backup":
		if len(args) != 2 || args[0] != "-o" {
			return cliErrUsage
		}
		return cliBackup(ctx, args[1], stdout)
	case "restore":
		if len(args) != 1 {
			return cliErrUsage
		}
		in := stdin
		if args[0] != "-" {
			f, err := os.Open(args[0])
			if err != nil {
				return err
			}
			defer f.Close()
			in = f
		}
		return Restore(ctx, coreEnv("ARCHIVE_DATA", "./data"), in)
	}

	fs := flag.NewFlagSet("archive "+cmd, flag.ContinueOnError)
	fs.Usage = func() { fmt.Fprint(fs.Output(), cliUsage) }
	addr := fs.String("addr", ":8080", "listen address (serve)")
	dryRun := fs.Bool("dry-run", false, "only preview (spec push)")
	typ := fs.String("type", "", "target type (intake)")
	limit := fs.Int("limit", 100, "page size (list)")
	after := fs.String("after", "", "list objects after this id (list)")
	all := fs.Bool("all", false, "include every built-in type (spec pull)")
	as := fs.String("as", "", "act as this user")
	name := fs.String("name", "", "name (user add, token create)")
	admin := fs.Bool("admin", false, "instance admin (user add)")
	full := fs.Bool("full", false, "token with all your rights (token create)")
	space := fs.String("space", "", "space (create, intake); space:role,... (token create)")
	expires := fs.Duration("expires", 0, "token lifetime (token create)")
	reveal := fs.Bool("reveal", false, "show granted secret values (list, get)")
	schedule := fs.String("schedule", "", "sync every duration, like 15m (source add)")
	rateLimit := fs.Float64("rate", 0, "requests per second to the platform (source add)")
	output := fs.String("o", "", "write the file content here, - for stdout (file get)")
	source := fs.String("source", "", "only items from this source (search)")
	from := fs.String("from", "", "only items added on or after this date (search)")
	to := fs.String("to", "", "only items added up to this date (search)")
	status := fs.String("status", "", "review, accepted, approved or rejected (review list)")
	if cmd == "-as" && len(args) > 1 {
		*as, cmd, args = args[0], args[1], args[2:]
	}
	pos, err := cliParse(fs, args)
	if err != nil {
		return err
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
	if ac == nil && cmd != "serve" && cmd != "types" && cmd != "user" && (cmd != "spec" || pos[0] != "pull") {
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
		return cliTypes(stdout, a.Spec())
	case "spec":
		return cliSpec(ctx, a, pos, *all, *dryRun, stdin, stdout)
	case "intake":
		if *typ == "" {
			return cliErrUsage
		}
		if id, err := uuid.Parse(pos[0]); err == nil {
			return out(a.IntakeFile(ctx, id, *typ))
		}
		return out(a.IntakeDir(ctx, *space, pos[0], *typ))
	case "file":
		return cliFile(ctx, a, pos, *space, *typ, *output, *limit, *after, stdout, out)
	case "source":
		in := SourceInput{Name: *name, Space: *space, Type: *typ, Schedule: *schedule, Rate: *rateLimit}
		return cliSource(ctx, a, pos, in, stdin, stdout, out)
	case "demo":
		return out(a.LoadDemo(ctx))
	case "review":
		return cliReview(ctx, a, pos, ReviewFilter{Status: *status, Type: *typ, Space: *space, Limit: *limit}, *after, out)
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
			err = a.Reveal(ctx, rs...)
		}
		return out(rs, err)
	case "search":
		f := SearchFilter{Type: *typ, Space: *space, Source: *source, From: *from, To: *to, Limit: *limit}
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
			err = a.Reveal(ctx, r)
		}
		return out(r, err)
	case "links":
		return out(a.Links(ctx, pos[0], id))
	case "provenance":
		return out(a.Provenance(ctx, pos[0], id))
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
		"serve": 0, "mcp": 0, "types": 0, "spec": 1, "intake": 1, "list": 1, "create": 1,
		"get": 2, "provenance": 2, "delete": 2, "set": 2, "update": 2, "links": 2,
		"user": 2, "space": 1, "token": 1, "secret": 3, "source": 1, "file": 1, "search": 1, "review": 1, "demo": 0,
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

func cliFile(ctx context.Context, a *Archive, pos []string, space, typ, output string, limit int, after string, stdout io.Writer, out func(any, error) error) error {
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
		f, err := a.AddFile(ctx, space, filepath.Base(pos[1]), typ, fh)
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

func cliTypes(w io.Writer, s *Spec) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	for _, name := range specKeys(s.Types) {
		t, err := s.resolve(name)
		if err != nil {
			return err
		}
		kind, origin := "composite", "user"
		if t.Schema != nil {
			kind = "scalar"
		}
		if t.Builtin {
			origin = map[bool]string{false: "built-in", true: "built-in, edited"}[t.Edited]
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", name, kind, t.Category, cmp.Or(t.Sensitivity, Public), origin)
	}
	return tw.Flush()
}

func cliPullSpec(s *Spec, all bool) *Spec {
	out := &Spec{Version: s.Version, Types: map[string]*Type{}}
	for name, t := range s.Types {
		if all || !t.Builtin || t.Edited {
			out.Types[name] = t
		}
	}
	return out
}

func cliSpec(ctx context.Context, a *Archive, pos []string, all, dryRun bool, stdin io.Reader, stdout io.Writer) error {
	switch {
	case pos[0] == "pull":
		b, err := cliPullSpec(a.Spec(), all).JSON()
		if err != nil {
			return err
		}
		if len(pos) > 1 && pos[1] != "-" {
			return os.WriteFile(pos[1], b, 0o644)
		}
		_, err = stdout.Write(b)
		return err
	case pos[0] == "push" && len(pos) > 1:
		var b []byte
		var err error
		if pos[1] == "-" {
			b, err = io.ReadAll(stdin)
		} else {
			b, err = os.ReadFile(pos[1])
		}
		if err != nil {
			return err
		}
		p, err := a.ApplySpec(ctx, b, dryRun)
		if p == nil && err == nil {
			fmt.Fprintln(os.Stderr, "spec is up to date")
			return nil
		}
		if p != nil {
			if err := cliJSON(stdout, p); err != nil {
				return err
			}
		}
		if err == nil && !dryRun {
			fmt.Fprintf(os.Stderr, "applied change %s: spec v%d\n", p.Change.ID, p.Change.ToVer)
		}
		return err
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
			fmt.Fprintf(os.Stderr, "archive: no users yet; create the first owner at %s/setup?token=%s\n", base, si.setup)
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
		if webBrowser(r) {
			to := "/sources"
			if s != nil {
				to += "?connected=" + url.QueryEscape(s.Name)
			}
			webRedirect(w, r, to, err)
			return
		}
		httpWrite(w, http.StatusOK, s, err)
	})
	tus := a.httpAuth(http.HandlerFunc(a.tus))
	mux.Handle("/api/files/tus", tus)
	mux.Handle("/api/files/tus/", tus)
	mux.HandleFunc("GET /files/{id}", a.fileSigned)
	mux.Handle("/auth/", si.handler())
	mux.Handle("/", webHandler())
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
		_, err := a.st.Q(ctx).Exec(ctx, `UPDATE files SET status = 'pending' WHERE status = 'processing'`)
		return err
	}); err != nil {
		return err
	}
	for range mediaWorkers {
		go a.fileLoop(syncCtx, &wg)
	}
	if a.embedOff == "" {
		wg.Add(1)
		go a.embedLoop(syncCtx, &wg)
	} else if a.intake.cfg.Embed != "" {
		fmt.Fprintf(os.Stderr, "archive: semantic search is off (%s); full-text search still works\n", a.embedOff)
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

	handle("GET /api/types", func(r *http.Request) (int, any, error) {
		spec := a.Spec()
		builtin, edited := []string{}, []string{}
		for _, name := range specKeys(spec.Types) {
			if t := spec.Types[name]; t.Builtin {
				builtin = append(builtin, name)
				if t.Edited {
					edited = append(edited, name)
				}
			}
		}
		return ok(map[string]any{"spec": spec, "builtin": builtin, "edited": edited}, nil)
	})
	handle("GET /api/changes", func(r *http.Request) (int, any, error) {
		return ok(a.Changes(r.Context()))
	})
	handle("GET /api/types/{name}", func(r *http.Request) (int, any, error) {
		return ok(httpType(a.Spec(), r.PathValue("name")))
	})
	handle("POST /api/changes", func(r *http.Request) (int, any, error) {
		var body struct {
			Ops []Op `json:"ops"`
		}
		if err := httpDecode(r, &body); err != nil {
			return 0, nil, err
		}
		c, err := a.Propose(r.Context(), body.Ops)
		return http.StatusCreated, c, err
	})
	change := func(f func(context.Context, uuid.UUID) (any, error)) func(r *http.Request) (int, any, error) {
		return func(r *http.Request) (int, any, error) {
			id, err := httpID(r)
			if err != nil {
				return 0, nil, err
			}
			return ok(f(r.Context(), id))
		}
	}
	handle("POST /api/changes/{id}/preview", change(func(ctx context.Context, id uuid.UUID) (any, error) { return a.Preview(ctx, id) }))
	handle("POST /api/changes/{id}/apply", change(func(ctx context.Context, id uuid.UUID) (any, error) { return a.Apply(ctx, id) }))
	handle("POST /api/changes/{id}/revert", change(func(ctx context.Context, id uuid.UUID) (any, error) { return a.Revert(ctx, id) }))

	a.fileRoutes(mux)
	handle("GET /api/search", func(r *http.Request) (int, any, error) {
		q := r.URL.Query()
		limit, _ := strconv.Atoi(q.Get("limit"))
		f := SearchFilter{Type: q.Get("type"), Space: q.Get("space"), Source: q.Get("source"), From: q.Get("from"), To: q.Get("to"), Limit: limit}
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

	handle("POST /api/demo", func(r *http.Request) (int, any, error) {
		return ok(a.LoadDemo(r.Context()))
	})
	handle("GET /api/review", func(r *http.Request) (int, any, error) {
		q := r.URL.Query()
		limit, _ := strconv.Atoi(q.Get("limit"))
		after, _ := uuid.Parse(q.Get("after"))
		return ok(a.Proposals(r.Context(), ReviewFilter{Status: q.Get("status"), Type: q.Get("type"), Space: q.Get("space"), Limit: limit, After: after}))
	})
	handle("GET /api/review/counts", func(r *http.Request) (int, any, error) {
		return ok(a.ProposalCounts(r.Context(), r.URL.Query().Get("space")))
	})
	handle("GET /api/review/{id}", change(func(ctx context.Context, id uuid.UUID) (any, error) { return a.GetProposal(ctx, id) }))
	handle("POST /api/review/approve", func(r *http.Request) (int, any, error) {
		var body struct {
			IDs   []uuid.UUID                  `json:"ids"`
			Edits map[uuid.UUID]map[string]any `json:"edits"`
		}
		if err := httpDecode(r, &body); err != nil {
			return 0, nil, err
		}
		return ok(a.Approve(r.Context(), body.IDs, body.Edits))
	})
	handle("POST /api/review/reject", func(r *http.Request) (int, any, error) {
		var body struct {
			IDs []uuid.UUID `json:"ids"`
		}
		if err := httpDecode(r, &body); err != nil {
			return 0, nil, err
		}
		return ok(a.Reject(r.Context(), body.IDs))
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
	handle("GET /api/views/{id}", change(func(ctx context.Context, id uuid.UUID) (any, error) { return a.GetView(ctx, id) }))
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
		if rs == nil {
			rs = []*Record{}
		}
		if err == nil && q.Get("reveal") == "true" {
			err = a.Reveal(r.Context(), rs...)
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
	withData := func(f func(ctx context.Context, typ string, id uuid.UUID, data map[string]any) (*Record, error)) func(r *http.Request) (int, any, error) {
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
			err = a.Reveal(ctx, rec)
		}
		return rec, err
	}))
	handle("GET /api/{type}/{id}/links", object(func(ctx context.Context, typ string, id uuid.UUID, _ *http.Request) (any, error) {
		return a.Links(ctx, typ, id)
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
	handle("GET /api/{type}/{id}/provenance", object(func(ctx context.Context, typ string, id uuid.UUID, _ *http.Request) (any, error) {
		return a.Provenance(ctx, typ, id)
	}))
	return mux
}

func httpType(s *Spec, name string) (any, error) {
	t, ok := s.Types[name]
	if !ok {
		return nil, fmt.Errorf("type %q: %w", name, ErrNotFound)
	}
	fields, err := s.Fields(name)
	if err != nil {
		return nil, err
	}
	schema, err := s.JSONSchema(name)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"name": name, "description": t.Description, "extends": t.Extends,
		"fields": fields, "subtypes": s.Subtypes(name), "schema": schema,
	}, nil
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
