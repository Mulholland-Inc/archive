package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Mulholland-Inc/archive/generated"
	"github.com/dop251/goja"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/robfig/cron/v3"
)

const (
	workflowBatch   = 100
	workflowDepth   = 4
	workflowPoll    = 3 * time.Second
	workflowQuiet   = time.Minute
	workflowRetry   = time.Minute
	workflowMaxSize = 256 << 10
	workflowMaxBody = 10 << 20
)

var (
	workflowNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}\.ts$`)
	workflowOnRe   = regexp.MustCompile(`(?i)^//\s*on\s*:(.*)$`)
	workflowLast   = uuid.Max
)

type Workflow struct {
	Name string       `json:"name"`
	On   string       `json:"on"`
	Next *time.Time   `json:"next,omitempty"`
	URL  string       `json:"url,omitempty"`
	Last *WorkflowRun `json:"last,omitempty"`
}

type WorkflowRun struct {
	ID      uuid.UUID       `json:"id"`
	At      time.Time       `json:"at"`
	Trigger string          `json:"trigger"`
	MS      int32           `json:"ms"`
	Result  json.RawMessage `json:"result"`
	Logs    json.RawMessage `json:"logs"`
	Error   string          `json:"error"`
}

type WorkflowError struct {
	File    string `json:"file"`
	Message string `json:"message"`
}

type WorkflowDeploy struct {
	Deployed  bool            `json:"deployed"`
	Errors    []WorkflowError `json:"errors,omitempty"`
	Workflows []Workflow      `json:"workflows,omitempty"`
}

type workflowOn struct {
	kind  string
	table *Table
	sched cron.Schedule
}

func workflowRunOf(r generated.ArchiveWorkflowRun) WorkflowRun {
	return WorkflowRun{ID: r.ID, At: r.CreatedAt.UTC(), Trigger: r.Trigger, MS: r.Ms, Result: r.Result, Logs: r.Logs, Error: r.Error}
}

