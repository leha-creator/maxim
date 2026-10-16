# План реализации: Каркас бэкенда и аккаунты

Branch: feature/backend-skeleton-accounts
Created: 2026-10-16

## Settings
- Testing: yes — тесты `domain` без базы; `app` + `postgres` и HTTP — на настоящей PostgreSQL через `testcontainers-go`, внешние системы поддельные (07 §9)
- Logging: verbose — `log/slog` JSON, уровень из `LOG_LEVEL` (по умолчанию `debug` в разработке, `info` в продакшене); поля и обязательные события — 07 §5
- Docs: yes — обязательная точка `/aif-docs` в конце: README бэкенда (запуск, переменные окружения, миграции, публикация оферт)

## Roadmap Linkage
Milestone: "Каркас бэкенда и аккаунты"
Rationale: план целиком реализует этот этап: основу приложения (`api`/`worker`/`migrate`, база, журнал, HTTP по спецификации, CI) и сценарии S-01, S-10 — вход, сессии, организации, реквизиты, принятие оферт.

## Research Context
Source: .ai-factory/RESEARCH.md (Active Summary) — выдержка, относящаяся к этапу

Goal: биржа рекламы в каналах MAX; этот этап даёт основу бэкенда, на которую ложатся остальные этапы.

Constraints:
- Стек: Go (бэкенд) + React (кабинеты, вне репозитория) — решение заказчика от 2026-10-12 (D-09).
- Блогеры — самозанятые, ИП и юрлица; физлица без статуса не допускаются (D-14). Рекламодатель — любой из трёх типов лица (D-24), физлицо недоступно (A-05).
- Вход только по почте с кодом, без пароля и без привязки аккаунта MAX (A-13, D-25).

Decisions: архитектура и библиотеки — `.ai-factory/ARCHITECTURE.md` (Explicit Architecture, pgx + sqlc + goose, oapi-codegen strict server).

Open questions, влияющие на этап: Q-01 (бот MAX на ИП — блокирует сдачу этапа по реестру, на код не влияет), Q-37 (допустимые сочетания `tax_regime`/`vat_rate` — реализуются по рабочей таблице 04 §2 с пометкой `// Q-37`).

## Источники ТЗ

- Сценарии: 02 S-01, S-10.
- Данные: 04 §1 (общие правила), §2 (аккаунты), §8 (`job`, `audit_log`), §9 (перечисления).
- Требования: 07 §1 (состав), §2 (очередь — в объёме этапа), §4 (безопасность), §5 (журнал, метрики, проверки готовности), §8 (конфигурация, миграции, окружения), §9 (требования к коду).
- Договор API: `api/openapi.yaml`, теги `auth`, `organizations`, `legal`.
- Архитектура: `.ai-factory/ARCHITECTURE.md` — структура папок, правила зависимостей, примеры.

## Границы этапа

Входит:
- Один модуль Go в корне: `cmd/maxim` с режимами `api`, `worker`, `migrate` и служебной командой `legal publish`.
- `platform/*` в объёме, нужном этапу: часы, идентификаторы, ошибки, журнал, конфигурация, база и транзакции, очередь заданий (ядро 07 §2), аудит, HTTP-middleware, хранилище, метрики, шифрование полезной нагрузки.
- Модуль `account` полностью: вход по коду, сессии, CSRF, ограничение частоты, организации и профили, реквизиты, документ о режиме налогообложения, оферты и их принятие, `GET /me`.
- Все прочие операции спецификации отвечают `501 not_implemented`. Их реализуют следующие этапы.
- Локальное окружение (PostgreSQL, MinIO, Mailpit), Dockerfile, проверки в CI.

Не входит (следующие этапы):
- Периодические задания, правила «после простоя» сверх возврата брошенных заданий, `inbound_event`/`outbound_request`, ограничитель MAX, оповещения дежурным — этап «Планировщик заданий и интеграционный слой».
- Уведомления в кабинете (`/notifications`) — этап «Уведомления и отчёт по кампании».
- API администратора (блокировка пользователей и организаций, загрузка оферт через кабинет) — этап «API администратора и отчётность ОРД». Пока оферты публикует команда `maxim legal publish`.
- Строка организации-платформы (`is_platform`) — этап «Маркировка через Яндекс ОРД».
- Выкладка бэкенда на тестовый стенд — этап «Юридическая и эксплуатационная готовность, пилот». Текущий деплой `mockapi` не трогаем.

## Решения этапа, требующие подтверждения

Документы этих мест не определяют или противоречат друг другу. План реализует указанный вариант. После подтверждения задача 17 вносит его в документы; при отказе меняется только отмеченная задача.

