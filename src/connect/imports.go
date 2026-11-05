package connect

import (
	"archive/zip"
	"bufio"
	"bytes"
	"cmp"
	"compress/flate"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"os"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/jsonschema-go/jsonschema"
	"golang.org/x/net/html/charset"
)

// An import reads one export a service hands out, like Google Takeout, from a file of yours: each stream walks the
// export's members, a zip's entries or the file itself, and parses those its formats claim; what none claims is kept
// as a file. Nothing is copied twice: members are read in place, and a page resumes at the member and record it left.

const (
	importOnce  = 100 * 365 * 24 * time.Hour
	importPage  = 2000
	importBytes = 24 << 20
	importTime  = 75 * time.Second
	importNest  = 256 << 20
	importWhole = 512 << 20
	importBig   = 32 << 20
	importText  = 64 << 10
	importWords = 3 << 20
	importDef   = "1"
)

type importStream struct {
	name, doc string
	to        Map
}

var importStreams = []importStream{
	{"mail", "Every message of an mbox, like Gmail's in Google Takeout or Apple's iCloud Mail, with who it is from and to, its text cut at 64 KB, " +
		"Gmail's labels and thread, and each attachment's name, type, size and words up to 3 MB.",
		Map{Table: "mail_message",
			Cols: map[string]SQL{"thread": "data->>'thread'", "subject": "data->>'subject'", "snippet": "data->>'snippet'", "text": "data->>'text'",
				"sent_at": "archive.sync_at(data->>'sent')", "labels": "coalesce(data->'labels', '[]'::jsonb)"},
			Children: []Kids{
				{Table: "mail_party", From: "jsonb_array_elements(data->'parties') p",
					Cols: map[string]SQL{"role": "p->>'role'", "name": "p->>'name'"}, People: map[string]SQL{"person": "p->>'address'"}},
				{Table: "mail_attachment", From: "jsonb_array_elements(data->'attachments') f",
					Cols: map[string]SQL{"filename": "f->>'filename'", "mime": "f->>'mime'", "size": "archive.sync_num(f->>'size')", "text": "f->>'text'"}},
			}}},
	{"events", "Every event of an iCalendar file, like Google Calendar's in Takeout or iCloud Calendars, with who organizes it and who is invited.",
		Map{Table: "calendar_event",
			Cols: map[string]SQL{"calendar": "data->>'calendar'", "title": "data->>'title'", "description": "data->>'description'", "location": "data->>'location'",
				"starts_at": "archive.sync_at(data->>'starts')", "ends_at": "archive.sync_at(data->>'ends')", "all_day": "archive.sync_bool(data->'all_day')",
				"status": "data->>'status'", "recurrence": "data->>'recurrence'", "organizer_name": "data->>'organizer_name'",
				"url": "data->>'url'", "edited_at": "archive.sync_at(data->>'edited')"},
			People: map[string]SQL{"organizer": "data->>'organizer'"},
			Refs:   map[string]SQL{"series": "data->>'series'"},
			Children: []Kids{{Table: "calendar_attendee", From: "jsonb_array_elements(data->'attendees') a",
				Cols: map[string]SQL{"name": "a->>'name'", "role": "a->>'role'", "response": "a->>'response'"}, People: map[string]SQL{"person": "a->>'email'"}}}}},
	{"contacts", "Every contact of a vCard file, like Google Contacts or iCloud Contacts, and the friends of a Facebook export.",
		Map{Table: "contact",
			Cols: map[string]SQL{"book": "data->>'book'", "first_name": "data->>'first'", "last_name": "data->>'last'", "full_name": "data->>'full'",
				"emails": "coalesce(data->'emails', '[]'::jsonb)", "phones": "coalesce(data->'phones', '[]'::jsonb)", "company": "data->>'company'",
				"job_title": "data->>'title'", "birthday": "archive.sync_day(data->>'birthday')", "address": "data->>'address'", "note": "data->>'note'",
				"url": "data->>'url'", "made_at": "archive.sync_at(data->>'made')"},
			People: map[string]SQL{"person": "data#>>'{emails,0}'"}}},
	{"places", "Where you were: Google's Location History and Timeline, as points, visits and trips, and where each Google Photos picture was taken.",
		Map{Table: "location",
			Cols: map[string]SQL{"kind": "data->>'kind'", "at": "archive.sync_at(data->>'at')", "ends_at": "archive.sync_at(data->>'ends')",
				"lat": "archive.sync_num(data->>'lat')", "lng": "archive.sync_num(data->>'lng')", "accuracy": "archive.sync_num(data->>'accuracy')",
				"place": "data->>'place'", "address": "data->>'address'", "activity": "data->>'activity'"}}},
	{"history", "What you watched, searched, visited and listened to: YouTube's watch and search history, Chrome's history and Google's My Activity in Takeout, and Apple Music's plays.",
		Map{Table: "history",
			Cols: map[string]SQL{"app": "data->>'app'", "kind": "data->>'kind'", "at": "archive.sync_at(data->>'at')", "title": "data->>'title'",
				"url": "data->>'url'", "detail": "data->>'detail'"}}},
	{"chats", "Every message of a chat export: Facebook Messenger and Instagram, WhatsApp, an iMessage chat.db, and AI chats from ChatGPT, Claude, Grok and Gemini, by conversation.",
		Map{Table: "chat_message",
			Cols: map[string]SQL{"channel": "data->>'channel'", "kind": "'chat'", "thread": "data->>'thread'", "sender_name": "data->>'sender'",
				"text": "archive.sync_clip(data->>'text', 65536)", "files": "coalesce(data->'files', '[]'::jsonb)", "sent_at": "archive.sync_at(data->>'at')"},
			People: map[string]SQL{"sender": "data->>'address'"}}},
	{"posts", "What you posted on Facebook and Instagram, with its text, place, link and the names of its photos and videos.",
		Map{Table: "post",
			Cols: map[string]SQL{"app": "data->>'app'", "at": "archive.sync_at(data->>'at')", "text": "data->>'text'", "url": "data->>'url'",
				"place": "data->>'place'", "files": "coalesce(data->'files', '[]'::jsonb)"}}},
	{"people", "Every person of a GEDCOM family tree, like Ancestry's, FamilySearch's or MyHeritage's, with their births, deaths and who their parents, spouses and children are.",
		Map{Table: "kin",
			Cols: map[string]SQL{"given": "data->>'given'", "surname": "data->>'surname'", "sex": "data->>'sex'", "born": "data->>'born'", "born_in": "data->>'born_in'",
				"died": "data->>'died'", "died_in": "data->>'died_in'", "note": "data->>'note'"},
			Children: []Kids{{Table: "kin_tie", From: "jsonb_array_elements(data->'ties') t",
				Cols: map[string]SQL{"kind": "t->>'kind'"}, Refs: map[string]SQL{"relative": "t->>'relative'"}}}}},
	{"dna", "A summary of each raw DNA file, from 23andMe, AncestryDNA, MyHeritage or FamilyTreeDNA: which company, the genome build, and how many markers it has and called; the file itself is kept as is.",
		Map{Table: "dna",
			Cols: map[string]SQL{"app": "data->>'app'", "build": "data->>'build'", "markers": "archive.sync_num(data->>'markers')", "called": "archive.sync_num(data->>'called')",
				"chromosomes": "archive.sync_num(data->>'chromosomes')"},
			Refs: map[string]SQL{"file": "data->>'file_source'"}}},
	{"orders", "Every order of Amazon's privacy export, Your Orders, since the account began: its items with their ASIN, quantity, price and tax, " +
		"what it came to with shipping and discounts, what was refunded, its status, how it was paid and its carrier tracking, by website and order id.",
		Map{Table: "commerce_order",
			Cols: map[string]SQL{"number": "data->>'number'", "customer_name": "'Amazon'",
				"total": importMoney("total"), "shipping": importMoney("shipping"), "discounts": importMoney("discounts"), "refunded": importMoney("refunded"),
				"fulfilled": "nullif(data->>'status', '')", "state": "data->>'state'", "country": "data->>'country'", "placed_at": "archive.sync_at(data->>'placed')",
				"tags": "jsonb_build_array(data->>'website')",
				"note": "nullif(concat_ws('; ', nullif(data->>'payment', ''), (SELECT string_agg(t #>> '{}', ', ') FROM archive.sync_list(data->'tracking') t)), '')"},
			Children: []Kids{{Table: "commerce_line", From: "jsonb_array_elements(data->'items') l",
				Cols: map[string]SQL{"title": "l->>'title'", "sku": "nullif(l->>'asin', '')", "product_source": "nullif(l->>'asin', '')", "variant": "nullif(l->>'condition', '')",
					"quantity": "archive.sync_num(l->>'quantity')",
					"price":    "archive.sync_money(l->>'price', coalesce(data->>'currency', 'USD'))", "total": "archive.sync_money(l->>'total', coalesce(data->>'currency', 'USD'))"}}}}},
	{"media", "The books of a Goodreads export and the films of a Letterboxd one, with your shelf, rating, review and when you read or watched them.",
		Map{Table: "media",
			Cols: map[string]SQL{"app": "data->>'app'", "kind": "data->>'kind'", "title": "data->>'title'", "creator": "data->>'creator'", "year": "archive.sync_num(data->>'year')",
				"rating": "archive.sync_num(data->>'rating')", "review": "data->>'review'", "shelf": "data->>'shelf'", "done_on": "archive.sync_day(data->>'done')",
				"added_on": "archive.sync_day(data->>'added')", "isbn": "data->>'isbn'", "url": "data->>'url'"}}},
}

func init() {
	streams := []*Stream{{Name: "files", Table: "file", Every: importOnce, Version: importDef, Most: 64 << 30, Pull: importFiles, Read: importRead,
		Doc: "Every file of the export no stream reads, like Google Drive's and Photos', with its folders, and a raw DNA file too; " +
			"a photo's time from its Takeout sidecar. Their bytes are copied up to 64 GB each, read in place from the export.",
		SQL: Map{Table: "file",
			Cols: map[string]SQL{"kind": "data->>'kind'", "name": "data->>'name'", "mime": "data->>'mime'", "version": "data->>'version'",
				"source": "coalesce(data->'source', '{}')", "drive": "data->>'drive'", "made_at": "archive.sync_at(data->>'made')", "edited_at": "archive.sync_at(data->>'edited')"},
			Refs: map[string]SQL{"parent": "data->>'parent'"}}.sql()}}
	for _, s := range importStreams {
		streams = append(streams, &Stream{Name: s.name, Doc: s.doc, Table: s.to.Table, SQL: s.to.sql(), Every: importOnce, Version: importDef, Pull: importPull(s.name)})
	}
	in, err := jsonschema.For[struct{}](nil)
	if err != nil {
		panic(err)
	}
	out, err := jsonschema.For[importContents](nil)
	if err != nil {
		panic(err)
	}
	Register(&Connector{
		ID: "import", Name: "Import",
		Info: Info{By: "Archive", Category: CatFiles, Icon: IconDrive,
			Description: "Bring in an export a service gave you: Google Takeout, Apple's privacy export, Facebook or Instagram, ChatGPT, Claude or Grok, a GEDCOM family tree, raw DNA, Amazon's Your Orders, Goodreads, Letterboxd, a WhatsApp chat or an iMessage chat.db",
			Scopes:      []string{"Read the export, which moves under the import so only you see it, and goes when you disconnect it"}},
		Keys:    []Key{{Name: "file", Label: "Export", File: true, Help: "The zip or file a service gave you, of any size. Choose JSON over HTML where it asks. To import one already in Archive, like a Takeout zip in your Google Drive, choose Import in its menu"}},
		Account: importAccount,
		Actions: map[string]*Action{"contents": {
			Doc:   "What the export holds: each member, its size and the stream that reads it, or files when it is kept as a file; the first 2,000 members",
			Input: in, Output: out,
			Do: func(c *Client, _ json.RawMessage) (any, error) { return importList(c) }}},
		Streams: streams,
	})
}

