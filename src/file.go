package main

import (
	"bytes"
	"cmp"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"math"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Mulholland-Inc/archive/generated"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/ledongthuc/pdf"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
	"golang.org/x/net/html"
)

type File struct {
	ID        uuid.UUID      `json:"id"`
	Hash      string         `json:"sha256"`
	Name      string         `json:"name"`
	MIME      string         `json:"mime"`
	Size      int64          `json:"size"`
	Metadata  map[string]any `json:"metadata"`
	Status    string         `json:"status"`
	Error     string         `json:"error,omitempty"`
	Space     uuid.UUID      `json:"space"`
	CreatedAt time.Time      `json:"created_at"`
}

const (
	fileMaxPost = 64 << 20
	fileLinkTTL = 24 * time.Hour
	tusVersion  = "1.0.0"
)

type fileRow interface {
	generated.FileByKeyRow | generated.CreateFileRow | generated.GetFileRow | generated.RenameFileRow | generated.LockFileRow | generated.NextPendingFileRow | generated.LockPendingFileRow | generated.ReplaceFileRow
}

func fileOf[R fileRow](r R, err error) (*File, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	f := File(r)
	return &f, nil
}

func fileName(name, hash string) string {
	name = strings.TrimSpace(filepath.Base(strings.ReplaceAll(name, "\\", "/")))
	if name == "." || name == "/" || name == "" {
		return hash[:12]
	}
	return name
}

func (a *Archive) AddFile(ctx context.Context, space, name string, r io.Reader) (*File, error) {
	sid, err := coreDo(a, ctx, func(ctx context.Context) (uuid.UUID, error) { return a.coreSpace(ctx, space) })
	if err != nil {
		return nil, err
	}
	return a.fileAdd(ctx, sid, name, r)
}

func (a *Archive) fileAdd(ctx context.Context, sid uuid.UUID, name string, r io.Reader) (*File, error) {
	tmp, err := os.CreateTemp("", "archive-post-")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	_, err = io.Copy(tmp, r)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return nil, err
	}
	return a.fileCommit(ctx, sid, name, func() (io.ReadCloser, error) { return os.Open(tmp.Name()) })
}

func (a *Archive) fileCommit(ctx context.Context, sid uuid.UUID, name string, open func() (io.ReadCloser, error)) (*File, error) {
	if err := a.limit(ctx, "stored bytes"); err != nil {
		return nil, err
	}
	r, err := open()
	if err != nil {
		return nil, err
	}
	h := sha256.New()
	head := make([]byte, 512)
	n, _ := io.ReadFull(r, head)
	h.Write(head[:n])
	size, err := io.Copy(h, r)
	r.Close()
	if err != nil {
		return nil, err
	}
	size += int64(n)
	hash := hex.EncodeToString(h.Sum(nil))
	name = fileName(name, hash)
	mt := mediaSniff(name, head[:n])
	if r, err = open(); err != nil {
		return nil, err
	}
	err = a.blobs.put(ctx, blobKey(hash), hash, r, size)
	r.Close()
	if err != nil {
		return nil, err
	}
	f, err := coreDo(a, ctx, func(ctx context.Context) (*File, error) {
		f, err := fileOf(a.st.db(ctx).FileByKey(ctx, sid, hash, name))
		if !errors.Is(err, ErrNotFound) {
			return f, err
		}
		return fileOf(a.st.db(ctx).CreateFile(ctx, generated.CreateFileParams{Hash: hash, Name: name, MIME: mt, Size: size, Space: sid}))
	})
	if err != nil {
		return nil, err
	}
	storeAfter(ctx, func() {
		if err := a.storedBytes(context.WithValue(context.WithoutCancel(ctx), storeTxKey{}, nil)); err != nil {
			fmt.Fprintf(os.Stderr, "archive: workspace %s usage: %v\n", a.ws.Short, err)
		}
	})
	return a.fileReady(ctx, f)
}

