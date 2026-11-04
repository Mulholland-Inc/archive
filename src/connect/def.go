package connect

import (
	"bytes"
	"cmp"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/http"
	"net/mail"
	"net/textproto"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
)

// A connector is data: a sync and some actions over its provider's API. Its file holds a Connector literal, with
// no funcs, which Register compiles into what the rest of the server runs. The engine here owns everything the
// file does not say: where a sync is, when it lists whole, and what a page of it deletes.
//
// Values in a definition are read at run time, so the compiler cannot check them: a Path is a /-separated walk of
// an answer, where * is every element of a list; a template is a string whose {path} is read from what the request
// knows. {path} is escaped in a URL, {+path} is not, {path,} joins a list with commas, and {path|x} is x when the
// path has nothing. A value that is only one {path} keeps its type, and with nothing there its key is left out.

type (
	// Path is a walk of an answer, like paging/next/after or */portalId.
	Path string
	// SQL is an expression Postgres evaluates over a record, the jsonb column data.
	SQL string
	// Q is a query or form, by template.
	Q map[string]string
	// Obj is a JSON body or output, whose strings are templates.
	Obj = map[string]any
	// Method is how a request is sent; GET unless it says.
	Method string
)

const (
	GET    Method = "GET"
	POST   Method = "POST"
	PUT    Method = "PUT"
	PATCH  Method = "PATCH"
	DELETE Method = "DELETE"
)

// Req is a request. A relative URL is on the connector's API. A Method other than GET writes unless it is Read.
type Req struct {
	Method Method
	URL    string
	Query  Q
	Form   Q
	Body   any
	Read   bool
	// exact is a URL a provider named, sent as it is.
	exact bool
}

func (r Req) writes() bool { return r.URL != "" && r.Method != "" && r.Method != GET && !r.Read }

// Info is how a connector shows.
type Info struct {
	By          string   `json:"by,omitempty"`
	Description string   `json:"description,omitempty"`
	Category    Category `json:"category,omitempty"`
	Icon        Icon     `json:"icon,omitempty"`
	// Scopes say in words what connecting allows.
	Scopes []string `json:"scopes,omitempty"`
}

type (
	Category string
	Icon     string
)

const (
	CatEmail    Category = "Email"
	CatCRM      Category = "CRM"
	CatPractice Category = "Practice"

	IconMail  Icon = "mail"
	IconUsers Icon = "users"
	IconList  Icon = "list"
)

// Who is how to ask whose account a token is for: the first of Names in the answer, lowercase when Lower.
type Who struct {
	Req
	Names []Path
	Lower bool
}

// Paging is how an answer names the next page, which the next request reads as {page}.
type Paging interface{ paging() }

type (
	// NextURL is an answer naming the next page's URL.
	NextURL struct{ At Path }
	// Cursor is an answer naming the next page's token, while More says so when there is a More. GraphQL pages
	// so, with {page} in its variables.
	Cursor struct{ At, More Path }
	// Offset is a count from From that goes up by Step while a page comes back with Size records.
	Offset struct{ From, Size, Step int }
)

func (NextURL) paging() {}
func (Cursor) paging()  {}
func (Offset) paging()  {}

// Change is how a stream reads what changed since its last sync; nil lists it whole every time. A position the
// provider no longer takes, one of a Change's Expired statuses, starts a full listing. A provider whose Change
// cannot say what was deleted is listed whole relistAfter after it last was.
type Change interface{ change() }

type (
	// SinceTime asks with {since}, in Unix milliseconds when Ms. Mark, when given, is the time on a record: a
	// sync pages on from the last it read rather than by page, which Most limits, past which it lists whole.
	SinceTime struct {
		Req     Req
		Ms      bool
		Mark    Path
		Most    int
		Expired []int
	}
	// NewestFirst pages the listing with Query too, newest change first, down to the last sync, by Updated. A
	// record without one lists whole.
	NewestFirst struct {
		Query   Q
		Expired []int
	}
	// Token asks with the {token} Start's answer named at At, or the last answer did. Each is the answer's
	// list of changes, whose Live and Gone are the ids of records to read and of those gone.
	Token struct {
		Start      Req
		At         Path
		Req        Req
		Each       Path
		Live, Gone []Path
		Expired    []int
	}
)

func (SinceTime) change()   {}
func (NewestFirst) change() {}
func (Token) change()       {}

// Parts fans one stream out over many of a thing, like calendars, each read as {part}. The parts a sync finds are
// listed whole when they differ from the last.
type Parts struct {
	Req       Req
	Items, ID Path
}

// Hydrate adds to the records of a page, after it is read.
type Hydrate interface{ hydrate() }

type (
	// Fetch is a request per record, Par at once, whose answer is the record, or goes at As. With Gone, a 404 is
	// a record deleted there.
	Fetch struct {
		Req  Req
		As   string
		Par  int
		Gone bool
	}
	// Batch is one request for a page, which reads {inputs}, the records' ids as [{id}], whose Results go on the
	// record whose id is at their Key, at As. A listing skips it with DeltaOnly.
	Batch struct {
		Req          Req
		As           string
		Results, Key Path
		DeltaOnly    bool
	}
	// Lookup is one request for a page, whose whole answer goes on each record at As. With Soft, only a provider
	// that is busy fails the page.
	Lookup struct {
		Req  Req
		As   string
		Soft bool
	}
	// Decode is a codec over each record.
	Decode struct{ Codec *Codec }
	// Files reads the words of the files of a record, the list at Each, Par at once, at As. Its request answers
	// with the content, base64url, at Data. A file without Has, over Most bytes at Size, or of a kind Archive does
	// not read is not asked for, and only a provider that is busy fails the page.
	Files struct {
		Each            Path
		Req             Req
		Has, Name, Mime Path
		Size, Data      Path
		As              string
		Most, Par       int
	}
)

func (Fetch) hydrate()  {}
func (Batch) hydrate()  {}
func (Lookup) hydrate() {}
func (Decode) hydrate() {}
func (Files) hydrate()  {}

// Map is what a record is as a row, each column a SQL expression over data. People are its known_person columns
// by address and Orgs its known_org columns by web domain, Refs its columns that are a row of the connection by
// that row's id there. Children are rows of other tables that belong to it.
type Map struct {
	Table              string
	Cols               map[string]SQL
	People, Orgs, Refs map[string]SQL
	Children           []Kids
}

