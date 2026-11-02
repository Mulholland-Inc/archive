package connect

import (
	"cmp"
	"fmt"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// Google Drive, read only, with ARCHIVE_GOOGLE like Gmail: the client's authorized redirect URIs list
// <API>/connect/gdrive/callback, and its consent screen asks for drive.readonly, a restricted scope.
func init() {
	Register(&Connector{
		ID: "gdrive", Name: "Google Drive", Env: "ARCHIVE_GOOGLE", Host: "https://accounts.google.com",
		Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.google.com/o/oauth2/v2/auth", TokenURL: "https://oauth2.googleapis.com/token",
			AuthStyle: oauth2.AuthStyleInParams},
		Revoke: "https://oauth2.googleapis.com/revoke",
		Scopes: []string{"https://www.googleapis.com/auth/drive.readonly"},
		Params: map[string]string{"access_type": "offline", "prompt": "consent"},
		Account: func(c *Client) (string, error) {
			var a struct {
				User struct {
					EmailAddress string `json:"emailAddress"`
				} `json:"user"`
			}
			err := c.Get(driveAPI+"/about?fields=user", &a)
			return strings.ToLower(a.User.EmailAddress), err
		},
		Actions: map[string]*Action{
			"search": Act("Find files in My Drive and the drives shared with you, newest change first, by words in their name or "+
				"text, or by a Drive query: { files: [{ id, name, mimeType, modifiedTime, size, webViewLink, owners }], next } (pass next as page for more)",
				func(c *Client, in struct {
					Text  string `json:"text,omitempty" jsonschema:"Words in the file's name or content"`
					Query string `json:"query,omitempty" jsonschema:"A Drive query instead, like mimeType = 'application/pdf' and modifiedTime > '2026-01-01'"`
					Limit int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 100; 25 when left out"`
					Page  string `json:"page,omitempty" jsonschema:"The next of the search before, for the files after it"`
				}) (any, error) {
					q := []string{"trashed = false"}
					if in.Text != "" {
						q = append(q, "fullText contains '"+strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(in.Text)+"'")
					}
					if in.Query != "" {
						q = append(q, "("+in.Query+")")
					}
					v := url.Values{"q": {strings.Join(q, " and ")}, "pageSize": {fmt.Sprint(clamp(in.Limit, 25, 100))}, "supportsAllDrives": {"true"},
						"includeItemsFromAllDrives": {"true"}, "fields": {"nextPageToken,files(" + driveFields + ")"}}
					if in.Text == "" {
						v.Set("orderBy", "modifiedTime desc")
					}
					if in.Page != "" {
						v.Set("pageToken", in.Page)
					}
					var out struct {
						Files []map[string]any `json:"files"`
						Next  string           `json:"nextPageToken"`
					}
					err := c.Get(driveAPI+"/files?"+v.Encode(), &out)
					return map[string]any{"files": out.Files, "next": out.Next}, err
				}),
			"file": Act("One file's details by id: { id, name, mimeType, modifiedTime, size, webViewLink, owners, parents, description }",
				func(c *Client, in struct {
					ID string `json:"id" jsonschema:"The file's id, from search"`
				}) (any, error) {
					var f map[string]any
					err := c.Get(driveAPI+"/files/"+url.PathEscape(in.ID)+"?supportsAllDrives=true&fields="+url.QueryEscape(driveFields+",parents,description"), &f)
					return f, err
				}),
			"export": Act("A file's content as text: a Google Doc as plain text, a Sheet as CSV (its first sheet), a Slides deck as plain text, "+
				"and a text, CSV, JSON or Markdown file as it is; up to 5 MB. Returns { id, name, mimeType, text }",
				func(c *Client, in struct {
					ID   string `json:"id" jsonschema:"The file's id, from search"`
					Type string `json:"type,omitempty" jsonschema:"The MIME type to export a Google file as, like text/html or text/markdown, in place of the default"`
				}) (any, error) {
					var f struct {
						ID       string `json:"id"`
						Name     string `json:"name"`
						MIMEType string `json:"mimeType"`
					}
					if err := c.Get(driveAPI+"/files/"+url.PathEscape(in.ID)+"?supportsAllDrives=true&fields=id,name,mimeType", &f); err != nil {
						return nil, err
					}
					var text string
					var err error
					switch as := driveExports[f.MIMEType]; {
					case strings.HasPrefix(f.MIMEType, "application/vnd.google-apps."):
						if in.Type != "" {
							as = in.Type
						}
						if as == "" {
							return nil, fmt.Errorf("%s is a %s, which has no text to export", f.Name, f.MIMEType)
						}
						err = c.Get(driveAPI+"/files/"+url.PathEscape(f.ID)+"/export?mimeType="+url.QueryEscape(as), &text)
						f.MIMEType = as
					case strings.HasPrefix(f.MIMEType, "text/") || f.MIMEType == "application/json":
						err = c.Get(driveAPI+"/files/"+url.PathEscape(f.ID)+"?alt=media&supportsAllDrives=true", &text)
					default:
						return nil, fmt.Errorf("%s is a %s file, not text; only Google files and text files export", f.Name, f.MIMEType)
					}
					return map[string]any{"id": f.ID, "name": f.Name, "mimeType": f.MIMEType, "text": text}, err
				}),
		},
		Streams: []*Stream{{Name: "files", Every: 5 * time.Minute, Pull: driveFiles,
			Doc: "Every file in My Drive, shared with you and in your shared drives, not trashed, as its details " +
				"(id, name, mimeType, modifiedTime, size, webViewLink, owners, parents, createdTime, description, driveId), not its content: " +
				"listed whole the first time, 1,000 a page, and then Drive's changes since, trashed and removed files deleted."}},
	})
}

