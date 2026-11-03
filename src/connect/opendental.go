package connect

import (
	"cmp"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Open Dental, the practice management system of dental offices, through its API, read only. It is a key
// connector with a key of the server's own: ARCHIVE_OPENDENTAL is https://<developer key>@api.opendental.com,
// the developer key Open Dental's Vendor Relations issues Mulholland, and a practice's owner pastes in the
// customer key the practice makes for it in Open Dental (Setup, Advanced Setup, API). Both go in one header,
// ODFHIR <developer key>/<customer key>. Another host in the variable, like a practice's own API Service,
// stands in for Open Dental's.
//
// What it syncs is health information under the practice's agreement with Mulholland: its providers, patients,
// appointments, procedures and payments, into the dental tables. An owner should narrow the connection to the
// group that may see them. Open Dental lists 100 a page, by offset, and lists what changed since a time it
// keeps in the practice's own clock, DateTStamp; a stream asks from the newest stamp it has seen, never from
// Archive's clock, so no time zone hides a change. Each is listed whole the first time and a week after, which
// deletes what is gone, like a deleted appointment; providers, which are few, each sync. Open Dental throttles
// each practice's key, so requests go at most four a second, and a 429 rests them all.
func init() {
	Register(&Connector{
		ID: "opendental", Name: "Open Dental", Integration: true, Env: "ARCHIVE_OPENDENTAL", Host: "https://api.opendental.com",
		Pace: 250 * time.Millisecond,
		Keys: []Key{{Name: "customer_key", Label: "Customer key", Secret: true,
			Help: "In Open Dental, Setup, Advanced Setup, API: add Mulholland's Archive and copy the customer key it makes"}},
		Authorize: func(r *http.Request, c *Client) {
			dev, _ := c.Credentials()
			r.Header.Set("Authorization", "ODFHIR "+dev+"/"+c.Key("customer_key"))
		},
		Account: func(c *Client) (string, error) {
			var prefs []map[string]any
			if err := c.Get(odAPI+"/preferences?PrefName=PracticeTitle", &prefs); err != nil {
				return "", err
			}
			for _, p := range prefs {
				if s := strings.TrimSpace(str(p, "ValueString")); s != "" {
					return s, nil
				}
			}
			return "Open Dental", nil
		},
		Actions: map[string]*Action{
			"get": Act("Read any of Open Dental's GET endpoints, like { path: \"/appointments\", query: \"dateStart=2026-11-01&dateEnd=2026-11-07\" } "+
				"or { path: \"/patients/12\" }: its JSON answer. Lists come 100 at a time; pass Offset=100 and on for more",
				func(c *Client, in struct {
					Path  string `json:"path" jsonschema:"The endpoint after /api/v1, like /patients, /appointments, /procedurelogs, /claims or /payments"`
					Query string `json:"query,omitempty" jsonschema:"Its query, like PatNum=12&Offset=100"`
				}) (any, error) {
					if !strings.HasPrefix(in.Path, "/") {
						return nil, errors.New("path starts with /, like /patients")
					}
					u := odAPI + in.Path
					if in.Query != "" {
						u += "?" + in.Query
					}
					var out any
					err := c.Get(u, &out)
					return out, err
				}),
		},
		Streams: []*Stream{
			odStream("providers", "/providers", "ProvNum", "", "dental_provider", odProviderRows, "Every provider, hidden ones too"),
			odStream("patients", "/patients/Simple", "PatNum", "DateTStamp", "dental_patient", odPatientRows,
				"Every patient, with their name, status, birthdate, contact details, address, primary provider and balance"),
			odStream("appointments", "/appointments", "AptNum", "DateTStamp", "dental_appointment", odAppointmentRows,
				"Every appointment, with its patient, providers, time, length, status, confirmation, procedures and note"),
			odStream("procedures", "/procedurelogs", "ProcNum", "DateTStamp", "dental_procedure", odProcedureRows,
				"Every procedure, planned, done or existing, with its patient, appointment, provider, code, status, date, fee, tooth and surfaces"),
			odStream("payments", "/payments", "PayNum", "DateEntry", "dental_payment", odPaymentRows,
				"Every payment, with its patient, date, amount, type and note"),
		},
	})
}

const odAPI = "https://api.opendental.com/api/v1"

// odPage is how many Open Dental lists a page.
const odPage = 100

// odCursor is where a stream is: Since the newest stamp it has read, in the practice's clock, Listed when its
// last full listing began, in Archive's, and while a sync is under way, Mode list or delta, At its next offset
// and Max the newest stamp it has read in this sync.
type odCursor struct {
	Since  string `json:"since,omitempty"`
	Listed string `json:"listed,omitempty"`
	Mode   string `json:"mode,omitempty"`
	At     int    `json:"at,omitempty"`
	Max    string `json:"max,omitempty"`
}

// odStream is the stream of Open Dental's list path, by the key field of its records, into table: by what
// changed since the newest of their stamp field it has read, which goes as the parameter of that name, or
// listed whole each sync when stamp is "".
func odStream(name, path, key, stamp, table string, rows func(map[string]any) *Row, what string) *Stream {
	doc := what + ", by its " + key + ": listed whole each sync, 100 a page."
	if stamp != "" {
		doc = what + ", by its " + key + ": listed whole the first time and a week after, 100 a page, which deletes those gone, and in " +
			"between those whose " + stamp + " is on or after the newest read before."
	}
	return &Stream{Name: name, Gap: time.Second, Table: table, Rows: rows, Doc: doc,
		Pull: func(c *Client, cursor string) (*Page, error) {
			var cur odCursor
			if err := cursorOf(cursor, &cur); err != nil {
				return nil, err
			}
			if cur.Mode == "" {
				cur.Mode, cur.At, cur.Max = "delta", 0, cur.Since
				if stamp == "" || cur.Since == "" || time.Since(stamp2(cur.Listed)) > 7*24*time.Hour {
					cur.Mode = "list"
					cur.Listed = time.Now().UTC().Format(time.RFC3339)
				}
			}
			q := url.Values{}
			if cur.At > 0 {
				q.Set("Offset", fmt.Sprint(cur.At))
			}
			if cur.Mode == "delta" {
				q.Set(stamp, cur.Since)
			}
			var items []map[string]any
			if err := c.Get(odAPI+path+"?"+q.Encode(), &items); err != nil {
				if r := (*Refusal)(nil); errors.As(err, &r) && (r.Status == http.StatusUnauthorized || r.Status == http.StatusForbidden) &&
					strings.Contains(strings.ToLower(r.Message), "permission") {
					return nil, unsupported("Open Dental does not let Archive read %s: %s", name, r.Message)
				}
				return nil, err
			}
			page := &Page{Full: cur.Mode == "list"}
			for _, m := range items {
				if id := str(m, key); id != "" {
					page.Records = append(page.Records, Record{ID: id, Data: m})
				}
				if s := str(m, stamp); stamp != "" && s > cur.Max {
					cur.Max = s
				}
			}
			if len(items) >= odPage {
				cur.At += len(items)
				page.More = true
			} else {
				cur = odCursor{Since: cmp.Or(cur.Max, cur.Since), Listed: cur.Listed}
			}
			page.Cursor = cursorFor(cur)
			return page, nil
		}}
}

// stamp2 is an RFC 3339 time, zero when it is none.
func stamp2(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// odRef is the id a field names, "" for 0, which is none.
func odRef(m map[string]any, k string) string {
	if v := str(m, k); v != "0" {
		return v
	}
	return ""
}

// odDay is a date, or nil for Open Dental's none, 0001-01-01.
func odDay(s string) any {
	if strings.HasPrefix(s, "0001-01-01") {
		return nil
	}
	return day(s)
}

// odTime is a time in the practice's clock, yyyy-MM-dd HH:mm:ss, as it is, or nil for none.
func odTime(s string) any {
	if _, err := time.Parse(time.DateTime, s); err != nil || strings.HasPrefix(s, "0001-01-01") {
		return nil
	}
	return s
}

// odBool is a field Open Dental sends as a boolean or as "true" or "false".
func odBool(m map[string]any, k string) any {
	switch v := m[k].(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(v, "true")
	}
	return nil
}

func odProviderRows(m map[string]any) *Row {
	return rowOf(map[string]any{"abbr": str(m, "Abbr"), "first_name": str(m, "FName"), "last_name": str(m, "LName"), "suffix": str(m, "Suffix"),
		"specialty": str(m, "Specialty"), "npi": str(m, "NationalProvID"), "hidden": odBool(m, "IsHidden"), "edited_at": odTime(str(m, "DateTStamp"))})
}

// odPatientRows is a patient, the person of their email, with their primary provider.
func odPatientRows(m map[string]any) *Row {
	email := Email(str(m, "Email"))
	addr := strings.TrimSpace(str(m, "Address") + " " + str(m, "Address2"))
	return rowOf(map[string]any{"first_name": str(m, "FName"), "last_name": str(m, "LName"), "middle_initial": str(m, "MiddleI"),
		"preferred_name": str(m, "Preferred"), "status": str(m, "PatStatus"), "gender": str(m, "Gender"), "birthdate": odDay(str(m, "Birthdate")),
		"email": email, "mobile_phone": str(m, "WirelessPhone"), "home_phone": str(m, "HmPhone"), "work_phone": str(m, "WkPhone"),
		"address": addr, "city": str(m, "City"), "state": str(m, "State"), "zip": str(m, "Zip"), "first_visit_on": odDay(str(m, "DateFirstVisit")),
		"balance": decimal(cmp.Or(num(m["BalTotal"]), num(m["EstBalance"]))), "edited_at": odTime(str(m, "DateTStamp"))}).
		person("person", email).ref("provider", odRef(m, "PriProv"))
}

// odAppointmentRows is an appointment: its length is its pattern's, five minutes a character.
func odAppointmentRows(m map[string]any) *Row {
	var minutes any
	if p := str(m, "Pattern"); p != "" {
		minutes = 5 * len(p)
	}
	return rowOf(map[string]any{"starts_at": odTime(str(m, "AptDateTime")), "minutes": minutes, "status": str(m, "AptStatus"),
		"confirmed": cmp.Or(str(m, "confirmed"), str(m, "Confirmed")), "procedures": str(m, "ProcDescript"), "note": str(m, "Note"),
		"new_patient": odBool(m, "IsNewPatient"), "operatory": odRef(m, "Op"), "edited_at": odTime(str(m, "DateTStamp"))}).
		ref("patient", odRef(m, "PatNum")).ref("provider", odRef(m, "ProvNum")).ref("hygienist", odRef(m, "ProvHyg"))
}

func odProcedureRows(m map[string]any) *Row {
	return rowOf(map[string]any{"code": cmp.Or(str(m, "procCode"), str(m, "ProcCode")), "description": cmp.Or(str(m, "descript"), str(m, "Descript")),
		"status": str(m, "ProcStatus"), "on_date": odDay(str(m, "ProcDate")), "fee": decimal(m["ProcFee"]), "tooth": str(m, "ToothNum"),
		"surface": str(m, "Surf"), "edited_at": odTime(str(m, "DateTStamp"))}).
		ref("patient", odRef(m, "PatNum")).ref("appointment", odRef(m, "AptNum")).ref("provider", odRef(m, "ProvNum"))
}

func odPaymentRows(m map[string]any) *Row {
	return rowOf(map[string]any{"paid_on": odDay(str(m, "PayDate")), "amount": decimal(m["PayAmt"]), "type": cmp.Or(str(m, "payType"), str(m, "PayType")),
		"check_number": str(m, "CheckNum"), "note": str(m, "PayNote"), "entered_on": odDay(str(m, "DateEntry"))}).ref("patient", odRef(m, "PatNum"))
}
