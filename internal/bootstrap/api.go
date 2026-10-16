package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/leha-creator/maxim/internal/platform/logx"
	"github.com/leha-creator/maxim/internal/platform/metrics"
)

const (
	apiShutdownTimeout = 15 * time.Second
	readHeaderTimeout  = 10 * time.Second
)

// namedServer — HTTP-сервер с именем для журнала.
type namedServer struct {
	name string
	srv  *http.Server
}

func newHTTPServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}

func metricsServer(addr string, reg *metrics.Registry) namedServer {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", reg.Handler())
	return namedServer{name: "metrics", srv: newHTTPServer(addr, mux)}
}

// serve запускает серверы и останавливает их плавно, когда отменяется ctx.
func serve(ctx context.Context, log *slog.Logger, servers ...namedServer) error {
	g, gctx := errgroup.WithContext(ctx)
	for _, s := range servers {
		ln, err := net.Listen("tcp", s.srv.Addr)
		if err != nil {
			return fmt.Errorf("listen %s on %s: %w", s.name, s.srv.Addr, err)
		}
		log.Info("component ready", logx.KeyOp, "Bootstrap.Serve", "component", s.name, "addr", ln.Addr().String())
		g.Go(func() error {
			if err := s.srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return fmt.Errorf("%s server: %w", s.name, err)
			}
			return nil
		})
	}
	g.Go(func() error {
		<-gctx.Done()
		start := time.Now()
		log.Info("shutdown started", logx.KeyOp, "Bootstrap.Serve")
		sctx, cancel := context.WithTimeout(context.Background(), apiShutdownTimeout)
		defer cancel()
		var errs []error
		for _, s := range servers {
			if err := s.srv.Shutdown(sctx); err != nil {
				errs = append(errs, fmt.Errorf("shutdown %s: %w", s.name, err))
			}
		}
		log.Info("shutdown finished", logx.KeyOp, "Bootstrap.Serve", "duration_ms", time.Since(start).Milliseconds())
		return errors.Join(errs...)
	})
	return g.Wait()
}
