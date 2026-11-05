package connect

// Phone is what the Archive app on an iPhone pushes to /device/{stream}: what only lives on the phone, which no API reaches.
// It is no connector to connect: the app signs in with Archive's own OAuth, and each phone is a personal connection, provider iphone.
var Phone = []*Sync{
	{Name: "health", Doc: "Apple Health samples by their HealthKit UUID: { type, value, unit, start, end, source, device, meta }, " +
		"new and deleted ones as HealthKit's anchored queries hear of them",
		To: Map{Table: "health_sample",
			Cols: map[string]SQL{"kind": "data->>'type'", "value": "(data->>'value')::double precision", "unit": "data->>'unit'",
				"starts_at": "archive.sync_at(data->>'start')", "ends_at": "archive.sync_at(data->>'end')",
				"source": "data->>'source'", "device": "data->>'device'", "meta": "nullif(data->'meta', '{}'::jsonb)"}}},
	{Name: "photos", Doc: "The Photos library by asset id: { name, kind, created, modified, width, height, duration, latitude, longitude, favorite, thumbnail }, " +
		"thumbnail a base64 JPEG the server keeps as a file",
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
}

var phone = map[string]*Stream{}

func init() {
	for _, s := range Phone {
		phone[s.Name] = &Stream{Name: s.Name, Doc: s.Doc, Table: s.To.Table, SQL: s.To.sql(), Version: s.version()}
	}
}

// PhoneStream is the stream the iPhone app pushes as name, or nil.
func PhoneStream(name string) *Stream { return phone[name] }
