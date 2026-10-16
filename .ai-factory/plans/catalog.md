# План реализации: Каталог

Branch: master (без отдельной ветки, по решению пользователя; `feature/channel-connection` влита в `master` перед этапом)
Created: 2026-10-16

## Settings
- Testing: yes. Цены, комиссия, интервалы недоступности, видимость в каталоге, модерация, правило РКН и расчёт метрик проверяются чистыми табличными тестами `channel/domain`. Сценарии `channel/app` с `postgres` — на настоящей PostgreSQL через `testcontainers-go` с `clock.Manual` и `maxfake`. Метод `GET /messages` — через `httptest`-сервер с записанными ответами. Операции кабинетов и администратора — через `httptest` поверх собранного сервера (07 §9).
- Logging: verbose. `log/slog` JSON, поля и обязательные события по 07 §5. Для этапа это действия блогера и администратора, переходы состояния канала, внешние вызовы MAX при сборе метрик, запуски периодических заданий. Текст сообщений канала, ИНН и почта в журнал не пишутся (07 §5.4).
- Docs: yes. В конце обязательная точка `/aif-docs`: README бэкенда (новые переменные окружения, команда `maxim admin grant`, как в разработке провести канал до каталога).

## Roadmap Linkage
Milestone: "Каталог"
Rationale: план целиком реализует этот этап: метрики каналов по расписанию, категории (справочник уже есть, этап даёт фильтр), цены по форматам, недоступность, модерацию каналов, проверку реестра РКН, фильтры и сортировку каталога.

## Research Context
Source: .ai-factory/RESEARCH.md (Active Summary), выдержка по этапу

- Метрики каталога обновляются раз в сутки; каждый канал проходит ручную модерацию; список категорий фиксированный; цену за формат ставит блогер, комиссия платформы добавляется сверху; принимаются только публичные каналы (A-14).
- Каналы от 10 000 подписчиков обязаны быть в реестре РКН, иначе реклама в них запрещена (D-11); номер проверяет администратор вручную (Q-34).
- Предел MAX — 30 запросов в секунду на платформу; 2 000 каналов × суточные метрики укладываются в него с запасом (07 §7).

## Источники ТЗ

- Сценарии: 02 S-02 (поиск в каталоге), S-13 (цены и форматы), S-16 (модерация канала), S-32 (обновление метрик каталога).
- Данные: 04 §3 (`channel`, `channel_metrics`, `channel_category`, `placement_format`, `channel_price`, `channel_unavailability`), §9 (`channel.status`, `channel.rkn_status`).
- Форматы и комиссия: 03 §2, 06 §3 (`fee_rate_bp`, округление вниз), 06 §6.3 (реестр РКН).
- Интеграции: 05 §2.3 (`GET /messages?chat_id=`, `GET /chats/{chatId}`), §2.7 (расчёт метрик и флаги аномалий), §2.8 (приоритеты ограничителя).
- Требования: 07 §2 (`refresh_channel_metrics`: раз в сутки на канал, 3 попытки, запуски распределены по суткам), §4 (роли администратора `moderator`/`senior`, CSRF), §5 (журнал), §7 (каталог с фильтрами — до 1 с на 2 000 каналах), §8 (конфигурация).
- Реестр: A-01, A-08, A-14, A-30, D-11, D-19, D-29, Q-04, Q-08, Q-22, Q-34.
- Архитектура: `.ai-factory/ARCHITECTURE.md` — модуль `channel`, правило «внешний вызов только из задания», `ForAdmin`-выборки после проверки роли.
- Существующий код: `internal/channel/{domain,app,postgres,httpapi,jobs}` (состояния R-8/R-9 из этапа «Подключение каналов», `UpdateProfile`, `nextRknStatus`, `AvailabilityListener`), `internal/integration/max` (`Client.do`, `GetChat`, бакеты), `internal/platform/job` (`RegisterPeriodic`, `DailyAt`, `Spread`), `internal/platform/money`, `internal/platform/httpx` (`RequireCabinet`, `Principal.AdminRole`), `internal/platform/audit`, `internal/bootstrap/{channel.go,config.go,modules.go,worker.go,unimplemented.go}`, `mockapi/internal/handlers/catalog.go` (поведение фильтров тестового сервера).

## Границы этапа

Входит:
- Справочник `placement_format` (03 §2) и `listPlacementFormats`.
- Цены по форматам: `setBloggerChannelPrices`, `prices` в `BloggerChannel` с ценой для рекламодателя.
- Недоступность: `addChannelUnavailability`, `deleteChannelUnavailability`, `unavailability` в `BloggerChannel` и карточке каталога.
- Каталог рекламодателя: `listCatalogChannels` (поиск, фильтры, сортировка, постраничный вывод), `getCatalogChannel` (карточка с историей метрик за 30 дней).
- Метрики: метод MAX `GET /messages?chat_id=`, суточное задание `refresh_channel_metrics`, таблица `channel_metrics`, флаги аномалий.
- Правило РКН: канал, переросший порог без подтверждённого номера, → `suspended` с причиной `rkn_required` (S-32 шаг 2); возврат после подтверждения.
- Модерация каналов и проверка номера РКН администратором: операции администратора **только для каналов** в `api/openapi.yaml` и их реализация. Команда `maxim admin grant` для выдачи роли администратора.

