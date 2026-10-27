package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"cmp"
	"context"
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
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/go-crdt/crdt"
	"github.com/go-crdt/crdt/awareness"
	"github.com/go-crdt/crdt/structured"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stripe/pg-schema-diff/pkg/diff"
)

var e2eBin, e2eTemplate string

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
	c := exec.Command(e2eBin, "types")
	c.Env = append(os.Environ(), "ARCHIVE_DATA="+e2eTemplate)
	if out, err := c.CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		panic(fmt.Sprintf("template data dir: %v\n%s", err, out))
	}
	flag.Set("test.parallel", "32")
	code := m.Run()
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
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(name)) STORED`) + "COMMENT ON TABLE company IS 'icon: building';\n",
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
    search     tsvector GENERATED ALWAYS AS (archive.search_doc(name, ssn)) STORED`) + "COMMENT ON COLUMN person.friends IS 'references: person';\n",
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
	if !slices.Contains(env, "ARCHIVE_PG_BIN=") {
		if out, err := exec.Command("cp", e2eCopyFlag, "-R", e2eTemplate+"/.", dir).CombinedOutput(); err != nil {
			t.Fatalf("copy template: %v %s", err, out)
		}
		pg := exec.Command("postgres", "-D", filepath.Join(dir, "pg"), "-c", "listen_addresses=", "-k", dir)
		pg.Env = append(os.Environ(), "LC_ALL=C")
		if err := pg.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			pg.Process.Signal(syscall.SIGINT)
			pg.Wait()
		})
		for deadline := time.Now().Add(30 * time.Second); exec.Command("pg_isready", "-q", "-h", dir).Run() != nil; time.Sleep(20 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatal("postgres did not start")
			}
		}
	}
	return &e2eInst{t: t, dir: dir, env: append([]string{"ARCHIVE_DATA=" + dir}, env...)}
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
	return &e2eInst{t: in.t, dir: dir, env: append([]string{"ARCHIVE_DATA=" + dir, "DATABASE_URL=host=" + in.dir + " user=archive dbname=archive",
		"ARCHIVE_SECRET_KEY=" + strings.TrimSpace(string(key))}, env...)}
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
	var once sync.Once
	s.stop = func() {
		once.Do(func() {
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
COMMENT ON TABLE person IS 'icon: user';`}
	in.push(with(e2eSchema, "zz.sql", extra["zz.sql"]))
	files := in.roundTrips()
	if _, ok := files["zz.sql"]; ok || !strings.Contains(files["person.sql"], "EXECUTE FUNCTION touch()") || !strings.Contains(files["person.sql"], "FUNCTION public.touch()") ||
		!strings.Contains(files["person.sql"], "person_name") || !strings.Contains(files["person.sql"], `'icon: user'`) ||
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

func TestUsersSpacesTokensAndSecretsAcrossEverySurface(t *testing.T) {
	t.Parallel()
	in := newInst(t, "ARCHIVE_ADMIN_EMAIL=alice@example.com")
	srv := in.serve()
	api := srv.url + "/api"

	in.must("user", "add", "bob@example.com", "-name", "Bob")
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
		role := in.psql("archive", "SELECT role FROM archive.\"user\" WHERE email = 'bob@example.com'")
		in.psql("archive", "ALTER ROLE "+role+" LOGIN")
		defer in.psql("archive", "ALTER ROLE "+role+" NOLOGIN")
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
		if c := cat.Tables["company"]; c == nil || c.col("vat") == nil || c.col("vat").Type != "vat_id" || c.Icon != "building" || cat.Types["archive.email"] == nil {
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

	t.Run("MagicLinkSignInFromTheLog", func(t *testing.T) {
		st, _ := call(t, "POST", srv.url+"/auth/magic", "", map[string]any{"email": "bob@example.com"}, nil)
		wantStatus(t, st, 202, "request magic link")
		st, _ = call(t, "POST", srv.url+"/auth/magic", "", map[string]any{"email": "nobody@example.com"}, nil)
		wantStatus(t, st, 202, "request magic link for unknown user")
		re := regexp.MustCompile(`sign-in link for bob@example\.com: (\S+)`)
		var link string
		for deadline := time.Now().Add(5 * time.Second); link == "" && time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
			if m := re.FindStringSubmatch(srv.log.String()); m != nil {
				link = m[1]
			}
		}
		if link == "" || strings.Contains(srv.log.String(), "nobody@example.com") {
			t.Fatalf("log:\n%s", srv.log)
		}
		resp, err := http.Get(link)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		wantStatus(t, resp.StatusCode, 200, "open magic link")
		var session *http.Cookie
		for _, c := range resp.Cookies() {
			if c.Name == "archive_session" && c.HttpOnly && c.SameSite == http.SameSiteLaxMode {
				session = c
			}
		}
		if session == nil {
			t.Fatalf("no HttpOnly Lax session cookie: %v", resp.Cookies())
		}
		var me struct {
			Actor Actor `json:"actor"`
		}
		call(t, "GET", srv.url+"/auth/me", "", nil, &me, session)
		if me.Actor.Email != "bob@example.com" {
			t.Fatalf("me = %+v", me.Actor)
		}
		var rs []row
		call(t, "GET", api+"/person", "", nil, &rs, session)
		if !strings.Contains(names(rs), "Bob private,Family member") {
			t.Errorf("session lists %q", names(rs))
		}
		again, _ := http.Get(link)
		again.Body.Close()
		wantStatus(t, again.StatusCode, 401, "reuse magic link")
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

func TestDeletingASpaceRemovesEverythingInItForOwnersOnly(t *testing.T) {
	t.Parallel()
	in := newInst(t, "ARCHIVE_ADMIN_EMAIL=alice@example.com")
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
	if !strings.Contains(errs, "personal space of alice@example.com") {
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

func TestFirstRunSetupTokenCreatesTheFirstOwner(t *testing.T) {
	t.Parallel()
	in := newInst(t)
	srv := in.serve()
	m := regexp.MustCompile(`/auth/setup \{"token": "([^"]+)"`).FindStringSubmatch(srv.log.String())
	if m == nil {
		t.Fatalf("no setup token in log:\n%s", srv.log)
	}
	if errs := in.fails("file", "list"); !strings.Contains(errs, "no users yet") {
		t.Fatalf("list before setup: %s", errs)
	}
	st, _ := call(t, "POST", srv.url+"/auth/setup", "", map[string]any{"token": m[1], "email": "first@example.com", "name": "First"}, nil)
	wantStatus(t, st, 200, "setup token creates the first owner")
	st, _ = call(t, "POST", srv.url+"/auth/setup", "", map[string]any{"token": m[1], "email": "second@example.com"}, nil)
	wantStatus(t, st, 401, "setup token used twice")
	spaces := decode[[]SpaceInfo](t, in.must("space", "list"))
	if len(spaces) != 1 || spaces[0].Level != "owner" {
		t.Errorf("spaces: %+v", spaces)
	}
}

func TestWithoutPostgresInstalledTheFirstRunDownloadsItIntoTheDataDir(t *testing.T) {
	t.Parallel()
	if os.Getenv("CI") == "" {
		t.Skip("downloads Postgres; runs in CI")
	}
	path := "PATH=/usr/bin:/bin:/usr/sbin:/sbin"
	for _, name := range []string{"initdb", "postgres"} {
		for _, d := range filepath.SplitList(strings.TrimPrefix(path, "PATH=")) {
			if _, err := os.Stat(filepath.Join(d, name)); err == nil {
				t.Skipf("%s is installed in %s", name, d)
			}
		}
	}
	in := newInst(t, path, "ARCHIVE_PG_BIN=")
	_, errs, err := in.run("", "types")
	if err != nil || !strings.Contains(errs, "downloading Postgres "+pgVersion) {
		t.Fatalf("first run: %v\n%s", err, errs)
	}
	if _, err := os.Stat(filepath.Join(in.dir, "pgbin", pgVersion, "bin", "postgres")); err != nil {
		t.Fatal(err)
	}
	srv := in.serve()
	if log := srv.log.String(); strings.Contains(log, "downloading") {
		t.Fatalf("second run:\n%s", log)
	}
	if out := in.must("types"); !strings.Contains(out, "archive.email") {
		t.Errorf("types while serve runs: %s", out)
	}
}

func TestDockerImageServesAndAcceptsCLICommandsFromDockerExec(t *testing.T) {
	t.Parallel()
	if os.Getenv("CI") == "" {
		t.Skip("builds the Docker image; runs in CI")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("docker is not available")
	}
	name := fmt.Sprintf("archive-e2e-%d", time.Now().UnixNano())
	docker := func(stdin string, args ...string) string {
		t.Helper()
		c := exec.Command("docker", args...)
		c.Stdin = strings.NewReader(stdin)
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("docker %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return string(out)
	}
	t.Cleanup(func() {
		exec.Command("docker", "rm", "-f", name).Run()
		exec.Command("docker", "volume", "rm", "-f", name).Run()
		exec.Command("docker", "rmi", "-f", name).Run()
	})
	docker("", "build", "-q", "-t", name, "..")
	docker("", "run", "-d", "--name", name, "-v", name+":/data", "-e", "ARCHIVE_ADMIN_EMAIL=ops@example.com", name)
	for deadline := time.Now().Add(90 * time.Second); ; time.Sleep(500 * time.Millisecond) {
		if exec.Command("docker", "exec", name, "wget", "-q", "-O", "/dev/null", "http://127.0.0.1:8080/healthz").Run() == nil {
			break
		}
		if time.Now().After(deadline) {
			out, _ := exec.Command("docker", "logs", name).CombinedOutput()
			t.Fatalf("container not healthy:\n%s", out)
		}
	}
	docker(e2eSchema["company.sql"], "exec", "-i", name, "sh", "-c", "mkdir -p /tmp/ddl && cat > /tmp/ddl/company.sql")
	docker("", "exec", name, "archive", "schema", "apply", "/tmp/ddl")
	docker("", "exec", name, "archive", "create", "company", `{"name":"Docker Inc"}`)
	if out := docker("", "exec", name, "archive", "list", "company"); !strings.Contains(out, "Docker Inc") {
		t.Fatalf("docker exec list: %s", out)
	}
	if logs := docker("", "logs", name); strings.Contains(logs, "already running") {
		t.Errorf("logs: %s", logs)
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
	in := newInst(t, "ARCHIVE_ADMIN_EMAIL=alice@example.com")
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
		blobs, _ := filepath.Glob(filepath.Join(in.dir, "blobs", "*", bigFile.Hash))
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
		role := in.psql("archive", "SELECT role FROM archive.\"user\" WHERE email = 'bob@example.com'")
		in.psql("archive", "ALTER ROLE "+role+" LOGIN")
		if got := in.psql(role, "SELECT count(*) FROM archive.file WHERE name = 'photo.jpg'"); got != "0" {
			t.Errorf("psql as bob sees %s photos", got)
		}
		in.psql("archive", "ALTER ROLE "+role+" NOLOGIN")
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

type e2eS3 struct {
	mu      sync.Mutex
	objects map[string][]byte
	puts    int
}

func (s *e2eS3) handler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "AWS4-HMAC-SHA256 Credential=key-id/") || !strings.Contains(auth, "/eu-test/s3/aws4_request, SignedHeaders=host;x-amz-content-sha256;x-amz-date, Signature=") {
			w.WriteHeader(403)
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		b, ok := s.objects[r.URL.Path]
		switch r.Method {
		case "PUT":
			body, _ := io.ReadAll(r.Body)
			if sum := sha256.Sum256(body); hex.EncodeToString(sum[:]) != r.Header.Get("X-Amz-Content-Sha256") {
				w.WriteHeader(400)
				return
			}
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

func TestS3BlobBackendAgainstAFakeS3(t *testing.T) {
	t.Parallel()
	s3 := &e2eS3{objects: map[string][]byte{}}
	fake := httptest.NewServer(s3.handler(t))
	defer fake.Close()
	in := newInst(t, "ARCHIVE_BLOB_URL=s3://photos/archive?endpoint="+url.QueryEscape(fake.URL)+"&region=eu-test",
		"ARCHIVE_BLOB_KEY=key-id", "ARCHIVE_BLOB_SECRET=key-secret")
	in.must("user", "add", "ann@example.com")
	content := bytes.Repeat([]byte("0123456789"), 5000)
	path := filepath.Join(in.dir, "numbers.txt")
	os.WriteFile(path, content, 0o644)
	f := decode[File](t, in.must("file", "add", path))
	os.WriteFile(filepath.Join(in.dir, "copy.txt"), content, 0o644)
	in.must("file", "add", filepath.Join(in.dir, "copy.txt"))
	key := "/photos/archive/" + f.Hash[:2] + "/" + f.Hash
	s3.mu.Lock()
	_, stored := s3.objects[key]
	puts := s3.puts
	s3.mu.Unlock()
	if !stored || puts != 1 || f.Status != "done" {
		t.Fatalf("object %s stored %v, %d puts, file %+v", key, stored, puts, f)
	}
	if _, err := os.Stat(filepath.Join(in.dir, "blobs")); err == nil {
		t.Error("blobs written to the data directory too")
	}
	if out := in.must("file", "get", f.ID.String(), "-o", "-"); out != string(content) {
		t.Errorf("download through s3: %d bytes", len(out))
	}
	srv := in.serve()
	tok := decode[Token](t, in.must("token", "create")).Secret
	resp, body := e2eGet(t, srv.url+"/api/files/"+f.ID.String()+"/content", tok, "Range", "bytes=49990-")
	if resp.StatusCode != 206 || string(body) != "0123456789" {
		t.Errorf("range through s3: %d %q", resp.StatusCode, body)
	}
	resp, body = e2eGet(t, srv.url+"/api/files/"+f.ID.String()+"/content", tok, "Range", "bytes=5-14")
	if resp.StatusCode != 206 || string(body) != "5678901234" {
		t.Errorf("middle range through s3: %d %q", resp.StatusCode, body)
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
	in := newInst(t, "ARCHIVE_ADMIN_EMAIL=alice@example.com")
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
		role := in.psql("archive", "SELECT role FROM archive.\"user\" WHERE email = 'bob@example.com'")
		in.psql("archive", "ALTER ROLE "+role+" LOGIN")
		defer in.psql("archive", "ALTER ROLE "+role+" NOLOGIN")
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
	in := newInst(t, "ARCHIVE_ADMIN_EMAIL=alice@example.com", `ARCHIVE_OIDC=[{"name":"google","issuer":"https://accounts.google.com","client_id":"web"}]`)
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
			"COMMENT ON VIEW people_board IS 'layout: board\nby: employer\nicon: users';\n"
		in.push(with(e2eSchema, "people_board.sql", board))
		var fam, solo row
		st, _ := call(t, "POST", api+"/person?space=family", aliceTok, map[string]any{"name": "Fam"}, &fam)
		wantStatus(t, st, 201, "person in family")
		st, _ = call(t, "POST", api+"/person", aliceTok, map[string]any{"name": "Solo"}, &solo)
		wantStatus(t, st, 201, "person in alice's space")

		var cat Catalog
		call(t, "GET", api+"/types", aliceTok, nil, &cat)
		v := cat.Tables["people_board"]
		if v == nil || v.Kind != "view" || v.Layout != "board" || v.By != "employer" || v.Icon != "users" || v.col("name") == nil {
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

		view := "CREATE VIEW bad WITH (security_invoker = true) AS SELECT * FROM person;\nCOMMENT ON VIEW bad IS '"
		for _, bad := range []struct{ sql, want string }{
			{"CREATE VIEW bad AS SELECT * FROM person;", "security_invoker = true"},
			{view + "layout: board\nby: nope';", `no column "nope"`},
			{view + "colour: red';", "unknown key colour"},
			{view + "icon: not-an-icon';", "unknown icon"},
			{view + "layout: calendar';", "needs a by:"},
			{view + "Everyone you know\nicon: users';", "there are no descriptions"},
			{"CREATE VIEW bad WITH (security_invoker = true) AS SELECT name FROM person;", "want an id uuid column"},
		} {
			if e := in.fails("schema", "apply", in.writeDir("bad", with(e2eSchema, "bad.sql", bad.sql))); !strings.Contains(e, bad.want) {
				t.Errorf("%s: %s", bad.sql, e)
			}
		}
		company := strings.Replace(e2eSchema["company.sql"], "'icon: building'", "'icon: building\nlayout: board'", 1)
		if e := in.fails("schema", "apply", in.writeDir("bad", with(e2eSchema, "company.sql", company))); !strings.Contains(e, "unknown key layout") {
			t.Errorf("layout on a table: %s", e)
		}

		code := `const files = await archive.schema();
files["open_people.sql"] = "CREATE VIEW open_people WITH (security_invoker = true) AS SELECT * FROM person WHERE name <> 'Solo';\n" +
  "COMMENT ON VIEW open_people IS 'layout: board\nby: employer\nicon: users';";
const made = await archive.apply(files, { confirm: true });
const t1 = (await archive.tables()).find((t) => t.name === "open_people");
const rows1 = (await archive.list("open_people")).map((r) => r.name);
files["open_people.sql"] = "CREATE VIEW open_people WITH (security_invoker = true) AS SELECT * FROM person WHERE name = 'Solo';\n" +
  "COMMENT ON VIEW open_people IS 'icon: user';";
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
		if r.T1.Kind != "view" || r.T1.Layout != "board" || r.T1.By != "employer" || r.T1.Icon != "users" || strings.Join(r.Rows1, ",") != "Fam" {
			t.Errorf("created view: %+v %v", r.T1, r.Rows1)
		}
		if r.T2.Layout != "table" || r.T2.By != "" || r.T2.Icon != "user" || strings.Join(r.Rows2, ",") != "Solo" {
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

	t.Run("SignInMethodsAndMagicLink", func(t *testing.T) {
		var methods struct {
			Providers []string `json:"providers"`
			Mail      bool     `json:"mail"`
		}
		st, _ := call(t, "GET", srv.url+"/auth/methods", "", nil, &methods)
		wantStatus(t, st, 200, "sign-in methods without a session")
		if !slices.Equal(methods.Providers, []string{"google"}) || methods.Mail {
			t.Errorf("methods: %+v", methods)
		}
		call(t, "POST", srv.url+"/auth/magic", "", map[string]any{"email": "bob@example.com"}, nil)
		re := regexp.MustCompile(`sign-in link for bob@example\.com: (\S+)`)
		var link string
		for deadline := time.Now().Add(5 * time.Second); link == "" && time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
			if m := re.FindStringSubmatch(srv.log.String()); m != nil {
				link = m[1]
			}
		}
		if link == "" {
			t.Fatalf("log:\n%s", srv.log)
		}
		resp, err := http.Get(link)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 || !slices.ContainsFunc(resp.Cookies(), func(c *http.Cookie) bool { return c.Name == "archive_session" && c.Value != "" }) {
			t.Errorf("magic link: %d %v", resp.StatusCode, resp.Cookies())
		}
		if resp, err := http.Get(link); err != nil || resp.StatusCode < 400 {
			t.Errorf("used magic link: %v %v", resp, err)
		}
		call(t, "POST", srv.url+"/auth/magic", "", map[string]any{"email": "bob@example.com"}, nil)
		var browser string
		for deadline := time.Now().Add(5 * time.Second); browser == "" && time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
			if m := re.FindAllStringSubmatch(srv.log.String(), -1); len(m) > 1 {
				browser = m[len(m)-1][1]
			}
		}
		req, _ := http.NewRequest("GET", browser, nil)
		req.Header.Set("Accept", "text/html,application/xhtml+xml")
		noFollow := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		resp, err = noFollow.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/" || !slices.ContainsFunc(resp.Cookies(), func(c *http.Cookie) bool { return c.Name == "archive_session" && c.Value != "" }) {
			t.Errorf("magic link in a browser: %d %q %v", resp.StatusCode, resp.Header.Get("Location"), resp.Cookies())
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
COMMENT ON TABLE house IS 'icon: house';
`

type e2eBearer string

func (b e2eBearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+string(b))
	return http.DefaultTransport.RoundTrip(r)
}

func TestCodeModeRunsTypeScriptInOneTransactionOverHTTPMCPAndCLI(t *testing.T) {
	t.Parallel()
	in := newInst(t, "ARCHIVE_ADMIN_EMAIL=alice@example.com", "ARCHIVE_RUN_TIMEOUT=30s")
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
			"interface House extends Base {\n    address: string;\n    price?: Money;\n    sold?: boolean;\n}", "house: House;", "type Email = string;", "read(fileId: string): Promise<Model>;", "edit(fileId: string, edits: Edit[]): Promise<StoredFile>;"} {
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
		got := ok(t, `const [doc, sheet, deck, page] = await Promise.all(["Plan.docx", "Costs.xlsx", "Pitch.pptx", "Notes.page"].map(n => archive.create(n)));
const blank = { doc: await archive.read(doc.id), sheet: await archive.read(sheet.id), deck: await archive.read(deck.id), page: await archive.read(page.id) };
await archive.edit(doc.id, [{ op: "insert_paragraph_after", para: blank.doc.paragraphs[0].id, text: "Kick-off", style: "Heading1" }]);
await archive.edit(sheet.id, [{ op: "set_cell", cell: "A1", value: "Rent" }, { op: "set_cell", cell: "B1", value: 1200 }, { op: "set_formula", cell: "B2", formula: "B1*12" }]);
await archive.edit(deck.id, [{ op: "replace_shape_text", slide: 1, shape: 0, text: "Series A" }, { op: "replace_shape_text", slide: 1, shape: 1, text: "October 2026" }]);
await archive.edit(page.id, [{ op: "insert_paragraph_after", text: "Agenda", style: "heading1" }]);
let bad = "";
try { await archive.create("Plan.txt"); } catch (e) { bad = e.message; }
const d = await archive.read(doc.id), x = await archive.read(sheet.id), k = await archive.read(deck.id), p = await archive.read(page.id);
return { names: [doc.name, sheet.name, deck.name, page.name].join(","), blankDoc: blank.doc.paragraphs.length, blankSheet: blank.sheet.sheets.map(s => s.name + ":" + s.cells.length).join(","),
  blankDeck: blank.deck.slides[0].shapes.length, blankPage: blank.page.paragraphs.length,
  doc: d.paragraphs.map(p => (p.style || "") + ":" + p.text).join("|"), sheet: x.sheets[0].cells.map(c => c.ref + "=" + (c.formula || c.value)).join(","),
  deck: k.slides[0].shapes.map(s => s.paragraphs.map(p => p.text).join("")).join("|"), page: p.paragraphs.map(p => p.style + ":" + p.text).join("|"), bad };`)
		want := map[string]string{"names": "Plan.docx,Costs.xlsx,Pitch.pptx,Notes.page", "blankDoc": "1", "blankSheet": "Sheet1:0", "blankDeck": "2", "blankPage": "0",
			"doc": ":|Heading1:Kick-off", "sheet": "A1=Rent,B1=1200,B2=B1*12", "deck": "Series A|October 2026", "page": "heading1:Agenda"}
		for k, v := range want {
			if fmt.Sprint(got[k]) != v {
				t.Errorf("%s = %v, want %s", k, got[k], v)
			}
		}
		if bad, _ := got["bad"].(string); !strings.Contains(bad, ".page, .docx, .xlsx or .pptx") {
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
		if fmt.Sprint(got["ddl"]) != `[ALTER TABLE house RENAME COLUMN price TO asking_price]` || fmt.Sprint(got["price"]) != "500000" {
			t.Errorf("rename: %v", got)
		}
		dry := ok(t, `const files = await archive.schema();
files["house.sql"] = files["house.sql"].replaceAll("asking_price", "list_price");
const plan = await archive.apply(files, { renames: ["ALTER TABLE house RENAME COLUMN asking_price TO list_price;"] });
const cols = await archive.sql("SELECT column_name FROM information_schema.columns WHERE table_name = 'house' AND column_name LIKE '%price'");
return { ddl: plan.statements.map(s => s.ddl), applied: plan.applied, cols: cols.map(c => c.column_name) };`)
		if fmt.Sprint(dry["ddl"]) != `[ALTER TABLE house RENAME COLUMN asking_price TO list_price]` || dry["applied"] != false || fmt.Sprint(dry["cols"]) != "[asking_price]" {
			t.Errorf("a dry run lists its renames and changes nothing: %v", dry)
		}
		if out := run(t, tok, `await archive.apply(await archive.schema(), { renames: ["DROP TABLE house"] })`); !strings.Contains(out.Error, "renames[0]: want one ALTER") {
			t.Errorf("a rename that drops: %q", out.Error)
		}
	})
}

func TestPagesMergeManyAgentsEditingAtOnceAndSurviveARestart(t *testing.T) {
	t.Parallel()
	in := newInst(t, "ARCHIVE_ADMIN_EMAIL=alice@example.com", "ARCHIVE_RUN_TIMEOUT=60s")
	srv := in.serve()
	in.must("user", "add", "bob@example.com")
	tok := decode[Token](t, in.must("token", "create")).Secret
	bobTok := decode[Token](t, in.must("-as", "bob@example.com", "token", "create")).Secret
	run := func(t *testing.T, tok, code string, out any) string {
		t.Helper()
		var res RunOutput
		st, _ := call(t, "POST", srv.url+"/api/run", tok, map[string]string{"code": code}, &res)
		wantStatus(t, st, 200, "run")
		if res.Error == "" && out != nil {
			if err := json.Unmarshal(res.Result, out); err != nil {
				t.Fatalf("%s: %v", res.Result, err)
			}
		}
		return res.Error
	}
	read := func(t *testing.T, id string) OfficeModel {
		t.Helper()
		var m OfficeModel
		if e := run(t, tok, `return await archive.read(`+strconv.Quote(id)+`)`, &m); e != "" {
			t.Fatal(e)
		}
		return m
	}
	texts := func(m OfficeModel) []string {
		var out []string
		for _, p := range m.Paragraphs {
			out = append(out, p.Style+":"+p.Text)
		}
		return out
	}
	const n = 30
	var tokens, inserts, want []string
	for i := range n {
		tokens = append(tokens, fmt.Sprintf("[t%d]", i))
		inserts = append(inserts, fmt.Sprintf(`{ op: "insert_paragraph_after", para: h, text: "item %d" }`, i))
		want = append(want, fmt.Sprintf("paragraph:item %d", i))
	}
	want = append([]string{"heading1:Renovation plan", "paragraph:Shared", "paragraph:" + strings.Join(tokens, " ")}, want...)
	var page string
	var start OfficeModel

	t.Run("CreateReadAndEditRoundTrip", func(t *testing.T) {
		var got struct {
			File File
			Page OfficeModel
		}
		if e := run(t, tok, `const file = await archive.create("Renovation.page");
const empty = await archive.read(file.id);
if (empty.paragraphs.length !== 0) throw new Error("not empty: " + JSON.stringify(empty));
await archive.edit(file.id, [{ op: "insert_paragraph_after", text: "Renovation plan", style: "heading1" }]);
const m = await archive.read(file.id);
const h = m.paragraphs[0].id;
await archive.edit(file.id, [{ op: "insert_paragraph_after", para: h, text: "Shared" },
  { op: "insert_paragraph_after", para: h, text: `+strconv.Quote(strings.Join(tokens, " "))+` }, `+strings.Join(inserts, ", ")+`]);
return { file: await archive.file(file.id), page: await archive.read(file.id) };`, &got); e != "" {
			t.Fatal(e)
		}
		page, start = got.File.ID.String(), got.Page
		if got.File.Name != "Renovation.page" || got.File.MIME != "application/vnd.archive.page" || got.File.Status != "done" {
			t.Errorf("file: %+v", got.File)
		}
		if got := texts(start); !slices.Equal(got, want) {
			t.Fatalf("page:\n%q\nwant\n%q", got, want)
		}
		if start.Type != "page" || !regexp.MustCompile(`^\d+@\d+$`).MatchString(start.Paragraphs[0].ID) {
			t.Errorf("model: %+v", start)
		}
		var hits []SearchHit
		if e := run(t, tok, `return await archive.search("renovation plan", { type: "file" })`, &hits); e != "" || len(hits) != 1 || hits[0].ID.String() != page {
			t.Errorf("search: %s %+v", e, hits)
		}
	})

	t.Run("EditsAreAllOrNothingAndErrorsSayWhy", func(t *testing.T) {
		h := start.Paragraphs[0].ID
		for code, want := range map[string]string{
			`[{ op: "insert_paragraph_after", para: "` + h + `", text: "lost" }, { op: "delete_paragraph", para: "999@9" }]`: "edit 1 (delete_paragraph): paragraph 999@9 does not exist",
			`[{ op: "replace_text", find: "item", replace: "x" }]`:                                                           "occurs in 30 paragraphs",
			`[{ op: "set_cell", cell: "A1", value: 1 }]`:                                                                     "page ops are replace_text, insert_paragraph_after, delete_paragraph, set_style",
			`[{ op: "replace_text", replace: "x" }]`:                                                                         `needs "find", or "para"`,
		} {
			if e := run(t, tok, `await archive.edit(`+strconv.Quote(page)+`, `+code+`)`, nil); !strings.Contains(e, want) {
				t.Errorf("%s: %q, want %q", code, e, want)
			}
		}
		if e := run(t, bobTok, `return await archive.read(`+strconv.Quote(page)+`)`, nil); !strings.Contains(e, "not found") {
			t.Errorf("bob reads alice's page: %q", e)
		}
		if got := read(t, page); !slices.Equal(texts(got), want) {
			t.Errorf("a failed edit changed the page: %q", texts(got))
		}
	})

	t.Run("ManyAgentsAtOnceMergeAndOneRewriteWins", func(t *testing.T) {
		h, shared, toks := start.Paragraphs[0].ID, start.Paragraphs[1].ID, start.Paragraphs[2].ID
		var ready, wg sync.WaitGroup
		gate := make(chan struct{})
		errs := make([]string, n)
		after := make([]OfficeModel, n)
		ready.Add(n)
		for i := range n {
			wg.Add(1)
			go func() {
				defer wg.Done()
				var mine OfficeModel
				errs[i] = run(t, tok, `return await archive.read(`+strconv.Quote(page)+`)`, &mine)
				ready.Done()
				<-gate
				if errs[i] != "" {
					return
				}
				errs[i] = run(t, tok, fmt.Sprintf(`const id = %q;
await archive.edit(id, [{ op: "insert_paragraph_after", para: %q, text: "note %d" },
  { op: "replace_text", para: %q, find: "item", replace: "done item" },
  { op: "replace_text", para: %q, find: "[t%d]", replace: "[T%d]" },
  { op: "replace_text", para: %q, replace: "winner %d" }]);
return await archive.read(id);`, page, h, i, mine.Paragraphs[3+i].ID, toks, i, i, shared, i), &after[i])
			}()
		}
		ready.Wait()
		close(gate)
		wg.Wait()
		for i, e := range errs {
			if e != "" {
				t.Fatalf("agent %d: %s", i, e)
			}
		}
		m := read(t, page)
		if len(m.Paragraphs) != len(start.Paragraphs)+n {
			t.Fatalf("%d paragraphs, want %d: %q", len(m.Paragraphs), len(start.Paragraphs)+n, texts(m))
		}
		var notes []string
		for _, p := range m.Paragraphs[1 : n+1] {
			notes = append(notes, p.Text)
		}
		for i := range n {
			if !slices.Contains(notes, fmt.Sprintf("note %d", i)) {
				t.Errorf("note %d is missing: %q", i, notes)
			}
		}
		for i, p := range append([]OfficeParagraph{m.Paragraphs[0]}, m.Paragraphs[n+1:]...) {
			if p.ID != start.Paragraphs[i].ID {
				t.Errorf("paragraph %d changed id from %s to %s", i, start.Paragraphs[i].ID, p.ID)
			}
		}
		for i := range n {
			if p := m.Paragraphs[n+3+i]; p.Text != fmt.Sprintf("done item %d", i) {
				t.Errorf("item %d reads %q", i, p.Text)
			}
		}
		if got := m.Paragraphs[n+2].Text; got != strings.ReplaceAll(strings.Join(tokens, " "), "[t", "[T") {
			t.Errorf("character edits to one paragraph did not all survive: %q", got)
		}
		var last []int
		for i, a := range after {
			if len(a.Paragraphs) == len(m.Paragraphs) {
				last = append(last, i)
			}
		}
		if winner := m.Paragraphs[n+1].Text; len(last) != 1 || winner != fmt.Sprintf("winner %d", last[0]) {
			t.Errorf("same-paragraph rewrites ended as %q; agents that saw the final page: %v", winner, last)
		}
		start = m
	})

	t.Run("ThePageSurvivesARestartAndKeepsMintingFreshIds", func(t *testing.T) {
		srv.stop()
		srv = in.serve()
		m := read(t, page)
		if !slices.Equal(texts(m), texts(start)) || m.Paragraphs[5].ID != start.Paragraphs[5].ID {
			t.Fatalf("after restart:\n%q\nwant\n%q", texts(m), texts(start))
		}
		h := m.Paragraphs[0].ID
		if e := run(t, tok, `await archive.edit(`+strconv.Quote(page)+`, [{ op: "insert_paragraph_after", para: "`+h+`", text: "after the restart", style: "todo" }, { op: "set_style", para: "`+h+`", style: "heading2" }])`, nil); e != "" {
			t.Fatal(e)
		}
		got := read(t, page)
		seen := map[string]bool{}
		for _, p := range got.Paragraphs {
			if seen[p.ID] {
				t.Errorf("id %s is used twice", p.ID)
			}
			seen[p.ID] = true
		}
		if len(got.Paragraphs) != len(m.Paragraphs)+1 || texts(got)[0] != "heading2:Renovation plan" || texts(got)[1] != "todo:after the restart" {
			t.Errorf("edit after restart: %q", texts(got))
		}
		var f runFile
		if e := run(t, tok, `return await archive.file(`+strconv.Quote(page)+`)`, &f); e != "" || !strings.HasPrefix(f.Text, "Renovation plan\nafter the restart\nnote ") {
			t.Errorf("extracted text: %s %q", e, f.Text)
		}
	})
}

func TestPageEditorsSyncLiveAcrossInstancesWithAgentsAndSeeWhoIsThere(t *testing.T) {
	t.Parallel()
	in := newInst(t, "ARCHIVE_ADMIN_EMAIL=alice@example.com")
	near, srv := in.serve(), in.twin().serve()
	tok := decode[Token](t, in.must("token", "create")).Secret
	run := func(code string, out any) string {
		t.Helper()
		var res RunOutput
		st, _ := call(t, "POST", srv.url+"/api/run", tok, map[string]string{"code": code}, &res)
		wantStatus(t, st, 200, "run")
		if res.Error == "" && out != nil {
			if err := json.Unmarshal(res.Result, out); err != nil {
				t.Fatalf("%s: %v", res.Result, err)
			}
		}
		return res.Error
	}
	var page, other File
	if e := run(`return await archive.create("Live.page")`, &page); e != "" {
		t.Fatal(e)
	}
	if e := run(`return await archive.create("Not live.docx")`, &other); e != "" {
		t.Fatal(e)
	}
	base := srv.url + "/api/pages/" + page.ID.String()
	type event struct{ kind, data string }
	open := func(on *e2eServer, site int) (chan event, context.CancelFunc) {
		ctx, cancel := context.WithCancel(context.Background())
		req, _ := http.NewRequestWithContext(ctx, "GET", on.url+"/api/pages/"+page.ID.String()+"/sync?site="+strconv.Itoa(site), nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		wantStatus(t, resp.StatusCode, 200, "sync")
		ch := make(chan event, 64)
		go func() {
			defer resp.Body.Close()
			sc, kind := bufio.NewScanner(resp.Body), "message"
			sc.Buffer(nil, 1<<20)
			for sc.Scan() {
				line := sc.Text()
				switch {
				case strings.HasPrefix(line, "event: "):
					kind = line[7:]
				case strings.HasPrefix(line, "data: "):
					ch <- event{kind, line[6:]}
					kind = "message"
				}
			}
			close(ch)
		}()
		t.Cleanup(cancel)
		return ch, cancel
	}
	next := func(ch chan event, kind string, want func([]byte) bool) []byte {
		t.Helper()
		timeout := time.After(10 * time.Second)
		for {
			select {
			case e, ok := <-ch:
				if !ok {
					t.Fatalf("stream ended waiting for %s", kind)
				}
				b, err := base64.StdEncoding.DecodeString(e.data)
				if err != nil {
					t.Fatal(err)
				}
				if e.kind == kind && (want == nil || want(b)) {
					return b
				}
			case <-timeout:
				t.Fatalf("no %s event", kind)
			}
		}
	}
	post := func(path string, b []byte) int {
		req, _ := http.NewRequest("POST", base+path, bytes.NewReader(b))
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	load := func(ch chan event, site crdt.SiteID) *structured.Blocks {
		d, err := structured.LoadBlocks(site, next(ch, "snapshot", nil))
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	apply := func(d *structured.Blocks, b []byte) {
		ops, err := crdt.ParsePartOps(b)
		if err == nil {
			err = d.Apply(ops...)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	list := func(d *structured.Blocks) []string {
		var out []string
		for _, b := range d.List() {
			out = append(out, b.Type+":"+b.Text)
		}
		return out
	}
	server := func() []string {
		var m OfficeModel
		if e := run(`return await archive.read(`+strconv.Quote(page.ID.String())+`)`, &m); e != "" {
			t.Fatal(e)
		}
		var out []string
		for _, p := range m.Paragraphs {
			out = append(out, p.Style+":"+p.Text)
		}
		return out
	}

	ada, stopAda := open(near, 5)
	grace, _ := open(near, 6)
	hopper, _ := open(srv, 7)
	adaDoc, graceDoc := load(ada, 5), load(grace, 6)
	load(hopper, 7)

	h, ops, err := adaDoc.Insert(structured.DocStart, "heading1")
	if err != nil {
		t.Fatal(err)
	}
	text, err := adaDoc.InsertText(h, 0, "Lisbon trip")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := crdt.AppendPartOps(nil, append(ops, text))
	if st := post("/ops", b); st != http.StatusNoContent {
		t.Fatalf("ops: %d", st)
	}
	apply(graceDoc, next(grace, "message", nil))
	if got := list(graceDoc); !slices.Equal(got, []string{"heading1:Lisbon trip"}) {
		t.Fatalf("grace sees %q", got)
	}

	if e := run(`await archive.edit(`+strconv.Quote(page.ID.String())+`, [{ op: "insert_paragraph_after", para: "`+h.String()+`", text: "rolled back" }]); throw new Error("agent gave up")`, nil); !strings.Contains(e, "agent gave up") {
		t.Fatalf("failing run: %q", e)
	}
	if e := run(`await archive.edit(`+strconv.Quote(page.ID.String())+`, [{ op: "insert_paragraph_after", para: "`+h.String()+`", text: "Book flights", style: "todo" }])`, nil); e != "" {
		t.Fatal(e)
	}
	apply(graceDoc, next(grace, "message", nil))
	if got, want := list(graceDoc), server(); !slices.Equal(got, want) || !slices.Equal(want, []string{"heading1:Lisbon trip", "todo:Book flights"}) {
		t.Fatalf("grace sees %q, the server has %q", got, want)
	}
	next(grace, "presence", func(b []byte) bool {
		var u awareness.Update
		return u.UnmarshalBinary(b) == nil && u.Site == 1 && u.Meta["agent"] == "1"
	})

	reg := awareness.New()
	at, _ := adaDoc.At(h, 6)
	u, _ := reg.Publish(5, awareness.Cursor{Anchor: at, Head: at}, map[string]string{"name": "Ada"}).MarshalBinary()
	if st := post("/presence", u); st != http.StatusNoContent {
		t.Fatalf("presence: %d", st)
	}
	var seen awareness.Update
	next(grace, "presence", func(b []byte) bool { return seen.UnmarshalBinary(b) == nil && seen.Site == 5 })
	if seen.Gone || seen.Meta["name"] != "Ada" || seen.Cursor.Head != at {
		t.Errorf("grace sees ada as %+v", seen)
	}
	stopAda()
	next(grace, "presence", func(b []byte) bool { return seen.UnmarshalBinary(b) == nil && seen.Site == 5 && seen.Gone })
	next(hopper, "presence", func(b []byte) bool { return seen.UnmarshalBinary(b) == nil && seen.Site == 5 && seen.Gone })

	fake, _ := (&awareness.Update{Site: 1, Clock: 99, Meta: map[string]string{"name": "Agent"}}).MarshalBinary()
	for path, b := range map[string][]byte{"/ops": []byte("not ops"), "/presence": fake} {
		if st := post(path, b); st != http.StatusBadRequest {
			t.Errorf("%s with bad input: %d", path, st)
		}
	}
	if st, _ := call(t, "GET", srv.url+"/api/pages/"+other.ID.String()+"/sync", tok, nil, nil); st != http.StatusNotFound {
		t.Errorf("sync on a docx: %d", st)
	}
}
