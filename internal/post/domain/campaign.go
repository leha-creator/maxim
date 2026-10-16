package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// msk — часовой пояс периодов кампании (A-39): Москва без перехода на летнее время.
var msk = time.FixedZone("MSK", 3*60*60)

// CampaignStatus — состояние кампании (04 §9).
type CampaignStatus string

const (
	CampaignDraft    CampaignStatus = "draft"
	CampaignOrdered  CampaignStatus = "ordered"
	CampaignArchived CampaignStatus = "archived"
)

// Пределы формы кампании (R-17).
const (
	MaxCampaignNameLength   = 200
	MaxExtraRequirements    = 2000
	MaxCampaignPeriodDays   = 14
	MinCampaignWindowMinute = 60
)

// Причины отказа в форме кампании (R-17, details.reason).
const (
	ReasonRequired      = "required"
	ReasonTooLong       = "too_long"
	ReasonPeriodOrder   = "period_order"
	ReasonPeriodTooLong = "period_too_long"
	ReasonPeriodInPast  = "period_in_past"
	ReasonTimePair      = "time_pair"
	ReasonTimeWindow    = "time_window"
)

// ErrNotDraft — период и требования меняются только в draft (R-17, 409).
var ErrNotDraft = errors.New("post: campaign is not a draft")

// TimeOfDay — время суток по Москве в минутах от полуночи.
type TimeOfDay int

// ParseTimeOfDay разбирает `ЧЧ:ММ`.
func ParseTimeOfDay(s string) (TimeOfDay, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, fmt.Errorf("time of day %q: %w", s, err)
	}
	return TimeOfDay(t.Hour()*60 + t.Minute()), nil
}

// String — `ЧЧ:ММ`.
func (t TimeOfDay) String() string { return fmt.Sprintf("%02d:%02d", int(t)/60, int(t)%60) }

// Date — календарный день (полночь UTC той же даты).
type Date = time.Time

// NewDate — день по году, месяцу и числу.
func NewDate(y int, m time.Month, d int) Date { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

// TodayMSK — сегодняшний день по Москве.
func TodayMSK(now time.Time) Date {
	n := now.In(msk)
	return NewDate(n.Year(), n.Month(), n.Day())
}

// Campaign — кампания с единым постом (04 §4, D-16).
type Campaign struct {
	ID                uuid.UUID
	OrganizationID    uuid.UUID
	Name              string
	Status            CampaignStatus
	DateFrom, DateTo  *Date
	TimeFrom, TimeTo  *TimeOfDay
	ExtraRequirements *string
	CreatedBy         uuid.UUID
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// CampaignInput — поля формы; nil — поле не передано (не меняется).
type CampaignInput struct {
	Name              *string
	DateFrom, DateTo  *Date
	TimeFrom, TimeTo  *TimeOfDay
	ExtraRequirements *string
}

// NewCampaign создаёт кампанию в draft (R-17). Обязательно только название.
func NewCampaign(id, orgID, userID uuid.UUID, in CampaignInput, now time.Time) (Campaign, error) {
	if in.Name == nil {
		return Campaign{}, &RuleError{Field: "name", Reason: ReasonRequired}
	}
	c := Campaign{ID: id, OrganizationID: orgID, Status: CampaignDraft, CreatedBy: userID, CreatedAt: now, UpdatedAt: now}
	if _, err := c.apply(in, now); err != nil {
		return Campaign{}, err
	}
	return c, nil
}

// Update меняет переданные поля (R-17) и возвращает имена изменённых полей.
// Период и требования вне draft — ErrNotDraft; название меняется всегда.
func (c *Campaign) Update(in CampaignInput, now time.Time) ([]string, error) {
	if c.Status != CampaignDraft && (in.DateFrom != nil || in.DateTo != nil || in.TimeFrom != nil ||
		in.TimeTo != nil || in.ExtraRequirements != nil) {
		return nil, ErrNotDraft
	}
	next := *c
	changed, err := next.apply(in, now)
	if err != nil {
		return nil, err
	}
	if len(changed) > 0 {
		next.UpdatedAt = now
	}
	*c = next
	return changed, nil
}

// apply переносит поля в кампанию и проверяет итог.
func (c *Campaign) apply(in CampaignInput, now time.Time) ([]string, error) {
	var changed []string
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		switch n := utf8.RuneCountInString(name); {
		case n == 0:
			return nil, &RuleError{Field: "name", Reason: ReasonRequired}
		case n > MaxCampaignNameLength:
			return nil, &RuleError{Field: "name", Reason: ReasonTooLong}
		}
		if name != c.Name {
			c.Name = name
			changed = append(changed, "name")
		}
	}
	if in.ExtraRequirements != nil {
		if utf8.RuneCountInString(*in.ExtraRequirements) > MaxExtraRequirements {
			return nil, &RuleError{Field: "extra_requirements", Reason: ReasonTooLong}
		}
		if c.ExtraRequirements == nil || *c.ExtraRequirements != *in.ExtraRequirements {
			v := *in.ExtraRequirements
			c.ExtraRequirements = &v
			changed = append(changed, "extra_requirements")
		}
	}
	datesChanged := false
	if in.DateFrom != nil && (c.DateFrom == nil || !c.DateFrom.Equal(*in.DateFrom)) {
		v := *in.DateFrom
		c.DateFrom, datesChanged = &v, true
		changed = append(changed, "date_from")
	}
	if in.DateTo != nil && (c.DateTo == nil || !c.DateTo.Equal(*in.DateTo)) {
		v := *in.DateTo
		c.DateTo, datesChanged = &v, true
		changed = append(changed, "date_to")
	}
	if in.TimeFrom != nil && (c.TimeFrom == nil || *c.TimeFrom != *in.TimeFrom) {
		v := *in.TimeFrom
		c.TimeFrom = &v
		changed = append(changed, "time_from")
	}
	if in.TimeTo != nil && (c.TimeTo == nil || *c.TimeTo != *in.TimeTo) {
		v := *in.TimeTo
		c.TimeTo = &v
		changed = append(changed, "time_to")
	}
	if err := c.checkPeriod(datesChanged, now); err != nil {
		return nil, err
	}
	return changed, nil
}

// checkPeriod проверяет период дат и окно времени (R-17). «Прошлое» проверяется,
// только когда даты меняются: переименование старой кампании не отклоняется.
func (c *Campaign) checkPeriod(datesChanged bool, now time.Time) error {
	if c.DateFrom != nil && c.DateTo != nil {
		if c.DateFrom.After(*c.DateTo) {
			return &RuleError{Field: "date_from", Reason: ReasonPeriodOrder}
		}
		if days := int(c.DateTo.Sub(*c.DateFrom).Hours()/24) + 1; days > MaxCampaignPeriodDays {
			return &RuleError{Field: "date_to", Reason: ReasonPeriodTooLong}
		}
	}
	if datesChanged {
		today := TodayMSK(now)
		if (c.DateFrom != nil && c.DateFrom.Before(today)) || (c.DateFrom == nil && c.DateTo != nil && c.DateTo.Before(today)) {
			return &RuleError{Field: "date_from", Reason: ReasonPeriodInPast}
		}
	}
	if (c.TimeFrom == nil) != (c.TimeTo == nil) {
		return &RuleError{Field: "time_from", Reason: ReasonTimePair}
	}
	if c.TimeFrom != nil && int(*c.TimeTo-*c.TimeFrom) < MinCampaignWindowMinute {
		return &RuleError{Field: "time_from", Reason: ReasonTimeWindow}
	}
	return nil
}
