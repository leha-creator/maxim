package store

import (
	"time"

	"github.com/google/uuid"

	"maxim/mockapi/internal/api"
)

// Названия сущностей и полей — из docs/04-domain-model.md. Поля, которые
// заглушке не нужны (хеши, ключи хранилища, синхронизация с ОРД), опущены.

type User struct {
	ID          uuid.UUID
	Email       string
	OrgID       *uuid.UUID // в первой версии у пользователя одна организация
	LastLoginAt *time.Time
	CreatedAt   time.Time
}

type Session struct {
	Token     string
	UserID    uuid.UUID
	CreatedAt time.Time
}

type Organization struct {
	ID                uuid.UUID
	LegalType         string
	INN               string
	Name              string
	ContactName       string
	Phone             string
	KPP               *string
	Address           *string
	TaxRegime         *string
	VatRate           *string
	SignerPosition    *string
	SignerName        *string
	SignerBasisName   *string
	SignerBasisNumber *string
	BankAccount       *string
	BankName          *string
	BankCorrAccount   *string
	BankBik           *string
	BloggerFullName   *string
	TaxDocUploaded    bool
	Status            string
	IsAdvertiser      bool
	IsBlogger         bool
	CreatedAt         time.Time
}

type LegalDocument struct {
	ID          uuid.UUID
	Kind        string
	Version     int
	PublishedAt time.Time
	Title       string
	URL         string
}

type Notification struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Template   string
	Title      string
	Text       string
	ObjectType *string
	ObjectID   *uuid.UUID
	ReadAt     *time.Time
	CreatedAt  time.Time
}

type Category struct {
	ID       uuid.UUID
	Slug     string
	Name     string
	Position int
}

type Format struct {
	ID            string
	Name          string
	TopHours      int
	DurationHours int
	Position      int
}

type Price struct {
	Amount    int64 // цена блогера без комиссии, копейки
	IsEnabled bool
}

type Unavailability struct {
	ID       uuid.UUID
	StartsAt time.Time
	EndsAt   time.Time
}

type Channel struct {
	ID             uuid.UUID
	OrgID          *uuid.UUID
	MaxChatID      int64
	Title          string
	MaxDescription string
	Description    string
	IconURL        string
	Link           string
	IsPublic       bool
	CategoryID     *uuid.UUID
	Status         string
	StatusReason   string
	BotIsAdmin     bool
	BotPermissions []string
	Subscribers    int
	AvgViews       int
	PostsPerWeek   float64
	RknNumber      string
	RknStatus      string
	Rating         int
	AnomalyFlags   []string
	Prices         map[string]*Price // ключ — format_id
	Unavailability []Unavailability
	CreatedAt      time.Time
}

type LinkCode struct {
	Code      string
	OrgID     uuid.UUID
	ExpiresAt time.Time
}

type CartItem struct {
	ID         uuid.UUID
	OrgID      uuid.UUID
	ChannelID  uuid.UUID
	FormatID   string
	ShownTotal int64
	CreatedAt  time.Time
}

type MediaFile struct {
	ID        uuid.UUID
	OrgID     uuid.UUID
	Name      string
	Mime      string
	SizeBytes int64
	Width     int
	Height    int
	DeletedAt *time.Time
	CreatedAt time.Time
}

type PostDraft struct {
	Doc           api.EditorDoc
	MediaIDs      []uuid.UUID
	TokenCarrier  string
	TokenPosition string
	UpdatedAt     time.Time
}

