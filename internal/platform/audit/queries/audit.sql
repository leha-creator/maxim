-- Журнал действий (04 §8). Таблица неизменяема: только вставка и чтение.

-- name: InsertAuditLog :exec
INSERT INTO audit_log (id, actor_type, actor_user_id, organization_id, action, object_type, object_id, reason, data, ip, created_at)
VALUES (@id, @actor_type, @actor_user_id, @organization_id, @action, @object_type, @object_id, @reason, @data, @ip, @created_at);

-- name: ListAuditLogByObject :many
SELECT * FROM audit_log WHERE object_type = @object_type AND object_id = @object_id ORDER BY created_at, id;

-- name: CountAuditByActorSince :one
SELECT count(*)::int FROM audit_log
WHERE action = @action AND actor_user_id = @actor_user_id AND created_at > @since;
