package connect

import (
	"time"

	"golang.org/x/oauth2"
)

// Outlook: the mail of a Microsoft 365 work or school account, or a personal Outlook.com one, read only through
// Microsoft Graph. It reads ARCHIVE_OUTLOOK, an app registration in Microsoft Entra for accounts in any
// directory and personal accounts, whose web redirect URIs list <API>/connect/outlook/callback, with the
// delegated Graph permissions User.Read, Mail.Read and Calendars.Read. offline_access asks for the refresh token, which
// Microsoft replaces on every refresh. There is no endpoint that revokes one grant: a person removes Archive
// at myapps.microsoft.com, or account.live.com/consent for a personal account.
//
// It syncs the mailbox into mail, as Gmail does, folder by folder with Graph's delta of each, which Graph keeps per
// folder, and the calendar into calendar_event. An account connected before it asked for Calendars.Read syncs its mail
// alone until it connects again.
//
// It pushes through a Graph subscription to every message of the mailbox, me/messages, and one to every event of
// the calendar, me/events, each with a clientState of Archive's own, for 10,020 minutes, under the 10,080 Outlook
// allows; the first sync a day after renews them with a PATCH, or makes another when Graph no longer has one. An
// account that did not allow Archive its calendar has the mail's alone. Graph checks the URL first by posting a
// validationToken, which Archive answers as it came, in plain text.
func init() {
	Register(&Connector{
		ID: "outlook", Name: "Outlook", Env: "ARCHIVE_OUTLOOK",
		Info: Info{By: "Microsoft", Description: "Read your Microsoft 365 or Outlook.com mail and calendar, never send or change them", Category: CatEmail, Icon: IconInbox,
			Scopes: []string{"Read your mail and its attachments", "Read your calendar"}},
		API: "https://graph.microsoft.com/v1.0",
		Endpoint: oauth2.Endpoint{AuthURL: "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
			TokenURL: "https://login.microsoftonline.com/common/oauth2/v2.0/token", AuthStyle: oauth2.AuthStyleInParams},
		Scopes: []string{"offline_access", "User.Read", "Mail.Read", "Calendars.Read"},
		Params: map[string]string{"prompt": "select_account"},
		Who:    &Who{Req: Req{URL: "/me?$select=mail,userPrincipalName"}, Names: []Path{"mail", "userPrincipalName"}, Lower: true},
		Actions: map[string]*Action{
			"search": Call[outlookFind, outlookFound](Plan{
				Doc: "Find messages, newest first, by words anywhere in them or by Outlook's search syntax, like { query: \"from:ann@example.com invoice\" }, " +
					"in every folder or one: { messages: [{ id, conversationId, subject, from, toRecipients, receivedDateTime, bodyPreview, hasAttachments, isRead, webLink }], next } (pass next as page for more)",
				Clamp: map[string][2]int{"limit": {25, 50}},
				Steps: []Step{
					{If: "query", As: "words", In: `"{"query}"`},
					{Unless: "query", As: "order", In: "receivedDateTime desc"},
					{If: "page", As: "got", Req: Req{URL: "{+page}"}},
					{Unless: "page", If: "folder", As: "got", Req: Req{URL: "/me/mailFolders/{folder}/messages", Query: outlookFinding}},
					{Unless: "page", Only: Test{At: "folder", Off: true}, As: "got", Req: Req{URL: "/me/messages", Query: outlookFinding}},
				},
				Out: Obj{"messages": "{got/value|[]}", "next": "{got/@odata.nextLink|}"},
			}),
			"message": Call[outlookOne, map[string]any](Plan{
				Doc: "One message by id with its body as text: { id, conversationId, subject, from, toRecipients, ccRecipients, receivedDateTime, " +
					"body: { contentType, content }, hasAttachments, attachments: [{ name, contentType, size }], webLink }",
				Steps: []Step{
					{As: "m", Req: Req{URL: "/me/messages/{id}?$select=" + outlookList + ",ccRecipients,body", Header: map[string][]string{"Prefer": {`outlook.body-content-type="text"`}}}},
					{If: "m/hasAttachments", As: "a", Req: Req{URL: "/me/messages/{id}/attachments?$select=name,contentType,size"}},
					{If: "a", As: "m/attachments", In: "{a/value}"},
				},
				Out: "{m}",
			}),
			"folders": Call[struct{}, []outlookFolder](Plan{
				Doc:   "The mail folders at the top of the mailbox: [{ id, displayName, totalItemCount, unreadItemCount, childFolderCount }]",
				Steps: []Step{{As: "got", Req: Req{URL: "/me/mailFolders?$top=100&$select=id,displayName,totalItemCount,unreadItemCount,childFolderCount"}}},
				Out:   "{got/value|[]}",
			}),
		},
		Listen: &Listen{Renew: 6 * 24 * time.Hour, Until: 10020 * time.Minute, Secret: true, Skip: []int{401, 403},
			Over:  &Parts{Fixed: []Obj{{"id": "me/messages"}, {"id": "me/events"}}},
			Watch: Req{Method: POST, URL: "/subscriptions", Body: Obj{"changeType": "created,updated,deleted", "notificationUrl": "{hook}", "resource": "{part}", "expirationDateTime": "{until}", "clientState": "{secret}"}},
			Key:   "{answer/id}", Resource: "{part}", Expires: "expirationDateTime",
			// A watch from before it named its resource is the mail's.
			OfPart:  &Reads{Of: "{resource|me/messages}"},
			Extend:  &Req{Method: PATCH, URL: "/subscriptions/{key}", Body: Obj{"expirationDateTime": "{until}"}},
			Unwatch: &Req{Method: DELETE, URL: "/subscriptions/{key}"},
			Hello:   []Hello{{Query: "validationToken"}},
			Kept:    &Kept{Each: "value", ID: "subscriptionId", Token: "clientState"},
		},
		Sync: []*Sync{
			// A message's text comes as text, not HTML, and 50 a page; a moved message is a new id in its new folder and gone
			// from its old one, so neither folder needs the other.
			{Name: "messages", Gap: time.Second, Parts: outlookFolders,
				Req:   Req{URL: "/me/mailFolders/{part}/messages/delta?$select=" + outlookSync, Header: map[string][]string{"Prefer": {"odata.maxpagesize=50", `outlook.body-content-type="text"`}}},
				Items: "value", Updated: "lastModifiedDateTime", Page: outlookPage, Change: outlookDelta, Gone: []Test{{At: "@removed"}}, Pass: []int{403, 404},
				Set: Obj{"folder": "{in/name}"},
				Hydrate: []Hydrate{
					Decode{Codec: Words, At: "body/content", As: "words", If: Test{At: "body/contentType", Is: "html"}},
					Fetch{If: Test{At: "hasAttachments", Is: "true"}, Req: Req{URL: "/me/messages/{id}/attachments?$select=id,name,contentType,size,isInline"}, As: "attachments", Par: 4, Gone: true},
					Download{Each: "attachments/value", If: Test{At: "@odata.type", Is: "#microsoft.graph.fileAttachment"}, Req: Req{URL: "/me/messages/{up/id}/attachments/{id}/$value"},
						Name: "{name}", Mime: "{contentType}", Size: "size", As: "text", Par: 4, Keep: true},
				},
				Doc: "Every message of every folder but Junk Email and Deleted Items, with its text, cut at 64 KB, its attachments' names, " +
					"each of them up to 25 MB kept as a file, and the words of each PDF, Word, Excel, PowerPoint or text one: " +
					"listed whole the first time, 50 a page and a folder at a time, and then each folder's delta since, messages added, changed, " +
					"moved or deleted. A folder made since is listed whole; one gone, or a delta Graph no longer keeps, lists the mailbox whole again.",
				To: Map{Table: "mail_message",
					Cols: map[string]SQL{"thread": "data->>'conversationId'", "subject": "data->>'subject'", "snippet": "data->>'bodyPreview'",
						"text":        "archive.sync_clip(coalesce(data->>'words', data#>>'{body,content}'), 65536)",
						"sent_at":     "coalesce(archive.sync_at(data->>'sentDateTime'), archive.sync_at(data->>'receivedDateTime'))",
						"received_at": "archive.sync_at(data->>'receivedDateTime')",
						"labels":      "jsonb_build_array(data->>'folder') || coalesce(data->'categories', '[]'::jsonb)"},
					Children: []Kids{
						{Table: "mail_party", From: "archive.sync_list(jsonb_build_array(data->'from')) p WHERE " + outlookHas, Cols: map[string]SQL{"role": "'from'", "name": outlookName}, People: outlookPerson},
						{Table: "mail_party", From: "archive.sync_list(data->'toRecipients') p WHERE " + outlookHas, Cols: map[string]SQL{"role": "'to'", "name": outlookName}, People: outlookPerson},
						{Table: "mail_party", From: "archive.sync_list(data->'ccRecipients') p WHERE " + outlookHas, Cols: map[string]SQL{"role": "'cc'", "name": outlookName}, People: outlookPerson},
						{Table: "mail_party", From: "archive.sync_list(data->'bccRecipients') p WHERE " + outlookHas, Cols: map[string]SQL{"role": "'bcc'", "name": outlookName}, People: outlookPerson},
						{Table: "mail_party", From: "archive.sync_list(data->'replyTo') p WHERE " + outlookHas, Cols: map[string]SQL{"role": "'reply_to'", "name": outlookName}, People: outlookPerson},
						{Table: "mail_attachment", From: "archive.sync_list(data#>'{attachments,value}') f",
							Cols: map[string]SQL{"filename": "f->>'name'", "mime": "f->>'contentType'", "size": "archive.sync_num(f->>'size')", "text": "f->>'text'", "file": "f->>'file'"}},
					}}},
			// The calendar's view from a year back to a year ahead is one part named by the month it starts in, so a new month
			// is a part gone, and lists the view whole again over the moved window. Times come in UTC.
			{Name: "events", Gap: time.Second,
				Parts: &Parts{Req: Req{URL: "/me/calendar?$select=id"},
					Under: &Parts{Span: &Span{Month: true, Last: Shift{1, 1, 0}, First: Shift{-2, -1, 0}, Step: Shift{2, 1, 1}}, ID: "calendar {start}", Vals: Obj{"start": "{start}", "end": "{end}"}}},
				Req: Req{URL: "/me/calendarView/delta", Query: Q{"startDateTime": "{in/start}T00:00:00Z", "endDateTime": "{in/end}T00:00:00Z"},
					Header: map[string][]string{"Prefer": {"odata.maxpagesize=50", `outlook.timezone="UTC"`, `outlook.body-content-type="text"`}}},
				Items: "value", Updated: "lastModifiedDateTime", Page: outlookPage, Change: outlookDelta, Gone: []Test{{At: "@removed"}}, Pass: []int{403, 404},
				// An account that did not allow Archive its calendar has none to walk until it connects again.
				Deny: []Deny{{Status: []int{401, 403}, Why: "Outlook did not allow Archive the calendar; connect Outlook again to sync it"}},
				Doc: "Every event of your calendar from a year back to a year ahead, each occurrence of a series on its own, with its attendees " +
					"and their answers and its text, cut at 64 KB: listed whole, 50 a page, and then Graph's delta since; the window moves a " +
					"month at a time, which lists it whole again.",
				To: Map{Table: "calendar_event",
					Cols: map[string]SQL{"calendar": "'Calendar'", "title": "data->>'subject'", "description": "archive.sync_clip(data#>>'{body,content}', 65536)", "location": "data#>>'{location,displayName}'",
						"starts_at": "archive.sync_at(left(data#>>'{start,dateTime}', 19) || 'Z')", "ends_at": "archive.sync_at(left(data#>>'{end,dateTime}', 19) || 'Z')", "all_day": "archive.sync_bool(data->'isAllDay')",
						"status": "CASE WHEN data->>'isCancelled' = 'true' THEN 'cancelled' WHEN data->>'showAs' = 'tentative' THEN 'tentative' ELSE 'confirmed' END",
						"recurrence": `CASE WHEN nullif(data#>>'{recurrence,pattern,type}', '') IS NOT NULL THEN concat(data#>>'{recurrence,pattern,type}', ', every ', coalesce(nullif(data#>>'{recurrence,pattern,interval}', ''), '1'),
							(SELECT ' on ' || string_agg(d#>>'{}', ', ') FROM archive.sync_list(data#>'{recurrence,pattern,daysOfWeek}') d)) END`,
						"organizer_name": "data#>>'{organizer,emailAddress,name}'", "url": "data->>'webLink'", "meeting_url": "data#>>'{onlineMeeting,joinUrl}'", "edited_at": "archive.sync_at(data->>'lastModifiedDateTime')"},
					People: map[string]SQL{"organizer": "data#>>'{organizer,emailAddress,address}'"},
					Refs:   map[string]SQL{"series": "data->>'seriesMasterId'"},
					Children: []Kids{{Table: "calendar_attendee", From: "archive.sync_list(data->'attendees') a WHERE archive.sync_email(a#>>'{emailAddress,address}') IS NOT NULL",
						Cols: map[string]SQL{"name": "a#>>'{emailAddress,name}'", "role": "a->>'type'",
							"response": "CASE a#>>'{status,response}' WHEN 'accepted' THEN 'accepted' WHEN 'declined' THEN 'declined' WHEN 'tentativelyAccepted' THEN 'tentative' WHEN 'organizer' THEN 'accepted' ELSE 'none' END"},
						People: map[string]SQL{"person": "a#>>'{emailAddress,address}'"}}}}},
		},
	})
}

