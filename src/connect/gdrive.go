package connect

import (
	"time"

	"golang.org/x/oauth2"
)

// Google Drive, with ARCHIVE_GOOGLE like Gmail: the client's authorized redirect URIs list
// <API>/connect/gdrive/callback, and its consent screen asks for drive, a restricted scope, to read every file
// and to make, change and trash them; it never deletes one for good.
// It syncs every file the account can see into archive.file from Drive's changes feed, which pushes too: changes.watch, a channel of Archive's own id that pushes to Archive with its
// token, for a week at most; the first sync a day after replaces it, and stops the one before. A push says its
// channel in X-Goog-Channel-ID and its token in X-Goog-Channel-Token; the sync push of a new channel says nothing changed.
func init() {
	Register(&Connector{
		ID: "gdrive", Name: "Google Drive", Env: "ARCHIVE_GOOGLE",
		Info: Info{By: "Google", Description: "Search and read your files, and make or change them when you ask", Category: CatFiles, Icon: IconDrive,
			Scopes: []string{"Read your files and documents", "Make, change and trash files when you ask"}},
		API: "https://www.googleapis.com/drive/v3",
		// Drive allows each user 325,000 quota units a minute and a project 1,000,000, and a download, the dearest read, costs
		// 200: 20 requests a second, every account's together, stay under both.
		Pace: 50 * time.Millisecond,
		Busy: []string{"Quota exceeded", "rateLimitExceeded", "userRateLimitExceeded", "Rate Limit Exceeded"},
		Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.google.com/o/oauth2/v2/auth", TokenURL: "https://oauth2.googleapis.com/token",
			AuthStyle: oauth2.AuthStyleInParams},
		Revoke: "https://oauth2.googleapis.com/revoke",
		Scopes: []string{"https://www.googleapis.com/auth/drive"},
		Params: map[string]string{"access_type": "offline", "prompt": "consent"},
		Who:    &Who{Req: Req{URL: "/about?fields=user"}, Names: []Path{"user/emailAddress"}, Lower: true},
		Actions: map[string]*Action{
			"search": Call[driveFind, driveFound](Plan{
				Doc: "Find files in My Drive and the drives shared with you, newest change first, by words in their name or " +
					"text, or by a Drive query: { files: [{ id, name, mimeType, modifiedTime, size, webViewLink, owners }], next } (pass next as page for more)",
				Clamp: map[string][2]int{"limit": {25, 100}},
				Steps: []Step{
					{If: "text", As: "a", In: "fullText contains '{'text}'"},
					{If: "query", As: "b", In: "({query})"},
					{As: "all", In: []any{"trashed = false", "{a}", "{b}"}},
					{Unless: "text", As: "order", In: "modifiedTime desc"},
					{As: "got", Req: Req{URL: "/files", Query: Q{"q": "{all, and }", "pageSize": "{limit}", "supportsAllDrives": "true", "includeItemsFromAllDrives": "true",
						"fields": "nextPageToken,files(" + driveFields + ")", "orderBy": "{order}", "pageToken": "{page}"}}},
				},
				Out: Obj{"files": "{got/files|[]}", "next": "{got/nextPageToken|}"},
			}),
			"file": Call[driveOne, map[string]any](Plan{
				Doc:   "One file's details by id: { id, name, mimeType, modifiedTime, size, webViewLink, owners, parents, description }",
				Steps: []Step{{Req: Req{URL: "/files/{id}", Query: Q{"supportsAllDrives": "true", "fields": driveFields + ",parents,description"}}}},
			}),
			"export": Call[driveExport, driveText](Plan{
				Doc: "A file's content as text: a Google Doc as plain text, a Sheet as CSV (its first sheet), a Slides deck as plain text, " +
					"and a text, CSV, JSON or Markdown file as it is; up to 5 MB. Returns { id, name, mimeType, text }",
				Steps: []Step{
					{As: "f", Req: Req{URL: "/files/{id}?supportsAllDrives=true&fields=id,name,mimeType"}},
					{Only: Test{At: "f/mimeType", Is: driveDoc}, As: "as", In: "text/plain"},
					{Only: Test{At: "f/mimeType", Is: driveSheet}, As: "as", In: "text/csv"},
					{Only: Test{At: "f/mimeType", Is: driveDeck}, As: "as", In: "text/plain"},
					{If: "type", Only: driveGoogle, As: "as", In: "{type}"},
					{Unless: "as", Only: driveGoogle, Fail: "{f/name} is a {f/mimeType}, which has no text to export"},
					{Only: Test{At: "f/mimeType", Re: `^(application/vnd\.google-apps\.|text/|application/json$)`, Off: true}, Fail: "{f/name} is a {f/mimeType} file, not text; only Google files and text files export"},
					{Only: driveGoogle, As: "text", Req: Req{URL: "/files/{f/id}/export", Query: Q{"mimeType": "{as}"}, Text: true}},
					{Unless: "as", As: "text", Req: Req{URL: "/files/{f/id}?alt=media&supportsAllDrives=true", Text: true}},
				},
				Out: Obj{"id": "{f/id}", "name": "{f/name}", "mimeType": "{as;f/mimeType}", "text": "{text|}"},
			}),
			"create": Call[driveNew, driveMade](Plan{
				Doc: "Make a file or folder in Drive, maybe with text content: { id, name, mimeType, modifiedTime, size, webViewLink, owners }",
				Steps: []Step{
					{If: "folder", As: "parents", In: []any{"{folder}"}},
					{Unless: "content", Req: Req{Method: POST, URL: "/files", Query: driveSaving, Body: driveNewMeta}},
					{If: "content", Req: Req{Method: POST, URL: driveUpload + "/files", Query: driveUploading, Parts: []Part{{Type: "application/json; charset=UTF-8", Body: driveNewMeta}, driveContent}}},
				},
			}),
			"update": Call[driveEdit, driveMade](Plan{
				Doc: "Rename a file, replace its content or move it to another folder: { id, name, mimeType, modifiedTime, size, webViewLink, owners }",
				Steps: []Step{
					{If: "folder", As: "f", Req: Req{URL: "/files/{id}?supportsAllDrives=true&fields=parents"}},
					{Unless: "content", Req: Req{Method: PATCH, URL: "/files/{id}", Query: Q{"supportsAllDrives": "true", "fields": driveFields, "addParents": "{folder}", "removeParents": "{f/parents,}"}, Body: Obj{"name": "{name}"}}},
					{If: "content", Req: Req{Method: PATCH, URL: driveUpload + "/files/{id}", Query: Q{"uploadType": "multipart", "supportsAllDrives": "true", "fields": driveFields, "addParents": "{folder}", "removeParents": "{f/parents,}"},
						Parts: []Part{{Type: "application/json; charset=UTF-8", Body: Obj{"name": "{name}"}}, driveContent}}},
				},
			}),
			"upload": Upload(Send{
				Doc: "Send a file of the file table to Drive, as a new file or a new version of one, in the time a run has: { id, name, mimeType, modifiedTime, size, webViewLink, owners }. " +
					"Drive has no write that takes a version, so Archive asks it first what version the file is; a change in the moment after is overwritten",
				Let:      []Step{{If: "in/id", As: "parents", In: []any{"{in/id}"}}},
				New:      Req{Method: POST, URL: driveUpload + "/files", Query: driveResuming, Header: driveUpHeader, Body: Obj{"name": "{name}", "parents": "{parents}"}},
				Over:     Req{Method: PATCH, URL: driveUpload + "/files/{it/id}", Query: driveResuming, Header: driveUpHeader, Body: Obj{"name": "{rename}"}},
				AtHeader: "Location",
				Check:    &Req{URL: "/files/{it/id}?supportsAllDrives=true&fields=md5Checksum,modifiedTime"},
				Now:      "{ans/md5Checksum;ans/modifiedTime}",
				Part:     Req{Method: PUT, Header: map[string][]string{"Content-Range": {"{range}"}}},
				Step:     8 << 20,
			}),
			"trash": Call[driveOne, driveMade](Plan{
				Doc:   "Move a file or folder to Drive's trash, from where its owner can restore it for 30 days: { id, name, mimeType, modifiedTime, size, webViewLink, owners }",
				Steps: []Step{{Req: Req{Method: PATCH, URL: "/files/{id}", Query: driveSaving, Body: Obj{"trashed": true}}}},
			}),
		},
		Listen: &Listen{Renew: 6 * 24 * time.Hour, Until: 7*24*time.Hour - time.Hour, Mint: "archive-{nonce}", Secret: true, Replace: true,
			Ask:      &Req{URL: "/changes/startPageToken?supportsAllDrives=true"},
			Watch:    Req{Method: POST, URL: "/changes/watch", Query: Q{"pageToken": "{ask/startPageToken}", "supportsAllDrives": "true", "includeItemsFromAllDrives": "true"}, Body: Obj{"id": "{key}", "type": "web_hook", "address": "{hook}", "token": "{secret}", "expiration": "{until_ms}"}},
			Resource: "{answer/resourceId}", Expires: "expiration",
			Unwatch: &Req{Method: POST, URL: "/channels/stop", Body: Obj{"id": "{key}", "resourceId": "{resource}"}},
			Kept:    &Kept{IDHeader: "X-Goog-Channel-ID", TokenHeader: "X-Goog-Channel-Token", Quiet: "X-Goog-Resource-State", QuietIs: "sync"},
		},
		Sync: []*Sync{{Name: "files", Gap: 2 * time.Second,
			Req: Req{URL: "/files", Query: Q{"q": "trashed = false", "pageSize": "100", "corpora": "allDrives", "supportsAllDrives": "true", "includeItemsFromAllDrives": "true",
				"fields": "nextPageToken,incompleteSearch,files(" + driveSync + ")", "pageToken": "{page}"}},
			Items: "files", Updated: "modifiedTime", Page: Cursor{At: "nextPageToken"}, Short: Test{At: "incompleteSearch", Is: "true"},
			Change: Resume{Start: &Req{URL: "/changes/startPageToken?supportsAllDrives=true"}, From: "startPageToken", At: "newStartPageToken",
				Req: Req{URL: "/changes", Query: Q{"pageToken": "{page;token}", "pageSize": "100", "includeRemoved": "true", "supportsAllDrives": "true", "includeItemsFromAllDrives": "true",
					"fields": "nextPageToken,newStartPageToken,changes(fileId,removed,file(" + driveSync + "))"}},
				Items: "changes", Inner: "file", ID: "fileId", Gone: []Test{{At: "removed", Is: "true"}, {At: "file", Off: true}, {At: "file/trashed", Is: "true"}}},
			Hydrate: []Hydrate{
				Fetch{If: Test{At: "driveId"}, Once: "{driveId}", Soft: true, Req: Req{URL: "/drives/{driveId}?fields=name"}, As: "shared"},
			},
			// Google exports at most 10 MB of a document, and has no bytes of what it makes that exports to nothing, like a form.
			Content: []Content{
				{If: Test{At: "mimeType", Is: driveDoc}, Req: Req{URL: "/files/{source/id}/export", Query: Q{"mimeType": driveDOCX}}, Too: driveTooBig},
				{If: Test{At: "mimeType", Is: driveSheet}, Req: Req{URL: "/files/{source/id}/export", Query: Q{"mimeType": driveXLSX}}, Too: driveTooBig},
				{If: Test{At: "mimeType", Is: driveDeck}, Req: Req{URL: "/files/{source/id}/export", Query: Q{"mimeType": drivePPTX}}, Too: driveTooBig},
				{If: Test{At: "mimeType", Re: `^application/vnd\.google-apps\.`, Off: true}, Req: Req{URL: "/files/{source/id}?alt=media&supportsAllDrives=true"}, Size: "size",
					Too: []Deny{{Status: []int{403}, Says: []string{"cannot be downloaded", "cannotDownload", "not downloadable"}, Why: "Google will not hand it over: {message}"}}},
			},
			Doc: "Every file and folder in My Drive, shared with you and in your shared drives, but those in the trash: its name, type, " +
				"folder, owner, who changed it last and when, and a copy of its bytes up to 1 GB, with its words: a Google Doc, Sheet or Slides deck as the Word, Excel or " +
				"PowerPoint file it exports to, up to the 10 MB Google exports. A listing Drive says may have left files out deletes none. " +
				"Listed whole the first time, 100 a page, and then Drive's changes since.",
			// A file's folder is its first parent, and its owner the first of them, or for a file of a shared drive, which has none, nobody.
			// Google's own documents are kept as the Office files they export to, named so.
			To: Map{Table: "file",
				Cols: map[string]SQL{"kind": "CASE WHEN data->>'mimeType' = 'application/vnd.google-apps.folder' THEN 'folder' ELSE 'file' END",
					"name": "CASE data->>'mimeType' WHEN '" + driveDoc + "' THEN regexp_replace(data->>'name', '(\\.docx)?$', '.docx', 'i') WHEN '" + driveSheet + "' THEN regexp_replace(data->>'name', '(\\.xlsx)?$', '.xlsx', 'i') " +
						"WHEN '" + driveDeck + "' THEN regexp_replace(data->>'name', '(\\.pptx)?$', '.pptx', 'i') ELSE data->>'name' END",
					"mime":    "CASE data->>'mimeType' WHEN '" + driveDoc + "' THEN '" + driveDOCX + "' WHEN '" + driveSheet + "' THEN '" + driveXLSX + "' WHEN '" + driveDeck + "' THEN '" + drivePPTX + "' ELSE coalesce(data->>'mimeType', 'application/octet-stream') END",
					"version": "coalesce(data->>'md5Checksum', data->>'modifiedTime')", "source": "jsonb_build_object('id', data->>'id', 'mimeType', data->>'mimeType', 'size', data->>'size')",
					"url":        "data->>'webViewLink'",
					"drive":      "CASE WHEN nullif(data->>'driveId', '') IS NOT NULL THEN coalesce(nullif(data#>>'{shared,name}', ''), 'Shared drive') WHEN data->>'ownedByMe' = 'true' THEN 'My Drive' ELSE 'Shared with me' END",
					"owner_name": "data#>>'{owners,0,displayName}'", "editor_name": "data#>>'{lastModifyingUser,displayName}'", "made_at": "archive.sync_at(data->>'createdTime')",
					"edited_at": "archive.sync_at(data->>'modifiedTime')"},
				People: map[string]SQL{"owner": "data#>>'{owners,0,emailAddress}'", "editor": "data#>>'{lastModifyingUser,emailAddress}'"},
				Refs:   map[string]SQL{"parent": "data#>>'{parents,0}'"}}}},
	})
}

