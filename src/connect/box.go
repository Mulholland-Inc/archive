package connect

import (
	"time"

	"golang.org/x/oauth2"
)

// Box, through its Content API. It reads ARCHIVE_BOX, a Platform App with User Authentication (OAuth 2.0) from
// Box's Developer Console, whose redirect URIs list <API>/connect/box/callback and whose application scopes tick
// Read all files and folders and Write all files and folders, root_readwrite. An enterprise that turned off
// unpublished apps has its admin enable the app by its client id, in the Admin Console's Platform Apps Manager,
// before its people can connect. Box replaces the refresh token on every refresh, and it lasts 60 days unused.
//
// It syncs every file and folder the account can see, its own and those shared with it, into drive_file, by
// type/id, as a file and a folder may share an id. Box lists a folder one level at a time and its events say what
// changed account-wide, not by folder, so each sync walks the folders from the top and lists each whole, 1,000 a
// page, which deletes what is gone; a file whose etag has not changed is not read again. It pushes nothing: a Box
// webhook watches one folder, never the top.
func init() {
	Register(&Connector{
		ID: "box", Name: "Box",
		Info: Info{By: "Box", Description: "Search and read your files, and make or change them when you ask", Category: CatFiles, Icon: IconDrive,
			Scopes: []string{"Read your files and folders", "Make, change and trash files when you ask"}},
		API: "https://api.box.com/2.0",
		Endpoint: oauth2.Endpoint{AuthURL: "https://account.box.com/api/oauth2/authorize", TokenURL: "https://api.box.com/oauth2/token",
			AuthStyle: oauth2.AuthStyleInParams},
		RevokeBy: &Req{Method: POST, URL: "https://api.box.com/oauth2/revoke", Form: Q{"client_id": "{client/id}", "client_secret": "{client/secret}", "token": "{refresh}"}},
		Scopes:   []string{"root_readwrite"},
		Who:      &Who{Req: Req{URL: "/users/me?fields=login"}, Names: []Path{"login"}, Lower: true},
		Actions: map[string]*Action{
			"search": Call[boxFind, boxFound](Plan{
				Doc: "Find files and folders by words in their name, description or content, best match first, maybe only of one type or under a folder: " +
					"{ entries: [{ type, id, name, size, modified_at, parent, owned_by }], total_count, offset } (pass offset plus limit as offset for more)",
				Clamp: map[string][2]int{"limit": {25, 200}},
				Enum:  map[string][]string{"type": {"file", "folder"}},
				Steps: []Step{{Req: Req{URL: "/search", Query: Q{"query": "{query}", "type": "{type}", "ancestor_folder_ids": "{folder}", "file_extensions": "{extensions,}",
					"limit": "{limit}", "offset": "{offset}", "fields": boxFields}}}},
			}),
			"folder": Call[boxIn, []map[string]any](Plan{
				Doc:   "What is in a folder, or at the top of the account: [{ type, id, name, size, modified_at, parent, owned_by }]",
				Steps: []Step{{Req: Req{URL: "/folders/{id|0}/items", Query: boxItems}, Page: boxPage, Items: "entries"}},
			}),
			"file": Call[boxOne, map[string]any](Plan{
				Doc:   "One file's or folder's details by id: { type, id, name, size, modified_at, parent, path_collection, owned_by, modified_by, description }",
				Enum:  map[string][]string{"type": {"file", "folder"}},
				Steps: []Step{{Req: Req{URL: "/{type|file}s/{id}", Query: Q{"fields": boxFields + ",path_collection,modified_by,description"}}}},
			}),
			"export": Call[boxFile, boxText](Plan{
				Doc: "A file's words: a PDF, Word, Excel, PowerPoint or text file's up to 25 MB. Returns { id, name, text }",
				Steps: []Step{
					{As: "f", Req: Req{URL: "/files/{id}?fields=id,name,size"}},
					{As: "text", Words: &Download{Req: Req{URL: "/files/{f/id}/content"}, Name: "{f/name}", Size: "f/size"}},
				},
				Out: Obj{"id": "{f/id}", "name": "{f/name}", "text": "{text|}"},
			}),
			"create": Call[boxNew, map[string]any](Plan{
				Doc: "Make a file with text content, or a folder, in a folder or at the top: { type, id, name, size, modified_at, parent, owned_by }",
				Steps: []Step{
					{If: "folder", Req: Req{Method: POST, URL: "/folders", Query: Q{"fields": boxFields}, Body: Obj{"name": "{name}", "parent": Obj{"id": "{parent|0}"}}}},
					{Unless: "folder", As: "got", Req: boxUpload("/files/content", `{"name":"{"name}","parent":{"id":"{parent|0}"}}`)},
					{Unless: "folder", In: "{got/entries/0}"},
				},
			}),
			"update": Call[boxEdit, map[string]any](Plan{
				Doc: "Replace a file's content with text, or rename a file or folder or move it to another folder, or both: " +
					"{ type, id, name, size, modified_at, parent, owned_by }",
				Enum: map[string][]string{"type": {"file", "folder"}},
				Steps: []Step{
					{If: "content", As: "got", Req: boxUpload("/files/{id}/content", `{}`)},
					{If: "content", In: "{got/entries/0}"},
					{If: "parent", As: "to", In: Obj{"id": "{parent}"}},
					{If: "name", Req: Req{Method: PUT, URL: "/{type|file}s/{id}", Query: Q{"fields": boxFields}, Body: Obj{"name": "{name}", "parent": "{to}"}}},
					{Unless: "name", If: "to", Req: Req{Method: PUT, URL: "/{type|file}s/{id}", Query: Q{"fields": boxFields}, Body: Obj{"parent": "{to}"}}},
				},
			}),
			"trash": Call[boxOne, boxTrashed](Plan{
				Doc: "Move a file, or a folder and all in it, to Box's trash, from where its owner can restore it for 30 days unless the enterprise " +
					"keeps it more or less: { id }",
				Enum: map[string][]string{"type": {"file", "folder"}},
				Steps: []Step{
					{Only: Test{At: "type", Is: "folder"}, As: "all", In: "true"},
					{Req: Req{Method: DELETE, URL: "/{type|file}s/{id}", Query: Q{"recursive": "{all}"}}},
				},
				Out: Obj{"id": "{id}"},
			}),
		},
		Sync: []*Sync{{Name: "files", Gap: time.Second, Parts: boxFolders, Pass: []int{403, 404},
			Req:   Req{URL: "/folders/{part}/items", Query: Q{"fields": boxSync, "limit": "1000", "usemarker": "true", "marker": "{page}"}},
			Items: "entries", Key: []Reads{{Of: "{type}/{id}"}}, Updated: "modified_at", Page: boxPage, Memo: "{etag}",
			// A web link is no file.
			Skip:    []Test{{At: "type", Is: "web_link"}},
			Hydrate: []Hydrate{Download{If: Test{At: "type", Is: "file"}, Req: Req{URL: "/files/{id}/content"}, Name: "{name}", Size: "size", As: "text", Par: 4}},
			Doc: "Every file and folder the account can see, its own and those shared with it, but those in the trash: its name, size, folder, " +
				"owner, who changed it last and when, and the words of a PDF, Word, Excel, PowerPoint or text file up to 25 MB, cut at 64 KB; other " +
				"content stays in Box. Each sync walks the folders from the top and lists each whole, 1,000 a page, by type/id, which deletes what " +
				"is gone; a file whose etag has not changed is not read again.",
			// A thing's folder is its parent, which is no row at the top.
			To: Map{Table: "drive_file",
				Cols: map[string]SQL{"kind": "CASE WHEN data->>'type' = 'folder' THEN 'folder' ELSE 'file' END", "title": "data->>'name'", "size": "archive.sync_num(data->>'size')",
					"url": "'https://app.box.com/' || (data->>'type') || '/' || (data->>'id')", "drive": "'Box'",
					"owner_name": "data#>>'{owned_by,name}'", "editor_name": "data#>>'{modified_by,name}'",
					"made_at": "archive.sync_at(data->>'created_at')", "edited_at": "archive.sync_at(data->>'modified_at')", "text": "data->>'text'"},
				People: map[string]SQL{"owner": "data#>>'{owned_by,login}'", "editor": "data#>>'{modified_by,login}'"},
				Refs:   map[string]SQL{"folder": "'folder/' || nullif(data#>>'{parent,id}', '0')"}}}},
	})
}

