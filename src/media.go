package archive

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/ledongthuc/pdf"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
	"golang.org/x/net/html"
)

const (
	mediaMax     = 64 << 20
	mediaThumb   = 320
	mediaWorkers = 2
	mediaOCR     = "Transcribe all text in this image exactly as written, keeping line breaks. Reply with the text only, or nothing if there is no text."
)

var mediaVision = map[string]bool{"image/jpeg": true, "image/png": true, "image/gif": true, "image/webp": true}

func mediaSniff(name string, b []byte) string {
	if len(b) >= 12 && string(b[4:8]) == "ftyp" {
		switch string(b[8:12]) {
		case "heic", "heix", "heim", "heis", "mif1", "msf1":
			return "image/heic"
		case "M4A ":
			return "audio/mp4"
		case "qt  ":
			return "video/quicktime"
		}
	}
	if strings.EqualFold(filepath.Ext(name), ".eml") {
		return "message/rfc822"
	}
	return folderMIME(name, b)
}

func (a *Archive) fileLoop(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	swept := time.Time{}
	for {
		for a.fileNext(ctx) {
		}
		if time.Since(swept) > time.Hour {
			a.tusSweep()
			swept = time.Now()
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		case <-a.wake:
		}
	}
}

func (a *Archive) fileNext(ctx context.Context) bool {
	var id uuid.UUID
	err := a.st.System(ctx, func(ctx context.Context) error {
		return a.st.Q(ctx).QueryRow(ctx, `UPDATE files SET status = 'processing' WHERE id = (
			SELECT id FROM files WHERE status = 'pending' ORDER BY id LIMIT 1 FOR UPDATE SKIP LOCKED) RETURNING id`).Scan(&id)
	})
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) && ctx.Err() == nil {
			fmt.Fprintln(os.Stderr, "archive: files:", err)
		}
		return false
	}
	a.fileProcess(ctx, id)
	return true
}

func (a *Archive) fileProcess(ctx context.Context, id uuid.UUID) {
	var f *File
	err := a.st.System(ctx, func(ctx context.Context) (err error) {
		f, err = storeOne[File](ctx, a.st, `SELECT `+fileCols+` FROM files WHERE id = $1`, id)
		return err
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "archive: file %s: %v\n", id, err)
		return
	}
	meta, text, perr := a.media(ctx, f)
	err = a.st.System(ctx, func(ctx context.Context) error {
		_, err := a.st.Q(ctx).Exec(ctx, `UPDATE files SET metadata = $2, text = $3 WHERE id = $1`, id, meta, strings.ToValidUTF8(strings.ReplaceAll(text, "\x00", ""), "�"))
		return err
	})
	if err == nil && perr == nil && f.Intake != "" {
		if _, ierr := a.IntakeFile(WithActor(ctx, &Actor{Role: f.Owner}), id, f.Intake); ierr != nil {
			perr = fmt.Errorf("intake: %w", ierr)
		}
	}
	status, msg := "done", ""
	if perr = errors.Join(err, perr); perr != nil {
		status, msg = "error", perr.Error()
	}
	err = a.st.System(context.WithoutCancel(ctx), func(ctx context.Context) error {
		_, err := a.st.Q(ctx).Exec(ctx, `UPDATE files SET status = $2, error = $3, intake = '' WHERE id = $1`, id, status, msg)
		return err
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "archive: file %s: %v\n", id, err)
	}
}

