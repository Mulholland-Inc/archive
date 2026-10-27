//go:build !js

package main

import (
	"cmp"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func main() {
	target, err := url.Parse(cmp.Or(os.Getenv("ARCHIVE_URL"), "http://localhost:8080"))
	if err != nil {
		log.Fatal(err)
	}
	token := os.Getenv("ARCHIVE_TOKEN")
	if token == "" {
		log.Fatal("set ARCHIVE_TOKEN to a token from: archive token create")
	}
	dir, err := os.MkdirTemp("", "archive-editor-")
	if err != nil {
		log.Fatal(err)
	}
	build := exec.Command("go", "build", "-o", filepath.Join(dir, "editor.wasm"), ".")
	build.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err := build.Run(); err != nil {
		log.Fatal(err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.FlushInterval = -1
	mux := http.NewServeMux()
	mux.Handle("/api/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("Authorization", "Bearer "+token)
		proxy.ServeHTTP(w, r)
	}))
	mux.HandleFunc("GET /editor.wasm", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, filepath.Join(dir, "editor.wasm")) })
	mux.HandleFunc("GET /wasm_exec.js", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(runtime.GOROOT(), "lib", "wasm", "wasm_exec.js"))
	})
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "index.html") })
	addr := ":" + cmp.Or(os.Getenv("PORT"), "5180")
	log.Printf("editor on http://localhost%s, proxying %s", addr, target)
	log.Fatal(http.ListenAndServe(addr, mux))
}
