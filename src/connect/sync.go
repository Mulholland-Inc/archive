package connect

import (
	"cmp"
	"context"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Stream struct {
	Name                string
	Doc                 string
	Gap                 time.Duration
	Every               time.Duration
	Pull                func(c *Client, cursor string) (*Page, error)
	Table, SQL, Version string
	Read                func(c *Client, source map[string]any) (*Bytes, error)
}

const MaxCopy = 1 << 30

type Bytes struct {
	Body io.ReadCloser
	Size int64
}

type Page struct {
	Records []Record
	Cursor  string
	More    bool
	Full    bool
}

type Record struct {
	ID      string
	Data    any
	Updated time.Time
	Deleted bool
	Same    bool
}

type Unsupported struct{ Reason string }

func (u *Unsupported) Error() string { return u.Reason }

func unsupported(format string, a ...any) error {
	return &Unsupported{Reason: fmt.Sprintf(format, a...)}
}

func cursorOf(cursor string, v any) error {
	if cursor == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(cursor), v); err != nil {
		return fmt.Errorf("the sync cursor %q does not read: %v", cursor, err)
	}
	return nil
}

func cursorFor(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func each(n, par int, f func(i int) error) error {
	var first error
	var mu sync.Mutex
	failed := func() bool {
		mu.Lock()
		defer mu.Unlock()
		return first != nil
	}
	sem := make(chan struct{}, par)
	var wg sync.WaitGroup
	for i := range n {
		sem <- struct{}{}
		if failed() {
			break
		}
		wg.Add(1)
		go func() {
			defer func() { <-sem; wg.Done() }()
			if err := f(i); err != nil {
				mu.Lock()
				defer mu.Unlock()
				if first == nil {
					first = err
				}
			}
		}()
	}
	wg.Wait()
	return first
}

func stamp(s string) time.Time {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999-0700"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	var ms int64
	if _, err := fmt.Sscan(s, &ms); err == nil && ms > 1e11 {
		return time.UnixMilli(ms)
	}
	return time.Time{}
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && n < len(s) && s[n]&0xC0 == 0x80 {
		n--
	}
	return s[:n]
}

func clamp(n, def, most int) int {
	if n <= 0 {
		return def
	}
	return min(n, most)
}

const maxText = 64 << 10

var tagRe = regexp.MustCompile(`(?s)<(script|style)\b.*?</(script|style)>|<[^>]*>`)

func htmlText(s string) string {
	s = tagRe.ReplaceAllString(s, " ")
	s = strings.NewReplacer("&nbsp;", " ", "&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`, "&#39;", "'").Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

var Extract func(name, mime string, b []byte) string

const maxFile = 25 << 20

func textual(name, mime string) bool {
	mime = strings.ToLower(mime)
	switch ext := strings.ToLower(name[strings.LastIndex(name, ".")+1:]); {
	case strings.HasPrefix(mime, "text/"), mime == "application/json", mime == "application/pdf", strings.Contains(mime, "openxmlformats-officedocument"),
		strings.Contains(mime, "macroenabled"):
		return true
	case strings.Contains(name, "."):
		return slices.Contains([]string{"pdf", "docx", "docm", "xlsx", "xlsm", "pptx", "pptm", "txt", "csv", "md", "html", "json"}, ext)
	}
	return false
}

func bytesText(name, mime string, b []byte) string {
	if Extract == nil || b == nil || !textual(name, mime) {
		return ""
	}
	return clip(strings.ToValidUTF8(Extract(name, mime, b), ""), maxText)
}

func (c *Client) keep(name string, b []byte) (string, error) {
	if c.Store == nil || b == nil {
		return "", nil
	}
	return c.Store(name, b)
}

func (c *Client) fileText(u, name, mime string, size int64) (string, error) {
	if Extract == nil || !textual(name, mime) || size > maxFile {
		return "", nil
	}
	b, err := c.fileBytes(u, maxFile)
	if err != nil || b == nil {
		return "", err
	}
	return bytesText(name, mime, b), nil
}

func (c *Client) fileBytes(u string, limit int64) ([]byte, error) {
	req, err := request(c.ctx, "GET", c.p.at(u), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "*/*")
	hc := *c.http
	hc.Timeout = 2 * time.Minute
	resp, err := hc.Do(req)
	if err != nil {
		if c.ctx.Err() != nil {
			return nil, c.ctx.Err()
		}
		return nil, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		if r := refusal(c.p, resp); Busy(r) {
			return nil, r
		}
		return nil, nil
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil || int64(len(b)) > limit {
		return nil, nil
	}
	return b, nil
}

func (c *Client) opened(ctx context.Context, r Req, sc map[string]any, body io.Reader, n int64, bare bool) (*http.Response, error) {
	u, err := c.urlOf(r.URL, sc)
	if err != nil {
		return nil, err
	}
	return c.send(ctx, r, u, sc, body, n, bare)
}

func (c *Client) send(ctx context.Context, r Req, u string, sc map[string]any, body io.Reader, n int64, bare bool) (*http.Response, error) {
	if q := values(r.Query, sc); len(q) > 0 {
		u += sepOf(u) + q.Encode()
	}
	var v any
	if body == nil && r.Body != nil {
		v, _ = fill(r.Body, sc)
	}
	req, err := request(ctx, string(cmp.Or(r.Method, GET)), c.p.at(u), v)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Body, req.ContentLength = io.NopCloser(body), n
	}
	req.Header.Set("Accept", "*/*")
	for h, vs := range r.headers(sc) {
		req.Header[h] = vs
	}
	hc := http.Client{}
	if !bare {
		hc.Transport = c.http.Transport
	}
	resp, err := hc.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, unreached{fmt.Errorf("%s could not be reached: %w", c.p.Name, err)}
	}
	return resp, nil
}

func (s *Sync) bytesOf(c *Client, source map[string]any) (*Bytes, error) {
	for _, k := range s.Content {
		if k.If.At != "" && !k.If.holds(source) {
			continue
		}
		if n, _ := strconv.ParseInt(text(pick(source, k.Size)), 10, 64); n > MaxCopy {
			return nil, unsupported("it is over the %d GB Archive copies", MaxCopy>>30)
		}
		resp, err := c.opened(c.ctx, k.Req, map[string]any{"source": source}, nil, 0, false)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode/100 != 2 {
			defer resp.Body.Close()
			r := refusal(c.p, resp)
			for _, d := range k.Too {
				if d.is(r) {
					why, _ := fillStr(d.Why, map[string]any{"message": r.Message})
					return nil, unsupported("%s", text(why))
				}
			}
			return nil, r
		}
		if resp.ContentLength > MaxCopy {
			resp.Body.Close()
			return nil, unsupported("it is over the %d GB Archive copies", MaxCopy>>30)
		}
		return &Bytes{Body: resp.Body, Size: resp.ContentLength}, nil
	}
	return nil, unsupported("%s has no bytes of it to copy", c.p.Name)
}

func jsonASCII(b []byte) string {
	var out strings.Builder
	for _, r := range string(b) {
		switch {
		case r < 0x7f:
			out.WriteRune(r)
		case r < 0x10000:
			fmt.Fprintf(&out, "\\u%04x", r)
		default:
			r -= 0x10000
			fmt.Fprintf(&out, "\\u%04x\\u%04x", 0xd800+(r>>10), 0xdc00+(r&0x3ff))
		}
	}
	return out.String()
}

type Yours interface {
	Open(id string) (*Upfile, error)
	Row(source string) (*Synced, error)
	Until() time.Time
}

type Upfile struct {
	Name, Mime string
	Size       int64
	Body       io.ReadSeekCloser
}

type Synced struct {
	Kind, Version string
	Source        map[string]any
}

type upInput struct {
	File    string `json:"file" jsonschema:"The id in the file table of the file to send"`
	Folder  string `json:"folder,omitempty" jsonschema:"The source_id in the file table of the folder it goes in; the top when left out"`
	Name    string `json:"name,omitempty" jsonschema:"Its name there; the file's own when left out"`
	Replace string `json:"replace,omitempty" jsonschema:"The source_id in the file table of the file it replaces, which it refuses to do if that file changed there since Archive last synced it"`
}

func Upload(s Send) *Action {
	in, err := jsonschema.For[upInput](nil)
	if err != nil {
		panic(err)
	}
	return &Action{Doc: s.Doc, Input: in, Output: map[string]any{}, Writes: true, Asks: true,
		Do: func(c *Client, raw json.RawMessage) (any, error) { return s.run(c, raw) }}
}

func (s *Send) stale(c *Client, r *Refusal, sc map[string]any) error {
	for _, d := range s.Stale {
		if d.is(r) {
			why, _ := fillStr(d.Why, map[string]any{"message": r.Message, "name": sc["name"]})
			return &Refusal{Provider: c.p.Name, Status: http.StatusConflict, Message: text(why)}
		}
	}
	if r.Status == http.StatusForbidden {
		r.Message = strings.TrimSpace(r.Message + " If it was connected before Archive could write there, connect it again in Settings, Connectors, to allow that.")
	}
	return r
}

func (s *Send) run(c *Client, raw json.RawMessage) (any, error) {
	var in upInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, err
	}
	up, err := c.Files.Open(in.File)
	if err != nil {
		return nil, err
	}
	defer up.Body.Close()
	if up.Size == 0 {
		return nil, fmt.Errorf("%s is empty, and %s takes no upload without bytes", up.Name, c.p.Name)
	}
	ctx, cancel := context.WithDeadline(c.ctx, c.Files.Until().Add(-2*time.Second))
	defer cancel()
	sc := map[string]any{"name": cmp.Or(in.Name, up.Name), "mime": up.Mime, "size": int(up.Size)}
	if in.Name != "" {
		sc["rename"] = in.Name
	}
	open := s.New
	if in.Folder != "" {
		row, err := c.Files.Row(in.Folder)
		if err != nil {
			return nil, err
		}
		if row.Kind != "folder" {
			return nil, fmt.Errorf("folder %s is a %s here, not a folder", in.Folder, row.Kind)
		}
		sc["in"] = row.Source
		if s.In != nil {
			open = *s.In
		}
	}
	if in.Replace != "" {
		row, err := c.Files.Row(in.Replace)
		if err != nil {
			return nil, err
		}
		if row.Kind != "file" || row.Version == "" {
			return nil, fmt.Errorf("replace %s is no file Archive has a version of; wait for a sync and look again", in.Replace)
		}
		sc["it"], sc["version"], open = row.Source, row.Version, s.Over
		if s.Check != nil {
			ans, err := c.req(*s.Check, sc)
			if err != nil {
				return nil, err
			}
			now, _ := fillStr(s.Now, map[string]any{"ans": ans})
			if text(now) != row.Version {
				return nil, &Refusal{Provider: c.p.Name, Status: http.StatusConflict, Message: fmt.Sprintf(
					"%s changed there since Archive last synced it, so it was not replaced: look at the newer version once it syncs, and upload again if yours should still replace it", sc["name"])}
			}
		}
	}
	if _, err := runSteps(c, s.Let, sc); err != nil {
		return nil, err
	}
	resp, err := c.opened(ctx, open, sc, nil, 0, false)
	if err != nil {
		return nil, s.late(err, 0, up)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, s.stale(c, refusal(c.p, resp), sc)
	}
	var session string
	if s.AtHeader != "" {
		session = resp.Header.Get(s.AtHeader)
	} else {
		b, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
		if err != nil {
			return nil, err
		}
		ans, err := readAll(b, c.p.Name)
		if err != nil {
			return nil, err
		}
		session = text(pick(ans, s.At))
	}
	if session == "" {
		return nil, fmt.Errorf("%s opened no upload", c.p.Name)
	}
	sc["session"] = session
	var out any
	for off := int64(0); off < up.Size; {
		n := min(s.Step, up.Size-off)
		last := off+n == up.Size
		if _, err := up.Body.Seek(off, io.SeekStart); err != nil {
			return nil, err
		}
		sc["offset"], sc["length"], sc["range"] = int(off), int(n), fmt.Sprintf("bytes %d-%d/%d", off, off+n-1, up.Size)
		r := s.Part
		if last && s.Last != nil {
			r = *s.Last
		}
		var resp *http.Response
		if r.URL == "" {
			resp, err = c.send(ctx, r, session, sc, io.LimitReader(up.Body, n), n, s.Bare)
		} else {
			resp, err = c.opened(ctx, r, sc, io.LimitReader(up.Body, n), n, s.Bare)
		}
		if err != nil {
			return nil, s.late(err, off, up)
		}
		if resp.StatusCode/100 != 2 && resp.StatusCode != http.StatusPermanentRedirect {
			defer resp.Body.Close()
			return nil, s.stale(c, refusal(c.p, resp), sc)
		}
		if last {
			b, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
			if err == nil && len(b) > 0 {
				out, err = readAll(b, c.p.Name)
			}
			if err != nil {
				resp.Body.Close()
				return nil, err
			}
		}
		resp.Body.Close()
		off += n
	}
	return out, nil
}

func (s *Send) late(err error, sent int64, up *Upfile) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("the run's time ran out with %d of %s's %d bytes sent, and nothing changed there: a run has 30 seconds, so send a smaller file", sent, up.Name, up.Size)
	}
	return err
}

