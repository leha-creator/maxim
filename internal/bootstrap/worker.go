package bootstrap

import (
	"context"
	"fmt"
	"log/slog"

	"golang.org/x/sync/errgroup"

	"github.com/leha-creator/maxim/internal/platform/clock"
	"github.com/leha-creator/maxim/internal/platform/db"
	"github.com/leha-creator/maxim/internal/platform/job"
	"github.com/leha-creator/maxim/internal/platform/logx"
	"github.com/leha-creator/maxim/internal/platform/metrics"
)

// runWorker — режим worker: обработчик заданий и метрики очереди.
func runWorker(ctx context.Context, cfg Config, log *slog.Logger) error {
	clk := clock.System{}
	pool, err := db.Open(ctx, cfg.DatabaseURL, cfg.DBMaxConns, log)
	if err != nil {
		return err
	}
	defer pool.Close()

	reg := metrics.New()
	reg.MustRegister(job.NewCollector(pool, clk, log.With(logx.KeyModule, "job")))
	w := job.NewWorker(pool, clk, log.With(logx.KeyModule, "job"))
	mods, err := newModules(ctx, cfg, pool, clk, log)
	if err != nil {
		return err
	}
	mods.registerJobs(w)
	if len(w.Kinds()) == 0 {
		return fmt.Errorf("no job kinds registered")
	}

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return w.Run(gctx) })
	g.Go(func() error { return serve(gctx, log, metricsServer(cfg.MetricsAddr, reg)) })
	return g.Wait()
}
