# План реализации: Приём оплаты и учёт денег

Branch: master (без отдельной ветки, по решению пользователя)
Created: 2026-10-20

## Settings
- Testing: yes. Правила проводок (счета и их стороны, допустимые пары счетов по каждому виду 06 §2.3, запрет отрицательного остатка пассива, отпечаток транзакции, ключи идемпотентности), формулы 03 §7 и 06 §3 (доли 0, 1, 9999, 10000, суммы с остатком при округлении), состояния счёта на оплату, сопоставление поступлений (05 §5), период и номер закрывающих документов — чистые табличные тесты `ledger/domain`, `billing/domain`, `placement/domain`. Разбор выписки «1С:Клиент-банк» — на образцах в `internal/integration/bank/clientbank/testdata` (windows-1251). Сценарии `ledger/app` и `billing/app` с `postgres` — на настоящей PostgreSQL через `testcontainers-go` с `clock.Manual` и `storagefake`. После каждой строки 06 §2.3 проверяется сходимость (07 §9). Сквозные сценарии — на собранных `api` и `worker`.
- Logging: verbose. `log/slog` JSON, события 06 §11, 05 §5, 07 §5.3: проводка создана (INFO: вид, основание, суммы в копейках, виды счетов), отклонена (WARN: отрицательный остаток, повтор ключа с другим составом), ручная денежная операция (WARN + `audit_log`), расхождение сходимости (ERROR + оповещение), счёт выставлен, истёк (INFO), выписка загружена, поступление зачислено (INFO), не сопоставлено (WARN), ручное сопоставление (WARN + `audit_log`), закрывающие документы сформированы (INFO). Поля `ledger_kind`, `ledger_transaction_id`, `idempotency_key`, `organization_id`, `placement_id`, `invoice_id`, `bank_transaction_id`, `bank_statement_id`, `amount`, `job_id`. ИНН — только маскированным; наименования организаций, назначение платежа, банковские реквизиты и номера счетов плательщика выше DEBUG в журнал не пишутся (07 §5.4, 06 §11).
- Docs: yes. В конце обязательная точка `/aif-docs`: README бэкенда (переменные реквизитов платформы и счёта, путь «выставить счёт → загрузить выписку → баланс пополнен → оформить заказ», команды `maxim admin money …`, где лежат PDF).

## Roadmap Linkage
Milestone: "Приём оплаты и учёт денег"
Rationale: план реализует этап целиком: журнал проводок с ключами идемпотентности (06 §2), счета и сопоставление поступлений из выписки (S-06, S-30, 05 §5), настоящие удержания по размещениям вместо `bridge.UnlimitedHolds` (Q-17, R-2 этапа заказов), проверку сходимости (06 §2.2 п. 3), отчёт агента и акт (06 §2.5). Выплаты (`payout`, `deposit_topup`, депозит Рокет Ворк) — следующий этап; распределение удержания при приёмке и споре (P-12, P-13, P-15) — этап «Приёмка, споры и возвраты»: здесь журнал умеет эти проводки и они покрыты тестами, но вызываются позже.

## Research Context
Source: .ai-factory/RESEARCH.md (Active Summary), выдержка по этапу

- Выбрана схема A: свой расчётный счёт ИП + агентский договор + внутренние балансы (A-03, A-04). Схемы «Безопасная сделка» ЮKassa (только карты) и номинальный счёт отложены (Q-11).
- Оплата только от юрлиц, ИП и самозанятых переводом по счёту: касса и эквайринг не нужны (A-05, Q-32 — проверить с юристом).
- Модель баланса: счёт в PDF на пополнение, заказы оплачиваются с баланса (A-06, Q-17); ежемесячно акт и отчёт агента; ЭДО нет (A-20).
- Деньги — копейки в `int64`, журнал только на добавление.
- Банк не выбран (Q-26): выписка — файлом в формате «1С:Клиент-банк» (A-35), разбор — адаптер за интерфейсом «список поступлений».

## Источники ТЗ

- Сценарии: 02 S-06 (пополнение и оплата, `insufficient_balance`, истечение счёта 14 суток), S-07, S-17 шаг 4, S-29 (снятие и повторное удержание), S-19 (ручная проводка с основанием), S-30 (зачисление оплаты), S-33 снят (D-26) — `balance_return` только администратором.
- Размещение: 03 §4 (столбец «Деньги», P-02 «есть активное удержание»), §7 (распределение), §10 (I-03, I-05, I-10, I-12), §11.
- Данные: 04 §1 (деньги `bigint`, неизменяемые таблицы правами роли), §2 (`organization.bank_*`), §7 (`invoice`, `bank_statement`, `bank_transaction`, `ledger_account`, `ledger_transaction`, `ledger_entry`, `hold`, `closing_act`), §8 (`audit_log`).
- Интеграции: 05 §5 (счёт, выписка, алгоритм сопоставления).
- Финансы и право: 06 §1, §2.1–§2.5, §3, §11.
- Требования: 07 §2 (`check_ledger_balance`, `generate_closing_acts`), §3 (сходимость раз в час), §4 «Деньги» (суммы только на сервере, ключи идемпотентности, роль `senior`, второй администратор для `manual` свыше 100 000 ₽, ограничение частоты счетов), §5.3–§5.5, §9 (проводки только через один модуль, обязательные тесты).
- Реестр: A-03, A-04, A-05, A-06, A-20, A-30, A-35, D-24, D-26, Q-02, Q-11, Q-17, Q-26, Q-32.
- Архитектура: `.ai-factory/ARCHITECTURE.md` — модули `ledger/` (единственный писатель `ledger_*` и `hold`) и `billing/` (счета, выписки, сопоставление, закрывающие документы); `platform/money.Kopecks`; «Порты к будущим модулям».
- Экраны: `docs/wireframe/index.html` — `adv/balance`, `adm/payments`, `adm/ledger`.
- Существующий код: `internal/placement/domain/{ports.go (Holds, HoldRequest),errors.go (InsufficientBalanceError),actions.go (SendToBlogger),placement.go,transitions.go,status.go}`, `internal/placement/app/{checkout.go,transition.go,versions.go,service.go}`, `internal/placement/placementtest` (поддельные удержания), `internal/bootstrap/bridge/placement.go` (`UnlimitedHolds`), `internal/bootstrap/{placement.go,placement_config_test.go,placement_http_test.go,modules.go,integration.go,config.go,config_ord.go,admin.go,admin_ord.go,dev.go,unimplemented.go}`, `internal/account/{domain/organization.go,app/platform.go,postgres/platform.go}`, `internal/platform/{money,storage,job/schedule.go (Every, MonthlyAt),stuck,alert,audit,ratelimit,metrics,tx}`, `migrations/00007_orders_placements.sql` (суммы размещения, `placement_amounts_immutable`), `api/openapi.yaml` (тег `balance`: `getBalance`, `listBalanceTransactions`, `listInvoices`, `createInvoice`, `getInvoicePdf`, `_mock/invoices/{id}/pay`), `mockapi/internal/{store/money.go,handlers/orders.go,handlers/mock.go}`.

## Границы этапа

