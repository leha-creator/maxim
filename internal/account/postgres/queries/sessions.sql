-- Сессии (04 §2). Хранится только хеш токена.

-- name: InsertSession :exec
INSERT INTO session (id, user_id, token_hash, expires_at, ip, user_agent, revoked_at, created_at, updated_at)
VALUES (@id, @user_id, @token_hash, @expires_at, @ip, @user_agent, NULL, @created_at, @created_at);

-- name: GetSessionByTokenHash :one
SELECT * FROM session WHERE token_hash = @token_hash;

-- name: SetSessionExpiresAt :exec
UPDATE session SET expires_at = @expires_at, updated_at = @now::timestamptz WHERE id = @id AND revoked_at IS NULL;

-- name: RevokeSession :exec
UPDATE session SET revoked_at = @now::timestamptz, updated_at = @now::timestamptz WHERE id = @id AND revoked_at IS NULL;
