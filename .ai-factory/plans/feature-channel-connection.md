# План реализации: Подключение каналов

Branch: feature/channel-connection
Created: 2026-10-16

## Settings
- Testing: yes. Правила состояний канала, коды подключения и разбор событий MAX проверяются чистыми тестами без базы. Сценарии `channel/app` с `postgres` проверяются на настоящей PostgreSQL через `testcontainers-go` с `clock.Manual` и `maxfake`. Адаптер MAX проверяется через `httptest`-сервер с записанными ответами. Приём Webhook и операции кабинета проверяются через `httptest` поверх собранного сервера (07 §9).
- Logging: verbose. `log/slog` JSON, поля и обязательные события по 07 §5. Для этапа это входящие события и их проверка подлинности, внешние вызовы, переходы состояния канала, действия блогера. Код подключения, токены и секреты Webhook в журнал не пишутся (07 §5.4).
- Docs: yes. В конце обязательная точка `/aif-docs`: README бэкенда (новые переменные окружения, подписка Webhook, проверка подключения канала в разработке без бота).

## Roadmap Linkage
Milestone: "Подключение каналов"
Rationale: план целиком реализует этот этап: адаптер MAX Bot API для каналов, приём Webhook с проверкой подлинности, подключение канала кодом, проверку прав бота и реакцию на их отзыв.

## Research Context
Source: .ai-factory/RESEARCH.md (Active Summary), выдержка по этапу

- MAX: в продакшене события приходят только через Webhook (HTTPS, доверенный сертификат). Long Polling — только в разработке. `GET /chats` отключён: список каналов бота ведётся по событиям `bot_added` и `bot_removed`.
- Права бота — `GET /chats/{chatId}/members/me` → `permissions`. Обязательны `write` и `delete` (A-01). `view_stats` боты не получают.
- `bot_admin_permissions_changed` приходит только через Webhook.
- `DELETE /messages` может ответить `200` с `success=false` — проверять тело.

## Источники ТЗ

- Сценарии: 02 S-12 (подключение канала), S-25 (отзыв прав бота).
- Данные: 04 §1 (общие правила), §2 (`auth_code`), §3 (`channel`, `channel_category`), §9 (`channel.status`, `channel.rkn_status`).
- Интеграции: 05 §1 (общие правила), §2.1 (доступ), §2.2 (Webhook и реакция на события), §2.3 (методы), §2.4 (подключение канала), §2.8 (приоритеты ограничителя).
- Требования: 07 §1 (`api` не ходит во внешние системы), §2 (задания), §4 (Webhook: подлинность, тело до 1 МБ), §5 (журнал), §7 (приём Webhook до 200 мс), §8 (конфигурация).
- Реестр: A-01, A-14, A-41, D-11, D-25, Q-35, Q-38.
- Архитектура: `.ai-factory/ARCHITECTURE.md` — модуль `channel` (`domain`, `app`, `postgres`, `httpapi`, `jobs`), порты, правило «внешний вызов только из задания».
- Существующий код: `internal/platform/inbound` (`Recorder`, `Dispatcher`), `internal/integration/max` (`Client.do`, бакеты ограничителя, `maxfake`), `internal/account/domain/login_code.go` (генерация и HMAC кода), `internal/platform/httpx` (`RequireCabinet`, `BodyLimit`), `internal/platform/audit`, `internal/bootstrap/{modules.go,worker.go,config.go,unimplemented.go}`.

## Границы этапа

Входит:
- Модуль `channel`: таблица `channel` по 04 §3, справочник `channel_category` с начальным списком, коды подключения (`auth_code` с `purpose = channel_link`).
- Методы MAX для каналов: сведения о чате, права бота, удаление сообщения, выход из чата, подписка Webhook. Разбор событий Webhook в типы адаптера. Расширение `maxfake`.
- Приём `POST /webhooks/max/{secret}`: проверка подлинности (Q-35), запись `inbound_event`, ответ `200`.
- Обработчики событий из таблицы 05 §2.2, относящиеся к каналу: `bot_added`, `bot_removed`, `bot_admin_permissions_changed`, `message_created` (код подключения и диалог с ботом), `chat_title_changed`. Остальные виды — `ignored`.
- Состояния канала `unclaimed`, `pending_rights`, `moderation`, `suspended`, `disconnected` и возврат в `active` без повторной модерации (05 §2.4, S-25 шаги 1 и 3).
- Выход бота из канала, не подключённого кодом за 7 суток (S-12, A-41), и из чатов, которые не являются каналом.
- Операции кабинета блогера: `listBloggerChannels`, `getBloggerChannel`, `createChannelLinkCode`, `updateBloggerChannel`, а также `listChannelCategories`.

