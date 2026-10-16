// Package domain — правила модуля post: документ редактора и его сериализация в
// форматирование MAX, файлы медиатеки и плашка, кампания, черновик поста, версии
// поста (04 §4, 05 §2.5, 06 §6). Без базы, сети и журнала.
package domain

import (
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"unicode"
)

// BlockType — вид блока документа редактора (D-05).
type BlockType string

const (
	BlockParagraph  BlockType = "paragraph"
	BlockHeading    BlockType = "heading"
	BlockBlockquote BlockType = "blockquote"
)

// MarkType — метка текста; набор равен форматированию MAX (05 §2.5).
type MarkType string

const (
	MarkLink      MarkType = "link"
	MarkBold      MarkType = "bold"
	MarkItalic    MarkType = "italic"
	MarkUnderline MarkType = "underline"
	MarkStrike    MarkType = "strike"
	MarkHighlight MarkType = "highlight"
	MarkCode      MarkType = "code"
)

// MarkOrder — порядок меток снаружи внутрь (R-10, рабочий ответ на Q-06). В нём же
// метки хранятся в нормализованном узле.
var MarkOrder = []MarkType{MarkLink, MarkBold, MarkItalic, MarkUnderline, MarkStrike, MarkHighlight, MarkCode}

// Mark — метка узла. Href есть только у link.
type Mark struct {
	Type MarkType `json:"type"`
	Href string   `json:"href,omitempty"`
}

// Text — текстовый узел. "\n" внутри текста — перенос строки.
type Text struct {
	Text  string `json:"text"`
	Marks []Mark `json:"marks,omitempty"`
}

// Block — блок документа. Пустой Content — пустая строка.
type Block struct {
	Type    BlockType `json:"type"`
	Content []Text    `json:"content,omitempty"`
}

// Doc — документ редактора.
type Doc struct {
	Blocks []Block
}

// docJSON — представление документа в формате редактора: {"type":"doc","content":[…]}.
// Узлы несут "type":"text".
type docJSON struct {
	Type    string      `json:"type"`
	Content []blockJSON `json:"content"`
}

type blockJSON struct {
	Type    BlockType  `json:"type"`
	Content []textJSON `json:"content,omitempty"`
}

type textJSON struct {
	Type  string `json:"type"`
	Text  string `json:"text"`
	Marks []Mark `json:"marks,omitempty"`
}

// MarshalJSON кодирует документ в формате редактора.
func (d Doc) MarshalJSON() ([]byte, error) {
	out := docJSON{Type: "doc", Content: make([]blockJSON, 0, len(d.Blocks))}
	for _, b := range d.Blocks {
		jb := blockJSON{Type: b.Type}
		for _, t := range b.Content {
			jb.Content = append(jb.Content, textJSON{Type: "text", Text: t.Text, Marks: t.Marks})
		}
		out.Content = append(out.Content, jb)
	}
	return json.Marshal(out)
}

// UnmarshalJSON читает документ в формате редактора.
func (d *Doc) UnmarshalJSON(data []byte) error {
	var in docJSON
	if err := json.Unmarshal(data, &in); err != nil {
		return err
	}
	if in.Type != "doc" {
		return fmt.Errorf("editor doc: type %q", in.Type)
	}
	d.Blocks = make([]Block, 0, len(in.Content))
	for _, jb := range in.Content {
		b := Block{Type: jb.Type}
		for _, jt := range jb.Content {
			b.Content = append(b.Content, Text{Text: jt.Text, Marks: jt.Marks})
		}
		d.Blocks = append(d.Blocks, b)
	}
	return nil
}

// EncodedSize — размер документа в JSON формата редактора, байт.
func (d Doc) EncodedSize() int {
	b, err := json.Marshal(d)
	if err != nil {
		return 0
	}
	return len(b)
}

// DocLimits — пределы документа (R-8).
type DocLimits struct {
	MaxBlocks    int
	MaxNodes     int
	MaxJSONBytes int
}

// DefaultDocLimits — пределы R-8: 300 блоков, 3000 текстовых узлов, 128 КБ JSON.
var DefaultDocLimits = DocLimits{MaxBlocks: 300, MaxNodes: 3000, MaxJSONBytes: 128 << 10}

// MaxHrefLength — предел длины адреса ссылки (R-8).
const MaxHrefLength = 2048

// ReasonInvalidDoc — причина 422 для нарушения документа; уточнение — DocError.Rule.
const ReasonInvalidDoc = "invalid_doc"

// Правила документа (details.rule).
const (
	RuleTooManyBlocks  = "too_many_blocks"
	RuleTooManyNodes   = "too_many_nodes"
	RuleDocTooLarge    = "doc_too_large"
	RuleInvalidBlock   = "invalid_block"
	RuleInvalidMark    = "invalid_mark"
	RuleHrefRequired   = "href_required"
	RuleHrefUnexpected = "href_unexpected"
	RuleHrefInvalid    = "href_invalid"
)

// DocError — нарушение документа: путь узла (`content.2.content.0`) и правило.
type DocError struct {
	Path string
	Rule string
}

func (e *DocError) Error() string { return "post: editor doc: " + e.Path + ": " + e.Rule }

