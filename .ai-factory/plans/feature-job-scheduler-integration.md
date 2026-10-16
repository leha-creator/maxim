# План реализации: Планировщик заданий и интеграционный слой

Branch: feature/job-scheduler-integration
Created: 2026-10-16

## Settings
- Testing: yes. `domain` и чистые функции (расписания, задержки, предохранитель, токены) тестируются без базы. Очередь, ограничитель, `inbound`, `outbound`, оповещения проверяются на настоящей PostgreSQL через `testcontainers-go` с `clock.Manual`. Адаптеры проверяются через `httptest`-сервер с записанными ответами (07 §9).
- Logging: verbose. `log/slog` JSON, поля и обязательные события по 07 §5. Для этапа это 5.3: задания, внешние вызовы, предохранитель, входящие события, простой.
- Docs: yes. В конце обязательная точка `/aif-docs`: README бэкенда (новые переменные окружения, виды заданий, оповещения, режим MAX в разработке).

## Roadmap Linkage
Milestone: "Планировщик заданий и интеграционный слой"
Rationale: план целиком реализует этот этап. Он даёт общий механизм, на котором стоят следующие этапы: задания с повторами, расписанием и правилами после простоя; учёт входящих и исходящих обменов; базовый адаптер MAX с ограничителем; поиск зависших объектов; оповещения дежурным.

## Research Context
Source: .ai-factory/RESEARCH.md (Active Summary), выдержка по этапу

- MAX: до 30 запросов в секунду на платформу, до 2 сообщений в секунду в один канал. Сервер использует сертификат Минцифры. В продакшене события приходят только через Webhook.
- Внешние системы: MAX, Яндекс ОРД, Рокет Ворк. Каждая закрыта адаптером с поддельной реализацией (05 §1, A-28).
- Общее состояние только в базе: `api` и `worker` работают в нескольких экземплярах (07 §1).

## Источники ТЗ

- Требования: 07 §1 (состав), §2 (планировщик целиком), §3 (сверка, зависшие объекты, оповещения), §5 (журнал, метрики), §8 (конфигурация), §9 (код и тесты).
- Интеграции: 05 §1 (общие правила), §2.1 (доступ MAX), §2.2 (приём события, шаги 2–4), §2.8 (ограничитель и приоритеты).
- Данные: 04 §8 (`job`, `inbound_event`, `outbound_request`).
- Архитектура: `.ai-factory/ARCHITECTURE.md`. Пакеты `platform/{job,inbound,outbound}`, `integration/max`, правила зависимостей.
- Существующий код: `internal/platform/job` (очередь, обработчик, повторы, возврат брошенных заданий, метрики), `internal/bootstrap/{worker.go,modules.go,config.go}`, `internal/integration/mail`.

## Границы этапа

Входит:
- Очередь заданий в полном объёме 07 §2. Уже есть: взятие, повторы, брошенные задания. Добавляется: управляемые повторы и отказ без повтора, отмена и перенос по `dedupe_key`, приоритет, периодические задания без «догоняющих» запусков, правила после простоя, оповещение при исчерпании попыток.
- Таблицы `inbound_event` и `outbound_request`, пакеты `platform/inbound` (запись, дедупликация, разбор по видам событий, задание `process_inbound_event`) и `platform/outbound` (таймауты, повторы, `429`, неизвестный исход, предохранитель, запись запросов без секретов).
- Общий ограничитель запросов MAX по 05 §2.8 с приоритетами, работающий на все экземпляры.
- Базовый адаптер `integration/max`: транспорт, авторизация, сертификат, разбор ошибок, `success=false`. Методы этапа: отправка сообщения в чат и пользователю, чтение сведений о боте. Поддельная реализация `maxfake`.
- Оповещения администратору: почта и чат дежурных в MAX, не чаще раза в час по одной причине. Поиск зависших объектов раз в час по проверкам, которые регистрируют модули. Задание `cleanup`.
- Метрики 07 §5.5 в объёме этапа: внешние вызовы, предохранитель, ограничитель, возраст необработанного входящего события.

Не входит (следующие этапы):
- Приём Webhook MAX и Рокет Ворк (`/webhooks/max/{secret}`, `/webhooks/rocketwork`), проверка подлинности, обработчики событий `bot_added` и других. Это этап «Подключение каналов» (MAX) и «Выплаты блогерам через Рокет Ворк». Операции по-прежнему отвечают `501`. Этап даёт им `inbound.Recorder`.
- Методы MAX для каналов, публикации, снятия и загрузки (`GET /chats/*`, `POST /messages?chat_id=` с вложениями, `DELETE /messages`, `POST /uploads`). Это этапы «Подключение каналов» и «Автопостинг и контроль».
- Адаптеры Яндекс ОРД и Рокет Ворк. Они строятся на `platform/outbound` по образцу `integration/max` в своих этапах.
- Проверки зависших размещений, публикаций, версий поста, объектов ОРД и выплат. Модули добавят их в реестр, когда появятся таблицы. Правило «`publish_placement` просрочен больше 30 мин → P-09» тоже реализует модуль размещений через политику просрочки из задачи 4.
- Уведомления пользователям (`notification`, `send_notification`): этап «Уведомления и отчёт по кампании».

