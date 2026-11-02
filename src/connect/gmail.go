package connect

import (
	"strings"

	"golang.org/x/oauth2"
)

// Gmail, read only. It reads ARCHIVE_GOOGLE, the Google client sign-in uses too: its authorized redirect URIs
// list <API>/connect/gmail/callback, like https://api.archive.dev/connect/gmail/callback, and its consent screen asks for
// gmail.readonly, a restricted scope. Google sends a refresh token only with access_type=offline, and again on
// a later connect only with prompt=consent.
func init() {
	Register(&Connector{
		ID: "gmail", Name: "Gmail", Env: "ARCHIVE_GOOGLE", Host: "https://accounts.google.com",
		Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.google.com/o/oauth2/v2/auth", TokenURL: "https://oauth2.googleapis.com/token",
			AuthStyle: oauth2.AuthStyleInParams},
		Revoke: "https://oauth2.googleapis.com/revoke",
		Scopes: []string{"https://www.googleapis.com/auth/gmail.readonly"},
		Params: map[string]string{"access_type": "offline", "prompt": "consent"},
		Account: func(get func(string, any) error) (string, error) {
			var p struct {
				EmailAddress string `json:"emailAddress"`
			}
			err := get("https://gmail.googleapis.com/gmail/v1/users/me/profile", &p)
			return strings.ToLower(p.EmailAddress), err
		},
	})
}
