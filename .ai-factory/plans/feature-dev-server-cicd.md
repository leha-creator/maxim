# План реализации: тестовый сервер и CI/CD для заглушки API

Branch: feature/dev-server-cicd
Created: 2026-10-15

## Цель

1. Привязать локальный репозиторий к `https://github.com/leha-creator/maxim.git`.
2. Подготовить тестовый сервер `95.163.229.132` (домен `dev.plwedding.ru`) к запуску заглушки API на Go из плана `feature-mock-api-swagger`.
3. Настроить GitHub Actions: каждый пуш в `master` проверяет код, собирает образ и обновляет версию на сервере.

Сервер тестовый, данные заглушки выдуманные. Это не production-контур: без мониторинга, резервных копий и нескольких окружений.

## Settings
- Testing: yes — проверки конфигурации без юнит-тестов: `shellcheck` для скрипта, `docker compose config`, `caddy validate`, `actionlint`, проверка после деплоя (smoke) прямо в workflow
- Logging: verbose — скрипт подготовки пишет каждый шаг с временем; Caddy пишет журнал доступа в JSON; у Docker ротация журналов; шаги workflow выводят версию, тег образа и результат проверки
- Docs: no — `deploy/README.md` входит в задачу 8, `/aif-docs` не нужен

## Roadmap Linkage
Milestone: "none"
Rationale: тестовый стенд для демонстрационной заглушки, ни один этап роадмапа не закрывает.

## Research Context
Source: .ai-factory/RESEARCH.md (Active Summary)

Из исследования для этой задачи важно одно: стек проекта — Go (бэкенд) + React (кабинеты), решение заказчика от 2026-10-12. Остальные решения касаются предметной области и на инфраструктуру стенда не влияют.

## Принятые решения

| Вопрос | Решение | Почему |
|---|---|---|
| Запуск на сервере | Docker Engine + `docker compose` | Выбор пользователя. Тот же способ подойдёт потом для бэкенда и фронтенда |
| Реестр образов | GHCR: `ghcr.io/leha-creator/maxim-mockapi`, теги `sha-<короткий sha>` и `master` | Пушит `GITHUB_TOKEN` без отдельных ключей; по тегу `sha-*` можно откатиться |
| Как сервер получает образ из закрытого GHCR | Деплой-шаг передаёт `GITHUB_TOKEN` в `docker login --password-stdin` на сервере, делает `pull` и сразу `docker logout` | Постоянный токен на сервере не хранится; `GITHUB_TOKEN` живёт только во время запуска |
| Прокси и HTTPS | Caddy в том же compose, автоматический сертификат Let's Encrypt для `dev.plwedding.ru` | Выбор пользователя. Пока DNS не обновился, Caddy сам повторяет выпуск сертификата |
| Сеть | Наружу открыты только 80/443 (Caddy) и 22. Заглушка слушает `:8080` во внутренней сети compose, порт на хост не публикуется | Docker обходит `ufw` для опубликованных портов, поэтому лишние порты просто не публикуются |
| Пока кода заглушки нет | Workflow проверяет наличие `mockapi/go.mod`. Нет файла — проверки и сборка пропускаются, деплой поднимает только Caddy, а Caddy отдаёт `503` со страницей «заглушка ещё не развёрнута» | Ветку инфраструктуры можно влить в `master` раньше заглушки, и пуши в `master` не будут красными |
| Доступ к серверу | Пароль `root` используется один раз при первом входе (через `plink -pw` из PuTTY: `sshpass` на машине нет). Дальше только ключи; вход по паролю отключается | Пароль был в чате, поэтому после настройки его нужно сменить |
| Пользователь для деплоя | `deploy` в группе `docker`, отдельный ключ `ed25519` только для GitHub Actions | `root` в CI не попадает. Группа `docker` фактически даёт root-доступ; для тестового стенда это допустимо и описано в README |
| Где лежат файлы на сервере | `/opt/maxim/`: `compose.yml`, `Caddyfile`, `.env` (тег образа, домен) | Workflow копирует их из репозитория при каждом деплое, на сервере ничего не правится руками |
| Контекст сборки образа | Корень репозитория, `docker build -f mockapi/Dockerfile .` | Генератор может понадобиться `api/openapi.yaml`; лишнее отсекает корневой `.dockerignore` |
| Окончания строк | `.gitattributes`: `*.sh`, `Caddyfile`, `*.yml` — `eol=lf` | Репозиторий ведётся на Windows; с CRLF скрипт на сервере не запустится |