## Решения этапа, требующие подтверждения

Документы этих мест не определяют. План реализует указанный вариант. После подтверждения задача 15 вносит его в документы; при отказе меняется только отмеченная задача.

| # | Вопрос | Решение в плане | Задача |
|---|---|---|---|
| R-1 | Где хранить состояние ограничителя MAX, если экземпляров несколько, а общее состояние только в базе (07 §1) | Таблица `rate_limit_bucket` (`key`, `tokens`, `refilled_at`). Корзина токенов обновляется одним `UPDATE … RETURNING`. Ключи: `max:global` (25/с) и `max:chat:{chat_id}` (1/с) | 1, 6 |
| R-2 | Как выполнить «задание низкого приоритета не задерживает публикацию больше 5 с» (05 §2.8) | Резерв токенов по приоритету: `publish` (публикация и снятие) берёт любой свободный токен; `reply` (ответы и уведомления) берёт, только если после этого остаётся ≥ 3 токенов; `reconcile` — ≥ 6; `metrics` — ≥ 10. Ожидание токена ограничено: для `publish` не ограничено, для остальных — 30 с, затем задание откладывается через `job.Retry` | 6 |
| R-3 | Где хранить счётчик предохранителя: «10 ошибок подряд по одной системе» (05 §1) | Таблица `outbound_circuit` (`system`, `consecutive_failures`, `open_until`). Её видят все экземпляры. Пока предохранитель открыт, вызов сразу возвращает `job.Retry(open_until − now)` без запроса | 1, 5 |
| R-4 | Как отправить оповещение «не чаще раза в час по той же причине» с нескольких экземпляров | Таблица `alert` (`reason`, `object_key`, `level`, `first_raised_at`, `last_raised_at`, `last_sent_at`, `raised_count`, `resolved_at`) с уникальной парой `(reason, object_key)`. Отправку выполняет новое задание `send_alert`. Повтор в течение часа только увеличивает счётчик | 1, 9 |
| R-5 | Что такое «чат дежурных» (07 §3, §6) | Чат в MAX, куда бот платформы пишет по `ALERT_MAX_CHAT_ID`. Почта — список `ALERT_EMAILS`. Если адресат не задан, канал пропускается с WARN при запуске | 9 |
| R-6 | Как узнать длительность простоя при запуске `worker` (07 §2) | Таблица `worker_heartbeat` (`instance`, `seen_at`). Каждый экземпляр отмечается раз в 30 с. Простой — разрыв между последней отметкой любого экземпляра и запуском больше `DOWNTIME_THRESHOLD` (по умолчанию 30 мин). Отметки старше суток удаляет `cleanup` | 1, 4 |
| R-7 | Официальный Go-клиент MAX (05 §2.1) не даёт записывать каждую попытку в `outbound_request` и управлять повторами | Свой тонкий HTTP-клиент по OpenAPI-схеме `max-messenger/api-schema` поверх `platform/outbound`. Официальный клиент не подключается. Если он позволяет передать свой `http.Client` и не повторяет запросы сам, исполнитель может взять его типы — с записью в «Технические решения» | 8 |
| R-8 | Как держать точность 10 с для публикаций, когда в очереди тысячи низкоприоритетных заданий (07 §2, 07 §7) | `HandlerOptions.Priority`. Сначала берутся задания высокого приоритета, затем остальные. 2 из 8 мест обработчика всегда держатся свободными для высокого приоритета | 2 |
| R-9 | Виды заданий без предела попыток («—» в 07 §2): `publish_placement`, `remove_placement`, опросы | Предел попыток задаётся большим (`job.Unlimited` = 1 000 000). Срок задаёт обработчик: `job.Retry(err, after)` — повтор через заданное время, `job.Permanent(err)` — сразу `failed` | 2 |
| R-10 | Сроки хранения в `cleanup` (07 §2 называет только `outbound_request` — 180 суток) | Завершённые (`succeeded`, `cancelled`) задания — 30 суток, `failed` — 180 суток. Коды входа — 7 суток после истечения, сессии — 7 суток после истечения. `inbound_event` в `processed` и `ignored` — 180 суток. Строки `alert` с `resolved_at` — 90 суток. Все значения задаются конфигурацией | 11 |
| R-11 | Новые виды заданий, которых нет в 07 §2 | `send_alert` (сразу, 5 попыток), `check_stuck_objects` (каждый час, 1 попытка, следующий запуск не зависит от исхода) | 9, 10 |

## Commit Plan
- **Commit 1** (после задач 1–4): `feat(job): priorities, periodic jobs and downtime rules`
- **Commit 2** (после задач 5–8): `feat(integration): outbound calls, inbound events, MAX client and rate limiter`
- **Commit 3** (после задач 9–12): `feat(platform): alerts, stuck objects check, cleanup and metrics`
- **Commit 4** (после задач 13–15): `chore(backend): worker wiring, scheduler e2e tests and docs sync`

