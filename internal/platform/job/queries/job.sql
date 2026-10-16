-- Очередь заданий на таблице job (07 §2).

-- name: InsertJob :execrows
-- Живое задание с тем же dedupe_key не дублируется: строка не вставляется.
INSERT INTO job (id, kind, dedupe_key, payload, run_at, status, attempts, max_attempts, created_at, updated_at)
VALUES (@id, @kind, @dedupe_key, @payload, @run_at, 'pending', 0, @max_attempts, @now::timestamptz, @now::timestamptz)
ON CONFLICT (dedupe_key) WHERE status IN ('pending', 'running') DO NOTHING;

-- name: ClaimJobs :many
-- Берёт готовые задания с блокировкой строк и пропуском заблокированных.
UPDATE job
SET status = 'running', locked_by = @locked_by, locked_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE id IN (
    SELECT j.id FROM job j
    WHERE j.status = 'pending' AND j.run_at <= @now::timestamptz AND j.kind = ANY (@kinds::text[])
    ORDER BY j.run_at
    LIMIT @batch
    FOR UPDATE SKIP LOCKED)
RETURNING id, kind, payload, attempts, max_attempts, run_at;

-- name: CompleteJob :execrows
UPDATE job
SET status = 'succeeded', finished_at = @now::timestamptz, updated_at = @now::timestamptz, locked_by = NULL, locked_at = NULL,
    payload = CASE WHEN @wipe_payload::boolean THEN '{}'::jsonb ELSE payload END
WHERE id = @id AND status = 'running' AND locked_by = @locked_by;

-- name: RetryJob :execrows
UPDATE job
SET status = 'pending', attempts = @attempts, run_at = @run_at, last_error = @last_error,
    updated_at = @now::timestamptz, locked_by = NULL, locked_at = NULL
WHERE id = @id AND status = 'running' AND locked_by = @locked_by;

-- name: FailJob :execrows
UPDATE job
SET status = 'failed', attempts = @attempts, last_error = @last_error, finished_at = @now::timestamptz,
    updated_at = @now::timestamptz, locked_by = NULL, locked_at = NULL,
    payload = CASE WHEN @wipe_payload::boolean THEN '{}'::jsonb ELSE payload END
WHERE id = @id AND status = 'running' AND locked_by = @locked_by;

-- name: ReleaseJobs :execrows
-- Возвращает в очередь задания экземпляра, не доделанные к остановке.
UPDATE job
SET status = 'pending', updated_at = @now::timestamptz, locked_by = NULL, locked_at = NULL
WHERE id = ANY (@ids::uuid[]) AND status = 'running' AND locked_by = @locked_by;

-- name: ReleaseAbandonedJobs :many
-- Задание в running дольше порога считается брошенным (07 §2).
UPDATE job
SET status = 'pending', updated_at = @now::timestamptz, locked_by = NULL, locked_at = NULL
WHERE status = 'running' AND locked_at < @locked_before::timestamptz
RETURNING id, kind;

-- name: JobCounts :many
SELECT kind, status, count(*)::bigint AS count
FROM job
WHERE status IN ('pending', 'running', 'failed')
GROUP BY kind, status;

-- name: OverdueJobCounts :many
SELECT kind, count(*)::bigint AS count
FROM job
WHERE status = 'pending' AND run_at < @run_before::timestamptz
GROUP BY kind;

-- name: GetJob :one
SELECT * FROM job WHERE id = @id;
