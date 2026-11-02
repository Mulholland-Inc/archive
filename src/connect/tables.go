package connect

// What a stream syncs lands in Archive's own tables (archive.sql), one row a record: a stream names its Table,
// and its Rows says what columns a record fills there, as rows of the domain it belongs to, like a Gmail
// message as a message of mail. The tables are the same for every connector of a domain, so Gmail and Outlook
// share mail, and Gusto and any other payroll share payroll: a connector fills in Rows from its records, and
// knows nothing of SQL.
//
// People and organizations are shared by every domain, by a key that is the same wherever they are seen: a
// person is an email address, an organization an EIN. A Gmail sender and a Gusto employee with one address are
// one person. The rest of a person, like their name, stays on the rows that said it, since another member may
// not see those.

import (
	"encoding/json"
	"math/big"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Row is what a record is in its stream's table.
type Row struct {
	// Cols are its columns by name, as JSON has them: text, numbers, booleans, dates as YYYY-MM-DD, times as RFC
	// 3339, lists, and Money. One left out is null.
	Cols map[string]any
	// People are its columns that are a known_person, by their email address; Orgs those that are a known_org,
	// by EIN. Refs are its columns that are another row of the connection, by that row's id there: the column's
	// table says which table, and <column>_source keeps the id, so it links once that row is there too.
	People, Orgs, Refs map[string]string
	// Children are the rows of other tables that belong to it, like a message's parties: written again with it,
	// all of them, each time it is.
	Children []Child
}

// Child is a row of Table that belongs to a row of a stream's table.
type Child struct {
	Table string
	Row
}

// rowOf is a row of cols with its people, orgs and refs, as pairs of column and key, left out when the key is
// empty.
func rowOf(cols map[string]any) *Row {
	return &Row{Cols: cols, People: map[string]string{}, Orgs: map[string]string{}, Refs: map[string]string{}}
}

// person makes column col the person at the address email, when it is one.
func (r *Row) person(col, email string) *Row {
	if e := Email(email); e != "" {
		r.People[col] = e
	}
	return r
}

// ref makes column col the row of the connection whose id there is id.
func (r *Row) ref(col, id string) *Row {
	if id != "" {
		r.Refs[col] = id
	}
	return r
}

// child adds a row of table.
func (r *Row) child(table string, c *Row) *Row {
	r.Children = append(r.Children, Child{Table: table, Row: *c})
	return r
}

// RowOf is the row of the record data, as JSON, in s's table; nil when it is none.
func (s *Stream) RowOf(data []byte) *Row {
	var m map[string]any
	if json.Unmarshal(data, &m) != nil || m == nil {
		return nil
	}
	return s.Rows(m)
}

// Money is an amount in a currency's minor units; its JSON is archive.money's, null when there is none.
type Money struct {
	Minor    *int64
	Currency string
}

func (m Money) MarshalJSON() ([]byte, error) {
	if m.Minor == nil {
		return []byte("null"), nil
	}
	return json.Marshal(map[string]any{"amount": *m.Minor, "currency": m.Currency})
}

// money is the amount v, a JSON number or a string in major units like "1234.56", in currency, which has
// two decimals; none when v is neither.
func money(v any, currency string) Money {
	s := num(v)
	r, ok := new(big.Rat).SetString(s)
	if s == "" || !ok {
		return Money{}
	}
	n, err := strconv.ParseInt(r.Mul(r, big.NewRat(100, 1)).FloatString(0), 10, 64)
	if err != nil {
		return Money{}
	}
	return Money{Minor: &n, Currency: currency}
}

// num is v, a JSON number or a numeric string, as a decimal; "" when it is neither.
func num(v any) string {
	switch x := v.(type) {
	case string:
		x = strings.TrimSpace(x)
		if _, ok := new(big.Rat).SetString(x); ok {
			return x
		}
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	}
	return ""
}

// decimal is v as num has it, or nil, for a numeric column.
func decimal(v any) any {
	if s := num(v); s != "" {
		return s
	}
	return nil
}

// sum is the amounts added up, none when none is there.
func sum(ms ...Money) Money {
	var out Money
	for _, m := range ms {
		if m.Minor == nil {
			continue
		}
		if out.Minor == nil {
			out = Money{Minor: new(int64), Currency: m.Currency}
		}
		*out.Minor += *m.Minor
	}
	return out
}

var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+$`)

// Email is s as archive.email has an address, lowercase, or "" when it is none.
func Email(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if !emailRe.MatchString(s) {
		return ""
	}
	return s
}

// addresses are the addresses of a header like To, as a mail reader shows them; one that does not parse is
// left out.
func addresses(header string) []mail.Address {
	if strings.TrimSpace(header) == "" {
		return nil
	}
	list, err := (&mail.AddressParser{}).ParseList(header)
	if err != nil {
		if a, err := mail.ParseAddress(header); err == nil {
			list = []*mail.Address{a}
		}
	}
	var out []mail.Address
	for _, a := range list {
		if e := Email(a.Address); e != "" {
			out = append(out, mail.Address{Address: e, Name: a.Name})
		}
	}
	return out
}

var einRe = regexp.MustCompile(`^(0[1-6]|1[0-6]|2[0-7]|3[0-9]|4[0-8]|5[0-9]|6[0-8]|7[1-7]|8[0-8]|9[0-58-9])-[0-9]{7}$`)

// EIN is s as archive.us_ein has one, NN-NNNNNNN, or "" when it is none.
func EIN(s string) string {
	d := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
	if len(d) != 9 || !einRe.MatchString(d[:2]+"-"+d[2:]) {
		return ""
	}
	return d[:2] + "-" + d[2:]
}

// day is s cut to its date, YYYY-MM-DD, or nil when it does not start with one, for a date column.
func day(s string) any {
	if t, err := time.Parse(time.DateOnly, clip(s, 10)); err == nil {
		return t.Format(time.DateOnly)
	}
	return nil
}

// instant is s, RFC 3339 or a Unix time in milliseconds, or nil, for a timestamptz column.
func instant(s string) any {
	if t := stamp(s); !t.IsZero() {
		return t.UTC().Format(time.RFC3339Nano)
	}
	return nil
}

// strs are the strings of v, a JSON array.
func strs(v any) []string {
	a, _ := v.([]any)
	out := []string{}
	for _, e := range a {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// flag is v[k] when it is a boolean, or nil.
func flag(v map[string]any, k string) any {
	if b, ok := v[k].(bool); ok {
		return b
	}
	return nil
}

// obj is v[k] as an object, empty when it is none.
func obj(v map[string]any, k string) map[string]any {
	m, _ := v[k].(map[string]any)
	if m == nil {
		m = map[string]any{}
	}
	return m
}
