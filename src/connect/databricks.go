package connect

import "time"

func init() {
	Register(&Connector{
		ID: "databricks", Name: "Databricks", Integration: true,
		Info: Info{By: "Databricks", Description: "Read the lakehouse’s tables and query them with SQL", Category: CatData, Icon: IconWarehouse,
			Scopes: []string{"Read the metadata of the tables you pick", "Run SQL on one SQL warehouse, as the service principal’s grants allow"}},
		Keys: []Key{
			{Name: "host", Label: "Workspace URL", Help: "Your workspace's address, like https://adb-1234567890123456.7.azuredatabricks.net or https://dbc-a1b2c3d4-e5f6.cloud.databricks.com"},
			{Name: "client_id", Label: "Service principal client ID",
				Help: "In the workspace, Settings, Identity and access, Service principals: add one for Archive and copy its Application ID"},
			{Name: "client_secret", Label: "OAuth secret", Secret: true,
				Help: "On the service principal's Secrets tab: generate a secret and copy it, which starts with dose"},
			{Name: "warehouse", Label: "SQL warehouse ID",
				Help: "In SQL Warehouses: pick the warehouse Archive runs SQL on, give the service principal Can use on it, and copy the ID from its Overview"},
			{Name: "tables", Label: "Catalogs, schemas or tables",
				Help: "What Archive syncs the metadata of, by commas, like main, finance.ledger, sales.crm.accounts; give the service principal USE CATALOG, USE SCHEMA and SELECT on them"},
		},
		API: "https://{+host}",
		Let: []Step{{As: "host", In: "{key/host}", Find: `^(?:[^/]*://)?([^/?#]*)`}},
		Trade: &Req{Method: POST, URL: "/oidc/v1/token", Header: map[string][]string{"Authorization": {"{basic}"}},
			Form: Q{"grant_type": "client_credentials", "scope": "all-apis"}},
		Sign: map[string]string{"Authorization": "Bearer {key/access}"},
		Who:  &Who{Req: Req{URL: "/oidc/.well-known/oauth-authorization-server"}, Names: []Path{"issuer"}, Find: `^https?://([^/]*)`},
		Actions: map[string]*Action{
			"query": Call[dbQuery, any](Plan{
				Doc: "Run one SQL statement that only reads on the SQL warehouse, like { query: \"SELECT region, sum(amount) AS total FROM main.sales.orders " +
					"WHERE order_date >= :since GROUP BY region\", parameters: [{ name: \"since\", value: \"2026-01-01\", type: \"DATE\" }] }: " +
					"{ state, columns: [{ name, type_name }], rows: [[...]], total_row_count, truncated, statement_id, next_chunk_index }, each value a string. " +
					"SELECT, WITH, SHOW, DESCRIBE, EXPLAIN, VALUES, TABLE and FROM only; a statement that writes is refused. Up to limit rows, 100 when left out, " +
					"at most 10,000, and 25 MB; it waits 50 seconds, past which it is cancelled. Read more of a long answer with rows. information_schema and " +
					"SHOW TABLES IN <catalog>.<schema> say what there is",
				Clamp: map[string][2]int{"limit": {100, 10000}},
				Steps: append([]Step{{Codec: SQLReads, In: "{query}"}}, dbRun(Req{Method: POST, Read: true, URL: "/api/2.0/sql/statements", Body: dbStatement})...),
				Out:   dbOut,
			}),
			"execute": Call[dbQuery, any](Plan{
				Doc: "Run one SQL statement that may write on the SQL warehouse, like { query: \"UPDATE main.crm.accounts SET tier = :tier WHERE id = :id\", " +
					"parameters: [{ name: \"tier\", value: \"gold\" }, { name: \"id\", value: \"42\", type: \"BIGINT\" }] }, or an INSERT, MERGE, CREATE TABLE " +
					"or DELETE: what query answers, rows the counts Databricks gives. Only when someone asks for the change; the service principal needs MODIFY " +
					"or more on the table. Use query to read",
				Clamp: map[string][2]int{"limit": {100, 10000}},
				Steps: dbRun(Req{Method: POST, URL: "/api/2.0/sql/statements", Body: dbStatement}),
				Out:   dbOut,
			}),
			"rows": Call[dbChunk, any](Plan{
				Doc:   "The next rows of a long answer of query, by its statement_id and next_chunk_index: { rows: [[...]], next_chunk_index }, none once next_chunk_index is gone",
				Steps: []Step{{As: "got", Req: Req{URL: "/api/2.0/sql/statements/{statement_id}/result/chunks/{chunk}"}}},
				Out:   Obj{"rows": "{got/data_array}", "row_offset": "{got/row_offset}", "next_chunk_index": "{got/next_chunk_index}"},
			}),
			"get": Call[dbGet, any](Plan{
				Doc: "Read any of the workspace's GET endpoints, like { path: \"/api/2.1/unity-catalog/catalogs\" }, { path: \"/api/2.1/unity-catalog/schemas\", " +
					"query: \"catalog_name=main\" }, { path: \"/api/2.1/unity-catalog/table-summaries\", query: \"catalog_name=main&schema_name_pattern=sales\" }, " +
					"{ path: \"/api/2.1/unity-catalog/tables/main.sales.orders\" } for a table's columns, or { path: \"/api/2.0/sql/warehouses\" }: its answer, " +
					"a list paged by next_page_token as page_token",
				Steps: []Step{{Req: Req{URL: "{+path}?{+query|}"}}},
			}),
		},
		Sync: []*Sync{
			{Name: "tables", Gap: time.Second, Every: 6 * time.Hour, Items: "tables", ID: "full_name", Page: Cursor{At: "next_page_token"},
				Absent: "next_page_token", Empty: true,
				Parts: &Parts{Split: "{key/tables}", ID: "{name}", Vals: Obj{"catalog": "{at/0}", "schema": "{at/1}", "table": "{at/2}"}},
				Req: Req{URL: "/api/2.1/unity-catalog/table-summaries",
					Query: Q{"catalog_name": "{in/catalog}", "schema_name_pattern": "{in/schema|%}", "table_name_pattern": "{in/table|%}", "max_results": "1000", "page_token": "{page}"}},
				Hydrate: []Hydrate{Fetch{Par: 4, Gone: true, Merge: true, Req: Req{URL: "/api/2.1/unity-catalog/tables/{full_name}", Query: Q{"include_browse": "true"}}}},
				Deny:    []Deny{{Status: []int{403}, Why: "the service principal may not read these tables; give it USE CATALOG, USE SCHEMA and SELECT: {message}"}},
				Pass:    []int{404},
				Doc: "Every table, view and materialized view of the catalogs, schemas and tables picked, with its kind, comment, when it was made and " +
					"last changed, and its columns, never its rows, by its full name: listed whole at most six hours apart, 1000 a page, which deletes those gone.",
				To: Map{Table: "warehouse_table",
					Cols: map[string]SQL{"database": "data->>'catalog_name'", "schema": "data->>'schema_name'", "title": "data->>'name'",
						"kind": "replace(data->>'table_type', '_', ' ')", "comment": "archive.sync_clip(data->>'comment', 65536)",
						"made_at": "archive.sync_at(data->>'created_at')", "edited_at": "archive.sync_at(data->>'updated_at')"},
					Children: []Kids{{Table: "warehouse_column", From: "archive.sync_list(data->'columns') c",
						Cols: map[string]SQL{"position": "archive.sync_num(c->>'position') + 1", "title": "c->>'name'", "type": "c->>'type_text'",
							"nullable": "archive.sync_bool(c->'nullable')", "comment": "c->>'comment'"}}}}},
		},
	})
}