Не входит (следующие этапы):
- Модерация канала администратором (S-16), переход `moderation → active` и `rejected`: этапы «Каталог» и «API администратора». В этом этапе канал доходит до `moderation`. Возврат в `active` проверяется тестом на канале, у которого одобрение записано напрямую в базу.
- Цены, форматы, недоступность (`setBloggerChannelPrices`, `addChannelUnavailability`, `deleteChannelUnavailability`, `listPlacementFormats`) и метрики каналов: этап «Каталог». В ответе `BloggerChannel` поля `prices` и `unavailability` пока пустые массивы.
- Проверка номера в реестре РКН администратором (Q-34): этап «Каталог».
- Обработка размещений канала при отзыве прав (S-25 шаг 2: `P-06`, `P-14`): этап «Заказ и размещения». Этап даёт порт `AvailabilityListener` с пустой реализацией (R-11).
- Уведомления блогеру о смене состояния канала (07 §6): этап «Уведомления и отчёт по кампании». До него событие пишется в журнал.
- События о публикациях (`message_edited`, `message_removed`, `message_created` для «часов в топе»): этап «Автопостинг и контроль».
- Приём Webhook Рокет Ворк: этап «Выплаты блогерам через Рокет Ворк».

## Решения этапа, требующие подтверждения

Документы этих мест не определяют или противоречат друг другу. План реализует указанный вариант. После подтверждения задача 13 вносит его в документы; при отказе меняется только отмеченная задача.

| # | Вопрос | Решение в плане | Задача |
|---|---|---|---|
| R-1 | Формат кода: в 02 S-12 и A-41 — 8 символов, в `api/openapi.yaml` — 6 цифр (`^[0-9]{6}$`); описание операции говорит «отправить боту», а не в канал | 8 символов из алфавита без похожих знаков `23456789ABCDEFGHJKLMNPQRSTUVWXYZ` (40 бит). Сравнение без учёта регистра и пробелов по краям. Спецификация приводится к документам: шаблон `^[2-9A-HJ-NP-Z]{8}$`, текст «отправить код сообщением в канал» | 1, 3 |
| R-2 | Как найти код по тексту сообщения, если хранится только хеш (04 §2) | `code_hash` = HMAC-SHA256(`AUTH_CODE_PEPPER`, `"channel_link\0" + код`). Поиск по хешу среди живых кодов (`consumed_at is null`, `expires_at > now`) по частичному индексу. У организации один живой код: новый гасит прежний. Не больше 10 кодов на организацию в час (`CHANNEL_LINK_CODES_PER_HOUR`) | 2, 3, 6 |
| R-3 | Как проверить подлинность Webhook MAX (Q-35) | Секретная часть пути `MAX_WEBHOOK_PATH_SECRET` и заголовок `X-Max-Bot-Api-Secret`, равный `MAX_WEBHOOK_SECRET` (секрет передаётся MAX при подписке). Оба сравниваются за постоянное время. Имя заголовка исполнитель сверяет с документацией MAX и схемой `max-messenger/api-schema`; если MAX секрет не передаёт, остаётся только путь и запись об этом в 05 §2.9 | 9 |
| R-4 | Strict server отдаёт обработчику разобранное тело без заголовков, а ключ события — SHA-256 от сырого тела | Приём Webhook — обычный `http.Handler` из `channel/httpapi`, смонтированный в `bootstrap` перед strict server на путь операции `maxWebhook`. Операция остаётся в спецификации как договор. Тело читается с пределом 1 МБ (07 §4), ключ — `inbound.KeyFromBody` | 9, 11 |
| R-5 | Кто подписывает бота на Webhook | `worker` в `production` при запуске: `GET /subscriptions`; если адреса `MAX_WEBHOOK_URL` нет — `POST /subscriptions` с адресом, видами событий этапа и `MAX_WEBHOOK_SECRET`. Неудача — ERROR и оповещение `max_subscription`, запуск не прерывается. В разработке подписки нет: события отправляются на приём вручную из `internal/integration/max/testdata/updates/*.json` | 4, 11 |
| R-6 | События обрабатываются параллельно: `message_created` с кодом может прийти раньше, чем обработан `bot_added` | Все обработчики берут строку канала `FOR UPDATE` по `max_chat_id`. Сообщение с текстом в формате кода в неизвестном канале → `job.Retry(30 s)`, пока событию меньше 5 минут; затем `ignored` с WARN | 7, 8 |
| R-7 | Где вызывать MAX при подключении: 05 §1 разрешает чтение сведений о канале в запросе, 07 §1 — только из заданий | Только из заданий `worker`. Обработчик события меняет базу и ставит задания в той же транзакции: `refresh_channel_info` (`GET /chats/{id}` и права, `dedupe_key` `channel_info:{id}`), `delete_max_message`, `leave_max_chat`, `reply_bot_dialog`. Повтор задания не повторяет разбор события | 7, 8 |
| R-8 | Полные правила смены состояния канала (05 §2.4, S-25) | Чистая функция `domain.Resolve`. Бот удалён → `disconnected` (кроме `blocked`). Не связан с организацией → `unclaimed`. Связан, права или публичность не в порядке → `suspended`, если канал одобрен (`moderated_at` и не `rejected`), иначе `pending_rights`. Права в порядке → `active` из `suspended`, `moderation` из `pending_rights`. `rejected`, `blocked`, `moderation`, `active` в порядке не меняются. Причины: `not_public`, `missing_rights` | 3 |
| R-9 | Повторное добавление бота: «вернуть `active`, если канал был `active`» — состояние до отключения нигде не хранится | Поле `channel.status_before_disconnect text`. При `bot_added` по каналу в `disconnected`: было `active` или `suspended` → дальше по R-8 как одобренный, иначе → `moderation` (или `pending_rights` по правам). Организация сохраняется; `added_by_max_user_id` обновляется | 2, 3, 7 |
| R-10 | Код другой организации в уже подключённом канале (05 §2.4 шаг 2) | Владелец не меняется, код не гасится, сообщение с кодом удаляется, запись в журнал действий `channel.link_rejected` и оповещение `channel_link_conflict:{channel_id}`. Код своей организации в своём канале — гасится, повторных действий нет | 8 |
| R-11 | Как обработать размещения при отзыве прав, когда модуля размещений ещё нет | Порт `channel/domain.AvailabilityListener` (`ChannelUnavailable`, `ChannelRestored`) вызывается в той же транзакции, что и смена состояния. В `bootstrap` — пустая реализация с DEBUG. Модуль размещений подставит свою в этапе «Заказ и размещения» | 3, 7, 11 |
| R-12 | Что делать с неподключённым каналом через 7 суток (A-41): удалять строки запрещено (04 §1) | Периодическое задание `release_unclaimed_channels` раз в сутки: канал в `unclaimed` дольше `CHANNEL_UNCLAIMED_TTL` (7 суток) → `disconnected` с причиной `unclaimed_expired` и задание `leave_max_chat`. Повторное добавление бота создаёт `unclaimed` заново | 8 |
| R-13 | `updateBloggerChannel` требует `category_id`, а справочник категорий — этап «Каталог» | Таблица `channel_category` и начальный список (из тестового сервера `mockapi`) создаются в этом этапе, `listChannelCategories` реализуется. Правка списка администратором — этап «API администратора» | 2, 6, 10 |
| R-14 | Перечень недостающих прав в кабинете (S-12 шаг 4): в `BloggerChannel` его нет | Поле `missing_permissions: string[]` в `BloggerChannel`, считается в `domain` из `bot_permissions` | 1, 3, 10 |
| R-15 | Кто пишет строки `auth_code` с `purpose = channel_link`: таблица создана модулем `account` | Модуль `channel` владеет строками `channel_link`: свои запросы в `channel/postgres/queries`, `account` их не трогает. Очистка старых кодов остаётся у `account` (общая для таблицы) | 5 |
| R-16 | Новые виды заданий, которых нет в 07 §2 | `refresh_channel_info` (сразу, 5 попыток, приоритет `Reply`), `delete_max_message` (сразу, 5), `leave_max_chat` (сразу, 5), `reply_bot_dialog` (через 5 с, 3, без оповещения при исчерпании), `release_unclaimed_channels` (раз в сутки, 05:00 МСК, 1). Реализация: «не чаще ответа в 10 минут» заменено на «один ответ на пачку сообщений, пока ответ не отправлен» — `dedupe_key` живёт только до завершения задания | 7, 8 |

