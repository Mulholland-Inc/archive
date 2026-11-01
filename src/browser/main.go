// Command browser serves one fresh headless Chrome per WebSocket connection, for archive.browse.
// Each connection is proxied to that Chrome's DevTools socket; Chrome and its profile go when it closes.
package main

import (
	"bufio"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

func main() {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "ok") })
	http.HandleFunc("/", serve)
	addr := ":" + cmp.Or(os.Getenv("PORT"), "8080")
	log.Printf("browser: listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func serve(w http.ResponseWriter, r *http.Request) {
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		http.Error(w, "want a websocket", http.StatusBadRequest)
		return
	}
	dir, err := os.MkdirTemp("", "chrome")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(dir)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	cmd := exec.CommandContext(ctx, cmp.Or(os.Getenv("CHROME"), "chromium"), "--headless=new", "--no-sandbox", "--disable-dev-shm-usage",
		"--disable-gpu", "--no-first-run", "--no-default-browser-check", "--disable-extensions", "--disable-background-networking",
		"--remote-debugging-address=127.0.0.1", "--remote-debugging-port=0", "--window-size=1280,800", "--user-data-dir="+dir+"/profile", "about:blank")
	cmd.Env = append(os.Environ(), "TMPDIR="+dir, "HOME="+dir)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 5 * time.Second
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cmd.Wait()
	defer cancel()
	devtools, err := listening(stderr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	go io.Copy(io.Discard, stderr)
	chrome, err := net.Dial("tcp", devtools.Host)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer chrome.Close()
	req := r.Clone(ctx)
	req.URL, req.Host, req.RequestURI = devtools, devtools.Host, ""
	for _, h := range []string{"Origin", "Authorization", "X-Serverless-Authorization", "Forwarded", "X-Forwarded-For", "X-Forwarded-Proto", "X-Forwarded-Host"} {
		req.Header.Del(h)
	}
	if err := req.Write(chrome); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	conn, buf, err := http.NewResponseController(w).Hijack()
	if err != nil {
		return
	}
	defer conn.Close()
	start := time.Now()
	done := make(chan struct{}, 2)
	go func() { io.Copy(chrome, buf); done <- struct{}{} }()
	go func() { io.Copy(conn, chrome); done <- struct{}{} }()
	<-done
	log.Printf("browser: session closed after %s", time.Since(start).Round(time.Millisecond))
}

func listening(stderr io.Reader) (*url.URL, error) {
	sc := bufio.NewScanner(stderr)
	found := make(chan *url.URL, 1)
	go func() {
		defer close(found)
		for sc.Scan() {
			if _, ws, ok := strings.Cut(sc.Text(), "DevTools listening on "); ok {
				if u, err := url.Parse(strings.TrimSpace(ws)); err == nil {
					found <- u
					return
				}
			}
		}
	}()
	select {
	case u, ok := <-found:
		if !ok {
			return nil, errors.New("chrome exited before it listened")
		}
		return u, nil
	case <-time.After(20 * time.Second):
		return nil, fmt.Errorf("chrome did not start in 20s")
	}
}
