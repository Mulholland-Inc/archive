package connect

import (
	"time"

	"golang.org/x/oauth2"
)

// Figma, through its REST API, as the person who connects: the folders (once projects) of their team, the files in
// them and their comments, which are text where a file's design is not. Its OAuth app, made in Figma's developer
// console, lists <API>/connect/figma/callback as its callback URL and the granular Scopes here, which replaced
// files:read and projects:read; it is published as a public app, which Figma reviews before anyone outside
// Mulholland's own team can sign in to it. Its client authenticates with HTTP Basic, and an access token lasts
// 90 days.
//
// Figma's API lists no one's teams: a team's id is only in its URL, figma.com/files/team/<id>, so Archive asks for
// that link before the sign-in, and each action that needs a team takes one too. It syncs the team's folders and
// every file in them, walking folders in folders, into file, each file with its last change and a copy of its
// thumbnail, the only picture of a design the API gives, as its bytes; and every comment and reply on those files
// into chat_message, a day apart. Each sync lists them whole, which deletes what is gone. Its webhooks are per team
// and need an admin, so it pushes nothing: connecting, asking and reading start a sync.
func init() {
	Register(&Connector{
		ID: "figma", Name: "Figma",
		Info: Info{By: "Figma", Description: "Keep your team’s folders, files and comments, and comment when you ask", Category: CatFiles, Icon: IconDrive,
			Scopes: []string{"Read your name and email", "Read your team’s folders, files’ details and comments", "Comment on files when you ask"}},
		Ask: &Key{Name: "team", Label: "Figma team link", Help: "Your team’s link, from Figma’s sidebar: figma.com/files/team/<number>/…",
			Find: `(?:^|team/)(\d+)`},
		API: "https://api.figma.com",
		Endpoint: oauth2.Endpoint{AuthURL: "https://www.figma.com/oauth", TokenURL: "https://api.figma.com/v1/oauth/token",
			AuthStyle: oauth2.AuthStyleInHeader},
		Scopes: []string{"current_user:read", "folders:read", "folder_metadata:read", "file_metadata:read", "file_comments:read", "file_comments:write"},
		Busy:   []string{"Rate limit exceeded"},
		Who:    &Who{Req: Req{URL: "/v1/me"}, Names: []Path{"email", "handle"}, Lower: true},
		Actions: map[string]*Action{
			"folders": Call[figmaFolders, figmaFound](Plan{
				Doc: "The folders of your team, or of another team you can see, or those in a folder: { folders: [{ id, name }] }. A team's id is in its URL, " +
					"figma.com/files/team/<id>, as Figma's API lists no teams",
				Steps: []Step{
					{As: "id", In: "{meta/team}"},
					{If: "team", As: "id", In: "{team}", Find: `(?:^|team/)(\d+)`},
					{If: "folder", As: "got", Req: Req{URL: "/v2/folders/{folder}/folders"}},
					{Unless: "folder", As: "got", Req: Req{URL: "/v2/teams/{id}/folders"}},
				},
				Out: Obj{"folders": "{got/folders;got/projects|[]}"},
			}),
			"files": Call[figmaIn, any](Plan{
				Doc:   "The files of a folder: { files: [{ key, name, thumbnail_url, last_modified }] }",
				Steps: []Step{{Req: Req{URL: "/v2/folders/{folder}/files"}}},
			}),
			"file": Call[figmaOne, any](Plan{
				Doc:   "A file's details, not its design: { file: { name, folder_name, editorType, creator, last_touched_at, last_touched_by, url, role, link_access } }",
				Steps: []Step{figmaKey, {Req: Req{URL: "/v1/files/{file}/meta"}}},
			}),
			"comments": Call[figmaOne, []map[string]any](Plan{
				Doc:   "Every comment on a file, in Markdown: [{ id, message, user, created_at, resolved_at, parent_id, order_id }], a reply with the parent_id of the comment it answers",
				Steps: []Step{figmaKey, {As: "got", Req: Req{URL: "/v1/files/{file}/comments", Query: Q{"as_md": "true"}}}},
				Out:   "{got/comments|[]}",
			}),
			"comment": Call[figmaSay, map[string]any](Plan{
				Doc:   "Comment on a file, as you, or reply to a comment that is not itself a reply: the comment, { id, message, user, created_at }",
				Steps: []Step{figmaKey, {Req: Req{Method: POST, URL: "/v1/files/{file}/comments", Body: Obj{"message": "{text}", "comment_id": "{reply_to}"}}}},
			}),
		},
		Sync: []*Sync{
			{Name: "folders", Gap: time.Second, Parts: figmaTree, Items: "**", Key: []Reads{{Of: "folder/{id}"}}, Set: Obj{"in": "{in/id}"}, Pass: []int{400, 404},
				Req: Req{URL: "{+in/url}"}, Doc: "The team's folders, by folder/id, and those in them: listed whole each sync, so one removed is deleted.",
				To: Map{Table: "file",
					Cols: map[string]SQL{"kind": "'folder'", "name": "data->>'name'", "drive": "'Figma'", "mime": "'inode/directory'",
						"url": "'https://www.figma.com/files/folder/' || (data->>'id')", "source": "jsonb_build_object('id', data->>'id')"},
					Refs: map[string]SQL{"parent": "CASE WHEN data->>'in' <> 'top' THEN 'folder/' || (data->>'in') END"}}},
			{Name: "files", Gap: time.Second, Parts: figmaFolderParts, Items: "**", Key: []Reads{{Of: "{key}"}}, Updated: "last_modified", Memo: "{last_modified}", Set: Obj{"folder": "{in/id}"},
				Pass: []int{400, 404},
				Req:  Req{URL: "/v2/folders/{part}/files"},
				// A thumbnail is a signed link of Figma's storage, which refuses Figma's token; its scheme is kept apart, as only a path may start the template.
				Content: []Content{{If: Test{At: "thumbnail"}, Req: Req{URL: "https://{+source/thumbnail}"}, Bare: true}},
				Doc: "Every file of the team's folders, by its key, with when it last changed and a copy of its thumbnail, as Figma's API gives no file's design: " +
					"listed whole each sync, so one removed is deleted; a thumbnail is copied again only when the file changed.",
				To: Map{Table: "file",
					Cols: map[string]SQL{"kind": "'file'", "name": "data->>'name'", "drive": "'Figma'", "mime": "'image/png'",
						"version": "data->>'last_modified'", "edited_at": "archive.sync_at(data->>'last_modified')",
						"url":    "'https://www.figma.com/design/' || (data->>'key')",
						"source": "jsonb_build_object('key', data->>'key', 'thumbnail', nullif(regexp_replace(data->>'thumbnail_url', '^https://', ''), ''), 'folder', data->>'folder')"},
					Refs: map[string]SQL{"parent": "'folder/' || (data->>'folder')"}}},
			{Name: "comments", Every: 24 * time.Hour, Gap: time.Second, Parts: figmaFileParts, Items: "comments", Key: []Reads{{Of: "{id}"}}, Pass: []int{400, 403, 404},
				Set: Obj{"file": "{in/key}", "file_name": "{in/name}"},
				Req: Req{URL: "/v1/files/{part}/comments", Query: Q{"as_md": "true"}},
				Doc: "Every comment and reply on the team's files, by its id, in Markdown, with who wrote it and when: listed a day apart, each file's whole, " +
					"so one deleted is deleted here.",
				To: Map{Table: "chat_message",
					Cols: map[string]SQL{"channel": "coalesce(nullif(data->>'file_name', ''), 'Figma')", "thread": "coalesce(nullif(data->>'parent_id', ''), data->>'id')",
						"sender_name": "data#>>'{user,handle}'", "text": "archive.sync_clip(data->>'message', 65536)",
						"sent_at": "archive.sync_at(data->>'created_at')",
						"url":     "'https://www.figma.com/design/' || (data->>'file') || '?comment-id=' || (data->>'id')"}}},
		},
	})
}

