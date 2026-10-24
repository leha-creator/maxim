# План реализации: Приёмка, споры и возвраты

Branch: feature/acceptance-disputes
Created: 2026-10-24
Refined: 2026-10-24 (`/aif-improve`, сверка с кодом)

## Settings
- Testing: yes. Правила претензии (окно T-05, одна претензия, причины, длина текста), исходы P-15 и доли (`full_payout` = 10000, `full_refund` = 0, `proportional` 1…9999, предложение системы по умолчанию), I-02, штраф рейтинга, действия кабинета — чистые табличные тесты `placement/domain`. Сценарии `placement/app` (P-12, P-13, P-14 рекламодателем и администратором, P-15 всеми исходами, снятие поста при возврате, удаление администратором) — на настоящей PostgreSQL (`platform/dbtest`), `clock.Manual`, `placementtest`. Деньги — на настоящем журнале в сквозных тестах (`harness_test.go`, собранные `api` и `worker`): после каждой денежной операции `ledger.CheckBalance` без оповещений. Операции кабинета — проверка ответов по OpenAPI; `mockapi` — контрактный тест и тест ошибок (`gaps_test.go`).
- Logging: verbose. `log/slog` JSON, события 03 §12, 06 §11, 07 §5.3: переход выполнен (INFO, общий журнал переходов), отклонён (WARN: `transition`, `status`, `reason`); претензия подана (INFO: `placement_id`, `claim_id`, `reason`; текст претензии не пишется); спор открыт (INFO: `dispute_id`, `opened_by`, `reason`, `suggested_share`, `deadline_at`); спор решён (WARN + `audit_log`: `admin_user_id`, `resolution`, `performance_share`, `payout_amount`, `fee_charged`, `refund_amount`; комментарий администратора не пишется); автоприёмка (INFO `placement timer fired` с `T-05`); рейтинг канала снижен (INFO: `channel_id`, `from`, `to`); удаление поста администратором и при возврате (WARN/INFO: `removal_reason`); нарушение I-02 (ERROR). Поля `placement_id`, `order_id`, `channel_id`, `dispute_id`, `claim_id`, `payout_id`, `job_id`.
- Docs: yes. В конце обязательная точка: 02 S-08, S-18, S-19, S-24 («Как сделано»), 03 §4 и §5 (P-12…P-15, T-05), 04 §5 (`claim`, `dispute`), 06 §2.3 и §3, 07 §2 и §8, `docs/launch/legal-documents.md` (штраф рейтинга), `docs/open-questions.md`, README команд (`maxim admin dispute …`).

## Roadmap Linkage
Milestone: "Приёмка, споры и возвраты"
Rationale: план реализует этап целиком: приёмку рекламодателем (P-12, S-08) и автоприёмку по T-05 (P-13, S-24) с распределением удержания и созданием выплаты, претензию (S-08, P-14), решение администратора (S-18, P-15) всеми тремя исходами с полным и пропорциональным возвратом через журнал, снятие поста при полном возврате, открытие спора и удаление поста администратором (S-19), уведомление `placement.dispute_resolved` и санкцию рейтингом (A-08). Операции администратора — команды `maxim admin dispute …`; их HTTP-API — этап «API администратора и отчётность ОРД».

## Источники ТЗ

- Сценарии: 02 S-08 (приёмка и претензия), S-18 (разбор спора), S-19 (`admin_remove_post`, `admin_open_dispute`), S-24 (автоприёмка), S-25…S-27 (споры системы — уже есть), S-15 (выплата после приёмки), S-33 (возврат остатка — уже есть).
- Размещение: 03 §3, §4 (P-12, P-13, P-14, P-15, особые правила про выплату и удаление в `disputed`), §5 (T-05, T-06, T-12), §6 (причины, санкции), §7 (распределение и предложение доли), §10 (I-02, I-04, I-10, I-11, I-12, I-13), §11 (ключи идемпотентности для «принять» и «подать претензию»), §12.
- Данные: 04 §5 (`claim`, `dispute`, `dispute_message`, `placement` поля распределения, проверка `placement_distribution_sum`), §3 `channel.rating`, §9 (`claim.reason`).
- Финансы: 06 §2.2 (ключ `settlement:{placement_id}`), §2.3 (`settlement` для P-12, P-13, P-15 с выплатой; `hold_release` для `full_refund`), §11.
- Требования: 07 §2 (`placement_timer`, `remove_placement`), §4 (роли администратора: споры — `moderator` и `senior`), §5.3–§5.5, §8 (`PLACEMENT_CLAIM_WINDOW`, `DISPUTE_DEADLINE`, `DISPUTE_TOP_VIOLATION_SHARE`).
- Правовое: `docs/launch/legal-documents.md` — строки «Срок для претензии» (24 ч), «Срок решения спора» (72 ч), «Основания», «Исходы», «Пропорциональный возврат», «Предложение доли при нарушении топа» (5000), «Приёмка» (автоприёмка через 24 ч). Сроки реализации совпадают с таблицей.
- Реестр: A-08, A-18, D-26, Q-03, Q-04, Q-12, Q-18.
- Контракт: `api/openapi.yaml` — `acceptPlacement`, `createPlacementClaim`, схемы `ClaimCreate`, `ClaimReason`, `PlacementClaim`, `PlacementDisputeSummary`, `AdvertiserPlacement.available_actions`. Операций споров администратора в OpenAPI нет.
- Существующий код:
  - `internal/placement/domain/{dispute.go (Dispute, NewSystemDispute, SuggestedShare),distribution.go (Distribute),transitions.go (P-12…P-15),publish.go (Remove, OpenDispute),timers.go (TimerClaim, TimerDispute, RemoveJob),order.go (AdvertiserActions),ports.go (Holds.Settle, Payouts, Disputes)}`;
  - `internal/placement/app/{devaccept.go (образец P-12 с Settle и Payouts.Create),violations.go (P-14 системой),timers.go (T-05 ждёт этапа, T-06),remove.go,advertiser.go (ключ идемпотентности, audit),transition.go (apply, record, ReleaseHold)}`;
  - `internal/placement/postgres/queries/{disputes.sql,placements.sql (SetDistribution)}`, `migrations/00008_publications.sql` (`dispute.claim_id` без ключа), `00010_money.sql` (`placement_distribution_sum`);
  - `internal/ledger/app/holds.go (Settle, Release)`, `internal/bootstrap/bridge/ledger.go (LedgerHolds.Settle — всегда actor system)`;
  - `internal/notification/domain/catalog.go (placement.dispute_resolved AwaitsModule)`, `templates/notification/placement.dispute_resolved.tmpl`;
  - `internal/bootstrap/{admin.go,admin_payout.go (образец команды),dev_payout.go (dev placement accept),unimplemented.go,placement.go,config.go}`;
  - `mockapi/internal/{store/operations.go (AcceptPlacement, ClaimPlacement),handlers/orders.go}`.

