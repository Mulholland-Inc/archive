package connect

import (
	"time"

	"golang.org/x/oauth2"
)

// Cal.com, through its API v2: the meetings people book with a member, read, and cancelled only when asked. Its OAuth
// client, made at app.cal.com/settings/developer/oauth and approved by Cal.com before it works, lists
// <API>/connect/calcom/callback as a redirect URI, with the scopes here. An access token lasts 30 minutes and each
// refresh names the refresh token to keep. Cal.com has nowhere to revoke one: a person removes Archive in Cal.com.
// Each member connects their own Cal.com user. Each endpoint has its own version, in cal-api-version; without it
// Cal.com answers with an older one.
//
// It syncs every booking of the user, past and future, cancelled ones too, into calendar_event, its attendees and
// guests into calendar_attendee, and what they answered when they booked into its description. Cal.com stamps no
// change time on a booking that was never changed, so each sync lists them all, 100 a page. Its webhooks are made
// per user by hand, so it pushes nothing: connecting, asking and reading start a sync.
func init() {
	Register(&Connector{
		ID: "calcom", Name: "Cal.com", Env: "ARCHIVE_CALCOM",
		Info: Info{By: "Cal.com", Description: "Read the meetings people book with you and who booked them, and cancel one when you ask", Category: CatCalendar, Icon: IconCalendar,
			Scopes: []string{"Read your profile, bookings and event types", "Cancel a booking when you ask"}},
		API: "https://api.cal.com/v2",
		Endpoint: oauth2.Endpoint{AuthURL: "https://app.cal.com/auth/oauth2/authorize", TokenURL: "https://api.cal.com/v2/auth/oauth2/token",
			AuthStyle: oauth2.AuthStyleInParams},
		Scopes: []string{"PROFILE_READ", "BOOKING_READ", "BOOKING_WRITE", "EVENT_TYPE_READ"},
		Who:    &Who{Req: Req{URL: "/me"}, Names: []Path{"data/email"}, Lower: true},
		Actions: map[string]*Action{
			"event_types": Call[struct{}, []map[string]any](Plan{
				Doc: "Your event types, the kinds of meeting people can book with you, hidden ones too: [{ id, title, slug, description, " +
					"lengthInMinutes, locations, hidden, bookingUrl, price, currency }]",
				Steps: []Step{{As: "got", Req: Req{URL: "/event-types", Header: calcomTypes}}},
				Out:   "{got/data|[]}",
			}),
			"bookings": Call[calcomFind, calcomFound](Plan{
				Doc: "Bookings with you, soonest first, maybe only of a status, between two times or with an attendee: { bookings: [{ uid, title, " +
					"description, status, start, end, duration, eventType, location, hosts, attendees, guests, bookingFieldsResponses, " +
					"cancellationReason }], next } (pass next as page for more)",
				Clamp: map[string][2]int{"limit": {50, 100}},
				Enum:  map[string][]string{"status": {"upcoming", "recurring", "past", "cancelled", "unconfirmed"}},
				Steps: []Step{{As: "got", Req: Req{URL: "/bookings", Header: calcomBookings, Query: Q{"sortStart": "asc", "limit": "{limit}",
					"status": "{status}", "afterStart": "{from}", "beforeEnd": "{to}", "attendeeEmail": "{attendee}", "cursor": "{page}"}}}},
				Out: Obj{"bookings": "{got/data|[]}", "next": "{got/pagination/nextCursor|}"},
			}),
			"cancel": Call[calcomCancel, map[string]any](Plan{
				Doc: "Cancel a booking, which Cal.com emails its attendees about with the reason; a recurring one's later bookings too with " +
					"all. Only when the person asks to. Returns the booking, or the bookings of a recurring one",
				Steps: []Step{{As: "got", Req: Req{Method: POST, URL: "/bookings/{uid}/cancel", Header: calcomCancels,
					Body: Obj{"cancellationReason": "{reason}", "cancelSubsequentBookings": "{all}"}}}},
				Out: Obj{"booking": "{got/data}"},
			}),
		},
		Sync: []*Sync{{Name: "bookings", Gap: time.Second,
			Req:   Req{URL: "/bookings", Header: calcomBookings, Query: Q{"sortCreated": "asc", "limit": "100", "cursor": "{page}"}},
			Items: "data", ID: "uid", Updated: "updatedAt", Page: Cursor{At: "pagination/nextCursor", More: "pagination/hasMore"},
			Doc: "Every booking with you, past and future, cancelled, rejected and unconfirmed ones too, by its uid, with its attendees and " +
				"guests, and what they answered when they booked, cut at 64 KB. Listed whole every sync, 100 a page, which deletes those gone.",
			To: Map{Table: "calendar_event",
				Cols: map[string]SQL{"calendar": "'Cal.com'", "title": "data->>'title'", "location": "data->>'location'",
					"description": `archive.sync_clip(nullif(concat_ws(E'\n', nullif(data->>'description', ''), (SELECT string_agg(concat(r.key, ': ', r.value #>> '{}'), E'\n' ORDER BY r.key)
						FROM jsonb_each(CASE WHEN jsonb_typeof(data->'bookingFieldsResponses') = 'object' THEN data->'bookingFieldsResponses' ELSE '{}' END) r
						WHERE r.key NOT IN ('name', 'email', 'guests', 'location') AND jsonb_typeof(r.value) IN ('string', 'number', 'boolean') AND nullif(r.value #>> '{}', '') IS NOT NULL)), ''), 65536)`,
					"starts_at": "archive.sync_at(data->>'start')", "ends_at": "archive.sync_at(data->>'end')", "all_day": "false",
					"status": "CASE data->>'status' WHEN 'accepted' THEN 'confirmed' WHEN 'cancelled' THEN 'cancelled' WHEN 'rejected' THEN 'cancelled' " +
						"WHEN 'pending' THEN 'tentative' WHEN 'awaiting_host' THEN 'tentative' END",
					"organizer_name": "data#>>'{hosts,0,name}'",
					"meeting_url":    "CASE WHEN data->>'location' ~ '^https?://' THEN data->>'location' ELSE nullif(data->>'meetingUrl', '') END",
					"edited_at":      "archive.sync_at(coalesce(data->>'updatedAt', data->>'createdAt'))"},
				People: map[string]SQL{"organizer": "data#>>'{hosts,0,email}'"},
				Children: []Kids{
					{Table: "calendar_attendee", From: "archive.sync_list(data->'attendees') a WHERE archive.sync_email(a->>'email') IS NOT NULL",
						Cols: map[string]SQL{"name": "a->>'name'", "role": "'required'",
							"response": "CASE data->>'status' WHEN 'accepted' THEN 'accepted' WHEN 'cancelled' THEN 'declined' WHEN 'rejected' THEN 'declined' ELSE 'none' END"},
						People: map[string]SQL{"person": "a->>'email'"}},
					{Table: "calendar_attendee", From: "archive.sync_list(data->'guests') g WHERE archive.sync_email(g #>> '{}') IS NOT NULL",
						Cols:   map[string]SQL{"name": "NULL", "role": "'optional'", "response": "'none'"},
						People: map[string]SQL{"person": "g #>> '{}'"}}}}}},
	})
}

