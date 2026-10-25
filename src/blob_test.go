package archive

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// A folder that cannot hard-link, as a Cloud Storage bucket mounted on Cloud Run cannot, still
// takes a file: it is copied in whole instead.
func TestAFolderWithoutHardLinksStillTakesAFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "upload")
	if err := os.WriteFile(src, []byte("a photograph"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := blobLink
	blobLink = func(string, string) error { return &os.LinkError{Op: "link", Err: syscall.ENOSYS} }
	t.Cleanup(func() { blobLink = link })

	d := &dirBlobs{filepath.Join(dir, "blobs")}
	hash := "5ed56efeecccb1d1d106c22a9c98d930862f37c40cc704fbd712bdf5f6a93562"
	if err := d.put(context.Background(), hash, src, 12); err != nil {
		t.Fatalf("put: %v", err)
	}
	got, err := os.ReadFile(d.path(hash))
	if err != nil || string(got) != "a photograph" {
		t.Fatalf("stored %q, %v", got, err)
	}
	if _, err := os.Stat(src); err != nil {
		t.Errorf("the upload itself is gone: %v", err)
	}
	if left, _ := filepath.Glob(d.path(hash) + ".*"); len(left) > 0 {
		t.Errorf("left behind: %v", left)
	}
}
