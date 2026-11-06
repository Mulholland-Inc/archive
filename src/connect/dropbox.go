package connect

import (
	"time"

	"golang.org/x/oauth2"
)

// Dropbox, through its API v2. It reads ARCHIVE_DROPBOX, a scoped app with Full Dropbox access from the App
// Console, whose redirect URIs list <API>/connect/dropbox/callback and whose permissions tick exactly the Scopes
// here: to read the account and every file, and to upload, move and delete them, which Dropbox keeps restorable
// for 30 days or more. Its access tokens last four hours, and token_access_type=offline asks for a refresh token,
// which Dropbox never replaces. A development app links 500 accounts at most, and two weeks after its 50th it
// links no more until Dropbox approves it for production. Revoking an access token ends the whole grant.
//
// Its pushes are the app's webhook, set up once in the App Console as <API>/push/dropbox, which Dropbox checks with
// a GET whose challenge it wants back. A push names the accounts that changed in list_folder.accounts, and
// X-Dropbox-Signature is the hex HMAC-SHA256 of its body with the app's secret.
func init() {
	Register(&Connector{
		ID: "dropbox", Name: "Dropbox",
		Info: Info{By: "Dropbox", Description: "Search and read your files, make or change them, and send files there when you ask", Category: CatFiles, Icon: IconDrive,
			Scopes: []string{"Read your files and folders", "Make, change, move, delete and replace files when you ask"}},
		API:  "https://api.dropboxapi.com/2",
		Busy: []string{"too_many_requests", "too_many_write_operations"},
		Endpoint: oauth2.Endpoint{AuthURL: "https://www.dropbox.com/oauth2/authorize", TokenURL: "https://api.dropboxapi.com/oauth2/token",
			AuthStyle: oauth2.AuthStyleInParams},
		RevokeBy: &Req{Method: POST, URL: "https://api.dropboxapi.com/2/auth/token/revoke", Header: map[string][]string{"Authorization": {"Bearer {access}"}}},
		Scopes:   []string{"account_info.read", "files.metadata.read", "files.content.read", "files.content.write"},
		Params:   map[string]string{"token_access_type": "offline"},
		Who:      &Who{Req: Req{Method: POST, Read: true, URL: "/users/get_current_account"}, Names: []Path{"email"}, Lower: true},
		Actions: map[string]*Action{
			"search": Call[dropboxFind, dropboxFound](Plan{
				Doc: "Find files and folders by words in their name or, on plans that index it, their content, maybe only under a folder: " +
					"{ files: [{ .tag, id, name, path_display, size, server_modified }], next } (pass next as page for more)",
				Clamp: map[string][2]int{"limit": {25, 1000}},
				Steps: []Step{
					{If: "page", As: "got", Req: Req{Method: POST, Read: true, URL: "/files/search/continue_v2", Body: Obj{"cursor": "{page}"}}},
					{Unless: "page", As: "got", Req: Req{Method: POST, Read: true, URL: "/files/search_v2", Body: Obj{"query": "{query}",
						"options": Obj{"path": "{folder}", "max_results": "{limit}", "file_status": "active", "filename_only": "{names}"}}}},
					{If: "got/has_more", As: "next", In: "{got/cursor}"},
				},
				Out: Obj{"files": "{got/matches/*/metadata/metadata|[]}", "next": "{next|}"},
			}),
			"folder": Call[dropboxIn, dropboxListed](Plan{
				Doc: "What is in a folder, or at the top of the account: { entries: [{ .tag, id, name, path_display, size, server_modified }], next } (pass next as page for more)",
				Steps: []Step{
					{If: "page", As: "got", Req: Req{Method: POST, Read: true, URL: "/files/list_folder/continue", Body: Obj{"cursor": "{page}"}}},
					{Unless: "page", As: "got", Req: Req{Method: POST, Read: true, URL: "/files/list_folder", Body: Obj{"path": "{path|}", "limit": 500}}},
					{If: "got/has_more", As: "next", In: "{got/cursor}"},
				},
				Out: Obj{"entries": "{got/entries|[]}", "next": "{next|}"},
			}),
			"file": Call[dropboxOne, map[string]any](Plan{
				Doc:   "One file's or folder's details by id or path: { .tag, id, name, path_display, size, rev, client_modified, server_modified, content_hash }",
				Steps: []Step{{Req: Req{Method: POST, Read: true, URL: "/files/get_metadata", Body: Obj{"path": "{id}"}}}},
			}),
			"export": Func("A file's words: a PDF, Word, Excel, PowerPoint or text file's up to 25 MB. Returns { id, name, path, text }", dropboxExport),
			"create": Call[dropboxNew, map[string]any](Plan{
				Doc: "Make a file with text content, or a folder, at a path; a name already taken gets a number, as Dropbox does: " +
					"{ .tag, id, name, path_display, size, server_modified }",
				Steps: []Step{
					{If: "folder", As: "got", Req: Req{Method: POST, URL: "/files/create_folder_v2", Body: Obj{"path": "{path}", "autorename": true}}},
					{If: "folder", In: "{got/metadata}"},
					{Unless: "folder", Req: dropboxPut(`"mode":"add","autorename":true`)},
				},
			}),
			"update": Call[dropboxEdit, map[string]any](Plan{
				Doc: "Replace a file's content with text, or move or rename a file or folder to another path, or both: " +
					"{ .tag, id, name, path_display, size, server_modified }",
				Steps: []Step{
					{If: "content", As: "path", In: "{id}"},
					{If: "content", As: "f", Req: dropboxPut(`"mode":"overwrite"`)},
					{If: "to", As: "moved", Req: Req{Method: POST, URL: "/files/move_v2", Body: Obj{"from_path": "{id}", "to_path": "{to}"}}},
					{If: "to", In: "{moved/metadata}"},
				},
			}),
			"trash": Call[dropboxOne, map[string]any](Plan{
				Doc:   "Delete a file or folder the way Dropbox's own delete does, which its owner can restore for 30 days or more: { .tag, name, path_display }",
				Steps: []Step{{As: "got", Req: Req{Method: POST, URL: "/files/delete_v2", Body: Obj{"path": "{id}"}}}},
				Out:   "{got/metadata}",
			}),
			"upload": Upload(Send{
				Doc: "Send a file of the file table to Dropbox, as a new file or a new version of one, in the time a run has: { id, name, path_display, rev, size, content_hash, server_modified }",
				Let: []Step{
					{As: "path", In: "/{name}"},
					{If: "in/path", As: "path", In: "{in/path}/{name}"},
					{If: "it/path", As: "path", In: "{it/path}"},
					{As: "mode", In: "add"},
					{If: "it/rev", As: "mode", In: Obj{".tag": "update", "update": "{it/rev}"}},
				},
				New:  dropboxStart,
				Over: dropboxStart,
				At:   "session_id",
				Part: Req{Method: POST, URL: dropboxContent + "/upload_session/append_v2", Header: dropboxBytes,
					Head: map[string]any{"Dropbox-API-Arg": Obj{"cursor": Obj{"session_id": "{session}", "offset": "{offset}"}}}},
				Last: &Req{Method: POST, URL: dropboxContent + "/upload_session/finish", Header: dropboxBytes,
					Head: map[string]any{"Dropbox-API-Arg": Obj{"cursor": Obj{"session_id": "{session}", "offset": "{offset}"},
						"commit": Obj{"path": "{path}", "mode": "{mode}", "autorename": false, "mute": true, "strict_conflict": true}}}},
				Step: 8 << 20,
				Stale: []Deny{{Status: []int{409}, Says: []string{"path/conflict"},
					Why: "{name} is not there as Archive last synced it, or the name is taken, so nothing was written: look at what is there once it syncs, and pass replace with its source_id to replace it"}},
			}),
		},
		Listen: &Listen{Renew: 24 * time.Hour, Lasts: 48 * time.Hour,
			Ask: &Req{Method: POST, Read: true, URL: "/users/get_current_account"}, Key: "{ask/account_id}", Keys: []Path{"list_folder/accounts"},
			Signs: []Sign{{Header: "X-Dropbox-Signature", Enc: Hex, Of: "{body}"}},
			Hello: []Hello{{Query: "challenge"}},
		},
		Sync: []*Sync{{Name: "files", Gap: time.Second,
			Req:   Req{Method: POST, URL: "/files/list_folder", Body: Obj{"path": "", "recursive": true, "include_deleted": true, "limit": 2000}},
			Items: "entries", Key: []Reads{{Of: "{path_lower}"}}, Updated: "server_modified",
			Page: Cursor{At: "cursor", More: "has_more", Then: &dropboxMore},
			Change: Resume{At: "cursor", Req: Req{Method: POST, URL: "/files/list_folder/continue", Body: Obj{"cursor": "{token}"}},
				Stale: []Deny{{Status: []int{409}, Says: []string{"reset"}}}},
			Gone: []Test{{At: ".tag", Is: "deleted"}},
			Content: []Content{{If: Test{At: "downloadable", Is: "true"}, Size: "size",
				Req: Req{Method: POST, URL: dropboxContent + "/download", Head: map[string]any{"Dropbox-API-Arg": Obj{"path": "{source/id}"}}}}},
			Doc: "Every file and folder of your Dropbox: its name, folder, when it changed and a copy of its bytes up to 1 GB, with its words, but those Dropbox only exports, like a Paper doc. " +
				"Listed whole the first time, 2000 a page, by lowercase path, and then what changed since.",
			To: Map{Table: "file",
				Cols: map[string]SQL{"kind": "CASE WHEN data->>'.tag' = 'folder' THEN 'folder' ELSE 'file' END", "name": "data->>'name'", "drive": "'Dropbox'",
					"mime":    "CASE WHEN data->>'.tag' = 'folder' THEN 'inode/directory' ELSE 'application/octet-stream' END",
					"version": "data->>'content_hash'", "edited_at": "archive.sync_at(data->>'server_modified')",
					"source": "jsonb_build_object('id', data->>'id', 'path', data->>'path_display', 'rev', data->>'rev', 'size', data->>'size', 'downloadable', coalesce(data->>'is_downloadable', 'true'))"},
				Refs: map[string]SQL{"parent": "regexp_replace(data->>'path_lower', '/[^/]*$', '')"}}}},
	})
}

