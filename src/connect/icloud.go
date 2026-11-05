package connect

import (
	"bytes"
	"cmp"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-imap"
	imapclient "github.com/emersion/go-imap/client"
	_ "github.com/emersion/go-message/charset"
	gomail "github.com/emersion/go-message/mail"
	"github.com/emersion/go-vcard"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"
)

// iCloud speaks IMAP for mail and CalDAV and CardDAV for calendars and contacts, not JSON, so its streams and actions
// are Go: each Pull keeps its own cursor, and its rows land through a Map like any other stream's.
func init() {
	Register(&Connector{
		ID: "icloud", Name: "iCloud",
		Info: Info{By: "Apple", Description: "Read your iCloud Mail, calendars and contacts, never send or change them", Category: CatEmail, Icon: IconMail,
			Scopes: []string{"Read your iCloud Mail and its attachments", "Read your iCloud calendars", "Read your iCloud contacts"}},
		API: icloudCalDAV,
		Keys: []Key{
			{Name: "email", Label: "Apple ID", Help: "The email you sign in to iCloud with, or your @icloud.com address if you use iCloud Mail"},
			{Name: "password", Label: "App-specific password", Secret: true, Link: "https://account.apple.com/account/manage/section/security",
				Help: "Apple’s Sign-In and Security page, then App-Specific Passwords, then +, named Archive. It looks like abcd-efgh-ijkl-mnop, and Archive checks it as you connect"},
		},
		Basic:   [2]string{"email", "password"},
		Account: icloudAccount,
		Actions: map[string]*Action{
			"search": Func("Find iCloud Mail messages, newest first, by words in their subject, sender or text, in one folder or the inbox: "+
				"{ messages: [{ id, folder, subject, from, to, date, unread }] }; folders lists the folders", icloudSearch),
			"message": Func("One iCloud Mail message by its id from search, with its text and its attachments' names", icloudOne),
			"folders": Func("The folders of your iCloud Mail, like INBOX, Sent Messages and Archive", icloudFolders),
		},
		Sync: []*Sync{
			{Name: "messages", Gap: time.Second, Pull: icloudMail, Memo: "imap 1",
				Doc: "Every message of every iCloud Mail folder but Junk and Deleted Messages, with its text cut at 64 KB and the words of each PDF, Word, Excel, " +
					"PowerPoint or text attachment up to 3 MB, by <folder>/<uidvalidity>/<uid>: listed whole the first time and once a day after, " +
					"which drops those deleted or moved, and in between only the messages new to each folder. A message's folder and read and " +
					"flagged state are as it was when Archive first read it, until it moves.",
				To: Map{Table: "mail_message",
					Cols: map[string]SQL{"thread": "data->>'threadId'", "subject": "data->>'subject'", "snippet": "data->>'snippet'", "text": "data->>'text'",
						"sent_at": "archive.sync_at(coalesce(data->>'sent', data->>'received'))", "received_at": "archive.sync_at(data->>'received')",
						"labels": "coalesce(data->'labelIds', '[]'::jsonb)"},
					Children: []Kids{
						{Table: "mail_party", From: "jsonb_array_elements(data->'parties') p",
							Cols: map[string]SQL{"role": "p->>'role'", "name": "p->>'name'"}, People: map[string]SQL{"person": "p->>'address'"}},
						{Table: "mail_attachment", From: "jsonb_array_elements(data->'attachments') f",
							Cols: map[string]SQL{"filename": "f->>'filename'", "mime": "f->>'mimeType'", "size": "archive.sync_num(f->>'size')", "text": "f->>'text'", "file": "f->>'file'"}},
					}}},
			{Name: "events", Gap: time.Second, Every: 15 * time.Minute, Pull: icloudEvents, Memo: "caldav 1",
				Doc: "Every event of each of your iCloud calendars, past and future, by <calendar>/<uid>: a series as one event with its rule, and each " +
					"occurrence that differs from it as another, by <calendar>/<uid>/<recurrence id>, with its attendees and their answers. " +
					"Listed whole every 15 minutes, a calendar a page, which drops those deleted.",
				To: Map{Table: "calendar_event",
					Cols: map[string]SQL{"calendar": "data->>'calendar'", "title": "data->>'summary'", "description": "archive.sync_clip(data->>'description', 65536)", "location": "data->>'location'",
						"starts_at": "coalesce(archive.sync_at(data#>>'{start,dateTime}'), archive.sync_at(archive.sync_day(data#>>'{start,date}')::text || 'T00:00:00Z'))",
						"ends_at":   "coalesce(archive.sync_at(data#>>'{end,dateTime}'), archive.sync_at(archive.sync_day(data#>>'{end,date}')::text || 'T00:00:00Z'))",
						"all_day":   "nullif(data#>>'{start,date}', '') IS NOT NULL", "status": "data->>'status'",
						"recurrence":     `(SELECT string_agg(r#>>'{}', E'\n') FROM archive.sync_list(data->'recurrence') r)`,
						"organizer_name": "data#>>'{organizer,displayName}'", "url": "data->>'url'", "edited_at": "archive.sync_at(data->>'updated')"},
					People: map[string]SQL{"organizer": "data#>>'{organizer,email}'"},
					Refs:   map[string]SQL{"series": "(data->>'calendarId') || '/' || nullif(data->>'recurringEventId', '')"},
					Children: []Kids{{Table: "calendar_attendee", From: "archive.sync_list(data->'attendees') a WHERE archive.sync_email(a->>'email') IS NOT NULL",
						Cols: map[string]SQL{"name": "a->>'displayName'", "role": "CASE WHEN a->>'resource' = 'true' THEN 'resource' WHEN a->>'optional' = 'true' THEN 'optional' ELSE 'required' END",
							"response": "CASE WHEN a->>'responseStatus' IN ('accepted', 'declined', 'tentative') THEN a->>'responseStatus' ELSE 'none' END"},
						People: map[string]SQL{"person": "a->>'email'"}}}}},
			{Name: "contacts", Gap: time.Second, Every: time.Hour, Pull: icloudContacts, Memo: "carddav 1",
				Doc: "Every contact of your iCloud address books, with their first email, phone, company, title and city, by <address book>/<card>. " +
					"Listed whole every hour, 200 a page, which drops those deleted.",
				To: Map{Table: "crm_contact",
					Cols: map[string]SQL{"first_name": "data->>'first_name'", "last_name": "data->>'last_name'", "email": "archive.sync_email(data->>'email')",
						"phone": "data->>'phone'", "job_title": "data->>'title'", "company_name": "data->>'company'", "city": "data->>'city'",
						"state": "data->>'state'", "country": "data->>'country'", "edited_at": "archive.sync_at(data->>'edited')"},
					People: map[string]SQL{"person": "data->>'email'"}}},
		},
	})
}

