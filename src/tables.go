package main

// Archive's tables of what connections sync (archive.sql): a record, besides its copy in archive.record, lands
// as rows of its domain's tables, mail, docs or payroll, as its stream's Rows maps it (connect/tables.go).
// syncSave writes them in the transaction it writes the records in, and deleting a record deletes them, with
// the people and organizations nothing names any more. Records from before a stream had its tables, or from
// before the tables last changed, are written into them again by syncFill, a batch at a time.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/Mulholland-Inc/archive/connect"
	"github.com/Mulholland-Inc/archive/generated"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// syncRowsVersion is the version of what the tables hold of a record: changing a mapping so that the rows of
// records already synced should change too counts it up, and syncFill writes every record again.
const syncRowsVersion = 1

// syncFillBatch is how many records syncFill writes again in one transaction.
const syncFillBatch = 500

// syncRow is a record by its id, and its id there, with its data as archive.record has it.
type syncRow struct {
	Record   uuid.UUID
	SourceID string
	Data     json.RawMessage
}

// syncRows writes what the records rows of connection id's stream s are in Archive's tables. A record that is
// nothing there any more loses the rows it had.
func syncRows(ctx context.Context, q *generated.Queries, id uuid.UUID, s *connect.Stream, rows []syncRow) error {
	type mail struct {
		Record      uuid.UUID  `json:"record"`
		Thread      string     `json:"thread"`
		Subject     string     `json:"subject"`
		Snippet     string     `json:"snippet"`
		Sent        *time.Time `json:"sent_at"`
		Labels      []string   `json:"labels"`
		Attachments int        `json:"attachments"`
	}
	type party struct {
		Message uuid.UUID `json:"message"`
		Email   string    `json:"email"`
		Role    string    `json:"role"`
		Name    string    `json:"name"`
	}
	type doc struct {
		Record uuid.UUID  `json:"record"`
		Kind   string     `json:"kind"`
		Title  string     `json:"title"`
		URL    string     `json:"url"`
		Icon   string     `json:"icon"`
		Edited *time.Time `json:"edited_at"`
	}
	type company struct {
		Record     uuid.UUID `json:"record"`
		SourceID   string    `json:"source_id"`
		EIN        string    `json:"ein"`
		Name       string    `json:"name"`
		TradeName  string    `json:"trade_name"`
		EntityType string    `json:"entity_type"`
	}
	type worker struct {
		Record        uuid.UUID `json:"record"`
		SourceID      string    `json:"source_id"`
		Kind          string    `json:"kind"`
		Company       string    `json:"company_source"`
		Email         string    `json:"email"`
		PersonalEmail string    `json:"personal_email"`
		Full          string    `json:"full_name"`
		Given         string    `json:"given"`
		Family        string    `json:"family"`
		Business      string    `json:"business_name"`
		Title         string    `json:"title"`
		Hired         *string   `json:"hired_on"`
		Terminated    *string   `json:"terminated_on"`
		Active        bool      `json:"active"`
		Rate          *int64    `json:"rate"`
		Currency      string    `json:"currency"`
		Per           string    `json:"pay_per"`
	}
	type run struct {
		Record        uuid.UUID `json:"record"`
		SourceID      string    `json:"source_id"`
		Company       string    `json:"company_source"`
		Start         *string   `json:"period_start"`
		End           *string   `json:"period_end"`
		Check         *string   `json:"check_date"`
		Processed     bool      `json:"processed"`
		OffCycle      bool      `json:"off_cycle"`
		External      bool      `json:"external"`
		Gross         *int64    `json:"gross"`
		Net           *int64    `json:"net"`
		Taxes         *int64    `json:"taxes"`
		EmployerTaxes *int64    `json:"employer_taxes"`
		Currency      string    `json:"currency"`
	}
	type pay struct {
		Record        uuid.UUID `json:"record"`
		Worker        string    `json:"worker_source"`
		Paid          *string   `json:"paid_on"`
		Gross         *int64    `json:"gross"`
		Net           *int64    `json:"net"`
		Deductions    *int64    `json:"deductions"`
		Taxes         *int64    `json:"taxes"`
		EmployerTaxes *int64    `json:"employer_taxes"`
		Currency      string    `json:"currency"`
		Method        string    `json:"payment_method"`
	}
	date := func(s string) *string {
		if s == "" {
			return nil
		}
		return &s
	}
	currency := func(ms ...connect.Money) string {
		for _, m := range ms {
			if m.Minor != nil {
				return m.Currency
			}
		}
		return ""
	}
	var (
		mails     []mail
		parties   = map[uuid.UUID][]party{}
		docs      []doc
		companies []company
		workers   []worker
		runs      []run
		pays      []pay
		payOf     []uuid.UUID
		emails    []string
		eins      []string
		none      []uuid.UUID
	)
	for _, r := range rows {
		m := s.RowsOf(r.Data)
		if m == nil {
			none = append(none, r.Record)
			continue
		}
		if x := m.Mail; x != nil {
			v := mail{Record: r.Record, Thread: x.Thread, Subject: x.Subject, Snippet: x.Snippet, Labels: x.Labels, Attachments: x.Attachments}
			if !x.Sent.IsZero() {
				v.Sent = &x.Sent
			}
			mails = append(mails, v)
			for _, p := range x.Parties {
				if e := connect.Email(p.Email); e != "" {
					emails = append(emails, e)
					parties[r.Record] = append(parties[r.Record], party{Email: e, Role: p.Role, Name: p.Name})
				}
			}
		}
		if x := m.Doc; x != nil {
			v := doc{Record: r.Record, Kind: x.Kind, Title: x.Title, URL: x.URL, Icon: x.Icon}
			if !x.Edited.IsZero() {
				v.Edited = &x.Edited
			}
			docs = append(docs, v)
		}
		if x := m.Company; x != nil {
			ein := connect.EIN(x.EIN)
			if ein != "" {
				eins = append(eins, ein)
			}
			companies = append(companies, company{Record: r.Record, SourceID: r.SourceID, EIN: ein, Name: x.Name, TradeName: x.TradeName, EntityType: x.EntityType})
		}
		if x := m.Worker; x != nil {
			e, pe := connect.Email(x.Email), connect.Email(x.PersonalEmail)
			if e == "" {
				e, pe = pe, ""
			}
			if pe == e {
				pe = ""
			}
			if e != "" {
				emails = append(emails, e)
			}
			full := x.Given
			if x.Family != "" {
				full = x.Given + " " + x.Family
			}
			workers = append(workers, worker{Record: r.Record, SourceID: r.SourceID, Kind: x.Kind, Company: x.Company, Email: e, PersonalEmail: pe, Full: full, Given: x.Given,
				Family: x.Family, Business: x.Business, Title: x.Title, Hired: date(x.Hired), Terminated: date(x.Terminated), Active: x.Active,
				Rate: x.Rate.Minor, Currency: x.Rate.Currency, Per: x.Per})
		}
		if x := m.Run; x != nil {
			runs = append(runs, run{Record: r.Record, SourceID: r.SourceID, Company: x.Company, Start: date(x.Start), End: date(x.End), Check: date(x.Check),
				Processed: x.Processed, OffCycle: x.OffCycle, External: x.External, Gross: x.Gross.Minor, Net: x.Net.Minor, Taxes: x.Taxes.Minor,
				EmployerTaxes: x.EmployerTaxes.Minor, Currency: currency(x.Gross, x.Net, x.Taxes, x.EmployerTaxes)})
		}
		if m.Run != nil || len(m.Pays) > 0 {
			payOf = append(payOf, r.Record)
		}
		// A worker paid twice in one record is paid as it says last.
		seen := map[string]bool{}
		for i := len(m.Pays) - 1; i >= 0; i-- {
			x := m.Pays[i]
			if x.Worker == "" || seen[x.Worker] {
				continue
			}
			seen[x.Worker] = true
			pays = append(pays, pay{Record: r.Record, Worker: x.Worker, Paid: date(x.Paid), Gross: x.Gross.Minor, Net: x.Net.Minor,
				Deductions: x.Deductions.Minor, Taxes: x.Taxes.Minor, EmployerTaxes: x.EmployerTaxes.Minor,
				Currency: currency(x.Gross, x.Net, x.Deductions, x.Taxes, x.EmployerTaxes), Method: x.Method})
		}
	}
	if len(none) > 0 {
		if err := q.ClearRecordRows(ctx, none); err != nil {
			return err
		}
	}
	if len(emails) > 0 {
		slices.Sort(emails)
		if err := q.SaveKnownPeople(ctx, slices.Compact(emails)); err != nil {
			return err
		}
	}
	if len(eins) > 0 {
		slices.Sort(eins)
		if err := q.SaveKnownOrgs(ctx, slices.Compact(eins)); err != nil {
			return err
		}
	}
	if len(mails) > 0 {
		saved, err := q.SaveMail(ctx, id, jsonOf(mails))
		if err != nil {
			return fmt.Errorf("mail: %w", err)
		}
		all, messages := []party{}, make([]uuid.UUID, len(saved))
		for i, m := range saved {
			messages[i] = m.ID
			for _, p := range parties[m.Record] {
				p.Message = m.ID
				all = append(all, p)
			}
		}
		if err := q.SaveMailParties(ctx, jsonOf(all), messages); err != nil {
			return fmt.Errorf("mail parties: %w", err)
		}
	}
	if len(docs) > 0 {
		if err := q.SaveDocPages(ctx, id, jsonOf(docs)); err != nil {
			return fmt.Errorf("doc pages: %w", err)
		}
	}
	// Companies before the workers and payrolls they link, and payrolls before the pays in them.
	if len(companies) > 0 {
		if err := q.SavePayrollCompanies(ctx, id, jsonOf(companies)); err != nil {
			return fmt.Errorf("payroll companies: %w", err)
		}
	}
	if len(workers) > 0 {
		if err := q.SavePayrollWorkers(ctx, id, jsonOf(workers)); err != nil {
			return fmt.Errorf("payroll workers: %w", err)
		}
	}
	if len(runs) > 0 {
		if err := q.SavePayrollRuns(ctx, id, jsonOf(runs)); err != nil {
			return fmt.Errorf("payroll runs: %w", err)
		}
	}
	if len(payOf) > 0 {
		if err := q.SavePayrollPays(ctx, id, jsonOf(pays), payOf); err != nil {
			return fmt.Errorf("payroll pays: %w", err)
		}
	}
	return nil
}

