-- Оферты и их принятие (04 §2). legal_acceptance неизменяема.

-- name: CurrentLegalDocuments :many
SELECT DISTINCT ON (kind) * FROM legal_document ORDER BY kind, version DESC;

-- name: AcceptedLegalDocumentIDs :many
SELECT legal_document_id FROM legal_acceptance WHERE organization_id = @organization_id;

-- name: InsertLegalAcceptance :exec
INSERT INTO legal_acceptance (id, legal_document_id, user_id, organization_id, accepted_at, ip, created_at)
VALUES (@id, @legal_document_id, @user_id, @organization_id, @accepted_at, @ip, @accepted_at)
ON CONFLICT (organization_id, legal_document_id) DO NOTHING;

-- name: LatestLegalVersion :one
SELECT COALESCE(max(version), 0)::int FROM legal_document WHERE kind = @kind;

-- name: InsertLegalDocument :exec
INSERT INTO legal_document (id, kind, version, published_at, storage_key, sha256, created_at)
VALUES (@id, @kind, @version, @published_at, @storage_key, @sha256, @created_at);