const (
	boxFields = "type,id,name,size,modified_at,parent,owned_by"
	// boxSync is what the files stream reads of a file or folder.
	boxSync = "type,id,etag,name,size,parent,created_at,modified_at,owned_by,modified_by"
)

var (
	boxPage  = Cursor{At: "next_marker"}
	boxItems = Q{"fields": boxFields, "limit": "1000", "usemarker": "true", "marker": "{page}"}
	// boxFolders are the top of the account and every folder under it, at any depth.
	boxFolders = &Parts{Req: Req{URL: "/folders/0/items", Query: boxWalk}, Items: "entries", Page: boxPage, Skip: []Test{{At: "type", Not: "folder"}},
		DeepIf: Test{At: "type", Is: "folder"}, Deep: &Req{URL: "/folders/{id}/items", Query: boxWalk},
		Also: []*Parts{{Fixed: []Obj{{"id": "0"}}}}}
	boxWalk = Q{"fields": "type,id", "limit": "1000", "usemarker": "true", "marker": "{page}"}
)

// boxUpload sends {content} to path on Box's upload host as the form Box takes, with attributes, a JSON template.
func boxUpload(path, attributes string) Req {
	return Req{Method: POST, URL: "https://upload.box.com/api/2.0" + path, Query: Q{"fields": boxFields},
		Header: map[string][]string{"Content-Type": {"multipart/form-data; boundary={nonce}"}},
		Body: "--{nonce}\r\nContent-Disposition: form-data; name=\"attributes\"\r\n\r\n" + attributes + "\r\n" +
			"--{nonce}\r\nContent-Disposition: form-data; name=\"file\"; filename=\"file\"\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n{content}\r\n--{nonce}--\r\n"}
}