const (
	driveFields = "id,name,mimeType,modifiedTime,size,webViewLink,owners(displayName,emailAddress)"
	// driveSync is what the files stream reads of a file.
	driveSync = "id,name,mimeType,size,md5Checksum,webViewLink,parents,driveId,ownedByMe,createdTime,modifiedTime,trashed," +
		"owners(displayName,emailAddress),lastModifyingUser(displayName,emailAddress)"
	driveUpload = "https://www.googleapis.com/upload/drive/v3"
	driveDOCX   = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	driveXLSX   = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	drivePPTX   = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	driveDoc    = "application/vnd.google-apps.document"
	driveSheet  = "application/vnd.google-apps.spreadsheet"
	driveDeck   = "application/vnd.google-apps.presentation"
)

var (
	driveTooBig    = []Deny{{Status: []int{403}, Says: []string{"too large to be exported", "exportSizeLimitExceeded"}, Why: "Google exports at most 10 MB of a document, and this one comes to more"}}
	driveGoogle    = Test{At: "f/mimeType", Re: `^application/vnd\.google-apps\.`}
	driveSaving    = Q{"supportsAllDrives": "true", "fields": driveFields}
	driveUploading = Q{"uploadType": "multipart", "supportsAllDrives": "true", "fields": driveFields}
	driveResuming  = Q{"uploadType": "resumable", "supportsAllDrives": "true", "fields": driveFields}
	driveUpHeader  = map[string][]string{"X-Upload-Content-Type": {"{mime}"}, "X-Upload-Content-Length": {"{size}"}, "Content-Type": {"application/json; charset=UTF-8"}}
	driveNewMeta   = Obj{"name": "{name}", "mimeType": "{type}", "parents": "{parents}"}
	driveContent   = Part{Type: "{contentType|text/plain}; charset=UTF-8", Body: "{content}"}
)

