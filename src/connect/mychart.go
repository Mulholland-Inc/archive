package connect

import (
	"time"

	"golang.org/x/oauth2"
)

// MyChart and Oracle Health's patient portal sign a patient in with SMART on FHIR at their own health system, which names their record (patient) as it grants the token.
func init() {
	Register(&Connector{
		ID: "mychart", Name: "MyChart", Env: "ARCHIVE_EPIC", Host: "https://fhir.epic.com",
		Info: Info{By: "Epic", Description: "Read your medical record from a health system's MyChart: conditions, medications, allergies, labs, vitals, immunizations, visits and documents",
			Category: CatHealth, Icon: IconHeart, Scopes: fhirInfo},
		Ask: &Key{Name: "epic", Label: "Health system's FHIR address",
			Help: "Your health system's Epic FHIR R4 address, like https://fhir.example.org/FHIR/api/FHIR/R4, from open.epic.com/MyApps/Endpoints",
			Find: `^(https://[a-z0-9.-]+(?::\d+)?(?:/[^?#\s]*?)?)/api/fhir/r4/?$`},
		Endpoint: oauth2.Endpoint{AuthURL: "{+meta/epic}/oauth2/authorize?aud={meta/epic}%2Fapi%2FFHIR%2FR4", TokenURL: "{+meta/epic}/oauth2/token",
			AuthStyle: oauth2.AuthStyleInHeader},
		Scopes: fhirScopes, Keep: []string{"patient"},
		Let: []Step{{As: "fhir", In: "{meta/epic}/api/FHIR/R4"}},
		API: "{+fhir}", Answer: fhirNext,
		Who:     fhirWho,
		Actions: fhirActions(),
		Sync:    fhirSync(),
	})

	Register(&Connector{
		ID: "oraclehealth", Name: "Oracle Health", Env: "ARCHIVE_CERNER", Host: "https://authorization.cerner.com",
		Info: Info{By: "Oracle", Description: "Read your medical record from a health system on Oracle Health (Cerner): conditions, medications, allergies, labs, vitals, immunizations, visits and documents",
			Category: CatHealth, Icon: IconHeart, Scopes: fhirInfo},
		Ask: &Key{Name: "tenant", Label: "Health system's FHIR address",
			Help: "Your health system's Oracle Health patient FHIR address, like https://fhir-myrecord.cerner.com/r4/ec2458f2-1e24-41c8-b71b-0e701af7583d",
			Find: `^(?:https://fhir-myrecord\.cerner\.com/r4/)?([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})/?$`},
		Endpoint: oauth2.Endpoint{AuthURL: "https://authorization.cerner.com/tenants/{meta/tenant}/protocols/oauth2/profiles/smart-v1/personas/patient/authorize?aud=https%3A%2F%2Ffhir-myrecord.cerner.com%2Fr4%2F{meta/tenant}",
			TokenURL: "https://authorization.cerner.com/tenants/{meta/tenant}/hosts/fhir-myrecord.cerner.com/protocols/oauth2/profiles/smart-v1/token", AuthStyle: oauth2.AuthStyleInHeader},
		Scopes: fhirScopes, Keep: []string{"patient"},
		Let: []Step{{As: "fhir", In: "https://fhir-myrecord.cerner.com/r4/{meta/tenant}"}},
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
	fhirWho  = &Who{Req: Req{URL: "/Patient/{meta/patient}"}, Names: []Path{"name/0/text", "id"}, As: "{who/name/0/text} · {who/managingOrganization/display}"}
	fhirPage = NextURL{At: "next"}

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
