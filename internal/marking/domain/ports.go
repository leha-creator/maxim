package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Receipt — ответ ОРД на POST и GET /status (ResponseStatus).
type Receipt struct {
	RequestID      string
	Status         string
	Token          string
	ObjectStatuses []ObjectStatus
	Errors         []string
}

// ObjectStatus — статус одного объекта запроса.
type ObjectStatus struct {
	ObjectID string
	Status   string
	Token    string
	Errors   []string
}

// For — статус, токен и ошибки объекта externalID: из object_statuses[] по нашему
// идентификатору, иначе общие (R-3).
func (r Receipt) For(externalID string) (status, token string, errs []string) {
	for _, o := range r.ObjectStatuses {
		if o.ObjectID == externalID {
			token = o.Token
			if token == "" {
				token = r.Token
			}
			return o.Status, token, append(append([]string(nil), o.Errors...), r.Errors...)
		}
	}
	return r.Status, r.Token, r.Errors
}

// OrdAPI — Яндекс ОРД, API v8 (05 §3). Реализация: integration/ord (переходник в bootstrap/bridge)
// или ordfake. Вызывается только из обработчиков заданий вне транзакции (I-13). Ошибки —
// в терминах планировщика: job.Permanent — отказ ОРД (4xx, 422), прочие — повтор.
type OrdAPI interface {
	UpsertOrganization(ctx context.Context, req OrganizationRequest) (Receipt, error)
	UpsertContract(ctx context.Context, req ContractRequest) (Receipt, error)
	UpsertPlatform(ctx context.Context, req PlatformRequest) (Receipt, error)
	UpsertCreative(ctx context.Context, req CreativeRequest) (Receipt, error)
	GetStatus(ctx context.Context, requestID string) (Receipt, error)
}

// Organizations — реквизиты организаций для ОРД. Реализация: переходник над account/postgres.
type Organizations interface {
	// Details — реквизиты организации; ErrNotFound — нет.
	Details(ctx context.Context, orgID uuid.UUID) (OrgDetails, error)
	// Platform — организация-платформа (organization.is_platform, R-5); ErrNotFound — нет.
	Platform(ctx context.Context) (OrgDetails, error)
	// FirstOfferAcceptance — первое принятие оферты вида kind (advertiser_offer, blogger_offer);
	// false — организация её не принимала.
	FirstOfferAcceptance(ctx context.Context, orgID uuid.UUID, kind string) (OfferAcceptance, bool, error)
}

// CreativeSource — содержание версии поста для креатива (R-8). Реализация: post/app.CreativeContent.
type CreativeSource interface {
	CreativeContent(ctx context.Context, versionID uuid.UUID) (CreativeContent, error)
}

// VersionMarker — перевод версии в ready с erid (post/app.MarkVersionReady) в транзакции
// вызывающего; последствия для размещений (P-02) выполняет слушатель версии модуля размещений.
type VersionMarker interface {
	MarkVersionReady(ctx context.Context, versionID uuid.UUID, erid string) error
}

// ChannelPlatformReady — площадка канала зарегистрирована: канал становится виден в каталоге
// (R-14). Реализация: channel/app.SetOrdPlatformReady.
type ChannelPlatformReady interface {
	SetOrdPlatformReady(ctx context.Context, channelID uuid.UUID, at time.Time) error
}

// CreativeListener — ЕРИР отклонил креатив с erid (S-28 шаг 2, R-12). Реализация:
// placement/app.CreativeRejected; вызывается в транзакции перехода креатива в error.
// published — уже опубликованные размещения версии: решение по ним принимает администратор.
type CreativeListener interface {
	CreativeRejected(ctx context.Context, versionID uuid.UUID) (published []uuid.UUID, err error)
}

// NopCreativeListener — модуль размещений не подключён.
type NopCreativeListener struct{}

func (NopCreativeListener) CreativeRejected(context.Context, uuid.UUID) ([]uuid.UUID, error) {
	return nil, nil
}

// OrdOrganizations — записи ord_organization.
type OrdOrganizations interface {
	// ByOrganization — запись организации без блокировки; ErrNotFound — нет.
	ByOrganization(ctx context.Context, orgID uuid.UUID) (Organization, error)
	// GetForUpdate блокирует запись; ErrNotFound — нет.
	GetForUpdate(ctx context.Context, id uuid.UUID) (Organization, error)
	// Insert вставляет запись; при гонке возвращает уже существующую и created = false.
	Insert(ctx context.Context, o Organization) (rec Organization, created bool, err error)
	Save(ctx context.Context, o Organization) error
}

// OrdContracts — записи ord_contract.
type OrdContracts interface {
	// ByParties — договор вида по сторонам без блокировки; ErrNotFound — нет.
	ByParties(ctx context.Context, kind ContractKind, client, contractor uuid.UUID) (Contract, error)
	GetForUpdate(ctx context.Context, id uuid.UUID) (Contract, error)
	Insert(ctx context.Context, c Contract) (rec Contract, created bool, err error)
	Save(ctx context.Context, c Contract) error
}