const (
	icloudIMAPHost = "imap.mail.me.com:993"
	icloudCalDAV   = "https://caldav.icloud.com"
	icloudCardDAV  = "https://contacts.icloud.com"
	icloudBatch    = 25
	icloudSames    = 5000
	icloudBudget   = 75 * time.Second
	icloudRelist   = 24 * time.Hour
	icloudWhole    = 25 << 20
	icloudFile     = 3 << 20
)

func icloudAccount(c *Client) (string, error) {
	if !strings.Contains(c.Key("email"), "@") {
		return "", icloudRefused(c, http.StatusUnauthorized, "an Apple ID is an email address, like you@icloud.com")
	}
	if !icloudAppPassword.MatchString(c.Key("password")) {
		return "", icloudRefused(c, http.StatusUnauthorized, "that is not an app-specific password, which looks like abcd-efgh-ijkl-mnop; Archive never takes your Apple ID password. Make one on Apple’s Sign-In and Security page")
	}
	if _, _, err := icloudHome(c, icloudCalDAV, true); err != nil {
		return "", err
	}
	return strings.ToLower(c.Key("email")), nil
}

var icloudAppPassword = regexp.MustCompile(`^[a-zA-Z]{4}-?[a-zA-Z]{4}-?[a-zA-Z]{4}-?[a-zA-Z]{4}$`)

func icloudRefused(c *Client, status int, msg string) *Refusal {
	return &Refusal{Provider: c.p.Name, Status: status, Message: msg}
}

func icloudDial(c *Client) (*imapclient.Client, error) {
	d := &net.Dialer{Timeout: 30 * time.Second}
	ic, err := imapclient.DialWithDialerTLS(d, icloudIMAPHost, &tls.Config{ServerName: "imap.mail.me.com"})
	if err != nil {
		return nil, unreached{fmt.Errorf("iCloud Mail could not be reached: %w", err)}
	}
	ic.Timeout = time.Minute
	if err := ic.Login(c.Key("email"), c.Key("password")); err != nil {
		ic.Logout()
		return nil, icloudRefused(c, http.StatusUnauthorized, "iCloud Mail did not take that Apple ID and app-specific password, or this Apple ID has no iCloud Mail: "+err.Error())
	}
	return ic, nil
}

type (
	icloudBox struct {
		Valid uint32 `json:"v"`
		Last  uint32 `json:"l"`
	}
	icloudMailState struct {
		Mode   string               `json:"mode,omitempty"`
		Boxes  []string             `json:"boxes,omitempty"`
		At     int                  `json:"at,omitempty"`
		From   uint32               `json:"from,omitempty"`
		Seen   map[string]icloudBox `json:"seen,omitempty"`
		Listed string               `json:"listed,omitempty"`
	}
)

