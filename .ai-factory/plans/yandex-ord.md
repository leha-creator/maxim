# План реализации: Маркировка через Яндекс ОРД

Branch: master (без отдельной ветки, по решению пользователя)
Created: 2026-10-17

## Settings
- Testing: yes. Правила синхронизации (переходы `sync_status`, классификация статусов ОРД, расписание опроса, готовность зависимостей, сборка полезной нагрузки и её хеш, выбор типа контрагента, формы креатива, даты договора) проверяются чистыми табличными тестами `marking/domain`. Адаптер ОРД — через `httptest` с образцами ответов в `internal/integration/ord/testdata`, снятыми со спецификации v8. Сценарии `marking/app` с `postgres` выполняются на настоящей PostgreSQL через `testcontainers-go` с `clock.Manual` и `ordfake`. Сквозные сценарии — на собранных `api` и `worker` (07 §9).
- Logging: verbose. `log/slog` JSON, поля и обязательные события 07 §5.3, 05 §1: объект ОРД поставлен в очередь (DEBUG), отправлен (INFO, внешняя операция выполнена), статус ОРД получен (DEBUG), подтверждён (INFO), ошибка (ERROR + оповещение), повтор (WARN), зависимость не готова (DEBUG), erid получен (INFO), ЕРИР отклонил креатив (ERROR + оповещение), истёк T-11 (WARN + оповещение). Поля `ord_object_type`, `ord_object_id`, `organization_id`, `channel_id`, `post_version_id`, `creative_id`, `order_id`, `reqid`, `ord_status`, `job_id`. ИНН — только маскированным, наименование организации, текст поста, OAuth-токен ОРД и полные тела ответов выше DEBUG в журнал не пишутся (07 §5.4).
- Docs: yes. В конце обязательная точка `/aif-docs`: README бэкенда (переменные ОРД, как в `compose.dev.yml` с `ordfake` пройти «одобрить пост → erid», как подключить тестовый контур, команда `maxim admin ord`).

## Roadmap Linkage
Milestone: "Маркировка через Яндекс ОРД"
Rationale: план реализует этап целиком: контрагенты (включая саму платформу), изначальные договоры и договоры с блогерами, площадки, креативы и erid в посте, опрос статусов и ошибки (S-16 шаг 3, S-17 шаг 3, S-20, S-28), один код ККТУ из конфигурации (A-42). Ежемесячная статистика и акты (S-31) — этап «API администратора и отчётность ОРД».

## Research Context
Source: .ai-factory/RESEARCH.md (Active Summary и сессия «Яндекс ОРД: проверка API»), выдержка по этапу

- ОРД — Яндекс, API v8 (D-06). Продакшен `https://ord.yandex.net`, тестовый контур `https://ord-prestable.yandex.net`, OAuth-токен пользователя кабинета; один пользователь — один кабинет.
- Маркирует платформа (A-02): регистрирует контрагентов, договоры, креативы, получает erid. Один креатив (версия поста) — один токен на весь заказ (D-07).
- Креатив регистрируется только на договор с конечным рекламодателем; токен возвращается сразу, но регистрация подтверждается только финальным статусом ЕРИР.
- До первого показа креатив можно обновить, после — нельзя; система обновление креатива не использует (D-03).
- Картинка с плашкой доступна по публичной ссылке для `mediaData`.
- Открыто: тип площадки для канала MAX (Q-13, рабочее `site`), типы договоров и роли (Q-14, A-33), лимиты API (Q-15, рабочее 5 rps), код ККТУ (Q-39, A-42).

Сверено со спецификацией `ord.yandex.ru/api/openapi.json` (2026-10-17):
- Все объекты идентифицируются **нашими** идентификаторами (`id`, `platformId`; 1–128 символов `^[A-Za-z0-9\-_]+$`); `POST` — «регистрация/обновление» по этому идентификатору. Ключа идемпотентности нет: повтор с тем же `id` безопасен.
- `POST /organization`: обязательны `id`, `type` (`ul`, `ip`, `fl`, `ffl`, `ful`), `isOrs`, `isRr`; `inn` — 10 цифр для `ul`, 12 для `ip`/`fl`; `name`, `kpp`, `address`.
- `POST /contract`: обязательны `id`, `type` (`contract`, `intermediary-contract`, `additional-agreement`), `contractorId`, `clientId`, `isRegReport`, `date`; `subjectType` (`org-distribution`, `mediation`, `distribution`, `representation`, `other`), `actionType` (`distribution`, `conclude`, `commercial`, `other`), `agentActingForPublisher` — только и обязательно для `intermediary-contract`; ИНН клиента и исполнителя различны.
- `POST /platforms`: `organizationId`, массив `platforms[]` (`platformId`, `isOwned`, `type` `site|apps|smarttv|hbbtv`, `name` ≤ 100, `url` ≤ 2000).
- `POST /creative`: `id`, `creativeType = creative`, `isSocial`, `isSocialQuota`, `coBranding` (при `false` — ровно один договор в `contractIds`), `kktuCodes` (`^\d{1,3}\.\d{1,3}\.\d{1,3}$`), `form` (`text-block`, `text-graphic-block`, …), `textData[]` (≤ 65 000 символов), `mediaData[]` (`mediaUrl`, `mediaUrlFileType = image`), `urls[]`, `description` ≤ 1000.
- Ответ всех `POST` и `GET /status?reqid=`: `request_id`, `status`, `token` (у креатива), `object_statuses[]` (`object_id`, `status`, `token`, `error_message`), `error_message[]`. Статусы: `Request accepted by ORD`, `ORD success`, `ORD error`, `ORD rejected`, `ERIR sync success`, `ERIR sync error`, `ERIR async success`, `ERIR async error`. Ошибка проверки — `422` с `detail[]` (`loc`, `msg`, `type`).

## Источники ТЗ

- Сценарии: 02 S-16 шаг 3 и отклонение (площадка, контрагент и договор блогера при одобрении канала; канал не в каталоге до регистрации площадки), S-17 шаг 3 (креатив и erid после одобрения), S-20 (ошибки ОРД), S-28 (ошибка регистрации креатива, отклонение ЕРИР), S-29 (новая версия — новый креатив); S-31 — только границы.
- Размещение: 03 §1.1 (`approved → ready` — получен erid), §1.2 (заказ `marking`), §4 (P-02, P-11 с причиной `erir_rejected`), §5 (T-11), §9 (версии и erid), §10 (I-01, I-13), §11.
- Данные: 04 §6 (сущности и поля синхронизации), §2 (`organization.is_platform`, `legal_type`), §9 (`sync_status`, `creative.erir_status`).
- Интеграции: 05 §1 (повторы, `4xx`, неизвестный исход, предохранитель, `outbound_request`), §3.1–§3.5, §3.8, §3.9.
- Финансы и право: 06 §5 (цепочка договоров, A-31, A-33), §6.1 (erid в посте), §7 (ККТУ, A-42), §8 (что передаётся в ОРД).
- Требования: 07 §2 (`sync_ord_object`, `poll_ord_status`), §3 (зависшие «версия в `approved` без erid 1 ч», «объект ОРД в `sent` 24 ч»), §4 (секреты), §5.3–§5.5 (метрика числа объектов ОРД в `error`), §8.
- Реестр: D-03, D-06, D-07, D-18, D-27, A-02, A-31, A-32, A-33, A-42, Q-13, Q-14, Q-15, Q-30, Q-31, Q-39.
- Архитектура: `.ai-factory/ARCHITECTURE.md` — модуль `marking/` и адаптер `integration/ord/` с `ordfake/` уже в дереве; «Модуль → внешняя система», «Модуль → модуль, синхронно», «Порты к будущим модулям», «Порядок блокировок».
- Существующий код: `internal/post/app/versions.go` (`ApproveVersion`, `MarkVersionReady` — работает в транзакции вызывающего, `GetVersion` с публичными ссылками), `internal/post/domain/{version.go,final_text.go,ports.go}` (`VersionListener`, `VersionChange`), `internal/placement/app/versions.go` (`VersionChanged`: `approved` → заказ `marking`, `ready` → P-02), `internal/placement/domain/{actions.go,status.go}` (`ReturnToCreated`, `ReasonErirRejected`), `internal/channel/app/moderation.go` (`Approve`, `adminDecision`), `internal/channel/postgres/catalog.go` (выборка каталога), `internal/account/{domain/organization.go,app/organizations.go}` (ИНН и тип лица неизменяемы, `UpdateOrganization`), `migrations/00001_accounts.sql` (`organization.is_platform`, `legal_acceptance`), `internal/platform/outbound/client.go` (`SystemYandexORD`, `Redact`), `internal/platform/ratelimit`, `internal/platform/stuck`, `internal/bootstrap/{integration.go,dev.go,admin.go,config.go,worker.go}`, `api/openapi.yaml` (`_mock/orders/{id}/approve-post`).

