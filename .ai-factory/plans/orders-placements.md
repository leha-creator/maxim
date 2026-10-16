# План реализации: Заказ и размещения

Branch: master (без отдельной ветки, по решению пользователя)
Created: 2026-10-17

## Settings
- Testing: yes. Таблица переходов, правила времени (окно, таймеры, выбор `publish_at`, I-06), расчёт сумм, состояние заказа, доступные действия и правила корзины проверяются чистыми табличными тестами `placement/domain`. Сценарии `placement/app` с `postgres` выполняются на настоящей PostgreSQL через `testcontainers-go` с `clock.Manual` и поддельными портами денег, уведомлений и Рокет Ворк. Операции кабинетов проверяются через `httptest` поверх собранного сервера (07 §9).
- Logging: verbose. `log/slog` JSON, поля и обязательные события по 07 §5 и 03 §12: переход выполнен (INFO) и отклонён (WARN), сработал таймер (INFO), попытка нарушить инвариант (ERROR), повторное событие проигнорировано (DEBUG), оформление заказа, новая версия, действия корзины. Поля `organization_id`, `user_id`, `order_id`, `placement_id`, `channel_id`, `job_id`. Текст поста, комментарий блогера при отказе, название и ИНН рекламодателя в журнал не пишутся (07 §5.4).
- Docs: yes. В конце обязательная точка `/aif-docs`: README бэкенда (новые переменные окружения, как пройти заказ руками в `compose.dev.yml` командой разработчика из задачи 14, какие порты пока работают как заглушки).

## Roadmap Linkage
Milestone: "Заказ и размещения"
Rationale: план реализует этап целиком: корзину, оформление заказа со своими состояниями кампании, одну таблицу переходов P-01 … P-18, таймеры T-01, T-13 и окно `created`, инварианты I-01 … I-13 в части без денег. Деньги — через порт с пустой реализацией до этапа «Приём оплаты и учёт денег».

## Research Context
Source: .ai-factory/RESEARCH.md (Active Summary), выдержка по этапу

- Единица работы — размещение (один канал в заказе), а не заказ: у каждого размещения своя машина состояний и деньги.
- После получения erid пост неизменяем; правка — новая версия и повторная регистрация креатива. erid привязан к версии, а не к размещению: одна версия — один erid на весь заказ.
- Деньги — внутренний учёт «баланс → удержание по размещению → выплата» (A-06, Q-17 — модель баланса принята рабочей).
- Открыто: приходят ли события о постах людей (Q-03, Q-04) — касается контроля топа, не этого этапа.

## Источники ТЗ

- Сценарии: 02 S-03 (корзина), S-06 (оплата кампании, P-01), S-07 (отслеживание и отмена), S-14 (принятие, отказ, планирование), S-17 шаги 3–4 (последствия решения модерации для размещений), S-23 (таймауты), S-25 шаг 2 (размещения при потере канала), S-29 (правка поста после оформления).
- Размещение: 03 целиком — §1 (заказ и его производное состояние), §3 (состояния), §4 (переходы и особые правила), §5 (таймеры и выбор времени), §9 (версии), §10 (инварианты), §11 (идемпотентность), §12 (журналирование).
- Данные: 04 §5 (`cart_item`, `order`, `placement`, `placement_state_history`), §4 (`campaign`: `order_id`, `ordered`, `archived`), §1 (неизменяемые таблицы и строки, `lock_version`), §9 (перечисления и причины переходов).
- Деньги: 06 §2.3 (какие переходы трогают удержание — для вызовов порта), §3 (комиссия и суммы).
- Требования: 07 §2 (`placement_timer`, `publish_placement`, `remove_placement`, `dedupe_key`, приоритет), §3 (зависшее размещение в `scheduled`), §4 «Доступ» (блогер видит пост только после P-02), «Деньги» (суммы только на сервере), §5 (журнал), §5.5 (число размещений по состояниям), §8 (конфигурация).
- Реестр: D-02, D-03, D-07, D-11, D-15, D-16, D-17, D-19, D-20, A-06, A-10, A-11, A-15, A-30, A-36, A-37, A-39, Q-17.
- Архитектура: `.ai-factory/ARCHITECTURE.md` — модуль `placement` (корзина, заказ, размещения), «Одна таблица переходов» (`placement/domain/transitions.go`, `Placement.Apply`), «Порт объявляет потребитель», «Порты к будущим модулям», запрет `UPDATE placement SET status` в обход `domain`.
- Существующий код: `internal/post/app/versions.go` (`CreateVersion` в транзакции вызывающего, `ApproveVersion`, `RejectVersion`, `MarkVersionReady`, `GetVersion`), `internal/post/app/campaigns.go`, `internal/post/domain/campaign.go` (`TodayMSK`, `TimeOfDay`), `internal/channel/domain/ports.go` (`PlacementSchedule`, `AvailabilityListener`), `internal/channel/app/service.go` (`NopPlacements`, `NopAvailability`, `save`), `internal/channel/app/prices.go` (`FeeRateBP`, `TotalAmount`), `internal/channel/domain/price.go` (`TotalAmount`), `internal/channel/httpapi/convert.go` (`in_cart`), `internal/platform/job` (`Scheduler`: `Enqueue`, `Cancel`, `Reschedule`; `HandlerOptions`, `Priority`), `internal/platform/stuck` (`Check`), `internal/bootstrap/admin.go` (образец команды CLI), `mockapi/internal/store/{operations.go,transitions.go}` (поведение тестового сервера — образец, не код для переноса).

## Границы этапа

Входит:
- Корзина: `getCart`, `addCartItem`, `updateCartItem`, `deleteCartItem`; признак `in_cart` в каталоге.
- Оформление: `checkoutCampaign` — полнота кампании, сверка корзины, версия поста 1, заказ, размещения (P-01), вызов порта удержаний, кампания `ordered`.
- Заказы рекламодателя: `listOrders`, `getOrder`, `cancelPlacement` (P-06), новая операция `cancelOrder` (S-07 шаг 3), `createOrderPostVersion` (S-29, P-11).
- Кабинет блогера: `listBloggerPlacements`, `getBloggerPlacement`, `bloggerAcceptPlacement` (P-03), `bloggerDeclinePlacement` (P-04), `bloggerSchedulePlacement` (P-17), `bloggerReschedulePlacement` (P-07).
- Таблица переходов P-01 … P-18 целиком в `domain` с тестами. Сценарии `app` — для P-01 … P-07, P-11, P-17, P-18.
- Последствия смены состояния версии поста: `approved` → заказ `marking`; `ready` → P-02; `rejected` → снятие удержаний (03 §4, особые правила).
- Таймеры: задание `placement_timer` для T-01 (P-05), T-13 (P-18) и окна `created` (P-06 `window_elapsed`, R-11).
- Порты `channel`: `AvailabilityListener` (P-06 `channel_unavailable`), `PlacementSchedule` (занятые интервалы), проверка нового интервала недоступности против размещений (R-8 этапа «Каталог»).
- Инварианты I-04 … I-08, I-10, I-11 (в части размещения), I-13 — кодом, ограничениями и триггерами базы; I-01, I-02, I-03, I-12 — только то, что относится к этому этапу (R-20).
- Метрика «число размещений по состояниям» (07 §5.5), проверка зависших `scheduled` (07 §3).

