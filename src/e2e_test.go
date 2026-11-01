package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"cmp"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"maps"
	"math/big"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var e2eBin, e2eTemplate string

var e2eCloudTasks *e2eTasks

var e2eCopyFlag = map[string]string{"darwin": "-c", "linux": "--reflink=auto"}[runtime.GOOS]

func TestMain(m *testing.M) {
	if os.Getenv("ARCHIVE_E2E_MAIN") == "1" {
		main()
		os.Exit(0)
	}
	os.Setenv("ARCHIVE_E2E_MAIN", "1")
	var err error
	if e2eBin, err = os.Executable(); err != nil {
		panic(err)
	}
	dir, err := os.MkdirTemp("", "arcbin")
	if err != nil {
		panic(err)
	}
	e2eTemplate = filepath.Join(dir, "template")
	if err := e2eTemplateDB(e2eTemplate); err != nil {
		os.RemoveAll(dir)
		panic(fmt.Sprintf("template database: %v", err))
	}
	if e2eCloudTasks, err = e2eTasksStart(false); err != nil {
		os.RemoveAll(dir)
		panic(fmt.Sprintf("cloud tasks: %v", err))
	}
	flag.Set("test.parallel", "32")
	code := m.Run()
	e2eCloudTasks.stop()
	os.RemoveAll(dir)
	os.Exit(code)
}

func e2eTable(name, cols string) string {
	return fmt.Sprintf(`CREATE TABLE %[1]s (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    access     text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
%[2]s
);
ALTER TABLE %[1]s ENABLE ROW LEVEL SECURITY;
ALTER TABLE %[1]s FORCE ROW LEVEL SECURITY;
CREATE POLICY access ON %[1]s USING (archive.can(access)) WITH CHECK (archive.can(access));
`, name, cols)
}

var e2eSchema = map[string]string{
	"company.sql": e2eTable("company", `    name       text NOT NULL,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(name)) STORED`) + "COMMENT ON TABLE company IS 'A company';\n",
	"contact.sql": "CREATE DOMAIN zip AS text CHECK (VALUE ~ '^[0-9]{5}(-[0-9]{4})?$');\n" + e2eTable("contact", `    name       text NOT NULL,
    zip        zip,
    email      archive.email,
    phone      archive.phone,
    birthday   date,
    balance    archive.money,
    tags       text[],
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(name, email)) STORED`),
	"person.sql": e2eTable("person", `    name       text NOT NULL,
    ssn        archive.us_ssn,
    employer   uuid REFERENCES company,
    friends    uuid[],
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(name, ssn)) STORED`) + "COMMENT ON COLUMN person.friends IS 'references person; people they know';\n",
	"invoice.sql": "CREATE TYPE invoice_status AS ENUM ('draft', 'sent', 'paid');\n" + e2eTable("invoice", `    number     text NOT NULL,
    total      archive.money,
    date       date,
    ein        archive.us_ein,
    status     invoice_status,
    document   uuid REFERENCES archive.file,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(number)) STORED`),
}

type e2eInst struct {
	t   *testing.T
	dir string
	env []string
}

func newInst(t *testing.T, env ...string) *e2eInst {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "arc")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	if out, err := exec.Command("cp", e2eCopyFlag, "-R", e2eTemplate+"/.", dir).CombinedOutput(); err != nil {
		t.Fatalf("copy template: %v %s", err, out)
	}
	stop, err := e2ePostgres(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(stop)
	return &e2eInst{t: t, dir: dir, env: append(append([]string{"DATABASE_URL=host=" + dir + " user=archive dbname=archive"}, e2eCloudTasks.queue(t, dir)...), env...)}
}

type e2eTasks struct {
	addr   string
	hold   bool
	issuer *httptest.Server
	meta   *httptest.Server
	srv    *httptest.Server

	mu     sync.Mutex
	names  map[string]bool
	posted []hubTask
	routes map[string]*e2eRoute
}

type e2eRoute struct{ url, aud string }

func e2eTasksStart(hold bool) (*e2eTasks, error) {
	issuer, err := e2eIssuerServer()
	if err != nil {
		return nil, err
	}
	meta := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Metadata-Flavor") != "Google" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		switch r.URL.Path {
		case "/computeMetadata/v1/project/project-id":
			io.WriteString(w, "e2e")
		case "/computeMetadata/v1/instance/region":
			io.WriteString(w, "projects/123/regions/here")
		case "/computeMetadata/v1/instance/service-accounts/default/token":
			json.NewEncoder(w).Encode(map[string]any{"access_token": "meta", "expires_in": 3600, "token_type": "Bearer"})
		default:
			http.NotFound(w, r)
		}
	}))
	f := &e2eTasks{hold: hold, issuer: issuer, meta: meta, names: map[string]bool{}, routes: map[string]*e2eRoute{}}
	f.srv = httptest.NewServer(http.HandlerFunc(f.createTask))
	f.addr = f.srv.URL
	return f, nil
}

func (f *e2eTasks) stop() {
	f.srv.Close()
	f.issuer.Close()
	f.meta.Close()
}

func (f *e2eTasks) queue(t *testing.T, dir string) []string {
	q := "projects/e2e/locations/here/queues/" + filepath.Base(dir)
	f.mu.Lock()
	f.routes[q] = &e2eRoute{}
	f.mu.Unlock()
	t.Cleanup(func() {
		f.mu.Lock()
		delete(f.routes, q)
		f.mu.Unlock()
	})
	return []string{"ARCHIVE_TASKS_URL=" + f.addr, "ARCHIVE_TASKS_ISSUER=" + f.issuer.URL, "GCE_METADATA_HOST=" + strings.TrimPrefix(f.meta.URL, "http://"), "K_SERVICE=" + filepath.Base(dir)}
}

func (f *e2eTasks) route(queue, url, aud string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r := f.routes[queue]; r != nil {
		r.url, r.aud = url, aud
	}
}

func (f *e2eTasks) createTask(w http.ResponseWriter, r *http.Request) {
	fail := func(code int, msg string) {
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": code, "message": msg}})
	}
	parent, ok := strings.CutSuffix(strings.TrimPrefix(r.URL.Path, "/v2/"), "/tasks")
	var req struct{ Task hubTask }
	if r.Method != "POST" || !ok || r.Header.Get("Content-Type") != "application/json" || json.NewDecoder(r.Body).Decode(&req) != nil {
		fail(http.StatusBadRequest, "want POST /v2/<queue>/tasks with a JSON task")
		return
	}
	tk := req.Task
	if !strings.HasPrefix(tk.Name, parent+"/tasks/") || tk.HTTPRequest.URL == "" {
		fail(http.StatusBadRequest, "want a named HTTP task in the queue")
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.posted = append(f.posted, tk)
	if f.names[tk.Name] {
		fail(http.StatusConflict, "task exists")
		return
	}
	f.names[tk.Name] = true
	if !f.hold {
		go f.deliver(parent, tk)
	}
	json.NewEncoder(w).Encode(tk)
}

func (f *e2eTasks) deliver(queue string, tk hubTask) {
	time.Sleep(time.Until(tk.ScheduleTime))
	for wait := 50 * time.Millisecond; ; wait = min(2*wait, time.Second) {
		f.mu.Lock()
		r, ok := f.routes[queue]
		var to e2eRoute
		if ok {
			to = *r
		}
		f.mu.Unlock()
		if !ok {
			return
		}
		if to.url != "" {
			if tok, err := f.token(map[string]any{"aud": to.aud, "sub": "1", "email": tk.HTTPRequest.OIDCToken.ServiceAccountEmail, "email_verified": true}); err == nil {
				req, _ := http.NewRequest("POST", to.url+"/tasks", bytes.NewReader(tk.HTTPRequest.Body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+tok)
				if resp, err := http.DefaultClient.Do(req); err == nil {
					resp.Body.Close()
					if resp.StatusCode/100 == 2 {
						return
					}
				}
			}
		}
		time.Sleep(wait)
	}
}

func (f *e2eTasks) token(claims map[string]any) (string, error) {
	code, _ := json.Marshal(claims)
	resp, err := http.PostForm(f.issuer.URL+"/token", url.Values{"code": {string(code)}})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		IDToken string `json:"id_token"`
	}
	return out.IDToken, json.NewDecoder(resp.Body).Decode(&out)
}

func (f *e2eTasks) tasks() []hubTask {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.posted)
}

func (in *e2eInst) queue() string {
	if u, err := url.Parse(in.getenv("ARCHIVE_TASKS_URL")); err == nil && strings.Trim(u.Path, "/") != "" {
		return strings.Trim(u.Path, "/")
	}
	return "projects/e2e/locations/here/queues/" + in.getenv("K_SERVICE")
}

func (in *e2eInst) getenv(key string) string {
	for _, kv := range slices.Backward(in.env) {
		if v, ok := strings.CutPrefix(kv, key+"="); ok {
			return v
		}
	}
	return ""
}

func e2ePostgres(dir string) (func(), error) {
	pg := exec.Command("postgres", "-D", filepath.Join(dir, "pg"), "-c", "listen_addresses=", "-k", dir)
	pg.Env = append(os.Environ(), "LC_ALL=C")
	if err := pg.Start(); err != nil {
		return nil, err
	}
	stop := func() {
		pg.Process.Signal(syscall.SIGINT)
		pg.Wait()
	}
	for deadline := time.Now().Add(30 * time.Second); exec.Command("pg_isready", "-q", "-h", dir).Run() != nil; time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			stop()
			return nil, errors.New("postgres did not start")
		}
	}
	return stop, nil
}

func e2eTemplateDB(dir string) error {
	initdb := exec.Command("initdb", "-D", filepath.Join(dir, "pg"), "-U", "archive", "--auth-local=trust", "--auth-host=reject", "-E", "UTF8", "--locale=C")
	initdb.Env = append(os.Environ(), "LC_ALL=C")
	if out, err := initdb.CombinedOutput(); err != nil {
		return fmt.Errorf("initdb: %v\n%s", err, out)
	}
	key := make([]byte, 32)
	rand.Read(key)
	os.MkdirAll(filepath.Join(dir, "data"), 0o700)
	if err := os.WriteFile(filepath.Join(dir, "data", "secret.key"), []byte(hex.EncodeToString(key)+"\n"), 0o600); err != nil {
		return err
	}
	stop, err := e2ePostgres(dir)
	if err != nil {
		return err
	}
	defer stop()
	ctx := context.Background()
	c, err := pgx.Connect(ctx, "host="+dir+" user=archive dbname=postgres")
	if err != nil {
		return err
	}
	defer c.Close(ctx)
	_, err = c.Exec(ctx, "CREATE DATABASE archive")
	return err
}

func (in *e2eInst) twin(env ...string) *e2eInst {
	in.t.Helper()
	dir, err := os.MkdirTemp("/tmp", "arc")
	if err != nil {
		in.t.Fatal(err)
	}
	in.t.Cleanup(func() { os.RemoveAll(dir) })
	key, err := os.ReadFile(filepath.Join(in.dir, "data", "secret.key"))
	if err != nil {
		in.t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(in.dir, "data", "blobs"), 0o700)
	os.MkdirAll(filepath.Join(dir, "data"), 0o700)
	if err := os.Symlink(filepath.Join(in.dir, "data", "blobs"), filepath.Join(dir, "data", "blobs")); err != nil {
		in.t.Fatal(err)
	}
	return &e2eInst{t: in.t, dir: dir, env: append(append([]string{"DATABASE_URL=host=" + in.dir + " user=archive dbname=archive",
		"ARCHIVE_SECRET_KEY=" + strings.TrimSpace(string(key))}, e2eCloudTasks.queue(in.t, dir)...), env...)}
}

func (in *e2eInst) on(t *testing.T) *e2eInst {
	c := *in
	c.t = t
	return &c
}

func (in *e2eInst) cmd(args ...string) *exec.Cmd {
	c := exec.Command(e2eBin, args...)
	c.Dir, c.Env = in.dir, in.environ()
	return c
}

func (in *e2eInst) environ() []string {
	return append([]string{"ARCHIVE_E2E_MAIN=1", "PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"), "TMPDIR=" + os.TempDir()}, in.env...)
}

func (in *e2eInst) run(stdin string, args ...string) (string, string, error) {
	c := in.cmd(args...)
	var out, errb bytes.Buffer
	c.Stdin, c.Stdout, c.Stderr = strings.NewReader(stdin), &out, &errb
	err := c.Run()
	return out.String(), errb.String(), err
}

func (in *e2eInst) must(args ...string) string {
	in.t.Helper()
	out, errs, err := in.run("", args...)
	if err != nil {
		in.t.Fatalf("archive %s: %v\n%s%s", strings.Join(args, " "), err, out, errs)
	}
	return out
}

func (in *e2eInst) fails(args ...string) string {
	in.t.Helper()
	out, errs, err := in.run("", args...)
	if err == nil {
		in.t.Fatalf("archive %s succeeded, want an error:\n%s", strings.Join(args, " "), out)
	}
	return errs
}

func (in *e2eInst) writeDir(name string, files map[string]string) string {
	in.t.Helper()
	dir := filepath.Join(in.dir, name)
	os.RemoveAll(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		in.t.Fatal(err)
	}
	for f, body := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte(body), 0o644); err != nil {
			in.t.Fatal(err)
		}
	}
	return dir
}

// push migrates the files, in name order, as one migration.
func (in *e2eInst) push(files map[string]string, args ...string) string {
	in.t.Helper()
	var sql string
	for _, name := range slices.Sorted(maps.Keys(files)) {
		sql += files[name] + "\n"
	}
	return in.migrate(sql, args...)
}

// migrate dry-runs the migration, then runs it with the confirm the dry run printed.
func (in *e2eInst) migrate(sql string, args ...string) string {
	in.t.Helper()
	out, errs, err := in.run(sql, append([]string{"schema", "migrate", "-"}, args...)...)
	if err == nil {
		out, errs, err = in.run(sql, append([]string{"schema", "migrate", "-", "-confirm", decode[SchemaMigration](in.t, out).Confirm}, args...)...)
	}
	if err != nil {
		in.t.Fatalf("archive schema migrate: %v\n%s%s\n%s", err, out, errs, sql)
	}
	return out
}

func (in *e2eInst) migrateFails(sql string, args ...string) string {
	in.t.Helper()
	out, errs, err := in.run(sql, append([]string{"schema", "migrate", "-"}, args...)...)
	if err == nil {
		in.t.Fatalf("archive schema migrate succeeded, want an error:\n%s", out)
	}
	return errs
}

func (in *e2eInst) role(email string) string {
	return in.psql("archive", "SELECT w.id::text || '_u' || translate(rtrim(encode(uuid_send(m.id), 'base64'), '='), '+/', '-_') FROM archive.member m, archive.workspace w WHERE m.email = '"+email+"' AND w.id = (SELECT id FROM archive.workspace ORDER BY created_at LIMIT 1)")
}

func (in *e2eInst) groups(email string, groups ...string) {
	in.t.Helper()
	b, _ := json.Marshal(map[string][]string{"groups": append([]string{}, groups...)})
	in.must("set", "member", id(decode[[]row](in.t, in.must("list", "member", "email="+email))[0]), string(b))
}

func (in *e2eInst) psql(user, sql string) string {
	in.t.Helper()
	out, err := exec.Command("psql", "-X", "-h", in.dir, "-U", user, "-d", "archive", "-Atc", sql).CombinedOutput()
	if err != nil {
		in.t.Fatalf("psql -U %s %q: %v\n%s", user, sql, err, out)
	}
	return strings.TrimSpace(string(out))
}

func decode[T any](t *testing.T, s string) T {
	t.Helper()
	var v T
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		t.Fatalf("decode %q: %v", s, err)
	}
	return v
}

func with(files map[string]string, kv ...string) map[string]string {
	out := map[string]string{}
	for k, v := range files {
		out[k] = v
	}
	for i := 0; i+1 < len(kv); i += 2 {
		if kv[i+1] == "" {
			delete(out, kv[i])
		} else {
			out[kv[i]] = kv[i+1]
		}
	}
	return out
}

type e2eLog struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *e2eLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *e2eLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

type e2eServer struct {
	url  string
	log  *e2eLog
	stop func()
}

func (in *e2eInst) serve() *e2eServer {
	in.t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		in.t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	s := &e2eServer{url: "http://" + addr, log: &e2eLog{}}
	c := in.cmd("serve", "-addr", addr)
	c.Stdout, c.Stderr = s.log, s.log
	if err := c.Start(); err != nil {
		in.t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { c.Wait(); close(done) }()
	queue, api := in.queue(), "http://localhost:"+strings.TrimPrefix(addr, "127.0.0.1:")
	if app, err := url.Parse(in.getenv("ARCHIVE_URL")); err == nil && strings.HasPrefix(app.Host, "app.") {
		api = app.Scheme + "://api." + strings.TrimPrefix(app.Host, "app.")
	}
	var once sync.Once
	s.stop = func() {
		once.Do(func() {
			e2eCloudTasks.route(queue, "", "")
			c.Process.Signal(syscall.SIGTERM)
			select {
			case <-done:
			case <-time.After(30 * time.Second):
				c.Process.Kill()
				<-done
			}
		})
	}
	in.t.Cleanup(s.stop)
	for deadline := time.Now().Add(60 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if resp, err := http.Get(s.url + "/health"); err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusNoContent {
				e2eCloudTasks.route(queue, s.url, strings.TrimRight(api, "/")+"/tasks")
				return s
			}
		}
		select {
		case <-done:
			in.t.Fatalf("serve exited:\n%s", s.log)
		default:
		}
	}
	in.t.Fatalf("serve not ready:\n%s", s.log)
	return nil
}

func call(t *testing.T, method, url, bearer string, body any, out any, cookies ...*http.Cookie) (int, http.Header) {
	t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, url, rd)
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	req.Header.Set("X-Forwarded-For", "203.0.113.7")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := (&http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if out != nil && len(b) > 0 {
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.UseNumber()
		if err := dec.Decode(out); err != nil {
			t.Fatalf("%s %s: %d %s", method, url, resp.StatusCode, b)
		}
	}
	return resp.StatusCode, resp.Header
}

func wantStatus(t *testing.T, got, want int, what string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: status %d, want %d", what, got, want)
	}
}

type row = map[string]any

func names(rs []row) string {
	var out []string
	for _, r := range rs {
		out = append(out, fmt.Sprint(r["name"]))
	}
	return strings.Join(out, ",")
}

func id(r row) string { return fmt.Sprint(r["id"]) }

func TestSchemaMigrationsDryRunFirstAndNormalizeMessyInput(t *testing.T) {
	t.Parallel()
	in := newInst(t)
	in.fails("list", "contact")
	in.must("user", "add", "ann@example.com", "-name", "Ann")
	m := decode[SchemaMigration](t, in.push(e2eSchema))
	if m.Revision == nil || !slices.ContainsFunc(m.Statements, func(s string) bool { return strings.HasPrefix(s, "CREATE TABLE invoice") }) {
		t.Fatalf("schema migrate: %+v", m)
	}
	out, _, _ := in.run(`const all = await archive.schema();
const invoice = await archive.schema("invoice");
const missing = await archive.schema("nope").then(() => "", (e) => e.code + ": " + e.message);
return { names: all.map((t) => t.name), same: JSON.stringify(all[2]) === JSON.stringify(invoice), missing,
  ddl: invoice.ddl.startsWith('CREATE TYPE "public"."invoice_status" AS ENUM'), invoice: { ...invoice, ddl: undefined } };`, "run", "-")
	var b bytes.Buffer
	json.Compact(&b, decode[RunOutput](t, out).Result)
	if b.String() != `{"names":["company","contact","invoice","person"],"same":true,"missing":"not_found: no table or view \"nope\"; there are company, contact, invoice, person: not found","ddl":true,`+
		`"invoice":{"name":"invoice","kind":"table","description":"","columns":[{"name":"id","type":"uuid","description":"","required":false},{"name":"access","type":"text","description":"","required":false},`+
		`{"name":"created_at","type":"timestamp with time zone","description":"","required":false},{"name":"updated_at","type":"timestamp with time zone","description":"","required":false},`+
		`{"name":"number","type":"text","description":"","required":true},{"name":"total","type":"archive.money","description":"","required":false},{"name":"date","type":"date","description":"","required":false},`+
		`{"name":"ein","type":"us_ein","description":"","required":false},{"name":"status","type":"invoice_status","description":"","required":false,"values":["draft","sent","paid"]},`+
		`{"name":"document","type":"uuid","description":"","required":false,"references":"archive.file"}]}}` {
		t.Errorf("schema() with and without a name, as tables() was, with the ddl: %s", b.String())
	}
	ddl := in.must("schema", "show")
	for _, want := range []string{"-- invoice\n", `CREATE TYPE "public"."invoice_status"`, `CREATE DOMAIN "public"."zip"`, "REFERENCES company(id)"} {
		if !strings.Contains(ddl, want) {
			t.Errorf("schema show lacks %q:\n%s", want, ddl)
		}
	}
	in.migrate(`CREATE INDEX person_name ON person (lower(name)) WHERE name <> '';
CREATE FUNCTION touch() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN NEW.updated_at = now(); RETURN NEW; END $$;
CREATE TRIGGER person_touch BEFORE UPDATE ON person FOR EACH ROW EXECUTE FUNCTION touch();
CREATE TYPE mood AS ENUM ('good', 'bad');
CREATE TYPE pair AS (a text, b integer);
CREATE SEQUENCE ticket START 100;
CREATE VIEW people WITH (security_invoker = true) AS SELECT * FROM person WHERE name <> '';
COMMENT ON VIEW people IS 'Everyone with a name';
COMMENT ON TABLE person IS 'Someone''s record';
COMMENT ON COLUMN company.name IS 'Legal name';`)
	ddl = in.must("schema", "show")
	for _, want := range []string{
		"-- person\nCREATE OR REPLACE FUNCTION public.touch()\n RETURNS trigger\n LANGUAGE plpgsql\nAS $function$ BEGIN NEW.updated_at = now(); RETURN NEW; END $function$;\nCREATE TABLE \"public\".\"person\" (\n\t\"id\" uuid DEFAULT uuidv7() NOT NULL,\n",
		"\nCOMMENT ON TABLE \"public\".\"person\" IS 'Someone''s record';\n",
		"\nALTER TABLE \"public\".\"person\" ENABLE ROW LEVEL SECURITY;\nALTER TABLE \"public\".\"person\" FORCE ROW LEVEL SECURITY;\n",
		"\nALTER TABLE \"public\".\"person\" ADD CONSTRAINT \"person_pkey\" PRIMARY KEY (id);\nALTER TABLE \"public\".\"person\" ADD CONSTRAINT \"person_employer_fkey\" FOREIGN KEY (employer) REFERENCES company(id);\n",
		"\nCREATE INDEX person_name ON public.person USING btree (lower(name)) WHERE (name <> ''::text);\n",
		"\nCREATE POLICY \"access\" ON \"public\".\"person\" USING (archive.can(access)) WITH CHECK (archive.can(access));\n",
		"\nCREATE TRIGGER person_touch BEFORE UPDATE ON public.person FOR EACH ROW EXECUTE FUNCTION touch();\n",
		"COMMENT ON COLUMN \"public\".\"company\".\"name\" IS 'Legal name';\n",
		"-- mood\nCREATE TYPE \"public\".\"mood\" AS ENUM ('good', 'bad');\n",
		"-- pair\nCREATE TYPE \"public\".\"pair\" AS (\n\t\"a\" text,\n\t\"b\" integer\n);\n",
		"-- people\nCREATE VIEW \"public\".\"people\" WITH (security_invoker=true) AS\n SELECT id,\n",
		"   FROM person\n  WHERE name <> ''::text;\nCOMMENT ON VIEW \"public\".\"people\" IS 'Everyone with a name';\n",
		"-- ticket\nCREATE SEQUENCE \"public\".\"ticket\" AS bigint START WITH 100 INCREMENT BY 1 MINVALUE 1 MAXVALUE 9223372036854775807 CACHE 1;\n",
	} {
		if !strings.Contains(ddl, want) {
			t.Errorf("schema show lacks %q:\n%s", want, ddl)
		}
	}
	if strings.Contains(ddl, "OWNER TO") || strings.Contains(ddl, "GRANT") {
		t.Errorf("schema show has owners or grants:\n%s", ddl)
	}

	r := decode[row](t, in.must("create", "contact", `{"name":" Ann ","email":"Ann@EXAMPLE.com","phone":"(415) 555-0123","birthday":"4/3/1990","balance":"$1,234.50","tags":"vip","zip":"94105"}`))
	want := map[string]string{"phone": "+14155550123", "birthday": "1990-04-03", "email": "Ann@example.com", "zip": "94105",
		"balance": "map[amount:1234.50 currency:USD]", "tags": "[vip]"}
	for k, v := range want {
		if fmt.Sprint(r[k]) != v {
			t.Errorf("%s = %v, want %v", k, r[k], v)
		}
	}
	if _, ok := r["access"]; !ok || r["access"] != nil || r["created_at"] == nil || r["search"] != nil {
		t.Errorf("row columns: %v", r)
	}

	errs := in.fails("create", "contact", `{"name":"Bad","email":"not an email","phone":"12","nope":1}`)
	if !strings.Contains(errs, "invalid contact") || !strings.Contains(errs, "email:") || !strings.Contains(errs, "phone:") || !strings.Contains(errs, "nope: unknown field") {
		t.Errorf("validation errors: %s", errs)
	}
	if errs := in.fails("create", "contact", `{"email":"a@b.com"}`); !strings.Contains(errs, "name: required") {
		t.Errorf("required: %s", errs)
	}
	if errs := in.fails("create", "contact", `{"name":"Pat","zip":"9410"}`); !strings.Contains(errs, `zip: value for domain zip violates check constraint "zip_check"`) {
		t.Errorf("domain check from Postgres: %s", errs)
	}
	if errs := in.fails("create", "invoice", `{"number":"X","status":"lost"}`); !strings.Contains(errs, "status: must be one of draft, sent, paid") {
		t.Errorf("enum: %s", errs)
	}
	if got := decode[row](t, in.must("create", "invoice", `{"number":"X","status":"PAID"}`)); got["status"] != "paid" {
		t.Errorf("enum case: %v", got)
	}

	if got := decode[[]row](t, in.must("list", "contact", "phone=415.555.0123")); len(got) != 1 || id(got[0]) != id(r) {
		t.Errorf("filter on messy phone: %v", got)
	}
	if got := decode[[]row](t, in.must("list", "contact", "phone=4155550199")); len(got) != 0 {
		t.Errorf("filter on other phone: %v", got)
	}
	in.must("set", "contact", id(r), `{"phone":null,"birthday":"1990-04-04"}`)
	if got := decode[row](t, in.must("get", "contact", id(r))); got["phone"] != nil || got["birthday"] != "1990-04-04" || got["email"] != "Ann@example.com" {
		t.Errorf("after set: %v", got)
	}
	in.must("update", "contact", id(r), `{"name":"Ann B"}`)
	if got := decode[row](t, in.must("get", "contact", id(r))); got["name"] != "Ann B" || got["email"] != nil {
		t.Errorf("after update: %v", got)
	}

	out, errs, err := in.run("ALTER TABLE contact DROP COLUMN tags", "schema", "migrate", "-")
	dry := decode[SchemaMigration](t, out)
	if err != nil || dry.Applied || dry.Revision != nil || len(dry.Confirm) != 64 || fmt.Sprint(dry.Drops) != "[{column contact.tags drop  text[] }]" {
		t.Fatalf("dry run of a drop: %v %s %+v", err, errs, dry)
	}
	if ddl := in.must("schema", "show"); !strings.Contains(ddl, "tags") {
		t.Errorf("a dry run changed the schema")
	}
	if errs := in.migrateFails("ALTER TABLE contact DROP COLUMN tags", "-confirm", strings.Repeat("0", 64)); !strings.Contains(errs, "dry-run them again") {
		t.Errorf("a wrong confirm: %s", errs)
	}
	out = in.must("schema", "migrate", "-confirm", dry.Confirm, filepath.Join(in.writeDir("drop", map[string]string{"drop.sql": "ALTER TABLE contact DROP COLUMN tags"}), "drop.sql"))
	if m := decode[[]SchemaMigration](t, out); len(m) != 1 || !m[0].Applied || m[0].Revision == nil || fmt.Sprint(m[0].Statements) != "[ALTER TABLE contact DROP COLUMN tags]" {
		t.Errorf("a named file with the confirm of the same sql dry-run unnamed: %s", out)
	}
	if got := decode[row](t, in.must("get", "contact", id(r))); got["tags"] != nil || got["name"] != "Ann B" {
		t.Errorf("after the drop: %v", got)
	}

	if errs := in.migrateFails("CREATE TABLE loose (id uuid PRIMARY KEY, name text)"); !strings.Contains(errs, "loose.id: want id uuid PRIMARY KEY DEFAULT uuidv7()") ||
		strings.Contains(errs, "loose.access") || strings.Contains(errs, "ROW LEVEL SECURITY") || strings.Contains(errs, "loose.search") {
		t.Errorf("a table with its own id and nothing else gets the rest but not a default on its id: %s", errs)
	}
	if errs := in.migrateFails("CREATE TABLE fine (name text);\nCREATE TABLE oops ("); !strings.Contains(errs, "statement 2 (CREATE TABLE oops (): ERROR: syntax error") {
		t.Errorf("broken DDL: %s", errs)
	}
	for _, bad := range []struct{ sql, want string }{
		{"UPDATE contact SET name = 'x'", "sql[0]: want CREATE, ALTER or DROP"},
		{"DO $$ BEGIN END $$", "sql[0]: want CREATE, ALTER or DROP"},
		{"CREATE ROLE sneaky", "sql[0]: want CREATE, ALTER or DROP"},
		{"CREATE TABLE copy AS SELECT * FROM contact", "CREATE TABLE ... AS are not allowed"},
		{"CREATE FUNCTION f() RETURNS int LANGUAGE sql SECURITY DEFINER AS 'SELECT 1'", "SECURITY DEFINER"},
		{"CREATE TABLE archive.sneaky (x text)", "not Archive's"},
		{"CREATE OR REPLACE FUNCTION archive.can(g text) RETURNS boolean LANGUAGE sql AS 'SELECT true'", "not Archive's"},
	} {
		if errs := in.migrateFails(bad.sql); !strings.Contains(errs, bad.want) {
			t.Errorf("%s: %s", bad.sql, errs)
		}
	}
	if n := in.psql("archive", "SELECT count(*) FROM pg_tables WHERE tablename IN ('loose', 'oops', 'fine', 'copy', 'sneaky')"); n != "0" {
		t.Errorf("%s rejected tables created", n)
	}

	dir := in.writeDir("migrations", map[string]string{"001_note.sql": e2eNote, "002_tag.sql": "ALTER TABLE note ADD COLUMN tag text"})
	plan := decode[[]SchemaMigration](t, in.must("schema", "migrate", dir))
	if len(plan) != 2 || plan[0].Applied || plan[1].Applied || plan[0].Confirm != plan[1].Confirm || len(plan[0].Confirm) != 64 ||
		fmt.Sprint(plan[1].Changes) != "[{column note.tag create   text}]" || in.psql("archive", "SELECT count(*) FROM pg_tables WHERE tablename = 'note'") != "0" {
		t.Fatalf("a dry run of a directory, each migration on the ones before: %+v", plan)
	}
	first := decode[[]SchemaMigration](t, in.must("schema", "migrate", dir, "-confirm", plan[0].Confirm))
	again := decode[[]SchemaMigration](t, in.must("schema", "migrate", dir))
	if len(first) != 2 || !first[0].Applied || !first[1].Applied || len(again) != 2 || !again[0].Skipped || again[1].Name != "002_tag.sql" {
		t.Errorf("named migrations run once: %+v %+v", first, again)
	}
	revs := decode[[]SchemaRevision](t, in.must("schema", "revisions"))
	if len(revs) != 5 || revs[0].Name != "002_tag.sql" || revs[4].Author != "ann@example.com" {
		t.Errorf("revisions: %+v", revs)
	}
}

