package money

import (
	"errors"
	"testing"
)

func TestParse(t *testing.T) {
	for in, want := range map[string]Kopecks{
		"0": 0, "1234": 123400, "1234.5": 123450, "1234,56": 123456, "1 234.56": 123456, " 0.01 ": 1,
		"100000.00": 10000000, "1 000": 100000,
	} {
		got, err := Parse(in)
		if err != nil || got != want {
			t.Errorf("Parse(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"", ".5", "1.", "1.234", "-1", "+1", "1e3", "1.2.3", "12a", "1,2,3", "10000000000000"} {
		if _, err := Parse(in); !errors.Is(err, ErrFormat) {
			t.Errorf("Parse(%q): %v", in, err)
		}
	}
}
