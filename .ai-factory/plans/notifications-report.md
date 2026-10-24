# План реализации: Уведомления и отчёт по кампании

Branch: feature/notifications-report (worktree `D:/work/maxim-notifications`, ветка создана оркестратором от master)
Created: 2026-10-24

## Settings
- Testing: yes. Каталог шаблонов (у каждого кода есть адресаты, каналы и тексты; каждый шаблон собирается на образце данных; длина сообщения MAX ≤ 4000), правила адресатов и каналов (блогер — бот MAX и почта, рекламодатель — почта, администратор — только кабинет; кабинет — всем), состояние уведомления (`pending → sent | failed`, `sent|failed → read` только для кабинета) — табличные тесты `notification/domain` и `notification/render`. Сценарии `notification/app` с `postgres` — на настоящей PostgreSQL (`testcontainers-go`, `clock.Manual`, `maxfake`, `mailfake`). Отчёт по заказу — тест `placement/app` на PostgreSQL. Сквозной сценарий — на собранных `api` и `worker`.
- Logging: verbose. `log/slog` JSON по 07 §5: INFO `notification created` (`template`, `channel`, `notification_id`, `user_id`, `organization_id`, `object_type`, `object_id`), INFO `notification sent` (`channel`, `elapsed_ms`), WARN `notification delivery failed` (повтор), ERROR `notification failed` (исчерпаны попытки или отказ без повтора), DEBUG `notification skipped` (нет адресата: нет почты, нет `added_by_max_user_id`), INFO `notifications marked read` (`count`). Адрес почты — только маскированным (`logx.MaskEmail`), текст уведомления в журнал не пишется (07 §5.4).
- Docs: yes. В конце обязательная точка `/aif-docs`: README (переменные `NOTIFICATION_*`, `CABINET_BASE_URL`, как посмотреть письма в Mailpit и сообщения в `maxfake`), документы 04 §8, 07 §6, 02 S-09.

## Roadmap Linkage
Milestone: "Уведомления и отчёт по кампании"
Rationale: план реализует этап целиком: задание `send_notification` (07 §2), доставку ботом MAX и почтой и уведомления в кабинете (07 §6, A-19, A-41), подключение всех событий master (размещения, заказ и пост, оплата и закрывающие документы, каналы) вместо `NopNotifier`, отчёт по заказу с просмотрами и ссылками по каждому размещению и UTM-метками (S-09, A-21). Выплаты и споры получают коды шаблонов; подключение — после слияния их этапов (раздел «Подключение выплат и споров»).

## Источники ТЗ

- Сценарии: 02 S-09 (отчёт, CSV, CPV по D-28), S-14, S-16, S-21, S-22, S-23, S-30, S-25; S-15, S-18 — только коды.
- Размещение: 03 §4 (столбец «Действия»: кого уведомлять в каждом переходе, «и администратора» у P-09 и P-14), §5 (T-01).
- Данные: 04 §8 `notification`, 04 §9 `notification.channel`, 04 §10 (почта — персональные данные), 04 §5 `publication` (`url`, `views`).
- Интеграции: 05 §2.2–2.3 (личное сообщение бота по `added_by_max_user_id`), §2.5 (UTM-метки, A-21), §2.7 (`stat.views`, Q-08), §2.8 (приоритет «ответы и уведомления»).
- Требования: 07 §2 (`send_notification`, 5 попыток), §5.3–§5.5, §6 (каналы и обязательные уведомления).
- Реестр: A-19, A-21, A-41, D-28, Q-08, Q-38.
- Контракт: `api/openapi.yaml` — тег `notifications` (`listNotifications`, `markNotificationRead`, `markAllNotificationsRead`), `getOrderReport` и `OrderReport*`.
- Фронтенд: `docs/frontend-api-gaps.md`, раздел «Уведомления (MAXI-50)»: нет `order_id` в уведомлении; нет `read-all` (в контракте уже есть, реализуем); в `mockapi` нет уведомлений администратора и шаблонов баланса, одобрения и спора. Раздел MAXI-46: нет выгрузки отчёта в CSV.
- Экраны: `docs/wireframe/index.html` — `common/notifications`, `adv/report`.
- Существующий код: `internal/placement/domain/{ports.go (Notice, Notifier),placement.go (коды Notice*),actions.go,publish.go}`, `internal/placement/app/{transition.go (record, notice),versions.go,creative.go,remove.go,orders.go}`, `internal/bootstrap/bridge/placement.go (NopNotifier)`, `internal/billing/{domain/ports.go,app/service.go (NopNotifier),app/statements.go,app/closing.go}`, `internal/channel/app/{service.go (save),moderation.go}`, `internal/integration/max/messages.go (SendUserMessage)`, `internal/integration/mail`, `templates/`, `internal/platform/{job,cleanup,ratelimit,logx}`, `mockapi/internal/{handlers/auth.go,handlers/orders.go,store/store.go}`.

## Границы этапа

