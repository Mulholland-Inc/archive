package connect

import "time"

// Gusto, through its hosted MCP server: everything its read tools show of the companies an admin who connects
// it reaches, their people and their payrolls. Its authorization server publishes a registration endpoint (RFC
// 7591) and takes PKCE with S256 and a client secret, so Archive registers its own client there like Notion's;
// Gusto's docs ask that only trusted clients connect, but name no approval. Archive asks for read scopes only,
// so its actions only read.
//
// One sign-in can reach several companies, and every tool but get_token_info acts on the one company_uuid
// names, or on one employee. get_token_info lists the companies, and names the account by them. Each stream
// walks what it reads, one listing of them all: each company, or each year of a company's history (a payroll list
// takes at most a year).
//
// None of its tools lists by when a record changed, and nothing it pushes reaches Archive, so a sync lists
// again, and what a stream remembers keeps that to what can have changed:
//   - Setup, like departments and pay schedules, and what changes with payrolls alone, time-off balances and
//     pay periods, is listed a day apart (Every); setup whole, so one gone is deleted.
//   - People are listed every sync, whole, but an employee whose version is the one read before is the same:
//     their employment history and work addresses, which take a call each, are read only for one new or
//     changed. Their terminations come with them.
//   - Payrolls, pay periods and contractor payments are listed from a year before the company joined Gusto
//     once, on the first sync, and after that only from three months back to 90 days ahead, as far as Gusto
//     lists what is coming. Those pages never delete. A processed payroll does not change, as Gusto corrects
//     one with another, off-cycle, so get_payroll reads each once, when it is first seen processed.
//
// Left out are the tools that read one record by its uuid, which the lists already hold, the onboarding and
// payroll-blocker tools, time records, which only third-party time tracking keeps, and the earnings summary,
// which payrolls add up to.
func init() {
	Register(&Connector{ID: "gusto", Name: "Gusto", Integration: true, MCP: "https://mcp.api.gusto.com", Pace: 500 * time.Millisecond, Busy: ToolBusy,
		Info: Info{By: "Gusto", Description: "Read the company, its people and its payrolls", Category: CatPayroll, Icon: IconUsers, Scopes: []string{"Read the company, employees, contractors and payrolls"}},
		Scopes: []string{"public", "companies:read", "employees:read", "contractors:read", "departments:read", "jobs:read", "compensations:read",
			"employments:read", "pay_schedules:read", "payrolls:read", "time_off_requests:read"},
		Who: &Who{Req: Req{Tool: "get_token_info"}, Each: "**/accessible_resources", Where: Test{At: "resource_type", Is: "company"}, Names: []Path{"name"}},
		Sync: []*Sync{
			{Name: "company", Every: gustoDaily, Parts: gustoCompanies, Req: Req{Tool: "get_company", Body: gustoOf}, Items: "company;", Key: []Reads{{Of: "{uuid;id;part}"}}, Updated: "updated_at",
				Doc: "Each company, as get_company answers: its name, entity type, EIN, status and admin, by its uuid; read a day apart.",
				To: Map{Table: "payroll_company",
					Cols: map[string]SQL{"name": "data->>'name'", "trade_name": "data->>'trade_name'", "entity_type": "data->>'entity_type'", "status": "data->>'company_status'", "tier": "data->>'tier'",
						"suspended": "archive.sync_bool(data->'is_suspended')", "joined_on": "archive.sync_day(data->>'join_date')",
						"admin_name":     "coalesce(nullif(btrim(data#>>'{primary_payroll_admin,full_name}'), ''), nullif(btrim(concat(data#>>'{primary_payroll_admin,first_name}', ' ', data#>>'{primary_payroll_admin,last_name}')), ''), data#>>'{primary_payroll_admin,business_name}')",
						"signatory_name": "coalesce(nullif(btrim(data#>>'{primary_signatory,full_name}'), ''), nullif(btrim(concat(data#>>'{primary_signatory,first_name}', ' ', data#>>'{primary_signatory,last_name}')), ''), data#>>'{primary_signatory,business_name}')"},
					People: map[string]SQL{"admin": "data#>>'{primary_payroll_admin,email}'", "signatory": "data#>>'{primary_signatory,email}'"},
					EINs:   map[string]SQL{"org": "data->>'ein'"}}},
			// 20 a page: each new or changed one costs two calls more, which at Gusto's pace must fit in a page's time.
			{Name: "employees", Parts: gustoStatuses, Items: "**", Key: gustoKey, Updated: "updated_at", Set: gustoCompany, Memo: "{version}",
				Req:  Req{Tool: "list_employees", Need: gustoNeed, Body: Obj{"company_uuid": "{in/company}", "terminated": "{in/terminated}", "include": "all_compensations,all_home_addresses,custom_fields"}},
				Page: ToolPage{Limit: "per,limit,page_size", Max: 20, Page: "page", After: "cursor,after"},
				Hydrate: []Hydrate{
					Fetch{Req: Req{Tool: "list_employee_employment_history", Body: Obj{"employee_uuid": "{uuid;id}"}}, Items: "**", As: "employment_history", Bare: true},
					Fetch{Req: Req{Tool: "get_employee_work_addresses", Body: Obj{"employee_uuid": "{uuid;id}"}}, Items: "**", As: "work_addresses", Bare: true},
				},
				Doc: "Every employee, the terminated too, as list_employees answers with their jobs, every compensation, home address and custom field, and " +
					"with their employment history and work addresses, which list_employee_employment_history and get_employee_work_addresses answer, " +
					"read again only when the employee's version changes; 100 a page, listed whole each sync, so one gone is deleted.",
				// An employee as a worker: their primary job, or else their first, says their title, start and pay, and their last
				// termination when they left. Their jobs up to that one, compensations, addresses, employment history and custom
				// fields are rows of their own.
				To: Map{Table: "payroll_worker",
					Cols: map[string]SQL{"kind": "'employee'", "name": gustoName, "first_name": "data->>'first_name'", "middle_initial": "data->>'middle_initial'", "last_name": "data->>'last_name'",
						"preferred_name": "data->>'preferred_first_name'", "email": "archive.sync_email(coalesce(nullif(data->>'work_email', ''), data->>'email'))", "personal_email": "archive.sync_email(data->>'email')",
						"phone": "data->>'phone'", "birthday": "archive.sync_day(data->>'date_of_birth')", "department_name": "data->>'department'", "active": "data->>'terminated' IS DISTINCT FROM 'true'",
						"onboarded": "archive.sync_bool(data->'onboarded')", "onboarding_status": "data->>'onboarding_status'", "payment_method": "data->>'payment_method'",
						"title": gustoJob + "->>'title'", "hired_on": "archive.sync_day(" + gustoJob + "->>'hire_date')", "pay_rate": "archive.sync_money(" + gustoJob + "->>'rate', 'USD')", "pay_per": gustoJob + "->>'payment_unit'",
						"flsa_status":   "(SELECT c->>'flsa_status' FROM archive.sync_list(" + gustoJob + "->'compensations') WITH ORDINALITY y(c, n) ORDER BY n DESC LIMIT 1)",
						"terminated_on": "(SELECT max(archive.sync_day(t->>'effective_date')) FROM archive.sync_list(data->'terminations') t)"},
					People: map[string]SQL{"person": "coalesce(nullif(data->>'work_email', ''), data->>'email')"},
					Refs:   map[string]SQL{"company": gustoRef, "department": "data->>'department_uuid'", "manager": "data->>'manager_uuid'"},
					Children: []Kids{
						{Table: "payroll_job", From: gustoJobs, Cols: map[string]SQL{"source_id": "coalesce(nullif(j->>'uuid', ''), j->>'id')", "title": "j->>'title'", "rate": "archive.sync_money(j->>'rate', 'USD')",
							"pay_per": "j->>'payment_unit'", "hired_on": "archive.sync_day(j->>'hire_date')", "primary_job": "coalesce(j->>'primary' = 'true', false)"}, Refs: map[string]SQL{"location": "j->>'location_uuid'"}},
						{Table: "payroll_compensation", From: "archive.sync_list(data->'jobs') WITH ORDINALITY x(j, i), archive.sync_list(j->'compensations') c WHERE " + gustoJobsTo,
							Cols: map[string]SQL{"job_source": "coalesce(nullif(j->>'uuid', ''), j->>'id')", "rate": "archive.sync_money(c->>'rate', 'USD')", "pay_per": "c->>'payment_unit'", "flsa_status": "c->>'flsa_status'", "effective_on": "archive.sync_day(c->>'effective_date')"}},
						{Table: "payroll_address", From: "archive.sync_list(data->'home_addresses') a", Cols: gustoHome},
						{Table: "payroll_address", From: "archive.sync_list(data->'work_addresses') a", Cols: gustoWork, Refs: map[string]SQL{"location": "a->>'location_uuid'"}},
						{Table: "payroll_employment", From: "archive.sync_list(data->'employment_history') e", Cols: map[string]SQL{"hired_on": "archive.sync_day(e->>'hire_date')", "terminated_on": "archive.sync_day(e->>'termination_date')", "status": "e->>'employment_status'"}},
						{Table: "payroll_custom_value", From: "archive.sync_list(data->'custom_fields') f", Cols: map[string]SQL{"name": "f->>'name'", "value": "f->>'value'"}, Refs: map[string]SQL{"field": "f->>'company_custom_field_id'"}},
					}}},
			{Name: "contractors", Parts: gustoStatuses, Items: "**", Key: gustoKey, Updated: "updated_at", Set: gustoCompany, Page: gustoPage,
				Req: Req{Tool: "get_contractors", Need: gustoNeed, Body: Obj{"company_uuid": "{in/company}", "terminated": "{in/terminated}"}},
				Doc: "Every contractor, the terminated too, as get_contractors answers, 100 a page, listed whole each sync, so one gone is deleted.",
				To: Map{Table: "payroll_worker",
					Cols: map[string]SQL{"kind": "'contractor'", "name": gustoName, "first_name": "data->>'first_name'", "last_name": "data->>'last_name'", "business_name": "data->>'business_name'",
						"contractor_type": "data->>'type'", "wage_type": "data->>'wage_type'", "email": "archive.sync_email(data->>'email')", "active": "data->>'is_active' IS DISTINCT FROM 'false'",
						"hired_on": "archive.sync_day(data->>'start_date')", "payment_method": "data->>'payment_method'",
						"pay_rate": "CASE WHEN lower(data->>'wage_type') = 'hourly' THEN archive.sync_money(data->>'hourly_rate', 'USD') END",
						"pay_per":  "CASE WHEN lower(data->>'wage_type') = 'hourly' THEN 'Hour' END"},
					People:   map[string]SQL{"person": "data->>'email'"},
					Refs:     map[string]SQL{"company": gustoRef, "department": "data->>'department_uuid'"},
					Children: []Kids{{Table: "payroll_address", From: "archive.sync_list(jsonb_build_array(data->'address')) a WHERE jsonb_typeof(a) = 'object'", Cols: gustoHome}}}},
			{Name: "departments", Every: gustoDaily, Parts: gustoCompanies, Items: "**", Key: gustoKey, Updated: "updated_at", Set: gustoCompany, Page: gustoPage, Req: Req{Tool: "get_departments", Need: gustoNeed, Body: gustoOf},
				Doc: "Every department, with who is in it, as get_departments answers, 100 a page, listed whole a day apart, so one gone is deleted.",
				To: Map{Table: "payroll_department", Cols: map[string]SQL{"title": "data->>'title'"}, Refs: map[string]SQL{"company": gustoRef},
					Children: []Kids{
						{Table: "payroll_department_member", From: "archive.sync_list(data->'employees') w", Cols: map[string]SQL{}, Refs: map[string]SQL{"worker": "coalesce(nullif(w->>'uuid', ''), w->>'id')"}},
						{Table: "payroll_department_member", From: "archive.sync_list(data->'contractors') w", Cols: map[string]SQL{}, Refs: map[string]SQL{"worker": "coalesce(nullif(w->>'uuid', ''), w->>'id')"}},
					}}},
			{Name: "locations", Every: gustoDaily, Parts: gustoCompanies, Items: "**", Key: gustoKey, Updated: "updated_at", Set: gustoCompany, Page: gustoPage, Req: Req{Tool: "get_locations", Need: gustoNeed, Body: gustoOf},
				Doc: "Every location, as get_locations answers, 100 a page, listed whole a day apart, so one gone is deleted.",
				To: Map{Table: "payroll_location", Refs: map[string]SQL{"company": gustoRef},
					Cols: map[string]SQL{"street_1": "data->>'street_1'", "street_2": "data->>'street_2'", "city": "data->>'city'", "state": "data->>'state'", "zip": "data->>'zip'", "country": "data->>'country'",
						"phone": "data->>'phone_number'", "active": "archive.sync_bool(data->'active')", "mailing_address": "archive.sync_bool(data->'mailing_address')", "filing_address": "archive.sync_bool(data->'filing_address')"}}},
			{Name: "pay_schedules", Every: gustoDaily, Parts: gustoCompanies, Items: "**", Key: gustoKey, Updated: "updated_at", Set: gustoCompany, Page: gustoPage, Req: Req{Tool: "get_pay_schedules", Need: gustoNeed, Body: gustoOf},
				Doc: "Every pay schedule, as get_pay_schedules answers, 100 a page, listed whole a day apart, so one gone is deleted.",
				To: Map{Table: "payroll_schedule", Refs: map[string]SQL{"company": gustoRef},
					Cols: map[string]SQL{"name": "coalesce(nullif(data->>'custom_name', ''), data->>'name')", "frequency": "data->>'frequency'", "anchor_pay_date": "archive.sync_day(data->>'anchor_pay_date')",
						"anchor_period_end": "archive.sync_day(data->>'anchor_end_of_pay_period')", "day_1": "archive.sync_num(data->>'day_1')", "day_2": "archive.sync_num(data->>'day_2')",
						"auto_pilot": "archive.sync_bool(data->'auto_pilot')", "active": "archive.sync_bool(data->'active')"}}},
			{Name: "pay_schedule_assignments", Every: gustoDaily, Parts: gustoCompanies, Key: []Reads{{Of: "{part}"}}, Set: gustoCompany, Req: Req{Tool: "list_pay_schedule_assignments", Body: gustoOf},
				Doc: "How each company assigns people to pay schedules, as list_pay_schedule_assignments answers, by the company's uuid; read a day apart.",
				To: Map{Table: "payroll_schedule_assignment", Cols: map[string]SQL{"type": "data->>'type'"},
					Refs: map[string]SQL{"company": gustoRef, "default_schedule": "data->>'default_pay_schedule_uuid'", "hourly_schedule": "data->>'hourly_pay_schedule_uuid'", "salaried_schedule": "data->>'salaried_pay_schedule_uuid'"},
					Children: []Kids{
						{Table: "payroll_schedule_member", From: "archive.sync_list(data->'employees') e", Cols: map[string]SQL{}, Refs: map[string]SQL{"worker": "e->>'employee_uuid'", "schedule": "e->>'pay_schedule_uuid'"}},
						{Table: "payroll_schedule_member", From: "archive.sync_list(data->'departments') d", Cols: map[string]SQL{}, Refs: map[string]SQL{"department": "d->>'department_uuid'", "schedule": "d->>'pay_schedule_uuid'"}},
					}}},
			// Default ones and a company's own are two lists of one answer.
			{Name: "earning_types", Every: gustoDaily, Items: "**/{in/kind}", Absent: "**", Key: gustoKey, Req: Req{Tool: "list_earning_types", Body: gustoOf},
				Parts: &Parts{Req: Req{Tool: "get_token_info"}, Items: "**/accessible_resources", Skip: gustoNotCompany,
					Under: &Parts{Fixed: []Obj{{"kind": "default"}, {"kind": "custom"}}, ID: "{up/uuid}/{kind}", Vals: Obj{"company": "{up/uuid}", "kind": "{kind}"}}},
				Set: Obj{"company_uuid": "{company_uuid;in/company}", "kind": "{in/kind}"},
				Doc: "Every earning type of each company, default and custom, as list_earning_types answers, by uuid; read a day apart.",
				To:  Map{Table: "payroll_earning_type", Cols: map[string]SQL{"name": "data->>'name'", "kind": "data->>'kind'"}, Refs: map[string]SQL{"company": gustoRef}}},
			{Name: "custom_fields", Every: gustoDaily, Parts: gustoCompanies, Items: "**", Key: gustoKey, Updated: "updated_at", Set: gustoCompany, Page: gustoPage, Req: Req{Tool: "list_custom_fields_schema", Need: gustoNeed, Body: gustoOf},
				Doc: "Every custom field each company defines, as list_custom_fields_schema answers, 100 a page, listed whole a day apart, so one gone is deleted.",
				To: Map{Table: "payroll_custom_field", Refs: map[string]SQL{"company": gustoRef},
					Cols: map[string]SQL{"name": "data->>'name'", "type": "data->>'type'", "description": "data->>'description'", "options": "coalesce(data->'selection_options', '[]'::jsonb)"}}},
			{Name: "time_off_requests", Parts: gustoCompanies, Items: "**", Key: gustoKey, Updated: "updated_at", Set: gustoCompany, Page: gustoPage, Req: Req{Tool: "get_time_off_requests", Need: gustoNeed, Body: gustoOf},
				Doc: "Every time-off request, as get_time_off_requests answers, 100 a page, listed whole each sync, so one gone is deleted.",
				// Its days, each with its hours, add up to its hours, from the first to the last.
				To: Map{Table: "payroll_time_off_request",
					Cols: map[string]SQL{"policy": "data->>'request_type'", "status": "data->>'status'", "employee_note": "data->>'employee_note'", "employer_note": "data->>'employer_note'",
						"employee_name": "coalesce(nullif(btrim(data#>>'{employee,full_name}'), ''), nullif(btrim(concat(data#>>'{employee,first_name}', ' ', data#>>'{employee,last_name}')), ''), data#>>'{employee,business_name}')",
						"approver_name": "coalesce(nullif(btrim(data#>>'{approver,full_name}'), ''), nullif(btrim(concat(data#>>'{approver,first_name}', ' ', data#>>'{approver,last_name}')), ''), data#>>'{approver,business_name}')",
						"starts_on":     "coalesce((SELECT min(archive.sync_day(k)) FROM " + gustoDays + "), archive.sync_day(data->>'start_date'))",
						"ends_on":       "coalesce((SELECT max(archive.sync_day(k)) FROM " + gustoDays + "), archive.sync_day(data->>'end_date'))",
						"hours":         "(SELECT coalesce(sum(archive.sync_num(v)), 0) FROM " + gustoDays + ")"},
					Refs:     map[string]SQL{"company": gustoRef, "worker": "coalesce(nullif(data#>>'{employee,uuid}', ''), data#>>'{employee,id}')", "approver": "coalesce(nullif(data#>>'{approver,uuid}', ''), data#>>'{approver,id}')"},
					Children: []Kids{{Table: "payroll_time_off_day", From: gustoDays, Cols: map[string]SQL{"on_date": "archive.sync_day(k)", "hours": "archive.sync_num(v)"}}}}},
			{Name: "time_off_balances", Every: gustoDaily, Parts: gustoCompanies, Items: "**", Key: []Reads{{Of: "{employee_uuid}"}}, Updated: "updated_at", Set: gustoCompany, Page: gustoPage, Req: Req{Tool: "get_time_off_balances", Need: gustoNeed, Body: gustoOf},
				Doc: "Each employee's time-off balances, per policy, as get_time_off_balances answers, 100 a page, listed whole a day apart, so one gone is deleted.",
				To: Map{Table: "payroll_time_off_balance", Cols: map[string]SQL{"name": "''"}, Refs: map[string]SQL{"company": gustoRef, "worker": "data->>'employee_uuid'"},
					Children: []Kids{{Table: "payroll_time_off_policy", From: "archive.sync_list(data->'balances') p",
						Cols: map[string]SQL{"name": "p->>'policy_name'", "policy_type": "p->>'policy_type'", "policy_source": "p->>'policy_uuid'", "hours": "archive.sync_num(p->>'balance_hours')",
							"accrued_hours": "archive.sync_num(p->>'accrued_hours')", "used_hours": "archive.sync_num(p->>'used_hours')", "pending_hours": "archive.sync_num(p->>'pending_hours')"}}}}},
			// A processed payroll is read with what get_payroll adds, every page of its employee compensations, once: after that
			// it is the same. Each line of what each employee was paid is a row: a tax is the employee's unless Gusto says it is the employer's.
			{Name: "payrolls", Gap: time.Second, Parts: gustoYears, Keep: true, Items: "**", Key: []Reads{{Of: "{payroll_uuid;uuid;id}"}}, Set: gustoCompany, Memo: "{processed}",
				Req:  Req{Tool: "list_payrolls", Body: Obj{"company_uuid": "{in/company}", "start_date": "{in/start}", "end_date": "{in/end}", "include": "taxes,totals", "processing_statuses": "processed,unprocessed", "payroll_types": "regular,off_cycle,external"}},
				Page: ToolPage{Limit: "per", Max: 25, Page: "page"},
				Hydrate: []Hydrate{Fetch{If: Test{At: "processed", Is: "true"}, Merge: true, Each: "employee_compensations", More: "employee_compensations_pagination/has_more",
					Req: Req{Tool: "get_payroll", Body: Obj{"company_uuid": "{company_uuid}", "payroll_uuid": "{payroll_uuid;uuid;id}", "employee_compensations_page": "{n}", "employee_compensations_per": 100}}}},
				Doc: "Every payroll, processed or not, of every type, with its totals and taxes, and once processed what get_payroll adds, read once: each " +
					"person's earnings, taxes, deductions, benefits and net pay. The first sync lists every year, 25 a page; the next ones from three " +
					"months back to 90 days ahead, by uuid, and delete nothing.",
				To: Map{Table: "payroll_run",
					Cols: map[string]SQL{"period_start": "archive.sync_day(data#>>'{pay_period,start_date}')", "period_end": "archive.sync_day(data#>>'{pay_period,end_date}')", "check_date": "archive.sync_day(data->>'check_date')",
						"deadline": "archive.sync_at(data->>'payroll_deadline')", "processed": "coalesce(data->>'processed' = 'true', false)", "processed_at": "archive.sync_at(data->>'processed_date')",
						"off_cycle": "coalesce(data->>'off_cycle' = 'true', false)", "off_cycle_reason": "data->>'off_cycle_reason'", "external": "coalesce(data->>'external' = 'true', false)",
						"gross_pay": "archive.sync_money(data#>>'{totals,gross_pay}', 'USD')", "net_pay": "archive.sync_money(data#>>'{totals,net_pay}', 'USD')", "employee_taxes": "archive.sync_money(data#>>'{totals,employee_taxes}', 'USD')",
						"employer_taxes": "archive.sync_money(data#>>'{totals,employer_taxes}', 'USD')", "benefits": "archive.sync_money(data#>>'{totals,benefits}', 'USD')", "deductions": "archive.sync_money(data#>>'{totals,deductions}', 'USD')",
						"reimbursements": "archive.sync_money(data#>>'{totals,reimbursements}', 'USD')", "check_amount": "archive.sync_money(data#>>'{totals,check_amount}', 'USD')", "company_debit": "archive.sync_money(data#>>'{totals,company_debit}', 'USD')"},
					Refs: map[string]SQL{"company": gustoRef, "schedule": "data#>>'{pay_period,pay_schedule_uuid}'"},
					Children: []Kids{
						{Table: "payroll_pay", From: "archive.sync_list(data->'employee_compensations') e", Refs: gustoEmployee,
							Cols: map[string]SQL{"gross_pay": "archive.sync_money(e->>'gross_pay', 'USD')", "net_pay": "archive.sync_money(e->>'net_pay', 'USD')", "check_amount": "archive.sync_money(e->>'check_amount', 'USD')",
								"deductions":     "archive.sync_money((SELECT sum(archive.sync_num(x->>'amount'))::text FROM archive.sync_list(e->'deductions') x), 'USD')",
								"employee_taxes": "archive.sync_money((SELECT sum(archive.sync_num(x->>'amount'))::text FROM archive.sync_list(e->'taxes') x WHERE x->>'employer' IS DISTINCT FROM 'true'), 'USD')",
								"employer_taxes": "archive.sync_money((SELECT sum(archive.sync_num(x->>'amount'))::text FROM archive.sync_list(e->'taxes') x WHERE x->>'employer' = 'true'), 'USD')",
								"hours":          "(SELECT coalesce(sum(archive.sync_num(x->>'hours')), 0) FROM archive.sync_list(e->'hourly_compensations') x)", "payment_method": "e->>'payment_method'"}},
						{Table: "payroll_pay_line", From: gustoEach + "e->'fixed_compensations') x", Refs: gustoEmployee, Cols: map[string]SQL{"kind": "'earning'", "name": "x->>'name'", "amount": "archive.sync_money(x->>'amount', 'USD')", "employer": "false"}},
						{Table: "payroll_pay_line", From: gustoEach + "e->'hourly_compensations') x", Refs: gustoEmployee, Cols: map[string]SQL{"kind": "'earning'", "name": "x->>'name'", "amount": "archive.sync_money(x->>'amount', 'USD')", "hours": "archive.sync_num(x->>'hours')", "employer": "false"}},
						{Table: "payroll_pay_line", From: gustoEach + "e->'paid_time_off') x", Refs: gustoEmployee, Cols: map[string]SQL{"kind": "'time_off'", "name": "x->>'name'", "hours": "archive.sync_num(x->>'hours')", "employer": "false"}},
						{Table: "payroll_pay_line", From: gustoEach + "e->'deductions') x", Refs: gustoEmployee, Cols: map[string]SQL{"kind": "'deduction'", "name": "x->>'name'", "amount": "archive.sync_money(x->>'amount', 'USD')", "employer": "false"}},
						{Table: "payroll_pay_line", From: gustoEach + "e->'benefits') x", Refs: gustoEmployee, Cols: map[string]SQL{"kind": "'benefit'", "name": "x->>'name'", "amount": "archive.sync_money(x->>'employee_deduction', 'USD')", "employer": "false"}},
						{Table: "payroll_pay_line", From: gustoEach + "e->'benefits') x WHERE archive.sync_num(x->>'company_contribution') <> 0", Refs: gustoEmployee, Cols: map[string]SQL{"kind": "'benefit'", "name": "x->>'name'", "amount": "archive.sync_money(x->>'company_contribution', 'USD')", "employer": "true"}},
						{Table: "payroll_pay_line", From: gustoEach + "e->'taxes') x", Refs: gustoEmployee, Cols: map[string]SQL{"kind": "'tax'", "name": "x->>'name'", "amount": "archive.sync_money(x->>'amount', 'USD')", "employer": "coalesce(x->>'employer' = 'true', false)"}},
					}}},
			{Name: "pay_periods", Every: gustoDaily, Parts: gustoYears, Keep: true, Items: "**", Key: []Reads{{Of: "{pay_schedule_uuid}/{start_date}"}}, Updated: "updated_at", Set: gustoCompany, Page: gustoPage, Req: Req{Tool: "list_pay_periods", Need: gustoNeed, Body: gustoSpan},
				Doc: "Every pay period and the payroll it is paid in, as list_pay_periods answers, 100 a page: every year on the first sync, and from three months back to 90 days ahead a day apart after, which deletes nothing.",
				To: Map{Table: "payroll_pay_period",
					Cols: map[string]SQL{"starts_on": "archive.sync_day(data->>'start_date')", "ends_on": "archive.sync_day(data->>'end_date')", "check_date": "archive.sync_day(data#>>'{payroll,check_date}')",
						"deadline": "archive.sync_at(data#>>'{payroll,payroll_deadline}')", "processed": "archive.sync_bool(data#>'{payroll,processed}')", "payroll_type": "data#>>'{payroll,payroll_type}'"},
					Refs: map[string]SQL{"company": gustoRef, "schedule": "data->>'pay_schedule_uuid'", "run": "data#>>'{payroll,payroll_uuid}'"}}},
			{Name: "contractor_payments", Parts: gustoYears, Keep: true, Items: "**", Key: gustoKey, Updated: "updated_at", Set: gustoCompany, Page: gustoPage, Req: Req{Tool: "get_contractor_payments", Need: gustoNeed, Body: gustoSpan},
				Doc: "Every contractor payment, as get_contractor_payments answers, 100 a page: every year on the first sync, and from three months back to 90 days ahead after, which deletes nothing.",
				To: Map{Table: "payroll_contractor_payment",
					Cols: map[string]SQL{"paid_on": "archive.sync_day(data->>'date')", "wage": "archive.sync_money(data->>'wage', 'USD')", "bonus": "archive.sync_money(data->>'bonus', 'USD')", "reimbursement": "archive.sync_money(data->>'reimbursement', 'USD')",
						"wage_total": "archive.sync_money(data->>'wage_total', 'USD')", "hours": "archive.sync_num(data->>'hours')", "hourly_rate": "archive.sync_money(data->>'hourly_rate', 'USD')", "wage_type": "data->>'wage_type'",
						"payment_method": "data->>'payment_method'", "status": "data->>'status'"},
					Refs: map[string]SQL{"company": gustoRef, "worker": "data->>'contractor_uuid'"}}},
			{Name: "contractor_payment_groups", Parts: gustoYears, Keep: true, Items: "**", Key: gustoKey, Updated: "updated_at", Set: gustoCompany, Page: gustoPage, Req: Req{Tool: "get_contractor_payment_groups", Need: gustoNeed, Body: gustoSpan},
				Doc: "Every batch of contractor payments, as get_contractor_payment_groups answers, 100 a page: every year on the first sync, and from three months back to 90 days ahead after, which deletes nothing.",
				To: Map{Table: "payroll_contractor_payment_group", Refs: map[string]SQL{"company": gustoRef},
					Cols: map[string]SQL{"check_date": "archive.sync_day(data->>'check_date')", "debit_date": "archive.sync_day(data->>'debit_date')", "status": "data->>'status'", "amount": "archive.sync_money(data#>>'{totals,amount}', 'USD')",
						"debit_amount": "archive.sync_money(data#>>'{totals,debit_amount}', 'USD')", "wages": "archive.sync_money(data#>>'{totals,wage_amount}', 'USD')", "reimbursements": "archive.sync_money(data#>>'{totals,reimbursement_amount}', 'USD')"}}},
			{Name: "tasks", Parts: gustoCompanies, Items: "**", Key: gustoKey, Updated: "updated_at", Set: gustoCompany, Page: gustoPage, Req: Req{Tool: "get_tasks", Need: gustoNeed, Body: gustoOf},
				Doc: "Each company's open to-dos in Gusto, like payroll blockers and documents to sign, as get_tasks answers, 100 a page, listed whole each sync, so one gone is deleted.",
				To: Map{Table: "payroll_task", Refs: map[string]SQL{"company": gustoRef},
					Cols: map[string]SQL{"title": "data->>'title'", "description": "data->>'message'", "due_on": "archive.sync_day(data->>'due_at')", "priority": "data->>'priority'", "category": "data->>'category'", "status": "data->>'status'", "url": "data->>'deep_link'"}}},
		}})
}