type (
	boxFind struct {
		Query      string   `json:"query" jsonschema:"Words to find, or a phrase in double quotes"`
		Type       string   `json:"type,omitempty" jsonschema:"Only files or only folders"`
		Folder     string   `json:"folder,omitempty" jsonschema:"Only under this folder's id"`
		Extensions []string `json:"extensions,omitempty" jsonschema:"Only files of these extensions, like pdf or docx"`
		Limit      int      `json:"limit,omitempty" jsonschema:"At most this many, 1 to 200; 25 when left out"`
		Offset     int      `json:"offset,omitempty" jsonschema:"How many to skip, for the ones after a search before"`
	}
	boxIn struct {
		ID string `json:"id,omitempty" jsonschema:"The folder's id; the top when left out"`
	}
	boxOne struct {
		ID   string `json:"id" jsonschema:"Its id, from search or folder"`
		Type string `json:"type,omitempty" jsonschema:"file or folder; file when left out"`
	}
	boxFile struct {
		ID string `json:"id" jsonschema:"The file's id, from search or folder"`
	}
	boxNew struct {
		Name    string `json:"name" jsonschema:"Its name, like Q3 notes.txt"`
		Content string `json:"content,omitempty" jsonschema:"The file's content, as text"`
		Folder  bool   `json:"folder,omitempty" jsonschema:"Make a folder instead of a file"`
		Parent  string `json:"parent,omitempty" jsonschema:"The id of the folder it goes in; the top when left out"`
	}
	boxEdit struct {
		ID      string `json:"id" jsonschema:"Its id, from search or folder"`
		Type    string `json:"type,omitempty" jsonschema:"file or folder; file when left out"`
		Content string `json:"content,omitempty" jsonschema:"A file's new content, as text, kept as a new version"`
		Name    string `json:"name,omitempty" jsonschema:"Its new name"`
		Parent  string `json:"parent,omitempty" jsonschema:"The id of the folder to move it to"`
	}
	boxFound struct {
		Entries    []map[string]any `json:"entries"`
		TotalCount int              `json:"total_count"`
		Offset     int              `json:"offset"`
	}
	boxTrashed struct {
		ID string `json:"id"`
	}
	boxText struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Text string `json:"text"`
	}
)