| # | Вопрос | Решение в плане | Задача |
|---|---|---|---|
| R-1 | Как отправить письмо с кодом, если `api` не обращается к внешним системам (07 §1), а код хранится только хешем (07 §4) | Новый вид задания `send_login_code`. Код лежит в `job.payload` зашифрованным (AES-256-GCM, ключ `APP_SECRETS_KEY` вне базы — разрешено 07 §4). После отправки полезная нагрузка затирается | 6, 9 |
| R-2 | Как считать лимит «20 кодов на IP в час» без отдельной таблицы | Поле `auth_code.requested_ip inet`; лимиты считаются выборкой по `auth_code` за час | 4, 9 |
| R-3 | Чем хешировать 6-значный код, чтобы его нельзя было перебрать по утёкшей базе | HMAC-SHA256 с секретом `AUTH_CODE_PEPPER` из окружения; сравнение за постоянное время | 5 |
| R-4 | Как защищать меняющие запросы от подделки (07 §4) | Double-submit: при входе выдаётся cookie `csrf_token` (без `HttpOnly`, `SameSite=Lax`), клиент повторяет её в заголовке `X-CSRF-Token`; без совпадения — `403 forbidden` | 11 |
| R-5 | Как загрузить документ о режиме налогообложения: в спецификации операции нет, а JSON файл не передаёт | Новая операция `PUT /organizations/current/tax-document` (`multipart/form-data`). Принимаются PDF, JPEG, PNG до 10 МБ, тип определяется по содержимому. Организация создаётся без файла; поле `tax_document_uploaded` в `Organization` | 2, 9, 11 |
| R-6 | `LegalDocument.version` в спецификации — строка-дата, в 04 §2 — `integer` | Правы документы: `integer`. `title` вычисляется сервером по `kind` | 2 |
| R-7 | Как ответить на операцию, которую этап ещё не реализует | `501` с кодом `not_implemented`; заглушки генерируются из `apigen.StrictServerInterface` | 2, 3 |
| R-8 | Кто публикует оферты до появления API администратора | Команда `maxim legal publish --kind <kind> --file <path>`: загружает файл в закрытую область хранилища и создаёт следующую версию | 13 |

## Commit Plan
- **Commit 1** (после задач 1–3): `feat(backend): bootstrap Go module, config, logging and spec updates`
- **Commit 2** (после задач 4–7): `feat(backend): add database, migrations, job queue and storage`
- **Commit 3** (после задач 8–11): `feat(account): login by email code, sessions and organizations`
- **Commit 4** (после задач 12–14): `feat(account): legal documents, e2e tests and legal publish command`
- **Commit 5** (после задач 15–17): `chore(backend): dev environment, CI checks and docs sync`

## Tasks

### Phase 1: Основа приложения

- [x] **Task 1: Модуль Go, точка входа, конфигурация и журнал**
  - Создать `go.mod` в корне: `module github.com/leha-creator/maxim`, `go 1.25`; инструменты через `tool` (`oapi-codegen`, `goose`). `mockapi/` остаётся отдельным модулем и в сборку не входит. Проверить, что `go build ./...` в корне его не затрагивает.
  - `cmd/maxim/main.go`: разбор подкоманд `api`, `worker`, `migrate`, `legal`; версия сборки через `-ldflags "-X main.version=<sha>"`; остановка по `SIGINT`/`SIGTERM` через `context`.
  - `internal/bootstrap/config.go`: вся конфигурация из окружения (`caarlos0/env/v11`). Группы: `APP_ENV` (`development`, `test`, `production`), `LOG_LEVEL`, `HTTP_ADDR`, `METRICS_ADDR`, `PUBLIC_BASE_URL`, `DATABASE_URL`, `DB_APP_ROLE`, `S3_*`, `SMTP_*`, `MAIL_FROM`, `APP_SECRETS_KEY` (32 байта, base64), `AUTH_CODE_PEPPER`, `COOKIE_SECURE`, параметры предметной области этапа со значениями по умолчанию из 07 §4: срок кода 600 с, 5 попыток, 5 кодов на почту в час, 20 на IP в час, сессия 30 суток, сессия администратора 12 ч. Проверка при запуске: обязательные поля, длина ключей, `COOKIE_SECURE=true` в `production`. При ошибке — запись ERROR со списком полей (без значений) и выход с кодом 1.
  - `internal/platform/logx`: `slog` JSON-обработчик в stdout; уровень из `LOG_LEVEL`; поля из `context`: `request_id`, `component` (`api`/`worker`), `module`, `op`, `user_id`, `organization_id`, `job_id`. Функции маскирования по 07 §5.4: `MaskEmail` (`i***@example.ru`), `MaskINN`, `MaskPhone`.
  - `internal/platform/clock` (`Clock`, `System`, `Manual` для тестов), `internal/platform/ids` (`New()` → UUID v7), `internal/platform/apperr` (коды `validation_failed`, `unauthorized`, `forbidden`, `not_found`, `conflict`, `inn_taken`, `rate_limited`, `not_implemented`, `internal` и поле `details`), `internal/platform/money` (`Kopecks int64`, пока только тип).
  - Журнал: INFO `app started` (`mode`, `version`, `env`), INFO `app stopped`; ERROR `config invalid` (`fields`); DEBUG — итоговая конфигурация с маскированными секретами.
  - Тесты: проверка конфигурации (нет обязательного, короткий ключ, `production` без `COOKIE_SECURE`); маскирование; `Manual` clock.
  - Файлы: `go.mod`, `cmd/maxim/main.go`, `internal/bootstrap/config.go`, `internal/platform/{logx,clock,ids,apperr,money}/*.go`.

