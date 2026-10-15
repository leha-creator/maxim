// Package logx настраивает структурированный журнал (07 §5.1) и переносит
// сквозной идентификатор запроса через context.
package logx

import (
	"context"
	"io"
	"log/slog"
	"strings"
)

type ctxKey struct{}

// New создаёт JSON-журнал с полем component=mockapi. Уровень — из строки
// LOG_LEVEL: debug, info, warn, error; пустое или неизвестное значение — debug.
func New(w io.Writer, level string) *slog.Logger {
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: ParseLevel(level),
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey && len(groups) == 0 {
				a.Value = slog.TimeValue(a.Value.Time().UTC()) // 07 §5.1: время в UTC
			}
			return a
		},
	})
	return slog.New(h).With("component", "mockapi")
}

// ParseLevel переводит значение LOG_LEVEL в уровень slog.
func ParseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelDebug
	}
}

// WithRequestID кладёт сквозной идентификатор в context.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// RequestID достаёт сквозной идентификатор; пусто, если его нет.
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}

// From возвращает журнал с полем request_id из context.
func From(ctx context.Context, base *slog.Logger) *slog.Logger {
	if id := RequestID(ctx); id != "" {
		return base.With("request_id", id)
	}
	return base
}
