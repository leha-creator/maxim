# План реализации: Выплаты блогерам через Рокет Ворк

Branch: feature/rocketwork-payouts
Created: 2026-10-20
Refined: 2026-10-24 (`/aif-improve`, сверка с кодом после 1b1a9d1 и 7e7d786)

## Settings
- Testing: yes. Состояния исполнителя и выплаты, коды отказов, приведение ответов адаптера к нашим статусам, расписание опроса (5 мин первый час, затем раз в час), формула контроля депозита, годовой лимит самозанятого и порог 90%, ключи идемпотентности — чистые табличные тесты `payout/domain`. Подпись вебхука и разбор событий — тесты адаптера и `rocketworkfake`. Сценарии `payout/app` с `postgres` — на настоящей PostgreSQL (`platform/dbtest`), `clock.Manual`, `rocketworkfake`, `ledgertest`, `placementtest`. После каждой денежной операции — `ledger.CheckBalance` без оповещений. Сквозные сценарии — на собранных `api` и `worker` (`harness_test.go`).
- Logging: verbose. `log/slog` JSON, события 06 §11, 05 §4, 07 §5.3: исполнитель создан или найден, статус исполнителя изменён (INFO: `contractor_id`, `organization_id`, `from`, `to`); выплата создана, отправлена, выполнена (INFO: `payout_id`, `placement_id`, `attempt`, `amount`, `provider_fee`); выплата не прошла (WARN, для `income_limit_exceeded` и исчерпания повторов — ERROR + оповещение: `failure_code`); нехватка депозита (ERROR + оповещение: `deposit`, `required`); расхождение депозита (ERROR + оповещение); вебхук отклонён (WARN), принят (DEBUG), событие обработано или пропущено (INFO/DEBUG); ручные команды (WARN + `audit_log`). Поля `payout_id`, `placement_id`, `contractor_id`, `organization_id`, `external_id`, `idempotency_key`, `job_id`, `inbound_event_id`. ИНН, ФИО, телефон — только маскированными (`logx.MaskINN` и аналоги); ссылка онбординга, ссылка на чек, тело ответа Рокет Ворк выше DEBUG не пишутся; токен и секрет вебхука не пишутся никогда (07 §5.4).
- Docs: yes. В конце обязательная точка `/aif-docs`: README бэкенда (переменные `ROCKETWORK_*` и `PAYOUT_*`, режимы `fake`/`disabled`, путь «подключить выплаты → принять размещение → приёмка → выплата → чек», команды `maxim admin payout …` и `maxim admin money deposit-topup`, команды `maxim dev rocketwork …`).

## Roadmap Linkage
Milestone: "Выплаты блогерам через Рокет Ворк"
Rationale: план реализует этап целиком: онбординг исполнителя (S-11) и проверку при принятии (P-03), выплату после приёмки (S-15) с проводкой `payout` и переходом P-16, чеки (`receipt_url`), статусы выплат и исполнителей, вебхуки и опрос, контроль и сверку депозита. Настоящий HTTP-клиент Рокет Ворк зависит от этапа «Проверки и доступы» (методы и подпись не известны, R-1); вызов создания выплаты из P-12, P-13, P-15 — этап «Приёмка, споры и возвраты».

## Research Context
Source: .ai-factory/RESEARCH.md (Active Summary), выдержка по этапу

- Выплаты блогерам через Рокет Ворк (b2b.rocketwork.ru); платформа (ИП) — заказчик, заранее пополняет депозит и даёт команду на выплату конкретному исполнителю (A-04). Комиссию Рокет Ворк платит платформа.
- Исполнитель регистрируется на стороне Рокет Ворк (паспорт, ИНН, селфи, разрешение в «Мой налог»); сервис проверяет статус в ФНС, регистрирует доход, формирует чек, платит налог.
- Документация API (`rocketwork.ru/api/docs`) при исследовании отдавала 503: имена методов, авторизация, вебхуки и статусы не известны (Q-10, Q-11, Q-12).
- «Безопасная сделка» в описанном виде не подходит; схема «баланс → удержание → выплата» ведётся во внутреннем учёте.

## Источники ТЗ

- Сценарии: 02 S-10, S-11 (онбординг), S-14 (`409 contractor_inactive`), S-15 (выплата, отказ, юрлицо — A-36), S-08, S-24, S-18 (откуда приходит `accepted`), S-19 (`retry_payout`).
- Размещение: 03 §3 («выплата не прошла» — без перехода), §4 (P-03, P-12, P-13, P-15, P-16), §5 T-12 (72 ч), §10 (I-02, I-10, I-12, I-13).
- Данные: 04 §2 (`organization`, `blogger_profile`), §7 (`rocketwork_contractor`, `payout`, частичный уникальный индекс), §10 (что не храним), 00003 (`inbound_event`, `outbound_request`, `outbound_circuit` уже допускают `rocketwork`).
- Интеграции: 05 §1 (общие правила, повторы, предохранитель), §4.1–§4.4 (роль, контракт адаптера, выплата, коды отказов, контроль депозита).
- Финансы и право: 06 §1, §2.1, §2.3 (`payout`, `deposit_topup`), §2.4 (сверка с Рокет Ворк), §3, §4 (кому и когда, лимит самозанятого), §8 (ПДн), §11.
- Требования: 07 §2 (`send_payout`, `poll_payout`, `poll_contractor`, `check_rocketwork_deposit`, `process_inbound_event`), §3 (T-12, зависшие события), §4 (секреты, подлинность вебхуков, `webhook_auth_failed`), §5.3–§5.5, §6 (уведомления блогеру), §9.
- Реестр: A-04, A-24, A-36, D-08, D-14, Q-10, Q-11, Q-12, Q-28, Q-36.
- Архитектура: `.ai-factory/ARCHITECTURE.md` — модуль `payout/` (исполнители, выплаты, контроль депозита), адаптер `integration/rocketwork/` с `rocketworkfake/`.
- Экраны: `docs/wireframe/index.html` — `blg/payouts`, `blg/placement`, `adm/payouts`, `adm/placement`.
- OpenAPI: тег `blogger-payouts` — `listBloggerPayouts`, `getRocketworkStatus`, `inviteRocketwork`; `rocketworkWebhook`; схемы `Payout`, `PayoutStatus`, `RocketworkStatus`, `RocketworkContractorStatus`; `bloggerAcceptPlacement` (`409 contractor_inactive`).
- Существующий код:
  - `internal/ledger/domain/{account.go,transaction.go,builders.go (NewPayout, NewDepositTopup, PayoutKey)}`, `internal/ledger/app/{post.go,check.go,admin.go}`;
  - `internal/placement/domain/{ports.go (ContractorGate, Holds.Settle, Notifier),transitions.go (P16Paid),errors.go (ErrContractorInactive)}`, `internal/placement/app/{blogger.go (Accept),transition.go (apply, record)}`, `internal/placement/placementtest`;
  - `internal/bootstrap/{modules.go,integration.go,placement.go (AllowAllContractors),billing.go,ledger.go,marking.go,channel.go (maxWebhookPattern),admin_money.go (reconcile),dev.go,dev_money.go,config.go,config_ord.go,unimplemented.go,harness_test.go,wiring_test.go}`, `internal/bootstrap/bridge/{placement.go,ledger.go,billing.go}`;
  - `internal/integration/ord` + `ordfake` (образец адаптера), `internal/channel/httpapi/webhook.go` (образец вебхука), `internal/channel/jobs/jobs.go` (обработчики `inbound`), `internal/marking/jobs/jobs.go` (образец заданий с полезной нагрузкой);
  - `internal/platform/{outbound (SystemRocketWork),inbound (SourceRocketWork),job,stuck (комментарий T-12),alert,audit,ratelimit,metrics,tx,clock,dbtest}`;
  - `internal/account/{domain/types.go,domain/tax.go,postgres}` (`legal_type`, `blogger_profile.full_name`, `ForBilling` как образец чтения для другого модуля);
  - `migrations/00010_money.sql` (`ledger_transaction.payout_id` без внешнего ключа «до этапа выплат»).

