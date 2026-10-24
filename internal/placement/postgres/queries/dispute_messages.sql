-- Переписка по спору (R-9 этапа «API администратора»). Только добавление.

-- name: InsertDisputeMessage :exec
INSERT INTO dispute_message (id, dispute_id, author_type, author_user_id, recipient, text, created_at)
VALUES (@id, @dispute_id, @author_type, @author_user_id, @recipient, @text, @created_at);

-- name: ListDisputeMessages :many
SELECT * FROM dispute_message WHERE dispute_id = @dispute_id ORDER BY created_at, id;