**Секреты.** Пароль, приватные ключи и токены не попадают ни в репозиторий, ни в план, ни в журналы. Приватный ключ деплоя хранится на машине разработчика в `~/.ssh/maxim_deploy` и в секрете GitHub `DEPLOY_SSH_KEY`.

## Что нужно от плана заглушки (`feature-mock-api-swagger`)

Эти пункты нужно внести в тот план (задача 8), иначе Swagger UI на стенде не будет работать как надо:
- `servers` в `api/openapi.yaml` — добавить относительный `/api/v1` первым, `http://localhost:8080/api/v1` оставить вторым; иначе «Try it out» на `https://dev.plwedding.ru/docs` будет ходить на `localhost`.
- `GET /healthz` → `200 {"status":"ok"}` вне `/api/v1` и без сессии, для проверки после деплоя (пока его нет, workflow проверяет `GET /openapi.yaml`).
- Cookie сессии: на стенде включать `Secure` (переменная окружения `MOCKAPI_COOKIE_SECURE=true`, локально по умолчанию `false`).
- Адрес по-прежнему из `MOCKAPI_ADDR`, по умолчанию `:8080`; журнал в JSON в stdout (уже есть в плане).

## Tasks

### Фаза 1. Репозиторий

- [x] **Task 1: Привязать GitHub-репозиторий.** `git remote add origin https://github.com/leha-creator/maxim.git`, `git push -u origin master`, затем `git push -u origin feature/mock-api-swagger feature/dev-server-cicd`. Перед пушем проверить `git ls-remote origin`: репозиторий должен быть пустым. Если там уже есть коммиты, остановиться и спросить пользователя, а не делать `--force`. Неотслеживаемые файлы (`Telega Public API.postman_collection.json`, PDF оферты в `docs/`, `.ai-factory/plans/feature-mock-api-swagger.md`) не коммитить без отдельного решения пользователя; в отчёте перечислить их.
  Логирование: вывести `git remote -v` и результат `git ls-remote origin` после пуша (список веток с sha).
  Если `git push` попросит авторизацию, пользователь входит через Git Credential Manager; учётные данные в чат не передаются.

### Фаза 2. Подготовка сервера