Не входит (следующие этапы):
- Регистрация площадки в ОРД при одобрении и условие «канал не показывается, пока площадка не зарегистрирована» (S-16 шаг 3 и отклонение): этап «Маркировка через Яндекс ОРД». Он добавит задание в транзакцию одобрения и условие видимости.
- Признак `in_cart` и занятые размещениями интервалы в карточке: этапы «Заказ и размещения». До них `in_cart = false`, в `unavailability` — только интервалы блогера.
- Проверка, что новый интервал недоступности не пересекается с запланированными размещениями: этап «Заказ и размещения» (порт R-8).
- Уведомления блогеру об одобрении, отказе и `rkn_required` (07 §6): этап «Уведомления и отчёт по кампании». До него событие пишется в журнал.
- Блокировка канала, правка справочников, модерация постов и прочие операции администратора: этап «API администратора».
- Снижение рейтинга по спорам (A-08): этап «Приёмка, споры и возвраты». Здесь `rating` только читается и сортируется.

## Решения этапа, требующие подтверждения

Документы этих мест не определяют или противоречат друг другу. План реализует указанный вариант. После подтверждения задача 14 вносит его в документы. При отказе меняется только отмеченная задача.

| # | Вопрос | Решение в плане | Задача |
|---|---|---|---|
| R-1 | Модерация есть в двух этапах, операций администратора в OpenAPI нет | В этом этапе — операции администратора для каналов под тегом `admin-channels`: `listAdminChannels` (фильтры `status`, `rkn_status`, `rkn_recheck_due`, курсор), `getAdminChannel` (сведения MAX, метрики, флаги, история модерации из `audit_log`), `approveChannel`, `rejectChannel` (причина обязательна, до 500 символов), `setChannelRknCheck` (`confirmed`/`rejected`). Доступ — роли `moderator` и `senior` (07 §4), CSRF обязателен. Остальные операции администратора — этап «API администратора» | 1, 8, 11 |
| R-2 | Как получить администратора: роль ставится только записью в базу | Команда `maxim admin grant --email <почта> --role moderator\|senior` (и `--revoke`): создаёт пользователя, если его нет, пишет `audit_log` с актором `system`. Работает под ролью владельца базы, как `migrate` | 12 |
| R-3 | Ограничение `active` «хотя бы одна включённая цена» не выражается `CHECK` (04 §3) | Отложенный триггер-ограничение (`CONSTRAINT TRIGGER … DEFERRABLE INITIALLY DEFERRED`) на `channel` и `channel_price`: при фиксации транзакции у канала в `active` есть включённая цена активного формата. Основная проверка — в `domain`, триггер — страховка | 2, 3 |
| R-4 | Блогер выключает все цены у канала в `active` (S-13: «без цены канал не выходит в каталог») | `422` с `details.reason = active_requires_price`: у активного канала должна остаться хотя бы одна включённая цена. Остальным состояниям выключать все цены можно | 3, 6 |
| R-5 | Правила набора цен | `PUT` передаёт полный набор; формат — из активных, без повторов; цена — от 10 000 до 1 000 000 000 копеек (04 §3). Формат, не переданный в наборе, удаляется. Цена для рекламодателя `total = amount + floor(amount × PLATFORM_FEE_RATE_BP / 10000)` (06 §3, A-30: 1500). Правка доступна в `pending_rights`, `moderation`, `active`, `suspended`, `rejected`; `409` для `unclaimed`, `disconnected`, `blocked`. Журнал действий `channel.prices_updated` с `audit.Diff` | 3, 6 |
| R-6 | Правила интервалов недоступности (04 §3 говорит только о `starts_at`/`ends_at`) | `starts_at < ends_at`, `ends_at > now`; длина — не больше 90 суток; начало — не дальше 365 суток от текущего момента; пересечение с существующим интервалом → `422` (`overlap`); живых интервалов не больше 50. Удалить можно любой интервал своего канала. Прошедшие интервалы в ответах не показываются и удаляются ежесуточной очисткой (`cleanup`) через 30 суток после окончания | 3, 6 |
| R-7 | Условие видимости в каталоге | `status = active` и (`subscribers_count < RKN_SUBSCRIBERS_THRESHOLD` или `rkn_status = confirmed`) и есть включённая цена активного формата. Канал, который не проходит условие, в каталоге не виден и в карточке даёт `404`. Условие о площадке ОРД добавит этап «Маркировка» | 3, 5, 7 |
| R-8 | Проверка недоступности против размещений | Порт `channel/domain.PlacementSchedule` (`BusyIntervals`) с пустой реализацией в `bootstrap`. Модуль размещений подставит свою реализацию, и тогда карточка покажет занятые интервалы | 3, 7, 12 |
| R-9 | Сортировки: S-02 просит подписчиков, просмотры, ERR, цену, CPV, а в спецификации нет просмотров и CPV | В `sort` добавляются `avg_views_desc` и `cpv_asc`. CPV = цена выбранного формата (или минимальная включённая, если формат не задан) / `avg_views`, в копейках, округление вниз. Без `format_id` цена для фильтра и сортировки — минимальная включённая цена канала (как в `mockapi`). Каналы без значения метрики идут в конце. При равенстве порядок — по `id` | 1, 5, 7 |
| R-10 | Постраничный вывод при разных сортировках | Курсор по ключу: base64url от JSON `{s: sort, v: значение ключа, id}`. Курсор другой сортировки или испорченный → `422` (`invalid_cursor`). Набор фильтров в курсор не входит: при их смене кабинет начинает с первой страницы | 5, 7 |
| R-11 | `sqlc` не умеет переменный `ORDER BY` и необязательные фильтры с ключевым курсором | Запрос каталога пишется руками на `pgx` в `channel/postgres/catalog.go`: условия добавляются из закрытого списка, имя сортировки сопоставляется с заранее заданным выражением, все значения — параметры. Цена для рекламодателя считается в SQL той же формулой с `fee_rate_bp` параметром. Поиск `q` — `ILIKE` по `title` и `description` с экранированием `%` и `_`. Исключение из «только `sqlc`» записывается в ARCHITECTURE | 5, 14 |
| R-12 | Кто видит каталог | Пользователь с открытым кабинетом рекламодателя (`403` иначе), как в `mockapi`. `listPlacementFormats` и `listChannelCategories` — любой вошедший пользователь | 7, 10 |
| R-13 | Расписание метрик: «раз в сутки на канал, запуски распределены по суткам» (07 §2) | Периодическое `schedule_channel_metrics` (раз в сутки, 00:05 МСК, 1 попытка) ставит по каждому каналу в `moderation`, `active`, `suspended` задание `refresh_channel_metrics` с `run_at` = начало суток + смещение `job.Spread` по `channel_id` в окне 24 ч и `dedupe_key` `channel_metrics:{id}:{date}`. `moderation` добавлена, чтобы администратор видел метрики в очереди (S-16 шаг 1). Приоритет ограничителя — фоновый | 9 |
| R-14 | Расчёт метрик при пределе `GET /messages` в 100 сообщений | Окно — 30 суток, не больше 5 страниц по 100. `avg_views` — среднее `stat.views` по не более чем 100 последним сообщениям старше 24 часов. `posts_per_week` — сообщения за 28 суток / 4; если окно не уместилось в 5 страниц, значение считается по найденным и пишется WARN. Сообщения бота платформы входят в расчёт. Нет `stat.views` ни у одного сообщения (Q-08) или сообщений нет (Q-04) → `avg_views`, `engagement_bp`, `posts_per_week` пусты, каталог показывает только подписчиков (05 §2.7) | 3, 4, 9 |
| R-15 | Пороги флагов аномалий — «параметры конфигурации» без имён | `METRICS_SUBSCRIBERS_SPIKE_BP` (2000 — рост больше 20% к снимку прошлых суток), `METRICS_VIEWS_MIN_BP` (100 — `avg_views` меньше 1% подписчиков). `views_disproportion` также при `avg_views > subscribers_count`. Без снимка прошлых суток `subscribers_spike` не ставится | 3, 12 |
| R-16 | Правило РКН при смене числа подписчиков | В `domain.target`: канал в `active` или `suspended`, у которого подписчиков не меньше порога, а `rkn_status` не `confirmed`, → `suspended` с причиной `rkn_required`. Если подписчиков стало меньше порога или номер подтверждён, причина `rkn_required` снимается и канал возвращается в `active` без модерации (как при правах, R-8 прошлого этапа). Подписчики обновляются и в `refresh_channel_info`, и в `refresh_channel_metrics`; правило срабатывает в обоих. Номер блогер вводит через `updateBloggerChannel` (`rkn_status = pending`); `rkn_status = rejected` оставляет канал в `suspended`. Переход вызывает `AvailabilityListener` | 3, 9 |
| R-17 | Повторная проверка РКН раз в 90 суток (06 §6.3: «канал помечается для администратора») | Отдельного состояния нет. Признак `rkn_recheck_due` считается: `rkn_status = confirmed` и `rkn_checked_at` старше `RKN_RECHECK_INTERVAL` (2160h). Его видно в `AdminChannel` и по фильтру `listAdminChannels`; канал продолжает работать | 3, 8 |
| R-18 | Предусловия одобрения (S-16) | `approveChannel` только из `moderation`; нужны права и публичность в порядке, категория, включённая цена, а при подписчиках не меньше порога — `rkn_status = confirmed` (иначе `409` с `details.missing`: `category`, `price`, `rkn`, `rights`). Одобрение: `active`, `moderated_at`, `moderated_by`, `audit_log` `channel.approved`. Отказ: только из `moderation`, `rejected` с текстом причины в `status_reason`, `channel.rejected`. `setChannelRknCheck` — для любого состояния, кроме `unclaimed` и `disconnected`, при заданном номере; пишет `rkn_checked_at`, `rkn_checked_by`, затем применяет R-16 | 3, 8 |
| R-19 | Хранение снимков метрик | Строки `channel_metrics` хранятся бессрочно (не больше 365 строк на канал в год, объём мал). Карточка и экран администратора показывают последние 30 снимков. Повтор задания в те же сутки перезаписывает снимок (`ON CONFLICT (channel_id, date) DO UPDATE`). Дата — по Москве | 2, 5, 9 |
| R-20 | Новые виды заданий, которых нет в 07 §2 | `schedule_channel_metrics` (раз в сутки, 00:05 МСК, 1). `refresh_channel_metrics` уже есть в таблице: 3 попытки, `dedupe_key` по R-13; `404` от MAX → `Permanent` и WARN (канал отключат события бота) | 9, 14 |

