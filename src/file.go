package archive

import (
	"bytes"
	"cmp"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
	Intake    string         `json:"intake,omitempty"`
	Owner     string         `json:"-"`
	Space     uuid.UUID      `json:"space"`
	CreatedAt time.Time      `json:"created_at"`
	// located: where the file was made (metadata's personal) was taken out for the person asking,
	// so its bytes go to them without it too
	located bool
}

// fileWithoutPlace takes where a file was made, a photo's GPS, out of what the person asking sees,
// unless they sent it or are an admin of its space: a photo shows the family who is in it, and
// where someone lives is theirs and the approvers' to know. Its bytes are served without it too.
func (a *Archive) fileWithoutPlace(ctx context.Context, fs ...*File) {
	ac := ActorFrom(ctx)
	admin := map[uuid.UUID]bool{}
	for _, f := range fs {
		if f == nil {
			continue
		}
		if _, ok := f.Metadata["personal"]; !ok {
			continue
		}
		if ac != nil && (f.Owner == ac.Role || f.Owner == ac.UserRole) {
			continue
		}
		if ac != nil {
			ok, seen := admin[f.Space]
			if !seen {
				var base string
				a.sys(ctx, func(ctx context.Context) error {
					return a.st.Q(ctx).QueryRow(ctx, `SELECT role FROM spaces WHERE id = $1`, f.Space).Scan(&base)
				})
				ok = base != "" && a.has(ctx, ac.UserRole, base+"_admin")
				admin[f.Space] = ok
			}
			if ok {
				continue
			}
		}
		m := maps.Clone(f.Metadata)
		delete(m, "personal")
		f.Metadata, f.located = m, true
	}
}

const (
	fileCols    = `id, hash, name, mime, size, metadata, status, error, intake, owner, space, created_at`
	fileMaxPost = 64 << 20
	fileLinkTTL = 24 * time.Hour
	tusVersion  = "1.0.0"
)

func (a *Archive) wakeFiles() {
	select {
	case a.wake <- struct{}{}:
	default:
	}
}

func fileName(name, hash string) string {
	name = strings.TrimSpace(filepath.Base(strings.ReplaceAll(name, "\\", "/")))
	if name == "." || name == "/" || name == "" {
		return hash[:12]
	}
	return name
}

func (a *Archive) AddFile(ctx context.Context, space, name, intake string, r io.Reader) (*File, error) {
	if intake != "" {
		if err := coreHasType(a.Spec(), intake); err != nil {
			return nil, err
		}
	}
	return coreDo(a, ctx, func(ctx context.Context) (*File, error) {
		sid, err := a.coreSpace(ctx, space)
		if err != nil {
			return nil, err
		}
		if intake != "" {
			if err := a.guard(ctx, intake, sid); err != nil {
				return nil, err
			}
		}
		return a.fileAdd(ctx, sid, name, intake, r, false)
	})
}

// AddRestrictedFile adds a file as AddFile does, restricted from the moment it is made: only the
// space's admins and owners and the person adding it see it, until an admin opens it, with no
// moment in between when anyone else could. Bytes the space already holds under the same name
// are another file, perhaps one others see, so they are refused with ErrConflict rather than
// shut or shared; the caller names its file again.
func (a *Archive) AddRestrictedFile(ctx context.Context, space, name string, r io.Reader) (*File, error) {
	return coreDo(a, ctx, func(ctx context.Context) (*File, error) {
		sid, err := a.coreSpace(ctx, space)
		if err != nil {
			return nil, err
		}
		return a.fileAdd(ctx, sid, name, "", r, true)
	})
}

func (a *Archive) fileAdd(ctx context.Context, sid uuid.UUID, name, intake string, r io.Reader, restricted bool) (*File, error) {
	tmp, err := os.CreateTemp(a.uploads, "post-")
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
	return a.fileCommit(ctx, sid, name, intake, tmp.Name(), restricted)
}

