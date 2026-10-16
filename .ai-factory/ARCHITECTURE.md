# Architecture: Explicit Architecture (модульный монолит, по слоям)

## Overview

Бэкенд — модульный монолит на Go: один исполняемый файл с режимами `api` и `worker` и одна база PostgreSQL (07 §1). Код делится на модули по группам модели данных (04 §2–§8). Внутри модуля есть четыре слоя: `domain`, `app`, адаптеры (`postgres`, `httpapi`, `jobs`) и точка сборки в `cmd/`. Зависимости направлены только внутрь, к `domain`.

Такой выбор объясняется предметной областью. Машина состояний размещения (P-01 … P-18), инварианты I-01 … I-13, журнал проводок, расчёт долей и возвратов — это правила, ошибка в которых стоит денег. Поэтому они живут в чистом `domain` без базы, сети и журнала и проверяются быстрыми тестами. Внешние системы (MAX, Яндекс ОРД, Рокет Ворк, почта, хранилище) подключаются через порты. По 07 §9 в тестах вместо них работают поддельные реализации, а база настоящая.

Микросервисы не нужны: команда маленькая, объёмы скромные (07 §7), а сценарии вроде «оформить заказ и удержать деньги» требуют одной транзакции на несколько модулей.

## Decision Rationale

- **Project type:** бэкенд биржи рекламы в каналах MAX: HTTP API кабинетов, приём Webhook, планировщик заданий, интеграции.
- **Tech stack:** Go 1.25+, PostgreSQL, S3-совместимое хранилище (D-09, A-26). Библиотеки — в разделе «Технические решения».
- **Key factor:** сложные денежные и юридические правила, которые 07 §9 требует собрать в одном месте и покрыть тестами без внешних систем; код пишут AI-агенты по ТЗ этапов, поэтому границы должны быть видны по путям импорта.

## Технические решения

Выбор библиотек, отложенный в 07 §1 до этапа «Каркас бэкенда и аккаунты». Смена любой строки — отдельное решение с записью сюда.

| Задача | Решение | Почему |
|---|---|---|
| Модуль Go | `github.com/leha-creator/maxim` в корне репозитория | `mockapi/` остаётся отдельным модулем и в сборку не входит |
| HTTP | `net/http` (`ServeMux` Go 1.22+), код из `api/openapi.yaml` через `oapi-codegen` v2: модели + strict server | Как в `mockapi`; сервер и спецификация не расходятся |
| Проверка запросов | `oapi-codegen/nethttp-middleware` по встроенной спецификации | Схема проверяется до обработчика |
| PostgreSQL | `jackc/pgx/v5` (пул `pgxpool`) | Нативные типы, `FOR UPDATE SKIP LOCKED`, `LISTEN/NOTIFY` при необходимости |
| Запросы | `sqlc`: SQL пишется руками, Go-код генерируется | Видно каждый запрос и блокировку; без ORM |
| Миграции | `pressly/goose/v3`, SQL-файлы, встроены в бинарник, команда `maxim migrate` | 07 §8: миграции до запуска новой версии |
| Идентификаторы | `google/uuid`, `uuid.NewV7()` | 04 §1: UUID v7 создаёт приложение |
| Конфигурация | `caarlos0/env/v11` + своя проверка при запуске | 07 §8: только окружение, не стартовать при ошибке |
| Журнал | `log/slog`, JSON в stdout | 07 §5.1 |
| Метрики | `prometheus/client_golang`, отдельный адрес `/metrics` | 07 §5.5 |
| Хранилище | `minio/minio-go/v7` | Любое S3-совместимое, подписанные ссылки |
| Изображения | `golang.org/x/image` (WebP, шрифты), `disintegration/imaging` | Плашка и превью без внешних процессов (D-04) |
| Тесты с базой | `testcontainers-go` (модуль postgres), пакет `platform/dbtest`: контейнер на тестовый пакет, миграции от владельца, тесты под ролью приложения | 07 §9: настоящая PostgreSQL; права роли проверяются тестами |
| Почта | `wneessen/go-mail` (SMTP), шаблоны `templates/email/*.tmpl` | 07 §6: тексты в репозитории; в разработке — Mailpit |
| Секреты в базе | `platform/secretbox`: AES-256-GCM, ключ `APP_SECRETS_KEY` | 07 §4: секрет в базе — только зашифрованным (R-1) |
| Права роли в миграциях | Параметр сессии `maxim.app_role`, который выставляет `maxim migrate` из `DB_APP_ROLE` | Миграции остаются разбираемыми `sqlc` |
| Клиент MAX | Свой тонкий HTTP-клиент поверх `platform/outbound`, без `max-bot-api-client-go` | Каждая попытка пишется в `outbound_request`; повторы и предохранитель общие для всех систем (05 §2.1) |
| Ограничитель и общее состояние интеграций | Таблицы PostgreSQL: `rate_limit_bucket`, `outbound_circuit`, `alert`, `worker_heartbeat` | 07 §1: общее состояние экземпляров — только в базе; брокер и Redis не нужны |

