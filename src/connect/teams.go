package connect

import (
	"time"

	"golang.org/x/oauth2"
)

// Microsoft Teams: the chats and the channel posts of a Microsoft 365 work or school account, read only through
// Microsoft Graph. It reads ARCHIVE_OUTLOOK, the app registration in Microsoft Entra that Outlook and SharePoint
// connect with: its web redirect URIs list <API>/connect/teams/callback too, and its delegated Graph permissions
// add User.ReadBasic.All, Chat.Read, Team.ReadBasic.All, Channel.ReadBasic.All and ChannelMessage.Read.All, which
// last a tenant's admin consents to once for everyone. Teams is work and school only, so it signs in through the
// organizations endpoint. Microsoft has no endpoint that revokes one grant: a person removes Archive at
// myapps.microsoft.com.
//
// It syncs the messages of every chat the account is in, and the posts of every channel of every team it is in,
// into chat_message, each sender as the person of their address. Graph keeps no delta of a person's chats, so a
// sync asks each chat for the messages changed since it last read it, which is one cheap call; a channel it
// reads by Graph's delta of it, as Outlook reads a folder. Graph's delta of a channel holds its posts, not their
// replies. Graph pushes chats and channels only to subscriptions that need resource data encrypted, so it pushes
// nothing yet: connecting, asking and reading start a sync.
func init() {
	Register(&Connector{
		ID: "teams", Name: "Microsoft Teams", Env: "ARCHIVE_OUTLOOK",
		Info: Info{By: "Microsoft", Description: "Read your Teams chats and the channels of your teams, never post or change them", Category: CatChat, Icon: IconChat,
			Scopes: []string{"Read your chats", "Read the channels of your teams"}},
		API: "https://graph.microsoft.com/v1.0",
		Endpoint: oauth2.Endpoint{AuthURL: "https://login.microsoftonline.com/organizations/oauth2/v2.0/authorize",
			TokenURL: "https://login.microsoftonline.com/organizations/oauth2/v2.0/token", AuthStyle: oauth2.AuthStyleInParams},
		Scopes: []string{"offline_access", "User.Read", "User.ReadBasic.All", "Chat.Read", "Team.ReadBasic.All", "Channel.ReadBasic.All",
			"ChannelMessage.Read.All"},
		Params: map[string]string{"prompt": "select_account"},
		Who:    &Who{Req: Req{URL: "/me?$select=mail,userPrincipalName"}, Names: []Path{"mail", "userPrincipalName"}, Lower: true},
		Actions: map[string]*Action{
			"chats": Call[teamsLimit, []map[string]any](Plan{
				Doc:   "Your chats, most recently active first: [{ id, topic, chatType, lastUpdatedDateTime, members: [{ displayName, email }], webUrl }]",
				Clamp: map[string][2]int{"limit": {25, 50}},
				Steps: []Step{{As: "got", Req: Req{URL: "/me/chats", Query: Q{"$expand": "members", "$orderby": "lastMessagePreview/createdDateTime desc", "$top": "{limit}"}}}},
				Out:   "{got/value|[]}",
			}),
			"messages": Call[teamsWhere, teamsFound](Plan{
				Doc: "The messages of a chat, or the posts of a channel, newest first: { messages: [{ id, from, body, createdDateTime, " +
					"lastModifiedDateTime, attachments, webUrl }], next } (pass next as page for more)",
				Steps: []Step{
					{If: "page", As: "got", Req: Req{URL: "{+page}"}},
					{Unless: "page", If: "chat", As: "got", Req: Req{URL: "/me/chats/{chat}/messages?$top=50"}},
					{Unless: "got", If: "team", Only: Test{At: "channel"}, As: "got", Req: Req{URL: "/teams/{team}/channels/{channel}/messages?$top=50"}},
					{Unless: "got", Fail: "give a chat, or a team and a channel"},
				},
				Out: Obj{"messages": "{got/value|[]}", "next": "{got/@odata.nextLink|}"},
			}),
			"teams": Call[struct{}, []teamsTeam](Plan{
				Doc: "The teams you are in, with their channels: [{ id, displayName, channels: [{ id, displayName, membershipType }] }]",
				Steps: []Step{
					{As: "got", Req: Req{URL: "/me/joinedTeams?$select=id,displayName"}},
					{Each: "got/value", Onto: "channels", Req: Req{URL: "/teams/{id}/channels?$select=id,displayName,membershipType"}, Page: NextURL{At: "@odata.nextLink"}, Items: "value", Soft: []int{403, 404}},
				},
			}),
		},
		Sync: []*Sync{
			// Teams says what it has done itself, like a member added, in messages that are left out.
			{Name: "chats", Gap: time.Second, Keep: true,
				Parts:  &Parts{Req: Req{URL: "/me/chats?$expand=members&$top=50"}, Items: "value", Page: teamsPage, Vals: Obj{"name": "{topic;members/*/displayName,, }"}},
				Req:    Req{URL: "/me/chats/{part}/messages", Query: Q{"$top": "50", "$orderby": "lastModifiedDateTime desc"}},
				Change: SinceTime{Lasts: true, Req: Req{URL: "/me/chats/{part}/messages", Query: Q{"$top": "50", "$orderby": "lastModifiedDateTime desc", "$filter": "lastModifiedDateTime gt {since}"}}},
				Items:  "value", Key: []Reads{{Of: "{part}/{id}"}}, Updated: "lastModifiedDateTime", Page: teamsPage, Gone: teamsGone, Skip: teamsSkip, Pass: []int{403, 404},
				Set:     Obj{"channel": "{in/name}", "kind": "chat", "thread": "{part}"},
				Hydrate: teamsMore, To: teamsMessage,
				Doc: "Every message of every chat you are in, one-on-one, group or meeting, with its text, cut at 64 KB, the names of the files it " +
					"shares and who sent it, by <chat id>/<message id>: each sync asks each chat for the messages changed since it last read it, " +
					"50 a page; a message deleted there is deleted."},
			{Name: "channels", Gap: time.Second,
				Parts: &Parts{Req: Req{URL: "/me/joinedTeams?$select=id,displayName"}, Items: "value",
					Under: &Parts{Req: Req{URL: "/teams/{up/id}/channels?$select=id,displayName,membershipType"}, Items: "value", Soft: []int{403, 404}, ID: "{up/id}/{id}",
						Vals: Obj{"name": "{up/displayName} / {displayName}", "team": "{up/id}", "channel": "{id}"}}},
				Req:    Req{URL: "/teams/{in/team}/channels/{in/channel}/messages/delta?$top=50"},
				Change: Resume{At: "@odata.deltaLink", Expired: []int{410}, Stale: []Deny{{Status: []int{400}, Says: []string{"SyncState", "resyncRequired", "sync state"}}}},
				Items:  "value", Key: []Reads{{Of: "{in/channel}/{id}"}}, Updated: "lastModifiedDateTime", Page: teamsPage, Gone: teamsGone, Skip: teamsSkip, Pass: []int{403, 404},
				Set:     Obj{"channel": "{in/name}", "kind": "channel", "thread": "{in/channel}/{replyToId;id}"},
				Hydrate: teamsMore, To: teamsMessage,
				Doc: "Every post of every channel of every team you are in, as chats are, by <channel id>/<message id>: listed whole the first " +
					"time, a channel at a time, and then each channel's delta since; replies to a post are not in it. A channel gone, or a delta " +
					"Graph no longer keeps, lists them all again."},
		},
	})
}

