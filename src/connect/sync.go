package connect

import (
	"cmp"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math/big"
	"net/http"
	"net/mail"
	"regexp"
	"slices"
	"strconv"
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
// A provider with no change feed is listed whole, and three things keep that from costing a listing of
// everything on every sync. The Cursor a stream ends a listing with is its memory: the next sync starts from
// it, so it can say which ids it has read in full, the versions it saw, or that it has read what never
// changes. A record it knows is as it was is Same, so it is neither read again nor deleted. And a stream that
// changes seldom, like a company's departments, says Every, and is pulled no more often than that.

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
	// Table is the table of archive.sql its records land in, and Rows the row a record's data is there
	// (below); nil leaves the record out.
	Table string
	Rows  func(data map[string]any) *Row
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

// What a stream syncs lands in Archive's own tables (archive.sql), one row a record: a stream names its Table,
// and its Rows says what columns a record fills there, as rows of the domain it belongs to, like a Gmail
// message as a message of mail. The tables are the same for every connector of a domain, so Gmail and Outlook
// share mail, and Gusto and any other payroll share payroll: a connector fills in Rows from its records, and
// knows nothing of SQL.
//
// People and organizations are shared by every domain, by a key that is the same wherever they are seen: a
// person is an email address, an organization an EIN. A Gmail sender and a Gusto employee with one address are
// one person. The rest of a person, like their name, stays on the rows that said it, since another member may
// not see those.

// Row is what a record is in its stream's table.
type Row struct {
	// Cols are its columns by name, as JSON has them: text, numbers, booleans, dates as YYYY-MM-DD, times as RFC
	// 3339, lists, and Money. One left out is null.
	Cols map[string]any
	// People are its columns that are a known_person, by their email address; Orgs those that are a known_org,
	// by EIN. Refs are its columns that are another row of the connection, by that row's id there: the column's
	// table says which table, and <column>_source keeps the id, so it links once that row is there too.
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

// rowOf is a row of cols with its people, orgs and refs, as pairs of column and key, left out when the key is
// empty.
func rowOf(cols map[string]any) *Row {
	return &Row{Cols: cols, People: map[string]string{}, Orgs: map[string]string{}, Refs: map[string]string{}}
}

// person makes column col the person at the address email, when it is one.
func (r *Row) person(col, email string) *Row {
	if e := Email(email); e != "" {
		r.People[col] = e
	}
	return r
}

// ref makes column col the row of the connection whose id there is id.
func (r *Row) ref(col, id string) *Row {
	if id != "" {
		r.Refs[col] = id
	}
	return r
}

// child adds a row of table.
func (r *Row) child(table string, c *Row) *Row {
	r.Children = append(r.Children, Child{Table: table, Row: *c})
	return r
}

// RowOf is the row of the record data, as JSON, in s's table; nil when it is none.
func (s *Stream) RowOf(data []byte) *Row {
	var m map[string]any
	if json.Unmarshal(data, &m) != nil || m == nil {
		return nil
	}
	return s.Rows(m)
}

// Money is an amount in a currency's minor units; its JSON is archive.money's, null when there is none.
type Money struct {
	Minor    *int64
	Currency string
}

func (m Money) MarshalJSON() ([]byte, error) {
	if m.Minor == nil {
		return []byte("null"), nil
	}
	return json.Marshal(map[string]any{"amount": *m.Minor, "currency": m.Currency})
}

// money is the amount v, a JSON number or a string in major units like "1234.56", in currency, which has
// two decimals; none when v is neither.
func money(v any, currency string) Money {
	s := num(v)
	r, ok := new(big.Rat).SetString(s)
	if s == "" || !ok {
		return Money{}
	}
	n, err := strconv.ParseInt(r.Mul(r, big.NewRat(100, 1)).FloatString(0), 10, 64)
	if err != nil {
		return Money{}
	}
	return Money{Minor: &n, Currency: currency}
}

// num is v, a JSON number or a numeric string, as a decimal; "" when it is neither.
func num(v any) string {
	switch x := v.(type) {
	case string:
		x = strings.TrimSpace(x)
		if _, ok := new(big.Rat).SetString(x); ok {
			return x
		}
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	}
	return ""
}

// decimal is v as num has it, or nil, for a numeric column.
func decimal(v any) any {
	if s := num(v); s != "" {
		return s
	}
	return nil
}

// sum is the amounts added up, none when none is there.
func sum(ms ...Money) Money {
	var out Money
	for _, m := range ms {
		if m.Minor == nil {
			continue
		}
		if out.Minor == nil {
			out = Money{Minor: new(int64), Currency: m.Currency}
		}
		*out.Minor += *m.Minor
	}
	return out
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

var einRe = regexp.MustCompile(`^(0[1-6]|1[0-6]|2[0-7]|3[0-9]|4[0-8]|5[0-9]|6[0-8]|7[1-7]|8[0-8]|9[0-58-9])-[0-9]{7}$`)

// EIN is s as archive.us_ein has one, NN-NNNNNNN, or "" when it is none.
func EIN(s string) string {
	d := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
	if len(d) != 9 || !einRe.MatchString(d[:2]+"-"+d[2:]) {
		return ""
	}
	return d[:2] + "-" + d[2:]
}

// day is s cut to its date, YYYY-MM-DD, or nil when it does not start with one, for a date column.
func day(s string) any {
	if t, err := time.Parse(time.DateOnly, clip(s, 10)); err == nil {
		return t.Format(time.DateOnly)
	}
	return nil
}

// instant is s, RFC 3339 or a Unix time in milliseconds, or nil, for a timestamptz column.
func instant(s string) any {
	if t := stamp(s); !t.IsZero() {
		return t.UTC().Format(time.RFC3339Nano)
	}
	return nil
}

// strs are the strings of v, a JSON array.
func strs(v any) []string {
	a, _ := v.([]any)
	out := []string{}
	for _, e := range a {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// flag is v[k] when it is a boolean, or nil.
func flag(v map[string]any, k string) any {
	if b, ok := v[k].(bool); ok {
		return b
	}
	return nil
}

// obj is v[k] as an object, empty when it is none.
func obj(v map[string]any, k string) map[string]any {
	m, _ := v[k].(map[string]any)
	if m == nil {
		m = map[string]any{}
	}
	return m
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

// mac is the HMAC-SHA256 of parts with key.
func mac(key string, parts ...string) []byte {
	m := hmac.New(sha256.New, []byte(key))
	for _, p := range parts {
		m.Write([]byte(p))
	}
	return m.Sum(nil)
}

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

func hexMAC(key string, parts ...string) string { return hex.EncodeToString(mac(key, parts...)) }
func b64MAC(key string, parts ...string) string {
	return base64.StdEncoding.EncodeToString(mac(key, parts...))
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

func readJSON(body []byte, v any) error {
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("a push that is not JSON: %v", err)
	}
	return nil
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
