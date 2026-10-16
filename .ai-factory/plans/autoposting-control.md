# План реализации: Автопостинг и контроль

Branch: master (без отдельной ветки, по решению пользователя)
Created: 2026-10-17

## Settings
- Testing: yes. Решения публикации, снятия и контроля (предусловия S-21, окно повторов T-03 и T-10, пересчёт `remove_at`, интервал топа, предложение доли 03 §7, классификация событий и расхождений сверки) проверяются чистыми табличными тестами `placement/domain`. Адаптер MAX — через `httptest` с образцами ответов в `internal/integration/max/testdata`. Сценарии `placement/app` с `postgres` выполняются на настоящей PostgreSQL через `testcontainers-go` с `clock.Manual`, `maxfake`, `storagefake` и поддельными портами денег и уведомлений. Сквозные сценарии — на собранных `api` и `worker` (07 §9).
- Logging: verbose. `log/slog` JSON, поля и обязательные события 07 §5.3, 03 §12, 05 §1: переход выполнен (INFO) и отклонён (WARN); пост опубликован и снят (INFO, внешняя операция выполнена); повтор публикации или снятия (WARN); повторы исчерпаны (ERROR + оповещение); нарушение найдено (WARN); повторное событие проигнорировано (DEBUG); истёк T-06, T-10 (WARN + оповещение). Поля `order_id`, `placement_id`, `channel_id`, `publication_id`, `max_message_id`, `job_id`, `inbound_event_id`. Текст поста, текст сообщений канала, токены вложений, название и ИНН рекламодателя в журнал не пишутся (07 §5.4).
- Docs: yes. В конце обязательная точка `/aif-docs`: README бэкенда (новые переменные окружения, как пройти публикацию и снятие руками в `compose.dev.yml` с `maxfake`, что T-05 ждёт этапа приёмки, что решение спора — этап администратора).

## Roadmap Linkage
Milestone: "Автопостинг и контроль"
Rationale: план реализует этап целиком: публикацию и снятие по расписанию (S-21, S-22, P-08, P-09, P-10), сверку факта с планом (T-09, 05 §2.8), контроль «часов в топе» (S-27, T-08), нарушения и открытие спора системой (S-25 для `published`, S-26, S-27, P-14). Решение спора, претензии и автоприёмка — следующий этап.

## Research Context
Source: .ai-factory/RESEARCH.md (Active Summary), выдержка по этапу

- Посты в канале всегда выходят с уведомлением (`notify=false` не поддерживается). Текст до 4000 символов, формат `html`.
- Вложения: `POST /uploads` → токен; после загрузки возможна ошибка `attachment.not.ready` — нужны повторы.
- До 30 rps на платформу, до 2 сообщений/с в канал (у нас 25 и 1, 05 §2.8).
- Бот-администратор обязателен: даёт автопостинг, автоудаление и контроль ручных публикаций через события.
- После получения erid пост неизменяем (D-03); правка рекламного поста в MAX не делается.
- Открыто и проверяется на тестовом канале (этап «Проверки и доступы»): приходят ли `message_created/edited/removed` для постов людей (Q-03), видит ли их `GET /messages` (Q-04), возвращается ли `stat.views` (Q-08), срок давности удаления (Q-07). Рабочие ответы реестра — «да» (A-17); план реализует оба пути контроля: события и сверку.

## Источники ТЗ

- Сценарии: 02 S-21 (публикация), S-22 (снятие), S-25 шаг 2 (`published` → P-14 `bot_access_lost`), S-26 (удалён или изменён раньше срока), S-27 (часы в топе); S-24 и S-18 — только границы (T-05, решение спора — следующие этапы).
- Размещение: 03 §2 (нарушения формата), §4 (P-08, P-09, P-10, P-14, особые правила снятия в `disputed`), §5 (T-02 … T-10), §6 (ветки отказа), §7 (предложение доли), §8 (контроль размещения), §10 (I-01, I-02, I-06, I-08, I-11, I-13), §11 (идемпотентность публикации, неизвестный исход), §12.
- Данные: 04 §5 (`publication`, `violation`, `dispute`; поля `placement.published_at`, `removed_at`, `claim_deadline_at`, `final_views`), §9 (перечисления).
- Интеграции: 05 §1 (повторы, неизвестный исход, успех по телу), §2.2 (реакция на события), §2.3 (методы), §2.6 (вложения, публикация, снятие), §2.7 (просмотры), §2.8 (ограничитель, приоритеты, сверка), §2.9.
- Требования: 07 §2 (`publish_placement`, `remove_placement`, `reconcile_publications`, «после простоя», точность 10 с), §3 (сверка, зависшие `scheduled` и `removal_overdue`, оповещения), §5.3, §5.5 (задержка публикации и снятия), §8.
- Реестр: A-01, A-17, A-18, A-23, A-27, Q-03, Q-04, Q-05, Q-07, Q-08.
- Архитектура: `.ai-factory/ARCHITECTURE.md` — «Модуль → внешняя система» (порт `placement/domain.ChannelPublisher`, реализация `integration/max`, вызов только из `jobs` через `app`), «Входящие события», «Порядок блокировок», «Одна таблица переходов», «Порты к будущим модулям».
- Существующий код: `internal/placement/domain/{transitions.go,timers.go,placement.go,ports.go,status.go}` (P-08 … P-14 в таблице, `PublishJob`, `RemoveJob`, `PlacementPayload`), `internal/placement/app/{transition.go,timers.go,channel_events.go,service.go}` (общий `apply`, `HandleTimer`, WARN `placement needs dispute, not implemented`, `OverdueScheduled`), `internal/placement/jobs/jobs.go` (`objectRef`), `internal/integration/max/{client.go,messages.go,updates.go,chats.go,subscriptions.go,errors.go}`, `internal/integration/max/maxfake/*`, `internal/channel/app/link.go` (`ChannelPostCreated` — сейчас только код подключения), `internal/channel/jobs/jobs.go` (маршрут `message_created`), `internal/platform/inbound/dispatcher.go` (один обработчик на вид события), `internal/platform/job/{downtime.go,worker.go}` (`Overdue`, `ObjectRef`, `PriorityHigh`), `internal/platform/ratelimit` (`PriorityPublish`, приоритеты сверки), `internal/post/app/versions.go` (`GetVersion`, `MarkVersionReady`), `internal/post/domain` (сборка `final_text`), `internal/platform/storage` (`Get`), `internal/bootstrap/{integration.go,channel.go,placement.go,worker.go}` (`maxAPI`, `ensureMAXSubscription`, `stuckScheduledCheck`).

## Границы этапа

Входит:
- Обработчик `publish_placement` (S-21): предусловия, загрузка изображений, отправка, неизвестный исход, повторы до T-03, P-08 и P-09, политика «после простоя».
- Обработчик `remove_placement` (S-22): итоговые просмотры, запись намерения, удаление, P-10, `removal_overdue`, повторы до T-10, снятие в `disputed`.
- Таблицы `publication`, `violation`, `dispute` (без полей решения в коде — только хранение).
- Контроль по событиям: `message_created` в канале (топ), `message_edited`, `message_removed` (S-26, S-27); `published` при потере канала (S-25) — P-14 `bot_access_lost`.
- P-14 системой: спор с причиной, нарушением, сроком T-06 и предложением доли (03 §7). Таймер T-06 — оповещение старшему администратору.
- Сверка T-09: `reconcile_publications` и проверка каждой живой публикации; поиск `scheduled` без задания публикации (05 §2.8 шаг 5); обновление просмотров.
- Методы адаптера MAX: загрузка изображения, отправка поста с вложениями, чтение сообщения, разбор `message_edited` и `message_removed`, идентификатор нашего бота; подписка на новые виды событий.
- Публикация в ответах кабинетов: `publication_url`, `views`, сводка спора.
- Зависшие `removal_overdue` (07 §3), метрики задержки публикации и снятия, число нарушений.

