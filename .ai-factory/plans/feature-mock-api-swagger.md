# План реализации: Swagger кабинетов и тестовый REST API на Go

Branch: feature/mock-api-swagger
Created: 2026-10-15

## Цель

1. Собрать спецификацию OpenAPI 3 (swagger) HTTP API кабинетов рекламодателя, блогера и администратора по документам `docs/02`, `docs/03`, `docs/04`, `docs/08`.
2. Сделать по ней тестовый сервер на Go: данные выдуманные, хранятся в памяти, внешних систем нет. Сервер нужен, чтобы пройти сценарии руками через Swagger UI и увидеть, как устроено API, — это не каркас продукта.

## Settings
- Testing: yes — минимальные: спецификация проходит проверку, каждая операция отвечает, ответ совпадает со схемой
- Logging: verbose — JSON через `log/slog`, уровень из `LOG_LEVEL`, по умолчанию `debug`; поля по 07 §5.1
- Docs: no — README заглушки входит в задачу 9; `/aif-docs` не нужен

## Roadmap Linkage
Milestone: "none"
Rationale: "Skipped by user" — демонстрационная заглушка, ни один этап роадмапа не закрывает.

## Research Context
Source: .ai-factory/RESEARCH.md (Active Summary)

Goal: биржа рекламы в каналах MAX: каталог → корзина → кампания с единым постом → оплата → размещения по каналам → публикация → приёмка → выплата блогеру.

Constraints, важные для API:
- Единица работы — размещение (`placement`); у него своя машина состояний (03 §3, §4: P-01 … P-18).
- Деньги — целые копейки (`integer`, `format: int64`); время — RFC 3339 в UTC; идентификаторы — UUID; перечисления — строго из 04 §9.
- Суммы считает сервер; клиент передаёт идентификаторы, а не суммы (07 §4).
- Вход по почте с кодом из 6 цифр, сессия в cookie `HttpOnly` (07 §4).
- Названия сущностей, полей и значений — из глоссария и 04 без изменений (07 §9).

## Принятые решения

| Вопрос | Решение | Почему |
|---|---|---|
| Где лежит | `api/openapi.yaml` — спецификация; `mockapi/` — отдельный Go-модуль заглушки | Заглушка не предрешает структуру настоящего бэкенда (этап «Каркас и аккаунты», 07 §1) |
| Версия OpenAPI | 3.0.3, один файл | Лучше всего поддерживается `oapi-codegen` и `kin-openapi` |
| Префикс | `/api/v1` | |
| Генерация | `oapi-codegen` v2: модели + strict server на стандартном `net/http` (Go 1.22+ `ServeMux`) | Сервер и спецификация не расходятся: код собирается только из спецификации |
| Данные | Хранилище в памяти с фиксированным начальным набором (seed); изменения живут до перезапуска; `POST /api/v1/_mock/reset` возвращает исходное состояние | «Посмотреть как работает» — действия должны менять статусы, а не возвращать одно и то же |
| Вход | `POST /auth/code` всегда успешен; `POST /auth/login` принимает код `000000`; три готовых пользователя: `adv@example.test`, `blg@example.test`, `adm@example.test` | Почты нет, код известен заранее |
| Защита от подделки запросов | Нет; в спецификации описан заголовок `X-CSRF-Token`, заглушка его не проверяет | Для демонстрации лишнее; помечено в README |
| Ошибки | Единый объект `{"error":{"code","message","details"},"request_id"}`; коды: `validation_failed`, `unauthorized`, `forbidden`, `not_found`, `invalid_transition`, `conflict` | Один формат для всех операций |
| Списки | `?limit=&cursor=` → `{"items":[...],"next_cursor":...}` | |
| Внешние системы | Webhook MAX и Рокет Ворк описаны, отвечают `200` и ничего не делают; ОРД, банк, почта не вызываются | Заглушка без сети |
| Swagger UI | `GET /docs` — страница Swagger UI (скрипт с cdnjs), `GET /openapi.yaml` — встроенная через `embed` спецификация | Открывается браузером, установки не требует |

## Состав API

Теги совпадают с разделами кабинета (08 §2). Одна строка — одна или несколько операций.

**auth, общие** — `POST /auth/code`, `POST /auth/login`, `POST /auth/logout`, `GET /me` (пользователь, организации, роли), `GET /legal-documents`, `POST /legal-acceptances`, `GET /notifications`, `POST /notifications/{id}/read`.

**organizations** (S-01, S-10; `auth/org-adv`, `auth/org-blg`, профили) — `POST /organizations`, `GET /organizations/current`, `PATCH /organizations/current`; правила `legal_type` → `tax_regime` → `vat_rate` из 04 §2 описаны в схеме и проверяются заглушкой.

