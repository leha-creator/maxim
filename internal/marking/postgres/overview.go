package postgres

import (
	"context"
	"time"

	"github.com/leha-creator/maxim/internal/marking/domain"
	"github.com/leha-creator/maxim/internal/marking/postgres/sqlc"
)

// Overview — выборки по всем видам объектов ОРД.
type Overview struct{ *Store }

// ListErrors — объекты в error по (updated_at, id) после курсора (S-20).
func (r *Overview) ListErrors(ctx context.Context, after *domain.ErrorCursor, limit int) ([]domain.ErrorItem, error) {
	p := sqlc.ListOrdErrorsParams{Lim: int32(limit)}
	if after != nil {
		at, id := after.UpdatedAt, after.ID
		p.AfterAt, p.AfterID = &at, &id
	}
	rows, err := r.q(ctx).ListOrdErrors(ctx, p)
	if err != nil {
		return nil, r.fail(ctx, "Overview.ListErrors", err)
	}
	out := make([]domain.ErrorItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.ErrorItem{
			Ref:        domain.ObjectRef{Type: domain.ObjectType(row.ObjectType), ID: row.ID},
			ExternalID: str(row.OrdExternalID), OrdStatus: str(row.OrdStatus), Error: str(row.SyncError),
			Attempts: int(row.SyncAttempts), UpdatedAt: row.UpdatedAt.UTC(), LastRequestID: row.LastRequestID,
		})
	}
	return out, nil
}

// StuckSent — объекты в sent с sent_at раньше before (07 §3).
func (r *Overview) StuckSent(ctx context.Context, before time.Time, limit int) ([]domain.StuckObject, error) {
	rows, err := r.q(ctx).ListOrdStuckSent(ctx, sqlc.ListOrdStuckSentParams{Before: before, Lim: int32(limit)})
	if err != nil {
		return nil, r.fail(ctx, "Overview.StuckSent", err)
	}
	out := make([]domain.StuckObject, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.StuckObject{
			Ref: domain.ObjectRef{Type: domain.ObjectType(row.ObjectType), ID: row.ID}, SentAt: ts(row.SentAt),
		})
	}
	return out, nil
}

// CreativesWithoutErid — креативы без erid, созданные раньше before (T-11).
func (r *Overview) CreativesWithoutErid(ctx context.Context, before time.Time, limit int) ([]domain.Creative, error) {
	rows, err := r.q(ctx).ListCreativesWithoutErid(ctx, sqlc.ListCreativesWithoutEridParams{Before: before, Lim: int32(limit)})
	if err != nil {
		return nil, r.fail(ctx, "Overview.CreativesWithoutErid", err)
	}
	out := make([]domain.Creative, 0, len(rows))
	for _, row := range rows {
		out = append(out, creativeFromRow(row))
	}
	return out, nil
}

// CountByStatus — число объектов по виду и состоянию; отсутствующие пары — 0.
func (r *Overview) CountByStatus(ctx context.Context) (map[domain.ObjectType]map[domain.SyncStatus]int, error) {
	rows, err := r.q(ctx).CountOrdObjectsByStatus(ctx)
	if err != nil {
		return nil, r.fail(ctx, "Overview.CountByStatus", err)
	}
	out := map[domain.ObjectType]map[domain.SyncStatus]int{}
	for _, t := range domain.ObjectTypes {
		out[t] = map[domain.SyncStatus]int{}
		for _, s := range domain.SyncStatuses {
			out[t][s] = 0
		}
	}
	for _, row := range rows {
		out[domain.ObjectType(row.ObjectType)][domain.SyncStatus(row.SyncStatus)] = int(row.N)
	}
	return out, nil
}