## Commit Plan
- **Commit 1** (после задач 1–2): `feat(channel): api contract and channel schema`
- **Commit 2** (после задач 3–5): `feat(channel): domain rules, MAX chat methods and channel storage`
- **Commit 3** (после задач 6–8): `feat(channel): link codes, MAX event handlers and channel jobs`
- **Commit 4** (после задач 9–11): `feat(channel): MAX webhook intake, blogger channel API and wiring`
- **Commit 5** (после задач 12–13): `test(channel): channel connection e2e and docs sync`

## Tasks

### Phase 1: Договор и данные

- [x] **Task 1: Спецификация операций каналов (R-1, R-14)**
  - `api/openapi.yaml`:
    - `ChannelLinkCode.code` — шаблон `^[2-9A-HJ-NP-Z]{8}$`. Описание `createChannelLinkCode`: бот добавляется администратором с правами на публикацию и удаление, затем код отправляется **сообщением в канал**; код действует 60 минут; новый код гасит прежний; `429` при превышении предела кодов в час.
    - `BloggerChannel`: поле `missing_permissions` (массив строк), описание `status_reason` со значениями `not_public`, `missing_rights`, `unclaimed_expired`.
    - `updateBloggerChannel`: `409`, если канал в `unclaimed`, `disconnected` или `blocked`.
    - `maxWebhook`: ответ `401` и упоминание заголовка секрета (R-3), предел тела 1 МБ.
  - `go generate ./internal/apigen`. Проверить `mockapi`: если он собирается из той же спецификации и выдаёт 6-значный код, привести генерацию кода к новому шаблону (иначе тестовый сервер отвечает не по схеме).
  - Журнал: не применимо.
  - Тесты: существующие тесты `bootstrap` и `mockapi` проходят; операции этапа всё ещё отвечают `501` до задачи 10.
  - Файлы: `api/openapi.yaml`, `internal/apigen/api.gen.go`, при необходимости `mockapi/internal/handlers/blogger.go`.