func (a *Archive) fileReady(ctx context.Context, f *File) (*File, error) {
	if f.Status != "pending" {
		return f, nil
	}
	if err := a.soon(ctx, "file", time.Now().Add(hubFileDelay)); err != nil {
		return nil, err
	}
	if _, ok := ctx.Value(storeTxKey{}).(storeTx); ok {
		bg := context.WithValue(context.WithoutCancel(ctx), storeTxKey{}, nil)
		storeAfter(ctx, func() {
			if _, err := a.fileProcess(bg, f.ID); err != nil {
				fmt.Fprintf(os.Stderr, "archive: file %s: %v\n", f.ID, err)
			}
		})
		return f, nil
	}
	return a.ProcessFile(ctx, f.ID)
}

func (a *Archive) Files(ctx context.Context, space string, limit int, after uuid.UUID) ([]*File, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	return coreDo(a, ctx, func(ctx context.Context) ([]*File, error) {
		var sid *uuid.UUID
		if space != "" {
			id, err := a.coreSpace(ctx, space)
			if err != nil {
				return nil, err
			}
			sid = &id
		}
		rows, err := a.st.db(ctx).Files(ctx, after, sid, int32(limit))
		fs := make([]*File, len(rows))
		for i := range rows {
			fs[i] = (*File)(&rows[i])
		}
		return fs, err
	})
}

func (a *Archive) GetFile(ctx context.Context, id uuid.UUID) (*File, error) {
	return coreDo(a, ctx, func(ctx context.Context) (*File, error) {
		f, err := fileOf(a.st.db(ctx).GetFile(ctx, id))
		if errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("file %s: %w", id, ErrNotFound)
		}
		return f, err
	})
}

func (a *Archive) RenameFile(ctx context.Context, id uuid.UUID, name string) (*File, error) {
	name = strings.TrimSpace(filepath.Base(strings.ReplaceAll(name, "\\", "/")))
	if name == "" || name == "." || name == "/" {
		return nil, fmt.Errorf("name: want a file name: %w", httpErrBadRequest)
	}
	return coreDo(a, ctx, func(ctx context.Context) (*File, error) {
		f, err := fileOf(a.st.db(ctx).RenameFile(ctx, id, name))
		var pe *pgconn.PgError
		switch {
		case errors.Is(err, ErrNotFound):
			return nil, fmt.Errorf("file %s: %w", id, ErrNotFound)
		case errors.As(err, &pe) && pe.Code == "23505":
			return nil, fmt.Errorf("a file named %q with the same content is already in this space: %w", name, ErrConflict)
		}
		return f, err
	})
}

func (a *Archive) FileText(ctx context.Context, id uuid.UUID) (string, error) {
	return coreDo(a, ctx, func(ctx context.Context) (string, error) {
		text, err := a.st.db(ctx).FileText(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("file %s: %w", id, ErrNotFound)
		}
		return text, err
	})
}

