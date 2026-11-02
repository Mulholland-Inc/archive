package connect

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

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
	})
}

const (
	graphAPI    = "https://graph.microsoft.com/v1.0"
	outlookList = "id,conversationId,subject,from,toRecipients,receivedDateTime,bodyPreview,hasAttachments,isRead,webLink"
)
