package connect

import (
	"bytes"
	"cmp"
	"compress/gzip"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/oauth2"
	"golang.org/x/sync/errgroup"
)

// MyChart and Oracle Health's patient portal sign a patient in with SMART on FHIR at their own health system, which names their record (patient) as it grants the token.
// The patient finds their health system by name in places.json.gz, which holds where each one signs in.
func init() {
	Register(&Connector{
		ID: "mychart", Name: "MyChart", Env: "ARCHIVE_EPIC", Host: "https://fhir.epic.com",
		Info: Info{By: "Epic", Description: "Read your medical record from a health system's MyChart: conditions, medications, allergies, labs, vitals, immunizations, visits and documents",
			Category: CatHealth, Icon: IconHeart, Scopes: fhirInfo},
		Ask:      fhirAsk("epic", "Search by the name on your MyChart sign-in, like Sutter Health or Kaiser Permanente"),
		Endpoint: fhirEndpoint,
		Scopes:   fhirScopes, Keep: []string{"patient"},
		Let: []Step{{As: "fhir", In: "{meta/fhir}"}},
		API: "{+fhir}", Answer: fhirNext,
		Who:     fhirWho,
		Actions: fhirActions(),
		Sync:    fhirSync(),
	})

	Register(&Connector{
		ID: "oraclehealth", Name: "Oracle Health", Env: "ARCHIVE_CERNER", Host: "https://authorization.cerner.com",
		Info: Info{By: "Oracle", Description: "Read your medical record from a health system on Oracle Health (Cerner): conditions, medications, allergies, labs, vitals, immunizations, visits and documents",
			Category: CatHealth, Icon: IconHeart, Scopes: fhirInfo},
		Ask:      fhirAsk("oracle", "Search by your health system's name, like Intermountain Health"),
		Endpoint: fhirEndpoint,
		Scopes:   fhirScopes, Keep: []string{"patient"},
		Let: []Step{{As: "fhir", In: "{meta/fhir}"}},
		API: "{+fhir}", Answer: fhirNext,
		Who:     fhirWho,
		Actions: fhirActions(),
		Sync:    fhirSync(),
	})
}

const fhirDoc = ": every one the portal shares, listed whole each sync, which deletes those gone."