## Границы этапа

Входит:
- Таблица `claim`, внешний ключ `dispute.claim_id`, проверка соответствия доли решению.
- P-12 (`acceptPlacement`), P-13 (T-05), P-14 рекламодателем (`createPlacementClaim`) и администратором, P-15 (решение) — с распределением удержания через журнал (`settlement`/`hold_release`), выплатой (`Payouts.Create`), уведомлениями и журналом действий.
- Снятие живой публикации при `full_refund` (причина `refund`) и `admin_remove_post` (причина `admin`).
- Снижение рейтинга канала по решению `proportional` и `full_refund` (A-08).
- Команды `maxim admin dispute list|show|open|resolve|remove-post`.
- Изменения OpenAPI кабинета рекламодателя (ключ идемпотентности, причины `409`, решение в сводке спора) и `mockapi`.

Не входит (этап «API администратора и отчётность ОРД»):
- HTTP-операции администратора по спорам (очередь, карточка, решение), экран `adm/dispute`.
- Переписка по спору (`dispute_message`, S-18 шаг 2): нет ни операций кабинетов, ни шаблона уведомления; таблица и операции появятся вместе с API администратора.
- `admin_cancel`, `admin_mark_removed`, `block_channel` (S-19) — не связаны со спорами.
- Возврат выплаты в Рокет Ворк, аннулирование чека: спор возможен только до `accepted`, решение окончательное (S-18), поэтому после выплаты возвращать нечего (Q-12).
- Возврат денег рекламодателю на расчётный счёт: возврат по спору приходит на баланс (06 §2.3), вывод — существующая `maxim admin money balance-return` (D-26, S-33).

## Решения этапа, требующие подтверждения

Документы этих мест не определяют или определяют не полностью. План реализует указанный вариант (рекомендуемый, без вопроса пользователю). Задача 12 вносит решения в документы.

