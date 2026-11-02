package connect

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/oauth2"
)

// Gmail, read only. It reads ARCHIVE_GOOGLE, the Google client sign-in uses too: its authorized redirect URIs
// list <API>/connect/gmail/callback, like https://api.archive.dev/connect/gmail/callback, and its consent screen asks for
// gmail.readonly, a restricted scope. Google sends a refresh token only with access_type=offline, and again on
// a later connect only with prompt=consent.
func init() {
	Register(&Connector{
		ID: "gmail", Name: "Gmail", Env: "ARCHIVE_GOOGLE", Host: "https://accounts.google.com",
		Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.google.com/o/oauth2/v2/auth", TokenURL: "https://oauth2.googleapis.com/token",
			AuthStyle: oauth2.AuthStyleInParams},
		Revoke: "https://oauth2.googleapis.com/revoke",
		Scopes: []string{"https://www.googleapis.com/auth/gmail.readonly"},
		Params: map[string]string{"access_type": "offline", "prompt": "consent"},
		Account: func(c *Client) (string, error) {
			var p struct {
				EmailAddress string `json:"emailAddress"`
			}
			err := c.Get(gmailAPI+"/profile", &p)
			return strings.ToLower(p.EmailAddress), err
		},
		Actions: map[string]*Action{
			"search": Act("Find messages as Gmail's search box does, like { query: \"from:ann@example.com newer_than:7d\" }, newest first: "+
				"{ messages: [{ id, threadId, from, to, subject, date, snippet, labelIds }], next } (pass next as page for more)",
				func(c *Client, in struct {
					Query string `json:"query" jsonschema:"Gmail search, like from:ann has:attachment after:2026/01/01"`
					Limit int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 50; 20 when left out"`
					Page  string `json:"page,omitempty" jsonschema:"The next of the search before, for the messages after it"`
				}) (any, error) {
					q := url.Values{"q": {in.Query}, "maxResults": {fmt.Sprint(clamp(in.Limit, 20, 50))}}
					if in.Page != "" {
						q.Set("pageToken", in.Page)
					}
					var list struct {
						Messages []struct {
							ID string `json:"id"`
						} `json:"messages"`
						Next string `json:"nextPageToken"`
					}
					if err := c.Get(gmailAPI+"/messages?"+q.Encode(), &list); err != nil {
						return nil, err
					}
					out := []map[string]any{}
					for _, m := range list.Messages {
						var raw gmailRaw
						if err := c.Get(gmailAPI+"/messages/"+url.PathEscape(m.ID)+"?format=metadata&metadataHeaders=From&metadataHeaders=To&metadataHeaders=Subject&metadataHeaders=Date", &raw); err != nil {
							return nil, err
						}
						out = append(out, raw.flat(false))
					}
					return map[string]any{"messages": out, "next": list.Next}, nil
				}),
			"message": Act("One message by id, with its text: { id, threadId, from, to, cc, subject, date, snippet, labelIds, text, html, attachments: [{ filename, mimeType, size }] }",
				func(c *Client, in struct {
					ID string `json:"id" jsonschema:"The message's id, from search"`
				}) (any, error) {
					var raw gmailRaw
					err := c.Get(gmailAPI+"/messages/"+url.PathEscape(in.ID)+"?format=full", &raw)
					return raw.flat(true), err
				}),
			"thread": Act("A conversation by its threadId, every message with its text, oldest first: { id, messages: [...as message returns] }",
				func(c *Client, in struct {
					ID string `json:"id" jsonschema:"The thread's id, a message's threadId"`
				}) (any, error) {
					var raw struct {
						ID       string     `json:"id"`
						Messages []gmailRaw `json:"messages"`
					}
					if err := c.Get(gmailAPI+"/threads/"+url.PathEscape(in.ID)+"?format=full", &raw); err != nil {
						return nil, err
					}
					msgs := make([]map[string]any, len(raw.Messages))
					for i, m := range raw.Messages {
						msgs[i] = m.flat(true)
					}
					return map[string]any{"id": raw.ID, "messages": msgs}, nil
				}),
		},
	})
}

const gmailAPI = "https://gmail.googleapis.com/gmail/v1/users/me"

// clamp is n within 1 and most, or def when it is not given.
func clamp(n, def, most int) int {
	if n <= 0 {
		return def
	}
	return min(n, most)
}

// gmailRaw is a message as the Gmail API has it, a tree of MIME parts.
type gmailRaw struct {
	ID       string    `json:"id"`
	ThreadID string    `json:"threadId"`
	LabelIDs []string  `json:"labelIds"`
	Snippet  string    `json:"snippet"`
	Payload  gmailPart `json:"payload"`
}

type gmailPart struct {
	MIMEType string `json:"mimeType"`
	Filename string `json:"filename"`
	Headers  []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"headers"`
	Body struct {
		Size int    `json:"size"`
		Data string `json:"data"`
	} `json:"body"`
	Parts []gmailPart `json:"parts"`
}

// flat is the message as actions return it: its headers by name, and with body its text, HTML and
// attachments out of its parts.
func (m gmailRaw) flat(body bool) map[string]any {
	out := map[string]any{"id": m.ID, "threadId": m.ThreadID, "labelIds": m.LabelIDs, "snippet": m.Snippet}
	for _, h := range m.Payload.Headers {
		if k := strings.ToLower(h.Name); k == "from" || k == "to" || k == "cc" || k == "subject" || k == "date" {
			out[k] = h.Value
		}
	}
	if !body {
		return out
	}
	var text, html strings.Builder
	atts := []map[string]any{}
	var walk func(p gmailPart)
	walk = func(p gmailPart) {
		switch {
		case p.Filename != "":
			atts = append(atts, map[string]any{"filename": p.Filename, "mimeType": p.MIMEType, "size": p.Body.Size})
		case p.MIMEType == "text/plain":
			text.Write(gmailData(p.Body.Data))
		case p.MIMEType == "text/html":
			html.Write(gmailData(p.Body.Data))
		}
		for _, q := range p.Parts {
			walk(q)
		}
	}
	walk(m.Payload)
	out["text"], out["html"], out["attachments"] = text.String(), html.String(), atts
	return out
}

func gmailData(s string) []byte {
	b, _ := base64.URLEncoding.DecodeString(s)
	if b == nil {
		b, _ = base64.RawURLEncoding.DecodeString(s)
	}
	return b
}
