package app

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/leha-creator/maxim/internal/placement/domain"
	"github.com/leha-creator/maxim/internal/platform/logx"
)

// HandleTimer — сработал таймер размещения (03 §5): T-01 → P-05, T-13 → P-18, T-14 → P-06
// window_elapsed. Размещение не в ожидаемом состоянии — таймер устарел, ничего не меняется
// (03 §11). Источник перехода в истории — задание.
func (s *Service) HandleTimer(ctx context.Context, payload domain.TimerPayload, jobID uuid.UUID) error {
	const op = "Placement.Timer"
	p0, err := s.Placements.Get(ctx, payload.PlacementID)
	if errors.Is(err, domain.ErrNotFound) {
		s.log.WarnContext(ctx, "placement timer for unknown placement", logx.KeyOp, op, KeyPlacementID, payload.PlacementID,
			"timer", payload.Timer)
		return nil
	}
	if err != nil {
		return err
	}
	return s.Tx.InTx(ctx, func(ctx context.Context) error {
		o, err := s.Orders.GetByIDForUpdate(ctx, p0.OrderID)
		if err != nil {
			return err
		}
		p, err := s.Placements.GetForUpdate(ctx, payload.PlacementID)
		if err != nil {
			return err
		}
		if p.Status != payload.Timer.ExpectedStatus() {
			s.log.DebugContext(ctx, "placement timer outdated", logx.KeyOp, op, KeyPlacementID, p.ID, "timer", payload.Timer,
				"status", p.Status)
			return nil
		}
		change, err := p.Expire(payload.Timer, s.Clock.Now())
		if err != nil {
			return s.rejected(ctx, op, p, domain.TransitionCode(payload.Timer), err)
		}
		if err := s.apply(ctx, op, o, p, change, source{JobID: &jobID}); err != nil {
			return err
		}
		if err := s.refreshOrder(ctx, op, &o); err != nil {
			return err
		}
		s.log.InfoContext(ctx, "placement timer fired", logx.KeyOp, op, KeyPlacementID, p.ID, "timer", payload.Timer,
			"result", change.To, "transition", change.Code)
		return nil
	})
}

// OverdueScheduled — ключи размещений в scheduled, чей publish_at раньше olderThan (07 §3):
// проверка зависших для platform/stuck.
func (s *Service) OverdueScheduled(ctx context.Context, olderThan time.Time) ([]string, error) {
	list, err := s.Placements.OverdueScheduled(ctx, olderThan)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(list))
	for _, id := range list {
		out = append(out, "placement:"+id.String())
	}
	return out, nil
}

// CountByStatus — число размещений по состояниям (07 §5.5).
func (s *Service) CountByStatus(ctx context.Context) (map[domain.Status]int, error) {
	return s.Placements.CountByStatus(ctx)
}
