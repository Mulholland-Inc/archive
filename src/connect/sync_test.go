package connect

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/oauth2"
)

// standIn is a Client of API connector id on a server that h answers, as the provider would; seen are the
// requests it was sent, "METHOD path?query" and the body after a space when there is one.
func standIn(t *testing.T, id string, h func(w http.ResponseWriter, r *http.Request, body string)) (*Client, *[]string) {
	t.Helper()
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		seen = append(seen, strings.TrimSpace(r.Method+" "+r.URL.RequestURI()+" "+string(b)))
		w.Header().Set("Content-Type", "application/json")
		h(w, r, string(b))
	}))
	t.Cleanup(srv.Close)
	p, err := load(t, map[string]string{registry[id].Env: strings.Replace(srv.URL, "://", "://app:shh@", 1)}).Get(id)
	if err != nil {
		t.Fatal(err)
	}
	return p.Open(t.Context(), &oauth2.Token{AccessToken: "at"}, map[string]string{"realmId": "9130"}), &seen
}

// mcpStandIn is a Client of MCP connector id on an MCP server with tools, each answering what its handler
// does with the arguments it was called with.
func mcpStandIn(t *testing.T, id string, tools map[string]mcpTool) *Client {
	t.Helper()
	s := mcp.NewServer(&mcp.Implementation{Name: "standin", Version: "1"}, nil)
	for name, tool := range tools {
		s.AddTool(&mcp.Tool{Name: name, InputSchema: tool.schema}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var args map[string]any
			json.Unmarshal(req.Params.Arguments, &args)
			out := tool.answer(args)
			if s, ok := out.(string); ok {
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: s}}}, nil
			}
			b, _ := json.Marshal(out)
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}, StructuredContent: out}, nil
		})
	}
	srv := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s }, nil))
	t.Cleanup(srv.Close)
	p := &Provider{Connector: registry[id], server: srv.URL}
	c := p.Open(t.Context(), &oauth2.Token{AccessToken: "at"}, nil)
	t.Cleanup(c.Close)
	return c
}

type mcpTool struct {
	schema map[string]any
	answer func(args map[string]any) any
}

func schemaOf(props ...string) map[string]any {
	ps := map[string]any{}
	for _, p := range props {
		ps[p] = map[string]any{}
	}
	return map[string]any{"type": "object", "properties": ps}
}

// pull is the page after cursor of c's stream name.
func pull(t *testing.T, c *Client, name, cursor string) *Page {
	t.Helper()
	for _, s := range c.p.Streams {
		if s.Name == name {
			page, err := s.Pull(c, cursor)
			if err != nil {
				t.Fatalf("%s %s from %q: %v", c.p.ID, name, cursor, err)
			}
			return page
		}
	}
	t.Fatalf("%s has no stream %s", c.p.ID, name)
	return nil
}

// ids are the page's records, each id with a - when deleted.
func ids(p *Page) string {
	var out []string
	for _, r := range p.Records {
		if r.Deleted {
			out = append(out, "-"+r.ID)
		} else {
			out = append(out, r.ID)
		}
	}
	return strings.Join(out, " ")
}

func wantPage(t *testing.T, what string, p *Page, records string, more, full bool) {
	t.Helper()
	if ids(p) != records || p.More != more || p.Full != full {
		t.Errorf("%s: records %q more %v full %v, want %q %v %v (cursor %s)", what, ids(p), p.More, p.Full, records, more, full, p.Cursor)
	}
}

func write(w http.ResponseWriter, v any) { json.NewEncoder(w).Encode(v) }

func has(t *testing.T, seen []string, want ...string) {
	t.Helper()
	last := seen[len(seen)-1]
	for _, w := range want {
		if !strings.Contains(last, w) {
			t.Errorf("the last request %s has no %s", last, w)
		}
	}
}

// Every stream is declared whole, with a schedule, and no connector has two of a name.
func TestStreamsAreDeclaredWhole(t *testing.T) {
	for _, c := range All() {
		if len(c.Streams) == 0 {
			t.Errorf("%s syncs nothing", c.ID)
		}
		names := map[string]bool{}
		for _, s := range c.Streams {
			if names[s.Name] || s.Name == "" || s.Doc == "" || s.Pull == nil || s.Every < time.Minute {
				t.Errorf("%s stream %q is not declared whole: %+v", c.ID, s.Name, s)
			}
			names[s.Name] = true
		}
	}
}