var (
	// figmaKey is a file's key, given as it or a URL of the file.
	figmaKey = Step{As: "file", In: "{key}", Find: `^(?:.*/(?:file|design|board|slides|proto|deck|site|make)/)?([0-9A-Za-z]+)`}

	// figmaFolderParts is every folder of the team, and every folder in those, by its id.
	figmaFolderParts = &Parts{Req: Req{URL: "/v2/teams/{meta/team}/folders"}, Items: "**", Soft: []int{400, 404},
		Deep: &Req{URL: "/v2/folders/{id}/folders"}, Vals: Obj{"id": "{id}", "name": "{name}", "url": "/v2/folders/{id}/folders"}}
	// figmaTree is top, the team, and every folder, each with the url that lists the folders in it.
	figmaTree = &Parts{Fixed: []Obj{{"id": "top"}}, Vals: Obj{"id": "top", "url": "/v2/teams/{meta/team}/folders"},
		Also: []*Parts{figmaFolderParts}}
	// figmaFileParts is every file of every folder, by its key.
	figmaFileParts = &Parts{Req: Req{URL: "/v2/teams/{meta/team}/folders"}, Items: "**", Soft: []int{400, 404},
		Deep:  &Req{URL: "/v2/folders/{id}/folders"},
		Under: &Parts{Req: Req{URL: "/v2/folders/{up/id}/files"}, Items: "**", Soft: []int{400, 404}, ID: "{key}", Vals: Obj{"key": "{key}", "name": "{name}"}}}
)

type (
	figmaFolders struct {
		Team   string `json:"team,omitempty" jsonschema:"Another team's id, or its URL, figma.com/files/team/<id>/..., for its top folders; yours when left out"`
		Folder string `json:"folder,omitempty" jsonschema:"A folder's id, for the folders in it"`
	}
	figmaIn struct {
		Folder string `json:"folder" jsonschema:"The folder's id, from folders"`
	}
	figmaOne struct {
		Key string `json:"key" jsonschema:"The file's key, or its URL, figma.com/design/<key>/..."`
	}
	figmaSay struct {
		Key     string `json:"key" jsonschema:"The file's key, or its URL"`
		Text    string `json:"text" jsonschema:"The comment"`
		ReplyTo string `json:"reply_to,omitempty" jsonschema:"The id of the comment to reply to, which must not be a reply"`
	}
	figmaFound struct {
		Folders []map[string]any `json:"folders"`
	}
)
