package connect

import (
	"time"

	"golang.org/x/oauth2"
)

// monday.com, through its GraphQL API, version 2026-10, as an app of monday's Developer Center with its New OAuth
// Flow (OAuth 2.1) on, whose tokens expire and come with a refresh token, replaced on every refresh, where the
// legacy flow's never expire and have none. Its OAuth tab lists <API>/connect/monday/callback as a redirect URL and
// exactly the Scopes here; to connect accounts other than Mulholland's own, the app is shared by its install link
// or listed in monday's marketplace. A refresh token is revoked with the client's credentials. A GraphQL document
// with anything but queries in it is refused before it goes.
//
// It syncs the items of every active board the account can see, subitem boards and docs left out, into
// work_item, each with its board, its first status, person, date and timeline columns as its status, assignee and
// dates: listed whole each sync, a board at a time, which deletes those gone. monday's webhooks are made per board
// and opened with a challenge answered in a JSON body, which Listen does not do: connecting, asking and reading
// start a sync.
func init() {
	Register(&Connector{
		ID: "monday", Name: "monday.com", Integration: true,
		Info: Info{By: "monday.com", Description: "Read boards, items and updates, and add, change, comment on and archive items when you ask", Category: CatProjects, Icon: IconList,
			Scopes: []string{"Read boards, items, updates and people", "Add, change, comment on and archive items when you ask"}},
		API: "https://api.monday.com",
		Endpoint: oauth2.Endpoint{AuthURL: "https://auth.monday.com/oauth2/authorize", TokenURL: "https://auth.monday.com/oauth_ms/oauth/token",
			AuthStyle: oauth2.AuthStyleInParams},
		RevokeBy: &Req{Method: POST, URL: "https://auth.monday.com/oauth_ms/oauth/revoke",
			Body: Obj{"client_id": "{client/id}", "client_secret": "{client/secret}", "token": "{refresh}", "token_type_hint": "refresh_token"}},
		Scopes: []string{"me:read", "account:read", "users:read", "boards:read", "boards:write", "updates:read", "updates:write"},
		// monday answers errors with a 200, and running out of the account's complexity budget or rate is one the sync tries again later.
		Errors: &Errors{At: "errors/0/message", Code: "errors/0/extensions/code",
			Busy: []string{"COMPLEXITY_BUDGET_EXHAUSTED", "RATE_LIMIT_EXCEEDED", "IP_RATE_LIMIT_EXCEEDED", "CONCURRENCY_LIMIT_EXCEEDED", "ComplexityException"}},
		Busy: []string{"Complexity budget exhausted", "Rate limit exceeded"},
		Who:  &Who{Req: Req{Method: POST, Read: true, URL: "/v2", Header: mondayVersion, Body: Obj{"query": "query { me { email account { slug name } } }"}}, Names: []Path{"data/me/account/slug", "data/me/account/name"}, Lower: true},
		Actions: map[string]*Action{
			"graphql": Call[mondayQuery, any](Plan{
				Doc: "Run a query of monday's GraphQL API, version 2026-10, like { query: \"query { boards(limit: 25) { id name groups { id title } columns { id title type } } }\" } " +
					"or { query: \"query($b: [ID!]) { boards(ids: $b) { items_page(limit: 50) { cursor items { id name column_values { id text } } } } }\", variables: { b: [\"123\"] } }: " +
					"its data. Queries only; a mutation is refused",
				Steps: []Step{
					{Codec: GraphQLReads, In: "{query}"},
					{As: "got", Req: Req{Method: POST, Read: true, URL: "/v2", Header: mondayVersion, Body: Obj{"query": "{query}", "variables": "{variables}"}}},
				},
				Out: "{got/data}",
			}),
			"item": Call[mondayOne, map[string]any](Plan{
				Doc: "One item with its board, group, every column's value and its 50 latest updates and their replies: " +
					"{ id, name, url, board, group, column_values: [{ id, type, text, value }], updates: [{ id, text_body, creator, created_at, replies }] }",
				Steps: []Step{
					{As: "got", Req: Req{Method: POST, Read: true, URL: "/v2", Header: mondayVersion, Body: Obj{"query": mondayItem, "variables": Obj{"id": []any{"{id}"}}}}},
					{Unless: "got/data/items/0", Fail: "monday has no item {id} this account can see"},
				},
				Out: "{got/data/items/0}",
			}),
			"create": Call[mondayNew, mondayMade](Plan{
				Doc: "Add an item to a board, maybe in a group and with some columns' values, like { board: \"123\", name: \"Renew the lease\", " +
					"columns: \"{\\\"status\\\": {\\\"label\\\": \\\"Working on it\\\"}, \\\"date4\\\": {\\\"date\\\": \\\"2026-11-30\\\"}}\" }: { id, name, url }",
				Steps: []Step{{As: "got", Req: Req{Method: POST, URL: "/v2", Header: mondayVersion, Body: Obj{
					"query":     "mutation($board: ID!, $group: String, $name: String!, $columns: JSON) { create_item(board_id: $board, group_id: $group, item_name: $name, column_values: $columns, create_labels_if_missing: false) { id name url } }",
					"variables": Obj{"board": "{board}", "group": "{group}", "name": "{name}", "columns": "{columns}"}}}}},
				Out: "{got/data/create_item}",
			}),
			"update": Call[mondayEdit, mondayMade](Plan{
				Doc: "Change an item's columns, its name among them as name, like { board: \"123\", id: \"456\", columns: \"{\\\"name\\\": \\\"Renew the lease\\\", " +
					"\\\"status\\\": {\\\"label\\\": \\\"Done\\\"}}\" }: only the columns given change. { id, name, url }",
				Steps: []Step{{As: "got", Req: Req{Method: POST, URL: "/v2", Header: mondayVersion, Body: Obj{
					"query":     "mutation($board: ID!, $id: ID!, $columns: JSON!) { change_multiple_column_values(board_id: $board, item_id: $id, column_values: $columns) { id name url } }",
					"variables": Obj{"board": "{board}", "id": "{id}", "columns": "{columns}"}}}}},
				Out: "{got/data/change_multiple_column_values}",
			}),
			"comment": Call[mondaySay, map[string]any](Plan{
				Doc: "Post an update on an item, as you, maybe as a reply to another update: { id, text_body, created_at }",
				Steps: []Step{{As: "got", Req: Req{Method: POST, URL: "/v2", Header: mondayVersion, Body: Obj{
					"query":     "mutation($id: ID!, $body: String!, $parent: ID) { create_update(item_id: $id, body: $body, parent_id: $parent) { id text_body created_at } }",
					"variables": Obj{"id": "{id}", "body": "{text}", "parent": "{reply_to}"}}}}},
				Out: "{got/data/create_update}",
			}),
			"archive": Call[mondayOne, mondayMade](Plan{
				Doc: "Archive an item: it leaves its board for the board's archive, where it can be restored. { id, name }",
				Steps: []Step{{As: "got", Req: Req{Method: POST, URL: "/v2", Header: mondayVersion, Body: Obj{
					"query": "mutation($id: ID!) { archive_item(item_id: $id) { id name } }", "variables": Obj{"id": "{id}"}}}}},
				Out: "{got/data/archive_item}",
			}),
		},
		Sync: []*Sync{
			{Name: "items", Gap: time.Second, Parts: mondayBoards,
				Req:   Req{Method: POST, Read: true, URL: "/v2", Header: mondayVersion, Body: Obj{"query": mondayItems, "variables": Obj{"board": []any{"{part}"}, "cursor": "{page}"}}},
				Items: "data/boards/0/items_page/items", Updated: "updated_at", Page: Cursor{At: "data/boards/0/items_page/cursor"},
				Set: Obj{"board": "{in/name}"},
				Doc: "Every item of each active board, by its id, with its board, group, status, people, dates and link: listed whole each sync, " +
					"100 a page and a board at a time, which deletes those gone; monday says nothing of what was deleted.",
				To: Map{Table: "work_item",
					Cols: map[string]SQL{"title": "data->>'name'", "type": "data->>'board'", "status": mondayStatus,
						"assignee_name": mondayPeople, "starts_on": mondayTimeline, "due_on": mondayDate, "url": "data->>'url'",
						"made_at": "archive.sync_at(data->>'created_at')", "edited_at": "archive.sync_at(data->>'updated_at')"}}},
		},
	})
}

