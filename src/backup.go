package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const backupKeep = 3

func BackupBeforeUpgrade(ctx context.Context, dataDir, version string) error {
	verFile := filepath.Join(dataDir, "version")
	old, err := os.ReadFile(verFile)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	oldVer := strings.TrimSpace(string(old))
	if oldVer == version {
		return nil
	}
	pgDir := filepath.Join(dataDir, "pg")
	if _, err := os.Stat(filepath.Join(pgDir, "PG_VERSION")); err == nil {
		if oldVer == "" {
			oldVer = "unknown"
		}
		dir := filepath.Join(dataDir, "backups")
		name := time.Now().UTC().Format("20060102T150405Z") + "-" + backupSafe(oldVer) + ".tar.gz"
		// the key goes with the database: its secret values cannot be read without it
		srcs := []string{pgDir}
		if key := filepath.Join(dataDir, "secret.key"); backupExists(key) {
			srcs = append(srcs, key)
		}
		if err := backupTar(ctx, srcs, dir, name); err != nil {
			return fmt.Errorf("backup before upgrade from %s: %w", oldVer, err)
		}
		if err := backupPrune(dir, backupKeep); err != nil {
			return err
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}
	tmp := verFile + ".tmp"
	if err := os.WriteFile(tmp, []byte(version+"\n"), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, verFile)
}

func backupTar(ctx context.Context, srcs []string, dir, name string) (err error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".partial-*")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			f.Close()
			os.Remove(f.Name())
		}
	}()
	zw := gzip.NewWriter(f)
	tw := tar.NewWriter(zw)
	for _, src := range srcs {
		if err := backupWalk(ctx, tw, src); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(dir, name))
}

func backupPrune(dir string, keep int) error {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var names []string
	for _, e := range ents {
		if !e.IsDir() && !strings.HasPrefix(e.Name(), ".") && strings.HasSuffix(e.Name(), ".tar.gz") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for len(names) > keep {
		if err := os.Remove(filepath.Join(dir, names[0])); err != nil {
			return err
		}
		names = names[1:]
	}
	return nil
}

func backupSafe(v string) string {
	var b bytes.Buffer
	for _, r := range v {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_', r == '+':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

// backupWalk adds src, a file or a folder, to tw under its name relative to src's parent.
func backupWalk(ctx context.Context, tw *tar.Writer, src string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		var link string
		switch {
		case info.Mode()&fs.ModeSymlink != 0:
			if link, err = os.Readlink(path); err != nil {
				return err
			}
		case !info.Mode().IsRegular() && !info.IsDir():
			return nil
		}
		hdr, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(filepath.Dir(src), path)
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if info.IsDir() {
			hdr.Name += "/"
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		r, err := os.Open(path)
		if err != nil {
			return err
		}
		defer r.Close()
		_, err = io.CopyN(tw, r, info.Size())
		return err
	})
}

func backupExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// backupBuild names this exact binary, so BackupBeforeUpgrade sees every new image even when nobody
// bumps cliVersion: the version plus a short hash of the executable.
func backupBuild() string {
	exe, err := os.Executable()
	if err != nil {
		return cliVersion
	}
	f, err := os.Open(exe)
	if err != nil {
		return cliVersion
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return cliVersion
	}
	return cliVersion + "+" + hex.EncodeToString(h.Sum(nil))[:12]
}

// Backup writes the whole archive to w as a gzipped tar, while it serves or not: archive.sql, a
// pg_dumpall of the database and its roles (row-level security lives in them); secret.key, without
// which no secret value can be read; and blobs/, the stored files, which never change once written.
// Restore reads it back into an empty data directory.
func Backup(ctx context.Context, dataDir string, w io.Writer) error {
	if os.Getenv("DATABASE_URL") != "" {
		return errors.New("backup covers the bundled Postgres; back up an external DATABASE_URL with its provider's tools")
	}
	dataDir, err := filepath.Abs(dataDir)
	if err != nil {
		return err
	}
	if !backupExists(filepath.Join(dataDir, "pg", "PG_VERSION")) {
		return fmt.Errorf("no archive in %s", dataDir)
	}
	if pgLive(dataDir) == "" {
		_, stop, err := StartPostgres(ctx, dataDir)
		if err != nil {
			return err
		}
		defer stop()
	}
	sock, err := pgSocketDir(dataDir)
	if err != nil {
		return err
	}
	dump, err := os.CreateTemp(dataDir, ".backup-*.sql")
	if err != nil {
		return err
	}
	defer os.Remove(dump.Name())
	defer dump.Close()
	bin, err := pgBin(ctx, dataDir, "pg_dumpall")
	if err != nil {
		return err
	}
	// no passwords: the only one, the app's login role, is set again from the key on every start
	cmd := exec.CommandContext(ctx, bin, "-h", sock, "-U", "archive", "--no-role-passwords")
	var stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr, cmd.Env = dump, &stderr, pgEnv()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_dumpall: %w\n%s", err, stderr.Bytes())
	}
	zw := gzip.NewWriter(w)
	tw := tar.NewWriter(zw)
	add := func(name, path string) error {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: info.Size(), ModTime: info.ModTime()}); err != nil {
			return err
		}
		_, err = io.CopyN(tw, f, info.Size())
		return err
	}
	if err := add("archive.sql", dump.Name()); err != nil {
		return err
	}
	for _, name := range []string{"secret.key", "version"} {
		if p := filepath.Join(dataDir, name); backupExists(p) {
			if err := add(name, p); err != nil {
				return err
			}
		}
	}
	if blobs := filepath.Join(dataDir, "blobs"); backupExists(blobs) {
		if err := backupWalk(ctx, tw, blobs); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return zw.Close()
}

