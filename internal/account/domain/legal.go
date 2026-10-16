package domain

import (
	"net/netip"
	"time"

	"github.com/google/uuid"
)

// LegalDocumentKind — вид юридического документа (04 §2).
type LegalDocumentKind string

const (
	DocAdvertiserOffer LegalDocumentKind = "advertiser_offer"
	DocBloggerOffer    LegalDocumentKind = "blogger_offer"
	DocPlatformRules   LegalDocumentKind = "platform_rules"
	DocPrivacyPolicy   LegalDocumentKind = "privacy_policy"
)

// LegalDocumentKinds — все виды в постоянном порядке.
var LegalDocumentKinds = []LegalDocumentKind{DocAdvertiserOffer, DocBloggerOffer, DocPlatformRules, DocPrivacyPolicy}

// Valid сообщает, что вид из перечисления.
func (k LegalDocumentKind) Valid() bool {
	for _, v := range LegalDocumentKinds {
		if v == k {
			return true
		}
	}
	return false
}

// Title — название документа для кабинета (R-6: вычисляется по виду).
func (k LegalDocumentKind) Title() string {
	switch k {
	case DocAdvertiserOffer:
		return "Оферта для рекламодателя"
	case DocBloggerOffer:
		return "Оферта для блогера"
	case DocPlatformRules:
		return "Правила площадки"
	case DocPrivacyPolicy:
		return "Политика обработки персональных данных"
	}
	return string(k)
}

// LegalDocument — версия оферты или правил. Действующая — с наибольшим Version (04 §2).
type LegalDocument struct {
	ID          uuid.UUID
	Kind        LegalDocumentKind
	Version     int
	PublishedAt time.Time
	StorageKey  string
	SHA256      string
	CreatedAt   time.Time
}

// LegalAcceptance — принятие версии документа организацией. Неизменяемо (04 §1).
type LegalAcceptance struct {
	ID              uuid.UUID
	LegalDocumentID uuid.UUID
	UserID          uuid.UUID
	OrganizationID  uuid.UUID
	AcceptedAt      time.Time
	IP              *netip.Addr
}

// RequiredDocuments — виды документов, которые нужно принять для открытых
// кабинетов (S-01 шаг 5, S-10 шаг 3).
func RequiredDocuments(cabinets []Cabinet) []LegalDocumentKind {
	need := map[LegalDocumentKind]bool{}
	for _, c := range cabinets {
		switch c {
		case CabinetAdvertiser:
			need[DocAdvertiserOffer] = true
		case CabinetBlogger:
			need[DocBloggerOffer] = true
		}
		need[DocPlatformRules] = true
		need[DocPrivacyPolicy] = true
	}
	var out []LegalDocumentKind
	for _, k := range LegalDocumentKinds {
		if need[k] {
			out = append(out, k)
		}
	}
	return out
}

// PendingDocuments — действующие документы из нужных, которые организация ещё
// не приняла. Новая версия требует нового принятия (A-25).
func PendingDocuments(current []LegalDocument, cabinets []Cabinet, accepted map[uuid.UUID]bool) []LegalDocument {
	required := map[LegalDocumentKind]bool{}
	for _, k := range RequiredDocuments(cabinets) {
		required[k] = true
	}
	var out []LegalDocument
	for _, d := range current {
		if required[d.Kind] && !accepted[d.ID] {
			out = append(out, d)
		}
	}
	return out
}