## Tasks

### Phase 1: Очередь заданий

- [x] **Task 1: Миграция служебных таблиц (R-1, R-3, R-4, R-6)**
  - `migrations/00003_integration.sql` строго по 04 §1, §8:
    - `inbound_event`: `id uuid`, `source text check in ('max','rocketwork')`, `external_key`, `event_type`, `payload jsonb`, `status check in ('received','processed','ignored','failed')`, `processed_at`, `error`, `created_at`, `updated_at`. Уникальность `(source, external_key)`, индекс `(status, created_at)`.
    - `outbound_request`: поля 04 §8, `system check in ('max','yandex_ord','rocketwork')`, `created_at`. Индексы `(created_at)` и `(object_type, object_id)`.
    - `rate_limit_bucket` (R-1), `outbound_circuit` (R-3), `alert` (R-4) с уникальностью `(reason, object_key)`, `worker_heartbeat` (R-6).
    - Индекс `job (status, finished_at)` для очистки.
  - Права роли выдаются через `ALTER DEFAULT PRIVILEGES` из `00002`. Неизменяемых таблиц в этапе нет. Проверить, что `00002` действительно покрывает новые таблицы.
  - `sqlc.yaml`: записи для `platform/inbound`, `platform/outbound`, `platform/ratelimit`, `platform/alert`. Запросы `platform/job` дополняются в задачах 2–4.
  - Журнал: INFO `migrations applied` уже пишется командой `migrate`.
  - Тесты: миграции `up` → `down` → `up`. Роль приложения читает и пишет новые таблицы. Повторная вставка `inbound_event` с той же парой `(source, external_key)` не создаёт вторую строку.
  - Файлы: `migrations/00003_integration.sql`, `sqlc.yaml`.

- [x] **Task 2: Управляемые повторы, отмена, перенос и приоритет (R-8, R-9)** (depends on 1)
  - `internal/platform/job/errors.go`: `Retry(err, after)` — повтор через `after` вместо `RetryDelay`, попытка засчитывается. `Permanent(err)` — сразу `failed` без повторов. `Unlimited` — константа предела попыток (R-9). Проверка через `errors.As` в `Worker.execute`.
  - `Queue.Cancel(ctx, dedupeKey)` — живое задание (`pending`) → `cancelled`, `finished_at`. Задание в `running` не трогается, его обработчик идемпотентен (03 §11). `Queue.Reschedule(ctx, dedupeKey, runAt)` — новый `run_at` для `pending`. Обе работают в транзакции из `context` (I-10) и возвращают признак, найдено ли задание. Добавить оба в интерфейс `Enqueuer` или в отдельный интерфейс `Scheduler` того же пакета: `app` зависит только от интерфейсов.
  - `HandlerOptions.Priority` (`PriorityHigh`, `PriorityNormal`). `claimAndStart` сначала берёт задания высокоприоритетных видов, затем остальные. Последние 2 места пакета (`reservedHigh = 2`) остаются за высоким приоритетом (R-8). Запрос `ClaimJobs` не меняется: вызывается дважды с разными списками видов.
  - Журнал: INFO `job cancelled` (`dedupe_key`, `job_id`), DEBUG `job rescheduled` (`dedupe_key`, `run_at`), WARN `job failed, will retry` с полем `retry_reason` (`backoff` или `requested`), ERROR `job failed permanently` (`kind`, `error`).
  - Тесты (PostgreSQL, `clock.Manual`): `Retry` с `after` = 30 с ставит `run_at` = now + 30 с. `Permanent` даёт `failed` с первой попытки. `Cancel` в откатившейся транзакции не отменяет задание. После `Cancel` можно создать новое задание с тем же `dedupe_key`. `Reschedule` не трогает `running`. При 8 низких и 1 высоком готовых заданиях высокое берётся первым, а низкие не занимают последние 2 места.
  - Файлы: `internal/platform/job/{errors.go,job.go,worker.go,job_test.go}`, `internal/platform/job/queries/job.sql`.

- [x] **Task 3: Периодические задания** (depends on 2)
  - `internal/platform/job/schedule.go`: интерфейс `Schedule { Next(after time.Time) time.Time }` и реализации `Every(d)`, `DailyAt(hh, mm)`, `MonthlyAt(day, hh, mm)`. Календарные расписания считаются по московскому времени (A-12) и хранятся в UTC. `Spread(schedule, key)` — смещение внутри интервала по хешу ключа, чтобы суточные запуски распределялись по суткам (07 §2, `refresh_channel_metrics`).
  - `Worker.RegisterPeriodic(kind, schedule, h, opts)`: регистрирует обработчик с `max_attempts` = 1. При запуске `Run` создаёт задание `periodic:{kind}` на `schedule.Next(now)`, если живого нет (`dedupe_key` защищает от гонки экземпляров). После завершения — с успехом или ошибкой — следующий запуск ставится от фактического времени завершения (07 §2: «следующий запуск создаётся независимо от исхода»). Пропущенные интервалы не догоняются.
  - Журнал: DEBUG `periodic job scheduled` (`kind`, `run_at`), INFO `periodic job registered` (`kind`, `schedule`) при запуске.
  - Тесты: чистые тесты `Next` — граница суток и месяца по Москве, 31-е число в коротком месяце (следующий месяц, последний день), `Spread` стабилен для ключа и не выходит за интервал. На PostgreSQL: два воркера при запуске создают одно задание. После ошибки обработчика следующий запуск всё равно создан. После «простоя» в 5 интервалов выполняется один запуск.
  - Файлы: `internal/platform/job/{schedule.go,schedule_test.go,worker.go}`.

