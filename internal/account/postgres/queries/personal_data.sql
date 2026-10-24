-- Выгрузка и обезличивание данных субъекта (152-ФЗ; этап «API администратора», R-26).

-- name: ListSubjectAcceptances :many
SELECT d.kind, d.version, a.accepted_at, a.ip
FROM legal_acceptance a JOIN legal_document d ON d.id = a.legal_document_id
WHERE a.user_id = @user_id
ORDER BY a.accepted_at, d.kind;

-- name: ListSubjectSessions :many
SELECT created_at, expires_at, revoked_at, ip, user_agent FROM session
WHERE user_id = @user_id ORDER BY created_at;

-- name: CountOrganizationMembers :one
SELECT count(*) FROM organization_member WHERE organization_id = @organization_id;

-- name: SetUserAnonymized :exec
UPDATE "user" SET email = @email, status = @status, blocked_at = sqlc.narg(blocked_at),
    blocked_reason = sqlc.narg(blocked_reason), anonymized_at = sqlc.narg(anonymized_at), updated_at = @at::timestamptz
WHERE id = @id;

-- name: ClearUserSessions :execrows
UPDATE session SET ip = NULL, user_agent = NULL, revoked_at = COALESCE(revoked_at, @at::timestamptz),
    updated_at = @at::timestamptz
WHERE user_id = @user_id;

-- name: DeleteLoginCodesByEmail :execrows
DELETE FROM auth_code WHERE purpose = 'login' AND email = @email;

-- name: ClearOrganizationContacts :exec
-- ПРЕДПОЛОЖЕНИЕ (Q-33): наименование, ИНН, подписант и реквизиты нужны для учёта — не трогаются.
UPDATE organization SET contact_name = '', phone = '', updated_at = @at::timestamptz WHERE id = @id;
