package connect

import (
	"cmp"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// Google Drive, read only, with ARCHIVE_GOOGLE like Gmail: the client's authorized redirect URIs list
// <API>/connect/gdrive/callback, and its consent screen asks for drive.readonly, a restricted scope.
// It syncs every file the account can see into drive_file, with the text of Google's own documents, from
// Drive's changes feed, which pushes too.
func init() {
	Register(&Connector{
		ID: "gdrive", Name: "Google Drive", Env: "ARCHIVE_GOOGLE", Host: "https://accounts.google.com",
		// Drive allows each user 12,000 requests a minute, and each file read for its text is one more of them.
		Pace: 50 * time.Millisecond,
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
		Push: drivePush,
		Streams: []*Stream{{Name: "files", Gap: 2 * time.Second, Pull: driveFiles, Table: "drive_file", Rows: driveRows,
			Doc: "Every file and folder in My Drive, shared with you and in your shared drives, but those in the trash: its name, type, size, " +
				"folder, owner, who changed it last and when, and its words, cut at 64 KB: a Google Doc, Sheet (its first sheet) or Slides " +
				"deck's, and a PDF, Word, Excel, PowerPoint or text file's up to 25 MB; other content stays in Drive. Listed whole the first time, 100 a page, and then Drive's changes since."}},
	})
}

// driveSync is what the files stream reads of a file.
const driveSync = "id,name,mimeType,size,webViewLink,parents,driveId,ownedByMe,createdTime,modifiedTime,trashed," +
	"owners(displayName,emailAddress),lastModifyingUser(displayName,emailAddress)"

// driveCursor is where the files stream is: Token is the changes' page token its listing began at, or its
// delta starts from, Page the next page of either, and Listing says a listing is under way.
type driveCursor struct {
	Token   string `json:"t"`
	Page    string `json:"p,omitempty"`
	Listing bool   `json:"l,omitempty"`
}

func driveFiles(c *Client, cursor string) (*Page, error) {
	var cur driveCursor
	if err := cursorOf(cursor, &cur); err != nil {
		return nil, err
	}
	if cur.Token == "" {
		var t struct {
			Token string `json:"startPageToken"`
		}
		if err := c.Get(driveAPI+"/changes/startPageToken?supportsAllDrives=true", &t); err != nil {
			return nil, err
		}
		cur = driveCursor{Token: t.Token, Listing: true}
	}
	drives := map[string]string{}
	if cur.Listing {
		v := url.Values{"q": {"trashed = false"}, "pageSize": {"100"}, "corpora": {"allDrives"}, "supportsAllDrives": {"true"},
			"includeItemsFromAllDrives": {"true"}, "fields": {"nextPageToken,files(" + driveSync + ")"}}
		if cur.Page != "" {
			v.Set("pageToken", cur.Page)
		}
		var list struct {
			Files []map[string]any `json:"files"`
			Next  string           `json:"nextPageToken"`
		}
		if err := c.Get(driveAPI+"/files?"+v.Encode(), &list); err != nil {
			return nil, err
		}
		page := &Page{Full: true, More: list.Next != ""}
		var err error
		page.Records, err = driveRecords(c, list.Files, drives)
		cur.Page, cur.Listing = list.Next, list.Next != ""
		page.Cursor = cursorFor(cur)
		return page, err
	}
	v := url.Values{"pageToken": {cmp.Or(cur.Page, cur.Token)}, "pageSize": {"100"}, "includeRemoved": {"true"}, "supportsAllDrives": {"true"},
		"includeItemsFromAllDrives": {"true"}, "fields": {"nextPageToken,newStartPageToken,changes(fileId,removed,file(" + driveSync + "))"}}
	var ch struct {
		Changes []struct {
			FileID  string         `json:"fileId"`
			Removed bool           `json:"removed"`
			File    map[string]any `json:"file"`
		} `json:"changes"`
		Next  string `json:"nextPageToken"`
		Start string `json:"newStartPageToken"`
	}
	if err := c.Get(driveAPI+"/changes?"+v.Encode(), &ch); err != nil {
		return nil, err
	}
	page := &Page{More: ch.Next != ""}
	var live []map[string]any
	for _, x := range ch.Changes {
		switch {
		case x.FileID == "":
		case x.Removed || x.File == nil || x.File["trashed"] == true:
			page.Records = append(page.Records, Record{ID: x.FileID, Deleted: true})
		default:
			live = append(live, x.File)
		}
	}
	got, err := driveRecords(c, live, drives)
	page.Records = append(page.Records, got...)
	if ch.Next != "" {
		cur.Page = ch.Next
	} else {
		cur = driveCursor{Token: cmp.Or(ch.Start, cur.Token)}
	}
	page.Cursor = cursorFor(cur)
	return page, err
}

// driveRecords are files as records, each with the drive it is in by name, and the text of those that read as
// text, four at a time; drives keeps the names of shared drives the page has looked up.
func driveRecords(c *Client, files []map[string]any, drives map[string]string) ([]Record, error) {
	for _, f := range files {
		switch id := str(f, "driveId"); {
		case id != "":
			if _, ok := drives[id]; !ok {
				var d struct {
					Name string `json:"name"`
				}
				if err := c.Get(driveAPI+"/drives/"+url.PathEscape(id)+"?fields=name", &d); err != nil && Busy(err) {
					return nil, err
				}
				drives[id] = cmp.Or(d.Name, "Shared drive")
			}
			f["drive"] = drives[id]
		case f["ownedByMe"] == true:
			f["drive"] = "My Drive"
		default:
			f["drive"] = "Shared with me"
		}
	}
	out := make([]Record, len(files))
	err := each(len(files), 4, func(i int) error {
		f := files[i]
		text, err := driveText(c, f)
		if err != nil {
			return err
		}
		f["text"] = clip(text, maxText)
		out[i] = Record{ID: str(f, "id"), Data: f, Updated: stamp(str(f, "modifiedTime"))}
		return nil
	})
	return out, err
}

// driveText is the words of f: a Google Doc, Sheet or Slides deck as export has it, and any other file Archive
// reads, like a PDF or a Word file, downloaded up to 25 MB; "" for the rest, or one Drive will not hand over.
// Only a busy Drive is an error.
func driveText(c *Client, f map[string]any) (string, error) {
	id, mime := url.PathEscape(str(f, "id")), str(f, "mimeType")
	if as := driveExports[mime]; as != "" {
		var text string
		if err := c.Get(driveAPI+"/files/"+id+"/export?mimeType="+url.QueryEscape(as), &text); err != nil {
			if Busy(err) {
				return "", err
			}
			return "", nil
		}
		return text, nil
	}
	if strings.HasPrefix(mime, "application/vnd.google-apps.") {
		return "", nil
	}
	size, _ := strconv.ParseInt(str(f, "size"), 10, 64)
	return c.fileText(driveAPI+"/files/"+id+"?alt=media&supportsAllDrives=true", str(f, "name"), mime, size)
}

// driveRows is a file as a file of a drive: its folder is its first parent, and its owner the first of them,
// or for a file of a shared drive, which has none, nobody.
func driveRows(m map[string]any) *Row {
	kind := "file"
	if str(m, "mimeType") == "application/vnd.google-apps.folder" {
		kind = "folder"
	}
	owner, editor := map[string]any{}, obj(m, "lastModifyingUser")
	if owners := objects(m["owners"], ""); len(owners) > 0 {
		owner = owners[0]
	}
	r := rowOf(map[string]any{"kind": kind, "title": str(m, "name"), "mime": str(m, "mimeType"), "size": decimal(m["size"]), "url": str(m, "webViewLink"),
		"drive": str(m, "drive"), "owner_name": str(owner, "displayName"), "editor_name": str(editor, "displayName"),
		"made_at": instant(str(m, "createdTime")), "edited_at": instant(str(m, "modifiedTime")), "text": str(m, "text")})
	if ps := strs(m["parents"]); len(ps) > 0 {
		r.ref("folder", ps[0])
	}
	return r.person("owner", str(owner, "emailAddress")).person("editor", str(editor, "emailAddress"))
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

// drivePush is changes.watch, a channel of Archive's own id that pushes to Archive with its token, for a week
// at most; the first sync a day after replaces it, and stops the one before. A push says its channel in
// X-Goog-Channel-ID and its token in X-Goog-Channel-Token; the sync push of a new channel says nothing changed.
var drivePush = &Push{Renew: 6 * 24 * time.Hour,
	Watch: func(c *Client, h Hook, old []Watch) ([]Watch, error) {
		var t struct {
			Token string `json:"startPageToken"`
		}
		if err := c.Get(driveAPI+"/changes/startPageToken?supportsAllDrives=true", &t); err != nil {
			return nil, err
		}
		id, token := "archive-"+nonce()[:32], nonce()
		var ch struct {
			ResourceID string `json:"resourceId"`
			Expiration string `json:"expiration"`
		}
		q := url.Values{"pageToken": {t.Token}, "supportsAllDrives": {"true"}, "includeItemsFromAllDrives": {"true"}}
		body := map[string]any{"id": id, "type": "web_hook", "address": h.URL(id), "token": token,
			"expiration": strconv.FormatInt(time.Now().Add(7*24*time.Hour-time.Hour).UnixMilli(), 10)}
		if err := c.Do("POST", driveAPI+"/changes/watch?"+q.Encode(), body, &ch); err != nil {
			return nil, err
		}
		for _, o := range old {
			c.Do("POST", driveAPI+"/channels/stop", map[string]string{"id": o.Key, "resourceId": o.Resource}, nil)
		}
		return []Watch{{Key: id, Resource: ch.ResourceID, Secret: token, Expires: stamp(ch.Expiration)}}, nil
	},
	Unwatch: func(c *Client, w Watch) error {
		return c.Do("POST", driveAPI+"/channels/stop", map[string]string{"id": w.Key, "resourceId": w.Resource}, nil)
	},
	Hear: func(p *Provider, r *http.Request, at string, body []byte, secret func(string) string) (*Heard, error) {
		id := r.Header.Get("X-Goog-Channel-ID")
		kept := secret(id)
		switch {
		case kept == "":
			return &Heard{}, nil
		case !same(kept, r.Header.Get("X-Goog-Channel-Token")):
			return nil, forged("channel %s's token", id)
		case r.Header.Get("X-Goog-Resource-State") == "sync":
			return &Heard{}, nil
		}
		return &Heard{Keys: []string{id}}, nil
	}}