- [x] **Task 2: Скрипт подготовки сервера** (depends on —). Создать `deploy/bootstrap.sh`: идемпотентный, `#!/usr/bin/env bash`, `set -Eeuo pipefail`, `trap` на `ERR` печатает номер строки и команду. Аргументы: `--deploy-pubkey <строка ключа>`, `--admin-pubkey <строка ключа>`. Шаги:
  1. Проверить ОС (`/etc/os-release`): поддерживаются Ubuntu 22.04/24.04 и Debian 12; на других ОС выйти с ошибкой.
  2. `apt-get update && apt-get -y upgrade`, пакеты `ca-certificates curl gnupg ufw unattended-upgrades dnsutils`; включить `unattended-upgrades` только для обновлений безопасности.
  3. Часовой пояс `UTC`. Если памяти меньше 2 ГБ и swap нет, создать `/swapfile` на 2 ГБ и прописать в `/etc/fstab`.
  4. Docker Engine и `docker-compose-plugin` из официального apt-репозитория Docker (не `get.docker.com`). В `/etc/docker/daemon.json` — `"log-driver": "json-file"`, `"log-opts": {"max-size": "10m", "max-file": "3"}`; `registry-mirrors` — только если задан `--registry-mirror` (на случай недоступности Docker Hub из РФ, например `https://mirror.gcr.io`).
  5. Пользователь `deploy`: без пароля, группа `docker`, `~deploy/.ssh/authorized_keys` = deploy-ключ. Ключ администратора добавить в `/root/.ssh/authorized_keys`. Ключи дописываются только если их там ещё нет.
  6. `/opt/maxim` с владельцем `deploy:deploy`, права `750`.
  7. `ufw`: `default deny incoming`, разрешить `22/tcp`, `80/tcp`, `443/tcp`, `443/udp` (HTTP/3), включить.
  8. SSH: drop-in `/etc/ssh/sshd_config.d/10-maxim.conf` с `PasswordAuthentication no`, `KbdInteractiveAuthentication no`, `PermitRootLogin prohibit-password`. Перед `systemctl reload ssh` выполнить `sshd -t`. Применяется последним шагом и только с флагом `--harden-ssh`, чтобы при первом запуске сначала проверить вход по ключу.
  Логирование: функция `log()` печатает `[2026-10-15T12:00:00Z] [bootstrap] [step N/8] <что делаем>` в stdout; в конце каждого шага — `done` или `skip (already configured)`. В конце — сводка: ОС, версия Docker и compose, RAM/swap, статус `ufw`, `sshd -T | grep -E 'passwordauthentication|permitrootlogin'`.
  Создать `.gitattributes` с `*.sh text eol=lf`, `Caddyfile text eol=lf`, `*.yml text eol=lf`, `*.yaml text eol=lf`.
  Проверка: `docker run --rm -v "$PWD:/mnt" koalaman/shellcheck:stable deploy/bootstrap.sh` без замечаний.
  Файлы: `deploy/bootstrap.sh`, `.gitattributes`.

- [x] **Task 3: Применить подготовку на сервере** (depends on 2). Шаги:
  1. Сгенерировать ключ деплоя: `ssh-keygen -t ed25519 -N "" -C "github-actions@maxim-dev" -f ~/.ssh/maxim_deploy`. Если файл уже есть, не перезаписывать.
  2. Первый вход по паролю через `plink -batch -pw` (PuTTY). Пароль берётся из сообщения пользователя, в файлы и команды-описания не пишется. Отпечаток ключа сервера сверить один раз и сохранить в `~/.ssh/known_hosts` (`ssh-keyscan -t ed25519 95.163.229.132`). Положить в `/root/.ssh/authorized_keys` ключ администратора `~/.ssh/id_ed25519.pub`.
  3. Проверить вход по ключу: `ssh root@95.163.229.132 true`. Дальше работать только по ключу.
  4. `scp deploy/bootstrap.sh` в `/root/`, запустить `bash /root/bootstrap.sh --deploy-pubkey "<maxim_deploy.pub>" --admin-pubkey "<id_ed25519.pub>"` и сохранить вывод в `/var/log/maxim-bootstrap.log` (`| tee`).
  5. Проверить вход `ssh -i ~/.ssh/maxim_deploy deploy@95.163.229.132 docker ps`, затем перезапустить скрипт с `--harden-ssh`. Убедиться, что вход по паролю отклоняется: `ssh -o PubkeyAuthentication=no -o PreferredAuthentications=password root@...` должен получить `Permission denied`.
  6. DNS: `dig +short dev.plwedding.ru @1.1.1.1` и `@8.8.8.8` должны вернуть `95.163.229.132`. Если не возвращают, это не блокирует остальные задачи: записать в отчёт, Caddy выпустит сертификат позже сам.
  Логирование: в отчёт задачи — сводка из конца скрипта, отпечаток ключа сервера, результат проверки DNS. Ключи и пароль в отчёт не выводить.
  После задачи сказать пользователю: смените пароль `root` в панели хостинга, он был передан в чате. Вход по паролю уже отключён, но пароль остаётся для консоли VNC.
  Файлы: изменений в репозитории нет.

### Фаза 3. Конфигурация запуска