func blockPath(i int) string      { return fmt.Sprintf("content.%d", i) }
func nodePath(i, j int) string    { return fmt.Sprintf("content.%d.content.%d", i, j) }
func markPath(i, j, k int) string { return fmt.Sprintf("content.%d.content.%d.marks.%d", i, j, k) }
func validBlock(t BlockType) bool {
	return t == BlockParagraph || t == BlockHeading || t == BlockBlockquote
}
func markRank(t MarkType) int    { return slices.Index(MarkOrder, t) }
func validMark(t MarkType) bool  { return markRank(t) >= 0 }
func sameMarks(a, b []Mark) bool { return slices.Equal(a, b) }
func hasMark(ms []Mark, t MarkType) bool {
	return slices.ContainsFunc(ms, func(m Mark) bool { return m.Type == t })
}

// ValidHref — адрес ссылки допустим (R-8): `https://`, есть хост, нет `userinfo`,
// пробелов и управляющих символов, не длиннее 2048.
func ValidHref(s string) bool {
	if len(s) > MaxHrefLength || !strings.HasPrefix(s, "https://") {
		return false
	}
	if strings.ContainsFunc(s, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return false
	}
	u, err := url.Parse(s)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
		return false
	}
	return true
}

// cleanText приводит переводы строк к LF, табуляцию — к пробелу и удаляет прочие
// управляющие символы (R-8).
func cleanText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\n':
			return r
		case r == '\t':
			return ' '
		case unicode.IsControl(r):
			return -1
		}
		return r
	}, s)
}

// Normalize проверяет документ и приводит его к каноническому виду (R-8): чистит
// текст, убирает повторы меток и упорядочивает их по MarkOrder, удаляет пустые
// узлы и сливает соседние с одинаковыми метками. Пустой блок сохраняется.
// Нарушение — *DocError с путём узла во входном документе.
func Normalize(doc Doc, lim DocLimits) (Doc, error) {
	if lim.MaxJSONBytes > 0 && doc.EncodedSize() > lim.MaxJSONBytes {
		return Doc{}, &DocError{Path: "content", Rule: RuleDocTooLarge}
	}
	if len(doc.Blocks) > lim.MaxBlocks {
		return Doc{}, &DocError{Path: "content", Rule: RuleTooManyBlocks}
	}
	nodes := 0
	for _, b := range doc.Blocks {
		nodes += len(b.Content)
	}
	if nodes > lim.MaxNodes {
		return Doc{}, &DocError{Path: "content", Rule: RuleTooManyNodes}
	}

	out := Doc{Blocks: make([]Block, 0, len(doc.Blocks))}
	for i, b := range doc.Blocks {
		if !validBlock(b.Type) {
			return Doc{}, &DocError{Path: blockPath(i), Rule: RuleInvalidBlock}
		}
		nb := Block{Type: b.Type}
		for j, t := range b.Content {
			marks, err := normalizeMarks(t.Marks, i, j)
			if err != nil {
				return Doc{}, err
			}
			nb.Content = append(nb.Content, Text{Text: cleanText(t.Text), Marks: marks})
		}
		nb.Content = mergeNodes(nb.Content)
		out.Blocks = append(out.Blocks, nb)
	}
	return out, nil
}

// normalizeMarks проверяет метки узла, убирает повторы (у ссылки остаётся первая)
// и упорядочивает их по MarkOrder.
func normalizeMarks(in []Mark, i, j int) ([]Mark, error) {
	var out []Mark
	for k, m := range in {
		switch {
		case !validMark(m.Type):
			return nil, &DocError{Path: markPath(i, j, k), Rule: RuleInvalidMark}
		case m.Type == MarkLink && m.Href == "":
			return nil, &DocError{Path: markPath(i, j, k), Rule: RuleHrefRequired}
		case m.Type == MarkLink && !ValidHref(m.Href):
			return nil, &DocError{Path: markPath(i, j, k), Rule: RuleHrefInvalid}
		case m.Type != MarkLink && m.Href != "":
			return nil, &DocError{Path: markPath(i, j, k), Rule: RuleHrefUnexpected}
		}
		if !hasMark(out, m.Type) {
			out = append(out, m)
		}
	}
	slices.SortFunc(out, func(a, b Mark) int { return markRank(a.Type) - markRank(b.Type) })
	return out, nil
}

// mergeNodes удаляет пустые узлы и сливает соседние с одинаковыми метками.
func mergeNodes(in []Text) []Text {
	var out []Text
	for _, t := range in {
		if t.Text == "" {
			continue
		}
		if n := len(out); n > 0 && sameMarks(out[n-1].Marks, t.Marks) {
			out[n-1].Text += t.Text
			continue
		}
		out = append(out, t)
	}
	return out
}

// HasLinks — в документе есть хотя бы одна ссылка.
func HasLinks(doc Doc) bool {
	for _, b := range doc.Blocks {
		for _, t := range b.Content {
			if hasMark(t.Marks, MarkLink) {
				return true
			}
		}
	}
	return false
}

// PlainText — текст документа без разметки, блоки через "\n"; для проверки на пустоту.
func PlainText(doc Doc) string {
	lines := make([]string, 0, len(doc.Blocks))
	for _, b := range doc.Blocks {
		var sb strings.Builder
		for _, t := range b.Content {
			sb.WriteString(t.Text)
		}
		lines = append(lines, sb.String())
	}
	return strings.Join(lines, "\n")
}

// LinkHrefs — адреса всех ссылок документа по порядку.
func LinkHrefs(doc Doc) []string {
	var out []string
	for _, b := range doc.Blocks {
		for _, t := range b.Content {
			for _, m := range t.Marks {
				if m.Type == MarkLink {
					out = append(out, m.Href)
				}
			}
		}
	}
	return out
}
