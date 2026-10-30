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

	taskspb "cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
	"github.com/jackc/pgx/v4"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stripe/pg-schema-diff/pkg/diff"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
    space      uuid NOT NULL REFERENCES archive.space,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
%[2]s
);
ALTER TABLE %[1]s ENABLE ROW LEVEL SECURITY;
ALTER TABLE %[1]s FORCE ROW LEVEL SECURITY;
CREATE POLICY viewer ON %[1]s FOR SELECT USING (space = ANY (archive.my_spaces()));
CREATE POLICY editor ON %[1]s USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));
`, name, cols)
}

var e2eSchema = map[string]string{
	"company.sql": e2eTable("company", `    name       text NOT NULL,
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(name)) STORED`) + "COMMENT ON TABLE company IS 'A company';\n",
	"contact.sql": "CREATE DOMAIN zip AS text CHECK (VALUE ~ '^[0-9]{4} [A-Z]{2}$');\n" + e2eTable("contact", `    name       text NOT NULL,
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
    bsn        archive.nl_bsn,
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
	return &e2eInst{t: t, dir: dir, env: append(append([]string{"ARCHIVE_DATA=" + dir, "DATABASE_URL=host=" + dir + " user=archive dbname=archive"}, e2eCloudTasks.queue(t, dir)...), env...)}
}

type e2eTasks struct {
	taskspb.UnimplementedCloudTasksServer
	addr   string
	hold   bool
	issuer *httptest.Server
	grpc   *grpc.Server

	mu     sync.Mutex
	names  map[string]bool
	posted []*taskspb.Task
	routes map[string]*e2eRoute
}

type e2eRoute struct{ url, aud string }

const e2eTasksAccount = "tasks@e2e.iam.gserviceaccount.com"

func e2eTasksStart(hold bool) (*e2eTasks, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	issuer, err := e2eIssuerServer()
	if err != nil {
		ln.Close()
		return nil, err
	}
	f := &e2eTasks{addr: "http://" + ln.Addr().String(), hold: hold, issuer: issuer, grpc: grpc.NewServer(), names: map[string]bool{}, routes: map[string]*e2eRoute{}}
	taskspb.RegisterCloudTasksServer(f.grpc, f)
	go f.grpc.Serve(ln)
	return f, nil
}

func (f *e2eTasks) stop() {
	f.grpc.Stop()
	f.issuer.Close()
}

func (f *e2eTasks) env(queue string) []string {
	return []string{"ARCHIVE_TASKS_QUEUE=" + queue, "ARCHIVE_TASKS_SERVICE_ACCOUNT=" + e2eTasksAccount, "ARCHIVE_TASKS_URL=" + f.addr, "ARCHIVE_TASKS_ISSUER=" + f.issuer.URL}
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
	return f.env(q)
}

func (f *e2eTasks) route(queue, url, aud string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r := f.routes[queue]; r != nil {
		r.url, r.aud = url, aud
	}
}

func (f *e2eTasks) CreateTask(_ context.Context, req *taskspb.CreateTaskRequest) (*taskspb.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	tk := req.GetTask()
	if !strings.HasPrefix(tk.GetName(), req.GetParent()+"/tasks/") || tk.GetHttpRequest() == nil {
		return nil, status.Error(codes.InvalidArgument, "want a named HTTP task in the queue")
	}
	f.posted = append(f.posted, tk)
	if f.names[tk.GetName()] {
		return nil, status.Error(codes.AlreadyExists, "task exists")
	}
	f.names[tk.GetName()] = true
	if !f.hold {
		go f.deliver(req.GetParent(), tk)
	}
	return tk, nil
}

