package archive

import (
	"bytes"
	"image"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// An iPhone photo (HEIC) gets a JPEG display copy and a thumbnail that every browser shows, with
// the location stripped from what is displayed; the original stays as it came.
func TestAnIPhonePhotoGetsADisplayCopyEveryBrowserShows(t *testing.T) {
	vips, err := exec.LookPath("vips")
	if err != nil {
		t.Skip("vips is not installed here; the image ships it")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "in.jpg")
	os.WriteFile(src, e2eJPEG(t), 0o644)
	heic := filepath.Join(dir, "IMG_0001.heic")
	if out, err := exec.Command(vips, "copy", src, heic).CombinedOutput(); err != nil {
		t.Skipf("this vips cannot write HEIC: %s", out)
	}
	b, _ := os.ReadFile(heic)

	in := newInst(t, "ARCHIVE_ADMIN_EMAIL=alice@example.com")
	srv := in.serve()
	api := srv.url + "/api"
	tok := decode[Token](t, in.must("token", "create", "-full")).Secret
	st, f := e2eUpload(t, api+"/files", tok, "IMG_0001.heic", b)
	wantStatus(t, st, 201, "upload")
	f = e2eWaitFile(t, api, tok, f.ID.String())
	if f.MIME != "image/heic" || f.Metadata["display"] == nil || f.Metadata["thumbnail"] == nil || f.Metadata["width"] == nil {
		t.Fatalf("processed: %s %v (%s)", f.MIME, f.Metadata, f.Error)
	}
	for _, part := range []string{"/display", "/thumbnail"} {
		resp, body := e2eGet(t, api+"/files/"+f.ID.String()+part, tok)
		if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "image/jpeg" {
			t.Fatalf("%s: %d %s", part, resp.StatusCode, resp.Header.Get("Content-Type"))
		}
		if _, err := jpeg.Decode(bytes.NewReader(body)); err != nil {
			t.Errorf("%s is not a JPEG: %v", part, err)
		}
		if part == "/display" && (bytes.Contains(body, []byte("Exif")) || bytes.Contains(body, []byte("GPS"))) {
			t.Errorf("the display copy keeps its metadata")
		}
	}
	resp, body := e2eGet(t, api+"/files/"+f.ID.String()+"/content", tok)
	if resp.StatusCode != 200 || !bytes.Equal(body, b) {
		t.Errorf("the original changed: %d", resp.StatusCode)
	}

	// a JPEG needs no copy: display is the file itself
	st, j := e2eUpload(t, api+"/files", tok, "plain.jpg", e2eJPEG(t))
	wantStatus(t, st, 201, "upload a jpeg")
	j = e2eWaitFile(t, api, tok, j.ID.String())
	if j.Metadata["display"] != nil {
		t.Errorf("a JPEG got a display copy")
	}
	if resp, body := e2eGet(t, api+"/files/"+j.ID.String()+"/display", tok); resp.StatusCode != 200 {
		t.Errorf("display of a jpeg: %d", resp.StatusCode)
	} else if _, _, err := image.Decode(bytes.NewReader(body)); err != nil {
		t.Errorf("display of a jpeg: %v", err)
	}
}