var (
	fhirInfo   = []string{"Read your medical record as your health system shares it with apps you choose, while you allow it"}
	fhirScopes = []string{"launch/patient", "offline_access", "patient/Patient.read", "patient/Condition.read", "patient/MedicationRequest.read",
		"patient/AllergyIntolerance.read", "patient/Observation.read", "patient/Immunization.read", "patient/Encounter.read",
		"patient/DiagnosticReport.read", "patient/DocumentReference.read", "patient/Binary.read"}
	fhirEndpoint = oauth2.Endpoint{AuthURL: "{+meta/authorize}?aud={meta/fhir}", TokenURL: "{+meta/token}", AuthStyle: oauth2.AuthStyleInHeader}
	fhirWho      = &Who{Req: Req{URL: "/Patient/{meta/patient}"}, Names: []Path{"name/0/text", "id"}, As: "{who/name/0/text} · {who/managingOrganization/display}"}
	fhirPage     = NextURL{At: "next"}

	fhirRecord = Map{Table: "health_record",
		Cols: map[string]SQL{
			"type": "data#>>'{resource,resourceType}'",
			"title": `coalesce(` + fhirText("code") + `, ` + fhirText("medicationCodeableConcept") + `, data#>>'{resource,medicationReference,display}', ` +
				fhirText("vaccineCode") + `, ` + fhirText("type,0") + `, data#>>'{resource,class,display}', data#>>'{resource,description}', ` +
				`nullif(data#>>'{resource,name,0,text}', ''), nullif(concat_ws(' ', data#>>'{resource,name,0,given,0}', data#>>'{resource,name,0,family}'), ''))`,
			"code":        `coalesce(data#>>'{resource,code,coding,0,code}', data#>>'{resource,medicationCodeableConcept,coding,0,code}', data#>>'{resource,vaccineCode,coding,0,code}', data#>>'{resource,type,0,coding,0,code}')`,
			"code_system": `coalesce(data#>>'{resource,code,coding,0,system}', data#>>'{resource,medicationCodeableConcept,coding,0,system}', data#>>'{resource,vaccineCode,coding,0,system}', data#>>'{resource,type,0,coding,0,system}')`,
			"category": `coalesce(data#>>'{resource,category,0,coding,0,code}', CASE jsonb_typeof(data#>'{resource,category,0}') WHEN 'string' THEN data#>>'{resource,category,0}' END, ` +
				`data#>>'{resource,category,0,text}', data#>>'{resource,class,code}')`,
			"status": `coalesce(data#>>'{resource,clinicalStatus,coding,0,code}', data#>>'{resource,status}')`,
			"value":  "archive.sync_num(data#>>'{resource,valueQuantity,value}')",
			"unit":   "coalesce(data#>>'{resource,valueQuantity,unit}', data#>>'{resource,valueQuantity,code}')",
			"value_text": `coalesce(data#>>'{resource,valueString}', ` + fhirText("valueCodeableConcept") + `, data#>>'{resource,dosageInstruction,0,text}', ` +
				`data#>>'{resource,reaction,0,manifestation,0,text}', data#>>'{resource,reaction,0,manifestation,0,coding,0,display}', data#>>'{resource,conclusion}')`,
			"low":  "archive.sync_num(data#>>'{resource,referenceRange,0,low,value}')",
			"high": "archive.sync_num(data#>>'{resource,referenceRange,0,high,value}')",
			"flag": "coalesce(data#>>'{resource,interpretation,0,coding,0,code}', data#>>'{resource,interpretation,0,text}')",
			"at": fhirTime(`coalesce(data#>>'{resource,effectiveDateTime}', data#>>'{resource,effectivePeriod,start}', data#>>'{resource,onsetDateTime}', data#>>'{resource,onsetPeriod,start}', ` +
				`data#>>'{resource,occurrenceDateTime}', data#>>'{resource,authoredOn}', data#>>'{resource,period,start}', data#>>'{resource,recordedDate}', ` +
				`data#>>'{resource,issued}', data#>>'{resource,date}', data#>>'{resource,birthDate}')`),
			"ends_at": fhirTime(`coalesce(data#>>'{resource,period,end}', data#>>'{resource,effectivePeriod,end}', data#>>'{resource,abatementDateTime}', data#>>'{resource,abatementPeriod,start}')`),
			"clinician": `coalesce(data#>>'{resource,performer,0,display}', data#>>'{resource,performer,0,actor,display}', data#>>'{resource,requester,display}', ` +
				`data#>>'{resource,recorder,display}', data#>>'{resource,asserter,display}', data#>>'{resource,participant,0,individual,display}', data#>>'{resource,author,0,display}')`,
			"organization": `coalesce(data#>>'{resource,serviceProvider,display}', data#>>'{resource,managingOrganization,display}', data#>>'{resource,custodian,display}', data#>>'{resource,performer,1,display}')`,
			"note":         `archive.sync_clip(nullif(concat_ws(E'\n\n', (SELECT string_agg(n->>'text', E'\n\n') FROM archive.sync_list(data#>'{resource,note}') n)), ''), 262144)`,
			"resource":     "data->'resource'",
			"edited_at":    "archive.sync_at(data#>>'{resource,meta,lastUpdated}')",
		}}
)

func fhirText(field string) SQL {
	return SQL("coalesce(nullif(data#>>'{resource," + field + ",text}', ''), data#>>'{resource," + field + ",coding,0,display}')")
}

func fhirTime(s string) SQL {
	return SQL("(SELECT coalesce(archive.sync_at(t), archive.sync_day(t)::timestamptz) FROM (SELECT " + s + " t) x)")
}

func fhirSearch(name, path, doc string, q Q) *Sync {
	query := Q{"patient": "{meta/patient}", "_count": "100"}
	for k, v := range q {
		query[k] = v
	}
	return &Sync{Name: name, Gap: time.Second, Req: Req{URL: "/" + path, Query: query}, Items: "entry", Page: fhirPage, Empty: true,
		Key: []Reads{{Of: "{resource/resourceType}/{resource/id}"}}, Skip: []Test{{At: "resource/resourceType", Not: path}},
		Doc: doc + fhirDoc, To: fhirRecord}
}