func (f *e2eTasks) deliver(queue string, tk *taskspb.Task) {
	time.Sleep(time.Until(tk.GetScheduleTime().AsTime()))
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
			if tok, err := f.token(map[string]any{"aud": to.aud, "sub": "1", "email": tk.GetHttpRequest().GetOidcToken().GetServiceAccountEmail(), "email_verified": true}); err == nil {
				req, _ := http.NewRequest("POST", to.url+"/tasks", bytes.NewReader(tk.GetHttpRequest().GetBody()))
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

func (f *e2eTasks) tasks() []*taskspb.Task {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.posted)
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
	if err := os.WriteFile(filepath.Join(dir, "secret.key"), []byte(hex.EncodeToString(key)+"\n"), 0o600); err != nil {
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
	key, err := os.ReadFile(filepath.Join(in.dir, "secret.key"))
	if err != nil {
		in.t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(in.dir, "blobs"), 0o700)
	if err := os.Symlink(filepath.Join(in.dir, "blobs"), filepath.Join(dir, "blobs")); err != nil {
		in.t.Fatal(err)
	}
	return &e2eInst{t: in.t, dir: dir, env: append(append([]string{"ARCHIVE_DATA=" + dir, "DATABASE_URL=host=" + in.dir + " user=archive dbname=archive",
		"ARCHIVE_SECRET_KEY=" + strings.TrimSpace(string(key))}, e2eCloudTasks.queue(in.t, dir)...), env...)}
}

func (in *e2eInst) on(t *testing.T) *e2eInst {
	c := *in
	c.t = t
	return &c
}

func (in *e2eInst) cmd(args ...string) *exec.Cmd {
	c := exec.Command(e2eBin, args...)
	c.Env = append(os.Environ(), in.env...)
	return c
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

func (in *e2eInst) push(files map[string]string, args ...string) string {
	in.t.Helper()
	return in.must(append([]string{"schema", "apply", in.writeDir("ddl", files)}, args...)...)
}

func (in *e2eInst) pulled() map[string]string {
	in.t.Helper()
	dir := filepath.Join(in.dir, "pulled")
	os.RemoveAll(dir)
	in.must("schema", "pull", dir)
	paths, _ := filepath.Glob(filepath.Join(dir, "*"))
	files := map[string]string{}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			in.t.Fatal(err)
		}
		files[filepath.Base(p)] = string(b)
	}
	return files
}

func (in *e2eInst) roundTrips() map[string]string {
	in.t.Helper()
	files := in.pulled()
	if p := decode[SchemaPlan](in.t, in.must("schema", "plan", in.writeDir("ddl", files))); len(p.Statements) != 0 {
		in.t.Errorf("applying the pulled schema plans %d statements: %+v", len(p.Statements), p.Statements)
	}
	return files
}

func (in *e2eInst) role(email string) string {
	return in.psql("archive", "SELECT w.id::text || '_u' || translate(rtrim(encode(uuid_send(u.id), 'base64'), '='), '+/', '-_') FROM archive.\"user\" u, archive.workspace w WHERE u.email = '"+email+"' AND w.id = (SELECT id FROM archive.workspace ORDER BY created_at LIMIT 1)")
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
	queue, api := in.getenv("ARCHIVE_TASKS_QUEUE"), cmp.Or(in.getenv("ARCHIVE_API_URL"), "http://localhost:"+strings.TrimPrefix(addr, "127.0.0.1:"))
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
		if resp, err := http.Get(s.url + "/healthz"); err == nil {
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

func TestSchemaPushNormalizesMessyDutchInputAndBlocksDrops(t *testing.T) {
	t.Parallel()
	in := newInst(t, "ARCHIVE_LOCALE=nl-NL")
	in.fails("list", "contact")
	in.must("user", "add", "ann@example.com", "-name", "Ann")
	plan := decode[SchemaPlan](t, in.push(e2eSchema))
	if !plan.Applied || !slices.ContainsFunc(plan.Statements, func(s SchemaStatement) bool { return strings.Contains(s.DDL, `CREATE TABLE "public"."invoice"`) }) {
		t.Fatalf("schema push: %+v", plan)
	}
	if files := in.roundTrips(); !slices.Equal(slices.Sorted(maps.Keys(files)), slices.Sorted(maps.Keys(e2eSchema))) || !strings.Contains(files["invoice.sql"], "CREATE TYPE \"public\".\"invoice_status\"") ||
		!strings.Contains(files["contact.sql"], "CREATE DOMAIN \"public\".\"zip\"") || !strings.Contains(files["person.sql"], "REFERENCES company(id)") {
		t.Errorf("schema pull: %v", files)
	}
	extra := map[string]string{"zz.sql": `CREATE INDEX person_name ON person (lower(name)) WHERE name <> '';
CREATE FUNCTION touch() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN NEW.updated_at = now(); RETURN NEW; END $$;
CREATE TRIGGER person_touch BEFORE UPDATE ON person FOR EACH ROW EXECUTE FUNCTION touch();
CREATE TYPE mood AS ENUM ('good', 'bad');
COMMENT ON TABLE person IS 'Someone''s record';
COMMENT ON COLUMN company.name IS 'Legal name';`}
	in.push(with(e2eSchema, "zz.sql", extra["zz.sql"]))
	files := in.roundTrips()
	if _, ok := files["zz.sql"]; ok || !strings.Contains(files["person.sql"], "EXECUTE FUNCTION touch()") || !strings.Contains(files["person.sql"], "FUNCTION public.touch()") ||
		!strings.Contains(files["person.sql"], "person_name") || !strings.Contains(files["person.sql"], `'Someone''s record'`) || !strings.Contains(files["company.sql"], "'Legal name'") ||
		!strings.Contains(files["mood.sql"], "CREATE TYPE") || strings.Contains(files["person.sql"], "OWNER TO") || strings.Contains(files["person.sql"], "GRANT") {
		t.Errorf("schema pull with an index, trigger, unused enum and comments: %v", files)
	}
	in.push(e2eSchema)

	r := decode[row](t, in.must("create", "contact", `{"name":" Ann ","email":"Ann@EXAMPLE.com","phone":"06 12345678","birthday":"3-4-1990","balance":"€ 1.234,50","tags":"vip","zip":"1012 LG"}`))
	want := map[string]string{"phone": "+31612345678", "birthday": "1990-04-03", "email": "Ann@example.com", "zip": "1012 LG",
		"balance": "map[amount:1234.50 currency:EUR]", "tags": "[vip]"}
	for k, v := range want {
		if fmt.Sprint(r[k]) != v {
			t.Errorf("%s = %v, want %v", k, r[k], v)
		}
	}
	if r["space"] == nil || r["created_at"] == nil || r["search"] != nil {
		t.Errorf("row columns: %v", r)
	}

	errs := in.fails("create", "contact", `{"name":"Bad","email":"not an email","phone":"12","nope":1}`)
	if !strings.Contains(errs, "invalid contact") || !strings.Contains(errs, "email:") || !strings.Contains(errs, "phone:") || !strings.Contains(errs, "nope: unknown field") {
		t.Errorf("validation errors: %s", errs)
	}
	if errs := in.fails("create", "contact", `{"email":"a@b.nl"}`); !strings.Contains(errs, "name: required") {
		t.Errorf("required: %s", errs)
	}
	if errs := in.fails("create", "contact", `{"name":"Pat","zip":"1012"}`); !strings.Contains(errs, `zip: value for domain zip violates check constraint "zip_check"`) {
		t.Errorf("domain check from Postgres: %s", errs)
	}
	if errs := in.fails("create", "invoice", `{"number":"X","status":"lost"}`); !strings.Contains(errs, "status: must be one of draft, sent, paid") {
		t.Errorf("enum: %s", errs)
	}
	if got := decode[row](t, in.must("create", "invoice", `{"number":"X","status":"PAID"}`)); got["status"] != "paid" {
		t.Errorf("enum case: %v", got)
	}

	if got := decode[[]row](t, in.must("list", "contact", "phone=06-1234 5678")); len(got) != 1 || id(got[0]) != id(r) {
		t.Errorf("filter on messy phone: %v", got)
	}
	if got := decode[[]row](t, in.must("list", "contact", "phone=0687654321")); len(got) != 0 {
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

	noTags := with(e2eSchema, "contact.sql", strings.Replace(e2eSchema["contact.sql"], "    tags       text[],\n", "", 1))
	out, errs, err := in.run("", "schema", "apply", in.writeDir("ddl", noTags))
	if err == nil || !strings.Contains(errs, "deletes data") || !strings.Contains(errs, `DROP COLUMN "tags"`) {
		t.Fatalf("drop without -allow-drop: %v %s%s", err, out, errs)
	}
	if pulled := in.must("schema", "pull"); !strings.Contains(pulled, "tags") {
		t.Errorf("a blocked apply changed the schema")
	}
	plan = decode[SchemaPlan](t, in.push(noTags, "-allow-drop"))
	if !plan.Applied || len(plan.Statements) != 1 || plan.Statements[0].Hazards[0].Type != "DELETES_DATA" {
		t.Errorf("allowed drop: %+v", plan)
	}
	if got := decode[row](t, in.must("get", "contact", id(r))); got["tags"] != nil || got["name"] != "Ann B" {
		t.Errorf("after the drop: %v", got)
	}

	bad := map[string]string{"bad.sql": "CREATE TABLE loose (id uuid PRIMARY KEY, name text);"}
	if errs := in.fails("schema", "apply", in.writeDir("bad", with(noTags, "bad.sql", bad["bad.sql"]))); !strings.Contains(errs, "loose.space: want space uuid NOT NULL REFERENCES archive.space") ||
		!strings.Contains(errs, "ROW LEVEL SECURITY") || !strings.Contains(errs, "loose.search") {
		t.Errorf("table without the required columns: %s", errs)
	}
	if errs := in.fails("schema", "apply", in.writeDir("bad", with(noTags, "bad.sql", "CREATE TABLE oops ("))); !strings.Contains(errs, "bad.sql:") {
		t.Errorf("broken DDL: %s", errs)
	}
	if n := in.psql("archive", "SELECT count(*) FROM pg_tables WHERE tablename IN ('loose', 'oops')"); n != "0" {
		t.Errorf("%s rejected tables created", n)
	}
}

func TestDemoVaultLoadsOnceWithLinkedRowsAndFiles(t *testing.T) {
	t.Parallel()
	in := newInst(t)
	load := func() (string, string, error) {
		c := exec.Command("../demo/load.sh", "ann@example.com")
		c.Env = append(os.Environ(), append(in.env, "ARCHIVE_BIN="+e2eBin)...)
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
	if spaces := in.must("space", "list"); !strings.Contains(spaces, `"family"`) {
		t.Errorf("family space missing: %s", spaces)
	}
	res, _, err := in.run(`return { files: (await archive.files({ limit: 100 })).length,
  receipts: (await archive.sql("SELECT count(*)::int AS n FROM payment WHERE receipt IS NOT NULL"))[0].n };`, "run", "-")
	if err != nil {
		t.Fatalf("run: %v %s", err, res)
	}
	got := decode[struct{ Result map[string]any }](t, res).Result
	if fmt.Sprint(got["files"]) != "35" || fmt.Sprint(got["receipts"]) == "0" {
		t.Errorf("files and links: %v", got)
	}
	if files := in.roundTrips(); len(files) != 25 || !strings.Contains(files["task_board.sql"], "by: status") || !strings.Contains(files["payment_card.sql"], `"number" archive.card`) || !strings.Contains(files["payment_card.sql"], "REFERENCES person(id)") {
		t.Errorf("demo schema pull: %v", slices.Sorted(maps.Keys(files)))
	}
	if _, errs, err := load(); err == nil || !strings.Contains(errs, "already has people") {
		t.Errorf("second load: %v %s", err, errs)
	}
}

func TestUsersSpacesTokensAndSecretsAcrossEverySurface(t *testing.T) {
	t.Parallel()
	in := newInst(t, append(e2eGoogle(t), "ANTHROPIC_API_KEY=")...)
	in.must("user", "add", "alice@example.com")
	srv := in.serve()
	api := srv.url + "/api"

	added := decode[User](t, in.must("user", "add", "bob@example.com", "-name", "Bob"))
	in.push(e2eSchema)
	in.must("space", "add", "family")
	in.must("space", "grant", "family", "bob@example.com", "viewer")
	aliceTok := decode[Token](t, in.must("token", "create", "-name", "Claude")).Secret
	narrowTok := decode[Token](t, in.must("token", "create", "-name", "Narrow", "-space", "family:viewer")).Secret
	bobTok := decode[Token](t, in.must("-as", "bob@example.com", "token", "create")).Secret

	var alicePriv, bobPriv, acme, fam row
	st, _ := call(t, "POST", api+"/person", aliceTok, map[string]any{"name": "Alice private"}, &alicePriv)
	wantStatus(t, st, 201, "alice creates in her space")
	st, _ = call(t, "POST", api+"/person", bobTok, map[string]any{"name": "Bob private"}, &bobPriv)
	wantStatus(t, st, 201, "bob creates in his space")
	call(t, "POST", api+"/company", aliceTok, map[string]any{"name": "Acme"}, &acme)
	st, _ = call(t, "POST", api+"/person?space=family", aliceTok, map[string]any{"name": "Family member", "ssn": "123-45-6789", "employer": id(acme), "friends": []string{id(alicePriv)}}, &fam)
	wantStatus(t, st, 201, "alice creates in family")

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

	t.Run("UsersOnlySeeTheirOwnSpacesThroughAPI", func(t *testing.T) {
		var a, b []row
		call(t, "GET", api+"/person", aliceTok, nil, &a)
		call(t, "GET", api+"/person", bobTok, nil, &b)
		if names(a) != "Alice private,Family member" || names(b) != "Bob private,Family member" {
			t.Fatalf("alice sees %q, bob sees %q", names(a), names(b))
		}
		st, _ := call(t, "GET", api+"/person/"+id(alicePriv), bobTok, nil, nil)
		wantStatus(t, st, 404, "bob reads alice's private row")
		st, _ = call(t, "DELETE", api+"/person/"+id(bobPriv), aliceTok, nil, nil)
		wantStatus(t, st, 404, "alice deletes bob's private row")
		st, _ = call(t, "GET", api+"/nothing", aliceTok, nil, nil)
		wantStatus(t, st, 404, "unknown table")
	})

	t.Run("UsersOnlySeeTheirOwnSpacesThroughCLIWhileServeRuns", func(t *testing.T) {
		in := in.on(t)
		b := names(decode[[]row](t, in.must("-as", "bob@example.com", "list", "person")))
		a := names(decode[[]row](t, in.must("list", "person")))
		if a != "Alice private,Family member" || b != "Bob private,Family member" {
			t.Fatalf("cli alice %q, bob %q", a, b)
		}
	})

	t.Run("PlainPsqlAsTheUserRoleSeesWhatTheAppSees", func(t *testing.T) {
		in := in.on(t)
		role := in.role("bob@example.com")
		in.psql("archive", "ALTER ROLE \""+role+"\" LOGIN")
		defer in.psql("archive", "ALTER ROLE \""+role+"\" NOLOGIN")
		if got := in.psql(role, "SELECT string_agg(name, ',' ORDER BY id) FROM person"); got != "Bob private,Family member" {
			t.Fatalf("psql as bob: %q", got)
		}
		if n := in.psql(role, "SELECT count(*) FROM archive.secret"); n != "0" {
			t.Errorf("psql as bob sees %s secrets", n)
		}
		out, err := exec.Command("psql", "-X", "-h", in.dir, "-U", role, "-d", "archive", "-Atc", "INSERT INTO person (name, space) SELECT 'x', id FROM archive.space WHERE name = 'family'").CombinedOutput()
		if err == nil || !strings.Contains(string(out), "row-level security") {
			t.Errorf("psql insert into family as viewer: %v %s", err, out)
		}
		out, err = exec.Command("psql", "-X", "-h", in.dir, "-U", role, "-d", "archive", "-Atc", "INSERT INTO contact (name, email, space) SELECT 'x', 'not-an-email', id FROM archive.space").CombinedOutput()
		if err == nil || !strings.Contains(string(out), "email_check") {
			t.Errorf("psql insert of a bad email: %v %s", err, out)
		}
	})

	t.Run("ViewerCannotWrite", func(t *testing.T) {
		in := in.on(t)
		st, _ := call(t, "POST", api+"/person?space=family", bobTok, map[string]any{"name": "Intruder"}, nil)
		wantStatus(t, st, 403, "viewer creates")
		st, _ = call(t, "PATCH", api+"/person/"+id(fam), bobTok, map[string]any{"name": "Changed"}, nil)
		wantStatus(t, st, 403, "viewer patches")
		st, _ = call(t, "DELETE", api+"/person/"+id(fam), bobTok, nil, nil)
		wantStatus(t, st, 403, "viewer deletes")
		in.fails("-as", "bob@example.com", "create", "person", "-space", "family", `{"name":"x"}`)
	})

	t.Run("NarrowedTokenSeesOnlyItsSpace", func(t *testing.T) {
		in := in.on(t)
		var rs []row
		call(t, "GET", api+"/person", narrowTok, nil, &rs)
		if names(rs) != "Family member" {
			t.Fatalf("narrow token sees %q", names(rs))
		}
		st, _ := call(t, "POST", api+"/person", narrowTok, map[string]any{"name": "x"}, nil)
		wantStatus(t, st, 403, "narrow viewer token creates")
		var me struct {
			Spaces []SpaceInfo `json:"spaces"`
		}
		call(t, "GET", srv.url+"/auth/me", narrowTok, nil, &me)
		if len(me.Spaces) != 1 || me.Spaces[0].Name != "family" || me.Spaces[0].Level != "viewer" {
			t.Errorf("narrow token spaces: %+v", me.Spaces)
		}
		st, _ = call(t, "POST", srv.url+"/auth/tokens", narrowTok, map[string]any{"spaces": map[string]string{"family": "owner"}}, nil)
		wantStatus(t, st, 403, "token mints a wider token")
		st, _ = call(t, "POST", srv.url+"/auth/tokens", bobTok, nil, nil)
		wantStatus(t, st, 400, "token endpoint without a body")
		in.fails("-as", "bob@example.com", "token", "create", "-space", "family:editor")
	})

	t.Run("ArchiveSpaceHeaderScopesARequestToOneSpace", func(t *testing.T) {
		in := in.on(t)
		scoped := func(method, path, tok, space string, body, out any) int {
			t.Helper()
			b, _ := json.Marshal(body)
			req, _ := http.NewRequest(method, srv.url+path, bytes.NewReader(b))
			req.Header.Set("Authorization", "Bearer "+tok)
			req.Header.Set("Archive-Space", space)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if out != nil {
				json.NewDecoder(resp.Body).Decode(out)
			}
			return resp.StatusCode
		}
		home := ""
		for _, sp := range decode[[]SpaceInfo](t, in.must("space", "list")) {
			if sp.Name != "family" {
				home = sp.Name
			}
		}
		for _, c := range []struct{ tok, space, want string }{
			{aliceTok, "family", "Family member"}, {aliceTok, home, "Alice private"}, {bobTok, "family", "Family member"},
			{bobTok, home, ""}, {aliceTok, "nope", ""}, {aliceTok, "", "Alice private,Family member"},
		} {
			var rs []row
			if st := scoped("GET", "/api/person", c.tok, c.space, nil, &rs); st != 200 || names(rs) != c.want {
				t.Errorf("space %q: %d %q, want %q", c.space, st, names(rs), c.want)
			}
		}
		var out RunOutput
		scoped("POST", "/api/run", aliceTok, "family", map[string]string{"code": `return { people: (await archive.list("person")).map(p => p.name), spaces: (await archive.spaces()).length }`}, &out)
		if string(out.Result) != `{"people":["Family member"],"spaces":2}` {
			t.Errorf("scoped run: %s %s", out.Result, out.Error)
		}
		var co row
		if st := scoped("POST", "/api/company", aliceTok, "family", map[string]string{"name": "Scoped Co"}, &co); st != 201 || co["space"] != fam["space"] {
			t.Errorf("create in the header's space: %d %v, want space %v", st, co, fam["space"])
		}
		if st := scoped("POST", "/api/company", bobTok, home, map[string]string{"name": "Intruder Co"}, nil); st < 400 {
			t.Errorf("bob creates in alice's space through the header: %d", st)
		}
		if st := scoped("DELETE", "/api/company/"+id(co), aliceTok, "family", nil, nil); st != 204 {
			t.Errorf("delete in the header's space: %d", st)
		}
		var sp []SpaceInfo
		scoped("GET", "/auth/spaces", aliceTok, "family", nil, &sp)
		if len(sp) != 2 {
			t.Errorf("the spaces list ignores the header: %+v", sp)
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
		st, _ = call(t, "POST", api+"/person", bobTok, map[string]any{"name": "Sneaky", "employer": id(acme), "friends": []string{id(fam), id(alicePriv)}}, &ve)
		wantStatus(t, st, 422, "bob references alice's company")
		if len(ve.Errors) != 2 || ve.Errors[0].Path != "employer" || ve.Errors[1].Path != "friends[1]" || !strings.HasPrefix(ve.Errors[1].Message, "no person") {
			t.Errorf("reference errors: %+v", ve)
		}
		st, _ = call(t, "DELETE", api+"/company/"+id(acme), aliceTok, nil, nil)
		wantStatus(t, st, 409, "delete a referenced company")
	})

	t.Run("SecretIsMaskedAndUnreadableWithoutGrant", func(t *testing.T) {
		in := in.on(t)
		ssn := func(tok string) string {
			var r row
			call(t, "GET", api+"/person/"+id(fam)+"?reveal=true", tok, nil, &r)
			return fmt.Sprint(r["ssn"])
		}
		var plain row
		call(t, "GET", api+"/person/"+id(fam), aliceTok, nil, &plain)
		if plain["ssn"] != "***-**-6789" {
			t.Fatalf("default ssn = %v", plain["ssn"])
		}
		if s := ssn(aliceTok); strings.Contains(s, "123456789") {
			t.Fatalf("alice reveals without grant: %s", s)
		}
		if n := in.psql("archive", "SELECT count(*) FROM archive.secret WHERE position(convert_to('123456789', 'UTF8') in value) > 0"); n != "0" {
			t.Fatalf("secret stored in plain text")
		}
		if n := in.psql("archive", "SELECT count(*) FROM person WHERE to_jsonb(person)::text LIKE '%123456789%'"); n != "0" {
			t.Fatalf("secret leaked into the table")
		}
		in.fails("-as", "bob@example.com", "secret", "grant", "bob@example.com", "us_ssn")
		in.fails("secret", "grant", "alice@example.com", "email")
		in.must("secret", "grant", "alice@example.com", "us_ssn")
		if s := in.must("get", "person", id(fam), "-reveal"); !strings.Contains(s, `"ssn": "123456789"`) {
			t.Errorf("cli reveal: %s", s)
		}
		if s := ssn(aliceTok); strings.Contains(s, "123456789") {
			t.Errorf("token reveals although tokens get no secret grants: %s", s)
		}
		if s := ssn(bobTok); strings.Contains(s, "123456789") {
			t.Fatalf("bob reveals: %s", s)
		}
		call(t, "PATCH", api+"/person/"+id(fam), aliceTok, map[string]any{"ssn": "078-05-1120"}, nil)
		if n := in.psql("archive", "SELECT count(*) FROM archive.secret WHERE row_id = '"+id(fam)+"'"); n != "1" {
			t.Errorf("old secret not replaced: %s rows", n)
		}
		var kept row
		call(t, "GET", api+"/person/"+id(fam), aliceTok, nil, &kept)
		st, _ := call(t, "PUT", api+"/person/"+id(fam), aliceTok, kept, nil)
		wantStatus(t, st, 200, "put the row back with its mask")
		if s := in.must("get", "person", id(fam), "-reveal"); !strings.Contains(s, `"ssn": "078051120"`) {
			t.Errorf("mask round trip lost the secret: %s", s)
		}
		st, _ = call(t, "PATCH", api+"/person/"+id(fam), aliceTok, map[string]any{"ssn": "***-**-0000"}, nil)
		wantStatus(t, st, 422, "a mask that is not the current one")
		call(t, "PATCH", api+"/person/"+id(fam), aliceTok, map[string]any{"ssn": nil}, nil)
		if n := in.psql("archive", "SELECT count(*) FROM archive.secret WHERE row_id = '"+id(fam)+"'"); n != "0" {
			t.Errorf("cleared secret kept: %s rows", n)
		}
	})

	t.Run("SchemaChangesNeedAdmin", func(t *testing.T) {
		files := with(e2eSchema, "company.sql", strings.Replace(e2eSchema["company.sql"], "    name       text NOT NULL,\n", "    name       text NOT NULL,\n    vat        archive.vat_id,\n", 1))
		var plan SchemaPlan
		st, _ := call(t, "POST", api+"/schema/plan", narrowTok, map[string]any{"files": files}, &plan)
		wantStatus(t, st, 200, "viewer token plans")
		if len(plan.Statements) != 1 || !strings.Contains(plan.Statements[0].DDL, `ADD COLUMN "vat" archive.vat_id`) || plan.Applied {
			t.Fatalf("plan: %+v", plan)
		}
		st, _ = call(t, "POST", api+"/schema/apply", narrowTok, map[string]any{"files": files}, nil)
		wantStatus(t, st, 403, "viewer token applies")
		st, _ = call(t, "POST", api+"/schema/apply", aliceTok, map[string]any{"files": files}, &plan)
		wantStatus(t, st, 200, "admin applies")
		var cat Catalog
		call(t, "GET", api+"/types", bobTok, nil, &cat)
		if c := cat.Tables["company"]; c == nil || c.col("vat") == nil || c.col("vat").Type != "vat_id" || c.Description != "A company" || cat.Types["archive.email"] == nil {
			t.Fatalf("catalog after apply: %+v", cat.Tables["company"])
		}
		var got struct {
			Files map[string]string `json:"files"`
		}
		call(t, "GET", api+"/schema", bobTok, nil, &got)
		if !strings.Contains(got.Files["company.sql"], "vat") || len(got.Files) != 4 {
			t.Errorf("schema files: %v", got.Files)
		}
		st, _ = call(t, "POST", api+"/company", aliceTok, map[string]any{"name": "Vat Co", "vat": "nl 123456789 b01"}, nil)
		wantStatus(t, st, 201, "use the new column")
		if out := in.on(t).must("list", "company", "vat=NL123456789B01"); !strings.Contains(out, "Vat Co") {
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
			Actor Actor `json:"actor"`
		}
		call(t, "GET", srv.url+"/auth/me", "", nil, &me, session)
		if me.Actor.Email != "bob@example.com" || me.Actor.Name != "Bob" {
			t.Fatalf("me = %+v", me.Actor)
		}
		var rs []row
		call(t, "GET", api+"/person", "", nil, &rs, session)
		if !strings.Contains(names(rs), "Bob private,Family member") {
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

	t.Run("MembershipAndTokenEndpoints", func(t *testing.T) {
		in := in.on(t)
		var ms []Member
		st, _ := call(t, "POST", srv.url+"/auth/spaces/family/members", bobTok, map[string]any{"email": "bob@example.com", "role": "owner"}, nil)
		wantStatus(t, st, 403, "token changes membership")
		in.must("space", "grant", "family", "bob@example.com", "editor")
		bobNarrow := decode[Token](t, in.must("-as", "bob@example.com", "token", "create", "-space", "family:editor"))
		st, _ = call(t, "POST", api+"/person?space=family", bobNarrow.Secret, map[string]any{"name": "Bob in family"}, nil)
		wantStatus(t, st, 201, "editor token creates")
		ms = decode[[]Member](t, in.must("space", "grant", "family", "bob@example.com", "none"))
		if len(ms) != 1 || ms[0].Email != "alice@example.com" {
			t.Errorf("members after removing bob: %+v", ms)
		}
		var left []row
		st, _ = call(t, "GET", api+"/person", bobNarrow.Secret, nil, &left)
		wantStatus(t, st, 200, "token whose user left its only space")
		if len(left) != 0 {
			t.Errorf("token whose user left its only space sees %q", names(left))
		}
		in.fails("space", "grant", "family", "alice@example.com", "editor")
		in.must("-as", "bob@example.com", "token", "revoke", bobNarrow.ID.String())
		st, _ = call(t, "GET", api+"/person", bobNarrow.Secret, nil, nil)
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

func TestDeletingASpaceRemovesEverythingInItForOwnersOnly(t *testing.T) {
	t.Parallel()
	in := newInst(t)
	in.must("user", "add", "alice@example.com")
	srv := in.serve()
	in.must("user", "add", "bob@example.com", "-name", "Bob")
	in.push(e2eSchema)
	in.must("space", "add", "team")
	in.must("space", "add", "scratch")
	in.must("space", "grant", "team", "bob@example.com", "editor")
	aliceTok := decode[Token](t, in.must("token", "create", "-name", "Claude")).Secret
	full := decode[Token](t, in.must("token", "create", "-name", "Terraform", "-full")).Secret

	acme := decode[row](t, in.must("create", "company", `{"name": "Acme"}`))
	teamCo := decode[row](t, in.must("create", "company", "-space", "team", `{"name": "Team co"}`))
	member := decode[row](t, in.must("create", "person", "-space", "team", `{"name": "Team member", "ssn": "123-45-6789", "employer": "`+id(teamCo)+`"}`))
	errs := in.fails("-as", "bob@example.com", "space", "remove", "team")
	if !strings.Contains(errs, "needs owner") {
		t.Errorf("editor removes team: %s", errs)
	}
	errs = in.fails("space", "remove", "alice_example_com")
	if !strings.Contains(errs, "is a personal space") {
		t.Errorf("owner removes her personal space: %s", errs)
	}
	errs = in.fails("-as", "bob@example.com", "space", "remove", "scratch")
	if !strings.Contains(errs, "not found") {
		t.Errorf("non-member removes scratch: %s", errs)
	}
	st, _ := call(t, "DELETE", srv.url+"/auth/spaces/team", aliceTok, nil, nil)
	wantStatus(t, st, 403, "scoped token deletes a space")

	spaces := decode[[]SpaceInfo](t, in.must("space", "remove", "team"))
	for _, s := range spaces {
		if s.Name == "team" {
			t.Fatalf("team still listed: %+v", spaces)
		}
	}
	if got := in.psql("archive", "SELECT (SELECT count(*) FROM person WHERE id = '"+id(member)+"') + (SELECT count(*) FROM company WHERE id = '"+id(teamCo)+"')"); got != "0" {
		t.Errorf("team rows survived: %s", got)
	}
	if got := in.psql("archive", "SELECT count(*) FROM archive.secret"); got != "0" {
		t.Errorf("secrets left: %s", got)
	}
	if got := in.psql("archive", "SELECT count(*) FROM pg_roles WHERE rolname LIKE 'a\\_s\\_team\\_%'"); got != "0" {
		t.Errorf("team roles left: %s", got)
	}
	if got := decode[row](t, in.must("get", "company", id(acme))); id(got) != id(acme) {
		t.Errorf("company outside team: %+v", got)
	}

	st, _ = call(t, "DELETE", srv.url+"/auth/spaces/scratch", full, nil, nil)
	wantStatus(t, st, 204, "full token deletes scratch")
	st, _ = call(t, "DELETE", srv.url+"/auth/spaces/scratch", full, nil, nil)
	wantStatus(t, st, 404, "delete scratch twice")
	in.must("space", "add", "team")
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
	var spaces []SpaceInfo
	call(t, "GET", srv.url+"/auth/spaces", "", nil, &spaces, c)
	if len(spaces) != 1 || spaces[0].Level != "owner" {
		t.Errorf("spaces: %+v", spaces)
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
	in.must("user", "add", "alice@example.com")
	srv := in.serve()
	api := srv.url + "/api"
	in.must("user", "add", "bob@example.com")
	in.push(e2eSchema)
	aliceTok := decode[Token](t, in.must("token", "create", "-name", "Claude")).Secret
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

		meta := "filename " + base64.StdEncoding.EncodeToString([]byte("big.txt"))
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
		st, again := e2eUpload(t, api+"/files", aliceTok, "big.txt", big)
		wantStatus(t, st, 201, "upload the same file again")
		_, copied := e2eUpload(t, api+"/files", aliceTok, "copy.txt", big)
		_, bobs := e2eUpload(t, api+"/files", bobTok, "big.txt", big)
		if again.ID != bigFile.ID || copied.ID == bigFile.ID || bobs.ID == bigFile.ID || copied.Hash != bigFile.Hash || bobs.Hash != bigFile.Hash {
			t.Fatalf("same name %s, copy %s, bob %s, first %s", again.ID, copied.ID, bobs.ID, bigFile.ID)
		}
		blobs, _ := filepath.Glob(filepath.Join(in.dir, "blobs", "*", "*", bigFile.Hash))
		if n := in.psql("archive", "SELECT count(*) FROM archive.file WHERE hash = '"+bigFile.Hash+"'"); n != "3" || len(blobs) != 1 {
			t.Errorf("%s file rows, %d blobs", n, len(blobs))
		}
		parts, _ := os.ReadDir(filepath.Join(in.dir, "blobs", "uploads"))
		if n := in.psql("archive", "SELECT count(*) FROM archive.upload"); n != "0" || len(parts) != 0 {
			t.Errorf("uploads left behind: %s rows, %d part directories", n, len(parts))
		}
	})

	photo := e2eJPEG(t)
	var photoFile File
	t.Run("PhotoMetadataThumbnailAndLocationMarkedPersonal", func(t *testing.T) {
		st, f := e2eUpload(t, api+"/files", aliceTok, "photo.jpg", photo)
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
		st, f := e2eUpload(t, api+"/files", aliceTok, "invoice.pdf", e2ePDF("Cover letter", "Invoice INV-9 total due"))
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
		st, _ = call(t, "POST", api+"/invoice", aliceTok, map[string]any{"number": "INV-9", "document": f.ID.String(), "total": "EUR 12.50"}, &inv)
		wantStatus(t, st, 201, "invoice that references its file")
		if inv["document"] != f.ID.String() {
			t.Errorf("invoice: %v", inv)
		}
		_, bobs := e2eUpload(t, api+"/files", bobTok, "bob.txt", []byte("bob"))
		var ve ValidationError
		st, _ = call(t, "POST", api+"/invoice", aliceTok, map[string]any{"number": "INV-10", "document": bobs.ID.String()}, &ve)
		wantStatus(t, st, 422, "invoice that references bob's file")
		if len(ve.Errors) != 1 || ve.Errors[0].Path != "document" || !strings.HasPrefix(ve.Errors[0].Message, "no file") {
			t.Errorf("reference to a hidden file: %+v", ve)
		}

		_, page := e2eUpload(t, api+"/files", aliceTok, "order.html", []byte("<html><head><style>p{}</style><script>var INV = 1</script></head><body><p>Order</p><p>INV-5</p></body></html>"))
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
		if r.URL.Path == "/computeMetadata/v1/instance/service-accounts/default/token" && r.Header.Get("Metadata-Flavor") == "Google" {
			json.NewEncoder(w).Encode(map[string]any{"access_token": "meta", "expires_in": 3600, "token_type": "Bearer"})
			return
		}
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
	in := newInst(t, "ARCHIVE_BLOB_URL=gs://photos/archive?endpoint="+url.QueryEscape(fake.URL), "GCE_METADATA_HOST="+strings.TrimPrefix(fake.URL, "http://"))
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
	if _, err := os.Stat(filepath.Join(in.dir, "blobs")); err == nil {
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
		var schema struct {
			Files map[string]string `json:"files"`
		}
		call(t, "GET", srvB.url+"/api/schema", tok, nil, &schema)
		if got := slices.Sorted(maps.Keys(schema.Files)); !slices.Equal(got, slices.Sorted(maps.Keys(e2eSchema))) {
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
		st, _ = call(t, "POST", srvA.url+"/api/schema/apply", tok, map[string]any{"files": with(schema.Files, "note.sql", e2eNote)}, nil)
		wantStatus(t, st, 200, "apply note on a")
		call(t, "GET", srvB.url+"/api/schema", tok, nil, &schema)
		if !strings.Contains(schema.Files["note.sql"], "CREATE INDEX note_search ON public.note USING gin (search)") {
			t.Errorf("note.sql on b: %v", schema.Files)
		}
		st, _ = call(t, "POST", srvB.url+"/api/note", tok, map[string]any{"title": "Shared"}, nil)
		wantStatus(t, st, 201, "create on b in a table applied on a")
	})

	t.Run("AFreshContainerKeepsTheSchema", func(t *testing.T) {
		c := a.twin()
		if files := c.roundTrips(); !strings.Contains(files["invoice.sql"], "invoice_status") {
			t.Fatalf("pull on a fresh container: %v", files)
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
		parts, _ := os.ReadDir(filepath.Join(a.dir, "blobs", "uploads"))
		if n := a.psql("archive", "SELECT count(*) FROM archive.upload"); n != "0" || len(parts) != 0 {
			t.Errorf("uploads left behind: %s rows, %d part directories", n, len(parts))
		}
	})
}

func TestTablesThatReferenceEachOtherRoundTripAndRecreate(t *testing.T) {
	t.Parallel()
	cols := func(refs string) string {
		return "    name       text NOT NULL,\n" + refs + "    search     tsvector GENERATED ALWAYS AS (archive.search_doc(name)) STORED"
	}
	cyclic := map[string]string{
		"alpha.sql": e2eTable("alpha", cols("    beta       uuid,\n")) + `ALTER TABLE alpha ADD FOREIGN KEY (beta) REFERENCES beta;
-- the name; trimmed
CREATE FUNCTION alpha_trim() RETURNS trigger LANGUAGE plpgsql AS $body$ BEGIN NEW.name = trim(NEW.name); RETURN NEW; END $body$;
CREATE TRIGGER alpha_trim BEFORE INSERT ON alpha FOR EACH ROW EXECUTE FUNCTION alpha_trim();
`,
		"beta.sql":  e2eTable("beta", cols("    alpha      uuid REFERENCES alpha,\n    gamma      uuid,\n")) + "ALTER TABLE beta ADD FOREIGN KEY (gamma) REFERENCES gamma;\n",
		"gamma.sql": e2eTable("gamma", cols("    alpha      uuid,\n")) + "ALTER TABLE gamma ADD FOREIGN KEY (alpha) REFERENCES alpha;\nCOMMENT ON TABLE gamma IS E'one; two \\' three';\n",
	}
	in := newInst(t)
	in.must("user", "add", "ann@example.com")
	in.push(cyclic)
	a := decode[row](t, in.must("create", "alpha", `{"name":" A "}`))
	in.must("create", "beta", `{"name":"B","alpha":"`+id(a)+`"}`)
	files := in.roundTrips()
	if !slices.Equal(slices.Sorted(maps.Keys(files)), []string{"alpha.sql", "beta.sql", "gamma.sql"}) || !strings.Contains(files["alpha.sql"], "REFERENCES beta(id)") ||
		!strings.Contains(files["gamma.sql"], "REFERENCES alpha(id)") || !strings.Contains(files["gamma.sql"], "one; two '' three") || a["name"] != "A" {
		t.Errorf("cyclic schema pull: %v %v", files, a)
	}
	if p := decode[SchemaPlan](t, in.must("schema", "plan", in.writeDir("ddl", cyclic))); len(p.Statements) != 0 {
		t.Errorf("the hand-written files plan %+v", p.Statements)
	}

	fresh := newInst(t)
	fresh.must("user", "add", "ann@example.com")
	fresh.push(files)
	if got := fresh.roundTrips(); !maps.Equal(got, files) {
		t.Errorf("a fresh instance from the generated files pulls %v", got)
	}
	if got := fresh.psql("archive", "SELECT count(*) FROM pg_constraint WHERE contype = 'f' AND conrelid::regclass::text IN ('alpha', 'beta', 'gamma') AND confrelid <> 'archive.space'::regclass"); got != "4" {
		t.Errorf("%s foreign keys between the tables on the fresh instance", got)
	}
	if errs := in.fails("schema", "plan", in.writeDir("ddl", with(cyclic, "gamma.sql", strings.Replace(cyclic["gamma.sql"], "REFERENCES alpha", "REFERENCES delta", 1)))); !strings.Contains(errs, "gamma.sql:") || !strings.Contains(errs, `"delta" does not exist`) {
		t.Errorf("a reference to a missing table: %s", errs)
	}
}

var e2eNote = e2eTable("note", `    title      text NOT NULL,
    body       text,
    bsn        archive.nl_bsn,
    search     tsvector GENERATED ALWAYS AS (setweight(archive.search_doc(title), 'A') || setweight(archive.search_doc(body, bsn), 'B')) STORED`) +
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

func TestFullTextSearchInDutchAndEnglishAcrossTablesFilesAndUsers(t *testing.T) {
	t.Parallel()
	in := newInst(t)
	in.must("user", "add", "alice@example.com")
	srv := in.serve()
	api := srv.url + "/api"
	in.must("user", "add", "bob@example.com")
	files := with(e2eSchema, "note.sql", e2eNote+"CREATE INDEX note_title ON note (title);\n")
	in.push(files)
	aliceTok := decode[Token](t, in.must("token", "create")).Secret
	bobTok := decode[Token](t, in.must("-as", "bob@example.com", "token", "create")).Secret
	note := func(tok string, data map[string]any) {
		t.Helper()
		st, _ := call(t, "POST", api+"/note", tok, data, nil)
		wantStatus(t, st, 201, "create note")
	}
	note(aliceTok, map[string]any{"title": "Huurcontract appartement Utrecht", "body": "Huurovereenkomst voor de woning aan de Oudegracht. Huur 1250 euro per maand."})
	note(aliceTok, map[string]any{"title": "Weekend away", "body": "Met de trein naar Utrecht"})
	note(aliceTok, map[string]any{"title": "Lease agreement London flat", "body": "Tenancy for the flat in Camden, rent paid monthly."})
	note(aliceTok, map[string]any{"title": "Café De Zaak", "body": "Koffie en appeltaart"})
	note(aliceTok, map[string]any{"title": "Belastingaangifte", "bsn": "111222333"})
	note(bobTok, map[string]any{"title": "Bob huurcontract Utrecht"})
	call(t, "POST", api+"/company", aliceTok, map[string]any{"name": "Utrecht Plumbing"}, nil)
	_, nl := e2eUpload(t, api+"/files", aliceTok, "huur.txt", []byte("De verhuurder verhuurt de woning aan de Prinsengracht aan de huurder."))
	_, en := e2eUpload(t, api+"/files", aliceTok, "policy.pdf", e2ePDF("Home insurance policy", "Covers water damage in the kitchen"))
	e2eDoneFile(t, api, aliceTok, nl.ID.String())
	e2eDoneFile(t, api, aliceTok, en.ID.String())

	t.Run("DutchAndEnglishRowsRankByTheirWords", func(t *testing.T) {
		hits := e2eSearch(t, api, aliceTok, "huurcontract utrecht")
		if got := e2eTitles(hits); !strings.HasPrefix(got, "Huurcontract appartement Utrecht|") || !strings.Contains(got, "Utrecht Plumbing") || !strings.Contains(got, "Weekend away") {
			t.Fatalf("huurcontract utrecht: %s", got)
		}
		if hits[0].Kind != "entity" || hits[0].Type != "note" {
			t.Errorf("hit: %+v", hits[0])
		}
		if got := e2eTitles(e2eSearch(t, api, aliceTok, "the lease for the London flat")); !strings.HasPrefix(got, "Lease agreement London flat") {
			t.Errorf("english: %s", got)
		}
		if got := e2eTitles(e2eSearch(t, api, aliceTok, "huur")); !strings.Contains(got, "Huurcontract appartement Utrecht") || !strings.Contains(got, "huur.txt") {
			t.Errorf("prefix huur: %s", got)
		}
	})

	t.Run("CaseAndAccentsAreFolded", func(t *testing.T) {
		for _, q := range []string{"cafe", "CAFÉ", "café de zaak"} {
			if got := e2eTitles(e2eSearch(t, api, aliceTok, q)); !strings.HasPrefix(got, "Café De Zaak") {
				t.Errorf("%s: %s", q, got)
			}
		}
	})

	t.Run("FileTextInDutchAndEnglish", func(t *testing.T) {
		hits := e2eSearch(t, api, aliceTok, "verhuurder Prinsengracht")
		if len(hits) == 0 || hits[0].ID != nl.ID || hits[0].Type != "file" || !strings.Contains(hits[0].Snippet, "**verhuurder**") {
			t.Fatalf("dutch file: %+v", hits)
		}
		hits = e2eSearch(t, api, aliceTok, "water damage", "type", "file")
		if len(hits) != 1 || hits[0].ID != en.ID {
			t.Fatalf("english pdf: %+v", hits)
		}
	})

	t.Run("FiltersNarrowTheResults", func(t *testing.T) {
		if got := e2eTitles(e2eSearch(t, api, aliceTok, "huur", "type", "note")); strings.Contains(got, "huur.txt") || !strings.Contains(got, "Huurcontract") {
			t.Errorf("type note: %s", got)
		}
		day := time.Now().Format(time.DateOnly)
		if n := len(e2eSearch(t, api, aliceTok, "huur", "from", day, "to", day)); n < 2 {
			t.Errorf("today: %d hits", n)
		}
		if n := len(e2eSearch(t, api, aliceTok, "huur", "from", time.Now().AddDate(0, 0, 1).Format(time.DateOnly))); n != 0 {
			t.Errorf("from tomorrow: %d hits", n)
		}
		if n := len(e2eSearch(t, api, aliceTok, "huur", "to", time.Now().AddDate(0, 0, -1).Format(time.DateOnly))); n != 0 {
			t.Errorf("to yesterday: %d hits", n)
		}
		st, _ := call(t, "GET", api+"/search?q=huur&space=family", aliceTok, nil, nil)
		wantStatus(t, st, 404, "unknown space")
		st, _ = call(t, "GET", api+"/search?q=huur&type=nothing", aliceTok, nil, nil)
		wantStatus(t, st, 404, "unknown type")
		st, _ = call(t, "GET", api+"/search?q=huur&from=soon", aliceTok, nil, nil)
		wantStatus(t, st, 422, "bad date")
		st, _ = call(t, "GET", api+"/search?q=+", aliceTok, nil, nil)
		wantStatus(t, st, 422, "empty query")
	})

	t.Run("SecretsNeverMatch", func(t *testing.T) {
		in := in.on(t)
		if hits := e2eSearch(t, api, aliceTok, "belastingaangifte"); len(hits) != 1 {
			t.Fatalf("secret row by title: %+v", hits)
		}
		for _, q := range []string{"111222333", "111 222 333"} {
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
		if got := e2eTitles(e2eSearch(t, api, bobTok, "utrecht")); got != "Bob huurcontract Utrecht" {
			t.Errorf("bob api: %s", got)
		}
		if got := e2eTitles(e2eSearch(t, api, aliceTok, "huurcontract")); strings.Contains(got, "Bob") {
			t.Errorf("alice api: %s", got)
		}
		if got := e2eSearch(t, api, bobTok, "verhuurder plumbing cafe"); len(got) != 0 {
			t.Errorf("bob sees alice's notes and files: %+v", got)
		}
		if got := in.must("-as", "bob@example.com", "search", "huurcontract", "utrecht"); !strings.Contains(got, "Bob huurcontract") || strings.Contains(got, "appartement") {
			t.Errorf("bob cli: %s", got)
		}
		if got := in.must("search", "huurcontract", "-type", "note"); !strings.Contains(got, "appartement") || strings.Contains(got, "Bob") {
			t.Errorf("alice cli: %s", got)
		}
		role := in.role("bob@example.com")
		in.psql("archive", "ALTER ROLE \""+role+"\" LOGIN")
		defer in.psql("archive", "ALTER ROLE \""+role+"\" NOLOGIN")
		q := "SELECT (SELECT count(*) FROM note WHERE search @@ archive.search_query('huurcontract verhuurder')) || ',' || " +
			"(SELECT count(*) FROM archive.file WHERE search @@ archive.search_query('verhuurder'))"
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
		plan := decode[SchemaPlan](t, in.push(with(files, "note.sql", e2eNote)))
		if !plan.Applied || len(plan.Statements) != 1 || plan.Statements[0].Blocked || !slices.ContainsFunc(plan.Statements[0].Hazards, func(h diff.MigrationHazard) bool { return h.Type == "INDEX_DROPPED" }) {
			t.Errorf("drop index plan: %+v", plan)
		}
		if n := in.psql("archive", q); n != "0" {
			t.Errorf("title index kept after it left the DDL: %s", n)
		}
	})
}

func TestViewsSchemaDropsAndFullTokensOverHTTP(t *testing.T) {
	t.Parallel()
	in := newInst(t, e2eGoogle(t)...)
	in.must("user", "add", "alice@example.com")
	srv := in.serve()
	api := srv.url + "/api"
	in.must("user", "add", "bob@example.com")
	in.push(e2eSchema)
	in.must("space", "add", "family")
	in.must("space", "grant", "family", "bob@example.com", "viewer")
	aliceTok := decode[Token](t, in.must("token", "create")).Secret
	bobTok := decode[Token](t, in.must("-as", "bob@example.com", "token", "create")).Secret

	t.Run("ViewsAreSQLWithTheirSettingsInComments", func(t *testing.T) {
		in := in.on(t)
		board := "CREATE VIEW people_board WITH (security_invoker = true) AS SELECT * FROM person;\n" +
			"COMMENT ON VIEW people_board IS 'Everyone you know\nby employer\nlayout: board\nby: employer\nicon: users';\n"
		in.push(with(e2eSchema, "people_board.sql", board))
		var fam, solo row
		st, _ := call(t, "POST", api+"/person?space=family", aliceTok, map[string]any{"name": "Fam"}, &fam)
		wantStatus(t, st, 201, "person in family")
		st, _ = call(t, "POST", api+"/person", aliceTok, map[string]any{"name": "Solo"}, &solo)
		wantStatus(t, st, 201, "person in alice's space")

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
			t.Errorf("bob through the view sees %s, want only the family space", names(rows))
		}
		st, _ = call(t, "POST", api+"/people_board", aliceTok, map[string]any{"name": "Nope"}, nil)
		wantStatus(t, st, 405, "create through a view")
		st, _ = call(t, "DELETE", api+"/people_board/"+id(fam), aliceTok, nil, nil)
		wantStatus(t, st, 405, "delete through a view")

		files := in.pulled()
		if !strings.Contains(files["people_board.sql"], "security_invoker=true") || !strings.Contains(files["people_board.sql"], "icon: users';") || strings.Contains(files["people_board.sql"], ";;") {
			t.Errorf("pulled view: %s", files["people_board.sql"])
		}
		in.roundTrips()
		files["people_board.sql"] = strings.Replace(files["people_board.sql"], "icon: users", "icon: contact", 1)
		if p := decode[SchemaPlan](t, in.must("schema", "plan", in.writeDir("ddl", files))); len(p.Statements) != 1 || !strings.HasPrefix(p.Statements[0].DDL, "COMMENT ON VIEW") {
			t.Errorf("changing only the comment plans %+v", p.Statements)
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
			if e := in.fails("schema", "apply", in.writeDir("bad", with(e2eSchema, "bad.sql", bad.sql))); !strings.Contains(e, bad.want) {
				t.Errorf("%s: %s", bad.sql, e)
			}
		}
		company := strings.Replace(e2eSchema["company.sql"], "'A company'", "'A company\nlayout: board'", 1)
		if e := in.fails("schema", "apply", in.writeDir("bad", with(e2eSchema, "company.sql", company))); !strings.Contains(e, "unknown key layout") {
			t.Errorf("layout on a table: %s", e)
		}

		code := `const files = await archive.schema();
files["open_people.sql"] = "CREATE VIEW open_people WITH (security_invoker = true) AS SELECT * FROM person WHERE name <> 'Solo';\n" +
  "COMMENT ON VIEW open_people IS 'People but Solo\nlayout: board\nby: employer\nicon: users';";
const made = await archive.apply(files, { confirm: true });
const t1 = (await archive.tables()).find((t) => t.name === "open_people");
const rows1 = (await archive.list("open_people")).map((r) => r.name);
files["open_people.sql"] = "CREATE VIEW open_people WITH (security_invoker = true) AS SELECT * FROM person WHERE name = 'Solo';\n" +
  "COMMENT ON VIEW open_people IS 'Only Solo\nicon: user';";
const changed = await archive.apply(files, { confirm: true });
const t2 = (await archive.tables()).find((t) => t.name === "open_people");
const rows2 = (await archive.list("open_people")).map((r) => r.name);
delete files["open_people.sql"];
const dropped = await archive.apply(files, { confirm: true });
const gone = !(await archive.tables()).some((t) => t.name === "open_people");
return { made: made.applied, t1, rows1, changed: changed.applied, t2, rows2, dropped: dropped.applied, gone };`
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
		in.push(e2eSchema)
		if got := in.psql("archive", "SELECT count(*) FROM pg_views WHERE schemaname = 'public'"); got != "0" {
			t.Errorf("%s views left after removing their files", got)
		}
	})

	t.Run("DroppingATableIsBlockedUntilAllowed", func(t *testing.T) {
		st, _ := call(t, "POST", api+"/invoice", aliceTok, map[string]any{"number": "INV-1"}, nil)
		wantStatus(t, st, 201, "create invoice")
		rest := with(e2eSchema, "invoice.sql", "")
		var plan SchemaPlan
		st, _ = call(t, "POST", api+"/schema/plan", aliceTok, map[string]any{"files": rest}, &plan)
		wantStatus(t, st, 200, "plan the drop")
		if !slices.ContainsFunc(plan.Statements, func(s SchemaStatement) bool {
			return s.Blocked && strings.Contains(s.DDL, `DROP TABLE "public"."invoice"`)
		}) {
			t.Fatalf("drop plan: %+v", plan)
		}
		st, _ = call(t, "POST", api+"/schema/apply", aliceTok, map[string]any{"files": rest}, nil)
		wantStatus(t, st, 409, "apply a drop without allow_drop")
		st, _ = call(t, "GET", api+"/invoice", aliceTok, nil, nil)
		wantStatus(t, st, 200, "invoice after the blocked drop")
		st, _ = call(t, "POST", api+"/schema/apply", aliceTok, map[string]any{"files": rest, "allow_drop": true}, &plan)
		wantStatus(t, st, 200, "apply the drop")
		st, _ = call(t, "GET", api+"/invoice", aliceTok, nil, nil)
		wantStatus(t, st, 404, "list a dropped table")
		if _, err := os.Stat(filepath.Join(in.dir, "schema", "invoice.sql")); err == nil {
			t.Error("invoice.sql is still a schema file")
		}
	})

	t.Run("FullTokenManagesMembersAndTokens", func(t *testing.T) {
		in := in.on(t)
		full := decode[Token](t, in.must("token", "create", "-name", "Terraform", "-full"))
		if !full.Full || len(full.Spaces) != 0 || full.Secret == "" {
			t.Fatalf("full token: %+v", full)
		}
		in.fails("token", "create", "-full", "-space", "family:viewer")
		st, _ := call(t, "POST", srv.url+"/auth/spaces/family/members", aliceTok, map[string]any{"email": "bob@example.com", "role": "editor"}, nil)
		wantStatus(t, st, 403, "scoped token changes membership")
		st, _ = call(t, "POST", srv.url+"/auth/tokens", aliceTok, map[string]any{"full": true}, nil)
		wantStatus(t, st, 403, "scoped token makes a full token")
		var ms []Member
		st, _ = call(t, "POST", srv.url+"/auth/spaces/family/members", full.Secret, map[string]any{"email": "bob@example.com", "role": "editor"}, &ms)
		wantStatus(t, st, 200, "full token changes membership")
		if len(ms) != 2 || ms[0].Email != "alice@example.com" || ms[1].Level != "editor" {
			t.Errorf("members: %+v", ms)
		}
		var made Token
		st, _ = call(t, "POST", srv.url+"/auth/tokens", full.Secret, map[string]any{"name": "Agent", "spaces": map[string]string{"family": "viewer"}}, &made)
		wantStatus(t, st, 201, "full token makes a scoped token")
		var ts []Token
		call(t, "GET", srv.url+"/auth/tokens", full.Secret, nil, &ts)
		var listed bool
		for _, tk := range ts {
			listed = listed || tk.ID == full.ID && tk.Full && tk.Secret == ""
		}
		if !listed || !slices.ContainsFunc(ts, func(tk Token) bool { return tk.ID == made.ID && tk.Spaces["family"] == "viewer" }) {
			t.Errorf("tokens: %+v", ts)
		}
		st, _ = call(t, "DELETE", srv.url+"/auth/tokens/"+full.ID.String(), full.Secret, nil, nil)
		wantStatus(t, st, 204, "revoke full token")
		st, _ = call(t, "GET", srv.url+"/auth/me", full.Secret, nil, nil)
		wantStatus(t, st, 401, "revoked full token")
	})

	t.Run("CatalogAndSpaceFilters", func(t *testing.T) {
		var cat Catalog
		call(t, "GET", api+"/types", bobTok, nil, &cat)
		if cat.Tables["person"] == nil || cat.Tables["person"].col("employer").Ref != "public.company" || cat.Types["archive.money"] == nil || cat.Types["archive.money"].Kind != "c" {
			t.Errorf("catalog: %+v", cat.Tables["person"])
		}
		var fam, own row
		call(t, "POST", api+"/company?space=family", aliceTok, map[string]any{"name": "Family firm"}, &fam)
		call(t, "POST", api+"/company", aliceTok, map[string]any{"name": "Own firm"}, &own)
		var rs []row
		st, _ := call(t, "GET", api+"/company?space=family", aliceTok, nil, &rs)
		wantStatus(t, st, 200, "list one space")
		if len(rs) != 1 || id(rs[0]) != id(fam) {
			t.Errorf("family companies: %+v", rs)
		}
		call(t, "GET", api+"/company", aliceTok, nil, &rs)
		if len(rs) != 2 {
			t.Errorf("all companies: %+v", rs)
		}
		st, _ = call(t, "GET", api+"/company?space=nowhere", aliceTok, nil, nil)
		wantStatus(t, st, 404, "list an unknown space")
		e2eUpload(t, api+"/files?space=family", aliceTok, "family.txt", []byte("family notes"))
		e2eUpload(t, api+"/files", aliceTok, "own.txt", []byte("own notes"))
		var fs []File
		call(t, "GET", api+"/files?space=family", aliceTok, nil, &fs)
		if len(fs) != 1 || fs[0].Name != "family.txt" {
			t.Errorf("family files: %+v", fs)
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
    space      uuid NOT NULL REFERENCES archive.space,
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
CREATE POLICY viewer ON house FOR SELECT USING (space = ANY (archive.my_spaces()));
CREATE POLICY editor ON house USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));
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
	in := newInst(t, "ARCHIVE_RUN_TIMEOUT=1s")
	in.must("user", "add", "alice@example.com")
	in.push(map[string]string{"house.sql": e2eHouse})
	srv := in.serve()
	tok := decode[Token](t, in.must("token", "create")).Secret
	start := time.Now()
	var out RunOutput
	st, _ := call(t, "POST", srv.url+"/api/run", tok, map[string]string{"code": `await archive.create("house", { address: "Loop 1" }); while (true) {}`}, &out)
	if st != 200 || out.Error != "timed out after 1s; nothing was saved" || time.Since(start) > 15*time.Second {
		t.Errorf("timeout: %d %+v after %s", st, out, time.Since(start))
	}
	if n := in.must("list", "house"); strings.Contains(n, "Loop 1") {
		t.Errorf("the timed out run saved a row: %s", n)
	}
}

func TestCodeModeRunsTypeScriptInOneTransactionOverHTTPMCPAndCLI(t *testing.T) {
	t.Parallel()
	in := newInst(t, "ARCHIVE_RUN_TIMEOUT=30s")
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
		return fmt.Sprint(ok(t, `return { n: (await archive.sql("SELECT count(*)::int AS n FROM house WHERE address = $1", `+strconv.Quote(address)+`))[0].n }`)["n"])
	}
	_, note := e2eUpload(t, api+"/files", tok, "viewing.txt", []byte("Viewing on Friday at the canal house"))
	e2eDoneFile(t, api, tok, note.ID.String())

	t.Run("DroppingATableFromARunAppliesAfterTheRunCommits", func(t *testing.T) {
		shed := e2eTable("shed", "    label text NOT NULL,\n    search tsvector GENERATED ALWAYS AS (archive.search_doc(label)) STORED")
		ok(t, `const files = await archive.schema();
files["shed.sql"] = `+strconv.Quote(shed)+`;
await archive.apply(files, { confirm: true });
await archive.create("shed", { label: "tools" });
return {};`)
		drop := `const files = await archive.schema(); delete files["shed.sql"]; await archive.apply(files, { confirm: true, allowDrop: true });`
		if out := run(t, tok, drop+` throw new Error("changed my mind");`); !strings.Contains(out.Error, "changed my mind") {
			t.Fatalf("rolled back run: %+v", out)
		}
		if kept := ok(t, `return { kept: (await archive.tables()).some((t) => t.name === "shed") };`); kept["kept"] != true {
			t.Errorf("a rolled back run still dropped the table: %v", kept)
		}
		got := ok(t, `const files = await archive.schema();
const before = (await archive.list("shed")).length;
delete files["shed.sql"];
const blocked = await archive.apply(files, { confirm: true }).then(() => "", (e) => e.message);
const plan = await archive.apply(files, { confirm: true, allowDrop: true });
return { before, blocked, applied: plan.applied, deferred: plan.deferred, still: (await archive.tables()).some((t) => t.name === "shed") };`)
		if fmt.Sprintf("%v %v %v %v", got["before"], got["applied"], got["deferred"], got["still"]) != "1 false true true" || !strings.Contains(fmt.Sprint(got["blocked"]), "allow_drop") {
			t.Fatalf("drop in a run: %v", got)
		}
		if gone := ok(t, `return { gone: !(await archive.tables()).some((t) => t.name === "shed") };`); gone["gone"] != true {
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
		wantStatus(t, st, 404, "bob renames alice's file")
		st, _ = call(t, "PATCH", api+"/files/"+note.ID.String(), tok, map[string]string{"name": " "}, nil)
		wantStatus(t, st, 400, "empty name")
	})

	t.Run("ApplyPlansThenConfirmsASchemaChange", func(t *testing.T) {
		got := ok(t, `const files = await archive.schema();
const house = `+strconv.Quote(e2eHouse)+`;
const plan = await archive.apply({ ...files, "house.sql": house });
const done = await archive.apply({ ...files, "house.sql": house }, { confirm: true });
const again = await archive.apply(await archive.schema());
return { planned: plan.statements.length, before: plan.applied, applied: done.applied, after: again.statements.length,
  tables: (await archive.tables()).map(t => t.name + ":" + t.columns.filter(c => c.required).map(c => c.name).join("+")) };`)
		if fmt.Sprintf("%v %v %v", got["before"], got["applied"], got["after"]) != "false true 0" || fmt.Sprint(got["planned"]) == "0" || fmt.Sprint(got["tables"]) != "[house:address]" {
			t.Errorf("apply: %v", got)
		}
		if files := in.must("schema", "pull"); !strings.Contains(files, "-- house.sql") {
			t.Errorf("schema pull lacks the applied table: %s", files)
		}
	})

	t.Run("DocsCarryTheSDKAndAnInterfacePerTable", func(t *testing.T) {
		resp, body := e2eGet(t, api+"/run/docs", tok)
		docs := string(body)
		for _, want := range []string{"declare const archive", "sql(query: string, ...params: unknown[]): Promise<Row[]>", "interface Money { amount: number; currency: Currency }",
			"/** A house you own or rent. */\ninterface House extends Base {\n    address: string;\n    /** Asking price. */\n    price?: Money;\n    sold?: boolean;\n}", "house: House;", "type Email = string;", "read(fileId: string): Promise<Model>;", "edit(fileId: string, edits: Edit[]): Promise<StoredFile>;"} {
			if resp.StatusCode != 200 || !strings.Contains(docs, want) {
				t.Errorf("docs lack %q:\n%s", want, docs)
			}
		}
		if cli := in.must("run", "-docs"); cli != docs {
			t.Errorf("archive run -docs differs from GET /api/run/docs")
		}
	})

	t.Run("CRUDSearchSQLAndFilesInOneScript", func(t *testing.T) {
		got := ok(t, `const a = await archive.create("house", { address: "Keizersgracht 1", price: "€ 950.000,00" });
const b = await archive.create("house", { address: "Oudegracht 2" });
const listed = await archive.list("house", { where: { address: "Oudegracht 2" } });
const priced = await archive.update("house", b.id, { price: { amount: 500000, currency: "EUR" } });
const cleared = await archive.update("house", a.id, { price: null });
const hits = await archive.search("keizersgracht");
const [{ n }] = await archive.sql("SELECT count(*)::int AS n FROM house WHERE price IS NOT NULL");
const files = await archive.files();
const file = await archive.file(files[0].id);
console.log("made", { a: a.address });
return { price: a.price, sold: b.sold, listed: listed.map(h => h.id === b.id), priced: priced.price?.amount, cleared: cleared.price,
  hit: hits.find(h => h.type === "house")?.data.address, n, text: file.text, spaces: (await archive.spaces()).length };`)
		want := map[string]string{"price": "map[amount:950000 currency:EUR]", "sold": "false", "listed": "[true]", "priced": "500000", "cleared": "<nil>",
			"hit": "Keizersgracht 1", "n": "1", "text": "Viewing on Friday at the canal house", "spaces": "1"}
		for k, v := range want {
			if fmt.Sprint(got[k]) != v {
				t.Errorf("%s = %v, want %s", k, got[k], v)
			}
		}
		if out := run(t, bobTok, `return (await archive.list("house")).length + (await archive.sql("SELECT * FROM house")).length`); string(out.Result) != "0" || out.Error != "" {
			t.Errorf("bob sees alice's houses: %+v", out)
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
		if out := run(t, tok, `await archive.sql("SELECT nope FROM house")`); !strings.Contains(out.Error, `column "nope" does not exist (SQLSTATE 42703)`) {
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
		if out := decode[RunOutput](t, stdout); err != nil || fmt.Sprint(decode[[]string](t, string(out.Result))) != "[Keizersgracht 1 Oudegracht 2]" {
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
		res, err := sess.CallTool(ctx, &mcp.CallToolParams{Name: "run", Arguments: map[string]any{"code": `console.log("hi"); return (await archive.list("house", { where: { address: "Oudegracht 2" } }))[0].price;`}})
		if err != nil || res.IsError || len(res.Content) != 1 {
			t.Fatalf("call: %v %+v", err, res)
		}
		out := decode[RunOutput](t, res.Content[0].(*mcp.TextContent).Text)
		if out.Error != "" || string(out.Result) != `{"amount":500000,"currency":"EUR"}` || !slices.Equal(out.Logs, []string{"hi"}) {
			t.Errorf("mcp run: %+v", out)
		}
		res, err = sess.CallTool(ctx, &mcp.CallToolParams{Name: "run", Arguments: map[string]any{"code": `return archive.get("house", "nope")`}})
		if err != nil || !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, `not a uuid`) {
			t.Errorf("mcp error: %v %+v", err, res)
		}
	})

	t.Run("RenamesRunFirstAndKeepTheData", func(t *testing.T) {
		got := ok(t, `const files = await archive.schema();
files["house.sql"] = files["house.sql"].replaceAll("price", "asking_price");
const plan = await archive.apply(files, { confirm: true, renames: ["ALTER TABLE house RENAME COLUMN price TO asking_price"] });
return { ddl: plan.statements.map(s => s.ddl), price: (await archive.sql("SELECT (asking_price).amount FROM house WHERE asking_price IS NOT NULL"))[0].amount };`)
		if fmt.Sprint(got["ddl"]) != `[ALTER TABLE house RENAME COLUMN price TO asking_price COMMENT ON COLUMN "public"."house"."asking_price" IS 'Asking asking_price']` || fmt.Sprint(got["price"]) != "500000" {
			t.Errorf("rename: %v", got)
		}
		dry := ok(t, `const files = await archive.schema();
files["house.sql"] = files["house.sql"].replaceAll("asking_price", "list_price");
const plan = await archive.apply(files, { renames: ["ALTER TABLE house RENAME COLUMN asking_price TO list_price;"] });
const cols = await archive.sql("SELECT column_name FROM information_schema.columns WHERE table_name = 'house' AND column_name LIKE '%price'");
return { ddl: plan.statements.map(s => s.ddl), applied: plan.applied, cols: cols.map(c => c.column_name) };`)
		if fmt.Sprint(dry["ddl"]) != `[ALTER TABLE house RENAME COLUMN asking_price TO list_price COMMENT ON COLUMN "public"."house"."list_price" IS 'Asking list_price']` || dry["applied"] != false || fmt.Sprint(dry["cols"]) != "[asking_price]" {
			t.Errorf("a dry run lists its renames and changes nothing: %v", dry)
		}
		if out := run(t, tok, `await archive.apply(await archive.schema(), { renames: ["DROP TABLE house"] })`); !strings.Contains(out.Error, "renames[0]: want one ALTER") {
			t.Errorf("a rename that drops: %q", out.Error)
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
		case first == "how many spaces?" && len(msgs) == 1:
			tool(`{"code": "return (await archive.spa`, `ces()).length * 7"}`)
		case answered:
			text("You have ", "one space.")
		default:
			text("Hello again.")
		}
	}))
	defer fake.Close()
	in := newInst(t, "ANTHROPIC_API_KEY=test", "ANTHROPIC_BASE_URL="+fake.URL, "ARCHIVE_CHAT_MODEL=")
	in.must("user", "add", "alice@example.com")
	srv := in.serve()
	api := srv.url + "/api"
	in.must("user", "add", "bob@example.com")
	tok := decode[Token](t, in.must("token", "create")).Secret
	bobTok := decode[Token](t, in.must("-as", "bob@example.com", "token", "create")).Secret
	ctx := context.Background()
	var chatID string

	t.Run("StreamsTextCodeAndRunResultsThenSavesTheTurn", func(t *testing.T) {
		st, evs := e2eChat(ctx, t, api, tok, map[string]any{"text": "how many spaces?"}, nil)
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
				if fmt.Sprint(ev["result"]) != "7" || ev["error"] != "" || ev["id"] != "t1" {
					t.Errorf("result event: %v", ev)
				}
			}
		}
		if strings.Join(codes, "|") != "return (await archive.spa|return (await archive.spaces()).length * 7" || strings.Join(text, "") != "You have one space." {
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
		if s, _ := json.Marshal(second["messages"]); len(second["messages"].([]any)) != 3 || !strings.Contains(string(s), `"signature":"sig"`) || !strings.Contains(string(s), `\"result\":7`) {
			t.Errorf("second model request: %s", s)
		}
	})

	t.Run("HistoryIsListedOpenedRenamedAndOnlyYours", func(t *testing.T) {
		var list []Chat
		call(t, "GET", api+"/chats", tok, nil, &list)
		if len(list) != 1 || list[0].ID.String() != chatID || list[0].Title != "how many spaces?" || list[0].Messages != nil {
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
		st, _ := call(t, "PATCH", api+"/chats/"+chatID, tok, map[string]string{"title": "  Counting   spaces "}, &renamed)
		wantStatus(t, st, 200, "rename")
		if renamed.Title != "Counting spaces" {
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
		if st != 200 || e2eKinds(evs) != "chat,text,done" || fmt.Sprint(evs[0]["chat"].(map[string]any)["title"]) != "Counting spaces" {
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
	in := newInst(t, "ARCHIVE_RUN_TIMEOUT=30s")
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
const [{ n }] = await archive.sql("SELECT count(*)::int AS n FROM house WHERE address IN ('Test 1', 'Draft 1', 'Thrown 1')");
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
	return []string{"ARCHIVE_GOOGLE_URL=" + e2eIssuer(t).URL, "ARCHIVE_GOOGLE_CLIENT_ID=web", "ARCHIVE_GOOGLE_CLIENT_SECRET=shh"}
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
		st, out := e2eRun(t, api, bob, `return await archive.sql("SELECT current_database() AS db, (SELECT count(*)::int FROM company) AS n")`)
		if st != 200 || !strings.Contains(string(out.Result), `"db":"archive_`+ws+`"`) || !strings.Contains(string(out.Result), `"n":1`) {
			t.Errorf("bob's raw SQL: %d %+v", st, out)
		}
		if _, out := e2eRun(t, api, bob, `return await archive.sql("SELECT set_config('role', '`+one+`_admin', true)")`); !strings.Contains(out.Error, "permission denied") {
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
		if _, err := os.Stat(filepath.Join(in.dir, "blobs", ws, g.Hash[:2], g.Hash)); err != nil {
			t.Errorf("bob's file is not under his workspace's prefix: %v", err)
		}
		if _, err := os.Stat(filepath.Join(in.dir, "blobs", one, f.Hash[:2], f.Hash)); err != nil {
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
	key, err := os.ReadFile(filepath.Join(in.dir, "secret.key"))
	if err != nil {
		in.t.Fatal(err)
	}
	dsn := "host=" + in.dir + " user=cloudsql password=cloudsql dbname=tenantmain"
	return &e2eInst{t: in.t, dir: dir, env: append(append([]string{"ARCHIVE_DATA=" + dir, "DATABASE_URL=" + dsn,
		"ARCHIVE_SECRET_KEY=" + strings.TrimSpace(string(key))}, e2eCloudTasks.queue(in.t, dir)...), env...)}
}

func TestWorkspacesIsolateWhenArchiveIsNotASuperuser(t *testing.T) {
	t.Parallel()
	host := newInst(t)
	in := host.cloudsql("ANTHROPIC_API_KEY=")
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
	aAdmin := a + "_admin"
	aUser := host.psqlIn("tenantmain", "SELECT '"+a+"' || '_u' || translate(rtrim(encode(uuid_send(id), 'base64'), '='), '+/', '-_') FROM archive.\"user\" WHERE email = 'alice@example.com'")

	t.Run("BobRunsInHisOwnDatabase", func(t *testing.T) {
		out := bob(`return await archive.sql("SELECT current_database() AS db")`)
		if out.Error != "" || !strings.Contains(string(out.Result), "archive_"+b) {
			t.Errorf("bob's database: %+v", out)
		}
	})
	t.Run("BobCannotReachTheOtherDatabase", func(t *testing.T) {
		out := bob(`return await archive.sql('SELECT count(*) FROM "tenantmain".ledger')`)
		if out.Error == "" {
			t.Errorf("bob read the main database: %v", out.Result)
		}
	})
	t.Run("BobCannotSwitchIntoTheOtherWorkspacesRoles", func(t *testing.T) {
		for _, role := range []string{aAdmin, aUser} {
			out := bob(`return await archive.sql("SELECT set_config('role', '` + role + `', true)")`)
			if !strings.Contains(out.Error, "permission denied") {
				t.Errorf("set_config role %s: %+v", role, out)
			}
			out = bob(`return await archive.sql('SET ROLE "` + role + `"')`)
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
		out := bob(`return await archive.sql("SELECT count(*)::int AS n FROM pg_class WHERE relname = 'only_in_a'")`)
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
	t.Run("RemovingASpaceDeletesItsRowsFilesSecretsAndRoles", func(t *testing.T) {
		in := in.on(t)
		in.must("space", "add", "team")
		base := host.psqlIn("tenantmain", "SELECT role FROM archive.space WHERE name = 'team'")
		path := filepath.Join(in.dir, "team.txt")
		os.WriteFile(path, []byte("team memo"), 0o644)
		f := decode[File](t, in.must("file", "add", path, "-space", "team"))
		in.must("create", "ledger", "-space", "team", `{"codename": "Team", "ssn": "123-45-6789", "document": "`+f.ID.String()+`"}`)
		in.must("create", "ledger", `{"codename": "Home"}`)
		in.must("space", "remove", "team")
		got := host.psqlIn("tenantmain", "SELECT (SELECT count(*) FROM ledger WHERE codename = 'Team') || ',' || (SELECT count(*) FROM ledger WHERE codename = 'Home') || ',' || "+
			"(SELECT count(*) FROM archive.secret) || ',' || (SELECT count(*) FROM archive.file WHERE id = '"+f.ID.String()+"') || ',' || "+
			"(SELECT count(*) FROM archive.space WHERE name = 'team') || ',' || (SELECT count(*) FROM pg_roles WHERE starts_with(rolname, '"+base+"'))")
		if got != "0,1,0,0,0,0" {
			t.Errorf("after removing team (team rows, home rows, secrets, team file, team space, team roles): %s", got)
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
			out = append(out, fmt.Sprintf("%s %s %v", p.Email, p.Name, p.Admin))
		}
		return strings.Join(out, ",")
	}
	var people []Person
	st, _ = e2eDo(t, "GET", srv.url+"/auth/users", nil, nil, &people, dave)
	if want := "alice@example.com  true,dave@example.com Dave Day false,frank@example.com  false"; st != 200 || who(people) != want {
		t.Errorf("dave lists the people in the workspace: %d %s", st, who(people))
	}
	st, _ = call(t, "GET", srv.url+"/auth/users", "", nil, nil)
	wantStatus(t, st, 401, "the people without signing in")
	st, _ = e2eDo(t, "POST", srv.url+"/auth/users", nil, map[string]any{"email": "carol@example.com"}, nil, dave)
	wantStatus(t, st, 403, "a non-admin invites someone")
	st, _ = e2eDo(t, "DELETE", srv.url+"/auth/users/frank@example.com", nil, nil, nil, dave)
	wantStatus(t, st, 403, "a non-admin removes someone")
	scoped := decode[Token](t, in.must("token", "create")).Secret
	st, _ = call(t, "POST", srv.url+"/auth/users", scoped, map[string]any{"email": "carol@example.com"}, nil)
	wantStatus(t, st, 403, "a scoped token invites someone")
	st, _ = e2eDo(t, "POST", srv.url+"/auth/users", nil, map[string]any{"email": "not an email"}, nil, alice)
	wantStatus(t, st, 422, "inviting something that is not an email")
	st, _ = e2eDo(t, "POST", srv.url+"/auth/users", nil, map[string]any{"email": " Carol@Example.com ", "admin": true}, &people, alice)
	if want := "alice@example.com  true,carol@example.com  true,dave@example.com Dave Day false,frank@example.com  false"; st != 201 || who(people) != want {
		t.Errorf("an admin invites carol as an admin: %d %s", st, who(people))
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
	if want := "alice@example.com  true,dave@example.com Dave Day false,frank@example.com  false"; st != 200 || who(people) != want {
		t.Errorf("an admin removes carol: %d %s", st, who(people))
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
	st, _ = e2eDo(t, "POST", srv.url+"/api/run", nil, map[string]string{"code": "return (await archive.spaces()).map((s) => s.level).join(',')"}, &out, carol)
	if st != 200 || string(out.Result) != `"owner"` {
		t.Errorf("carol runs in her workspace: %d %+v", st, out)
	}
	st, _ = e2eDo(t, "POST", srv.url+"/auth/workspaces", nil, map[string]string{"name": "Mine", "owner": "carol@example.com"}, nil, carol)
	wantStatus(t, st, 403, "a workspace owner creates a workspace for someone")

	full := decode[Token](t, in.must("token", "create", "-full")).Secret
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
	st, _ = e2eDo(t, "POST", srv.url+"/api/run", map[string]string{"Archive-Workspace": second.ID}, map[string]string{"code": "return (await archive.spaces()).map((s) => s.level).join(',')"}, &out, alice)
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
	if errs := in.fails("-workspace", second.ID, "-as", "carol@example.com", "user", "remove", "alice@example.com"); !strings.Contains(errs, "only admins") {
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
	if n := in.psql("archive", "SELECT count(*) FROM pg_roles WHERE starts_with(rolname, '"+second.ID+"_u') AND rolname <> '"+second.ID+"_user'"); n != "1" {
		t.Errorf("%s user roles left in the second workspace, want alice's", n)
	}
	if got := decode[[]Member](t, in.must("-workspace", second.ID, "space", "members", "alice_example_com")); len(got) != 1 {
		t.Errorf("members of alice's space in the second workspace: %+v", got)
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
		"ARCHIVE_GITHUB_URL="+gh.URL, "ARCHIVE_GITHUB_CLIENT_ID=gh", "ARCHIVE_GITHUB_CLIENT_SECRET=shh",
		"ARCHIVE_MICROSOFT_URL="+ms.URL, "ARCHIVE_MICROSOFT_CLIENT_ID=web", "ARCHIVE_MICROSOFT_CLIENT_SECRET=shh",
		"ARCHIVE_SLACK_CLIENT_ID=slack", "ARCHIVE_SLACK_CLIENT_SECRET=shh", "ARCHIVE_GOOGLE_CLIENT_ID=only-an-id")
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
	in := newInst(t, append(e2eGoogle(t), "ARCHIVE_URL=http://app.test", "ARCHIVE_API_URL=http://api.test:9/")...)
	in.must("user", "add", "alice@example.com")
	srv := in.serve()
	api := srv.url + "/api"
	one := in.psql("archive", "SELECT id FROM archive.workspace ORDER BY created_at LIMIT 1")
	tok := decode[Token](t, in.must("token", "create")).Secret
	_, out := e2eRun(t, api, tok, `return (await archive.deploy({ "hook.ts": "// on: webhook\nreturn 1;" })).workflows[0].url;`)
	if !strings.HasPrefix(string(out.Result), `"http://api.test:9/hooks/run/`+one+`/`) {
		t.Errorf("webhook URL: %+v", out)
	}
	_, f := e2eUpload(t, api+"/files", tok, "a.txt", []byte("a"))
	var link FileLink
	call(t, "POST", api+"/files/"+f.ID.String()+"/link", tok, nil, &link)
	if !strings.HasPrefix(link.URL, "http://api.test:9/files/"+one+"/") {
		t.Errorf("file link: %s", link.URL)
	}
	st, hdr := call(t, "GET", srv.url+"/auth/oidc/google", "", nil, nil)
	loc, _ := url.Parse(hdr.Get("Location"))
	if st != 302 || loc == nil || loc.Query().Get("redirect_uri") != "http://api.test:9/auth/oidc/google/callback" || loc.Query().Get("state") == "" {
		t.Errorf("OIDC redirect: %d %s", st, hdr.Get("Location"))
	}
	st, _ = call(t, "GET", srv.url+"/auth/oidc/google/callback?state=forged&code=x", "", nil, nil)
	wantStatus(t, st, 401, "an OIDC callback with a forged state")
	if !strings.Contains(srv.log.String(), "the API on http://api.test:9/api") {
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
		req.Header.Set("Access-Control-Request-Headers", "content-type,archive-workspace,archive-space")
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
	in := newInst(t, append(e2eGoogle(t), "ARCHIVE_URL=https://app.test", "ARCHIVE_API_URL=https://api.test")...)
	in.must("user", "add", "alice@example.com")
	srv := in.serve()
	st, hdr := e2eFrom(t, "OPTIONS", srv.url+"/api/chats", "https://app.test")
	allowed := hdr.Get("Access-Control-Allow-Headers") + " " + hdr.Get("Access-Control-Allow-Methods")
	if st != 204 || hdr.Get("Access-Control-Allow-Origin") != "https://app.test" || hdr.Get("Access-Control-Allow-Credentials") != "true" || hdr.Get("Access-Control-Max-Age") == "" ||
		!strings.Contains(hdr.Get("Vary"), "Origin") || !strings.Contains(allowed, "Archive-Workspace") || !strings.Contains(allowed, "Archive-Space") || !strings.Contains(allowed, "Content-Type") || !strings.Contains(allowed, "PATCH") {
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
	queue, acct := "projects/p/locations/l/queues/q", "tasks@p.iam.gserviceaccount.com"
	in := newInst(t, append(fake.env(queue), "ARCHIVE_TASKS_SERVICE_ACCOUNT="+acct, "ARCHIVE_TASKS_ISSUER="+google.URL, "ARCHIVE_API_URL=http://api.test")...)
	in.must("user", "add", "alice@example.com")
	in.push(map[string]string{"house.sql": e2eHouse})
	srv := in.serve()
	api := srv.url + "/api"
	one := in.psql("archive", "SELECT id FROM archive.workspace ORDER BY created_at LIMIT 1")
	tok := decode[Token](t, in.must("token", "create")).Secret
	last := func(t *testing.T) *taskspb.Task {
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
	hr := tk.GetHttpRequest()
	var body map[string]string
	json.Unmarshal(hr.GetBody(), &body)
	if !tk.GetScheduleTime().AsTime().Equal(want) || !strings.HasPrefix(tk.GetName(), queue+"/tasks/") || hr.GetHttpMethod() != taskspb.HttpMethod_POST ||
		hr.GetUrl() != "http://api.test/tasks" || hr.GetOidcToken().GetAudience() != hr.GetUrl() ||
		hr.GetOidcToken().GetServiceAccountEmail() != acct || fmt.Sprint(body) != fmt.Sprint(work) {
		t.Fatalf("the cron workflow's task: %+v %v, want it at %s", tk, body, want)
	}
	aud = hr.GetUrl()

	deploy(t, map[string]string{"tick.ts": "// on: cron 0 9 * * *\nreturn 'tick';",
		"sold.ts": "// on: house\nfor (const r of input.rows) await archive.update('house', r.id, { sold: true });\nreturn input.rows.map((r) => r.address).join(',');"})
	if tk := last(t); time.Until(tk.GetScheduleTime().AsTime()) > time.Second {
		t.Errorf("a table workflow's first task is at %s, want now", tk.GetScheduleTime().AsTime())
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
	if tk := last(t); !tk.GetScheduleTime().AsTime().Equal(want) {
		t.Errorf("after the run the next task is at %s, want the cron time %s", tk.GetScheduleTime().AsTime(), want)
	}
	if at := in.psql("archive", "SELECT at = '"+want.Format(time.RFC3339Nano)+"' FROM archive.due WHERE kind = 'workflow'"); at != "t" {
		t.Errorf("due after the run: %s", in.psql("archive", "SELECT at FROM archive.due"))
	}

	n := count()
	st, out := e2eRun(t, api, tok, `const [h] = await archive.list("house"); await archive.update("house", h.id, { sold: false }); return 0;`)
	if st != 200 || out.Error != "" || count() != n+1 || time.Until(last(t).GetScheduleTime().AsTime()) > time.Second || time.Since(last(t).GetScheduleTime().AsTime()) > 5*time.Second {
		t.Fatalf("a write made %d tasks, the last at %s: %+v", count()-n, last(t).GetScheduleTime().AsTime(), out)
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
	var info WorkspaceInfo
	st, _ := call(t, "GET", srv.url+"/auth/workspace", bob, nil, &info)
	if st != 200 || info.ID != ws {
		t.Errorf("bob's workspace on first use: %d %+v\n%s", st, info, srv.log)
	}
}

func TestChatUsesTheWorkspacesOwnAnthropicKey(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var keys []string
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		keys = append(keys, r.Header.Get("x-api-key"))
		mu.Unlock()
		for _, ev := range []string{
			`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hi."}}`,
			`{"type":"content_block_stop","index":0}`,
			`{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`,
		} {
			fmt.Fprintf(w, "event: x\ndata: %s\n\n", ev)
		}
	}))
	defer model.Close()
	in := newInst(t, "ANTHROPIC_API_KEY=shared", "ANTHROPIC_BASE_URL="+model.URL, "ARCHIVE_CHAT_MODEL=")
	in.must("user", "add", "alice@example.com")
	ws := decode[Workspace](t, in.must("workspace", "add", "Bob Co", "bob@example.com")).ID
	srv := in.serve()
	bob := decode[Token](t, in.must("-workspace", ws, "-as", "bob@example.com", "token", "create", "-full")).Secret
	chat := func(t *testing.T) int {
		t.Helper()
		st, _ := call(t, "POST", srv.url+"/api/chat", bob, map[string]string{"text": "hi"}, nil)
		return st
	}
	wantStatus(t, chat(t), 200, "chat on the shared key")
	var i WorkspaceInfo
	st, _ := call(t, "PATCH", srv.url+"/auth/workspace", bob, map[string]string{"anthropic_key": "own-key"}, &i)
	wantStatus(t, st, 200, "set the workspace's own Anthropic key")
	if !i.AnthropicKey {
		t.Errorf("workspace info after setting a key: %+v", i)
	}
	wantStatus(t, chat(t), 200, "chat on the workspace's own key")
	mu.Lock()
	if !slices.Equal(keys, []string{"shared", "own-key"}) {
		t.Errorf("Anthropic keys used: %v", keys)
	}
	mu.Unlock()
}