- [x] **Task 4: Правила после простоя (R-6)** (depends on 3)
  - `worker_heartbeat`: `Worker` отмечает себя при запуске и раз в 30 с. Запрос `LastHeartbeat` — максимум `seen_at` по всем экземплярам до своей отметки.
  - `HandlerOptions.Overdue` — политика просроченного задания, проверяется при взятии в работу:
    - `OverdueRunNow` (по умолчанию) — выполнить. Подходит для `remove_placement` и `placement_timer`.
    - Периодические — всегда один запуск (уже обеспечено задачей 3).
    - `OverdueExpire{After, OnExpired}` — если `now − run_at > After`, вызвать `OnExpired(ctx, Running)` вместо обработчика и завершить задание как `succeeded`. Модуль размещений позже подставит P-09 с причиной `publish_error` для `publish_placement` при `After` = 30 мин (T-03).
  - При запуске `Run`: если разрыв с последней отметкой больше `DOWNTIME_THRESHOLD` (по умолчанию 30 мин) — запись ERROR `worker started after downtime` (`downtime`, `overdue_jobs` по видам) и оповещение `downtime` (задача 9 подключает отправку; до неё — интерфейс `Alerter` с пустой реализацией). Затронутые объекты — список `object_ref` из полезной нагрузки просроченных заданий высокого приоритета (поле `HandlerOptions.ObjectRef func(payload) string`). Размещения появятся в этапе «Заказ и размещения».
  - Журнал: DEBUG `heartbeat`, ERROR `worker started after downtime`, WARN `overdue job expired` (`kind`, `job_id`, `overdue`).
  - Тесты (PostgreSQL, `clock.Manual`): отметки 40 мин назад → ERROR и вызов `Alerter` со списком объектов. Отметки 5 мин назад → без оповещения. Первая отметка в пустой базе — не простой. Задание с `OverdueExpire{30m}` просрочено на 31 мин → `OnExpired`, обработчик не вызван. Просрочено на 29 мин → обработчик.
  - Файлы: `internal/platform/job/{heartbeat.go,overdue.go,worker.go,*_test.go}`, `internal/platform/job/queries/job.sql`.
<!-- Commit checkpoint: tasks 1-4 -->

### Phase 2: Интеграционный слой

- [x] **Task 5: `platform/outbound` — внешние вызовы (R-3)** (depends on 2)
  - `outbound.Client` создаётся на систему (`max`, `yandex_ord`, `rocketwork`): `Do(ctx, Call) (Response, error)`. `Call`: операция, метод, URL, тело, заголовки, `IdempotencyKey`, `ObjectType`/`ObjectID`, `Create bool` (признак создания объекта), `CheckExisting func(ctx) (found bool, err error)` для неизвестного исхода, `Upload bool` (таймаут 120 с).
  - Правила 05 §1:
    - Таймаут соединения 5 с, запрос целиком 30 с, загрузка файла 120 с.
    - До 3 попыток внутри вызова с задержками 1, 2, 4 с. Повторяются сетевые ошибки, таймауты, `5xx`, `429`. При `429` задержка берётся из `Retry-After`, если она меньше оставшегося времени задания; иначе `job.Retry(after)`.
    - `4xx`, кроме `429`, — `job.Permanent` с ошибкой `outbound.RejectedError` (код, тело без персональных данных).
    - Таймаут при `Create` → перед повтором вызвать `CheckExisting`. Нашли — вернуть успех с признаком `Recovered`.
    - Ключ идемпотентности передаётся всегда, если задан.
    - `ValidateBody func(Response) error` — адаптер превращает `200` с отказом в теле в ошибку (MAX `success=false`).
  - Запись каждой попытки в `outbound_request` вне транзакции вызывающего: запрос без заголовков авторизации и полей из списка `Redact` (по умолчанию `token`, `access_token`, `authorization`, `secret`, `password`, `code`), код, тело ответа (обрезка до 64 КБ), длительность, ошибка, объект.
  - Предохранитель (R-3): успех обнуляет `consecutive_failures`. Ошибка, которая считается сбоем системы (сеть, таймаут, `5xx`), увеличивает счётчик. На 10-й ошибке `open_until` = now + 60 с. Пока предохранитель открыт, вызов возвращает `job.Retry(open_until − now)` без запроса. Срабатывание — ERROR и оповещение `circuit_open:{system}`.
  - Журнал (05 §1): DEBUG `outbound call started` и `outbound call finished` (`system`, `operation`, `attempt`, `duration_ms`, `status`). WARN `outbound call failed, will retry` (то же и `error`). ERROR `outbound call rejected` и `outbound retries exhausted` (то же и тело без персональных данных). ERROR `circuit breaker opened` (`system`, `failures`). Сообщение INFO «операция выполнена» пишет адаптер, а не `outbound`.
  - Тесты (`httptest` + PostgreSQL, `clock.Manual`, задержки внедряются как функция `sleep`): `503`, `503`, `200` → успех, 3 строки `outbound_request`. `400` → `Permanent`, одна попытка. `429` с `Retry-After: 2` → повтор через 2 с. `429` с `Retry-After: 600` → `job.Retry(600 s)`. Таймаут при `Create` + `CheckExisting` = найдено → успех без второго запроса. 10 ошибок `5xx` подряд → предохранитель открыт, 11-й вызов без запроса. Через 60 с вызов проходит. В `outbound_request` нет заголовка `Authorization` и поля `token`.
  - Файлы: `internal/platform/outbound/{client.go,circuit.go,redact.go,*_test.go}`, `internal/platform/outbound/queries/*.sql`.

