package main

import (
	"bytes"
	"cmp"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
	FileByKeyRow | CreateFileRow | GetFileRow | LockFileRow | NextPendingFileRow | LockPendingFileRow | SavePageRow | ReplaceFileRow
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
		return fileOf(a.st.db(ctx).CreateFile(ctx, CreateFileParams{Hash: hash, Name: name, MIME: mt, Size: size, Space: sid}))
	})
	if err != nil {
		return nil, err
	}
	return a.fileReady(ctx, f)
}

func (a *Archive) fileReady(ctx context.Context, f *File) (*File, error) {
	if f.Status != "pending" {
		return f, nil
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
	return &FileLink{URL: fmt.Sprintf("%s/files/%s?exp=%d&sig=%s", a.base, id, exp, a.fileSign(id, exp)), ExpiresAt: time.Unix(exp, 0).UTC()}, nil
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

type tusUpload = ArchiveUpload

var errTusOffset = errors.New("upload offset")

func (up *tusUpload) offset() int64 {
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
		get := (*Queries).Upload
		if lock {
			get = (*Queries).LockUpload
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
			return a.st.db(ctx).CreateUpload(ctx, CreateUploadParams{ID: up.ID, Role: up.Role, Space: up.Space, Name: up.Name, Length: up.Length})
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
			h.Set("Upload-Offset", strconv.FormatInt(up.offset(), 10))
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
			off := up.offset()
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
