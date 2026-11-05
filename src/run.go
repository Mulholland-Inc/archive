package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"path"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/Mulholland-Inc/archive/connect"
	"github.com/Mulholland-Inc/archive/generated"
	"github.com/dop251/goja"
	"github.com/evanw/esbuild/pkg/api"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

//go:embed sdk.d.ts
var runSDK string

const (
	runMaxResult = 256 << 10
	runMaxLogs   = 64 << 10
	runMaxFetch  = 5 << 20
	runTimeout   = 30 * time.Second
	runWrap      = "WITH q AS (\n"
)

type RunOutput struct {
	Result json.RawMessage `json:"result"`
	Logs   []string        `json:"logs"`
	Error  string          `json:"error"`
	Files  []runChange     `json:"files,omitempty"`
}

type runChange struct {
	ID     uuid.UUID `json:"id"`
	Name   string    `json:"name"`
	Action string    `json:"action"`
}

const runReadMax = 20

var runActions = map[string]int{"read": 0, "edited": 1, "created": 2}

func (o *RunOutput) change(c runChange) {
	i := slices.IndexFunc(o.Files, func(x runChange) bool { return x.ID == c.ID })
	if i < 0 {
		if c.Action != "read" || len(o.Files) < runReadMax {
			o.Files = append(o.Files, c)
		}
		return
	}
	if runActions[o.Files[i].Action] > runActions[c.Action] {
		c.Action = o.Files[i].Action
	}
	o.Files[i] = c
}

type runFile struct {
	ID         uuid.UUID      `json:"id"`
	Hash       string         `json:"sha256"`
	Name       string         `json:"name"`
	MIME       string         `json:"mime"`
	Size       int64          `json:"size"`
	Metadata   map[string]any `json:"metadata"`
	Status     string         `json:"status"`
	Error      string         `json:"error,omitempty"`
	Access     *uuid.UUID     `json:"access"`
	CreatedAt  time.Time      `json:"created_at"`
	Kind       string         `json:"kind"`
	Parent     *uuid.UUID     `json:"parent"`
	Connection *uuid.UUID     `json:"connection"`
	Stream     *string        `json:"stream,omitempty"`
	SourceID   *string        `json:"source_id,omitempty"`
	Version    *string        `json:"version,omitempty"`
	URL        string         `json:"url,omitempty"`
	Drive      *string        `json:"drive,omitempty"`
	OwnerName  *string        `json:"owner_name,omitempty"`
	EditorName *string        `json:"editor_name,omitempty"`
	MadeAt     *time.Time     `json:"made_at,omitempty"`
	EditedAt   *time.Time     `json:"edited_at,omitempty"`
	UpdatedAt  time.Time      `json:"updated_at"`
	Text       string         `json:"text"`
}

type runTable struct {
	Name        string      `json:"name"`
	Kind        string      `json:"kind"`
	Description string      `json:"description"`
	Layout      string      `json:"layout,omitempty"`
	By          string      `json:"by,omitempty"`
	Columns     []runColumn `json:"columns"`
	DDL         string      `json:"ddl"`
}

type runColumn struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Required    bool     `json:"required"`
	References  string   `json:"references,omitempty"`
	Values      []string `json:"values,omitempty"`
}

type runQueryErr struct {
	err error
	q   string
}

func (e *runQueryErr) Error() string { return e.err.Error() }
func (e *runQueryErr) Unwrap() error { return e.err }

type runner struct {
	a         *Archive
	vm        *goja.Runtime
	ctx       context.Context
	js        string
	out       *RunOutput
	input     json.RawMessage
	at        time.Time
	depth     int
	clock     *runClock
	logBytes  int
	clients   map[uuid.UUID]*connect.Client
	stringify goja.Callable
	parse     goja.Callable
}

var (
	errRunFailed = errors.New("script failed")
	runStackRe   = regexp.MustCompile(`script\.js:(\d+):\d+`)
)

func (a *Archive) refresh(ctx context.Context) {
	if rev, err := generated.New(a.st.sys).Revision(ctx); err == nil && rev != a.Catalog().stamp {
		a.reload(ctx)
	}
}

type runCompileErr struct {
	line, col int
	msg, src  string
}

func (e runCompileErr) text() string {
	if e.line == 0 {
		return e.msg
	}
	return fmt.Sprintf("line %d:%d: %s", e.line, e.col, e.msg)
}

func runTransform(code string) (string, []runCompileErr) {
	res := api.Transform("(async () => {\n"+code+"\n})()", api.TransformOptions{Loader: api.LoaderTS, Target: api.ES2020, Sourcefile: "script.ts"})
	var errs []runCompileErr
	for _, m := range res.Errors {
		e := runCompileErr{msg: m.Text}
		if l := m.Location; l != nil {
			e.line, e.col, e.src = max(l.Line-1, 1), l.Column+1, strings.TrimSpace(l.LineText)
		}
		errs = append(errs, e)
	}
	return string(res.Code), errs
}

func runCompile(code string) (string, error) {
	js, errs := runTransform(code)
	if len(errs) == 0 {
		return js, nil
	}
	var msgs []string
	for _, e := range errs {
		if e.line > 0 {
			msgs = append(msgs, e.text()+"\n  "+e.src)
		} else {
			msgs = append(msgs, e.msg)
		}
	}
	return "", errors.New("compile error: " + strings.Join(msgs, "\n"))
}

func (a *Archive) Run(ctx context.Context, code string) (*RunOutput, error) {
	out, _, err := a.run(ctx, code, nil)
	return out, err
}