func importAccount(c *Client) (string, error) {
	if c.Export == nil {
		return c.Meta["file"], nil
	}
	f, err := c.Export()
	if err != nil {
		return "", err
	}
	f.Body.Close()
	return f.Name, nil
}

// The export and its members

type importExport struct {
	name string
	size int64
	body io.ReadSeekCloser
	at   *importAt
	zip  *zip.Reader
	file string
}

type importAt struct {
	mu sync.Mutex
	r  io.ReadSeeker
}

func (a *importAt) ReadAt(p []byte, off int64) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, err := a.r.Seek(off, io.SeekStart); err != nil {
		return 0, err
	}
	n, err := io.ReadFull(a.r, p)
	if errors.Is(err, io.ErrUnexpectedEOF) {
		err = io.EOF
	}
	return n, err
}

func importOpen(c *Client) (*importExport, error) {
	if c.Export == nil {
		return nil, errors.New("this import has no export to read")
	}
	f, err := c.Export()
	if err != nil {
		return nil, err
	}
	x := &importExport{name: f.Name, size: f.Size, body: f.Body, at: &importAt{r: f.Body}, file: c.Meta["file"]}
	low := strings.ToLower(f.Name)
	if strings.HasSuffix(low, ".tgz") || strings.HasSuffix(low, ".tar.gz") || strings.HasSuffix(low, ".tar") {
		f.Body.Close()
		return nil, unsupported("%s is a tar archive, which Archive cannot read in place; export it as .zip", f.Name)
	}
	if z, err := zip.NewReader(x.at, f.Size); err == nil {
		x.zip = z
	} else if strings.HasSuffix(low, ".zip") {
		f.Body.Close()
		return nil, unsupported("%s does not read as a zip: %v", f.Name, err)
	}
	return x, nil
}

func (x *importExport) Close() error { return x.body.Close() }

// A member is the export itself, an entry of its zip or an entry of a zip in its zip.
type importMember struct {
	Path   string
	f      *zip.File
	base   int64
	nested bool
	whole  bool
	zip    *zip.Reader
}

func (m *importMember) size() int64 {
	if m.f == nil {
		return 0
	}
	return int64(m.f.UncompressedSize64)
}

func (m *importMember) baseName() string { return path.Base(m.Path) }

func (x *importExport) open(m *importMember) (io.ReadCloser, error) {
	if m.f == nil {
		return io.NopCloser(io.NewSectionReader(x.at, 0, x.size)), nil
	}
	return m.f.Open()
}

func (m *importMember) isZip() bool {
	return m.f != nil && !m.nested && strings.EqualFold(path.Ext(m.Path), ".zip")
}

// each walks the members from the i-th of the export, and in a zip in it from its j-th; j is -1 for the member itself.
func (x *importExport) each(i0, j0 int, fn func(i, j int, m *importMember) (bool, error)) error {
	if x.zip == nil {
		if i0 > 0 {
			return nil
		}
		_, err := fn(0, -1, &importMember{Path: x.name, whole: true})
		return err
	}
	for i := i0; i < len(x.zip.File); i++ {
		f := x.zip.File[i]
		if strings.HasSuffix(f.Name, "/") {
			continue
		}
		m := &importMember{Path: f.Name, f: f, zip: x.zip}
		j := -1
		if i == i0 {
			j = j0
		}
		if j < 0 {
			if more, err := fn(i, -1, m); err != nil || !more {
				return err
			}
			j = 0
		}
		if !m.isZip() {
			continue
		}
		inner, base, err := x.nest(m)
		if err != nil || inner == nil {
			continue
		}
		for ; j < len(inner.File); j++ {
			g := inner.File[j]
			if strings.HasSuffix(g.Name, "/") {
				continue
			}
			n := &importMember{Path: m.Path + "/" + g.Name, f: g, base: base, nested: true, zip: inner}
			if more, err := fn(i, j, n); err != nil || !more {
				return err
			}
		}
	}
	return nil
}

// nest opens a zip in the export: in place when it is stored, and in memory up to 256 MB when it is compressed.
// base is where it starts in the export, or -1 when it is in memory.
func (x *importExport) nest(m *importMember) (*zip.Reader, int64, error) {
	if m.f.Method == zip.Store {
		off, err := m.f.DataOffset()
		if err != nil {
			return nil, 0, err
		}
		z, err := zip.NewReader(io.NewSectionReader(x.at, off, int64(m.f.CompressedSize64)), int64(m.f.CompressedSize64))
		return z, off, err
	}
	if m.f.UncompressedSize64 > importNest {
		return nil, 0, nil
	}
	r, err := m.f.Open()
	if err != nil {
		return nil, 0, err
	}
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, 0, err
	}
	z, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	return z, -1, err
}

func (x *importExport) head(m *importMember) []byte {
	r, err := x.open(m)
	if err != nil {
		return nil
	}
	defer r.Close()
	b := make([]byte, 4096)
	n, _ := io.ReadFull(r, b)
	return b[:n]
}

// Formats, in the order they claim members

type importFormat struct {
	stream string
	name   string
	match  func(p, base string, head func() []byte) bool
	parse  func(x *importRun, m *importMember, r io.Reader, from int64) error
	seek   bool
}

var (
	importFBThread = regexp.MustCompile(`(^|/)messages/(inbox|archived_threads|filtered_threads|message_requests|e2ee_cutover)/[^/]+/message_\d+\.json$`)
	importPosts    = regexp.MustCompile(`^(your_posts.*|posts_\d+|your_posts__check_ins__photos_and_videos_\d+)\.json$`)
	importWALine   = regexp.MustCompile(`^\x{200e}?\[?(\d{1,4})[/.\-](\d{1,2})[/.\-](\d{1,4}),? (\d{1,2})[:.](\d{2})(?:[:.](\d{2}))?\s?([AaPp]\.?\s?[Mm]\.?)?\]?(?: -|:)? (.*)$`)
	importLetter   = []string{"diary.csv", "ratings.csv", "reviews.csv", "watched.csv", "watchlist.csv"}
)

var importFormats []*importFormat

func init() {
	ext := func(e ...string) func(string, string, func() []byte) bool {
		return func(p, base string, _ func() []byte) bool { return slices.Contains(e, strings.ToLower(path.Ext(base))) }
	}
	importFormats = []*importFormat{
		{stream: "mail", name: "mbox", seek: true, parse: importMbox,
			match: func(p, base string, _ func() []byte) bool {
				return strings.EqualFold(path.Ext(base), ".mbox") || base == "mbox"
			}},
		{stream: "events", name: "iCalendar", parse: importICS, match: ext(".ics", ".ical", ".ifb")},
		{stream: "contacts", name: "vCard", parse: importVCF, match: ext(".vcf", ".vcard")},
		{stream: "places", name: "Location History", seek: true, parse: importRecords,
			match: func(p, base string, _ func() []byte) bool {
				return (strings.Contains(p, "Location History") || strings.Contains(p, "Standortverlauf")) && (base == "Records.json" || base == "Location History.json")
			}},
		{stream: "places", name: "Semantic Location History", seek: true, parse: importSemantic,
			match: func(p, base string, _ func() []byte) bool {
				return strings.Contains(p, "Semantic Location History/") && strings.HasSuffix(base, ".json")
			}},
		{stream: "places", name: "Timeline", seek: true, parse: importTimeline,
			match: func(p, base string, _ func() []byte) bool {
				return base == "Timeline.json" || base == "location-history.json" || strings.HasPrefix(base, "Timeline") && strings.HasSuffix(base, ".json") && strings.Contains(p, "Timeline")
			}},
		{stream: "places", name: "Google Photos sidecar", parse: importPhotoPlace,
			match: func(p, base string, _ func() []byte) bool {
				return strings.Contains(p, "Google Photos/") && strings.HasSuffix(base, ".json")
			}},
		{stream: "history", name: "Chrome history", seek: true, parse: importChrome,
			match: func(p, base string, _ func() []byte) bool {
				return strings.Contains(p, "Chrome/") && (base == "History.json" || base == "BrowserHistory.json")
			}},
		{stream: "chats", name: "Gemini", seek: true, parse: importGemini,
			match: func(p, base string, _ func() []byte) bool {
				return (strings.Contains(p, "Gemini Apps/") || strings.Contains(p, "Bard/")) && strings.HasSuffix(base, ".json")
			}},
		{stream: "history", name: "Google activity", seek: true, parse: importActivity,
			match: func(p, base string, _ func() []byte) bool {
				return base == "watch-history.json" || base == "search-history.json" ||
					strings.Contains(p, "My Activity/") && (base == "MyActivity.json" || base == "My Activity.json")
			}},
		{stream: "chats", name: "Messenger or Instagram thread", parse: importThread,
			match: func(p, base string, _ func() []byte) bool { return importFBThread.MatchString(p) }},
		{stream: "posts", name: "Facebook or Instagram posts", seek: true, parse: importPostsOf,
			match: func(p, base string, _ func() []byte) bool {
				return importPosts.MatchString(base) && !strings.Contains(p, "Google")
			}},
		{stream: "contacts", name: "Facebook friends", parse: importFriends,
			match: func(p, base string, _ func() []byte) bool {
				return base == "your_friends.json" || base == "friends.json" && strings.Contains(p, "friends")
			}},
		{stream: "chats", name: "ChatGPT or Claude conversations", seek: true, parse: importConversations,
			match: func(p, base string, _ func() []byte) bool { return base == "conversations.json" }},
		{stream: "chats", name: "Grok", seek: true, parse: importGrok,
			match: func(p, base string, _ func() []byte) bool {
				return strings.HasSuffix(base, ".json") && strings.Contains(strings.ToLower(p), "grok")
			}},
		{stream: "people", name: "GEDCOM", parse: importGEDCOM, match: ext(".ged", ".gedcom")},
		{stream: "dna", name: "raw DNA", parse: importDNA,
			match: func(p, base string, head func() []byte) bool {
				e := strings.ToLower(path.Ext(base))
				if e != ".txt" && e != ".csv" && e != ".tsv" {
					return false
				}
				h := strings.ToLower(string(head()))
				return strings.Contains(h, "rsid") && (strings.Contains(h, "chromosome") || strings.Contains(h, "23andme") || strings.Contains(h, "ancestrydna"))
			}},
		{stream: "orders", name: "Amazon orders", parse: importAmazon, match: importAmazonMatch},
		{stream: "media", name: "Goodreads", parse: importGoodreads,
			match: func(p, base string, head func() []byte) bool {
				return strings.HasSuffix(base, ".csv") && bytes.HasPrefix(bytes.TrimPrefix(head(), []byte("\ufeff")), []byte("Book Id,Title,Author"))
			}},
		{stream: "media", name: "Letterboxd", parse: importLetterboxd,
			match: func(p, base string, head func() []byte) bool {
				return slices.Contains(importLetter, base) && bytes.Contains(head(), []byte("Letterboxd URI"))
			}},
		{stream: "history", name: "Apple Music plays", parse: importAppleMusic,
			match: func(p, base string, _ func() []byte) bool {
				return strings.HasSuffix(base, ".csv") && strings.Contains(base, "Play History Daily Tracks")
			}},
		{stream: "chats", name: "WhatsApp chat", parse: importWhatsApp,
			match: func(p, base string, head func() []byte) bool {
				if !strings.EqualFold(path.Ext(base), ".txt") {
					return false
				}
				if base == "_chat.txt" || strings.HasPrefix(base, "WhatsApp Chat") {
					return true
				}
				line, _, _ := strings.Cut(string(head()), "\n")
				return importWALine.MatchString(strings.TrimSpace(line))
			}},
		{stream: "chats", name: "iMessage", parse: importIMessage,
			match: func(p, base string, head func() []byte) bool {
				return (base == "chat.db" || strings.EqualFold(path.Ext(base), ".db") && strings.Contains(strings.ToLower(base), "chat")) &&
					bytes.HasPrefix(head(), []byte("SQLite format 3\x00"))
			}},
	}
}