func icloudSkipped(m *imap.MailboxInfo) bool {
	for _, a := range m.Attributes {
		if slices.Contains([]string{imap.NoSelectAttr, imap.JunkAttr, imap.TrashAttr, "\\NonExistent"}, a) {
			return true
		}
	}
	return slices.Contains([]string{"Junk", "Deleted Messages", "Trash"}, m.Name)
}

func icloudBoxes(ic *imapclient.Client) ([]string, error) {
	ch := make(chan *imap.MailboxInfo, 50)
	done := make(chan error, 1)
	go func() { done <- ic.List("", "*", ch) }()
	var out []string
	for m := range ch {
		if !icloudSkipped(m) {
			out = append(out, m.Name)
		}
	}
	if err := <-done; err != nil {
		return nil, err
	}
	slices.Sort(out)
	return out, nil
}

func icloudMail(c *Client, cursor string) (*Page, error) {
	var st icloudMailState
	if err := cursorOf(cursor, &st); err != nil {
		return nil, err
	}
	if st.Seen == nil {
		st.Seen = map[string]icloudBox{}
	}
	ic, err := icloudDial(c)
	if err != nil {
		var r *Refusal
		if errors.As(err, &r) {
			return nil, unsupported("%s", r.Message)
		}
		return nil, err
	}
	defer ic.Logout()
	if st.Mode == "" {
		if st.Boxes, err = icloudBoxes(ic); err != nil {
			return nil, err
		}
		st.Mode, st.At, st.From = "delta", 0, 0
		if t, err := time.Parse(time.RFC3339, st.Listed); err != nil || time.Since(t) > icloudRelist {
			st.Mode = "list"
		}
	}
	list := st.Mode == "list"
	until := time.Now().Add(icloudBudget)
	if d, ok := c.ctx.Deadline(); ok && d.Add(-30*time.Second).Before(until) {
		until = d.Add(-30 * time.Second)
	}
	var recs []Record
	sames := 0
	for st.At < len(st.Boxes) && time.Now().Before(until) && len(recs) < 8*icloudBatch && sames < icloudSames {
		box := st.Boxes[st.At]
		status, err := ic.Select(box, true)
		if err != nil {
			st.At, st.From = st.At+1, 0
			continue
		}
		was := st.Seen[box]
		if was.Valid != status.UidValidity {
			was = icloudBox{Valid: status.UidValidity}
		}
		crit := imap.NewSearchCriteria()
		if !list {
			crit.Uid = new(imap.SeqSet)
			crit.Uid.AddRange(max(was.Last, st.From)+1, 0)
		}
		uids, err := ic.UidSearch(crit)
		if err != nil {
			return nil, err
		}
		if !list {
			uids = slices.DeleteFunc(uids, func(u uint32) bool { return u <= was.Last })
		}
		slices.Sort(uids)
		var fetch []uint32
		for _, u := range uids {
			if u <= st.From {
				continue
			}
			if u <= was.Last {
				if list && sames < icloudSames {
					recs = append(recs, Record{ID: icloudID(box, status.UidValidity, u), Same: true})
					sames++
					st.From = u
				}
				continue
			}
			if len(fetch) < icloudBatch && sames < icloudSames {
				fetch = append(fetch, u)
			}
		}
		got, err := icloudFetch(c, ic, box, status.UidValidity, fetch)
		if err != nil {
			return nil, err
		}
		recs = append(recs, got...)
		if len(fetch) > 0 {
			st.From = fetch[len(fetch)-1]
			was.Last = max(was.Last, st.From)
		}
		st.Seen[box] = was
		if len(uids) == 0 || st.From >= uids[len(uids)-1] {
			st.At, st.From = st.At+1, 0
		}
	}
	more := st.At < len(st.Boxes)
	if !more {
		if list {
			st.Listed = time.Now().UTC().Format(time.RFC3339)
			for k := range st.Seen {
				if !slices.Contains(st.Boxes, k) {
					delete(st.Seen, k)
				}
			}
		}
		st.Mode, st.Boxes, st.At, st.From = "", nil, 0, 0
	}
	return &Page{Records: recs, Cursor: cursorFor(st), More: more, Full: list}, nil
}

func icloudID(box string, valid, uid uint32) string { return fmt.Sprintf("%s/%d/%d", box, valid, uid) }