func (a *Archive) media(ctx context.Context, f *File) (meta map[string]any, text string, err error) {
	meta = map[string]any{}
	defer func() {
		if _, exists := meta["extraction"]; exists {
			return
		}
		status, reason := "ready", ""
		if err != nil {
			status, reason = "invalid", "The file's contents could not be read."
			if strings.Contains(strings.ToLower(err.Error()), "password") || strings.Contains(strings.ToLower(err.Error()), "encrypt") {
				status, reason = "encrypted", "The file is encrypted; an unlocked copy is needed to read it."
			}
		} else if strings.TrimSpace(text) == "" {
			status, reason = "empty", "No readable text was extracted."
		}
		meta["extraction"] = extractionStatus(status, f.MIME, reason)
	}()
	if f.Size > mediaMax {
		meta["extraction"] = extractionStatus("too_large", f.MIME, "The file is saved, but exceeds the 64 MiB automatic extraction limit.")
		return meta, "", nil
	}
	rc, err := a.blobs.open(ctx, f.Hash, f.Size)
	if err != nil {
		return meta, "", err
	}
	b, err := io.ReadAll(io.LimitReader(rc, mediaMax+1))
	rc.Close()
	if err != nil {
		return meta, "", err
	}
	if len(b) > mediaMax {
		meta["extraction"] = extractionStatus("too_large", f.MIME, "The file exceeds the automatic extraction limit.")
		return meta, "", nil
	}
	if officeFormat(f.Name) != "" {
		return mediaOffice(f.Name, b)
	}
	cfg := a.intake.cfg
	top, _, _ := strings.Cut(f.MIME, "/")
	switch {
	case f.MIME == "application/pdf":
		text, pages, err := mediaPDF(b)
		meta["pages"] = pages
		return meta, text, err
	case f.MIME == "text/html":
		return meta, mediaHTML(b), nil
	case f.MIME == "message/rfc822":
		headers, body, atts := imapParse(b)
		meta["headers"] = headers
		return meta, mailPayload(headers, body, atts, nil)["text"].(string), nil
	case top == "text" || f.MIME == "application/json":
		return meta, string(b), nil
	case top == "image":
		mediaEXIF(b, meta)
		pic, picMIME := b, f.MIME
		if !mediaVision[f.MIME] {
			if d, err := a.mediaDisplay(ctx, b); err == nil {
				if t, err := a.mediaPut(ctx, d); err == nil {
					meta["display"] = t
					pic, picMIME = d, "image/jpeg"
				}
			}
		}
		if c, _, err := image.DecodeConfig(bytes.NewReader(pic)); err == nil {
			meta["width"], meta["height"] = c.Width, c.Height
			if d, ok := meta["display"].(map[string]any); ok {
				d["width"], d["height"] = c.Width, c.Height
			}
		}
		if t, err := a.mediaThumbnail(ctx, pic); err == nil && t != nil {
			meta["thumbnail"] = t
		}
		if cfg.Vision != "" && mediaVision[picMIME] {
			text, err := a.mediaOCR(ctx, picMIME, pic)
			return meta, text, err
		}
	case top == "audio" || top == "video":
		if cfg.Whisper != "" {
			text, err := a.mediaTranscribe(ctx, f.Name, b)
			return meta, text, err
		}
		meta["extraction"] = extractionStatus("unsupported", f.MIME, "Transcription is not configured; the original is saved.")
	default:
		meta["extraction"] = extractionStatus("unsupported", f.MIME, "Text extraction is not available for this format; the original is saved.")
	}
	return meta, "", nil
}

func mediaPDF(b []byte) (text string, pages int, err error) {
	defer func() {
		if recover() != nil {
			err = errors.New("unreadable PDF")
		}
	}()
	r, err := pdf.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return "", 0, fmt.Errorf("pdf: %w", err)
	}
	parts := make([]string, r.NumPage())
	for i := range parts {
		if p := r.Page(i + 1); !p.V.IsNull() {
			parts[i], _ = p.GetPlainText(nil)
		}
	}
	return strings.Join(parts, "\f"), len(parts), nil
}

func mediaHTML(b []byte) string {
	z := html.NewTokenizer(bytes.NewReader(b))
	var sb strings.Builder
	skip := 0
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			return strings.Join(strings.Fields(sb.String()), " ")
		case html.StartTagToken, html.EndTagToken:
			if n, _ := z.TagName(); string(n) == "script" || string(n) == "style" {
				if tt == html.StartTagToken {
					skip++
				} else {
					skip--
				}
			}
			sb.WriteByte(' ')
		case html.TextToken:
			if skip <= 0 {
				sb.Write(z.Text())
			}
		}
	}
}

type mediaIFD struct {
	b    []byte
	tiff int
	bo   binary.ByteOrder
}

type mediaTag struct {
	typ   uint16
	count uint32
	at    int
}

var mediaTypeSize = map[uint16]int{1: 1, 2: 1, 3: 2, 4: 4, 5: 8, 7: 1, 9: 4, 10: 8}

func (d *mediaIFD) read(off uint32) (map[uint16]mediaTag, int, int) {
	start := d.tiff + int(off)
	if off == 0 || start+2 > len(d.b) {
		return nil, 0, 0
	}
	n := int(d.bo.Uint16(d.b[start:]))
	end := start + 2 + 12*n + 4
	if end > len(d.b) {
		return nil, 0, 0
	}
	tags := map[uint16]mediaTag{}
	for i := range n {
		e := d.b[start+2+12*i:]
		t := mediaTag{typ: d.bo.Uint16(e[2:]), count: d.bo.Uint32(e[4:]), at: start + 2 + 12*i + 8}
		size := mediaTypeSize[t.typ] * int(t.count)
		if size > 4 {
			t.at = d.tiff + int(d.bo.Uint32(e[8:]))
		}
		if size == 0 || t.at+size > len(d.b) || t.at < 0 {
			continue
		}
		tags[d.bo.Uint16(e)] = t
	}
	return tags, start, end
}

