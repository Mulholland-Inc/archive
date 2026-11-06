package connect

import (
	"time"

	"golang.org/x/oauth2"
)

// DocuSign's eSignature REST API v2.1: a person's envelopes, the agreements they send or are sent for signing. It
// reads ARCHIVE_DOCUSIGN, an app's integration key and secret key from DocuSign's Apps and Keys page, whose
// redirect URIs list <API>/connect/docusign/callback, with the authorization code grant. A key starts in the
// developer environment, account-d.docusign.com, which https://<key>:<secret>@account-d.docusign.com connects
// to; DocuSign's Go-Live review, once the key has made 20 calls there, moves it to production. The scope signature
// reads and writes envelopes, and extended keeps the refresh token alive while it is used, where it would end after
// 30 days. DocuSign has nowhere to revoke a grant: a person removes Archive in their DocuSign profile. Each
// account has its own host, which the user info of a token names; a person in several connects their default one.
//
// It syncs every envelope into sign_envelope, with each recipient in sign_recipient, and the words of a completed
// envelope's documents as signed: listed whole the first time and a week after, 100 a page, which deletes those
// gone, and in between those whose status changed since the sync before. DocuSign asks that nothing ask about an
// envelope more than once in 15 minutes, so a sync that caught up waits that long before reading again. It
// pushes nothing: DocuSign Connect is set up per account by an admin.
func init() {
	Register(&Connector{
		ID: "docusign", Name: "DocuSign",
		Info: Info{By: "DocuSign", Description: "Read your envelopes, who signed and the signed documents, and draft, send or void envelopes when you ask", Category: CatDocs, Icon: IconNotebook,
			Scopes: []string{"Read your envelopes and their documents", "Draft, send and void envelopes when you ask"}},
		Hosts: []string{"https://account.docusign.com"},
		API:   "{+acct/0}",
		Let: []Step{
			{As: "who", Req: Req{URL: "https://account.docusign.com/oauth/userinfo"}},
			{As: "acct", Each: "who/accounts", Only: Test{At: "is_default", Is: "true"}, In: "{base_uri}/restapi/v2.1/accounts/{account_id}"},
			{Unless: "acct", Fail: "DocuSign named no default account for this person"},
		},
		Busy: []string{"HOURLY_APIINVOCATION_LIMIT_EXCEEDED", "BURST_APIINVOCATION_LIMIT_EXCEEDED"},
		Endpoint: oauth2.Endpoint{AuthURL: "https://account.docusign.com/oauth/auth", TokenURL: "https://account.docusign.com/oauth/token",
			AuthStyle: oauth2.AuthStyleInHeader},
		Scopes: []string{"signature", "extended"},
		Who:    &Who{Req: Req{URL: "https://account.docusign.com/oauth/userinfo"}, Names: []Path{"email"}, Lower: true},
		Actions: map[string]*Action{
			"search": Call[dsFind, dsFound](Plan{
				Doc: "Find envelopes by words in their subject, sender, recipients or documents, maybe only of a status or since a date, newest change first: " +
					"{ envelopes: [{ envelopeId, emailSubject, status, sender, sentDateTime, completedDateTime, recipients }], next } (pass next as page for more)",
				Clamp: map[string][2]int{"limit": {25, 100}},
				Enum:  map[string][]string{"status": {"created", "sent", "delivered", "completed", "declined", "voided"}},
				Steps: []Step{
					{As: "got", Req: Req{URL: "/envelopes", Query: Q{"from_date": "{since|2000-01-01}", "search_text": "{query}", "status": "{status}", "include": "recipients",
						"order_by": "last_modified", "order": "desc", "count": "{limit}", "start_position": "{page}"}}},
					{If: "got/nextUri", As: "next", Codec: Sum, In: []any{"{page|0}", "{limit}"}},
				},
				Out: Obj{"envelopes": "{got/envelopes|[]}", "next": "{next}"},
			}),
			"get": Call[dsOne, map[string]any](Plan{
				Doc:   "One envelope by id, with its recipients and where each is, and its documents: { envelopeId, emailSubject, status, sender, recipients, envelopeDocuments, ... }",
				Steps: []Step{{Req: Req{URL: "/envelopes/{id}", Query: Q{"include": "recipients,documents"}}}},
			}),
			"document": Func("The words of an envelope's documents, all of them together or one by its documentId from get, as they stand: signed once it is completed. "+
				"Returns { id, text }", dsDocument),
			"create": Call[dsNew, dsMade](Plan{
				Doc: "Draft an envelope of a document, as HTML or as a file in base64, for signers and people copied, which goes to no one until send; " +
					"with send it goes now. With no fields placed, each signer places their own signature. Returns { envelopeId, status, statusDateTime }",
				Steps: []Step{
					{If: "base64", Unless: "extension", Fail: "a document in base64 needs its extension, like pdf or docx"},
					{Unless: "html", Only: Test{At: "base64", Off: true}, Fail: "an envelope needs a document, as html or base64"},
					{As: "status", In: "created"},
					{If: "send", As: "status", In: "sent"},
					{If: "html", As: "doc", In: Obj{"documentId": "1", "name": "{name}", "htmlDefinition": Obj{"source": "{html}"}}},
					{Unless: "html", As: "doc", In: Obj{"documentId": "1", "name": "{name}", "fileExtension": "{extension}", "documentBase64": "{base64}"}},
					{Req: Req{Method: POST, URL: "/envelopes", Body: Obj{"emailSubject": "{subject}", "emailBlurb": "{message}", "status": "{status}", "documents": []any{"{doc}"},
						"recipients": Obj{"signers": "{signers}", "carbonCopies": "{cc}"}}}},
				},
			}),
			"send": Call[dsOne, dsMade](Plan{
				Doc:   "Send a draft envelope to its recipients: { envelopeId, status }",
				Steps: []Step{{Req: Req{Method: PUT, URL: "/envelopes/{id}", Body: Obj{"status": "sent"}}}},
				Out:   Obj{"envelopeId": "{id}", "status": "sent"},
			}),
			"void": Call[dsVoid, dsMade](Plan{
				Doc:   "Void an envelope that was sent and is not yet completed, telling its recipients why; DocuSign keeps it, voided: { envelopeId, status }",
				Steps: []Step{{Req: Req{Method: PUT, URL: "/envelopes/{id}", Body: Obj{"status": "voided", "voidedReason": "{reason}"}}}},
				Out:   Obj{"envelopeId": "{id}", "status": "voided"},
			}),
		},
		Sync: []*Sync{{Name: "envelopes", Gap: 2 * time.Second, Every: 15 * time.Minute,
			Req:   Req{URL: "/envelopes", Query: Q{"from_date": "2000-01-01", "include": "recipients,documents", "count": "100", "start_position": "{page}"}},
			Items: "envelopes", Absent: "resultSetSize", ID: "envelopeId", Updated: "lastModifiedDateTime", Page: Offset{Size: 100, Step: 100}, Memo: "{lastModifiedDateTime}",
			Change: SinceTime{Req: Req{URL: "/envelopes", Query: Q{"from_date": "{since}", "include": "recipients,documents", "count": "100", "start_position": "{page}"}}},
			Hydrate: []Hydrate{Download{If: Test{At: "status", Is: "completed"}, Req: Req{URL: "/envelopes/{envelopeId}/documents/combined", Query: Q{"certificate": "false"}},
				Name: "{envelopeId}.pdf", Mime: "application/pdf", As: "text", Par: 2}},
			Doc: "Every envelope you sent or were sent, drafts too: its subject, status, sender, documents' names and when it was made, sent, completed or " +
				"voided, each recipient with their role, place in the order, status and when they signed, and, once completed, the words of its documents " +
				"as signed, cut at 64 KB. Listed whole the first time and a week after, 100 a page, which deletes those gone, and in between those whose " +
				"status changed since the sync before.",
			To: Map{Table: "sign_envelope",
				Cols: map[string]SQL{"title": "data->>'emailSubject'", "status": "data->>'status'", "sender_name": "data#>>'{sender,userName}'",
					"documents": "(SELECT array_agg(d->>'name') FROM archive.sync_list(data->'envelopeDocuments') d WHERE d->>'type' IS DISTINCT FROM 'summary')",
					"made_at":   "archive.sync_at(data->>'createdDateTime')", "sent_at": "archive.sync_at(data->>'sentDateTime')",
					"completed_at": "archive.sync_at(data->>'completedDateTime')", "voided_at": "archive.sync_at(data->>'voidedDateTime')",
					"edited_at": "archive.sync_at(data->>'lastModifiedDateTime')", "text": "data->>'text'"},
				People: map[string]SQL{"sender": "data#>>'{sender,email}'"},
				// Recipients come in a list of each kind, like signers and carbonCopies.
				Children: []Kids{{Table: "sign_recipient",
					From: "jsonb_each(CASE WHEN jsonb_typeof(data->'recipients') = 'object' THEN data->'recipients' ELSE '{}' END) k, archive.sync_list(k.value) r " +
						"WHERE archive.sync_email(r->>'email') IS NOT NULL",
					Cols: map[string]SQL{"name": "r->>'name'", "status": "r->>'status'", "routing": "archive.sync_num(r->>'routingOrder')::int",
						"role": "CASE k.key WHEN 'signers' THEN 'signer' WHEN 'carbonCopies' THEN 'cc' WHEN 'certifiedDeliveries' THEN 'certified delivery' " +
							"WHEN 'inPersonSigners' THEN 'in person signer' ELSE regexp_replace(k.key, 's$', '') END",
						"signed_at": "archive.sync_at(r->>'signedDateTime')"},
					People: map[string]SQL{"person": "r->>'email'"}}}}}},
	})
}