- [x] **Task 4: Compose и Caddy** (depends on 2). Создать:
  - `deploy/compose.yml`: проект `maxim`; сервис `mockapi` — `image: ghcr.io/leha-creator/maxim-mockapi:${MOCKAPI_TAG:-master}`, `restart: unless-stopped`, `environment: MOCKAPI_ADDR=:8080, LOG_LEVEL=${LOG_LEVEL:-debug}, MOCKAPI_COOKIE_SECURE=true`, `read_only: true`, `security_opt: [no-new-privileges:true]`, ограничение памяти `256m`, порт не публикуется. Сервис `caddy` — `image: caddy:<точная версия 2.10.x, закрепить при реализации>`, порты `80:80`, `443:443`, `443:443/udp`, тома `caddy_data`, `caddy_config` и `./Caddyfile:/etc/caddy/Caddyfile:ro`, `environment: DOMAIN=${DOMAIN}`, `restart: unless-stopped`. `depends_on` не указывать: Caddy должен подниматься и без заглушки. Общая сеть `maxim`.
  - `deploy/Caddyfile`: сайт `{$DOMAIN}`; `encode zstd gzip`; `header X-Robots-Tag "noindex, nofollow"`; `reverse_proxy mockapi:8080` с `health_uri /openapi.yaml`; `handle_errors 502 503` → `respond "Заглушка API ещё не развёрнута" 503`; `redir / /docs` для корня; журнал `log { output stdout; format json }`. В глобальном блоке — `email` для Let's Encrypt (адрес владельца домена узнать у пользователя; если его нет, блок не добавлять).
  - `deploy/.env.example`: `DOMAIN=dev.plwedding.ru`, `MOCKAPI_TAG=master`, `LOG_LEVEL=debug`.
  Проверка: `docker compose -f deploy/compose.yml --env-file deploy/.env.example config` без ошибок; `docker run --rm -v "$PWD/deploy/Caddyfile:/etc/caddy/Caddyfile:ro" -e DOMAIN=dev.plwedding.ru caddy:<версия> caddy validate --config /etc/caddy/Caddyfile`.
  Логирование: Caddy пишет журнал доступа в JSON в stdout, Docker его ротирует (задача 2). Уровень журнала заглушки задаёт `LOG_LEVEL` из `.env`.
  Файлы: `deploy/compose.yml`, `deploy/Caddyfile`, `deploy/.env.example`.

- [ ] **Task 5: Образ заглушки** (depends on —). `mockapi/Dockerfile` (multi-stage):
  - сборка: `golang:<версия из mockapi/go.mod>-alpine` (точный тег), `WORKDIR /src`, сначала `COPY mockapi/go.mod mockapi/go.sum`, затем `go mod download` (с кэшем BuildKit `--mount=type=cache`), потом `COPY mockapi/ ./` и `COPY api/ /api/`; `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/mockapi ./cmd/mockapi`;
  - запуск: `gcr.io/distroless/static-debian12:nonroot`, `USER nonroot`, `EXPOSE 8080`, `ENTRYPOINT ["/mockapi"]`;
  - `ARG VERSION`, `LABEL org.opencontainers.image.source=https://github.com/leha-creator/maxim`. Метка связывает пакет GHCR с репозиторием, и `GITHUB_TOKEN` получает право на `pull`.
  Корневой `.dockerignore`: исключить всё, затем разрешить `mockapi/**` и `api/**`; внутри — исключить `mockapi/*.exe`, `**/*_test.go` не исключать (тесты не мешают и нужны для `go vet`).
  Проверка: `docker run --rm -i hadolint/hadolint < mockapi/Dockerfile`. Сборку проверить на ветке `feature/mock-api-swagger`, когда там появится `mockapi/cmd/mockapi`; до этого — только `hadolint`.
  Логирование: при старте заглушка пишет версию (`main.version`) первой строкой журнала. Это требование к плану заглушки, внести его в задаче 8.
  Файлы: `mockapi/Dockerfile`, `.dockerignore`.

### Фаза 4. CI/CD

