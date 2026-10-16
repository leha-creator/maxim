// Package money — денежные суммы в копейках (README «Деньги», 06 §3).
// Дробные типы для денег запрещены.
package money

// Kopecks — сумма в копейках.
type Kopecks int64

// FullBP — 100 % в базисных пунктах.
const FullBP = 10000

// ShareBP — доля bp базисных пунктов от суммы с округлением вниз (06 §3).
// Сумма и доля неотрицательны.
func (k Kopecks) ShareBP(bp int) Kopecks {
	return Kopecks(int64(k) * int64(bp) / FullBP)
}
