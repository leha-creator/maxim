package domain

import (
	"fmt"
	"strings"
)

// ProblemCode — нарушение автоматической проверки поста (S-05).
type ProblemCode string

const (
	ProblemEmptyText             ProblemCode = "empty_text"
	ProblemBodyTooLong           ProblemCode = "body_too_long"
	ProblemTooManyMedia          ProblemCode = "too_many_media"
	ProblemLinkNotHTTPS          ProblemCode = "link_not_https"
	ProblemMediaRequiredForToken ProblemCode = "media_required_for_token"
	ProblemLinkRequiredForToken  ProblemCode = "link_required_for_token"
	ProblemMediaProcessing       ProblemCode = "media_processing"
	ProblemRequisitesMissing     ProblemCode = "requisites_missing"
)

// SampleErid — условный erid предпросмотра.
const SampleErid = "erid_sample"

// Problem — нарушение с сообщением для кабинета.
type Problem struct {
	Code    ProblemCode
	Message string
}

// Requisites — название и ИНН рекламодателя для маркировки (06 §6.1, R-14).
type Requisites struct {
	Name string
	INN  string
}

// Complete — есть и название, и ИНН.
func (r Requisites) Complete() bool {
	return strings.TrimSpace(r.Name) != "" && strings.TrimSpace(r.INN) != ""
}

// ProblemContext — сведения, которых нет в черновике.
type ProblemContext struct {
	BodyLimit      int
	FinalTextLimit int
	AlbumLimit     int
	// MediaReady — все нужные производные изображений черновика построены.
	MediaReady bool
	Requisites Requisites
}

// Problems проверяет черновик (S-05, R-13). body — сериализованный текст с UTM-метками.
// Пустой список — пост можно оформлять.
func Problems(d Draft, body string, pc ProblemContext) []Problem {
	var out []Problem
	add := func(code ProblemCode, msg string) { out = append(out, Problem{Code: code, Message: msg}) }

	if strings.TrimSpace(PlainText(d.Doc)) == "" {
		add(ProblemEmptyText, "post text is empty")
	}
	if n := Length(body); n > pc.BodyLimit {
		add(ProblemBodyTooLong, fmt.Sprintf("body is %d characters, limit is %d", n, pc.BodyLimit))
	} else if pc.Requisites.Complete() {
		// A-38: резерв под строку маркировки не вмещает длинное название — итоговый текст длиннее предела.
		m := Marking{Carrier: d.TokenCarrier, Position: d.TokenPosition, Name: pc.Requisites.Name, INN: pc.Requisites.INN}
		if _, err := FinalText(body, m, SampleErid, pc.FinalTextLimit); err != nil {
			add(ProblemBodyTooLong, fmt.Sprintf("body with the marking line exceeds %d characters", pc.FinalTextLimit))
		}
	}
	if len(d.MediaIDs) > pc.AlbumLimit {
		add(ProblemTooManyMedia, fmt.Sprintf("album has %d images, limit is %d", len(d.MediaIDs), pc.AlbumLimit))
	}
	for _, href := range LinkHrefs(d.Doc) {
		if !ValidHref(href) {
			add(ProblemLinkNotHTTPS, "links must use https://")
			break
		}
	}
	if d.TokenCarrier == CarrierMedia && len(d.MediaIDs) == 0 {
		add(ProblemMediaRequiredForToken, "token_carrier=media requires at least one image")
	}
	if d.TokenPosition == PositionLink && !HasLinks(d.Doc) {
		add(ProblemLinkRequiredForToken, "token_position=link requires at least one link")
	}
	if len(d.MediaIDs) > 0 && !pc.MediaReady {
		add(ProblemMediaProcessing, "images are still being processed, retry the preview")
	}
	if !pc.Requisites.Complete() {
		add(ProblemRequisitesMissing, "organization name or INN is missing")
	}
	return out
}

// Codes — коды проблем по порядку.
func Codes(ps []Problem) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, string(p.Code))
	}
	return out
}