Не входит (следующие этапы):
- Обработчики `publish_placement` и `remove_placement`, P-08, P-09, P-10, сверка, нарушения, P-14 по событиям: этап «Автопостинг и контроль». Здесь P-17 и P-07 только ставят эти задания (R-12).
- Регистрация креатива и получение erid: этап «Маркировка через Яндекс ОРД». Здесь P-02 запускается вызовом `post/app.MarkVersionReady`, который сделает тот этап.
- Журнал проводок, удержания, баланс: этап «Приём оплаты и учёт денег». Здесь — порт `Holds` с пустой реализацией (R-2).
- Проверка исполнителя Рокет Ворк при P-03: этап «Выплаты блогерам». Здесь — порт с пропускающей реализацией (R-3).
- `acceptPlacement`, `createPlacementClaim`, P-12, P-13, P-15, P-16: этапы «Приёмка, споры и возвраты» и «Выплаты». Операции отвечают `501`.
- Отправка уведомлений: этап «Уведомления и отчёт по кампании». Здесь — порт `Notifier` с пустой реализацией (R-4). `getOrderReport` — тот же этап, `501`.
- Модерация поста администратором, T-07 и оповещение о срочном посте (D-21): этап «API администратора».

## Решения этапа, требующие подтверждения

Документы этих мест не определяют или противоречат друг другу. План реализует указанный вариант. После подтверждения задача 16 вносит его в документы. При отказе меняется только отмеченная задача.

