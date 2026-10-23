package main

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/emersion/go-imap/v2/imapserver/imapmemserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var e2eBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "arcbin")
	if err != nil {
		panic(err)
	}
	e2eBin = filepath.Join(dir, "archive")
	if out, err := exec.Command("go", "build", "-o", e2eBin, ".").CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		panic(fmt.Sprintf("go build: %v\n%s", err, out))
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

const e2eSpec = `{"types": {
  "company": {"fields": {"name": {"type": "text", "required": true}}},
  "contact": {"fields": {
    "name": {"type": "text", "required": true},
    "email": {"type": "email"},
    "phone": {"type": "phone"},
    "birthday": {"type": "date"},
    "balance": {"type": "money"}}},
  "person": {"fields": {
    "name": {"type": "text", "required": true},
    "ssn": {"type": "us_ssn"},
    "employer": {"type": "reference", "to": "company"}}},
  "invoice": {"fields": {
    "number": {"type": "text", "required": true},
    "total": {"type": "money"},
    "date": {"type": "date"},
    "bsn": {"type": "nl_bsn"}}}
}}`

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
	return &e2eInst{t: t, dir: dir, env: append([]string{"ARCHIVE_DATA=" + dir}, env...)}
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

func (in *e2eInst) writeSpec() string {
	p := filepath.Join(in.dir, "spec.json")
	if err := os.WriteFile(p, []byte(e2eSpec), 0o644); err != nil {
		in.t.Fatal(err)
	}
	return p
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
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("decode %q: %v", s, err)
	}
	return v
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
		if err := json.Unmarshal(b, out); err != nil {
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

type e2eRecord struct {
	ID    string         `json:"id"`
	Type  string         `json:"type"`
	Space string         `json:"space"`
	Data  map[string]any `json:"data"`
}

func names(rs []e2eRecord) string {
	var out []string
	for _, r := range rs {
		out = append(out, fmt.Sprint(r.Data["name"]))
	}
	return strings.Join(out, ",")
}

type bearerTransport string

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+string(b))
	return http.DefaultTransport.RoundTrip(r)
}

func mcpCall(t *testing.T, url, token, tool string, args map[string]any) (string, bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c := mcp.NewClient(&mcp.Implementation{Name: "e2e", Version: "0"}, nil)
	s, err := c.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: url + "/mcp", HTTPClient: &http.Client{Transport: bearerTransport(token)}}, nil)
	if err != nil {
		t.Fatalf("mcp connect: %v", err)
	}
	defer s.Close()
	res, err := s.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Fatalf("mcp %s: %v", tool, err)
	}
	var text strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text.WriteString(tc.Text)
		}
	}
	return text.String(), res.IsError
}

func TestSpecPushNormalizesMessyDutchInputAndKeepsBuiltinEditsAcrossRestarts(t *testing.T) {
	in := newInst(t, "ARCHIVE_LOCALE=nl-NL")
	in.fails("list", "contact")
	in.must("user", "add", "ann@example.com", "-name", "Ann")
	_, errs, err := in.run("", "spec", "push", in.writeSpec())
	if err != nil || !strings.Contains(errs, "applied change") {
		t.Fatalf("spec push: %v %s", err, errs)
	}

	r := decode[e2eRecord](t, in.must("create", "contact", `{"name":" Ann ","email":"Ann@EXAMPLE.com","phone":"06 12345678","birthday":"3-4-1990","balance":"€ 1.234,50"}`))
	want := map[string]any{"phone": "+31612345678", "birthday": "1990-04-03", "email": "Ann@example.com",
		"balance": map[string]any{"amount": "1234.50", "currency": "EUR"}}
	for k, v := range want {
		if fmt.Sprint(r.Data[k]) != fmt.Sprint(v) {
			t.Errorf("%s = %v, want %v", k, r.Data[k], v)
		}
	}

	errs = in.fails("create", "contact", `{"name":"Bad","email":"not an email","phone":"12"}`)
	if !strings.Contains(errs, "invalid contact") || !strings.Contains(errs, "email:") || !strings.Contains(errs, "phone:") {
		t.Errorf("validation errors: %s", errs)
	}

	if got := decode[[]e2eRecord](t, in.must("list", "contact", "phone=06-1234 5678")); len(got) != 1 || got[0].ID != r.ID {
		t.Errorf("filter on messy phone: %v", got)
	}
	if got := decode[[]e2eRecord](t, in.must("list", "contact", "phone=0687654321")); len(got) != 0 {
		t.Errorf("filter on other phone: %v", got)
	}

	full := decode[map[string]any](t, in.must("spec", "pull", "-all"))
	full["types"].(map[string]any)["email"].(map[string]any)["description"] = "Edited email"
	b, _ := json.Marshal(full)
	if _, errs, err := in.run(string(b), "spec", "push", "-"); err != nil {
		t.Fatalf("push edited built-in: %v %s", err, errs)
	}
	pulled := in.must("spec", "pull")
	if !strings.Contains(pulled, "Edited email") || !strings.Contains(in.must("types"), "built-in, edited") {
		t.Errorf("built-in edit did not survive a restart:\n%s", pulled)
	}

	withIcon := func(icon string) string {
		sp := decode[map[string]any](t, pulled)
		sp["types"].(map[string]any)["invoice"].(map[string]any)["icon"] = icon
		b, _ := json.Marshal(sp)
		return string(b)
	}
	out, errs, err := in.run(withIcon("receipt"), "spec", "push", "-")
	if err != nil || !strings.Contains(errs, "applied change") {
		t.Fatalf("push icon: %v %s", err, errs)
	}
	ops := decode[struct{ Change struct{ Ops []Op } }](t, out).Change.Ops
	if len(ops) != 1 || ops[0].Op != "change_type" || ops[0].Type != "invoice" || !strings.Contains(fmt.Sprint(ops[0].Def), "icon:receipt") {
		t.Errorf("icon change ops = %+v", ops)
	}
	sp := decode[struct{ Types map[string]Type }](t, in.must("spec", "pull"))
	if sp.Types["invoice"].Icon != "receipt" || sp.Types["invoice"].Fields["number"] == nil || sp.Types["person"].Icon != "" {
		t.Errorf("icon after push: invoice %+v, person %q", sp.Types["invoice"], sp.Types["person"].Icon)
	}
	for _, bad := range []string{"Receipt", "sticky note", "../x", strings.Repeat("a", 65)} {
		if _, errs, err := in.run(withIcon(bad), "spec", "push", "-"); err == nil || !strings.Contains(errs, "Lucide icon name") {
			t.Errorf("push icon %q: %v %s", bad, err, errs)
		}
	}
}

func TestIntakeMapsFilesThroughAFakeOpenAIServer(t *testing.T) {
	var calls int
	var mu sync.Mutex
	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		content := `{"number":"INV-7","total":"€ 1.234,50","date":"03-04-2025","bsn":"111222333"}`
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": content}}}})
	}))
	defer llm.Close()
	in := newInst(t, "ARCHIVE_LOCALE=nl-NL", "ARCHIVE_LLM_URL="+llm.URL+"/v1", "ARCHIVE_LLM_MODEL=fake")
	in.must("user", "add", "ann@example.com")
	in.must("spec", "push", in.writeSpec())
	inbox := filepath.Join(in.dir, "inbox")
	os.MkdirAll(inbox, 0o755)
	os.WriteFile(filepath.Join(inbox, "inv.txt"), []byte("Factuur INV-7, totaal € 1.234,50, datum 03-04-2025, BSN 111222333"), 0o644)

	res := decode[[]IntakeResult](t, in.must("intake", inbox, "-type", "invoice"))
	if len(res) != 1 || res[0].RecordID.String() == "00000000-0000-0000-0000-000000000000" || calls != 3 {
		t.Fatalf("intake: %+v (calls %d)", res, calls)
	}
	for _, f := range res[0].Fields {
		if f.Path == "bsn" && strings.Contains(fmt.Sprint(f.Value), "111222333") {
			t.Errorf("intake result shows the secret bsn: %v", f.Value)
		}
	}
	got := decode[e2eRecord](t, in.must("get", "invoice", res[0].RecordID.String()))
	if fmt.Sprint(got.Data["date"]) != "2025-04-03" || fmt.Sprint(got.Data["total"]) != "map[amount:1234.50 currency:EUR]" {
		t.Errorf("intake record: %v", got.Data)
	}
	if bsn, _ := got.Data["bsn"].(map[string]any); bsn["masked"] != "*****2333" {
		t.Errorf("bsn = %v, want masked", got.Data["bsn"])
	}
	prov := in.must("provenance", "invoice", res[0].RecordID.String())
	if !strings.Contains(prov, "intake:fake") || strings.Contains(prov, "111222333") {
		t.Errorf("provenance: %s", prov)
	}
	if again := decode[[]IntakeResult](t, in.must("intake", inbox, "-type", "invoice")); len(again) != 0 {
		t.Errorf("second intake read the file again: %+v", again)
	}
}