## Границы этапа

Входит:
- Модуль `marking`: `ord_organization`, `ord_contract`, `ord_platform`, `creative` с полями синхронизации 04 §6.1.
- Адаптер `integration/ord` (v8) и `ordfake`: контрагент, договор, площадка, креатив, статус.
- Строка платформы в `organization` (`is_platform = true`) из конфигурации.
- Регистрация по S-16 шаг 3: контрагент блогера, площадка канала, договор с блогером; видимость канала в каталоге только после регистрации площадки.
- Регистрация по S-17 шаг 3: контрагент платформы и рекламодателя, изначальный договор, креатив → erid → `MarkVersionReady` → P-02 (уже есть в `placement/app.VersionChanged`).
- Опрос `GET /status` по расписанию 05 §3.5, `status_timeout` через 24 ч.
- S-28: ошибка до erid — заказ в `marking`, ошибка в списке S-20; отклонение ЕРИР после erid — P-11 `erir_rejected` для неопубликованных размещений версии, оповещение по опубликованным.
- Повторная отправка контрагента при смене наименования или КПП (по хешу полезной нагрузки).
- S-20 до API администратора: служебная команда `maxim admin ord errors|retry`.
- Зависшие объекты (T-11, `sent` 24 ч), метрики, конфигурация.

Не входит (следующие этапы):
- Ежемесячная статистика, акты, `ord_statistics`, `ord_invoice`, `ord_invoice_item`, снимок просмотров на 1-е число, `ord_monthly_report` (S-31): этап «API администратора и отчётность ОРД». Риск: если этап выйдет после первой границы месяца с живыми публикациями, просмотры на конец месяца придётся брать из последнего `publication.views` до границы — решение R-19.
- Экран и операции администратора «Ошибки ОРД» в OpenAPI (`adm/ord-errors`), модерация поста через API (`approve`/`reject` версии администратором): этап «API администратора». Здесь одобрение — служебные методы и команда `maxim dev post-version approve`.
- Ежесуточный проверочный прогон против тестового контура (05 §3.8): этап «Юридическая и эксплуатационная готовность» (тестовое окружение). Здесь — только переключение адреса и токена конфигурацией.
- Решение по опубликованным размещениям с отклонённым креативом: ручное, этап администратора; здесь — только оповещение со списком.
- Уведомления пользователям: порт `Notifier` остаётся `NopNotifier`, новые коды шаблонов — R-17.

## Решения этапа, требующие подтверждения

Документы этих мест не определяют или противоречат друг другу. План реализует указанный вариант. После подтверждения задача 14 вносит его в документы. При отказе меняется только отмеченная задача.

