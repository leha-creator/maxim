package money

import (
	"errors"
	"strings"
)

// ErrFormat — строка не сумма в рублях с копейками.
var ErrFormat = errors.New("money: invalid amount")

// maxRubles — предел целой части: сумма в копейках помещается в int64 с запасом.
const maxRubles = 1_000_000_000_000

// Parse разбирает сумму в рублях без float: «1234», «1234.5», «1234,56», «1 234.56».
// Больше двух знаков после запятой, знак и пустая строка — ErrFormat.
func Parse(s string) (Kopecks, error) {
	s = strings.NewReplacer(" ", "", " ", "").Replace(strings.TrimSpace(s))
	whole, frac, dot := strings.Cut(strings.Replace(s, ",", ".", 1), ".")
	if whole == "" || (dot && (frac == "" || len(frac) > 2)) {
		return 0, ErrFormat
	}
	var rub int64
	for _, c := range whole {
		if c < '0' || c > '9' {
			return 0, ErrFormat
		}
		if rub = rub*10 + int64(c-'0'); rub > maxRubles {
			return 0, ErrFormat
		}
	}
	var kop int64
	for i := range 2 {
		kop *= 10
		if i < len(frac) {
			c := frac[i]
			if c < '0' || c > '9' {
				return 0, ErrFormat
			}
			kop += int64(c - '0')
		}
	}
	return Kopecks(rub*100 + kop), nil
}
