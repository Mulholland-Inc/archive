package archive

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
)

// Sources that run a program or read the server's own disk are the instance admin's alone.
func TestOnlyInstanceAdminsAddSourcesThatRunProgramsOrReadTheServersDisk(t *testing.T) {
	in := newInst(t, "ARCHIVE_ADMIN_EMAIL=alice@example.com")
	srv := in.serve()
	api := srv.url + "/api"

	in.must("user", "add", "bob@example.com", "-name", "Bob")
	in.must("spec", "push", in.writeSpec())
	in.must("space", "add", "family")
	in.must("space", "grant", "family", "bob@example.com", "viewer")
	bobTok := decode[Token](t, in.must("-as", "bob@example.com", "token", "create")).Secret
	narrowTok := decode[Token](t, in.must("token", "create", "-name", "Narrow", "-space", "family:editor")).Secret
	aliceTok := decode[Token](t, in.must("token", "create", "-name", "Claude", "-full")).Secret

	pwned := filepath.Join(in.dir, "pwned")
	command := map[string]any{"kind": "external", "name": "x", "config": map[string]any{"command": []string{"sh", "-c", "touch " + pwned}}}
	folder := map[string]any{"kind": "folder", "name": "etc", "type": "invoice", "config": map[string]any{"path": "/etc"}}

	t.Run("OthersCannotAddACommandSource", func(t *testing.T) {
		for who, tok := range map[string]string{"a user": bobTok, "the admin's token without admin rights": narrowTok} {
			st, _ := call(t, "POST", api+"/sources", tok, command, nil)
			wantStatus(t, st, 403, who+" adds an external source")
			if _, err := os.Stat(pwned); err == nil {
				t.Fatalf("%s ran a program on the server", who)
			}
		}
	})

	t.Run("OthersCannotAddAFolderSource", func(t *testing.T) {
		st, _ := call(t, "POST", api+"/sources", bobTok, folder, nil)
		wantStatus(t, st, 403, "a user adds a folder source")
		if n := in.on(t).psql("archive", "SELECT count(*) FROM raw"); n != "0" {
			t.Fatalf("a user read %s files off the server's disk", n)
		}
	})

	t.Run("OthersCannotIntakeAFolderThroughMCP", func(t *testing.T) {
		out, isErr := mcpCall(t, srv.url, bobTok, "intake", map[string]any{"dir": "/etc", "type": "invoice"})
		if !isErr || !strings.Contains(out, "instance admin") {
			t.Fatalf("mcp intake of /etc as a user: %v %s", isErr, out)
		}
	})

	t.Run("TheInstanceAdminStillCan", func(t *testing.T) {
		st, _ := call(t, "POST", api+"/sources", aliceTok, folder, nil)
		wantStatus(t, st, 201, "the admin adds a folder source")
	})
}

// Changing the ontology rewrites records in every space, so it needs the instance admin, not
// merely admin of some space; every user owns their home space.
func TestOnlyInstanceAdminsChangeTheOntology(t *testing.T) {
	in := newInst(t, "ARCHIVE_ADMIN_EMAIL=alice@example.com")
	srv := in.serve()
	api := srv.url + "/api"

	in.must("user", "add", "bob@example.com", "-name", "Bob")
	in.must("spec", "push", in.writeSpec())
	aliceTok := decode[Token](t, in.must("token", "create", "-name", "Claude", "-full")).Secret
	bobTok := decode[Token](t, in.must("-as", "bob@example.com", "token", "create")).Secret
	var fam e2eRecord
	st, _ := call(t, "POST", api+"/contact", aliceTok, map[string]any{"name": "Alice", "phone": "+31612345678"}, &fam)
	wantStatus(t, st, 201, "alice creates a contact")

	drop := map[string]any{"ops": []Op{{Op: "remove_field", Type: "contact", Field: "phone"}}}
	var c Change
	st, _ = call(t, "POST", api+"/changes", bobTok, drop, &c)
	wantStatus(t, st, 201, "a user proposes a change")
	st, _ = call(t, "POST", api+"/changes/"+c.ID.String()+"/apply", bobTok, nil, nil)
	if st == 200 {
		t.Fatalf("a user applied a change that removes a field from everyone's records")
	}
	if got := in.psql("archive", "SELECT data->>'phone' FROM records WHERE type = 'contact'"); got != "+31612345678" {
		t.Fatalf("alice's contact lost its phone: %q", got)
	}

	st, _ = call(t, "POST", api+"/changes/"+c.ID.String()+"/apply", aliceTok, nil, nil)
	wantStatus(t, st, 200, "the instance admin applies it")
}