## Commit Plan
- **Commit 1** (после задач 1–2): `feat(catalog): api contract and catalog schema`
- **Commit 2** (после задач 3–5): `feat(catalog): domain rules, MAX messages and catalog storage`
- **Commit 3** (после задач 6–9): `feat(catalog): prices, unavailability, catalog, moderation and metrics scenarios`
- **Commit 4** (после задач 10–12): `feat(catalog): cabinet and admin channel API, admin grant command and wiring`
- **Commit 5** (после задач 13–14): `test(catalog): catalog e2e and docs sync`

## Tasks

### Phase 1: Договор и данные

- [x] **Task 1: Спецификация каталога и операций администратора для каналов (R-1, R-9)**
  - `api/openapi.yaml`:
    - `listCatalogChannels`: в `sort` добавить `avg_views_desc`, `cpv_asc`; описать R-7 (условие видимости), R-9 (цена без `format_id` — минимальная включённая, порядок пустых значений), `422` для испорченного курсора (R-10). В `CatalogChannel` — поле `cpv` (`Money`, необязательное: цена выбранного формата или минимальная / `avg_views`).
    - `setBloggerChannelPrices`: `409` для недопустимого состояния, `422` с причинами `active_requires_price`, `duplicate_format`, `unknown_format`. `addChannelUnavailability`: `409`, `422` с причинами `overlap`, `too_long`, `too_far`, `in_past`, `too_many` (R-6).
    - Тег `admin-channels`, пути `/admin/channels`, `/admin/channels/{id}`, `/admin/channels/{id}/approve`, `/admin/channels/{id}/reject`, `/admin/channels/{id}/rkn-check`. Схемы `AdminChannel` (поля `BloggerChannel` + организация (`id`, название, ИНН маскированный), `max_description`, `link`, `subscribers_count`, `avg_views`, `engagement_bp`, `posts_per_week`, `anomaly_flags`, `metrics_history`, `rkn_checked_at`, `rkn_recheck_due`, `moderated_at`, `moderation_history`), `ChannelRejectInput` (`reason` 1…500), `ChannelRknCheckInput` (`result: confirmed|rejected`, `comment`). Ответы `403` (нет роли), `404`, `409` с `details.missing` (R-18).
  - `go generate ./internal/apigen`. `mockapi` собирается из той же спецификации без тега `_mock`: добавить заглушки новых операций администратора (`501`) в тестовый сервер и поддержать новые сортировки в `mockapi/internal/handlers/catalog.go`, чтобы контрактный тест `mockapi` проходил.
  - Журнал: не применимо.
  - Тесты: существующие тесты `bootstrap` и `mockapi` (`contract_test.go`) проходят; операции этапа отвечают `501` до задач 10–11.
  - Файлы: `api/openapi.yaml`, `internal/apigen/api.gen.go`, `mockapi/internal/api/api.gen.go`, `mockapi/internal/handlers/{catalog.go,mock.go}`.

