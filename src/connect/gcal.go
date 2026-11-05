package connect

import (
	"time"

	"golang.org/x/oauth2"
)

func init() {
	Register(&Connector{
		ID: "gcal", Name: "Google Calendar", Env: "ARCHIVE_GOOGLE",
		Info: Info{By: "Google", Description: "Read your calendars’ events and who is invited, and book or change events when you ask", Category: CatCalendar, Icon: IconCalendar,
			Scopes: []string{"Read your calendars and events", "Make, change and cancel events when you ask"}},
		API:  "https://www.googleapis.com/calendar/v3",
		Busy: []string{"Quota exceeded", "rateLimitExceeded", "userRateLimitExceeded", "Rate Limit Exceeded"},
		Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.google.com/o/oauth2/v2/auth", TokenURL: "https://oauth2.googleapis.com/token",
			AuthStyle: oauth2.AuthStyleInParams},
		Revoke: "https://oauth2.googleapis.com/revoke",
		Scopes: []string{"https://www.googleapis.com/auth/calendar"},
		Params: map[string]string{"access_type": "offline", "prompt": "consent"},
		Who:    &Who{Req: Req{URL: "/calendars/primary"}, Names: []Path{"id"}, Lower: true},
		Actions: map[string]*Action{
			"calendars": Call[struct{}, []gcalCalendar](Plan{
				Doc:   "The calendars on your list: [{ id, summary, description, timeZone, accessRole, primary }]",
				Steps: []Step{{As: "got", Req: Req{URL: "/users/me/calendarList?maxResults=250&fields=items(id,summary,description,timeZone,accessRole,primary)"}}},
				Out:   "{got/items|[]}",
			}),
			"events": Call[gcalFind, gcalFound](Plan{
				Doc: "Events of a calendar, each occurrence of a series on its own, soonest first, maybe only between two times or with " +
					"some words: { events: [{ id, summary, description, location, start, end, status, organizer, attendees, htmlLink, hangoutLink }], next } " +
					"(pass next as page for more)",
				Clamp: map[string][2]int{"limit": {50, 250}},
				Steps: []Step{
					{Unless: "from", As: "from", In: "{now}"},
					{As: "got", Req: Req{URL: "/calendars/{calendar|primary}/events", Query: Q{"singleEvents": "true", "orderBy": "startTime", "maxResults": "{limit}",
						"timeMin": "{from}", "timeMax": "{to}", "q": "{query}", "pageToken": "{page}"}}},
				},
				Out: Obj{"events": "{got/items|[]}", "next": "{got/nextPageToken|}"},
			}),
			"create": Call[gcalNew, map[string]any](Plan{
				Doc: "Put an event on a calendar and invite people to it, maybe with a Google Meet link: the event, as events returns one",
				Steps: []Step{
					{Unless: "summary", Fail: gcalNeeds}, {Unless: "start", Fail: gcalNeeds}, {Unless: "end", Fail: gcalNeeds},
					gcalNotify, gcalQuiet, gcalStart, gcalStartDay, gcalEnd, gcalEndDay, gcalPeople, gcalMeet, gcalVersion,
					{Req: Req{Method: POST, URL: "/calendars/{calendar|primary}/events", Query: gcalSend, Body: gcalEvent}},
				},
			}),
			"update": Call[gcalEdit, map[string]any](Plan{
				Doc: "Change an event, a whole series when its id is the series': only what you give changes. Returns the event, as events returns one",
				Steps: []Step{
					gcalNotify, gcalQuiet, gcalStart, gcalStartDay, gcalEnd, gcalEndDay, gcalPeople, gcalMeet, gcalVersion,
					{Req: Req{Method: PATCH, URL: "/calendars/{calendar|primary}/events/{id}", Query: gcalSend, Body: gcalEvent}},
				},
			}),
			"delete": Call[gcalGone, gcalDeleted](Plan{
				Doc: "Cancel an event, or a whole series by the series' id, telling who was invited: { id }",
				Steps: []Step{
					gcalNotify, gcalQuiet,
					{Req: Req{Method: DELETE, URL: "/calendars/{calendar|primary}/events/{id}", Query: Q{"sendUpdates": "{notify}"}}},
				},
				Out: Obj{"id": "{id}"},
			}),
		},
		Listen: &Listen{Renew: 6 * 24 * time.Hour, Until: 7*24*time.Hour - time.Hour, Over: gcalCalendars, Mint: "archive-{nonce}", Secret: true, Replace: true, Lax: true,
			Watch:    Req{Method: POST, URL: "/calendars/{part}/events/watch", Body: Obj{"id": "{key}", "type": "web_hook", "address": "{hook}", "token": "{secret}", "expiration": "{until_ms}"}},
			Resource: "{answer/resourceId}", Expires: "expiration",
			Unwatch: &Req{Method: POST, URL: "/channels/stop", Body: Obj{"id": "{key}", "resourceId": "{resource}"}},
			Kept:    &Kept{IDHeader: "X-Goog-Channel-ID", TokenHeader: "X-Goog-Channel-Token", Quiet: "X-Goog-Resource-State", QuietIs: "sync"},
		},
		Sync: []*Sync{{Name: "events", Gap: time.Second, Parts: gcalCalendars,
			Req:   Req{URL: "/calendars/{part}/events", Query: Q{"maxResults": "250", "pageToken": "{page}"}},
			Items: "items", Absent: "kind", Key: []Reads{{Of: "{part}/{id}"}}, Updated: "updated", Page: Cursor{At: "nextPageToken"},
			Change: Resume{At: "nextSyncToken", Expired: []int{410}, Gone: []Test{{At: "status", Is: "cancelled"}},
				Req: Req{URL: "/calendars/{part}/events", Query: Q{"maxResults": "250", "syncToken": "{token}", "showDeleted": "true", "pageToken": "{page}"}}},
			Set:     Obj{"calendarId": "{part}", "calendar": "{in/name}"},
			Hydrate: []Hydrate{Decode{Codec: Words, At: "description", As: "description", If: Test{At: "description", Re: "<"}}},
			Doc: "Every event of each calendar on your list, shared, subscribed and hidden ones too, past and future, by <calendar id>/<event id>: a series as one event with " +
				"its rule, and each occurrence that differs from it as another, with its attendees and their answers and its description, cut " +
				"at 64 KB. Listed whole the first time, 250 a page and a calendar at a time, and then each calendar's changes since; a calendar " +
				"gone, or a sync token Google no longer keeps, lists them all again.",
			To: Map{Table: "calendar_event",
				Cols: map[string]SQL{"calendar": "data->>'calendar'", "title": "data->>'summary'", "description": "archive.sync_clip(data->>'description', 65536)", "location": "data->>'location'",
					"starts_at": "coalesce(archive.sync_at(data#>>'{start,dateTime}'), archive.sync_at(archive.sync_day(data#>>'{start,date}')::text || 'T00:00:00Z'))",
					"ends_at":   "coalesce(archive.sync_at(data#>>'{end,dateTime}'), archive.sync_at(archive.sync_day(data#>>'{end,date}')::text || 'T00:00:00Z'))",
					"all_day":   "nullif(data#>>'{start,date}', '') IS NOT NULL", "status": "data->>'status'",
					"recurrence":     `(SELECT string_agg(r#>>'{}', E'\n') FROM archive.sync_list(data->'recurrence') r)`,
					"organizer_name": "data#>>'{organizer,displayName}'", "url": "data->>'htmlLink'",
					"meeting_url": `coalesce(nullif(data->>'hangoutLink', ''), (SELECT e->>'uri' FROM archive.sync_list(data#>'{conferenceData,entryPoints}') e WHERE e->>'entryPointType' = 'video' AND nullif(e->>'uri', '') IS NOT NULL LIMIT 1))`,
					"edited_at":   "archive.sync_at(data->>'updated')"},
				People: map[string]SQL{"organizer": "data#>>'{organizer,email}'"},
				Refs:   map[string]SQL{"series": "(data->>'calendarId') || '/' || nullif(data->>'recurringEventId', '')"},
				Children: []Kids{{Table: "calendar_attendee", From: "archive.sync_list(data->'attendees') a WHERE archive.sync_email(a->>'email') IS NOT NULL",
					Cols: map[string]SQL{"name": "a->>'displayName'", "role": "CASE WHEN a->>'resource' = 'true' THEN 'resource' WHEN a->>'optional' = 'true' THEN 'optional' ELSE 'required' END",
						"response": "CASE WHEN a->>'responseStatus' IN ('accepted', 'declined', 'tentative') THEN a->>'responseStatus' ELSE 'none' END"},
					People: map[string]SQL{"person": "a->>'email'"}}}}}},
	})
}