func TestUsersSpacesTokensAndSecretsAcrossEverySurface(t *testing.T) {
	in := newInst(t, "ARCHIVE_ADMIN_EMAIL=alice@example.com")
	srv := in.serve()
	api := srv.url + "/api"

	in.must("user", "add", "bob@example.com", "-name", "Bob")
	in.must("spec", "push", in.writeSpec())
	in.must("space", "add", "family")
	in.must("space", "grant", "family", "bob@example.com", "viewer")
	aliceTok := decode[Token](t, in.must("token", "create", "-name", "Claude")).Secret
	narrowTok := decode[Token](t, in.must("token", "create", "-name", "Narrow", "-space", "family:viewer")).Secret
	bobTok := decode[Token](t, in.must("-as", "bob@example.com", "token", "create")).Secret

	var alicePriv, bobPriv, acme, fam e2eRecord
	st, _ := call(t, "POST", api+"/person", aliceTok, map[string]any{"name": "Alice private"}, &alicePriv)
	wantStatus(t, st, 201, "alice creates in her space")
	st, _ = call(t, "POST", api+"/person", bobTok, map[string]any{"name": "Bob private"}, &bobPriv)
	wantStatus(t, st, 201, "bob creates in his space")
	call(t, "POST", api+"/company", aliceTok, map[string]any{"name": "Acme"}, &acme)
	st, _ = call(t, "POST", api+"/person?space=family", aliceTok, map[string]any{"name": "Family member", "ssn": "123-45-6789", "employer": acme.ID}, &fam)
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
		var a, b []e2eRecord
		call(t, "GET", api+"/person", aliceTok, nil, &a)
		call(t, "GET", api+"/person", bobTok, nil, &b)
		if names(a) != "Alice private,Family member" || names(b) != "Bob private,Family member" {
			t.Fatalf("alice sees %q, bob sees %q", names(a), names(b))
		}
		st, _ := call(t, "GET", api+"/person/"+alicePriv.ID, bobTok, nil, nil)
		wantStatus(t, st, 404, "bob reads alice's private record")
		st, _ = call(t, "DELETE", api+"/person/"+bobPriv.ID, aliceTok, nil, nil)
		wantStatus(t, st, 404, "alice deletes bob's private record")
	})

	t.Run("UsersOnlySeeTheirOwnSpacesThroughMCP", func(t *testing.T) {
		a, _ := mcpCall(t, srv.url, aliceTok, "query", map[string]any{"type": "person"})
		b, _ := mcpCall(t, srv.url, bobTok, "query", map[string]any{"type": "person"})
		if !strings.Contains(a, "Alice private") || strings.Contains(a, "Bob private") || !strings.Contains(b, "Bob private") || strings.Contains(b, "Alice private") {
			t.Fatalf("mcp alice: %s\nmcp bob: %s", a, b)
		}
	})

	t.Run("UsersOnlySeeTheirOwnSpacesThroughCLIWhileServeRuns", func(t *testing.T) {
		in := in.on(t)
		b := names(decode[[]e2eRecord](t, in.must("-as", "bob@example.com", "list", "person")))
		a := names(decode[[]e2eRecord](t, in.must("list", "person")))
		if a != "Alice private,Family member" || b != "Bob private,Family member" {
			t.Fatalf("cli alice %q, bob %q", a, b)
		}
	})

	t.Run("PlainPsqlAsTheUserRoleSeesWhatTheAppSees", func(t *testing.T) {
		in := in.on(t)
		role := in.psql("archive", "SELECT role FROM users WHERE email = 'bob@example.com'")
		in.psql("archive", "ALTER ROLE "+role+" LOGIN")
		got := in.psql(role, "SELECT string_agg(data->>'name', ',' ORDER BY id) FROM records WHERE type = 'person'")
		if got != "Bob private,Family member" {
			t.Fatalf("psql as bob: %q", got)
		}
		if n := in.psql(role, "SELECT count(*) FROM secrets"); n != "0" {
			t.Errorf("psql as bob sees %s secrets", n)
		}
		if n := in.psql(role, "SELECT count(*) FROM links"); n != "0" {
			t.Errorf("psql as bob sees %s links", n)
		}
		out, err := exec.Command("psql", "-X", "-h", in.dir, "-U", role, "-d", "archive", "-Atc", "INSERT INTO records (type, spec_version, data, space) SELECT 'person', 1, '{}', id FROM spaces WHERE name = 'family'").CombinedOutput()
		if err == nil || !strings.Contains(string(out), "row-level security") {
			t.Errorf("psql insert into family as viewer: %v %s", err, out)
		}
		in.psql("archive", "ALTER ROLE "+role+" NOLOGIN")
	})

	t.Run("ViewerCannotWrite", func(t *testing.T) {
		in := in.on(t)
		st, _ := call(t, "POST", api+"/person?space=family", bobTok, map[string]any{"name": "Intruder"}, nil)
		wantStatus(t, st, 403, "viewer creates")
		st, _ = call(t, "PATCH", api+"/person/"+fam.ID, bobTok, map[string]any{"name": "Changed"}, nil)
		wantStatus(t, st, 403, "viewer patches")
		st, _ = call(t, "DELETE", api+"/person/"+fam.ID, bobTok, nil, nil)
		wantStatus(t, st, 403, "viewer deletes")
		if _, isErr := mcpCall(t, srv.url, bobTok, "upsert", map[string]any{"type": "person", "id": fam.ID, "data": map[string]any{"name": "x"}}); !isErr {
			t.Error("viewer upsert over mcp succeeded")
		}
		in.fails("-as", "bob@example.com", "create", "person", "-space", "family", `{"name":"x"}`)
	})

	t.Run("NarrowedTokenSeesOnlyItsSpace", func(t *testing.T) {
		in := in.on(t)
		var rs []e2eRecord
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

	t.Run("LinkAcrossSpacesIsHidden", func(t *testing.T) {
		var a, b []Link
		call(t, "GET", api+"/person/"+fam.ID+"/links", aliceTok, nil, &a)
		call(t, "GET", api+"/person/"+fam.ID+"/links", bobTok, nil, &b)
		if len(a) != 1 || len(b) != 0 {
			t.Fatalf("alice sees %d links, bob sees %d", len(a), len(b))
		}
		st, _ := call(t, "GET", api+"/company/"+acme.ID, bobTok, nil, nil)
		wantStatus(t, st, 404, "bob reads the linked company")
		got, _ := mcpCall(t, srv.url, bobTok, "get", map[string]any{"type": "person", "id": fam.ID})
		if !strings.Contains(got, `"links":[]`) {
			t.Errorf("mcp get for bob: %s", got)
		}
	})

	t.Run("SecretIsMaskedAndUnreadableWithoutGrant", func(t *testing.T) {
		in := in.on(t)
		ssn := func(tok string) string {
			var r e2eRecord
			call(t, "GET", api+"/person/"+fam.ID+"?reveal=true", tok, nil, &r)
			return fmt.Sprint(r.Data["ssn"])
		}
		var plain e2eRecord
		call(t, "GET", api+"/person/"+fam.ID, aliceTok, nil, &plain)
		if m, _ := plain.Data["ssn"].(map[string]any); m["masked"] != "***-**-6789" {
			t.Fatalf("default ssn = %v", plain.Data["ssn"])
		}
		if s := ssn(aliceTok); strings.Contains(s, "123456789") {
			t.Fatalf("alice reveals without grant: %s", s)
		}
		if n := in.psql("archive", "SELECT count(*) FROM secrets WHERE position(convert_to('123456789', 'UTF8') in value) > 0"); n != "0" {
			t.Fatalf("secret stored in plain text")
		}
		if n := in.psql("archive", "SELECT count(*) FROM records WHERE data::text LIKE '%123456789%' OR id IN (SELECT record_id FROM provenance WHERE original LIKE '%6789%')"); n != "0" {
			t.Fatalf("secret leaked into records or provenance")
		}
		in.fails("-as", "bob@example.com", "secret", "grant", "bob@example.com", "us_ssn")
		in.must("secret", "grant", "alice@example.com", "us_ssn")
		if s := in.must("get", "person", fam.ID, "-reveal"); !strings.Contains(s, `"ssn": "123456789"`) {
			t.Errorf("cli reveal: %s", s)
		}
		if got, _ := mcpCall(t, srv.url, aliceTok, "get", map[string]any{"type": "person", "id": fam.ID, "reveal": true}); strings.Contains(got, "123456789") {
			t.Errorf("token reveals although tokens get no secret grants: %s", got)
		}
		if s := ssn(bobTok); strings.Contains(s, "123456789") {
			t.Fatalf("bob reveals: %s", s)
		}
		call(t, "PATCH", api+"/person/"+fam.ID, aliceTok, map[string]any{"ssn": "078-05-1120"}, nil)
		if n := in.psql("archive", "SELECT count(*) FROM secrets WHERE record_id = '"+fam.ID+"'"); n != "1" {
			t.Errorf("old secret not pruned: %s rows", n)
		}
		st, _ := call(t, "PATCH", api+"/person/"+fam.ID, aliceTok, map[string]any{"ssn": map[string]any{"$secret": "01a166cb-2d4c-7eff-9923-6316a92882b7"}}, nil)
		wantStatus(t, st, 422, "forged secret reference")
	})

	t.Run("OntologyChangesNeedAdminAndAreAttributed", func(t *testing.T) {
		ops := []Op{{Op: "add_field", Type: "company", Field: "vat", Def: map[string]any{"type": "vat_id"}}}
		var c Change
		st, _ := call(t, "POST", api+"/changes", narrowTok, map[string]any{"ops": ops}, &c)
		wantStatus(t, st, 201, "viewer token proposes")
		if c.ProposedBy != "Narrow, on behalf of alice@example.com" {
			t.Errorf("proposed_by = %q", c.ProposedBy)
		}
		st, _ = call(t, "POST", api+"/changes/"+c.ID.String()+"/apply", narrowTok, nil, nil)
		wantStatus(t, st, 403, "viewer token applies")
		got, isErr := mcpCall(t, srv.url, aliceTok, "apply", map[string]any{"change_id": c.ID.String()})
		if isErr || !strings.Contains(got, `"applied"`) {
			t.Fatalf("admin applies over mcp: %s", got)
		}
		got, _ = mcpCall(t, srv.url, aliceTok, "propose", map[string]any{"ops": []Op{{Op: "remove_field", Type: "company", Field: "vat"}}})
		if !strings.Contains(got, "Claude, on behalf of alice@example.com") {
			t.Errorf("mcp proposal: %s", got)
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
		var rs []e2eRecord
		call(t, "GET", api+"/person", "", nil, &rs, session)
		if names(rs) != "Bob private,Family member" {
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
		st, _ = call(t, "GET", api+"/person", bobNarrow.Secret, nil, nil)
		wantStatus(t, st, 403, "token whose user left its only space")
		in.fails("space", "grant", "family", "alice@example.com", "editor")
		in.must("-as", "bob@example.com", "token", "revoke", bobNarrow.ID.String())
		st, _ = call(t, "GET", api+"/person", bobNarrow.Secret, nil, nil)
		wantStatus(t, st, 401, "revoked token")
	})
}

func TestFirstRunSetupLinkAndUpgradeIntoADefaultSpace(t *testing.T) {
	in := newInst(t)
	srv := in.serve()
	m := regexp.MustCompile(`/auth/setup\?token=(\S+)`).FindStringSubmatch(srv.log.String())
	if m == nil {
		t.Fatalf("no setup link in log:\n%s", srv.log)
	}
	in.psql("archive", `
		ALTER TABLE records ALTER COLUMN space DROP NOT NULL;
		ALTER TABLE raw ALTER COLUMN space DROP NOT NULL;
		ALTER TABLE provenance ALTER COLUMN space DROP NOT NULL;
		ALTER TABLE links ALTER COLUMN space DROP NOT NULL;
		INSERT INTO types (name, extends, def, updated_at) VALUES ('legacy', '', '{"fields": {"name": {"type": "text"}, "ssn": {"type": "us_ssn"}}}', now());
		INSERT INTO records (type, spec_version, data) VALUES ('legacy', 0, '{"name": "Old", "ssn": "123456789"}')`)
	if errs := in.fails("list", "legacy"); !strings.Contains(errs, "no users yet") {
		t.Fatalf("list before setup: %s", errs)
	}
	st, _ := call(t, "POST", srv.url+"/auth/setup?token="+m[1], "", map[string]any{"email": "first@example.com", "name": "First"}, nil)
	wantStatus(t, st, 200, "setup link creates the first owner")
	st, _ = call(t, "POST", srv.url+"/auth/setup?token="+m[1], "", map[string]any{"email": "second@example.com"}, nil)
	wantStatus(t, st, 401, "setup link used twice")

	rs := decode[[]e2eRecord](t, in.must("list", "legacy"))
	if len(rs) != 1 || rs[0].Data["name"] != "Old" {
		t.Fatalf("legacy records: %+v", rs)
	}
	if ssn, _ := rs[0].Data["ssn"].(map[string]any); ssn["masked"] != "***-**-6789" {
		t.Errorf("legacy ssn not moved into secrets: %v", rs[0].Data["ssn"])
	}
	spaces := decode[[]SpaceInfo](t, in.must("space", "list"))
	if len(spaces) != 2 || spaces[0].Name != "default" || spaces[0].Level != "owner" || spaces[0].ID.String() != rs[0].Space {
		t.Errorf("spaces: %+v", spaces)
	}
	if n := in.psql("archive", "SELECT count(*) FROM information_schema.columns WHERE column_name = 'space' AND is_nullable = 'YES'"); n != "0" {
		t.Errorf("%s space columns still nullable", n)
	}
}

func TestDockerImageServesAndAcceptsCLICommandsFromDockerExec(t *testing.T) {
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
	docker(e2eSpec, "exec", "-i", name, "archive", "spec", "push", "-")
	docker("", "exec", name, "archive", "create", "company", `{"name":"Docker Inc"}`)
	if out := docker("", "exec", name, "archive", "list", "company"); !strings.Contains(out, "Docker Inc") {
		t.Fatalf("docker exec list: %s", out)
	}
	if logs := docker("", "logs", name); strings.Contains(logs, "already running") {
		t.Errorf("logs: %s", logs)
	}
}

type e2eGoogle struct {
	mu        sync.Mutex
	history   int
	added     map[int]string
	subjects  map[string]string
	fail      int
	listCalls int
	fetched   []string
	starts    []string
	newFile   bool
}

func (g *e2eGoogle) deliver(id, subject string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.history++
	g.added[g.history] = id
	g.subjects[id] = subject
}

func (g *e2eGoogle) handler(t *testing.T) http.Handler {
	b64 := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	file := func(id, name, mime string) map[string]any {
		return map[string]any{"id": id, "name": name, "mimeType": mime, "modifiedTime": "2026-10-01T10:00:00Z"}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		id, secret, _ := r.BasicAuth()
		id, secret = cmp.Or(id, r.Form.Get("client_id")), cmp.Or(secret, r.Form.Get("client_secret"))
		ok := id == "cid" && secret == "csecret"
		switch r.Form.Get("grant_type") {
		case "authorization_code":
			ok = ok && r.Form.Get("code") == "good-code" && r.Form.Get("code_verifier") != ""
		case "refresh_token":
			ok = ok && r.Form.Get("refresh_token") == "rt-1"
		}
		w.Header().Set("Content-Type", "application/json")
		if !ok {
			w.WriteHeader(400)
			w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		w.Write([]byte(`{"access_token":"at-1","refresh_token":"rt-1","token_type":"Bearer","expires_in":3600}`))
	})
	api := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer at-1" {
			w.WriteHeader(401)
			return
		}
		g.mu.Lock()
		defer g.mu.Unlock()
		api.ServeHTTP(w, r)
	}))
	api.HandleFunc("GET /gmail/v1/users/me/profile", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"historyId": strconv.Itoa(g.history)})
	})
	api.HandleFunc("GET /gmail/v1/users/me/messages", func(w http.ResponseWriter, r *http.Request) {
		g.listCalls++
		if g.fail > 0 {
			g.fail--
			w.WriteHeader(500)
			return
		}
		var ms []map[string]string
		for h := g.history; h > 0; h-- {
			if id, ok := g.added[h]; ok {
				ms = append(ms, map[string]string{"id": id})
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"messages": ms})
	})
	api.HandleFunc("GET /gmail/v1/users/me/history", func(w http.ResponseWriter, r *http.Request) {
		start, _ := strconv.Atoi(r.URL.Query().Get("startHistoryId"))
		g.starts = append(g.starts, r.URL.Query().Get("startHistoryId"))
		var hs []any
		for h := start + 1; h <= g.history; h++ {
			if id, ok := g.added[h]; ok {
				hs = append(hs, map[string]any{"messagesAdded": []any{map[string]any{"message": map[string]string{"id": id}}}})
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"history": hs, "historyId": strconv.Itoa(g.history)})
	})
	api.HandleFunc("GET /gmail/v1/users/me/messages/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		g.fetched = append(g.fetched, id)
		json.NewEncoder(w).Encode(map[string]any{"id": id, "threadId": "t-" + id, "labelIds": []string{"INBOX"}, "snippet": "snip",
			"payload": map[string]any{"mimeType": "multipart/mixed",
				"headers": []any{map[string]string{"name": "From", "value": "Ann <ann@example.com>"}, map[string]string{"name": "Subject", "value": g.subjects[id]}},
				"parts": []any{
					map[string]any{"mimeType": "multipart/alternative", "parts": []any{
						map[string]any{"mimeType": "text/plain", "body": map[string]any{"data": b64("Body of " + g.subjects[id])}},
						map[string]any{"mimeType": "text/html", "body": map[string]any{"data": b64("<p>html</p>")}}}},
					map[string]any{"mimeType": "application/pdf", "filename": "invoice.pdf", "body": map[string]any{"attachmentId": "att-" + id, "size": 1234}}}}})
	})
	api.HandleFunc("GET /gmail/v1/users/me/messages/{id}/attachments/{aid}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("aid") != "att-"+r.PathValue("id") {
			w.WriteHeader(404)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"size": 1234, "data": base64.URLEncoding.EncodeToString(e2ePDF("Attachment of " + r.PathValue("id")))})
	})
	api.HandleFunc("GET /drive/v3/changes/startPageToken", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"startPageToken":"1"}`))
	})
	api.HandleFunc("GET /drive/v3/files", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"files": []any{
			file("d1", "Plan", "application/vnd.google-apps.document"), file("d2", "notes.txt", "text/plain"), file("d3", "photo.jpg", "image/jpeg")}})
	})
	api.HandleFunc("GET /drive/v3/changes", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("pageToken") == "1" && g.newFile {
			json.NewEncoder(w).Encode(map[string]any{"changes": []any{map[string]any{"file": file("d4", "new.txt", "text/plain")}}, "newStartPageToken": "2"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"changes": []any{}, "newStartPageToken": r.URL.Query().Get("pageToken")})
	})
	api.HandleFunc("GET /drive/v3/files/d1/export", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("Doc text from Drive")) })
	api.HandleFunc("GET /drive/v3/files/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Contents of " + r.PathValue("id")))
	})
	return mux
}

const e2eAirbyte = `#!/bin/sh
cmd=$1; shift
config=; state=
while [ $# -gt 0 ]; do
  case $1 in --config) config=$2; shift;; --state) state=$2; shift;; --catalog) catalog=$2; shift;; esac
  shift