Входит:
- Модуль `ledger`: счета учёта, транзакции и проводки, удержания (`hold`), остатки, все виды 06 §2.3 на уровне журнала (включая `settlement`, `payout`, `deposit_topup`), проверка сходимости.
- Настоящая реализация порта `placement/domain.Holds` вместо `UnlimitedHolds`; проверка «есть активное удержание» в P-02.
- Формулы распределения 03 §7 в `placement/domain` и операция журнала `Settle` (вызов из P-12/P-13/P-15 — на этапе приёмки).
- Модуль `billing`: счета на оплату (PDF), их истечение, загрузка выписки «1С:Клиент-банк», сопоставление и зачисление (S-30), ручное сопоставление и пометка «постороннее», отчёт агента и акт за месяц (PDF).
- Операции кабинета рекламодателя: `getBalance`, `listBalanceTransactions`, `listInvoices`, `createInvoice`, `getInvoicePdf`, новые `listClosingActs`, `getClosingActPdf`.
- До API администратора — служебные команды `maxim admin money …`: загрузка выписки, разбор несопоставленных, ручная проводка, возврат остатка, отмена счёта, сверка.
- Метрики, оповещения, конфигурация реквизитов платформы.

Не входит (следующие этапы):
- Выплаты, `payout`, `rocketwork_contractor`, депозит и его контроль, команда `deposit_topup` (этап «Выплаты блогерам через Рокет Ворк»). Вид проводки `deposit_topup` и счета `rocketwork_deposit`, `payout_fees` в журнале уже есть.
- Вызов распределения из P-12, P-13, P-15 (этап «Приёмка, споры и возвраты»).
- Экраны и операции администратора в OpenAPI (`adm/payments`, `adm/ledger`, реестр выплат), подтверждение вторым администратором через интерфейс (этап «API администратора»). Здесь второй администратор — параметр команды (R-14).
- Уведомления «Баланс пополнен», «Сформированы закрывающие документы»: порт `Notifier` остаётся `NopNotifier`, коды шаблонов — R-17.
- API банка, ЭДО, касса, сбор 3% (Q-26, A-20, Q-32, Q-27).
- Проверка принятия оферт при оформлении заказа (S-06, предусловие) — не денежная, остаётся отдельным пунктом (R-18).

## Решения этапа, требующие подтверждения

Документы этих мест не определяют или противоречат друг другу. План реализует указанный вариант. После подтверждения задача 15 вносит его в документы. При отказе меняется только отмеченная задача.

