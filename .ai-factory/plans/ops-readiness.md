# План реализации: юридическая и эксплуатационная готовность, пилот

Branch: feature/ops-readiness
Created: 2026-10-24

## Цель

Подготовить в репозитории всё, что нужно для выпуска бэкенда в тестовое окружение и продакшен и для пилота на первых каналах:

1. Образ бэкенда и стек на сервере: PostgreSQL, резервное копирование, `api` и `worker`, сайт для edge-прокси.
2. Выпуск без простоя (07 §8): проверка конфигурации, миграции до новой версии, замена `api` и `worker` по одному, откат.
3. Резервные копии (07 §8): ежедневная полная копия базы с хранением 30 суток, непрерывный архив журнала изменений для восстановления на момент времени в пределах 7 суток, версионирование и копия объектного хранилища, скрипт проверки восстановления.
4. Окружения: шаблоны переменных для тестового окружения (тестовые контуры или подделки MAX, Яндекс ОРД, банка, почты; Рокет Ворк — после слияния выплат) и продакшена, с отведённым местом для переменных модулей, которые ещё в разработке.
5. Материалы для юридической части: чек-листы оферт и правил площадки, персональные данные, черновик уведомления РКН, план пилота. Сами действия (подписание, подача, запуск) выполняют люди; в документах они помечены «требует действий человека».

Ничего не выкатывается и не отправляется во внешние системы. Работоспособность проверяется локально: сборка образов, `docker compose config`, прогон выпуска и резервного копирования с восстановлением на локальных контейнерах, `shellcheck`, `actionlint`, `go build ./... && go vet ./... && go test ./...`.

## Settings
- Testing: yes — тесты Go на новые команды (`probe`, `config check`), готовность `worker` и паузу перед остановкой `api`; прогоны `deploy/local/rehearse-release.sh` и `deploy/local/rehearse-backup.sh` на локальном Docker; `shellcheck`, `actionlint`, `hadolint`, `docker compose config`, `caddy validate`
- Logging: verbose — Go-код пишет события по 07 §5 (INFO на старте и остановке, WARN на отказ проверки готовности, DEBUG на подробности); скрипты пишут JSON-строки `{"ts","level","component","event",...}` в stdout: каждый шаг, длительность, размер, итог; ошибки — `level=error` с командой и кодом выхода
- Docs: yes — обязательная проверка документации в конце; документы эксплуатации и запуска входят в задачи 11–12

## Roadmap Linkage
Milestone: "Юридическая и эксплуатационная готовность, пилот"
Rationale: план закрывает часть этапа, которую можно сделать в репозитории. Этап не отмечается выполненным: юридические шаги, уведомление РКН и пилот остаются за людьми.

## Research Context
Source: docs/07-non-functional.md §8, docs/06-finance-legal.md §5, §8, §9, §10, docs/04-domain-model.md §10, docs/open-questions.md (A-24, A-25, Q-02, Q-12, Q-16, Q-31, Q-33)

- Площадка: серверы, база, хранилище и резервные копии — в России (A-24).
- Окружения: разработка (подделки), тестовое (тестовый бот MAX, тестовый контур ОРД, тестовая среда Рокет Ворк, если есть — Q-12), продакшен. Данные продакшена в другие окружения не переносятся.
- Миграции выполняются до запуска новой версии и совместимы с предыдущей версией кода.
- Резервные копии: полная копия ежедневно, 30 суток; журнал изменений для восстановления на момент времени в пределах 7 суток; хранилище с версионированием; проверка восстановления раз в квартал; потеря данных не больше 5 минут, восстановление не больше 4 часов.
- Выпуск: экземпляры `api` и `worker` заменяются по одному; `worker` перед остановкой заканчивает задания или возвращает их в очередь (уже есть: `shutdownDeadline = 30s`, `ReleaseJobs`).
- Юридическое: четыре документа (`advertiser_offer`, `blogger_offer`, `platform_rules`, `privacy_policy`) публикуются `maxim legal publish`; до запуска — уведомление РКН, политика, согласия (Q-33); тексты пишет юрист (Q-31).
- Справочно прочитана оферта агентского договора Telega.in (ООО «Телекот», ред. от 01.07.2026; PDF в `D:/work/maxim/docs/`, в репозиторий не коммитится). Она используется только как пример состава разделов агентской оферты; текст не копируется.

## Что уже есть

| Что | Где | Что с ним делаем |
|---|---|---|
| Образ бэкенда (distroless, `api`/`worker`/`migrate`/`legal`/`admin`/`dev`) | `Dockerfile`, `Dockerfile.dockerignore` | Оставляем; в distroless нет оболочки и `curl`, поэтому проверку здоровья делает сам исполняемый файл (`maxim probe`, задача 1) |
| `GET /livez`, `GET /readyz` у `api` | `internal/bootstrap/server.go` | `worker` получает такие же на порту метрик (задача 2) |
| Плавная остановка `api` (15 с) и `worker` (30 с, возврат заданий в очередь) | `internal/bootstrap/api.go`, `internal/platform/job/worker.go` | Добавляем паузу перед остановкой `api`, чтобы прокси успел убрать экземпляр (задача 2) |
| Стенд заглушки и edge-Caddy | `deploy/compose.yml`, `deploy/maxim.caddy`, `deploy/bootstrap.sh`, `deploy/README.md` | Не меняем; стек бэкенда кладём рядом в `deploy/app/` по тому же образцу (общая сеть `edge`, файл сайта с `__DOMAIN__`) |
| CI бэкенда | `.github/workflows/backend.yml` | Не меняем; проверки эксплуатации — отдельный `ops.yml`, выпуск — `release.yml` (задача 10) |

## Принятые решения

