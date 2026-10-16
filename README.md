# Maxim — бэкенд биржи рекламы в каналах MAX

> Рекламодатель выбирает каналы и платит, блогер публикует пост, платформа маркирует рекламу, контролирует размещение и рассчитывается.

В репозитории только бэкенд: Go, PostgreSQL, S3-совместимое хранилище. Один исполняемый файл с режимами `api`, `worker`, `migrate`. Кабинеты на React живут отдельно. Что строим и почему — [проектная документация](docs/README.md), этапы — [роадмап](.ai-factory/ROADMAP.md), устройство кода — [ARCHITECTURE.md](.ai-factory/ARCHITECTURE.md).

Готово:
- **Каркас и аккаунты.** Вход по коду из письма, сессии, организации и реквизиты, документ о режиме налогообложения и оферты (S-01, S-10).
- **Планировщик и интеграционный слой.** Задания с повторами, расписанием и правилами после простоя; учёт входящих событий и исходящих запросов; адаптер MAX с общим ограничителем; оповещения дежурным.
- **Подключение каналов.** Приём Webhook MAX с проверкой подлинности, подключение канала кодом из кабинета, проверка прав бота, реакция на их отзыв и удаление бота (S-12, S-25).
- **Каталог.** Цены по форматам с комиссией, интервалы недоступности, модерация каналов и проверка номера РКН администратором, суточные метрики и флаги аномалий, каталог рекламодателя с фильтрами, сортировками и курсором (S-02, S-13, S-16, S-32).
- **Медиатека и пост.** Загрузка изображений с проверкой по содержимому, превью и копии с плашкой «Реклама. название, ИНН», кампании, черновик поста в форматировании MAX с распознаванием ссылок, предпросмотр с проверками, версии поста для следующих этапов (S-04, S-05).

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
| <http://localhost:9001> | Консоль MinIO (`minio` / `minio-secret`). Корзины: `maxim-private` и `maxim-public` (анонимное чтение без перечисления) |
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
| `maxim admin grant --email <почта> --role moderator\|senior` | Роль администратора (до появления API администратора). Пользователя без учётной записи создаёт; повтор ничего не меняет |
| `maxim admin revoke --email <почта>` | Снять роль администратора |

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
| `schedule_channel_metrics` | 00:05 МСК | Ставит `refresh_channel_metrics` каналам в `moderation`, `active`, `suspended`, распределяя запуски по суткам |
| `refresh_channel_metrics` | В течение суток | Подписчики и сообщения из MAX, суточный снимок метрик, флаги аномалий; 10 000 подписчиков без подтверждённого номера РКН — `suspended` с причиной `rkn_required` |
| `build_media_derivative` | Сразу | Превью при загрузке, копия или копия с плашкой при сохранении черновика; не больше `MEDIA_BUILD_CONCURRENCY` построений одновременно |
| `publish_media_derivative` | Сразу | Копия производной из версии поста в публичной корзине и её постоянная ссылка |
| `purge_media` | 04:30 МСК | Удаляет из хранилища удалённые оригиналы и производные вне версий старше `MEDIA_RETENTION_DELETED` |

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

## Канал до каталога в разработке

Продолжение предыдущего раздела. Права бота поддельный адаптер не даёт, поэтому шаг 1 заменяет их запись в базу; остальное — через API.

1. Канал с правами и публичностью — на модерацию:

```bash
docker compose -f compose.dev.yml exec postgres psql -U maxim -d maxim -c \
  "UPDATE channel SET is_public = true, bot_is_admin = true, bot_permissions = '{write,delete}',
   status = 'moderation', status_reason = NULL WHERE status = 'pending_rights'"
```

2. Блогер: категория — `PATCH /api/v1/blogger/channels/{id}` с `category_id` из `GET /api/v1/channel-categories`; цены — `PUT /api/v1/blogger/channels/{id}/prices` (форматы — `GET /api/v1/placement-formats`); интервал недоступности — `POST .../unavailability` (необязательно).
3. Роль администратора, затем вход этой почтой:

```bash
docker compose -f compose.dev.yml run --rm --no-deps api admin grant --email admin@maxim.local --role moderator
```

4. Администратор: очередь — `GET /api/v1/admin/channels?status=moderation`, одобрение — `POST /api/v1/admin/channels/{id}/approve` (`409` с `details.missing` назовёт, чего не хватает).
5. Рекламодатель (организация с кабинетом `advertiser`): `GET /api/v1/catalog/channels`, карточка — `GET /api/v1/catalog/channels/{id}`. Цена в каталоге — с комиссией `PLATFORM_FEE_RATE_BP`.

Суточные метрики с поддельным адаптером не считаются (он не знает канал). Весь путь, включая метрики и правило РКН, проверяет `internal/bootstrap/catalog_e2e_test.go`.

## Пост с плашкой в разработке

