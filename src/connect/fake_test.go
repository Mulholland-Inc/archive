package connect

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"golang.org/x/oauth2"
)

// A fake is a provider's API as a test serves it. A connector's variable naming another host moves every URL of
// the connector onto it (Provider.at), so the fake gets each request by its path and query, as the provider
// would, and answers it with the handler the test gave. It is TLS, as some connectors build https URLs of their
// own, like Shopify's store address; the client trusts it through http.DefaultTransport, which the OAuth and key
// transports use. It speaks HTTP/2, as Graph does, which takes a request line HTTP/1.1 refuses, like the raw
// space in Teams' $orderby.
type fake struct {
	*httptest.Server
	mu   sync.Mutex
	reqs []*fakeReq
}

// fakeReq is a request the fake answered, with its body read.
type fakeReq struct {
	Method, Path string
	Query        url.Values
	Body         string
}

// newFake serves h for the test, and is the fake.
func newFake(t *testing.T, h http.HandlerFunc) *fake {
	t.Helper()
	f := &fake{}
	f.Server = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(strings.NewReader(string(b)))
		f.mu.Lock()
		f.reqs = append(f.reqs, &fakeReq{Method: r.Method, Path: r.URL.Path, Query: r.URL.Query(), Body: string(b)})
		f.mu.Unlock()
		h(w, r)
	}))
	f.EnableHTTP2 = true
	f.StartTLS()
	t.Cleanup(f.Close)
	was := http.DefaultTransport
	http.DefaultTransport = f.Client().Transport
	t.Cleanup(func() { http.DefaultTransport = was })
	return f
}

// host is the fake's host and port, like 127.0.0.1:53211.
func (f *fake) host() string { return strings.TrimPrefix(f.URL, "https://") }

// requests are the requests the fake has answered so far.
func (f *fake) requests() []*fakeReq {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*fakeReq(nil), f.reqs...)
}

// open is a Client of connector id on the fake, as Load and Open make one from a variable that names it: an OAuth
// connector's with an access token, a key connector's with keys, and either with meta as its grant's Meta. query
// is the variable's options, like sandbox=1.
func (f *fake) open(t *testing.T, id, query string, keys, meta map[string]string) *Client {
	t.Helper()
	c := Get(id)
	if c == nil {
		t.Fatalf("there is no connector %q", id)
	}
	env := map[string]string{}
	if c.Env != "" {
		v := "https://id:secret@" + f.host()
		if query != "" {
			v += "?" + query
		}
		env[c.Env] = v
	}
	ps, err := Load(func(k string) string { return env[k] })
	if err != nil {
		t.Fatal(err)
	}
	p, err := ps.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	tok := &oauth2.Token{AccessToken: "token"}
	if len(c.Keys) > 0 {
		b, _ := json.Marshal(keys)
		tok.AccessToken = string(b)
	}
	cl := p.Open(context.Background(), tok, meta)
	t.Cleanup(cl.Close)
	return cl
}

// stream is connector id's stream name.
func stream(t *testing.T, id, name string) *Stream {
	t.Helper()
	for _, s := range Get(id).Streams {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("%s has no stream %q", id, name)
	return nil
}

// pull is every page of s from cursor until one has no more to come, and the cursor it ended with.
func pull(t *testing.T, c *Client, s *Stream, cursor string) ([]*Page, string) {
	t.Helper()
	var pages []*Page
	for range 100 {
		p, err := s.Pull(c, cursor)
		if err != nil {
			t.Fatalf("%s: %v", s.Name, err)
		}
		pages = append(pages, p)
		cursor = p.Cursor
		if !p.More {
			return pages, cursor
		}
	}
	t.Fatalf("%s: still more after 100 pages", s.Name)
	return nil, ""
}

// ids are the ids of the records of pages, in order, those deleted with a leading -.
func ids(pages ...*Page) []string {
	var out []string
	for _, p := range pages {
		for _, r := range p.Records {
			if r.Deleted {
				out = append(out, "-"+r.ID)
			} else {
				out = append(out, r.ID)
			}
		}
	}
	return out
}

// writeJSON answers v as JSON.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
