package archive

import (
	"bytes"
	"encoding/json"
	"image"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// A thumbnail answers from the moment its file arrives. Until the archive has made it, a photo a
// browser shows stands in, its location taken out, and anything else is asked for again shortly;
// only a file that is done and has none is not found.
func TestAThumbnailAnswersBeforeItIsMade(t *testing.T) {
	// the model reads the photo and hears the memo only when the test lets it, so both wait
	release := make(chan struct{})
	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		if strings.HasSuffix(r.URL.Path, "/audio/transcriptions") {
			json.NewEncoder(w).Encode(map[string]string{"text": "a voice memo"})
			return
		}
		json.NewEncoder(w).Encode(e2eSure("a photo"))
	}))
	defer llm.Close()
	defer close(release)
	in := newInst(t, "ARCHIVE_ADMIN_EMAIL=alice@example.com", "ARCHIVE_LLM_URL="+llm.URL+"/v1", "ARCHIVE_VISION_MODEL=vision-fake", "ARCHIVE_WHISPER_MODEL=whisper-fake")
	srv := in.serve()
	api := srv.url + "/api"
	tok := decode[Token](t, in.must("token", "create", "-full")).Secret

	photo := e2eJPEG(t)
	st, p := e2eUpload(t, api+"/files", tok, "photo.jpg", photo)
	wantStatus(t, st, 201, "upload a photo")
	wav := append([]byte("RIFF\x24\x00\x00\x00WAVEfmt "), make([]byte, 64)...)
	st, m := e2eUpload(t, api+"/files", tok, "memo.wav", wav)
	wantStatus(t, st, 201, "upload a memo")

	resp, body := e2eGet(t, api+"/files/"+p.ID.String()+"/thumbnail", tok)
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "image/jpeg" || resp.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("the photo's thumbnail before it is made: %d %v", resp.StatusCode, resp.Header)
	}
	if bytes.Contains(body, e2eLatitude) || len(body) != len(photo) {
		t.Errorf("the photo stands in with its location: %d bytes of %d", len(body), len(photo))
	}
	resp, _ = e2eGet(t, api+"/files/"+m.ID.String()+"/thumbnail", tok)
	if resp.StatusCode != http.StatusAccepted || resp.Header.Get("Retry-After") == "" {
		t.Errorf("the memo's thumbnail before it is done: %d %v", resp.StatusCode, resp.Header)
	}

	release <- struct{}{}
	release <- struct{}{}
	for _, f := range []File{p, m} {
		if f := e2eWaitFile(t, api, tok, f.ID.String()); f.Status != "done" {
			t.Fatalf("%s: %s %s", f.Name, f.Status, f.Error)
		}
	}
	resp, body = e2eGet(t, api+"/files/"+p.ID.String()+"/thumbnail", tok)
	if c, _, err := image.DecodeConfig(bytes.NewReader(body)); resp.StatusCode != 200 || err != nil || c.Width != 320 {
		t.Errorf("the photo's thumbnail once made: %d %v %+v", resp.StatusCode, err, c)
	}
	resp, _ = e2eGet(t, api+"/files/"+m.ID.String()+"/thumbnail", tok)
	wantStatus(t, resp.StatusCode, 404, "the memo's thumbnail once it is done")
}
