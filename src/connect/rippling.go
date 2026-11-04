package connect

import (
	"time"

	"golang.org/x/oauth2"
)

// Rippling's REST API: the company an admin who connects it is, its departments, work locations and people. Third
// parties reach it only through an App Shop listing, with OAuth: Rippling's terms bar an app from using an API token a
// customer made. The listing, named archive, lists <API>/connect/rippling/callback among its redirect URLs and asks for
// companies.read, departments.read, work-locations.read, workers.read and users.read, all read only; Rippling takes a
// listing's scopes from the listing, not the authorization request, so Scopes is empty. One token is one company. The
// token request signs with the client in a Basic header, and a refresh token is used once, for a new one.
//
// Rippling counts an install done once the app says so, which mark_app_installed does: once a Client, before its
// first request, which connecting makes too, asking the company's name.
//
// Workers have no names of their own: each lists with its user, who has them, and its department. Nothing is listed
// by when it changed, and pushes need a webhook Archive does not set up, so a sync lists everything again: the setup a
// day apart, and people every time, whole, so one gone is deleted. Pay, compensation, birthdays, home addresses,
// demographics and custom fields are left out, as are payrolls, which the REST API does not have.
func init() {
	Register(&Connector{
		ID: "rippling", Name: "Rippling", Integration: true,
		Info: Info{By: "Rippling", Description: "Read the company, its departments, locations and people", Category: CatPayroll, Icon: IconUsers,
			Scopes: []string{"Read the company, departments, work locations and employees"}},
		API: "https://rest.ripplingapis.com",
		Endpoint: oauth2.Endpoint{AuthURL: "https://app.rippling.com/apps/PLATFORM/archive/authorize", TokenURL: "https://api.rippling.com/api/o/token/",
			AuthStyle: oauth2.AuthStyleInHeader},
		Let: []Step{{Req: Req{Method: POST, Read: true, URL: "https://api.rippling.com/platform/api/mark_app_installed"}}},
		Who: &Who{Req: Req{URL: "/companies/"}, Names: []Path{"results/0/name", "results/0/legal_name"}},
		Sync: []*Sync{
			{Name: "company", Every: ripplingDaily, Req: Req{URL: "/companies/"}, Items: "results", Updated: "updated_at",
				Doc: "The company, with its legal name and the name it does business as, by id; read a day apart.",
				To: Map{Table: "payroll_company",
					Cols:   map[string]SQL{"name": "coalesce(nullif(data->>'name', ''), data->>'legal_name')", "trade_name": "data->>'doing_business_as_name'"},
					People: map[string]SQL{"admin": "data->>'primary_email'"}}},
			{Name: "departments", Every: ripplingDaily, Parts: ripplingCompanies, Set: ripplingCompany, Req: Req{URL: "/departments/", Query: ripplingLimit}, Items: "results", Updated: "updated_at", Page: ripplingPage,
				Doc: "Every department, by id, 100 a page, listed whole a day apart, so one gone is deleted.",
				To:  Map{Table: "payroll_department", Cols: map[string]SQL{"title": "data->>'name'"}, Refs: map[string]SQL{"company": ripplingRef}}},
			{Name: "locations", Every: ripplingDaily, Parts: ripplingCompanies, Set: ripplingCompany, Req: Req{URL: "/work-locations/", Query: ripplingLimit}, Items: "results", Updated: "updated_at", Page: ripplingPage,
				Doc: "Every work location, with its address, by id, 100 a page, listed whole a day apart, so one gone is deleted.",
				To: Map{Table: "payroll_location", Refs: map[string]SQL{"company": ripplingRef},
					Cols: map[string]SQL{"name": "data->>'name'", "street_1": "coalesce(nullif(data#>>'{address,street_address}', ''), data#>>'{address,formatted}')",
						"city": "data#>>'{address,locality}'", "state": "data#>>'{address,region}'", "zip": "data#>>'{address,postal_code}'", "country": "data#>>'{address,country}'"}}},
			{Name: "workers", Parts: ripplingCompanies, Set: ripplingCompany, Items: "results", Updated: "updated_at", Page: ripplingPage,
				Req: Req{URL: "/workers/", Query: Q{"limit": "100", "expand": "user,department"}},
				Doc: "Every worker, those who left too, with their user's name and their department, by id, 100 a page, listed whole each sync, so one gone is deleted.",
				// A worker's one job is their title, start and work location; their status in Rippling, like HIRED before they
				// start or TERMINATED, is their employment.
				To: Map{Table: "payroll_worker",
					Cols: map[string]SQL{"kind": "'employee'", "name": ripplingName, "first_name": "data#>>'{user,name,given_name}'", "last_name": "data#>>'{user,name,family_name}'",
						"preferred_name": "data#>>'{user,name,preferred_given_name}'", "email": "archive.sync_email(data->>'work_email')", "title": "data->>'title'",
						"department_name": "data#>>'{department,name}'", "hired_on": "archive.sync_day(data->>'start_date')", "active": "data->>'status' = 'ACTIVE'",
						"terminated_on": "CASE WHEN data->>'status' = 'TERMINATED' THEN archive.sync_day(data->>'end_date') END"},
					People: map[string]SQL{"person": "data->>'work_email'"},
					Refs:   map[string]SQL{"company": ripplingRef, "department": "data->>'department_id'", "manager": "data->>'manager_id'"},
					Children: []Kids{
						{Table: "payroll_job", From: "(SELECT 1) j WHERE coalesce(data->>'title', '') <> ''",
							Cols: map[string]SQL{"title": "data->>'title'", "hired_on": "archive.sync_day(data->>'start_date')", "primary_job": "true"},
							Refs: map[string]SQL{"location": "data#>>'{location,work_location_id}'"}},
						{Table: "payroll_employment", From: "(SELECT 1) e",
							Cols: map[string]SQL{"status": "data->>'status'", "hired_on": "archive.sync_day(coalesce(nullif(data->>'original_start_date', ''), data->>'start_date'))",
								"terminated_on": "CASE WHEN data->>'status' = 'TERMINATED' THEN archive.sync_day(data->>'end_date') END"}},
					}}},
		},
	})
}

const (
	// ripplingDaily is how far apart setup, which seldom changes, is read.
	ripplingDaily     = 24 * time.Hour
	ripplingRef   SQL = "data->>'company_id'"
	ripplingName  SQL = "coalesce(nullif(btrim(data#>>'{user,name,formatted}'), ''), nullif(btrim(data#>>'{user,display_name}'), ''), " +
		"nullif(btrim(concat(data#>>'{user,name,given_name}', ' ', data#>>'{user,name,family_name}')), ''), data->>'work_email')"
)

var (
	// A list is 100 a page, Rippling's most, each naming the next as a URL.
	ripplingLimit = Q{"limit": "100"}
	ripplingPage  = NextURL{At: "next_link"}
	// ripplingCompanies is the token's company, as /companies/ lists it, so each record says whose it is.
	ripplingCompanies = &Parts{Req: Req{URL: "/companies/"}, Items: "results", ID: "{id}", Vals: Obj{"company": "{id}"}}
	ripplingCompany   = Obj{"company_id": "{company_id;in/company}"}
)
