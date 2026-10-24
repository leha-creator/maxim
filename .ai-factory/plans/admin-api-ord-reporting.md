# План реализации: API администратора и отчётность ОРД

Branch: feature/admin-api-ord-reporting
Created: 2026-10-24
Refined: 2026-10-24 (`/aif-improve`, сверка с кодом)

## Settings
- Testing: yes. Чистые табличные тесты в `domain`: роли операций, второе подтверждение (порог, свой запрос, срок), разбиение просмотров и дат по месяцу, суммы статистики и актов, номер акта, видимость переписки по спору, правила `admin_cancel` и `admin_mark_removed`, блокировка канала и пользователя. Сценарии `app` + `postgres` — на настоящей PostgreSQL (`platform/dbtest`, `clock.Manual`), ОРД — `ordfake`. HTTP — собранный сервер (`bootstrap/*_http_test.go`): ответ по OpenAPI, `401`, `403 admin_required` для чужой роли (`moderator` на операциях `senior`), CSRF. Сквозной тест ежемесячной отчётности на собранных `api` и `worker`. `mockapi` — каждая новая операция в `contract_test`, регрессии в `gaps_test.go`.
- Logging: verbose. `log/slog` JSON, 07 §5.3: каждое изменяющее действие администратора — WARN с `op`, `admin_user_id`, объектом и `reason` (текст основания в журнал приложения не пишется, только в `audit_log`), плюс `audit_log` с актором `admin`; отказ по роли — WARN `admin access denied` (уже есть в `httpx.RequireAdmin`); запрос второго подтверждения — WARN `admin approval requested` (`approval_id`, `kind`, `amount`), решение — WARN `admin approval approved|rejected`, исполнение — WARN владельца операции; переписка по спору — INFO `dispute message posted` (`dispute_id`, `author_type`, `to`; текст не пишется); отчётность ОРД — INFO `ord report built` (`period`, `statistics`, `invoices`, `amount`), INFO `ord views snapshot taken` (`period`, `publications`), DEBUG по каждому объекту, ERROR `ord report overdue` с 20-го числа; ПДн — WARN `personal data anonymized` (`user_id`, без почты), INFO `personal data exported` (`user_id`). Почта, ИНН, телефон, ФИО — только маскированными (07 §5.4).
- Docs: yes. В конце обязательная точка: 02 S-16…S-20, S-30, S-31 («Как сделано»), 04 §5 (`dispute_message`), §6 (`ord_statistics`, `ord_invoice`, `ord_invoice_item`, `ord_report`, `publication_views_snapshot`), §2 (`admin_approval`, блокировка), 05 §3.6, §3.7, 07 §2 (задания), §4 (матрица ролей), §8 (переменные), 08 (экраны `adm/*` — какие операции), `docs/open-questions.md`, `docs/frontend-api-gaps.md`, `docs/launch/personal-data.md`, `docs/launch/pilot-plan.md`, `deploy/app/README.md`, README команд, `.ai-factory/ARCHITECTURE.md` (модуль `admin`).

## Roadmap Linkage
Milestone: "API администратора и отчётность ОРД"
Rationale: план реализует этап целиком: операции администратора в OpenAPI и их реализацию (модерация каналов и постов, размещения и ручные операции S-19, споры с перепиской, реестр выплат, деньги со вторым подтверждением, оплаты и счета, организации и пользователи с блокировкой, справочник категорий, журнал действий, ошибки ОРД), ежемесячную статистику и акты в ОРД (S-31) со снимком просмотров, а также эксплуатационные хвосты этапа из `ops-readiness.md` (доступ к `/api/v1/admin` на edge-Caddy, обезличивание и выгрузка ПДн, порядок дежурного, переменные окружения).

## Источники ТЗ

- Сценарии: 02 S-16 (модерация канала — есть), S-17 (модерация поста), S-18 (спор и переписка), S-19 (ручные операции: `admin_cancel`, `admin_remove_post`, `admin_mark_removed`, `admin_open_dispute`, `retry_payout`, `manual_ledger_entry`, `block_channel`), S-20 (ошибки ОРД), S-30 (зачисление оплаты), S-31 (ежемесячная отчётность ОРД), S-33 (возврат остатка).
- Роли: 07 §4 — `moderator`: модерация каналов и постов, споры; `senior`: дополнительно ручные операции с деньгами, справочники, блокировки; второе подтверждение `senior` выше `LEDGER_MANUAL_SECOND_APPROVAL_THRESHOLD`.
- ОРД: 05 §3.2 (последовательность), §3.3 (`ord_statistics`, `ord_invoice`), §3.6 (статистика: показы, даты, сумма), §3.7 (цикл месяца: снимок, формирование, отправка 1–5, ошибки до 20, срок 30); 04 §6 (таблицы); 07 §2 (`ord_monthly_report`).
- Данные: 04 §5 (`dispute_message`), §2 (`user.status`, `organization.status`), §10 и 06 §8 (ПДн), `docs/launch/personal-data.md` (обезличивание и выгрузка — этап API администратора).
- Эксплуатация: `.ai-factory/plans/ops-readiness.md` «Что дописать» (строка этапа), `deploy/app/maxim-app.caddy`, `deploy/app/release.sh`, `deploy/scripts/check-env.sh`, `docs/launch/pilot-plan.md` (порядок дежурного).
- Фронтенд: `docs/frontend-api-gaps.md` — разделы MAXI-51 и MAXI-52 (адреса операций предложены фронтом), `docs/wireframe/index.html` экраны `adm/*`, `docs/08-screens.md` №22–37.
- Существующий код:
  - `internal/channel/httpapi/admin.go` (образец обработчика администратора, `httpx.RequireAdmin`), `internal/channel/app/moderation.go`;
  - `internal/placement/app/disputes.go` (`ResolveDispute`, `OpenDisputeByAdmin`, `RemovePostByAdmin`, `AdminDisputes`, `AdminDispute`, `adminTx`), `placement/domain/transitions.go` (P-06 и P-10 с актором `admin` уже в таблице), `placement/domain/actions.go` (`Cancel`);
  - `internal/post/app/versions.go` (`ApproveVersion`, `RejectVersion` — роль проверяет вызывающий), `internal/bootstrap/dev.go` (единственный вызов сейчас);
  - `internal/ledger/app/admin.go` (`Manual`, `DepositTopup`, `BalanceReturn`, `Journal`, `ErrSecondApproval`), `internal/payout/app/admin.go` (`AdminList`, `Retry`, `MarkLegalPaid`), `internal/billing/app/{statements.go,admin.go,invoices.go}`;
  - `internal/marking/app/{errors.go,sync.go,records.go,poll.go}`, `marking/domain/{sync.go,ports.go,status.go}`, `internal/integration/ord/{client.go,objects.go,types.go,redact.go}`, `ordfake/fake.go`, `bootstrap/bridge/marking.go`;
  - `internal/bootstrap/{admin.go,admin_dispute.go,admin_money.go,admin_payout.go,admin_ord.go}` (команды остаются), `internal/platform/audit` (только запись), `internal/platform/job/schedule.go` (`MonthlyAt`, `DailyAt`);
  - `mockapi/internal/{handlers/admin.go,store/*,server/contract_test.go,gaps_test.go}`.

## Границы этапа

Входит:
- OpenAPI: операции администратора по тегам `admin-orders`, `admin-placements`, `admin-disputes`, `admin-payouts`, `admin-payments`, `admin-ledger`, `admin-approvals`, `admin-organizations`, `admin-dictionaries`, `admin-audit`, `admin-ord`, `admin` (счётчики), дополнения `admin-channels` (блокировка); переписка по спору в кабинетах рекламодателя и блогера. Реализация в бэкенде и `mockapi`.
- Второе подтверждение ручных денежных операций вторым `senior` через HTTP: запрос и подтверждение (таблица `admin_approval`).
- `admin_cancel`, `admin_mark_removed`, `block_channel` (и снятие блокировки), блокировка пользователя, справочник категорий каналов.
- Ежемесячная отчётность ОРД: снимок просмотров, `ord_statistics`, `ord_invoice`, `ord_invoice_item`, `ord_report`, задания `ord_monthly_report` и `ord_report_check`, отправка через `sync_ord_object`, операции и команды «сформировать заново» и «отправить».
- Метрики решений по спорам (время решения), счётчики подтверждений.
- Эксплуатация: ограничение `/api/v1/admin` по IP на edge-Caddy, команда обезличивания и выгрузки ПДн, порядок дежурного, переменные окружения.

Не входит:
- Правка ИНН и типа лица организации администратором: они неизменяемы (`account/domain`, R-15 этапа «Маркировка»); исправление — новая организация (вопрос Q-46).
- Блокировка организации целиком (`organization.status`): блокируются пользователи; запрет кабинетных операций по статусу организации — следующий этап (вопрос Q-47).
- Сравнение версий поста (`adm/order`, «сравнение версий») — кабинет строит его из двух версий сам.
- Ежедневный проверочный прогон против тестового контура ОРД (05 §3.8) — этап «Юридическая и эксплуатационная готовность».
- Поиск и `total` во всех старых списках кабинетов (замечание «Общее») — в новых списках администратора `total` отдаётся.

## Решения этапа, требующие подтверждения

Документы этих мест не определяют или определяют не полностью. План реализует рекомендуемый вариант без вопроса пользователю; задача 24 вносит решения в документы.