| Вопрос | Решение | Почему |
|---|---|---|
| Где PostgreSQL | В контейнере на сервере приложения, свой образ `maxim-postgres` (`postgres:17-alpine` + клиент S3 `mc` + скрипты) | Управляемая база у хостера неизвестна (ПРЕДПОЛОЖЕНИЕ). Свой образ даёт одинаковые версии сервера и `pg_dump`/`pg_basebackup` и проверяется локально. Переход на управляемую базу описан в README: тогда отпадают задачи 4 и 8, остаётся `restore-check.sh` |
| Резервное копирование базы | Ежедневно `pg_dump -Fc` (логическая копия, 30 суток) и `pg_basebackup` (физическая, 8 суток); WAL — `archive_command` в отдельный том, контейнер `backup` каждые 30 с выгружает сегменты в S3 и удаляет выгруженные; `archive_timeout = 60s` | Только стандартные инструменты PostgreSQL. Потеря не больше `archive_timeout` + интервал выгрузки ≈ 1,5 мин (цель — 5 мин). Физическая копия + WAL дают восстановление на момент времени за 7 суток |
| Куда копии | Отдельная корзина `BACKUP_S3_BUCKET` у другого хостера или в другом регионе, отдельный ключ только на запись и чтение этой корзины | Копия не должна пропасть вместе с основным хранилищем. Ключ основного хранилища к корзине копий доступа не имеет |
| Объектное хранилище | `mc version enable` на обеих корзинах, правило удаления прежних версий через 30 суток; ежедневная копия корзин в `BACKUP_S3_BUCKET/objects/` (`mc mirror --overwrite`, без удаления) | 07 §8 требует версионирования; копия защищает от потери учётной записи хостера |
| Расписание копий | `crond` в контейнере `backup` (busybox), время в UTC (ПРЕДПОЛОЖЕНИЕ: 00:30 — физическая копия, 01:00 — логическая, 02:00 — объекты, 03:00 — очистка, воскресенье 04:00 — проверка восстановления) | Не зависит от настроек сервера; расписание лежит в репозитории |
| Проверка восстановления | `restore-check.sh`: скачивает последнюю логическую копию, поднимает временный кластер PostgreSQL внутри контейнера `backup` (`initdb` в `/tmp`, порт 5433), восстанавливает, проверяет версию миграций, наличие таблиц, сходимость кеша остатков журнала проводок, возраст копии и наличие файлов из `media_file` в копии объектов; печатает отчёт и длительность (оценка времени восстановления). `restore-pitr.sh`: физическая копия + WAL на заданный момент | Проверка без Docker внутри Docker и без переноса данных продакшена в другое окружение: временный кластер живёт и удаляется на том же сервере |
| Оповещение о сбое копии | Скрипты пишут `level=error` в журнал, контейнер `backup` становится `unhealthy`; письмо на `ALERT_EMAILS` через `curl` по SMTP с теми же `SMTP_*`, не чаще раза в час на вид сбоя | Без новой миграции и без связи с кодом приложения. Таблица `alert` не используется: миграции параллельных веток заняли бы тот же номер |
| Выпуск без простоя | Две ячейки `a` и `b`: сервисы `api-a`, `api-b`, `worker-a`, `worker-b` в `deploy/app/compose.yml` (профили `slot-a`, `slot-b`). `release.sh`: проверка конфигурации новым образом → миграции → запуск новой ячейки → ожидание готовности → плавная остановка старой. Старая ячейка остаётся остановленной для быстрого отката (`release.sh --rollback`) | Детерминированнее, чем `--scale` у compose. Два `worker` разных версий работают одновременно безопасно: задания берутся с `SKIP LOCKED`, а `worker` берёт только известные ему виды заданий |
| Прокси | Edge-Caddy сервера (как у стенда заглушки): `reverse_proxy` на обе ячейки, `lb_policy first`, активная проверка `/readyz` каждые 2 с, `lb_try_duration 10s` | Остановленная ячейка выпадает по проверке; повтор соединения переводит запрос на живую |
| Остановка `api` | Новая переменная `HTTP_SHUTDOWN_DRAIN` (по умолчанию `0s`, от 0 до 60 с; в шаблонах окружений `5s`): после SIGTERM `/readyz` отвечает `503`, через паузу — `Shutdown` | Прокси успевает убрать экземпляр до закрытия соединений; ни один запрос не обрывается |
| Проверка здоровья в distroless | Команда `maxim probe <url>`: GET с тайм-аутом 2 с, код 0 при `200`, без загрузки конфигурации | В образе нет оболочки и `curl`; `HEALTHCHECK` compose вызывает сам исполняемый файл |
| Проверка конфигурации до выпуска | Команда `maxim config check --for api\|worker\|migrate\|legal\|admin\|dev`: загружает и проверяет конфигурацию режима, ничего не подключает | Ошибка в переменных останавливает выпуск до миграций; в CI проверяет шаблон тестового окружения |
| Миграции | Правило «расширить → перенести → сузить»: удаление и переименование столбцов и таблиц, смена типа, `SET NOT NULL` — только отдельной миграцией в следующем выпуске после того, как код перестал их использовать; строка `-- release: contract` в такой миграции. Проверка `deploy/scripts/check-migrations.sh` в CI | 07 §8: миграции совместимы с предыдущей версией кода, иначе откат образа сломается |
| Окружения | `deploy/app/env/staging.env.example` (`APP_ENV=development`: без токена — подделка, с токеном — тестовый контур; почта — Mailpit из профиля `mailpit` или настоящий SMTP; банк — файл выписки из `maxim dev statement`) и `production.env.example` (`APP_ENV=production`) | `APP_ENV=test` всегда включает подделки и хранилище в памяти — оно для тестов Go. В `development` подделка включается отсутствием токена, а команды `maxim dev` для имитации оплаты доступны |
| Место для новых переменных | В `.env.example` и в шаблонах окружений — блок «Модули в разработке» в конце с подзаголовками «Выплаты», «Уведомления», «Споры», «API администратора»; сами переменные этих модулей здесь не заводятся. `check-env.sh` проверяет, что каждая переменная шаблона окружения известна `.env.example` (или из списка переменных стека) | Ветки модулей добавляют переменные в свой блок и не конфликтуют построчно с остальным файлом |
| Образ в реестре | `ghcr.io/leha-creator/maxim`, теги `sha-<7>`; `release.yml` запускается только вручную (`workflow_dispatch`) с выбором окружения | Ничего не выкатывается само; окружение `production` в GitHub получает обязательное подтверждение человеком |
| Юридические документы | `docs/launch/`: `README.md` (общий чек-лист запуска), `legal-documents.md`, `personal-data.md`, `rkn-notification.md`, `pilot-plan.md` | Отдельно от проектной документации 01–08; ссылки из `docs/README.md` и 06 §8–§9 |

**Проверено при уточнении плана (`/aif-improve`).** `worker` берёт только известные ему виды заданий (`kind = ANY(@kinds)` в `internal/platform/job/queries`), поэтому старый и новый `worker` безопасно работают одновременно. `api` и `worker` не сверяют версию схемы при запуске, так что старая версия кода работает на схеме после расширяющей миграции, а откат образа не требует `migrate down`. Публичных `GET` операций API без сессии нет — репетиция выпуска проверяет `/livez` и `401` от `/api/v1/me`. Сходимость журнала в проверке восстановления повторяет запросы `reports.sql`, а не придумывает свои. Решения скиллов, требовавших выбора, приняты по рекомендуемому варианту: полный режим, тесты — да, журнал — подробный, документация — да, привязка к этапу роадмапа; все уточнения применены.

## Допущения

Все значения ниже придуманы, в файлах помечены `ПРЕДПОЛОЖЕНИЕ` и подлежат замене людьми.

