package connect

import (
	"time"

	"golang.org/x/oauth2"
)

// Zoom: the meetings, cloud recordings and AI Companion summaries of the person who connects, through Zoom's REST
// API. Its app is a General app of Zoom's App Marketplace, managed by users, whose OAuth redirect URL is
// <API>/connect/zoom/callback and whose granular scopes are exactly the Scopes here; Zoom replaces the refresh token
// on every refresh. Recordings and summaries need a paid plan, and summaries AI Companion's meeting summary turned
// on; an account without them syncs its meetings alone.
//
// It syncs the meetings Zoom lists as scheduled, those still to come and recent ones not yet expired, into
// calendar_event, listed whole each sync. Each cloud recording is a file in drive_file with the words of its
// transcript, and each summary another with its text, read a month at a time, as Zoom lists recordings at most a
// month at a time, from two years back to this month, every month whole each sync. A recording or summary that has
// not changed is not read again. Zoom's webhooks open with a handshake Archive does not answer, so it pushes
// nothing: connecting, asking and reading start a sync.
func init() {
	Register(&Connector{
		ID: "zoom", Name: "Zoom",
		Info: Info{By: "Zoom", Description: "Read your meetings, recordings’ transcripts and AI summaries, and schedule or change meetings when you ask", Category: CatCalendar, Icon: IconCalendar,
			Scopes: []string{"Read your meetings, cloud recordings, transcripts and meeting summaries", "Schedule and change meetings when you ask"}},
		API: "https://api.zoom.us/v2",
		Endpoint: oauth2.Endpoint{AuthURL: "https://zoom.us/oauth/authorize", TokenURL: "https://zoom.us/oauth/token",
			AuthStyle: oauth2.AuthStyleInHeader},
		RevokeBy: &Req{Method: POST, URL: "https://zoom.us/oauth/revoke?token={refresh}", Header: map[string][]string{"Authorization": {"{basic}"}}},
		Scopes: []string{"user:read:user", "meeting:read:list_meetings", "meeting:read:meeting", "meeting:write:meeting", "meeting:update:meeting",
			"cloud_recording:read:list_user_recordings", "cloud_recording:read:list_recording_files", "meeting:read:list_summaries", "meeting:read:summary"},
		Who: &Who{Req: Req{URL: "/users/me"}, Names: []Path{"email"}, Lower: true},
		Actions: map[string]*Action{
			"meetings": Call[zoomFind, zoomFound](Plan{
				Doc: "Your meetings: upcoming ones by default, or scheduled, live or previous_meetings: { meetings: [{ id, uuid, topic, type, start_time, " +
					"duration, timezone, agenda, join_url }], next } (pass next as page for more)",
				Clamp: map[string][2]int{"limit": {30, 300}},
				Enum:  map[string][]string{"type": {"upcoming", "scheduled", "live", "previous_meetings"}},
				Steps: []Step{{As: "got", Req: Req{URL: "/users/me/meetings", Query: Q{"type": "{type|upcoming}", "page_size": "{limit}", "next_page_token": "{page}"}}}},
				Out:   Obj{"meetings": "{got/meetings|[]}", "next": "{got/next_page_token|}"},
			}),
			"meeting": Call[zoomOne, map[string]any](Plan{
				Doc:   "One meeting by id: { id, uuid, topic, type, status, start_time, duration, timezone, agenda, join_url, recurrence, occurrences, settings }",
				Steps: []Step{{Req: Req{URL: "/meetings/{id}"}}},
			}),
			"recordings": Call[zoomSpan, zoomRecordings](Plan{
				Doc: "Your cloud recordings between two days at most a month apart, the last day by default: { meetings: [{ uuid, id, topic, start_time, " +
					"duration, share_url, recording_files: [{ id, file_type, file_extension, file_size, recording_type, play_url }] }], next } (pass next as page for more)",
				Steps: []Step{{As: "got", Req: Req{URL: "/users/me/recordings", Query: Q{"from": "{from}", "to": "{to}", "page_size": "300", "next_page_token": "{page}"}}}},
				Out:   Obj{"meetings": "{got/meetings|[]}", "next": "{got/next_page_token|}"},
			}),
			"transcript": Call[zoomOne, zoomTranscript](Plan{
				Doc: "The transcript of a meeting's cloud recording, by its uuid from recordings, or its id for the last time it met, as WebVTT text " +
					"with who spoke when: { topic, start_time, transcript }, empty when it has none",
				Steps: []Step{
					{As: "rec", Req: Req{URL: "/meetings/{id}/recordings"}},
					{As: "words", Each: "rec/recording_files", Only: Test{At: "file_type", Is: "TRANSCRIPT"},
						Words: &Download{Req: Req{URL: "{+download_url}"}, Name: "transcript.vtt", Mime: "text/vtt", Size: "file_size"}},
				},
				Out: Obj{"topic": "{rec/topic|}", "start_time": "{rec/start_time|}", "transcript": "{words/0|}"},
			}),
			"summary": Call[zoomOne, map[string]any](Plan{
				Doc: "AI Companion's summary of a meeting, by its uuid, or its id for the last time it met: { meeting_topic, meeting_start_time, " +
					"summary_title, summary_content, in Markdown, summary_doc_url }",
				Steps: []Step{{Req: Req{URL: "/meetings/{id}/meeting_summary"}}},
			}),
			"create": Call[zoomNew, map[string]any](Plan{
				Doc: "Schedule a meeting as its host, nobody invited: the meeting, with its join_url to share. Only when asked to",
				Steps: []Step{
					{Unless: "topic", Fail: zoomNeeds}, {Unless: "start_time", Fail: zoomNeeds},
					{Req: Req{Method: POST, URL: "/users/me/meetings", Body: zoomMeeting}},
				},
			}),
			"update": Call[zoomEdit, map[string]any](Plan{
				Doc: "Change a meeting you host, a whole series by its id: only what you give changes, and a start must be in the future. Returns the " +
					"meeting, as meeting does. Only when asked to",
				Steps: []Step{
					{Req: Req{Method: PATCH, URL: "/meetings/{id}", Body: Obj{"topic": "{topic}", "start_time": "{start_time}", "duration": "{duration}",
						"timezone": "{timezone}", "agenda": "{agenda}"}}},
					{As: "m", Req: Req{URL: "/meetings/{id}"}},
				},
				Out: "{m}",
			}),
		},
		Sync: []*Sync{
			{Name: "meetings", Gap: time.Second, Req: Req{URL: "/users/me/meetings", Query: Q{"type": "scheduled", "page_size": "300", "next_page_token": "{page}"}},
				Items: "meetings", Absent: "page_size", Page: zoomPage,
				Doc: "The meetings you host that Zoom lists as scheduled, upcoming, live and recent ones not yet expired, a recurring one as one, " +
					"by its id: listed whole each sync, 300 a page, so one Zoom no longer lists is deleted.",
				To: Map{Table: "calendar_event",
					Cols: map[string]SQL{"calendar": "'Zoom'", "title": "data->>'topic'", "description": "archive.sync_clip(data->>'agenda', 65536)",
						"starts_at": "archive.sync_at(data->>'start_time')", "all_day": "false",
						"ends_at":     "archive.sync_at(data->>'start_time') + make_interval(mins => archive.sync_num(data->>'duration')::int)",
						"meeting_url": "data->>'join_url'"}}},
			{Name: "recordings", Gap: time.Second, Parts: zoomMonths, Items: "meetings", Absent: "page_size", ID: "uuid", Page: zoomPage, Memo: "{recording_count}/{total_size}",
				Req:  Req{URL: "/users/me/recordings", Query: Q{"from": "{in/start}", "to": "{in/end}", "page_size": "300", "next_page_token": "{page}"}},
				Deny: zoomPaid,
				Hydrate: []Hydrate{Download{Each: "recording_files", If: Test{At: "file_type", Is: "TRANSCRIPT"}, Req: Req{URL: "{+download_url}"},
					Name: "transcript.vtt", Mime: "text/vtt", Size: "file_size", As: "text", Par: 4}},
				Doc: "Each meeting's cloud recording, by the uuid of the time it met: its topic, size, link, and the words of its transcript, " +
					"with who spoke when, cut at 64 KB; the video stays in Zoom. Two years of months, a month at a time and every month whole " +
					"each sync, 300 a page, so one deleted there is deleted; a recording that has not changed is not read again.",
				To: Map{Table: "drive_file",
					Cols: map[string]SQL{"kind": "'file'", "title": "data->>'topic'", "drive": "'Zoom recordings'", "size": "archive.sync_num(data->>'total_size')",
						"url": "data->>'share_url'", "made_at": "archive.sync_at(data->>'start_time')",
						"text": `archive.sync_clip((SELECT f->>'text' FROM archive.sync_list(data->'recording_files') f WHERE f->>'file_type' = 'TRANSCRIPT' AND nullif(f->>'text', '') IS NOT NULL LIMIT 1), 65536)`}}},
			{Name: "summaries", Gap: time.Second, Parts: zoomMonths, Items: "summaries", Absent: "page_size", ID: "meeting_uuid", Updated: "summary_last_modified_time",
				Page: zoomPage, Memo: "{summary_last_modified_time}", Deny: zoomPaid,
				Req: Req{URL: "/users/me/meeting_summaries", Query: Q{"from": "{in/start}T00:00:00Z", "to": "{in/end}T23:59:59Z", "page_size": "300", "next_page_token": "{page}"}},
				// A uuid with a / Zoom wants escaped twice, which a template does not do: its summary is left out.
				Hydrate: []Hydrate{Fetch{Req: Req{URL: "/meetings/{meeting_uuid}/meeting_summary"}, As: "summary", Par: 4, Soft: true}},
				Doc: "AI Companion's summary of each meeting you host, by the uuid of the time it met: its title, its text in Markdown, cut at " +
					"64 KB, and who last edited it. Read as recordings are, by month, and one whose summary changed is read again.",
				To: Map{Table: "drive_file",
					Cols: map[string]SQL{"kind": "'file'", "title": "coalesce(nullif(data#>>'{summary,summary_title}', ''), data->>'meeting_topic')", "drive": "'Zoom summaries'",
						"mime": "'text/markdown'", "url": "data#>>'{summary,summary_doc_url}'", "made_at": "archive.sync_at(data->>'summary_created_time')",
						"edited_at": "archive.sync_at(data->>'summary_last_modified_time')", "owner_name": "data->>'meeting_host_email'",
						"text": "archive.sync_clip(coalesce(nullif(data#>>'{summary,summary_content}', ''), data#>>'{summary,summary_overview}'), 65536)"},
					People: map[string]SQL{"owner": "data->>'meeting_host_email'", "editor": "data#>>'{summary,summary_last_modified_user_email}'"}}},
		},
	})
}

