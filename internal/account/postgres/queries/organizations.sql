-- Организации, участники и профили (04 §2). Выборки кабинета ограничены
-- организацией пользователя (07 §4).

-- name: InsertOrganization :exec
INSERT INTO organization (
    id, legal_type, inn, name, contact_name, phone, kpp, address, tax_regime, vat_rate,
    tax_document_storage_key, tax_document_sha256, signer_position, signer_name, signer_basis_name,
    signer_basis_number, bank_account, bank_name, bank_corr_account, bank_bik, is_platform, status,
    created_at, updated_at)
VALUES (
    @id, @legal_type, @inn, @name, @contact_name, @phone, @kpp, @address, @tax_regime, @vat_rate,
    @tax_document_storage_key, @tax_document_sha256, @signer_position, @signer_name, @signer_basis_name,
    @signer_basis_number, @bank_account, @bank_name, @bank_corr_account, @bank_bik, false, @status,
    @created_at, @updated_at);

-- name: UpdateOrganization :exec
UPDATE organization SET
    name = @name, contact_name = @contact_name, phone = @phone, kpp = @kpp, address = @address,
    tax_regime = @tax_regime, vat_rate = @vat_rate, tax_document_storage_key = @tax_document_storage_key,
    tax_document_sha256 = @tax_document_sha256, signer_position = @signer_position, signer_name = @signer_name,
    signer_basis_name = @signer_basis_name, signer_basis_number = @signer_basis_number,
    bank_account = @bank_account, bank_name = @bank_name, bank_corr_account = @bank_corr_account,
    bank_bik = @bank_bik, status = @status, updated_at = @updated_at
WHERE id = @id;

-- name: InsertOrganizationMember :exec
INSERT INTO organization_member (id, organization_id, user_id, role, created_at, updated_at)
VALUES (@id, @organization_id, @user_id, @role, @now::timestamptz, @now::timestamptz);

-- name: InsertAdvertiserProfile :exec
INSERT INTO advertiser_profile (organization_id, created_at, updated_at)
VALUES (@organization_id, @now::timestamptz, @now::timestamptz)
ON CONFLICT (organization_id) DO NOTHING;

-- name: UpsertBloggerProfile :exec
INSERT INTO blogger_profile (organization_id, full_name, created_at, updated_at)
VALUES (@organization_id, @full_name, @now::timestamptz, @now::timestamptz)
ON CONFLICT (organization_id) DO UPDATE SET full_name = EXCLUDED.full_name, updated_at = EXCLUDED.updated_at
WHERE blogger_profile.full_name IS DISTINCT FROM EXCLUDED.full_name;

-- name: GetOrganizationByUser :one
SELECT sqlc.embed(o),
       (ap.organization_id IS NOT NULL)::boolean AS is_advertiser,
       (bp.organization_id IS NOT NULL)::boolean AS is_blogger,
       bp.full_name AS blogger_full_name
FROM organization o
JOIN organization_member m ON m.organization_id = o.id
LEFT JOIN advertiser_profile ap ON ap.organization_id = o.id
LEFT JOIN blogger_profile bp ON bp.organization_id = o.id
WHERE m.user_id = @user_id;

-- name: GetOrganizationForUpdate :one
SELECT sqlc.embed(o),
       (ap.organization_id IS NOT NULL)::boolean AS is_advertiser,
       (bp.organization_id IS NOT NULL)::boolean AS is_blogger,
       bp.full_name AS blogger_full_name
FROM organization o
LEFT JOIN advertiser_profile ap ON ap.organization_id = o.id
LEFT JOIN blogger_profile bp ON bp.organization_id = o.id
WHERE o.id = @id
FOR UPDATE OF o;

-- name: INNExists :one
SELECT EXISTS (SELECT 1 FROM organization WHERE inn = @inn)::boolean;

-- name: GetOrganizationRequisites :one
-- Название и ИНН для маркировки поста (модуль post, R-14); без блокировки.
SELECT name, inn FROM organization WHERE id = @id;
