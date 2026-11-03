package connect

import (
	"net/http"
	"slices"
	"strings"
	"testing"
)

// driveFake is an account whose files Drive lists in two pages, a and b then c, saying of the pages incomplete
// marks that they may have left files out; its changes feed answers changes. Every file is a folder, which has
// no text to read.
func driveFake(t *testing.T, incomplete map[string]bool, changes []any) *fake {
	folder := func(id string) map[string]any {
		return map[string]any{"id": id, "name": "Folder " + id, "mimeType": "application/vnd.google-apps.folder",
			"ownedByMe": true, "modifiedTime": "2026-10-01T10:00:00Z"}
	}
	return newFake(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch r.URL.Path {
		case "/drive/v3/changes/startPageToken":
			writeJSON(w, map[string]any{"startPageToken": "100"})
		case "/drive/v3/files":
			if q.Get("q") != "trashed = false" {
				t.Errorf("listed with q %q", q.Get("q"))
			}
			switch p := q.Get("pageToken"); p {
			case "":
				writeJSON(w, map[string]any{"files": []any{folder("a"), folder("b")}, "nextPageToken": "p2", "incompleteSearch": incomplete[""]})
			case "p2":
				writeJSON(w, map[string]any{"files": []any{folder("c")}, "incompleteSearch": incomplete["p2"]})
			default:
				t.Errorf("listed page %q", p)
				http.NotFound(w, r)
			}
		case "/drive/v3/changes":
			if q.Get("pageToken") != "100" || q.Get("includeRemoved") != "true" {
				t.Errorf("changes asked with %v", q)
			}
			writeJSON(w, map[string]any{"changes": changes, "newStartPageToken": "101"})
		default:
			t.Errorf("asked %s", r.URL.Path)
			http.NotFound(w, r)
		}
	})
}

// A first sync lists every file, a page at a time by nextPageToken, each page Full, and then reads the changes
// feed from the token it began at.
func TestDriveListingIsFull(t *testing.T) {
	f := driveFake(t, nil, nil)
	c := f.open(t, "gdrive", "", nil, nil)
	pages, cursor := pull(t, c, stream(t, "gdrive", "files"), "")
	if got, want := ids(pages...), []string{"a", "b", "c"}; !slices.Equal(got, want) {
		t.Errorf("listed %v, want %v", got, want)
	}
	if len(pages) != 2 {
		t.Errorf("%d pages, want 2", len(pages))
	}
	for i, p := range pages {
		if !p.Full {
			t.Errorf("page %d of a complete listing is not Full", i+1)
		}
	}
	if !strings.Contains(cursor, `"t":"100"`) || strings.Contains(cursor, `"l"`) {
		t.Errorf("after the listing the cursor is %s, want the changes from 100", cursor)
	}
}

// A listing Drive says may have left files out (incompleteSearch, as it does across many shared drives) is no
// listing of everything: its pages from that one on are not Full, so the page that ends it deletes nothing.
// Before #290 every listing page was Full, and the files Drive left out were deleted.
func TestDriveIncompleteListingDeletesNothing(t *testing.T) {
	f := driveFake(t, map[string]bool{"": true}, nil)
	c := f.open(t, "gdrive", "", nil, nil)
	pages, cursor := pull(t, c, stream(t, "gdrive", "files"), "")
	if got, want := ids(pages...), []string{"a", "b", "c"}; !slices.Equal(got, want) {
		t.Errorf("listed %v, want %v", got, want)
	}
	for i, p := range pages {
		if p.Full {
			t.Errorf("page %d of an incomplete listing is Full, so it deletes what Drive left out", i+1)
		}
	}
	if strings.Contains(cursor, "partial") {
		t.Errorf("after the listing the cursor is %s; the next listing should start whole", cursor)
	}
}

// After a listing the files stream reads Drive's changes: a changed file is a record, a removed or trashed one
// a deleted record, and the page deletes nothing else; the cursor moves on to the feed's new start.
func TestDriveChangesAfterAListing(t *testing.T) {
	f := driveFake(t, nil, []any{
		map[string]any{"fileId": "a", "file": map[string]any{"id": "a", "name": "Renamed", "mimeType": "application/vnd.google-apps.folder"}},
		map[string]any{"fileId": "b", "removed": true},
		map[string]any{"fileId": "c", "file": map[string]any{"id": "c", "mimeType": "application/vnd.google-apps.folder", "trashed": true}},
	})
	c := f.open(t, "gdrive", "", nil, nil)
	p, err := stream(t, "gdrive", "files").Pull(c, cursorFor(driveCursor{Token: "100"}))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := ids(p), []string{"-b", "-c", "a"}; !slices.Equal(got, want) {
		t.Errorf("changes %v, want %v", got, want)
	}
	if p.Full || p.More {
		t.Errorf("a changes page is Full %v, More %v; want neither", p.Full, p.More)
	}
	if !strings.Contains(p.Cursor, `"t":"101"`) {
		t.Errorf("after the changes the cursor is %s, want 101", p.Cursor)
	}
}