const (
	gcalDay   = `^\d{4}-\d{2}-\d{2}$`
	gcalNeeds = "an event needs a summary, a start and an end"
)

var (
	gcalCalendars = &Parts{Req: Req{URL: "/users/me/calendarList", Query: Q{"maxResults": "250", "showHidden": "true", "fields": "nextPageToken,items(id,summary,summaryOverride)", "pageToken": "{page}"}},
		Items: "items", Page: Cursor{At: "nextPageToken"}, Skip: []Test{{At: "id", Re: "#(holiday|contacts)@"}}, Vals: Obj{"name": "{summaryOverride;summary}"}}
	gcalNotify   = Step{As: "notify", In: "all"}
	gcalQuiet    = Step{If: "quiet", As: "notify", In: "none"}
	gcalStart    = Step{If: "start", As: "begin", In: Obj{"dateTime": "{start}", "timeZone": "{timeZone}"}}
	gcalStartDay = Step{If: "start", Only: Test{At: "start", Re: gcalDay}, As: "begin", In: Obj{"date": "{start}"}}
	gcalEnd      = Step{If: "end", As: "finish", In: Obj{"dateTime": "{end}", "timeZone": "{timeZone}"}}
	gcalEndDay   = Step{If: "end", Only: Test{At: "end", Re: gcalDay}, As: "finish", In: Obj{"date": "{end}"}}
	gcalPeople   = Step{Each: "attendees", As: "people", In: Obj{"email": "{it}"}}
	gcalMeet     = Step{If: "meet", As: "conference", In: Obj{"createRequest": Obj{"requestId": "{nonce}", "conferenceSolutionKey": Obj{"type": "hangoutsMeet"}}}}
	gcalVersion  = Step{If: "meet", As: "version", In: "1"}
	gcalSend     = Q{"sendUpdates": "{notify}", "conferenceDataVersion": "{version}"}
	gcalEvent    = Obj{"summary": "{summary}", "description": "{description}", "location": "{location}", "start": "{begin}", "end": "{finish}", "attendees": "{people}", "conferenceData": "{conference}"}
)