- [x] **Task 2: Миграция каналов (R-2, R-9, R-13)**
  - `migrations/00004_channels.sql` по 04 §1, §3:
    - `channel_category`: `id uuid`, `slug text unique`, `name text`, `position integer`, `is_active boolean`, `created_at`, `updated_at`. Начальный список — как в `mockapi` (идентификаторы UUID v7 записаны в миграции константами).
    - `channel`: все поля 04 §3, `lock_version`, `status check` по 04 §9, `rkn_status check`, `rating check 0..100 default 100`, уникальный `max_chat_id`, `status_before_disconnect` (R-9), `unclaimed_since timestamptz` для R-12. Ограничение `active` из 04 §3 без условия о цене (цены — этап «Каталог», условие добавит его миграция) — записать комментарием. Индексы: `(organization_id)`, `(status, unclaimed_since)`.
    - `auth_code`: частичный индекс `(code_hash) where purpose = 'channel_link' and consumed_at is null` (R-2).
  - Права роли — через `ALTER DEFAULT PRIVILEGES` из `00002`; проверить, что новые таблицы покрыты.
  - `sqlc.yaml`: запись для `internal/channel/postgres`.
  - Журнал: не применимо (INFO `migrations applied` пишет `migrate`).
  - Тесты: `up` → `down` → `up`; роль приложения читает и пишет `channel`, `channel_category`; второй канал с тем же `max_chat_id` не вставляется; `status` вне перечня отклоняется.
  - Файлы: `migrations/00004_channels.sql`, `sqlc.yaml`.
<!-- Commit checkpoint: tasks 1-2 -->

### Phase 2: Предметные правила, адаптер, хранение

- [x] **Task 3: `channel/domain` — канал, состояния, коды (R-1, R-2, R-8, R-9, R-11, R-14)** (depends on 2)
  - `internal/channel/domain`:
    - `Channel` (поля 04 §3), `Status` и `RknStatus` — константы с именами из 04 §9, `Permission` (`write`, `delete`, …), `RequiredPermissions` = `write`, `delete` (A-01).
    - `Resolve(ch, now) StatusChange` — чистая функция по R-8 и R-9. Методы-события: `BotAdded(addedBy, now)`, `BotRemoved(now)`, `PermissionsUpdated(isAdmin, perms, isPublic, info, now)`, `LinkTo(orgID, now) error` (ошибки `ErrAlreadyLinked`, `ErrLinkedToOther`), `ReleaseUnclaimed(now)`, `UpdateProfile(description, categoryID, rknNumber, rknThreshold)` (пересчёт `rkn_status`: `not_required` / `pending`; из `pending_rights` и `rejected` → `moderation`, если права в порядке; `409` для `unclaimed`, `disconnected`, `blocked`). Каждое изменение увеличивает `LockVersion` и возвращает `StatusChange{From, To, Reason}`.
    - `MissingPermissions()` (R-14), `Approved()` (`moderated_at` задан и статус не `rejected`).
    - `LinkCode`: `GenerateLinkCode()` (8 символов, `crypto/rand`, R-1), `NormalizeLinkCode(text) (code string, ok bool)` (обрезка, верхний регистр, проверка алфавита и длины), `HashLinkCode(pepper, code)` (R-2).
    - Порты: `Channels` (репозиторий), `LinkCodes`, `Categories`, `ChatReader` (`GetChat`, `GetMembership`), `ChatManager` (`DeleteMessage`, `LeaveChat`, `SendUserMessage`), `AvailabilityListener` (R-11).
  - Журнал: не применимо (`domain` не пишет журнал).
  - Тесты: табличные — каждое состояние × каждое событие по R-8/R-9, включая `blocked` и `rejected`; возврат `suspended → active` без модерации; повторное добавление после `disconnected` для было-`active` и было-`moderation`; `rkn_status` на пороге 9 999 / 10 000; `NormalizeLinkCode` (регистр, пробелы, `0`/`O`, длина 7 и 9); хеши разных кодов различаются, одинаковых — совпадают.
  - Файлы: `internal/channel/domain/{channel.go,status.go,link_code.go,ports.go,*_test.go}`.