| # | Вопрос | Решение в плане | Задача |
|---|---|---|---|
| R-1 | Формат номера счёта на оплату | `MX-` и шесть цифр из `invoice_number_seq` с 1 (05 §5): по нему работает поиск `MX-\d{6}` в назначении платежа. Пример в OpenAPI (`СЧ-2026-000101`) и номер в `mockapi` приводятся к `MX-000101`. После `MX-999999` — семь цифр, шаблон поиска `MX-\d{6,}` | 3, 9 |
| R-2 | Отпечаток транзакции и повтор ключа | `ledger_transaction.entries_hash` — SHA-256 канонического списка проводок (вид, счета по виду и разрезу, суммы; порядок проводок фиксирован правилом вида). Повтор ключа с тем же отпечатком — no-op, возвращается существующая транзакция; с другим — `ErrIdempotencyConflict`, WARN `ledger transaction rejected` (`reason = key_conflict`) | 1, 2, 4 |
| R-3 | Ключи идемпотентности | `deposit:{bank_transaction_id}`, `hold:{placement_id}:{post_version_id}`, `hold_release:{placement_id}:{post_version_id}` (версия — из строки активного удержания, поэтому в `hold` добавляется `post_version_id`), `settlement:{placement_id}`, `payout:{payout_id}`, `balance_return:{uuid команды}`, `manual:{uuid команды}`, `deposit_topup:{uuid}`. Для ручных операций ключ задаёт вызывающий (`--key`, по умолчанию новый UUIDv7, печатается для повтора) | 2, 12 |
| R-4 | Допустимые пары счетов | Правило вида из 06 §2.3 зашито в `ledger/domain`: `deposit` — только `bank → advertiser_balance`; `hold` — `advertiser_balance → placement_hold`; `hold_release` — `placement_hold → advertiser_balance`; `settlement` — 2 или 3 проводки с `placement_hold`; `payout` — 1–2 проводки; `deposit_topup`, `balance_return` — одна; `manual` — любые разные счета. Нарушение — `ErrInvalidPosting` (ошибка программы, ERROR) | 2 |
| R-5 | Блокировки и I-10 | Журнал работает только в транзакции вызывающего (`tx.FromContext`; без транзакции — `ErrNoTransaction`). Строки счетов блокируются `SELECT … FOR UPDATE` в порядке `id` после блокировок вызывающего модуля (размещения: каналы → заказ → размещения → счета учёта). Счёт создаётся при первом обращении `INSERT … ON CONFLICT DO NOTHING` + повторное чтение с блокировкой | 4 |
| R-6 | Удержания и «снять» | `hold` получает `post_version_id`. `HoldAll` в одной транзакции: пропуск размещений с активным удержанием, сумма остальных против `advertiser_balance` — при нехватке ничего не пишется, `InsufficientBalanceError{Missing}` (общая нехватка по всем). `Release` снимает остаток `placement_hold` целиком, `hold.status = released`; без активного удержания — no-op. После снятия и распределения в той же транзакции проверяется нулевой остаток `placement_hold` (06 §2.3, I-12), иначе откат и ERROR `ledger invariant violated` | 5 |
| R-7 | P-02 «есть активное удержание» | Порт `Holds` дополняется `HasActive(ctx, placementIDs) (map[uuid.UUID]bool)`; `placement/app` передаёт признак в `domain.SendToBlogger`; без удержания P-02 не выполняется, размещение остаётся `created`, ERROR `placement without active hold` (нарушение I-03 — признак ошибки в коде, а не сценарий) | 5 |
| R-8 | Распределение удержания | `placement/domain.Distribute(blogger, fee, total, share) (payout, feeCharged, refund)` по 03 §7 и проверка I-12. `ledger/app.Settle(ctx, placementID, Distribution)` — `settlement` (проводка возврата только при `refund > 0`, нулевые суммы не проводятся), `hold.status = distributed`. Порт для `placement` (`Holds.Settle`) вводится здесь, вызов из P-12/P-13/P-15 — на этапе приёмки. Миграция добавляет к `placement` проверку I-12 для `accepted`, `paid`, `refunded` | 1, 2, 5 |
| R-9 | Что видит рекламодатель в операциях | `listBalanceTransactions` — проводки, затрагивающие `advertiser_balance` организации: кредит со знаком «+», дебет «−». Вид для кабинета: `deposit`, `hold`, `hold_release`, проводка возврата из `settlement` → `refund`, `balance_return`, `manual` (два новых значения перечисления `BalanceTransaction.kind`). Распределение в пользу блогера и платформы — не движение свободного баланса и в списке не показывается. `getBalance`: `available_amount` — остаток `advertiser_balance`, `held_amount` — сумма остатков `placement_hold` активных удержаний организации | 9 |
| R-10 | Счёт: PDF и реквизиты | PDF строится при выставлении (`go-pdf/fpdf` + встроенные шрифты Go `gofont` — кириллица есть, лицензия BSD, как у плашки), кладётся в закрытую область хранилища до вставки строки; при ошибке вставки файл остаётся сиротой (безвредно). Реквизиты получателя — строка платформы (`organization.is_platform`, наименование и ИНН из `ORD_PLATFORM_*`) плюс новые `PLATFORM_OGRNIP`, `PLATFORM_ADDRESS`, `PLATFORM_BANK_ACCOUNT`, `PLATFORM_BANK_NAME`, `PLATFORM_BANK_BIK`, `PLATFORM_BANK_CORR_ACCOUNT` (обязательны в `production`, проверка БИК 9 цифр и счетов 20 цифр). Строка НДС — `INVOICE_VAT_NOTE` (по умолчанию «Без НДС», Q-32). Плательщик — снимок `name`, `inn`, `bank_*` на момент выставления (D-26), хранится в `invoice` (`payer_*`) | 1, 7, 8 |
| R-11 | Истечение и отмена счёта | Периодическое задание `expire_invoices` раз в час (`INVOICE_EXPIRE_INTERVAL`): `issued` с `expires_at ≤ now` → `expired`. Поступление по `expired` счёту зачисляется (S-06) → `paid`. `cancelled` — только командой администратора из `issued`/`expired` с основанием; по отменённому счёту поступление не сопоставляется автоматически (уходит в `unmatched`). Ограничение выставления — `INVOICE_RATE_PER_USER` (10 в час) и `INVOICE_RATE_PER_IP` (30 в час) через `platform/ratelimit`, `429` | 8 |
| R-12 | Выписка и остаток для сверки с банком | `bank_statement` дополняется `account_number`, `date_from`, `date_to`, `opening_balance`, `closing_balance`, `file_sha256` (уникально: повторная загрузка того же файла — отказ `statement_already_imported`), счётчики (`incoming`, `matched`, `unmatched`, `duplicates`). Берутся только поступления на `PLATFORM_BANK_ACCOUNT` (`ПолучательСчет`); выписка по другому счёту — отказ. `external_id` (в формате 1С уникального идентификатора нет) — SHA-256 от `ДатаПоступило|Номер|Сумма|ПлательщикИНН|ПлательщикСчет`; номер счёта плательщика в `bank_transaction.payer_account` | 1, 6, 10 |
| R-13 | Ручное сопоставление и «постороннее» | `unmatched` → `matched` (команда: организация, необязательно счёт, основание) — `deposit` на фактическую сумму, `matched_by`, WARN + `audit_log`; `unmatched` → `ignored` с основанием (`ignored_reason`). `matched` назад не возвращается: ошибочное зачисление исправляется `manual` (06 §2.2 п. 5). `ignored` → `matched` разрешено | 10 |
| R-14 | Ручные операции до API администратора | Команды требуют `--email` администратора с ролью `senior` и `--reason`. `manual` с суммой больше `LEDGER_MANUAL_SECOND_APPROVAL_THRESHOLD` (100 000 ₽) требует `--approver-email` другого администратора `senior`; оба записываются в `audit_log` (07 §4). `balance_return` — не больше остатка `advertiser_balance` | 12 |
| R-15 | Сходимость | Задание `check_ledger_balance` раз в час (`max_attempts` 1, после простоя — один запуск): (1) сумма остатков активов = сумма остатков пассивов по проводкам; (2) кеш `ledger_account.balance` = сумма проводок по каждому счёту; (3) активное удержание ⇔ остаток `placement_hold` = `hold.amount`, закрытое ⇒ 0; (4) остаток пассива ≥ 0. Расхождение — ERROR и оповещения `ledger_imbalance`, `ledger_cache_mismatch:{account_id}`, `ledger_hold_mismatch:{placement_id}`; при восстановлении — закрытие оповещения. Сверка по всему журналу (объёмы 07 §7 позволяют) | 11 |
| R-16 | Закрывающие документы | Один `closing_act` на организацию и месяц, два PDF: отчёт агента и акт — `report_pdf_storage_key`, `act_pdf_storage_key` (вместо одного `pdf_storage_key`). Номер — `А-` и шесть цифр из `closing_act_number_seq`. Период — месяц по Москве, в отчёт попадают размещения с транзакцией `settlement` в этом месяце; организации без них документов не получают. Задание `generate_closing_acts` — 1-го числа в 03:00 МСК (`MonthlyAt`), 3 попытки; срок «до 5-го» (06 §2.5) соблюдается с запасом. Повтор — по уникальной паре `(organization_id, period)`: готовые пропускаются. Команда `maxim admin money closing-acts --period YYYY-MM` для ручного запуска | 13 |
| R-17 | Уведомления | Порт `Notifier` (`NopNotifier`): коды `balance.deposited` (рекламодателю при зачислении), `closing_acts.ready`. Оповещения администратору: ERROR `ledger_imbalance`, `ledger_cache_mismatch`, `ledger_hold_mismatch`; WARN `bank_transaction_unmatched:{id}` | 10, 11, 13 |
| R-18 | Оферты при оформлении | Не входит в этап: предусловие S-06 «приняты актуальные документы» не денежное. Записывается в открытые вопросы этапа «Юридическая готовность» | 15 |
| R-19 | Новые операции кабинета | В OpenAPI (тег `balance`): `GET /closing-acts` (`listClosingActs`: период, номер, `placements_amount`, `fee_amount`, ссылки) и `GET /closing-acts/{id}/{document}` (`getClosingActPdf`, `document` ∈ `report`, `act`). `mockapi` получает такие же операции на своих данных, чтобы собиралась и проходила контрактный тест | 9, 13 |

## Commit Plan
- **Commit 1** (после задач 1–3): `feat(ledger): money schema, posting rules and billing domain`
- **Commit 2** (после задач 4–7): `feat(ledger): journal, holds, statement parser and pdf documents`
- **Commit 3** (после задач 8–10): `feat(billing): invoices, balance api and statement matching`
- **Commit 4** (после задач 11–13): `feat(ledger): convergence check, admin money commands, closing acts`
- **Commit 5** (после задачи 14): `feat(billing): config and wiring`
- **Commit 6** (после задач 15–16): `test(billing): money e2e, docs sync`

## Tasks

### Phase 1: Данные и правила

