# Maxim — бэкенд биржи рекламы в каналах MAX

> Рекламодатель выбирает каналы и платит, блогер публикует пост, платформа маркирует рекламу, контролирует размещение и рассчитывается.

В репозитории только бэкенд: Go, PostgreSQL, S3-совместимое хранилище. Один исполняемый файл с режимами `api`, `worker`, `migrate`. Кабинеты на React живут отдельно. Что строим и почему — [проектная документация](docs/README.md), этапы — [роадмап](.ai-factory/ROADMAP.md), устройство кода — [ARCHITECTURE.md](.ai-factory/ARCHITECTURE.md).

Готово:
- **Каркас и аккаунты.** Вход по коду из письма, сессии, организации и реквизиты, документ о режиме налогообложения и оферты (S-01, S-10).
- **Планировщик и интеграционный слой.** Задания с повторами, расписанием и правилами после простоя; учёт входящих событий и исходящих запросов; адаптер MAX с общим ограничителем; оповещения дежурным.
- **Подключение каналов.** Приём Webhook MAX с проверкой подлинности, подключение канала кодом из кабинета, проверка прав бота, реакция на их отзыв и удаление бота (S-12, S-25).

Остальные операции [спецификации](api/openapi.yaml) отвечают `501 not_implemented`.

## Быстрый старт

Нужен Docker.

```bash
docker compose -f compose.dev.yml up --build
```

| Адрес | Что |
|---|---|
| <http://localhost:8080/docs> | Swagger UI API (не в продакшене) |
| <http://localhost:8025> | Mailpit: письма с кодами входа и оповещения дежурным (`oncall@maxim.local`) |
| <http://localhost:9001> | Консоль MinIO (`minio` / `minio-secret`) |
| <http://localhost:9090/metrics>, `:9091/metrics` | Метрики `api` и `worker` |

Если порт занят, его меняет переменная `DEV_*_PORT`, например `DEV_API_PORT=58080 DEV_MAILPIT_PORT=58025`. Полный список — в [compose.dev.yml](compose.dev.yml).

Вход: `POST /api/v1/auth/code` с почтой, код из письма в Mailpit, затем `POST /api/v1/auth/login`. Меняющие запросы повторяют cookie `csrf_token` в заголовке `X-CSRF-Token` (Swagger UI делает это сам).

## Команды

| Команда | Что делает |
|---|---|
| `maxim api` | HTTP API кабинетов (`HTTP_ADDR`) и метрики (`METRICS_ADDR`) |
| `maxim worker` | Обработчик заданий из таблицы `job` и метрики (`METRICS_ADDR`) |
| `maxim migrate up\|down\|status` | Миграции. `DATABASE_URL` — владелец схемы; права выдаются роли `DB_APP_ROLE` |
| `maxim legal publish --kind <kind> --file <pdf> [--published-at <RFC3339>]` | Новая версия оферты или правил (до появления API администратора) |
| `maxim legal list` | Действующие версии документов |

Виды документов: `advertiser_offer`, `blogger_offer`, `platform_rules`, `privacy_policy`. Публикация в локальном окружении:

```bash
docker compose -f compose.dev.yml run --rm --no-deps -v "$PWD/offer.pdf:/tmp/offer.pdf:ro" \
  api legal publish --kind advertiser_offer --file /tmp/offer.pdf
```

В Git Bash на Windows перед командой нужен `MSYS_NO_PATHCONV=1`, иначе путь `/tmp/...` будет переписан.

## Фоновые задания и оповещения

`worker` берёт задания из таблицы `job` (07 §2). При запуске он сравнивает свою отметку с отметками других экземпляров: после простоя дольше `DOWNTIME_THRESHOLD` пишет ERROR и оповещает дежурных. Служебные виды заданий:

| Вид | Когда | Что делает |
|---|---|---|
| `process_inbound_event` | Сразу | Разбор входящего события MAX или Рокет Ворк по виду; неизвестные — `ignored` |
| `send_alert` | Сразу | Письмо на `ALERT_EMAILS` и сообщение в чат дежурных `ALERT_MAX_CHAT_ID` |
| `check_stuck_objects` | Каждый час | Поиск зависших объектов (07 §3), оповещение один раз на объект |
| `cleanup` | 04:00 МСК | Удаление устаревших записей; сроки — `RETENTION_*` |
| `refresh_channel_info` | Сразу | Сведения о канале и права бота из MAX после `bot_added` и смены прав |
| `delete_max_message` | Сразу | Удаление сообщения с кодом подключения из канала |
| `leave_max_chat` | Сразу | Бот покидает чат, который не канал, или канал без кода |
| `reply_bot_dialog` | Через 5 с | Ответ инструкцией на сообщение боту в личном диалоге |
| `release_unclaimed_channels` | 05:00 МСК | Каналы без кода дольше `CHANNEL_UNCLAIMED_TTL` — `disconnected`, бот уходит |

Оповещения по одной причине уходят не чаще раза в час. Причины: задание исчерпало попытки, сработал предохранитель внешней системы, внешний вызов окончательно не удался, простой, зависший объект.

**MAX.** Без `MAX_BOT_TOKEN` в разработке и всегда в `APP_ENV=test` работает поддельный адаптер. С токеном запросы идут в MAX через общий ограничитель: 25 запросов в секунду на все экземпляры и 1 сообщение в секунду в чат. Каждая попытка пишется в `outbound_request` без токена. В продакшене `worker` при запуске проверяет бота (`GET /me`) и не стартует без связи. Затем подписывает бота на Webhook `MAX_WEBHOOK_URL` + `/` + `MAX_WEBHOOK_PATH_SECRET`, если такой подписки ещё нет; неудача — ERROR и оповещение, `worker` продолжает работу. Нужен корневой сертификат Минцифры: [deploy/certs/README.md](deploy/certs/README.md).

