package connect

import (
	"time"

	"golang.org/x/oauth2"
)

func init() {
	Register(&Connector{
		ID: "clickup", Name: "ClickUp", Integration: true,
		Info: Info{By: "ClickUp", Description: "Read workspaces, lists, tasks and comments, and add or update tasks and comment when you ask", Category: CatProjects, Icon: IconList,
			Scopes: []string{"Read the workspaces you pick, their spaces, lists, tasks and comments", "Add and change tasks and post comments when you ask"}},
		API: "https://api.clickup.com/api/v2",
		Endpoint: oauth2.Endpoint{AuthURL: "https://app.clickup.com/api", TokenURL: "https://api.clickup.com/api/v2/oauth/token",
			AuthStyle: oauth2.AuthStyleInParams},
		Who: &Who{Req: Req{URL: "/user"}, Names: []Path{"user/email", "user/username"}, Lower: true},
		Actions: map[string]*Action{
			"workspaces": Call[struct{}, []clickupWorkspace](Plan{
				Doc:   "The workspaces you let Archive reach, with their members: [{ id, name, members: [{ user: { id, username, email } }] }]",
				Steps: []Step{{As: "got", Req: Req{URL: "/team"}}},
				Out:   "{got/teams|[]}",
			}),
			"spaces": Call[clickupIn, []map[string]any](Plan{
				Doc:   "The spaces of a workspace that are not archived, with their statuses: [{ id, name, statuses, private }]",
				Steps: []Step{{As: "got", Req: Req{URL: "/team/{workspace}/space", Query: Q{"archived": "false"}}}},
				Out:   "{got/spaces|[]}",
			}),
			"lists": Call[clickupSpace, clickupLists](Plan{
				Doc:   "The folders of a space, each with its lists, and its lists in no folder: { folders: [{ id, name, lists: [{ id, name }] }], lists: [{ id, name, task_count }] }",
				Steps: []Step{{As: "folders", Req: Req{URL: "/space/{space}/folder", Query: Q{"archived": "false"}}}, {As: "lists", Req: Req{URL: "/space/{space}/list", Query: Q{"archived": "false"}}}},
				Out:   Obj{"folders": "{folders/folders|[]}", "lists": "{lists/lists|[]}"},
			}),
			"tasks": Call[clickupFind, clickupFound](Plan{
				Doc: "Tasks of a workspace, maybe only of a list, assigned to someone or changed since a time, open ones only unless all, last changed first, 100 a page: " +
					"{ tasks: [{ id, custom_id, name, status, assignees, due_date, date_updated, list, folder, space, url, ... }], last } (pass page + 1 for more, until last)",
				Steps: []Step{{As: "got", Req: Req{URL: "/team/{workspace}/task", Query: Q{"page": "{page|0}", "subtasks": "true", "include_closed": "{all}",
					"list_ids[]": "{list}", "assignees[]": "{assignee}", "date_updated_gt": "{since}"}}}},
				Out: Obj{"tasks": "{got/tasks|[]}", "last": "{got/last_page|true}"},
			}),
			"task": Call[clickupOne, clickupFull](Plan{
				Doc: "One task with its description in Markdown, subtasks and comments: { task: { id, name, markdown_description, status, assignees, " +
					"due_date, list, url, subtasks, ... }, comments: [{ id, comment_text, user, date, resolved }] }",
				Steps: []Step{
					{As: "task", Req: Req{URL: "/task/{id}", Query: Q{"include_markdown_description": "true", "include_subtasks": "true"}}},
					{As: "comments", Req: Req{URL: "/task/{id}/comment"}},
				},
				Out: Obj{"task": "{task}", "comments": "{comments/comments|[]}"},
			}),
			"create": Call[clickupNew, map[string]any](Plan{
				Doc: "Add a task to a list, maybe as a subtask of another: the task, as task returns one",
				Steps: []Step{{Req: Req{Method: POST, URL: "/list/{list}/task", Body: Obj{"name": "{name}", "markdown_content": "{description}", "status": "{status}",
					"priority": "{priority}", "assignees": "{assignees}", "due_date": "{due}", "start_date": "{start}", "tags": "{tags}", "parent": "{parent}"}}}},
			}),
			"update": Call[clickupEdit, map[string]any](Plan{
				Doc: "Change a task: only what you give changes. Returns the task, as task returns one",
				Steps: []Step{{Req: Req{Method: PUT, URL: "/task/{id}", Body: Obj{"name": "{name}", "markdown_content": "{description}", "status": "{status}",
					"priority": "{priority}", "due_date": "{due}", "start_date": "{start}", "assignees": Obj{"add": "{add|[]}", "rem": "{remove|[]}"}}}}},
			}),
			"comment": Call[clickupSay, map[string]any](Plan{
				Doc:   "Post a comment on a task, telling everyone on it when notify: { id, hist_id, date }",
				Steps: []Step{{Req: Req{Method: POST, URL: "/task/{id}/comment", Body: Obj{"comment_text": "{text}", "notify_all": "{notify|false}"}}}},
			}),
		},
		Listen: &Listen{Renew: 6 * 24 * time.Hour, Lasts: 7 * 24 * time.Hour, Over: clickupWorkspaces,
			Mint:     "{conn}/{part}/{nonce}",
			Keep:     "{answer/webhook/secret}",
			Signs:    []Sign{{Header: "X-Signature", Kept: true, Enc: Hex, Of: "{body}"}},
			Watch:    Req{Method: POST, URL: "/team/{part}/webhook", Body: Obj{"endpoint": "{hook}", "events": clickupEvents}},
			Resource: "{answer/id}",
			OfPart:   &Reads{Of: "{key}", Find: `^[^/]*/([^/]*)/`},
			Extend:   &Req{Method: PUT, URL: "/webhook/{resource}", Body: Obj{"events": clickupEvents, "status": "active"}},
			Unwatch:  &Req{Method: DELETE, URL: "/webhook/{resource}"},
			Kept:     &Kept{ByQuery: true},
			Only:     Test{At: "event"},
		},
		Sync: []*Sync{{Name: "tasks", Gap: time.Second, Parts: clickupWorkspaces,
			Req:   Req{URL: "/team/{part}/task", Query: clickupAll},
			Items: "tasks", Updated: "date_updated", Page: Offset{Size: 100, Step: 1},
			Change: SinceTime{Ms: true, Req: Req{URL: "/team/{part}/task", Query: clickupSince}},
			Set:    Obj{"workspace": "{in/name}"},
			Doc: "Every task of each workspace that is not archived, open or closed, subtasks too, by its id, with its status, assignees, dates, tags and " +
				"its space, folder and list: listed whole the first time and a week after, 100 a page and a workspace at a time, which deletes " +
				"those gone, and in between those changed since the sync before.",
			To: Map{Table: "work_item",
				Cols: map[string]SQL{"title": "data->>'name'", "type": "data#>>'{list,name}'", "status": "data#>>'{status,status}'",
					"assignee_name": "data#>>'{assignees,0,username}'",
					"starts_on":     "archive.sync_at(data->>'start_date')::date", "due_on": "archive.sync_at(data->>'due_date')::date",
					"done_on":     "archive.sync_at(coalesce(data->>'date_done', data->>'date_closed'))::date",
					"description": "archive.sync_clip(data->>'text_content', 65536)", "url": "data->>'url'",
					"made_at": "archive.sync_at(data->>'date_created')", "edited_at": "archive.sync_at(data->>'date_updated')"},
				People: map[string]SQL{"assignee": "data#>>'{assignees,0,email}'"}}}},
	})
}

