package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// The run tool's description is the SDK alone, the same bytes in every workspace, so all of them share one
// prompt cache entry; each workspace's tables come first in the system prompt, cached on their own, and the
// date and who it talks with after them.
func TestChatSharesTheSDKPromptAcrossWorkspaces(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var reqs []map[string]any
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var req map[string]any
		json.Unmarshal(raw, &req)
		mu.Lock()
		reqs = append(reqs, req)
		mu.Unlock()
		for _, s := range []string{`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hi"}}`, `{"type":"content_block_stop","index":0}`,
			`{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`} {
			fmt.Fprintf(w, "event: x\ndata: %s\n\n", s)
		}
	}))
	defer fake.Close()
	in := newInst(t, "ANTHROPIC_API_KEY=test", "ANTHROPIC_BASE_URL="+fake.URL)
	in.must("user", "add", "alice@example.com")
	in.push(map[string]string{"house.sql": e2eHouse})
	ws := decode[Workspace](t, in.must("workspace", "add", "Acme", "bob@acme.test")).ID
	in.push(map[string]string{"company.sql": e2eSchema["company.sql"]}, "-workspace", ws, "-as", "bob@acme.test")
	alice := decode[Token](t, in.must("token", "create")).Secret
	bob := decode[Token](t, in.must("-workspace", ws, "-as", "bob@acme.test", "token", "create")).Secret
	srv := in.serve()
	for _, tok := range []string{alice, bob} {
		if st, _ := e2eChat(context.Background(), t, srv.url+"/api", tok, map[string]any{"text": "hi"}, nil); st != 200 {
			t.Fatalf("chat: %d", st)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(reqs) != 2 {
		t.Fatalf("%d model requests", len(reqs))
	}
	tool := func(req map[string]any) string {
		b, _ := json.Marshal(req["tools"])
		return string(b)
	}
	if tool(reqs[0]) != tool(reqs[1]) {
		t.Errorf("the run tool differs between workspaces:\n%.300s\n%.300s", tool(reqs[0]), tool(reqs[1]))
	}
	if d := fmt.Sprint(reqs[0]["tools"].([]any)[0].(map[string]any)["description"]); !strings.Contains(d, "declare const archive") || strings.Contains(d, "interface House") || strings.Contains(d, "interface Company") {
		t.Errorf("the run tool's description has a workspace's tables: %.200s", d)
	}
	for i, want := range []struct{ has, not string }{{"interface House", "interface Company"}, {"interface Company", "interface House"}} {
		sys := reqs[i]["system"].([]any)
		first, second := sys[0].(map[string]any), sys[1].(map[string]any)
		if len(sys) != 2 || !strings.Contains(fmt.Sprint(first["text"]), want.has) || strings.Contains(fmt.Sprint(first["text"]), want.not) ||
			fmt.Sprint(first["cache_control"]) != "map[type:ephemeral]" || second["cache_control"] != nil || !strings.Contains(fmt.Sprint(second["text"]), "Today is") {
			t.Errorf("workspace %d's system prompt: %v", i+1, sys)
		}
		if fmt.Sprint(reqs[i]["cache_control"]) != "map[type:ephemeral]" || fmt.Sprint(reqs[i]["tools"].([]any)[0].(map[string]any)["cache_control"]) != "map[type:ephemeral]" {
			t.Errorf("workspace %d's breakpoints: %v %v", i+1, reqs[i]["cache_control"], reqs[i]["tools"])
		}
	}
}
