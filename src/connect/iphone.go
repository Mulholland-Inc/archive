package connect

// Phone is what the Archive app on an iPhone pushes to /device/{stream}: what only lives on the phone, which no API reaches.
// It is no connector to connect: the app signs in with Archive's own OAuth, and each phone is a personal connection, provider iphone.
var Phone = []*Sync{
	{Name: "health", Doc: "Apple Health samples by their HealthKit UUID: { kind, value, unit, start, end, source, device }, kind in the other health " +
		"connectors' words, like steps, heart_rate or sleep_deep in minutes; new and deleted ones as HealthKit's anchored queries hear of them",
		To: Map{Table: "health_sample",
			Cols: map[string]SQL{"kind": "data->>'kind'", "value": "archive.sync_num(data->>'value')", "unit": "data->>'unit'",
				"starts_at": "archive.sync_at(data->>'start')", "ends_at": "archive.sync_at(data->>'end')",
				"device": "coalesce(nullif(data->>'device', ''), data->>'source')"}}},
	{Name: "workouts", Doc: "Apple Health workouts by their HealthKit UUID: { sport, start, end, seconds, kcal, meters, source, device }",
		To: Map{Table: "health_workout",
			Cols: map[string]SQL{"sport": "data->>'sport'", "starts_at": "archive.sync_at(data->>'start')", "ends_at": "archive.sync_at(data->>'end')",
				"minutes": "archive.sync_num(data->>'seconds') / 60", "calories": "archive.sync_num(data->>'kcal')", "distance": "archive.sync_num(data->>'meters')",
				"device": "coalesce(nullif(data->>'device', ''), data->>'source')", "route": "(data->>'route')::uuid"}}},
	{Name: "sleep", Doc: "Apple Health nights and naps by <first sample's UUID>, each the stages HealthKit has between going to bed and getting up: " +
		"{ day, start, end, asleep, deep, rem, core, awake (minutes), inBed, nap, heartRate, lowestHeartRate, hrv, respiratoryRate, source, device }",
		To: Map{Table: "health_sleep",
			Cols: map[string]SQL{"on_date": "archive.sync_day(data->>'day')", "starts_at": "archive.sync_at(data->>'start')", "ends_at": "archive.sync_at(data->>'end')",
				"asleep_minutes": "archive.sync_num(data->>'asleep')", "deep_minutes": "archive.sync_num(data->>'deep')", "rem_minutes": "archive.sync_num(data->>'rem')",
				"light_minutes": "archive.sync_num(data->>'core')", "awake_minutes": "archive.sync_num(data->>'awake')",
				"efficiency": "round(100 * archive.sync_num(data->>'asleep') / nullif(archive.sync_num(data->>'inBed'), 0), 1)",
				"heart_rate": "archive.sync_num(data->>'heartRate')", "lowest_heart_rate": "archive.sync_num(data->>'lowestHeartRate')",
				"hrv": "archive.sync_num(data->>'hrv')", "respiratory_rate": "archive.sync_num(data->>'respiratoryRate')", "nap": "archive.sync_bool(data->'nap')",
				"device": "coalesce(nullif(data->>'device', ''), data->>'source')"}}},
	{Name: "days", Doc: "Apple Health's activity rings by day: { day, steps, activeCalories, calories, meters, exerciseMinutes, standHours, restingHeartRate, hrv, " +
		"moveGoal, exerciseGoal, standGoal }, the last few days sent again as they fill in",
		To: Map{Table: "health_day",
			Cols: map[string]SQL{"kind": "'activity'", "on_date": "archive.sync_day(data->>'day')", "steps": "archive.sync_num(data->>'steps')",
				"active_calories": "archive.sync_num(data->>'activeCalories')", "calories": "archive.sync_num(data->>'calories')", "distance": "archive.sync_num(data->>'meters')",
				"active_minutes": "archive.sync_num(data->>'exerciseMinutes')", "resting_heart_rate": "archive.sync_num(data->>'restingHeartRate')",
				"hrv": "archive.sync_num(data->>'hrv')", "detail": "data - 'day'"}}},
	{Name: "records", Doc: "Apple Health's medical record, by the record's UUID, each a FHIR R4 resource { resource, organization }: what Health Records downloaded " +
		"from your hospitals, and what the phone measured or you logged that has no number of its own, as FHIR too: ECGs and hearing tests as Observations, " +
		"moods as an Observation of State of mind, medications as MedicationStatements and their doses as MedicationAdministrations",
		To: fhirRecord},
	{Name: "photos", Doc: "The Photos library by asset id: { name, kind, created, modified, width, height, duration, latitude, longitude, favorite, thumbnail }, " +
		"thumbnail a base64 JPEG the server keeps as a file; an original comes apart, to PUT /device/originals?photo=<asset id>&name=<file name>",
		To: Map{Table: "photo",
			Cols: map[string]SQL{"title": "data->>'name'", "kind": "data->>'kind'", "taken_at": "archive.sync_at(data->>'created')",
				"edited_at": "archive.sync_at(data->>'modified')", "width": "(data->>'width')::integer", "height": "(data->>'height')::integer",
				"seconds": "nullif((data->>'duration')::double precision, 0)", "latitude": "(data->>'latitude')::double precision",
				"longitude": "(data->>'longitude')::double precision", "favorite": "archive.sync_bool(data->'favorite')", "thumbnail": "(data->>'thumbnail')::uuid"}}},
	{Name: "contacts", Doc: "Contacts by identifier: { given, family, organization, job, emails, phones, city, state, country }",
		To: Map{Table: "crm_contact",
			Cols: map[string]SQL{"first_name": "data->>'given'", "last_name": "data->>'family'", "email": "archive.sync_email(data#>>'{emails,0}')",
				"phone": "data#>>'{phones,0}'", "job_title": "data->>'job'", "company_name": "data->>'organization'",
				"city": "data->>'city'", "state": "data->>'state'", "country": "data->>'country'"},
			People: map[string]SQL{"person": "data#>>'{emails,0}'"}}},
	{Name: "events", Doc: "Calendar events by <calendar id>/<event id>/<start>: { calendar, title, notes, location, start, end, allDay, status, rule, url, modified, " +
		"organizer: { email, name }, attendees: [{ email, name, role, status }] }, listed whole from a year back to two ahead",
		To: Map{Table: "calendar_event",
			Cols: map[string]SQL{"calendar": "data->>'calendar'", "title": "data->>'title'", "description": "archive.sync_clip(data->>'notes', 65536)",
				"location": "data->>'location'", "starts_at": "archive.sync_at(data->>'start')", "ends_at": "archive.sync_at(data->>'end')",
				"all_day": "archive.sync_bool(data->'allDay')", "status": "data->>'status'", "recurrence": "data->>'rule'", "url": "data->>'url'",
				"organizer_name": "data#>>'{organizer,name}'", "edited_at": "archive.sync_at(data->>'modified')"},
			People: map[string]SQL{"organizer": "data#>>'{organizer,email}'"},
			Children: []Kids{{Table: "calendar_attendee", From: "archive.sync_list(data->'attendees') a WHERE archive.sync_email(a->>'email') IS NOT NULL",
				Cols:   map[string]SQL{"name": "a->>'name'", "role": "a->>'role'", "response": "a->>'status'"},
				People: map[string]SQL{"person": "a->>'email'"}}}}},
	{Name: "reminders", Doc: "Reminders by identifier: { list, title, notes, url, priority, due, start, done, completed, created, modified }, listed whole",
		To: Map{Table: "work_item",
			Cols: map[string]SQL{"title": "data->>'title'", "type": "'reminder'", "status": "CASE WHEN archive.sync_bool(data->'done') THEN 'done' ELSE 'open' END",
				"client_name": "data->>'list'", "starts_on": "archive.sync_day(left(data->>'start', 10))", "due_on": "archive.sync_day(left(data->>'due', 10))",
				"done_on": "archive.sync_day(left(data->>'completed', 10))", "description": "archive.sync_clip(data->>'notes', 65536)", "url": "data->>'url'",
				"made_at": "archive.sync_at(data->>'created')", "edited_at": "archive.sync_at(data->>'modified')"}}},
	{Name: "places", Doc: "Places the phone saw you stay a while, by <arrival>/<latitude>,<longitude>: { title, address, city, country, category, latitude, longitude, " +
		"accuracy, arrived, departed }, departed missing while you are there",
		To: Map{Table: "place_visit",
			Cols: map[string]SQL{"title": "data->>'title'", "address": "data->>'address'", "city": "data->>'city'", "country": "data->>'country'",
				"category": "data->>'category'", "latitude": "archive.sync_num(data->>'latitude')::double precision",
				"longitude": "archive.sync_num(data->>'longitude')::double precision", "accuracy": "archive.sync_num(data->>'accuracy')::double precision",
				"arrived_at": "archive.sync_at(data->>'arrived')", "departed_at": "archive.sync_at(data->>'departed')"}}},
	{Name: "wallet_accounts", Doc: "Apple Card, Apple Cash and Apple Savings from Wallet, by FinanceKit's account id: { name, institution, kind, balance, available, " +
		"limit, currency }, listed whole",
		To: Map{Table: "bank_account",
			Cols: map[string]SQL{"title": "data->>'name'", "official_name": "data->>'institution'", "type": "data->>'kind'", "subtype": "data->>'institution'",
				"balance":      "archive.sync_money(data->>'balance', coalesce(nullif(data->>'currency', ''), 'USD'))",
				"available":    "archive.sync_money(data->>'available', coalesce(nullif(data->>'currency', ''), 'USD'))",
				"credit_limit": "archive.sync_money(data->>'limit', coalesce(nullif(data->>'currency', ''), 'USD'))"}}},
	{Name: "wallet_transactions", Doc: "Wallet transactions by FinanceKit's id: { account, posted, authorized, amount (negative is money out), currency, description, " +
		"merchant, category, kind, pending }, new, changed and deleted ones as FinanceKit's history hears of them",
		To: Map{Table: "bank_transaction",
			Cols: map[string]SQL{"posted_on": "archive.sync_day(left(data->>'posted', 10))", "authorized_on": "archive.sync_day(left(data->>'authorized', 10))",
				"amount":      "archive.sync_money(data->>'amount', coalesce(nullif(data->>'currency', ''), 'USD'))",
				"description": "data->>'description'", "merchant": "data->>'merchant'", "category": "data->>'category'", "detail": "data->>'kind'",
				"channel": "'apple pay'", "pending": "archive.sync_bool(data->'pending')"},
			Refs: map[string]SQL{"account": "data->>'account'"}}},
}

var phone = map[string]*Stream{}

func init() {
	for _, s := range Phone {
		phone[s.Name] = &Stream{Name: s.Name, Doc: s.Doc, Table: s.To.Table, SQL: s.To.sql(), Version: s.version()}
	}
}

// PhoneStream is the stream the iPhone app pushes as name, or nil.
func PhoneStream(name string) *Stream { return phone[name] }
