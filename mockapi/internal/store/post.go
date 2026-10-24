package store

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"maxim/mockapi/internal/api"
)

// Пределы длины текста (A-38).
const (
	BodyLimit      = 3800
	FinalTextLimit = 4000
	AlbumLimit     = 10 // A-23
)

// markOrder — фиксированный порядок вложения меток, снаружи внутрь (05 §2.5).
var markOrder = []api.EditorMarkType{"link", "bold", "italic", "underline", "strike", "highlight", "code"}

var markTags = map[api.EditorMarkType]string{
	"bold": "b", "italic": "i", "underline": "u", "strike": "s", "highlight": "mark", "code": "code",
}

// Serialize превращает документ редактора в текст формата html API MAX.
// Точные теги MAX заглушка не проверяет — это задача сериализатора продукта (Q-06).
func Serialize(doc api.EditorDoc) string {
	var blocks []string
	for _, b := range doc.Content {
		var sb strings.Builder
		if b.Content != nil {
			for _, t := range *b.Content {
				sb.WriteString(serializeText(t))
			}
		}
		s := sb.String()
		switch b.Type {
		case "heading":
			s = "<h1>" + s + "</h1>"
		case "blockquote":
			s = "<blockquote>" + s + "</blockquote>"
		}
		blocks = append(blocks, s)
	}
	return strings.Join(blocks, "\n")
}

func serializeText(t api.EditorText) string {
	s := html.EscapeString(t.Text)
	if t.Marks == nil {
		return s
	}
	marks := map[api.EditorMarkType]api.EditorMark{}
	for _, m := range *t.Marks {
		marks[m.Type] = m
	}
	for i := len(markOrder) - 1; i >= 0; i-- {
		m, ok := marks[markOrder[i]]
		if !ok {
			continue
		}
		if m.Type == "link" {
			href := ""
			if m.Href != nil {
				href = *m.Href
			}
			s = fmt.Sprintf(`<a href="%s">%s</a>`, html.EscapeString(href), s)
			continue
		}
		tag := markTags[m.Type]
		s = "<" + tag + ">" + s + "</" + tag + ">"
	}
	return s
}

// PlainText возвращает текст документа без разметки — для проверки на пустоту.
func PlainText(doc api.EditorDoc) string {
	var sb strings.Builder
	for _, b := range doc.Content {
		if b.Content != nil {
			for _, t := range *b.Content {
				sb.WriteString(t.Text)
			}
		}
	}
	return strings.TrimSpace(sb.String())
}

// Problem — нарушение автоматической проверки поста.
type Problem struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// CheckDraft выполняет автоматические проверки черновика (S-05, S-06).
func CheckDraft(d PostDraft) (body string, problems []Problem) {
	body = Serialize(d.Doc)
	if PlainText(d.Doc) == "" {
		problems = append(problems, Problem{"empty_text", "post text is empty"})
	}
	if n := len([]rune(body)); n > BodyLimit {
		problems = append(problems, Problem{"body_too_long", fmt.Sprintf("body is %d characters, limit is %d", n, BodyLimit)})
	}
	if len(d.MediaIDs) > AlbumLimit {
		problems = append(problems, Problem{"too_many_media", fmt.Sprintf("album has %d images, limit is %d", len(d.MediaIDs), AlbumLimit)})
	}
	hasLinks := false
	for _, b := range d.Doc.Content {
		if b.Content == nil {
			continue
		}
		for _, t := range *b.Content {
			if t.Marks == nil {
				continue
			}
			for _, m := range *t.Marks {
				if m.Type != "link" {
					continue
				}
				hasLinks = true
				if m.Href == nil || !strings.HasPrefix(*m.Href, "https://") {
					problems = append(problems, Problem{"link_not_https", "links must use https://"})
				}
			}
		}
	}
	if d.TokenCarrier == "media" && len(d.MediaIDs) == 0 {
		problems = append(problems, Problem{"media_required_for_token", "token_carrier=media requires at least one image"})
	}
	if d.TokenPosition == "link" && !hasLinks {
		problems = append(problems, Problem{"link_required_for_token", "token_position=link requires at least one link"})
	}
	return body, problems
}

var hrefRe = regexp.MustCompile(`href="([^"]*)"`)

// FinalText собирает итоговый текст со строкой маркировки (05 §2.5, D-18).
func FinalText(body, carrier, position, name, inn, erid string) string {
	label := ""
	if carrier == "text" {
		label = fmt.Sprintf("Реклама. %s, ИНН %s.", name, inn)
	}
	line := label
	if position != "link" {
		line = strings.TrimSpace(label + " erid: " + erid)
	} else {
		body = hrefRe.ReplaceAllStringFunc(body, func(m string) string {
			u := hrefRe.FindStringSubmatch(m)[1]
			sep := "?"
			if strings.Contains(u, "?") {
				sep = "&amp;"
			}
			return `href="` + u + sep + "erid=" + erid + `"`
		})
	}
	switch {
	case line == "":
		return body
	case position == "above":
		return line + "\n" + body
	default:
		return body + "\n\n" + line
	}
}

// DefaultCarrier — место пометки по умолчанию (D-30).
func DefaultCarrier(mediaCount int) string {
	if mediaCount > 0 {
		return "media"
	}
	return "text"
}