func fhirSync() []*Sync {
	return []*Sync{
		fhirSearch("patient", "Patient", "You as the health system records you: name, birth date, sex, address and phone", Q{"_id": "{meta/patient}", "patient": ""}),
		fhirSearch("conditions", "Condition", "Each condition on your problem list and each diagnosis from a visit, with its status and when it began and ended", nil),
		fhirSearch("medications", "MedicationRequest", "Each medication prescribed, with its dose instructions, status and prescriber", nil),
		fhirSearch("allergies", "AllergyIntolerance", "Each allergy and intolerance, with its reactions", nil),
		fhirSearch("labs", "Observation", "Each lab result, with its value, unit, normal range and flag", Q{"category": "laboratory"}),
		fhirSearch("vitals", "Observation", "Each vital sign, like blood pressure, weight, height, pulse and temperature", Q{"category": "vital-signs"}),
		fhirSearch("reports", "DiagnosticReport", "Each report, like a lab panel, imaging or pathology, with its conclusion", nil),
		fhirSearch("immunizations", "Immunization", "Each vaccine given", nil),
		fhirSearch("encounters", "Encounter", "Each visit, admission and telehealth call, with who saw you and where", nil),
		{Name: "documents", Gap: time.Second, Req: Req{URL: "/DocumentReference", Query: Q{"patient": "{meta/patient}", "_count": "100"}}, Items: "entry", Page: fhirPage, Empty: true,
			Key: []Reads{{Of: "{resource/id}"}}, Skip: []Test{{At: "resource/resourceType", Not: "DocumentReference"}, {At: "resource/content/0/attachment/url", Off: true}},
			Content: []Content{{Req: Req{URL: "{+source/url}", Header: map[string][]string{"Accept": {"{source/mime}"}}}, Size: "size"}},
			Doc:     "Each document, like a visit note, discharge summary or scanned letter, as a file with a copy of its bytes and words" + fhirDoc,
			To: Map{Table: "file",
				Cols: map[string]SQL{"kind": "'file'",
					"name": `coalesce(nullif(data#>>'{resource,content,0,attachment,title}', ''), nullif(data#>>'{resource,description}', ''), ` + fhirText("type") +
						`, 'Document') || ' ' || coalesce(left(data#>>'{resource,date}', 10), '')`,
					"mime":    "coalesce(data#>>'{resource,content,0,attachment,contentType}', 'application/octet-stream')",
					"source":  "jsonb_build_object('url', CASE WHEN data#>>'{resource,content,0,attachment,url}' ~ '^https://' THEN data#>>'{resource,content,0,attachment,url}' ELSE '/' || ltrim(data#>>'{resource,content,0,attachment,url}', '/') END, 'mime', coalesce(data#>>'{resource,content,0,attachment,contentType}', '*/*'), 'size', data#>>'{resource,content,0,attachment,size}')",
					"drive":   "'Medical record'",
					"made_at": fhirTime("data#>>'{resource,date}'"), "edited_at": "archive.sync_at(data#>>'{resource,meta,lastUpdated}')",
					"owner_name": "data#>>'{resource,author,0,display}'"}}},
	}
}

func fhirActions() map[string]*Action {
	return map[string]*Action{
		"search": Call[fhirFind, any](Plan{
			Doc: "Search your record with FHIR R4, like { type: \"Observation\", query: \"category=laboratory&code=http://loinc.org|4548-4\" } " +
				"or { type: \"Condition\" }: a Bundle of { entry: [{ resource }], next }. Types are Patient, Condition, MedicationRequest, " +
				"AllergyIntolerance, Observation (with a category: laboratory, vital-signs or social-history), DiagnosticReport, Immunization, " +
				"Encounter and DocumentReference; the search is always yours",
			Steps: []Step{
				{Only: Test{At: "type", Re: `^[A-Z][A-Za-z]+$`, Off: true}, Fail: "type is a FHIR resource type, like Observation"},
				{Only: Test{At: "query", Re: `(^|&)(patient|subject|_id)=`}, Fail: "the search is always yours; leave out patient, subject and _id"},
				{Req: Req{URL: "/{type}?{+query|}", Query: Q{"patient": "{meta/patient}"}}},
			},
		}),
		"read": Call[fhirOne, any](Plan{
			Doc:   "One resource of your record by type and id, like { type: \"DiagnosticReport\", id: \"eX1…\" } or { type: \"Binary\", id } for a document's content",
			Steps: []Step{{Only: Test{At: "type", Re: `^[A-Z][A-Za-z]+$`, Off: true}, Fail: "type is a FHIR resource type, like Observation"}, {Req: Req{URL: "/{type}/{id}"}}},
		}),
	}
}

var fhirNext = &Codec{"fhir_next", func(v any) (any, error) {
	m, _ := v.(map[string]any)
	if m == nil || m["resourceType"] != "Bundle" {
		return v, nil
	}
	for _, l := range listOf(m["link"]) {
		if x, _ := l.(map[string]any); x != nil && x["relation"] == "next" {
			m["next"] = x["url"]
		}
	}
	return m, nil
}}

