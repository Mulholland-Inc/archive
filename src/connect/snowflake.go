package connect

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

// Snowflake, a company's data warehouse, through its SQL API. A warehouse is too big to copy, so Archive keeps only
// what is in the databases, schemas and tables an owner picks, each table's or view's columns, rows and size, and
// runs SQL there live when asked: query only reads, and execute, which writes, goes only where the role was granted
// it, and is refused to a viewer's token.
//
// It is a key connector with key-pair authentication, which needs nothing of Mulholland's and no browser: the
// account's admin makes a service user with an RSA public key, a role that may SELECT what Archive is to read,
// and write only where the admin chooses, and a default warehouse, and an owner pastes in the account, the user, its private key and the picks.
// Each request is signed with a JWT of the key that lasts an hour. A role that may change the account, like
// SYSADMIN, cannot connect. The key lasts until the admin unsets it from the user.
//
// The SQL API answers rows as lists, which Archive reads into objects by the names of their columns; a statement
// that takes over 45 seconds answers with its handle, which the query action reads again.
//
// It syncs every table and view of each pick into warehouse_table, with its columns into warehouse_column, from
// each database's INFORMATION_SCHEMA, listed whole every six hours, which deletes those gone. Each sync wakes the
// warehouse for a minute or so.
func init() {
	Register(&Connector{
		ID: "snowflake", Name: "Snowflake", Integration: true,
		Info: Info{By: "Snowflake", Description: "Read the tables you pick in your warehouse, query them, and write where you allow", Category: CatData, Icon: IconList,
			Scopes: []string{"Read the tables and views the service user’s role may SELECT, and run SQL on them", "Write only where the role is granted it, when asked"}},
		Keys: []Key{
			{Name: "account", Label: "Account identifier", Help: "Your <org>-<account>, like ACME-PROD, as SELECT CURRENT_ORGANIZATION_NAME() || '-' || CURRENT_ACCOUNT_NAME() gives it; the account's URL is fine"},
			{Name: "user", Label: "Service user", Help: "The user your admin made for Archive, like ARCHIVE"},
			{Name: "key", Label: "Private key", Secret: true,
				Help: "The unencrypted PEM private key whose public key is the user's RSA_PUBLIC_KEY, from -----BEGIN PRIVATE KEY----- to -----END PRIVATE KEY-----"},
			{Name: "scope", Label: "What to sync", Help: "Databases, schemas or tables, comma separated, like ANALYTICS, RAW.SHOPIFY, FINANCE.PUBLIC.FORECAST"},
		},
		Authorize: snowAuthorize,
		Account:   snowWho,
		Answer:    snowRows,
		API:       "https://{-account}.snowflakecomputing.com/api/v2",
		Let:       []Step{{As: "account", In: "{key/account}", Find: `(?i)^(?:[^/]*://)?([^/]*?)(?:\.snowflakecomputing\.com)?(?:/.*)?$`}},
		Actions: map[string]*Action{
			"query": Call[snowQuery, any](Plan{
				Doc: "Run one read-only SQL statement, SELECT, WITH, SHOW, DESCRIBE or EXPLAIN, as the service user's role, like { statement: \"SELECT region, SUM(net_sales) " +
					"FROM analytics.public.daily_sales WHERE day >= DATEADD(month, -3, CURRENT_DATE) GROUP BY 1\" }: { columns: [{ name, type, ... }], rows: [{ <column>: value }], " +
					"rowCount, partitions, handle }, every value a string as Snowflake gives it. The tables there are in warehouse_table and warehouse_column. Rows past the " +
					"first partition come with { handle, partition: n }, as lists in the order of columns. One that runs over 45 seconds answers { handle, message }: ask " +
					"again with { handle } for its rows",
				Steps: []Step{
					{Unless: "handle", Codec: snowReads, In: "{statement}"},
					{Unless: "handle", As: "got", Req: Req{Method: POST, Read: true, URL: "/statements",
						Body: Obj{"statement": "{statement}", "timeout": 600, "database": "{database}", "schema": "{schema}", "parameters": snowParams}}},
					{If: "handle", As: "got", Req: Req{URL: "/statements/{handle}", Query: Q{"partition": "{partition}"}}},
				},
				Out: Obj{"columns": "{got/resultSetMetaData/rowType}", "rows": "{got/data}", "rowCount": "{got/resultSetMetaData/numRows}",
					"partitions": "{got/resultSetMetaData/partitionInfo}", "handle": "{got/statementHandle}", "message": "{got/message}"},
			}),
			// A write goes only where the role was granted it, like a schema of Archive's own; the setup grants none.
			"execute": Call[snowQuery, any](Plan{
				Doc: "Run one SQL statement that writes, like { statement: \"INSERT INTO archive.forecast.weekly (week, sku, units) VALUES ('2026-11-09', 'RUG-8X10', 140)\" } " +
					"or a CREATE TABLE, MERGE, UPDATE or DELETE, as the service user's role, which may write only where its admin granted it: { rowCount, rows, handle, message }; " +
					"rows say how many it inserted, updated or deleted. Use query to read",
				Steps: []Step{
					{Unless: "handle", Codec: snowOne, In: "{statement}"},
					{Unless: "handle", As: "got", Req: Req{Method: POST, URL: "/statements",
						Body: Obj{"statement": "{statement}", "timeout": 600, "database": "{database}", "schema": "{schema}", "parameters": snowParams}}},
					{If: "handle", As: "got", Req: Req{URL: "/statements/{handle}"}},
				},
				Out: Obj{"rows": "{got/data}", "rowCount": "{got/resultSetMetaData/numRows}", "handle": "{got/statementHandle}", "message": "{got/message}"},
			}),
			"cancel": Call[snowCancel, any](Plan{
				Doc:   "Stop a statement the query action left running, by its handle",
				Steps: []Step{{Req: Req{Method: POST, Read: true, URL: "/statements/{handle}/cancel"}}},
			}),
		},
		Sync: []*Sync{
			// A pick is a database, a schema of one or a table of one; the parts are the picks, each listed from its
			// database's INFORMATION_SCHEMA, 200 tables a page after the last one's name.
			{Name: "tables", Gap: time.Second, Every: 6 * time.Hour, Items: "data", ID: "id", Page: Cursor{At: "data/199/id"},
				Parts: &Parts{Req: snowStatement(snowPicks), Items: "data", Vals: Obj{"db": "{db}", "schema": "{schema}", "table": "{table}"}},
				Req:   snowStatement(snowTables),
				// A pick that is not there, or that the role may not read, is passed over until the next sync.
				Pass: []int{http.StatusNotFound},
				Deny: []Deny{{Status: []int{http.StatusUnprocessableEntity}, Says: []string{"warehouse"}, Why: "the service user has no warehouse to run on: {message}"},
					{Status: []int{http.StatusUnprocessableEntity}, Says: []string{"does not exist or not authorized"}, Why: "the service user's role may not read every pick: {message}"}},
				Doc: "Every table and view of the databases, schemas and tables picked, but INFORMATION_SCHEMA's, with its kind, rows, size, comment, clustering key and " +
					"when it was made and last changed, and its columns, by <database>.<schema>.<name>: listed whole every six hours, which deletes those gone.",
				To: Map{Table: "warehouse_table",
					Cols: map[string]SQL{"database": "data->>'database'", "schema": "data->>'schema'", "title": "data->>'name'", "kind": "data->>'kind'",
						"row_count": "archive.sync_num(data->>'rows')", "bytes": "archive.sync_num(data->>'bytes')", "comment": "data->>'comment'",
						"clustering": "data->>'clustering'", "made_at": "archive.sync_at(data->>'created')", "edited_at": "archive.sync_at(data->>'altered')"},
					Children: []Kids{{Table: "warehouse_column", From: "archive.sync_list(data->'columns') c",
						Cols: map[string]SQL{"position": "archive.sync_num(c->>'position')", "title": "c->>'name'", "type": "c->>'type'",
							"nullable": "archive.sync_bool(c->'nullable')", "comment": "c->>'comment'"}}}}},
		},
	})
}