Входит:
- Модуль `internal/notification` (`domain`, `app`, `postgres`, `httpapi`, `jobs`, `render`): каталог шаблонов, адресаты, записи `notification`, задание `send_notification`, кабинет.
- Тексты в репозитории: `templates/notification/*.tmpl`, общий макет письма.
- Доставка: бот MAX (`SendUserMessage` с приоритетом «ответы и уведомления»), почта (`integration/mail`), кабинет.
- Подключение событий master: переходы размещений (включая напоминание о T-01 за 3 часа), одобрение и отклонение поста, ЕРИР, снятие с опозданием, зачисление оплаты, закрывающие документы, смена состояния канала. Уведомления администратора в кабинете о P-09 и P-14.
- Отчёт по заказу `getOrderReport` и новая выгрузка `getOrderReportCsv`; CPV по D-28.
- Правки контракта и `mockapi` (поле `order_id` уведомления, CSV, CPV, шаблоны администратора, баланса, одобрения, спора).
- Очистка старых уведомлений, метрики, конфигурация.

Не входит:
- Выплаты, споры и их события: модулей `payout` и споров ещё нет в master. Здесь — только коды шаблонов и тексты (R-11).
- Сокращатель ссылок и подсчёт переходов: A-21 их исключает. Отчёт показывает ссылку на пост и UTM-метки, переходы считает аналитика рекламодателя.
- Настройки уведомлений пользователя (отписка, выбор каналов): в документах нет.
- Оповещения администратору (почта, чат дежурных) остаются в `platform/alert`.

## Решения этапа, требующие подтверждения

Документы этих мест не определяют. План реализует указанный вариант; задача 13 вносит решения в документы.