Не входит (следующие этапы):
- P-12, P-13 (автоприёмка по T-05), выплата: этап «Приёмка, споры и возвраты» и «Выплаты». Здесь P-10 записывает `claim_deadline_at` и ставит T-05 (R-14).
- Претензия рекламодателя (`createPlacementClaim`, таблица `claim`), P-15, решение спора, переписка (`dispute_message`), `admin_open_dispute`, `admin_mark_removed`: этапы приёмки и API администратора.
- Отправка уведомлений: порт `Notifier` с пустой реализацией (R-4 этапа «Заказ и размещения»), новые коды шаблонов — R-19.
- Отчёт по кампании и UTM-статистика (`getOrderReport`): этап «Уведомления и отчёт».
- Регистрация креатива и erid: этап «Маркировка»; здесь erid только проверяется (I-01).

## Решения этапа, требующие подтверждения

Документы этих мест не определяют или противоречат друг другу. План реализует указанный вариант. После подтверждения задача 16 вносит его в документы. При отказе меняется только отмеченная задача.

| # | Вопрос | Решение в плане | Задача |
|---|---|---|---|
| R-1 | Порты публикации | `placement/domain.ChannelPublisher` (реализация — переходник над `integration/max` в `bootstrap`): `UploadImage(ctx, chatID, r io.Reader, size, contentType) (token, error)`, `SendPost(ctx, chatID, Post{Text, ImageTokens}) (SentMessage{ID, URL, At, ContentHash}, error)`, `GetMessage(ctx, messageID) (ChannelMessage{ID, At, SenderID, ContentHash, Views}, error)` (нет сообщения — `ErrMessageNotFound`), `DeleteMessage(ctx, chatID, messageID) error`, `ListMessages(ctx, chatID, from, to) ([]ChannelMessage, error)`, `BotUserID(ctx) (int64, error)`. Содержание — порт `placement/domain.PublicationContent` (реализация — `post/app`): `Get(ctx, versionID) (Content{VersionID, Status, Erid, FinalText, Media []MediaRef{DerivativeID, ContentType, Size}}, error)` и `Open(ctx, derivativeID) (io.ReadCloser, error)`. `domain` не знает о `io` хранилища: `Open` возвращает `io.ReadCloser` из стандартной библиотеки | 3, 5, 7, 14 |
| R-2 | Публикация в три шага (I-13) | Шаг 1 — транзакция: блокировка канала (`LockForPlacement`, порядок блокировок), затем размещения; не `scheduled` или публикация уже есть — DEBUG `publish job outdated` и выход; предусловия S-21 шаг 1 (R-3) не выполнены — сразу P-09 `precondition_failed`. Шаг 2 — вне транзакции: загрузка изображений по порядку (токены не переиспользуются, 05 §2.6), `SendPost`. Шаг 3 — транзакция: вставка `publication`, P-08 (`published_at` — время сообщения из ответа MAX, `remove_at = published_at + duration_hours`), перенос задания снятия `Reschedule(remove:{id}, remove_at)`, уведомления. Если шаг 3 не удался после отправки — повтор задания найдёт сообщение по R-5 | 8 |
| R-3 | Предусловия S-21 шаг 1 и I-01 | Размещение `scheduled`; канал `active` (I-08; `active` уже означает права `write` и `delete`, 05 §2.4); у версии статус `ready`, erid не пуст, `final_text` равен тексту, собранному заново сборщиком `post/domain` из `body` и параметров маркировки версии (`post/domain.VerifyFinalText`), и содержит erid, если erid в тексте (над или под постом). Формулировка I-01 «заканчивается строкой маркировки» противоречит 05 §2.5 (строка бывает над постом) — исправляется в 03 (задача 16). Причина отказа — в `reason_text` истории: `channel_not_active`, `version_not_ready`, `marking_mismatch` | 3, 7, 8 |
| R-4 | Повторы публикации (T-03) | Ошибки, которые 05 §1 повторяет, и занятый ограничитель — `job.Retry(…, PUBLISH_RETRY_INTERVAL)` (30 с), пока `now < publish_at + PUBLISH_RETRY_WINDOW` (30 мин, T-03); дальше — P-09 `publish_error`. Отказ `4xx` (например, `403` — бот не администратор) — P-09 `publish_error` сразу, код ошибки MAX в `reason_text`. `attachment.not.ready` повторяет адаптер внутри вызова: 1, 2, 4, 8, 15, 30 с (05 §2.6 шаг 4). P-09: отмена `remove:{id}`, `Holds.Release`, уведомления обоим, оповещение ERROR `placement_publish_failed:{id}`. Политика просрочки вида: `Overdue{After: PUBLISH_RETRY_WINDOW, OnExpired: P-09 publish_error}` (07 §2, «После простоя») | 3, 8, 12 |
| R-5 | Неизвестный исход отправки (03 §11) | Перед каждой повторной попыткой (`Attempts > 0`) и при ошибке шага 3 задание читает `ListMessages` канала за `[publish_at − 1 мин, now]` и ищет сообщение нашего бота с тем же хешем содержимого, что дал бы отправленный текст (R-6), либо, если хеш сравнить нельзя, с текстом, содержащим erid версии. Найдено — шаг 3 с этим сообщением, INFO `publication found after unknown outcome`. Окно поиска ограничено `PUBLISH_FIND_LOOKBACK` (40 мин) | 5, 8 |
| R-6 | Хеш содержимого (`publication.content_hash`) | Текст, который возвращает MAX, может отличаться от отправленного HTML (разметка может приходить отдельно). Поэтому хеш считается адаптером из сообщения в том виде, в каком его хранит MAX: SHA-256 от нормализованного `body.text`, `body.markup` (если есть) и списка вложений (тип и идентификатор фото по порядку). Одна функция `max.ContentHash(Message)` — для ответа `POST /messages`, `GET /messages/{id}`, `GET /messages` и события `message_edited`. Проверяется на тестовом канале (этап «Проверки и доступы», метка Q-06) | 5 |
| R-7 | Снятие (S-22, 05 §2.6) | Шаг 1 — транзакция: блокировка размещения и публикации; публикация уже снята (`removed_at` есть, не `removal_overdue`) — DEBUG и выход; размещение не `published` и не `disputed` — DEBUG и выход. Шаг 2 — вне транзакции: `GetMessage` — просмотры; `ErrMessageNotFound` — досрочное удаление (S-26, R-10) и выход. Шаг 3 — транзакция: намерение `removed_at = now`, `removal_reason = schedule`. Шаг 4 — вне транзакции: `DeleteMessage` (успех по телу). Шаг 5 — транзакция: `published` → P-10 (`removed_at`, `final_views`, `claim_deadline_at = removed_at + CLAIM_WINDOW`, T-05 по R-14, уведомления); `disputed` — состояние не меняется (03 §4). Ошибка шага 4 — транзакция: откат намерения, `removal_overdue = true`, повтор с задержкой `min(30 с × 2^attempts, 1 ч)` до `remove_at + REMOVE_RETRY_WINDOW` (24 ч, T-10); после — без повторов, WARN `publication removal overdue` и оповещение `publication_removal_overdue:{id}`; публикация остаётся `removal_overdue` до `admin_mark_removed` (этап администратора). Задержка снятия — не нарушение блогера (07 §2) | 3, 9 |
| R-8 | Свои действия против чужих | Своё удаление отличается от чужого записью намерения до вызова (05 §2.2): `message_removed` по публикации с `removed_at` — повтор, DEBUG. Своё сообщение — `sender.user_id` равен `BotUserID` (из `GET /me`, кешируется адаптером на время жизни процесса; можно задать `MAX_BOT_USER_ID`). Сообщение без отправителя (пост от имени канала) — чужое | 5, 10 |
| R-9 | Маршрут событий сообщений | `message_edited` и `message_removed` регистрирует `placement/jobs` в `inbound.Dispatcher` (публикация ищется по `max_message_id`, канал не нужен). `message_created` остаётся у `channel/jobs`: `channel/app.ChannelPostCreated` разбирает код подключения, а всё остальное в известном канале передаёт новому порту `channel/domain.ChannelPostListener.ChannelPostCreated(ctx, ChannelPost{ChannelID, MaxChatID, MessageID, SenderID, At, InboundEventID})` (пустая реализация по умолчанию, `bootstrap` подставляет `placement/app` через `SetPlacementPorts`). Сообщение, разобранное как код подключения, топ не нарушает: бот его удаляет. Сообщения в неизвестном канале — `ignored` | 5, 7, 10 |
| R-10 | Нарушения и P-14 (S-25, S-26, S-27) | Одна функция `placement/app.reportViolation`: в транзакции блокировка размещения; вставка `violation{kind, occurred_at, source, inbound_event_id, details}`; размещение `published` или `removed` и спора ещё нет → P-14 и `dispute`; уже `disputed` (I-11) → только нарушение, WARN `violation recorded, dispute exists`. Виды: `removed_early` — публикация `removed_at = occurred_at`, `removal_reason = external`, отмена `remove:{id}`; `content_modified` — хеш из события или сверки не равен `content_hash`; `top_violated` — чужое сообщение с временем в `[published_at, published_at + top_hours)`; `bot_access_lost` — потеря канала при `published`. Время нарушения — время сообщения MAX (событие может прийти с задержкой), для сверки удаления — время проверки. WARN `placement violation detected` (`kind`, `source`, `occurred_at`). Повтор события (тот же `inbound_event_id`) или повтор сверки по тому же виду при открытом споре новой записи не создаёт | 2, 4, 10, 11 |
| R-11 | Спор, который открывает система | `dispute`: `reason` = вид нарушения, `opened_by_type = system`, `violation_id`, `deadline_at = now + DISPUTE_DEADLINE` (72 ч, T-06), `suggested_share` по 03 §7: `removed_early` — `floor(10000 × (removed_at − published_at) / (remove_at − published_at))`, `top_violated` — `DISPUTE_TOP_VIOLATION_SHARE` (5000), остальные — пусто. P-14: отмена T-05 (если из `removed`), таймер T-06 (`placement_timer`, ожидаемое `disputed`) — по срабатыванию WARN `dispute deadline passed` и оповещение `dispute_overdue:{id}`, состояние не меняется. Уведомления обоим (`placement.disputed`) и оповещение администратору WARN `placement_disputed:{id}`. Поля решения в таблице есть, пишет их этап «Приёмка, споры и возвраты» | 2, 3, 10 |
| R-12 | Сверка T-09 (05 §2.8, A-27) | Периодическое `reconcile_publications` (каждые `RECONCILE_INTERVAL`, 5 мин) ставит на каждую живую публикацию (`removed_at` пуст) задание `check_publication` с `run_at`, распределённым по интервалу по `publication_id`, `dedupe_key check_publication:{id}`, 3 попытки, приоритет ограничителя — сверка. `check_publication`: `GetMessage` — нет → `removed_early`, хеш отличается → `content_modified`; пока идут часы в топе — `ListMessages` за `[published_at, min(now, published_at + top_hours))`, первое чужое сообщение → `top_violated`; просмотры — если с `views_checked_at` прошло больше `VIEWS_REFRESH_INTERVAL` (1 ч). Там же шаг 5: размещения `scheduled` с `publish_at` старше `RECONCILE_SCHEDULED_GRACE` (5 мин) без публикации и без живого задания `publish:{id}` — задание создаётся заново, ERROR `publish job missing, recreated`. Новый вид `check_publication` вносится в 07 §2 | 4, 11, 12 |
| R-13 | Сообщения канала для сверки | `max.ChannelMessage` получает `SenderID` и `ContentHash`; текст читается только внутри адаптера для хеша и поиска erid и не возвращается наружу и не пишется в журнал (07 §5.4). Метрики каталога (`channel/app`) не меняются: `FromBot` остаётся | 5 |
| R-14 | T-05 до этапа приёмки | P-10 записывает `claim_deadline_at` и ставит `placement_timer` T-05 (`dedupe_key timer:{id}:T-05`, ожидаемое `removed`, попытки — без предела). До этапа приёмки обработчик T-05 отвечает `job.Retry(…, 1 ч)` с DEBUG `auto acceptance not implemented yet`: таймер не теряется, этап приёмки заменит ветку на P-13. Альтернатива (не ставить T-05, а досоздать при выпуске этапа приёмки) — отклонена: проще потерять размещение | 3, 9 |
| R-15 | Точность и приоритет | `publish_placement`, `remove_placement` — `PriorityHigh`, `ObjectRef` `placement:{id}` (список затронутых после простоя); вызовы MAX — `ratelimit.PriorityPublish` (ожидание без предела). `check_publication` и `reconcile_publications` — обычный приоритет, ограничитель — сверка. Метрики `maxim_placement_publish_delay_seconds` и `maxim_placement_remove_delay_seconds` (гистограммы `published_at − publish_at`, `removed_at − remove_at`, 07 §5.5), `maxim_placement_violations_total{kind,source}` | 8, 9, 12 |
| R-16 | Изображения для публикации | Источник — производная версии (`post_version_media.derivative_id`), байты из закрытой области хранилища через `PublicationContent.Open` (не по публичной ссылке: не зависим от CDN). Не больше 10 изображений (A-23); больше — P-09 `precondition_failed`, `reason_text = too_many_images`. Загрузка — `POST /uploads?type=image`, затем передача файла по выданному адресу (таймаут 120 с, 05 §1) | 5, 7, 8 |
| R-17 | Подписка на события | `max.ChannelUpdateTypes` дополняется `message_edited`, `message_removed`. `ensureMAXSubscription` переподписывает бота, если подписка с нашим адресом есть, но без какого-либо вида из списка (сейчас сверяет только адрес) | 5, 14 |
| R-18 | Ответы кабинетов | `AdvertiserPlacement` и `BloggerPlacement`: `publication_url` — `publication.url`; `views` — `final_views`, если снято, иначе `publication.views`; новое поле `dispute` (`PlacementDisputeSummary`: `reason`, `opened_by_type`, `opened_at`, `deadline_at`) — только чтение. Предложение доли и нарушения в кабинет не выводятся (это материал администратора) | 1, 13 |
| R-19 | Уведомления этапа | Коды шаблонов для порта `Notifier`: `placement.published` (P-08, обоим), `placement.failed` (P-09, обоим), `placement.removed` (P-10, обоим), `placement.removal_overdue` (T-10, блогеру), `placement.disputed` (P-14, обоим). Реализация до модуля уведомлений — `NopNotifier` | 8–10 |
| R-20 | Права на таблицы | `violation` — только вставка (как `placement_state_history`, 04 §1). `publication` и `dispute` — изменяемые; уникальны `publication.placement_id`, `publication.max_message_id`, `dispute.placement_id` (I-11) | 2 |