var (
	dbStatement = Obj{"statement": "{query}", "warehouse_id": "{warehouse;meta/warehouse}", "catalog": "{catalog}", "schema": "{schema}",
		"parameters": "{parameters}", "row_limit": "{limit}", "wait_timeout": "50s", "on_wait_timeout": "CANCEL", "disposition": "INLINE", "format": "JSON_ARRAY"}
	dbOut = Obj{"state": "{got/status/state}", "columns": "{got/manifest/schema/columns}", "rows": "{got/result/data_array}",
		"total_row_count": "{got/manifest/total_row_count}", "truncated": "{got/manifest/truncated}", "statement_id": "{got/statement_id}",
		"next_chunk_index": "{got/result/next_chunk_index}"}
)

func dbRun(r Req) []Step {
	return []Step{{As: "got", Req: r}, {If: "got/status/error/message", Fail: "Databricks did not run it: {got/status/error/message}"}}
}

type (
	dbQuery struct {
		Query      string    `json:"query" jsonschema:"The SQL statement, one, with :name markers for its parameters"`
		Parameters []dbParam `json:"parameters,omitempty" jsonschema:"Its named parameters"`
		Catalog    string    `json:"catalog,omitempty" jsonschema:"The catalog unqualified names are in"`
		Schema     string    `json:"schema,omitempty" jsonschema:"The schema unqualified names are in"`
		Warehouse  string    `json:"warehouse,omitempty" jsonschema:"Another SQL warehouse's ID to run it on; the connection's when left out"`
		Limit      int       `json:"limit,omitempty" jsonschema:"At most this many rows, 1 to 10000; 100 when left out"`
	}
	dbParam struct {
		Name  string `json:"name" jsonschema:"Its name, as :name in the statement"`
		Value string `json:"value,omitempty" jsonschema:"Its value, as text; null when left out"`
		Type  string `json:"type,omitempty" jsonschema:"Its SQL type, like STRING, INT, BIGINT, DECIMAL(10,2), DATE or TIMESTAMP; STRING when left out"`
	}
	dbChunk struct {
		StatementID string `json:"statement_id" jsonschema:"The statement_id query answered with"`
		Chunk       int    `json:"chunk" jsonschema:"The next_chunk_index it answered with"`
	}
	dbGet struct {
		Path  string `json:"path" jsonschema:"The endpoint's path, like /api/2.1/unity-catalog/catalogs"`
		Query string `json:"query,omitempty" jsonschema:"Its query string, like catalog_name=main&max_results=100"`
	}
)
