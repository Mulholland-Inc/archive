package main

// Actions: archive.action(name, input, opts) in a run, which does something with one of the caller's own
// connections. name is "<connector>.<action>". An API connector's actions are the Go funcs of its file in the
// connect package; an MCP connector's are its server's tools, called with tools/call, each named for its tool
// without the connector's own prefix (notion-search is notion.search). The caller's connection to the
// connector is the one used, or the one opts.connection names, by account or id, when they have several.
//
// actionTypes is the TypeScript the run tool's docs end with: an Actions interface of the actions of the
// caller's connections, with their input and output types from the actions' JSON Schemas, which
// archive.action's signature in sdk.d.ts reads, as archive.list reads Tables. A workspace keeps, between
// runs, each connection's access token while it lasts and each MCP connection's tools for an hour.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Mulholland-Inc/archive/connect"
	"github.com/Mulholland-Inc/archive/generated"
	"github.com/dop251/goja"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/oauth2"
)

const (
	actionToolsTTL = time.Hour
	actionFailTTL  = time.Minute
)

// archiveConns is what a workspace keeps of its connections between runs.
type archiveConns struct {
	mu     sync.Mutex
	access map[uuid.UUID]*oauth2.Token
	tools  map[uuid.UUID]*actionTools
}

// actionTools are an MCP connection's tools as its server last listed them, or why it could not.
type actionTools struct {
	at    time.Time
	tools []*mcp.Tool
	err   error
}

// forget drops what is kept of connection id, as when it is disconnected.
func (cs *archiveConns) forget(id uuid.UUID) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	delete(cs.access, id)
	delete(cs.tools, id)
}

// connectionAccess is an access token of the connection id, the one kept while it has a minute left, or a
// fresh one.
func (a *Archive) connectionAccess(ctx context.Context, id uuid.UUID) (*oauth2.Token, error) {
	a.conns.mu.Lock()
	tok := a.conns.access[id]
	a.conns.mu.Unlock()
	if tok != nil && (tok.Expiry.IsZero() || time.Until(tok.Expiry) > time.Minute) {
		return tok, nil
	}
	tok, err := a.connectionToken(ctx, id)
	if err != nil {
		return nil, err
	}
	a.conns.mu.Lock()
	if a.conns.access == nil {
		a.conns.access = map[uuid.UUID]*oauth2.Token{}
	}
	a.conns.access[id] = tok
	a.conns.mu.Unlock()
	return tok, nil
}

// memberConnections are the connections of member, by connector and account.
func (a *Archive) memberConnections(ctx context.Context, member uuid.UUID) ([]generated.ArchiveConnection, error) {
	var out []generated.ArchiveConnection
	err := a.st.System(ctx, func(ctx context.Context) (err error) {
		out, err = a.st.db(ctx).MemberConnections(ctx, member)
		return err
	})
	return out, err
}