| # | Вопрос | Решение в плане | Задача |
|---|---|---|---|
| R-1 | Идентификаторы в ОРД | В ОРД передаются наши UUID (подходят под шаблон `^[A-Za-z0-9\-_]+$`): контрагент — `organization.id`, договор — `ord_contract.id`, площадка — `channel.id`, креатив — `creative.id`. `ord_external_id` хранит отправленный идентификатор. Так повтор после неизвестного исхода (таймаут) — обычное обновление по тому же `id`, проверка «создан ли объект» (03 §11) не нужна; ключ идемпотентности ОРД не поддерживает | 2, 3 |
| R-2 | Состояния синхронизации и готовность зависимости | `pending` → (запрос принят) `sent` → (финальный успех) `synced`; финальная ошибка, отказ `4xx`/`422`, исчерпание повторов или `status_timeout` → `error`; `error` → `pending` только по «Повторить» (S-20) или при новой полезной нагрузке; `synced` → `pending` при новой полезной нагрузке (кроме креатива, R-8). Сырой последний статус ОРД — новое поле `ord_status`, время отправки — `sent_at`. Зависимость **готова**, когда `sync_status = synced` или `sent` с `ord_status` из `ORD success`, `ERIR sync success` (объект принят ОРД; ждать ЕРИР до 24 ч для договора нельзя — T-11 1 ч). Проверяется на тестовом контуре — метка `// Q-40` (новый вопрос: принимает ли ОРД договор на контрагента, ещё не подтверждённого ЕРИР) | 2, 6 |
| R-3 | Классификация статусов ОРД | `Request accepted by ORD` — в работе; `ORD success`, `ERIR sync success` — принято, опрос продолжается; `ERIR async success` — финальный успех; `ORD error`, `ORD rejected`, `ERIR sync error`, `ERIR async error` — финальная ошибка. Статус площадки и иных объектов без ЕРИР: финальный успех — `ORD success`, если `ERIR async success` для них не приходит (метка `// Q-40`). Статус объекта берётся из `object_statuses[]` по нашему `object_id`, иначе из общего `status`; `error_message[]` — в `sync_error` (до 2000 символов) | 2, 3 |
| R-4 | Порядок регистрации креатива (05 §3.2) | Задание `sync_ord_object` креатива проверяет цепочку: контрагент платформы → контрагент рекламодателя → изначальный договор. Нет записи — создаётся `pending` и ставится её `sync_ord_object` (в одной транзакции); запись не готова (R-2) — креатив ждёт: `job.Retry(…, ORD_DEPENDENCY_WAIT)` (30 с), без счёта попытки; запись в `error` — креатив не отправляется, DEBUG `ord dependency failed`, повтор раз в `ORD_DEPENDENCY_WAIT_FAILED` (10 мин) — после «Повторить» у зависимости креатив продолжит сам. Та же схема — для договора с блогером (платформа → блогер → площадка канала не нужна договору; площадка зависит только от контрагента блогера) | 6, 7 |
| R-5 | Платформа как контрагент | При запуске `worker` и `api` модуль `account` обеспечивает строку `organization` с `is_platform = true` из конфигурации: `ORD_PLATFORM_INN`, `ORD_PLATFORM_NAME`, `ORD_PLATFORM_LEGAL_TYPE` (по умолчанию `sole_proprietor`, D-08). Нет строки — вставка; есть с тем же ИНН — обновление наименования; ИНН занят обычной организацией или у строки платформы другой ИНН — ошибка запуска. Для ОРД: `isRr = false`, `isOrs = false`. В разработке и тестах с `ordfake` — значения по умолчанию `dev` | 5, 12 |
| R-6 | Сопоставление контрагента (05 §3.3) | `legal_entity` → `ul` (ИНН 10, `kpp` если есть), `sole_proprietor` → `ip`, `self_employed` → `fl` (ИНН 12). `name` — `organization.name` (≤ 255, обрезка не делается: длиннее — ошибка `name_too_long` до вызова). `isRr = true` у блогера (контрагент создаётся из канала), `false` у рекламодателя; организация, которая и рекламодатель, и блогер, — `isRr = true` (одна запись на организацию, уникальность `organization_id`). Хеш полезной нагрузки — SHA-256 канонического JSON запроса | 2, 6 |
| R-7 | Параметры договоров (Q-14, A-33) | Из конфигурации, а не констант. Изначальный: `ORD_INITIAL_CONTRACT_TYPE` (`intermediary-contract`), `ORD_INITIAL_SUBJECT_TYPE` (`mediation`), `ORD_INITIAL_ACTION_TYPE` (`conclude`), `isRegReport = true` (отчитывается платформа, A-33), `agentActingForPublisher = false`; клиент — рекламодатель, исполнитель — платформа. С блогером: `ORD_PUBLISHER_CONTRACT_TYPE` (`contract`), `ORD_PUBLISHER_SUBJECT_TYPE` (`distribution`), `ORD_PUBLISHER_ACTION_TYPE` (`distribution`), `isRegReport = true`; клиент — платформа, исполнитель — блогер. `agentActingForPublisher` отправляется только для `intermediary-contract`. `date` — дата по Москве первого принятия оферты организацией (`advertiser_offer` или `blogger_offer`, `legal_acceptance.accepted_at`); `number` — `ORD_CONTRACT_NUMBER_PREFIX` + номер версии оферты (пусто — не отправляется). Нет принятия — `error` `offer_not_accepted` без вызова. Проверки при запуске: значения из перечислений спецификации | 2, 6, 12 |
| R-8 | Креатив и erid (05 §3.4) | Запись `creative` (`pending`) и задание создаются в транзакции перехода версии в `approved` (через `VersionListener`, R-11). Отправка: `creativeType = creative`, `isSocial = false`, `isSocialQuota = false`, `coBranding = false`, `contractIds = [изначальный]`, `kktuCodes = [ORD_KKTU_CODE]` (A-42, Q-39), `form = text-graphic-block` при изображениях, иначе `text-block` (уточнение 05 §3.4: «всегда `text-graphic-block`» неверно для поста без изображений), `textData = [текст body без разметки]`, `mediaData` — `public_url` производных версии по порядку (`image`), `urls` — адреса ссылок из body (уже с UTM), `description` — «{название кампании}, заказ {номер}» (≤ 1000). Токен из `token` ответа или `object_statuses[].token`; нет токена — креатив `sent`, токен ждёт опрос. Получен токен — в одной транзакции: `creative.erid`, `post/app.MarkVersionReady` (итоговый текст, затем P-02 через существующий `VersionChanged`), INFO `erid received`. После записи erid креатив повторно не отправляется никогда (D-03): «Повторить» для креатива с erid только возобновляет опрос | 6, 7 |
| R-9 | Изображения ещё не в публичной области | Производная без `public_url` (задание `publish_media_derivative` не выполнено) — креатив ждёт, как неготовая зависимость (R-4). Это не ошибка ОРД | 7 |
| R-10 | Опрос статуса (05 §3.5) | Одно задание `poll_ord_status` на объект: `dedupe_key ord_status:{type}:{id}`, ставится в транзакции перехода в `sent`. Задержка следующего опроса: `ORD_POLL_FAST_INTERVAL` (1 мин) пока `now − sent_at < ORD_POLL_FAST_PERIOD` (10 мин), затем `ORD_POLL_SLOW_INTERVAL` (10 мин); при `now ≥ sent_at + ORD_STATUS_TIMEOUT` (24 ч) — `error` `status_timeout`, оповещение. Ошибка сети при опросе — следующий опрос по тому же расписанию, без отдельного счёта. Политика после простоя — выполнить немедленно, один раз | 2, 8 |
| R-11 | Несколько слушателей версии | `post/domain.VersionListener` один; `bootstrap/bridge.VersionListeners` вызывает по порядку `placement/app` и `marking/app` в той же транзакции. `marking` реагирует только на `To = approved`. Версия, ставшая `ready` в обход маркировки (`maxim dev post-version ready`, тестовые сборки), — задание креатива видит версию не в `approved` и завершает работу: DEBUG `creative skipped, version not approved`, `creative.sync_status = error` с `sync_error = version_not_approved` не ставится — запись удаляется в той же транзакции (не было отправки) | 7, 11 |
| R-12 | Отклонение ЕРИР после erid (S-28 шаг 2) | Финальная ошибка креатива с erid → `creative.erir_status = error`, `sync_status = error` и в той же транзакции порт `marking/domain.CreativeListener.CreativeRejected(ctx, versionID)` (реализация `placement/app`): для каждого размещения версии в `awaiting_blogger`, `awaiting_schedule`, `scheduled` — P-11 с актором `system`, причиной `erir_rejected`, версия не меняется (`ReturnToCreated` получает ту же версию); размещения в `created` с этой версией остаются. Размещения `published` и дальше — ERROR `creative rejected after publication` и оповещение `ord_creative_rejected:{creative_id}` со списком. Новая отправка блогерам — только новой версией (S-29). Публикация уже запущенного задания отменяется P-11 (отмена `publish:{id}`). Порядок блокировок модуля размещений сохраняется (каналы → заказ → размещения) | 8, 9 |
| R-13 | Регистрация по одобрению канала (S-16) | `channel/app.Approve` в своей транзакции вызывает порт `channel/domain.ChannelApprovalListener.ChannelApproved(ctx, ChannelRef{ID, OrganizationID, Title, Link})` (реализация `marking/app`): создаётся или обновляется `ord_platform` (`pending`, если хеш изменился), ставятся задания; контрагент блогера и договор с блогером создаются как зависимости. Площадка — `type = ORD_PLATFORM_TYPE` (`site`, A-32, Q-13), `isOwned = false`, `name` — название канала (обрезка до 100 символов по рунам), `url` — публичная ссылка канала, `organizationId` — организация блогера. Ошибка ОРД одобрение не отменяет | 9, 10 |
| R-14 | Канал в каталоге до регистрации площадки (S-16, отклонение) | Новое поле `channel.ord_platform_ready_at timestamptz`; каталог (`channel/postgres/catalog.go`) и добавление в корзину показывают канал только при непустом значении. Ставит его модуль `channel` по вызову `channel/app.SetOrdPlatformReady(ctx, channelID, at)` из `marking`, когда площадка готова (R-2). Миграция заполняет поле `now()` для каналов, уже `active` (данные разработки). Кабинет блогера поле не показывает (спецификация не меняется); при `ORD_ENABLED=false` (R-18) поле ставится при одобрении | 1, 10 |
| R-15 | Смена реквизитов (S-20 «исправить данные») | ИНН и тип лица неизменяемы (`account/domain`). `account/app.UpdateOrganization` после сохранения вызывает порт `account/domain.OrganizationListener.OrganizationChanged(ctx, orgID)`; `marking` пересчитывает хеш контрагента, при отличии — `pending` и `sync_ord_object` (в том числе из `error`). Наименование платформы — по R-5 при запуске | 5, 9 |
| R-16 | Что видно в `outbound_request` (07 §5.4, 06 §8) | Запрос пишется для S-20, но с маскированием: `inn` → `77******1234`, `textData` → длина текста, `Authorization` скрыт. Ответ ОРД пишется целиком (в нём нет персональных данных, кроме наших идентификаторов). Список полей — `Options.Redact` клиента ОРД и маскирование ИНН в адаптере | 3 |
| R-17 | Оповещения и уведомления | Оповещения администратору: ERROR `ord_sync_failed:{type}:{id}` (финальная ошибка или отказ), ERROR `ord_creative_rejected:{id}`, WARN `post_version_erid_overdue:{version_id}` (T-11), WARN `ord_object_sent_overdue:{type}:{id}` (зависший `sent` 24 ч). Уведомления пользователям — порт `Notifier` (`NopNotifier`): `placement.returned` блогеру при P-11 (уже есть), новый `order.erir_rejected` рекламодателю | 8, 9, 10 |
| R-18 | Отключение ОРД | `ORD_ENABLED` (по умолчанию `true`; в `production` выключить нельзя — ошибка запуска). При `false` (локальная разработка без `ordfake`, тесты других модулей) слушатели маркировки не подключаются, одобрение канала сразу ставит `ord_platform_ready_at`, а `maxim dev post-version ready --erid` остаётся единственным путём к `ready`. Режим адаптера: `ORD_OAUTH_TOKEN` пуст в `development`/`test` — `ordfake` (как MAX) | 11, 12 |
| R-19 | Снимок просмотров на 1-е число | Не входит в этап (S-31). До этапа отчётности для границы месяца используется последний `publication.views` с `views_checked_at` до 00:00 МСК 1-го числа — сверка обновляет его раз в час (`PUBLICATION_VIEWS_REFRESH`). Записывается в 05 §3.6 как допустимое приближение до этапа отчётности | 14 |
| R-20 | Ограничитель | Одна корзина `ord:global`, `ORD_RATE` (5 rps, Q-15), без приоритетов: все вызовы ОРД — фоновые. Предохранитель и повторы — общие `outbound` (05 §1); `429` — задержка из ответа | 3, 12 |