Нужна организация с кабинетом `advertiser` (`POST /api/v1/organizations`, название и ИНН попадут на плашку). Всё — через Swagger UI или `curl` с cookie сессии и заголовком `X-CSRF-Token`.

1. Изображение: `POST /api/v1/media`, `multipart/form-data` с частью `file` (JPEG, PNG или WebP; сторона от 160 до 7680, до 50 МБ):

```bash
curl -b cookies.txt -H "X-CSRF-Token: $CSRF" -F "file=@photo.jpg" http://localhost:8080/api/v1/media
```

2. Кампания: `POST /api/v1/campaigns` с `{"name": "Осень"}` — вместе с ней создаётся пустой пост.
3. Пост: `PUT /api/v1/campaigns/{id}/post` с `content_doc` (документ редактора, пример — в Swagger UI) и `media_file_ids`. Адрес `https://…` в тексте станет ссылкой. По умолчанию пометка — на плашке, erid — под постом.
4. Предпросмотр: `POST /api/v1/campaigns/{id}/post/preview`. Пока `worker` строит изображения, в `problems` будет `media_processing` — повторите через пару секунд. Ссылки `media_urls` открываются в браузере: это изображения с плашкой из MinIO, подписанные для `localhost:9000` (`S3_PRESIGN_ENDPOINT`).

Создание версии поста, публикация производных в `maxim-public` и переходы версии — сервис для этапов заказа, модерации и маркировки; их API появится там. Весь путь проверяет `internal/bootstrap/post_e2e_test.go`.

## Конфигурация

Только переменные окружения. Все они с пояснениями и безопасными для разработки значениями перечислены в [.env.example](.env.example). При ошибке в конфигурации процесс пишет в журнал `config invalid` со списком полей и не стартует. Обязательные переменные зависят от режима: `migrate` нужна только база. `worker` в продакшене требует `MAX_BOT_TOKEN`, хотя бы одного адресата оповещений, `MAX_WEBHOOK_URL` (https) и оба секрета Webhook; `api` в продакшене — `MAX_WEBHOOK_PATH_SECRET` и `MAX_WEBHOOK_SECRET` (не короче 32 символов). Подключение каналов: `MAX_BOT_NAME` (имя бота в инструкции), `CHANNEL_LINK_CODE_TTL` (60 мин), `CHANNEL_LINK_CODES_PER_HOUR` (10), `CHANNEL_UNCLAIMED_TTL` (7 суток), `RKN_SUBSCRIBERS_THRESHOLD` (10 000). Каталог: `PLATFORM_FEE_RATE_BP` (1500 — комиссия 15 % сверху цены блогера, 0…10 000), `RKN_RECHECK_INTERVAL` (90 суток до повторной проверки номера), пороги флагов аномалий `METRICS_SUBSCRIBERS_SPIKE_BP` (2000) и `METRICS_VIEWS_MIN_BP` (100), пределы интервалов недоступности `CHANNEL_UNAVAILABILITY_MAX_DAYS` (90), `CHANNEL_UNAVAILABILITY_HORIZON_DAYS` (365), `CHANNEL_UNAVAILABILITY_MAX_COUNT` (50), хранение закончившихся интервалов `RETENTION_CHANNEL_UNAVAILABILITY` (30 суток). Медиатека и пост: хранилище нужно `api` и `worker`; публичная корзина `S3_BUCKET_PUBLIC` и её адрес `S3_PUBLIC_BASE_URL` (https в продакшене), адрес в подписанных ссылках `S3_PRESIGN_ENDPOINT` (если браузер видит хранилище иначе, чем сервер), `MEDIA_MAX_BYTES` (50 МБ, не больше), `MEDIA_MIN_SIDE` (160), `MEDIA_PREVIEW_SIDE` (640), `MEDIA_UPLOADS_PER_HOUR` (60), `MEDIA_UPLOAD_CONCURRENCY` (4), `MEDIA_BUILD_CONCURRENCY` (2), `MEDIA_ALBUM_LIMIT` (10, не больше), `MEDIA_URL_TTL` (1 ч), `MEDIA_RETENTION_DELETED` (30 суток), `POST_BODY_LIMIT` (3800) и `POST_FINAL_TEXT_LIMIT` (4000). `maxim admin` нужна только база.

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
| `internal/channel` | Модуль каналов: подключение кодом, права бота, приём Webhook MAX, кабинет блогера, каталог, модерация, метрики |
| `internal/post` | Модуль поста: медиатека и производные (`imaging`), кампании, черновик и сериализация в MAX, предпросмотр, версии |
| `internal/integration` | Адаптеры внешних систем и их подделки: почта, MAX |
| `migrations`, `templates` | Миграции goose и шаблоны писем |
| `docs`, `api` | Проектная документация и договор HTTP API |
| `mockapi`, `deploy` | Тестовый сервер API с выдуманными данными и его стенд |