- [x] **Task 6: Ограничитель запросов MAX (R-1, R-2)** (depends on 1)
  - `internal/platform/ratelimit`: `Limiter.Wait(ctx, key, rate, burst, priority) error`. Корзина токенов в `rate_limit_bucket`: одно выражение `INSERT … ON CONFLICT DO UPDATE` пересчитывает токены по `refilled_at` и списывает токен, если хватает с учётом резерва приоритета (R-2). Иначе возвращает время до следующего токена, и `Wait` спит через `clock` (`clock.Manual` в тестах).
  - Приоритеты по 05 §2.8: `PriorityPublish`, `PriorityReply`, `PriorityReconcile`, `PriorityMetrics`. Для приоритетов ниже `Publish` ожидание ограничено 30 с, затем `ErrBusy` → вызывающий возвращает `job.Retry`.
  - Параметры из конфигурации: `MAX_RATE_GLOBAL` (25), `MAX_RATE_PER_CHAT` (1) — 07 §8, «лимиты ограничителей».
  - Журнал: DEBUG `rate limit wait` (`key`, `priority`, `wait_ms`), WARN `rate limit busy` (`key`, `priority`).
  - Тесты (PostgreSQL, `clock.Manual`): 25 запросов за секунду проходят, 26-й ждёт. Для `max:chat:{id}` второй запрос в ту же секунду ждёт. Низкий приоритет не берёт последние 10 токенов, `Publish` берёт. Два «экземпляра» (две горутины с разными `Limiter`) вместе не превышают предел. Низкий приоритет после 30 с ожидания получает `ErrBusy`.
  - Файлы: `internal/platform/ratelimit/{limiter.go,*_test.go}`, `internal/platform/ratelimit/queries/*.sql`.

- [x] **Task 7: `platform/inbound` — входящие события** (depends on 2)
  - `Recorder.Record(ctx, Event{Source, ExternalKey, EventType, Payload}) (duplicate bool, err error)` — в одной транзакции вставка `inbound_event` (`ON CONFLICT (source, external_key) DO NOTHING`) и, если строка новая, задание `process_inbound_event` (`max_attempts` 10, `dedupe_key` = `inbound:{id}`). Ключ события по 05 §2.2: если внешней системой ключ не передан, `KeyFromBody(body)` возвращает SHA-256 от тела. Сквозной идентификатор запроса переходит в задание (уже делает `job.Enqueue`).
  - `Dispatcher`: модули регистрируют обработчики `Register(source, eventType, Handler)` и `Ignore(source, eventTypes...)`. Обработчик задания читает событие. Если статус уже не `received` — DEBUG и выход (идемпотентность). Иначе вызывает обработчик; итог — `processed`, `ignored` (вид без обработчика или явно игнорируемый) или ошибка (повтор задания). После исчерпания попыток — `failed` с текстом ошибки через `HandlerOptions.OnExhausted` (добавить в `job` этой задачей).
  - Журнал: INFO `inbound event accepted` (`source`, `event_type`, `inbound_event_id`), DEBUG `inbound event duplicate` (`source`, `external_key`), INFO `inbound event processed` / `inbound event ignored`, ERROR `inbound event failed`. Тело события не пишется.
  - Тесты (PostgreSQL): повторная запись того же события — одна строка и одно задание (07 §9: идемпотентность каждого входящего события). Повторный запуск задания по обработанному событию не вызывает обработчик. Неизвестный вид → `ignored`. Ошибка обработчика 10 раз → `failed`.
  - Файлы: `internal/platform/inbound/{recorder.go,dispatcher.go,*_test.go}`, `internal/platform/inbound/queries/*.sql`, `internal/platform/job/worker.go` (`OnExhausted`).

