package archive

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

// A product's careful save checks what it read under the record's lock: many writers that all
// read the same note save at once, one of them wins and the rest are told it changed, never
// overwriting the winner without having seen it.
func TestPatchIfLetsOnlyOneOfManyWritersWhoReadTheSameRecordSave(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "toymodule")
	if out, err := exec.Command("go", "build", "-o", bin, "./testdata/toymodule").CombinedOutput(); err != nil {
		t.Fatalf("build the toy module: %v\n%s", err, out)
	}
	in := newInst(t, "ARCHIVE_ADMIN_EMAIL=alice@example.com")
	in.bin = bin
	srv := in.serve()
	api := srv.url + "/api"
	tok := decode[Token](t, in.must("token", "create", "-full")).Secret

	var n e2eRecord
	st, _ := call(t, "POST", api+"/note", tok, map[string]any{"text": "first"}, &n)
	wantStatus(t, st, 201, "create a note")
	var wg sync.WaitGroup
	codes := make([]int, 16)
	for i := range codes {
		wg.Go(func() {
			codes[i], _ = call(t, "PATCH", api+"/toy/note/"+n.ID, tok, map[string]any{"from": "FIRST", "to": fmt.Sprint("SECOND ", i)}, nil)
		})
	}
	wg.Wait()
	won := -1
	for i, c := range codes {
		switch {
		case c == 200 && won < 0:
			won = i
		case c != 409:
			t.Errorf("writer %d: %d", i, c)
		}
	}
	if won < 0 {
		t.Fatalf("no writer saved: %v", codes)
	}
	call(t, "GET", api+"/note/"+n.ID, tok, nil, &n)
	if n.Data["text"] != fmt.Sprint("SECOND ", won) {
		t.Errorf("the note says %v, the winner wrote SECOND %d", n.Data["text"], won)
	}
	st, _ = call(t, "PATCH", api+"/toy/note/"+n.ID, tok, map[string]any{"from": fmt.Sprint("SECOND ", won), "to": "THIRD"}, nil)
	wantStatus(t, st, 200, "a writer who read the note as it is saves")
}
