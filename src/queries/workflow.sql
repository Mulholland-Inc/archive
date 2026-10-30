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
