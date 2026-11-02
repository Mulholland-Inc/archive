package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// e2eSave saves the workflow name, creating it or changing the one there is, and returns its row.
func e2eSave(t *testing.T, api, tok, name, source string, on any) map[string]any {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"name": name, "source": source, "on": on})
	st, out := e2eRun(t, api, tok, `const w = `+string(b)+`;
const [old] = await archive.list("workflow", { where: { name: w.name } });
return old ? await archive.update("workflow", old.id, w) : await archive.create("workflow", w);`)
	if st != 200 || out.Error != "" {
		t.Fatalf("save %s: %d %+v", name, st, out)
	}
	return decode[map[string]any](t, string(out.Result))
}

// e2eWorkflowRuns are the runs of the workflow name, newest first, as its owner reads them from workflow_run.
func e2eWorkflowRuns(t *testing.T, api, tok, name string) []WorkflowRun {
	t.Helper()
	b, _ := json.Marshal(name)
	st, out := e2eRun(t, api, tok, `return await archive.query("SELECT r.id, r.created_at AS at, r.trigger, r.ms, r.result, r.logs, r.error FROM workflow_run r JOIN workflow w ON w.id = r.workflow WHERE w.name = $1 ORDER BY r.created_at DESC, r.id DESC", `+string(b)+`)`)
	if st != 200 || out.Error != "" {
		t.Fatalf("runs of %s: %d %+v", name, st, out)
	}
	return decode[[]WorkflowRun](t, string(out.Result))
}

