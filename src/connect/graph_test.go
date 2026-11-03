package connect

import (
	"net/http"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// graphParts are the parts a Graph fake has, mail folders or drives, by the ids it lists now; each holds two
// items, <id>1 and <id>2. refuse answers a part's delta with a status and message instead.
type graphParts struct {
	listed []string
	refuse map[string]graphRefusal
}

type graphRefusal struct {
	status  int
	message string
}

// graphDelta are the paths of a mail folder's delta and of a drive's.
var graphDelta = regexp.MustCompile(`^/v1\.0/(?:me/mailFolders/([^/]+)/messages|drives/([^/]+)/root)/delta$`)

// graphFake is Graph with parts: the mailbox's folders, or a site search that finds a site per part with a
// library each. A part's delta lists it whole in two pages, by @odata.nextLink, and ends at a deltaLink of
// $deltatoken=<id>; from there it has nothing new. Links are Graph's own, which the client moves onto the fake.
func graphFake(t *testing.T, g *graphParts) *fake {
	return newFake(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch p := r.URL.Path; {
		case p == "/v1.0/me/mailFolders/junkemail" || p == "/v1.0/me/mailFolders/deleteditems" || p == "/v1.0/me/drive":
			graphError(w, http.StatusNotFound, "ErrorItemNotFound", "The specified object was not found in the store.")
		case p == "/v1.0/me/mailFolders":
			var v []any
			for _, id := range g.listed {
				v = append(v, map[string]any{"id": id, "displayName": "Folder " + id, "childFolderCount": 0})
			}
			writeJSON(w, map[string]any{"value": v})
		case p == "/v1.0/sites":
			var v []any
			for _, id := range g.listed {
				v = append(v, map[string]any{"id": "site-" + id, "displayName": "Site " + id})
			}
			writeJSON(w, map[string]any{"value": v})
		case strings.HasPrefix(p, "/v1.0/sites/site-") && strings.HasSuffix(p, "/drives"):
			id := strings.TrimSuffix(strings.TrimPrefix(p, "/v1.0/sites/site-"), "/drives")
			writeJSON(w, map[string]any{"value": []any{map[string]any{"id": id, "name": "Documents"}}})
		case graphDelta.MatchString(p):
			m := graphDelta.FindStringSubmatch(p)
			id, link := m[1]+m[2], "https://graph.microsoft.com"+p
			if no, ok := g.refuse[id]; ok {
				graphError(w, no.status, "", no.message)
				return
			}
			item := func(n string) map[string]any {
				return map[string]any{"id": id + n, "name": id + n, "subject": id + n, "folder": map[string]any{},
					"lastModifiedDateTime": "2026-10-01T10:00:00Z"}
			}
			switch {
			case q.Get("$deltatoken") != "":
				writeJSON(w, map[string]any{"value": []any{}, "@odata.deltaLink": link + "?$deltatoken=" + id})
			case q.Get("$skiptoken") != "":
				writeJSON(w, map[string]any{"value": []any{item("2")}, "@odata.deltaLink": link + "?$deltatoken=" + id})
			default:
				writeJSON(w, map[string]any{"value": []any{item("1")}, "@odata.nextLink": link + "?$skiptoken=" + id})
			}
		default:
			t.Errorf("Graph asked %s", r.URL)
			http.NotFound(w, r)
		}
	})
}

// graphError answers as Graph refuses: { error: { code, message } }.
func graphError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	writeJSON(w, map[string]any{"error": map[string]any{"code": code, "message": message}})
}

// asked are the deltas f was asked from request from on, each as the part's id and whether it was read whole
// or since its delta link; the next pages of a delta are left out.
func asked(f *fake, from int) []string {
	var out []string
	for _, r := range f.requests()[from:] {
		m := graphDelta.FindStringSubmatch(r.Path)
		if m == nil || r.Query.Get("$skiptoken") != "" {
			continue
		}
		if r.Query.Get("$deltatoken") != "" {
			out = append(out, m[1]+m[2]+" since")
		} else {
			out = append(out, m[1]+m[2]+" whole")
		}
	}
	return out
}

