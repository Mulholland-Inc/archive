package connect

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/oauth2"
)

// GitHub, read only, as a GitHub App acting for the person who connects. It reads ARCHIVE_GITHUB_APP, the
// app's client id and secret (not its app id or private key): a GitHub App, not the OAuth App sign-in uses,
// because only a GitHub App's user tokens expire and come with a refresh token (8 hours, and 6 months,
// replaced on every refresh), and its permissions are fine-grained: Metadata, Contents, Issues and Pull
// requests, all read-only, with its callback URL <API>/connect/github/callback and "Expire user
// authorization tokens" on. A GitHub App has no scopes; it reads the repositories its installations reach,
// so a person installs it on their account or organization too. A grant is revoked with an access token.
func init() {
	Register(&Connector{
		ID: "github", Name: "GitHub", Env: "ARCHIVE_GITHUB_APP", Host: "https://github.com",
		Endpoint: oauth2.Endpoint{AuthURL: "https://github.com/login/oauth/authorize", TokenURL: "https://github.com/login/oauth/access_token",
			AuthStyle: oauth2.AuthStyleInParams},
		RevokeWith: func(rv *Revocation) error {
			at, err := rv.Access()
			if err != nil {
				return err
			}
			id, secret := rv.Client()
			return rv.Send("DELETE", githubAPI+"/applications/"+url.PathEscape(id)+"/grant", http.Header{"Authorization": {basic(id, secret)}}, map[string]string{"access_token": at})
		},
		Account: func(c *Client) (string, error) {
			var u struct {
				Login string `json:"login"`
			}
			err := c.Get(githubAPI+"/user", &u)
			return u.Login, err
		},
		Actions: map[string]*Action{
			"search": Act("Find issues and pull requests with GitHub's search syntax, like { query: \"repo:acme/web is:open label:bug\" }, best match first: "+
				"{ total, items: [{ number, title, state, html_url, repository_url, user, labels, created_at, updated_at, pull_request? }] }",
				func(c *Client, in struct {
					Query string `json:"query" jsonschema:"GitHub search, like org:acme is:pr is:open review-requested:@me"`
					Limit int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 100; 30 when left out"`
					Page  int    `json:"page,omitempty" jsonschema:"The page of results, from 1"`
				}) (any, error) {
					var out struct {
						Total int   `json:"total_count"`
						Items []any `json:"items"`
					}
					err := c.Get(fmt.Sprintf("%s/search/issues?q=%s&per_page=%d&page=%d", githubAPI, url.QueryEscape(in.Query), clamp(in.Limit, 30, 100), max(in.Page, 1)), &out)
					return map[string]any{"total": out.Total, "items": out.Items}, err
				}),
			"issue": Act("One issue or pull request with its comments: { issue: { number, title, state, body, user, labels, ... }, comments: [{ user, body, created_at }] }",
				func(c *Client, in struct {
					Repo   string `json:"repo" jsonschema:"owner/name, like acme/web"`
					Number int    `json:"number" jsonschema:"The issue's or pull request's number"`
				}) (any, error) {
					var issue, comments any
					base := fmt.Sprintf("%s/repos/%s/issues/%d", githubAPI, githubRepo(in.Repo), in.Number)
					if err := c.Get(base, &issue); err != nil {
						return nil, err
					}
					err := c.Get(base+"/comments?per_page=100", &comments)
					return map[string]any{"issue": issue, "comments": comments}, err
				}),
			"file": Act("A file of a repository as text, or a directory's entries: { path, sha, size, text } for a file, [{ name, path, type, size }] for a directory",
				func(c *Client, in struct {
					Repo string `json:"repo" jsonschema:"owner/name, like acme/web"`
					Path string `json:"path,omitempty" jsonschema:"The file's or directory's path, like docs/README.md; the top directory when left out"`
					Ref  string `json:"ref,omitempty" jsonschema:"A branch, tag or commit; the default branch when left out"`
				}) (any, error) {
					var parts []string
					for _, part := range strings.Split(strings.Trim(in.Path, "/"), "/") {
						if part != "" && part != "." && part != ".." {
							parts = append(parts, url.PathEscape(part))
						}
					}
					u := githubAPI + "/repos/" + githubRepo(in.Repo) + "/contents/" + strings.Join(parts, "/")
					if in.Ref != "" {
						u += "?ref=" + url.QueryEscape(in.Ref)
					}
					var raw []byte
					if err := c.Get(u, &raw); err != nil {
						return nil, err
					}
					var f struct {
						Path, SHA, Content, Encoding string
						Size                         int
					}
					if raw = []byte(strings.TrimSpace(string(raw))); len(raw) > 0 && raw[0] == '[' {
						var entries []struct {
							Name string `json:"name"`
							Path string `json:"path"`
							Type string `json:"type"`
							Size int    `json:"size"`
						}
						err := json.Unmarshal(raw, &entries)
						return entries, err
					}
					if err := json.Unmarshal(raw, &f); err != nil {
						return nil, err
					}
					text, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(f.Content, "\n", ""))
					if err != nil || f.Encoding != "base64" {
						return nil, fmt.Errorf("%s is over 1 MB or not a file GitHub sends the text of", f.Path)
					}
					return map[string]any{"path": f.Path, "sha": f.SHA, "size": f.Size, "text": string(text)}, nil
				}),
		},
	})
}

const githubAPI = "https://api.github.com"

// githubRepo is owner/name with each part escaped.
func githubRepo(s string) string {
	owner, name, _ := strings.Cut(s, "/")
	return url.PathEscape(owner) + "/" + url.PathEscape(name)
}