## Границы этапа

Входит:
- Модуль `payout` (`domain`, `app`, `postgres`, `httpapi`, `jobs`, `payouttest`): исполнители Рокет Ворк, выплаты и попытки, обработка событий, опрос, контроль депозита, годовой лимит самозанятого.
- Адаптер `integration/rocketwork`: контракт 05 §4.2 (интерфейс и типы, приведение статусов и кодов, проверка подписи вебхука) и поддельная реализация `rocketworkfake`. Настоящий HTTP-клиент — после ответов Рокет Ворк (R-1).
- Настоящий `ContractorGate` вместо `bridge.AllowAllContractors` (P-03, S-14).
- Порт создания выплаты для размещения (`placement/domain.Payouts`) и системный переход P-16 при успехе выплаты в одной транзакции с проводкой `payout`.
- Путь выплаты юрлицу вручную (A-36): команда администратора и проводка из `bank`.
- Операции кабинета блогера: `listBloggerPayouts`, `getRocketworkStatus`, `inviteRocketwork`, новые `refreshRocketworkStatus`, `getBloggerPayoutSummary`; приём `rocketworkWebhook`.
- Команды `maxim admin payout …` и `maxim admin money deposit-topup` до API администратора; сверка депозита в `admin money reconcile`.
- Проверка зависших: T-12 и выплаты в `sent` без исхода.

Не входит (следующие этапы):
- Сверка HTTP-клиента с настоящим API Рокет Ворк (методы, авторизация, подпись, лимиты) — этап «Проверки и доступы» (Q-10, Q-11, Q-12). Здесь — клиент по допущениям (задача 3a) и режим `disabled`.
- Вызов `Payouts.Create` и `Holds.Settle` из P-12, P-13, P-15 — этап «Приёмка, споры и возвраты». Здесь вызов делает служебная команда `maxim dev placement accept` (только `development`/`test`) и тесты.
- Реестр выплат в OpenAPI администратора, экран `adm/payouts`, выгрузка CSV, повтор через интерфейс — этап «API администратора».
- Отправка уведомлений: порт `Notifier` остаётся `NopNotifier`, коды шаблонов — R-15.
- Аннулирование чека, возврат выплаты, возврат остатка депозита из Рокет Ворк — нет в ТЗ, вносится в открытые вопросы (R-17).

## Решения этапа, требующие подтверждения

Документы этих мест не определяют или противоречат друг другу. План реализует указанный вариант. После подтверждения задача 14 вносит его в документы. При отказе меняется только отмеченная задача.