| № | Допущение | Где используется |
|---|---|---|
| П-1 | Хостинг — российский облачный провайдер (например, Selectel, Москва): тестовое окружение — ВМ 2 vCPU / 4 ГБ / 40 ГБ SSD, продакшен — 4 vCPU / 8 ГБ / 80 ГБ SSD, Ubuntu 24.04, подготовка `deploy/bootstrap.sh` | `deploy/app/README.md` |
| П-2 | Адреса: тестовое окружение `https://stage-api.maxim-ads.ru`, продакшен `https://api.maxim-ads.ru`; публичная корзина `https://cdn.maxim-ads.ru` | шаблоны окружений, README |
| П-3 | Объектное хранилище — S3 того же провайдера (`s3.ru-1.storage.selcloud.ru`), корзины `maxim-<env>-private`, `maxim-<env>-public`; корзина копий `maxim-backups` — в другом регионе (`ru-3`, Санкт-Петербург) под отдельным ключом | шаблоны, скрипты копий |
| П-4 | Почта — SMTP российского сервиса рассылок (`smtp.unisender.ru:465`, TLS); в тестовом окружении — Mailpit | шаблоны |
| П-5 | Реквизиты ИП платформы: «ИП Иванов Иван Иванович», ИНН `500100732259`, ОГРНИП `304500116000157`, адрес — Москва; банк — условный; все — заглушки с корректными контрольными суммами только для проверки конфигурации | шаблон продакшена, черновик уведомления РКН |
| П-6 | Пилот: 4 недели с 2026-11-16 по 2026-12-13; 5–10 каналов MAX от 1 000 до 10 000 подписчиков (ниже порога РКН), 3–5 рекламодателей, до 30 размещений; комиссия 15 % | `docs/launch/pilot-plan.md` |
| П-7 | Время задач резервного копирования (UTC): 00:30, 01:00, 02:00, 03:00, проверка — воскресенье 04:00 | `deploy/postgres/crontab` |
| П-8 | Уровень защищённости ПДн — УЗ-4 (ПП РФ № 1119: нет специальных и биометрических категорий, только сотрудники и клиенты не в штате, менее 100 000 субъектов) | `docs/launch/personal-data.md` |
| П-9 | Ответственный за обработку ПДн и дежурный — сам ИП; дежурство в рабочие часы 10:00–19:00 МСК, вне их — по оповещению в чат дежурных | `docs/launch/*` |

## Tasks

### Фаза 1. Бэкенд для эксплуатации

- [x] **Task 1: Команды `maxim probe` и `maxim config check`** (depends on —).
  - `internal/bootstrap/ops.go`: `runProbe(ctx, args, stdout, stderr) int` — один аргумент URL (`http://` или `https://`), GET с тайм-аутом 2 с, код 0 при статусе `200`, 1 при другом статусе или ошибке, 2 при неверных аргументах; конфигурация не загружается, журнал не пишется (вызов каждые 5 с засорил бы журнал), на ошибке — одна строка в stderr. `runConfigCheck` — `config check --for <mode>`: `LoadConfig(environ, mode)`; при успехе INFO `config valid` с `mode`, `env` и `version`, при ошибке ERROR `config invalid` со списком полей (как в `Run`), код 1.
  - `internal/bootstrap/run.go`: режимы `ModeProbe = "probe"` (обрабатывается до загрузки конфигурации) и `ModeConfig = "config"` (загружает конфигурацию целевого режима, а не своего); компонент журнала `cli`.
  - `cmd/maxim/main.go`: строки usage и комментарий пакета.
  - Тесты `internal/bootstrap/ops_test.go`: `probe` — 200, 503, недоступный адрес, неверные аргументы (через `httptest.Server`); `config check` — валидная конфигурация разработки для каждого режима, ошибка `APP_ENV=production` без секретов, неизвестный режим → код 2.
  - Логирование: `config check` — INFO `config valid` (`op=Bootstrap.ConfigCheck`, `mode`, `env`), ERROR `config invalid` с полями; `probe` — без журнала, причина отказа в stderr.
  - Файлы: `internal/bootstrap/ops.go`, `internal/bootstrap/ops_test.go`, `internal/bootstrap/run.go`, `cmd/maxim/main.go`.

- [x] **Task 2: Готовность `worker` и пауза перед остановкой `api`** (depends on 1).
  - `worker`: на порту метрик — `GET /livez` (всегда `200`) и `GET /readyz` (`200`, если обработчик заданий запущен и база отвечает за 1 с; `503` во время остановки). Флаг «запущен/останавливается» выставляет `runWorker`; если в `job.Worker` нет нужной точки, достаточно атомарного флага в `bootstrap`.
  - `api`: переменная `HTTP_SHUTDOWN_DRAIN` (`time.Duration`, по умолчанию `0s`, допустимо от 0 до 60 с, проверка в `Validate`). После отмены контекста `serve` переводит `/readyz` в `503` (`status: draining`), ждёт паузу и только потом вызывает `Shutdown`. Флаг передаётся в `Ready` из `runAPI`.
  - `.env.example`: `HTTP_SHUTDOWN_DRAIN=0s` с комментарием (в шаблонах окружений — `5s`); 07 §8, абзац «Выпуск»: пауза, `/readyz` у `worker`, команды `probe` и `config check`.
  - Тесты: `/readyz` у `worker` до запуска, во время работы и после отмены контекста; `serve` с паузой отвечает `503` на `/readyz` во время паузы и закрывается после неё; проверка границ `HTTP_SHUTDOWN_DRAIN` в `config_test.go`.
  - Логирование: INFO `drain started` (`op=Bootstrap.Serve`, `drain_ms`), INFO `shutdown started`/`shutdown finished` уже есть; WARN `readiness check failed` для `worker` — как у `api`; DEBUG `readiness probe` не нужен (частый вызов).
  - Файлы: `internal/bootstrap/api.go`, `internal/bootstrap/worker.go`, `internal/bootstrap/server.go`, `internal/bootstrap/config.go`, `internal/bootstrap/config_test.go`, `internal/bootstrap/server_test.go` (или новый `ops_test.go`), `.env.example`, `docs/07-non-functional.md`.

### Фаза 2. Конфигурация окружений

