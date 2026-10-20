-- Журнал проводок (06 §2.2): только добавление. Ключ идемпотентности уникален (R-2).

-- name: GetLedgerTransactionByKey :one
SELECT id, kind, idempotency_key, entries_hash, created_at
FROM ledger_transaction WHERE idempotency_key = @idempotency_key;

-- name: InsertLedgerTransaction :exec
INSERT INTO ledger_transaction (
    id, kind, placement_id, invoice_id, bank_transaction_id, payout_id, actor_type, actor_user_id,
    reason, idempotency_key, entries_hash, created_at
) VALUES (
    @id, @kind, @placement_id, @invoice_id, @bank_transaction_id, @payout_id, @actor_type, @actor_user_id,
    @reason, @idempotency_key, @entries_hash, @created_at
);

-- name: InsertLedgerEntry :exec
INSERT INTO ledger_entry (id, ledger_transaction_id, position, debit_account_id, credit_account_id, amount, created_at)
VALUES (@id, @ledger_transaction_id, @position, @debit_account_id, @credit_account_id, @amount, @created_at);
