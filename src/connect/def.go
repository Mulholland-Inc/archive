package connect

import (
	"bytes"
	"cmp"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"math/big"
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
	"sync"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
)

// A connector is data: a sync and some actions over its provider's API. Its file holds a Connector literal, with
// no funcs, which Register compiles into what the rest of the server runs. The engine here owns everything the
// file does not say: where a sync is, when it lists whole, and what a page of it deletes.
//
// Values in a definition are read at run time, so the compiler cannot check them: a Path is a /-separated walk of
// an answer, where * is every element of a list, a number one of them, ** the first list of objects at any depth,
// **/key the list under key at any depth, a;b the first of two walks that finds anything, and a+b the lists of
// both, one after the other. A template is a
// string whose {path} is read from what the request knows. {path} is escaped in a URL, {+path} is not, {~path} is
// escaped a part at a time, {-path} is made a label of a host name, lower case with - for _, {path,} joins a list
// with commas or {path,x} with x, {'path} and {"path} are escaped to go in a string quoted so, and {path|x} is x
// when the path has nothing. A value that is only one {path} keeps its type, and with nothing there its key is left
// out.

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
// Header goes with it, by template. Tool asks the connector's MCP server instead, with Body as its arguments, those
// its schema does not name left out; Need are those the stream cannot do without. Or is sent in its place when it is
// refused as a bad request. Text says the answer is text, not JSON. Parts send it as multipart/related.
type Req struct {
	Method Method
	URL    string
	Query  Q
	Form   Q
	Body   any
	Read   bool
	Header map[string][]string `json:",omitempty"`
	Tool   string              `json:",omitempty"`
	Need   []string            `json:",omitempty"`
	Or     *Req                `json:",omitempty"`
	Text   bool                `json:",omitempty"`
	Parts  []Part              `json:",omitempty"`
	// exact is a URL a provider named, sent as it is.
	exact bool
}

// Part is a part of a multipart request, its Body sent as JSON unless it is a string.
type Part struct {
	Type string
	Body any
}

func (r Req) writes() bool { return r.URL != "" && r.Method != "" && r.Method != GET && !r.Read }

// Test asks something of a record or answer: that At has anything, or Is it, in any element of a list, whatever its
// case; Not, that it has something other; or Re, that it matches. Off turns the answer round.
type Test struct {
	At      Path
	Is, Not string `json:",omitempty"`
	Re      string `json:",omitempty"`
	Off     bool   `json:",omitempty"`
}

func (t Test) holds(v any) bool {
	x := pick(v, t.At)
	var ok bool
	switch {
	case t.Is != "":
		ok = slices.ContainsFunc(listOf(x), func(e any) bool { return strings.EqualFold(text(e), t.Is) })
	case t.Not != "":
		return !none(x) && !strings.EqualFold(text(x), t.Not)
	case t.Re != "":
		ok = reOf(t.Re).MatchString(text(x))
	default:
		ok = x != nil && x != ""
	}
	return ok != t.Off
}

func anyHolds(ts []Test, v any) bool {
	return slices.ContainsFunc(ts, func(t Test) bool { return t.holds(v) })
}

var regexps sync.Map

func reOf(s string) *regexp.Regexp {
	if r, ok := regexps.Load(s); ok {
		return r.(*regexp.Regexp)
	}
	r := regexp.MustCompile(s)
	regexps.Store(s, r)
	return r
}

// Deny is a refusal a definition knows: of one of Status, saying one of Says if it lists any, in any case. In Deny of
// a Sync, Why says what it means, reading {message}.
type Deny struct {
	Status []int
	Says   []string `json:",omitempty"`
	Why    string   `json:",omitempty"`
}

func (d Deny) is(r *Refusal) bool {
	if !slices.Contains(d.Status, r.Status) {
		return false
	}
	m := strings.ToLower(r.Message)
	return len(d.Says) == 0 || slices.ContainsFunc(d.Says, func(s string) bool { return strings.Contains(m, strings.ToLower(s)) })
}

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
	CatEmail     Category = "Email"
	CatCRM       Category = "CRM"
	CatPractice  Category = "Practice"
	CatFiles     Category = "Files"
	CatCalendar  Category = "Calendar"
	CatChat      Category = "Chat"
	CatDocs      Category = "Docs"
	CatCode      Category = "Code"
	CatFinance   Category = "Finance"
	CatMarketing Category = "Marketing"
	CatCommerce  Category = "Commerce"
	CatDental    Category = "Dental"
	CatProjects  Category = "Projects"
	CatPayroll   Category = "Payroll"

	CatCompliance Category = "Compliance"
	CatData       Category = "Data"

	IconMail     Icon = "mail"
	IconUsers    Icon = "users"
	IconList     Icon = "list"
	IconDrive    Icon = "drive"
	IconCalendar Icon = "calendar"
	IconInbox    Icon = "inbox"
	IconChat     Icon = "chat"
	IconNotebook Icon = "notebook"
	IconBranch   Icon = "branch"
	IconLedger   Icon = "ledger"
	IconCard     Icon = "card"
	IconBug      Icon = "bug"
)

// Who is how to ask whose account a token is for: the first of Names in the answer, lowercase when Lower. With
// Each, the first of Names of each of its elements that pass Where, joined by commas. Tool asks an MCP server's tool,
// without arguments, and reads what it says the way such tools say it: the first address or name in it, or its first
// line.
type Who struct {
	Req
	Names []Path
	Lower bool
	Each  Path
	Where Test
	Tool  string
}

// Paging is how an answer names the next page, which the next request reads as {page}.
type Paging interface{ paging() }

type (
	// NextURL is an answer naming the next page's URL, maybe on the API's own path, without the query keys of Drop.
	NextURL struct {
		At   Path
		Drop []string `json:",omitempty"`
	}
	// Cursor is an answer naming the next page's token, while More says so when there is a More. GraphQL pages
	// so, with {page} in its variables.
	Cursor struct{ At, More Path }
	// Offset is a count from From that goes up by Step while a page comes back with Size records.
	Offset struct{ From, Size, Step int }
	// ToolPage is an MCP tool that pages as its schema says: Max at a time in the first of Limit that it takes, and
	// the next page in the first of After, which the answer names under one of the usual keys, or by number in Page
	// while a page comes back full. Each is comma separated. A next page that is the one before, or starts as it did,
	// ends the listing.
	ToolPage struct {
		Limit, After, Page string
		Max                int
	}
)

func (NextURL) paging()  {}
func (Cursor) paging()   {}
func (Offset) paging()   {}
func (ToolPage) paging() {}

// Change is how a stream reads what changed since its last sync; nil lists it whole every time. A position the
// provider no longer takes, one of a Change's Expired statuses, starts a full listing. A provider whose Change
// cannot say what was deleted is listed whole relistAfter after it last was.
type Change interface{ change() }