func (d *mediaIFD) str(t mediaTag) string {
	if t.typ != 2 {
		return ""
	}
	return strings.TrimSpace(strings.TrimRight(string(d.b[t.at:t.at+int(t.count)]), "\x00"))
}

func (d *mediaIFD) num(t mediaTag, i int) float64 {
	switch t.typ {
	case 1, 7:
		return float64(d.b[t.at+i])
	case 3:
		return float64(d.bo.Uint16(d.b[t.at+2*i:]))
	case 4:
		return float64(d.bo.Uint32(d.b[t.at+4*i:]))
	case 5:
		if den := d.bo.Uint32(d.b[t.at+8*i+4:]); den != 0 {
			return float64(d.bo.Uint32(d.b[t.at+8*i:])) / float64(den)
		}
	}
	return math.NaN()
}

func mediaTIFF(b []byte) *mediaIFD {
	for i := 0; ; {
		j := bytes.Index(b[i:], []byte("Exif\x00\x00"))
		if j < 0 {
			return nil
		}
		t := i + j + 6
		if t+8 <= len(b) {
			switch string(b[t : t+4]) {
			case "MM\x00\x2a":
				return &mediaIFD{b, t, binary.BigEndian}
			case "II\x2a\x00":
				return &mediaIFD{b, t, binary.LittleEndian}
			}
		}
		i = t
	}
}

func mediaEXIF(b []byte, meta map[string]any) [][2]int {
	d := mediaTIFF(b)
	if d == nil {
		return nil
	}
	ifd0, _, _ := d.read(d.bo.Uint32(d.b[d.tiff+4:]))
	if ifd0 == nil {
		return nil
	}
	if camera := strings.TrimSpace(d.str(ifd0[0x010F]) + " " + d.str(ifd0[0x0110])); camera != "" {
		meta["camera"] = camera
	}
	if t, ok := ifd0[0x0112]; ok && t.typ == 3 {
		meta["orientation"] = d.num(t, 0)
	}
	if p, ok := ifd0[0x8769]; ok && p.typ == 4 {
		exif, _, _ := d.read(uint32(d.num(p, 0)))
		if ts, err := time.Parse("2006:01:02 15:04:05", d.str(exif[0x9003])); err == nil {
			meta["taken_at"] = ts.Format("2006-01-02T15:04:05")
		}
		if w, h := exif[0xA002], exif[0xA003]; meta["width"] == nil && (w.typ == 3 || w.typ == 4) && (h.typ == 3 || h.typ == 4) {
			meta["width"], meta["height"] = d.num(w, 0), d.num(h, 0)
		}
	}
	p, ok := ifd0[0x8825]
	if !ok || p.typ != 4 {
		return nil
	}
	gps, start, end := d.read(uint32(d.num(p, 0)))
	if gps == nil {
		return nil
	}
	spans := [][2]int{{start, end}}
	for _, t := range gps {
		if size := mediaTypeSize[t.typ] * int(t.count); size > 4 {
			spans = append(spans, [2]int{t.at, t.at + size})
		}
	}
	coord := func(ref, val uint16, neg string) (float64, bool) {
		t, ok := gps[val]
		if !ok || t.typ != 5 || t.count < 3 {
			return 0, false
		}
		v := d.num(t, 0) + d.num(t, 1)/60 + d.num(t, 2)/3600
		if strings.EqualFold(d.str(gps[ref]), neg) {
			v = -v
		}
		return math.Round(v*1e7) / 1e7, !math.IsNaN(v)
	}
	lat, ok1 := coord(1, 2, "S")
	lon, ok2 := coord(3, 4, "W")
	if ok1 && ok2 {
		loc := map[string]any{"lat": lat, "lon": lon}
		if t, ok := gps[6]; ok && t.typ == 5 && !math.IsNaN(d.num(t, 0)) {
			alt := d.num(t, 0)
			if r, ok := gps[5]; ok && d.num(r, 0) == 1 {
				alt = -alt
			}
			loc["alt"] = alt
		}
		meta["personal"] = map[string]any{"location": loc}
	}
	return spans
}

func mediaStripGPS(b []byte) []byte {
	spans := mediaEXIF(b, map[string]any{})
	if len(spans) == 0 {
		return b
	}
	out := bytes.Clone(b)
	for _, s := range spans {
		clear(out[s[0]:s[1]])
	}
	return out
}