const (
	// snowPicks are the picks of the scope key, each split into its database, schema and table, upper case as
	// Snowflake keeps names it was not given in quotes.
	snowPicks = `SELECT DISTINCT UPPER(TRIM(value)) AS "id", UPPER(SPLIT_PART(TRIM(value), '.', 1)) AS "db", UPPER(SPLIT_PART(TRIM(value), '.', 2)) AS "schema", ` +
		`UPPER(SPLIT_PART(TRIM(value), '.', 3)) AS "table" FROM TABLE(SPLIT_TO_TABLE(REGEXP_REPLACE('{'meta/scope}', '[\\s;]+', ','), ',')) WHERE TRIM(value) <> ''`
	// snowTables are a pick's tables and views, with their columns, 200 a page after the last one's name.
	snowTables = `SELECT t.table_catalog || '.' || t.table_schema || '.' || t.table_name AS "id", t.table_catalog AS "database", t.table_schema AS "schema", ` +
		`t.table_name AS "name", t.table_type AS "kind", t.row_count AS "rows", t.bytes AS "bytes", t.comment AS "comment", t.clustering_key AS "clustering", ` +
		`TO_VARCHAR(t.created, 'YYYY-MM-DD"T"HH24:MI:SS.FF3TZH:TZM') AS "created", TO_VARCHAR(t.last_altered, 'YYYY-MM-DD"T"HH24:MI:SS.FF3TZH:TZM') AS "altered", ` +
		`ARRAY_AGG(IFF(c.column_name IS NULL, NULL, OBJECT_CONSTRUCT('position', c.ordinal_position, 'name', c.column_name, 'type', c.data_type, ` +
		`'nullable', c.is_nullable = 'YES', 'comment', c.comment))) WITHIN GROUP (ORDER BY c.ordinal_position) AS "columns" ` +
		`FROM IDENTIFIER('{'in/db}.INFORMATION_SCHEMA.TABLES') t LEFT JOIN IDENTIFIER('{'in/db}.INFORMATION_SCHEMA.COLUMNS') c ` +
		`ON c.table_schema = t.table_schema AND c.table_name = t.table_name ` +
		`WHERE t.table_schema <> 'INFORMATION_SCHEMA' AND ('{'in/schema}' = '' OR UPPER(t.table_schema) = '{'in/schema}') ` +
		`AND ('{'in/table}' = '' OR UPPER(t.table_name) = '{'in/table}') AND t.table_catalog || '.' || t.table_schema || '.' || t.table_name > '{'page}' ` +
		`GROUP BY 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11 ORDER BY 1 LIMIT 200`
)