func (a *Archive) OpenFile(ctx context.Context, id uuid.UUID) (*File, io.ReadSeekCloser, error) {
	f, err := a.GetFile(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	rc, err := a.blobs.open(ctx, blobKey(f.Hash), f.Size)
	return f, rc, err
}

func (a *Archive) ProcessFile(ctx context.Context, id uuid.UUID) (*File, error) {
	if _, err := a.fileProcess(context.WithoutCancel(ctx), id); err != nil {
		return nil, err
	}
	return a.GetFile(ctx, id)
}

func (a *Archive) fileSign(id uuid.UUID, exp int64) string {
	m := hmac.New(sha256.New, a.linkKey)
	fmt.Fprintf(m, "%s.%d", id, exp)
	return hex.EncodeToString(m.Sum(nil))
}

type FileLink struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (a *Archive) LinkFile(ctx context.Context, id uuid.UUID, ttl time.Duration) (*FileLink, error) {
	if _, err := a.GetFile(ctx, id); err != nil {
		return nil, err
	}
	if ttl <= 0 {
		ttl = fileLinkTTL
	}
	exp := time.Now().Add(ttl).Unix()
	return &FileLink{URL: fmt.Sprintf("%s/files/%s/%s?exp=%d&sig=%s", a.hub.api, a.ws.Short, id, exp, a.fileSign(id, exp)), ExpiresAt: time.Unix(exp, 0).UTC()}, nil
}

func mediaInline(mt string) bool {
	top, _, _ := strings.Cut(mt, "/")
	return top == "image" && mt != "image/svg+xml" || top == "audio" || top == "video" || mt == "application/pdf" || mt == "text/plain"
}

func (a *Archive) fileServe(w http.ResponseWriter, r *http.Request, f *File, strip bool) {
	rc, err := a.blobs.open(r.Context(), blobKey(f.Hash), f.Size)
	if err != nil {
		httpWrite(w, 0, nil, err)
		return
	}
	defer rc.Close()
	var content io.ReadSeeker = rc
	etag := `"` + f.Hash + `"`
	if _, gps := f.Metadata["personal"]; strip && gps {
		b, err := io.ReadAll(rc)
		if err != nil {
			httpWrite(w, 0, nil, err)
			return
		}
		content, etag = bytes.NewReader(mediaStripGPS(b)), `"`+f.Hash+`-nogps"`
	}
	disp := "attachment"
	if mediaInline(f.MIME) {
		disp = "inline"
	}
	h := w.Header()
	h.Set("Content-Type", f.MIME)
	h.Set("Content-Disposition", mime.FormatMediaType(disp, map[string]string{"filename": f.Name}))
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("ETag", etag)
	http.ServeContent(w, r, "", f.CreatedAt, content)
}

func (a *Archive) fileRoutes(mux *http.ServeMux) {
	handle := func(pattern string, f func(r *http.Request) (int, any, error)) { httpHandle(mux, pattern, f) }
	file := func(r *http.Request) (*File, error) {
		id, err := httpID(r)
		if err != nil {
			return nil, err
		}
		return a.GetFile(r.Context(), id)
	}
	handle("GET /api/files", func(r *http.Request) (int, any, error) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		var after uuid.UUID
		if s := r.URL.Query().Get("after"); s != "" {
			var err error
			if after, err = uuid.Parse(s); err != nil {
				return 0, nil, fmt.Errorf("after: %w", httpErrBadRequest)
			}
		}
		fs, err := a.Files(r.Context(), r.URL.Query().Get("space"), limit, after)
		return http.StatusOK, fs, err
	})
	handle("POST /api/files", func(r *http.Request) (int, any, error) {
		r.Body = http.MaxBytesReader(nil, r.Body, fileMaxPost)
		mr, err := r.MultipartReader()
		if err != nil {
			return 0, nil, fmt.Errorf("want a multipart/form-data body with a file part: %w", httpErrBadRequest)
		}
		for {
			p, err := mr.NextPart()
			if err != nil {
				return 0, nil, fmt.Errorf("no file part: %w", httpErrBadRequest)
			}
			if p.FormName() == "file" {
				f, err := a.AddFile(r.Context(), r.URL.Query().Get("space"), p.FileName(), p)
				return http.StatusCreated, f, err
			}
		}
	})
	handle("GET /api/files/{id}", func(r *http.Request) (int, any, error) {
		f, err := file(r)
		return http.StatusOK, f, err
	})
	handle("PATCH /api/files/{id}", func(r *http.Request) (int, any, error) {
		id, err := httpID(r)
		if err != nil {
			return 0, nil, err
		}
		var body struct {
			Name string `json:"name"`
		}
		if err := httpDecode(r, &body); err != nil {
			return 0, nil, err
		}
		f, err := a.RenameFile(r.Context(), id, body.Name)
		return http.StatusOK, f, err
	})
	handle("POST /api/files/{id}/link", func(r *http.Request) (int, any, error) {
		id, err := httpID(r)
		if err != nil {
			return 0, nil, err
		}
		var ttl time.Duration
		if s := r.URL.Query().Get("expires"); s != "" {
			if ttl, err = time.ParseDuration(s); err != nil || ttl <= 0 {
				return 0, nil, fmt.Errorf("expires: want a duration like 1h: %w", httpErrBadRequest)
			}
		}
		l, err := a.LinkFile(r.Context(), id, ttl)
		return http.StatusOK, l, err
	})
	mux.HandleFunc("GET /api/files/{id}/content", func(w http.ResponseWriter, r *http.Request) {
		f, err := file(r)
		if err != nil {
			httpWrite(w, 0, nil, err)
			return
		}
		a.fileServe(w, r, f, r.URL.Query().Get("strip") == "gps")
	})
	mux.HandleFunc("GET /api/files/{id}/thumbnail", func(w http.ResponseWriter, r *http.Request) {
		f, err := file(r)
		t := f.thumbnail()
		if err == nil && t == nil {
			err = fmt.Errorf("file %s has no thumbnail: %w", f.ID, ErrNotFound)
		}
		if err != nil {
			httpWrite(w, 0, nil, err)
			return
		}
		a.fileServe(w, r, t, false)
	})
	mux.HandleFunc("GET /api/files/{id}/text", func(w http.ResponseWriter, r *http.Request) {
		id, err := httpID(r)
		var text string
		if err == nil {
			text, err = a.FileText(r.Context(), id)
		}
		if err != nil {
			httpWrite(w, 0, nil, err)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		io.WriteString(w, text)
	})
}

func (f *File) thumbnail() *File {
	if f == nil {
		return nil
	}
	t, _ := f.Metadata["thumbnail"].(map[string]any)
	hash, _ := t["sha256"].(string)
	size, _ := t["size"].(float64)
	if hash == "" {
		return nil
	}
	return &File{ID: f.ID, Hash: hash, Size: int64(size), Name: strings.TrimSuffix(f.Name, filepath.Ext(f.Name)) + ".thumb.jpg",
		MIME: "image/jpeg", CreatedAt: f.CreatedAt}
}

func (a *Archive) fileSigned(w http.ResponseWriter, r *http.Request) {
	id, err := httpID(r)
	q := r.URL.Query()
	exp, _ := strconv.ParseInt(q.Get("exp"), 10, 64)
	if err == nil && (time.Now().Unix() > exp || !hmac.Equal([]byte(q.Get("sig")), []byte(a.fileSign(id, exp)))) {
		err = fmt.Errorf("link is invalid or expired: %w", ErrForbidden)
	}
	var f *File
	if err == nil {
		err = a.st.System(r.Context(), func(ctx context.Context) (err error) {
			f, err = fileOf(a.st.db(ctx).GetFile(ctx, id))
			return err
		})
	}
	if err != nil {
		httpWrite(w, 0, nil, err)
		return
	}
	a.fileServe(w, r, f, true)
}

type tusUpload = generated.ArchiveUpload

var errTusOffset = errors.New("upload offset")

func tusOffset(up *tusUpload) int64 {
	var n int64
	for _, p := range up.Parts {
		n += p
	}
	return n
}

func tusKey(id uuid.UUID, part int) string {
	return "uploads/" + id.String() + "/" + strconv.Itoa(part)
}

type tusReader struct {
	a   *Archive
	ctx context.Context
	up  *tusUpload
	i   int
	rc  io.ReadCloser
}

func (t *tusReader) Read(p []byte) (int, error) {
	for {
		if t.rc == nil {
			if t.i == len(t.up.Parts) {
				return 0, io.EOF
			}
			rc, err := t.a.blobs.open(t.ctx, tusKey(t.up.ID, t.i), t.up.Parts[t.i])
			if err != nil {
				return 0, err
			}
			t.rc = rc
		}
		n, err := t.rc.Read(p)
		if err == io.EOF {
			t.rc.Close()
			t.rc, t.i, err = nil, t.i+1, nil
		}
		if n > 0 || err != nil {
			return n, err
		}
	}
}

func (t *tusReader) Close() error {
	if t.rc == nil {
		return nil
	}
	return t.rc.Close()
}

func tusMetadata(s string) map[string]string {
	out := map[string]string{}
	for _, kv := range strings.Split(s, ",") {
		k, v, _ := strings.Cut(strings.TrimSpace(kv), " ")
		if b, err := base64.StdEncoding.DecodeString(v); err == nil && k != "" {
			out[k] = string(b)
		}
	}
	return out
}

func (a *Archive) tus(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("Tus-Resumable", tusVersion)
	if r.Method == http.MethodOptions {
		h.Set("Tus-Version", tusVersion)
		h.Set("Tus-Extension", "creation,termination")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Header.Get("Tus-Resumable") != tusVersion {
		h.Set("Tus-Version", tusVersion)
		w.WriteHeader(http.StatusPreconditionFailed)
		return
	}
	ref := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/files/tus"), "/")
	if ref == "" && r.Method == http.MethodPost {
		a.tusCreate(w, r)
		return
	}
	id, err := uuid.Parse(ref)
	if err != nil || ref == "" || r.Method != http.MethodHead && r.Method != http.MethodPatch && r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	a.tusUpload(w, r, id)
}

func (a *Archive) tusLocked(ctx context.Context, id uuid.UUID, lock bool, fn func(context.Context, *tusUpload) error) error {
	role := ActorFrom(ctx).Role
	return a.st.System(ctx, func(ctx context.Context) error {
		get := (*generated.Queries).Upload
		if lock {
			get = (*generated.Queries).LockUpload
		}
		up, err := get(a.st.db(ctx), ctx, id)
		if errors.Is(err, pgx.ErrNoRows) || err == nil && up.Role != role {
			err = ErrNotFound
		}
		if err != nil {
			return err
		}
		return fn(ctx, &up)
	})
}

func (a *Archive) tusCreate(w http.ResponseWriter, r *http.Request) {
	ctx := context.WithoutCancel(r.Context())
	n, err := strconv.ParseInt(r.Header.Get("Upload-Length"), 10, 64)
	if err != nil || n < 0 {
		httpWrite(w, 0, nil, fmt.Errorf("Upload-Length: want the size in bytes: %w", httpErrBadRequest))
		return
	}
	meta := tusMetadata(r.Header.Get("Upload-Metadata"))
	up := &tusUpload{ID: coreNewID(), Length: n, Name: cmp.Or(meta["filename"], meta["name"]), Role: ActorFrom(ctx).Role}
	up.Space, err = coreDo(a, ctx, func(ctx context.Context) (uuid.UUID, error) { return a.coreSpace(ctx, meta["space"]) })
	if err == nil {
		err = a.st.System(ctx, func(ctx context.Context) error {
			return a.st.db(ctx).CreateUpload(ctx, generated.CreateUploadParams{ID: up.ID, Role: up.Role, Space: up.Space, Name: up.Name, Length: up.Length})
		})
	}
	var f *File
	if err == nil && n == 0 {
		err = a.tusLocked(ctx, up.ID, true, func(ctx context.Context, up *tusUpload) (err error) {
			f, err = a.tusFinish(ctx, up)
			return err
		})
	}
	if err != nil {
		httpWrite(w, 0, nil, err)
		return
	}
	w.Header().Set("Location", "/api/files/tus/"+up.ID.String())
	if f != nil {
		w.Header().Set("Archive-File-Id", f.ID.String())
	}
	w.WriteHeader(http.StatusCreated)
}

func (a *Archive) tusUpload(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ctx := context.WithoutCancel(r.Context())
	h := w.Header()
	var f *File
	var cerr error
	var err error
	switch r.Method {
	case http.MethodHead:
		err = a.tusLocked(ctx, id, false, func(ctx context.Context, up *tusUpload) error {
			h.Set("Upload-Offset", strconv.FormatInt(tusOffset(up), 10))
			h.Set("Upload-Length", strconv.FormatInt(up.Length, 10))
			h.Set("Cache-Control", "no-store")
			return nil
		})
	case http.MethodDelete:
		err = a.tusLocked(ctx, id, true, a.tusDrop)
	case http.MethodPatch:
		if ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); ct != "application/offset+octet-stream" {
			w.WriteHeader(http.StatusUnsupportedMediaType)
			return
		}
		err = a.tusLocked(ctx, id, true, func(ctx context.Context, up *tusUpload) error {
			off := tusOffset(up)
			h.Set("Upload-Offset", strconv.FormatInt(off, 10))
			if r.Header.Get("Upload-Offset") != strconv.FormatInt(off, 10) {
				return errTusOffset
			}
			tmp, err := os.CreateTemp("", "archive-tus-")
			if err != nil {
				return err
			}
			defer os.Remove(tmp.Name())
			defer tmp.Close()
			sum := sha256.New()
			var n int64
			n, cerr = io.Copy(io.MultiWriter(tmp, sum), io.LimitReader(r.Body, up.Length-off))
			if n > 0 {
				if _, err := tmp.Seek(0, io.SeekStart); err != nil {
					return err
				}
				if err := a.blobs.put(ctx, tusKey(up.ID, len(up.Parts)), hex.EncodeToString(sum.Sum(nil)), tmp, n); err != nil {
					return err
				}
				up.Parts = append(up.Parts, n)
				if err := a.st.db(ctx).SetUploadParts(ctx, up.ID, up.Parts); err != nil {
					return err
				}
				h.Set("Upload-Offset", strconv.FormatInt(off+n, 10))
			}
			if cerr == nil && off+n == up.Length {
				f, err = a.tusFinish(ctx, up)
			}
			return err
		})
	}
	switch {
	case errors.Is(err, ErrNotFound):
		w.WriteHeader(http.StatusNotFound)
	case errors.Is(err, errTusOffset):
		w.WriteHeader(http.StatusConflict)
	case err != nil || cerr != nil:
		httpWrite(w, 0, nil, cmp.Or(err, cerr))
	case r.Method == http.MethodHead:
		w.WriteHeader(http.StatusOK)
	default:
		if f != nil {
			h.Set("Archive-File-Id", f.ID.String())
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (a *Archive) tusFinish(ctx context.Context, up *tusUpload) (*File, error) {
	f, err := a.fileCommit(ctx, up.Space, up.Name, func() (io.ReadCloser, error) { return &tusReader{a: a, ctx: ctx, up: up}, nil })
	if err != nil {
		return nil, err
	}
	return f, a.tusDrop(ctx, up)
}

func (a *Archive) tusDrop(ctx context.Context, up *tusUpload) error {
	if err := a.st.db(ctx).DeleteUpload(ctx, up.ID); err != nil {
		return err
	}
	storeAfter(ctx, func() {
		for i := range up.Parts {
			if err := a.blobs.remove(ctx, tusKey(up.ID, i)); err != nil {
				fmt.Fprintf(os.Stderr, "archive: upload %s: %v\n", up.ID, err)
			}
		}
	})
	return nil
}

const (
	mediaMax   = 64 << 20
	mediaThumb = 320
)

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
	mt, _, _ := mime.ParseMediaType(http.DetectContentType(b))
	if mt == "application/octet-stream" || mt == "text/plain" {
		if ext, _, err := mime.ParseMediaType(mime.TypeByExtension(strings.ToLower(filepath.Ext(name)))); err == nil {
			return ext
		}
	}
	return mt
}

func (a *Archive) fileProcess(ctx context.Context, id uuid.UUID) (bool, error) {
	found := false
	err := a.st.System(ctx, func(ctx context.Context) error {
		var f *File
		var err error
		if id == uuid.Nil {
			f, err = fileOf(a.st.db(ctx).NextPendingFile(ctx))
		} else {
			f, err = fileOf(a.st.db(ctx).LockPendingFile(ctx, id))
		}
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		found = true
		meta, text, perr := a.media(ctx, f)
		err = pgx.BeginFunc(ctx, a.st.Q(ctx), func(tx pgx.Tx) error {
			return generated.New(tx).SetFileContent(ctx, f.ID, meta, strings.ToValidUTF8(strings.ReplaceAll(text, "\x00", ""), "�"))
		})
		status, msg := "done", ""
		if perr = errors.Join(err, perr); perr != nil {
			status, msg = "error", perr.Error()
		}
		return a.st.db(ctx).SetFileStatus(ctx, f.ID, status, msg)
	})
	return found, err
}

func (a *Archive) media(ctx context.Context, f *File) (map[string]any, string, error) {
	meta := map[string]any{}
	if f.Size > mediaMax {
		return meta, "", nil
	}
	rc, err := a.blobs.open(ctx, blobKey(f.Hash), f.Size)
	if err != nil {
		return meta, "", err
	}
	b, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return meta, "", err
	}
	top, _, _ := strings.Cut(f.MIME, "/")
	switch {
	case f.MIME == "application/pdf":
		text, pages, err := mediaPDF(b)
		meta["pages"] = pages
		return meta, text, err
	case f.MIME == "text/html":
		return meta, mediaHTML(b), nil
	case top == "text" || f.MIME == "application/json":
		return meta, string(b), nil
	case top == "image":
		if c, _, err := image.DecodeConfig(bytes.NewReader(b)); err == nil {
			meta["width"], meta["height"] = c.Width, c.Height
		}
		mediaEXIF(b, meta)
		if t, err := a.mediaThumbnail(ctx, b); err == nil && t != nil {
			meta["thumbnail"] = t
		}
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
	hash, err := a.blobPut(ctx, buf.Bytes())
	if err != nil {
		return nil, err
	}
	return map[string]any{"sha256": hash, "size": buf.Len(), "width": dst.Bounds().Dx(), "height": dst.Bounds().Dy()}, nil
}

type blobStore interface {
	put(ctx context.Context, key, sum string, r io.Reader, size int64) error
	open(ctx context.Context, key string, size int64) (io.ReadSeekCloser, error)
	remove(ctx context.Context, key string) error
}

func blobKey(hash string) string { return hash[:2] + "/" + hash }

func (a *Archive) blobPut(ctx context.Context, b []byte) (string, error) {
	sum := sha256.Sum256(b)
	hash := hex.EncodeToString(sum[:])
	return hash, a.blobs.put(ctx, blobKey(hash), hash, bytes.NewReader(b), int64(len(b)))
}

func blobOpen(dir string) (blobStore, error) {
	raw := os.Getenv("ARCHIVE_BLOB_URL")
	if raw == "" {
		return &dirBlobs{filepath.Join(dir, "blobs")}, nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "s3" || u.Host == "" {
		return nil, fmt.Errorf("ARCHIVE_BLOB_URL %q: want s3://bucket/prefix?endpoint=https://...&region=...", raw)
	}
	q := u.Query()
	s := &s3Blobs{prefix: strings.Trim(u.Path, "/"), region: cmp.Or(q.Get("region"), "us-east-1"),
		key:    cmp.Or(os.Getenv("ARCHIVE_BLOB_KEY"), os.Getenv("AWS_ACCESS_KEY_ID")),
		secret: cmp.Or(os.Getenv("ARCHIVE_BLOB_SECRET"), os.Getenv("AWS_SECRET_ACCESS_KEY"))}
	if s.key == "" || s.secret == "" {
		return nil, errors.New("ARCHIVE_BLOB_URL needs ARCHIVE_BLOB_KEY and ARCHIVE_BLOB_SECRET")
	}
	s.base = "https://" + u.Host + ".s3." + s.region + ".amazonaws.com"
	if ep := q.Get("endpoint"); ep != "" {
		s.base = strings.TrimRight(ep, "/") + "/" + u.Host
	}
	return s, nil
}

type dirBlobs struct{ dir string }

func (d *dirBlobs) path(key string) string { return filepath.Join(d.dir, filepath.FromSlash(key)) }

func (d *dirBlobs) put(_ context.Context, key, _ string, r io.Reader, _ int64) error {
	p := d.path(key)
	if _, err := os.Stat(p); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".put-")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, err = io.Copy(tmp, r)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp.Name(), p)
}

func (d *dirBlobs) open(_ context.Context, key string, _ int64) (io.ReadSeekCloser, error) {
	f, err := os.Open(d.path(key))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("blob %s: %w", key, ErrNotFound)
	}
	return f, err
}

func (d *dirBlobs) remove(_ context.Context, key string) error {
	if err := os.Remove(d.path(key)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	os.Remove(filepath.Dir(d.path(key)))
	return nil
}

const s3EmptyHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

type s3Blobs struct {
	base, prefix, region, key, secret string
}

func (s *s3Blobs) url(key string) string {
	if s.prefix != "" {
		key = s.prefix + "/" + key
	}
	var b strings.Builder
	for _, c := range []byte(key) {
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || strings.IndexByte("-._~/", c) >= 0 {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return s.base + "/" + b.String()
}

func s3HMAC(key []byte, s string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(s))
	return m.Sum(nil)
}

func (s *s3Blobs) do(ctx context.Context, method, obj string, body io.Reader, size int64, payload, rng string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, s.url(obj), body)
	if err != nil {
		return nil, err
	}
	req.ContentLength = size
	if rng != "" {
		req.Header.Set("Range", rng)
	}
	now := time.Now().UTC()
	stamp, day := now.Format("20060102T150405Z"), now.Format("20060102")
	scope := day + "/" + s.region + "/s3/aws4_request"
	signed := "host;x-amz-content-sha256;x-amz-date"
	canon := strings.Join([]string{method, req.URL.EscapedPath(), "",
		"host:" + req.URL.Host + "\nx-amz-content-sha256:" + payload + "\nx-amz-date:" + stamp + "\n", signed, payload}, "\n")
	sum := sha256.Sum256([]byte(canon))
	key := []byte("AWS4" + s.secret)
	for _, part := range []string{day, s.region, "s3", "aws4_request"} {
		key = s3HMAC(key, part)
	}
	sig := hex.EncodeToString(s3HMAC(key, "AWS4-HMAC-SHA256\n"+stamp+"\n"+scope+"\n"+hex.EncodeToString(sum[:])))
	req.Header.Set("X-Amz-Date", stamp)
	req.Header.Set("X-Amz-Content-Sha256", payload)
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+s.key+"/"+scope+", SignedHeaders="+signed+", Signature="+sig)
	return http.DefaultClient.Do(req)
}

func s3Err(resp *http.Response) error {
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("s3 %s: %w", resp.Request.Method, ErrNotFound)
	}
	return fmt.Errorf("s3 %s: %s: %s", resp.Request.Method, resp.Status, bytes.TrimSpace(b))
}