## Commit Plan
- **Commit 1** (после задач 1–2): `feat(placement): publication, violation and dispute schema`
- **Commit 2** (после задач 3–4): `feat(placement): publish, remove and control rules`
- **Commit 3** (после задач 5–7): `feat(max): post publishing adapter and placement ports`
- **Commit 4** (после задач 8–10): `feat(placement): autoposting, removal and violations`
- **Commit 5** (после задач 11–12): `feat(placement): publication reconciliation, stuck checks and metrics`
- **Commit 6** (после задач 13–14): `feat(placement): publication in cabinets and wiring`
- **Commit 7** (после задач 15–16): `test(placement): autoposting e2e, docs sync`

## Tasks

### Phase 1: Договор и данные

- [x] **Task 1: Спецификация: публикация и спор в ответах кабинетов (R-18)**
  - `api/openapi.yaml`:
    - Схема `PlacementDisputeSummary` (`reason` — перечисление 03 §6, `opened_by_type`, `opened_at`, `deadline_at`; только чтение).
    - `AdvertiserPlacement.dispute`, `BloggerPlacement.dispute`; описания `publication_url` (ссылка MAX) и `views` (последнее считанное значение; после снятия — итоговое).
  - `go generate ./internal/apigen`. `mockapi` собирается из той же спецификации: поле заполняется там, где тестовый сервер уже моделирует спор, иначе остаётся пустым — контрактный тест проходит.
  - Журнал: не применимо.
  - Тесты: существующие тесты `bootstrap` и `mockapi` (`contract_test.go`) проходят.
  - Файлы: `api/openapi.yaml`, `internal/apigen/api.gen.go`, `mockapi/internal/api/api.gen.go`, при необходимости `mockapi/internal/handlers/*`.