| # | Вопрос | Решение в плане | Задача |
|---|---|---|---|
| R-1 | Претензия: хранение и проверки (S-08) | Таблица `claim` (04 §5): `placement_id` уникален, `reason` ∈ `removed_early`, `top_violated`, `content_mismatch`, `other`, `text` 1…2000 символов (после обрезки пробелов), `created_by`. Неизменяема. Претензия и спор (`opened_by_type = advertiser`, `claim_id`, `reason` = причина претензии) создаются в транзакции P-14 | 1, 2, 5 |
| R-2 | Окно претензии | Из `published` — всегда; из `removed` — пока `now < claim_deadline_at`; позже — `409 invalid_transition`, `details.reason = claim_window_closed`. Если спор уже открыт системой или администратором (`disputed`), или претензия есть — `409 conflict`, `details.reason = claim_exists` (претензия есть) либо `invalid_transition` с фактическим состоянием (спор без претензии) | 2, 5, 8 |
| R-3 | Предложение доли для претензии | Как у системы (03 §7): `removed_early` — по публикации, если она снята (`removed_at` публикации), иначе предложения нет; `top_violated` — `DISPUTE_TOP_VIOLATION_SHARE`; остальные — нет | 2, 5 |
| R-4 | Ключ идемпотентности для «принять» и «подать претензию» (03 §11) | В `acceptPlacement` и `createPlacementClaim` добавляется необязательный заголовок `Idempotency-Key` (как у `cancelPlacement`); повтор с тем же ключом возвращает текущее размещение без перехода (`200`/`201`). Без ключа повтор — `409` | 4, 5, 8 |
| R-5 | Общая приёмка (P-12, P-13, P-15 `accepted`) | Одна функция `placement/app.settleAccepted` в транзакции перехода: проверка I-02 (публикация снята, иначе ERROR и `409 invalid_transition` `publication_live`), `Distribute`, `SetDistribution` до смены состояния (проверка `placement_distribution_sum`), переход, `Holds.Settle`, `Payouts.Create` при `payout_amount > 0`. `DevAccept` вызывает её же и остаётся только для `development`/`test` | 4 |
| R-6 | Актор проводки `settlement` | Порт `Holds.Settle` получает актора (`Settler{Actor, UserID}`): P-12 — `advertiser` и пользователь, P-13 — `system`, P-15 — `admin` и администратор. `hold_release` при `full_refund` — через существующий `Release` (актор `system`, причина `dispute_full_refund`) | 4, 6, 9 |
| R-7 | Автоприёмка | T-05 → P-13 (актор `system`, причина `auto_accept`); размещение не `removed` — таймер устарел. Попытки таймера — 5, как у других таймеров (ожидание «до этапа приёмки» убирается). Уже стоящие в очереди T-05 с повтором раз в час выполнятся при следующем запуске | 4 |
| R-8 | Решение администратора (P-15) | `resolve --resolution full_payout|proportional|full_refund [--share N] --comment …`: `full_payout` — 10000, `full_refund` — 0, `proportional` — `--share` 1…9999, без `--share` — предложение системы (`suggested_share` вне 1…9999 или его нет — `422 share_required`). Комментарий обязателен (до 2000 символов). Решение окончательное: повтор — `409 dispute_resolved` (S-18). Роль — `moderator` или `senior` (07 §4). В одной транзакции: решение в `dispute`, распределение, переход, проводки, выплата, отмена T-06, уведомление, рейтинг, `audit_log`; после — закрытие оповещений `placement_disputed` и `dispute_overdue` | 2, 6, 9 |
| R-9 | `accepted` при живой публикации | Отказ `409 invalid_transition`, `details.reason = publication_live` (I-02): администратор ждёт снятия по сроку или удаляет пост (`remove-post`) | 2, 6 |
| R-10 | `full_refund` при живой публикации | Переход в `refunded` (удержание снимается `hold_release` в той же транзакции), задание снятия переставляется на «сейчас» с причиной `refund`; `remove_placement` удаляет пост и в `refunded`, состояние не меняется | 6, 7 |
| R-11 | `admin_remove_post` (S-19) | `published` или `disputed` с живой публикацией: в одной транзакции — P-14 администратором с причиной `admin` (если спора нет) и снятие «сейчас» с причиной `admin`. Иначе `409` | 6, 7 |
| R-12 | `admin_open_dispute` (S-19) | P-14 администратором из `published`/`removed` с причиной `admin`; основание (`--reason`) — в `audit_log` и `reason_text` истории; предложения доли нет. T-05 отменяется, T-06 ставится | 6 |
| R-13 | Санкция рейтингом (A-08, 03 §6) | `channel.rating` уменьшается в транзакции P-15: `full_refund` — на `DISPUTE_RATING_PENALTY_REFUND` (10), `proportional` — на `DISPUTE_RATING_PENALTY_PROPORTIONAL` (5), не ниже 0; `full_payout` — без изменений. Блокировка канала — решение администратора (не автоматически) | 2, 3, 6, 9 |
| R-14 | Уведомления | P-15 — `placement.dispute_resolved` обоим (`Data`: `resolution`, `performance_share`, `refund_amount`, `payout_amount`). P-12, P-13 — без отдельного шаблона: блогер узнаёт о приёмке из `payout.sent` (план уведомлений). P-14 рекламодателем и администратором — `placement.disputed` обоим и администратору (как у системы). Код `placement.dispute_resolved` больше не «ждёт модуль» | 2, 9 |
| R-15 | Кабинет рекламодателя | `available_actions`: `accept` — из `removed`; `claim` — из `published` и из `removed` до `claim_deadline_at`, если претензии нет. В размещении — `claim` (причина, текст, время). `PlacementDisputeSummary` получает необязательные `resolution` и `resolved_at` (комментарий администратора сторонам не показывается) | 5, 8 |
| R-16 | Метрики | `maxim_disputes_opened_total{opened_by,reason}`, `maxim_disputes_resolved_total{resolution}`, `maxim_placements_accepted_total{transition}` (P-12/P-13/P-15) — через `placement/app.Observer` | 4, 5, 6, 9 |
| R-17 | Команды администратора | `maxim admin dispute list [--open|--all] [--overdue] [--limit]`, `show --placement`, `open --placement --reason --email`, `resolve --placement --resolution [--share] --comment --email`, `remove-post --placement --reason --email`. Изменяющие — WARN + `audit_log`; без роли администратора — отказ (`admin_required`) | 10 |

## Уточнения после сверки с кодом (aif-improve, 2026-10-24)