| # | Вопрос | Решение в плане | Задача |
|---|---|---|---|
| R-1 | Матрица ролей операций (07 §4, вопрос фронтенда «кто может») | Обе роли (`moderator`, `senior`): каналы (модерация), заказы (модерация поста), размещения (чтение), `admin_remove_post`, `admin_open_dispute`, споры (чтение, предпросмотр, решение, переписка), ошибки ОРД (чтение, «Повторить»), отчёты ОРД (чтение), организации (чтение, ИНН у `moderator` маскирован), счётчики. Только `senior`: `admin_cancel`, `admin_mark_removed` (ручные операции с последствием для денег и контроля), `block_channel`/снятие, блокировка пользователей, правка организации, справочник категорий, выплаты, оплаты и счета, проводки и сверка, подтверждения, журнал действий, «сформировать заново» и «отправить» отчёт ОРД. Отказ — `403 admin_required` (WARN `admin access denied`). Матрица — константы в `platform/httpx` (`AdminAny`, `AdminSenior`) и таблица в описании OpenAPI | 1, 4 |
| R-2 | Второе подтверждение через HTTP | Операция с суммой больше порога (`ledger manual`, `deposit-topup`, `payout mark-paid`) не исполняется сразу: создаётся `admin_approval` (`pending`), ответ `202` с запросом; другой `senior` подтверждает `POST /admin/approvals/{id}/approve` — операция исполняется с `ApproverID`, подтверждение и исполнение — в одной транзакции; отклонение — `…/reject` с основанием. Свой запрос подтвердить нельзя — `409 same_admin`; повтор решения — `409 approval_decided`; старше `ADMIN_APPROVAL_TTL` (72 ч) — `409 approval_expired` (строка переходит в `expired`). Сумма не больше порога — исполнение сразу (`201`). Ключ идемпотентности запроса (`Idempotency-Key`) хранится в запросе и передаётся операции | 3, 9, 10, 11 |
| R-3 | Модуль подтверждений, журнала действий и счётчиков | Новый модуль `internal/admin` (`domain`, `app`, `postgres`, `httpapi`): `admin_approval`, чтение `audit_log`, счётчики меню. Исполнители подтверждения — порты `admin/domain.Executor` по виду (переходники `bridge.AdminLedger`, `bridge.AdminPayouts`); чужие таблицы модуль не пишет | 9, 14 |
| R-4 | Модерация поста (S-17) | `GET /admin/orders?status=moderation|marking|all` — очередь по началу периода кампании, затем по времени ожидания, признак `urgent` (A-40); `GET /admin/orders/{id}` — заказ, рекламодатель (ИНН маскирован), итоговый текст и изображения текущей версии, размещения, креатив (`sync_status`, `erid`, `erir_status`); `approve` — только заказ в `moderation` с версией `pending_moderation`; `reject` — причина 1…500. Вызов — порт `placement/domain.PostModeration` (`bridge.PlacementPostModeration` над `post/app.ApproveVersion|RejectVersion`), дальше всё как в `maxim dev post-version` | 5 |
| R-5 | `admin_cancel` (S-19) | `POST /admin/placements/{id}/cancel` `{reason}`: P-06 с актором `admin` и причиной `admin_cancel` из `created`, `awaiting_blogger`, `awaiting_schedule`, `scheduled`; удержание снимается (`ReleaseHold`), задания публикации отменяются, уведомления обеим сторонам (как при отмене рекламодателем); иначе `409 invalid_transition` | 7 |
| R-6 | `admin_mark_removed` (S-19) | `POST /admin/placements/{id}/mark-removed` `{removed_at, reason}`: только `published`, `removed_at` между `published_at` и «сейчас»; публикация снята с причиной `admin` и этим временем, P-10 с актором `admin`, `final_views` — последнее `publication.views`; нарушение не записывается; задание снятия отменяется; T-05 ставится от `removed_at` | 7 |
| R-7 | `block_channel` (S-19) | `POST /admin/channels/{id}/block` `{reason}` из любого состояния, кроме `blocked`: канал `blocked`, `status_reason` — основание, исчезает из каталога и корзины, новые размещения невозможны, текущие продолжаются (слушатель недоступности не вызывается). `…/unblock` — в `moderation` (повторная проверка). Уведомление блогеру `channel.blocked` | 6 |
| R-8 | Справочник категорий («справочники», `senior`) | `POST /admin/channel-categories` (`id`, `name`, `position`), `PATCH /admin/channel-categories/{id}` (`name`, `position`, `archived`). Архивная категория не выбирается для новых каналов и не показывается в фильтре каталога, уже выбранная у канала остаётся. Форматы размещения — без операций (часть правил, 03 §2) | 6 |
| R-9 | Переписка по спору (S-18 шаг 2) | Таблица `dispute_message` (`dispute_id`, `author_type` `admin|advertiser|blogger`, `author_user_id`, `recipient` `advertiser|blogger|both` — у сообщения администратора, `text` 1…2000). Администратор пишет стороне или обеим; сторона видит сообщения администратора, адресованные ей, и свои; отвечает только администратору. Писать можно, пока спор не решён (`409 dispute_resolved`). Уведомление `dispute.message` адресату (администраторам — при ответе стороны). Текст в журнал не пишется | 3, 8 |
| R-10 | Идентификатор спора в адресах | `/admin/disputes/{id}` — `dispute.id`; кабинеты — по размещению (`/placements/{id}/dispute/messages`, `/blogger/placements/{id}/dispute/messages`). Предпросмотр распределения по доле — `GET /admin/disputes/{id}/preview?resolution=…&share=…` | 8 |
| R-11 | Метрики решений | `maxim_dispute_resolution_seconds{resolution}` (гистограмма от открытия до решения), `maxim_disputes_resolved_total` уже есть; `maxim_admin_approvals_total{kind,decision}` | 8, 9 |
| R-12 | Блокировка пользователя | `POST /admin/users/{id}/block|unblock` `{reason}` (`senior`): `user.status = blocked`, все сессии удаляются, вход отказывается (уже есть); администратора блокировать нельзя (`409 admin_user`) — сначала `maxim admin revoke`. Блокировка организации — Q-47 | 3, 13 |
| R-13 | Правка организации администратором (S-20 «исправить данные») | `PATCH /admin/organizations/{id}` `{name?, kpp?, reason}` (`senior`): та же проверка, что в кабинете, и тот же слушатель (`OrganizationListener` → контрагент ОРД на повторную отправку). ИНН и тип лица не меняются (Q-46). Ошибки полей — `422` с `details.field` | 13 |
| R-14 | ИНН в ответах | Списки и карточки для `moderator` — маскированный ИНН (`77****1234`), для `senior` — полный в организациях, оплатах и выплатах (нужен для сверки и исправления ОРД). CSV выплат — ИНН маскирован, ФИО/наименование — как в реестре. Выгрузка CSV пишет `audit_log` `payout.exported` | 11, 12, 13 |
| R-15 | Журнал действий (`adm/audit`) | `GET /admin/audit-log` (`senior`): фильтры `actor_user_id`, `actor_type`, `action`, `object_type`, `object_id`, `organization_id`, `from`, `to`, `manual_only` (= `actor_type = admin`), курсор по `(created_at, id)`; `data` — как записано (маскирование при записи, `audit.Mask`), `ip` не отдаётся. Справочник кодов действий — перечисление в описании операции | 14 |
| R-16 | Счётчики меню | `GET /admin/counters`: каналы в `moderation`, заказы в `moderation`, открытые и просроченные споры, объекты ОРД в `error`, несопоставленные поступления, выплаты `failed`, подтверждения `pending`, отчёт ОРД прошлого месяца не отправлен. Счёт — через порты модулей (`admin/domain.Counters`) | 14 |
| R-17 | Ошибки ОРД через HTTP (S-20) | `GET /admin/ord/errors?type=` — как `ListErrors`; `GET /admin/ord/objects/{type}/{id}` — объект, состояние синхронизации, последний запрос и ответ из `outbound_request` (тело уже маскировано, R-16 «Маркировки»), что из-за него стоит (зависимые объекты); `POST …/retry` `{reason}` — `Retry` | 15 |
| R-18 | Снимок просмотров (05 §3.6, R-19 «Маркировки») | Задание `ord_monthly_report` в 00:05 МСК 1-го числа сначала сохраняет `publication_views_snapshot(publication_id, period, views, views_checked_at)` для публикаций, живых на 00:00 МСК: значение — последнее `publication.views` (сверка обновляет раз в час). Отдельного вызова MAX нет (I-13, задание не ходит наружу в транзакции); погрешность — до часа просмотров (ПРЕДПОЛОЖЕНИЕ Q-08) | 16, 19 |
| R-19 | Показы, даты и суммы статистики (05 §3.6) | Для пары «креатив (версия) + площадка (канал)» за месяц: все размещения с публикацией, пересекающей месяц. Показы — снято в месяце: `final_views` (или последнее `views`) минус снимок начала месяца (если публикация началась раньше); живо на конец месяца: снимок конца минус снимок начала; не меньше 0. Даты — `published_at`…`removed_at`, обрезанные месяцем, по Москве. Сумма — `total_amount − coalesce(refund_amount, 0)` размещений, снятых в месяце; при переходе через границу — целиком в месяце снятия. Несколько размещений пары — сумма показов и денег, минимум и максимум дат, `placement_id` — первое. Пара без показов и без суммы не создаётся | 18 |
| R-20 | Акты (05 §3.7) | По изначальному договору рекламодателя — акт на сумму статистики его креативов за месяц, разбивка по парам; по договору с блогером — сумма `payout_amount` (до распределения — `blogger_amount`) размещений его каналов, снятых в месяце, разбивка по парам. Тип — `ORD_INVOICE_TYPE_INITIAL` (`intermediary-report`) и `ORD_INVOICE_TYPE_PUBLISHER` (`invoice`), Q-14. Номер — `ORD_INVOICE_NUMBER_PREFIX` + `YYYYMM` + `-` + порядковый номер в месяце; дата — последний день месяца. Акт с суммой 0 не создаётся | 18 |
| R-21 | Состояние отчёта месяца | `ord_report(period, built_at, built_by, send_requested_at, sent_by)`; состояние для экрана — из строк: `not_built`, `built` (есть `pending`, отправка не запрошена), `sending`, `sent` (всё `synced`), `has_errors`. «Сформировать заново» пересчитывает строки: новая нагрузка у `synced` — обратно в `pending` (правка в течение 30 дней), строки, которых больше нет и не отправлялись, удаляются; отправленные остаются (ПРЕДПОЛОЖЕНИЕ Q-42: удаление в ОРД не используется, сумма правится новой нагрузкой). «Отправить» ставит `sync_ord_object` всем `pending` и `error` месяца. Задание 1-го числа: снимок, формирование и отправка (`ORD_REPORT_AUTO_SEND=true`) | 19, 20 |
| R-22 | Отправка статистики и акта (новые виды объектов ОРД) | Виды `statistics` и `invoice` в `marking`: общий ход `sync_ord_object` и `poll_ord_status` (три шага, I-13). Статистика ждёт креатив (erid получен) и площадку; акт — договор и креативы разбивки. Акт — два вызова в одном шаге: `POST /invoice`, затем `POST /invoice/creatives`; опрос — по `reqid` второго (Q-42). Финальный успех — `ORD success` (ЕРИР для отчётности не проверяется, Q-42) | 17, 19 |
| R-23 | Ошибки отчёта до 20-го (05 §3.7) | Задание `ord_report_check` ежедневно в 10:00 МСК: с `ORD_REPORT_ERROR_DAY` (20) числа, если за прошлый месяц есть строки не `synced`, — оповещение `ord_report_overdue:{YYYY-MM}` уровня ERROR (раз в сутки — повтор оповещения не чаще часа, 07 §3); всё `synced` — оповещение закрывается | 19 |
| R-24 | Попытки `ord_monthly_report` | 1 попытка, как у всех периодических (07 §2 «Одна попытка…»); в таблице 07 §2 исправить «3» на 1. Сбой — ERROR, повтор — `maxim admin ord report build --period` или `POST …/rebuild` | 19, 24 |
| R-25 | Доступ к `/api/v1/admin` на edge-Caddy (ops-readiness) | Список адресов: переменная стека `EDGE_ADMIN_ALLOW` (CIDR через пробел, обязательна), `release.sh` подставляет `__ADMIN_ALLOW__`; запрос к `/api/v1/admin*` не из списка — `403` JSON `admin_network` от Caddy. Отдельный домен не нужен: кабинет администратора — тот же фронтенд. Роль всё равно проверяет бэкенд | 22 |
| R-26 | ПДн по запросу субъекта (`personal-data.md`) | `maxim admin pd export --email <субъект> --operator-email <senior>` — JSON в stdout: пользователь, членства, организации (реквизиты целиком — это данные субъекта), принятия документов, активные сессии (IP), записи `audit_log` с его участием; `audit_log` `personal_data.exported`. `maxim admin pd anonymize --email … --reason … --operator-email …` — блокировка, `email` → `anonymized-<id>@invalid`, сессии и коды удаляются, IP в `legal_acceptance` и `audit_log` не трогаются (доказательство принятия, хранение ¹), `contact_name`, `phone`, `signer_name` организаций, где он единственный участник, — `null`/«обезличено»; ИНН, наименование, договоры, проводки сохраняются; `audit_log` `personal_data.anonymized`. Только `senior` | 21 |