## Folder Structure

```
maxim/
├── api/
│   └── openapi.yaml                 # ЕДИНСТВЕННЫЙ договор HTTP API (кабинеты, администратор, Webhook)
├── cmd/
│   └── maxim/
│       └── main.go                  # точка сборки: режимы api | worker | migrate
├── internal/
│   ├── apigen/                      # СГЕНЕРИРОВАНО из api/openapi.yaml (тег _mock исключён). Руками не править
│   │   ├── generate.go
│   │   └── api.gen.go
│   │
│   ├── bootstrap/                   # СБОРКА: конфиг, пулы, адаптеры, модули, HTTP-сервер, worker
│   │   ├── config.go                #   вся конфигурация и её проверка (07 §8)
│   │   ├── api.go                   #   режим api
│   │   ├── worker.go                #   режим worker, регистрация видов заданий
│   │   └── server.go                #   сборка strict server из обработчиков модулей
│   │
│   ├── platform/                    # ── ОБЩЕЕ ЯДРО И ИНФРАСТРУКТУРА (без предметных правил) ──
│   │   ├── clock/                   #   Clock: подменяемое текущее время (07 §9)
│   │   ├── money/                   #   Kopecks (int64), доли, округление вниз (06 §3)
│   │   ├── ids/                     #   UUID v7
│   │   ├── apperr/                  #   коды ошибок: validation_failed, not_found, invalid_transition…
│   │   ├── tx/                      #   Runner: транзакция в context; интерфейс + реализация на pgx
│   │   ├── db/                      #   пул pgxpool, helpers для sqlc
│   │   ├── job/                     #   очередь на таблице job: Enqueue (в той же tx), Worker, повторы (07 §2)
│   │   ├── inbound/                 #   запись и разбор inbound_event, дедупликация
│   │   ├── outbound/                #   обёртка внешних вызовов: outbound_request, повторы, предохранитель (05 §1)
│   │   ├── ratelimit/               #   общий ограничитель на корзинах токенов в базе, приоритеты (05 §2.8)
│   │   ├── alert/                   #   оповещения администратору: почта и чат дежурных, не чаще раза в час (07 §3)
│   │   ├── stuck/                   #   поиск зависших объектов раз в час; проверки регистрируют модули (07 §3)
│   │   ├── cleanup/                 #   ежесуточная очистка; запросы удаления остаются у владельцев таблиц
│   │   ├── audit/                   #   запись audit_log
│   │   ├── logx/                    #   slog: поля из context, маскирование (07 §5.4)
│   │   ├── metrics/                 #   реестр Prometheus, HTTP-метрики (07 §5.5)
│   │   ├── secretbox/               #   шифрование секретов в полезной нагрузке заданий (R-1)
│   │   ├── dbtest/                  #   PostgreSQL в testcontainers для тестов
│   │   ├── httpx/                   #   middleware: request_id, сессия, CSRF, лимиты, ошибки → JSON
│   │   └── storage/                 #   порт ObjectStorage + реализация на minio-go; storagefake/
│   │
│   ├── integration/                 # ── АДАПТЕРЫ ВНЕШНИХ СИСТЕМ ──
│   │   ├── max/                     #   клиент MAX Bot API через outbound и ratelimit (25 rps, 1/с на чат), разбор Webhook
│   │   │   └── maxfake/             #   поддельная реализация для тестов и разработки
│   │   ├── ord/                     #   Яндекс ОРД
│   │   │   └── ordfake/
│   │   ├── rocketwork/
│   │   │   └── rocketworkfake/
│   │   └── mail/
│   │       └── mailfake/
│   │
│   ├── account/                     # ── МОДУЛЬ: аккаунты и организации (04 §2) ──
│   │   ├── domain/                  #   сущности, правила legal_type → tax_regime → vat_rate, порты
│   │   ├── app/                     #   сценарии: запросить код, войти, создать организацию…
│   │   ├── postgres/                #   реализация портов-репозиториев
│   │   │   ├── queries/*.sql        #   исходники sqlc
│   │   │   └── sqlc/                #   СГЕНЕРИРОВАНО
│   │   ├── httpapi/                 #   обработчики операций OpenAPI этого модуля
│   │   ├── accounttest/             #   тестовые данные модуля (корректные ИНН, реквизиты)
│   │   └── jobs/                    #   обработчики видов заданий модуля (если есть)
│   │
│   ├── channel/                     # каналы, метрики, категории, форматы, цены, каталог (04 §3)
│   ├── post/                        # медиатека, кампании, пост, версии, сериализация в MAX (04 §4)
│   ├── placement/                   # корзина, заказ, размещения, публикации, нарушения, споры (04 §5)
│   ├── marking/                     # объекты ОРД, erid, отчётность (04 §6)
│   ├── ledger/                      # счета учёта, проводки, удержания — ЕДИНСТВЕННЫЙ вход в ledger_* (06 §2)
│   ├── billing/                     # счета, выписки, сопоставление поступлений, закрывающие акты (04 §7)
│   ├── payout/                      # исполнители Рокет Ворк, выплаты, контроль депозита (04 §7)
│   └── notification/                # уведомления, шаблоны, отправка (04 §8, 07 §6)
│
├── migrations/                      # goose, одна схема на всё приложение: 00001_init.sql …
├── sqlc.yaml                        # по записи на модуль: queries → <module>/postgres/sqlc
├── templates/                       # тексты уведомлений и писем (07 §6): email/*.tmpl, embed.go
├── compose.dev.yml                  # локальное окружение: PostgreSQL, MinIO, Mailpit, migrate, api, worker
├── Dockerfile                       # образ бэкенда (distroless); контекст — Dockerfile.dockerignore
├── docs/                            # проектная документация (справочно)
└── mockapi/                         # тестовый сервер в памяти (справочно, отдельный модуль)
```