done
case $cmd in
spec) echo '{"type":"SPEC","spec":{"connectionSpecification":{"required":["api_key"]}}}';;
check)
  if grep -q '"api_key":"s3cret"' "$config"; then echo '{"type":"CONNECTION_STATUS","connectionStatus":{"status":"SUCCEEDED"}}'
  else echo '{"type":"CONNECTION_STATUS","connectionStatus":{"status":"FAILED","message":"bad key"}}'; fi;;
discover) echo '{"type":"CATALOG","catalog":{"streams":[{"name":"notes","json_schema":{},"supported_sync_modes":["full_refresh","incremental"],"source_defined_cursor":true,"source_defined_primary_key":[["id"]]}]}}';;
read)
  grep -q '"sync_mode":"incremental"' "$catalog" || exit 3
  [ -f DIR/slow ] && sleep 3
  last=0
  [ -n "$state" ] && last=$(sed -n 's/.*"cursor":\([0-9]*\).*/\1/p' "$state")
  n=$(cat DIR/count)
  echo 'not json, ignored'
  echo '{"type":"LOG","log":{"level":"INFO","message":"reading"}}'
  i=$((last+1))
  while [ $i -le $n ]; do
    echo "{\"type\":\"RECORD\",\"record\":{\"stream\":\"notes\",\"data\":{\"id\":$i,\"body\":\"Note $i\"},\"emitted_at\":0}}"
    i=$((i+1))
  done
  echo "{\"type\":\"STATE\",\"state\":{\"type\":\"STREAM\",\"stream\":{\"stream_descriptor\":{\"name\":\"notes\"},\"stream_state\":{\"cursor\":$n}}}}";;