**Webhook MAX.** `POST /api/v1/webhooks/max/{MAX_WEBHOOK_PATH_SECRET}` с заголовком `X-Max-Bot-Api-Secret: {MAX_WEBHOOK_SECRET}`. Неверный секрет — `401`, повторные отказы — оповещение. Событие сохраняется в `inbound_event` и разбирается заданием; то же тело второй раз не обрабатывается. Секретная часть пути в журнал и метрики не попадает.

**Метрики `worker`.** `maxim_jobs*` (очередь, просроченные, задержка `maxim_jobs_lag_seconds`), `maxim_outbound_requests_total` и `maxim_outbound_request_duration_seconds`, `maxim_outbound_circuit_open`, `maxim_ratelimit_wait_seconds`, `maxim_inbound_event_oldest_unprocessed_seconds`. У `api` — ещё `maxim_max_webhook_requests_total{outcome}`.

## Подключение канала в разработке

Без `MAX_BOT_TOKEN` события MAX отправляются на приём вручную; образцы тел — в [internal/integration/max/testdata/updates](internal/integration/max/testdata/updates). Секреты — из [compose.dev.yml](compose.dev.yml).

1. Блогер входит, создаёт организацию с кабинетом `blogger` и получает код: `POST /api/v1/blogger/channels/link-code`.
2. Бот добавлен в канал:

```bash
curl -X POST "http://localhost:8080/api/v1/webhooks/max/dev-only-webhook-path-secret-0123456789" \
  -H "Content-Type: application/json" -H "X-Max-Bot-Api-Secret: dev-only-webhook-header-secret-012345678" \
  --data @internal/integration/max/testdata/updates/bot_added_channel.json
```

3. Код в канале: тот же запрос с телом `message_created_channel.json`, где `K7M2Q9XA` заменён на полученный код. Канал появляется в `GET /api/v1/blogger/channels`.

Ограничение: поддельный адаптер не знает сведений о канале и прав бота, а через API они не задаются. Поэтому в разработке `refresh_channel_info` завершается ошибкой (после попыток — оповещение в Mailpit), канал остаётся без названия и после кода получает `pending_rights` с `missing_permissions: [write, delete]`. Путь до `moderation` и реакция на отзыв прав проверяются тестами `internal/channel/app` и `internal/bootstrap/channel_e2e_test.go` или на тестовом боте с `MAX_BOT_TOKEN`. Повторная отправка того же тела — дубликат, его не разберут: меняйте `timestamp`.

## Конфигурация

Только переменные окружения. Все они с пояснениями и безопасными для разработки значениями перечислены в [.env.example](.env.example). При ошибке в конфигурации процесс пишет в журнал `config invalid` со списком полей и не стартует. Обязательные переменные зависят от режима: `migrate` нужна только база. `worker` в продакшене требует `MAX_BOT_TOKEN`, хотя бы одного адресата оповещений, `MAX_WEBHOOK_URL` (https) и оба секрета Webhook; `api` в продакшене — `MAX_WEBHOOK_PATH_SECRET` и `MAX_WEBHOOK_SECRET` (не короче 32 символов). Подключение каналов: `MAX_BOT_NAME` (имя бота в инструкции), `CHANNEL_LINK_CODE_TTL` (60 мин), `CHANNEL_LINK_CODES_PER_HOUR` (10), `CHANNEL_UNCLAIMED_TTL` (7 суток), `RKN_SUBSCRIBERS_THRESHOLD` (10 000).

## Разработка

| Задача | Команда |
|---|---|
| Тесты | `go test ./...` |
| Код API из спецификации | `go generate ./internal/apigen` |
| Код доступа к базе из SQL | `docker run --rm -v "$PWD:/src" -w /src sqlc/sqlc:1.30.0 generate` |

Тестам нужен Docker: PostgreSQL 17, MinIO и Mailpit запускаются через testcontainers. Без Docker тесты с ними пропускаются, в CI (`CI=true`) падают. Тест отправки письма через Mailpit пропускается, если локальное ПО (например, антивирус) перехватывает SMTP через проброс портов.

Порядок изменений: сначала [api/openapi.yaml](api/openapi.yaml), затем генерация, затем обработчик. Схема базы меняется только новой миграцией в [migrations/](migrations/); запросы пишутся в `internal/*/postgres/queries/*.sql` и `internal/platform/*/queries/*.sql`.

## CI

| Workflow | Что делает |
|---|---|
| [backend.yml](.github/workflows/backend.yml) | Генерация без изменений в git, `sqlc vet`, `go vet`, `go test -race` с контейнерами, сборка образа |
| [deploy.yml](.github/workflows/deploy.yml) | Заглушка API: проверки, образ, выкладка на [тестовый стенд](deploy/README.md) |

## Структура

| Путь | Что |
|---|---|
| `cmd/maxim` | Точка входа |
| `internal/bootstrap` | Конфигурация и сборка режимов |
| `internal/platform` | Общее ядро: журнал, база, очередь заданий, входящие и исходящие обмены, ограничитель, оповещения, HTTP, хранилище, аудит |
| `internal/account` | Модуль аккаунтов: `domain`, `app`, `postgres`, `httpapi`, `jobs` |
| `internal/channel` | Модуль каналов: подключение кодом, права бота, приём Webhook MAX, кабинет блогера |
| `internal/integration` | Адаптеры внешних систем и их подделки: почта, MAX |
| `migrations`, `templates` | Миграции goose и шаблоны писем |
| `docs`, `api` | Проектная документация и договор HTTP API |
| `mockapi`, `deploy` | Тестовый сервер API с выдуманными данными и его стенд |