func (x *importExport) format(m *importMember) *importFormat {
	if m.isZip() {
		return nil
	}
	var h []byte
	head := func() []byte {
		if h == nil {
			h = x.head(m)
		}
		return h
	}
	base := m.baseName()
	for _, f := range importFormats {
		if f.match(m.Path, base, head) {
			return f
		}
	}
	return nil
}

// A run is one page of a stream

type importCursor struct {
	I   int   `json:"i"`
	J   int   `json:"j"`
	N   int   `json:"n,omitempty"`
	Off int64 `json:"off,omitempty"`
}

type importRun struct {
	c      *Client
	x      *importExport
	start  time.Time
	page   *Page
	bytes  int
	at     importCursor
	skip   int
	seen   int
	stop   bool
	halt   bool
	member *importMember
}

func (r *importRun) full() bool {
	return r.stop || len(r.page.Records) >= importPage || r.bytes >= importBytes || time.Since(r.start) > importTime || r.c.ctx.Err() != nil
}

// put adds a record the member's parser read, which ends at byte at of the member; false asks it to stop at the next
// record it can resume from.
func (r *importRun) put(id string, data map[string]any, at int64) bool {
	r.seen++
	if r.seen <= r.skip {
		return true
	}
	if id == "" {
		return !r.full()
	}
	b, _ := json.Marshal(data)
	r.bytes += len(b)
	r.page.Records = append(r.page.Records, Record{ID: id, Data: json.RawMessage(b)})
	r.at.N, r.at.Off = r.seen, at
	if r.full() {
		r.stop = true
	}
	return !r.stop
}

func importPull(stream string) func(c *Client, cursor string) (*Page, error) {
	return func(c *Client, cursor string) (*Page, error) {
		cur := importCursor{J: -1}
		if err := cursorOf(cursor, &cur); err != nil {
			return nil, err
		}
		x, err := importOpen(c)
		if err != nil {
			return nil, err
		}
		defer x.Close()
		run := &importRun{c: c, x: x, start: time.Now(), page: &Page{Full: true}}
		err = x.each(cur.I, cur.J, func(i, j int, m *importMember) (bool, error) {
			f := x.format(m)
			if f == nil || f.stream != stream || f.parse == nil {
				return true, nil
			}
			if run.full() {
				run.at, run.halt = importCursor{I: i, J: j}, true
				return false, nil
			}
			run.at, run.seen, run.skip, run.member = importCursor{I: i, J: j}, 0, 0, m
			if i == cur.I && j == cur.J {
				run.at.N, run.at.Off = cur.N, cur.Off
				if f.seek {
					run.seen = cur.N
				} else {
					run.skip = cur.N
				}
			}
			r, err := x.open(m)
			if err != nil {
				return false, err
			}
			defer r.Close()
			from := int64(0)
			if f.seek && run.at.Off > 0 {
				if _, err := io.CopyN(io.Discard, r, run.at.Off); err != nil {
					return false, fmt.Errorf("%s: resuming at byte %d: %w", m.Path, run.at.Off, err)
				}
				from = run.at.Off
			}
			if err := f.parse(run, m, r, from); err != nil {
				if c.ctx.Err() != nil {
					return false, c.ctx.Err()
				}
				// A member that does not parse keeps what it gave and is passed over, so one bad file cannot stop the rest.
				fmt.Fprintf(os.Stderr, "archive: import %s: %s: %v\n", x.name, m.Path, err)
				run.stop = false
			}
			return !run.stop, nil
		})
		if err != nil {
			return nil, err
		}
		if run.stop || run.halt {
			run.page.More = true
			run.page.Cursor = cursorFor(run.at)
		}
		return run.page, nil
	}
}

// Files

func importFiles(c *Client, cursor string) (*Page, error) {
	cur := importCursor{J: -1}
	if err := cursorOf(cursor, &cur); err != nil {
		return nil, err
	}
	x, err := importOpen(c)
	if err != nil {
		return nil, err
	}
	defer x.Close()
	page := &Page{Full: true}
	if x.zip == nil {
		return page, nil
	}
	start, dirs := time.Now(), map[string]bool{}
	sidecars := importSidecars(x.zip)
	var last importCursor
	full := func() bool {
		return len(page.Records) >= importPage/4 || time.Since(start) > importTime || c.ctx.Err() != nil
	}
	folder := func(dir string) {
		for d := dir; d != "." && d != "" && !dirs[d]; d = path.Dir(d) {
			dirs[d] = true
			parent := path.Dir(d)
			rec := map[string]any{"kind": "folder", "name": strings.TrimSuffix(path.Base(d), ".zip"), "mime": "inode/directory", "drive": x.name}
			if parent != "." {
				rec["parent"] = parent + "/"
			}
			page.Records = append(page.Records, Record{ID: d + "/", Data: rec})
		}
	}
	stopped := false
	err = x.each(cur.I, cur.J, func(i, j int, m *importMember) (bool, error) {
		if full() {
			last, stopped = importCursor{I: i, J: j}, true
			return false, nil
		}
		if m.isZip() {
			if m.f.Method == zip.Store {
				return true, nil
			}
		} else if m.nested && m.base < 0 {
			return true, nil
		} else if f := x.format(m); f != nil && f.stream != "dna" {
			return true, nil
		}
		off, err := m.f.DataOffset()
		if err != nil {
			return true, nil
		}
		if m.nested {
			off += m.base
		}
		dir := path.Dir(m.Path)
		folder(dir)
		name := m.baseName()
		mt, _, _ := strings.Cut(mime.TypeByExtension(strings.ToLower(path.Ext(name))), ";")
		if mt == "" {
			mt = "application/octet-stream"
		}
		rec := map[string]any{"kind": "file", "name": name, "mime": mt, "drive": x.name,
			"version": fmt.Sprintf("%08x-%d", m.f.CRC32, m.f.UncompressedSize64),
			"source":  map[string]any{"path": m.Path, "off": off, "csize": m.f.CompressedSize64, "size": m.f.UncompressedSize64, "method": m.f.Method}}
		if dir != "." {
			rec["parent"] = dir + "/"
		}
		if t := m.f.Modified; !t.IsZero() {
			rec["edited"] = t.UTC().Format(time.RFC3339)
		}
		if !m.nested {
			if s := importSidecar(sidecars, m.Path); s != nil {
				if t := importSidecarTime(s); t != "" {
					rec["made"] = t
				}
			}
		}
		page.Records = append(page.Records, Record{ID: m.Path, Data: rec})
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	if stopped {
		page.More, page.Cursor = true, cursorFor(last)
	}
	return page, nil
}

func importRead(c *Client, source map[string]any) (*Bytes, error) {
	x, err := importOpen(c)
	if err != nil {
		return nil, err
	}
	num := func(k string) int64 { return int64(importNum(source[k])) }
	off, csize, size := num("off"), num("csize"), num("size")
	if off <= 0 || off+csize > x.size {
		x.Close()
		return nil, unsupported("the export no longer has %s where it was", text(source["path"]))
	}
	var r io.Reader = io.NewSectionReader(x.at, off, csize)
	switch num("method") {
	case int64(zip.Store):
	case int64(zip.Deflate):
		fr := flate.NewReader(r)
		return &Bytes{Body: importCloser{fr, func() error { fr.Close(); return x.Close() }}, Size: size}, nil
	default:
		x.Close()
		return nil, unsupported("%s is compressed in a way Archive does not read", text(source["path"]))
	}
	return &Bytes{Body: importCloser{r, x.Close}, Size: size}, nil
}

type importCloser struct {
	io.Reader
	close func() error
}

func (c importCloser) Close() error { return c.close() }

// importSidecars indexes the JSON files beside Google Photos pictures, by folder.
func importSidecars(z *zip.Reader) map[string][]*zip.File {
	out := map[string][]*zip.File{}
	for _, f := range z.File {
		if strings.Contains(f.Name, "Google Photos/") && strings.HasSuffix(f.Name, ".json") {
			out[path.Dir(f.Name)] = append(out[path.Dir(f.Name)], f)
		}
	}
	return out
}

// importSidecar finds a picture's sidecar: IMG_1.jpg.json, IMG_1.jpg.supplemental-metadata.json, or one Takeout cut
// to 51 characters, and an edited copy's original's.
func importSidecar(sidecars map[string][]*zip.File, p string) map[string]any {
	list := sidecars[path.Dir(p)]
	if len(list) == 0 || strings.HasSuffix(p, ".json") {
		return nil
	}
	base := path.Base(p)
	names := []string{base}
	if e := path.Ext(base); strings.HasSuffix(strings.TrimSuffix(base, e), "-edited") {
		names = append(names, strings.TrimSuffix(strings.TrimSuffix(base, e), "-edited")+e)
	}
	for _, name := range names {
		var best *zip.File
		for _, f := range list {
			s := strings.TrimSuffix(path.Base(f.Name), ".json")
			if len(s) >= min(len(name), 46) && strings.HasPrefix(name+".supplemental-metadata", s) && (best == nil || len(f.Name) > len(best.Name)) {
				best = f
			}
		}
		if best != nil {
			r, err := best.Open()
			if err != nil {
				return nil
			}
			var v map[string]any
			importDecoder(io.LimitReader(r, 1<<20)).Decode(&v)
			r.Close()
			return v
		}
	}
	return nil
}

// importPhotoPlace reads where a Google Photos picture was taken from its sidecar.
func importPhotoPlace(x *importRun, m *importMember, r io.Reader, _ int64) error {
	var v map[string]any
	if err := importDecoder(io.LimitReader(r, 1<<20)).Decode(&v); err != nil {
		return err
	}
	lat, lng := importNum(pick(v, "geoData/latitude")), importNum(pick(v, "geoData/longitude"))
	if lat == 0 && lng == 0 {
		lat, lng = importNum(pick(v, "geoDataExif/latitude")), importNum(pick(v, "geoDataExif/longitude"))
	}
	t := importSidecarTime(v)
	if lat == 0 && lng == 0 || t == "" {
		return nil
	}
	x.put(m.Path, map[string]any{"kind": "point", "at": t, "lat": lat, "lng": lng, "place": text(v["title"])}, 0)
	return nil
}

func importSidecarTime(v map[string]any) string {
	return importUnix(text(pick(v, "photoTakenTime/timestamp")))
}

func importUnix(s string) string {
	n, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || n <= 0 {
		return ""
	}
	if n > 1e11 {
		n /= 1000
	}
	sec, frac := math.Modf(n)
	return time.Unix(int64(sec), int64(frac*1e9)).UTC().Format(time.RFC3339Nano)
}

// The contents action

type importContents struct {
	Name    string           `json:"name"`
	Size    int64            `json:"size"`
	Count   int              `json:"count" jsonschema:"How many members it has"`
	Members []importListItem `json:"members"`
}

type importListItem struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	Format string `json:"format,omitempty" jsonschema:"What Archive reads it as, like mbox; empty when it is kept as a file"`
	Stream string `json:"stream" jsonschema:"The stream it lands in, like mail, or files"`
}

func importList(c *Client) (any, error) {
	x, err := importOpen(c)
	if err != nil {
		return nil, err
	}
	defer x.Close()
	out := importContents{Name: x.name, Size: x.size, Members: []importListItem{}}
	err = x.each(0, -1, func(i, j int, m *importMember) (bool, error) {
		out.Count++
		if len(out.Members) >= 2000 {
			return true, nil
		}
		it := importListItem{Path: m.Path, Size: cmp.Or(m.size(), x.size), Stream: "files"}
		if m.isZip() {
			it.Format, it.Stream = "zip", "its members"
		} else if f := x.format(m); f != nil {
			it.Format = f.name
			if f.stream != "" && f.stream != "dna" {
				it.Stream = f.stream
			} else if f.stream == "dna" {
				it.Stream = "dna and files"
			} else {
				it.Stream = "files, as its picture's time"
			}
		}
		out.Members = append(out.Members, it)
		return true, nil
	})
	return out, err
}

// Shared helpers

func importID(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(h[:12])
}

func importClip(s string) string { return clip(strings.TrimSpace(s), importText) }

// importArray reads the array at keys of a JSON document, or the document when keys are none, one element at a time,
// from the byte from where an element before ended; fn gets each and where it ends, and stops it by returning false.
func importArray(r io.Reader, from int64, keys []string, fn func(raw json.RawMessage, at int64) bool) error {
	var dec *json.Decoder
	base := from
	if from == 0 {
		dec = json.NewDecoder(r)
		if err := importFind(dec, keys); err != nil {
			return err
		}
	} else {
		br := bufio.NewReader(r)
		for {
			b, err := br.ReadByte()
			if err != nil {
				return err
			}
			base++
			if b == ']' {
				return nil
			}
			if b != ',' && b != ' ' && b != '\n' && b != '\r' && b != '\t' {
				br.UnreadByte()
				base--
				break
			}
		}
		dec = json.NewDecoder(io.MultiReader(strings.NewReader("["), br))
		base--
		if _, err := dec.Token(); err != nil {
			return err
		}
	}
	for dec.More() {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return err
		}
		if !fn(raw, base+dec.InputOffset()) {
			return nil
		}
	}
	return nil
}