type Campaign struct {
	ID                uuid.UUID
	OrgID             uuid.UUID
	Name              string
	Status            string
	DateFrom          *time.Time // дата без времени, 00:00 UTC
	DateTo            *time.Time
	TimeFrom          *string // ЧЧ:ММ, МСК
	TimeTo            *string
	ExtraRequirements *string
	OrderID           *uuid.UUID
	Post              PostDraft
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type PostVersion struct {
	ID              uuid.UUID
	Number          int
	Status          string
	Doc             api.EditorDoc
	Body            string
	FinalText       string
	Erid            string
	MediaIDs        []uuid.UUID
	TokenCarrier    string
	TokenPosition   string
	AdvertiserName  string
	AdvertiserINN   string
	RejectionReason string
	CreatedAt       time.Time
}

type Order struct {
	ID          uuid.UUID
	Number      int64
	OrgID       uuid.UUID
	CampaignID  uuid.UUID
	Versions    []*PostVersion // последняя — текущая (order.current_post_version_id)
	Status      string         // производное, 03 §1.2
	TotalAmount int64
	CreatedAt   time.Time
}

// CurrentVersion возвращает текущую версию поста заказа.
func (o *Order) CurrentVersion() *PostVersion {
	return o.Versions[len(o.Versions)-1]
}

// Version ищет версию по идентификатору.
func (o *Order) Version(id uuid.UUID) *PostVersion {
	for _, v := range o.Versions {
		if v.ID == id {
			return v
		}
	}
	return nil
}

type Claim struct {
	Reason    string
	Text      string
	CreatedAt time.Time
}

type HistoryEntry struct {
	Transition string
	From       string
	To         string
	ActorType  string
	ReasonCode string
	CreatedAt  time.Time
}

type Placement struct {
	ID                 uuid.UUID
	OrderID            uuid.UUID
	ChannelID          uuid.UUID
	BloggerOrgID       uuid.UUID
	FormatID           string
	TopHours           int
	DurationHours      int
	WindowStart        time.Time
	WindowEnd          time.Time
	TimeFrom           string
	TimeTo             string
	PostVersionID      uuid.UUID
	Status             string
	StatusReason       string
	StatusChangedAt    time.Time
	FeeRateBP          int
	BloggerAmount      int64
	FeeAmount          int64
	TotalAmount        int64
	HoldActive         bool // активное удержание (I-03)
	AcceptDeadlineAt   *time.Time
	ScheduleDeadlineAt *time.Time
	PublishAt          *time.Time
	RemoveAt           *time.Time
	PublishedAt        *time.Time
	RemovedAt          *time.Time
	ClaimDeadlineAt    *time.Time
	PerformanceShare   *int
	PayoutAmount       *int64
	FeeCharged         *int64
	RefundAmount       *int64
	Views              *int
	PublicationURL     string
	Claim              *Claim
	History            []HistoryEntry
	CreatedAt          time.Time
}

type Invoice struct {
	ID         uuid.UUID
	Number     string
	OrgID      uuid.UUID
	Amount     int64
	Status     string
	ExpiresAt  time.Time
	PaidAt     *time.Time
	PaidAmount *int64
	CreatedAt  time.Time
}

// LedgerEntry — движение по балансу рекламодателя. Полный план счетов
// (06 §2) заглушке не нужен: хранится только то, что видит рекламодатель.
type LedgerEntry struct {
	ID          uuid.UUID
	OrgID       uuid.UUID
	Kind        string // deposit, hold, hold_release, refund
	Amount      int64  // со знаком
	Description string
	OrderID     *uuid.UUID
	PlacementID *uuid.UUID
	InvoiceID   *uuid.UUID
	CreatedAt   time.Time
}

type Payout struct {
	ID          uuid.UUID
	PlacementID uuid.UUID
	OrgID       uuid.UUID
	Amount      int64
	Status      string
	Attempt     int
	ReceiptURL  string
	CreatedAt   time.Time
	CompletedAt *time.Time
}

type RocketworkContractor struct {
	Status        string // none, invited, pending, active, blocked
	OnboardingURL string
}

// Data — всё изменяемое состояние заглушки. Сессии хранятся отдельно,
// чтобы переживать POST /_mock/reset.
type Data struct {
	Users         map[uuid.UUID]*User
	Orgs          map[uuid.UUID]*Organization
	LegalDocs     []*LegalDocument
	Acceptances   map[uuid.UUID]map[uuid.UUID]time.Time // org → документ → когда принят
	Notifications []*Notification
	Categories    []*Category
	Formats       []*Format
	Channels      []*Channel
	LinkCodes     []*LinkCode
	CartItems     []*CartItem
	Media         []*MediaFile
	Campaigns     []*Campaign
	Orders        []*Order
	Placements    []*Placement
	Invoices      []*Invoice
	Ledger        []*LedgerEntry
	Payouts       []*Payout
	Rocketwork    map[uuid.UUID]*RocketworkContractor
	NextOrderNo   int64
	NextInvoiceNo int64
	NextMaxChatID int64
}

// NewData создаёт пустое состояние.
func NewData() *Data {
	return &Data{
		Users:         map[uuid.UUID]*User{},
		Orgs:          map[uuid.UUID]*Organization{},
		Acceptances:   map[uuid.UUID]map[uuid.UUID]time.Time{},
		Rocketwork:    map[uuid.UUID]*RocketworkContractor{},
		NextOrderNo:   1001,
		NextInvoiceNo: 101,
		NextMaxChatID: -70000000100,
	}
}