| # | Что найдено в коде | Решение | Задача |
|---|---|---|---|
| I-1 | `internal/bootstrap/unimplemented.go` целиком генерирует `apigen/cmd/genstub`; реализованные операции перекрывают заглушки в `ServeMux`, вручную из файла ничего не убирается | Задача 8 только перегенерирует файл (`go generate ./internal/apigen/...`) | 8 |
| I-2 | `Holds.Settle` вызывают `placement/app/devaccept.go`, `placementtest.Holds`, `bridge.LedgerHolds` и тест `bootstrap/billing_e2e_test.go` | Смена сигнатуры порта (R-6) правит все четыре места | 4 |
| I-3 | Порядок блокировок модуля (04 §5): каналы → заказ → размещения. Снижение рейтинга после блокировки заказа и размещения перевернёт порядок и может дать взаимную блокировку с оформлением заказа (`LockForPlacement`) | P-15 сначала блокирует строку канала (`Channels.LockForPlacement`), затем заказ и размещение; `Ratings.Lower` — в той же транзакции после проверок | 6 |
| I-4 | В `disputed` снятие по сроку не делает P-10 (`recordRemoved`): `placement.removed_at` и `final_views` остаются пустыми, хотя пост снят; кабинет и отчёт не видят времени снятия | `recordRemoved` в `disputed` и `refunded` сохраняет `removed_at` и `final_views` размещения без перехода (`UpdateIfStatus` с тем же состоянием); I-02 проверяется по `publication.removed_at` | 7 |
| I-5 | `moneyCmd.senior` допускает только `senior`; спорам по 07 §4 нужен и `moderator` | `bootstrap/admin_dispute.go`: помощник `admin(ctx, email)` — любая роль администратора; отказ — WARN `admin dispute command refused` и ошибка `admin_required` | 10 |
| I-6 | Метрики размещений — `placement/postgres.ControlMetrics` (реализация `app.Observer`) | Новые счётчики R-16 — методы `Observer` и `ControlMetrics`, `NopObserver` | 4, 9 |

## Допущения о внешних данных

Внешних образцов для этапа почти нет: деньги этапа не выходят за пределы журнала, а удаление поста использует уже реализованный вызов MAX. Всё ниже помечено в коде комментарием `// ПРЕДПОЛОЖЕНИЕ (Q-NN): …`.

1. **Q-12, Рокет Ворк.** Возврата выплаты и аннулирования чека нет: спор невозможен после `accepted`, решение P-15 окончательное, выплата создаётся только на `payout_amount` после решения. Пропорциональная выплата уходит в Рокет Ворк обычной выплатой на меньшую сумму, чек — на неё же.
2. **Q-26, банк.** Возврат по спору зачисляется на баланс рекламодателя (`settlement` или `hold_release`), а не на расчётный счёт. Перевод в банк — существующий `balance-return` по обращению (D-26, S-33); платёжного API банка для возврата нет.
3. **Q-03/Q-04, MAX.** Удаление поста при `full_refund` и `admin_remove_post` — тот же `DELETE /messages`, что снятие по сроку; пост, которого уже нет (`GET /messages/{id}` — 404), при этих причинах считается удалённым, нарушение не записывается (спор уже решён или открыт).
4. **Q-18.** Денежных штрафов блогеру нет; санкция — рейтинг (R-13), величины штрафа — параметры конфигурации, не текст правил.

## Commit Plan
- **Commit 1** (после задач 1–3): `feat(placement): claims schema, dispute resolution rules`
- **Commit 2** (после задач 4–7): `feat(placement): acceptance, auto acceptance, claims and dispute resolution`
- **Commit 3** (после задач 8–10): `feat(placement): cabinet accept and claim api, admin dispute commands, wiring`
- **Commit 4** (после задач 11–12): `test(placement): acceptance and disputes e2e, docs sync`

## Tasks

### Phase 1: Данные и правила

- [x] **Task 1: Миграция претензий и решения спора (R-1, R-8)**
  - `migrations/00013_disputes.sql`:
    - `claim` (`id`, `placement_id uuid unique not null references placement`, `reason check in (removed_early, top_violated, content_mismatch, other)`, `text check (char_length between 1 and 2000)`, `created_by uuid not null references "user"`, `created_at`).
    - `dispute.claim_id` — внешний ключ на `claim(id)` (`NOT VALID` + `VALIDATE`).
    - `dispute_resolution_share`: `full_payout` ⇒ 10000, `full_refund` ⇒ 0, `proportional` ⇒ 1…9999; `resolution` ⇒ `resolved_by` задан.
    - Индекс открытых споров `(deadline_at) WHERE resolution IS NULL`.
  - Права роли: `REVOKE UPDATE, DELETE, TRUNCATE ON claim`; `REVOKE DELETE, TRUNCATE ON dispute`.
  - Журнал: не применимо (INFO `migrations applied` пишет `migrate`).
  - Тесты (`migrations` или `placement/postgres`): `up` → `down` → `up`; вторая претензия по размещению не вставляется; текст длиннее 2000 — отказ; `proportional` с долей 10000 — отказ; роль приложения не может `UPDATE claim`.
  - Файлы: `migrations/00013_disputes.sql`, тест миграции рядом с существующими.