func importFind(dec *json.Decoder, keys []string) error {
	for _, want := range keys {
		t, err := dec.Token()
		if err != nil {
			return err
		}
		if d, ok := t.(json.Delim); !ok || d != '{' {
			return fmt.Errorf("want an object holding %s", want)
		}
		for {
			t, err := dec.Token()
			if err != nil {
				return err
			}
			if d, ok := t.(json.Delim); ok && d == '}' {
				return fmt.Errorf("no %s", want)
			}
			if k, _ := t.(string); k == want {
				break
			}
			var skip json.RawMessage
			if err := dec.Decode(&skip); err != nil {
				return err
			}
		}
	}
	t, err := dec.Token()
	if err != nil {
		return err
	}
	if d, ok := t.(json.Delim); !ok || d != '[' {
		return errors.New("want an array")
	}
	return nil
}

func importObj(raw json.RawMessage) map[string]any {
	var v map[string]any
	importDecoder(bytes.NewReader(raw)).Decode(&v)
	return v
}

// importDecoder keeps numbers as written, as text reads them.
func importDecoder(r io.Reader) *json.Decoder {
	d := json.NewDecoder(r)
	d.UseNumber()
	return d
}

func importStr(v any, at Path) string { return strings.TrimSpace(text(pick(v, at))) }

func importWhen(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if t := stamp(s); !t.IsZero() {
		return t.UTC().Format(time.RFC3339Nano)
	}
	for _, layout := range []string{"2006-01-02T15:04:05Z0700", "2006-01-02 15:04:05 MST", "2006-01-02 15:04:05", "2006-01-02T15:04:05", "Jan 2, 2006, 3:04:05 PM MST"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC().Format(time.RFC3339Nano)
		}
	}
	return importUnix(s)
}

// Mail