// e2eSure is a chat completion whose every token the model was all but certain of, as a server that
// returns logprobs reports it.
func e2eSure(content string) map[string]any {
	var toks []map[string]any
	for _, r := range content {
		toks = append(toks, map[string]any{"token": string(r), "logprob": -0.001})
	}
	return map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": content}, "logprobs": map[string]any{"content": toks}}}}
}

func e2eLLM(t *testing.T, reply func() map[string]any) string {
	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(reply()) }))
	t.Cleanup(llm.Close)
	return llm.URL + "/v1"
}

func e2eIntakeOne(t *testing.T, llm, typ, text string) (*e2eInst, IntakeResult) {
	in := newInst(t, "ARCHIVE_LOCALE=nl-NL", "ARCHIVE_LLM_URL="+llm, "ARCHIVE_LLM_MODEL=fake")
	in.must("user", "add", "ann@example.com")
	in.must("spec", "push", in.writeSpec())
	inbox := filepath.Join(in.dir, "inbox")
	os.MkdirAll(inbox, 0o755)
	os.WriteFile(filepath.Join(inbox, "doc.txt"), []byte(text), 0o644)
	res := decode[[]IntakeResult](t, in.must("intake", inbox, "-type", typ))
	if len(res) != 1 {
		t.Fatalf("intake: %+v", res)
	}
	return in, res[0]
}

func e2eRoutes(r IntakeResult) string {
	var out []string
	for _, f := range r.Fields {
		out = append(out, f.Path+"="+f.Route)
	}
	return strings.Join(out, ",")
}

// A model server that gives no token probabilities (Ollama, the default) cannot tell a sure value
// from one it gets wrong the same way three times, so with no labels either, a person reviews it.
func TestIntakeWithoutLogprobsOrLabelsWaitsForReview(t *testing.T) {
	llm := e2eLLM(t, func() map[string]any {
		return map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": `{"number":"INV-7","date":"03-04-2025"}`}}}}
	})
	in, res := e2eIntakeOne(t, llm, "invoice", "Factuur INV-7, datum 03-04-2025")
	if res.RecordID != uuid.Nil {
		t.Fatalf("saved a record on agreement alone: %s", e2eRoutes(res))
	}
	if got := e2eRoutes(res); !strings.Contains(got, "date=review,number=review") {
		t.Errorf("routes: %s", got)
	}
	if got := decode[[]e2eRecord](t, in.must("list", "invoice")); len(got) != 0 {
		t.Errorf("invoices saved: %v", got)
	}
	if c := decode[map[string]int](t, in.must("review", "counts")); c["review"] != 1 || c["accepted"] != 0 {
		t.Errorf("review counts: %v", c)
	}
}

// What intake saves on its own passes the same checks as any write: a sure model that names an
// employer that does not exist leaves the person for review instead of saving a dangling reference.
func TestIntakeSavesOnlyWhatAPersonCouldSave(t *testing.T) {
	ghost := "01a16a5f-0000-7000-8000-000000000000"
	llm := e2eLLM(t, func() map[string]any { return e2eSure(`{"name":"Ann","employer":"` + ghost + `"}`) })
	in, res := e2eIntakeOne(t, llm, "person", "Ann works at a company")
	if res.RecordID != uuid.Nil {
		t.Fatalf("saved a person whose employer does not exist: %s", e2eRoutes(res))
	}
	if got := e2eRoutes(res); got != "employer=review,name=review,ssn=drop" {
		t.Errorf("routes: %s", got)
	}
	if got := decode[[]e2eRecord](t, in.must("list", "person")); len(got) != 0 {
		t.Errorf("people saved: %v", got)
	}
}

// One field the model gets wrong does not leave the others as it wrote them.
func TestIntakeNormalizesGoodFieldsWhenAnotherIsBad(t *testing.T) {
	llm := e2eLLM(t, func() map[string]any { return e2eSure(`{"number":"INV-8","date":"03-04-2025","bsn":"123"}`) })
	in, res := e2eIntakeOne(t, llm, "invoice", "Factuur INV-8, datum 03-04-2025, BSN 123")
	if res.RecordID == uuid.Nil {
		t.Fatalf("nothing saved: %s", e2eRoutes(res))
	}
	got := decode[e2eRecord](t, in.must("get", "invoice", res.RecordID.String()))
	if got.Data["date"] != "2025-04-03" || got.Data["bsn"] != nil {
		t.Errorf("saved %v, want the date normalized and no bsn", got.Data)
	}
}