| # | Вопрос | Решение в плане | Задача |
|---|---|---|---|
| R-1 | API Рокет Ворк не известен | Модуль работает только через интерфейс `payout/domain.Provider` по контракту 05 §4.2 (`EnsureContractor`, `GetContractor`, `GetDepositBalance`, `CreatePayout`, `GetPayout`, `FindPayout` по нашему ключу, `ParseWebhook`). `ROCKETWORK_MODE` ∈ `fake`, `disabled`, `live`. `fake` — `rocketworkfake` (по умолчанию в `development`/`test`). `live` — HTTP-клиент `integration/rocketwork.Client` по **предполагаемому** контракту (раздел «Допущения о внешних данных», каждое место помечено `// ПРЕДПОЛОЖЕНИЕ (Q-NN): …`); включать только после сверки с настоящей документацией. `disabled` — все вызовы возвращают `ErrProviderDisabled`: `inviteRocketwork` — `503 rocketwork_unavailable`, задания откладываются `job.Postpone` на час с WARN, вебхук — `404`. В `production` по умолчанию `disabled`; `fake` в `production` — ошибка конфигурации; `live` в `production` — WARN `rocketwork live client uses an unverified contract` при запуске | 3, 3a, 11 |
| R-2 | Статусы Рокет Ворк | Приведение к нашим статусам — внутри адаптера: исполнитель → `invited`, `pending`, `active`, `blocked` (+ `blocked_reason`); выплата → `sent`, `succeeded`, `failed` (+ наш `failure_code` из 05 §4.3). Неизвестный статус сервиса — `pending`/`sent` с WARN `rocketwork unknown status`, объект продолжают опрашивать | 2, 3 |
| R-3 | Юрлицо (A-36) | Для `legal_entity` исполнитель не создаётся, `getRocketworkStatus` → `required = false`, `status = none`; гейт пропускает. Выплата создаётся с `method = bank_transfer`, `rocketwork_contractor_id = null`, задание `send_payout` не ставится. Администратор переводит деньги вручную и выполняет `maxim admin payout mark-paid --payout … --bank-ref … --reason …`: `succeeded`, проводка `payout` `blogger_payable → bank` (правило вида `payout` расширяется вторым вариантом: одна проводка в `bank`, комиссии нет), P-16. До ответа на Q-36 | 1, 2, 5, 10 |
| R-4 | Кто создаёт выплату | Порт `placement/domain.Payouts{Create(ctx, PayoutRequest{PlacementID, BloggerOrgID, Amount}) (uuid.UUID, error)}` вызывается в транзакции перехода в `accepted` (I-10) и ставит `send_payout` (I-13). Реализация — `bridge.PlacementPayouts` → `payout/app.Create`. До этапа приёмки — `placementtest.Payouts` и служебная `maxim dev placement accept --placement … --share …` (P-12 с `Settle` и `Create`, только `development`/`test`) | 5, 6, 11 |
| R-5 | Успех выплаты | В одной транзакции: `payout` → `succeeded` (`provider_fee`, `receipt_url`, `completed_at`), проводка `ledger.NewPayout` (ключ `payout:{payout_id}`), P-16 через порт `payout/domain.Placements.MarkPaid(ctx, placementID, payoutID)` → `placement/app.MarkPaid` (актор `system`, история). Повтор события после успеха — no-op. Если размещение уже не `accepted` — откат, ERROR `payout placement state mismatch`, оповещение `payout_state_mismatch:{payout_id}` | 5, 6 |
| R-6 | Ключ идемпотентности для Рокет Ворк | `payout.idempotency_key = "rw-payout:" + payout.id`; новая попытка — новая строка и новый ключ. Неизвестный исход `CreatePayout` (таймаут) → `outbound.Call{Create: true, CheckExisting: FindPayout}`; если `FindPayout` не находит — повтор с тем же ключом | 3, 6 |
| R-7 | Проверки перед отправкой | `send_payout`: исполнитель `active` по последнему известному статусу, при `status_checked_at` старше 1 ч — сначала `GetContractor`. Не `active` → `failed` `contractor_not_active`. Депозит `GetDepositBalance` < суммы → выплата остаётся `created`, ERROR + оповещение `rocketwork_deposit_insufficient`, `job.Postpone(1h)` (попытка не тратится — автоповтор каждый час без предела, 05 §4.3) | 6 |
| R-8 | Коды отказов | `contractor_not_active`, `tax_status_lost`, `income_limit_exceeded` — окончательный `failed`, повтор только `retry`; `provider_error` — повторы задания (10 попыток), после исчерпания (`OnExhausted`) — `failed` `provider_error`, ERROR + оповещение `payout_failed:{payout_id}`. Отказ после `sent` (событие или опрос) — `failed` с кодом от адаптера. Размещение остаётся `accepted`, начисление на `blogger_payable` | 2, 6 |
| R-9 | Повтор (`retry_payout`, S-19) | Только из последней попытки `failed` размещения в `accepted`: новая строка `attempt + 1`, `created`, `send_payout`. Команда `maxim admin payout retry --payout … --email … --reason …` (WARN + `audit_log`). Для `bank_transfer` повтор не нужен | 10 |
| R-10 | Вебхуки и опрос | Опрос делается всегда, вебхук только ускоряет (Q-12: наличие вебхуков не подтверждено). `poll_payout:{payout_id}` ставится при `sent`: каждые 5 мин первый час, затем раз в час (`PAYOUT_POLL_FAST`, `PAYOUT_POLL_SLOW`), до окончательного статуса. `poll_contractor:{contractor_id}` — для `invited`/`pending`, то же расписание, не дольше 30 суток (`ROCKETWORK_CONTRACTOR_POLL_MAX_AGE`), для `active` — раз в сутки (лишение статуса до выплаты). Кнопка «Проверить статус» — `POST /blogger/rocketwork/refresh`: ставит `poll_contractor` сейчас (I-13), отвечает текущим статусом | 6, 7, 8 |
| R-11 | Подлинность вебхука | Проверка — в адаптере (`Provider.ParseWebhook(header, body)`), обработчик не знает формата. `rocketworkfake` — HMAC-SHA256 тела секретом `ROCKETWORK_WEBHOOK_SECRET` в заголовке `X-Rocketwork-Signature` (сравнение за постоянное время). Отказ — `401`, WARN, пять за 10 минут — оповещение `webhook_auth_failed:rocketwork`. Тело до 1 МиБ. `ExternalKey` — идентификатор события сервиса, иначе `inbound.KeyFromBody` | 3, 8 |
| R-12 | Контроль депозита | `check_rocketwork_deposit` раз в час: `need = Σ blogger_payable (только rocketwork-получатели) + Σ blogger_amount размещений в published, removed с исполнителем Рокет Ворк`; остаток API < `need` → ERROR + оповещение `rocketwork_deposit_low` (сумма недостачи), иначе закрытие оповещения. Сверка (06 §2.4): `|остаток rocketwork_deposit в журнале − остаток API| > ROCKETWORK_DEPOSIT_TOLERANCE` (100 коп.) → ERROR + `rocketwork_deposit_mismatch`. Те же числа печатает `admin money reconcile`. Метрика `maxim_rocketwork_deposit_kopecks{source=api|ledger}` | 9 |
| R-13 | Годовой лимит самозанятого | Сумма `succeeded` выплат по исполнителю `self_employed` за календарный год по Москве; при создании выплаты, если сумма с новой ≥ 90% `PAYOUT_NPD_ANNUAL_LIMIT` (по умолчанию 2 400 000 ₽, 06 §4) — WARN + оповещение `npd_income_limit:{organization_id}`. Выплату не блокирует (решает сервис, код `income_limit_exceeded`) | 2, 5 |
| R-14 | Зависшие | `stuck`: `payout_accepted_unpaid` — размещение в `accepted` без `succeeded` дольше 72 ч (T-12); `payout_sent_no_outcome` — `sent` дольше 24 ч; `payout_created_not_sent` — `created` дольше 2 ч (кроме `bank_transfer`). Оповещение раз в час | 9 |
| R-15 | Уведомления | Порт `payout/domain.Notifier` (`NopNotifier`): `payout.sent` («Принято, выплата отправлена»), `payout.succeeded` (сумма, ссылка на чек), `payout.failed` (причина), `rocketwork.active` (выплаты подключены), `rocketwork.blocked`. Администратору — оповещения из R-7, R-8, R-12, R-13, R-14 | 5, 6, 7 |
| R-16 | Изменения OpenAPI | `Payout`: + `failure_code` (enum 05 §4.3), `method` (`rocketwork`, `bank_transfer`). `RocketworkStatus`: + `blocked_reason`, `status_checked_at`. Новые `POST /blogger/rocketwork/refresh` (`refreshRocketworkStatus`) и `GET /blogger/payouts/summary` (`getBloggerPayoutSummary`: `received_amount`, `in_progress_amount`, «Итог» на `blg/payouts`). `inviteRocketwork`: `409 contractor_not_required` для юрлица, `503 rocketwork_unavailable`. `mockapi` получает те же операции | 12 |
| R-17 | Аннулирование чека, возврат выплаты, возврат депозита | В ТЗ нет. Не делается; записывается в открытые вопросы к Рокет Ворк (Q-12) | 14 |
| R-18 | ПДн | В Рокет Ворк уходят только ИНН, ФИО (`blogger_profile.full_name`), телефон, тип лица (06 §8). Ссылку онбординга храним (`onboarding_url`) — она для блогера; в журнал выше DEBUG не пишется | 3, 5 |

## Уточнения после сверки с кодом (aif-improve, 2026-10-24)

| # | Что найдено в коде | Решение | Задача |
|---|---|---|---|
| I-1 | `newModules` строит `placement` раньше остальных; `payout` нужен `placement/app.MarkPaid`, а `placement` — `ContractorGate` и `Payouts` из `payout`: цикл при сборке | `placement/app.Service.SetPayoutPorts(gate, payouts)` (как `channels.SetPlacementPorts`); модуль выплат собирается после `placement`, затем вызывается `SetPayoutPorts`. До вызова — `AllowAllContractors` и `bridge.NoPayouts` (ошибка `payouts are not wired`) | 7, 11 |
| I-2 | Патч 2026-10-24-14.25: mockapi обязан отдавать те же `details.reason`, что backend; сейчас mock `inviteRocketwork` отвечает `200` и `409 reason=conflict` | mock: `202`, `409 details.reason = contractor_not_required`; мгновенная активация в mock остаётся (так описано в операции); новые поля `Payout.failure_code`, `method`, операции `refresh` и `summary`; тест ошибок в стиле `gaps_test.go` | 12 |
| I-3 | `job.Queue.Reschedule` переносит только ожидающее задание; выполняющийся опрос себя не переносит. Образец — `marking/app/poll.go`: `job.Postpone(err, next)` | `PollPayout`/`PollContractor` возвращают `job.Postpone(…, PollDelay(...))` — попытки не тратятся | 6 |
| I-4 | `PageMeta.total` (7e7d786) необязателен | `listBloggerPayouts` заполняет `total` | 12 |
| I-5 | `rocketworkWebhook` есть в strict server; сырой маршрут в `Routes` перекрывает его в `ServeMux` (как `maxWebhookPattern`) | заглушка в `unimplemented.go` остаётся ради интерфейса strict server, приём — `payout/httpapi.Webhook` в `Routes` | 8 |
| I-6 | Нужен правдоподобный контракт HTTP-клиента, а не только место под него | задача 3a: `integration/rocketwork.Client` через `platform/outbound` по допущениям ниже; тесты на `httptest` | 3a |

