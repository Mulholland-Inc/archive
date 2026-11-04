package connect

import "golang.org/x/oauth2"

// Slack: the channels, conversations and files of a workspace, as the person who connects sees them, read when
// asked and never kept. It reads ARCHIVE_SLACK, the Slack app sign-in with Slack uses too: its OAuth settings list
// <API>/connect/slack/callback among its redirect URLs, the user token scopes here, and token rotation, so that a
// token ends after 12 hours and a refresh token replaces itself. It signs in for a user token alone, through
// Slack's v2_user flow, with PKCE; Slack takes the scopes comma separated, so they go as one.
//
// It syncs nothing. Slack's API terms bar an app that others install from bulk exporting message and file data,
// and its Marketplace asks an assistant to keep none of it, reading what it needs when asked; an app outside the
// Marketplace may read history once a minute, 15 messages at a time. So every action reads Slack live: search is
// the Real-time Search API, which only a Marketplace app or a workspace's own may call, and the others read a
// conversation, a thread, files or a person by id. Slack answers a refusal as a 200 with ok false and its error.
func init() {
	Register(&Connector{
		ID: "slack", Name: "Slack", Options: "which sign-in with Slack reads too",
		Info: Info{By: "Slack", Description: "Search and read your channels, messages and files as you see them, and post when you ask", Category: CatChat, Icon: IconChat,
			Scopes: []string{"Search and read the channels, conversations and files you can see", "Post messages and replies as you when you ask"}},
		API: "https://slack.com/api",
		Endpoint: oauth2.Endpoint{AuthURL: "https://slack.com/oauth/v2_user/authorize", TokenURL: "https://slack.com/api/oauth.v2.user.access",
			AuthStyle: oauth2.AuthStyleInHeader},
		RevokeBy: &Req{Method: POST, URL: "https://slack.com/api/auth.revoke", Form: Q{"token": "{access}"}},
		Scopes: []string{"channels:read,groups:read,im:read,mpim:read,channels:history,groups:history,im:history,mpim:history," +
			"search:read.public,search:read.private,search:read.im,search:read.mpim,search:read.files,files:read," +
			"users:read,users:read.email,users.profile:read,chat:write"},
		Errors: &Errors{At: "error", Code: "error", Busy: []string{"ratelimited"}},
		Who:    &Who{Req: Req{URL: "/users.profile.get"}, Names: []Path{"profile/email"}, Lower: true},
		Actions: map[string]*Action{
			"search": Call[slackFind, slackFound](Plan{
				Doc: "Find messages and files by words or by meaning, in every channel and conversation you can see, best match first, maybe only between " +
					"two times: { messages: [{ author_name, author_user_id, channel_id, channel_name, message_ts, content, permalink, context_messages }], " +
					"files: [{ file_id, title, file_type, author_name, date_created, permalink }], next } (pass next as page for more)",
				Clamp: map[string][2]int{"limit": {20, 20}},
				Enum:  map[string][]string{"sort": {"score", "timestamp"}},
				Steps: []Step{{As: "got", Req: Req{Method: POST, Read: true, URL: "/assistant.search.context",
					Body: Obj{"query": "{query}", "channel_types": slackTypes, "content_types": []string{"messages", "files"}, "include_context_messages": true,
						"after": "{after}", "before": "{before}", "sort": "{sort}", "limit": "{limit}", "cursor": "{page}"}}}},
				Out: Obj{"messages": "{got/results/messages|[]}", "files": "{got/results/files|[]}", "next": "{got/response_metadata/next_cursor|}"},
			}),
			"channels": Call[slackPage, slackChannels](Plan{
				Doc: "The channels, group conversations and direct messages you are in, not those archived: { channels: [{ id, name, is_channel, " +
					"is_private, is_im, is_mpim, user, topic, purpose }], next } (pass next as page for more); a direct message's user is who it is with",
				Steps: []Step{{As: "got", Req: Req{URL: "/users.conversations", Query: Q{"types": "public_channel,private_channel,mpim,im", "exclude_archived": "true",
					"limit": "200", "cursor": "{page}"}}}},
				Out: Obj{"channels": "{got/channels|[]}", "next": "{got/response_metadata/next_cursor|}"},
			}),
			"messages": Call[slackWhere, slackMessages](Plan{
				Doc: "The messages of a conversation, newest first, maybe only between two times, or with thread the replies to one, oldest first: " +
					"{ messages: [{ ts, user, text, thread_ts, reply_count, files, edited }], next } (pass next as page for more). Slack gives an app " +
					"outside its Marketplace 15 at a time, once a minute",
				Clamp: map[string][2]int{"limit": {15, 100}},
				Steps: []Step{
					{If: "thread", As: "got", Req: Req{URL: "/conversations.replies", Query: Q{"channel": "{channel}", "ts": "{thread}", "limit": "{limit}", "cursor": "{page}"}}},
					{Unless: "thread", As: "got", Req: Req{URL: "/conversations.history", Query: Q{"channel": "{channel}", "oldest": "{after}", "latest": "{before}",
						"limit": "{limit}", "cursor": "{page}"}}},
				},
				Out: Obj{"messages": "{got/messages|[]}", "next": "{got/response_metadata/next_cursor|}"},
			}),
			"files": Call[slackFiles, slackFilesFound](Plan{
				Doc: "The files shared where you can see them, newest first, maybe only in one conversation or by one person: { files: [{ id, name, " +
					"title, mimetype, size, user, created, channels, permalink }], page, pages }; their content stays in Slack",
				Clamp: map[string][2]int{"limit": {50, 100}, "page": {1, 1000}},
				Steps: []Step{{As: "got", Req: Req{URL: "/files.list", Query: Q{"channel": "{channel}", "user": "{user}", "ts_from": "{after}", "ts_to": "{before}",
					"count": "{limit}", "page": "{page}"}}}},
				Out: Obj{"files": "{got/files|[]}", "page": "{got/paging/page|1}", "pages": "{got/paging/pages|1}"},
			}),
			"person": Call[slackOne, map[string]any](Plan{
				Doc:   "Someone by their user id, as messages name them: { id, name, real_name, tz, is_bot, profile: { display_name, real_name, title, email, phone } }",
				Steps: []Step{{As: "got", Req: Req{URL: "/users.info", Query: Q{"user": "{id}"}}}},
				Out:   "{got/user}",
			}),
			"post": Call[slackNew, slackPosted](Plan{
				Doc: "Post a message as you to a conversation, by its id from channels, or a reply in a thread by its parent's ts: { channel, ts }. " +
					"Only when asked to",
				Steps: []Step{
					{Unless: "text", Fail: "a message needs its text"},
					{As: "sent", Req: Req{Method: POST, URL: "/chat.postMessage", Body: Obj{"channel": "{channel}", "text": "{text}", "thread_ts": "{thread}",
						"reply_broadcast": "{broadcast}"}}},
				},
				Out: Obj{"channel": "{sent/channel}", "ts": "{sent/ts}"},
			}),
		},
	})
}

