package connect

// Besides its copy in archive.record, what a stream syncs lands in Archive's own tables (archive.sql): a
// stream's Rows says what one of its records is there, as rows of the domain it belongs to, like a Gmail
// message as a message of mail, and Archive writes them with the record and deletes them with it. The tables
// are the same for every connector of a domain, so Gmail and Outlook share mail, and Gusto and any other payroll
// share payroll: a connector fills in the types below from its records, and knows nothing of SQL.
//
// People and organizations are shared by every domain, by a key that is the same wherever they are seen: a
// person is an email address, an organization an EIN. A Gmail sender and a Gusto employee with one address are
// one person. The rest of a person, like their name, stays with the record that said it, since another member
// may not see that record.

import (
	"encoding/json"
	"math/big"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Rows are what one record is in Archive's tables; each is nil, or empty, when the record is none of it.
type Rows struct {
	Mail    *Mail
	Doc     *Doc
	Company *PayrollCompany
	Worker  *PayrollWorker
	Run     *PayrollRun
	Pays    []PayrollPay
}

// RowsOf is what the record data, as archive.record keeps it, is in Archive's tables; nil when s has no Rows.
func (s *Stream) RowsOf(data []byte) *Rows {
	if s.Rows == nil {
		return nil
	}
	var m map[string]any
	if json.Unmarshal(data, &m) != nil || m == nil {
		return nil
	}
	return s.Rows(m)
}

// Mail is a message of a mailbox.
type Mail struct {
	Thread, Subject, Snippet string
	Sent                     time.Time
	Labels                   []string
	Attachments              int
	// Parties are who it is from and to, by role: from, to, cc, bcc or reply_to.
	Parties []MailParty
}

// MailParty is someone a message names, in one of its roles.
type MailParty struct {
	Role string
	Address
}

// Address is a person by their email address, with the name they were given there.
type Address struct{ Email, Name string }

// Doc is a page of docs or a wiki, like a Notion page.
type Doc struct {
	Kind, Title, URL, Icon string
	Edited                 time.Time
}

// PayrollCompany is a company as a payroll provider has it: the employer. EIN is its US employer
// identification number, which is the organization it is.
type PayrollCompany struct {
	Name, TradeName, EntityType, EIN string
}

// PayrollWorker is someone a company pays: an employee or a contractor (Kind). Company is the id there of the
// company, Email the address the person is, WorkEmail first. Dates are YYYY-MM-DD; Rate is per Per, like Hour
// or Year.
type PayrollWorker struct {
	Kind, Company                                 string
	Given, Family, Business, Email, PersonalEmail string
	Title, Hired, Terminated                      string
	Active                                        bool
	Rate                                          Money
	Per                                           string
}

// PayrollRun is a payroll: a company paying its people for a pay period, on its check date.
type PayrollRun struct {
	Company                          string
	Start, End, Check                string
	Processed, OffCycle, External    bool
	Gross, Net, EmployerTaxes, Taxes Money
}

// PayrollPay is what one worker was paid, in a payroll or on its own, like a contractor payment. Worker is the
// worker's id there.
type PayrollPay struct {
	Worker, Paid, Method                         string
	Gross, Net, Deductions, Taxes, EmployerTaxes Money
}

// Money is an amount in a currency's minor units, nil when there is none.
type Money struct {
	Minor    *int64
	Currency string
}

// money is the amount v, a JSON number or a string in major units like "1234.56", in currency, which has
// two decimals; none when v is neither.
func money(v any, currency string) Money {
	var s string
	switch x := v.(type) {
	case string:
		s = strings.TrimSpace(x)
	case float64:
		s = strconv.FormatFloat(x, 'f', -1, 64)
	default:
		return Money{}
	}
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return Money{}
	}
	n, err := strconv.ParseInt(r.Mul(r, big.NewRat(100, 1)).FloatString(0), 10, 64)
	if err != nil {
		return Money{}
	}
	return Money{Minor: &n, Currency: currency}
}

// sum is the amounts added up, nil when none is there.
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
func addresses(header string) []Address {
	if strings.TrimSpace(header) == "" {
		return nil
	}
	list, err := (&mail.AddressParser{}).ParseList(header)
	if err != nil {
		if a, err := mail.ParseAddress(header); err == nil {
			list = []*mail.Address{a}
		}
	}
	var out []Address
	for _, a := range list {
		if e := Email(a.Address); e != "" {
			out = append(out, Address{Email: e, Name: a.Name})
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

// day is s cut to its date, YYYY-MM-DD, or "" when it does not start with one.
func day(s string) string {
	if t, err := time.Parse(time.DateOnly, clip(s, 10)); err == nil {
		return t.Format(time.DateOnly)
	}
	return ""
}
