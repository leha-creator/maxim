package domain

import (
	"errors"
	"fmt"
)

var (
	// ErrNotFound — записи нет или она чужая.
	ErrNotFound = errors.New("payout: not found")
	// ErrInvalidPayout — данные выплаты не согласованы (сумма, способ, исполнитель).
	ErrInvalidPayout = errors.New("payout: invalid payout")
	// ErrInvalidPayoutState — переход попытки недопустим из её состояния.
	ErrInvalidPayoutState = errors.New("payout: invalid payout state")
	// ErrPayoutNotRetryable — повтор возможен только из последней попытки failed (R-9).
	ErrPayoutNotRetryable = errors.New("payout: payout is not retryable")
	// ErrSecondApproval — отметка выплаты выше порога без второго администратора (R-2).
	ErrSecondApproval = errors.New("payout: second senior admin approval is required")
	// ErrPayoutExists — у размещения уже есть живая выплата (04 §7).
	ErrPayoutExists = errors.New("payout: placement already has a live payout")
	// ErrContractorNotRequired — юрлицу исполнитель Рокет Ворк не нужен (A-36).
	ErrContractorNotRequired = errors.New("payout: rocketwork contractor is not required")
	// ErrConcurrentUpdate — строку изменили одновременно (lock_version).
	ErrConcurrentUpdate = errors.New("payout: concurrent update")
	// ErrPlacementState — размещение не в том состоянии для выплаты (R-5).
	ErrPlacementState = errors.New("payout: placement is not accepted")

	// ErrProviderDisabled — Рокет Ворк выключен (ROCKETWORK_MODE=disabled, R-1).
	ErrProviderDisabled = errors.New("rocketwork: disabled")
	// ErrProviderUnavailable — временная ошибка сервиса: повторить позже.
	ErrProviderUnavailable = errors.New("rocketwork: unavailable")
	// ErrProviderNotFound — объекта нет в сервисе.
	ErrProviderNotFound = errors.New("rocketwork: not found")
	// ErrWebhookAuth — подпись вебхука не прошла проверку (R-11).
	ErrWebhookAuth = errors.New("rocketwork: webhook authentication failed")
	// ErrWebhookFormat — тело вебхука не разобрано.
	ErrWebhookFormat = errors.New("rocketwork: webhook format error")
)

// RejectedError — сервис окончательно отказал в операции с нашим кодом (05 §4.3).
type RejectedError struct {
	Code    FailureCode
	Message string
}

func (e *RejectedError) Error() string {
	return fmt.Sprintf("rocketwork: rejected: %s: %s", e.Code, e.Message)
}

// AsRejected извлекает отказ сервиса.
func AsRejected(err error) (*RejectedError, bool) {
	var r *RejectedError
	if errors.As(err, &r) {
		return r, true
	}
	return nil, false
}
