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
