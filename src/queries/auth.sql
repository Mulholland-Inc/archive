-- name: HasRole :one
SELECT pg_has_role(sqlc.arg(member)::name, rolname, 'USAGE') FROM pg_catalog.pg_roles WHERE rolname = sqlc.arg(grp)::name;

-- name: RoleExists :one
SELECT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = sqlc.arg(role)::name);

-- name: DirectRoles :many
SELECT r.rolname::text FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles r ON r.oid = m.roleid JOIN pg_catalog.pg_roles u ON u.oid = m.member
WHERE u.rolname = sqlc.arg(member)::name AND r.rolname = ANY(sqlc.arg(grps)::name[]);

-- name: CountOwners :one
SELECT count(*) FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles r ON r.oid = m.roleid
JOIN pg_catalog.pg_roles u ON u.oid = m.member JOIN archive."user" ON "user".role = u.rolname WHERE r.rolname = sqlc.arg(owner)::name;

-- name: CreateSpace :one
INSERT INTO archive.space (name, role) VALUES ($1, $2) RETURNING id;

-- name: SpaceTaken :one
SELECT EXISTS (SELECT FROM archive.space WHERE name = $1);

-- name: SpaceRole :one
SELECT role FROM archive.space WHERE name = $1;

-- name: SpaceRoleByID :one
SELECT role FROM archive.space WHERE id = $1;

-- name: SpaceByName :one
SELECT id, role FROM archive.space WHERE name = $1;

-- name: SpaceRoles :many
SELECT name, role FROM archive.space ORDER BY name;

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

-- name: CountUsers :one
SELECT count(*) FROM archive."user";

-- name: NoUsers :one
SELECT NOT EXISTS (SELECT FROM archive."user");

-- name: CreateUser :one
INSERT INTO archive."user" (id, email, name, role, home) VALUES ($1, $2, $3, $4, $5) RETURNING created_at;

-- name: UserByEmail :one
SELECT * FROM archive."user" WHERE email = $1;

-- name: UserByID :one
SELECT * FROM archive."user" WHERE id = $1;

-- name: FirstUser :one
SELECT * FROM archive."user" ORDER BY created_at, id LIMIT $1;

-- name: UserByOIDC :one
SELECT * FROM archive."user" WHERE signin->'oidc' @> sqlc.arg(link)::jsonb;

-- name: UserRole :one
SELECT id, role FROM archive."user" WHERE email = $1;

-- name: HomeOwner :one
SELECT email FROM archive."user" WHERE home = $1;

-- name: Members :many
SELECT email, name, role AS level FROM archive."user" ORDER BY email;

-- name: SetSignin :exec
UPDATE archive."user" SET signin = $2 WHERE id = $1;

-- name: Authenticate :one
SELECT t.id, t.kind, t.name, coalesce(t.role, '') AS role, u.id AS user_id, u.email, u.name AS user_name, u.role AS user_role, u.home,
    coalesce((SELECT r.id::text || ':' || r.hash FROM archive.revision r ORDER BY r.created_at DESC, r.id DESC LIMIT 1), '')::text AS stamp
FROM archive.token t JOIN archive."user" u ON u.id = t.user_id
WHERE t.hash = $1 AND t.kind IN ('api', 'session', 'user') AND (t.expires_at IS NULL OR t.expires_at > now());

-- name: CreateLoginToken :exec
INSERT INTO archive.token (user_id, kind, hash, expires_at) VALUES ($1, $2, $3, now() + sqlc.arg(ttl)::text::interval);

-- name: UseMagicLink :one
DELETE FROM archive.token WHERE hash = $1 AND kind = 'magic' AND expires_at > now() RETURNING user_id;

-- name: DeleteToken :exec
DELETE FROM archive.token WHERE id = $1;

-- name: TokenRoles :many
SELECT coalesce(role, '')::text FROM archive.token WHERE user_id = $1 AND role IS NOT NULL;

-- name: CreateUserToken :one
INSERT INTO archive.token (id, user_id, kind, name, hash, expires_at) VALUES ($1, $2, 'user', $3, $4, $5) RETURNING created_at, expires_at;

-- name: CreateAPIToken :one
INSERT INTO archive.token (id, user_id, kind, name, role, hash, expires_at) VALUES ($1, $2, 'api', $3, $4, $5, $6) RETURNING created_at, expires_at;

-- name: Tokens :many
SELECT id, name, coalesce(role, '')::text AS role, kind = 'user' AS full, expires_at, created_at FROM archive.token
WHERE user_id = $1 AND kind IN ('api', 'user') ORDER BY id;

-- name: RevokeToken :one
DELETE FROM archive.token WHERE id = $1 AND user_id = $2 AND kind IN ('api', 'user') RETURNING coalesce(role, '')::text;