- [x] **Task 2: Правила `placement/domain` (R-1 … R-3, R-8 … R-15)** (depends on 1)
  - `internal/placement/domain/claim.go`: `ClaimReason`, `Claim`, `NewClaim(id, placementID, userID, reason, text, now)` (проверки R-1), `ClaimWindowOpen(p, now)`.
  - `internal/placement/domain/dispute.go`: `Resolution` (`full_payout`, `proportional`, `full_refund`), `ResolutionShare(res, share *int, suggested *int) (int, error)` (R-8), `(*Dispute).Resolve(res, share, comment, adminID, now)`, `NewClaimDispute`, `NewAdminDispute`, поля `ClaimID`, `PerformanceShare`, `ResolutionComment`, `ResolvedBy`, `ResolvedAt`; `RatingPenalty(res, rules)` (R-13).
  - `internal/placement/domain/acceptance.go`: `(*Placement).AcceptWork(code P12|P13, actor, userID, now)` — отмена T-05; `(*Placement).Claim(userID, reason, now)` — P-14 advertiser, отмена T-05, T-06, уведомления; `(*Placement).OpenDisputeByAdmin(adminID, now, rules)`; `(*Placement).ResolveDispute(res, adminID, d Distribution, now)` — `ApplyTo(P-15, accepted|refunded)`, отмена T-06, `placement.dispute_resolved` обоим с данными R-14; `CheckSettleable(pub)` — I-02 (`ErrPublicationLive`).
  - `internal/placement/domain/order.go`: `AdvertiserActions(p, hasClaim, now)` (R-15).
  - `internal/placement/domain/timers.go`: `PlacementPayload.Reason` (`removal_reason`), `RemoveNowJob(id, reason, now)`; `publication.go` — `MarkRemovingFor(reason, now)`.
  - `ControlRules`: `RatingPenaltyRefund`, `RatingPenaltyProportional` (по умолчанию 10 и 5).
  - Ошибки: `ErrClaimExists`, `ErrClaimWindowClosed`, `ErrPublicationLive`, `ErrDisputeResolved`, `ErrShareRequired`, `ErrNoDispute`.
  - Журнал: не применимо (`domain` не пишет журнал).
  - Тесты (табличные): окно претензии на `claim_deadline_at − 1 с` / `= claim_deadline_at`; из `published` без срока; причины и длина текста (2000 и 2001 символ, пробелы); доли всех решений, предложение по умолчанию, предложение 10000 для `proportional` — `ErrShareRequired`; `ResolveDispute` в `accepted`/`refunded`, повтор — `ErrDisputeResolved`; I-02; действия кабинета; штраф рейтинга не ниже 0; таблица переходов по-прежнему сверяется с 03 §4.
  - Файлы: `internal/placement/domain/{claim.go,claim_test.go,dispute.go,dispute_test.go,acceptance.go,acceptance_test.go,order.go,order_test.go,timers.go,publication.go,errors.go,publish.go}`.

- [x] **Task 3: Хранилище (`placement/postgres`, рейтинг канала) (R-1, R-8, R-13)** (depends on 1, 2)
  - `internal/placement/postgres/queries/claims.sql`: `InsertClaim` (уникальность → `ErrClaimExists`), `GetClaimByPlacement`, `ListClaimsByPlacements`.
  - `queries/disputes.sql`: `InsertDispute` с `claim_id`; `GetDisputeByPlacementForUpdate`; `ResolveDispute` (`WHERE resolution IS NULL`, иначе `ErrDisputeResolved`); `ListDisputesAdmin` (открытые/все, просроченные, порядок `deadline_at`); `queries/violations.sql` — `ListViolationsByPlacement`.
  - Порты `domain.Claims`, расширенный `domain.Disputes`, `Violations.ListByPlacement`; `domain.ChannelRatings{Lower(ctx, channelID, points) (from, to int, error)}`.
  - `internal/channel/postgres/queries/*.sql` + `channel/app`: `LowerRating(ctx, channelID, points)` — `UPDATE … SET rating = greatest(rating - @points, 0) RETURNING` в транзакции вызывающего.
  - `sqlc generate` (sqlc 1.30.0) — модели всех модулей.
  - Журнал: DEBUG `placement store` при конфликте уникальности.
  - Тесты (`dbtest`): вторая претензия — `ErrClaimExists`; повторное решение — `ErrDisputeResolved`; рейтинг 3 − 5 → 0; список открытых споров по сроку.
  - Файлы: `internal/placement/postgres/{store.go,claims.go,store_test.go,queries/claims.sql,queries/disputes.sql,queries/violations.sql,sqlc/*}`, `internal/placement/domain/ports.go`, `internal/channel/{postgres,app}/…`, `sqlc.yaml`.
<!-- Commit checkpoint: tasks 1-3 -->

### Phase 2: Сценарии

