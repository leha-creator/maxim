package clock

import (
	"testing"
	"time"
)

func TestManual(t *testing.T) {
	start := time.Date(2026, 10, 16, 12, 0, 0, 0, time.FixedZone("MSK", 3*3600))
	c := NewManual(start)
	if got := c.Now(); !got.Equal(start) || got.Location() != time.UTC {
		t.Fatalf("Now() = %v, want %v in UTC", got, start)
	}
	if got := c.Advance(90 * time.Second); !got.Equal(start.Add(90 * time.Second)) {
		t.Fatalf("Advance() = %v", got)
	}
	later := start.Add(time.Hour)
	c.Set(later)
	if got := c.Now(); !got.Equal(later) {
		t.Fatalf("after Set Now() = %v, want %v", got, later)
	}
}

func TestSystemIsUTC(t *testing.T) {
	if loc := (System{}).Now().Location(); loc != time.UTC {
		t.Fatalf("System.Now() location = %v, want UTC", loc)
	}
}
