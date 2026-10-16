package bootstrap

import (
	"context"
	"log/slog"

	"golang.org/x/sync/errgroup"

	"github.com/leha-creator/maxim/internal/platform/db"
	"github.com/leha-creator/maxim/internal/platform/job"
	"github.com/leha-creator/maxim/internal/platform/logx"
	"github.com/leha-creator/maxim/internal/platform/metrics"
)

// runWorker — режим worker: обработчик заданий и метрики очереди.
func runWorker(ctx context.Context, cfg Config, log *slog.Logger) error {
	pool, err := db.Open(ctx, cfg.DatabaseURL, cfg.DBMaxConns, log)
	if err != nil {
		return err
	}
	defer pool.Close()
	in, err := newInfra(ctx, cfg, ModeWorker, pool, log)
	if err != nil {
		return err
	}
	w := newWorker(in, newModules(cfg, in))
	reg := metrics.New()
	reg.MustRegister(job.NewCollector(pool, in.clock, log.With(logx.KeyModule, "job")))

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return w.Run(gctx) })
	g.Go(func() error { return serve(gctx, log, metricsServer(cfg.MetricsAddr, reg)) })
	return g.Wait()
}

// newWorker создаёт обработчик заданий со всеми видами заданий модулей.
func newWorker(in *infra, m *modules) *job.Worker {
	w := job.NewWorker(in.pool, in.clock, in.log.With(logx.KeyModule, "job"))
	m.registerJobs(w)
	return w
}