// Drive lists every file the first time, from a changes token it took before, and then reads the changes
// from it, page by page, a removed or trashed file deleted.
func TestDriveListsThenReadsItsChanges(t *testing.T) {
	c, seen := standIn(t, "gdrive", func(w http.ResponseWriter, r *http.Request, _ string) {
		q := r.URL.Query()
		switch r.URL.Path + " " + q.Get("pageToken") {
		case "/drive/v3/changes/startPageToken ":
			write(w, map[string]string{"startPageToken": "s1"})
		case "/drive/v3/files ":
			write(w, map[string]any{"files": []any{map[string]any{"id": "f1", "modifiedTime": "2026-01-01T00:00:00Z"}}, "nextPageToken": "p2"})
		case "/drive/v3/files p2":
			write(w, map[string]any{"files": []any{map[string]any{"id": "f2"}}})
		case "/drive/v3/changes s1":
			write(w, map[string]any{"nextPageToken": "s2", "changes": []any{
				map[string]any{"changeType": "file", "fileId": "f1", "removed": true},
				map[string]any{"changeType": "file", "fileId": "f3", "file": map[string]any{"id": "f3", "trashed": true}},
				map[string]any{"changeType": "file", "fileId": "f4", "file": map[string]any{"id": "f4"}},
				map[string]any{"changeType": "drive", "driveId": "d1"}}})
		case "/drive/v3/changes s2":
			write(w, map[string]any{"changes": []any{}, "newStartPageToken": "s3"})
		default:
			w.WriteHeader(404)
		}
	})
	p := pull(t, c, "files", "")
	wantPage(t, "the first page", p, "f1", true, true)
	has(t, *seen, "q=trashed+%3D+false", "corpora=allDrives", "pageSize=1000")
	if !p.Records[0].Updated.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("a file's last change: %v", p.Records[0].Updated)
	}
	p = pull(t, c, "files", p.Cursor)
	wantPage(t, "the listing's last page", p, "f2", false, true)
	p = pull(t, c, "files", p.Cursor)
	wantPage(t, "the changes since", p, "-f1 -f3 f4", true, false)
	has(t, *seen, "includeRemoved=true")
	p = pull(t, c, "files", p.Cursor)
	wantPage(t, "caught up", p, "", false, false)
	if p.Cursor != `{"s":"s3"}` {
		t.Errorf("the next delta starts from %s", p.Cursor)
	}
}