## Допущения о внешних данных

Документации API Рокет Ворк нет (Q-10, Q-11, Q-12). Всё ниже придумано и помечено в коде комментарием `// ПРЕДПОЛОЖЕНИЕ (Q-NN): …`. После получения документации заменить в `internal/integration/rocketwork/{client.go,types.go,webhook.go,testdata/*}` и `rocketworkfake`.

1. **Q-10, адрес.** Базовый адрес `https://api.rocketwork.ru/api/v2` (`ROCKETWORK_BASE_URL`), JSON, UTF-8.
2. **Q-10, авторизация.** `Authorization: Bearer <ROCKETWORK_TOKEN>` — постоянный токен кабинета заказчика; `401/403` — окончательная ошибка (`provider_error`), оповещение от `outbound`.
3. **Q-10, исполнитель.** `POST /contractors` `{external_id: <organization_id>, inn, phone, full_name, legal_form: self_employed|individual_entrepreneur}` → `{id, status, onboarding_url, blocked_reason?}`; повтор с тем же `inn` возвращает существующего. `GET /contractors/{id}` — то же тело.
4. **Q-11, статусы исполнителя.** `invited` → `invited`; `registering`, `documents_check` → `pending`; `active` → `active`; `blocked`, `fns_unbound`, `deleted` → `blocked` (`blocked_reason`; `fns_unbound` → `tax_status_lost`). Неизвестный — `pending` + WARN.
5. **Q-10, депозит.** `GET /account/balance` → `{available_kopecks, currency: "RUB"}`.
6. **Q-10, выплата.** `POST /payments`, заголовок `Idempotency-Key: rw-payout:<payout_id>`, тело `{contractor_id, amount_kopecks, purpose, external_id: <payout_id>}` → `{id, status, external_id, amount_kopecks, fee_kopecks?, receipt_url?, failure_reason?, failure_message?}`; повтор с тем же ключом возвращает ту же выплату. `GET /payments/{id}`; `GET /payments?external_id=<payout_id>` → `{items: [...]}` (`FindPayout`).
7. **Q-11, статусы выплаты.** `new`, `processing` → `sent`; `paid` → `succeeded` (`fee_kopecks`, `receipt_url`); `failed`, `rejected` → `failed` с `failure_reason` ∈ `contractor_not_active`, `tax_status_lost`, `income_limit_exceeded`, `insufficient_funds` (→ `insufficient_deposit`), прочее → `provider_error`. Неизвестный — `sent` + WARN.
8. **Q-10, ошибки.** Тело `{error: {code, message}}`; `422 insufficient_funds` при создании выплаты; `409 contractor_not_active`; `404` — объекта нет; `429` с `Retry-After`; `5xx` — повтор.
9. **Q-12, вебхук.** Тело `{id: <event_id>, event: contractor.status_changed|payment.status_changed, object_id, occurred_at}`; заголовок `X-Rocketwork-Signature: sha256=<hex HMAC-SHA256(body, ROCKETWORK_WEBHOOK_SECRET)>`. Событие несёт только идентификатор — состояние запрашиваем сами.
10. **Q-11, комиссия.** Комиссия сервиса списывается с депозита сверх суммы и приходит в `fee_kopecks` успешной выплаты (отдельная проводка `payout_fees`, 06 §2.3).
11. **Q-10, лимит запросов.** 5 запросов в секунду на заказчика (`ROCKETWORK_RATE`, корзина `rocketwork:global`).
12. **Данные `rocketworkfake`.** Комиссия — 3% суммы, не меньше 1 ₽; начальный депозит 0 (задаётся `maxim dev rocketwork deposit`); ссылка онбординга `https://rocketwork.example.test/onboarding/<id>`; чек `https://rocketwork.example.test/receipts/<payment_id>`.

## Commit Plan
- **Commit 1** (после задач 1–3a): `feat(payout): payout schema, domain rules and rocket work adapter contract`
- **Commit 2** (после задач 4–7): `feat(payout): contractors, payouts, send and poll jobs, P-16`
- **Commit 3** (после задач 8–10): `feat(payout): rocket work webhook, deposit control, admin payout commands`
- **Commit 4** (после задач 11–12): `feat(payout): blogger payouts api, config and wiring`
- **Commit 5** (после задач 13–14): `test(payout): payouts e2e, docs sync`

## Tasks

### Phase 1: Данные, правила, контракт адаптера

- [x] **Task 1: Миграция выплат (R-3, R-6, R-10)**
  - `migrations/00011_payouts.sql` по 04 §7:
    - `rocketwork_contractor` (`organization_id unique not null`, `external_id unique`, `status check in (invited, pending, active, blocked)`, `blocked_reason`, `onboarding_url`, `status_checked_at`, `last_error`, `created_at`, `updated_at`).
    - `payout` (`placement_id`, `blogger_organization_id`, `method check in (rocketwork, bank_transfer)`, `rocketwork_contractor_id` — обязателен при `rocketwork`, пуст при `bank_transfer`, `amount > 0`, `attempt >= 1`, `status check in (created, sent, succeeded, failed)`, `external_id`, `provider_fee >= 0`, `receipt_url`, `bank_reference`, `failure_code check in (contractor_not_active, tax_status_lost, income_limit_exceeded, insufficient_deposit, provider_error)`, `failure_message`, `sent_at`, `completed_at`, `idempotency_key unique`, `lock_version`, `created_by`); `UNIQUE (placement_id, attempt)`; частичный уникальный индекс `(placement_id) WHERE status in (created, sent, succeeded)`; `check`: `succeeded` ⇒ `completed_at`, `failed` ⇒ `failure_code`; индексы `(blogger_organization_id, created_at DESC, id DESC)`, `(status, created_at)`.
    - `ledger_transaction.payout_id` — внешний ключ на `payout(id)` (`NOT VALID` + `VALIDATE`).
  - Права роли: `REVOKE DELETE` на `rocketwork_contractor`, `payout`.
  - Журнал: не применимо (INFO `migrations applied` пишет `migrate`).
  - Тесты: `up` → `down` → `up`; две живые выплаты размещения не вставляются; после `failed` вставляется попытка 2; `rocketwork` без исполнителя и `bank_transfer` с исполнителем не проходят; `failed` без кода не проходит; роль приложения не может `DELETE payout`.
  - Файлы: `migrations/00011_payouts.sql`.

