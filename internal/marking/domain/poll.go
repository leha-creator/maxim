package domain

import "time"

// PollRules — расписание опроса статуса ОРД (05 §3.5, R-10).
type PollRules struct {
	// FastInterval — интервал опроса в первые FastPeriod после отправки (ORD_POLL_FAST_INTERVAL).
	FastInterval time.Duration
	// FastPeriod — ORD_POLL_FAST_PERIOD.
	FastPeriod time.Duration
	// SlowInterval — интервал после FastPeriod (ORD_POLL_SLOW_INTERVAL).
	SlowInterval time.Duration
	// Timeout — без финального статуса дольше — error status_timeout (ORD_STATUS_TIMEOUT).
	Timeout time.Duration
}

// WithDefaults подставляет значения по умолчанию.
func (r PollRules) WithDefaults() PollRules {
	if r.FastInterval <= 0 {
		r.FastInterval = time.Minute
	}
	if r.FastPeriod <= 0 {
		r.FastPeriod = 10 * time.Minute
	}
	if r.SlowInterval <= 0 {
		r.SlowInterval = 10 * time.Minute
	}
	if r.Timeout <= 0 {
		r.Timeout = 24 * time.Hour
	}
	return r
}

// NextPoll — через сколько опросить статус снова; timeout — срок ожидания истёк (R-10).
// Следующий опрос не позже срока: последний опрос выполняется ровно в момент таймаута.
func NextPoll(sentAt, now time.Time, r PollRules) (delay time.Duration, timeout bool) {
	deadline := sentAt.Add(r.Timeout)
	if !now.Before(deadline) {
		return 0, true
	}
	delay = r.SlowInterval
	if now.Sub(sentAt) < r.FastPeriod {
		delay = r.FastInterval
	}
	return min(delay, deadline.Sub(now)), false
}
