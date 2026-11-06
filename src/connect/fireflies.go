package connect

import "time"

// Fireflies.ai, the notetaker, through its GraphQL API, read only. It has no sign-in for apps: a person pastes the
// API key of their own account, from Integrations, Fireflies API, which reads the meetings they recorded or were
// in. Every plan has the API, Free at 500 requests a day. A GraphQL document with anything but queries in it is
// refused before it goes.
//
// It syncs every meeting Fireflies recorded into calendar_event, with its summary, action items and transcript, and
// who took part into calendar_attendee. Fireflies lists meetings by when they were held, and adds a summary some
// minutes after, so each sync lists them all, 50 a page, and reads the transcript of only those that are new or
// changed. Its webhooks are set up per account by hand, so it pushes nothing: connecting, asking and reading start
// a sync.
func init() {
	Register(&Connector{
		ID: "fireflies", Name: "Fireflies.ai",
		Info: Info{By: "Fireflies.ai", Description: "Read your recorded meetings, their summaries, action items and transcripts", Category: CatCalendar, Icon: IconNotebook,
			Scopes: []string{"Read your meetings and their transcripts with your API key"}},
		Keys: []Key{{Name: "key", Label: "API key", Secret: true, Help: "In Fireflies, Integrations, Fireflies API: your API key"}},
		Sign: map[string]string{"Authorization": "Bearer {key/key}"},
		API:  "https://api.fireflies.ai",
		// Fireflies answers errors with a 200, and running out of the plan's requests is one the sync tries again later.
		Errors: &Errors{At: "errors/0/message", Code: "errors/0/extensions/code;errors/0/code", Busy: []string{"too_many_requests"}},
		Who:    &Who{Req: Req{Method: POST, Read: true, URL: "/graphql", Body: Obj{"query": "{ user { email name } }"}}, Names: []Path{"data/user/email"}, Lower: true},
		Actions: map[string]*Action{
			"search": Call[firefliesFind, []map[string]any](Plan{
				Doc: "Your recorded meetings, newest first, maybe only between two days or with some words in their title or what was said: " +
					"[{ id, title, dateString, duration, organizer_email, participants, transcript_url, summary: { overview, action_items, keywords } }] " +
					"(pass skip for more)",
				Clamp: map[string][2]int{"limit": {20, 50}},
				Steps: []Step{{As: "got", Req: Req{Method: POST, Read: true, URL: "/graphql", Body: Obj{"query": "query($limit: Int, $skip: Int, $from: DateTime, $to: DateTime, $keyword: String) " +
					"{ transcripts(limit: $limit, skip: $skip, fromDate: $from, toDate: $to, keyword: $keyword, scope: all) { id title dateString duration organizer_email " +
					"participants transcript_url summary { overview action_items keywords } } }",
					"variables": Obj{"limit": "{limit}", "skip": "{skip}", "from": "{from}", "to": "{to}", "keyword": "{query}"}}}}},
				Out: "{got/data/transcripts|[]}",
			}),
			"transcript": Call[firefliesOne, map[string]any](Plan{
				Doc: "A recorded meeting whole: { id, title, dateString, duration, organizer_email, meeting_attendees, transcript_url, " +
					"summary: { overview, action_items, outline, keywords }, sentences: [{ speaker_name, text, start_time }] }",
				Steps: []Step{{As: "got", Req: Req{Method: POST, Read: true, URL: "/graphql", Body: Obj{"query": "query($id: String!) { transcript(id: $id) { id title dateString duration " +
					"organizer_email meeting_attendees { displayName email } transcript_url meeting_link summary { overview action_items outline keywords } " +
					"sentences { speaker_name text start_time } } }", "variables": Obj{"id": "{id}"}}}}},
				Out: "{got/data/transcript}",
			}),
			"graphql": Call[firefliesQuery, any](Plan{
				Doc: "Run a query of Fireflies' GraphQL API, like { query: \"{ user { name num_transcripts } }\" }: its data. Queries only; a mutation is refused",
				Steps: []Step{
					{Codec: GraphQLReads, In: "{query}"},
					{As: "got", Req: Req{Method: POST, Read: true, URL: "/graphql", Body: Obj{"query": "{query}", "variables": "{variables}"}}},
				},
				Out: "{got/data}",
			}),
		},
		Sync: []*Sync{{Name: "meetings", Gap: time.Second,
			// Paging goes in the document, as Fireflies wants a number and a page is text.
			Req: Req{Method: POST, Read: true, URL: "/graphql", Body: Obj{"query": "{ transcripts(limit: 50, skip: {page|\"0\"}) { id title date duration organizer_email " +
				"participants transcript_url meeting_link meeting_attendees { displayName email } summary { overview bullet_gist action_items } } }"}},
			Items: "data/transcripts", Page: Offset{Size: 50, Step: 50},
			Memo: "{title} {date} {duration} {summary/overview} {summary/action_items}",
			Hydrate: []Hydrate{Fetch{Req: Req{Method: POST, Read: true, URL: "/graphql", Body: Obj{"query": "query($id: String!) { transcript(id: $id) { sentences { speaker_name text } } }",
				"variables": Obj{"id": "{id}"}}}, As: "said", Items: "data/transcript/sentences", Par: 2}},
			Doc: "Every meeting Fireflies recorded that you were in, by its id, with its summary, action items and who took part, and what was " +
				"said, a line a sentence, cut at 512 KB. Listed whole every sync, 50 a page, which deletes those gone; the transcript of a " +
				"meeting is read again only when it changed.",
			To: Map{Table: "calendar_event",
				Cols: map[string]SQL{"calendar": "'Fireflies'", "title": "data->>'title'", "starts_at": "archive.sync_at(data->>'date')",
					"ends_at": "archive.sync_at(data->>'date') + make_interval(secs => coalesce(archive.sync_num(data->>'duration'), 0) * 60)",
					"all_day": "false", "url": "data->>'transcript_url'", "meeting_url": "data->>'meeting_link'",
					"summary":      "archive.sync_clip(nullif(concat_ws(E'\\n\\n', nullif(data#>>'{summary,overview}', ''), nullif(data#>>'{summary,bullet_gist}', '')), ''), 65536)",
					"action_items": "archive.sync_clip(nullif(btrim(data#>>'{summary,action_items}', E' \\t\\r\\n'), ''), 65536)",
					"transcript": `archive.sync_clip((SELECT string_agg(concat(coalesce(nullif(s->>'speaker_name', ''), 'Someone'), ': ', s->>'text'), E'\n' ORDER BY n)
						FROM archive.sync_list(data->'said') WITH ORDINALITY AS t(s, n)), 524288)`},
				People: map[string]SQL{"organizer": "data->>'organizer_email'"},
				Children: []Kids{{Table: "calendar_attendee",
					From: `(SELECT DISTINCT ON (archive.sync_email(e)) e, n FROM (SELECT a->>'email' e, nullif(coalesce(a->>'displayName', a->>'name'), '') n FROM archive.sync_list(data->'meeting_attendees') a
						UNION ALL SELECT p#>>'{}', NULL FROM archive.sync_list(data->'participants') p) x WHERE archive.sync_email(e) IS NOT NULL ORDER BY archive.sync_email(e), n NULLS LAST) a`,
					Cols:   map[string]SQL{"name": "a.n", "role": "'required'", "response": "'none'"},
					People: map[string]SQL{"person": "a.e"}}}}}},
	})
}

type (
	firefliesFind struct {
		Query string `json:"query,omitempty" jsonschema:"Words in a meeting's title or in what was said"`
		From  string `json:"from,omitempty" jsonschema:"Only those held at or after this time, RFC 3339 like 2026-11-03T00:00:00Z"`
		To    string `json:"to,omitempty" jsonschema:"Only those held before this time, RFC 3339"`
		Limit int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 50; 20 when left out"`
		Skip  int    `json:"skip,omitempty" jsonschema:"How many to pass over, for the meetings after a search before"`
	}
	firefliesOne struct {
		ID string `json:"id" jsonschema:"The meeting's id, from search"`
	}
	firefliesQuery struct {
		Query     string         `json:"query" jsonschema:"The GraphQL query"`
		Variables map[string]any `json:"variables,omitempty" jsonschema:"Its variables"`
	}
)