const dropboxContent = "https://content.dropboxapi.com/2/files"

var (
	dropboxMore  = Req{Method: POST, URL: "/files/list_folder/continue", Body: Obj{"cursor": "{page}"}}
	dropboxBytes = map[string][]string{"Content-Type": {"application/octet-stream"}}
	dropboxStart = Req{Method: POST, URL: dropboxContent + "/upload_session/start", Header: dropboxBytes, Head: map[string]any{"Dropbox-API-Arg": Obj{"close": false}}}
)

// dropboxExport reads the words of the file at an id or path; a folder has none.
func dropboxExport(c *Client, in dropboxOne) (dropboxText, error) {
	sc := map[string]any{"id": in.ID}
	if _, err := runSteps(c, []Step{
		{As: "f", Req: Req{Method: POST, Read: true, URL: "/files/get_metadata", Body: Obj{"path": "{id}"}}},
		{Only: Test{At: "f/.tag", Is: "file", Off: true}, Fail: "{f/path_display} is a folder, which has no text"},
	}, sc); err != nil {
		return dropboxText{}, err
	}
	words, _, err := c.download(Download{Req: dropboxGet("{f/id}"), Name: "{f/name}", Size: "f/size"}, sc)
	if err != nil {
		return dropboxText{}, err
	}
	return dropboxText{ID: text(pick(sc, "f/id")), Name: text(pick(sc, "f/name")), Path: text(pick(sc, "f/path_display")), Text: words}, nil
}