// snowParams tag Archive's statements, so the account's query history names them, and keep each to one statement.
var snowParams = Obj{"QUERY_TAG": "archive", "MULTI_STATEMENT_COUNT": "1"}

// snowStatement is a request that runs the statement q.
func snowStatement(q string) Req {
	return Req{Method: POST, Read: true, URL: "/statements", Body: Obj{"statement": q, "timeout": 300, "parameters": snowParams}}
}

type (
	snowQuery struct {
		Statement string `json:"statement,omitempty" jsonschema:"The SQL, one statement that only reads"`
		Database  string `json:"database,omitempty" jsonschema:"The database unqualified names are in"`
		Schema    string `json:"schema,omitempty" jsonschema:"The schema unqualified names are in"`
		Handle    string `json:"handle,omitempty" jsonschema:"The handle of a statement run before, to read its rows in place of running one"`
		Partition int    `json:"partition,omitempty" jsonschema:"With handle, which partition of its rows, from 0"`
	}
	snowCancel struct {
		Handle string `json:"handle" jsonschema:"The statement's handle"`
	}
)

// snowRows is an answer of the SQL API with its rows, lists, as objects by the names of their columns, and a VARIANT,
// OBJECT or ARRAY read out of the JSON Snowflake gives it as. An answer without the columns, like a later
// partition's, stays as it is.
var snowRows = &Codec{"snowflake_rows", func(v any) (any, error) {
	m, _ := v.(map[string]any)
	rows, ok := m["data"].([]any)
	cols := listOf(pick(m, "resultSetMetaData/rowType"))
	if !ok || len(cols) == 0 {
		return v, nil
	}
	out := make([]any, len(rows))
	for i, r := range rows {
		vals := listOf(r)
		o := map[string]any{}
		for j, col := range cols {
			if j >= len(vals) {
				break
			}
			x := vals[j]
			if s, ok := x.(string); ok && slices.Contains([]string{"variant", "object", "array"}, strings.ToLower(text(pick(col, "type")))) {
				if y, err := readAll([]byte(s), "Snowflake"); err == nil {
					x = y
				}
			}
			o[text(pick(col, "name"))] = x
		}
		out[i] = o
	}
	m = maps.Clone(m)
	m["data"] = out
	return m, nil
}}