// Two people changing different fields of one object at the same time both keep their change.
func TestConcurrentPatchesOfDifferentFieldsBothLand(t *testing.T) {
	in := newInst(t, "ARCHIVE_ADMIN_EMAIL=alice@example.com")
	srv := in.serve()
	api := srv.url + "/api"
	in.must("spec", "push", in.writeSpec())
	tok := decode[Token](t, in.must("token", "create", "-full")).Secret
	var c e2eRecord
	st, _ := call(t, "POST", api+"/contact", tok, map[string]any{"name": "Ann"}, &c)
	wantStatus(t, st, 201, "create")

	for round := range 30 {
		email, phone := fmt.Sprintf("ann%d@example.com", round), fmt.Sprintf("+3161234%04d", round)
		var wg sync.WaitGroup
		for _, p := range []map[string]any{{"email": email}, {"phone": phone}} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				b, _ := json.Marshal(p)
				req, _ := http.NewRequest("PATCH", api+"/contact/"+c.ID, strings.NewReader(string(b)))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+tok)
				if resp, err := http.DefaultClient.Do(req); err == nil {
					resp.Body.Close()
				}
			}()
		}
		wg.Wait()
		var got e2eRecord
		call(t, "GET", api+"/contact/"+c.ID, tok, nil, &got)
		if got.Data["email"] != email || got.Data["phone"] != phone {
			t.Fatalf("round %d: one edit wiped the other: %v", round, got.Data)
		}
	}
}

// A backup taken while the archive serves restores into an empty directory with everything a
// person would miss: records, secret values (the key travels with them), files and sign-in tokens.
func TestBackupWhileServingRestoresRecordsSecretsFilesAndTokens(t *testing.T) {
	in := newInst(t, "ARCHIVE_ADMIN_EMAIL=alice@example.com")
	srv := in.serve()
	api := srv.url + "/api"
	in.must("spec", "push", in.writeSpec())
	in.must("secret", "grant", "alice@example.com", "us_ssn")
	tok := decode[Token](t, in.must("token", "create", "-full")).Secret
	var p e2eRecord
	st, _ := call(t, "POST", api+"/person", tok, map[string]any{"name": "Ann", "ssn": "123-45-6789"}, &p)
	wantStatus(t, st, 201, "create")
	st, f := e2eUpload(t, api+"/files", tok, "note.txt", []byte("the letter from 1931"))
	wantStatus(t, st, 201, "upload")
	before := in.must("get", "person", p.ID, "-reveal")
	if !strings.Contains(before, "123456789") {
		t.Fatalf("reveal before backup: %s", before)
	}

	file := filepath.Join(t.TempDir(), "archive.tgz")
	in.must("backup", "-o", file)
	srv.stop()

	again := newInst(t, "ARCHIVE_ADMIN_EMAIL=alice@example.com")
	if out, errs, err := again.run("", "restore", file); err != nil {
		t.Fatalf("restore: %v\n%s%s", err, out, errs)
	}
	if errs := again.fails("restore", file); !strings.Contains(errs, "already holds an archive") {
		t.Errorf("restore over an archive: %s", errs)
	}
	srv2 := again.serve()
	if after := again.must("get", "person", p.ID, "-reveal"); after != before {
		t.Errorf("after restore:\n%s\nbefore:\n%s", after, before)
	}
	resp, body := e2eGet(t, srv2.url+"/api/files/"+f.ID.String()+"/content", tok)
	if resp.StatusCode != 200 || string(body) != "the letter from 1931" {
		t.Errorf("file after restore: %d %q", resp.StatusCode, body)
	}
}

// Every new binary is a new version to BackupBeforeUpgrade, and its snapshot carries the key.
func TestStartingANewBinaryKeepsASnapshotWithTheKey(t *testing.T) {
	in := newInst(t, "ARCHIVE_ADMIN_EMAIL=alice@example.com")
	in.must("user", "add", "alice@example.com")
	os.WriteFile(filepath.Join(in.dir, "version"), []byte(cliVersion+"+an-older-build\n"), 0o600)
	in.must("types")
	snaps, _ := filepath.Glob(filepath.Join(in.dir, "backups", "*.tar.gz"))
	if len(snaps) != 1 {
		t.Fatalf("snapshots: %v", snaps)
	}
	out, err := exec.Command("tar", "-tzf", snaps[0]).Output()
	if err != nil || !strings.Contains(string(out), "secret.key") || !strings.Contains(string(out), "pg/PG_VERSION") {
		t.Errorf("snapshot holds:\n%s", out)
	}
}