**catalog** (S-02; `adv/catalog`, `adv/channel`) — `GET /catalog/channels` (фильтры: категория, формат, цена от/до, подписчики от/до, сортировка), `GET /catalog/channels/{id}`, `GET /channel-categories`, `GET /placement-formats`.

**cart** (S-03) — `GET /cart`, `POST /cart/items`, `PATCH /cart/items/{id}` (формат), `DELETE /cart/items/{id}`.

**campaigns и post** (S-04, S-05, S-29; `adv/campaigns`, `adv/campaign-new`, `adv/campaign-post`) — `GET/POST /campaigns`, `GET/PATCH /campaigns/{id}`, `GET/PUT /campaigns/{id}/post` (черновик: документ редактора, медиа, `token_carrier`, `token_position`), `POST /campaigns/{id}/post/preview` (возвращает `body`/`final_text` и предупреждения), `POST /campaigns/{id}/checkout` (оформление заказа, P-01).

**media** (`adv/media`) — `GET /media`, `POST /media` (multipart; заглушка проверяет только тип и размер, файл не хранит, отдаёт картинку-заглушку), `DELETE /media/{id}`.

**orders и placements рекламодателя** (S-06 … S-09; `adv/orders`, `adv/order`, `adv/report`) — `GET /orders`, `GET /orders/{id}` (с размещениями), `POST /orders/{id}/post-versions` (S-29, P-11), `GET /orders/{id}/report`, `POST /placements/{id}/cancel` (P-06), `POST /placements/{id}/accept` (P-12), `POST /placements/{id}/claims` (P-14).

**balance** (S-06, S-33; `adv/balance`) — `GET /balance`, `GET /balance/transactions`, `GET/POST /invoices`, `GET /invoices/{id}/pdf` (заглушка — PDF в одну страницу).

**blogger** (S-11 … S-15; `blg/*`) — `GET /blogger/channels`, `POST /blogger/channels/link-code` (код привязки канала), `GET/PATCH /blogger/channels/{id}`, `PUT /blogger/channels/{id}/prices`, `POST /blogger/channels/{id}/unavailability`, `DELETE /blogger/channels/{id}/unavailability/{uid}`, `GET /blogger/placements`, `GET /blogger/placements/{id}`, `POST /blogger/placements/{id}/accept` (P-03), `/decline` (P-04), `/schedule` (P-17, включая «опубликовать сейчас»), `/reschedule` (P-07), `GET /blogger/payouts`, `GET /blogger/rocketwork`, `POST /blogger/rocketwork/invite`.

**admin** (S-16 … S-20, S-30, S-31; `adm/*`) — очередь каналов `GET /admin/channels`, `GET /admin/channels/{id}`, `POST .../approve`, `.../reject`, `.../rkn`; очередь постов `GET /admin/post-versions`, `POST .../approve`, `.../reject`; `GET /admin/placements`, `GET /admin/placements/{id}` (с историей состояний), `POST .../admin-cancel`, `.../open-dispute`, `.../mark-removed`; `GET /admin/disputes`, `GET /admin/disputes/{id}`, `POST .../messages`, `POST .../resolve` (P-15); `GET /admin/invoices`, `POST /admin/bank-statements`, `GET /admin/bank-transactions`, `POST /admin/bank-transactions/{id}/match`; `GET /admin/payouts`, `POST /admin/payouts/{id}/retry`; `GET /admin/ledger`, `POST /admin/ledger/manual`; `GET /admin/ord/errors`, `POST /admin/ord/errors/{id}/retry`, `GET /admin/ord/reports`; `GET /admin/organizations`, `POST /admin/organizations/{id}/block`; `GET /admin/audit-log`. Действия администратора требуют поля `reason` (04 `audit_log`).

**webhooks** — `POST /webhooks/max/{secret}`, `POST /webhooks/rocketwork`.

**_mock** — `POST /_mock/reset`, `POST /_mock/clock` (сдвиг подменяемых часов, чтобы срабатывали таймеры T-01, T-05, T-13), `POST /_mock/placements/{id}/advance` (системные переходы P-02, P-08, P-10, P-16 вручную, без планировщика).

## Commit Plan
- **Commit 1** (после задач 1–3): `docs(api): add OpenAPI spec for cabinet API`
- **Commit 2** (после задач 4–7): `feat(mockapi): add in-memory mock server generated from OpenAPI`
- **Commit 3** (после задач 8–9): `test(mockapi): validate every operation against the spec; add README`

## Tasks

### Phase 1: Спецификация

