-- Запросы второго подтверждения (R-2). Удаления нет (миграция 00014).

-- name: InsertApproval :exec
INSERT INTO admin_approval (id, kind, payload, amount, summary, reason, idempotency_key, requested_by, status,
                            expires_at, created_at, updated_at)
VALUES (@id, @kind, @payload, @amount, @summary, @reason, @idempotency_key, @requested_by, @status,
        @expires_at, @created_at, @updated_at);

-- name: GetApprovalByKey :one
SELECT * FROM admin_approval WHERE idempotency_key = @idempotency_key;

-- name: GetApprovalForUpdate :one
SELECT * FROM admin_approval WHERE id = @id FOR UPDATE;

-- name: UpdateApproval :exec
UPDATE admin_approval
SET status = @status, decided_by = @decided_by, decided_at = @decided_at, decision_reason = @decision_reason,
    result_id = @result_id, updated_at = @updated_at
WHERE id = @id;

-- name: ExpireApprovals :many
UPDATE admin_approval SET status = 'expired', updated_at = @now
WHERE status = 'pending' AND expires_at <= @now
RETURNING *;

-- name: ListApprovals :many
-- pending — старые первыми (очередь); остальные — новые первыми.
SELECT * FROM admin_approval
WHERE (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
ORDER BY CASE WHEN status = 'pending' THEN created_at END ASC,
         CASE WHEN status <> 'pending' THEN created_at END DESC, id
LIMIT @lim OFFSET @off;

-- name: CountApprovals :one
SELECT count(*)::int FROM admin_approval
WHERE (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'));
