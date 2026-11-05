package connect

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

func init() {
	Register(&Connector{
		ID: "dentrix", Name: "Dentrix", Integration: true, Env: "ARCHIVE_DENTRIX", Host: "https://nexhealth.info",
		Info: Info{By: "NexHealth", Description: "Read the practice’s patients, appointments, procedures and payments", Category: CatDental, Icon: IconUsers,
			Scopes: []string{"Read practice data through NexHealth with the practice’s access code"}},
		API:  "https://nexhealth.info",
		Pace: 125 * time.Millisecond,
		Keys: []Key{{Name: "code", Label: "Access code", Secret: true,
			Help: "The code Mulholland sends once NexHealth's Synchronizer is set up on your Dentrix, like acme/123/9f86d081884c7d65"}},
		Mint: []Step{
			{In: Obj{"secret": "{client/id}", "code": "{key/code}"}, Codec: dxSigned},
			{As: "key/subdomain", In: "{key/code}", Find: `^([^/]+)/`},
			{As: "key/location", In: "{key/code}", Find: `^[^/]+/([^/]+)/`},
			{As: "got", Req: Req{Method: POST, URL: "https://nexhealth.info/authenticates",
				Header: map[string][]string{"Authorization": {"{client/id}"}, "Nex-Api-Version": {dxVersion}}}},
			{As: "key/access", In: "{got/data/token}"},
			{Only: Test{At: "key/access", Off: true}, Fail: "NexHealth gave no token for Archive's API key"},
		},
		Sign: map[string]string{"Authorization": "Bearer {key/access}", "Nex-Api-Version": dxVersion},
		Pin:  Q{"subdomain": "{key/subdomain}", "location_id": "{key/location}"},
		Who:  &Who{Req: Req{URL: "/locations/{key/location}"}, Names: []Path{"data/name"}},
		Actions: map[string]*Action{
			"get": Call[dxGet, any](Plan{
				Doc: "Read one of NexHealth's GET endpoints for the practice's Dentrix, like { path: \"/appointments\", " +
					"query: \"start=2026-11-01T00:00:00Z&end=2026-11-08T00:00:00Z\" } or { path: \"/patients/12\" }: its answer, { data, page_info }. " +
					"Lists come 5 at a time unless per_page says, up to 1000; pass end_cursor=<page_info.end_cursor> for more. " +
					"The practice's subdomain and location_id are added",
				Steps: []Step{
					{Only: Test{At: "path", Re: dxPaths, Off: true}, Fail: "Dentrix through NexHealth reads only /patients, /providers, /appointments, /procedures, " +
						"/payments, /charges, /adjustments, /claims, /insurance_plans, /insurance_coverages, /treatment_plans, /patient_recalls, " +
						"/appointment_types, /operatories, /payment_types, /adjustment_types and /working_hours"},
					{Req: Req{URL: "{+path}?{+query|}"}},
				},
			}),
		},
		Sync: []*Sync{
			{Name: "providers", Gap: time.Second, Req: Req{URL: "/providers", Query: dxAt(dxInactive)}, Parts: dxBoth, Items: "data", Page: dxPage, Deny: dxDeny,
				Doc: "Every provider by its id, active and inactive, which NexHealth lists apart: listed whole each sync, 1,000 a page.",
				To: Map{Table: "dental_provider",
					Cols: map[string]SQL{"first_name": "data->>'first_name'", "last_name": "data->>'last_name'", "specialty": "data->>'specialty_code'",
						"npi": "data->>'npi'", "hidden": "lower(data->>'inactive') = 'true'", "edited_at": dxEdited}}},
			{Name: "patients", Gap: time.Second, Req: Req{URL: "/patients", Query: dxAt(dxInactive, Q{"location_strict": "true"})}, Parts: dxBoth, Items: "data", Page: dxPage, Deny: dxDeny,
				Change: SinceTime{Req: Req{URL: "/patients", Query: dxAt(dxInactive, Q{"location_strict": "true", "updated_since": "{since}"})}, Stamp: "updated_at"},
				Doc:    "Every patient of the location, active and inactive, which NexHealth lists apart, with their name, status, birthdate, contact details, address and primary provider, by its id: listed whole the first time and a week after, 1,000 a page, which deletes those gone, and in between those updated since the newest updated_at read before.",
				To: Map{Table: "dental_patient",
					Cols: map[string]SQL{"first_name": "data->>'first_name'", "last_name": "data->>'last_name'", "middle_initial": "nullif(left(data->>'middle_name', 1), '')",
						"status": "CASE WHEN lower(data->>'inactive') = 'true' THEN 'Inactive' WHEN lower(data#>>'{bio,non_patient}') = 'true' THEN 'NonPatient' ELSE 'Patient' END",
						"gender": "data#>>'{bio,gender}'", "birthdate": "archive.sync_day(data#>>'{bio,date_of_birth}')", "email": "archive.sync_email(data->>'email')",
						"mobile_phone": "coalesce(nullif(data#>>'{bio,cell_phone_number}', ''), data#>>'{bio,phone_number}')", "home_phone": "data#>>'{bio,home_phone_number}'",
						"work_phone": "data#>>'{bio,work_phone_number}'", "address": "nullif(btrim(concat(data#>>'{bio,address_line_1}', ' ', data#>>'{bio,address_line_2}')), '')",
						"city": "data#>>'{bio,city}'", "state": "data#>>'{bio,state}'", "zip": "data#>>'{bio,zip_code}'", "edited_at": dxEdited},
					People: map[string]SQL{"person": "data->>'email'"},
					Refs:   map[string]SQL{"provider": "nullif(data->>'provider_id', '0')"}}},
			{Name: "appointments", Gap: time.Second, Req: Req{URL: "/appointments", Query: dxAt(dxEver)}, Items: "data", Page: dxPage, Deny: dxDeny,
				Change: SinceTime{Req: Req{URL: "/appointments", Query: dxAt(dxEver, Q{"updated_since": "{since}"})}, Stamp: "updated_at"},
				Doc:    "Every appointment of the location, cancelled ones too, with its patient, provider, time, length, status, confirmation, chair and note, by its id: listed whole the first time and a week after, 1,000 a page, which deletes those gone, and in between those updated since the newest updated_at read before.",
				To: Map{Table: "dental_appointment",
					Cols: map[string]SQL{"starts_at": dxLocal("start_time"),
						"minutes":   "(extract(epoch FROM archive.sync_at(data->>'end_time') - archive.sync_at(data->>'start_time')) / 60)::int",
						"status":    "CASE WHEN lower(data->>'cancelled') = 'true' THEN 'Cancelled' WHEN lower(data->>'patient_missed') = 'true' THEN 'Missed' WHEN lower(data->>'checked_out') = 'true' THEN 'Complete' ELSE 'Scheduled' END",
						"confirmed": "CASE WHEN lower(data->>'confirmed') = 'true' THEN 'Confirmed' WHEN lower(data->>'confirmed') = 'false' THEN 'Unconfirmed' END",
						"note":      "data->>'note'", "operatory": "nullif(data->>'operatory_id', '0')", "edited_at": dxEdited},
					Refs: map[string]SQL{"patient": "nullif(data->>'patient_id', '0')", "provider": "nullif(data->>'provider_id', '0')"}}},
			{Name: "procedures", Gap: time.Second, Req: Req{URL: "/procedures", Query: dxAt(Q{"updated_since": dxStart})}, Items: "data", Page: dxPage, Deny: dxDeny,
				Change: SinceTime{Req: Req{URL: "/procedures", Query: dxAt(Q{"updated_since": "{since}"})}, Stamp: "updated_at"},
				Doc:    "Every procedure of the location, planned, scheduled, completed, existing, referred or inactive, with its patient, appointment, provider, code, date, fee, teeth and surface, by its id: listed whole the first time and a week after, 1,000 a page, which deletes those gone, and in between those updated since the newest updated_at read before.",
				To: Map{Table: "dental_procedure",
					Cols: map[string]SQL{"code": "data->>'code'", "description": "data->>'name'", "status": "data->>'status'",
						"on_date": "archive.sync_day(coalesce(data->>'start_date', data->>'end_date'))", "fee": "archive.sync_num(data#>>'{fee,amount}')",
						"tooth":   "CASE WHEN jsonb_typeof(data#>'{body_site,tooth}') = 'array' THEN nullif(array_to_string(ARRAY(SELECT jsonb_array_elements_text(data#>'{body_site,tooth}')), ','), '') ELSE data#>>'{body_site,tooth}' END",
						"surface": "data#>>'{body_site,surface}'", "edited_at": dxEdited},
					Refs: map[string]SQL{"patient": "nullif(data->>'patient_id', '0')", "appointment": "nullif(data->>'appointment_id', '0')", "provider": "nullif(data->>'provider_id', '0')"}}},
			{Name: "payments", Gap: time.Second, Req: Req{URL: "/payments", Query: dxAt(Q{"updated_since": dxStart})}, Items: "data", Page: dxPage, Deny: dxDeny,
				Change: SinceTime{Req: Req{URL: "/payments", Query: dxAt(Q{"updated_since": "{since}", "include_deleted": "true"})}, Stamp: "updated_at"},
				Gone:   []Test{{At: "deleted_at"}},
				Doc:    "Every payment of the location, with its patient, date, amount and description, by its id: listed whole the first time and a week after, 1,000 a page, which deletes those gone, and in between those updated since the newest updated_at read before, deleted ones too, which it deletes.",
				To: Map{Table: "dental_payment",
					Cols: map[string]SQL{"paid_on": "archive.sync_day(data->>'paid_at')", "amount": "archive.sync_num(data#>>'{payment_amount,amount}')", "note": "data->>'description'"},
					Refs: map[string]SQL{"patient": "nullif(data->>'patient_id', '0')"}}},
		},
	})
}