// e2eMigrate is a migrate for runs that dry-runs the sql, then runs it with the dry run's confirm.
const e2eMigrate = "const migrate = async (sql) => archive.migrate(sql, { confirm: (await archive.migrate(sql)).confirm });\n"

func TestAgentsPatchTheSchemaInsideTheirRun(t *testing.T) {
	t.Parallel()
	in := newInst(t)
	in.must("user", "add", "ann@example.com", "-name", "Ann")
	run := func(t *testing.T, code string) map[string]any {
		t.Helper()
		stdout, _, _ := in.run(e2eMigrate+code, "run", "-")
		out := decode[RunOutput](t, stdout)
		var got map[string]any
		if out.Error != "" || json.Unmarshal(out.Result, &got) != nil {
			t.Fatalf("run: %+v", out)
		}
		return got
	}
	fails := func(t *testing.T, code string) string {
		t.Helper()
		stdout, _, _ := in.run(e2eMigrate+code, "run", "-")
		return decode[RunOutput](t, stdout).Error
	}

	got := run(t, `const m = await migrate("CREATE TABLE house (address text NOT NULL, city text);\n" +
  "COMMENT ON TABLE house IS 'A house';\n" +
  "CREATE VIEW open_house WITH (security_invoker = true) AS SELECT * FROM house;\n" +
  "COMMENT ON VIEW open_house IS 'Every house';");
const made = await archive.create("house", { address: "Canal 1", city: "Amsterdam" });
return { ran: m.statements.join("\n"), made, view: await archive.list("open_house"),
  hit: (await archive.search("amsterdam")).map((h) => h.type), ddl: (await archive.schema("house")).ddl };`)
	if ran := fmt.Sprint(got["ran"]); !strings.Contains(ran, "ADD COLUMN id uuid NOT NULL DEFAULT uuidv7()") || !strings.Contains(ran, "CREATE POLICY access ON") {
		t.Errorf("a migration with only the table's own columns runs what every table has too: %s", ran)
	}
	made := got["made"].(map[string]any)
	if made["id"] == nil || made["created_at"] == nil || fmt.Sprint(got["hit"]) != "[house]" || !strings.Contains(fmt.Sprint(got["view"]), fmt.Sprint(made["id"])) {
		t.Errorf("a table and a view written without the boilerplate: %v", got)
	}
	if ddl := fmt.Sprint(got["ddl"]); !strings.Contains(ddl, `"id" uuid DEFAULT uuidv7() NOT NULL`) || !strings.Contains(ddl, `COMMENT ON TABLE "public"."house" IS 'A house'`) {
		t.Errorf("schema() shows the whole table: %s", ddl)
	}

	got = run(t, `await migrate("ALTER TABLE house ADD COLUMN street text");
return { set: await archive.update("house", (await archive.list("house"))[0].id, { street: "Damrak" }), after: (await archive.search("damrak")).length,
  view: (await archive.schema()).some((t) => t.name === "open_house") };`)
	if got["after"] != 1.0 || got["view"] != true {
		t.Errorf("adding a text column rederives search: %v", got)
	}

	got = run(t, `await migrate("ALTER TABLE house ADD COLUMN rooms integer");
for (const h of await archive.list("house")) await archive.update("house", h.id, { rooms: 3 });
await migrate("ALTER TABLE house ALTER COLUMN rooms SET NOT NULL");
return { rooms: (await archive.list("house"))[0].rooms, required: (await archive.schema()).find((t) => t.name === "house").columns.find((c) => c.name === "rooms").required };`)
	if got["rooms"] != 3.0 || got["required"] != true {
		t.Errorf("add, backfill, then require a column in one run: %v", got)
	}
	if e := fails(t, `await archive.migrate("ALTER TABLE house ADD COLUMN floors integer NOT NULL"); return {};`); !strings.Contains(e, "floors") {
		t.Errorf("a dry run of a required column without a default on a table with rows: %q", e)
	}
	if e := fails(t, `await archive.migrate("UPDATE house SET rooms = 4"); return {};`); !strings.Contains(e, "change rows with archive.create, archive.update and archive.delete") {
		t.Errorf("rows through migrate: %q", e)
	}

	got = run(t, `const m = await migrate("ALTER TABLE house RENAME COLUMN rooms TO bedrooms;\n" +
  "ALTER TABLE house ALTER COLUMN bedrooms TYPE text USING bedrooms || ' rooms';");
return { m, bedrooms: (await archive.list("house"))[0].bedrooms, hit: (await archive.search("rooms")).length };`)
	if got["bedrooms"] != "3 rooms" || got["hit"] != 1.0 || !strings.Contains(fmt.Sprint(got["m"]), `SET EXPRESSION AS (archive.search_doc(VARIADIC ARRAY["address"::text, "bedrooms"::text, "city"::text, "street"::text]))`) ||
		!strings.Contains(fmt.Sprint(got["m"]), "drops:[map[action:alter after:text NOT NULL before:integer NOT NULL from:house.rooms kind:column name:house.bedrooms]]") {
		t.Errorf("a rename and a type change with USING keep the data, rederive search and report the rewrite: %v", got)
	}

	run(t, `await migrate("ALTER TABLE house DROP COLUMN street"); return {};`)
	got = run(t, `const revs = await archive.revisions();
return { n: revs.length, by: revs[revs.length - 1].author, last: revs[0].statements, hit: (await archive.search("Damrak")).length };`)
	if got["n"] != 6.0 || got["by"] != "ann@example.com" || got["hit"] != 0.0 || !strings.Contains(fmt.Sprint(got["last"]), "DROP COLUMN street") {
		t.Errorf("a dropped text column leaves search, and revisions: %v", got)
	}

	in.must("user", "add", "bob@example.com", "-name", "Bob", "-role", "viewer")
	viewer := func(code string) RunOutput {
		stdout, _, _ := in.run(code, "-as", "bob@example.com", "run", "-")
		return decode[RunOutput](t, stdout)
	}
	if out := viewer(`await archive.migrate("CREATE TABLE barn (name text)")`); !strings.Contains(out.Error, "only owners change the schema") {
		t.Errorf("a viewer migrates: %+v", out)
	}
	if out := viewer(`await archive.query("SELECT archive.ddl('guess', 'DROP TABLE house')")`); !strings.Contains(out.Error, "only archive.migrate changes the schema") {
		t.Errorf("calling archive.ddl without its key: %+v", out)
	}
	if e := fails(t, `await migrate("CREATE TYPE kind AS ENUM ('flat')");
await migrate("ALTER TYPE kind ADD VALUE 'villa'");
await archive.query("SELECT 'villa'::kind"); return {};`); !strings.Contains(e, "migrate in one run, use the value in the next") {
		t.Errorf("a new enum value in the run that adds it: %q", e)
	}
}

func TestMigrateDryRunsAndQueryOnlyReads(t *testing.T) {
	t.Parallel()
	in := newInst(t)
	in.must("user", "add", "ann@example.com", "-name", "Ann")
	run := func(t *testing.T, code string) RunOutput {
		t.Helper()
		stdout, _, _ := in.run(e2eMigrate+code, "run", "-")
		out := decode[RunOutput](t, stdout)
		var b bytes.Buffer
		json.Compact(&b, out.Result)
		out.Result = b.Bytes()
		return out
	}
	in.migrate("CREATE TABLE shed (label text NOT NULL, size integer);\nCREATE FUNCTION shed_bump() RETURNS void LANGUAGE sql AS $$ UPDATE shed SET size = size + 1 $$;")
	in.must("create", "shed", `{"label":"Blue","size":2}`)
	ddl := in.must("schema", "show")

	for _, c := range []struct{ sql, changes, drops string }{
		{"ALTER TABLE shed ADD COLUMN door text", "[create column shed.door alter column shed.search]", "[]"},
		{"ALTER TABLE shed DROP COLUMN size", "[drop column shed.size]", "[drop column shed.size]"},
		{"ALTER TABLE shed ALTER COLUMN size TYPE text", "[alter column shed.search alter column shed.size]", "[alter column shed.size]"},
		{"ALTER TABLE shed RENAME COLUMN size TO area", "[alter column shed.area]", "[]"},
		{"DROP TABLE shed CASCADE", "[drop table shed]", "[drop table shed]"},
		{"CREATE VIEW sheds WITH (security_invoker = true) AS SELECT * FROM shed", "[create view sheds]", "[]"},
		{"CREATE FUNCTION shed_touch() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$;\n" +
			"CREATE TRIGGER shed_touch BEFORE UPDATE ON shed FOR EACH ROW EXECUTE FUNCTION shed_touch()", "[create trigger shed.shed_touch create function shed_touch()]", "[]"},
		{"COMMENT ON POLICY access ON shed IS 'Rows by group'", "[alter policy shed.access]", "[]"},
		{"CREATE TYPE mood AS ENUM ('good', 'bad');\nALTER TYPE mood ADD VALUE 'meh'", "[create type mood]", "[]"},
	} {
		out := run(t, `const m = await archive.migrate(`+strconv.Quote(c.sql)+`);
const say = (cs) => cs.map((c) => c.action + " " + c.kind + " " + c.name);
return { applied: m.applied, changes: say(m.changes), drops: say(m.drops), confirm: m.confirm.length };`)
		var got struct {
			Applied        bool
			Changes, Drops []string
			Confirm        int
		}
		if json.Unmarshal(out.Result, &got); out.Error != "" || got.Applied || got.Confirm != 64 || fmt.Sprint(got.Changes) != c.changes || fmt.Sprint(got.Drops) != c.drops {
			t.Errorf("dry run of %s: %+v %s", c.sql, got, out.Error)
		}
	}
	if after := in.must("schema", "show"); after != ddl {
		t.Errorf("dry runs changed the schema:\n%s\nwas\n%s", after, ddl)
	}

	out := run(t, `const sql = "ALTER TABLE shed ADD COLUMN roof text";
const plan = await archive.migrate(sql);
const other = await archive.migrate("ALTER TABLE shed ADD COLUMN roof  text", { confirm: plan.confirm }).then(() => "", (e) => e.code);
await migrate("ALTER TABLE shed ADD COLUMN wall text");
const stale = await archive.migrate(sql, { confirm: plan.confirm }).then(() => "", (e) => e.code + ": " + e.message);
const fresh = await archive.migrate(sql);
const done = await archive.migrate(sql, { confirm: fresh.confirm });
return { other, stale, same: plan.confirm === fresh.confirm, applied: done.applied, revision: !!done.revision, changes: done.changes.length };`)
	if string(out.Result) != `{"other":"conflict","stale":"conflict: the schema or this migration changed since its dry run; dry-run it again for a new confirm: conflict","same":false,"applied":true,"revision":true,"changes":2}` {
		t.Errorf("a confirm holds only for its sql and schema: %s %s", out.Result, out.Error)
	}
	plan := run(t, `return await archive.migrate("ALTER TABLE shed ADD COLUMN floor text")`)
	var p SchemaMigration
	json.Unmarshal(plan.Result, &p)
	if out := run(t, `return (await archive.migrate("ALTER TABLE shed ADD COLUMN floor text", { confirm: "`+p.Confirm+`" })).applied`); string(out.Result) != "true" {
		t.Errorf("a confirm from an earlier run: %+v", out)
	}

	for _, sql := range []string{"INSERT INTO shed (label) VALUES ('x') RETURNING id", "UPDATE shed SET size = 3", "DELETE FROM shed",
		"WITH d AS (DELETE FROM shed RETURNING id) SELECT count(*) FROM d", "SELECT shed_bump()", "CREATE TABLE barn (name text)",
		"ALTER TABLE shed ADD COLUMN garden boolean"} {
		out := run(t, `const e = await archive.query(`+strconv.Quote(sql)+`).then(() => "", (e) => e.code + ": " + e.message);
const made = await archive.create("shed", { label: "After" });
await archive.delete("shed", made.id);
return { e, rows: (await archive.query("SELECT label, size FROM shed")) };`)
		if !strings.Contains(string(out.Result), `"e":"forbidden: cannot execute `) || !strings.Contains(string(out.Result), "in a read-only transaction: archive.query only reads") ||
			!strings.Contains(string(out.Result), `"rows":[{"label":"Blue","size":2}]`) {
			t.Errorf("archive.query(%q) writes, or leaves the run read only: %s %s", sql, out.Result, out.Error)
		}
	}
	if out := run(t, `return await archive.query("SELECT label FROM shed WHERE size > $1", 1)`); string(out.Result) != `[{"label":"Blue"}]` {
		t.Errorf("a query with params: %+v", out)
	}
}

func TestDemoVaultLoadsOnceWithLinkedRowsAndFiles(t *testing.T) {
	t.Parallel()
	in := newInst(t)
	load := func() (string, string, error) {
		load, _ := filepath.Abs("../demo/load.sh")
		c := exec.Command(load, "ann@example.com")
		c.Dir, c.Env = in.dir, append(in.environ(), "ARCHIVE_BIN="+e2eBin)
		var out, errb bytes.Buffer
		c.Stdout, c.Stderr = &out, &errb
		err := c.Run()
		return out.String(), errb.String(), err
	}
	out, errs, err := load()
	if err != nil {
		t.Fatalf("load: %v\n%s%s", err, out, errs)
	}
	counts := decode[struct{ Result map[string]int }](t, out).Result
	if counts["person"] < 10 || counts["payment"] < 100 || counts["measurement"] < 100 || len(counts) != 18 {
		t.Fatalf("demo counts: %v", counts)
	}
	members := decode[[]struct {
		Email, Role string
		Groups      []string
	}](t, in.must("list", "member"))
	if len(members) != 2 || members[0].Email != "ann@example.com" || members[0].Role != "owner" || !slices.Equal(members[0].Groups, []string{"family"}) ||
		members[1].Email != "sam@example.com" || members[1].Role != "editor" || !slices.Equal(members[1].Groups, []string{"family"}) {
		t.Errorf("demo members: %+v", members)
	}
	if n := in.psql("archive", "SELECT count(*) FROM archive.file WHERE access = 'family'"); n != "11" {
		t.Errorf("%s family files, want 11", n)
	}
	res, _, err := in.run(`return { files: (await archive.files({ limit: 100 })).length,
  receipts: (await archive.query("SELECT count(*)::int AS n FROM payment WHERE receipt IS NOT NULL"))[0].n };`, "run", "-")
	if err != nil {
		t.Fatalf("run: %v %s", err, res)
	}
	got := decode[struct{ Result map[string]any }](t, res).Result
	if fmt.Sprint(got["files"]) != "35" || fmt.Sprint(got["receipts"]) == "0" {
		t.Errorf("files and links: %v", got)
	}
	if ddl := in.must("schema", "show"); strings.Count(ddl, "\n-- ") != 24 || !strings.Contains(ddl, "by: status") || !strings.Contains(ddl, `"number" archive.card`) || !strings.Contains(ddl, "REFERENCES person(id)") {
		t.Errorf("demo schema: %s", ddl)
	}
	if _, errs, err := load(); err == nil || !strings.Contains(errs, "already has people") {
		t.Errorf("second load: %v %s", err, errs)
	}
}

