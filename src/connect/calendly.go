package connect

import (
	"time"

	"golang.org/x/oauth2"
)

// Calendly, through its API v2: the meetings people book with a member, read, and cancelled only when asked. Its
// OAuth app, made at developer.calendly.com, lists <API>/connect/calendly/callback as its redirect URI and asks for
// users:read, event_types:read, scheduled_events:read and scheduled_events:write, the last only to cancel. Calendly
// wants PKCE, and a refresh token works once: each refresh names the one to keep. Each member connects their own
// Calendly user.
//
// It syncs every meeting booked with the user, past and future, into calendar_event, its invitees and guests into
// calendar_attendee, and what they wrote when they booked into its description. Calendly lists them by when they
// start, not by when they changed, so each sync lists them all, and reads the invitees of only those that changed.
// Its webhooks need a paid plan, so it pushes nothing: connecting, asking and reading start a sync.
func init() {
	Register(&Connector{
		ID: "calendly", Name: "Calendly", Env: "ARCHIVE_CALENDLY",
		Info: Info{By: "Calendly", Description: "Read the meetings people book with you and who booked them, and cancel one when you ask", Category: CatCalendar, Icon: IconCalendar,
			Scopes: []string{"Read your booked meetings, invitees and event types", "Cancel a meeting when you ask"}},
		API: "https://api.calendly.com",
		Endpoint: oauth2.Endpoint{AuthURL: "https://auth.calendly.com/oauth/authorize", TokenURL: "https://auth.calendly.com/oauth/token",
			AuthStyle: oauth2.AuthStyleInParams},
		RevokeBy: &Req{Method: POST, URL: "https://auth.calendly.com/oauth/revoke",
			Form: Q{"client_id": "{client/id}", "client_secret": "{client/secret}", "token": "{refresh}"}},
		Scopes: calendlyScopes,
		Who:    &Who{Req: calendlyMe.Req, Names: []Path{"resource/email"}, Lower: true},
		Actions: map[string]*Action{
			"event_types": Call[struct{}, []map[string]any](Plan{
				Doc: "Your event types, the kinds of meeting people can book with you: [{ uri, name, slug, active, duration, kind, " +
					"scheduling_url, description_plain, locations }]",
				Steps: []Step{
					{As: "me", Req: calendlyMe.Req},
					{As: "got", Req: Req{URL: "/event_types", Query: Q{"user": "{me/resource/uri}", "count": "100"}}},
				},
				Out: "{got/collection|[]}",
			}),
			"events": Call[calendlyFind, calendlyFound](Plan{
				Doc: "Meetings booked with you, soonest first, maybe only between two times, of a status or with an invitee: { events: [{ uri, name, " +
					"status, start_time, end_time, event_type, location, invitees_counter, event_memberships, event_guests, cancellation }], next } " +
					"(pass next as page for more)",
				Clamp: map[string][2]int{"limit": {50, 100}},
				Enum:  map[string][]string{"status": {"active", "canceled"}},
				Steps: []Step{
					{As: "me", Req: calendlyMe.Req},
					{As: "got", Req: Req{URL: "/scheduled_events", Query: Q{"user": "{me/resource/uri}", "sort": "start_time:asc", "count": "{limit}",
						"min_start_time": "{from}", "max_start_time": "{to}", "status": "{status}", "invitee_email": "{invitee}", "page_token": "{page}"}}},
				},
				Out: Obj{"events": "{got/collection|[]}", "next": "{got/pagination/next_page_token|}"},
			}),
			"invitees": Call[calendlyOne, []map[string]any](Plan{
				Doc: "Who booked a meeting, with what they answered when they did: [{ uri, name, email, status, timezone, questions_and_answers, " +
					"no_show, rescheduled, cancellation, created_at }]",
				Steps: []Step{{As: "got", Req: Req{URL: "/scheduled_events/{id}/invitees", Query: Q{"count": "100"}}}},
				Out:   "{got/collection|[]}",
			}),
			"cancel": Call[calendlyCancel, calendlyCancelled](Plan{
				Doc:   "Cancel a booked meeting, which Calendly emails its invitees about with the reason. Only when the person asks to: { id, canceled_by, reason }",
				Steps: []Step{{As: "got", Req: Req{Method: POST, URL: "/scheduled_events/{id}/cancellation", Body: Obj{"reason": "{reason}"}}}},
				Out:   Obj{"id": "{id}", "canceled_by": "{got/resource/canceled_by|}", "reason": "{got/resource/reason|}"},
			}),
		},
		Sync: []*Sync{{Name: "events", Gap: time.Second, Parts: &calendlyMe,
			Req:   Req{URL: "/scheduled_events", Query: Q{"user": "{part}", "sort": "start_time:asc", "count": "100", "page_token": "{page}"}},
			Items: "collection", Key: []Reads{{Of: "{uri}", Find: `/([^/]+)$`}}, Updated: "updated_at", Page: Cursor{At: "pagination/next_page_token"},
			Memo:    "{updated_at} {invitees_counter/total} {invitees_counter/active}",
			Hydrate: []Hydrate{Fetch{Req: Req{URL: "{+uri}/invitees", Query: Q{"count": "100"}}, As: "invitees", Items: "collection", Par: 4, Gone: true}},
			Doc: "Every meeting booked with you, past and future, cancelled ones too, by its id, with its invitees and guests, and what the " +
				"invitees wrote when they booked, cut at 64 KB. Listed whole every sync, 100 a page, which deletes those gone; the invitees " +
				"of a meeting are read again only when it changed.",
			To: Map{Table: "calendar_event",
				Cols: map[string]SQL{"calendar": "'Calendly'", "title": "data->>'name'", "location": "data#>>'{location,location}'",
					"description": `archive.sync_clip(nullif(concat_ws(E'\n', nullif(data->>'meeting_notes_plain', ''), (SELECT string_agg(concat(i->>'name', ', ', q->>'question', ': ', q->>'answer'), E'\n')
						FROM archive.sync_list(data->'invitees') i, archive.sync_list(i->'questions_and_answers') q WHERE nullif(q->>'answer', '') IS NOT NULL)), ''), 65536)`,
					"starts_at": "archive.sync_at(data->>'start_time')", "ends_at": "archive.sync_at(data->>'end_time')", "all_day": "false",
					"status":         "CASE data->>'status' WHEN 'canceled' THEN 'cancelled' WHEN 'active' THEN 'confirmed' END",
					"organizer_name": "data#>>'{event_memberships,0,user_name}'", "meeting_url": "data#>>'{location,join_url}'",
					"edited_at": "archive.sync_at(data->>'updated_at')"},
				People: map[string]SQL{"organizer": "data#>>'{event_memberships,0,user_email}'"},
				Children: []Kids{
					{Table: "calendar_attendee", From: "archive.sync_list(data->'invitees') a WHERE archive.sync_email(a->>'email') IS NOT NULL",
						Cols: map[string]SQL{"name": "a->>'name'", "role": "'required'",
							"response": "CASE a->>'status' WHEN 'active' THEN 'accepted' WHEN 'canceled' THEN 'declined' ELSE 'none' END"},
						People: map[string]SQL{"person": "a->>'email'"}},
					{Table: "calendar_attendee", From: "archive.sync_list(data->'event_guests') a WHERE archive.sync_email(a->>'email') IS NOT NULL",
						Cols:   map[string]SQL{"name": "NULL", "role": "'optional'", "response": "'none'"},
						People: map[string]SQL{"person": "a->>'email'"}}}}}},
	})
}