// snowReads passes one statement that only reads, by its first word, and refuses any other. The role's grants are what
// keep it from writing; this says so before Snowflake does.
var snowReads = &Codec{"snowflake_reads", func(v any) (any, error) {
	first, err := snowFirst(text(v))
	if err != nil {
		return nil, err
	}
	if !slices.Contains([]string{"SELECT", "WITH", "SHOW", "DESCRIBE", "DESC", "EXPLAIN"}, first) {
		return nil, fmt.Errorf("the query action only reads: a statement starts with SELECT, WITH, SHOW, DESCRIBE or EXPLAIN, not %q; execute runs one that writes", first)
	}
	return text(v), nil
}}

// snowOne passes one statement of any kind.
var snowOne = &Codec{"snowflake_one", func(v any) (any, error) {
	if _, err := snowFirst(text(v)); err != nil {
		return nil, err
	}
	return text(v), nil
}}

// snowFirst is the first word of q, upper case, once q is one statement: comments, strings and quoted names are
// skipped, and a semicolon may only end it.
func snowFirst(q string) (string, error) {
	first, ended := "", false
	for i := 0; i < len(q); {
		rest := q[i:]
		switch {
		case strings.HasPrefix(rest, "--"), strings.HasPrefix(rest, "//"):
			n := strings.IndexByte(rest, '\n')
			if n < 0 {
				n = len(rest) - 1
			}
			i += n + 1
		case strings.HasPrefix(rest, "/*"):
			n := strings.Index(rest[2:], "*/")
			if n < 0 {
				return "", errors.New("the statement has a comment that does not end")
			}
			i += n + 4
		case strings.HasPrefix(rest, "$$"):
			n := strings.Index(rest[2:], "$$")
			if n < 0 {
				return "", errors.New("the statement has a $$ string that does not end")
			}
			i += n + 4
		case rest[0] == '\'' || rest[0] == '"':
			j := 1
			for ; j < len(rest); j++ {
				if rest[j] == '\\' && rest[0] == '\'' {
					j++
				} else if rest[j] == rest[0] {
					if j+1 < len(rest) && rest[j+1] == rest[0] {
						j++
					} else {
						break
					}
				}
			}
			if j >= len(rest) {
				return "", errors.New("the statement has a quote that does not end")
			}
			i += j + 1
		case rest[0] == ';':
			ended = true
			i++
		case rest[0] == ' ' || rest[0] == '\t' || rest[0] == '\n' || rest[0] == '\r':
			i++
		default:
			if ended {
				return "", errors.New("Snowflake runs one statement at a time here; send each on its own")
			}
			if first == "" {
				first = strings.ToUpper(snowWord.FindString(rest))
			}
			i++
		}
	}
	return first, nil
}

var (
	snowWord  = regexp.MustCompile(`^[A-Za-z_]*`)
	snowArmor = regexp.MustCompile(`-----[A-Z ]+-----`)
	// snowAdmin are roles that may change the account, which Archive does not connect as.
	snowAdmin = []string{"ACCOUNTADMIN", "ORGADMIN", "SECURITYADMIN", "SYSADMIN", "USERADMIN", "GLOBALORGADMIN"}
)

// snowWho checks the key, and names the account by its organization and name, once the user's role is not one that
// may change the whole account and it has a warehouse.
func snowWho(c *Client) (string, error) {
	if _, err := snowKey(c.Key("key")); err != nil {
		return "", err
	}
	ans, err := c.req(snowStatement(`SELECT CURRENT_ORGANIZATION_NAME() || '-' || CURRENT_ACCOUNT_NAME() AS "account", CURRENT_ROLE() AS "role", CURRENT_WAREHOUSE() AS "warehouse"`), map[string]any{})
	if err != nil {
		return "", err
	}
	role := strings.ToUpper(text(pick(ans, "data/0/role")))
	switch {
	case slices.Contains(snowAdmin, role):
		return "", fmt.Errorf("the user's default role is %s, which may change the whole account; give it a role of its own, like ARCHIVE", role)
	case text(pick(ans, "data/0/warehouse")) == "":
		return "", errors.New("the user has no warehouse to run on; give it a DEFAULT_WAREHOUSE its role may use")
	}
	return text(pick(ans, "data/0/account")), nil
}

