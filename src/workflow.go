package main

// Workflows are rows of archive.workflow, which the catalog shows as the table workflow, and their runs rows of
// archive.workflow_run, the read-only table workflow_run. archive.create, update and delete of a workflow go
// through the write hook here, workflowSave and workflowDelete, which compile its source and check its name
// and on before anything is saved, and keep what Archive owns: who it runs as, its webhook secret, when it is
// due and the cursor of the table it watches. Reads are plain list, get and query, under row-level security:
// each person sees their own workflows and their runs (archive.mine).

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
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
	workflowRetry   = time.Minute
	workflowMaxSize = 256 << 10
	workflowMaxBody = 10 << 20
)

var (
	workflowNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}(\.ts)?$`)
	workflowOnRe   = regexp.MustCompile(`(?i)^//\s*on\s*:(.*)$`)
	workflowLast   = uuid.Max
	workflowEvents = []string{"created", "updated"}
)

// WorkflowRun is one run as Archive records it.
type WorkflowRun struct {
	ID      uuid.UUID       `json:"id"`
	At      time.Time       `json:"at"`
	Trigger string          `json:"trigger"`
	MS      int32           `json:"ms"`
	Result  json.RawMessage `json:"result"`
	Logs    json.RawMessage `json:"logs"`
	Error   string          `json:"error"`
}

// workflowOn is a workflow's on: when it runs. Its JSON is what the on column holds.
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

// trigger is on as Archive schedules it, in the trigger column: an edit that keeps it keeps the workflow's
// place in its schedule and its table. It is the // on: line of before workflows were rows, without events.
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

// workflowTables are the tables a workflow may watch: the public ones and the members.
func workflowTables(cat *Catalog) []string {
	var out []string
	for _, n := range cat.names() {
		if t := cat.Tables[n]; t.Kind == "table" && (t.Schema == "public" || n == "member") {
			out = append(out, n)
		}
	}
	return out
}

// workflowParseOn checks an on value, as a write passes it or the column holds it, and cleans it: a table
// workflow without events gets both. Each problem is one error, with its path.
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

// workflowOf is the on of a saved workflow, checked against the tables there are now.
func (a *Archive) workflowOf(ctx context.Context, w generated.ArchiveWorkflow) (*workflowOn, error) {
	b := w.On
	if len(b) == 0 && w.Trigger != "" {
		b = workflowLegacyOn(w.Trigger)
	}
	o, errs := a.workflowParseOn(ctx, json.RawMessage(b))
	if len(errs) > 0 {
		return nil, (&ValidationError{Type: "workflow", Errors: errs}).err()
	}
	return o, nil
}

// workflowLegacyOn is the on of a // on: line from before workflows were rows: cron <spec> with an optional
// CRON_TZ= or TZ=, webhook, or a table name.
func workflowLegacyOn(trigger string) []byte {
	word, rest, _ := strings.Cut(strings.Join(strings.Fields(trigger), " "), " ")
	var o workflowOn
	switch strings.ToLower(word) {
	case "":
		return nil
	case "webhook":
		o.Webhook = true
	case "cron":
		o.Cron = rest
		if z, spec, ok := strings.Cut(rest, " "); ok && (strings.HasPrefix(z, "CRON_TZ=") || strings.HasPrefix(z, "TZ=")) {
			_, o.TZ, _ = strings.Cut(z, "=")
			o.Cron = spec
		}
	default:
		o.Table, o.Events = strings.TrimPrefix(word, "public."), workflowEvents
		if rest != "" {
			o.Table += " " + rest
		}
	}
	b, _ := json.Marshal(o)
	return b
}

// workflowStrip takes a // on: first line off a source from before workflows were rows.
func workflowStrip(src string) string {
	first, rest, _ := strings.Cut(src, "\n")
	if workflowOnRe.MatchString(strings.TrimSpace(strings.TrimPrefix(first, "\ufeff"))) {
		return rest
	}
	return src
}

// workflowUpgrade gives the workflows saved before they were rows what a row has: on, parsed from the
// // on: line their source began with, which it takes off, and the member they belong to. It keeps their
// names, webhook secrets, cursors and next times as they are, and runs each time the workspace opens, so a
// workflow saved by an older instance in the meantime is upgraded too.
func (a *Archive) workflowUpgrade(ctx context.Context) error {
	ctx = context.WithValue(context.WithoutCancel(ctx), storeTxKey{}, nil)
	return a.st.System(ctx, func(ctx context.Context) error {
		q := a.st.db(ctx)
		old, err := q.OldWorkflows(ctx)
		if err != nil || len(old) == 0 {
			return err
		}
		members := map[uuid.UUID]*uuid.UUID{}
		for _, w := range old {
			p := generated.UpgradeWorkflowParams{ID: w.ID, Member: w.Member, Source: w.Source, OnJson: w.On, Trigger: w.Trigger}
			if len(w.On) == 0 && w.Trigger != "" {
				p.OnJson, p.Source = workflowLegacyOn(w.Trigger), workflowStrip(w.Source)
				var o workflowOn
				json.Unmarshal(p.OnJson, &o)
				p.Trigger = o.trigger()
			}
			if p.Member == nil {
				m, ok := members[w.UserID]
				if !ok {
					if u, err := userBy(ctx, a.hub, (*generated.Queries).UserByID, w.UserID); err == nil {
						if row, err := q.MemberByEmail(ctx, u.Email); err == nil {
							m = &row.ID
						}
					}
					members[w.UserID] = m
				}
				p.Member = m
			}
			if err := q.UpgradeWorkflow(ctx, p); err != nil {
				return err
			}
		}
		return nil
	})
}