- [x] **Task 2: `payout/domain` (R-2, R-3, R-8, R-10, R-13)** (depends on 1)
  - `internal/payout/domain/contractor.go`: `ContractorStatus`, `Contractor`, `Required(legalType) bool` (юрлицо — нет), `CanAccept()`, `Apply(ProviderContractor) (changed bool)`.
  - `internal/payout/domain/payout.go`: `Status`, `Method`, `FailureCode` (05 §4.3, флаг `Terminal()` и `Retryable()`), `Payout`; переходы `MarkSent(externalID, at)`, `MarkSucceeded(fee, receiptURL, at)`, `MarkFailed(code, message, at)`, `NextAttempt()` — с ошибкой `ErrInvalidPayoutState`; `IdempotencyKey(id)` (R-6).
  - `internal/payout/domain/schedule.go`: `PollDelay(since, now, fast, slow)` (R-10).
  - `internal/payout/domain/limits.go`: `DepositNeed(payable, inFlight)`, `NPDWarning(yearSum, amount, limit) bool` (R-13, 90%).
  - `internal/payout/domain/ports.go`: `Provider` (R-1) и типы `ProviderContractor`, `ProviderPayout`, `WebhookEvent{Kind (contractor_status, payout_status), ExternalID, EventID}`; `Store`, `Placements{MarkPaid, Brief(ids)}`, `Ledger{Payout(tx), LegalPayout(tx), PayableTotal, DepositBalance}`, `Organizations{ForPayout(orgID)}` (ИНН, тип лица, ФИО, телефон), `Notifier`; ошибки `ErrProviderDisabled`, `ErrContractorNotRequired`, `ErrNotFound`, `ErrPayoutNotRetryable`.
  - `internal/ledger/domain/transaction.go`, `builders.go`: второй вариант правила `payout` — одна проводка `blogger_payable → bank` (R-3), `NewLegalPayout(payoutID, placementID, bloggerOrgID, amount, adminID, reason)`; ключ тот же `payout:{payout_id}`.
  - Журнал: не применимо (`domain` не пишет журнал).
  - Тесты (табличные): все допустимые и недопустимые переходы выплаты и исполнителя; повторный `MarkSucceeded` с теми же данными — no-op, с другими — ошибка; `PollDelay` на 0, 59, 60, 61 мин; `NPDWarning` на 89,99% / 90% / 100%; `DepositNeed`; правило `payout` — оба варианта валидны, `payout_fees` при выплате в `bank` — `ErrInvalidPosting`.
  - Файлы: `internal/payout/domain/{contractor.go,payout.go,schedule.go,limits.go,ports.go,errors.go,*_test.go}`, `internal/ledger/domain/{transaction.go,builders.go,transaction_test.go}`.

- [x] **Task 3: Контракт адаптера и `rocketworkfake` (R-1, R-2, R-6, R-11, R-18)** (depends on 2)
  - `internal/integration/rocketwork/types.go`: запросы и ответы контракта 05 §4.2 в наших терминах, коды отказов, `ErrDisabled`; `disabled.go` — реализация режима `disabled`.
  - `internal/integration/rocketwork/redact.go`: маскирование ИНН, ФИО, телефона в теле для `outbound_request` (образец — `integration/ord/redact.go`); `webhook.go`: общая проверка HMAC (для подделки и будущего клиента), лимит тела.
  - `internal/integration/rocketwork/rocketworkfake`: `New()`, исполнители и выплаты в памяти; управление тестами: `ActivateContractor`, `BlockContractor(reason)`, `SetDeposit`, `CompletePayout(fee)`, `FailPayout(code)`, `FailNext`, `TimeoutNext`, `UnknownStatusNext`, `Calls()`; `SignedWebhook(event) (header, body)`; `CreatePayout` идемпотентен по ключу, `FindPayout` по ключу; списание депозита при успехе.
  - `internal/bootstrap/bridge/payout.go`: `PayoutProvider` — приведение типов адаптера к `payout/domain.Provider`.
  - Журнал: подделка — DEBUG `rocketworkfake call` (операция, ключ); `disabled` — WARN `rocketwork disabled` один раз на операцию в минуту.
  - Тесты: идемпотентность `CreatePayout` и `EnsureContractor` (по ИНН); подпись — верная, чужой секрет, изменённое тело, пустой заголовок; маскирование тела; неизвестный статус → `pending`/`sent`.
  - Файлы: `internal/integration/rocketwork/{types.go,disabled.go,redact.go,webhook.go,*_test.go}`, `internal/integration/rocketwork/rocketworkfake/{fake.go,fake_test.go}`, `internal/bootstrap/bridge/payout.go`.

- [x] **Task 3a: HTTP-клиент Рокет Ворк по предполагаемому контракту (R-1, I-6)** (depends on 3)
  - `internal/integration/rocketwork/client.go`: `Client` через `platform/outbound` (`SystemRocketWork`) и `ratelimit` (как `integration/ord`): `EnsureContractor`, `GetContractor`, `GetDepositBalance`, `CreatePayout` (`Idempotency-Key`), `GetPayout`, `FindPayout`, `ParseWebhook` — по разделу «Допущения о внешних данных»; каждое допущение — `// ПРЕДПОЛОЖЕНИЕ (Q-NN): …`. Приведение статусов и кодов — общие функции `types.go` (их же использует подделка).
  - Журнал: `outbound_request` с маскированным телом (`redact.go`); DEBUG тела ответа.
  - Тесты (`httptest`): каждый метод, `Idempotency-Key`, `401`, `422 insufficient_funds`, `404`, `5xx`, неизвестный статус.
  - Файлы: `internal/integration/rocketwork/{client.go,client_test.go}`.
<!-- Commit checkpoint: tasks 1-3a -->

### Phase 2: Хранилище и сценарии

- [ ] **Task 4: `payout/postgres` (R-5, R-9)** (depends on 1, 2)
  - `internal/payout/postgres/queries/{contractors.sql,payouts.sql,reports.sql}` + `sqlc.yaml`: исполнитель по организации и по `external_id` с `FOR UPDATE`, вставка с `ON CONFLICT (organization_id) DO NOTHING` + повторное чтение; выплата по `id`/`external_id`/`idempotency_key` с `FOR UPDATE`, последняя попытка размещения, обновление с проверкой `lock_version`; список выплат организации (курсор `created_at, id`), итоги (`succeeded`, в работе), сумма `succeeded` за год, выборки для `stuck`, сумма `blogger_amount` размещений в `published`/`removed` с исполнителем Рокет Ворк (только чтение `placement`).
  - `internal/payout/postgres/store.go`: реализация `domain.Store` в транзакции вызывающего (`tx.FromContext`).
  - Журнал: DEBUG `payout store` при конфликте `lock_version` (повторное чтение).
  - Тесты (`dbtest`): гонка двух обновлений одной выплаты — одно проходит; курсор без пропусков и повторов; годовая сумма по Москве на границе 31 декабря / 1 января.
  - Файлы: `internal/payout/postgres/{store.go,store_test.go,queries/*.sql,sqlc/*}`, `sqlc.yaml`.