- [x] **Task 2: Миграция публикаций, нарушений и споров (R-10, R-11, R-20)**
  - `migrations/00008_publications.sql` по 04 §5:
    - `publication` (`id`, `placement_id` уникально → `placement`, `channel_id`, `max_message_id text` уникально, `url`, `published_at`, `removed_at`, `removal_reason check in (schedule, external, admin, refund)`, `removal_overdue boolean default false`, `views`, `views_checked_at`, `content_hash text not null`, `created_at`, `updated_at`; `check`: `removal_reason` задаётся вместе с `removed_at`; индексы `(removed_at) WHERE removed_at IS NULL` для живых, `(channel_id) WHERE removed_at IS NULL`, `(removal_overdue) WHERE removal_overdue`).
    - `violation` (`id`, `placement_id`, `kind check` по 04 §5, `occurred_at`, `source check in (event, reconciliation)`, `inbound_event_id`, `details jsonb not null default '{}'`, `created_at`; уникальна пара `(placement_id, inbound_event_id)` при непустом событии; индекс `(placement_id, occurred_at)`).
    - `dispute` (поля 04 §5, `reason check` по 03 §6, `opened_by_type check`, `claim_id` без внешнего ключа до этапа претензий, `violation_id` → `violation`, `deadline_at`, `suggested_share check between 0 and 10000`, поля решения пустые, `check` на `resolution`, `performance_share between 0 and 10000`; `placement_id` уникально).
    - Индекс `placement (publish_at) WHERE status = 'scheduled'` (04 §5, 07 §3), если его нет в `00007`.
  - Права роли: через `ALTER DEFAULT PRIVILEGES` из `00002`; для `violation` — `REVOKE UPDATE, DELETE` у роли приложения.
  - Журнал: не применимо (INFO `migrations applied` пишет `migrate`).
  - Тесты: `up` → `down` → `up`. Вторая публикация размещения и второй спор не вставляются (I-11). Роль приложения не меняет и не удаляет `violation`. `removal_reason` без `removed_at` не проходит.
  - Файлы: `migrations/00008_publications.sql`.
<!-- Commit checkpoint: tasks 1-2 -->

### Phase 2: Предметные правила

- [x] **Task 3: `placement/domain` — публикация, снятие, спор, таймеры (R-1 … R-4, R-7, R-11, R-14)** (depends on 2)
  - `internal/placement/domain/publication.go`: `Publication` (поля 04 §5), `RemovalReason`, `(*Publication).MarkRemoving(now)`, `RollbackRemoving()`, `MarkOverdue()`, `MarkRemovedExternally(at)`, `UpdateViews(v, at)`.
  - `internal/placement/domain/publish.go`: `CheckPublish(p, channel ChannelOffer, content ContentCheck, maxImages) error` по R-3 (`PreconditionError{Reason}`); `PublishRetry(p, now, rules) (retryAfter time.Duration, fail bool)` по R-4; `RemoveRetry(p, attempts, now, rules) (retryAfter, overdue bool)` по R-7.
  - Методы сценариев на `Placement`, собирающие `StateChange` с заданиями и уведомлениями: `Publish(msg SentMessage, now)` (P-08: `published_at`, `remove_at` заново, перенос `remove:{id}`), `FailPublish(reason, text, now)` (P-09: отмена `remove:{id}`), `Remove(views, now, rules)` (P-10: `removed_at`, `final_views`, `claim_deadline_at`, T-05), `OpenDispute(v Violation, pub Publication, now, rules) (Dispute, StateChange, error)` (P-14: отмена T-05, T-06).
  - `internal/placement/domain/dispute.go`: `Dispute`, `Violation`, `ViolationKind` (`removed_early`, `content_modified`, `top_violated`, `bot_access_lost`), `SuggestedShare(kind, pub, p, rules)` по 03 §7 через целочисленную арифметику.
  - `internal/placement/domain/timers.go`: `TimerClaim` (T-05), `TimerDispute` (T-06); `KindCheckPublication`, `KindReconcilePublications`; `CheckPublicationJob(id, runAt)`; `allJobKeys` дополняется T-05 и T-06. `ControlRules` (`PublishRetryWindow`, `PublishRetryInterval`, `RemoveRetryWindow`, `ClaimWindow`, `DisputeDeadline`, `TopViolationShare`, `MaxImages`, `ViewsRefresh`, `ScheduledGrace`) с `WithDefaults`.
  - `internal/placement/domain/ports.go`: `ChannelPublisher`, `PublicationContent` (R-1), `Publications` (`Insert`, `GetByPlacementForUpdate`, `FindByMessageIDForUpdate`, `Update`, `ListLive`, `ListOverdue`), `Violations` (`Insert`, `ExistsForEvent`), `Disputes` (`Insert`, `GetByPlacement`); `ChannelOffer` получает `MaxChatID`. Ошибки `ErrMessageNotFound`, `ErrPublicationExists`.
  - Журнал: не применимо (`domain` не пишет журнал).
  - Тесты (табличные): каждое предусловие R-3 по отдельности; `PublishRetry` ровно на `publish_at + 30 мин` и на секунду раньше; `remove_at` пересчитывается от фактического `published_at` (опубликовано на 7 с позже — снятие на 7 с позже); `RemoveRetry` — рост задержки и граница 24 ч; P-10 ставит T-05 на `removed_at + 24 ч`; P-14 из `published` и `removed`, из `disputed` — отказ; предложение доли — пример 03 §7 (18 ч из 24 → 7500), `top_violated` → 5000, остальные — пусто; P-09 отменяет снятие; таблица переходов по-прежнему сверяется с 03 §4.
  - Файлы: `internal/placement/domain/{publication.go,publish.go,dispute.go,timers.go,ports.go,errors.go,publication_test.go,publish_test.go,dispute_test.go,timers_test.go}`.