const (
	dxVersion = "v3.0.0"
	dxStart   = "1990-01-01T00:00:00Z"
	dxPaths   = `^/(patients|providers|appointments|procedures|payments|charges|adjustments|claims|insurance_plans|insurance_coverages|treatment_plans|patient_recalls|appointment_types|operatories|payment_types|adjustment_types|working_hours)(/[0-9]+)?$`
	dxEdited  = SQL("archive.sync_at(data->>'updated_at') AT TIME ZONE 'UTC'")
)

var (
	dxPage     = Cursor{At: "page_info/end_cursor", More: "page_info/has_next_page"}
	dxDeny     = []Deny{{Status: []int{403}, Why: "NexHealth does not let Archive read {stream} of this Dentrix: {message}"}}
	dxBoth     = &Parts{Fixed: []Obj{{"id": "false"}, {"id": "true"}}}
	dxInactive = Q{"inactive": "{part}"}
	dxEver     = Q{"start": dxStart, "end": "2100-01-01T00:00:00Z"}
)

func dxAt(qs ...Q) Q {
	q := Q{"per_page": "1000", "end_cursor": "{page}"}
	for _, x := range qs {
		for k, v := range x {
			q[k] = v
		}
	}
	return q
}

func dxLocal(key string) SQL {
	return SQL("CASE WHEN data->>'timezone' <> '' AND pg_input_is_valid('2000-01-01 00:00 ' || (data->>'timezone'), 'timestamptz') " +
		"THEN archive.sync_at(data->>'" + key + "') AT TIME ZONE (data->>'timezone') " +
		"ELSE archive.sync_clock(replace(left(data->>'" + key + "', 19), 'T', ' ')) END")
}

type dxGet struct {
	Path  string `json:"path" jsonschema:"The endpoint, like /patients, /appointments, /procedures, /payments, /charges, /claims or /patients/12"`
	Query string `json:"query,omitempty" jsonschema:"Its query, like patient_id=12&per_page=100"`
}

var dxSigned = &Codec{"dentrix_signed", func(v any) (any, error) {
	key, parts := text(pick(v, "secret")), strings.Split(strings.TrimSpace(text(pick(v, "code"))), "/")
	if len(parts) == 3 && parts[0] != "" && parts[1] != "" && key != "" {
		mac := hmac.New(sha256.New, []byte(key))
		mac.Write([]byte(parts[0] + "/" + parts[1]))
		if hmac.Equal([]byte(strings.ToLower(parts[2])), []byte(hex.EncodeToString(mac.Sum(nil))[:16])) {
			return nil, nil
		}
	}
	return nil, errors.New("that access code is not one Mulholland made; ask Mulholland for the practice's code")
}}