func TestUsersGroupsTokensAndSecretsAcrossEverySurface(t *testing.T) {
	t.Parallel()
	in := newInst(t, e2eGoogle(t)...)
	in.must("user", "add", "alice@example.com")
	srv := in.serve()
	api := srv.url + "/api"

	added := decode[User](t, in.must("user", "add", "bob@example.com", "-name", "Bob"))
	in.must("user", "add", "carl@example.com", "-role", "editor")
	in.push(e2eSchema)
	carlID := id(decode[[]row](t, in.must("list", "member", "email=carl@example.com"))[0])
	in.must("set", "member", carlID, `{"groups":["hr"]}`)
	aliceTok := decode[Token](t, in.must("token", "create", "-name", "Claude")).Secret
	aliceView := decode[Token](t, in.must("token", "create", "-name", "Reader", "-viewer")).Secret
	bobTok := decode[Token](t, in.must("-as", "bob@example.com", "token", "create")).Secret
	carlTok := decode[Token](t, in.must("-as", "carl@example.com", "token", "create")).Secret
	carlView := decode[Token](t, in.must("-as", "carl@example.com", "token", "create", "-viewer")).Secret

	var all, acme, hr row
	st, _ := call(t, "POST", api+"/person", aliceTok, map[string]any{"name": "Everyone"}, &all)
	wantStatus(t, st, 201, "alice creates a row for everyone")
	call(t, "POST", api+"/company", aliceTok, map[string]any{"name": "Acme", "access": "hr"}, &acme)
	st, _ = call(t, "POST", api+"/person", aliceTok, map[string]any{"name": "HR only", "access": "hr", "ssn": "123-45-6789", "employer": id(acme), "friends": []string{id(all)}}, &hr)
	wantStatus(t, st, 201, "alice creates an hr row")

	t.Run("UnauthenticatedRequestsGet401", func(t *testing.T) {
		st, h := call(t, "GET", api+"/person", "", nil, nil)
		wantStatus(t, st, 401, "api without token")
		if !strings.HasPrefix(h.Get("WWW-Authenticate"), "Bearer") {
			t.Errorf("WWW-Authenticate = %q", h.Get("WWW-Authenticate"))
		}
		st, _ = call(t, "GET", api+"/person", "arc_nope", nil, nil)
		wantStatus(t, st, 401, "api with a bad token")
		st, _ = call(t, "POST", srv.url+"/mcp", "", map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize"}, nil)
		wantStatus(t, st, 401, "mcp without token")
		st, _ = call(t, "GET", srv.url+"/auth/me", "", nil, nil)
		wantStatus(t, st, 401, "me without session")
	})

	t.Run("RowsFollowTheirAccessThroughAPI", func(t *testing.T) {
		var a, b, c []row
		call(t, "GET", api+"/person", aliceTok, nil, &a)
		call(t, "GET", api+"/person", bobTok, nil, &b)
		call(t, "GET", api+"/person", carlTok, nil, &c)
		if names(a) != "Everyone,HR only" || names(b) != "Everyone" || names(c) != "Everyone,HR only" {
			t.Fatalf("alice sees %q, bob %q, carl %q", names(a), names(b), names(c))
		}
		st, _ := call(t, "GET", api+"/person/"+id(hr), bobTok, nil, nil)
		wantStatus(t, st, 404, "bob reads the hr row")
		st, _ = call(t, "DELETE", api+"/person/"+id(hr), bobTok, nil, nil)
		wantStatus(t, st, 404, "bob deletes the hr row")
		var only []row
		call(t, "GET", api+"/person?access=hr", aliceTok, nil, &only)
		if names(only) != "HR only" {
			t.Errorf("filter on access: %q", names(only))
		}
		st, _ = call(t, "GET", api+"/nothing", aliceTok, nil, nil)
		wantStatus(t, st, 404, "unknown table")
	})

	t.Run("RowsFollowTheirAccessThroughCLIWhileServeRuns", func(t *testing.T) {
		in := in.on(t)
		b := names(decode[[]row](t, in.must("-as", "bob@example.com", "list", "person")))
		a := names(decode[[]row](t, in.must("list", "person")))
		if a != "Everyone,HR only" || b != "Everyone" {
			t.Fatalf("cli alice %q, bob %q", a, b)
		}
	})

	t.Run("PlainPsqlAsTheUserRoleSeesWhatTheAppSees", func(t *testing.T) {
		in := in.on(t)
		bob, carl := in.role("bob@example.com"), in.role("carl@example.com")
		in.psql("archive", "ALTER ROLE \""+bob+"\" LOGIN; ALTER ROLE \""+carl+"\" LOGIN")
		defer in.psql("archive", "ALTER ROLE \""+bob+"\" NOLOGIN; ALTER ROLE \""+carl+"\" NOLOGIN")
		if got := in.psql(bob, "SELECT string_agg(name, ',' ORDER BY id) FROM person"); got != "Everyone" {
			t.Fatalf("psql as bob: %q", got)
		}
		if got := in.psql(carl, "SELECT string_agg(name, ',' ORDER BY id) FROM person"); got != "Everyone,HR only" {
			t.Fatalf("psql as carl: %q", got)
		}
		if got := in.psql(bob, "SET archive.space = 'hr'; SET archive.members = '1'; SELECT count(*) FROM person"); !strings.HasSuffix(got, "\n1") {
			t.Errorf("a session setting widens what bob sees: %q", got)
		}
		if n := in.psql(carl, "SELECT count(*) FROM archive.secret"); n != "0" {
			t.Errorf("psql as carl sees %s secrets", n)
		}
		out, err := exec.Command("psql", "-X", "-h", in.dir, "-U", bob, "-d", "archive", "-Atc", "INSERT INTO person (name) VALUES ('x')").CombinedOutput()
		if err == nil || !strings.Contains(string(out), "permission denied") {
			t.Errorf("psql insert as viewer: %v %s", err, out)
		}
		out, err = exec.Command("psql", "-X", "-h", in.dir, "-U", carl, "-d", "archive", "-Atc", "INSERT INTO person (name, access) VALUES ('x', 'finance')").CombinedOutput()
		if err == nil || !strings.Contains(string(out), "row-level security") {
			t.Errorf("psql insert into a group carl is not in: %v %s", err, out)
		}
		out, err = exec.Command("psql", "-X", "-h", in.dir, "-U", carl, "-d", "archive", "-Atc", "INSERT INTO contact (name, email) VALUES ('x', 'not-an-email')").CombinedOutput()
		if err == nil || !strings.Contains(string(out), "email_check") {
			t.Errorf("psql insert of a bad email: %v %s", err, out)
		}
		out, err = exec.Command("psql", "-X", "-h", in.dir, "-U", carl, "-d", "archive", "-Atc", "UPDATE archive.member SET role = 'owner'").CombinedOutput()
		if err == nil || !strings.Contains(string(out), "permission denied") {
			t.Errorf("psql editor promotes themselves: %v %s", err, out)
		}
	})

	t.Run("ViewerCannotWriteAndEditorsWriteOnlyTheirGroups", func(t *testing.T) {
		in := in.on(t)
		st, _ := call(t, "POST", api+"/person", bobTok, map[string]any{"name": "Intruder"}, nil)
		wantStatus(t, st, 403, "viewer creates")
		st, _ = call(t, "PATCH", api+"/person/"+id(all), bobTok, map[string]any{"name": "Changed"}, nil)
		wantStatus(t, st, 403, "viewer patches")
		st, _ = call(t, "DELETE", api+"/person/"+id(all), bobTok, nil, nil)
		wantStatus(t, st, 403, "viewer deletes")
		in.fails("-as", "bob@example.com", "create", "person", `{"name":"x"}`)
		st, _ = call(t, "POST", api+"/person", carlTok, map[string]any{"name": "Finance", "access": "finance"}, nil)
		wantStatus(t, st, 403, "editor creates in a group they are not in")
		var mine row
		st, _ = call(t, "POST", api+"/person", carlTok, map[string]any{"name": "Carl's", "access": "hr"}, &mine)
		wantStatus(t, st, 201, "editor creates in their group")
		st, _ = call(t, "PATCH", api+"/person/"+id(mine), carlTok, map[string]any{"access": "finance"}, nil)
		wantStatus(t, st, 403, "editor moves a row out of their groups")
		st, _ = call(t, "DELETE", api+"/person/"+id(mine), carlTok, nil, nil)
		wantStatus(t, st, 204, "editor deletes in their group")
	})

	t.Run("ViewerOnlyTokenReadsAsItsOwnerAndFollowsTheirGroups", func(t *testing.T) {
		in := in.on(t)
		var rs []row
		call(t, "GET", api+"/person", carlView, nil, &rs)
		if names(rs) != "Everyone,HR only" {
			t.Fatalf("carl's viewer token sees %q", names(rs))
		}
		st, _ := call(t, "POST", api+"/person", carlView, map[string]any{"name": "x"}, nil)
		wantStatus(t, st, 403, "viewer token creates")
		var me struct {
			Role   string   `json:"role"`
			Groups []string `json:"groups"`
		}
		call(t, "GET", srv.url+"/auth/me", carlView, nil, &me)
		if me.Role != "viewer" || !slices.Equal(me.Groups, []string{"hr"}) {
			t.Errorf("viewer token me: %+v", me)
		}
		st, _ = call(t, "POST", srv.url+"/auth/tokens", carlView, map[string]any{"name": "wider"}, nil)
		wantStatus(t, st, 403, "viewer token mints a token")
		st, _ = call(t, "POST", srv.url+"/auth/tokens", carlTok, nil, nil)
		wantStatus(t, st, 400, "token endpoint without a body")
		in.must("set", "member", carlID, `{"groups":[]}`)
		call(t, "GET", api+"/person", carlView, nil, &rs)
		if names(rs) != "Everyone" {
			t.Errorf("carl's viewer token after he left hr sees %q", names(rs))
		}
		if n := in.psql("archive", "SELECT count(*) FROM pg_roles WHERE rolname LIKE '%\\_g\\_hr'"); n != "0" {
			t.Errorf("the empty hr group keeps its role: %s", n)
		}
		in.must("set", "member", carlID, `{"groups":["hr"]}`)
		call(t, "GET", api+"/person", carlView, nil, &rs)
		if names(rs) != "Everyone,HR only" {
			t.Errorf("carl's viewer token back in hr sees %q", names(rs))
		}
	})

	t.Run("ChatWithoutAnAnthropicKeyIs503", func(t *testing.T) {
		var out map[string]string
		st, _ := call(t, "POST", api+"/chat", aliceTok, map[string]any{"text": "hi"}, &out)
		wantStatus(t, st, 503, "chat without ANTHROPIC_API_KEY")
		if !strings.Contains(out["error"], "ANTHROPIC_API_KEY") {
			t.Errorf("chat error: %v", out)
		}
	})

	t.Run("ReferencesNeedRowsYouCanSee", func(t *testing.T) {
		st, _ := call(t, "GET", api+"/company/"+id(acme), bobTok, nil, nil)
		wantStatus(t, st, 404, "bob reads the referenced company")
		var ve ValidationError
		st, _ = call(t, "POST", api+"/person", bobTok, map[string]any{"name": "Sneaky", "employer": id(acme), "friends": []string{id(all), id(hr)}}, &ve)
		wantStatus(t, st, 422, "bob references hr rows")
		if len(ve.Errors) != 2 || ve.Errors[0].Path != "employer" || ve.Errors[1].Path != "friends[1]" || !strings.HasPrefix(ve.Errors[1].Message, "no person") {
			t.Errorf("reference errors: %+v", ve)
		}
		st, _ = call(t, "DELETE", api+"/company/"+id(acme), aliceTok, nil, nil)
		wantStatus(t, st, 409, "delete a referenced company")
	})

	t.Run("SecretsAreMaskedAndOnlyOwnersRevealThem", func(t *testing.T) {
		in := in.on(t)
		ssn := func(tok string) string {
			var r row
			call(t, "GET", api+"/person/"+id(hr)+"?reveal=true", tok, nil, &r)
			return fmt.Sprint(r["ssn"])
		}
		var plain row
		call(t, "GET", api+"/person/"+id(hr), aliceTok, nil, &plain)
		if plain["ssn"] != "***-**-6789" {
			t.Fatalf("default ssn = %v", plain["ssn"])
		}
		if n := in.psql("archive", "SELECT count(*) FROM archive.secret WHERE position(convert_to('123456789', 'UTF8') in value) > 0"); n != "0" {
			t.Fatalf("secret stored in plain text")
		}
		if n := in.psql("archive", "SELECT count(*) FROM person WHERE to_jsonb(person)::text LIKE '%123456789%'"); n != "0" {
			t.Fatalf("secret leaked into the table")
		}
		if s := ssn(aliceTok); s != "123456789" {
			t.Errorf("an owner's token reveals: %s", s)
		}
		if s := in.must("get", "person", id(hr), "-reveal"); !strings.Contains(s, `"ssn": "123456789"`) {
			t.Errorf("cli reveal: %s", s)
		}
		for who, tok := range map[string]string{"alice's viewer token": aliceView, "carl": carlTok, "carl's viewer token": carlView} {
			if s := ssn(tok); strings.Contains(s, "123456789") {
				t.Errorf("%s reveals: %s", who, s)
			}
		}
		st, _ := call(t, "PATCH", api+"/person/"+id(hr), carlTok, map[string]any{"ssn": "078-05-1120"}, nil)
		wantStatus(t, st, 200, "an editor in hr changes the secret")
		if n := in.psql("archive", "SELECT count(*) FROM archive.secret WHERE row_id = '"+id(hr)+"' AND access = 'hr'"); n != "1" {
			t.Errorf("old secret not replaced: %s rows", n)
		}
		var kept row
		call(t, "GET", api+"/person/"+id(hr), aliceTok, nil, &kept)
		st, _ = call(t, "PUT", api+"/person/"+id(hr), aliceTok, kept, nil)
		wantStatus(t, st, 200, "put the row back with its mask")
		if s := in.must("get", "person", id(hr), "-reveal"); !strings.Contains(s, `"ssn": "078051120"`) {
			t.Errorf("mask round trip lost the secret: %s", s)
		}
		st, _ = call(t, "PATCH", api+"/person/"+id(hr), aliceTok, map[string]any{"ssn": "***-**-0000"}, nil)
		wantStatus(t, st, 422, "a mask that is not the current one")
		call(t, "PATCH", api+"/person/"+id(hr), aliceTok, map[string]any{"ssn": nil}, nil)
		if n := in.psql("archive", "SELECT count(*) FROM archive.secret WHERE row_id = '"+id(hr)+"'"); n != "0" {
			t.Errorf("cleared secret kept: %s rows", n)
		}
		call(t, "PATCH", api+"/person/"+id(hr), aliceTok, map[string]any{"ssn": "123-45-6789"}, nil)
	})

	t.Run("SchemaChangesNeedAnOwner", func(t *testing.T) {
		body := map[string]any{"sql": "ALTER TABLE company ADD COLUMN ein archive.us_ein"}
		st, _ := call(t, "POST", api+"/schema/migrate", aliceView, body, nil)
		wantStatus(t, st, 403, "an owner's viewer token migrates")
		st, _ = call(t, "POST", api+"/schema/migrate", carlTok, body, nil)
		wantStatus(t, st, 403, "an editor migrates")
		var dry, m SchemaMigration
		st, _ = call(t, "POST", api+"/schema/migrate", aliceTok, body, &dry)
		wantStatus(t, st, 200, "owner dry-runs")
		body["confirm"] = dry.Confirm
		st, _ = call(t, "POST", api+"/schema/migrate", aliceTok, body, &m)
		wantStatus(t, st, 200, "owner migrates")
		if dry.Applied || dry.Revision != nil || !m.Applied || len(m.Statements) != 2 || !strings.Contains(m.Statements[1], `ARRAY["ein"::text, "name"::text]`) || m.Revision == nil {
			t.Fatalf("migration: %+v %+v", dry, m)
		}
		var cat Catalog
		call(t, "GET", api+"/types", bobTok, nil, &cat)
		if c := cat.Tables["company"]; c == nil || c.col("ein") == nil || c.col("ein").Type != "us_ein" || c.Description != "A company" || !c.Policy || c.Schema != "public" || cat.Types["archive.email"] == nil {
			t.Fatalf("catalog after apply: %+v", cat.Tables["company"])
		}
		if m := cat.Tables["member"]; m == nil || m.Schema != "archive" || m.col("groups") == nil {
			t.Errorf("member in the catalog: %+v", m)
		}
		var got []runTable
		var one runTable
		call(t, "GET", api+"/schema", bobTok, nil, &got)
		call(t, "GET", api+"/schema?name=company", bobTok, nil, &one)
		if len(got) != 4 || got[0].Name != "company" || !strings.Contains(got[0].DDL, "ein") || one.Name != "company" || one.DDL != got[0].DDL || one.Columns[5].Name != "ein" {
			t.Errorf("schema: %+v %+v", got, one)
		}
		var nf map[string]any
		st, _ = call(t, "GET", api+"/schema?name=nope", bobTok, nil, &nf)
		wantStatus(t, st, 404, "schema of a table that does not exist")
		if !strings.Contains(fmt.Sprint(nf), "there are company, contact, invoice, person") {
			t.Errorf("not found lists the tables: %v", nf)
		}
		st, _ = call(t, "POST", api+"/company", aliceTok, map[string]any{"name": "Ein Co", "ein": "12 3456789"}, nil)
		wantStatus(t, st, 201, "use the new column")
		if out := in.on(t).must("list", "company", "ein=123456789"); !strings.Contains(out, "Ein Co") {
			t.Errorf("cli sees the new column: %s", out)
		}
	})

	t.Run("BrowserSessionsFromGoogleSignIn", func(t *testing.T) {
		if added.Email != "bob@example.com" || added.Name != "Bob" {
			t.Fatalf("user add prints the person: %+v", added)
		}
		in.on(t).fails("user", "link", "bob@example.com")
		st, session := srv.oidc(t, "google", map[string]any{"sub": "bob-1", "email": "bob@example.com", "email_verified": true})
		if st != 303 || session == nil || !session.HttpOnly || session.SameSite != http.SameSiteLaxMode {
			t.Fatalf("bob signs in with Google: %d %v", st, session)
		}
		var me struct {
			Actor Actor  `json:"actor"`
			Role  string `json:"role"`
		}
		call(t, "GET", srv.url+"/auth/me", "", nil, &me, session)
		if me.Actor.Email != "bob@example.com" || me.Actor.Name != "Bob" || me.Role != "viewer" {
			t.Fatalf("me = %+v", me)
		}
		var rs []row
		call(t, "GET", api+"/person", "", nil, &rs, session)
		if names(rs) != "Everyone" {
			t.Errorf("session lists %q", names(rs))
		}
		var opts struct {
			PublicKey struct {
				User struct {
					Name string `json:"name"`
				} `json:"user"`
			} `json:"publicKey"`
		}
		st, _ = call(t, "POST", srv.url+"/auth/passkey/register", "", nil, &opts, session)
		if st != 200 || opts.PublicKey.User.Name != "bob@example.com" {
			t.Errorf("a person signed in with Google starts adding a passkey: %d %+v", st, opts)
		}
		st, _ = call(t, "POST", srv.url+"/auth/signout", "", nil, nil, session)
		wantStatus(t, st, 204, "sign out")
		st, _ = call(t, "GET", srv.url+"/auth/me", "", nil, nil, session)
		wantStatus(t, st, 401, "me after sign out")
	})

	t.Run("OwnersManageMembersAsRowsAndTheRolesFollow", func(t *testing.T) {
		in := in.on(t)
		st, _ := call(t, "POST", api+"/member", bobTok, map[string]any{"email": "eve@example.com"}, nil)
		wantStatus(t, st, 403, "a viewer adds a member")
		st, _ = call(t, "PATCH", api+"/member/"+carlID, carlTok, map[string]any{"role": "owner"}, nil)
		wantStatus(t, st, 403, "an editor promotes themselves")
		var people []Person
		st, _ = call(t, "POST", srv.url+"/auth/users", aliceTok, map[string]any{"email": "Dan@Example.com", "name": "Dan", "role": "editor"}, &people)
		wantStatus(t, st, 201, "an owner's token invites dan")
		i := slices.IndexFunc(people, func(p Person) bool { return p.Email == "dan@example.com" })
		if i < 0 || people[i].Role != "editor" || people[i].Name != "Dan" || len(people) != 4 {
			t.Fatalf("people after inviting dan: %+v", people)
		}
		dan := people[i]
		st, _ = call(t, "POST", srv.url+"/auth/users", aliceTok, map[string]any{"email": "dan@example.com"}, nil)
		wantStatus(t, st, 409, "invite dan twice")
		st, _ = call(t, "POST", api+"/member", aliceTok, map[string]any{"email": "dan@example.com"}, nil)
		wantStatus(t, st, 409, "add dan's row twice")
		if got := in.psql("archive", "SELECT count(*) FROM archive.\"user\" WHERE email = 'dan@example.com' AND workspaces <> '{}'"); got != "1" {
			t.Errorf("dan is not in the workspace in the main database: %s", got)
		}
		var m row
		st, _ = call(t, "PATCH", api+"/member/"+dan.ID.String(), aliceTok, map[string]any{"groups": []string{"payroll", "hr", "hr"}}, &m)
		if st != 200 || fmt.Sprint(m["groups"]) != "[hr payroll]" {
			t.Errorf("dan's groups: %d %v", st, m)
		}
		role := in.role("dan@example.com")
		if got := in.psql("archive", "SELECT pg_has_role('"+role+"', '"+strings.SplitN(role, "_u", 2)[0]+"_g_payroll', 'USAGE')"); got != "t" {
			t.Errorf("dan's role is not in payroll: %s", got)
		}
		st, _ = call(t, "PATCH", api+"/member/"+dan.ID.String(), aliceTok, map[string]any{"email": "dan2@example.com"}, nil)
		wantStatus(t, st, 422, "change an email")
		st, _ = call(t, "PATCH", api+"/member/"+dan.ID.String(), aliceTok, map[string]any{"groups": []string{"Not ok"}}, nil)
		wantStatus(t, st, 422, "a bad group name")
		danTok := decode[Token](t, in.must("-as", "dan@example.com", "token", "create", "-viewer"))
		st, _ = call(t, "GET", api+"/person", danTok.Secret, nil, nil)
		wantStatus(t, st, 200, "dan's viewer token")
		var aliceRow []row
		call(t, "GET", api+"/member?email=alice@example.com", aliceTok, nil, &aliceRow)
		st, _ = call(t, "PATCH", api+"/member/"+id(aliceRow[0]), aliceTok, map[string]any{"role": "editor"}, nil)
		wantStatus(t, st, 409, "demote the last owner")
		st, _ = call(t, "DELETE", srv.url+"/auth/users/alice@example.com", aliceTok, nil, nil)
		wantStatus(t, st, 409, "remove the last owner")
		in.must("set", "member", dan.ID.String(), `{"role":"owner"}`)
		st, _ = call(t, "DELETE", srv.url+"/auth/users/dan@example.com", aliceTok, nil, &people)
		if st != 200 || len(people) != 3 {
			t.Fatalf("remove dan: %d %+v", st, people)
		}
		st, _ = call(t, "GET", api+"/person", danTok.Secret, nil, nil)
		wantStatus(t, st, 401, "dan's token after he was removed")
		if got := in.psql("archive", "SELECT (SELECT count(*) FROM archive.\"user\" WHERE email = 'dan@example.com' AND workspaces <> '{}') || ',' || (SELECT count(*) FROM archive.token t JOIN archive.\"user\" u ON u.id = t.user_id WHERE u.email = 'dan@example.com') || ',' || (SELECT count(*) FROM pg_roles WHERE rolname IN ('"+role+"', '"+strings.Replace(role, "_u", "_m", 1)+"') OR rolname LIKE '%\\_g\\_payroll')"); got != "0,0,0" {
			t.Errorf("left after removing dan (workspaces, tokens, roles): %s", got)
		}
		for _, tk := range decode[[]Token](t, in.must("token", "list")) {
			if tk.Name == "Reader" && tk.Viewer {
				in.must("token", "revoke", tk.ID.String())
			}
		}
		st, _ = call(t, "GET", api+"/person", aliceView, nil, nil)
		wantStatus(t, st, 401, "revoked token")
	})
}

func TestOnThisMachineTheFirstOwnerIsSignedInWithoutLoggingIn(t *testing.T) {
	t.Parallel()
	local := newInst(t)
	local.must("user", "add", "alice@example.com")
	srv := local.serve()
	local.must("user", "add", "bob@example.com")
	here := map[string]string{"X-Forwarded-For": ""}
	me := func(url string, hdr map[string]string) (int, string) {
		var m struct {
			Actor Actor `json:"actor"`
		}
		h := map[string]string{"X-Forwarded-For": ""}
		maps.Copy(h, hdr)
		st, _ := e2eDo(t, "GET", url+"/auth/me", h, nil, &m)
		return st, m.Actor.Email
	}
	if st, who := me(srv.url, nil); st != 200 || who != "alice@example.com" {
		t.Errorf("a request from this machine: %d %q", st, who)
	}
	host := strings.Replace(srv.url, "127.0.0.1", "localhost", 1)
	if st, who := me(host, nil); st != 200 || who != "alice@example.com" {
		t.Errorf("a request to localhost: %d %q", st, who)
	}
	if st, _ := e2eDo(t, "GET", srv.url+"/api/person", here, nil, nil); st == 401 {
		t.Errorf("the API from this machine: %d", st)
	}
	if st, _ := e2eDo(t, "GET", srv.url+"/auth/me", here, nil, nil, &http.Cookie{Name: "archive_session", Value: "arc_stale"}); st != 200 {
		t.Errorf("a stale session cookie on this machine: %d", st)
	}
	if st, _ := e2eDo(t, "GET", srv.url+"/auth/me", map[string]string{"X-Forwarded-For": "", "Authorization": "Bearer arc_nope"}, nil, nil); st != 401 {
		t.Errorf("a bad bearer token on this machine: %d", st)
	}
	for _, hdr := range []map[string]string{{"X-Forwarded-For": "203.0.113.7"}, {"Forwarded": "for=203.0.113.7"}, {"Host": "app.example.com"}} {
		if st, _ := me(srv.url, hdr); st != 401 {
			t.Errorf("a request with %v: %d", hdr, st)
		}
	}
	if st, _ := e2eDo(t, "POST", srv.url+"/mcp", here, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize"}, nil); st != 401 {
		t.Errorf("mcp from this machine without a token: %d", st)
	}

	public := newInst(t, "ARCHIVE_URL=https://app.example.com")
	public.must("user", "add", "alice@example.com")
	pub := public.serve()
	if st, _ := me(pub.url, nil); st != 401 {
		t.Errorf("a request from this machine when ARCHIVE_URL is public: %d", st)
	}
	if !strings.Contains(srv.log.String(), "requests from this machine are signed in as the first owner") || !strings.Contains(pub.log.String(), "no sign-in provider is set") {
		t.Errorf("logs:\n%s\n%s", srv.log, pub.log)
	}
	inside := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("internal")) }))
	defer inside.Close()
	script := filepath.Join(public.dir, "inside.ts")
	os.WriteFile(script, []byte(`return await fetch(`+strconv.Quote(inside.URL)+`).then((r) => r.text(), (e) => e.message);`), 0o644)
	if out := public.must("run", script); !strings.Contains(out, "127.0.0.1 is an internal address, which scripts cannot reach") {
		t.Errorf("a script on a public install reached this machine: %s", out)
	}
}

func TestAnHROnlyTableIsSeenOnlyByTheHRGroupAndTheOwners(t *testing.T) {
	t.Parallel()
	in := newInst(t)
	in.must("user", "add", "alice@example.com")
	in.must("user", "add", "hana@example.com", "-role", "editor")
	in.must("user", "add", "bob@example.com", "-role", "editor")
	in.must("set", "member", id(decode[[]row](t, in.must("list", "member", "email=hana@example.com"))[0]), `{"groups":["hr"]}`)
	salary := strings.Replace(e2eTable("salary", `    who        text NOT NULL,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(who)) STORED`), "    access     text,", "    access     text NOT NULL DEFAULT 'hr' CHECK (access = 'hr'),", 1)
	in.push(with(e2eSchema, "salary.sql", salary))
	r := decode[row](t, in.must("-as", "hana@example.com", "create", "salary", `{"who": "Bob"}`))
	if r["access"] != "hr" {
		t.Fatalf("an hr row without access: %v", r)
	}
	if r := decode[row](t, in.must("-as", "hana@example.com", "create", "salary", `{"who": "Ann", "access": null}`)); r["access"] != "hr" {
		t.Errorf("a null access on an hr table: %v", r)
	}
	if errs := in.fails("create", "salary", `{"who": "Open", "access": "family"}`); !strings.Contains(errs, "salary_access_check") {
		t.Errorf("an owner moves a salary out of hr: %s", errs)
	}
	for who, want := range map[string]string{"alice@example.com": "Bob,Ann", "hana@example.com": "Bob,Ann", "bob@example.com": ""} {
		if got := names2(decode[[]row](t, in.must("-as", who, "list", "salary")), "who"); got != want {
			t.Errorf("%s sees salaries %q, want %q", who, got, want)
		}
		if got := in.must("-as", who, "search", "Bob"); strings.Contains(got, `"salary"`) != (want != "") {
			t.Errorf("%s searching salaries: %s", who, got)
		}
	}
	if errs := in.fails("-as", "bob@example.com", "create", "salary", `{"who": "Me"}`); !strings.Contains(errs, "forbidden") {
		t.Errorf("bob writes a salary: %s", errs)
	}
}

func TestTheFirstOwnerIsAddedOnTheCLIAndSignsInWithAProvider(t *testing.T) {
	t.Parallel()
	in := newInst(t, e2eGoogle(t)...)
	srv := in.serve()
	if log := srv.log.String(); !strings.Contains(log, "no users yet; add the first owner") || strings.Contains(log, "no sign-in provider") {
		t.Errorf("startup log:\n%s", log)
	}
	if errs := in.fails("file", "list"); !strings.Contains(errs, "no users yet") {
		t.Fatalf("list before the first owner: %s", errs)
	}
	if st, _ := call(t, "POST", srv.url+"/auth/setup", "", map[string]any{"email": "first@example.com"}, nil); st != 404 && st != 405 {
		t.Errorf("a setup link: %d", st)
	}
	for _, origin := range []string{strings.Replace(srv.url, "127.0.0.1", "localhost", 1), "https://evil.test"} {
		if _, hdr := e2eFrom(t, "OPTIONS", srv.url+"/api/types", origin); hdr.Get("Access-Control-Allow-Origin") != "" {
			t.Errorf("a localhost install answers %s with CORS: %v", origin, hdr)
		}
	}
	first := map[string]any{"sub": "first-1", "email": "first@example.com", "email_verified": true}
	if st, c := srv.oidc(t, "google", maps.Clone(first)); st != 403 || c != nil {
		t.Errorf("signing in before anyone is added: %d", st)
	}
	if u := decode[User](t, in.must("user", "add", "first@example.com", "-name", "First")); u.Email != "first@example.com" || u.Name != "First" {
		t.Errorf("user add prints the person: %+v", u)
	}
	st, c := srv.oidc(t, "google", maps.Clone(first))
	if st != 303 || c == nil {
		t.Fatalf("the first owner signs in with Google: %d", st)
	}
	var me struct {
		Role string `json:"role"`
	}
	call(t, "GET", srv.url+"/auth/me", "", nil, &me, c)
	if me.Role != "owner" {
		t.Errorf("the first person's role: %+v", me)
	}
}

func e2ePDF(pages ...string) []byte {
	kids := make([]string, len(pages))
	objs := []string{"<< /Type /Catalog /Pages 2 0 R >>", "", "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"}
	for i, text := range pages {
		kids[i] = fmt.Sprintf("%d 0 R", 4+2*i)
		content := "BT /F1 12 Tf 72 712 Td (" + text + ") Tj ET"
		objs = append(objs, fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>", 5+2*i),
			fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content))
	}
	objs[1] = fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(pages))
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offs := make([]int, len(objs))
	for i, o := range objs {
		offs[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, o := range offs {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return b.Bytes()
}

var e2eLatitude = []byte{0, 0, 0, 52, 0, 0, 0, 1, 0, 0, 0, 22, 0, 0, 0, 1, 0, 0, 0x07, 0x8c, 0, 0, 0, 100}

func e2eJPEG(t *testing.T) []byte {
	type ent struct {
		tag, typ uint16
		count    uint32
		data     []byte
	}
	be := binary.BigEndian
	size := func(es []ent) int {
		n := 2 + 12*len(es) + 4
		for _, e := range es {
			if len(e.data) > 4 {
				n += len(e.data)
			}
		}
		return n
	}
	build := func(at int, es []ent) []byte {
		b := be.AppendUint16(nil, uint16(len(es)))
		var tail []byte
		for _, e := range es {
			b = be.AppendUint16(be.AppendUint16(b, e.tag), e.typ)
			b = be.AppendUint32(b, e.count)
			if len(e.data) > 4 {
				b = be.AppendUint32(b, uint32(at+2+12*len(es)+4+len(tail)))
				tail = append(tail, e.data...)
			} else {
				b = append(b, append(bytes.Clone(e.data), make([]byte, 4-len(e.data))...)...)
			}
		}
		return append(be.AppendUint32(b, 0), tail...)
	}
	lon := []byte{0, 0, 0, 4, 0, 0, 0, 1, 0, 0, 0, 53, 0, 0, 0, 1, 0, 0, 0x0d, 0x08, 0, 0, 0, 100}
	ifd0 := []ent{{0x010F, 2, 6, []byte("Canon\x00")}, {0x0110, 2, 4, []byte("EOS\x00")}, {0x8769, 4, 1, nil}, {0x8825, 4, 1, nil}}
	exif := []ent{{0x9003, 2, 20, []byte("2026:10:01 12:30:00\x00")}}
	gps := []ent{{1, 2, 2, []byte("N\x00")}, {2, 5, 3, e2eLatitude}, {3, 2, 2, []byte("E\x00")}, {4, 5, 3, lon}}
	exifAt := 8 + size(ifd0)
	gpsAt := exifAt + size(exif)
	ifd0[2].data, ifd0[3].data = be.AppendUint32(nil, uint32(exifAt)), be.AppendUint32(nil, uint32(gpsAt))
	tiff := append([]byte("MM\x00\x2a\x00\x00\x00\x08"), build(8, ifd0)...)
	tiff = append(append(tiff, build(exifAt, exif)...), build(gpsAt, gps)...)
	app1 := append([]byte("Exif\x00\x00"), tiff...)
	img := image.NewRGBA(image.Rect(0, 0, 640, 480))
	for x := range 640 {
		for y := range 480 {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 128, 255})
		}
	}
	var enc bytes.Buffer
	if err := jpeg.Encode(&enc, img, nil); err != nil {
		t.Fatal(err)
	}
	j := enc.Bytes()
	out := append([]byte{0xFF, 0xD8, 0xFF, 0xE1, byte((len(app1) + 2) >> 8), byte(len(app1) + 2)}, app1...)
	return append(out, j[2:]...)
}

func e2eUpload(t *testing.T, url, tok, name string, b []byte) (int, File) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, _ := w.CreateFormFile("file", name)
	part.Write(b)
	w.Close()
	req, _ := http.NewRequest("POST", url, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var f File
	json.NewDecoder(resp.Body).Decode(&f)
	return resp.StatusCode, f
}

func e2eGet(t *testing.T, url, tok string, hdr ...string) (*http.Response, []byte) {
	t.Helper()
	req, _ := http.NewRequest("GET", url, nil)
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, b
}

func e2eDoneFile(t *testing.T, api, tok, id string) File {
	t.Helper()
	var f File
	call(t, "GET", api+"/files/"+id, tok, nil, &f)
	if f.Status != "done" {
		t.Fatalf("file %s is %q right after its upload, want done", id, f.Status)
	}
	return f
}

type e2eCut struct{ r io.Reader }

func (c *e2eCut) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if err == io.EOF {
		return n, errors.New("connection lost")
	}
	return n, err
}

func e2eTus(t *testing.T, method, url, tok string, body io.Reader, size int64, hdr ...string) (*http.Response, error) {
	t.Helper()
	req, _ := http.NewRequest(method, url, body)
	req.ContentLength = size
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Tus-Resumable", "1.0.0")
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
	return resp, err
}