func importMbox(x *importRun, m *importMember, r io.Reader, from int64) error {
	br := bufio.NewReaderSize(r, 1<<16)
	off := from
	var msg bytes.Buffer
	begun, lineStart, blank := false, true, true
	flush := func(end int64) bool {
		if !begun {
			return true
		}
		rec := importMail(msg.Bytes())
		msg.Reset()
		if rec == nil {
			return x.put("", nil, end)
		}
		id, _ := rec["id"].(string)
		delete(rec, "id")
		return x.put(id, rec, end)
	}
	for {
		line, err := br.ReadSlice('\n')
		if len(line) > 0 {
			if lineStart && blank && bytes.HasPrefix(line, []byte("From ")) {
				if !flush(off) {
					return nil
				}
				begun = true
			} else if begun && msg.Len() < importBig {
				if lineStart && bytes.HasPrefix(line, []byte(">From ")) {
					line = line[1:]
				}
				msg.Write(line)
			}
			off += int64(len(line))
			lineStart = line[len(line)-1] == '\n'
			if lineStart {
				blank = len(bytes.TrimRight(line, "\r\n")) == 0
			}
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		if err == io.EOF {
			flush(off)
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func importMail(raw []byte) map[string]any {
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return nil
	}
	h := msg.Header
	dec := &mime.WordDecoder{CharsetReader: charset.NewReaderLabel}
	word := func(s string) string {
		if d, err := dec.DecodeHeader(s); err == nil {
			return strings.TrimSpace(d)
		}
		return strings.TrimSpace(s)
	}
	id := strings.Trim(strings.TrimSpace(h.Get("Message-Id")), "<>")
	if id == "" {
		id = importID(string(raw[:min(len(raw), 64<<10)]))
	}
	rec := map[string]any{"id": id, "subject": word(h.Get("Subject"))}
	if t, err := h.Date(); err == nil {
		rec["sent"] = t.UTC().Format(time.RFC3339)
	}
	thread := strings.TrimSpace(h.Get("X-Gm-Thrid"))
	if thread == "" {
		if refs := strings.Fields(h.Get("References")); len(refs) > 0 {
			thread = strings.Trim(refs[0], "<>")
		} else if irt := strings.Trim(strings.TrimSpace(h.Get("In-Reply-To")), "<>"); irt != "" {
			thread = irt
		} else {
			thread = id
		}
	}
	rec["thread"] = thread
	if l := h.Get("X-Gmail-Labels"); l != "" {
		var labels []string
		for _, s := range strings.Split(word(l), ",") {
			if s = strings.TrimSpace(s); s != "" {
				labels = append(labels, s)
			}
		}
		rec["labels"] = labels
	}
	parties := []map[string]any{}
	for _, role := range []string{"from", "to", "cc", "bcc", "reply_to"} {
		v := h.Get(strings.ReplaceAll(role, "_", "-"))
		if v == "" {
			continue
		}
		addrs, err := (&mail.AddressParser{WordDecoder: dec}).ParseList(v)
		if err != nil {
			for _, part := range strings.Split(v, ",") {
				if a, err := (&mail.AddressParser{WordDecoder: dec}).Parse(part); err == nil {
					addrs = append(addrs, a)
				}
			}
		}
		for _, a := range addrs {
			parties = append(parties, map[string]any{"role": role, "name": a.Name, "address": strings.ToLower(a.Address)})
		}
	}
	rec["parties"] = parties
	parts := &importParts{files: []map[string]any{}}
	importWalk(h, msg.Body, parts, 0)
	body := parts.text
	if strings.TrimSpace(body) == "" && parts.html != "" {
		body = htmlText(parts.html)
	}
	rec["text"] = clip(body, importText)
	rec["snippet"] = clip(strings.Join(strings.Fields(body), " "), 200)
	rec["attachments"] = parts.files
	return rec
}

type importParts struct {
	text, html string
	files      []map[string]any
}

type importHeader interface{ Get(string) string }

func importWalk(h importHeader, body io.Reader, out *importParts, depth int) {
	ct := h.Get("Content-Type")
	mt, params, err := mime.ParseMediaType(ct)
	if err != nil || ct == "" {
		mt, params = "text/plain", map[string]string{}
	}
	if strings.HasPrefix(mt, "multipart/") && depth < 8 {
		mr := multipart.NewReader(body, params["boundary"])
		for {
			p, err := mr.NextRawPart()
			if err != nil {
				return
			}
			importWalk(p.Header, p, out, depth+1)
		}
	}
	switch strings.ToLower(h.Get("Content-Transfer-Encoding")) {
	case "base64":
		body = base64Lines(body)
	case "quoted-printable":
		body = quotedprintable.NewReader(body)
	}
	disp, dparams, _ := mime.ParseMediaType(h.Get("Content-Disposition"))
	name := cmp.Or(dparams["filename"], params["name"])
	if name != "" || disp == "attachment" || !strings.HasPrefix(mt, "text/") && mt != "message/rfc822" {
		b, _ := io.ReadAll(io.LimitReader(body, importWords+1))
		f := map[string]any{"filename": name, "mime": mt, "size": len(b)}
		if len(b) <= importWords && Extract != nil {
			if t := Extract(name, mt, b); t != "" {
				f["text"] = clip(t, importText)
			}
		}
		if n, _ := io.Copy(io.Discard, body); n > 0 {
			f["size"] = int64(len(b)) + n
		}
		out.files = append(out.files, f)
		return
	}
	if cs := params["charset"]; cs != "" && !strings.EqualFold(cs, "utf-8") && !strings.EqualFold(cs, "us-ascii") {
		if r, err := charset.NewReaderLabel(cs, body); err == nil {
			body = r
		}
	}
	b, _ := io.ReadAll(io.LimitReader(body, 4<<20))
	switch {
	case mt == "text/html" && out.html == "":
		out.html = string(b)
	case mt == "text/plain" && out.text == "":
		out.text = string(b)
	}
}

// base64Lines decodes base64 broken into lines, as mail sends it.
func base64Lines(r io.Reader) io.Reader { return base64.NewDecoder(base64.StdEncoding, r) }

// iCalendar

func importUnfold(r io.Reader) ([]string, error) {
	b, err := io.ReadAll(io.LimitReader(r, importWhole))
	if err != nil {
		return nil, err
	}
	s := strings.ReplaceAll(string(b), "\r\n", "\n")
	s = strings.ReplaceAll(s, "\n ", "")
	s = strings.ReplaceAll(s, "\n\t", "")
	return strings.Split(s, "\n"), nil
}

type importProp struct {
	name   string
	params map[string]string
	value  string
}

func importPropOf(line string) importProp {
	head, value, _ := strings.Cut(line, ":")
	if i := strings.Index(line, ":"); i >= 0 {
		// A parameter value may be quoted and hold a colon, like CN="Ann: Office".
		inq := false
		for j, c := range line {
			if c == '"' {
				inq = !inq
			}
			if c == ':' && !inq {
				head, value = line[:j], line[j+1:]
				break
			}
		}
	}
	parts := strings.Split(head, ";")
	p := importProp{name: strings.ToUpper(parts[0]), params: map[string]string{}, value: value}
	if i := strings.LastIndex(p.name, "."); i >= 0 {
		p.name = p.name[i+1:]
	}
	for _, kv := range parts[1:] {
		k, v, _ := strings.Cut(kv, "=")
		p.params[strings.ToUpper(k)] = strings.Trim(v, `"`)
	}
	return p
}

func importUnescape(s string) string {
	r := strings.NewReplacer(`\n`, "\n", `\N`, "\n", `\,`, ",", `\;`, ";", `\\`, `\`)
	return strings.TrimSpace(r.Replace(s))
}

func importICalTime(p importProp) (string, bool) {
	v := strings.TrimSpace(p.value)
	if p.params["VALUE"] == "DATE" || len(v) == 8 {
		if t, err := time.Parse("20060102", v); err == nil {
			return t.Format(time.RFC3339), true
		}
		return "", true
	}
	loc := time.UTC
	if tz := p.params["TZID"]; tz != "" {
		if l, err := time.LoadLocation(tz); err == nil {
			loc = l
		}
	}
	if strings.HasSuffix(v, "Z") {
		if t, err := time.Parse("20060102T150405Z", v); err == nil {
			return t.UTC().Format(time.RFC3339), false
		}
	}
	if t, err := time.ParseInLocation("20060102T150405", v, loc); err == nil {
		return t.UTC().Format(time.RFC3339), false
	}
	return "", false
}

func importICS(x *importRun, m *importMember, r io.Reader, _ int64) error {
	lines, err := importUnfold(r)
	if err != nil {
		return err
	}
	cal := strings.TrimSuffix(m.baseName(), path.Ext(m.baseName()))
	var ev map[string]any
	var uid, rid string
	var attendees []map[string]any
	for _, line := range lines {
		if line == "" {
			continue
		}
		p := importPropOf(line)
		switch {
		case p.name == "X-WR-CALNAME" && ev == nil:
			cal = importUnescape(p.value)
		case p.name == "BEGIN" && strings.EqualFold(p.value, "VEVENT"):
			ev, uid, rid, attendees = map[string]any{"calendar": cal}, "", "", []map[string]any{}
		case p.name == "END" && strings.EqualFold(p.value, "VEVENT") && ev != nil:
			ev["attendees"] = attendees
			id := uid
			if id == "" {
				id = importID(fmt.Sprint(ev["title"]), fmt.Sprint(ev["starts"]))
			}
			if rid != "" {
				ev["series"] = uid
				id += "@" + rid
			}
			if !x.put(id, ev, 0) {
				return nil
			}
			ev = nil
		case ev == nil:
		case p.name == "UID":
			uid = strings.TrimSpace(p.value)
		case p.name == "RECURRENCE-ID":
			rid = strings.TrimSpace(p.value)
		case p.name == "SUMMARY":
			ev["title"] = importUnescape(p.value)
		case p.name == "DESCRIPTION":
			ev["description"] = importClip(importUnescape(p.value))
		case p.name == "LOCATION":
			ev["location"] = importUnescape(p.value)
		case p.name == "URL":
			ev["url"] = strings.TrimSpace(p.value)
		case p.name == "STATUS":
			ev["status"] = strings.ToLower(strings.TrimSpace(p.value))
		case p.name == "RRULE":
			ev["recurrence"] = "RRULE:" + strings.TrimSpace(p.value)
		case p.name == "LAST-MODIFIED":
			ev["edited"], _ = importICalTime(p)
		case p.name == "DTSTART":
			t, day := importICalTime(p)
			ev["starts"], ev["all_day"] = t, day
		case p.name == "DTEND":
			ev["ends"], _ = importICalTime(p)
		case p.name == "ORGANIZER":
			ev["organizer"] = importMailto(p.value)
			ev["organizer_name"] = p.params["CN"]
		case p.name == "ATTENDEE":
			if e := importMailto(p.value); e != "" {
				role := map[string]string{"REQ-PARTICIPANT": "required", "OPT-PARTICIPANT": "optional", "NON-PARTICIPANT": "optional", "CHAIR": "required"}[p.params["ROLE"]]
				if p.params["CUTYPE"] == "ROOM" || p.params["CUTYPE"] == "RESOURCE" {
					role = "resource"
				}
				resp := map[string]string{"ACCEPTED": "accepted", "DECLINED": "declined", "TENTATIVE": "tentative", "NEEDS-ACTION": "none"}[p.params["PARTSTAT"]]
				attendees = append(attendees, map[string]any{"email": e, "name": p.params["CN"], "role": role, "response": resp})
			}
		}
	}
	return nil
}

func importMailto(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 7 && strings.EqualFold(s[:7], "mailto:") {
		s = s[7:]
	}
	if !strings.Contains(s, "@") {
		return ""
	}
	return strings.ToLower(s)
}

// vCard

func importVCF(x *importRun, m *importMember, r io.Reader, _ int64) error {
	lines, err := importUnfold(r)
	if err != nil {
		return err
	}
	book := "Contacts"
	switch {
	case strings.Contains(m.Path, "iCloud"):
		book = "iCloud Contacts"
	case strings.Contains(m.Path, "Takeout/") || strings.Contains(m.Path, "Google Contacts"):
		book = "Google Contacts"
	}
	var c map[string]any
	var uid string
	var emails, phones []string
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if line == "" {
			continue
		}
		p := importPropOf(line)
		if strings.EqualFold(p.params["ENCODING"], "QUOTED-PRINTABLE") {
			for strings.HasSuffix(p.value, "=") && i+1 < len(lines) {
				i++
				p.value = p.value[:len(p.value)-1] + lines[i]
			}
			if b, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(p.value))); err == nil {
				p.value = string(b)
			}
		}
		switch {
		case p.name == "BEGIN" && strings.EqualFold(p.value, "VCARD"):
			c, uid, emails, phones = map[string]any{"book": book}, "", []string{}, []string{}
		case p.name == "END" && strings.EqualFold(p.value, "VCARD") && c != nil:
			c["emails"], c["phones"] = emails, phones
			id := uid
			if id == "" {
				id = importID(fmt.Sprint(c["full"]), strings.Join(emails, ","), strings.Join(phones, ","))
			}
			if !x.put(id, c, 0) {
				return nil
			}
			c = nil
		case c == nil:
		case p.name == "UID":
			uid = strings.TrimSpace(p.value)
		case p.name == "FN":
			c["full"] = importUnescape(p.value)
		case p.name == "N":
			n := strings.Split(p.value, ";")
			c["last"] = importUnescape(n[0])
			if len(n) > 1 {
				c["first"] = importUnescape(n[1])
			}
		case p.name == "EMAIL":
			if e := strings.ToLower(strings.TrimSpace(p.value)); e != "" && !slices.Contains(emails, e) {
				emails = append(emails, e)
			}
		case p.name == "TEL":
			if t := strings.TrimSpace(strings.TrimPrefix(p.value, "tel:")); t != "" {
				phones = append(phones, t)
			}
		case p.name == "ORG":
			c["company"] = importUnescape(strings.Trim(strings.ReplaceAll(p.value, ";", " "), " "))
		case p.name == "TITLE":
			c["title"] = importUnescape(p.value)
		case p.name == "BDAY":
			c["birthday"] = importDay(p.value)
		case p.name == "ADR" && c["address"] == nil:
			var parts []string
			for _, s := range strings.Split(p.value, ";") {
				if s = importUnescape(s); s != "" {
					parts = append(parts, s)
				}
			}
			c["address"] = strings.Join(parts, ", ")
		case p.name == "NOTE":
			c["note"] = importClip(importUnescape(p.value))
		case p.name == "URL" && c["url"] == nil:
			c["url"] = strings.TrimSpace(p.value)
		}
	}
	return nil
}

func importDay(s string) string {
	s = strings.TrimSpace(s)
	if md := strings.ReplaceAll(strings.TrimPrefix(s, "--"), "-", ""); strings.HasPrefix(s, "--") && len(md) == 4 {
		s = "1604-" + md[:2] + "-" + md[2:]
	}
	for _, layout := range []string{"2006-01-02", "20060102", "2006/01/02", "2006-01-02T15:04:05Z"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("2006-01-02")
		}
	}
	if len(s) >= 10 {
		if t, err := time.Parse("2006-01-02", s[:10]); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return ""
}

// Places

func importE7(v any) (float64, bool) {
	f := importNum(v)
	return f / 1e7, f != 0
}

func importRecords(x *importRun, m *importMember, r io.Reader, from int64) error {
	return importArray(r, from, []string{"locations"}, func(raw json.RawMessage, at int64) bool {
		v := importObj(raw)
		lat, ok1 := importE7(v["latitudeE7"])
		lng, ok2 := importE7(v["longitudeE7"])
		t := importWhen(cmp.Or(text(v["timestamp"]), text(v["timestampMs"])))
		if !ok1 || !ok2 || t == "" {
			return x.put("", nil, at)
		}
		rec := map[string]any{"kind": "point", "at": t, "lat": lat, "lng": lng, "accuracy": v["accuracy"]}
		if a := importStr(v, "activity/0/activity/0/type"); a != "" {
			rec["activity"] = strings.ToLower(a)
		}
		return x.put(importID("p", t, text(v["latitudeE7"]), text(v["longitudeE7"])), rec, at)
	})
}

func importSemantic(x *importRun, m *importMember, r io.Reader, from int64) error {
	return importArray(r, from, []string{"timelineObjects"}, func(raw json.RawMessage, at int64) bool {
		v := importObj(raw)
		if pv, ok := v["placeVisit"].(map[string]any); ok {
			lat, _ := importE7(pick(pv, "location/latitudeE7"))
			lng, _ := importE7(pick(pv, "location/longitudeE7"))
			start := importWhen(cmp.Or(importStr(pv, "duration/startTimestamp"), importStr(pv, "duration/startTimestampMs")))
			rec := map[string]any{"kind": "visit", "at": start, "lat": lat, "lng": lng,
				"ends":  importWhen(cmp.Or(importStr(pv, "duration/endTimestamp"), importStr(pv, "duration/endTimestampMs"))),
				"place": importStr(pv, "location/name"), "address": importStr(pv, "location/address")}
			return x.put(importID("v", start, importStr(pv, "location/placeId")), rec, at)
		}
		if as, ok := v["activitySegment"].(map[string]any); ok {
			lat, _ := importE7(pick(as, "startLocation/latitudeE7"))
			lng, _ := importE7(pick(as, "startLocation/longitudeE7"))
			start := importWhen(cmp.Or(importStr(as, "duration/startTimestamp"), importStr(as, "duration/startTimestampMs")))
			rec := map[string]any{"kind": "move", "at": start, "lat": lat, "lng": lng,
				"ends":     importWhen(cmp.Or(importStr(as, "duration/endTimestamp"), importStr(as, "duration/endTimestampMs"))),
				"activity": strings.ToLower(importStr(as, "activityType"))}
			return x.put(importID("m", start), rec, at)
		}
		return x.put("", nil, at)
	})
}

var importLatLng = regexp.MustCompile(`(-?\d+(?:\.\d+)?)°?,\s*(-?\d+(?:\.\d+)?)`)

func importPoint(s string) (float64, float64, bool) {
	f := importLatLng.FindStringSubmatch(s)
	if f == nil {
		return 0, 0, false
	}
	lat, _ := strconv.ParseFloat(f[1], 64)
	lng, _ := strconv.ParseFloat(f[2], 64)
	return lat, lng, true
}

// importTimeline reads Google Maps' Timeline as phones export it since 2024: Android's Timeline.json, an object of
// semanticSegments, and iOS's location-history.json, an array of them.
func importTimeline(x *importRun, m *importMember, r io.Reader, from int64) error {
	var keys []string
	if m.baseName() != "location-history.json" {
		keys = []string{"semanticSegments"}
	}
	return importArray(r, from, keys, func(raw json.RawMessage, at int64) bool {
		v := importObj(raw)
		start, end := importWhen(text(v["startTime"])), importWhen(text(v["endTime"]))
		if start == "" {
			return x.put("", nil, at)
		}
		ok := true
		if vis, has := v["visit"].(map[string]any); has {
			loc := cmp.Or(importStr(vis, "topCandidate/placeLocation/latLng"), importStr(vis, "topCandidate/placeLocation"))
			lat, lng, _ := importPoint(loc)
			ok = x.put(importID("v", start, importStr(vis, "topCandidate/placeId")), map[string]any{"kind": "visit", "at": start, "ends": end, "lat": lat, "lng": lng,
				"place": strings.ToLower(importStr(vis, "topCandidate/semanticType"))}, at) && ok
		}
		if act, has := v["activity"].(map[string]any); has {
			loc := cmp.Or(importStr(act, "start/latLng"), importStr(act, "start"))
			lat, lng, _ := importPoint(loc)
			ok = x.put(importID("m", start), map[string]any{"kind": "move", "at": start, "ends": end, "lat": lat, "lng": lng,
				"activity": strings.ToLower(importStr(act, "topCandidate/type"))}, at) && ok
		}
		for _, p := range listOf(v["timelinePath"]) {
			lat, lng, has := importPoint(importStr(p, "point"))
			t := importWhen(importStr(p, "time"))
			if mins := importStr(p, "durationMinutesOffsetFromStartTime"); t == "" && mins != "" {
				if st, err := time.Parse(time.RFC3339Nano, start); err == nil {
					n, _ := strconv.Atoi(mins)
					t = st.Add(time.Duration(n) * time.Minute).Format(time.RFC3339)
				}
			}
			if has && t != "" {
				ok = x.put(importID("p", t, fmt.Sprint(lat, lng)), map[string]any{"kind": "point", "at": t, "lat": lat, "lng": lng}, at) && ok
			}
		}
		return ok
	})
}

// History

func importChrome(x *importRun, m *importMember, r io.Reader, from int64) error {
	return importArray(r, from, []string{"Browser History"}, func(raw json.RawMessage, at int64) bool {
		v := importObj(raw)
		t := importUnix(fmt.Sprint(importNum(v["time_usec"]) / 1e6))
		u := text(v["url"])
		if t == "" || u == "" {
			return x.put("", nil, at)
		}
		return x.put(importID("chrome", text(v["time_usec"]), u), map[string]any{"app": "Chrome", "kind": "visit", "at": t, "title": text(v["title"]), "url": u,
			"detail": strings.ToLower(text(v["page_transition"]))}, at)
	})
}

func importNum(v any) float64 {
	if f, ok := v.(float64); ok {
		return f
	}
	f, _ := strconv.ParseFloat(text(v), 64)
	return f
}

var importVerbs = []struct{ prefix, kind string }{{"Watched ", "watch"}, {"Searched for ", "search"}, {"Visited ", "visit"}, {"Listened to ", "listen"},
	{"Viewed ", "visit"}, {"Used ", "activity"}}

func importActivity(x *importRun, m *importMember, r io.Reader, from int64) error {
	return importArray(r, from, nil, func(raw json.RawMessage, at int64) bool {
		v := importObj(raw)
		t := importWhen(text(v["time"]))
		title := strings.TrimSpace(text(v["title"]))
		if t == "" || title == "" {
			return x.put("", nil, at)
		}
		kind := "activity"
		for _, vb := range importVerbs {
			if strings.HasPrefix(title, vb.prefix) {
				kind, title = vb.kind, strings.TrimPrefix(title, vb.prefix)
				break
			}
		}
		app := cmp.Or(text(v["header"]), path.Base(path.Dir(m.Path)))
		return x.put(importID("act", app, t, title), map[string]any{"app": app, "kind": kind, "at": t, "title": title,
			"url": text(v["titleUrl"]), "detail": importStr(v, "subtitles/0/name")}, at)
	})
}

func importAppleMusic(x *importRun, m *importMember, r io.Reader, _ int64) error {
	return importCSV(r, func(row map[string]string) bool {
		day, track := row["Date Played"], row["Track Description"]
		if track == "" || len(day) != 8 {
			return x.put("", nil, 0)
		}
		t := day[:4] + "-" + day[4:6] + "-" + day[6:] + "T00:00:00Z"
		return x.put(importID("music", day, track), map[string]any{"app": "Apple Music", "kind": "listen", "at": t, "title": track,
			"detail": strings.TrimSpace(row["Play Count"] + " plays")}, 0)
	})
}

// Chats

// importFix undoes Facebook's and Instagram's exports, which write each byte of UTF-8 as a character of its own.
func importFix(s string) string {
	b := make([]byte, 0, len(s))
	for _, r := range s {
		if r > 255 {
			return s
		}
		b = append(b, byte(r))
	}
	if utf8.Valid(b) {
		return string(b)
	}
	return s
}

func importApp(p string) string {
	if strings.Contains(strings.ToLower(p), "instagram") {
		return "Instagram"
	}
	return "Facebook"
}

func importThread(x *importRun, m *importMember, r io.Reader, _ int64) error {
	var v struct {
		Title        string `json:"title"`
		ThreadPath   string `json:"thread_path"`
		Participants []struct {
			Name string `json:"name"`
		} `json:"participants"`
		Messages []map[string]any `json:"messages"`
	}
	if err := importDecoder(io.LimitReader(r, importWhole)).Decode(&v); err != nil {
		return err
	}
	thread := cmp.Or(v.ThreadPath, path.Dir(m.Path))
	channel := importFix(v.Title)
	if channel == "" {
		var names []string
		for _, p := range v.Participants {
			names = append(names, importFix(p.Name))
		}
		channel = strings.Join(names, ", ")
	}
	for _, msg := range v.Messages {
		var files []string
		for _, k := range []string{"photos", "videos", "files", "audio_files", "gifs"} {
			for _, f := range listOf(msg[k]) {
				if u := importStr(f, "uri"); u != "" {
					files = append(files, path.Base(u))
				}
			}
		}
		txt := importFix(text(msg["content"]))
		if l := importStr(msg, "share/link"); l != "" {
			txt = strings.TrimSpace(txt + "\n" + l)
		}
		ts := text(msg["timestamp_ms"])
		sender := importFix(text(msg["sender_name"]))
		if !x.put(importID(thread, ts, sender, txt), map[string]any{"channel": channel, "thread": thread, "sender": sender, "text": txt,
			"files": files, "at": importUnix(ts)}, 0) {
			return nil
		}
	}
	return nil
}

func importConversations(x *importRun, m *importMember, r io.Reader, from int64) error {
	return importArray(r, from, nil, func(raw json.RawMessage, at int64) bool {
		v := importObj(raw)
		ok := true
		if mapping, has := v["mapping"].(map[string]any); has {
			conv := cmp.Or(text(v["conversation_id"]), text(v["id"]))
			title := text(v["title"])
			for id, node := range mapping {
				msg, _ := pick(node, "message").(map[string]any)
				role := importStr(msg, "author/role")
				if msg == nil || role != "user" && role != "assistant" {
					continue
				}
				var parts []string
				for _, p := range listOf(pick(msg, "content/parts")) {
					if s, isText := p.(string); isText && strings.TrimSpace(s) != "" {
						parts = append(parts, s)
					}
				}
				if t := importStr(msg, "content/text"); t != "" && len(parts) == 0 {
					parts = append(parts, t)
				}
				if len(parts) == 0 {
					continue
				}
				who := map[string]string{"user": "You", "assistant": "ChatGPT"}[role]
				ok = x.put(cmp.Or(text(msg["id"]), id), map[string]any{"channel": cmp.Or(title, "ChatGPT"), "thread": conv, "sender": who,
					"text": strings.Join(parts, "\n\n"), "at": importUnix(text(msg["create_time"]))}, at) && ok
			}
			return ok
		}
		if msgs, has := v["chat_messages"]; has {
			conv, title := text(v["uuid"]), text(v["name"])
			for _, msg := range listOf(msgs) {
				body := importStr(msg, "text")
				if body == "" {
					var parts []string
					for _, c := range listOf(pick(msg, "content")) {
						if importStr(c, "type") == "text" {
							parts = append(parts, importStr(c, "text"))
						}
					}
					body = strings.Join(parts, "\n\n")
				}
				var files []string
				for _, k := range []string{"attachments", "files"} {
					for _, f := range listOf(pick(msg, Path(k))) {
						if n := importStr(f, "file_name"); n != "" {
							files = append(files, n)
						}
					}
				}
				if body == "" && len(files) == 0 {
					continue
				}
				who := map[string]string{"human": "You", "assistant": "Claude"}[importStr(msg, "sender")]
				ok = x.put(importStr(msg, "uuid"), map[string]any{"channel": cmp.Or(title, "Claude"), "thread": conv, "sender": who, "text": body,
					"files": files, "at": importWhen(importStr(msg, "created_at"))}, at) && ok
			}
			return ok
		}
		return x.put("", nil, at)
	})
}

// importGrok reads grok.com's export, an object of conversations each with its responses.
func importGrok(x *importRun, m *importMember, r io.Reader, from int64) error {
	return importArray(r, from, []string{"conversations"}, func(raw json.RawMessage, at int64) bool {
		v := importObj(raw)
		conv := cmp.Or(importStr(v, "conversation/id"), importStr(v, "conversation/conversationId"))
		title := cmp.Or(importStr(v, "conversation/title"), "Grok")
		ok := true
		for _, resp := range listOf(v["responses"]) {
			rsp, _ := pick(resp, "response").(map[string]any)
			if rsp == nil {
				rsp, _ = resp.(map[string]any)
			}
			body := importStr(rsp, "message")
			if body == "" {
				continue
			}
			who := "Grok"
			if s := strings.ToLower(importStr(rsp, "sender")); s == "human" || s == "user" {
				who = "You"
			}
			t := importWhen(cmp.Or(importStr(rsp, "create_time/$date/$numberLong"), importStr(rsp, "create_time/$date"), importStr(rsp, "create_time")))
			id := cmp.Or(importStr(rsp, "_id"), importStr(rsp, "responseId"), importID(conv, t, body))
			ok = x.put(id, map[string]any{"channel": title, "thread": conv, "sender": who, "text": body, "at": t}, at) && ok
		}
		return ok
	})
}

// importGemini reads Gemini's activity in Takeout: each prompt with Gemini's answer, as two messages of the day's thread.
func importGemini(x *importRun, m *importMember, r io.Reader, from int64) error {
	return importArray(r, from, nil, func(raw json.RawMessage, at int64) bool {
		v := importObj(raw)
		t := importWhen(text(v["time"]))
		prompt := strings.TrimSpace(strings.TrimPrefix(text(v["title"]), "Prompted "))
		if t == "" || prompt == "" {
			return x.put("", nil, at)
		}
		thread := "gemini:" + t[:10]
		ok := x.put(importID("gemini", t, "you"), map[string]any{"channel": "Gemini", "thread": thread, "sender": "You", "text": prompt, "at": t}, at)
		var answer []string
		for _, h := range listOf(v["safeHtmlItem"]) {
			if s := htmlText(importStr(h, "html")); s != "" {
				answer = append(answer, s)
			}
		}
		if len(answer) > 0 {
			ok = x.put(importID("gemini", t, "gemini"), map[string]any{"channel": "Gemini", "thread": thread, "sender": "Gemini", "text": strings.Join(answer, "\n\n"), "at": t}, at) && ok
		}
		return ok
	})
}

// importWhatsApp reads a chat WhatsApp exported as text, whose dates are day first unless they cannot be.
func importWhatsApp(x *importRun, m *importMember, r io.Reader, _ int64) error {
	lines, err := importUnfold(r)
	if err != nil {
		return err
	}
	channel := strings.TrimSuffix(m.baseName(), ".txt")
	if channel == "_chat" || channel == "" {
		channel = path.Base(path.Dir(m.Path))
		if channel == "." {
			channel = strings.TrimSuffix(x.x.name, path.Ext(x.x.name))
		}
	}
	for _, p := range []string{"WhatsApp Chat with ", "WhatsApp Chat - ", "WhatsApp Chat"} {
		channel = strings.TrimPrefix(channel, p)
	}
	channel = strings.TrimSuffix(strings.TrimSpace(channel), ".zip")
	if channel == "" || channel == "." {
		channel = "WhatsApp"
	}
	dayFirst, yearFirst := true, false
	for _, l := range lines {
		f := importWALine.FindStringSubmatch(strings.TrimSpace(l))
		if f == nil {
			continue
		}
		a, _ := strconv.Atoi(f[1])
		b, _ := strconv.Atoi(f[2])
		if len(f[1]) == 4 {
			yearFirst = true
			break
		}
		if a > 12 {
			break
		}
		if b > 12 {
			dayFirst = false
			break
		}
	}
	type wa struct {
		at, sender, text string
		files            []string
	}
	var cur *wa
	n := 0
	emit := func() bool {
		if cur == nil {
			return true
		}
		n++
		return x.put(fmt.Sprintf("%s:%d", m.Path, n), map[string]any{"channel": channel, "thread": channel, "sender": cur.sender, "text": importClip(cur.text),
			"files": cur.files, "at": cur.at}, 0)
	}
	for _, l := range lines {
		l = strings.TrimRight(l, "\r")
		f := importWALine.FindStringSubmatch(strings.TrimSpace(l))
		if f == nil {
			if cur != nil {
				cur.text += "\n" + l
			}
			continue
		}
		if !emit() {
			return nil
		}
		d, mo, y := f[1], f[2], f[3]
		if yearFirst {
			y, mo, d = f[1], f[2], f[3]
		} else if !dayFirst {
			d, mo = f[2], f[1]
		}
		yy, _ := strconv.Atoi(y)
		if yy < 100 {
			yy += 2000
		}
		mm, _ := strconv.Atoi(mo)
		dd, _ := strconv.Atoi(d)
		hh, _ := strconv.Atoi(f[4])
		mi, _ := strconv.Atoi(f[5])
		ss, _ := strconv.Atoi(f[6])
		if ap := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(f[7], ".", ""), " ", "")); ap == "pm" && hh < 12 {
			hh += 12
		} else if ap == "am" && hh == 12 {
			hh = 0
		}
		at := time.Date(yy, time.Month(mm), dd, hh, mi, ss, 0, time.UTC).Format(time.RFC3339)
		sender, body, found := strings.Cut(f[8], ": ")
		if !found {
			sender, body = "", f[8]
		}
		cur = &wa{at: at, sender: strings.Trim(sender, "‎‪‬ "), text: strings.TrimLeft(body, "‎")}
		if i := strings.Index(cur.text, "<attached: "); i >= 0 {
			name := strings.TrimSuffix(strings.TrimSpace(cur.text[i+len("<attached: "):]), ">")
			cur.files, cur.text = []string{name}, strings.TrimSpace(cur.text[:i])
		} else if strings.HasSuffix(cur.text, " (file attached)") {
			name := strings.TrimSuffix(cur.text, " (file attached)")
			cur.files, cur.text = []string{name}, ""
		}
	}
	emit()
	return nil
}

// Posts and friends

func importPostsOf(x *importRun, m *importMember, r io.Reader, from int64) error {
	app := importApp(m.Path)
	return importArray(r, from, nil, func(raw json.RawMessage, at int64) bool {
		v := importObj(raw)
		rec := map[string]any{"app": app}
		var texts, files []string
		for _, d := range listOf(v["data"]) {
			if s := importFix(importStr(d, "post")); s != "" {
				texts = append(texts, s)
			}
		}
		ts := cmp.Or(text(v["timestamp"]), text(v["creation_timestamp"]))
		for _, a := range listOf(v["attachments"]) {
			for _, d := range listOf(pick(a, "data")) {
				if u := importStr(d, "media/uri"); u != "" {
					files = append(files, path.Base(u))
				}
				if p := importFix(importStr(d, "place/name")); p != "" {
					rec["place"] = p
				}
				if u := importStr(d, "external_context/url"); u != "" {
					rec["url"] = u
				}
			}
		}
		for _, md := range listOf(v["media"]) {
			if u := importStr(md, "uri"); u != "" {
				files = append(files, path.Base(u))
			}
			if ts == "" {
				ts = importStr(md, "creation_timestamp")
			}
			if s := importFix(importStr(md, "title")); s != "" && len(texts) == 0 {
				texts = append(texts, s)
			}
		}
		if t := importFix(text(v["title"])); len(texts) == 0 && t != "" && app == "Instagram" {
			texts = append(texts, t)
		}
		rec["text"], rec["files"], rec["at"] = strings.Join(texts, "\n\n"), files, importUnix(ts)
		if rec["at"] == "" {
			return x.put("", nil, at)
		}
		return x.put(importID(app, ts, strings.Join(files, ",")), rec, at)
	})
}

func importFriends(x *importRun, m *importMember, r io.Reader, _ int64) error {
	var v map[string]any
	if err := importDecoder(io.LimitReader(r, importWhole)).Decode(&v); err != nil {
		return err
	}
	list := v["friends_v2"]
	if list == nil {
		list = v["friends"]
	}
	for _, f := range listOf(list) {
		name := importFix(importStr(f, "name"))
		if name == "" {
			continue
		}
		first, last, _ := strings.Cut(name, " ")
		if !x.put("facebook:"+name, map[string]any{"book": "Facebook friends", "full": name, "first": first, "last": last,
			"made": importUnix(importStr(f, "timestamp"))}, 0) {
			return nil
		}
	}
	return nil
}

// Family trees

func importGEDCOM(x *importRun, m *importMember, r io.Reader, _ int64) error {
	lines, err := importUnfold(r)
	if err != nil {
		return err
	}
	type person struct {
		id                                              string
		given, surname, sex, born, bornIn, died, diedIn string
		note                                            []string
		fams, famc                                      []string
	}
	type family struct {
		husb, wife string
		kids       []string
	}
	people, fams := map[string]*person{}, map[string]*family{}
	var order []string
	var p *person
	var f *family
	var event string
	for _, line := range lines {
		line = strings.TrimLeft(strings.TrimSpace(line), "\ufeff")
		fields := strings.SplitN(line, " ", 3)
		if len(fields) < 2 {
			continue
		}
		level := fields[0]
		var xref, tag, val string
		if strings.HasPrefix(fields[1], "@") {
			xref = strings.Trim(fields[1], "@")
			if len(fields) > 2 {
				tag, _, _ = strings.Cut(fields[2], " ")
			}
		} else {
			tag = fields[1]
			if len(fields) > 2 {
				val = fields[2]
			}
		}
		switch level {
		case "0":
			p, f, event = nil, nil, ""
			switch tag {
			case "INDI":
				p = &person{id: xref}
				people[xref] = p
				order = append(order, xref)
			case "FAM":
				f = &family{}
				fams[xref] = f
			}
		case "1":
			event = tag
			ref := strings.Trim(val, "@")
			switch {
			case p != nil && tag == "NAME" && p.given == "" && p.surname == "":
				given, rest, _ := strings.Cut(val, "/")
				surname, _, _ := strings.Cut(rest, "/")
				p.given, p.surname = strings.TrimSpace(given), strings.TrimSpace(surname)
			case p != nil && tag == "SEX":
				p.sex = map[string]string{"M": "male", "F": "female"}[strings.ToUpper(strings.TrimSpace(val))]
			case p != nil && tag == "NOTE" && val != "":
				p.note = append(p.note, val)
			case p != nil && tag == "FAMS":
				p.fams = append(p.fams, ref)
			case p != nil && tag == "FAMC":
				p.famc = append(p.famc, ref)
			case f != nil && tag == "HUSB":
				f.husb = ref
			case f != nil && tag == "WIFE":
				f.wife = ref
			case f != nil && tag == "CHIL":
				f.kids = append(f.kids, ref)
			}
		case "2":
			if p == nil {
				continue
			}
			switch {
			case event == "NAME" && tag == "GIVN":
				p.given = strings.TrimSpace(val)
			case event == "NAME" && tag == "SURN":
				p.surname = strings.TrimSpace(val)
			case event == "BIRT" && tag == "DATE":
				p.born = val
			case event == "BIRT" && tag == "PLAC":
				p.bornIn = val
			case event == "DEAT" && tag == "DATE":
				p.died = val
			case event == "DEAT" && tag == "PLAC":
				p.diedIn = val
			case event == "NOTE" && (tag == "CONT" || tag == "CONC"):
				if n := len(p.note); n > 0 {
					sep := map[string]string{"CONT": "\n", "CONC": ""}[tag]
					p.note[n-1] += sep + val
				}
			}
		}
	}
	// A family names its spouses and children; each person gets their ties both ways.
	ties := map[string][]map[string]any{}
	tie := func(a, b, kind string) {
		if a == "" || b == "" || people[a] == nil || people[b] == nil {
			return
		}
		for _, t := range ties[a] {
			if t["relative"] == b && t["kind"] == kind {
				return
			}
		}
		ties[a] = append(ties[a], map[string]any{"relative": b, "kind": kind})
	}
	for _, fam := range fams {
		tie(fam.husb, fam.wife, "spouse")
		tie(fam.wife, fam.husb, "spouse")
		for _, k := range fam.kids {
			for _, par := range []string{fam.husb, fam.wife} {
				tie(k, par, "parent")
				tie(par, k, "child")
			}
		}
	}
	for _, id := range order {
		p := people[id]
		if !x.put(id, map[string]any{"given": p.given, "surname": p.surname, "sex": p.sex, "born": p.born, "born_in": p.bornIn,
			"died": p.died, "died_in": p.diedIn, "note": importClip(strings.Join(p.note, "\n\n")), "ties": append([]map[string]any{}, ties[id]...)}, 0) {
			return nil
		}
	}
	return nil
}

// DNA

func importDNA(x *importRun, m *importMember, r io.Reader, _ int64) error {
	br := bufio.NewReaderSize(r, 1<<16)
	app, build := "", ""
	markers, called := 0, 0
	chroms := map[string]bool{}
	for {
		line, err := br.ReadString('\n')
		line = strings.TrimSpace(line)
		low := strings.ToLower(line)
		switch {
		case line == "":
		case strings.HasPrefix(line, "#") || strings.HasPrefix(low, "rsid") || strings.HasPrefix(low, `"rsid`):
			for _, a := range []string{"23andMe", "AncestryDNA", "MyHeritage", "FamilyTreeDNA", "Living DNA"} {
				if app == "" && strings.Contains(low, strings.ToLower(a)) {
					app = a
				}
			}
			for _, b := range []struct{ s, build string }{{"build 37", "GRCh37"}, {"grch37", "GRCh37"}, {"build 36", "NCBI36"}, {"build 38", "GRCh38"}, {"grch38", "GRCh38"}} {
				if build == "" && strings.Contains(low, b.s) {
					build = b.build
				}
			}
		default:
			f := strings.FieldsFunc(line, func(r rune) bool { return r == '\t' || r == ',' })
			if len(f) < 4 {
				break
			}
			for i := range f {
				f[i] = strings.Trim(f[i], `" `)
			}
			markers++
			chroms[f[1]] = true
			g := strings.Join(f[3:], "")
			if g != "" && !strings.Contains(g, "-") && !strings.Contains(g, "0") {
				called++
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	if markers == 0 {
		return nil
	}
	if app == "" && strings.Contains(strings.ToLower(m.Path), "myheritage") {
		app = "MyHeritage"
	}
	rec := map[string]any{"app": app, "build": build, "markers": markers, "called": called, "chromosomes": len(chroms)}
	// A DNA file that is the export itself is linked by its own id, as it was kept under the import with it
	rec["file_source"] = m.Path
	if m.whole {
		rec["file_source"] = x.x.file
	}
	x.put(m.Path, rec, 0)
	return nil
}

// Books and films

func importCSV(r io.Reader, fn func(row map[string]string) bool) error {
	cr := csv.NewReader(bufio.NewReader(r))
	cr.FieldsPerRecord, cr.LazyQuotes = -1, true
	head, err := cr.Read()
	if err != nil {
		return err
	}
	for i := range head {
		head[i] = strings.TrimSpace(strings.TrimPrefix(head[i], "\ufeff"))
	}
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		row := map[string]string{}
		for i, v := range rec {
			if i < len(head) {
				row[head[i]] = strings.TrimSpace(v)
			}
		}
		if !fn(row) {
			return nil
		}
	}
}

func importGoodreads(x *importRun, m *importMember, r io.Reader, _ int64) error {
	return importCSV(r, func(row map[string]string) bool {
		id := row["Book Id"]
		if id == "" {
			return x.put("", nil, 0)
		}
		rating := row["My Rating"]
		if rating == "0" {
			rating = ""
		}
		isbn := strings.Trim(strings.TrimPrefix(cmp.Or(row["ISBN13"], row["ISBN"]), "="), `"`)
		return x.put("goodreads:"+id, map[string]any{"app": "Goodreads", "kind": "book", "title": row["Title"], "creator": row["Author"],
			"year": cmp.Or(row["Original Publication Year"], row["Year Published"]), "rating": rating, "review": importClip(row["My Review"]),
			"shelf": row["Exclusive Shelf"], "done": importDay(row["Date Read"]), "added": importDay(row["Date Added"]), "isbn": isbn,
			"url": "https://www.goodreads.com/book/show/" + id}, 0)
	})
}

// importLetterboxd reads every film of a Letterboxd export from its first CSV, merging the others by title and year;
// the other CSVs then give nothing.
func importLetterboxd(x *importRun, m *importMember, r io.Reader, _ int64) error {
	var sibs []*zip.File
	if m.zip != nil {
		for _, f := range m.zip.File {
			if path.Dir(f.Name) == path.Dir(m.f.Name) && slices.Contains(importLetter, path.Base(f.Name)) {
				sibs = append(sibs, f)
			}
		}
	}
	if len(sibs) > 0 && sibs[0] != m.f {
		return nil
	}
	films := map[string]map[string]any{}
	var order []string
	read := func(name string, r io.Reader) error {
		return importCSV(r, func(row map[string]string) bool {
			title, year := row["Name"], row["Year"]
			if title == "" {
				return true
			}
			key := title + " (" + year + ")"
			f := films[key]
			if f == nil {
				f = map[string]any{"app": "Letterboxd", "kind": "film", "title": title, "year": year, "shelf": "watched"}
				films[key] = f
				order = append(order, key)
			}
			switch name {
			case "watchlist.csv":
				if f["done"] == nil && f["rating"] == nil {
					f["shelf"] = "watchlist"
				}
				f["added"] = importDay(row["Date"])
			case "watched.csv":
				f["shelf"] = "watched"
				f["url"] = row["Letterboxd URI"]
				if f["done"] == nil {
					f["done"] = importDay(row["Date"])
				}
			case "ratings.csv":
				f["shelf"], f["rating"] = "watched", row["Rating"]
			case "diary.csv", "reviews.csv":
				f["shelf"] = "watched"
				if d := importDay(cmp.Or(row["Watched Date"], row["Date"])); d != "" && (f["done"] == nil || d > fmt.Sprint(f["done"])) {
					f["done"] = d
				}
				if row["Rating"] != "" {
					f["rating"] = row["Rating"]
				}
				if row["Review"] != "" {
					f["review"] = importClip(row["Review"])
				}
			}
			return true
		})
	}
	if len(sibs) == 0 {
		if err := read(m.baseName(), r); err != nil {
			return err
		}
	}
	for _, f := range sibs {
		fr, err := f.Open()
		if err != nil {
			return err
		}
		err = read(path.Base(f.Name), fr)
		fr.Close()
		if err != nil {
			return err
		}
	}
	for _, k := range order {
		if !x.put("letterboxd:"+k, films[k], 0) {
			return nil
		}
	}
	return nil
}

// iMessage: a chat.db, read with a small reader of SQLite's file format, as Messages keeps it on a Mac.

func importIMessage(x *importRun, m *importMember, r io.Reader, _ int64) error {
	b, err := io.ReadAll(io.LimitReader(r, importWhole+1))
	if err != nil {
		return err
	}
	if len(b) > importWhole {
		return errors.New("chat.db is over 512 MB")
	}
	db, err := sqliteOpen(b)
	if err != nil {
		return err
	}
	handles := map[int64]string{}
	db.scan("handle", func(row map[string]any) bool {
		handles[toInt(row["ROWID"])] = text(row["id"])
		return true
	})
	chats := map[int64][2]string{}
	db.scan("chat", func(row map[string]any) bool {
		chats[toInt(row["ROWID"])] = [2]string{cmp.Or(text(row["guid"]), text(row["chat_identifier"])), cmp.Or(text(row["display_name"]), text(row["chat_identifier"]))}
		return true
	})
	chatOf := map[int64]int64{}
	db.scan("chat_message_join", func(row map[string]any) bool {
		chatOf[toInt(row["message_id"])] = toInt(row["chat_id"])
		return true
	})
	names := map[int64]string{}
	db.scan("attachment", func(row map[string]any) bool {
		names[toInt(row["ROWID"])] = cmp.Or(text(row["transfer_name"]), path.Base(text(row["filename"])))
		return true
	})
	files := map[int64][]string{}
	db.scan("message_attachment_join", func(row map[string]any) bool {
		if n := names[toInt(row["attachment_id"])]; n != "" {
			files[toInt(row["message_id"])] = append(files[toInt(row["message_id"])], n)
		}
		return true
	})
	apple := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	return db.scan("message", func(row map[string]any) bool {
		id := toInt(row["ROWID"])
		body := text(row["text"])
		if body == "" {
			if blob, ok := row["attributedBody"].([]byte); ok {
				body = importTypedString(blob)
			}
		}
		fs := files[id]
		if strings.TrimSpace(strings.ReplaceAll(body, "￼", "")) == "" && len(fs) == 0 {
			return x.put("", nil, 0)
		}
		d := toInt(row["date"])
		if d > 1e12 {
			d /= 1e9
		}
		handle := handles[toInt(row["handle_id"])]
		c, has := chats[chatOf[id]]
		if !has {
			c = [2]string{handle, handle}
		}
		sender, addr := handle, ""
		if toInt(row["is_from_me"]) == 1 {
			sender = "You"
		} else if strings.Contains(handle, "@") {
			addr = strings.ToLower(handle)
		}
		return x.put(cmp.Or(text(row["guid"]), strconv.FormatInt(id, 10)), map[string]any{"channel": cmp.Or(c[1], "iMessage"), "thread": c[0], "sender": sender,
			"address": addr, "text": strings.TrimSpace(strings.ReplaceAll(body, "￼", "")), "files": fs, "at": apple.Add(time.Duration(d) * time.Second).Format(time.RFC3339)}, 0)
	})
}

// importTypedString takes the text out of an attributedBody, an NSAttributedString in Apple's typedstream format.
func importTypedString(b []byte) string {
	i := bytes.Index(b, []byte("NSString"))
	if i < 0 {
		return ""
	}
	p := i + len("NSString") + 5
	if p >= len(b) {
		return ""
	}
	n := int(b[p])
	p++
	switch n {
	case 0x81:
		if p+2 > len(b) {
			return ""
		}
		n, p = int(binary.LittleEndian.Uint16(b[p:])), p+2
	case 0x82:
		if p+4 > len(b) {
			return ""
		}
		n, p = int(binary.LittleEndian.Uint32(b[p:])), p+4
	}
	if p+n > len(b) || n < 0 {
		return ""
	}
	return string(b[p : p+n])
}

func toInt(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case float64:
		return int64(n)
	}
	return 0
}

type sqliteDB struct {
	b      []byte
	page   int
	usable int
	tables map[string]sqliteTable
}

type sqliteTable struct {
	root int
	cols []string
	pk   int
}

func sqliteOpen(b []byte) (*sqliteDB, error) {
	if len(b) < 100 || !bytes.HasPrefix(b, []byte("SQLite format 3\x00")) {
		return nil, errors.New("not a SQLite database")
	}
	db := &sqliteDB{b: b, page: int(binary.BigEndian.Uint16(b[16:])), tables: map[string]sqliteTable{}}
	if db.page == 1 {
		db.page = 65536
	}
	db.usable = db.page - int(b[20])
	err := db.walk(1, func(_ int64, rec []any) bool {
		if len(rec) >= 5 && text(rec[0]) == "table" {
			cols, pk := sqliteCols(text(rec[4]))
			db.tables[text(rec[1])] = sqliteTable{root: int(toInt(rec[3])), cols: cols, pk: pk}
		}
		return true
	})
	return db, err
}

// sqliteCols reads a CREATE TABLE's column names, and which is an INTEGER PRIMARY KEY, kept as the rowid.
func sqliteCols(sql string) ([]string, int) {
	i, j := strings.Index(sql, "("), strings.LastIndex(sql, ")")
	if i < 0 || j < i {
		return nil, -1
	}
	var cols []string
	pk, depth, start := -1, 0, i+1
	body := sql[:j]
	for k := i + 1; k <= len(body); k++ {
		if k < len(body) {
			switch body[k] {
			case '(':
				depth++
				continue
			case ')':
				depth--
				continue
			case ',':
				if depth > 0 {
					continue
				}
			default:
				continue
			}
		}
		def := strings.TrimSpace(body[start:k])
		start = k + 1
		f := strings.Fields(def)
		if len(f) == 0 {
			continue
		}
		switch strings.ToUpper(f[0]) {
		case "PRIMARY", "UNIQUE", "CHECK", "FOREIGN", "CONSTRAINT":
			continue
		}
		up := strings.ToUpper(def)
		if strings.Contains(up, "INTEGER PRIMARY KEY") {
			pk = len(cols)
		}
		cols = append(cols, strings.Trim(f[0], "`\"[]"))
	}
	return cols, pk
}

func (db *sqliteDB) scan(table string, fn func(row map[string]any) bool) error {
	t, ok := db.tables[table]
	if !ok {
		return nil
	}
	return db.walk(t.root, func(rowid int64, rec []any) bool {
		row := map[string]any{"ROWID": rowid}
		for i, c := range t.cols {
			if i < len(rec) {
				row[c] = rec[i]
			}
		}
		if t.pk >= 0 {
			row[t.cols[t.pk]] = rowid
		}
		return fn(row)
	})
}

func (db *sqliteDB) at(n int) ([]byte, int, error) {
	start := (n - 1) * db.page
	if n < 1 || start+db.page > len(db.b) {
		return nil, 0, fmt.Errorf("page %d is past the end", n)
	}
	hdr := 0
	if n == 1 {
		hdr = 100
	}
	return db.b[start : start+db.page], hdr, nil
}

func (db *sqliteDB) walk(n int, fn func(rowid int64, rec []any) bool) error {
	stop := false
	var walk func(n, depth int) error
	walk = func(n, depth int) error {
		if depth > 64 || stop {
			return nil
		}
		p, h, err := db.at(n)
		if err != nil {
			return err
		}
		kind, cells := p[h], int(binary.BigEndian.Uint16(p[h+3:]))
		switch kind {
		case 0x05:
			for i := range cells {
				off := int(binary.BigEndian.Uint16(p[h+12+2*i:]))
				if err := walk(int(binary.BigEndian.Uint32(p[off:])), depth+1); err != nil || stop {
					return err
				}
			}
			return walk(int(binary.BigEndian.Uint32(p[h+8:])), depth+1)
		case 0x0d:
			for i := range cells {
				off := int(binary.BigEndian.Uint16(p[h+8+2*i:]))
				size, k := sqliteVarint(p[off:])
				rowid, k2 := sqliteVarint(p[off+k:])
				payload, err := db.payload(p, off+k+k2, int(size))
				if err != nil {
					return err
				}
				if !fn(rowid, sqliteRecord(payload)) {
					stop = true
					return nil
				}
			}
		}
		return nil
	}
	return walk(n, 0)
}

func (db *sqliteDB) payload(p []byte, off, size int) ([]byte, error) {
	u := db.usable
	x := u - 35
	if size <= x {
		return p[off : off+size], nil
	}
	mm := ((u-12)*32)/255 - 23
	local := mm + (size-mm)%(u-4)
	if local > x {
		local = mm
	}
	out := append([]byte{}, p[off:off+local]...)
	next := int(binary.BigEndian.Uint32(p[off+local:]))
	for len(out) < size && next != 0 {
		op, _, err := db.at(next)
		if err != nil {
			return nil, err
		}
		next = int(binary.BigEndian.Uint32(op))
		out = append(out, op[4:min(u, 4+size-len(out))]...)
	}
	return out, nil
}

func sqliteVarint(b []byte) (int64, int) {
	var v int64
	for i := 0; i < 9 && i < len(b); i++ {
		if i == 8 {
			return v<<8 | int64(b[i]), 9
		}
		v = v<<7 | int64(b[i]&0x7f)
		if b[i]&0x80 == 0 {
			return v, i + 1
		}
	}
	return v, len(b)
}

func sqliteRecord(b []byte) []any {
	hsize, k := sqliteVarint(b)
	var types []int64
	for k < int(hsize) && k < len(b) {
		t, n := sqliteVarint(b[k:])
		types, k = append(types, t), k+n
	}
	off := int(hsize)
	out := make([]any, len(types))
	for i, t := range types {
		n := 0
		switch {
		case t >= 1 && t <= 4:
			n = int(t)
		case t == 5:
			n = 6
		case t == 6 || t == 7:
			n = 8
		case t >= 12:
			n = int(t-12) / 2
		}
		if off+n > len(b) {
			break
		}
		v := b[off : off+n]
		switch {
		case t == 0:
		case t >= 1 && t <= 6:
			var x int64
			for _, c := range v {
				x = x<<8 | int64(c)
			}
			if n < 8 && v[0]&0x80 != 0 {
				x -= 1 << (8 * n)
			}
			out[i] = x
		case t == 7:
			out[i] = math.Float64frombits(binary.BigEndian.Uint64(v))
		case t == 8:
			out[i] = int64(0)
		case t == 9:
			out[i] = int64(1)
		case t >= 12 && t%2 == 0:
			out[i] = append([]byte{}, v...)
		case t >= 13:
			out[i] = string(v)
		}
		off += n
	}
	return out
}
