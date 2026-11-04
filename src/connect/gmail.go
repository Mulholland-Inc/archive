package connect

import (
	"time"

	"golang.org/x/oauth2"
)

// Gmail. Its Google client is the one sign-in uses too: its authorized redirect URIs list <API>/connect/gmail/callback,
// and its consent screen asks for gmail.modify, a restricted scope: reading, sending, drafts and labels, but never
// deleting for good. Google sends a refresh token only with access_type=offline, and again on a later connect only
// with prompt=consent. Its pushes come through the Pub/Sub topic its push settings name, which gmail-api-push@
// system.gserviceaccount.com publishes to and a push subscription delivers to Archive; a watch lasts 7 days, and
// Google asks that it be renewed daily. A push's message is { emailAddress, historyId }, the mailbox it is for.
func init() {
	Register(&Connector{
		ID: "gmail", Name: "Gmail", Env: "ARCHIVE_GOOGLE", Options: "which sign-in with Google reads too",
		Info: Info{By: "Google", Description: "Read your mail and attachments, and send, draft or label mail when you ask", Category: CatEmail, Icon: IconMail,
			Scopes: []string{"Read your mail and its attachments", "Send, draft and label mail when you ask"}},
		API: "https://gmail.googleapis.com/gmail/v1/users/me",
		// Every mailbox's requests together, as this is the server's; each mailbox's own limit is its stream's Gap.
		Pace: 200 * time.Millisecond,
		// Gmail turns a user away for now with a 403 that says so.
		Busy: []string{"Quota exceeded", "rateLimitExceeded", "userRateLimitExceeded", "Rate Limit Exceeded"},
		Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.google.com/o/oauth2/v2/auth", TokenURL: "https://oauth2.googleapis.com/token",
			AuthStyle: oauth2.AuthStyleInParams},
		Revoke: "https://oauth2.googleapis.com/revoke",
		Scopes: []string{"https://www.googleapis.com/auth/gmail.modify"},
		Params: map[string]string{"access_type": "offline", "prompt": "consent"},
		Who:    &Who{Req: Req{URL: "/profile"}, Names: []Path{"emailAddress"}, Lower: true},
		Actions: map[string]*Action{
			"search": Call[gmailFind, gmailFound](Plan{
				Doc:   "Find messages as Gmail's search box does, like { query: \"from:ann@example.com newer_than:7d\" }, newest first (pass next as page for more)",
				Clamp: map[string][2]int{"limit": {20, 50}},
				Steps: []Step{
					{As: "list", Req: Req{URL: "/messages", Query: Q{"q": "{query}", "maxResults": "{limit}", "pageToken": "{page}"}}},
					{As: "found", Each: "list/messages", Codec: MIMEHead, Req: Req{URL: "/messages/{id}?format=metadata&metadataHeaders=From&metadataHeaders=To&metadataHeaders=Subject&metadataHeaders=Date"}},
				},
				Out: Obj{"messages": "{found|[]}", "next": "{list/nextPageToken|}"},
			}),
			"message": Call[gmailOne, gmailMessage](Plan{
				Doc:   "One message by id, with its text",
				Steps: []Step{{Codec: MIME, Req: Req{URL: "/messages/{id}?format=full"}}},
			}),
			"thread": Call[gmailOne, gmailThread](Plan{
				Doc: "A conversation by its threadId, every message with its text, oldest first",
				Steps: []Step{
					{As: "thread", Req: Req{URL: "/threads/{id}?format=full"}},
					{As: "messages", Each: "thread/messages", Codec: MIME},
				},
				Out: Obj{"id": "{thread/id}", "messages": "{messages}"},
			}),
			"send": Call[gmailNew, gmailSent](Plan{
				Doc: "Send a message from your Gmail, maybe as a reply that joins its thread",
				Steps: []Step{gmailParent, gmailCompose,
					{As: "sent", Req: Req{Method: POST, URL: "/messages/send", Body: Obj{"raw": "{mail/raw}", "threadId": "{parent/threadId}"}}}},
				Out: Obj{"id": "{sent/id}", "threadId": "{sent/threadId}"},
			}),
			"draft": Call[gmailNew, gmailDraft](Plan{
				Doc: "Save a message to your Gmail drafts without sending it, maybe as a reply in its thread",
				Steps: []Step{gmailParent, gmailCompose,
					{Req: Req{Method: POST, URL: "/drafts", Body: Obj{"message": Obj{"raw": "{mail/raw}", "threadId": "{parent/threadId}"}}}}},
			}),
			"label": Call[gmailRelabel, gmailLabeled](Plan{
				Doc:   "Add and remove a message's labels: remove INBOX to archive it, UNREAD to mark it read, add STARRED or TRASH, or a label of yours by its id from a message's labelIds",
				Steps: []Step{{Req: Req{Method: POST, URL: "/messages/{id}/modify", Body: Obj{"addLabelIds": "{add}", "removeLabelIds": "{remove}"}}}},
			}),
		},
		Listen: &Listen{Env: "ARCHIVE_GMAIL_TOPIC", OIDC: true, Renew: 6 * 24 * time.Hour,
			Watch: Req{Method: POST, URL: "/watch", Body: Obj{"topicName": "{target}"}}, Key: "{account}", Expires: "expiration",
			Unwatch: &Req{Method: POST, URL: "/stop"},
			Wrap:    "message/data", Codec: Base64JSON, Keys: []Path{"emailAddress"}, Lower: true},
		Sync: []*Sync{{Name: "messages", Gap: 50 * time.Second,
			// Gmail allows each user 6,000 quota units a minute, and format=full costs 60 a message, so a page of 50
			// costs about 3,005 and takes 10 seconds at Pace. A page a minute keeps a mailbox's sync at about half the
			// limit, which leaves the rest to actions, and reads a 5,000-message mailbox in about 100 minutes.
			Req: Req{URL: "/messages", Query: Q{"maxResults": "50", "pageToken": "{page}"}}, Items: "messages", Absent: "resultSizeEstimate", Page: Cursor{At: "nextPageToken"},
			Change: Token{Start: Req{URL: "/profile"}, At: "historyId", Expired: []int{404},
				Req:  Req{URL: "/history?historyTypes=messageAdded&historyTypes=messageDeleted&historyTypes=labelAdded&historyTypes=labelRemoved", Query: Q{"startHistoryId": "{token}", "maxResults": "100", "pageToken": "{page}"}},
				Each: "history", Live: []Path{"messagesAdded/*/message/id", "labelsAdded/*/message/id", "labelsRemoved/*/message/id"}, Gone: []Path{"messagesDeleted/*/message/id"}},
			Hydrate: []Hydrate{
				Fetch{Req: Req{URL: "/messages/{id}?format=full"}, Par: 5, Gone: true},
				Decode{Codec: MIMEWords},
				// What fits in the 5 MB a Client reads, as Gmail sends it in base64.
				Files{Each: "attachments", Req: Req{URL: "/messages/{id}/attachments/{attachmentId}"}, Par: 5, Most: 3 << 20,
					Has: "attachmentId", Name: "filename", Mime: "mimeType", Size: "size", Data: "data", As: "text"},
			},
			Doc: "Every message but spam and trash, each as the message action returns it without its HTML, its text cut at 64 KB, " +
				"with the words of each PDF, Word, Excel, PowerPoint or text attachment up to 3 MB: " +
				"listed whole the first time, 50 a page, and then the mailbox's history since, messages added, deleted or relabeled. " +
				"A history Gmail no longer keeps, about a week's, lists the mailbox whole again.",
			To: Map{Table: "mail_message",
				Cols: map[string]SQL{"thread": "data->>'threadId'", "subject": "data->>'subject'", "snippet": "data->>'snippet'", "text": "data->>'text'",
					"sent_at": "archive.sync_at(coalesce(data->>'sent', data->>'internalDate'))", "received_at": "archive.sync_at(data->>'internalDate')",
					"labels": "coalesce(data->'labelIds', '[]'::jsonb)"},
				Children: []Kids{
					{Table: "mail_party", From: "jsonb_array_elements(data->'parties') p",
						Cols: map[string]SQL{"role": "p->>'role'", "name": "p->>'name'"}, People: map[string]SQL{"person": "p->>'address'"}},
					{Table: "mail_attachment", From: "jsonb_array_elements(data->'attachments') f",
						Cols: map[string]SQL{"filename": "f->>'filename'", "mime": "f->>'mimeType'", "size": "archive.sync_num(f->>'size')", "text": "f->>'text'", "file": "f->>'file'"}},
				}}}},
	})
}