func (s *s3Blobs) put(ctx context.Context, key, sum string, r io.Reader, size int64) error {
	resp, err := s.do(ctx, "HEAD", key, nil, 0, s3EmptyHash, "")
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	if resp, err = s.do(ctx, "PUT", key, io.NopCloser(r), size, sum, ""); err != nil {
		return err
	}
	if resp.StatusCode/100 != 2 {
		return s3Err(resp)
	}
	resp.Body.Close()
	return nil
}

func (s *s3Blobs) open(ctx context.Context, key string, size int64) (io.ReadSeekCloser, error) {
	return &s3Object{s: s, ctx: ctx, key: key, size: size}, nil
}

func (s *s3Blobs) remove(ctx context.Context, key string) error {
	resp, err := s.do(ctx, "DELETE", key, nil, 0, s3EmptyHash, "")
	if err != nil {
		return err
	}
	if resp.StatusCode/100 != 2 && resp.StatusCode != http.StatusNotFound {
		return s3Err(resp)
	}
	resp.Body.Close()
	return nil
}

type s3Object struct {
	s         *s3Blobs
	ctx       context.Context
	key       string
	size, off int64
	body      io.ReadCloser
}

func (o *s3Object) Read(p []byte) (int, error) {
	if o.off >= o.size {
		return 0, io.EOF
	}
	if o.body == nil {
		resp, err := o.s.do(o.ctx, "GET", o.key, nil, 0, s3EmptyHash, fmt.Sprintf("bytes=%d-", o.off))
		if err != nil {
			return 0, err
		}
		if resp.StatusCode != http.StatusPartialContent && (resp.StatusCode != http.StatusOK || o.off != 0) {
			return 0, s3Err(resp)
		}
		o.body = resp.Body
	}
	n, err := o.body.Read(p)
	o.off += int64(n)
	return n, err
}

func (o *s3Object) Seek(off int64, whence int) (int64, error) {
	switch whence {
	case io.SeekCurrent:
		off += o.off
	case io.SeekEnd:
		off += o.size
	}
	if off < 0 {
		return 0, errors.New("s3: negative seek")
	}
	if off != o.off {
		o.Close()
		o.body = nil
	}
	o.off = off
	return off, nil
}

func (o *s3Object) Close() error {
	if o.body == nil {
		return nil
	}
	return o.body.Close()
}