func (a *Archive) fileCommit(ctx context.Context, sid uuid.UUID, name, intake, path string, restricted bool) (*File, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	h := sha256.New()
	head := make([]byte, 512)
	n, _ := io.ReadFull(fh, head)
	h.Write(head[:n])
	size, err := io.Copy(h, fh)
	fh.Close()
	if err != nil {
		return nil, err
	}
	size += int64(n)
	hash := hex.EncodeToString(h.Sum(nil))
	name = fileName(name, hash)
	mt := mediaSniff(name, head[:n])
	if err := a.blobs.put(ctx, hash, path, size); err != nil {
		return nil, err
	}
	f, err := coreDo(a, ctx, func(ctx context.Context) (*File, error) {
		f, err := storeOne[File](ctx, a.st, `SELECT `+fileCols+` FROM files WHERE space = $1 AND hash = $2 AND name = $3`, sid, hash, name)
		if err == nil && restricted {
			return nil, fmt.Errorf("the space holds these bytes as %q already: %w", name, ErrConflict)
		}
		if err == nil && intake != "" {
			f, err = storeOne[File](ctx, a.st, `UPDATE files SET intake = $2, status = 'pending' WHERE id = $1 RETURNING `+fileCols, f.ID, intake)
		}
		if !errors.Is(err, ErrNotFound) {
			return f, err
		}
		spec := a.Spec()
		data, _, err := a.coreCheck(ctx, spec, "file", map[string]any{"sha256": hash, "name": name, "mime": mt})
		if err != nil {
			return nil, err
		}
		rec := &Record{Type: "file", SpecVersion: spec.Version, Space: sid, Restricted: restricted}
		if err := a.coreSave(ctx, rec, data, nil); err != nil {
			return nil, err
		}
		f, err = storeOne[File](ctx, a.st, `INSERT INTO files (id, hash, name, mime, size, intake, owner, space)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING `+fileCols, rec.ID, hash, name, mt, size, intake, ActorFrom(ctx).Role, sid)
		if pe := (*pgconn.PgError)(nil); errors.As(err, &pe) && pe.Code == "23505" {
			// the same bytes and name as a file the person adding cannot see
			return nil, fmt.Errorf("the space holds these bytes as %q already: %w", name, ErrConflict)
		}
		return f, err
	})
	if err == nil {
		a.wakeFiles()
	}
	return f, err
}

func (a *Archive) Files(ctx context.Context, space string, limit int, after uuid.UUID) ([]*File, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	return coreDo(a, ctx, func(ctx context.Context) ([]*File, error) {
		var sid any
		if space != "" {
			id, err := a.coreSpace(ctx, space)
			if err != nil {
				return nil, err
			}
			sid = id
		}
		fs, err := storeAll[File](ctx, a.st, `SELECT `+fileCols+` FROM files WHERE id > $1 AND ($3::uuid IS NULL OR space = $3) ORDER BY id LIMIT $2`, after, limit, sid)
		if fs == nil {
			fs = []*File{}
		}
		a.fileWithoutPlace(ctx, fs...)
		return fs, err
	})
}

func (a *Archive) GetFile(ctx context.Context, id uuid.UUID) (*File, error) {
	return coreDo(a, ctx, func(ctx context.Context) (*File, error) {
		f, err := storeOne[File](ctx, a.st, `SELECT `+fileCols+` FROM files WHERE id = $1`, id)
		if errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("file %s: %w", id, ErrNotFound)
		}
		a.fileWithoutPlace(ctx, f)
		return f, err
	})
}

