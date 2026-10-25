package archive

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A product can add a file restricted from the moment it is made, as an editor, with no moment in
// between when the rest of the space could read it. The same bytes under the same name are another
// file, perhaps one others see, so they are refused rather than shut or shared, and so is a plain
// upload of a restricted file's bytes and name by someone who cannot see it.
func TestAnEditorAddsAFileRestrictedFromTheMomentItIsMade(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "toymodule")
	if out, err := exec.Command("go", "build", "-o", bin, "./testdata/toymodule").CombinedOutput(); err != nil {
		t.Fatalf("build the toy module: %v\n%s", err, out)
	}
	in := newInst(t, "ARCHIVE_ADMIN_EMAIL=alice@example.com")
	in.bin = bin
	srv := in.serve()
	api := srv.url + "/api"
	in.must("user", "add", "bob@example.com", "-name", "Bob")
	in.must("user", "add", "carol@example.com", "-name", "Carol")
	in.must("space", "add", "family")
	in.must("space", "grant", "family", "bob@example.com", "editor")
	in.must("space", "grant", "family", "carol@example.com", "editor")
	alice := decode[Token](t, in.must("token", "create", "-full")).Secret
	bob := decode[Token](t, in.must("-as", "bob@example.com", "token", "create", "-full")).Secret
	carol := decode[Token](t, in.must("-as", "carol@example.com", "token", "create", "-full")).Secret

	diary := []byte("Bob's diary, for the admins until they open it")
	add := func(tok, name string, b []byte) (int, File) {
		t.Helper()
		req, _ := http.NewRequest("POST", api+"/toy/files?space=family&name="+name, bytes.NewReader(b))
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
	st, f := add(bob, "diary.txt", diary)
	wantStatus(t, st, 201, "an editor adds a restricted file")
	id := f.ID.String()
	if got := in.psql("archive", "SELECT restricted FROM records WHERE id = '"+id+"'"); !strings.Contains(got, "t") {
		t.Fatalf("the file's record is not restricted: %q", got)
	}

	t.Run("OnlyTheAdminsAndWhoeverAddedItSeeIt", func(t *testing.T) {
		for who, tok := range map[string]string{"alice": alice, "bob": bob} {
			if resp, body := e2eGet(t, api+"/files/"+id+"/content", tok); resp.StatusCode != 200 || !bytes.Equal(body, diary) {
				t.Errorf("%s reads the file: %d", who, resp.StatusCode)
			}
		}
		for _, part := range []string{"", "/content", "/text"} {
			resp, _ := e2eGet(t, api+"/files/"+id+part, carol)
			wantStatus(t, resp.StatusCode, 404, "carol reads the file"+part)
		}
		var list []File
		call(t, "GET", api+"/files", carol, nil, &list)
		if len(list) != 0 {
			t.Errorf("carol lists %+v", list)
		}
	})

	t.Run("TheSameBytesAndNameAreRefused", func(t *testing.T) {
		st, _ := add(bob, "diary.txt", diary)
		wantStatus(t, st, 409, "bob adds his restricted file again")
		st, _ = e2eUpload(t, api+"/files?space=family", carol, "diary.txt", diary)
		wantStatus(t, st, 409, "carol uploads the bytes and name of a file she cannot see")
		st, _ = e2eUpload(t, api+"/files?space=family", carol, "copy.txt", diary)
		wantStatus(t, st, 201, "carol uploads the same bytes under a name of her own")
		if n := in.psql("archive", "SELECT count(*) FROM files WHERE hash = '"+f.Hash+"'"); n != "2" {
			t.Errorf("%s file rows, want bob's and carol's copy", n)
		}
	})

	t.Run("AnAdminOpensIt", func(t *testing.T) {
		st, _ := call(t, "PUT", api+"/file/"+id+"/restricted", alice, map[string]any{"restricted": false}, nil)
		wantStatus(t, st, 200, "alice opens the file")
		resp, _ := e2eGet(t, api+"/files/"+id, carol)
		wantStatus(t, resp.StatusCode, 200, "carol reads the opened file")
	})
}