- [x] **Task 4: Приёмка P-12, P-13 и общая функция распределения (R-4 … R-7, R-16)** (depends on 3)
  - `internal/placement/domain/ports.go`: `Holds.Settle(ctx, placementID, bloggerOrgID, d, by Settler)`; `placementtest.Holds` и `bridge.LedgerHolds` — актор в `ledgerdomain.ActorType` (R-6).
  - `internal/placement/app/accept.go`: `settleAccepted(ctx, op, o, p, change, d, src, by)` (R-5); `AcceptPlacement(ctx, orgID, userID, id, idemKey)` — P-12, `audit_log` `placement.accept_work`, повтор по ключу; `autoAccept(ctx, op, p, jobID)` — P-13.
  - `internal/placement/app/timers.go`: `TimerClaim` → `autoAccept`; убрать `errAutoAcceptancePending`; `transition.go` — `toJob` без бесконечных попыток T-05.
  - `internal/placement/app/devaccept.go`: через `settleAccepted`, комментарий «только development/test».
  - Журнал: INFO общий журнал переходов; INFO `placement accepted` (`transition`, `payout_amount`, `payout_id`); ERROR `placement invariant violated` (I-02); DEBUG `placement timer outdated`.
  - Тесты (`dbtest`, `placementtest`): P-12 — распределение 10000, `Settle` с актором `advertiser`, выплата создана, T-05 отменён; повтор с ключом — без второй выплаты; P-12 из `published` — `409`; P-13 по таймеру и устаревший таймер (`disputed`); `DevAccept` через ту же функцию.
  - Метрика `maxim_placements_accepted_total{transition}` — `Observer.Accepted(code)` и `placement/postgres.ControlMetrics` (I-6).
  - Файлы: `internal/placement/app/{accept.go,accept_test.go,timers.go,timers_test.go,devaccept.go,transition.go,service.go}`, `internal/placement/domain/ports.go`, `internal/placement/placementtest/placementtest.go`, `internal/placement/postgres/metrics.go`, `internal/bootstrap/bridge/ledger.go`, `internal/bootstrap/billing_e2e_test.go` (I-2).

- [x] **Task 5: Претензия рекламодателя P-14 (R-1 … R-4, R-15, R-16)** (depends on 4)
  - `internal/placement/app/claims.go`: `CreateClaim(ctx, orgID, userID, id, reason, text, idemKey)` — в транзакции: претензия, спор (`NewClaimDispute`, предложение R-3), P-14, `audit_log` `placement.claim`; после — оповещение `placement_disputed`, метрика.
  - `internal/placement/app/orders.go`, `blogger.go`: претензия в `PlacementView` (`Claims.ListByPlacements`), действия R-15.
  - Журнал: INFO `placement claim filed` (`claim_id`, `reason`), INFO `placement dispute opened` (`opened_by=advertiser`); WARN отказ (`claim_window_closed`, `claim_exists`). Текст претензии не пишется.
  - Тесты: претензия из `published` и из `removed`; после срока — `claim_window_closed`; вторая — `claim_exists`; при открытом споре системы — `invalid_transition`; предложение доли для `top_violated`; уведомления обоим и администратору; повтор с ключом.
  - Файлы: `internal/placement/app/{claims.go,claims_test.go,orders.go,blogger.go}`.

- [x] **Task 6: Решение администратора P-15, открытие спора и удаление поста (R-8 … R-14, R-16)** (depends on 4)
  - `internal/placement/app/disputes.go`:
    - `ResolveDispute(ctx, ResolveInput{PlacementID, Resolution, Share *int, Comment, AdminID})` — R-8: доля, `Dispute.Resolve`, распределение, `accepted` → `settleAccepted` с актором `admin`; `refunded` → `SetDistribution` (доля 0) и `ReleaseHold`; живая публикация при `full_refund` — снятие «сейчас» с причиной `refund` (R-10); рейтинг (R-13); `audit_log` `dispute.resolved`; после — `Alerts.Resolve` для `placement_disputed` и `dispute_overdue`.
    - `OpenDisputeByAdmin(ctx, placementID, adminID, reason)` — R-12.
    - `RemovePostByAdmin(ctx, placementID, adminID, reason)` — R-11.
    - `AdminDisputes(ctx, filter)`, `AdminDispute(ctx, placementID)` — спор, претензия, нарушения, история, публикация, предпросмотр распределения по предложенной доле.
  - Порядок блокировок (I-3): канал (`Channels.LockForPlacement`) → заказ → размещение → спор → публикация.
  - `Alerter` в `placement/app` получает `Resolve(ctx, reason, objectKey)`.
  - Журнал: WARN `dispute resolved` (`resolution`, `performance_share`, суммы, `admin_user_id`); INFO `channel rating lowered`; WARN `admin dispute opened`, `admin post removal requested`; WARN отказы (`publication_live`, `dispute_resolved`, `share_required`).
  - Тесты (`dbtest`, `placementtest`): `full_payout` (выплата на всю цену, рейтинг без изменений); `proportional` с предложением 7500 (пример 03 §7: выплата 7 500,00, комиссия 1 125,00, возврат 2 875,00); `proportional` без предложения и без `--share` — `share_required`; `full_refund` — `refunded`, удержание снято, рейтинг −10, снятие поста поставлено с причиной `refund`; `accepted` при живой публикации — `publication_live`; повторное решение — `dispute_resolved`; T-06 после решения не поднимает оповещение; `open` из `published` и `removed`; `remove-post` из `published` открывает спор.
  - Файлы: `internal/placement/app/{disputes.go,disputes_test.go,service.go}`.