func icloudFetch(c *Client, ic *imapclient.Client, box string, valid uint32, uids []uint32) ([]Record, error) {
	if len(uids) == 0 {
		return nil, nil
	}
	set := new(imap.SeqSet)
	set.AddNum(uids...)
	sizes := map[uint32]uint32{}
	if err := icloudEach(ic, set, []imap.FetchItem{imap.FetchUid, imap.FetchRFC822Size}, func(m *imap.Message) {
		sizes[m.Uid] = m.Size
	}); err != nil {
		return nil, err
	}
	whole, head := new(imap.SeqSet), new(imap.SeqSet)
	for _, u := range uids {
		if sizes[u] <= icloudWhole {
			whole.AddNum(u)
		} else {
			head.AddNum(u)
		}
	}
	var recs []Record
	read := func(set *imap.SeqSet, sec *imap.BodySectionName) error {
		if set.Empty() {
			return nil
		}
		return icloudEach(ic, set, []imap.FetchItem{imap.FetchUid, imap.FetchFlags, imap.FetchInternalDate, sec.FetchItem()}, func(m *imap.Message) {
			var raw []byte
			if r := m.GetBody(sec); r != nil {
				raw, _ = io.ReadAll(r)
			}
			data := icloudMessage(c, raw, sizes[m.Uid] > icloudWhole)
			data["folder"], data["uid"] = box, m.Uid
			data["received"] = m.InternalDate.UTC().Format(time.RFC3339)
			data["labelIds"] = icloudLabels(box, m.Flags)
			recs = append(recs, Record{ID: icloudID(box, valid, m.Uid), Data: data, Updated: m.InternalDate})
		})
	}
	if err := read(whole, &imap.BodySectionName{Peek: true}); err != nil {
		return nil, err
	}
	if err := read(head, &imap.BodySectionName{Peek: true, BodyPartName: imap.BodyPartName{Specifier: imap.HeaderSpecifier}}); err != nil {
		return nil, err
	}
	return recs, nil
}

func icloudEach(ic *imapclient.Client, set *imap.SeqSet, items []imap.FetchItem, f func(*imap.Message)) error {
	ch := make(chan *imap.Message, 10)
	done := make(chan error, 1)
	go func() { done <- ic.UidFetch(set, items, ch) }()
	for m := range ch {
		f(m)
	}
	return <-done
}

func icloudLabels(box string, flags []string) []string {
	out := []string{box}
	if strings.EqualFold(box, "INBOX") {
		out[0] = "INBOX"
	}
	if !slices.Contains(flags, imap.SeenFlag) {
		out = append(out, "UNREAD")
	}
	if slices.Contains(flags, imap.FlaggedFlag) {
		out = append(out, "STARRED")
	}
	if slices.Contains(flags, imap.DraftFlag) {
		out = append(out, "DRAFT")
	}
	return out
}

func icloudMessage(c *Client, raw []byte, headOnly bool) map[string]any {
	out := map[string]any{"attachments": []any{}, "parties": []any{}}
	mr, err := gomail.CreateReader(bytes.NewReader(raw))
	if err != nil && mr == nil {
		return out
	}
	h := mr.Header
	out["subject"], _ = h.Subject()
	if t, err := h.Date(); err == nil && !t.IsZero() {
		out["sent"] = t.UTC().Format(time.RFC3339)
	}
	id, _ := h.MessageID()
	out["messageId"] = id
	thread := id
	if refs, _ := h.MsgIDList("References"); len(refs) > 0 {
		thread = refs[0]
	} else if irt, _ := h.MsgIDList("In-Reply-To"); len(irt) > 0 {
		thread = irt[0]
	}
	out["threadId"] = thread
	parties := []any{}
	for _, role := range []string{"from", "to", "cc", "bcc", "reply_to"} {
		as, _ := h.AddressList(strings.ReplaceAll(role, "_", "-"))
		for _, a := range as {
			parties = append(parties, map[string]any{"role": role, "name": a.Name, "address": a.Address})
		}
	}
	out["parties"] = parties
	if headOnly {
		out["text"] = "(This message is over 25 MB, so Archive kept only who it is from and to and its subject.)"
		return out
	}
	var plain, html strings.Builder
	atts := []any{}
	for {
		p, err := mr.NextPart()
		if err != nil {
			break
		}
		switch ph := p.Header.(type) {
		case *gomail.InlineHeader:
			kind, _, _ := ph.ContentType()
			b, _ := io.ReadAll(io.LimitReader(p.Body, 4*maxText))
			switch kind {
			case "text/plain":
				plain.Write(b)
			case "text/html":
				html.Write(b)
			}
		case *gomail.AttachmentHeader:
			name, _ := ph.Filename()
			kind, _, _ := ph.ContentType()
			if kind == "" {
				kind = mime.TypeByExtension(path.Ext(name))
			}
			b, _ := io.ReadAll(io.LimitReader(p.Body, icloudFile+1))
			a := map[string]any{"filename": name, "mimeType": kind, "size": len(b)}
			if len(b) <= icloudFile {
				a["text"] = bytesText(name, kind, b)
				if fid, err := c.keep(name, b); err == nil && fid != "" {
					a["file"] = fid
				}
			}
			atts = append(atts, a)
		}
	}
	words := plain.String()
	if strings.TrimSpace(words) == "" {
		words = htmlText(html.String())
	}
	out["text"] = clip(words, maxText)
	out["snippet"] = clip(strings.Join(strings.Fields(words), " "), 200)
	out["attachments"] = atts
	return out
}