// Outlook reads each folder's delta in turn, by folder and message, with immutable ids, keeps each folder's
// deltaLink for the next round, drops a folder gone, and reads a folder whose delta expired whole again.
func TestOutlookReadsEachFoldersDelta(t *testing.T) {
	round := 0
	c, seen := standIn(t, "outlook", func(w http.ResponseWriter, r *http.Request, _ string) {
		base := "http://" + r.Host + "/v1.0"
		q := r.URL.Query()
		switch {
		case r.URL.Path == "/v1.0/me/mailFolders/delta" && round == 0 && q.Get("$skiptoken") == "":
			write(w, map[string]any{"value": []any{map[string]any{"id": "inbox"}, map[string]any{"id": "x", "@removed": map[string]any{}}}, "@odata.nextLink": base + "/me/mailFolders/delta?$skiptoken=f2"})
		case r.URL.Path == "/v1.0/me/mailFolders/delta" && round == 0:
			write(w, map[string]any{"value": []any{map[string]any{"id": "sent"}}, "@odata.deltaLink": base + "/me/mailFolders/delta?$deltatoken=f"})
		case r.URL.Path == "/v1.0/me/mailFolders/delta":
			write(w, map[string]any{"value": []any{map[string]any{"id": "inbox"}}, "@odata.deltaLink": base + "/me/mailFolders/delta?$deltatoken=f"})
		case r.URL.Path == "/v1.0/me/mailFolders/inbox/messages/delta" && q.Get("$deltatoken") == "d1":
			w.WriteHeader(410)
			write(w, map[string]any{"error": map[string]string{"code": "SyncStateNotFound", "message": "expired"}})
		case r.URL.Path == "/v1.0/me/mailFolders/inbox/messages/delta" && q.Get("$skiptoken") == "":
			write(w, map[string]any{"value": []any{map[string]any{"id": "m1", "body": map[string]any{"content": strings.Repeat("x", maxText+10)}}},
				"@odata.nextLink": base + "/me/mailFolders/inbox/messages/delta?$skiptoken=s"})
		case r.URL.Path == "/v1.0/me/mailFolders/inbox/messages/delta":
			write(w, map[string]any{"value": []any{map[string]any{"id": "m2", "@removed": map[string]string{"reason": "deleted"}}},
				"@odata.deltaLink": base + "/me/mailFolders/inbox/messages/delta?$deltatoken=d1"})
		case r.URL.Path == "/v1.0/me/mailFolders/sent/messages/delta":
			write(w, map[string]any{"value": []any{map[string]any{"id": "m3"}}, "@odata.deltaLink": base + "/me/mailFolders/sent/messages/delta?$deltatoken=d2"})
		default:
			w.WriteHeader(404)
		}
	})
	p := pull(t, c, "messages", "")
	wantPage(t, "the inbox's first page", p, "inbox/m1", true, false)
	if b := p.Records[0].Data.(map[string]any)["body"].(map[string]any)["content"].(string); len(b) != maxText {
		t.Errorf("a body is kept whole: %d bytes", len(b))
	}
	if last := (*seen)[len(*seen)-1]; !strings.Contains(last, "$select=") {
		t.Errorf("the delta query: %s", last)
	}
	p = pull(t, c, "messages", p.Cursor)
	wantPage(t, "the inbox's last page", p, "-inbox/m2", true, false)
	p = pull(t, c, "messages", p.Cursor)
	wantPage(t, "the sent folder", p, "sent/m3", false, false)
	var cur outlookCursor
	json.Unmarshal([]byte(p.Cursor), &cur)
	if len(cur.Links) != 2 || !strings.HasSuffix(cur.Links["inbox"], "deltatoken=d1") || len(cur.Todo) != 0 {
		t.Fatalf("after a round: %s", p.Cursor)
	}
	round++
	p = pull(t, c, "messages", p.Cursor)
	wantPage(t, "an expired delta", p, "", true, false)
	cur = outlookCursor{}
	json.Unmarshal([]byte(p.Cursor), &cur)
	if len(cur.Links) != 0 || len(cur.Todo) != 1 {
		t.Errorf("an expired delta and a folder gone are forgotten: %s", p.Cursor)
	}
	p = pull(t, c, "messages", p.Cursor)
	wantPage(t, "the inbox read whole again", p, "inbox/m1", true, false)

	folders := pull(t, c, "folders", "")
	wantPage(t, "the folders", folders, "inbox", false, false)
	if !strings.HasSuffix(folders.Cursor, "deltatoken=f") {
		t.Errorf("the folders' next delta: %s", folders.Cursor)
	}
}

// QuickBooks lists an entity whole, 1,000 a page, then reads its change data capture from when the listing
// began, deletions too; a cursor older than change data capture reaches lists it whole again.
func TestQuickBooksListsThenReadsChangeDataCapture(t *testing.T) {
	c, seen := standIn(t, "quickbooks", func(w http.ResponseWriter, r *http.Request, _ string) {
		q := r.URL.Query()
		switch {
		case r.URL.Path == "/v3/company/9130/query" && strings.Contains(q.Get("query"), "STARTPOSITION 1 "):
			var rows []any
			for i := range 1000 {
				rows = append(rows, map[string]any{"Id": fmt.Sprint(i + 1)})
			}
			write(w, map[string]any{"QueryResponse": map[string]any{"Invoice": rows}})
		case r.URL.Path == "/v3/company/9130/query":
			write(w, map[string]any{"QueryResponse": map[string]any{"Invoice": []any{map[string]any{"Id": "1001"}}}})
		case r.URL.Path == "/v3/company/9130/cdc" && q.Get("entities") == "Invoice":
			write(w, map[string]any{"CDCResponse": []any{map[string]any{"QueryResponse": []any{map[string]any{"Invoice": []any{
				map[string]any{"Id": "5", "MetaData": map[string]any{"LastUpdatedTime": "2026-03-01T10:00:00-08:00"}},
				map[string]any{"Id": "6", "status": "Deleted"}}}}}}})
		default:
			w.WriteHeader(404)
		}
	})
	p := pull(t, c, "invoice", "")
	if len(p.Records) != 1000 || !p.More || !p.Full {
		t.Fatalf("the first page: %d records, more %v", len(p.Records), p.More)
	}
	has(t, *seen, plus("SELECT * FROM Invoice ORDERBY Id STARTPOSITION 1 MAXRESULTS 1000"), "minorversion=75")
	p = pull(t, c, "invoice", p.Cursor)
	wantPage(t, "the listing's last page", p, "1001", false, true)
	has(t, *seen, plus("STARTPOSITION 1001 "))
	p = pull(t, c, "invoice", p.Cursor)
	wantPage(t, "its changes since", p, "5 -6", false, false)
	has(t, *seen, "/cdc?entities=Invoice&changedSince=")
	old := cursorFor(qboCursor{Since: time.Now().Add(-40 * 24 * time.Hour).Format(time.RFC3339)})
	p = pull(t, c, "invoice", old)
	if len(p.Records) != 1000 || !p.Full {
		t.Errorf("a cursor older than 30 days lists it whole: %d records", len(p.Records))
	}
}

