package connect

import (
	"time"

	"golang.org/x/oauth2"
)

// Asana, read only: workspaces, projects and tasks. It reads ARCHIVE_ASANA, an app of the Asana developer
// console distributed to any workspace, whose redirect URLs list <API>/connect/asana/callback and whose OAuth
// permission scopes include the Scopes here; with webhooks:read, webhooks:write and webhooks:delete too, and
// ?push=1 on the variable, it asks for those, which its pushes need. It syncs nothing until its
// tasks have tables in archive.sql. Asana's MCP server lets no client register itself, so
// this is its REST API. Revoking takes the client's credentials with the refresh token.
//
// It pushes through a webhook on each project of the account that is not archived, pushing to Archive's URL with a
// key of the connection's, the project and a nonce, which only Asana learns. Asana creates one only once its
// handshake, posted to that URL with X-Hook-Secret, is answered with the same header; Archive keeps that secret
// by the key, and a push after is signed with it: X-Hook-Signature, the hex HMAC-SHA256 of the body. Asana
// deletes a webhook that fails for a day, so the first sync a week after checks each is still there. A push
// without events is Asana's heartbeat, and starts nothing.
func init() {
	Register(&Connector{
		ID: "asana", Name: "Asana", Integration: true, Env: "ARCHIVE_ASANA", Options: "?push=1 once the app has the webhooks scopes",
		Info: Info{By: "Asana", Description: "Read workspaces, projects and tasks", Category: CatProjects, Icon: IconList, Scopes: []string{"Read projects, tasks and comments"}},
		API:  "https://app.asana.com/api/1.0",
		Endpoint: oauth2.Endpoint{AuthURL: "https://app.asana.com/-/oauth_authorize", TokenURL: "https://app.asana.com/-/oauth_token",
			AuthStyle: oauth2.AuthStyleInParams},
		RevokeBy: &Req{Method: POST, URL: "https://app.asana.com/-/oauth_revoke", Form: Q{"client_id": "{client/id}", "client_secret": "{client/secret}", "token": "{refresh}"}},
		Scopes:   []string{"users:read", "workspaces:read", "projects:read", "tasks:read", "stories:read"},
		Optional: []string{"webhooks:read", "webhooks:write", "webhooks:delete"},
		Who:      &Who{Req: Req{URL: "/users/me?opt_fields=email"}, Names: []Path{"data/email"}, Lower: true},
		Actions: map[string]*Action{
			"workspaces": Call[struct{}, []asanaWorkspace](Plan{
				Doc:   "The workspaces and organizations you are in: [{ gid, name }]",
				Steps: []Step{{As: "got", Req: Req{URL: "/workspaces?limit=100"}}},
				Out:   "{got/data}",
			}),
			"projects": Call[asanaIn, []asanaProject](Plan{
				Doc:   "The projects of a workspace that are not archived: [{ gid, name, owner, due_on, permalink_url }]",
				Steps: []Step{{As: "got", Req: Req{URL: "/projects", Query: Q{"archived": "false", "limit": "100", "opt_fields": "name,owner.name,due_on,permalink_url", "workspace": "{workspace}"}}}},
				Out:   "{got/data}",
			}),
			"tasks": Call[asanaFind, []asanaTask](Plan{
				Doc: "Tasks of a project, or the ones assigned to you in a workspace, open ones only unless all: " +
					"[{ gid, name, assignee, due_on, completed, completed_at, modified_at, permalink_url }]",
				Clamp: map[string][2]int{"limit": {50, 100}},
				Steps: []Step{
					{Unless: "project", Only: Test{At: "workspace", Off: true}, Fail: "pass project, or workspace for your own tasks"},
					{Unless: "project", As: "mine", In: "{workspace}"},
					{Unless: "project", As: "me", In: "me"},
					{Unless: "all", As: "open", In: "now"},
					{As: "got", Req: Req{URL: "/tasks", Query: Q{"limit": "{limit}", "opt_fields": "name,assignee.name,due_on,completed,completed_at,modified_at,permalink_url",
						"project": "{project}", "workspace": "{mine}", "assignee": "{me}", "completed_since": "{open}"}}},
				},
				Out: "{got/data}",
			}),
			"task": Call[asanaOne, asanaFull](Plan{
				Doc: "One task with its description and comments: { task: { gid, name, notes, assignee, due_on, completed, projects, permalink_url, ... }, comments: [{ created_by, text, created_at }] }",
				Steps: []Step{
					{As: "task", Req: Req{URL: "/tasks/{id}"}},
					{As: "stories", Req: Req{URL: "/tasks/{id}/stories?limit=100&opt_fields=type,created_by.name,text,created_at"}},
					{As: "comments", Each: "stories/data", Only: Test{At: "type", Is: "comment"}, In: Obj{"gid": "{gid}", "created_by": "{created_by}", "text": "{text}", "created_at": "{created_at}"}},
				},
				Out: Obj{"task": "{task/data}", "comments": "{comments|[]}"},
			}),
		},
		Listen: &Listen{Renew: 6 * 24 * time.Hour, Lasts: 7 * 24 * time.Hour,
			Over: &Parts{Req: Req{URL: "/workspaces", Query: asanaAll}, Items: "data", Page: asanaPage,
				Under: &Parts{Req: Req{URL: "/projects", Query: Q{"archived": "false", "opt_fields": "name", "workspace": "{up/gid}", "limit": "100", "offset": "{page}"}}, Items: "data", Page: asanaPage, ID: "{gid}"}},
			Mint:     "{conn}/{part}/{nonce}",
			Watch:    Req{Method: POST, URL: "/webhooks", Body: Obj{"data": Obj{"resource": "{part}", "target": "{hook}"}}},
			Resource: "{answer/data/gid}",
			OfPart:   &Reads{Of: "{key}", Find: `^[^/]*/([^/]*)/`},
			Extend:   &Req{URL: "/webhooks/{resource}?opt_fields=active"}, Alive: Test{At: "data/active", Is: "true"},
			Unwatch: &Req{Method: DELETE, URL: "/webhooks/{resource}"},
			Hello:   []Hello{{Header: "X-Hook-Secret"}},
			Kept:    &Kept{ByQuery: true},
			Signs:   []Sign{{Header: "X-Hook-Signature", Kept: true, Enc: Hex, Of: "{body}"}},
			Only:    Test{At: "events/0"},
		},
	})
}

