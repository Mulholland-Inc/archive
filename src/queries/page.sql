-- name: Notify :exec
SELECT pg_notify('archive_page', sqlc.arg(payload));

-- name: Listen :exec
LISTEN archive_page;