- [ ] **Task 5: Онбординг, гейт и создание выплаты (R-3, R-4, R-13, R-15, R-18)** (depends on 3, 4)
  - `internal/payout/app/service.go`: `Deps{Store, Provider, Ledger, Placements, Organizations, Tx, Queue, Alerts, Audit, Notifier, Metrics, Clock, Log, Config}`.
  - `internal/payout/app/contractors.go`: `Status(ctx, orgID)` (юрлицо → `required=false`, `none`); `Invite(ctx, orgID, userID)` (см. ниже); `Refresh(ctx, orgID)` — ставит `poll_contractor` сейчас; `ApplyContractor(ctx, ProviderContractor)` — общий для опроса и вебхука, уведомления `rocketwork.active`/`rocketwork.blocked`; `CanAccept(ctx, orgID)` — реализация `placement/domain.ContractorGate`.
    - I-13: `Invite` не вызывает Рокет Ворк в запросе. Он создаёт строку `invited` без ссылки и задание `ensure_contractor` (`dedupe_key` `ensure_contractor:{organization_id}`, 5 попыток), отвечает `202` с текущим статусом; ссылка появляется после задания (кабинет повторяет `getRocketworkStatus`). Повторный `Invite` при наличии ссылки — тот же ответ без задания.
  - `internal/payout/app/create.go`: `Create(ctx, PayoutRequest)` в транзакции вызывающего: метод по `legal_type` (R-3), строка `created`, задание `send_payout:{payout_id}` (`max_attempts` 10) для `rocketwork`, проверка лимита НПД (R-13).
  - `internal/payout/app/succeed.go`: `complete(ctx, payout, ProviderPayout)` — R-5 (выплата, проводка, `MarkPaid`, уведомление `payout.succeeded`, метрика), `fail(ctx, payout, code, msg)` — R-8 (уведомление `payout.failed`, оповещения).
  - Журнал: INFO `rocketwork contractor invited`, `rocketwork contractor status changed`, `payout created` (`method`, `amount`, `attempt`); WARN `npd income limit approaching`; INFO `payout succeeded`; WARN/ERROR `payout failed`; ERROR `payout placement state mismatch`. ИНН, ФИО, телефон — маскированными.
  - Тесты (`dbtest`, `rocketworkfake`, `ledgertest`, `placementtest`): юрлицо — `required=false`, гейт пропускает, `Create` → `bank_transfer` без задания; самозанятый `invited` — гейт не пропускает, после `active` — пропускает; `Create` дважды для одного размещения — вторая живая выплата не создаётся; успех — проводка `payout` из двух строк, `MarkPaid` вызван, сходимость журнала; повтор успеха — no-op.
  - Файлы: `internal/payout/app/{service.go,contractors.go,create.go,succeed.go,*_test.go}`, `internal/payout/payouttest/payouttest.go`.

- [ ] **Task 6: Задания отправки и опроса выплат (R-6, R-7, R-8, R-10)** (depends on 5)
  - `internal/payout/app/send.go`: `Send(ctx, payoutID)` — вне транзакции: актуальность исполнителя (R-7), `GetDepositBalance`, `CreatePayout` с ключом, затем в транзакции `MarkSent`, уведомление `payout.sent`, задание `poll_payout:{payout_id}`. Ответ сразу окончательный — `complete`/`fail`. `insufficient_deposit` → `job.Postpone(1h)` + оповещение; `provider_error` → `job.Retry`; `OnExhausted` → `fail(provider_error)`.
  - `internal/payout/app/poll.go`: `PollPayout(ctx, payoutID)` — `GetPayout`, окончательный статус → `complete`/`fail`, иначе `job.Postpone` на `PollDelay` (I-3); `PollContractor(ctx, contractorID)`, `EnsureContractor(ctx, orgID)`.
  - `internal/payout/jobs/jobs.go`: `Register(w, svc, Options)` — `send_payout`, `poll_payout`, `poll_contractor`, `ensure_contractor` с полезной нагрузкой JSON, `ObjectRef`, `job.Permanent` при ошибке разбора (образец — `marking/jobs`).
  - Журнал: INFO `payout sent` (`external_id`); ERROR `rocketwork deposit insufficient` (`deposit`, `required`); WARN `payout send retry` (`attempt`, `error`); DEBUG `payout poll` (`next_run_at`); ERROR `payout send exhausted`.
  - Тесты: успех; ответ «выполнено» сразу; таймаут создания — `FindPayout` находит, второй выплаты в сервисе нет; таймаут и не найдена — повтор с тем же ключом; нехватка депозита — выплата `created`, попытка не потрачена, после `SetDeposit` — отправлена; исполнитель заблокирован — `failed contractor_not_active`; 10 ошибок — `failed provider_error`; опрос на 5-й и 65-й минуте; `disabled` — откладывание без смены статуса.
  - Файлы: `internal/payout/app/{send.go,poll.go,*_test.go}`, `internal/payout/jobs/{jobs.go,jobs_test.go}`.

- [ ] **Task 7: Порт выплат в размещениях и P-16 (R-4, R-5)** (depends on 5)
  - `internal/placement/domain/ports.go`: `Payouts{Create(ctx, PayoutRequest) (uuid.UUID, error)}` с комментарием «вызывается из P-12, P-13, P-15 на этапе приёмки».
  - `internal/placement/app/service.go`: `SetPayoutPorts(gate, payouts)` (I-1); `bridge.NoPayouts` до подключения.
  - `internal/placement/app/paid.go`: `MarkPaid(ctx, placementID, payoutID)` — P-16 (`ActorSystem`) через `apply` в транзакции вызывающего, история с `payout_id`; размещение не `accepted` — `ErrInvalidTransition`.
  - `internal/bootstrap/bridge/payout.go`: `PlacementPayouts` (→ `payout/app.Create`), `PayoutPlacements` (→ `placement/app.MarkPaid`, `Brief`), `PayoutLedger` (→ `ledger.Service.Post` с `NewPayout`/`NewLegalPayout`, итоги), `PayoutOrganizations` (→ новый `accountpg.Organizations.ForPayout`).
  - `internal/account/postgres`: `ForPayout(orgID)` — ID, `legal_type`, ИНН, ФИО из `blogger_profile`, телефон.
  - `internal/placement/placementtest`: `Payouts` (поддельный, `Created()`).
  - Журнал: INFO `placement paid` (`placement_id`, `payout_id`) — общим журналом переходов.
  - Тесты: P-16 из `accepted` — `paid`, история; из `removed` — отказ; `ForPayout` для самозанятого и юрлица.
  - Файлы: `internal/placement/domain/ports.go`, `internal/placement/app/{paid.go,paid_test.go}`, `internal/placement/placementtest/placementtest.go`, `internal/bootstrap/bridge/payout.go`, `internal/account/postgres/{organizations.go,organizations_test.go,queries/*.sql}`.
<!-- Commit checkpoint: tasks 4-7 -->

### Phase 3: Вебхук, депозит, команды администратора

