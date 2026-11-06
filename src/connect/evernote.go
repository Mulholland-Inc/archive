package connect

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/oauth2"
)

// Evernote signs in with OAuth 1.0a and speaks Thrift over HTTP, not JSON: its token is the authentication token every
// call carries, and the access response names the person's NoteStore, where their notes are. Its streams and actions
// are Go. A key starts on sandbox.evernote.com: ARCHIVE_EVERNOTE's host says which, and every URL but the NoteStore's
// follows it.
func init() {
	Register(&Connector{
		ID: "evernote", Name: "Evernote",
		Info: Info{By: "Evernote", Description: "Read your notebooks, notes and their attachments, never change them", Category: CatDocs, Icon: IconNotebook,
			Scopes: []string{"Read your notebooks and notes", "Read the files in your notes"}},
		API:      evernoteHost,
		OAuth1:   evernoteHost + "/oauth",
		Endpoint: oauth2.Endpoint{AuthURL: evernoteHost + "/OAuth.action", TokenURL: evernoteHost + "/oauth"},
		Lasts:    true,
		Keep:     []string{"edam_noteStoreUrl", "edam_userId"},
		Account:  evernoteAccount,
		RevokeWith: func(rv *Revocation) error {
			tok, err := rv.Access()
			if err == nil {
				_, err = evernoteDo(rv.ctx, rv.p.at(evernoteUsers), "revokeLongSession", tok)
			}
			if rf := (*Refusal)(nil); errors.As(err, &rf) && rf.Status == http.StatusUnauthorized {
				return nil
			}
			return err
		},
		Actions: map[string]*Action{
			"search": Func("Find Evernote notes, newest edited first, by Evernote's search grammar, like words, tag:recipes or notebook:Travel: "+
				"{ total, notes: [{ id, title, notebook, created, updated }] }, 50 at most", evernoteSearch),
			"note": Func("One Evernote note by its id from search, with its words as plain text, its notebook's id and its tags", evernoteOne),
		},
		Sync: []*Sync{
			{Name: "notebooks", Gap: time.Second, Every: 15 * time.Minute, Pull: evernoteNotebooks, Memo: "thrift 1",
				Doc: "Every notebook of yours, by its guid, listed whole every 15 minutes, which drops those deleted. Notebooks others share with you are not in it.",
				To: Map{Table: "notebook",
					Cols: map[string]SQL{"title": "data->>'name'", "stack": "data->>'stack'", "is_default": "(data->>'default')::boolean",
						"made_at": "archive.sync_at(data->>'created')", "edited_at": "archive.sync_at(data->>'updated')"}}},
			{Name: "notes", Gap: time.Second, Every: 15 * time.Minute, Pull: evernoteNotes, Memo: "thrift 1",
				Doc: "Every note of your notebooks but those in the trash, by its guid, with its words as plain text cut at 1 MB, its tags, and each file in it " +
					"kept with the words of each PDF, Word, Excel, PowerPoint or text file up to 25 MB. The first time, every note by its update number, " +
					"25 a page; after, every 15 minutes, only the notes changed or deleted since, unless Evernote asks for a whole pass again. " +
					"Evernote limits how many calls an app makes an hour, so a big account's first sync pauses and resumes until it is whole.",
				To: Map{Table: "note",
					Cols: map[string]SQL{"title": "data->>'title'", "text": "data->>'text'", "tags": "data->>'tags'", "author": "data->>'author'",
						"source_url": "data->>'source_url'", "place": "data->>'place'", "url": "data->>'url'",
						"reminder_at": "archive.sync_at(data->>'reminder')", "done_at": "archive.sync_at(data->>'done')",
						"made_at": "archive.sync_at(data->>'created')", "edited_at": "archive.sync_at(data->>'updated')"},
					Refs: map[string]SQL{"notebook": "data->>'notebook'"},
					Children: []Kids{{Table: "note_attachment", From: "archive.sync_list(data->'attachments') f",
						Cols: map[string]SQL{"filename": "f->>'filename'", "mime": "f->>'mime'", "size": "archive.sync_num(f->>'size')", "text": "f->>'text'", "file": "f->>'file'"}}}}},
		},
	})
}

