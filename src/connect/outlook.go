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
// delegated Graph permissions User.Read, Mail.Read and Calendars.Read. offline_access asks for the refresh token, which
// Microsoft replaces on every refresh. There is no endpoint that revokes one grant: a person removes Archive
// at myapps.microsoft.com, or account.live.com/consent for a personal account.
// It syncs the mailbox into mail, as Gmail does, folder by folder with Graph's delta of each, and the calendar
// into calendar_event. An account connected before it asked for Calendars.Read syncs its mail alone until it
// connects again.
func init() {
	Register(&Connector{
		ID: "outlook", Name: "Outlook", Env: "ARCHIVE_OUTLOOK", Host: "https://login.microsoftonline.com",
		Endpoint: oauth2.Endpoint{AuthURL: "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
			TokenURL: "https://login.microsoftonline.com/common/oauth2/v2.0/token", AuthStyle: oauth2.AuthStyleInParams},
		Scopes: []string{"offline_access", "User.Read", "Mail.Read", "Calendars.Read"},
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
		Push: outlookPush,
		Streams: []*Stream{{Name: "messages", Gap: time.Second, Pull: outlookMessages.pull, Table: "mail_message", Rows: outlookRows,
			Doc: "Every message of every folder but Junk Email and Deleted Items, with its text, cut at 64 KB, its attachments' names, and the " +
				"words of each PDF, Word, Excel, PowerPoint or text attachment up to 25 MB: " +
				"listed whole the first time, 50 a page and a folder at a time, and then each folder's delta since, messages added, changed, " +
				"moved or deleted. A folder made since is listed whole; one gone, or a delta Graph no longer keeps, lists the mailbox whole again."},
			{Name: "events", Gap: time.Second, Pull: outlookEvents.pull, Table: "calendar_event", Rows: outlookEventRows,
				Doc: "Every event of your calendar from a year back to a year ahead, each occurrence of a series on its own, with its attendees " +
					"and their answers and its text, cut at 64 KB: listed whole, 50 a page, and then Graph's delta since; the window moves a " +
					"month at a time, which lists it whole again."}},
	})
}

const (
	graphAPI    = "https://graph.microsoft.com/v1.0"
	outlookList = "id,conversationId,subject,from,toRecipients,receivedDateTime,bodyPreview,hasAttachments,isRead,webLink"
	// outlookSync is what the messages stream reads of a message.
	outlookSync = "id,conversationId,subject,from,toRecipients,ccRecipients,bccRecipients,replyTo,receivedDateTime,sentDateTime," +
		"bodyPreview,body,hasAttachments,categories,isDraft,lastModifiedDateTime"
)

// outlookEvents walks the calendar's view from a year back to a year ahead, which is one part named by the
// month it starts in, so a new month is a part gone, and lists the view whole again over the moved window. Times
// come in UTC.
var outlookEvents = &graphWalk{
	hdr:   http.Header{"Prefer": {"odata.maxpagesize=50", `outlook.timezone="UTC"`, `outlook.body-content-type="text"`}},
	parts: outlookCalendar,
	records: func(c *Client, p graphPart, items []map[string]any) ([]Record, error) {
		out := make([]Record, 0, len(items))
		for _, m := range items {
			if _, gone := m["@removed"]; gone {
				out = append(out, Record{ID: str(m, "id"), Deleted: true})
				continue
			}
			m["description"] = clip(str(obj(m, "body"), "content"), maxText)
			delete(m, "body")
			out = append(out, Record{ID: str(m, "id"), Data: m, Updated: stamp(str(m, "lastModifiedDateTime"))})
		}
		return out, nil
	},
}

// outlookCalendar is the one part outlookEvents walks; an account that did not allow Archive its calendar has
// none to walk until it connects again.
func outlookCalendar(c *Client) ([]graphPart, error) {
	if err := c.Get(graphAPI+"/me/calendar?$select=id", nil); err != nil {
		if r := (*Refusal)(nil); errors.As(err, &r) && (r.Status == http.StatusForbidden || r.Status == http.StatusUnauthorized) {
			return nil, unsupported("Outlook did not allow Archive the calendar; connect Outlook again to sync it")
		}
		return nil, err
	}
	now := time.Now().UTC()
	from := time.Date(now.Year()-1, now.Month(), 1, 0, 0, 0, 0, time.UTC)
	q := url.Values{"startDateTime": {from.Format(time.RFC3339)}, "endDateTime": {from.AddDate(2, 1, 0).Format(time.RFC3339)}}
	return []graphPart{{ID: "calendar " + from.Format("2006-01"), Name: "Calendar", Start: graphAPI + "/me/calendarView/delta?" + q.Encode()}}, nil
}

