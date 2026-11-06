package connect

import (
	"time"

	"golang.org/x/oauth2"
)

func init() {
	Register(&Connector{
		ID: "github", Name: "GitHub", Integration: true, Env: "ARCHIVE_GITHUB_APP", Options: "?hook=<webhook secret>",
		Info: Info{By: "GitHub", Description: "Read issues, pull requests and files of your repositories", Category: CatCode, Icon: IconBranch, Scopes: []string{"Read repositories you install Archive on"}},
		API:  "https://api.github.com",
		Endpoint: oauth2.Endpoint{AuthURL: "https://github.com/login/oauth/authorize", TokenURL: "https://github.com/login/oauth/access_token",
			AuthStyle: oauth2.AuthStyleInParams},
		RevokeBy: &Req{Method: DELETE, URL: "https://api.github.com/applications/{client/id}/grant",
			Header: map[string][]string{"Authorization": {"{basic}"}}, Body: Obj{"access_token": "{access}"}},
		Who:  &Who{Req: Req{URL: "/user"}, Names: []Path{"login"}},
		Busy: []string{"rate limit"},
		Actions: map[string]*Action{
			"search": Call[githubFind, githubFound](Plan{
				Doc: "Find issues and pull requests with GitHub's search syntax, like { query: \"repo:acme/web is:open label:bug\" }, best match first: " +
					"{ total, items: [{ number, title, state, html_url, repository_url, user, labels, created_at, updated_at, pull_request? }] }",
				Clamp: map[string][2]int{"limit": {30, 100}, "page": {1, 1 << 30}},
				Steps: []Step{{As: "got", Req: Req{URL: "/search/issues", Query: Q{"q": "{query}", "per_page": "{limit}", "page": "{page}"}}}},
				Out:   Obj{"total": "{got/total_count|0}", "items": "{got/items|[]}"},
			}),
			"issue": Call[githubOne, githubIssue](Plan{
				Doc: "One issue or pull request with its comments: { issue: { number, title, state, body, user, labels, ... }, comments: [{ user, body, created_at }] }",
				Steps: []Step{
					{As: "issue", Req: Req{URL: "/repos/{~repo}/issues/{number}"}},
					{As: "comments", Req: Req{URL: "/repos/{~repo}/issues/{number}/comments?per_page=100"}},
				},
				Out: Obj{"issue": "{issue}", "comments": "{comments}"},
			}),
			"file": Call[githubPath, any](Plan{
				Doc: "A file of a repository as text, or a directory's entries: { path, sha, size, text } for a file, [{ name, path, type, size }] for a directory",
				Steps: []Step{
					{As: "got", Req: Req{URL: "/repos/{~repo}/contents/{~path|}", Query: Q{"ref": "{ref}"}}},
					{Unless: "got/path", Each: "got", In: Obj{"name": "{name}", "path": "{path}", "type": "{type}", "size": "{size}"}},
					{If: "got/path", Only: Test{At: "got/encoding", Is: "base64", Off: true}, Fail: "{got/path} is over 1 MB or not a file GitHub sends the text of"},
					{If: "got/path", As: "text", Codec: Base64Text, In: "{got/content}"},
					{If: "got/path", In: Obj{"path": "{got/path}", "sha": "{got/sha}", "size": "{got/size}", "text": "{text|}"}},
				},
			}),
		},
		Listen: &Listen{Renew: 24 * time.Hour, Lasts: 48 * time.Hour,
			Over: &Parts{Req: Req{URL: "/user/installations", Query: githubAll}, Items: "installations", Page: githubPage},
			Key:  "{part}", Keys: []Path{"installation/id"},
			Signs: []Sign{{Header: "X-Hub-Signature-256", Hook: true, Enc: Hex, Pre: "sha256=", Of: "{body}"}},
		},
		Sync: []*Sync{{Name: "issues", Gap: time.Second, Parts: githubRepos, ID: "id", Updated: "updated_at", Page: githubPage, Pass: []int{404, 410, 451},
			Req:    Req{URL: "/repos/{~part}/issues", Query: Q{"state": "all", "sort": "created", "direction": "desc", "per_page": "100", "page": "{page}"}},
			Change: SinceTime{Req: Req{URL: "/repos/{~part}/issues", Query: Q{"state": "all", "sort": "updated", "direction": "desc", "per_page": "100", "page": "{page}", "since": "{since}"}}},
			Doc: "Every issue and pull request of each repository, open or closed, by its id: listed whole the first time and a week after, 100 a page and a repository at a time, " +
				"which deletes those gone, and in between those updated since the sync before. A repository with issues turned off is left out, its pull requests too.",
			To: Map{Table: "work_item",
				Cols: map[string]SQL{"title": "data->>'title'", "type": "CASE WHEN data->'pull_request' IS NOT NULL THEN 'pull request' ELSE 'issue' END", "status": "data->>'state'",
					"assignee_name": "data#>>'{assignee,login}'", "done_on": "archive.sync_at(data->>'closed_at')::date", "description": "archive.sync_clip(data->>'body', 65536)",
					"url": "data->>'html_url'", "made_at": "archive.sync_at(data->>'created_at')", "edited_at": "archive.sync_at(data->>'updated_at')"}}}},
	})
}

var (
	githubAll   = Q{"per_page": "100", "page": "{page}"}
	githubPage  = Offset{From: 1, Size: 100, Step: 1}
	githubRepos = &Parts{Req: Req{URL: "/user/installations", Query: githubAll}, Items: "installations", Page: githubPage,
		Under: &Parts{Req: Req{URL: "/user/installations/{up/id}/repositories", Query: githubAll}, Items: "repositories", Page: githubPage, ID: "{full_name}", Skip: []Test{{At: "has_issues", Is: "false"}}}}
)

type (
	githubFind struct {
		Query string `json:"query" jsonschema:"GitHub search, like org:acme is:pr is:open review-requested:@me"`
		Limit int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 100; 30 when left out"`
		Page  int    `json:"page,omitempty" jsonschema:"The page of results, from 1"`
	}
	githubOne struct {
		Repo   string `json:"repo" jsonschema:"owner/name, like acme/web"`
		Number int    `json:"number" jsonschema:"The issue's or pull request's number"`
	}
	githubPath struct {
		Repo string `json:"repo" jsonschema:"owner/name, like acme/web"`
		Path string `json:"path,omitempty" jsonschema:"The file's or directory's path, like docs/README.md; the top directory when left out"`
		Ref  string `json:"ref,omitempty" jsonschema:"A branch, tag or commit; the default branch when left out"`
	}
	githubFound struct {
		Total int              `json:"total"`
		Items []map[string]any `json:"items"`
	}
	githubIssue struct {
		Issue    map[string]any   `json:"issue"`
		Comments []map[string]any `json:"comments"`
	}
)