## Commit Plan
- **Commit 1** (после задач 1–2): `feat(marking): ord objects schema and sync rules`
- **Commit 2** (после задач 3–5): `feat(ord): yandex ord v8 adapter, marking storage and module ports`
- **Commit 3** (после задач 6–8): `feat(marking): creative registration, erid and status polling`
- **Commit 4** (после задач 9–11): `feat(marking): channel platforms, erir rejection, admin retry`
- **Commit 5** (после задачи 12): `feat(marking): config and wiring`
- **Commit 6** (после задач 13–14): `test(marking): ord e2e, docs sync`

## Tasks

### Phase 1: Данные и правила

- [x] **Task 1: Миграция объектов ОРД (R-1, R-2, R-14)**
  - `migrations/00009_marking.sql` по 04 §6:
    - Общие поля синхронизации (04 §6.1) и сверх них `ord_status text`, `sent_at timestamptz`, `created_at`, `updated_at`; `sync_status check in (pending, sent, synced, error)`, `sync_attempts >= 0`; `check`: `sent_at` задан при `sent`/`synced`.
    - `ord_organization` (`organization_id` уникально → `organization`, `is_rr boolean`).
    - `ord_contract` (`kind check in (initial, publisher)`, `client_organization_id`, `contractor_organization_id` → `organization`, `number`, `signed_date date`; уникальна тройка `(kind, client_organization_id, contractor_organization_id)`).
    - `ord_platform` (`channel_id` уникально → `channel`, `organization_id`, `name`, `url`).
    - `creative` (`post_version_id` уникально → `post_version`, `ord_contract_id` → `ord_contract`, `erid` уникально при непустом, `erir_status check in (pending, success, error)`, `erir_error`, `erid_received_at`).
    - Индексы `(sync_status)` по каждой таблице для списка S-20 и метрики; `(sync_status, sent_at) WHERE sync_status = 'sent'` для зависших.
    - `channel.ord_platform_ready_at timestamptz`; заполнение `now()` для `status = 'active'`.
  - Права роли: через `ALTER DEFAULT PRIVILEGES` из `00002`; удаление разрешено только у `creative` (R-11), у остальных — `REVOKE DELETE`.
  - Журнал: не применимо (INFO `migrations applied` пишет `migrate`).
  - Тесты: `up` → `down` → `up`. Второй креатив версии и второй контрагент организации не вставляются; два одинаковых erid не вставляются; `sent` без `sent_at` не проходит; роль приложения не удаляет `ord_contract`.
  - Файлы: `migrations/00009_marking.sql`.

- [x] **Task 2: `marking/domain` — объекты, синхронизация, полезная нагрузка (R-1 … R-8, R-10, R-13)** (depends on 1)
  - `internal/marking/domain/sync.go`: `SyncStatus`, `SyncState{Status, ExternalID, RequestID, OrdStatus, Error, Attempts, SentAt, SyncedAt, PayloadHash}`; методы `MarkSent(reqID, ordStatus, now)`, `ApplyStatus(ordStatus, errs, now) (Outcome)`, `Fail(reason, now)`, `Retry(now)` (только из `error`), `NeedsResend(hash)`; `Ready()` по R-2.
  - `internal/marking/domain/status.go`: `ClassifyOrdStatus(s) (Progress|Accepted|FinalSuccess|FinalError)` по R-3 с признаком «объект без ЕРИР» для площадки; метка `// Q-40`.
  - `internal/marking/domain/poll.go`: `NextPoll(sentAt, now, rules) (delay, timeout bool)` по R-10.
  - `internal/marking/domain/{organization.go,contract.go,platform.go,creative.go}`: сущности 04 §6; построители запросов `OrganizationRequest(org, isRr)` (R-6), `ContractRequest(kind, client, contractor, signedDate, params)` (R-7), `PlatformRequest(channel)` (R-13), `CreativeRequest(content, contractID, kktu)` (R-8); `PayloadHash(req)` — SHA-256 канонического JSON.
  - `internal/marking/domain/chain.go`: `CreativeDependencies`, `PublisherDependencies` — какие объекты нужны и в каком порядке (R-4); решение `DependencyVerdict(states) (missing []Ref, wait bool, failed bool)`.
  - `internal/marking/domain/ports.go`: `OrdAPI` (`UpsertOrganization`, `UpsertContract`, `UpsertPlatform`, `UpsertCreative`, `GetStatus` → `Receipt{RequestID, Status, Token, ObjectStatuses, Errors}`), `Organizations` (реквизиты и дата принятия оферты, R-7), `CreativeContent` (версия: статус, body-текст, ссылки, публичные адреса, название кампании, номер заказа, организация), `VersionMarker` (`MarkVersionReady`), `ChannelPlatformReady` (R-14), `CreativeListener` (R-12), репозитории `OrdOrganizations`, `OrdContracts`, `OrdPlatforms`, `Creatives`; `Config` с `WithDefaults` (параметры R-7, R-8, R-10, R-13). Ошибки `ErrNotFound`, `ErrNameTooLong`, `ErrOfferNotAccepted`, `ErrPermanent`.
  - Журнал: не применимо (`domain` не пишет журнал).
  - Тесты (табличные): каждая строка R-3; `NextPoll` на 9:59, 10:00 и 24:00 после отправки; `Retry` из `synced` — отказ; `Ready` для `sent` + `ORD success` — да, `sent` + `Request accepted by ORD` — нет; три типа лица → `ul`/`ip`/`fl`, `kpp` только у `ul`; `agentActingForPublisher` только у `intermediary-contract`; дата договора — сутки по Москве (принятие в 23:30 UTC → следующая дата); креатив без изображений → `text-block`, с изображениями → `text-graphic-block` и `mediaData` по порядку; хеш не меняется от порядка ключей и меняется от наименования; название канала 150 символов → 100 рун; цепочка креатива при отсутствующих, неготовых и ошибочных зависимостях.
  - Файлы: `internal/marking/domain/{sync.go,status.go,poll.go,organization.go,contract.go,platform.go,creative.go,chain.go,ports.go,errors.go,*_test.go}`.