func plus(s string) string { return strings.ReplaceAll(strings.ReplaceAll(s, " ", "+"), "*", "%2A") }

// HubSpot searches by the last change, oldest first, from the latest it has read, and searches again from
// there before it pages 10,000 deep.
func TestHubSpotSearchesByTheLastChange(t *testing.T) {
	answers := []string{
		`{"results":[{"id":"1","updatedAt":"2026-01-01T00:00:00Z"},{"id":"2","updatedAt":"2026-01-02T00:00:00Z"}],"paging":{"next":{"after":"2"}}}`,
		`{"results":[{"id":"3","updatedAt":"2026-01-03T00:00:00Z"}]}`,
		`{"results":[{"id":"4","updatedAt":"2026-01-04T00:00:00Z"}],"paging":{"next":{"after":"9900"}}}`,
	}
	c, seen := standIn(t, "hubspot", func(w http.ResponseWriter, r *http.Request, _ string) {
		if r.URL.Path != "/crm/v3/objects/contacts/search" || len(answers) == 0 {
			w.WriteHeader(404)
			return
		}
		io.WriteString(w, answers[0])
		answers = answers[1:]
	})
	p := pull(t, c, "contacts", "")
	wantPage(t, "the first page", p, "1 2", true, false)
	if last := (*seen)[0]; strings.Contains(last, "filterGroups") || !strings.Contains(last, `"sorts":[{"direction":"ASCENDING","propertyName":"lastmodifieddate"}]`) {
		t.Errorf("the first search: %s", last)
	}
	p = pull(t, c, "contacts", p.Cursor)
	wantPage(t, "the last page", p, "3", false, false)
	has(t, *seen, `"after":"2"`)
	jan3 := fmt.Sprint(time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC).UnixMilli())
	p = pull(t, c, "contacts", p.Cursor)
	wantPage(t, "a delta", p, "4", true, false)
	has(t, *seen, `"operator":"GTE","propertyName":"lastmodifieddate","value":"`+jan3+`"`)
	var cur hubspotCursor
	json.Unmarshal([]byte(p.Cursor), &cur)
	if cur.After != "" || cur.Since != fmt.Sprint(time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC).UnixMilli()) {
		t.Errorf("a search 9,900 deep starts again from its last change: %s", p.Cursor)
	}
}

// GitHub reads issues by their last update, oldest first, each page from the last update the one before read,
// and pages on at the same time when a whole page shares it; repositories are listed whole.
func TestGitHubReadsIssuesByTheirLastUpdate(t *testing.T) {
	c, seen := standIn(t, "github", func(w http.ResponseWriter, r *http.Request, _ string) {
		q := r.URL.Query()
		issues := func(n int, at string, from int) {
			var out []any
			for i := range n {
				out = append(out, map[string]any{"id": from + i, "updated_at": at})
			}
			write(w, out)
		}
		switch {
		case r.URL.Path == "/issues" && q.Get("since") == "":
			issues(100, "2026-01-01T00:00:00Z", 0)
		case r.URL.Path == "/issues" && q.Get("page") == "1":
			issues(100, "2026-01-01T00:00:00Z", 100)
		case r.URL.Path == "/issues":
			issues(2, "2026-01-02T00:00:00Z", 200)
		case r.URL.Path == "/user/repos" && q.Get("page") == "1":
			issues(100, "", 0)
		case r.URL.Path == "/user/repos":
			issues(1, "", 100)
		default:
			w.WriteHeader(404)
		}
	})
	p := pull(t, c, "issues", "")
	has(t, *seen, "direction=asc", "filter=all", "state=all", "sort=updated")
	if len(p.Records) != 100 || !p.More || p.Cursor != `{"since":"2026-01-01T00:00:00Z"}` {
		t.Fatalf("the first page: %d %s", len(p.Records), p.Cursor)
	}
	p = pull(t, c, "issues", p.Cursor)
	if p.Cursor != `{"since":"2026-01-01T00:00:00Z","page":2}` {
		t.Fatalf("a whole page at one time pages on: %s", p.Cursor)
	}
	p = pull(t, c, "issues", p.Cursor)
	wantPage(t, "the last page", p, "200 201", false, false)
	has(t, *seen, "page=2", "since=2026-01-01T00%3A00%3A00Z")
	if p.Cursor != `{"since":"2026-01-02T00:00:00Z"}` {
		t.Errorf("the next delta: %s", p.Cursor)
	}
	r := pull(t, c, "repos", "")
	if len(r.Records) != 100 || !r.More || !r.Full || r.Cursor != "2" {
		t.Errorf("the repositories' first page: %d %s", len(r.Records), r.Cursor)
	}
	r = pull(t, c, "repos", r.Cursor)
	wantPage(t, "the repositories' last page", r, "100", false, true)
}

