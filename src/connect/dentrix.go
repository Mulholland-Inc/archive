package connect

import (
	"cmp"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Dentrix, Henry Schein One's dental practice management system, on the practice's server (G-series) or in the cloud
// (Ascend), read only through NexHealth's Synchronizer API, which reads Dentrix for its developers without Henry
// Schein One's own developer program. It is a key connector with a key of the server's own: ARCHIVE_DENTRIX is
// https://<API key>@nexhealth.info, Mulholland's NexHealth API key, which POST /authenticates trades for a bearer
// token that lasts an hour, every call sending Nex-Api-Version v3.0.0. Mulholland adds each practice in NexHealth's
// developer portal, and the practice installs NexHealth's Synchronizer on its Dentrix server (or, on Ascend, gives
// NexHealth a user); NexHealth names it by a subdomain and a location id.
//
// One API key reads every practice Mulholland added, so a practice connects with an access code Mulholland gives it:
// <subdomain>/<location id>/<signature>, the signature the first 16 hex digits of the HMAC-SHA256 of
// <subdomain>/<location id> keyed with the API key, which connecting checks. Every request then goes to that
// subdomain and location, whatever it asked.
//
// What it syncs is health information under the practice's agreement with Mulholland, and NexHealth's: its
// providers, patients, appointments, procedures and payments, into the dental tables. An owner should narrow the
// connection to the group that may see them. NexHealth lists up to 1,000 a page, by cursor, and lists what changed
// since a time with updated_since: a stream asks from the newest updated_at it has seen, NexHealth's clock, not
// Archive's. Each is listed whole the first time and a week after, which deletes what is gone; providers, which are
// few, each sync. Appointment times are the practice's local ones, by the time zone NexHealth gives each; edited_at
// is NexHealth's, in UTC. NexHealth takes 8 procedure listings a second, so requests go at most that fast.
func init() {
	Register(&Connector{
		ID: "dentrix", Name: "Dentrix", Integration: true, Env: "ARCHIVE_DENTRIX", Host: "https://nexhealth.info",
		Info: Info{By: "NexHealth", Description: "Read the practice’s patients, appointments, procedures and payments", Category: CatDental, Icon: IconUsers,
			Scopes: []string{"Read practice data through NexHealth with the practice’s access code"}},
		API:  "https://nexhealth.info",
		Pace: 125 * time.Millisecond,
		Keys: []Key{{Name: "code", Label: "Access code", Secret: true,
			Help: "The code Mulholland sends once NexHealth's Synchronizer is set up on your Dentrix, like acme/123/9f86d081884c7d65"}},
		Authorize: dxAuthorize,
		Account:   dxAccount,
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
	// dxStart is before any practice's records, for listings NexHealth will not give without a time.
	dxStart  = "1990-01-01T00:00:00Z"
	dxPaths  = `^/(patients|providers|appointments|procedures|payments|charges|adjustments|claims|insurance_plans|insurance_coverages|treatment_plans|patient_recalls|appointment_types|operatories|payment_types|adjustment_types|working_hours)(/[0-9]+)?$`
	dxEdited = SQL("archive.sync_at(data->>'updated_at') AT TIME ZONE 'UTC'")
)

var (
	dxPage = Cursor{At: "page_info/end_cursor", More: "page_info/has_next_page"}
	dxDeny = []Deny{{Status: []int{403}, Why: "NexHealth does not let Archive read {stream} of this Dentrix: {message}"}}
	// dxBoth lists active and inactive apart, as NexHealth lists only one or the other.
	dxBoth     = &Parts{Fixed: []Obj{{"id": "false"}, {"id": "true"}}}
	dxInactive = Q{"inactive": "{part}"}
	// dxEver is every appointment's time, which NexHealth asks for.
	dxEver = Q{"start": dxStart, "end": "2100-01-01T00:00:00Z"}
)

// dxAt is a listing's query: qs, 1,000 a page, from the cursor.
func dxAt(qs ...Q) Q {
	q := Q{"per_page": "1000", "end_cursor": "{page}"}
	for _, x := range qs {
		for k, v := range x {
			q[k] = v
		}
	}
	return q
}

// dxLocal is the time at key in the practice's own clock, by the record's time zone when it has one Postgres knows,
// else as NexHealth wrote it.
func dxLocal(key string) SQL {
	return SQL("CASE WHEN data->>'timezone' <> '' AND pg_input_is_valid('2000-01-01 00:00 ' || (data->>'timezone'), 'timestamptz') " +
		"THEN archive.sync_at(data->>'" + key + "') AT TIME ZONE (data->>'timezone') " +
		"ELSE archive.sync_clock(replace(left(data->>'" + key + "', 19), 'T', ' ')) END")
}

type dxGet struct {
	Path  string `json:"path" jsonschema:"The endpoint, like /patients, /appointments, /procedures, /payments, /charges, /claims or /patients/12"`
	Query string `json:"query,omitempty" jsonschema:"Its query, like patient_id=12&per_page=100"`
}

// dxCode is the subdomain and location an access code is for, if it is one the server's API key signed.
func dxCode(key, code string) (sub, loc string, ok bool) {
	parts := strings.Split(strings.TrimSpace(code), "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || key == "" {
		return "", "", false
	}
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(parts[0] + "/" + parts[1]))
	want := hex.EncodeToString(mac.Sum(nil))[:16]
	return parts[0], parts[1], hmac.Equal([]byte(strings.ToLower(parts[2])), []byte(want))
}

// dxAccount checks the access code and names the location it is for.
func dxAccount(c *Client) (string, error) {
	key, _ := c.Credentials()
	_, loc, ok := dxCode(key, c.Key("code"))
	if !ok {
		return "", errors.New("that access code is not one Mulholland made; ask Mulholland for the practice's code")
	}
	var ans struct {
		Data struct{ Name string } `json:"data"`
	}
	if err := c.Get("https://nexhealth.info/locations/"+loc, &ans); err != nil {
		return "", err
	}
	return cmp.Or(ans.Data.Name, "Dentrix"), nil
}

// dxTokens are the bearer tokens of API keys, kept until shortly before NexHealth's hour is up.
var dxTokens = struct {
	sync.Mutex
	m map[string]dxToken
}{m: map[string]dxToken{}}

type dxToken struct {
	token string
	until time.Time
}

// dxAuthorize signs a request with the API key's bearer token, and sends it to the access code's subdomain and
// location; the request that makes the token carries the key itself.
func dxAuthorize(r *http.Request, c *Client) {
	key, _ := c.Credentials()
	r.Header.Set("Nex-Api-Version", dxVersion)
	if strings.HasSuffix(r.URL.Path, "/authenticates") {
		r.Header.Set("Authorization", key)
		return
	}
	sub, loc, ok := dxCode(key, c.Key("code"))
	if !ok {
		return
	}
	q := r.URL.Query()
	q.Set("subdomain", sub)
	q.Set("location_id", loc)
	r.URL.RawQuery = q.Encode()
	if t := dxBearer(c, key); t != "" {
		r.Header.Set("Authorization", "Bearer "+t)
	}
}

// dxBearer is the API key's bearer token, made when it has none that lasts; a request without one is refused as
// NexHealth refuses it.
func dxBearer(c *Client, key string) string {
	dxTokens.Lock()
	defer dxTokens.Unlock()
	if t := dxTokens.m[key]; time.Now().Before(t.until) {
		return t.token
	}
	var ans struct {
		Data struct{ Token string } `json:"data"`
	}
	if err := c.Send("POST", "https://nexhealth.info/authenticates", nil, nil, &ans); err != nil || ans.Data.Token == "" {
		return ""
	}
	dxTokens.m[key] = dxToken{ans.Data.Token, time.Now().Add(50 * time.Minute)}
	return ans.Data.Token
}