// deletes says whether a sync saving pages would delete the rows the listing did not read, as syncSave does
// at a Full page with no more to come.
func deletes(pages []*Page) bool {
	return slices.ContainsFunc(pages, func(p *Page) bool { return p.Full && !p.More })
}

// graphWalks pulls walk after walk of s on the fake, each from where the one before ended.
type graphWalks struct {
	t      *testing.T
	f      *fake
	c      *Client
	s      *Stream
	cursor string
}

// walk is one sync's pages of the stream, and the deltas it asked.
func (w *graphWalks) walk() ([]*Page, []string) {
	w.t.Helper()
	from := len(w.f.requests())
	pages, cursor := pull(w.t, w.c, w.s, w.cursor)
	w.cursor = cursor
	return pages, asked(w.f, from)
}

func outlookWalks(t *testing.T, g *graphParts) *graphWalks {
	f := graphFake(t, g)
	return &graphWalks{t: t, f: f, c: f.open(t, "outlook", "", nil, nil), s: stream(t, "outlook", "messages")}
}

// A folder Graph leaves out of one walk deletes nothing; left out of a second, the walk after reads every
// folder whole, which deletes what was in it. Before #290 the first miss alone listed the mailbox whole, so a
// folder Graph skipped once lost every row.
func TestGraphFolderGoneOnlyAfterTwoMisses(t *testing.T) {
	g := &graphParts{listed: []string{"A", "B"}}
	w := outlookWalks(t, g)
	pages, got := w.walk()
	if want := []string{"A whole", "B whole"}; !slices.Equal(got, want) || !deletes(pages) {
		t.Fatalf("the first walk asked %v, deletes %v; want %v as a listing", got, deletes(pages), want)
	}
	if got, want := ids(pages...), []string{"A1", "A2", "B1", "B2"}; !slices.Equal(got, want) {
		t.Errorf("the first walk listed %v, want %v", got, want)
	}

	g.listed = []string{"A"}
	pages, got = w.walk()
	if deletes(pages) {
		t.Error("a walk that missed B once deletes what it did not list")
	}
	if want := []string{"A since"}; !slices.Equal(got, want) {
		t.Errorf("the walk that missed B asked %v, want %v", got, want)
	}

	pages, got = w.walk()
	if !deletes(pages) {
		t.Error("a walk after B was missed twice does not delete B's rows")
	}
	if want := []string{"A whole"}; !slices.Equal(got, want) {
		t.Errorf("the walk after the second miss asked %v, want %v", got, want)
	}
	if got, want := ids(pages...), []string{"A1", "A2"}; !slices.Equal(got, want) {
		t.Errorf("the listing listed %v, want %v", got, want)
	}

	pages, got = w.walk()
	if deletes(pages) || !slices.Equal(got, []string{"A since"}) {
		t.Errorf("after the listing a walk asked %v, deletes %v; want A since, deleting nothing", got, deletes(pages))
	}
}

// A folder missed once and back the walk after goes on from its delta link, and nothing is listed whole.
// Before #290 the miss listed every folder whole and forgot B's delta.
func TestGraphFolderBackAfterOneMissKeepsDelta(t *testing.T) {
	g := &graphParts{listed: []string{"A", "B"}}
	w := outlookWalks(t, g)
	w.walk()
	g.listed = []string{"A"}
	w.walk()
	g.listed = []string{"A", "B"}
	pages, got := w.walk()
	if deletes(pages) {
		t.Error("a walk with B back deletes what it did not list")
	}
	if want := []string{"A since", "B since"}; !slices.Equal(got, want) {
		t.Errorf("the walk with B back asked %v, want %v", got, want)
	}
}