// Restore unpacks a Backup into dataDir, which must not hold a database yet, and loads archive.sql
// into a fresh bundled Postgres. Run it with the server stopped, then start the server as usual.
func Restore(ctx context.Context, dataDir string, r io.Reader) error {
	if os.Getenv("DATABASE_URL") != "" {
		return errors.New("restore loads the bundled Postgres; unset DATABASE_URL")
	}
	dataDir, err := filepath.Abs(dataDir)
	if err != nil {
		return err
	}
	if backupExists(filepath.Join(dataDir, "pg", "PG_VERSION")) {
		return fmt.Errorf("%s already holds an archive; restore into an empty data directory", dataDir)
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}
	zr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("not an archive backup: %w", err)
	}
	tr := tar.NewReader(zr)
	sqlPath := filepath.Join(dataDir, ".restore.sql")
	defer os.Remove(sqlPath)
	var sawSQL bool
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("not an archive backup: %w", err)
		}
		name := strings.TrimSuffix(hdr.Name, "/")
		if !fs.ValidPath(name) {
			return fmt.Errorf("backup holds an unsafe path %q", hdr.Name)
		}
		var dest string
		switch {
		case name == "archive.sql":
			dest, sawSQL = sqlPath, true
		case name == "secret.key", name == "version", name == "blobs", strings.HasPrefix(name, "blobs/"):
			dest = filepath.Join(dataDir, filepath.FromSlash(name))
		default:
			continue
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(dest, 0o700); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
				return err
			}
			f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
			if err != nil {
				return err
			}
			_, err = io.Copy(f, tr)
			if err == nil {
				err = f.Sync()
			}
			if cerr := f.Close(); err == nil {
				err = cerr
			}
			if err != nil {
				return err
			}
		}
	}
	if !sawSQL {
		return errors.New("not an archive backup: no archive.sql in it")
	}
	_, stop, err := StartPostgres(ctx, dataDir)
	if err != nil {
		return err
	}
	defer stop()
	sock, err := pgSocketDir(dataDir)
	if err != nil {
		return err
	}
	bin, err := pgBin(ctx, dataDir, "psql")
	if err != nil {
		return err
	}
	// the dump creates the archive role that initdb made too; that one error is expected, so psql
	// goes on and the database's presence is checked after
	cmd := exec.CommandContext(ctx, bin, "-X", "-q", "-h", sock, "-U", "archive", "-d", "postgres", "-f", sqlPath)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr, cmd.Env = &out, &out, pgEnv()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("psql: %w\n%s", err, out.Bytes())
	}
	check := exec.CommandContext(ctx, bin, "-X", "-At", "-h", sock, "-U", "archive", "-d", "archive", "-c", "SELECT count(*) FROM users")
	check.Env = pgEnv()
	if b, err := check.CombinedOutput(); err != nil {
		return fmt.Errorf("restored database does not answer: %w\n%s\n%s", err, b, out.Bytes())
	}
	return nil
}
