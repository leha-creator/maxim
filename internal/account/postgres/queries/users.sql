-- Пользователи (04 §2).

-- name: GetUserByEmail :one
SELECT * FROM "user" WHERE email = @email;

-- name: GetUserByID :one
SELECT * FROM "user" WHERE id = @id;

-- name: InsertUser :exec
INSERT INTO "user" (id, email, admin_role, status, last_login_at, created_at, updated_at)
VALUES (@id, @email, @admin_role, @status, @last_login_at, @created_at, @updated_at);

-- name: SetUserLastLogin :exec
UPDATE "user" SET last_login_at = @at::timestamptz, updated_at = @at::timestamptz WHERE id = @id;

-- name: SetUserAdminRole :exec
UPDATE "user" SET admin_role = sqlc.narg(admin_role), updated_at = @at::timestamptz WHERE id = @id;

-- name: ListOrganizationMemberIDs :many
-- Адресаты уведомлений организации (модуль notification, R-1): владелец первым.
SELECT m.user_id FROM organization_member m JOIN "user" u ON u.id = m.user_id
WHERE m.organization_id = @organization_id AND u.status = 'active'
ORDER BY (m.role = 'owner') DESC, m.created_at, m.user_id;

-- name: ListAdminUserIDs :many
SELECT id FROM "user" WHERE admin_role IS NOT NULL AND status = 'active' ORDER BY created_at, id;