<!-- Commit checkpoint: tasks 1-2 -->

### Phase 2: Адаптер, хранение, порты других модулей

- [x] **Task 3: `integration/ord` — клиент API v8 и `ordfake` (R-1, R-3, R-16, R-20)**
  - `internal/integration/ord/client.go`: `New(out *outbound.Client, limiter, Options{BaseURL, APIVersion "v8", Token, Rate}, log)`; адрес `{BaseURL}/api/{APIVersion}/…` (05 §3.8: версия — конфигурация, все обращения в пакете); `Authorization: Bearer` в `Redact`; корзина `ord:global` (R-20).
  - `internal/integration/ord/{organization.go,contract.go,platforms.go,creative.go,status.go}`: методы порта `OrdAPI` по полям спецификации (Research Context); разбор `request_id`, `status`, `token`, `object_statuses[]`, `error_message[]`.
  - `internal/integration/ord/errors.go`: `422` → `job.Permanent` с текстом из `detail[]` (`loc: msg`); прочие `4xx` кроме `429` — `job.Permanent`; `5xx`, `429`, сеть — повтор через `outbound` (05 §1).
  - `internal/integration/ord/redact.go`: маскирование `inn` и замена `textData` длиной в записи `outbound_request` (R-16).
  - `internal/integration/ord/ordfake`: память по нашим `id`; `Upsert*` возвращают `request_id` и `Request accepted by ORD`, креатив — `token` `fake-erid-{n}`; `GetStatus` продвигает статус по шагам до `ERIR async success`. Управление для тестов: `FailNext(op, status, body)`, `RejectObject(id, ordStatus, msg)` (финальная ошибка при опросе), `TokenOnlyInStatus()`, `HoldStatus(id)` (статус не меняется), `Calls(op)`, `Objects()`.
  - Журнал: DEBUG вызовы (пишет `outbound`), INFO `ord object submitted` (`ord_object_type`, `ord_object_id`, `reqid`, `ord_status`), DEBUG `ord status read` (`reqid`, `ord_status`, `errors`), WARN повтор и ERROR отказ — `outbound`. ИНН, наименование, текст и токен OAuth — не пишутся.
  - Тесты (`httptest`, образцы в `testdata` по спецификации): каждый метод — тело запроса совпадает с образцом; `422` с `detail` → постоянная ошибка с понятным текстом; `500` дважды, затем успех; `429` с задержкой; `token` в `object_statuses`; маскированный ИНН и отсутствие `textData` и токена в `outbound_request`; адрес содержит версию из настроек.
  - Файлы: `internal/integration/ord/{client.go,organization.go,contract.go,platforms.go,creative.go,status.go,errors.go,redact.go,*_test.go}`, `internal/integration/ord/testdata/*`, `internal/integration/ord/ordfake/{fake.go,fake_test.go}`.

- [x] **Task 4: `marking/postgres` — репозитории (R-2, R-4, R-10)** (depends on 1, 2)
  - `sqlc.yaml`: запись модуля `marking`.
  - Запросы `queries/{organizations,contracts,platforms,creatives,errors}.sql`: вставка и обновление с полями синхронизации; получение с `FOR UPDATE` по нашему ключу (организация, тройка договора, канал, версия поста); список ошибок по всем таблицам с курсором `(updated_at, id)` для S-20; число объектов по `(type, sync_status)` для метрики; зависшие `sent` старше порога; креативы без erid старше порога (T-11); удаление креатива без отправки (R-11).
  - Вставка зависимости при гонке двух заданий — `ON CONFLICT DO NOTHING` и повторное чтение.
  - Журнал: DEBUG `ord object saved` (`ord_object_type`, `ord_object_id`, `sync_status`).
  - Тесты (PostgreSQL): два одновременных создания контрагента — одна запись; список ошибок обходится курсором по всем видам ровно по разу; зависшие по порогу 24 ч; счётчики по состояниям.
  - Файлы: `sqlc.yaml`, `internal/marking/postgres/{store.go,organizations.go,contracts.go,platforms.go,creatives.go,errors.go,*_test.go}`, `internal/marking/postgres/queries/*.sql`, `internal/marking/postgres/sqlc/*`.

- [x] **Task 5: Порты в `account`, `channel`, `post` (R-5, R-7, R-8, R-11, R-13 … R-15)** (depends on 2)
  - `account`:
    - `account/app.EnsurePlatformOrganization(ctx, PlatformOrg{INN, Name, LegalType})` по R-5 (транзакция, запись `audit_log` с актором `system`).
    - Чтение для маркировки (переходник `bridge.MarkingOrganizations` над `account/postgres`): тип лица, ИНН, наименование, КПП, признак платформы, первая дата принятия оферты вида.
    - `account/domain.OrganizationListener` (пустая по умолчанию, `SetOrganizationListener`), вызов из `UpdateOrganization` после сохранения при изменении `name` или `kpp` (R-15).
  - `channel`:
    - `channel/domain.ChannelApprovalListener` (пустая по умолчанию; задаётся `SetPlacementPorts` или отдельным `SetMarkingPorts`), вызов в транзакции `Approve` (R-13).
    - `channel/app.SetOrdPlatformReady(ctx, channelID, at)`; поле `OrdPlatformReadyAt` в `Channel`; каталог и проверка канала при добавлении в корзину (`CartLookup`/`ChannelOffer` для корзины) учитывают поле (R-14). Ссылка на канал — существующее `link`.
  - `post`:
    - `post/domain.PlainText(body)` — текст body без разметки (сущности HTML раскрываются), `post/domain.LinkURLs(body)` — адреса ссылок по порядку без повторов.
    - `post/app.CreativeContent(ctx, versionID)` — статус версии, `PlainText`, `LinkURLs`, публичные адреса производных (`GetVersion`), название кампании, номер заказа, организация.
    - `bootstrap/bridge.VersionListeners` (R-11).
  - Журнал: INFO `platform organization ensured` (`organization_id`, `created`), DEBUG `organization change passed to marking` (`organization_id`), DEBUG `channel approval passed to marking` (`channel_id`), INFO `channel ord platform ready` (`channel_id`). ИНН — маскированным.
  - Тесты: платформа создаётся, повторный запуск ничего не меняет, другой ИНН — ошибка; смена телефона слушателя не вызывает, смена наименования — вызывает; одобрение канала вызывает слушателя, ошибка слушателя откатывает одобрение; канал без `ord_platform_ready_at` не виден в каталоге и не добавляется в корзину (`409`), с ним — виден; `PlainText` и `LinkURLs` на образцах сериализатора (жирный, ссылка, цитата, `&amp;`); `CreativeContent` возвращает пустой адрес для неопубликованной производной; оба слушателя версии вызываются по порядку, ошибка второго откатывает переход.
  - Файлы: `internal/account/{app/platform.go,app/organizations.go,domain/ports.go,app/platform_test.go}`, `internal/channel/{domain/ports.go,domain/channel.go,app/moderation.go,app/marking_ports.go,postgres/channels.go,postgres/catalog.go,postgres/queries/*.sql,postgres/sqlc/*,app/*_test.go}`, `internal/post/{domain/plain_text.go,domain/plain_text_test.go,app/creative.go,app/creative_test.go}`, `internal/bootstrap/bridge/{versions.go,marking.go}`.