- [x] **Task 2: Миграция каталога (R-3, R-19)**
  - `migrations/00005_catalog.sql` по 04 §3:
    - `placement_format` (`id text` PK, `name`, `top_hours`, `duration_hours`, `is_active`, `position`), строки `f_1_24`, `f_2_48`, `f_3_72` из 03 §2.
    - `channel_price` (`id uuid`, `channel_id` → `channel`, `format_id` → `placement_format`, `amount bigint check 10000..1000000000`, `is_enabled`, `created_at`, `updated_at`, уникальная пара `(channel_id, format_id)`).
    - `channel_unavailability` (`id uuid`, `channel_id`, `starts_at`, `ends_at`, `created_at`, `check (starts_at < ends_at)`), индекс `(channel_id, ends_at)`.
    - `channel_metrics` (поля 04 §3, `created_at`, уникальная пара `(channel_id, date)`, `anomaly_flags text[] not null default '{}'` с проверкой значений).
    - Отложенный триггер-ограничение `channel_active_requires_price` (R-3) на `channel` (`INSERT`/`UPDATE OF status`) и `channel_price` (`UPDATE`/`DELETE`). Комментарий в `channel_active_requirements` из `00004` обновить: условие о цене закреплено триггером.
    - Индекс каталога: `channel (status) WHERE status = 'active'` и `channel (status, rkn_status, rkn_checked_at)` для очереди администратора.
  - Права роли — через `ALTER DEFAULT PRIVILEGES` из `00002`; проверить, что новые таблицы покрыты. Удаление строк `channel_price` и `channel_unavailability` роли приложения разрешено (это не неизменяемые таблицы 04 §1).
  - Журнал: не применимо (INFO `migrations applied` пишет `migrate`).
  - Тесты: `up` → `down` → `up`; роль приложения читает и пишет новые таблицы; канал `active` без включённой цены не фиксируется (ошибка на `COMMIT`); выключение последней цены активного канала не фиксируется; второй снимок метрик на ту же дату не вставляется без `ON CONFLICT`.
  - Файлы: `migrations/00005_catalog.sql`.
<!-- Commit checkpoint: tasks 1-2 -->

### Phase 2: Предметные правила, адаптер, хранение