- [x] **Task 3: Шаблоны окружений и место для новых переменных** (depends on 2).
  - `deploy/app/env/staging.env.example` и `deploy/app/env/production.env.example`: группы как в `.env.example` (стек и образ, база, хранилище, почта, секреты, MAX, ОРД, деньги, оповещения, выпуск); заданы только переменные, обязательные для окружения или отличные от значений по умолчанию в коде. Переменные стека: `COMPOSE_PROJECT_NAME`, `MAXIM_IMAGE`, `DOMAIN`, `EDGE_ALIAS_PREFIX`, `POSTGRES_PASSWORD`, `POSTGRES_APP_PASSWORD`, `POSTGRES_BACKUP_PASSWORD`, `MIGRATE_DATABASE_URL`, `BACKUP_*`. Для каждой внешней системы в тестовом окружении — комментарий: «пусто — подделка, заполнено — тестовый контур» (MAX: тестовый бот; ОРД: `ORD_OAUTH_TOKEN` кабинета `ord-prestable`; банк: `maxim dev statement` + `maxim admin money statement import`; почта: Mailpit или SMTP). Секреты — `CHANGE_ME`, реальные значения только из хранилища секретов; придуманные значения с меткой `ПРЕДПОЛОЖЕНИЕ`.
  - В конце `.env.example` и обоих шаблонов — блок `# === Модули в разработке ===` с подзаголовками «Выплаты блогерам (Рокет Ворк): ROCKETWORK_*, PAYOUT_*», «Уведомления», «Приёмка, споры и возвраты», «API администратора и отчётность ОРД» и указанием: переменные добавляет ветка модуля, здесь их не заводить.
  - `deploy/scripts/check-env.sh`: каждая переменная из `deploy/app/env/*.env.example` есть в `.env.example` или в списке переменных стека; нет повторов ключей; нет пустых обязательных переменных стека; вывод — JSON-строки, код 1 при ошибке. Ключ `--print-missing` выводит переменные `.env.example`, которых нет в шаблоне (для ручной сверки после слияния модулей).
  - `.gitattributes`: `*.caddy`, `*.env.example`, `crontab`, `Dockerfile` — `text eol=lf` (репозиторий ведётся на Windows; с CRLF скрипты, `crontab` и env-файлы в контейнере ломаются).
  - Логирование: скрипт пишет `{"level":"info","event":"env_check","file":…,"keys":N}` по каждому файлу и `level=error` с ключом и файлом на каждое нарушение.
  - Файлы: `deploy/app/env/staging.env.example`, `deploy/app/env/production.env.example`, `.env.example`, `.gitattributes`, `deploy/scripts/check-env.sh`, `deploy/scripts/lib.sh` (общая функция журнала для скриптов).
  - Как сделано: на каждое окружение два файла — `deploy/app/env/<env>/app.env.example` (переменные бэкенда, `env_file` контейнеров `api`, `worker`, `migrate`, `backup`) и `stack.env.example` (подстановка в compose, `postgres`, `backup`: пароль суперпользователя, роли, корзина копий). Так пароль владельца схемы и ключ корзины копий не попадают в контейнеры приложения. `check-env.sh` проверяет и это: переменная стека в `app.env.example` — ошибка. В `.gitattributes` добавлены также `*.golden` и `**/testdata/*.txt`: в этом worktree при `core.autocrlf=true` эталоны тестов получили CRLF, и `go test` падал в `clientbank` и `post/domain`.

### Фаза 3. Стек и выпуск без простоя

- [x] **Task 4: Образ PostgreSQL с архивом WAL** (depends on —).
  - `deploy/postgres/Dockerfile`: `FROM postgres:17-alpine` (точный тег), `COPY --from` клиента `mc` из закреплённого образа (`pgsty/mc:<тег>`; бинарный файл статический), `apk add curl` (письма о сбоях), каталог `/var/lib/postgresql/wal-archive` с владельцем `postgres`, скрипты в `/usr/local/bin/`, `initdb/` в `/docker-entrypoint-initdb.d/`, `HEALTHCHECK` не задаётся (задаёт compose).
  - `deploy/postgres/initdb/10-roles.sh`: роль приложения (`POSTGRES_APP_ROLE`, по умолчанию `maxim_app`, пароль `POSTGRES_APP_PASSWORD`), роль копирования `maxim_backup` (`LOGIN REPLICATION`, `pg_read_all_data`, пароль `POSTGRES_BACKUP_PASSWORD`); строка `host replication maxim_backup all scram-sha-256` в `pg_hba.conf`. Пароли через переменные `psql -v`, не в тексте команд.
  - `deploy/postgres/postgresql.conf.d/` или аргументы `command` в compose: `wal_level=replica`, `archive_mode=on`, `archive_command='/usr/local/bin/wal-archive.sh %p %f'`, `archive_timeout=60`, `max_wal_senders=4`, `password_encryption=scram-sha-256`.
  - `deploy/postgres/bin/wal-archive.sh`: `test ! -f` → копия во временный файл → `fsync`/`mv` в архив; при существующем файле с тем же содержимым — успех, с другим — ошибка (правило документации PostgreSQL для `archive_command`).
  - Проверка: `docker build -f deploy/postgres/Dockerfile deploy/postgres`, `hadolint`, `shellcheck`.
  - Логирование: `wal-archive.sh` пишет в stderr PostgreSQL только ошибки (`level=error`, имя сегмента); успешная архивация не пишется — её видно по выгрузке в задаче 8.
  - Файлы: `deploy/postgres/Dockerfile`, `deploy/postgres/initdb/10-roles.sh`, `deploy/postgres/bin/wal-archive.sh`.
  - Как сделано: `postgres:17.10-alpine3.24`, `mc` из `pgsty/mc:RELEASE.2026-09-16T00-00-00Z`, `curl` и `jq`; контекст сборки — `deploy/` (нужен `deploy/scripts/lib.sh`), ограничен `deploy/postgres/Dockerfile.dockerignore`: `docker build -f deploy/postgres/Dockerfile deploy`. Параметры WAL — аргументами `command` в compose. Образ на сервер тянется из GHCR (`MAXIM_POSTGRES_IMAGE`), собирает `release.yml`.

