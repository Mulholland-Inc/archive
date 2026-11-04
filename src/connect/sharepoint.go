package connect

import (
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
// Outlook syncs each folder, with the words of each PDF, Word, Excel, PowerPoint or text file up to 25 MB.
//
// Graph pushes a library's changes only to a subscription of each library: to the top of it, which tells of a change
// anywhere in it, with a clientState of Archive's own, for 42,000 minutes, under the 42,300 Graph allows a drive's: the
// first sync after a library's comes within 27 days of ending renews it, one for a library made since is made, and one
// for a library gone is deleted. A library Graph will not subscribe to, as some system libraries are, is left to the
// syncs that start otherwise.
func init() {
	Register(&Connector{
		ID: "sharepoint", Name: "SharePoint", Env: "ARCHIVE_OUTLOOK",
		Info: Info{By: "Microsoft", Description: "Search and read the files of your SharePoint sites and OneDrive, never change them", Category: CatFiles, Icon: IconDrive,
			Scopes: []string{"Read files in every site and library you can read"}},
		API: "https://graph.microsoft.com/v1.0",
		Endpoint: oauth2.Endpoint{AuthURL: "https://login.microsoftonline.com/organizations/oauth2/v2.0/authorize",
			TokenURL: "https://login.microsoftonline.com/organizations/oauth2/v2.0/token", AuthStyle: oauth2.AuthStyleInParams},
		Scopes: []string{"offline_access", "User.Read", "Files.Read.All", "Sites.Read.All"},
		Params: map[string]string{"prompt": "select_account"},
		Who:    &Who{Req: Req{URL: "/me?$select=mail,userPrincipalName"}, Names: []Path{"mail", "userPrincipalName"}, Lower: true},
		Actions: map[string]*Action{
			"search": Call[spFind, spFound](Plan{
				Doc: "Find files and folders in every SharePoint site and OneDrive you can reach, best match first, by words in their name or " +
					"content, or by KQL like filetype:docx author:ann: { files: [{ id, driveId, name, webUrl, size, lastModifiedDateTime, lastModifiedBy, summary }], next } " +
					"(pass next as page for more)",
				Clamp: map[string][2]int{"limit": {25, 50}},
				Steps: []Step{
					{As: "got", Req: Req{Method: POST, Read: true, URL: "/search/query", Body: Obj{"requests": []Obj{{"entityTypes": []string{"driveItem"}, "query": Obj{"queryString": "{query}"}, "from": "{page|0}", "size": "{limit}"}}}}},
					{As: "files", Each: "got/value/*/hitsContainers/*/hits/*", In: Obj{"id": "{resource/id}", "driveId": "{resource/parentReference/driveId}", "name": "{resource/name}",
						"webUrl": "{resource/webUrl}", "size": "{resource/size}", "lastModifiedDateTime": "{resource/lastModifiedDateTime}", "lastModifiedBy": "{resource/lastModifiedBy/user/displayName}", "summary": "{summary}"}},
					{If: "got/value/*/hitsContainers/*/moreResultsAvailable", As: "next", Codec: Sum, In: []any{"{page|0}", "{limit}"}},
				},
				Out: Obj{"files": "{files|[]}", "next": "{next}"},
			}),
			"sites": Call[spSites, []map[string]any](Plan{
				Doc: "The SharePoint sites you can reach, maybe only those whose name has some words, with their document libraries: " +
					"[{ id, displayName, webUrl, drives: [{ id, name, webUrl }] }]",
				Steps: []Step{
					{As: "sites", Req: Req{URL: "/sites?$select=id,displayName,name,webUrl&$top=200", Query: Q{"search": "{name|*}"}}, Page: spPage, Items: "value"},
					{Each: "sites", Onto: "drives", Req: Req{URL: "/sites/{id}/drives?$select=id,name,webUrl"}, Page: spPage, Items: "value"},
				},
			}),
			"folder": Call[spIn, []map[string]any](Plan{
				Doc: "What is in a folder of a library, or at its top: [{ id, name, webUrl, size, folder, file, lastModifiedDateTime, lastModifiedBy }]",
				Steps: []Step{
					{If: "id", Req: Req{URL: "/drives/{drive}/items/{id}/children"}, Page: spPage, Items: "value"},
					{Unless: "id", Req: Req{URL: "/drives/{drive}/root/children"}, Page: spPage, Items: "value"},
				},
			}),
			"file": Call[spOne, map[string]any](Plan{
				Doc: "One file's details, and its words when it is a PDF, Word, Excel, PowerPoint or text file up to 25 MB: { id, name, webUrl, size, file, parentReference, createdBy, " +
					"lastModifiedBy, lastModifiedDateTime, text }",
				Steps: []Step{
					{As: "f", Req: Req{URL: "/drives/{drive}/items/{id}"}},
					{If: "f/file", As: "f/text", Words: &Download{Req: Req{URL: "/drives/{drive}/items/{id}/content"}, Name: "{f/name}", Mime: "{f/file/mimeType}", Size: "f/size"}},
				},
				Out: "{f}",
			}),
		},
		Listen: &Listen{Renew: 27 * 24 * time.Hour, Until: 42000 * time.Minute, Secret: true, Lax: true, Over: spLibraries,
			Watch: Req{Method: POST, URL: "/subscriptions", Body: Obj{"changeType": "updated", "notificationUrl": "{hook}", "resource": "/drives/{part}/root", "expirationDateTime": "{until}", "clientState": "{secret}"}},
			Key:   "{answer/id}", Resource: "{part}", Expires: "expirationDateTime",
			OfPart:  &Reads{Of: "{resource}"},
			Extend:  &Req{Method: PATCH, URL: "/subscriptions/{key}", Body: Obj{"expirationDateTime": "{until}"}},
			Unwatch: &Req{Method: DELETE, URL: "/subscriptions/{key}"},
			Hello:   []Hello{{Query: "validationToken"}},
			Kept:    &Kept{Each: "value", ID: "subscriptionId", Token: "clientState"},
		},
		Sync: []*Sync{{Name: "files", Gap: time.Second, Parts: spLibraries,
			Req:   Req{URL: "/drives/{part}/root/delta?$select=id,name,size,webUrl,file,folder,root,parentReference,createdDateTime,lastModifiedDateTime,createdBy,lastModifiedBy,deleted"},
			Items: "value", Key: []Reads{{Of: "{part}/{id}"}}, Updated: "lastModifiedDateTime", Page: spPage, Pass: []int{403, 404},
			Change: Resume{At: "@odata.deltaLink", Expired: []int{410}, Stale: []Deny{{Status: []int{400}, Says: []string{"SyncState", "resyncRequired", "sync state"}}}},
			// A library's top is left out.
			Gone: []Test{{At: "deleted"}, {At: "@removed"}}, Skip: []Test{{At: "root"}},
			Set:     Obj{"drive": "{in/name}", "driveId": "{part}"},
			Hydrate: []Hydrate{Download{If: Test{At: "file"}, Req: Req{URL: "/drives/{driveId}/items/{id}/content"}, Name: "{name}", Mime: "{file/mimeType}", Size: "size", As: "text", Par: 4}},
			Doc: "Every file and folder of your OneDrive and of each document library of each SharePoint site you can reach: its name, type, size, " +
				"folder, library, who made it and changed it last and when, and the words of a PDF, Word, Excel, PowerPoint or text " +
				"file up to 25 MB, cut at 64 KB; other content stays in SharePoint. Listed whole the first time, a library at a time, by <library id>/<item id>, and then each " +
				"library's delta since. A library made since is listed whole; one gone, or a delta Graph no longer keeps, lists them all again.",
			// An item's folder is its parent, which links unless it is the library's top, which is no row; and its owner who made it.
			To: Map{Table: "drive_file",
				Cols: map[string]SQL{"kind": "CASE WHEN data->'folder' IS NOT NULL THEN 'folder' ELSE 'file' END", "title": "data->>'name'", "mime": "data#>>'{file,mimeType}'", "size": "archive.sync_num(data->>'size')",
					"url": "data->>'webUrl'", "drive": "data->>'drive'", "owner_name": "data#>>'{createdBy,user,displayName}'", "editor_name": "data#>>'{lastModifiedBy,user,displayName}'",
					"made_at": "archive.sync_at(data->>'createdDateTime')", "edited_at": "archive.sync_at(data->>'lastModifiedDateTime')", "text": "data->>'text'"},
				People: map[string]SQL{"owner": "data#>>'{createdBy,user,email}'", "editor": "data#>>'{lastModifiedBy,user,email}'"},
				Refs:   map[string]SQL{"folder": "coalesce(nullif(data#>>'{parentReference,driveId}', ''), data->>'driveId') || '/' || nullif(data#>>'{parentReference,id}', '')"}}}},
	})
}

var (
	spPage = NextURL{At: "@odata.nextLink"}
	// spLibraries are the account's OneDrive, when it has one, and the libraries of the sites it reaches, each once,
	// named by its site and itself, like Finance / Documents. A site Graph will not list the libraries of, as it answers
	// 403 for some system sites, is left out.
	spLibraries = &Parts{Req: Req{URL: "/me/drive?$select=id"}, Soft: []int{404}, Vals: Obj{"name": "OneDrive"},
		Also: []*Parts{{Req: Req{URL: "/sites?$select=id,displayName,name,webUrl&$top=200", Query: Q{"search": "*"}}, Items: "value", Page: spPage,
			Under: &Parts{Req: Req{URL: "/sites/{up/id}/drives?$select=id,name,webUrl"}, Items: "value", Page: spPage, Soft: []int{403, 404}, Vals: Obj{"name": "{up/displayName;up/name} / {name}"}}}}}
)

type (
	spFind struct {
		Query string `json:"query" jsonschema:"Words or KQL, like budget filetype:xlsx"`
		Limit int    `json:"limit,omitempty" jsonschema:"At most this many, 1 to 50; 25 when left out"`
		Page  int    `json:"page,omitempty" jsonschema:"The next of the search before, for the files after it"`
	}
	spSites struct {
		Name string `json:"name,omitempty" jsonschema:"Only sites whose name has these words"`
	}
	spIn struct {
		Drive string `json:"drive" jsonschema:"The library's id, a drive id from sites or search"`
		ID    string `json:"id,omitempty" jsonschema:"The folder's id; its top when left out"`
	}
	spOne struct {
		Drive string `json:"drive" jsonschema:"The library's id, the driveId from search"`
		ID    string `json:"id" jsonschema:"The file's id, from search or folder"`
	}
	spHit struct {
		ID                   string `json:"id"`
		DriveID              string `json:"driveId,omitempty"`
		Name                 string `json:"name,omitempty"`
		WebURL               string `json:"webUrl,omitempty"`
		Size                 int64  `json:"size,omitempty"`
		LastModifiedDateTime string `json:"lastModifiedDateTime,omitempty"`
		LastModifiedBy       string `json:"lastModifiedBy,omitempty"`
		Summary              string `json:"summary,omitempty"`
	}
	spFound struct {
		Files []spHit `json:"files"`
		Next  int     `json:"next,omitempty" jsonschema:"Pass as page for more; left out when there is no more"`
	}
)