- [x] **Task 4: `placement/domain` — классификация событий и сверки (R-8, R-10, R-12)** (depends on 3)
  - `internal/placement/domain/control.go`:
    - `InTop(p, at) bool` — полуоткрытый `[published_at, published_at + top_hours)` (T-08).
    - `ClassifyChannelPost(live []LivePlacement, post ChannelPost, botUserID) []Violation` — чужое сообщение во время топа одного из размещений канала → `top_violated` с `details.message_id`; своё сообщение и сообщение вне топа — ничего.
    - `ClassifyRemoved(pub, at)` — публикация без намерения → `removed_early`; с намерением — повтор.
    - `ClassifyEdited(pub, hash)` — хеш отличается → `content_modified`.
    - `ReconcileDecision(pub, p, msg *ChannelMessage, found bool, recent []ChannelMessage, botUserID, now, rules) ReconcileResult{Violations, UpdateViews}` по R-12.
  - Журнал: не применимо.
  - Тесты (табличные): сообщение на последней секунде топа — нарушение, ровно на его конце — нет; два размещения канала с соседними топами — нарушение относится к тому, в чей интервал попало; сообщение нашего бота (другая реклама) — не нарушение (03 §8); сообщение без отправителя — нарушение; удаление с намерением — повтор; хеш совпал — ничего; сверка: не найдено, изменено, чужое сообщение в топе, просмотры через 59 и 61 мин.
  - Файлы: `internal/placement/domain/{control.go,control_test.go}`.
<!-- Commit checkpoint: tasks 3-4 -->

### Phase 3: Адаптер, хранение и порты других модулей

- [x] **Task 5: `integration/max` — публикация поста, чтение сообщения, новые события (R-1, R-5, R-6, R-8, R-13, R-16, R-17)**
  - `internal/integration/max/uploads.go`: `UploadImage(ctx, r, size, contentType)` — `POST /uploads?type=image`, затем передача файла по выданному адресу (multipart, таймаут 120 с), токен из ответа; адрес загрузки и токен — в `redact`.
  - `internal/integration/max/messages.go`:
    - `SendChannelPost(ctx, chatID, text, imageTokens)` — `POST /messages?chat_id=`, `format = html`, `disable_link_preview = true`, вложения `image` по порядку, корзина канала; `attachment.not.ready` — повторы внутри вызова 1, 2, 4, 8, 15, 30 с без счёта как неудачной попытки; ответ → `PostedMessage{ID, URL, At, ContentHash}`.
    - `GetMessage(ctx, messageID)` — `GET /messages/{id}`: отправитель, время, хеш, `stat.views`; `404`/`not.found` → `ErrMessageNotFound`.
    - `DeleteMessage` — проверка `success` уже есть; общий метод для каналов и размещений.
    - `ListMessages`: `ChannelMessage` получает `SenderID`, `ContentHash` и внутренний признак «текст содержит подстроку» (`ContainsText(s)`) для поиска erid по R-5; текст не экспортируется.
    - `ContentHash(text, markup, attachments)` — одна функция нормализации (R-6), метка `// Q-06`.
    - `BotUserID(ctx)` — `GET /me` один раз, затем из памяти; `Options.BotUserID` из `MAX_BOT_USER_ID` — без запроса.
  - `internal/integration/max/updates.go`: `message_edited` (сообщение целиком) и `message_removed` (`message_id`, `chat_id`, `user_id`) в `Update`; `MessageID()`, `MessageAt()`, `ContentHash()` события; `ChannelUpdateTypes` дополняется (R-17).
  - Приоритет ограничителя берётся из `context` (`WithPriority`): публикация и снятие — `PriorityPublish`, сверка — `PriorityReconcile`.
  - `internal/integration/max/maxfake`: `UploadImage`, `SendChannelPost` (запоминает пост, время из часов теста), `GetMessage`, `BotUserID`; управление для тестов: `RemoveExternally(messageID)`, `EditExternally(messageID)`, `PostAsHuman(chatID, at)`, `FailSendAfterCreate(err)` (сообщение создано, но ответ — ошибка: неизвестный исход), `FailUploadsWith`, `AttachmentNotReady(n)`.
  - Журнал: DEBUG вызовы (пишет `outbound`), INFO `max post published` (`chat_id`, `message_id`, `images`), INFO `max image uploaded` (`chat_id`), WARN `max attachment not ready, retrying` (`attempt`, `delay`), DEBUG `max message read` (`message_id`, `views`). Текст, токены и адрес загрузки — не пишутся.
  - Тесты (`httptest`, образцы в `testdata`): загрузка → токен; отправка с двумя изображениями — тело запроса по 05 §2.6; `attachment.not.ready` дважды, затем успех — один вызов `SendChannelPost`; `success=false` при удалении → ошибка; `GetMessage` `404` → `ErrMessageNotFound`; хеш одинаков для ответа отправки и чтения того же сообщения и отличается после правки; разбор `message_edited` и `message_removed` из `testdata/updates`; `BotUserID` не делает второй запрос; адрес загрузки и токен не попадают в `outbound_request`.
  - Файлы: `internal/integration/max/{uploads.go,messages.go,updates.go,client.go,uploads_test.go,messages_test.go,updates_test.go}`, `internal/integration/max/testdata/*`, `internal/integration/max/testdata/updates/{message_edited.json,message_removed.json}`, `internal/integration/max/maxfake/{fake.go,channels.go,posts.go}`.

- [x] **Task 6: `placement/postgres` — публикации, нарушения, споры (R-10 … R-12, R-20)** (depends on 2, 3)
  - Запросы `queries/{publications,violations,disputes}.sql`:
    - публикации: вставка, по размещению с `FOR UPDATE`, по `max_message_id` с `FOR UPDATE`, обновление (снятие, намерение, `removal_overdue`, просмотры), живые с курсором `(id)` для сверки, живые канала вместе с размещениями (`published`, `disputed`) для контроля топа, `removal_overdue` старше порога;
    - нарушения: вставка, существует ли по `(placement_id, inbound_event_id)`, существует ли вид по размещению;
    - споры: вставка, по размещению, по списку размещений (для ответов кабинетов).
    - размещения: поля `published_at`, `removed_at`, `claim_deadline_at`, `final_views` в существующем обновлении; `scheduled` без публикации с `publish_at` раньше порога.
  - Репозитории реализуют порты задачи 3; уникальность `publication` → `ErrPublicationExists`, `dispute` → `conflict`.
  - Журнал: DEBUG `publication saved` (`publication_id`, `removed`, `removal_overdue`), DEBUG `violation saved` (`placement_id`, `kind`).
  - Тесты (PostgreSQL): вторая публикация размещения → `ErrPublicationExists`; поиск по `max_message_id`; живые публикации обходятся курсором ровно по разу; живые канала возвращают только `published`/`disputed` без `removed_at`; повтор нарушения по тому же событию не вставляется; список `removal_overdue` по порогу.
  - Файлы: `sqlc.yaml` (без изменений записи модуля, если пути те же), `internal/placement/postgres/{publications.go,violations.go,disputes.go,placements.go,*_test.go}`, `internal/placement/postgres/queries/{publications,violations,disputes,placements}.sql`, `internal/placement/postgres/sqlc/*`.

