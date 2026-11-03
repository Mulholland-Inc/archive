package connect

import (
	"net/http"
	"slices"
	"strings"
	"testing"
)

// A chat whose name and members stay as they were is still asked for its messages each sync: its
// lastUpdatedDateTime does not move when messages come. Before #290 a sync read only the chats whose
// lastUpdatedDateTime moved since, so a chat stopped syncing after the first.
func TestTeamsChatsAskedEverySync(t *testing.T) {
	var asked []string // the $filter each ask of the chat's messages had
	msgs := []any{map[string]any{"id": "m1", "messageType": "message", "lastModifiedDateTime": "2026-10-01T10:00:00Z",
		"createdDateTime": "2026-10-01T10:00:00Z", "body": map[string]any{"contentType": "text", "content": "hello"}}}
	f := newFake(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1.0/me/chats":
			writeJSON(w, map[string]any{"value": []any{map[string]any{"id": "c1", "topic": "Ops",
				"lastUpdatedDateTime": "2026-01-01T00:00:00Z"}}})
		case "/v1.0/me/chats/c1/messages":
			asked = append(asked, r.URL.Query().Get("$filter"))
			writeJSON(w, map[string]any{"value": msgs})
		default:
			t.Errorf("asked %s", r.URL.Path)
			http.NotFound(w, r)
		}
	})
	c := f.open(t, "teams", "", nil, nil)
	s := stream(t, "teams", "chats")
	pages, cursor := pull(t, c, s, "")
	if got, want := ids(pages...), []string{"c1/m1"}; !slices.Equal(got, want) {
		t.Errorf("first sync read %v, want %v", got, want)
	}
	msgs = append(msgs, map[string]any{"id": "m2", "messageType": "message", "lastModifiedDateTime": "2026-11-01T10:00:00Z",
		"createdDateTime": "2026-11-01T10:00:00Z", "body": map[string]any{"contentType": "text", "content": "again"}})
	msgs = msgs[1:]
	pages, _ = pull(t, c, s, cursor)
	if got, want := ids(pages...), []string{"c1/m2"}; !slices.Equal(got, want) {
		t.Errorf("second sync read %v, want %v", got, want)
	}
	if len(asked) != 2 {
		t.Fatalf("the chat's messages were asked %d times in two syncs, want 2", len(asked))
	}
	if asked[0] != "" {
		t.Errorf("first sync filtered by %q, want none", asked[0])
	}
	if !strings.HasPrefix(asked[1], "lastModifiedDateTime gt ") {
		t.Errorf("second sync filtered by %q, want what changed since the first", asked[1])
	}
}
