package main

// Archive's tables of what connections sync (archive.sql): each stream's records land as rows of the table it
// names, as its Rows maps them (connect/tables.go), with the rows that belong to each, like a message's
// parties. syncSave writes them a page at a time, in the transaction it saves the stream's cursor in: a row
// for each source_id of each stream of each connection, written again, with all its own rows, when it is read
// again, only seen when the stream says it is the Same, and deleted when it is gone there, and with the people
// and organizations nothing names any more.
//
// Every row has its connection and its access, the connection's, so row-level security sees it as it sees the
// connection. A column that names another row of the connection keeps that row's id there in <column>_source,
// and links once both are there, whichever came first (syncLink).

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Mulholland-Inc/archive/connect"
	"github.com/Mulholland-Inc/archive/generated"
	"github.com/google/uuid"
)

// syncRowsVersion is the version of the tables: a stream whose rows are of one before syncs from scratch, its
// cursor and memory forgotten, so its rows are written whole again. Changing a table or a mapping so that the
// rows already synced should change counts it up.
const syncRowsVersion = 2

// syncTable is what syncSave needs to know of a table of archive.sql: for a row that belongs to a row of another
// table, that table, its parent, and the column that names it; and its columns that link another row of the
// connection, by the table of that row.
type syncTable struct {
	parent, of string
	links      map[string]string
}

// syncSchema are the tables of archive.sql that syncs write.
var syncSchema = map[string]syncTable{
	"mail_message":                     {},
	"mail_party":                       {parent: "mail_message", of: "message"},
	"mail_attachment":                  {parent: "mail_message", of: "message"},
	"doc_page":                         {},
	"payroll_company":                  {},
	"payroll_department":               {links: map[string]string{"company": "payroll_company"}},
	"payroll_location":                 {links: map[string]string{"company": "payroll_company"}},
	"payroll_schedule":                 {links: map[string]string{"company": "payroll_company"}},
	"payroll_worker":                   {links: map[string]string{"company": "payroll_company", "department": "payroll_department", "manager": "payroll_worker"}},
	"payroll_job":                      {parent: "payroll_worker", of: "worker", links: map[string]string{"location": "payroll_location"}},
	"payroll_compensation":             {parent: "payroll_worker", of: "worker"},
	"payroll_address":                  {parent: "payroll_worker", of: "worker", links: map[string]string{"location": "payroll_location"}},
	"payroll_employment":               {parent: "payroll_worker", of: "worker"},
	"payroll_custom_field":             {links: map[string]string{"company": "payroll_company"}},
	"payroll_custom_value":             {parent: "payroll_worker", of: "worker", links: map[string]string{"field": "payroll_custom_field"}},
	"payroll_department_member":        {parent: "payroll_department", of: "department", links: map[string]string{"worker": "payroll_worker"}},
	"payroll_schedule_assignment":      {links: map[string]string{"company": "payroll_company", "default_schedule": "payroll_schedule", "hourly_schedule": "payroll_schedule", "salaried_schedule": "payroll_schedule"}},
	"payroll_schedule_member":          {parent: "payroll_schedule_assignment", of: "assignment", links: map[string]string{"worker": "payroll_worker", "department": "payroll_department", "schedule": "payroll_schedule"}},
	"payroll_earning_type":             {links: map[string]string{"company": "payroll_company"}},
	"payroll_time_off_request":         {links: map[string]string{"company": "payroll_company", "worker": "payroll_worker", "approver": "payroll_worker"}},
	"payroll_time_off_day":             {parent: "payroll_time_off_request", of: "request"},
	"payroll_time_off_balance":         {links: map[string]string{"company": "payroll_company", "worker": "payroll_worker"}},
	"payroll_time_off_policy":          {parent: "payroll_time_off_balance", of: "balance"},
	"payroll_run":                      {links: map[string]string{"company": "payroll_company", "schedule": "payroll_schedule"}},
	"payroll_pay":                      {parent: "payroll_run", of: "run", links: map[string]string{"worker": "payroll_worker"}},
	"payroll_pay_line":                 {parent: "payroll_run", of: "run", links: map[string]string{"worker": "payroll_worker"}},
	"payroll_pay_period":               {links: map[string]string{"company": "payroll_company", "schedule": "payroll_schedule", "run": "payroll_run"}},
	"payroll_contractor_payment":       {links: map[string]string{"company": "payroll_company", "worker": "payroll_worker"}},
	"payroll_contractor_payment_group": {links: map[string]string{"company": "payroll_company"}},
	"payroll_task":                     {links: map[string]string{"company": "payroll_company"}},
}

// syncRec is a record of a page to write, its data as JSON.
type syncRec struct {
	ID      string
	Data    json.RawMessage
	Updated time.Time
}

// syncDB is the transaction syncSave writes in: its queries, and the statements of tables.go.
type syncDB struct {
	q   *generated.Queries
	db  generated.DBTX
	id  uuid.UUID
	acc *string
	now time.Time
}