type (
	fhirFind struct {
		Type  string `json:"type" jsonschema:"The resource type, like Observation, Condition or MedicationRequest"`
		Query string `json:"query,omitempty" jsonschema:"More search parameters, like category=laboratory&date=ge2026-01-01&_count=100"`
	}
	fhirOne struct {
		Type string `json:"type" jsonschema:"The resource type, like DiagnosticReport"`
		ID   string `json:"id" jsonschema:"Its id"`
	}
)

func fhirAsk(places, help string) *Key {
	return &Key{Name: "fhir", Label: "Your health system", Help: help, Places: places}
}

// A place is a health system a patient can pick, at the FHIR server (Servers[Server]) that signs them in.
type (
	fhirServer struct {
		FHIR      string `json:"fhir"`
		Authorize string `json:"authorize"`
		Token     string `json:"token"`
	}
	fhirPlace struct {
		Name   string `json:"name"`
		State  string `json:"state,omitempty"`
		Server int    `json:"server"`
	}
	fhirPlaces struct {
		Servers []fhirServer `json:"servers"`
		Places  []fhirPlace  `json:"places"`
	}
	Place struct {
		Name  string `json:"name"`
		State string `json:"state,omitempty"`
		Value string `json:"value"`
	}
)

//go:embed places.json.gz
var placesGz []byte

var places = sync.OnceValue(func() map[string]*fhirPlaces {
	var out map[string]*fhirPlaces
	r, err := gzip.NewReader(bytes.NewReader(placesGz))
	if err == nil {
		err = json.NewDecoder(r).Decode(&out)
	}
	if err != nil {
		panic("connect: places.json.gz: " + err.Error())
	}
	return out
})

// Places finds up to n of a connector's health systems whose names have words starting with each of q's, those that start with q first.
func (p *Provider) Places(q string, n int) []Place {
	d := p.Ask.dir()
	if d == nil {
		return nil
	}
	words := placeWords(q)
	var first, rest []Place
	for _, pl := range d.Places {
		name := placeWords(pl.Name + " " + pl.State)
		if slices.ContainsFunc(words, func(w string) bool {
			return !slices.ContainsFunc(name, func(x string) bool { return strings.HasPrefix(x, w) })
		}) {
			continue
		}
		hit := Place{Name: pl.Name, State: pl.State, Value: d.Servers[pl.Server].FHIR}
		if len(words) > 0 && strings.HasPrefix(strings.Join(name, " "), strings.Join(words, " ")) {
			first = append(first, hit)
		} else {
			rest = append(rest, hit)
		}
	}
	out := append(first, rest...)
	return out[:min(n, len(out))]
}

func (k *Key) dir() *fhirPlaces {
	if k == nil || k.Places == "" {
		return nil
	}
	return places()[k.Places]
}

