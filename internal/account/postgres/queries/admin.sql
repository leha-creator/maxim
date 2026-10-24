-- Организации и пользователи для администратора (этап «API администратора», R-12, R-13).

-- name: AdminListOrganizations :many
-- Поиск по началу ИНН (модератору — только по ИНН целиком, R-14), части наименования или точной почте участника; организация-платформа не
-- показывается. Кабинет — по профилю.
SELECT o.id, o.name, o.inn, o.legal_type, o.status, o.created_at,
       (ap.organization_id IS NOT NULL)::boolean AS is_advertiser,
       (bp.organization_id IS NOT NULL)::boolean AS is_blogger
FROM organization o
LEFT JOIN advertiser_profile ap ON ap.organization_id = o.id
LEFT JOIN blogger_profile bp ON bp.organization_id = o.id
WHERE NOT o.is_platform
  AND (@query::text = '' OR (CASE WHEN @exact_inn::boolean THEN o.inn = @query ELSE o.inn LIKE @query || '%' END) OR lower(o.name) LIKE '%' || lower(@query) || '%'
       OR EXISTS (SELECT 1 FROM organization_member m JOIN "user" u ON u.id = m.user_id
                  WHERE m.organization_id = o.id AND u.email = lower(@query)))
  AND (@cabinet::text = '' OR (@cabinet = 'advertiser' AND ap.organization_id IS NOT NULL)
       OR (@cabinet = 'blogger' AND bp.organization_id IS NOT NULL))
ORDER BY o.created_at DESC, o.id DESC
LIMIT @lim OFFSET @off;

-- name: AdminCountOrganizations :one
SELECT count(*)::int
FROM organization o
LEFT JOIN advertiser_profile ap ON ap.organization_id = o.id
LEFT JOIN blogger_profile bp ON bp.organization_id = o.id
WHERE NOT o.is_platform
  AND (@query::text = '' OR (CASE WHEN @exact_inn::boolean THEN o.inn = @query ELSE o.inn LIKE @query || '%' END) OR lower(o.name) LIKE '%' || lower(@query) || '%'
       OR EXISTS (SELECT 1 FROM organization_member m JOIN "user" u ON u.id = m.user_id
                  WHERE m.organization_id = o.id AND u.email = lower(@query)))
  AND (@cabinet::text = '' OR (@cabinet = 'advertiser' AND ap.organization_id IS NOT NULL)
       OR (@cabinet = 'blogger' AND bp.organization_id IS NOT NULL));

-- name: AdminGetOrganization :one
SELECT sqlc.embed(o),
       (ap.organization_id IS NOT NULL)::boolean AS is_advertiser,
       (bp.organization_id IS NOT NULL)::boolean AS is_blogger,
       bp.full_name AS blogger_full_name
FROM organization o
LEFT JOIN advertiser_profile ap ON ap.organization_id = o.id
LEFT JOIN blogger_profile bp ON bp.organization_id = o.id
WHERE o.id = @id AND NOT o.is_platform;

-- name: ListOrganizationMembers :many
SELECT u.* FROM organization_member m JOIN "user" u ON u.id = m.user_id
WHERE m.organization_id = @organization_id
ORDER BY (m.role = 'owner') DESC, m.created_at, u.id;

-- name: GetUserForUpdate :one
SELECT * FROM "user" WHERE id = @id FOR UPDATE;

-- name: SetUserBlocked :exec
-- Блокировка и снятие (R-12): blocked_at и основание — только у blocked.
UPDATE "user" SET status = @status, blocked_at = sqlc.narg(blocked_at), blocked_reason = sqlc.narg(blocked_reason),
    updated_at = @at::timestamptz
WHERE id = @id;