esac
`

const e2eSinger = `#!/bin/sh
while [ $# -gt 0 ]; do [ "$1" = --state ] && exit 0; shift; done
echo '{"type":"SCHEMA","stream":"tasks","schema":{},"key_properties":["id"]}'
echo '{"type":"RECORD","stream":"tasks","record":{"id":"t1","title":"Buy milk"}}'
echo '{"type":"RECORD","stream":"tasks","record":{"id":"t2","title":"Call mom"}}'
echo '{"type":"STATE","value":{"done":true}}'
`

func TestSourcesSyncGmailDriveIMAPAndExternalConnectorsIntoRawAndTypes(t *testing.T) {
	g := &e2eGoogle{history: 99, added: map[int]string{}, subjects: map[string]string{}, fail: 1}
	g.deliver("m1", "Hello one")
	g.deliver("m2", "Hello two")
	google := httptest.NewServer(g.handler(t))
	defer google.Close()
	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		text := req.Messages[len(req.Messages)-1].Content
		subject := regexp.MustCompile(`Subject: (.*)`).FindStringSubmatch(text)
		from := regexp.MustCompile(`[\w.+-]+@[\w.-]+`).FindString(text)
		if subject == nil {
			subject = []string{"", ""}
		}
		content, _ := json.Marshal(map[string]string{"subject": subject[1], "from": from})
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": string(content)}}}})
	}))
	defer llm.Close()

	mem := imapmemserver.New()
	mailbox := imapmemserver.NewUser("bob@icloud.test", "app-pass")
	mailbox.Create("INBOX", nil)
	mem.AddUser(mailbox)
	deliverIMAP := func(subject string) {
		msg := "From: Carol <carol@example.com>\r\nTo: bob@icloud.test\r\nSubject: " + subject + "\r\nDate: Thu, 22 Oct 2026 10:00:00 +0000\r\n" +
			"MIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=b\r\n\r\n--b\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nHi from " + subject +
			"\r\n--b\r\nContent-Type: text/csv\r\nContent-Disposition: attachment; filename=data.csv\r\n\r\na,b\r\n--b--\r\n"
		if _, err := mailbox.Append("INBOX", bytes.NewReader([]byte(msg)), &imap.AppendOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	deliverIMAP("Mail one")
	deliverIMAP("Mail two")
	imapSrv := imapserver.New(&imapserver.Options{
		NewSession: func(*imapserver.Conn) (imapserver.Session, *imapserver.GreetingData, error) {
			return mem.NewSession(), nil, nil
		},
		Caps:         imap.CapSet{imap.CapIMAP4rev1: {}},
		InsecureAuth: true,
	})
	imapLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go imapSrv.Serve(imapLn)
	defer imapSrv.Close()

	in := newInst(t, "ARCHIVE_ADMIN_EMAIL=alice@example.com", "ARCHIVE_GOOGLE_CLIENT_ID=cid", "ARCHIVE_GOOGLE_CLIENT_SECRET=csecret",
		"ARCHIVE_GOOGLE_API_URL="+google.URL, "ARCHIVE_GOOGLE_TOKEN_URL="+google.URL+"/token", "ARCHIVE_LLM_URL="+llm.URL+"/v1", "ARCHIVE_LLM_MODEL=fake")
	srv := in.serve()
	api := srv.url + "/api"
	in.must("user", "add", "bob@example.com")
	specPath := filepath.Join(in.dir, "message.json")
	os.WriteFile(specPath, []byte(`{"types": {"message": {"fields": {"subject": {"type": "text", "required": true}, "from": {"type": "email"}}}}}`), 0o644)
	in.must("spec", "push", specPath)
	aliceTok := decode[Token](t, in.must("token", "create", "-name", "Claude")).Secret
	bobTok := decode[Token](t, in.must("-as", "bob@example.com", "token", "create")).Secret
	aliceHome := in.psql("archive", "SELECT home FROM users WHERE email = 'alice@example.com'")
	rawCount := func(src Source) string {
		return in.psql("archive", "SELECT count(*) FROM raw WHERE source = '"+src.Kind+":"+src.ID.String()+"' AND space = '"+aliceHome+"'")
	}
	runCLI := func(t *testing.T, name string) SyncResult {
		t.Helper()
		return decode[SyncResult](t, in.on(t).must("source", "run", name))
	}

	var gmail Source
	t.Run("GmailOAuthInTheCLIThenFullSyncRetriesA500AndIntakeMapsMail", func(t *testing.T) {
		out, errs, err := in.run("good-code\n", "source", "add", "gmail", "-name", "work", "-type", "message")
		if err != nil {
			t.Fatalf("source add gmail: %v\n%s%s", err, out, errs)
		}
		gmail = decode[Source](t, out)
		m := regexp.MustCompile(`https://accounts\.google\.com/o/oauth2/auth\?\S+`).FindString(errs)
		u, _ := url.Parse(m)
		q := u.Query()
		if gmail.Status != "new" || q.Get("client_id") != "cid" || q.Get("access_type") != "offline" || q.Get("code_challenge") == "" ||
			q.Get("redirect_uri") != "http://localhost:8080/api/sources/oauth/callback" || !strings.HasPrefix(q.Get("state"), gmail.ID.String()+".") ||
			!strings.Contains(q.Get("scope"), "gmail.readonly") {
			t.Fatalf("source %+v, auth url %q", gmail, m)
		}
		res := runCLI(t, "work")
		if res.Fetched != 2 || res.New != 2 || res.Mapped != 2 || res.Status != "ok" || g.listCalls != 2 {
			t.Fatalf("first sync %+v, list calls %d", res, g.listCalls)
		}
		if n := rawCount(gmail); n != "2" {
			t.Fatalf("%s raw rows in alice's space", n)
		}
		row := in.psql("archive", "SELECT payload->'attachments'->0->>'filename', payload->>'text' FROM raw WHERE external_id = 'm1'")
		if !strings.HasPrefix(row, "invoice.pdf|From: Ann <ann@example.com>\nSubject: Hello one\n\nBody of Hello one") {
			t.Errorf("payload: %q", row)
		}
		fid := in.psql("archive", "SELECT payload->'attachments'->0->>'file_id' FROM raw WHERE external_id = 'm1'")
		f := e2eWaitFile(t, api, aliceTok, fid)
		if f.Name != "invoice.pdf" || f.MIME != "application/pdf" || f.Status != "done" || f.Space.String() != aliceHome || f.Metadata["pages"] != 1.0 {
			t.Errorf("attachment file: %+v", f)
		}
		if text := in.psql("archive", "SELECT text FROM files WHERE id = '"+fid+"'"); !strings.Contains(text, "Attachment of m1") {
			t.Errorf("attachment text: %q", text)
		}
		rs := decode[[]e2eRecord](t, in.must("list", "message"))
		if len(rs) != 2 || rs[0].Data["subject"] != "Hello one" || rs[1].Data["from"] != "ann@example.com" {
			t.Errorf("mapped messages: %+v", rs)
		}
	})

	t.Run("GmailIncrementalSyncFetchesOnlyNewMail", func(t *testing.T) {
		g.deliver("m3", "Hello three")
		res := runCLI(t, "work")
		if res.Fetched != 1 || res.New != 1 || res.Mapped != 1 || strings.Join(g.fetched, ",") != "m1,m2,m3" || strings.Join(g.starts, ",") != "101" {
			t.Fatalf("second sync %+v, fetched %v, history starts %v", res, g.fetched, g.starts)
		}
	})

	t.Run("CursorPersistsAcrossARestart", func(t *testing.T) {
		srv.stop()
		srv = in.serve()
		api = srv.url + "/api"
		var s Source
		call(t, "GET", api+"/sources/work", aliceTok, nil, &s)
		if !strings.Contains(string(s.State), `"history_id":"102"`) || s.LastRunAt == nil {
			t.Fatalf("state after restart: %s", s.State)
		}
		var res SyncResult
		st, _ := call(t, "POST", api+"/sources/work/run", aliceTok, nil, &res)
		wantStatus(t, st, 200, "run over the API")
		if res.Fetched != 0 || g.listCalls != 2 || strings.Join(g.starts, ",") != "101,102" {
			t.Fatalf("after restart %+v, list calls %d, starts %v", res, g.listCalls, g.starts)
		}
	})

	t.Run("DriveOAuthCallbackThenScheduledIncrementalSync", func(t *testing.T) {
		var created struct {
			Source
			AuthURL string `json:"auth_url"`
		}
		st, _ := call(t, "POST", api+"/sources", aliceTok, map[string]any{"kind": "drive", "name": "docs", "schedule": "1s"}, &created)
		wantStatus(t, st, 201, "add drive")
		u, _ := url.Parse(created.AuthURL)
		redirect := u.Query().Get("redirect_uri")
		if created.Status != "needs_auth" || !strings.HasSuffix(redirect, "/api/sources/oauth/callback") {
			t.Fatalf("drive %+v", created)
		}
		cb := srv.url + "/api/sources/oauth/callback?code=good-code&state="
		st, _ = call(t, "GET", cb+"forged", "", nil, nil)
		wantStatus(t, st, 401, "callback with a forged state")
		st, _ = call(t, "GET", cb+created.ID.String()+".forged", "", nil, nil)
		wantStatus(t, st, 401, "callback with the right source and a forged secret")
		st, _ = call(t, "GET", cb+url.QueryEscape(u.Query().Get("state")), "", nil, nil)
		wantStatus(t, st, 200, "callback")
		waitRaw := func(want string) {
			for deadline := time.Now().Add(20 * time.Second); rawCount(created.Source) != want; time.Sleep(200 * time.Millisecond) {
				if time.Now().After(deadline) {
					t.Fatalf("drive raw rows %s, want %s\n%s", rawCount(created.Source), want, srv.log)
				}
			}
		}
		waitRaw("3")
		texts := in.psql("archive", "SELECT string_agg(external_id || '=' || (payload->>'text'), ';' ORDER BY external_id) FROM raw WHERE source = 'drive:"+created.ID.String()+"'")
		if texts != "d1=Doc text from Drive;d2=Contents of d2;d3=" {
			t.Errorf("drive texts: %q", texts)
		}
		if f := in.psql("archive", "SELECT f.name || '|' || f.mime FROM raw r JOIN files f ON f.id = (r.payload->>'file_id')::uuid WHERE r.external_id = 'd3'"); f != "photo.jpg|image/jpeg" {
			t.Errorf("drive binary file: %q", f)
		}
		g.mu.Lock()
		g.newFile = true
		g.mu.Unlock()
		waitRaw("4")
		var s Source
		call(t, "GET", api+"/sources/docs", aliceTok, nil, &s)
		if !strings.Contains(string(s.State), `"page_token":"2"`) || s.Status == "error" {
			t.Errorf("drive source: %+v %s", s, s.State)
		}
	})

	t.Run("IMAPSyncsByUIDOverMCPAndCLI", func(t *testing.T) {
		in := in.on(t)
		cfg := `{"config":{"host":"` + imapLn.Addr().String() + `","username":"bob@icloud.test","plaintext":true},"credentials":{"password":"app-pass"}}`
		in.must("source", "add", "imap", "-name", "icloud", cfg)
		got, isErr := mcpCall(t, srv.url, aliceTok, "sync", map[string]any{"source": "icloud"})
		if isErr || !strings.Contains(got, `"new":2`) {
			t.Fatalf("mcp sync: %s", got)
		}
		deliverIMAP("Mail three")
		if res := runCLI(t, "icloud"); res.Fetched != 1 || res.New != 1 {
			t.Fatalf("imap incremental: %+v", res)
		}
		row := in.psql("archive", "SELECT payload->'headers'->>'Subject', payload->'attachments'->0->>'filename', payload->>'text' FROM raw WHERE external_id LIKE 'INBOX:%:3'")
		if !strings.HasPrefix(row, "Mail three|data.csv|From: Carol <carol@example.com>") || !strings.Contains(row, "Hi from Mail three") {
			t.Errorf("imap payload: %q", row)
		}
		if n := in.psql("archive", "SELECT count(*) FROM secrets WHERE position(convert_to('app-pass', 'UTF8') in value) > 0 OR position(convert_to('rt-1', 'UTF8') in value) > 0"); n != "0" {
			t.Fatalf("credentials stored in plain text")
		}
		if n := in.psql("archive", "SELECT count(*) FROM sources WHERE config::text LIKE '%app-pass%' OR state::text LIKE '%app-pass%'"); n != "0" {
			t.Fatalf("password in source config")
		}
		in.must("source", "add", "imap", "-name", "wrong", strings.Replace(cfg, "app-pass", "nope", 1))
		if errs := in.fails("source", "run", "wrong"); !strings.Contains(errs, "imap login") {
			t.Errorf("wrong password: %s", errs)
		}
		var s Source
		call(t, "GET", api+"/sources/wrong", aliceTok, nil, &s)
		if s.Status != "error" || s.Failures != 1 || !strings.Contains(s.Error, "imap login") {
			t.Errorf("failed source: %+v", s)
		}
	})

	t.Run("ExternalAirbyteAndSingerConnectorsKeepTheirState", func(t *testing.T) {
		in := in.on(t)
		script := filepath.Join(in.dir, "airbyte.sh")
		os.WriteFile(script, []byte(strings.ReplaceAll(e2eAirbyte, "DIR", in.dir)), 0o755)
		os.WriteFile(filepath.Join(in.dir, "count"), []byte("2"), 0o644)
		body := func(key string) string {
			return `{"config":{"command":["` + script + `"],"config":{"region":"eu"}},"credentials":{"api_key":"` + key + `"}}`
		}
		if errs := in.fails("source", "add", "external", "-name", "notes", body("wrong")); !strings.Contains(errs, "connector check failed: bad key") {
			t.Errorf("check with a bad key: %s", errs)
		}
		if errs := in.fails("source", "add", "external", "-name", "notes", `{"config":{"command":["`+script+`"]}}`); !strings.Contains(errs, "config.api_key: required by the connector") {
			t.Errorf("spec required keys: %s", errs)
		}
		notes := decode[Source](t, in.must("source", "add", "external", "-name", "notes", body("s3cret")))
		if res := runCLI(t, "notes"); res.New != 2 {
			t.Fatalf("first read: %+v", res)
		}
		if res := runCLI(t, "notes"); res.Fetched != 0 {
			t.Fatalf("read with state: %+v", res)
		}
		os.WriteFile(filepath.Join(in.dir, "count"), []byte("3"), 0o644)
		if res := runCLI(t, "notes"); res.Fetched != 1 || res.New != 1 {
			t.Fatalf("incremental read: %+v", res)
		}
		if ids := in.psql("archive", "SELECT string_agg(external_id, ',' ORDER BY external_id) FROM raw WHERE source = 'external:"+notes.ID.String()+"'"); ids != "notes:1,notes:2,notes:3" {
			t.Errorf("external ids: %s", ids)
		}

		os.WriteFile(filepath.Join(in.dir, "slow"), nil, 0o644)
		done := make(chan SyncResult)
		go func() {
			var res SyncResult
			call(t, "POST", api+"/sources/notes/run", aliceTok, nil, &res)
			done <- res
		}()
		for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(50 * time.Millisecond) {
			var s Source
			call(t, "GET", api+"/sources/notes", aliceTok, nil, &s)
			if s.Status == "running" {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("slow sync never started")
			}
		}
		if errs := in.fails("source", "run", "notes"); !strings.Contains(errs, "already syncing") {
			t.Errorf("second run while the first holds the lock: %s", errs)
		}
		if res := <-done; res.Status != "ok" {
			t.Errorf("slow run: %+v", res)
		}
		os.Remove(filepath.Join(in.dir, "slow"))

		singer := filepath.Join(in.dir, "singer.sh")
		os.WriteFile(singer, []byte(e2eSinger), 0o755)
		in.must("source", "add", "external", "-name", "tasks", `{"config":{"command":["`+singer+`"],"protocol":"singer"}}`)
		if res := runCLI(t, "tasks"); res.New != 2 {
			t.Fatalf("singer: %+v", res)
		}
		if res := runCLI(t, "tasks"); res.Fetched != 0 {
			t.Fatalf("singer with state: %+v", res)
		}

		in.must("source", "remove", "notes")
		if n := in.psql("archive", "SELECT count(*) FROM secrets WHERE source_id = '"+notes.ID.String()+"'"); n != "0" {
			t.Errorf("%s secrets left after remove", n)
		}
		if n := in.psql("archive", "SELECT count(*) FROM raw WHERE source = 'external:"+notes.ID.String()+"'"); n != "3" {
			t.Errorf("raw after remove: %s", n)
		}
	})

	t.Run("FolderIsAConnectorKind", func(t *testing.T) {
		in := in.on(t)
		inbox := filepath.Join(in.dir, "inbox")
		os.MkdirAll(inbox, 0o755)
		os.WriteFile(filepath.Join(inbox, "mail.txt"), []byte("From: Dave <dave@example.com>\nSubject: From a folder"), 0o644)
		in.must("source", "add", "folder", "-name", "inbox", "-type", "message", `{"config":{"path":"`+inbox+`"}}`)
		if res := runCLI(t, "inbox"); res.New != 1 || res.Mapped != 1 {
			t.Fatalf("folder: %+v", res)
		}
		if n := in.psql("archive", "SELECT count(*) FROM raw WHERE source = 'folder:"+inbox+"'"); n != "1" {
			t.Errorf("folder raw: %s", n)
		}
	})

	t.Run("AnotherUserCannotSeeTheSourcesOrTheirData", func(t *testing.T) {
		in := in.on(t)
		if out := strings.TrimSpace(in.must("-as", "bob@example.com", "source", "list")); out != "[]" {
			t.Errorf("bob lists %s", out)
		}
		var list []Source
		call(t, "GET", api+"/sources", bobTok, nil, &list)
		if len(list) != 0 {
			t.Errorf("bob sees %d sources over the API", len(list))
		}
		for _, c := range []struct{ method, path string }{{"GET", "/sources/work"}, {"POST", "/sources/work/run"}, {"DELETE", "/sources/work"}} {
			st, _ := call(t, c.method, api+c.path, bobTok, nil, nil)
			wantStatus(t, st, 404, "bob "+c.method+" "+c.path)
		}
		if got, _ := mcpCall(t, srv.url, bobTok, "sources", map[string]any{}); strings.Contains(got, "work") {
			t.Errorf("bob's mcp sources: %s", got)
		}
		if got, _ := mcpCall(t, srv.url, aliceTok, "sources", map[string]any{}); !strings.Contains(got, `"name":"work"`) || !strings.Contains(got, `"name":"docs"`) {
			t.Errorf("alice's mcp sources: %s", got)
		}
		role := in.psql("archive", "SELECT role FROM users WHERE email = 'bob@example.com'")
		in.psql("archive", "ALTER ROLE "+role+" LOGIN")
		if got := in.psql(role, "SELECT (SELECT count(*) FROM raw) || ',' || (SELECT count(*) FROM sources) || ',' || (SELECT count(*) FROM secrets) || ',' || (SELECT count(*) FROM files)"); got != "0,0,0,0" {
			t.Errorf("psql as bob sees raw, sources, secrets, files: %s", got)
		}
		in.psql("archive", "ALTER ROLE "+role+" NOLOGIN")
		alice := in.psql("archive", "SELECT role FROM users WHERE email = 'alice@example.com'")
		in.psql("archive", "ALTER ROLE "+alice+" LOGIN")
		if got := in.psql(alice, "SELECT (SELECT count(*) FROM sources) || ',' || (SELECT count(*) FROM secrets)"); got != "6,0" {
			t.Errorf("psql as alice sees sources, secrets: %s", got)
		}
		in.psql("archive", "ALTER ROLE "+alice+" NOLOGIN")
	})
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

