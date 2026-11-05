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
	"encoding/json"
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
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Mulholland-Inc/archive/connect"
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
	ID         uuid.UUID      `json:"id"`
	Hash       string         `json:"sha256"`
	Name       string         `json:"name"`
	MIME       string         `json:"mime"`
	Size       int64          `json:"size"`
	Metadata   map[string]any `json:"metadata"`
	Status     string         `json:"status"`
	Error      string         `json:"error,omitempty"`
	Access     *uuid.UUID     `json:"access"`
	CreatedAt  time.Time      `json:"created_at"`
	Kind       string         `json:"kind"`
	Parent     *uuid.UUID     `json:"parent"`
	Connection *uuid.UUID     `json:"connection"`
	Stream     *string        `json:"stream,omitempty"`
	SourceID   *string        `json:"source_id,omitempty"`
	Version    *string        `json:"version,omitempty"`
	URL        string         `json:"url,omitempty"`
	Drive      *string        `json:"drive,omitempty"`
	OwnerName  *string        `json:"owner_name,omitempty"`
	EditorName *string        `json:"editor_name,omitempty"`
	MadeAt     *time.Time     `json:"made_at,omitempty"`
	EditedAt   *time.Time     `json:"edited_at,omitempty"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

const (
	fileTextMax = 1 << 20
	fileMaxPost = 64 << 20
	fileLinkTTL = 24 * time.Hour
	tusVersion  = "1.0.0"
)

type fileRow interface {
	generated.FileByKeyRow | generated.CreateFileRow | generated.FileKeptRow | generated.KeepFileRow | generated.GetFileRow | generated.RenameFileRow | generated.LockFileRow | generated.NextPendingFileRow | generated.LockPendingFileRow | generated.ReplaceFileRow | generated.SetFileAccessRow | generated.AddFolderRow | generated.MoveFileRow | generated.CopyFileRow
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

func (a *Archive) AddFile(ctx context.Context, access, name string, parent *uuid.UUID, r io.Reader) (*File, error) {
	if _, err := signedIn(ctx); err != nil {
		return nil, err
	}
	g, err := a.fileGroup(ctx, access)
	if err != nil {
		return nil, err
	}
	return a.fileAdd(ctx, g, name, parent, r)
}

func (a *Archive) AddFolder(ctx context.Context, access, name string, parent *uuid.UUID) (*File, error) {
	name = strings.TrimSpace(filepath.Base(strings.ReplaceAll(name, "\\", "/")))
	if name == "" || name == "." || name == "/" {
		return nil, fmt.Errorf("name: want a folder name: %w", httpErrBadRequest)
	}
	g, err := a.fileGroup(ctx, access)
	if err != nil {
		return nil, err
	}
	return coreDo(a, ctx, func(ctx context.Context) (*File, error) {
		return fileOf(a.st.db(ctx).AddFolder(ctx, name, g, parent))
	})
}

func (a *Archive) MoveFile(ctx context.Context, id uuid.UUID, parent *uuid.UUID) (*File, error) {
	if err := a.fileMine(ctx, id, "move"); err != nil {
		return nil, err
	}
	return coreDo(a, ctx, func(ctx context.Context) (*File, error) {
		return fileOf(a.st.db(ctx).MoveFile(ctx, parent, id))
	})
}

func (a *Archive) fileMine(ctx context.Context, id uuid.UUID, verb string) error {
	f, err := a.GetFile(ctx, id)
	if err == nil && f.Connection != nil {
		return fmt.Errorf("cannot %s %s: its connection syncs it, so it is read only here; archive.copy(%q) makes a copy you can change: %w", verb, f.Name, id, ErrForbidden)
	}
	return err
}

func (a *Archive) DeleteFile(ctx context.Context, id uuid.UUID) error {
	if err := a.fileMine(ctx, id, "delete"); err != nil {
		return err
	}
	_, err := coreDo(a, ctx, func(ctx context.Context) (uuid.UUID, error) { return a.st.db(ctx).DeleteFile(ctx, id) })
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("file %s: %w", id, ErrNotFound)
	}
	return err
}

func (a *Archive) blobNext(ctx context.Context) (bool, error) {
	found := false
	err := a.sys(ctx, func(ctx context.Context) error {
		q := a.st.db(ctx)
		hash, err := q.NextBlobGone(ctx)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		found = true
		if err := q.LockHash(ctx, hash); err != nil {
			return err
		}
		used, err := q.BlobUsed(ctx, hash)
		if err != nil {
			return err
		}
		if !used {
			if err := a.blobs.remove(ctx, blobKey(hash)); err != nil {
				fmt.Fprintf(os.Stderr, "archive: workspace %s: blob %s: %v\n", a.ws.ID, hash, err)
				return q.RetryBlobGone(ctx, time.Now().Add(time.Minute), hash)
			}
		}
		return q.DropBlobGone(ctx, hash)
	})
	return found, err
}

func (a *Archive) CopyFile(ctx context.Context, id uuid.UUID, name string, access *string) (*File, error) {
	f, err := a.GetFile(ctx, id)
	if err == nil {
		err = f.blob()
	}
	if err != nil {
		return nil, err
	}
	var g *uuid.UUID
	if access != nil {
		if g, err = a.fileGroup(ctx, *access); err != nil {
			return nil, err
		}
	} else if g = f.Access; f.Connection != nil {
		var c generated.ArchiveConnection
		if err := a.sys(ctx, func(ctx context.Context) (err error) {
			c, err = a.st.db(ctx).Connection(ctx, *f.Connection)
			return err
		}); err != nil {
			return nil, err
		}
		if !c.Integration {
			return nil, fmt.Errorf("%s is yours alone, as your %s connection's is, and a copy has no such setting: pass access, a group's name or \"\" for everyone, to say who sees the copy: %w", f.Name, c.Provider, httpErrBadRequest)
		}
	}
	name = fileName(cmp.Or(name, f.Name), f.Hash)
	return coreDo(a, ctx, func(ctx context.Context) (*File, error) {
		q := a.st.db(ctx)
		if err := q.LockHash(ctx, f.Hash); err != nil {
			return nil, err
		}
		c, err := fileOf(q.FileByKey(ctx, generated.FileByKeyParams{Access: g, Hash: f.Hash, Name: name}))
		if !errors.Is(err, ErrNotFound) {
			return c, err
		}
		return fileOf(q.CopyFile(ctx, name, g, id))
	})
}

type actionFiles struct {
	a     *Archive
	ctx   context.Context
	conn  uuid.UUID
	until time.Time
}

func (x actionFiles) Until() time.Time { return x.until }

func (x actionFiles) Open(s string) (*connect.Upfile, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return nil, fmt.Errorf("file: want an id in the file table, not %q: %w", s, httpErrBadRequest)
	}
	f, rc, err := x.a.OpenFile(x.ctx, id)
	if err != nil {
		return nil, err
	}
	return &connect.Upfile{Name: f.Name, Mime: f.MIME, Size: f.Size, Body: rc}, nil
}

func (x actionFiles) Row(source string) (*connect.Synced, error) {
	return coreDo(x.a, x.ctx, func(ctx context.Context) (*connect.Synced, error) {
		r, err := x.a.st.db(ctx).FileBySource(ctx, &x.conn, &source)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("this connection synced nothing with the source_id %s: %w", source, ErrNotFound)
		}
		if err != nil {
			return nil, err
		}
		return &connect.Synced{Kind: r.Kind, Version: r.Version, Source: r.Source}, nil
	})
}

func (f *File) blob() error {
	switch {
	case f.Kind == "folder":
		return fmt.Errorf("%s is a folder: %w", f.Name, httpErrBadRequest)
	case f.Hash == "" && f.Error != "":
		return fmt.Errorf("Archive has no copy of %s: %s: %w", f.Name, f.Error, ErrNotFound)
	case f.Hash == "":
		return fmt.Errorf("Archive has not copied %s yet: %w", f.Name, ErrNotFound)
	}
	return nil
}

func (a *Archive) fileGroup(ctx context.Context, access string) (g *uuid.UUID, err error) {
	if access == "" {
		return nil, nil
	}
	err = a.sys(ctx, func(ctx context.Context) error {
		g, err = a.groupID(ctx, access, false)
		return err
	})
	return g, err
}

func (a *Archive) fileAdd(ctx context.Context, access *uuid.UUID, name string, parent *uuid.UUID, r io.Reader) (*File, error) {
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
	return a.fileCommit(ctx, access, name, parent, nil, func() (io.ReadCloser, error) { return os.Open(tmp.Name()) })
}

func (a *Archive) fileCommit(ctx context.Context, access *uuid.UUID, name string, parent, conn *uuid.UUID, open func() (io.ReadCloser, error)) (*File, error) {
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
	tmp := blobTemp()
	err = a.blobs.put(ctx, tmp, r, size)
	r.Close()
	if err != nil {
		return nil, err
	}
	defer a.blobs.remove(context.WithoutCancel(ctx), tmp)
	commit := func(ctx context.Context) (*File, error) {
		q := a.st.db(ctx)
		if err := q.LockHash(ctx, hash); err != nil {
			return nil, err
		}
		var f *File
		var err error
		access := access
		if access == nil && parent != nil {
			if up, err := fileOf(q.GetFile(ctx, *parent)); err == nil {
				access = up.Access
			}
		}
		if conn != nil {
			f, err = fileOf(q.FileKept(ctx, conn, hash, name))
		} else {
			f, err = fileOf(q.FileByKey(ctx, generated.FileByKeyParams{Access: access, Parent: parent, Hash: hash, Name: name}))
		}
		if !errors.Is(err, ErrNotFound) {
			return f, err
		}
		if conn != nil {
			f, err = fileOf(q.KeepFile(ctx, generated.KeepFileParams{Hash: hash, Name: name, MIME: mt, Size: size, Access: access, Connection: conn}))
		} else {
			f, err = fileOf(q.CreateFile(ctx, generated.CreateFileParams{Hash: hash, Name: name, MIME: mt, Size: size, Access: access, Parent: parent}))
		}
		if err != nil {
			return nil, err
		}
		return f, a.blobs.move(ctx, tmp, blobKey(hash))
	}
	var f *File
	if conn != nil {
		f, err = commit(ctx)
	} else {
		f, err = coreDo(a, ctx, commit)
	}
	if err != nil {
		return nil, err
	}
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

type FileTree struct{ Parent, Connection *uuid.UUID }

func (a *Archive) Files(ctx context.Context, limit int, after uuid.UUID, tree *FileTree) ([]*File, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	p := generated.FilesParams{After: after, Lim: int32(limit)}
	if tree != nil {
		p.Tree, p.InFolder, p.OfConnection = true, tree.Parent, tree.Connection
	}
	return coreDo(a, ctx, func(ctx context.Context) ([]*File, error) {
		rows, err := a.st.db(ctx).Files(ctx, p)
		fs := make([]*File, len(rows))
		for i := range rows {
			fs[i] = (*File)(&rows[i])
		}
		return fs, err
	})
}

// ExportFiles lists files a person could import as an export, such as a Takeout zip a Drive or Dropbox connection copied in,
// optionally only those whose name has q in it.
func (a *Archive) ExportFiles(ctx context.Context, q string) ([]*File, error) {
	return coreDo(a, ctx, func(ctx context.Context) ([]*File, error) {
		rows, err := a.st.db(ctx).ExportFiles(ctx, strings.TrimSpace(q))
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
	if err := a.fileMine(ctx, id, "rename"); err != nil {
		return nil, err
	}
	return coreDo(a, ctx, func(ctx context.Context) (*File, error) {
		f, err := fileOf(a.st.db(ctx).RenameFile(ctx, id, name))
		var pe *pgconn.PgError
		switch {
		case errors.Is(err, ErrNotFound):
			return nil, fmt.Errorf("file %s: %w", id, ErrNotFound)
		case errors.As(err, &pe) && pe.Code == "23505":
			return nil, fmt.Errorf("a file named %q with the same content is already here: %w", name, ErrConflict)
		}
		return f, err
	})
}

func (a *Archive) SetFileAccess(ctx context.Context, id uuid.UUID, access string) (*File, error) {
	g, err := a.fileGroup(ctx, access)
	if err != nil {
		return nil, err
	}
	if err := a.fileMine(ctx, id, "change who sees"); err != nil {
		return nil, err
	}
	return coreDo(a, ctx, func(ctx context.Context) (*File, error) {
		f, err := fileOf(a.st.db(ctx).SetFileAccess(ctx, g, id))
		var pe *pgconn.PgError
		switch {
		case errors.Is(err, ErrNotFound):
			return nil, fmt.Errorf("file %s: %w", id, ErrNotFound)
		case errors.As(err, &pe) && pe.Code == "23505":
			return nil, fmt.Errorf("a file by that name with the same content is already there: %w", ErrConflict)
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
	if err == nil {
		err = f.blob()
	}
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

func (a *Archive) fileSign(id uuid.UUID, exp int64, role string) string {
	m := hmac.New(sha256.New, a.linkKey)
	fmt.Fprintf(m, "%s.%d.%s", id, exp, role)
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
	ac, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	if ttl <= 0 {
		ttl = fileLinkTTL
	}
	exp := time.Now().Add(ttl).Unix()
	q := url.Values{"exp": {strconv.FormatInt(exp, 10)}, "by": {ac.Role}, "sig": {a.fileSign(id, exp, ac.Role)}}
	return &FileLink{URL: fmt.Sprintf("%s/files/%s/%s?%s", a.hub.api, a.ws.ID.String(), id, q.Encode()), ExpiresAt: time.Unix(exp, 0).UTC()}, nil
}

func mediaInline(mt string) bool {
	top, _, _ := strings.Cut(mt, "/")
	return top == "image" && mt != "image/svg+xml" || top == "audio" || top == "video" || mt == "application/pdf" || mt == "text/plain"
}

func (a *Archive) fileServe(w http.ResponseWriter, r *http.Request, f *File, strip bool) {
	if err := f.blob(); err != nil {
		httpWrite(w, 0, nil, err)
		return
	}
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
	if mediaInline(f.MIME) && r.URL.Query().Get("download") == "" {
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
		var tree *FileTree
		if q := r.URL.Query(); q.Has("parent") || q.Has("connection") || q.Has("top") {
			tree = &FileTree{}
			var err error
			if tree.Parent, err = fileParam(r, "parent"); err == nil {
				tree.Connection, err = fileParam(r, "connection")
			}
			if err != nil {
				return 0, nil, err
			}
		}
		fs, err := a.Files(r.Context(), limit, after, tree)
		return http.StatusOK, fs, err
	})
	handle("GET /api/files/exports", func(r *http.Request) (int, any, error) {
		fs, err := a.ExportFiles(r.Context(), r.URL.Query().Get("q"))
		return http.StatusOK, fs, err
	})
	handle("GET /api/files/stored", func(r *http.Request) (int, any, error) {
		n, err := a.Stored(r.Context())
		return http.StatusOK, map[string]int64{"bytes": n}, err
	})
	handle("GET /api/files/sources", func(r *http.Request) (int, any, error) {
		s, err := a.FileSources(r.Context())
		return http.StatusOK, s, err
	})
	handle("DELETE /api/files/{id}", func(r *http.Request) (int, any, error) {
		id, err := httpID(r)
		if err != nil {
			return 0, nil, err
		}
		return http.StatusNoContent, nil, a.DeleteFile(r.Context(), id)
	})
	handle("POST /api/folders", func(r *http.Request) (int, any, error) {
		var body struct {
			Name   string     `json:"name"`
			Access *string    `json:"access"`
			Parent *uuid.UUID `json:"parent"`
		}
		if err := httpDecode(r, &body); err != nil {
			return 0, nil, err
		}
		f, err := a.AddFolder(r.Context(), deref(body.Access), body.Name, body.Parent)
		return http.StatusCreated, f, err
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
				parent, err := fileParam(r, "parent")
				if err != nil {
					return 0, nil, err
				}
				f, err := a.AddFile(r.Context(), r.URL.Query().Get("access"), p.FileName(), parent, p)
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
			Name   *string         `json:"name"`
			Access json.RawMessage `json:"access"`
			Parent json.RawMessage `json:"parent"`
		}
		if err := httpDecode(r, &body); err != nil {
			return 0, nil, err
		}
		var f *File
		if body.Name != nil {
			if f, err = a.RenameFile(r.Context(), id, *body.Name); err != nil {
				return 0, nil, err
			}
		}
		if len(body.Access) > 0 {
			var g *string
			if err := json.Unmarshal(body.Access, &g); err != nil {
				return 0, nil, fmt.Errorf("access: want a group's id or name, or null for everyone: %w", httpErrBadRequest)
			}
			if f, err = a.SetFileAccess(r.Context(), id, deref(g)); err != nil {
				return 0, nil, err
			}
		}
		if len(body.Parent) > 0 {
			var p *uuid.UUID
			if err := json.Unmarshal(body.Parent, &p); err != nil {
				return 0, nil, fmt.Errorf("parent: want a folder's id, or null for the top: %w", httpErrBadRequest)
			}
			f, err = a.MoveFile(r.Context(), id, p)
		}
		if f == nil && err == nil {
			f, err = a.GetFile(r.Context(), id)
		}
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
	handle("GET /api/files/{id}/model", func(r *http.Request) (int, any, error) {
		id, err := httpID(r)
		if err != nil {
			return 0, nil, err
		}
		m, err := a.ViewDocument(r.Context(), id)
		return http.StatusOK, m, err
	})
	handle("POST /api/files/{id}/edit", func(r *http.Request) (int, any, error) {
		id, err := httpID(r)
		if err != nil {
			return 0, nil, err
		}
		var ops []OfficeOp
		if err := httpDecode(r, &ops); err != nil {
			return 0, nil, err
		}
		f, err := a.EditDocument(r.Context(), id, ops)
		return http.StatusOK, f, err
	})
	mux.HandleFunc("GET /api/files/{id}/media/{part...}", func(w http.ResponseWriter, r *http.Request) {
		id, err := httpID(r)
		var pic []byte
		if err == nil {
			var b []byte
			if _, b, err = a.documentBytes(r.Context(), id); err == nil {
				pic, err = OfficeMedia(b, r.PathValue("part"))
			}
		}
		if err != nil {
			httpWrite(w, 0, nil, err)
			return
		}
		w.Header().Set("Content-Type", cmp.Or(mime.TypeByExtension(path.Ext(r.PathValue("part"))), "application/octet-stream"))
		w.Header().Set("Content-Security-Policy", "sandbox")
		w.Write(pic)
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

func fileParam(r *http.Request, name string) (*uuid.UUID, error) {
	s := r.URL.Query().Get(name)
	if s == "" {
		return nil, nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil, fmt.Errorf("%s: want an id: %w", name, httpErrBadRequest)
	}
	return &id, nil
}

type FileSource struct {
	ID       uuid.UUID `json:"id"`
	Provider string    `json:"provider"`
	Name     string    `json:"name"`
	Account  string    `json:"account"`
}

func (a *Archive) FileSources(ctx context.Context) ([]FileSource, error) {
	return coreDo(a, ctx, func(ctx context.Context) ([]FileSource, error) {
		rows, err := a.st.db(ctx).FileSources(ctx)
		out := make([]FileSource, len(rows))
		for i, c := range rows {
			out[i] = FileSource{ID: c.ID, Provider: c.Provider, Name: c.Provider, Account: c.Account}
			if k := connect.Get(c.Provider); k != nil {
				out[i].Name = k.Name
			}
		}
		return out, err
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
	by := q.Get("by")
	if err == nil && (time.Now().Unix() > exp || !hmac.Equal([]byte(q.Get("sig")), []byte(a.fileSign(id, exp, by)))) {
		err = fmt.Errorf("link is invalid or expired: %w", ErrForbidden)
	}
	var f *File
	if err == nil {
		f, err = a.fileAs(r.Context(), by, id)
	}
	if err != nil {
		httpWrite(w, 0, nil, err)
		return
	}
	a.fileServe(w, r, f, true)
}

func (a *Archive) fileAs(ctx context.Context, role string, id uuid.UUID) (f *File, err error) {
	gone := fmt.Errorf("link no longer reaches a file its maker can see: %w", ErrForbidden)
	var ok bool
	if err := a.sys(ctx, func(ctx context.Context) (err error) {
		ok, err = a.st.db(ctx).RoleExists(ctx, role)
		return err
	}); err != nil {
		return nil, err
	}
	if !ok {
		return nil, gone
	}
	err = a.st.As(ctx, role, func(ctx context.Context) (err error) {
		f, err = fileOf(a.st.db(ctx).GetFile(ctx, id))
		return err
	})
	if errors.Is(err, ErrNotFound) {
		return nil, gone
	}
	return f, err
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
	up.Access, err = a.fileGroup(ctx, meta["access"])
	if err != nil {
		httpWrite(w, 0, nil, err)
		return
	}
	err = a.st.System(ctx, func(ctx context.Context) error {
		return a.st.db(ctx).CreateUpload(ctx, generated.CreateUploadParams{ID: up.ID, Role: up.Role, Access: up.Access, Name: up.Name, Length: up.Length})
	})
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
			var n int64
			n, cerr = io.Copy(tmp, io.LimitReader(r.Body, up.Length-off))
			if n > 0 {
				if _, err := tmp.Seek(0, io.SeekStart); err != nil {
					return err
				}
				if err := a.blobs.put(ctx, tusKey(up.ID, len(up.Parts)), tmp, n); err != nil {
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
	f, err := a.fileCommit(ctx, up.Access, up.Name, nil, nil, func() (io.ReadCloser, error) { return &tusReader{a: a, ctx: ctx, up: up}, nil })
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
			return generated.New(tx).SetFileContent(ctx, f.ID, meta, fileWords(text))
		})
		status, msg := "done", ""
		if perr = errors.Join(err, perr); perr != nil {
			status, msg = "error", perr.Error()
		}
		return a.st.db(ctx).SetFileStatus(ctx, f.ID, status, msg)
	})
	return found, err
}

func fileWords(text string) string {
	text = strings.ToValidUTF8(strings.ReplaceAll(text, "\x00", ""), "�")
	if len(text) <= fileTextMax {
		return text
	}
	return strings.ToValidUTF8(text[:fileTextMax], "")
}

const (
	pullPar      = 4
	pullBudget   = 4 * time.Minute
	pullTime     = 5 * time.Minute
	pullReadTime = 2 * time.Minute
	pullRetry    = time.Minute
	pullBusyWait = 15 * time.Minute
	pullTries    = 6
)

func (a *Archive) pullAll(ctx context.Context) {
	end := time.Now().Add(pullBudget)
	var wg sync.WaitGroup
	for range pullPar {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(end) && ctx.Err() == nil {
				var f generated.ClaimPullRow
				err := a.sys(ctx, func(ctx context.Context) (err error) {
					f, err = a.st.db(ctx).ClaimPull(ctx, time.Now().Add(pullTime+time.Minute))
					return err
				})
				if err != nil {
					if !errors.Is(err, pgx.ErrNoRows) && ctx.Err() == nil {
						fmt.Fprintf(os.Stderr, "archive: workspace %s pulls: %v\n", a.ws.ID, err)
					}
					return
				}
				a.pull(ctx, f)
			}
		}()
	}
	wg.Wait()
}

func (a *Archive) pull(ctx context.Context, f generated.ClaimPullRow) {
	pctx, cancel := context.WithTimeout(ctx, pullTime)
	defer cancel()
	err := a.pullCopy(pctx, f)
	if err == nil {
		err = a.pullRead(pctx, f)
	}
	if err == nil {
		return
	}
	stopped := ctx.Err() != nil
	if errors.Is(err, context.DeadlineExceeded) && !stopped {
		err = fmt.Errorf("copying it took over %s", pullTime)
	}
	bg := context.WithoutCancel(ctx)
	id, version := f.ID, f.Version
	var skip *connect.Unsupported
	err = a.sys(bg, func(ctx context.Context) error {
		q := a.st.db(ctx)
		now := time.Now()
		switch {
		case errors.As(err, &skip):
			return q.SetPullEnd(ctx, generated.SetPullEndParams{ID: id, Version: version, Status: "skipped", Error: "Archive does not copy this file: " + skip.Reason})
		case errors.Is(err, errRevoked):
			return q.SetPullRetry(ctx, generated.SetPullRetryParams{ID: id, Version: version, At: now.Add(time.Hour), Tries: f.Tries - 1, Error: err.Error()})
		case connect.Busy(err) || stopped:
			wait := max(min(pullRetry<<min(max(f.Tries-1, 0), 16), pullBusyWait), connect.RetryAfter(err))
			return q.SetPullRetry(ctx, generated.SetPullRetryParams{ID: id, Version: version, At: now.Add(wait), Tries: f.Tries - 1, Error: ""})
		case f.Tries >= pullTries:
			return q.SetPullEnd(ctx, generated.SetPullEndParams{ID: id, Version: version, Status: "error", Error: err.Error()})
		}
		return q.SetPullRetry(ctx, generated.SetPullRetryParams{ID: id, Version: version, At: now.Add(pullRetry << min(f.Tries-1, 16)), Tries: f.Tries, Error: ""})
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "archive: workspace %s: file %s: %v\n", a.ws.ID, id, err)
	}
}

func (a *Archive) pullCopy(ctx context.Context, f generated.ClaimPullRow) error {
	if f.Copied != nil && f.Version != nil && *f.Copied == *f.Version && f.Hash != "" {
		return nil
	}
	var c generated.ArchiveConnection
	err := a.sys(ctx, func(ctx context.Context) (err error) {
		c, err = a.st.db(ctx).Connection(ctx, *f.Connection)
		return err
	})
	if err != nil {
		return err
	}
	p, err := a.hub.connect.Get(c.Provider)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(p.Streams, func(s *connect.Stream) bool { return f.Stream != nil && s.Name == *f.Stream })
	if i < 0 || p.Streams[i].Read == nil {
		return &connect.Unsupported{Reason: p.Name + " has no bytes of it to copy"}
	}
	cl, err := a.connClient(ctx, c)
	if err != nil {
		return err
	}
	defer cl.Close()
	body, err := p.Streams[i].Read(cl, f.Source)
	if err != nil {
		return err
	}
	defer body.Body.Close()
	h := sha256.New()
	var head fileHead
	counted := &fileCount{r: io.TeeReader(io.TeeReader(body.Body, h), &head), most: cmp.Or(p.Streams[i].Most, connect.MaxCopy)}
	tmp := blobTemp()
	if err := a.blobs.put(ctx, tmp, counted, body.Size); err != nil {
		if counted.over {
			return &connect.Unsupported{Reason: fmt.Sprintf("it is over the %d GB Archive copies", counted.most>>30)}
		}
		return err
	}
	hash := hex.EncodeToString(h.Sum(nil))
	moved := false
	err = a.sys(context.WithoutCancel(ctx), func(ctx context.Context) error {
		q := a.st.db(ctx)
		if err := q.LockHash(ctx, hash); err != nil {
			return err
		}
		row, err := fileOf(q.LockFile(ctx, f.ID))
		if errors.Is(err, ErrNotFound) || err == nil && !slices.Equal(ptrSlice(row.Version), ptrSlice(f.Version)) {
			return nil
		}
		if err != nil {
			return err
		}
		if _, err = q.SetPulled(ctx, generated.SetPulledParams{ID: f.ID, Version: f.Version, Hash: hash, Size: counted.n, Sniffed: mediaSniff(f.Name, head.b)}); err != nil {
			return err
		}
		if err = a.blobs.move(ctx, tmp, blobKey(hash)); err == nil {
			moved = true
		}
		return err
	})
	if !moved {
		a.blobs.remove(context.WithoutCancel(ctx), tmp)
	}
	return err
}

func ptrSlice(p *string) []string {
	if p == nil {
		return nil
	}
	return []string{*p}
}

type fileHead struct{ b []byte }

func (h *fileHead) Write(p []byte) (int, error) {
	if n := 512 - len(h.b); n > 0 {
		h.b = append(h.b, p[:min(n, len(p))]...)
	}
	return len(p), nil
}

type fileCount struct {
	r       io.Reader
	n, most int64
	over    bool
}

func (c *fileCount) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if c.n += int64(n); c.n > c.most {
		c.over = true
		return n, errors.New("over the most Archive copies")
	}
	return n, err
}

func (a *Archive) pullRead(ctx context.Context, c generated.ClaimPullRow) error {
	var f *File
	err := a.sys(ctx, func(ctx context.Context) (err error) {
		f, err = fileOf(a.st.db(ctx).GetFile(ctx, c.ID))
		return err
	})
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if f.Hash == "" || !slices.Equal(ptrSlice(f.Version), ptrSlice(c.Version)) {
		return nil
	}
	type read struct {
		meta map[string]any
		text string
		err  error
	}
	done := func(ctx context.Context, r read) error {
		status, msg := "done", ""
		if r.err != nil {
			status, msg = "error", r.err.Error()
		}
		if r.meta == nil {
			r.meta = map[string]any{}
		}
		return a.st.db(ctx).SetPullDone(ctx, generated.SetPullDoneParams{ID: f.ID, Version: c.Version, Status: status, Error: msg, Metadata: r.meta, Body: fileWords(r.text)})
	}
	bg := context.WithoutCancel(ctx)
	late := false
	err = a.sys(bg, func(ctx context.Context) error {
		got := make(chan read, 1)
		a.reading.Lock()
		go func() {
			defer a.reading.Unlock()
			m, t, e := a.media(ctx, f)
			got <- read{m, t, e}
		}()
		select {
		case r := <-got:
			return done(ctx, r)
		case <-time.After(pullReadTime):
			late = true
			return errors.New("too long")
		}
	})
	if late {
		return a.sys(bg, func(ctx context.Context) error {
			return done(ctx, read{err: fmt.Errorf("reading its words took over %s", pullReadTime)})
		})
	}
	return err
}

func (a *Archive) Stored(ctx context.Context) (int64, error) {
	return coreDo(a, ctx, func(ctx context.Context) (int64, error) { return a.st.db(ctx).Stored(ctx) })
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
	if f.MIME == "application/octet-stream" {
		f.MIME = mediaSniff(f.Name, b[:min(len(b), 512)])
	}
	top, _, _ := strings.Cut(f.MIME, "/")
	if _, office := officeMIME[strings.ToLower(f.MIME)]; office {
		return meta, mediaText(f.Name, f.MIME, b), nil
	}
	if _, err := OfficeKind(f.Name); err == nil {
		return meta, mediaText(f.Name, f.MIME, b), nil
	}
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
	put(ctx context.Context, key string, r io.Reader, size int64) error
	move(ctx context.Context, from, to string) error
	open(ctx context.Context, key string, size int64) (io.ReadSeekCloser, error)
	remove(ctx context.Context, key string) error
}

func blobKey(hash string) string { return hash[:2] + "/" + hash }

func blobTemp() string { return "copies/" + coreNewID().String() + ".part" }

func (a *Archive) blobPut(ctx context.Context, b []byte) (string, error) {
	sum := sha256.Sum256(b)
	hash := hex.EncodeToString(sum[:])
	if err := a.st.db(ctx).LockHash(ctx, hash); err != nil {
		return "", err
	}
	return hash, a.blobs.put(ctx, blobKey(hash), bytes.NewReader(b), int64(len(b)))
}

func blobOpen(dir string) (blobStore, error) {
	raw := os.Getenv("ARCHIVE_BLOB_URL")
	if raw == "" {
		return &dirBlobs{filepath.Join(dir, "blobs")}, nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "gs" || u.Host == "" {
		return nil, fmt.Errorf("ARCHIVE_BLOB_URL %q: want gs://bucket/prefix", raw)
	}
	base := cmp.Or(strings.TrimRight(u.Query().Get("endpoint"), "/"), "https://storage.googleapis.com") + "/" + u.Host
	return &gcsBlobs{base: base, prefix: strings.Trim(u.Path, "/"), gcp: gcpMetadata()}, nil
}

type dirBlobs struct{ dir string }

func (d *dirBlobs) path(key string) string { return filepath.Join(d.dir, filepath.FromSlash(key)) }

func (d *dirBlobs) put(_ context.Context, key string, r io.Reader, _ int64) error {
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

func (d *dirBlobs) move(_ context.Context, from, to string) error {
	if _, err := os.Stat(d.path(to)); err == nil {
		return os.Remove(d.path(from))
	}
	if err := os.MkdirAll(filepath.Dir(d.path(to)), 0o700); err != nil {
		return err
	}
	return os.Rename(d.path(from), d.path(to))
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

type gcsBlobs struct {
	base, prefix string
	gcp          *gcpToken
}

func (s *gcsBlobs) url(key string) string {
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

func (s *gcsBlobs) do(ctx context.Context, method, obj string, body io.Reader, size int64, rng string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, s.url(obj), body)
	if err != nil {
		return nil, err
	}
	req.ContentLength = size
	if rng != "" {
		req.Header.Set("Range", rng)
	}
	if from, ok := strings.CutPrefix(rng, "copy:"); ok {
		req.Header.Del("Range")
		req.Header.Set("x-goog-copy-source", "/"+strings.TrimPrefix(s.url(from), strings.TrimRight(s.origin(), "/")+"/"))
	}
	tok, err := s.gcp.access(ctx)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	return http.DefaultClient.Do(req)
}

func gcsErr(resp *http.Response) error {
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("gcs %s: %w", resp.Request.Method, ErrNotFound)
	}
	return fmt.Errorf("gcs %s: %s: %s", resp.Request.Method, resp.Status, bytes.TrimSpace(b))
}

func (s *gcsBlobs) put(ctx context.Context, key string, r io.Reader, size int64) error {
	resp, err := s.do(ctx, "HEAD", key, nil, 0, "")
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	if resp, err = s.do(ctx, "PUT", key, io.NopCloser(r), size, ""); err != nil {
		return err
	}
	if resp.StatusCode/100 != 2 {
		return gcsErr(resp)
	}
	resp.Body.Close()
	return nil
}

func (s *gcsBlobs) origin() string { return s.base[:strings.LastIndex(s.base, "/")] }

func (s *gcsBlobs) move(ctx context.Context, from, to string) error {
	resp, err := s.do(ctx, "HEAD", to, nil, 0, "")
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if resp, err = s.do(ctx, "PUT", to, nil, 0, "copy:"+from); err != nil {
			return err
		}
		if resp.StatusCode/100 != 2 {
			return gcsErr(resp)
		}
		resp.Body.Close()
		if resp, err = s.do(ctx, "HEAD", to, nil, 0, ""); err != nil {
			return err
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("gcs: copying to %s left nothing there: %s", to, resp.Status)
		}
	}
	return s.remove(ctx, from)
}

func (s *gcsBlobs) open(ctx context.Context, key string, size int64) (io.ReadSeekCloser, error) {
	return &gcsObject{s: s, ctx: ctx, key: key, size: size}, nil
}

func (s *gcsBlobs) remove(ctx context.Context, key string) error {
	resp, err := s.do(ctx, "DELETE", key, nil, 0, "")
	if err != nil {
		return err
	}
	if resp.StatusCode/100 != 2 && resp.StatusCode != http.StatusNotFound {
		return gcsErr(resp)
	}
	resp.Body.Close()
	return nil
}

type gcsObject struct {
	s         *gcsBlobs
	ctx       context.Context
	key       string
	size, off int64
	body      io.ReadCloser
}

func (o *gcsObject) Read(p []byte) (int, error) {
	if o.off >= o.size {
		return 0, io.EOF
	}
	if o.body == nil {
		resp, err := o.s.do(o.ctx, "GET", o.key, nil, 0, fmt.Sprintf("bytes=%d-", o.off))
		if err != nil {
			return 0, err
		}
		if resp.StatusCode != http.StatusPartialContent && (resp.StatusCode != http.StatusOK || o.off != 0) {
			return 0, gcsErr(resp)
		}
		o.body = resp.Body
	}
	n, err := o.body.Read(p)
	o.off += int64(n)
	return n, err
}

func (o *gcsObject) Seek(off int64, whence int) (int64, error) {
	switch whence {
	case io.SeekCurrent:
		off += o.off
	case io.SeekEnd:
		off += o.size
	}
	if off < 0 {
		return 0, errors.New("gcs: negative seek")
	}
	if off != o.off {
		o.Close()
		o.body = nil
	}
	o.off = off
	return off, nil
}

func (o *gcsObject) Close() error {
	if o.body == nil {
		return nil
	}
	return o.body.Close()
}

func init() { connect.Extract = mediaText }

var officeMIME = map[string]string{
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   "docx",
	"application/vnd.ms-word.document.macroenabled.12":                          "docm",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         "xlsx",
	"application/vnd.ms-excel.sheet.macroenabled.12":                            "xlsm",
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": "pptx",
	"application/vnd.ms-powerpoint.presentation.macroenabled.12":                "pptm",
}

var byExt = map[string]string{".pdf": "application/pdf", ".txt": "text/plain", ".csv": "text/csv", ".md": "text/markdown",
	".html": "text/html", ".htm": "text/html", ".json": "application/json"}

func mediaText(name, mime string, b []byte) string {
	mime = strings.ToLower(mime)
	if mime == "" || mime == "application/octet-stream" {
		mime = byExt[strings.ToLower(path.Ext(name))]
	}
	top, _, _ := strings.Cut(mime, "/")
	if ext, ok := officeMIME[mime]; ok {
		return officeText(b, "file."+ext)
	}
	if _, err := OfficeKind(name); err == nil {
		return officeText(b, path.Base(name))
	}
	switch {
	case mime == "application/pdf":
		text, _, _ := mediaPDF(b)
		return text
	case mime == "text/html":
		return mediaHTML(b)
	case top == "text" || mime == "application/json":
		return string(b)
	}
	return ""
}

func officeText(b []byte, name string) (text string) {
	defer func() {
		if recover() != nil {
			text = ""
		}
	}()
	m, err := OfficeRead(b, name)
	if err != nil {
		return ""
	}
	var sb strings.Builder
	line := func(s string) {
		if s = strings.TrimSpace(s); s != "" {
			sb.WriteString(s)
			sb.WriteByte('\n')
		}
	}
	for _, p := range m.Paragraphs {
		line(p.Text)
	}
	for _, t := range m.Tables {
		for _, r := range t.Rows {
			line(strings.Join(r, "\t"))
		}
	}
	for _, s := range m.Sheets {
		line("# " + s.Name)
		row, cells := "", []string{}
		flush := func() {
			line(strings.Join(cells, "\t"))
			cells = cells[:0]
		}
		for _, c := range s.Cells {
			if r := strings.TrimLeft(c.Ref, "ABCDEFGHIJKLMNOPQRSTUVWXYZ$"); r != row {
				flush()
				row = r
			}
			if c.Value == "" {
				continue
			}
			col := 0
			for _, ch := range strings.TrimRight(strings.TrimLeft(c.Ref, "$"), "0123456789") {
				if ch >= 'A' && ch <= 'Z' {
					col = col*26 + int(ch-'A') + 1
				}
			}
			for len(cells) < col-1 {
				cells = append(cells, "")
			}
			cells = append(cells, c.Value)
		}
		flush()
	}
	for _, s := range m.Slides {
		if s.Hidden {
			continue
		}
		for _, sh := range s.Shapes {
			for _, p := range sh.Paragraphs {
				line(p.Text)
			}
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}
