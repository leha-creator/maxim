-- Креативы версий поста (04 §6, D-07).

-- name: InsertCreative :execrows
INSERT INTO creative (
    id, post_version_id, ord_contract_id, erid, erid_received_at, erir_status, erir_error, ord_external_id,
    sync_status, sync_request_id, sync_error, sync_attempts, synced_at, payload_hash, ord_status, sent_at,
    created_at, updated_at
) VALUES (
    @id, @post_version_id, @ord_contract_id, @erid, @erid_received_at, @erir_status, @erir_error, @ord_external_id,
    @sync_status, @sync_request_id, @sync_error, @sync_attempts, @synced_at, @payload_hash, @ord_status, @sent_at,
    @created_at, @updated_at
) ON CONFLICT (post_version_id) DO NOTHING;

-- name: GetCreativeByVersion :one
SELECT * FROM creative WHERE post_version_id = @post_version_id;

-- name: GetCreativeForUpdate :one
SELECT * FROM creative WHERE id = @id FOR UPDATE;

-- name: SaveCreative :exec
UPDATE creative SET
    ord_contract_id = @ord_contract_id, erid = @erid, erid_received_at = @erid_received_at,
    erir_status = @erir_status, erir_error = @erir_error, ord_external_id = @ord_external_id,
    sync_status = @sync_status, sync_request_id = @sync_request_id, sync_error = @sync_error,
    sync_attempts = @sync_attempts, synced_at = @synced_at, payload_hash = @payload_hash, ord_status = @ord_status,
    sent_at = @sent_at, updated_at = @updated_at
WHERE id = @id;

-- name: DeleteUnsentCreative :execrows
-- R-11: удаляется только креатив, который не отправлялся.
DELETE FROM creative WHERE id = @id AND sent_at IS NULL AND erid IS NULL;