- [x] **Task 5: Compose стека и сайт edge-прокси** (depends on 3, 4).
  - `deploy/app/compose.yml`: проект из `COMPOSE_PROJECT_NAME` (`maxim-staging`, `maxim-production`).
    - `postgres` — образ `maxim-postgres` (собирается на сервере из `deploy/postgres` или тянется из GHCR — выбрать при реализации и записать), тома `pgdata` и `wal-archive`, `command` с параметрами WAL, `healthcheck` `pg_isready`, `stop_grace_period: 60s`, сети только `backend`, `shm_size: 256m`.
    - `backup` — тот же образ, `entrypoint` `backup-agent.sh` (задача 8), тома `wal-archive` и `backup-tmp`, `healthcheck` по файлу состояния, `depends_on: postgres healthy`.
    - `migrate` — профиль `tools`, образ `${MAXIM_IMAGE}:${MAXIM_TAG_NEW}`, `command: ["migrate","up"]`, `DATABASE_URL=${MIGRATE_DATABASE_URL}`.
    - `api-a`, `api-b`, `worker-a`, `worker-b` — профили `slot-a`/`slot-b`, образы `${MAXIM_IMAGE}:${MAXIM_TAG_A}` / `${MAXIM_TAG_B}`, `env_file: ${APP_ENV_FILE:-app.env}`, `healthcheck: ["CMD","/maxim","probe","http://127.0.0.1:8080/readyz"]` (у `worker` — порт метрик `9090`), `interval: 5s`, `start_period: 60s`; `stop_grace_period`: `api` 30 s (пауза 5 s + `Shutdown` 15 s + запас), `worker` 45 s (`shutdownDeadline` 30 s + запас); `read_only: true`, `tmpfs: /tmp`, `no-new-privileges`, `mem_limit` (`api` 512m, `worker` 768m — ПРЕДПОЛОЖЕНИЕ по 07 §7); у `api` алиас в сети `edge` — `${EDGE_ALIAS_PREFIX}-api-a|b`. Порты наружу не публикуются.
    - Сервис `mailpit` — профиль `mailpit` для тестового окружения без SMTP.
    - Сети: `backend` (внутренняя), `edge` (внешняя, `name: edge`).
  - `deploy/app/maxim-app.caddy`: сайт `__DOMAIN__`, `reverse_proxy __PREFIX__-api-a:8080 __PREFIX__-api-b:8080` с `lb_policy first`, `lb_try_duration 10s`, `lb_try_interval 250ms`, `health_uri /readyz`, `health_interval 2s`, `health_timeout 1s`, `fail_duration 10s`; `header` безопасности (`Strict-Transport-Security`, `X-Content-Type-Options`, `Referrer-Policy`); `request_body max_size 52MB`; `/metrics` наружу не проксируется (порт метрик отдельный и не в сети `edge`); журнал JSON.
  - Проверка: `docker compose -f deploy/app/compose.yml --env-file deploy/app/env/staging.env.example config --quiet` и то же для продакшена (с профилями `slot-a`, `slot-b`, `tools`); `caddy validate` с подставленными значениями.
  - Логирование: журналы контейнеров — JSON в stdout, ротация Docker (`bootstrap.sh`, 3 × 10 МБ); Caddy — журнал доступа JSON.
  - Файлы: `deploy/app/compose.yml`, `deploy/app/maxim-app.caddy`.
  - Как сделано: переменные разделены на `app.env` (бэкенд) и `stack.env` (подстановка compose, пароли postgres и ключи корзины копий — в api и worker не попадают); у `backup` ещё том `backup-state`; `handle_errors` edge-сайта отвечает JSON `503 unavailable`. `docker compose config --quiet` — оба шаблона и репетиция со всеми профилями; `caddy validate` (caddy:2.11.7) — «Valid configuration».

- [x] **Task 6: Скрипт выпуска и проверка миграций** (depends on 1, 2, 5).
  - `deploy/app/release.sh` (bash, `set -Eeuo pipefail`, `trap ERR`): аргументы `--tag <tag>` | `--rollback`, `--dir <APP_DIR>` (по умолчанию каталог скрипта), `--env-file`, `--health-timeout 120`, `--skip-pull`, `--edge-reload` (команда перезагрузки edge-Caddy, по умолчанию как в `deploy.yml`), `--dry-run`. Состояние ячеек — `slots.env` (`ACTIVE_SLOT`, `MAXIM_TAG_A`, `MAXIM_TAG_B`), пишется атомарно через временный файл. Блокировка `flock` на `release.lock`: два выпуска одновременно не идут.
  - Порядок выпуска: (1) `docker pull` нового тега; (2) `maxim config check --for api` и `--for worker` новым образом с `app.env`, `--for migrate` с `MIGRATE_DATABASE_URL`; (3) `docker compose up -d postgres backup` и ожидание `healthy`; (4) `docker compose run --rm migrate`; (5) запуск `api-<new>` и `worker-<new>` с новым тегом; ожидание `healthy` до `--health-timeout`, при неудаче — остановка новой ячейки, печать журналов, код 1 (старая ячейка продолжает работать; миграции совместимы по правилу); (6) пауза 3 с, чтобы edge-Caddy увидел новую ячейку живой; (7) `docker compose stop` старой ячейки (плавно, по `stop_grace_period`); (8) запись `slots.env`; (9) итог: тег, ячейка, длительность.
  - `--rollback`: если у неактивной ячейки есть тег — запустить её (без миграций), дождаться готовности, остановить активную, поменять `ACTIVE_SLOT`.
  - Первый выпуск (нет `slots.env`): ячейка `a`, остановки старой нет.
  - Тот же тег, что у активной ячейки, — выход без изменений (идемпотентность).
  - `deploy/scripts/check-migrations.sh [<base-ref>]`: новые и изменённые файлы `migrations/*.sql` относительно базы (`git diff --name-only <base>...HEAD`, по умолчанию `origin/master`); в разделе `+goose Up` ищет `DROP TABLE`, `DROP COLUMN`, `RENAME`, `ALTER COLUMN … TYPE`, `SET NOT NULL`, `DROP CONSTRAINT`. Найдено без строки `-- release: contract` — ошибка с файлом и строкой. Изменение уже существующей миграции — ошибка всегда.
  - Правила миграций и порядок выпуска — в `deploy/app/README.md` (задача 11) и строкой в 07 §8.
  - Логирование: JSON-строки на каждый шаг (`event=release_step`, `step`, `slot`, `tag`, `duration_ms`), `level=error` с шагом и кодом выхода; при неудаче готовности — последние 50 строк журнала новой ячейки.
  - Файлы: `deploy/app/release.sh`, `deploy/scripts/check-migrations.sh`.
  - Как сделано: замок — каталог `.release.lock` (`mkdir` атомарен, `flock` есть не везде, где идёт репетиция); вместо `--edge-reload` — `--edge-dir DIR` (по умолчанию `/opt/edge`, если есть) и `--no-edge`, команда reload — как в `deploy.yml`; при ошибке validate/reload прежний файл сайта возвращается, новая ячейка останавливается; `--compose-file` — дополнительный файл compose (репетиция). Ключа `--env-file` нет: файл переменных бэкенда задаёт `APP_ENV_FILE` в `stack.env`. `config check` для api, worker и migrate идёт через сервис `migrate` (новый образ, только сеть `backend`): временный контейнер не получает алиас ячейки в сети `edge`. `check-migrations.sh` проверен на всей истории (`checked=10`, без ошибок) и на отрицательном примере во временном репозитории (изменённая миграция, `ALTER COLUMN … TYPE`, `RENAME` — ошибки; файл с `-- release: contract` — предупреждение).

