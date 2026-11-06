package connect

import (
	"strings"
	"time"
)

// Otter.ai, through its hosted MCP server: the meetings Otter recorded that the person can see. Its authorization
// server registers any client (RFC 7591) and asks for profile:read and conversations:read, so any plan signs in with
// no app keys. Otter's REST API is for Enterprise workspaces only, with keys an admin makes, so this reads through
// the server instead.
//
// Otter documents its tools as search, which finds meetings with their overviews, and fetch, which reads one whole:
// its transcript, summary, action items and attendees. Their schemas are listed only to a signed-in person, so the
// sync passes arguments under the names they may take, which toolReq keeps to those the tool's schema has, and reads
// fields under the names they may have, as Monarch's does.
//
// It syncs every meeting into calendar_event, with its summary, action items and transcript, and who took part,
// by email, into calendar_attendee. It searches three months at a time from five years back the first time, and
// after that the last 30 days each sync; a meeting is fetched again only when what search says of it changed.
func init() {
	Register(&Connector{
		ID: "otter", Name: "Otter.ai", MCP: "https://mcp.otter.ai/mcp", Scopes: []string{"profile:read", "conversations:read"}, Pace: 500 * time.Millisecond, Busy: ToolBusy,
		Info: Info{By: "Otter.ai", Description: "Read your recorded meetings, their summaries, action items and transcripts", Category: CatCalendar, Icon: IconNotebook,
			Scopes: []string{"Read your profile and the meetings shared with you"}},
		Who: &Who{Tool: "get_user_info,get_user,get_me,whoami"},
		Sync: []*Sync{{Name: "meetings", Gap: time.Second, Parts: otterQuarters, Keep: true, Items: "**", Key: []Reads{{Of: "{id;conversation_id;otid}"}}, Page: otterPage,
			Req: Req{Tool: "search", Body: Obj{"query": "", "start_date": "{in/start}", "end_date": "{in/end}", "startDate": "{in/start}", "endDate": "{in/end}",
				"from": "{in/start}", "to": "{in/end}", "after": "{in/start}", "before": "{in/after}", "date_from": "{in/start}", "date_to": "{in/end}"}},
			Memo:    "{title|} {updated_at;modified_at;updated|} {text;overview;summary|}",
			Hydrate: []Hydrate{Fetch{Req: Req{Tool: "fetch", Text: true, Body: Obj{"id": "{id;conversation_id;otid}"}}, As: "full", Par: 2}},
			Doc: "Every meeting Otter recorded that you can see, by its id, with its summary, action items, who took part and what was said, " +
				"cut at 512 KB: searched three months at a time from five years back the first time, and after that the last 30 days each sync. " +
				"A search deletes nothing, so one deleted in Otter stays here; a meeting is fetched again only when what search says of it changed.",
			To: Map{Table: "calendar_event",
				Cols: map[string]SQL{"calendar": "'Otter.ai'", "title": otterText("title", "name"), "all_day": "false",
					"starts_at": "archive.sync_at(" + otterText("start_time", "metadata,start_time", "startTime", "created_at", "metadata,created_at", "date") + ")",
					"ends_at":   "archive.sync_at(" + otterText("end_time", "metadata,end_time", "endTime") + ")",
					"url":       otterText("url", "metadata,url", "link"),
					"summary": "archive.sync_clip(coalesce(" + otterLines("summary", "metadata,summary", "overview", "metadata,overview") +
						", nullif(btrim(CASE jsonb_typeof(data->'text') WHEN 'string' THEN data->>'text' END), '')), 65536)",
					"action_items": "archive.sync_clip(" + otterLines("action_items", "metadata,action_items", "actionItems") + ", 65536)",
					"transcript": "archive.sync_clip(CASE jsonb_typeof(data->'full') WHEN 'string' THEN data->>'full' ELSE " +
						otterSaid("full,transcript", "full,metadata,transcript", "full,text") + " END, 524288)"},
				Children: []Kids{{Table: "calendar_attendee",
					From: `(SELECT DISTINCT ON (archive.sync_email(e)) e, n FROM (SELECT coalesce(a->>'email', a#>>'{}') e, nullif(coalesce(a->>'name', a->>'display_name'), '') n
						FROM archive.sync_list(coalesce(data#>'{full,attendees}', data#>'{full,metadata,attendees}', data#>'{full,participants}', data#>'{full,metadata,participants}', data->'attendees')) a) x
						WHERE archive.sync_email(e) IS NOT NULL ORDER BY archive.sync_email(e), n NULLS LAST) a`,
					Cols:   map[string]SQL{"name": "a.n", "role": "'required'", "response": "'none'"},
					People: map[string]SQL{"person": "a.e"}}}}}},
	})
}