- [x] **Task 4: Методы MAX для каналов и разбор событий (R-5)** (depends on 2)
  - `internal/integration/max`:
    - `GetChat(ctx, chatID) (Chat, error)` — `GET /chats/{chatId}`: `title`, `description`, `icon.url`, `link`, `is_public`, `participants_count`, `type`.
    - `GetMembership(ctx, chatID) (Membership, error)` — `GET /chats/{chatId}/members/me`: `is_admin`, `permissions`.
    - `DeleteMessage(ctx, messageID) error` — `DELETE /messages?message_id=`, `success=false` → ошибка через `ValidateBody` (05 §1).
    - `LeaveChat(ctx, chatID) error` — `DELETE /chats/{chatId}/members/me`.
    - `ListSubscriptions`, `Subscribe(ctx, url, updateTypes, secret)` (R-5).
    - Чтения — с приоритетом из `context`; по умолчанию `PriorityReply` (подключение ждёт блогер). Запросы к каналу берут токен `max:chat:{chat_id}` только для записи (удаление сообщения).
    - `ParseUpdate(body) (Update, error)` — тип события (`update_type`), `chat_id`, признак канала, `user` добавившего, `message` (идентификатор, автор, текст, тип чата `dialog`/`channel`), новое название. Неизвестный вид — не ошибка.
  - `maxfake`: каналы и права задаются тестом (`SetChat`, `SetMembership`), запоминает удалённые сообщения и покинутые чаты, `FailWith(err)`.
  - Записанные ответы и тела событий: `testdata/chat.json`, `membership.json`, `delete_ok.json`, `delete_false.json`, `testdata/updates/{bot_added_channel,bot_added_dialog,bot_removed,bot_admin_permissions_changed,message_created_channel,message_created_dialog,chat_title_changed}.json`. Структуру сверить со схемой `max-messenger/api-schema`.
  - Журнал: DEBUG `max chat read` (`chat_id`), INFO `max message deleted` (`chat_id`, `message_id`), INFO `max chat left` (`chat_id`), INFO `max webhook subscribed` (`url` без секретной части пути). Текст сообщений не пишется.
  - Тесты (`httptest`): каждый метод разбирает записанный ответ; `success=false` при удалении → ошибка; `404` при `GetChat` → `Permanent`; `ParseUpdate` на каждом файле `updates/`; неизвестный `update_type` разбирается без ошибки.
  - Файлы: `internal/integration/max/{chats.go,messages.go,subscriptions.go,updates.go,*_test.go,testdata/**}`, `internal/integration/max/maxfake/fake.go`.

- [x] **Task 5: `channel/postgres` — хранение (R-15)** (depends on 3)
  - Запросы `sqlc`: канал по `id` и организации; по `max_chat_id` с `FOR UPDATE`; вставка; сохранение с проверкой `lock_version`; список каналов организации; `unclaimed` старше срока. Коды: вставка, погашение живых кодов организации, поиск живого по хешу с `FOR UPDATE`, погашение, счёт кодов организации за час. Категории: список активных, существование по `id`.
  - Все кабинетные выборки принимают `organizationID` явно (07 §4).
  - Журнал: DEBUG `channel saved` (`channel_id`, `status`, `lock_version`); конфликт `lock_version` → `apperr` без журнала (пишет `app`).
  - Тесты (PostgreSQL): сохранение с устаревшей `lock_version` отклоняется; список другой организации пуст; погашенный и просроченный код не находится; два кода с одинаковым хешем не мешают поиску живого.
  - Файлы: `internal/channel/postgres/{store.go,channels.go,link_codes.go,categories.go,store_test.go}`, `internal/channel/postgres/queries/*.sql`.
<!-- Commit checkpoint: tasks 3-5 -->

### Phase 3: Сценарии

- [x] **Task 6: Сценарии кабинета блогера (R-2, R-13)** (depends on 5)
  - `internal/channel/app`:
    - `CreateLinkCode(ctx, orgID, userID)` — предел кодов в час (R-2, `429` `rate_limited`), погашение прежнего живого кода, новый код с `CHANNEL_LINK_CODE_TTL` (60 мин), запись `channel.link_code_created` в журнал действий без кода. Возвращает код, срок, имя бота `MAX_BOT_NAME` и текст инструкции (бот администратором с правами публикации и удаления, затем код сообщением в канал).
    - `List`, `Get` (организация обязательна), `Update` (PATCH: проверка категории, длина описания, `UpdateProfile` из `domain`, `AvailabilityListener` не вызывается — доступность не меняется, журнал действий `channel.updated` с `audit.Diff`).
    - `Categories`.
  - Журнал: INFO `channel link code issued` (`organization_id`, `expires_at`), WARN `channel link code rate limited` (`organization_id`), INFO `channel updated` (`channel_id`, `status`, `changed`), WARN `channel update rejected` (`channel_id`, `status`).
  - Тесты (PostgreSQL, `clock.Manual`): второй код гасит первый; 11-й код в час → `429`; код не попадает ни в журнал, ни в `audit_log`; PATCH из `pending_rights` с правами в порядке → `moderation`; номер РКН при 12 000 подписчиков → `rkn_status = pending`; несуществующая категория → `422`; чужой канал → `404`.
  - Файлы: `internal/channel/app/{service.go,link_code.go,cabinet.go,*_test.go}`.