| # | Вопрос | Решение в плане | Задача |
|---|---|---|---|
| R-1 | Адресаты уведомления | `Notice` адресован организации и роли. Адресаты — все члены организации (`organization_member`, по A-13 у пользователя одна организация). Администратор — пользователи с `admin_role`; уведомление администратора без организации (`organization_id` пуст) | 2, 5, 6 |
| R-2 | Каналы по ролям (07 §6, A-19) | Кабинет — всегда, каждому адресату. Блогер — дополнительно бот MAX и почта; рекламодатель — почта; администратор — только кабинет (почту и чат дежурных ему шлёт `platform/alert`, дублировать нельзя). Почта блогеру идёт всегда, а не только после сбоя бота: бот может не писать в личку без начатого диалога (Q-38), а узнать это можно только после отказа; «остаётся почта» (07 §6) выполняется без отдельной логики | 2 |
| R-3 | Адресат бота MAX | Одна запись `max_bot` на уведомление: `channel.added_by_max_user_id` канала из `Data.channel_id` (A-41). `user_id` записи — владелец организации. Нет канала в данных или нет `added_by_max_user_id` — запись не создаётся, DEBUG `notification skipped` | 2, 5, 6 |
| R-4 | Хранение | `notification`: поля 04 §8 и `organization_id`, `attempts`, `last_error`, `updated_at`. Одна строка на адресата и канал. Строка кабинета создаётся в `sent` (доставлена в момент создания), `read` — только у кабинета. `max_bot`/`email` — `pending`, задание `send_notification` (`dedupe_key` `notification:{id}`, 5 попыток) в той же транзакции (07 §6, I-10) | 1, 4, 5 |
| R-5 | Тексты и данные | Шаблон — код из каталога `notification/domain`; тексты `templates/notification/{код}.tmpl` с блоками `title`, `text` (кабинет и бот, Markdown MAX) и необязательным `email_text`. Письмо — общий макет `templates/email/notification.*.tmpl`. В `payload` — только данные для текста (номер заказа, название канала, время, ссылка, просмотры, суммы); имена организаций, ИНН и почта в `payload` не кладутся. Текст собирается при чтении кабинета и при отправке, поэтому правка шаблона меняет и старые уведомления | 2, 3 |
| R-6 | Обогащение данных | Модуль уведомлений при создании добавляет в `payload` название канала (`channel_title`) через порт `Channels` и ссылку в кабинет (`link`, относительный путь: заказ рекламодателя, размещение блогера, канал). Модуль размещений сам добавляет `publish_at`, `remove_at`, `publication_url`, `views` | 2, 5, 8 |
| R-7 | Отказы доставки | MAX: `403`/`404` при личном сообщении (бот не может писать пользователю, Q-38) — без повторов, `failed`. Почта: неверный адрес — без повторов. Остальное — повтор по 07 §2; после 5 попыток — `failed` через `OnExhausted`, ERROR, оповещение администратору не поднимается (`NoFailureAlert`): сбой уведомления не требует человека и не влияет на событие (07 §6) | 5, 6 |
| R-8 | Напоминание T-01 | Задание `placement_reminder` (`dedupe_key` `reminder:{placement_id}:T-01`, 3 попытки) на `accept_deadline_at − 3 ч` ставится вместе с P-02, если до срока больше 3 ч; отменяется P-03, P-04, P-05, P-06. Обработчик проверяет `awaiting_blogger` и тот же срок, иначе устарело. Шаблон `placement.accept_reminder` | 8 |
| R-9 | Уведомление администратора | P-09 и P-14 по 03 §4 уведомляют администратора: в кабинете (`placement.failed`, `placement.disputed` с ролью `admin`). Остальные события администратора — оповещения `platform/alert` | 2, 8 |
| R-10 | Повтор события | Транзакционные события повторно уведомление не создают (переход атомарен). Для заданий-напоминаний — ключ `Notice.Key`: уникальный индекс `(user_id, channel, dedupe_key)` и `ON CONFLICT DO NOTHING` | 1, 4, 8 |
| R-11 | Выплаты и споры | Коды есть в каталоге и с текстами: `placement.dispute_resolved` (P-15, обоим), `payout.sent` («Принято, выплата отправлена», блогеру), `payout.succeeded` (P-16, блогеру, со ссылкой на чек `receipt_url`), `payout.failed` (блогеру), `rocketwork.active` и `rocketwork.blocked` (статус исполнителя, блогеру). Коды совпадают с R-15 плана `rocketwork-payouts.md`. Вызывающих в master нет; подключение — раздел «Подключение выплат и споров» | 2, 3 |
| R-12 | Кабинет: видимость | `listNotifications` — уведомления пользователя сессии (по `user_id`) в канале `cabinet`, новые сверху, курсор `(created_at, id)`; `unread=true` — без `read_at`. `total` — число строк по фильтру. `markNotificationRead` — чужое или не кабинетное — `404`; повтор — `204`. `markAllNotificationsRead` — все непрочитанные пользователя, повтор безопасен | 4, 7 |
| R-13 | `order_id` в уведомлении (MAXI-50) | В схему `Notification` добавляется необязательный `order_id` (из `payload.order_id`), чтобы кабинет вёл на заказ. Плюс необязательный `link` — относительный путь в кабинете (R-6) | 7 |
| R-14 | Отчёт по заказу (S-09) | `getOrderReport` по правилам `mockapi`: строка — канал, состояние, `published_at`, `removed_at`, `views` (последнее считанное значение, Q-08), `publication_url`, `utm_campaign = order_{номер}` (A-21); суммы — `spent` (`accepted`, `paid`: цена минус `refund_amount`; `refunded`: 0), `refunded` (финальные без распределения, удержание снято, — вся цена; `refund_amount` после распределения), `held` (активное удержание). Новое: `cpv_amount` строки и итога — потрачено, делённое на просмотры, округление вниз в копейках; нет просмотров — нет CPV (D-28). `links` отчёта — адреса ссылок текущей версии поста с UTM (как ушли в канал) | 10 |
| R-15 | Выгрузка CSV | `GET /orders/{id}/report.csv` (`getOrderReportCsv`, `text/csv; charset=utf-8`, BOM для Excel, разделитель `;`, суммы в рублях с запятой, время по Москве). Те же строки и итог, что в JSON | 10 |
| R-16 | Очистка | Задача `cleanup` удаляет уведомления старше `NOTIFICATION_RETENTION` (365 суток) в любом конечном состоянии; `pending` не трогает | 4, 11 |
| R-17 | Ссылки в письмах и боте | Абсолютная ссылка — `CABINET_BASE_URL` + `link` (по умолчанию `http://localhost:5173`, в `production` обязателен `https://`) | 3, 11 |
| R-18 | Сокращатель ссылок | Не делается (A-21). Поля переходов в отчёте нет | — |

## Допущения о внешних данных

Внешних образцов нет; форматы придуманы правдоподобно, в коде помечены `// ПРЕДПОЛОЖЕНИЕ: …`.