type (
	icloudFind struct {
		Query  string `json:"query" jsonschema:"Words in the subject, sender or text"`
		Folder string `json:"folder,omitempty" jsonschema:"The folder, from folders; INBOX when left out"`
		Limit  int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 50; 20 when left out"`
	}
	icloudHit struct {
		ID      string `json:"id"`
		Folder  string `json:"folder"`
		Subject string `json:"subject,omitempty"`
		From    string `json:"from,omitempty"`
		To      string `json:"to,omitempty"`
		Date    string `json:"date,omitempty"`
		Unread  bool   `json:"unread,omitempty"`
	}
	icloudFound struct {
		Messages []icloudHit `json:"messages"`
	}
	icloudOneIn struct {
		ID string `json:"id" jsonschema:"The message's id, from search: <folder>/<uidvalidity>/<uid>"`
	}
	icloudAttachment struct {
		Filename string `json:"filename"`
		MIMEType string `json:"mimeType,omitempty"`
		Size     int    `json:"size"`
	}
	icloudMessageOut struct {
		ID          string             `json:"id"`
		Folder      string             `json:"folder"`
		Subject     string             `json:"subject,omitempty"`
		From        string             `json:"from,omitempty"`
		To          string             `json:"to,omitempty"`
		Cc          string             `json:"cc,omitempty"`
		Date        string             `json:"date,omitempty"`
		Text        string             `json:"text"`
		Attachments []icloudAttachment `json:"attachments"`
	}
	icloudFolderList struct {
		Folders []string `json:"folders"`
	}
)

func icloudAddrs(as []*imap.Address) string {
	var out []string
	for _, a := range as {
		out = append(out, strings.TrimSpace(a.PersonalName+" <"+a.Address()+">"))
	}
	return strings.Join(out, ", ")
}

func icloudSearch(c *Client, in icloudFind) (icloudFound, error) {
	out := icloudFound{Messages: []icloudHit{}}
	ic, err := icloudDial(c)
	if err != nil {
		return out, err
	}
	defer ic.Logout()
	box := cmp.Or(in.Folder, "INBOX")
	status, err := ic.Select(box, true)
	if err != nil {
		return out, fmt.Errorf("iCloud Mail has no folder %s: %w", box, err)
	}
	crit := imap.NewSearchCriteria()
	for _, w := range strings.Fields(in.Query) {
		crit.Or = append(crit.Or, [2]*imap.SearchCriteria{
			{Header: map[string][]string{"Subject": {w}}},
			{Or: [][2]*imap.SearchCriteria{{{Header: map[string][]string{"From": {w}}}, {Body: []string{w}}}}},
		})
	}
	uids, err := ic.UidSearch(crit)
	if err != nil {
		return out, err
	}
	slices.Sort(uids)
	slices.Reverse(uids)
	uids = uids[:min(len(uids), clamp(in.Limit, 20, 50))]
	if len(uids) == 0 {
		return out, nil
	}
	set := new(imap.SeqSet)
	set.AddNum(uids...)
	at := map[uint32]icloudHit{}
	err = icloudEach(ic, set, []imap.FetchItem{imap.FetchUid, imap.FetchFlags, imap.FetchEnvelope}, func(m *imap.Message) {
		h := icloudHit{ID: icloudID(box, status.UidValidity, m.Uid), Folder: box, Unread: !slices.Contains(m.Flags, imap.SeenFlag)}
		if e := m.Envelope; e != nil {
			h.Subject, h.From, h.To = e.Subject, icloudAddrs(e.From), icloudAddrs(e.To)
			if !e.Date.IsZero() {
				h.Date = e.Date.UTC().Format(time.RFC3339)
			}
		}
		at[m.Uid] = h
	})
	for _, u := range uids {
		if h, ok := at[u]; ok {
			out.Messages = append(out.Messages, h)
		}
	}
	return out, err
}

