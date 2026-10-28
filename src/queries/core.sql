-- name: SpaceID :one
SELECT id FROM archive.space WHERE name = $1;

-- name: DefaultSpace :one
SELECT id FROM archive.space WHERE id = ANY(archive.my_spaces('editor')) ORDER BY id = sqlc.arg(home)::uuid DESC, id LIMIT 1;

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
