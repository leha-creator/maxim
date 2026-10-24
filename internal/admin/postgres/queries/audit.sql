-- Журнал действий для экрана adm/audit (R-15): только чтение, ip не выбирается.

-- name: ListAuditLog :many
SELECT id, actor_type, actor_user_id, organization_id, action, object_type, object_id, reason, data, created_at
FROM audit_log
WHERE (sqlc.narg('actor_user_id')::uuid IS NULL OR actor_user_id = sqlc.narg('actor_user_id'))
  AND (sqlc.narg('actor_type')::text IS NULL OR actor_type = sqlc.narg('actor_type'))
  AND (sqlc.narg('action')::text IS NULL OR action = sqlc.narg('action'))
  AND (sqlc.narg('object_type')::text IS NULL OR object_type = sqlc.narg('object_type'))
  AND (sqlc.narg('object_id')::text IS NULL OR object_id = sqlc.narg('object_id'))
  AND (sqlc.narg('organization_id')::uuid IS NULL OR organization_id = sqlc.narg('organization_id'))
  AND (sqlc.narg('created_from')::timestamptz IS NULL OR created_at >= sqlc.narg('created_from'))
  AND (sqlc.narg('created_to')::timestamptz IS NULL OR created_at < sqlc.narg('created_to'))
ORDER BY created_at DESC, id DESC
LIMIT @lim OFFSET @off;

-- name: CountAuditLog :one
SELECT count(*)::int FROM audit_log
WHERE (sqlc.narg('actor_user_id')::uuid IS NULL OR actor_user_id = sqlc.narg('actor_user_id'))
  AND (sqlc.narg('actor_type')::text IS NULL OR actor_type = sqlc.narg('actor_type'))
  AND (sqlc.narg('action')::text IS NULL OR action = sqlc.narg('action'))
  AND (sqlc.narg('object_type')::text IS NULL OR object_type = sqlc.narg('object_type'))
  AND (sqlc.narg('object_id')::text IS NULL OR object_id = sqlc.narg('object_id'))
  AND (sqlc.narg('organization_id')::uuid IS NULL OR organization_id = sqlc.narg('organization_id'))
  AND (sqlc.narg('created_from')::timestamptz IS NULL OR created_at >= sqlc.narg('created_from'))
  AND (sqlc.narg('created_to')::timestamptz IS NULL OR created_at < sqlc.narg('created_to'));