var (
	calendlyScopes = []string{"users:read", "event_types:read", "scheduled_events:read", "scheduled_events:write"}
	// calendlyMe is the connected user, whose meetings every list is of, by their URI.
	calendlyMe = Parts{Req: Req{URL: "/users/me"}, Items: "resource", ID: "{uri}"}
)

type (
	calendlyFind struct {
		From    string `json:"from,omitempty" jsonschema:"Only those starting at or after this time, RFC 3339 like 2026-11-03T00:00:00Z"`
		To      string `json:"to,omitempty" jsonschema:"Only those starting before this time, RFC 3339"`
		Status  string `json:"status,omitempty" jsonschema:"active or canceled; both when left out"`
		Invitee string `json:"invitee,omitempty" jsonschema:"Only those an invitee of this email address booked"`
		Limit   int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 100; 50 when left out"`
		Page    string `json:"page,omitempty" jsonschema:"The next of the call before, for the meetings after it"`
	}
	calendlyFound struct {
		Events []map[string]any `json:"events"`
		Next   string           `json:"next" jsonschema:"Pass as page for more; empty when there is no more"`
	}
	calendlyOne struct {
		ID string `json:"id" jsonschema:"The meeting's id, the last part of its uri"`
	}
	calendlyCancel struct {
		ID     string `json:"id" jsonschema:"The meeting's id, the last part of its uri"`
		Reason string `json:"reason,omitempty" jsonschema:"Why, which Calendly tells the invitees"`
	}
	calendlyCancelled struct {
		ID         string `json:"id"`
		CanceledBy string `json:"canceled_by"`
		Reason     string `json:"reason"`
	}
)