var slackTypes = []string{"public_channel", "private_channel", "mpim", "im"}

type (
	slackFind struct {
		Query  string `json:"query" jsonschema:"What to find, words or a question; Slack's modifiers like in:#general or from:@ann work too"`
		After  int64  `json:"after,omitempty" jsonschema:"Only those after this time, in Unix seconds"`
		Before int64  `json:"before,omitempty" jsonschema:"Only those before this time, in Unix seconds"`
		Sort   string `json:"sort,omitempty" jsonschema:"score, the best match first, or timestamp, the newest first; score when left out"`
		Limit  int    `json:"limit,omitempty" jsonschema:"At most this many of each, 1 to 20; 20 when left out"`
		Page   string `json:"page,omitempty" jsonschema:"The next of the call before, for the results after it"`
	}
	slackFound struct {
		Messages []map[string]any `json:"messages"`
		Files    []map[string]any `json:"files"`
		Next     string           `json:"next" jsonschema:"Pass as page for more; empty when there is no more"`
	}
	slackPage struct {
		Page string `json:"page,omitempty" jsonschema:"The next of the call before, for the channels after it"`
	}
	slackChannels struct {
		Channels []map[string]any `json:"channels"`
		Next     string           `json:"next" jsonschema:"Pass as page for more; empty when there is no more"`
	}
	slackWhere struct {
		Channel string `json:"channel" jsonschema:"The conversation's id, from channels or search, like C0123ABCD"`
		Thread  string `json:"thread,omitempty" jsonschema:"A message's ts, for the replies to it"`
		After   string `json:"after,omitempty" jsonschema:"Only those after this ts or time in Unix seconds, like 1762300800"`
		Before  string `json:"before,omitempty" jsonschema:"Only those before this ts or time in Unix seconds"`
		Limit   int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 100; 15 when left out"`
		Page    string `json:"page,omitempty" jsonschema:"The next of the call before, for the messages after it"`
	}
	slackMessages struct {
		Messages []map[string]any `json:"messages"`
		Next     string           `json:"next" jsonschema:"Pass as page for more; empty when there is no more"`
	}
	slackFiles struct {
		Channel string `json:"channel,omitempty" jsonschema:"Only those shared in this conversation, by its id"`
		User    string `json:"user,omitempty" jsonschema:"Only those this person shared, by their user id"`
		After   string `json:"after,omitempty" jsonschema:"Only those shared after this time, in Unix seconds"`
		Before  string `json:"before,omitempty" jsonschema:"Only those shared before this time, in Unix seconds"`
		Limit   int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 100; 50 when left out"`
		Page    int    `json:"page,omitempty" jsonschema:"Which page, from 1"`
	}
	slackFilesFound struct {
		Files []map[string]any `json:"files"`
		Page  int              `json:"page"`
		Pages int              `json:"pages" jsonschema:"How many pages there are; ask for page + 1 while page is less"`
	}
	slackOne struct {
		ID string `json:"id" jsonschema:"Their user id, like U0123ABCD"`
	}
	slackNew struct {
		Channel   string `json:"channel" jsonschema:"The conversation's id, from channels or search"`
		Text      string `json:"text" jsonschema:"What to say, in Slack's markup: *bold*, _italic_, <@U0123ABCD> to mention someone"`
		Thread    string `json:"thread,omitempty" jsonschema:"The ts of the message to reply to, for a reply in its thread"`
		Broadcast bool   `json:"broadcast,omitempty" jsonschema:"Show a reply in the conversation too"`
	}
	slackPosted struct {
		Channel string `json:"channel"`
		TS      string `json:"ts" jsonschema:"The message's ts, its id in the conversation"`
	}
)