- [x] **Task 3: `channel/domain` — цены, недоступность, видимость, модерация, РКН, метрики (R-3 … R-9, R-14 … R-18)** (depends on 2)
  - `internal/channel/domain`:
    - `Format` (поля `placement_format`), `Price{FormatID, Amount money.Kopecks, IsEnabled}`, `TotalAmount(amount, feeRateBP) money.Kopecks` через `platform/money` (округление вниз). `(*Channel).SetPrices(prices, activeFormats, now) error` по R-4, R-5 (ошибки `ErrActiveRequiresPrice`, `ErrDuplicateFormat`, `ErrUnknownFormat`, `ErrNotEditable`, `ErrPriceOutOfRange`). `Channel` получает поле `Prices []Price` (загружается репозиторием).
    - `Unavailability{ID, StartsAt, EndsAt}`, `ValidateUnavailability(new, existing, now, limits) error` по R-6 с отдельной ошибкой на каждую причину.
    - `CatalogVisible(c, rknThreshold) bool` по R-7.
    - Модерация: `Approve(adminID, rknThreshold, now) (StatusChange, error)` и `Reject(adminID, reason, now)` по R-18; `ErrApproveBlocked` несёт список `missing`. `RknChecked(result, adminID, rknThreshold, now) (StatusChange, error)`. `RknRecheckDue(now, interval) bool` (R-17).
    - Правило РКН в `target` (R-16): причина `ReasonRknRequired = "rkn_required"`; `target` и `resolve` получают порог (поле в `Channel` не хранить: передавать параметром, обновить вызовы `RightsRefreshed`, `UpdateProfile`, `LinkTo`). Возврат `suspended → active` при снятой причине — по тем же правилам, что права.
    - Метрики: `ComputeMetrics(subscribers int, messages []MessageStat, prev *MetricsSnapshot, now time.Time, cfg MetricsConfig) MetricsSnapshot` — чистая функция по 05 §2.7 и R-14, R-15. `(*Channel).MetricsRefreshed(info ChatInfo, rknThreshold, now) StatusChange` обновляет сведения MAX и подписчиков и применяет `resolve`.
    - Порты: `Prices`, `Unavailabilities`, `Formats`, `Metrics`, `Catalog` (выборка каталога и карточки), `AdminChannels` (`…ForAdmin`), `MessageReader` (`ListMessages`), `PlacementSchedule` (R-8).
  - Журнал: не применимо (`domain` не пишет журнал).
  - Тесты (табличные): `TotalAmount` на 10 000, 10 001, 1 000 000 000 при ставке 0, 1500, 10000; `SetPrices` для каждого состояния, повтор формата, выключение последней цены в `active` и в `moderation`; каждая причина отказа `ValidateUnavailability`, включая соприкосновение границ (не пересечение); `CatalogVisible` на пороге 9 999 / 10 000 × каждый `rkn_status`; `Approve` с каждым недостающим условием и их сочетанием; `Reject` вне `moderation` → ошибка; R-16: `active` 9 999 → 10 000 без номера → `suspended`/`rkn_required`, подтверждение → `active`, падение ниже порога → `active`, `rkn_rejected` → остаётся `suspended`; причина `missing_rights` не снимается подтверждением РКН; `ComputeMetrics`: пустой список, нет `stat.views`, сообщения моложе 24 ч не входят, больше 100 сообщений, рост подписчиков 19,99% / 20,01%, `avg_views` 0,99% / 1%, `avg_views > subscribers`, `subscribers = 0`.
  - Файлы: `internal/channel/domain/{price.go,unavailability.go,catalog.go,moderation.go,metrics.go,channel.go,ports.go,*_test.go}`.

- [x] **Task 4: Чтение сообщений канала в MAX (R-14)** (depends on 2)
  - `internal/integration/max/messages.go`: `ListMessages(ctx, chatID, from, to time.Time, count int) ([]Message, error)` — `GET /messages?chat_id=&from=&to=&count=`; из ответа берутся идентификатор, `timestamp`, отправитель (признак бота), `stat.views` (может отсутствовать). Текст сообщения не разбирается и не хранится. Приоритет ограничителя — фоновый (`PriorityBackground`), если в `context` не задан другой.
  - `maxfake`: `SetMessages(chatID, []Message)`, учёт интервала `from`/`to` и `count`, `FailWith(err)` для этого метода.
  - Записанные ответы: `testdata/messages.json` (со `stat.views`), `messages_no_stat.json`, `messages_empty.json`. Структуру сверить со схемой `max-messenger/api-schema`.
  - Журнал: DEBUG `max messages read` (`chat_id`, `count`, `from`, `to`). Текст сообщений не пишется.
  - Тесты (`httptest`): разбор каждого файла; отсутствие `stat` → `Views == nil`; параметры запроса (`from`/`to` в миллисекундах — сверить с документацией MAX); `403` (бот не администратор) → `Permanent`.
  - Файлы: `internal/integration/max/{messages.go,messages_test.go,testdata/messages*.json}`, `internal/integration/max/maxfake/{fake.go,channels.go}`.

- [x] **Task 5: `channel/postgres` — цены, недоступность, метрики, каталог, выборки администратора (R-7, R-9 … R-11, R-19)** (depends on 3)
  - Запросы `sqlc`: форматы (активные по порядку); цены канала (список, замена набора в транзакции: удаление отсутствующих, `upsert` переданных); интервалы (живые интервалы канала, вставка, удаление своего интервала с проверкой организации, удаление прошедших старше срока для `cleanup`); метрики (`upsert` снимка, последний снимок до даты, последние N снимков); каналы для расписания метрик (`moderation`, `active`, `suspended`, порциями по `id`); выборки администратора `ListForAdmin` (фильтры R-1, курсор по `(updated_at, id)`), `GetForAdmin`.
  - `channel/postgres/catalog.go` на `pgx` по R-11: `ListCatalog(ctx, filter, sort, cursor, limit, feeRateBP, rknThreshold)` и `GetCatalogChannel(ctx, id, feeRateBP, rknThreshold)`. Последний снимок метрик соединяется `LATERAL`-подзапросом. Сортировка выбирается из таблицы `sortExpr` по имени, значения — только параметры. Условие R-7 — в SQL.
  - Загрузка `Channel.Prices` в существующих методах получения канала.
  - Журнал: DEBUG `catalog queried` (`sort`, `filters` — имена заданных фильтров без значения `q`, `count`, `elapsed_ms`); DEBUG `channel prices saved` (`channel_id`, `count`).
  - Тесты (PostgreSQL): каждый фильтр по отдельности и вместе; каждая сортировка и порядок пустых значений; обход всех страниц курсором даёт каждый канал ровно один раз при равных значениях ключа; канал без цены, в `moderation`, с 10 000 подписчиков без подтверждения в выдачу не попадают; `q` с `%` и `_` ищет буквально; 2 000 каналов × 3 цены × 30 снимков — запрос с фильтрами и сортировкой `cpv_asc` укладывается в 1 с (07 §7, тест с меткой `-short` пропускает); `ListForAdmin` без роли не вызывается (проверяет `app`).
  - Файлы: `internal/channel/postgres/{catalog.go,prices.go,unavailability.go,metrics.go,admin.go,channels.go,*_test.go}`, `internal/channel/postgres/queries/{formats,prices,unavailability,metrics,admin}.sql`, `internal/channel/postgres/sqlc/*`.
