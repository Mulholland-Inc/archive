package archive

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

// An upload nobody has touched for a week is let go, finished or not; a newer one, and anything
// in the uploads directory that is not an upload, stays.
func TestTusSweepLetsGoOfAWeekOldUploadsOnly(t *testing.T) {
	a := &Archive{uploads: t.TempDir()}
	old, fresh := uuid.NewString(), uuid.NewString()
	for _, name := range []string{old, old + ".json", fresh, fresh + ".json", "post-123"} {
		os.WriteFile(filepath.Join(a.uploads, name), []byte("x"), 0o600)
	}
	then := time.Now().Add(-tusKept - time.Hour)
	for _, name := range []string{old, old + ".json", "post-123"} {
		os.Chtimes(filepath.Join(a.uploads, name), then, then)
	}
	a.tusSweep()
	for name, want := range map[string]bool{old: false, old + ".json": false, fresh: true, fresh + ".json": true, "post-123": true} {
		if _, err := os.Stat(filepath.Join(a.uploads, name)); (err == nil) != want {
			t.Errorf("%s: kept %v, want %v", name, err == nil, want)
		}
	}
}