// graphTime is a Graph dateTimeTimeZone, like { dateTime: "2026-11-03T15:00:00.0000000", timeZone: "UTC" }, as an
// instant; nil when it is none. Graph sends UTC when asked to, so another zone is read as UTC too, as near as
// Archive gets without its zone database.
func graphTime(v map[string]any) any {
	s := str(v, "dateTime")
	if s == "" {
		return nil
	}
	if t, err := time.Parse("2006-01-02T15:04:05", clip(s, 19)); err == nil {
		return t.UTC().Format(time.RFC3339)
	}
	return instant(s)
}

// outlookEventRows is an event of the calendar: who organizes it and who is invited, with what they answered.
func outlookEventRows(m map[string]any) *Row {
	status := "confirmed"
	if m["isCancelled"] == true {
		status = "cancelled"
	} else if str(m, "showAs") == "tentative" {
		status = "tentative"
	}
	recurrence := ""
	if p := obj(obj(m, "recurrence"), "pattern"); str(p, "type") != "" {
		recurrence = fmt.Sprintf("%s, every %s", str(p, "type"), cmp.Or(str(p, "interval"), "1"))
		if days := strs(p["daysOfWeek"]); len(days) > 0 {
			recurrence += " on " + strings.Join(days, ", ")
		}
	}
	org := obj(obj(m, "organizer"), "emailAddress")
	r := rowOf(map[string]any{"calendar": "Calendar", "title": str(m, "subject"), "description": str(m, "description"),
		"location": str(obj(m, "location"), "displayName"), "starts_at": graphTime(obj(m, "start")), "ends_at": graphTime(obj(m, "end")),
		"all_day": flag(m, "isAllDay"), "status": status, "recurrence": recurrence, "organizer_name": str(org, "name"), "url": str(m, "webLink"),
		"meeting_url": str(obj(m, "onlineMeeting"), "joinUrl"), "edited_at": instant(str(m, "lastModifiedDateTime"))}).
		ref("series", str(m, "seriesMasterId")).person("organizer", str(org, "address"))
	responses := map[string]string{"accepted": "accepted", "declined": "declined", "tentativelyAccepted": "tentative", "organizer": "accepted"}
	for _, a := range objects(m["attendees"], "") {
		e := obj(a, "emailAddress")
		if Email(str(e, "address")) == "" {
			continue
		}
		r.child("calendar_attendee", rowOf(map[string]any{"name": str(e, "name"), "role": str(a, "type"),
			"response": cmp.Or(responses[str(obj(a, "status"), "response")], "none")}).person("person", str(e, "address")))
	}
	return r
}

// outlookMessages walks each folder's delta of messages. Its text comes as text, not HTML, and 50 a page; a
// moved message is a new id in its new folder and gone from its old one, so neither folder needs the other.
var outlookMessages = &graphWalk{
	hdr:     http.Header{"Prefer": {"odata.maxpagesize=50", `outlook.body-content-type="text"`}},
	parts:   outlookFolders,
	records: outlookRecords,
}

// outlookFolders are every mail folder of the mailbox, the folders in folders too, but Junk Email and Deleted
// Items and what is in them, as Gmail leaves out spam and trash.
func outlookFolders(c *Client) ([]graphPart, error) {
	skip := map[string]bool{}
	for _, wk := range []string{"junkemail", "deleteditems"} {
		var f struct {
			ID string `json:"id"`
		}
		if err := c.Get(graphAPI+"/me/mailFolders/"+wk+"?$select=id", &f); err == nil {
			skip[f.ID] = true
		} else if r := (*Refusal)(nil); !errors.As(err, &r) || r.Status != http.StatusNotFound {
			return nil, err
		}
	}
	var out []graphPart
	var walk func(u string) error
	walk = func(u string) error {
		for u != "" {
			var list struct {
				Value []struct {
					ID       string `json:"id"`
					Name     string `json:"displayName"`
					Children int    `json:"childFolderCount"`
				} `json:"value"`
				Next string `json:"@odata.nextLink"`
			}
			if err := c.Get(u, &list); err != nil {
				return err
			}
			for _, f := range list.Value {
				if skip[f.ID] {
					continue
				}
				out = append(out, graphPart{ID: f.ID, Name: f.Name,
					Start: graphAPI + "/me/mailFolders/" + url.PathEscape(f.ID) + "/messages/delta?$select=" + outlookSync})
				if f.Children > 0 {
					if err := walk(graphAPI + "/me/mailFolders/" + url.PathEscape(f.ID) + "/childFolders?$top=100&$select=id,displayName,childFolderCount"); err != nil {
						return err
					}
				}
			}
			u = list.Next
		}
		return nil
	}
	err := walk(graphAPI + "/me/mailFolders?$top=100&$select=id,displayName,childFolderCount")
	return out, err
}

