-- Выборки администратора (R-1). Вызываются только после проверки роли (07 §4).
-- Название и ИНН владельца читаются из organization (модуль account) только для показа.

-- name: ListChannelsForAdmin :many
SELECT sqlc.embed(c), o.name AS organization_name, o.inn AS organization_inn
FROM channel c LEFT JOIN organization o ON o.id = c.organization_id
WHERE c.status <> 'unclaimed'
  AND (sqlc.narg(status)::text IS NULL OR c.status = sqlc.narg(status)::text)
  AND (sqlc.narg(rkn_status)::text IS NULL OR c.rkn_status = sqlc.narg(rkn_status)::text)
  AND (sqlc.narg(recheck_due)::boolean IS NULL
       OR coalesce(c.rkn_status = 'confirmed' AND c.rkn_checked_at < @recheck_before::timestamptz, false) = sqlc.narg(recheck_due)::boolean)
  AND (sqlc.narg(after_updated_at)::timestamptz IS NULL
       OR (c.updated_at, c.id) > (sqlc.narg(after_updated_at)::timestamptz, @after_id::uuid))
ORDER BY c.updated_at, c.id
LIMIT @lim;

-- name: GetChannelForAdmin :one
SELECT sqlc.embed(c), o.name AS organization_name, o.inn AS organization_inn
FROM channel c LEFT JOIN organization o ON o.id = c.organization_id
WHERE c.id = @id;

-- name: GetChannelForAdminForUpdate :one
SELECT * FROM channel WHERE id = @id FOR UPDATE;

-- name: ListModerationHistory :many
-- Решения модерации и проверки РКН из журнала действий (S-16).
SELECT action, actor_type, actor_user_id, reason, coalesce(data ->> 'result', '')::text AS result, created_at
FROM audit_log
WHERE object_type = 'channel' AND object_id = @channel_id::text
  AND action IN ('channel.approved', 'channel.rejected', 'channel.rkn_checked')
ORDER BY created_at, id;