| # | Что | Допущение | Где |
|---|---|---|---|
| E-1 | Личное сообщение бота пользователю, который не начинал диалог (Q-38) | MAX отвечает `403` с кодом `chat.denied` или `404` `user.not.found`; это отказ без повторов | `notification/app` (классификация через `max.ExternalError`/`job.Permanent`), `bridge.NotificationMessenger` |
| E-2 | Разметка личного сообщения | `format: markdown` (как у сообщений в чат дежурных) понимает `**жирный**` и `[текст](url)`; спецсимволы `*_[]()` в данных экранируются обратной косой | `notification/render` |
| E-3 | Длина личного сообщения | Тот же предел 4000 символов, что у сообщения канала (`max.MaxTextLength`); длиннее — текст обрезается по границе строки с «…» и ссылкой в конце | `notification/render` |
| E-4 | Просмотры (Q-08) | `stat.views` из `GET /messages/{id}` — число просмотров на момент чтения; значение не убывает. Снятое размещение показывает значение перед удалением; без поля — просмотров нет, CPV нет | `placement/app` (отчёт) |
| E-5 | Публичная ссылка на пост | `publication.url` — из ответа MAX (`message.url`), вида `https://max.ru/c/{chat}/{mid}`; если MAX не вернул — ссылки в строке нет | отчёт, шаблоны |
| E-6 | Почта | SMTP принимает письмо синхронно, отказ адреса — ошибка `5xx` при `RCPT TO`; отчёты о недоставке (bounce) не обрабатываются | `integration/mail` |
| E-7 | Тексты уведомлений | Тексты написаны для этапа, без согласования с продуктом; правятся в `templates/notification` без изменения кода | `templates/notification` |
| E-8 | Чек выплаты | `payout.succeeded` получает `receipt_url` — ссылку на чек «Мой налог» из Рокет Ворк (05 §4.3); формат ссылки неизвестен, в шаблоне выводится как есть | `templates/notification/payout.succeeded.tmpl` |
| E-9 | Адреса экранов кабинета | Пути по 08 §2 с идентификатором последним сегментом: `/adv/orders/{id}`, `/adv/balance`, `/blg/placements?placement={id}`, `/blg/channels/{id}`, `/blg/payouts`, `/adm/placements/{id}`, `/adm/channels/{id}`, `/adm/orders/{id}`; маршруты фронтенда не сверялись | `notification/domain.Link` |
| E-10 | Отказ личного сообщения в журнале | Ожидаемые отказы `403`/`404` на `SendUserMessage` (`outbound.Call.ExpectedRejections`) — WARN `outbound call rejected as expected` без оповещения администратору; прочие `4xx` — как раньше, ERROR и оповещение `outbound_failed:max:SendUserMessage`. `bridge.NotificationMessenger` пишет код ошибки MAX на DEBUG | `integration/max`, `platform/outbound`, `bridge.NotificationMessenger` |

## Каталог шаблонов

| Код | Кому | Событие |
|---|---|---|
| `balance.deposited` | рекламодатель | S-30, зачисление |
| `closing_acts.ready` | рекламодатель | закрывающие документы |
| `order.post_approved` | рекламодатель | версия поста одобрена (новое) |
| `order.post_rejected` | рекламодатель | версия отклонена |
| `order.erir_rejected` | рекламодатель | ЕРИР отклонил креатив |
| `placement.request` | блогер | P-02 |
| `placement.accept_reminder` | блогер | 3 ч до T-01 (новое, R-8) |
| `placement.accepted` | рекламодатель | P-03 |
| `placement.scheduled`, `placement.rescheduled` | рекламодатель | P-17, P-07 |
| `placement.declined` | рекламодатель | P-04 |
| `placement.expired`, `placement.cancelled` | оба | P-05, P-18, P-06 |
| `placement.returned` | блогер | P-11 |
| `placement.published` | оба | P-08, со ссылкой |
| `placement.failed` | оба, администратор | P-09 |
| `placement.removed` | оба | P-10, с просмотрами (рекламодателю — с призывом принять или открыть претензию) |
| `placement.removal_overdue` | блогер | T-10 |
| `placement.disputed` | оба, администратор | P-14 |
| `placement.dispute_resolved` | оба | P-15 (только код) |
| `payout.sent`, `payout.succeeded`, `payout.failed`, `rocketwork.active`, `rocketwork.blocked` | блогер | выплаты и статус исполнителя (только коды) |
| `channel.approved`, `channel.rejected`, `channel.suspended`, `channel.disconnected` | блогер | смена состояния канала (новое) |
| `channel.restored` | блогер | права бота вернулись (S-25 шаг 3) |

## Подключение выплат и споров (после слияния этапов)

Выплаты подключены при слиянии ветки выплат (план `rocketwork-payouts.md`, раздел «Слияние с master»); споры — ещё нет.

- **Выплаты (`internal/payout`).** Порт `payout/domain.Notifier` с той же сигнатурой `Notify(ctx, Notice)`; в `bootstrap` — `bridge.PayoutNotifier` над `notification/app.Notify` (как `bridge.BillingNotifier`). Вызовы в транзакции перехода выплаты: создание выплаты после P-12/P-13 — `payout.sent` (роль `blogger`, `Data`: `placement_id`, `channel_id`, `order_number`, `amount`); P-16 — `payout.succeeded` (+`receipt_url`); выплата отклонена или исчерпаны попытки `send_payout` — `payout.failed` (+`reason` кодом, без текста Рокет Ворк). Объект — `payout`, ссылка — `/blg/payouts`. Статус исполнителя (`ApplyContractor`) — `rocketwork.active` / `rocketwork.blocked` (+`reason` кодом), объект — `rocketwork_contractor`. `payout.domain.Notice` без роли: переходник ставит `RoleBlogger`. `ContractorGate` в `newPlacementService` заменить на реализацию модуля выплат.
- **Приёмка и споры.** Модуль размещений уже уведомляет из `StateChange.Notices`: для P-12/P-13 добавить в `domain` `Notices` по таблице (блогеру — через `payout.sent` выплаты, отдельного шаблона приёмки нет); для P-15 — `p.notifyBoth(NoticeDisputeResolved)` с `Data` `performance_share`, `refund_amount`. Претензия (S-08) — P-14 уже уведомляет обоих и администратора.
- Проверка после подключения: тест каталога (`TestCatalogCodesHaveCallers` переводит коды из списка «ждут модуль» в обычные) и сквозной сценарий выплаты.

