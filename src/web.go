package archive

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"strings"
)

//go:embed all:web
var webFiles embed.FS

const webCSP = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; " +
	"media-src 'self' blob:; frame-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'"

func webHandler() http.Handler {
	dist, _ := fs.Sub(webFiles, "web/dist")
	index, indexErr := fs.ReadFile(dist, "index.html")
	files := http.FileServerFS(dist)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			httpWrite(w, 0, nil, fmt.Errorf("%s %s: %w", r.Method, r.URL.Path, ErrNotFound))
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p != "" && p != "index.html" {
			if fi, err := fs.Stat(dist, p); err == nil && !fi.IsDir() {
				if strings.HasPrefix(p, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			}
			if path.Ext(p) != "" {
				http.NotFound(w, r)
				return
			}
		}
		if indexErr != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			io.WriteString(w, "This archive binary was built without the web dashboard; run npm run build in apps/web, then build it again.\n")
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Security-Policy", webCSP)
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Write(index)
	})
}

func webBrowser(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}

func webRedirect(w http.ResponseWriter, r *http.Request, to string, err error) {
	if err != nil {
		sep := "?"
		if strings.Contains(to, "?") {
			sep = "&"
		}
		to += sep + "error=" + url.QueryEscape(err.Error())
	}
	http.Redirect(w, r, to, http.StatusSeeOther)
}