var (
	otterPage     = ToolPage{Limit: "limit,page_size,max_results,count", Max: 50, After: "cursor,after_cursor,next_cursor,page_token", Page: "page", Offset: "offset,skip"}
	otterQuarters = &Parts{Span: &Span{First: Shift{-5, 0, 0}, Step: Shift{0, 3, 0}, Then: Shift{0, 0, -30}}, ID: "{start}", Vals: Obj{"start": "{start}", "end": "{end}", "after": "{after}"}}
)

// otterAt is a field of what fetch read, under full, or else of what search found, each a comma path like metadata,start_time.
func otterAt(f string) []string {
	return []string{"data#>'{full," + f + "}'", "data#>'{" + f + "}'"}
}

// otterText is the first of a meeting's fields that has a value as text.
func otterText(fields ...string) SQL {
	alts := []string{}
	for _, f := range fields {
		for _, at := range otterAt(f) {
			alts = append(alts, "CASE jsonb_typeof("+at+") WHEN 'string' THEN nullif("+at+"#>>'{}', '') WHEN 'number' THEN "+at+"#>>'{}' END")
		}
	}
	return SQL("coalesce(" + strings.Join(alts, ", ") + ")")
}

// otterLines is the first of a meeting's fields that has a value, a list a line an entry, its text or title when an entry is an object.
func otterLines(fields ...string) SQL {
	alts := []string{}
	for _, f := range fields {
		for _, at := range otterAt(f) {
			alts = append(alts, "CASE jsonb_typeof("+at+") WHEN 'string' THEN nullif(btrim("+at+"#>>'{}', E' \\t\\r\\n'), '') WHEN 'array' THEN "+
				"(SELECT string_agg(CASE jsonb_typeof(e) WHEN 'object' THEN coalesce(e->>'text', e->>'title', e->>'content', e::text) ELSE e#>>'{}' END, E'\\n') FROM jsonb_array_elements("+at+") e) END")
		}
	}
	return SQL("coalesce(" + strings.Join(alts, ", ") + ")")
}

// otterSaid is a transcript, a line each, Speaker: words, from text or a list of what each speaker said.
func otterSaid(fields ...string) SQL {
	alts := []string{}
	for _, f := range fields {
		at := "data#>'{" + f + "}'"
		alts = append(alts, "CASE jsonb_typeof("+at+") WHEN 'string' THEN nullif("+at+"#>>'{}', '') WHEN 'array' THEN "+
			"(SELECT string_agg(CASE jsonb_typeof(s) WHEN 'object' THEN concat(coalesce(nullif(coalesce(s->>'speaker', s->>'speaker_name', s#>>'{speaker,name}', s->>'name'), ''), 'Someone'), ': ', "+
			"coalesce(s->>'text', s->>'transcript', s->>'content')) ELSE s#>>'{}' END, E'\\n' ORDER BY n) FROM jsonb_array_elements("+at+") WITH ORDINALITY AS t(s, n)) END")
	}
	return SQL("coalesce(" + strings.Join(alts, ", ") + ")")
}
