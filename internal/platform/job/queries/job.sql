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

-- name: CancelJob :execrows
-- Отменяет ожидающее задание по dedupe_key. Задание в running не трогается:
-- его обработчик идемпотентен (03 §11).
UPDATE job
SET status = 'cancelled', finished_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE dedupe_key = @dedupe_key AND status = 'pending';

-- name: RescheduleJob :execrows
UPDATE job
SET run_at = @run_at, updated_at = @now::timestamptz
WHERE dedupe_key = @dedupe_key AND status = 'pending';

-- name: LastHeartbeat :one
-- Последняя отметка любого экземпляра worker (R-6).
SELECT seen_at FROM worker_heartbeat ORDER BY seen_at DESC LIMIT 1;

-- name: UpsertHeartbeat :exec
INSERT INTO worker_heartbeat (instance, started_at, seen_at)
VALUES (@instance, @now::timestamptz, @now::timestamptz)
ON CONFLICT (instance) DO UPDATE SET seen_at = EXCLUDED.seen_at;

-- name: OverdueJobs :many
-- Просроченные ожидающие задания указанных видов: список затронутых объектов после простоя.
SELECT id, kind, payload, run_at
FROM job
WHERE status = 'pending' AND run_at < @run_before::timestamptz AND kind = ANY (@kinds::text[])
ORDER BY run_at
LIMIT 1000;

-- name: DeleteFinishedJobsBefore :execrows
-- Очистка (R-10): завершённые задания указанных статусов старше срока, пачками.
DELETE FROM job
WHERE id IN (SELECT j.id FROM job j
             WHERE j.status = ANY (@statuses::text[]) AND j.finished_at < @before::timestamptz LIMIT @batch);

-- name: DeleteHeartbeatsBefore :execrows
DELETE FROM worker_heartbeat WHERE seen_at < @before::timestamptz;

-- name: OldestReadyJobs :many
-- Самое раннее время запуска готовых, но не взятых заданий по видам: задержка очереди.
SELECT kind, min(run_at)::timestamptz AS oldest_run_at
FROM job
WHERE status = 'pending' AND run_at <= @now::timestamptz
GROUP BY kind;
