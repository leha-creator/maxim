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
