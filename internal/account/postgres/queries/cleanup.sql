-- Очистка устаревших кодов и сессий (07 §2, задание cleanup; R-10).

-- name: DeleteExpiredAuthCodes :execrows
DELETE FROM auth_code
WHERE id IN (SELECT a.id FROM auth_code a WHERE a.expires_at < @before::timestamptz LIMIT @batch);

-- name: DeleteExpiredSessions :execrows
DELETE FROM session
WHERE id IN (SELECT s.id FROM session s WHERE s.expires_at < @before::timestamptz LIMIT @batch);