| # | Вопрос | Решение в плане | Задача |
|---|---|---|---|
| R-1 | Где граница модулей при оформлении (кампания и версия — `post`, заказ — `placement`) | Сценарий `checkoutCampaign` принадлежит `placement/app.Checkout`. В одной транзакции: блокировка кампании через порт `placement/domain.Campaigns` (реализация — `post/app`), версия через `post/app.CreateVersion`, перевод кампании в `ordered` через тот же порт. `post` о заказах не знает: поле `campaign.order_id` пишет метод `post/app.MarkCampaignOrdered(ctx, orgID, campaignID, orderID)`. Обработчик `checkoutCampaign` переезжает из `post/httpapi` (сейчас `501`) в `placement/httpapi` | 7, 9, 13 |
| R-2 | Деньги в этапе «без денег» | Порт `placement/domain.Holds`: `HoldAll(ctx, orgID, []HoldRequest) error` — атомарно, пропускает размещения с активным удержанием, при нехватке — `ErrInsufficientBalance{Missing}`; `Release(ctx, placementID, reason) error` — идемпотентно, без активного удержания ничего не делает. Вызовы — в той же транзакции, что и переход (I-10). Ключи идемпотентности для будущего журнала: `hold:{placement_id}:{post_version_id}`, `hold_release:{placement_id}:{post_version_id}`. Реализация до этапа денег — `bootstrap.UnlimitedHolds` (баланс не ограничен, DEBUG `holds skipped, no ledger yet`). В тестах — `placementtest.Holds` в памяти: баланс, активные удержания, проверка вызова внутри транзакции | 5, 9–12, 14 |
| R-3 | Проверка исполнителя Рокет Ворк при P-03 (A-36) | Порт `ContractorGate.CanAccept(ctx, bloggerOrgID) (bool, error)`. До этапа выплат — `bootstrap.AllowAllContractors` (DEBUG). Отказ — `409` `contractor_inactive`. Проверка юрлица (A-36) — внутри будущей реализации | 5, 11, 14 |
| R-4 | Уведомления до модуля уведомлений | Порт `Notifier.Notify(ctx, Notice{Template, OrganizationID, Role, ObjectType, ObjectID, Data})` вызывается в транзакции перехода и только ставит работу (в будущем — задание `send_notification`). До этапа — `bootstrap.NopNotifier` (DEBUG `notification skipped`). Коды шаблонов: `placement.request` (P-02), `placement.accepted` (P-03), `placement.declined` (P-04), `placement.expired` (P-05, P-18), `placement.cancelled` (P-06), `placement.scheduled` (P-17), `placement.rescheduled` (P-07), `placement.returned` (P-11), `order.post_rejected`. В тестах — запись вызовов | 5, 10–12, 14 |
| R-5 | Последствия смены состояния версии поста | Порт `post/domain.VersionListener.VersionChanged(ctx, VersionChange{VersionID, PostID, From, To})` вызывается из `post/app` в транзакции перехода версии (по образцу `channel.AvailabilityListener`). Пустая реализация `post/app.NopVersionListener` по умолчанию; `bootstrap` подставляет `placement/app`. `approved` — пересчёт состояния заказа. `ready` — P-02 для размещений заказа в `created`, привязанных к этой версии. `rejected` — `Holds.Release` для размещений в `created`, состояние заказа `rejected`. Событие по версии, которая не текущая в заказе, игнорируется (DEBUG) | 7, 10 |
| R-6 | Полнота кампании при оплате (A-39 целиком) | Кампания в `draft`; заданы `date_from`, `date_to`, `time_from`, `time_to`; `date_from` не раньше сегодняшнего дня по Москве (D-20); до конца окна больше `PLACEMENT_WINDOW_GUARD` (1 ч). Пост без проблем предпросмотра (`post_not_ready` из `CreateVersion`). Отказ — `409` `campaign_incomplete`, `details`: `missing_fields`, `post_problems`. Кампания не в `draft` — `409` `conflict` | 5, 9 |
| R-7 | Сверка корзины при оплате | Корзина не пуста; каждая позиция: канал `active` и виден в каталоге (D-11), формат активен и включён, текущий итог равен `shown_total_amount`. Иначе `409` `cart_changed`, `details.items` — идентификаторы позиций. Подтверждение изменившейся цены (S-03) — `updateCartItem` с тем же форматом: он обновляет `shown_total_amount`. Строки каналов блокируются в порядке `channel_id` (без взаимных блокировок) | 8, 9 |
| R-8 | Правила корзины | Позиция только для канала, видимого в каталоге (иначе `404`), формата из активных с включённой ценой (иначе `422` `format_unavailable`). Повтор канала — `409` `already_in_cart`. Свой канал (владелец — та же организация) — `422` `own_channel`. Не больше `CART_MAX_ITEMS` (50) позиций — `422` `cart_full`. `getCart`: текущий итог, `price_changed`, `available`; позиции недоступных каналов не удаляются. `in_cart` каталога — через порт `channel/domain.CartLookup` | 5, 8 |
| R-9 | Окно размещения | `window_start` — 00:00 МСК дня `date_from`, `window_end` — 00:00 МСК дня после `date_to` (конец последнего дня). `time_from`/`time_to` — снимок; публикация допустима, когда время суток по Москве в `[time_from, time_to)`. Снимок не меняется при правке кампании (правка периода после оплаты запрещена — `409`, уже в `post`) | 4, 9 |
| R-10 | Сроки T-01 и T-13 | `accept_deadline_at = min(P-02 + PLACEMENT_ACCEPT_TIMEOUT (24 ч), window_end − PLACEMENT_WINDOW_GUARD)`. `schedule_deadline_at = min(P-03 + PLACEMENT_SCHEDULE_TIMEOUT (24 ч), начало последнего допустимого слота)`; последний слот — `time_to − 5 мин` последнего дня окна. Если при P-02 до конца окна не больше `PLACEMENT_WINDOW_GUARD` — вместо P-02 P-06 `window_elapsed`; если при P-02 канал не `active` — P-06 `channel_unavailable` | 4, 10 |
| R-11 | Окно истекло, пока размещение в `created` (03 §4, особые правила) | Отдельный таймер окна (новый номер T-14 в 03 §5): задание `placement_timer` на `window_end − PLACEMENT_WINDOW_GUARD`, ставится при P-01 и P-11, `dedupe_key timer:{placement_id}:T-14`. Срабатывает только в `created` → P-06 `window_elapsed`; в других состояниях — DEBUG и выход | 4, 9, 12 |
| R-12 | Задания публикации до этапа автопостинга | P-17 ставит `publish_placement` (`run_at = publish_at`, `dedupe_key publish:{placement_id}`) и `remove_placement` (`run_at = remove_at`, `remove:{placement_id}`), полезная нагрузка — `placement_id`. Обработчиков в этом этапе нет: `worker` берёт только зарегистрированные виды, задания ждут. P-07 переносит их `Reschedule`; P-06 и P-11 отменяют `Cancel`. Проверка зависших `scheduled` (35 мин) регистрируется сейчас | 4, 11, 12 |
| R-13 | Выбор `publish_at` (03 §5, I-06, I-07) | `mode=at`: внутри окна и периода времени суток, не раньше `now + PLACEMENT_MIN_LEAD` (1 ч), кратно `PLACEMENT_SLOT_STEP` (5 мин), не в интервале недоступности канала, интервал топа `[publish_at, publish_at + top_hours)` не пересекается с топом других размещений канала в `scheduled` и `published`. `mode=now`: текущий момент внутри окна и периода времени, проверки недоступности и I-06 — от текущего момента; правила часа и кратности не применяются. Причины `422`: `publish_at_required`, `outside_window`, `outside_time`, `too_soon`, `not_aligned`, `channel_unavailable_interval`, `slot_taken`. `remove_at = publish_at + duration_hours`. Интервал топа полуоткрытый: стык двух топов допустим | 4, 11 |
| R-14 | Как держать I-06 при одновременных запросах | Проверка — в транзакции под блокировкой строки канала: порт `placement/domain.Channels.LockForPlacement(ctx, channelID)` (реализация — `channel/postgres`, `SELECT … FOR UPDATE`) возвращает снимок состояния и владельца — заодно проверка I-08. Ограничения исключения (`btree_gist`) не вводится: все записи `scheduled` идут через `placement/app` под этой блокировкой | 7, 11 |
| R-15 | Правка поста после оформления (S-29) | Доступна, когда заказ в `rejected` или `in_progress`. В `moderation` и `marking` — `409` `post_version_in_progress`: версия на модерации или регистрации не заменяется, иначе прежняя осталась бы на модерации без заказа (уточнение S-29: там `moderation` в предусловиях). Нет размещений в `created`, `awaiting_blogger`, `awaiting_schedule`, `scheduled` — `409` `nothing_to_update`. В транзакции: `CreateVersion` (номер заказа тот же), P-11 для `awaiting_*` и `scheduled`, все `created` привязываются к новой версии; размещения, у которых до конца окна меньше `PLACEMENT_REEDIT_GUARD` (24 ч), — P-06 `window_elapsed` (S-29, отклонения); `Holds.HoldAll` для `created` — при нехватке `409` `insufficient_balance`, ничего не сохраняется; `order.current_post_version_id` меняется. Удержания при P-11 сохраняются | 10 |
| R-16 | Видимость для блогера (07 §4) | Размещение видно блогеру, если хоть раз было отправлено ему: новое поле `placement.sent_to_blogger_at` (первый P-02). Пост (`BloggerPost`) показывается, только пока версия размещения в `ready`, а состояние не `created`. После P-11 блогер видит размещение в `created` без поста и без действий | 2, 11 |
| R-17 | Отмена всего заказа (S-07 шаг 3) | Новая операция `POST /orders/{id}/cancel` (`cancelOrder`): P-06 `advertiser_cancel` для размещений в `created` и `awaiting_blogger`, остальные не меняются. Ответ — заказ. Нечего отменять — `409` `nothing_to_cancel` | 1, 11 |
| R-18 | Ключ идемпотентности действий пользователя (03 §11) | Необязательный заголовок `Idempotency-Key` (до 64 символов) у `checkoutCampaign`, `cancelPlacement`, `cancelOrder`, `bloggerAcceptPlacement`, `bloggerDeclinePlacement`, `bloggerSchedulePlacement`, `bloggerReschedulePlacement`, `createOrderPostVersion`. Ключ хранится в `placement_state_history.idempotency_key` (уникален в паре с `placement_id`) и в `order.idempotency_key` (уникален в паре с `organization_id`). Повтор с тем же ключом возвращает текущее состояние объекта, `200`/`201`, без нового перехода. Без ключа повтор даёт `409 invalid_transition` с фактическим состоянием | 1, 2, 9, 11 |
| R-19 | Кампания после оформления | `ordered` — при оформлении, поле `order_id`. `archived` — когда заказ переходит в `completed` или `cancelled` (порт `Campaigns.Archive`). В `archived` черновик поста не сохраняется (`409`), название меняется. Новый заказ из кампании в первой версии не оформляется (D-16) | 7, 10, 11 |
| R-20 | Инварианты «без денег» | I-04 — `domain` и триггер `placement_terminal_guard` (запрет смены состояния из конечного). I-05 — триггер `placement_amounts_immutable`. I-06, I-07 — R-13, R-14. I-08 — проверка состояния канала под блокировкой. I-10 — история и задания в транзакции перехода; `placement_state_history` — только вставка (права роли). I-11 — уникальность пары `(order_id, channel_id)` и `placement_id` в будущих `publication`, `dispute` (здесь — только размещение). I-13 — переходы ставят задания. I-01, I-02, I-12 — проверки появятся в этапах автопостинга, приёмки и денег; здесь `domain` не даёт P-02 без версии `ready`. I-03 — договор порта `Holds` (R-2). Нарушение, пойманное базой, — ERROR `placement invariant violated` и `500` | 2, 3, 6 |
| R-21 | Номер заказа и имя таблицы | Таблица `"order"` (имя из 04, в SQL в кавычках). Номер — последовательность `order_number_seq` с 1001, берётся до `CreateVersion` (UTM-метки, R-12 этапа «Медиатека и пост») | 2, 9 |
| R-22 | Доступные действия в ответах | `available_actions` содержит только действия, операции которых реализованы: рекламодатель — `cancel`; блогер — `accept`, `decline`, `schedule`, `reschedule` (по R-13, `reschedule` — пока до `publish_at` больше 1 ч). `accept` и `claim` рекламодателя добавит этап приёмки | 4, 13 |
| R-23 | Ручной проход в разработке | Команда `maxim dev post-version --order <номер> approve|ready|reject [--erid X] [--reason Y]` вызывает `post/app.ApproveVersion`, `MarkVersionReady`, `RejectVersion` и через R-5 двигает размещения. Работает только при `APP_ENV` `development` или `test`, иначе ошибка запуска. Пишет `audit_log` с актором `system` | 14 |

## Commit Plan
- **Commit 1** (после задач 1–2): `feat(placement): orders api contract and schema`
- **Commit 2** (после задач 3–5): `feat(placement): transitions, timing rules and checkout domain`
- **Commit 3** (после задач 6–7): `feat(placement): storage and cross-module ports`
- **Commit 4** (после задач 8–10): `feat(placement): cart, checkout and post version reactions`
- **Commit 5** (после задач 11–12): `feat(placement): advertiser and blogger actions, timers`
- **Commit 6** (после задач 13–14): `feat(placement): cabinet API and wiring`
- **Commit 7** (после задач 15–16): `test(placement): order e2e, docs sync`

