// Command browser serves one fresh headless Chrome per WebSocket connection, for archive.browse.
// Each connection is proxied to that Chrome's DevTools socket; Chrome and its profile go when it closes.
// The next Chrome starts as soon as the current one is ready, so a connection rarely waits for one.
package main

import (
	"bufio"
	"cmp"
	"errors"
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

var started = time.Now()

type chrome struct {
	cmd   *exec.Cmd
	dir   string
	ws    *url.URL
	err   error
	ready chan struct{}
}

var next = make(chan *chrome, 1)

func main() {
	next <- launch()
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "ok") })
	http.HandleFunc("/", serve)
	addr := ":" + cmp.Or(os.Getenv("PORT"), "8080")
	log.Printf("browser: listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

// launch starts Chrome; ready closes once its DevTools address is known or it failed.
func launch() *chrome {
	start := time.Now()
	c := &chrome{ready: make(chan struct{})}
	if c.dir, c.err = os.MkdirTemp("", "chrome"); c.err != nil {
		close(c.ready)
		return c
	}
	c.cmd = exec.Command(cmp.Or(os.Getenv("CHROME"), "chromium"), "--headless=new", "--no-sandbox", "--no-zygote", "--disable-dev-shm-usage",
		"--disable-gpu", "--no-first-run", "--no-default-browser-check", "--disable-extensions", "--disable-background-networking",
		"--disable-component-update", "--disable-sync", "--disable-default-apps", "--disable-breakpad", "--mute-audio",
		"--disable-features=Translate,MediaRouter,OptimizationHints",
		"--remote-debugging-address=127.0.0.1", "--remote-debugging-port=0", "--window-size=1280,800", "--user-data-dir="+c.dir+"/profile", "about:blank")
	c.cmd.Env = append(os.Environ(), "TMPDIR="+c.dir, "HOME="+c.dir)
	c.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stderr, _ := c.cmd.StderrPipe()
	if c.err = c.cmd.Start(); c.err != nil {
		close(c.ready)
		return c
	}
	go func() {
		defer close(c.ready)
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			if _, ws, ok := strings.Cut(sc.Text(), "DevTools listening on "); ok {
				if c.ws, c.err = url.Parse(strings.TrimSpace(ws)); c.err == nil {
					log.Printf("browser: chrome ready in %s, %s after the process started", time.Since(start).Round(time.Millisecond), time.Since(started).Round(time.Millisecond))
					go io.Copy(io.Discard, stderr)
					return
				}
			}
		}
		c.err = errors.New("chrome exited before it listened")
	}()
	return c
}

func (c *chrome) close() {
	if c.cmd != nil && c.cmd.Process != nil {
		syscall.Kill(-c.cmd.Process.Pid, syscall.SIGKILL)
		c.cmd.Wait()
	}
	os.RemoveAll(c.dir)
}

// take returns a ready Chrome: the waiting one, or a new one when that failed.
func take(r *http.Request) (*chrome, net.Conn, error) {
	var c *chrome
	select {
	case c = <-next:
	case <-r.Context().Done():
		return nil, nil, r.Context().Err()
	}
	for try := 0; ; try++ {
		var err error
		select {
		case <-c.ready:
			err = c.err
		case <-time.After(20 * time.Second):
			err = errors.New("chrome did not start in 20s")
		case <-r.Context().Done():
			next <- c
			return nil, nil, r.Context().Err()
		}
		var cdp net.Conn
		if err == nil {
			cdp, err = net.Dial("tcp", c.ws.Host)
		}
		if err == nil || try == 1 {
			go func() { next <- launch() }()
			return c, cdp, err
		}
		log.Printf("browser: the waiting chrome failed (%v); starting another", err)
		c.close()
		c = launch()
	}
}

func serve(w http.ResponseWriter, r *http.Request) {
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		http.Error(w, "want a websocket", http.StatusBadRequest)
		return
	}
	asked := time.Now()
	c, cdp, err := take(r)
	if c != nil {
		defer c.close()
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer cdp.Close()
	wait := time.Since(asked)
	req := r.Clone(r.Context())
	req.URL, req.Host, req.RequestURI = c.ws, c.ws.Host, ""
	for _, h := range []string{"Origin", "Authorization", "X-Serverless-Authorization", "Forwarded", "X-Forwarded-For", "X-Forwarded-Proto", "X-Forwarded-Host"} {
		req.Header.Del(h)
	}
	if err := req.Write(cdp); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	conn, buf, err := http.NewResponseController(w).Hijack()
	if err != nil {
		return
	}
	defer conn.Close()
	log.Printf("browser: session started, %s waiting for chrome", wait.Round(time.Millisecond))
	start := time.Now()
	done := make(chan struct{}, 2)
	go func() { io.Copy(cdp, buf); done <- struct{}{} }()
	go func() { io.Copy(conn, cdp); done <- struct{}{} }()
	<-done
	log.Printf("browser: session closed after %s", time.Since(start).Round(time.Millisecond))
}