func (a *Archive) run(ctx context.Context, code string, input json.RawMessage) (*RunOutput, time.Time, error) {
	if _, err := signedIn(ctx); err != nil {
		return nil, time.Time{}, err
	}
	free, err := a.slot(ctx)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer free()
	a.refresh(ctx)
	out := &RunOutput{Result: json.RawMessage("null"), Logs: []string{}}
	js, err := runCompile(code)
	if err != nil {
		out.Error = err.Error()
		return out, time.Now(), nil
	}
	limit := runTimeout
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	clock := &runClock{t: time.AfterFunc(limit, func() { cancel(context.DeadlineExceeded) }), end: time.Now().Add(limit), browse: browseTimeout, model: modelTimeout, do: doTimeout}
	defer clock.t.Stop()
	r := &runner{a: a, out: out, js: js, input: input, at: time.Now(), clock: clock}
	err = a.as(ctx, r.run)
	switch {
	case err != nil && errors.Is(context.Cause(ctx), context.DeadlineExceeded):
		out.Error = fmt.Sprintf("timed out after %s; nothing was saved", limit)
	case err != nil && out.Error == "":
		out.Error = runMessage(err)
	}
	if out.Error != "" {
		out.Result, out.Files = json.RawMessage("null"), nil
	}
	return out, r.at, nil
}

type runClock struct {
	t                 *time.Timer
	end               time.Time
	browse, model, do time.Duration
	paused            int
	left              time.Duration
}

func (c *runClock) pause(budget *time.Duration, limit time.Duration, what string) (resume func(), err error) {
	if *budget <= 0 {
		return nil, fmt.Errorf("this run already %s for %s, its limit", what, limit)
	}
	if c.paused == 0 {
		if !c.t.Stop() {
			return nil, errors.New("the run timed out")
		}
		c.left = time.Until(c.end)
	}
	c.paused++
	start := time.Now()
	return func() {
		*budget -= time.Since(start)
		if c.paused--; c.paused == 0 {
			c.end = time.Now().Add(c.left)
			c.t.Reset(c.left)
		}
	}, nil
}

func (r *runner) nested(ctx context.Context, code string, input json.RawMessage) (*RunOutput, error) {
	out := &RunOutput{Result: json.RawMessage("null"), Logs: []string{}}
	if r.depth >= workflowDepth {
		out.Error = fmt.Sprintf("workflows can run other workflows only %d deep; this one would be deeper", workflowDepth)
		return out, nil
	}
	js, err := runCompile(code)
	if err != nil {
		out.Error = err.Error()
		return out, nil
	}
	t, ok := ctx.Value(storeTxKey{}).(storeTx)
	if !ok {
		return nil, errors.New("no transaction")
	}
	sp, err := t.tx.Begin(ctx)
	if err != nil {
		return nil, err
	}
	after := &[]func(){}
	n := &runner{a: r.a, out: out, js: js, input: input, at: r.at, depth: r.depth + 1, clock: r.clock}
	err = n.run(context.WithValue(ctx, storeTxKey{}, storeTx{sp, t.role, after, t.sch}))
	switch {
	case err != nil && ctx.Err() != nil:
		out.Error = "timed out; nothing it did was saved"
	case err != nil && out.Error == "":
		out.Error = runMessage(err)
	}
	if out.Error != "" {
		out.Result, out.Files = json.RawMessage("null"), nil
		return out, sp.Rollback(ctx)
	}
	if err := sp.Commit(ctx); err != nil {
		return nil, err
	}
	for _, c := range out.Files {
		r.out.change(c)
	}
	if t.after != nil {
		*t.after = append(*t.after, *after...)
	}
	return out, nil
}

func (r *runner) run(ctx context.Context) error {
	r.ctx = ctx
	defer r.closeClients()
	if at, err := r.a.st.db(ctx).TxNow(ctx); err == nil {
		r.at = at
	}
	r.vm = goja.New()
	r.vm.SetMaxCallStackSize(10000)
	j := r.vm.Get("JSON").ToObject(r.vm)
	r.stringify, _ = goja.AssertFunction(j.Get("stringify"))
	r.parse, _ = goja.AssertFunction(j.Get("parse"))
	r.console()
	r.fetch()
	r.sdk()
	in := goja.Undefined()
	if len(r.input) > 0 {
		v, err := r.parse(goja.Undefined(), r.vm.ToValue(string(r.input)))
		if err != nil {
			r.out.Error = "input: " + err.Error()
			return errRunFailed
		}
		in = v
	}
	r.vm.Set("input", in)
	stop := context.AfterFunc(ctx, func() { r.vm.Interrupt(ctx.Err()) })
	defer stop()
	v, err := r.vm.RunScript("script.js", r.js)
	if err != nil {
		var ex *goja.Exception
		if errors.As(err, &ex) {
			r.out.Error = r.errText(ex.Value())
		} else {
			r.out.Error = err.Error()
		}
		return errRunFailed
	}
	p, _ := v.Export().(*goja.Promise)
	if p == nil {
		r.out.Error = "the script did not run as an async function"
		return errRunFailed
	}
	switch p.State() {
	case goja.PromiseStatePending:
		r.out.Error = "the script never finished: it awaits a promise that never settles (there are no timers)"
		return errRunFailed
	case goja.PromiseStateRejected:
		r.out.Error = r.errText(p.Result())
		return errRunFailed
	}
	res, err := r.json(p.Result())
	if err != nil {
		r.out.Error = "result: " + err.Error()
		return errRunFailed
	}
	if res == nil {
		res = []byte("null")
	}
	if len(res) > runMaxResult {
		r.out.Error = fmt.Sprintf("the result is %d KB of JSON, over the %d KB limit; return fewer rows or fields; nothing was saved", len(res)>>10, runMaxResult>>10)
		return errRunFailed
	}
	r.out.Result = res
	return nil
}

func (r *runner) json(v goja.Value) (b []byte, err error) {
	defer func() {
		if x := recover(); x != nil {
			if ex, ok := x.(*goja.Exception); ok {
				err = errors.New(r.errText(ex.Value()))
				return
			}
			panic(x)
		}
	}()
	s, err := r.stringify(goja.Undefined(), v)
	if err != nil {
		var ex *goja.Exception
		if errors.As(err, &ex) {
			return nil, errors.New(r.errText(ex.Value()))
		}
		return nil, err
	}
	if goja.IsUndefined(s) {
		return nil, nil
	}
	return []byte(s.String()), nil
}

