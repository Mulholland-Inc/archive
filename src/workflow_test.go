package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
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

// A workspace's workflows from before they were rows keep their names, webhook URLs, cursors and schedules,
// and fire as before.
func TestWorkflowsFromBeforeRowsKeepTheirURLsCursorsAndSchedules(t *testing.T) {
	t.Parallel()
	in := newInst(t)
	in.must("user", "add", "alice@example.com")
	in.push(map[string]string{"house.sql": e2eHouse})
	user := in.psql("archive", "SELECT id FROM archive.\"user\" WHERE email = 'alice@example.com'")
	role := in.role("alice@example.com")
	in.psql("archive", `DROP TABLE archive.connection; DROP POLICY mine ON archive.workflow_run; DROP POLICY mine ON archive.workflow;
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