- [ ] **Task 6: Workflow GitHub Actions** (depends on 4, 5). `.github/workflows/deploy.yml`:
  - Триггеры: `push` в `master`; `pull_request` в `master` (только проверки, без деплоя); `workflow_dispatch` с необязательным входом `tag` (откат на `sha-xxxxxxx` без сборки).
  - `concurrency: { group: deploy-dev, cancel-in-progress: false }` — деплои не наслаиваются друг на друга.
  - `permissions: contents: read, packages: write`.
  - Job `detect`: вывод `has_mockapi = hashFiles('mockapi/go.mod') != ''`.
  - Job `check` (если `has_mockapi`): `actions/checkout@v5`, `actions/setup-go@v6` с `go-version-file: mockapi/go.mod` и кэшем; в `mockapi/`: `go generate ./...` и `git diff --exit-code` (сгенерированный код не устарел), `go vet ./...`, `go test -race ./...`.
  - Job `build` (`needs: check`, только `push`/`workflow_dispatch` без `tag`): `docker/setup-buildx-action@v3`, `docker/login-action@v3` (GHCR, `GITHUB_TOKEN`), `docker/build-push-action@v6` с `file: mockapi/Dockerfile`, `context: .`, `build-args: VERSION=${{ github.sha }}`, теги `sha-<7 символов>` и `master`, кэш `type=gha`.
  - Job `deploy` (`needs: [detect, build]` с `if: always() && !failure() && !cancelled()`, только `master`/`workflow_dispatch`; `environment: dev` с URL `https://dev.plwedding.ru/docs`):
    1. Записать `DEPLOY_SSH_KEY` в `~/.ssh/id_ed25519` (`chmod 600`), `DEPLOY_KNOWN_HOSTS` в `~/.ssh/known_hosts`. `StrictHostKeyChecking=yes`, `ssh-keyscan` не использовать.
    2. `scp deploy/compose.yml deploy/Caddyfile` → `/opt/maxim/`; записать `/opt/maxim/.env` (`DOMAIN`, `MOCKAPI_TAG`, `LOG_LEVEL`) через `ssh ... 'cat > /opt/maxim/.env'`.
    3. Есть заглушка: `echo "$GITHUB_TOKEN" | ssh ... docker login ghcr.io -u ${{ github.actor }} --password-stdin`, затем `docker compose pull`, `docker compose up -d --remove-orphans`, `docker logout ghcr.io`. Нет заглушки: `docker compose up -d caddy`.
    4. `docker image prune -f --filter "until=168h"`.
    5. Проверка: до 12 попыток с интервалом 5 с — `curl -fsS https://dev.plwedding.ru/openapi.yaml` (позже `/healthz`). Если сертификата ещё нет (DNS не обновился), повторить через `curl --resolve dev.plwedding.ru:80:<host> http://...` к `:80` и пометить шаг предупреждением `::warning::TLS not ready`, а не ошибкой. Без заглушки ожидается `503` с текстом заглушки.
    6. При неудачной проверке вывести `docker compose ps` и `docker compose logs --tail=100 mockapi caddy` и завершить job с ошибкой.
  - Хост, пользователь и домен — `vars`/`secrets` репозитория, не захардкожены (кроме значений по умолчанию в `.env.example`).
  Логирование: каждый шаг деплоя печатает `::group::` с понятным названием; в `$GITHUB_STEP_SUMMARY` — коммит, тег образа, digest, результат проверки и ссылка на `/docs`. Секреты в вывод не попадают (`set +x` в шагах с ключами).
  Проверка: `docker run --rm -v "$PWD:/repo" -w /repo rhysd/actionlint:latest` без замечаний.
  Файлы: `.github/workflows/deploy.yml`.