// Asana reads the tasks of every project and those assigned to you, a list after another, page by page; the
// next round reads those modified since this one began.
func TestAsanaReadsEachProjectsTasks(t *testing.T) {
	c, seen := standIn(t, "asana", func(w http.ResponseWriter, r *http.Request, _ string) {
		q := r.URL.Query()
		data := func(next string, gids ...string) {
			var d []any
			for _, g := range gids {
				d = append(d, map[string]any{"gid": g, "modified_at": "2026-01-01T00:00:00Z"})
			}
			out := map[string]any{"data": d, "next_page": nil}
			if next != "" {
				out["next_page"] = map[string]string{"offset": next}
			}
			write(w, out)
		}
		switch {
		case r.URL.Path == "/api/1.0/workspaces":
			data("", "w1")
		case r.URL.Path == "/api/1.0/projects" && q.Get("offset") == "":
			data("o1", "p1")
		case r.URL.Path == "/api/1.0/projects":
			data("", "p2")
		case r.URL.Path == "/api/1.0/tasks" && q.Get("project") == "p1" && q.Get("offset") == "":
			data("x", "t1")
		case r.URL.Path == "/api/1.0/tasks" && q.Get("project") == "p1":
			data("", "t2")
		case r.URL.Path == "/api/1.0/tasks" && q.Get("assignee") == "me":
			data("", "t1", "t3")
		case r.URL.Path == "/api/1.0/tasks":
			data("")
		default:
			w.WriteHeader(404)
		}
	})
	p := pull(t, c, "tasks", "")
	wantPage(t, "the first project's first page", p, "t1", true, false)
	for _, want := range []string{"t2", "", "t1 t3"} {
		p = pull(t, c, "tasks", p.Cursor)
		if ids(p) != want {
			t.Errorf("tasks %q, want %q", ids(p), want)
		}
	}
	if p.More || !strings.HasPrefix(p.Cursor, `{"since":"`) {
		t.Fatalf("after the last list: %v %s", p.More, p.Cursor)
	}
	has(t, *seen, "assignee=me", "workspace=w1")
	p = pull(t, c, "tasks", p.Cursor)
	has(t, *seen, "project=p1", "modified_since=")
	projects := pull(t, c, "projects", "")
	wantPage(t, "the projects of a workspace", projects, "p1 p2", false, true)
}

// An MCP stream reads what changed since when its tool takes a time, paging with the cursor the answer names;
// lists whole every relist when it does not; and is skipped when the server has no tool for it.
func TestMCPListsFollowTheToolsSchema(t *testing.T) {
	var calls []map[string]any
	c := mcpStandIn(t, "linear", map[string]mcpTool{
		"list_issues": {schemaOf("limit", "after", "updatedAt", "orderBy"), func(args map[string]any) any {
			calls = append(calls, args)
			if args["after"] == nil {
				return `{"issues":[{"id":"i1","updatedAt":"2026-01-01T00:00:00Z"}],"nextCursor":"c1"}`
			}
			return `{"issues":[{"id":"i2"}],"nextCursor":null}`
		}},
		"list_teams": {schemaOf("limit"), func(args map[string]any) any {
			return map[string]any{"teams": []any{map[string]any{"id": "t1", "name": "Core"}}}
		}},
		"list_projects": {schemaOf("limit"), func(args map[string]any) any { return "Here are your projects: Core." }},
	})
	p := pull(t, c, "issues", "")
	wantPage(t, "the first page", p, "i1", true, true)
	if a := calls[0]; a["limit"] != 250.0 || a["orderBy"] != "updatedAt" || a["updatedAt"] != nil || a["includeArchived"] != nil {
		t.Errorf("the first call: %v", a)
	}
	p = pull(t, c, "issues", p.Cursor)
	wantPage(t, "the last page", p, "i2", false, true)
	if calls[1]["after"] != "c1" {
		t.Errorf("the second call: %v", calls[1])
	}
	p = pull(t, c, "issues", p.Cursor)
	if calls[2]["updatedAt"] == nil || calls[2]["after"] != nil || p.Full {
		t.Errorf("a delta: %v, full %v", calls[2], p.Full)
	}
	teams := pull(t, c, "teams", "")
	wantPage(t, "a tool without a time", teams, "t1", false, true)
	if teams.Every != 6*time.Hour {
		t.Errorf("a tool without a time is listed whole every %v", teams.Every)
	}
	for name, why := range map[string]string{"documents": "Linear's MCP server has no tool list_documents", "projects": "answers in prose"} {
		_, err := streamNamed(c, name).Pull(c, "")
		if u := (*Unsupported)(nil); !asUnsupported(err, &u) || !strings.Contains(u.Reason, why) {
			t.Errorf("%s: %v, want it skipped as %q", name, err, why)
		}
	}
}