// Kids are the child rows of a record: one for each row of From, a SQL table expression over data.
type Kids struct {
	Table              string
	From               SQL
	Cols               map[string]SQL
	People, Orgs, Refs map[string]SQL
}

// Sync is a kind of record a connector syncs: its listing, how it reads what changed, what else a page reads, and
// the row a record is. Vars are templates' own values, so that streams share theirs. Items is the answer's list of
// records, ID each one's id, id when empty, and Updated when it last changed there.
type Sync struct {
	Name, Doc string
	Gap       time.Duration
	Vars      Obj
	Req       Req
	Items     Path
	ID        Path
	Updated   Path
	Page      Paging
	Change    Change
	Parts     *Parts
	Hydrate   []Hydrate
	To        Map
}

// relistAfter is how long after a full listing a provider that cannot say what was deleted lists whole again, and
// overlap how far before a sync's start the next reads from, against the provider's clock.
const (
	relistAfter = 7 * 24 * time.Hour
	overlap     = time.Minute
)

// Plan is an action: its Steps in order, each reading the input and the steps' As before it, and Out, or else the
// answer of the last. Clamp keeps a number within what a provider takes: { def, most }. Enum names the only
// values an input may be.
type Plan struct {
	Doc   string
	Clamp map[string][2]int
	Enum  map[string][]string
	Steps []Step
	Out   any
}

// Step is a request, a codec, or the one over the other. Each runs it for every element of a list, which reads the
// element's fields too. It runs only with If, and only without Unless. A codec reads the request's answer, or In.
type Step struct {
	Req
	As         string
	If, Unless Path
	Each       Path
	Codec      *Codec
	In         any
}

// Call is the action of a Plan, whose In and Out are its input's and output's types, for the docs and the check of
// a script's input. It writes when any step does.
func Call[In, Out any](p Plan) *Action {
	in, err := jsonschema.For[In](nil)
	if err != nil {
		panic(err)
	}
	out, err := jsonschema.For[Out](nil)
	if err != nil {
		panic(err)
	}
	for k, vals := range p.Enum {
		if in.Properties[k] == nil {
			panic(fmt.Sprintf("connect: an enum of %s, which the input does not have", k))
		}
		in.Properties[k].Enum = make([]any, len(vals))
		for i, v := range vals {
			in.Properties[k].Enum[i] = v
		}
	}
	a := &Action{Doc: p.Doc, Input: in, Output: out, plan: &p}
	a.Writes = slices.ContainsFunc(p.Steps, func(s Step) bool { return s.Req.writes() })
	a.Do = func(c *Client, raw json.RawMessage) (any, error) { return p.run(c, raw) }
	return a
}

func (p *Plan) run(c *Client, raw json.RawMessage) (any, error) {
	v, err := readAll(raw, c.p.Name)
	if err != nil {
		return nil, err
	}
	sc, _ := v.(map[string]any)
	if sc == nil {
		sc = map[string]any{}
	}
	for k, lim := range p.Clamp {
		n, _ := strconv.Atoi(text(sc[k]))
		sc[k] = clamp(n, lim[0], lim[1])
	}
	var last any
	for _, s := range p.Steps {
		if s.If != "" && none(pick(sc, s.If)) || s.Unless != "" && !none(pick(sc, s.Unless)) {
			continue
		}
		var out any
		if s.Each == "" {
			if out, err = s.once(c, sc, nil); err != nil {
				return nil, err
			}
		} else {
			list := []any{}
			for _, e := range listOf(pick(sc, s.Each)) {
				x := maps.Clone(sc)
				if m, ok := e.(map[string]any); ok {
					maps.Copy(x, m)
				}
				r, err := s.once(c, x, e)
				if err != nil {
					return nil, err
				}
				list = append(list, r)
			}
			out = list
		}
		if s.As != "" {
			sc[s.As] = out
		}
		last = out
	}
	if p.Out != nil {
		out, _ := fill(p.Out, sc)
		return out, nil
	}
	return last, nil
}

func (s Step) once(c *Client, sc map[string]any, e any) (v any, err error) {
	v = e
	if s.URL != "" {
		if v, err = c.req(s.Req, sc); err != nil {
			return nil, err
		}
	}
	if s.Codec != nil {
		if s.In != nil {
			v, _ = fill(s.In, sc)
		}
		return s.Codec.fn(v)
	}
	return v, nil
}

// Sign is a way a push proves it is the provider's, when it has the header Header and, if it says, When is Is.
// The signature is Digest of Of, a template of {method}, {url}, {body}, {secret} and {header/<name>}, with HMAC
// keyed by the app's secret. Plain are characters {url} has not percent-escaped, as the provider signs it, and
// Fresh a header of its time in Unix milliseconds, which must be under 5 minutes old.
type Sign struct {
	Header, When, Is string
	Digest           Digest
	Enc              Enc
	Of               string
	Plain, Fresh     string
}

type (
	Digest int
	Enc    int
)

const (
	HMACSHA256 Digest = iota
	SHA256
)

const (
	Base64 Enc = iota
	Hex
)

// Listen is how a provider tells Archive an account changed. Env names where it pushes, read as {target} and not
// asked for without it. OIDC is a Pub/Sub push, signed for Archive's tasks account; Signs the app's other schemes,
// the first whose header a push has deciding. Watch asks for pushes, with {account}, {client} and {token}; Key is
// the template of what they name it by, with its answer at {answer}, Expires when it ends, and Renew how long
// before that a sync renews it. A push's Keys are read from its body, or from what Codec makes of what is at Wrap.
type Listen struct {
	Env     string
	OIDC    bool
	Signs   []Sign
	Watch   Req
	Key     string
	Expires Path
	Unwatch *Req
	Renew   time.Duration
	Wrap    Path
	Codec   *Codec
	Keys    Path
}