- [x] **Task 8: Базовый адаптер MAX и `maxfake` (R-7)** (depends on 5, 6)
  - `internal/integration/max`: `Client` поверх `outbound.Client` (система `max`) и `ratelimit.Limiter`. Адрес `MAX_API_URL` (по умолчанию `https://platform-api2.max.ru`), токен `MAX_BOT_TOKEN` в заголовке `Authorization` (05 §2.1). Корневой сертификат Минцифры: `MAX_CA_CERT_FILE` (PEM) добавляется к системным в `tls.Config`. В образ сертификат кладёт задача 13.
  - Общие части для следующих этапов: построение запросов, разбор ошибок MAX в `outbound.RejectedError`, проверка `success` в теле (`ValidateBody`), ожидание токенов `max:global` и `max:chat:{chat_id}` перед каждым запросом с приоритетом из `context` (`max.WithPriority`).
  - Методы этапа: `GetMe(ctx)` (проверка токена при запуске `worker` в `production`), `SendChatMessage(ctx, chatID, text)` и `SendUserMessage(ctx, userID, text)` — текст до 4000 символов, `format` `markdown`. Оба метода нужны оповещениям в чат дежурных (R-5) и уведомлениям блогеру в следующих этапах.
  - `maxfake`: те же методы в памяти, запоминает сообщения, `FailWith(err)` как в `mailfake`. Используется в `APP_ENV=test` и в разработке без `MAX_BOT_TOKEN`.
  - Журнал: INFO `max message sent` (`operation`, `chat_id` или `user_id`, `message_id`). Текст сообщения не пишется (07 §5.4).
  - Тесты (`httptest` с записанными ответами в `testdata/`): успешная отправка, `success=false` → ошибка, `401` → `Permanent`, `429` → повтор, заголовок `Authorization` передан и не попал в `outbound_request`, текст длиннее 4000 символов → ошибка без запроса. Тест сертификата: клиент с `MAX_CA_CERT_FILE` доверяет `httptest.NewTLSServer`, без него — нет.
  - Файлы: `internal/integration/max/{client.go,messages.go,errors.go,*_test.go,testdata/*}`, `internal/integration/max/maxfake/fake.go`.
<!-- Commit checkpoint: tasks 5-8 -->

### Phase 3: Контроль и оповещения

- [x] **Task 9: Оповещения администратору (R-4, R-5, R-11)** (depends on 2, 8)
  - `internal/platform/alert`: `Alerter.Raise(ctx, Alert{Reason, ObjectKey, Level, Summary, Fields})` и `Resolve(ctx, reason, objectKey)`. `Raise` пишет или обновляет строку `alert` (`raised_count + 1`, `last_raised_at`). Если `last_sent_at` пуст или старше часа (07 §3), ставит задание `send_alert` (5 попыток, `dedupe_key` = `alert:{reason}:{object_key}`) и отмечает `last_sent_at`. `Summary` и `Fields` — без персональных данных (07 §5.4), проверяется тестом.
  - Обработчик `send_alert`: письмо на каждый адрес `ALERT_EMAILS` (шаблон `templates/email/alert.tmpl`, тема `[ERROR] <reason>`) и сообщение в `ALERT_MAX_CHAT_ID` через `max.SendChatMessage` с приоритетом `PriorityReply`. Сбой одного канала не отменяет другой: ошибка возвращается после попытки обоих, повтор — только по неотправленным (признаки в полезной нагрузке).
  - Почтовый адаптер: метод `SendAlert(ctx, to, subject, text)` в `integration/mail` и `mailfake`.
  - Подключить источники: исчерпание попыток любого задания (`Worker`, ERROR `job exhausted attempts` → `job_failed:{kind}`), предохранитель (`circuit_open:{system}`), простой (`downtime`), отказ `outbound` после повторов (`outbound_failed:{system}:{operation}`).
  - Журнал: WARN или ERROR `alert raised` (`reason`, `object_key`, `raised_count`), DEBUG `alert suppressed` (повтор в течение часа), INFO `alert sent` (`reason`, `channels`), WARN `alert channel not configured` при запуске.
  - Тесты (PostgreSQL, `clock.Manual`, `mailfake`, `maxfake`): два `Raise` за 10 мин → одно задание `send_alert`, `raised_count` = 2. Через 61 мин → второе задание. Почта упала, MAX прошёл → повтор отправляет только почту. Без `ALERT_MAX_CHAT_ID` отправка идёт только почтой. Исчерпание попыток тестового задания создаёт оповещение `job_failed:<kind>`.
  - Файлы: `internal/platform/alert/{alert.go,send.go,*_test.go}`, `internal/platform/alert/queries/*.sql`, `internal/integration/mail/{mail.go,mailfake/fake.go}`, `templates/email/alert.tmpl`, `internal/platform/job/worker.go`.

