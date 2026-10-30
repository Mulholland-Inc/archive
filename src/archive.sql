CREATE SCHEMA archive;

CREATE TABLE archive.space (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    name       text NOT NULL UNIQUE,
    role       text NOT NULL UNIQUE,
    home       uuid UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE archive.workspace (
    id            uuid PRIMARY KEY DEFAULT uuidv7(),
    name          text NOT NULL,
    key           bytea NOT NULL,
    anthropic_key bytea,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE archive."user" (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    email      text NOT NULL UNIQUE,
    name       text NOT NULL DEFAULT '',
    workspaces uuid[] NOT NULL DEFAULT '{}',
    signin     jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX user_workspaces_idx ON archive."user" USING gin (workspaces);

CREATE TABLE archive.token (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id    uuid NOT NULL REFERENCES archive."user" ON DELETE CASCADE,
    workspace  uuid REFERENCES archive.workspace ON DELETE CASCADE,
    kind       text NOT NULL,
    name       text NOT NULL DEFAULT '',
    role       text UNIQUE,
    hash       text NOT NULL UNIQUE,
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX token_user_idx ON archive.token (user_id);

CREATE TABLE archive.chat (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id    uuid NOT NULL,
    title      text NOT NULL DEFAULT '',
    messages   jsonb NOT NULL DEFAULT '[]',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX chat_user_idx ON archive.chat (user_id, updated_at);

CREATE TABLE archive.workflow (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id    uuid NOT NULL,
    role       text NOT NULL,
    name       text NOT NULL,
    source     text NOT NULL,
    trigger    text NOT NULL DEFAULT '',
    hook       text NOT NULL UNIQUE,
    next_at    timestamptz,
    cursor_at  timestamptz,
    cursor_id  uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, name)
);
CREATE INDEX workflow_next_idx ON archive.workflow (next_at) WHERE next_at IS NOT NULL;

CREATE TABLE archive.workflow_run (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    workflow   uuid NOT NULL REFERENCES archive.workflow ON DELETE CASCADE,
    trigger    text NOT NULL,
    result     jsonb NOT NULL DEFAULT 'null',
    logs       jsonb NOT NULL DEFAULT '[]',
    error      text NOT NULL DEFAULT '',
    ms         integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX workflow_run_idx ON archive.workflow_run (workflow, created_at);

CREATE TABLE archive.revision (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    hash       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE archive.due (
    workspace uuid NOT NULL REFERENCES archive.workspace ON DELETE CASCADE,
    kind      text NOT NULL,
    at        timestamptz NOT NULL,
    PRIMARY KEY (workspace, kind)
);
CREATE INDEX due_at_idx ON archive.due (at);

CREATE FUNCTION archive.search_fold(t text) RETURNS text LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT translate(replace(replace(replace(lower(regexp_replace(regexp_replace(normalize(coalesce(t, ''), NFD),
        '[̀-ͯ]', '', 'g'), '[._/\\]+', ' ', 'g')), 'ß', 'ss'), 'æ', 'ae'), 'œ', 'oe'), 'øłđ', 'old')
$$;

CREATE FUNCTION archive.search_doc(VARIADIC parts text[]) RETURNS tsvector LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT to_tsvector('simple', archive.search_fold(left(array_to_string(parts, ' '), 250000)))
$$;

CREATE FUNCTION archive.search_query(q text) RETURNS tsquery LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT array_to_string(array(SELECT quote_literal(l) || ':*' FROM unnest(to_tsvector('simple', archive.search_fold(q))) u(l)), ' | ')::tsquery
$$;

CREATE TABLE archive.file (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    hash       text NOT NULL,
    name       text NOT NULL,
    mime       text NOT NULL,
    size       bigint NOT NULL,
    metadata   jsonb NOT NULL DEFAULT '{}',
    text       text NOT NULL DEFAULT '',
    status     text NOT NULL DEFAULT 'pending',
    error      text NOT NULL DEFAULT '',
    space      uuid NOT NULL REFERENCES archive.space,
    created_at timestamptz NOT NULL DEFAULT now(),
    search     tsvector GENERATED ALWAYS AS (setweight(archive.search_doc(name), 'A') || setweight(archive.search_doc(text), 'C')) STORED,
    UNIQUE (space, hash, name)
);
CREATE INDEX file_pending_idx ON archive.file (id) WHERE status = 'pending';
CREATE INDEX file_search_idx ON archive.file USING gin (search);

CREATE TABLE archive.upload (
    id         uuid PRIMARY KEY,
    role       text NOT NULL,
    space      uuid NOT NULL REFERENCES archive.space ON DELETE CASCADE,
    name       text NOT NULL,
    length     bigint NOT NULL,
    parts      bigint[] NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE archive.secret (
    id          uuid PRIMARY KEY,
    table_name  text NOT NULL,
    row_id      uuid NOT NULL,
    column_name text NOT NULL,
    type        text NOT NULL,
    value       bytea NOT NULL,
    space       uuid NOT NULL REFERENCES archive.space,
    UNIQUE (table_name, row_id, column_name)
);
CREATE INDEX secret_space_idx ON archive.secret (space);

CREATE FUNCTION archive.my_spaces(lvl text DEFAULT 'viewer') RETURNS uuid[] LANGUAGE sql STABLE AS $$
    SELECT coalesce(array_agg(id), '{}') FROM archive.space WHERE pg_has_role(current_user, (role || '_' || lvl)::name, 'USAGE')
        AND coalesce(nullif(current_setting('archive.space', true), '') = name, true)
$$;

CREATE FUNCTION archive.ontology_admin() RETURNS bool LANGUAGE sql STABLE AS $$
    SELECT pg_has_role(current_user, '{p}_admin', 'USAGE') OR cardinality(archive.my_spaces('admin')) > 0
$$;

CREATE FUNCTION archive.secret_granted(typ text) RETURNS bool LANGUAGE sql STABLE AS $$
    SELECT EXISTS (SELECT FROM pg_roles WHERE rolname = '{p}_secret_' || typ AND pg_has_role(current_user, oid, 'USAGE'))
$$;

CREATE FUNCTION archive.secret_clear(tbl text, rid uuid, col text) RETURNS void LANGUAGE sql SECURITY DEFINER SET search_path = pg_catalog AS $$
    DELETE FROM archive.secret s USING archive.space sp
    WHERE s.table_name = tbl AND s.row_id = rid AND (col IS NULL OR s.column_name = col) AND sp.id = s.space
      AND pg_has_role(current_setting('role')::name, (sp.role || '_editor')::name, 'USAGE')
$$;

GRANT USAGE ON SCHEMA archive TO "{p}_user";
GRANT SELECT ON archive.space TO "{p}_user";
GRANT SELECT, INSERT, UPDATE, DELETE ON archive.file, archive.secret TO "{p}_user";
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO "{p}_user";

ALTER TABLE archive.space ENABLE ROW LEVEL SECURITY;
CREATE POLICY member ON archive.space FOR SELECT USING (pg_has_role(current_user, (role || '_viewer')::name, 'USAGE'));

ALTER TABLE archive.file ENABLE ROW LEVEL SECURITY;
CREATE POLICY viewer ON archive.file FOR SELECT USING (space = ANY (archive.my_spaces()));
CREATE POLICY editor ON archive.file USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));


ALTER TABLE archive.secret ENABLE ROW LEVEL SECURITY;
CREATE POLICY viewer ON archive.secret FOR SELECT USING (space = ANY (archive.my_spaces()) AND archive.secret_granted(type));
CREATE POLICY editor_insert ON archive.secret FOR INSERT WITH CHECK (space = ANY (archive.my_spaces('editor')));
CREATE POLICY editor_update ON archive.secret FOR UPDATE USING (space = ANY (archive.my_spaces('editor'))) WITH CHECK (space = ANY (archive.my_spaces('editor')));
CREATE POLICY editor_delete ON archive.secret FOR DELETE USING (space = ANY (archive.my_spaces('editor')));

CREATE DOMAIN archive.duration AS text CHECK (VALUE ~ '^P([0-9]+Y)?([0-9]+M)?([0-9]+D)?(T([0-9]+H)?([0-9]+M)?([0-9]+(\.[0-9]+)?S)?)?$');
COMMENT ON DOMAIN archive.duration IS 'ISO 8601 duration';
CREATE DOMAIN archive.timezone AS text CHECK (VALUE ~ '^[A-Za-z][A-Za-z0-9_+\-/]*$');
COMMENT ON DOMAIN archive.timezone IS 'IANA time zone';
CREATE DOMAIN archive.recurrence AS text CHECK (VALUE ~ '^FREQ=(SECONDLY|MINUTELY|HOURLY|DAILY|WEEKLY|MONTHLY|YEARLY)(;[A-Z]+=[A-Z0-9,+\-]+)*$');
COMMENT ON DOMAIN archive.recurrence IS 'RFC 5545 recurrence rule';
CREATE DOMAIN archive.currency AS text CHECK (VALUE ~ '^[A-Z]{3}$');
COMMENT ON DOMAIN archive.currency IS 'ISO 4217 currency code';
CREATE DOMAIN archive.percentage AS numeric;
COMMENT ON DOMAIN archive.percentage IS 'Fraction, 0.21 is 21%';
CREATE DOMAIN archive.iban AS text CHECK (VALUE ~ '^((NO)[0-9]{2}[A-Z0-9]{11}|(BE)[0-9]{2}[A-Z0-9]{12}|(DK|FI|FK|FO|GL|NL|SD)[0-9]{2}[A-Z0-9]{14}|(MK|SI)[0-9]{2}[A-Z0-9]{15}|(AT|BA|EE|KZ|LT|LU|MN|XK)[0-9]{2}[A-Z0-9]{16}|(CH|HR|LI|LV)[0-9]{2}[A-Z0-9]{17}|(BG|BH|CR|DE|GB|GE|IE|ME|RS|VA)[0-9]{2}[A-Z0-9]{18}|(AE|GI|IL|IQ|OM|SO|TL)[0-9]{2}[A-Z0-9]{19}|(AD|CZ|ES|MD|PK|RO|SA|SE|SK|TN|VG)[0-9]{2}[A-Z0-9]{20}|(LY|PT|ST)[0-9]{2}[A-Z0-9]{21}|(IS|TR)[0-9]{2}[A-Z0-9]{22}|(BI|DJ|FR|GR|IT|MC|MR|SM)[0-9]{2}[A-Z0-9]{23}|(AL|AZ|BY|CY|DO|GT|HU|LB|NI|PL|SV)[0-9]{2}[A-Z0-9]{24}|(BR|EG|PS|QA|UA)[0-9]{2}[A-Z0-9]{25}|(JO|KW|MU|YE)[0-9]{2}[A-Z0-9]{26}|(MT|SC)[0-9]{2}[A-Z0-9]{27}|(LC)[0-9]{2}[A-Z0-9]{28}|(RU)[0-9]{2}[A-Z0-9]{29})$');
COMMENT ON DOMAIN archive.iban IS 'ISO 13616 bank account number';
CREATE DOMAIN archive.bic AS text CHECK (VALUE ~ '^[A-Z]{6}[A-Z0-9]{2}([A-Z0-9]{3})?$');
COMMENT ON DOMAIN archive.bic IS 'ISO 9362 bank identifier';
CREATE DOMAIN archive.card AS text;
COMMENT ON DOMAIN archive.card IS 'Payment card, stored encrypted; the column holds its mask';
CREATE DOMAIN archive.us_routing AS text CHECK (VALUE ~ '^[0-9]{9}$');
COMMENT ON DOMAIN archive.us_routing IS 'US ABA routing number';
CREATE DOMAIN archive.lei AS text CHECK (VALUE ~ '^[A-Z0-9]{18}[0-9]{2}$');
COMMENT ON DOMAIN archive.lei IS 'ISO 17442 legal entity identifier';
CREATE DOMAIN archive.isin AS text CHECK (VALUE ~ '^[A-Z]{2}[A-Z0-9]{9}[0-9]$');
COMMENT ON DOMAIN archive.isin IS 'ISO 6166 securities identifier';
CREATE DOMAIN archive.vat_id AS text CHECK (VALUE ~ '^(ATU[0-9]{8}|BE[01][0-9]{9}|BG[0-9]{9,10}|CY[0-9]{8}[A-Z]|CZ[0-9]{8,10}|DE[0-9]{9}|DK[0-9]{8}|EE[0-9]{9}|EL[0-9]{9}|ES[A-Z0-9][0-9]{7}[A-Z0-9]|FI[0-9]{8}|FR[A-HJ-NP-Z0-9]{2}[0-9]{9}|HR[0-9]{11}|HU[0-9]{8}|IE([0-9]{7}[A-W][A-I]?|[0-9][A-Z+*][0-9]{5}[A-W])|IT[0-9]{11}|LT([0-9]{9}|[0-9]{12})|LU[0-9]{8}|LV[0-9]{11}|MT[0-9]{8}|NL[0-9]{9}B[0-9]{2}|PL[0-9]{10}|PT[0-9]{9}|RO[0-9]{2,10}|SE[0-9]{12}|SI[0-9]{8}|SK[0-9]{10}|(GB|XI)([0-9]{9}|[0-9]{12}|GD[0-9]{3}|HA[0-9]{3}))$');
COMMENT ON DOMAIN archive.vat_id IS 'EU or UK VAT number with its country prefix';
CREATE DOMAIN archive.us_ein AS text CHECK (VALUE ~ '^(0[1-6]|1[0-6]|2[0-7]|3[0-9]|4[0-8]|5[0-9]|6[0-8]|7[1-7]|8[0-8]|9[0-58-9])-[0-9]{7}$');
COMMENT ON DOMAIN archive.us_ein IS 'US employer identification number';
CREATE DOMAIN archive.us_ssn AS text;
COMMENT ON DOMAIN archive.us_ssn IS 'US social security number, stored encrypted; the column holds its mask';
CREATE DOMAIN archive.nl_bsn AS text;
COMMENT ON DOMAIN archive.nl_bsn IS 'Dutch citizen service number, stored encrypted; the column holds its mask';
CREATE DOMAIN archive.national AS text;
COMMENT ON DOMAIN archive.national IS 'National id number with its country and scheme, stored encrypted; the column holds its mask';
CREATE DOMAIN archive.passport AS text;
COMMENT ON DOMAIN archive.passport IS 'Passport number with its issuing country, stored encrypted; the column holds its mask';
CREATE DOMAIN archive.email AS text CHECK (VALUE ~ '^[^@\s]+@[^@\s]+$');
COMMENT ON DOMAIN archive.email IS 'Email address; the domain is lowercase and punycode';
CREATE DOMAIN archive.phone AS text CHECK (VALUE ~ '^\+[1-9][0-9]{6,14}$');
COMMENT ON DOMAIN archive.phone IS 'Phone number in E.164';
CREATE DOMAIN archive.url AS text;
COMMENT ON DOMAIN archive.url IS 'Web address, WHATWG-normalized';
CREATE DOMAIN archive.country AS text CHECK (VALUE ~ '^[A-Z]{2}$');
COMMENT ON DOMAIN archive.country IS 'ISO 3166-1 alpha-2 country code';
CREATE DOMAIN archive.region AS text CHECK (VALUE ~ '^[A-Z]{2}-[A-Z0-9]{1,3}$');
COMMENT ON DOMAIN archive.region IS 'ISO 3166-2 subdivision code';
CREATE DOMAIN archive.locale AS text CHECK (VALUE ~ '^[a-zA-Z]{2,8}(-[a-zA-Z0-9]{1,8})*$');
COMMENT ON DOMAIN archive.locale IS 'BCP 47 language tag';
CREATE DOMAIN archive.gtin AS text CHECK (VALUE ~ '^[0-9]{14}$');
COMMENT ON DOMAIN archive.gtin IS 'GS1 trade item number as 14 digits; accepts EAN-13 and UPC-A';
CREATE DOMAIN archive.isbn AS text CHECK (VALUE ~ '^97[89][0-9]{10}$');
COMMENT ON DOMAIN archive.isbn IS 'ISBN-13; ISBN-10 is converted';
CREATE DOMAIN archive.ip AS inet;
COMMENT ON DOMAIN archive.ip IS 'IPv4 or IPv6 address';
CREATE DOMAIN archive.domain AS text CHECK (VALUE ~ '^[a-z0-9]([a-z0-9.\-]*[a-z0-9])?$');
COMMENT ON DOMAIN archive.domain IS 'Domain name, lowercase punycode';
CREATE DOMAIN archive.mime AS text CHECK (VALUE ~ '^[a-z]+/[a-z0-9!#$&^_.+\-]+(;.*)?$');
COMMENT ON DOMAIN archive.mime IS 'IANA media type';
CREATE DOMAIN archive.richtext AS text;
COMMENT ON DOMAIN archive.richtext IS 'Markdown (CommonMark)';

CREATE TYPE archive.timestamp AS (utc timestamptz, zone archive.timezone);
COMMENT ON TYPE archive.timestamp IS 'Instant in UTC, with the IANA zone it was given in';
CREATE TYPE archive.time AS (time time, zone archive.timezone);
COMMENT ON TYPE archive.time IS 'Time of day, with an optional IANA zone';
CREATE TYPE archive.interval AS (start text, "end" text);
COMMENT ON TYPE archive.interval IS 'Start and end, each a date or UTC instant; null is open';
CREATE TYPE archive.money AS (amount numeric, currency archive.currency);
COMMENT ON TYPE archive.money IS 'Amount in the currency minor units, with its currency';
CREATE TYPE archive.address AS (lines text[], locality text, region archive.region, postcode text, country archive.country);
COMMENT ON TYPE archive.address IS 'Postal address';
CREATE TYPE archive.name AS ("full" text, given text, family text, prefixes text);
COMMENT ON TYPE archive.name IS 'Person name: the full original, and its parts when known';
CREATE TYPE archive.postcode AS (country archive.country, code text);
COMMENT ON TYPE archive.postcode IS 'Postcode with the country it is valid in';
CREATE TYPE archive.geo AS (lat double precision, lon double precision);
COMMENT ON TYPE archive.geo IS 'WGS 84 latitude and longitude';
CREATE TYPE archive.quantity AS (value double precision, unit text, original_unit text);
COMMENT ON TYPE archive.quantity IS 'Value in the SI base unit, with the UCUM unit it was given in';

CREATE TABLE archive.composite_use (
    "timestamp" archive.timestamp,
    "time"      archive.time,
    "interval"  archive.interval,
    money       archive.money,
    address     archive.address,
    name        archive.name,
    postcode    archive.postcode,
    geo         archive.geo,
    quantity    archive.quantity
);
COMMENT ON TABLE archive.composite_use IS 'Always empty; it uses every built-in composite type, so pg-schema-diff sees them as used by a table before and after a user table starts using one';

-- name: HasRole :one
SELECT pg_has_role(sqlc.arg(member)::name, rolname, 'USAGE') FROM pg_catalog.pg_roles WHERE rolname = sqlc.arg(grp)::name;

-- name: RoleExists :one
SELECT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = sqlc.arg(role)::name);

-- name: DirectRoles :many
SELECT r.rolname::text FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles r ON r.oid = m.roleid JOIN pg_catalog.pg_roles u ON u.oid = m.member
WHERE u.rolname = sqlc.arg(member)::name AND r.rolname = ANY(sqlc.arg(grps)::name[]);

-- name: CountOwners :one
SELECT count(*) FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles r ON r.oid = m.roleid
JOIN pg_catalog.pg_roles u ON u.oid = m.member WHERE r.rolname = sqlc.arg(owner)::name AND starts_with(u.rolname, sqlc.arg(users)::text);

-- name: CreateSpace :one
INSERT INTO archive.space (name, role, home) VALUES ($1, $2, $3) RETURNING id;

-- name: SpaceTaken :one
SELECT EXISTS (SELECT FROM archive.space WHERE name = $1);

-- name: SpaceRole :one
SELECT role FROM archive.space WHERE name = $1;

-- name: SpaceRoleByID :one
SELECT role FROM archive.space WHERE id = $1;

-- name: SpaceByName :one
SELECT id, role, home FROM archive.space WHERE name = $1;

-- name: SpaceRoles :many
SELECT name, role, home FROM archive.space ORDER BY name;

-- name: HomeSpace :one
SELECT id FROM archive.space WHERE home = $1;

-- name: Spaces :many
SELECT id, name, CASE
    WHEN pg_has_role(current_user, (role || '_owner')::name, 'USAGE') THEN 'owner'
    WHEN pg_has_role(current_user, (role || '_admin')::name, 'USAGE') THEN 'admin'
    WHEN pg_has_role(current_user, (role || '_editor')::name, 'USAGE') THEN 'editor'
    ELSE 'viewer' END::text AS level
FROM archive.space ORDER BY name;

-- name: DeleteSpaceSecrets :exec
DELETE FROM archive.secret WHERE space = $1;

-- name: DeleteSpaceFiles :exec
DELETE FROM archive.file WHERE space = $1;

-- name: DeleteSpace :exec
DELETE FROM archive.space WHERE id = $1;

-- name: UserByEmail :one
SELECT * FROM archive."user" WHERE email = $1;

-- name: UserByID :one
SELECT * FROM archive."user" WHERE id = $1;

-- name: UserByOIDC :one
SELECT * FROM archive."user" WHERE signin->'oidc' @> sqlc.arg(link)::jsonb;

-- name: WorkspaceUser :one
SELECT * FROM archive."user" WHERE email = sqlc.arg(email) AND workspaces @> ARRAY[sqlc.arg(workspace)::uuid];

-- name: FirstUser :one
SELECT * FROM archive."user" WHERE workspaces @> ARRAY[sqlc.arg(workspace)::uuid] ORDER BY created_at, id LIMIT 1;

-- name: NoMembers :one
SELECT NOT EXISTS (SELECT FROM archive."user" WHERE workspaces @> ARRAY[sqlc.arg(workspace)::uuid]);

-- name: WorkspaceUsers :many
SELECT id, email, name FROM archive."user" WHERE workspaces @> ARRAY[sqlc.arg(workspace)::uuid] ORDER BY email;

-- name: UserWorkspaces :many
SELECT w.id, w.name FROM archive.workspace w JOIN archive."user" u ON w.id = ANY(u.workspaces) WHERE u.id = $1 ORDER BY array_position(u.workspaces, w.id);

-- name: JoinWorkspace :one
INSERT INTO archive."user" (email, name, workspaces) VALUES (sqlc.arg(email), sqlc.arg(name), ARRAY[sqlc.arg(workspace)::uuid])
ON CONFLICT (email) DO UPDATE SET workspaces = archive."user".workspaces || EXCLUDED.workspaces WHERE NOT archive."user".workspaces @> EXCLUDED.workspaces
RETURNING *;

-- name: LeaveWorkspace :exec
UPDATE archive."user" SET workspaces = array_remove(workspaces, sqlc.arg(workspace)::uuid) WHERE id = sqlc.arg(id);

-- name: SetSignin :exec
UPDATE archive."user" SET signin = $2 WHERE id = $1;

-- name: NameUser :exec
UPDATE archive."user" SET name = $2 WHERE id = $1 AND name = '';

-- name: Authenticate :one
SELECT t.id, t.kind, t.name, coalesce(t.role, '')::text AS role, u.id AS user_id, u.email, u.name AS user_name,
    coalesce(t.workspace::text, '')::text AS workspace,
    ARRAY(SELECT w.id::text FROM archive.workspace w WHERE w.id = ANY(u.workspaces) ORDER BY array_position(u.workspaces, w.id))::text[] AS workspaces
FROM archive.token t JOIN archive."user" u ON u.id = t.user_id
WHERE t.hash = $1 AND t.kind IN ('api', 'session', 'user') AND (t.expires_at IS NULL OR t.expires_at > now());

-- name: CreateSession :exec
INSERT INTO archive.token (user_id, kind, hash, expires_at) VALUES ($1, 'session', $2, now() + interval '30 days');

-- name: SignOut :exec
DELETE FROM archive.token WHERE hash = $1 AND kind = 'session';

-- name: TokenRoles :many
SELECT coalesce(role, '')::text FROM archive.token WHERE user_id = $1 AND workspace = $2 AND role IS NOT NULL;

-- name: CreateUserToken :one
INSERT INTO archive.token (id, user_id, workspace, kind, name, hash, expires_at) VALUES ($1, $2, $3, 'user', $4, $5, $6) RETURNING created_at, expires_at;

-- name: CreateAPIToken :one
INSERT INTO archive.token (id, user_id, workspace, kind, name, role, hash, expires_at) VALUES ($1, $2, $3, 'api', $4, $5, $6, $7) RETURNING created_at, expires_at;

-- name: Tokens :many
SELECT id, name, coalesce(role, '')::text AS role, kind = 'user' AS full, expires_at, created_at FROM archive.token
WHERE user_id = $1 AND workspace = $2 AND kind IN ('api', 'user') ORDER BY id;

-- name: RevokeToken :one
DELETE FROM archive.token WHERE id = $1 AND user_id = $2 AND workspace = $3 AND kind IN ('api', 'user') RETURNING coalesce(role, '')::text;

-- name: DeleteWorkspaceTokens :many
DELETE FROM archive.token WHERE user_id = $1 AND workspace = $2 RETURNING coalesce(role, '')::text;

-- name: ForgetUser :exec
WITH gone AS (DELETE FROM archive.chat c WHERE c.user_id = sqlc.arg(id)::uuid) DELETE FROM archive.workflow w WHERE w.user_id = sqlc.arg(id)::uuid;

-- name: Chats :many
SELECT id, title, created_at, updated_at FROM archive.chat WHERE user_id = $1 ORDER BY updated_at DESC LIMIT 200;

-- name: ChatByID :one
SELECT * FROM archive.chat WHERE id = $1 AND user_id = $2;

-- name: CreateChat :one
INSERT INTO archive.chat (user_id, title) VALUES ($1, $2) RETURNING *;

-- name: AppendChat :exec
UPDATE archive.chat SET messages = messages || sqlc.arg(messages)::jsonb, updated_at = now() WHERE id = sqlc.arg(id);

-- name: RenameChat :execrows
UPDATE archive.chat SET title = $3 WHERE id = $1 AND user_id = $2;

-- name: DeleteChat :execrows
DELETE FROM archive.chat WHERE id = $1 AND user_id = $2;

-- name: SpaceID :one
SELECT id FROM archive.space WHERE name = $1;

-- name: DefaultSpace :one
SELECT id FROM archive.space WHERE id = ANY(archive.my_spaces('editor')) ORDER BY home IS NOT DISTINCT FROM sqlc.arg(owner)::uuid DESC, id LIMIT 1;

-- name: CanWrite :one
SELECT sqlc.arg(space)::uuid = ANY(archive.my_spaces('editor'));

-- name: OntologyAdmin :one
SELECT archive.ontology_admin();

-- name: InputErrors :many
SELECT i::bigint AS i, e.message::text AS message
FROM generate_subscripts(sqlc.arg(texts)::text[], 1) i, pg_input_error_info((sqlc.arg(texts)::text[])[i], (sqlc.arg(regs)::text[])[i]) e
WHERE e.message IS NOT NULL ORDER BY i;

-- name: ClearSecrets :exec
SELECT archive.secret_clear(sqlc.arg(tbl), sqlc.arg(rid), sqlc.narg(col));

-- name: CreateSecret :exec
INSERT INTO archive.secret (id, table_name, row_id, column_name, type, value, space) VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: Secrets :many
SELECT id, row_id, column_name, type, value FROM archive.secret WHERE table_name = $1 AND row_id = ANY(sqlc.arg(ids)::uuid[]);

-- name: FileByKey :one
SELECT id, hash, name, mime, size, metadata, status, error, space, created_at FROM archive.file WHERE space = $1 AND hash = $2 AND name = $3;

-- name: CreateFile :one
INSERT INTO archive.file (hash, name, mime, size, space) VALUES ($1, $2, $3, $4, $5)
RETURNING id, hash, name, mime, size, metadata, status, error, space, created_at;

-- name: Files :many
SELECT id, hash, name, mime, size, metadata, status, error, space, created_at FROM archive.file
WHERE id > sqlc.arg(after) AND (sqlc.narg(space)::uuid IS NULL OR space = sqlc.narg(space)) ORDER BY id LIMIT sqlc.arg(lim);

-- name: GetFile :one
SELECT id, hash, name, mime, size, metadata, status, error, space, created_at FROM archive.file WHERE id = $1;

-- name: RenameFile :one
UPDATE archive.file SET name = $2 WHERE id = $1
RETURNING id, hash, name, mime, size, metadata, status, error, space, created_at;

-- name: LockFile :one
SELECT id, hash, name, mime, size, metadata, status, error, space, created_at FROM archive.file WHERE id = $1 FOR UPDATE;

-- name: FileText :one
SELECT text FROM archive.file WHERE id = $1;

-- name: FileWithText :one
SELECT id, hash, name, mime, size, metadata, status, error, space, created_at, text FROM archive.file WHERE id = $1;

-- name: FilesWithText :many
SELECT id, hash, name, mime, size, metadata, status, error, space, created_at, text FROM archive.file
WHERE id > sqlc.arg(after) AND (sqlc.narg(space)::uuid IS NULL OR space = sqlc.narg(space)) ORDER BY id LIMIT sqlc.arg(lim);

-- name: NextPendingFile :one
SELECT id, hash, name, mime, size, metadata, status, error, space, created_at FROM archive.file
WHERE status = 'pending' ORDER BY id LIMIT 1 FOR UPDATE SKIP LOCKED;

-- name: LockPendingFile :one
SELECT id, hash, name, mime, size, metadata, status, error, space, created_at FROM archive.file
WHERE id = $1 AND status = 'pending' FOR UPDATE;

-- name: SetFileContent :exec
UPDATE archive.file SET metadata = $2, text = $3 WHERE id = $1;

-- name: SetFileStatus :exec
UPDATE archive.file SET status = $2, error = $3 WHERE id = $1;

-- name: ResetProcessing :exec
UPDATE archive.file SET status = 'pending' WHERE status = 'processing';

-- name: ReplaceFile :one
UPDATE archive.file SET hash = $2, size = $3, status = 'pending', text = '', error = '', metadata = '{}' WHERE id = $1
RETURNING id, hash, name, mime, size, metadata, status, error, space, created_at;

-- name: Upload :one
SELECT * FROM archive.upload WHERE id = $1;

-- name: LockUpload :one
SELECT * FROM archive.upload WHERE id = $1 FOR UPDATE;

-- name: CreateUpload :exec
INSERT INTO archive.upload (id, role, space, name, length) VALUES ($1, $2, $3, $4, $5);

-- name: SetUploadParts :exec
UPDATE archive.upload SET parts = $2 WHERE id = $1;

-- name: DeleteUpload :exec
DELETE FROM archive.upload WHERE id = $1;

-- name: Revision :one
SELECT coalesce((SELECT id::text || ':' || hash FROM archive.revision ORDER BY created_at DESC, id DESC LIMIT 1), '')::text;

-- name: InsertRevision :exec
INSERT INTO archive.revision (hash) VALUES ($1);

-- name: SetLocal :exec
SELECT set_config(sqlc.arg(name), sqlc.arg(value), true);

-- name: AdvisoryLock :exec
SELECT pg_advisory_lock(sqlc.arg(key));

-- name: AdvisoryUnlock :exec
SELECT pg_advisory_unlock(sqlc.arg(key));

-- name: AdvisoryXactLock :exec
SELECT pg_advisory_xact_lock(sqlc.arg(key));

-- name: Catalog :one
WITH col AS (
    SELECT a.attrelid AS rel, a.attnum AS num, jsonb_build_object(
        'name', a.attname,
        'description', coalesce(col_description(a.attrelid, a.attnum), ''),
        'type', e.typname,
        'schema', e.typnamespace::regnamespace::text,
        'reg', format_type(e.oid, a.atttypmod),
        'list', t.typcategory = 'A' AND t.typelem <> 0,
        'not_null', a.attnotnull,
        'default', a.atthasdef,
        'generated', a.attgenerated <> '',
        'pk', EXISTS (SELECT FROM pg_catalog.pg_constraint k WHERE k.conrelid = a.attrelid AND k.contype = 'p' AND k.conkey = ARRAY[a.attnum]),
        'ref', coalesce((SELECT r.relnamespace::regnamespace::text || '.' || r.relname FROM pg_catalog.pg_constraint k JOIN pg_catalog.pg_class r ON r.oid = k.confrelid
            WHERE k.conrelid = a.attrelid AND k.contype = 'f' AND k.conkey = ARRAY[a.attnum] LIMIT 1), '')) AS c
    FROM pg_catalog.pg_attribute a
    JOIN pg_catalog.pg_type t ON t.oid = a.atttypid
    JOIN pg_catalog.pg_type e ON e.oid = CASE WHEN t.typcategory = 'A' AND t.typelem <> 0 THEN t.typelem ELSE t.oid END
    WHERE a.attnum > 0 AND NOT a.attisdropped
)
SELECT jsonb_build_object(
    'tables', coalesce((SELECT jsonb_object_agg(c.relname, jsonb_build_object(
        'name', c.relname,
        'kind', CASE c.relkind WHEN 'v' THEN 'view' ELSE 'table' END,
        'description', coalesce(obj_description(c.oid, 'pg_class'), ''),
        'rls', c.relrowsecurity,
        'force', c.relforcerowsecurity,
        'security_invoker', coalesce((SELECT o.option_value::bool FROM pg_options_to_table(c.reloptions) o WHERE o.option_name = 'security_invoker'), false),
        'policies', (SELECT count(*) FROM pg_catalog.pg_policy p WHERE p.polrelid = c.oid),
        'columns', (SELECT jsonb_agg(col.c ORDER BY col.num) FROM col WHERE col.rel = c.oid)))
        FROM pg_catalog.pg_class c WHERE c.relnamespace = 'public'::regnamespace AND c.relkind IN ('r', 'p', 'v')), '{}'),
    'types', coalesce((SELECT jsonb_object_agg(t.typnamespace::regnamespace::text || '.' || t.typname, jsonb_build_object(
        'name', t.typname,
        'schema', t.typnamespace::regnamespace::text,
        'kind', t.typtype,
        'description', coalesce(obj_description(t.oid, 'pg_type'), ''),
        'base', coalesce((SELECT b.typname FROM pg_catalog.pg_type b WHERE b.oid = t.typbasetype), ''),
        'base_schema', coalesce((SELECT b.typnamespace::regnamespace::text FROM pg_catalog.pg_type b WHERE b.oid = t.typbasetype), ''),
        'enum', coalesce((SELECT jsonb_agg(l.enumlabel ORDER BY l.enumsortorder) FROM pg_catalog.pg_enum l WHERE l.enumtypid = t.oid), '[]'),
        'attrs', coalesce((SELECT jsonb_agg(col.c ORDER BY col.num) FROM col WHERE col.rel = t.typrelid), '[]')))
        FROM pg_catalog.pg_type t LEFT JOIN pg_catalog.pg_class r ON r.oid = t.typrelid
        WHERE t.typnamespace IN ('archive'::regnamespace, 'public'::regnamespace)
          AND (t.typtype IN ('d', 'e') OR t.typtype = 'c' AND r.relkind = 'c')), '{}'));

-- name: Workflows :many
SELECT * FROM archive.workflow WHERE user_id = $1 ORDER BY name;

-- name: WorkflowByName :one
SELECT * FROM archive.workflow WHERE user_id = $1 AND name = $2;

-- name: WorkflowByHook :one
SELECT * FROM archive.workflow WHERE hook = $1 AND trigger = 'webhook';

-- name: DeleteWorkflowsExcept :exec
DELETE FROM archive.workflow WHERE user_id = $1 AND NOT (name = ANY (sqlc.arg(names)::text[]));

-- name: DeleteWorkflow :exec
DELETE FROM archive.workflow WHERE user_id = $1 AND name = $2;

-- name: SaveWorkflow :one
INSERT INTO archive.workflow (user_id, role, name, source, trigger, hook, next_at, cursor_at, cursor_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (user_id, name) DO UPDATE SET role = EXCLUDED.role, source = EXCLUDED.source, trigger = EXCLUDED.trigger, next_at = EXCLUDED.next_at,
    cursor_at = CASE WHEN archive.workflow.trigger = EXCLUDED.trigger THEN archive.workflow.cursor_at ELSE EXCLUDED.cursor_at END,
    cursor_id = CASE WHEN archive.workflow.trigger = EXCLUDED.trigger THEN archive.workflow.cursor_id ELSE EXCLUDED.cursor_id END,
    updated_at = now()
RETURNING *;

-- name: ClaimWorkflow :one
UPDATE archive.workflow SET next_at = sqlc.arg(lease)::timestamptz
WHERE id = (SELECT w.id FROM archive.workflow w WHERE w.next_at <= now() ORDER BY w.next_at LIMIT 1 FOR UPDATE SKIP LOCKED)
RETURNING *;

-- name: SetWorkflowNext :exec
UPDATE archive.workflow SET next_at = $3 WHERE id = $1 AND trigger = $2;

-- name: AdvanceWorkflow :exec
UPDATE archive.workflow SET next_at = sqlc.arg(next_at), cursor_at = sqlc.arg(cursor_at), cursor_id = sqlc.arg(cursor_id)
WHERE id = sqlc.arg(id) AND trigger = sqlc.arg(trigger);

-- name: AddWorkflowRun :one
INSERT INTO archive.workflow_run (workflow, trigger, result, logs, error, ms, created_at)
SELECT w.id, sqlc.arg(trigger), sqlc.arg(result), sqlc.arg(logs), sqlc.arg(error), sqlc.arg(ms), sqlc.arg(created_at)
FROM archive.workflow w WHERE w.id = sqlc.arg(workflow)
RETURNING *;

-- name: TrimWorkflowRuns :exec
DELETE FROM archive.workflow_run d WHERE d.workflow = $1
    AND d.id NOT IN (SELECT r.id FROM archive.workflow_run r WHERE r.workflow = $1 ORDER BY r.created_at DESC, r.id DESC LIMIT 100);

-- name: WorkflowRuns :many
SELECT * FROM archive.workflow_run WHERE workflow = $1 ORDER BY created_at DESC, id DESC LIMIT $2;

-- name: LastWorkflowRuns :many
SELECT DISTINCT ON (r.workflow) r.* FROM archive.workflow_run r JOIN archive.workflow w ON w.id = r.workflow
WHERE w.user_id = $1 ORDER BY r.workflow, r.created_at DESC, r.id DESC;

-- name: WorkflowRunTimes :many
SELECT created_at FROM archive.workflow_run WHERE workflow = $1 AND created_at >= sqlc.arg(since)::timestamptz;

-- name: TxNow :one
SELECT now()::timestamptz;

-- name: TxWrote :one
SELECT (pg_current_xact_id_if_assigned() IS NOT NULL)::bool;

-- name: WakeTableWorkflows :execrows
UPDATE archive.workflow SET next_at = now() WHERE cursor_id IS NOT NULL AND next_at IS NULL;

-- name: NextWorkflowAt :one
SELECT next_at FROM archive.workflow WHERE next_at IS NOT NULL ORDER BY next_at LIMIT 1;

-- name: HasPendingFiles :one
SELECT EXISTS (SELECT FROM archive.file WHERE status = 'pending');

-- name: FirstWorkspace :one
SELECT * FROM archive.workspace ORDER BY created_at, id LIMIT 1;

-- name: WorkspaceByID :one
SELECT * FROM archive.workspace WHERE id = $1;

-- name: AllWorkspaces :many
SELECT * FROM archive.workspace ORDER BY created_at, id;

-- name: CreateWorkspace :one
INSERT INTO archive.workspace (id, name, key) VALUES ($1, $2, $3)
ON CONFLICT DO NOTHING RETURNING *;

-- name: DeleteWorkspace :exec
WITH gone AS (UPDATE archive."user" u SET workspaces = array_remove(u.workspaces, sqlc.arg(id)::uuid) WHERE u.workspaces @> ARRAY[sqlc.arg(id)::uuid])
DELETE FROM archive.workspace w WHERE w.id = sqlc.arg(id)::uuid;

-- name: RolesWithPrefix :many
SELECT rolname::text FROM pg_catalog.pg_roles WHERE starts_with(rolname, sqlc.arg(prefix)::text);

-- name: RenameWorkspace :exec
UPDATE archive.workspace SET name = $2 WHERE id = $1;

-- name: SetWorkspaceAnthropicKey :exec
UPDATE archive.workspace SET anthropic_key = $2 WHERE id = $1;

-- name: SoonDue :one
WITH was AS (SELECT x.at FROM archive.due x WHERE x.workspace = sqlc.arg(workspace) AND x.kind = sqlc.arg(kind))
INSERT INTO archive.due (workspace, kind, at) VALUES (sqlc.arg(workspace), sqlc.arg(kind), sqlc.arg(at))
ON CONFLICT (workspace, kind) DO UPDATE SET at = least(archive.due.at, EXCLUDED.at)
RETURNING at, (at IS DISTINCT FROM (SELECT was.at FROM was))::bool AS changed;

-- name: ClaimDue :one
UPDATE archive.due d SET at = sqlc.arg(lease)::timestamptz FROM archive.workspace w
WHERE w.id = d.workspace AND (d.workspace, d.kind) = (SELECT x.workspace, x.kind FROM archive.due x WHERE x.at <= now() ORDER BY x.at LIMIT 1 FOR UPDATE SKIP LOCKED)
RETURNING w.id, d.kind, d.at;

-- name: ClaimDueFor :one
UPDATE archive.due d SET at = sqlc.arg(lease)::timestamptz
WHERE d.workspace = sqlc.arg(id) AND d.kind = sqlc.arg(kind) AND d.at <= now()
RETURNING d.at;

-- name: DueAt :one
SELECT at FROM archive.due WHERE workspace = $1 AND kind = $2;

-- name: EarliestDue :one
SELECT at FROM archive.due ORDER BY at LIMIT 1;

-- name: NotifyDue :exec
SELECT pg_notify('archive_due', sqlc.arg(at)::text);

-- name: SetDue :execrows
UPDATE archive.due d SET at = sqlc.arg(next)::timestamptz FROM archive.workspace w
WHERE w.id = d.workspace AND w.id = sqlc.arg(id) AND d.kind = sqlc.arg(kind) AND d.at = sqlc.arg(claimed)::timestamptz;

-- name: ClearDue :exec
DELETE FROM archive.due d USING archive.workspace w
WHERE w.id = d.workspace AND w.id = sqlc.arg(id) AND d.kind = sqlc.arg(kind) AND d.at = sqlc.arg(claimed)::timestamptz;