// workflowSave is the write hook of archive.create and archive.update on workflow: it takes name, source and
// on, refuses the write with every problem when one is wrong, and otherwise saves it as the person writing,
// who it then runs as. With id it changes that workflow, and full replaces it whole.
func (a *Archive) workflowSave(ctx context.Context, tb *Table, id uuid.UUID, data map[string]any, full bool) (map[string]any, error) {
	ac, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	return coreDo(a, ctx, func(ctx context.Context) (map[string]any, error) {
		q := a.st.db(ctx)
		var old *generated.ArchiveWorkflow
		var name, source string
		var on any
		if id != uuid.Nil {
			w, err := q.LockWorkflow(ctx, id)
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, fmt.Errorf("workflow %s: %w", id, ErrNotFound)
			}
			if err != nil {
				return nil, err
			}
			old = &w
			if !full {
				name, source, on = w.Name, w.Source, json.RawMessage(w.On)
			}
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
			case "id", "created_at", "updated_at", "next_at", "url":
			default:
				bad(k, "unknown field; a workflow takes name, source and on")
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
		default:
			if _, errs := runTransform(source); len(errs) > 0 {
				for _, e := range errs {
					verr.Errors = append(verr.Errors, FieldError{Path: "source", Message: e.text(), Line: e.line, Column: e.col})
				}
			}
		}
		o, errs := a.workflowParseOn(ctx, on)
		verr.Errors = append(verr.Errors, errs...)
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
				OnJson: o.json(), Trigger: trig, Hook: workflowToken(), NextAt: next, CursorAt: cursorAt, CursorID: cursorID})
		} else {
			_, err = q.UpdateWorkflow(ctx, generated.UpdateWorkflowParams{ID: id, Name: name, Source: source, OnJson: o.json(), Trigger: trig,
				NextAt: next, CursorAt: cursorAt, CursorID: cursorID})
		}
		if err != nil {
			return nil, storeErr(err)
		}
		storeAfter(ctx, func() { a.workflowSchedule(ctx) })
		row, err := a.coreGet(ctx, tb, id)
		a.workflowRows(row)
		return row, err
	})
}

// workflowDelete is the write hook of archive.delete on workflow: the workflow goes with its runs, its
// schedule and its cursor.
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

// workflowRows gives each workflow row read through the catalog its webhook URL in place of its secret.
func (a *Archive) workflowRows(rows ...map[string]any) {
	for _, r := range rows {
		if r == nil {
			continue
		}
		hook, _ := r["hook"].(string)
		delete(r, "hook")
		r["url"] = nil
		if on, _ := r["on"].(map[string]any); on != nil && on["webhook"] == true && hook != "" {
			r["url"] = a.hub.api + "/hooks/run/" + a.ws.ID.String() + "/" + hook
		}
	}
}

// workflow is the caller's workflow named name, or name.ts, as their transaction sees it.
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
		if o, t, err := a.run(WithActor(ctx, ac), w.Source, in); err != nil {
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
	return true, errors.Join(err, a.sys(context.WithoutCancel(ctx), func(ctx context.Context) error {
		return a.st.db(ctx).SetWorkflowNext(ctx, w.ID, w.Trigger, next)
	}))
}

func (a *Archive) workflowTable(ctx context.Context, w generated.ArchiveWorkflow, on *workflowOn) error {
	ctx = context.WithoutCancel(ctx)
	tb := on.table
	wait := func() error {
		return a.sys(ctx, func(ctx context.Context) error { return a.st.db(ctx).SetWorkflowNext(ctx, w.ID, w.Trigger, nil) })
	}
	fail := func(err error) error {
		_, rerr := a.workflowRecord(ctx, w.ID, "table", time.Now(), &RunOutput{Result: json.RawMessage("null"), Logs: []string{}, Error: err.Error()}, 0)
		return errors.Join(rerr, wait())
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
	// A row is created while its created_at and updated_at are the same, as archive.create leaves them, and
	// updated once archive.update has moved updated_at on.
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
		return a.sys(ctx, func(ctx context.Context) error { return a.st.db(ctx).SetWorkflowNext(ctx, w.ID, w.Trigger, &now) })
	}
	// rows is both, in order, as workflows from before created and updated read them.
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
		out, err := r.nested(ctx, w.Source, in)
		if err != nil {
			return nil, err
		}
		d, at := time.Since(start), r.at
		storeAfter(ctx, func() { a.workflowRecord(context.WithoutCancel(ctx), w.ID, "run", at, out, d) })
		return out, nil
	})
}