<!-- Commit checkpoint: tasks 3-5 -->

### Phase 3: Сценарии

- [x] **Task 6: Синхронизация контрагентов и договоров — `sync_ord_object` (R-2, R-4, R-6, R-7, R-17)** (depends on 3, 4, 5)
  - `internal/marking/app/sync.go`: `Sync(ctx, ref ObjectRef{Type, ID}, jobRef)` — общий ход в три шага (I-13): шаг 1 — транзакция: блокировка записи; `synced` с тем же хешем или `sent` — DEBUG `ord object up to date` и выход; зависимости по R-4 (создать недостающие и поставить их задания, ждать неготовые); сборка запроса и хеша; ошибки сборки (`ErrNameTooLong`, `ErrOfferNotAccepted`) → `error` без вызова. Шаг 2 — вне транзакции: вызов `OrdAPI`. Шаг 3 — транзакция: `MarkSent`, `poll_ord_status` (R-10); постоянная ошибка → `error`, оповещение `ord_sync_failed`; повторяемая — ошибка задания (повтор планировщика, `max_attempts` 10, затем `error` по `OnFailed`).
  - `internal/marking/app/organizations.go`, `contracts.go`: ветки `Sync` для контрагента (R-6, `isRr` пересчитывается: стал блогером — новый хеш) и договора (R-7).
  - `OrganizationChanged(ctx, orgID)` (R-15): пересчёт хеша, `pending` и задание.
  - `EnsurePublisherChain(ctx, blogerOrgID)`, `EnsureInitialChain(ctx, advertiserOrgID)` — создание записей `pending` и заданий в транзакции вызывающего.
  - Журнал: DEBUG `ord object queued` (`ord_object_type`, `ord_object_id`, `reason`: `new`, `changed`, `retry`), DEBUG `ord dependency not ready` (`dependency_type`, `dependency_id`, `ord_status`), DEBUG `ord dependency failed`, INFO `ord object sent` (`reqid`, `ord_status`, `attempt`), ERROR `ord object sync failed` (`reason`, `error`), DEBUG `ord object up to date`.
  - Тесты (PostgreSQL, `clock.Manual`, `ordfake`): контрагент рекламодателя `ip` — запрос с `type = ip`, `isRr = false`, запись `sent`, опрос поставлен; повтор задания после `sent` — второго вызова нет; изначальный договор без контрагентов — создаются оба, договор ждёт, после `ORD success` у обоих — отправлен с датой принятия оферты; `422` → `error`, оповещение, без повторов; `5xx` 10 раз → `error`; оферта не принята → `error` `offer_not_accepted` без вызова; смена наименования после `synced` → повторная отправка с тем же `id`; смена телефона — ничего; таймаут (неизвестный исход) — повтор с тем же `id`, в `ordfake` один объект.
  - Файлы: `internal/marking/app/{service.go,sync.go,organizations.go,contracts.go,sync_test.go,organizations_test.go,contracts_test.go}`, `internal/marking/markingtest/*` (фикстуры: организации с принятой офертой, `ordfake`).

- [x] **Task 7: Креатив и erid — S-17 шаг 3, S-28 шаг 1 (R-8, R-9, R-11)** (depends on 6)
  - `internal/marking/app/creatives.go`:
    - `VersionChanged(ctx, change)` (реализация `post/domain.VersionListener`, R-11): `To = approved` → запись `creative` `pending`, `EnsureInitialChain`, задание `sync_ord_object` креатива.
    - Ветка `Sync` для креатива: версия не `approved` — R-11; зависимости R-4; изображения без публичного адреса — ожидание (R-9); запрос R-8; токен в ответе — в шаге 3 `creative.erid`, `erid_received_at`, `VersionMarker.MarkVersionReady` в той же транзакции (P-02 выполняет `placement`), метрика ожидания erid; токена нет — `sent`, токен ждёт опрос (задача 8). Ошибка `MarkVersionReady` (`final_text_too_long`) — ERROR и оповещение, креатив остаётся с erid, версия в `approved` (решает администратор).
  - Журнал: INFO `creative registration started` (`creative_id`, `post_version_id`, `order_id`, `images`, `urls`), INFO `erid received` (`creative_id`, `post_version_id`, `wait_ms`) — erid можно писать (это не секрет), DEBUG `creative waits for media` (`pending_images`), DEBUG `creative skipped, version not approved` (`status`), ERROR `post version ready failed after erid`.
  - Тесты (PostgreSQL, `clock.Manual`, `ordfake`, `storagefake`): одобрение версии → креатив `pending`, после `worker.RunOnce` по цепочке — контрагенты, договор, креатив с `kktuCodes` из конфигурации и ссылками с UTM; erid записан, версия `ready`, итоговый текст содержит erid, размещения `awaiting_blogger` (P-02), заказ `in_progress`; токен только в статусе — версия `ready` после опроса; изображение без публичного адреса — ждёт, после публикации производной — отправлен; повтор задания после erid — второго креатива в `ordfake` нет; версия стала `ready` через `MarkVersionReady` напрямую — запись креатива удалена, вызова нет; ошибка договора — заказ остаётся `marking`, размещения `created`.
  - Файлы: `internal/marking/app/{creatives.go,creatives_test.go}`.

- [x] **Task 8: Опрос статусов — `poll_ord_status` (R-3, R-10, R-12, R-17)** (depends on 7)
  - `internal/marking/app/poll.go`: `Poll(ctx, ref, jobRef)` — объект не `sent` (уже `synced` или `error`) — DEBUG и выход; вне транзакции `GetStatus(reqid)`; в транзакции `ApplyStatus`: в работе или принято — `job.Retry(…, NextPoll)`; финальный успех — `synced` (креатив — `erir_status = success`); финальная ошибка — `error`, оповещение `ord_sync_failed`, для креатива с erid — R-12 (`CreativeListener.CreativeRejected`, оповещение `ord_creative_rejected`); токен креатива впервые в статусе — как в задаче 7; таймаут 24 ч — `error` `status_timeout`. Готовность площадки (R-2) → `ChannelPlatformReady.SetOrdPlatformReady` (задача 10).
  - Журнал: DEBUG `ord status polled` (`reqid`, `ord_status`, `next_poll_in`), INFO `ord object confirmed` (`ord_object_type`, `ord_object_id`, `ord_status`, `elapsed_ms`), ERROR `ord object rejected` (`ord_status`, `errors_count`) — тексты ошибок ОРД только на DEBUG, ERROR `ord status timeout`.
  - Тесты (PostgreSQL, `clock.Manual`, `ordfake`): расписание опроса — 10 опросов раз в минуту, затем раз в 10 мин; `ERIR async success` → `synced`; `ERIR async error` у контрагента → `error`, оповещение, зависимый креатив не отправляется; 24 ч без финального статуса → `status_timeout`; ошибка сети при опросе — следующий опрос по расписанию; опрос после простоя выполняется один раз.
  - Файлы: `internal/marking/app/{poll.go,poll_test.go}`.
<!-- Commit checkpoint: tasks 6-8 -->

