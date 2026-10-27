package main

import (
	"bytes"
	"cmp"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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