## Уточнения после сверки с кодом (aif-improve, 2026-10-24)

| # | Что найдено в коде | Решение | Задача |
|---|---|---|---|
| I-1 | `payout/app.DepositReport` вызывает `Provider.GetDepositBalance` (Рокет Ворк). Из HTTP-обработчика внешний вызов запрещён (ARCHITECTURE: `app` и `httpapi` не вызывают `integration/*` вне заданий) | Сводки `getAdminPayoutSummary` и `getLedgerSummary` берут только журнал и таблицы: `payout/app.DepositLedgerView` (остаток депозита в журнале, к выплате, в работе, нехватка относительно журнала, число `failed`); значение API депозита и расхождение — только команда `admin money reconcile` и задание `check_rocketwork_deposit` | 10, 11 |
| I-2 | При `ORD_ENABLED=false` (только разработка) слушатели маркировки не подключены: одобренная версия остаётся `approved`, пока `maxim dev post-version ready` не выдаст erid | `ApproveOrderPost` при выключенной маркировке (порт `PostModeration.MarkingDisabled()`) после одобрения выполняет `MarkVersionReady` с erid `dev-<номер заказа>` — тот же путь, что `dev post-version ready`; в журнале WARN `marking disabled, version made ready` | 5 |
| I-3 | В `account/postgres` есть только `DeleteExpiredSessions`; удаления сессий пользователя нет | Запрос `DeleteSessionsByUser` (блокировка и обезличивание), `DeleteAuthCodesByEmail` (обезличивание) | 13, 21 |
| I-4 | CSV отчёта по заказу пишется в `placement/httpapi/report.go` своим ответом (`Content-Type`, BOM) | Отдельный `httpx/csv.go` не нужен: CSV выплат — тем же приёмом в `payout/httpapi` (`exportAdminPayouts`), запись строк — `encoding/csv` с `;` | 4, 11 |
| I-5 | Порог второго подтверждения для `mark-paid` проверяется в команде (`admin_payout.go:167`), а не в сервисе | Проверка переносится в `payout/app.MarkLegalPaid` (`ErrSecondApproval` из `ledger/app` через порт порога или собственное поле `ManualSecondApproval`); команда остаётся с `--approver-email`, HTTP — через подтверждение | 11 |
| I-6 | `internal/bootstrap/unimplemented.go` целиком генерирует `genstub`; ручная правка не нужна | Задача 1 только перегенерирует; пока операция не реализована, она отвечает `501` — допустимо между коммитами, к концу этапа заглушек администратора нет | 1 |
| I-7 | В `ModeAdmin` (`maxim admin …`) клиента ОРД нет (`connectORD` только в `worker`) | Команды и HTTP-операции отчёта только формируют строки и ставят `sync_ord_object`; отправляет `worker` | 19, 20 |
| I-8 | `AdminDisputes` читает размещение по одному на строку (N+1) | Список споров для HTTP — одна выборка с соединением `dispute` + `placement` и `total` | 8 |
| I-9 | Тег `admin-channels` в OpenAPI говорит «тестовый сервер не реализует (`501`)», хотя `mockapi` реализует; бэкенд `listAdminChannels` не отдаёт `total` | Описание тега исправляется; `total` в `listAdminChannels` (счёт в `channel/postgres`) | 1, 6 |

## Допущения о внешних данных

Форматов ежемесячной статистики и актов Яндекс ОРД v8 в репозитории нет (`ord.yandex.ru/api/openapi.json` не скачан, тестовый контур недоступен). Всё ниже помечено в коде `// ПРЕДПОЛОЖЕНИЕ (Q-NN): …` и проверяется на тестовом контуре в этапе «Проверки и доступы».

1. **Q-42, `POST /statistics`.** Тело — массив записей `{creativeId, platformId, impsFact, impsPlan, startDateFact, endDateFact, startDatePlan, endDatePlan, amount: {incl: копейки/100 строкой}}`; один вызов на запись (`id` записи — `ord_statistics.id`, внешний идентификатор — тот же). Ответ — как у прочих методов (`request_id`, `status`). Финальный успех — `ORD success`; ЕРИР для статистики не проверяется. План показов равен факту.
2. **Q-42, `POST /invoice` и `POST /invoice/creatives`.** Акт: `{id, contractId, type, number, date, startDate, endDate, amount, isVat: false}`; разбивка: `{invoiceId, items: [{creativeId, platformId, amount}]}` — второй вызов после первого в одном шаге, опрос по `reqid` второго. Повтор с тем же `id` — обновление.
3. **Q-43, тип и номер акта.** Тип изначального договора — `intermediary-report`, договора с блогером — `invoice` (конфигурация); номер акта задаёт платформа, дата — последний день месяца.
4. **Q-44, сумма при незавершённом споре.** В сумму месяца входит `total_amount − refund_amount`, где неизвестный ещё возврат — 0; после решения спора отчёт формируется заново и отправляется в течение 30 дней (05 §3.7).
5. **Q-08, показы.** Источник — `publication.views` (сверка) и `placement.final_views`; без `stat.views` показы 0, запись статистики с суммой всё равно подаётся.
6. **Q-45, срок подтверждения.** 72 часа (`ADMIN_APPROVAL_TTL`): запрос не висит бесконечно, решение «подтвердить» после срока отказывается.
7. **Q-15.** Ограничитель ОРД общий (`ord:global`, 5 rps): отправка отчёта за месяц сотен пар идёт через ту же очередь.
8. **Q-33, обезличивание.** IP в `legal_acceptance` и `audit_log` сохраняются как доказательство (срок хранения ¹), почта заменяется, контакты организации, где субъект — единственный участник, стираются.

## Commit Plan
- **Commit 1** (после задач 1–2): `feat(api): admin operations contract and mockapi`
- **Commit 2** (после задач 3–4): `feat(admin): admin schema, approvals and role matrix`
- **Commit 3** (после задач 5–8): `feat(admin): order moderation, placements, channel block, disputes api`
- **Commit 4** (после задач 9–12): `feat(admin): money operations api with second approval`
- **Commit 5** (после задач 13–15): `feat(admin): organizations, users, audit log, counters, ord errors api`
- **Commit 6** (после задач 16–20): `feat(marking): monthly ord statistics and invoices`
- **Commit 7** (после задач 21–22): `feat(ops): personal data commands, admin network on edge, env`
- **Commit 8** (после задач 23–24): `test(admin): admin api and ord reporting e2e; docs`