// OrdPlatforms — записи ord_platform.
type OrdPlatforms interface {
	ByChannel(ctx context.Context, channelID uuid.UUID) (Platform, error)
	GetForUpdate(ctx context.Context, id uuid.UUID) (Platform, error)
	Insert(ctx context.Context, p Platform) (rec Platform, created bool, err error)
	Save(ctx context.Context, p Platform) error
}

// Creatives — записи creative.
type Creatives interface {
	ByVersion(ctx context.Context, versionID uuid.UUID) (Creative, error)
	GetForUpdate(ctx context.Context, id uuid.UUID) (Creative, error)
	Insert(ctx context.Context, c Creative) (rec Creative, created bool, err error)
	Save(ctx context.Context, c Creative) error
	// Delete удаляет креатив без отправки (R-11).
	Delete(ctx context.Context, id uuid.UUID) error
}

// ErrorItem — объект в error для списка S-20.
type ErrorItem struct {
	Ref        ObjectRef
	ExternalID string
	OrdStatus  string
	Error      string
	Attempts   int
	UpdatedAt  time.Time
	// LastRequestID — последний вызов ОРД по объекту в outbound_request; uuid.Nil — вызовов не было.
	LastRequestID uuid.UUID
}

// RequestLog — вызов ОРД из outbound_request: тела — как сохранены адаптером (ИНН с маской).
type RequestLog struct {
	ID             uuid.UUID
	Operation      string
	Attempt        int
	Request        []byte
	ResponseStatus *int
	Response       []byte
	Error          string
	CreatedAt      time.Time
}

// ErrorCursor — ключ страницы списка ошибок: (updated_at, id).
type ErrorCursor struct {
	UpdatedAt time.Time
	ID        uuid.UUID
}

// StuckObject — объект в sent дольше порога (07 §3).
type StuckObject struct {
	Ref    ObjectRef
	SentAt time.Time
}

// Overview — выборки по всем видам объектов: список S-20, зависшие, метрика.
type Overview interface {
	// ListErrors — объекты в error по (updated_at, id) после курсора.
	ListErrors(ctx context.Context, typ ObjectType, after *ErrorCursor, limit int) ([]ErrorItem, error)
	// LastRequest — последний вызов ОРД по объекту; nil — вызовов не было (этап «API администратора», R-17).
	LastRequest(ctx context.Context, ref ObjectRef, externalID string) (*RequestLog, error)
	// StuckSent — объекты в sent с sent_at раньше before.
	StuckSent(ctx context.Context, before time.Time, limit int) ([]StuckObject, error)
	// CreativesWithoutErid — креативы без erid, созданные раньше before (T-11).
	CreativesWithoutErid(ctx context.Context, before time.Time, limit int) ([]Creative, error)
	// CountByStatus — число объектов по виду и состоянию.
	CountByStatus(ctx context.Context) (map[ObjectType]map[SyncStatus]int, error)
}

// Config — параметры модуля (07 §8).
type Config struct {
	// KKTUCode — код ККТУ всех креативов (ORD_KKTU_CODE, A-42, Q-39).
	KKTUCode string
	// PlatformType — тип площадки канала (ORD_PLATFORM_TYPE, A-32, Q-13).
	PlatformType string
	Contracts    ContractRules
	Poll         PollRules
	// DependencyWait — ожидание неготовой зависимости (ORD_DEPENDENCY_WAIT).
	DependencyWait time.Duration
	// DependencyWaitFailed — ожидание зависимости в error (ORD_DEPENDENCY_WAIT_FAILED).
	DependencyWaitFailed time.Duration
	// EridWait — креатив без erid дольше — оповещение T-11 (ORD_ERID_WAIT).
	EridWait time.Duration
	// StuckSentAge — объект в sent дольше — зависший (STUCK_ORD_SENT_AGE).
	StuckSentAge time.Duration
}

// WithDefaults подставляет значения по умолчанию.
func (c Config) WithDefaults() Config {
	if c.KKTUCode == "" {
		c.KKTUCode = "30.15.1" // только для разработки и тестов: в production задаётся явно (A-42)
	}
	if c.PlatformType == "" {
		c.PlatformType = "site" // A-32, Q-13
	}
	c.Contracts = c.Contracts.WithDefaults()
	c.Poll = c.Poll.WithDefaults()
	if c.DependencyWait <= 0 {
		c.DependencyWait = 30 * time.Second
	}
	if c.DependencyWaitFailed <= 0 {
		c.DependencyWaitFailed = 10 * time.Minute
	}
	if c.EridWait <= 0 {
		c.EridWait = time.Hour
	}
	if c.StuckSentAge <= 0 {
		c.StuckSentAge = 24 * time.Hour
	}
	return c
}