- [x] **Task 1: Общая часть спецификации.** Создать `api/openapi.yaml`: `info` (описание на русском, ссылка на `docs/README.md`), `servers` (`http://localhost:8080/api/v1`), теги из раздела «Состав API», `securitySchemes` (cookie `session`), общие параметры (`limit`, `cursor`, `X-Request-Id`), общие ответы (`400`, `401`, `403`, `404`, `409`, `422`) и схемы `Error`, `Money` (int64, копейки), `Uuid`, `Timestamp`, `Page`. Все перечисления из 04 §9 — отдельными схемами `components/schemas/*Status`, `LegalType`, `TaxRegime`, `VatRate`, `TokenCarrier`, `TokenPosition`, `PlacementFormatId` и т. д.
  Логирование: в спецификации не применимо; описать заголовок ответа `X-Request-Id`, по которому запрос находится в журнале заглушки.
  Файлы: `api/openapi.yaml`.

- [x] **Task 2: Операции кабинетов рекламодателя и блогера** (depends on 1). Описать теги auth, organizations, catalog, cart, campaigns, media, orders/placements, balance, blogger, notifications по разделу «Состав API». Для каждой операции: `operationId` в camelCase, схемы запроса и ответа, коды ответов, пример (`example`) с правдоподобными выдуманными данными. Схемы ресурсов строить из полей 04 (только поля, видимые пользователю: без `*_hash`, `storage_key`, `lock_version`, служебных `sync_*`; блогеру не отдавать баланс рекламодателя, рекламодателю — реквизиты блогера, 07 §4). У операций-переходов в описании указать номер перехода (`P-03`), допустимые исходные состояния и ответ `409 invalid_transition`.
  Логирование: не применимо.
  Файлы: `api/openapi.yaml`.

- [x] **Task 3: Операции администратора, webhook и `_mock`** — админка отложена по решению пользователя (2026-10-15); сделаны webhook и `_mock` (depends on 1). Описать тег admin (поле `reason` обязательно в теле каждого действия), webhooks (тело — произвольный JSON, ответ `200`), `_mock` (описание: «только в тестовом сервере»). Проверить спецификацию: `npx @redocly/cli lint api/openapi.yaml` без ошибок; предупреждения о незаполненных `license` и т. п. допустимы.
  Логирование: не применимо.
  Файлы: `api/openapi.yaml`, при необходимости `api/redocly.yaml` с отключёнными лишними правилами.

### Phase 2: Тестовый сервер

- [x] **Task 4: Модуль и генерация** (depends on 3). Предусловие: установлен Go ≥ 1.24 (сейчас на машине Go не найден — поставить). Создать `mockapi/go.mod` (модуль `maxim/mockapi`), инструмент `oapi-codegen` подключить директивой `tool` в `go.mod`; конфиг `mockapi/oapi-codegen.yaml` (`generate: models, std-http-server, strict-server, embedded-spec`; пакет `api`; вывод `mockapi/internal/api/api.gen.go`). Директива `//go:generate go tool oapi-codegen -config ../../oapi-codegen.yaml ../../../api/openapi.yaml` в `mockapi/internal/api/generate.go`. Сгенерированный файл коммитится.
  Логирование: не применимо (генерация).
  Файлы: `mockapi/go.mod`, `mockapi/oapi-codegen.yaml`, `mockapi/internal/api/generate.go`, `mockapi/internal/api/api.gen.go`.

- [x] **Task 5: Каркас сервера** (depends on 4). `mockapi/cmd/mockapi/main.go`: адрес из `MOCKAPI_ADDR` (по умолчанию `:8080`), `LOG_LEVEL` (по умолчанию `debug`), корректная остановка по сигналу. Промежуточные слои: сквозной `request_id` (из `X-Request-Id` или новый UUID, возвращается в ответе), журнал запроса, восстановление после паники → `500` в формате `Error`, проверка запроса по спецификации (`oapi-codegen/nethttp-middleware` + `kin-openapi`; ошибки → `422 validation_failed`), сессия по cookie. Маршруты `GET /docs` (Swagger UI с cdnjs, точная версия) и `GET /openapi.yaml`. Подменяемые часы `internal/clock`.
  Логирование: `slog` JSON в stdout; обязательные поля `component=mockapi`, `op` в виде `Модуль.Операция`, `request_id`; INFO — старт/остановка сервера с адресом, каждый запрос (метод, путь, статус, длительность); DEBUG — тело ошибки валидации, найденная сессия (`user_id`, без токена); ERROR — паника со стеком. Токен сессии и код входа в журнал не пишутся (07 §5.4).
  Файлы: `mockapi/cmd/mockapi/main.go`, `mockapi/internal/server/*.go`, `mockapi/internal/clock/clock.go`.

