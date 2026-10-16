package job

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/leha-creator/maxim/internal/platform/clock"
	"github.com/leha-creator/maxim/internal/platform/job/sqlc"
	"github.com/leha-creator/maxim/internal/platform/logx"
)

// Параметры обработчика по 07 §2.
const (
	retryBase        = 30 * time.Second
	retryMax         = time.Hour
	abandonedAfter   = 10 * time.Minute
	defaultBatch     = 8
	pollInterval     = time.Second
	abandonInterval  = 30 * time.Second
	jobTimeout       = 5 * time.Minute // меньше порога брошенного задания
	shutdownDeadline = 30 * time.Second
)

// Running — задание, взятое в работу.
type Running struct {
	ID       uuid.UUID
	Kind     string
	Payload  []byte // JSON без служебных полей
	Attempts int    // сколько раз задание уже завершалось ошибкой
}

// Handler выполняет задание. Ошибка — повтор с задержкой или failed.
type Handler func(ctx context.Context, j Running) error

// HandlerOptions — особенности вида задания.
type HandlerOptions struct {
	// WipePayload затирает полезную нагрузку после завершения (успех или
	// исчерпание попыток): для заданий с секретами (R-1).
	WipePayload bool
}

type registration struct {
	handler Handler
	opts    HandlerOptions
}

// Worker берёт задания из таблицы и выполняет их обработчиками по виду.
type Worker struct {
	pool     *pgxpool.Pool
	clock    clock.Clock
	log      *slog.Logger
	name     string
	handlers map[string]registration
	batch    int

	mu      sync.Mutex
	running map[uuid.UUID]context.CancelFunc
	wg      sync.WaitGroup
}

// NewWorker создаёт обработчик. Имя экземпляра — хост и PID (locked_by).
func NewWorker(pool *pgxpool.Pool, clk clock.Clock, log *slog.Logger) *Worker {
	host, _ := os.Hostname()
	return &Worker{
		pool:     pool,
		clock:    clk,
		log:      log,
		name:     fmt.Sprintf("%s:%d", host, os.Getpid()),
		handlers: map[string]registration{},
		batch:    defaultBatch,
		running:  map[uuid.UUID]context.CancelFunc{},
	}
}

// Register связывает вид задания с обработчиком.
func (w *Worker) Register(kind string, h Handler, opts HandlerOptions) {
	w.handlers[kind] = registration{handler: h, opts: opts}
}

// Kinds — зарегистрированные виды.
func (w *Worker) Kinds() []string {
	return slices.Sorted(maps.Keys(w.handlers))
}

// Run обрабатывает задания, пока не отменён ctx. После отмены новые задания не
// берутся, текущие доделываются до shutdownDeadline, остальные возвращаются в очередь.
func (w *Worker) Run(ctx context.Context) error {
	w.log.InfoContext(ctx, "worker started", logx.KeyOp, "Job.Run", "locked_by", w.name, "kinds", w.Kinds())
	w.releaseAbandoned(ctx)
	poll := time.NewTicker(pollInterval)
	defer poll.Stop()
	abandon := time.NewTicker(abandonInterval)
	defer abandon.Stop()

	jobsCtx, cancelJobs := context.WithCancel(context.WithoutCancel(ctx))
	defer cancelJobs()
	for {
		select {
		case <-ctx.Done():
			w.shutdown(cancelJobs)
			return nil
		case <-abandon.C:
			w.releaseAbandoned(ctx)
		case <-poll.C:
			if _, err := w.claimAndStart(jobsCtx); err != nil && ctx.Err() == nil {
				w.log.ErrorContext(ctx, "job poll failed", logx.KeyOp, "Job.Run", logx.KeyError, err.Error())
			}
		}
	}
}

// RunOnce берёт готовые задания и выполняет их синхронно. Возвращает число
// выполненных. Нужен тестам: время задаётся clock.Manual.
func (w *Worker) RunOnce(ctx context.Context) (int, error) {
	n, err := w.claimAndStart(ctx)
	w.wg.Wait()
	return n, err
}

// ReleaseAbandoned возвращает в очередь брошенные задания (07 §2).
func (w *Worker) ReleaseAbandoned(ctx context.Context) int { return w.releaseAbandoned(ctx) }

func (w *Worker) claimAndStart(ctx context.Context) (int, error) {
	if len(w.handlers) == 0 {
		return 0, nil
	}
	w.mu.Lock()
	free := w.batch - len(w.running)
	w.mu.Unlock()
	if free <= 0 {
		return 0, nil
	}
	now := w.clock.Now()
	rows, err := sqlc.New(w.pool).ClaimJobs(ctx, sqlc.ClaimJobsParams{
		LockedBy: &w.name, Now: now, Kinds: w.Kinds(), Batch: int32(free),
	})
	if err != nil {
		return 0, fmt.Errorf("claim jobs: %w", err)
	}
	for _, r := range rows {
		jctx, cancel := context.WithTimeout(ctx, jobTimeout)
		w.mu.Lock()
		w.running[r.ID] = cancel
		w.mu.Unlock()
		w.wg.Add(1)
		go func() {
			defer w.wg.Done()
			defer w.finish(r.ID, cancel)
			w.execute(jctx, r)
		}()
	}
	return len(rows), nil
}

func (w *Worker) finish(id uuid.UUID, cancel context.CancelFunc) {
	cancel()
	w.mu.Lock()
	delete(w.running, id)
	w.mu.Unlock()
}