- [x] **Task 10: Поиск зависших объектов (R-11)** (depends on 3, 9)
  - `internal/platform/stuck`: реестр проверок `Register(Check{Name, Threshold, Level, Find func(ctx, olderThan time.Time) ([]string, error)})`. Периодическое задание `check_stuck_objects` (каждый час, 07 §3): для каждой проверки — `Find`, на каждый найденный объект `alert.Raise(reason = "stuck:"+Name, objectKey)`. Объекты, которые больше не находятся, получают `Resolve`. Ошибка одной проверки не останавливает остальные.
  - Проверки этапа (таблица 07 §3): «Входящее событие в `received`» — 10 мин; «Задание в `failed`» — сразу (выборка `failed` без строки `resolved_at` в `alert`). Остальные строки таблицы регистрируют модули в своих этапах. Перечень с порогами и этапами записать комментарием рядом с реестром.
  - Пороги — конфигурация: `STUCK_INBOUND_EVENT_AGE` (10m).
  - Журнал: INFO `stuck objects check finished` (`check`, `found`), ERROR `stuck objects check failed` (`check`, `error`).
  - Тесты (PostgreSQL, `clock.Manual`): событие в `received` 11 мин → оповещение. 9 мин → нет. После обработки события следующий запуск закрывает оповещение. Падающая проверка не мешает второй.
  - Файлы: `internal/platform/stuck/{stuck.go,checks.go,*_test.go}`, `internal/platform/stuck/queries/*.sql`.

- [x] **Task 11: Задание `cleanup` (R-10)** (depends on 3)
  - Периодическое задание `cleanup` (раз в сутки, `DailyAt(04, 00)` по Москве, 1 попытка). Удаление пачками по 1 000 строк, чтобы не держать долгие блокировки. Что удаляется: `outbound_request` старше 180 суток (04 §8); `job` в `succeeded`/`cancelled` старше 30 суток и `failed` старше 180 суток; `auth_code` и `session` через 7 суток после истечения; `inbound_event` в `processed`/`ignored` старше 180 суток; `alert` с `resolved_at` старше 90 суток; `worker_heartbeat` старше суток. Сроки — конфигурация `RETENTION_*`.
  - Запросы к таблицам `account` (коды, сессии) не пишутся в `platform`: модуль `account` регистрирует свою функцию очистки в реестре `cleanup` через `bootstrap` (правило зависимостей). Реестр — `platform/job` или отдельный `platform/cleanup`, на выбор исполнителя.
  - Журнал: INFO `cleanup finished` (`table`, `deleted`, `duration_ms`), ERROR `cleanup failed` (`table`, `error`).
  - Тесты (PostgreSQL, `clock.Manual`): по строке до и после каждого срока — удаляется только старая. `failed` 31 суток остаётся. Таблица с 2 500 старыми строками очищается за один запуск.
  - Файлы: `internal/platform/cleanup/*.go` (или `internal/platform/job/cleanup.go`), `internal/account/postgres/queries/cleanup.sql`, `internal/account/app/cleanup.go`.

- [x] **Task 12: Метрики интеграционного слоя** (depends on 5, 6, 7)
  - По 07 §5.5: `outbound_requests_total{system,operation,outcome}` и `outbound_request_duration_seconds{system,operation}`. `outbound_circuit_open{system}` (0/1). `ratelimit_wait_seconds{key_kind,priority}`, где `key_kind` — `global` или `chat`, без идентификатора чата. `inbound_event_oldest_unprocessed_seconds{source}` — сборщик по запросу к базе, как `job.NewCollector`. Метрики очереди уже есть; добавить `job_overdue` с порогом 10 с для видов высокого приоритета.
  - Регистрация в реестре `worker`; сборщик `inbound` подключается и в `api`.
  - Журнал: не применимо; ошибки сборщика — WARN `metrics collect failed` (`collector`).
  - Тесты: после вызова через `outbound` счётчик вырос с нужными метками. Сборщик `inbound` отдаёт возраст старейшего события в `received` (`testutil.CollectAndCompare`).
  - Файлы: `internal/platform/outbound/metrics.go`, `internal/platform/ratelimit/metrics.go`, `internal/platform/inbound/metrics.go`, `internal/platform/job/metrics.go`.
<!-- Commit checkpoint: tasks 9-12 -->

### Phase 4: Сборка, проверка, документы

- [x] **Task 13: Конфигурация, сборка `worker`, окружение** (depends on 8, 9, 10, 11, 12)
  - `internal/bootstrap/config.go`: `MAX_API_URL`, `MAX_BOT_TOKEN`, `MAX_CA_CERT_FILE`, `MAX_RATE_GLOBAL` (25), `MAX_RATE_PER_CHAT` (1), `ALERT_EMAILS` (через запятую), `ALERT_MAX_CHAT_ID`, `DOWNTIME_THRESHOLD` (30m), `STUCK_INBOUND_EVENT_AGE` (10m), `RETENTION_*`. Проверка: в `production` для режима `worker` обязательны `MAX_BOT_TOKEN` и хотя бы один адресат оповещений. Скорости положительные. `MAX_CA_CERT_FILE` читается и разбирается при запуске. Секреты в журнале маскируются.
  - `internal/bootstrap/modules.go`, `worker.go`: `infra` получает `max` (`maxfake` в `APP_ENV=test` и в разработке без токена), `limiter`, `outbound`-клиент MAX, `alerter`. Регистрация `process_inbound_event`, `send_alert`, `check_stuck_objects`, `cleanup`. Сердцебиение и правила простоя включены. В `production` при запуске `worker` вызывается `GetMe`; неудача — ERROR и выход с кодом 1.
  - `Dockerfile`: корневой сертификат Минцифры (`deploy/certs/russian_trusted_root_ca.pem`, источник и отпечаток SHA-256 — в комментарии) копируется в образ, `MAX_CA_CERT_FILE` указывает на него.
  - `compose.dev.yml`, `.env.example`: новые переменные с безопасными значениями. `ALERT_EMAILS=oncall@maxim.local` уходит в Mailpit.
  - Журнал: INFO `worker started` с полями `kinds`, `periodic`, `max_mode` (`fake` или `live`). WARN `alert channel not configured`.
  - Тесты: проверка конфигурации (`production` без `MAX_BOT_TOKEN`, неверный PEM, нулевая скорость). `wiring_test.go`: все виды заданий этапа зарегистрированы, периодические — с расписанием.
  - Файлы: `internal/bootstrap/{config.go,config_test.go,modules.go,worker.go,wiring_test.go}`, `Dockerfile`, `deploy/certs/*.pem`, `compose.dev.yml`, `.env.example`.