## Tasks

### Phase 1: Контракт (разблокирует фронтенд)

- [x] **Task 1: OpenAPI операций администратора (R-1 … R-17, R-21)**
  - `api/openapi.yaml`: теги и операции (имена — `operationId`):
    - `admin-orders`: `listAdminOrders`, `getAdminOrder`, `approveOrderPost`, `rejectOrderPost`.
    - `admin-channels` (дополнение): `blockChannel`, `unblockChannel`; описание тега без «тестовый сервер не реализует».
    - `admin-dictionaries`: `createChannelCategory`, `updateChannelCategory`.
    - `admin-placements`: `listAdminPlacements`, `getAdminPlacement`, `adminCancelPlacement`, `adminRemovePost`, `adminMarkRemoved`, `adminOpenDispute`.
    - `admin-disputes`: `listAdminDisputes`, `getAdminDispute`, `previewDisputeResolution`, `resolveDispute`, `listAdminDisputeMessages`, `createAdminDisputeMessage`; кабинеты: `listPlacementDisputeMessages`, `createPlacementDisputeMessage`, `listBloggerDisputeMessages`, `createBloggerDisputeMessage`.
    - `admin-payouts`: `listAdminPayouts`, `getAdminPayoutSummary`, `exportAdminPayouts` (`text/csv`), `retryPayout`, `markPayoutPaid` (`200` или `202` с подтверждением).
    - `admin-payments`: `getLatestBankStatement`, `uploadBankStatement` (multipart), `listBankTransactions`, `matchBankTransaction`, `ignoreBankTransaction`, `listAdminInvoices`, `cancelAdminInvoice`.
    - `admin-ledger`: `getLedgerSummary`, `listLedgerEntries`, `createManualLedgerEntry` (`Idempotency-Key` обязателен; `201` или `202`), `createDepositTopup`, `createBalanceReturn`.
    - `admin-approvals`: `listAdminApprovals`, `approveAdminApproval`, `rejectAdminApproval`.
    - `admin-organizations`: `listAdminOrganizations` (`query`, `role`), `getAdminOrganization`, `updateAdminOrganization`, `blockUser`, `unblockUser`.
    - `admin-audit`: `listAuditLog`.
    - `admin-ord`: `listOrdErrors`, `getOrdObject`, `retryOrdObject`, `listOrdReports`, `getOrdReport`, `rebuildOrdReport`, `sendOrdReport`.
    - `admin`: `getAdminCounters`.
  - Схемы: `AdminOrderListItem`, `AdminOrder`, `AdminPlacementListItem`, `AdminPlacement`, `AdminDisputeListItem`, `AdminDispute`, `DisputeResolutionInput`, `DisputePreview`, `DisputeMessage`, `DisputeMessageInput`, `AdminPayout`, `AdminPayoutSummary`, `BankStatementSummary`, `BankTransaction`, `BankTransactionMatchInput`, `AdminInvoice`, `LedgerSummary`, `LedgerEntry`, `ManualLedgerInput`, `AdminApproval`, `AdminOrganizationListItem`, `AdminOrganization`, `AdminOrganizationUpdate`, `AuditLogEntry`, `OrdErrorItem`, `OrdObject`, `OrdReportSummary`, `OrdReport`, `OrdStatisticsRow`, `OrdInvoiceRow`, `AdminCounters`, `AdminReasonInput`, `ChannelCategoryInput`; ответ `Accepted` (подтверждение). В `Forbidden` — `admin_required`; у каждой операции в описании — роль (`Роль: moderator, senior` или `Роль: senior`). Все списки — `PageMeta` с `total`. Примеры тел для `contract_test`.
  - `info.description`: матрица ролей (R-1), второе подтверждение (R-2), ИНН в ответах (R-14).
  - `go generate ./internal/apigen/...` (`api.gen.go`, `internal/bootstrap/unimplemented.go`), `cd mockapi && go generate ./internal/api/...`.
  - Журнал: не применимо.
  - Тесты: `mockapi/internal/api/spec_test.go` (спецификация валидна); `go build ./...`.
  - Файлы: `api/openapi.yaml`, `internal/apigen/api.gen.go`, `internal/bootstrap/unimplemented.go`, `mockapi/internal/api/api.gen.go`.

- [x] **Task 2: `mockapi` для новых операций** (depends on 1)
  - `mockapi/internal/handlers/admin_*.go`, `store/admin.go`: операции на данных в памяти — очередь и модерация заказов (одобрение двигает заказ как `_mock/approve-post`), размещения и ручные операции (через существующие переходы), споры (решение — существующий P-15 мок), переписка, выплаты и CSV, выписка (разбор не нужен: фиксированная сводка), поступления, счета, проводки (журнал из модели денег мока), подтверждения (порог как у бэкенда: `senior` второй — другой пользователь сида), организации, блокировка, журнал действий (что пишет мок), ошибки и отчёты ОРД (данные сида), счётчики. Роли: `moderator` и `senior` различаются (сид: второй администратор `mod@example.test` — `moderator`, `adm2@example.test` — `senior`).
  - `seed.go`: пользователи-администраторы, спор, выплата `failed`, поступление `unmatched`, ошибка ОРД, отчёт ОРД прошлого месяца.
  - `contract_test.go`: случай на каждую новую операцию; `gaps_test.go`: `moderator` на операции `senior` — `403 admin_required`; второе подтверждение — `202`, подтверждение тем же администратором — `409 same_admin`, другим — исполнено; переписка видна адресату.
  - `mockapi/README.md`: что реализовано из администратора.
  - Журнал: как в моке (`logx`), без текстов сообщений.
  - Файлы: `mockapi/internal/{handlers,store,seed,server}/*.go`, `mockapi/README.md`.
<!-- Commit checkpoint: tasks 1-2 -->

### Phase 2: Данные и общие правила

- [x] **Task 3: Миграция `00014_admin.sql` (R-2, R-7, R-9, R-12)**
  - `dispute_message` (`id`, `dispute_id → dispute`, `author_type check in (admin, advertiser, blogger)`, `author_user_id → user`, `recipient check in (advertiser, blogger, both)` — обязателен у `admin`, пуст у сторон, `text` 1…2000, `created_at`); индекс `(dispute_id, created_at)`; `REVOKE UPDATE, DELETE, TRUNCATE`.
  - `admin_approval` (`id`, `kind check in (ledger_manual, deposit_topup, payout_mark_paid)`, `payload jsonb`, `amount bigint > 0`, `reason` 1…1000, `idempotency_key unique`, `requested_by → user`, `status check in (pending, approved, rejected, expired)`, `decided_by`, `decided_at`, `decision_reason`, `result_id uuid`, `expires_at`, `created_at`, `updated_at`); проверки: решение ⇒ `decided_by`/`decided_at`, `decided_by <> requested_by`; индекс `(status, created_at)`; `REVOKE DELETE, TRUNCATE`.
  - `channel_category.archived_at timestamptz`.
  - `"user".blocked_at`, `blocked_reason`, `anonymized_at`.
  - Журнал: не применимо.
  - Тесты (`migrations` рядом с существующими): `up` → `down` → `up`; решение тем же администратором — отказ; сообщение администратора без адресата — отказ; роль приложения не может `UPDATE dispute_message`.
  - Файлы: `migrations/00014_admin.sql`, тест миграций.

- [x] **Task 4: Роли и общие помощники (R-1, R-14)** (depends on 1)
  - `internal/platform/httpx/auth.go`: `AdminAny = []string{"moderator","senior"}`, `AdminSeniorOnly = []string{"senior"}`, `IsSenior(p)`; `channel/httpapi` переходит на константы.
  - `platform/logx`: `MaskINN` уже есть; помощник `INNFor(p *Principal, inn)` — полный для `senior`, маска для `moderator` (R-14).
  - Журнал: отказ — WARN `admin access denied` (как сейчас).
  - Тесты: `auth_test.go` — `moderator` на `AdminSeniorOnly` — `403`; `INNFor`.
  - Файлы: `internal/platform/httpx/{auth.go,auth_test.go}`, `internal/channel/httpapi/admin.go`.
<!-- Commit checkpoint: tasks 3-4 -->

### Phase 3: Модерация, размещения, споры

- [x] **Task 5: Модерация поста (S-17, R-4)** (depends on 1, 4)
  - `placement/domain/ports.go`: `PostModeration{Approve(ctx, versionID, adminID) error; Reject(ctx, versionID, adminID, reason) error}`; `bridge.PlacementPostModeration` над `post/app`.
  - `placement/app/admin_orders.go`: `AdminOrders(ctx, filter)` (очередь: `moderation` по `campaign.date_from`, затем `moderation_started_at`/`updated_at`; `urgent` — до начала периода меньше 6 ч, A-40; `total`), `AdminOrder(ctx, id)` (заказ, размещения, версия поста с итоговым текстом — порт `PublicationContent` или `OrderView.Version`, креатив — порт `placement/domain.CreativeInfo` над `marking`), `ApproveOrderPost`, `RejectOrderPost` (заказ в `moderation`, иначе `409 invalid_transition`; причина обязательна). `placement/postgres`: выборка `ListOrdersForAdmin`.
  - `placement/httpapi/admin_orders.go`: обработчики (роль — обе).
  - Журнал: WARN `admin order post approved|rejected` (`order_id`, `version_id`, `admin_user_id`); `audit_log` пишет `post/app` (уже).
  - Тесты (`dbtest`): очередь по сроку, `urgent`; одобрение — версия `approved`, при `ORD_ENABLED=false` → `ready` и P-02 (как `dev post-version`); отказ — заказ `rejected`, удержания сняты; заказ не в `moderation` — `409`.
  - Файлы: `internal/placement/{domain/ports.go,app/admin_orders.go,app/admin_orders_test.go,postgres/queries/orders.sql,postgres/*.go,httpapi/admin_orders.go}`, `internal/bootstrap/bridge/placement.go`, `internal/bootstrap/placement.go`.

