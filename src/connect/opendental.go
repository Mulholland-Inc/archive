package connect

import "time"

func init() {
	Register(&Connector{
		ID: "opendental", Name: "Open Dental", Integration: true, Env: "ARCHIVE_OPENDENTAL", Host: "https://api.opendental.com",
		Info: Info{By: "Open Dental", Description: "Read the practice’s patients, appointments, procedures and payments", Category: CatDental, Icon: IconUsers, Scopes: []string{"Read practice data with the practice’s customer key"}},
		API:  "https://api.opendental.com/api/v1",
		Pace: 250 * time.Millisecond,
		Keys: []Key{{Name: "customer_key", Label: "Customer key", Secret: true,
			Help: "In Open Dental, Setup, Advanced Setup, API: add Mulholland's Archive and copy the customer key it makes"}},
		Sign: map[string]string{"Authorization": "ODFHIR {client/id}/{key/customer_key}"},
		Who:  &Who{Req: Req{URL: "/preferences?PrefName=PracticeTitle"}, Names: []Path{"*/ValueString"}},
		Actions: map[string]*Action{
			"get": Call[odGet, any](Plan{
				Doc: "Read any of Open Dental's GET endpoints, like { path: \"/appointments\", query: \"dateStart=2026-11-01&dateEnd=2026-11-07\" } " +
					"or { path: \"/patients/12\" }: its JSON answer. Lists come 100 at a time; pass Offset=100 and on for more",
				Steps: []Step{{Req: Req{URL: "{+path}?{+query|}"}}},
			}),
		},
		Sync: []*Sync{
			{Name: "providers", Gap: time.Second, Req: Req{URL: "/providers", Query: odAt}, ID: "ProvNum", Page: odPage, Deny: odDeny,
				Doc: "Every provider, hidden ones too, by its ProvNum: listed whole each sync, 100 a page.",
				To: Map{Table: "dental_provider",
					Cols: map[string]SQL{"abbr": "data->>'Abbr'", "first_name": "data->>'FName'", "last_name": "data->>'LName'", "suffix": "data->>'Suffix'", "specialty": "data->>'Specialty'",
						"npi": "data->>'NationalProvID'", "hidden": "lower(data->>'IsHidden') = 'true'", "edited_at": "archive.sync_clock(data->>'DateTStamp')"}}},
			{Name: "patients", Gap: time.Second, Req: Req{URL: "/patients/Simple", Query: odAt}, ID: "PatNum", Page: odPage, Deny: odDeny,
				Change: SinceTime{Req: Req{URL: "/patients/Simple", Query: Q{"DateTStamp": "{since}", "Offset": "{page}"}}, Stamp: "DateTStamp"},
				Doc:    "Every patient, with their name, status, birthdate, contact details, address, primary provider and balance, by its PatNum: listed whole the first time and a week after, 100 a page, which deletes those gone, and in between those whose DateTStamp is on or after the newest read before.",
				To: Map{Table: "dental_patient",
					Cols: map[string]SQL{"first_name": "data->>'FName'", "last_name": "data->>'LName'", "middle_initial": "data->>'MiddleI'", "preferred_name": "data->>'Preferred'",
						"status": "data->>'PatStatus'", "gender": "data->>'Gender'", "birthdate": "nullif(archive.sync_day(data->>'Birthdate'), '0001-01-01')", "email": "archive.sync_email(data->>'Email')",
						"mobile_phone": "data->>'WirelessPhone'", "home_phone": "data->>'HmPhone'", "work_phone": "data->>'WkPhone'", "address": "btrim(concat(data->>'Address', ' ', data->>'Address2'))",
						"city": "data->>'City'", "state": "data->>'State'", "zip": "data->>'Zip'", "first_visit_on": "nullif(archive.sync_day(data->>'DateFirstVisit'), '0001-01-01')",
						"balance": "coalesce(archive.sync_num(data->>'BalTotal'), archive.sync_num(data->>'EstBalance'))", "edited_at": "archive.sync_clock(data->>'DateTStamp')"},
					People: map[string]SQL{"person": "data->>'Email'"},
					Refs:   map[string]SQL{"provider": "nullif(data->>'PriProv', '0')"}}},
			{Name: "appointments", Gap: time.Second, Req: Req{URL: "/appointments", Query: odAt}, ID: "AptNum", Page: odPage, Deny: odDeny,
				Change: SinceTime{Req: Req{URL: "/appointments", Query: Q{"DateTStamp": "{since}", "Offset": "{page}"}}, Stamp: "DateTStamp"},
				Doc:    "Every appointment, with its patient, providers, time, length, status, confirmation, procedures and note, by its AptNum: listed whole the first time and a week after, 100 a page, which deletes those gone, and in between those whose DateTStamp is on or after the newest read before.",
				To: Map{Table: "dental_appointment",
					Cols: map[string]SQL{"starts_at": "archive.sync_clock(data->>'AptDateTime')", "minutes": "5 * nullif(length(data->>'Pattern'), 0)", "status": "data->>'AptStatus'",
						"confirmed": "coalesce(nullif(data->>'confirmed', ''), data->>'Confirmed')", "procedures": "data->>'ProcDescript'", "note": "data->>'Note'",
						"new_patient": "lower(data->>'IsNewPatient') = 'true'", "operatory": "nullif(data->>'Op', '0')", "edited_at": "archive.sync_clock(data->>'DateTStamp')"},
					Refs: map[string]SQL{"patient": "nullif(data->>'PatNum', '0')", "provider": "nullif(data->>'ProvNum', '0')", "hygienist": "nullif(data->>'ProvHyg', '0')"}}},
			{Name: "procedures", Gap: time.Second, Req: Req{URL: "/procedurelogs", Query: odAt}, ID: "ProcNum", Page: odPage, Deny: odDeny,
				Change: SinceTime{Req: Req{URL: "/procedurelogs", Query: Q{"DateTStamp": "{since}", "Offset": "{page}"}}, Stamp: "DateTStamp"},
				Doc:    "Every procedure, planned, done or existing, with its patient, appointment, provider, code, status, date, fee, tooth and surfaces, by its ProcNum: listed whole the first time and a week after, 100 a page, which deletes those gone, and in between those whose DateTStamp is on or after the newest read before.",
				To: Map{Table: "dental_procedure",
					Cols: map[string]SQL{"code": "coalesce(nullif(data->>'procCode', ''), data->>'ProcCode')", "description": "coalesce(nullif(data->>'descript', ''), data->>'Descript')",
						"status": "data->>'ProcStatus'", "on_date": "nullif(archive.sync_day(data->>'ProcDate'), '0001-01-01')", "fee": "archive.sync_num(data->>'ProcFee')", "tooth": "data->>'ToothNum'",
						"surface": "data->>'Surf'", "edited_at": "archive.sync_clock(data->>'DateTStamp')"},
					Refs: map[string]SQL{"patient": "nullif(data->>'PatNum', '0')", "appointment": "nullif(data->>'AptNum', '0')", "provider": "nullif(data->>'ProvNum', '0')"}}},
			{Name: "payments", Gap: time.Second, Req: Req{URL: "/payments", Query: odAt}, ID: "PayNum", Page: odPage, Deny: odDeny,
				Change: SinceTime{Req: Req{URL: "/payments", Query: Q{"DateEntry": "{since}", "Offset": "{page}"}}, Stamp: "DateEntry"},
				Doc:    "Every payment, with its patient, date, amount, type and note, by its PayNum: listed whole the first time and a week after, 100 a page, which deletes those gone, and in between those whose DateEntry is on or after the newest read before.",
				To: Map{Table: "dental_payment",
					Cols: map[string]SQL{"paid_on": "nullif(archive.sync_day(data->>'PayDate'), '0001-01-01')", "amount": "archive.sync_num(data->>'PayAmt')", "type": "coalesce(nullif(data->>'payType', ''), data->>'PayType')",
						"check_number": "data->>'CheckNum'", "note": "data->>'PayNote'", "entered_on": "nullif(archive.sync_day(data->>'DateEntry'), '0001-01-01')"},
					Refs: map[string]SQL{"patient": "nullif(data->>'PatNum', '0')"}}},
		},
	})
}

var (
	odAt   = Q{"Offset": "{page}"}
	odPage = Offset{Size: 100, Step: 100}
	odDeny = []Deny{{Status: []int{401, 403}, Says: []string{"permission"}, Why: "Open Dental does not let Archive read {stream}: {message}"}}
)

type odGet struct {
	Path  string `json:"path" jsonschema:"The endpoint after /api/v1, like /patients, /appointments, /procedurelogs, /claims or /payments"`
	Query string `json:"query,omitempty" jsonschema:"Its query, like PatNum=12&Offset=100"`
}