var (
	clickupWorkspaces = &Parts{Req: Req{URL: "/team"}, Items: "teams", Vals: Obj{"name": "{name}"}}
	clickupAll        = Q{"page": "{page|0}", "subtasks": "true", "include_closed": "true"}
	clickupSince      = Q{"page": "{page|0}", "subtasks": "true", "include_closed": "true", "date_updated_gt": "{since}", "order_by": "updated", "reverse": "true"}
	clickupEvents     = []string{"taskCreated", "taskUpdated", "taskDeleted", "taskMoved", "taskStatusUpdated", "taskAssigneeUpdated",
		"taskDueDateUpdated", "taskPriorityUpdated", "taskTagUpdated", "taskCommentPosted", "taskCommentUpdated"}
)

type (
	clickupIn struct {
		Workspace string `json:"workspace" jsonschema:"The workspace's id, from workspaces"`
	}
	clickupSpace struct {
		Space string `json:"space" jsonschema:"The space's id, from spaces"`
	}
	clickupFind struct {
		Workspace string `json:"workspace" jsonschema:"The workspace's id, from workspaces"`
		List      string `json:"list,omitempty" jsonschema:"A list's id, from lists, for its tasks only"`
		Assignee  string `json:"assignee,omitempty" jsonschema:"A member's user id, from workspaces, for the tasks assigned to them"`
		Since     int64  `json:"since,omitempty" jsonschema:"Only those changed after this time, in Unix milliseconds"`
		All       bool   `json:"all,omitempty" jsonschema:"Closed tasks too"`
		Page      int    `json:"page,omitempty" jsonschema:"The page, from 0"`
	}
	clickupOne struct {
		ID string `json:"id" jsonschema:"The task's id"`
	}
	clickupNew struct {
		List        string   `json:"list" jsonschema:"The list's id, from lists"`
		Name        string   `json:"name" jsonschema:"The task's name"`
		Description string   `json:"description,omitempty" jsonschema:"What it is about, in Markdown"`
		Status      string   `json:"status,omitempty" jsonschema:"One of its list's statuses, like to do or in progress; the list's first when left out"`
		Priority    int      `json:"priority,omitempty" jsonschema:"1 urgent, 2 high, 3 normal or 4 low"`
		Assignees   []int64  `json:"assignees,omitempty" jsonschema:"The user ids of who to assign it to, from workspaces"`
		Due         int64    `json:"due,omitempty" jsonschema:"When it is due, in Unix milliseconds"`
		Start       int64    `json:"start,omitempty" jsonschema:"When it starts, in Unix milliseconds"`
		Tags        []string `json:"tags,omitempty" jsonschema:"Tags of its space to put on it"`
		Parent      string   `json:"parent,omitempty" jsonschema:"A task's id, to make it a subtask of that task"`
	}
	clickupEdit struct {
		ID          string  `json:"id" jsonschema:"The task's id"`
		Name        string  `json:"name,omitempty" jsonschema:"Its new name"`
		Description string  `json:"description,omitempty" jsonschema:"Its new description, in Markdown"`
		Status      string  `json:"status,omitempty" jsonschema:"One of its list's statuses, like in progress or complete"`
		Priority    int     `json:"priority,omitempty" jsonschema:"1 urgent, 2 high, 3 normal or 4 low"`
		Due         int64   `json:"due,omitempty" jsonschema:"When it is due, in Unix milliseconds"`
		Start       int64   `json:"start,omitempty" jsonschema:"When it starts, in Unix milliseconds"`
		Add         []int64 `json:"add,omitempty" jsonschema:"The user ids of who to assign it to too"`
		Remove      []int64 `json:"remove,omitempty" jsonschema:"The user ids of who to take off it"`
	}
	clickupSay struct {
		ID     string `json:"id" jsonschema:"The task's id"`
		Text   string `json:"text" jsonschema:"The comment"`
		Notify bool   `json:"notify,omitempty" jsonschema:"Notify everyone on the task, the creator too"`
	}
	clickupWorkspace struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Members []any  `json:"members,omitempty"`
	}
	clickupLists struct {
		Folders []map[string]any `json:"folders"`
		Lists   []map[string]any `json:"lists"`
	}
	clickupFound struct {
		Tasks []map[string]any `json:"tasks"`
		Last  bool             `json:"last" jsonschema:"There are no more pages"`
	}
	clickupFull struct {
		Task     map[string]any   `json:"task"`
		Comments []map[string]any `json:"comments"`
	}
)