- [x] **Task 7: Локальная репетиция выпуска** (depends on 6).
  - `deploy/local/compose.rehearsal.yml` — дополнение к `deploy/app/compose.yml`: сеть `edge` не внешняя (`name: maxim-rehearsal-edge`), свой Caddy (`caddy:2.11.7`) с сайтом из `maxim-app.caddy` (адрес `:80`, порт хоста `127.0.0.1:${REHEARSAL_HTTP_PORT:-18480}`), MinIO с созданием корзин и Mailpit; всё под проектом `maxim-rehearsal`, порты не пересекаются со стандартными.
  - `deploy/local/rehearse-release.sh`: собирает образ бэкенда дважды с разными `VERSION` (`maxim:rehearsal-v1`, `maxim:rehearsal-v2`) и образ `maxim-postgres`; первый выпуск v1; фоновый цикл запросов через Caddy каждые 50 мс к `GET /livez` (отвечает само приложение, `200`, пока процесс обслуживает запросы; публичных `GET` операций API без сессии нет — `/auth/code` и `/auth/login` меняющие) и к `GET /api/v1/me` (ожидается `401` от приложения, а не `502/503` от прокси); выпуск v2; `--rollback` на v1; остановка цикла. Успех — ни одного `5xx` и ни одной ошибки соединения, версии ячеек совпадают с ожидаемыми, задания не потеряны (нет `running` без `locked_by` после выпуска). В конце — `docker compose down -v` своего проекта (только `maxim-rehearsal`; чужие контейнеры не трогаются), ключ `--keep` оставляет стек.
  - Логирование: JSON-строки шагов; итог `event=rehearsal_result` с числом запросов, ошибок и длительностью каждого выпуска.
  - Файлы: `deploy/local/compose.rehearsal.yml`, `deploy/local/rehearse-release.sh`, `deploy/local/rehearsal.env` (значения разработки).
  - Как сделано: edge-прокси репетиции — отдельный проект `deploy/local/edge` (как `/opt/edge` на сервере: `Caddyfile` с `import sites/*.caddy`, сайт кладёт `release.sh`, reload через `docker compose exec`), сеть `maxim-rehearsal-edge`; переменные — `rehearsal.app.env` и `rehearsal.stack.env` (два файла, как на сервере); рабочие копии — `deploy/local/.state/` (в `.gitignore`). Итог на Windows + Docker Desktop: `requests=503, unexpected=0, errors_5xx=0, connection_errors=0, orphan_running_jobs=0`, выпуск ≈ 29 с, откат ≈ 25 с; после прогона контейнеров, томов и сетей `maxim-rehearsal*` не осталось.

### Фаза 4. Резервные копии и восстановление

- [x] **Task 8: Скрипты резервного копирования** (depends on 4, 5).
  - `deploy/postgres/bin/backup-lib.sh`: журнал JSON, настройка `mc` через `MC_HOST_backup` и `MC_HOST_app` (из `BACKUP_S3_*` и `S3_*`), путь копий `${BACKUP_S3_BUCKET}/${BACKUP_PREFIX}/…`, файл состояния `/var/lib/backup-state/<kind>.json` (время, итог, размер), письмо о сбое `notify_failure <kind> <text>` через `curl --url smtp(s)://…` на `ALERT_EMAILS`, не чаще раза в час на вид.
  - `backup-dump.sh`: `pg_dump -Fc -Z 6` в `backup-tmp`, `sha256sum`, выгрузка `db/dump/YYYY/MM/DD/maxim-<ts>.dump` и `.sha256`, манифест (`goose` версия, размер, длительность, `server_version`), удаление временного файла.
  - `backup-base.sh`: `pg_basebackup -U maxim_backup -Ft -z -X none -c fast --checkpoint=fast -l <метка>` (WAL берётся из архива), выгрузка `db/base/<ts>/` и манифест с начальным сегментом (`backup_label`).
  - `wal-upload.sh`: каждый сегмент из `wal-archive` (кроме `.tmp`) — `mc cp` в `db/wal/<сегмент>`, проверка размера, удаление локального файла; итог в файл состояния.
  - `backup-prune.sh`: логические копии старше `BACKUP_DUMP_RETENTION_DAYS` (30), физические старше `BACKUP_BASE_RETENTION_DAYS` (8), но всегда хранится хотя бы одна; WAL старше начального сегмента самой старой оставшейся физической копии.
  - `backup-objects.sh`: однократно `mc version enable` и правило `noncurrent expire 30d` на корзинах приложения (если провайдер не поддерживает — WARN и продолжение); ежедневно `mc mirror --overwrite` корзин `S3_BUCKET_PRIVATE` и `S3_BUCKET_PUBLIC` в `objects/<bucket>/`.
  - `backup-agent.sh`: точка входа контейнера `backup` — пишет `crontab` из `deploy/postgres/crontab` (расписание из переменных `BACKUP_*_CRON` с допущением П-7), запускает `crond -f -d 8` в фоне и цикл `wal-upload.sh` каждые `BACKUP_WAL_INTERVAL` (30 с); `SIGTERM` — последняя выгрузка WAL и выход. `backup-health.sh` для `healthcheck`: последняя успешная выгрузка WAL не старше 5 минут, последняя логическая копия не старше 26 часов (после первых суток).
  - Логирование: `{"ts","level","component":"backup","event":"dump_done","bytes","duration_ms","key"}` и аналогично для каждого вида; `level=error` с командой и кодом выхода; пароли и ключи не выводятся (`MC_HOST_*` не печатается).
  - Файлы: `deploy/postgres/bin/backup-*.sh`, `deploy/postgres/bin/wal-upload.sh`, `deploy/postgres/crontab`, `deploy/postgres/Dockerfile` (копирование скриптов).
  - Как сделано: `mc` настраивается `alias set` во временном каталоге процесса (а не `MC_HOST_*`: так ключи не попадают в окружение дочерних процессов и не остаются на томе); `crond` работает от root, задания переходят на `postgres` через `gosu`; проверка размера — `mc stat --no-list` (без него `stat` по ключу `x.dump` выдаёт и `x.dump.sha256` — найдено репетицией).

- [x] **Task 9: Проверка восстановления и репетиция** (depends on 8).
  - `deploy/postgres/bin/restore-check.sh [--key <dump>]`: последняя логическая копия (или заданная) → проверка `sha256` → временный кластер (`initdb` в `/tmp/restore-check`, порт 5433, `pg_ctl start`) → `pg_restore --no-owner --exit-on-error` → проверки: версия `goose_db_version` равна версии из манифеста копии; таблицы из списка (`organization`, `placement`, `ledger_account`, `ledger_entry`, `media_file` …) существуют; журнал проводок сходится — те же запросы, что у задания `check_ledger_balance` (`internal/ledger/postgres/queries/reports.sql`: `SumBalancesBySide` — активы равны пассивам, `ListCacheMismatches` — кеш остатка равен сумме проводок со знаком «+» для активов `bank`, `rocketwork_deposit`, `payout_fees`, `ListNegativeLiabilities` — нет отрицательных пассивов; в скрипте ссылка на источник, чтобы править вместе); копия моложе `BACKUP_MAX_AGE` (26 ч); для выборки до 50 `media_file.storage_key` без `purged_at` объект есть в `objects/<bucket>/` копии (если копии объектов ещё нет и `media_file` пуст — WARN и пропуск, иначе отсутствие объекта — ошибка); печать длительности восстановления. Кластер удаляется в `trap EXIT`. Код 1 при любой неудаче, письмо через `notify_failure`.
  - `deploy/postgres/bin/restore-pitr.sh --target-time <RFC3339> --pgdata <dir>`: подходящая физическая копия (старт раньше цели) → распаковка → `restore_command` = `mc cp backup/…/db/wal/%f %p` → `recovery_target_time`, `recovery_target_action=promote` → запуск и ожидание конца восстановления. Используется при аварии и на квартальной проверке.
  - `deploy/local/rehearse-backup.sh`: проект `maxim-rehearsal-backup`, свободные порты; поднимает PostgreSQL из `maxim-postgres`, MinIO, `backup`; применяет миграции образом бэкенда; вставляет тестовые строки (организация, счёт журнала, проводка, `media_file` с объектом в MinIO); делает `backup-base.sh`, ещё вставки, фиксирует время T, ещё вставки; `backup-dump.sh`, `backup-objects.sh`; `restore-check.sh` → успех; `restore-pitr.sh --target-time T` во временный каталог → строки до T есть, после T нет; затем порча копии (`sha256` не сходится) → `restore-check.sh` падает. В конце `down -v` своего проекта.
  - Логирование: `restore-check` — `event=restore_check_step` на каждую проверку и итог `event=restore_check_result` (`ok`, `duration_ms`, `dump_key`, `dump_age_h`, `migration_version`, `objects_checked`); `restore-pitr` — шаги и найденная физическая копия.
  - Файлы: `deploy/postgres/bin/restore-check.sh`, `deploy/postgres/bin/restore-pitr.sh`, `deploy/local/rehearse-backup.sh`.
  - Как сделано: временный кластер — в `BACKUP_TMP_DIR` (том, а не `/tmp` в памяти). Репетиция вставляет пользователя, организацию, `media_file` с объектом и таблицу меток `rehearsal_marker`; проводки журнала не вставляются (сверка проходит на пустом журнале — проверка запросов, а не данных). Итог: `restore-check` — все шаги `ok`, `objects_checked=1`; PITR на T — строки `1,2` (строка 3 после T отсутствует, «recovery stopping before commit»); испорченная `.sha256` — `restore-check` падает на шаге `checksum`.