func e2eWaitFile(t *testing.T, api, tok, id string) File {
	t.Helper()
	for deadline := time.Now().Add(30 * time.Second); ; time.Sleep(100 * time.Millisecond) {
		var f File
		call(t, "GET", api+"/files/"+id, tok, nil, &f)
		if f.Status == "done" || f.Status == "error" {
			return f
		}
		if time.Now().After(deadline) {
			t.Fatalf("file %s still %q", id, f.Status)
		}
	}
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

type e2eOpenAI struct {
	mu      sync.Mutex
	images  []string
	audio   []string
	models  []string
	prompts []string
}

func (o *e2eOpenAI) handler() http.Handler {
	mux := http.NewServeMux()
	reply := func(w http.ResponseWriter, content string) {
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": content}}}})
	}
	mux.HandleFunc("POST /v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model    string `json:"model"`
			Messages []struct {
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		o.mu.Lock()
		defer o.mu.Unlock()
		o.models = append(o.models, req.Model)
		last := req.Messages[len(req.Messages)-1].Content
		var parts []struct {
			ImageURL struct {
				URL string `json:"url"`
			} `json:"image_url"`
		}
		if json.Unmarshal(last, &parts) == nil {
			for _, p := range parts {
				if p.ImageURL.URL != "" {
					o.images = append(o.images, p.ImageURL.URL)
				}
			}
			reply(w, "Receipt R-42\nTotal 12.50")
			return
		}
		var text string
		json.Unmarshal(last, &text)
		o.prompts = append(o.prompts, text)
		content, _ := json.Marshal(map[string]string{"number": regexp.MustCompile(`INV-\d+`).FindString(text)})
		reply(w, string(content))
	})
	mux.HandleFunc("POST /v1/audio/transcriptions", func(w http.ResponseWriter, r *http.Request) {
		f, h, err := r.FormFile("file")
		if err != nil {
			w.WriteHeader(400)
			return
		}
		b, _ := io.ReadAll(f)
		o.mu.Lock()
		o.audio = append(o.audio, r.FormValue("model")+":"+h.Filename+":"+strconv.Itoa(len(b)))
		o.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]string{"text": "Hello from the voice memo"})
	})
	return mux
}

