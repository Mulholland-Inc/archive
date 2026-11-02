package connect

// A connector's Streams are what Archive keeps a copy of for each account connected to it: the kinds of record
// it pulls, like Gmail's messages. A stream pulls a page at a time from a cursor, which the caller keeps with
// the connection between pages: "" the first time, when it lists everything there is, and after that the
// Cursor of the page before, which pages on or, once caught up, starts the next delta, the records changed
// since. Each pull is one page of a provider's quota, so the caller runs them as tasks of their own, Gap
// apart, while one has more to come, and stops once it has caught up: what starts the next delta is the
// account changing, as its connector's Push hears (push.go), or someone asking.

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Stream is a kind of record a connector syncs.
type Stream struct {
	// Name is the record's stream in archive.record, like messages.
	Name string
	// Doc says what it pulls and how, and what it cannot see, for the docs.
	Doc string
	// Gap is how long after a page with more to come the next one runs.
	Gap time.Duration
	// Pull is the page after cursor. An *Unsupported error skips the stream, a day at a time.
	Pull func(c *Client, cursor string) (*Page, error)
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
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t
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

// maxText is the most of a record's text a stream keeps, like a message's body.
const maxText = 64 << 10

var tagRe = regexp.MustCompile(`(?s)<(script|style)\b.*?</(script|style)>|<[^>]*>`)

// htmlText is the words of an HTML document, roughly, for a message that has no text of its own.
func htmlText(s string) string {
	s = tagRe.ReplaceAllString(s, " ")
	s = strings.NewReplacer("&nbsp;", " ", "&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`, "&#39;", "'").Replace(s)
	return strings.Join(strings.Fields(s), " ")
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

// answer is a tool's result as JSON: its structured content, or its text read as JSON. A tool that answers in
// prose is one Archive cannot sync from.
func answer(c *Client, tool string, res *mcp.CallToolResult) (any, error) {
	if res.StructuredContent != nil {
		b, _ := json.Marshal(res.StructuredContent)
		var v any
		return v, json.Unmarshal(b, &v)
	}
	var v any
	if err := json.Unmarshal([]byte(Text(res)), &v); err != nil {
		return nil, unsupported("%s's tool %s answers in prose, not JSON, which Archive cannot sync from", c.p.Name, tool)
	}
	return v, nil
}

// objects are the records in v: the array under key, at any depth, or with no key the first array of objects
// there is, v itself when it is one.
func objects(v any, key string) []map[string]any {
	var arr []any
	var walk func(v any) bool
	walk = func(v any) bool {
		switch v := v.(type) {
		case []any:
			if key == "" && len(v) > 0 {
				if _, ok := v[0].(map[string]any); ok {
					arr = v
					return true
				}
			}
			for _, e := range v {
				if walk(e) {
					return true
				}
			}
		case map[string]any:
			if a, ok := v[key].([]any); ok && key != "" {
				arr = a
				return true
			}
			keys := make([]string, 0, len(v))
			for k := range v {
				keys = append(keys, k)
			}
			slices.Sort(keys)
			for _, k := range keys {
				if walk(v[k]) {
					return true
				}
			}
		}
		return false
	}
	walk(v)
	out := make([]map[string]any, 0, len(arr))
	for _, e := range arr {
		if m, ok := e.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// str is v[k] as a string, numbers too.
func str(v map[string]any, k string) string {
	switch x := v[k].(type) {
	case string:
		return x
	case float64:
		return fmt.Sprint(int64(x))
	case json.Number:
		return x.String()
	}
	return ""
}

// mcpList is a stream an MCP connector pulls with one of its server's list tools, with as much of it as the
// tool's input schema, read from the server on every pull, says it takes: a page size, a cursor to the next
// page, and a time to list only what changed since. A tool that takes a time is pulled for what changed since
// the pull before began, and pages until it has it all; one that does not is listed whole each time, and what
// it no longer lists is deleted. A server without the tool, or whose tool does not take args, skips it.
type mcpList struct {
	tool string
	// args go with every call; opts too, those the schema does not name left out. in, when set, is the
	// argument the page size, cursor and time go in, like Stripe's parameters, which the schema cannot say.
	args, opts map[string]any
	in         string
	// limit is the argument of the page size, max; after the next page's cursor; page the next page's number,
	// from 1, while a page comes back full; since the time, ISO 8601. Each is the first of its names, comma
	// separated, that the schema has.
	limit, after, page, since string
	max                       int
	// items is the key of the answer's array of records, "" for the first there is; id and updated the keys of
	// a record's id and last change, id and updatedAt when empty, or key the record's id.
	items, id, updated string
	key                func(it map[string]any) string
	// lastID pages with the last record's id as the cursor when the answer has none and the page is full.
	lastID bool
}

// mcpCursor is where an mcpList is: since the time of its delta, after the next page's cursor or number,
// start when this listing began, and first the first record of the page before, which a server that does not
// page sends again.
type mcpCursor struct {
	Since string `json:"since,omitempty"`
	After string `json:"after,omitempty"`
	Start string `json:"start,omitempty"`
	First string `json:"first,omitempty"`
}

func (l *mcpList) stream(name, doc string) *Stream {
	return &Stream{Name: name, Doc: doc, Pull: l.pull}
}

func (l *mcpList) pull(c *Client, cursor string) (*Page, error) {
	var st mcpCursor
	if err := cursorOf(cursor, &st); err != nil {
		return nil, err
	}
	t, err := c.tool(l.tool)
	if err != nil {
		return nil, err
	}
	props, need := takes(t)
	has := func(k string) bool { return k != "" && (props == nil || props[k]) }
	args, put := map[string]any{}, map[string]any{}
	for k, v := range l.args {
		if !has(k) {
			return nil, unsupported("%s's tool %s takes no %s", c.p.Name, l.tool, k)
		}
		args[k] = v
	}
	for k, v := range l.opts {
		if has(k) {
			args[k] = v
		}
	}
	can, after := has, ""
	if l.in != "" {
		if m, ok := args[l.in].(map[string]any); ok {
			maps.Copy(put, m)
		}
		args[l.in] = put
		can = func(k string) bool { return k != "" }
	} else {
		put = args
	}
	first := func(names string) string {
		for _, n := range strings.Split(names, ",") {
			if can(n) {
				return n
			}
		}
		return ""
	}
	after, numbered := first(l.after), false
	if after == "" {
		after = first(l.page)
		numbered = after != ""
	}
	if lim := first(l.limit); lim != "" && l.max > 0 {
		put[lim] = l.max
	}
	if st.Start == "" {
		st.Start = time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)
	}
	since := first(l.since)
	delta := since != ""
	if delta && st.Since != "" {
		put[since] = st.Since
	}
	if st.After != "" && after != "" {
		put[after] = st.After
		if numbered {
			put[after], _ = strconv.Atoi(st.After)
		}
	}
	for _, k := range need {
		if _, ok := args[k]; !ok && l.in == "" {
			return nil, unsupported("%s's tool %s needs %s, which Archive has nothing to give", c.p.Name, l.tool, k)
		}
	}
	res, err := c.Call(t.Name, args)
	if err != nil {
		return nil, err
	}
	v, err := answer(c, t.Name, res)
	if err != nil {
		return nil, err
	}
	id, updated := cmp.Or(l.id, "id"), cmp.Or(l.updated, "updatedAt")
	key := l.key
	if key == nil {
		key = func(it map[string]any) string { return str(it, id) }
	}
	items := objects(v, l.items)
	page := &Page{Full: !delta || st.Since == ""}
	for _, it := range items {
		if rid := key(it); rid != "" {
			page.Records = append(page.Records, Record{ID: rid, Data: it, Updated: stamp(str(it, updated))})
		}
	}
	next, firstID := "", ""
	if len(items) > 0 {
		firstID = key(items[0])
	}
	full := len(items) > 0 && len(items) >= l.max
	switch {
	case numbered && full:
		n, _ := strconv.Atoi(st.After)
		next = strconv.Itoa(max(n, 1) + 1)
	case after != "":
		next = nextCursor(v, items, id)
		if next == "" && l.lastID && full {
			next = str(items[len(items)-1], id)
		}
	}
	// A next page that is the one before, or starts as it did, is a server that does not page: the listing
	// ends there rather than read it forever.
	if next != "" && next != st.After && (firstID == "" || firstID != st.First) {
		st.After, st.First, page.More = next, firstID, true
	} else {
		st = mcpCursor{Since: st.Start}
	}
	page.Cursor = cursorFor(st)
	return page, nil
}

// nextCursor is the cursor of the page after items in an answer v: one it names at its top or a level down,
// like nextCursor or pageInfo.endCursor while hasNextPage, or with has_more the last record's id, as Stripe
// pages; "" when it says there is no more.
func nextCursor(v any, items []map[string]any, id string) string {
	m, _ := v.(map[string]any)
	levels := []map[string]any{m}
	for _, e := range m {
		if n, ok := e.(map[string]any); ok {
			levels = append(levels, n)
		}
	}
	for _, l := range levels {
		for _, k := range []string{"hasMore", "has_more", "hasNextPage"} {
			if more, ok := l[k].(bool); ok && !more {
				return ""
			}
		}
	}
	for _, l := range levels {
		for _, k := range []string{"nextCursor", "next_cursor", "endCursor", "cursor"} {
			if s, ok := l[k].(string); ok && s != "" {
				return s
			}
		}
	}
	for _, l := range levels {
		for _, k := range []string{"hasMore", "has_more"} {
			if more, _ := l[k].(bool); more && len(items) > 0 {
				return str(items[len(items)-1], id)
			}
		}
	}
	return ""
}
