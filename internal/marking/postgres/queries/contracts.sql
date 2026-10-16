-- Договоры в ОРД (04 §6): изначальный и с блогером.

-- name: InsertOrdContract :execrows
INSERT INTO ord_contract (
    id, kind, client_organization_id, contractor_organization_id, number, signed_date, ord_external_id,
    sync_status, sync_request_id, sync_error, sync_attempts, synced_at, payload_hash, ord_status, sent_at,
    created_at, updated_at
) VALUES (
    @id, @kind, @client_organization_id, @contractor_organization_id, @number, @signed_date, @ord_external_id,
    @sync_status, @sync_request_id, @sync_error, @sync_attempts, @synced_at, @payload_hash, @ord_status, @sent_at,
    @created_at, @updated_at
) ON CONFLICT (kind, client_organization_id, contractor_organization_id) DO NOTHING;

-- name: GetOrdContractByParties :one
SELECT * FROM ord_contract
WHERE kind = @kind AND client_organization_id = @client_organization_id
  AND contractor_organization_id = @contractor_organization_id;

-- name: GetOrdContractForUpdate :one
SELECT * FROM ord_contract WHERE id = @id FOR UPDATE;

-- name: SaveOrdContract :exec
UPDATE ord_contract SET
    number = @number, signed_date = @signed_date, ord_external_id = @ord_external_id, sync_status = @sync_status,
    sync_request_id = @sync_request_id, sync_error = @sync_error, sync_attempts = @sync_attempts,
    synced_at = @synced_at, payload_hash = @payload_hash, ord_status = @ord_status, sent_at = @sent_at,
    updated_at = @updated_at
WHERE id = @id;