// dropboxGet downloads the file at path, a template; its argument goes in the query, which takes any character,
// where a header takes only ASCII.
func dropboxGet(path string) Req {
	return Req{URL: dropboxContent + "/download", Query: Q{"arg": `{"path":"` + path + `"}`}}
}

// dropboxPut uploads {content} to {path} with the given write mode.
func dropboxPut(mode string) Req {
	return Req{Method: POST, URL: dropboxContent + "/upload", Query: Q{"arg": `{"path":"{"path}",` + mode + `}`},
		Header: dropboxBytes, Body: "{content}"}
}

type (
	dropboxFind struct {
		Query  string `json:"query" jsonschema:"Words in the file's name or content"`
		Folder string `json:"folder,omitempty" jsonschema:"Only under this folder's path, like /Clients/Acme"`
		Names  bool   `json:"names,omitempty" jsonschema:"Match names only, not content"`
		Limit  int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 1000; 25 when left out"`
		Page   string `json:"page,omitempty" jsonschema:"The next of the search before, for the files after it"`
	}
	dropboxIn struct {
		Path string `json:"path,omitempty" jsonschema:"The folder's path, like /Clients, or its id; the top when left out"`
		Page string `json:"page,omitempty" jsonschema:"The next of the call before, for what comes after"`
	}
	dropboxOne struct {
		ID string `json:"id" jsonschema:"The file's id, like id:a4ayc_80_OEAAAAAAAAAXw, or its path, like /Reports/Q3.pdf"`
	}
	dropboxNew struct {
		Path    string `json:"path" jsonschema:"Where it goes, with its name, like /Reports/Q3 notes.txt"`
		Content string `json:"content,omitempty" jsonschema:"The file's content, as text"`
		Folder  bool   `json:"folder,omitempty" jsonschema:"Make a folder at the path instead of a file"`
	}
	dropboxEdit struct {
		ID      string `json:"id" jsonschema:"The file's id or path"`
		Content string `json:"content,omitempty" jsonschema:"Its new content, as text, in place of what it has"`
		To      string `json:"to,omitempty" jsonschema:"The path to move or rename it to, with its name, like /Archive/Q3.pdf"`
	}
	dropboxFound struct {
		Files []map[string]any `json:"files"`
		Next  string           `json:"next" jsonschema:"Pass as page for more; empty when there is no more"`
	}
	dropboxListed struct {
		Entries []map[string]any `json:"entries"`
		Next    string           `json:"next" jsonschema:"Pass as page for more; empty when there is no more"`
	}
	dropboxText struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Path string `json:"path"`
		Text string `json:"text"`
	}
)
