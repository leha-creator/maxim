-- Закрывающие документы (R-16).

-- name: NextClosingActNumber :one
SELECT nextval('closing_act_number_seq')::bigint AS number;

-- name: InsertClosingAct :exec
INSERT INTO closing_act (id, organization_id, period, number, placements_amount, fee_amount,
    report_pdf_storage_key, act_pdf_storage_key, created_at)
VALUES (@id, @organization_id, @period, @number, @placements_amount, @fee_amount,
    @report_pdf_storage_key, @act_pdf_storage_key, @created_at);

-- name: ClosingActExists :one
SELECT EXISTS (SELECT 1 FROM closing_act WHERE organization_id = @organization_id AND period = @period) AS found;

-- name: ListClosingActs :many
SELECT * FROM closing_act
WHERE organization_id = @organization_id
  AND (NOT @has_cursor::bool OR (period, id) < (@cursor_period::date, @cursor_id::uuid))
ORDER BY period DESC, id DESC
LIMIT @lim;

-- name: GetClosingAct :one
SELECT * FROM closing_act WHERE id = @id AND organization_id = @organization_id;