var (
	asanaAll  = Q{"opt_fields": "name", "limit": "100", "offset": "{page}"}
	asanaPage = Cursor{At: "next_page/offset"}
)

type (
	asanaIn struct {
		Workspace string `json:"workspace" jsonschema:"The workspace's gid, from workspaces"`
	}
	asanaFind struct {
		Project   string `json:"project,omitempty" jsonschema:"A project's gid, for its tasks"`
		Workspace string `json:"workspace,omitempty" jsonschema:"A workspace's gid, for the tasks assigned to you there, without project"`
		All       bool   `json:"all,omitempty" jsonschema:"Completed tasks too"`
		Limit     int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 100; 50 when left out"`
	}
	asanaOne struct {
		ID string `json:"id" jsonschema:"The task's gid"`
	}
	asanaWorkspace struct {
		GID  string `json:"gid"`
		Name string `json:"name"`
	}
	asanaProject struct {
		GID       string `json:"gid"`
		Name      string `json:"name"`
		Owner     any    `json:"owner,omitempty"`
		DueOn     string `json:"due_on,omitempty"`
		Permalink string `json:"permalink_url,omitempty"`
	}
	asanaTask struct {
		GID         string `json:"gid"`
		Name        string `json:"name"`
		Assignee    any    `json:"assignee,omitempty"`
		DueOn       string `json:"due_on,omitempty"`
		Completed   bool   `json:"completed,omitempty"`
		CompletedAt string `json:"completed_at,omitempty"`
		ModifiedAt  string `json:"modified_at,omitempty"`
		Permalink   string `json:"permalink_url,omitempty"`
	}
	asanaComment struct {
		CreatedBy any    `json:"created_by,omitempty"`
		Text      string `json:"text,omitempty"`
		CreatedAt string `json:"created_at,omitempty"`
	}
	asanaFull struct {
		Task     map[string]any `json:"task"`
		Comments []asanaComment `json:"comments"`
	}
)