func (c *Client) tool(name string) (*mcp.Tool, error) {
	if c.tools == nil {
		ts, err := c.Tools()
		if err != nil {
			return nil, err
		}
		c.tools = ts
	}
	for _, t := range c.tools {
		if t.Name == name {
			return t, nil
		}
	}
	return nil, unsupported("%s's MCP server has no tool %s", c.p.Name, name)
}

func takes(t *mcp.Tool) (map[string]bool, []string) {
	var s struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	b, _ := json.Marshal(t.InputSchema)
	if json.Unmarshal(b, &s) != nil || len(s.Properties) == 0 {
		return nil, s.Required
	}
	out := map[string]bool{}
	for k := range s.Properties {
		out[k] = true
	}
	return out, s.Required
}

type Row struct {
	Cols               map[string]any
	People, Orgs, Refs map[string]string
	Children           []Child
}

type Child struct {
	Table string
	Row
}

var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+$`)

func Email(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if !emailRe.MatchString(s) {
		return ""
	}
	return s
}

func addresses(header string) []mail.Address {
	if strings.TrimSpace(header) == "" {
		return nil
	}
	list, err := (&mail.AddressParser{}).ParseList(header)
	if err != nil {
		if a, err := mail.ParseAddress(header); err == nil {
			list = []*mail.Address{a}
		}
	}
	var out []mail.Address
	for _, a := range list {
		if e := Email(a.Address); e != "" {
			out = append(out, mail.Address{Address: e, Name: a.Name})
		}
	}
	return out
}

type Push struct {
	Watch   func(c *Client, h Hook, old []Watch) ([]Watch, error)
	Unwatch func(c *Client, w Watch) error
	Renew   time.Duration
	Env     string
	OIDC    bool
	Hear    func(p *Provider, r *http.Request, at string, body []byte, secret func(key string) string) (*Heard, error)
}

type Hook struct {
	ID, Account string
	URL         func(key string) string
	Target      string
}

type Watch struct {
	Key, Resource, Secret string
	Expires               time.Time
}

type Heard struct {
	Keys   []string
	Status int
	Header http.Header
	Body   string
	Keep   map[string]string
	All    bool
	Forget map[string][]string
}

var ErrForged = errors.New("not the provider's")

func forged(format string, a ...any) error {
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, a...), ErrForged)
}

func (p *Provider) PushOf() (*Push, string) { return p.Push, p.pushTo }

func (p *Provider) HookSecret() string { return p.opts.Get("hook") }

func same(a, b string) bool {
	return a != "" && b != "" && hmac.Equal([]byte(a), []byte(b))
}

func nonce() string {
	b := make([]byte, 24)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func keysOf(keys ...string) []string {
	var out []string
	seen := map[string]bool{}
	for _, k := range keys {
		if k != "" && !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	return out
}

func SealWatch(aead cipher.AEAD, id, key, secret string) []byte {
	return seal(aead, []byte(secret), "watch "+id+" "+key)
}

func OpenWatch(aead cipher.AEAD, id, key string, sealed []byte) (string, error) {
	b, err := open(aead, sealed, "watch "+id+" "+key)
	return string(b), err
}