type (
	driveFind struct {
		Text  string `json:"text,omitempty" jsonschema:"Words in the file's name or content"`
		Query string `json:"query,omitempty" jsonschema:"A Drive query instead, like mimeType = 'application/pdf' and modifiedTime > '2026-01-01'"`
		Limit int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 100; 25 when left out"`
		Page  string `json:"page,omitempty" jsonschema:"The next of the search before, for the files after it"`
	}
	driveOne struct {
		ID string `json:"id" jsonschema:"The file's id, from search"`
	}
	driveExport struct {
		ID   string `json:"id" jsonschema:"The file's id, from search"`
		Type string `json:"type,omitempty" jsonschema:"The MIME type to export a Google file as, like text/html or text/markdown, in place of the default"`
	}
	driveNew struct {
		Name        string `json:"name" jsonschema:"The file's name, like Q3 notes"`
		Content     string `json:"content,omitempty" jsonschema:"Its content, as text"`
		ContentType string `json:"contentType,omitempty" jsonschema:"The content's MIME type, like text/csv, text/html or text/markdown; text/plain when left out"`
		Type        string `json:"type,omitempty" jsonschema:"The file's MIME type when it differs from the content's: application/vnd.google-apps.document makes a Google Doc of text, HTML or Markdown, application/vnd.google-apps.spreadsheet a Sheet of CSV, application/vnd.google-apps.folder a folder"`
		Folder      string `json:"folder,omitempty" jsonschema:"The id of the folder it goes in; the top of My Drive when left out"`
	}
	driveEdit struct {
		ID          string `json:"id" jsonschema:"The file's id, from search"`
		Name        string `json:"name,omitempty" jsonschema:"Its new name"`
		Content     string `json:"content,omitempty" jsonschema:"Its new content, as text; a Google Doc or Sheet converts it as create does"`
		ContentType string `json:"contentType,omitempty" jsonschema:"The content's MIME type; text/plain when left out"`
		Folder      string `json:"folder,omitempty" jsonschema:"The id of the folder to move it to"`
	}
	driveMade struct {
		ID           string           `json:"id"`
		Name         string           `json:"name"`
		MIMEType     string           `json:"mimeType"`
		ModifiedTime string           `json:"modifiedTime,omitempty"`
		Size         string           `json:"size,omitempty"`
		WebViewLink  string           `json:"webViewLink,omitempty"`
		Owners       []map[string]any `json:"owners,omitempty"`
	}
	driveFound struct {
		Files []driveMade `json:"files"`
		Next  string      `json:"next" jsonschema:"Pass as page for more; empty when there is no more"`
	}
	driveText struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		MIMEType string `json:"mimeType"`
		Text     string `json:"text"`
	}
)