func TestFilesUploadProcessShareAndIntake(t *testing.T) {
	ai := &e2eOpenAI{}
	llm := httptest.NewServer(ai.handler())
	defer llm.Close()
	in := newInst(t, "ARCHIVE_ADMIN_EMAIL=alice@example.com", "ARCHIVE_LLM_URL="+llm.URL+"/v1", "ARCHIVE_LLM_MODEL=fake",
		"ARCHIVE_VISION_MODEL=vision-fake", "ARCHIVE_WHISPER_MODEL=whisper-fake")
	srv := in.serve()
	api := srv.url + "/api"
	in.must("user", "add", "bob@example.com")
	in.must("spec", "push", in.writeSpec())
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
		bigFile = e2eWaitFile(t, api, aliceTok, resp.Header.Get("Archive-File-Id"))
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
		if n := in.psql("archive", "SELECT count(*) FROM files WHERE hash = '"+bigFile.Hash+"'"); n != "3" || len(blobs) != 1 {
			t.Errorf("%s file rows, %d blobs", n, len(blobs))
		}
		if leftovers, _ := os.ReadDir(filepath.Join(in.dir, "uploads")); len(leftovers) != 0 {
			t.Errorf("uploads left behind: %d", len(leftovers))
		}
	})

	photo := e2eJPEG(t)
	var photoFile File
	t.Run("PhotoMetadataThumbnailAndLocationMarkedPersonal", func(t *testing.T) {
		st, f := e2eUpload(t, api+"/files", aliceTok, "photo.jpg", photo)
		wantStatus(t, st, 201, "upload photo")
		photoFile = e2eWaitFile(t, api, aliceTok, f.ID.String())
		m := photoFile.Metadata
		loc, _ := m["personal"].(map[string]any)["location"].(map[string]any)
		if photoFile.MIME != "image/jpeg" || m["width"] != 640.0 || m["height"] != 480.0 || m["camera"] != "Canon EOS" ||
			m["taken_at"] != "2026-10-01T12:30:00" || loc["lat"] != 52.3720333 || loc["lon"] != 4.8926 {
			t.Fatalf("photo: %+v", photoFile)
		}
		resp, body := e2eGet(t, api+"/files/"+f.ID.String()+"/thumbnail", aliceTok)
		c, _, err := image.DecodeConfig(bytes.NewReader(body))
		if resp.StatusCode != 200 || err != nil || c.Width != 320 || c.Height != 240 || bytes.Contains(body, e2eLatitude) {
			t.Errorf("thumbnail: %d %v %+v", resp.StatusCode, err, c)
		}
		if got, _ := mcpCall(t, srv.url, aliceTok, "file", map[string]any{"id": f.ID.String()}); !strings.Contains(got, "Canon EOS") {
			t.Errorf("mcp file: %s", got)
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
		st, _ := call(t, "POST", api+"/files/"+photoFile.ID.String()+"/link?expires=2s", aliceTok, nil, &link)
		wantStatus(t, st, 200, "create link")
		resp, shared := e2eGet(t, link.URL, "")
		if resp.StatusCode != 200 || bytes.Contains(shared, e2eLatitude) || !bytes.Equal(shared, stripped) {
			t.Fatalf("signed link: %d, %d bytes", resp.StatusCode, len(shared))
		}
		resp, _ = e2eGet(t, link.URL+"0", "")
		wantStatus(t, resp.StatusCode, 403, "tampered link")
		resp, _ = e2eGet(t, strings.Replace(link.URL, photoFile.ID.String(), bigFile.ID.String(), 1), "")
		wantStatus(t, resp.StatusCode, 403, "signature for another file")
		time.Sleep(time.Until(link.ExpiresAt) + 1500*time.Millisecond)
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
		if got, _ := mcpCall(t, srv.url, bobTok, "files", map[string]any{}); strings.Contains(got, "photo.jpg") {
			t.Errorf("bob's mcp files: %s", got)
		}
		in.fails("-as", "bob@example.com", "file", "get", id, "-o", "-")
		role := in.psql("archive", "SELECT role FROM users WHERE email = 'bob@example.com'")
		in.psql("archive", "ALTER ROLE "+role+" LOGIN")
		if got := in.psql(role, "SELECT count(*) FROM files WHERE name = 'photo.jpg'"); got != "0" {
			t.Errorf("psql as bob sees %s photos", got)
		}
		in.psql("archive", "ALTER ROLE "+role+" NOLOGIN")
	})

	t.Run("TextExtractionFeedsIntakeWithPageProvenance", func(t *testing.T) {
		in := in.on(t)
		st, f := e2eUpload(t, api+"/files?intake=invoice", aliceTok, "invoice.pdf", e2ePDF("Cover letter", "Invoice INV-9 total due"))
		wantStatus(t, st, 201, "upload pdf for intake")
		f = e2eWaitFile(t, api, aliceTok, f.ID.String())
		if f.Status != "done" || f.Metadata["pages"] != 2.0 || f.Intake != "" {
			t.Fatalf("pdf: %+v", f)
		}
		text, _ := mcpCall(t, srv.url, aliceTok, "file_text", map[string]any{"id": f.ID.String()})
		if !strings.Contains(text, "Cover letter") || !strings.Contains(text, "INV-9") || !strings.Contains(text, `\f`) {
			t.Errorf("pdf text: %s", text)
		}
		rs := decode[[]e2eRecord](t, in.must("list", "invoice", "number=INV-9"))
		if len(rs) != 1 {
			t.Fatalf("invoices: %+v", rs)
		}
		prov := decode[[]Provenance](t, in.must("provenance", "invoice", rs[0].ID))
		if len(prov) != 1 || prov[0].FileID == nil || *prov[0].FileID != f.ID || prov[0].Page != 2 || prov[0].Method != "intake:fake" {
			t.Errorf("provenance: %+v", prov)
		}

		_, page := e2eUpload(t, api+"/files", aliceTok, "order.html", []byte("<html><head><style>p{}</style><script>var INV = 1</script></head><body><p>Order</p><p>INV-5</p></body></html>"))
		page = e2eWaitFile(t, api, aliceTok, page.ID.String())
		resp, body := e2eGet(t, api+"/files/"+page.ID.String()+"/text", aliceTok)
		if resp.StatusCode != 200 || string(body) != "Order INV-5" {
			t.Errorf("html text: %d %q", resp.StatusCode, body)
		}
		res := decode[IntakeResult](t, in.must("intake", page.ID.String(), "-type", "invoice"))
		if got := decode[e2eRecord](t, in.must("get", "invoice", res.RecordID.String())); got.Data["number"] != "INV-5" {
			t.Errorf("intake from html: %+v", got)
		}

		eml := filepath.Join(in.dir, "lunch.eml")
		os.WriteFile(eml, []byte("From: Carol <carol@example.com>\r\nSubject: Lunch\r\nContent-Type: text/plain\r\n\r\nSee you at noon, INV-6"), 0o644)
		mail := decode[File](t, in.must("file", "add", eml, "-type", "invoice"))
		if mail.MIME != "message/rfc822" || mail.Status != "done" || !strings.Contains(fmt.Sprint(mail.Metadata["headers"]), "Subject:Lunch") {
			t.Errorf("eml: %+v", mail)
		}
		if out := in.must("file", "get", mail.ID.String(), "-o", "-"); !strings.HasPrefix(out, "From: Carol") {
			t.Errorf("file get -o -: %q", out)
		}
		if rs := decode[[]e2eRecord](t, in.must("list", "invoice", "number=INV-6")); len(rs) != 1 {
			t.Errorf("intake on file add: %+v", rs)
		}
	})

	t.Run("OCRAndTranscriptionGoThroughTheFakeOpenAIServer", func(t *testing.T) {
		var scanPNG bytes.Buffer
		png.Encode(&scanPNG, image.NewGray(image.Rect(0, 0, 40, 20)))
		_, scan := e2eUpload(t, api+"/files", aliceTok, "scan.png", scanPNG.Bytes())
		wav := append([]byte("RIFF\x24\x00\x00\x00WAVEfmt "), make([]byte, 64)...)
		_, memo := e2eUpload(t, api+"/files", aliceTok, "memo.wav", wav)
		for _, f := range []File{scan, memo} {
			if f := e2eWaitFile(t, api, aliceTok, f.ID.String()); f.Status != "done" {
				t.Fatalf("%s: %+v", f.Name, f)
			}
		}
		_, ocr := e2eGet(t, api+"/files/"+scan.ID.String()+"/text", aliceTok)
		_, speech := e2eGet(t, api+"/files/"+memo.ID.String()+"/text", aliceTok)
		ai.mu.Lock()
		defer ai.mu.Unlock()
		want := "data:image/png;base64," + base64.StdEncoding.EncodeToString(scanPNG.Bytes())
		if string(ocr) != "Receipt R-42\nTotal 12.50" || len(ai.images) != 2 || !slices.Contains(ai.images, want) || !strings.Contains(strings.Join(ai.models, ","), "vision-fake") {
			t.Errorf("ocr %q, images %d, models %v", ocr, len(ai.images), ai.models)
		}
		if string(speech) != "Hello from the voice memo" || strings.Join(ai.audio, ",") != fmt.Sprintf("whisper-fake:memo.wav:%d", len(wav)) {
			t.Errorf("transcription %q, calls %v", speech, ai.audio)
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

const e2eSearchSpec = `{"types": {"note": {"fields": {
  "title": {"type": "text", "required": true, "indexed": true},
  "body": {"type": "text"},
  "bsn": {"type": "nl_bsn"}}}}}`

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
		out = append(out, fmt.Sprint(cmp.Or[any](d["title"], d["name"], h.Source)))
	}
	return strings.Join(out, "|")
}

func TestFullTextSearchInDutchAndEnglishAcrossRecordsFilesRawAndUsers(t *testing.T) {
	in := newInst(t, "ARCHIVE_ADMIN_EMAIL=alice@example.com")
	srv := in.serve()
	api := srv.url + "/api"
	in.must("user", "add", "bob@example.com")
	spec := filepath.Join(in.dir, "search.json")
	os.WriteFile(spec, []byte(e2eSearchSpec), 0o644)
	in.must("spec", "push", spec)
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
	_, nl := e2eUpload(t, api+"/files", aliceTok, "huur.txt", []byte("De verhuurder verhuurt de woning aan de Prinsengracht aan de huurder."))
	_, en := e2eUpload(t, api+"/files", aliceTok, "policy.pdf", e2ePDF("Home insurance policy", "Covers water damage in the kitchen"))
	e2eWaitFile(t, api, aliceTok, nl.ID.String())
	e2eWaitFile(t, api, aliceTok, en.ID.String())
	inbox := filepath.Join(in.dir, "inbox")
	os.MkdirAll(inbox, 0o755)
	os.WriteFile(filepath.Join(inbox, "memo.txt"), []byte("Reminder: the plumber comes on Tuesday"), 0o644)
	in.must("source", "add", "folder", "-name", "inbox", `{"config":{"path":"`+inbox+`"}}`)
	in.must("source", "run", "inbox")

	t.Run("DutchAndEnglishRecordsRankByTheirWords", func(t *testing.T) {
		hits := e2eSearch(t, api, aliceTok, "huurcontract utrecht")
		if got := e2eTitles(hits); !strings.HasPrefix(got, "Huurcontract appartement Utrecht|Weekend away") {
			t.Fatalf("huurcontract utrecht: %s", got)
		}
		if hits[0].TextRank != 1 || hits[0].VectorRank != 0 || hits[0].Kind != "record" || hits[0].Type != "note" {
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

	t.Run("UnmappedSyncedItemsAreFoundByTheirSource", func(t *testing.T) {
		hits := e2eSearch(t, api, aliceTok, "plumber", "source", "inbox")
		if len(hits) != 1 || hits[0].Kind != "raw" || hits[0].Source != "folder:"+inbox || !strings.Contains(hits[0].Snippet, "**plumber**") {
			t.Fatalf("raw: %+v", hits)
		}
		if hits := e2eSearch(t, api, aliceTok, "huurcontract", "source", "inbox"); len(hits) != 0 {
			t.Errorf("source filter lets records through: %s", e2eTitles(hits))
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
		st, _ = call(t, "GET", api+"/search?q=huur&from=soon", aliceTok, nil, nil)
		wantStatus(t, st, 422, "bad date")
		st, _ = call(t, "GET", api+"/search?q=+", aliceTok, nil, nil)
		wantStatus(t, st, 422, "empty query")
	})

	t.Run("SecretsNeverMatch", func(t *testing.T) {
		in := in.on(t)
		if hits := e2eSearch(t, api, aliceTok, "belastingaangifte"); len(hits) != 1 {
			t.Fatalf("secret record by title: %+v", hits)
		}
		for _, q := range []string{"111222333", "2333", "111 222 333"} {
			if hits := e2eSearch(t, api, aliceTok, q); len(hits) != 0 {
				t.Errorf("%s matches %s", q, e2eTitles(hits))
			}
		}
		if n := in.psql("archive", "SELECT count(*) FROM records WHERE search_record(type, data)::text LIKE '%2333%' OR search_text(data) LIKE '%2333%'"); n != "0" {
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
		if got := e2eSearch(t, api, bobTok, "verhuurder plumber cafe"); len(got) != 0 {
			t.Errorf("bob sees alice's files and raw: %+v", got)
		}
		if got := in.must("-as", "bob@example.com", "search", "huurcontract", "utrecht"); !strings.Contains(got, "Bob huurcontract") || strings.Contains(got, "appartement") {
			t.Errorf("bob cli: %s", got)
		}
		if got := in.must("search", "huurcontract", "-type", "note"); !strings.Contains(got, "appartement") || strings.Contains(got, "Bob") {
			t.Errorf("alice cli: %s", got)
		}
		if got, isErr := mcpCall(t, srv.url, aliceTok, "search", map[string]any{"query": "the lease for the Utrecht flat"}); isErr || !strings.Contains(got, "Huurcontract appartement Utrecht") || strings.Contains(got, "Bob") {
			t.Errorf("alice mcp: %s", got)
		}
		if got, _ := mcpCall(t, srv.url, bobTok, "search", map[string]any{"query": "huurcontract utrecht"}); strings.Contains(got, "appartement") {
			t.Errorf("bob mcp: %s", got)
		}
		role := in.psql("archive", "SELECT role FROM users WHERE email = 'bob@example.com'")
		in.psql("archive", "ALTER ROLE "+role+" LOGIN")
		defer in.psql("archive", "ALTER ROLE "+role+" NOLOGIN")
		q := "SELECT (SELECT count(*) FROM records WHERE search_record(type, data) @@ search_query('huurcontract verhuurder')) || ',' || " +
			"(SELECT count(*) FROM files WHERE search_file(text) @@ search_query('verhuurder')) || ',' || (SELECT count(*) FROM raw WHERE search_raw(payload) @@ search_query('plumber'))"
		if got := in.psql(role, q); got != "1,0,0" {
			t.Errorf("psql as bob: %s", got)
		}
	})

	t.Run("IndexedFieldsGetExpressionIndexes", func(t *testing.T) {
		in := in.on(t)
		q := "SELECT count(*) FROM pg_indexes WHERE tablename = 'records' AND indexdef LIKE '%(data ->> ''title''::text)%'"
		if n := in.psql("archive", q); n != "1" {
			t.Fatalf("title index: %s", n)
		}
		if rs := decode[[]e2eRecord](t, in.must("list", "note", "title=Weekend away")); len(rs) != 1 {
			t.Errorf("list by indexed field: %+v", rs)
		}
		os.WriteFile(spec, []byte(strings.Replace(e2eSearchSpec, `, "indexed": true`, "", 1)), 0o644)
		in.must("spec", "push", spec)
		if n := in.psql("archive", q); n != "0" {
			t.Errorf("title index kept after indexed was dropped: %s", n)
		}
	})

	t.Run("WithoutAnEmbeddingModelSearchIsFullTextOnly", func(t *testing.T) {
		in := in.on(t)
		if n := in.psql("archive", "SELECT count(*) FROM pg_tables WHERE tablename = 'embeddings'"); n != "0" {
			t.Errorf("embeddings table without a model: %s", n)
		}
		for _, h := range e2eSearch(t, api, aliceTok, "huur") {
			if h.VectorRank != 0 {
				t.Errorf("vector rank without a model: %+v", h)
			}
		}
	})
}

type e2eEmbedder struct {
	mu    sync.Mutex
	fail  bool
	calls int
}

var e2eConcepts = [][]string{{"huur", "lease", "rent", "tenancy", "apartment", "appartement", "woning", "flat"}, {"factuur", "invoice", "bill", "energie"}, {"foto", "photo", "whiteboard"}, {"utrecht"}}

func e2eVector(text string) []float64 {
	v := make([]float64, 8)
	v[7] = 0.05
	for _, w := range strings.Fields(strings.ToLower(text)) {
		for i, c := range e2eConcepts {
			if slices.ContainsFunc(c, func(s string) bool { return strings.HasPrefix(w, s) }) {
				v[i]++
			}
		}
	}
	return v
}

func (e *e2eEmbedder) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/embeddings", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		e.mu.Lock()
		e.calls++
		fail := e.fail
		e.mu.Unlock()
		if fail || req.Model != "embed-fake" {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"message": "model is loading"}})
			return
		}
		var data []any
		for i, s := range req.Input {
			data = append(data, map[string]any{"index": i, "embedding": e2eVector(s)})
		}
		json.NewEncoder(w).Encode(map[string]any{"data": data})
	})
	mux.HandleFunc("POST /v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": "Whiteboard with the Q3 roadmap"}}}})
	})
	return mux
}