// snowAuthorize signs a request with a JWT of the account's key; one with a key that is no key goes unsigned, for
// Snowflake to refuse.
func snowAuthorize(r *http.Request, c *Client) {
	r.Header.Set("User-Agent", "Archive")
	if t, err := snowJWT(c.Key("account"), c.Key("user"), c.Key("key")); err == nil {
		r.Header.Set("Authorization", "Bearer "+t)
		r.Header.Set("X-Snowflake-Authorization-Token-Type", "KEYPAIR_JWT")
	}
}

// snowTokens are the JWTs of keys, kept until shortly before their hour is up.
var snowTokens = struct {
	sync.Mutex
	m map[[32]byte]snowToken
}{m: map[[32]byte]snowToken{}}

type snowToken struct {
	token string
	until time.Time
}

// snowJWT is a JWT of user of account, signed with key, as Snowflake's key-pair authentication has it: issued by
// <ACCOUNT>.<USER>.SHA256:<the public key's fingerprint>, for <ACCOUNT>.<USER>, the account without its region.
func snowJWT(account, user, key string) (string, error) {
	id := sha256.Sum256([]byte(account + "\x00" + user + "\x00" + key))
	snowTokens.Lock()
	defer snowTokens.Unlock()
	if t := snowTokens.m[id]; time.Now().Before(t.until) {
		return t.token, nil
	}
	k, err := snowKey(key)
	if err != nil {
		return "", err
	}
	pub, err := x509.MarshalPKIXPublicKey(&k.PublicKey)
	if err != nil {
		return "", err
	}
	fp := sha256.Sum256(pub)
	acct, _, _ := strings.Cut(snowAccount(account), ".")
	sub := strings.ToUpper(acct) + "." + strings.ToUpper(strings.TrimSpace(user))
	now := time.Now()
	head, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	claims, _ := json.Marshal(map[string]any{"iss": sub + ".SHA256:" + base64.StdEncoding.EncodeToString(fp[:]), "sub": sub, "iat": now.Unix(), "exp": now.Add(time.Hour).Unix()})
	msg := base64.RawURLEncoding.EncodeToString(head) + "." + base64.RawURLEncoding.EncodeToString(claims)
	sum := sha256.Sum256([]byte(msg))
	sig, err := rsa.SignPKCS1v15(rand.Reader, k, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	tok := msg + "." + base64.RawURLEncoding.EncodeToString(sig)
	snowTokens.m[id] = snowToken{tok, now.Add(50 * time.Minute)}
	return tok, nil
}

// snowAccount is the account of an identifier or URL, as its host names it: xy12345.us-east-2.aws or acme-prod.
func snowAccount(s string) string {
	s = strings.TrimSpace(s)
	if _, after, ok := strings.Cut(s, "://"); ok {
		s = after
	}
	s, _, _ = strings.Cut(s, "/")
	return strings.TrimSuffix(strings.ToLower(s), ".snowflakecomputing.com")
}

// snowKey is the RSA private key of a PEM, its lines maybe run together as a form's field leaves them, PKCS #8 or #1.
func snowKey(s string) (*rsa.PrivateKey, error) {
	if strings.Contains(s, "ENCRYPTED") {
		return nil, errors.New("that private key is encrypted; paste one made with -nocrypt")
	}
	der, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(snowArmor.ReplaceAllString(s, "")), ""))
	if err != nil {
		return nil, errors.New("that is not a PEM private key")
	}
	if k, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		if r, ok := k.(*rsa.PrivateKey); ok {
			return r, nil
		}
		return nil, errors.New("Snowflake takes an RSA key, and that is another kind")
	}
	if r, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return r, nil
	}
	return nil, errors.New("that is not an RSA private key")
}