## Tasks

### Phase 1: Договор и данные

- [x] **Task 1: Спецификация корзины, заказов и размещений (R-6 … R-8, R-13, R-15, R-17, R-18, R-22)**
  - `api/openapi.yaml`:
    - Параметр `IdempotencyKey` (заголовок `Idempotency-Key`, строка до 64) у операций R-18; описание повтора.
    - `addCartItem`: `404` для канала вне каталога, `409` `already_in_cart`, `422` с причинами `format_unavailable`, `own_channel`, `cart_full`. `updateCartItem`: подтверждение цены тем же форматом (R-7).
    - `checkoutCampaign`: коды `409` из R-6, R-7 и `insufficient_balance` с `details.missing_amount`.
    - Новая операция `cancelOrder` (`POST /orders/{id}/cancel`, тег `orders`, ответ `Order`, `409` `nothing_to_cancel`).
    - `createOrderPostVersion`: `409` `post_version_in_progress`, `nothing_to_update`, `insufficient_balance`, `campaign_incomplete` (R-15).
    - `bloggerSchedulePlacement`, `bloggerReschedulePlacement`: причины `422` из R-13. `bloggerAcceptPlacement`: `409` `contractor_inactive`, `channel_not_active`.
    - `AdvertiserPlacement.available_actions`, `BloggerPlacement.available_actions`: описать R-22. `BloggerPlacement.post`: только при версии в `ready` (R-16).
  - `go generate ./internal/apigen`. `mockapi` собирается из той же спецификации: реализовать в нём `cancelOrder` через существующий `CancelPlacement` по размещениям заказа, чтобы `mockapi` собирался и проходил контрактный тест.
  - `internal/bootstrap/unimplemented.go` перегенерировать (`genstub`): новая операция отвечает `501` до задачи 13.
  - Журнал: не применимо.
  - Тесты: существующие тесты `bootstrap` и `mockapi` (`contract_test.go`) проходят.
  - Файлы: `api/openapi.yaml`, `internal/apigen/api.gen.go`, `internal/bootstrap/unimplemented.go`, `mockapi/internal/api/api.gen.go`, `mockapi/internal/handlers/orders.go` (обработчик `cancelOrder`).

- [x] **Task 2: Миграция корзины, заказов и размещений (R-16, R-18, R-20, R-21)**
  - `migrations/00007_orders_placements.sql` по 04 §5:
    - `cart_item` (`id`, `organization_id`, `channel_id`, `format_id` → `placement_format`, `shown_total_amount bigint check (> 0)`, `created_by`, `created_at`, `updated_at`; уникальна пара `(organization_id, channel_id)`).
    - `order_number_seq` с 1001; `"order"` (поля 04 §5, `status check` по 04 §9, `total_amount >= 0`, уникален `number`, уникален `campaign_id` — один заказ на кампанию в первой версии, `idempotency_key text`, уникальна пара `(organization_id, idempotency_key)`, индекс `(organization_id, created_at DESC, id DESC)`).
    - `placement` (поля 04 §5, `sent_to_blogger_at`, `status check`, `status_reason`, `lock_version`; уникальна пара `(order_id, channel_id)`; `check`: `total_amount = blogger_amount + fee_amount`, `fee_rate_bp between 0 and 10000`, `window_start < window_end`, `time_from < time_to`, `performance_share between 0 and 10000`; индексы `(channel_id, status)`, `(status, accept_deadline_at)`, `(order_id)`, `(blogger_organization_id, created_at DESC, id DESC) WHERE sent_to_blogger_at IS NOT NULL`).
    - `placement_state_history` (поля 03 §12, `actor_type check`, `idempotency_key`, уникальна пара `(placement_id, idempotency_key)`, индекс `(placement_id, created_at)`).
    - `campaign.order_id uuid references "order"` (R-1).
    - Триггеры `placement_terminal_guard` и `placement_amounts_immutable` (R-20).
  - Права роли: через `ALTER DEFAULT PRIVILEGES` из `00002`; для `placement_state_history` — `REVOKE UPDATE, DELETE` у роли приложения (04 §1).
  - Журнал: не применимо (INFO `migrations applied` пишет `migrate`).
  - Тесты: `up` → `down` → `up`. Роль приложения читает и пишет новые таблицы, но не меняет и не удаляет `placement_state_history`. Смена `blogger_amount` не проходит. Выход из `cancelled` не проходит. Вторая позиция того же канала в корзине организации и второй заказ кампании не вставляются. Номер заказа начинается с 1001.
  - Файлы: `migrations/00007_orders_placements.sql`.
<!-- Commit checkpoint: tasks 1-2 -->

### Phase 2: Предметные правила

- [x] **Task 3: `placement/domain` — состояния, таблица переходов, `Apply` (R-20)**
  - `internal/placement/domain/status.go`: `Status` и значения 04 §9, `Terminal()`, `ActorType`, коды причин 04 §9 (`ReasonAdvertiserCancel` …), `OrderStatus`.
  - `internal/placement/domain/transitions.go`: коды `P01Create` … `P18ScheduleTimeout` (имена по глоссарию), единственная таблица `transitions` (из, в, допустимые акторы) строго по 03 §4, включая P-08 … P-16. `(*Placement).Apply(code, actor, reason, now) (StateChange, error)`: проверка состояния и актора, `apperr.InvalidTransition` с фактическим состоянием, смена `status`, `status_changed_at`, `status_reason`, `lock_version++`. `StateChange{PlacementID, Code, From, To, Actor, UserID, Reason, ReasonText, At, Jobs []job-описания, CancelJobs []string, Notices}` — задания и уведомления как значения, не вызовы.
  - Особые правила: P-06 рекламодателем — только из `created`, `awaiting_blogger`; P-02 требует версию в `ready` (I-01 в части этапа).
  - Журнал: не применимо (`domain` не пишет журнал).
  - Тесты (табличные): каждый переход × каждое состояние × каждый актор — допустим ровно там, где в 03 §4; выход из конечных невозможен (I-04); тест сверяет таблицу с матрицей, переписанной из 03 §4, и перечисления с 04 §9 (обязателен по ARCHITECTURE).
  - Файлы: `internal/placement/domain/{status.go,transitions.go,placement.go,status_test.go,transitions_test.go}`.