func e2ePost(t *testing.T, url, ctype, body string) (int, string) {
	t.Helper()
	resp, err := http.Post(url, ctype, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, strings.TrimSpace(string(b))
}

func TestWorkflowsAreRowsThatRunAndFireOnTheirOwn(t *testing.T) {
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
	viewer := decode[Token](t, in.must("token", "create", "-viewer")).Secret
	bobTok := decode[Token](t, in.must("-as", "bob@example.com", "token", "create")).Secret
	run := func(t *testing.T, tok, code string) any {
		t.Helper()
		var out RunOutput
		st, _ := call(t, "POST", api+"/run", tok, map[string]string{"code": code}, &out)
		wantStatus(t, st, 200, "run")
		if out.Error != "" {
			t.Fatalf("run: %s\nlogs: %v", out.Error, out.Logs)
		}
		return decode[map[string]any](t, "{\"v\":"+string(out.Result)+"}")["v"]
	}
	js := func(v any) string { b, _ := json.Marshal(v); return string(b) }
	runs := func(t *testing.T, name string) []WorkflowRun { return e2eWorkflowRuns(t, api, tok, name) }
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
	saved := map[string]map[string]any{}

	t.Run("ABadSaveIsRefusedWithEveryErrorAndSavesNothing", func(t *testing.T) {
		got := run(t, tok, `const tries = {
  compile: { name: "broken", source: "const x = 1;\nconst y = ;" },
  cron: { name: "late", source: "return 1", on: { cron: "0 9 * *" } },
  zone: { name: "zoned", source: "return 1", on: { cron: "0 9 * * *", tz: "Mars/Olympus" } },
  table: { name: "where", source: "return 1", on: { table: "shed" } },
  events: { name: "events", source: "return 1", on: { table: "house", events: ["deleted"] } },
  two: { name: "two", source: "return 1", on: { cron: "@daily", webhook: true } },
  name: { name: "Bad Name", source: "return 1" },
  role: { name: "sneaky", source: "return 1", role: "x_owner" },
  empty: { name: "empty", source: " " },
};
const out = {};
for (const [k, w] of Object.entries(tries)) {
  out[k] = await archive.create("workflow", w).then(() => "saved", (e) => ({ code: e.code, errors: e.errors }));
}
out.after = (await archive.list("workflow")).length;
return out;`).(map[string]any)
		want := map[string]string{
			"compile": `[map[column:11 line:2 message:line 2:11: Unexpected ";" path:source]]`,
			"cron":    "expected exactly 5 fields",
			"zone":    `unknown time zone "Mars/Olympus"`,
			"table":   `unknown table "shed"; the tables are house, member`,
			"events":  "path:on.events[0]",
			"two":     "exactly one of cron, table or webhook",
			"name":    "path:name",
			"role":    "path:role",
			"empty":   "path:source",
		}
		for k, v := range want {
			e, _ := got[k].(map[string]any)
			if e["code"] != "invalid" || !strings.Contains(fmt.Sprint(e["errors"]), v) {
				t.Errorf("%s: %v, want invalid with %q", k, got[k], v)
			}
		}
		if got["after"] != json.Number("0") {
			t.Errorf("a refused save saved: %v", got["after"])
		}
	})

	t.Run("InsertUpdateRenameAndDeleteLikeRows", func(t *testing.T) {
		saved["hook"] = e2eSave(t, api, tok, "hook", "return { got: input.body, method: input.method, x: input.query.x ?? null, type: input.headers['content-type'] ?? null, trigger: input.trigger };", map[string]any{"webhook": true})
		saved["nightly"] = e2eSave(t, api, tok, "nightly", "return 1;", map[string]any{"cron": "0 9 * * 1-5", "tz": "America/New_York"})
		saved["manual"] = e2eSave(t, api, tok, "manual", "await archive.create('house', { address: input?.address ?? 'Manual 1' });\nconsole.log('made', input?.address);\nreturn 'made';", nil)
		saved["boom"] = e2eSave(t, api, tok, "boom", "throw new Error('bad input: ' + JSON.stringify(input.body));", map[string]any{"webhook": true})
		h, n, m := saved["hook"], saved["nightly"], saved["manual"]
		url := fmt.Sprint(h["url"])
		if !strings.Contains(url, srv.url[strings.LastIndex(srv.url, ":"):]+"/hooks/run/") || len(url) < 60 || fmt.Sprint(h["on"]) != "map[webhook:true]" || h["next_at"] != nil {
			t.Errorf("the webhook workflow: %v", h)
		}
		for _, k := range []string{"role", "hook", "user_id", "member", "trigger", "cursor_at", "cursor_id"} {
			if _, ok := h[k]; ok {
				t.Errorf("a workflow row shows %s: %v", k, h)
			}
		}
		if n["next_at"] == nil || n["url"] != nil || fmt.Sprint(n["on"]) != "map[cron:0 9 * * 1-5 tz:America/New_York]" || m["on"] != nil || m["url"] != nil || m["next_at"] != nil {
			t.Errorf("the cron and manual workflows: %v %v", n, m)
		}
		dup := run(t, tok, `return await archive.create("workflow", { name: "manual", source: "return 1" }).then(() => "saved", (e) => e.code);`)
		if dup != "conflict" {
			t.Errorf("a second workflow named manual: %v", dup)
		}

		// An edit and a rename keep the webhook URL; the cron workflow's next time follows its new schedule.
		got := run(t, tok, `const hook = await archive.update("workflow", `+js(h["id"])+`, { source: "return 'edited';" });
const renamed = await archive.update("workflow", hook.id, { name: "hook2" });
const nightly = await archive.update("workflow", `+js(n["id"])+`, { on: { cron: "0 9 1 * *" } });
const same = await archive.update("workflow", nightly.id, { source: "return 2;" });
return { url: renamed.url, name: renamed.name, edited: hook.source, next: nightly.next_at, same: same.next_at };`).(map[string]any)
		if got["url"] != url || got["name"] != "hook2" || got["edited"] != "return 'edited';" || got["next"] == n["next_at"] || got["same"] != got["next"] {
			t.Errorf("after the edits: %v, want url %s and a new next time from %v", got, url, n["next_at"])
		}
		if st, body := e2ePost(t, url, "application/json", "{}"); st != 200 || body != `"edited"` {
			t.Errorf("the edited webhook: %d %s", st, body)
		}
		if back := run(t, tok, `return (await archive.update("workflow", `+js(h["id"])+`, { name: "hook" })).url;`); back != url {
			t.Errorf("renamed back: %v", back)
		}
	})

	t.Run("ATokenCannotSaveAWorkflowThatGainsItRights", func(t *testing.T) {
		adder := e2eSave(t, api, tok, "adder", "await archive.create('house', { address: 'Hooked ' + input.body.n }); return 'added';", map[string]any{"webhook": true})
		url := fmt.Sprint(adder["url"])
		if st, body := e2ePost(t, url, "application/json", `{"n": 1}`); st != 200 || body != `"added"` {
			t.Fatalf("alice's adder: %d %s", st, body)
		}
		// The viewer token sees alice's workflows and may save one, but it then runs as the token, which only reads.
		got := run(t, viewer, `const [w] = await archive.list("workflow", { where: { name: "adder" } });
const saved = await archive.update("workflow", w.id, { source: w.source + "\n// checked" });
const made = await archive.create("workflow", { name: "viewer-add", source: "await archive.create('house', { address: 'Viewer' });" });
return { url: saved.url, made: made.name };`).(map[string]any)
		if got["url"] != url || got["made"] != "viewer-add" {
			t.Fatalf("the viewer token's saves: %v", got)
		}
		if st, body := e2ePost(t, url, "application/json", `{"n": 2}`); st != 500 || !strings.Contains(body, "permission denied") {
			t.Errorf("the adder saved by a viewer token wrote: %d %s", st, body)
		}
		if role := in.psql("archive", "SELECT left(role, length(role) - 22) FROM archive.workflow WHERE name = 'viewer-add'"); !strings.HasSuffix(role, "_t") {
			t.Errorf("a workflow the viewer token made runs as %q", role)
		}
		e2eSave(t, api, tok, "adder", fmt.Sprint(adder["source"]), map[string]any{"webhook": true})
		if st, body := e2ePost(t, url, "application/json", `{"n": 3}`); st != 200 || body != `"added"` {
			t.Errorf("the adder saved again by alice: %d %s", st, body)
		}
		run(t, tok, `for (const w of await archive.list("workflow")) if (w.name === "adder" || w.name === "viewer-add") await archive.delete("workflow", w.id); return 0;`)
	})

	t.Run("RunRunsForRealInsideTheRunAndRecordsIt", func(t *testing.T) {
		got := run(t, tok, `const out = await archive.run("manual", { address: "Started 1" });
const failed = await archive.run("boom", { body: { x: 1 } });
const missing = await archive.run("nosuch").then(() => "", (e) => e.code + ": " + e.message);
const fresh = await archive.create("workflow", { name: "fresh", source: "return input * 2;" });
const twice = await archive.run("fresh", 21);
await archive.delete("workflow", fresh.id);
return { out, failed: failed.error, missing, twice: twice.result };`).(map[string]any)
		if fmt.Sprint(got["out"]) != "map[error: logs:[made Started 1] result:made]" || got["failed"] != "bad input: {\"x\":1}\n  at: throw new Error(\"bad input: \" + JSON.stringify(input.body));" ||
			got["missing"] != "not_found: workflow nosuch not found; yours are boom, hook, manual, nightly" || got["twice"] != json.Number("42") {
			t.Errorf("archive.run: %v", got)
		}
		rs := runs(t, "manual")
		if len(rs) != 1 || rs[0].Trigger != "run" || string(rs[0].Result) != `"made"` || rs[0].Error != "" || string(rs[0].Logs) != `["made Started 1"]` || rs[0].At.IsZero() {
			t.Errorf("the recorded run: %+v", rs)
		}
		if b := runs(t, "boom"); len(b) != 1 || b[0].Trigger != "run" || !strings.HasPrefix(b[0].Error, "bad input") {
			t.Errorf("boom's run: %+v", b)
		}
		if n := run(t, tok, `return (await archive.list("house", { where: { address: "Started 1" } })).length;`); n != json.Number("1") {
			t.Errorf("houses archive.run made: %v", n)
		}
		// A run that throws after archive.run keeps neither the workflow's writes nor its run.
		e2eRun(t, api, tok, `await archive.run("manual", { address: "Thrown 1" }); throw new Error("no")`)
		if rs := runs(t, "manual"); len(rs) != 1 {
			t.Errorf("a rolled-back run was recorded: %+v", rs)
		}
	})

	t.Run("RunsAreReadOnlyAndOnlyTheirOwnersSeeThem", func(t *testing.T) {
		manual := saved["manual"]
		var rs []row
		st, _ := call(t, "GET", api+"/workflow_run?workflow="+fmt.Sprint(manual["id"]), tok, nil, &rs)
		if wantStatus(t, st, 200, "list runs"); len(rs) != 1 || rs[0]["trigger"] != "run" {
			t.Errorf("alice's runs of manual over HTTP: %v", rs)
		}
		joined := run(t, tok, `return await archive.query("SELECT w.name, count(r.id)::int AS n FROM workflow w LEFT JOIN workflow_run r ON r.workflow = w.id GROUP BY w.name ORDER BY w.name");`)
		if fmt.Sprint(joined) != "[map[n:1 name:boom] map[n:1 name:hook] map[n:1 name:manual] map[n:0 name:nightly]]" {
			t.Errorf("runs joined to workflows: %v", joined)
		}
		for _, c := range []struct {
			method, path string
			body         any
		}{{"POST", "/workflow_run", map[string]any{"workflow": manual["id"], "trigger": "run"}}, {"PATCH", "/workflow_run/" + fmt.Sprint(rs[0]["id"]), map[string]any{"error": "x"}},
			{"DELETE", "/workflow_run/" + fmt.Sprint(rs[0]["id"]), nil}} {
			st, _ := call(t, c.method, api+c.path, tok, c.body, nil)
			wantStatus(t, st, 405, c.method+" "+c.path)
		}
		if st, _ := call(t, "GET", api+"/workflow_run", "", nil, nil); st != 401 {
			t.Errorf("runs without a token: %d", st)
		}
		sql := run(t, tok, `return await archive.query("DELETE FROM workflow_run RETURNING id").then(() => "deleted", (e) => e.code);`)
		if sql != "forbidden" || len(runs(t, "manual")) != 1 {
			t.Errorf("a delete in SQL: %v", sql)
		}
		if out, err := exec.Command("psql", "-X", "-h", in.dir, "-U", "archive", "-d", "archive", "-Atc", `SET ROLE "`+in.role("alice@example.com")+`"; INSERT INTO archive.workflow_run (workflow, trigger) VALUES ('`+fmt.Sprint(manual["id"])+`', 'run')`).CombinedOutput(); err == nil || !strings.Contains(string(out), "permission denied") {
			t.Errorf("alice's role inserts a run: %v %s", err, out)
		}

		bob := run(t, bobTok, `return {
  workflows: (await archive.list("workflow")).length,
  runs: (await archive.list("workflow_run")).length,
  sql: (await archive.query("SELECT count(*)::int AS n FROM workflow_run"))[0].n,
  get: await archive.get("workflow", `+js(manual["id"])+`).then(() => "", (e) => e.code),
  update: await archive.update("workflow", `+js(manual["id"])+`, { source: "return 'bob';" }).then(() => "", (e) => e.code),
  del: await archive.delete("workflow", `+js(manual["id"])+`).then(() => "", (e) => e.code),
  run: await archive.run("manual").then(() => "", (e) => e.message),
};`)
		if fmt.Sprint(bob) != `map[del:not_found get:not_found run:workflow manual not found; you have none yet, save one with archive.create("workflow", { name, source, on }) runs:0 sql:0 update:not_found workflows:0]` {
			t.Errorf("bob sees alice's workflows: %v", bob)
		}
	})

	t.Run("FetchCallsOtherServicesSynchronously", func(t *testing.T) {
		got := run(t, tok, `const res = await fetch(`+js(echo.URL+"/echo?x=1")+`, { method: "POST", headers: { "X-Test": "yes" }, body: { a: 1 } });
const sent = await res.json();
const text = await (await fetch(`+js(echo.URL+"/plain")+`)).text();
const notJSON = await (await fetch(`+js(echo.URL+"/plain")+`)).json().then(() => "", (e) => e.message);
const down = await fetch("http://127.0.0.1:1/").then(() => "", (e) => e.message);
const metadata = await fetch("http://169.254.169.254/computeMetadata/v1/", { headers: { "Metadata-Flavor": "Google" } }).then(() => "", (e) => e.message);
return { ok: res.ok, status: res.status, statusText: res.statusText, reply: res.headers.get("X-Reply"), raw: res.headers["x-reply"], sent, text, notJSON, down, metadata };`).(map[string]any)
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

	t.Run("CronAndTableWorkflowsFireAndKeepTheirCursorThroughAnEditAndARename", func(t *testing.T) {
		tick := e2eSave(t, api, tok, "tick", "return input.trigger + ' ' + (input.at ? 'at' : '');", map[string]any{"cron": "@every 1s"})
		sold := e2eSave(t, api, tok, "sold", `const names = (rs) => rs.map((r) => r.address).join(",");
for (const r of [...input.created, ...input.updated]) await archive.update("house", r.id, { sold: true });
return input.table + " created:" + names(input.created) + " updated:" + names(input.updated);`, map[string]any{"table": "house", "events": []string{"created", "updated"}})
		added := e2eSave(t, api, tok, "added", "return input.table + ':' + input.created.map((r) => r.email).join(',') + '|' + input.updated.length;", map[string]any{"table": "member", "events": []string{"created"}})
		if rs := waitRuns(t, "tick", 2); rs[0].Trigger != "cron" || string(rs[0].Result) != `"cron at"` || rs[0].Error != "" || !rs[0].At.After(rs[1].At) {
			t.Errorf("cron runs: %+v", rs)
		}
		run(t, tok, `await archive.create("house", { address: "Canal 1" }); await archive.create("house", { address: "Canal 2" }); return 0;`)
		if rs := waitRuns(t, "sold", 1); rs[0].Trigger != "table" || string(rs[0].Result) != `"house created:Canal 1,Canal 2 updated:"` || rs[0].Error != "" {
			t.Fatalf("table run: %+v", rs)
		}
		run(t, tok, `await archive.create("member", { email: "carol@example.com" }); return 0;`)
		if rs := waitRuns(t, "added", 1); string(rs[0].Result) != `"member:carol@example.com|0"` {
			t.Errorf("the created-only run: %+v", rs)
		}
		in.groups("carol@example.com", "hr")
		time.Sleep(3 * time.Second)
		if rs := runs(t, "sold"); len(rs) != 1 {
			t.Fatalf("the workflow's own updates ran it again: %+v", rs)
		}
		// An edit and a rename keep the cursor: Canal 1 and 2 do not come again.
		run(t, tok, `await archive.update("workflow", `+js(sold["id"])+`, { name: "sold-houses", source: `+js(fmt.Sprint(sold["source"])+"\n// renamed")+` }); return 0;`)
		time.Sleep(3 * time.Second)
		if rs := runs(t, "sold-houses"); len(rs) != 1 {
			t.Fatalf("an edit and a rename reran old rows: %+v", rs)
		}
		run(t, tok, `const [h] = await archive.list("house", { where: { address: "Canal 2" } }); await archive.update("house", h.id, { sold: false }); await archive.create("house", { address: "Canal 3" }); return 0;`)
		if rs := waitRuns(t, "sold-houses", 2); string(rs[0].Result) != `"house created:Canal 3 updated:Canal 2"` {
			t.Errorf("an update by someone else and a new row: %+v", rs)
		}
		if rs := runs(t, "added"); len(rs) != 1 {
			t.Errorf("the created-only workflow ran for an update: %+v", rs)
		}
		// Changing the table it watches starts it from now; deleting it takes its runs, schedule and cursor.
		run(t, tok, `await archive.update("workflow", `+js(added["id"])+`, { on: { table: "house", events: ["created"] } }); return 0;`)
		if c := in.psql("archive", "SELECT cursor_id = 'ffffffff-ffff-ffff-ffff-ffffffffffff' FROM archive.workflow WHERE name = 'added'"); c != "t" {
			t.Errorf("the cursor after a new table: %s", c)
		}
		run(t, tok, `for (const id of `+js([]any{tick["id"], sold["id"], added["id"]})+`) await archive.delete("workflow", id); return 0;`)
		if n := in.psql("archive", "SELECT (SELECT count(*) FROM archive.workflow WHERE name IN ('tick', 'sold-houses', 'added')) || ',' || (SELECT count(*) FROM archive.workflow_run r WHERE r.workflow IN ('"+fmt.Sprint(tick["id"])+"', '"+fmt.Sprint(sold["id"])+"'))"); n != "0,0" {
			t.Errorf("left after deleting: %s", n)
		}
		if n := in.psql("archive", "SELECT count(*) FROM archive.workflow WHERE next_at IS NOT NULL AND trigger NOT LIKE 'cron %'"); n != "0" {
			t.Errorf("%s workflows still due", n)
		}
	})

	t.Run("WebhookPostRunsTheWorkflowAndReturnsItsResult", func(t *testing.T) {
		hook := e2eSave(t, api, tok, "hook", "return { got: input.body, method: input.method, x: input.query.x ?? null, type: input.headers['content-type'] ?? null, trigger: input.trigger };", map[string]any{"webhook": true})
		url := fmt.Sprint(hook["url"])
		if url != fmt.Sprint(saved["hook"]["url"]) {
			t.Errorf("the webhook URL changed: %s, was %s", url, saved["hook"]["url"])
		}
		if st, body := e2ePost(t, url+"?x=7", "application/json", `{"a": 1}`); st != 200 || body != `{"got":{"a":1},"method":"POST","x":"7","type":"application/json","trigger":"webhook"}` {
			t.Errorf("json webhook: %d %s", st, body)
		}
		if st, body := e2ePost(t, url, "text/plain", "just text"); st != 200 || body != `{"got":"just text","method":"POST","x":null,"type":"text/plain","trigger":"webhook"}` {
			t.Errorf("text webhook: %d %s", st, body)
		}
		if st, body := e2ePost(t, fmt.Sprint(saved["boom"]["url"]), "application/json", `[1]`); st != 500 || !strings.Contains(body, `"error":"bad input: [1]`) {
			t.Errorf("failing webhook: %d %s", st, body)
		}
		if st, _ := e2ePost(t, srv.url+"/hooks/run/"+strings.Repeat("x", 32), "application/json", "{}"); st != 404 {
			t.Errorf("unknown webhook token: %d", st)
		}
		if resp, err := http.Get(url); err != nil || resp.StatusCode != 405 {
			t.Errorf("GET on a webhook: %v %v", resp, err)
		}
		// Turned into a cron workflow, it has no URL; turned back, the same one.
		cron := e2eSave(t, api, tok, "hook", "return 1", map[string]any{"cron": "@daily"})
		back := e2eSave(t, api, tok, "hook", fmt.Sprint(hook["source"]), map[string]any{"webhook": true})
		if cron["url"] != nil || back["url"] != url {
			t.Errorf("the webhook URL through a change of on: %v %v", cron["url"], back["url"])
		}
	})

	t.Run("HTTPAndTheCLIWriteWorkflowsAsRows", func(t *testing.T) {
		var w row
		st, _ := call(t, "POST", api+"/workflow", tok, map[string]any{"name": "http", "source": "return 'http';", "on": map[string]any{"webhook": true}}, &w)
		wantStatus(t, st, 201, "create over HTTP")
		var bad struct {
			Errors []FieldError `json:"errors"`
		}
		st, _ = call(t, "PATCH", api+"/workflow/"+fmt.Sprint(w["id"]), tok, map[string]any{"source": "return (;"}, &bad)
		if wantStatus(t, st, 422, "a bad patch"); len(bad.Errors) != 1 || bad.Errors[0].Path != "source" || bad.Errors[0].Line != 1 || bad.Errors[0].Column != 9 {
			t.Errorf("a bad patch: %+v", bad)
		}
		var ws []row
		st, _ = call(t, "GET", api+"/workflow", tok, nil, &ws)
		if wantStatus(t, st, 200, "list"); len(ws) != 5 {
			t.Errorf("alice's workflows over HTTP: %v", ws)
		}
		st, _ = call(t, "GET", api+"/workflow", bobTok, nil, &ws)
		if wantStatus(t, st, 200, "bob's list"); len(ws) != 0 {
			t.Errorf("bob's workflows over HTTP: %v", ws)
		}
		st, _ = call(t, "DELETE", api+"/workflow/"+fmt.Sprint(w["id"]), tok, nil, nil)
		wantStatus(t, st, 204, "delete over HTTP")
		st, _ = call(t, "DELETE", api+"/workflow/"+fmt.Sprint(w["id"]), tok, nil, nil)
		wantStatus(t, st, 404, "delete again")

		made := decode[row](t, in.must("create", "workflow", `{"name": "cli", "source": "return 'cli';"}`))
		if made["name"] != "cli" || made["on"] != nil {
			t.Errorf("archive create workflow: %v", made)
		}
		if out := in.fails("set", "workflow", fmt.Sprint(made["id"]), `{"on": {"cron": "every day"}}`); !strings.Contains(out, "on.cron") {
			t.Errorf("archive set workflow with a bad cron: %s", out)
		}
		if out := in.fails("create", "workflow_run", `{"workflow": "`+fmt.Sprint(made["id"])+`", "trigger": "run"}`); !strings.Contains(out, "read only") {
			t.Errorf("archive create workflow_run: %s", out)
		}
		in.must("delete", "workflow", fmt.Sprint(made["id"]))
		if out := in.must("list", "workflow", "name=cli"); strings.TrimSpace(out) != "[]" {
			t.Errorf("after archive delete workflow: %s", out)
		}
	})

	t.Run("TheSDKDocsAreTheNewLoop", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		defer cancel()
		cl := mcp.NewClient(&mcp.Implementation{Name: "e2e", Version: "1"}, nil)
		sess, err := cl.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: srv.url + "/mcp", HTTPClient: &http.Client{Transport: e2eBearer(tok)}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer sess.Close()
		tools, err := sess.ListTools(ctx, nil)
		if err != nil || len(tools.Tools) != 1 {
			t.Fatalf("tools: %v %v", tools, err)
		}
		docs := tools.Tools[0].Description
		for _, want := range []string{`archive.create("workflow", { name, source, on })`, "run(name: string, input?: unknown): Promise<RunResult>;", "interface Workflow extends Base", "on?: WorkflowOn | null;", "interface WorkflowRun extends Base"} {
			if !strings.Contains(docs, want) {
				t.Errorf("the run tool's docs lack %q", want)
			}
		}
		for _, gone := range []string{"deploy(", "archive.test", "archive.start", "archive.runs", "workflows():"} {
			if strings.Contains(docs, gone) {
				t.Errorf("the run tool's docs still have %q", gone)
			}
		}
		res, err := sess.CallTool(ctx, &mcp.CallToolParams{Name: "run", Arguments: map[string]any{"code": `return (await archive.run("manual", { address: "MCP 1" })).result;`}})
		if err != nil || res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, `"result":"made"`) {
			t.Errorf("archive.run over MCP: %v %+v", err, res)
		}
	})
}

// A workspace's workflows from before they were rows keep their names, webhook URLs, cursors and schedules,
// and fire as before.
func TestWorkflowsFromBeforeRowsKeepTheirURLsCursorsAndSchedules(t *testing.T) {
	t.Parallel()
	in := newInst(t)
	in.must("user", "add", "alice@example.com")
	in.push(map[string]string{"house.sql": e2eHouse})
	user := in.psql("archive", "SELECT id FROM archive.\"user\" WHERE email = 'alice@example.com'")
	role := in.role("alice@example.com")
	in.psql("archive", `DROP POLICY mine ON archive.workflow_run; DROP POLICY mine ON archive.workflow;
ALTER TABLE archive.workflow DISABLE ROW LEVEL SECURITY; ALTER TABLE archive.workflow_run DISABLE ROW LEVEL SECURITY;
DROP TRIGGER workflow_saved ON archive.workflow; DROP FUNCTION archive.workflow_saved(); DROP FUNCTION archive.mine(uuid);
REVOKE ALL ON archive.workflow, archive.workflow_run FROM "`+strings.Split(role, "_u")[0]+`_viewer";
ALTER TABLE archive.workflow DROP COLUMN member, DROP COLUMN "on";
COMMENT ON TABLE archive.workflow IS NULL; COMMENT ON TABLE archive.workflow_run IS NULL;
INSERT INTO archive.workflow (user_id, role, name, source, trigger, hook, next_at) VALUES
  ('`+user+`', '`+role+`', 'tick.ts', E'// on: cron @every 1s\nreturn "tick";', 'cron @every 1s', 'tickhook000000000000000000000000', now()),
  ('`+user+`', '`+role+`', 'nightly.ts', E'// on: cron CRON_TZ=Europe/Amsterdam 0 9 * * *\nreturn 1;', 'cron CRON_TZ=Europe/Amsterdam 0 9 * * *', 'nightlyhook0000000000000000000000', '2099-01-01T08:00:00Z'),
  ('`+user+`', '`+role+`', 'hook.ts', E'// on: webhook\nreturn input.body;', 'webhook', 'oldhooksecret000000000000000000000', NULL),
  ('`+user+`', '`+role+`', 'manual.ts', 'return 7;', '', 'manualhook0000000000000000000000', NULL);
INSERT INTO public.house (address, created_at, updated_at) VALUES ('Seen 1', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
INSERT INTO archive.workflow (user_id, role, name, source, trigger, hook, next_at, cursor_at, cursor_id) VALUES
  ('`+user+`', '`+role+`', 'sold.ts', E'// on: house\nreturn input.rows.map((r) => r.address).join(",");', 'house', 'soldhook000000000000000000000000', NULL,
   '2026-01-01T00:00:01Z', 'ffffffff-ffff-ffff-ffff-ffffffffffff');
INSERT INTO archive.revision (hash) VALUES ('before workflow rows')`)
	srv := in.serve()
	api := srv.url + "/api"
	tok := decode[Token](t, in.must("token", "create")).Secret
	got := in.psql("archive", `SELECT string_agg(name || '|' || coalesce("on"::text, 'null') || '|' || source || '|' || trigger || '|' || hook || '|' || coalesce(cursor_at::text, '') || '|' || coalesce(next_at::text, '') || '|' || (member IS NOT NULL), E'\n' ORDER BY name) FROM archive.workflow WHERE name <> 'tick.ts'`)
	want := `hook.ts|{"webhook": true}|return input.body;|webhook|oldhooksecret000000000000000000000|||true
manual.ts|null|return 7;||manualhook0000000000000000000000|||true
nightly.ts|{"tz": "Europe/Amsterdam", "cron": "0 9 * * *"}|return 1;|cron CRON_TZ=Europe/Amsterdam 0 9 * * *|nightlyhook0000000000000000000000||2099-01-01 08:00:00+00|true
sold.ts|{"table": "house", "events": ["created", "updated"]}|return input.rows.map((r) => r.address).join(",");|house|soldhook000000000000000000000000|2026-01-01 00:00:01+00||true`
	if got != want {
		t.Errorf("the upgraded workflows:\n%s\nwant\n%s", got, want)
	}
	if rs := e2eWorkflowRuns(t, api, tok, "tick.ts"); len(rs) == 0 {
		e2eWait(t, "the old cron workflow to run", func() bool { return len(e2eWorkflowRuns(t, api, tok, "tick.ts")) > 0 })
	}
	if rs := e2eWorkflowRuns(t, api, tok, "tick.ts"); rs[0].Trigger != "cron" || string(rs[0].Result) != `"tick"` {
		t.Errorf("the old cron workflow's run: %+v", rs)
	}
	if st, body := e2ePost(t, srv.url+"/hooks/run/"+in.psql("archive", "SELECT id FROM archive.workspace ORDER BY created_at LIMIT 1")+"/oldhooksecret000000000000000000000", "application/json", `{"a": 1}`); st != 200 || body != `{"a":1}` {
		t.Errorf("the old webhook URL: %d %s", st, body)
	}
	call(t, "POST", api+"/house", tok, map[string]any{"address": "New 1"}, nil)
	e2eWait(t, "the old table workflow to run", func() bool { return len(e2eWorkflowRuns(t, api, tok, "sold.ts")) > 0 })
	if rs := e2eWorkflowRuns(t, api, tok, "sold.ts"); string(rs[0].Result) != `"New 1"` {
		t.Errorf("the old table workflow saw %s, want only New 1 past its cursor", rs[0].Result)
	}
	ws := decode[[]row](t, in.must("list", "workflow"))
	names := []string{}
	for _, w := range ws {
		names = append(names, fmt.Sprint(w["name"]))
	}
	if strings.Join(names, ",") != "tick.ts,nightly.ts,hook.ts,manual.ts,sold.ts" {
		t.Errorf("the workflows alice sees: %v", names)
	}
}
