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
