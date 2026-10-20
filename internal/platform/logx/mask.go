package logx

import (
	"strings"
	"unicode/utf8"
)

// Маскирование персональных данных для журнала и данных аудита (07 §5.4).

// MaskEmail: `ivan@example.ru` → `i***@example.ru`.
func MaskEmail(email string) string {
	at := strings.LastIndexByte(email, '@')
	if at <= 0 {
		return "***"
	}
	r, _ := utf8.DecodeRuneInString(email)
	return string(r) + "***" + email[at:]
}

// MaskINN: первые 2 и последние 4 цифры, `7712345678` → `77****5678`.
func MaskINN(inn string) string { return maskMiddle(inn, 2, 4) }

// MaskPhone: `+79991234567` → `+7******4567`.
func MaskPhone(phone string) string { return maskMiddle(phone, 2, 4) }

// MaskAccount: последние 4 цифры счёта, `40802810000000000001` → `****************0001`.
func MaskAccount(account string) string { return maskMiddle(account, 0, 4) }

// MaskName: первая буква каждого слова, `Иванов Иван` → `И*** И***`.
func MaskName(name string) string {
	words := strings.Fields(name)
	for i, w := range words {
		r, _ := utf8.DecodeRuneInString(w)
		words[i] = string(r) + "***"
	}
	return strings.Join(words, " ")
}

func maskMiddle(s string, head, tail int) string {
	runes := []rune(s)
	if len(runes) <= head+tail {
		return strings.Repeat("*", len(runes))
	}
	return string(runes[:head]) + strings.Repeat("*", len(runes)-head-tail) + string(runes[len(runes)-tail:])
}
