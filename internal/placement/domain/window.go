package domain

import "time"

// msk — часовой пояс периодов кампании (A-39): Москва без перехода на летнее время.
var msk = time.FixedZone("MSK", 3*60*60)

// TodayMSK — сегодняшний день по Москве (полночь UTC той же даты, как post/domain.Date).
func TodayMSK(now time.Time) time.Time {
	n := now.In(msk)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
}

// Window — окно размещения (R-9): период дат [Start, End) и период времени суток
// [TimeFrom, TimeTo) по Москве. Снимок кампании, не меняется после оформления.
type Window struct {
	Start    time.Time
	End      time.Time
	TimeFrom TimeOfDay
	TimeTo   TimeOfDay
}

// NewWindow — окно по датам кампании: Start — 00:00 МСК дня dateFrom, End — 00:00 МСК дня
// после dateTo (конец последнего дня). Даты — календарные дни (полночь UTC той же даты).
func NewWindow(dateFrom, dateTo time.Time, timeFrom, timeTo TimeOfDay) Window {
	return Window{
		Start:    time.Date(dateFrom.Year(), dateFrom.Month(), dateFrom.Day(), 0, 0, 0, 0, msk).UTC(),
		End:      time.Date(dateTo.Year(), dateTo.Month(), dateTo.Day()+1, 0, 0, 0, 0, msk).UTC(),
		TimeFrom: timeFrom,
		TimeTo:   timeTo,
	}
}

// sinceMidnightMSK — сколько прошло от полуночи по Москве.
func sinceMidnightMSK(t time.Time) time.Duration {
	m := t.In(msk)
	return m.Sub(time.Date(m.Year(), m.Month(), m.Day(), 0, 0, 0, 0, msk))
}

// InTimeOfDay — время суток t по Москве в [TimeFrom, TimeTo); TimeTo не входит.
func (w Window) InTimeOfDay(t time.Time) bool {
	d := sinceMidnightMSK(t)
	return d >= w.TimeFrom.Duration() && d < w.TimeTo.Duration()
}

// InDates — t внутри периода дат [Start, End).
func (w Window) InDates(t time.Time) bool { return !t.Before(w.Start) && t.Before(w.End) }

// Contains — t внутри периода дат и периода времени суток (I-07).
func (w Window) Contains(t time.Time) bool { return w.InDates(t) && w.InTimeOfDay(t) }

// LastSlot — начало последнего допустимого слота: TimeTo − step последнего дня окна (R-10).
func (w Window) LastSlot(step time.Duration) time.Time {
	lastDay := w.End.Add(-24 * time.Hour)
	return lastDay.Add(w.TimeTo.Duration() - step)
}
