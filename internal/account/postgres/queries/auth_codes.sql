-- Коды входа: auth_code с purpose = 'login' (04 §2).

-- name: InsertLoginCode :exec
INSERT INTO auth_code (id, purpose, email, code_hash, expires_at, attempts, requested_ip, created_at, updated_at)
VALUES (@id, 'login', @email, @code_hash, @expires_at, 0, @requested_ip, @created_at, @created_at);

-- name: LatestActiveLoginCodeForUpdate :one
SELECT * FROM auth_code
WHERE purpose = 'login' AND email = @email AND consumed_at IS NULL
ORDER BY created_at DESC, id DESC
LIMIT 1
FOR UPDATE;

-- name: ConsumeActiveLoginCodes :exec
UPDATE auth_code SET consumed_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE purpose = 'login' AND email = @email AND consumed_at IS NULL;

-- name: SaveLoginCode :exec
UPDATE auth_code SET attempts = @attempts, consumed_at = @consumed_at, updated_at = @updated_at WHERE id = @id;

-- name: CountLoginCodesByEmailSince :one
SELECT count(*)::int AS count, COALESCE(min(created_at), @since::timestamptz)::timestamptz AS oldest
FROM auth_code
WHERE purpose = 'login' AND email = @email AND created_at > @since::timestamptz;

-- name: CountLoginCodesByIPSince :one
SELECT count(*)::int AS count, COALESCE(min(created_at), @since::timestamptz)::timestamptz AS oldest
FROM auth_code
WHERE purpose = 'login' AND requested_ip = @ip::inet AND created_at > @since::timestamptz;