func TestFilesUploadProcessShareAndReference(t *testing.T) {
	t.Parallel()
	in := newInst(t)
	in.must("user", "add", "owner@example.com")
	srv := in.serve()
	api := srv.url + "/api"
	in.must("user", "add", "alice@example.com", "-role", "editor")
	in.must("user", "add", "bob@example.com", "-role", "editor")
	in.groups("alice@example.com", "alice")
	in.groups("bob@example.com", "bob")
	in.push(e2eSchema)
	aliceTok := decode[Token](t, in.must("-as", "alice@example.com", "token", "create", "-name", "Claude")).Secret
	bobTok := decode[Token](t, in.must("-as", "bob@example.com", "token", "create")).Secret

	big := bytes.Repeat([]byte("archive keeps all your stuff. "), 100000)
	var bigFile File

	t.Run("TusUploadResumesAfterAnInterruption", func(t *testing.T) {
		tus := api + "/files/tus"
		resp, _ := e2eTus(t, "OPTIONS", tus, aliceTok, nil, 0)
		if resp.StatusCode != 204 || !strings.Contains(resp.Header.Get("Tus-Extension"), "creation") || !strings.Contains(resp.Header.Get("Tus-Extension"), "termination") {
			t.Fatalf("options: %d %v", resp.StatusCode, resp.Header)
		}
		req, _ := http.NewRequest("POST", tus, nil)
		req.Header.Set("Authorization", "Bearer "+aliceTok)
		req.Header.Set("Upload-Length", "10")
		resp, _ = http.DefaultClient.Do(req)
		resp.Body.Close()
		wantStatus(t, resp.StatusCode, 412, "create without Tus-Resumable")

		meta := "filename " + base64.StdEncoding.EncodeToString([]byte("big.txt")) + ",access " + base64.StdEncoding.EncodeToString([]byte("alice"))
		resp, _ = e2eTus(t, "POST", tus, aliceTok, nil, 0, "Upload-Length", strconv.Itoa(len(big)), "Upload-Metadata", meta)
		wantStatus(t, resp.StatusCode, 201, "create upload")
		loc := srv.url + resp.Header.Get("Location")
		offset := func(tok string) (int, int) {
			resp, _ := e2eTus(t, "HEAD", loc, tok, nil, 0)
			n, _ := strconv.Atoi(resp.Header.Get("Upload-Offset"))
			return resp.StatusCode, n
		}
		if st, _ := offset(bobTok); st != 404 {
			t.Errorf("bob reads alice's upload: %d", st)
		}

		half := 1 << 20
		if _, err := e2eTus(t, "PATCH", loc, aliceTok, &e2eCut{bytes.NewReader(big[:half])}, int64(len(big)),
			"Content-Type", "application/offset+octet-stream", "Upload-Offset", "0"); err == nil {
			t.Fatal("interrupted PATCH succeeded")
		}
		var got int
		for deadline := time.Now().Add(10 * time.Second); got == 0; time.Sleep(50 * time.Millisecond) {
			var st int
			if st, got = offset(aliceTok); st != 200 || time.Now().After(deadline) {
				t.Fatalf("HEAD after the interruption: %d offset %d", st, got)
			}
		}
		if got > half {
			t.Fatalf("offset %d after sending %d bytes", got, half)
		}
		resp, _ = e2eTus(t, "PATCH", loc, aliceTok, bytes.NewReader(big[got+1:]), int64(len(big)-got-1),
			"Content-Type", "application/offset+octet-stream", "Upload-Offset", strconv.Itoa(got+1))
		wantStatus(t, resp.StatusCode, 409, "PATCH at the wrong offset")
		resp, _ = e2eTus(t, "PATCH", loc, aliceTok, bytes.NewReader(big[got:]), int64(len(big)-got),
			"Content-Type", "application/offset+octet-stream", "Upload-Offset", strconv.Itoa(got))
		if resp.StatusCode != 204 || resp.Header.Get("Upload-Offset") != strconv.Itoa(len(big)) || resp.Header.Get("Archive-File-Id") == "" {
			t.Fatalf("resume: %d %v", resp.StatusCode, resp.Header)
		}
		if st, _ := offset(aliceTok); st != 404 {
			t.Errorf("finished upload still answers HEAD: %d", st)
		}
		bigFile = e2eDoneFile(t, api, aliceTok, resp.Header.Get("Archive-File-Id"))
		_, body := e2eGet(t, api+"/files/"+bigFile.ID.String()+"/content", aliceTok)
		if !bytes.Equal(body, big) || bigFile.Name != "big.txt" || bigFile.Size != int64(len(big)) {
			t.Fatalf("downloaded %d bytes, want %d; file %+v", len(body), len(big), bigFile)
		}

		resp, _ = e2eTus(t, "POST", tus, aliceTok, nil, 0, "Upload-Length", "5")
		loc = srv.url + resp.Header.Get("Location")
		resp, _ = e2eTus(t, "DELETE", loc, aliceTok, nil, 0)
		wantStatus(t, resp.StatusCode, 204, "terminate upload")
		if st, _ := offset(aliceTok); st != 404 {
			t.Errorf("terminated upload answers HEAD: %d", st)
		}
	})

	t.Run("RangeRequestsLetVideoSeek", func(t *testing.T) {
		u := api + "/files/" + bigFile.ID.String() + "/content"
		resp, body := e2eGet(t, u, aliceTok, "Range", "bytes=1000-1999")
		if resp.StatusCode != 206 || !bytes.Equal(body, big[1000:2000]) || resp.Header.Get("Content-Range") != fmt.Sprintf("bytes 1000-1999/%d", len(big)) {
			t.Fatalf("range: %d %q %v", resp.StatusCode, resp.Header.Get("Content-Range"), len(body))
		}
		resp, body = e2eGet(t, u, aliceTok, "Range", "bytes=-10")
		if resp.StatusCode != 206 || !bytes.Equal(body, big[len(big)-10:]) || resp.Header.Get("Accept-Ranges") != "bytes" {
			t.Errorf("suffix range: %d %q", resp.StatusCode, body)
		}
		resp, _ = e2eGet(t, u, aliceTok, "If-None-Match", `"`+bigFile.Hash+`"`)
		wantStatus(t, resp.StatusCode, 304, "etag")
	})

	t.Run("IdenticalContentIsStoredOnce", func(t *testing.T) {
		st, again := e2eUpload(t, api+"/files?access=alice", aliceTok, "big.txt", big)
		wantStatus(t, st, 201, "upload the same file again")
		_, copied := e2eUpload(t, api+"/files?access=alice", aliceTok, "copy.txt", big)
		_, bobs := e2eUpload(t, api+"/files", bobTok, "big.txt", big)
		if again.ID != bigFile.ID || copied.ID == bigFile.ID || bobs.ID == bigFile.ID || copied.Hash != bigFile.Hash || bobs.Hash != bigFile.Hash {
			t.Fatalf("same name %s, copy %s, bob %s, first %s", again.ID, copied.ID, bobs.ID, bigFile.ID)
		}
		blobs, _ := filepath.Glob(filepath.Join(in.dir, "data", "blobs", "*", "*", bigFile.Hash))
		if n := in.psql("archive", "SELECT count(*) FROM archive.file WHERE hash = '"+bigFile.Hash+"'"); n != "3" || len(blobs) != 1 {
			t.Errorf("%s file rows, %d blobs", n, len(blobs))
		}
		parts, _ := os.ReadDir(filepath.Join(in.dir, "data", "blobs", "uploads"))
		if n := in.psql("archive", "SELECT count(*) FROM archive.upload"); n != "0" || len(parts) != 0 {
			t.Errorf("uploads left behind: %s rows, %d part directories", n, len(parts))
		}
	})

	photo := e2eJPEG(t)
	var photoFile File
	t.Run("PhotoMetadataThumbnailAndLocationMarkedPersonal", func(t *testing.T) {
		st, f := e2eUpload(t, api+"/files?access=alice", aliceTok, "photo.jpg", photo)
		wantStatus(t, st, 201, "upload photo")
		photoFile = e2eDoneFile(t, api, aliceTok, f.ID.String())
		m := photoFile.Metadata
		loc, _ := m["personal"].(map[string]any)["location"].(map[string]any)
		if photoFile.MIME != "image/jpeg" || fmt.Sprint(m["width"]) != "640" || fmt.Sprint(m["height"]) != "480" || m["camera"] != "Canon EOS" ||
			m["taken_at"] != "2026-10-01T12:30:00" || fmt.Sprint(loc["lat"]) != "52.3720333" || fmt.Sprint(loc["lon"]) != "4.8926" {
			t.Fatalf("photo: %+v", photoFile)
		}
		resp, body := e2eGet(t, api+"/files/"+f.ID.String()+"/thumbnail", aliceTok)
		c, _, err := image.DecodeConfig(bytes.NewReader(body))
		if resp.StatusCode != 200 || err != nil || c.Width != 320 || c.Height != 240 || bytes.Contains(body, e2eLatitude) {
			t.Errorf("thumbnail: %d %v %+v", resp.StatusCode, err, c)
		}
	})

	t.Run("GPSIsStrippedOnRequestAndInSignedLinks", func(t *testing.T) {
		u := api + "/files/" + photoFile.ID.String() + "/content"
		_, plain := e2eGet(t, u, aliceTok)
		_, stripped := e2eGet(t, u+"?strip=gps", aliceTok)
		_, err := jpeg.Decode(bytes.NewReader(stripped))
		if !bytes.Equal(plain, photo) || bytes.Contains(stripped, e2eLatitude) || len(stripped) != len(photo) || !bytes.Contains(stripped, []byte("Canon")) || err != nil {
			t.Fatalf("plain %d bytes, stripped %d bytes, decode %v", len(plain), len(stripped), err)
		}
		var link FileLink
		st, _ := call(t, "POST", api+"/files/"+photoFile.ID.String()+"/link?expires=1s", aliceTok, nil, &link)
		wantStatus(t, st, 200, "create link")
		resp, shared := e2eGet(t, link.URL, "")
		if resp.StatusCode != 200 || bytes.Contains(shared, e2eLatitude) || !bytes.Equal(shared, stripped) {
			t.Fatalf("signed link: %d, %d bytes", resp.StatusCode, len(shared))
		}
		resp, _ = e2eGet(t, link.URL+"0", "")
		wantStatus(t, resp.StatusCode, 403, "tampered link")
		resp, _ = e2eGet(t, strings.Replace(link.URL, photoFile.ID.String(), bigFile.ID.String(), 1), "")
		wantStatus(t, resp.StatusCode, 403, "signature for another file")
		time.Sleep(time.Until(link.ExpiresAt) + 1100*time.Millisecond)
		resp, _ = e2eGet(t, link.URL, "")
		wantStatus(t, resp.StatusCode, 403, "expired link")
	})

	t.Run("AnotherUserCannotSeeAFileOrItsBlob", func(t *testing.T) {
		in := in.on(t)
		id := photoFile.ID.String()
		for _, p := range []string{"", "/content", "/thumbnail", "/text"} {
			resp, _ := e2eGet(t, api+"/files/"+id+p, bobTok)
			wantStatus(t, resp.StatusCode, 404, "bob GET file"+p)
		}
		st, _ := call(t, "POST", api+"/files/"+id+"/link", bobTok, nil, nil)
		wantStatus(t, st, 404, "bob links alice's file")
		var list []File
		call(t, "GET", api+"/files", bobTok, nil, &list)
		if len(list) != 1 || list[0].Name != "big.txt" {
			t.Errorf("bob lists %+v", list)
		}
		in.fails("-as", "bob@example.com", "file", "get", id, "-o", "-")
		role := in.role("bob@example.com")
		in.psql("archive", "ALTER ROLE \""+role+"\" LOGIN")
		if got := in.psql(role, "SELECT count(*) FROM archive.file WHERE name = 'photo.jpg'"); got != "0" {
			t.Errorf("psql as bob sees %s photos", got)
		}
		in.psql("archive", "ALTER ROLE \""+role+"\" NOLOGIN")
	})

	t.Run("TextExtractionAndRowsThatReferenceFiles", func(t *testing.T) {
		in := in.on(t)
		st, f := e2eUpload(t, api+"/files?access=alice", aliceTok, "invoice.pdf", e2ePDF("Cover letter", "Invoice INV-9 total due"))
		wantStatus(t, st, 201, "upload pdf")
		f = e2eDoneFile(t, api, aliceTok, f.ID.String())
		if f.Status != "done" || fmt.Sprint(f.Metadata["pages"]) != "2" {
			t.Fatalf("pdf: %+v", f)
		}
		_, text := e2eGet(t, api+"/files/"+f.ID.String()+"/text", aliceTok)
		if !strings.Contains(string(text), "Cover letter") || !strings.Contains(string(text), "INV-9") || !strings.Contains(string(text), "\f") {
			t.Errorf("pdf text: %q", text)
		}
		var inv row
		st, _ = call(t, "POST", api+"/invoice", aliceTok, map[string]any{"number": "INV-9", "document": f.ID.String(), "total": "USD 12.50"}, &inv)
		wantStatus(t, st, 201, "invoice that references its file")
		if inv["document"] != f.ID.String() {
			t.Errorf("invoice: %v", inv)
		}
		_, bobs := e2eUpload(t, api+"/files?access=bob", bobTok, "bob.txt", []byte("bob"))
		var ve ValidationError
		st, _ = call(t, "POST", api+"/invoice", aliceTok, map[string]any{"number": "INV-10", "document": bobs.ID.String()}, &ve)
		wantStatus(t, st, 422, "invoice that references bob's file")
		if len(ve.Errors) != 1 || ve.Errors[0].Path != "document" || !strings.HasPrefix(ve.Errors[0].Message, "no file") {
			t.Errorf("reference to a hidden file: %+v", ve)
		}

		_, page := e2eUpload(t, api+"/files?access=alice", aliceTok, "order.html", []byte("<html><head><style>p{}</style><script>var INV = 1</script></head><body><p>Order</p><p>INV-5</p></body></html>"))
		page = e2eDoneFile(t, api, aliceTok, page.ID.String())
		resp, body := e2eGet(t, api+"/files/"+page.ID.String()+"/text", aliceTok)
		if resp.StatusCode != 200 || string(body) != "Order INV-5" {
			t.Errorf("html text: %d %q", resp.StatusCode, body)
		}

		eml := filepath.Join(in.dir, "lunch.eml")
		os.WriteFile(eml, []byte("From: Carol <carol@example.com>\r\nSubject: Lunch\r\nContent-Type: text/plain\r\n\r\nSee you at noon, INV-6"), 0o644)
		mail := decode[File](t, in.must("file", "add", eml))
		if mail.Status != "done" || len(mail.Metadata) != 0 {
			t.Errorf("eml: %+v", mail)
		}
		if _, text := e2eGet(t, api+"/files/"+mail.ID.String()+"/text", aliceTok); len(text) != 0 {
			t.Errorf("eml text: %q", text)
		}
		if out := in.must("file", "get", mail.ID.String(), "-o", "-"); !strings.HasPrefix(out, "From: Carol") {
			t.Errorf("file get -o -: %q", out)
		}
	})
}

type e2eGCS struct {
	mu      sync.Mutex
	objects map[string][]byte
	puts    int
}

func (s *e2eGCS) handler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer meta" {
			w.WriteHeader(403)
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		b, ok := s.objects[r.URL.Path]
		switch r.Method {
		case "PUT":
			body, _ := io.ReadAll(r.Body)
			s.objects[r.URL.Path] = body
			s.puts++
		case "DELETE":
			delete(s.objects, r.URL.Path)
			w.WriteHeader(204)
		case "HEAD", "GET":
			if !ok {
				w.WriteHeader(404)
				return
			}
			http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(b))
		}
	})
}

func TestGCSBlobBackendSignsWithTheServiceAccountTokenFromTheMetadataServer(t *testing.T) {
	t.Parallel()
	gcs := &e2eGCS{objects: map[string][]byte{}}
	fake := httptest.NewServer(gcs.handler(t))
	defer fake.Close()
	in := newInst(t, "ARCHIVE_BLOB_URL=gs://photos/archive?endpoint="+url.QueryEscape(fake.URL))
	in.must("user", "add", "ann@example.com")
	content := bytes.Repeat([]byte("0123456789"), 5000)
	path := filepath.Join(in.dir, "numbers.txt")
	os.WriteFile(path, content, 0o644)
	f := decode[File](t, in.must("file", "add", path))
	os.WriteFile(filepath.Join(in.dir, "copy.txt"), content, 0o644)
	in.must("file", "add", filepath.Join(in.dir, "copy.txt"))
	key := "/photos/archive/" + in.psql("archive", "SELECT id FROM archive.workspace ORDER BY created_at LIMIT 1") + "/" + f.Hash[:2] + "/" + f.Hash
	gcs.mu.Lock()
	_, stored := gcs.objects[key]
	puts := gcs.puts
	gcs.mu.Unlock()
	if !stored || puts != 1 || f.Status != "done" {
		t.Fatalf("object %s stored %v, %d puts, file %+v", key, stored, puts, f)
	}
	if _, err := os.Stat(filepath.Join(in.dir, "data", "blobs")); err == nil {
		t.Error("blobs written to the data directory too")
	}
	if out := in.must("file", "get", f.ID.String(), "-o", "-"); out != string(content) {
		t.Errorf("download through gcs: %d bytes", len(out))
	}
	srv := in.serve()
	tok := decode[Token](t, in.must("token", "create")).Secret
	resp, body := e2eGet(t, srv.url+"/api/files/"+f.ID.String()+"/content", tok, "Range", "bytes=49990-")
	if resp.StatusCode != 206 || string(body) != "0123456789" {
		t.Errorf("range through gcs: %d %q", resp.StatusCode, body)
	}
	resp, body = e2eGet(t, srv.url+"/api/files/"+f.ID.String()+"/content", tok, "Range", "bytes=5-14")
	if resp.StatusCode != 206 || string(body) != "5678901234" {
		t.Errorf("middle range through gcs: %d %q", resp.StatusCode, body)
	}
}

func TestTwoInstancesShareOnePostgresAndBlobStore(t *testing.T) {
	t.Parallel()
	a := newInst(t)
	a.must("user", "add", "alice@example.com")
	b := a.twin()
	srvA, srvB := a.serve(), b.serve()
	a.push(e2eSchema)
	tok := decode[Token](t, a.must("token", "create")).Secret

	t.Run("SchemaAppliedOnOneIsUsedOnTheOther", func(t *testing.T) {
		var schema []runTable
		tables := func() (out []string) {
			call(t, "GET", srvB.url+"/api/schema", tok, nil, &schema)
			for _, s := range schema {
				out = append(out, s.Name)
			}
			return out
		}
		if got := tables(); fmt.Sprint(got) != "[company contact invoice person]" {
			t.Fatalf("schema on b: %v", got)
		}
		st, _ := call(t, "POST", srvB.url+"/api/company", tok, map[string]any{"name": "Acme"}, nil)
		wantStatus(t, st, 201, "create on b in a table pushed on a")
		var cos []row
		call(t, "GET", srvA.url+"/api/company", tok, nil, &cos)
		if names(cos) != "Acme" {
			t.Errorf("companies on a: %v", cos)
		}
		if _, err := os.Stat(filepath.Join(a.dir, "schema")); err == nil {
			t.Error("schema files written to the data directory")
		}
		var dry SchemaMigration
		st, _ = call(t, "POST", srvB.url+"/api/schema/migrate", tok, map[string]any{"sql": e2eNote}, &dry)
		wantStatus(t, st, 200, "dry-run note on b")
		st, _ = call(t, "POST", srvA.url+"/api/schema/migrate", tok, map[string]any{"sql": e2eNote, "confirm": dry.Confirm}, nil)
		wantStatus(t, st, 200, "migrate note on a with b's confirm")
		if got := tables(); fmt.Sprint(got) != "[company contact invoice note person]" || !strings.Contains(schema[3].DDL, "CREATE INDEX note_search ON public.note USING gin (search)") {
			t.Errorf("note on b: %+v", schema)
		}
		st, _ = call(t, "POST", srvB.url+"/api/note", tok, map[string]any{"title": "Shared"}, nil)
		wantStatus(t, st, 201, "create on b in a table applied on a")
	})

	t.Run("AFreshContainerKeepsTheSchema", func(t *testing.T) {
		c := a.twin()
		if ddl := c.must("schema", "show"); !strings.Contains(ddl, "invoice_status") {
			t.Fatalf("schema on a fresh container: %s", ddl)
		}
		if n := a.psql("archive", "SELECT count(*) FROM company"); n != "1" {
			t.Errorf("%s companies after a fresh container started", n)
		}
	})

	t.Run("AFileIsDoneWhenItsUploadReturns", func(t *testing.T) {
		st, f := e2eUpload(t, srvB.url+"/api/files", tok, "plan.html", []byte("<p>Stateless on Cloud Run</p>"))
		wantStatus(t, st, 201, "upload")
		_, text := e2eGet(t, srvA.url+"/api/files/"+f.ID.String()+"/text", tok)
		if f.Status != "done" || string(text) != "Stateless on Cloud Run" {
			t.Errorf("upload returned %+v, text %q", f, text)
		}
	})

	t.Run("TusUploadStartedOnOneFinishesOnTheOther", func(t *testing.T) {
		data := bytes.Repeat([]byte("stateless "), 50000)
		meta := "filename " + base64.StdEncoding.EncodeToString([]byte("two.txt"))
		resp, _ := e2eTus(t, "POST", srvA.url+"/api/files/tus", tok, nil, 0, "Upload-Length", strconv.Itoa(len(data)), "Upload-Metadata", meta)
		wantStatus(t, resp.StatusCode, 201, "create on a")
		loc := resp.Header.Get("Location")
		resp, _ = e2eTus(t, "PATCH", srvA.url+loc, tok, bytes.NewReader(data[:200000]), 200000,
			"Content-Type", "application/offset+octet-stream", "Upload-Offset", "0")
		wantStatus(t, resp.StatusCode, 204, "first half on a")
		resp, _ = e2eTus(t, "HEAD", srvB.url+loc, tok, nil, 0)
		if resp.StatusCode != 200 || resp.Header.Get("Upload-Offset") != "200000" {
			t.Fatalf("head on b: %d %v", resp.StatusCode, resp.Header)
		}
		resp, _ = e2eTus(t, "PATCH", srvB.url+loc, tok, bytes.NewReader(data[200000:]), int64(len(data)-200000),
			"Content-Type", "application/offset+octet-stream", "Upload-Offset", "200000")
		if resp.StatusCode != 204 || resp.Header.Get("Archive-File-Id") == "" {
			t.Fatalf("rest on b: %d %v", resp.StatusCode, resp.Header)
		}
		f := e2eDoneFile(t, srvA.url+"/api", tok, resp.Header.Get("Archive-File-Id"))
		_, body := e2eGet(t, srvA.url+"/api/files/"+f.ID.String()+"/content", tok)
		if !bytes.Equal(body, data) || f.Name != "two.txt" {
			t.Fatalf("downloaded %d bytes of %s", len(body), f.Name)
		}
		parts, _ := os.ReadDir(filepath.Join(a.dir, "data", "blobs", "uploads"))
		if n := a.psql("archive", "SELECT count(*) FROM archive.upload"); n != "0" || len(parts) != 0 {
			t.Errorf("uploads left behind: %s rows, %d part directories", n, len(parts))
		}
	})
}

func TestTablesThatReferenceEachOther(t *testing.T) {
	t.Parallel()
	in := newInst(t)
	in.must("user", "add", "ann@example.com")
	in.migrate(`CREATE TABLE alpha (name text NOT NULL, beta uuid);
CREATE TABLE beta (name text NOT NULL, alpha uuid REFERENCES alpha, gamma uuid);
CREATE TABLE gamma (name text NOT NULL, alpha uuid REFERENCES alpha);
ALTER TABLE alpha ADD FOREIGN KEY (beta) REFERENCES beta;
ALTER TABLE beta ADD FOREIGN KEY (gamma) REFERENCES gamma;
-- the name; trimmed
CREATE FUNCTION alpha_trim() RETURNS trigger LANGUAGE plpgsql AS $body$ BEGIN NEW.name = trim(NEW.name); RETURN NEW; END $body$;
CREATE TRIGGER alpha_trim BEFORE INSERT ON alpha FOR EACH ROW EXECUTE FUNCTION alpha_trim();
COMMENT ON TABLE gamma IS E'one; two \' three';`)
	a := decode[row](t, in.must("create", "alpha", `{"name":" A "}`))
	in.must("create", "beta", `{"name":"B","alpha":"`+id(a)+`"}`)
	ddl := in.must("schema", "show")
	if !strings.Contains(ddl, "REFERENCES beta(id)") || !strings.Contains(ddl, "REFERENCES alpha(id)") || !strings.Contains(ddl, "one; two '' three") || a["name"] != "A" {
		t.Errorf("tables that reference each other: %s %v", ddl, a)
	}
	if got := in.psql("archive", "SELECT count(*) FROM pg_constraint WHERE contype = 'f' AND conrelid::regclass::text IN ('alpha', 'beta', 'gamma')"); got != "4" {
		t.Errorf("%s foreign keys between the tables", got)
	}
	if errs := in.migrateFails("CREATE TABLE delta (name text, alpha uuid REFERENCES alpha);\nALTER TABLE delta ADD FOREIGN KEY (name) REFERENCES epsilon"); !strings.Contains(errs, "statement ") || !strings.Contains(errs, `"epsilon" does not exist`) {
		t.Errorf("a reference to a missing table: %s", errs)
	}
}

var e2eNote = e2eTable("note", `    title      text NOT NULL,
    body       text,
    ssn        archive.us_ssn,
    search     tsvector GENERATED ALWAYS AS (setweight(archive.search_doc(title), 'A') || setweight(archive.search_doc(body, ssn), 'B')) STORED`) +
	"CREATE INDEX note_search ON note USING gin (search);\n"

func e2eSearch(t *testing.T, api, tok, q string, params ...string) []SearchHit {
	t.Helper()
	u := api + "/search?q=" + url.QueryEscape(q)
	for i := 0; i+1 < len(params); i += 2 {
		u += "&" + params[i] + "=" + url.QueryEscape(params[i+1])
	}
	var hits []SearchHit
	st, _ := call(t, "GET", u, tok, nil, &hits)
	wantStatus(t, st, 200, "search "+q)
	return hits
}

func e2eTitles(hits []SearchHit) string {
	var out []string
	for _, h := range hits {
		var d map[string]any
		json.Unmarshal(h.Data, &d)
		out = append(out, fmt.Sprint(cmp.Or[any](d["title"], d["name"])))
	}
	return strings.Join(out, "|")
}

func TestFullTextSearchAcrossTablesFilesAndUsers(t *testing.T) {
	t.Parallel()
	in := newInst(t)
	in.must("user", "add", "owner@example.com")
	srv := in.serve()
	in.must("user", "add", "alice@example.com", "-role", "editor")
	api := srv.url + "/api"
	in.must("user", "add", "bob@example.com", "-role", "editor")
	in.groups("alice@example.com", "alice")
	in.groups("bob@example.com", "bob")
	files := with(e2eSchema, "note.sql", e2eNote+"CREATE INDEX note_title ON note (title);\n")
	in.push(files)
	aliceTok := decode[Token](t, in.must("-as", "alice@example.com", "token", "create")).Secret
	bobTok := decode[Token](t, in.must("-as", "bob@example.com", "token", "create")).Secret
	note := func(tok string, data map[string]any) {
		t.Helper()
		data["access"] = map[string]string{aliceTok: "alice", bobTok: "bob"}[tok]
		st, _ := call(t, "POST", api+"/note", tok, data, nil)
		wantStatus(t, st, 201, "create note")
	}
	note(aliceTok, map[string]any{"title": "Rental agreement Brooklyn apartment", "body": "Rental agreement for the apartment on Atlantic Avenue. Rent is 2450 dollars a month."})
	note(aliceTok, map[string]any{"title": "Weekend away", "body": "Train to Brooklyn"})
	note(aliceTok, map[string]any{"title": "Lease agreement London flat", "body": "Tenancy for the flat in Camden, rent paid monthly."})
	note(aliceTok, map[string]any{"title": "Café Grumpy", "body": "Coffee and apple pie"})
	note(aliceTok, map[string]any{"title": "Tax return", "ssn": "111222333"})
	note(bobTok, map[string]any{"title": "Bob rental agreement Brooklyn"})
	call(t, "POST", api+"/company", aliceTok, map[string]any{"name": "Brooklyn Plumbing", "access": "alice"}, nil)
	_, txt := e2eUpload(t, api+"/files?access=alice", aliceTok, "rental.txt", []byte("The landlord rents the apartment on Bedford Avenue to the tenant."))
	_, pdf := e2eUpload(t, api+"/files?access=alice", aliceTok, "policy.pdf", e2ePDF("Home insurance policy", "Covers water damage in the kitchen"))
	e2eDoneFile(t, api, aliceTok, txt.ID.String())
	e2eDoneFile(t, api, aliceTok, pdf.ID.String())

	t.Run("RowsRankByTheirWords", func(t *testing.T) {
		hits := e2eSearch(t, api, aliceTok, "rental brooklyn")
		if got := e2eTitles(hits); !strings.HasPrefix(got, "Rental agreement Brooklyn apartment|") || !strings.Contains(got, "Brooklyn Plumbing") || !strings.Contains(got, "Weekend away") {
			t.Fatalf("rental brooklyn: %s", got)
		}
		if hits[0].Kind != "entity" || hits[0].Type != "note" {
			t.Errorf("hit: %+v", hits[0])
		}
		if got := e2eTitles(e2eSearch(t, api, aliceTok, "the lease for the London flat")); !strings.HasPrefix(got, "Lease agreement London flat") {
			t.Errorf("lease: %s", got)
		}
		if got := e2eTitles(e2eSearch(t, api, aliceTok, "rent")); !strings.Contains(got, "Rental agreement Brooklyn apartment") || !strings.Contains(got, "rental.txt") {
			t.Errorf("prefix rent: %s", got)
		}
	})

	t.Run("CaseAndAccentsAreFolded", func(t *testing.T) {
		for _, q := range []string{"cafe", "CAFÉ", "café grumpy"} {
			if got := e2eTitles(e2eSearch(t, api, aliceTok, q)); !strings.HasPrefix(got, "Café Grumpy") {
				t.Errorf("%s: %s", q, got)
			}
		}
	})

	t.Run("FileTextIsSearched", func(t *testing.T) {
		hits := e2eSearch(t, api, aliceTok, "landlord Bedford")
		if len(hits) == 0 || hits[0].ID != txt.ID || hits[0].Type != "file" || !strings.Contains(hits[0].Snippet, "**landlord**") {
			t.Fatalf("text file: %+v", hits)
		}
		hits = e2eSearch(t, api, aliceTok, "water damage", "type", "file")
		if len(hits) != 1 || hits[0].ID != pdf.ID {
			t.Fatalf("pdf: %+v", hits)
		}
	})

	t.Run("FiltersNarrowTheResults", func(t *testing.T) {
		if got := e2eTitles(e2eSearch(t, api, aliceTok, "rent", "type", "note")); strings.Contains(got, "rental.txt") || !strings.Contains(got, "Rental agreement") {
			t.Errorf("type note: %s", got)
		}
		day := time.Now().Format(time.DateOnly)
		if n := len(e2eSearch(t, api, aliceTok, "rent", "from", day, "to", day)); n < 2 {
			t.Errorf("today: %d hits", n)
		}
		if n := len(e2eSearch(t, api, aliceTok, "rent", "from", time.Now().AddDate(0, 0, 1).Format(time.DateOnly))); n != 0 {
			t.Errorf("from tomorrow: %d hits", n)
		}
		if n := len(e2eSearch(t, api, aliceTok, "rent", "to", time.Now().AddDate(0, 0, -1).Format(time.DateOnly))); n != 0 {
			t.Errorf("to yesterday: %d hits", n)
		}
		st, _ := call(t, "GET", api+"/search?q=rent&type=nothing", aliceTok, nil, nil)
		wantStatus(t, st, 404, "unknown type")
		st, _ = call(t, "GET", api+"/search?q=rent&from=soon", aliceTok, nil, nil)
		wantStatus(t, st, 422, "bad date")
		st, _ = call(t, "GET", api+"/search?q=+", aliceTok, nil, nil)
		wantStatus(t, st, 422, "empty query")
	})

	t.Run("SecretsNeverMatch", func(t *testing.T) {
		in := in.on(t)
		if hits := e2eSearch(t, api, aliceTok, "tax return"); len(hits) != 1 {
			t.Fatalf("secret row by title: %+v", hits)
		}
		for _, q := range []string{"111222333", "111 22 2333"} {
			if hits := e2eSearch(t, api, aliceTok, q); len(hits) != 0 {
				t.Errorf("%s matches %s", q, e2eTitles(hits))
			}
		}
		if n := in.psql("archive", "SELECT count(*) FROM note WHERE search::text LIKE '%111222333%'"); n != "0" {
			t.Errorf("secret in %s search vectors", n)
		}
	})

	t.Run("AnotherUsersDataNeverShowsUp", func(t *testing.T) {
		in := in.on(t)
		if got := e2eTitles(e2eSearch(t, api, bobTok, "brooklyn")); got != "Bob rental agreement Brooklyn" {
			t.Errorf("bob api: %s", got)
		}
		if got := e2eTitles(e2eSearch(t, api, aliceTok, "rental")); strings.Contains(got, "Bob") {
			t.Errorf("alice api: %s", got)
		}
		if got := e2eSearch(t, api, bobTok, "landlord plumbing cafe"); len(got) != 0 {
			t.Errorf("bob sees alice's notes and files: %+v", got)
		}
		if got := in.must("-as", "bob@example.com", "search", "rental", "brooklyn"); !strings.Contains(got, "Bob rental") || strings.Contains(got, "apartment") {
			t.Errorf("bob cli: %s", got)
		}
		if got := in.must("-as", "alice@example.com", "search", "rental", "-type", "note"); !strings.Contains(got, "apartment") || strings.Contains(got, "Bob") {
			t.Errorf("alice cli: %s", got)
		}
		role := in.role("bob@example.com")
		in.psql("archive", "ALTER ROLE \""+role+"\" LOGIN")
		defer in.psql("archive", "ALTER ROLE \""+role+"\" NOLOGIN")
		q := "SELECT (SELECT count(*) FROM note WHERE search @@ archive.search_query('rental landlord')) || ',' || " +
			"(SELECT count(*) FROM archive.file WHERE search @@ archive.search_query('landlord'))"
		if got := in.psql(role, q); got != "1,0" {
			t.Errorf("psql as bob: %s", got)
		}
	})

	t.Run("IndexesComeAndGoWithTheDDL", func(t *testing.T) {
		in := in.on(t)
		q := "SELECT count(*) FROM pg_indexes WHERE tablename = 'note' AND indexname = 'note_title'"
		if n := in.psql("archive", q); n != "1" {
			t.Fatalf("title index: %s", n)
		}
		if rs := decode[[]row](t, in.must("list", "note", "title=Weekend away")); len(rs) != 1 {
			t.Errorf("list by title: %+v", rs)
		}
		if m := decode[SchemaMigration](t, in.migrate("DROP INDEX note_title")); len(m.Statements) != 1 {
			t.Errorf("drop index: %+v", m)
		}
		if n := in.psql("archive", q); n != "0" {
			t.Errorf("title index kept after it left the DDL: %s", n)
		}
	})
}