// driveCursor is where the files stream is: Start is the changes page token taken before a listing began, the
// changes to read once it ends, and Page the listing's next page; without Page, Start is the delta's.
type driveCursor struct {
	Start   string `json:"s"`
	Page    string `json:"p,omitempty"`
	Listing bool   `json:"l,omitempty"`
}

const driveSync = driveFields + ",parents,trashed,createdTime,description,driveId"

func driveFiles(c *Client, cursor string) (*Page, error) {
	var cur driveCursor
	if err := cursorOf(cursor, &cur); err != nil {
		return nil, err
	}
	all := url.Values{"supportsAllDrives": {"true"}, "includeItemsFromAllDrives": {"true"}, "pageSize": {"1000"}}
	if cur.Start == "" {
		var t struct {
			Token string `json:"startPageToken"`
		}
		if err := c.Get(driveAPI+"/changes/startPageToken?supportsAllDrives=true", &t); err != nil {
			return nil, err
		}
		cur = driveCursor{Start: t.Token, Listing: true}
	}
	if cur.Listing {
		q := all
		q.Set("q", "trashed = false")
		q.Set("corpora", "allDrives")
		q.Set("fields", "nextPageToken,files("+driveSync+")")
		if cur.Page != "" {
			q.Set("pageToken", cur.Page)
		}
		var out struct {
			Files []map[string]any `json:"files"`
			Next  string           `json:"nextPageToken"`
		}
		if err := c.Get(driveAPI+"/files?"+q.Encode(), &out); err != nil {
			return nil, err
		}
		page := &Page{Full: true, More: out.Next != ""}
		for _, f := range out.Files {
			page.Records = append(page.Records, driveRecord(f, false))
		}
		cur.Page, cur.Listing = out.Next, out.Next != ""
		page.Cursor = cursorFor(cur)
		return page, nil
	}
	q := all
	q.Set("pageToken", cur.Start)
	q.Set("includeRemoved", "true")
	q.Set("fields", "nextPageToken,newStartPageToken,changes(changeType,fileId,removed,file("+driveSync+"))")
	var out struct {
		Changes []struct {
			Type    string         `json:"changeType"`
			FileID  string         `json:"fileId"`
			Removed bool           `json:"removed"`
			File    map[string]any `json:"file"`
		} `json:"changes"`
		Next     string `json:"nextPageToken"`
		NewStart string `json:"newStartPageToken"`
	}
	if err := c.Get(driveAPI+"/changes?"+q.Encode(), &out); err != nil {
		return nil, err
	}
	page := &Page{More: out.Next != ""}
	for _, ch := range out.Changes {
		switch {
		case ch.Type == "drive":
		case ch.Removed || ch.File == nil:
			page.Records = append(page.Records, Record{ID: ch.FileID, Deleted: true})
		default:
			page.Records = append(page.Records, driveRecord(ch.File, ch.File["trashed"] == true))
		}
	}
	cur.Start = cmp.Or(out.Next, out.NewStart)
	page.Cursor = cursorFor(cur)
	return page, nil
}

func driveRecord(f map[string]any, deleted bool) Record {
	return Record{ID: str(f, "id"), Data: f, Updated: stamp(str(f, "modifiedTime")), Deleted: deleted}
}

const (
	driveAPI    = "https://www.googleapis.com/drive/v3"
	driveFields = "id,name,mimeType,modifiedTime,size,webViewLink,owners(displayName,emailAddress)"
)

var driveExports = map[string]string{
	"application/vnd.google-apps.document":     "text/plain",
	"application/vnd.google-apps.spreadsheet":  "text/csv",
	"application/vnd.google-apps.presentation": "text/plain",
}