- [x] **Task 1: Миграция денег (R-2, R-3, R-6, R-8, R-10, R-12, R-16)**
  - `migrations/00010_money.sql` по 04 §7:
    - `ledger_account` (`kind check in (bank, rocketwork_deposit, payout_fees, advertiser_balance, placement_hold, blogger_payable, platform_revenue)`, `organization_id`, `placement_id`, `balance bigint`, `UNIQUE NULLS NOT DISTINCT (kind, organization_id, placement_id)` — PostgreSQL 17; `check`: разрез соответствует виду — организация только у `advertiser_balance`, `blogger_payable`, размещение только у `placement_hold`; пассив `balance >= 0`).
    - `ledger_transaction` (`kind check in (deposit, hold, hold_release, settlement, payout, deposit_topup, balance_return, manual)`, `placement_id`, `invoice_id`, `bank_transaction_id`, `payout_id` (без внешнего ключа до этапа выплат), `actor_type check in (system, admin, advertiser)`, `actor_user_id`, `reason` — обязателен для `manual`, `balance_return`, `idempotency_key unique`, `entries_hash`).
    - `ledger_entry` (`debit_account_id <> credit_account_id`, `amount > 0`); индексы `(debit_account_id, created_at)`, `(credit_account_id, created_at)`.
    - `hold` (`placement_id`, `post_version_id`, `amount > 0`, `status check in (active, released, distributed)`, `closed_at`; частичный уникальный индекс `(placement_id) WHERE status = 'active'` — I-03).
    - `invoice` (`number unique`, `organization_id`, `amount`, `status check in (issued, paid, expired, cancelled)`, `expires_at`, `paid_at`, `paid_amount`, `payer_name`, `payer_inn`, `payer_bank_account`, `payer_bank_name`, `payer_bank_bik`, `payer_bank_corr_account`, `pdf_storage_key`, `created_by`, `cancel_reason`); `invoice_number_seq`; индексы `(organization_id, created_at DESC, id DESC)`, `(status, expires_at)`.
    - `bank_statement` (`uploaded_by`, `storage_key`, `file_sha256 unique`, `account_number`, `date_from`, `date_to`, `opening_balance`, `closing_balance`, счётчики).
    - `bank_transaction` (`external_id unique`, `operation_date`, `amount > 0`, `payer_inn`, `payer_name`, `payer_account`, `purpose`, `match_status check in (matched, unmatched, ignored)`, `invoice_id`, `organization_id`, `matched_by`, `ignored_reason`; `check`: `matched` ⇒ `organization_id` задан); индекс `(match_status, operation_date)`.
    - `closing_act` (`organization_id`, `period date` — первое число месяца, `number unique`, `placements_amount`, `fee_amount`, `report_pdf_storage_key`, `act_pdf_storage_key`, `UNIQUE (organization_id, period)`); `closing_act_number_seq`.
    - `placement`: `check` I-12 — при `status in (accepted, paid, refunded)` суммы распределения заданы и `payout_amount + fee_charged + refund_amount = total_amount`.
  - Права роли: `REVOKE UPDATE, DELETE` на `ledger_transaction`, `ledger_entry`; `REVOKE DELETE` на `ledger_account`, `hold`, `invoice`, `bank_statement`, `bank_transaction`, `closing_act`.
  - Журнал: не применимо (INFO `migrations applied` пишет `migrate`).
  - Тесты: `up` → `down` → `up`. Два активных удержания размещения не вставляются; два счёта учёта `bank` (оба без разреза) не вставляются; отрицательный остаток пассива не проходит; роль приложения не может `UPDATE ledger_entry`; проводка с одинаковыми счетами не проходит; `accepted` с несходящимися суммами не проходит.
  - Файлы: `migrations/00010_money.sql`.

- [x] **Task 2: `ledger/domain` и формулы распределения (R-2, R-3, R-4, R-8)** (depends on 1)
  - `internal/ledger/domain/account.go`: `AccountKind`, `Side` (`Asset`, `Liability`), `AccountRef{Kind, OrganizationID, PlacementID}` с проверкой разреза; `Delta(side, debit bool, amount)`.
  - `internal/ledger/domain/transaction.go`: `Kind`, `Posting{Debit, Credit AccountRef, Amount money.Kopecks}`, `Transaction{Kind, Basis{PlacementID, InvoiceID, BankTransactionID, PayoutID}, Actor, Reason, IdempotencyKey, Postings}`; `Validate()` по правилам вида (R-4) и обязательности основания и причины; `EntriesHash()` (R-2).
  - `internal/ledger/domain/builders.go`: `Deposit(bankTxID, orgID, amount)`, `Hold(placementID, versionID, orgID, amount)`, `HoldRelease(...)`, `Settlement(placementID, advertiserOrgID, bloggerOrgID, Distribution)`, `Payout(...)`, `DepositTopup(...)`, `BalanceReturn(...)`, `Manual(...)` — с ключами R-3.
  - `internal/ledger/domain/balance.go`: `ApplyPostings(balances map[AccountRef]Kopecks, postings) (new, error)` — `ErrNegativeLiability{Account, Missing}`; суммарная нехватка по `advertiser_balance` для `HoldAll`.
  - `internal/ledger/domain/ports.go`: репозитории `Accounts`, `Transactions`, `Holds`; ошибки `ErrIdempotencyConflict`, `ErrInvalidPosting`, `ErrNoTransaction`, `ErrNotFound`.
  - `internal/placement/domain/distribution.go`: `Distribute(blogger, fee, total, share) (Distribution, error)` по 03 §7, проверка `0 ≤ share ≤ 10000` и I-12; `SuggestedShare` для `removed_early` и `top_violated` не делается (этап споров).
  - Журнал: не применимо (`domain` не пишет журнал).
  - Тесты (табличные): каждая строка 06 §2.3 — валидна, перестановка дебета и кредита — `ErrInvalidPosting`; `manual` без основания — отказ; отпечаток не зависит от порядка полей и меняется от суммы; `ApplyPostings` пассива в минус — отказ с недостающей суммой, актива в минус — разрешено; `Distribute` на долях 0, 1, 5000, 7500, 9999, 10000 и цене 10 000,01 ₽ — сумма сходится, остаток округления у рекламодателя; пример 06 §2.3 (11 500 → 7 500 / 1 125 / 2 875).
  - Файлы: `internal/ledger/domain/{account.go,transaction.go,builders.go,balance.go,ports.go,errors.go,*_test.go}`, `internal/placement/domain/{distribution.go,distribution_test.go}`.

