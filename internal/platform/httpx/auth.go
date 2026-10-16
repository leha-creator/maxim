package httpx

import (
	"context"
	"log/slog"
	"net/http"
	"slices"

	"github.com/google/uuid"

	"github.com/leha-creator/maxim/internal/platform/apperr"
	"github.com/leha-creator/maxim/internal/platform/audit"
	"github.com/leha-creator/maxim/internal/platform/logx"
)

// Имена cookie (07 §4, R-4).
const (
	SessionCookie = "session"
	CSRFCookie    = "csrf_token"
	CSRFHeader    = "X-CSRF-Token"
)

// Principal — пользователь запроса. Модули получают его из context.
type Principal struct {
	UserID         uuid.UUID
	SessionID      uuid.UUID
	OrganizationID *uuid.UUID
	Cabinets       []string
	AdminRole      string
}

// Authenticator находит пользователя по токену сессии. Ошибка с кодом
// unauthorized — сессии нет; другая ошибка — сбой.
type Authenticator func(ctx context.Context, token string) (*Principal, error)

type principalKey struct{}

// PrincipalFrom возвращает пользователя запроса.
func PrincipalFrom(ctx context.Context) (*Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(*Principal)
	return p, ok
}

// WithPrincipal кладёт пользователя в context (для тестов и служебных вызовов).
func WithPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// Session читает cookie сессии и кладёт пользователя в context, поля журнала
// и действующее лицо журнала действий. Без сессии запрос идёт дальше: требование
// сессии проверяет слой спецификации (security), чтобы открытые операции работали.
func Session(log *slog.Logger, authn Authenticator) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie(SessionCookie)
			if err != nil || c.Value == "" {
				next.ServeHTTP(w, r)
				return
			}
			ctx := r.Context()
			p, err := authn(ctx, c.Value)
			if err != nil {
				if apperr.Is(err, apperr.CodeUnauthorized) {
					log.DebugContext(ctx, "session rejected", logx.KeyOp, "HTTP.Session")
					next.ServeHTTP(w, r)
					return
				}
				WriteError(w, r, log, err)
				return
			}
			ctx = WithPrincipal(ctx, p)
			ctx = logx.With(ctx, logx.KeyUserID, p.UserID.String())
			if p.OrganizationID != nil {
				ctx = logx.With(ctx, logx.KeyOrganizationID, p.OrganizationID.String())
			}
			actor := audit.ActorFrom(ctx)
			actor.UserID, actor.OrganizationID = &p.UserID, p.OrganizationID
			ctx = audit.WithActor(ctx, actor)
			log.DebugContext(ctx, "principal resolved", logx.KeyOp, "HTTP.Session", "cabinets", p.Cabinets)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Authenticated сообщает, что у запроса есть пользователь.
func Authenticated(ctx context.Context) bool {
	_, ok := PrincipalFrom(ctx)
	return ok
}

// ReasonCabinetRequired — у организации нет нужного кабинета.
const ReasonCabinetRequired = "cabinet_required"

// RequireCabinet проверяет, что у организации пользователя открыт кабинет.
func RequireCabinet(ctx context.Context, cabinet string) (*Principal, error) {
	p, ok := PrincipalFrom(ctx)
	if !ok {
		return nil, apperr.Unauthorized("session is missing")
	}
	if !slices.Contains(p.Cabinets, cabinet) {
		return nil, apperr.Forbidden(ReasonCabinetRequired).WithDetail("cabinet", cabinet)
	}
	return p, nil
}
