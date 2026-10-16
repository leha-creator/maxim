-- Коды подключения канала: строки auth_code с purpose = 'channel_link' (04 §2, R-15).
-- Модуль account их не читает и не пишет.

-- name: InsertLinkCode :exec
INSERT INTO auth_code (id, purpose, organization_id, code_hash, expires_at, attempts, created_at, updated_at)
VALUES (@id, 'channel_link', @organization_id, @code_hash, @expires_at, 0, @created_at, @created_at);

-- name: ConsumeLiveLinkCodes :exec
UPDATE auth_code SET consumed_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE purpose = 'channel_link' AND organization_id = @organization_id AND consumed_at IS NULL;

-- name: FindLiveLinkCodeForUpdate :one
SELECT id, organization_id, code_hash, expires_at, consumed_at, created_at, updated_at FROM auth_code
WHERE purpose = 'channel_link' AND code_hash = @code_hash AND consumed_at IS NULL
  AND expires_at > @now::timestamptz
ORDER BY created_at DESC, id DESC
LIMIT 1
FOR UPDATE;

-- name: ConsumeLinkCode :exec
UPDATE auth_code SET consumed_at = @consumed_at, updated_at = @updated_at
WHERE id = @id AND purpose = 'channel_link';

-- name: CountLinkCodesSince :one
SELECT count(*)::int FROM auth_code
WHERE purpose = 'channel_link' AND organization_id = @organization_id AND created_at > @since::timestamptz;