- [x] **Task 7: Снятие поста при возврате и по решению администратора (R-10, R-11)** (depends on 2)
  - `internal/placement/app/remove.go`: `removable` — ещё `refunded` с живой публикацией; причина снятия из `PlacementPayload.Reason` (по умолчанию `schedule`); при `refund`/`admin` пропажа поста до удаления — снятие без нарушения (допущение 3); P-10 только из `published` (как сейчас); в `disputed` и `refunded` — `removed_at` и `final_views` размещения без перехода (I-4).
  - Журнал: INFO `placement removal started` (`removal_reason`); INFO `publication removed, placement state kept`.
  - Тесты (`maxfake`/`placementtest` как в `remove_test.go`): удаление в `refunded` — публикация `refund`, состояние не меняется; `admin` в `disputed` до `remove_at`; пост уже удалён — без нарушения.
  - Файлы: `internal/placement/app/{remove.go,remove_test.go}`, `internal/placement/domain/{timers.go,publication.go}`.
<!-- Commit checkpoint: tasks 4-7 -->

### Phase 3: Контракт, подключение, команды

- [ ] **Task 8: OpenAPI, кабинет и `mockapi` (R-2, R-4, R-15)** (depends on 5, 6)
  - `api/openapi.yaml`: `IdempotencyKey` в `acceptPlacement` и `createPlacementClaim`; описания `409` (`claim_window_closed`, `claim_exists`, `publication_live`); `available_actions` без «появятся с этапом приёмки»; `PlacementDisputeSummary.resolution`, `resolved_at`.
  - `go generate ./internal/apigen/...` и `./mockapi/internal/api/...` (`internal/apigen/api.gen.go`, `internal/bootstrap/unimplemented.go` — целиком генерируется `genstub`, I-1; `mockapi/internal/api/api.gen.go`).
  - `internal/placement/httpapi/orders.go`, `convert.go`: `AcceptPlacement`, `CreatePlacementClaim` (`201`), претензия и решение в ответах.
  - `mockapi`: ключ идемпотентности принимается; отказы с теми же `details.reason`; решение в сводке спора; `contract_test`, `gaps_test.go`.
  - Журнал: как в сценариях (handlers не пишут сами).
  - Тесты (`placement_http_test.go`): ответы по OpenAPI; чужое размещение — `404`; `422` на пустой текст; `409` с причинами.
  - Файлы: `api/openapi.yaml`, `internal/apigen/api.gen.go`, `internal/bootstrap/unimplemented.go`, `internal/placement/httpapi/{orders.go,convert.go}`, `internal/bootstrap/placement_http_test.go`, `mockapi/internal/{api/api.gen.go,store/*.go,handlers/*.go,server/*_test.go}`.

- [ ] **Task 9: Подключение, конфигурация, уведомления, метрики (R-6, R-13, R-14, R-16)** (depends on 6, 7)
  - `internal/bootstrap/config.go`: `DISPUTE_RATING_PENALTY_REFUND` (10), `DISPUTE_RATING_PENALTY_PROPORTIONAL` (5), границы 0…100; `placement.go` — `Claims`, `Ratings` (`bridge.PlacementChannelRatings`), `Alerts` с `Resolve`.
  - `internal/notification/domain/catalog.go`: `placement.dispute_resolved` без `AwaitsModule`; `catalog_callers_test.go` — код обычный; текст шаблона — исход и суммы.
  - Метрики R-16 (`placement/postgres/metrics.go` или наблюдатель в `bootstrap`).
  - `.env.example` и блок «Модули в разработке» → «Приёмка, споры и возвраты» в `deploy/app/env/{staging,production}/app.env.example`; `bash deploy/scripts/check-env.sh`.
  - Журнал: INFO конфигурации в общем выводе `config loaded`.
  - Тесты: проверки конфигурации; `wiring_test.go` — порты подключены; `TestCatalogCodesHaveCallers`.
  - Файлы: `internal/bootstrap/{config.go,config_test.go,placement.go,wiring_test.go}`, `internal/bootstrap/bridge/{placement.go,ledger.go}`, `internal/notification/domain/{catalog.go,catalog_callers_test.go}`, `templates/notification/placement.dispute_resolved.tmpl`, `.env.example`, `deploy/app/env/*/app.env.example`.

- [ ] **Task 10: Команды `maxim admin dispute …` (R-17)** (depends on 6, 9)
  - `internal/bootstrap/admin_dispute.go`: `list`, `show`, `open`, `resolve`, `remove-post` по образцу `admin_payout.go`; администратор `moderator` или `senior` по `--email` (помощник `admin`, I-5); таблица списка (срок, размещение, причина, кто открыл, предложение, решение); `show` — сводка спора, претензия (текст выводится, только для администратора), нарушения, публикация, история, предпросмотр сумм; `admin.go` — ветка `dispute`.
  - Журнал: WARN `admin dispute resolve`/`open`/`remove-post` + `audit_log` с актором `admin`.
  - Тесты (`admin_dispute_test.go`): не администратор — отказ; `resolve proportional` печатает суммы и сходится с журналом; `list --overdue`.
  - Файлы: `internal/bootstrap/{admin.go,admin_dispute.go,admin_dispute_test.go}`.