// A walk that finds no folders where the one before found some is an error, and the sync after goes on from
// the cursor before it. Before #290 it was a listing of nothing, which deleted the whole mailbox.
func TestGraphNoFoldersAfterSomeIsAnError(t *testing.T) {
	g := &graphParts{listed: []string{"A", "B"}}
	w := outlookWalks(t, g)
	w.walk()
	g.listed = nil
	if p, err := w.s.Pull(w.c, w.cursor); err == nil {
		t.Fatalf("a walk of no folders after two is %d records, Full %v, More %v; want an error", len(p.Records), p.Full, p.More)
	}
	g.listed = []string{"A", "B"}
	pages, got := w.walk()
	if want := []string{"A since", "B since"}; !slices.Equal(got, want) || deletes(pages) {
		t.Errorf("after the error a walk asked %v, deletes %v; want %v, deleting nothing", got, deletes(pages), want)
	}
}

// The first walk of an account with no folders is no error: there is nothing it had.
func TestGraphNoFoldersAtFirstIsEmpty(t *testing.T) {
	w := outlookWalks(t, &graphParts{})
	if pages, _ := w.walk(); len(ids(pages...)) != 0 {
		t.Errorf("an empty mailbox listed %v", ids(pages...))
	}
}

// A folder whose delta Graph refuses, 403, is skipped this walk while the others sync, and keeps its delta link
// for the next. Before #290 the refusal failed the walk, every folder after it unsynced until Graph relented.
func TestGraphRefusedFolderKeepsDelta(t *testing.T) {
	g := &graphParts{listed: []string{"A", "B", "C"}}
	w := outlookWalks(t, g)
	w.walk()
	g.refuse = map[string]graphRefusal{"B": {http.StatusForbidden, "Access is denied. Check credentials and try again."}}
	pages, got := w.walk()
	if want := []string{"A since", "B since", "C since"}; !slices.Equal(got, want) {
		t.Errorf("the walk with B refused asked %v, want %v", got, want)
	}
	if deletes(pages) {
		t.Error("the walk with B refused deletes what it did not list")
	}
	g.refuse = nil
	if _, got := w.walk(); !slices.Equal(got, []string{"A since", "B since", "C since"}) {
		t.Errorf("the walk after B was refused asked %v, want B since its delta", got)
	}
}

// A folder Graph refuses, 404, in a listing keeps the listing from deleting, as B's rows were not read; the next
// walk reads B whole as its delta. Before #290 the refusal failed the first sync of the mailbox.
func TestGraphRefusedFolderInAListingDeletesNothing(t *testing.T) {
	g := &graphParts{listed: []string{"A", "B", "C"}, refuse: map[string]graphRefusal{"B": {http.StatusNotFound, "The specified object was not found in the store."}}}
	w := outlookWalks(t, g)
	pages, got := w.walk()
	if want := []string{"A whole", "B whole", "C whole"}; !slices.Equal(got, want) {
		t.Errorf("the listing asked %v, want %v", got, want)
	}
	if deletes(pages) {
		t.Error("a listing that skipped B deletes what it did not list")
	}
	if got, want := ids(pages...), []string{"A1", "A2", "C1", "C2"}; !slices.Equal(got, want) {
		t.Errorf("the listing listed %v, want %v", got, want)
	}
	g.refuse = nil
	pages, got = w.walk()
	if want := []string{"A since", "B whole", "C since"}; !slices.Equal(got, want) || deletes(pages) {
		t.Errorf("the walk after asked %v, deletes %v; want %v, deleting nothing", got, deletes(pages), want)
	}
}

