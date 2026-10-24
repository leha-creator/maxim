-- Попытки выплат (04 §7, S-15, S-19).

-- name: InsertPayout :exec
INSERT INTO payout (id, placement_id, blogger_organization_id, method, rocketwork_contractor_id, amount, attempt,
    status, external_id, provider_fee, receipt_url, bank_reference, failure_code, failure_message, sent_at,
    completed_at, idempotency_key, lock_version, created_by, created_at, updated_at)
VALUES (@id, @placement_id, @blogger_organization_id, @method, @rocketwork_contractor_id, @amount, @attempt,
    @status, @external_id, @provider_fee, @receipt_url, @bank_reference, @failure_code, @failure_message, @sent_at,
    @completed_at, @idempotency_key, 0, @created_by, @created_at, @updated_at);

-- name: GetPayout :one
SELECT * FROM payout WHERE id = @id;

-- name: GetPayoutForUpdate :one
SELECT * FROM payout WHERE id = @id FOR UPDATE;

-- name: GetPayoutByExternalID :one
SELECT * FROM payout WHERE external_id = @external_id;

-- name: GetPayoutByExternalIDForUpdate :one
SELECT * FROM payout WHERE external_id = @external_id FOR UPDATE;

-- name: GetLatestPayout :one
SELECT * FROM payout WHERE placement_id = @placement_id ORDER BY attempt DESC LIMIT 1;

-- name: GetLatestPayoutForUpdate :one
SELECT * FROM payout WHERE placement_id = @placement_id ORDER BY attempt DESC LIMIT 1 FOR UPDATE;

-- name: UpdatePayout :execrows
-- Оптимистическая блокировка: строка меняется только при совпадении lock_version.
UPDATE payout SET status = @status, external_id = @external_id, provider_fee = @provider_fee,
    receipt_url = @receipt_url, bank_reference = @bank_reference, failure_code = @failure_code,
    failure_message = @failure_message, sent_at = @sent_at, completed_at = @completed_at,
    lock_version = lock_version + 1, updated_at = @updated_at
WHERE id = @id AND lock_version = @lock_version;

-- name: ListPayouts :many
SELECT * FROM payout
WHERE blogger_organization_id = @organization_id
  AND (NOT @has_cursor::bool OR (created_at, id) < (@cursor_at::timestamptz, @cursor_id::uuid))
ORDER BY created_at DESC, id DESC
LIMIT @lim;

-- name: PayoutSummary :one
-- Итог blg/payouts: получено и в работе; count — для PageMeta.total.
SELECT
    coalesce(sum(amount) FILTER (WHERE status = 'succeeded'), 0)::bigint AS received,
    coalesce(sum(amount) FILTER (WHERE status IN ('created', 'sent')), 0)::bigint AS in_progress,
    count(*)::int AS n
FROM payout WHERE blogger_organization_id = @organization_id;

-- name: SucceededSum :one
-- Годовой лимит самозанятого (R-13): выплаты, завершённые в [from, to).
SELECT coalesce(sum(amount), 0)::bigint AS total FROM payout
WHERE blogger_organization_id = @organization_id AND status = 'succeeded'
  AND completed_at >= @from_at AND completed_at < @to_at;

-- name: AdminListPayouts :many
SELECT * FROM payout
WHERE (@status::text = '' OR status = @status)
  AND (NOT @has_org::bool OR blogger_organization_id = @organization_id)
  AND (NOT @has_from::bool OR created_at >= @from_at)
  AND (NOT @has_to::bool OR created_at < @to_at)
ORDER BY created_at DESC, id DESC
LIMIT @lim;