- [x] **Task 3: `billing/domain` — счёт, сопоставление, закрывающие документы (R-1, R-11, R-12, R-13, R-16)** (depends on 1)
  - `internal/billing/domain/invoice.go`: `Invoice`, состояния и переходы (`issued → paid|expired|cancelled`, `expired → paid|cancelled`), `FormatNumber(seq)` (R-1), `NumberPattern`, пределы суммы (1 000 ₽ … 1 000 000 000 ₽ — как в `InvoiceCreate`), `ExpiresAt(now, ttl)`.
  - `internal/billing/domain/statement.go`: `Incoming{ExternalID, OperationDate, Amount, PayerINN, PayerName, PayerAccount, Purpose}`, `Statement{Account, DateFrom, DateTo, Opening, Closing, Incoming []Incoming}`, порт `StatementParser` («список поступлений», 05 §5).
  - `internal/billing/domain/match.go`: `Match(in Incoming, byNumber *Invoice, byINNAndAmount []Invoice) MatchResult` — шаги 2–4 05 §5 (шаг 1 — уникальность `external_id` в хранилище); `cancelled` не сопоставляется (R-11).
  - `internal/billing/domain/closing.go`: `Period(month)` по Москве (границы `[1-е 00:00 МСК, 1-е след. месяца)`), `PreviousPeriod(now)`, `FormatActNumber(seq)`, строки отчёта агента и итоги.
  - `internal/billing/domain/ports.go`: репозитории `Invoices`, `Statements`, `BankTransactions`, `ClosingActs`; порты `Ledger` (зачисление, ручные операции, выборка settlement за период), `Organizations` (реквизиты, ИНН, признак платформы), `Renderer` (PDF), `Storage`, `Notifier`.
  - Журнал: не применимо.
  - Тесты (табличные): номер `MX-000001`, `MX-999999`, `MX-1000000`; поиск номера в назначении с пробелами, в нижнем регистре, рядом с текстом (`оплата по счету №MX-000123 без НДС`); номер найден, но счёт `cancelled` → дальше по ИНН; два `issued` с тем же ИНН и суммой → `unmatched`; `expired` по номеру → сопоставлено; период за январь по Москве (операция 31.01 22:30 UTC → февраль); переходы счёта — недопустимые отклоняются.
  - Файлы: `internal/billing/domain/{invoice.go,statement.go,match.go,closing.go,ports.go,errors.go,*_test.go}`.
<!-- Commit checkpoint: tasks 1-3 -->

### Phase 2: Журнал, адаптеры, документы

- [x] **Task 4: `ledger/postgres` и `ledger/app.Post` (R-2, R-3, R-5)** (depends on 2)
  - `sqlc.yaml`: записи модулей `ledger` и `billing`.
  - Запросы `internal/ledger/postgres/queries/{accounts,transactions,holds,reports}.sql`: счёт по разрезу с `FOR UPDATE`, создание при первом обращении (R-5); вставка транзакции и проводок; обновление кеша; транзакция по ключу; проводки счёта с курсором `(created_at, id)`; суммы по видам счетов; суммы по проводкам для сверки; активные удержания по размещениям; `settlement` организации за период с размещением и каналом.
  - `internal/ledger/app/post.go`: `Post(ctx, domain.Transaction) (Result{ID, Replayed bool}, error)` — только в транзакции вызывающего; ключ найден: отпечаток совпал — `Replayed`, нет — `ErrIdempotencyConflict`; иначе блокировка счетов по `id`, `ApplyPostings`, вставка, кеш. Единственный путь записи в `ledger_*` (07 §9).
  - `internal/ledger/app/balances.go`: `Balance(ctx, orgID) (available, held)`, `Transactions(ctx, orgID, cursor, limit)` для R-9, `Totals(ctx)` по видам.
  - Журнал: INFO `ledger transaction posted` (`ledger_kind`, `ledger_transaction_id`, `idempotency_key`, `postings` — виды счетов и суммы, `organization_id`, `placement_id`), DEBUG `ledger transaction replayed`, WARN `ledger transaction rejected` (`reason`: `insufficient_balance`, `key_conflict`, `missing`), ERROR `ledger invalid posting`.
  - Тесты (PostgreSQL): каждая строка 06 §2.3 через `Post` — остатки и сходимость после неё (07 §9); повтор ключа — одна транзакция; повтор с другой суммой — конфликт, ничего не записано; два одновременных `hold` одной организации на весь баланс — проходит один; вызов без транзакции — `ErrNoTransaction`; откат транзакции вызывающего откатывает проводку.
  - Файлы: `internal/ledger/postgres/{store.go,accounts.go,transactions.go,holds.go,reports.go,*_test.go}`, `internal/ledger/postgres/queries/*.sql`, `internal/ledger/postgres/sqlc/*`, `internal/ledger/app/{service.go,post.go,balances.go,*_test.go}`, `internal/ledger/ledgertest/*`, `sqlc.yaml`.

- [x] **Task 5: Удержания вместо `UnlimitedHolds` (R-6, R-7, R-8)** (depends on 4)
  - `internal/ledger/app/holds.go`: `HoldAll(ctx, orgID, []HoldRequest)`, `Release(ctx, placementID, reason)`, `Settle(ctx, placementID, Distribution)`, `HasActive(ctx, ids)`; запись `hold` и проводка — одна транзакция вызывающего; проверка нулевого остатка `placement_hold` после снятия и распределения.
  - `internal/placement/domain/ports.go`: `Holds` дополняется `HasActive` и `Settle`; `domain.SendToBlogger` получает признак активного удержания (R-7).
  - `internal/placement/app/{versions.go,transition.go}`: P-02 передаёт признак; остальные вызовы без изменений.
  - `internal/bootstrap/bridge/ledger.go`: переходник `placement/domain.Holds` → `ledger/app` с переводом нехватки в `placement/domain.InsufficientBalanceError`. `UnlimitedHolds` удаляется.
  - `internal/placement/placementtest`: поддельные удержания получают `HasActive`, `Settle`.
  - Журнал: INFO `holds placed` (`organization_id`, `order_id`, `placements`, `amount`), INFO `hold released` (`placement_id`, `reason`, `amount`), INFO `hold distributed` (`placement_id`, `payout_amount`, `fee_charged`, `refund_amount`), ERROR `placement without active hold`, ERROR `ledger invariant violated`.
  - Тесты (PostgreSQL, `clock.Manual`): оформление при достаточном балансе — удержания и проводки на каждое размещение, свободный остаток уменьшился на сумму заказа; нехватка — `409 insufficient_balance` с `missing_amount`, ничего не записано; повтор оформления с тем же ключом — без новых проводок; P-04/P-05/P-06/P-09/P-18 — `hold_release`, удержание `released`; отклонение версии и новая версия (S-29) — снятие и новое удержание с ключом новой версии, при нехватке версия не сохраняется; P-11 — удержание сохраняется; `Settle` с долей 7500 — три проводки, остаток удержания 0, `distributed`; P-02 без удержания не выполняется.
  - Файлы: `internal/ledger/app/{holds.go,holds_test.go}`, `internal/placement/domain/{ports.go,actions.go,actions_test.go}`, `internal/placement/app/{versions.go,transition.go,*_test.go}`, `internal/placement/placementtest/placementtest.go`, `internal/bootstrap/bridge/{ledger.go,placement.go}`.

- [x] **Task 6: Разбор выписки «1С:Клиент-банк» (R-12)** (depends on 3)
  - `internal/integration/bank/clientbank/parser.go`: реализация `billing/domain.StatementParser`; кодировка windows-1251 (`golang.org/x/text/encoding/charmap`) и UTF-8 по признаку `Кодировка=`; заголовок `1CClientBankExchange`, `СекцияРасчСчет` (`РасчСчет`, `ДатаНачала`, `ДатаКонца`, `НачальныйОстаток`, `КонечныйОстаток`), документы `СекцияДокумент=…` … `КонецДокумента`; поступлением считается документ с `ПолучательСчет` = счёту выписки; сумма `1234.56` → копейки без `float`; `ДатаПоступило` (если пусто — `Дата`); `external_id` по R-12. Ограничение размера файла `STATEMENT_MAX_BYTES` (10 МБ).
  - Ошибки разбора — `ErrStatementFormat` с номером строки; неизвестные ключи пропускаются.
  - Журнал: DEBUG `statement parsed` (`documents`, `incoming`, `account` маскированным).
  - Тесты (образцы в `testdata`: Сбер, Точка, Т-Банк — по открытым описаниям формата; windows-1251 и UTF-8): суммы с копейками и без; исходящие платежи отброшены; повторный документ в файле — один `external_id`; повреждённый файл — ошибка со строкой; остатки начала и конца.
  - Файлы: `internal/integration/bank/clientbank/{parser.go,parser_test.go,testdata/*}`, `go.mod` (`golang.org/x/text` прямой зависимостью).