- [ ] **Task 7: Секреты GitHub и первый деплой** (depends on 1, 3, 6).
  1. Секреты репозитория: `DEPLOY_HOST=95.163.229.132`, `DEPLOY_USER=deploy`, `DEPLOY_SSH_KEY` (содержимое `~/.ssh/maxim_deploy`), `DEPLOY_KNOWN_HOSTS` (строка `ssh-keyscan -t ed25519 95.163.229.132`, сверенная с отпечатком из задачи 3). Переменная `DEPLOY_DOMAIN=dev.plwedding.ru`.
     Способ: `gh` на машине не установлен. Предложить `winget install GitHub.cli` и вход пользователя командой `! gh auth login`, затем `gh secret set DEPLOY_SSH_KEY < ~/.ssh/maxim_deploy` и т. д. Если пользователь против `gh`, дать пошаговую инструкцию для Settings → Secrets and variables → Actions. Приватный ключ в чат не выводить.
  2. Создать окружение `dev` (Settings → Environments), защитные правила не нужны.
  3. Проверить настройки пакетов: Settings → Actions → General → Workflow permissions — «Read and write» не требуется, хватает `permissions` в workflow. Убедиться, что Actions включены.
  4. Влить `feature/dev-server-cicd` в `master` (через PR или `git merge` — спросить пользователя) и запушить. Следить за запуском (`gh run watch` или вкладка Actions).
  5. Ожидаемый результат до появления заглушки: job `check`/`build` пропущены, `deploy` поднял Caddy, `https://dev.plwedding.ru` отвечает `503 Заглушка API ещё не развёрнута` с валидным сертификатом (или предупреждение TLS, если DNS ещё не обновился).
  Логирование: в отчёт — ссылка на запуск, статус каждого job и вывод проверки.
  Файлы: изменений в репозитории нет.

### Фаза 5. Документация и согласование

- [ ] **Task 8: README стенда и правки плана заглушки** (depends on 6).
  - `deploy/README.md`: схема (GitHub → GHCR → сервер, Caddy → mockapi); что лежит на сервере и где; как работает деплой; как откатиться (`workflow_dispatch` с `tag=sha-xxxxxxx`); как посмотреть журналы (`ssh deploy@... 'cd /opt/maxim && docker compose logs -f mockapi'`); как перезапустить; повторная подготовка нового сервера через `bootstrap.sh`; список секретов и переменных GitHub; ограничения (тестовый стенд, данные в памяти пропадают при каждом деплое, группа `docker` = root, нет мониторинга и бэкапов); что делать, если Docker Hub недоступен (`--registry-mirror`).
  - `docs/README.md`: строка со ссылкой на `deploy/README.md` и адресом `https://dev.plwedding.ru/docs`.
  - `.ai-factory/plans/feature-mock-api-swagger.md`: внести пункты из раздела «Что нужно от плана заглушки» — `servers` с `/api/v1`, `GET /healthz`, `MOCKAPI_COOKIE_SECURE`, версия первой строкой журнала, `mockapi/Dockerfile` уже есть (задача 9 заглушки не должна создавать его заново), ссылку `README` заглушки на стенд.
  Логирование: не требуется (только документация); в README описать, где искать журналы и в каком формате.
  Файлы: `deploy/README.md`, `docs/README.md`, `.ai-factory/plans/feature-mock-api-swagger.md`.

## Commit Plan

- **После задач 2 и 4:** `chore(deploy): add server bootstrap script and compose/caddy config`
- **После задач 5 и 6:** `ci: build mockapi image and deploy to dev server on push to master`
- **После задачи 8:** `docs(deploy): describe dev stand and align mock api plan`

Задачи 1, 3 и 7 меняют внешнее состояние (GitHub, сервер), а не файлы, поэтому коммитов у них нет. Задачу 1 делать первой: ветки должны быть на GitHub до пушей из следующих задач.

## Риски

- **DNS ещё не обновился.** Caddy будет повторять выпуск сертификата, и проверка в workflow выдаст предупреждение вместо ошибки. Лимиты Let's Encrypt на неудачные проверки (5 в час на домен) при этом не исчерпываются: Caddy увеличивает интервал между попытками.
- **Docker Hub из РФ.** Если `caddy`, `golang` или `distroless` не скачиваются, в `bootstrap.sh` есть флаг `--registry-mirror`. GHCR и `gcr.io` обычно доступны.
- **Пароль `root` был в чате.** Вход по паролю отключается в задаче 3, но сам пароль нужно сменить в панели хостинга.
- **Заглушка открыта всем.** Данные выдуманные, вход по коду `000000`. Если нужно закрыть стенд, в `Caddyfile` можно добавить `basic_auth` одним блоком (сейчас этого нет).