- [ ] **Task 8: Вебхук Рокет Ворк (R-10, R-11)** (depends on 6)
  - `internal/payout/httpapi/webhook.go`: `Webhook.ServeHTTP` — лимит 1 МиБ, `Provider.ParseWebhook`, отказ — `401` и счётчик отказов с оповещением `webhook_auth_failed:rocketwork` (образец — `channel/httpapi/webhook.go`), `inbound.Recorder.Record(Event{Source: SourceRocketWork, ExternalKey, EventType})`, ответ `200` независимо от разбора; `disabled` — `404`.
  - `internal/payout/jobs/jobs.go`: `Dispatcher.Handle(SourceRocketWork, "contractor_status" | "payout_status", …)` — событие несёт только внешний идентификатор; обработчик запрашивает свежее состояние (`GetContractor`/`GetPayout`) и применяет через `ApplyContractor`/`complete`/`fail`. Неизвестный объект — `app.ErrIgnored`.
  - `internal/bootstrap/payout.go`: маршрут `POST {BaseURL}/webhooks/rocketwork` в `Routes` (вне строгого сервера; CSRF уже исключён по префиксу).
  - Журнал: WARN `rocketwork webhook rejected` (`reason`, `remote_ip`); DEBUG `rocketwork webhook received` (`event_type`, `duplicate`); INFO `rocketwork event processed`; DEBUG `rocketwork event ignored`.
  - Тесты: подписанное событие — `inbound_event`, обработка переводит выплату в `succeeded` и размещение в `paid`; повтор события — дубликат; неверная подпись — `401`, пятый отказ — оповещение; тело > 1 МиБ — `413`.
  - Файлы: `internal/payout/httpapi/{webhook.go,webhook_test.go}`, `internal/payout/jobs/jobs.go`, `internal/bootstrap/payout.go`.

- [ ] **Task 9: Контроль депозита, лимит, зависшие (R-12, R-14)** (depends on 6)
  - `internal/payout/app/deposit.go`: `CheckDeposit(ctx)` — R-12, оповещения `rocketwork_deposit_low`, `rocketwork_deposit_mismatch` и их закрытие, метрика; `DepositReport(ctx)` для `reconcile`.
  - `internal/payout/jobs/jobs.go`: периодическое `check_rocketwork_deposit` (`ROCKETWORK_DEPOSIT_CHECK_INTERVAL`, 1 ч, `max_attempts` 1).
  - `internal/payout/app/stuck.go`: проверки `stuck.Check` R-14 — регистрируются в `m.platform.stuck`; убрать заглушку-комментарий в `internal/platform/stuck/stuck.go`.
  - Метрики: `maxim_payouts_total{status,method}`, `maxim_payout_amount_kopecks_total`, `maxim_rocketwork_deposit_kopecks{source}`, `maxim_rocketwork_contractors{status}`.
  - Журнал: INFO `rocketwork deposit checked` (`api`, `ledger`, `need`); ERROR `rocketwork deposit low` / `rocketwork deposit mismatch`; INFO `rocketwork deposit alert resolved`.
  - Тесты: депозит меньше обязательств — оповещение, после пополнения — закрыто; расхождение 1,00 ₽ — нет, 1,01 ₽ — есть; выплаты юрлицам и их размещения в расчёт не входят; три проверки `stuck` на границах порогов.
  - Файлы: `internal/payout/app/{deposit.go,stuck.go,*_test.go}`, `internal/payout/jobs/jobs.go`, `internal/platform/stuck/stuck.go`.

- [ ] **Task 10: Команды администратора (R-3, R-9, R-12)** (depends on 6, 9)
  - `internal/bootstrap/admin_payout.go`: `maxim admin payout list [--status] [--org-inn] [--from --to]` (таблица: дата, блогер — маскированный ИНН, размещение, попытка, сумма, состояние, код); `retry --payout --email --reason` (R-9); `mark-paid --payout --bank-ref --email --reason` (R-3, только `bank_transfer`, `senior`; сумма выше `LEDGER_MANUAL_SECOND_APPROVAL_THRESHOLD` — `--approver-email`); `contractor refresh --org-inn`.
  - `internal/bootstrap/admin_money.go`: `deposit-topup --amount --email --reason [--key]` — `ledger.NewDepositTopup` (`senior`, порог второго администратора, ключ печатается); `reconcile` дополняется строкой «Рокет Ворк: журнал / API / расхождение» из `DepositReport`.
  - `internal/payout/app/admin.go`: `Retry`, `MarkLegalPaid` с `audit_log`.
  - Журнал: WARN `admin payout retry`, `admin payout marked paid`, `admin deposit topup` (`admin_user_id`, `amount`, `key`) + `audit_log`.
  - Тесты (`admin_payout_test.go`): повтор не из `failed` — отказ; повтор — попытка 2 и задание; `mark-paid` — проводка `blogger_payable → bank`, P-16, сходимость; без роли `senior` — отказ; `deposit-topup` повтор с тем же ключом — no-op.
  - Файлы: `internal/bootstrap/{admin_payout.go,admin_payout_test.go,admin_money.go,admin_money_test.go,admin.go}`, `internal/payout/app/{admin.go,admin_test.go}`.
<!-- Commit checkpoint: tasks 8-10 -->

### Phase 4: Кабинет блогера, конфигурация, подключение

- [ ] **Task 11: Конфигурация и подключение модуля (R-1, R-4)** (depends on 7, 8, 9, 10)
  - `internal/bootstrap/config_rocketwork.go`: `RocketWorkConfig` — `ROCKETWORK_MODE` (`fake`/`disabled`/`live`; в `production` не `fake`, R-1), `ROCKETWORK_BASE_URL`, `ROCKETWORK_TOKEN` (обязателен при `live`), `ROCKETWORK_RATE`, `ROCKETWORK_WEBHOOK_SECRET` (обязателен при `fake` вне `test`), `ROCKETWORK_DEPOSIT_CHECK_INTERVAL`, `ROCKETWORK_DEPOSIT_TOLERANCE`, `ROCKETWORK_CONTRACTOR_POLL_MAX_AGE`, `PAYOUT_POLL_FAST`, `PAYOUT_POLL_SLOW`, `PAYOUT_NPD_ANNUAL_LIMIT`; проверки границ; маскированная выдача (`secret(...)`).
  - `internal/bootstrap/payout.go`: `newPayoutService(...)`; `integration.go` — `connectRocketWork` (`infra.rocketwork`; в `test` — подделка из `harness`); `modules.go` — поле, `registerJobs`, `stuck`, маршрут вебхука, обработчики в `apiImpl`; `placement.go` — `Contractors` и `Payouts` из модуля выплат через `SetPayoutPorts` после сборки `payout` (I-1).
  - `internal/bootstrap/dev.go`: `maxim dev rocketwork activate|block|deposit|complete|fail …` и `maxim dev placement accept --placement --share` (R-4). Подделка используется процессами `api` и `worker`, поэтому в `development` `rocketworkfake` хранит состояние в JSON-файле `ROCKETWORK_FAKE_STATE_PATH` (общий для процессов, без новых таблиц); команда меняет состояние и при `--webhook` отправляет подписанный вебхук на `api`.
  - `.env.example`, `deploy/.env.example`, `compose.dev.yml`: новые переменные.
  - Журнал: INFO `rocketwork configured` (`mode`, интервалы); WARN при `disabled` в `production` («выплаты не отправляются до подключения клиента»).
  - Тесты: проверки конфигурации (`fake` в `production` — ошибка); `wiring_test.go` — виды заданий и политики, `placement` получил настоящий гейт и порт выплат, обработчик `rocketwork` зарегистрирован в диспетчере.
  - Файлы: `internal/bootstrap/{config.go,config_rocketwork.go,config_rocketwork_test.go,payout.go,integration.go,modules.go,placement.go,dev.go,wiring_test.go}`, `internal/integration/rocketwork/rocketworkfake/state.go`, `.env.example`, `deploy/.env.example`, `compose.dev.yml`.