Все модули устроены одинаково (`domain`, `app`, `postgres`, `httpapi`, `jobs`). Пустой слой не создаётся.

## Dependency Rules

```
cmd/maxim ─► bootstrap ─► всё остальное (только сборка)

<module>/httpapi ─┐
<module>/jobs    ─┼─► <module>/app ─► <module>/domain ─► platform/{money, ids, clock, apperr}
<module>/postgres┘                          ▲
         │                                  │ реализует порты
integration/* ──────────────────────────────┘
```

- ✅ `domain` импортирует только стандартную библиотеку и `platform/{money, ids, clock, apperr}`.
- ✅ `app` импортирует свой `domain` и `platform/{tx, job, audit, logx, clock, money, ids, apperr}`, причём из `tx` и `job` — только интерфейсы.
- ✅ `postgres` импортирует свой `domain`, свой `sqlc` и `platform/{db, tx}`.
- ✅ `httpapi` импортирует свой `app`, свой `domain` (типы для преобразования) и `apigen`.
- ✅ `jobs` импортирует свой `app` и `platform/job`.
- ✅ `integration/*` реализуют порты: импортируют `platform/{outbound, ratelimit, job, alert, logx, clock}` и не импортируют модули. Из `job` — только `Retry`/`Permanent`, из `alert` — тип `Notice`.
- ✅ Пакеты `platform/` друг от друга зависят только «вниз»: `alert`, `stuck`, `cleanup`, `inbound` используют `job`, `tx`, `db`; `job` и `outbound` об оповещениях знают только через свой интерфейс `Alerter`.
- ✅ Только `bootstrap` знает обо всех пакетах и связывает порты с реализациями.
- ❌ `domain` не импортирует `pgx`, `net/http`, `log/slog`, `apigen`, `integration/*` и любой `app`.
- ❌ Модуль не импортирует внутренности другого модуля (`placement` → `channel/postgres`, `channel/domain`). Связь идёт через порт (см. ниже).
- ❌ `httpapi` не вызывает `postgres` в обход `app`.
- ❌ `app` и `httpapi` не вызывают `integration/*`. Внешний вызов делается из обработчика задания через порт (07 §1, 07 §9).
- ❌ Никто, кроме `ledger/postgres`, не пишет в `ledger_account`, `ledger_transaction`, `ledger_entry`, `hold`.
- ❌ Никто, кроме `placement/domain`, не меняет `placement.status`.