var (
	gmailParent  = Step{If: "reply", As: "parent", Req: Req{URL: "/messages/{reply}?format=metadata&metadataHeaders=Message-ID&metadataHeaders=References&metadataHeaders=Subject&metadataHeaders=From&metadataHeaders=Reply-To"}}
	gmailCompose = Step{As: "mail", Codec: RFC5322, In: Obj{"to": "{to}", "cc": "{cc}", "bcc": "{bcc}", "subject": "{subject}", "text": "{text}", "html": "{html}", "parent": "{parent}"}}
)

type (
	gmailFind struct {
		Query string `json:"query" jsonschema:"Gmail search, like from:ann has:attachment after:2026/01/01"`
		Limit int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 50; 20 when left out"`
		Page  string `json:"page,omitempty" jsonschema:"The next of the search before, for the messages after it"`
	}
	gmailOne struct {
		ID string `json:"id" jsonschema:"The id, from search, or a message's threadId"`
	}
	gmailNew struct {
		To      string `json:"to,omitempty" jsonschema:"Who it goes to, like Ann <ann@example.com>, bob@example.com; a reply's sender when left out"`
		Cc      string `json:"cc,omitempty" jsonschema:"Who gets a copy"`
		Bcc     string `json:"bcc,omitempty" jsonschema:"Who gets a copy the others don't see"`
		Subject string `json:"subject,omitempty" jsonschema:"The subject; a reply's is Re: and its message's when left out"`
		Text    string `json:"text" jsonschema:"The body, as plain text"`
		HTML    string `json:"html,omitempty" jsonschema:"The body as HTML too, which mail apps show in place of the text"`
		Reply   string `json:"reply,omitempty" jsonschema:"The id of the message this answers, from search; it joins that message's thread"`
	}
	gmailRelabel struct {
		ID     string   `json:"id" jsonschema:"The message's id, from search"`
		Add    []string `json:"add,omitempty" jsonschema:"Label ids to add, like STARRED"`
		Remove []string `json:"remove,omitempty" jsonschema:"Label ids to remove, like INBOX or UNREAD"`
	}
	gmailPreview struct {
		ID       string   `json:"id"`
		ThreadID string   `json:"threadId"`
		From     string   `json:"from,omitempty"`
		To       string   `json:"to,omitempty"`
		Subject  string   `json:"subject,omitempty"`
		Date     string   `json:"date,omitempty"`
		Snippet  string   `json:"snippet,omitempty"`
		LabelIDs []string `json:"labelIds,omitempty"`
	}
	gmailFound struct {
		Messages []gmailPreview `json:"messages"`
		Next     string         `json:"next" jsonschema:"Pass as page for more; empty when there is no more"`
	}
	gmailAttachment struct {
		Filename string `json:"filename"`
		MIMEType string `json:"mimeType"`
		Size     int    `json:"size"`
	}
	gmailMessage struct {
		gmailPreview
		Cc          string            `json:"cc,omitempty"`
		Text        string            `json:"text"`
		HTML        string            `json:"html"`
		Attachments []gmailAttachment `json:"attachments"`
	}
	gmailThread struct {
		ID       string         `json:"id"`
		Messages []gmailMessage `json:"messages"`
	}
	gmailSent struct {
		ID       string `json:"id"`
		ThreadID string `json:"threadId"`
	}
	gmailDraft struct {
		ID      string    `json:"id"`
		Message gmailSent `json:"message"`
	}
	gmailLabeled struct {
		ID       string   `json:"id"`
		ThreadID string   `json:"threadId"`
		LabelIDs []string `json:"labelIds,omitempty"`
	}
)