func (w *Worker) execute(ctx context.Context, r sqlc.ClaimJobsRow) {
	payload, requestID := decodePayload(r.Payload)
	if requestID != "" {
		ctx = logx.WithRequestID(ctx, requestID)
	}
	ctx = logx.With(ctx, logx.KeyJobID, r.ID.String())
	reg := w.handlers[r.Kind]
	attempt := int(r.Attempts) + 1
	start := w.clock.Now()
	w.log.InfoContext(ctx, "job started", logx.KeyOp, "Job.Execute", "kind", r.Kind, "attempt", attempt)

	err := safeCall(ctx, reg.handler, Running{ID: r.ID, Kind: r.Kind, Payload: payload, Attempts: int(r.Attempts)})
	// Итог пишется и после отмены context задания: до остановки процесса.
	dbctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	q := sqlc.New(w.pool)
	now := w.clock.Now()
	dur := now.Sub(start).Milliseconds()

	if err == nil {
		if _, uerr := q.CompleteJob(dbctx, sqlc.CompleteJobParams{
			ID: r.ID, LockedBy: &w.name, Now: now, WipePayload: reg.opts.WipePayload,
		}); uerr != nil {
			w.log.ErrorContext(ctx, "job state update failed", logx.KeyOp, "Job.Execute", logx.KeyError, uerr.Error())
			return
		}
		w.log.InfoContext(ctx, "job finished", logx.KeyOp, "Job.Execute", "kind", r.Kind, "attempt", attempt, "duration_ms", dur)
		return
	}

	attempts := r.Attempts + 1
	msg := err.Error()
	if attempts >= r.MaxAttempts {
		if _, uerr := q.FailJob(dbctx, sqlc.FailJobParams{
			ID: r.ID, LockedBy: &w.name, Now: now, Attempts: attempts, LastError: &msg, WipePayload: reg.opts.WipePayload,
		}); uerr != nil {
			w.log.ErrorContext(ctx, "job state update failed", logx.KeyOp, "Job.Execute", logx.KeyError, uerr.Error())
			return
		}
		w.log.ErrorContext(ctx, "job exhausted attempts", logx.KeyOp, "Job.Execute", "kind", r.Kind,
			"attempt", attempt, "max_attempts", r.MaxAttempts, "duration_ms", dur, logx.KeyError, msg)
		return
	}
	next := now.Add(RetryDelay(int(r.Attempts)))
	if _, uerr := q.RetryJob(dbctx, sqlc.RetryJobParams{
		ID: r.ID, LockedBy: &w.name, Now: now, Attempts: attempts, RunAt: next, LastError: &msg,
	}); uerr != nil {
		w.log.ErrorContext(ctx, "job state update failed", logx.KeyOp, "Job.Execute", logx.KeyError, uerr.Error())
		return
	}
	w.log.WarnContext(ctx, "job failed, will retry", logx.KeyOp, "Job.Execute", "kind", r.Kind,
		"attempt", attempt, "next_run_at", next, "duration_ms", dur, logx.KeyError, msg)
}

// RetryDelay — задержка перед повтором: min(30 с × 2^attempts, 1 ч), где
// attempts — число неудач до этой (07 §2).
func RetryDelay(attempts int) time.Duration {
	if attempts >= 7 { // 30 с × 2^7 больше часа
		return retryMax
	}
	return min(retryBase<<attempts, retryMax)
}

func safeCall(ctx context.Context, h Handler, r Running) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("job handler panic: %v", p)
		}
	}()
	return h(ctx, r)
}

func (w *Worker) releaseAbandoned(ctx context.Context) int {
	now := w.clock.Now()
	rows, err := sqlc.New(w.pool).ReleaseAbandonedJobs(ctx, sqlc.ReleaseAbandonedJobsParams{
		Now: now, LockedBefore: now.Add(-abandonedAfter),
	})
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			w.log.ErrorContext(ctx, "abandoned jobs check failed", logx.KeyOp, "Job.ReleaseAbandoned", logx.KeyError, err.Error())
		}
		return 0
	}
	for _, r := range rows {
		w.log.WarnContext(ctx, "abandoned job returned to queue", logx.KeyOp, "Job.ReleaseAbandoned",
			logx.KeyJobID, r.ID, "kind", r.Kind)
	}
	return len(rows)
}

// shutdown ждёт текущие задания до shutdownDeadline, затем отменяет их и
// возвращает в очередь.
func (w *Worker) shutdown(cancelJobs context.CancelFunc) {
	ctx := context.Background()
	w.log.InfoContext(ctx, "worker stopping", logx.KeyOp, "Job.Shutdown")
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		w.log.InfoContext(ctx, "worker stopped", logx.KeyOp, "Job.Shutdown")
		return
	case <-time.After(shutdownDeadline):
	}
	w.mu.Lock()
	ids := slices.Collect(maps.Keys(w.running))
	w.mu.Unlock()
	cancelJobs()
	<-done
	n, err := sqlc.New(w.pool).ReleaseJobs(ctx, sqlc.ReleaseJobsParams{Ids: ids, LockedBy: &w.name, Now: w.clock.Now()})
	if err != nil {
		w.log.ErrorContext(ctx, "jobs release failed", logx.KeyOp, "Job.Shutdown", logx.KeyError, err.Error())
		return
	}
	w.log.WarnContext(ctx, "unfinished jobs returned to queue", logx.KeyOp, "Job.Shutdown", "count", n)
}