func placeWords(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// placeOf is the server a patient picked, by its FHIR address.
func (k *Key) placeOf(v string) (map[string]string, bool) {
	v = strings.TrimRight(strings.TrimSpace(v), "/")
	for _, s := range k.dir().Servers {
		if strings.EqualFold(s.FHIR, v) {
			return map[string]string{k.Name: s.FHIR, "authorize": s.Authorize, "token": s.Token}, true
		}
	}
	return nil, false
}

// WritePlaces gathers Epic's and Oracle Health's published directories of health systems patients sign in at, and where each signs in, into file.
func WritePlaces(ctx context.Context, file string) error {
	epic, err := fhirDirectory(ctx, "https://open.epic.com/Endpoints/Brands", func(org fhirOrg) bool { return len(org.Endpoint) > 0 })
	if err != nil {
		return err
	}
	epic.Places = append(epic.Places, fhirPlace{Name: "Epic sandbox (test patients)", Server: len(epic.Servers)})
	epic.Servers = append(epic.Servers, fhirServer{FHIR: "https://fhir.epic.com/interconnect-fhir-oauth/api/FHIR/R4"})
	oracle, err := fhirDirectory(ctx, "https://raw.githubusercontent.com/oracle-samples/ignite-endpoints/main/oracle_health_fhir_endpoints/millennium_patient_r4_endpoints.json",
		func(fhirOrg) bool { return true })
	if err != nil {
		return err
	}
	oracle.Places = append(oracle.Places, fhirPlace{Name: "Oracle Health sandbox (test patients)", Server: len(oracle.Servers)})
	oracle.Servers = append(oracle.Servers, fhirServer{FHIR: "https://fhir-myrecord.cerner.com/r4/ec2458f2-1e24-41c8-b71b-0e701af7583d"})
	for _, d := range []*fhirPlaces{epic, oracle} {
		if err := smartOf(ctx, d.Servers); err != nil {
			return err
		}
		slices.SortStableFunc(d.Places, func(a, b fhirPlace) int {
			return cmp.Or(strings.Compare(a.Name, b.Name), strings.Compare(a.State, b.State))
		})
	}
	var buf bytes.Buffer
	w, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err := json.NewEncoder(w).Encode(map[string]*fhirPlaces{"epic": epic, "oracle": oracle}); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return os.WriteFile(file, buf.Bytes(), 0o644)
}

type fhirOrg struct {
	Name    string `json:"name"`
	Address []struct {
		State string `json:"state"`
	} `json:"address"`
	Endpoint []struct {
		Reference string `json:"reference"`
	} `json:"endpoint"`
}

func fhirDirectory(ctx context.Context, from string, brand func(fhirOrg) bool) (*fhirPlaces, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, from, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/fhir+json")
	r, err := (&http.Client{Timeout: 10 * time.Minute}).Do(req)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s answered %s", from, r.Status)
	}
	var b struct {
		Entry []struct {
			FullURL  string `json:"fullUrl"`
			Resource struct {
				ResourceType string          `json:"resourceType"`
				ID           string          `json:"id"`
				Address      json.RawMessage `json:"address"`
				fhirOrg
			} `json:"resource"`
		} `json:"entry"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		return nil, fmt.Errorf("%s: %w", from, err)
	}
	at := map[string]string{}
	for _, e := range b.Entry {
		if e.Resource.ResourceType == "Endpoint" {
			var addr string
			json.Unmarshal(e.Resource.Address, &addr)
			at[e.FullURL], at["Endpoint/"+e.Resource.ID] = strings.TrimRight(addr, "/"), strings.TrimRight(addr, "/")
		}
	}
	d, server := &fhirPlaces{}, map[string]int{}
	for _, e := range b.Entry {
		org := e.Resource.fhirOrg
		if e.Resource.ResourceType != "Organization" || len(org.Endpoint) == 0 || !brand(org) || at[org.Endpoint[0].Reference] == "" {
			continue
		}
		json.Unmarshal(e.Resource.Address, &org.Address)
		fhir := at[org.Endpoint[0].Reference]
		i, ok := server[fhir]
		if !ok {
			i, server[fhir] = len(d.Servers), len(d.Servers)
			d.Servers = append(d.Servers, fhirServer{FHIR: fhir})
		}
		pl := fhirPlace{Name: strings.TrimSpace(org.Name), Server: i}
		if len(org.Address) > 0 {
			pl.State = org.Address[0].State
		}
		d.Places = append(d.Places, pl)
	}
	if len(d.Places) == 0 {
		return nil, fmt.Errorf("%s listed no health systems", from)
	}
	return d, nil
}

// smartOf reads where each server signs patients in from its SMART configuration, or, when it doesn't answer, Epic's usual place beside the FHIR address.
func smartOf(ctx context.Context, servers []fhirServer) error {
	var g errgroup.Group
	g.SetLimit(32)
	for i := range servers {
		s := &servers[i]
		g.Go(func() error {
			var smart struct {
				Authorize string `json:"authorization_endpoint"`
				Token     string `json:"token_endpoint"`
			}
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.FHIR+"/.well-known/smart-configuration", nil)
			if err != nil {
				return err
			}
			req.Header.Set("Accept", "application/json")
			if r, err := client.Do(req); err == nil {
				if r.StatusCode == http.StatusOK {
					json.NewDecoder(r.Body).Decode(&smart)
				}
				r.Body.Close()
			}
			if smart.Authorize == "" || smart.Token == "" {
				base, ok := strings.CutSuffix(s.FHIR, "/api/FHIR/R4")
				if !ok {
					return fmt.Errorf("%s has no SMART configuration", s.FHIR)
				}
				smart.Authorize, smart.Token = base+"/oauth2/authorize", base+"/oauth2/token"
				fmt.Fprintf(os.Stderr, "places: %s has no SMART configuration, so it signs in beside its FHIR address\n", s.FHIR)
			}
			s.Authorize, s.Token = smart.Authorize, smart.Token
			return nil
		})
	}
	return g.Wait()
}