const (
	// gustoDaily is how far apart setup, which seldom changes, is read.
	gustoDaily      = 24 * time.Hour
	gustoRef    SQL = "data->>'company_uuid'"
	gustoName   SQL = "coalesce(nullif(btrim(data->>'full_name'), ''), nullif(btrim(concat(data->>'first_name', ' ', data->>'last_name')), ''), data->>'business_name')"
	gustoJob        = "(SELECT j FROM archive.sync_list(data->'jobs') WITH ORDINALITY x(j, i) ORDER BY j->>'primary' = 'true' DESC NULLS LAST, i LIMIT 1)"
	gustoJobsTo     = "i <= coalesce((SELECT min(n) FROM archive.sync_list(data->'jobs') WITH ORDINALITY y(k, n) WHERE k->>'primary' = 'true'), 2147483647)"
	gustoJobs   SQL = "archive.sync_list(data->'jobs') WITH ORDINALITY x(j, i) WHERE " + gustoJobsTo
	gustoDays   SQL = "jsonb_each_text(CASE WHEN jsonb_typeof(data->'days') = 'object' THEN data->'days' ELSE '{}'::jsonb END) d(k, v) WHERE archive.sync_day(k) IS NOT NULL"
	gustoEach       = "archive.sync_list(data->'employee_compensations') e, archive.sync_list("
)