- [x] **Task 2: Уточнение `api/openapi.yaml` под аккаунты (R-5, R-6, R-7)**
  - `ErrorCode` дополнить значениями `inn_taken`, `rate_limited`, `not_implemented`. Добавить ответ `RateLimited` (`429`, заголовок `Retry-After`) к `requestLoginCode`, `login`, `uploadTaxDocument`, ответ `NotImplemented` (`501`) — в `components/responses`.
  - `createOrganization`: `409` с кодом `inn_taken` описать в `description`.
  - Новая операция `PUT /organizations/current/tax-document` (`operationId: uploadTaxDocument`, тег `organizations`, `multipart/form-data`, поле `file`; ответ `200` — `Organization`). В `Organization` — поле `tax_document_uploaded: boolean`.
  - `LegalDocument.version` → `integer`.
  - В `info.description` описать CSRF (R-4): cookie `csrf_token` выдаётся при входе; меняющие запросы, кроме `/auth/code` и `/auth/login`, передают её значение в `X-CSRF-Token`.
  - Заглушка `mockapi`: `go generate ./...` в `mockapi/`, добавить обработчик `UploadTaxDocument` (сохраняет признак в памяти), поправить место, где `version` был строкой. `go test ./...` в `mockapi/` проходит — иначе CI `deploy.yml` упадёт.
  - Журнал: в `mockapi` — по его правилам (DEBUG вход в обработчик, INFO `tax document uploaded`).
  - Файлы: `api/openapi.yaml`, `mockapi/internal/api/api.gen.go`, `mockapi/internal/handlers/*.go`, `mockapi/internal/store/*.go`.

- [x] **Task 3: Генерация API и HTTP-сервер (R-7)** (depends on 1, 2)
  - `internal/apigen/generate.go` + `oapi-codegen.yaml`: модели, `std-http-server`, `strict-server`, `embedded-spec`; `exclude-tags: [_mock]`. Сгенерированное коммитится.
  - `internal/apigen/cmd/genstub` (или `go:generate` с `go/types`): по `StrictServerInterface` генерирует `internal/bootstrap/unimplemented.go` — тип `unimplemented` с методами, возвращающими `apperr.NotImplemented`. Сервер собирается встраиванием: обработчики модулей перекрывают заглушки.
  - `internal/platform/httpx`: middleware `RequestID` (`X-Request-Id` из запроса, если это UUID, иначе новый; в ответ и в `context`), `AccessLog`, `Recover` (ERROR с `stack`, ответ `500 internal`), `BodyLimit` (1 МБ по умолчанию, 11 МБ для `uploadTaxDocument`), `SecurityHeaders` (`X-Content-Type-Options`, `Referrer-Policy`, `Cache-Control: no-store` для `/api/`), проверка запроса по встроенной спецификации (`oapi-codegen/nethttp-middleware`) с ответом `422 validation_failed` и полем `details.field`. Единый обработчик ошибок strict server: `apperr` → HTTP-код и тело `Error` с `request_id`.
  - `internal/bootstrap/server.go`: маршруты под `/api/v1`, `GET /livez` (процесс жив), `GET /readyz` (`SELECT 1` к базе с таймаутом 1 с). `METRICS_ADDR` — отдельный сервер с `/metrics` (`prometheus/client_golang`): число и длительность запросов по маршруту и коду (07 §5.5).
  - `internal/bootstrap/api.go`: режим `api` — запуск, плавная остановка за 15 с.
  - Журнал: DEBUG `http request started` (`method`, `route`), INFO `http request finished` (`status`, `duration_ms`); WARN `request validation failed` (`field`); WARN `access denied` для `403`; ERROR `panic recovered`. Тело запроса и cookie не пишутся.
  - Тесты: `httptest` — неизвестный маршрут `404`, операция без реализации `501 not_implemented` с `request_id`, неверное тело `422`, `X-Request-Id` в каждом ответе, `/readyz` без базы — `503`.
  - Файлы: `internal/apigen/*`, `internal/platform/httpx/*.go`, `internal/platform/metrics/*.go`, `internal/bootstrap/{server.go,api.go,unimplemented.go}`.
<!-- Commit checkpoint: tasks 1-3 -->

### Phase 2: Данные и инфраструктура

