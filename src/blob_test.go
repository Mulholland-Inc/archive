package archive

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
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

// Two uploads of the same bytes at once, on a folder without hard links, each copy into a file
// of its own: neither cuts the other short, both succeed, and the blob is whole.
func TestTwoCopiesOfTheSameBlobAtOnceLeaveItWhole(t *testing.T) {
	link := blobLink
	blobLink = func(string, string) error { return &os.LinkError{Op: "link", Err: syscall.ENOSYS} }
	t.Cleanup(func() { blobLink = link })
	body := bytes.Repeat([]byte("a long letter home "), 200000)
	sum := sha256.Sum256(body)
	hash := hex.EncodeToString(sum[:])
	for round := range 5 {
		dir := t.TempDir()
		d := &dirBlobs{filepath.Join(dir, "blobs")}
		var wg sync.WaitGroup
		errs := make(chan error, 16)
		for i := range 16 {
			src := filepath.Join(dir, fmt.Sprintf("upload-%d", i))
			if err := os.WriteFile(src, body, 0o600); err != nil {
				t.Fatal(err)
			}
			wg.Go(func() { errs <- d.put(context.Background(), hash, src, int64(len(body))) })
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Errorf("round %d: put: %v", round, err)
			}
		}
		got, err := os.ReadFile(d.path(hash))
		if err != nil || !bytes.Equal(got, body) {
			t.Fatalf("round %d: the blob is %d bytes of %d, %v", round, len(got), len(body), err)
		}
		if left, _ := filepath.Glob(filepath.Join(filepath.Dir(d.path(hash)), "*")); len(left) != 1 {
			t.Errorf("round %d: left behind: %v", round, left)
		}
	}
}