### Фаза 5. CI

- [x] **Task 10: Проверки эксплуатации и ручной выпуск в CI** (depends on 3, 6, 7, 9).
  - `.github/workflows/ops.yml`: на PR и пуш в `master` по путям `deploy/**`, `migrations/**`, `Dockerfile`, `.env.example`, `.github/workflows/ops.yml`. Шаги: `shellcheck` всех `deploy/**/*.sh`; `deploy/scripts/check-env.sh`; `deploy/scripts/check-migrations.sh origin/master` (на PR); `docker compose config` для обоих шаблонов; `caddy validate` сайта; `hadolint` для `deploy/postgres/Dockerfile`; `maxim config check` для шаблона тестового окружения (собранный образ, секреты-заглушки); `deploy/local/rehearse-backup.sh`; `deploy/local/rehearse-release.sh`.
  - `.github/workflows/release.yml`: только `workflow_dispatch` с входами `environment` (`staging` | `production`) и `ref`. Job `image`: сборка и пуш `ghcr.io/leha-creator/maxim:sha-<7>` и `maxim-postgres:sha-<7>`. Job `deploy` (`environment: ${{ inputs.environment }}`; для `production` в GitHub настраиваются обязательные проверяющие — действие человека): SSH на хост окружения (`vars.<ENV>_HOST`, секреты `<ENV>_SSH_KEY`, `<ENV>_KNOWN_HOSTS` по образцу `deploy.yml`), копирование `deploy/app/compose.yml`, `release.sh`, сайта Caddy, `docker login` временным `GITHUB_TOKEN` → `release.sh --tag sha-<7>` → `docker logout` → проверка `https://<домен>/readyz`; при сбое — журналы. `concurrency: release-<env>`.
  - Проверка: `actionlint` обоих файлов.
  - Логирование: `::group::` на каждый шаг, итог в `$GITHUB_STEP_SUMMARY` (окружение, тег, ячейка, итог проверки); секреты не выводятся.
  - Файлы: `.github/workflows/ops.yml`, `.github/workflows/release.yml`.
  - Как сделано: переменные и секреты — в GitHub Environments `staging` и `production` с одинаковыми именами (`DEPLOY_HOST`, `DEPLOY_USER`, `DEPLOY_DOMAIN`, `DEPLOY_SSH_KEY`, `DEPLOY_KNOWN_HOSTS`), а не `<ENV>_HOST`; вход `rollback` — `release.sh --rollback` без сборки; образ PostgreSQL собирается и публикуется, но на сервере меняется вручную (`MAXIM_POSTGRES_IMAGE`, перезапуск базы — короткий простой). Шаг `maxim config check` в `ops.yml` нашёл ошибку шаблона тестового окружения: путь `MAX_CA_CERT_FILE` при отсутствующем в образе сертификате не даёт запуститься worker — путь в шаблоне пуст до добавления сертификата (действие человека). Проверено: `actionlint` 1.7.9 обоих файлов — без замечаний; шаги `config check` и `shellcheck` повторены локально.

### Фаза 6. Документация и материалы для людей

- [x] **Task 11: Руководство по эксплуатации** (depends on 3, 6, 9, 10).
  - `deploy/app/README.md`: окружения и что в них настоящее, что подделка; схема (edge-Caddy → ячейки `api` → PostgreSQL, `worker`, `backup` → S3); первый запуск сервера (`bootstrap.sh`, каталог `/opt/maxim-<env>`, `app.env` из шаблона и хранилища секретов, корзины и ключи S3, роль владельца схемы, публикация юридических документов `maxim legal publish`, выдача роли администратора); выпуск и откат; правила миграций «расширить → перенести → сузить»; резервные копии (что, куда, когда, сколько хранится), восстановление логической копии, восстановление на момент времени, квартальная проверка (порядок, протокол, кто подписывает — действие человека); переход на управляемую PostgreSQL; действия при сбое копии, переполнении диска, недоступности S3.
  - `deploy/README.md`: ссылка на `deploy/app/README.md` (стенд заглушки остаётся как есть). `README.md` корня: раздел «Эксплуатация» со ссылками, строки `ops.yml`, `release.yml` в таблице CI. `.ai-factory/ARCHITECTURE.md`: `deploy/app`, `deploy/postgres`, `deploy/local`, `deploy/scripts` в структуре каталогов. `docs/07-non-functional.md` §8: ссылка на руководство в абзацах «Резервные копии» и «Выпуск».
  - Логирование: не требуется (документация); в руководстве — где смотреть журналы `backup`, `release.sh` и Caddy и как фильтровать JSON (`docker compose logs backup | jq`).
  - Файлы: `deploy/app/README.md`, `deploy/README.md`, `README.md`, `.ai-factory/ARCHITECTURE.md`, `docs/07-non-functional.md`.
  - Как сделано: шаги восстановления логической копии из руководства проверены на стеке репетиции (`dropdb`/`createdb`, `pg_restore` потоком из контейнера `backup` — строки и права роли приложения на месте).