## Layer/Module Communication

- **HTTP → модуль.** `oapi-codegen` генерирует один интерфейс `apigen.StrictServerInterface`. Каждый `<module>/httpapi.Handlers` реализует методы своих операций. `bootstrap/server.go` собирает их встраиванием структур в одну реализацию интерфейса. Имена методов совпадают с `operationId`, поэтому конфликтов нет.
- **Модуль → модуль, синхронно.** Потребитель объявляет порт в своём `domain` на своём языке, например `placement/domain.ChannelPricing`. `bootstrap` передаёт реализацию: сервис `channel/app`, если сигнатуры совпадают, иначе тонкий переходник в `bootstrap`. Циклов импорта нет, потому что модули друг друга не импортируют.
- **Общая транзакция.** `tx.Runner.InTx(ctx, fn)` кладёт транзакцию в `context`. Все репозитории и `job.Enqueue` внутри `fn` работают в ней. Так оформление заказа, удержание в журнале проводок и создание заданий выполняются атомарно (I-10).
- **Модуль → модуль, асинхронно.** Последствие перехода, которое не обязано выполниться в той же транзакции, ставится заданием в той же транзакции, что и переход. Обработчик задания в другом модуле идемпотентен (03 §11).
- **Модуль → внешняя система.** Порт объявлен в `domain` модуля, например `placement/domain.ChannelPublisher`. Реализация — `integration/max`. Вызывает его только обработчик задания из `<module>/jobs` через `app`. Каждый вызов проходит через `platform/outbound`, который пишет `outbound_request`, повторяет запрос и маскирует секреты.
- **Входящие события.** `httpapi` проверяет подлинность Webhook, записывает `inbound_event` и ставит задание `process_inbound_event`. Затем отвечает `200`. Разбор и действия выполняются в `jobs` (07 §7: ответ за 200 мс).
- **Доступ по организации.** Методы репозиториев для данных организации принимают `organizationID` явным параметром. Выборки без него для кабинетных сценариев не пишутся (07 §4). Администраторские выборки — отдельные методы с суффиксом `ForAdmin`, вызываются только после проверки роли.

## Key Principles