func (r *runner) errText(v goja.Value) string {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return "the script threw " + fmt.Sprint(v)
	}
	o, ok := v.(*goja.Object)
	if !ok || o.ClassName() != "Error" {
		if b, err := r.json(v); err == nil && b != nil && ok {
			return "the script threw " + string(b)
		}
		return v.String()
	}
	msg := o.Get("message").String()
	if name := o.Get("name"); name != nil && name.String() != "Error" {
		msg = name.String() + ": " + msg
	}
	if st := o.Get("stack"); st != nil {
		if m := runStackRe.FindStringSubmatch(st.String()); m != nil {
			n, _ := strconv.Atoi(m[1])
			if lines := strings.Split(r.js, "\n"); n >= 1 && n <= len(lines) {
				msg += "\n  at: " + strings.TrimSpace(lines[n-1])
			}
		}
	}
	return msg
}

func (r *runner) console() {
	c := r.vm.NewObject()
	for name, prefix := range map[string]string{"log": "", "info": "", "debug": "", "warn": "warn: ", "error": "error: "} {
		c.Set(name, func(call goja.FunctionCall) goja.Value {
			parts := make([]string, len(call.Arguments))
			for i, v := range call.Arguments {
				parts[i] = r.text(v)
			}
			r.log(prefix + strings.Join(parts, " "))
			return goja.Undefined()
		})
	}
	r.vm.Set("console", c)
}

func (r *runner) fetch() {
	r.vm.Set("fetch", func(call goja.FunctionCall) goja.Value {
		p, resolve, reject := r.vm.NewPromise()
		if res, err := r.fetchDo(call); err != nil {
			reject(r.jsErr(err))
		} else {
			resolve(res)
		}
		return r.vm.ToValue(p)
	})
}

var runFetchPublic, runFetchLocal = runFetchClient(false), runFetchClient(true)

func runFetchClient(local bool) *http.Client {
	dial := &net.Dialer{Timeout: 10 * time.Second, Control: func(_, address string, _ syscall.RawConn) error {
		host, _, _ := net.SplitHostPort(address)
		ip := net.ParseIP(host)
		if ip == nil || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() || ip.Equal(net.ParseIP("fd00:ec2::254")) ||
			!local && (ip.IsLoopback() || ip.IsPrivate()) {
			return fmt.Errorf("%s is an internal address, which scripts cannot reach", host)
		}
		return nil
	}}
	return &http.Client{Transport: &http.Transport{DialContext: dial.DialContext, ForceAttemptHTTP2: true, TLSHandshakeTimeout: 10 * time.Second}}
}

func (r *runner) fetchDo(call goja.FunctionCall) (goja.Value, error) {
	u, ok := call.Argument(0).Export().(string)
	if !ok {
		return nil, errors.New(`fetch: the url must be a string, like fetch("https://example.com/api")`)
	}
	method, header := "GET", http.Header{}
	var body io.Reader
	if o, ok := call.Argument(1).(*goja.Object); ok {
		if m := o.Get("method"); m != nil && !goja.IsUndefined(m) && !goja.IsNull(m) {
			method = strings.ToUpper(m.String())
		}
		if h, ok := o.Get("headers").(*goja.Object); ok {
			for _, k := range h.Keys() {
				header.Set(k, h.Get(k).String())
			}
		}
		switch b := o.Get("body"); {
		case b == nil || goja.IsUndefined(b) || goja.IsNull(b):
		case b.ExportType() == reflect.TypeFor[string]():
			body = strings.NewReader(b.String())
		default:
			j, err := r.json(b)
			if err != nil {
				return nil, fmt.Errorf("fetch %s: body: %v", u, err)
			}
			body = bytes.NewReader(j)
			if header.Get("Content-Type") == "" {
				header.Set("Content-Type", "application/json")
			}
		}
	}
	req, err := http.NewRequestWithContext(r.ctx, method, u, body)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %v", u, err)
	}
	req.Header = header
	client := runFetchPublic
	if r.a.hub.localApp() {
		client = runFetchLocal
	}
	resp, err := client.Do(req)
	if err != nil {
		if r.ctx.Err() != nil {
			return nil, fmt.Errorf("fetch %s: the run timed out", u)
		}
		return nil, fmt.Errorf("fetch: %v", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, runMaxFetch+1))
	if err != nil {
		return nil, fmt.Errorf("fetch %s: reading the response: %v", u, err)
	}
	if len(b) > runMaxFetch {
		return nil, fmt.Errorf("fetch %s: the response is over %d MB", u, runMaxFetch>>20)
	}
	text := string(b)
	res, hs := r.vm.NewObject(), r.vm.NewObject()
	for k, v := range resp.Header {
		hs.Set(strings.ToLower(k), strings.Join(v, ", "))
	}
	hs.DefineDataProperty("get", r.vm.ToValue(func(name string) goja.Value {
		if v := resp.Header.Values(name); len(v) > 0 {
			return r.vm.ToValue(strings.Join(v, ", "))
		}
		return goja.Null()
	}), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_FALSE)
	res.Set("ok", resp.StatusCode >= 200 && resp.StatusCode < 300)
	res.Set("status", resp.StatusCode)
	res.Set("statusText", http.StatusText(resp.StatusCode))
	res.Set("url", resp.Request.URL.String())
	res.Set("headers", hs)
	res.Set("text", func(goja.FunctionCall) goja.Value {
		p, resolve, _ := r.vm.NewPromise()
		resolve(text)
		return r.vm.ToValue(p)
	})
	res.Set("json", func(goja.FunctionCall) goja.Value {
		p, resolve, reject := r.vm.NewPromise()
		if v, err := r.parse(goja.Undefined(), r.vm.ToValue(text)); err != nil {
			start := []rune(strings.TrimSpace(text))
			reject(r.jsErr(fmt.Errorf("fetch %s: the response (status %d) is not JSON: %q", u, resp.StatusCode, string(start[:min(len(start), 200)]))))
		} else {
			resolve(v)
		}
		return r.vm.ToValue(p)
	})
	return res, nil
}