func icloudOne(c *Client, in icloudOneIn) (icloudMessageOut, error) {
	var out icloudMessageOut
	i := strings.LastIndex(in.ID, "/")
	j := strings.LastIndex(in.ID[:max(i, 0)], "/")
	if j < 0 {
		return out, errors.New("an id is <folder>/<uidvalidity>/<uid>, from search")
	}
	box, valid, uid := in.ID[:j], uint32(atoi(in.ID[j+1:i])), uint32(atoi(in.ID[i+1:]))
	ic, err := icloudDial(c)
	if err != nil {
		return out, err
	}
	defer ic.Logout()
	status, err := ic.Select(box, true)
	if err != nil {
		return out, fmt.Errorf("iCloud Mail has no folder %s: %w", box, err)
	}
	if status.UidValidity != valid {
		return out, fmt.Errorf("the folder %s was rebuilt since that id; search again", box)
	}
	set := new(imap.SeqSet)
	set.AddNum(uid)
	sec := &imap.BodySectionName{Peek: true}
	var raw []byte
	if err := icloudEach(ic, set, []imap.FetchItem{imap.FetchUid, sec.FetchItem()}, func(m *imap.Message) {
		if r := m.GetBody(sec); r != nil {
			raw, _ = io.ReadAll(io.LimitReader(r, icloudWhole))
		}
	}); err != nil {
		return out, err
	}
	if raw == nil {
		return out, icloudRefused(c, http.StatusNotFound, "no message "+in.ID)
	}
	data := icloudMessage(c, raw, false)
	out = icloudMessageOut{ID: in.ID, Folder: box, Subject: text(data["subject"]), Date: text(data["sent"]), Text: text(data["text"]), Attachments: []icloudAttachment{}}
	by := map[string][]string{}
	for _, p := range data["parties"].([]any) {
		m := p.(map[string]any)
		by[text(m["role"])] = append(by[text(m["role"])], strings.TrimSpace(text(m["name"])+" <"+text(m["address"])+">"))
	}
	out.From, out.To, out.Cc = strings.Join(by["from"], ", "), strings.Join(by["to"], ", "), strings.Join(by["cc"], ", ")
	for _, a := range data["attachments"].([]any) {
		m := a.(map[string]any)
		out.Attachments = append(out.Attachments, icloudAttachment{Filename: text(m["filename"]), MIMEType: text(m["mimeType"]), Size: atoi(text(m["size"]))})
	}
	return out, nil
}

func icloudFolders(c *Client, _ struct{}) (icloudFolderList, error) {
	ic, err := icloudDial(c)
	if err != nil {
		return icloudFolderList{}, err
	}
	defer ic.Logout()
	boxes, err := icloudBoxes(ic)
	return icloudFolderList{Folders: boxes}, err
}

type icloudHTTP struct{ c *Client }

func (h icloudHTTP) Do(r *http.Request) (*http.Response, error) {
	resp, err := h.c.http.Do(r.WithContext(h.c.ctx))
	if err == nil && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) {
		resp.Body.Close()
		return nil, icloudRefused(h.c, resp.StatusCode, "Apple turned down that Apple ID and app-specific password: check the Apple ID, and make a new app-specific password if that one was revoked or mistyped")
	}
	return resp, err
}

// icloudHome finds the calendar or address book home of the Apple ID, which is on a host of its own, like
// p42-caldav.icloud.com, and answers a DAV client of that host and the home's path.
func icloudHome(c *Client, base string, cal bool) (webdav.HTTPClient, string, error) {
	hc := icloudHTTP{c}
	wc, err := webdav.NewClient(hc, base)
	if err != nil {
		return nil, "", err
	}
	principal, err := wc.FindCurrentUserPrincipal(c.ctx)
	if err != nil {
		return nil, "", err
	}
	var home string
	if cal {
		cc, _ := caldav.NewClient(hc, base)
		home, err = cc.FindCalendarHomeSet(c.ctx, principal)
	} else {
		cc, _ := carddav.NewClient(hc, base)
		home, err = cc.FindAddressBookHomeSet(c.ctx, principal)
	}
	if err != nil {
		return nil, "", err
	}
	if u, err := url.Parse(home); err == nil && u.Host != "" {
		return &icloudHost{hc, u.Scheme + "://" + u.Host}, u.Path, nil
	}
	return hc, home, nil
}

type icloudHost struct {
	icloudHTTP
	host string
}

func (h *icloudHost) Do(r *http.Request) (*http.Response, error) {
	u, _ := url.Parse(h.host)
	r.URL.Scheme, r.URL.Host, r.Host = u.Scheme, u.Host, u.Host
	return h.icloudHTTP.Do(r)
}

type icloudDAVState struct {
	Parts []string          `json:"parts,omitempty"`
	Names map[string]string `json:"names,omitempty"`
	At    int               `json:"at,omitempty"`
	Skip  int               `json:"skip,omitempty"`
	Home  string            `json:"home,omitempty"`
	Host  string            `json:"host,omitempty"`
}