- [x] **Task 7: Порты в `channel` и `post` (R-1, R-3, R-9, R-16)** (depends on 3)
  - `channel`:
    - `ChannelOffer` / `LockForPlacement` возвращают `MaxChatID` (переходник в `bootstrap`).
    - `channel/domain.ChannelPostListener` и `ChannelPost` (R-9); пустая реализация по умолчанию; `SetPlacementPorts` принимает её.
    - `channel/app.ChannelPostCreated`: код подключения — как сейчас; не код и канал известен → вызов слушателя вне транзакции канала (слушатель открывает свою); неизвестный канал → `ErrIgnored`. `channel/jobs` передаёт `SenderID`, время сообщения и идентификатор события.
  - `post`:
    - `post/domain.VerifyFinalText(v Version) error` — пересборка итогового текста из `body` и параметров маркировки той же функцией, что в `MarkVersionReady`, и сравнение (R-3).
    - `post/app.PublicationContent(ctx, versionID)` — статус, erid, итоговый текст, проверка `VerifyFinalText`, производные по порядку (`derivative_id`, тип, размер); `post/app.OpenDerivative(ctx, derivativeID) (io.ReadCloser, error)` — чтение из закрытой области хранилища (`storage.Get`).
  - Журнал: DEBUG `channel post passed to placements` (`channel_id`, `message_id`), WARN `final text mismatch` (`version_id`) — без текста, DEBUG `derivative opened` (`derivative_id`).
  - Тесты: сообщение с кодом подключения не доходит до слушателя, обычное — доходит с отправителем и временем; сообщение в неизвестном канале — `ignored`; `VerifyFinalText` для каждого сочетания `token_carrier` × `token_position` и с подменённым erid; `OpenDerivative` читает байты из `storagefake`; `LockForPlacement` возвращает `MaxChatID`.
  - Файлы: `internal/channel/{domain/ports.go,app/link.go,app/service.go,app/placement_ports.go,jobs/jobs.go,app/events_test.go}`, `internal/post/{domain/final_text.go,domain/final_text_test.go,app/publication.go,app/publication_test.go}`.
<!-- Commit checkpoint: tasks 5-7 -->

### Phase 4: Сценарии

- [x] **Task 8: Публикация по расписанию — S-21, P-08, P-09 (R-2 … R-5, R-15, R-16, R-19)** (depends on 4, 5, 6, 7)
  - `internal/placement/app/publish.go`: `Publish(ctx, placementID, jobRef{ID, Attempts})` по R-2: шаг 1 (блокировки, выход при повторе, `CheckPublish` → P-09 `precondition_failed`); при `Attempts > 0` — поиск по R-5; шаг 2 (`PublicationContent.Get`, `Open` и `UploadImage` по порядку, `SendPost` с `PriorityPublish`); шаг 3 (`Publications.Insert`, `Apply(P-08)`, `Reschedule` снятия, `Notifier`, метрика задержки). Ошибки — по R-4: повтор с `job.Retry` или P-09 `publish_error` (`Holds.Release`, отмена снятия, оповещение ERROR).
  - `ExpirePublish(ctx, placementID, jobID)` — `OnExpired` политики просрочки: P-09 `publish_error`, `reason_text = downtime`.
  - История: `job_id` и `outbound_request_id` последнего вызова, если известен.
  - Журнал: INFO `placement publish started` (`placement_id`, `channel_id`, `attempt`, `delay`), INFO `placement published` (`placement_id`, `max_message_id`, `published_at`, `remove_at`, `delay_ms`), WARN `placement publish retry` (`attempt`, `retry_after`, `error`), ERROR `placement publish failed` (`reason`, `error_code`), INFO `publication found after unknown outcome` (`max_message_id`), DEBUG `publish job outdated` (`status`).
  - Тесты (PostgreSQL, `clock.Manual`, `maxfake`, `storagefake`): успех с двумя изображениями — публикация, P-08, `remove_at` от фактического времени, задание снятия перенесено, уведомления `placement.published`; повтор задания после успеха — ничего не меняет, второго сообщения нет; канал `suspended` → P-09 `precondition_failed` без вызова MAX, удержание снято; версия с подменённым итоговым текстом → P-09; ошибка `5xx` до 30 мин — повтор, после — P-09 `publish_error` и оповещение; `403` — P-09 сразу; неизвестный исход (`FailSendAfterCreate`) — повтор находит сообщение, второго поста нет; `attachment.not.ready` — успех; просрочка после простоя 31 мин → P-09 без вызова MAX, 29 мин → публикация; 11 изображений → P-09.
  - Файлы: `internal/placement/app/{publish.go,publish_test.go}`, `internal/placement/placementtest/*` (фикстура: размещение в `scheduled` с версией `ready` и производными в `storagefake`).

- [x] **Task 9: Снятие по расписанию — S-22, P-10, T-10 (R-7, R-14, R-15, R-19)** (depends on 8)
  - `internal/placement/app/remove.go`: `Remove(ctx, placementID, jobRef)` по R-7 (пять шагов, `PriorityPublish`); `ErrMessageNotFound` на шаге 2 → `reportViolation(removed_early)` (задача 10); итоговые просмотры в `publication.views` и `placement.final_views`; T-05 по R-14; ошибка удаления → `removal_overdue`, повтор до T-10, затем WARN и оповещение `publication_removal_overdue:{id}`, уведомление `placement.removal_overdue`.
  - `internal/placement/app/timers.go`: ветка T-05 по R-14 (`job.Retry` на 1 ч, DEBUG).
  - Журнал: INFO `placement removal started` (`placement_id`, `max_message_id`, `delay`), INFO `placement removed` (`final_views`, `removed_at`, `claim_deadline_at`, `delay_ms`), WARN `placement removal retry` (`attempt`, `retry_after`, `error`), WARN `publication removal overdue` (`publication_id`, `since`), DEBUG `remove job outdated` (`status`), DEBUG `auto acceptance not implemented yet` (`placement_id`).
  - Тесты (PostgreSQL, `clock.Manual`, `maxfake`): снятие в `published` → `removed`, итоговые просмотры, `claim_deadline_at`, T-05 поставлен; повтор задания — ничего; снятие в `disputed` — сообщение удалено, состояние `disputed`; событие `message_removed` после своего удаления — не нарушение (намерение записано); ошибка удаления → `removal_overdue`, намерение откатано, следующие попытки с ростом задержки, через 24 ч — без повторов и оповещение; сообщения уже нет к моменту снятия → `removed_early` и спор; простой — снятие выполняется сразу, `removed_at` — фактическое время; T-05 не меняет `removed`.
  - Файлы: `internal/placement/app/{remove.go,remove_test.go,timers.go,timers_test.go}`.