- [x] **Task 6: Блокировка канала и справочник категорий (R-7, R-8)** (depends on 3, 4)
  - `channel/domain`: `(c *Channel) Block(reason, now)`, `Unblock(now)` (в `moderation`); категория `Archived`.
  - `channel/app/moderation.go`: `Block(ctx, adminID, id, reason)`, `Unblock(ctx, adminID, id, reason)` — `audit_log` `channel.blocked|unblocked`, уведомление `channel.blocked` (шаблон `templates/notification/channel.blocked.tmpl`, каталог уведомлений); каталог и корзина уже фильтруют по `active`; проверка оформления — канал не `active` → как сейчас.
  - `channel/app/categories.go`: `CreateCategory`, `UpdateCategory` (`audit_log` `channel_category.created|updated`), `ListCategories` без архивных для кабинетов.
  - `channel/httpapi/admin.go`: `BlockChannel`, `UnblockChannel`, `CreateChannelCategory`, `UpdateChannelCategory` (только `senior`).
  - Журнал: WARN `admin channel blocked|unblocked` (`channel_id`, `admin_user_id`); WARN `channel category changed`.
  - Тесты: блокировка из `active` — нет в каталоге, текущее размещение продолжается; повторная — `409`; снятие — `moderation`; архивная категория не в списке кабинета; `moderator` — `403`.
  - Файлы: `internal/channel/{domain/channel.go,domain/category.go,app/moderation.go,app/categories.go,app/*_test.go,postgres/queries/*.sql,postgres/*.go,httpapi/admin.go}`, `internal/notification/domain/catalog.go`, `templates/notification/channel.blocked.tmpl`.

- [x] **Task 7: Размещения администратора и ручные операции (R-5, R-6)** (depends on 4)
  - `placement/app/admin_placements.go`: `AdminPlacements(ctx, filter{status, channel_id, order_id, from, to, overdue_removal, cursor, limit})` с `total`; `AdminPlacement(ctx, id)` — размещение, заказ, история, публикация, нарушения, спор (сводка), выплаты (порт `placement/domain.PayoutInfo` над `payout/app`), действия `available_admin_actions` (по состоянию и роли).
  - `CancelByAdmin(ctx, id, adminID, reason)` — P-06 `admin` (`ReasonAdminCancel`), `ReleaseHold`, `audit_log` `placement.admin_cancel`.
  - `MarkRemovedByAdmin(ctx, id, adminID, removedAt, reason)` — R-6: `publication.MarkRemoved(admin, removedAt)`, P-10 `admin`, `final_views`, отмена `remove_placement`, T-05 от `removed_at`, `audit_log` `placement.admin_mark_removed`.
  - `placement/domain`: `(p *Placement) MarkRemovedByAdmin(...)`, проверки времени (`ErrRemovedAtInvalid` → `422 removed_at`).
  - `placement/httpapi/admin_placements.go`: `ListAdminPlacements`, `GetAdminPlacement` (обе роли), `AdminCancelPlacement`, `AdminMarkRemoved` (`senior`), `AdminRemovePost`, `AdminOpenDispute` (обе — существующие сервисы).
  - Журнал: WARN `admin placement cancelled`, `admin placement marked removed` (`removed_at`); WARN отказы.
  - Тесты (`dbtest`): отмена из `scheduled` — удержание снято, задания отменены; из `published` — `409`; отметка снятия — `removed`, `final_views`, T-05 от времени снятия, без нарушения; время раньше публикации — `422`; фильтр «просрочено снятие».
  - Файлы: `internal/placement/{app/admin_placements.go,app/admin_placements_test.go,domain/publish.go,domain/publication.go,domain/ports.go,postgres/queries/placements.sql,postgres/*.go,httpapi/admin_placements.go,httpapi/convert.go}`, `internal/bootstrap/bridge/placement.go`.

- [x] **Task 8: Споры через HTTP и переписка (R-9 … R-11)** (depends on 3, 4, 7)
  - `placement/domain/dispute_message.go`: `DisputeMessage`, `NewAdminMessage(to)`, `NewPartyMessage(author)`, `VisibleTo(author_type)`; ошибки `ErrDisputeResolved` (уже), `ErrMessageText`.
  - `placement/postgres/queries/dispute_messages.sql`: вставка, список по спору; `disputes.sql`: `GetDisputeByID`, `ListDisputesAdmin` с `state` (`open|resolved|all`), курсором и `total`.
  - `placement/app/dispute_messages.go`: `PostAdminMessage(ctx, disputeID, adminID, to, text)`, `PostPartyMessage(ctx, orgID, userID, placementID, side, text)`, `Messages(...)` для администратора и сторон; уведомление `dispute.message` (шаблон, каталог; адресат — роль стороны или администраторы).
  - `placement/app/disputes.go`: `AdminDisputeByID`, `PreviewResolution(ctx, disputeID, resolution, share)` (через `ResolutionShare` и `Distribute`); метрика `Observer.DisputeResolutionTime(resolution, seconds)` — `maxim_dispute_resolution_seconds`.
  - `placement/httpapi/admin_disputes.go`, `orders.go`, `blogger.go`: операции споров и переписки.
  - Журнал: INFO `dispute message posted` (`dispute_id`, `author_type`, `recipient`; без текста); WARN `dispute resolved` (уже).
  - Тесты: решение по HTTP — те же суммы, что у команды (03 §7: 7500 → 7 500,00 / 1 125,00 / 2 875,00); предпросмотр без побочных эффектов; сообщение администратора блогеру не видно рекламодателю; ответ стороны после решения — `409 dispute_resolved`; чужое размещение — `404`.
  - Файлы: `internal/placement/{domain/dispute_message.go,domain/dispute_message_test.go,app/dispute_messages.go,app/dispute_messages_test.go,app/disputes.go,postgres/queries/{dispute_messages.sql,disputes.sql},postgres/*.go,postgres/metrics.go,httpapi/{admin_disputes.go,orders.go,blogger.go}}`, `internal/notification/domain/catalog.go`, `templates/notification/dispute.message.tmpl`.
<!-- Commit checkpoint: tasks 5-8 -->

### Phase 4: Деньги

- [x] **Task 9: Модуль `admin`: второе подтверждение (R-2, R-3, R-11)** (depends on 3)
  - `internal/admin/domain`: `Approval`, `Kind`, `Status`, `NewApproval(…, ttl)`, `(a *Approval) Approve(adminID, now)`, `Reject(adminID, reason, now)`, ошибки `ErrSameAdmin`, `ErrApprovalDecided`, `ErrApprovalExpired`; порты `Store`, `Executor{Execute(ctx, a Approval, approverID) (resultID uuid.UUID, err error)}` по виду, `Counters` (задача 14).
  - `internal/admin/app`: `RequestApproval(ctx, kind, key, payload, amount, adminID, reason)` (повтор ключа — тот же запрос), `List(ctx, status, cursor, limit)`, `Approve(ctx, id, approverID)` — блокировка строки, проверка, исполнение через `Executor` в той же транзакции, `audit_log` `admin_approval.approved`; `Reject` — `admin_approval.rejected`; просроченные при чтении — `expired`.
  - `internal/admin/postgres` (sqlc, запись в `sqlc.yaml`), `internal/admin/httpapi`: `ListAdminApprovals`, `ApproveAdminApproval`, `RejectAdminApproval` (`senior`).
  - `bootstrap/bridge/admin.go`: `AdminLedger` (виды `ledger_manual`, `deposit_topup` → `ledger/app.Manual|DepositTopup` с `ApproverID`), `AdminPayouts` (`payout_mark_paid` → `payout/app.MarkLegalPaid`); `bootstrap/admin.go` (модуль) и `ADMIN_APPROVAL_TTL` (72 ч) в `config.go`.
  - Метрика `maxim_admin_approvals_total{kind,decision}`.
  - Журнал: WARN `admin approval requested|approved|rejected` (`approval_id`, `kind`, `amount`, `admin_user_id`); INFO `admin approval expired`.
  - Тесты (`domain` и `dbtest`): свой запрос — `ErrSameAdmin`; повтор решения; срок; подтверждение исполняет проводку один раз (повтор после сбоя — по ключу).
  - Файлы: `internal/admin/{domain,app,postgres,httpapi}/*.go`, `internal/admin/postgres/queries/approvals.sql`, `sqlc.yaml`, `internal/bootstrap/{admin_module.go,config.go,modules.go}`, `internal/bootstrap/bridge/admin.go`.

