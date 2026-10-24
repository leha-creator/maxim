package render

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// msk — московское время в текстах (README «Время»).
var msk = time.FixedZone("MSK", 3*60*60)

// funcs — функции шаблонов уведомлений.
func funcs() map[string]any {
	return map[string]any{
		"rub":     rub,
		"msk":     mskTime,
		"num":     num,
		"has":     has,
		"plural":  plural,
		"channel": channelName,
		"order":   orderNo,
	}
}

// has — значение есть и не пустое (в шаблонах вместо `if`, который падает на отсутствующем ключе).
func has(m map[string]any, key string) bool {
	v, ok := m[key]
	if !ok || v == nil {
		return false
	}
	if s, ok := v.(string); ok {
		return s != ""
	}
	return true
}

// toInt64 — число из данных уведомления: после jsonb числа приходят как json.Number или float64.
func toInt64(v any) (int64, bool) {
	switch x := v.(type) {
	case int:
		return int64(x), true
	case int64:
		return x, true
	case float64:
		return int64(math.Round(x)), true
	case json.Number:
		n, err := x.Int64()
		return n, err == nil
	case string:
		n, err := strconv.ParseInt(x, 10, 64)
		return n, err == nil
	}
	return 0, false
}

// rub — копейки в рубли: «11 500 ₽» или «11 500,50 ₽».
func rub(v any) string {
	k, ok := toInt64(v)
	if !ok {
		return ""
	}
	sign := ""
	if k < 0 {
		sign, k = "−", -k
	}
	s := group(k / 100)
	if c := k % 100; c != 0 {
		s += fmt.Sprintf(",%02d", c)
	}
	return sign + s + " ₽"
}

// num — целое с пробелами между разрядами: «12 345».
func num(v any) string {
	n, ok := toInt64(v)
	if !ok {
		return ""
	}
	if n < 0 {
		return "−" + group(-n)
	}
	return group(n)
}

func group(n int64) string {
	s := strconv.FormatInt(n, 10)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteRune(0x00a0) // неразрывный пробел
		}
		b.WriteRune(r)
	}
	return b.String()
}

// mskTime — время RFC 3339 (или time.Time) по Москве: «24.10.2026 15:00».
func mskTime(v any) string {
	var t time.Time
	switch x := v.(type) {
	case time.Time:
		t = x
	case *time.Time:
		if x == nil {
			return ""
		}
		t = *x
	case string:
		p, err := time.Parse(time.RFC3339Nano, x)
		if err != nil {
			return x
		}
		t = p
	default:
		return ""
	}
	return t.In(msk).Format("02.01.2006 15:04")
}

// plural — форма слова по числу: plural n "просмотр" "просмотра" "просмотров".
func plural(v any, one, few, many string) string {
	n, _ := toInt64(v)
	n %= 100
	if n < 0 {
		n = -n
	}
	switch {
	case n >= 11 && n <= 14:
		return many
	case n%10 == 1:
		return one
	case n%10 >= 2 && n%10 <= 4:
		return few
	}
	return many
}

// channelName — «Название канала» в кавычках или «канал», если названия нет.
func channelName(m map[string]any) string {
	if s, ok := m["channel_title"].(string); ok && s != "" {
		return "«" + s + "»"
	}
	return "канал"
}

// orderNo — «№ 1001» или пусто.
func orderNo(m map[string]any) string {
	if n, ok := toInt64(m["order_number"]); ok && n > 0 {
		return "№ " + strconv.FormatInt(n, 10)
	}
	return ""
}