func jsonOf(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// syncCleared deletes the rows of the records ids, which the query that deleted them returned with err, and
// the people and organizations nothing names any more.
func syncCleared(ctx context.Context, q *generated.Queries, ids []uuid.UUID, err error) error {
	if err != nil || len(ids) == 0 {
		return err
	}
	if err := q.ClearRecordRows(ctx, ids); err != nil {
		return err
	}
	return q.DeleteUnknown(ctx)
}

// syncFill writes the records connection c's streams synced before into Archive's tables, a batch at a time
// until it is done or budget has passed, and keeps where it is in each stream's cursor, so the next page goes
// on from there.
func (a *Archive) syncFill(ctx context.Context, c generated.ArchiveConnection, streams []*connect.Stream, budget time.Duration) error {
	end := time.Now().Add(budget)
	for _, s := range streams {
		if s.Rows == nil || syncStateOf(c.Cursor).of(s.Name).Rows >= syncRowsVersion {
			continue
		}
		for done := false; !done; {
			if time.Now().After(end) {
				return nil
			}
			err := a.sys(ctx, func(ctx context.Context) error {
				q := a.st.db(ctx)
				c2, err := q.ConnectionLocked(ctx, c.ID)
				if err != nil {
					return err
				}
				st := syncStateOf(c2.Cursor)
				v := st.of(s.Name)
				if v.Rows >= syncRowsVersion {
					done = true
					return nil
				}
				recs, err := q.StreamRecords(ctx, generated.StreamRecordsParams{Connection: c.ID, Stream: s.Name, After: v.RowsAfter, N: syncFillBatch})
				if err != nil {
					return err
				}
				rows := make([]syncRow, len(recs))
				for i, r := range recs {
					rows[i] = syncRow{Record: r.ID, SourceID: r.SourceID, Data: r.Data}
				}
				if err := syncRows(ctx, q, c.ID, s, rows); err != nil {
					return fmt.Errorf("writing %s into tables: %w", s.Name, err)
				}
				if len(recs) < syncFillBatch {
					v.Rows, v.RowsAfter, done = syncRowsVersion, "", true
				} else {
					v.RowsAfter = recs[len(recs)-1].SourceID
				}
				cursor, _ := json.Marshal(st)
				return q.SaveSyncCursor(ctx, c.ID, cursor)
			})
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}