// write writes recs of stream s into its table, with the rows that belong to each, and links what they name.
// A record that is no row there is deleted.
func (w *syncDB) write(ctx context.Context, s *connect.Stream, recs []syncRec) error {
	t, ok := syncSchema[s.Table]
	if !ok || t.parent != "" {
		return fmt.Errorf("%s lands in %q, which is no table of archive.sql a stream lands in", s.Name, s.Table)
	}
	rows := make([]*connect.Row, 0, len(recs))
	var kept []syncRec
	var none []string
	for _, r := range recs {
		if row := s.RowOf(r.Data); row != nil {
			rows, kept = append(rows, row), append(kept, r)
		} else {
			none = append(none, r.ID)
		}
	}
	if len(none) > 0 {
		if err := w.delete(ctx, s, none); err != nil {
			return err
		}
	}
	if len(rows) == 0 {
		return nil
	}
	people, orgs, err := w.known(ctx, rows)
	if err != nil {
		return err
	}
	objs := make([]map[string]any, len(rows))
	for i, row := range rows {
		o := w.object(s.Table, row, people, orgs)
		o["stream"], o["source_id"], o["seen_at"], o["updated_at"] = s.Name, kept[i].ID, w.now, w.now
		if !kept[i].Updated.IsZero() {
			o["updated_at"] = kept[i].Updated
		}
		objs[i] = o
	}
	cols := syncCols(objs)
	set := []string{}
	var compare []string
	for _, c := range cols {
		if c == "connection" || c == "stream" || c == "source_id" || c == "updated_at" {
			continue
		}
		set = append(set, c+" = EXCLUDED."+c)
		if c != "seen_at" && c != "access" && t.links[c] == "" {
			compare = append(compare, c)
		}
	}
	tbl := "archive." + s.Table
	sql := "INSERT INTO " + tbl + " AS t (" + strings.Join(cols, ", ") + ") SELECT " + strings.Join(cols, ", ") +
		" FROM jsonb_populate_recordset(NULL::" + tbl + ", $1) ON CONFLICT (connection, stream, source_id) DO UPDATE SET " + strings.Join(set, ", ") +
		", updated_at = CASE WHEN (" + syncTuple("t", compare) + ") IS DISTINCT FROM (" + syncTuple("EXCLUDED", compare) + ") THEN EXCLUDED.updated_at ELSE t.updated_at END" +
		" RETURNING id, source_id"
	res, err := w.db.Query(ctx, sql, jsonOf(objs))
	if err != nil {
		return fmt.Errorf("%s: %w", s.Table, err)
	}
	ids := map[string]uuid.UUID{}
	for res.Next() {
		var id uuid.UUID
		var src string
		if err := res.Scan(&id, &src); err != nil {
			res.Close()
			return err
		}
		ids[src] = id
	}
	res.Close()
	if err := res.Err(); err != nil {
		return fmt.Errorf("%s: %w", s.Table, err)
	}
	// The rows that belong to them are theirs alone: those of before go.
	written := []string{s.Table}
	parents := slices.Collect(maps.Values(ids))
	kids := map[string][]map[string]any{}
	for i, row := range rows {
		for _, c := range row.Children {
			ct, ok := syncSchema[c.Table]
			if !ok || ct.parent != s.Table {
				return fmt.Errorf("%s: %q is no table of the rows of %s", s.Name, c.Table, s.Table)
			}
			o := w.object(c.Table, &c.Row, people, orgs)
			o[ct.of] = ids[kept[i].ID]
			kids[c.Table] = append(kids[c.Table], o)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(syncSchema)) {
		ct := syncSchema[name]
		if ct.parent != s.Table {
			continue
		}
		written = append(written, name)
		if _, err := w.db.Exec(ctx, "DELETE FROM archive."+name+" WHERE "+ct.of+" = ANY($1)", parents); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if len(kids[name]) == 0 {
			continue
		}
		cols := syncCols(kids[name])
		if _, err := w.db.Exec(ctx, "INSERT INTO archive."+name+" ("+strings.Join(cols, ", ")+") SELECT "+strings.Join(cols, ", ")+
			" FROM jsonb_populate_recordset(NULL::archive."+name+", $1)", jsonOf(kids[name])); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return w.link(ctx, written)
}

// object is the columns of a row of table: cols, its people and organizations by their ids, each link by its
// <column>_source, which links it once both are there, and its connection and access.
func (w *syncDB) object(table string, row *connect.Row, people, orgs map[string]uuid.UUID) map[string]any {
	o := maps.Clone(row.Cols)
	if o == nil {
		o = map[string]any{}
	}
	for col, e := range row.People {
		o[col] = people[e]
	}
	for col, e := range row.Orgs {
		o[col] = orgs[e]
	}
	for col := range syncSchema[table].links {
		o[col], o[col+"_source"] = nil, row.Refs[col]
	}
	o["connection"], o["access"] = w.id, w.acc
	return o
}

// known are the people and organizations rows name, by address and EIN, each added unless there is one.
func (w *syncDB) known(ctx context.Context, rows []*connect.Row) (map[string]uuid.UUID, map[string]uuid.UUID, error) {
	var emails, eins []string
	for _, r := range rows {
		for _, x := range append([]connect.Row{*r}, children(r)...) {
			emails = append(emails, slices.Collect(maps.Values(x.People))...)
			eins = append(eins, slices.Collect(maps.Values(x.Orgs))...)
		}
	}
	people, orgs := map[string]uuid.UUID{}, map[string]uuid.UUID{}
	if len(emails) > 0 {
		got, err := w.q.KnownPeople(ctx, emails)
		if err != nil {
			return nil, nil, err
		}
		for _, p := range got {
			people[p.Email] = p.ID
		}
	}
	if len(eins) > 0 {
		got, err := w.q.KnownOrgs(ctx, eins)
		if err != nil {
			return nil, nil, err
		}
		for _, o := range got {
			orgs[o.Ein] = o.ID
		}
	}
	return people, orgs, nil
}

func children(r *connect.Row) []connect.Row {
	out := make([]connect.Row, len(r.Children))
	for i, c := range r.Children {
		out[i] = c.Row
	}
	return out
}

// link links the rows of the connection that name a row of the tables written, or are of them, to the rows
// whose ids there they keep.
func (w *syncDB) link(ctx context.Context, written []string) error {
	for _, name := range slices.Sorted(maps.Keys(syncSchema)) {
		for _, col := range slices.Sorted(maps.Keys(syncSchema[name].links)) {
			to := syncSchema[name].links[col]
			if !slices.Contains(written, name) && !slices.Contains(written, to) {
				continue
			}
			if _, err := w.db.Exec(ctx, "UPDATE archive."+name+" x SET "+col+" = p.id FROM archive."+to+" p WHERE x.connection = $1 AND p.connection = $1"+
				" AND p.source_id = x."+col+"_source AND x."+col+" IS DISTINCT FROM p.id", w.id); err != nil {
				return fmt.Errorf("linking %s.%s: %w", name, col, err)
			}
		}
	}
	return nil
}

// see marks the rows of ids of stream s seen, as the Same.
func (w *syncDB) see(ctx context.Context, s *connect.Stream, ids []string) error {
	_, err := w.db.Exec(ctx, "UPDATE archive."+s.Table+" SET seen_at = $4 WHERE connection = $1 AND stream = $2 AND source_id = ANY($3)", w.id, s.Name, ids, w.now)
	return err
}

// delete deletes the rows of ids of stream s, gone there, with the rows that belong to them.
func (w *syncDB) delete(ctx context.Context, s *connect.Stream, ids []string) error {
	return w.gone(ctx, "DELETE FROM archive."+s.Table+" WHERE connection = $1 AND stream = $2 AND source_id = ANY($3)", w.id, s.Name, ids)
}

// unseen deletes the rows of stream s that a full listing begun at since did not read.
func (w *syncDB) unseen(ctx context.Context, s *connect.Stream, since time.Time) error {
	return w.gone(ctx, "DELETE FROM archive."+s.Table+" WHERE connection = $1 AND stream = $2 AND seen_at < $3", w.id, s.Name, since)
}

// stream deletes the rows of a stream the connector no longer declares, in whichever table they are.
func (w *syncDB) stream(ctx context.Context, name string) error {
	for _, t := range slices.Sorted(maps.Keys(syncSchema)) {
		if syncSchema[t].parent != "" {
			continue
		}
		if err := w.gone(ctx, "DELETE FROM archive."+t+" WHERE connection = $1 AND stream = $2", w.id, name); err != nil {
			return err
		}
	}
	return nil
}

// gone runs a delete, and deletes the people and organizations nothing names any more once it deleted any.
func (w *syncDB) gone(ctx context.Context, sql string, args ...any) error {
	tag, err := w.db.Exec(ctx, sql, args...)
	if err != nil || tag.RowsAffected() == 0 {
		return err
	}
	return w.q.DeleteUnknown(ctx)
}

// syncNarrow gives every row connection id synced the access its connection now has.
func syncNarrow(ctx context.Context, db generated.DBTX, id uuid.UUID, access *string) error {
	for _, t := range slices.Sorted(maps.Keys(syncSchema)) {
		if _, err := db.Exec(ctx, "UPDATE archive."+t+" SET access = $2 WHERE connection = $1 AND access IS DISTINCT FROM $2", id, access); err != nil {
			return err
		}
	}
	return nil
}

// syncCols are the columns of objs, all of each's keys, in order; a row without one has it null. A key that is
// no column name is left out, so nothing a mapping says becomes SQL; one that is no column of the table fails.
func syncCols(objs []map[string]any) []string {
	var cols []string
	for _, o := range objs {
		for k := range o {
			if syncColRe.MatchString(k) && !slices.Contains(cols, k) {
				cols = append(cols, k)
			}
		}
	}
	slices.Sort(cols)
	return cols
}

var syncColRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// syncTuple is cols of alias as a row, to compare.
func syncTuple(alias string, cols []string) string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = alias + "." + c
	}
	return "ROW(" + strings.Join(out, ", ") + ")"
}

func jsonOf(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