func TestViewsSchemaDropsAndTokensOverHTTP(t *testing.T) {
	t.Parallel()
	in := newInst(t, e2eGoogle(t)...)
	in.must("user", "add", "alice@example.com")
	srv := in.serve()
	api := srv.url + "/api"
	in.must("user", "add", "bob@example.com")
	in.push(e2eSchema)
	in.groups("bob@example.com", "family")
	aliceTok := decode[Token](t, in.must("token", "create")).Secret
	bobTok := decode[Token](t, in.must("-as", "bob@example.com", "token", "create")).Secret

	t.Run("ViewsAreSQLWithTheirSettingsInComments", func(t *testing.T) {
		in := in.on(t)
		board := "CREATE VIEW people_board WITH (security_invoker = true) AS SELECT * FROM person;\n" +
			"COMMENT ON VIEW people_board IS 'Everyone you know\nby employer\nlayout: board\nby: employer\nicon: users';\n"
		in.migrate(board)
		var fam, solo row
		st, _ := call(t, "POST", api+"/person", aliceTok, map[string]any{"name": "Fam", "access": "family"}, &fam)
		wantStatus(t, st, 201, "person in family")
		st, _ = call(t, "POST", api+"/person", aliceTok, map[string]any{"name": "Solo", "access": "solo"}, &solo)
		wantStatus(t, st, 201, "person only owners see")

		var cat Catalog
		call(t, "GET", api+"/types", aliceTok, nil, &cat)
		v := cat.Tables["people_board"]
		if v == nil || v.Kind != "view" || v.Layout != "board" || v.By != "employer" || v.Icon != "users" || v.Description != "Everyone you know\nby employer" || v.col("name") == nil {
			t.Fatalf("view in the catalog: %+v", v)
		}
		if p := cat.Tables["person"]; p.Kind != "table" || p.Layout != "" {
			t.Errorf("table in the catalog: %+v", p)
		}
		var rows []row
		call(t, "GET", api+"/people_board", aliceTok, nil, &rows)
		if names(rows) != "Fam,Solo" {
			t.Errorf("alice through the view: %s", names(rows))
		}
		call(t, "GET", api+"/people_board", bobTok, nil, &rows)
		if names(rows) != "Fam" {
			t.Errorf("bob through the view sees %s, want only the family group's", names(rows))
		}
		st, _ = call(t, "POST", api+"/people_board", aliceTok, map[string]any{"name": "Nope"}, nil)
		wantStatus(t, st, 405, "create through a view")
		st, _ = call(t, "DELETE", api+"/people_board/"+id(fam), aliceTok, nil, nil)
		wantStatus(t, st, 405, "delete through a view")

		ddl := in.must("schema", "show")
		if !strings.Contains(ddl, "-- people_board\n") || !strings.Contains(ddl, "security_invoker=true") || !strings.Contains(ddl, "icon: users';") || strings.Contains(ddl, ";;") {
			t.Errorf("view DDL: %s", ddl)
		}

		view := "CREATE VIEW bad WITH (security_invoker = true) AS SELECT * FROM person;\nCOMMENT ON VIEW bad IS 'x\n"
		for _, bad := range []struct{ sql, want string }{
			{"CREATE VIEW bad AS SELECT * FROM person;", "security_invoker = true"},
			{view + "layout: board\nby: nope';", `no column "nope"`},
			{view + "colour: red';", "unknown key colour"},
			{view + "icon: not-an-icon';", "unknown icon"},
			{view + "layout: calendar';", "needs a by:"},
			{"CREATE VIEW bad WITH (security_invoker = true) AS SELECT name FROM person;", "want an id uuid column"},
		} {
			if e := in.migrateFails(bad.sql); !strings.Contains(e, bad.want) {
				t.Errorf("%s: %s", bad.sql, e)
			}
		}
		if e := in.migrateFails("COMMENT ON TABLE company IS 'A company\nlayout: board'"); !strings.Contains(e, "unknown key layout") {
			t.Errorf("layout on a table: %s", e)
		}

		code := e2eMigrate + `const made = await migrate("CREATE VIEW open_people WITH (security_invoker = true) AS SELECT * FROM person WHERE name <> 'Solo';\n" +
  "COMMENT ON VIEW open_people IS 'People but Solo\nlayout: board\nby: employer\nicon: users';");
const t1 = (await archive.schema()).find((t) => t.name === "open_people");
const rows1 = (await archive.list("open_people")).map((r) => r.name);
const changed = await migrate("CREATE OR REPLACE VIEW open_people WITH (security_invoker = true) AS SELECT * FROM person WHERE name = 'Solo';\n" +
  "COMMENT ON VIEW open_people IS 'Only Solo\nicon: user';");
const t2 = (await archive.schema()).find((t) => t.name === "open_people");
const rows2 = (await archive.list("open_people")).map((r) => r.name);
const dropped = await migrate("DROP VIEW open_people");
const gone = !(await archive.schema()).some((t) => t.name === "open_people");
return { made: !!made.revision, t1, rows1, changed: !!changed.revision, t2, rows2, dropped: !!dropped.revision, gone };`
		var res struct {
			Result struct {
				Made, Changed, Dropped, Gone bool
				T1, T2                       runTable
				Rows1, Rows2                 []string
			}
			Error string
		}
		st, _ = call(t, "POST", srv.url+"/api/run", aliceTok, map[string]string{"code": code}, &res)
		wantStatus(t, st, 200, "view lifecycle in code mode")
		r := res.Result
		if res.Error != "" || !r.Made || !r.Changed || !r.Dropped || !r.Gone {
			t.Fatalf("view lifecycle: %+v", res)
		}
		if r.T1.Kind != "view" || r.T1.Layout != "board" || r.T1.By != "employer" || r.T1.Icon != "users" || r.T1.Description != "People but Solo" || strings.Join(r.Rows1, ",") != "Fam" {
			t.Errorf("created view: %+v %v", r.T1, r.Rows1)
		}
		if r.T2.Layout != "table" || r.T2.By != "" || r.T2.Icon != "user" || r.T2.Description != "Only Solo" || strings.Join(r.Rows2, ",") != "Solo" {
			t.Errorf("changed view: %+v %v", r.T2, r.Rows2)
		}
		in.migrate("DROP VIEW people_board")
		if got := in.psql("archive", "SELECT count(*) FROM pg_views WHERE schemaname = 'public'"); got != "0" {
			t.Errorf("%s views left after dropping them", got)
		}
	})

	t.Run("DroppingATableTakesTheConfirmOfItsDryRun", func(t *testing.T) {
		st, _ := call(t, "POST", api+"/invoice", aliceTok, map[string]any{"number": "INV-1"}, nil)
		wantStatus(t, st, 201, "create invoice")
		var dry, m SchemaMigration
		st, _ = call(t, "POST", api+"/schema/migrate", aliceTok, map[string]any{"sql": "DROP TABLE invoice"}, &dry)
		wantStatus(t, st, 200, "the dry run")
		if dry.Applied || len(dry.Drops) != 1 || dry.Drops[0].Name != "invoice" || !strings.Contains(dry.Drops[0].Before, `CREATE TABLE "public"."invoice"`) {
			t.Errorf("the dry run of a drop: %+v", dry)
		}
		st, _ = call(t, "GET", api+"/invoice", aliceTok, nil, nil)
		wantStatus(t, st, 200, "invoice after the dry run")
		st, _ = call(t, "POST", api+"/schema/migrate", aliceTok, map[string]any{"sql": "DROP TABLE invoice", "confirm": strings.Repeat("0", 64)}, nil)
		wantStatus(t, st, 409, "the drop with a wrong confirm")
		st, _ = call(t, "POST", api+"/schema/migrate", aliceTok, map[string]any{"sql": "DROP TABLE invoice", "confirm": dry.Confirm}, &m)
		wantStatus(t, st, 200, "the drop")
		st, _ = call(t, "GET", api+"/invoice", aliceTok, nil, nil)
		wantStatus(t, st, 404, "list a dropped table")
	})

	t.Run("TokensActAsTheirOwnerAndManageMembersAndTokens", func(t *testing.T) {
		in := in.on(t)
		tf := decode[Token](t, in.must("token", "create", "-name", "Terraform"))
		if tf.Viewer || tf.Secret == "" {
			t.Fatalf("token: %+v", tf)
		}
		var view Token
		st, _ := call(t, "POST", srv.url+"/auth/tokens", tf.Secret, map[string]any{"name": "Agent", "viewer": true}, &view)
		wantStatus(t, st, 201, "a token makes a viewer-only token")
		st, _ = call(t, "POST", srv.url+"/auth/tokens", view.Secret, map[string]any{"name": "wider"}, nil)
		wantStatus(t, st, 403, "a viewer-only token makes a token")
		var bob []row
		call(t, "GET", api+"/member?email=bob@example.com", view.Secret, nil, &bob)
		st, _ = call(t, "PATCH", api+"/member/"+id(bob[0]), view.Secret, map[string]any{"role": "editor"}, nil)
		wantStatus(t, st, 403, "a viewer-only token changes a member")
		var m row
		st, _ = call(t, "PATCH", api+"/member/"+id(bob[0]), tf.Secret, map[string]any{"role": "editor"}, &m)
		if st != 200 || m["role"] != "editor" || fmt.Sprint(m["groups"]) != "[family]" {
			t.Errorf("a token changes a member: %d %v", st, m)
		}
		var ts []Token
		call(t, "GET", srv.url+"/auth/tokens", tf.Secret, nil, &ts)
		if !slices.ContainsFunc(ts, func(tk Token) bool { return tk.ID == tf.ID && !tk.Viewer && tk.Secret == "" }) ||
			!slices.ContainsFunc(ts, func(tk Token) bool { return tk.ID == view.ID && tk.Viewer }) {
			t.Errorf("tokens: %+v", ts)
		}
		st, _ = call(t, "DELETE", srv.url+"/auth/tokens/"+tf.ID.String(), tf.Secret, nil, nil)
		wantStatus(t, st, 204, "revoke the token")
		st, _ = call(t, "GET", srv.url+"/auth/me", tf.Secret, nil, nil)
		wantStatus(t, st, 401, "revoked token")
		if n := in.psql("archive", "SELECT count(*) FROM pg_roles WHERE starts_with(rolname, (SELECT id::text FROM archive.workspace ORDER BY created_at LIMIT 1) || '_t')"); n != "1" {
			t.Errorf("%s token roles, want the viewer-only one", n)
		}
	})

	t.Run("CatalogAndAccessFilters", func(t *testing.T) {
		var cat Catalog
		call(t, "GET", api+"/types", bobTok, nil, &cat)
		if cat.Tables["person"] == nil || cat.Tables["person"].col("employer").Ref != "public.company" || cat.Types["archive.money"] == nil || cat.Types["archive.money"].Kind != "c" {
			t.Errorf("catalog: %+v", cat.Tables["person"])
		}
		var fam, own row
		call(t, "POST", api+"/company", aliceTok, map[string]any{"name": "Family firm", "access": "family"}, &fam)
		call(t, "POST", api+"/company", aliceTok, map[string]any{"name": "Own firm"}, &own)
		var rs []row
		st, _ := call(t, "GET", api+"/company?access=family", aliceTok, nil, &rs)
		wantStatus(t, st, 200, "list one group's rows")
		if len(rs) != 1 || id(rs[0]) != id(fam) {
			t.Errorf("family companies: %+v", rs)
		}
		call(t, "GET", api+"/company", aliceTok, nil, &rs)
		if len(rs) != 2 {
			t.Errorf("all companies: %+v", rs)
		}
		_, ff := e2eUpload(t, api+"/files?access=family", aliceTok, "family.txt", []byte("family notes"))
		_, of := e2eUpload(t, api+"/files?access=solo", aliceTok, "own.txt", []byte("own notes"))
		if ff.Access == nil || *ff.Access != "family" || of.Access == nil || *of.Access != "solo" {
			t.Errorf("uploaded files: %+v %+v", ff, of)
		}
		var fs []File
		call(t, "GET", api+"/files", bobTok, nil, &fs)
		if len(fs) != 1 || fs[0].Name != "family.txt" {
			t.Errorf("bob's files: %+v", fs)
		}
		call(t, "GET", api+"/files", aliceTok, nil, &fs)
		if len(fs) != 2 {
			t.Errorf("all files: %+v", fs)
		}
	})

	t.Run("SignInMethodsAndGoogleInTheBrowser", func(t *testing.T) {
		var methods map[string]any
		st, _ := call(t, "GET", srv.url+"/auth/methods", "", nil, &methods)
		wantStatus(t, st, 200, "sign-in methods without a session")
		if fmt.Sprint(methods["providers"]) != "[map[id:google name:Google]]" || len(methods) != 1 {
			t.Errorf("methods: %+v", methods)
		}
		for _, m := range []string{"GET", "POST"} {
			if st, _ = call(t, m, srv.url+"/auth/magic?token=arc_x", "", map[string]any{"email": "bob@example.com"}, nil); st != 405 && st != 404 {
				t.Errorf("%s a sign-in link: %d", m, st)
			}
		}
		st, to, c := srv.signIn(t, "google", map[string]any{"sub": "bob-1", "email": "bob@example.com", "email_verified": true})
		if st != http.StatusSeeOther || !strings.HasPrefix(to, "http://localhost:") || !strings.HasSuffix(to, "/") || c == nil || c.Value == "" {
			t.Errorf("Google sign-in in a browser: %d %q %v", st, to, c)
		}
	})
}

const e2eHouse = `CREATE TABLE house (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    access     text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    address    text NOT NULL,
    price      archive.money,
    sold       boolean NOT NULL DEFAULT false,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(address)) STORED
);
CREATE INDEX house_search ON house USING gin (search);
ALTER TABLE house ENABLE ROW LEVEL SECURITY;
ALTER TABLE house FORCE ROW LEVEL SECURITY;
CREATE POLICY access ON house USING (archive.can(access)) WITH CHECK (archive.can(access));
COMMENT ON TABLE house IS 'A house you own or rent';
COMMENT ON COLUMN house.price IS 'Asking price';
`

type e2eBearer string

func (b e2eBearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+string(b))
	return http.DefaultTransport.RoundTrip(r)
}

func TestARunThatTimesOutStopsAndSavesNothing(t *testing.T) {
	t.Parallel()
	in := newInst(t)
	in.must("user", "add", "alice@example.com")
	in.push(map[string]string{"house.sql": e2eHouse})
	srv := in.serve()
	tok := decode[Token](t, in.must("token", "create")).Secret
	start := time.Now()
	var out RunOutput
	st, _ := call(t, "POST", srv.url+"/api/run", tok, map[string]string{"code": `await archive.create("house", { address: "Loop 1" }); while (true) {}`}, &out)
	if st != 200 || out.Error != "timed out after 30s; nothing was saved" || time.Since(start) > 45*time.Second {
		t.Errorf("timeout: %d %+v after %s", st, out, time.Since(start))
	}
	if n := in.must("list", "house"); strings.Contains(n, "Loop 1") {
		t.Errorf("the timed out run saved a row: %s", n)
	}
}

func TestCodeModeRunsTypeScriptInOneTransactionOverHTTPMCPAndCLI(t *testing.T) {
	t.Parallel()
	in := newInst(t)
	in.must("user", "add", "alice@example.com")
	srv := in.serve()
	api := srv.url + "/api"
	in.must("user", "add", "bob@example.com")
	tok := decode[Token](t, in.must("token", "create")).Secret
	bobTok := decode[Token](t, in.must("-as", "bob@example.com", "token", "create")).Secret
	run := func(t *testing.T, tok, code string) RunOutput {
		t.Helper()
		var out RunOutput
		st, _ := call(t, "POST", api+"/run", tok, map[string]string{"code": code}, &out)
		wantStatus(t, st, 200, "run")
		return out
	}
	ok := func(t *testing.T, code string) map[string]any {
		t.Helper()
		out := run(t, tok, code)
		if out.Error != "" {
			t.Fatalf("run: %s\nlogs: %v", out.Error, out.Logs)
		}
		return decode[map[string]any](t, string(out.Result))
	}
	count := func(t *testing.T, address string) string {
		t.Helper()
		return fmt.Sprint(ok(t, `return { n: (await archive.query("SELECT count(*)::int AS n FROM house WHERE address = $1", `+strconv.Quote(address)+`))[0].n }`)["n"])
	}
	_, note := e2eUpload(t, api+"/files", tok, "viewing.txt", []byte("Viewing on Friday at the canal house"))
	e2eDoneFile(t, api, tok, note.ID.String())

	t.Run("SchemaChangesInARunAreUndoneWhenTheRunFails", func(t *testing.T) {
		if out := run(t, tok, e2eMigrate+`await migrate("CREATE TABLE barn (label text NOT NULL)");
await archive.create("barn", { label: "hay" });
throw new Error("not today");`); !strings.Contains(out.Error, "not today") {
			t.Fatalf("rolled back run: %+v", out)
		}
		if gone := ok(t, `return { gone: !(await archive.schema()).some((t) => t.name === "barn") };`); gone["gone"] != true {
			t.Errorf("a rolled back run still made the table: %v", gone)
		}
		shed := e2eTable("shed", "    label text NOT NULL,\n    search tsvector GENERATED ALWAYS AS (archive.search_doc(label)) STORED")
		ok(t, e2eMigrate+`await migrate(`+strconv.Quote(shed)+`);
await archive.create("shed", { label: "tools" });
return {};`)
		if out := run(t, tok, e2eMigrate+`await migrate("DROP TABLE shed"); throw new Error("changed my mind");`); !strings.Contains(out.Error, "changed my mind") {
			t.Fatalf("rolled back run: %+v", out)
		}
		if kept := ok(t, `return { kept: (await archive.schema()).some((t) => t.name === "shed") };`); kept["kept"] != true {
			t.Errorf("a rolled back run still dropped the table: %v", kept)
		}
		got := ok(t, `const before = (await archive.list("shed")).length;
const plan = await archive.migrate("DROP TABLE shed");
const kept = (await archive.schema()).some((t) => t.name === "shed");
const m = await archive.migrate("DROP TABLE shed", { confirm: plan.confirm });
return { before, drops: plan.drops.map((d) => d.kind + " " + d.name), kept, applied: m.applied && !!m.revision, still: (await archive.schema()).some((t) => t.name === "shed") };`)
		if fmt.Sprintf("%v %v %v %v %v", got["before"], got["drops"], got["kept"], got["applied"], got["still"]) != "1 [table shed] true true false" {
			t.Fatalf("drop in a run: %v", got)
		}
		if gone := ok(t, `return { gone: !(await archive.schema()).some((t) => t.name === "shed") };`); gone["gone"] != true {
			t.Errorf("shed is still there after the run: %v", gone)
		}
	})

	t.Run("RenamingAFileKeepsItsContentAndNeedsAnEditor", func(t *testing.T) {
		var f File
		st, _ := call(t, "PATCH", api+"/files/"+note.ID.String(), tok, map[string]string{"name": "  dir/Viewing notes.txt "}, &f)
		wantStatus(t, st, 200, "rename")
		if f.Name != "Viewing notes.txt" || f.ID != note.ID || f.Hash != note.Hash {
			t.Errorf("renamed: %+v", f)
		}
		_, body := e2eGet(t, api+"/files/"+note.ID.String()+"/content", tok)
		if string(body) != "Viewing on Friday at the canal house" {
			t.Errorf("content after rename: %q", body)
		}
		st, _ = call(t, "PATCH", api+"/files/"+note.ID.String(), bobTok, map[string]string{"name": "mine.txt"}, nil)
		wantStatus(t, st, 403, "a viewer renames a file")
		st, _ = call(t, "PATCH", api+"/files/"+note.ID.String(), tok, map[string]string{"name": " "}, nil)
		wantStatus(t, st, 400, "empty name")
	})

	t.Run("MigrateTheSchema", func(t *testing.T) {
		got := ok(t, e2eMigrate+`const done = await migrate(`+strconv.Quote(e2eHouse)+`);
return { planned: done.statements.length, applied: !!done.revision, plan: typeof archive.plan, apply: typeof archive.apply,
  tables: (await archive.schema()).map(t => t.name + ":" + t.columns.filter(c => c.required).map(c => c.name).join("+")) };`)
		if fmt.Sprintf("%v %v %v", got["plan"], got["apply"], got["applied"]) != "undefined undefined true" || fmt.Sprint(got["planned"]) == "0" || fmt.Sprint(got["tables"]) != "[house:address]" {
			t.Errorf("migrate: %v", got)
		}
		if ddl := in.must("schema", "show"); !strings.Contains(ddl, "-- house\n") {
			t.Errorf("schema show lacks the migrated table: %s", ddl)
		}
	})

	t.Run("DocsCarryTheSDKAndAnInterfacePerTable", func(t *testing.T) {
		resp, body := e2eGet(t, api+"/run/docs", tok)
		docs := string(body)
		for _, want := range []string{"declare const archive", "query(sql: string, ...params: unknown[]): Promise<Row[]>", "interface Money { amount: number; currency: Currency }",
			"/** A house you own or rent. */\ninterface House extends Base {\n    /** Group that sees the row, like hr; null is the whole workspace. */\n    access?: string;\n    address: string;\n    /** Asking price. */\n    price?: Money;\n    sold?: boolean;\n}", "house: House;", "type Email = string;", "read(fileId: string): Promise<Model>;", "edit(fileId: string, edits: Edit[]): Promise<StoredFile>;"} {
			if resp.StatusCode != 200 || !strings.Contains(docs, want) {
				t.Errorf("docs lack %q:\n%s", want, docs)
			}
		}
		if cli := in.must("run", "-docs"); cli != docs {
			t.Errorf("archive run -docs differs from GET /api/run/docs")
		}
	})

	t.Run("CRUDSearchSQLAndFilesInOneScript", func(t *testing.T) {
		got := ok(t, `const a = await archive.create("house", { address: "12 Willow Street", price: "$950,000.00" });
const b = await archive.create("house", { address: "34 Maple Avenue" });
const listed = await archive.list("house", { where: { address: "34 Maple Avenue" } });
const priced = await archive.update("house", b.id, { price: { amount: 500000, currency: "USD" } });
const cleared = await archive.update("house", a.id, { price: null });
const hits = await archive.search("willow");
const [{ n }] = await archive.query("SELECT count(*)::int AS n FROM house WHERE price IS NOT NULL");
const files = await archive.files();
const file = await archive.file(files[0].id);
console.log("made", { a: a.address });
return { price: a.price, sold: b.sold, listed: listed.map(h => h.id === b.id), priced: priced.price?.amount, cleared: cleared.price,
  hit: hits.find(h => h.type === "house")?.data.address, n, text: file.text, members: (await archive.list("member")).length };`)
		want := map[string]string{"price": "map[amount:950000 currency:USD]", "sold": "false", "listed": "[true]", "priced": "500000", "cleared": "<nil>",
			"hit": "12 Willow Street", "n": "1", "text": "Viewing on Friday at the canal house", "members": "2"}
		for k, v := range want {
			if fmt.Sprint(got[k]) != v {
				t.Errorf("%s = %v, want %s", k, got[k], v)
			}
		}
		if out := run(t, bobTok, `return (await archive.list("house")).length + (await archive.query("SELECT * FROM house")).length`); string(out.Result) != "4" || out.Error != "" {
			t.Errorf("bob, a viewer, reads the houses: %+v", out)
		}
		if out := run(t, bobTok, `await archive.create("house", { address: "1 Bob Lane" })`); !strings.Contains(out.Error, "permission denied") {
			t.Errorf("bob, a viewer, adds a house: %+v", out)
		}
	})

	t.Run("ReadAndEditOfficeFilesInPlace", func(t *testing.T) {
		ids := map[string]string{}
		orig := map[string][]byte{}
		for _, name := range []string{"Letter to landlord.docx", "family/Monthly budget.xlsx", "Lisbon trip.pptx"} {
			b, err := os.ReadFile(filepath.Join("..", "demo", "files", name))
			if err != nil {
				t.Fatal(err)
			}
			_, f := e2eUpload(t, api+"/files", tok, filepath.Base(name), b)
			ids[filepath.Ext(name)], orig[filepath.Ext(name)] = f.ID.String(), b
		}
		got := ok(t, `const [doc, sheet, deck] = [`+strconv.Quote(ids[".docx"])+`, `+strconv.Quote(ids[".xlsx"])+`, `+strconv.Quote(ids[".pptx"])+`];
const before = (await archive.read(doc)).paragraphs.find(p => p.text.includes("window frame"));
await archive.edit(doc, [{ op: "replace_text", find: "window frame", replace: "window sill" }, { op: "insert_paragraph_after", para: before.id, text: "It got worse in October." }]);
await archive.edit(sheet, [{ op: "set_cell", cell: "B4", value: 500 }, { op: "set_formula", cell: "C5", formula: "B5*12" }]);
await archive.edit(deck, [{ op: "replace_shape_text", slide: 1, shape: 1, text: "Porto long weekend" }]);
let bad = "";
try { await archive.edit(doc, [{ op: "replace_text", find: "no such words", replace: "x" }]); } catch (e) { bad = e.message; }
const d = await archive.read(doc), x = await archive.read(sheet), k = await archive.read(deck);
return { para: d.paragraphs.map(p => p.text).filter(t => t.includes("window") || t.startsWith("It got")), cells: x.sheets[0].cells.filter(c => ["B4", "C5"].includes(c.ref)),
  title: k.slides[0].shapes[1].paragraphs[0].text, bad };`)
		want := map[string]string{"para": "[The bedroom window sill still lets in a draft. It got worse in October.]", "cells": "[map[ref:B4 value:500] map[formula:B5*12 ref:C5]]", "title": "Porto long weekend"}
		for k, v := range want {
			if fmt.Sprint(got[k]) != v {
				t.Errorf("%s = %v, want %s", k, got[k], v)
			}
		}
		if bad, _ := got["bad"].(string); !strings.Contains(bad, "no such words") {
			t.Errorf("a failing edit: %q", bad)
		}
		for ext, touched := range map[string][]string{".docx": {"word/document.xml"}, ".xlsx": {"xl/worksheets/sheet1.xml", "xl/workbook.xml"}, ".pptx": {"ppt/slides/slide1.xml"}} {
			_, edited := e2eGet(t, api+"/files/"+ids[ext]+"/content", tok)
			a, err := zip.NewReader(bytes.NewReader(orig[ext]), int64(len(orig[ext])))
			if err != nil {
				t.Fatal(err)
			}
			b, err := zip.NewReader(bytes.NewReader(edited), int64(len(edited)))
			if err != nil {
				t.Fatalf("%s: %v", ext, err)
			}
			raw := map[string][]byte{}
			for _, f := range b.File {
				r, _ := f.OpenRaw()
				raw[f.Name], _ = io.ReadAll(r)
			}
			for _, f := range a.File {
				r, _ := f.OpenRaw()
				was, _ := io.ReadAll(r)
				if !slices.Contains(touched, f.Name) && !bytes.Equal(was, raw[f.Name]) {
					t.Errorf("%s: %s changed", ext, f.Name)
				}
			}
		}
	})

	t.Run("CreateEmptyDocumentsOfEveryTypeAndFillThem", func(t *testing.T) {
		got := ok(t, `const [doc, sheet, deck] = await Promise.all(["Plan.docx", "Costs.xlsx", "Pitch.pptx"].map(n => archive.create(n)));
const blank = { doc: await archive.read(doc.id), sheet: await archive.read(sheet.id), deck: await archive.read(deck.id) };
await archive.edit(doc.id, [{ op: "insert_paragraph_after", para: blank.doc.paragraphs[0].id, text: "Kick-off", style: "Heading1" }]);
await archive.edit(sheet.id, [{ op: "set_cell", cell: "A1", value: "Rent" }, { op: "set_cell", cell: "B1", value: 1200 }, { op: "set_formula", cell: "B2", formula: "B1*12" }]);
await archive.edit(deck.id, [{ op: "replace_shape_text", slide: 1, shape: 0, text: "Series A" }, { op: "replace_shape_text", slide: 1, shape: 1, text: "October 2026" }]);
let bad = "";
try { await archive.create("Plan.txt"); } catch (e) { bad = e.message; }
const d = await archive.read(doc.id), x = await archive.read(sheet.id), k = await archive.read(deck.id);
return { names: [doc.name, sheet.name, deck.name].join(","), blankDoc: blank.doc.paragraphs.length, blankSheet: blank.sheet.sheets.map(s => s.name + ":" + s.cells.length).join(","),
  blankDeck: blank.deck.slides[0].shapes.length,
  doc: d.paragraphs.map(p => (p.style || "") + ":" + p.text).join("|"), sheet: x.sheets[0].cells.map(c => c.ref + "=" + (c.formula || c.value)).join(","),
  deck: k.slides[0].shapes.map(s => s.paragraphs.map(p => p.text).join("")).join("|"), bad };`)
		want := map[string]string{"names": "Plan.docx,Costs.xlsx,Pitch.pptx", "blankDoc": "1", "blankSheet": "Sheet1:0", "blankDeck": "2",
			"doc": ":|Heading1:Kick-off", "sheet": "A1=Rent,B1=1200,B2=B1*12", "deck": "Series A|October 2026"}
		for k, v := range want {
			if fmt.Sprint(got[k]) != v {
				t.Errorf("%s = %v, want %s", k, got[k], v)
			}
		}
		if bad, _ := got["bad"].(string); !strings.Contains(bad, "end the name in .docx, .xlsx or .pptx") {
			t.Errorf("creating Plan.txt: %q", bad)
		}
	})

	t.Run("ManyAgentsEditOneDocxAndOneXlsxAtOnceAndNoEditIsLost", func(t *testing.T) {
		const agents = 30
		upload := func(src, name string) string {
			b, err := os.ReadFile(filepath.Join("..", "demo", "files", src))
			if err != nil {
				t.Fatal(err)
			}
			_, f := e2eUpload(t, api+"/files", tok, name, b)
			return strconv.Quote(f.ID.String())
		}
		doc, sheet := upload("Letter to landlord.docx", "Shared letter.docx"), upload("family/Monthly budget.xlsx", "Shared budget.xlsx")
		before := ok(t, `return { n: (await archive.read(`+doc+`)).paragraphs.length }`)
		agent := func(code string) {
			var out RunOutput
			if st, _ := call(t, "POST", api+"/run", tok, map[string]string{"code": code}, &out); st != 200 || out.Error != "" {
				t.Errorf("agent run: %d %s", st, out.Error)
			}
		}
		start := time.Now()
		var wg sync.WaitGroup
		for i := range agents {
			wg.Go(func() {
				agent(`const p = (await archive.read(` + doc + `)).paragraphs.find(p => p.text.includes("window frame"));
await archive.edit(` + doc + `, [{ op: "insert_paragraph_after", para: p.id, text: "Agent ` + strconv.Itoa(i) + ` was here" }]);`)
			})
			wg.Go(func() {
				agent(`await archive.edit(` + sheet + `, [{ op: "set_cell", cell: "J` + strconv.Itoa(40+i) + `", value: "Agent ` + strconv.Itoa(i) + `" }]);`)
			})
		}
		wg.Wait()
		t.Logf("%d agents in %s", 2*agents, time.Since(start).Round(time.Millisecond))
		got := ok(t, `const d = await archive.read(`+doc+`), x = await archive.read(`+sheet+`);
return { n: d.paragraphs.length, agents: d.paragraphs.map(p => p.text).filter(t => t.startsWith("Agent ")).sort(),
  cells: x.sheets[0].cells.filter(c => c.ref.startsWith("J") && +c.ref.slice(1) >= 40).map(c => c.ref + "=" + c.value).sort() };`)
		var wantParas, wantCells []string
		for i := range agents {
			wantParas = append(wantParas, "Agent "+strconv.Itoa(i)+" was here")
			wantCells = append(wantCells, "J"+strconv.Itoa(40+i)+"=Agent "+strconv.Itoa(i))
		}
		slices.Sort(wantParas)
		slices.Sort(wantCells)
		if fmt.Sprint(got["agents"]) != fmt.Sprint(wantParas) {
			t.Errorf("docx paragraphs by agents:\n got %v\nwant %v", got["agents"], wantParas)
		}
		if fmt.Sprint(got["cells"]) != fmt.Sprint(wantCells) {
			t.Errorf("xlsx cells by agents:\n got %v\nwant %v", got["cells"], wantCells)
		}
		if n, _ := before["n"].(json.Number).Int64(); fmt.Sprint(got["n"]) != fmt.Sprint(n+agents) {
			t.Errorf("paragraphs: %v, want %d", got["n"], n+agents)
		}
	})

	t.Run("AThrowingScriptLeavesNoRowsBehind", func(t *testing.T) {
		out := run(t, tok, `await archive.create("house", { address: "Ghost 1" });
console.log("created");
throw new Error("changed my mind");`)
		if !strings.Contains(out.Error, "changed my mind") || string(out.Result) != "null" || !slices.Equal(out.Logs, []string{"created"}) {
			t.Errorf("throw: %+v", out)
		}
		if n := count(t, "Ghost 1"); n != "0" {
			t.Errorf("the thrown run saved %s rows", n)
		}
	})

	t.Run("ValidationErrorsNameEveryField", func(t *testing.T) {
		out := run(t, tok, `await archive.create("house", { price: "lots", bogus: 1 });`)
		for _, want := range []string{"invalid house", "address: required", "bogus: unknown field", "price: "} {
			if !strings.Contains(out.Error, want) {
				t.Errorf("error %q lacks %q", out.Error, want)
			}
		}
		got := ok(t, `try { await archive.create("house", { price: "lots" }); } catch (e) { return { code: e.code, paths: e.errors.map(x => x.path) }; }`)
		if fmt.Sprint(got["code"], " ", got["paths"]) != "invalid [address price]" {
			t.Errorf("caught validation error: %v", got)
		}
		if out := run(t, tok, `await archive.query("SELECT nope FROM house")`); !strings.Contains(out.Error, `column "nope" does not exist (SQLSTATE 42703)`) {
			t.Errorf("sql error: %q", out.Error)
		}
	})

	t.Run("BadRequestsAndTheCLI", func(t *testing.T) {
		st, _ := call(t, "POST", api+"/run", tok, map[string]string{}, nil)
		wantStatus(t, st, 400, "run without code")
		st, _ = call(t, "POST", api+"/run", "", map[string]string{"code": "return 1"}, nil)
		wantStatus(t, st, 401, "run without a token")
		if out := decode[RunOutput](t, in.must("run", "-")); out.Error != "" || string(out.Result) != "null" {
			t.Errorf("empty script from stdin: %+v", out)
		}
		stdout, _, err := in.run(`return (await archive.list("house")).map(h => h.address).sort()`, "run", "-")
		if out := decode[RunOutput](t, stdout); err != nil || fmt.Sprint(decode[[]string](t, string(out.Result))) != "[12 Willow Street 34 Maple Avenue]" {
			t.Errorf("archive run -: %v %s", err, stdout)
		}
	})

	t.Run("MCPHasExactlyOneRunTool", func(t *testing.T) {
		ctx := t.Context()
		cl := mcp.NewClient(&mcp.Implementation{Name: "e2e", Version: "1"}, nil)
		sess, err := cl.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: srv.url + "/mcp", HTTPClient: &http.Client{Transport: e2eBearer(tok)}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer sess.Close()
		tools, err := sess.ListTools(ctx, nil)
		if err != nil || len(tools.Tools) != 1 || tools.Tools[0].Name != "run" || !strings.Contains(tools.Tools[0].Description, "interface House extends Base") {
			t.Fatalf("tools: %v %+v", err, tools)
		}
		res, err := sess.CallTool(ctx, &mcp.CallToolParams{Name: "run", Arguments: map[string]any{"code": `console.log("hi"); return (await archive.list("house", { where: { address: "34 Maple Avenue" } }))[0].price;`}})
		if err != nil || res.IsError || len(res.Content) != 1 {
			t.Fatalf("call: %v %+v", err, res)
		}
		out := decode[RunOutput](t, res.Content[0].(*mcp.TextContent).Text)
		if out.Error != "" || string(out.Result) != `{"amount":500000,"currency":"USD"}` || !slices.Equal(out.Logs, []string{"hi"}) {
			t.Errorf("mcp run: %+v", out)
		}
		res, err = sess.CallTool(ctx, &mcp.CallToolParams{Name: "run", Arguments: map[string]any{"code": `return archive.get("house", "nope")`}})
		if err != nil || !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, `not a uuid`) {
			t.Errorf("mcp error: %v %+v", err, res)
		}
	})

	t.Run("RenamesKeepTheData", func(t *testing.T) {
		got := ok(t, e2eMigrate+`const m = await migrate("ALTER TABLE house RENAME COLUMN price TO asking_price");
return { ran: m.statements, price: (await archive.query("SELECT (asking_price).amount FROM house WHERE asking_price IS NOT NULL"))[0].amount };`)
		if fmt.Sprint(got["ran"]) != `[ALTER TABLE house RENAME COLUMN price TO asking_price]` || fmt.Sprint(got["price"]) != "500000" {
			t.Errorf("rename: %v", got)
		}
		dry := ok(t, `const m = await archive.migrate("ALTER TABLE house RENAME COLUMN asking_price TO list_price");
return { changes: m.changes, drops: m.drops };`)
		cols := ok(t, `return { cols: (await archive.query("SELECT column_name FROM information_schema.columns WHERE table_name = 'house' AND column_name LIKE '%price'")).map(c => c.column_name) }`)["cols"]
		if fmt.Sprint(dry) != "map[changes:[map[action:alter from:house.asking_price kind:column name:house.list_price]] drops:[]]" || fmt.Sprint(cols) != "[asking_price]" {
			t.Errorf("a dry run of a rename shows it and keeps nothing: %v %v", dry, cols)
		}
	})
}

