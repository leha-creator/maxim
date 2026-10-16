// Package jobs — обработчики заданий модуля размещений (07 §2); сценарии выполняет app.
// publish_placement и remove_placement регистрирует этап «Автопостинг и контроль»: до него
// worker их не берёт, задания ждут (R-12).
package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/leha-creator/maxim/internal/placement/app"
	"github.com/leha-creator/maxim/internal/placement/domain"
	"github.com/leha-creator/maxim/internal/platform/job"
)

// Register регистрирует виды заданий модуля: таймеры размещений — с высоким приоритетом (R-8 этапа
// «Интеграция планировщика»).
func Register(w *job.Worker, svc *app.Service) {
	w.Register(domain.KindPlacementTimer, func(ctx context.Context, r job.Running) error {
		var p domain.TimerPayload
		if err := decode(r, &p); err != nil {
			return err
		}
		return svc.HandleTimer(ctx, p, r.ID)
	}, job.HandlerOptions{Priority: job.PriorityHigh, ObjectRef: objectRef})
}

// objectRef — объект задания для списка затронутых после простоя: placement:<id>.
func objectRef(payload []byte) string {
	var p domain.PlacementPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return ""
	}
	return "placement:" + p.PlacementID.String()
}

func decode(r job.Running, v any) error {
	if err := json.Unmarshal(r.Payload, v); err != nil {
		return job.Permanent(fmt.Errorf("decode %s payload: %w", r.Kind, err))
	}
	return nil
}
