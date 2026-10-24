package domain

import (
	"fmt"
	"net/url"
	"strings"
	"unicode/utf16"
)

// Пределы длины (A-38, R-11) по умолчанию; рабочие значения — из конфигурации.
const (
	DefaultBodyLimit      = 3800
	DefaultFinalTextLimit = 4000
	// SaveLimit — сохранение черновика длиннее этого отклоняется (R-11).
	SaveLimit = 8000
)

// Теги html MAX (R-10). Сверены со страницей dev.max.ru о форматировании
// (docs-api/use-cases/sending-messages/text-formatting).
var markTags = map[MarkType]string{
	MarkBold: "b", MarkItalic: "i", MarkUnderline: "u", MarkStrike: "s", MarkHighlight: "mark", MarkCode: "code",
}

var blockTags = map[BlockType]string{BlockHeading: "h1", BlockBlockquote: "blockquote"}

// escaper — спецсимволы HTML в тексте и адресах (R-10).
var escaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")

// unescaper — обратное преобразование для адресов в готовом тексте.
var unescaper = strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`)

// EscapeHTML экранирует `& < > "`.
func EscapeHTML(s string) string { return escaper.Replace(s) }

// Постоянные UTM-метки ссылок поста (A-21); utm_campaign — номер заказа.
const (
	UTMSource = "max"
	UTMMedium = "channel_ad"
)

// UTM — метки ссылок поста (A-21). Campaign — значение utm_campaign.
type UTM struct {
	Campaign string
}

// UTMForOrder — метки с настоящим номером заказа: utm_campaign=order_{номер}.
func UTMForOrder(orderNumber int64) UTM { return UTM{Campaign: fmt.Sprintf("order_%d", orderNumber)} }

// PreviewUTM — метки предпросмотра: заглушка номера заказа фиксированной ширины,
// чтобы длина не была занижена (R-12).
var PreviewUTM = UTM{Campaign: "order_0000000000"}

func (u UTM) params() [][2]string {
	return [][2]string{{"utm_source", UTMSource}, {"utm_medium", UTMMedium}, {"utm_campaign", u.Campaign}}
}

// Serialize превращает нормализованный документ в текст формата html MAX (05 §2.5):
// блоки через "\n", метки вложены в порядке MarkOrder, спецсимволы экранированы.
// utm != nil — к адресам ссылок дописываются недостающие UTM-метки (R-12).
func Serialize(doc Doc, utm *UTM) string {
	blocks := make([]string, 0, len(doc.Blocks))
	for _, b := range doc.Blocks {
		var sb strings.Builder
		for _, t := range b.Content {
			sb.WriteString(serializeText(t, utm))
		}
		s := sb.String()
		if tag, ok := blockTags[b.Type]; ok {
			s = "<" + tag + ">" + s + "</" + tag + ">"
		}
		blocks = append(blocks, s)
	}
	return strings.Join(blocks, "\n")
}

// serializeText оборачивает текст метками изнутри наружу: последняя по MarkOrder — ближе всех к тексту.
func serializeText(t Text, utm *UTM) string {
	s := EscapeHTML(t.Text)
	for i := len(MarkOrder) - 1; i >= 0; i-- {
		for _, m := range t.Marks {
			if m.Type != MarkOrder[i] {
				continue
			}
			if m.Type == MarkLink {
				href := m.Href
				if utm != nil {
					href = AddQueryParams(href, utm.params())
				}
				s = `<a href="` + EscapeHTML(href) + `">` + s + "</a>"
				continue
			}
			tag := markTags[m.Type]
			s = "<" + tag + ">" + s + "</" + tag + ">"
		}
	}
	return s
}

// AddQueryParams дописывает в конец строки запроса параметры, которых в адресе
// ещё нет; заданные не меняются, порядок исходных сохраняется, фрагмент остаётся в конце.
func AddQueryParams(href string, params [][2]string) string {
	base, fragment, hasFragment := strings.Cut(href, "#")
	path, query, _ := strings.Cut(base, "?")
	present := map[string]bool{}
	if query != "" {
		for _, kv := range strings.Split(query, "&") {
			k, _, _ := strings.Cut(kv, "=")
			if uk, err := url.QueryUnescape(k); err == nil {
				k = uk
			}
			present[k] = true
		}
	}
	var add []string
	for _, p := range params {
		if !present[p[0]] {
			add = append(add, url.QueryEscape(p[0])+"="+url.QueryEscape(p[1]))
		}
	}
	if len(add) > 0 {
		if query != "" {
			query += "&"
		}
		query += strings.Join(add, "&")
	}
	out := path
	if query != "" {
		out += "?" + query
	}
	if hasFragment {
		out += "#" + fragment
	}
	return out
}

// Length — длина строки в кодовых единицах UTF-16 (R-11): не мягче подсчёта
// по символам для кириллицы и эмодзи.
func Length(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return n
}
