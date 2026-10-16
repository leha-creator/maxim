package domain

// Причины отказа в ИНН (details.reason).
const (
	ReasonInvalidLength   = "invalid_length"
	ReasonInvalidChecksum = "invalid_checksum"
)

var (
	innWeights10 = []int{2, 4, 10, 3, 5, 9, 4, 6, 8}
	innWeights11 = []int{7, 2, 4, 10, 3, 5, 9, 4, 6, 8}
	innWeights12 = []int{3, 7, 2, 4, 10, 3, 5, 9, 4, 6, 8}
)

// CheckINN проверяет ИНН для типа лица (04 §2): 10 цифр у юрлица, 12 у ИП и
// самозанятого, контрольные числа по алгоритму ФНС. Возвращает причину отказа
// или пустую строку.
func CheckINN(inn string, t LegalType) string {
	want := 12
	if t == LegalEntity {
		want = 10
	}
	if len(inn) != want {
		return ReasonInvalidLength
	}
	d := make([]int, len(inn))
	for i := range len(inn) {
		c := inn[i]
		if c < '0' || c > '9' {
			return ReasonInvalidLength
		}
		d[i] = int(c - '0')
	}
	if want == 10 {
		if innCheckDigit(d, innWeights10) != d[9] {
			return ReasonInvalidChecksum
		}
		return ""
	}
	if innCheckDigit(d, innWeights11) != d[10] || innCheckDigit(d, innWeights12) != d[11] {
		return ReasonInvalidChecksum
	}
	return ""
}

func innCheckDigit(d, weights []int) int {
	sum := 0
	for i, w := range weights {
		sum += d[i] * w
	}
	return sum % 11 % 10
}
