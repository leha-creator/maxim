-- Поиск зависших объектов (07 §3). Остальные проверки таблицы 07 §3 добавят
-- модули своих этапов.

-- name: StaleInboundEvents :many
-- Входящее событие в received дольше порога.
SELECT id::text AS id FROM inbound_event
WHERE status = 'received' AND created_at < @before::timestamptz
ORDER BY created_at
LIMIT 500;

-- name: FailedJobsWithoutAlert :many
-- Задание в failed, по которому ещё не поднято оповещение (например, отправка не удалась).
SELECT j.id::text AS id FROM job j
WHERE j.status = 'failed' AND j.finished_at < @before::timestamptz
  AND NOT EXISTS (SELECT 1 FROM alert a WHERE a.object_key = j.id::text)
ORDER BY j.finished_at DESC
LIMIT 500;