var (
	teamsPage = NextURL{At: "@odata.nextLink"}
	teamsGone = []Test{{At: "@removed"}, {At: "deletedDateTime"}}
	teamsSkip = []Test{{At: "messageType", Not: "message"}}
	// A message's text, and the address of who sent it, which Graph does not say and User.ReadBasic.All looks up, once a
	// page for each sender.
	teamsMore = []Hydrate{
		Fetch{If: Test{At: "from/user/id"}, Once: "{from/user/id}", Soft: true, Req: Req{URL: "/users/{from/user/id}?$select=mail,userPrincipalName"}, As: "sender"},
		Decode{Codec: Words, At: "body/content", As: "words", If: Test{At: "body/contentType", Is: "html"}},
	}
	teamsMessage = Map{Table: "chat_message",
		Cols: map[string]SQL{"channel": "data->>'channel'", "kind": "data->>'kind'", "thread": "data->>'thread'", "sender_name": "data#>>'{from,user,displayName}'",
			"text":    "archive.sync_clip(coalesce(data->>'words', data#>>'{body,content}'), 65536)",
			"files":   `(SELECT coalesce(jsonb_agg(a->>'name' ORDER BY a->>'name'), '[]'::jsonb) FROM archive.sync_list(data->'attachments') a WHERE a->>'contentType' = 'reference' AND nullif(a->>'name', '') IS NOT NULL)`,
			"sent_at": "archive.sync_at(data->>'createdDateTime')", "edited_at": "archive.sync_at(data->>'lastModifiedDateTime')", "url": "data->>'webUrl'"},
		People: map[string]SQL{"sender": "coalesce(nullif(data#>>'{sender,mail}', ''), data#>>'{sender,userPrincipalName}')"}}
)

type (
	teamsLimit struct {
		Limit int `json:"limit,omitempty" jsonschema:"At most this many, 1 to 50; 25 when left out"`
	}
	teamsWhere struct {
		Chat    string `json:"chat,omitempty" jsonschema:"A chat's id, from chats"`
		Team    string `json:"team,omitempty" jsonschema:"A team's id, from teams, with channel"`
		Channel string `json:"channel,omitempty" jsonschema:"A channel's id, from teams"`
		Page    string `json:"page,omitempty" jsonschema:"The next of the call before, for the messages after it"`
	}
	teamsFound struct {
		Messages []map[string]any `json:"messages"`
		Next     string           `json:"next" jsonschema:"Pass as page for more; empty when there is no more"`
	}
	teamsChannel struct {
		ID             string `json:"id"`
		DisplayName    string `json:"displayName"`
		MembershipType string `json:"membershipType,omitempty"`
	}
	teamsTeam struct {
		ID          string         `json:"id"`
		DisplayName string         `json:"displayName"`
		Channels    []teamsChannel `json:"channels"`
	}
)