<!-- Commit checkpoint: tasks 3-5 -->

### Phase 3: Сценарии

- [x] **Task 6: Цены и недоступность в кабинете блогера (R-4 … R-6, R-8)** (depends on 5)
  - `internal/channel/app`:
    - `SetPrices(ctx, orgID, userID, channelID, prices)` — `FOR UPDATE` канала организации, `domain.SetPrices`, сохранение набора, `audit_log` `channel.prices_updated` (до и после), всё в одной транзакции.
    - `AddUnavailability`, `DeleteUnavailability` — проверка по R-6 под блокировкой канала (два одновременных запроса не создают пересечение), `audit_log` `channel.unavailability_added` / `channel.unavailability_deleted`.
    - `Formats(ctx)`.
    - `Get`/`List` возвращают цены с `total_amount` (ставка из конфигурации) и живые интервалы.
  - Журнал: INFO `channel prices updated` (`channel_id`, `enabled_formats`), WARN `channel prices rejected` (`channel_id`, `reason`), INFO `channel unavailability added` (`channel_id`, `unavailability_id`, `starts_at`, `ends_at`), INFO `channel unavailability deleted`, WARN `channel unavailability rejected` (`channel_id`, `reason`).
  - Тесты (PostgreSQL, `clock.Manual`): полный набор заменяет прежний, исключённый формат удалён; выключение последней цены у `active` → `422`, у `moderation` — сохранено; цена 99,99 ₽ → `422`; чужой канал → `404`; `disconnected` → `409`; два параллельных пересекающихся интервала — один принят; прошедший интервал не возвращается; `audit_log` содержит изменение цены.
  - Файлы: `internal/channel/app/{prices.go,unavailability.go,cabinet.go,*_test.go}`.

- [x] **Task 7: Каталог рекламодателя (R-7 … R-12)** (depends on 5)
  - `internal/channel/app/catalog.go`: `ListCatalog(ctx, query)` — проверка фильтров (`price_from <= price_to`, `subscribers_from <= subscribers_to`, `format_id` из активных, иначе `422`), разбор курсора (R-10), вызов `Catalog` с `fee_rate_bp` и порогом РКН, сборка следующего курсора. `GetCatalogChannel(ctx, id)` — карточка: последние 30 снимков, `posts_per_week`, живые интервалы блогера и `PlacementSchedule.BusyIntervals` (R-8), `404` для невидимого канала. `in_cart = false` до этапа «Заказ и размещения».
  - Журнал: DEBUG `catalog listed` (`op` `Catalog.List`, `sort`, `count`, `has_next`); WARN `catalog query rejected` (`reason`). Значение `q` не пишется.
  - Тесты (PostgreSQL): видимый канал есть в выдаче и карточке, после выключения цен и перевода в `suspended` — `404`; испорченный курсор и курсор другой сортировки → `422`; `price_from > price_to` → `422`; цена в выдаче равна `amount + floor(amount × 1500 / 10000)`; CPV по R-9.
  - Файлы: `internal/channel/app/{catalog.go,catalog_test.go}`.

- [x] **Task 8: Модерация и проверка РКН администратором (R-1, R-17, R-18)** (depends on 5)
  - `internal/channel/app/moderation.go`: `AdminList`, `AdminGet` (с историей модерации из `audit_log` канала), `Approve(ctx, adminID, channelID)`, `Reject(ctx, adminID, channelID, reason)`, `SetRknCheck(ctx, adminID, channelID, result, comment)`. Каждый — транзакция с `FOR UPDATE`, `AvailabilityListener` при смене доступности, `audit_log` с актором `admin` (`channel.approved`, `channel.rejected`, `channel.rkn_checked`). Проверка роли — в `httpapi` (R-1); `app` принимает идентификатор администратора и вызывает только методы `…ForAdmin`.
  - Журнал: INFO `channel approved` (`channel_id`, `admin_id`), WARN `channel approve blocked` (`channel_id`, `missing`), INFO `channel rejected` (`channel_id`, `admin_id`), INFO `channel rkn checked` (`channel_id`, `result`, `admin_id`), INFO `channel status changed` (уже есть). Текст причины отказа и комментарий в журнал не пишутся (могут содержать персональные данные), только в `audit_log`.
  - Тесты (PostgreSQL, `clock.Manual`): одобрение канала с категорией и ценой → `active`, `moderated_at` и `moderated_by` записаны, канал виден в каталоге; без цены → `409` с `missing = [price]`; 12 000 подписчиков без подтверждения → `409` с `rkn`; отказ → `rejected` с причиной, блогер правит профиль → снова `moderation`; подтверждение РКН у `suspended`/`rkn_required` → `active`; отказ РКН → остаётся `suspended`; `rkn_recheck_due` через 90 суток + 1 мин.
  - Файлы: `internal/channel/app/{moderation.go,moderation_test.go}`.