func (a *Archive) FileText(ctx context.Context, id uuid.UUID) (string, error) {
	return coreDo(a, ctx, func(ctx context.Context) (string, error) {
		var text string
		err := a.st.Q(ctx).QueryRow(ctx, `SELECT text FROM files WHERE id = $1`, id).Scan(&text)
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
	rc, err := a.blobs.open(ctx, f.Hash, f.Size)
	return f, rc, err
}

func (a *Archive) IntakeFile(ctx context.Context, id uuid.UUID, typ string) (*IntakeResult, error) {
	if err := coreHasType(a.Spec(), typ); err != nil {
		return nil, err
	}
	raw, err := coreDo(a, ctx, func(ctx context.Context) (*Raw, error) {
		f, err := a.GetFile(ctx, id)
		if err != nil {
			return nil, err
		}
		if err := a.guard(ctx, typ, f.Space); err != nil {
			return nil, err
		}
		text, err := a.FileText(ctx, id)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("file %s has no text to read (status %s): %w", id, f.Status, ErrConflict)
		}
		payload := map[string]any{"file": f.ID, "name": f.Name, "mime": f.MIME, "text": text}
		if f.MIME == "application/pdf" {
			payload["pages"] = strings.Split(text, "\f")
		}
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		raw := &Raw{Source: "file", ExternalID: f.ID.String(), Payload: b, Space: f.Space}
		_, err = a.st.PutRaw(ctx, raw)
		return raw, err
	})
	if err != nil {
		return nil, err
	}
	return a.ingest(ctx, raw, typ)
}

func (a *Archive) ProcessFile(ctx context.Context, id uuid.UUID) (*File, error) {
	var claimed bool
	err := a.st.System(ctx, func(ctx context.Context) error {
		tag, err := a.st.Q(ctx).Exec(ctx, `UPDATE files SET status = 'processing' WHERE id = $1 AND status = 'pending'`, id)
		claimed = tag.RowsAffected() == 1
		return err
	})
	if err != nil {
		return nil, err
	}
	if claimed {
		a.fileProcess(ctx, id)
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
	rc, err := a.blobs.open(r.Context(), f.Hash, f.Size)
	if err != nil {
		httpWrite(w, 0, nil, err)
		return
	}
	defer rc.Close()
	var content io.ReadSeeker = rc
	etag := `"` + f.Hash + `"`
	if _, gps := f.Metadata["personal"]; f.located || strip && gps {
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
	handle := func(pattern string, f func(r *http.Request) (int, any, error)) {
		httpHandle(mux, pattern, func(r *http.Request) (int, any, error) { return f(r.WithContext(outside(r.Context()))) })
	}
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
				f, err := a.AddFile(r.Context(), r.URL.Query().Get("space"), p.FileName(), r.URL.Query().Get("intake"), p)
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
	// what to put in front of a person: the display copy when there is one, else the file itself
	mux.HandleFunc("GET /api/files/{id}/display", func(w http.ResponseWriter, r *http.Request) {
		f, err := file(r)
		if err != nil {
			httpWrite(w, 0, nil, err)
			return
		}
		if d := f.display(); d != nil {
			a.fileServe(w, r, d, false)
			return
		}
		a.fileServe(w, r, f, r.URL.Query().Get("strip") == "gps")
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

func (f *File) thumbnail() *File { return f.made("thumbnail", ".thumb.jpg") }

// display is the JPEG made for an image browsers do not all show, such as HEIC or TIFF.
func (f *File) display() *File { return f.made("display", ".jpg") }

func (f *File) made(key, ext string) *File {
	if f == nil {
		return nil
	}
	t, _ := f.Metadata[key].(map[string]any)
	hash, _ := t["sha256"].(string)
	size, _ := t["size"].(float64)
	if hash == "" {
		return nil
	}
	return &File{ID: f.ID, Hash: hash, Size: int64(size), Name: strings.TrimSuffix(f.Name, filepath.Ext(f.Name)) + ext,
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
			f, err = storeOne[File](ctx, a.st, `SELECT `+fileCols+` FROM files WHERE id = $1`, id)
			return err
		})
	}
	if err != nil {
		httpWrite(w, 0, nil, err)
		return
	}
	a.fileServe(w, r, f, true)
}

type tusUpload struct {
	Length int64     `json:"length"`
	Name   string    `json:"name"`
	Intake string    `json:"intake"`
	Space  uuid.UUID `json:"space"`
	Role   string    `json:"role"`
	// File is the file a finished upload became: kept a week, so a client whose last answer was
	// lost (a phone in a lift) asks again and finds it, rather than sending it all again
	File string `json:"file,omitempty"`
}

// tusKept is how long an upload waits untouched, finished or not, before it is let go.
const tusKept = 7 * 24 * time.Hour

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
	a.tusUpload(w, r, id.String())
}

func (a *Archive) tusCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	n, err := strconv.ParseInt(r.Header.Get("Upload-Length"), 10, 64)
	if err != nil || n < 0 {
		httpWrite(w, 0, nil, fmt.Errorf("Upload-Length: want the size in bytes: %w", httpErrBadRequest))
		return
	}
	meta := tusMetadata(r.Header.Get("Upload-Metadata"))
	up := &tusUpload{Length: n, Name: cmp.Or(meta["filename"], meta["name"]), Intake: meta["intake"], Role: ActorFrom(ctx).Role}
	if up.Intake != "" {
		err = coreHasType(a.Spec(), up.Intake)
	}
	if err == nil {
		up.Space, err = coreDo(a, ctx, func(ctx context.Context) (uuid.UUID, error) {
			sid, err := a.coreSpace(ctx, meta["space"])
			if err == nil && up.Intake != "" {
				err = a.guard(outside(ctx), up.Intake, sid)
			}
			return sid, err
		})
	}
	id := coreNewID().String()
	path := filepath.Join(a.uploads, id)
	if err == nil {
		err = os.WriteFile(path, nil, 0o600)
	}
	if err == nil {
		err = a.tusSave(id, up)
	}
	if err != nil {
		httpWrite(w, 0, nil, err)
		return
	}
	w.Header().Set("Location", "/api/files/tus/"+id)
	if n == 0 {
		f, err := a.tusFinish(ctx, id, up)
		if err != nil {
			httpWrite(w, 0, nil, err)
			return
		}
		w.Header().Set("Archive-File-Id", f.ID.String())
	}
	w.WriteHeader(http.StatusCreated)
}

func (a *Archive) tusSave(id string, up *tusUpload) error {
	b, err := json.Marshal(up)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(a.uploads, id+".json"), b, 0o600)
}

func (a *Archive) tusUpload(w http.ResponseWriter, r *http.Request, id string) {
	v, _ := a.tusLocks.LoadOrStore(id, &sync.Mutex{})
	v.(*sync.Mutex).Lock()
	defer v.(*sync.Mutex).Unlock()

	h := w.Header()
	path := filepath.Join(a.uploads, id)
	var up tusUpload
	b, err := os.ReadFile(path + ".json")
	if err == nil {
		err = json.Unmarshal(b, &up)
	}
	if err != nil || up.Role != ActorFrom(r.Context()).Role {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if up.File != "" {
		switch r.Method {
		case http.MethodDelete:
			os.Remove(path + ".json")
			w.WriteHeader(http.StatusNoContent)
			return
		case http.MethodPatch:
			if r.Header.Get("Upload-Offset") != strconv.FormatInt(up.Length, 10) {
				h.Set("Upload-Offset", strconv.FormatInt(up.Length, 10))
				w.WriteHeader(http.StatusConflict)
				return
			}
		}
		h.Set("Upload-Offset", strconv.FormatInt(up.Length, 10))
		h.Set("Upload-Length", strconv.FormatInt(up.Length, 10))
		h.Set("Archive-File-Id", up.File)
		h.Set("Cache-Control", "no-store")
		w.WriteHeader(map[bool]int{true: http.StatusOK, false: http.StatusNoContent}[r.Method == http.MethodHead])
		return
	}
	st, err := os.Stat(path)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	off := st.Size()
	switch r.Method {
	case http.MethodHead:
		h.Set("Upload-Offset", strconv.FormatInt(off, 10))
		h.Set("Upload-Length", strconv.FormatInt(up.Length, 10))
		h.Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
	case http.MethodDelete:
		os.Remove(path)
		os.Remove(path + ".json")
		a.tusLocks.Delete(id)
		w.WriteHeader(http.StatusNoContent)
	case http.MethodPatch:
		if ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); ct != "application/offset+octet-stream" {
			w.WriteHeader(http.StatusUnsupportedMediaType)
			return
		}
		if r.Header.Get("Upload-Offset") != strconv.FormatInt(off, 10) {
			h.Set("Upload-Offset", strconv.FormatInt(off, 10))
			w.WriteHeader(http.StatusConflict)
			return
		}
		fh, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
		if err != nil {
			httpWrite(w, 0, nil, err)
			return
		}
		n, cerr := io.Copy(fh, io.LimitReader(r.Body, up.Length-off))
		if err := fh.Close(); cerr == nil {
			cerr = err
		}
		off += n
		now := time.Now()
		os.Chtimes(path+".json", now, now) // a slow upload is not an abandoned one
		h.Set("Upload-Offset", strconv.FormatInt(off, 10))
		if cerr != nil {
			httpWrite(w, 0, nil, cerr)
			return
		}
		if off == up.Length {
			f, err := a.tusFinish(r.Context(), id, &up)
			if err != nil {
				httpWrite(w, 0, nil, err)
				return
			}
			h.Set("Archive-File-Id", f.ID.String())
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (a *Archive) tusFinish(ctx context.Context, id string, up *tusUpload) (*File, error) {
	path := filepath.Join(a.uploads, id)
	f, err := a.fileCommit(ctx, up.Space, up.Name, up.Intake, path, false)
	if err != nil {
		return nil, err
	}
	os.Remove(path)
	up.File = f.ID.String()
	if err := a.tusSave(id, up); err != nil {
		return nil, err
	}
	return f, nil
}

// tusSweep lets go of uploads nobody has touched for tusKept: the half-sent, and the record of
// the finished.
func (a *Archive) tusSweep() {
	es, _ := os.ReadDir(a.uploads)
	for _, e := range es {
		id := strings.TrimSuffix(e.Name(), ".json")
		if _, err := uuid.Parse(id); err != nil {
			continue
		}
		if i, err := e.Info(); err == nil && time.Since(i.ModTime()) > tusKept {
			os.Remove(filepath.Join(a.uploads, e.Name()))
			a.tusLocks.Delete(id)
		}
	}
}
