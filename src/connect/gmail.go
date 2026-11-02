package connect

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

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
		Push: gmailPush,
		Streams: []*Stream{{Name: "messages", Gap: 5 * time.Second, Pull: gmailMessages,
			Doc: "Every message but spam and trash, each as the message action returns it without its HTML, its text cut at 64 KB: " +
				"listed whole the first time, 50 a page, and then the mailbox's history since, messages added, deleted or relabeled. " +
				"A history Gmail no longer keeps, about a week's, lists the mailbox whole again."}},
	})
}

// gmailPage is how many messages a page of a full listing reads: 5 quota units for the list and 5 for each
// message, 255 a page, so pages 5 seconds apart keep a mailbox's sync under half of Gmail's 6,000 a minute
// for each user, which leaves the rest to actions. A history page lists up to 100 changes.
const gmailPage = 50

// gmailCursor is where the messages stream is: History is the mailbox's historyId its listing began at, or
// its delta starts from, Page the next page of either, and Listing says a listing is under way.
type gmailCursor struct {
	History string `json:"h"`
	Page    string `json:"p,omitempty"`
	Listing bool   `json:"l,omitempty"`
}

func gmailMessages(c *Client, cursor string) (*Page, error) {
	var cur gmailCursor
	if err := cursorOf(cursor, &cur); err != nil {
		return nil, err
	}
	if cur.History == "" {
		var p struct {
			HistoryID string `json:"historyId"`
		}
		if err := c.Get(gmailAPI+"/profile", &p); err != nil {
			return nil, err
		}
		cur = gmailCursor{History: p.HistoryID, Listing: true}
	}
	if cur.Listing {
		q := url.Values{"maxResults": {fmt.Sprint(gmailPage)}}
		if cur.Page != "" {
			q.Set("pageToken", cur.Page)
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
		ids := make([]string, len(list.Messages))
		for i, m := range list.Messages {
			ids[i] = m.ID
		}
		page := &Page{Full: true, More: list.Next != ""}
		var err error
		page.Records, err = gmailRecords(c, ids, nil)
		cur.Page, cur.Listing = list.Next, list.Next != ""
		page.Cursor = cursorFor(cur)
		return page, err
	}
	q := url.Values{"startHistoryId": {cur.History}, "maxResults": {"100"},
		"historyTypes": {"messageAdded", "messageDeleted", "labelAdded", "labelRemoved"}}
	if cur.Page != "" {
		q.Set("pageToken", cur.Page)
	}
	type ref struct {
		Message struct {
			ID string `json:"id"`
		} `json:"message"`
	}
	var h struct {
		History []struct {
			Added   []ref `json:"messagesAdded"`
			Deleted []ref `json:"messagesDeleted"`
			Labeled []ref `json:"labelsAdded"`
			Removed []ref `json:"labelsRemoved"`
		} `json:"history"`
		Next      string `json:"nextPageToken"`
		HistoryID string `json:"historyId"`
	}
	if err := c.Get(gmailAPI+"/history?"+q.Encode(), &h); err != nil {
		if r := (*Refusal)(nil); errors.As(err, &r) && r.Status == 404 {
			return &Page{More: true}, nil
		}
		return nil, err
	}
	// A message's last change in the history says what it is now: gone, or there to read again.
	gone, order := map[string]bool{}, []string{}
	note := func(refs []ref, deleted bool) {
		for _, r := range refs {
			if _, seen := gone[r.Message.ID]; !seen {
				order = append(order, r.Message.ID)
			}
			gone[r.Message.ID] = deleted
		}
	}
	for _, e := range h.History {
		note(e.Added, false)
		note(e.Labeled, false)
		note(e.Removed, false)
		note(e.Deleted, true)
	}
	var read []string
	page := &Page{More: h.Next != ""}
	for _, id := range order {
		if gone[id] {
			page.Records = append(page.Records, Record{ID: id, Deleted: true})
		} else {
			read = append(read, id)
		}
	}
	got, err := gmailRecords(c, read, gone)
	page.Records = append(page.Records, got...)
	if h.Next != "" {
		cur.Page = h.Next
	} else {
		cur = gmailCursor{History: h.HistoryID}
	}
	page.Cursor = cursorFor(cur)
	return page, err
}

// gmailRecords reads the messages ids, five at a time, as records; one deleted since is a deleted record when
// gone is given, and left out when not.
func gmailRecords(c *Client, ids []string, gone map[string]bool) ([]Record, error) {
	out := make([]Record, len(ids))
	err := each(len(ids), 5, func(i int) error {
		var raw gmailRaw
		err := c.Get(gmailAPI+"/messages/"+url.PathEscape(ids[i])+"?format=full", &raw)
		if r := (*Refusal)(nil); errors.As(err, &r) && r.Status == 404 {
			out[i] = Record{ID: ids[i], Deleted: gone != nil}
			return nil
		}
		if err != nil {
			return err
		}
		m := raw.flat(true)
		if m["text"] == "" {
			m["text"] = htmlText(m["html"].(string))
		}
		m["text"] = clip(m["text"].(string), maxText)
		delete(m, "html")
		m["internalDate"], m["historyId"] = raw.InternalDate, raw.HistoryID
		out[i] = Record{ID: raw.ID, Data: m}
		return nil
	})
	if err != nil {
		return nil, err
	}
	kept := out[:0]
	for _, r := range out {
		if r.Data != nil || r.Deleted {
			kept = append(kept, r)
		}
	}
	return kept, nil
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
	ID           string    `json:"id"`
	InternalDate string    `json:"internalDate"`
	HistoryID    string    `json:"historyId"`
	ThreadID     string    `json:"threadId"`
	LabelIDs     []string  `json:"labelIds"`
	Snippet      string    `json:"snippet"`
	Payload      gmailPart `json:"payload"`
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