// A 400 for a sync state Graph no longer keeps, as mail answers a stale delta, lists every folder whole again, as
// 410 does. Before #290 only 410 did, and the 400 failed every sync after.
func TestGraphSyncStateResyncs(t *testing.T) {
	g := &graphParts{listed: []string{"A", "B"}}
	w := outlookWalks(t, g)
	w.walk()
	g.refuse = map[string]graphRefusal{"B": {http.StatusBadRequest, "The sync state generation is not found or is invalid."}}
	p, err := w.s.Pull(w.c, w.cursor)
	if err != nil {
		t.Fatalf("A's delta: %v", err)
	}
	if p, err = w.s.Pull(w.c, p.Cursor); err != nil {
		t.Fatalf("a stale sync state is %v, want a resync", err)
	}
	if !p.More {
		t.Fatal("the resync has no more to come")
	}
	g.refuse = nil
	from := len(w.f.requests())
	pages, cursor := pull(t, w.c, w.s, p.Cursor)
	w.cursor = cursor
	if got, want := asked(w.f, from), []string{"A whole", "B whole"}; !slices.Equal(got, want) || !deletes(pages) {
		t.Errorf("the resync asked %v, deletes %v; want %v as a listing", got, deletes(pages), want)
	}
	if _, got := w.walk(); !slices.Equal(got, []string{"A since", "B since"}) {
		t.Errorf("the walk after the resync asked %v, want both since", got)
	}
}

// A resync after which Graph lists no folders is an error, not a listing of none: the resync keeps the folders
// it had. Before, it forgot them, so the guard against an empty walk could not fire, and the listing of nothing
// deleted the whole mailbox.
func TestGraphResyncThenNoFoldersIsAnError(t *testing.T) {
	g := &graphParts{listed: []string{"A", "B"}}
	w := outlookWalks(t, g)
	w.walk()
	g.refuse = map[string]graphRefusal{"A": {http.StatusGone, "resyncRequired"}}
	p, err := w.s.Pull(w.c, w.cursor)
	if err != nil || !p.More {
		t.Fatalf("A gone is More %v, %v; want a resync", p != nil && p.More, err)
	}
	g.refuse, g.listed = nil, nil
	if p, err := w.s.Pull(w.c, p.Cursor); err == nil {
		t.Errorf("a resync that finds no folders is Full %v, More %v; want an error", p.Full, p.More)
	}
}

// A 400 that is not about a sync state is an error, not a resync that lists the mailbox whole.
func TestGraphBadRequestIsNoResync(t *testing.T) {
	g := &graphParts{listed: []string{"A"}}
	w := outlookWalks(t, g)
	w.walk()
	g.refuse = map[string]graphRefusal{"A": {http.StatusBadRequest, "Invalid filter clause"}}
	if p, err := w.s.Pull(w.c, w.cursor); err == nil {
		t.Errorf("a bad request is Full %v, More %v; want an error", p.Full, p.More)
	}
}

// A site the search leaves out once deletes none of its library's files; out twice, the walk after lists every
// library whole. Before #290 a site search that skipped one site once deleted every file of it.
func TestGraphSharePointLibraryGoneOnlyAfterTwoMisses(t *testing.T) {
	g := &graphParts{listed: []string{"D", "E"}}
	f := graphFake(t, g)
	w := &graphWalks{t: t, f: f, c: f.open(t, "sharepoint", "", nil, nil), s: stream(t, "sharepoint", "files")}
	pages, got := w.walk()
	if want := []string{"D whole", "E whole"}; !slices.Equal(got, want) || !deletes(pages) {
		t.Fatalf("the first walk asked %v, deletes %v; want %v as a listing", got, deletes(pages), want)
	}
	if got, want := ids(pages...), []string{"D/D1", "D/D2", "E/E1", "E/E2"}; !slices.Equal(got, want) {
		t.Errorf("the first walk listed %v, want %v", got, want)
	}
	g.listed = []string{"D"}
	if pages, got := w.walk(); deletes(pages) || !slices.Equal(got, []string{"D since"}) {
		t.Errorf("the walk that missed E asked %v, deletes %v; want D since, deleting nothing", got, deletes(pages))
	}
	if pages, got := w.walk(); !deletes(pages) || !slices.Equal(got, []string{"D whole"}) {
		t.Errorf("the walk after E was missed twice asked %v, deletes %v; want D whole as a listing", got, deletes(pages))
	}
}