- [x] **Task 4: `placement/domain` — окно, таймеры, выбор времени, заказ, действия (R-9 … R-13, R-22)** (depends on 3)
  - `internal/placement/domain/window.go`: `NewWindow(dateFrom, dateTo, timeFrom, timeTo)` по R-9; `InTimeOfDay(t)`, `Contains(t)`, `LastSlot(step)`.
  - `internal/placement/domain/timers.go`: `TimerCode` (`T-01`, `T-13`, `T-14`), `AcceptDeadline`, `ScheduleDeadline` по R-10; `TimerJob(placementID, code, runAt, expected Status)` — описание задания `placement_timer` с `dedupe_key timer:{id}:{code}`; описания `publish_placement`/`remove_placement` (R-12).
  - `internal/placement/domain/schedule.go`: `CheckPublishAt(p, mode, at, now, busy []Interval, unavailable []Interval, rules) (publishAt, removeAt, error)` по R-13 с `RuleError{Reason}`; `CanReschedule(p, now)`; `TopInterval(p)`.
  - Методы сценариев на `Placement`, собирающие `StateChange` с заданиями: `SendToBlogger(now, cfg)` (P-02 + T-01 + `sent_to_blogger_at`), `Accept` (P-03, отмена T-01, T-13), `Decline`, `Schedule` (P-17, отмена T-13, задания R-12), `Reschedule` (P-07), `Cancel(actor, reason)` (P-06, отмена всех таймеров и заданий), `ReturnToCreated(newVersionID)` (P-11, очистка `publish_at`, `remove_at`, дедлайнов, T-14), `Expire(timer)` (P-05/P-18).
  - `internal/placement/domain/order.go`: `DeriveOrderStatus(statuses []Status, version VersionStatus) OrderStatus` по 03 §1.2; `AdvertiserActions(p)`, `BloggerActions(p, now)` по R-22.
  - Журнал: не применимо.
  - Тесты (табличные): окно на границах суток МСК (23:59, 00:00), `time_to` не входит; T-01 — 24 ч и обрезка окном; T-13 — обрезка последним слотом; каждая причина R-13 на границах (ровно 1 ч и на минуту меньше, 09:05 и 09:07, стык двух топов допустим, пересечение на минуту — нет); `mode=now` вне периода времени — отказ; `remove_at` по формату; P-07 при 61 и 59 мин до `publish_at`; P-11 очищает время и меняет версию; состояние заказа — все строки 03 §1.2, включая «все отменены» и «все конечные»; действия по каждому состоянию.
  - Файлы: `internal/placement/domain/{window.go,timers.go,schedule.go,order.go,window_test.go,timers_test.go,schedule_test.go,order_test.go}`.

- [x] **Task 5: `placement/domain` — корзина, оформление, суммы, порты (R-2 … R-8)** (depends on 3)
  - `internal/placement/domain/cart.go`: `CartItem`, `ChannelOffer` (состояние, видимость в каталоге, владелец, цены по форматам, `ChannelRef`), `EvaluateCart(items, offers, feeRateBP) []CartLine` (текущий итог, `price_changed`, `available`), `CheckAdd(item, offer, orgID, count, limit) error` по R-8.
  - `internal/placement/domain/checkout.go`: `CampaignSnapshot`, `CheckCampaign(c, now, guard) error` по R-6 (`IncompleteError{MissingFields}`), `CheckCartForCheckout(lines) error` (`CartChangedError{Items}`), `NewPlacement(id, orderID, offer, format, window, version, feeRateBP, now)` — суммы по 06 §3 через `money.Kopecks.ShareBP`, снимок окна и формата.
  - `internal/placement/domain/ports.go`: `Carts`, `Orders`, `Placements` (с `UpdateIfStatus` — обновление «если состояние равно ожидаемому», 03 §11), `History`; `Channels` (`Offers`, `LockForPlacement`, `Unavailability`), `Campaigns` (`LockForCheckout`, `MarkOrdered`, `Archive`), `PostVersions` (`Create`, `Get`, `Status`), `Holds` (R-2), `ContractorGate` (R-3), `Notifier` (R-4). Ошибки `ErrInsufficientBalance{Missing}`, `ErrStaleVersion`.
  - Журнал: не применимо.
  - Тесты (табличные): суммы 06 §3 (10 000,00 ₽ при 1500 → 1 500,00 и 11 500,00; округление вниз на нечётных копейках); каждая причина R-8; каждое поле R-6 по отдельности; позиция с изменившейся ценой и с выключенным форматом → `cart_changed`; пустая корзина → `cart_changed`.
  - Файлы: `internal/placement/domain/{cart.go,checkout.go,ports.go,errors.go,cart_test.go,checkout_test.go}`.
<!-- Commit checkpoint: tasks 3-5 -->

### Phase 3: Хранение и порты других модулей

- [x] **Task 6: `placement/postgres` — корзина, заказы, размещения, история (R-18, R-20)** (depends on 2, 5)
  - `sqlc.yaml`: запись модуля `placement`. Запросы `queries/*.sql`:
    - корзина: список организации, вставка, смена формата и `shown_total_amount`, удаление своей позиции, очистка, число позиций, набор `channel_id` в корзине (для `in_cart`);
    - заказы: следующий номер, вставка, получение своего (и `FOR UPDATE`), список с фильтром `status` и курсором `(created_at, id)`, поиск по `idempotency_key`, смена состояния и текущей версии, поиск по `current_post_version_id`;
    - размещения: вставка, получение своего (рекламодатель — по `order.organization_id`, блогер — по `blogger_organization_id`) с `FOR UPDATE`, обновление «если `status` и `lock_version` равны ожидаемым», список заказа, список блогера с фильтрами `status`, `channel_id` и курсором, занятые интервалы канала (`scheduled`, `published`), число по состояниям (метрика);
    - история: вставка, список по размещению, поиск по `(placement_id, idempotency_key)`.
  - Репозитории реализуют порты `domain`. Ошибки триггеров R-20 и нарушения уникальности переводятся в `apperr` (`invariant_violated` → `500`, `conflict` → `409`).
  - Журнал: DEBUG `placement saved` (`placement_id`, `status`, `lock_version`), ERROR `placement invariant violated` (`placement_id`, `constraint`).
  - Тесты (PostgreSQL): обновление с устаревшим `lock_version` → `ErrStaleVersion`; курсоры заказов и размещений блогера обходят записи ровно по разу при одинаковом `created_at`; рекламодатель другой организации не получает размещение (`not_found`); блогер не видит размещение до `sent_to_blogger_at`; занятые интервалы возвращают только `scheduled` и `published`; повтор `idempotency_key` находит прежнюю запись.
  - Файлы: `sqlc.yaml`, `internal/placement/postgres/{store.go,carts.go,orders.go,placements.go,history.go,*_test.go}`, `internal/placement/postgres/queries/{carts,orders,placements,history}.sql`, `internal/placement/postgres/sqlc/*`.

- [x] **Task 7: Порты в `channel` и `post` (R-1, R-5, R-8, R-14, R-19)** (depends on 2)
  - `channel`:
    - `channel/postgres`: `OffersForPlacement(ctx, channelIDs)` — состояние, видимость в каталоге (`active`, РКН по D-11), владелец, цены и форматы, `ChannelRef`; `LockForPlacement(ctx, id)` — то же с `FOR UPDATE`. Методы сервиса `channel/app` с теми же сигнатурами для переходника в `bootstrap`.
    - `channel/domain.CartLookup.InCart(ctx, orgID, channelIDs) (map[uuid.UUID]bool, error)` с пустой реализацией по умолчанию; `channel/httpapi` заполняет `in_cart` каталога и карточки.
    - `channel/app.AddUnavailability`: интервал, пересекающий `[publish_at, remove_at)` размещения в `scheduled`, — `422` `placement_conflict` (порт `PlacementSchedule`, R-8 этапа «Каталог»).
  - `post`:
    - `post/domain.VersionListener` и `VersionChange`; `post/app.transition` вызывает его в транзакции после сохранения версии (R-5); `NopVersionListener` по умолчанию.
    - `post/app.LockCampaignForCheckout(ctx, orgID, campaignID)` (`FOR UPDATE`, снимок полей кампании), `MarkCampaignOrdered(ctx, orgID, campaignID, orderID)` (`draft` → `ordered`, `audit_log` `campaign.ordered`), `ArchiveCampaign(ctx, campaignID)` (`ordered` → `archived`, `audit_log` `campaign.archived`). `SaveDraft` в `archived` — `409` (R-19).
    - `post/postgres`: запросы под эти методы; `domain.Campaign` получает `OrderID`.
  - Журнал: INFO `campaign status changed` (`campaign_id`, `from`, `to`), DEBUG `post version listener notified` (`version_id`, `to`), WARN `unavailability conflicts with placement` (`channel_id`).
  - Тесты: `LockForPlacement` блокирует строку (второй вызов ждёт до конца первой транзакции); канал от 10 000 подписчиков без РКН — не виден; `in_cart` в каталоге с поддельным `CartLookup`; интервал недоступности поверх запланированного размещения → `422`; слушатель версии вызывается в транзакции перехода и его ошибка откатывает переход; `MarkCampaignOrdered` из `ordered` → `409`; сохранение черновика архивной кампании → `409`.
  - Файлы: `internal/channel/{domain/ports.go,app/service.go,app/placement_ports.go,app/unavailability.go,postgres/placement.go,postgres/queries/placement.sql,httpapi/convert.go}`, `internal/post/{domain/ports.go,domain/campaign.go,app/versions.go,app/campaigns.go,app/draft.go,postgres/campaigns.go,postgres/queries/campaigns.sql}` и тесты к ним.
