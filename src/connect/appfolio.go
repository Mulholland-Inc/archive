package connect

import "time"

// AppFolio, the property management system, through its Reports API v2, read only. It needs no partnership with
// AppFolio: on the Plus and Max plans, a System Administrator finds a Client ID and Client Secret in General
// Settings, Manage API Settings, Reports API Credentials, and an owner pastes them in with the database's address,
// like acme.appfolio.com. AppFolio keeps one pair: making a new one ends the old, and every tool that used it.
//
// A report is a POST of its filters to /{report}.json, signed with HTTP Basic, which answers 5,000 rows of results
// a page and the next page's next_page_url. It syncs the property, unit and tenant directories into rental_property,
// rental_unit and rental_tenant, each tenant with the lease of their unit, the vendor directory into ledger_contact
// and work orders into work_item. Its reports say nothing of what changed, so each is listed whole every sync, which
// deletes what is gone. AppFolio takes 7 requests in 15 seconds of a database's keys, so they go at most one every
// 2.2 seconds. It pushes nothing.
func init() {
	Register(&Connector{
		ID: "appfolio", Name: "AppFolio", Integration: true,
		Info: Info{By: "AppFolio", Description: "Read the properties, units, tenants, leases, vendors and work orders", Category: CatProperty, Icon: IconList,
			Scopes: []string{"Read reports with the database's Reports API keys"}},
		Pace: 2200 * time.Millisecond,
		Keys: []Key{
			{Name: "database", Label: "AppFolio address", Help: "Where you sign in to AppFolio, like acme.appfolio.com"},
			{Name: "client_id", Label: "Client ID", Help: "In AppFolio, General Settings, Manage API Settings, Reports API Credentials: the Client ID. " +
				"Copy the pair that is there; generating a new one stops the old one working"},
			{Name: "client_secret", Label: "Client Secret", Secret: true, Help: "On the same page: the Client Secret"},
		},
		Sign: map[string]string{"Authorization": "Basic {key/client_id}:{key/client_secret}"},
		API:  "https://{+database}/api/v2/reports",
		Let: []Step{
			{As: "database", In: "{key/database}", Find: `^(?:[^/]*://)?([^/]*)`},
			{Only: Test{At: "database", Re: `^[^.]*$`}, As: "database", In: "{database}.appfolio.com"},
		},
		Who: &Who{Req: Req{Method: POST, Read: true, URL: "/property_directory.json", Body: Obj{"columns": []string{"property_id"}}}},
		Actions: map[string]*Action{
			"report": Call[afReport, any](Plan{
				Doc: "Run any of AppFolio's reports, with its filters, like { report: \"rent_roll\", filters: {} }, { report: \"delinquency\", filters: " +
					"{ tenant_statuses: [\"0\"] } } or { report: \"general_ledger\", filters: { posted_on_from: \"2026-10-01\", posted_on_to: \"2026-10-31\" } }: " +
					"{ results, next_page_url }, 5,000 rows a page (pass next_page_url as page for more). Dates are YYYY-MM-DD and amounts strings",
				Steps: []Step{
					{If: "page", Req: Req{Method: POST, Read: true, URL: "{+page}", Body: Obj{}}},
					{Unless: "page", Req: Req{Method: POST, Read: true, URL: "/{report}.json", Body: "{filters}"}},
				},
			}),
		},
		Sync: []*Sync{
			{Name: "properties", Gap: time.Second, Req: afList, Items: "results", ID: "property_id", Page: afPage, Deny: afDeny,
				Vars: Obj{"report": "property_directory", "filters": Obj{"property_visibility": "all", "columns": []string{"property_id", "property_name",
					"property_street", "property_street2", "property_city", "property_state", "property_zip", "property_type", "units", "sqft", "year_built",
					"market_rent", "owners", "portfolio", "site_manager_name", "visibility", "management_start_date", "management_end_date", "description",
					"property_created_on"}}},
				Doc: "Every property, hidden ones too, with its address, type, units, size, market rent, owners, portfolio and manager, by its property_id: listed whole each sync, 5,000 a page.",
				To: Map{Table: "rental_property",
					Cols: map[string]SQL{"title": "data->>'property_name'", "address": "nullif(btrim(concat_ws(' ', data->>'property_street', data->>'property_street2')), '')",
						"city": "data->>'property_city'", "state": "data->>'property_state'", "zip": "data->>'property_zip'", "type": "data->>'property_type'",
						"units": "archive.sync_num(data->>'units')::integer", "sqft": "archive.sync_num(replace(data->>'sqft', ',', ''))",
						"year_built": "archive.sync_num(data->>'year_built')::integer", "market_rent": "archive.sync_money(replace(data->>'market_rent', ',', ''), 'USD')",
						"owners": "data->>'owners'", "portfolio": "data->>'portfolio'", "manager": "data->>'site_manager_name'", "status": "data->>'visibility'",
						"managed_from": "archive.sync_day(data->>'management_start_date')", "managed_until": "archive.sync_day(data->>'management_end_date')",
						"description": "archive.sync_clip(data->>'description', 65536)", "made_on": "archive.sync_day(data->>'property_created_on')"}}},
			{Name: "units", Gap: time.Second, Req: afList, Items: "results", ID: "unit_id", Page: afPage, Deny: afDeny,
				Vars: Obj{"report": "unit_directory", "filters": Obj{"unit_visibility": "all", "columns": []string{"unit_id", "unit_name", "property_id",
					"unit_street", "unit_street2", "unit_city", "unit_state", "unit_zip", "unit_type", "unit_tags", "bedrooms", "bathrooms", "sqft", "market_rent",
					"rent_status", "rent_ready", "visibility", "marketing_description", "created_on"}}},
				Doc: "Every unit, hidden ones too, with its property, address, type, rooms, size, market rent, whether it is let and ready, and its tags, by its unit_id: listed whole each sync, 5,000 a page.",
				To: Map{Table: "rental_unit",
					Cols: map[string]SQL{"title": "data->>'unit_name'", "address": "nullif(btrim(concat_ws(' ', data->>'unit_street', data->>'unit_street2')), '')",
						"city": "data->>'unit_city'", "state": "data->>'unit_state'", "zip": "data->>'unit_zip'", "type": "data->>'unit_type'",
						"bedrooms": "archive.sync_num(data->>'bedrooms')", "bathrooms": "archive.sync_num(data->>'bathrooms')", "sqft": "archive.sync_num(replace(data->>'sqft', ',', ''))",
						"market_rent": "archive.sync_money(replace(data->>'market_rent', ',', ''), 'USD')", "rent_status": "data->>'rent_status'",
						"rent_ready": "coalesce(archive.sync_bool(data->'rent_ready'), lower(data->>'rent_ready') IN ('yes', 'true'))", "status": "data->>'visibility'",
						"tags": "data->>'unit_tags'", "description": "archive.sync_clip(data->>'marketing_description', 65536)", "made_on": "archive.sync_day(data->>'created_on')"},
					Refs: map[string]SQL{"property": "data->>'property_id'"}}},
			// A tenant is a person on a lease: one who moves again has another, so a tenant is the lease's and theirs.
			{Name: "tenants", Gap: time.Second, Req: afList, Items: "results", Page: afPage, Deny: afDeny,
				Key: []Reads{{Of: "{occupancy_id}/{selected_tenant_id}"}, {Of: "{occupancy_id}/{tenant}"}},
				Vars: Obj{"report": "tenant_directory", "filters": Obj{"tenant_visibility": "all", "tenant_statuses": []string{"0", "1", "2", "3", "4"},
					"tenant_types": []string{"0", "1", "2"}, "columns": []string{"selected_tenant_id", "occupancy_id", "tenant", "first_name", "last_name",
						"company_name", "primary_tenant", "status", "tenant_type", "phone_numbers", "emails", "property_id", "unit_id", "move_in", "move_out",
						"lease_from", "lease_to", "rent", "deposit", "tenant_tags"}}},
				Doc: "Every tenant, current, past, future, on notice or evicted, with their contact details and the lease of their unit, its dates, rent and deposit, by its occupancy_id and selected_tenant_id: listed whole each sync, 5,000 a page.",
				To: Map{Table: "rental_tenant",
					Cols: map[string]SQL{"first_name": "data->>'first_name'", "last_name": "data->>'last_name'", "company": "nullif(data->>'company_name', '')",
						"email": "archive.sync_email(split_part(data->>'emails', ',', 1))", "phones": "data->>'phone_numbers'", "status": "data->>'status'",
						"type": "data->>'tenant_type'", "primary_tenant": "coalesce(archive.sync_bool(data->'primary_tenant'), lower(data->>'primary_tenant') IN ('yes', 'true'))",
						"lease_source": "data->>'occupancy_id'", "move_in": "archive.sync_day(data->>'move_in')", "move_out": "archive.sync_day(data->>'move_out')",
						"lease_from": "archive.sync_day(data->>'lease_from')", "lease_to": "archive.sync_day(data->>'lease_to')",
						"rent": "archive.sync_money(replace(data->>'rent', ',', ''), 'USD')", "deposit": "archive.sync_money(replace(data->>'deposit', ',', ''), 'USD')",
						"tags": "data->>'tenant_tags'"},
					People: map[string]SQL{"person": "split_part(data->>'emails', ',', 1)"},
					Refs:   map[string]SQL{"property": "data->>'property_id'", "unit": "data->>'unit_id'"}}},
			{Name: "vendors", Gap: time.Second, Req: afList, Items: "results", ID: "vendor_id", Page: afPage, Deny: afDeny,
				Vars: Obj{"report": "vendor_directory", "filters": Obj{"vendor_visibility": "all", "columns": []string{"vendor_id", "company_name", "name",
					"street", "street2", "city", "state", "zip", "phone_numbers", "email"}}},
				Doc: "Every vendor, hidden ones too, with its company, contact, email, phones and address, by its vendor_id: listed whole each sync, 5,000 a page.",
				To: Map{Table: "ledger_contact",
					Cols: map[string]SQL{"kind": "'vendor'", "title": "coalesce(nullif(data->>'company_name', ''), data->>'name')", "company": "nullif(data->>'company_name', '')",
						"email": "archive.sync_email(split_part(data->>'email', ',', 1))", "phone": "data->>'phone_numbers'",
						"address": "nullif(btrim(concat_ws(', ', nullif(btrim(concat_ws(' ', data->>'street', data->>'street2')), ''), nullif(data->>'city', ''), " +
							"nullif(btrim(concat_ws(' ', data->>'state', data->>'zip')), ''))), '')"},
					People: map[string]SQL{"person": "split_part(data->>'email', ',', 1)"}}},
			// The work order report lists what it does by default, which AppFolio decides.
			{Name: "work_orders", Gap: time.Second, Req: afList, Items: "results", ID: "work_order_id", Page: afPage, Deny: afDeny,
				Vars: Obj{"report": "work_order", "filters": Obj{"columns": []string{"work_order_id", "work_order_number", "job_description",
					"service_request_description", "work_order_issue", "work_order_type", "status", "priority", "property_id", "property_name", "unit_name",
					"vendor", "assigned_user", "scheduled_start", "completed_on", "created_at"}}},
				Doc: "Work orders, with their issue, type, status, property, unit, vendor, dates and description, by its work_order_id: listed whole each sync, 5,000 a page.",
				To: Map{Table: "work_item",
					Cols: map[string]SQL{"title": "coalesce(nullif(data->>'job_description', ''), nullif(data->>'work_order_issue', ''), concat('Work order ', data->>'work_order_number'))",
						"type": "coalesce(nullif(data->>'work_order_type', ''), data->>'work_order_issue')", "status": "data->>'status'",
						"client_name": "nullif(concat_ws(' · ', nullif(data->>'property_name', ''), nullif(data->>'unit_name', '')), '')", "client_source": "data->>'property_id'",
						"assignee_name": "coalesce(nullif(data->>'vendor', ''), data->>'assigned_user')", "starts_on": "archive.sync_day(data->>'scheduled_start')",
						"done_on": "archive.sync_day(data->>'completed_on')", "description": "archive.sync_clip(data->>'service_request_description', 65536)",
						"made_at": "archive.sync_at(data->>'created_at')"}}},
		},
	})
}

var (
	afList = Req{Method: POST, Read: true, URL: "/{report}.json", Body: "{filters}"}
	afPage = NextURL{At: "next_page_url"}
	afDeny = []Deny{{Status: []int{403, 404}, Why: "AppFolio does not let these keys read {stream}: {message}"}}
)

type afReport struct {
	Report  string         `json:"report" jsonschema:"The report, like rent_roll, delinquency, tenant_ledger, charge_detail, bill_detail, general_ledger, owner_directory or lease_expiration_detail"`
	Filters map[string]any `json:"filters" jsonschema:"Its filters, {} for its defaults, like { properties: { properties_ids: [\"12\"] }, columns: [\"unit\", \"rent\"] }"`
	Page    string         `json:"page,omitempty" jsonschema:"The next_page_url of the call before"`
}