func (r *runner) text(v goja.Value) string {
	if s, ok := v.Export().(string); ok {
		return s
	}
	if o, ok := v.(*goja.Object); ok && o.ClassName() == "Error" {
		return r.errText(v)
	}
	if b, err := r.json(v); err == nil && b != nil {
		return string(b)
	}
	return v.String()
}

func (r *runner) log(s string) {
	if r.logBytes > runMaxLogs {
		return
	}
	r.logBytes += len(s)
	if r.logBytes > runMaxLogs {
		s = fmt.Sprintf("[logs stop here: over %d KB]", runMaxLogs>>10)
	}
	r.out.Logs = append(r.out.Logs, s)
}

func (r *runner) toJS(v any) (goja.Value, error) {
	if v == nil {
		return goja.Undefined(), nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return r.parse(goja.Undefined(), r.vm.ToValue(string(b)))
}

func (r *runner) jsErr(err error) goja.Value {
	ctor, _ := r.vm.Get("Error").(*goja.Object)
	e, cerr := r.vm.New(ctor, r.vm.ToValue(runMessage(err)))
	if cerr != nil {
		return r.vm.ToValue(runMessage(err))
	}
	var ve *ValidationError
	var pe *pgconn.PgError
	code := ""
	switch {
	case errors.As(err, &ve):
		code = "invalid"
		if list, err := r.toJS(ve.Errors); err == nil {
			e.Set("errors", list)
		}
	case errors.Is(err, ErrNotFound):
		code = "not_found"
	case errors.Is(err, ErrForbidden):
		code = "forbidden"
	case errors.Is(err, ErrConflict):
		code = "conflict"
	case errors.Is(err, ErrUnauthorized):
		code = "unauthorized"
	case errors.Is(err, errActionInvalid):
		code = "invalid"
	case errors.As(err, &pe):
		code = pe.Code
	}
	if code != "" {
		e.Set("code", code)
	}
	return e
}

func runMessage(err error) string {
	var ve *ValidationError
	if errors.As(err, &ve) {
		var parts []string
		for _, e := range ve.Errors {
			if e.Path == "" {
				parts = append(parts, e.Message)
			} else {
				parts = append(parts, e.Path+": "+e.Message)
			}
		}
		if len(parts) == 0 {
			return "invalid " + ve.Type
		}
		return "invalid " + ve.Type + ": " + strings.Join(parts, "; ")
	}
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		s := pe.Message + " (SQLSTATE " + pe.Code + ")"
		if p, _, ok := strings.Cut(err.Error(), pe.Message); ok && strings.HasPrefix(p, "statement ") {
			s = strings.TrimSuffix(p, "ERROR: ") + s
		}
		if pe.Detail != "" {
			s += "; " + pe.Detail
		}
		if pe.Hint != "" {
			s += "; hint: " + pe.Hint
		}
		if pe.Code == "55P04" {
			s += "; a value added to an enum in a run can be used only after that run: migrate in one run, use the value in the next"
		}
		if pe.Code == "55P03" {
			s += "; another transaction holds a lock on the table; try again"
		}
		var qe *runQueryErr
		if errors.As(err, &qe) && pe.Position > 0 {
			if q := []rune(qe.q); int(pe.Position) <= len(q) {
				s += fmt.Sprintf("; at %q", strings.TrimRightFunc(string(q[pe.Position-1:min(len(q), int(pe.Position)+40)]), unicode.IsSpace))
			}
		}
		return s
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timed out"
	}
	return err.Error()
}

func (r *runner) arg(call goja.FunctionCall, i int, fn, name string, dst any) error {
	v := call.Argument(i)
	if goja.IsUndefined(v) || goja.IsNull(v) {
		return nil
	}
	b, err := r.json(v)
	if err != nil || b == nil {
		return fmt.Errorf("archive.%s: %s: not JSON: %v", fn, name, err)
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.UseNumber()
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return &ValidationError{Type: "archive." + fn + " " + name, Errors: []FieldError{{Message: strings.TrimPrefix(err.Error(), "json: ")}}}
	}
	return nil
}

func (r *runner) str(call goja.FunctionCall, i int, fn, name string) (string, error) {
	s, ok := call.Argument(i).Export().(string)
	if !ok {
		return "", fmt.Errorf("archive.%s: %s must be a string: %w", fn, name, httpErrBadRequest)
	}
	return s, nil
}

func (r *runner) id(call goja.FunctionCall, i int, fn string) (uuid.UUID, error) {
	s, err := r.str(call, i, fn, "id")
	if err != nil {
		return uuid.Nil, err
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, &ValidationError{Type: "archive." + fn, Errors: []FieldError{{Path: "id", Message: fmt.Sprintf("%q is not a uuid", s)}}}
	}
	return id, nil
}

func (r *runner) row(call goja.FunctionCall, fn string) (string, uuid.UUID, error) {
	table, err := r.str(call, 0, fn, "table")
	if err != nil {
		return "", uuid.Nil, err
	}
	id, err := r.id(call, 1, fn)
	return table, id, err
}

func runAfter(fn, s string) (uuid.UUID, error) {
	if s == "" {
		return uuid.Nil, nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, &ValidationError{Type: "archive." + fn, Errors: []FieldError{{Path: "after", Message: fmt.Sprintf("%q is not a uuid", s)}}}
	}
	return id, nil
}

func (r *runner) def(o *goja.Object, name string, f func(ctx context.Context, call goja.FunctionCall) (any, error)) {
	o.Set(name, func(call goja.FunctionCall) goja.Value {
		return r.promise(r.call(runReads[name], func(ctx context.Context) (any, error) { return f(ctx, call) }))
	})
}