// compile fills in what the rest of the server runs from a connector's data, and fails on data that cannot run.
func (c *Connector) compile() error {
	if c.API == "" {
		return nil
	}
	if len(c.Keys) == 0 && c.MCP == "" {
		if c.Env == "" {
			c.Env = "ARCHIVE_" + strings.ToUpper(c.ID)
		}
		if u, err := url.Parse(c.Endpoint.AuthURL); err == nil && c.Host == "" {
			c.Host = u.Scheme + "://" + u.Host
		}
	}
	if c.Sign != nil {
		sign := c.Sign
		c.Authorize = func(r *http.Request, cl *Client) {
			for h, t := range sign {
				v, _ := fillStr(t, cl.scope())
				r.Header.Set(h, text(v))
			}
		}
	}
	if w := c.Who; w != nil {
		if err := w.Req.check("who", "client", "token", "key"); err != nil {
			return err
		}
		c.Account = func(cl *Client) (string, error) {
			ans, err := cl.req(w.Req, cl.scope())
			if err != nil {
				return "", err
			}
			s := ""
			for _, n := range w.Names {
				if s = text(pick(ans, n)); s != "" {
					break
				}
			}
			if w.Lower {
				s = strings.ToLower(s)
			}
			if s == "" && len(c.Keys) > 0 {
				s = c.Name
			}
			return s, nil
		}
	}
	if r := c.RevokeBy; r != nil {
		c.RevokeWith = func(rv *Revocation) error {
			id, secret := rv.Client()
			sc := map[string]any{"client": map[string]any{"id": id, "secret": secret}, "refresh": rv.Refresh}
			return rv.Form(r.URL, nil, values(r.Form, sc))
		}
	}
	for name, a := range c.Actions {
		if a.plan != nil {
			if err := a.check(); err != nil {
				return fmt.Errorf("action %s: %w", name, err)
			}
		}
	}
	if l := c.Listen; l != nil {
		if err := l.compile(c); err != nil {
			return fmt.Errorf("listen: %w", err)
		}
	}
	for _, s := range c.Sync {
		if err := s.check(); err != nil {
			return fmt.Errorf("stream %s: %w", s.Name, err)
		}
		c.Streams = append(c.Streams, &Stream{Name: s.Name, Doc: s.Doc, Gap: s.Gap, Table: s.To.Table, Pull: s.pull, SQL: s.To.sql(), Version: s.version()})
	}
	return nil
}

func (r Req) check(where string, roots ...string) error {
	switch r.Method {
	case "", GET, POST, PUT, PATCH, DELETE:
	default:
		return fmt.Errorf("%s: no method %q", where, r.Method)
	}
	if r.URL == "" {
		return fmt.Errorf("%s: no URL", where)
	}
	var seen []string
	note := func(t string) {
		for _, m := range varRe.FindAllString(t, -1) {
			v := varOf(m)
			root, _, _ := strings.Cut(v.path, "/")
			seen = append(seen, root)
		}
	}
	note(r.URL)
	for _, t := range r.Query {
		note(t)
	}
	for _, t := range r.Form {
		note(t)
	}
	var walk func(v any)
	walk = func(v any) {
		switch v := v.(type) {
		case string:
			note(v)
		case map[string]any:
			for _, e := range v {
				walk(e)
			}
		case []map[string]any:
			for _, e := range v {
				walk(e)
			}
		case []any:
			for _, e := range v {
				walk(e)
			}
		}
	}
	walk(r.Body)
	for _, s := range seen {
		if !slices.Contains(roots, s) {
			return fmt.Errorf("%s: {%s} is nothing it knows; it knows %s", where, s, strings.Join(roots, ", "))
		}
	}
	return nil
}

func (a *Action) check() error {
	roots := slices.Collect(maps.Keys(a.Input.Properties))
	for i, s := range a.plan.Steps {
		if s.URL == "" && s.Codec == nil {
			return fmt.Errorf("step %d is neither a request nor a codec", i)
		}
		if s.URL != "" && s.Each == "" {
			if err := s.Req.check(fmt.Sprintf("step %d", i), roots...); err != nil {
				return err
			}
		}
		if s.As != "" {
			roots = append(roots, s.As)
		}
	}
	return nil
}

func (s *Sync) check() error {
	if s.Name == "" || s.Doc == "" || s.To.Table == "" {
		return errors.New("needs a Name, a Doc and a To.Table")
	}
	if len(s.To.Cols) == 0 || len(s.To.Cols) > 50 {
		return errors.New("To.Cols has between 1 and 50 columns")
	}
	roots := append(slices.Collect(maps.Keys(s.Vars)), "stream", "part", "page")
	if err := s.Req.check("listing", roots...); err != nil {
		return err
	}
	switch ch := s.Change.(type) {
	case SinceTime:
		if err := ch.Req.check("delta", append(roots, "since")...); err != nil {
			return err
		}
	case Token:
		if ch.At == "" || ch.Each == "" || len(ch.Live) == 0 {
			return errors.New("a Token needs At, Each and Live")
		}
		if err := ch.Start.check("token start", roots...); err != nil {
			return err
		}
		if err := ch.Req.check("delta", append(roots, "token")...); err != nil {
			return err
		}
	case NewestFirst:
		if s.Updated == "" {
			return errors.New("NewestFirst needs Updated")
		}
	}
	for _, h := range s.Hydrate {
		if d, ok := h.(Decode); ok && d.Codec == nil {
			return errors.New("a Decode needs a Codec")
		}
	}
	return nil
}

