package connect

import (
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/oauth2"
)

// Asana, read only: workspaces, projects and tasks. It reads ARCHIVE_ASANA, an app of the Asana developer
// console distributed to any workspace, whose redirect URLs list <API>/connect/asana/callback and whose OAuth
// permission scopes include exactly the Scopes here. Asana's MCP server lets no client register itself, so
// this is its REST API. Revoking takes the client's credentials with the refresh token.
func init() {
	Register(&Connector{
		ID: "asana", Name: "Asana", Env: "ARCHIVE_ASANA", Host: "https://app.asana.com",
		Endpoint: oauth2.Endpoint{AuthURL: "https://app.asana.com/-/oauth_authorize", TokenURL: "https://app.asana.com/-/oauth_token",
			AuthStyle: oauth2.AuthStyleInParams},
		RevokeWith: func(rv *Revocation) error {
			id, secret := rv.Client()
			return rv.Form("https://app.asana.com/-/oauth_revoke", nil, url.Values{"client_id": {id}, "client_secret": {secret}, "token": {rv.Refresh}})
		},
		Scopes: []string{"users:read", "workspaces:read", "projects:read", "tasks:read", "stories:read"},
		Account: func(c *Client) (string, error) {
			var me struct {
				Data struct {
					Email string `json:"email"`
				} `json:"data"`
			}
			err := c.Get(asanaAPI+"/users/me?opt_fields=email", &me)
			return strings.ToLower(me.Data.Email), err
		},
		Actions: map[string]*Action{
			"workspaces": Act("The workspaces and organizations you are in: [{ gid, name }]",
				func(c *Client, in struct{}) (any, error) { return asanaData(c, "/workspaces?limit=100") }),
			"projects": Act("The projects of a workspace that are not archived: [{ gid, name, owner, due_on, permalink_url }]",
				func(c *Client, in struct {
					Workspace string `json:"workspace" jsonschema:"The workspace's gid, from workspaces"`
				}) (any, error) {
					return asanaData(c, "/projects?archived=false&limit=100&opt_fields=name,owner.name,due_on,permalink_url&workspace="+url.QueryEscape(in.Workspace))
				}),
			"tasks": Act("Tasks of a project, or the ones assigned to you in a workspace, open ones only unless all: "+
				"[{ gid, name, assignee, due_on, completed, completed_at, modified_at, permalink_url }]",
				func(c *Client, in struct {
					Project   string `json:"project,omitempty" jsonschema:"A project's gid, for its tasks"`
					Workspace string `json:"workspace,omitempty" jsonschema:"A workspace's gid, for the tasks assigned to you there, without project"`
					All       bool   `json:"all,omitempty" jsonschema:"Completed tasks too"`
					Limit     int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 100; 50 when left out"`
				}) (any, error) {
					q := url.Values{"limit": {fmt.Sprint(clamp(in.Limit, 50, 100))}, "opt_fields": {"name,assignee.name,due_on,completed,completed_at,modified_at,permalink_url"}}
					switch {
					case in.Project != "":
						q.Set("project", in.Project)
					case in.Workspace != "":
						q.Set("workspace", in.Workspace)
						q.Set("assignee", "me")
					default:
						return nil, fmt.Errorf("pass project, or workspace for your own tasks")
					}
					if !in.All {
						q.Set("completed_since", "now")
					}
					return asanaData(c, "/tasks?"+q.Encode())
				}),
			"task": Act("One task with its description and comments: { task: { gid, name, notes, assignee, due_on, completed, projects, permalink_url, ... }, comments: [{ created_by, text, created_at }] }",
				func(c *Client, in struct {
					ID string `json:"id" jsonschema:"The task's gid"`
				}) (any, error) {
					task, err := asanaData(c, "/tasks/"+url.PathEscape(in.ID))
					if err != nil {
						return nil, err
					}
					stories, err := asanaData(c, "/tasks/"+url.PathEscape(in.ID)+"/stories?limit=100&opt_fields=type,created_by.name,text,created_at")
					var comments []any
					for _, s := range asList(stories) {
						if m, _ := s.(map[string]any); m["type"] == "comment" {
							delete(m, "type")
							comments = append(comments, m)
						}
					}
					return map[string]any{"task": task, "comments": comments}, err
				}),
		},
	})
}

const asanaAPI = "https://app.asana.com/api/1.0"

// asanaData is the data of an answer of Asana's API.
func asanaData(c *Client, path string) (any, error) {
	var out struct {
		Data any `json:"data"`
	}
	err := c.Get(asanaAPI+path, &out)
	return out.Data, err
}

func asList(v any) []any {
	l, _ := v.([]any)
	return l
}