func (a *Archive) mediaThumbnail(ctx context.Context, b []byte) (map[string]any, error) {
	src, kind, err := image.Decode(bytes.NewReader(b))
	if err != nil || kind == "webp" {
		return nil, err
	}
	sb := src.Bounds()
	scale := min(1, float64(mediaThumb)/float64(max(sb.Dx(), sb.Dy())))
	dst := image.NewRGBA(image.Rect(0, 0, max(1, int(float64(sb.Dx())*scale)), max(1, int(float64(sb.Dy())*scale))))
	draw.Draw(dst, dst.Bounds(), image.White, image.Point{}, draw.Src)
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, sb, draw.Over, nil)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 80}); err != nil {
		return nil, err
	}
	t, err := a.mediaPut(ctx, buf.Bytes())
	if err != nil {
		return nil, err
	}
	t["width"], t["height"] = dst.Bounds().Dx(), dst.Bounds().Dy()
	return t, nil
}

// mediaPut stores bytes the archive made from a file, such as a thumbnail, as a blob of their own.
func (a *Archive) mediaPut(ctx context.Context, b []byte) (map[string]any, error) {
	sum := sha256.Sum256(b)
	hash := hex.EncodeToString(sum[:])
	tmp, err := os.CreateTemp(a.uploads, "made-")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	_, err = tmp.Write(b)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = a.blobs.put(ctx, hash, tmp.Name(), int64(len(b)))
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"sha256": hash, "size": len(b)}, nil
}

// mediaDisplay makes a JPEG anyone's browser shows from an image only some do, such as an iPhone's
// HEIC or a scanner's TIFF, turned the way the camera held it and with its metadata, location
// included, stripped. It needs the vips command, which the image ships with; without it the
// original is all there is.
func (a *Archive) mediaDisplay(ctx context.Context, b []byte) ([]byte, error) {
	vips, err := exec.LookPath("vips")
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp(a.uploads, "display-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	in, out := filepath.Join(dir, "in"), filepath.Join(dir, "out.jpg")
	if err := os.WriteFile(in, b, 0o600); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if msg, err := exec.CommandContext(ctx, vips, "autorot", in, out+"[Q=85,strip]").CombinedOutput(); err != nil {
		return nil, fmt.Errorf("vips: %w: %s", err, bytes.TrimSpace(msg))
	}
	return os.ReadFile(out)
}

func (a *Archive) mediaPost(ctx context.Context, path, ct string, body []byte, out any) error {
	cfg := a.intake.cfg
	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(cfg.Endpoint, "/")+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", ct)
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}
	resp, err := a.intake.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var e struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(b, &e); e.Error != nil || resp.StatusCode != http.StatusOK {
		msg := strings.TrimSpace(string(b[:min(len(b), 300)]))
		if e.Error != nil {
			msg = e.Error.Message
		}
		return fmt.Errorf("%s: %s: %s", path, resp.Status, msg)
	}
	return json.Unmarshal(b, out)
}

func (a *Archive) mediaOCR(ctx context.Context, mt string, b []byte) (string, error) {
	body, err := json.Marshal(map[string]any{"model": a.intake.cfg.Vision, "messages": []any{map[string]any{"role": "user", "content": []any{
		map[string]any{"type": "text", "text": mediaOCR},
		map[string]any{"type": "image_url", "image_url": map[string]string{"url": "data:" + mt + ";base64," + base64.StdEncoding.EncodeToString(b)}},
	}}}})
	if err != nil {
		return "", err
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := a.mediaPost(ctx, "/chat/completions", "application/json", body, &out); err != nil {
		return "", fmt.Errorf("ocr: %w", err)
	}
	if len(out.Choices) == 0 {
		return "", errors.New("ocr: no choices")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

func (a *Archive) mediaTranscribe(ctx context.Context, name string, b []byte) (string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	w.WriteField("model", a.intake.cfg.Whisper)
	w.WriteField("response_format", "json")
	part, err := w.CreateFormFile("file", name)
	if err != nil {
		return "", err
	}
	part.Write(b)
	if err := w.Close(); err != nil {
		return "", err
	}
	var out struct {
		Text string `json:"text"`
	}
	if err := a.mediaPost(ctx, "/audio/transcriptions", w.FormDataContentType(), buf.Bytes(), &out); err != nil {
		return "", fmt.Errorf("transcription: %w", err)
	}
	return strings.TrimSpace(out.Text), nil
}
