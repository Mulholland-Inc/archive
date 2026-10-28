package main

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/ulikunitz/xz"
)

const pgSocketMax = 103

func StartPostgres(ctx context.Context, dataDir string) (dsn string, stop func() error, err error) {
	dataDir, err = filepath.Abs(dataDir)
	if err != nil {
		return "", nil, err
	}
	pgDir := filepath.Join(dataDir, "pg")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return "", nil, err
	}

	if _, err := os.Stat(filepath.Join(pgDir, "PG_VERSION")); errors.Is(err, os.ErrNotExist) {
		initdb, err := pgBin(ctx, dataDir, "initdb")
		if err != nil {
			return "", nil, err
		}
		cmd := exec.CommandContext(ctx, initdb, "-D", pgDir, "-U", "archive",
			"--auth-local=trust", "--auth-host=reject", "-E", "UTF8", "--locale=C")
		cmd.Env = pgEnv()
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", nil, fmt.Errorf("initdb: %w\n%s", err, out)
		}
	}

	sockDir, err := pgSocketDir(dataDir)
	if err != nil {
		return "", nil, err
	}
	if err := pgClearStale(pgDir); err != nil {
		return "", nil, err
	}

	postgres, err := pgBin(ctx, dataDir, "postgres")
	if err != nil {
		return "", nil, err
	}
	logf, err := os.OpenFile(filepath.Join(dataDir, "postgres.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return "", nil, err
	}

	cmd := exec.Command(postgres, "-D", pgDir, "-c", "listen_addresses=", "-k", sockDir)
	cmd.Stdout, cmd.Stderr, cmd.Env = logf, logf, pgEnv()
	if err := cmd.Start(); err != nil {
		logf.Close()
		return "", nil, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait(); logf.Close() }()

	stop = func() error {
		_ = cmd.Process.Signal(syscall.SIGINT)
		select {
		case err := <-done:
			done <- err
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				return nil
			}
			return err
		case <-time.After(time.Minute):
			_ = cmd.Process.Kill()
			return errors.New("postgres did not stop within a minute; killed")
		}
	}

	host := pgQuote(sockDir)
	if err := pgWaitReady(ctx, "host="+host+" user=archive dbname=postgres", done, dataDir); err != nil {
		_ = stop()
		return "", nil, err
	}
	if err := pgEnsureDB(ctx, "host="+host+" user=archive dbname=postgres"); err != nil {
		_ = stop()
		return "", nil, err
	}
	return "host=" + host + " user=archive dbname=archive", stop, nil
}

func pgLive(dataDir string) string {
	b, err := os.ReadFile(filepath.Join(dataDir, "pg", "postmaster.pid"))
	if err != nil {
		return ""
	}
	lines := strings.Split(string(b), "\n")
	if len(lines) < 5 {
		return ""
	}
	pid, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	sock, port := strings.TrimSpace(lines[4]), strings.TrimSpace(lines[3])
	if err != nil || pid <= 0 || sock == "" || syscall.Kill(pid, 0) != nil || !pgIsPostgres(pid) {
		return ""
	}
	return "host=" + pgQuote(sock) + " port=" + port + " user=archive dbname=archive"
}

func pgEnv() []string { return append(os.Environ(), "LC_ALL=C") }

func pgBin(ctx context.Context, dataDir, name string) (string, error) {
	if dir := os.Getenv("ARCHIVE_PG_BIN"); dir != "" {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("%s not found in $ARCHIVE_PG_BIN: %w", name, err)
		}
		return p, nil
	}
	cached := filepath.Join(dataDir, "pgbin", pgVersion, "bin", name)
	if _, err := os.Stat(cached); err == nil {
		return cached, nil
	}
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	if err := pgDownload(ctx, filepath.Join(dataDir, "pgbin")); err != nil {
		return "", fmt.Errorf("%s not found on PATH or in $ARCHIVE_PG_BIN, and downloading Postgres failed: %w", name, err)
	}
	return cached, nil
}

const pgVersion = "18.6.0"

var pgSums = map[string]string{
	"darwin/arm64": "8f6ed6f38195b50a40712ada8b2a12060b6193c1a6977a6eb5cff053bb5a1b89",
	"darwin/amd64": "690941008aac9ee360820fde9bb7870a4f3dcac5e9e2f9e45e42333a436ad0f9",
	"linux/amd64":  "008ee189eaa7ca2b58bd01dd950291e638b998392cf982297acd4779ec898ac1",
	"linux/arm64":  "5f54e016723afa6213925a8acb939bac52944b1d348f65c7a20f0cd2928c31be",
}

