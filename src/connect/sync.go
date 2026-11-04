package connect

import (
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
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// A connector's Streams are what Archive keeps a copy of for each account connected to it, in its tables: the
// kinds of record it pulls, like Gmail's messages. A stream pulls a page at a time from a cursor, which the
// caller keeps with the connection between pages: "" the first time, when it lists everything there is, and after that the
// Cursor of the page before, which pages on or, once caught up, starts the next delta, the records changed
// since. Each pull is one page of a provider's quota, so the caller runs them as tasks of their own, Gap
// apart, while one has more to come, and stops once it has caught up: what starts the next delta is the
// account changing, as its connector's Push hears (below), or someone asking.
//
// A provider with no change feed is listed whole, and two things keep that from costing a listing of everything on
// every sync. A record the stream's Memo knows is as it was is Same, so it is neither read again nor deleted. And a
// stream that changes seldom, like a company's departments, says Every, and is pulled no more often than that.

// Stream is a kind of record a connector syncs.
type Stream struct {
	// Name is the stream, like messages, which its rows keep in their stream column.
	Name string
	// Doc says what it pulls and how, and what it cannot see, for the docs.
	Doc string
	// Gap is how long after a page with more to come the next one runs.
	Gap time.Duration
	// Every is how long after it last caught up the stream is pulled again; 0 is on every sync. Until then an
	// ask leaves it as it is.
	Every time.Duration
	// Pull is the page after cursor. An *Unsupported error skips the stream, a day at a time.
	Pull func(c *Client, cursor string) (*Page, error)
	// Table is the table of archive.sql its records land in, SQL the query of the rows of a page's records, as Map has it
	// (def.go), and Version the hash of the definition its rows come from.
	Table, SQL, Version string
}

// Page is what one pull found.
type Page struct {
	Records []Record
	// Cursor is where the next pull starts: the next page while More, and once caught up the next delta.
	Cursor string
	More   bool
	// Full says the page is part of a listing of everything there is: once it ends, the records it did not
	// list are deleted.
	Full bool
}

// Record is one thing a stream found, by its ID at the provider.
type Record struct {
	ID   string
	Data any
	// Updated is when it last changed there, zero when the provider does not say.
	Updated time.Time
	// Deleted says it is gone there; its Data is not read.
	Deleted bool
	// Same says it is there as the stream last saved it, as the stream's memory knows: it is only seen, so a
	// full listing keeps it, and not written again; its Data is not read.
	Same bool
}

// Unsupported is a stream the account's provider has no way to pull, like an MCP server without its tool; it
// is skipped until the next sync.
type Unsupported struct{ Reason string }

func (u *Unsupported) Error() string { return u.Reason }

func unsupported(format string, a ...any) error {
	return &Unsupported{Reason: fmt.Sprintf(format, a...)}
}

// cursorOf reads cursor, the JSON a stream wrote with cursorFor, into v; "" leaves v as it is.
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

// each calls f for 0 to n-1, at most par at once, and is the first error.
func each(n, par int, f func(i int) error) error {
	errs := make([]error, n)
	sem := make(chan struct{}, par)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer func() { <-sem; wg.Done() }()
			errs[i] = f(i)
		}()
	}
	wg.Wait()
	return errors.Join(errs...)
}

// stamp is the time s, RFC 3339 or a Unix time in milliseconds as a string; zero when it is neither.
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

// clip is s cut to at most n bytes, on a rune boundary.
func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && n < len(s) && s[n]&0xC0 == 0x80 {
		n--
	}
	return s[:n]
}

// clamp is n within 1 and most, or def when it is not given.
func clamp(n, def, most int) int {
	if n <= 0 {
		return def
	}
	return min(n, most)
}

// maxText is the most of a record's text a stream keeps, like a message's body.
const maxText = 64 << 10

var tagRe = regexp.MustCompile(`(?s)<(script|style)\b.*?</(script|style)>|<[^>]*>`)

