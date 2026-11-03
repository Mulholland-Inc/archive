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

// Google Calendar, read only, with ARCHIVE_GOOGLE like Gmail: the client's authorized redirect URIs list
// <API>/connect/gcal/callback, and its consent screen asks for calendar.readonly, a sensitive scope, not a
// restricted one like Gmail's, so Google verifies it without a security assessment.
//
// It syncs every event of each calendar the account owns or can edit into calendar_event, the past and the
// future, a series as one event with its rule and each occurrence that differs from it as another: listed whole
// the first time, and then each calendar's sync token, which says what changed, deleted too. Holidays and the
// birthdays of contacts are left out. A calendar it watches pushes, through a channel of its own.
func init() {
	Register(&Connector{
		ID: "gcal", Name: "Google Calendar", Env: "ARCHIVE_GOOGLE", Host: "https://accounts.google.com",
		Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.google.com/o/oauth2/v2/auth", TokenURL: "https://oauth2.googleapis.com/token",
			AuthStyle: oauth2.AuthStyleInParams},
		Revoke: "https://oauth2.googleapis.com/revoke",
		Scopes: []string{"https://www.googleapis.com/auth/calendar.readonly"},
		Params: map[string]string{"access_type": "offline", "prompt": "consent"},
		Account: func(c *Client) (string, error) {
			var cal struct {
				ID string `json:"id"`
			}
			err := c.Get(calendarAPI+"/calendars/primary", &cal)
			return strings.ToLower(cal.ID), err
		},
		Actions: map[string]*Action{
			"calendars": Act("The calendars on your list: [{ id, summary, description, timeZone, accessRole, primary }]",
				func(c *Client, in struct{}) (any, error) {
					var out struct {
						Items []map[string]any `json:"items"`
					}
					err := c.Get(calendarAPI+"/users/me/calendarList?maxResults=250&fields=items(id,summary,description,timeZone,accessRole,primary)", &out)
					return out.Items, err
				}),
			"events": Act("Events of a calendar, each occurrence of a series on its own, soonest first, maybe only between two times or with "+
				"some words: { events: [{ id, summary, description, location, start, end, status, organizer, attendees, htmlLink, hangoutLink }], next } "+
				"(pass next as page for more)",
				func(c *Client, in struct {
					Calendar string `json:"calendar,omitempty" jsonschema:"The calendar's id, from calendars; the primary one when left out"`
					From     string `json:"from,omitempty" jsonschema:"Only those ending after this time, RFC 3339 like 2026-11-03T00:00:00Z; now when left out"`
					To       string `json:"to,omitempty" jsonschema:"Only those starting before this time, RFC 3339"`
					Query    string `json:"query,omitempty" jsonschema:"Words in the event, its attendees or its location"`
					Limit    int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 250; 50 when left out"`
					Page     string `json:"page,omitempty" jsonschema:"The next of the call before, for the events after it"`
				}) (any, error) {
					q := url.Values{"singleEvents": {"true"}, "orderBy": {"startTime"}, "maxResults": {strconv.Itoa(clamp(in.Limit, 50, 250))},
						"timeMin": {cmp.Or(in.From, time.Now().UTC().Format(time.RFC3339))}}
					if in.To != "" {
						q.Set("timeMax", in.To)
					}
					if in.Query != "" {
						q.Set("q", in.Query)
					}
					if in.Page != "" {
						q.Set("pageToken", in.Page)
					}
					var out struct {
						Items []map[string]any `json:"items"`
						Next  string           `json:"nextPageToken"`
					}
					err := c.Get(calendarAPI+"/calendars/"+url.PathEscape(cmp.Or(in.Calendar, "primary"))+"/events?"+q.Encode(), &out)
					return map[string]any{"events": out.Items, "next": out.Next}, err
				}),
		},
		Push: gcalPush,
		Streams: []*Stream{{Name: "events", Gap: time.Second, Pull: gcalEvents, Table: "calendar_event", Rows: gcalRows,
			Doc: "Every event of each calendar you own or can edit, past and future, by <calendar id>/<event id>: a series as one event with " +
				"its rule, and each occurrence that differs from it as another, with its attendees and their answers and its description, cut " +
				"at 64 KB. Listed whole the first time, 250 a page and a calendar at a time, and then each calendar's changes since; a calendar " +
				"gone, or a sync token Google no longer keeps, lists them all again."}},
	})
}