func (r *runner) promise(v any, err error) goja.Value {
	p, resolve, reject := r.vm.NewPromise()
	var jv goja.Value
	if err == nil {
		jv, err = r.toJS(v)
	}
	if err != nil {
		reject(r.jsErr(err))
	} else {
		resolve(jv)
	}
	return r.vm.ToValue(p)
}

var runReads = map[string]bool{"email": true, "action": true, "query": true, "schema": true, "list": true, "get": true, "search": true, "files": true, "file": true}

func (r *runner) call(read bool, f func(ctx context.Context) (any, error)) (any, error) {
	t, ok := r.ctx.Value(storeTxKey{}).(storeTx)
	if !ok {
		return nil, errors.New("no transaction")
	}
	sp, err := t.tx.Begin(r.ctx)
	if err != nil {
		return nil, storeErr(err)
	}
	v, err := f(context.WithValue(r.ctx, storeTxKey{}, storeTx{sp, t.role, t.after, t.sch}))
	if err != nil || read {
		sp.Rollback(r.ctx)
		return v, storeErr(err)
	}
	return v, storeErr(sp.Commit(r.ctx))
}

func (r *runner) read(ctx context.Context, id uuid.UUID) {
	if f, err := r.a.GetFile(ctx, id); err == nil && f.Kind != "folder" {
		r.out.change(runChange{f.ID, f.Name, "read"})
	}
}

func (r *runner) noted(action string) func(*File, error) (any, error) {
	return func(f *File, err error) (any, error) {
		if err != nil {
			return nil, err
		}
		r.out.change(runChange{f.ID, f.Name, action})
		return f, nil
	}
}