- [x] **Task 7: События бота и права (R-6, R-7, R-8, R-9, R-11, R-16)** (depends on 4, 5)
  - `internal/channel/app` + `internal/channel/jobs`, обработчики `inbound.Dispatcher` для источника `max`:
    - `bot_added`, канал: создать канал `unclaimed` (`unclaimed_since`) или восстановить по R-9; записать `added_by_max_user_id`; поставить `refresh_channel_info`. Не канал: поставить `leave_max_chat`.
    - `bot_removed`: известный канал → `BotRemoved`, `status_before_disconnect`, `AvailabilityListener.ChannelUnavailable`. Неизвестный → `ignored`.
    - `bot_admin_permissions_changed`: поставить `refresh_channel_info` (права берутся из `GET /chats/{id}/members/me`, а не из тела события, — одно место расчёта).
    - Задание `refresh_channel_info`: вне транзакции — `GetChat` и `GetMembership`; затем в транзакции `FOR UPDATE` → `PermissionsUpdated`, `Resolve`, вызов `AvailabilityListener` при переходе в `suspended`/`disconnected` или обратно в `active`, журнал действий `channel.status_changed` с актором `system`.
    - Все обработчики идемпотентны: повтор события или задания не меняет канал второй раз.
  - Журнал: INFO `channel bot added` (`channel_id`, `max_chat_id`, `status`), INFO `channel bot removed`, INFO `channel status changed` (`channel_id`, `from`, `to`, `reason`), INFO `channel rights refreshed` (`channel_id`, `bot_is_admin`, `missing`), WARN `channel rights lost` (`channel_id`, `missing`), INFO `max chat is not a channel, leaving` (`max_chat_id`).
  - Тесты (PostgreSQL, `maxfake`, `clock.Manual`): `bot_added` дважды → один канал и одно задание `refresh_channel_info`; права без `delete` у подключённого неодобренного канала → `pending_rights` с `missing_permissions = [delete]`; у одобренного → `suspended`, `AvailabilityListener` вызван; права вернулись → `active` без модерации; `bot_removed` → `disconnected`, повторное добавление → прежнее состояние по R-9; `bot_added` в группе → `leave_max_chat`; закрытый канал → `pending_rights` с `not_public`.
  - Файлы: `internal/channel/app/{events.go,refresh.go,*_test.go}`, `internal/channel/jobs/jobs.go`.

- [x] **Task 8: Код подключения, диалог, название, неподключённые каналы (R-6, R-10, R-12, R-16)** (depends on 6, 7)
  - `message_created`:
    - Диалог с ботом → задание `reply_bot_dialog`: `SendUserMessage` с инструкцией (как зарегистрироваться и подключить канал, D-25). Не чаще одного ответа пользователю в 10 минут (`dedupe_key` `bot_dialog:{user_id}` + время запуска).
    - Канал, автор — наш бот → `processed` без действий. Текст не в формате кода → `processed` без действий (часы в топе — этап «Автопостинг»).
    - Текст в формате кода: канал неизвестен → R-6. Живой код не найден → `processed`, WARN, канал остаётся `unclaimed`. Найден: канал `unclaimed` → `LinkTo`, погасить код, `Resolve` (→ `moderation` или `pending_rights`), `delete_max_message`, журнал действий `channel.linked` (актор `system`, организация из кода). Канал своей организации → погасить код, удалить сообщение. Чужой организации → R-10.
  - `chat_title_changed` → новое `title` у известного канала.
  - Задание `delete_max_message`: без права `delete` удалить нельзя → `job.Permanent` и WARN, канал уже `pending_rights`.
  - Периодическое `release_unclaimed_channels` (R-12): раз в сутки, `ReleaseUnclaimed` и `leave_max_chat` по каждому каналу в отдельной транзакции.
  - Журнал: INFO `channel linked` (`channel_id`, `organization_id`, `status`), WARN `channel link code not found` (`max_chat_id`), WARN `channel link conflict` (`channel_id`, `organization_id` владельца), INFO `bot dialog replied` (`max_user_id`), INFO `channel title updated` (`channel_id`), INFO `unclaimed channels released` (`count`). Код и текст сообщений не пишутся.
  - Тесты (PostgreSQL, `maxfake`, `clock.Manual`): верный код → канал связан, код погашен, сообщение удалено; тот же код второй раз → ничего; код через 61 мин → не связан; код в нижнем регистре с пробелами → связан; код другой организации в подключённом канале → владелец прежний, `audit_log` и оповещение; `message_created` раньше `bot_added` → повтор, после `bot_added` канал связан; `unclaimed` 7 суток + 1 мин → `disconnected` и выход из чата, 6 суток → без изменений; два сообщения в диалог за 5 минут → один ответ.
  - Файлы: `internal/channel/app/{link.go,dialog.go,release.go,*_test.go}`, `internal/channel/jobs/jobs.go`.