- [x] **Task 9: Отклонение ЕРИР — S-28 шаг 2, P-11 `erir_rejected` (R-12, R-17)** (depends on 8)
  - `internal/placement/app/creative.go`: `CreativeRejected(ctx, versionID)` (реализация `marking/domain.CreativeListener`): заказ по версии; блокировки каналы → заказ → размещения; для `awaiting_blogger`, `awaiting_schedule`, `scheduled` с этой версией — `ReturnToCreated(ActorSystem, ReasonErirRejected, versionID, …)` через общий `apply`; опубликованные и дальше — список в результате. Уведомление `order.erir_rejected` рекламодателю (R-17). Пересчёт состояния заказа.
  - `internal/placement/domain`: шаблон `NoticeErirRejected`; проверка, что P-11 с причиной `erir_rejected` допустим для актора `system` (уже в таблице — тест).
  - Журнал: INFO переходы — общий `apply`; INFO `placements returned on erir rejection` (`order_id`, `post_version_id`, `returned`, `published`); ERROR `creative rejected after publication` (`placement_ids`).
  - Тесты (PostgreSQL, `clock.Manual`): размещения в `awaiting_blogger`, `scheduled` и `published` одной версии → первые два `created` с причиной `erir_rejected`, задания публикации и таймеры отменены, удержание сохранено, `published` не тронут и попал в список; размещение на другой (новой) версии не затронуто; повтор вызова — без новых записей истории.
  - Файлы: `internal/placement/app/{creative.go,creative_test.go}`, `internal/placement/domain/{actions.go,status.go,actions_test.go}`.

- [x] **Task 10: Площадки каналов — S-16 шаг 3 (R-13, R-14)** (depends on 6, 8)
  - `internal/marking/app/platforms.go`:
    - `ChannelApproved(ctx, ref)` (реализация `channel/domain.ChannelApprovalListener`): `ord_platform` (новая или с изменённым хешем) `pending`, `EnsurePublisherChain` и задания; площадка уже готова с тем же хешем — сразу `SetOrdPlatformReady`.
    - Ветка `Sync` для площадки (зависимость — контрагент блогера, R-4) и договора с блогером.
    - Готовность площадки (из `Sync` при `ORD success` в ответе или из `Poll`) → `SetOrdPlatformReady` в той же транзакции.
  - Журнал: INFO `ord platform queued` (`channel_id`, `reason`), INFO `channel visible after ord platform` (`channel_id`, `wait_ms`), ERROR — через `Sync`.
  - Тесты (PostgreSQL, `clock.Manual`, `ordfake`): одобрение канала → контрагент блогера `isRr = true`, площадка `site` с ссылкой и названием, договор с блогером; до готовности площадки канал не виден в каталоге, после — виден; ошибка площадки — канал `active`, но не в каталоге, оповещение; организация — рекламодатель и блогер одновременно — один контрагент с `isRr = true` (повторная отправка после смены признака); повторное одобрение после повторной модерации с тем же названием — без вызова.
  - Файлы: `internal/marking/app/{platforms.go,platforms_test.go}`.

- [x] **Task 11: S-20 до API администратора, зависшие, метрики (R-2, R-17)** (depends on 8)
  - `internal/marking/app/errors.go`: `ListErrors(ctx, cursor, limit)` (объект, `sync_error`, `ord_status`, попытки, ссылка на последний `outbound_request`), `Retry(ctx, ref, adminID, reason)` — `error` → `pending` и задание; креатив с erid — только возобновление опроса (R-8); запись `audit_log` с актором `admin` на уровне WARN (S-19, «каждая операция требует основания»).
  - `internal/bootstrap/admin.go`: `maxim admin ord errors [--limit N]` (таблица в stdout: тип, идентификатор, статус ОРД, ошибка, попытки) и `maxim admin ord retry --object <type>:<id> --reason <текст> --email <администратор>`.
  - `internal/marking/app/stuck.go`: проверки для `platform/stuck` — креатив без erid дольше `ORD_ERID_WAIT` (1 ч, T-11, оповещение WARN `post_version_erid_overdue`) и объект в `sent` дольше `STUCK_ORD_SENT_AGE` (24 ч).
  - `internal/marking/postgres/metrics.go`: `maxim_ord_objects{type,sync_status}` (сборщик по запросу, 07 §5.5 «число объектов ОРД в `error`»), гистограмма `maxim_ord_erid_wait_seconds` (от `approved` до erid).
  - Журнал: WARN `ord object retry requested` (`ord_object_type`, `ord_object_id`, `admin_id`), WARN `post version erid overdue` (`post_version_id`, `since`) — через `stuck`.
  - Тесты: `Retry` из `synced` — `409`; из `error` — задание поставлено, журнал действий записан; команда `admin ord errors` выводит маскированные данные; зависший креатив через 61 мин найден, через 59 — нет; метрика считает `error` по видам (`testutil`).
  - Файлы: `internal/marking/app/{errors.go,stuck.go,errors_test.go,stuck_test.go}`, `internal/marking/postgres/metrics.go`, `internal/bootstrap/{admin.go,admin_test.go}`.
<!-- Commit checkpoint: tasks 9-11 -->

### Phase 4: Сборка

- [x] **Task 12: Конфигурация, задания и сборка (R-5, R-7, R-10, R-11, R-13, R-18, R-20)** (depends on 9, 10, 11)
  - `internal/bootstrap/config.go`: `ORD_ENABLED` (true), `ORD_API_URL` (`https://ord-prestable.yandex.net` в `development`/`test`, обязателен в `production`), `ORD_API_VERSION` (`v8`), `ORD_OAUTH_TOKEN` (секрет; пуст в `development`/`test` → `ordfake`), `ORD_RATE` (5, Q-15), `ORD_KKTU_CODE` (обязателен при настоящем адаптере, проверка шаблона, A-42), `ORD_PLATFORM_INN`, `ORD_PLATFORM_NAME`, `ORD_PLATFORM_LEGAL_TYPE`, `ORD_PLATFORM_TYPE` (`site`, A-32), параметры договоров R-7 и `ORD_CONTRACT_NUMBER_PREFIX`, `ORD_POLL_FAST_INTERVAL` (1m), `ORD_POLL_FAST_PERIOD` (10m), `ORD_POLL_SLOW_INTERVAL` (10m), `ORD_STATUS_TIMEOUT` (24h), `ORD_DEPENDENCY_WAIT` (30s), `ORD_DEPENDENCY_WAIT_FAILED` (10m), `ORD_ERID_WAIT` (1h, T-11), `STUCK_ORD_SENT_AGE` (24h). Проверки при запуске: перечисления спецификации, ИНН платформы по типу лица, `ORD_ENABLED=false` в `production` — ошибка, интервалы опроса меньше таймаута.
  - `internal/bootstrap/integration.go`: `connectORD` — `ordfake` или `ord.New` через `outbound` (`SystemYandexORD`) и ограничитель.
  - `internal/bootstrap/marking.go` (новый): сборка `marking/app`, репозиториев, переходников; `EnsurePlatformOrganization` при запуске `api` и `worker`; подстановка в `post` (`bridge.VersionListeners`), `channel` (`ChannelApprovalListener`), `account` (`OrganizationListener`), `placement` как `CreativeListener`; проверки `stuck`; метрики.
  - `internal/marking/jobs/jobs.go`: `sync_ord_object` (`max_attempts` 10, `OnFailed` → `error` и оповещение, `ObjectRef` `ord:{type}:{id}`), `poll_ord_status` (без предела, задержку задаёт обработчик, после простоя — один запуск).
  - `internal/bootstrap/worker.go`: регистрация видов.
  - `internal/bootstrap/dev.go`: `post-version approve` — при `ORD_ENABLED` дальше работает маркировка (erid выдаёт `ordfake` или тестовый контур); `post-version ready --erid` — только при `ORD_ENABLED=false`, иначе ошибка с подсказкой.
  - `.env.example`, `compose.dev.yml`: переменные; `ORD_OAUTH_TOKEN` пуст (`ordfake`). Заметка для `/aif-docs`: в разработке `maxim dev post-version --order N approve` → через несколько секунд `worker` выдаёт erid и отправляет размещения блогерам; для тестового контура — `ORD_OAUTH_TOKEN` и `ORD_KKTU_CODE`.
  - Журнал: конфигурация — маскированной, как сейчас (`ORD_OAUTH_TOKEN` скрыт, ИНН платформы маскирован); INFO `ord adapter connected` (`mode`: `fake`/`real`, `base_url`, `api_version`).
  - Тесты: проверки конфигурации на границах; `wiring_test.go`: виды `sync_ord_object`, `poll_ord_status` зарегистрированы с нужными политиками; у `post` два слушателя версии; `channel` получил слушателя одобрения; при `ORD_ENABLED=false` слушатели не подключены и одобрение канала сразу делает его видимым.
  - Файлы: `internal/bootstrap/{config.go,config_test.go,integration.go,marking.go,modules.go,worker.go,dev.go,wiring_test.go}`, `internal/marking/jobs/jobs.go`, `.env.example`, `compose.dev.yml`.