- [x] **Task 10: Контроль по событиям и открытие спора — S-25, S-26, S-27, P-14, T-06 (R-8 … R-11, R-19)** (depends on 8)
  - `internal/placement/app/violations.go`: `reportViolation(ctx, placementID, v Violation, src Source)` по R-10 и R-11 (вставка нарушения, P-14, `dispute`, T-06, отмена T-05, `Notifier`, оповещение WARN, метрика); идемпотентность по `inbound_event_id`.
  - `internal/placement/app/events.go`:
    - `ChannelPostCreated(ctx, post)` (реализация `channel/domain.ChannelPostListener`): живые размещения канала, `BotUserID`, `ClassifyChannelPost` → нарушения.
    - `MessageEdited(ctx, ev, messageID, hash)`, `MessageRemoved(ctx, ev, messageID, at)`: публикация по `max_message_id` (нет — `ErrIgnored`), `ClassifyEdited`/`ClassifyRemoved`.
  - `internal/placement/app/channel_events.go`: `published` при потере канала — `reportViolation(bot_access_lost)` вместо WARN-заглушки.
  - `internal/placement/app/timers.go`: T-06 — размещение в `disputed`, спор без решения → WARN `dispute deadline passed`, оповещение `dispute_overdue:{id}`; иначе устарел.
  - `internal/placement/jobs/jobs.go`: обработчики `message_edited`, `message_removed` в `inbound.Dispatcher` (разбор `max.ParseUpdate`, `ErrIgnored` → `inbound.ErrIgnore`).
  - Журнал: WARN `placement violation detected` (`placement_id`, `kind`, `source`, `occurred_at`, `inbound_event_id`), INFO `placement dispute opened` (`placement_id`, `reason`, `suggested_share`, `deadline_at`), WARN `violation recorded, dispute exists` (`placement_id`, `kind`), DEBUG `channel post ignored` (`reason`: `own_bot`, `outside_top`, `no_live_publications`), DEBUG `message event for unknown publication`, WARN `dispute deadline passed` (`placement_id`, `deadline_at`).
  - Тесты (PostgreSQL, `clock.Manual`): чужое сообщение в топе → нарушение, P-14, спор `top_violated` с долей 5000 и T-06; то же после конца топа — ничего; наше сообщение другой рекламы — ничего; повтор того же события — одна запись; `message_removed` чужой → публикация `external`, снятие отменено, спор `removed_early` с долей по формуле; `message_edited` с другим хешем → `content_modified`; второе нарушение при открытом споре — только запись; потеря канала при `published` → `bot_access_lost`; T-06 через 72 ч — оповещение, состояние `disputed`; событие по неизвестному сообщению — `ignored`.
  - Файлы: `internal/placement/app/{violations.go,events.go,channel_events.go,timers.go,violations_test.go,events_test.go,channel_events_test.go}`, `internal/placement/jobs/jobs.go`.
<!-- Commit checkpoint: tasks 8-10 -->

- [x] **Task 11: Сверка публикаций — T-09 (R-12, R-13, R-15)** (depends on 10)
  - `internal/placement/app/reconcile.go`:
    - `ReconcilePublications(ctx)` — периодическое задание: живые публикации страницами, `check_publication` с распределением по интервалу; шаг 5 R-12 (размещения `scheduled` без живого `publish:{id}` — `Enqueue(PublishJob)` и ERROR); INFO с итогами.
    - `CheckPublication(ctx, publicationID, jobID)` — вызовы MAX с приоритетом сверки вне транзакции, решение `ReconcileDecision`, затем `reportViolation` и обновление просмотров в транзакции. Публикация уже снята — DEBUG и выход.
  - Порт `job.Scheduler` получает (или переиспользует) проверку «есть ли живое задание по ключу» — `Exists(ctx, dedupeKey)` в `platform/job`, если её нет.
  - Журнал: INFO `publications reconciliation scheduled` (`publications`, `scheduled_checked`, `recreated`), ERROR `publish job missing, recreated` (`placement_id`, `publish_at`), DEBUG `publication checked` (`publication_id`, `found`, `hash_matches`, `top_checked`, `views`), WARN нарушения — через `reportViolation`.
  - Тесты (PostgreSQL, `clock.Manual`, `maxfake`, `worker.RunOnce`): 3 живые публикации → 3 задания `check_publication` с разным `run_at` внутри 5 мин, повтор периодического не ставит дубли; пропавшее сообщение → `removed_early` (`source = reconciliation`); изменённое → `content_modified`; чужое сообщение в топе, которого не было в событиях (Q-03), → `top_violated`; после конца топа `ListMessages` не вызывается; просмотры обновляются раз в час; `scheduled` без задания через 6 мин после `publish_at` — задание создано, ERROR записан; ошибка MAX при проверке — повтор задания, без нарушения.
  - Файлы: `internal/placement/app/{reconcile.go,reconcile_test.go}`, при необходимости `internal/platform/job/{job.go,queries/*.sql,sqlc/*,job_test.go}`.

- [x] **Task 12: Регистрация заданий, зависшие, метрики (R-4, R-12, R-15)** (depends on 9, 11)
  - `internal/placement/jobs/jobs.go`: `publish_placement` (`PriorityHigh`, `ObjectRef`, `Overdue{After: PUBLISH_RETRY_WINDOW, OnExpired: ExpirePublish}`), `remove_placement` (`PriorityHigh`, `ObjectRef`), `RegisterPeriodic(reconcile_publications, job.Every(RECONCILE_INTERVAL))`, `check_publication`. Комментарий пакета о «до этапа автопостинга задания ждут» удаляется.
  - `internal/placement/app/stuck.go`: проверка `publication_removal_overdue` (дольше `STUCK_REMOVAL_OVERDUE_AGE`, 1 ч, 07 §3) для `platform/stuck`.
  - `internal/placement/app/metrics.go` или `postgres/metrics.go`: гистограммы задержки публикации и снятия, счётчик нарушений (R-15); регистрация в `metrics`.
  - Журнал: INFO `worker started` перечисляет новые виды (пишет `job`); оповещения зависших — `stuck`.
  - Тесты: `wiring`-проверка задачи 14 — виды зарегистрированы с нужным приоритетом и политикой; проверка зависших находит публикацию с `removal_overdue` через 61 мин и не находит через 59; метрика задержки получает значение после публикации (`testutil`).
  - Файлы: `internal/placement/jobs/jobs.go`, `internal/placement/app/{stuck.go,metrics.go,stuck_test.go}`.
<!-- Commit checkpoint: tasks 11-12 -->

### Phase 5: HTTP и сборка

- [x] **Task 13: Публикация и спор в ответах кабинетов (R-18)** (depends on 6, 10)
  - `internal/placement/app/orders.go`, `blogger.go`: чтение публикаций и споров размещений одним запросом на заказ или страницу (без N+1).
  - `internal/placement/httpapi/convert.go`: `publication_url`, `views` по R-18, `dispute` (`PlacementDisputeSummary`); время — UTC.
  - Журнал: не применимо (ошибки — в `app`).
  - Тесты (`httptest` поверх собранного сервера): после публикации `getOrder` и `getBloggerPlacement` отдают ссылку и просмотры; после снятия — итоговые просмотры; после P-14 — сводка спора без предложенной доли; ответы проходят проверку схемой.
  - Файлы: `internal/placement/app/{orders.go,blogger.go}`, `internal/placement/httpapi/{convert.go,handlers_test.go}`.