// version is the stream's definition's hash, all but how it shows and how fast it goes: a stream whose rows were
// written by another starts from scratch.
func (s *Sync) version() string {
	x := *s
	x.Doc, x.Gap = "", 0
	h := sha256.New()
	b, _ := json.Marshal(x)
	h.Write(b)
	fmt.Fprintf(h, "%T %T", s.Page, s.Change)
	for _, y := range s.Hydrate {
		fmt.Fprintf(h, " %T", y)
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// flow is where a stream is. Since are the parts' positions, times or tokens, of the last sync, and Listed when the
// last full listing began. While a sync is under way, Mode is list or delta, Began when it began, Parts what it
// goes through, At which it is on, Page and Mark where in it, and Tokens the parts' positions a listing took.
type flow struct {
	Since  map[string]string `json:"since,omitempty"`
	Listed string            `json:"listed,omitempty"`
	Mode   string            `json:"mode,omitempty"`
	Began  string            `json:"began,omitempty"`
	Parts  []string          `json:"parts,omitempty"`
	At     int               `json:"at,omitempty"`
	Page   string            `json:"page,omitempty"`
	Mark   string            `json:"mark,omitempty"`
	Tokens map[string]string `json:"tokens,omitempty"`
}

func (st flow) cursor() string { return cursorFor(st) }

// listing is a stream that restarts as a full listing, part one, keeping when it began.
func (st flow) listing() *Page {
	st.Mode, st.At, st.Page, st.Mark, st.Tokens = "list", 0, "", "", nil
	return &Page{More: true, Cursor: st.cursor()}
}

func (s *Sync) pull(c *Client, cursor string) (*Page, error) {
	var st flow
	if err := cursorOf(cursor, &st); err != nil {
		return nil, err
	}
	sc := maps.Clone(s.Vars)
	if sc == nil {
		sc = Obj{}
	}
	sc["stream"] = s.Name
	if st.Mode == "" {
		parts, err := s.parts(c, sc)
		if err != nil {
			return nil, err
		}
		st.Mode, st.Began, st.Parts, st.At, st.Page, st.Mark, st.Tokens = "delta", time.Now().Add(-overlap).UTC().Format(time.RFC3339Nano), parts, 0, "", "", nil
		_, token := s.Change.(Token)
		if s.Change == nil || !st.covers(parts) || !token && time.Since(stamp(st.Listed)) > relistAfter {
			st.Mode = "list"
		}
	}
	part := st.Parts[st.At]
	if part != "" {
		sc["part"] = part
	}
	if st.Mode == "delta" {
		return s.delta(c, st, part, sc)
	}
	if tk, ok := s.Change.(Token); ok && st.Tokens[part] == "" {
		ans, err := c.req(tk.Start, sc)
		if err != nil {
			return nil, err
		}
		if st.Tokens == nil {
			st.Tokens = map[string]string{}
		}
		st.Tokens[part] = text(pick(ans, tk.At))
	}
	ans, recs, err := s.read(c, s.Req, &st, sc)
	if err != nil {
		return nil, err
	}
	return s.done(c, st, part, sc, recs, nextOf(s.Page, ans, len(recs), st.Page), cmp.Or(st.Tokens[part], st.Began))
}

// covers says the last sync read exactly these parts, so a delta can start from where they each are.
func (st flow) covers(parts []string) bool {
	if len(st.Since) != len(parts) {
		return false
	}
	for _, p := range parts {
		if st.Since[p] == "" {
			return false
		}
	}
	return true
}

func (s *Sync) parts(c *Client, sc map[string]any) ([]string, error) {
	if s.Parts == nil {
		return []string{""}, nil
	}
	ans, err := c.req(s.Parts.Req, sc)
	if err != nil {
		return nil, err
	}
	id := s.Parts.ID
	if id == "" {
		id = "id"
	}
	parts := []string{}
	for _, it := range listOf(pick(ans, s.Parts.Items)) {
		if p := text(pick(it, id)); p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		return nil, unsupported("%s has none of what %s reads", c.p.Name, s.Name)
	}
	return parts, nil
}

// read sends req for the page st is on, and is its answer and records.
func (s *Sync) read(c *Client, req Req, st *flow, sc map[string]any) (any, []Record, error) {
	ans, err := s.send(c, req, st, sc)
	if err != nil {
		return nil, nil, err
	}
	return ans, s.records(pick(ans, s.Items)), nil
}

func (s *Sync) send(c *Client, req Req, st *flow, sc map[string]any) (any, error) {
	delete(sc, "page")
	if st.Page != "" {
		sc["page"] = st.Page
		if _, ok := s.Page.(NextURL); ok {
			req.URL, req.Query, req.exact = st.Page, nil, true
		}
	} else if o, ok := s.Page.(Offset); ok {
		sc["page"] = strconv.Itoa(o.From)
	}
	return c.req(req, sc)
}

func (s *Sync) records(items any) []Record {
	id := s.ID
	if id == "" {
		id = "id"
	}
	var out []Record
	for _, it := range listOf(items) {
		if m, ok := it.(map[string]any); ok {
			out = append(out, Record{ID: text(pick(m, id)), Data: m})
		}
	}
	return out
}

// delta reads what changed, as its Change says, from the position part has.
func (s *Sync) delta(c *Client, st flow, part string, sc map[string]any) (*Page, error) {
	var ans any
	var recs []Record
	var err error
	expired := func(codes []int) bool {
		r := (*Refusal)(nil)
		return errors.As(err, &r) && slices.Contains(codes, r.Status)
	}
	switch ch := s.Change.(type) {
	case NewestFirst:
		req := s.Req
		req.Query = maps.Clone(req.Query)
		if req.Query == nil {
			req.Query = Q{}
		}
		maps.Copy(req.Query, ch.Query)
		if ans, recs, err = s.read(c, req, &st, sc); err != nil {
			if expired(ch.Expired) {
				return st.listing(), nil
			}
			return nil, err
		}
		next := nextOf(s.Page, ans, len(recs), st.Page)
		since := stamp(st.Since[part])
		kept := recs[:0]
		for _, r := range recs {
			at := stamp(text(pick(r.Data, s.Updated)))
			if at.IsZero() {
				return st.listing(), nil
			}
			if !at.After(since) {
				next = ""
				break
			}
			kept = append(kept, r)
		}
		return s.done(c, st, part, sc, kept, next, st.Began)
	case SinceTime:
		if n, _ := strconv.Atoi(st.Page); ch.Most > 0 && n >= ch.Most {
			return st.listing(), nil
		}
		mark := stamp(st.Since[part])
		if m := stamp(st.Mark); m.After(mark) {
			mark = m
		}
		sc["since"] = mark.Format(time.RFC3339Nano)
		if ch.Ms {
			sc["since"] = strconv.FormatInt(mark.UnixMilli(), 10)
		}
		if ans, recs, err = s.read(c, ch.Req, &st, sc); err != nil {
			if expired(ch.Expired) {
				return st.listing(), nil
			}
			return nil, err
		}
		next := nextOf(s.Page, ans, len(recs), st.Page)
		if ch.Mark != "" && len(recs) > 0 && next != "" {
			t, _ := fillStr(string(ch.Mark), sc)
			if last := stamp(text(pick(recs[len(recs)-1].Data, Path(text(t))))); last.After(mark) {
				st.Mark, st.Page = last.UTC().Format(time.RFC3339Nano), ""
				return s.page(c, st, "delta", sc, recs, true)
			}
		}
		return s.done(c, st, part, sc, recs, next, st.Began)
	case Token:
		sc["token"] = st.Since[part]
		if ans, err = s.send(c, ch.Req, &st, sc); err != nil {
			if expired(ch.Expired) {
				return st.listing(), nil
			}
			return nil, err
		}
		return s.done(c, st, part, sc, ch.changes(ans), nextOf(s.Page, ans, 0, st.Page), cmp.Or(text(pick(ans, ch.At)), st.Since[part]))
	}
	return nil, fmt.Errorf("%s has no way to read what changed", s.Name)
}

// changes are the records an answer's changes name: each as its last change says, gone or there to read.
func (t Token) changes(ans any) []Record {
	gone, order := map[string]bool{}, []string{}
	note := func(paths []Path, e any, deleted bool) {
		for _, p := range paths {
			for _, x := range listOf(pick(e, p)) {
				id := text(x)
				if _, seen := gone[id]; !seen {
					order = append(order, id)
				}
				gone[id] = deleted
			}
		}
	}
	for _, e := range listOf(pick(ans, t.Each)) {
		note(t.Live, e, false)
		note(t.Gone, e, true)
	}
	out := make([]Record, len(order))
	for i, id := range order {
		out[i] = Record{ID: id, Deleted: gone[id], Data: map[string]any{"id": id}}
	}
	return out
}

// done is the page of recs, with the sync moving on: to the next page, or with the part at position, to the next
// part, or to its end. A listing keeps its positions to itself until it ends: until then it has not read them all.
func (s *Sync) done(c *Client, st flow, part string, sc map[string]any, recs []Record, next, position string) (*Page, error) {
	mode := st.Mode
	if next != "" {
		st.Page = next
		return s.page(c, st, mode, sc, recs, true)
	}
	if mode == "list" {
		if st.Tokens == nil {
			st.Tokens = map[string]string{}
		}
		st.Tokens[part] = position
	} else {
		st.Since[part] = position
	}
	st.Page, st.Mark, st.At = "", "", st.At+1
	if st.At < len(st.Parts) {
		return s.page(c, st, mode, sc, recs, true)
	}
	if mode == "list" {
		st.Listed, st.Since = st.Began, st.Tokens
	}
	st.Mode, st.Began, st.Parts, st.At, st.Tokens = "", "", nil, 0, nil
	return s.page(c, st, mode, sc, recs, false)
}

// page hydrates recs and is them as a page of the sync's mode.
func (s *Sync) page(c *Client, st flow, mode string, sc map[string]any, recs []Record, more bool) (*Page, error) {
	if err := s.hydrated(c, mode, recs, sc); err != nil {
		return nil, err
	}
	for i, r := range recs {
		if s.Updated != "" && !r.Deleted {
			recs[i].Updated = stamp(text(pick(r.Data, s.Updated)))
		}
	}
	return &Page{Records: recs, Cursor: st.cursor(), More: more, Full: mode == "list"}, nil
}

func nextOf(p Paging, ans any, n int, cur string) string {
	switch p := p.(type) {
	case NextURL:
		return text(pick(ans, p.At))
	case Cursor:
		if m, _ := pick(ans, p.More).(bool); p.More != "" && !m {
			return ""
		}
		return text(pick(ans, p.At))
	case Offset:
		if n < p.Size {
			return ""
		}
		at := p.From
		if cur != "" {
			at, _ = strconv.Atoi(cur)
		}
		return strconv.Itoa(at + p.Step)
	}
	return ""
}

// hydrated runs the stream's Hydrate over recs, those deleted left alone.
func (s *Sync) hydrated(c *Client, mode string, recs []Record, base map[string]any) error {
	data := func(r *Record) map[string]any {
		m, _ := r.Data.(map[string]any)
		return m
	}
	scope := func(r *Record) map[string]any {
		x := maps.Clone(base)
		maps.Copy(x, data(r))
		x["id"] = r.ID
		return x
	}
	for _, h := range s.Hydrate {
		switch h := h.(type) {
		case Fetch:
			err := each(len(recs), max(h.Par, 1), func(i int) error {
				r := &recs[i]
				if r.Deleted {
					return nil
				}
				ans, err := c.req(h.Req, scope(r))
				if f := (*Refusal)(nil); h.Gone && errors.As(err, &f) && f.Status == http.StatusNotFound {
					r.Deleted = true
					return nil
				}
				if err != nil {
					return err
				}
				if h.As != "" {
					data(r)[h.As] = ans
				} else if m, ok := ans.(map[string]any); ok {
					r.Data = m
				}
				return nil
			})
			if err != nil {
				return err
			}
		case Batch:
			inputs := []any{}
			for i := range recs {
				if !recs[i].Deleted {
					inputs = append(inputs, map[string]any{"id": recs[i].ID})
				}
			}
			if h.DeltaOnly && mode != "delta" || len(inputs) == 0 {
				continue
			}
			x := maps.Clone(base)
			x["inputs"] = inputs
			ans, err := c.req(h.Req, x)
			if err != nil {
				return err
			}
			got := map[string]any{}
			for _, e := range listOf(pick(ans, h.Results)) {
				got[text(pick(e, h.Key))] = e
			}
			for i := range recs {
				if e, ok := got[recs[i].ID]; ok && !recs[i].Deleted {
					data(&recs[i])[h.As] = e
				}
			}
		case Lookup:
			if len(recs) == 0 {
				continue
			}
			ans, err := c.req(h.Req, base)
			if err != nil {
				if h.Soft && !Busy(err) {
					continue
				}
				return err
			}
			for i := range recs {
				if !recs[i].Deleted {
					data(&recs[i])[h.As] = ans
				}
			}
		case Decode:
			for i := range recs {
				if recs[i].Deleted {
					continue
				}
				v, err := h.Codec.fn(recs[i].Data)
				if err != nil {
					return err
				}
				recs[i].Data = v
			}
		case Files:
			err := each(len(recs), max(h.Par, 1), func(i int) error {
				r := &recs[i]
				if r.Deleted {
					return nil
				}
				for _, f := range listOf(pick(r.Data, h.Each)) {
					a, _ := f.(map[string]any)
					name, kind := text(pick(a, h.Name)), text(pick(a, h.Mime))
					size, _ := strconv.Atoi(text(pick(a, h.Size)))
					if text(pick(a, h.Has)) == "" || size > h.Most || Extract == nil || !textual(name, kind) {
						continue
					}
					x := scope(r)
					maps.Copy(x, a)
					ans, err := c.req(h.Req, x)
					if Busy(err) {
						return err
					}
					if err == nil {
						b, _ := unb64(text(pick(ans, h.Data)))
						a[h.As] = bytesText(name, kind, b)
					}
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// scope is what a request of the account's own, not a script's, reads.
func (c *Client) scope() map[string]any {
	id, secret := c.Credentials()
	keys := map[string]any{}
	for k, v := range c.keys {
		keys[k] = v
	}
	return map[string]any{"client": map[string]any{"id": id, "secret": secret}, "token": c.Token(), "key": keys}
}

// req sends r, its templates read from sc, and is its answer, its numbers exact.
func (c *Client) req(r Req, sc map[string]any) (any, error) {
	u := r.URL
	if !r.exact {
		var err error
		if u, err = c.p.urlOf(r.URL, sc); err != nil {
			return nil, err
		}
	}
	if q := values(r.Query, sc); len(q) > 0 {
		sep := "?"
		if strings.Contains(u, "?") {
			sep = "&"
		}
		u += sep + q.Encode()
	}
	var body any
	switch {
	case r.Form != nil:
		body = values(r.Form, sc)
	case r.Body != nil:
		body, _ = fill(r.Body, sc)
	}
	var raw []byte
	if err := c.Send(string(cmp.Or(r.Method, GET)), u, nil, body, &raw); err != nil {
		return nil, err
	}
	return readAll(raw, c.p.Name)
}

// urlOf is a request's URL: an absolute one in a definition goes where it says, and a template that starts with a
// value, which a script may have given, stays on the API.
func (c *Connector) urlOf(t string, sc map[string]any) (string, error) {
	missing := ""
	u := varRe.ReplaceAllStringFunc(t, func(m string) string {
		v := varOf(m)
		x, ok := v.of(sc)
		if !ok {
			if missing == "" {
				missing = v.path
			}
			return ""
		}
		if v.raw {
			return text(x)
		}
		return url.PathEscape(text(x))
	})
	if missing != "" {
		return "", fmt.Errorf("%s needs %s", c.Name, missing)
	}
	u = strings.TrimSuffix(u, "?")
	switch {
	case strings.HasPrefix(u, "/"):
		return c.API + u, nil
	case !strings.HasPrefix(t, "{"), strings.HasPrefix(u, c.API+"/"):
		return u, nil
	}
	return "", fmt.Errorf("%s is neither a path, which starts with /, nor a URL of %s's API", u, c.Name)
}

// values is a query or form: a list repeats its key.
func values(q Q, sc map[string]any) url.Values {
	out := url.Values{}
	for k, t := range q {
		x, ok := fillStr(t, sc)
		if !ok {
			continue
		}
		if l, isList := x.([]any); isList {
			for _, e := range l {
				out.Add(k, text(e))
			}
		} else if s := text(x); s != "" {
			out.Set(k, s)
		}
	}
	return out
}

var varRe = regexp.MustCompile(`\{\+?[A-Za-z_][\w\-*/.]*,?(\|[^{}]*)?\}`)

// tvar is a {path} of a template.
type tvar struct {
	path, def string
	raw, join bool
	hasDef    bool
}

func varOf(m string) tvar {
	s := m[1 : len(m)-1]
	var v tvar
	s, v.raw = strings.CutPrefix(s, "+")
	s, v.def, v.hasDef = strings.Cut(s, "|")
	v.path, v.join = strings.CutSuffix(s, ",")
	return v
}

// of is the value of v in sc, its default when it has none.
func (v tvar) of(sc map[string]any) (any, bool) {
	x := pick(sc, Path(v.path))
	if none(x) {
		if !v.hasDef {
			return nil, false
		}
		var d any
		if json.Unmarshal([]byte(v.def), &d) == nil {
			return d, true
		}
		return v.def, true
	}
	if v.join {
		var s []string
		for _, e := range listOf(x) {
			s = append(s, text(e))
		}
		return strings.Join(s, ","), true
	}
	return x, true
}

// fillStr is the value of template t; false when it is only a {path} that has nothing.
func fillStr(t string, sc map[string]any) (any, bool) {
	if m := varRe.FindString(t); m != "" && m == t {
		return varOf(m).of(sc)
	}
	return varRe.ReplaceAllStringFunc(t, func(m string) string {
		x, _ := varOf(m).of(sc)
		return text(x)
	}), true
}

// fill is the value of a body or output, whose strings are templates, with those that have nothing left out.
func fill(t any, sc map[string]any) (any, bool) {
	switch v := t.(type) {
	case string:
		return fillStr(v, sc)
	case map[string]any:
		out := map[string]any{}
		for k, e := range v {
			if x, ok := fill(e, sc); ok {
				out[k] = x
			}
		}
		return out, true
	case []map[string]any:
		out := []any{}
		for _, e := range v {
			if x, ok := fill(e, sc); ok {
				out = append(out, x)
			}
		}
		return out, true
	case []any:
		out := []any{}
		for _, e := range v {
			if x, ok := fill(e, sc); ok {
				out = append(out, x)
			}
		}
		return out, true
	}
	return t, true
}

// pick walks path in v: nil when it leads nowhere.
func pick(v any, path Path) any {
	if path == "" {
		return v
	}
	head, rest, _ := strings.Cut(string(path), "/")
	if head == "*" {
		out := []any{}
		for _, e := range listOf(v) {
			x := pick(e, Path(rest))
			if l, ok := x.([]any); ok && strings.Contains(rest, "*") {
				out = append(out, l...)
			} else if x != nil {
				out = append(out, x)
			}
		}
		return out
	}
	m, _ := v.(map[string]any)
	return pick(m[head], Path(rest))
}

func none(v any) bool { return v == nil || v == "" }

func listOf(v any) []any {
	switch v := v.(type) {
	case []any:
		return v
	case []string:
		out := make([]any, len(v))
		for i, s := range v {
			out[i] = s
		}
		return out
	case nil:
		return nil
	}
	return []any{v}
}

func text(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case json.Number:
		return x.String()
	case int:
		return strconv.Itoa(x)
	case bool:
		return strconv.FormatBool(x)
	}
	return ""
}

// readAll is the JSON b, its numbers exact; nil when it is empty.
func readAll(b []byte, provider string) (any, error) {
	if len(bytes.TrimSpace(b)) == 0 {
		return nil, nil
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var v any
	if err := d.Decode(&v); err != nil {
		return nil, fmt.Errorf("%s answered with what is not JSON: %v", provider, err)
	}
	return v, nil
}

// sql is the query of a page's records, a jsonb array in $1, as the JSON of a Row each, in order.
func (m Map) sql() string {
	return "SELECT " + rowSQL(m.Cols, m.People, m.Orgs, m.Refs, m.Children) + " FROM jsonb_array_elements($1::jsonb) WITH ORDINALITY AS r(data, i) ORDER BY i"
}

func rowSQL(cols, people, orgs, refs map[string]SQL, kids []Kids) string {
	wrap := func(cols map[string]SQL, f string) string {
		var parts []string
		for _, k := range slices.Sorted(maps.Keys(cols)) {
			parts = append(parts, "'"+k+"', "+fmt.Sprintf(f, cols[k]))
		}
		return "jsonb_build_object(" + strings.Join(parts, ", ") + ")"
	}
	s := "jsonb_build_object('cols', " + wrap(cols, "(%s)") +
		", 'people', jsonb_strip_nulls(" + wrap(people, "archive.sync_email(%s)") + ")" +
		", 'orgs', jsonb_strip_nulls(" + wrap(orgs, "archive.sync_domain(%s)") + ")" +
		", 'refs', jsonb_strip_nulls(" + wrap(refs, "nullif(%s, '')") + ")"
	if len(kids) > 0 {
		var ks []string
		for _, k := range kids {
			ks = append(ks, "(SELECT coalesce(jsonb_agg("+rowSQL(k.Cols, k.People, k.Orgs, k.Refs, nil)+" || jsonb_build_object('table', '"+k.Table+"')), '[]'::jsonb) FROM "+string(k.From)+")")
		}
		s += ", 'children', " + strings.Join(ks, " || ")
	}
	return s + ")"
}

// Codec is a generic way to read or write a value, inside the engine.
type Codec struct {
	Name string
	fn   func(any) (any, error)
}

// The codecs. MIME reads what is at payload, a tree of MIME parts as JSON with mimeType, filename, headers, body
// and parts, into the headers people read, text, html and attachments; MIMEHead into the headers alone, and
// MIMEWords as MIME does for keeping: its text falls back on its HTML's words and is cut at maxText, its HTML goes,
// and it adds parties, the people of its headers, and sent, when its Date says. RFC5322 writes a message, maybe as
// a reply to parent, a message as JSON, into raw, base64url. Base64JSON reads JSON out of base64.
var (
	MIME       = &Codec{"mime", func(v any) (any, error) { return mimeRead(v, true), nil }}
	MIMEHead   = &Codec{"mime_head", func(v any) (any, error) { return mimeRead(v, false), nil }}
	MIMEWords  = &Codec{"mime_words", mimeWords}
	RFC5322    = &Codec{"rfc5322", rfc5322}
	Base64JSON = &Codec{"base64_json", func(v any) (any, error) {
		b, err := base64.StdEncoding.DecodeString(text(v))
		if err != nil {
			return nil, fmt.Errorf("data that is not base64: %v", err)
		}
		return readAll(b, "the push")
	}}
)

// MarshalJSON is what a stream's version sees of a codec.
func (c *Codec) MarshalJSON() ([]byte, error) { return json.Marshal(c.Name) }

func unb64(s string) ([]byte, error) {
	b, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return base64.RawURLEncoding.DecodeString(s)
	}
	return b, nil
}

func mimeRead(v any, body bool) map[string]any {
	m, _ := v.(map[string]any)
	out := maps.Clone(m)
	if out == nil {
		out = map[string]any{}
	}
	delete(out, "payload")
	root := pick(m, "payload")
	for _, h := range listOf(pick(root, "headers")) {
		if k := strings.ToLower(text(pick(h, "name"))); slices.Contains([]string{"from", "to", "cc", "bcc", "reply-to", "subject", "date"}, k) {
			out[k] = text(pick(h, "value"))
		}
	}
	if !body {
		return out
	}
	var plain, html strings.Builder
	atts := []any{}
	var walk func(p any)
	walk = func(p any) {
		name, kind := text(pick(p, "filename")), text(pick(p, "mimeType"))
		switch {
		case name != "":
			size, _ := strconv.Atoi(text(pick(p, "body/size")))
			atts = append(atts, map[string]any{"filename": name, "mimeType": kind, "size": size, "attachmentId": text(pick(p, "body/attachmentId"))})
		case kind == "text/plain":
			b, _ := unb64(text(pick(p, "body/data")))
			plain.Write(b)
		case kind == "text/html":
			b, _ := unb64(text(pick(p, "body/data")))
			html.Write(b)
		}
		for _, q := range listOf(pick(p, "parts")) {
			walk(q)
		}
	}
	if root != nil {
		walk(root)
	}
	out["text"], out["html"], out["attachments"] = plain.String(), html.String(), atts
	return out
}

func mimeWords(v any) (any, error) {
	m := mimeRead(v, true)
	words := m["text"].(string)
	if words == "" {
		words = htmlText(m["html"].(string))
	}
	m["text"] = clip(words, maxText)
	delete(m, "html")
	if t, err := mail.ParseDate(text(m["date"])); err == nil {
		m["sent"] = t.UTC().Format(time.RFC3339)
	}
	parties := []any{}
	for _, role := range []string{"from", "to", "cc", "bcc", "reply_to"} {
		for _, a := range addresses(text(m[strings.ReplaceAll(role, "_", "-")])) {
			parties = append(parties, map[string]any{"role": role, "name": a.Name, "address": a.Address})
		}
	}
	m["parties"] = parties
	return m, nil
}

func rfc5322(v any) (any, error) {
	in, _ := v.(map[string]any)
	f := func(k string) string { return text(in[k]) }
	to, subject := f("to"), f("subject")
	var hdr []string
	if parent, ok := in["parent"]; ok {
		h := map[string]string{}
		for _, x := range listOf(pick(parent, "payload/headers")) {
			h[strings.ToLower(text(pick(x, "name")))] = text(pick(x, "value"))
		}
		if to == "" {
			to = cmp.Or(h["reply-to"], h["from"])
		}
		if subject == "" {
			subject = h["subject"]
			if !strings.HasPrefix(strings.ToLower(subject), "re:") {
				subject = "Re: " + subject
			}
		}
		if id := h["message-id"]; id != "" {
			hdr = append(hdr, "In-Reply-To: "+id, "References: "+strings.TrimSpace(h["references"]+" "+id))
		}
	}
	for _, x := range []struct{ k, v string }{{"To", to}, {"Cc", f("cc")}, {"Bcc", f("bcc")}} {
		if x.v == "" {
			continue
		}
		as, err := mail.ParseAddressList(x.v)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", strings.ToLower(x.k), err)
		}
		s := make([]string, len(as))
		for i, a := range as {
			s[i] = a.String()
		}
		hdr = append(hdr, x.k+": "+strings.Join(s, ", "))
	}
	hdr = append(hdr, "Subject: "+mime.QEncoding.Encode("utf-8", subject), "MIME-Version: 1.0")
	var b bytes.Buffer
	part := func(w io.Writer, s string) {
		q := quotedprintable.NewWriter(w)
		q.Write([]byte(s))
		q.Close()
	}
	if f("html") == "" {
		hdr = append(hdr, "Content-Type: text/plain; charset=UTF-8", "Content-Transfer-Encoding: quoted-printable")
		part(&b, f("text"))
	} else {
		mw := multipart.NewWriter(&b)
		hdr = append(hdr, "Content-Type: multipart/alternative; boundary="+mw.Boundary())
		for _, p := range []struct{ typ, s string }{{"text/plain", f("text")}, {"text/html", f("html")}} {
			w, _ := mw.CreatePart(textproto.MIMEHeader{"Content-Type": {p.typ + "; charset=UTF-8"}, "Content-Transfer-Encoding": {"quoted-printable"}})
			part(w, p.s)
		}
		mw.Close()
	}
	return map[string]any{"raw": base64.URLEncoding.EncodeToString([]byte(strings.Join(hdr, "\r\n") + "\r\n\r\n" + b.String()))}, nil
}

func (l *Listen) compile(c *Connector) error {
	if !l.OIDC && len(l.Signs) == 0 {
		return errors.New("a push must prove itself: OIDC or Signs")
	}
	if l.Keys == "" || l.Key == "" {
		return errors.New("needs Keys and Key")
	}
	if err := l.Watch.check("watch", "client", "token", "key", "target", "account"); err != nil {
		return err
	}
	p := &Push{Env: l.Env, OIDC: l.OIDC, Renew: l.Renew}
	p.Watch = func(cl *Client, h Hook, old []Watch) ([]Watch, error) {
		sc := cl.scope()
		sc["target"], sc["account"] = h.Target, h.Account
		ans, err := cl.req(l.Watch, sc)
		if err != nil {
			return nil, err
		}
		sc["answer"] = ans
		k, _ := fillStr(l.Key, sc)
		key := strings.ToLower(text(k))
		if key == "" || key == "0" {
			return nil, fmt.Errorf("%s named nothing to watch", c.Name)
		}
		return []Watch{{Key: key, Expires: stamp(text(pick(ans, l.Expires)))}}, nil
	}
	if u := l.Unwatch; u != nil {
		p.Unwatch = func(cl *Client, w Watch) error {
			_, err := cl.req(*u, cl.scope())
			return err
		}
	}
	p.Hear = func(pr *Provider, r *http.Request, at string, body []byte, _ func(string) string) (*Heard, error) {
		if !l.OIDC {
			if err := l.verify(pr, r, at, body); err != nil {
				return nil, err
			}
		}
		v, err := readAll(body, "a push")
		if err != nil {
			return nil, fmt.Errorf("a push that is not JSON: %v", err)
		}
		if l.Codec != nil {
			if v, err = l.Codec.fn(pick(v, l.Wrap)); err != nil {
				return nil, err
			}
		}
		var keys []string
		for _, k := range listOf(pick(v, l.Keys)) {
			keys = append(keys, strings.ToLower(text(k)))
		}
		return &Heard{Keys: keysOf(keys...)}, nil
	}
	c.Push = p
	return nil
}

// verify checks a push against the first of Signs whose header it has.
func (l *Listen) verify(p *Provider, r *http.Request, at string, body []byte) error {
	for _, s := range l.Signs {
		got := r.Header.Get(s.Header)
		if got == "" || s.When != "" && r.Header.Get(s.When) != s.Is {
			continue
		}
		if s.Fresh != "" {
			ts := r.Header.Get(s.Fresh)
			ms, err := strconv.ParseInt(ts, 10, 64)
			if err != nil || time.Since(time.UnixMilli(ms)).Abs() > 5*time.Minute {
				return forged("%s %q", s.Fresh, ts)
			}
		}
		for _, ch := range s.Plain {
			at = strings.ReplaceAll(at, fmt.Sprintf("%%%02X", ch), string(ch))
		}
		sc := map[string]any{"method": r.Method, "url": at, "body": string(body), "secret": p.secret}
		msg := varRe.ReplaceAllStringFunc(s.Of, func(m string) string {
			v := varOf(m)
			if h, ok := strings.CutPrefix(v.path, "header/"); ok {
				return r.Header.Get(h)
			}
			x, _ := v.of(sc)
			return text(x)
		})
		var sum []byte
		if s.Digest == HMACSHA256 {
			m := hmac.New(sha256.New, []byte(p.secret))
			m.Write([]byte(msg))
			sum = m.Sum(nil)
		} else {
			x := sha256.Sum256([]byte(msg))
			sum = x[:]
		}
		want := base64.StdEncoding.EncodeToString(sum)
		if s.Enc == Hex {
			want = hex.EncodeToString(sum)
		}
		if !same(got, want) {
			return forged("%s", s.Header)
		}
		return nil
	}
	return forged("no signature it knows")
}

// Secrets are the environment variables that hold what connectors need of this server, each described from the
// connectors that read it, for Terraform and the docs.
func Secrets() map[string]string {
	type secret struct {
		names, opts []string
		host        string
		keys        bool
	}
	by := map[string]*secret{}
	for _, c := range All() {
		if c.Env == "" {
			continue
		}
		s := by[c.Env]
		if s == nil {
			s = &secret{host: strings.TrimPrefix(c.Host, "https://"), keys: len(c.Keys) > 0}
			by[c.Env] = s
		}
		s.names = append(s.names, c.Name)
		if c.Options != "" {
			s.opts = append(s.opts, c.Options)
		}
	}
	out := map[string]string{}
	for env, s := range by {
		who := s.names[0]
		if n := len(s.names); n > 1 {
			who = strings.Join(s.names[:n-1], ", ") + " and " + s.names[n-1]
		}
		format := "https://<client id>:<client secret>@" + s.host
		if s.keys {
			format = "https://<server key>@" + s.host
		}
		d := who + ": " + format
		if len(s.opts) > 0 {
			d += ", " + strings.Join(s.opts, ", ")
		}
		out[env] = d + ". A space leaves it off."
	}
	return out
}