- [ ] **Task 4: База, транзакции, миграции и схема аккаунтов (R-2)** (depends on 1)
  - `internal/platform/db`: `pgxpool` из `DATABASE_URL`, проверка соединения при старте, метрики пула.
  - `internal/platform/tx`: интерфейс `Runner { InTx(ctx, fn) error }` и реализация на pgx. Транзакция кладётся в `context`; `db.Querier(ctx)` возвращает транзакцию или пул. Вложенный `InTx` использует внешнюю транзакцию. Уровень `READ COMMITTED`.
  - `migrations/` (goose, SQL, `embed`), команда `maxim migrate up|down|status`. Первая миграция `00001_accounts.sql`: расширение `citext`; таблицы `user`, `auth_code` (+ `requested_ip inet`, R-2), `session`, `organization`, `organization_member`, `advertiser_profile`, `blogger_profile`, `legal_document`, `legal_acceptance`, `audit_log`, `job` строго по 04 §1, §2, §8: `id uuid` без значения по умолчанию, `created_at`/`updated_at`, `check` для перечислений 04 §9, уникальности (`user.email`, `organization.inn`, `session.token_hash`, `(legal_document.kind, version)`, `(organization_id, user_id)`), частичный уникальный индекс `job(dedupe_key) where status in ('pending','running')`, индексы `job(status, run_at)`, `auth_code(email, created_at)`, `auth_code(requested_ip, created_at)`.
  - `00002_app_role_grants.sql` (goose `ENVSUB ON`, роль из `DB_APP_ROLE`): права роли приложения; на `legal_acceptance`, `audit_log` — только `SELECT, INSERT` (04 §1, 07 §4).
  - `sqlc.yaml` с записью для модуля `account` и `platform/job`, `platform/audit`; движок `postgresql`, `pgx/v5`, переопределения типов: `uuid` → `github.com/google/uuid.UUID`, `timestamptz` → `time.Time`, `inet` → `netip.Addr`.
  - `internal/platform/dbtest`: запуск PostgreSQL 17 в `testcontainers-go` один раз на пакет (`TestMain`), применение миграций, очистка таблиц между тестами. Если Docker недоступен — `t.Skip` с понятным сообщением локально и падение в CI (`CI=true`).
  - Журнал: INFO `migrations applied` (`from`, `to`), ERROR `migration failed` (`version`); DEBUG `db connected` (`max_conns`); WARN `tx rollback` с причиной.
  - Тесты: миграции `up` → `down` → `up` без ошибок; роль приложения не может `UPDATE`/`DELETE` в `audit_log` и `legal_acceptance`; `InTx` откатывает всё при ошибке.
  - Файлы: `internal/platform/{db,tx,dbtest}/*.go`, `migrations/*.sql`, `migrations/embed.go`, `sqlc.yaml`, `internal/bootstrap/migrate.go`.

- [ ] **Task 5: Домен аккаунтов (R-3)** (depends on 1)
  - `internal/account/domain`, только стандартная библиотека и `platform/{ids,clock,apperr}`:
    - `Organization`: конструктор `NewOrganization(input, cabinet)` и `Update(patch)` с правилами 04 §2 и D-24: ИНН — 10 цифр для `legal_entity`, 12 для остальных, контрольные числа ФНС; `kpp`, подписант — только `legal_entity`; `address` обязателен для `legal_entity` и `sole_proprietor`, у `self_employed` запрещён; `tax_regime`, `vat_rate` — только в профиле блогера и обязательны ему; допустимые сочетания по таблицам 04 §2 (`// Q-37`); `blogger_full_name` обязателен блогеру `self_employed`/`sole_proprietor`, у `legal_entity` пуст. ИНН и `legal_type` не меняются. `AddCabinet(cabinet, fields)` — второй профиль у существующей организации: реквизиты не меняются, но недостающие для роли блогера поля обязательны.
    - `LoginCode`: генерация 6 цифр через `crypto/rand`; `HashCode(pepper, email, code)` — HMAC-SHA256 (R-3); `Verify(code, now)` — срок, число попыток (не больше 5), сравнение за постоянное время, однократность.
    - `Session`: токен 32 байта `crypto/rand` (base64url), хранится SHA-256; срок 30 суток от последнего использования, у пользователя с `admin_role` — 12 ч (07 §4); `Touch(now)` продлевает, если с последнего продления прошло больше часа.
    - `User` (`status`, `admin_role`), `Cabinet` (`advertiser`, `blogger`), `LegalDocumentKind`, `RequiredDocuments(cabinets)` — набор документов для открытых кабинетов (S-01 шаг 5, S-10 шаг 3).
    - Порты: `Users`, `AuthCodes`, `Sessions`, `Organizations`, `LegalDocuments` (репозитории); `LoginCodeSender`; `FileStorage`.
  - Ошибки с кодами: `inn_taken`, `validation_failed` (`details.field`, `details.reason`), `unauthorized`, `rate_limited`.
  - Журнал: в `domain` нет.
  - Тесты (табличные, без базы): контрольные числа ИНН (верные, неверные, неверная длина для типа); каждая строка таблиц `tax_regime`/`vat_rate`; обязательность полей по типу лица и кабинету; запрет смены ИНН; код — истёк, 5 неверных попыток, повторное использование; продление сессии; срок сессии администратора.
  - Файлы: `internal/account/domain/*.go`, `internal/account/domain/*_test.go`.

