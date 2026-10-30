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