// dsDocument reads the words of an envelope's documents, or of one of them.
func dsDocument(c *Client, in dsDoc) (dsText, error) {
	sc := map[string]any{"id": in.ID}
	if in.Document != "" {
		sc["document"] = in.Document
	}
	words, _, err := c.download(Download{Req: Req{URL: "/envelopes/{id}/documents/{document|combined}", Query: Q{"certificate": "false"}},
		Name: "document.pdf", Mime: "application/pdf"}, sc)
	if err != nil {
		return dsText{}, err
	}
	return dsText{ID: in.ID, Text: words}, nil
}

type (
	dsFind struct {
		Query  string `json:"query,omitempty" jsonschema:"Words in the subject, sender, recipients or documents"`
		Status string `json:"status,omitempty" jsonschema:"Only envelopes of this status"`
		Since  string `json:"since,omitempty" jsonschema:"Only envelopes whose status changed since this date, like 2026-01-01"`
		Limit  int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 100; 25 when left out"`
		Page   int    `json:"page,omitempty" jsonschema:"The next of the search before, for the envelopes after it"`
	}
	dsOne struct {
		ID string `json:"id" jsonschema:"The envelope's id, an envelopeId from search"`
	}
	dsDoc struct {
		ID       string `json:"id" jsonschema:"The envelope's id"`
		Document string `json:"document,omitempty" jsonschema:"One document's documentId, from get; all of them together when left out"`
	}
	dsVoid struct {
		ID     string `json:"id" jsonschema:"The envelope's id"`
		Reason string `json:"reason" jsonschema:"Why it is voided, which its recipients are told"`
	}
	dsWho struct {
		Name         string `json:"name" jsonschema:"Their name"`
		Email        string `json:"email" jsonschema:"Their email address"`
		RecipientID  string `json:"recipientId" jsonschema:"A number unique among the envelope's recipients, like 1, 2 or 3"`
		RoutingOrder string `json:"routingOrder,omitempty" jsonschema:"Their place in the order it goes round, from 1; together with those of the same number; 1 when left out"`
	}
	dsNew struct {
		Subject   string  `json:"subject" jsonschema:"The subject of the email recipients get, and the envelope's name"`
		Message   string  `json:"message,omitempty" jsonschema:"The email's message"`
		Name      string  `json:"name" jsonschema:"The document's name, like Engagement letter"`
		HTML      string  `json:"html,omitempty" jsonschema:"The document, as HTML"`
		Base64    string  `json:"base64,omitempty" jsonschema:"The document instead as a file, in base64, like a PDF or Word file"`
		Extension string  `json:"extension,omitempty" jsonschema:"The file's extension, like pdf or docx, with base64"`
		Signers   []dsWho `json:"signers" jsonschema:"Who signs"`
		CC        []dsWho `json:"cc,omitempty" jsonschema:"Who gets a copy once it is completed"`
		Send      bool    `json:"send,omitempty" jsonschema:"Send it now, rather than keep it a draft; only when asked to send"`
	}
	dsMade struct {
		EnvelopeID     string `json:"envelopeId"`
		Status         string `json:"status"`
		StatusDateTime string `json:"statusDateTime,omitempty"`
	}
	dsFound struct {
		Envelopes []map[string]any `json:"envelopes"`
		Next      int              `json:"next,omitempty" jsonschema:"Pass as page for more; left out when there is no more"`
	}
	dsText struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	}
)