const calendarAPI = "https://www.googleapis.com/calendar/v3"

// gcalCal is a calendar the events stream walks, and Token its sync token, where its changes start.
type gcalCal struct {
	ID    string `json:"id"`
	Name  string `json:"n,omitempty"`
	Token string `json:"t,omitempty"`
}

// gcalCursor is where the events stream is: the calendars, the one under way, the next page of it, and whether
// this walk lists every calendar whole.
type gcalCursor struct {
	Cals    []gcalCal `json:"cals,omitempty"`
	At      int       `json:"at,omitempty"`
	Page    string    `json:"p,omitempty"`
	Listing bool      `json:"l,omitempty"`
}

// gcalCalendars are the calendars of the account's list it owns or can edit, but holidays and the birthdays of
// contacts, which are Google's.
func gcalCalendars(c *Client) ([]gcalCal, error) {
	var out []gcalCal
	q := url.Values{"maxResults": {"250"}, "minAccessRole": {"writer"}, "fields": {"nextPageToken,items(id,summary,summaryOverride)"}}
	for {
		var list struct {
			Items []struct {
				ID       string `json:"id"`
				Summary  string `json:"summary"`
				Override string `json:"summaryOverride"`
			} `json:"items"`
			Next string `json:"nextPageToken"`
		}
		if err := c.Get(calendarAPI+"/users/me/calendarList?"+q.Encode(), &list); err != nil {
			return nil, err
		}
		for _, it := range list.Items {
			if !strings.Contains(it.ID, "#holiday@") && !strings.Contains(it.ID, "#contacts@") {
				out = append(out, gcalCal{ID: it.ID, Name: cmp.Or(it.Override, it.Summary)})
			}
		}
		if list.Next == "" {
			return out, nil
		}
		q.Set("pageToken", list.Next)
	}
}

func gcalEvents(c *Client, cursor string) (*Page, error) {
	var cur gcalCursor
	if err := cursorOf(cursor, &cur); err != nil {
		return nil, err
	}
	if cursor == "" {
		cur.Listing = true
	}
	// A walk begins with the calendars there are now, each where its changes ended before.
	if cur.At == 0 && cur.Page == "" {
		cals, err := gcalCalendars(c)
		if err != nil {
			return nil, err
		}
		tokens := map[string]string{}
		for _, k := range cur.Cals {
			tokens[k.ID] = k.Token
			if !slices.ContainsFunc(cals, func(n gcalCal) bool { return n.ID == k.ID }) {
				cur.Listing = true
			}
		}
		for i := range cals {
			if !cur.Listing {
				cals[i].Token = tokens[cals[i].ID]
			}
		}
		cur.Cals = cals
	}
	page := &Page{Full: cur.Listing}
	if cur.At >= len(cur.Cals) {
		page.Cursor = cursorFor(gcalCursor{Cals: cur.Cals})
		return page, nil
	}
	cal := &cur.Cals[cur.At]
	q := url.Values{"maxResults": {"250"}}
	if cal.Token != "" {
		q.Set("syncToken", cal.Token)
		q.Set("showDeleted", "true")
	}
	if cur.Page != "" {
		q.Set("pageToken", cur.Page)
	}
	var res struct {
		Items []map[string]any `json:"items"`
		Next  string           `json:"nextPageToken"`
		Sync  string           `json:"nextSyncToken"`
	}
	if err := c.Get(calendarAPI+"/calendars/"+url.PathEscape(cal.ID)+"/events?"+q.Encode(), &res); err != nil {
		if r := (*Refusal)(nil); errors.As(err, &r) && r.Status == http.StatusGone {
			page.More, page.Cursor = true, cursorFor(gcalCursor{Listing: true})
			return page, nil
		}
		return nil, err
	}
	for _, e := range res.Items {
		id := cal.ID + "/" + str(e, "id")
		if str(e, "status") == "cancelled" && cal.Token != "" {
			page.Records = append(page.Records, Record{ID: id, Deleted: true})
			continue
		}
		e["calendarId"], e["calendar"] = cal.ID, cal.Name
		page.Records = append(page.Records, Record{ID: id, Data: e, Updated: stamp(str(e, "updated"))})
	}
	if res.Next != "" {
		cur.Page = res.Next
	} else {
		cal.Token, cur.Page = res.Sync, ""
		cur.At++
	}
	if cur.At < len(cur.Cals) {
		page.More = true
	} else {
		cur.At, cur.Listing = 0, false
	}
	page.Cursor = cursorFor(cur)
	return page, nil
}