- [x] **Task 14: Конфигурация и сборка (R-1, R-4, R-7, R-8, R-11, R-12, R-17)** (depends on 12, 13)
  - `internal/bootstrap/config.go`: `PUBLISH_RETRY_WINDOW` (30m, T-03), `PUBLISH_RETRY_INTERVAL` (30s), `PUBLISH_FIND_LOOKBACK` (40m), `PUBLISH_MAX_IMAGES` (10, A-23), `REMOVE_RETRY_WINDOW` (24h, T-10), `PLACEMENT_CLAIM_WINDOW` (24h, T-05), `DISPUTE_DEADLINE` (72h, T-06), `DISPUTE_TOP_VIOLATION_SHARE` (5000), `RECONCILE_INTERVAL` (5m, A-27), `RECONCILE_SCHEDULED_GRACE` (5m), `PUBLICATION_VIEWS_REFRESH` (1h), `STUCK_REMOVAL_OVERDUE_AGE` (1h), `MAX_BOT_USER_ID` (необязательно). Проверки при запуске: `PUBLISH_RETRY_INTERVAL < PUBLISH_RETRY_WINDOW`, доля в `[0, 10000]`, `PUBLISH_FIND_LOOKBACK > PUBLISH_RETRY_WINDOW`.
  - `internal/bootstrap/placement.go`: переходники `ChannelPublisher` (над `maxAPI`, приоритеты ограничителя) и `PublicationContent` (над `post/app`); подстановка `placement/app` в `channel` как `ChannelPostListener`; новые репозитории; `stuck`-проверка `removal_overdue`.
  - `internal/bootstrap/integration.go`: `maxAPI` дополняется методами задачи 5.
  - `internal/bootstrap/channel.go`: `ensureMAXSubscription` по R-17.
  - `internal/bootstrap/worker.go`: регистрация задачи 12; обработчики `message_edited`, `message_removed`.
  - `.env.example`, `compose.dev.yml`: новые переменные. Заметка для `/aif-docs`: в разработке `maxfake` — «Опубликовать сейчас» в Swagger → `worker` публикует и через `duration_hours` снимает (сдвиг сроков — короткими значениями формата не делается; для ручной проверки — `maxim dev post-version` и `mode=now`), события чужих сообщений проверяются тестами.
  - Журнал: конфигурация — маскированной, как сейчас; INFO `max subscription updated` (`added_types`).
  - Тесты: проверки конфигурации на границах; `wiring_test.go`: виды `publish_placement`, `remove_placement`, `check_publication`, `reconcile_publications` зарегистрированы, у публикации — политика просрочки, обработчики `message_edited`/`message_removed` есть в диспетчере, `channel` получил `ChannelPostListener`; подписка без `message_removed` обновляется, с полным набором — нет.
  - Файлы: `internal/bootstrap/{config.go,config_test.go,placement.go,integration.go,channel.go,worker.go,wiring_test.go}`, `.env.example`, `compose.dev.yml`.
<!-- Commit checkpoint: tasks 13-14 -->

### Phase 6: Проверка и документы

- [x] **Task 15: Сквозные тесты автопостинга и контроля** (depends on 14)
  - `internal/bootstrap/autoposting_e2e_test.go` на собранных `api` и `worker` (PostgreSQL, `clock.Manual`, `maxfake`, `storagefake`, `mailfake`, поддельные порты денег):
    - Сквозной сигнал успеха (RESEARCH): заказ двух каналов → `MarkVersionReady` → блогеры принимают; первый — «Опубликовать сейчас», второй — время через 2 ч. `worker.RunOnce`: первый пост в `maxfake` с итоговым текстом и изображением, P-08; сдвиг часов к `publish_at` второго — P-08 не позже 10 с от плана. Сдвиг к `remove_at` — оба P-10, итоговые просмотры в кабинетах.
    - S-27: третье размещение, чужое сообщение через Webhook-приёмник (`POST /webhooks/max/{secret}`, `message_created`) во время топа → `disputed`, `top_violated`; то же после топа другого размещения — без спора.
    - S-26: чужое удаление через Webhook (`message_removed`) → `removed_early`, снятие отменено; правка (`message_edited`) другой публикации → `content_modified`.
    - Сверка без событий (Q-03): `maxfake.RemoveExternally` без Webhook → через `RECONCILE_INTERVAL` спор `removed_early` с `source = reconciliation`.
    - S-21 отклонения: MAX отвечает `5xx` 31 мин → `failed`, удержание снято, оповещение; простой `worker` 40 мин перед `publish_at` → после запуска P-09 и ERROR простоя со списком размещений.
    - S-25: бот лишился прав при `published` → `bot_access_lost`; снятие по сроку не удаётся → `removal_overdue`, оповещение через 24 ч.
  - Проверка журнала сборщиком записей: обязательные события 07 §5.3 этапа (переход выполнен и отклонён, внешняя операция выполнена, повтор, исчерпание, нарушение найдено, задание запущено и завершено) присутствуют; текст поста, текст сообщений, токены вложений, название и ИНН рекламодателя не встречаются.
  - Проверка истории: P-08, P-10, P-14 с актором `system` и источником (`job_id` у заданий, `inbound_event_id` у событий).
  - Файлы: `internal/bootstrap/autoposting_e2e_test.go`, при необходимости `internal/bootstrap/harness_test.go`.

- [x] **Task 16: Синхронизация документов с решениями этапа** (depends on 14)
  - После подтверждения R-1 … R-20 внести их в документы:
    - 02 S-21 — шаги по R-2, предусловия R-3, неизвестный исход R-5, `4xx` без повторов R-4; S-22 — шаги R-7, снятие при пропавшем сообщении; S-26, S-27 — время нарушения и повторное нарушение при открытом споре (R-10); S-25 — `bot_access_lost` через общий путь.
    - 03 §4 — P-08 (`published_at` из ответа MAX), P-09 (причины `precondition_failed`, `publish_error`), P-10 (T-05 по R-14); §5 — T-03 и T-10 как параметры, T-05 до этапа приёмки, T-06 — оповещение; §8 — свой бот по `user_id`, сообщение без отправителя — чужое (R-8), сообщение с кодом подключения (R-9); §10 — формулировка I-01 (R-3).
    - 04 §5 — `publication` (индексы, `content_hash` по R-6), `violation` (уникальность по событию, только вставка), `dispute` (`claim_id` без внешнего ключа до этапа претензий).
    - 05 §2.2 — маршрут `message_created` через порт канала (R-9), подписка на `message_edited`, `message_removed` (R-17); §2.6 — изображения из закрытой области, предел 10 (R-16), хеш содержимого (R-6); §2.8 — `check_publication` и распределение сверки (R-12).
    - 07 §2 — вид `check_publication`, политика просрочки публикации; §3 — `removal_overdue`; §5.5 — имена метрик; §8 — новые параметры (задача 14).
    - `.ai-factory/ARCHITECTURE.md` — порты `ChannelPublisher`, `PublicationContent`, `channel/domain.ChannelPostListener`; пакет `placement/jobs` — новые виды; дерево `placement/postgres` — `publication`, `violation`, `dispute`.
    - `docs/open-questions.md` — Q-03, Q-04, Q-06, Q-07, Q-08: что проверить на тестовом канале для этого этапа (хеш содержимого, свой бот, события сообщений, удаление через 72 ч).
  - Журнал: не применимо.
  - Файлы: `docs/02-scenarios.md`, `docs/03-placement.md`, `docs/04-domain-model.md`, `docs/05-integrations.md`, `docs/07-non-functional.md`, `docs/open-questions.md`, `.ai-factory/ARCHITECTURE.md`.
<!-- Commit checkpoint: tasks 15-16 -->

## Критерии готовности этапа

- `go build ./...`, `go vet ./...`, `go test -race ./...` проходят локально и в CI. `sqlc generate` и `go generate ./...` не дают изменений. `mockapi` собирается и проходит свой контрактный тест.
- Таблица переходов по-прежнему совпадает с 03 §4; P-08, P-09, P-10, P-14 выполняются только через `Placement.Apply`.
- Внешние вызовы MAX выполняются только в обработчиках заданий вне транзакции перехода (I-13); публикация создаётся только при версии `ready` с проверенным итоговым текстом (I-01); у размещения не больше одной публикации и одного спора (I-11).
- Повтор задания публикации, снятия, сверки и повтор события не создаёт второго поста, второй записи истории и второго нарушения (03 §11); неизвестный исход отправки не даёт дубля.
- Публикация и снятие начинаются не позже 10 с после назначенного времени при работающем `worker` (07 §2) — проверено сквозным тестом на `clock.Manual`.
- Чужое сообщение в топе, чужое удаление и правка находятся и событием, и сверкой.
- Текст поста, текст сообщений канала, токены вложений, название и ИНН рекламодателя отсутствуют в журнале и `outbound_request`.
- Решения R-1 … R-20 подтверждены и внесены в документы (задача 16).
