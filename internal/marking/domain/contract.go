package domain

import (
	"strconv"
	"time"

	"github.com/google/uuid"
)

// msk — даты договоров считаются по Москве (R-7).
var msk = time.FixedZone("MSK", 3*60*60)

// ContractKind — вид договора (04 §6, A-31).
type ContractKind string

const (
	// ContractInitial — изначальный: клиент — рекламодатель, исполнитель — платформа.
	ContractInitial ContractKind = "initial"
	// ContractPublisher — с блогером: клиент — платформа, исполнитель — блогер.
	ContractPublisher ContractKind = "publisher"
)

// Типы договора ОРД (ContractType).
const (
	OrdContract             = "contract"
	OrdIntermediaryContract = "intermediary-contract"
	OrdAdditionalAgreement  = "additional-agreement"
)

// Перечисления спецификации v8 — для проверки конфигурации при запуске (R-7).
var (
	OrdContractTypes = []string{OrdContract, OrdIntermediaryContract, OrdAdditionalAgreement}
	OrdSubjectTypes  = []string{"other", "representation", "mediation", "org-distribution", "distribution"}
	OrdActionTypes   = []string{"other", "commercial", "distribution", "conclude"}
)

// ContractParams — параметры договора вида для ОРД (R-7, Q-14, A-33).
type ContractParams struct {
	Type        string
	SubjectType string
	ActionType  string
}

// ContractRules — параметры обоих видов договора и префикс номера.
type ContractRules struct {
	Initial   ContractParams
	Publisher ContractParams
	// NumberPrefix — ORD_CONTRACT_NUMBER_PREFIX; пусто — номер не отправляется.
	NumberPrefix string
}

// WithDefaults подставляет рабочие значения R-7.
func (r ContractRules) WithDefaults() ContractRules {
	if r.Initial == (ContractParams{}) {
		r.Initial = ContractParams{Type: OrdIntermediaryContract, SubjectType: "mediation", ActionType: "conclude"}
	}
	if r.Publisher == (ContractParams{}) {
		r.Publisher = ContractParams{Type: OrdContract, SubjectType: "distribution", ActionType: "distribution"}
	}
	return r
}

// Params — параметры вида.
func (r ContractRules) Params(kind ContractKind) ContractParams {
	if kind == ContractPublisher {
		return r.Publisher
	}
	return r.Initial
}

// OfferKind — какую оферту принимает сторона договора вида kind (R-7): изначальный —
// рекламодатель (advertiser_offer), с блогером — блогер (blogger_offer).
func OfferKind(kind ContractKind) string {
	if kind == ContractPublisher {
		return "blogger_offer"
	}
	return "advertiser_offer"
}

// OfferAcceptance — первое принятие оферты организацией: время и версия документа.
type OfferAcceptance struct {
	AcceptedAt time.Time
	Version    int
}

// Contract — договор в ОРД (04 §6). В ОРД идентифицируется id записи (R-1).
type Contract struct {
	ID                       uuid.UUID
	Kind                     ContractKind
	ClientOrganizationID     uuid.UUID
	ContractorOrganizationID uuid.UUID
	Number                   string
	// SignedDate — дата договора (полночь UTC той даты); нулевая — ещё не определена.
	SignedDate time.Time
	Sync       SyncState
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Ref — ссылка для заданий.
func (c Contract) Ref() ObjectRef { return ObjectRef{Type: TypeContract, ID: c.ID} }

// ExternalID — идентификатор в ОРД.
func (c Contract) ExternalID() string { return c.ID.String() }

// OfferOrganization — сторона, чьё принятие оферты задаёт дату договора.
func (c Contract) OfferOrganization() uuid.UUID {
	if c.Kind == ContractPublisher {
		return c.ContractorOrganizationID
	}
	return c.ClientOrganizationID
}

// ContractRequest — тело POST /contract (API v8).
type ContractRequest struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	ContractorID string `json:"contractorId"`
	ClientID     string `json:"clientId"`
	IsRegReport  bool   `json:"isRegReport"`
	Date         string `json:"date"`
	Number       string `json:"number,omitempty"`
	SubjectType  string `json:"subjectType,omitempty"`
	ActionType   string `json:"actionType,omitempty"`
	// AgentActingForPublisher — только и обязательно у intermediary-contract.
	AgentActingForPublisher *bool `json:"agentActingForPublisher,omitempty"`
}

// ContractDate — дата договора: сутки по Москве момента принятия оферты (R-7).
func ContractDate(acceptedAt time.Time) time.Time {
	y, m, d := acceptedAt.In(msk).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// ContractNumber — номер договора: префикс + версия оферты; пустой префикс — без номера.
func ContractNumber(prefix string, offerVersion int) string {
	if prefix == "" {
		return ""
	}
	return prefix + strconv.Itoa(offerVersion)
}

// NewContractRequest собирает запрос договора (R-7). Отчитывается платформа (isRegReport, A-33).
// Идентификаторы сторон — идентификаторы организаций (R-1).
func NewContractRequest(c Contract, p ContractParams) ContractRequest {
	req := ContractRequest{
		ID: c.ExternalID(), Type: p.Type,
		ContractorID: c.ContractorOrganizationID.String(), ClientID: c.ClientOrganizationID.String(),
		IsRegReport: true, Date: c.SignedDate.Format(time.DateOnly), Number: c.Number,
		SubjectType: p.SubjectType, ActionType: p.ActionType,
	}
	if p.Type == OrdIntermediaryContract {
		agent := false
		req.AgentActingForPublisher = &agent
	}
	return req
}