<!-- Commit checkpoint: tasks 6-8 -->

### Phase 4: HTTP и сборка

- [x] **Task 9: Приём Webhook MAX (R-3, R-4)** (depends on 4)
  - `internal/channel/httpapi/webhook.go`: `http.Handler` для `POST {BaseURL}/webhooks/max/{secret}`:
    1. Секрет пути и заголовок `X-Max-Bot-Api-Secret` сравниваются через `subtle.ConstantTimeCompare`. Не прошло → `401`, WARN `inbound event rejected` (`source`, `reason`: `path`/`header`, IP), счётчик; при 5 и более отказах за 10 минут — `alert.Raise("webhook_auth_failed:max")` (07 §5.3 «при повторении»).
    2. Тело через `http.MaxBytesReader` до 1 МБ (`413` сверх), JSON проверяется на корректность, `update_type` берётся из тела.
    3. `inbound.Recorder.Record` с ключом `KeyFromBody(body)`. Дубликат → `200`.
    4. Ответ `200` без тела. Никаких вызовов MAX и разбора событий в запросе (07 §7: до 200 мс).
  - Метрика `max_webhook_requests_total{outcome}` (`accepted`, `duplicate`, `unauthorized`, `invalid`).
  - Журнал: INFO `inbound event accepted` уже пишет `Recorder`; WARN `inbound event rejected`; секрет пути не попадает ни в журнал запросов, ни в метрики (маскировать путь в `httpx` логировании запросов: `/webhooks/max/***`).
  - Тесты (`httptest`): верные секреты → `200`, строка `inbound_event` и задание; неверный путь или заголовок → `401` без записи; тело 1 МБ + 1 байт → `413`; одно и то же тело дважды → одна строка; секрет не встречается в записанном журнале.
  - Файлы: `internal/channel/httpapi/{webhook.go,webhook_test.go}`, `internal/platform/httpx/` (маскирование пути).

- [x] **Task 10: Операции кабинета блогера** (depends on 6)
  - `internal/channel/httpapi/handlers.go`: `ListBloggerChannels`, `GetBloggerChannel`, `CreateChannelLinkCode`, `UpdateBloggerChannel`, `ListChannelCategories`. Кабинетные — через `httpx.RequireCabinet(ctx, "blogger")`, организация из сессии. Преобразование `domain.Channel` → `apigen.BloggerChannel` с `missing_permissions` и пустыми `prices`/`unavailability`.
  - Ошибки — через `apperr` (`404` чужой канал, `409` состояние, `422` проверка, `429` предел кодов).
  - Журнал: WARN `403` пишет `httpx`; остальное — в `app`.
  - Тесты (`httptest` поверх собранного сервера): без кабинета блогера → `403`; код соответствует шаблону спецификации; ответ проходит проверку схемой (`nethttp-middleware` на ответ в тесте или `openapi3filter`); чужой канал → `404`; CSRF обязателен для POST и PATCH.
  - Файлы: `internal/channel/httpapi/{handlers.go,convert.go,handlers_test.go}`.

- [x] **Task 11: Конфигурация и сборка (R-4, R-5, R-11)** (depends on 7, 8, 9, 10)
  - `internal/bootstrap/config.go`: `MAX_WEBHOOK_PATH_SECRET`, `MAX_WEBHOOK_SECRET`, `MAX_WEBHOOK_URL`, `MAX_BOT_NAME`, `CHANNEL_LINK_CODE_TTL` (60m), `CHANNEL_LINK_CODES_PER_HOUR` (10), `CHANNEL_UNCLAIMED_TTL` (168h), `RKN_SUBSCRIBERS_THRESHOLD` (10000). Проверка: в `production` для `api` обязательны оба секрета Webhook (не короче 32 символов), для `worker` — `MAX_WEBHOOK_URL` с `https://`. Секреты маскируются в журнале конфигурации.
  - `modules.go`: модуль `channel` (store, service, handlers), `apiImpl` встраивает `channelhttp.Handlers`; Webhook монтируется перед strict server (R-4); пустой `AvailabilityListener` (R-11). В `api` нужен `inbound.Recorder`.
  - `worker.go`: обработчики событий в `Dispatcher`, задания R-16, периодическое `release_unclaimed_channels`, подписка Webhook при запуске в `production` (R-5). `maxfake` в `APP_ENV=test` реализует новые порты.
  - `unimplemented.go`: убрать заглушки реализованных операций.
  - `.env.example`, `compose.dev.yml`: новые переменные с безопасными значениями; в README-заметке для `/aif-docs` — команда `curl` для отправки события из `testdata/updates`.
  - Журнал: INFO `worker started` с новыми видами заданий; INFO `max webhook subscription ok` / ERROR `max webhook subscription failed`.
  - Тесты: проверки конфигурации (короткий секрет, `http://` адрес в `production`); `wiring_test.go` — все виды заданий этапа и обработчики событий `bot_added`, `bot_removed`, `bot_admin_permissions_changed`, `message_created`, `chat_title_changed` зарегистрированы; подписка: адреса нет → `POST /subscriptions`, есть → без запроса.
  - Файлы: `internal/bootstrap/{config.go,config_test.go,modules.go,worker.go,api.go,unimplemented.go,wiring_test.go}`, `.env.example`, `compose.dev.yml`.
