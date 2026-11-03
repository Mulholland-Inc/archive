-- The main database is the one DATABASE_URL names. It holds what every workspace shares and nothing of any one
-- of them: the workspaces and their data keys, the people who sign in and their tokens, when each workspace's
-- work is due, and what connectors' providers push to. Every workspace is a database of its own, archive_<id>,
-- made from archive.sql. Like archive.sql, this is applied to the main database as it is, by diff, at startup
-- (Hub.migrate), and after it come the sqlc queries of the Hub.
CREATE SCHEMA IF NOT EXISTS archive;

-- A migration of the main database, by the hash of this file; the newest says what it is now.
CREATE TABLE archive.main_revision (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    hash       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- What a workspace is for, which picks the icons the web app offers it.
CREATE TYPE archive.workspace_type AS ENUM ('personal', 'family', 'business');

-- icon and color are how the web app shows the workspace: one of its icons, and one of its 8 tints by index;
-- null is the default for its type.
CREATE TABLE archive.workspace (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    name       text NOT NULL,
    key        bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    type       archive.workspace_type NOT NULL DEFAULT 'personal',
    icon       text CONSTRAINT workspace_icon CHECK (icon IN ('robot', 'chest', 'safe', 'bookcase', 'pinecone', 'cottage', 'castle',
        'townhouse', 'farmhouse', 'oak', 'pine', 'baobab', 'skyscraper', 'capitol', 'pyramid', 'ledger', 'cabinet', 'bank', 'warehouse', 'factory')),
    color      smallint CONSTRAINT workspace_color CHECK (color BETWEEN 0 AND 7)
);

-- A person who signs in, and the workspaces they are a member of. admin says they run this Archive: they make
-- and delete workspaces and name other admins, whichever workspaces they are in. The first person added to a
-- new install is one.
CREATE TABLE archive."user" (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    email      text NOT NULL UNIQUE,
    name       text NOT NULL DEFAULT '',
    workspaces uuid[] NOT NULL DEFAULT '{}',
    signin     jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now(),
    admin      boolean NOT NULL DEFAULT false
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
    -- An API token's secret, sealed with the server's key, so its owner can copy it again; null for others.
    sealed     bytea,
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX token_user_idx ON archive.token (user_id);

CREATE TABLE archive.due (
    workspace uuid NOT NULL REFERENCES archive.workspace ON DELETE CASCADE,
    kind      text NOT NULL,
    at        timestamptz NOT NULL,
    PRIMARY KEY (workspace, kind)
);

-- An OAuth client Archive registered for itself at an MCP connector's authorization server (connect/connect.go):
-- one per connector, for every workspace, since the redirect URI is the server's. client is the
-- connect.Registration, its secret with it, sealed with ARCHIVE_SECRET_KEY. Only Go reads it.
CREATE TABLE archive.oauth_client (
    provider   text PRIMARY KEY,
    client     bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- A watch is something a connector's provider pushes for a connection's account (connect/sync.go): a Gmail
-- users.watch, a Drive channel, a Graph subscription, an Asana webhook, or the key a push of the app's own
-- webhook names it by, like a QuickBooks realmId. key is what a push names it by, resource what the provider
-- calls it, secret what its pushes prove themselves with, sealed with ARCHIVE_SECRET_KEY, and expires_at when
-- it lapses unless a sync renews it. Kept here for every workspace, since a push says only the key; only Go
-- reads it.
CREATE TABLE archive.watch (
    provider   text NOT NULL,
    key        text NOT NULL,
    workspace  uuid NOT NULL,
    connection uuid NOT NULL,
    resource   text NOT NULL DEFAULT '',
    secret     bytea,
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (provider, key, connection)
);
CREATE INDEX watch_connection_idx ON archive.watch (connection);

-- name: MainRevision :one
SELECT coalesce((SELECT hash FROM archive.main_revision ORDER BY created_at DESC, id DESC LIMIT 1), '')::text;

-- name: InsertMainRevision :exec
INSERT INTO archive.main_revision (hash) VALUES ($1);

-- name: UserByEmail :one
SELECT * FROM archive."user" WHERE email = $1;

-- name: UserByID :one
SELECT * FROM archive."user" WHERE id = $1;

-- name: UserByOIDC :one
SELECT * FROM archive."user" WHERE signin->'oidc' @> sqlc.arg(link)::jsonb;

-- name: FirstUser :one
SELECT * FROM archive."user" WHERE workspaces @> ARRAY[sqlc.arg(workspace)::uuid] ORDER BY created_at, id LIMIT 1;

-- name: FirstAdmin :one
SELECT * FROM archive."user" WHERE admin ORDER BY created_at, id LIMIT 1;

-- name: Admins :many
SELECT * FROM archive."user" WHERE admin ORDER BY email;

-- The first person a new install adds becomes its admin: they are the first member of a workspace while there
-- is none.
-- name: AdminIfNone :exec
UPDATE archive."user" u SET admin = true WHERE u.id = $1 AND NOT EXISTS (SELECT FROM archive."user" x WHERE x.admin);

-- An admin is named by their email, and stops being one unless they are the last.
-- name: SetAdmin :one
UPDATE archive."user" u SET admin = sqlc.arg(admin)::bool
WHERE u.email = sqlc.arg(email)::text AND (sqlc.arg(admin)::bool OR EXISTS (SELECT FROM archive."user" x WHERE x.admin AND x.id <> u.id))
RETURNING *;

-- A new install: no one and no workspace yet.
-- name: NoUsers :one
SELECT (NOT EXISTS (SELECT FROM archive."user") AND NOT EXISTS (SELECT FROM archive.workspace))::bool;

-- name: WorkspaceUsers :many
SELECT id, email, name FROM archive."user" WHERE workspaces @> ARRAY[sqlc.arg(workspace)::uuid] ORDER BY email;

-- name: UserWorkspaces :many
SELECT w.* FROM archive.workspace w JOIN archive."user" u ON w.id = ANY(u.workspaces) WHERE u.id = $1 ORDER BY array_position(u.workspaces, w.id);

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
WHERE t.hash = $1 AND t.kind IN ('api', 'session') AND (t.expires_at IS NULL OR t.expires_at > now());

-- name: CreateSession :exec
INSERT INTO archive.token (user_id, kind, hash, expires_at) VALUES ($1, 'session', $2, now() + interval '30 days');

-- name: SignOut :exec
DELETE FROM archive.token WHERE hash = $1 AND kind = 'session';

-- name: CreateToken :one
INSERT INTO archive.token (id, user_id, workspace, kind, name, role, hash, sealed, expires_at) VALUES ($1, $2, $3, 'api', $4, $5, $6, $7, $8) RETURNING created_at, expires_at;

-- name: TokenSealed :one
SELECT sealed FROM archive.token WHERE id = $1 AND user_id = $2 AND workspace = $3 AND kind = 'api' AND sealed IS NOT NULL;

-- name: Tokens :many
SELECT id, name, (role IS NOT NULL)::bool AS viewer, expires_at, created_at FROM archive.token
WHERE user_id = $1 AND workspace = $2 AND kind = 'api' ORDER BY id;

-- name: RevokeToken :one
DELETE FROM archive.token WHERE id = $1 AND user_id = $2 AND workspace = $3 AND kind = 'api' RETURNING coalesce(role, '')::text;

-- name: DeleteWorkspaceTokens :many
DELETE FROM archive.token WHERE user_id = $1 AND workspace = $2 RETURNING coalesce(role, '')::text;

-- name: FirstWorkspace :one
SELECT * FROM archive.workspace ORDER BY created_at, id LIMIT 1;

-- name: WorkspaceByID :one
SELECT * FROM archive.workspace WHERE id = $1;

-- name: AllWorkspaces :many
SELECT * FROM archive.workspace ORDER BY created_at, id;

-- name: CreateWorkspace :one
INSERT INTO archive.workspace (id, name, key, type, icon, color) VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT DO NOTHING RETURNING *;

-- name: DeleteWorkspace :exec
WITH gone AS (UPDATE archive."user" u SET workspaces = array_remove(u.workspaces, sqlc.arg(id)::uuid) WHERE u.workspaces @> ARRAY[sqlc.arg(id)::uuid])
DELETE FROM archive.workspace w WHERE w.id = sqlc.arg(id)::uuid;

-- name: RolesWithPrefix :many
SELECT rolname::text FROM pg_catalog.pg_roles WHERE starts_with(rolname, sqlc.arg(prefix)::text);

-- name: UpdateWorkspace :exec
UPDATE archive.workspace SET name = coalesce(sqlc.narg(name), name), type = coalesce(sqlc.narg(type), type),
    icon = coalesce(sqlc.narg(icon), icon), color = coalesce(sqlc.narg(color), color) WHERE id = sqlc.arg(id);

-- name: SoonDue :one
WITH was AS (SELECT x.at FROM archive.due x WHERE x.workspace = sqlc.arg(workspace) AND x.kind = sqlc.arg(kind))
INSERT INTO archive.due (workspace, kind, at) VALUES (sqlc.arg(workspace), sqlc.arg(kind), sqlc.arg(at))
ON CONFLICT (workspace, kind) DO UPDATE SET at = least(archive.due.at, EXCLUDED.at)
RETURNING at, (at IS DISTINCT FROM (SELECT was.at FROM was))::bool AS changed;

-- name: ClaimDueFor :one
UPDATE archive.due d SET at = sqlc.arg(lease)::timestamptz
WHERE d.workspace = sqlc.arg(id) AND d.kind = sqlc.arg(kind) AND d.at <= now()
RETURNING d.at;

-- name: DueAt :one
SELECT at FROM archive.due WHERE workspace = $1 AND kind = $2;

-- name: AllDue :many
SELECT workspace, kind, at FROM archive.due;

-- name: SetDue :execrows
UPDATE archive.due d SET at = sqlc.arg(next)::timestamptz FROM archive.workspace w
WHERE w.id = d.workspace AND w.id = sqlc.arg(id) AND d.kind = sqlc.arg(kind) AND d.at = sqlc.arg(claimed)::timestamptz;

-- name: ClearDue :exec
DELETE FROM archive.due d USING archive.workspace w
WHERE w.id = d.workspace AND w.id = sqlc.arg(id) AND d.kind = sqlc.arg(kind) AND d.at = sqlc.arg(claimed)::timestamptz;

-- name: OAuthClient :one
SELECT client FROM archive.oauth_client WHERE provider = $1;

-- name: SetOAuthClient :exec
INSERT INTO archive.oauth_client (provider, client) VALUES (sqlc.arg(provider), sqlc.arg(client))
ON CONFLICT (provider) DO UPDATE SET client = EXCLUDED.client, created_at = now() WHERE archive.oauth_client.client = sqlc.narg(old);

-- name: DeleteOAuthClient :exec
DELETE FROM archive.oauth_client WHERE provider = $1 AND client = $2;

-- name: WatchesFor :many
SELECT workspace, connection FROM archive.watch WHERE provider = $1 AND key = $2;

-- name: WatchSecret :one
SELECT secret FROM archive.watch WHERE provider = $1 AND key = $2 AND secret IS NOT NULL LIMIT 1;

-- name: ConnectionWatches :many
SELECT * FROM archive.watch WHERE connection = $1 ORDER BY key;

-- name: SaveWatch :exec
INSERT INTO archive.watch (provider, key, workspace, connection, resource, secret, expires_at)
VALUES (sqlc.arg(provider), sqlc.arg(key), sqlc.arg(workspace), sqlc.arg(connection), sqlc.arg(resource), sqlc.narg(secret), sqlc.narg(expires_at))
ON CONFLICT (provider, key, connection) DO UPDATE SET resource = EXCLUDED.resource, secret = coalesce(EXCLUDED.secret, archive.watch.secret),
    expires_at = EXCLUDED.expires_at;

-- A handshake's secret, kept by the key in the URL only it was sent: the first one wins.
-- name: KeepWatchSecret :execrows
INSERT INTO archive.watch (provider, key, workspace, connection, secret) VALUES ($1, $2, $3, $4, $5) ON CONFLICT DO NOTHING;

-- name: DropWatches :exec
DELETE FROM archive.watch WHERE connection = sqlc.arg(connection) AND NOT key = ANY(sqlc.arg(keep)::text[]);