// outlookRecords are a page of a folder's delta: a message removed is gone, and the rest are kept with the
// folder they are in and the names of their attachments, which the delta leaves out.
func outlookRecords(c *Client, p graphPart, items []map[string]any) ([]Record, error) {
	out := make([]Record, len(items))
	err := each(len(items), 4, func(i int) error {
		m := items[i]
		id := str(m, "id")
		if _, gone := m["@removed"]; gone {
			out[i] = Record{ID: id, Deleted: true}
			return nil
		}
		body := obj(m, "body")
		text := str(body, "content")
		if strings.EqualFold(str(body, "contentType"), "html") {
			text = htmlText(text)
		}
		m["text"], m["folder"] = clip(text, maxText), p.Name
		delete(m, "body")
		if m["hasAttachments"] == true {
			var atts struct {
				Value []map[string]any `json:"value"`
			}
			err := c.Get(graphAPI+"/me/messages/"+url.PathEscape(id)+"/attachments?$select=id,name,contentType,size,isInline", &atts)
			if r := (*Refusal)(nil); errors.As(err, &r) && r.Status == http.StatusNotFound {
				out[i] = Record{ID: id, Deleted: true}
				return nil
			}
			if err != nil {
				return err
			}
			for _, a := range atts.Value {
				if str(a, "@odata.type") != "#microsoft.graph.fileAttachment" {
					continue
				}
				size, _ := a["size"].(float64)
				text, err := c.fileText(graphAPI+"/me/messages/"+url.PathEscape(id)+"/attachments/"+url.PathEscape(str(a, "id"))+"/$value",
					str(a, "name"), str(a, "contentType"), int64(size))
				if err != nil {
					return err
				}
				a["text"] = text
			}
			m["attachments"] = atts.Value
		}
		out[i] = Record{ID: id, Data: m, Updated: stamp(str(m, "lastModifiedDateTime"))}
		return nil
	})
	return out, err
}

// outlookRows is a message as mail: its text, its senders and recipients as its parties, its folder and
// categories as its labels, and its attachments.
func outlookRows(m map[string]any) *Row {
	labels := []string{}
	if f := str(m, "folder"); f != "" {
		labels = append(labels, f)
	}
	labels = append(labels, strs(m["categories"])...)
	sent := instant(str(m, "sentDateTime"))
	if sent == nil {
		sent = instant(str(m, "receivedDateTime"))
	}
	r := rowOf(map[string]any{"thread": str(m, "conversationId"), "subject": str(m, "subject"), "snippet": str(m, "bodyPreview"), "text": str(m, "text"),
		"sent_at": sent, "received_at": instant(str(m, "receivedDateTime")), "labels": labels})
	party := func(role string, v any) {
		if one, ok := v.(map[string]any); ok {
			v = []any{one}
		}
		for _, p := range objects(v, "") {
			a := obj(p, "emailAddress")
			if e := Email(str(a, "address")); e != "" {
				r.child("mail_party", rowOf(map[string]any{"role": role, "name": str(a, "name")}).person("person", e))
			}
		}
	}
	party("from", m["from"])
	party("to", m["toRecipients"])
	party("cc", m["ccRecipients"])
	party("bcc", m["bccRecipients"])
	party("reply_to", m["replyTo"])
	for _, f := range objects(m["attachments"], "") {
		r.child("mail_attachment", rowOf(map[string]any{"filename": str(f, "name"), "mime": str(f, "contentType"), "size": decimal(f["size"]), "text": str(f, "text")}))
	}
	return r
}