func icloudEvents(c *Client, cursor string) (*Page, error) {
	var st icloudDAVState
	if err := cursorOf(cursor, &st); err != nil {
		return nil, err
	}
	hc, home, err := icloudHome(c, icloudCalDAV, true)
	if err != nil {
		return nil, err
	}
	cc, err := caldav.NewClient(hc, icloudCalDAV)
	if err != nil {
		return nil, err
	}
	if len(st.Parts) == 0 {
		cals, err := cc.FindCalendars(c.ctx, home)
		if err != nil {
			return nil, err
		}
		st.Names = map[string]string{}
		for _, k := range cals {
			if len(k.SupportedComponentSet) == 0 || slices.Contains(k.SupportedComponentSet, "VEVENT") {
				st.Parts = append(st.Parts, k.Path)
				st.Names[k.Path] = cmp.Or(k.Name, path.Base(strings.TrimRight(k.Path, "/")))
			}
		}
		if len(st.Parts) == 0 {
			return &Page{Full: true}, nil
		}
	}
	cal := st.Parts[st.At]
	objs, err := cc.QueryCalendar(c.ctx, cal, &caldav.CalendarQuery{
		CompRequest: caldav.CalendarCompRequest{Name: "VCALENDAR", AllProps: true, AllComps: true},
		CompFilter:  caldav.CompFilter{Name: "VCALENDAR", Comps: []caldav.CompFilter{{Name: "VEVENT"}}},
	})
	if err != nil {
		return nil, err
	}
	calID := path.Base(strings.TrimRight(cal, "/"))
	var recs []Record
	for _, o := range objs {
		if o.Data == nil {
			continue
		}
		for _, ev := range o.Data.Events() {
			r := icloudEvent(&ev, calID, st.Names[cal])
			if r.ID != "" {
				recs = append(recs, r)
			}
		}
	}
	st.At++
	more := st.At < len(st.Parts)
	if !more {
		st = icloudDAVState{}
	}
	return &Page{Records: recs, Cursor: cursorFor(st), More: more, Full: true}, nil
}

func icloudWhen(p *ical.Prop) map[string]any {
	if p == nil {
		return nil
	}
	if p.ValueType() == ical.ValueDate {
		t, err := p.DateTime(time.UTC)
		if err != nil {
			return nil
		}
		return map[string]any{"date": t.Format("2006-01-02")}
	}
	t, err := p.DateTime(time.UTC)
	if err != nil {
		return nil
	}
	return map[string]any{"dateTime": t.Format(time.RFC3339)}
}

func icloudMailto(v string) string {
	if i := strings.Index(strings.ToLower(v), "mailto:"); i >= 0 {
		return v[i+len("mailto:"):]
	}
	return ""
}

func icloudEvent(ev *ical.Event, calID, calName string) Record {
	props := ev.Props
	val := func(k string) string {
		if p := props.Get(k); p != nil {
			s, _ := p.Text()
			return s
		}
		return ""
	}
	uid := val(ical.PropUID)
	if uid == "" {
		return Record{}
	}
	id := calID + "/" + uid
	data := map[string]any{"calendarId": calID, "calendar": calName, "uid": uid, "summary": val(ical.PropSummary),
		"description": val(ical.PropDescription), "location": val(ical.PropLocation), "url": val(ical.PropURL)}
	if p := props.Get(ical.PropRecurrenceID); p != nil {
		id += "/" + p.Value
		data["recurringEventId"] = uid
	}
	data["id"] = id
	data["start"] = icloudWhen(props.Get(ical.PropDateTimeStart))
	end := icloudWhen(props.Get(ical.PropDateTimeEnd))
	if end == nil {
		if s := props.Get(ical.PropDateTimeStart); s != nil {
			if t, err := s.DateTime(time.UTC); err == nil {
				if d := props.Get(ical.PropDuration); d != nil {
					if dur, err := d.Duration(); err == nil {
						end = map[string]any{"dateTime": t.Add(dur).Format(time.RFC3339)}
					}
				} else if s.ValueType() == ical.ValueDate {
					end = map[string]any{"date": t.AddDate(0, 0, 1).Format("2006-01-02")}
				}
			}
		}
	}
	data["end"] = end
	switch strings.ToUpper(val(ical.PropStatus)) {
	case "CANCELLED":
		data["status"] = "cancelled"
	case "TENTATIVE":
		data["status"] = "tentative"
	default:
		data["status"] = "confirmed"
	}
	var rules []any
	for _, k := range []string{ical.PropRecurrenceRule, ical.PropRecurrenceDates, ical.PropExceptionDates} {
		for _, p := range props.Values(k) {
			rules = append(rules, k+":"+p.Value)
		}
	}
	data["recurrence"] = rules
	if p := props.Get(ical.PropOrganizer); p != nil {
		data["organizer"] = map[string]any{"email": icloudMailto(p.Value), "displayName": p.Params.Get(ical.ParamCommonName)}
	}
	var people []any
	for _, p := range props.Values(ical.PropAttendee) {
		resp := strings.ToLower(p.Params.Get(ical.ParamParticipationStatus))
		people = append(people, map[string]any{"email": icloudMailto(p.Value), "displayName": p.Params.Get(ical.ParamCommonName), "responseStatus": resp,
			"optional": p.Params.Get(ical.ParamRole) == "OPT-PARTICIPANT", "resource": slices.Contains([]string{"RESOURCE", "ROOM"}, p.Params.Get(ical.ParamCalendarUserType))})
	}
	data["attendees"] = people
	var updated time.Time
	for _, k := range []string{ical.PropLastModified, ical.PropDateTimeStamp} {
		if p := props.Get(k); p != nil {
			if t, err := p.DateTime(time.UTC); err == nil {
				updated = t
				data["updated"] = t.Format(time.RFC3339)
				break
			}
		}
	}
	return Record{ID: id, Data: data, Updated: updated}
}