<!-- Commit checkpoint: task 12 -->

### Phase 5: Проверка и документы

- [x] **Task 13: Сквозные тесты маркировки** (depends on 12)
  - `internal/bootstrap/marking_e2e_test.go` на собранных `api` и `worker` (PostgreSQL, `clock.Manual`, `ordfake`, `maxfake`, `storagefake`, `mailfake`):
    - S-16: блогер подключает канал, администратор одобряет через API (`approveChannel`) → в `ordfake` контрагент блогера, площадка, договор с блогером; канал появляется в каталоге только после готовности площадки.
    - S-17: рекламодатель оформляет заказ на этот канал → `dev post-version approve` (служебный метод) → заказ `marking` → `worker.RunOnce`: контрагенты платформы и рекламодателя, изначальный договор, креатив; erid в версии и в итоговом тексте; заказ `in_progress`, размещение `awaiting_blogger`; блогер видит пост с erid. Опрос доводит все объекты до `synced`.
    - Сквозной путь с автопостингом: блогер «Опубликовать сейчас» → в `maxfake` пост с erid из ОРД (I-01).
    - S-28 шаг 1: `ordfake.FailNext(contract, 422)` → заказ стоит в `marking`, ошибка в `admin ord errors`, оповещение; через 1 ч — оповещение T-11; исправление (`ordfake` снова отвечает) и `admin ord retry` → erid, P-02.
    - S-28 шаг 2: после erid `ordfake.RejectObject(creative, ERIR async error)` → размещение из `scheduled` в `created` (`erir_rejected`), задание публикации отменено, оповещение; новая версия (S-29) → новый креатив и новый erid.
    - S-29: правка поста после отклонения модерации → у новой версии свой креатив, старый креатив не используется.
    - R-15: рекламодатель меняет наименование → повторная отправка контрагента с тем же `id`.
  - Проверка журнала сборщиком записей: обязательные события 07 §5.3 (внешняя операция выполнена, повтор, отказ, задание запущено и завершено, T-11) присутствуют; ИНН только маскированным, наименования организаций, текст поста и OAuth-токен не встречаются. Проверка `outbound_request`: ИНН маскирован, `textData` нет, `Authorization` нет.
  - Файлы: `internal/bootstrap/marking_e2e_test.go`, при необходимости `internal/bootstrap/harness_test.go`.

- [x] **Task 14: Синхронизация документов с решениями этапа** (depends on 12)
  - После подтверждения R-1 … R-20 внести их в документы:
    - 02 S-16 — порядок регистрации (R-13), видимость в каталоге (R-14); S-17 шаг 3 — креатив по одобрению, ожидание изображений (R-8, R-9); S-20 — «Повторить», команда до API администратора, повторная отправка при правке реквизитов (R-15); S-28 — P-11 без смены версии, размещения `created` остаются (R-12).
    - 03 §1.1 — версия в `ready` в обход маркировки только при `ORD_ENABLED=false` (R-18); §4 — P-11 с причиной `erir_rejected` (версия не меняется); §5 — T-11 через проверку зависших.
    - 04 §6 — поля `ord_status`, `sent_at`, `erid_received_at`, `channel.ord_platform_ready_at`; идентификаторы R-1; удаление креатива без отправки (R-11).
    - 05 §3.2 — готовность зависимости (R-2), порядок цепочек (R-4); §3.3 — поля по спецификации v8 (`isSocial`, `isSocialQuota`, `coBranding`, `creativeType`, `platforms[]`, `agentActingForPublisher` только у посреднического), параметры договоров (R-7), `isRr` у организации-блогера (R-6); §3.4 — `form` по наличию изображений (R-8); §3.5 — классификация статусов (R-3), расписание (R-10); §3.6 — R-19; §3.9 — Q-40; §1 — маскирование в `outbound_request` (R-16).
    - 07 §2 — политики `sync_ord_object`, `poll_ord_status`; §3 — пороги зависших; §5.5 — имена метрик; §8 — новые параметры (задача 12).
    - `.ai-factory/ARCHITECTURE.md` — модуль `marking` (слои, порты `OrdAPI`, `CreativeListener`, `ChannelApprovalListener`, `OrganizationListener`, `bridge.VersionListeners`), `integration/ord`.
    - `docs/open-questions.md` — новый Q-40 (готовность объекта ОРД для зависимых и финальный статус площадки); Q-13, Q-14, Q-15, Q-39 — где в конфигурации рабочие значения.
  - Журнал: не применимо.
  - Файлы: `docs/02-scenarios.md`, `docs/03-placement.md`, `docs/04-domain-model.md`, `docs/05-integrations.md`, `docs/07-non-functional.md`, `docs/open-questions.md`, `.ai-factory/ARCHITECTURE.md`.
<!-- Commit checkpoint: tasks 13-14 -->

## Критерии готовности этапа

- `go build ./...`, `go vet ./...`, `go test -race ./...` проходят локально и в CI. `sqlc generate` и `go generate ./...` не дают изменений. `mockapi` собирается и проходит свой контрактный тест.
- Версия поста становится `ready` только с erid, полученным от ОРД для креатива этой версии (при `ORD_ENABLED=true`); пост без erid в канал не выходит (I-01, 01 §8 п. 4).
- Вызовы ОРД выполняются только в обработчиках заданий вне транзакции (I-13); повтор задания и неизвестный исход не создают второго объекта в ОРД и второго креатива на версию (03 §11, R-1).
- Креатив регистрируется только после готовых контрагентов и изначального договора (05 §3.2); после erid креатив не отправляется повторно (D-03).
- Канал без зарегистрированной площадки не виден в каталоге; ошибка ОРД не отменяет одобрение канала.
- Отклонение ЕРИР возвращает неопубликованные размещения версии в `created` и поднимает оповещение по опубликованным.
- Каждая ошибка ОРД видна в `maxim admin ord errors` и повторяется `retry`; объекты в `error` видны в метрике.
- ИНН (кроме маскированного), наименования организаций, текст поста и OAuth-токен отсутствуют в журнале; в `outbound_request` ИНН маскирован, текста поста и токена нет.
- Решения R-1 … R-20 подтверждены и внесены в документы (задача 14).
