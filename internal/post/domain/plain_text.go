package domain

import (
	"regexp"
	"strings"
)

// Разбор body версии (текст формата html MAX, 05 §2.5) для креатива в ОРД (этап «Маркировка», R-8).
// body собирает Serialize: теги — только из markTags, blockTags и <a href="…">, спецсимволы
// текста и адресов экранированы, поэтому достаточно простых выражений.
var (
	tagRe  = regexp.MustCompile(`<[^>]*>`)
	hrefRe = regexp.MustCompile(`<a href="([^"]*)">`)
)

// BodyPlainText — текст body без разметки: теги убраны, сущности HTML раскрыты, блоки через "\n".
func BodyPlainText(body string) string {
	return strings.TrimSpace(unescaper.Replace(tagRe.ReplaceAllString(body, "")))
}

// BodyLinkURLs — адреса ссылок body по порядку без повторов (уже с UTM-метками версии).
func BodyLinkURLs(body string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, m := range hrefRe.FindAllStringSubmatch(body, -1) {
		u := unescaper.Replace(m[1])
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		out = append(out, u)
	}
	return out
}