func workflowUTC(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func workflowName(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasSuffix(s, ".ts") {
		s += ".ts"
	}
	return s
}

func workflowTrigger(src string) (string, error) {
	lines := strings.Split(strings.TrimPrefix(src, "\ufeff"), "\n")
	on := ""
	for i, l := range lines {
		m := workflowOnRe.FindStringSubmatch(strings.TrimSpace(l))
		switch {
		case m == nil:
		case i > 0:
			return "", fmt.Errorf("line %d: the // on: line must be the very first line of the file", i+1)
		default:
			on = strings.Join(strings.Fields(m[1]), " ")
			if on == "" {
				return "", errors.New("line 1: // on: is empty; write // on: cron 0 9 * * 1-5, // on: webhook or // on: <table>, or leave the line out for a workflow that runs only when started")
			}
		}
	}
	return on, nil
}

func (a *Archive) workflowParse(on string) (workflowOn, error) {
	word, rest, _ := strings.Cut(on, " ")
	switch strings.ToLower(word) {
	case "":
		return workflowOn{}, nil
	case "webhook":
		if rest != "" {
			return workflowOn{}, fmt.Errorf("on: webhook takes nothing after it, got %q", rest)
		}
		return workflowOn{kind: "webhook"}, nil
	case "cron":
		if rest == "" {
			return workflowOn{}, errors.New("on: cron needs a schedule, like // on: cron 0 9 * * 1-5 (minute hour day month weekday, UTC), // on: cron @every 15m or // on: cron CRON_TZ=America/New_York 0 9 * * *")
		}
		s, err := cron.ParseStandard(rest)
		if err != nil {
			return workflowOn{}, fmt.Errorf("on: cron %s: %v; want five fields, minute hour day month weekday, like cron 0 9 * * 1-5, or @every 15m, @hourly, @daily, @weekly", rest, err)
		}
		return workflowOn{kind: "cron", sched: s}, nil
	}
	cat := a.Catalog()
	var tables []string
	for _, n := range cat.names() {
		if cat.Tables[n].Kind != "view" {
			tables = append(tables, n)
		}
	}
	list := "there are no tables yet"
	if len(tables) > 0 {
		list = "the tables are " + strings.Join(tables, ", ")
	}
	tb, ok := cat.Tables[strings.TrimPrefix(word, "public.")]
	switch {
	case rest != "" || !ok:
		return workflowOn{}, fmt.Errorf("on: %s: unknown trigger; want cron <schedule>, webhook, or a table name (%s); leave the line out for a workflow that runs only when started", on, list)
	case tb.Kind == "view":
		return workflowOn{}, fmt.Errorf("on: %s is a view; trigger on a table it reads from instead (%s)", word, list)
	}
	return workflowOn{kind: "table", table: tb}, nil
}

func (a *Archive) workflowCheck(name, src string) (string, workflowOn, error) {
	if !workflowNameRe.MatchString(name) {
		return "", workflowOn{}, errors.New("the name must be letters, digits, - and _, ending in .ts, like overdue-invoices.ts")
	}
	if len(src) > workflowMaxSize {
		return "", workflowOn{}, fmt.Errorf("the file is %d KB, over the %d KB limit", len(src)>>10, workflowMaxSize>>10)
	}
	on, err := workflowTrigger(src)
	if err != nil {
		return "", workflowOn{}, err
	}
	if _, err := runCompile(src); err != nil {
		return "", workflowOn{}, err
	}
	w, err := a.workflowParse(on)
	return on, w, err
}

func workflowUser(ctx context.Context) (*Actor, error) {
	ac := ActorFrom(ctx)
	if ac == nil {
		return nil, fmt.Errorf("not signed in: %w", ErrUnauthorized)
	}
	return ac, nil
}

func (a *Archive) workflowOut(w generated.ArchiveWorkflow) Workflow {
	out := Workflow{Name: w.Name, On: w.Trigger}
	if on, err := a.workflowParse(w.Trigger); err == nil {
		switch on.kind {
		case "cron":
			out.Next = workflowUTC(w.NextAt)
		case "webhook":
			out.URL = a.hub.api + "/hooks/run/" + a.ws.Short + "/" + w.Hook
		}
	}
	return out
}

func (a *Archive) Deploy(ctx context.Context, files map[string]string) (*WorkflowDeploy, error) {
	ac, err := workflowUser(ctx)
	if err != nil {
		return nil, err
	}
	names := specKeys(files)
	type checked struct {
		on string
		w  workflowOn
	}
	ok := map[string]checked{}
	res := &WorkflowDeploy{}
	for _, n := range names {
		on, w, err := a.workflowCheck(n, files[n])
		if err != nil {
			res.Errors = append(res.Errors, WorkflowError{File: n, Message: err.Error()})
			continue
		}
		ok[n] = checked{on, w}
	}
	if len(res.Errors) > 0 {
		return res, nil
	}
	err = a.sys(ctx, func(ctx context.Context) error {
		q := a.st.db(ctx)
		now, err := q.TxNow(ctx)
		if err != nil {
			return err
		}
		if err := q.DeleteWorkflowsExcept(ctx, ac.UserID, append([]string{}, names...)); err != nil {
			return err
		}
		for _, n := range names {
			c := ok[n]
			p := generated.SaveWorkflowParams{UserID: ac.UserID, Role: ac.Role, Name: n, Source: files[n], Trigger: c.on, Hook: workflowToken()}
			switch c.w.kind {
			case "cron":
				next := c.w.sched.Next(now.UTC())
				p.NextAt = &next
			case "table":
				p.NextAt, p.CursorAt, p.CursorID = &now, &now, &workflowLast
			}
			w, err := q.SaveWorkflow(ctx, p)
			if err != nil {
				return err
			}
			res.Workflows = append(res.Workflows, a.workflowOut(w))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := a.scheduleWorkflows(ctx); err != nil {
		return nil, err
	}
	res.Deployed = true
	if res.Workflows == nil {
		res.Workflows = []Workflow{}
	}
	return res, nil
}

func workflowToken() string {
	b := make([]byte, 24)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func (a *Archive) workflowRows(ctx context.Context) ([]generated.ArchiveWorkflow, error) {
	ac, err := workflowUser(ctx)
	if err != nil {
		return nil, err
	}
	var ws []generated.ArchiveWorkflow
	return ws, a.sys(ctx, func(ctx context.Context) (err error) {
		ws, err = a.st.db(ctx).Workflows(ctx, ac.UserID)
		return err
	})
}

func (a *Archive) WorkflowFiles(ctx context.Context) (map[string]string, error) {
	ws, err := a.workflowRows(ctx)
	out := map[string]string{}
	for _, w := range ws {
		out[w.Name] = w.Source
	}
	return out, err
}

func (a *Archive) Workflows(ctx context.Context) ([]Workflow, error) {
	ac, err := workflowUser(ctx)
	if err != nil {
		return nil, err
	}
	out := []Workflow{}
	return out, a.sys(ctx, func(ctx context.Context) error {
		q := a.st.db(ctx)
		ws, err := q.Workflows(ctx, ac.UserID)
		if err != nil {
			return err
		}
		last, err := q.LastWorkflowRuns(ctx, ac.UserID)
		if err != nil {
			return err
		}
		for _, w := range ws {
			o := a.workflowOut(w)
			if i := slices.IndexFunc(last, func(r generated.ArchiveWorkflowRun) bool { return r.Workflow == w.ID }); i >= 0 {
				r := workflowRunOf(last[i])
				r.Result, r.Logs = nil, nil
				o.Last = &r
			}
			out = append(out, o)
		}
		return nil
	})
}

func (a *Archive) workflow(ctx context.Context, name string) (generated.ArchiveWorkflow, error) {
	ac, err := workflowUser(ctx)
	if err != nil {
		return generated.ArchiveWorkflow{}, err
	}
	name = workflowName(name)
	var w generated.ArchiveWorkflow
	var all []generated.ArchiveWorkflow
	err = a.sys(ctx, func(ctx context.Context) (err error) {
		q := a.st.db(ctx)
		if w, err = q.WorkflowByName(ctx, ac.UserID, name); errors.Is(err, pgx.ErrNoRows) {
			all, err = q.Workflows(ctx, ac.UserID)
			if err == nil {
				err = pgx.ErrNoRows
			}
		}
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		var ns []string
		for _, w := range all {
			ns = append(ns, w.Name)
		}
		if len(ns) == 0 {
			return w, fmt.Errorf("workflow %s %w; you have none yet, save one with archive.deploy", name, ErrNotFound)
		}
		return w, fmt.Errorf("workflow %s %w; yours are %s", name, ErrNotFound, strings.Join(ns, ", "))
	}
	return w, err
}

func (a *Archive) WorkflowRuns(ctx context.Context, name string, limit int) ([]WorkflowRun, error) {
	w, err := a.workflow(ctx, name)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	out := []WorkflowRun{}
	return out, a.sys(ctx, func(ctx context.Context) error {
		rs, err := a.st.db(ctx).WorkflowRuns(ctx, w.ID, int32(limit))
		for _, r := range rs {
			out = append(out, workflowRunOf(r))
		}
		return err
	})
}

func (a *Archive) workflowRecord(ctx context.Context, id uuid.UUID, trigger string, at time.Time, out *RunOutput, d time.Duration) (WorkflowRun, error) {
	logs, _ := json.Marshal(out.Logs)
	var r generated.ArchiveWorkflowRun
	err := a.sys(context.WithoutCancel(ctx), func(ctx context.Context) (err error) {
		q := a.st.db(ctx)
		r, err = q.AddWorkflowRun(ctx, generated.AddWorkflowRunParams{Workflow: id, Trigger: trigger, Result: out.Result, Logs: logs, Error: out.Error, Ms: int32(d.Milliseconds()), CreatedAt: at})
		if err != nil {
			return err
		}
		return q.TrimWorkflowRuns(ctx, id)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		err = nil
		r = generated.ArchiveWorkflowRun{ID: coreNewID(), Workflow: id, Trigger: trigger, Ms: int32(d.Milliseconds()), CreatedAt: at}
	}
	run := workflowRunOf(r)
	run.Result, run.Logs, run.Error = out.Result, logs, out.Error
	return run, err
}

func (a *Archive) workflowActor(ctx context.Context, w generated.ArchiveWorkflow) (*Actor, error) {
	u, err := userBy(ctx, a.hub, (*generated.Queries).UserByID, w.UserID)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(u.Workspaces, a.ws.ID) {
		return nil, errors.New("the person who deployed this workflow left the workspace, so it cannot run")
	}
	ac := a.userActor(u)
	if w.Role == ac.Role {
		return ac, nil
	}
	var ok bool
	if err := a.sys(ctx, func(ctx context.Context) (err error) {
		ok, err = a.st.db(ctx).RoleExists(ctx, w.Role)
		return err
	}); err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("the token that deployed this workflow was revoked, so it cannot run; deploy it again")
	}
	ac.Role = w.Role
	return ac, nil
}

func (a *Archive) workflowExec(ctx context.Context, w generated.ArchiveWorkflow, ac *Actor, trigger string, input any) (WorkflowRun, error) {
	start := time.Now()
	out, at := &RunOutput{Result: json.RawMessage("null"), Logs: []string{}}, start
	var err error
	if ac == nil {
		ac, err = a.workflowActor(ctx, w)
	}
	if err != nil {
		out.Error = err.Error()
	} else {
		var in json.RawMessage
		if input != nil {
			if in, err = json.Marshal(input); err != nil {
				return WorkflowRun{}, err
			}
		}
		if o, t, err := a.run(WithActor(ctx, ac), w.Source, in, true); err != nil {
			out.Error = err.Error()
		} else {
			out, at = o, t
		}
	}
	return a.workflowRecord(ctx, w.ID, trigger, at, out, time.Since(start))
}

func (a *Archive) DeleteWorkflow(ctx context.Context, name string) error {
	w, err := a.workflow(ctx, name)
	if err != nil {
		return err
	}
	return a.sys(ctx, func(ctx context.Context) error {
		return a.st.db(ctx).DeleteWorkflow(ctx, w.UserID, w.Name)
	})
}

func (a *Archive) RunWorkflow(ctx context.Context, name string) (WorkflowRun, error) {
	w, err := a.workflow(ctx, name)
	if err != nil {
		return WorkflowRun{}, err
	}
	return a.workflowExec(ctx, w, nil, "run", nil)
}

func (a *Archive) workflowNext(ctx context.Context) (bool, error) {
	var w generated.ArchiveWorkflow
	err := a.sys(ctx, func(ctx context.Context) (err error) {
		w, err = a.st.db(ctx).ClaimWorkflow(ctx, time.Now().Add(runTimeout()+workflowRetry))
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	a.refresh(ctx)
	on, perr := a.workflowParse(w.Trigger)
	var next *time.Time
	at := func(d time.Duration) *time.Time { t := time.Now().Add(d); return &t }
	switch {
	case perr != nil:
		_, err = a.workflowRecord(ctx, w.ID, "table", time.Now(), &RunOutput{Result: json.RawMessage("null"), Logs: []string{}, Error: perr.Error() + "; deploy it again with a trigger that exists"}, 0)
		next = at(workflowRetry)
	case on.kind == "cron":
		_, err = a.workflowExec(ctx, w, nil, "cron", nil)
		t := on.sched.Next(time.Now().UTC())
		next = &t
	case on.kind == "table":
		return true, a.workflowTable(ctx, w, on.table)
	}
	return true, errors.Join(err, a.sys(context.WithoutCancel(ctx), func(ctx context.Context) error {
		return a.st.db(ctx).SetWorkflowNext(ctx, w.ID, w.Trigger, next)
	}))
}

func (a *Archive) workflowTable(ctx context.Context, w generated.ArchiveWorkflow, tb *Table) error {
	ctx = context.WithoutCancel(ctx)
	next := func(d time.Duration) error {
		t := time.Now().Add(d)
		return a.sys(ctx, func(ctx context.Context) error { return a.st.db(ctx).SetWorkflowNext(ctx, w.ID, w.Trigger, &t) })
	}
	fail := func(err error) error {
		_, rerr := a.workflowRecord(ctx, w.ID, "table", time.Now(), &RunOutput{Result: json.RawMessage("null"), Logs: []string{}, Error: err.Error()}, 0)
		return errors.Join(rerr, next(workflowRetry))
	}
	if w.CursorAt == nil || w.CursorID == nil {
		return fail(errors.New("this workflow has no table cursor; deploy it again"))
	}
	ac, err := a.workflowActor(ctx, w)
	if err != nil {
		return fail(err)
	}
	var own []time.Time
	if err := a.sys(ctx, func(ctx context.Context) (err error) {
		own, err = a.st.db(ctx).WorkflowRunTimes(ctx, w.ID, *w.CursorAt)
		return err
	}); err != nil {
		return err
	}
	if own == nil {
		own = []time.Time{}
	}
	type found struct {
		row map[string]any
		at  time.Time
		id  uuid.UUID
	}
	var rows []found
	err = a.as(WithActor(ctx, ac), func(ctx context.Context) error {
		ts := "greatest(t.created_at, t.updated_at)"
		res, err := a.st.Q(ctx).Query(ctx, `SELECT `+tb.rowSQL("t")+`, `+ts+`, t.id FROM `+tb.ident()+` t WHERE (`+ts+`, t.id) > ($1, $2) AND `+ts+` <> ALL ($3::timestamptz[]) ORDER BY 2, 3 LIMIT `+strconv.Itoa(workflowBatch), *w.CursorAt, *w.CursorID, own)
		if err != nil {
			return err
		}
		var b []byte
		var f found
		_, err = pgx.ForEachRow(res, []any{&b, &f.at, &f.id}, func() error {
			r, err := rowDecode(b)
			f.row = r
			rows = append(rows, f)
			return err
		})
		return err
	})
	if err != nil {
		return fail(fmt.Errorf("reading new %s rows: %s", tb.Name, runMessage(err)))
	}
	if len(rows) == 0 {
		return next(min(max(time.Since(*w.CursorAt)/10, workflowPoll), workflowQuiet))
	}
	in := make([]map[string]any, len(rows))
	for i, f := range rows {
		in[i] = f.row
	}
	rec, err := a.workflowExec(ctx, w, ac, "table", map[string]any{"rows": in})
	if err != nil {
		return errors.Join(err, next(workflowRetry))
	}
	if rec.Error != "" {
		return next(workflowRetry)
	}
	last, wait := rows[len(rows)-1], workflowPoll
	if len(rows) == workflowBatch {
		wait = 0
	}
	t := time.Now().Add(wait)
	return a.sys(ctx, func(ctx context.Context) error {
		return a.st.db(ctx).AdvanceWorkflow(ctx, generated.AdvanceWorkflowParams{ID: w.ID, Trigger: w.Trigger, NextAt: &t, CursorAt: &last.at, CursorID: &last.id})
	})
}

func (a *Archive) workflowHook(rw http.ResponseWriter, r *http.Request) {
	var w generated.ArchiveWorkflow
	err := a.sys(r.Context(), func(ctx context.Context) (err error) {
		w, err = a.st.db(ctx).WorkflowByHook(ctx, r.PathValue("token"))
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		httpWrite(rw, http.StatusNotFound, map[string]string{"error": "no workflow has this webhook URL"}, nil)
		return
	}
	if err != nil {
		httpWrite(rw, 0, nil, err)
		return
	}
	b, err := io.ReadAll(http.MaxBytesReader(rw, r.Body, workflowMaxBody))
	if err != nil {
		httpWrite(rw, http.StatusRequestEntityTooLarge, map[string]string{"error": fmt.Sprintf("the body is over %d MB", workflowMaxBody>>20)}, nil)
		return
	}
	var body any
	switch {
	case len(b) == 0:
	case json.Valid(b):
		body = json.RawMessage(b)
	default:
		body = string(b)
	}
	query, headers := map[string]string{}, map[string]string{}
	for k, v := range r.URL.Query() {
		query[k] = v[0]
	}
	for k, v := range r.Header {
		headers[strings.ToLower(k)] = strings.Join(v, ", ")
	}
	rec, err := a.workflowExec(context.WithoutCancel(r.Context()), w, nil, "webhook", map[string]any{"method": r.Method, "query": query, "headers": headers, "body": body})
	switch {
	case err != nil:
		httpWrite(rw, 0, nil, err)
	case rec.Error != "":
		msg, _, _ := strings.Cut(rec.Error, "\n  at: ")
		httpWrite(rw, http.StatusInternalServerError, map[string]string{"error": msg}, nil)
	default:
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusOK)
		rw.Write(rec.Result)
	}
}

func (a *Archive) workflowRoutes(mux *http.ServeMux) {
	handle := func(pattern string, f func(r *http.Request) (int, any, error)) { httpHandle(mux, pattern, f) }
	handle("GET /api/workflows", func(r *http.Request) (int, any, error) {
		ws, err := a.Workflows(r.Context())
		return http.StatusOK, ws, err
	})
	handle("GET /api/workflows/{name}/runs", func(r *http.Request) (int, any, error) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		rs, err := a.WorkflowRuns(r.Context(), r.PathValue("name"), limit)
		return http.StatusOK, rs, err
	})
	handle("POST /api/workflows/{name}/run", func(r *http.Request) (int, any, error) {
		rec, err := a.RunWorkflow(context.WithoutCancel(r.Context()), r.PathValue("name"))
		return http.StatusOK, rec, err
	})
	handle("DELETE /api/workflows/{name}", func(r *http.Request) (int, any, error) {
		return http.StatusNoContent, nil, a.DeleteWorkflow(r.Context(), r.PathValue("name"))
	})
}

func (r *runner) workflowSDK(o *goja.Object) {
	a := r.a
	r.def(o, "workflows", func(ctx context.Context, call goja.FunctionCall) (any, error) {
		return a.WorkflowFiles(ctx)
	})
	r.def(o, "deploy", func(ctx context.Context, call goja.FunctionCall) (any, error) {
		var files map[string]string
		if err := r.arg(call, 0, "deploy", "files", &files); err != nil {
			return nil, err
		}
		if files == nil {
			return nil, fmt.Errorf("archive.deploy: files must be an object of <name>.ts to TypeScript, like the one archive.workflows() returns: %w", httpErrBadRequest)
		}
		return a.Deploy(ctx, files)
	})
	input := func(call goja.FunctionCall, fn string) (json.RawMessage, error) {
		v := call.Argument(1)
		if goja.IsUndefined(v) {
			return nil, nil
		}
		b, err := r.json(v)
		if err != nil {
			return nil, fmt.Errorf("archive.%s: input is not JSON: %v: %w", fn, err, httpErrBadRequest)
		}
		return b, nil
	}
	r.def(o, "test", func(ctx context.Context, call goja.FunctionCall) (any, error) {
		s, err := r.str(call, 0, "test", "nameOrSource")
		if err != nil {
			return nil, err
		}
		in, err := input(call, "test")
		if err != nil {
			return nil, err
		}
		src := s
		if workflowNameRe.MatchString(workflowName(s)) && !strings.ContainsAny(s, " \n;(") {
			w, err := a.workflow(ctx, s)
			if err != nil {
				return nil, err
			}
			src = w.Source
		} else if on, err := workflowTrigger(src); err != nil {
			return &RunOutput{Result: json.RawMessage("null"), Logs: []string{}, Error: err.Error()}, nil
		} else if _, err := a.workflowParse(on); err != nil {
			return &RunOutput{Result: json.RawMessage("null"), Logs: []string{}, Error: err.Error()}, nil
		}
		return r.nested(ctx, src, in, false)
	})
	r.def(o, "start", func(ctx context.Context, call goja.FunctionCall) (any, error) {
		name, err := r.str(call, 0, "start", "name")
		if err != nil {
			return nil, err
		}
		in, err := input(call, "start")
		if err != nil {
			return nil, err
		}
		w, err := a.workflow(ctx, name)
		if err != nil {
			return nil, err
		}
		start := time.Now()
		out, err := r.nested(ctx, w.Source, in, true)
		if err != nil {
			return nil, err
		}
		d, at := time.Since(start), r.at
		storeAfter(ctx, func() { a.workflowRecord(context.WithoutCancel(ctx), w.ID, "start", at, out, d) })
		return out, nil
	})
	r.def(o, "runs", func(ctx context.Context, call goja.FunctionCall) (any, error) {
		name, err := r.str(call, 0, "runs", "name")
		if err != nil {
			return nil, err
		}
		var opts struct {
			Limit int `json:"limit"`
		}
		if err := r.arg(call, 1, "runs", "opts", &opts); err != nil {
			return nil, err
		}
		return a.WorkflowRuns(ctx, name, opts.Limit)
	})
}
