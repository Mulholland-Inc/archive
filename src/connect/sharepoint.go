package connect

import (
	"cmp"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// SharePoint: the document libraries of the SharePoint sites a Microsoft 365 work or school account can reach,
// and its own OneDrive, which is one of them, read only through Microsoft Graph. It reads ARCHIVE_OUTLOOK, the
// app registration in Microsoft Entra that Outlook connects with, like Gmail and Google Drive share
// ARCHIVE_GOOGLE: its web redirect URIs list <API>/connect/sharepoint/callback too, and its delegated Graph
// permissions add Files.Read.All and Sites.Read.All, which a tenant may ask an admin to consent to. Personal
// Microsoft accounts have no SharePoint, so it signs in through the organizations endpoint. Microsoft replaces
// the refresh token on every refresh, and has no endpoint that revokes one grant: a person removes Archive at
// myapps.microsoft.com.
//
// It syncs every file and folder of every library into drive_file, each library with Graph's delta of it, as
// Outlook syncs each folder: the text of a text file is kept, and the content of the rest, like a Word file,
// stays in SharePoint. Graph pushes a library's changes only to a subscription of each library, so it pushes
// nothing yet: connecting, asking and reading start a sync.
func init() {
	Register(&Connector{
		ID: "sharepoint", Name: "SharePoint", Env: "ARCHIVE_OUTLOOK", Host: "https://login.microsoftonline.com",
		Endpoint: oauth2.Endpoint{AuthURL: "https://login.microsoftonline.com/organizations/oauth2/v2.0/authorize",
			TokenURL: "https://login.microsoftonline.com/organizations/oauth2/v2.0/token", AuthStyle: oauth2.AuthStyleInParams},
		Scopes: []string{"offline_access", "User.Read", "Files.Read.All", "Sites.Read.All"},
		Params: map[string]string{"prompt": "select_account"},
		Account: func(c *Client) (string, error) {
			var me struct {
				Mail string `json:"mail"`
				UPN  string `json:"userPrincipalName"`
			}
			err := c.Get(graphAPI+"/me?$select=mail,userPrincipalName", &me)
			return strings.ToLower(cmp.Or(me.Mail, me.UPN)), err
		},
		Actions: map[string]*Action{
			"search": Act("Find files and folders in every SharePoint site and OneDrive you can reach, best match first, by words in their name or "+
				"content, or by KQL like filetype:docx author:ann: { files: [{ id, driveId, name, webUrl, size, lastModifiedDateTime, lastModifiedBy, summary }], next } "+
				"(pass next as page for more)",
				func(c *Client, in struct {
					Query string `json:"query" jsonschema:"Words or KQL, like budget filetype:xlsx"`
					Limit int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 50; 25 when left out"`
					Page  int    `json:"page,omitempty" jsonschema:"The next of the search before, for the files after it"`
				}) (any, error) {
					size, from := clamp(in.Limit, 25, 50), max(in.Page, 0)
					req := map[string]any{"requests": []map[string]any{{"entityTypes": []string{"driveItem"}, "query": map[string]string{"queryString": in.Query},
						"from": from, "size": size}}}
					var res struct {
						Value []struct {
							Containers []struct {
								Hits []struct {
									Summary  string         `json:"summary"`
									Resource map[string]any `json:"resource"`
								} `json:"hits"`
								More bool `json:"moreResultsAvailable"`
							} `json:"hitsContainers"`
						} `json:"value"`
					}
					if err := c.Do("POST", graphAPI+"/search/query", req, &res); err != nil {
						return nil, err
					}
					files := []map[string]any{}
					var next any
					for _, v := range res.Value {
						for _, hc := range v.Containers {
							for _, h := range hc.Hits {
								r := h.Resource
								files = append(files, map[string]any{"id": str(r, "id"), "driveId": str(obj(r, "parentReference"), "driveId"), "name": str(r, "name"),
									"webUrl": str(r, "webUrl"), "size": r["size"], "lastModifiedDateTime": str(r, "lastModifiedDateTime"),
									"lastModifiedBy": str(obj(obj(r, "lastModifiedBy"), "user"), "displayName"), "summary": h.Summary})
							}
							if hc.More {
								next = from + size
							}
						}
					}
					return map[string]any{"files": files, "next": next}, nil
				}),
			"sites": Act("The SharePoint sites you can reach, maybe only those whose name has some words, with their document libraries: "+
				"[{ id, displayName, webUrl, drives: [{ id, name, webUrl }] }]",
				func(c *Client, in struct {
					Name string `json:"name,omitempty" jsonschema:"Only sites whose name has these words"`
				}) (any, error) {
					sites, err := spSites(c, cmp.Or(in.Name, "*"))
					if err != nil {
						return nil, err
					}
					out := []map[string]any{}
					for _, s := range sites {
						drives, err := spDrives(c, s.ID)
						if err != nil {
							return nil, err
						}
						ds := []map[string]any{}
						for _, d := range drives {
							ds = append(ds, map[string]any{"id": d.ID, "name": d.Name, "webUrl": d.URL})
						}
						out = append(out, map[string]any{"id": s.ID, "displayName": s.Name, "webUrl": s.URL, "drives": ds})
					}
					return out, nil
				}),
			"folder": Act("What is in a folder of a library, or at its top: [{ id, name, webUrl, size, folder, file, lastModifiedDateTime, lastModifiedBy }]",
				func(c *Client, in struct {
					Drive string `json:"drive" jsonschema:"The library's id, a drive id from sites or search"`
					ID    string `json:"id,omitempty" jsonschema:"The folder's id; its top when left out"`
				}) (any, error) {
					u := graphAPI + "/drives/" + url.PathEscape(in.Drive) + "/root/children"
					if in.ID != "" {
						u = graphAPI + "/drives/" + url.PathEscape(in.Drive) + "/items/" + url.PathEscape(in.ID) + "/children"
					}
					out := []map[string]any{}
					for u != "" {
						var page struct {
							Value []map[string]any `json:"value"`
							Next  string           `json:"@odata.nextLink"`
						}
						if err := c.Get(u, &page); err != nil {
							return nil, err
						}
						out, u = append(out, page.Value...), page.Next
					}
					return out, nil
				}),
			"file": Act("One file's details, and its text when it is a text file up to 5 MB: { id, name, webUrl, size, file, parentReference, createdBy, "+
				"lastModifiedBy, lastModifiedDateTime, text }",
				func(c *Client, in struct {
					Drive string `json:"drive" jsonschema:"The library's id, the driveId from search"`
					ID    string `json:"id" jsonschema:"The file's id, from search or folder"`
				}) (any, error) {
					var f map[string]any
					if err := c.Get(graphAPI+"/drives/"+url.PathEscape(in.Drive)+"/items/"+url.PathEscape(in.ID), &f); err != nil {
						return nil, err
					}
					text, err := spText(c, in.Drive, f)
					f["text"] = text
					return f, err
				}),
		},
		Streams: []*Stream{{Name: "files", Gap: time.Second, Pull: spFiles.pull, Table: "drive_file", Rows: spRows,
			Doc: "Every file and folder of your OneDrive and of each document library of each SharePoint site you can reach: its name, type, size, " +
				"folder, library, who made it and changed it last and when, and the text of a text file, cut at 64 KB; the content of the rest, " +
				"like a Word file, stays in SharePoint. Listed whole the first time, a library at a time, by <library id>/<item id>, and then each " +
				"library's delta since. A library made since is listed whole; one gone, or a delta Graph no longer keeps, lists them all again."}},
	})
}

// spSync is what the files stream reads of a file.
const spSync = "id,name,size,webUrl,file,folder,root,parentReference,createdDateTime,lastModifiedDateTime,createdBy,lastModifiedBy,deleted"

// spFiles walks the delta of each library: the account's OneDrive, then every library of every site.
var spFiles = &graphWalk{parts: spLibraries, records: spRecords}

type spPlace struct{ ID, Name, URL string }

// spSites are the sites whose name has words, every one with *, every page of them.
func spSites(c *Client, words string) ([]spPlace, error) {
	var out []spPlace
	u := graphAPI + "/sites?search=" + url.QueryEscape(words) + "&$select=id,displayName,name,webUrl&$top=200"
	for u != "" {
		var page struct {
			Value []struct {
				ID          string `json:"id"`
				DisplayName string `json:"displayName"`
				Name        string `json:"name"`
				URL         string `json:"webUrl"`
			} `json:"value"`
			Next string `json:"@odata.nextLink"`
		}
		if err := c.Get(u, &page); err != nil {
			return nil, err
		}
		for _, s := range page.Value {
			out = append(out, spPlace{ID: s.ID, Name: cmp.Or(s.DisplayName, s.Name), URL: s.URL})
		}
		u = page.Next
	}
	return out, nil
}

// spDrives are the document libraries of site.
func spDrives(c *Client, site string) ([]spPlace, error) {
	var out []spPlace
	u := graphAPI + "/sites/" + url.PathEscape(site) + "/drives?$select=id,name,webUrl"
	for u != "" {
		var page struct {
			Value []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
				URL  string `json:"webUrl"`
			} `json:"value"`
			Next string `json:"@odata.nextLink"`
		}
		if err := c.Get(u, &page); err != nil {
			return nil, err
		}
		for _, d := range page.Value {
			out = append(out, spPlace{ID: d.ID, Name: d.Name, URL: d.URL})
		}
		u = page.Next
	}
	return out, nil
}