## Уточнения второго прохода (aif-improve)

- Задача 6: в `maxfake` общий `FailWith` ломает все вызовы, поэтому нужна подстановка отказа только для личных сообщений.
- Задача 7: обработчики кабинета не требуют открытого кабинета — администратор без организации тоже видит свои уведомления.
- Задача 10: у `placement/domain.Content` нет ссылок поста; адреса с UTM берутся из `post/app` (как для креатива ОРД). Зависимость от задачи 8 заменена на 7: задачи связаны только через `api/openapi.yaml`.

## Commit Plan
- **Commit 1** (после задач 1–3): `feat(notification): schema, template catalog and texts`
- **Commit 2** (после задач 4–7): `feat(notification): delivery by MAX bot and email, cabinet api`
- **Commit 3** (после задач 8–9): `feat(notification): notify on placement, order, payment and channel events`
- **Commit 4** (после задачи 10): `feat(placement): order report with views, links, cpv and csv export`
- **Commit 5** (после задач 11–12): `feat(notification): config, wiring and e2e`
- **Commit 6** (после задачи 13): `docs: notifications and campaign report`

## Tasks

### Phase 1: Данные, каталог, тексты

- [x] **Task 1: Миграция `notification` (R-4, R-10, R-16)**
  - Новая миграция (следующий свободный номер; в коде и документах номер не упоминается): `notification` (`id`, `user_id` → `user`, `organization_id` → `organization` (может быть пуст), `channel check in (max_bot, email, cabinet)`, `template`, `payload jsonb`, `status check in (pending, sent, failed, read)`, `object_type`, `object_id`, `dedupe_key`, `max_user_id bigint` (только `max_bot`), `attempts int`, `last_error text`, `sent_at`, `read_at`, `created_at`, `updated_at`); проверки: `read` только у `cabinet`, `read` ⇔ `read_at`, `max_bot` ⇔ `max_user_id`; индексы `(user_id, created_at DESC, id DESC) WHERE channel = 'cabinet'`, частичный `(user_id) WHERE channel = 'cabinet' AND read_at IS NULL`, `(status, created_at)`, уникальный `(user_id, channel, dedupe_key) WHERE dedupe_key IS NOT NULL`.
  - Права роли приложения: `SELECT, INSERT, UPDATE, DELETE` (удаление — очистка).
  - Журнал: не применимо.
  - Тесты: `up → down → up`; `read` у `email` не вставляется; повтор ключа не вставляется.
  - Файлы: `migrations/000NN_notifications.sql`, `internal/platform/dbtest` при необходимости.

- [x] **Task 2: `notification/domain` — каталог, адресаты, каналы (R-1, R-2, R-3, R-5, R-6, R-9, R-11)** (depends on 1)
  - `internal/notification/domain/notification.go`: `Channel`, `Status`, `Notification`, `Role` (`advertiser`, `blogger`, `admin`), `Notice{Template, OrganizationID, Role, ObjectType, ObjectID, Data, Key}`.
  - `catalog.go`: `Template{Code, Roles, Link func(data) string}`; список кодов из таблицы «Каталог шаблонов»; `Lookup(code)`; неизвестный код — `ErrUnknownTemplate` (ошибка программы).
  - `plan.go`: `Plan(notice, Recipients) []Notification` — строки по R-1…R-3, `payload` = данные + `link`; `MarkRead`, `MarkSent`, `MarkFailed`.
  - `ports.go`: `Repository`, `Recipients` (`Members(orgID)`, `Admins()`), `Channels` (`Channel(id) → title, maxUserID`), `Renderer`, `Messenger` (`SendUserMessage`), `Mailer` (`SendNotification`).
  - Журнал: не применимо (domain).
  - Тесты: каждому коду — роли; блогер получает cabinet+max_bot+email, без `added_by_max_user_id` — без max_bot; рекламодатель — cabinet+email; администратор — cabinet; пустые адресаты — нет строк; ключ переносится.
  - Файлы: `internal/notification/domain/{notification.go,catalog.go,plan.go,ports.go,errors.go,*_test.go}`.

- [x] **Task 3: Тексты и `notification/render` (R-5, R-17, E-2, E-3, E-7, E-8)** (depends on 2)
  - `templates/notification/{код}.tmpl` для каждого кода (блоки `title`, `text`, необязательный `email_text`); `templates/email/notification.{subject,txt,html}.tmpl`; `templates/embed.go` — шаблоны уведомлений в `FS`.
  - `internal/notification/render`: реализация `domain.Renderer`: `Cabinet(n) (title, text)`, `MAX(n) string` (Markdown, экранирование E-2, предел E-3, ссылка `CABINET_BASE_URL + link`), `Email(n) (subject, text, html)`. Функции шаблонов: `money` (рубли с пробелами), `msk` (время по Москве), `plural`.
  - Журнал: DEBUG `notification rendered` (`template`, `channel`, `length`).
  - Тесты: каждый код каталога имеет шаблон и собирается на образце данных без `<no value>`; длинное название канала обрезается до 4000; экранирование; HTML экранирует данные.
  - Файлы: `templates/notification/*.tmpl`, `templates/email/notification.*.tmpl`, `templates/embed.go`, `internal/notification/render/{render.go,funcs.go,render_test.go}`.