const (
	evernoteHost   = "https://www.evernote.com"
	evernoteUsers  = evernoteHost + "/edam/user"
	evernoteBatch  = 25
	evernoteBudget = 75 * time.Second
	evernoteText   = 1 << 20
)

var evernoteHTTP = &http.Client{Timeout: time.Minute}

// evernoteDo calls a Thrift method with the token as its first argument and args after, and turns Evernote's
// exceptions into refusals: an expired or revoked token is a 401, a rate limit is busy for as long as Evernote says.
func evernoteDo(ctx context.Context, u, method, token string, args ...any) (TReply, error) {
	st := TStruct{{1, token}}
	for i, a := range args {
		st = append(st, TField{int16(i + 2), a})
	}
	r, err := ThriftCall(ctx, evernoteHTTP, u, method, st)
	var rf *Refusal
	var ne net.Error
	switch {
	case errors.As(err, &rf):
		rf.Provider = "Evernote"
		rf.Busy = busyStatus(rf.Status)
		return nil, rf
	case errors.As(err, &ne), errors.Is(err, net.ErrClosed):
		return nil, unreached{fmt.Errorf("Evernote could not be reached: %w", err)}
	case err != nil:
		return nil, err
	}
	if e := r.Struct(1); len(e) > 0 {
		return nil, evernoteRefusal(e.Int(1), e.Str(2), 0)
	}
	if e := r.Struct(2); len(e) > 0 {
		return nil, evernoteRefusal(e.Int(1), e.Str(2), e.Int(3))
	}
	if e := r.Struct(3); len(e) > 0 {
		return nil, &Refusal{Provider: "Evernote", Status: http.StatusNotFound, Message: strings.TrimSpace(e.Str(1) + " " + e.Str(2))}
	}
	return r, nil
}

// evernoteRefusal is an EDAMErrorCode as a refusal.
func evernoteRefusal(code int64, msg string, wait int64) *Refusal {
	rf := &Refusal{Provider: "Evernote", Message: fmt.Sprintf("error %d", code)}
	if msg != "" {
		rf.Message += ": " + msg
	}
	switch code {
	case 8, 9:
		rf.Status, rf.Message = http.StatusUnauthorized, "Archive's access to your Evernote expired or was revoked; connect Evernote again"
	case 3:
		rf.Status = http.StatusForbidden
	case 19:
		rf.Busy, rf.Retry = true, time.Duration(max(wait, 60))*time.Second
		rf.Message = fmt.Sprintf("Evernote limits how often Archive may ask; it resumes in %s", rf.Retry)
	case 4, 12:
		rf.Busy = true
	}
	return rf
}

func evernoteCall(c *Client, method string, args ...any) (TReply, error) {
	store, err := evernoteStore(c)
	if err != nil {
		return nil, err
	}
	return evernoteDo(c.ctx, store, method, c.Token(), args...)
}

// evernoteStore is the person's NoteStore, from their sign-in, which must be Evernote's.
func evernoteStore(c *Client) (string, error) {
	u, err := url.Parse(c.Meta["edam_noteStoreUrl"])
	if err != nil || u.Scheme != "https" || u.Host != "evernote.com" && !strings.HasSuffix(u.Host, ".evernote.com") {
		return "", fmt.Errorf("Evernote named a NoteStore Archive does not trust: %q", c.Meta["edam_noteStoreUrl"])
	}
	return u.String(), nil
}

// evernoteWeb is where a note opens: the NoteStore's shard, /nl/<user>/<note>/.
func evernoteWeb(c *Client, guid string) string {
	store, err := evernoteStore(c)
	if err != nil || c.Meta["edam_userId"] == "" {
		return ""
	}
	return strings.TrimSuffix(store, "/notestore") + "/nl/" + c.Meta["edam_userId"] + "/" + guid + "/"
}