- [x] **Task 14: Сквозные тесты планировщика** (depends on 13)
  - `internal/bootstrap/scheduler_e2e_test.go` на собранном `worker` (PostgreSQL, `clock.Manual`, `mailfake`, `maxfake`):
    - Простой: отметка 45 мин назад, просроченные тестовые задания `OverdueRunNow`, `OverdueExpire{30m}` и периодическое. После запуска первое выполнено, второе ушло в `OnExpired`, периодическое выполнено один раз. ERROR `worker started after downtime`, письмо и сообщение в чат дежурных.
    - Входящее событие: `Recorder.Record` дважды → одно событие, одно задание, обработчик вызван один раз. Событие без обработчика → `ignored`.
    - Отказ внешней системы: `maxfake` возвращает ошибку `5xx` 10 раз подряд → предохранитель открыт, оповещение `circuit_open:max` ушло один раз, хотя причина повторялась.
    - Зависшее событие: событие в `received` 11 мин → `check_stuck_objects` создаёт оповещение. Повтор через 30 мин новых писем не даёт.
  - Проверка журнала тем же сборщиком записей, что в `e2e_test.go`: обязательные события этапа (07 §5.3: задания, внешние вызовы, повтор, предохранитель, входящие события, простой) присутствуют. Токен бота, заголовок `Authorization` и адреса почты в открытом виде не встречаются.
  - Файлы: `internal/bootstrap/scheduler_e2e_test.go`.

- [x] **Task 15: Синхронизация документов с решениями этапа** (depends on 13)
  - После подтверждения R-1 … R-11 внести их в документы, чтобы правы были документы (README «Соглашения»):
    - 04 §8 — таблицы `rate_limit_bucket`, `outbound_circuit`, `alert`, `worker_heartbeat` (R-1, R-3, R-4, R-6) и сроки хранения (R-10).
    - 07 §2 — строки `send_alert`, `check_stuck_objects` (R-11); виды без предела попыток (R-9); приоритет и резерв мест (R-8); определение простоя (R-6). 07 §3 — каналы оповещения (R-5) и механизм «раз в час» (R-4). 07 §8 — новые параметры конфигурации.
    - 05 §2.1 — свой HTTP-клиент вместо официального (R-7); 05 §2.8 — резерв токенов по приоритету (R-2).
    - `.ai-factory/ARCHITECTURE.md` — пакеты `platform/{ratelimit,alert,stuck,cleanup}` в структуре папок и правилах зависимостей; строка про клиент MAX в «Технических решениях».
  - Журнал: не применимо.
  - Файлы: `docs/04-domain-model.md`, `docs/05-integrations.md`, `docs/07-non-functional.md`, `.ai-factory/ARCHITECTURE.md`.
<!-- Commit checkpoint: tasks 13-15 -->

## Критерии готовности этапа

- `go build ./...`, `go vet ./...`, `go test -race ./...` проходят локально и в CI; `sqlc generate` и `go generate ./...` не дают изменений.
- `docker compose -f compose.dev.yml up` поднимает `worker` с `maxfake`. Задания `cleanup` и `check_stuck_objects` созданы по расписанию. Оповещение о сбойном задании приходит в Mailpit.
- Каждое правило 07 §2 покрыто тестом: повторы и задержки, `dedupe_key`, отмена, брошенные задания, периодические без догоняющих запусков, просрочка после простоя, оповещение при исчерпании попыток.
- Каждое правило 05 §1 покрыто тестом `outbound`: таймауты, повторы, `429`, `4xx`, неизвестный исход, предохранитель, запись без секретов.
- Ограничитель MAX не превышает 25 запросов в секунду и 1 сообщение в секунду на чат при двух экземплярах. Низкий приоритет не берёт резерв.
- Повтор каждого входящего события и каждого задания этапа идемпотентен (07 §9).
- Решения R-1 … R-11 подтверждены и внесены в документы (задача 15).