<!-- Commit checkpoint: tasks 1-3 -->

### Phase 2: Хранение, доставка, кабинет

- [x] **Task 4: `notification/postgres` (R-4, R-10, R-12, R-16)** (depends on 1, 2)
  - `sqlc.yaml`: запись модуля; `queries/notifications.sql`: вставка с `ON CONFLICT DO NOTHING`, чтение для отправки с `FOR UPDATE`, отметки `sent`/`failed`/попытка, список кабинета с курсором и `unread`, число по фильтру, отметка прочитанным (своё, кабинет), отметка всех, удаление старых конечных пачкой.
  - Журнал: не применимо (адаптер без событий предметной области).
  - Тесты (PostgreSQL): курсор, `unread`, чужое не отмечается, повтор ключа, очистка не трогает `pending`.
  - Файлы: `internal/notification/postgres/{store.go,notifications.go,*_test.go,queries/notifications.sql,sqlc/*}`, `sqlc.yaml`.

- [x] **Task 5: `notification/app` и задание `send_notification` (R-1…R-7, R-10)** (depends on 3, 4)
  - `Notify(ctx, Notice)` — только в транзакции вызывающего (`tx`): адресаты, `Plan`, вставка, `send_notification` для `max_bot`/`email` (I-10).
  - `Send(ctx, notificationID, attempt)` — строка не `pending` — выход; почта адресата читается в момент отправки (не в `payload` задания); бот — приоритет «ответы и уведомления»; успех — `sent`; отказ без повтора — `failed`; иначе ошибка (повтор), попытка и `last_error` (без персональных данных); `Exhausted` — `failed`.
  - `List`, `MarkRead`, `MarkAllRead`.
  - `internal/notification/jobs`: регистрация `send_notification` (5 попыток, `OnExhausted`, `NoFailureAlert`).
  - Журнал: события из Settings.
  - Тесты (PostgreSQL, `maxfake`, `mailfake`): уведомление блогеру — три строки и два задания; отправка — `sent`, текст в `maxfake`; отказ MAX `403` — `failed` без повтора, письмо всё равно ушло; повтор задания после `sent` — без второй отправки; откат транзакции события — ни строк, ни заданий.
  - Файлы: `internal/notification/app/{service.go,notify.go,send.go,cabinet.go,*_test.go}`, `internal/notification/jobs/jobs.go`, `internal/notification/notificationtest/*`.

- [x] **Task 6: Адаптеры доставки и переходники (R-1, R-3, R-7, E-1, E-6)** (depends on 5)
  - `internal/integration/mail`: `SendNotification(ctx, to, subject, text, html)`; `mailfake` запоминает такие письма.
  - `internal/bootstrap/bridge/notification.go`: `NotificationRecipients` (члены организации и администраторы над `account/postgres`), `NotificationChannels` (название и `added_by_max_user_id` над `channel/postgres`), `NotificationMessenger` (над `maxAPI.SendUserMessage` с `max.WithPriority(PriorityReply)`, отказ `403/404` — `job.Permanent`, E-1).
  - Журнал: адаптеры MAX и почты уже пишут свои события; переходники — DEBUG при пропуске адресата.
  - `maxfake`: отдельная подстановка отказа только для личных сообщений (`FailUserMessagesWith`), чтобы проверить «бот не может написать — письмо ушло»; общий `FailWith` ломает и публикации. `mailfake`: подстановка отказа для писем уведомлений.
  - Тесты: `mailfake` хранит письмо; переходник MAX переводит `403` в отказ без повтора (поддельный клиент).
  - Файлы: `internal/integration/mail/{mail.go,mail_test.go,mailfake/fake.go}`, `internal/bootstrap/bridge/notification.go`, при необходимости `internal/account/postgres`, `internal/channel/postgres` (выборки).