func evernoteAccount(c *Client) (string, error) {
	if _, err := evernoteStore(c); err != nil {
		return "", err
	}
	r, err := evernoteDo(c.ctx, c.p.at(evernoteUsers), "getUser", c.Token())
	if err != nil {
		return "", err
	}
	u := r.Struct(0)
	return strings.ToLower(cmp.Or(u.Str(3), u.Str(2), u.Str(4))), nil
}

func evernoteMs(ms int64) string {
	if ms <= 0 {
		return ""
	}
	return time.UnixMilli(ms).UTC().Format(time.RFC3339)
}

func evernoteNotebooks(c *Client, cursor string) (*Page, error) {
	r, err := evernoteCall(c, "listNotebooks")
	if err != nil {
		return nil, err
	}
	var recs []Record
	for _, nb := range r.Structs(0) {
		guid := nb.Str(1)
		recs = append(recs, Record{ID: guid, Updated: time.UnixMilli(nb.Int(8)), Data: map[string]any{"id": guid, "name": nb.Str(2),
			"stack": nb.Str(12), "default": nb.Bool(6), "created": evernoteMs(nb.Int(7)), "updated": evernoteMs(nb.Int(8))}})
	}
	return &Page{Records: recs, Full: true}, nil
}

// evernoteState is where the notes stream is in Evernote's sync: the update number it has every change up to, whether
// this pass is whole or only what changed, when the last pass ended by Evernote's clock, and every tag's name by guid,
// since a note names its tags only by guid.
type evernoteState struct {
	USN    int64             `json:"u,omitempty"`
	Mode   string            `json:"m,omitempty"`
	Synced int64             `json:"s,omitempty"`
	Tags   map[string]string `json:"t,omitempty"`
}

func evernoteNotes(c *Client, cursor string) (*Page, error) {
	var st evernoteState
	if err := cursorOf(cursor, &st); err != nil {
		return nil, err
	}
	if st.Mode == "" {
		r, err := evernoteCall(c, "getSyncState")
		if err != nil {
			return nil, err
		}
		ss := r.Struct(0)
		if st.USN > 0 && ss.Int(2) > st.Synced {
			st = evernoteState{}
		}
		if ss.Int(3) == st.USN {
			st.Synced = ss.Int(1)
			return &Page{Cursor: cursorFor(st)}, nil
		}
		st.Mode = "delta"
		if st.USN == 0 {
			st.Mode, st.Tags = "full", nil
		}
	}
	if st.Tags == nil {
		st.Tags = map[string]string{}
	}
	full := st.Mode == "full"
	filter := TStruct{{1, true}, {3, true}, {5, true}, {9, !full}}
	r, err := evernoteCall(c, "getFilteredSyncChunk", int32(st.USN), int32(evernoteBatch), filter)
	if err != nil {
		return nil, err
	}
	chunk := r.Struct(0)
	for _, t := range chunk.Structs(6) {
		st.Tags[t.Str(1)] = t.Str(2)
	}
	for _, g := range chunk.Strs(11) {
		delete(st.Tags, g)
	}
	var recs []Record
	for _, g := range chunk.Strs(9) {
		recs = append(recs, Record{ID: g, Deleted: true})
	}
	high, count := chunk.Int(2), chunk.Int(3)
	if !chunk.Has(2) {
		high = count
	}
	notes := chunk.Structs(4)
	slices.SortFunc(notes, func(a, b TReply) int { return int(a.Int(10) - b.Int(10)) })
	until := time.Now().Add(evernoteBudget)
	if d, ok := c.ctx.Deadline(); ok && d.Add(-30*time.Second).Before(until) {
		until = d.Add(-30 * time.Second)
	}
	done := st.USN
	for i, n := range notes {
		if i > 0 && time.Now().After(until) {
			high = done
			break
		}
		guid, rec := n.Str(1), Record{ID: n.Str(1), Deleted: true}
		if !n.Has(9) || n.Bool(9) {
			var err error
			rec, err = evernoteRecord(c, guid, st.Tags)
			var rf *Refusal
			switch {
			case errors.As(err, &rf) && rf.Status == http.StatusNotFound:
				rec = Record{ID: guid, Deleted: true}
			case err != nil && Busy(err) && i > 0:
				// Keep the notes read so far; the next page meets the limit and waits it out.
				high = done
			case err != nil:
				return nil, err
			}
			if err != nil && rec.ID == "" {
				break
			}
		}
		recs = append(recs, rec)
		done = n.Int(10)
	}
	st.USN = max(high, st.USN)
	more := st.USN < count
	if !more {
		st.Mode, st.Synced = "", chunk.Int(1)
	}
	return &Page{Records: recs, Cursor: cursorFor(st), More: more, Full: full}, nil
}

