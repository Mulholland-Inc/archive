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
// It syncs every file and folder of the account into drive_file, by its lowercase path, as Dropbox names a file
// deleted by its path alone: a move is the old path gone and the new one made. It is listed whole the first time,
// 2,000 a page, and then read from the cursor of the listing, which says what changed, deleted too; a cursor
// Dropbox resets lists whole again. Dropbox need not name each thing inside a folder deleted whole, so those may stay
// until then. Only what the account itself holds is read: the team space of a Dropbox team is not, as it needs
// a path root.
//
// Its pushes are the app's webhook, set up once in the App Console as <API>/push/dropbox, which Dropbox checks with
// a GET whose challenge it wants back. A push names the accounts that changed in list_folder.accounts, and
// X-Dropbox-Signature is the hex HMAC-SHA256 of its body with the app's secret.
func init() {
	Register(&Connector{
		ID: "dropbox", Name: "Dropbox",
		Info: Info{By: "Dropbox", Description: "Search and read your files, and make or change them when you ask", Category: CatFiles, Icon: IconDrive,
			Scopes: []string{"Read your files and folders", "Make, change, move and delete files when you ask"}},
		API:  "https://api.dropboxapi.com/2",
		Busy: []string{"too_many_requests", "too_many_write_operations"},
		Endpoint: oauth2.Endpoint{AuthURL: "https://www.dropbox.com/oauth2/authorize", TokenURL: "https://api.dropboxapi.com/oauth2/token",
			AuthStyle: oauth2.AuthStyleInParams},
		RevokeBy: &Req{Method: POST, URL: "https://api.dropboxapi.com/2/auth/token/revoke", Header: map[string][]string{"Authorization": {"Bearer {access}"}}},
		Scopes:   []string{"account_info.read", "files.metadata.read", "files.content.read", "files.content.write"},
		Params:   map[string]string{"token_access_type": "offline"},
		Who:      &Who{Req: dropboxMe, Names: []Path{"email"}, Lower: true},
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
			"export": Call[dropboxOne, dropboxText](Plan{
				Doc: "A file's words: a PDF, Word, Excel, PowerPoint or text file's up to 25 MB. Returns { id, name, path, text }",
				Steps: []Step{
					{As: "f", Req: Req{Method: POST, Read: true, URL: "/files/get_metadata", Body: Obj{"path": "{id}"}}},
					{Only: Test{At: "f/.tag", Is: "file", Off: true}, Fail: "{f/path_display} is a folder, which has no text"},
					{As: "text", Words: &Download{Req: dropboxGet("{f/id}"), Name: "{f/name}", Size: "f/size"}},
				},
				Out: Obj{"id": "{f/id}", "name": "{f/name}", "path": "{f/path_display}", "text": "{text|}"},
			}),
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
		},
		Listen: &Listen{Watch: dropboxMe, Key: "{answer/account_id}", Keys: []Path{"list_folder/accounts"},
			Hello: []Hello{{Query: "challenge"}},
			Signs: []Sign{{Header: "X-Dropbox-Signature", Enc: Hex, Of: "{body}"}}},
		// A listing's first page is list_folder and the rest list_folder/continue: a request without a cursor is refused as
		// a bad one, and sends the first in its place.
		Sync: []*Sync{{Name: "files", Gap: time.Second,
			Req: Req{Method: POST, Read: true, URL: "/files/list_folder/continue", Body: Obj{"cursor": "{page}"},
				Or: &Req{Method: POST, Read: true, URL: "/files/list_folder", Body: Obj{"path": "", "recursive": true, "include_deleted": false, "limit": 2000}}},
			Items: "entries", Key: []Reads{{Of: "{path_lower}"}}, Updated: "server_modified", Page: Cursor{At: "cursor", More: "has_more"},
			Change: Resume{At: "cursor", Req: Req{Method: POST, Read: true, URL: "/files/list_folder/continue", Body: Obj{"cursor": "{page;token}"}},
				Gone: []Test{{At: ".tag", Is: "deleted"}}, Stale: []Deny{{Status: []int{409}, Says: []string{"reset"}}}},
			Hydrate: []Hydrate{Download{If: Test{At: ".tag", Is: "file"}, Req: dropboxGet("{id}"), Name: "{name}", Size: "size", As: "text", Par: 4}},
			Doc: "Every file and folder of the account, those of folders shared with it too: its name, size, folder and when it changed, and the words " +
				"of a PDF, Word, Excel, PowerPoint or text file up to 25 MB, cut at 64 KB; other content stays in Dropbox. Listed whole the first time, " +
				"2,000 a page, by its lowercase path, and then what changed since, from Dropbox's cursor; a cursor Dropbox resets lists whole again.",
			// A thing's folder is the path it is in, which is no row at the top.
			To: Map{Table: "drive_file",
				Cols: map[string]SQL{"kind": "CASE WHEN data->>'.tag' = 'folder' THEN 'folder' ELSE 'file' END", "title": "data->>'name'", "size": "archive.sync_num(data->>'size')",
					"url": "'https://www.dropbox.com/home' || (data->>'path_display')", "drive": "'Dropbox'",
					"made_at": "archive.sync_at(data->>'client_modified')", "edited_at": "archive.sync_at(data->>'server_modified')", "text": "data->>'text'"},
				Refs: map[string]SQL{"folder": "regexp_replace(data->>'path_lower', '/[^/]*$', '')"}}}},
	})
}

// dropboxContent is where files go up and come down.
const dropboxContent = "https://content.dropboxapi.com/2"

var dropboxMe = Req{Method: POST, Read: true, URL: "/users/get_current_account"}

// dropboxGet downloads the file at path, a template; its argument goes in the query, which takes any character,
// where a header takes only ASCII.
func dropboxGet(path string) Req {
	return Req{URL: dropboxContent + "/files/download", Query: Q{"arg": `{"path":"` + path + `"}`}}
}

// dropboxPut uploads {content} to {path} with the given write mode.
func dropboxPut(mode string) Req {
	return Req{Method: POST, URL: dropboxContent + "/files/upload", Query: Q{"arg": `{"path":"{"path}",` + mode + `}`},
		Header: map[string][]string{"Content-Type": {"application/octet-stream"}}, Body: "{content}"}
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