type e2eVecInst struct {
	url     string
	cli     func(stdin string, args ...string) string
	psql    func(user, sql string) string
	restart func(between func())
}

func e2eVecLocal(t *testing.T, llm string) *e2eVecInst {
	in := newInst(t, "ARCHIVE_ADMIN_EMAIL=alice@example.com", "ARCHIVE_LLM_URL="+llm+"/v1", "ARCHIVE_EMBED_MODEL=embed-fake", "ARCHIVE_EMBED_DIM=8", "ARCHIVE_VISION_MODEL=vision-fake")
	srv := in.serve()
	v := &e2eVecInst{url: srv.url, psql: in.psql}
	v.cli = func(stdin string, args ...string) string {
		t.Helper()
		out, errs, err := in.run(stdin, args...)
		if err != nil {
			t.Fatalf("archive %s: %v\n%s%s", strings.Join(args, " "), err, out, errs)
		}
		return out
	}
	v.restart = func(between func()) {
		srv.stop()
		between()
		srv = in.serve()
		v.url = srv.url
	}
	return v
}

func e2eVecDocker(t *testing.T, llm string) *e2eVecInst {
	name := fmt.Sprintf("archive-e2e-vec-%d", time.Now().UnixNano())
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
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(llm, "http://"))
	docker("", "run", "-d", "--name", name, "-v", name+":/data", "-p", "127.0.0.1::8080", "-e", "ARCHIVE_ADMIN_EMAIL=alice@example.com",
		"-e", "ARCHIVE_LLM_URL=http://host.docker.internal:"+port+"/v1", "-e", "ARCHIVE_EMBED_MODEL=embed-fake", "-e", "ARCHIVE_EMBED_DIM=8",
		"-e", "ARCHIVE_VISION_MODEL=vision-fake", name)
	v := &e2eVecInst{}
	ready := func() {
		for deadline := time.Now().Add(90 * time.Second); ; time.Sleep(500 * time.Millisecond) {
			if exec.Command("docker", "exec", name, "wget", "-q", "-O", "/dev/null", "http://127.0.0.1:8080/healthz").Run() == nil {
				break
			}
			if time.Now().After(deadline) {
				out, _ := exec.Command("docker", "logs", name).CombinedOutput()
				t.Fatalf("container not healthy:\n%s", out)
			}
		}
		v.url = "http://" + strings.TrimSpace(strings.Split(docker("", "port", name, "8080"), "\n")[0])
	}
	ready()
	v.cli = func(stdin string, args ...string) string {
		return docker(stdin, append([]string{"exec", "-i", name, "archive"}, args...)...)
	}
	v.psql = func(user, sql string) string {
		return strings.TrimSpace(docker("", "exec", name, "psql", "-X", "-h", "/data", "-U", user, "-d", "archive", "-Atc", sql))
	}
	v.restart = func(between func()) {
		docker("", "stop", name)
		between()
		docker("", "start", name)
		ready()
	}
	return v
}

func TestSemanticAndHybridSearchWithPgvector(t *testing.T) {
	emb := &e2eEmbedder{}
	llm := httptest.NewServer(emb.handler())
	defer llm.Close()
	var v *e2eVecInst
	share, _ := exec.Command("pg_config", "--sharedir").Output()
	if _, err := os.Stat(filepath.Join(strings.TrimSpace(string(share)), "extension", "vector.control")); err == nil {
		v = e2eVecLocal(t, llm.URL)
	} else if exec.Command("docker", "info").Run() == nil {
		t.Log("the local Postgres has no pgvector; running the vector assertions in the Docker image")
		v = e2eVecDocker(t, llm.URL)
	} else {
		t.Skip("vector assertions skipped: the local Postgres has no pgvector and Docker is not available; full-text search is covered by TestFullTextSearchInDutchAndEnglishAcrossRecordsFilesRawAndUsers")
	}
	api := func() string { return v.url + "/api" }
	v.cli("", "user", "add", "bob@example.com")
	v.cli(e2eSearchSpec, "spec", "push", "-")
	aliceTok := decode[Token](t, v.cli("", "token", "create")).Secret
	bobTok := decode[Token](t, v.cli("", "-as", "bob@example.com", "token", "create")).Secret
	note := func(tok, title, body string) string {
		t.Helper()
		var r e2eRecord
		st, _ := call(t, "POST", api()+"/note", tok, map[string]any{"title": title, "body": body}, &r)
		wantStatus(t, st, 201, "create note")
		return r.ID
	}
	lease := note(aliceTok, "Huurcontract woning Oudegracht", "Maandelijkse huur vooruit betalen")
	trip := note(aliceTok, "Utrecht trip", "Foto's van de Dom")
	note(aliceTok, "Factuur energie", "Eneco september")
	bobLease := note(bobTok, "Huurcontract van Bob", "huur woning")
	var shot bytes.Buffer
	png.Encode(&shot, image.NewGray(image.Rect(0, 0, 40, 20)))
	_, scan := e2eUpload(t, api()+"/files", aliceTok, "scan.png", shot.Bytes())
	e2eWaitFile(t, api(), aliceTok, scan.ID.String())
	settled := func(want int) {
		t.Helper()
		for deadline := time.Now().Add(60 * time.Second); ; time.Sleep(200 * time.Millisecond) {
			got := v.psql("archive", "SELECT (SELECT count(*) FROM embed_queue) || ',' || (SELECT count(DISTINCT record_id) FROM embeddings)")
			if got == "0,"+strconv.Itoa(want) {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("queue, embedded = %s, want 0,%d", got, want)
			}
		}
	}
	settled(5)

	t.Run("ACloseMeaningWinsWithoutSharedWords", func(t *testing.T) {
		hits := e2eSearch(t, api(), aliceTok, "rental agreement for an apartment")
		if len(hits) == 0 || hits[0].ID.String() != lease || hits[0].VectorRank != 1 || hits[0].TextRank != 0 {
			t.Fatalf("semantic: %+v", hits)
		}
		for _, h := range hits {
			if h.ID.String() == bobLease {
				t.Errorf("bob's lease in alice's results")
			}
		}
	})

	t.Run("HybridFusionCombinesBothRankings", func(t *testing.T) {
		hits := e2eSearch(t, api(), aliceTok, "Utrecht huur")
		if len(hits) < 2 || hits[0].ID.String() != lease && hits[0].ID.String() != trip {
			t.Fatalf("hybrid: %+v", hits)
		}
		both := 0
		for _, h := range hits[:2] {
			if h.TextRank > 0 && h.VectorRank > 0 {
				both++
			}
		}
		if both != 2 || hits[0].Score <= hits[len(hits)-1].Score {
			t.Errorf("hybrid ranks: %+v", hits)
		}
	})

	t.Run("PhotosAreFoundByTheirOCRText", func(t *testing.T) {
		hits := e2eSearch(t, api(), aliceTok, "photos of whiteboards")
		if len(hits) == 0 || hits[0].ID != scan.ID || hits[0].Type != "file" || hits[0].VectorRank != 1 {
			t.Fatalf("photo: %+v", hits)
		}
	})

	t.Run("AnotherUsersVectorsNeverShowUp", func(t *testing.T) {
		if out := v.cli("", "-as", "bob@example.com", "search", "rental", "apartment"); !strings.Contains(out, "Huurcontract van Bob") || strings.Contains(out, "Oudegracht") {
			t.Errorf("bob cli: %s", out)
		}
		if out, isErr := mcpCall(t, v.url, aliceTok, "search", map[string]any{"query": "rental apartment"}); isErr || !strings.Contains(out, "Oudegracht") || strings.Contains(out, "van Bob") {
			t.Errorf("alice mcp: %s", out)
		}
		role := v.psql("archive", "SELECT role FROM users WHERE email = 'bob@example.com'")
		v.psql("archive", "ALTER ROLE "+role+" LOGIN")
		defer v.psql("archive", "ALTER ROLE "+role+" NOLOGIN")
		if got := v.psql(role, "SELECT string_agg(DISTINCT record_id::text, ',') FROM embeddings"); got != bobLease {
			t.Errorf("psql as bob sees embeddings of %s", got)
		}
	})

	t.Run("TheWorkerResumesAfterARestart", func(t *testing.T) {
		emb.mu.Lock()
		emb.fail = true
		emb.mu.Unlock()
		bill := note(aliceTok, "Waterbedrijf", "Factuur kwartaal drie")
		for deadline := time.Now().Add(30 * time.Second); v.psql("archive", "SELECT tries FROM embed_queue WHERE record_id = '"+bill+"'") < "1"; time.Sleep(200 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatal("the failing embedding was not retried")
			}
		}
		v.restart(func() {
			emb.mu.Lock()
			emb.fail = false
			emb.mu.Unlock()
		})
		settled(6)
		hits := e2eSearch(t, api(), aliceTok, "invoice", "type", "note")
		if len(hits) < 2 || hits[0].VectorRank == 0 || !strings.Contains(e2eTitles(hits[:2]), "Waterbedrijf") {
			t.Errorf("after restart: %s %+v", e2eTitles(hits), hits)
		}
	})
}