1. **Названия из глоссария.** Пакеты, типы, поля, константы перечислений, коды переходов (`P01AcceptByBlogger`), таймеров и причин совпадают с `docs/glossary.md` и 04 §9 без переименований.
2. **Одна таблица переходов.** Таблица живёт в `placement/domain/transitions.go` и сверяется с 03 §4. Состояние меняет только `Placement.Apply`. Результат применения — новое состояние, запись истории и список заданий, которые `app` сохраняет в той же транзакции.
3. **Деньги — `money.Kopecks` (`int64`).** `float` для денег запрещён. Доли и возвраты считаются функциями в `domain` по 03 §7 и 06 §3, округление вниз.
4. **Время — только через `clock.Clock`.** `time.Now()` вне `platform/clock` запрещён: таймеры и сроки должны проверяться тестами.
5. **Идемпотентность по умолчанию.** Каждый обработчик задания и входящего события сначала читает текущее состояние и выходит, если действие уже выполнено. Денежные операции несут ключ идемпотентности (06 §2.2).
6. **Метки реестра в коде.** Место, реализующее допущение или рабочий ответ на вопрос, помечается комментарием `// A-10`, `// Q-03`. Параметры допущений берутся из конфигурации (README, «Метки»).
7. **Журнал по 07 §5.** Пишет слой `app` и адаптеры, `domain` не пишет. Сообщение — постоянная строка на английском. Операция — в поле `op` в виде `Модуль.Операция`. Обязательные события 07 §5.3 — часть критериев приёмки.
8. **Спецификация первична.** Новая операция сначала появляется в `api/openapi.yaml`, затем `go generate ./internal/apigen`, затем обработчик. Тестовый сервер `mockapi` при этом не обязан обновляться.

## Code Examples

### Машина состояний в `domain`: чистые правила, без базы и журнала

```go
// internal/placement/domain/transitions.go
package domain

import (
	"time"

	"github.com/leha-creator/maxim/internal/platform/apperr"
)

type Status string

const (
	StatusAwaitingBlogger  Status = "awaiting_blogger"
	StatusAwaitingSchedule Status = "awaiting_schedule"
	StatusDeclined         Status = "declined"
	// … полный список — 04 §9
)

type TransitionCode string

const (
	P03AcceptByBlogger  TransitionCode = "P-03"
	P04DeclineByBlogger TransitionCode = "P-04"
	// … P-01 … P-18 — 03 §4
)

type transition struct {
	from  []Status
	to    Status
	actor ActorType
}

// Единственная таблица переходов (07 §9). Сверяется с 03 §4 тестом.
var transitions = map[TransitionCode]transition{
	P03AcceptByBlogger:  {from: []Status{StatusAwaitingBlogger}, to: StatusAwaitingSchedule, actor: ActorBlogger},
	P04DeclineByBlogger: {from: []Status{StatusAwaitingBlogger}, to: StatusDeclined, actor: ActorBlogger},
}

// Apply — единственный способ сменить состояние размещения.
func (p *Placement) Apply(code TransitionCode, actor ActorType, now time.Time) (StateChange, error) {
	t, ok := transitions[code]
	if !ok || t.actor != actor || !t.allows(p.Status) {
		return StateChange{}, apperr.InvalidTransition(string(code), string(p.Status))
	}
	change := StateChange{PlacementID: p.ID, Code: code, From: p.Status, To: t.to, At: now}
	p.Status = t.to
	p.LockVersion++
	change.Jobs = p.jobsAfter(code, now) // таймеры 03 §5 — значения, а не вызовы
	return change, nil
}
```

### Сценарий в `app`: транзакция, порты, журнал

```go
// internal/placement/app/accept.go
package app

func (s *Service) AcceptByBlogger(ctx context.Context, orgID, placementID uuid.UUID) error {
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		p, err := s.placements.GetForUpdate(ctx, orgID, placementID) // выборка ограничена организацией
		if err != nil {
			return err
		}
		change, err := p.Apply(domain.P03AcceptByBlogger, domain.ActorBlogger, s.clock.Now())
		if err != nil {
			s.log.WarnContext(ctx, "placement transition rejected",
				"op", "Placement.AcceptByBlogger", "placement_id", placementID, "transition", domain.P03AcceptByBlogger)
			return err
		}
		if err := s.placements.Save(ctx, p, change); err != nil { // placement + placement_state_history
			return err
		}
		if err := s.jobs.Enqueue(ctx, change.Jobs...); err != nil { // в той же транзакции (I-10)
			return err
		}
		s.log.InfoContext(ctx, "placement transition applied",
			"op", "Placement.AcceptByBlogger", "placement_id", placementID,
			"transition", change.Code, "from", change.From, "to", change.To)
		return nil
	})
}
```

