package connect

import "time"

// ADP Workforce Now, through ADP's APIs, read only. ADP lets an app in only with mutual TLS, a client certificate
// ADP signs, and an OAuth client of the company's own, by client credentials: there is no sign-in a person can
// give Archive. So it is a key connector that connects as the company itself, through ADP API Central, ADP's
// paid add-on with which a Workforce Now company makes its own API projects: in a project with the worker and
// pay statement APIs, an admin generates a certificate and private key, and pastes them in with the project's
// client ID and secret. Until Mulholland is an ADP Marketplace partner, whose one certificate and consented
// client credentials would stand in for these, this is the only way in.
//
// Every call takes a token, which the certificate and client trade for at accounts.adp.com and which lasts an
// hour, so it is kept for the account for 50 minutes, or until a 401 says it is gone.
//
// It syncs the company's workers, the ones who left too, with their jobs and home address, into payroll_worker,
// listed whole each sync, 100 a page, which deletes those gone; ADP lists by $skip, and pages overlap by one, so
// the page after a full one is never empty, which ADP answers as a 204. And each worker's pay statements of the last
// 50 pay dates, a day apart, each one once, as a payroll of one person: payroll_run, with what they were paid in
// payroll_pay and line by line in payroll_pay_line. ADP pushes nothing an app of a company's own can hear.
func init() {
	Register(&Connector{
		ID: "adp", Name: "ADP", Integration: true,
		Info: Info{By: "ADP", Description: "Read the company’s workers and their pay statements from ADP Workforce Now", Category: CatPayroll, Icon: IconUsers,
			Scopes: []string{"Read workers and pay statements with the company’s API Central project"}},
		API: "https://api.adp.com",
		Keys: []Key{
			{Name: "client_id", Label: "Client ID", Help: "In ADP API Central, your project, Credentials: the Client ID"},
			{Name: "client_secret", Label: "Client secret", Secret: true, Help: "On the same page: the Client Secret"},
			{Name: "certificate", Label: "Certificate", Secret: true, Help: "In the project's certificates, the certificate API Central generated, a .pem that starts with -----BEGIN CERTIFICATE-----"},
			{Name: "private_key", Label: "Private key", Secret: true, Help: "The private key API Central gave with it, shown only once, which starts with -----BEGIN PRIVATE KEY-----"},
		},
		Cert: "certificate", CertKey: "private_key",
		Let: []Step{{As: "auth", Req: Req{Method: POST, Read: true, URL: "https://accounts.adp.com/auth/oauth/v2/token",
			Form: Q{"grant_type": "client_credentials", "client_id": "{key/client_id}", "client_secret": "{key/client_secret}"}}}},
		LetFor: 50 * time.Minute,
		Sign:   map[string]string{"Authorization": "Bearer {let/auth/access_token}"},
		Who:    &Who{Req: Req{URL: "/hr/v2/workers", Query: Q{"$top": "1"}}},
		Actions: map[string]*Action{
			"get": Call[adpGet, any](Plan{
				Doc: "Read any of ADP's GET endpoints the project may, like { path: \"/hr/v2/workers\", query: \"$top=100&$skip=0\" }, { path: \"/hr/v2/workers/<associateOID>\" } " +
					"or { path: \"/payroll/v1/workers/<associateOID>/pay-statements\", query: \"numberoflastpaydates=10\" }: its answer; nothing at all when there is none",
				Steps: []Step{{Req: Req{URL: "{+path}?{+query|}"}}},
			}),
		},
		Sync: []*Sync{
			// A worker's primary assignment, or else their first, says their title, department, manager and pay.
			{Name: "workers", Gap: time.Second, Req: adpWorkers, Items: "workers", ID: "associateOID", Page: adpPage, Absent: Empty,
				Doc: "Every worker, the ones who left too, with their name, emails, birthday, status, dates, and every assignment's title, department, manager and pay, " +
					"and their home address, by associateOID: listed whole each sync, 100 a page, which deletes those gone.",
				To: Map{Table: "payroll_worker",
					Cols: map[string]SQL{"kind": "'employee'",
						"name":       "coalesce(nullif(btrim(data#>>'{person,legalName,formattedName}'), ''), btrim(concat(data#>>'{person,legalName,givenName}', ' ', data#>>'{person,legalName,familyName1}')))",
						"first_name": "data#>>'{person,legalName,givenName}'", "middle_initial": "left(nullif(data#>>'{person,legalName,middleName}', ''), 1)",
						"last_name": "data#>>'{person,legalName,familyName1}'", "preferred_name": "data#>>'{person,preferredName,givenName}'",
						"email":          "archive.sync_email(coalesce(nullif(data#>>'{businessCommunication,emails,0,emailUri}', ''), data#>>'{person,communication,emails,0,emailUri}'))",
						"personal_email": "archive.sync_email(data#>>'{person,communication,emails,0,emailUri}')",
						"phone":          "coalesce(nullif(data#>>'{businessCommunication,landlines,0,formattedNumber}', ''), data#>>'{businessCommunication,mobiles,0,formattedNumber}', data#>>'{person,communication,mobiles,0,formattedNumber}')",
						"birthday":       "archive.sync_day(data#>>'{person,birthDate}')",
						"title":          "coalesce(nullif(" + adpJob + "->>'jobTitle', ''), " + adpJob + "#>>'{jobCode,shortName}')",
						"department_name": "(SELECT coalesce(nullif(u#>>'{nameCode,shortName}', ''), u#>>'{nameCode,longName}', u#>>'{nameCode,codeValue}') FROM archive.sync_list(" + adpJob + "->'homeOrganizationalUnits') u " +
							"WHERE u#>>'{typeCode,codeValue}' ILIKE 'department' LIMIT 1)",
						"hired_on":      "archive.sync_day(coalesce(nullif(data#>>'{workerDates,rehireDate}', ''), data#>>'{workerDates,originalHireDate}'))",
						"terminated_on": "archive.sync_day(data#>>'{workerDates,terminationDate}')",
						"active":        "data#>>'{workerStatus,statusCode,codeValue}' ILIKE 'active'",
						"pay_rate":      adpRate(adpJob), "pay_per": adpPer(adpJob),
						"flsa_status": "coalesce(nullif(" + adpJob + "#>>'{wageLawCoverage,coverageCode,shortName}', ''), " + adpJob + "#>>'{wageLawCoverage,coverageCode,codeValue}')"},
					People: map[string]SQL{"person": "coalesce(nullif(data#>>'{businessCommunication,emails,0,emailUri}', ''), data#>>'{person,communication,emails,0,emailUri}')"},
					Refs:   map[string]SQL{"manager": adpJob + "#>>'{reportsTo,0,associateOID}'"},
					Children: []Kids{
						{Table: "payroll_job", From: "archive.sync_list(data->'workAssignments') a",
							Cols: map[string]SQL{"source_id": "a->>'itemID'", "title": "coalesce(nullif(a->>'jobTitle', ''), a#>>'{jobCode,shortName}')", "rate": adpRate("a"), "pay_per": adpPer("a"),
								"hired_on": "archive.sync_day(a->>'hireDate')", "primary_job": "coalesce(a->>'primaryIndicator' = 'true', false)"}},
						{Table: "payroll_address", From: "archive.sync_list(jsonb_build_array(data#>'{person,legalAddress}')) a WHERE jsonb_typeof(a) = 'object'",
							Cols: map[string]SQL{"kind": "'home'", "street_1": "a->>'lineOne'", "street_2": "a->>'lineTwo'", "city": "a->>'cityName'",
								"state": "coalesce(nullif(a#>>'{countrySubdivisionLevel1,codeValue}', ''), a#>>'{countrySubdivisionLevel1,shortName}')", "zip": "a->>'postalCode'", "country": "a->>'countryCode'", "active": "true"}},
					}}},
			// A statement does not change once there, so the one detailed before is only seen.
			{Name: "pay_statements", Gap: time.Second, Every: 24 * time.Hour, Keep: true, Absent: Empty, Memo: "{payDate}",
				Parts: &Parts{Req: adpWorkers, Items: "workers", Page: adpPage, ID: "{associateOID}"},
				Req:   Req{URL: "/payroll/v1/workers/{part}/pay-statements", Query: Q{"numberoflastpaydates": "50"}}, Items: "payStatements",
				Key: []Reads{{Of: "{payDetailUri/href}", Find: `pay-statements/([^/?]+)`}}, Set: Obj{"associateOID": "{part}"},
				Hydrate: []Hydrate{Fetch{If: Test{At: "payDetailUri/href"}, Req: Req{URL: "{+payDetailUri/href}"}, As: "detail", Soft: true}},
				Doc: "Each worker's pay statements of their last 50 pay dates, with their pay period, gross and net pay, hours, and each earning and deduction, " +
					"read once, a day apart, which deletes nothing.",
				To: Map{Table: "payroll_run",
					Cols: map[string]SQL{"check_date": "archive.sync_day(coalesce(nullif(data->>'payDate', ''), " + adpStatement + "->>'payDate'))",
						"period_start": "archive.sync_day(" + adpStatement + "#>>'{payPeriod,startDate}')", "period_end": "archive.sync_day(" + adpStatement + "#>>'{payPeriod,endDate}')",
						"processed": "true", "gross_pay": adpMoney("coalesce(data->'grossPayAmount', " + adpStatement + "->'grossPayAmount')"),
						"net_pay": adpMoney("coalesce(data->'netPayAmount', " + adpStatement + "->'netPayAmount')")},
					Children: []Kids{
						// One row, the statement itself as s, for an aggregate over it to be the child's and not the statement's.
						{Table: "payroll_pay", From: "archive.sync_list(jsonb_build_array(data)) s", Refs: map[string]SQL{"worker": "s->>'associateOID'"},
							Cols: map[string]SQL{"gross_pay": adpMoney("coalesce(s->'grossPayAmount', s#>'{detail,payStatement,grossPayAmount}')"),
								"net_pay": adpMoney("coalesce(s->'netPayAmount', s#>'{detail,payStatement,netPayAmount}')"),
								"hours":   "coalesce(archive.sync_num(s->>'totalHours'), archive.sync_num(s#>>'{detail,payStatement,totalHours}'))"}},
						{Table: "payroll_pay_line", From: "archive.sync_list(" + adpStatement + "->'earnings') x", Refs: adpWorker,
							Cols: map[string]SQL{"kind": "'earning'", "name": adpCode("earningCode"), "amount": adpMoney("x->'earningAmount'"), "hours": "archive.sync_num(x->>'payPeriodHours')", "employer": "false"}},
						// ADP lists taxes with the deductions, by their category; it gives them as negative amounts.
						{Table: "payroll_pay_line", From: "archive.sync_list(" + adpStatement + "->'deductions') x", Refs: adpWorker,
							Cols: map[string]SQL{"kind": "CASE WHEN concat(x->>'deductionCategoryCodeName', x#>>'{deductionCategoryCode,codeValue}', x#>>'{deductionCode,shortName}') ILIKE '%tax%' THEN 'tax' ELSE 'deduction' END",
								"name": adpCode("deductionCode"), "amount": "archive.sync_money(abs(archive.sync_num(x#>>'{deductionAmount,amountValue}'))::text, coalesce(nullif(x#>>'{deductionAmount,currencyCode}', ''), 'USD'))",
								"employer": "false"}},
					}}},
		},
	})
}