func e2eChat(ctx context.Context, t *testing.T, url, tok string, body map[string]any, each func(ev map[string]any) bool) (int, []map[string]any) {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, "POST", url+"/chat", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var evs []map[string]any
	if resp.StatusCode != 200 {
		return resp.StatusCode, nil
	}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	for sc.Scan() {
		ev := decode[map[string]any](t, sc.Text())
		evs = append(evs, ev)
		if each != nil && !each(ev) {
			break
		}
	}
	return resp.StatusCode, evs
}

func e2eKinds(evs []map[string]any) string {
	var out []string
	for _, ev := range evs {
		if k := fmt.Sprint(ev["type"]); len(out) == 0 || out[len(out)-1] != k {
			out = append(out, k)
		}
	}
	return strings.Join(out, ",")
}

func TestChatStreamsTheAgentLoopThroughRunAndKeepsEachUsersHistory(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var model []map[string]any
	var headers []http.Header
	hung := make(chan struct{})
	hangDone := false
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		mu.Lock()
		model, headers = append(model, req), append(headers, r.Header.Clone())
		mu.Unlock()
		msgs := req["messages"].([]any)
		first := fmt.Sprint(msgs[0].(map[string]any)["content"].([]any)[0].(map[string]any)["text"])
		last := msgs[len(msgs)-1].(map[string]any)
		answered := false
		for _, b := range last["content"].([]any) {
			answered = answered || b.(map[string]any)["type"] == "tool_result"
		}
		ev := func(s string) {
			fmt.Fprintf(w, "event: x\ndata: %s\n\n", s)
			w.(http.Flusher).Flush()
		}
		text := func(parts ...string) {
			ev(`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)
			for _, p := range parts {
				b, _ := json.Marshal(p)
				ev(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":` + string(b) + `}}`)
			}
			ev(`{"type":"content_block_stop","index":0}`)
			ev(`{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`)
		}
		tool := func(code ...string) {
			ev(`{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}`)
			ev(`{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"sig"}}`)
			ev(`{"type":"content_block_stop","index":0}`)
			ev(`{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"t` + strconv.Itoa(len(msgs)) + `","name":"run","input":{}}}`)
			for _, c := range code {
				b, _ := json.Marshal(c)
				ev(`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":` + string(b) + `}}`)
			}
			ev(`{"type":"content_block_stop","index":1}`)
			ev(`{"type":"message_delta","delta":{"stop_reason":"tool_use"}}`)
		}
		switch {
		case first == "fail":
			w.WriteHeader(400)
			io.WriteString(w, `{"type":"error","error":{"type":"invalid_request_error","message":"prompt is too long"}}`)
		case first == "hang" && !hangDone:
			hangDone = true
			ev(`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)
			ev(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Let me "}}`)
			<-r.Context().Done()
			close(hung)
		case first == "boom" && len(msgs) == 1:
			tool(`{"code": "throw new Error(\"boom\")"}`)
		case first == "how many members?" && len(msgs) == 1:
			tool(`{"code": "return (await archive.list(\"mem`, `ber\")).length * 7"}`)
		case answered:
			text("You have ", "two members.")
		default:
			text("Hello again.")
		}
	}))
	defer fake.Close()
	in := newInst(t, "ANTHROPIC_API_KEY=test", "ANTHROPIC_BASE_URL="+fake.URL)
	in.must("user", "add", "alice@example.com")
	srv := in.serve()
	api := srv.url + "/api"
	in.must("user", "add", "bob@example.com")
	tok := decode[Token](t, in.must("token", "create")).Secret
	bobTok := decode[Token](t, in.must("-as", "bob@example.com", "token", "create")).Secret
	ctx := context.Background()
	var chatID string

	t.Run("StreamsTextCodeAndRunResultsThenSavesTheTurn", func(t *testing.T) {
		st, evs := e2eChat(ctx, t, api, tok, map[string]any{"text": "how many members?"}, nil)
		wantStatus(t, st, 200, "chat")
		chatID = fmt.Sprint(evs[0]["chat"].(map[string]any)["id"])
		if got := e2eKinds(evs); got != "chat,thinking,tool,code,run,result,text,done" {
			t.Fatalf("events: %s\n%v", got, evs)
		}
		var codes, text []string
		for _, ev := range evs {
			switch ev["type"] {
			case "code":
				codes = append(codes, fmt.Sprint(ev["code"]))
			case "text":
				text = append(text, fmt.Sprint(ev["text"]))
			case "result":
				if fmt.Sprint(ev["result"]) != "14" || ev["error"] != "" || ev["id"] != "t1" {
					t.Errorf("result event: %v", ev)
				}
			}
		}
		if strings.Join(codes, "|") != `return (await archive.list("mem|return (await archive.list("member")).length * 7` || strings.Join(text, "") != "You have two members." {
			t.Errorf("streamed code %q, text %q", codes, text)
		}
		mu.Lock()
		first, h := model[len(model)-2], headers[len(headers)-2]
		second := model[len(model)-1]
		mu.Unlock()
		tool := first["tools"].([]any)[0].(map[string]any)
		if first["model"] != "claude-haiku-5-5" || len(first["tools"].([]any)) != 1 || tool["name"] != "run" || tool["eager_input_streaming"] != true ||
			!strings.Contains(fmt.Sprint(tool["description"]), "declare const archive") || h.Get("anthropic-version") != "2026-10-21" || h.Get("x-api-key") != "test" {
			t.Errorf("model request: model %v, tool %v %v, version %q", first["model"], tool["name"], tool["eager_input_streaming"], h.Get("anthropic-version"))
		}
		if sys := fmt.Sprint(first["system"]); !strings.Contains(sys, "Today is") || !strings.Contains(sys, "alice@example.com") {
			t.Errorf("system: %s", sys)
		}
		if s, _ := json.Marshal(second["messages"]); len(second["messages"].([]any)) != 3 || !strings.Contains(string(s), `"signature":"sig"`) || !strings.Contains(string(s), `\"result\":14`) {
			t.Errorf("second model request: %s", s)
		}
	})

	t.Run("HistoryIsListedOpenedRenamedAndOnlyYours", func(t *testing.T) {
		var list []Chat
		call(t, "GET", api+"/chats", tok, nil, &list)
		if len(list) != 1 || list[0].ID.String() != chatID || list[0].Title != "how many members?" || list[0].Messages != nil {
			t.Fatalf("chats: %+v", list)
		}
		var c struct {
			Messages []map[string]any `json:"messages"`
		}
		call(t, "GET", api+"/chats/"+chatID, tok, nil, &c)
		roles := []string{}
		for _, m := range c.Messages {
			roles = append(roles, fmt.Sprint(m["role"]))
		}
		if strings.Join(roles, ",") != "user,assistant,user,assistant" {
			t.Errorf("saved roles: %v", roles)
		}
		var renamed Chat
		st, _ := call(t, "PATCH", api+"/chats/"+chatID, tok, map[string]string{"title": "  Counting   members "}, &renamed)
		wantStatus(t, st, 200, "rename")
		if renamed.Title != "Counting members" {
			t.Errorf("renamed: %+v", renamed)
		}
		var bobs []Chat
		call(t, "GET", api+"/chats", bobTok, nil, &bobs)
		st, _ = call(t, "GET", api+"/chats/"+chatID, bobTok, nil, nil)
		wantStatus(t, st, 404, "bob opens alice's chat")
		st, _ = call(t, "DELETE", api+"/chats/"+chatID, bobTok, nil, nil)
		wantStatus(t, st, 404, "bob deletes alice's chat")
		st, _ = e2eChat(ctx, t, api, bobTok, map[string]any{"chat": chatID, "text": "hi"}, nil)
		wantStatus(t, st, 404, "bob writes in alice's chat")
		if len(bobs) != 0 {
			t.Errorf("bob lists %+v", bobs)
		}
		st, evs := e2eChat(ctx, t, api, tok, map[string]any{"chat": chatID, "text": "thanks"}, nil)
		if st != 200 || e2eKinds(evs) != "chat,text,done" || fmt.Sprint(evs[0]["chat"].(map[string]any)["title"]) != "Counting members" {
			t.Errorf("follow-up: %d %v", st, evs)
		}
	})

	t.Run("AFailingRunIsReportedToTheModelAndTheUser", func(t *testing.T) {
		_, evs := e2eChat(ctx, t, api, tok, map[string]any{"text": "boom"}, nil)
		for _, ev := range evs {
			if ev["type"] == "result" && (!strings.Contains(fmt.Sprint(ev["error"]), "boom") || ev["result"] != nil) {
				t.Errorf("result: %v", ev)
			}
		}
		mu.Lock()
		last, _ := json.Marshal(model[len(model)-1]["messages"])
		mu.Unlock()
		if e2eKinds(evs) != "chat,thinking,tool,code,run,result,text,done" || !strings.Contains(string(last), `"is_error":true`) {
			t.Errorf("events %s, model saw %s", e2eKinds(evs), last)
		}
	})

	t.Run("ModelErrorsStreamAndRetryContinuesTheSavedChat", func(t *testing.T) {
		_, evs := e2eChat(ctx, t, api, tok, map[string]any{"text": "fail"}, nil)
		if e2eKinds(evs) != "chat,error" || !strings.Contains(fmt.Sprint(evs[1]["error"]), "prompt is too long") {
			t.Fatalf("error events: %v", evs)
		}
		id := fmt.Sprint(evs[0]["chat"].(map[string]any)["id"])
		_, evs = e2eChat(ctx, t, api, tok, map[string]any{"chat": id}, nil)
		if e2eKinds(evs) != "chat,error" {
			t.Errorf("retry: %v", evs)
		}
		st, _ := e2eChat(ctx, t, api, tok, map[string]any{}, nil)
		wantStatus(t, st, 400, "chat without text")
	})

	t.Run("CancellingStopsTheModelCallAndKeepsTheQuestion", func(t *testing.T) {
		cctx, cancel := context.WithCancel(ctx)
		var id string
		e2eChat(cctx, t, api, tok, map[string]any{"text": "hang"}, func(ev map[string]any) bool {
			if ev["type"] == "chat" {
				id = fmt.Sprint(ev["chat"].(map[string]any)["id"])
			}
			if ev["type"] == "text" {
				cancel()
				return false
			}
			return true
		})
		cancel()
		select {
		case <-hung:
		case <-time.After(15 * time.Second):
			t.Fatal("the model call kept running after the client went away")
		}
		var c struct {
			Messages []map[string]any `json:"messages"`
		}
		call(t, "GET", api+"/chats/"+id, tok, nil, &c)
		if len(c.Messages) != 1 || c.Messages[0]["role"] != "user" {
			t.Errorf("saved after cancel: %v", c.Messages)
		}
		_, evs := e2eChat(ctx, t, api, tok, map[string]any{"chat": id}, nil)
		if e2eKinds(evs) != "chat,text,done" {
			t.Errorf("continue after cancel: %v", evs)
		}
		st, _ := call(t, "DELETE", api+"/chats/"+id, tok, nil, nil)
		wantStatus(t, st, 204, "delete")
		st, _ = call(t, "GET", api+"/chats/"+id, tok, nil, nil)
		wantStatus(t, st, 404, "deleted chat")
	})
}

func TestWorkflowsDeployTestStartAndFireOnTheirOwn(t *testing.T) {
	t.Parallel()
	echo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/plain" {
			io.WriteString(w, "hello")
			return
		}
		b, _ := io.ReadAll(r.Body)
		w.Header().Set("X-Reply", "done")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"method": r.Method, "body": string(b), "type": r.Header.Get("Content-Type"), "test": r.Header.Get("X-Test"), "x": r.URL.Query().Get("x")})
	}))
	defer echo.Close()
	in := newInst(t)
	in.must("user", "add", "alice@example.com")
	in.push(map[string]string{"house.sql": e2eHouse})
	srv := in.serve()
	api := srv.url + "/api"
	in.must("user", "add", "bob@example.com")
	tok := decode[Token](t, in.must("token", "create")).Secret
	bobTok := decode[Token](t, in.must("-as", "bob@example.com", "token", "create")).Secret
	run := func(t *testing.T, tok, code string) map[string]any {
		t.Helper()
		var out RunOutput
		st, _ := call(t, "POST", api+"/run", tok, map[string]string{"code": code}, &out)
		wantStatus(t, st, 200, "run")
		if out.Error != "" {
			t.Fatalf("run: %s\nlogs: %v", out.Error, out.Logs)
		}
		return decode[map[string]any](t, "{\"v\":"+string(out.Result)+"}")
	}
	js := func(v any) string { b, _ := json.Marshal(v); return string(b) }
	runs := func(t *testing.T, name string) []WorkflowRun {
		t.Helper()
		var rs []WorkflowRun
		st, _ := call(t, "GET", api+"/workflows/"+name+"/runs?limit=100", tok, nil, &rs)
		wantStatus(t, st, 200, "runs")
		return rs
	}
	waitRuns := func(t *testing.T, name string, n int) []WorkflowRun {
		t.Helper()
		for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); time.Sleep(200 * time.Millisecond) {
			if rs := runs(t, name); len(rs) >= n {
				return rs
			}
		}
		t.Fatalf("%s did not run %d times: %v", name, n, runs(t, name))
		return nil
	}
	base := map[string]string{
		"hook.ts":    "// on: webhook\nreturn { got: input.body, method: input.method, x: input.query.x ?? null, type: input.headers['content-type'] ?? null };",
		"nightly.ts": "// on: cron CRON_TZ=America/New_York 0 9 * * 1-5\nreturn 1;",
		"manual.ts":  "await archive.create('house', { address: input?.address ?? 'Manual 1' });\nconsole.log('made', input?.address);\nreturn 'made';",
		"boom.ts":    "// on: webhook\nthrow new Error('bad input: ' + JSON.stringify(input.body));",
	}
	var hookURL string

	t.Run("DeployReturnsEveryErrorAndSavesNothingThenSavesAndKeepsWebhookURLs", func(t *testing.T) {
		got := run(t, tok, `const bad = await archive.deploy({
  "broken.ts": "const x = ;",
  "late.ts": "// on: cron 0 9 * *",
  "where.ts": "// on: shed\nreturn 1",
  "Bad Name.ts": "return 1",
  "second.ts": "return 1\n// on: webhook",
  "fine.ts": "return 1",
});
return { bad, after: await archive.workflows() };`)["v"].(map[string]any)
		bad := got["bad"].(map[string]any)
		errs := map[string]string{}
		for _, e := range bad["errors"].([]any) {
			m := e.(map[string]any)
			errs[fmt.Sprint(m["file"])] = fmt.Sprint(m["message"])
		}
		for file, want := range map[string]string{"broken.ts": "compile error: line 1:11", "late.ts": "expected exactly 5 fields", "where.ts": "the tables are house",
			"Bad Name.ts": "ending in .ts", "second.ts": "line 2: the // on: line must be the very first line"} {
			if !strings.Contains(errs[file], want) {
				t.Errorf("%s: %q, want %q", file, errs[file], want)
			}
		}
		if bad["deployed"] != false || len(errs) != 5 || fmt.Sprint(got["after"]) != "map[]" {
			t.Errorf("a bad deploy: %v", got)
		}
		done := run(t, tok, `return await archive.deploy(`+js(with(base, "temp.ts", "return 2"))+`);`)["v"].(map[string]any)
		ws := map[string]map[string]any{}
		for _, w := range done["workflows"].([]any) {
			ws[fmt.Sprint(w.(map[string]any)["name"])] = w.(map[string]any)
		}
		hookURL = fmt.Sprint(ws["hook.ts"]["url"])
		if done["deployed"] != true || len(ws) != 5 || !strings.Contains(hookURL, srv.url[strings.LastIndex(srv.url, ":"):]+"/hooks/run/") || len(hookURL) < 40 {
			t.Fatalf("deploy: %v", done)
		}
		if ws["nightly.ts"]["next"] == nil || ws["nightly.ts"]["on"] != "cron CRON_TZ=America/New_York 0 9 * * 1-5" || ws["manual.ts"]["on"] != "" || ws["manual.ts"]["url"] != nil || ws["manual.ts"]["next"] != nil {
			t.Errorf("deployed workflows: %v", ws)
		}
		again := run(t, tok, `const files = await archive.workflows();
delete files["temp.ts"];
const done = await archive.deploy(files);
return { names: Object.keys(await archive.workflows()).join(","), url: done.workflows.find((w) => w.name === "hook.ts").url };`)["v"].(map[string]any)
		if again["names"] != "boom.ts,hook.ts,manual.ts,nightly.ts" || again["url"] != hookURL {
			t.Errorf("redeploy without temp.ts: %v, want the same url %s", again, hookURL)
		}
	})

	t.Run("TestRollsBackItsWritesAndReturnsLogsResultsAndErrors", func(t *testing.T) {
		got := run(t, tok, `const saved = await archive.test("manual", { address: "Test 1" });
const draft = await archive.test("console.log('hi', { n: 1 }); await archive.create('house', { address: 'Draft 1' }); return input.n * 2;", { n: 21 });
const broken = await archive.test("const = 1");
const thrown = await archive.test("await archive.create('house', { address: 'Thrown 1' }); throw new Error('nope')");
const badOn = await archive.test("// on: cron every day\nreturn 1");
const missing = await archive.test("nosuch").then(() => "", (e) => e.code + ": " + e.message);
const [{ n }] = await archive.query("SELECT count(*)::int AS n FROM house WHERE address IN ('Test 1', 'Draft 1', 'Thrown 1')");
return { saved, draft, broken, thrown, badOn, missing, n };`)["v"].(map[string]any)
		want := map[string]string{
			"saved":   "map[error: logs:[made Test 1] result:made]",
			"draft":   "map[error: logs:[hi {\"n\":1}] result:42]",
			"missing": "not_found: workflow nosuch.ts not found; yours are boom.ts, hook.ts, manual.ts, nightly.ts",
			"n":       "0",
		}
		for k, v := range want {
			if fmt.Sprint(got[k]) != v {
				t.Errorf("%s = %v, want %s", k, got[k], v)
			}
		}
		for k, v := range map[string]string{"broken": "compile error: line 1:7", "thrown": "nope", "badOn": "on: cron every day"} {
			if e := fmt.Sprint(got[k].(map[string]any)["error"]); !strings.Contains(e, v) {
				t.Errorf("%s error %q, want %q", k, e, v)
			}
		}
		if rs := runs(t, "manual"); len(rs) != 0 {
			t.Errorf("test recorded runs: %v", rs)
		}
	})

	t.Run("StartRunsForRealAndRecordsTheRun", func(t *testing.T) {
		got := run(t, tok, `const out = await archive.start("manual.ts", { address: "Started 1" });
const failed = await archive.start("boom", { body: { x: 1 } });
return { out, failed: failed.error };`)["v"].(map[string]any)
		if fmt.Sprint(got["out"]) != "map[error: logs:[made Started 1] result:made]" || got["failed"] != "bad input: {\"x\":1}\n  at: throw new Error(\"bad input: \" + JSON.stringify(input.body));" {
			t.Errorf("start: %v", got)
		}
		later := run(t, tok, `return { runs: await archive.runs("manual"), n: (await archive.list("house", { where: { address: "Started 1" } })).length, boom: (await archive.runs("boom")).map((r) => r.trigger + ":" + r.error.split("\n")[0]) };`)["v"].(map[string]any)
		rs := later["runs"].([]any)
		if len(rs) != 1 || later["n"] != json.Number("1") || fmt.Sprint(later["boom"]) != `[start:bad input: {"x":1}]` {
			t.Fatalf("after start: %v", later)
		}
		r := rs[0].(map[string]any)
		if r["trigger"] != "start" || r["result"] != "made" || r["error"] != "" || fmt.Sprint(r["logs"]) != "[made Started 1]" || r["at"] == nil || r["ms"] == nil {
			t.Errorf("recorded run: %v", r)
		}
	})

	t.Run("FetchCallsOtherServicesSynchronously", func(t *testing.T) {
		got := run(t, tok, `const res = await fetch(`+js(echo.URL+"/echo?x=1")+`, { method: "POST", headers: { "X-Test": "yes" }, body: { a: 1 } });
const sent = await res.json();
const text = await (await fetch(`+js(echo.URL+"/plain")+`)).text();
const notJSON = await (await fetch(`+js(echo.URL+"/plain")+`)).json().then(() => "", (e) => e.message);
const down = await fetch("http://127.0.0.1:1/").then(() => "", (e) => e.message);
const metadata = await fetch("http://169.254.169.254/computeMetadata/v1/", { headers: { "Metadata-Flavor": "Google" } }).then(() => "", (e) => e.message);
return { ok: res.ok, status: res.status, statusText: res.statusText, reply: res.headers.get("X-Reply"), raw: res.headers["x-reply"], sent, text, notJSON, down, metadata };`)["v"].(map[string]any)
		want := map[string]string{"ok": "true", "status": "201", "statusText": "Created", "reply": "done", "raw": "done", "text": "hello",
			"sent": `map[body:{"a":1} method:POST test:yes type:application/json x:1]`}
		for k, v := range want {
			if fmt.Sprint(got[k]) != v {
				t.Errorf("%s = %v, want %s", k, got[k], v)
			}
		}
		if !strings.Contains(fmt.Sprint(got["notJSON"]), `is not JSON: "hello"`) || !strings.Contains(fmt.Sprint(got["down"]), "connection refused") {
			t.Errorf("fetch errors: %v", got)
		}
		if !strings.Contains(fmt.Sprint(got["metadata"]), "169.254.169.254 is an internal address, which scripts cannot reach") {
			t.Errorf("fetch reached the cloud metadata address: %v", got["metadata"])
		}
	})

	t.Run("CronAndTableWorkflowsFireOnTheirOwnAndIgnoreTheirOwnWrites", func(t *testing.T) {
		files := with(base, "tick.ts", "// on: cron @every 1s\nreturn 'tick';",
			"sold.ts", "// on: house\nfor (const r of input.rows) await archive.update('house', r.id, { sold: true });\nreturn input.rows.map((r) => r.address).join(',');")
		done := run(t, tok, `const d = await archive.deploy(`+js(files)+`); return d.workflows.find((w) => w.name === "hook.ts").url;`)["v"]
		if done != hookURL {
			t.Errorf("redeploy changed the webhook url: %v, want %s", done, hookURL)
		}
		if rs := waitRuns(t, "tick", 2); rs[0].Trigger != "cron" || string(rs[0].Result) != `"tick"` || rs[0].Error != "" || !rs[0].At.After(rs[1].At) {
			t.Errorf("cron runs: %+v", rs)
		}
		run(t, tok, `await archive.create("house", { address: "Canal 1" }); await archive.create("house", { address: "Canal 2" }); return 0;`)
		rs := waitRuns(t, "sold", 1)
		if rs[0].Trigger != "table" || string(rs[0].Result) != `"Canal 1,Canal 2"` || rs[0].Error != "" {
			t.Fatalf("table run: %+v", rs)
		}
		if sold := run(t, tok, `return (await archive.list("house")).filter((h) => h.sold).map((h) => h.address).join(",");`)["v"]; sold != "Canal 1,Canal 2" {
			t.Errorf("sold houses: %v", sold)
		}
		time.Sleep(3 * time.Second)
		if rs := runs(t, "sold"); len(rs) != 1 {
			t.Fatalf("the workflow's own updates triggered it again: %+v", rs)
		}
		run(t, tok, `const [h] = await archive.list("house", { where: { address: "Canal 2" } }); await archive.update("house", h.id, { sold: false }); return 0;`)
		if rs := waitRuns(t, "sold", 2); string(rs[0].Result) != `"Canal 2"` {
			t.Errorf("an update by someone else: %+v", rs)
		}
		run(t, tok, `await archive.deploy(`+js(with(files, "tick.ts", "", "sold.ts", ""))+`); return 0;`)
	})

	t.Run("WebhookPostRunsTheWorkflowAndReturnsItsResult", func(t *testing.T) {
		post := func(url, ctype, body string) (int, string) {
			resp, err := http.Post(url, ctype, strings.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			b, _ := io.ReadAll(resp.Body)
			return resp.StatusCode, strings.TrimSpace(string(b))
		}
		if st, body := post(hookURL+"?x=7", "application/json", `{"a": 1}`); st != 200 || body != `{"got":{"a":1},"method":"POST","x":"7","type":"application/json"}` {
			t.Errorf("json webhook: %d %s", st, body)
		}
		if st, body := post(hookURL, "text/plain", "just text"); st != 200 || body != `{"got":"just text","method":"POST","x":null,"type":"text/plain"}` {
			t.Errorf("text webhook: %d %s", st, body)
		}
		boom := run(t, tok, `return (await archive.deploy(await archive.workflows())).workflows.find((w) => w.name === "boom.ts").url;`)["v"].(string)
		if st, body := post(boom, "application/json", `[1]`); st != 500 || !strings.Contains(body, `"error":"bad input: [1]`) {
			t.Errorf("failing webhook: %d %s", st, body)
		}
		if st, _ := post(srv.url+"/hooks/run/"+strings.Repeat("x", 32), "application/json", "{}"); st != 404 {
			t.Errorf("unknown webhook token: %d", st)
		}
		if resp, err := http.Get(hookURL); err != nil || resp.StatusCode != 405 {
			t.Errorf("GET on a webhook: %v %v", resp, err)
		}
		if rs := runs(t, "hook"); len(rs) != 2 || rs[0].Trigger != "webhook" || rs[1].Error != "" {
			t.Errorf("webhook runs: %+v", rs)
		}
	})

	t.Run("APIListsRunsAndRunsWorkflowsAndShowsOnlyYours", func(t *testing.T) {
		var rec WorkflowRun
		st, _ := call(t, "POST", api+"/workflows/manual/run", tok, nil, &rec)
		if wantStatus(t, st, 200, "run"); rec.Trigger != "run" || string(rec.Result) != `"made"` || rec.Error != "" {
			t.Errorf("run from the API: %+v", rec)
		}
		var ws []Workflow
		st, _ = call(t, "GET", api+"/workflows", tok, nil, &ws)
		wantStatus(t, st, 200, "workflows")
		got := map[string]Workflow{}
		for _, w := range ws {
			got[w.Name] = w
		}
		if len(ws) != 4 || got["hook.ts"].URL != hookURL || got["nightly.ts"].Next == nil || got["manual.ts"].Last == nil || got["manual.ts"].Last.Trigger != "run" ||
			got["boom.ts"].Last == nil || got["boom.ts"].Last.Error == "" || got["nightly.ts"].Last != nil {
			t.Errorf("workflows: %+v", ws)
		}
		if rs := runs(t, "manual.ts"); len(rs) != 2 || rs[0].ID != rec.ID {
			t.Errorf("runs: %+v", rs)
		}
		st, _ = call(t, "GET", api+"/workflows", bobTok, nil, &ws)
		if wantStatus(t, st, 200, "bob's workflows"); len(ws) != 0 {
			t.Errorf("bob sees alice's workflows: %+v", ws)
		}
		for _, c := range [][2]string{{"GET", "/workflows/manual/runs"}, {"POST", "/workflows/manual/run"}, {"DELETE", "/workflows/manual"}} {
			st, _ = call(t, c[0], api+c[1], bobTok, nil, nil)
			wantStatus(t, st, 404, "bob "+c[0]+" "+c[1])
		}
		bob := run(t, bobTok, `return { files: await archive.workflows(), test: await archive.test("manual").then(() => "", (e) => e.message), runs: await archive.runs("manual").then(() => "", (e) => e.code) };`)["v"]
		if fmt.Sprint(bob) != "map[files:map[] runs:not_found test:workflow manual.ts not found; you have none yet, save one with archive.deploy]" {
			t.Errorf("bob's SDK: %v", bob)
		}
		st, _ = call(t, "GET", api+"/workflows", "", nil, nil)
		wantStatus(t, st, 401, "workflows without a token")
		st, _ = call(t, "DELETE", api+"/workflows/manual", tok, nil, nil)
		wantStatus(t, st, 204, "delete")
		st, _ = call(t, "GET", api+"/workflows", tok, nil, &ws)
		if wantStatus(t, st, 200, "workflows after delete"); len(ws) != 3 {
			t.Errorf("after delete: %+v", ws)
		}
		st, _ = call(t, "DELETE", api+"/workflows/manual", tok, nil, nil)
		wantStatus(t, st, 404, "delete again")
	})
}