func (r *runner) sdk() {
	a, o := r.a, r.vm.NewObject()
	r.def(o, "query", func(ctx context.Context, call goja.FunctionCall) (any, error) {
		q, err := r.str(call, 0, "query", "sql")
		if err != nil {
			return nil, err
		}
		params := make([]any, 0, len(call.Arguments))
		for i := 1; i < len(call.Arguments); i++ {
			var p any
			if err := r.arg(call, i, "query", fmt.Sprintf("param $%d", i), &p); err != nil {
				return nil, err
			}
			params = append(params, runParam(p))
		}
		return a.runQuery(ctx, q, params)
	})
	r.def(o, "list", func(ctx context.Context, call goja.FunctionCall) (any, error) {
		table, err := r.str(call, 0, "list", "table")
		if err != nil {
			return nil, err
		}
		var opts struct {
			Where map[string]any `json:"where"`
			Limit int            `json:"limit"`
			After string         `json:"after"`
		}
		if err := r.arg(call, 1, "list", "opts", &opts); err != nil {
			return nil, err
		}
		after, err := runAfter("list", opts.After)
		if err != nil {
			return nil, err
		}
		return a.List(ctx, table, opts.Where, opts.Limit, after)
	})
	r.def(o, "get", func(ctx context.Context, call goja.FunctionCall) (any, error) {
		table, id, err := r.row(call, "get")
		if err != nil {
			return nil, err
		}
		row, err := a.Get(ctx, table, id)
		if err == nil && table == "file" {
			r.read(ctx, id)
		}
		return row, err
	})
	r.def(o, "create", func(ctx context.Context, call goja.FunctionCall) (any, error) {
		table, err := r.str(call, 0, "create", "table")
		if err != nil {
			return nil, err
		}
		if table == "file" {
			var f struct {
				Kind   string     `json:"kind"`
				Name   string     `json:"name"`
				Parent *uuid.UUID `json:"parent"`
				Access string     `json:"access"`
			}
			if err := r.arg(call, 1, "create", "data", &f); err != nil {
				return nil, err
			}
			if f.Kind != "folder" {
				return nil, &ValidationError{Type: "file", Errors: []FieldError{{Path: "kind", Message: `the file table takes folders, { kind: "folder", name, parent, access }; archive.create("name.docx") makes a document`}}}
			}
			return a.AddFolder(ctx, f.Access, f.Name, f.Parent)
		}
		var data map[string]any
		if path.Ext(table) != "" {
			var opts struct {
				Access string `json:"access"`
			}
			if err := r.arg(call, 1, "create", "opts", &opts); err != nil {
				return nil, err
			}
			return r.noted("created")(a.CreateDocument(ctx, opts.Access, table))
		}
		if err := r.arg(call, 1, "create", "data", &data); err != nil {
			return nil, err
		}
		if data == nil {
			return nil, fmt.Errorf("archive.create: data must be an object: %w", httpErrBadRequest)
		}
		return a.Create(ctx, table, data)
	})
	r.def(o, "update", func(ctx context.Context, call goja.FunctionCall) (any, error) {
		table, id, err := r.row(call, "update")
		if err != nil {
			return nil, err
		}
		var patch map[string]any
		if err := r.arg(call, 2, "update", "patch", &patch); err != nil {
			return nil, err
		}
		if patch == nil {
			return nil, fmt.Errorf("archive.update: patch must be an object: %w", httpErrBadRequest)
		}
		if table == "file" {
			if access, ok := patch["access"]; ok && len(patch) == 1 {
				g, ok := access.(string)
				if !ok && access != nil {
					return nil, &ValidationError{Type: "file", Errors: []FieldError{{Path: "access", Message: "want a group's id or name, or null for everyone"}}}
				}
				return a.SetFileAccess(ctx, id, g)
			}
			parent, err := runFileParent(patch)
			if err != nil {
				return nil, err
			}
			return a.MoveFile(ctx, id, parent)
		}
		return a.Patch(ctx, table, id, patch)
	})
	r.def(o, "delete", func(ctx context.Context, call goja.FunctionCall) (any, error) {
		if call.Argument(0).String() == "file" {
			id, err := r.id(call, 1, "delete")
			if err != nil {
				return nil, err
			}
			if err := a.DeleteFile(ctx, id); err != nil {
				return nil, err
			}
			r.out.Files = slices.DeleteFunc(r.out.Files, func(c runChange) bool { return c.ID == id })
			return nil, nil
		}
		table, id, err := r.row(call, "delete")
		if err != nil {
			return nil, err
		}
		return nil, a.Delete(ctx, table, id)
	})
	r.def(o, "search", func(ctx context.Context, call goja.FunctionCall) (any, error) {
		text, err := r.str(call, 0, "search", "text")
		if err != nil {
			return nil, err
		}
		var opts struct {
			Type  string `json:"type"`
			From  string `json:"from"`
			To    string `json:"to"`
			Limit int    `json:"limit"`
		}
		if err := r.arg(call, 1, "search", "opts", &opts); err != nil {
			return nil, err
		}
		return a.Search(ctx, text, SearchFilter{Type: opts.Type, From: opts.From, To: opts.To, Limit: opts.Limit})
	})
	r.def(o, "schema", func(ctx context.Context, call goja.FunctionCall) (any, error) {
		name := ""
		if v := call.Argument(0); !goja.IsUndefined(v) && !goja.IsNull(v) {
			var err error
			if name, err = r.str(call, 0, "schema", "name"); err != nil {
				return nil, err
			}
		}
		return a.Schema(ctx, name)
	})
	r.def(o, "migrate", func(ctx context.Context, call goja.FunctionCall) (any, error) {
		sql, err := r.str(call, 0, "migrate", "sql")
		if err != nil {
			return nil, err
		}
		var opts struct {
			Confirm string `json:"confirm"`
		}
		if err := r.arg(call, 1, "migrate", "opts", &opts); err != nil {
			return nil, err
		}
		return a.Migrate(ctx, sql, "", opts.Confirm)
	})
	r.def(o, "files", func(ctx context.Context, call goja.FunctionCall) (any, error) {
		var opts struct {
			Limit int    `json:"limit"`
			After string `json:"after"`
		}
		if err := r.arg(call, 0, "files", "opts", &opts); err != nil {
			return nil, err
		}
		after, err := runAfter("files", opts.After)
		if err != nil {
			return nil, err
		}
		return a.runFiles(ctx, opts.Limit, after)
	})
	r.def(o, "file", func(ctx context.Context, call goja.FunctionCall) (any, error) {
		id, err := r.id(call, 0, "file")
		if err != nil {
			return nil, err
		}
		f, err := coreDo(a, ctx, func(ctx context.Context) (*runFile, error) {
			r, err := a.st.db(ctx).FileWithText(ctx, id)
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, fmt.Errorf("file %s: %w", id, ErrNotFound)
			}
			return (*runFile)(&r), err
		})
		if err == nil {
			r.read(ctx, id)
		}
		return f, err
	})
	r.def(o, "copy", func(ctx context.Context, call goja.FunctionCall) (any, error) {
		id, err := r.id(call, 0, "copy")
		if err != nil {
			return nil, err
		}
		var opts struct {
			Name   string  `json:"name"`
			Access *string `json:"access"`
		}
		if err := r.arg(call, 1, "copy", "opts", &opts); err != nil {
			return nil, err
		}
		return r.noted("created")(a.CopyFile(ctx, id, opts.Name, opts.Access))
	})
	r.def(o, "read", func(ctx context.Context, call goja.FunctionCall) (any, error) {
		id, err := r.id(call, 0, "read")
		if err != nil {
			return nil, err
		}
		m, err := a.ReadDocument(ctx, id)
		if err == nil {
			r.read(ctx, id)
		}
		return m, err
	})
	r.def(o, "edit", func(ctx context.Context, call goja.FunctionCall) (any, error) {
		id, err := r.id(call, 0, "edit")
		if err != nil {
			return nil, err
		}
		raw, err := r.json(call.Argument(1))
		if err != nil {
			return nil, err
		}
		ops, err := OfficeParseOps(raw)
		if err != nil {
			return nil, fmt.Errorf("archive.edit: %v: %w", err, httpErrBadRequest)
		}
		return r.noted("edited")(a.EditDocument(ctx, id, ops))
	})
	r.workflowSDK(o)
	r.browseSDK(o)
	r.modelSDK(o)
	r.doSDK(o)
	r.actionSDK(o)
	r.emailSDK(o)
	r.vm.Set("archive", o)
}

func runFileParent(patch map[string]any) (*uuid.UUID, error) {
	v, ok := patch["parent"]
	if !ok || len(patch) != 1 {
		return nil, &ValidationError{Type: "file", Errors: []FieldError{{Path: "parent", Message: "of a file, only { parent }, a folder's id or null, or { access }, a group or null, changes; archive.edit changes its content"}}}
	}
	if v == nil {
		return nil, nil
	}
	s, _ := v.(string)
	id, err := uuid.Parse(s)
	if err != nil {
		return nil, &ValidationError{Type: "file", Errors: []FieldError{{Path: "parent", Message: fmt.Sprintf("%v is not a folder's id", v)}}}
	}
	return &id, nil
}

func runParam(v any) any {
	switch x := v.(type) {
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return i
		}
		f, _ := x.Float64()
		return f
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = runParam(e)
		}
		return out
	}
	return v
}