func pgDownload(ctx context.Context, root string) error {
	sum, ok := pgSums[runtime.GOOS+"/"+runtime.GOARCH]
	if !ok {
		return fmt.Errorf("no Postgres build for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	artifact := "embedded-postgres-binaries-" + runtime.GOOS + "-" + map[string]string{"amd64": "amd64", "arm64": "arm64v8"}[runtime.GOARCH]
	url := "https://repo1.maven.org/maven2/io/zonky/test/postgres/" + artifact + "/" + pgVersion + "/" + artifact + "-" + pgVersion + ".jar"
	fmt.Fprintf(os.Stderr, "archive: no Postgres found; downloading Postgres %s into %s\n", pgVersion, root)

	jar, err := os.CreateTemp(root, "download-*.jar")
	if err != nil {
		return err
	}
	defer os.Remove(jar.Name())
	defer jar.Close()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(jar, h), resp.Body)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != sum {
		return fmt.Errorf("%s has sha256 %s, want %s", url, got, sum)
	}

	zr, err := zip.NewReader(jar, n)
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(root, "extract-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	for _, f := range zr.File {
		if !strings.HasSuffix(f.Name, ".txz") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		err = pgUntar(rc, tmp)
		rc.Close()
		if err != nil {
			return err
		}
		return os.Rename(tmp, filepath.Join(root, pgVersion))
	}
	return fmt.Errorf("%s holds no .txz", url)
}

func pgUntar(r io.Reader, dir string) error {
	xr, err := xz.NewReader(bufio.NewReader(r))
	if err != nil {
		return err
	}
	tr := tar.NewReader(xr)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if !filepath.IsLocal(h.Name) {
			return fmt.Errorf("unsafe path %q in the Postgres archive", h.Name)
		}
		p := filepath.Join(dir, h.Name)
		switch h.Typeflag {
		case tar.TypeDir:
			err = os.MkdirAll(p, 0o755)
		case tar.TypeSymlink:
			if !filepath.IsLocal(filepath.Join(filepath.Dir(h.Name), h.Linkname)) {
				return fmt.Errorf("unsafe link %q -> %q in the Postgres archive", h.Name, h.Linkname)
			}
			if err = os.MkdirAll(filepath.Dir(p), 0o755); err == nil {
				err = os.Symlink(h.Linkname, p)
			}
		case tar.TypeReg:
			err = pgWrite(p, tr, h.FileInfo().Mode().Perm())
		}
		if err != nil {
			return err
		}
	}
}

func pgWrite(p string, r io.Reader, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func pgSocketDir(dataDir string) (string, error) {
	const sock = "/.s.PGSQL.5432"
	if len(dataDir)+len(sock) <= pgSocketMax {
		return dataDir, nil
	}
	sum := sha256.Sum256([]byte(dataDir))
	name := "archive-pg-" + hex.EncodeToString(sum[:6])
	dir := filepath.Join(os.TempDir(), name)
	if len(dir)+len(sock) > pgSocketMax {
		dir = filepath.Join("/tmp", name)
	}
	return dir, os.MkdirAll(dir, 0o700)
}

func pgClearStale(pgDir string) error {
	pidFile := filepath.Join(pgDir, "postmaster.pid")
	b, err := os.ReadFile(pidFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	line, _, _ := bytes.Cut(b, []byte("\n"))
	pid, err := strconv.Atoi(strings.TrimSpace(string(line)))
	if err == nil && pid > 0 && syscall.Kill(pid, 0) == nil && pgIsPostgres(pid) {
		return fmt.Errorf("postgres (pid %d) is already running on %s", pid, pgDir)
	}
	return os.Remove(pidFile)
}

func pgIsPostgres(pid int) bool {
	if b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/comm"); err == nil {
		return strings.HasPrefix(string(b), "postgres")
	}
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=").Output()
	if err != nil {
		return len(out) > 0 || !errors.As(err, new(*exec.ExitError))
	}
	return strings.Contains(string(out), "postgres")
}

func pgWaitReady(ctx context.Context, dsn string, done chan error, dataDir string) error {
	deadline := time.Now().Add(time.Minute)
	for {
		c, err := pgx.Connect(ctx, dsn)
		if err == nil {
			return c.Close(ctx)
		}
		select {
		case err := <-done:
			done <- err
			return fmt.Errorf("postgres exited during startup (%v); see %s", err, filepath.Join(dataDir, "postgres.log"))
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("postgres not ready after a minute: %w", err)
		}
	}
}

func pgEnsureDB(ctx context.Context, dsn string) error {
	c, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return err
	}
	defer c.Close(ctx)
	q := New(c)
	ok, err := q.DatabaseExists(ctx)
	if err == nil && !ok {
		err = q.CreateDatabase(ctx)
	}
	return err
}

func pgQuote(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'"
}