func (in *e2eInst) psqlIn(db, sql string) string {
	in.t.Helper()
	out, err := exec.Command("psql", "-X", "-h", in.dir, "-U", "archive", "-d", db, "-Atc", sql).CombinedOutput()
	if err != nil {
		in.t.Fatalf("psql -d %s %q: %v\n%s", db, sql, err, out)
	}
	return strings.TrimSpace(string(out))
}

func e2eWait(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); time.Sleep(200 * time.Millisecond) {
		if ok() {
			return
		}
	}
	t.Fatalf("timed out waiting for %s", what)
}

func e2eDo(t *testing.T, method, url string, hdr map[string]string, body any, out any, cookies ...*http.Cookie) (int, []*http.Cookie) {
	t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, url, rd)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "203.0.113.7")
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	if req.Header.Get("X-Forwarded-For") == "" {
		req.Header.Del("X-Forwarded-For")
	}
	req.Host = cmp.Or(hdr["Host"], req.Host)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := (&http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if out != nil && len(b) > 0 {
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.UseNumber()
		if err := dec.Decode(out); err != nil {
			t.Fatalf("%s %s: %d %s", method, url, resp.StatusCode, b)
		}
	}
	return resp.StatusCode, resp.Cookies()
}

func e2eSession(cs []*http.Cookie) *http.Cookie {
	for _, c := range cs {
		if c.Name == "archive_session" {
			return c
		}
	}
	return nil
}

func e2eGoogle(t *testing.T) []string {
	return []string{"ARCHIVE_GOOGLE=" + strings.Replace(e2eIssuer(t).URL, "://", "://web:shh@", 1)}
}

func e2eIssuer(t *testing.T) *httptest.Server {
	t.Helper()
	issuer, err := e2eIssuerServer()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(issuer.Close)
	return issuer
}

func e2eIssuerServer() (*httptest.Server, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	b64 := base64.RawURLEncoding.EncodeToString
	var issuer *httptest.Server
	issuer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/jwks":
			json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{"kty": "RSA", "alg": "RS256", "use": "sig", "kid": "k",
				"n": b64(key.N.Bytes()), "e": b64(big.NewInt(int64(key.E)).Bytes())}}})
		case "/token":
			r.ParseForm()
			claims := map[string]any{}
			json.Unmarshal([]byte(r.Form.Get("code")), &claims)
			now := time.Now().Unix()
			if claims["iss"] == nil {
				claims["iss"] = issuer.URL
			}
			if claims["aud"] == nil {
				claims["aud"] = "web"
			}
			claims["iat"], claims["exp"] = now, now+300
			body, _ := json.Marshal(claims)
			signed := b64([]byte(`{"alg":"RS256","kid":"k","typ":"JWT"}`)) + "." + b64(body)
			sum := sha256.Sum256([]byte(signed))
			sig, _ := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"access_token": "at", "token_type": "Bearer", "expires_in": 300, "id_token": signed + "." + b64(sig)})
		default:
			json.NewEncoder(w).Encode(map[string]any{"issuer": issuer.URL, "authorization_endpoint": issuer.URL + "/authorize", "token_endpoint": issuer.URL + "/token",
				"jwks_uri": issuer.URL + "/jwks", "id_token_signing_alg_values_supported": []string{"RS256"}})
		}
	}))
	return issuer, nil
}

func (s *e2eServer) oidc(t *testing.T, provider string, claims map[string]any) (int, *http.Cookie) {
	t.Helper()
	st, _, c := s.signIn(t, provider, claims)
	return st, c
}

func (s *e2eServer) signIn(t *testing.T, provider string, claims map[string]any) (int, string, *http.Cookie) {
	t.Helper()
	st, hdr := call(t, "GET", s.url+"/auth/oidc/"+provider, "", nil, nil)
	loc, _ := url.Parse(hdr.Get("Location"))
	bind := slices.IndexFunc((&http.Response{Header: hdr}).Cookies(), func(c *http.Cookie) bool { return c.Name == "archive_oidc" })
	if st != 302 || loc == nil || bind < 0 {
		t.Fatalf("start sign-in with %s: %d %v", provider, st, hdr)
	}
	claims["nonce"] = loc.Query().Get("nonce")
	code, _ := json.Marshal(claims)
	st, back := call(t, "GET", s.url+"/auth/oidc/"+provider+"/callback?"+url.Values{"state": {loc.Query().Get("state")}, "code": {string(code)}}.Encode(), "", nil, nil, (&http.Response{Header: hdr}).Cookies()[bind])
	return st, back.Get("Location"), e2eSession((&http.Response{Header: back}).Cookies())
}

func e2eRun(t *testing.T, api, tok, code string) (int, RunOutput) {
	t.Helper()
	var out RunOutput
	st, _ := call(t, "POST", api+"/run", tok, map[string]string{"code": code}, &out)
	return st, out
}

func TestTwoWorkspacesInOneProcessSeeNothingOfEachOther(t *testing.T) {
	t.Parallel()
	in := newInst(t)
	in.must("user", "add", "alice@example.com")
	in.push(e2eSchema)
	ws := decode[Workspace](t, in.must("workspace", "add", "Acme", "bob@acme.test")).ID
	in.push(with(e2eSchema, "note.sql", e2eNote), "-workspace", ws, "-as", "bob@acme.test")
	one := in.psql("archive", "SELECT id FROM archive.workspace ORDER BY created_at LIMIT 1")
	srv := in.serve()
	api := srv.url + "/api"
	alice := decode[Token](t, in.must("token", "create")).Secret
	bob := decode[Token](t, in.must("-workspace", ws, "-as", "bob@acme.test", "token", "create")).Secret
	if !strings.HasPrefix(alice, "arc_") || !strings.HasPrefix(bob, "arc_") || ws == one {
		t.Fatalf("tokens %q and %q for workspaces %s and %s", alice, bob, one, ws)
	}

	t.Run("TablesRowsAndRawSQL", func(t *testing.T) {
		wantStatus(t, func() int {
			st, _ := call(t, "POST", api+"/company", alice, map[string]any{"name": "Alpha"}, nil)
			return st
		}(), 201, "alice creates")
		wantStatus(t, func() int {
			st, _ := call(t, "POST", api+"/company", bob, map[string]any{"name": "Beta"}, nil)
			return st
		}(), 201, "bob creates")
		var a, b []row
		call(t, "GET", api+"/company", alice, nil, &a)
		e2eDo(t, "GET", api+"/company", map[string]string{"Authorization": "Bearer " + bob, "Archive-Workspace": ws}, nil, &b)
		if names(a) != "Alpha" || names(b) != "Beta" {
			t.Errorf("alice sees %q, bob sees %q", names(a), names(b))
		}
		var ca, cb Catalog
		call(t, "GET", api+"/types", alice, nil, &ca)
		call(t, "GET", api+"/types", bob, nil, &cb)
		if ca.Tables["note"] != nil || cb.Tables["note"] == nil {
			t.Errorf("note is in alice's catalog: %v, in bob's: %v", ca.Tables["note"] != nil, cb.Tables["note"] != nil)
		}
		st, out := e2eRun(t, api, bob, `return await archive.query("SELECT current_database() AS db, (SELECT count(*)::int FROM company) AS n")`)
		if st != 200 || !strings.Contains(string(out.Result), `"db":"archive_`+ws+`"`) || !strings.Contains(string(out.Result), `"n":1`) {
			t.Errorf("bob's raw SQL: %d %+v", st, out)
		}
		if _, out := e2eRun(t, api, bob, `return await archive.query("SELECT set_config('role', '`+one+`_owner', true)")`); !strings.Contains(out.Error, "permission denied") {
			t.Errorf("bob takes on a role of workspace one: %+v", out)
		}
		if n := in.psql("archive", "SELECT count(*) FROM pg_roles WHERE rolname = '"+ws+"_authenticator'"); n != "1" {
			t.Errorf("%s authenticators for workspace %s", n, ws)
		}
	})

	t.Run("Files", func(t *testing.T) {
		st, f := e2eUpload(t, api+"/files", alice, "alpha.txt", []byte("alpha only"))
		wantStatus(t, st, 201, "alice uploads")
		var fs []File
		call(t, "GET", api+"/files", bob, nil, &fs)
		st, _ = call(t, "GET", api+"/files/"+f.ID.String(), bob, nil, nil)
		if len(fs) != 0 || st != 404 {
			t.Errorf("bob sees alice's file: %v, %d", fs, st)
		}
		st, g := e2eUpload(t, api+"/files", bob, "beta.txt", []byte("beta only"))
		wantStatus(t, st, 201, "bob uploads")
		if _, err := os.Stat(filepath.Join(in.dir, "data", "blobs", ws, g.Hash[:2], g.Hash)); err != nil {
			t.Errorf("bob's file is not under his workspace's prefix: %v", err)
		}
		if _, err := os.Stat(filepath.Join(in.dir, "data", "blobs", one, f.Hash[:2], f.Hash)); err != nil {
			t.Errorf("alice's file moved: %v", err)
		}
		var link FileLink
		call(t, "POST", api+"/files/"+g.ID.String()+"/link", bob, nil, &link)
		resp, body := e2eGet(t, link.URL, "")
		if resp.StatusCode != 200 || string(body) != "beta only" || !strings.Contains(link.URL, "/files/"+ws+"/") {
			t.Errorf("bob's file link %s: %d %q", link.URL, resp.StatusCode, body)
		}
		resp, _ = e2eGet(t, strings.Replace(link.URL, "/files/"+ws+"/", "/files/"+one+"/", 1), "")
		wantStatus(t, resp.StatusCode, 403, "bob's link in workspace one")
	})

	t.Run("WorkflowsWebhooksAndDueWork", func(t *testing.T) {
		st, out := e2eRun(t, api, alice, `return (await archive.deploy({ "hook.ts": "// on: webhook\nreturn 'alpha';" })).workflows[0].url;`)
		var hookURL string
		json.Unmarshal(out.Result, &hookURL)
		if st != 200 || !strings.Contains(hookURL, "/hooks/run/"+one+"/") {
			t.Fatalf("alice's webhook: %d %+v", st, out)
		}
		token := hookURL[strings.LastIndex(hookURL, "/")+1:]
		hookURL = srv.url + "/hooks/run/" + one + "/" + token
		var res any
		st, _ = call(t, "POST", hookURL, "", nil, &res)
		if st != 200 || res != "alpha" {
			t.Errorf("alice's webhook: %d %v", st, res)
		}
		st, _ = call(t, "POST", srv.url+"/hooks/run/"+ws+"/"+token, "", nil, nil)
		wantStatus(t, st, 404, "alice's webhook in bob's workspace")
		st, _ = call(t, "POST", srv.url+"/hooks/run/nosuch/"+token, "", nil, nil)
		wantStatus(t, st, 404, "a webhook in no workspace")

		st, out = e2eRun(t, api, bob, `return (await archive.deploy({
  "tick.ts": "// on: cron @every 1s\nawait archive.create('note', { title: 'tick' });",
  "seen.ts": "// on: company\nfor (const r of input.rows) await archive.create('note', { title: 'saw ' + r.name });",
})).deployed;`)
		if st != 200 || string(out.Result) != "true" {
			t.Fatalf("bob's deploy: %d %+v", st, out)
		}
		if n := in.psql("archive", "SELECT count(*) FROM archive.due d JOIN archive.workspace w ON w.id = d.workspace WHERE w.id::text = '"+ws+"' AND d.kind = 'workflow'"); n != "1" {
			t.Errorf("%s due rows for bob's workflows", n)
		}
		call(t, "POST", api+"/company", bob, map[string]any{"name": "Gamma"}, nil)
		e2eWait(t, "bob's workflows to run", func() bool {
			var ns []row
			call(t, "GET", api+"/note?limit=1000", bob, nil, &ns)
			return strings.Contains(names2(ns, "title"), "tick") && strings.Contains(names2(ns, "title"), "saw Gamma")
		})
		var ws1 []Workflow
		call(t, "GET", api+"/workflows", alice, nil, &ws1)
		if len(ws1) != 1 || ws1[0].Name != "hook.ts" {
			t.Errorf("alice's workflows: %+v", ws1)
		}
		if n := in.psql("archive", "SELECT count(*) FROM archive.workflow_run"); n != "1" {
			t.Errorf("workspace one holds %s workflow runs, want alice's 1 webhook run", n)
		}
	})

	t.Run("ATokenOnlyWorksInItsOwnWorkspace", func(t *testing.T) {
		st, _ := e2eDo(t, "GET", api+"/company", map[string]string{"Authorization": "Bearer " + bob, "Archive-Workspace": one}, nil, nil)
		wantStatus(t, st, 403, "bob's token in workspace one")
		st, _ = e2eDo(t, "GET", api+"/company", map[string]string{"Authorization": "Bearer " + alice, "Archive-Workspace": ws}, nil, nil)
		wantStatus(t, st, 403, "alice's token in bob's workspace")
		st, _ = e2eDo(t, "GET", api+"/company", map[string]string{"Authorization": "Bearer " + alice, "Archive-Workspace": "nosuch"}, nil, nil)
		wantStatus(t, st, 403, "alice's token in no workspace")
		if n := in.psql("archive", "SELECT count(*) FROM archive.token t JOIN archive.workspace w ON w.id = t.workspace WHERE w.id::text = '"+ws+"'"); n != "1" {
			t.Errorf("%s tokens for workspace %s in the main database", n, ws)
		}
		if n := in.psqlIn("archive_"+ws, `SELECT (SELECT count(*) FROM archive."user") + (SELECT count(*) FROM archive.token)`); n != "0" {
			t.Errorf("%s users and tokens in bob's workspace database", n)
		}
		ctx := t.Context()
		cl := mcp.NewClient(&mcp.Implementation{Name: "e2e", Version: "1"}, nil)
		sess, err := cl.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: srv.url + "/mcp", HTTPClient: &http.Client{Transport: e2eBearer(bob)}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer sess.Close()
		res, err := sess.CallTool(ctx, &mcp.CallToolParams{Name: "run", Arguments: map[string]any{"code": `return (await archive.list("company")).map((c) => c.name).join(",");`}})
		if err != nil || res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "Beta,Gamma") {
			t.Errorf("MCP as bob: %v %+v", err, res)
		}
	})
}

func (in *e2eInst) cloudsql(env ...string) *e2eInst {
	in.t.Helper()
	in.psql("archive", "CREATE ROLE cloudsql LOGIN CREATEROLE CREATEDB NOSUPERUSER PASSWORD 'cloudsql'")
	in.psql("archive", "CREATE DATABASE tenantmain OWNER cloudsql")
	dir, err := os.MkdirTemp("/tmp", "arc")
	if err != nil {
		in.t.Fatal(err)
	}
	in.t.Cleanup(func() { os.RemoveAll(dir) })
	key, err := os.ReadFile(filepath.Join(in.dir, "data", "secret.key"))
	if err != nil {
		in.t.Fatal(err)
	}
	dsn := "host=" + in.dir + " user=cloudsql password=cloudsql dbname=tenantmain"
	return &e2eInst{t: in.t, dir: dir, env: append(append([]string{"DATABASE_URL=" + dsn,
		"ARCHIVE_SECRET_KEY=" + strings.TrimSpace(string(key))}, e2eCloudTasks.queue(in.t, dir)...), env...)}
}

func TestWorkspacesIsolateWhenArchiveIsNotASuperuser(t *testing.T) {
	t.Parallel()
	host := newInst(t)
	in := host.cloudsql()
	if s := host.psqlIn("tenantmain", "SELECT rolsuper::text FROM pg_roles WHERE rolname = 'cloudsql'"); s != "false" {
		t.Fatalf("archive's role is a superuser: %q", s)
	}
	ledger := e2eTable("ledger", `    codename   text NOT NULL,
    ssn        archive.us_ssn,
    document   uuid REFERENCES archive.file,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(codename)) STORED`)
	onlyA := e2eTable("only_in_a", `    note       text NOT NULL,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(note)) STORED`)
	in.must("user", "add", "alice@example.com")
	in.push(map[string]string{"ledger.sql": ledger, "only_in_a.sql": onlyA})
	in.must("create", "ledger", `{"codename": "Zephyr"}`)
	a := host.psqlIn("tenantmain", "SELECT id FROM archive.workspace ORDER BY created_at LIMIT 1")
	b := decode[Workspace](t, in.must("workspace", "add", "Bee", "bob@bee.test")).ID
	in.push(map[string]string{"ledger.sql": ledger}, "-workspace", b, "-as", "bob@bee.test")
	in.must("-workspace", b, "-as", "bob@bee.test", "create", "ledger", `{"codename": "Hornet"}`)

	bob := func(code string) RunOutput {
		s, _, _ := in.run(code, "-workspace", b, "-as", "bob@bee.test", "run", "-")
		return decode[RunOutput](t, s)
	}
	aAdmin := a + "_owner"
	aUser := host.psqlIn("tenantmain", "SELECT '"+a+"' || '_u' || translate(rtrim(encode(uuid_send(id), 'base64'), '='), '+/', '-_') FROM archive.member WHERE email = 'alice@example.com'")

	t.Run("BobRunsInHisOwnDatabase", func(t *testing.T) {
		out := bob(`return await archive.query("SELECT current_database() AS db")`)
		if out.Error != "" || !strings.Contains(string(out.Result), "archive_"+b) {
			t.Errorf("bob's database: %+v", out)
		}
	})
	t.Run("BobCannotReachTheOtherDatabase", func(t *testing.T) {
		out := bob(`return await archive.query('SELECT count(*) FROM "tenantmain".ledger')`)
		if out.Error == "" {
			t.Errorf("bob read the main database: %v", out.Result)
		}
	})
	t.Run("BobCannotSwitchIntoTheOtherWorkspacesRoles", func(t *testing.T) {
		for _, role := range []string{aAdmin, aUser} {
			out := bob(`return await archive.query("SELECT set_config('role', '` + role + `', true)")`)
			if !strings.Contains(out.Error, "permission denied") {
				t.Errorf("set_config role %s: %+v", role, out)
			}
			out = bob(`return await archive.query('SET ROLE "` + role + `"')`)
			if !strings.Contains(out.Error, "permission denied") {
				t.Errorf("set role %s: %+v", role, out)
			}
		}
	})
	t.Run("SameTableNameInBothAndBobSeesOnlyHisRows", func(t *testing.T) {
		mine := decode[[]row](t, in.must("-workspace", b, "-as", "bob@bee.test", "list", "ledger"))
		if names2(mine, "codename") != "Hornet" {
			t.Errorf("bob's ledger: %s", names2(mine, "codename"))
		}
		alice := decode[[]row](t, in.must("list", "ledger"))
		if names2(alice, "codename") != "Zephyr" {
			t.Errorf("alice's ledger: %s", names2(alice, "codename"))
		}
	})
	t.Run("SchemaAppliedToOneOnly", func(t *testing.T) {
		out := bob(`return await archive.query("SELECT count(*)::int AS n FROM pg_class WHERE relname = 'only_in_a'")`)
		var got []struct{ N int }
		json.Unmarshal(out.Result, &got)
		if out.Error != "" || len(got) != 1 || got[0].N != 0 {
			t.Errorf("only_in_a visible to bob: %+v", out)
		}
		if n := host.psqlIn("tenantmain", "SELECT count(*) FROM pg_class WHERE relname = 'only_in_a' AND relkind = 'r'"); n != "1" {
			t.Errorf("%s only_in_a tables in the first workspace", n)
		}
	})
	t.Run("SearchStaysInItsWorkspace", func(t *testing.T) {
		if got := in.must("search", "Zephyr"); !strings.Contains(got, "Zephyr") {
			t.Errorf("alice cannot find her own row: %s", got)
		}
		if got := in.must("-workspace", b, "-as", "bob@bee.test", "search", "Zephyr"); strings.Contains(got, "Zephyr") {
			t.Errorf("bob found alice's row: %s", got)
		}
	})
	t.Run("FilesStayInTheirWorkspace", func(t *testing.T) {
		path := filepath.Join(in.dir, "memo.txt")
		os.WriteFile(path, []byte("Zephyr memo"), 0o644)
		in.must("file", "add", path)
		if got := in.must("-workspace", b, "-as", "bob@bee.test", "file", "list"); strings.Contains(got, "memo.txt") {
			t.Errorf("bob sees alice's file: %s", got)
		}
		if got := names(decode[[]row](t, in.must("file", "list"))); !strings.Contains(got, "memo.txt") {
			t.Errorf("alice cannot see her own file: %s", got)
		}
	})
	t.Run("MembersAndGroupsBecomeRolesThatGoAwayWithThem", func(t *testing.T) {
		in := in.on(t)
		in.must("user", "add", "tess@example.com", "-role", "editor")
		in.groups("tess@example.com", "team")
		path := filepath.Join(in.dir, "team.txt")
		os.WriteFile(path, []byte("team memo"), 0o644)
		f := decode[File](t, in.must("-as", "tess@example.com", "file", "add", path, "-access", "team"))
		in.must("-as", "tess@example.com", "create", "ledger", `{"codename": "Team", "access": "team", "document": "`+f.ID.String()+`"}`)
		if got := names2(decode[[]row](t, in.must("-as", "tess@example.com", "list", "ledger")), "codename"); !strings.Contains(got, "Team") || !strings.Contains(got, "Zephyr") {
			t.Errorf("tess's ledger: %s", got)
		}
		role := host.psqlIn("tenantmain", "SELECT '"+a+"' || '_u' || translate(rtrim(encode(uuid_send(id), 'base64'), '='), '+/', '-_') FROM archive.member WHERE email = 'tess@example.com'")
		in.must("user", "remove", "tess@example.com")
		got := host.psqlIn("tenantmain", "SELECT (SELECT count(*) FROM ledger WHERE codename = 'Team') || ',' || (SELECT count(*) FROM archive.file WHERE id = '"+f.ID.String()+"') || ',' || "+
			"(SELECT count(*) FROM pg_roles WHERE rolname IN ('"+role+"', '"+strings.Replace(role, "_u", "_m", 1)+"', '"+a+"_g_team'))")
		if got != "1,1,0" {
			t.Errorf("after removing tess (her rows, her file, her roles): %s", got)
		}
		if n := host.psqlIn("tenantmain", "SELECT count(*) FROM pg_auth_members m JOIN pg_roles r ON r.oid = m.roleid JOIN pg_roles u ON u.oid = m.member WHERE u.rolname = 'cloudsql' AND m.set_option AND starts_with(r.rolname, '"+a+"')"); n != "0" {
			t.Errorf("archive's role can still set %s workspace roles", n)
		}
	})
	t.Run("DeletingARowClearsItsSecret", func(t *testing.T) {
		in := in.on(t)
		r := decode[row](t, in.must("create", "ledger", `{"codename": "Secretive", "ssn": "234-56-7890"}`))
		if n := host.psqlIn("tenantmain", "SELECT count(*) FROM archive.secret WHERE row_id = '"+id(r)+"'"); n != "1" {
			t.Fatalf("%s secrets for the new row", n)
		}
		in.must("delete", "ledger", id(r))
		if n := host.psqlIn("tenantmain", "SELECT count(*) FROM archive.secret WHERE row_id = '"+id(r)+"'"); n != "0" {
			t.Errorf("%s secrets left after deleting the row", n)
		}
	})
	t.Run("WorkflowsWriteAsTheirOwner", func(t *testing.T) {
		in := in.on(t)
		s, _, _ := in.run(`await archive.deploy({ "add.ts": "await archive.create('ledger', { codename: 'Flowed' });" });
await archive.start("add.ts");
return (await archive.list("ledger")).map((r) => r.codename).join(",");`, "run", "-")
		out := decode[RunOutput](t, s)
		if out.Error != "" || !strings.Contains(string(out.Result), "Flowed") {
			t.Errorf("workflow write: %+v", out)
		}
	})
	t.Run("DeletingBDropsItsDatabaseAndRoles", func(t *testing.T) {
		in.must("workspace", "delete", b)
		got := host.psqlIn("tenantmain", "SELECT (SELECT count(*) FROM pg_database WHERE datname = 'archive_"+b+"') || ',' || "+
			"(SELECT count(*) FROM pg_roles WHERE starts_with(rolname, '"+b+"')) || ',' || "+
			"(SELECT count(*) FROM archive.workspace WHERE id = '"+b+"') || ',' || "+
			"(SELECT count(*) FROM archive.\"user\" WHERE workspaces @> ARRAY['"+b+"'::uuid])")
		if got != "0,0,0,0" {
			t.Errorf("left after deleting workspace B: %s", got)
		}
	})
}

func names2(rs []row, col string) string {
	var out []string
	for _, r := range rs {
		out = append(out, fmt.Sprint(r[col]))
	}
	return strings.Join(out, ",")
}

