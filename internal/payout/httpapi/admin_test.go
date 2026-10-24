package httpapi

import "testing"

func TestCSVSafe(t *testing.T) {
	cases := map[string]string{
		"":                  "",
		"ИП Иванов":         "ИП Иванов",
		"=HYPERLINK(\"x\")": "'=HYPERLINK(\"x\")",
		"+7 канал":          "'+7 канал",
		"-1":                "'-1",
		"@SUM(A1)":          "'@SUM(A1)",
		"\tскрыто":          "'\tскрыто",
		"\rскрыто":          "'\rскрыто",
		"п/п 12 = оплачено": "п/п 12 = оплачено",
	}
	for in, want := range cases {
		if got := csvSafe(in); got != want {
			t.Errorf("csvSafe(%q) = %q, want %q", in, got, want)
		}
	}
}