// An item's status, assignee, start and due date are the text of its first column of that type that has any.
const (
	mondayStatus   SQL = "(SELECT c->>'text' FROM archive.sync_list(data->'column_values') c WHERE c->>'type' = 'status' AND nullif(c->>'text', '') IS NOT NULL LIMIT 1)"
	mondayPeople   SQL = "(SELECT c->>'text' FROM archive.sync_list(data->'column_values') c WHERE c->>'type' = 'people' AND nullif(c->>'text', '') IS NOT NULL LIMIT 1)"
	mondayTimeline SQL = "archive.sync_day((SELECT c->>'text' FROM archive.sync_list(data->'column_values') c WHERE c->>'type' = 'timeline' AND nullif(c->>'text', '') IS NOT NULL LIMIT 1))"
	mondayDate     SQL = "archive.sync_day((SELECT c->>'text' FROM archive.sync_list(data->'column_values') c WHERE c->>'type' = 'date' AND nullif(c->>'text', '') IS NOT NULL LIMIT 1))"
)

const (
	mondayItems = "query($board: [ID!], $cursor: String) { boards(ids: $board) { items_page(limit: 100, cursor: $cursor) { cursor items { " +
		"id name state url created_at updated_at group { id title } creator { name } column_values { id type text } } } } }"
	mondayItem = "query($id: [ID!]) { items(ids: $id) { id name state url created_at updated_at board { id name } group { id title } creator { name } " +
		"column_values { id type text value } updates(limit: 50) { id text_body created_at creator { name } replies { id text_body created_at creator { name } } } } }"
)