// A graphWalk is a stream over Microsoft Graph's deltas, which Graph keeps per part, not per account: per mail
// folder, per drive and per channel. A sync lists the parts again, keeps the delta link each ended at last time,
// and reads each part's delta from there, a page at a time, then the next part's. The first walk reads every part
// whole, as a full listing, and so does one after Graph answers 410 Gone, or 400 for a sync state it no longer
// keeps. A part that is new lists itself whole as its delta, and needs nothing else.
//
// A part is gone only when two walks in a row miss it, as a site search or a team's channels can leave one out
// now and then: then the walk after reads every part whole, which deletes what was in it. A walk that finds no
// parts where the one before found some is an error, not an empty account. A part Graph refuses, 403 or 404, like
// a channel shared from another team, is skipped this walk, its delta kept.
type graphWalk struct {
	// parts are what there is to walk now, each with the first URL of its delta.
	parts func(c *Client) ([]graphPart, error)
	// hdr goes with every page, like the page size Graph is asked for.
	hdr http.Header
	// records are the records of a page of part p's delta.
	records func(c *Client, p graphPart, items []map[string]any) ([]Record, error)
}

// graphPart is a part of a graphWalk: its id and name there, the first URL of its delta, and Delta the delta
// link it ended at, where the next walk starts.
type graphPart struct {
	ID    string `json:"id"`
	Name  string `json:"n,omitempty"`
	Start string `json:"s"`
	Delta string `json:"d,omitempty"`
}

// graphCursor is where a graphWalk is: the parts, the one under way, the next page of it, whether the walk
// reads every part whole, and the parts the walk before missed, with their delta links.
type graphCursor struct {
	Parts   []graphPart `json:"parts,omitempty"`
	At      int         `json:"at,omitempty"`
	Next    string      `json:"next,omitempty"`
	Listing bool        `json:"l,omitempty"`
	Missed  []graphPart `json:"missed,omitempty"`
}

// graphResync says Graph no longer keeps where a delta was: 410 Gone, or a 400 that says so, as mail does.
func graphResync(err error) bool {
	r := (*Refusal)(nil)
	if !errors.As(err, &r) {
		return false
	}
	return r.Status == http.StatusGone || r.Status == http.StatusBadRequest &&
		(strings.Contains(r.Message, "SyncState") || strings.Contains(r.Message, "resyncRequired") || strings.Contains(strings.ToLower(r.Message), "sync state"))
}

func (g *graphWalk) pull(c *Client, cursor string) (*Page, error) {
	var cur graphCursor
	if err := cursorOf(cursor, &cur); err != nil {
		return nil, err
	}
	if cursor == "" {
		cur.Listing = true
	}
	// A walk begins with the parts there are now, each where it ended before.
	if cur.At == 0 && cur.Next == "" {
		parts, err := g.parts(c)
		if err != nil {
			return nil, err
		}
		// A listing too: one of no parts is Full, and would delete every row.
		if len(parts) == 0 && len(cur.Parts) > 0 {
			return nil, errors.New("Microsoft listed none of what it listed before; Archive keeps what it has and asks again next sync")
		}
		ended := map[string]string{}
		for _, p := range append(cur.Missed, cur.Parts...) {
			ended[p.ID] = p.Delta
		}
		has := func(ps []graphPart, id string) bool {
			return slices.ContainsFunc(ps, func(q graphPart) bool { return q.ID == id })
		}
		var missed []graphPart
		for _, p := range cur.Parts {
			if !has(parts, p.ID) {
				if has(cur.Missed, p.ID) {
					cur.Listing = true
				}
				missed = append(missed, p)
			}
		}
		for _, p := range cur.Missed {
			if !has(parts, p.ID) && !has(missed, p.ID) {
				cur.Listing = true
			}
		}
		for i := range parts {
			if !cur.Listing {
				parts[i].Delta = ended[parts[i].ID]
			}
		}
		cur.Parts, cur.Missed = parts, nil
		if !cur.Listing {
			cur.Missed = missed
		}
	}
	page := &Page{Full: cur.Listing}
	if cur.At >= len(cur.Parts) {
		page.Cursor = cursorFor(graphCursor{Parts: cur.Parts, Missed: cur.Missed})
		return page, nil
	}
	p := &cur.Parts[cur.At]
	var res struct {
		Value []map[string]any `json:"value"`
		Next  string           `json:"@odata.nextLink"`
		Delta string           `json:"@odata.deltaLink"`
	}
	err := c.Send("GET", cmp.Or(cur.Next, p.Delta, p.Start), g.hdr, nil, &res)
	switch r := (*Refusal)(nil); {
	case err == nil:
	case graphResync(err):
		// The walk starts again as a listing that keeps its parts, so a listing of none is still refused above.
		page.More, page.Cursor = true, cursorFor(graphCursor{Parts: cur.Parts, Listing: true})
		return page, nil
	case errors.As(err, &r) && (r.Status == http.StatusForbidden || r.Status == http.StatusNotFound):
		// A part refused this walk keeps its delta for the next; a listing that skips one deletes nothing.
		cur.Next = ""
		cur.At++
		page.Full = false
		if cur.Listing {
			cur.Listing = false
			cur.Missed = append(cur.Missed, *p)
		}
		res.Value = nil
	default:
		return nil, err
	}
	if err == nil {
		recs, err := g.records(c, *p, res.Value)
		if err != nil {
			return nil, err
		}
		page.Records = recs
		if res.Next != "" {
			cur.Next = res.Next
		} else {
			p.Delta, cur.Next = res.Delta, ""
			cur.At++
		}
	}
	if cur.At < len(cur.Parts) {
		page.More = true
	} else {
		cur.At, cur.Listing = 0, false
	}
	page.Cursor = cursorFor(cur)
	return page, nil
}

