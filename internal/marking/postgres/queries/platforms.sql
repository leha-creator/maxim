-- Площадки каналов в ОРД (04 §6, A-32).

-- name: InsertOrdPlatform :execrows
INSERT INTO ord_platform (
    id, channel_id, organization_id, name, url, ord_external_id, sync_status, sync_request_id, sync_error,
    sync_attempts, synced_at, payload_hash, ord_status, sent_at, created_at, updated_at
) VALUES (
    @id, @channel_id, @organization_id, @name, @url, @ord_external_id, @sync_status, @sync_request_id, @sync_error,
    @sync_attempts, @synced_at, @payload_hash, @ord_status, @sent_at, @created_at, @updated_at
) ON CONFLICT (channel_id) DO NOTHING;

-- name: GetOrdPlatformByChannel :one
SELECT * FROM ord_platform WHERE channel_id = @channel_id;

-- name: GetOrdPlatformForUpdate :one
SELECT * FROM ord_platform WHERE id = @id FOR UPDATE;

-- name: SaveOrdPlatform :exec
UPDATE ord_platform SET
    organization_id = @organization_id, name = @name, url = @url, ord_external_id = @ord_external_id,
    sync_status = @sync_status, sync_request_id = @sync_request_id, sync_error = @sync_error,
    sync_attempts = @sync_attempts, synced_at = @synced_at, payload_hash = @payload_hash, ord_status = @ord_status,
    sent_at = @sent_at, updated_at = @updated_at
WHERE id = @id;