- [x] **Task 7: Кабинет и контракт уведомлений (R-12, R-13)** (depends on 5)
  - `api/openapi.yaml`: `Notification.order_id`, `Notification.link`; `go generate ./internal/apigen`.
  - `internal/notification/httpapi`: `listNotifications`, `markNotificationRead`, `markAllNotificationsRead` (нужна только сессия: принципал `httpx` из context без `RequireCabinet` — у администратора нет организации и кабинета); `unimplemented.go` генерируется `genstub` по всем операциям и не правится: обработчики модуля встраиваются в `apiImpl` выше заглушек. В `listNotifications` добавлен ответ `422` (испорченный курсор).
  - `mockapi`: `order_id` и `link` в уведомлениях; уведомления администратора, шаблоны `balance.deposited`, `order.post_approved`, `placement.disputed` в сценариях заглушки; контрактный тест проходит.
  - Журнал: INFO `notifications marked read` (`count`), DEBUG `notifications listed`.
  - Тесты (`httptest` поверх сервера): `401` без сессии; список, `unread`, курсор; чужое — `404`; `read-all` дважды — `204`.
  - Файлы: `api/openapi.yaml`, `internal/apigen/api.gen.go`, `internal/notification/httpapi/{handlers.go,handlers_test.go}`, `internal/bootstrap/unimplemented.go`, `mockapi/internal/{api/api.gen.go,handlers/auth.go,store/*,seed/seed.go}`.
<!-- Commit checkpoint: tasks 4-7 -->

### Phase 3: События

- [x] **Task 8: Размещения, заказ и пост (R-6, R-8, R-9, R-10)** (depends on 5)
  - `bridge.PlacementNotifier` (`placement/domain.Notice` → `notification/domain.Notice`) вместо `NopNotifier`.
  - `placement/domain`: роль `ActorAdmin` в `NoticeTo` для P-09 и P-14; `NoticePostApproved`, `NoticeAcceptReminder`; задание `placement_reminder` с P-02 и отмена при P-03…P-06 (R-8).
  - `placement/app`: `notice()` добавляет `publish_at`, `remove_at`; при P-08 — `publication_url`, при P-10 — `views`; `VersionChanged(approved)` — `order.post_approved`; `Remind(ctx, placementID)` для задания; `placement/jobs` регистрирует `placement_reminder`.
  - Журнал: INFO `placement reminder sent`, DEBUG `placement reminder outdated`.
  - Тесты: таблица переходов — уведомления по 03 §4 (кто получает); P-02 ставит напоминание, P-03 его отменяет; напоминание после P-05 — без уведомления; одобрение версии — `order.post_approved`.
  - Файлы: `internal/placement/domain/{placement.go,actions.go,publish.go,jobs.go,*_test.go}`, `internal/placement/app/{transition.go,versions.go,reminder.go,*_test.go}`, `internal/placement/jobs/jobs.go`, `internal/bootstrap/bridge/{placement.go,notification.go}`.

- [x] **Task 9: Оплата, закрывающие документы, каналы** (depends on 5)
  - `bridge.BillingNotifier` вместо `billingapp.NopNotifier`.
  - `channel/domain.Notifier` и вызов в `channel/app.save` при смене состояния: `moderation → active` — `channel.approved`; `→ rejected` — `channel.rejected` (причина — текст администратора, в `payload` можно: это не журнал); `active → suspended` — `channel.suspended`; `→ disconnected` — `channel.disconnected` (кроме `unclaimed_expired`: канал без организации); `suspended → active` — `channel.restored`. `bridge.ChannelNotifier`.
  - Журнал: без новых событий (пишет `notification/app`).
  - Тесты: одобрение и отклонение канала — уведомления блогеру; зачисление оплаты — `balance.deposited`; закрывающие документы — `closing_acts.ready`.
  - Файлы: `internal/channel/{domain/ports.go,app/service.go,app/*_test.go}`, `internal/bootstrap/{billing.go,channel.go}`, `internal/bootstrap/bridge/notification.go`.
<!-- Commit checkpoint: tasks 8-9 -->

### Phase 4: Отчёт

- [x] **Task 10: Отчёт по заказу и CSV (R-14, R-15, E-4, E-5)** (depends on 7: обе задачи правят `api/openapi.yaml` и генерацию)
  - `placement/domain/report.go`: `ReportRow`, `ReportTotals`, `BuildReport(order, placements, publications, holds)` — суммы по R-14, CPV (D-28) с округлением вниз; CSV — `WriteReportCSV`.
  - Уточнение при реализации: ссылки и постоянные UTM-метки идут через `VersionInfo` (`Links`, `UTMSource`, `UTMMedium`, заполняет `bridge.PlacementVersions` из `post/domain.BodyLinkURLs` и констант `post/domain.UTMSource/UTMMedium`), а не через `Content`: отчёту не нужно содержание публикации. В итог добавлен `held_amount`; CPV итога — потрачено, делённое на суммарные просмотры (D-28).
  - `placement/app.OrderReport(ctx, orgID, orderID)` — заказ своей организации, размещения, публикации, каналы, активные удержания (`Holds.HasActive`), ссылки текущей версии. В `placement/domain.Content` ссылок нет: добавить `Links []string` и заполнить в `bridge.PlacementContent` из `post/app` (адреса с UTM уже собирает `post/app/creative.go` для креатива ОРД — переиспользовать, не дублировать разбор).
  - `api/openapi.yaml`: `OrderReportRow.cpv_amount`, `OrderReportTotals.cpv_amount`, `OrderReport.links`, `OrderReport.utm` (`source`, `medium`, `campaign`), операция `getOrderReportCsv`; генерация; `mockapi` — те же поля и CSV.
  - `placement/httpapi`: `GetOrderReport`, `GetOrderReportCsv` (`Content-Disposition: attachment; filename="order-{номер}-report.csv"`).
  - Журнал: DEBUG `order report built` (`order_id`, `rows`, `views`, `spent_amount`).
  - Тесты: доменные — суммы и CPV на граничных случаях (нет просмотров, 0 просмотров, 1 просмотр, спор), CSV; PostgreSQL — отчёт после публикации и снятия с просмотрами; чужой заказ — `404`.
  - Файлы: `internal/placement/domain/{report.go,report_test.go}`, `internal/placement/app/{report.go,report_test.go}`, `internal/placement/httpapi/{report.go,report_test.go}`, `api/openapi.yaml`, `internal/apigen/api.gen.go`, `mockapi/internal/{api/api.gen.go,handlers/orders.go,server/contract_test.go}`.