// gcalTime is an event's start or end: its dateTime, or for an all-day event its date at midnight UTC.
func gcalTime(v map[string]any) any {
	if t := instant(str(v, "dateTime")); t != nil {
		return t
	}
	if d := day(str(v, "date")); d != nil {
		return d.(string) + "T00:00:00Z"
	}
	return nil
}

// gcalRows is an event of a calendar: who organizes it and who is invited, with what they answered, and where
// to join it.
func gcalRows(m map[string]any) *Row {
	desc := str(m, "description")
	if strings.Contains(desc, "<") {
		desc = htmlText(desc)
	}
	meeting := str(m, "hangoutLink")
	for _, ep := range objects(obj(m, "conferenceData")["entryPoints"], "") {
		if meeting == "" && str(ep, "entryPointType") == "video" {
			meeting = str(ep, "uri")
		}
	}
	org := obj(m, "organizer")
	cal := str(m, "calendarId")
	series := ""
	if id := str(m, "recurringEventId"); id != "" {
		series = cal + "/" + id
	}
	r := rowOf(map[string]any{"calendar": str(m, "calendar"), "title": str(m, "summary"), "description": clip(desc, maxText),
		"location": str(m, "location"), "starts_at": gcalTime(obj(m, "start")), "ends_at": gcalTime(obj(m, "end")),
		"all_day": str(obj(m, "start"), "date") != "", "status": str(m, "status"), "recurrence": strings.Join(strs(m["recurrence"]), "\n"),
		"organizer_name": str(org, "displayName"), "url": str(m, "htmlLink"), "meeting_url": meeting, "edited_at": instant(str(m, "updated"))}).
		ref("series", series).person("organizer", str(org, "email"))
	responses := map[string]string{"accepted": "accepted", "declined": "declined", "tentative": "tentative"}
	for _, a := range objects(m["attendees"], "") {
		if Email(str(a, "email")) == "" {
			continue
		}
		role := "required"
		if a["resource"] == true {
			role = "resource"
		} else if a["optional"] == true {
			role = "optional"
		}
		r.child("calendar_attendee", rowOf(map[string]any{"name": str(a, "displayName"), "role": role,
			"response": cmp.Or(responses[str(a, "responseStatus")], "none")}).person("person", str(a, "email")))
	}
	return r
}

// gcalPush is events.watch on each calendar the stream walks, a channel of Archive's own id that pushes to
// Archive with its token, for a week less an hour: the first sync a day after replaces them all, and stops the
// ones before. Its pushes are Drive's, read the same way.
var gcalPush = &Push{Renew: 6 * 24 * time.Hour,
	Watch: func(c *Client, h Hook, old []Watch) ([]Watch, error) {
		cals, err := gcalCalendars(c)
		if err != nil {
			return nil, err
		}
		exp := strconv.FormatInt(time.Now().Add(7*24*time.Hour-time.Hour).UnixMilli(), 10)
		var out []Watch
		for _, cal := range cals {
			id, token := "archive-"+nonce()[:32], nonce()
			var ch struct {
				ResourceID string `json:"resourceId"`
				Expiration string `json:"expiration"`
			}
			body := map[string]any{"id": id, "type": "web_hook", "address": h.URL(id), "token": token, "expiration": exp}
			if err := c.Do("POST", calendarAPI+"/calendars/"+url.PathEscape(cal.ID)+"/events/watch", body, &ch); err != nil {
				if Busy(err) {
					return nil, err
				}
				continue
			}
			out = append(out, Watch{Key: id, Resource: ch.ResourceID, Secret: token, Expires: stamp(ch.Expiration)})
		}
		for _, o := range old {
			c.Do("POST", calendarAPI+"/channels/stop", map[string]string{"id": o.Key, "resourceId": o.Resource}, nil)
		}
		return out, nil
	},
	Unwatch: func(c *Client, w Watch) error {
		return c.Do("POST", calendarAPI+"/channels/stop", map[string]string{"id": w.Key, "resourceId": w.Resource}, nil)
	},
	Hear: func(p *Provider, r *http.Request, at string, body []byte, secret func(string) string) (*Heard, error) {
		return drivePush.Hear(p, r, at, body, secret)
	},
}