<!-- Commit checkpoint: tasks 6-7 -->

### Phase 4: Сценарии

- [x] **Task 8: Корзина (R-7, R-8)** (depends on 6, 7)
  - `internal/placement/app/cart.go`: `GetCart(ctx, orgID)`, `AddCartItem(ctx, orgID, userID, channelID, formatID)`, `UpdateCartItem` (смена формата или подтверждение цены), `DeleteCartItem`; `InCart` для порта каталога. Итог корзины — сумма текущих итогов доступных позиций. `audit_log`: `cart.item_added`, `cart.item_updated`, `cart.item_removed`.
  - `internal/placement/app/service.go`: `Deps` (репозитории, порты, `tx.Runner`, `job.Scheduler`, `audit`, `clock`, `Log`, `Config`), `New`.
  - Журнал: INFO `cart item added` (`channel_id`, `format_id`), INFO `cart item updated` (`channel_id`, `price_confirmed`), INFO `cart item removed`, WARN `cart item rejected` (`channel_id`, `reason`).
  - Тесты (PostgreSQL, `clock.Manual`): добавление → итог с комиссией; повтор канала → `409`; свой канал → `422 own_channel`; 51-я позиция → `422 cart_full`; блогер меняет цену → `price_changed = true`, `updateCartItem` тем же форматом снимает признак; канал ушёл в `suspended` → `available = false`, позиция остаётся; чужая позиция → `404`.
  - Файлы: `internal/placement/app/{service.go,cart.go,cart_test.go,app_test.go}`, `internal/placement/placementtest/*` (поддельные `Holds`, `Notifier`, `ContractorGate`, фикстуры рекламодателя, блогера, канала с ценами).

- [x] **Task 9: Оформление заказа и чтение заказов (R-1, R-2, R-6, R-7, R-9, R-11, R-18, R-21)** (depends on 8)
  - `internal/placement/app/checkout.go`: `Checkout(ctx, orgID, userID, campaignID, idemKey)` в одной транзакции: повтор по `idemKey` → прежний заказ; `LockCampaignForCheckout`; `CheckCampaign`; корзина и `LockForPlacement` каналов по порядку `channel_id`; `CheckCartForCheckout`; номер заказа; `PostVersions.Create` (`post_not_ready` → `campaign_incomplete` с проблемами); вставка заказа и размещений, `Apply(P-01)`, история, T-14 (R-11); `Holds.HoldAll` (нехватка → `409 insufficient_balance`, откат всего); очистка корзины; `MarkCampaignOrdered`; состояние заказа `moderation`; `audit_log` `order.created`.
  - `internal/placement/app/orders.go`: `ListOrders(ctx, orgID, status, cursor, limit)` (счётчики по состояниям), `GetOrder(ctx, orgID, id)` — размещения с историей, действиями (R-22), текущая версия через `PostVersions.Get`.
  - `internal/placement/app/transition.go`: общий `apply(ctx, op, p, change, idemKey)` — `UpdateIfStatus`, история, `Enqueue`/`Cancel`/`Reschedule` заданий из `StateChange`, `Notifier`, пересчёт состояния заказа, журнал. Используют задачи 9–12.
  - Журнал: INFO `order created` (`order_id`, `order_number`, `placements`, `total_amount`), WARN `checkout rejected` (`campaign_id`, `reason`, `items`/`missing_fields`/`missing_amount`), INFO `placement transition applied` (`placement_id`, `transition`, `from`, `to`, `actor`, `reason`), WARN `placement transition rejected` (`placement_id`, `transition`, `status`, `reason`), INFO `order status changed` (`order_id`, `from`, `to`), DEBUG `idempotent repeat` (`key_hash`).
  - Тесты (PostgreSQL, `clock.Manual`, поддельные порты): успешное оформление трёх каналов — версия 1 с `utm_campaign=order_1001`, заказ `moderation`, три размещения `created` с суммами и снимком окна, история P-01, задания T-14, корзина пуста, кампания `ordered`; повтор с тем же ключом — тот же заказ, второй не создан; нет дат → `campaign_incomplete` с `missing_fields`; пост с проблемой → `campaign_incomplete` с `post_problems`; цена изменилась → `cart_changed`, ничего не создано; не хватает баланса → `insufficient_balance` с `missing_amount`, ничего не создано; кампания уже `ordered` → `409`; два параллельных оформления одной кампании — один заказ; список заказов и курсор.
  - Файлы: `internal/placement/app/{checkout.go,orders.go,transition.go,checkout_test.go,orders_test.go}`.

- [x] **Task 10: Реакция на версию поста и правка после оформления (R-2, R-4, R-5, R-10, R-15, R-19)** (depends on 9)
  - `internal/placement/app/versions.go`:
    - `VersionChanged(ctx, change)` (реализация `post/domain.VersionListener`): заказ по `current_post_version_id` (нет или версия не текущая — DEBUG и выход); `approved` → пересчёт (`marking`); `ready` → для каждого `created` с этой версией: `LockForPlacement`, P-02 (`SendToBlogger`) либо P-06 по R-10; `rejected` → `Holds.Release` для `created`, уведомление `order.post_rejected`; пересчёт состояния заказа.
    - `CreateOrderPostVersion(ctx, orgID, userID, orderID, idemKey)` по R-15.
    - Пересчёт состояния заказа в конце каждого перехода; `completed`/`cancelled` → `Campaigns.Archive` (R-19).
  - Журнал: INFO `post version change applied` (`order_id`, `version_id`, `to`, `placements_sent`, `placements_cancelled`), INFO `holds released on rejection` (`order_id`, `count`), INFO `order post version created` (`order_id`, `version_id`, `number`, `returned`, `cancelled`), WARN `post version update rejected` (`order_id`, `reason`), DEBUG `version change ignored` (`version_id`, `reason`).
  - Тесты (PostgreSQL, `clock.Manual`): `ApproveVersion` → заказ `marking`; `MarkVersionReady` → размещения `awaiting_blogger` с `accept_deadline_at`, T-01 поставлен, уведомления `placement.request`, заказ `in_progress`; до конца окна 40 мин → P-06 `window_elapsed`; канал `suspended` → P-06 `channel_unavailable`; отклонение → удержания сняты, заказ `rejected`; новая версия после отклонения → удержания снова, `created` на новой версии, заказ `moderation`; нехватка баланса при новой версии → `409`, версия не создана; новая версия при `in_progress` → `awaiting_blogger` и `scheduled` уходят в `created` (P-11), задания публикации отменены; при `moderation` → `409 post_version_in_progress`; размещение с окном через 20 ч → P-06; повтор события той же версии ничего не меняет; все размещения отменены → кампания `archived`.
  - Файлы: `internal/placement/app/{versions.go,versions_test.go}`.

