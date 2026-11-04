package connect

import "golang.org/x/oauth2"

// Figma, through its REST API, as the person who connects: the folders (once projects) of their teams, the files
// in them and their comments, which are text where a file's design is not. Its OAuth app, made in Figma's
// developer console, lists <API>/connect/figma/callback as its callback URL and the granular Scopes here, which
// replaced files:read and projects:read; it is published as a public app, which Figma reviews before anyone outside
// Mulholland's own team can sign in to it. Its client authenticates with HTTP Basic, and an access token lasts
// 90 days.
//
// Figma's API lists no one's teams: a team's id is only in its URL, figma.com/files/team/<id>, so it syncs
// nothing, and each action that needs one takes that id or URL. Its webhooks are per team too.
func init() {
	Register(&Connector{
		ID: "figma", Name: "Figma",
		Info: Info{By: "Figma", Description: "Read your teams’ folders, files and comments, and comment when you ask", Category: CatFiles, Icon: IconDrive,
			Scopes: []string{"Read your name and email", "Read your teams’ folders, files’ details and comments", "Comment on files when you ask"}},
		API: "https://api.figma.com",
		Endpoint: oauth2.Endpoint{AuthURL: "https://www.figma.com/oauth", TokenURL: "https://api.figma.com/v1/oauth/token",
			AuthStyle: oauth2.AuthStyleInHeader},
		Scopes: []string{"current_user:read", "folders:read", "folder_metadata:read", "file_metadata:read", "file_comments:read", "file_comments:write"},
		Busy:   []string{"Rate limit exceeded"},
		Who:    &Who{Req: Req{URL: "/v1/me"}, Names: []Path{"email", "handle"}, Lower: true},
		Actions: map[string]*Action{
			"folders": Call[figmaFolders, figmaFound](Plan{
				Doc: "The folders of a team you can see, or those in a folder: { folders: [{ id, name }] }. A team's id is in its URL, " +
					"figma.com/files/team/<id>, as Figma's API lists no teams",
				Steps: []Step{
					{Unless: "folder", Only: Test{At: "team", Off: true}, Fail: "pass team, the id or URL of a team, or folder"},
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
	})
}

// figmaKey is a file's key, given as it or a URL of the file.
var figmaKey = Step{As: "file", In: "{key}", Find: `^(?:.*/(?:file|design|board|slides|proto|deck|site|make)/)?([0-9A-Za-z]+)`}

type (
	figmaFolders struct {
		Team   string `json:"team,omitempty" jsonschema:"A team's id, or its URL, figma.com/files/team/<id>/..., for its top folders"`
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
