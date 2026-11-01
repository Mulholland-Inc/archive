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
}

type runFile struct {
	ID        uuid.UUID      `json:"id"`
	Hash      string         `json:"sha256"`
	Name      string         `json:"name"`
	MIME      string         `json:"mime"`
	Size      int64          `json:"size"`
	Metadata  map[string]any `json:"metadata"`
	Status    string         `json:"status"`
	Error     string         `json:"error,omitempty"`
	Access    *string        `json:"access"`
	CreatedAt time.Time      `json:"created_at"`
	Text      string         `json:"text"`
}

type runTable struct {
	Name        string      `json:"name"`
	Kind        string      `json:"kind"`
	Description string      `json:"description"`
	Icon        string      `json:"icon,omitempty"`
	Layout      string      `json:"layout,omitempty"`
	By          string      `json:"by,omitempty"`
	Columns     []runColumn `json:"columns"`
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

func runCompile(code string) (string, error) {
	res := api.Transform("(async () => {\n"+code+"\n})()", api.TransformOptions{Loader: api.LoaderTS, Target: api.ES2020, Sourcefile: "script.ts"})
	if len(res.Errors) == 0 {
		return string(res.Code), nil
	}
	var msgs []string
	for _, m := range res.Errors {
		if l := m.Location; l != nil {
			msgs = append(msgs, fmt.Sprintf("line %d:%d: %s\n  %s", max(l.Line-1, 1), l.Column+1, m.Text, strings.TrimSpace(l.LineText)))
		} else {
			msgs = append(msgs, m.Text)
		}
	}
	return "", errors.New("compile error: " + strings.Join(msgs, "\n"))
}

func (a *Archive) Run(ctx context.Context, code string) (*RunOutput, error) {
	out, _, err := a.run(ctx, code, nil)
	return out, err
}

func (a *Archive) run(ctx context.Context, code string, input json.RawMessage) (*RunOutput, time.Time, error) {
	if ActorFrom(ctx) == nil {
		return nil, time.Time{}, fmt.Errorf("not signed in: %w", ErrUnauthorized)
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
	clock := &runClock{t: time.AfterFunc(limit, func() { cancel(context.DeadlineExceeded) }), end: time.Now().Add(limit), browse: browseTimeout}
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
		out.Result = json.RawMessage("null")
	}
	return out, r.at, nil
}

func (r *runner) nested(ctx context.Context, code string, input json.RawMessage, keep bool) (*RunOutput, error) {
	out := &RunOutput{Result: json.RawMessage("null"), Logs: []string{}}
	if r.depth >= workflowDepth {
		out.Error = fmt.Sprintf("workflows can test or start other workflows only %d deep; this one would be deeper", workflowDepth)
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
	if out.Error != "" || !keep {
		if out.Error != "" {
			out.Result = json.RawMessage("null")
		}
		return out, sp.Rollback(ctx)
	}
	if err := sp.Commit(ctx); err != nil {
		return nil, err
	}
	if t.after != nil {
		*t.after = append(*t.after, *after...)
	}
	return out, nil
}

func (r *runner) run(ctx context.Context) error {
	r.ctx = ctx
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

var runReads = map[string]bool{"schema": true, "revisions": true, "list": true, "get": true, "search": true, "tables": true, "files": true, "file": true, "workflows": true, "runs": true}

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

func (r *runner) sdk() {
	a, o := r.a, r.vm.NewObject()
	r.def(o, "sql", func(ctx context.Context, call goja.FunctionCall) (any, error) {
		q, err := r.str(call, 0, "sql", "query")
		if err != nil {
			return nil, err
		}
		params := make([]any, 0, len(call.Arguments))
		for i := 1; i < len(call.Arguments); i++ {
			var p any
			if err := r.arg(call, i, "sql", fmt.Sprintf("param $%d", i), &p); err != nil {
				return nil, err
			}
			params = append(params, runParam(p))
		}
		return a.runSQL(ctx, q, params)
	})
	r.def(o, "list", func(ctx context.Context, call goja.FunctionCall) (any, error) {
		table, err := r.str(call, 0, "list", "table")
		if err != nil {
			return nil, err
		}
		var opts struct {
			Where  map[string]any `json:"where"`
			Limit  int            `json:"limit"`
			After  string         `json:"after"`
			Reveal bool           `json:"reveal"`
		}
		if err := r.arg(call, 1, "list", "opts", &opts); err != nil {
			return nil, err
		}
		after, err := runAfter("list", opts.After)
		if err != nil {
			return nil, err
		}
		rs, err := a.List(ctx, table, opts.Where, opts.Limit, after)
		if err == nil && opts.Reveal {
			err = a.Reveal(ctx, table, rs...)
		}
		return rs, err
	})
	r.def(o, "get", func(ctx context.Context, call goja.FunctionCall) (any, error) {
		table, err := r.str(call, 0, "get", "table")
		if err != nil {
			return nil, err
		}
		id, err := r.id(call, 1, "get")
		if err != nil {
			return nil, err
		}
		var opts struct {
			Reveal bool `json:"reveal"`
		}
		if err := r.arg(call, 2, "get", "opts", &opts); err != nil {
			return nil, err
		}
		row, err := a.Get(ctx, table, id)
		if err == nil && opts.Reveal {
			err = a.Reveal(ctx, table, row)
		}
		return row, err
	})
	r.def(o, "create", func(ctx context.Context, call goja.FunctionCall) (any, error) {
		table, err := r.str(call, 0, "create", "table")
		if err != nil {
			return nil, err
		}
		var data map[string]any
		if path.Ext(table) != "" {
			var opts struct {
				Access string `json:"access"`
			}
			if err := r.arg(call, 1, "create", "opts", &opts); err != nil {
				return nil, err
			}
			return a.CreateDocument(ctx, opts.Access, table)
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
		table, err := r.str(call, 0, "update", "table")
		if err != nil {
			return nil, err
		}
		id, err := r.id(call, 1, "update")
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
		return a.Patch(ctx, table, id, patch)
	})
	r.def(o, "delete", func(ctx context.Context, call goja.FunctionCall) (any, error) {
		table, err := r.str(call, 0, "delete", "table")
		if err != nil {
			return nil, err
		}
		id, err := r.id(call, 1, "delete")
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
		return a.Schema(ctx)
	})
	r.def(o, "migrate", func(ctx context.Context, call goja.FunctionCall) (any, error) {
		sql, err := r.str(call, 0, "migrate", "sql")
		if err != nil {
			return nil, err
		}
		var opts struct {
			AllowDrop bool `json:"allowDrop"`
		}
		if err := r.arg(call, 1, "migrate", "opts", &opts); err != nil {
			return nil, err
		}
		return a.Migrate(ctx, sql, "", opts.AllowDrop)
	})
	r.def(o, "revisions", func(ctx context.Context, call goja.FunctionCall) (any, error) {
		var opts struct {
			Limit int `json:"limit"`
		}
		if err := r.arg(call, 0, "revisions", "opts", &opts); err != nil {
			return nil, err
		}
		return a.SchemaRevisions(ctx, opts.Limit)
	})
	r.def(o, "tables", func(ctx context.Context, call goja.FunctionCall) (any, error) {
		return runTables(a.catalog(ctx)), nil
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
		return coreDo(a, ctx, func(ctx context.Context) (*runFile, error) {
			r, err := a.st.db(ctx).FileWithText(ctx, id)
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, fmt.Errorf("file %s: %w", id, ErrNotFound)
			}
			return (*runFile)(&r), err
		})
	})
	r.def(o, "read", func(ctx context.Context, call goja.FunctionCall) (any, error) {
		id, err := r.id(call, 0, "read")
		if err != nil {
			return nil, err
		}
		return a.ReadDocument(ctx, id)
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
		return a.EditDocument(ctx, id, ops)
	})
	r.workflowSDK(o)
	r.browseSDK(o)
	r.vm.Set("archive", o)
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

func (a *Archive) runSQL(ctx context.Context, query string, params []any) ([]map[string]any, error) {
	q := strings.TrimRightFunc(strings.TrimSpace(query), func(r rune) bool { return r == ';' || unicode.IsSpace(r) })
	return coreDo(a, ctx, func(ctx context.Context) ([]map[string]any, error) {
		t, _ := ctx.Value(storeTxKey{}).(storeTx)
		sp, err := t.tx.Begin(ctx)
		if err != nil {
			return nil, err
		}
		rows, err := storeRows(context.WithValue(ctx, storeTxKey{}, storeTx{sp, t.role, t.after, t.sch}), a.st, runWrap+q+"\n) SELECT to_jsonb(q) FROM q", params...)
		if err == nil {
			return rows, sp.Commit(ctx)
		}
		sp.Rollback(ctx)
		var pe *pgconn.PgError
		if !errors.As(err, &pe) || pe.Code != "42601" && pe.Code != "0A000" {
			if pe != nil && pe.Position > int32(len([]rune(runWrap))) {
				pe.Position -= int32(len([]rune(runWrap)))
			} else if pe != nil {
				pe.Position = 0
			}
			return nil, &runQueryErr{err, q}
		}
		res, err := t.tx.Query(ctx, q, append([]any{pgx.QueryResultFormats{pgx.TextFormatCode}}, params...)...)
		if err != nil {
			return nil, &runQueryErr{err, q}
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
			return nil, &runQueryErr{err, q}
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
		t := runTable{Name: name, Kind: tb.Kind, Description: tb.Description, Icon: tb.Icon, Layout: tb.Layout, By: tb.By, Columns: []runColumn{}}
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
	a.refresh(ctx)
	return strings.ReplaceAll(runSDK, "{timeout}", runTimeout.String()) + "\n" + runTypes(a.Catalog())
}

var runStatic = []string{"Base", "Row", "Hit", "Migration", "Table", "StoredFile", "Tables", "FetchResponse", "RunResult", "BrowseResult", "Deployed", "WorkflowRun",
	"Money", "Timestamp", "Time", "Interval", "Address", "Name", "Postcode", "Geo", "Quantity"}

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

func runTypes(cat *Catalog) string {
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
	for _, name := range cat.names() {
		tables[name] = n.name("table."+name, name)
	}
	b.WriteString("\n/** Table name to row type. */\ninterface Tables {\n")
	for _, name := range cat.names() {
		fmt.Fprintf(&b, "    %s: %s;\n", runProp(name), tables[name])
	}
	b.WriteString("}\n")
	for _, name := range cat.names() {
		tb := cat.Tables[name]
		ro := ""
		if tb.Kind == "view" {
			ro = "View; read only"
		}
		b.WriteString("\n" + runDoc("", tb.Description, ro))
		fmt.Fprintf(&b, "interface %s extends Base {\n", tables[name])
		for _, c := range tb.Columns {
			if catalogSystem[c.Name] || c.Type == "tsvector" {
				continue
			}
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
				desc = "Group that sees the row, like hr; null is the whole workspace"
			}
			b.WriteString(runDoc("    ", desc, ref, runFormat(c.Schema, c.Type), gen))
			opt := "?"
			if c.required() {
				opt = ""
			}
			fmt.Fprintf(&b, "    %s%s: %s;\n", runProp(c.Name), opt, n.ts(c.Schema, c.Type, c.List))
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