- [x] **Task 11: Действия рекламодателя и блогера (R-3, R-12 … R-14, R-16 … R-18, R-22)** (depends on 9)
  - `internal/placement/app/advertiser.go`: `CancelPlacement(ctx, orgID, userID, id, idemKey)` (P-06 `advertiser_cancel`, `Holds.Release`), `CancelOrder` (R-17).
  - `internal/placement/app/blogger.go`: `ListBloggerPlacements`, `GetBloggerPlacement` (пост по R-16 через `PostVersions.Get`, `extra_requirements` кампании), `Accept` (P-03: `LockForPlacement` → канал `active`, `ContractorGate`), `Decline(comment)` (P-04, `Holds.Release`; комментарий — в `reason_text` истории, не в журнал), `Schedule(mode, at)` (P-17: блокировка канала, занятые интервалы, недоступность канала, `CheckPublishAt`), `Reschedule(at)` (P-07). `audit_log`: `placement.accept`, `placement.decline`, `placement.schedule`, `placement.reschedule`, `placement.cancel`.
  - Журнал: переходы — общим `apply` (задача 9); WARN `placement schedule rejected` (`placement_id`, `reason`), WARN `placement accept blocked` (`placement_id`, `reason`: `contractor_inactive`, `channel_not_active`).
  - Тесты (PostgreSQL, `clock.Manual`): отмена из `created` и `awaiting_blogger` — `cancelled`, удержание снято, T-01 и T-14 отменены; из `awaiting_schedule` → `409`; `cancelOrder` отменяет только допустимые; повтор отмены с тем же ключом → `200` без второй записи истории, без ключа → `409`; принятие при `suspended` канале → `409`; отказ с комментарием; планирование на каждую причину R-13; два блогерских размещения одного канала с пересекающимся топом параллельно — одно `scheduled`, второе `422 slot_taken`; `mode=now` → `publish_placement` с `run_at = now`; перенос переносит оба задания; перенос за 50 мин → `409`; чужой блогер → `404`; блогер не видит размещение до P-02 и пост после P-11.
  - Файлы: `internal/placement/app/{advertiser.go,blogger.go,advertiser_test.go,blogger_test.go}`.

- [x] **Task 12: Таймеры, доступность канала, занятые интервалы, зависшие, метрика (R-11, R-12, R-20)** (depends on 9)
  - `internal/placement/app/timers.go`: `HandleTimer(ctx, payload{placement_id, timer, expected_status})`: состояние не то — DEBUG `placement timer outdated` и выход; T-01 → P-05, T-13 → P-18, T-14 → P-06 `window_elapsed`; `Holds.Release`. Актор `system`, `job_id` в истории.
  - `internal/placement/app/channel_events.go`: `ChannelUnavailable(ctx, change)` (реализация `channel/domain.AvailabilityListener`): P-06 `channel_unavailable` для `created`, `awaiting_blogger`, `awaiting_schedule`; `scheduled` не меняется (S-25); `published` — WARN `placement needs dispute, not implemented` (P-14 — этап автопостинга). `ChannelRestored` — DEBUG. `BusyIntervals(ctx, channelID, from, to)` (реализация `PlacementSchedule`).
  - `internal/placement/app/stuck.go`: проверка `placement_scheduled_overdue` (35 мин после `publish_at`) для `platform/stuck`.
  - `internal/placement/app/metrics.go`: коллектор Prometheus `maxim_placements` по `status` (07 §5.5).
  - `internal/placement/jobs/jobs.go`: `Register(placement_timer, 5 попыток, PriorityHigh)`.
  - Журнал: INFO `placement timer fired` (`placement_id`, `timer`, `result`), DEBUG `placement timer outdated` (`placement_id`, `timer`, `status`), INFO `placements cancelled on channel loss` (`channel_id`, `count`).
  - Тесты (PostgreSQL, `clock.Manual`, `worker.RunOnce`): T-01 через 24 ч → `expired`, удержание снято; принял за минуту до T-01 → таймер устарел, ничего не меняется; T-13 → `expired` с `schedule_timeout`; T-14 в `created` → `cancelled` `window_elapsed`, в `awaiting_blogger` — без изменений; потеря канала отменяет `created`, `awaiting_blogger`, `awaiting_schedule` и не трогает `scheduled`; повтор задания ничего не меняет; `BusyIntervals` в карточке каталога; зависшее `scheduled` находится через 35 мин; метрика считает по состояниям.
  - Файлы: `internal/placement/app/{timers.go,channel_events.go,stuck.go,metrics.go,timers_test.go,channel_events_test.go}`, `internal/placement/jobs/jobs.go`.
<!-- Commit checkpoint: tasks 8-12 -->

### Phase 5: HTTP и сборка

- [x] **Task 13: Операции кабинетов (R-1, R-17, R-18, R-22)** (depends on 10, 11)
  - `internal/placement/httpapi`: `GetCart`, `AddCartItem`, `UpdateCartItem`, `DeleteCartItem`, `CheckoutCampaign`, `ListOrders`, `GetOrder`, `CancelOrder`, `CancelPlacement`, `CreateOrderPostVersion` — `httpx.RequireCabinet(ctx, "advertiser")`; `ListBloggerPlacements`, `GetBloggerPlacement`, `BloggerAcceptPlacement`, `BloggerDeclinePlacement`, `BloggerSchedulePlacement`, `BloggerReschedulePlacement` — `httpx.RequireCabinet(ctx, "blogger")`. Чтение `Idempotency-Key`. Преобразование в `Cart`, `Order`, `OrderSummary`, `AdvertiserPlacement`, `BloggerPlacement`, `PostVersion`, `PlacementHistoryEntry` (время — UTC, `time_from` — `HH:MM`).
  - `internal/post/httpapi`: убрать `CheckoutCampaign` из обработчиков модуля (R-1).
  - Ошибки через `apperr`: `404`, `409` с `code` и `details` (R-6, R-7, R-15, R-17, `insufficient_balance`), `409 invalid_transition` с фактическим состоянием, `422` с `details.reason`.
  - Журнал: WARN `403` пишет `httpx`; остальное — в `app`.
  - Тесты (`httptest` поверх собранного сервера): без нужного кабинета → `403`; CSRF обязателен для меняющих запросов; ответы проходят проверку схемой; `checkoutCampaign` → `201` `Order`; `insufficient_balance` → `409` с `details.missing_amount`; повтор с `Idempotency-Key` → тот же ответ; `bloggerSchedulePlacement` с `mode=at` без `publish_at` → `422 publish_at_required`; `acceptPlacement`, `createPlacementClaim`, `getOrderReport` → `501`.
  - Файлы: `internal/placement/httpapi/{handlers.go,cart.go,orders.go,blogger.go,convert.go,handlers_test.go}`, `internal/post/httpapi/*`.

