// Package apperr — ошибки приложения с кодом из ErrorCode спецификации API.
// Слой HTTP переводит код в статус ответа; остальные слои о HTTP не знают.
package apperr

import (
	"errors"
	"fmt"
	"time"
)

// Code — машинный код ошибки, совпадает со значением ErrorCode в api/openapi.yaml.
type Code string

const (
	CodeValidationFailed  Code = "validation_failed"
	CodeUnauthorized      Code = "unauthorized"
	CodeForbidden         Code = "forbidden"
	CodeNotFound          Code = "not_found"
	CodeConflict          Code = "conflict"
	CodeInvalidTransition Code = "invalid_transition"
	CodeINNTaken          Code = "inn_taken"
	// Отказы оформления заказа (S-06, этап «Заказ и размещения»).
	CodeInsufficientBalance Code = "insufficient_balance"
	CodeCartChanged         Code = "cart_changed"
	CodeCampaignIncomplete  Code = "campaign_incomplete"
	CodeRateLimited         Code = "rate_limited"
	// CodeServiceUnavailable — внешний сервис не подключён или недоступен (503, этап выплат).
	CodeServiceUnavailable Code = "service_unavailable"
	CodeNotImplemented     Code = "not_implemented"
	CodeInternal           Code = "internal"
)

// Error — ошибка с кодом. Message — короткая постоянная строка на английском,
// без персональных данных: она уходит клиенту.
type Error struct {
	Code    Code
	Message string
	Details map[string]any
	// RetryAfter заполняется у rate_limited: через сколько можно повторить.
	RetryAfter time.Duration
	// cause — исходная ошибка для журнала; клиенту не показывается.
	cause error
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.cause }

// WithCause прикрепляет исходную ошибку.
func (e *Error) WithCause(err error) *Error {
	e.cause = err
	return e
}

// WithDetail добавляет поле в details.
func (e *Error) WithDetail(key string, value any) *Error {
	if e.Details == nil {
		e.Details = map[string]any{}
	}
	e.Details[key] = value
	return e
}

// New создаёт ошибку с кодом и сообщением.
func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// Validation — поле не прошло проверку; reason — машинная причина (`required`, `invalid_checksum`…).
func Validation(field, reason string) *Error {
	return &Error{
		Code:    CodeValidationFailed,
		Message: "request is invalid",
		Details: map[string]any{"field": field, "reason": reason},
	}
}

// Unauthorized — нет сессии, неверный или просроченный код.
func Unauthorized(message string) *Error { return New(CodeUnauthorized, message) }

// Forbidden — действие запрещено; reason попадает в details.reason.
func Forbidden(reason string) *Error {
	return New(CodeForbidden, "access denied").WithDetail("reason", reason)
}

// NotFound — объекта нет или он чужой.
func NotFound(message string) *Error { return New(CodeNotFound, message) }

// Conflict — действие невозможно в текущем состоянии данных.
func Conflict(message string) *Error { return New(CodeConflict, message) }

// InvalidTransition — переход недопустим из текущего состояния.
func InvalidTransition(transition, status string) *Error {
	return New(CodeInvalidTransition, "transition is not allowed from current status").
		WithDetail("transition", transition).WithDetail("status", status)
}

// INNTaken — ИНН уже принадлежит другой организации.
func INNTaken() *Error { return New(CodeINNTaken, "inn is already registered") }

// RateLimited — превышен лимит частоты; scope — `email`, `ip`, `user`.
func RateLimited(scope string, retryAfter time.Duration) *Error {
	e := New(CodeRateLimited, "too many requests").WithDetail("scope", scope)
	e.RetryAfter = retryAfter
	return e
}

// NotImplemented — операция описана в спецификации, но этап её ещё не реализует.
func NotImplemented(operation string) *Error {
	return New(CodeNotImplemented, "operation is not implemented yet").WithDetail("operation", operation)
}

// Internal оборачивает непредвиденную ошибку.
func Internal(err error) *Error {
	return New(CodeInternal, "internal error").WithCause(err)
}

// As достаёт *Error из цепочки ошибок.
func As(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// Is сообщает, что в цепочке есть *Error с кодом code.
func Is(err error, code Code) bool {
	e, ok := As(err)
	return ok && e.Code == code
}