// Sentry lists every organization's projects, each organization's pages in turn.
func TestSentryListsEachOrganizationsProjects(t *testing.T) {
	c := mcpStandIn(t, "sentry", map[string]mcpTool{
		"find_organizations": {schemaOf("query", "cursor"), func(args map[string]any) any {
			return map[string]any{"organizations": []any{map[string]any{"slug": "acme", "regionUrl": "https://us.sentry.io"}, map[string]any{"slug": "beta"}}, "hasMore": false}
		}},
		"find_projects": {schemaOf("organizationSlug", "regionUrl", "cursor"), func(args map[string]any) any {
			switch {
			case args["organizationSlug"] == "acme" && args["cursor"] == nil:
				return map[string]any{"projects": []any{map[string]any{"slug": "web"}}, "hasMore": true, "nextCursor": "n1"}
			case args["organizationSlug"] == "acme" && args["regionUrl"] == "https://us.sentry.io":
				return map[string]any{"projects": []any{map[string]any{"slug": "api"}}, "hasMore": false}
			}
			return map[string]any{"projects": []any{}, "hasMore": false}
		}},
	})
	p := pull(t, c, "projects", "")
	wantPage(t, "the first page", p, "acme/web", true, true)
	p = pull(t, c, "projects", p.Cursor)
	wantPage(t, "the next page", p, "acme/api", true, true)
	p = pull(t, c, "projects", p.Cursor)
	wantPage(t, "the next organization's", p, "", false, true)
	orgs := pull(t, c, "organizations", "")
	wantPage(t, "the organizations", orgs, "acme beta", false, true)
}

// Stripe lists through stripe_api_read with the operation and its parameters, paging with starting_after
// while has_more, and is skipped when the tool takes no operation id.
func TestStripeListsThroughTheReadTool(t *testing.T) {
	var calls []map[string]any
	c := mcpStandIn(t, "stripe", map[string]mcpTool{
		"stripe_api_read": {schemaOf("stripe_api_operation_id", "parameters"), func(args map[string]any) any {
			calls = append(calls, args)
			if args["parameters"].(map[string]any)["starting_after"] == nil {
				return `{"object":"list","data":[{"id":"cus_1"},{"id":"cus_2"}],"has_more":true}`
			}
			return `{"object":"list","data":[{"id":"cus_3"}],"has_more":false}`
		}},
	})
	p := pull(t, c, "customers", "")
	wantPage(t, "the first page", p, "cus_1 cus_2", true, true)
	p = pull(t, c, "customers", p.Cursor)
	wantPage(t, "the last page", p, "cus_3", false, true)
	if b, _ := json.Marshal(calls[1]); string(b) != `{"parameters":{"limit":100,"starting_after":"cus_2"},"stripe_api_operation_id":"GetCustomers"}` {
		t.Errorf("the second call: %s", b)
	}
	other := mcpStandIn(t, "stripe", map[string]mcpTool{"stripe_api_read": {schemaOf("path"), func(map[string]any) any { return "{}" }}})
	if _, err := streamNamed(other, "customers").Pull(other, ""); !asUnsupported(err, new(*Unsupported)) {
		t.Errorf("a read tool of another shape: %v", err)
	}
}

func streamNamed(c *Client, name string) *Stream {
	for _, s := range c.p.Streams {
		if s.Name == name {
			return s
		}
	}
	return nil
}

func asUnsupported(err error, u **Unsupported) bool {
	v, ok := err.(*Unsupported)
	*u = v
	return ok
}