const (
	outlookList = "id,conversationId,subject,from,toRecipients,receivedDateTime,bodyPreview,hasAttachments,isRead,webLink"
	// outlookSync is what the messages stream reads of a message.
	outlookSync = "id,conversationId,subject,from,toRecipients,ccRecipients,bccRecipients,replyTo,receivedDateTime,sentDateTime," +
		"bodyPreview,body,hasAttachments,categories,isDraft,lastModifiedDateTime"
	outlookTree     = "?$top=100&$select=id,displayName,childFolderCount"
	outlookHas      = "archive.sync_email(p#>>'{emailAddress,address}') IS NOT NULL"
	outlookName SQL = "p#>>'{emailAddress,name}'"
)

var (
	outlookPage    = NextURL{At: "@odata.nextLink"}
	outlookPerson  = map[string]SQL{"person": "p#>>'{emailAddress,address}'"}
	outlookFinding = Q{"$top": "{limit}", "$select": outlookList, "$search": "{words}", "$orderby": "{order}"}
	// outlookDelta is Graph's delta of a part, kept per part. Graph answers 410 Gone, or 400 for a sync state it no longer keeps.
	outlookDelta = Resume{At: "@odata.deltaLink", Expired: []int{410}, Stale: []Deny{{Status: []int{400}, Says: []string{"SyncState", "resyncRequired", "sync state"}}}}
	// outlookFolders are every mail folder of the mailbox, the folders in folders too, but Junk Email and Deleted Items
	// and what is in them, as Gmail leaves out spam and trash.
	outlookFolders = &Parts{Req: Req{URL: "/me/mailFolders" + outlookTree}, Items: "value", Page: outlookPage, Vals: Obj{"name": "{displayName}"},
		Not:    []Req{{URL: "/me/mailFolders/junkemail?$select=id"}, {URL: "/me/mailFolders/deleteditems?$select=id"}},
		Deep:   &Req{URL: "/me/mailFolders/{id}/childFolders" + outlookTree},
		DeepIf: Test{At: "childFolderCount", Re: "^[1-9]"}}
)

type (
	outlookFind struct {
		Query  string `json:"query,omitempty" jsonschema:"Words or KQL like from:ann subject:invoice received>=2026-01-01; every message when left out"`
		Folder string `json:"folder,omitempty" jsonschema:"A folder's id from folders, or a well-known name like inbox, sentitems or archive"`
		Limit  int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 50; 25 when left out"`
		Page   string `json:"page,omitempty" jsonschema:"The next of the search before, for the messages after it"`
	}
	outlookOne struct {
		ID string `json:"id" jsonschema:"The message's id, from search"`
	}
	outlookFound struct {
		Messages []map[string]any `json:"messages"`
		Next     string           `json:"next" jsonschema:"Pass as page for more; empty when there is no more"`
	}
	outlookFolder struct {
		ID               string `json:"id"`
		DisplayName      string `json:"displayName"`
		TotalItemCount   int    `json:"totalItemCount,omitempty"`
		UnreadItemCount  int    `json:"unreadItemCount,omitempty"`
		ChildFolderCount int    `json:"childFolderCount,omitempty"`
	}
)