func TestInviteOnlySignInUsersAndSigningInToSeveralWorkspaces(t *testing.T) {
	t.Parallel()
	in := newInst(t, e2eGoogle(t)...)
	in.must("user", "add", "alice@example.com")
	srv := in.serve()
	one := in.psql("archive", "SELECT id FROM archive.workspace ORDER BY created_at LIMIT 1")
	carolID := map[string]any{"sub": "carol-1", "email": "Carol@Example.com", "email_verified": true, "name": " Carol  Co "}
	if st, c := srv.oidc(t, "google", maps.Clone(carolID)); st != 403 || c != nil {
		t.Fatalf("Google sign-in for someone nobody invited: %d %v", st, c)
	}
	if n := in.psql("archive", "SELECT (SELECT count(*) FROM archive.workspace) || ' ' || (SELECT count(*) FROM archive.\"user\")"); n != "1 1" {
		t.Errorf("an uninvited sign-in left workspaces and users behind: %s", n)
	}
	in.must("user", "add", "dave@example.com")
	st, dave := srv.oidc(t, "google", map[string]any{"sub": "dave-1", "email": "dave@example.com", "email_verified": true, "name": "Dave  Day"})
	var me struct {
		Actor     Actor       `json:"actor"`
		Workspace string      `json:"workspace"`
		All       []Workspace `json:"workspaces"`
	}
	if st != 303 || dave == nil {
		t.Fatalf("an invited person signs in with Google: %d", st)
	}
	if e2eDo(t, "GET", srv.url+"/auth/me", nil, nil, &me, dave); me.Actor.Email != "dave@example.com" || me.Actor.Name != "Dave Day" || me.Workspace != one {
		t.Fatalf("an invited person's first Google sign-in links them, with their name: %+v", me)
	}
	if st, c := srv.oidc(t, "google", map[string]any{"sub": "dave-1", "email": "dave@elsewhere.test", "name": "Someone Else"}); st != 303 || c == nil {
		t.Errorf("the linked Google account signs in again without its email: %d", st)
	}
	if e2eDo(t, "GET", srv.url+"/auth/me", nil, nil, &me, dave); me.Actor.Name != "Dave Day" {
		t.Errorf("a later sign-in renamed dave: %+v", me.Actor)
	}
	in.must("user", "add", "frank@example.com")
	if st, _ := srv.oidc(t, "google", map[string]any{"sub": "frank-1", "email": "frank@example.com", "email_verified": false}); st != 403 {
		t.Errorf("an unverified email takes over an invite: %d", st)
	}
	if st, _ = e2eDo(t, "POST", srv.url+"/auth/signup", nil, map[string]string{"name": "Dave Two"}, nil, dave); st != 404 && st != 405 {
		t.Errorf("a signed-in person adds a workspace of their own: %d", st)
	}

	st, alice := srv.oidc(t, "google", map[string]any{"sub": "alice-1", "email": "alice@example.com", "email_verified": true})
	if st != 303 || alice == nil {
		t.Fatalf("the first owner signs in with Google: %d", st)
	}
	who := func(ps []Person) string {
		var out []string
		for _, p := range ps {
			out = append(out, fmt.Sprintf("%s %s %s", p.Email, p.Name, p.Role))
		}
		return strings.Join(out, ",")
	}
	var people []Person
	st, _ = e2eDo(t, "GET", srv.url+"/auth/users", nil, nil, &people, dave)
	if want := "alice@example.com  owner,dave@example.com Dave Day viewer,frank@example.com  viewer"; st != 200 || who(people) != want {
		t.Errorf("dave lists the people in the workspace: %d %s", st, who(people))
	}
	st, _ = call(t, "GET", srv.url+"/auth/users", "", nil, nil)
	wantStatus(t, st, 401, "the people without signing in")
	st, _ = e2eDo(t, "POST", srv.url+"/auth/users", nil, map[string]any{"email": "carol@example.com"}, nil, dave)
	wantStatus(t, st, 403, "a viewer invites someone")
	st, _ = e2eDo(t, "DELETE", srv.url+"/auth/users/frank@example.com", nil, nil, nil, dave)
	wantStatus(t, st, 403, "a viewer removes someone")
	scoped := decode[Token](t, in.must("token", "create", "-viewer")).Secret
	st, _ = call(t, "POST", srv.url+"/auth/users", scoped, map[string]any{"email": "carol@example.com"}, nil)
	wantStatus(t, st, 403, "an owner's viewer-only token invites someone")
	st, _ = e2eDo(t, "POST", srv.url+"/auth/users", nil, map[string]any{"email": "not an email"}, nil, alice)
	wantStatus(t, st, 422, "inviting something that is not an email")
	st, _ = e2eDo(t, "POST", srv.url+"/auth/users", nil, map[string]any{"email": " Carol@Example.com ", "role": "owner"}, &people, alice)
	if want := "alice@example.com  owner,carol@example.com  owner,dave@example.com Dave Day viewer,frank@example.com  viewer"; st != 201 || who(people) != want {
		t.Errorf("an owner invites carol as an owner: %d %s", st, who(people))
	}
	st, _ = e2eDo(t, "POST", srv.url+"/auth/users", nil, map[string]any{"email": "carol@example.com"}, nil, alice)
	wantStatus(t, st, 409, "inviting carol twice")
	st, carol := srv.oidc(t, "google", maps.Clone(carolID))
	if st != 303 || carol == nil {
		t.Fatalf("carol signs in once invited: %d", st)
	}
	if e2eDo(t, "GET", srv.url+"/auth/me", nil, nil, &me, carol); me.Actor.Email != "carol@example.com" || me.Actor.Name != "Carol Co" || me.Workspace != one {
		t.Fatalf("carol's first Google sign-in: %+v", me)
	}
	st, _ = e2eDo(t, "DELETE", srv.url+"/auth/users/carol@example.com", nil, nil, &people, alice)
	if want := "alice@example.com  owner,dave@example.com Dave Day viewer,frank@example.com  viewer"; st != 200 || who(people) != want {
		t.Errorf("an owner removes carol: %d %s", st, who(people))
	}
	if st, c := srv.oidc(t, "google", maps.Clone(carolID)); st != 403 || c != nil {
		t.Errorf("carol signs in with Google after she was removed: %d", st)
	}
	st, _ = e2eDo(t, "GET", srv.url+"/auth/me", nil, nil, nil, carol)
	wantStatus(t, st, 403, "carol's session after she was removed")

	made := decode[Workspace](t, in.must("workspace", "add", "Carol Co", "carol@example.com")).ID
	if got := in.psql("archive", "SELECT w.name || ' ' || (SELECT count(*) FROM pg_database WHERE datname = 'archive_' || w.id) FROM archive.workspace w WHERE w.id::text = '"+made+"'"); got != "Carol Co 1" {
		t.Errorf("carol's workspace row and database: %q", got)
	}
	st, carol = srv.oidc(t, "google", maps.Clone(carolID))
	if st != 303 || carol == nil {
		t.Fatalf("carol signs in to the workspace an admin made for her: %d", st)
	}
	if e2eDo(t, "GET", srv.url+"/auth/me", nil, nil, &me, carol); me.Workspace != made || len(me.All) != 1 {
		t.Fatalf("carol's workspaces: %+v", me)
	}
	var out RunOutput
	st, _ = e2eDo(t, "POST", srv.url+"/api/run", nil, map[string]string{"code": "return (await archive.list('member')).map((m) => m.role).join(',')"}, &out, carol)
	if st != 200 || string(out.Result) != `"owner"` {
		t.Errorf("carol runs in her workspace: %d %+v", st, out)
	}
	st, _ = e2eDo(t, "POST", srv.url+"/auth/workspaces", nil, map[string]string{"name": "Mine", "owner": "carol@example.com"}, nil, carol)
	wantStatus(t, st, 403, "a workspace owner creates a workspace for someone")

	full := decode[Token](t, in.must("token", "create")).Secret
	var second Workspace
	st, _ = call(t, "POST", srv.url+"/auth/workspaces", full, map[string]string{"name": "Second", "owner": "alice@example.com"}, &second)
	wantStatus(t, st, 201, "an instance admin creates a workspace")
	e2eDo(t, "GET", srv.url+"/auth/me", nil, nil, &me, alice)
	if me.Workspace != one || len(me.All) != 2 || me.All[0].ID != one || me.All[1].ID != second.ID {
		t.Errorf("me without a workspace header: %+v", me)
	}
	st, _ = e2eDo(t, "GET", srv.url+"/auth/me", map[string]string{"Archive-Workspace": second.ID}, nil, &me, alice)
	if st != 200 || me.Workspace != second.ID {
		t.Errorf("me in the workspace the header names: %d %+v", st, me)
	}
	st, _ = e2eDo(t, "POST", srv.url+"/api/run", map[string]string{"Archive-Workspace": second.ID}, map[string]string{"code": "return (await archive.list('member')).map((m) => m.role).join(',')"}, &out, alice)
	if st != 200 || string(out.Result) != `"owner"` {
		t.Errorf("alice runs in the second workspace with the same cookie: %d %+v", st, out)
	}
	st, _ = e2eDo(t, "GET", srv.url+"/auth/me", map[string]string{"Archive-Workspace": made}, nil, nil, alice)
	wantStatus(t, st, 403, "alice in carol's workspace")
	st, _ = e2eDo(t, "GET", srv.url+"/auth/me", map[string]string{"Archive-Workspace": one}, nil, nil, carol)
	wantStatus(t, st, 403, "carol in workspace one")

	in.must("-workspace", second.ID, "user", "add", "carol@example.com")
	in.fails("-workspace", second.ID, "user", "add", "carol@example.com")
	in2 := map[string]string{"Archive-Workspace": second.ID}
	var tok Token
	st, _ = e2eDo(t, "POST", srv.url+"/auth/tokens", in2, map[string]any{"name": "carol in second"}, &tok, carol)
	wantStatus(t, st, 201, "carol makes a token in the second workspace")
	st, _ = call(t, "GET", srv.url+"/api/types", tok.Secret, nil, nil)
	wantStatus(t, st, 200, "carol's token in the second workspace")
	if errs := in.fails("-workspace", second.ID, "-as", "carol@example.com", "user", "remove", "alice@example.com"); !strings.Contains(errs, "forbidden") {
		t.Errorf("carol removes alice: %s", errs)
	}
	in.must("-workspace", second.ID, "user", "remove", "carol@example.com")
	st, _ = e2eDo(t, "GET", srv.url+"/auth/me", in2, nil, nil, carol)
	wantStatus(t, st, 403, "carol in the workspace she was removed from")
	st, _ = call(t, "GET", srv.url+"/api/types", tok.Secret, nil, nil)
	wantStatus(t, st, 401, "carol's token after she was removed")
	st, _ = e2eDo(t, "GET", srv.url+"/auth/me", nil, nil, &me, carol)
	if st != 200 || me.Workspace != made || len(me.All) != 1 {
		t.Errorf("carol still in her own workspace: %d %+v", st, me)
	}
	if n := in.psql("archive", "SELECT count(*) FROM pg_roles WHERE starts_with(rolname, '"+second.ID+"_u')"); n != "1" {
		t.Errorf("%s user roles left in the second workspace, want alice's", n)
	}
	if got := decode[[]row](t, in.must("-workspace", second.ID, "list", "member")); len(got) != 1 || got[0]["email"] != "alice@example.com" {
		t.Errorf("members of the second workspace: %+v", got)
	}

	st, cs := e2eDo(t, "POST", srv.url+"/auth/signout", nil, nil, nil, alice)
	if left := e2eSession(cs); st != 204 || left == nil || left.Value != "" {
		t.Fatalf("sign out: %d %v", st, left)
	}
	for _, ws := range []string{one, second.ID} {
		st, _ = e2eDo(t, "GET", srv.url+"/auth/me", map[string]string{"Archive-Workspace": ws}, nil, nil, alice)
		wantStatus(t, st, 401, "the signed-out session in "+ws)
	}
}

func TestGitHubAndMicrosoftSignInOnlyMatchVerifiedEmails(t *testing.T) {
	t.Parallel()
	b64 := base64.RawURLEncoding
	var gh *httptest.Server
	gh = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/login/oauth/access_token" {
			r.ParseForm()
			id, secret, _ := r.BasicAuth()
			id, _ = url.QueryUnescape(cmp.Or(id, r.Form.Get("client_id")))
			secret, _ = url.QueryUnescape(cmp.Or(secret, r.Form.Get("client_secret")))
			if id != "gh" || secret != "s/h:h@ +" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"access_token": b64.EncodeToString([]byte(r.Form.Get("code"))), "token_type": "bearer"})
			return
		}
		raw, _ := b64.DecodeString(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		var user map[string]any
		if json.Unmarshal(raw, &user) != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/user":
			json.NewEncoder(w).Encode(map[string]any{"id": user["id"], "login": "gina-g", "name": user["name"]})
		case "/user/emails":
			json.NewEncoder(w).Encode(user["emails"])
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(gh.Close)
	ms := e2eIssuer(t)
	in := newInst(t,
		"ARCHIVE_GITHUB="+strings.Replace(gh.URL, "://", "://gh:s%2Fh%3Ah%40%20%2B@", 1), "ARCHIVE_MICROSOFT="+strings.Replace(ms.URL, "://", "://web:shh@", 1),
		"ARCHIVE_SLACK=https://slack:shh@slack.com", "ARCHIVE_GOOGLE=https://only-an-id@accounts.google.com")
	in.must("user", "add", "alice@example.com")
	srv := in.serve()
	var methods struct {
		Providers []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"providers"`
	}
	call(t, "GET", srv.url+"/auth/methods", "", nil, &methods)
	if fmt.Sprint(methods.Providers) != "[{microsoft Microsoft} {slack Slack} {github GitHub}]" {
		t.Errorf("providers with both a client id and secret: %v", methods.Providers)
	}
	in.must("user", "add", "gina@example.com")
	in.must("user", "add", "mia@example.com")
	email := func(addr string, primary, verified bool) map[string]any {
		return map[string]any{"email": addr, "primary": primary, "verified": verified}
	}
	for what, emails := range map[string][]any{
		"an unverified primary email":          {email("gina@example.com", true, false)},
		"a verified email that is not primary": {email("gina@example.com", false, true), email("gina@other.test", true, true)},
		"no email at all":                      {},
	} {
		if st, c := srv.oidc(t, "github", map[string]any{"id": 42, "name": "Gina G", "emails": emails}); st != 403 || c != nil {
			t.Errorf("GitHub sign-in with %s: %d", what, st)
		}
	}
	st, gina := srv.oidc(t, "github", map[string]any{"id": 42, "name": "Gina G", "emails": []any{email("gina@other.test", false, true), email("Gina@Example.com", true, true)}})
	if st != 303 || gina == nil {
		t.Fatalf("GitHub sign-in with the invited email as the verified primary: %d", st)
	}
	var me struct {
		Actor Actor `json:"actor"`
	}
	if e2eDo(t, "GET", srv.url+"/auth/me", nil, nil, &me, gina); me.Actor.Email != "gina@example.com" || me.Actor.Name != "Gina G" {
		t.Errorf("gina after GitHub sign-in: %+v", me.Actor)
	}
	if got := in.psql("archive", "SELECT signin->'oidc' FROM archive.\"user\" WHERE email = 'gina@example.com'"); got != `[{"iss": "`+gh.URL+`", "sub": "42"}]` {
		t.Errorf("gina's GitHub link: %s", got)
	}
	if st, c := srv.oidc(t, "github", map[string]any{"id": 42, "emails": []any{}}); st != 303 || c == nil {
		t.Errorf("gina signs in again by her GitHub user id: %d", st)
	}
	if st, _ := srv.oidc(t, "github", map[string]any{"id": 0, "emails": []any{email("gina@example.com", true, true)}}); st != 401 {
		t.Errorf("a GitHub user without an id: %d", st)
	}

	tenant := ms.URL + "/9188040d-6c67-4c5b-b112-36a304b66dad/v2.0"
	if st, _ := srv.oidc(t, "microsoft", map[string]any{"iss": tenant, "sub": "mia-1", "email": "mia@example.com"}); st != 403 {
		t.Errorf("Microsoft sign-in with an email nobody vouched for: %d", st)
	}
	if st, _ := srv.oidc(t, "microsoft", map[string]any{"iss": "http://evil.test/tenant/v2.0", "sub": "mia-1", "email": "mia@example.com", "xms_edov": true}); st != 401 {
		t.Errorf("Microsoft sign-in from another issuer: %d", st)
	}
	if st, c := srv.oidc(t, "microsoft", map[string]any{"iss": tenant, "sub": "mia-1", "email": "mia@example.com", "xms_edov": true}); st != 303 || c == nil {
		t.Errorf("Microsoft sign-in with a domain-verified email: %d", st)
	}
}

func TestGeneratedURLsUseTheAPIAddress(t *testing.T) {
	t.Parallel()
	in := newInst(t, append(e2eGoogle(t), "ARCHIVE_URL=http://app.test/")...)
	in.must("user", "add", "alice@example.com")
	srv := in.serve()
	api := srv.url + "/api"
	one := in.psql("archive", "SELECT id FROM archive.workspace ORDER BY created_at LIMIT 1")
	tok := decode[Token](t, in.must("token", "create")).Secret
	_, out := e2eRun(t, api, tok, `return (await archive.deploy({ "hook.ts": "// on: webhook\nreturn 1;" })).workflows[0].url;`)
	if !strings.HasPrefix(string(out.Result), `"http://api.test/hooks/run/`+one+`/`) {
		t.Errorf("webhook URL: %+v", out)
	}
	_, f := e2eUpload(t, api+"/files", tok, "a.txt", []byte("a"))
	var link FileLink
	call(t, "POST", api+"/files/"+f.ID.String()+"/link", tok, nil, &link)
	if !strings.HasPrefix(link.URL, "http://api.test/files/"+one+"/") {
		t.Errorf("file link: %s", link.URL)
	}
	st, hdr := call(t, "GET", srv.url+"/auth/oidc/google", "", nil, nil)
	loc, _ := url.Parse(hdr.Get("Location"))
	if st != 302 || loc == nil || loc.Query().Get("redirect_uri") != "http://api.test/auth/oidc/google/callback" || loc.Query().Get("state") == "" {
		t.Errorf("OIDC redirect: %d %s", st, hdr.Get("Location"))
	}
	st, _ = call(t, "GET", srv.url+"/auth/oidc/google/callback?state=forged&code=x", "", nil, nil)
	wantStatus(t, st, 401, "an OIDC callback with a forged state")
	if !strings.Contains(srv.log.String(), "the API on http://api.test/api") {
		t.Errorf("startup line:\n%s", srv.log)
	}
}

func e2eFrom(t *testing.T, method, url, origin string, cookies ...*http.Cookie) (int, http.Header) {
	t.Helper()
	req, _ := http.NewRequest(method, url, nil)
	req.Header.Set("Origin", origin)
	req.Header.Set("X-Forwarded-For", "203.0.113.7")
	if method == "OPTIONS" {
		req.Header.Set("Access-Control-Request-Method", "PATCH")
		req.Header.Set("Access-Control-Request-Headers", "content-type,archive-workspace")
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := (&http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode, resp.Header
}

func TestTheWebAppOnItsOwnHostCallsTheAPIHostWithTheAPIsCookie(t *testing.T) {
	t.Parallel()
	in := newInst(t, append(e2eGoogle(t), "ARCHIVE_URL=https://app.test")...)
	in.must("user", "add", "alice@example.com")
	srv := in.serve()
	st, hdr := e2eFrom(t, "OPTIONS", srv.url+"/api/chats", "https://app.test")
	allowed := hdr.Get("Access-Control-Allow-Headers") + " " + hdr.Get("Access-Control-Allow-Methods")
	if st != 204 || hdr.Get("Access-Control-Allow-Origin") != "https://app.test" || hdr.Get("Access-Control-Allow-Credentials") != "true" || hdr.Get("Access-Control-Max-Age") == "" ||
		!strings.Contains(hdr.Get("Vary"), "Origin") || !strings.Contains(allowed, "Archive-Workspace") || strings.Contains(allowed, "Archive-Space") || !strings.Contains(allowed, "Content-Type") || !strings.Contains(allowed, "PATCH") {
		t.Errorf("preflight from the app: %d %v", st, hdr)
	}
	if strings.Contains(allowed, "Authorization") {
		t.Errorf("the app's preflight allows a bearer token: %v", hdr)
	}
	for _, m := range []string{"OPTIONS", "GET"} {
		for _, origin := range []string{"https://evil.test", "https://api.test.evil.test", "null"} {
			_, hdr := e2eFrom(t, m, srv.url+"/auth/methods", origin)
			if hdr.Get("Access-Control-Allow-Origin") != "*" || hdr.Get("Access-Control-Allow-Credentials") != "" || m == "OPTIONS" && !strings.Contains(hdr.Get("Access-Control-Allow-Headers"), "Authorization") {
				t.Errorf("%s from %s: %v", m, origin, hdr)
			}
		}
	}
	if _, hdr := e2eFrom(t, "OPTIONS", srv.url+"/mcp", "https://app.test"); hdr.Get("Access-Control-Allow-Origin") != "*" || hdr.Get("Access-Control-Allow-Credentials") != "" {
		t.Errorf("MCP from the app is a bearer call like from anywhere: %v", hdr)
	}
	tok := decode[Token](t, in.must("token", "create")).Secret
	req, _ := http.NewRequest("GET", srv.url+"/api/types", nil)
	req.Header.Set("Origin", "https://tool.example")
	req.Header.Set("Authorization", "Bearer "+tok)
	if resp, err := http.DefaultClient.Do(req); err != nil || resp.StatusCode != 200 || resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("a bearer call from another site: %v %v", err, resp)
	} else {
		resp.Body.Close()
	}
	_, start := call(t, "GET", srv.url+"/auth/oidc/google", "", nil, nil)
	loc, _ := url.Parse(start.Get("Location"))
	if st, _ := call(t, "GET", srv.url+"/auth/oidc/google/callback?"+url.Values{"state": {loc.Query().Get("state")}, "code": {"{}"}}.Encode(), "", nil, nil); st != 401 {
		t.Errorf("a callback in a browser that did not start the sign-in: %d", st)
	}
	st, to, c := srv.signIn(t, "google", map[string]any{"sub": "alice-1", "email": "alice@example.com", "email_verified": true})
	if st != http.StatusSeeOther || to != "https://app.test/" || c == nil || c.Domain != "" || c.Path != "/" || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode {
		t.Fatalf("Google sign-in sets the API's own cookie and returns to the app: %d %q %+v", st, to, c)
	}
	for _, path := range []string{"/auth/me", "/api/types", "/api/chats"} {
		if st, hdr := e2eFrom(t, "GET", srv.url+path, "https://app.test", c); st != 200 || hdr.Get("Access-Control-Allow-Origin") != "https://app.test" || hdr.Get("Access-Control-Allow-Credentials") != "true" {
			t.Errorf("GET %s from the app with the cookie: %d %v", path, st, hdr)
		}
	}
	st, out := e2eFrom(t, "POST", srv.url+"/auth/signout", "https://app.test", c)
	gone := e2eSession((&http.Response{Header: out}).Cookies())
	if st != 204 || gone == nil || gone.MaxAge >= 0 || out.Get("Access-Control-Allow-Origin") != "https://app.test" {
		t.Errorf("sign out from the app: %d %v", st, out)
	}
	if st, _ := e2eFrom(t, "GET", srv.url+"/auth/me", "https://app.test", c); st != 401 {
		t.Errorf("me after sign out: %d", st)
	}
}

func TestCloudTasksWakeTheServiceAtEachDueTimeAndNothingRunsOnItsOwn(t *testing.T) {
	t.Parallel()
	google, forger := e2eIssuer(t), e2eIssuer(t)
	fake, err := e2eTasksStart(true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(fake.stop)
	queue, acct := "projects/p/locations/l/queues/q", "q-tasks@p.iam.gserviceaccount.com"
	in := newInst(t, "ARCHIVE_TASKS_URL="+fake.addr+"/"+queue, "ARCHIVE_TASKS_ISSUER="+google.URL, "ARCHIVE_URL=http://app.test")
	in.must("user", "add", "alice@example.com")
	in.push(map[string]string{"house.sql": e2eHouse})
	srv := in.serve()
	api := srv.url + "/api"
	one := in.psql("archive", "SELECT id FROM archive.workspace ORDER BY created_at LIMIT 1")
	tok := decode[Token](t, in.must("token", "create")).Secret
	last := func(t *testing.T) hubTask {
		t.Helper()
		posted := fake.tasks()
		if len(posted) == 0 {
			t.Fatal("no task was created")
		}
		return posted[len(posted)-1]
	}
	count := func() int { return len(fake.tasks()) }
	deploy := func(t *testing.T, files map[string]string) map[string]any {
		t.Helper()
		b, _ := json.Marshal(files)
		st, out := e2eRun(t, api, tok, `const d = await archive.deploy(`+string(b)+`); return Object.fromEntries(d.workflows.map((w) => [w.name, w.next ?? null]));`)
		if st != 200 || out.Error != "" {
			t.Fatalf("deploy: %d %+v", st, out)
		}
		return decode[map[string]any](t, string(out.Result))
	}
	runs := func(t *testing.T, name string) []WorkflowRun {
		t.Helper()
		var rs []WorkflowRun
		st, _ := call(t, "GET", api+"/workflows/"+name+"/runs", tok, nil, &rs)
		wantStatus(t, st, 200, "runs")
		return rs
	}
	idToken := func(t *testing.T, iss *httptest.Server, claims map[string]any) string {
		t.Helper()
		code, _ := json.Marshal(claims)
		resp, err := http.PostForm(iss.URL+"/token", url.Values{"code": {string(code)}})
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var out struct {
			IDToken string `json:"id_token"`
		}
		json.NewDecoder(resp.Body).Decode(&out)
		return out.IDToken
	}
	var aud string
	claims := func(kv ...any) map[string]any {
		c := map[string]any{"iss": google.URL, "aud": aud, "sub": "1", "email": acct, "email_verified": true}
		for i := 0; i+1 < len(kv); i += 2 {
			c[kv[i].(string)] = kv[i+1]
		}
		return c
	}
	deliver := func(t *testing.T, tok string, body any) int {
		t.Helper()
		hdr := map[string]string{}
		if tok != "" {
			hdr["Authorization"] = "Bearer " + tok
		}
		st, _ := e2eDo(t, "POST", srv.url+"/tasks", hdr, body, nil)
		return st
	}
	work := map[string]string{"workspace": one, "work": "workflows"}

	next := deploy(t, map[string]string{"tick.ts": "// on: cron 0 9 * * *\nreturn 'tick';"})
	want, _ := time.Parse(time.RFC3339Nano, fmt.Sprint(next["tick.ts"]))
	tk := last(t)
	hr := tk.HTTPRequest
	var body map[string]string
	json.Unmarshal(hr.Body, &body)
	if !tk.ScheduleTime.Equal(want) || !strings.HasPrefix(tk.Name, queue+"/tasks/") || hr.HTTPMethod != "POST" ||
		hr.URL != "http://api.test/tasks" || hr.OIDCToken.Audience != hr.URL ||
		hr.OIDCToken.ServiceAccountEmail != acct || fmt.Sprint(body) != fmt.Sprint(work) {
		t.Fatalf("the cron workflow's task: %+v %v, want it at %s", tk, body, want)
	}
	aud = hr.URL

	deploy(t, map[string]string{"tick.ts": "// on: cron 0 9 * * *\nreturn 'tick';",
		"sold.ts": "// on: house\nfor (const r of input.rows) await archive.update('house', r.id, { sold: true });\nreturn input.rows.map((r) => r.address).join(',');"})
	if tk := last(t); time.Until(tk.ScheduleTime) > time.Second {
		t.Errorf("a table workflow's first task is at %s, want now", tk.ScheduleTime)
	}
	call(t, "POST", api+"/house", tok, map[string]any{"address": "Canal 1"}, nil)
	srv.stop()
	srv = in.serve()
	api = srv.url + "/api"
	time.Sleep(3 * time.Second)
	if rs := runs(t, "sold"); len(rs) != 0 {
		t.Fatalf("a workflow ran without a task arriving: %+v", rs)
	}

	for what, c := range map[string]struct {
		tok  string
		want int
	}{
		"no token":                {"", 401},
		"a forged token":          {idToken(t, forger, claims()), 401},
		"another audience":        {idToken(t, google, claims("aud", "https://elsewhere.test/tasks")), 401},
		"another service account": {idToken(t, google, claims("email", "someone@p.iam.gserviceaccount.com")), 403},
		"an unverified email":     {idToken(t, google, claims("email_verified", false)), 403},
	} {
		if st := deliver(t, c.tok, work); st != c.want {
			t.Errorf("%s: status %d, want %d", what, st, c.want)
		}
	}
	if rs := runs(t, "sold"); len(rs) != 0 {
		t.Fatalf("a refused call ran workflows: %+v", rs)
	}
	good := idToken(t, google, claims())
	if st := deliver(t, good, map[string]string{"workspace": one, "work": "everything"}); st != 400 {
		t.Errorf("unknown work: %d", st)
	}

	if st := deliver(t, good, work); st != 204 {
		t.Fatalf("delivering the task: %d\n%s", st, srv.log)
	}
	if rs := runs(t, "sold"); len(rs) != 1 || string(rs[0].Result) != `"Canal 1"` || rs[0].Trigger != "table" {
		t.Fatalf("the task's runs: %+v", rs)
	}
	if tk := last(t); !tk.ScheduleTime.Equal(want) {
		t.Errorf("after the run the next task is at %s, want the cron time %s", tk.ScheduleTime, want)
	}
	if at := in.psql("archive", "SELECT at = '"+want.Format(time.RFC3339Nano)+"' FROM archive.due WHERE kind = 'workflow'"); at != "t" {
		t.Errorf("due after the run: %s", in.psql("archive", "SELECT at FROM archive.due"))
	}

	n := count()
	st, out := e2eRun(t, api, tok, `const [h] = await archive.list("house"); await archive.update("house", h.id, { sold: false }); return 0;`)
	if st != 200 || out.Error != "" || count() != n+1 || time.Until(last(t).ScheduleTime) > time.Second || time.Since(last(t).ScheduleTime) > 5*time.Second {
		t.Fatalf("a write made %d tasks, the last at %s: %+v", count()-n, last(t).ScheduleTime, out)
	}
	time.Sleep(time.Second)
	if rs := runs(t, "sold"); len(rs) != 1 {
		t.Fatalf("the write ran a workflow without a task: %+v", rs)
	}
	if st := deliver(t, good, work); st != 204 {
		t.Fatalf("delivering the second task: %d", st)
	}
	if rs := runs(t, "sold"); len(rs) != 2 || string(rs[0].Result) != `"Canal 1"` {
		t.Errorf("after the second task: %+v", rs)
	}
	if st := deliver(t, good, work); st != 204 || len(runs(t, "sold")) != 2 || len(runs(t, "tick")) != 0 {
		t.Errorf("a task with nothing due: %d", st)
	}
}

func TestServeOpensOtherWorkspacesOnlyWhenTheyAreUsed(t *testing.T) {
	t.Parallel()
	in := newInst(t)
	in.must("user", "add", "alice@example.com")
	ws := decode[Workspace](t, in.must("workspace", "add", "Acme", "bob@acme.test")).ID
	bob := decode[Token](t, in.must("-workspace", ws, "-as", "bob@acme.test", "token", "create")).Secret
	in.psqlIn("postgres", `ALTER DATABASE "archive_`+ws+`" ALLOW_CONNECTIONS false`)
	srv := in.serve()
	time.Sleep(time.Second)
	if strings.Contains(srv.log.String(), ws) {
		t.Fatalf("serve opened workspace %s at startup:\n%s", ws, srv.log)
	}
	in.psqlIn("postgres", `ALTER DATABASE "archive_`+ws+`" ALLOW_CONNECTIONS true`)
	var info Workspace
	st, _ := call(t, "GET", srv.url+"/auth/workspace", bob, nil, &info)
	if st != 200 || info.ID != ws {
		t.Errorf("bob's workspace on first use: %d %+v\n%s", st, info, srv.log)
	}
}

func TestServeStartsWhenTheFirstUserHasNoMemberRow(t *testing.T) {
	t.Parallel()
	in := newInst(t)
	in.must("user", "add", "alice@example.com")
	in.must("user", "add", "bob@example.com", "-role", "owner")
	in.psql("archive", "DELETE FROM archive.member WHERE email = 'alice@example.com'")
	srv := in.serve()
	if st, _ := e2eDo(t, "GET", srv.url+"/auth/me", map[string]string{"X-Forwarded-For": ""}, nil, nil); st != 403 {
		t.Errorf("the first user without a member row on this machine: %d", st)
	}
}