- [x] **Task 10: Проводки и сверка через HTTP (S-19, R-2)** (depends on 9)
  - `ledger/httpapi/admin.go`: `GetLedgerSummary` (остатки по видам счетов, сверка с выпиской и депозитом — как `admin money reconcile`; чтение банка и депозита — порты `ledger/domain.ReconcileSources` над `billing/app.BankPosition` и `payout/app.DepositReport`), `ListLedgerEntries` (`Journal` с курсором, фильтры `kind`, `organization_id`, `account_kind`, `from`, `to`), `CreateManualLedgerEntry` (порог → `admin/app.RequestApproval`, иначе `Manual`), `CreateDepositTopup` (так же), `CreateBalanceReturn`.
  - `ledger/app`: курсор и `total` в `Journal`; `ManualAllowed(amount)`.
  - Журнал: WARN `manual ledger operation` (уже); WARN `admin approval requested`.
  - Тесты (`*_http_test.go`): 1 000 ₽ — `201`, проводка; 150 000 ₽ — `202`, проводки нет, подтверждение другим `senior` — проводка с `approver_id` в `audit_log`; повтор `Idempotency-Key`; `moderator` — `403`; сверка сходится.
  - Файлы: `internal/ledger/{httpapi/admin.go,app/admin.go,domain/ports.go,postgres/*.go}`, `internal/bootstrap/{ledger.go,bridge/ledger.go,ledger_http_test.go}`.

- [x] **Task 11: Реестр выплат через HTTP (R-2, R-14)** (depends on 9, 4)
  - `payout/app/admin.go`: `AdminPage(ctx, filter, cursor, limit)` с `total`, `AdminSummary(ctx)` (депозит — последний `DepositReport` без внешнего вызова: остаток журнала и последнее значение API из метрики/таблицы, в работе, не прошли, к выплате), `ExportCSV(ctx, filter, w)` (ИНН маскирован, `audit_log` `payout.exported`), `MarkLegalPaid` — проверка порога переносится в сервис (`ErrSecondApproval`).
  - `payout/httpapi/admin.go`: `ListAdminPayouts`, `GetAdminPayoutSummary`, `ExportAdminPayouts`, `RetryPayout`, `MarkPayoutPaid` (порог → подтверждение) — `senior`.
  - Журнал: WARN `admin payout retry` (уже), INFO `payouts exported` (`count`, `admin_user_id`).
  - Тесты: фильтры и `total`; CSV — заголовок, BOM, маска ИНН; `mark-paid` выше порога — `202`; `retry` не последней попытки — `409`.
  - Файлы: `internal/payout/{app/admin.go,app/export.go,app/*_test.go,httpapi/admin.go,postgres/queries/payouts.sql,postgres/*.go}`, `internal/bootstrap/{payout_http_test.go,admin_payout.go}`.

- [x] **Task 12: Оплаты и счета через HTTP (S-30)** (depends on 4)
  - `billing/httpapi/admin.go`: `GetLatestBankStatement` (итоги последней загрузки), `UploadBankStatement` (multipart, предел `STATEMENT_MAX_BYTES`, тело — `bodyLimit` в `server.go`), `ListBankTransactions` (`match_status`, курсор, `total`), `MatchBankTransaction`, `IgnoreBankTransaction`, `ListAdminInvoices` (`status`, `organization_id`, `number`), `CancelAdminInvoice` — `senior`.
  - `billing/app`: `LatestStatement`, `AdminInvoices`.
  - Журнал: как в сервисах (уже); WARN отказ размера.
  - Тесты: загрузка выписки «1С» по HTTP — итог совпадает с командой; повтор файла — `409 statement_already_imported`; сопоставление — зачисление и `audit_log`.
  - Файлы: `internal/billing/{httpapi/admin.go,app/admin.go,postgres/queries/*.sql,postgres/*.go}`, `internal/bootstrap/{server.go,billing_http_test.go}`.
<!-- Commit checkpoint: tasks 9-12 -->

### Phase 5: Организации, журнал, счётчики, ошибки ОРД

- [x] **Task 13: Организации и пользователи (R-12 … R-14)** (depends on 3, 4)
  - `account/app/admin_orgs.go`: `AdminOrganizations(ctx, query, role, cursor, limit)` (поиск по ИНН, наименованию, почте участника; роль — кабинет), `AdminOrganization(ctx, id)` (реквизиты, участники, кабинеты, статус ОРД контрагента — порт `account/domain.OrdStatus` над `marking`), `AdminUpdateOrganization(ctx, adminID, id, name, kpp, reason)` (проверки кабинета, `OrganizationListener`, `audit_log` `organization.admin_updated` с `audit.Diff`), `BlockUser`, `UnblockUser` (`audit_log` `user.blocked|unblocked`, сессии удаляются).
  - `account/httpapi/admin.go`: `ListAdminOrganizations`, `GetAdminOrganization` (обе роли, ИНН по R-14), `UpdateAdminOrganization`, `BlockUser`, `UnblockUser` (`senior`).
  - Журнал: WARN `admin organization updated` (`organization_id`, `fields`), WARN `user blocked|unblocked` (`user_id`).
  - Тесты: поиск по ИНН и почте; правка наименования ставит контрагента ОРД на повторную отправку; блокировка — сессия перестаёт работать (`401`); администратора — `409 admin_user`.
  - Файлы: `internal/account/{app/admin_orgs.go,app/admin_orgs_test.go,domain/ports.go,postgres/queries/*.sql,postgres/*.go,httpapi/admin.go}`, `internal/bootstrap/bridge/account.go`.

- [x] **Task 14: Журнал действий и счётчики (R-15, R-16)** (depends on 9)
  - `admin/postgres/queries/audit.sql` (чтение `audit_log` без `ip`), `admin/app/audit.go` (`AuditLog(filter, cursor, limit)`), `admin/app/counters.go` (порты `Counters` по модулям: `bridge.AdminCounters` над `channel`, `placement`, `marking`, `billing`, `payout`, `admin`).
  - `admin/httpapi`: `ListAuditLog` (`senior`), `GetAdminCounters` (обе).
  - Журнал: DEBUG `audit log listed`.
  - Тесты: фильтры `manual_only`, `action`, период; счётчики на сидах `dbtest`.
  - Файлы: `internal/admin/{app/audit.go,app/counters.go,postgres/queries/audit.sql,httpapi/*.go}`, `internal/bootstrap/bridge/admin.go`.

- [x] **Task 15: Ошибки ОРД через HTTP (S-20, R-17)** (depends on 4)
  - `marking/app/errors.go`: фильтр по виду; `Object(ctx, ref)` — состояние, последний вызов (`outbound_request` по `LastRequestID`: операция, статус, тело запроса и ответа), зависимые (креативы договора, площадки контрагента).
  - `marking/httpapi/admin.go` (новый слой модуля): `ListOrdErrors`, `GetOrdObject`, `RetryOrdObject` (обе роли).
  - Журнал: `Retry` пишет сам (уже).
  - Тесты: список и повтор по HTTP; объект без вызовов — без запроса.
  - Файлы: `internal/marking/{app/errors.go,postgres/queries/overview.sql,postgres/*.go,httpapi/admin.go}`, `internal/bootstrap/marking.go`.
<!-- Commit checkpoint: tasks 13-15 -->

### Phase 6: Отчётность ОРД

- [x] **Task 16: Миграция `00015_ord_reporting.sql` (R-18 … R-21)**
  - `publication_views_snapshot` (`publication_id → publication`, `period date` — первое число месяца, на начало которого снимок, `views integer`, `views_checked_at`, `created_at`; PK `(publication_id, period)`).
  - `ord_statistics` (поля синхронизации 04 §6.1, `creative_id → creative`, `ord_platform_id → ord_platform`, `placement_id`, `period`, `impressions >= 0`, `shown_from`, `shown_to`, `amount >= 0`; `unique (creative_id, ord_platform_id, period)`).
  - `ord_invoice` (синхронизация, `ord_contract_id → ord_contract`, `period`, `type`, `number unique`, `invoice_date`, `amount > 0`; `unique (ord_contract_id, period)`); `ord_invoice_item` (`ord_invoice_id`, `creative_id`, `ord_platform_id`, `amount >= 0`; `unique (ord_invoice_id, creative_id, ord_platform_id)`).
  - `ord_report` (`period date pk`, `built_at`, `built_by`, `send_requested_at`, `send_requested_by`, `updated_at`).
  - Индексы `(sync_status)`, `(period)`; права: `REVOKE TRUNCATE`; `DELETE` только у `ord_invoice_item` и неотправленных строк (через приложение).
  - Тесты миграции: `up`/`down`/`up`; уникальности.
  - Файлы: `migrations/00015_ord_reporting.sql`, тест миграций.

- [x] **Task 17: Клиент ОРД и `ordfake`: статистика и акты (R-22, допущения 1–3)** (depends on 16)
  - `integration/ord`: `OpStatistics = "statistics"`, `OpInvoice = "invoice"`, `OpInvoiceCreatives = "invoice/creatives"`; типы `Statistics`, `Invoice`, `InvoiceCreatives`; `UpsertStatistics`, `UpsertInvoice`, `UpsertInvoiceCreatives` (путь отдельно от имени операции); `recordBody` без изменений (ИНН в теле нет).
  - `ordfake`: новые методы, `advance` для `statistics`/`invoice` останавливается на `ORD success`; `FailNext` по новым операциям; `Objects()` их видит.
  - `bridge.OrdClient`, `bridge.MarkingOrd`: новые методы порта.
  - Журнал: INFO `ord object submitted` (уже в `submit`).
  - Тесты: `httptest` — путь `/api/v8/invoice/creatives`, тело по записанному образцу `testdata/{statistics,invoice,invoice_creatives}_request.json`; `ordfake` — шаги статуса.
  - Файлы: `internal/integration/ord/{client.go,objects.go,types.go,objects_test.go,testdata/*.json}`, `internal/integration/ord/ordfake/{fake.go,fake_test.go}`, `internal/bootstrap/bridge/marking.go`.