- [ ] **Task 12: Операции кабинета блогера и OpenAPI (R-16)** (depends on 11)
  - `api/openapi.yaml`: изменения R-16; `go generate ./...` (`internal/apigen`).
  - `internal/payout/httpapi/handlers.go`: `ListBloggerPayouts` (номер заказа и канал через `Placements.Brief`), `GetBloggerPayoutSummary`, `GetRocketworkStatus`, `InviteRocketwork` (`202`, `409 contractor_not_required`, `503 rocketwork_unavailable`), `RefreshRocketworkStatus`; только роль блогера своей организации; ограничение частоты `invite`/`refresh` (`platform/ratelimit`, 10 в час на организацию, `429`).
  - `internal/bootstrap/unimplemented.go`: убрать реализованные операции.
  - `inviteRocketwork`: ответ `200` → `202` (I-2); `listBloggerPayouts` заполняет `total` (I-4).
  - `mockapi/internal/{store,handlers}`: новые поля и операции, `202`, `409 details.reason = contractor_not_required` (I-2), контрактный тест и тест ошибок в `gaps_test.go`.
  - Журнал: DEBUG на чтение; INFO `rocketwork invite requested`; WARN `rate limited`.
  - Тесты (`payout_http_test.go`): контрактная проверка ответов по OpenAPI; чужая выплата не видна; юрлицо — `409`; `disabled` — `503`; курсор; итоги.
  - Файлы: `api/openapi.yaml`, `internal/apigen/api.gen.go`, `internal/payout/httpapi/{handlers.go,handlers_test.go}`, `internal/bootstrap/{unimplemented.go,payout_http_test.go}`, `mockapi/internal/store/*.go`, `mockapi/internal/handlers/*.go`.
<!-- Commit checkpoint: tasks 11-12 -->

### Phase 5: Проверка и документы

- [ ] **Task 13: Сквозные тесты выплат** (depends on 12)
  - `internal/bootstrap/payout_e2e_test.go` на собранных `api` и `worker` (PostgreSQL, `clock.Manual`, `storagefake`, `ordfake`, `maxfake`, `mailfake`, `rocketworkfake`):
    - S-10 → S-11: самозанятый блогер, `inviteRocketwork` → `202`, после `ensure_contractor` — ссылка, принятие размещения — `409 contractor_inactive`; подделка активирует, опрос через 5 мин — `active`, принятие проходит.
    - S-15: пополнение баланса рекламодателя и депозита (`deposit-topup`), заказ, публикация, снятие, `dev placement accept` с долей 10000 → выплата `created` → `send_payout` → `sent` → подписанный вебхук → `succeeded`, размещение `paid`, `receipt_url` в `listBloggerPayouts`, итоги сходятся; проводки `settlement` и `payout` (с комиссией сервиса) в журнале.
    - Опрос без вебхука: выплата завершается на опросе через 65 мин.
    - Нехватка депозита — оповещение, выплата ждёт; `deposit-topup` — отправлена на следующем часе.
    - `tax_status_lost` — `failed`, размещение `accepted`; `admin payout retry` — попытка 2, успех.
    - Юрлицо: гейт пропускает, выплата `bank_transfer`, `admin payout mark-paid` — `paid`, проводка из `bank`.
    - T-12: выплата не прошла, через 72 ч — оповещение `payout_accepted_unpaid`.
    - `check_ledger_balance` и `check_rocketwork_deposit` после всех шагов — без оповещений; `admin money reconcile` — расхождения по Рокет Ворк нет.
  - Проверка журнала сборщиком записей: события 06 §11 присутствуют; ИНН только маскированным; ФИО, телефон, ссылки онбординга и чека, секрет вебхука не встречаются.
  - Файлы: `internal/bootstrap/payout_e2e_test.go`, при необходимости `internal/bootstrap/harness_test.go`.

- [ ] **Task 14: Синхронизация документов с решениями этапа** (depends on 13)
  - После подтверждения R-1 … R-18 внести их в документы:
    - 02 S-11 — задание `ensure_contractor`, «Проверить статус» (R-10); S-15 — проверки перед отправкой, коды, юрлицо командой (R-3, R-7, R-8); S-19 — `admin payout retry`, `mark-paid`, `deposit-topup` (R-9).
    - 03 §4 — P-16 одной транзакцией с проводкой (R-5); порт `Payouts` для P-12/P-13/P-15 (R-4).
    - 04 §7 — `payout.method`, `blogger_organization_id`, `bank_reference`, `created_by`, `rocketwork_contractor.blocked_reason`, внешний ключ `ledger_transaction.payout_id`.
    - 05 §4 — интерфейс `Provider`, режимы `fake`/`disabled`, приведение статусов в адаптере, опрос всегда, проверка подписи в адаптере, `FindPayout` (R-1, R-2, R-6, R-10, R-11).
    - 06 §2.3 — `payout` юрлицу `blogger_payable → bank` (R-3); §2.4 — сверка депозита и допуск (R-12); §4 — `PAYOUT_NPD_ANNUAL_LIMIT` (R-13).
    - 07 §2 — `ensure_contractor`, расписания и `Postpone` для нехватки депозита; §3 — проверки R-14; §4 — `webhook_auth_failed:rocketwork`; §5.5 — метрики; §8 — новые параметры.
    - `docs/open-questions.md` — аннулирование чека, возврат выплаты и остатка депозита (R-17) к Q-12; формат подписи вебхука Рокет Ворк (аналог Q-35); Q-36 — рабочий порядок `mark-paid`.
    - `.ai-factory/ARCHITECTURE.md` — модуль `payout` (слои, порты, `bridge.Payout*`), `integration/rocketwork` и режимы.
  - Журнал: не применимо.
  - Файлы: `docs/02-scenarios.md`, `docs/03-placement.md`, `docs/04-domain-model.md`, `docs/05-integrations.md`, `docs/06-finance-legal.md`, `docs/07-non-functional.md`, `docs/open-questions.md`, `.ai-factory/ARCHITECTURE.md`.
<!-- Commit checkpoint: tasks 13-14 -->

## Критерии готовности этапа

- `go build ./...`, `go vet ./...`, `go test -race ./...` проходят локально и в CI. `sqlc generate` и `go generate ./...` не дают изменений. `mockapi` собирается и проходит контрактный тест.
- Самозанятый или ИП не принимает размещение без `active` исполнителя; юрлицо принимает (A-36).
- У размещения не больше одной живой выплаты; каждая попытка — отдельная строка со своим ключом; повторная отправка не создаёт вторую выплату в сервисе.
- Успех выплаты, проводка `payout` и P-16 — одна транзакция; повтор события ничего не меняет.
- Ни один вызов Рокет Ворк не выполняется в обработчике запроса или в транзакции перехода (I-13).
- Вебхук без верной подписи отклоняется до сохранения; выплаты завершаются и без вебхуков (опрос).
- Нехватка и расхождение депозита дают оповещение раз в час; `reconcile` показывает сверку с сервисом.
- ИНН (кроме маскированного), ФИО, телефон, ссылки онбординга и чека, секреты отсутствуют в журнале.
- В `production` модуль запускается в `disabled` (по умолчанию) или `live`; `live` опирается на «Допущения о внешних данных» и включается после их сверки (этап «Проверки и доступы»).
- Решения R-1 … R-18 подтверждены и внесены в документы (задача 14).