- [x] **Task 6: Хранилище и начальные данные** (depends on 4). `internal/store`: структуры в памяти под защитой `sync.RWMutex`, поиск с ограничением по организации пользователя (07 §4). `internal/seed`: фиксированные UUID; 3 пользователя и 3 организации (рекламодатель ООО, блогер ИП, платформа), 8 категорий, 3 формата из 03 §2, 12 каналов со статусами `active`/`moderation`/`rejected` и метриками, цены, корзина с 2 позициями, 3 кампании (`draft` без заказа; `ordered` с размещениями в `awaiting_blogger`, `scheduled`, `published`; `ordered` с размещениями в `removed`, `disputed`, `paid`), 1 спор, 1 выплата, счета, проводки, 2 ошибки ОРД, уведомления, записи журнала действий. Все названия явно выдуманные («Тестовый канал о кулинарии»), почты на `example.test`.
  Логирование: INFO при загрузке начальных данных (сколько объектов каждого вида) и при `_mock/reset`; DEBUG — промах поиска с видом объекта и `id`.
  Файлы: `mockapi/internal/store/*.go`, `mockapi/internal/seed/seed.go`.

- [x] **Task 7: Обработчики** — без раздела admin (отложен 2026-10-15) (depends on 5, 6). Реализовать интерфейс strict server целиком по группам (по файлу на тег в `internal/handlers/`). Переходы размещения — через одну таблицу `internal/store/transitions.go` по 03 §4 (из состояния, в состояние, кто запускает); недопустимый переход → `409 invalid_transition` с текущим состоянием в `details`. Деньги при оформлении: `total_amount = blogger_amount + fee`, комиссия 15 % (`fee_rate_bp = 1500`, константа с комментарием `// A-30, Q-22`), округление вниз до копейки. Таймеры: при каждом запросе проверять T-01, T-05, T-13 относительно подменяемых часов и выполнять P-05, P-13, P-18. `_mock/*` — по описанию в спецификации. Сложных проверок (баланс, ОРД, Рокет Ворк) нет: оформление заказа проходит, если баланса хватает, иначе `409`.
  Логирование: INFO — каждый выполненный переход (`placement_id`, `from`, `to`, `transition`), оформление заказа (`order_id`, `total_amount`), действие администратора (`action`, `reason`); WARN — отклонённый переход (`placement_id`, `status`, `transition`), отказ в доступе к чужому объекту; DEBUG — входные параметры фильтров каталога и число найденных строк.
  Файлы: `mockapi/internal/handlers/*.go`, `mockapi/internal/store/transitions.go`.

### Phase 3: Проверка и запуск

- [ ] **Task 8: Тесты** (depends on 7). `mockapi/internal/server/contract_test.go`: поднять сервер через `httptest`, войти каждым из трёх пользователей; для каждой операции из спецификации (перебор `paths` из встроенной спецификации) выполнить запрос с данными из `example` и id из начальных данных; ответ проверить `openapi3filter.ValidateResponse`; тест падает, если какая-то операция не покрыта. Отдельно: сквозной сценарий «корзина → кампания → пост → оформление → `_mock/advance` P-02 → блогер принимает и планирует → `_mock/clock` → `advance` P-08, P-10 → рекламодатель принимает»; повторный `accept` даёт `409`. Тест на `api/openapi.yaml`: `openapi3.NewLoader().LoadFromFile` + `Validate`.
  Логирование: в тестах уровень `warn`, чтобы вывод не засорялся; при падении печатать `request_id` и тело ответа.
  Файлы: `mockapi/internal/server/contract_test.go`, `mockapi/internal/server/scenario_test.go`, `mockapi/internal/api/spec_test.go`.

- [ ] **Task 9: Запуск и README** (depends on 7). `mockapi/README.md`: что это и чем не является (не каркас продукта, данные выдуманные, всё в памяти), установка Go, `go generate ./...`, `go run ./cmd/mockapi`, адрес Swagger UI, готовые пользователи и код `000000`, как пройти сквозной сценарий из задачи 8 через Swagger UI, что не реализовано (CSRF, реальные проверки, файлы, внешние системы). Строка в `docs/README.md` — ссылка на `api/openapi.yaml` и `mockapi/README.md` с пометкой «при расхождении правы документы 01–08». `Makefile` не заводить; команды — в README. Добавить в `.gitignore` бинарник `mockapi/mockapi.exe`.
  Логирование: в README описать `LOG_LEVEL` и пример строки журнала.
  Файлы: `mockapi/README.md`, `docs/README.md`, `.gitignore`.

## Риски
- Go на машине не установлен — задачи 4–9 начинаются с установки (`winget install GoLang.Go`).
- Спецификация большая (около 90 операций). Если нужно быстрее увидеть результат, задачи 3 и раздел admin в задаче 7 можно отложить: тест покрытия из задачи 8 тогда сверяет только описанные операции.
- Экраны 08 в основном «не разобраны»: состав полей в API может поменяться после согласования экранов. Спецификация — черновик к этапам, а не утверждённый контракт.
