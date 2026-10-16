// Package domain — правила модуля аккаунтов (04 §2): пользователи, коды входа,
// сессии, организации и их реквизиты, оферты. Без базы, сети и журнала.
package domain

import "errors"

// ErrNotFound — записи нет. Репозитории возвращают его, слой app решает, что ответить.
var ErrNotFound = errors.New("account: not found")

// LegalType — тип лица (04 §9).
type LegalType string

const (
	LegalEntity    LegalType = "legal_entity"
	SoleProprietor LegalType = "sole_proprietor"
	SelfEmployed   LegalType = "self_employed"
)

// Valid сообщает, что значение из перечисления.
func (t LegalType) Valid() bool {
	return t == LegalEntity || t == SoleProprietor || t == SelfEmployed
}

// TaxRegime — режим налогообложения (04 §9).
type TaxRegime string

const (
	TaxOSNO TaxRegime = "osno"
	TaxUSN  TaxRegime = "usn"
	TaxAUSN TaxRegime = "ausn"
	TaxESHN TaxRegime = "eshn"
	TaxPSN  TaxRegime = "psn"
	TaxNPD  TaxRegime = "npd"
)

// VatRate — ставка НДС (04 §9).
type VatRate string

const (
	VatNone VatRate = "none"
	Vat5    VatRate = "vat_5"
	Vat7    VatRate = "vat_7"
	Vat22   VatRate = "vat_22"
)

// Cabinet — роль организации: кабинет рекламодателя или блогера.
type Cabinet string

const (
	CabinetAdvertiser Cabinet = "advertiser"
	CabinetBlogger    Cabinet = "blogger"
)

// Valid сообщает, что значение из перечисления.
func (c Cabinet) Valid() bool { return c == CabinetAdvertiser || c == CabinetBlogger }

// Status — состояние пользователя или организации.
type Status string

const (
	StatusActive  Status = "active"
	StatusBlocked Status = "blocked"
)

// AdminRole — роль администратора (07 §4).
type AdminRole string

const (
	AdminModerator AdminRole = "moderator"
	AdminSenior    AdminRole = "senior"
)

// MemberRole — роль участника организации. В первой версии только owner (A-13).
type MemberRole string

const MemberOwner MemberRole = "owner"
