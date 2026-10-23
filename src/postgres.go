package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
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
		initdb, err := pgBin("initdb")
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

	postgres, err := pgBin("postgres")
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

func pgBin(name string) (string, error) {
	if dir := os.Getenv("ARCHIVE_PG_BIN"); dir != "" {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("%s not found in $ARCHIVE_PG_BIN: %w", name, err)
		}
		return p, nil
	}
	p, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%s not found on PATH (set $ARCHIVE_PG_BIN): %w", name, err)
	}
	return p, nil
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
	var ok bool
	if err := c.QueryRow(ctx, `SELECT EXISTS (SELECT FROM pg_database WHERE datname = 'archive')`).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		_, err = c.Exec(ctx, `CREATE DATABASE archive`)
	}
	return err
}

func pgQuote(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'"
}