- [x] **Task 18: Правила отчёта в `marking/domain` (R-19, R-20)** (depends on 16)
  - `marking/domain/report.go`: `Period` (месяц по Москве), `ReportPlacement` (вход: размещение, креатив, площадка, договоры, `published_at`, `removed_at`, `final_views`, снимки начала и конца, суммы), `BuildStatistics([]ReportPlacement, period) []StatisticsLine`, `BuildInvoices(lines, contracts, prefix, period) []InvoiceDraft`, `InvoiceNumber(prefix, period, n)`.
  - `marking/domain/statistics.go`, `invoice.go`: сущности с `Sync SyncState`, `Ref()`, построители запросов и `PayloadHash`; типы `TypeStatistics`, `TypeInvoice` в `ObjectTypes`, `ParseObjectRef` без жёсткого списка в тексте; `PassesErir` — `false` для новых видов.
  - Журнал: не применимо.
  - Тесты (табличные): публикация внутри месяца; через границу (первый месяц — снимок конца минус 0; второй — итог минус снимок; сумма только во втором); снимок больше итога — 0; две публикации одной пары; даты по Москве на границе суток; акт рекламодателя и блогера; нулевые суммы не дают акта; номер акта.
  - Файлы: `internal/marking/domain/{report.go,report_test.go,statistics.go,invoice.go,sync.go,status.go}`.

- [x] **Task 19: Формирование, отправка и задания (R-18, R-21 … R-24)** (depends on 17, 18)
  - `marking/domain/ports.go`: `ReportSource{LivePublications(ctx, at) …; Placements(ctx, period) []ReportPlacement}` (`bridge.MarkingReportSource` над `placement/postgres`), репозитории `Statistics`, `Invoices`, `Reports`, `ViewsSnapshots`.
  - `marking/app/report.go`: `TakeSnapshot(ctx, period)`, `BuildReport(ctx, period, adminID *uuid.UUID)` (R-21: пересчёт, сохранение, `ord_report.built_at`, `audit_log` `ord_report.built` при ручном), `SendReport(ctx, period, adminID *uuid.UUID)` (`sync_ord_object` для `pending`/`error`), `Report(ctx, period)` и `Reports(ctx)` для экрана, `CheckReport(ctx, now)` (R-23).
  - `marking/app/records.go`, `sync.go`: записи `statisticsRecord`, `invoiceRecord`, ветки `lock`, `build` (зависимости: креатив с erid и площадка; договор и креативы разбивки), два вызова для акта (R-22).
  - `marking/postgres`: запросы новых таблиц, объединения `overview.sql` (ошибки, зависшие, счёт — по шести видам).
  - `marking/jobs`: `ord_monthly_report` (`job.MonthlyAt(1, 0, 5)`: снимок → формирование → отправка при `ORD_REPORT_AUTO_SEND`), `ord_report_check` (`job.DailyAt(10, 0)`).
  - `bootstrap/config_ord.go`: `ORD_INVOICE_TYPE_INITIAL`, `ORD_INVOICE_TYPE_PUBLISHER`, `ORD_INVOICE_NUMBER_PREFIX`, `ORD_REPORT_AUTO_SEND`, `ORD_REPORT_ERROR_DAY`; проверка перечислений.
  - Журнал: INFO `ord views snapshot taken`, INFO `ord report built`, INFO `ord report send requested` (`objects`), ERROR `ord report overdue` + оповещение `ord_report_overdue:{period}`, DEBUG `ord report object unchanged`.
  - Тесты (`dbtest`, `ordfake`): снимок только живых; формирование — строки и акты; повтор формирования без изменений — хеш тот же, строки не трогаются; правка после спора — `synced` → `pending`; отправка → `synced` после опроса; ошибка ОРД по статистике — в списке ошибок; проверка 20-го — оповещение; всё `synced` — закрыто.
  - Файлы: `internal/marking/{domain/ports.go,app/report.go,app/report_test.go,app/records.go,app/sync.go,app/service.go,postgres/queries/{report.sql,overview.sql},postgres/*.go,jobs/jobs.go}`, `internal/bootstrap/{marking.go,config_ord.go,config_ord_test.go,worker.go}`, `internal/bootstrap/bridge/marking.go`.

- [x] **Task 20: Операции и команды отчёта (R-21)** (depends on 19)
  - `marking/httpapi/admin.go`: `ListOrdReports`, `GetOrdReport` (обе роли), `RebuildOrdReport`, `SendOrdReport` (`senior`); срок сдачи в ответе — 30-е число следующего месяца (05 §3.7, «проверить с юристом»).
  - `bootstrap/admin_ord.go`: `maxim admin ord report show|build|send --period YYYY-MM [--email]` (изменяющие — `senior`).
  - Журнал: WARN `admin ord report rebuild|send` (`period`, `admin_user_id`) + `audit_log`.
  - Тесты: `*_http_test.go` — формирование и отправка по HTTP, `moderator` на `send` — `403`; команда `show`.
  - Файлы: `internal/marking/httpapi/admin.go`, `internal/bootstrap/{admin_ord.go,admin_ord_test.go,marking_http_test.go}`.
<!-- Commit checkpoint: tasks 16-20 -->

### Phase 7: ПДн и эксплуатация

- [x] **Task 21: Выгрузка и обезличивание ПДн (R-26)** (depends on 3, 13)
  - `account/app/personal_data.go`: `ExportSubject(ctx, email, operatorID) (SubjectData, error)`, `AnonymizeSubject(ctx, email, operatorID, reason) (AnonymizeResult, error)` — в одной транзакции; администратора обезличить нельзя (`admin_user`).
  - `account/postgres/queries/personal_data.sql`: выборки и обновления (почта, сессии, коды, контакты организаций единственного участника).
  - `bootstrap/admin_pd.go`: `maxim admin pd export --email … --operator-email …` (JSON в stdout), `maxim admin pd anonymize --email … --reason … --operator-email …` (`senior`).
  - Журнал: INFO `personal data exported` (`user_id`), WARN `personal data anonymized` (`user_id`, `organizations`); почта не пишется.
  - Тесты (`dbtest`): выгрузка содержит организации и принятия; после обезличивания вход по старой почте невозможен, ИНН и проводки на месте, `audit_log` записан; организация с двумя участниками — контакты не тронуты.
  - Файлы: `internal/account/{app/personal_data.go,app/personal_data_test.go,postgres/queries/personal_data.sql,postgres/*.go}`, `internal/bootstrap/{admin.go,admin_pd.go,admin_pd_test.go}`.

- [x] **Task 22: Подключение, переменные, edge-Caddy, порядок дежурного (R-25)** (depends on 9, 19)
  - `bootstrap`: модули `admin` и новые обработчики в `apiImpl`, порты и переходники; `wiring_test.go` — порты подключены; `bodyLimit` для выписки.
  - `.env.example`, `deploy/app/env/{staging,production}/app.env.example` (блок «API администратора и отчётность ОРД»): `ADMIN_APPROVAL_TTL`, `ORD_INVOICE_TYPE_INITIAL`, `ORD_INVOICE_TYPE_PUBLISHER`, `ORD_INVOICE_NUMBER_PREFIX`, `ORD_REPORT_AUTO_SEND`, `ORD_REPORT_ERROR_DAY`.
  - `deploy/app/maxim-app.caddy`: matcher `/api/v1/admin*` + `not remote_ip __ADMIN_ALLOW__` → `403` JSON; `deploy/app/release.sh` — подстановка из `EDGE_ADMIN_ALLOW`; `stack.env.example` обоих окружений и `deploy/local`; `deploy/scripts/check-env.sh` — `EDGE_ADMIN_ALLOW` в `STACK_KEYS` и `STACK_REQUIRED`; `bash deploy/scripts/check-env.sh` проходит.
  - `docs/launch/pilot-plan.md` (порядок дежурного: отчётность ОРД 1–5 и до 20-го, подтверждения), `deploy/app/README.md` (ссылка на порядок, команды `pd`, `ord report`, доступ к администраторскому API), `docs/launch/personal-data.md`.
  - Журнал: `config loaded` (общий вывод).
  - Тесты: `config_test.go` / `config_ord_test.go`; `ops_test.go` (если проверяет шаблоны и caddy).
  - Файлы: `internal/bootstrap/{modules.go,api.go,worker.go,config.go,config_ord.go,wiring_test.go,server.go}`, `.env.example`, `deploy/app/{maxim-app.caddy,release.sh,README.md}`, `deploy/app/env/*/{app,stack}.env.example`, `deploy/scripts/check-env.sh`, `docs/launch/{pilot-plan.md,personal-data.md}`.
<!-- Commit checkpoint: tasks 21-22 -->

### Phase 8: Проверка и документы

- [x] **Task 23: Сквозные тесты администратора и отчётности** (depends on 22)
  - `internal/bootstrap/admin_e2e_test.go`: вход `moderator` и `senior`; модерация поста по HTTP → `marking` → erid (`ordfake`) → `awaiting_blogger`; спор: претензия → переписка → решение по HTTP; ручная проводка выше порога: `202` → подтверждение другим `senior` → проводка, `check_ledger_balance` без оповещений; `moderator` на `senior`-операциях — `403`; блокировка канала — не в каталоге.
  - `internal/bootstrap/ord_report_e2e_test.go`: публикация через границу месяца; 1-е число (`clock.Manual`) — снимок, формирование, отправка; опрос — `synced`; акты обеих сторон; повтор формирования после спора — правка.
  - Файлы: `internal/bootstrap/{admin_e2e_test.go,ord_report_e2e_test.go,harness_test.go}`.

