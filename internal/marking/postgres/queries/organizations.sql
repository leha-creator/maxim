-- Контрагенты в ОРД (04 §6). Вставка при гонке двух заданий не падает: ON CONFLICT
-- DO NOTHING и повторное чтение (R-4).

-- name: InsertOrdOrganization :execrows
INSERT INTO ord_organization (
    id, organization_id, is_rr, ord_external_id, sync_status, sync_request_id, sync_error, sync_attempts,
    synced_at, payload_hash, ord_status, sent_at, created_at, updated_at
) VALUES (
    @id, @organization_id, @is_rr, @ord_external_id, @sync_status, @sync_request_id, @sync_error, @sync_attempts,
    @synced_at, @payload_hash, @ord_status, @sent_at, @created_at, @updated_at
) ON CONFLICT (organization_id) DO NOTHING;

-- name: GetOrdOrganizationByOrganization :one
SELECT * FROM ord_organization WHERE organization_id = @organization_id;

-- name: GetOrdOrganizationForUpdate :one
SELECT * FROM ord_organization WHERE id = @id FOR UPDATE;

-- name: SaveOrdOrganization :exec
UPDATE ord_organization SET
    is_rr = @is_rr, ord_external_id = @ord_external_id, sync_status = @sync_status,
    sync_request_id = @sync_request_id, sync_error = @sync_error, sync_attempts = @sync_attempts,
    synced_at = @synced_at, payload_hash = @payload_hash, ord_status = @ord_status, sent_at = @sent_at,
    updated_at = @updated_at
WHERE id = @id;