// spLibraries are the parts the files stream walks: the account's OneDrive, when it has one, and the libraries
// of the sites it reaches, each once, named by its site and itself, like Finance / Documents. A site Graph
// will not list the libraries of, as it answers 403 for some system sites, is left out.
func spLibraries(c *Client) ([]graphPart, error) {
	var out []graphPart
	seen := map[string]bool{}
	add := func(id, name string) {
		if id != "" && !seen[id] {
			seen[id] = true
			out = append(out, graphPart{ID: id, Name: name, Start: graphAPI + "/drives/" + url.PathEscape(id) + "/root/delta?$select=" + spSync})
		}
	}
	var me struct {
		ID string `json:"id"`
	}
	if err := c.Get(graphAPI+"/me/drive?$select=id", &me); err == nil {
		add(me.ID, "OneDrive")
	} else if r := (*Refusal)(nil); !errors.As(err, &r) || r.Status != http.StatusNotFound {
		return nil, err
	}
	sites, err := spSites(c, "*")
	if err != nil {
		return nil, err
	}
	for _, s := range sites {
		drives, err := spDrives(c, s.ID)
		if r := (*Refusal)(nil); errors.As(err, &r) && (r.Status == http.StatusForbidden || r.Status == http.StatusNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, d := range drives {
			add(d.ID, s.Name+" / "+d.Name)
		}
	}
	return out, nil
}

// spRecords are a page of a library's delta, by <library id>/<item id>, as an item's id is only its library's:
// one deleted is gone, its library's top is left out, and the rest are kept with their library's name and the
// text of a text file.
func spRecords(c *Client, p graphPart, items []map[string]any) ([]Record, error) {
	var out []Record
	var live []map[string]any
	for _, it := range items {
		id := p.ID + "/" + str(it, "id")
		switch {
		case it["deleted"] != nil || it["@removed"] != nil:
			out = append(out, Record{ID: id, Deleted: true})
		case it["root"] != nil:
		default:
			it["drive"], it["driveId"] = p.Name, p.ID
			live = append(live, it)
		}
	}
	got := make([]Record, len(live))
	err := each(len(live), 4, func(i int) error {
		it := live[i]
		text, err := spText(c, p.ID, it)
		if err != nil {
			return err
		}
		it["text"] = clip(text, maxText)
		got[i] = Record{ID: p.ID + "/" + str(it, "id"), Data: it, Updated: stamp(str(it, "lastModifiedDateTime"))}
		return nil
	})
	return append(out, got...), err
}

// spText is the text of f in the library drive when it is a text file up to 5 MB, and "" for any other; only a
// busy Graph is an error.
func spText(c *Client, drive string, f map[string]any) (string, error) {
	mime := str(obj(f, "file"), "mimeType")
	size, _ := f["size"].(float64)
	if !(strings.HasPrefix(mime, "text/") || mime == "application/json") || size > maxBody {
		return "", nil
	}
	var text string
	err := c.Get(graphAPI+"/drives/"+url.PathEscape(drive)+"/items/"+url.PathEscape(str(f, "id"))+"/content", &text)
	if err != nil {
		if Busy(err) {
			return "", err
		}
		return "", nil
	}
	return text, nil
}

// spRows is an item of a library as a file of a drive: its folder is its parent, which links unless it is the
// library's top, which is no row; and its owner who made it.
func spRows(m map[string]any) *Row {
	kind := "file"
	if m["folder"] != nil {
		kind = "folder"
	}
	made, edited := obj(obj(m, "createdBy"), "user"), obj(obj(m, "lastModifiedBy"), "user")
	r := rowOf(map[string]any{"kind": kind, "title": str(m, "name"), "mime": str(obj(m, "file"), "mimeType"), "size": decimal(m["size"]),
		"url": str(m, "webUrl"), "drive": str(m, "drive"), "owner_name": str(made, "displayName"), "editor_name": str(edited, "displayName"),
		"made_at": instant(str(m, "createdDateTime")), "edited_at": instant(str(m, "lastModifiedDateTime")), "text": str(m, "text")})
	if parent := obj(m, "parentReference"); str(parent, "id") != "" {
		r.ref("folder", cmp.Or(str(parent, "driveId"), str(m, "driveId"))+"/"+str(parent, "id"))
	}
	return r.person("owner", str(made, "email")).person("editor", str(edited, "email"))
}