- [ ] **Task 24: Документы** (depends on 23)
  - 02 S-16…S-20, S-30, S-31 — «Как сделано (этап API администратора)»; 04 §2 (`blocked_at`, `anonymized_at`), §5 (`dispute_message`), §6 (новые таблицы, исправить разрыв таблицы), новая §8-подраздел `admin_approval`; 05 §3.6, §3.7 (снимок, состояния, задания, допущения); 07 §2 (`ord_monthly_report` — 1 попытка, `ord_report_check`), §4 (матрица ролей R-1, второе подтверждение через HTTP, доступ по IP), §5.5 (новые метрики), §8 (переменные); 08 — экраны `adm/*` и их операции; `docs/open-questions.md` — Q-42 … Q-47, R-24; `docs/frontend-api-gaps.md` — закрытые пункты MAXI-51, MAXI-52; README — команды `maxim admin pd …`, `maxim admin ord report …`; `.ai-factory/ARCHITECTURE.md` — модуль `admin`, слой `marking/httpapi`; план — «Решения при реализации».
  - Журнал: не применимо.
  - Файлы: `docs/{02-scenarios.md,04-domain-model.md,05-integrations.md,07-non-functional.md,08-screens.md,open-questions.md,frontend-api-gaps.md}`, `README.md`, `.ai-factory/ARCHITECTURE.md`.
<!-- Commit checkpoint: tasks 23-24 -->

## Решения при реализации

- D-1 (задача 1). `listOrdErrors` отдаёт `OrdObject` (та же схема, что `getOrdObject`, без `last_request`): отдельная `OrdErrorItem` не нужна. Схема `AdminDisputeBrief` — общая часть спора в карточке размещения и в списке споров.
- D-2 (задача 1). `ChannelCategoryInput` принимает `slug` (латиница, уникален), а не `id`: идентификатор создаёт сервер, `slug` нужен каталогу. Архив — существующий `channel_category.is_active` (миграция 00004), поле `archived_at` не добавляется.
- D-3 (задача 1). ИНН в `AdminOrganizationRef` всегда с маской (ссылки в списках размещений, споров, выплат); полный ИНН у `senior` — в `AdminOrganization*`, `BankTransaction.payer_inn`. Почта и телефон в `AdminOrganization` у `moderator` — с маской.
- D-4 (задача 2). `mockapi`: второй `senior` — `adm2@example.test`, `moderator` — `mod@example.test`; юрлицо-блогер `ООО «Тестовые медиа»` с приостановленным каналом 114 — выплата переводом; размещение 808 — выплата `failed`. `contract_test` передаёт заголовки (`Idempotency-Key`) через `opCase.header`.

## Критерии готовности этапа

- `go build ./...`, `go vet ./...`, `go test ./...` (PostgreSQL) и `cd mockapi && go test ./...` проходят; `sqlc generate` и `go generate ./...` не дают изменений; `bash deploy/scripts/check-env.sh` проходит.
- Каждая операция администратора есть в `api/openapi.yaml`, реализована в бэкенде (без `501`) и в `mockapi`; роль проверяется по матрице R-1.
- Ручная денежная операция выше порога исполняется только после подтверждения другим `senior`; оба — в `audit_log`.
- Отчёт ОРД за месяц формируется и отправляется заданием 1-го числа и вручную; суммы и показы — по 05 §3.6; ошибки видны в списке S-20; с 20-го — оповещение.
- Ни текст основания, ни текст переписки, ни почта/ИНН без маскирования не пишутся в журнал приложения; выгрузки ИНН маскируют.
- D-5 (задачи 5–8). Списки администратора — смещение в курсоре (`internal/platform/adminpage`, base64url `o:N`, `limit` 1…100, по умолчанию 20) и `total`; чтения — SQL с соединением таблиц модулей (как сверка). Ссылки на юрлицо и почту — `internal/platform/adminref` (ИНН всегда с маской). Действующее лицо audit_log администратора ставит `httpx.AdminContext`. Блокировка канала не вызывает `ChannelUnavailable` (S-25): начатые размещения продолжаются, блогер получает `channel.blocked`. Без маркировки (`ORD_ENABLED=false`) одобрение поста сразу выдаёт `erid` вида `dev-<номер заказа>`. Метрика `maxim_dispute_resolution_seconds{resolution}` — время от открытия до решения спора.
- D-6 (задачи 9–12, 14). Ключ запроса подтверждения и проводки HTTP — `http:<вид>:<admin_id>:<Idempotency-Key>`: у каждого администратора и вида свой, команды CLI не пересекаются. Исполнитель подтверждения получает ключ запроса как ключ операции, поэтому повтор после сбоя не создаёт второй проводки; сбой исполнения откатывает решение (запрос остаётся `pending`). Автор отклоняет свой запрос — это отзыв (`withdrawn` в `audit_log`). Просроченные `pending` переводятся в `expired` при чтении списка, счётчиков и при попытке решения. Проверка порога отметки выплаты перенесена в `payout/app.MarkLegalPaid` (`ErrSecondApproval`); проверка в команде CLI оставлена как ранний отказ. Модуль `admin` читает `audit_log` без `ip`; `approver_user_id` проводки — из записи журнала действий ручной операции. Порт `placement/httpapi.AdminPayouts` — обработчики `payout/httpapi` (moderator получает пустой список). `getLatestBankStatement` — последняя по времени загрузки (для сверки — по дате конца периода, как в команде).
- D-7 (задачи 13–15). Порты статуса ОРД в карточках (`account/httpapi.OrdCounterparty`, `placement/httpapi.AdminCreatives`) реализует `marking/httpapi.Handlers` и подключаются только при `ORD_ENABLED=true`; отдельного порта `account/domain.OrdStatus` нет — домен аккаунтов статус ОРД не использует. Блокировка: повтор ничего не меняет (без записи в журнал), администратора — `409 admin_user`; все сессии отзываются в той же транзакции. Правка реквизитов администратором — `organization.admin_updated` с основанием, проверки кабинета (КПП только у блогера-юрлица). `GetOrdObject` читает объект под блокировкой строки в короткой транзакции (как `Retry`) и последний `outbound_request` по `(object_type, ord_external_id)`; в списке ошибок `title` — `<вид>:<id>` без чтения объекта; поле `blocked` (зависимые объекты) не заполняется — оставлено на следующий этап.
- D-8 (задачи 16–20). `ORD_REPORT_AUTO_SEND` по умолчанию `false`: задание 1-го числа делает снимок и формирует отчёт, отправляет senior после просмотра (`POST …/send` или `maxim admin ord report send`). Акт — два вызова в одном шаге (`invoice`, затем `invoice/creatives`), опрос по квитанции второго (ПРЕДПОЛОЖЕНИЕ Q-42). Строка статистики, пропавшая при повторном формировании, не удаляется: неотправленная удаляется, отправленная обнуляется и снова ждёт отправки (ОРД хранит её по `id`). Сумма отправленного акта, ставшая нулевой, — только WARN (акт в ОРД исправляет человек). Креатив готов для отчётности, когда у него есть erid и он не в `error`; размещение без креатива или площадки пропускается с WARN `ord report placement without creative|platform`. Пустой отчёт (нет строк) считается отправленным. Счётчик S-20 «отчёт не отправлен» и задания отчётности подключаются только при `ORD_ENABLED=true`. Карточка отчёта читает зависимости через `GetForUpdate` вне транзакции (блокировка не держится). Сквозной тест — `internal/bootstrap/ord_report_http_test.go` (по расписанию, по HTTP, команда); отдельного `marking/app/report_test.go` нет — правила покрыты `marking/domain/report_test.go`, связка — сквозным тестом.
- D-9 (задачи 21–22). `maxim admin pd export|anonymize` — обе только `senior` (выгрузка содержит полный ИНН и контакты); API для этих операций не добавлялось (запросы субъектов редки, 152-ФЗ даёт 10 рабочих дней). Обезличивание: почта → `anonymized-<id>@anonymized.invalid`, блокировка с основанием `personal_data_request` (прежнее основание блокировки сохраняется), снять блокировку обезличенного нельзя (`Unblock` ничего не меняет); сессии завершаются и теряют IP и браузер, коды входа по прежней почте удаляются; у организации с единственным участником стираются `contact_name` и `phone` (пустые строки, NOT NULL); ИНН, наименование, подписант, реквизиты, принятия с IP и `audit_log` сохраняются (ПРЕДПОЛОЖЕНИЕ Q-33). Повтор по прежней почте — «не найден». Доступ к `/api/v1/admin*` на edge — `EDGE_ADMIN_ALLOW` в `stack.env` (CIDR через пробел или `private_ranges`; на репетиции — `private_ranges`, в шаблонах — адрес из документационной сети `203.0.113.10/32`, ПРЕДПОЛОЖЕНИЕ); `release.sh` без значения не выпускает; сайт проверен `caddy validate` (2.11.7) и запросом: `403` JSON на `/api/v1/admin*`, прочие пути — к api.
- D-10 (задача 23). Сквозные сценарии администратора разложены по файлам задач (`admin_placement_http_test.go` — модерация поста → erid, споры по HTTP, блокировка канала; `admin_money_http_test.go` — проводка выше порога и подтверждение другим senior; `admin_orgs_http_test.go`; `ord_report_http_test.go` — отчёт по расписанию, отправка, `synced`, акты обеих сторон, повтор формирования); `admin_e2e_test.go` проверяет матрицу ролей по контракту: каждая операция `/admin/*` объявляет роль в описании, без сессии — 401, пользователь кабинета — 403, moderator на «Роль: senior» — 403 (тело — пример из контракта). Найдено и исправлено: пользователь без роли получал 422 с полями тела раньше 403 — добавлен `httpx.AdminGate` перед проверкой по спецификации; у пяти операций каналов (этап «Каталог») не была указана роль — дописано `Роль: moderator, senior` (apigen и mockapi перегенерированы).