// connClient is a client of the connection c.
func (a *Archive) connClient(ctx context.Context, c generated.ArchiveConnection) (*connect.Client, error) {
	p, err := a.hub.provider(ctx, c.Provider, false)
	if err != nil {
		return nil, err
	}
	tok, err := a.connectionAccess(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	return p.Open(ctx, tok, c.Meta), nil
}

// connTools are the tools of the MCP connection c, kept for an hour, and a failure to list them for a minute.
func (a *Archive) connTools(ctx context.Context, c generated.ArchiveConnection) ([]*mcp.Tool, error) {
	a.conns.mu.Lock()
	t := a.conns.tools[c.ID]
	a.conns.mu.Unlock()
	if t != nil && (t.err == nil && time.Since(t.at) < actionToolsTTL || t.err != nil && time.Since(t.at) < actionFailTTL) {
		return t.tools, t.err
	}
	t = &actionTools{at: time.Now()}
	cl, err := a.connClient(ctx, c)
	if err == nil {
		t.tools, err = cl.Tools()
		cl.Close()
	}
	t.err = err
	a.conns.mu.Lock()
	if a.conns.tools == nil {
		a.conns.tools = map[uuid.UUID]*actionTools{}
	}
	a.conns.tools[c.ID] = t
	a.conns.mu.Unlock()
	return t.tools, t.err
}

// actionName is the action of an MCP connector's tool: its name, without the connector's id before it.
func actionName(p *connect.Provider, tool string) string {
	for _, sep := range []string{"-", "_", "."} {
		if s, ok := strings.CutPrefix(tool, p.ID+sep); ok && s != "" {
			return s
		}
	}
	return tool
}

// actionError is a failed action, as the script reads it: its message, and what kind of failure, for its
// code.
type actionError struct {
	msg  string
	kind error
}

// errActionInvalid is the kind of an action asked for wrong, or that the service said was, as code invalid.
var errActionInvalid = errors.New("invalid")

func (e *actionError) Error() string { return e.msg }
func (e *actionError) Unwrap() error { return e.kind }

// Action does the action name with input on one of the caller's connections: the one pick names, by account
// or id, or their only one to its connector. clients are the connections' clients this run opened, which it
// closes when it ends.
func (a *Archive) Action(ctx context.Context, name string, input any, pick string, clients map[uuid.UUID]*connect.Client) (any, error) {
	ac, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	fail := func(kind error, format string, args ...any) error {
		return &actionError{msg: "archive.action " + name + ": " + fmt.Sprintf(format, args...), kind: kind}
	}
	id, act, ok := strings.Cut(name, ".")
	if !ok || act == "" {
		return nil, fail(errActionInvalid, `name is "<connector>.<action>", like "gmail.search"`)
	}
	p, err := a.hub.connect.Get(id)
	if err != nil {
		return nil, fail(ErrNotFound, "%v", err)
	}
	all, err := a.memberConnections(ctx, ac.member)
	if err != nil {
		return nil, err
	}
	var mine []generated.ArchiveConnection
	for _, c := range all {
		if c.Provider == id && (pick == "" || pick == c.Account || pick == c.ID.String()) {
			mine = append(mine, c)
		}
	}
	switch {
	case len(mine) == 0 && pick != "":
		return nil, fail(ErrNotFound, "you have no %s connection %q", p.Name, pick)
	case len(mine) == 0:
		return nil, fail(ErrNotFound, "you have not connected %s; connect it in Settings, Connectors", p.Name)
	case len(mine) > 1:
		accounts := make([]string, len(mine))
		for i, c := range mine {
			accounts[i] = strconv.Quote(c.Account)
		}
		return nil, fail(errActionInvalid, "you have %d %s connections, so pass { connection } with one of %s", len(mine), p.Name, strings.Join(accounts, ", "))
	}
	c := mine[0]
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	if string(raw) == "null" {
		raw = []byte("{}")
	}
	var in any
	json.Unmarshal(raw, &in)
	cl := clients[c.ID]
	if cl == nil {
		if cl, err = a.connClient(ctx, c); err != nil {
			return nil, actionFailed(fail, err)
		}
		clients[c.ID] = cl
	}
	if p.MCP == "" {
		do := p.Actions[act]
		if do == nil {
			return nil, fail(ErrNotFound, "%s has no action %s; it has %s", p.Name, act, strings.Join(actionNames(p), ", "))
		}
		if err := actionCheck(name, do.Input, in); err != nil {
			return nil, err
		}
		out, err := do.Do(cl, raw)
		if err != nil {
			return nil, actionFailed(fail, err)
		}
		return out, nil
	}
	tools, err := a.connTools(ctx, c)
	if err != nil {
		return nil, actionFailed(fail, err)
	}
	var tool *mcp.Tool
	var names []string
	for _, t := range tools {
		if actionName(p, t.Name) == act {
			tool = t
		}
		names = append(names, actionName(p, t.Name))
	}
	if tool == nil {
		return nil, fail(ErrNotFound, "%s has no action %s; it has %s", p.Name, act, strings.Join(names, ", "))
	}
	if s := actionSchema(tool.InputSchema); s != nil {
		if err := actionCheck(name, s, in); err != nil {
			return nil, err
		}
	}
	res, err := cl.Call(tool.Name, in)
	if err != nil {
		return nil, actionFailed(fail, err)
	}
	return actionResult(res), nil
}

// actionFailed is err as the action's failure, with the code its kind has: a provider's 404 is not_found, a
// refused grant is unauthorized, and so on.
func actionFailed(fail func(error, string, ...any) error, err error) error {
	var kind error
	var rf *connect.Refusal
	switch {
	case errors.As(err, &rf) && rf.Status == 401 || errors.Is(err, ErrUnauthorized):
		kind = ErrUnauthorized
	case errors.As(err, &rf) && rf.Status == 403:
		kind = ErrForbidden
	case errors.As(err, &rf) && rf.Status == 404 || errors.Is(err, ErrNotFound):
		kind = ErrNotFound
	case errors.As(err, &rf) && rf.Status == 409:
		kind = ErrConflict
	case errors.As(err, &rf) && rf.Status == 400:
		kind = errActionInvalid
	}
	return fail(kind, "%s", strings.TrimPrefix(err.Error(), "connection "))
}

// actionCheck checks input against the action's schema s, as an invalid error naming the action.
func actionCheck(name string, s *jsonschema.Schema, input any) error {
	rs, err := s.Resolve(nil)
	if err != nil {
		return nil
	}
	if err := rs.Validate(input); err != nil {
		msg := err.Error()
		if _, after, ok := strings.Cut(msg, "validating root: "); ok {
			msg = after
		}
		return &ValidationError{Type: "archive.action " + name + " input", Errors: []FieldError{{Message: msg}}}
	}
	return nil
}

// actionSchema is an MCP tool's schema, nil when it does not read as one.
func actionSchema(v any) *jsonschema.Schema {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var s jsonschema.Schema
	if json.Unmarshal(b, &s) != nil {
		return nil
	}
	return &s
}

// actionResult is what a tool's result is in the script: its structured content, or its text, parsed when
// it is JSON.
func actionResult(res *mcp.CallToolResult) any {
	if res.StructuredContent != nil {
		return res.StructuredContent
	}
	text := connect.Text(res)
	var v any
	if json.Unmarshal([]byte(text), &v) == nil {
		return v
	}
	return text
}

func actionNames(p *connect.Provider) []string {
	var out []string
	for n := range p.Actions {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func (r *runner) actionSDK(o *goja.Object) {
	r.def(o, "action", func(ctx context.Context, call goja.FunctionCall) (any, error) {
		name, err := r.str(call, 0, "action", "name")
		if err != nil {
			return nil, err
		}
		var input any
		if err := r.arg(call, 1, "action", "input", &input); err != nil {
			return nil, err
		}
		var opts struct {
			Connection string `json:"connection"`
		}
		if err := r.arg(call, 2, "action", "opts", &opts); err != nil {
			return nil, err
		}
		if r.clients == nil {
			r.clients = map[uuid.UUID]*connect.Client{}
		}
		return r.a.Action(ctx, name, input, opts.Connection, r.clients)
	})
}

// closeClients ends the sessions the run's actions opened.
func (r *runner) closeClients() {
	for _, c := range r.clients {
		c.Close()
	}
}

// actionTypes is the TypeScript of the actions of the caller's connections: the Actions interface, from
// action name to its input and output. A connector the caller has several connections to has its actions
// once, from the first whose server answers.
func (a *Archive) actionTypes(ctx context.Context) string {
	var b strings.Builder
	var conns []generated.ArchiveConnection
	if ac := ActorFrom(ctx); ac != nil {
		var err error
		if conns, err = a.memberConnections(ctx, ac.member); err != nil {
			fmt.Fprintf(os.Stderr, "archive: workspace %s: the actions of %s: %v\n", a.ws.ID, ac.Email, err)
		}
	}
	var yours []string
	for _, c := range conns {
		if p, err := a.hub.connector(c.Provider); err == nil {
			yours = append(yours, p.Name+" "+c.Account)
		}
	}
	intro := "Actions on the accounts you connected, by name, for archive.action (see Connected accounts in the header)"
	if len(yours) == 0 {
		intro += ". You have connected none"
	} else {
		intro += ": " + strings.Join(yours, ", ")
	}
	b.WriteString("\n" + runDoc("", intro) + "interface Actions {\n")
	done := map[string]bool{}
	for _, c := range conns {
		p, err := a.hub.connector(c.Provider)
		if err != nil || done[p.ID] {
			continue
		}
		if p.MCP == "" {
			done[p.ID] = true
			for _, n := range actionNames(p) {
				act := p.Actions[n]
				actionEntry(&b, p.ID+"."+n, act.Doc, act.Input, nil)
			}
			continue
		}
		tools, err := a.connTools(ctx, c)
		if err != nil {
			fmt.Fprintf(os.Stderr, "archive: workspace %s: the tools of %s %s: %v\n", a.ws.ID, p.Name, c.Account, err)
			continue
		}
		done[p.ID] = true
		slices.SortFunc(tools, func(x, y *mcp.Tool) int { return strings.Compare(x.Name, y.Name) })
		for _, t := range tools {
			doc, _, _ := strings.Cut(strings.TrimSpace(t.Description), "\n\n")
			if len(doc) > 600 {
				doc = doc[:600] + "..."
			}
			actionEntry(&b, p.ID+"."+actionName(p, t.Name), doc, t.InputSchema, t.OutputSchema)
		}
	}
	b.WriteString("}\n")
	return b.String()
}

func actionEntry(b *strings.Builder, name, doc string, input, output any) {
	b.WriteString(runDoc("    ", strings.Join(strings.Fields(doc), " ")))
	out := "any"
	if output != nil {
		out = schemaTS(output, "        ", 0)
	}
	fmt.Fprintf(b, "    %s: {\n        input: %s;\n        output: %s;\n    };\n", strconv.Quote(name), schemaTS(input, "        ", 0), out)
}

// schemaTS is the TypeScript type of the JSON Schema v, as far as one says: types, enums, unions, objects
// with their properties' descriptions, arrays and maps, and any for the rest, like a $ref.
func schemaTS(v any, indent string, depth int) string {
	var m map[string]any
	if b, err := json.Marshal(v); err != nil || json.Unmarshal(b, &m) != nil || m == nil || depth > 6 {
		return "any"
	}
	if e, ok := m["enum"].([]any); ok && len(e) > 0 {
		var parts []string
		for _, x := range e {
			j, _ := json.Marshal(x)
			parts = append(parts, string(j))
		}
		return strings.Join(parts, " | ")
	}
	if c, ok := m["const"]; ok {
		j, _ := json.Marshal(c)
		return string(j)
	}
	for _, k := range []string{"anyOf", "oneOf"} {
		if alts, ok := m[k].([]any); ok && len(alts) > 0 {
			var parts []string
			for _, alt := range alts {
				parts = append(parts, schemaTS(alt, indent, depth+1))
			}
			return strings.Join(parts, " | ")
		}
	}
	var types []string
	switch t := m["type"].(type) {
	case string:
		types = []string{t}
	case []any:
		for _, x := range t {
			if s, ok := x.(string); ok {
				types = append(types, s)
			}
		}
	}
	if len(types) == 0 && m["properties"] != nil {
		types = []string{"object"}
	}
	if len(types) == 0 {
		return "any"
	}
	var parts []string
	for _, t := range types {
		switch t {
		case "string", "boolean", "null":
			parts = append(parts, t)
		case "number", "integer":
			parts = append(parts, "number")
		case "array":
			item := schemaTS(m["items"], indent, depth+1)
			if strings.Contains(item, "|") {
				item = "(" + item + ")"
			}
			parts = append(parts, item+"[]")
		case "object":
			parts = append(parts, schemaObject(m, indent, depth))
		default:
			parts = append(parts, "any")
		}
	}
	return strings.Join(parts, " | ")
}

func schemaObject(m map[string]any, indent string, depth int) string {
	props, _ := m["properties"].(map[string]any)
	if len(props) == 0 {
		if ap, ok := m["additionalProperties"].(map[string]any); ok && len(ap) > 0 {
			return "Record<string, " + schemaTS(ap, indent, depth+1) + ">"
		}
		if m["additionalProperties"] == false {
			return "{}"
		}
		return "Record<string, any>"
	}
	var required []string
	if rs, ok := m["required"].([]any); ok {
		for _, r := range rs {
			if s, ok := r.(string); ok {
				required = append(required, s)
			}
		}
	}
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		ri, rj := slices.Contains(required, keys[i]), slices.Contains(required, keys[j])
		if ri != rj {
			return ri
		}
		return keys[i] < keys[j]
	})
	inner := indent + "    "
	var b strings.Builder
	b.WriteString("{\n")
	for _, k := range keys {
		p, _ := props[k].(map[string]any)
		desc, _ := p["description"].(string)
		b.WriteString(runDoc(inner, strings.Join(strings.Fields(desc), " ")))
		opt := "?"
		if slices.Contains(required, k) {
			opt = ""
		}
		fmt.Fprintf(&b, "%s%s%s: %s;\n", inner, runProp(k), opt, schemaTS(p, inner, depth+1))
	}
	b.WriteString(indent + "}")
	return b.String()
}
