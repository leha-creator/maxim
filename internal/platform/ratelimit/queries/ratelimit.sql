-- Корзина токенов общего ограничителя (05 §2.8, R-1).

-- name: EnsureBucket :exec
INSERT INTO rate_limit_bucket (key, tokens, refilled_at)
VALUES (@key, @tokens, @now::timestamptz)
ON CONFLICT (key) DO NOTHING;

-- name: LockBucket :one
SELECT tokens, refilled_at FROM rate_limit_bucket WHERE key = @key FOR UPDATE;

-- name: UpdateBucket :exec
UPDATE rate_limit_bucket SET tokens = @tokens, refilled_at = @refilled_at WHERE key = @key;
