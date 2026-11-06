package main

import (
	"bytes"
	"cmp"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"reflect"
	"regexp"
	"runtime"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/Mulholland-Inc/archive/connect"
	"github.com/Mulholland-Inc/archive/generated"
	"github.com/buke/typescript-go-internal/v7/pkg/ast"
	"github.com/buke/typescript-go-internal/v7/pkg/astnav"
	"github.com/buke/typescript-go-internal/v7/pkg/bundled"
	tschecker "github.com/buke/typescript-go-internal/v7/pkg/checker"
	"github.com/buke/typescript-go-internal/v7/pkg/compiler"
	"github.com/buke/typescript-go-internal/v7/pkg/core"
	"github.com/buke/typescript-go-internal/v7/pkg/scanner"
	"github.com/buke/typescript-go-internal/v7/pkg/sourcemap"
	"github.com/buke/typescript-go-internal/v7/pkg/tsoptions"
	"github.com/buke/typescript-go-internal/v7/pkg/tspath"
	"github.com/buke/typescript-go-internal/v7/pkg/vfs/vfstest"
	"github.com/dop251/goja"
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

	runCheckCommand = "check"
	runCheckAtOnce  = 2
	runCheckErrs    = 30
	runCheckTime    = 5 * time.Second
	runCheckAllTime = 20 * time.Second
	runCheckStack   = 256 << 20
	runCheckMemory  = 512 << 20
	runTimeout      = 30 * time.Second
	runWrap         = "WITH q AS (\n"
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
	Hash       string         `json:"hash"`
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
	Icon        string      `json:"icon,omitempty"`
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

type runWroteKey struct{}

type runFreshKey struct{}

type runner struct {
	a         *Archive
	vm        *goja.Runtime
	ctx       context.Context
	src       string
	js        string
	lines     []int32
	out       *RunOutput
	input     json.RawMessage
	at        time.Time
	stamp     bool
	wrote     *bool
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

type checkFile struct {
	Source string
	Input  string
}

type checkIn struct {
	Files     []checkFile
	Types     string
	Check     bool
	Emit      bool
	Saved     bool
	Generated bool
}

type checkErr struct {
	File int
	Line int
	Col  int
	Msg  string
}

func (e checkErr) text() string {
	if e.Line == 0 {
		return e.Msg
	}
	return fmt.Sprintf("line %d:%d: %s", e.Line, e.Col, e.Msg)
}

type checkQuery struct {
	File int
	Line int
	Col  int
	SQL  string
}

type checkScan struct {
	Errs     []checkErr
	Queries  []checkQuery
	Actions  []string
	Migrates bool
}

type checkPlan struct {
	Types string
	More  string
}

type checkOut struct {
	Errs  []checkErr
	Types []string
	JS    string
	Lines []int32
}

var (
	errCheckLimit = errors.New("check limit")
	checkSlots    = make(chan struct{}, runCheckAtOnce)
	checkPragmaRe = regexp.MustCompile(`@ts-(ignore|expect-error|nocheck)\b`)
	checkAtRe     = regexp.MustCompile(`script\.js:(?: Line )?(\d+):(\d+)`)
	checkHints    = map[int32]string{
		7006:  "give it a type",
		7030:  "end every path with a return",
		7031:  "give it a type",
		7034:  "give it a type",
		7053:  "assert the object first, like (o as Row)[key], or use a Map",
		18046: "narrow it with typeof, in or Array.isArray, or assert it once with as",
		18047: "check for null first, or use ?. or ??",
		18048: "check for undefined first, or use ?. or ??",
		18049: "check first, or use ?. or ??",
		2531:  "check for null first, or use ?. or ??",
		2532:  "check for undefined first, or use ?. or ??",
		2533:  "check first, or use ?. or ??",
		2375:  "leave the property out rather than setting it to undefined",
		2379:  "leave the property out rather than setting it to undefined",
	}
)

func checkWrap(f checkFile) string {
	head := ""
	if f.Input != "" {
		head = "declare const input: " + f.Input + "; "
	}
	return head + "(async () => {\n" + f.Source + "\n})()"
}

func checkName(i int) string { return fmt.Sprintf("/s%d.ts", i) }

type checkHost struct {
	compiler.CompilerHost
	parsed map[string]*ast.SourceFile
}

func (h *checkHost) GetSourceFile(o ast.SourceFileParseOptions) *ast.SourceFile {
	if f := h.parsed[o.FileName]; f != nil {
		return f
	}
	f := h.CompilerHost.GetSourceFile(o)
	h.parsed[o.FileName] = f
	return f
}

type checkRun struct {
	in     checkIn
	ctx    context.Context
	parsed map[string]*ast.SourceFile
	p      *compiler.Program
	errs   []checkErr
	lines  []int
}

func (c *checkRun) program(types, more string) {
	vf := map[string]string{"/sdk.d.ts": types, "/more.d.ts": more}
	roots := []string{"/sdk.d.ts", "/more.d.ts"}
	for i, f := range c.in.Files {
		vf[checkName(i)] = checkWrap(f)
		roots = append(roots, checkName(i))
	}
	opts := &core.CompilerOptions{Strict: core.TSTrue, NoEmit: core.TSTrue, Target: core.ScriptTargetESNext, Lib: []string{"lib.esnext.d.ts"}, Types: []string{}, ModuleDetection: core.ModuleDetectionKindForce,
		NoUncheckedIndexedAccess: core.TSTrue, ExactOptionalPropertyTypes: core.TSTrue, NoImplicitReturns: core.TSTrue, NoFallthroughCasesInSwitch: core.TSTrue, SingleThreaded: core.TSTrue}
	cfg := tsoptions.NewParsedCommandLine(opts, roots, tspath.ComparePathsOptions{UseCaseSensitiveFileNames: true, CurrentDirectory: "/"})
	host := &checkHost{compiler.NewCompilerHost("/", bundled.WrapFS(vfstest.FromMap(vf, true)), bundled.LibPath(), nil, nil), c.parsed}
	c.p = compiler.NewProgram(compiler.ProgramOptions{Host: host, Config: cfg})
}

func (c *checkRun) add(i int, f *ast.SourceFile, pos int, msg string) {
	l, ch := scanner.GetECMALineAndUTF16CharacterOfPosition(f, pos)
	c.errs = append(c.errs, checkErr{File: i, Line: min(max(l, 1), c.lines[i]), Col: int(ch) + 1, Msg: msg})
}

func (c *checkRun) node(i int, f *ast.SourceFile, n *ast.Node, msg string) {
	c.add(i, f, scanner.GetTokenPosOfNode(n, f, false), msg)
}

func checkText(d *ast.Diagnostic) string {
	var b strings.Builder
	fmt.Fprintf(&b, "TS%d: %s", d.Code(), d.String())
	var chain func(list []*ast.Diagnostic, in string)
	chain = func(list []*ast.Diagnostic, in string) {
		for _, m := range list {
			b.WriteString("\n" + in + m.String())
			chain(m.MessageChain(), in+"  ")
		}
	}
	chain(d.MessageChain(), "    ")
	s := b.String()
	if len(s) > 800 {
		s = s[:800] + "..."
	}
	if h := checkHints[d.Code()]; h != "" {
		s = strings.TrimSuffix(s, ".") + "; " + h
	}
	return s
}

func (c *checkRun) scan() checkScan {
	var out checkScan
	c.program(c.in.Types, "")
	for i := range c.in.Files {
		f := c.p.GetSourceFile(checkName(i))
		for _, d := range c.p.GetSyntacticDiagnostics(c.ctx, f) {
			c.add(i, f, d.Pos(), checkText(d))
		}
	}
	if len(c.errs) > 0 {
		out.Errs = c.errs
		return out
	}
	ch, done := c.p.GetTypeChecker(c.ctx)
	defer done()
	literal := func(n *ast.Node) (string, bool) {
		t := ch.GetTypeAtLocation(n)
		if t.Flags()&tschecker.TypeFlagsStringLiteral == 0 {
			return "", false
		}
		s, ok := t.AsLiteralType().Value().(string)
		return s, ok
	}
	global := func(n *ast.Node) bool {
		if !ast.IsIdentifier(n) || n.Text() != "archive" {
			return false
		}
		s := ch.GetSymbolAtLocation(n)
		return s != nil && s.ValueDeclaration != nil && ast.GetSourceFileOfNode(s.ValueDeclaration).FileName() == "/sdk.d.ts"
	}
	for i := range c.in.Files {
		f := c.p.GetSourceFile(checkName(i))
		var visit func(n *ast.Node) bool
		visit = func(n *ast.Node) bool {
			if ast.IsPropertyAccessExpression(n) && global(n.Expression()) {
				name := n.Name().Text()
				if name == "migrate" {
					out.Migrates = true
				}
				if ast.IsCallExpression(n.Parent) && n.Parent.Expression() == n {
					if a := n.Parent.Arguments(); len(a) > 0 {
						s, ok := literal(a[0])
						switch {
						case name == "action" && ok:
							out.Actions = append(out.Actions, s)
						case name == "query" && ok:
							pos := scanner.GetTokenPosOfNode(a[0], f, false)
							l, col := scanner.GetECMALineAndUTF16CharacterOfPosition(f, pos)
							out.Queries = append(out.Queries, checkQuery{i, max(l, 1), int(col) + 1, s})
						case name == "query" && c.in.Saved:
							c.node(i, f, n.Parent, "a saved workflow needs constant SQL, so that it can be checked: write the whole statement as one string, one template without ${} or a const holding one, not joined with +, and pass values as $1, $2 params; to build SQL at run time, use a plain run, not a workflow")
						}
					}
				} else if c.in.Saved {
					c.node(i, f, n, "a saved workflow calls the SDK directly, like await archive.query(...), so that it can be checked; do not store its methods or pass them around")
				}
				return false
			}
			if c.in.Saved && global(n) {
				c.node(i, f, n, "a saved workflow calls the SDK directly, like await archive.query(...), so that it can be checked; do not store archive or pass it around")
			}
			n.ForEachChild(visit)
			return false
		}
		f.AsNode().ForEachChild(visit)
	}
	return out
}

func (c *checkRun) rules(ch *tschecker.Checker, i int, f *ast.SourceFile) {
	text := f.Text()
	for _, m := range checkPragmaRe.FindAllStringIndex(text, -1) {
		tok := astnav.GetTokenAtPosition(f, m[0])
		if tok == nil || m[0] < scanner.GetTokenPosOfNode(tok, f, false) {
			c.add(i, f, m[0], text[m[0]:m[1]]+" is not allowed, as it hides what the checker found: fix the error by narrowing the value, giving it a type, or asserting it once with as")
		}
	}
	var keywords func(n *ast.Node) bool
	keywords = func(n *ast.Node) bool {
		if n.Kind == ast.KindAnyKeyword {
			c.node(i, f, n, "any is not allowed: write the real type, or use unknown and narrow it with typeof, in or Array.isArray")
		}
		n.ForEachChild(keywords)
		return false
	}
	before := len(c.errs)
	f.AsNode().ForEachChild(keywords)
	keyworded := len(c.errs) > before
	reported := -1
	var visit func(n *ast.Node) bool
	visit = func(n *ast.Node) bool {
		switch {
		case ast.IsAssertionExpression(n):
			in := ast.SkipParentheses(n.Expression())
			if ast.IsAssertionExpression(in) && (in.Type().Kind == ast.KindUnknownKeyword || in.Type().Kind == ast.KindNeverKeyword) {
				c.node(i, f, n, "a double cast through unknown is not allowed: it claims a type the value may not have, so narrow the value with a check, or fix the type it comes from")
			}
		}
		n.ForEachChild(visit)
		if keyworded || !ast.IsExpressionNode(n) || n.Kind == ast.KindYieldExpression || reported >= n.Pos() {
			return false
		}
		up := n.Parent
		for up != nil && ast.IsParenthesizedExpression(up) {
			up = up.Parent
		}
		if up != nil && ast.IsAssertionExpression(up) {
			return false
		}
		if t := ch.GetTypeAtLocation(n); t.Flags()&tschecker.TypeFlagsAny != 0 && t.AsIntrinsicType().IntrinsicName() == "any" {
			reported = n.Pos()
			src := strings.Join(strings.Fields(text[scanner.GetTokenPosOfNode(n, f, false):n.End()]), " ")
			if len(src) > 60 {
				src = src[:60] + "..."
			}
			c.node(i, f, n, fmt.Sprintf("%s has type any, which turns off checking for everything it touches: give it a type, like new Map<string, number>() or (x: unknown) =>, or assert it once with as", src))
		}
		return false
	}
	f.AsNode().ForEachChild(visit)
}

func (c *checkRun) check(plan checkPlan) checkOut {
	types := c.in.Types
	if plan.Types != "" {
		types = plan.Types
		delete(c.parsed, "/sdk.d.ts")
	}
	if plan.Types != "" || plan.More != "" {
		delete(c.parsed, "/more.d.ts")
		c.program(types, plan.More)
	}
	var out checkOut
	for _, name := range []string{"/sdk.d.ts", "/more.d.ts"} {
		f := c.p.GetSourceFile(name)
		ds := c.p.GetSyntacticDiagnostics(c.ctx, f)
		if c.in.Generated {
			ds = append(ds, c.p.GetSemanticDiagnostics(c.ctx, f)...)
		}
		for _, d := range ds {
			l, col := scanner.GetECMALineAndUTF16CharacterOfPosition(f, d.Pos())
			out.Types = append(out.Types, fmt.Sprintf("%s:%d:%d %s", name, l+1, int(col)+1, checkText(d)))
		}
	}
	ch, done := c.p.GetTypeChecker(c.ctx)
	defer done()
	for i := range c.in.Files {
		f := c.p.GetSourceFile(checkName(i))
		for _, d := range c.p.GetSemanticDiagnostics(c.ctx, f) {
			c.add(i, f, d.Pos(), checkText(d))
		}
		c.rules(ch, i, f)
	}
	slices.SortStableFunc(c.errs, checkOrder)
	seen := map[int]int{}
	for _, e := range c.errs {
		if seen[e.File]++; seen[e.File] <= runCheckErrs {
			out.Errs = append(out.Errs, e)
		}
	}
	for file, n := range seen {
		if n > runCheckErrs {
			out.Errs = append(out.Errs, checkErr{File: file, Line: c.lines[file], Col: 1, Msg: fmt.Sprintf("and %d more errors; fix these first", n-runCheckErrs)})
		}
	}
	slices.SortStableFunc(out.Errs, checkOrder)
	return out
}

func checkOrder(a, b checkErr) int {
	return cmp.Or(cmp.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line), cmp.Compare(a.Col, b.Col))
}

func checkEmit(source string, out *checkOut) {
	vf := map[string]string{"/s.ts": checkWrap(checkFile{Source: source})}
	opts := &core.CompilerOptions{Target: core.ScriptTargetES2020, NoLib: core.TSTrue, NoCheck: core.TSTrue, Types: []string{}, SourceMap: core.TSTrue, RemoveComments: core.TSTrue, ModuleDetection: core.ModuleDetectionKindLegacy, SingleThreaded: core.TSTrue}
	cfg := tsoptions.NewParsedCommandLine(opts, []string{"/s.ts"}, tspath.ComparePathsOptions{UseCaseSensitiveFileNames: true, CurrentDirectory: "/"})
	host := compiler.NewCompilerHost("/", bundled.WrapFS(vfstest.FromMap(vf, true)), bundled.LibPath(), nil, nil)
	p := compiler.NewProgram(compiler.ProgramOptions{Host: host, Config: cfg})
	nlines := strings.Count(source, "\n") + 1
	f := p.GetSourceFile("/s.ts")
	for _, d := range p.GetSyntacticDiagnostics(context.Background(), f) {
		l, col := scanner.GetECMALineAndUTF16CharacterOfPosition(f, d.Pos())
		out.Errs = append(out.Errs, checkErr{Line: min(max(l, 1), nlines), Col: int(col) + 1, Msg: checkText(d)})
	}
	if len(out.Errs) > 0 {
		return
	}
	var sm string
	p.Emit(context.Background(), compiler.EmitOptions{WriteFile: func(name, text string, _ *compiler.WriteFileData) error {
		if strings.HasSuffix(name, ".map") {
			sm = text
		} else {
			out.JS = text
		}
		return nil
	}})
	var m struct {
		Mappings string `json:"mappings"`
	}
	json.Unmarshal([]byte(sm), &m)
	out.Lines = make([]int32, strings.Count(out.JS, "\n")+2)
	seen := -1
	for x := range sourcemap.DecodeMappings(m.Mappings).Values() {
		if x.GeneratedLine != seen && x.GeneratedLine < len(out.Lines) {
			seen = x.GeneratedLine
			out.Lines[seen] = int32(x.SourceLine)
		}
	}
	for i := 1; i < len(out.Lines); i++ {
		if out.Lines[i] == 0 {
			out.Lines[i] = out.Lines[i-1]
		}
	}
	if _, err := goja.Compile("script.js", out.JS, false); err != nil {
		e := checkErr{Msg: strings.ReplaceAll(err.Error(), "SyntaxError: SyntaxError: ", "")}
		if a := checkAtRe.FindStringSubmatch(e.Msg); a != nil {
			n, _ := strconv.Atoi(a[1])
			if n >= 1 && n <= len(out.Lines) {
				e.Line, e.Col = min(max(int(out.Lines[n-1]), 1), nlines), 1
			}
		}
		out.Errs, out.JS = append(out.Errs, e), ""
	}
}

func runCheckChild() error {
	debug.SetMaxStack(runCheckStack)
	go func() {
		var m runtime.MemStats
		for {
			if runtime.ReadMemStats(&m); m.Sys > runCheckMemory {
				os.Exit(1)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	dec, enc := json.NewDecoder(os.Stdin), json.NewEncoder(os.Stdout)
	var in checkIn
	if err := dec.Decode(&in); err != nil {
		return err
	}
	if !in.Check {
		var out checkOut
		checkEmit(in.Files[0].Source, &out)
		return enc.Encode(out)
	}
	c := &checkRun{in: in, ctx: context.Background(), parsed: map[string]*ast.SourceFile{}}
	for _, f := range in.Files {
		c.lines = append(c.lines, strings.Count(f.Source, "\n")+1)
	}
	scan := c.scan()
	if err := enc.Encode(scan); err != nil || len(scan.Errs) > 0 {
		return err
	}
	var plan checkPlan
	if err := dec.Decode(&plan); err != nil {
		return err
	}
	out := c.check(plan)
	if in.Emit && len(out.Errs) == 0 && len(out.Types) == 0 {
		checkEmit(in.Files[0].Source, &out)
	}
	return enc.Encode(out)
}

func checkApart(ctx context.Context, limit time.Duration, in checkIn, plan func(checkScan) (checkPlan, []checkErr, error)) (checkOut, error) {
	var out checkOut
	wait := time.NewTimer(limit)
	defer wait.Stop()
	select {
	case checkSlots <- struct{}{}:
		defer func() { <-checkSlots }()
	case <-wait.C:
		return out, fmt.Errorf("too many scripts are being checked; try again: %w", errCheckLimit)
	case <-ctx.Done():
		return out, ctx.Err()
	}
	self, err := os.Executable()
	if err != nil {
		return out, err
	}
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	cmd := exec.CommandContext(ctx, self, runCheckCommand)
	cmd.Env = []string{}
	cmd.WaitDelay = time.Second
	w, err := cmd.StdinPipe()
	if err != nil {
		return out, err
	}
	r, err := cmd.StdoutPipe()
	if err != nil {
		return out, err
	}
	if err := cmd.Start(); err != nil {
		return out, err
	}
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()
	gone := func() error {
		if ctx.Err() != nil {
			return fmt.Errorf("checking this took over %s, so it did not run: simplify its types and nesting: %w", limit, errCheckLimit)
		}
		return fmt.Errorf("checking this ran out of stack or memory, so it did not run: flatten its nesting and simplify its types: %w", errCheckLimit)
	}
	send := func(v any) {
		go func() {
			b, _ := json.Marshal(v)
			w.Write(b)
		}()
	}
	dec := json.NewDecoder(r)
	send(in)
	if !in.Check {
		if err := dec.Decode(&out); err != nil {
			return out, gone()
		}
		return out, nil
	}
	var scan checkScan
	if err := dec.Decode(&scan); err != nil {
		return out, gone()
	}
	if len(scan.Errs) > 0 {
		out.Errs = scan.Errs
		return out, nil
	}
	p, bad, err := plan(scan)
	if err != nil {
		return out, err
	}
	send(p)
	if err := dec.Decode(&out); err != nil {
		return out, gone()
	}
	if len(bad) > 0 {
		files := map[int]bool{}
		for _, e := range bad {
			files[e.File] = true
		}
		out.Errs = append(slices.DeleteFunc(out.Errs, func(e checkErr) bool { return files[e.File] }), bad...)
		slices.SortStableFunc(out.Errs, checkOrder)
	}
	return out, nil
}

type runDesc struct {
	ts  string
	err error
}

type runPgType struct {
	schema, name, typtype string
	list                  bool
}

var (
	runWriteRe     = regexp.MustCompile(`(?i)^\s*(INSERT|UPDATE|DELETE|MERGE|TRUNCATE|CREATE|ALTER|DROP)\b`)
	runMaybeNullRe = regexp.MustCompile(`(?i)\b(left|right|full|outer|grouping|rollup|cube)\b`)
)

func (n *runNames) pg(t runPgType) string {
	s := ""
	switch {
	case t.schema == "pg_catalog" && t.name == "oid":
		s = "number"
	case t.name == "record" || t.typtype == "p":
		s = "Row"
	case t.typtype == "c" && n.names["table."+t.name] != "":
		s = n.names["table."+t.name]
	case t.typtype == "c" && n.cat.typ(t.schema, t.name) == nil:
		s = "Row"
	default:
		return n.ts(t.schema, t.name, t.list)
	}
	if t.list {
		s += "[]"
	}
	return s
}

func runDescribe(ctx context.Context, tx pgx.Tx, n *runNames, queries []checkQuery) (map[string]runDesc, error) {
	out := map[string]runDesc{}
	types := map[uint32]runPgType{}
	notNull := map[[2]uint32]bool{}
	seen := map[uint32]bool{}
	prepare := func(sql string) (*pgconn.StatementDescription, error, error) {
		sp, err := tx.Begin(ctx)
		if err != nil {
			return nil, nil, err
		}
		sd, err := tx.Conn().PgConn().Prepare(ctx, "", sql, nil)
		if err != nil {
			var pe *pgconn.PgError
			if !errors.As(err, &pe) {
				return nil, nil, err
			}
			return nil, err, sp.Rollback(ctx)
		}
		return sd, nil, sp.Commit(ctx)
	}
	for _, query := range queries {
		q := query.SQL
		if _, ok := out[q]; ok {
			continue
		}
		if runWriteRe.MatchString(q) {
			out[q] = runDesc{err: errors.New("archive.query only reads; change rows with archive.create, archive.update and archive.delete, and the schema with archive.migrate")}
			continue
		}
		bare := strings.TrimRight(strings.TrimSpace(q), "; \t\r\n")
		sd, bad, err := prepare(bare)
		if err != nil {
			return nil, err
		}
		if bad != nil {
			out[q] = runDesc{err: &runQueryErr{bad, bare}}
			continue
		}
		_, bad, err = prepare(runWrap + bare + "\n) SELECT to_jsonb(q) FROM q")
		if err != nil {
			return nil, err
		}
		text := false
		var pe *pgconn.PgError
		if errors.As(bad, &pe) {
			if pe.Code != "42601" && pe.Code != "0A000" {
				if pe.Position > int32(len(runWrap)) {
					pe.Position -= int32(len(runWrap))
				} else {
					pe.Position = 0
				}
				out[q] = runDesc{err: &runQueryErr{bad, bare}}
				continue
			}
			text = true
		}
		var want, rels []uint32
		for _, f := range sd.Fields {
			if _, ok := types[f.DataTypeOID]; !ok {
				want = append(want, f.DataTypeOID)
			}
			if f.TableOID != 0 && !seen[f.TableOID] {
				seen[f.TableOID] = true
				rels = append(rels, f.TableOID)
			}
		}
		if len(want) > 0 {
			rows, _ := tx.Query(ctx, `SELECT t.oid, en.nspname::text, e.typname::text, e.typtype::text, t.typcategory = 'A' AND t.typelem <> 0
				FROM pg_catalog.pg_type t JOIN pg_catalog.pg_type e ON e.oid = CASE WHEN t.typcategory = 'A' AND t.typelem <> 0 THEN t.typelem ELSE t.oid END
				JOIN pg_catalog.pg_namespace en ON en.oid = e.typnamespace WHERE t.oid = ANY($1::oid[])`, want)
			var oid uint32
			var t runPgType
			if _, err := pgx.ForEachRow(rows, []any{&oid, &t.schema, &t.name, &t.typtype, &t.list}, func() error {
				types[oid] = t
				return nil
			}); err != nil {
				return nil, err
			}
		}
		if len(rels) > 0 {
			rows, _ := tx.Query(ctx, `SELECT attrelid::oid, attnum::int4 FROM pg_catalog.pg_attribute WHERE attrelid = ANY($1::oid[]) AND attnotnull AND attnum > 0`, rels)
			var rel, att uint32
			if _, err := pgx.ForEachRow(rows, []any{&rel, &att}, func() error {
				notNull[[2]uint32{rel, att}] = true
				return nil
			}); err != nil {
				return nil, err
			}
		}
		nullable := text || runMaybeNullRe.MatchString(bare)
		var b strings.Builder
		b.WriteString("{ ")
		done := map[string]bool{}
		for _, f := range sd.Fields {
			if done[f.Name] {
				continue
			}
			done[f.Name] = true
			ts := "string"
			if !text {
				ts = n.pg(types[f.DataTypeOID])
			}
			if ts != "unknown" && (nullable || !notNull[[2]uint32{f.TableOID, uint32(f.TableAttributeNumber)}]) {
				ts += " | null"
			}
			fmt.Fprintf(&b, "%s: %s; ", runProp(f.Name), ts)
		}
		b.WriteString("}")
		out[q] = runDesc{ts: b.String()}
	}
	return out, nil
}

func runQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func runSQLTypes(queries []checkQuery, desc map[string]runDesc) string {
	var b strings.Builder
	done := map[string]bool{}
	for _, q := range queries {
		if d := desc[q.SQL]; d.err == nil && !done[q.SQL] {
			done[q.SQL] = true
			fmt.Fprintf(&b, "interface ArchiveSDK {\n    query(sql: %s, ...params: unknown[]): Promise<%s[]>;\n}\n", runQuote(q.SQL), d.ts)
		}
	}
	return b.String()
}

func runSQLErrs(files []checkFile, queries []checkQuery, desc map[string]runDesc) []checkErr {
	var out []checkErr
	for _, q := range queries {
		err := desc[q.SQL].err
		if err == nil {
			continue
		}
		e := checkErr{File: q.File, Line: q.Line, Col: q.Col, Msg: runMessage(err)}
		var pe *pgconn.PgError
		if errors.As(err, &pe) && pe.Position > 0 {
			lead := len(q.SQL) - len(strings.TrimLeft(q.SQL, " \t\r\n"))
			runes := []rune(strings.TrimSpace(q.SQL))
			before := q.SQL[:lead] + string(runes[:min(int(pe.Position)-1, len(runes))])
			lines := strings.SplitAfter(files[q.File].Source, "\n")
			if q.Line <= len(lines) && q.Col <= len(lines[q.Line-1]) && strings.HasPrefix(strings.Join(lines[q.Line-1:], "")[q.Col:], before) {
				if nl := strings.Count(before, "\n"); nl > 0 {
					e.Line, e.Col = q.Line+nl, len(before)-strings.LastIndex(before, "\n")
				} else {
					e.Col += 1 + len(before)
				}
			}
		}
		out = append(out, e)
	}
	return out
}

func (a *Archive) checkOne(ctx context.Context, source, input string, saved bool) (checkOut, error) {
	cat := a.catalog(ctx)
	types, n := runTypesFor(cat, nil, false)
	conns := a.RunConnections(ctx)
	in := checkIn{Files: []checkFile{{source, input}}, Types: runDocs + "\n" + types + conns, Check: true, Emit: true, Saved: saved}
	out, err := checkApart(ctx, runCheckTime, in, func(scan checkScan) (checkPlan, []checkErr, error) {
		if scan.Migrates && !saved {
			open, _ := runTypesFor(cat, nil, true)
			return checkPlan{Types: runDocs + "\n" + open + conns}, nil, nil
		}
		if len(scan.Queries) == 0 {
			return checkPlan{}, nil, nil
		}
		var desc map[string]runDesc
		err := a.as(context.WithValue(ctx, storeReadKey{}, true), func(ctx context.Context) (err error) {
			t, _ := ctx.Value(storeTxKey{}).(storeTx)
			desc, err = runDescribe(ctx, t.tx, n, scan.Queries)
			return err
		})
		if err != nil {
			return checkPlan{}, nil, err
		}
		var bad []checkErr
		if saved {
			bad = runSQLErrs(in.Files, scan.Queries, desc)
		}
		return checkPlan{More: runSQLTypes(scan.Queries, desc)}, bad, nil
	})
	if err == nil && len(out.Types) > 0 {
		err = errors.New("the generated types do not check: " + strings.Join(out.Types, "; "))
	}
	return out, err
}

func checkLimit(err error) string {
	return strings.TrimSuffix(err.Error(), ": "+errCheckLimit.Error())
}

func runErrText(source string, errs []checkErr) string {
	lines := strings.Split(source, "\n")
	var msgs []string
	for _, e := range errs {
		s := e.text()
		if e.Line > 0 && e.Line <= len(lines) {
			s += "\n  " + strings.TrimSpace(lines[e.Line-1])
		}
		msgs = append(msgs, s)
	}
	return "compile error: " + strings.Join(msgs, "\n")
}

func (a *Archive) build(ctx context.Context, source string, saved bool) (string, []int32, string, error) {
	var out checkOut
	var err error
	if saved {
		out, err = checkApart(ctx, runCheckTime, checkIn{Files: []checkFile{{Source: source}}, Emit: true}, nil)
	} else {
		out, err = a.checkOne(ctx, source, "undefined", false)
	}
	if errors.Is(err, errCheckLimit) {
		return "", nil, "compile error: " + checkLimit(err), nil
	}
	if err != nil {
		return "", nil, "", err
	}
	if len(out.Errs) > 0 {
		return "", nil, runErrText(source, out.Errs), nil
	}
	return out.JS, out.Lines, "", nil
}

func (a *Archive) Run(ctx context.Context, code string) (*RunOutput, error) {
	out, _, err := a.run(ctx, code, nil, false)
	return out, err
}

func (a *Archive) run(ctx context.Context, code string, input json.RawMessage, saved bool) (*RunOutput, time.Time, error) {
	if _, err := signedIn(ctx); err != nil {
		return nil, time.Time{}, err
	}
	free, err := a.slot(ctx)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer free()
	if ctx.Value(runFreshKey{}) == nil {
		a.refresh(ctx)
	}
	out := &RunOutput{Result: json.RawMessage("null"), Logs: []string{}}
	js, lines, bad, err := a.build(ctx, code, saved)
	if err != nil {
		return nil, time.Time{}, err
	}
	if bad != "" {
		out.Error = bad
		return out, time.Now(), nil
	}
	limit := runTimeout
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	clock := &runClock{t: time.AfterFunc(limit, func() { cancel(context.DeadlineExceeded) }), end: time.Now().Add(limit), browse: browseTimeout, model: modelTimeout, do: doTimeout}
	defer clock.t.Stop()
	wrote := new(bool)
	r := &runner{a: a, out: out, src: code, js: js, lines: lines, input: input, at: time.Now(), stamp: saved, wrote: wrote, clock: clock}
	err = a.as(context.WithValue(ctx, runWroteKey{}, wrote), r.run)
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

func (r *runner) nested(ctx context.Context, code string, input json.RawMessage, saved bool) (*RunOutput, error) {
	out := &RunOutput{Result: json.RawMessage("null"), Logs: []string{}}
	if r.depth >= workflowDepth {
		out.Error = fmt.Sprintf("workflows can run other workflows only %d deep; this one would be deeper", workflowDepth)
		return out, nil
	}
	js, lines, bad, err := r.a.build(ctx, code, saved)
	if err != nil {
		return nil, err
	}
	if bad != "" {
		out.Error = bad
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
	n := &runner{a: r.a, out: out, src: code, js: js, lines: lines, input: input, at: r.at, wrote: r.wrote, depth: r.depth + 1, clock: r.clock}
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
	if r.stamp {
		if at, err := r.a.st.db(ctx).TxNow(ctx); err == nil {
			r.at = at
		}
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
			if lines := strings.Split(r.src, "\n"); n >= 1 && n <= len(r.lines) && r.lines[n-1] >= 1 && int(r.lines[n-1]) <= len(lines) {
				msg += fmt.Sprintf("\n  at: line %d: %s", r.lines[n-1], strings.TrimSpace(lines[r.lines[n-1]-1]))
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
		if !runReads[name] || name == "email" || name == "action" {
			*r.wrote = true
		}
		v, err := r.call(runReads[name], func(ctx context.Context) (any, error) { return f(ctx, call) })
		return r.promise(v, err)
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

var runReads = map[string]bool{"email": true, "action": true, "schema": true, "list": true, "get": true, "search": true, "files": true, "file": true}

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
	o.Set("query", func(call goja.FunctionCall) goja.Value {
		q, err := r.str(call, 0, "query", "sql")
		if err != nil {
			return r.promise(nil, err)
		}
		params := make([]any, 0, len(call.Arguments))
		for i := 1; i < len(call.Arguments); i++ {
			var p any
			if err := r.arg(call, i, "query", fmt.Sprintf("param $%d", i), &p); err != nil {
				return r.promise(nil, err)
			}
			params = append(params, runParam(p))
		}
		rows, err := a.runQuery(r.ctx, q, params)
		return r.promise(rows, storeErr(err))
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
			Desc  bool           `json:"desc"`
		}
		if err := r.arg(call, 1, "list", "opts", &opts); err != nil {
			return nil, err
		}
		after, err := runAfter("list", opts.After)
		if err != nil {
			return nil, err
		}
		return a.List(ctx, table, opts.Where, opts.Limit, after, opts.Desc)
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
		return a.Patch(ctx, table, id, patch)
	})
	r.def(o, "delete", func(ctx context.Context, call goja.FunctionCall) (any, error) {
		table, id, err := r.row(call, "delete")
		if err != nil {
			return nil, err
		}
		if err := a.Delete(ctx, table, id); err != nil {
			return nil, err
		}
		if table == "file" {
			r.out.Files = slices.DeleteFunc(r.out.Files, func(c runChange) bool { return c.ID == id })
		}
		return nil, nil
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
			Name    string `json:"name"`
			Breaks  bool   `json:"breaks"`
		}
		if err := r.arg(call, 1, "migrate", "opts", &opts); err != nil {
			return nil, err
		}
		return a.Migrate(ctx, sql, opts.Name, opts.Confirm, opts.Breaks)
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
		readOnly := func() (pgx.Tx, error) {
			ro, err := t.tx.Begin(ctx)
			if err != nil {
				return nil, err
			}
			return ro, generated.New(ro).SetLocal(ctx, "transaction_read_only", "on")
		}
		ro, err := readOnly()
		defer func() {
			if ro != nil {
				ro.Rollback(context.WithoutCancel(ctx))
			}
		}()
		if err != nil {
			return nil, err
		}
		rows, err := storeRows(context.WithValue(ctx, storeTxKey{}, storeTx{ro, t.role, t.after, t.sch}), a.st, runWrap+q+"\n) SELECT to_jsonb(q) FROM q", params...)
		if err == nil {
			return rows, nil
		}
		var pe *pgconn.PgError
		if !errors.As(err, &pe) || pe.Code != "42601" && pe.Code != "0A000" {
			if pe != nil && pe.Position > int32(len([]rune(runWrap))) {
				pe.Position -= int32(len([]rune(runWrap)))
			} else if pe != nil {
				pe.Position = 0
			}
			return nil, fail(err)
		}
		ro.Rollback(ctx)
		if ro, err = readOnly(); err != nil {
			return nil, err
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
		t := runTable{Name: name, Kind: tb.Kind, Description: tb.Description, Layout: tb.Layout, By: tb.By, Icon: tb.Icon, Columns: []runColumn{}}
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
	s, _ := runTypesFor(cat, a.syncShown(ctx, cat), false)
	return s
}

func (a *Archive) RunConnections(ctx context.Context) string {
	return a.actionTypes(ctx) + a.streamTypes(ctx)
}

var runStatic = append(append(runSDKTypes(runSDK), "Actions", "Streams", "Tables", "Creates", "Synced"), strings.Fields(runGlobals)...)

const runGlobals = `AggregateError Array ArrayBuffer ArrayBufferConstructor ArrayLike Atomics Awaited BigInt BigInt64Array BigUint64Array Boolean CallableFunction Capitalize ConstructorParameters DataView Date DisposableStack Error EvalError Exclude Extract FinalizationRegistry Float16Array Float32Array Float64Array Function Generator GeneratorFunction Int16Array Int32Array Int8Array Intl Iterable Iterator JSON Lowercase Map Math NewableFunction NoInfer NonNullable Number Object Omit OmitThisParameter Parameters Partial Pick Promise PromiseLike PropertyDescriptor PropertyKey Proxy RangeError Readonly ReadonlyArray ReadonlyMap ReadonlySet Record ReferenceError Reflect RegExp RegExpExecArray RegExpMatchArray Required ReturnType Set SharedArrayBuffer String Symbol SyntaxError TemplateStringsArray ThisParameterType ThisType TypeError URIError Uint16Array Uint32Array Uint8Array Uint8ClampedArray Uncapitalize Uppercase WeakKey WeakMap WeakRef WeakSet`

func runSDKTypes(src string) []string {
	var out []string
	for _, m := range regexp.MustCompile(`(?m)^(?:interface|type) (\w+)`).FindAllStringSubmatch(src, -1) {
		out = append(out, m[1])
	}
	return out
}

var runNotIdentRe = regexp.MustCompile(`[^A-Za-z0-9_$]+`)

func runPascal(s string) string {
	var b strings.Builder
	for _, w := range strings.Split(runNotIdentRe.ReplaceAllString(s, "_"), "_") {
		if w != "" {
			b.WriteString(strings.ToUpper(w[:1]) + w[1:])
		}
	}
	if b.Len() == 0 || b.String()[0] >= '0' && b.String()[0] <= '9' {
		return "T" + b.String()
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

func runTypesFor(cat *Catalog, shown map[string]bool, open bool) (string, *runNames) {
	names := slices.DeleteFunc(cat.names(), func(name string) bool { return shown != nil && !shown[name] })
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
					vals[i] = runQuote(v)
				}
				fmt.Fprintf(&b, "type %s = %s;\n", n.names[k], strings.Join(vals, " | "))
			case "c":
				fmt.Fprintf(&b, "interface %s {\n", n.names[k])
				for _, at := range t.Attrs {
					b.WriteString(runDoc("    ", at.Description, runFormat(at.Schema, at.Type)))
					fmt.Fprintf(&b, "    %s?: %s | null;\n", runProp(at.Name), n.ts(at.Schema, at.Type, at.List))
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
	if open {
		b.WriteString("    [table: string]: Row;\n")
	} else {
		for _, name := range names {
			fmt.Fprintf(&b, "    %s: %s;\n", runProp(name), tables[name])
		}
	}
	b.WriteString("}\n\n/** Table name to the columns a create must set. */\ninterface Creates {\n")
	if open {
		b.WriteString("    [table: string]: never;\n")
	} else {
		for _, name := range names {
			must := []string{}
			if tb := cat.Tables[name]; tb.Schema == "public" {
				for _, c := range tb.Columns {
					if c.writable() && c.required() {
						must = append(must, runQuote(c.Name))
					}
				}
			}
			if len(must) == 0 {
				must = append(must, "never")
			}
			fmt.Fprintf(&b, "    %s: %s;\n", runProp(name), strings.Join(must, " | "))
		}
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
		ts := n.ts(c.Schema, c.Type, c.List)
		switch {
		case tb.isWorkflow() && c.Name == "on":
			ts = "WorkflowOn"
		case tb.Schema == "archive" && tb.Name == "workflow_run" && c.Name == "logs":
			ts = "string[]"
		case tb.Schema == "archive" && tb.Name == "workflow_run" && c.Name == "result":
			ts = "unknown"
		}
		if !c.NotNull && ts != "unknown" {
			ts += " | null"
		}
		return runDoc("    ", desc, ref, runFormat(c.Schema, c.Type), gen) + fmt.Sprintf("    %s: %s;\n", runProp(c.Name), ts)
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
		if tb.isWorkflow() {
			b.WriteString("    /** The id of the group that may see it; only you when left out. Not read back. */\n    access?: string | null;\n")
		}
		b.WriteString("}\n")
	}
	return b.String(), n
}

var runIdentRe = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

func runProp(s string) string {
	if runIdentRe.MatchString(s) {
		return s
	}
	return runQuote(s)
}