func icloudContacts(c *Client, cursor string) (*Page, error) {
	var st icloudDAVState
	if err := cursorOf(cursor, &st); err != nil {
		return nil, err
	}
	hc, home, err := icloudHome(c, icloudCardDAV, false)
	if err != nil {
		return nil, err
	}
	cc, err := carddav.NewClient(hc, icloudCardDAV)
	if err != nil {
		return nil, err
	}
	if len(st.Parts) == 0 {
		books, err := cc.FindAddressBooks(c.ctx, home)
		if err != nil {
			return nil, err
		}
		for _, b := range books {
			st.Parts = append(st.Parts, b.Path)
		}
		if len(st.Parts) == 0 {
			return &Page{Full: true}, nil
		}
	}
	book := st.Parts[st.At]
	wc, err := webdav.NewClient(hc, icloudCardDAV)
	if err != nil {
		return nil, err
	}
	files, err := wc.ReadDir(c.ctx, book, false)
	if err != nil {
		return nil, err
	}
	var cards []string
	for _, f := range files {
		if !f.IsDir && strings.HasSuffix(f.Path, ".vcf") {
			cards = append(cards, f.Path)
		}
	}
	slices.Sort(cards)
	chunk := cards[min(st.Skip, len(cards)):min(st.Skip+200, len(cards))]
	var recs []Record
	if len(chunk) > 0 {
		objs, err := cc.MultiGetAddressBook(c.ctx, book, &carddav.AddressBookMultiGet{Paths: chunk, DataRequest: carddav.AddressDataRequest{AllProp: true}})
		if err != nil {
			return nil, err
		}
		bookID := path.Base(strings.TrimRight(book, "/"))
		for _, o := range objs {
			if o.Card == nil || o.Card.Kind() == vcard.KindGroup || strings.EqualFold(o.Card.Value("X-ADDRESSBOOKSERVER-KIND"), "group") {
				continue
			}
			recs = append(recs, icloudContact(o.Card, bookID+"/"+strings.TrimSuffix(path.Base(o.Path), ".vcf")))
		}
	}
	st.Skip += 200
	if st.Skip >= len(cards) {
		st.At, st.Skip = st.At+1, 0
	}
	more := st.At < len(st.Parts)
	if !more {
		st = icloudDAVState{}
	}
	return &Page{Records: recs, Cursor: cursorFor(st), More: more, Full: true}, nil
}

func icloudContact(card vcard.Card, id string) Record {
	data := map[string]any{"name": card.PreferredValue(vcard.FieldFormattedName), "email": card.PreferredValue(vcard.FieldEmail),
		"phone": card.PreferredValue(vcard.FieldTelephone), "title": card.PreferredValue(vcard.FieldTitle)}
	if n := card.Name(); n != nil {
		data["first_name"], data["last_name"] = n.GivenName, n.FamilyName
	}
	if org := card.PreferredValue(vcard.FieldOrganization); org != "" {
		data["company"] = strings.TrimRight(strings.Split(org, ";")[0], " ")
	}
	if a := card.Address(); a != nil {
		data["city"], data["state"], data["country"] = a.Locality, a.Region, a.Country
	}
	var edited time.Time
	t, err := card.Revision()
	if err != nil {
		t, err = time.Parse(time.RFC3339, card.Value(vcard.FieldRevision))
	}
	if err == nil {
		edited = t
		data["edited"] = t.UTC().Format(time.RFC3339)
	}
	data["id"] = id
	return Record{ID: id, Data: data, Updated: edited}
}
