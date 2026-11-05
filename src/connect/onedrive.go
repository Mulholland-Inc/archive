package connect

import (
	"time"

	"golang.org/x/oauth2"
)

// OneDrive is the personal OneDrive of an Outlook.com, Hotmail or Live account. SharePoint signs in work and school
// accounts only, and asks for sites and Microsoft Search, which a personal account has none of.
func init() {
	Register(&Connector{
		ID: "onedrive", Name: "OneDrive", Env: "ARCHIVE_OUTLOOK",
		Info: Info{By: "Microsoft", Description: "Search and read the files of your personal OneDrive, and send files there when you ask", Category: CatFiles, Icon: IconDrive,
			Scopes: []string{"Read your OneDrive files", "Add and replace files there when you ask"}},
		API: "https://graph.microsoft.com/v1.0",
		Endpoint: oauth2.Endpoint{AuthURL: "https://login.microsoftonline.com/consumers/oauth2/v2.0/authorize",
			TokenURL: "https://login.microsoftonline.com/consumers/oauth2/v2.0/token", AuthStyle: oauth2.AuthStyleInParams},
		Scopes: []string{"offline_access", "User.Read", "Files.ReadWrite"},
		Params: map[string]string{"prompt": "select_account"},
		Who:    &Who{Req: Req{URL: "/me?$select=mail,userPrincipalName"}, Names: []Path{"mail", "userPrincipalName"}, Lower: true},
		Actions: map[string]*Action{
			"search": Call[odFind, odFound](Plan{
				Doc: "Find files and folders in your OneDrive by words in their name or content: { files: [{ id, name, webUrl, size, folder, file, lastModifiedDateTime, parentReference }], next } " +
					"(pass next as page for more)",
				Clamp: map[string][2]int{"limit": {25, 200}},
				Steps: []Step{
					{If: "page", As: "got", Req: Req{URL: "{+page}"}},
					{Unless: "page", As: "got", Req: Req{URL: "/me/drive/root/search(q='{query}')", Query: Q{"$top": "{limit}"}}},
				},
				Out: Obj{"files": "{got/value|[]}", "next": "{got/@odata.nextLink|}"},
			}),
			"folder": Call[odIn, []map[string]any](Plan{
				Doc: "What is in a folder of your OneDrive, or at its top: [{ id, name, webUrl, size, folder, file, lastModifiedDateTime }]",
				Steps: []Step{
					{If: "id", Req: Req{URL: "/me/drive/items/{id}/children"}, Page: spPage, Items: "value"},
					{Unless: "id", Req: Req{URL: "/me/drive/root/children"}, Page: spPage, Items: "value"},
				},
			}),
			"file": Call[odOne, map[string]any](Plan{
				Doc:   "One file's details: { id, name, webUrl, size, file, parentReference, createdBy, lastModifiedBy, lastModifiedDateTime }; its words are in the file table",
				Steps: []Step{{Req: Req{URL: "/me/drive/items/{id}"}}},
			}),
			"upload": Upload(Send{
				Doc: "Send a file of the file table to your OneDrive, as a new file or a new version of one, in the time a run has; without a folder it goes in the top: " +
					"{ id, name, webUrl, size, file, parentReference }",
				New:  Req{Method: POST, URL: "/me/drive/root:/{name}:/createUploadSession", Body: spUpBody},
				In:   &Req{Method: POST, URL: "/drives/{in/drive}/items/{in/item}:/{name}:/createUploadSession", Body: spUpBody},
				Over: Req{Method: POST, URL: "/drives/{it/drive}/items/{it/item}/createUploadSession", Header: map[string][]string{"If-Match": {"{version}"}}, Body: Obj{"item": Obj{"name": "{rename}"}}},
				At:   "uploadUrl",
				Part: Req{Method: PUT, Header: map[string][]string{"Content-Range": {"{range}"}}},
				Step: 32 * 320 << 10,
				Bare: true,
				Stale: []Deny{
					{Status: []int{412}, Why: "{name} changed there since Archive last synced it, so it was not replaced: look at the newer version once it syncs, and upload again if yours should still replace it"},
					{Status: []int{409}, Why: "there is a {name} there already: pass replace with its source_id to replace it, or name to give yours another name"},
				},
			}),
		},
		Listen: &Listen{Renew: 27 * 24 * time.Hour, Until: 42000 * time.Minute, Secret: true, Lax: true, Over: odDrive,
			Watch: Req{Method: POST, URL: "/subscriptions", Body: Obj{"changeType": "updated", "notificationUrl": "{hook}", "resource": "/me/drive/root", "expirationDateTime": "{until}", "clientState": "{secret}"}},
			Key:   "{answer/id}", Resource: "{part}", Expires: "expirationDateTime",
			OfPart:  &Reads{Of: "{resource}"},
			Extend:  &Req{Method: PATCH, URL: "/subscriptions/{key}", Body: Obj{"expirationDateTime": "{until}"}},
			Unwatch: &Req{Method: DELETE, URL: "/subscriptions/{key}"},
			Hello:   []Hello{{Query: "validationToken"}},
			Kept:    &Kept{Each: "value", ID: "subscriptionId", Token: "clientState"},
		},
		Sync: []*Sync{{Name: "files", Gap: time.Second, Parts: odDrive,
			Req:   Req{URL: "/drives/{part}/root/delta?$select=id,name,size,eTag,cTag,webUrl,file,folder,root,parentReference,createdDateTime,lastModifiedDateTime,createdBy,lastModifiedBy,deleted"},
			Items: "value", Key: []Reads{{Of: "{part}/{id}"}}, Updated: "lastModifiedDateTime", Page: spPage,
			Change: Resume{At: "@odata.deltaLink", Expired: []int{410}, Stale: []Deny{{Status: []int{400}, Says: []string{"SyncState", "resyncRequired", "sync state"}}}},
			Gone:   []Test{{At: "deleted"}, {At: "@removed"}}, Skip: []Test{{At: "root"}},
			Set:     Obj{"drive": "OneDrive", "driveId": "{part}"},
			Content: []Content{{Req: Req{URL: "/drives/{source/drive}/items/{source/item}/content"}, Size: "size"}},
			Doc: "Every file and folder of your OneDrive: its name, type, folder, who made it and changed it last and when, and a copy of its bytes up to 1 GB, " +
				"with its words. Listed whole the first time, by <drive id>/<item id>, and then the drive's delta since; a delta Graph no longer keeps lists it all again.",
			To: Map{Table: "file",
				Cols: map[string]SQL{"kind": "CASE WHEN data->'folder' IS NOT NULL THEN 'folder' ELSE 'file' END", "name": "data->>'name'",
					"mime": "CASE WHEN data->'folder' IS NOT NULL THEN 'inode/directory' ELSE coalesce(data#>>'{file,mimeType}', 'application/octet-stream') END", "version": "coalesce(data->>'cTag', data->>'eTag')",
					"source": "jsonb_build_object('drive', data->>'driveId', 'item', data->>'id', 'size', data->>'size')",
					"url":    "data->>'webUrl'", "drive": "data->>'drive'", "owner_name": "data#>>'{createdBy,user,displayName}'", "editor_name": "data#>>'{lastModifiedBy,user,displayName}'",
					"made_at": "archive.sync_at(data->>'createdDateTime')", "edited_at": "archive.sync_at(data->>'lastModifiedDateTime')"},
				People: map[string]SQL{"owner": "data#>>'{createdBy,user,email}'", "editor": "data#>>'{lastModifiedBy,user,email}'"},
				Refs:   map[string]SQL{"parent": "coalesce(nullif(data#>>'{parentReference,driveId}', ''), data->>'driveId') || '/' || nullif(data#>>'{parentReference,id}', '')"}}}},
	})
}

var odDrive = &Parts{Req: Req{URL: "/me/drive?$select=id"}, Vals: Obj{"name": "OneDrive"}}

type (
	odFind struct {
		Query string `json:"query" jsonschema:"Words in the file's name or content"`
		Limit int    `json:"limit,omitempty" jsonschema:"At most this many a page, 1 to 200; 25 when left out"`
		Page  string `json:"page,omitempty" jsonschema:"The next of the search before, for the files after it"`
	}
	odIn struct {
		ID string `json:"id,omitempty" jsonschema:"The folder's id; the top when left out"`
	}
	odOne struct {
		ID string `json:"id" jsonschema:"The file's id, from search or folder"`
	}
	odFound struct {
		Files []map[string]any `json:"files"`
		Next  string           `json:"next" jsonschema:"Pass as page for more; empty when there is no more"`
	}
)
