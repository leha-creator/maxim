package job

import (
	"errors"
	"time"
)

// Unlimited — предел попыток для видов заданий без предела («—» в 07 §2):
// срок задаёт сам обработчик через Retry и Permanent (R-9).
const Unlimited = 1_000_000

type retryError struct {
	err   error
	after time.Duration
}

func (e *retryError) Error() string { return e.err.Error() }
func (e *retryError) Unwrap() error { return e.err }

type permanentError struct{ err error }

func (e *permanentError) Error() string { return e.err.Error() }
func (e *permanentError) Unwrap() error { return e.err }

// Retry просит повторить задание через after вместо обычной задержки
// min(30 с × 2^attempts, 1 ч). Попытка засчитывается: после max_attempts — failed.
func Retry(err error, after time.Duration) error {
	if err == nil {
		err = errors.New("retry requested")
	}
	return &retryError{err: err, after: max(after, 0)}
}

// Permanent завершает задание как failed без повторов: ошибка данных, которую
// повтор не исправит (например, ответ 4xx внешней системы, 05 §1).
func Permanent(err error) error {
	if err == nil {
		err = errors.New("permanent failure")
	}
	return &permanentError{err: err}
}

type postponeError struct {
	err   error
	after time.Duration
}

func (e *postponeError) Error() string { return e.err.Error() }
func (e *postponeError) Unwrap() error { return e.err }

// Postpone откладывает задание на after без счёта попытки: обработчик ждёт внешнего условия,
// а не повторяет сбой (зависимость ещё не готова, следующий опрос статуса). Предел
// max_attempts на такие ожидания не тратится; журнал — DEBUG, не WARN.
func Postpone(reason error, after time.Duration) error {
	if reason == nil {
		reason = errors.New("postponed")
	}
	return &postponeError{err: reason, after: max(after, 0)}
}

// PostponedFor возвращает задержку, запрошенную через Postpone.
func PostponedFor(err error) (time.Duration, bool) {
	var p *postponeError
	if errors.As(err, &p) {
		return p.after, true
	}
	return 0, false
}

// RetryAfter возвращает задержку, запрошенную через Retry.
func RetryAfter(err error) (time.Duration, bool) {
	var r *retryError
	if errors.As(err, &r) {
		return r.after, true
	}
	return 0, false
}

// IsPermanent сообщает, что err помечена Permanent.
func IsPermanent(err error) bool {
	var p *permanentError
	return errors.As(err, &p)
}