<!-- Commit checkpoint: tasks 9-11 -->

### Phase 5: Проверка и документы

- [x] **Task 12: Сквозные тесты подключения канала** (depends on 11)
  - `internal/bootstrap/channel_e2e_test.go` на собранных `api` и `worker` (PostgreSQL, `clock.Manual`, `maxfake`, `mailfake`):
    - S-12: блогер входит, создаёт организацию с кабинетом блогера, получает код → Webhook `bot_added` → `worker` → канал `unclaimed`, права прочитаны → Webhook `message_created` с кодом → канал в списке блогера в `moderation`, сообщение с кодом удалено, код второй раз не срабатывает → PATCH с категорией и номером РКН.
    - Нет права `delete` → `pending_rights` с `missing_permissions`; права выданы (`bot_admin_permissions_changed`) → `moderation`.
    - S-25: одобрение записано в базу → `bot_admin_permissions_changed` без `write` → `suspended`; права вернулись → `active`; `bot_removed` → `disconnected`; повторное `bot_added` → `active` по R-9.
    - Неверный секрет Webhook → `401`, событие не записано.
    - Неподключённый канал через 7 суток → `disconnected`, бот вышел из чата.
  - Проверка журнала сборщиком записей из `e2e_test.go`: обязательные события этапа (07 §5.3: входящее событие принято и обработано, отказ проверки подлинности, внешняя операция выполнена) присутствуют; код подключения, секреты Webhook и токен бота не встречаются.
  - Файлы: `internal/bootstrap/channel_e2e_test.go`.

- [x] **Task 13: Синхронизация документов с решениями этапа** (depends on 11)
  - После подтверждения R-1 … R-16 внести их в документы, чтобы правы были документы:
    - 02 S-12 — формат кода (R-1), порядок событий (R-6), конфликт владельца (R-10); S-25 — возврат по R-8/R-9.
    - 04 §2 — хеш и единственный живой код `channel_link` (R-2); §3 — `status_before_disconnect`, `unclaimed_since` (R-9, R-12), ограничение `active` по этапам.
    - 05 §2.2 — проверка подлинности (R-3), приём сырым обработчиком (R-4); §2.3 — `DELETE /chats/{id}/members/me`, `/subscriptions`; §2.4 — правила R-8, R-9, R-12, вызовы MAX только из заданий (R-7, правка исключения в §1); §2.9 — ответ на Q-35, если подтверждён.
    - 07 §2 — виды заданий R-16; §8 — новые параметры.
    - `docs/open-questions.md` — A-41 (подробности кода), Q-35 (рабочий ответ).
    - `.ai-factory/ARCHITECTURE.md` — модуль `channel` реализован, Webhook как `http.Handler` модуля (R-4), порт `AvailabilityListener`.
  - Журнал: не применимо.
  - Файлы: `docs/02-scenarios.md`, `docs/04-domain-model.md`, `docs/05-integrations.md`, `docs/07-non-functional.md`, `docs/open-questions.md`, `.ai-factory/ARCHITECTURE.md`.
<!-- Commit checkpoint: tasks 12-13 -->

## Критерии готовности этапа

- `go build ./...`, `go vet ./...`, `go test -race ./...` проходят локально и в CI; `sqlc generate` и `go generate ./...` не дают изменений.
- `docker compose -f compose.dev.yml up` + события из `testdata/updates` через `curl` проводят канал от `bot_added` до `moderation` с `maxfake`.
- Каждая строка таблицы 05 §2.2 для событий этапа покрыта тестом; повтор каждого события и задания идемпотентен (07 §9).
- Каждое состояние канала × каждое событие покрыто табличным тестом `domain.Resolve`.
- Webhook без верных секретов не создаёт `inbound_event`; приём не обращается к MAX.
- Код подключения, секреты Webhook и токен бота отсутствуют в журнале, `audit_log` и `outbound_request`.
- Решения R-1 … R-16 подтверждены и внесены в документы (задача 13).