type (
	// SinceTime asks with {since}, in Unix milliseconds when Ms. Mark, when given, is the time on a record: a
	// sync pages on from the last it read rather than by page, which Most limits, past which it lists whole. Items
	// is the answer's list of records, when it is not the listing's. An answer of Cap records or more is cut short,
	// and lists whole too. Stamp, a record's own time in the provider's clock, is what {since} is rather than
	// Archive's: the newest it has read. Lasts says it tells of deletes, so it never lists whole for them; Within how
	// long ago it still reaches.
	SinceTime struct {
		Req     Req
		Ms      bool
		Mark    Path
		Most    int
		Expired []int
		Items   Path          `json:",omitempty"`
		Cap     int           `json:",omitempty"`
		Stamp   Path          `json:",omitempty"`
		Lasts   bool          `json:",omitempty"`
		Within  time.Duration `json:",omitempty"`
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
	// Resume is a provider whose answers name where the next sync goes on from: at At on the last page. Req asks from
	// it as {token}; with none it is a URL. With Start, it comes from there before the listing, at From. Its answers
	// list records at Items, or changes with each record at Inner and its id at ID, Gone those that are records gone.
	// Stale says what else means it no longer takes the position.
	Resume struct {
		Start   *Req `json:",omitempty"`
		From    Path `json:",omitempty"`
		At      Path
		Req     Req
		Items   Path   `json:",omitempty"`
		Inner   Path   `json:",omitempty"`
		ID      Path   `json:",omitempty"`
		Gone    []Test `json:",omitempty"`
		Expired []int
		Stale   []Deny `json:",omitempty"`
	}
)

func (SinceTime) change()   {}
func (NewestFirst) change() {}
func (Token) change()       {}
func (Resume) change()      {}

// Parts fans one stream out over many of a thing, like calendars, each read as {part}, the rest of it as {in/...}.
// The parts a walk finds are read from where each ended before: one new is read whole on its own, and one missing
// from two walks in a row lists every part whole, which deletes what it had.
//
// A part is ID, by template, {id} when empty, Vals what else it keeps. The things come from Req's answer at Items,
// every page of it, or are Fixed, or are the dates of Span. Soft are refusals that mean there are none. Skip are those
// that are no part, and Not requests that name one that is none, by id, when they do not answer 404. Deep lists those
// under one that passes DeepIf the way Req does, whose {id} is its. Under lists more under each, which read it as
// {up/...}, and those are the parts. First skips this level once the stream has listed whole. Also are other places
// to find them.
type Parts struct {
	Req    Req      `json:",omitzero"`
	Items  Path     `json:",omitempty"`
	Page   Paging   `json:",omitempty"`
	ID     string   `json:",omitempty"`
	Vals   Obj      `json:",omitempty"`
	Fixed  []Obj    `json:",omitempty"`
	Span   *Span    `json:",omitempty"`
	Soft   []int    `json:",omitempty"`
	Skip   []Test   `json:",omitempty"`
	Not    []Req    `json:",omitempty"`
	Deep   *Req     `json:",omitempty"`
	DeepIf Test     `json:",omitzero"`
	Under  *Parts   `json:",omitempty"`
	First  bool     `json:",omitempty"`
	Also   []*Parts `json:",omitempty"`
}

// Shift is a move in time, in years, months and days.
type Shift [3]int

func (s Shift) from(t time.Time) time.Time { return t.AddDate(s[0], s[1], s[2]) }

// Span is the dates from a start to Last, a shift from today, or the first of this month with Month, a Step at a
// time, each as {start} and {end}. It starts at First, a shift from Last. With From, on the walk that lists whole it
// starts at that date moved by Back, First when there is no date. Once it has listed whole, it starts at Then.
type Span struct {
	Last, First, Step, Back, Then Shift
	From                          string `json:",omitempty"`
	Month                         bool   `json:",omitempty"`
}

// Hydrate adds to the records of a page, after it is read.
type Hydrate interface{ hydrate() }

type (
	// Fetch is a request per record, Par at once, whose answer is the record, or goes at As, or is added to it with
	// Merge. With Gone, a 404 is a record deleted there. With If, only records that pass. With Once, only one request a
	// page for each value of that template, which a record with the same shares. With Soft, only a provider that is
	// busy fails the page; with Bare a tool the server does not have leaves nothing, not a stream skipped. Items takes
	// that much of the answer. More and Each read pages {n} of a list: Each from every page, while More says more.
	Fetch struct {
		Req   Req
		As    string
		Par   int
		Gone  bool
		If    Test   `json:",omitzero"`
		Once  string `json:",omitempty"`
		Soft  bool   `json:",omitempty"`
		Bare  bool   `json:",omitempty"`
		Merge bool   `json:",omitempty"`
		Items Path   `json:",omitempty"`
		More  Path   `json:",omitempty"`
		Each  Path   `json:",omitempty"`
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
	// Decode is a codec over each record, or over what is at At, going to As, for those that pass If.
	Decode struct {
		Codec *Codec
		At    Path   `json:",omitempty"`
		As    string `json:",omitempty"`
		If    Test   `json:",omitzero"`
	}
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
	// Download reads the words of a file its Req answers with, down to 25 MB, at As: the record's own, or each of
	// the list at Each that passes If, reading the record as {up/...}. Name and Mime are templates, and Size a path. One
	// that already has As is left alone. Only a provider that is busy fails the page.
	Download struct {
		Each       Path `json:",omitempty"`
		If         Test `json:",omitzero"`
		Req        Req
		Name, Mime string
		Size       Path
		As         string
		Par        int
	}
)

func (Fetch) hydrate()    {}
func (Batch) hydrate()    {}
func (Lookup) hydrate()   {}
func (Decode) hydrate()   {}
func (Files) hydrate()    {}
func (Download) hydrate() {}

// Map is what a record is as a row, each column a SQL expression over data. People are its known_person columns
// by address and Orgs its known_org columns by web domain, or EINs by EIN, Refs its columns that are a row of the
// connection by that row's id there. Children are rows of other tables that belong to it.
type Map struct {
	Table              string
	Cols               map[string]SQL
	People, Orgs, Refs map[string]SQL
	Children           []Kids
	EINs               map[string]SQL `json:",omitempty"`
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
// records, ID each one's id, id when empty, and Updated when it last changed there. An answer without its Items is
// refused, so a full listing never ends empty by accident; Absent is a key a provider's answer has when it leaves
// Items out for having none, and Empty says an answer with nothing in it at all, as Zoho's 204 is, has none.
//
// Key makes the id when ID cannot, the first that reads to anything. Set adds to each record, by template. Gone are
// the records that are deleted there, and Skip those that are no record. Every is how long after it caught up the
// stream reads again. Keep says a listing deletes nothing, and Short what in an answer says this listing is short of
// everything, so that it deletes nothing either. Memo is what a record that has not changed still reads the same: such
// a record is only seen, and not read further. Deny are refusals that skip the stream, Restart those that start it
// again when a page was under way, and Pass those that pass over a part until the next walk.
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
	Absent    Path          `json:",omitempty"`
	Empty     bool          `json:",omitempty"`
	Key       []Reads       `json:",omitempty"`
	Set       Obj           `json:",omitempty"`
	Gone      []Test        `json:",omitempty"`
	Skip      []Test        `json:",omitempty"`
	Every     time.Duration `json:",omitempty"`
	Keep      bool          `json:",omitempty"`
	Short     Test          `json:",omitzero"`
	Memo      string        `json:",omitempty"`
	Deny      []Deny        `json:",omitempty"`
	Restart   []int         `json:",omitempty"`
	Pass      []int         `json:",omitempty"`
}

// Reads is what template Of reads to, all of it there, or the first group of Find in that.
type Reads struct {
	Of   string
	Find string `json:",omitempty"`
}

func (k Reads) read(sc map[string]any) string {
	whole := true
	s := varRe.ReplaceAllStringFunc(k.Of, func(m string) string {
		x, ok := varOf(m).of(sc)
		if !ok {
			whole = false
		}
		return text(x)
	})
	if !whole {
		return ""
	}
	if k.Find != "" {
		f := reOf(k.Find).FindStringSubmatch(s)
		if f == nil {
			return ""
		}
		return f[1]
	}
	return s
}

// relistAfter is how long after a full listing a provider that cannot say what was deleted lists whole again, and
// overlap how far before a sync's start the next reads from, against the provider's clock.
const (
	relistAfter = 7 * 24 * time.Hour
	overlap     = time.Minute
)

// Plan is an action: its Steps in order, each reading the input and the steps' As before it, {now}, a fresh {nonce} and {meta/...}, and Out, or
// else the answer of the last. Clamp keeps a number within what a provider takes: { def, most }. Enum names the only
// values an input may be.
type Plan struct {
	Doc   string
	Clamp map[string][2]int
	Enum  map[string][]string
	Steps []Step
	Out   any
}

// Step is a request, a codec, the one over the other, or only In's value. Each runs it for every element of a list,
// when there is one, which reads the element's fields too and the element as {it}; Onto puts each result on its
// element. It runs only with If, without Unless and when Only holds, of each element with Each. A codec reads the
// request's answer, or In. Find is the first group of a regexp over the value. Fail ends the action with that message.
// With Page, the request is every page of what is at Items. Soft are refusals that are nothing rather than failure.
// Words reads a file's words. As may be a path into what a step before named.
type Step struct {
	Req
	As         string
	If, Unless Path
	Only       Test
	Each       Path
	Onto       string
	Codec      *Codec
	In         any
	Find       string
	Fail       string
	Page       Paging
	Items      Path
	Soft       []int
	Words      *Download
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
	sc["now"], sc["nonce"], sc["meta"] = time.Now().UTC().Format(time.RFC3339), nonce(), c.scope()["meta"]
	for k, lim := range p.Clamp {
		n, _ := strconv.Atoi(text(sc[k]))
		sc[k] = clamp(n, lim[0], lim[1])
	}
	last, err := runSteps(c, p.Steps, sc)
	if err != nil {
		return nil, err
	}
	if p.Out != nil {
		out, _ := fill(p.Out, sc)
		return out, nil
	}
	return last, nil
}

// runSteps runs steps over sc, and is the answer of the last that ran.
func runSteps(c *Client, steps []Step, sc map[string]any) (any, error) {
	var last any
	for _, s := range steps {
		if s.If != "" && !truth(pick(sc, s.If)) || s.Unless != "" && truth(pick(sc, s.Unless)) || s.Each == "" && s.Only.At != "" && !s.Only.holds(sc) {
			continue
		}
		if s.Each != "" && pick(sc, s.Each) == nil {
			continue
		}
		var out any
		var err error
		if s.Each == "" {
			if out, err = s.once(c, sc, nil); err != nil {
				return nil, err
			}
		} else {
			list := []any{}
			for _, e := range listOf(pick(sc, s.Each)) {
				x := maps.Clone(sc)
				m, ok := e.(map[string]any)
				if ok {
					maps.Copy(x, m)
				}
				x["it"] = e
				if s.Only.At != "" && !s.Only.holds(x) {
					continue
				}
				r, err := s.once(c, x, e)
				if err != nil {
					return nil, err
				}
				if s.Onto != "" && ok {
					m[s.Onto] = r
					r = m
				}
				list = append(list, r)
			}
			out = list
		}
		if s.As != "" {
			put(sc, s.As, out)
		}
		last = out
	}
	return last, nil
}

// put sets what path walks to in sc, making what it walks through.
func put(sc map[string]any, path string, v any) {
	head, rest, more := strings.Cut(path, "/")
	if !more {
		sc[head] = v
		return
	}
	m, _ := sc[head].(map[string]any)
	if m == nil {
		m = map[string]any{}
		sc[head] = m
	}
	put(m, rest, v)
}

func (s Step) once(c *Client, sc map[string]any, e any) (v any, err error) {
	if s.Fail != "" {
		msg, _ := fillStr(s.Fail, sc)
		return nil, errors.New(text(msg))
	}
	v = e
	switch {
	case s.Words != nil:
		if v, err = c.words(*s.Words, sc); err != nil {
			return nil, err
		}
	case s.Req.URL != "" || s.Tool != "":
		if s.Page != nil {
			v, err = c.every(s.Req, s.Items, s.Page, sc)
		} else {
			v, err = c.req(s.Req, sc)
		}
		if f := (*Refusal)(nil); errors.As(err, &f) && slices.Contains(s.Soft, f.Status) {
			v, err = []any{}, nil
		}
		if err != nil {
			return nil, err
		}
	}
	if s.In != nil {
		v, _ = fill(s.In, sc)
	}
	if s.Find != "" {
		f := reOf(s.Find).FindStringSubmatch(text(v))
		v = nil
		if f != nil {
			v = f[1]
		}
	}
	if s.Codec != nil {
		return s.Codec.fn(v)
	}
	return v, nil
}

// Sign is a way a push proves it is the provider's, when it has the header Header and, if it says, When is Is.
// The signature is Digest of Of, a template of {method}, {url}, {body}, {secret} and {header/<name>}, with HMAC
// keyed by the app's secret, or the hook option of its variable with Hook, or the secret a handshake left with Kept,
// after Pre. Plain are characters {url} has not percent-escaped, as the provider signs it, and Fresh a header of its
// time in Unix milliseconds, which must be under 5 minutes old.
//
// With Key, Header is a JWS signed with ES256 by a key of the provider's, which Key asks for with the server's
// client and the JWS's kid as {kid}, at JWK in its answer, kept by kid: its claim Claim is what Digest and Enc make
// of Of, and its iat must be under 5 minutes old.
type Sign struct {
	Header, When, Is string
	Digest           Digest
	Enc              Enc
	Of               string
	Plain, Fresh     string
	Hook, Kept       bool
	Pre              string
	Key              *Req   `json:",omitempty"`
	JWK              Path   `json:",omitempty"`
	Claim            string `json:",omitempty"`
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
// the first whose header a push has deciding.
//
// Asking: Over is what to watch one by one, each read as {part} and {in/...}; without it the account is watched
// once. Ask comes first, and Watch reads its answer as {ask}, with {account}, {client}, {token}, {conn} the connection,
// a fresh {nonce} and {secret}, and {until} and {until_ms} the time Until from now. Mint is a key made before asking,
// as {key}, which goes on {hook}, where the provider pushes; else Key is the template of what they name it by, with
// the answer at {answer}. Resource is what it is called there, and Secret keeps {secret} to check pushes with. It ends
// at Expires in the answer, or Lasts from now, and Renew is how long before that a sync renews it.
//
// Renewing: OfPart says which of Over an old watch is on, reading {key}, {resource}. Extend keeps such a watch, its
// answer passing Alive if it says, which a 404 or anything else with Alive replaces. Replace makes new watches and
// ends the old. An old watch on nothing there any more is ended. Skip are refusals that mean this one cannot be
// watched, and Lax says any refusal that is not a busy provider's does. Unwatch ends one.
//
// Hearing: Hello is the handshake a provider opens a watch with. A push's Keys are read from its body, or from what
// Codec makes of what is at Wrap, the first of them that has any, or from Kept, which proves them too. Only is what a
// push says for it to mean anything.
type Listen struct {
	Env      string
	OIDC     bool
	Signs    []Sign
	Over     *Parts
	Ask      *Req
	Watch    Req
	Mint     string
	Key      string
	Until    time.Duration
	Resource string
	Secret   bool
	Expires  Path
	Lasts    time.Duration
	Unwatch  *Req
	Renew    time.Duration
	OfPart   *Reads
	Extend   *Req
	Alive    Test
	Replace  bool
	Skip     []int
	Lax      bool
	Lower    bool
	Hello    []Hello
	Wrap     Path
	Codec    *Codec
	Keys     []Path
	Kept     *Kept
	Only     Test
}

// Hello is a handshake: a query parameter whose value is answered as it came, in plain text, or with Header, a header
// answered with itself, and kept as the secret of the watch the URL names.
type Hello struct{ Query, Header string }

// Kept is how a push names a watch and proves it with the secret Archive kept. With ByQuery the key is in the push's
// URL, its Sign proves it, and a push of a watch with no secret starts nothing. With IDHeader and TokenHeader they are
// headers, and a push whose Quiet header is QuietIs says nothing changed. With Each they are at ID and Token in each
// element of the body.
type Kept struct {
	ByQuery               bool
	IDHeader, TokenHeader string
	Quiet, QuietIs        string
	Each, ID, Token       Path
}

// compile fills in what the rest of the server runs from a connector's data, and fails on data that cannot run.
func (c *Connector) compile() error {
	if c.API == "" && c.MCP == "" {
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
	if c.Trade != nil {
		if err := c.Trade.check("trade", "key", "client", "option"); err != nil {
			return err
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
		if w.Tool != "" {
			c.Account = mcpAccount(w.Tool)
		} else {
			if err := w.Req.check("who", "client", "token", "key", "meta", "option"); err != nil {
				return err
			}
			c.Account = func(cl *Client) (string, error) { return w.read(cl, c) }
		}
	}
	if r := c.RevokeBy; r != nil {
		c.RevokeWith = func(rv *Revocation) error { return r.revoke(rv) }
	}
	if l := c.Link; l != nil {
		if len(c.Keys) == 0 {
			return errors.New("link: a widget hands over Keys")
		}
		if err := l.check(); err != nil {
			return err
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
		c.Streams = append(c.Streams, &Stream{Name: s.Name, Doc: s.Doc, Gap: s.Gap, Every: s.Every, Table: s.To.Table, Pull: s.pull, SQL: s.To.sql(), Version: s.version()})
	}
	return nil
}

// read is the account Who asks for.
func (w *Who) read(cl *Client, c *Connector) (string, error) {
	ans, err := cl.req(w.Req, cl.scope())
	if err != nil {
		return "", err
	}
	first := func(v any) string {
		for _, n := range w.Names {
			for _, e := range listOf(pick(v, n)) {
				if s := strings.TrimSpace(text(e)); s != "" {
					return s
				}
			}
		}
		return ""
	}
	s := first(ans)
	if w.Each != "" {
		var names []string
		for _, e := range listOf(pick(ans, w.Each)) {
			if n := first(e); n != "" && (w.Where.At == "" || w.Where.holds(e)) {
				names = append(names, n)
			}
		}
		s = strings.Join(names, ", ")
	}
	if w.Lower {
		s = strings.ToLower(s)
	}
	if s == "" && len(c.Keys) > 0 {
		s = c.Name
	}
	return s, nil
}

// revoke ends a grant as r says, which reads {client}, {refresh}, {basic} the client as HTTP Basic, {meta/...} what the
// grant kept, {key/...} a key connector's keys, and {access} an access token of the grant, only asked for when it does.
func (r *Req) revoke(rv *Revocation) error {
	id, secret := rv.Client()
	sc := rv.p.metaScope(rv.Meta)
	sc["client"], sc["refresh"], sc["basic"] = map[string]any{"id": id, "secret": secret}, rv.Refresh, basic(id, secret)
	if len(rv.p.Keys) > 0 {
		keys := map[string]any{}
		for k, v := range keysFrom(rv.Refresh) {
			keys[k] = v
		}
		sc["key"] = keys
	}
	if slices.Contains(r.roots(), "access") {
		at, err := rv.Access()
		if err != nil {
			return err
		}
		sc["access"] = at
	}
	u, err := expand(r.URL, sc)
	if err != nil {
		return err
	}
	var hdr http.Header
	if r.Header != nil {
		hdr = r.headers(sc)
	}
	if r.Form != nil {
		return rv.Form(u, hdr, values(r.Form, sc))
	}
	body, _ := fill(r.Body, sc)
	return rv.Send(string(cmp.Or(r.Method, POST)), u, hdr, body)
}

// roots are the first parts of the paths r reads.
func (r Req) roots() []string {
	var seen []string
	note := func(t string) {
		for _, m := range varRe.FindAllString(t, -1) {
			if varOf(m).hasDef {
				continue
			}
			for _, alt := range strings.Split(varOf(m).path, ";") {
				root, _, _ := strings.Cut(alt, "/")
				seen = append(seen, root)
			}
		}
	}
	note(r.URL)
	for _, t := range r.Query {
		note(t)
	}
	for _, t := range r.Form {
		note(t)
	}
	for _, ts := range r.Header {
		for _, t := range ts {
			note(t)
		}
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
	for _, x := range r.Parts {
		note(x.Type)
		walk(x.Body)
	}
	if r.Or != nil {
		seen = append(seen, r.Or.roots()...)
	}
	return seen
}

func (r Req) check(where string, roots ...string) error {
	switch r.Method {
	case "", GET, POST, PUT, PATCH, DELETE:
	default:
		return fmt.Errorf("%s: no method %q", where, r.Method)
	}
	if r.URL == "" && r.Tool == "" {
		return fmt.Errorf("%s: no URL", where)
	}
	for _, s := range r.roots() {
		if !slices.Contains(roots, s) {
			return fmt.Errorf("%s: {%s} is nothing it knows; it knows %s", where, s, strings.Join(roots, ", "))
		}
	}
	return nil
}

func (a *Action) check() error {
	roots := append(slices.Collect(maps.Keys(a.Input.Properties)), "now", "nonce", "meta")
	for i, s := range a.plan.Steps {
		if s.URL == "" && s.Tool == "" && s.Codec == nil && s.In == nil && s.Fail == "" && s.Words == nil {
			return fmt.Errorf("step %d is nothing to do", i)
		}
		if s.URL != "" && s.Each == "" {
			if err := s.Req.check(fmt.Sprintf("step %d", i), append(roots, "page")...); err != nil {
				return err
			}
		}
		if s.As != "" {
			root, _, _ := strings.Cut(s.As, "/")
			roots = append(roots, root)
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
	roots := append(slices.Collect(maps.Keys(s.Vars)), "stream", "part", "in", "page", "key", "meta", "option")
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
	case Resume:
		if ch.At == "" {
			return errors.New("a Resume needs At")
		}
		if ch.Req.URL != "" {
			if err := ch.Req.check("delta", append(roots, "token")...); err != nil {
				return err
			}
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
	x.Doc, x.Gap, x.Every, x.Absent, x.Empty, x.Deny, x.Restart, x.Pass = "", 0, 0, "", false, nil, nil, nil
	h := sha256.New()
	b, _ := json.Marshal(x)
	h.Write(b)
	fmt.Fprintf(h, "%T %T", s.Page, s.Change)
	for _, y := range s.Hydrate {
		fmt.Fprintf(h, " %T", y)
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// flow is where a stream is. Since are the parts' positions, times or tokens, of the last sync, Listed when the
// last full listing began, Missed the parts the walk before did not find, and Seen what Memo read of each record in
// the last listing. While a sync is under way, Mode is list or delta, Began when it began, Parts what it goes through
// and Info what they keep, At which it is on, Page and Mark where in it, Tokens the parts' positions a listing took,
// Max the newest Stamp it has read, First the first record of the page before, Partial that its listing is short of
// everything, and Next what Seen will be.
type flow struct {
	Since   map[string]string `json:"since,omitempty"`
	Listed  string            `json:"listed,omitempty"`
	Mode    string            `json:"mode,omitempty"`
	Began   string            `json:"began,omitempty"`
	Parts   []string          `json:"parts,omitempty"`
	At      int               `json:"at,omitempty"`
	Page    string            `json:"page,omitempty"`
	Mark    string            `json:"mark,omitempty"`
	Tokens  map[string]string `json:"tokens,omitempty"`
	Info    map[string]Obj    `json:"info,omitempty"`
	Missed  []string          `json:"missed,omitempty"`
	Max     string            `json:"max,omitempty"`
	First   string            `json:"first,omitempty"`
	Partial bool              `json:"partial,omitempty"`
	Seen    map[string]string `json:"seen,omitempty"`
	Next    map[string]string `json:"next,omitempty"`
}

func (st flow) cursor() string { return cursorFor(st) }

// listing is a stream that restarts as a full listing, part one, keeping when it began.
func (st flow) listing() *Page {
	st.Mode, st.At, st.Page, st.Mark, st.Tokens, st.First, st.Partial, st.Next = "list", 0, "", "", nil, "", false, nil
	return &Page{More: true, Cursor: st.cursor()}
}

// over is st with no sync under way, its memory kept.
func (st flow) over() flow {
	st.Mode, st.Began, st.Parts, st.Info, st.At, st.Page, st.Mark, st.Tokens, st.Max, st.First, st.Partial, st.Next = "", "", nil, nil, 0, "", "", nil, "", "", false, nil
	return st
}

func (s *Sync) pull(c *Client, cursor string) (*Page, error) {
	var st flow
	if err := cursorOf(cursor, &st); err != nil {
		return nil, err
	}
	page, err := s.step(c, st)
	if err == nil {
		return page, nil
	}
	if r := (*Refusal)(nil); errors.As(err, &r) {
		if st.Page != "" && slices.Contains(s.Restart, r.Status) {
			return &Page{More: true, Cursor: st.over().cursor()}, nil
		}
		// A listing's next page can expire as a position does, and a cursor kept on it would fail for good.
		if ch, ok := s.Change.(Resume); ok && st.Page != "" && (slices.Contains(ch.Expired, r.Status) || slices.ContainsFunc(ch.Stale, func(d Deny) bool { return d.is(r) })) {
			return st.listing(), nil
		}
		for _, d := range s.Deny {
			if d.is(r) {
				x := maps.Clone(s.Vars)
				if x == nil {
					x = Obj{}
				}
				x["message"], x["stream"] = r.Message, s.Name
				why, _ := fillStr(d.Why, x)
				return nil, unsupported("%s", text(why))
			}
		}
	}
	return nil, err
}

func (s *Sync) step(c *Client, st flow) (*Page, error) {
	sc := maps.Clone(s.Vars)
	if sc == nil {
		sc = Obj{}
	}
	sc["stream"] = s.Name
	cs := c.scope()
	sc["key"], sc["meta"], sc["option"] = cs["key"], cs["meta"], cs["option"]
	if st.Mode == "" {
		parts, info, err := s.parts(c, sc, st)
		if err != nil {
			return nil, err
		}
		if s.Parts != nil && len(parts) == 0 {
			if len(st.Since) > 0 {
				return nil, fmt.Errorf("%s listed none of what it listed before; Archive keeps what it has and asks again next sync", c.p.Name)
			}
			return &Page{Cursor: st.cursor()}, nil
		}
		st.Mode, st.Began, st.Parts, st.Info, st.At, st.Page, st.Mark, st.Tokens = "delta", time.Now().Add(-overlap).UTC().Format(time.RFC3339Nano), parts, info, 0, "", "", nil
		if s.lists(&st, parts) {
			st.Mode = "list"
		}
	}
	part := st.Parts[st.At]
	if part != "" {
		sc["part"] = part
		sc["in"] = st.Info[part]
	}
	page, err := s.part(c, st, part, sc)
	if r := (*Refusal)(nil); errors.As(err, &r) && slices.Contains(s.Pass, r.Status) {
		st.Partial = st.Partial || st.Mode == "list"
		return s.done(c, st, part, sc, nil, "", st.Since[part])
	}
	return page, err
}

// lists says this sync reads everything rather than what changed.
func (s *Sync) lists(st *flow, parts []string) bool {
	if s.Change == nil {
		return true
	}
	lasts := false
	switch ch := s.Change.(type) {
	case Token, Resume:
		lasts = true
	case SinceTime:
		lasts = ch.Lasts
		if ch.Within > 0 && time.Since(stamp(st.Since[""])) > ch.Within {
			return true
		}
	}
	if !lasts && time.Since(stamp(st.Listed)) > relistAfter {
		return true
	}
	if s.Parts == nil {
		return !st.covers(parts)
	}
	if len(st.Since) == 0 {
		return true
	}
	if s.Keep {
		return false
	}
	var gone []string
	again := false
	for _, p := range slices.Sorted(maps.Keys(st.Since)) {
		if !slices.Contains(parts, p) {
			gone = append(gone, p)
			again = again || slices.Contains(st.Missed, p)
		}
	}
	st.Missed = gone
	return again
}

// part reads the page of part that st is on.
func (s *Sync) part(c *Client, st flow, part string, sc map[string]any) (*Page, error) {
	if st.Mode == "delta" && st.Since[part] != "" {
		return s.delta(c, st, part, sc)
	}
	var position string
	if start, at := s.start(); start != nil {
		if st.Tokens[part] == "" {
			ans, err := c.req(*start, sc)
			if err != nil {
				return nil, err
			}
			if st.Tokens == nil {
				st.Tokens = map[string]string{}
			}
			st.Tokens[part] = text(pick(ans, at))
		}
		position = st.Tokens[part]
	}
	ans, recs, err := s.read(c, s.Req, &st, sc)
	if err != nil {
		return nil, err
	}
	if s.Short.At != "" && st.Mode == "list" && s.Short.holds(ans) {
		st.Partial = true
	}
	next := s.next(c, ans, recs, &st)
	if r, ok := s.Change.(Resume); ok && r.Start == nil {
		position = text(pick(ans, r.At))
	}
	if t, ok := s.Change.(SinceTime); ok && t.Stamp != "" {
		position = max(st.Max, st.Since[part])
	}
	return s.done(c, st, part, sc, recs, next, cmp.Or(position, s.began(st)))
}

// began is the position a time gives, for a stream whose answers give it none.
func (s *Sync) began(st flow) string {
	switch ch := s.Change.(type) {
	case Resume:
		return ""
	case SinceTime:
		if ch.Stamp != "" {
			return ""
		}
	}
	return st.Began
}

// start is what asks for the position before a listing, and where its answer has it.
func (s *Sync) start() (*Req, Path) {
	switch ch := s.Change.(type) {
	case Token:
		return &ch.Start, ch.At
	case Resume:
		return ch.Start, ch.From
	}
	return nil, ""
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

func (s *Sync) parts(c *Client, sc map[string]any, st flow) ([]string, map[string]Obj, error) {
	if s.Parts == nil {
		return []string{""}, nil, nil
	}
	return s.Parts.find(c, sc, nil, st.Listed != "")
}

// find are the parts p finds under up, each once, and what each keeps. again says the stream has listed whole.
func (p *Parts) find(c *Client, sc, up map[string]any, again bool) ([]string, map[string]Obj, error) {
	ids, info := []string{}, map[string]Obj{}
	add := func(more []string, of map[string]Obj) {
		for _, id := range more {
			if _, ok := info[id]; !ok {
				ids, info[id] = append(ids, id), of[id]
			}
		}
	}
	if p.First && again {
		return p.Under.find(c, sc, up, again)
	}
	x := maps.Clone(sc)
	if up != nil {
		x["up"] = up
	}
	items, err := p.items(c, x, again)
	if r := (*Refusal)(nil); errors.As(err, &r) && slices.Contains(p.Soft, r.Status) {
		items, err = nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	not := []string{}
	for _, n := range p.Not {
		ans, err := c.req(n, x)
		if r := (*Refusal)(nil); errors.As(err, &r) && r.Status == http.StatusNotFound {
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		not = append(not, text(pick(ans, "id")))
	}
	for _, it := range items {
		m, _ := it.(map[string]any)
		if m == nil || anyHolds(p.Skip, m) || slices.Contains(not, text(m["id"])) {
			continue
		}
		y := maps.Clone(x)
		maps.Copy(y, m)
		if p.Under != nil {
			next := maps.Clone(m)
			if up != nil {
				next = maps.Clone(up)
				maps.Copy(next, m)
			}
			more, of, err := p.Under.find(c, sc, next, again)
			if err != nil {
				return nil, nil, err
			}
			add(more, of)
		} else {
			id, _ := fillStr(cmp.Or(p.ID, "{id}"), y)
			if s := text(id); s != "" {
				vals, _ := fill(p.Vals, y)
				o, _ := vals.(map[string]any)
				add([]string{s}, map[string]Obj{s: o})
			}
		}
		if p.Deep != nil && p.DeepIf.holds(m) {
			q := *p
			q.Req, q.Deep, q.Not = *p.Deep, p.Deep, nil
			q.Also = nil
			z := maps.Clone(sc)
			z["id"] = m["id"]
			more, of, err := q.find(c, z, up, again)
			if err != nil {
				return nil, nil, err
			}
			add(more, of)
		}
	}
	for _, a := range p.Also {
		more, of, err := a.find(c, sc, up, again)
		if err != nil {
			return nil, nil, err
		}
		add(more, of)
	}
	return ids, info, nil
}

func (p *Parts) items(c *Client, sc map[string]any, again bool) ([]any, error) {
	switch {
	case p.Fixed != nil:
		out := make([]any, len(p.Fixed))
		for i, f := range p.Fixed {
			out[i] = f
		}
		return out, nil
	case p.Span != nil:
		return p.Span.dates(sc, again), nil
	case p.Req.URL == "" && p.Req.Tool == "":
		return nil, nil
	case p.Page != nil:
		return c.every(p.Req, p.Items, p.Page, sc)
	}
	ans, err := c.req(p.Req, sc)
	if err != nil {
		return nil, err
	}
	return listOf(pick(ans, p.Items)), nil
}

func (sp *Span) dates(sc map[string]any, again bool) []any {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	if sp.Month {
		today = time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
	last := sp.Last.from(today)
	from := sp.First.from(last)
	switch {
	case again && sp.Then != Shift{}:
		from = sp.Then.from(last)
	case sp.From != "":
		v, _ := fillStr(sp.From, sc)
		t := stamp(text(v))
		if t.IsZero() {
			t, _ = time.Parse(time.DateOnly, clip(text(v), 10))
		}
		if !t.IsZero() {
			from = sp.Back.from(t).Truncate(24 * time.Hour)
		}
	}
	out := []any{}
	for start := from; !start.After(last); start = sp.Step.from(start) {
		end := sp.Step.from(start).AddDate(0, 0, -1)
		if end.After(last) {
			end = last
		}
		out = append(out, map[string]any{"start": start.Format(time.DateOnly), "end": end.Format(time.DateOnly)})
	}
	return out
}

// read sends req for the page st is on, and is its answer and records.
func (s *Sync) read(c *Client, req Req, st *flow, sc map[string]any) (any, []Record, error) {
	ans, err := s.send(c, req, st, sc)
	if err != nil {
		return nil, nil, err
	}
	items, err := s.itemsOf(c, ans, s.Items, sc)
	if err != nil {
		return nil, nil, err
	}
	recs := s.records(items, sc)
	s.remember(st, recs, sc)
	return ans, recs, nil
}

// itemsOf is the list at at in ans. An answer without it is refused, so a listing never ends empty by accident and
// deletes what it did not read, unless the stream declares what an answer with nothing has in its place. A stream
// whose answer is its records, each with its ID, is refused any answer but a list.
func (s *Sync) itemsOf(c *Client, ans any, at Path, sc map[string]any) (any, error) {
	if t, _ := fillStr(string(at), sc); strings.Contains(string(at), "{") {
		at = Path(text(t))
	}
	items := pick(ans, at)
	if _, list := items.([]any); items != nil && (at != "" || s.ID == "" || list) {
		return items, nil
	}
	if s.Absent != "" && pick(ans, s.Absent) != nil || s.Empty && ans == nil {
		return []any{}, nil
	}
	return nil, fmt.Errorf("%s answered %s without its list of records, at %q", c.p.Name, s.Name, at)
}

func (s *Sync) send(c *Client, req Req, st *flow, sc map[string]any) (any, error) {
	if p, ok := s.Page.(ToolPage); ok {
		var err error
		if req, err = p.asking(c, req, st.Page); err != nil {
			return nil, err
		}
		return c.req(req, sc)
	}
	req = pageOf(req, s.Page, st.Page, sc)
	return c.req(req, sc)
}

// pageOf is req for the page at, which sc reads as {page}: "" is the first.
func pageOf(req Req, pg Paging, at string, sc map[string]any) Req {
	delete(sc, "page")
	if at != "" {
		sc["page"] = at
		if _, ok := pg.(NextURL); ok {
			req.URL, req.Query, req.exact = at, nil, true
		}
	} else if o, ok := pg.(Offset); ok && o.From != 0 {
		sc["page"] = strconv.Itoa(o.From)
	}
	return req
}

// every is the list at items in every page of req.
func (c *Client) every(req Req, items Path, pg Paging, sc map[string]any) ([]any, error) {
	x := maps.Clone(sc)
	out := []any{}
	for at := ""; ; {
		ans, err := c.req(pageOf(req, pg, at, x), x)
		if err != nil {
			return nil, err
		}
		got := listOf(pick(ans, items))
		out = append(out, got...)
		if at = nextOf(pg, ans, len(got), at); at == "" {
			return out, nil
		}
	}
}

func (s *Sync) records(items any, sc map[string]any) []Record {
	var out []Record
	for _, it := range listOf(items) {
		if m, ok := it.(map[string]any); ok {
			if r, ok := s.record(m, m, s.ID, nil, sc); ok {
				out = append(out, r)
			}
		}
	}
	return out
}

// record is what m says, its data at data; false when it is no record. id and gone are a Change's own when it has them.
func (s *Sync) record(m, data map[string]any, id Path, gone []Test, sc map[string]any) (Record, bool) {
	x := maps.Clone(sc)
	maps.Copy(x, m)
	rid := ""
	if len(s.Key) > 0 {
		for _, k := range s.Key {
			if rid = k.read(x); rid != "" {
				break
			}
		}
	} else {
		rid = text(pick(m, cmp.Or(id, "id")))
	}
	if anyHolds(gone, m) || anyHolds(s.Gone, m) {
		return Record{ID: rid, Deleted: true}, true
	}
	if anyHolds(s.Skip, m) {
		return Record{}, false
	}
	if data == nil {
		return Record{ID: rid, Deleted: true}, true
	}
	for k, t := range s.Set {
		if v, ok := fill(t, x); ok {
			data[k] = v
		}
	}
	return Record{ID: rid, Data: data}, true
}

// remember keeps what Stamp and Memo read of recs, and makes those Memo knows the same.
func (s *Sync) remember(st *flow, recs []Record, sc map[string]any) {
	if t, ok := s.Change.(SinceTime); ok && t.Stamp != "" {
		for _, r := range recs {
			st.Max = max(st.Max, text(pick(r.Data, t.Stamp)))
		}
	}
	if s.Memo == "" {
		return
	}
	if st.Next == nil {
		st.Next = map[string]string{}
	}
	for i, r := range recs {
		if r.Deleted {
			continue
		}
		x := maps.Clone(sc)
		maps.Copy(x, r.Data.(map[string]any))
		v, _ := fillStr(s.Memo, x)
		if m := text(v); m != "" {
			st.Next[r.ID] = m
			if st.Seen[r.ID] == m {
				recs[i] = Record{ID: r.ID, Same: true}
			}
		}
	}
}

// delta reads what changed, as its Change says, from the position part has.
func (s *Sync) delta(c *Client, st flow, part string, sc map[string]any) (*Page, error) {
	var ans any
	var recs []Record
	var err error
	expired := func(codes []int, stale ...Deny) bool {
		r := (*Refusal)(nil)
		return errors.As(err, &r) && (slices.Contains(codes, r.Status) || slices.ContainsFunc(stale, func(d Deny) bool { return d.is(r) }))
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
		next := s.next(c, ans, recs, &st)
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
		sc["since"] = mark.UTC().Format(time.RFC3339)
		switch {
		case ch.Ms:
			sc["since"] = strconv.FormatInt(mark.UnixMilli(), 10)
		case ch.Stamp != "":
			sc["since"] = st.Since[part]
		}
		from := s
		if ch.Items != "" {
			x := *s
			x.Items = ch.Items
			from = &x
		}
		if ans, recs, err = from.read(c, ch.Req, &st, sc); err != nil {
			if expired(ch.Expired) {
				return st.listing(), nil
			}
			return nil, err
		}
		if ch.Cap > 0 && len(recs) >= ch.Cap {
			return st.listing(), nil
		}
		next := s.next(c, ans, recs, &st)
		if ch.Mark != "" && len(recs) > 0 && next != "" {
			t, _ := fillStr(string(ch.Mark), sc)
			if last := stamp(text(pick(recs[len(recs)-1].Data, Path(text(t))))); last.After(mark) {
				st.Mark, st.Page = last.UTC().Format(time.RFC3339Nano), ""
				return s.page(c, st, "delta", sc, recs, true, false)
			}
		}
		position := st.Began
		if ch.Stamp != "" {
			position = max(st.Max, st.Since[part])
		}
		return s.done(c, st, part, sc, recs, next, position)
	case Token:
		sc["token"] = st.Since[part]
		if ans, err = s.send(c, ch.Req, &st, sc); err != nil {
			if expired(ch.Expired) {
				return st.listing(), nil
			}
			return nil, err
		}
		return s.done(c, st, part, sc, ch.changes(ans), s.next(c, ans, nil, &st), cmp.Or(text(pick(ans, ch.At)), st.Since[part]))
	case Resume:
		req := ch.Req
		sc["token"] = st.Since[part]
		if req.URL == "" {
			req = Req{URL: st.Since[part], Header: s.Req.Header, exact: true}
		}
		if ans, err = s.send(c, req, &st, sc); err != nil {
			if expired(ch.Expired, ch.Stale...) {
				return st.listing(), nil
			}
			return nil, err
		}
		items, err := s.itemsOf(c, ans, cmp.Or(ch.Items, s.Items), sc)
		if err != nil {
			return nil, err
		}
		for _, it := range listOf(items) {
			m, ok := it.(map[string]any)
			if !ok {
				continue
			}
			data := m
			if ch.Inner != "" {
				data, _ = pick(m, ch.Inner).(map[string]any)
			}
			if r, ok := s.record(m, data, cmp.Or(ch.ID, s.ID), ch.Gone, sc); ok {
				recs = append(recs, r)
			}
		}
		s.remember(&st, recs, sc)
		return s.done(c, st, part, sc, recs, s.next(c, ans, recs, &st), cmp.Or(text(pick(ans, ch.At)), st.Since[part]))
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
	mode, whole := st.Mode, st.Mode == "list" && !s.Keep && !st.Partial
	if next != "" {
		st.Page = next
		return s.page(c, st, mode, sc, recs, true, whole)
	}
	if mode == "list" {
		if st.Tokens == nil {
			st.Tokens = map[string]string{}
		}
		st.Tokens[part] = position
	} else {
		if st.Since == nil {
			st.Since = map[string]string{}
		}
		st.Since[part] = position
	}
	st.Page, st.Mark, st.First, st.At = "", "", "", st.At+1
	if st.At < len(st.Parts) {
		return s.page(c, st, mode, sc, recs, true, whole)
	}
	if mode == "list" {
		st.Listed, st.Since, st.Missed = st.Began, st.Tokens, nil
	}
	if s.Memo != "" {
		st.Seen = st.Next
	}
	return s.page(c, st.over(), mode, sc, recs, false, whole)
}

// page hydrates recs and is them as a page of the sync's mode, whole when it is of a listing of everything.
func (s *Sync) page(c *Client, st flow, mode string, sc map[string]any, recs []Record, more, whole bool) (*Page, error) {
	if err := s.hydrated(c, mode, recs, sc); err != nil {
		return nil, err
	}
	for i, r := range recs {
		if s.Updated != "" && !r.Deleted && !r.Same {
			recs[i].Updated = stamp(text(pick(r.Data, s.Updated)))
		}
	}
	return &Page{Records: recs, Cursor: st.cursor(), More: more, Full: whole}, nil
}

// next is the page after this one, recs in it.
func (s *Sync) next(c *Client, ans any, recs []Record, st *flow) string {
	p, ok := s.Page.(ToolPage)
	if !ok {
		return nextOf(s.Page, ans, len(recs), st.Page)
	}
	first := ""
	if len(recs) > 0 {
		first = recs[0].ID
	}
	next := p.after(c, s.Req.Tool, ans, recs, st.Page)
	if next == st.Page || first != "" && first == st.First {
		return ""
	}
	st.First = first
	return next
}

func nextOf(p Paging, ans any, n int, cur string) string {
	switch p := p.(type) {
	case NextURL:
		next := text(pick(ans, p.At))
		if len(p.Drop) > 0 && next != "" {
			if u, err := url.Parse(next); err == nil {
				q := u.Query()
				for _, k := range p.Drop {
					q.Del(k)
				}
				u.RawQuery = q.Encode()
				next = u.String()
			}
		}
		return next
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

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// firstTaken is the first of names, comma separated, that takes has.
func firstTaken(names string, has func(string) bool) string {
	for _, n := range strings.Split(names, ",") {
		if n != "" && has(n) {
			return n
		}
	}
	return ""
}

// asking is req with the page size and page in its arguments, as the tool's schema names them. A server without the
// tool skips the stream.
func (p ToolPage) asking(c *Client, req Req, at string) (Req, error) {
	t, err := c.tool(req.Tool)
	if err != nil {
		return req, err
	}
	props, _ := takes(t)
	has := func(k string) bool { return props == nil || props[k] }
	args, _ := req.Body.(map[string]any)
	args = maps.Clone(args)
	if args == nil {
		args = map[string]any{}
	}
	if l := firstTaken(p.Limit, has); l != "" {
		args[l] = p.Max
	}
	if at != "" {
		if a := firstTaken(p.After, has); a != "" {
			args[a] = at
		} else if n := firstTaken(p.Page, has); n != "" {
			args[n] = atoi(at)
		}
	}
	req.Body = args
	return req, nil
}

// after is the page after the one at: named in ans, or by number while a page comes back full.
func (p ToolPage) after(c *Client, tool string, ans any, recs []Record, at string) string {
	t, err := c.tool(tool)
	if err != nil {
		return ""
	}
	props, _ := takes(t)
	has := func(k string) bool { return props == nil || props[k] }
	if firstTaken(p.After, has) != "" {
		return namedNext(ans, recs)
	}
	if firstTaken(p.Page, has) != "" && len(recs) > 0 && len(recs) >= p.Max {
		return strconv.Itoa(max(atoi(at), 1) + 1)
	}
	return ""
}

// namedNext is the cursor of the page after recs in an answer: one it names at its top or a level down, like
// namedNext or pageInfo.endCursor while hasNextPage, or with has_more the last record's id; "" when it says there is
// no more.
func namedNext(v any, recs []Record) string {
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
			if more, _ := l[k].(bool); more && len(recs) > 0 {
				return recs[len(recs)-1].ID
			}
		}
	}
	return ""
}

// hydrated runs the stream's Hydrate over recs, those deleted or the same left alone.
func (s *Sync) hydrated(c *Client, mode string, recs []Record, base map[string]any) error {
	data := func(r *Record) map[string]any {
		m, _ := r.Data.(map[string]any)
		return m
	}
	scope := func(r *Record) map[string]any {
		x := maps.Clone(base)
		maps.Copy(x, data(r))
		if x["id"] == nil {
			x["id"] = r.ID
		}
		return x
	}
	skip := func(r *Record) bool { return r.Deleted || r.Same }
	for _, h := range s.Hydrate {
		switch h := h.(type) {
		case Fetch:
			var mu sync.Mutex
			once := map[string]any{}
			par := max(h.Par, 1)
			if h.Once != "" {
				par = 1
			}
			err := each(len(recs), par, func(i int) error {
				r := &recs[i]
				if skip(r) || h.If.At != "" && !h.If.holds(r.Data) {
					return nil
				}
				x := scope(r)
				key := ""
				if h.Once != "" {
					k, _ := fillStr(h.Once, x)
					key = text(k)
					mu.Lock()
					ans, ok := once[key]
					mu.Unlock()
					if ok {
						if h.As != "" {
							data(r)[h.As] = ans
						}
						return nil
					}
				}
				ans, err := h.read(c, x)
				if f := (*Refusal)(nil); h.Gone && errors.As(err, &f) && f.Status == http.StatusNotFound {
					r.Deleted = true
					return nil
				}
				if u := (*Unsupported)(nil); h.Bare && errors.As(err, &u) {
					return nil
				}
				if h.Soft && err != nil && !Busy(err) {
					ans, err = nil, nil
				}
				if err != nil {
					return err
				}
				if h.Items != "" {
					ans = pick(ans, h.Items)
				}
				if h.Once != "" {
					mu.Lock()
					once[key] = ans
					mu.Unlock()
				}
				switch m, isMap := ans.(map[string]any); {
				case h.Merge && isMap:
					maps.Copy(data(r), m)
				case h.As != "":
					data(r)[h.As] = ans
				case isMap:
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
				if !skip(&recs[i]) {
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
				if e, ok := got[recs[i].ID]; ok && !skip(&recs[i]) {
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
				if !skip(&recs[i]) {
					data(&recs[i])[h.As] = ans
				}
			}
		case Decode:
			for i := range recs {
				if skip(&recs[i]) || h.If.At != "" && !h.If.holds(recs[i].Data) {
					continue
				}
				if h.At != "" {
					v, err := h.Codec.fn(pick(recs[i].Data, h.At))
					if err != nil {
						return err
					}
					data(&recs[i])[h.As] = v
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
				if skip(r) {
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
		case Download:
			err := each(len(recs), max(h.Par, 1), func(i int) error {
				r := &recs[i]
				if skip(r) {
					return nil
				}
				x := scope(r)
				if h.Each == "" {
					if h.If.At != "" && !h.If.holds(r.Data) || !none(pick(r.Data, Path(h.As))) {
						return nil
					}
					t, err := c.words(h, x)
					if err != nil {
						return err
					}
					if t != "" {
						data(r)[h.As] = t
					}
					return nil
				}
				x["up"] = data(r)
				for _, f := range listOf(pick(r.Data, h.Each)) {
					a, _ := f.(map[string]any)
					if h.If.At != "" && !h.If.holds(a) {
						continue
					}
					y := maps.Clone(x)
					maps.Copy(y, a)
					t, err := c.words(h, y)
					if err != nil {
						return err
					}
					a[h.As] = t
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

// read is the answer of a Fetch: with More and Each, its first page with Each from all of them.
func (f Fetch) read(c *Client, sc map[string]any) (any, error) {
	if f.Each == "" {
		return c.req(f.Req, sc)
	}
	var first any
	all := []any{}
	for n := 1; n <= 100; n++ {
		sc["n"] = n
		ans, err := c.req(f.Req, sc)
		if err != nil {
			return nil, err
		}
		if first == nil {
			first = ans
		}
		got := listOf(pick(ans, f.Each))
		all = append(all, got...)
		if !truth(pick(ans, f.More)) || len(got) == 0 {
			break
		}
	}
	if m, ok := first.(map[string]any); ok {
		m[string(f.Each)] = all
	}
	return first, nil
}

// words are the words of the file d says, downloaded whole; "" for what it does not read.
func (c *Client) words(d Download, sc map[string]any) (string, error) {
	u, err := c.urlOf(d.Req.URL, sc)
	if err != nil {
		return "", err
	}
	if q := values(d.Req.Query, sc); len(q) > 0 {
		u = u + sepOf(u) + q.Encode()
	}
	name, _ := fillStr(d.Name, sc)
	kind, _ := fillStr(d.Mime, sc)
	size, _ := strconv.ParseInt(text(pick(sc, d.Size)), 10, 64)
	return c.fileText(u, text(name), text(kind), size)
}

func sepOf(u string) string {
	if strings.Contains(u, "?") {
		return "&"
	}
	return "?"
}

// scope is what a request of the account's own, not a script's, reads.
func (c *Client) scope() map[string]any {
	id, secret := c.Credentials()
	keys := map[string]any{}
	for k, v := range c.keys {
		keys[k] = v
	}
	meta := map[string]any{}
	for k, v := range c.Meta {
		meta[k] = v
	}
	opts := map[string]any{}
	for k := range c.p.opts {
		opts[k] = c.p.opts.Get(k)
	}
	return map[string]any{"client": map[string]any{"id": id, "secret": secret}, "token": c.Token(), "key": keys, "meta": meta, "option": opts}
}

// api is where the connector's relative URLs go: its API, read from the account, and what its Let finds, once.
func (c *Client) api() (string, error) {
	c.apiOnce.Do(func() {
		sc := c.scope()
		if _, c.apiErr = runSteps(c, c.p.Let, sc); c.apiErr == nil {
			c.apiURL, c.apiErr = expand(c.p.API, sc)
		}
	})
	return c.apiURL, c.apiErr
}

// req sends r, its templates read from sc, and is its answer, its numbers exact.
func (c *Client) req(r Req, sc map[string]any) (any, error) {
	if r.Tool != "" {
		return c.toolReq(r, sc)
	}
	v, err := c.http1(r, sc)
	if f := (*Refusal)(nil); r.Or != nil && errors.As(err, &f) && f.Status == http.StatusBadRequest {
		return c.req(*r.Or, sc)
	}
	return v, err
}

func (c *Client) http1(r Req, sc map[string]any) (any, error) {
	u := r.URL
	if !r.exact {
		var err error
		if u, err = c.urlOf(r.URL, sc); err != nil {
			return nil, err
		}
	} else if strings.HasPrefix(u, "/") {
		base, err := c.api()
		if err != nil {
			return nil, err
		}
		u = base + u
	}
	if q := values(r.Query, sc); len(q) > 0 {
		u += sepOf(u) + q.Encode()
	}
	var body any
	switch {
	case r.Form != nil:
		body = values(r.Form, sc)
	case r.Body != nil:
		body, _ = fill(r.Body, sc)
	}
	hdr := r.headers(sc)
	if r.Parts != nil {
		var b bytes.Buffer
		mw := multipart.NewWriter(&b)
		for _, p := range r.Parts {
			t, _ := fillStr(p.Type, sc)
			w, _ := mw.CreatePart(textproto.MIMEHeader{"Content-Type": {text(t)}})
			switch v, _ := fill(p.Body, sc); v := v.(type) {
			case string:
				io.WriteString(w, v)
			case nil:
			default:
				j, _ := json.Marshal(v)
				w.Write(j)
			}
		}
		mw.Close()
		body = b.String()
		if hdr == nil {
			hdr = http.Header{}
		}
		hdr.Set("Content-Type", "multipart/related; boundary="+mw.Boundary())
	}
	var raw []byte
	if err := c.Send(string(cmp.Or(r.Method, GET)), u, hdr, body, &raw); err != nil {
		return nil, err
	}
	if r.Text {
		return string(raw), nil
	}
	ans, err := readAll(raw, c.p.Name)
	if err != nil {
		return nil, err
	}
	if c.p.Answer != nil {
		if ans, err = c.p.Answer.fn(ans); err != nil {
			return nil, err
		}
	}
	if e := c.p.Errors; e != nil {
		if msg := text(pick(ans, e.At)); msg != "" {
			f := &Refusal{Provider: c.p.Name, Status: cmp.Or(e.Status, http.StatusBadRequest), Message: msg}
			if slices.Contains(e.Busy, text(pick(ans, e.Code))) {
				f.Status, f.Busy = http.StatusTooManyRequests, true
			}
			return nil, f
		}
	}
	return ans, nil
}

func (r Req) headers(sc map[string]any) http.Header {
	if r.Header == nil {
		return nil
	}
	hdr := http.Header{}
	for k, ts := range r.Header {
		for _, t := range ts {
			v, _ := fillStr(t, sc)
			hdr.Add(k, text(v))
		}
	}
	return hdr
}

// toolReq calls the MCP tool r names, with what of its arguments the tool takes.
func (c *Client) toolReq(r Req, sc map[string]any) (any, error) {
	t, err := c.tool(r.Tool)
	if err != nil {
		return nil, err
	}
	props, need := takes(t)
	has := func(k string) bool { return props == nil || props[k] }
	for _, k := range r.Need {
		if !has(k) {
			return nil, unsupported("%s's tool %s takes no %s", c.p.Name, r.Tool, k)
		}
	}
	v, _ := fill(r.Body, sc)
	args, _ := v.(map[string]any)
	if args == nil {
		args = map[string]any{}
	}
	maps.DeleteFunc(args, func(k string, _ any) bool { return !has(k) })
	for _, k := range need {
		if _, ok := args[k]; !ok {
			return nil, unsupported("%s's tool %s needs %s, which Archive has nothing to give", c.p.Name, r.Tool, k)
		}
	}
	res, err := c.Call(t.Name, args)
	if err != nil {
		return nil, err
	}
	if res.StructuredContent != nil {
		b, _ := json.Marshal(res.StructuredContent)
		return readAll(b, c.p.Name)
	}
	ans, err := readAll([]byte(Text(res)), c.p.Name)
	if err != nil {
		return nil, unsupported("%s's tool %s answers in prose, not JSON, which Archive cannot sync from", c.p.Name, r.Tool)
	}
	return ans, nil
}

// expand is template t as a URL: its values escaped as they ask, and it fails on one that has nothing.
func expand(t string, sc map[string]any) (string, error) {
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
		switch {
		case v.raw:
			return text(x)
		case v.segs:
			var parts []string
			for _, p := range strings.Split(text(x), "/") {
				if p != "" && p != "." && p != ".." {
					parts = append(parts, url.PathEscape(p))
				}
			}
			return strings.Join(parts, "/")
		}
		return url.PathEscape(text(x))
	})
	if missing != "" {
		return "", fmt.Errorf("it needs %s", missing)
	}
	return u, nil
}

// urlOf is a request's URL: an absolute one in a definition goes where it says, and a template that starts with a
// value, which a script may have given, stays on the API.
func (c *Client) urlOf(t string, sc map[string]any) (string, error) {
	u, err := expand(t, sc)
	if err != nil {
		return "", fmt.Errorf("%s: %w", c.p.Name, err)
	}
	u = strings.TrimSuffix(u, "?")
	if !strings.HasPrefix(u, "/") && !strings.HasPrefix(u, "?") && !strings.HasPrefix(t, "{") {
		return u, nil
	}
	base, err := c.api()
	if err != nil {
		return "", err
	}
	switch {
	case strings.HasPrefix(u, "/"), strings.HasPrefix(u, "?"):
		return base + u, nil
	case strings.HasPrefix(u, base+"/"), strings.HasPrefix(u, base+"?"):
		return u, nil
	}
	return "", fmt.Errorf("%s is neither a path, which starts with /, nor a URL of %s's API", u, c.p.Name)
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

var varRe = regexp.MustCompile(`\{[+~'"-]?[A-Za-z_][\w\-*/.@;]*(,[^|{}]*)?(\|[^{}]*)?\}`)

// tvar is a {path} of a template.
type tvar struct {
	path, def, sep        string
	raw, segs, join, host bool
	quote                 byte
	hasDef                bool
}

func varOf(m string) tvar {
	s := m[1 : len(m)-1]
	var v tvar
	switch s[0] {
	case '+':
		v.raw, s = true, s[1:]
	case '~':
		v.segs, s = true, s[1:]
	case '-':
		v.host, s = true, s[1:]
	case '\'', '"':
		v.quote, s = s[0], s[1:]
	}
	s, v.def, v.hasDef = strings.Cut(s, "|")
	v.path, v.sep, v.join = strings.Cut(s, ",")
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
		return strings.Join(s, cmp.Or(v.sep, ",")), true
	}
	if v.host {
		return strings.ReplaceAll(strings.ToLower(text(x)), "_", "-"), true
	}
	switch v.quote {
	case '\'':
		return strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(text(x)), true
	case '"':
		return strings.ReplaceAll(text(x), `"`, `\"`), true
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
	if strings.Contains(string(path), "+") {
		out := []any{}
		for _, part := range strings.Split(string(path), "+") {
			out = append(out, listOf(pick(v, Path(part)))...)
		}
		return out
	}
	if strings.Contains(string(path), ";") {
		for _, alt := range strings.Split(string(path), ";") {
			if x := pick(v, Path(alt)); !none(x) {
				return x
			}
		}
		return nil
	}
	head, rest, _ := strings.Cut(string(path), "/")
	switch head {
	case "*":
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
	case "**":
		return deep(v, rest)
	}
	if l, ok := v.([]any); ok {
		if i, err := strconv.Atoi(head); err == nil && i >= 0 && i < len(l) {
			return pick(l[i], Path(rest))
		}
		return nil
	}
	m, _ := v.(map[string]any)
	return pick(m[head], Path(rest))
}

// deep is the list under key at any depth, or with no key the first list of objects there is, v itself when it is one,
// or else the first empty list; nil when it has none.
func deep(v any, key string) any {
	var empty any
	var walk func(v any) any
	walk = func(v any) any {
		switch v := v.(type) {
		case []any:
			if key == "" {
				if len(v) == 0 && empty == nil {
					empty = v
				}
				if len(v) > 0 {
					if _, ok := v[0].(map[string]any); ok {
						return v
					}
				}
			}
			for _, e := range v {
				if x := walk(e); x != nil {
					return x
				}
			}
		case map[string]any:
			if a, ok := v[key].([]any); ok && key != "" {
				return a
			}
			for _, k := range slices.Sorted(maps.Keys(v)) {
				if x := walk(v[k]); x != nil {
					return x
				}
			}
		}
		return nil
	}
	if x := walk(v); x != nil {
		return x
	}
	return empty
}

// none says v is nothing for a template.
func none(v any) bool { return v == nil || v == "" || v == false }

// truth says v is something for a step or a Fetch to go on: a list is when any element is.
func truth(v any) bool {
	if l, ok := v.([]any); ok {
		return slices.ContainsFunc(l, truth)
	}
	return !none(v)
}

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
	return "SELECT " + rowSQL(m.Cols, m.People, m.Orgs, m.EINs, m.Refs, m.Children) + " FROM jsonb_array_elements($1::jsonb) WITH ORDINALITY AS r(data, i) ORDER BY i"
}

func rowSQL(cols, people, orgs, eins, refs map[string]SQL, kids []Kids) string {
	wrap := func(cols map[string]SQL, f string) string {
		var parts []string
		for _, k := range slices.Sorted(maps.Keys(cols)) {
			parts = append(parts, "'"+k+"', "+fmt.Sprintf(f, cols[k]))
		}
		return "jsonb_build_object(" + strings.Join(parts, ", ") + ")"
	}
	s := "jsonb_build_object('cols', " + wrap(cols, "(%s)") +
		", 'people', jsonb_strip_nulls(" + wrap(people, "archive.sync_email(%s)") + ")" +
		", 'orgs', jsonb_strip_nulls(" + wrap(orgs, "archive.sync_domain(%s)") + " || " + wrap(eins, "archive.sync_ein(%s)") + ")" +
		", 'refs', jsonb_strip_nulls(" + wrap(refs, "nullif(%s, '')") + ")"
	if len(kids) > 0 {
		var ks []string
		for _, k := range kids {
			ks = append(ks, "(SELECT coalesce(jsonb_agg("+rowSQL(k.Cols, k.People, k.Orgs, nil, k.Refs, nil)+" || jsonb_build_object('table', '"+k.Table+"')), '[]'::jsonb) FROM "+string(k.From)+")")
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
// a reply to parent, a message as JSON, into raw, base64url. Base64JSON reads JSON out of base64, and Base64Text text.
// Words is the words of HTML. GraphQLReads passes a GraphQL document that only reads, and refuses any other. Sum adds
// a list of numbers up.
// ToolBusy are what MCP tools say when the API behind them turned them away for now.
var ToolBusy = []string{"Too Many Requests", " 429", " 502", " 503", " 504", "Service Unavailable", "Bad Gateway", "Gateway Timeout", "Rate Limit Exceeded"}

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
	Base64Text = &Codec{"base64_text", func(v any) (any, error) {
		b, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(text(v), "\n", ""))
		if err != nil {
			return nil, fmt.Errorf("data that is not base64: %v", err)
		}
		return string(b), nil
	}}
	Words        = &Codec{"words", func(v any) (any, error) { return htmlText(text(v)), nil }}
	GraphQLReads = &Codec{"graphql_reads", graphQLReads}
	Sum          = &Codec{"sum", func(v any) (any, error) {
		n := 0
		for _, e := range listOf(v) {
			x, _ := strconv.Atoi(text(e))
			n += x
		}
		return n, nil
	}}
)

// graphQLReads passes a document whose operations are all queries, and refuses one that has anything else, wherever it
// is in the document: comments and strings are skipped, and only what is outside every brace is read as an operation.
func graphQLReads(v any) (any, error) {
	doc := text(v)
	depth := 0
	for i := 0; i < len(doc); {
		switch c := doc[i]; {
		case c == '#':
			for i < len(doc) && doc[i] != '\n' && doc[i] != '\r' {
				i++
			}
		case strings.HasPrefix(doc[i:], `"""`):
			end := strings.Index(doc[i+3:], `"""`)
			for end > 0 && doc[i+3+end-1] == '\\' {
				next := strings.Index(doc[i+3+end+3:], `"""`)
				if next < 0 {
					end = -1
					break
				}
				end += 3 + next
			}
			if end < 0 {
				return nil, errors.New("a block string that does not end")
			}
			i += 3 + end + 3
		case c == '"':
			i++
			for i < len(doc) && doc[i] != '"' {
				if doc[i] == '\\' {
					i++
				}
				i++
			}
			i++
		case c == '{':
			depth++
			i++
		case c == '}':
			depth--
			i++
		case depth == 0 && (c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'):
			j := i
			for j < len(doc) && (doc[j] == '_' || doc[j] >= 'a' && doc[j] <= 'z' || doc[j] >= 'A' && doc[j] <= 'Z' || doc[j] >= '0' && doc[j] <= '9') {
				j++
			}
			word := doc[i:j]
			i = j
			if word != "query" && word != "fragment" {
				return nil, fmt.Errorf("Archive only reads here, and runs no %s", word)
			}
			// What follows, a name, variables or directives, up to the opening brace, is the operation's own.
			paren := 0
			for i < len(doc) && (doc[i] != '{' || paren > 0) {
				switch doc[i] {
				case '(':
					paren++
				case ')':
					paren--
				case '"':
					i++
					for i < len(doc) && doc[i] != '"' {
						if doc[i] == '\\' {
							i++
						}
						i++
					}
				case '#':
					for i < len(doc) && doc[i] != '\n' && doc[i] != '\r' {
						i++
					}
					continue
				}
				i++
			}
		default:
			i++
		}
	}
	return v, nil
}

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
	if !l.OIDC && len(l.Signs) == 0 && l.Kept == nil {
		return errors.New("a push must prove itself: OIDC, Signs or Kept")
	}
	if len(l.Keys) == 0 && l.Kept == nil {
		return errors.New("needs Keys or Kept")
	}
	for _, s := range l.Signs {
		if s.Key == nil {
			continue
		}
		if len(c.Keys) == 0 || s.JWK == "" || s.Claim == "" {
			return errors.New("a JWS is a key connector's, with JWK and Claim")
		}
		if err := s.Key.check("jws key", "client", "option", "kid"); err != nil {
			return err
		}
	}
	if l.Key == "" && l.Mint == "" {
		return errors.New("needs Key or Mint")
	}
	if l.Watch.URL != "" {
		if err := l.Watch.check("watch", "client", "token", "key", "meta", "option", "target", "account", "conn", "ask", "nonce", "secret", "hook", "part", "in", "until", "until_ms", "up"); err != nil {
			return err
		}
	}
	p := &Push{Env: l.Env, OIDC: l.OIDC, Renew: l.Renew}
	p.Watch = l.watch(c)
	if u := l.Unwatch; u != nil {
		p.Unwatch = func(cl *Client, w Watch) error {
			_, err := cl.req(*u, watchScope(cl, w))
			return err
		}
	}
	p.Hear = l.hear
	c.Push = p
	return nil
}

func watchScope(cl *Client, w Watch) map[string]any {
	sc := cl.scope()
	sc["key"], sc["resource"], sc["secret"] = w.Key, w.Resource, w.Secret
	return sc
}

// watch is how the connector asks for pushes, as a Push does: over each part, keeping or replacing what there was.
func (l *Listen) watch(c *Connector) func(cl *Client, h Hook, old []Watch) ([]Watch, error) {
	return func(cl *Client, h Hook, old []Watch) ([]Watch, error) {
		base := cl.scope()
		base["target"], base["account"], base["conn"] = h.Target, h.Account, h.ID
		if l.Until > 0 {
			t := time.Now().Add(l.Until)
			base["until"], base["until_ms"] = t.UTC().Format(time.RFC3339), strconv.FormatInt(t.UnixMilli(), 10)
		}
		if l.Ask != nil {
			ans, err := cl.req(*l.Ask, base)
			if err != nil {
				return nil, err
			}
			base["ask"] = ans
		}
		parts, info := []string{""}, map[string]Obj{}
		if l.Over != nil {
			var err error
			if parts, info, err = l.Over.find(cl, base, nil, false); err != nil {
				return nil, err
			}
		}
		had := map[string]Watch{}
		for _, o := range old {
			k := o.Key
			if l.OfPart != nil {
				k = l.OfPart.read(map[string]any{"key": o.Key, "resource": o.Resource})
			}
			had[k] = o
		}
		var out []Watch
		for _, part := range parts {
			if o, ok := had[part]; ok && l.Extend != nil {
				delete(had, part)
				sc := maps.Clone(base)
				sc["part"], sc["key"], sc["resource"], sc["secret"] = part, o.Key, o.Resource, o.Secret
				ans, err := cl.req(*l.Extend, sc)
				if err == nil && (l.Alive.At == "" || l.Alive.holds(ans)) {
					if t := l.ends(ans); !t.IsZero() {
						o.Expires = t
					}
					out = append(out, o)
					continue
				}
				if r := (*Refusal)(nil); err != nil && l.Alive.At == "" && (!errors.As(err, &r) || r.Status != http.StatusNotFound) {
					return nil, err
				}
			}
			w, err := l.make(cl, c, h, base, part, info[part])
			if err != nil {
				r := (*Refusal)(nil)
				if errors.As(err, &r) && slices.Contains(l.Skip, r.Status) || l.Lax && !Busy(err) {
					continue
				}
				return out, err
			}
			out = append(out, w)
		}
		if l.OfPart != nil || l.Replace {
			for _, o := range had {
				if l.Unwatch != nil {
					cl.req(*l.Unwatch, watchScope(cl, o))
				}
			}
		}
		return out, nil
	}
}

func (l *Listen) make(cl *Client, c *Connector, h Hook, base map[string]any, part string, in Obj) (Watch, error) {
	sc := maps.Clone(base)
	if part != "" {
		sc["part"], sc["in"] = part, in
	}
	sc["nonce"], sc["secret"] = nonce(), nonce()
	key := ""
	if l.Mint != "" {
		k, _ := fillStr(l.Mint, sc)
		key = text(k)
		sc["key"] = key
	}
	sc["hook"] = ""
	if h.URL != nil {
		sc["hook"] = h.URL(key)
	}
	var ans any
	if l.Watch.URL != "" {
		var err error
		if ans, err = cl.req(l.Watch, sc); err != nil {
			return Watch{}, err
		}
	}
	sc["answer"] = ans
	if key == "" {
		k, _ := fillStr(l.Key, sc)
		key = text(k)
	}
	if l.Lower {
		key = strings.ToLower(key)
	}
	if key == "" || key == "0" {
		return Watch{}, fmt.Errorf("%s named nothing to watch", c.Name)
	}
	w := Watch{Key: key, Expires: l.ends(ans)}
	if l.Resource != "" {
		r, _ := fillStr(l.Resource, sc)
		w.Resource = text(r)
	}
	if l.Secret {
		w.Secret = text(sc["secret"])
	}
	return w, nil
}

// ends is when a watch its answer is of ends.
func (l *Listen) ends(ans any) time.Time {
	if l.Lasts > 0 {
		return time.Now().Add(l.Lasts)
	}
	return stamp(text(pick(ans, l.Expires)))
}

// hear reads a push, as a Push does.
func (l *Listen) hear(pr *Provider, r *http.Request, at string, body []byte, secret func(string) string) (*Heard, error) {
	q := r.URL.Query()
	for _, h := range l.Hello {
		if h.Query != "" {
			if v := q.Get(h.Query); v != "" {
				return &Heard{Status: http.StatusOK, Header: http.Header{"Content-Type": {"text/plain; charset=utf-8"}}, Body: v}, nil
			}
		}
		if h.Header != "" {
			if s := r.Header.Get(h.Header); s != "" {
				key := q.Get("k")
				if key == "" || secret(key) != "" {
					return nil, forged("a handshake for a webhook that has one")
				}
				return &Heard{Status: http.StatusOK, Header: http.Header{http.CanonicalHeaderKey(h.Header): {s}}, Keep: map[string]string{key: s}}, nil
			}
		}
	}
	kept := ""
	if k := l.Kept; k != nil {
		if k.IDHeader != "" {
			id := r.Header.Get(k.IDHeader)
			keep := secret(id)
			switch {
			case keep == "":
				return &Heard{}, nil
			case !same(keep, r.Header.Get(k.TokenHeader)):
				return nil, forged("channel %s's token", id)
			case k.Quiet != "" && r.Header.Get(k.Quiet) == k.QuietIs:
				return &Heard{}, nil
			}
			return &Heard{Keys: []string{id}}, nil
		}
		if k.ByQuery {
			if kept = secret(q.Get("k")); kept == "" {
				return &Heard{}, nil
			}
		}
	}
	if !l.OIDC && len(l.Signs) > 0 {
		if err := l.verify(pr, r, at, body, kept); err != nil {
			return nil, err
		}
	}
	v, err := readAll(body, "a push")
	if err != nil {
		return nil, fmt.Errorf("a push that is not JSON: %v", err)
	}
	if l.Only.At != "" && !l.Only.holds(v) {
		return &Heard{}, nil
	}
	if k := l.Kept; k != nil {
		if k.ByQuery {
			return &Heard{Keys: []string{q.Get("k")}}, nil
		}
		var keys []string
		for _, e := range listOf(pick(v, k.Each)) {
			id := text(pick(e, k.ID))
			switch keep := secret(id); {
			case keep == "":
			case !same(keep, text(pick(e, k.Token))):
				return nil, forged("subscription %s's clientState", id)
			default:
				keys = append(keys, id)
			}
		}
		return &Heard{Keys: keysOf(keys...)}, nil
	}
	if l.Codec != nil {
		if v, err = l.Codec.fn(pick(v, l.Wrap)); err != nil {
			return nil, err
		}
	}
	var keys []string
	for _, path := range l.Keys {
		for _, k := range listOf(pick(v, path)) {
			s := text(k)
			if l.Lower {
				s = strings.ToLower(s)
			}
			keys = append(keys, s)
		}
	}
	return &Heard{Keys: keysOf(keys...)}, nil
}

// verify checks a push against the first of Signs whose header it has. kept is the secret of the watch it names.
func (l *Listen) verify(p *Provider, r *http.Request, at string, body []byte, kept string) error {
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
		key := p.secret
		switch {
		case s.Hook:
			key = p.HookSecret()
		case s.Kept:
			key = kept
		}
		if key == "" && s.Key == nil {
			return forged("%s", s.Header)
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
			m := hmac.New(sha256.New, []byte(key))
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
		if s.Key != nil {
			claims, err := s.jws(p, r, got)
			if err != nil {
				return err
			}
			iat, _ := claims["iat"].(float64)
			if time.Since(time.Unix(int64(iat), 0)).Abs() > 5*time.Minute {
				return forged("%s's iat", s.Header)
			}
			got, _ = claims[s.Claim].(string)
		}
		if !same(got, s.Pre+want) {
			return forged("%s", s.Header)
		}
		return nil
	}
	return forged("no signature it knows")
}

// jwsKeys are the providers' keys that sign their pushes' JWSs, by provider and kid.
var jwsKeys sync.Map

// jws are the claims of the JWS token, once its signature is the provider's.
func (s Sign) jws(p *Provider, r *http.Request, token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, forged("%s is no JWS", s.Header)
	}
	var head struct{ Alg, Kid string }
	if b, err := base64.RawURLEncoding.DecodeString(parts[0]); err != nil || json.Unmarshal(b, &head) != nil || head.Alg != "ES256" || head.Kid == "" {
		return nil, forged("%s's header", s.Header)
	}
	k, ok := jwsKeys.Load(p.ID + " " + head.Kid)
	if !ok {
		c := p.openKeys(r.Context(), nil, nil)
		defer c.Close()
		sc := c.scope()
		sc["kid"] = head.Kid
		ans, err := c.req(*s.Key, sc)
		if err != nil {
			return nil, err
		}
		jwk, _ := pick(ans, s.JWK).(map[string]any)
		x, errX := base64.RawURLEncoding.DecodeString(text(jwk["x"]))
		y, errY := base64.RawURLEncoding.DecodeString(text(jwk["y"]))
		if errX != nil || errY != nil || text(jwk["crv"]) != "P-256" || len(x) != 32 || len(y) != 32 {
			return nil, fmt.Errorf("%s's key %s is no P-256 key", p.Name, head.Kid)
		}
		pub, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), append(append([]byte{4}, x...), y...))
		if err != nil {
			return nil, fmt.Errorf("%s's key %s: %w", p.Name, head.Kid, err)
		}
		k, _ = jwsKeys.LoadOrStore(p.ID+" "+head.Kid, pub)
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(sig) != 64 {
		return nil, forged("%s's signature", s.Header)
	}
	h := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if !ecdsa.Verify(k.(*ecdsa.PublicKey), h[:], new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])) {
		return nil, forged("%s", s.Header)
	}
	var claims map[string]any
	if b, err := base64.RawURLEncoding.DecodeString(parts[1]); err != nil || json.Unmarshal(b, &claims) != nil {
		return nil, forged("%s's claims", s.Header)
	}
	return claims, nil
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