func (a *Archive) runQuery(ctx context.Context, query string, params []any) ([]map[string]any, error) {
	q := strings.TrimRightFunc(strings.TrimSpace(query), func(r rune) bool { return r == ';' || unicode.IsSpace(r) })
	fail := func(err error) error {
		var pe *pgconn.PgError
		if errors.As(err, &pe) && pe.Code == "25006" {
			return fmt.Errorf("%s: archive.query only reads; change rows with archive.create, archive.update and archive.delete, and the schema with archive.migrate: %w", pe.Message, ErrForbidden)
		}
		return &runQueryErr{err, q}
	}
	return coreDo(a, ctx, func(ctx context.Context) ([]map[string]any, error) {
		t, _ := ctx.Value(storeTxKey{}).(storeTx)
		ro, err := t.tx.Begin(ctx)
		if err != nil {
			return nil, err
		}
		defer ro.Rollback(context.WithoutCancel(ctx))
		if err := generated.New(ro).SetLocal(ctx, "transaction_read_only", "on"); err != nil {
			return nil, err
		}
		sp, err := ro.Begin(ctx)
		if err != nil {
			return nil, err
		}
		rows, err := storeRows(context.WithValue(ctx, storeTxKey{}, storeTx{sp, t.role, t.after, t.sch}), a.st, runWrap+q+"\n) SELECT to_jsonb(q) FROM q", params...)
		if err == nil {
			return rows, nil
		}
		sp.Rollback(ctx)
		var pe *pgconn.PgError
		if !errors.As(err, &pe) || pe.Code != "42601" && pe.Code != "0A000" {
			if pe != nil && pe.Position > int32(len([]rune(runWrap))) {
				pe.Position -= int32(len([]rune(runWrap)))
			} else if pe != nil {
				pe.Position = 0
			}
			return nil, fail(err)
		}
		res, err := ro.Query(ctx, q, append([]any{pgx.QueryResultFormats{pgx.TextFormatCode}}, params...)...)
		if err != nil {
			return nil, fail(err)
		}
		defer res.Close()
		out := []map[string]any{}
		for res.Next() {
			m := map[string]any{}
			for i, fd := range res.FieldDescriptions() {
				if raw := res.RawValues()[i]; raw != nil {
					m[fd.Name] = string(raw)
				} else {
					m[fd.Name] = nil
				}
			}
			out = append(out, m)
		}
		if err := res.Err(); err != nil {
			return nil, fail(err)
		}
		return out, nil
	})
}

func (a *Archive) runFiles(ctx context.Context, limit int, after uuid.UUID) ([]*runFile, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	return coreDo(a, ctx, func(ctx context.Context) ([]*runFile, error) {
		rows, err := a.st.db(ctx).FilesWithText(ctx, after, int32(limit))
		fs := make([]*runFile, len(rows))
		for i := range rows {
			fs[i] = (*runFile)(&rows[i])
		}
		return fs, err
	})
}

func runTables(cat *Catalog) []runTable {
	out := []runTable{}
	for _, name := range cat.names() {
		tb := cat.Tables[name]
		t := runTable{Name: name, Kind: tb.Kind, Description: tb.Description, Layout: tb.Layout, By: tb.By, Columns: []runColumn{}}
		for _, c := range tb.Columns {
			if c.Type == "tsvector" {
				continue
			}
			col := runColumn{Name: c.Name, Type: c.Reg, Description: c.Description, Required: c.required() && !catalogSystem[c.Name], References: c.target()}
			if pt := cat.typ(c.Schema, c.Type); pt != nil && pt.Kind == "e" {
				col.Values = pt.Enum
			}
			t.Columns = append(t.Columns, col)
		}
		out = append(out, t)
	}
	return out
}

func (a *Archive) RunDocs(ctx context.Context) string {
	return runDocs + "\n" + a.RunTypes(ctx) + a.RunConnections(ctx)
}

var runDocs = strings.ReplaceAll(runSDK, "{timeout}", runTimeout.String())

func (a *Archive) RunTypes(ctx context.Context) string {
	a.refresh(ctx)
	cat := a.Catalog()
	return runTypesFor(cat, a.syncShown(ctx, cat))
}

func (a *Archive) RunConnections(ctx context.Context) string {
	return a.actionTypes(ctx) + a.streamTypes(ctx)
}

var runStatic = append(runSDKTypes(runSDK), "Actions", "Streams", "Tables", "Synced")

func runSDKTypes(src string) []string {
	var out []string
	for _, m := range regexp.MustCompile(`(?m)^(?:interface|type) (\w+)`).FindAllStringSubmatch(src, -1) {
		out = append(out, m[1])
	}
	return out
}

func runPascal(s string) string {
	var b strings.Builder
	for _, w := range strings.Split(s, "_") {
		if w != "" {
			b.WriteString(strings.ToUpper(w[:1]) + w[1:])
		}
	}
	return b.String()
}

type runNames struct {
	cat   *Catalog
	used  map[string]bool
	names map[string]string
}

func (n *runNames) name(key, base string) string {
	if s, ok := n.names[key]; ok {
		return s
	}
	s := runPascal(base)
	for n.used[s] {
		s += "Row"
	}
	n.used[s], n.names[key] = true, s
	return s
}

func (n *runNames) ts(schema, name string, list bool) string {
	s := "string"
	switch t := n.cat.typ(schema, name); {
	case t != nil && (t.Kind == "d" || t.Kind == "e" || t.Kind == "c"):
		s = n.names[schema+"."+name]
	case schema == "pg_catalog":
		switch n.cat.kind(schema, name) {
		case "number":
			s = "number"
		case "boolean":
			s = "boolean"
		case "":
			s = "unknown"
		}
	}
	if s == "" {
		s = "unknown"
	}
	if list {
		s += "[]"
	}
	return s
}

func runDoc(indent string, parts ...string) string {
	var keep []string
	for _, p := range parts {
		if p = strings.TrimSpace(strings.ReplaceAll(p, "*/", "* /")); p != "" {
			keep = append(keep, strings.TrimRight(p, "."))
		}
	}
	if len(keep) == 0 {
		return ""
	}
	return indent + "/** " + strings.Join(keep, ". ") + ". */\n"
}

func runFormat(schema, name string) string {
	if schema != "pg_catalog" {
		return ""
	}
	return map[string]string{"date": "YYYY-MM-DD", "timestamptz": "RFC 3339 UTC instant", "time": "HH:MM:SS", "timetz": "HH:MM:SS+zone",
		"timestamp": "YYYY-MM-DDTHH:MM:SS, no zone", "interval": "Postgres interval, like 1 day 02:00:00"}[name]
}