- [x] **Task 7: PDF документов (R-10, R-16)** (depends on 3)
  - `internal/billing/pdf/`: реализация `billing/domain.Renderer` на `github.com/go-pdf/fpdf` с `gofont` (`goregular`, `gobold`): `Invoice(InvoiceDoc)` — шапка с банковскими реквизитами получателя, номер и дата, поставщик, плательщик (D-26: без банковских реквизитов — только наименование и ИНН), строка «Пополнение баланса по счёту № {номер}», сумма цифрами и прописью, строка НДС, срок действия; `AgentReport(ReportDoc)` — таблица размещений (канал, период показа, выплачено блогеру, вознаграждение) и итоги; `Act(ActDoc)` — вознаграждение за месяц. Сумма прописью — `internal/billing/pdf/words.go`. Метаданные PDF без персональных данных, время создания — из часов.
  - Журнал: DEBUG `pdf rendered` (`document`, `bytes`).
  - Тесты: прописью 0,01 ₽, 1 ₽, 21 ₽, 1 000 000,00 ₽, 1 234 567,89 ₽ (склонения «рубль/рубля/рублей»); PDF начинается с `%PDF`, содержит номер счёта (поиск по извлечённому тексту); длинное наименование переносится; отсутствие банковских реквизитов плательщика; два вызова с одинаковыми данными и часами дают одинаковые байты.
  - Файлы: `internal/billing/pdf/{renderer.go,invoice.go,report.go,act.go,words.go,*_test.go}`, `go.mod`.
<!-- Commit checkpoint: tasks 4-7 -->

### Phase 3: Сценарии

- [x] **Task 8: Счета на оплату — S-06 шаг 1 (R-1, R-10, R-11)** (depends on 4, 7)
  - `internal/billing/postgres`: репозитории `Invoices` (вставка, список организации с курсором, по номеру, `issued` по ИНН и сумме, истёкшие с `FOR UPDATE SKIP LOCKED`), запросы и `sqlc`.
  - `internal/billing/app/invoices.go`: `CreateInvoice(ctx, orgID, userID, amount)` — проверки роли (рекламодатель), суммы, номер из последовательности, снимок плательщика, PDF → хранилище (`invoices/{id}.pdf`), вставка, `audit_log` `invoice.issued`; `ListInvoices`; `InvoicePDF` (чтение из хранилища, только своей организации); `ExpireInvoices(ctx)`; `CancelInvoice(ctx, id, adminID, reason)`.
  - `internal/billing/httpapi`: `createInvoice` (`201`, ограничение частоты R-11), `listInvoices`, `getInvoicePdf` (`application/pdf`, `Content-Disposition` с номером).
  - `internal/billing/jobs`: `expire_invoices` раз в час.
  - Журнал: INFO `invoice issued` (`invoice_id`, `number`, `organization_id`, `amount`), INFO `invoice expired` (`invoice_id`), WARN `invoice cancelled` (`admin_id`, `reason`), WARN `invoice rate limited` (`user_id`).
  - Тесты (PostgreSQL, `clock.Manual`, `storagefake`): выставление — номер `MX-000001`, `expires_at` +14 суток, PDF в хранилище; блогер — `403`; сумма меньше 1 000 ₽ — `422`; 11-й счёт за час — `429`; чужой PDF — `404`; через 14 суток и 1 мин — `expired`; повторный запуск истечения — без изменений.
  - Файлы: `internal/billing/{app/service.go,app/invoices.go,app/invoices_test.go,postgres/*,httpapi/invoices.go,httpapi/invoices_test.go,jobs/jobs.go}`.

- [x] **Task 9: Баланс и операции в кабинете, правки OpenAPI (R-1, R-9, R-19)** (depends on 5)
  - `api/openapi.yaml`: пример номера счёта `MX-000101`; `BalanceTransaction.kind` + `balance_return`, `manual`; новые `listClosingActs`, `getClosingActPdf` и схема `ClosingAct` (R-19). `go generate ./...` (`apigen`, `unimplemented.go`).
  - `internal/ledger/httpapi`: `getBalance`, `listBalanceTransactions` (R-9; описание операции — «Пополнение по счёту MX-…», «Удержание по заказу №…» — собирается из основания через порт чтения заказов и счетов).
  - `mockapi`: номер счёта `MX-%06d`, новые виды операций, `listClosingActs` и `getClosingActPdf` на данных заглушки; контрактный тест `mockapi` проходит.
  - Журнал: DEBUG `balance read` (`organization_id`).
  - Тесты: баланс после пополнения и оформления — `available` и `held`; операции в порядке убывания времени с курсором, знаки, `refund` только при `refund_amount > 0`; блогер — `403`; спецификация проходит проверку `kin-openapi`.
  - Файлы: `api/openapi.yaml`, `internal/apigen/*`, `internal/bootstrap/unimplemented.go`, `internal/ledger/httpapi/{balance.go,balance_test.go}`, `mockapi/internal/{store/money.go,store/model.go,handlers/orders.go,handlers/closing.go}`.

- [x] **Task 10: Выписка и зачисление — S-30 (R-12, R-13, R-17)** (depends on 6, 8)
  - `internal/billing/app/statements.go`: `ImportStatement(ctx, adminID, file)` — `file_sha256` (повтор — `statement_already_imported`), проверка счёта получателя, файл в хранилище (`statements/{id}`), запись `bank_statement`; каждое поступление в своей транзакции: `external_id` уже есть — счётчик повторов; `Match` (05 §5); сопоставлено — `bank_transaction` `matched`, `deposit` через `Ledger`, счёт `paid` с `paid_amount`, `Notifier` `balance.deposited`; иначе `unmatched`, оповещение `bank_transaction_unmatched:{id}`. Итог — `ImportResult` со счётчиками (как в `adm/payments`). `audit_log` `bank_statement.imported`.
  - `MatchManually(ctx, bankTxID, orgID, invoiceID?, adminID, reason)`, `Ignore(ctx, bankTxID, adminID, reason)` (R-13), `ListBankTransactions(ctx, status, cursor)`.
  - Журнал: INFO `bank statement imported` (`bank_statement_id`, `incoming`, `matched`, `unmatched`, `duplicates`), INFO `payment credited` (`bank_transaction_id`, `organization_id`, `invoice_id`, `amount`, `invoice_amount`, `by`: `number`/`inn_amount`/`manual`), WARN `payment unmatched` (`bank_transaction_id`, `amount`, ИНН маскированным), WARN `payment matched manually` и `payment ignored` (`admin_id`).
  - Тесты (PostgreSQL, `clock.Manual`, `storagefake`): номер в назначении → зачислено, счёт `paid`; сумма меньше счёта — зачислена фактическая, `paid_amount` ≠ `amount`; по ИНН и сумме без номера; два кандидата — `unmatched`; `expired` по номеру — `paid`; повторная загрузка того же файла — отказ; другой файл с той же операцией — повтор пропущен, двойного зачисления нет; ручное сопоставление — проводка, `matched_by`, журнал действий; `ignored` → `matched`; сходимость после импорта.
  - Файлы: `internal/billing/{app/statements.go,app/statements_test.go,postgres/statements.go,postgres/bank_transactions.go,postgres/queries/*.sql}`.
