package connect

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// Asana, read only: workspaces, projects and tasks. It reads ARCHIVE_ASANA, an app of the Asana developer
// console distributed to any workspace, whose redirect URLs list <API>/connect/asana/callback and whose OAuth
// permission scopes include the Scopes here; with webhooks:read, webhooks:write and webhooks:delete too, and
// ?push=1 on the variable, it asks for those, which its pushes need (asanaPush). It syncs nothing until its
// tasks have tables in archive.sql. Asana's MCP server lets no client register itself, so
// this is its REST API. Revoking takes the client's credentials with the refresh token.
func init() {
	Register(&Connector{
		ID: "asana", Name: "Asana", Integration: true, Env: "ARCHIVE_ASANA", Options: "?push=1 once the app has the webhooks scopes", Host: "https://app.asana.com",
		Endpoint: oauth2.Endpoint{AuthURL: "https://app.asana.com/-/oauth_authorize", TokenURL: "https://app.asana.com/-/oauth_token",
			AuthStyle: oauth2.AuthStyleInParams},
		RevokeWith: func(rv *Revocation) error {
			id, secret := rv.Client()
			return rv.Form("https://app.asana.com/-/oauth_revoke", nil, url.Values{"client_id": {id}, "client_secret": {secret}, "token": {rv.Refresh}})
		},
		Scopes:   []string{"users:read", "workspaces:read", "projects:read", "tasks:read", "stories:read"},
		Optional: []string{"webhooks:read", "webhooks:write", "webhooks:delete"},
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
		Push: asanaPush,
	})
}

// asanaPage is a page of a list of Asana's: its data, and the offset of the next.
type asanaPage struct {
	Data []map[string]any `json:"data"`
	Next *struct {
		Offset string `json:"offset"`
	} `json:"next_page"`
}

func (p *asanaPage) next() string {
	if p.Next == nil {
		return ""
	}
	return p.Next.Offset
}

// asanaAll is every page of the list at path.
func asanaAll(c *Client, path string) ([]map[string]any, error) {
	var out []map[string]any
	for offset := ""; ; {
		u := asanaAPI + path + "&limit=100"
		if offset != "" {
			u += "&offset=" + url.QueryEscape(offset)
		}
		var p asanaPage
		if err := c.Get(u, &p); err != nil {
			return nil, err
		}
		out = append(out, p.Data...)
		if offset = p.next(); offset == "" {
			return out, nil
		}
	}
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

// asanaPush is a webhook on each project of the account that is not archived, pushing to Archive's URL with a
// key of the connection's, the project and a nonce, which only Asana learns. Asana creates one only once its
// handshake, posted to that URL with X-Hook-Secret, is answered with the same header; Archive keeps that secret
// by the key, and a push after is signed with it: X-Hook-Signature, the hex HMAC-SHA256 of the body. Asana
// deletes a webhook that fails for a day, so the first sync a week after checks each is still there. A push
// without events is Asana's heartbeat, and starts nothing.
var asanaPush = &Push{Renew: 6 * 24 * time.Hour,
	Watch: func(c *Client, h Hook, old []Watch) ([]Watch, error) {
		have := map[string]Watch{}
		for _, o := range old {
			var w struct {
				Data struct {
					Active bool `json:"active"`
				} `json:"data"`
			}
			if err := c.Get(asanaAPI+"/webhooks/"+url.PathEscape(o.Resource)+"?opt_fields=active", &w); err == nil && w.Data.Active {
				o.Expires = time.Now().Add(7 * 24 * time.Hour)
				have[asanaProject(o.Key)] = o
			}
		}
		ws, err := asanaAll(c, "/workspaces?opt_fields=name")
		if err != nil {
			return nil, err
		}
		var out []Watch
		for _, w := range ws {
			ps, err := asanaAll(c, "/projects?archived=false&opt_fields=name&workspace="+url.QueryEscape(str(w, "gid")))
			if err != nil {
				return nil, err
			}
			for _, p := range ps {
				gid := str(p, "gid")
				if o, ok := have[gid]; ok {
					out = append(out, o)
					continue
				}
				key := h.ID + "/" + gid + "/" + nonce()[:16]
				var made struct {
					Data struct {
						GID string `json:"gid"`
					} `json:"data"`
				}
				if err := c.Do("POST", asanaAPI+"/webhooks", map[string]any{"data": map[string]string{"resource": gid, "target": h.URL(key)}}, &made); err != nil {
					return out, err
				}
				out = append(out, Watch{Key: key, Resource: made.Data.GID, Expires: time.Now().Add(7 * 24 * time.Hour)})
			}
		}
		return out, nil
	},
	Unwatch: func(c *Client, w Watch) error {
		return c.Do("DELETE", asanaAPI+"/webhooks/"+url.PathEscape(w.Resource), nil, nil)
	},
	Hear: func(p *Provider, r *http.Request, at string, body []byte, secret func(string) string) (*Heard, error) {
		key := r.URL.Query().Get("k")
		if s := r.Header.Get("X-Hook-Secret"); s != "" {
			if key == "" || secret(key) != "" {
				return nil, forged("a handshake for a webhook that has one")
			}
			return &Heard{Status: http.StatusOK, Header: http.Header{"X-Hook-Secret": {s}}, Keep: map[string]string{key: s}}, nil
		}
		kept := secret(key)
		if kept == "" {
			return &Heard{}, nil
		}
		if !same(r.Header.Get("X-Hook-Signature"), hexMAC(kept, string(body))) {
			return nil, forged("X-Hook-Signature")
		}
		var e struct {
			Events []any `json:"events"`
		}
		if err := readJSON(body, &e); err != nil {
			return nil, err
		}
		if len(e.Events) == 0 {
			return &Heard{}, nil
		}
		return &Heard{Keys: []string{key}}, nil
	}}

// asanaProject is the project of an Asana watch's key, <connection>/<project>/<nonce>.
func asanaProject(key string) string {
	parts := strings.Split(key, "/")
	if len(parts) != 3 {
		return ""
	}
	return parts[1]
}