// The versions of the endpoints Archive reads them at.
var (
	calcomTypes    = map[string][]string{"cal-api-version": {"2026-06-12"}}
	calcomBookings = map[string][]string{"cal-api-version": {"2026-05-01"}}
	calcomCancels  = map[string][]string{"cal-api-version": {"2026-02-25"}}
)

type (
	calcomFind struct {
		Status   string `json:"status,omitempty" jsonschema:"upcoming, recurring, past, cancelled or unconfirmed; all of them when left out"`
		From     string `json:"from,omitempty" jsonschema:"Only those starting at or after this time, RFC 3339 like 2026-11-03T00:00:00Z"`
		To       string `json:"to,omitempty" jsonschema:"Only those ending before this time, RFC 3339"`
		Attendee string `json:"attendee,omitempty" jsonschema:"Only those with an attendee of this email address"`
		Limit    int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 100; 50 when left out"`
		Page     string `json:"page,omitempty" jsonschema:"The next of the call before, for the bookings after it"`
	}
	calcomFound struct {
		Bookings []map[string]any `json:"bookings"`
		Next     string           `json:"next" jsonschema:"Pass as page for more; empty when there is no more"`
	}
	calcomCancel struct {
		UID    string `json:"uid" jsonschema:"The booking's uid, from bookings"`
		Reason string `json:"reason,omitempty" jsonschema:"Why, which Cal.com tells the attendees"`
		All    bool   `json:"all,omitempty" jsonschema:"For a recurring booking, cancel its later bookings too"`
	}
)
