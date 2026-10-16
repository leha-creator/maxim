package domain

import (
	"time"

	"github.com/google/uuid"
)

// Причины отказа в интервале недоступности (R-6).
var (
	ErrUnavailabilityInPast  = &RuleError{Field: "ends_at", Reason: "in_past"}
	ErrUnavailabilityTooLong = &RuleError{Field: "ends_at", Reason: "too_long"}
	ErrUnavailabilityTooFar  = &RuleError{Field: "starts_at", Reason: "too_far"}
	ErrUnavailabilityOverlap = &RuleError{Field: "starts_at", Reason: "overlap"}
	ErrUnavailabilityTooMany = &RuleError{Field: "starts_at", Reason: "too_many"}
)

// Unavailability — интервал, в который блогер не принимает публикации (S-13).
type Unavailability struct {
	ID       uuid.UUID
	StartsAt time.Time
	EndsAt   time.Time
}

// Live — интервал ещё не закончился: такие показываются и учитываются (R-6).
func (u Unavailability) Live(now time.Time) bool { return u.EndsAt.After(now) }

// overlaps — интервалы пересекаются; соприкосновение границ — не пересечение.
func (u Unavailability) overlaps(o Unavailability) bool {
	return u.StartsAt.Before(o.EndsAt) && o.StartsAt.Before(u.EndsAt)
}

// UnavailabilityLimits — пределы интервалов (R-6, 07 §8).
type UnavailabilityLimits struct {
	// MaxLength — наибольшая длина интервала (CHANNEL_UNAVAILABILITY_MAX_DAYS, 90 суток).
	MaxLength time.Duration
	// Horizon — насколько вперёд может начинаться интервал (CHANNEL_UNAVAILABILITY_HORIZON_DAYS, 365 суток).
	Horizon time.Duration
	// MaxCount — сколько живых интервалов у канала (CHANNEL_UNAVAILABILITY_MAX_COUNT, 50).
	MaxCount int
}

// WithDefaults — пределы с значениями по умолчанию вместо нулевых.
func (l UnavailabilityLimits) WithDefaults() UnavailabilityLimits {
	if l.MaxLength == 0 {
		l.MaxLength = 90 * 24 * time.Hour
	}
	if l.Horizon == 0 {
		l.Horizon = 365 * 24 * time.Hour
	}
	if l.MaxCount == 0 {
		l.MaxCount = 50
	}
	return l
}

// ValidateUnavailability проверяет новый интервал против живых интервалов канала (R-6).
// Проверка против размещений — порт PlacementSchedule в этапе «Заказ и размещения» (R-8).
func ValidateUnavailability(u Unavailability, existing []Unavailability, now time.Time, l UnavailabilityLimits) error {
	l = l.WithDefaults()
	if !u.StartsAt.Before(u.EndsAt) || !u.Live(now) {
		return ErrUnavailabilityInPast
	}
	if u.EndsAt.Sub(u.StartsAt) > l.MaxLength {
		return ErrUnavailabilityTooLong
	}
	if u.StartsAt.After(now.Add(l.Horizon)) {
		return ErrUnavailabilityTooFar
	}
	live := 0
	for _, e := range existing {
		if !e.Live(now) {
			continue
		}
		live++
		if u.overlaps(e) {
			return ErrUnavailabilityOverlap
		}
	}
	if live >= l.MaxCount {
		return ErrUnavailabilityTooMany
	}
	return nil
}
