package domain

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

const autolinkScheme = "https://"

// autolinkTrailing — символы, которые не входят в конец распознанного адреса (R-9).
const autolinkTrailing = `.,;:!?»"'`

// Autolink превращает адреса `https://` в тексте без ссылки в ссылки (D-22, R-9).
// Адрес начинается с `https://` в начале узла или после символа, который не буква
// и не цифра, и заканчивается пробелом или концом узла. Завершающие `.,;:!?»"'`
// и `)` без парной `(` в адрес не входят. Адрес, не прошедший ValidHref, остаётся
// текстом. Остальные метки узла сохраняются. Ожидает нормализованный документ.
func Autolink(doc Doc) (Doc, int) {
	out := Doc{Blocks: make([]Block, 0, len(doc.Blocks))}
	found := 0
	for _, b := range doc.Blocks {
		nb := Block{Type: b.Type}
		for _, t := range b.Content {
			if hasMark(t.Marks, MarkLink) {
				nb.Content = append(nb.Content, t)
				continue
			}
			parts, n := splitLinks(t)
			found += n
			nb.Content = append(nb.Content, parts...)
		}
		nb.Content = mergeNodes(nb.Content)
		out.Blocks = append(out.Blocks, nb)
	}
	return out, found
}

// splitLinks делит узел на текст и ссылки; возвращает узлы и число найденных адресов.
func splitLinks(t Text) ([]Text, int) {
	var out []Text
	found := 0
	rest := t.Text
	for {
		start, end := nextAddress(rest)
		if start < 0 {
			break
		}
		if start > 0 {
			out = append(out, Text{Text: rest[:start], Marks: t.Marks})
		}
		marks := append(slices.Clone(t.Marks), Mark{Type: MarkLink, Href: rest[start:end]})
		slices.SortFunc(marks, func(a, b Mark) int { return markRank(a.Type) - markRank(b.Type) })
		out = append(out, Text{Text: rest[start:end], Marks: marks})
		found++
		rest = rest[end:]
	}
	if rest != "" {
		out = append(out, Text{Text: rest, Marks: t.Marks})
	}
	return out, found
}

// nextAddress ищет первый допустимый адрес в s: байтовые границы [start, end) или -1.
func nextAddress(s string) (start, end int) {
	offset := 0
	for {
		i := strings.Index(s[offset:], autolinkScheme)
		if i < 0 {
			return -1, -1
		}
		i += offset
		offset = i + len(autolinkScheme)
		if i > 0 {
			if r, _ := utf8.DecodeLastRuneInString(s[:i]); unicode.IsLetter(r) || unicode.IsDigit(r) {
				continue
			}
		}
		j := strings.IndexFunc(s[i:], unicode.IsSpace)
		if j < 0 {
			j = len(s)
		} else {
			j += i
		}
		candidate := trimAddress(s[i:j])
		if ValidHref(candidate) {
			return i, i + len(candidate)
		}
	}
}

// trimAddress убирает завершающие знаки препинания и непарные `)`.
func trimAddress(a string) string {
	for len(a) > len(autolinkScheme) {
		r, size := utf8.DecodeLastRuneInString(a)
		switch {
		case strings.ContainsRune(autolinkTrailing, r):
		case r == ')' && strings.Count(a, "(") < strings.Count(a, ")"):
		default:
			return a
		}
		a = a[:len(a)-size]
	}
	return a
}
