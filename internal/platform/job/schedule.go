package job

import (
	"fmt"
	"hash/fnv"
	"time"
)

// msk — календарные расписания считаются по московскому времени (A-12), а
// хранятся в UTC.
var msk = time.FixedZone("MSK", 3*60*60)

// Schedule — расписание периодического задания.
type Schedule interface {
	// Next возвращает первое время запуска строго позже after.
	Next(after time.Time) time.Time
	String() string
}

type every time.Duration

// Every — запуск через каждые d.
func Every(d time.Duration) Schedule {
	if d <= 0 {
		panic("job.Every: interval must be positive")
	}
	return every(d)
}

func (e every) Next(after time.Time) time.Time { return after.Add(time.Duration(e)).UTC() }
func (e every) String() string                 { return "every " + time.Duration(e).String() }

type dailyAt struct{ hour, minute int }

// DailyAt — раз в сутки в hh:mm по Москве.
func DailyAt(hour, minute int) Schedule { return dailyAt{hour, minute} }

func (d dailyAt) Next(after time.Time) time.Time {
	a := after.In(msk)
	t := time.Date(a.Year(), a.Month(), a.Day(), d.hour, d.minute, 0, 0, msk)
	if !t.After(a) {
		t = t.AddDate(0, 0, 1)
	}
	return t.UTC()
}

func (d dailyAt) String() string { return fmt.Sprintf("daily at %02d:%02d MSK", d.hour, d.minute) }

type monthlyAt struct{ day, hour, minute int }

// MonthlyAt — раз в месяц в день day в hh:mm по Москве. Если в месяце меньше
// дней, запуск в последний день месяца.
func MonthlyAt(day, hour, minute int) Schedule {
	if day < 1 || day > 31 {
		panic("job.MonthlyAt: day must be 1..31")
	}
	return monthlyAt{day, hour, minute}
}

func (m monthlyAt) Next(after time.Time) time.Time {
	a := after.In(msk)
	for i := 0; ; i++ {
		first := time.Date(a.Year(), a.Month()+time.Month(i), 1, 0, 0, 0, 0, msk)
		day := min(m.day, daysIn(first))
		t := time.Date(first.Year(), first.Month(), day, m.hour, m.minute, 0, 0, msk)
		if t.After(a) {
			return t.UTC()
		}
	}
}

func (m monthlyAt) String() string {
	return fmt.Sprintf("monthly on day %d at %02d:%02d MSK", m.day, m.hour, m.minute)
}

func daysIn(firstOfMonth time.Time) int {
	return firstOfMonth.AddDate(0, 1, -1).Day()
}

type spread struct {
	inner  Schedule
	offset time.Duration
}

// Spread сдвигает расписание на постоянное для key смещение внутри window:
// запуски разных объектов распределяются по интервалу (07 §2, метрики каналов).
func Spread(s Schedule, key string, window time.Duration) Schedule {
	if window < time.Second {
		return s
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	offset := time.Duration(h.Sum64() % uint64(window/time.Second) * uint64(time.Second))
	return spread{inner: s, offset: offset}
}

func (s spread) Next(after time.Time) time.Time {
	return s.inner.Next(after.Add(-s.offset)).Add(s.offset)
}

func (s spread) String() string { return s.inner.String() + " +" + s.offset.String() }
