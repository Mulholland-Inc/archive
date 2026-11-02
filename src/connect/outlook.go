package connect

import (
	"cmp"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// Outlook: the mail of a Microsoft 365 work or school account, or a personal Outlook.com one, read only through
// Microsoft Graph. It reads ARCHIVE_OUTLOOK, an app registration in Microsoft Entra for accounts in any
// directory and personal accounts, whose web redirect URIs list <API>/connect/outlook/callback, with the
// delegated Graph permissions User.Read and Mail.Read. offline_access asks for the refresh token, which
// Microsoft replaces on every refresh. There is no endpoint that revokes one grant: a person removes Archive
// at myapps.microsoft.com, or account.live.com/consent for a personal account.
func init() {
	Register(&Connector{
		ID: "outlook", Name: "Outlook", Env: "ARCHIVE_OUTLOOK", Host: "https://login.microsoftonline.com",
		Endpoint: oauth2.Endpoint{AuthURL: "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
			TokenURL: "https://login.microsoftonline.com/common/oauth2/v2.0/token", AuthStyle: oauth2.AuthStyleInParams},
		Scopes: []string{"offline_access", "User.Read", "Mail.Read"},
		Params: map[string]string{"prompt": "select_account"},
		Account: func(c *Client) (string, error) {
			var me struct {
				Mail string `json:"mail"`
				UPN  string `json:"userPrincipalName"`
			}
			err := c.Get(graphAPI+"/me?$select=mail,userPrincipalName", &me)
			if me.Mail == "" {
				me.Mail = me.UPN
			}
			return strings.ToLower(me.Mail), err
		},
		Actions: map[string]*Action{
			"search": Act("Find messages, newest first, by words anywhere in them or by Outlook's search syntax, like { query: \"from:ann@example.com invoice\" }, "+
				"in every folder or one: { messages: [{ id, conversationId, subject, from, toRecipients, receivedDateTime, bodyPreview, hasAttachments, isRead, webLink }], next } (pass next as page for more)",
				func(c *Client, in struct {
					Query  string `json:"query,omitempty" jsonschema:"Words or KQL like from:ann subject:invoice received>=2026-01-01; every message when left out"`
					Folder string `json:"folder,omitempty" jsonschema:"A folder's id from folders, or a well-known name like inbox, sentitems or archive"`
					Limit  int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 50; 25 when left out"`
					Page   string `json:"page,omitempty" jsonschema:"The next of the search before, for the messages after it"`
				}) (any, error) {
					u := in.Page
					if u == "" {
						path := "/me/messages"
						if in.Folder != "" {
							path = "/me/mailFolders/" + url.PathEscape(in.Folder) + "/messages"
						}
						q := url.Values{"$top": {fmt.Sprint(clamp(in.Limit, 25, 50))}, "$select": {outlookList}}
						if in.Query != "" {
							q.Set("$search", `"`+strings.ReplaceAll(in.Query, `"`, `\"`)+`"`)
						} else {
							q.Set("$orderby", "receivedDateTime desc")
						}
						u = graphAPI + path + "?" + q.Encode()
					} else if !strings.HasPrefix(u, graphAPI+"/") {
						return nil, fmt.Errorf("page is the next of a search before, a %s URL", graphAPI)
					}
					var out struct {
						Value []map[string]any `json:"value"`
						Next  string           `json:"@odata.nextLink"`
					}
					err := c.Get(u, &out)
					return map[string]any{"messages": out.Value, "next": out.Next}, err
				}),
			"message": Act("One message by id with its body as text: { id, conversationId, subject, from, toRecipients, ccRecipients, receivedDateTime, "+
				"body: { contentType, content }, hasAttachments, attachments: [{ name, contentType, size }], webLink }",
				func(c *Client, in struct {
					ID string `json:"id" jsonschema:"The message's id, from search"`
				}) (any, error) {
					var m map[string]any
					u := graphAPI + "/me/messages/" + url.PathEscape(in.ID)
					if err := c.Send("GET", u+"?$select="+outlookList+",ccRecipients,body", http.Header{"Prefer": {`outlook.body-content-type="text"`}}, nil, &m); err != nil {
						return nil, err
					}
					var atts struct {
						Value []map[string]any `json:"value"`
					}
					if m["hasAttachments"] == true {
						if err := c.Get(u+"/attachments?$select=name,contentType,size", &atts); err != nil {
							return nil, err
						}
					}
					m["attachments"] = atts.Value
					return m, nil
				}),
			"folders": Act("The mail folders at the top of the mailbox: [{ id, displayName, totalItemCount, unreadItemCount, childFolderCount }]",
				func(c *Client, in struct{}) (any, error) {
					var out struct {
						Value []map[string]any `json:"value"`
					}
					err := c.Get(graphAPI+"/me/mailFolders?$top=100&$select=id,displayName,totalItemCount,unreadItemCount,childFolderCount", &out)
					return out.Value, err
				}),
		},
		Streams: []*Stream{
			{Name: "folders", Every: time.Hour, Pull: outlookFolders,
				Doc: "Every mail folder, at any depth: { id, displayName, parentFolderId, childFolderCount, totalItemCount, unreadItemCount }, " +
					"from Graph's folder delta query, which says which were removed too."},
			{Name: "messages", Every: 5 * time.Minute, Pull: outlookMessages,
				Doc: "Every message of every folder, as the message action returns it with its body as text cut at 64 KB, by <folder id>/<message id> " +
					"with immutable ids: a delta query of each folder, 50 a page, whole the first time and then what changed since, " +
					"so a moved message is deleted in one folder and added in the other. A folder whose delta Graph no longer keeps is read whole again, " +
					"and what was deleted from it in between is not seen."},
		},
	})
}

// outlookPrefer are the headers of a delta query of messages: 50 a page, ids that stay when a message moves,
// and bodies as text.
var outlookPrefer = http.Header{"Prefer": {"odata.maxpagesize=50", `IdType="ImmutableId"`, `outlook.body-content-type="text"`}}

const outlookSync = outlookList + ",ccRecipients,body,parentFolderId,lastModifiedDateTime,sentDateTime,categories,flag,internetMessageId"

// outlookDelta is a page of a delta query.
type outlookDelta struct {
	Value []map[string]any `json:"value"`
	Next  string           `json:"@odata.nextLink"`
	Delta string           `json:"@odata.deltaLink"`
}

func outlookFolders(c *Client, cursor string) (*Page, error) {
	u := cmp.Or(cursor, graphAPI+"/me/mailFolders/delta?$select=displayName,parentFolderId,childFolderCount,totalItemCount,unreadItemCount")
	var d outlookDelta
	if err := c.Send("GET", u, nil, nil, &d); err != nil {
		return nil, err
	}
	page := &Page{More: d.Next != "", Cursor: cmp.Or(d.Next, d.Delta)}
	for _, f := range d.Value {
		_, removed := f["@removed"]
		page.Records = append(page.Records, Record{ID: str(f, "id"), Data: f, Deleted: removed})
	}
	return page, nil
}

// outlookCursor is where the messages stream is: each folder's deltaLink, by its id, the folders this round
// has left with the first under way, and the nextLink of that one.
type outlookCursor struct {
	Links map[string]string `json:"links,omitempty"`
	Todo  []string          `json:"todo,omitempty"`
	Next  string            `json:"next,omitempty"`
}

func outlookMessages(c *Client, cursor string) (*Page, error) {
	var cur outlookCursor
	if err := cursorOf(cursor, &cur); err != nil {
		return nil, err
	}
	if len(cur.Todo) == 0 {
		// A round reads every folder there is now; a folder gone takes its deltaLink with it.
		for u := graphAPI + "/me/mailFolders/delta?$select=id"; u != ""; {
			var d outlookDelta
			if err := c.Get(u, &d); err != nil {
				return nil, err
			}
			for _, f := range d.Value {
				if _, removed := f["@removed"]; !removed {
					cur.Todo = append(cur.Todo, str(f, "id"))
				}
			}
			u = d.Next
		}
		for id := range cur.Links {
			if !slices.Contains(cur.Todo, id) {
				delete(cur.Links, id)
			}
		}
		if len(cur.Todo) == 0 {
			return &Page{Cursor: cursorFor(cur)}, nil
		}
	}
	f := cur.Todo[0]
	u := cmp.Or(cur.Next, cur.Links[f], graphAPI+"/me/mailFolders/"+url.PathEscape(f)+"/messages/delta?$select="+outlookSync)
	var d outlookDelta
	if err := c.Send("GET", u, outlookPrefer, nil, &d); err != nil {
		if r := (*Refusal)(nil); errors.As(err, &r) && r.Status == http.StatusGone {
			delete(cur.Links, f)
			cur.Next = ""
			return &Page{More: true, Cursor: cursorFor(cur)}, nil
		}
		return nil, err
	}
	page := &Page{}
	for _, m := range d.Value {
		_, removed := m["@removed"]
		if b, ok := m["body"].(map[string]any); ok {
			if t, ok := b["content"].(string); ok {
				b["content"] = clip(t, maxText)
			}
		}
		page.Records = append(page.Records, Record{ID: f + "/" + str(m, "id"), Data: m, Updated: stamp(str(m, "lastModifiedDateTime")), Deleted: removed})
	}
	if cur.Next = d.Next; d.Next == "" {
		if cur.Links == nil {
			cur.Links = map[string]string{}
		}
		cur.Links[f], cur.Todo = d.Delta, cur.Todo[1:]
	}
	page.More = len(cur.Todo) > 0
	page.Cursor = cursorFor(cur)
	return page, nil
}

const (
	graphAPI    = "https://graph.microsoft.com/v1.0"
	outlookList = "id,conversationId,subject,from,toRecipients,receivedDateTime,bodyPreview,hasAttachments,isRead,webLink"
)