const (
	adpJob       = "(SELECT a FROM archive.sync_list(data->'workAssignments') WITH ORDINALITY x(a, i) ORDER BY a->>'primaryIndicator' = 'true' DESC NULLS LAST, i LIMIT 1)"
	adpStatement = "(data#>'{detail,payStatement}')"
)

var (
	adpWorkers = Req{URL: "/hr/v2/workers", Query: Q{"$top": "100", "$skip": "{page}"}}
	adpPage    = Offset{Size: 100, Step: 99}
	adpWorker  = map[string]SQL{"worker": "data->>'associateOID'"}
)

// adpRate is what the assignment a pays, by the hour when it says so, else by the year, else by the pay period.
func adpRate(a string) SQL {
	return "coalesce(" + adpMoney(a+"#>'{baseRemuneration,hourlyRateAmount}'") + ", " + adpMoney(a+"#>'{baseRemuneration,annualRateAmount}'") + ", " +
		adpMoney(a+"#>'{baseRemuneration,payPeriodRateAmount}'") + ")"
}

// adpPer is what adpRate is for.
func adpPer(a string) SQL {
	return SQL("CASE WHEN " + a + "#>>'{baseRemuneration,hourlyRateAmount,amountValue}' IS NOT NULL THEN 'Hour' WHEN " + a +
		"#>>'{baseRemuneration,annualRateAmount,amountValue}' IS NOT NULL THEN 'Year' WHEN " + a + "#>>'{baseRemuneration,payPeriodRateAmount,amountValue}' IS NOT NULL THEN 'Paycheck' END")
}

// adpMoney is ADP's { amountValue, currencyCode } at v as money.
func adpMoney(v string) SQL {
	return SQL("archive.sync_money(" + v + "->>'amountValue', coalesce(nullif(" + v + "->>'currencyCode', ''), 'USD'))")
}

// adpCode is the name of the code at key of a line x.
func adpCode(key string) SQL {
	return SQL("coalesce(nullif(x->>'CodeName', ''), nullif(x#>>'{" + key + ",shortName}', ''), nullif(x#>>'{" + key + ",longName}', ''), x#>>'{" + key + ",codeValue}', '')")
}

type adpGet struct {
	Path  string `json:"path" jsonschema:"The endpoint, like /hr/v2/workers or /payroll/v1/workers/<associateOID>/pay-statements"`
	Query string `json:"query,omitempty" jsonschema:"Its query, like $top=100&$skip=100 or numberoflastpaydates=10"`
}
