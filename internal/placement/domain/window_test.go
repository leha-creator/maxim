package domain

import (
	"testing"
	"time"
)

func date(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

// mskTime — момент по Москве.
func mskTime(y int, m time.Month, d, hh, mm int) time.Time {
	return time.Date(y, m, d, hh, mm, 0, 0, msk).UTC()
}

// testWindow — кампания 20–22 октября, 09:00–21:00 МСК.
func testWindow() Window { return NewWindow(date(2026, 10, 20), date(2026, 10, 22), 9*60, 21*60) }

func TestNewWindowBoundaries(t *testing.T) {
	w := testWindow()
	if want := time.Date(2026, 10, 19, 21, 0, 0, 0, time.UTC); !w.Start.Equal(want) {
		t.Fatalf("start %s, want %s", w.Start, want)
	}
	if want := time.Date(2026, 10, 22, 21, 0, 0, 0, time.UTC); !w.End.Equal(want) {
		t.Fatalf("end %s, want %s (00:00 MSK after date_to)", w.End, want)
	}
	tests := []struct {
		name    string
		at      time.Time
		inDates bool
		inTime  bool
	}{
		{"23:59 MSK before first day", mskTime(2026, 10, 19, 23, 59), false, false},
		{"00:00 MSK first day", mskTime(2026, 10, 20, 0, 0), true, false},
		{"08:59 MSK", mskTime(2026, 10, 20, 8, 59), true, false},
		{"09:00 MSK time_from", mskTime(2026, 10, 20, 9, 0), true, true},
		{"20:59 MSK", mskTime(2026, 10, 21, 20, 59), true, true},
		{"21:00 MSK time_to excluded", mskTime(2026, 10, 21, 21, 0), true, false},
		{"23:59 MSK last day", mskTime(2026, 10, 22, 23, 59), true, false},
		{"00:00 MSK after last day", mskTime(2026, 10, 23, 0, 0), false, false},
		{"10:00 MSK after last day", mskTime(2026, 10, 23, 10, 0), false, true},
	}
	for _, tc := range tests {
		if got := w.InDates(tc.at); got != tc.inDates {
			t.Errorf("%s: in dates %t", tc.name, got)
		}
		if got := w.InTimeOfDay(tc.at); got != tc.inTime {
			t.Errorf("%s: in time %t", tc.name, got)
		}
		if got := w.Contains(tc.at); got != (tc.inDates && tc.inTime) {
			t.Errorf("%s: contains %t", tc.name, got)
		}
	}
	if got, want := w.LastSlot(5*time.Minute), mskTime(2026, 10, 22, 20, 55); !got.Equal(want) {
		t.Errorf("last slot %s, want %s", got, want)
	}
}

func TestTodayMSK(t *testing.T) {
	if got := TodayMSK(time.Date(2026, 10, 19, 21, 30, 0, 0, time.UTC)); !got.Equal(date(2026, 10, 20)) {
		t.Errorf("21:30 UTC is next day in Moscow: %s", got)
	}
	if got := TodayMSK(time.Date(2026, 10, 19, 20, 59, 0, 0, time.UTC)); !got.Equal(date(2026, 10, 19)) {
		t.Errorf("20:59 UTC: %s", got)
	}
}

func TestParseTimeOfDay(t *testing.T) {
	v, err := ParseTimeOfDay("09:05")
	if err != nil || v != 9*60+5 || v.String() != "09:05" {
		t.Fatalf("parse: %v %v", v, err)
	}
	if _, err := ParseTimeOfDay("25:00"); err == nil {
		t.Fatal("invalid time accepted")
	}
}