- [x] **Task 9: Суточные метрики и правило РКН (R-13 … R-16, R-19, R-20)** (depends on 4, 5)
  - `internal/channel/app/metrics.go`:
    - `ScheduleMetrics(ctx, day)` — по R-13 порциями ставит `refresh_channel_metrics`; повторный запуск в те же сутки заданий не дублирует (`dedupe_key`).
    - `RefreshMetrics(ctx, channelID, day)` — вне транзакции: `GetChat` и `ListMessages` (R-14); затем в транзакции `FOR UPDATE`: если канал уже не в `moderation`/`active`/`suspended` — выйти; `ComputeMetrics` с прошлым снимком, `upsert` снимка, `MetricsRefreshed` (R-16), `AvailabilityListener` при смене доступности, `audit_log` `channel.status_changed` с актором `system` при `rkn_required`.
    - В существующем `refresh_channel_info` (`events.go`) — передать порог РКН, чтобы R-16 срабатывал и там.
  - `internal/channel/jobs/jobs.go`: `RegisterPeriodic(KindScheduleChannelMetrics, job.DailyAt(0, 5), …)`, `Register(KindRefreshChannelMetrics, …)` с 3 попытками; `404` → `job.Permanent`.
  - `cleanup`: удаление прошедших интервалов недоступности старше 30 суток (R-6) — запрос владельца таблицы, регистрируется в `platform/cleanup`.
  - Журнал: INFO `channel metrics scheduled` (`count`, `date`), INFO `channel metrics refreshed` (`channel_id`, `subscribers_count`, `avg_views`, `anomaly_flags`), WARN `channel metrics window truncated` (`channel_id`, `pages`), WARN `channel rkn required` (`channel_id`, `subscribers_count`), WARN `channel anomaly detected` (`channel_id`, `flags`), INFO `channel metrics skipped` (`channel_id`, `status`).
  - Тесты (PostgreSQL, `maxfake`, `clock.Manual`): расписание ставит задания только нужным состояниям, с разными `run_at` в пределах суток; повтор расписания — без новых заданий; снимок записан, повтор в те же сутки перезаписывает; 9 999 → 10 000 подписчиков у `active` без номера → `suspended`/`rkn_required`, канал пропал из каталога, `AvailabilityListener` вызван; `ListMessages` с ошибкой → повтор, канал не меняется; канал стал `disconnected` до запуска → задание завершено без вызова MAX записи; `refresh_channel_info` с ростом подписчиков → то же правило.
  - Файлы: `internal/channel/app/{metrics.go,metrics_test.go,events.go,service.go}`, `internal/channel/jobs/jobs.go`, `internal/channel/postgres/queries/unavailability.sql`.
<!-- Commit checkpoint: tasks 6-9 -->

### Phase 4: HTTP, команда, сборка

- [x] **Task 10: Операции кабинетов: цены, недоступность, форматы, каталог (R-12)** (depends on 6, 7)
  - `internal/channel/httpapi`: `SetBloggerChannelPrices`, `AddChannelUnavailability`, `DeleteChannelUnavailability`, `ListPlacementFormats`, `ListCatalogChannels`, `GetCatalogChannel`. Кабинет рекламодателя — `httpx.RequireCabinet(ctx, "advertiser")`. Преобразование в `apigen.CatalogChannel`, `CatalogChannelDetail`, `BloggerPrice`, `Unavailability`; `bloggerChannel` заполняет `prices` и `unavailability`.
  - Ошибки через `apperr`: `404`, `409`, `422` с `details.reason` из R-4 … R-6, R-10.
  - Журнал: WARN `403` пишет `httpx`; остальное — в `app`.
  - Тесты (`httptest` поверх собранного сервера): каталог без кабинета рекламодателя → `403`; ответы проходят проверку схемой; CSRF обязателен для `PUT`/`POST`/`DELETE`; `next_cursor` ведёт на следующую страницу; `422` для неизвестной сортировки отдаёт проверка запроса до обработчика.
  - Файлы: `internal/channel/httpapi/{handlers.go,catalog.go,prices.go,convert.go,handlers_test.go}`.

- [x] **Task 11: Операции администратора для каналов (R-1)** (depends on 8)
  - `internal/platform/httpx/auth.go`: `RequireAdmin(ctx, roles ...string) (*Principal, error)` — `401` без сессии, `403` с `details.reason = admin_required` без нужной роли.
  - `internal/channel/httpapi/admin.go`: `ListAdminChannels`, `GetAdminChannel`, `ApproveChannel`, `RejectChannel`, `SetChannelRknCheck` с ролями `moderator`, `senior`. Преобразование в `apigen.AdminChannel`; ИНН организации маскируется (07 §5.4 применяется и к ответу, где полное значение не нужно).
  - Журнал: WARN `admin access denied` (`op`, `user_id`, `required_roles`) пишет `RequireAdmin`; остальное — в `app`.
  - Тесты (`httptest`): обычный пользователь → `403`; `moderator` одобряет → `200`, канал в каталоге; `409` с `missing`; отказ без причины → `422`; сессия администратора действует 12 часов (уже реализовано — проверить на этих операциях).
  - Файлы: `internal/platform/httpx/{auth.go,auth_test.go}`, `internal/channel/httpapi/{admin.go,admin_test.go}`.