var runSyncedCols = []string{"access", "connection", "stream", "source_id"}

func runTypesFor(cat *Catalog, shown map[string]bool) string {
	names := slices.DeleteFunc(cat.names(), func(name string) bool { return !shown[name] })
	n := &runNames{cat: cat, used: map[string]bool{}, names: map[string]string{}}
	for _, s := range runStatic {
		n.used[s] = true
	}
	keys := specKeys(cat.Types)
	for _, k := range keys {
		t := cat.Types[k]
		if t.Schema == "archive" && t.Kind == "c" && slices.Contains(runStatic, runPascal(t.Name)) {
			n.names[k] = runPascal(t.Name)
		}
	}
	for _, schema := range []string{"archive", "public"} {
		for _, k := range keys {
			if t := cat.Types[k]; t.Schema == schema {
				n.name(k, t.Name)
			}
		}
	}
	var b strings.Builder
	decl := func(schema string) {
		for _, k := range keys {
			t := cat.Types[k]
			if t.Schema != schema || t.Schema == "archive" && t.Kind == "c" && slices.Contains(runStatic, n.names[k]) {
				continue
			}
			b.WriteString(runDoc("", t.Description))
			switch t.Kind {
			case "d":
				fmt.Fprintf(&b, "type %s = %s;\n", n.names[k], n.ts(t.BaseSchema, t.Base, false))
			case "e":
				vals := make([]string, len(t.Enum))
				for i, v := range t.Enum {
					vals[i] = strconv.Quote(v)
				}
				fmt.Fprintf(&b, "type %s = %s;\n", n.names[k], strings.Join(vals, " | "))
			case "c":
				fmt.Fprintf(&b, "interface %s {\n", n.names[k])
				for _, at := range t.Attrs {
					b.WriteString(runDoc("    ", at.Description, runFormat(at.Schema, at.Type)))
					fmt.Fprintf(&b, "    %s?: %s;\n", runProp(at.Name), n.ts(at.Schema, at.Type, at.List))
				}
				b.WriteString("}\n")
			}
		}
	}
	b.WriteString("\n/** Built-in types: writes clean messy input into these formats, then Postgres checks them. */\n")
	decl("archive")
	b.WriteString("\n/** Your types, from the live database. */\n")
	decl("public")
	tables := map[string]string{}
	for _, name := range names {
		tables[name] = n.name("table."+name, name)
	}
	b.WriteString("\n/** Table name to row type. */\ninterface Tables {\n")
	for _, name := range names {
		fmt.Fprintf(&b, "    %s: %s;\n", runProp(name), tables[name])
	}
	b.WriteString("}\n")
	field := func(tb *Table, c *Column) string {
		ref := ""
		if tg := c.target(); tg == "archive.file" {
			ref = "References a file: archive.file(id)"
		} else if tg != "" && !strings.HasPrefix(strings.ToLower(c.Description), "references") {
			ref = "References " + strings.TrimPrefix(tg, "public.")
		}
		gen := ""
		if c.Generated {
			gen = "Generated; read only"
		}
		desc := c.Description
		if c.Name == "access" && desc == "" && tb.Schema == "public" {
			desc = "The id of the group that sees the row, null for the whole workspace; a write may name the group instead, like hr"
		}
		if c.Name == "name" && desc == "" {
			desc = "The row's title, which the app shows it and every link to it by"
			if tb.Schema == "public" && tb.Kind == "table" {
				desc += "; set it on every create"
			}
		}
		opt := "?"
		if c.required() {
			opt = ""
		}
		ts := n.ts(c.Schema, c.Type, c.List)
		switch {
		case tb.isWorkflow() && c.Name == "on":
			ts = "WorkflowOn | null"
		case tb.Schema == "archive" && tb.Name == "workflow_run" && c.Name == "logs":
			ts = "string[]"
		case tb.Schema == "archive" && tb.Name == "workflow_run" && c.Name == "result":
			ts = "any"
		}
		return runDoc("    ", desc, ref, runFormat(c.Schema, c.Type), gen) + fmt.Sprintf("    %s%s: %s;\n", runProp(c.Name), opt, ts)
	}
	syncedFields := func(tb *Table) string {
		var out strings.Builder
		for _, name := range runSyncedCols {
			c := tb.col(name)
			if c == nil {
				return ""
			}
			out.WriteString(field(tb, c))
		}
		return out.String()
	}
	synced := ""
	if tb := cat.Tables["mail_message"]; tb != nil {
		synced = syncedFields(tb)
	}
	extends := func(tb *Table) bool { return synced != "" && tb.synced() && syncedFields(tb) == synced }
	if slices.ContainsFunc(names, func(name string) bool { return extends(cat.Tables[name]) }) {
		b.WriteString("\n/** What every table of a connection's rows has. */\ninterface Synced extends Base {\n" + synced + "}\n")
	}
	for _, name := range names {
		tb := cat.Tables[name]
		ro := ""
		if tb.Kind == "view" {
			ro = "View; read only"
		}
		base := "Base"
		if extends(tb) {
			base = "Synced"
		}
		b.WriteString("\n" + runDoc("", tb.Description, ro))
		fmt.Fprintf(&b, "interface %s extends %s {\n", tables[name], base)
		for _, c := range tb.Columns {
			if catalogSystem[c.Name] || c.Type == "tsvector" || base == "Synced" && slices.Contains(runSyncedCols, c.Name) {
				continue
			}
			b.WriteString(field(tb, c))
		}
		b.WriteString("}\n")
	}
	return b.String()
}

var runIdentRe = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

func runProp(s string) string {
	if runIdentRe.MatchString(s) {
		return s
	}
	return strconv.Quote(s)
}