<!-- Commit checkpoint: tasks 8-10 -->

### Phase 4: Проверка и документы

- [ ] **Task 11: Сквозные тесты приёмки и споров** (depends on 10)
  - `internal/bootstrap/acceptance_e2e_test.go` на собранных `api` и `worker` (`clock.Manual`, `maxfake`, `ordfake`, `rocketworkfake`, настоящий журнал):
    - S-08 P-12: публикация, снятие, `acceptPlacement` → `accepted`, выплата `created`, проводка `settlement`, повтор с ключом — без изменений.
    - S-24 P-13: снятие, +24 ч — `accepted` и выплата.
    - S-08 претензия → `disputed`; `admin dispute resolve proportional --share 7500` → `accepted`, суммы 03 §7, возврат на баланс рекламодателя, выплата на `payout_amount`, `placement.dispute_resolved` обоим.
    - Претензия из `published` → `full_refund` → `refunded`, пост удалён (`maxfake`), удержание снято, рейтинг канала −10, заказ завершён.
    - Окно претензии истекло — `409 claim_window_closed`.
    - `check_ledger_balance` после каждого денежного шага — без оповещений.
  - Файлы: `internal/bootstrap/acceptance_e2e_test.go`, при необходимости `harness_test.go`.

- [ ] **Task 12: Документы** (depends on 11)
  - 02 S-08, S-18, S-19, S-24 — «Как сделано» (команды, причины отказов, ключи); 03 §4 (особые правила: P-12…P-15, I-02, удаление при возврате; убрать «до этапа приёмки»), §5 (T-05 без ожидания); 04 §5 (`claim`, проверка доли, `claim_id`), §9; 06 §2.3 (актор `settlement`); 07 §2, §8 (новые параметры); `docs/launch/legal-documents.md` — строка «Санкция за нарушение: снижение рейтинга»; `docs/open-questions.md` — R-13 (величина штрафа), допущения 1–4; README бэкенда — команды `maxim admin dispute …`; план — «Решения при реализации».
  - Журнал: не применимо.
  - Файлы: `docs/02-scenarios.md`, `docs/03-placement.md`, `docs/04-domain-model.md`, `docs/06-finance-legal.md`, `docs/07-non-functional.md`, `docs/launch/legal-documents.md`, `docs/open-questions.md`, `README.md` (бэкенд).
<!-- Commit checkpoint: tasks 11-12 -->

## Решения при реализации

- `settleAccepted` из R-5 реализован как `placement/app.settle`: общий для P-12, P-13, P-15 и `DevAccept`; для `refunded` он только записывает распределение и переход (удержание снимает `ReleaseHold`).
- Сначала проверяется допустимость перехода, затем I-02: ошибка пользователя (`accept` из `published`) — обычный `409 invalid_transition` без ERROR; `publication_live` и ERROR `placement invariant violated` — только при рассогласовании (`removed` с живой публикацией) и для P-15 с исходом `accepted`.
- `hold_release` при `full_refund` получает причину `full_refund` (код исхода, как `reason_code` истории P-15), а не `dispute_full_refund` из R-6.
- Претензия: `claim_exists` проверяется до правил перехода, чтобы повторная претензия без ключа получала понятную причину; при споре системы без претензии — `invalid_transition`.
- Снятие с причиной `refund`/`admin` подтверждается и в `disputed`/`refunded` (I-4); незавершённое намерение определяется по `placement.removed_at IS NULL`. Причина передаётся заданию через `PlacementPayload.Reason` → `app.JobRef.Reason`.
- Порт рейтинга: `bridge.PlacementChannelRatings` поверх `channel/app.Service.LowerRating`; штрафы по умолчанию задаёт конфигурация (`WithDefaults` их не подставляет, 0 — без санкции).

## Критерии готовности этапа

- `go build ./...`, `go vet ./...`, `go test ./...` (тесты на PostgreSQL) и `cd mockapi && go test ./...` проходят; `sqlc generate` и `go generate ./...` не дают изменений.
- P-12, P-13, P-15 с исходом `accepted` распределяют удержание, создают выплату и ставят `send_payout` в транзакции перехода (I-10, I-13); `DevAccept` — только служебная команда.
- I-02: в `accepted` нельзя попасть с живой публикацией; I-12 держится для `accepted`, `paid`, `refunded`.
- Возвраты проходят через журнал (`settlement` с возвратом или `hold_release`), `ledger.CheckBalance` сходится после каждой операции.
- Решение спора окончательное и пишется в `audit_log`; уведомление `placement.dispute_resolved` получают обе стороны.
- Сроки T-05 (24 ч) и T-06 (72 ч), доля 5000 для топа совпадают с `docs/launch/legal-documents.md`.