- [ ] **Task 6: Очередь заданий — ядро (R-1)** (depends on 4)
  - `internal/platform/job`: `Enqueue(ctx, Job...)` пишет в таблицу `job` через транзакцию из `context` (I-10); `dedupe_key` — повтор при живом задании не создаёт второе и не считается ошибкой.
  - `Worker`: опрос раз в секунду (`FOR UPDATE SKIP LOCKED`, `status = pending`, `run_at <= now()`), отметка `running`, `locked_by` (имя хоста + PID), `locked_at`; обработчики по `kind`; успех → `succeeded`, `finished_at`; ошибка → `attempts + 1`, `run_at = now + min(30 с × 2^attempts, 1 ч)`, после `max_attempts` → `failed`. Задание в `running` дольше 10 минут → `pending` (07 §2). Плавная остановка: новые не берутся, текущие доделываются до 30 с, иначе возвращаются в `pending`.
  - `internal/platform/secretbox`: `Seal`/`Open` (AES-256-GCM, ключ `APP_SECRETS_KEY`) — для полезной нагрузки с секретами (R-1).
  - `internal/bootstrap/worker.go`: режим `worker`, регистрация обработчиков модулей.
  - Метрики: задания в очереди, в работе, просроченные, сбойные — по `kind` (07 §5.5).
  - Журнал: INFO `job started`, `job finished` (`job_id`, `kind`, `attempt`, `duration_ms`); WARN `job failed, will retry` (`next_run_at`, `error`); ERROR `job exhausted attempts`; WARN `abandoned job returned to queue`; DEBUG `job enqueued` (`kind`, `dedupe_key`, `run_at`). Сквозной `request_id` из `context` записывается в `payload._request_id` и восстанавливается в `context` обработчика (07 §5.1).
  - Тесты (`dbtest`, `clock.Manual`): задание из откатившейся транзакции не появляется; дубликат по `dedupe_key`; повтор с правильной задержкой; `failed` после `max_attempts`; два воркера не берут одно задание; брошенное задание возвращается; `Seal`/`Open` и отказ при чужом ключе.
  - Файлы: `internal/platform/job/*.go`, `internal/platform/secretbox/*.go`, `internal/bootstrap/worker.go`.

