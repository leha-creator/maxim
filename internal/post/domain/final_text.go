package domain

import (
	"errors"
	"regexp"
	"strings"
)

// TokenCarrier — где пометка «Реклама» с названием и ИНН (D-18, A-38).
type TokenCarrier string

const (
	CarrierText  TokenCarrier = "text"
	CarrierMedia TokenCarrier = "media"
)

// Valid — значение из перечня 04 §9.
func (c TokenCarrier) Valid() bool { return c == CarrierText || c == CarrierMedia }

// TokenPosition — где erid (D-18, A-38).
type TokenPosition string

const (
	PositionLink  TokenPosition = "link"
	PositionAbove TokenPosition = "above"
	PositionBelow TokenPosition = "below"
)

// Valid — значение из перечня 04 §9.
func (p TokenPosition) Valid() bool {
	return p == PositionLink || p == PositionAbove || p == PositionBelow
}

// ErrFinalTextTooLong — итоговый текст длиннее предела (A-38).
var ErrFinalTextTooLong = errors.New("post: final text is too long")

// Marking — сведения о рекламе для итогового текста (05 §2.5, 06 §6.1).
type Marking struct {
	Carrier  TokenCarrier
	Position TokenPosition
	Name     string
	INN      string
}

// MarkingLine — строка маркировки (05 §2.5): «Реклама. {название}, ИНН {ИНН}.» при
// пометке в тексте и «erid: {erid}» при erid над или под постом. Обычный текст:
// спецсимволы HTML экранированы. Пустая — строки нет. Состав сведений рабочий (Q-30).
func MarkingLine(m Marking, erid string) string {
	var parts []string
	if m.Carrier == CarrierText {
		parts = append(parts, "Реклама. "+m.Name+", ИНН "+m.INN+".")
	}
	if m.Position != PositionLink {
		parts = append(parts, "erid: "+erid)
	}
	return EscapeHTML(strings.Join(parts, " "))
}

var hrefAttr = regexp.MustCompile(`href="([^"]*)"`)

// AddEridToLinks дописывает параметр erid к каждой ссылке готового текста (A-38).
func AddEridToLinks(body, erid string) string {
	return hrefAttr.ReplaceAllStringFunc(body, func(attr string) string {
		href := unescaper.Replace(hrefAttr.FindStringSubmatch(attr)[1])
		return `href="` + EscapeHTML(AddQueryParams(href, [][2]string{{"erid", erid}})) + `"`
	})
}

// FinalText собирает итоговый текст по месту токена (05 §2.5):
// строка маркировки над body при erid над постом, иначе под body через пустую
// строку; при erid в ссылке параметр erid добавляется к каждой ссылке. Длиннее
// limit кодовых единиц UTF-16 — ErrFinalTextTooLong.
func FinalText(body string, m Marking, erid string, limit int) (string, error) {
	if m.Position == PositionLink {
		body = AddEridToLinks(body, erid)
	}
	line := MarkingLine(m, erid)
	var out string
	switch {
	case line == "":
		out = body
	case m.Position == PositionAbove:
		out = line + "\n" + body
	default:
		out = body + "\n\n" + line
	}
	if Length(out) > limit {
		return "", ErrFinalTextTooLong
	}
	return out, nil
}

// ErrMarkingMismatch — итоговый текст версии не совпал с собранным заново (I-01).
var ErrMarkingMismatch = errors.New("post: final text does not match marking")

// VerifyFinalText — I-01 перед публикацией (R-3 этапа «Автопостинг и контроль»): версия
// ready, erid задан, итоговый текст равен собранному заново из body и параметров маркировки
// версии и содержит erid, если erid над или под постом.
func VerifyFinalText(v Version, limit int) error {
	if v.Status != VersionReady || strings.TrimSpace(v.Erid) == "" {
		return ErrMarkingMismatch
	}
	want, err := FinalText(v.Body, Marking{Carrier: v.TokenCarrier, Position: v.TokenPosition,
		Name: v.AdvertiserName, INN: v.AdvertiserINN}, v.Erid, limit)
	if err != nil || want != v.FinalText {
		return ErrMarkingMismatch
	}
	if v.TokenPosition != PositionLink && !strings.Contains(v.FinalText, EscapeHTML(v.Erid)) {
		return ErrMarkingMismatch
	}
	return nil
}