<!-- Commit checkpoint: tasks 8-10 -->

- [x] **Task 11: Сходимость, метрики, оповещения (R-15, R-17)** (depends on 5)
  - `internal/ledger/app/check.go`: `CheckBalance(ctx) (Report, error)` по R-15; оповещения через `platform/alert` с закрытием при восстановлении.
  - `internal/ledger/jobs`: `check_ledger_balance` раз в час (`LEDGER_CHECK_INTERVAL`), `max_attempts` 1.
  - `internal/ledger/postgres/metrics.go`: `maxim_ledger_balance_kopecks{kind}` (сборщик по запросу), `maxim_ledger_transactions_total{kind}`, `maxim_ledger_check_failures_total{check}`; `internal/billing/postgres/metrics.go`: `maxim_bank_transactions{match_status}`, `maxim_invoices{status}`.
  - Журнал: INFO `ledger balance check passed` (`assets`, `liabilities`, `accounts`, `elapsed_ms`), ERROR `ledger imbalance` (`assets`, `liabilities`, `diff`), ERROR `ledger cache mismatch` (`account_id`, `kind`, `cached`, `actual`), ERROR `ledger hold mismatch` (`placement_id`, `hold_status`, `balance`).
  - Тесты (PostgreSQL): чистый журнал — проходит; кеш, испорченный в тесте под ролью владельца, — оповещение, после исправления — закрыто; активное удержание при нулевом остатке — оповещение; метрики по видам (`testutil`).
  - Файлы: `internal/ledger/{app/check.go,app/check_test.go,jobs/jobs.go,postgres/metrics.go}`, `internal/billing/postgres/metrics.go`.

- [x] **Task 12: Служебные команды `maxim admin money` (R-3, R-11, R-13, R-14)** (depends on 10, 11)
  - `internal/bootstrap/admin_money.go`:
    - `statement import --file <путь> --email <адм>` — печатает итог; `bank-tx list [--status unmatched]`; `bank-tx match --id <id> --org <id|ИНН> [--invoice MX-…] --reason … --email …`; `bank-tx ignore --id … --reason … --email …`;
    - `invoice cancel --number MX-… --reason … --email …`;
    - `ledger manual --debit <вид[:разрез]> --credit <вид[:разрез]> --amount <руб.коп> --reason … --email … [--approver-email …] [--key …]` (R-14);
    - `balance-return --org … --amount … --reason … --email … [--key …]`;
    - `reconcile` — остаток `bank` против `closing_balance` последней выписки, несопоставленные поступления, обязательства (`advertiser_balance` + `placement_hold`) против `bank` + `rocketwork_deposit`, итоги по видам (06 §2.4);
    - `ledger list [--org …] [--kind …] [--since …]` — журнал денег.
  - Проверка роли `senior`, основание обязательно; запись `audit_log` (актор `admin`) для каждой операции; суммы вводятся строкой рублей с копейками и разбираются без `float` (`money.Parse`).
  - Журнал: WARN `manual ledger operation` (`ledger_kind`, `admin_id`, `approver_id`, `amount`), остальное — через `billing`/`ledger`.
  - Тесты: `manual` 100 000,01 ₽ без второго администратора — отказ, с тем же — отказ, с другим `senior` — проведено; `balance-return` больше остатка — отказ; повтор с тем же `--key` — без второй проводки; `reconcile` показывает расхождение на сумму несопоставленного поступления; ИНН в выводе маскирован.
  - Файлы: `internal/bootstrap/{admin.go,admin_money.go,admin_money_test.go}`, `internal/platform/money/{parse.go,parse_test.go}`.

- [x] **Task 13: Закрывающие документы (R-16, R-17, R-19)** (depends on 7, 9)
  - `internal/billing/app/closing.go`: `GenerateClosingActs(ctx, period)` — организации с `settlement` за период (порт `Ledger`), для каждой в своей транзакции: пропуск, если акт есть; строки отчёта (канал, `published_at`–`removed_at`, `payout_amount`, `fee_charged`), итоги, номер, два PDF в хранилище (`closing/{org}/{period}-report.pdf`, `-act.pdf`), вставка, `Notifier` `closing_acts.ready`. `ListClosingActs`, `ClosingActPDF(ctx, orgID, id, document)`.
  - `internal/billing/httpapi`: `listClosingActs`, `getClosingActPdf`.
  - `internal/billing/jobs`: `generate_closing_acts` — `MonthlyAt(1, 3, 0)` по Москве, 3 попытки, предыдущий месяц.
  - Журнал: INFO `closing acts generated` (`period`, `organizations`, `skipped`, `elapsed_ms`), INFO `closing act created` (`closing_act_id`, `organization_id`, `placements_amount`, `fee_amount`), ERROR `closing act failed` (`organization_id`).
  - Тесты (PostgreSQL, `clock.Manual`, `storagefake`): два размещения с `settlement` в октябре (через `Settle` напрямую) и одно в ноябре — в октябрьском акте два, суммы сходятся с проводками; повторный запуск — без новых актов; организация без распределений — без акта; граница месяца по Москве; чужой акт — `404`.
  - Файлы: `internal/billing/{app/closing.go,app/closing_test.go,postgres/closing_acts.go,httpapi/closing.go,httpapi/closing_test.go,jobs/jobs.go}`.
<!-- Commit checkpoint: tasks 11-13 -->

### Phase 4: Сборка

