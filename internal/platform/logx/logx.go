// Package logx — структурированный журнал приложения (07 §5.1).
//
// Записи — JSON в одну строку. Поля, известные из context (request_id,
// user_id, organization_id, job_id), добавляются обработчиком автоматически:
// достаточно вызывать методы *Context (InfoContext, WarnContext…).
package logx

import (
	"context"
	"io"
	"log/slog"
	"strings"
)

// Имена полей журнала (07 §5.1).
const (
	KeyComponent      = "component"
	KeyModule         = "module"
	KeyOp             = "op"
	KeyRequestID      = "request_id"
	KeyUserID         = "user_id"
	KeyOrganizationID = "organization_id"
	KeyJobID          = "job_id"
	KeyError          = "error"
)

type attrsKey struct{}

type requestIDKey struct{}

// New создаёт JSON-журнал. level — значение LOG_LEVEL; component — `api`,
// `worker`, `migrate`, `cli`.
func New(w io.Writer, level slog.Leveler, component string) *slog.Logger {
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey && len(groups) == 0 {
				a.Value = slog.TimeValue(a.Value.Time().UTC()) // 07 §5.1: время в UTC
			}
			return a
		},
	})
	return slog.New(NewContextHandler(h)).With(KeyComponent, component)
}

// ParseLevel переводит значение LOG_LEVEL в уровень. Пустое значение — def.
// Второй результат false, если значение не распознано.
func ParseLevel(s string, def slog.Level) (slog.Level, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return def, true
	case "debug":
		return slog.LevelDebug, true
	case "info":
		return slog.LevelInfo, true
	case "warn":
		return slog.LevelWarn, true
	case "error":
		return slog.LevelError, true
	default:
		return def, false
	}
}

// ContextHandler добавляет к записи поля, сохранённые в context через With.
type ContextHandler struct {
	next slog.Handler
}

// NewContextHandler оборачивает обработчик.
func NewContextHandler(next slog.Handler) *ContextHandler { return &ContextHandler{next: next} }

func (h *ContextHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.next.Enabled(ctx, l)
}

func (h *ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	if ctx != nil {
		if id, ok := ctx.Value(requestIDKey{}).(string); ok && id != "" {
			r.AddAttrs(slog.String(KeyRequestID, id))
		}
		if attrs, ok := ctx.Value(attrsKey{}).([]slog.Attr); ok {
			r.AddAttrs(attrs...)
		}
	}
	return h.next.Handle(ctx, r)
}

func (h *ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ContextHandler{next: h.next.WithAttrs(attrs)}
}

func (h *ContextHandler) WithGroup(name string) slog.Handler {
	return &ContextHandler{next: h.next.WithGroup(name)}
}

// WithRequestID кладёт сквозной идентификатор в context.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestID возвращает сквозной идентификатор; пусто, если его нет.
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// With добавляет поля, которые попадут во все записи с этим context.
// Значение с тем же ключом заменяет прежнее.
func With(ctx context.Context, args ...any) context.Context {
	prev, _ := ctx.Value(attrsKey{}).([]slog.Attr)
	added := argsToAttrs(args)
	merged := make([]slog.Attr, 0, len(prev)+len(added))
	for _, p := range prev {
		if !hasKey(added, p.Key) {
			merged = append(merged, p)
		}
	}
	merged = append(merged, added...)
	return context.WithValue(ctx, attrsKey{}, merged)
}

func argsToAttrs(args []any) []slog.Attr {
	var r slog.Record
	r.Add(args...)
	out := make([]slog.Attr, 0, r.NumAttrs())
	r.Attrs(func(a slog.Attr) bool {
		out = append(out, a)
		return true
	})
	return out
}

func hasKey(attrs []slog.Attr, key string) bool {
	for _, a := range attrs {
		if a.Key == key {
			return true
		}
	}
	return false
}

// Discard — журнал, который ничего не пишет (для тестов).
func Discard() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}