<!-- Commit checkpoint: task 10 -->

### Phase 5: Сборка, сквозной сценарий, документы

- [x] **Task 11: Конфигурация, сборка, очистка, метрики (R-16, R-17)** (depends on 6, 7, 9, 10)
  - `internal/bootstrap/config_notification.go`: `CABINET_BASE_URL`, `NOTIFICATION_RETENTION` (8760h), `NOTIFICATION_SEND_ATTEMPTS` (5); проверки (абсолютный адрес, `https` в `production`).
  - `internal/bootstrap/notification.go`: сборка `notification/app`, рендер, переходники; маршруты httpapi; задания `send_notification`, `placement_reminder`; задача очистки; метрика `maxim_notifications{channel,status}` — число строк по каналу и состоянию, снимается из базы при сборе (один сборщик вместо счётчика и отдельного `pending`: состояние видно из базы и после перезапуска worker).
  - `.env.example`, `compose.dev.yml` (`deploy/` — только mockapi, отдельного примера окружения бэкенда там нет).
  - Журнал: INFO `notifications configured` (`cabinet_base_url`, `retention`).
  - Тесты: конфигурация на границах; виды заданий зарегистрированы; операции уведомлений больше не `501`.
  - Файлы: `internal/bootstrap/{config.go,config_notification.go,config_notification_test.go,notification.go,modules.go,placement.go,billing.go,channel.go,worker.go}`, `.env.example`, `deploy/.env.example`, `compose.dev.yml`.

- [x] **Task 12: Сквозной сценарий** (depends on 11)
  - `internal/bootstrap/notification_e2e_test.go` на собранных `api` и `worker`: оформление → одобрение версии (`order.post_approved` рекламодателю), P-02 — блогеру бот MAX (`maxfake`), письмо (`mailfake`), кабинет; планирование, публикация — рекламодателю письмо со ссылкой; снятие — просмотры в письме; отчёт и CSV с просмотрами, ссылкой и `utm_campaign`; `read-all`; в журнале нет почты без маски и текста уведомлений.
  - Журнал: проверка сборщиком записей.
  - Файлы: `internal/bootstrap/notification_e2e_test.go`, при необходимости `internal/bootstrap/harness_test.go`.

- [x] **Task 13: Документы** (depends on 12)
  - 04 §8 `notification` (новые поля, R-4), 07 §2 (`send_notification`, `placement_reminder`), §5.5 (метрики), §6 (R-1…R-3, R-7, R-9), §8 (параметры); 02 S-09 (CSV, CPV, ссылки, R-14, R-15); 03 §5 (напоминание T-01, R-8); `docs/open-questions.md` (Q-38 — рабочий ответ: почта блогеру всегда); `docs/frontend-api-gaps.md` (закрытые пункты MAXI-50, CSV MAXI-46); `.ai-factory/ARCHITECTURE.md` (модуль `notification`, переходники); README.
  - Журнал: не применимо.
  - Файлы: `docs/02-scenarios.md`, `docs/03-placement.md`, `docs/04-domain-model.md`, `docs/07-non-functional.md`, `docs/open-questions.md`, `docs/frontend-api-gaps.md`, `.ai-factory/ARCHITECTURE.md`, `README.md`.
<!-- Commit checkpoint: task 13 -->

## Критерии готовности этапа

- `go build ./...`, `go vet ./...`, `go test ./...` проходят; `sqlc generate` и `go generate ./...` не дают изменений; `mockapi` собирается и проходит контрактный тест.
- Каждое событие из таблицы 07 §6, которое есть в master, создаёт уведомление в транзакции события; сбой доставки не откатывает событие.
- `send_notification` идемпотентно: повтор после `sent` не отправляет второй раз.
- Кабинет: список, прочитано, все прочитаны; `order_id` и `link` в уведомлении.
- Отчёт по заказу: просмотры, ссылки, UTM, суммы и CPV по каждому размещению; CSV.
- Почта и текст уведомлений не попадают в журнал без маски.
