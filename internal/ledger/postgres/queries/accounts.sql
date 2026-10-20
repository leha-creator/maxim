-- Счета учёта (06 §2.1). Счёт создаётся при первом обращении; строки блокируются в порядке
-- id после блокировок вызывающего модуля (R-5).

-- name: EnsureLedgerAccount :exec
INSERT INTO ledger_account (id, kind, organization_id, placement_id, balance, created_at, updated_at)
VALUES (@id, @kind, sqlc.narg(organization_id)::uuid, sqlc.narg(placement_id)::uuid, 0, @now, @now)
ON CONFLICT ON CONSTRAINT ledger_account_key DO NOTHING;

-- name: GetLedgerAccount :one
SELECT * FROM ledger_account
WHERE kind = @kind
    AND organization_id IS NOT DISTINCT FROM sqlc.narg(organization_id)::uuid
    AND placement_id IS NOT DISTINCT FROM sqlc.narg(placement_id)::uuid;

-- name: LockLedgerAccounts :many
SELECT * FROM ledger_account WHERE id = ANY(@ids::uuid[]) ORDER BY id FOR UPDATE;

-- name: SetLedgerAccountBalance :exec
UPDATE ledger_account SET balance = @balance, updated_at = @now WHERE id = @id;

-- name: SumLedgerBalancesByKind :many
SELECT kind, sum(balance)::bigint AS balance, count(*)::bigint AS accounts
FROM ledger_account GROUP BY kind ORDER BY kind;
