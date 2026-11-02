package connect

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

// Every stream is declared whole, with the table it lands in, and no connector has two of a name.
func TestStreamsAreDeclaredWhole(t *testing.T) {
	for _, c := range All() {
		names := map[string]bool{}
		for _, s := range c.Streams {
			if names[s.Name] || s.Name == "" || s.Doc == "" || s.Pull == nil || s.Table == "" || s.Rows == nil {
				t.Errorf("%s stream %q is not declared whole: %+v", c.ID, s.Name, s)
			}
			names[s.Name] = true
		}
	}
}

// An MCP list reads what changed since when its tool takes a time, paging with the cursor the answer names;
// lists whole when it does not; and is skipped when the server has no tool for it, or its tool answers in
// prose.
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
	list := func(tool string) *mcpList {
		return &mcpList{tool: tool, opts: map[string]any{"orderBy": "updatedAt", "includeArchived": true}, limit: "limit", max: 250,
			after: "after,cursor", since: "updatedAt", lastID: true}
	}
	issues := list("list_issues")
	p, err := issues.pull(c, "")
	if err != nil {
		t.Fatal(err)
	}
	wantPage(t, "the first page", p, "i1", true, true)
	if a := calls[0]; a["limit"] != 250.0 || a["orderBy"] != "updatedAt" || a["updatedAt"] != nil || a["includeArchived"] != nil {
		t.Errorf("the first call: %v", a)
	}
	p, _ = issues.pull(c, p.Cursor)
	wantPage(t, "the last page", p, "i2", false, true)
	if calls[1]["after"] != "c1" {
		t.Errorf("the second call: %v", calls[1])
	}
	p, _ = issues.pull(c, p.Cursor)
	if calls[2]["updatedAt"] == nil || calls[2]["after"] != nil || p.Full {
		t.Errorf("a delta: %v, full %v", calls[2], p.Full)
	}
	teams, _ := list("list_teams").pull(c, "")
	wantPage(t, "a tool without a time", teams, "t1", false, true)
	for tool, why := range map[string]string{"list_documents": "Linear's MCP server has no tool list_documents", "list_projects": "answers in prose"} {
		_, err := list(tool).pull(c, "")
		if u := (*Unsupported)(nil); !asUnsupported(err, &u) || !strings.Contains(u.Reason, why) {
			t.Errorf("%s: %v, want it skipped as %q", tool, err, why)
		}
	}
}

func asUnsupported(err error, u **Unsupported) bool {
	v, ok := err.(*Unsupported)
	*u = v
	return ok
}

// Asana asks for the scopes its webhooks need only once its variable says its app has them.
func TestAsanaAsksForWebhooksOnlyWhenItsAppHasThem(t *testing.T) {
	for opts, want := range map[string]bool{"": false, "?push=1": true} {
		u := provider(t, "asana", opts).AuthURL("https://api.example.com/cb", "st", "v")
		if got := strings.Contains(u, "webhooks%3Awrite"); got != want {
			t.Errorf("%q asks for webhooks:write: %v", opts, got)
		}
	}
}