### Порт объявляет потребитель, реализацию подставляет `bootstrap`

```go
// internal/placement/domain/ports.go
package domain

// ChannelPublisher — публикация и снятие поста в канале. Реализация: integration/max.
// Вызывается только из обработчиков заданий publish_placement / remove_placement.
type ChannelPublisher interface {
	Publish(ctx context.Context, req PublishRequest) (PublishedMessage, error)
	Remove(ctx context.Context, channelExternalID, messageID string) error
}

// ChannelPricing — цена формата в канале. Реализация: сервис channel/app.
type ChannelPricing interface {
	Price(ctx context.Context, channelID uuid.UUID, format string) (money.Kopecks, error)
}
```

```go
// internal/bootstrap/worker.go (фрагмент)
maxClient := max.NewClient(cfg.MAX, outbound.New(pool, log, clk)) // или maxfake.New() в разработке
placementSvc := placementapp.New(placementapp.Deps{
	Tx:        txRunner,
	Repo:      placementpg.New(pool),
	Jobs:      jobQueue,
	Publisher: maxClient,             // integration/max реализует domain.ChannelPublisher
	Pricing:   channelapp.New(/*…*/), // structural typing: метод Price совпадает
	Clock:     clk,
	Log:       log,
})
jobs.Register(worker, placementSvc) // publish_placement, remove_placement, placement_timer
```

### Нарушение: внешний вызов из HTTP-обработчика

```go
// ❌ httpapi не должен ходить во внешние системы и менять статус напрямую
func (h *Handlers) PublishNow(ctx context.Context, req apigen.PublishNowRequestObject) (apigen.PublishNowResponseObject, error) {
	h.max.SendMessage(ctx, ...)                               // нарушает 07 §1: только через задание
	h.db.Exec(ctx, "UPDATE placement SET status='published'") // нарушает 07 §9: только через Apply
	...
}
```

## Тестирование

| Слой | Чем | Что |
|---|---|---|
| `domain` | Обычные табличные тесты, без базы | Каждый переход P-01 … P-18 из каждого состояния; инварианты; формулы 03 §7, 06 §3 на граничных долях 0, 1, 9999, 10000; сериализатор поста (05 §2.5) |
| `app` + `postgres` | `testcontainers-go`, настоящая PostgreSQL, миграции goose, поддельные интеграции, `clock` с ручным ходом | Сценарии, таймеры, идемпотентность повторов, строки таблицы проводок 06 §2.3 и сходимость |
| `httpapi` | `httptest` поверх собранного сервера | Коды ответов, проверка организации, CSRF, соответствие спецификации |
| `integration/*` | `httptest`-сервер с записанными ответами | Разбор ответов и ошибок, маскирование, повторы |

Тест, сверяющий таблицу переходов с 03 §4 и перечисления с 04 §9, обязателен.

## Anti-Patterns

- ❌ **Анемичное размещение.** Проверки состояния и расчёт сумм в `app` или `httpapi` вместо методов `domain`.
- ❌ **Прямой `UPDATE placement SET status`** или запись в `ledger_entry` в обход владельца.
- ❌ **Внешний вызов в транзакции базы или в HTTP-запросе кабинета.** Внешние вызовы — только в заданиях, вне открытой транзакции.
- ❌ **Импорт чужого модуля** ради одного типа или функции. Объявите порт у себя и свяжите в `bootstrap`.
- ❌ **Общий пакет `utils/` или `common/`** с предметной логикой. В `platform/` — только то, что не знает о рекламе, каналах и деньгах сверх типа `Kopecks`.
- ❌ **`time.Now()`, `uuid.New()` (v4), `float64` для денег** в коде модулей.
- ❌ **Правка `apigen/` или `sqlc/` руками.** Меняется спецификация или SQL, затем генерация.
- ❌ **Логирование текста поста, ИНН, почты без маскирования, токенов и кодов** (07 §5.4).
- ❌ **Глобальные переменные с состоянием** (пулы, клиенты, конфигурация). Зависимости передаются через конструкторы из `bootstrap`.
