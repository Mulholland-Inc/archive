package connect

import (
	"cmp"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
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
// sync asks each chat for the messages changed since it last read it; a channel it
// reads by Graph's delta of it, as Outlook reads a folder. Graph's delta of a channel holds its posts, not their
// replies. Graph pushes chats and channels only to subscriptions that need resource data encrypted, so it pushes
// nothing yet: connecting, asking and reading start a sync.
func init() {
	Register(&Connector{
		ID: "teams", Name: "Microsoft Teams", Env: "ARCHIVE_OUTLOOK", Host: "https://login.microsoftonline.com",
		Endpoint: oauth2.Endpoint{AuthURL: "https://login.microsoftonline.com/organizations/oauth2/v2.0/authorize",
			TokenURL: "https://login.microsoftonline.com/organizations/oauth2/v2.0/token", AuthStyle: oauth2.AuthStyleInParams},
		Scopes: []string{"offline_access", "User.Read", "User.ReadBasic.All", "Chat.Read", "Team.ReadBasic.All", "Channel.ReadBasic.All",
			"ChannelMessage.Read.All"},
		Params: map[string]string{"prompt": "select_account"},
		Account: func(c *Client) (string, error) {
			var me struct {
				Mail string `json:"mail"`
				UPN  string `json:"userPrincipalName"`
			}
			err := c.Get(graphAPI+"/me?$select=mail,userPrincipalName", &me)
			return strings.ToLower(cmp.Or(me.Mail, me.UPN)), err
		},
		Actions: map[string]*Action{
			"chats": Act("Your chats, most recently active first: [{ id, topic, chatType, lastUpdatedDateTime, members: [{ displayName, email }], webUrl }]",
				func(c *Client, in struct {
					Limit int `json:"limit,omitempty" jsonschema:"At most this many, 1 to 50; 25 when left out"`
				}) (any, error) {
					var out struct {
						Value []map[string]any `json:"value"`
					}
					q := url.Values{"$expand": {"members"}, "$orderby": {"lastMessagePreview/createdDateTime desc"}, "$top": {strconv.Itoa(clamp(in.Limit, 25, 50))}}
					err := c.Get(graphAPI+"/me/chats?"+q.Encode(), &out)
					return out.Value, err
				}),
			"messages": Act("The messages of a chat, or the posts of a channel, newest first: { messages: [{ id, from, body, createdDateTime, "+
				"lastModifiedDateTime, attachments, webUrl }], next } (pass next as page for more)",
				func(c *Client, in struct {
					Chat    string `json:"chat,omitempty" jsonschema:"A chat's id, from chats"`
					Team    string `json:"team,omitempty" jsonschema:"A team's id, from teams, with channel"`
					Channel string `json:"channel,omitempty" jsonschema:"A channel's id, from teams"`
					Page    string `json:"page,omitempty" jsonschema:"The next of the call before, for the messages after it"`
				}) (any, error) {
					u := in.Page
					switch {
					case u != "":
						if !strings.HasPrefix(u, graphAPI+"/") {
							return nil, errors.New("page is the next of a call before, a Graph URL")
						}
					case in.Chat != "":
						u = graphAPI + "/me/chats/" + url.PathEscape(in.Chat) + "/messages?$top=50"
					case in.Team != "" && in.Channel != "":
						u = graphAPI + "/teams/" + url.PathEscape(in.Team) + "/channels/" + url.PathEscape(in.Channel) + "/messages?$top=50"
					default:
						return nil, errors.New("give a chat, or a team and a channel")
					}
					var out struct {
						Value []map[string]any `json:"value"`
						Next  string           `json:"@odata.nextLink"`
					}
					err := c.Get(u, &out)
					return map[string]any{"messages": out.Value, "next": out.Next}, err
				}),
			"teams": Act("The teams you are in, with their channels: [{ id, displayName, channels: [{ id, displayName, membershipType }] }]",
				func(c *Client, in struct{}) (any, error) {
					parts, err := teamsChannels(c)
					if err != nil {
						return nil, err
					}
					byTeam, order := map[string][]map[string]any{}, []string{}
					names := map[string]string{}
					for _, p := range parts {
						team, channel, _ := strings.Cut(p.ID, "/")
						tn, cn, _ := strings.Cut(p.Name, " / ")
						if _, ok := byTeam[team]; !ok {
							order = append(order, team)
							names[team] = tn
						}
						byTeam[team] = append(byTeam[team], map[string]any{"id": channel, "displayName": cn})
					}
					out := []map[string]any{}
					for _, t := range order {
						out = append(out, map[string]any{"id": t, "displayName": names[t], "channels": byTeam[t]})
					}
					return out, nil
				}),
		},
		Streams: []*Stream{
			{Name: "chats", Gap: time.Second, Pull: teamsChats, Table: "chat_message", Rows: teamsRows,
				Doc: "Every message of every chat you are in, one-on-one, group or meeting, with its text, cut at 64 KB, the names of the files it " +
					"shares and who sent it, by <chat id>/<message id>: each sync asks each chat for the messages changed since it last read it, " +
					"50 a page; a message deleted there is deleted."},
			{Name: "channels", Gap: time.Second, Pull: teamsPosts.pull, Table: "chat_message", Rows: teamsRows,
				Doc: "Every post of every channel of every team you are in, as chats are, by <channel id>/<message id>: listed whole the first " +
					"time, a channel at a time, and then each channel's delta since; replies to a post are not in it. A channel gone, or a delta " +
					"Graph no longer keeps, lists them all again."},
		},
	})
}

// teamsChannels are the channels of every team the account is in, by <team id>/<channel id>, named by their
// team and themselves, like Finance / General.
func teamsChannels(c *Client) ([]graphPart, error) {
	var teams struct {
		Value []struct {
			ID   string `json:"id"`
			Name string `json:"displayName"`
		} `json:"value"`
	}
	if err := c.Get(graphAPI+"/me/joinedTeams?$select=id,displayName", &teams); err != nil {
		return nil, err
	}
	var out []graphPart
	for _, t := range teams.Value {
		var chans struct {
			Value []struct {
				ID   string `json:"id"`
				Name string `json:"displayName"`
			} `json:"value"`
		}
		err := c.Get(graphAPI+"/teams/"+url.PathEscape(t.ID)+"/channels?$select=id,displayName,membershipType", &chans)
		if r := (*Refusal)(nil); errors.As(err, &r) && (r.Status == http.StatusForbidden || r.Status == http.StatusNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, ch := range chans.Value {
			out = append(out, graphPart{ID: t.ID + "/" + ch.ID, Name: t.Name + " / " + ch.Name,
				Start: graphAPI + "/teams/" + url.PathEscape(t.ID) + "/channels/" + url.PathEscape(ch.ID) + "/messages/delta?$top=50"})
		}
	}
	return out, nil
}

// teamsPosts walks each channel's delta of posts.
var teamsPosts = &graphWalk{
	parts: teamsChannels,
	records: func(c *Client, p graphPart, items []map[string]any) ([]Record, error) {
		_, channel, _ := strings.Cut(p.ID, "/")
		return teamsRecords(c, channel, p.Name, "channel", items, func(m map[string]any) string {
			return channel + "/" + cmp.Or(str(m, "replyToId"), str(m, "id"))
		})
	},
}

// teamsChat is a chat the chats stream reads in a sync, from Since, when the sync that last read it began.
type teamsChat struct {
	ID    string `json:"id"`
	Name  string `json:"n,omitempty"`
	Since string `json:"s,omitempty"`
}

// teamsCursor is where the chats stream is: Read when each chat was last read as of, and while a sync is under
// way, the chats it reads, the one under way and the next page of it, and when it began.
type teamsCursor struct {
	Read  map[string]string `json:"read,omitempty"`
	Chats []teamsChat       `json:"chats,omitempty"`
	At    int               `json:"at,omitempty"`
	Next  string            `json:"next,omitempty"`
	Began string            `json:"began,omitempty"`
}

func teamsChats(c *Client, cursor string) (*Page, error) {
	var cur teamsCursor
	if err := cursorOf(cursor, &cur); err != nil {
		return nil, err
	}
	if cur.Read == nil {
		cur.Read = map[string]string{}
	}
	page := &Page{}
	// A sync begins with the chats that changed since each was last read.
	if cur.Began == "" {
		cur.Began = time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)
		u := graphAPI + "/me/chats?$expand=members&$top=50"
		for u != "" {
			var list struct {
				Value []struct {
					ID      string `json:"id"`
					Topic   string `json:"topic"`
					Members []struct {
						Name string `json:"displayName"`
					} `json:"members"`
				} `json:"value"`
				Next string `json:"@odata.nextLink"`
			}
			if err := c.Get(u, &list); err != nil {
				return nil, err
			}
			// A chat's lastUpdatedDateTime moves with its name and members, not its messages, so every chat is
			// asked, each for the messages changed since it was last read, which is one cheap call.
			for _, ch := range list.Value {
				since := cur.Read[ch.ID]
				name := ch.Topic
				if name == "" {
					var names []string
					for _, m := range ch.Members {
						if m.Name != "" {
							names = append(names, m.Name)
						}
					}
					name = strings.Join(names, ", ")
				}
				cur.Chats = append(cur.Chats, teamsChat{ID: ch.ID, Name: name, Since: since})
			}
			u = list.Next
		}
	}
	if cur.At < len(cur.Chats) {
		ch := cur.Chats[cur.At]
		u := cur.Next
		if u == "" {
			u = graphAPI + "/me/chats/" + url.PathEscape(ch.ID) + "/messages?$top=50&$orderby=lastModifiedDateTime desc"
			if ch.Since != "" {
				u += "&$filter=" + url.QueryEscape("lastModifiedDateTime gt "+ch.Since)
			}
		}
		var res struct {
			Value []map[string]any `json:"value"`
			Next  string           `json:"@odata.nextLink"`
		}
		err := c.Get(u, &res)
		if r := (*Refusal)(nil); errors.As(err, &r) && (r.Status == http.StatusForbidden || r.Status == http.StatusNotFound) {
			res.Value, res.Next = nil, ""
		} else if err != nil {
			return nil, err
		}
		recs, err := teamsRecords(c, ch.ID, ch.Name, "chat", res.Value, func(map[string]any) string { return ch.ID })
		if err != nil {
			return nil, err
		}
		page.Records = recs
		if res.Next != "" {
			cur.Next = res.Next
		} else {
			cur.Read[ch.ID], cur.Next = cur.Began, ""
			cur.At++
		}
	}
	if cur.At < len(cur.Chats) {
		page.More = true
	} else {
		cur = teamsCursor{Read: cur.Read}
	}
	page.Cursor = cursorFor(cur)
	return page, nil
}

// teamsRecords are the messages of a chat or a channel, by <its id>/<message id>, as one's id is only its chat's
// or channel's: one deleted is gone, what Teams says of itself, like a member added, is left out, and the rest
// are kept with where they are, their thread, and the address of who sent them, which Graph does not say and
// User.ReadBasic.All looks up, once a page for each sender.
func teamsRecords(c *Client, where, name, kind string, items []map[string]any, thread func(map[string]any) string) ([]Record, error) {
	var out []Record
	emails := map[string]string{}
	for _, m := range items {
		id := where + "/" + str(m, "id")
		if m["@removed"] != nil || str(m, "deletedDateTime") != "" {
			out = append(out, Record{ID: id, Deleted: true})
			continue
		}
		if t := str(m, "messageType"); t != "" && t != "message" {
			continue
		}
		user := obj(obj(m, "from"), "user")
		if uid := str(user, "id"); uid != "" {
			if _, ok := emails[uid]; !ok {
				var u struct {
					Mail string `json:"mail"`
					UPN  string `json:"userPrincipalName"`
				}
				err := c.Get(graphAPI+"/users/"+url.PathEscape(uid)+"?$select=mail,userPrincipalName", &u)
				if err != nil && Busy(err) {
					return nil, err
				}
				emails[uid] = cmp.Or(u.Mail, u.UPN)
			}
			m["senderEmail"] = emails[uid]
		}
		body := obj(m, "body")
		text := str(body, "content")
		if strings.EqualFold(str(body, "contentType"), "html") {
			text = htmlText(text)
		}
		var files []string
		for _, a := range objects(m["attachments"], "") {
			if str(a, "contentType") == "reference" && str(a, "name") != "" {
				files = append(files, str(a, "name"))
			}
		}
		m["text"], m["files"], m["channel"], m["kind"], m["thread"] = clip(text, maxText), files, name, kind, thread(m)
		delete(m, "body")
		out = append(out, Record{ID: id, Data: m, Updated: stamp(str(m, "lastModifiedDateTime"))})
	}
	return out, nil
}

// teamsRows is a message of a chat or a channel, sent by the person of its sender's address.
func teamsRows(m map[string]any) *Row {
	files := strs(m["files"])
	slices.Sort(files)
	return rowOf(map[string]any{"channel": str(m, "channel"), "kind": str(m, "kind"), "thread": str(m, "thread"),
		"sender_name": str(obj(obj(m, "from"), "user"), "displayName"), "text": str(m, "text"), "files": files,
		"sent_at": instant(str(m, "createdDateTime")), "edited_at": instant(str(m, "lastModifiedDateTime")), "url": str(m, "webUrl")}).
		person("sender", str(m, "senderEmail"))
}
