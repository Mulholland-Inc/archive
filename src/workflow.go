package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Mulholland-Inc/archive/generated"
	"github.com/dop251/goja"
	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/file"
	"github.com/dop251/goja/parser"
	"github.com/dop251/goja/token"
	"github.com/evanw/esbuild/pkg/api"
	"github.com/go-sourcemap/sourcemap"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/robfig/cron/v3"
)

const (
	workflowBatch   = 100
	workflowDepth   = 4
	workflowRetry   = time.Minute
	workflowMaxSize = 256 << 10
	workflowMaxBody = 10 << 20
)

var (
	workflowNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}(\.ts)?$`)
	workflowLast   = uuid.Max
	workflowEvents = []string{"created", "updated"}
)

type WorkflowRun struct {
	ID      uuid.UUID       `json:"id"`
	At      time.Time       `json:"at"`
	Trigger string          `json:"trigger"`
	MS      int32           `json:"ms"`
	Result  json.RawMessage `json:"result"`
	Logs    json.RawMessage `json:"logs"`
	Error   string          `json:"error"`
}

type workflowOn struct {
	Cron    string   `json:"cron,omitempty"`
	TZ      string   `json:"tz,omitempty"`
	Table   string   `json:"table,omitempty"`
	Events  []string `json:"events,omitempty"`
	Webhook bool     `json:"webhook,omitempty"`

	sched cron.Schedule
	table *Table
}

func (o *workflowOn) kind() string {
	switch {
	case o == nil:
		return ""
	case o.Cron != "":
		return "cron"
	case o.Table != "":
		return "table"
	case o.Webhook:
		return "webhook"
	}
	return ""
}

func (o *workflowOn) trigger() string {
	switch o.kind() {
	case "cron":
		if o.TZ != "" {
			return "cron CRON_TZ=" + o.TZ + " " + o.Cron
		}
		return "cron " + o.Cron
	case "table":
		return o.Table
	case "webhook":
		return "webhook"
	}
	return ""
}

func (o *workflowOn) json() []byte {
	if o.kind() == "" {
		return nil
	}
	b, _ := json.Marshal(o)
	return b
}

func (o *workflowOn) wants(event string) bool {
	return o.kind() == "table" && slices.Contains(o.Events, event)
}

func workflowRunOf(r generated.ArchiveWorkflowRun) WorkflowRun {
	return WorkflowRun{ID: r.ID, At: r.CreatedAt.UTC(), Trigger: r.Trigger, MS: r.Ms, Result: r.Result, Logs: r.Logs, Error: r.Error}
}

func workflowToken() string {
	b := make([]byte, 24)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func (a *Archive) hookSign(hook string) string {
	m := hmac.New(sha256.New, secretDerive(a.hub.key, "workflow hook "+a.ws.ID.String()))
	m.Write([]byte(hook))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil)[:18])
}

func workflowTables(cat *Catalog) []string {
	var out []string
	for _, n := range cat.names() {
		t := cat.Tables[n]
		sync, ok := cat.sync[n]
		if t.Kind == "table" && (t.Schema == "public" || n == "member" || n == "inbox" || t.Schema == "archive" && ok && sync.parent == "") {
			out = append(out, n)
		}
	}
	return out
}

func (a *Archive) workflowParseOn(ctx context.Context, v any) (*workflowOn, []FieldError) {
	if b, ok := v.(json.RawMessage); ok {
		if len(b) == 0 {
			return &workflowOn{}, nil
		}
		v = nil
		dec := json.NewDecoder(strings.NewReader(string(b)))
		dec.UseNumber()
		if err := dec.Decode(&v); err != nil {
			return nil, []FieldError{{Path: "on", Message: "not JSON: " + err.Error()}}
		}
	}
	if v == nil {
		return &workflowOn{}, nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, []FieldError{{Path: "on", Message: `want { cron: "0 9 * * 1-5", tz?: "Europe/Amsterdam" }, { table: "invoice", events: ["created", "updated"] }, { webhook: true }, or null for a workflow that runs only with archive.run`}}
	}
	var errs []FieldError
	bad := func(path, msg string) { errs = append(errs, FieldError{Path: path, Message: msg}) }
	str := func(k string) string {
		s, ok := m[k].(string)
		if m[k] != nil && !ok {
			bad("on."+k, "must be a string")
		}
		return strings.TrimSpace(s)
	}
	o := &workflowOn{}
	var kinds []string
	for _, k := range specKeys(m) {
		switch k {
		case "cron", "table", "webhook":
			kinds = append(kinds, k)
		case "tz", "events":
		default:
			bad("on."+k, "unknown field; on takes cron and tz, table and events, or webhook")
		}
	}
	if len(kinds) != 1 {
		bad("on", "want exactly one of cron, table or webhook, like { cron: \"0 9 * * *\" }, { table: \"invoice\", events: [\"created\"] } or { webhook: true }")
		return nil, errs
	}
	if kinds[0] != "cron" && m["tz"] != nil {
		bad("on.tz", "only a cron workflow takes a time zone")
	}
	if kinds[0] != "table" && m["events"] != nil {
		bad("on.events", "only a table workflow takes events")
	}
	switch kinds[0] {
	case "cron":
		o.Cron, o.TZ = strings.Join(strings.Fields(str("cron")), " "), str("tz")
		spec := o.Cron
		switch {
		case o.Cron == "":
			bad("on.cron", "want a schedule: five fields, minute hour day month weekday, like 0 9 * * 1-5, or @every 15m, @hourly, @daily, @weekly, @monthly")
		case strings.Contains(o.Cron, "TZ="):
			bad("on.cron", "put the time zone in tz, like { cron: \"0 9 * * *\", tz: \"America/New_York\" }")
		}
		if o.TZ != "" {
			if _, err := time.LoadLocation(o.TZ); err != nil || o.TZ == "Local" {
				bad("on.tz", fmt.Sprintf("unknown time zone %q; want an IANA name like Europe/Amsterdam or America/New_York", o.TZ))
			} else {
				spec = "CRON_TZ=" + o.TZ + " " + spec
			}
		}
		if len(errs) == 0 {
			s, err := cron.ParseStandard(spec)
			if err != nil {
				bad("on.cron", fmt.Sprintf("%s: %v; want five fields, minute hour day month weekday, like 0 9 * * 1-5, or @every 15m, @hourly, @daily, @weekly, @monthly", o.Cron, err))
			}
			o.sched = s
		}
	case "table":
		o.Table = strings.TrimPrefix(str("table"), "public.")
		cat := a.catalog(ctx)
		names := workflowTables(cat)
		list := "there are no tables yet"
		if len(names) > 0 {
			list = "the tables are " + strings.Join(names, ", ")
		}
		switch tb := cat.Tables[o.Table]; {
		case o.Table == "":
			bad("on.table", "want the name of a table; "+list)
		case tb != nil && tb.Kind == "view":
			bad("on.table", o.Table+" is a view; watch a table it reads from instead ("+list+")")
		case !slices.Contains(names, o.Table):
			bad("on.table", "unknown table "+strconv.Quote(o.Table)+"; "+list)
		default:
			o.table = tb
		}
		switch ev := m["events"].(type) {
		case nil:
			o.Events = workflowEvents
		case []any:
			for i, e := range ev {
				s, _ := e.(string)
				if !slices.Contains(workflowEvents, s) {
					bad(fmt.Sprintf("on.events[%d]", i), "want created or updated")
				}
			}
			for _, e := range workflowEvents {
				if slices.Contains(ev, any(e)) {
					o.Events = append(o.Events, e)
				}
			}
			if len(ev) == 0 {
				bad("on.events", `want created, updated or both, like ["created", "updated"]`)
			}
		default:
			bad("on.events", `want a list, like ["created", "updated"]`)
		}
	case "webhook":
		if m["webhook"] != true {
			bad("on.webhook", "want true; for a workflow that runs only with archive.run, set on to null")
		}
		o.Webhook = true
	}
	if len(errs) > 0 {
		return nil, errs
	}
	return o, nil
}

func (a *Archive) workflowOf(ctx context.Context, w generated.ArchiveWorkflow) (*workflowOn, error) {
	o, errs := a.workflowParseOn(ctx, json.RawMessage(w.On))
	if len(errs) > 0 {
		return nil, (&ValidationError{Type: "workflow", Errors: errs}).err()
	}
	return o, nil
}

func (a *Archive) workflowSave(ctx context.Context, id uuid.UUID, data map[string]any) (map[string]any, error) {
	ac, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	return coreDo(a, ctx, func(ctx context.Context) (map[string]any, error) {
		q := a.st.db(ctx)
		var old *generated.ArchiveWorkflow
		var name, source string
		var on any
		var access *uuid.UUID
		if id != uuid.Nil {
			w, err := q.LockWorkflow(ctx, id)
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, fmt.Errorf("workflow %s: %w", id, ErrNotFound)
			}
			if err != nil {
				return nil, err
			}
			old = &w
			name, source, on, access = w.Name, w.Source, json.RawMessage(w.On), w.Access
		}
		verr := &ValidationError{Type: "workflow"}
		bad := func(path, msg string) { verr.Errors = append(verr.Errors, FieldError{Path: path, Message: msg}) }
		text := func(k string) string {
			s, ok := data[k].(string)
			if !ok {
				bad(k, "must be a string")
			}
			return s
		}
		for _, k := range specKeys(data) {
			switch k {
			case "name":
				name = strings.TrimSpace(text(k))
			case "source":
				source = text(k)
			case "on":
				on = data[k]
			case "access":
				access = nil
				if data[k] != nil {
					s, _ := data[k].(string)
					g, err := uuid.Parse(s)
					if err != nil {
						bad(k, "want a group's id, or null for only you")
						continue
					}
					access = &g
				}
			case "id", "created_at", "updated_at", "next_at", "url", "member", "broken":
			default:
				bad(k, "unknown field; a workflow takes name, source, on and access")
			}
		}
		switch {
		case name == "":
			bad("name", "required")
		case !workflowNameRe.MatchString(name):
			bad("name", "want letters, digits, - and _, starting with a letter or digit, at most 64, like overdue-invoices")
		}
		switch {
		case strings.TrimSpace(source) == "":
			bad("source", "required: the TypeScript it runs")
		case len(source) > workflowMaxSize:
			bad("source", fmt.Sprintf("%d KB, over the %d KB limit", len(source)>>10, workflowMaxSize>>10))
		}
		o, errs := a.workflowParseOn(ctx, on)
		verr.Errors = append(verr.Errors, errs...)
		if strings.TrimSpace(source) != "" && len(source) <= workflowMaxSize {
			var tb *Table
			if o != nil {
				tb = o.table
			}
			t, _ := ctx.Value(storeTxKey{}).(storeTx)
			f, err := flowCheck(&flowTx{ctx: ctx, tx: t.tx}, a.catalog(ctx), source, tb)
			if err != nil {
				return nil, err
			}
			for _, e := range f.errs {
				verr.Errors = append(verr.Errors, FieldError{Path: "source", Message: e.text(), Line: e.line, Column: e.col})
			}
		}
		if err := verr.err(); err != nil {
			return nil, err
		}
		now, err := q.TxNow(ctx)
		if err != nil {
			return nil, err
		}
		trig := o.trigger()
		var next, cursorAt *time.Time
		var cursorID *uuid.UUID
		switch {
		case old != nil && old.Trigger == trig:
			next, cursorAt, cursorID = old.NextAt, old.CursorAt, old.CursorID
			if o.kind() == "table" {
				next = &now
			}
		case o.kind() == "cron":
			t := o.sched.Next(now.UTC())
			next = &t
		case o.kind() == "table":
			next, cursorAt, cursorID = &now, &now, &workflowLast
		}
		if old == nil {
			id, err = q.InsertWorkflow(ctx, generated.InsertWorkflowParams{UserID: ac.UserID, Member: &ac.member, Role: ac.Role, Name: name, Source: source,
				OnJson: o.json(), Trigger: trig, Hook: workflowToken(), NextAt: next, CursorAt: cursorAt, CursorID: cursorID, Access: access})
		} else {
			_, err = q.UpdateWorkflow(ctx, generated.UpdateWorkflowParams{ID: id, Name: name, Source: source, OnJson: o.json(), Trigger: trig,
				NextAt: next, CursorAt: cursorAt, CursorID: cursorID, Access: access})
		}
		if err != nil {
			return nil, storeErr(err)
		}
		storeAfter(ctx, func() { a.workflowSchedule(ctx) })
		tb, err := a.catalog(ctx).table("workflow")
		if err != nil {
			return nil, err
		}
		row, err := a.coreGet(ctx, tb, id)
		a.workflowRows(ctx, row)
		return row, err
	})
}

func (a *Archive) workflowDelete(ctx context.Context, id uuid.UUID) error {
	return a.as(ctx, func(ctx context.Context) error {
		n, err := a.st.db(ctx).DeleteWorkflow(ctx, id)
		if err != nil {
			return storeErr(err)
		}
		if n == 0 {
			return fmt.Errorf("workflow %s: %w", id, ErrNotFound)
		}
		storeAfter(ctx, func() { a.workflowSchedule(ctx) })
		return nil
	})
}

func (a *Archive) workflowSchedule(ctx context.Context) {
	if err := a.scheduleWorkflows(context.WithValue(context.WithoutCancel(ctx), storeTxKey{}, nil)); err != nil {
		fmt.Fprintf(os.Stderr, "archive: workspace %s workflows: %v\n", a.ws.ID, err)
	}
}

func (a *Archive) workflowRows(ctx context.Context, rows ...map[string]any) {
	var me string
	if ac := ActorFrom(ctx); ac != nil {
		me = ac.member.String()
	}
	for _, r := range rows {
		if r == nil {
			continue
		}
		hook, _ := r["hook"].(string)
		delete(r, "hook")
		r["url"] = nil
		if on, _ := r["on"].(map[string]any); on != nil && on["webhook"] == true && hook != "" && fmt.Sprint(r["member"]) == me {
			r["url"] = a.hub.api + "/hooks/run/" + a.ws.ID.String() + "/" + hook + "." + a.hookSign(hook)
		}
	}
}

func (a *Archive) workflow(ctx context.Context, name string) (generated.ArchiveWorkflow, error) {
	ac, err := signedIn(ctx)
	if err != nil {
		return generated.ArchiveWorkflow{}, err
	}
	name = strings.TrimSpace(name)
	var w generated.ArchiveWorkflow
	var all []string
	err = a.as(ctx, func(ctx context.Context) (err error) {
		q := a.st.db(ctx)
		w, err = q.WorkflowByName(ctx, ac.UserID, name)
		if errors.Is(err, pgx.ErrNoRows) && !strings.HasSuffix(name, ".ts") {
			w, err = q.WorkflowByName(ctx, ac.UserID, name+".ts")
		}
		if errors.Is(err, pgx.ErrNoRows) {
			if all, err = q.WorkflowNames(ctx, ac.UserID); err == nil {
				err = pgx.ErrNoRows
			}
		}
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		if len(all) == 0 {
			return w, fmt.Errorf("workflow %s %w; you have none yet, save one with archive.create(\"workflow\", { name, source, on })", name, ErrNotFound)
		}
		return w, fmt.Errorf("workflow %s %w; yours are %s", name, ErrNotFound, strings.Join(all, ", "))
	}
	return w, err
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
		return nil, errors.New("the person who saved this workflow left the workspace, so it cannot run")
	}
	ac, err := a.userActor(ctx, u)
	if err != nil {
		return nil, errors.New("the person who saved this workflow left the workspace, so it cannot run")
	}
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
		return nil, errors.New("the token that saved this workflow was revoked, so it cannot run; save it again, with archive.update")
	}
	ac.Role = w.Role
	return ac, nil
}

func (a *Archive) workflowExec(ctx context.Context, w generated.ArchiveWorkflow, ac *Actor, trigger string, input any) (WorkflowRun, error) {
	start := time.Now()
	out, at := &RunOutput{Result: json.RawMessage("null"), Logs: []string{}}, start
	var err error
	if w.Broken != nil {
		err = workflowStopped(w)
	} else if ac == nil {
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

func (a *Archive) workflowNext(ctx context.Context) (bool, error) {
	var w generated.ArchiveWorkflow
	err := a.sys(ctx, func(ctx context.Context) (err error) {
		w, err = a.st.db(ctx).ClaimWorkflow(ctx, time.Now().Add(runTimeout+browseTimeout+workflowRetry))
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	a.refresh(ctx)
	on, perr := a.workflowOf(ctx, w)
	var next *time.Time
	switch {
	case perr != nil:
		_, err = a.workflowRecord(ctx, w.ID, w.Trigger, time.Now(), &RunOutput{Result: json.RawMessage("null"), Logs: []string{}, Error: runMessage(perr) + "; change its on with archive.update"}, 0)
	case on.kind() == "cron":
		_, err = a.workflowExec(ctx, w, nil, "cron", map[string]any{"trigger": "cron", "at": time.Now().UTC().Format(time.RFC3339)})
		t := on.sched.Next(time.Now().UTC())
		next = &t
	case on.kind() == "table":
		return true, a.workflowTable(ctx, w, on)
	}
	return true, errors.Join(err, a.workflowAt(context.WithoutCancel(ctx), w, next))
}

func (a *Archive) workflowTable(ctx context.Context, w generated.ArchiveWorkflow, on *workflowOn) error {
	ctx = context.WithoutCancel(ctx)
	tb := on.table
	wait := func() error { return a.workflowAt(ctx, w, nil) }
	fail := func(err error) error {
		_, rerr := a.workflowRecord(ctx, w.ID, "table", time.Now(), &RunOutput{Result: json.RawMessage("null"), Logs: []string{}, Error: err.Error()}, 0)
		return errors.Join(rerr, wait())
	}
	if w.Broken != nil {
		return fail(workflowStopped(w))
	}
	if w.CursorAt == nil || w.CursorID == nil {
		return fail(errors.New("this workflow has no table cursor; save it again with archive.update"))
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
	which := ""
	switch {
	case !on.wants("updated"):
		which = " AND t.created_at = t.updated_at"
	case !on.wants("created"):
		which = " AND t.created_at <> t.updated_at"
	}
	type found struct {
		row     map[string]any
		at      time.Time
		id      uuid.UUID
		created bool
	}
	read := func() (rows []found, err error) {
		err = a.as(WithActor(ctx, ac), func(ctx context.Context) error {
			ts := "greatest(t.created_at, t.updated_at)"
			res, err := a.st.Q(ctx).Query(ctx, `SELECT `+tb.rowSQL()+`, `+ts+`, t.id, t.created_at = t.updated_at FROM `+tb.ident()+` t WHERE (`+ts+`, t.id) > ($1, $2) AND `+ts+` <> ALL ($3::timestamptz[])`+which+` ORDER BY 2, 3 LIMIT `+strconv.Itoa(workflowBatch), *w.CursorAt, *w.CursorID, own)
			if err != nil {
				return err
			}
			var b []byte
			var f found
			_, err = pgx.ForEachRow(res, []any{&b, &f.at, &f.id, &f.created}, func() error {
				r, err := rowDecode(b)
				f.row = r
				rows = append(rows, f)
				return err
			})
			return err
		})
		return rows, err
	}
	rows, err := read()
	if err != nil {
		return fail(fmt.Errorf("reading new %s rows: %s", tb.Name, runMessage(err)))
	}
	if len(rows) == 0 {
		if err := wait(); err != nil {
			return err
		}
		if rows, err = read(); err != nil || len(rows) == 0 {
			return err
		}
		now := time.Now()
		return a.workflowAt(ctx, w, &now)
	}
	created, updated, all := []map[string]any{}, []map[string]any{}, []map[string]any{}
	for _, f := range rows {
		if f.created {
			created = append(created, f.row)
		} else {
			updated = append(updated, f.row)
		}
		all = append(all, f.row)
	}
	rec, err := a.workflowExec(ctx, w, ac, "table", map[string]any{"trigger": "table", "table": tb.Name, "created": created, "updated": updated, "rows": all})
	if err != nil {
		return errors.Join(err, wait())
	}
	if rec.Error != "" {
		return wait()
	}
	last, t := rows[len(rows)-1], time.Now()
	return a.sys(ctx, func(ctx context.Context) error {
		a.moved(ctx, "archive.workflow", w.ID.String())
		return a.st.db(ctx).AdvanceWorkflow(ctx, generated.AdvanceWorkflowParams{ID: w.ID, Trigger: w.Trigger, NextAt: &t, CursorAt: &last.at, CursorID: &last.id})
	})
}

func workflowStopped(w generated.ArchiveWorkflow) error {
	return fmt.Errorf("a migration stopped this workflow until it is saved clean with archive.update: %s", *w.Broken)
}

func (a *Archive) workflowAt(ctx context.Context, w generated.ArchiveWorkflow, next *time.Time) error {
	return a.sys(ctx, func(ctx context.Context) error {
		a.moved(ctx, "archive.workflow", w.ID.String())
		return a.st.db(ctx).SetWorkflowNext(ctx, w.ID, w.Trigger, next)
	})
}

func (a *Archive) workflowHook(rw http.ResponseWriter, r *http.Request) {
	var w generated.ArchiveWorkflow
	hook, sig, signed := strings.Cut(r.PathValue("token"), ".")
	err := a.sys(r.Context(), func(ctx context.Context) (err error) {
		w, err = a.st.db(ctx).WorkflowByHook(ctx, hook)
		return err
	})
	if err == nil && (signed && !hmac.Equal([]byte(sig), []byte(a.hookSign(hook))) || !signed && w.Access != nil) {
		err = pgx.ErrNoRows
	}
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
	rec, err := a.workflowExec(context.WithoutCancel(r.Context()), w, nil, "webhook", map[string]any{"trigger": "webhook", "method": r.Method, "query": query, "headers": headers, "body": body})
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

func (r *runner) workflowSDK(o *goja.Object) {
	a := r.a
	r.def(o, "run", func(ctx context.Context, call goja.FunctionCall) (any, error) {
		name, err := r.str(call, 0, "run", "name")
		if err != nil {
			return nil, err
		}
		var in json.RawMessage
		if v := call.Argument(1); !goja.IsUndefined(v) {
			if in, err = r.json(v); err != nil {
				return nil, fmt.Errorf("archive.run: input is not JSON: %v: %w", err, httpErrBadRequest)
			}
		}
		w, err := a.workflow(ctx, name)
		if err != nil {
			return nil, err
		}
		start := time.Now()
		var out *RunOutput
		if w.Broken != nil {
			out = &RunOutput{Result: json.RawMessage("null"), Logs: []string{}, Error: workflowStopped(w).Error()}
		} else if out, err = r.nested(ctx, w.Source, in); err != nil {
			return nil, err
		}
		d, at := time.Since(start), r.at
		storeAfter(ctx, func() { a.workflowRecord(context.WithoutCancel(ctx), w.ID, "run", at, out, d) })
		return out, nil
	})
}

type flowSrc struct {
	table, sql string
	line, col  int
	names      []string
	kinds      map[string]string
	open       bool
	reads      map[string][2]int
}

func (s *flowSrc) what() string {
	if s.table != "" {
		return s.table
	}
	return "this query"
}

type flowVal struct {
	src   *flowSrc
	many  bool
	input bool
	lit   bool
	str   string
	at    file.Idx
}

type flowDesc struct {
	names []string
	kinds map[string]string
	err   error
}

type flowTx struct {
	ctx   context.Context
	tx    pgx.Tx
	sqls  map[string]*flowDesc
	kinds map[uint32]string
}

var flowWriteRe = regexp.MustCompile(`(?i)^\s*(INSERT|UPDATE|DELETE|MERGE|TRUNCATE|CREATE|ALTER|DROP)\b`)

func (d *flowTx) describe(q string) (*flowDesc, error) {
	if x := d.sqls[q]; x != nil {
		return x, nil
	}
	x := &flowDesc{kinds: map[string]string{}}
	if d.sqls == nil {
		d.sqls, d.kinds = map[string]*flowDesc{}, map[uint32]string{}
	}
	d.sqls[q] = x
	if flowWriteRe.MatchString(q) {
		x.err = errors.New("archive.query only reads; change rows with archive.create, archive.update and archive.delete, and the schema with archive.migrate")
		return x, nil
	}
	sp, err := d.tx.Begin(d.ctx)
	if err != nil {
		return nil, err
	}
	bare := strings.TrimRight(strings.TrimSpace(q), "; \t\r\n")
	sd, err := d.tx.Conn().PgConn().Prepare(d.ctx, "", bare, nil)
	if err != nil {
		var pe *pgconn.PgError
		if !errors.As(err, &pe) {
			return nil, err
		}
		x.err = &runQueryErr{err, bare}
		return x, sp.Rollback(d.ctx)
	}
	if err := sp.Commit(d.ctx); err != nil {
		return nil, err
	}
	var want []uint32
	for _, c := range sd.Fields {
		if _, ok := d.kinds[c.DataTypeOID]; !ok {
			want = append(want, c.DataTypeOID)
		}
	}
	if len(want) > 0 {
		rows, _ := d.tx.Query(d.ctx, `SELECT oid, CASE WHEN typname IN ('int2', 'int4', 'int8', 'numeric', 'float4', 'float8') THEN 'number' WHEN typname = 'bool' THEN 'boolean'
            WHEN typname IN ('json', 'jsonb') THEN '' WHEN typcategory = 'A' THEN 'array' WHEN typtype = 'c' OR typname = 'record' THEN 'object' ELSE 'string' END
            FROM pg_catalog.pg_type WHERE oid = ANY($1::oid[])`, want)
		var oid uint32
		var kind string
		if _, err := pgx.ForEachRow(rows, []any{&oid, &kind}, func() error {
			d.kinds[oid] = kind
			return nil
		}); err != nil {
			return nil, err
		}
	}
	for _, c := range sd.Fields {
		x.names, x.kinds[c.Name] = append(x.names, c.Name), d.kinds[c.DataTypeOID]
	}
	return x, nil
}

type flow struct {
	d     *flowTx
	cat   *Catalog
	on    *Table
	src   string
	file  *file.File
	sm    *sourcemap.Consumer
	scope []map[string]flowVal
	fixed map[string]bool
	sets  map[string]bool
	depth int
	onSrc *flowSrc
	srcs  []*flowSrc
	errs  []runCompileErr
	lost  int
	fail  error
}

func flowCheck(d *flowTx, cat *Catalog, source string, on *Table) (*flow, error) {
	f := &flow{d: d, cat: cat, on: on, src: source, fixed: map[string]bool{}, sets: map[string]bool{}}
	res, errs := runBuild(source, api.SourceMapExternal)
	if len(errs) > 0 {
		f.errs = errs
		return f, nil
	}
	var err error
	if f.sm, err = sourcemap.Parse("", res.Map); err != nil {
		return nil, err
	}
	prog, err := parser.ParseFile(nil, "script.js", string(res.Code), 0, parser.WithDisableSourceMaps)
	if err != nil {
		var list parser.ErrorList
		if !errors.As(err, &list) {
			return nil, err
		}
		for _, e := range list {
			l, c := f.pos(e.Position.Line, e.Position.Column)
			f.errs = append(f.errs, runCompileErr{line: l, col: c, msg: e.Message})
		}
		return f, nil
	}
	f.file = prog.File
	f.scan(prog)
	f.scope = []map[string]flowVal{{}}
	for _, s := range prog.Body {
		f.use(s)
	}
	return f, f.fail
}

func (f *flow) pos(line, col int) (int, int) {
	if _, _, l, c, ok := f.sm.Source(line, col-1); ok {
		return max(l-1, 1), c + 1
	}
	return 0, 0
}

func (f *flow) at(i file.Idx) (int, int) {
	p := f.file.Position(int(i) - f.file.Base())
	return f.pos(p.Line, p.Column)
}

func (f *flow) bad(i file.Idx, format string, args ...any) {
	l, c := f.at(i)
	f.errs = append(f.errs, runCompileErr{line: l, col: c, msg: fmt.Sprintf(format, args...)})
}

func flowEach(n any, fn func(any)) {
	v := reflect.Indirect(reflect.ValueOf(n))
	if v.Kind() != reflect.Struct {
		return
	}
	var kid func(v reflect.Value)
	kid = func(v reflect.Value) {
		switch v.Kind() {
		case reflect.Interface, reflect.Pointer:
			if !v.IsNil() && v.CanInterface() {
				fn(v.Interface())
			}
		case reflect.Slice:
			for i := range v.Len() {
				kid(v.Index(i))
			}
		}
	}
	for i := range v.NumField() {
		kid(v.Field(i))
	}
}

var (
	flowMutators = map[string]bool{"push": true, "unshift": true, "splice": true, "pop": true, "shift": true, "fill": true, "copyWithin": true}
	flowProto    = map[string]bool{"constructor": true, "hasOwnProperty": true, "isPrototypeOf": true, "propertyIsEnumerable": true, "toLocaleString": true, "toString": true, "valueOf": true, "__proto__": true, "then": true, "toJSON": true}
	flowClose    = map[string]bool{"console": true, "JSON": true, "Object": true}
)

func (f *flow) scan(n any) {
	switch x := n.(type) {
	case *file.File:
		return
	case *ast.AssignExpression:
		f.target(x.Left)
	case *ast.UnaryExpression:
		if x.Operator == token.INCREMENT || x.Operator == token.DECREMENT {
			f.target(x.Operand)
		}
	case *ast.ForIntoExpression:
		f.target(x.Expression)
	case *ast.CallExpression:
		if d, ok := x.Callee.(*ast.DotExpression); ok {
			name := d.Identifier.Name.String()
			if id, ok := d.Left.(*ast.Identifier); ok && id.Name == "Object" && name == "assign" && len(x.ArgumentList) > 0 {
				f.target(x.ArgumentList[0])
			} else if flowMutators[name] {
				f.target(d.Left)
			}
		}
	}
	flowEach(n, f.scan)
}

func (f *flow) target(e any) {
	switch x := e.(type) {
	case *ast.Identifier:
		f.fixed[x.Name.String()] = true
	case *ast.DotExpression:
		f.sets[x.Identifier.Name.String()] = true
	case *ast.BracketExpression:
		if s, ok := x.Member.(*ast.StringLiteral); ok {
			f.sets[s.Value.String()] = true
		} else {
			f.target(x.Left)
		}
	case *ast.AssignExpression:
		f.target(x.Left)
	case *ast.ArrayPattern:
		for _, e := range x.Elements {
			f.target(e)
		}
		f.target(x.Rest)
	case *ast.ObjectPattern:
		for _, p := range x.Properties {
			switch p := p.(type) {
			case *ast.PropertyShort:
				f.fixed[p.Name.Name.String()] = true
			case *ast.PropertyKeyed:
				f.target(p.Value)
			}
		}
		f.target(x.Rest)
	}
}

func (f *flow) get(name string) (flowVal, bool) {
	for i := len(f.scope) - 1; i >= 0; i-- {
		if v, ok := f.scope[i][name]; ok {
			return v, true
		}
	}
	return flowVal{}, false
}

func (f *flow) escape(v flowVal) {
	if v.src != nil {
		v.src.open = true
		f.lost++
	}
}

func (f *flow) use(n any) { f.escape(f.eval(n)) }

func (f *flow) inner(fn func()) {
	f.scope = append(f.scope, map[string]flowVal{})
	fn()
	f.scope = f.scope[:len(f.scope)-1]
}

func (f *flow) eval(n any) flowVal {
	switch x := n.(type) {
	case *ast.StringLiteral:
		return flowVal{lit: true, str: x.Value.String(), at: x.Idx}
	case *ast.TemplateLiteral:
		if x.Tag != nil {
			flowEach(n, f.use)
			return flowVal{}
		}
		lit, s := true, ""
		for i, e := range x.Elements {
			s += e.Parsed.String()
			if i < len(x.Expressions) {
				v := f.eval(x.Expressions[i])
				f.escape(v)
				lit, s = lit && v.lit, s+v.str
			}
		}
		v := flowVal{lit: lit, str: s}
		if len(x.Expressions) == 0 {
			v.at = x.OpenQuote
		}
		return v
	case *ast.Identifier:
		name := x.Name.String()
		if v, ok := f.get(name); ok {
			return v
		}
		if name == "archive" {
			f.bad(x.Idx, "a saved workflow calls the SDK directly, like await archive.query(...), so that it can be checked; do not store archive or pass it around")
		}
		return flowVal{input: name == "input" && f.on != nil}
	case *ast.AwaitExpression:
		return f.eval(x.Argument)
	case *ast.OptionalChain:
		return f.eval(x.Expression)
	case *ast.Optional:
		return f.eval(x.Expression)
	case *ast.DotExpression:
		v := f.eval(x.Left)
		name := x.Identifier.Name.String()
		switch {
		case v.input && flowInput[name]:
			return flowVal{src: f.inputSrc(), many: true}
		case v.src != nil && !v.many:
			f.read(v.src, name, x.Identifier.Idx)
		case v.src != nil && name != "length":
			f.escape(v)
		}
	case *ast.BracketExpression:
		v := f.eval(x.Left)
		if v.src != nil && v.many {
			f.use(x.Member)
			return flowVal{src: v.src}
		}
		m := f.eval(x.Member)
		f.escape(m)
		if v.src != nil {
			if m.lit {
				f.read(v.src, m.str, x.LeftBracket)
			} else {
				f.escape(v)
			}
		}
	case *ast.CallExpression:
		return f.call(x)
	case *ast.BinaryExpression:
		switch {
		case x.Comparison:
			f.eval(x.Left)
			f.eval(x.Right)
		case x.Operator == token.LOGICAL_AND:
			f.eval(x.Left)
			return f.eval(x.Right)
		case x.Operator == token.PLUS:
			l, r := f.eval(x.Left), f.eval(x.Right)
			f.escape(l)
			f.escape(r)
			return flowVal{lit: l.lit && r.lit, str: l.str + r.str}
		default:
			f.use(x.Left)
			f.use(x.Right)
		}
	case *ast.UnaryExpression:
		if x.Operator == token.NOT || x.Operator == token.TYPEOF {
			f.eval(x.Operand)
		} else {
			f.use(x.Operand)
		}
	case *ast.ConditionalExpression:
		f.eval(x.Test)
		f.use(x.Consequent)
		f.use(x.Alternate)
	case *ast.IfStatement:
		f.eval(x.Test)
		f.use(x.Consequent)
		f.use(x.Alternate)
	case *ast.BlockStatement:
		f.inner(func() {
			for _, s := range x.List {
				f.use(s)
			}
		})
	case *ast.ExpressionStatement:
		f.eval(x.Expression)
	case *ast.LexicalDeclaration:
		f.binds(x.List)
	case *ast.VariableStatement:
		f.binds(x.List)
	case *ast.ReturnStatement:
		v := f.eval(x.Argument)
		if v.src != nil && f.depth <= 1 {
			v.src.open = true
		} else {
			f.escape(v)
		}
	case *ast.ForOfStatement:
		v := f.eval(x.Source)
		if !v.many {
			f.escape(v)
		}
		f.inner(func() {
			f.into(x.Into, flowVal{src: v.src})
			f.use(x.Body)
		})
	case *ast.ForInStatement:
		f.use(x.Source)
		f.inner(func() {
			f.into(x.Into, flowVal{})
			f.use(x.Body)
		})
	case *ast.ForStatement:
		f.inner(func() {
			switch i := x.Initializer.(type) {
			case *ast.ForLoopInitializerLexicalDecl:
				f.binds(i.LexicalDeclaration.List)
			case *ast.ForLoopInitializerVarDeclList:
				f.binds(i.List)
			case *ast.ForLoopInitializerExpression:
				f.use(i.Expression)
			}
			f.use(x.Test)
			f.use(x.Update)
			f.use(x.Body)
		})
	case *ast.CatchStatement:
		f.inner(func() {
			if x.Parameter != nil {
				f.bind(x.Parameter, flowVal{})
			}
			f.use(x.Body)
		})
	case *ast.FunctionDeclaration:
		if x.Function.Name != nil {
			f.scope[len(f.scope)-1][x.Function.Name.Name.String()] = flowVal{}
		}
		f.fn(x.Function)
	case *ast.FunctionLiteral, *ast.ArrowFunctionLiteral:
		f.fn(x)
	case *ast.PropertyShort:
		f.use(&x.Name)
		f.use(x.Initializer)
	default:
		if n != nil {
			flowEach(n, f.use)
		}
	}
	return flowVal{}
}

func (f *flow) fn(n any, args ...flowVal) {
	f.depth++
	f.inner(func() {
		var params *ast.ParameterList
		var body any
		switch x := n.(type) {
		case *ast.FunctionLiteral:
			params, body = x.ParameterList, x.Body
			if x.Name != nil {
				f.scope[len(f.scope)-1][x.Name.Name.String()] = flowVal{}
			}
		case *ast.ArrowFunctionLiteral:
			params, body = x.ParameterList, x.Body
		}
		for i, b := range params.List {
			var v flowVal
			if i < len(args) {
				v = args[i]
			}
			f.bind(b.Target, v)
			f.use(b.Initializer)
		}
		if params.Rest != nil {
			f.bind(params.Rest, flowVal{})
		}
		if e, ok := body.(*ast.ExpressionBody); ok {
			f.use(e.Expression)
		} else {
			f.use(body)
		}
	})
	f.depth--
}

func (f *flow) binds(list []*ast.Binding) {
	for _, b := range list {
		f.bind(b.Target, f.eval(b.Initializer))
	}
}

func (f *flow) into(i ast.ForInto, v flowVal) {
	switch x := i.(type) {
	case *ast.ForDeclaration:
		f.bind(x.Target, v)
	case *ast.ForIntoVar:
		f.bind(x.Binding.Target, v)
	case *ast.ForIntoExpression:
		f.escape(v)
	}
}

var flowInput = map[string]bool{"rows": true, "created": true, "updated": true}

func (f *flow) bind(t any, v flowVal) {
	switch x := t.(type) {
	case *ast.Identifier:
		name := x.Name.String()
		if f.fixed[name] {
			f.escape(v)
			v = flowVal{}
		}
		f.scope[len(f.scope)-1][name] = v
	case *ast.AssignExpression:
		f.use(x.Right)
		f.bind(x.Left, v)
	case *ast.ObjectPattern:
		one := func(key string, at file.Idx) flowVal {
			switch {
			case v.src != nil && !v.many:
				f.read(v.src, key, at)
			case v.input && flowInput[key]:
				return flowVal{src: f.inputSrc(), many: true}
			}
			return flowVal{}
		}
		for _, p := range x.Properties {
			switch p := p.(type) {
			case *ast.PropertyShort:
				f.bind(&p.Name, one(p.Name.Name.String(), p.Name.Idx))
				f.use(p.Initializer)
			case *ast.PropertyKeyed:
				if s, ok := p.Key.(*ast.StringLiteral); ok && !p.Computed {
					f.bind(p.Value, one(s.Value.String(), s.Idx))
				} else {
					f.use(p.Key)
					f.escape(v)
					f.bind(p.Value, flowVal{})
				}
			}
		}
		if x.Rest != nil {
			f.escape(v)
			f.bind(x.Rest, flowVal{})
		}
	case *ast.ArrayPattern:
		for _, e := range x.Elements {
			if e != nil {
				if v.many {
					f.bind(e, flowVal{src: v.src})
				} else {
					f.bind(e, flowVal{})
				}
			}
		}
		if x.Rest != nil {
			f.bind(x.Rest, flowVal{src: v.src, many: v.many})
		}
		if !v.many {
			f.escape(v)
		}
	}
}

func (f *flow) read(s *flowSrc, name string, at file.Idx) {
	if flowProto[name] || f.sets[name] {
		return
	}
	l, c := f.at(at)
	if _, ok := s.reads[name]; !ok {
		s.reads[name] = [2]int{l, c}
	}
	if s.kinds == nil {
		return
	}
	if _, ok := s.kinds[name]; !ok {
		if s.table != "" {
			f.bad(at, "%s.%s is not a column; it has %s", s.table, name, strings.Join(s.names, ", "))
		} else {
			f.bad(at, "this query returns no column %s; it returns %s", name, strings.Join(s.names, ", "))
		}
	}
}

func (f *flow) add(s *flowSrc, at file.Idx) *flowSrc {
	s.line, s.col = f.at(at)
	s.reads = map[string][2]int{}
	f.srcs = append(f.srcs, s)
	return s
}

func (f *flow) tableSrc(tb *Table, at file.Idx) *flowSrc {
	s := &flowSrc{table: tb.Name, kinds: map[string]string{}}
	for _, c := range tb.Columns {
		if c.Type == "tsvector" || slices.Contains(tb.internal, c.Name) || tb.isWorkflow() && c.Name == "hook" {
			continue
		}
		s.names, s.kinds[c.Name] = append(s.names, c.Name), f.cat.kind(c.Schema, c.Type)
		if c.List {
			s.kinds[c.Name] = "array"
		}
	}
	if tb.isWorkflow() {
		s.names, s.kinds["url"] = append(s.names, "url"), "string"
	}
	return f.add(s, at)
}

func (f *flow) inputSrc() *flowSrc {
	if f.onSrc == nil {
		f.onSrc = f.tableSrc(f.on, 1)
	}
	return f.onSrc
}

func (f *flow) call(x *ast.CallExpression) flowVal {
	callee := x.Callee
	if o, ok := callee.(*ast.Optional); ok {
		callee = o.Expression
	}
	d, ok := callee.(*ast.DotExpression)
	if !ok {
		f.use(callee)
		f.args(x.ArgumentList)
		return flowVal{}
	}
	name := d.Identifier.Name.String()
	if id, ok := d.Left.(*ast.Identifier); ok {
		if _, bound := f.get(id.Name.String()); !bound {
			switch {
			case id.Name == "archive":
				return f.sdk(name, x)
			case flowClose[id.Name.String()]:
				for _, a := range x.ArgumentList {
					if v := f.eval(a); v.src != nil {
						v.src.open = true
					}
				}
				return flowVal{}
			}
		}
	}
	v := f.eval(d.Left)
	switch {
	case v.src == nil:
		f.args(x.ArgumentList)
		return flowVal{}
	case !v.many:
		f.read(v.src, name, d.Identifier.Idx)
		f.args(x.ArgumentList)
		return flowVal{}
	}
	row := flowVal{src: v.src}
	switch name {
	case "map", "forEach", "flatMap", "some", "every", "filter", "find", "findLast", "findIndex", "findLastIndex", "sort", "toSorted", "reduce", "reduceRight":
		cb := []flowVal{row}
		if name == "reduce" || name == "reduceRight" {
			cb = []flowVal{{}, row}
		} else if name == "sort" || name == "toSorted" {
			cb = []flowVal{row, row}
		}
		for i, a := range x.ArgumentList {
			_, fl := a.(*ast.FunctionLiteral)
			_, al := a.(*ast.ArrowFunctionLiteral)
			switch {
			case i == 0 && (fl || al):
				f.fn(a, cb...)
			case i == 0:
				f.escape(v)
				f.use(a)
			default:
				f.use(a)
			}
		}
		switch name {
		case "filter", "sort", "toSorted":
			return v
		case "find", "findLast":
			return row
		}
		return flowVal{}
	case "slice", "reverse", "toReversed", "concat":
		f.args(x.ArgumentList)
		return v
	case "at":
		f.args(x.ArgumentList)
		return row
	}
	f.escape(v)
	f.args(x.ArgumentList)
	return flowVal{}
}

func (f *flow) args(list []ast.Expression) {
	for _, a := range list {
		f.use(a)
	}
}

func (f *flow) sdk(name string, x *ast.CallExpression) flowVal {
	args := x.ArgumentList
	first := func() flowVal {
		if len(args) > 0 {
			return f.eval(args[0])
		}
		return flowVal{}
	}
	switch name {
	case "query":
		v := first()
		f.args(args[min(len(args), 1):])
		if !v.lit {
			f.bad(x.LeftParenthesis, "a saved workflow needs constant SQL, so that it can be checked: write the whole statement as a string and pass values as $1, $2 params; to build SQL at run time, use a plain run, not a workflow")
			return flowVal{}
		}
		desc, err := f.d.describe(v.str)
		if err != nil {
			f.fail = errors.Join(f.fail, err)
			return flowVal{}
		}
		s := f.add(&flowSrc{sql: v.str, names: desc.names, kinds: desc.kinds}, x.LeftParenthesis)
		if desc.err != nil {
			s.kinds = nil
			f.sqlBad(v, x.LeftParenthesis, desc.err)
		}
		return flowVal{src: s, many: true}
	case "list", "get", "create", "update", "delete":
		v := first()
		if !v.lit {
			f.bad(x.LeftParenthesis, "a saved workflow needs a constant table name, so that it can be checked, like archive.%s(\"invoice\"); write one call per table; to pick a table at run time, use a plain run, not a workflow", name)
			f.args(args[min(len(args), 1):])
			return flowVal{}
		}
		if name == "create" && path.Ext(v.str) != "" {
			f.args(args[1:])
			return flowVal{}
		}
		tb := f.cat.Tables[v.str]
		if tb == nil {
			f.bad(args[0].Idx0(), "unknown table %q; the tables are %s", v.str, strings.Join(workflowTables(f.cat), ", "))
			f.args(args[1:])
			return flowVal{}
		}
		if tb.Kind == "view" && name != "list" && name != "get" {
			f.bad(args[0].Idx0(), "%s is a view, and views are read only; write to the tables it selects from", v.str)
		}
		for i, a := range args[1:] {
			switch {
			case name == "list" && i == 0:
				f.opts(tb, a)
			case tb.Schema == "public" && (name == "create" && i == 0 || name == "update" && i == 1):
				f.keys(tb, a, name)
			default:
				f.use(a)
			}
		}
		if name == "delete" || tb.write().save != nil && name != "get" && name != "list" {
			return flowVal{}
		}
		return flowVal{src: f.tableSrc(tb, x.LeftParenthesis), many: name == "list"}
	}
	f.args(args)
	return flowVal{}
}

func (f *flow) sqlBad(v flowVal, at file.Idx, err error) {
	if v.at != 0 {
		at = v.at
	}
	l, c := f.at(at)
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Position > 0 && v.at != 0 && l > 0 {
		lines := strings.SplitAfter(f.src, "\n")
		runes := []rune(strings.TrimSpace(v.str))
		lead := len(v.str) - len(strings.TrimLeft(v.str, " \t\r\n"))
		before := v.str[:lead] + string(runes[:min(int(pe.Position)-1, len(runes))])
		if l <= len(lines) && c <= len(lines[l-1]) && strings.HasPrefix(strings.Join(lines[l-1:], "")[c:], before) {
			if n := strings.Count(before, "\n"); n > 0 {
				l, c = l+n, len(before)-strings.LastIndex(before, "\n")
			} else {
				c += 1 + len(before)
			}
		}
	}
	f.errs = append(f.errs, runCompileErr{line: l, col: c, msg: runMessage(err)})
}

func (f *flow) opts(tb *Table, a ast.Expression) {
	lit, ok := a.(*ast.ObjectLiteral)
	if !ok {
		f.lost++
		f.use(a)
		return
	}
	for _, p := range lit.Value {
		if k, ok := p.(*ast.PropertyKeyed); ok && !k.Computed {
			if s, ok := k.Key.(*ast.StringLiteral); ok && s.Value.String() == "where" {
				f.keys(tb, k.Value, "where")
				continue
			}
		}
		f.use(p)
	}
}

func (f *flow) keys(tb *Table, a ast.Expression, mode string) {
	lit, ok := a.(*ast.ObjectLiteral)
	if !ok {
		f.lost++
		f.use(a)
		return
	}
	seen, whole := map[string]bool{}, true
	one := func(key string, at file.Idx, val ast.Expression) {
		seen[key] = true
		_, null := val.(*ast.NullLiteral)
		c := tb.col(key)
		switch {
		case c == nil && null && mode != "where":
		case c == nil || mode == "where" && !c.writable():
			f.bad(at, "%s.%s is not a column; it has %s", tb.Name, key, strings.Join(tb.colNames(), ", "))
		default:
			if s, ok := val.(*ast.StringLiteral); ok {
				if t := f.cat.typ(c.Schema, c.Type); t != nil && t.Kind == "e" && !slices.ContainsFunc(t.Enum, func(e string) bool { return strings.EqualFold(strings.TrimSpace(s.Value.String()), e) }) {
					f.bad(s.Idx, "%s.%s must be one of %s", tb.Name, key, strings.Join(t.Enum, ", "))
				}
			}
		}
	}
	for _, p := range lit.Value {
		switch p := p.(type) {
		case *ast.PropertyKeyed:
			s, ok := p.Key.(*ast.StringLiteral)
			if !ok || p.Computed {
				f.lost++
				whole = false
				f.use(p.Key)
			} else {
				one(s.Value.String(), s.Idx, p.Value)
			}
			f.use(p.Value)
		case *ast.PropertyShort:
			one(p.Name.Name.String(), p.Name.Idx, nil)
			f.use(p)
		default:
			whole = false
			f.lost++
			f.use(p)
		}
	}
	if mode == "create" && whole {
		for _, c := range tb.Columns {
			if c.writable() && c.required() && !seen[c.Name] {
				f.bad(lit.LeftBrace, "%s.%s is required; a create must set it", tb.Name, c.Name)
			}
		}
	}
}

type flowBreak struct {
	Workflow string     `json:"workflow"`
	ID       *uuid.UUID `json:"id,omitempty"`
	Line     int        `json:"line,omitempty"`
	Column   int        `json:"column,omitempty"`
	Message  string     `json:"message"`

	row uuid.UUID
	why string
}

func (b flowBreak) String() string {
	if b.Line == 0 {
		return b.Workflow + " " + b.Message
	}
	return fmt.Sprintf("%s line %d:%d: %s", b.Workflow, b.Line, b.Column, b.Message)
}

type flowSnap struct {
	source string
	srcs   []*flowSrc
}

func flowOn(w generated.WorkflowAllRow, cat *Catalog) (*Table, string) {
	var on struct {
		Table string `json:"table"`
	}
	json.Unmarshal(w.On, &on)
	if on.Table == "" {
		return nil, ""
	}
	if names := workflowTables(cat); !slices.Contains(names, on.Table) {
		return nil, fmt.Sprintf("on.table: unknown table %q; the tables are %s", on.Table, strings.Join(names, ", "))
	}
	return cat.Tables[on.Table], ""
}

func flowSnapshot(ctx context.Context, tx pgx.Tx, key string, cat *Catalog) (map[uuid.UUID]flowSnap, error) {
	all, err := generated.New(tx).WorkflowAll(ctx, key)
	if err != nil {
		return nil, err
	}
	d, out := &flowTx{ctx: ctx, tx: tx}, map[uuid.UUID]flowSnap{}
	for _, w := range all {
		on, _ := flowOn(w, cat)
		f, err := flowCheck(d, cat, w.Source, on)
		if err != nil {
			return nil, err
		}
		out[w.ID] = flowSnap{w.Source, f.srcs}
	}
	return out, nil
}

func flowBreaks(ctx context.Context, tx pgx.Tx, key string, cat *Catalog, before map[uuid.UUID]flowSnap, seen []generated.WorkflowSeenRow) ([]flowBreak, int, error) {
	all, err := generated.New(tx).WorkflowAll(ctx, key)
	if err != nil {
		return nil, 0, err
	}
	d, out, lost := &flowTx{ctx: ctx, tx: tx}, []flowBreak{}, 0
	for _, w := range all {
		if w.Broken {
			continue
		}
		add := func(l, c int, msg string) {
			b := flowBreak{Workflow: w.Name, Line: l, Column: c, Message: msg, row: w.ID, why: msg}
			if l > 0 {
				b.why = fmt.Sprintf("line %d:%d: %s", l, c, msg)
			}
			i := slices.IndexFunc(seen, func(s generated.WorkflowSeenRow) bool { return s.ID == w.ID })
			switch {
			case i < 0:
				b.Message = "this change breaks it; only its owner can see why"
			case seen[i].Mine:
				b.ID = &w.ID
			}
			out = append(out, b)
		}
		on, why := flowOn(w, cat)
		if why != "" {
			add(0, 0, why)
		}
		f, err := flowCheck(d, cat, w.Source, on)
		if err != nil {
			return nil, 0, err
		}
		lost += f.lost
		for _, e := range f.errs {
			add(e.line, e.col, e.msg)
		}
		snap, ok := before[w.ID]
		if !ok || snap.source != w.Source {
			continue
		}
		for i, s := range f.srcs {
			if i >= len(snap.srcs) || snap.srcs[i].table != s.table || snap.srcs[i].sql != s.sql || snap.srcs[i].kinds == nil || s.kinds == nil {
				continue
			}
			was := snap.srcs[i]
			for _, name := range was.names {
				at, read := was.reads[name]
				if !read && !was.open {
					continue
				}
				if !read {
					at = [2]int{s.line, s.col}
				}
				kind, has := s.kinds[name]
				switch {
				case !has && !read:
					add(at[0], at[1], fmt.Sprintf("%s no longer has %s, and this workflow passes its rows on", s.what(), name))
				case has && kind != was.kinds[name]:
					add(at[0], at[1], fmt.Sprintf("%s in %s reads as %s and would read as %s", name, s.what(), flowKind(was.kinds[name]), flowKind(kind)))
				}
			}
		}
	}
	return out, lost, nil
}

func flowKind(k string) string {
	if k == "" {
		return "JSON"
	}
	return "a " + k
}

func (a *Archive) workflowGuard(ctx context.Context) error {
	cat := a.st.txCatalog(ctx)
	if cat == nil {
		return nil
	}
	t := ctx.Value(storeTxKey{}).(storeTx)
	q := generated.New(t.tx)
	key, done, err := a.st.ddlKey(ctx, t.tx)
	if err != nil {
		return err
	}
	defer done()
	seen, err := q.WorkflowSeen(ctx)
	if err != nil {
		return err
	}
	breaks, _, err := flowBreaks(ctx, t.tx, key, cat, t.sch.before, seen)
	if err != nil || len(breaks) == 0 {
		return err
	}
	if t.sch.stop {
		why := map[string]string{}
		for _, b := range breaks {
			why[b.row.String()] = strings.TrimPrefix(why[b.row.String()]+"\n"+b.why, "\n")
		}
		j, err := json.Marshal(why)
		if err != nil {
			return err
		}
		return q.WorkflowStop(ctx, key, j)
	}
	lines := make([]string, len(breaks))
	for i, b := range breaks {
		lines[i] = b.String()
	}
	return fmt.Errorf("this migration breaks saved workflows; nothing was saved\n%s\nFix yours in this run with archive.update(\"workflow\", id, { source }), or, if the person you work for agrees, migrate with { confirm, breaks: true } to stop them until they are saved clean: %w", strings.Join(lines, "\n"), ErrConflict)
}
