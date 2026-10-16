package bootstrap

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/leha-creator/maxim/internal/platform/clock"
	"github.com/leha-creator/maxim/internal/platform/job"
)

// modules — собранные модули приложения. Заполняется в задаче 12 плана.
type modules struct{}

func newModules(ctx context.Context, cfg Config, pool *pgxpool.Pool, clk clock.Clock, log *slog.Logger) (*modules, error) {
	return &modules{}, nil
}

// registerJobs регистрирует обработчики заданий всех модулей.
func (m *modules) registerJobs(w *job.Worker) {}