- [ ] **Task 7: Хранилище файлов и журнал действий** (depends on 4)
  - `internal/platform/storage`: порт `ObjectStorage` (`PutPrivate(ctx, key, r, size, contentType)`, `PresignGet(ctx, key, ttl)`, `Delete`); реализация на `minio-go/v7` (`S3_ENDPOINT`, `S3_BUCKET_PRIVATE`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_USE_SSL`); `storagefake` в памяти для тестов. Ключи генерирует система: `<area>/<uuidv7>` (07 §4).
  - `internal/platform/audit`: `Record(ctx, Entry)` пишет `audit_log` в транзакции из `context`; `actor_type`, `actor_user_id`, `organization_id`, `ip` берутся из `context` (кладёт HTTP-слой); `data` — значения до и после с маскированными ИНН, телефоном, почтой.
  - Журнал: DEBUG `object stored` (`key`, `size`, `duration_ms`), ERROR `object storage failed`; DEBUG `audit recorded` (`action`, `object_type`, `object_id`).
  - Тесты: `audit` пишет в транзакции и откатывается вместе с ней; маскирование в `data`; `storagefake` — контрактный тест, общий с MinIO (MinIO-вариант запускается через `testcontainers-go`, в CI обязательно).
  - Файлы: `internal/platform/storage/**`, `internal/platform/audit/*.go`, `internal/platform/audit/queries.sql`.
<!-- Commit checkpoint: tasks 4-7 -->

### Phase 3: Аккаунты

- [ ] **Task 8: Хранилище аккаунтов (`account/postgres`)** (depends on 4, 5)
  - `internal/account/postgres/queries/*.sql` → `sqlc` → `internal/account/postgres/sqlc`. Реализации портов `domain`:
    - `Users`: найти по почте, создать, обновить `last_login_at`.
    - `AuthCodes`: создать; последний непогашенный код по почте `FOR UPDATE`; записать попытку; погасить; счётчики за час по почте и по `requested_ip` (R-2).
    - `Sessions`: создать; найти по хешу токена (не отозвана, не истекла); продлить; отозвать.
    - `Organizations`: создать с участником `owner` и профилем; найти по пользователю; `GetForUpdate(ctx, organizationID)`; обновить; проверить занятость ИНН. Каждый метод кабинета принимает `organizationID` явно (07 §4).
    - `LegalDocuments`: действующие версии по видам (наибольший `version`); принятые организацией; записать принятие; создать версию (для команды публикации).
  - Нарушение уникальности ИНН (`23505` на `organization_inn_key`) превращается в `inn_taken`.
  - Журнал: DEBUG на каждый метод (`op` вида `Account.<Repo>.<Method>`, ключевые идентификаторы, без почты и ИНН в открытом виде); ERROR при неожиданной ошибке базы.
  - Тесты (`dbtest`): каждый метод; гонка двух созданий с одним ИНН → один `inn_taken`; сессия не находится после отзыва и после срока; счётчики лимитов.
  - Файлы: `internal/account/postgres/**`, запись `account` в `sqlc.yaml`.

- [ ] **Task 9: Сценарии аккаунтов (`account/app`) и задание отправки кода (R-1, R-2, R-5)** (depends on 5, 6, 7, 8)
  - `internal/account/app.Service`, зависимости через `Deps` (порты, `tx.Runner`, `job` enqueuer, `audit`, `clock`, `logx`, параметры из конфигурации):
    - `RequestLoginCode(email, ip)`: лимиты 5 на почту и 20 на IP в час → `rate_limited` с `retry_after`; новый код (прежние непогашенные гаснут); в той же транзакции — задание `send_login_code` (`dedupe_key = login_code:<auth_code_id>`, `max_attempts = 5`, полезная нагрузка — `Seal(email, code)`). Ответ одинаков для нового и существующего адреса (07 §4).
    - `Login(email, code, ip, userAgent)`: проверка кода, `attempts + 1` при неверном; пользователь создаётся при первом входе; `status = blocked` → `forbidden`; сессия; `last_login_at`; аудит `auth.login`. Возвращает токен сессии и токен CSRF.
    - `Authenticate(token)`: принципал (пользователь, организация, кабинеты, `admin_role`); продление сессии.
    - `Logout`: отзыв сессии, аудит `auth.logout`.
    - `Me`: пользователь, организации с кабинетами, `pending_legal_document_ids` для открытых кабинетов.
    - `CreateOrganization(input)`: нет организации → создать с профилем; есть → добавить второй профиль (`AddCabinet`); `inn_taken`; аудит `organization.create` / `organization.add_cabinet`.
    - `GetOrganization`, `UpdateOrganization(patch)`: правила домена; аудит `organization.update` с полями до и после.
    - `UploadTaxDocument(file)`: только для кабинета блогера; тип по содержимому (`http.DetectContentType` + сигнатура PDF), PDF/JPEG/PNG, до 10 МБ; запись в хранилище `tax_document/<uuid>`, `sha256`; прежний файл не удаляется (история через аудит); аудит `organization.tax_document_upload`.
    - `ListLegalDocuments` (с признаком `accepted` и подписанной ссылкой на 10 минут), `AcceptLegalDocuments(ids, ip)`: только действующие версии, нужные открытым кабинетам; повторное принятие — без ошибки и без дубля; аудит `legal.accept`.
  - `internal/account/jobs`: обработчик `send_login_code` — `Open`, отправка через `LoginCodeSender`, затем `payload` заменяется на `{}`. Код просрочен к моменту отправки → задание завершается без письма (WARN).
  - Журнал (07 §5.3): INFO `login code requested` (`email` маскированный, `ip`), WARN `login code rate limited` (`scope`: `email`/`ip`), INFO `user logged in` (`user_id`, `new_user`), WARN `login failed` (`reason`: `wrong_code`/`expired`/`attempts_exceeded`/`blocked`), INFO `user logged out`, INFO `organization created` / `cabinet added` / `organization updated` (`organization_id`, `cabinet`, изменённые поля без значений), WARN `organization rejected` (`reason`), INFO `legal documents accepted` (`ids`), INFO `tax document uploaded` (`size`, `content_type`), INFO `login code sent` (`job_id`). Код, токены, ИНН и почта в открытом виде не пишутся.
  - Тесты (`dbtest`, `storagefake`, `mailfake`, `clock.Manual`): каждый сценарий, успешный путь и отклонения S-01/S-10: `inn_taken`, лимиты по почте и IP, 5 неверных кодов, просроченный код, повторный код гасит прежний, заблокированный пользователь, второй кабинет без недостающих полей блогера, неверный тип файла, принятие чужого или устаревшего документа; повтор задания `send_login_code` не отправляет второе письмо.
  - Файлы: `internal/account/app/*.go`, `internal/account/jobs/*.go`.

- [ ] **Task 10: Почта (`integration/mail`)** (depends on 1)
  - `internal/integration/mail`: SMTP-клиент (`wneessen/go-mail`, `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD`, `SMTP_TLS`, `MAIL_FROM`), таймаут 10 с; реализует `account/domain.LoginCodeSender`. Шаблон письма — `templates/email/login_code.{subject,txt,html}.tmpl` (07 §6: тексты в репозитории), язык русский, срок действия в минутах, время в МСК.
  - `internal/integration/mail/mailfake`: в памяти, для тестов — последнее письмо на адрес.
  - Журнал: DEBUG `mail send started` (`template`, `to` маскированный), INFO `mail sent` (`template`, `duration_ms`), ERROR `mail send failed` (`error` без учётных данных). Тело письма не пишется.
  - Тесты: рендеринг шаблона (код подставлен, срок в минутах); отправка на SMTP-сервер Mailpit через `testcontainers-go` (в CI обязательно).
  - Файлы: `internal/integration/mail/**`, `templates/email/*`, `templates/embed.go`.

- [ ] **Task 11: HTTP аккаунтов: сессия, CSRF, лимиты, обработчики (R-4, R-5)** (depends on 3, 9)
  - `internal/platform/httpx/auth.go`: middleware читает cookie `session`, вызывает `Authenticate` (через интерфейс, без импорта модуля), кладёт принципал в `context` и поля журнала `user_id`, `organization_id`. Операции с `security: []` пропускаются; остальные без сессии → `401 unauthorized`.
  - `httpx/csrf.go`: для `POST`/`PUT`/`PATCH`/`DELETE`, кроме `requestLoginCode` и `login`, сверка `X-CSRF-Token` с cookie `csrf_token` за постоянное время → иначе `403 forbidden` (`details.reason = csrf`).
  - `internal/account/httpapi.Handlers`: `RequestLoginCode`, `Login` (cookie `session`: `HttpOnly`, `Secure` по `COOKIE_SECURE`, `SameSite=Lax`, `Path=/`, `Max-Age` по сроку; cookie `csrf_token` без `HttpOnly`), `Logout` (очистка обеих), `GetMe`, `CreateOrganization`, `GetCurrentOrganization`, `UpdateCurrentOrganization`, `UploadTaxDocument`, `ListLegalDocuments`, `AcceptLegalDocuments`. Преобразование `apigen` ↔ `domain` — только здесь. `rate_limited` → `429` с `Retry-After`. Нет организации у пользователя → `404` для `/organizations/current`.
  - Помощник `RequireCabinet(ctx, cabinet)` в `httpx` для следующих этапов: нет кабинета → `403 forbidden` (`details.reason = cabinet_required`).
  - IP клиента: из `RemoteAddr`; `X-Forwarded-For` учитывается только при `TRUSTED_PROXY_CIDRS` (edge-прокси стенда).
  - Журнал: WARN `csrf check failed` (`route`), WARN `unauthorized request` (`route`), DEBUG `principal resolved` (`user_id`, `cabinets`). Cookie и токены не пишутся.
  - Тесты (`httptest` поверх собранного сервера, `dbtest`): каждый обработчик; `Set-Cookie` с нужными признаками; запрос без CSRF → `403`; без сессии → `401`; `429` с `Retry-After`; ответы совпадают со схемой спецификации (проверка ответа через `kin-openapi`).
  - Файлы: `internal/platform/httpx/{auth.go,csrf.go,clientip.go}`, `internal/account/httpapi/*.go`, `internal/bootstrap/server.go`.
<!-- Commit checkpoint: tasks 8-11 -->

### Phase 4: Оферты и сквозная проверка

- [ ] **Task 12: Сборка режимов и проводка зависимостей** (depends on 6, 9, 10, 11)
  - `internal/bootstrap`: создание пула, `tx.Runner`, `job` enqueuer, `audit`, хранилища, почты, `account/app.Service`; встраивание `account/httpapi.Handlers` в сервер; регистрация `account/jobs` в `worker`. Выбор реализаций по `APP_ENV`: в `test` — поддельные, иначе настоящие (Mailpit и MinIO в разработке — это настоящие SMTP и S3).
  - Глобальных переменных с состоянием нет; всё передаётся конструкторами.
  - Журнал: INFO `component ready` (`component`, `addr`), INFO `shutdown started` / `shutdown finished` (`duration_ms`).
  - Тесты: `go build ./...`; запуск `api` и `worker` в одном тесте на `dbtest` — `/readyz` отвечает `200`.
  - Файлы: `internal/bootstrap/*.go`, `cmd/maxim/main.go`.

- [ ] **Task 13: Команда публикации оферт (R-8)** (depends on 7, 8, 12)
  - `maxim legal publish --kind <advertiser_offer|blogger_offer|platform_rules|privacy_policy> --file <path> [--published-at <RFC3339>]`: проверяет вид и файл (PDF), загружает в `legal/<uuid>.pdf`, считает `sha256`, создаёт `legal_document` со следующим `version` в транзакции; аудит `legal.publish` с `actor_type = system`. `maxim legal list` — действующие версии.
  - Журнал: INFO `legal document published` (`kind`, `version`, `sha256`), ERROR при сбое загрузки (без создания записи).
  - Тесты: публикация двух версий одного вида → действующая — вторая; пользователь, принявший первую, снова видит документ в `pending_legal_document_ids` (A-25).
  - Файлы: `cmd/maxim/legal.go`, `internal/account/app/legal_publish.go`.

- [ ] **Task 14: Сквозные тесты S-01 и S-10** (depends on 12, 13)
  - `internal/bootstrap/e2e_test.go` (или `test/e2e`): собранный сервер + `worker` в одном процессе, `dbtest`, `mailfake`, `storagefake`, `clock.Manual`.
    - S-01: запросить код → воркер отправил письмо → вход по коду из письма → `GET /me` пустые организации → `POST /organizations` (`advertiser`, ООО) → `pending_legal_document_ids` = оферта рекламодателя, правила, политика → принятие → список пуст.
    - S-10: блогер-самозанятый с `npd`/`none` → загрузка документа → принятие оферты блогера; затем тот же пользователь добавляет кабинет рекламодателя — реквизиты не меняются, нужна оферта рекламодателя.
    - Отклонения: `inn_taken` для второго пользователя; ИП с `osno` и `vat_5` → `422`; запрос без CSRF; выход — старая cookie даёт `401`.
  - Проверка журнала: в тесте подключается обработчик `slog`, собирающий записи; обязательные события этапа присутствуют, а код входа, токен сессии, ИНН и почта в открытом виде не встречаются ни в одной записи.
  - Файлы: `internal/bootstrap/e2e_test.go`, `internal/platform/logx/logxtest/*.go`.

### Phase 5: Окружение, CI, документы

- [ ] **Task 15: Локальное окружение и образ** (depends on 12)
  - `Dockerfile` в корне: сборка на `golang:1.27`, итог — `gcr.io/distroless/static:nonroot`, бинарник `maxim`, `ENTRYPOINT ["/maxim"]`, версия через `--build-arg`.
  - `compose.dev.yml`: PostgreSQL 17 (две роли: владелец для миграций и `maxim_app`), MinIO с созданием закрытой корзины, Mailpit (SMTP 1025, веб 8025); сервисы `migrate`, `api`, `worker` из того же образа. `.env.example` со всеми переменными и безопасными значениями для разработки.
  - `.dockerignore` дополнить, чтобы `docs/` и `mockapi/` не попадали в контекст сборки бэкенда.
  - Журнал: `LOG_LEVEL=debug` в `.env.example`.
  - Проверка: `docker compose -f compose.dev.yml up` → вход по коду из письма в Mailpit проходит через Swagger/`curl`.
  - Файлы: `Dockerfile`, `compose.dev.yml`, `.env.example`, `.dockerignore`.

- [ ] **Task 16: Проверки бэкенда в CI** (depends on 14)
  - `.github/workflows/backend.yml` (на `pull_request` и `push` в `master`, отдельно от `deploy.yml`): `setup-go` 1.27 (как в `deploy.yml`), `sqlc-dev/setup-sqlc`; шаги: `go generate ./...` и `sqlc generate` без изменений в git; `sqlc vet`; `go vet ./...`; `go test -race ./...` с `CI=true` (Docker на `ubuntu-latest` есть — `testcontainers-go` работает); `docker build` корневого `Dockerfile` без публикации.
  - `deploy.yml` не меняется: деплой `mockapi` остаётся как есть.
  - Журнал: не применимо (конфигурация CI); в шагах — `::error::` с понятной причиной.
  - Файлы: `.github/workflows/backend.yml`.

- [ ] **Task 17: Синхронизация документов с решениями этапа** (depends on 2, 9, 11)
  - После подтверждения R-1 … R-8 внести их в документы, чтобы правы были документы (README «Соглашения»):
    - 07 §2 — строка `send_login_code` в таблице видов заданий (R-1); 07 §4 — HMAC кода (R-3), CSRF double-submit (R-4), форматы документа о режиме (R-5), ключ `APP_SECRETS_KEY` для полезной нагрузки (R-1).
    - 04 §2 — поле `auth_code.requested_ip` (R-2); 04 §2 `legal_document` — способ публикации до API администратора (R-8).
    - 08 — экран профиля блогера: загрузка документа отдельной операцией (R-5), если расходится.
    - `.ai-factory/ARCHITECTURE.md` — `wneessen/go-mail`, `secretbox`, `dbtest` в таблице решений и структуре папок.
  - Журнал: не применимо.
  - Файлы: `docs/04-domain-model.md`, `docs/07-non-functional.md`, `docs/08-screens.md`, `.ai-factory/ARCHITECTURE.md`.
<!-- Commit checkpoint: tasks 15-17 -->

## Критерии готовности этапа

- `go build ./...`, `go vet ./...`, `go test -race ./...` проходят локально и в CI; сгенерированный код совпадает с исходниками.
- `docker compose -f compose.dev.yml up` поднимает `api` и `worker`; сценарии S-01 и S-10 проходятся через HTTP с письмом из Mailpit.
- Все операции спецификации, кроме аккаунтов, отвечают `501 not_implemented`; `/livez`, `/readyz`, `/metrics` работают.
- Обязательные события журнала этапа (07 §5.3: вход, выход, неудачный вход, отказ в доступе, задания) пишутся; секреты и персональные данные в открытом виде — нет (проверено тестом задачи 14).
- Роль приложения не может менять `audit_log` и `legal_acceptance`.
- Решения R-1 … R-8 подтверждены и внесены в документы (задача 17).