// evernoteRecord reads a note whole, and keeps each file in it.
func evernoteRecord(c *Client, guid string, tags map[string]string) (Record, error) {
	r, err := evernoteCall(c, "getNote", guid, true, false, false, false)
	if err != nil {
		return Record{}, err
	}
	n := r.Struct(0)
	var names []string
	for _, g := range n.Strs(12) {
		if tags[g] != "" {
			names = append(names, tags[g])
		}
	}
	data := evernoteNote(c, n)
	data["tags"] = strings.Join(names, ", ")
	var files []map[string]any
	for _, res := range n.Structs(13) {
		if res.Has(8) && !res.Bool(8) {
			continue
		}
		f, err := evernoteFile(c, res)
		if err != nil {
			return Record{}, err
		}
		files = append(files, f)
	}
	data["attachments"] = files
	return Record{ID: guid, Data: data, Updated: time.UnixMilli(n.Int(7))}, nil
}

func evernoteNote(c *Client, n TReply) map[string]any {
	guid, at := n.Str(1), n.Struct(14)
	data := map[string]any{"id": guid, "title": n.Str(2), "text": clip(enmlText(n.Str(3)), evernoteText), "notebook": n.Str(11),
		"created": evernoteMs(n.Int(6)), "updated": evernoteMs(n.Int(7)), "url": evernoteWeb(c, guid),
		"author": at.Str(13), "source_url": at.Str(15), "reminder": evernoteMs(at.Int(20)), "done": evernoteMs(at.Int(19))}
	switch lat, lon := at[10], at[11]; {
	case at.Str(21) != "":
		data["place"] = at.Str(21)
	case lat != nil && lon != nil:
		data["place"] = fmt.Sprintf("%.5f, %.5f", lat, lon)
	}
	return data
}

// evernoteFile is a file in a note, its bytes kept and read when it is not over 25 MB.
func evernoteFile(c *Client, res TReply) (map[string]any, error) {
	guid, kind, size := res.Str(1), res.Str(4), res.Struct(3).Int(2)
	name := res.Struct(11).Str(10)
	if name == "" {
		name = guid
		if exts, _ := mime.ExtensionsByType(kind); len(exts) > 0 {
			name += exts[0]
		}
	}
	f := map[string]any{"filename": res.Struct(11).Str(10), "mime": kind, "size": size}
	if size > maxFile || c.Store == nil {
		return f, nil
	}
	r, err := evernoteCall(c, "getResourceData", guid)
	if err != nil {
		return nil, err
	}
	b := []byte(r.Str(0))
	if f["file"], err = c.keep(name, b); err != nil {
		return nil, err
	}
	f["text"] = bytesText(name, kind, b)
	return f, nil
}

type evernoteFind struct {
	Query  string `json:"query" jsonschema:"Words, or Evernote's search grammar, like tag:recipes, notebook:Travel, intitle:invoice or created:day-7"`
	Limit  int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 50; 20 when left out"`
	Offset int    `json:"offset,omitempty" jsonschema:"How many to skip, for more"`
}