func TestSavedViewsSourceUpdatesAndFullTokensOverHTTP(t *testing.T) {
	in := newInst(t, "ARCHIVE_ADMIN_EMAIL=alice@example.com", `ARCHIVE_OIDC=[{"name":"google","issuer":"https://accounts.google.com","client_id":"web"}]`)
	srv := in.serve()
	api := srv.url + "/api"
	in.must("user", "add", "bob@example.com")
	in.must("spec", "push", in.writeSpec())
	in.must("space", "add", "family")
	in.must("space", "grant", "family", "bob@example.com", "viewer")
	aliceTok := decode[Token](t, in.must("token", "create")).Secret
	bobTok := decode[Token](t, in.must("-as", "bob@example.com", "token", "create")).Secret

	t.Run("SavedViewsFollowSpaceMembership", func(t *testing.T) {
		var v View
		body := map[string]any{"name": "Family people", "space": "family", "type": "person", "layout": "board", "by": "employer", "where": map[string]string{"name": "Ann"}}
		st, _ := call(t, "POST", api+"/views", aliceTok, body, &v)
		wantStatus(t, st, 201, "create view")
		if v.Layout != "board" || v.By != "employer" || v.Where["name"] != "Ann" || !strings.HasSuffix(v.CreatedBy, "on behalf of alice@example.com") {
			t.Fatalf("view: %+v", v)
		}
		st, _ = call(t, "POST", api+"/views", aliceTok, body, nil)
		wantStatus(t, st, 409, "same name in the same space")
		st, _ = call(t, "POST", api+"/views", aliceTok, map[string]any{"name": "Bad", "type": "person", "by": "nope"}, nil)
		wantStatus(t, st, 422, "unknown field")
		st, _ = call(t, "POST", api+"/views", aliceTok, map[string]any{"name": "Mine", "type": "company"}, nil)
		wantStatus(t, st, 201, "private view")

		var seen []View
		call(t, "GET", api+"/views", bobTok, nil, &seen)
		if len(seen) != 1 || seen[0].ID != v.ID {
			t.Fatalf("bob sees %+v", seen)
		}
		st, _ = call(t, "PUT", api+"/views/"+v.ID.String(), bobTok, map[string]any{"name": "Hijacked", "type": "person"}, nil)
		wantStatus(t, st, 403, "viewer changes a view")
		var u View
		st, _ = call(t, "PUT", api+"/views/"+v.ID.String(), aliceTok, map[string]any{"name": "Everyone", "type": "person"}, &u)
		wantStatus(t, st, 200, "owner changes a view")
		if u.Name != "Everyone" || u.Space != v.Space || u.Layout != "table" || len(u.Where) != 0 || u.By != "" {
			t.Errorf("updated view: %+v", u)
		}
		st, _ = call(t, "DELETE", api+"/views/"+v.ID.String(), bobTok, nil, nil)
		wantStatus(t, st, 403, "viewer removes a view")
		st, _ = call(t, "DELETE", api+"/views/"+v.ID.String(), aliceTok, nil, nil)
		wantStatus(t, st, 204, "owner removes a view")
		st, _ = call(t, "GET", api+"/views/"+v.ID.String(), aliceTok, nil, nil)
		wantStatus(t, st, 404, "removed view")
	})

	t.Run("SourceScheduleRateAndTypeChangeInPlace", func(t *testing.T) {
		inbox := filepath.Join(in.dir, "inbox")
		os.MkdirAll(inbox, 0o755)
		var s Source
		st, _ := call(t, "POST", api+"/sources", aliceTok, map[string]any{"kind": "folder", "name": "inbox", "config": map[string]string{"path": inbox}}, &s)
		wantStatus(t, st, 201, "add source")
		var u Source
		st, _ = call(t, "PATCH", api+"/sources/"+s.ID.String(), aliceTok, map[string]any{"schedule": "1h", "rate": 2, "type": "invoice"}, &u)
		wantStatus(t, st, 200, "patch source")
		if u.ID != s.ID || u.Schedule != "1h" || u.Rate != 2 || u.Type != "invoice" || u.NextRunAt == nil {
			t.Fatalf("patched: %+v", u)
		}
		var c Source
		st, _ = call(t, "PATCH", api+"/sources/inbox", aliceTok, map[string]any{"schedule": ""}, &c)
		wantStatus(t, st, 200, "clear schedule")
		if c.Schedule != "" || c.NextRunAt != nil || c.Type != "invoice" || c.Rate != 2 {
			t.Errorf("cleared: %+v", c)
		}
		st, _ = call(t, "PATCH", api+"/sources/inbox", aliceTok, map[string]any{"schedule": "soon"}, nil)
		wantStatus(t, st, 422, "bad schedule")
		st, _ = call(t, "PATCH", api+"/sources/"+s.ID.String(), bobTok, map[string]any{"rate": 9}, nil)
		wantStatus(t, st, 404, "another user's source")
	})

	t.Run("RemovingATypeThatHasObjectsApplies", func(t *testing.T) {
		st, _ := call(t, "POST", api+"/invoice", aliceTok, map[string]any{"number": "INV-1"}, nil)
		wantStatus(t, st, 201, "create invoice")
		var c Change
		call(t, "POST", api+"/changes", aliceTok, map[string]any{"ops": []Op{{Op: "remove_type", Type: "invoice"}}}, &c)
		var p Preview
		st, _ = call(t, "POST", api+"/changes/"+c.ID.String()+"/preview", aliceTok, nil, &p)
		wantStatus(t, st, 200, "preview type removal")
		if p.Touched != 1 || p.Failing != 1 {
			t.Errorf("preview: touched %d, failing %d", p.Touched, p.Failing)
		}
		st, _ = call(t, "POST", api+"/changes/"+c.ID.String()+"/apply", aliceTok, nil, &c)
		wantStatus(t, st, 200, "apply type removal")
		st, _ = call(t, "GET", api+"/invoice", aliceTok, nil, nil)
		wantStatus(t, st, 404, "list removed type")
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

	t.Run("ChangeHistoryBuiltinFlagsAndSpaceFilters", func(t *testing.T) {
		var types struct {
			Spec    Spec     `json:"spec"`
			Builtin []string `json:"builtin"`
			Edited  []string `json:"edited"`
		}
		call(t, "GET", api+"/types", bobTok, nil, &types)
		if !slices.Contains(types.Builtin, "email") || slices.Contains(types.Builtin, "person") || types.Spec.Types["person"] == nil || len(types.Edited) != 0 {
			t.Errorf("types: builtin %v, edited %v", types.Builtin, types.Edited)
		}
		var cs []Change
		st, _ := call(t, "GET", api+"/changes", bobTok, nil, &cs)
		wantStatus(t, st, 200, "viewer lists changes")
		if len(cs) < 2 || cs[0].Status != "applied" || cs[0].Ops[0].Op != "remove_type" || !strings.Contains(cs[0].ProposedBy, "alice@example.com") ||
			slices.ContainsFunc(cs, func(c Change) bool { return strings.HasPrefix(c.ProposedBy, "revert:") }) {
			t.Fatalf("changes: %+v", cs)
		}
		removed := cs[0]
		st, _ = call(t, "POST", api+"/changes/"+removed.ID.String()+"/revert", aliceTok, nil, nil)
		wantStatus(t, st, 200, "revert")
		call(t, "GET", api+"/changes", aliceTok, nil, &cs)
		if cs[0].ProposedBy != "revert:"+removed.ID.String() || cs[0].Status != "applied" || cs[0].FromVer != removed.ToVer || cs[1].ID != removed.ID || cs[1].Status != "reverted" {
			t.Errorf("after revert: %+v", cs[:2])
		}

		var fam, own e2eRecord
		call(t, "POST", api+"/company?space=family", aliceTok, map[string]any{"name": "Family firm"}, &fam)
		call(t, "POST", api+"/company", aliceTok, map[string]any{"name": "Own firm"}, &own)
		var rs []e2eRecord
		st, _ = call(t, "GET", api+"/company?space=family", aliceTok, nil, &rs)
		wantStatus(t, st, 200, "list one space")
		if len(rs) != 1 || rs[0].ID != fam.ID {
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
	})
}
