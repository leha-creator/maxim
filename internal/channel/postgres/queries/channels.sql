-- Каналы (04 §3). Кабинетные выборки ограничены организацией (07 §4).

-- name: InsertChannel :exec
INSERT INTO channel (
    id, organization_id, max_chat_id, title, max_description, icon_url, link, is_public,
    added_by_max_user_id, description, category_id, status, status_reason, status_before_disconnect,
    unclaimed_since, bot_is_admin, bot_permissions, bot_permissions_checked_at, subscribers_count,
    rkn_number, rkn_status, rkn_checked_at, rating, moderated_at, lock_version, created_at, updated_at
) VALUES (
    @id, @organization_id, @max_chat_id, @title, @max_description, @icon_url, @link, @is_public,
    @added_by_max_user_id, @description, @category_id, @status, @status_reason, @status_before_disconnect,
    @unclaimed_since, @bot_is_admin, @bot_permissions, @bot_permissions_checked_at, @subscribers_count,
    @rkn_number, @rkn_status, @rkn_checked_at, @rating, @moderated_at, @lock_version, @created_at, @updated_at
);

-- name: SaveChannel :execrows
-- Оптимистическая блокировка (04 §1): обновляется строка с предыдущей версией.
UPDATE channel SET
    organization_id = @organization_id, title = @title, max_description = @max_description,
    icon_url = @icon_url, link = @link, is_public = @is_public, added_by_max_user_id = @added_by_max_user_id,
    description = @description, category_id = @category_id, status = @status, status_reason = @status_reason,
    status_before_disconnect = @status_before_disconnect, unclaimed_since = @unclaimed_since,
    bot_is_admin = @bot_is_admin, bot_permissions = @bot_permissions,
    bot_permissions_checked_at = @bot_permissions_checked_at, subscribers_count = @subscribers_count,
    rkn_number = @rkn_number, rkn_status = @rkn_status, rkn_checked_at = @rkn_checked_at,
    rkn_checked_by = @rkn_checked_by, moderated_at = @moderated_at, moderated_by = @moderated_by,
    lock_version = @lock_version, updated_at = @updated_at
WHERE id = @id AND lock_version = @prev_lock_version;

-- name: GetChannelByMaxChatIDForUpdate :one
SELECT * FROM channel WHERE max_chat_id = @max_chat_id FOR UPDATE;

-- name: GetChannelByMaxChatID :one
SELECT * FROM channel WHERE max_chat_id = @max_chat_id;

-- name: GetChannelForUpdate :one
SELECT * FROM channel WHERE id = @id FOR UPDATE;

-- name: GetChannelForOrganization :one
SELECT * FROM channel WHERE id = @id AND organization_id = @organization_id;

-- name: GetChannelForOrganizationForUpdate :one
SELECT * FROM channel WHERE id = @id AND organization_id = @organization_id FOR UPDATE;

-- name: ListChannelsByOrganization :many
SELECT * FROM channel WHERE organization_id = @organization_id ORDER BY created_at, id;

-- name: ListUnclaimedChannelsBefore :many
SELECT id FROM channel
WHERE status = 'unclaimed' AND unclaimed_since < @before::timestamptz
ORDER BY unclaimed_since, id
LIMIT @lim;

-- name: ListChannelsForMetrics :many
-- R-13: каналы, которым считаются суточные метрики, порцией по id.
SELECT id FROM channel
WHERE status IN ('moderation', 'active', 'suspended') AND id > @after::uuid
ORDER BY id
LIMIT @lim;

-- name: SetChannelOrdPlatformReady :execrows
-- R-14 этапа «Маркировка»: площадка в ОРД зарегистрирована; первая отметка остаётся.
UPDATE channel SET ord_platform_ready_at = @at
WHERE id = @id AND ord_platform_ready_at IS NULL;