type (
	gcalFind struct {
		Calendar string `json:"calendar,omitempty" jsonschema:"The calendar's id, from calendars; the primary one when left out"`
		From     string `json:"from,omitempty" jsonschema:"Only those ending after this time, RFC 3339 like 2026-11-03T00:00:00Z; now when left out"`
		To       string `json:"to,omitempty" jsonschema:"Only those starting before this time, RFC 3339"`
		Query    string `json:"query,omitempty" jsonschema:"Words in the event, its attendees or its location"`
		Limit    int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 250; 50 when left out"`
		Page     string `json:"page,omitempty" jsonschema:"The next of the call before, for the events after it"`
	}
	gcalNew struct {
		Calendar    string   `json:"calendar,omitempty" jsonschema:"The calendar's id, from calendars; the primary one when left out"`
		Summary     string   `json:"summary,omitempty" jsonschema:"The event's title"`
		Start       string   `json:"start,omitempty" jsonschema:"When it starts, RFC 3339 like 2026-11-04T15:00:00-08:00, or a date like 2026-11-04 for an all-day event"`
		End         string   `json:"end,omitempty" jsonschema:"When it ends, as start; an all-day event's is the day after its last"`
		TimeZone    string   `json:"timeZone,omitempty" jsonschema:"An IANA time zone, like America/Los_Angeles; the calendar's when left out"`
		Description string   `json:"description,omitempty" jsonschema:"What it is about"`
		Location    string   `json:"location,omitempty" jsonschema:"Where it is"`
		Attendees   []string `json:"attendees,omitempty" jsonschema:"The email addresses of who to invite; on update, everyone invited"`
		Meet        bool     `json:"meet,omitempty" jsonschema:"Add a Google Meet link"`
		Quiet       bool     `json:"quiet,omitempty" jsonschema:"Don't email the people invited"`
	}
	gcalEdit struct {
		ID string `json:"id" jsonschema:"The event's id, from events"`
		gcalNew
	}
	gcalGone struct {
		Calendar string `json:"calendar,omitempty" jsonschema:"The calendar's id, from calendars; the primary one when left out"`
		ID       string `json:"id" jsonschema:"The event's id, from events"`
		Quiet    bool   `json:"quiet,omitempty" jsonschema:"Don't email the people invited"`
	}
	gcalCalendar struct {
		ID          string `json:"id"`
		Summary     string `json:"summary"`
		Description string `json:"description,omitempty"`
		TimeZone    string `json:"timeZone,omitempty"`
		AccessRole  string `json:"accessRole,omitempty"`
		Primary     bool   `json:"primary,omitempty"`
	}
	gcalFound struct {
		Events []map[string]any `json:"events"`
		Next   string           `json:"next" jsonschema:"Pass as page for more; empty when there is no more"`
	}
	gcalDeleted struct {
		ID string `json:"id"`
	}
)