func evernoteSearch(c *Client, in evernoteFind) (map[string]any, error) {
	limit := clamp(cmp.Or(in.Limit, 20), 1, 50)
	filter := TStruct{{1, int32(2)}, {2, false}, {3, in.Query}}
	spec := TStruct{{2, true}, {6, true}, {7, true}, {11, true}}
	r, err := evernoteCall(c, "findNotesMetadata", filter, int32(max(in.Offset, 0)), int32(limit), spec)
	if err != nil {
		return nil, err
	}
	list := r.Struct(0)
	notes := []map[string]any{}
	for _, n := range list.Structs(3) {
		notes = append(notes, map[string]any{"id": n.Str(1), "title": n.Str(2), "notebook": n.Str(11),
			"created": evernoteMs(n.Int(6)), "updated": evernoteMs(n.Int(7)), "url": evernoteWeb(c, n.Str(1))})
	}
	return map[string]any{"total": list.Int(2), "notes": notes}, nil
}

type evernoteID struct {
	ID string `json:"id" jsonschema:"The note's id, from search"`
}

func evernoteOne(c *Client, in evernoteID) (map[string]any, error) {
	if strings.TrimSpace(in.ID) == "" {
		return nil, &Refusal{Provider: "Evernote", Status: http.StatusBadRequest, Message: "which note? Give its id from search"}
	}
	r, err := evernoteCall(c, "getNote", in.ID, true, false, false, false)
	if err != nil {
		return nil, err
	}
	n := r.Struct(0)
	data := evernoteNote(c, n)
	if t, err := evernoteCall(c, "getNoteTagNames", in.ID); err == nil {
		data["tags"] = t.Strs(0)
	}
	var files []string
	for _, res := range n.Structs(13) {
		files = append(files, cmp.Or(res.Struct(11).Str(10), res.Str(4)))
	}
	data["attachments"] = files
	return data, nil
}

// enmlText is a note's ENML as plain text: a line for each block, a checkbox as [ ] or [x], a file as [its type].
func enmlText(enml string) string {
	var b strings.Builder
	z := html.NewTokenizer(strings.NewReader(enml))
	line := func() {
		if s := b.String(); s != "" && !strings.HasSuffix(s, "\n") && s != "- " && !strings.HasSuffix(s, "\n- ") {
			b.WriteByte('\n')
		}
	}
	skip := 0
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}
		name, attrs := z.TagName()
		tag := string(name)
		attr := map[string]string{}
		for attrs {
			var k, v []byte
			k, v, attrs = z.TagAttr()
			attr[string(k)] = string(v)
		}
		switch tt {
		case html.TextToken:
			if skip == 0 {
				b.WriteString(strings.Join(strings.FieldsFunc(string(z.Text()), func(r rune) bool { return r == '\n' || r == '\r' || r == '\t' }), " "))
			}
		case html.StartTagToken, html.SelfClosingTagToken:
			if enmlBlock(tag) {
				line()
			}
			switch tag {
			case "style", "script", "title":
				if tt == html.StartTagToken {
					skip++
				}
			case "br", "hr":
				b.WriteByte('\n')
			case "li":
				b.WriteString("- ")
			case "en-todo":
				if attr["checked"] == "true" {
					b.WriteString("[x] ")
				} else {
					b.WriteString("[ ] ")
				}
			case "en-media":
				b.WriteString("[" + cmp.Or(attr["type"], "file") + "]")
			case "en-crypt":
				b.WriteString("[encrypted]")
				skip++
			case "td", "th":
				b.WriteByte('\t')
			}
		case html.EndTagToken:
			switch {
			case tag == "style" || tag == "script" || tag == "title" || tag == "en-crypt":
				skip = max(skip-1, 0)
			case enmlBlock(tag):
				line()
			}
		}
	}
	var out []string
	for _, l := range strings.Split(b.String(), "\n") {
		l = strings.TrimSpace(strings.ReplaceAll(l, "\u00a0", " "))
		if l != "" || len(out) > 0 && out[len(out)-1] != "" {
			out = append(out, l)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func enmlBlock(tag string) bool {
	switch tag {
	case "div", "p", "h1", "h2", "h3", "h4", "h5", "h6", "ul", "ol", "li", "table", "tr", "blockquote", "pre", "en-note":
		return true
	}
	return false
}