- [x] **Task 14: Конфигурация, задания и сборка (R-10, R-11, R-14, R-15, R-16)** (depends on 12, 13)
  - `internal/bootstrap/config_billing.go`: `PLATFORM_OGRNIP`, `PLATFORM_ADDRESS`, `PLATFORM_BANK_ACCOUNT`, `PLATFORM_BANK_NAME`, `PLATFORM_BANK_BIK`, `PLATFORM_BANK_CORR_ACCOUNT` (обязательны в `production`; в `development`/`test` — значения по умолчанию `dev`), `INVOICE_TTL` (336h), `INVOICE_VAT_NOTE` («Без НДС»), `INVOICE_RATE_PER_USER` (10/ч), `INVOICE_RATE_PER_IP` (30/ч), `INVOICE_EXPIRE_INTERVAL` (1h), `STATEMENT_MAX_BYTES` (10 МБ), `LEDGER_CHECK_INTERVAL` (1h), `LEDGER_MANUAL_SECOND_APPROVAL_THRESHOLD` (10 000 000 коп.), `CLOSING_ACTS_DAY` (1), `CLOSING_ACTS_TIME` (03:00). Проверки при запуске: БИК 9 цифр, счета 20 цифр, ключ корр. счёта по БИК, `INVOICE_TTL` > 0.
  - `internal/bootstrap/billing.go` (новый): сборка `ledger/app`, `billing/app`, репозиториев, парсера, PDF; `bridge.LedgerHolds` в `placement`; маршруты `ledger/httpapi`, `billing/httpapi` в `apiImpl`; задания `expire_invoices`, `check_ledger_balance`, `generate_closing_acts` в `registerJobs`; метрики.
  - `internal/bootstrap/placement.go`, `placement_config_test.go`: `Holds` — журнал, не `UnlimitedHolds`.
  - `internal/bootstrap/dev.go`: `maxim dev balance deposit --org … --amount …` (только `development`/`test`: проводка `deposit` с поддельной операцией банка — замена `_mock/invoices/{id}/pay` для ручных прогонов) и `maxim dev statement --invoice MX-…` (печатает файл выписки «1С» с поступлением по счёту для `admin money statement import`).
  - `.env.example`, `deploy/.env.example`, `compose.dev.yml`: новые переменные.
  - Журнал: конфигурация — маскированной (счёт платформы — последние 4 цифры); INFO `billing configured` (`invoice_ttl`, `check_interval`).
  - Тесты: проверки конфигурации на границах; `wiring_test.go`: виды заданий зарегистрированы с нужными политиками, `placement` получил журнал, операции `balance` больше не `501`.
  - Файлы: `internal/bootstrap/{config.go,config_billing.go,config_billing_test.go,billing.go,modules.go,placement.go,placement_config_test.go,worker.go,dev.go,wiring_test.go}`, `.env.example`, `deploy/.env.example`, `compose.dev.yml`.
<!-- Commit checkpoint: task 14 -->

### Phase 5: Проверка и документы

- [x] **Task 15: Синхронизация документов с решениями этапа** (depends on 14)
  - После подтверждения R-1 … R-19 внести их в документы:
    - 02 S-06 — ограничение частоты счетов, отмена только администратором (R-11); S-30 — выписка по другому счёту, повторный файл, «постороннее» и обратно (R-12, R-13); S-19 — ручные операции до API администратора (R-14).
    - 03 §4 — P-02 проверяет активное удержание (R-7); §7 — где формула (R-8).
    - 04 §7 — `hold.post_version_id`, `ledger_transaction.entries_hash`, поля `invoice.payer_*`, `cancel_reason`, `bank_statement` (R-12), `bank_transaction.payer_account`, `ignored_reason`, `closing_act` с двумя PDF (R-16), проверка I-12 в `placement`.
    - 05 §5 — `external_id` для «1С», только поступления на счёт платформы, номер `MX-` (R-1, R-12).
    - 06 §2.2 — ключи идемпотентности и отпечаток (R-2, R-3); §2.4 — команда `reconcile`; §2.5 — номер акта, срок формирования, два PDF (R-16).
    - 07 §2 — `expire_invoices`, `check_ledger_balance`, `generate_closing_acts`; §5.5 — имена метрик; §8 — новые параметры; §4 — порог второго администратора в конфигурации.
    - `docs/open-questions.md` — R-18 (оферты при оформлении) в открытые вопросы; Q-26 — где адаптер выписки; Q-32 — `INVOICE_VAT_NOTE`.
    - `.ai-factory/ARCHITECTURE.md` — модули `ledger` и `billing` (слои, порты, `bridge.LedgerHolds`), `integration/bank/clientbank`, `billing/pdf`.
  - Журнал: не применимо.
  - Файлы: `docs/02-scenarios.md`, `docs/03-placement.md`, `docs/04-domain-model.md`, `docs/05-integrations.md`, `docs/06-finance-legal.md`, `docs/07-non-functional.md`, `docs/open-questions.md`, `.ai-factory/ARCHITECTURE.md`.

- [x] **Task 16: Сквозные тесты денег** (depends on 14)
  - `internal/bootstrap/billing_e2e_test.go` на собранных `api` и `worker` (PostgreSQL, `clock.Manual`, `storagefake`, `ordfake`, `maxfake`, `mailfake`):
    - S-06: рекламодатель сохраняет банковские реквизиты, выставляет счёт на 20 000 ₽, скачивает PDF; оформление заказа на 11 500 ₽ — `409 insufficient_balance` с `missing_amount = 1 150 000`.
    - S-30: `dev statement` → `admin money statement import` → счёт `paid`, баланс 20 000 ₽, повторная загрузка — отказ, другой файл с той же операцией — повтор пропущен.
    - Оформление — удержание, `available` 8 500 ₽, `held` 11 500 ₽; отклонение версии — удержание снято; новая версия — удержание снова; блогер отказывается (P-04) — удержание снято, операции кабинета показывают всю цепочку.
    - Поступление без номера и с чужим ИНН — `unmatched`, оповещение; ручное сопоставление — зачислено.
    - Через 14 суток невыплаченный счёт `expired`; поступление по нему — `paid`.
    - `Settle` через порт с долей 7500 (имитация приёмки) → `generate_closing_acts` за месяц → кабинет видит акт, оба PDF скачиваются, суммы 7 500 ₽ и 1 125 ₽.
    - `check_ledger_balance` после всех шагов — без оповещений; `admin money reconcile` — расхождение только на несопоставленные.
  - Проверка журнала сборщиком записей: события 06 §11 присутствуют; ИНН только маскированным, наименования организаций, назначения платежей и номера банковских счетов плательщиков не встречаются.
  - Файлы: `internal/bootstrap/billing_e2e_test.go`, при необходимости `internal/bootstrap/harness_test.go`.
<!-- Commit checkpoint: tasks 15-16 -->

## Критерии готовности этапа

- `go build ./...`, `go vet ./...`, `go test -race ./...` проходят локально и в CI. `sqlc generate` и `go generate ./...` не дают изменений. `mockapi` собирается и проходит свой контрактный тест.
- В `ledger_*` и `hold` пишет только `ledger/postgres`; роль приложения не может изменить или удалить проводку.
- Каждое движение денег — транзакция журнала с уникальным ключом; повтор ничего не меняет, повтор с другим составом отклоняется.
- Оформление заказа и новая версия поста при нехватке баланса отклоняются `409 insufficient_balance` без частичных записей; удержание, переход и история — в одной транзакции (I-10); у размещения не больше одного активного удержания (I-03); P-02 без удержания не выполняется.
- Поступление из выписки зачисляется ровно один раз; несопоставленные видны администратору и разбираются командой.
- Сходимость проверяется раз в час, расхождение даёт оповещение.
- Отчёт агента и акт формируются за месяц, доступны рекламодателю в PDF.
- ИНН (кроме маскированного), наименования организаций, назначения платежей и банковские реквизиты плательщиков отсутствуют в журнале.
- Решения R-1 … R-19 подтверждены и внесены в документы (задача 15).