- [x] **Task 12: Материалы для юридической готовности и пилота** (depends on —).
  - `docs/launch/README.md`: общий чек-лист запуска «код готов → окружение готово → юридически готово → пилот», каждый пункт с владельцем и пометкой «требует действий человека» там, где нужно действие вне репозитория; ссылки на остальные файлы.
  - `docs/launch/legal-documents.md`: для каждого из четырёх документов — обязательные разделы и значения, которые система исполняет и которые должны совпасть с текстом (таймеры 03 §5 с переменными окружения, формула пропорционального возврата 03 §7, основания споров 03 §6, комиссия `PLATFORM_FEE_RATE_BP`, порядок пополнения по счёту и сроки счёта `INVOICE_TTL`, маркировка силами платформы — Q-31, закрывающие документы — Q-02, блокировка заказов до принятия новой версии — 06 §9); что видно в справочной оферте Telega.in по составу разделов (без копирования текста) и чем наша схема отличается (ИП, а не ООО; комиссия сверху цены блогера, а не коэффициент; без ЭДО; выплаты через Рокет Ворк); порядок публикации версии (`maxim legal publish`, проверка `legal list`) и что происходит с пользователями.
  - `docs/launch/personal-data.md`: реестр обработки по 04 §10 (данные, субъекты, цель, основание, срок, где хранится, кому передаётся: Яндекс ОРД, Рокет Ворк, почтовый сервис, хостинг); локализация в России; меры защиты, которые уже есть в коде (маскирование в журнале, шифрование секретов, роли базы, HTTPS, резервные копии в России); уровень защищённости (П-8); порядок по запросу субъекта (доступ, исправление, удаление = блокировка и обезличивание контактов, 06 §8) и сроки ответа по 152-ФЗ; порядок при утечке (уведомление РКН в течение 24 часов и результаты расследования в течение 72 часов, ст. 21 ч. 3.1); что сделать в коде позже (команда обезличивания — в API администратора); вопросы юристу (Q-33).
  - `docs/launch/rkn-notification.md`: черновик сведений для уведомления об обработке ПДн по ч. 3 ст. 22 152-ФЗ, заполненный по реестру обработки, с заглушками П-5 и П-9; порядок подачи (портал pd.rkn.gov.ru или Госуслуги, до начала обработки) — «требует действий человека»; что проверить юристу.
  - `docs/launch/pilot-plan.md`: цели и критерии успеха (доля размещений без ручного вмешательства, нарушения и споры, время реакции дежурного, сходимость журнала проводок, erid у каждой публикации); отбор каналов и рекламодателей (П-6); подготовка (тестовое окружение пройдено по сценариям S-01 … S-xx, юридические документы опубликованы, уведомление РКН подано, бот MAX и ОРД боевые, Рокет Ворк подключён — после этапа выплат); ежедневный порядок дежурного (оповещения, зависшие объекты, `admin ord errors`, выписка банка, сверка); критерии остановки и откат; что собрать по итогам.
  - `docs/README.md`: строка про `docs/launch/`. `docs/06-finance-legal.md` §8 и §9: ссылки на соответствующие файлы `docs/launch/`.
  - Вопросы, которые появились (восстановление копии продакшена для квартальной проверки — где, если данные продакшена нельзя переносить; кто подписывает протокол проверки), — в раздел «Вопросы» `docs/launch/README.md` с предложением внести их в реестр `docs/open-questions.md` при слиянии (номера `Q-NN` не занимаются, чтобы не конфликтовать с параллельными ветками).
  - Логирование: не требуется (документы).
  - Файлы: `docs/launch/README.md`, `docs/launch/legal-documents.md`, `docs/launch/personal-data.md`, `docs/launch/rkn-notification.md`, `docs/launch/pilot-plan.md`, `docs/README.md`, `docs/06-finance-legal.md`.
  - Как сделано: в `docs/launch/README.md` к двум вопросам плана добавлены ещё два (поручение обработки хостингу и почтовому сервису; срок хранения данных удалённой учётной записи) с предложениями по умолчанию; справочная оферта Telega.in описана только по составу разделов, текст не копировался, PDF не коммитился.

## Commit Plan

- **После задач 1–2:** `feat(ops): health probe, config check and drain before api shutdown`
- **После задачи 3:** `chore(deploy): environment templates with a slot for upcoming modules`
- **После задач 4–9 (одним коммитом: стек compose ссылается на контейнер `backup` и его скрипты, по отдельности образ и репетиция не собираются):** `feat(deploy): backend stack with zero-downtime release, backups and restore check`
- **После задачи 10:** `ci: ops checks and manual release workflow`
- **После задачи 11:** `docs(deploy): operations runbook`
- **После задачи 12:** `docs(launch): legal readiness checklists, personal data, RKN notice draft and pilot plan`

## Что дописать после слияния других этапов

| Этап | Что дописать |
|---|---|
| Выплаты блогерам (Рокет Ворк) | Переменные `ROCKETWORK_*`, `PAYOUT_*` — в блок «Модули в разработке» обоих шаблонов окружений (тестовое — тестовая среда Рокет Ворк или подделка, Q-12); `check-env.sh --print-missing` для сверки; адрес вебхука Рокет Ворк в руководстве (идёт через тот же сайт Caddy); в `restore-check.sh` — таблицы выплат в списке обязательных; в `pilot-plan.md` — снять пометку «после этапа выплат». Сделано при слиянии выплат (план `rocketwork-payouts.md`, M-3 … M-5); тестовое окружение — `disabled` |
| Уведомления и отчёт по кампании | Переменные модуля — в блок шаблонов; в тестовом окружении письма — в Mailpit; оповещения о сбое копий можно перевести на общий механизм уведомлений |
| Приёмка, споры и возвраты | Переменные модуля — в блок шаблонов; сроки споров в `legal-documents.md` сверить с реализацией |
| API администратора и отчётность ОРД | Переменные модуля — в блок шаблонов; ограничение доступа к `/api/v1/admin` на edge-Caddy (по IP или отдельному домену) — решить и дописать в `maxim-app.caddy`; команда обезличивания по запросу субъекта ПДн — в `personal-data.md`; ежемесячная отчётность ОРД — в ежедневный порядок дежурного пилота |

## Риски

- **Версионирование у провайдера S3.** Если хранилище не поддерживает версионирование или правила жизненного цикла, `backup-objects.sh` выдаёт WARN, и остаётся только ежедневная копия: потеря объектов за сутки. Проверить у провайдера (действие человека).
- **Диск сервера.** Архив WAL копится локально, если S3 недоступен. Здоровье `backup` падает через 5 минут; письмо уходит, если доступна почта. Размер тома и порог — в руководстве.
- **Ресурсы.** Временный кластер `restore-check` на сервере продакшена требует места не меньше размера базы и нагружает диск; проверка ставится на ночь воскресенья.
- **MinIO вместо S3 провайдера локально.** Поведение `mc mirror` и версионирования у провайдера может отличаться; первая настройка на тестовом окружении — с ручной проверкой.
- **Юридические тексты.** Чек-листы не заменяют юриста; запуск пилота без опубликованных юристом документов и поданного уведомления РКН недопустим.
