package connect

import (
	"time"

	"golang.org/x/oauth2"
)

// GitHub, read only, as a GitHub App acting for the person who connects. It reads ARCHIVE_GITHUB_APP, the
// app's client id and secret (not its app id or private key): a GitHub App, not the OAuth App sign-in uses,
// because only a GitHub App's user tokens expire and come with a refresh token (8 hours, and 6 months,
// replaced on every refresh), and its permissions are fine-grained: Metadata, Contents, Issues and Pull
// requests, all read-only, with its callback URL <API>/connect/github/callback and "Expire user
// authorization tokens" on. A GitHub App has no scopes; it reads the repositories its installations reach,
// so a person installs it on their account or organization too. A grant is revoked with an access token.
// It syncs nothing until its issues are mapped into the tables of archive.sql; its push waits for that too.
//
// It pushes through the app's webhook, set up once on the app: its URL <API>/push/github, its secret the hook
// option of ARCHIVE_GITHUB_APP, and its events Issues, Pull request and Repository. A push names the
// installation it is for, and an account is the installations its token reaches, read again on a sync a day
// after. X-Hub-Signature-256 is sha256= and the HMAC-SHA256 of the body with the secret, in hex.
func init() {
	Register(&Connector{
		ID: "github", Name: "GitHub", Integration: true, Env: "ARCHIVE_GITHUB_APP", Options: "?hook=<webhook secret>",
		Info: Info{By: "GitHub", Description: "Read issues, pull requests and files of your repositories", Category: CatCode, Icon: IconBranch, Scopes: []string{"Read repositories you install Archive on"}},
		API:  "https://api.github.com",
		Endpoint: oauth2.Endpoint{AuthURL: "https://github.com/login/oauth/authorize", TokenURL: "https://github.com/login/oauth/access_token",
			AuthStyle: oauth2.AuthStyleInParams},
		RevokeBy: &Req{Method: DELETE, URL: "https://api.github.com/applications/{client/id}/grant",
			Header: map[string][]string{"Authorization": {"{basic}"}}, Body: Obj{"access_token": "{access}"}},
		Who: &Who{Req: Req{URL: "/user"}, Names: []Path{"login"}},
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
			Over: &Parts{Req: Req{URL: "/user/installations", Query: Q{"per_page": "100", "page": "{page}"}}, Items: "installations", Page: Offset{From: 1, Size: 100, Step: 1}},
			Key:  "{part}", Keys: []Path{"installation/id"},
			Signs: []Sign{{Header: "X-Hub-Signature-256", Hook: true, Enc: Hex, Pre: "sha256=", Of: "{body}"}},
		},
	})
}

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