// outlookPush is a Graph subscription to every message of the mailbox, me/messages, and one to every event of
// the calendar, me/events, each with a clientState of Archive's own, for 10,020 minutes, under the 10,080 Outlook
// allows; the first sync a day after renews them with a PATCH, or makes another when Graph no longer has one. An
// account that did not allow Archive its calendar has the mail's alone. Graph checks the URL first by posting a
// validationToken, which Archive answers as it came, in plain text.
var outlookPush = &Push{Renew: 6 * 24 * time.Hour,
	Watch: func(c *Client, h Hook, old []Watch) ([]Watch, error) {
		until := time.Now().Add(10020 * time.Minute).UTC().Format(time.RFC3339)
		had := map[string]Watch{}
		for _, o := range old {
			// A watch from before it named its resource is the mail's.
			had[cmp.Or(o.Resource, "me/messages")] = o
		}
		var out []Watch
		for _, res := range []string{"me/messages", "me/events"} {
			if o, ok := had[res]; ok {
				var s struct {
					Expiration string `json:"expirationDateTime"`
				}
				err := c.Do("PATCH", graphAPI+"/subscriptions/"+url.PathEscape(o.Key), map[string]string{"expirationDateTime": until}, &s)
				if err == nil {
					o.Resource, o.Expires = res, stamp(s.Expiration)
					out = append(out, o)
					continue
				}
				if r := (*Refusal)(nil); !errors.As(err, &r) || r.Status != http.StatusNotFound {
					return nil, err
				}
			}
			state := nonce()[:32]
			var s struct {
				ID         string `json:"id"`
				Expiration string `json:"expirationDateTime"`
			}
			err := c.Do("POST", graphAPI+"/subscriptions", map[string]string{"changeType": "created,updated,deleted", "notificationUrl": h.URL(""),
				"resource": res, "expirationDateTime": until, "clientState": state}, &s)
			if r := (*Refusal)(nil); res == "me/events" && errors.As(err, &r) && (r.Status == http.StatusForbidden || r.Status == http.StatusUnauthorized) {
				continue
			}
			if err != nil {
				return nil, err
			}
			out = append(out, Watch{Key: s.ID, Resource: res, Secret: state, Expires: stamp(s.Expiration)})
		}
		return out, nil
	},
	Unwatch: func(c *Client, w Watch) error {
		return c.Do("DELETE", graphAPI+"/subscriptions/"+url.PathEscape(w.Key), nil, nil)
	},
	Hear: func(p *Provider, r *http.Request, at string, body []byte, secret func(string) string) (*Heard, error) {
		if v := r.URL.Query().Get("validationToken"); v != "" {
			return &Heard{Status: http.StatusOK, Header: http.Header{"Content-Type": {"text/plain; charset=utf-8"}}, Body: v}, nil
		}
		var n struct {
			Value []struct {
				SubscriptionID string `json:"subscriptionId"`
				ClientState    string `json:"clientState"`
			} `json:"value"`
		}
		if err := readJSON(body, &n); err != nil {
			return nil, err
		}
		var keys []string
		for _, v := range n.Value {
			switch kept := secret(v.SubscriptionID); {
			case kept == "":
			case !same(kept, v.ClientState):
				return nil, forged("subscription %s's clientState", v.SubscriptionID)
			default:
				keys = append(keys, v.SubscriptionID)
			}
		}
		return &Heard{Keys: keysOf(keys...)}, nil
	}}