// htmlText is the words of an HTML document, roughly, for a message that has no text of its own.
func htmlText(s string) string {
	s = tagRe.ReplaceAllString(s, " ")
	s = strings.NewReplacer("&nbsp;", " ", "&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`, "&#39;", "'").Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

// Extract is the words of a file's content b, named name, of media type mime, as Archive reads an upload's: a
// PDF's, a Word, Excel or PowerPoint file's, HTML's or plain text's; "" for a kind it has none of. The server
// sets it, as this package cannot import it.
var Extract func(name, mime string, b []byte) string

// maxFile is the largest file a stream downloads to read its words; a bigger one keeps only its name.
const maxFile = 25 << 20

// textual says whether a file named name of media type mime is a kind Extract reads.
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

// bytesText is the words of b, a file named name of media type mime, as fileText reads them.
func bytesText(name, mime string, b []byte) string {
	if Extract == nil || !textual(name, mime) {
		return ""
	}
	return clip(strings.ToValidUTF8(Extract(name, mime, b), ""), maxText)
}

// fileText is the words of the file at the provider's URL u, named name, of media type mime and size bytes,
// downloaded whole and read with Extract, cut at maxText: "" for a kind Extract does not read, one over maxFile,
// or one the provider will not hand over or that does not come in time. Only a provider turning Archive away
// for now is an error, which the sync tries again later.
func (c *Client) fileText(u, name, mime string, size int64) (string, error) {
	if Extract == nil || !textual(name, mime) || size > maxFile {
		return "", nil
	}
	req, err := request(c.ctx, "GET", c.p.at(u), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "*/*")
	hc := *c.http
	hc.Timeout = 2 * time.Minute
	resp, err := hc.Do(req)
	if err != nil {
		if c.ctx.Err() != nil {
			return "", c.ctx.Err()
		}
		return "", nil
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		if r := refusal(c.p, resp); Busy(r) {
			return "", r
		}
		return "", nil
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxFile+1))
	if err != nil || len(b) > maxFile {
		return "", nil
	}
	return bytesText(name, mime, b), nil
}

// tool is the MCP server's tool name, from its list, which the Client keeps; a server without it makes the
// stream Unsupported.
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

// takes is what tool's input schema says it takes, or nil when it says nothing: then it takes anything; and
// what it says it needs.
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

// What a stream syncs lands in Archive's own tables (archive.sql), one row a record: a stream names its Table, and its
// Map says what columns a record fills there, as rows of the domain it belongs to, like a Gmail message as a message of
// mail. The tables are the same for every connector of a domain, so Gmail and Outlook share mail, and Gusto and any
// other payroll share payroll.
//
// People and organizations are shared by every domain, by a key that is the same wherever they are seen: a
// person is an email address, an organization an EIN. A Gmail sender and a Gusto employee with one address are
// one person. The rest of a person, like their name, stays on the rows that said it, since another member may
// not see those.

// Row is what a record is in its stream's table.
type Row struct {
	// Cols are its columns by name, as JSON has them. One left out is null.
	Cols map[string]any
	// People are its columns that are a known_person, by their email address; Orgs those that are a known_org,
	// by EIN or web domain. Refs are its columns that are another row of the connection, by that row's id there:
	// the column's table says which table, and <column>_source keeps the id, so it links once that row is there
	// too.
	People, Orgs, Refs map[string]string
	// Children are the rows of other tables that belong to it, like a message's parties: written again with it,
	// all of them, each time it is.
	Children []Child
}

// Child is a row of Table that belongs to a row of a stream's table.
type Child struct {
	Table string
	Row
}

var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+$`)

// Email is s as archive.email has an address, lowercase, or "" when it is none.
func Email(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if !emailRe.MatchString(s) {
		return ""
	}
	return s
}

// addresses are the addresses of a header like To, as a mail reader shows them; one that does not parse is
// left out.
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

// A connector's Push is how its provider tells Archive that an account changed, so that its sync runs then and
// only then: nothing in Archive asks a provider on a timer. A push only says which account; what changed is
// the streams' delta from their cursors. So a push that is lost, or one missed while a watch had lapsed, loses
// nothing: the next sync, whatever starts it, reads everything since the last.
//
// A provider pushes one of two ways. Some push for every account of the app, as its dashboard is set up once
// (GitHub, HubSpot, QuickBooks): their Watch only names the keys their pushes carry for an account, like its
// QuickBooks realmId, and Hear checks a push with the app's secret. Others push for an account only once
// Archive asks them to, to Archive's URL (Gmail, Drive, Outlook, Asana), until the watch Expires: the first
// sync of the account asks, and a sync that runs once a watch is within Renew of expiring renews it. Nothing
// renews one on a timer; an account nobody touches lets its watch lapse, and the next sync, on a read or an
// open, asks again.

// Push is how a connector's provider tells Archive an account changed.
type Push struct {
	// Watch asks the provider to push the account's changes, renewing or replacing old, the watches it has, and
	// is the watches there are now; an app's pushes need only their keys. Watch with h.URL empty is not asked.
	Watch func(c *Client, h Hook, old []Watch) ([]Watch, error)
	// Unwatch ends w, when the account disconnects; nil when there is nothing to end.
	Unwatch func(c *Client, w Watch) error
	// Renew is how long before a watch expires the sync after renews it.
	Renew time.Duration
	// Env is the variable that names where the provider pushes, like Gmail's Pub/Sub topic; Watch reads it as
	// Hook.Target, and is not asked without it.
	Env string
	// OIDC says the provider pushes through Google Pub/Sub, whose push subscription signs each with the OIDC
	// token of Archive's tasks account, which Archive checks before Hear.
	OIDC bool
	// Hear reads a push the provider sent to the URL at, checking it is the provider's, as the keys of the
	// watches it is for: secret is the secret Archive kept with the watch of key, "" when there is none.
	Hear func(p *Provider, r *http.Request, at string, body []byte, secret func(key string) string) (*Heard, error)
}

// Hook is where a Watch asks for pushes.
type Hook struct {
	// ID is the connection's, which starts the keys that must be its own; Account its account.
	ID, Account string
	// URL is where the provider pushes for the watch of key: Archive's, with the key in its query.
	URL func(key string) string
	// Target is the value of the Push's Env.
	Target string
}

// Watch is one thing a provider pushes for an account: Key is what its pushes name it by, Resource what the
// provider calls it, to end it, Secret what its pushes prove themselves with, and Expires when it ends on its
// own, zero for never.
type Watch struct {
	Key, Resource, Secret string
	Expires               time.Time
}

// Heard is what a push said: the keys of the watches it is for, and what to answer, 202 Accepted when Status
// is 0. Keep are secrets a handshake handed over, by the key in the push's URL, to check its pushes with.
type Heard struct {
	Keys   []string
	Status int
	Header http.Header
	Body   string
	Keep   map[string]string
	// All and Forget are what it asks Archive to forget of the accounts of Keys (Listen.Forget): All every one,
	// disconnected; Forget the records of each stream, by id.
	All    bool
	Forget map[string][]string
}

// ErrForged is a push that is not the provider's: unsigned, or signed with another secret.
var ErrForged = errors.New("not the provider's")

func forged(format string, a ...any) error {
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, a...), ErrForged)
}

// PushOf is the push of the connector this server has a client for, and where it pushes; nil when it has none.
func (p *Provider) PushOf() (*Push, string) { return p.Push, p.pushTo }

// HookSecret is the app's webhook secret, the hook option of the connector's variable, like ?hook=<secret>.
func (p *Provider) HookSecret() string { return p.opts.Get("hook") }

// same says whether a and b are the same, in constant time; never when either is empty.
func same(a, b string) bool {
	return a != "" && b != "" && hmac.Equal([]byte(a), []byte(b))
}

// nonce is a random secret, URL-safe.
func nonce() string {
	b := make([]byte, 24)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// keysOf are the distinct non-empty keys.
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

// SealWatch is the secret of watch key of connector id, encrypted and bound to it, to keep.
func SealWatch(aead cipher.AEAD, id, key, secret string) []byte {
	return seal(aead, []byte(secret), "watch "+id+" "+key)
}

// OpenWatch is the secret SealWatch sealed for watch key of connector id.
func OpenWatch(aead cipher.AEAD, id, key string, sealed []byte) (string, error) {
	b, err := open(aead, sealed, "watch "+id+" "+key)
	return string(b), err
}