- [x] **Task 12: Конфигурация, команда `maxim admin grant` и сборка (R-2, R-8, R-13, R-15, R-17)** (depends on 9, 10, 11)
  - `internal/bootstrap/config.go`: `PLATFORM_FEE_RATE_BP` (1500, 0…10000), `RKN_RECHECK_INTERVAL` (2160h), `METRICS_SUBSCRIBERS_SPIKE_BP` (2000), `METRICS_VIEWS_MIN_BP` (100), `CHANNEL_UNAVAILABILITY_MAX_DAYS` (90), `CHANNEL_UNAVAILABILITY_HORIZON_DAYS` (365), `CHANNEL_UNAVAILABILITY_MAX_COUNT` (50). Проверка диапазонов при запуске.
  - `cmd/maxim/main.go` + `internal/bootstrap/admin.go`: подкоманда `admin grant|revoke` (R-2) через сервис `account/app` (новый метод `SetAdminRole` в `account/app` и запрос в `account/postgres`), журнал действий `user.admin_role_changed`.
  - `channel.go`, `modules.go`, `worker.go`: зависимости новых портов, пустой `PlacementSchedule` (R-8), `maxfake` реализует `MessageReader` в `APP_ENV=test`, регистрация заданий R-20 и запроса очистки. `unimplemented.go`: убрать заглушки реализованных операций.
  - `.env.example`, `compose.dev.yml`: новые переменные; в README-заметке для `/aif-docs` — как выдать роль и одобрить канал в разработке.
  - Журнал: INFO `admin role changed` (`user_id`, `role`, без почты); INFO `worker started` с новыми видами заданий.
  - Тесты: проверки конфигурации (ставка 10 001, отрицательные пороги); `wiring_test.go` — `schedule_channel_metrics` и `refresh_channel_metrics` зарегистрированы, все операции этапа больше не отвечают `501`; `admin grant` создаёт пользователя и ставит роль, повтор не меняет ничего, `revoke` снимает роль.
  - Файлы: `internal/bootstrap/{config.go,config_test.go,admin.go,channel.go,modules.go,worker.go,unimplemented.go,wiring_test.go}`, `cmd/maxim/main.go`, `internal/account/app/admin.go`, `internal/account/postgres/{users.go,queries/users.sql}`, `.env.example`, `compose.dev.yml`.
<!-- Commit checkpoint: tasks 10-12 -->

### Phase 5: Проверка и документы

- [x] **Task 13: Сквозные тесты каталога** (depends on 12)
  - `internal/bootstrap/catalog_e2e_test.go` на собранных `api` и `worker` (PostgreSQL, `clock.Manual`, `maxfake`, `mailfake`):
    - S-13 → S-16 → S-02: канал подключён (помощник из `channel_e2e_test.go`), блогер задаёт категорию и цены, добавляет интервал недоступности → администратор (роль через `admin grant`) видит канал в очереди, одобряет → рекламодатель находит канал по категории и цене, сортирует по `cpv_asc`, открывает карточку с интервалом блогера.
    - Отказ модерации с причиной → блогер правит описание → канал снова в очереди.
    - S-32: сутки проходят, `schedule_channel_metrics` и `refresh_channel_metrics` выполняются, в карточке появился снимок; рост до 10 000 подписчиков → канал пропал из каталога (`rkn_required`) → блогер вводит номер → администратор подтверждает → канал снова в каталоге.
    - Блогер пытается выключить все цены у активного канала → `422`, канал виден.
  - Проверка журнала сборщиком записей: обязательные события 07 §5.3 этапа (внешняя операция выполнена, действие администратора, смена состояния канала) присутствуют; значения `q`, ИНН, почта, текст причины отказа не встречаются.
  - Файлы: `internal/bootstrap/catalog_e2e_test.go`.

- [x] **Task 14: Синхронизация документов с решениями этапа** (depends on 12)
  - После подтверждения R-1 … R-20 внести их в документы, чтобы документы оставались правы:
    - 02 S-02 — сортировки и цена без формата (R-9), видимость (R-7); S-13 — правила цен и интервалов (R-4 … R-6); S-16 — предусловия одобрения (R-18), повторная проверка (R-17); S-32 — состояния и расписание (R-13), возврат из `rkn_required` (R-16).
    - 04 §3 — триггер ограничения `active` (R-3), поля `id`, `created_at` новых таблиц, хранение снимков (R-19), `status_reason = rkn_required`.
    - 05 §2.3 — параметры `GET /messages`; §2.7 — окно и пределы страниц (R-14), имена порогов (R-15).
    - 06 §6.3 — R-16, R-17.
    - 07 §2 — `schedule_channel_metrics`, уточнение `refresh_channel_metrics` (R-13, R-20); §4 — операции администратора для каналов и команда `admin grant` (R-1, R-2); §8 — новые параметры.
    - `docs/open-questions.md` — Q-34 (рабочий ответ реализован), Q-04 и Q-08 (поведение при отсутствии данных).
    - `.ai-factory/ARCHITECTURE.md` — запрос каталога на `pgx` как исключение из «только `sqlc`» (R-11), порт `PlacementSchedule`, `RequireAdmin`.
  - Журнал: не применимо.
  - Файлы: `docs/02-scenarios.md`, `docs/04-domain-model.md`, `docs/05-integrations.md`, `docs/06-finance-legal.md`, `docs/07-non-functional.md`, `docs/open-questions.md`, `.ai-factory/ARCHITECTURE.md`.
<!-- Commit checkpoint: tasks 13-14 -->

## Критерии готовности этапа

- `go build ./...`, `go vet ./...`, `go test -race ./...` проходят локально и в CI; `sqlc generate` и `go generate ./...` не дают изменений; `mockapi` собирается и проходит свой контрактный тест.
- `docker compose -f compose.dev.yml up` + события из `testdata/updates` + `maxim admin grant` проводят канал от `bot_added` до каталога рекламодателя с `maxfake`.
- Каждая причина отказа цен, интервалов и одобрения покрыта табличным тестом `domain`; правило R-16 — тестом каждого состояния × порог.
- Каталог с фильтрами и сортировкой на 2 000 каналах отвечает быстрее 1 с (07 §7).
- Канал вне условия R-7 не виден ни в списке, ни в карточке.
- Повтор `schedule_channel_metrics` и `refresh_channel_metrics` идемпотентен (07 §9).
- Текст сообщений канала, значение поиска, ИНН, почта и причины отказа отсутствуют в журнале.
- Решения R-1 … R-20 подтверждены и внесены в документы (задача 14).
