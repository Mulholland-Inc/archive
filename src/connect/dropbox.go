package connect

import (
	"time"

	"golang.org/x/oauth2"
)

func init() {
	Register(&Connector{
		ID: "dropbox", Name: "Dropbox",
		Info: Info{By: "Dropbox", Description: "Search and read the files of your Dropbox, and send files there when you ask", Category: CatFiles, Icon: IconDrive,
			Scopes: []string{"Read your files and folders", "Add and replace files when you ask"}},
		API: "https://api.dropboxapi.com/2",
		Endpoint: oauth2.Endpoint{AuthURL: "https://www.dropbox.com/oauth2/authorize", TokenURL: "https://api.dropboxapi.com/oauth2/token",
			AuthStyle: oauth2.AuthStyleInParams},
		RevokeBy: &Req{Method: POST, URL: "https://api.dropboxapi.com/2/auth/token/revoke", Header: map[string][]string{"Authorization": {"Bearer {access}"}}},
		Scopes:   []string{"account_info.read", "files.metadata.read", "files.content.read", "files.content.write"},
		Params:   map[string]string{"token_access_type": "offline"},
		Who:      &Who{Req: Req{Method: POST, Read: true, URL: "/users/get_current_account"}, Names: []Path{"email"}, Lower: true},
		Actions: map[string]*Action{
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