const zoomNeeds = "a meeting needs a topic and a start_time"

var (
	zoomPage = Cursor{At: "next_page_token"}
	// Each month from two years back to this one, and the first day of the next, as {start} and {end}.
	zoomMonths = &Parts{Span: &Span{Month: true, Last: Shift{0, 1, 0}, First: Shift{-2, -1, 0}, Step: Shift{0, 1, 0}}, ID: "{start}", Vals: Obj{"start": "{start}", "end": "{end}"}}
	// A free account, or one whose admin keeps summaries to email, has no recordings or summaries to read.
	zoomPaid    = []Deny{{Status: []int{400, 403}, Says: []string{"paid account", "not enabled", "restricted", "no permission"}, Why: "Zoom has no cloud recordings or summaries for this account: {message}"}}
	zoomMeeting = Obj{"topic": "{topic}", "type": 2, "start_time": "{start_time}", "duration": "{duration}", "timezone": "{timezone}", "agenda": "{agenda}"}
)

type (
	zoomFind struct {
		Type  string `json:"type,omitempty" jsonschema:"upcoming, scheduled, live or previous_meetings; upcoming when left out"`
		Limit int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 300; 30 when left out"`
		Page  string `json:"page,omitempty" jsonschema:"The next of the call before, for the meetings after it"`
	}
	zoomFound struct {
		Meetings []map[string]any `json:"meetings"`
		Next     string           `json:"next" jsonschema:"Pass as page for more; empty when there is no more"`
	}
	zoomOne struct {
		ID string `json:"id" jsonschema:"The meeting's id, like 85746065432, or the uuid of one time it met, from recordings"`
	}
	zoomSpan struct {
		From string `json:"from,omitempty" jsonschema:"The first day, YYYY-MM-DD in UTC; today when left out"`
		To   string `json:"to,omitempty" jsonschema:"The last day, at most a month after from; the day after from when left out"`
		Page string `json:"page,omitempty" jsonschema:"The next of the call before, for the recordings after it"`
	}
	zoomRecordings struct {
		Meetings []map[string]any `json:"meetings"`
		Next     string           `json:"next" jsonschema:"Pass as page for more; empty when there is no more"`
	}
	zoomTranscript struct {
		Topic      string `json:"topic"`
		StartTime  string `json:"start_time"`
		Transcript string `json:"transcript" jsonschema:"WebVTT: each line of speech with its time and who spoke, cut at 64 KB"`
	}
	// zoomNew is a meeting to schedule, or what to change of one.
	zoomNew struct {
		Topic     string `json:"topic,omitempty" jsonschema:"The meeting's title"`
		StartTime string `json:"start_time,omitempty" jsonschema:"When it starts, like 2026-11-05T15:00:00Z, or local to timezone without the Z"`
		Duration  int    `json:"duration,omitempty" jsonschema:"How long it is, in minutes"`
		Timezone  string `json:"timezone,omitempty" jsonschema:"An IANA time zone, like America/Los_Angeles; your Zoom profile's when left out"`
		Agenda    string `json:"agenda,omitempty" jsonschema:"What it is about, up to 2,000 characters"`
	}
	zoomEdit struct {
		ID string `json:"id" jsonschema:"The meeting's id, from meetings"`
		zoomNew
	}
)