- [x] **Task 14: Конфигурация, сборка, команда разработчика (R-2 … R-5, R-10, R-11, R-13, R-15, R-23)** (depends on 12, 13)
  - `internal/bootstrap/config.go`: `PLACEMENT_ACCEPT_TIMEOUT` (24h), `PLACEMENT_SCHEDULE_TIMEOUT` (24h), `PLACEMENT_WINDOW_GUARD` (1h), `PLACEMENT_MIN_LEAD` (1h), `PLACEMENT_SLOT_STEP` (5m), `PLACEMENT_REEDIT_GUARD` (24h), `CART_MAX_ITEMS` (50). Проверка диапазонов при запуске (`SLOT_STEP` делит час, `WINDOW_GUARD <= ACCEPT_TIMEOUT`). Комиссия — существующий `PLATFORM_FEE_RATE_BP` (`A-30`).
  - `internal/bootstrap/placement.go`: сборка `placement/app.Service`; переходники `Channels` (поверх `channel/app`), `Campaigns` и `PostVersions` (поверх `post/app`); `UnlimitedHolds`, `NopNotifier`, `AllowAllContractors` с метками этапов, которые их заменят. Подстановка `placement/app` в `channel` (`AvailabilityListener`, `PlacementSchedule`, `CartLookup`) и в `post` (`VersionListener`). `modules.go`, `worker.go` (`placement_timer`, проверка зависших, метрика), `server.go`.
  - `internal/bootstrap/dev.go`: команда `maxim dev post-version` (R-23), `cmd/maxim/main.go`.
  - `.env.example`, `compose.dev.yml`: новые переменные. Заметка для `/aif-docs`: как пройти S-03 → S-06 → S-14 руками (Swagger и `maxim dev post-version`), что `publish_placement` пока не выполняется.
  - Журнал: INFO `worker started` с `placement_timer`; конфигурация — маскированной, как сейчас; WARN `dev command used` (`command`, `order_number`).
  - Тесты: проверки конфигурации на границах; `wiring_test.go`: `placement_timer` зарегистрирован, операции этапа не отвечают `501`, `acceptPlacement` — всё ещё `501`, порты `channel` и `post` получили реализации `placement`; команда `dev` при `APP_ENV=production` завершается ошибкой.
  - Файлы: `internal/bootstrap/{config.go,config_test.go,placement.go,dev.go,dev_test.go,modules.go,worker.go,server.go,channel.go,post.go,unimplemented.go,wiring_test.go}`, `cmd/maxim/main.go`, `.env.example`, `compose.dev.yml`.
<!-- Commit checkpoint: tasks 13-14 -->

### Phase 6: Проверка и документы

- [x] **Task 15: Сквозные тесты заказа и размещений** (depends on 14)
  - `internal/bootstrap/placement_e2e_test.go` на собранных `api` и `worker` (PostgreSQL, `clock.Manual`, поддельные порты, `storagefake`, `mailfake`):
    - S-03 → S-06: рекламодатель собирает корзину из трёх каналов двух блогеров, блогер меняет цену одного — оформление `409 cart_changed`; подтверждение цены; оформление → заказ `moderation`, три размещения `created`, кампания `ordered`, `in_cart` в каталоге — `false`.
    - S-17 (вызовом `post/app`, как будущий этап): одобрение → `marking`; `MarkVersionReady` → `awaiting_blogger`, блогер видит запрос с постом и erid.
    - S-14: первый блогер принимает и планирует; второй блогер отказывается от одного своего размещения (`declined`, удержание снято), а второе его размещение истекает по T-01 (`worker.RunOnce` после сдвига часов).
    - S-29: правка поста → запланированное размещение возвращается в `created`, задания публикации отменены; после `ready` новой версии — снова `awaiting_blogger`.
    - S-25: канал первого блогера `suspended` → размещение в `awaiting_schedule` отменено (`channel_unavailable`).
    - S-07: `cancelOrder` отменяет оставшееся; заказ `cancelled`, кампания `archived`.
  - Проверка журнала сборщиком записей: обязательные события 07 §5.3 этапа (переход выполнен и отклонён, сработал таймер, задание запущено и завершено) присутствуют. Текст поста, комментарий блогера, название и ИНН рекламодателя не встречаются.
  - Проверка истории: у каждого размещения записи в порядке переходов, с актором и источником (`job_id` у таймеров).
  - Файлы: `internal/bootstrap/placement_e2e_test.go`.

- [x] **Task 16: Синхронизация документов с решениями этапа** (depends on 14)
  - После подтверждения R-1 … R-23 внести их в документы:
    - 02 S-03 — правила корзины (R-8), подтверждение цены (R-7); S-06 — коды отказа (R-6, R-7); S-07 — `cancelOrder` (R-17); S-14 — причины отказа планирования (R-13); S-29 — доступность только в `rejected` и `in_progress` (R-15).
    - 03 §4 — P-02 при истёкшем окне и недоступном канале (R-10); §5 — T-14 (R-11), формулы T-01 и T-13 (R-10), полуоткрытый интервал топа и `[time_from, time_to)` (R-9, R-13); §11 — заголовок `Idempotency-Key` (R-18).
    - 04 §4 — `campaign.order_id`, переход в `archived` (R-19); §5 — `placement.sent_to_blogger_at` (R-16), `idempotency_key` в `order` и `placement_state_history` (R-18), уникальный `order.campaign_id`, имя `"order"` и последовательность (R-21), триггеры (R-20).
    - 07 §2 — `placement_timer` с T-14; §8 — новые параметры (задача 14).
    - `.ai-factory/ARCHITECTURE.md` — модуль `placement` в дереве с пакетами, порты `Holds`, `ContractorGate`, `Notifier` с пустыми реализациями (R-2 … R-4), `post/domain.VersionListener` (R-5), `channel/domain.CartLookup`, оформление в `placement/app` (R-1), блокировка канала для I-06 (R-14).
    - `docs/open-questions.md` — Q-17 (модель баланса реализуется через порт), A-10 (значения таймеров в конфигурации).
  - Журнал: не применимо.
  - Файлы: `docs/02-scenarios.md`, `docs/03-placement.md`, `docs/04-domain-model.md`, `docs/07-non-functional.md`, `docs/open-questions.md`, `.ai-factory/ARCHITECTURE.md`.
<!-- Commit checkpoint: tasks 15-16 -->

## Критерии готовности этапа

- `go build ./...`, `go vet ./...`, `go test -race ./...` проходят локально и в CI. `sqlc generate` и `go generate ./...` не дают изменений. `mockapi` собирается и проходит свой контрактный тест.
- Таблица переходов совпадает с 03 §4 (тест-сверка), перечисления — с 04 §9. Состояние размещения меняет только `Placement.Apply`.
- Каждый переход этапа пишет историю, задания и вызовы портов в одной транзакции (I-10); повтор таймера, события версии и действия с тем же ключом ничего не меняет (03 §11).
- Два одновременных планирования в одном канале не дают пересекающихся топов (I-06); выход из конечного состояния и смена сумм невозможны и прямым запросом к базе (I-04, I-05).
- `docker compose -f compose.dev.yml up`: корзина → оплата → `maxim dev post-version … ready` → принятие и планирование блогером проходят руками через Swagger.
- Текст поста, комментарий блогера, название и ИНН рекламодателя отсутствуют в журнале.
- Решения R-1 … R-23 подтверждены и внесены в документы (задача 16).
