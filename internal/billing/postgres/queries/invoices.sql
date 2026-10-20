-- Счета на пополнение баланса (S-06, R-1, R-10, R-11).

-- name: NextInvoiceNumber :one
SELECT nextval('invoice_number_seq')::bigint AS number;

-- name: InsertInvoice :exec
INSERT INTO invoice (id, number, organization_id, amount, status, expires_at, paid_at, paid_amount,
    payer_name, payer_inn, payer_bank_account, payer_bank_name, payer_bank_bik, payer_bank_corr_account,
    pdf_storage_key, created_by, cancel_reason, created_at, updated_at)
VALUES (@id, @number, @organization_id, @amount, @status, @expires_at, @paid_at, @paid_amount,
    @payer_name, @payer_inn, @payer_bank_account, @payer_bank_name, @payer_bank_bik, @payer_bank_corr_account,
    @pdf_storage_key, @created_by, @cancel_reason, @created_at, @updated_at);

-- name: GetInvoice :one
SELECT * FROM invoice WHERE id = @id AND organization_id = @organization_id;

-- name: GetInvoiceByID :one
SELECT * FROM invoice WHERE id = @id;

-- name: GetInvoiceByIDForUpdate :one
SELECT * FROM invoice WHERE id = @id FOR UPDATE;

-- name: ListInvoicesByNumberForUpdate :many
SELECT * FROM invoice WHERE number = ANY(@numbers::text[]) ORDER BY id FOR UPDATE;

-- name: ListIssuedInvoicesByINNAndAmountForUpdate :many
SELECT * FROM invoice WHERE status = 'issued' AND payer_inn = @payer_inn AND amount = @amount ORDER BY id FOR UPDATE;

-- name: ListInvoices :many
SELECT * FROM invoice
WHERE organization_id = @organization_id
  AND (NOT @has_cursor::bool OR (created_at, id) < (@cursor_at::timestamptz, @cursor_id::uuid))
ORDER BY created_at DESC, id DESC
LIMIT @lim;

-- name: ListDueInvoicesForUpdate :many
SELECT * FROM invoice WHERE status = 'issued' AND expires_at <= @now
ORDER BY expires_at, id
LIMIT @lim
FOR UPDATE SKIP LOCKED;

-- name: UpdateInvoice :execrows
UPDATE invoice SET status = @status, paid_at = @paid_at, paid_amount = @paid_amount, cancel_reason = @cancel_reason,
    updated_at = @updated_at
WHERE id = @id;

-- name: ListInvoiceNumbers :many
SELECT id, number FROM invoice WHERE id = ANY(@ids::uuid[]);

-- name: CountInvoicesByStatus :many
SELECT status, count(*)::int AS n FROM invoice GROUP BY status ORDER BY status;
