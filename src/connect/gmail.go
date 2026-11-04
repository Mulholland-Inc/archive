package connect

import (
	"bytes"
	"cmp"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/http"
	"net/mail"
	"net/textproto"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// Gmail. It reads ARCHIVE_GOOGLE, the Google client sign-in uses too: its authorized redirect URIs list
// <API>/connect/gmail/callback, like https://api.archive.dev/connect/gmail/callback, and its consent screen asks for
// gmail.modify, a restricted scope: reading, sending, drafts and labels, but never deleting for good. Google sends
// a refresh token only with access_type=offline, and again on a later connect only with prompt=consent.
func init() {
	Register(&Connector{
		ID: "gmail", Name: "Gmail", Env: "ARCHIVE_GOOGLE", Host: "https://accounts.google.com",
		// Gmail allows each user 6,000 units of query cost a minute, and reading a whole message costs a lot of
		// them: 300 requests a minute stay under it, which reads a 5,000-message mailbox in about 17 minutes.
		Pace: 200 * time.Millisecond,
		Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.google.com/o/oauth2/v2/auth", TokenURL: "https://oauth2.googleapis.com/token",
			AuthStyle: oauth2.AuthStyleInParams},
		Revoke: "https://oauth2.googleapis.com/revoke",
		Scopes: []string{"https://www.googleapis.com/auth/gmail.modify"},
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
			"send": Write("Send a message from your Gmail, maybe as a reply that joins its thread: { id, threadId }",
				func(c *Client, in gmailNew) (any, error) {
					m, err := in.message(c)
					if err != nil {
						return nil, err
					}
					var out struct {
						ID       string `json:"id"`
						ThreadID string `json:"threadId"`
					}
					err = c.Do("POST", gmailAPI+"/messages/send", m, &out)
					return out, err
				}),
			"draft": Write("Save a message to your Gmail drafts without sending it, maybe as a reply in its thread: { id, message: { id, threadId } }",
				func(c *Client, in gmailNew) (any, error) {
					m, err := in.message(c)
					if err != nil {
						return nil, err
					}
					var out map[string]any
					err = c.Do("POST", gmailAPI+"/drafts", map[string]any{"message": m}, &out)
					return out, err
				}),
			"label": Write("Add and remove a message's labels: remove INBOX to archive it, UNREAD to mark it read, add STARRED or TRASH, "+
				"or a label of yours by its id from a message's labelIds. Returns { id, threadId, labelIds }",
				func(c *Client, in struct {
					ID     string   `json:"id" jsonschema:"The message's id, from search"`
					Add    []string `json:"add,omitempty" jsonschema:"Label ids to add, like STARRED"`
					Remove []string `json:"remove,omitempty" jsonschema:"Label ids to remove, like INBOX or UNREAD"`
				}) (any, error) {
					var out map[string]any
					err := c.Do("POST", gmailAPI+"/messages/"+url.PathEscape(in.ID)+"/modify",
						map[string]any{"addLabelIds": in.Add, "removeLabelIds": in.Remove}, &out)
					return out, err
				}),
		},
		Push: gmailPush,
		Streams: []*Stream{{Name: "messages", Gap: 5 * time.Second, Pull: gmailMessages, Table: "mail_message", Rows: gmailRows,
			Doc: "Every message but spam and trash, each as the message action returns it without its HTML, its text cut at 64 KB, " +
				"with the words of each PDF, Word, Excel, PowerPoint or text attachment up to 3 MB: " +
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
		if err := gmailAttachmentText(c, raw.ID, m["attachments"].([]map[string]any)); err != nil {
			return err
		}
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

// gmailAttached is the largest attachment the messages stream reads the words of: what fits in the 5 MB a
// Client reads, as Gmail sends it in base64.
const gmailAttached = 3 << 20

// gmailAttachmentText adds to each attachment of message id that Archive reads, like a PDF, its words.
func gmailAttachmentText(c *Client, id string, atts []map[string]any) error {
	for _, a := range atts {
		name, mime, aid := str(a, "filename"), str(a, "mimeType"), str(a, "attachmentId")
		size, _ := a["size"].(int)
		if aid == "" || size > gmailAttached || Extract == nil || !textual(name, mime) {
			continue
		}
		var body struct {
			Data string `json:"data"`
		}
		err := c.Get(gmailAPI+"/messages/"+url.PathEscape(id)+"/attachments/"+url.PathEscape(aid), &body)
		if Busy(err) {
			return err
		}
		if err == nil {
			a["text"] = bytesText(name, mime, gmailData(body.Data))
		}
	}
	return nil
}

// gmailRows is a message as mail: its text, its headers' addresses as its parties, and its attachments. It was
// sent when its Date says, or else when Gmail had it.
func gmailRows(m map[string]any) *Row {
	sent := instant(str(m, "internalDate"))
	if t, err := mail.ParseDate(str(m, "date")); err == nil {
		sent = t.UTC().Format(time.RFC3339)
	}
	r := rowOf(map[string]any{"thread": str(m, "threadId"), "subject": str(m, "subject"), "snippet": str(m, "snippet"), "text": str(m, "text"),
		"sent_at": sent, "received_at": instant(str(m, "internalDate")), "labels": strs(m["labelIds"])})
	for _, role := range []string{"from", "to", "cc", "bcc", "reply_to"} {
		for _, a := range addresses(str(m, strings.ReplaceAll(role, "_", "-"))) {
			r.child("mail_party", rowOf(map[string]any{"role": role, "name": a.Name}).person("person", a.Address))
		}
	}
	for _, f := range objects(m["attachments"], "") {
		r.child("mail_attachment", rowOf(map[string]any{"filename": str(f, "filename"), "mime": str(f, "mimeType"), "size": decimal(f["size"]), "text": str(f, "text")}))
	}
	return r
}

const gmailAPI = "https://gmail.googleapis.com/gmail/v1/users/me"

// clamp is n within 1 and most, or def when it is not given.
func clamp(n, def, most int) int {
	if n <= 0 {
		return def
	}
	return min(n, most)
}

// gmailNew is a message to send or draft.
type gmailNew struct {
	To      string `json:"to,omitempty" jsonschema:"Who it goes to, like Ann <ann@example.com>, bob@example.com; a reply's sender when left out"`
	Cc      string `json:"cc,omitempty" jsonschema:"Who gets a copy"`
	Bcc     string `json:"bcc,omitempty" jsonschema:"Who gets a copy the others don't see"`
	Subject string `json:"subject,omitempty" jsonschema:"The subject; a reply's is Re: and its message's when left out"`
	Text    string `json:"text" jsonschema:"The body, as plain text"`
	HTML    string `json:"html,omitempty" jsonschema:"The body as HTML too, which mail apps show in place of the text"`
	Reply   string `json:"reply,omitempty" jsonschema:"The id of the message this answers, from search; it joins that message's thread"`
}

// message is n as the Gmail API takes one, RFC 5322 in raw.
func (n gmailNew) message(c *Client) (map[string]any, error) {
	out := map[string]any{}
	var hdr []string
	if n.Reply != "" {
		var r gmailRaw
		q := "?format=metadata&metadataHeaders=Message-ID&metadataHeaders=References&metadataHeaders=Subject&metadataHeaders=From&metadataHeaders=Reply-To"
		if err := c.Get(gmailAPI+"/messages/"+url.PathEscape(n.Reply)+q, &r); err != nil {
			return nil, err
		}
		h := map[string]string{}
		for _, x := range r.Payload.Headers {
			h[strings.ToLower(x.Name)] = x.Value
		}
		out["threadId"] = r.ThreadID
		if n.To == "" {
			n.To = cmp.Or(h["reply-to"], h["from"])
		}
		if n.Subject == "" {
			n.Subject = h["subject"]
			if !strings.HasPrefix(strings.ToLower(n.Subject), "re:") {
				n.Subject = "Re: " + n.Subject
			}
		}
		if id := h["message-id"]; id != "" {
			hdr = append(hdr, "In-Reply-To: "+id, "References: "+strings.TrimSpace(h["references"]+" "+id))
		}
	}
	for _, f := range []struct{ k, v string }{{"To", n.To}, {"Cc", n.Cc}, {"Bcc", n.Bcc}} {
		if f.v == "" {
			continue
		}
		as, err := mail.ParseAddressList(f.v)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", strings.ToLower(f.k), err)
		}
		s := make([]string, len(as))
		for i, a := range as {
			s[i] = a.String()
		}
		hdr = append(hdr, f.k+": "+strings.Join(s, ", "))
	}
	hdr = append(hdr, "Subject: "+mime.QEncoding.Encode("utf-8", n.Subject), "MIME-Version: 1.0")
	var b bytes.Buffer
	part := func(w io.Writer, s string) {
		q := quotedprintable.NewWriter(w)
		q.Write([]byte(s))
		q.Close()
	}
	if n.HTML == "" {
		hdr = append(hdr, "Content-Type: text/plain; charset=UTF-8", "Content-Transfer-Encoding: quoted-printable")
		part(&b, n.Text)
	} else {
		mw := multipart.NewWriter(&b)
		hdr = append(hdr, "Content-Type: multipart/alternative; boundary="+mw.Boundary())
		for _, p := range []struct{ typ, s string }{{"text/plain", n.Text}, {"text/html", n.HTML}} {
			w, _ := mw.CreatePart(textproto.MIMEHeader{"Content-Type": {p.typ + "; charset=UTF-8"}, "Content-Transfer-Encoding": {"quoted-printable"}})
			part(w, p.s)
		}
		mw.Close()
	}
	out["raw"] = base64.URLEncoding.EncodeToString([]byte(strings.Join(hdr, "\r\n") + "\r\n\r\n" + b.String()))
	return out, nil
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
		Size         int    `json:"size"`
		Data         string `json:"data"`
		AttachmentID string `json:"attachmentId"`
	} `json:"body"`
	Parts []gmailPart `json:"parts"`
}

// flat is the message as actions return it: its headers by name, and with body its text, HTML and
// attachments out of its parts.
func (m gmailRaw) flat(body bool) map[string]any {
	out := map[string]any{"id": m.ID, "threadId": m.ThreadID, "labelIds": m.LabelIDs, "snippet": m.Snippet}
	for _, h := range m.Payload.Headers {
		if k := strings.ToLower(h.Name); k == "from" || k == "to" || k == "cc" || k == "bcc" || k == "reply-to" || k == "subject" || k == "date" {
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
			atts = append(atts, map[string]any{"filename": p.Filename, "mimeType": p.MIMEType, "size": p.Body.Size, "attachmentId": p.Body.AttachmentID})
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

// gmailPush is users.watch to the Pub/Sub topic ARCHIVE_GMAIL_TOPIC names, which gmail-api-push@
// system.gserviceaccount.com publishes to and a push subscription delivers to Archive, signed by the tasks
// account. A watch lasts 7 days, and Google asks that it be renewed daily: the first sync a day after renews it.
// A push's message is { emailAddress, historyId }, the mailbox it is for.
var gmailPush = &Push{Env: "ARCHIVE_GMAIL_TOPIC", OIDC: true, Renew: 6 * 24 * time.Hour,
	Watch: func(c *Client, h Hook, old []Watch) ([]Watch, error) {
		var w struct {
			Expiration string `json:"expiration"`
		}
		if err := c.Do("POST", gmailAPI+"/watch", map[string]any{"topicName": h.Target}, &w); err != nil {
			return nil, err
		}
		return []Watch{{Key: strings.ToLower(h.Account), Expires: stamp(w.Expiration)}}, nil
	},
	Unwatch: func(c *Client, w Watch) error { return c.Do("POST", gmailAPI+"/stop", nil, nil) },
	Hear: func(p *Provider, r *http.Request, at string, body []byte, secret func(string) string) (*Heard, error) {
		var env struct {
			Message struct {
				Data string `json:"data"`
			} `json:"message"`
		}
		if err := readJSON(body, &env); err != nil {
			return nil, err
		}
		data, err := base64.StdEncoding.DecodeString(env.Message.Data)
		if err != nil {
			return nil, fmt.Errorf("a push whose data is not base64: %v", err)
		}
		var m struct {
			EmailAddress string `json:"emailAddress"`
		}
		if err := readJSON(data, &m); err != nil {
			return nil, err
		}
		return &Heard{Keys: keysOf(strings.ToLower(m.EmailAddress))}, nil
	}}
