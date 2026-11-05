CREATE SCHEMA IF NOT EXISTS archive;

CREATE TABLE archive.main_revision (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    hash       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TYPE archive.workspace_type AS ENUM ('personal', 'family', 'business');

CREATE TABLE archive.workspace (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    name       text NOT NULL,
    key        bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    type       archive.workspace_type NOT NULL DEFAULT 'personal',
    icon       text,
    color      smallint CONSTRAINT workspace_color CHECK (color BETWEEN 0 AND 31),
    sent       integer NOT NULL DEFAULT 0,
    sent_on    date,
    slug       text NOT NULL UNIQUE CONSTRAINT workspace_slug CHECK (slug ~ '^[a-z0-9]{1,24}$')
);

CREATE FUNCTION archive.workspace_base(name text) RETURNS text LANGUAGE sql IMMUTABLE SET search_path = pg_catalog AS $$
    SELECT coalesce(nullif(regexp_replace(lower(regexp_replace(normalize(name, NFD), '[\u0300-\u036f]', '', 'g')), '[^a-z0-9]', '', 'g'), ''), 'workspace');
$$;

CREATE FUNCTION archive.workspace_slug() RETURNS trigger LANGUAGE plpgsql SET search_path = pg_catalog AS $$
DECLARE
    base text := archive.workspace_base(NEW.name);
    n integer := 1;
BEGIN
    IF TG_OP = 'UPDATE' AND base = archive.workspace_base(OLD.name) THEN
        NEW.slug := OLD.slug;
        RETURN NEW;
    END IF;
    NEW.slug := left(base, 24);
    WHILE NOT pg_try_advisory_xact_lock(hashtext('workspace slug ' || NEW.slug))
        OR EXISTS (SELECT FROM archive.workspace w WHERE w.slug = NEW.slug AND w.id <> NEW.id) LOOP
        n := n + 1;
        NEW.slug := left(base, 24 - length(n::text)) || n;
    END LOOP;
    RETURN NEW;
END $$;

CREATE TRIGGER slug BEFORE INSERT OR UPDATE ON archive.workspace FOR EACH ROW EXECUTE FUNCTION archive.workspace_slug();

CREATE TABLE archive."user" (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    email      text NOT NULL UNIQUE,
    name       text NOT NULL DEFAULT '',
    workspaces uuid[] NOT NULL DEFAULT '{}',
    signin     jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now(),
    admin      boolean NOT NULL DEFAULT false,
    unread     uuid[] NOT NULL DEFAULT '{}',
    totp       bytea,
    totp_step  bigint NOT NULL DEFAULT 0,
    totp_fails integer NOT NULL DEFAULT 0,
    totp_until timestamptz
);
CREATE INDEX user_workspaces_idx ON archive."user" USING gin (workspaces);

CREATE TABLE archive.mcp_client (
    id            uuid PRIMARY KEY DEFAULT uuidv7(),
    name          text NOT NULL,
    redirect_uris text[] NOT NULL,
    secret        text,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE archive.token (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id    uuid NOT NULL REFERENCES archive."user" ON DELETE CASCADE,
    workspace  uuid REFERENCES archive.workspace ON DELETE CASCADE,
    kind       text NOT NULL,
    name       text NOT NULL DEFAULT '',
    role       text UNIQUE,
    hash       text NOT NULL UNIQUE,
    sealed     bytea,
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    client     uuid REFERENCES archive.mcp_client ON DELETE CASCADE,
    refresh    text UNIQUE,
    refresh_expires_at timestamptz
);
CREATE INDEX token_user_idx ON archive.token (user_id);

CREATE TABLE archive.mcp_code (
    hash         text PRIMARY KEY,
    client       uuid NOT NULL REFERENCES archive.mcp_client ON DELETE CASCADE,
    user_id      uuid NOT NULL REFERENCES archive."user" ON DELETE CASCADE,
    workspace    uuid NOT NULL REFERENCES archive.workspace ON DELETE CASCADE,
    redirect_uri text NOT NULL,
    challenge    text NOT NULL,
    token        uuid,
    expires_at   timestamptz NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE archive.due (
    workspace uuid NOT NULL REFERENCES archive.workspace ON DELETE CASCADE,
    kind      text NOT NULL,
    at        timestamptz NOT NULL,
    PRIMARY KEY (workspace, kind)
);

CREATE TABLE archive.oauth_client (
    provider   text PRIMARY KEY,
    client     bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

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

-- name: FirstAdmin :one
SELECT * FROM archive."user" WHERE admin ORDER BY created_at, id LIMIT 1;

-- name: AdminIfNone :exec
UPDATE archive."user" u SET admin = true WHERE u.id = $1 AND NOT EXISTS (SELECT FROM archive."user" x WHERE x.admin);

-- name: NoUsers :one
SELECT (NOT EXISTS (SELECT FROM archive."user") AND NOT EXISTS (SELECT FROM archive.workspace))::bool;

-- name: WorkspaceUsers :many
SELECT id, email, name FROM archive."user" WHERE workspaces @> ARRAY[sqlc.arg(workspace)::uuid] ORDER BY email;

-- name: UserWorkspaces :many
SELECT w.* FROM archive.workspace w JOIN archive."user" u ON w.id = ANY(u.workspaces) WHERE u.id = $1 ORDER BY array_position(u.workspaces, w.id);

-- name: MarkUnread :exec
UPDATE archive."user" SET unread = array_append(array_remove(unread, sqlc.arg(workspace)::uuid), sqlc.arg(workspace)::uuid) WHERE id = sqlc.arg(id);

-- name: ClearUnread :exec
UPDATE archive."user" SET unread = array_remove(unread, sqlc.arg(workspace)::uuid) WHERE id = sqlc.arg(id);

-- name: JoinWorkspace :one
INSERT INTO archive."user" (email, name, workspaces) VALUES (sqlc.arg(email), sqlc.arg(name), ARRAY[sqlc.arg(workspace)::uuid])
ON CONFLICT (email) DO UPDATE SET workspaces = archive."user".workspaces || EXCLUDED.workspaces WHERE NOT archive."user".workspaces @> EXCLUDED.workspaces
RETURNING *;

-- name: LeaveWorkspace :exec
UPDATE archive."user" SET workspaces = array_remove(workspaces, sqlc.arg(workspace)::uuid), unread = array_remove(unread, sqlc.arg(workspace)::uuid) WHERE id = sqlc.arg(id);

-- name: SetSignin :exec
UPDATE archive."user" SET signin = $2 WHERE id = $1;

-- name: NameUser :exec
UPDATE archive."user" SET name = $2 WHERE id = $1 AND name = '';

-- name: Authenticate :one
SELECT t.id, t.kind, t.name, coalesce(t.role, '')::text AS role, u.id AS user_id, u.email, u.name AS user_name,
    coalesce(t.workspace::text, '')::text AS workspace,
    ARRAY(SELECT w.id::text FROM archive.workspace w WHERE w.id = ANY(u.workspaces) ORDER BY array_position(u.workspaces, w.id))::text[] AS workspaces
FROM archive.token t JOIN archive."user" u ON u.id = t.user_id
WHERE t.hash = $1 AND t.kind = ANY(sqlc.arg(kinds)::text[]) AND (t.expires_at IS NULL OR t.expires_at > now());

-- name: CreateSession :exec
INSERT INTO archive.token (user_id, kind, hash, expires_at) VALUES ($1, 'session', $2, now() + interval '30 days');

-- name: SignOut :exec
DELETE FROM archive.token WHERE hash = $1 AND kind IN ('session', 'totp');

-- name: CreateTotpToken :exec
WITH old AS (DELETE FROM archive.token x WHERE x.kind = 'totp' AND x.expires_at < now())
INSERT INTO archive.token (user_id, kind, hash, expires_at) VALUES ($1, 'totp', $2, now() + interval '5 minutes');

-- name: TotpSession :execrows
WITH gone AS (DELETE FROM archive.token x WHERE x.hash = sqlc.arg(pending) AND x.kind = 'totp' AND x.expires_at > now() RETURNING x.user_id)
INSERT INTO archive.token (user_id, kind, hash, expires_at) SELECT gone.user_id, 'session', sqlc.arg(hash), now() + interval '30 days' FROM gone;

-- name: TotpTry :one
UPDATE archive."user" SET
    totp_fails = CASE WHEN totp_until > now() THEN totp_fails + 1 ELSE 1 END,
    totp_until = CASE WHEN totp_until > now() THEN totp_until ELSE now() + interval '15 minutes' END
WHERE id = $1 AND totp IS NOT NULL AND (totp_until IS NULL OR totp_until <= now() OR totp_fails < 5)
RETURNING totp;

-- name: TotpUse :execrows
UPDATE archive."user" SET totp_step = sqlc.arg(step), totp_fails = 0, totp_until = NULL WHERE id = sqlc.arg(id) AND totp_step < sqlc.arg(step);

-- name: EnableTotp :one
WITH linked AS (
    UPDATE archive."user" u SET totp = sqlc.arg(totp), totp_step = sqlc.arg(step), totp_fails = 0, totp_until = NULL
    WHERE u.id = sqlc.arg(id) AND u.totp IS NULL RETURNING u.id
), gone AS (
    DELETE FROM archive.token x USING linked WHERE x.user_id = linked.id AND x.kind = 'session' AND x.hash <> sqlc.arg(keep)
)
SELECT count(*) FROM linked;

-- name: DisableTotp :execrows
UPDATE archive."user" SET totp = NULL, totp_step = 0, totp_fails = 0, totp_until = NULL WHERE id = sqlc.arg(id) AND totp_step < sqlc.arg(step);

-- name: CreateToken :one
INSERT INTO archive.token (id, user_id, workspace, kind, name, role, hash, sealed, expires_at) VALUES ($1, $2, $3, 'api', $4, $5, $6, $7, $8) RETURNING created_at, expires_at;

-- name: KeyWorkspace :one
SELECT workspace FROM archive.token WHERE id = $1 AND user_id = $2 AND kind = 'api';

-- name: TokenSealed :one
SELECT sealed FROM archive.token WHERE id = $1 AND user_id = $2 AND kind = 'api' AND sealed IS NOT NULL AND (NOT sqlc.arg(scoped)::bool OR workspace = sqlc.arg(workspace));

-- name: Tokens :many
SELECT t.id, t.name, (t.role IS NOT NULL)::bool AS viewer, t.expires_at, t.refresh_expires_at, t.created_at, (t.kind = 'mcp')::bool AS app,
    coalesce(w.id::text, '')::text AS workspace, coalesce(w.name, '')::text AS workspace_name
FROM archive.token t LEFT JOIN archive.workspace w ON w.id = t.workspace
WHERE t.user_id = $1 AND t.kind IN ('api', 'mcp') AND (NOT sqlc.arg(scoped)::bool OR t.workspace = sqlc.arg(workspace)) ORDER BY t.id;

-- name: RevokeToken :one
DELETE FROM archive.token WHERE id = $1 AND user_id = $2 AND kind IN ('api', 'mcp') AND (workspace = sqlc.arg(workspace) OR (NOT sqlc.arg(scoped)::bool AND role IS NULL)) RETURNING coalesce(role, '')::text;

-- name: CreateMCPClient :one
INSERT INTO archive.mcp_client (name, redirect_uris, secret) VALUES ($1, $2, $3) RETURNING id, created_at;

-- name: MCPClient :one
SELECT * FROM archive.mcp_client WHERE id = $1;

-- name: CreateMCPCode :exec
WITH old AS (DELETE FROM archive.mcp_code c WHERE c.expires_at < now() - interval '1 hour')
INSERT INTO archive.mcp_code (hash, client, user_id, workspace, redirect_uri, challenge, expires_at)
VALUES (sqlc.arg(hash), sqlc.arg(client), sqlc.arg(user_id), sqlc.arg(workspace), sqlc.arg(redirect_uri), sqlc.arg(challenge), now() + interval '1 minute');

-- name: UseMCPCode :one
UPDATE archive.mcp_code SET token = sqlc.arg(token) WHERE hash = sqlc.arg(hash) AND token IS NULL AND expires_at > now() RETURNING *;

-- name: UsedMCPCode :one
SELECT token FROM archive.mcp_code WHERE hash = $1 AND token IS NOT NULL;

-- name: CreateMCPToken :exec
WITH old AS (DELETE FROM archive.token x WHERE x.kind = 'mcp' AND x.refresh_expires_at < now())
INSERT INTO archive.token (id, user_id, workspace, kind, name, hash, expires_at, client, refresh, refresh_expires_at)
VALUES (sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(workspace), 'mcp', sqlc.arg(name), sqlc.arg(hash), sqlc.arg(expires_at),
    sqlc.arg(client), sqlc.arg(refresh), sqlc.arg(refresh_expires_at));

-- name: RefreshMCPToken :one
UPDATE archive.token SET hash = sqlc.arg(hash), expires_at = sqlc.arg(expires_at), refresh = sqlc.arg(next), refresh_expires_at = sqlc.arg(refresh_expires_at)
WHERE refresh = sqlc.arg(refresh) AND client = sqlc.arg(client) AND kind = 'mcp' AND refresh_expires_at > now()
RETURNING id, workspace;

-- name: RevokeMCPToken :one
DELETE FROM archive.token WHERE kind = 'mcp' AND client = $1 AND (hash = $2 OR refresh = $2) RETURNING id, workspace;

-- name: DeleteMCPToken :one
DELETE FROM archive.token WHERE id = $1 AND kind = 'mcp' RETURNING workspace;

-- name: DeleteWorkspaceTokens :many
DELETE FROM archive.token WHERE user_id = $1 AND workspace = $2 RETURNING coalesce(role, '')::text;

-- name: WorkspaceByID :one
SELECT * FROM archive.workspace WHERE id = $1;

-- name: WorkspaceBySlug :one
SELECT * FROM archive.workspace WHERE slug = $1;

-- name: CountEmail :one
UPDATE archive.workspace SET sent = CASE WHEN sent_on = (now() AT TIME ZONE 'UTC')::date THEN sent + 1 ELSE 1 END, sent_on = (now() AT TIME ZONE 'UTC')::date
WHERE id = sqlc.arg(id) AND (sent_on IS DISTINCT FROM (now() AT TIME ZONE 'UTC')::date OR sent < sqlc.arg(cap)::integer) RETURNING sent;

-- name: UncountEmail :exec
UPDATE archive.workspace SET sent = greatest(sent - 1, 0) WHERE id = $1;

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

-- name: KeepWatchSecret :execrows
INSERT INTO archive.watch (provider, key, workspace, connection, secret) VALUES ($1, $2, $3, $4, $5) ON CONFLICT DO NOTHING;

-- name: DropWatches :exec
DELETE FROM archive.watch WHERE connection = sqlc.arg(connection) AND NOT key = ANY(sqlc.arg(keep)::text[]);