var (
	gustoNeed       = []string{"company_uuid"}
	gustoOf         = Obj{"company_uuid": "{in/company}"}
	gustoSpan       = Obj{"company_uuid": "{in/company}", "start_date": "{in/start}", "end_date": "{in/end}"}
	gustoKey        = []Reads{{Of: "{uuid;id}"}}
	gustoCompany    = Obj{"company_uuid": "{company_uuid;in/company}"}
	gustoPage       = ToolPage{Limit: "per,limit,page_size", Max: 100, Page: "page", After: "cursor,after"}
	gustoNotCompany = []Test{{At: "resource_type", Is: "company", Off: true}, {At: "uuid", Off: true}}
	// gustoCompanies are the companies the token reaches, as get_token_info lists them.
	gustoCompanies = &Parts{Req: Req{Tool: "get_token_info"}, Items: "**/accessible_resources", Skip: gustoNotCompany, ID: "{uuid}", Vals: Obj{"company": "{uuid}"}}
	// gustoStatuses are each company twice: as it is, and for those no longer active, whom a list leaves out unless asked for.
	gustoStatuses = &Parts{Req: Req{Tool: "get_token_info"}, Items: "**/accessible_resources", Skip: gustoNotCompany,
		Under: &Parts{Fixed: []Obj{{"tag": "active"}, {"tag": "gone", "terminated": true}}, ID: "{up/uuid}/{tag}", Vals: Obj{"company": "{up/uuid}", "terminated": "{terminated}"}}}
	// gustoYears are each company's years to list, to 90 days ahead, as far as Gusto lists what is coming: from a year before
	// it joined Gusto, for history brought in with it, ten years back when it does not say, until the stream has listed them
	// once, and after that the recent months alone.
	gustoYears = &Parts{Req: Req{Tool: "get_token_info"}, Items: "**/accessible_resources", Skip: gustoNotCompany,
		Under: &Parts{First: true, Req: Req{Tool: "get_company", Body: Obj{"company_uuid": "{up/uuid}"}},
			Under: &Parts{Span: &Span{Last: Shift{0, 0, 90}, First: Shift{-10, 0, 0}, From: "{up/join_date}", Back: Shift{-1, 0, 0}, Then: Shift{0, -3, -90}, Step: Shift{1, 0, 0}},
				ID: "{up/uuid}/{start}", Vals: Obj{"company": "{up/uuid}", "start": "{start}", "end": "{end}"}}}}
	gustoEmployee = map[string]SQL{"worker": "e->>'employee_uuid'"}
	gustoHome     = map[string]SQL{"kind": "'home'", "street_1": "a->>'street_1'", "street_2": "a->>'street_2'", "city": "a->>'city'", "state": "a->>'state'", "zip": "a->>'zip'", "country": "a->>'country'", "active": "archive.sync_bool(a->'active')", "effective_on": "archive.sync_day(a->>'effective_date')"}
	gustoWork     = map[string]SQL{"kind": "'work'", "street_1": "a->>'street_1'", "street_2": "a->>'street_2'", "city": "a->>'city'", "state": "a->>'state'", "zip": "a->>'zip'", "country": "a->>'country'", "active": "archive.sync_bool(a->'active')", "effective_on": "archive.sync_day(a->>'effective_date')"}
)