var (
	// mondayVersion pins the API's version, so that a new one monday makes current changes nothing.
	mondayVersion = map[string][]string{"API-Version": {"2026-10"}}
	// mondayBoards are the account's active boards that hold items: subitem boards are read with their items' own, and docs hold none.
	mondayBoards = &Parts{Req: Req{Method: POST, Read: true, URL: "/v2", Header: mondayVersion,
		Body: Obj{"query": "query { boards(limit: 100, page: {page}, state: active, order_by: created_at) { id name type } }"}},
		Items: "data/boards", Page: Offset{From: 1, Size: 100, Step: 1}, Skip: []Test{{At: "type", Not: "board"}}, Vals: Obj{"name": "{name}"}}
)

type (
	mondayQuery struct {
		Query     string         `json:"query" jsonschema:"The GraphQL query"`
		Variables map[string]any `json:"variables,omitempty" jsonschema:"Its variables"`
	}
	mondayOne struct {
		ID string `json:"id" jsonschema:"The item's id"`
	}
	mondayNew struct {
		Board   string `json:"board" jsonschema:"The board's id"`
		Group   string `json:"group,omitempty" jsonschema:"The id of the board's group to put it in; the top group when left out"`
		Name    string `json:"name" jsonschema:"Its name"`
		Columns string `json:"columns,omitempty" jsonschema:"Its columns' values as a JSON string, by column id, in monday's format for each type: {\"status\": {\"label\": \"Done\"}, \"date4\": {\"date\": \"2026-11-30\"}, \"person\": {\"personsAndTeams\": [{\"id\": 123, \"kind\": \"person\"}]}}"`
	}
	mondayEdit struct {
		Board   string `json:"board" jsonschema:"The board's id"`
		ID      string `json:"id" jsonschema:"The item's id"`
		Columns string `json:"columns" jsonschema:"The columns to change as a JSON string, by column id, as create takes them; name renames the item"`
	}
	mondaySay struct {
		ID      string `json:"id" jsonschema:"The item's id"`
		Text    string `json:"text" jsonschema:"The update, in plain text or simple HTML"`
		ReplyTo string `json:"reply_to,omitempty" jsonschema:"The id of the update to reply to"`
	}
	mondayMade struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		URL  string `json:"url,omitempty"`
	}
)
