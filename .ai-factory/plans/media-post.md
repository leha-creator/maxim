# План реализации: Медиатека и пост

Branch: master (без отдельной ветки, по решению пользователя)
Created: 2026-10-16

## Settings
- Testing: yes. Документ редактора, распознавание ссылок, сериализатор, длина, UTM-метки, итоговый текст, раскладка плашки, правила кампании и черновика, переходы версии поста проверяются чистыми табличными тестами `post/domain`. Построение изображений — тестами `post/imaging` с эталонными файлами. Сценарии `post/app` с `postgres` выполняются на настоящей PostgreSQL через `testcontainers-go` с `clock.Manual` и `storagefake`. Операции кабинета проверяются через `httptest` поверх собранного сервера (07 §9).
- Logging: verbose. `log/slog` JSON, поля и обязательные события по 07 §5. Для этапа это загрузка и удаление файлов, построение и публикация производных, сохранение черновика, предпросмотр, создание версии и смена её состояния, периодическая очистка. Текст поста, документ редактора, адреса ссылок, название и ИНН рекламодателя, имя файла от пользователя в журнал не пишутся (07 §5.4).
- Docs: yes. В конце обязательная точка `/aif-docs`: README бэкенда (новые переменные окружения, публичная корзина MinIO в `compose.dev.yml`, как в разработке загрузить картинку и посмотреть плашку).

## Roadmap Linkage
Milestone: "Медиатека и пост"
Rationale: план целиком реализует этот этап: загрузку в хранилище с проверкой типа и размера, производные с плашкой (название и ИНН), документ редактора и сериализацию в форматирование MAX, распознавание ссылок, предпросмотр и версии поста.

## Research Context
Source: .ai-factory/RESEARCH.md (Active Summary), выдержка по этапу

- Текст поста в MAX — до 4000 символов. Форматирование: жирный, курсив, подчёркнутый, зачёркнутый, моноширинный, ссылка, выделение (`<mark>`), заголовок, цитата. Списков и спойлеров нет.
- Изображение — до 50 МБ и до 7680×7680. Альбом входит в первую версию (рабочий предел — 10 изображений, Q-05).
- Плашка «название + ИНН» — производная картинка, которую строит сервер. Оригинал в медиатеке не меняется (D-04).
- Редактор — ограниченный WYSIWYG: его схема равна набору форматирования MAX, сервер сериализует документ в формат API (D-05).
- После получения erid пост неизменяем. Правка создаёт новую версию и заново регистрирует креатив (D-03).
- Открыто: вложенность стилей (Q-06), число изображений в посте (Q-05), плашка на каждом изображении или только на первом (Q-24), состав сведений (Q-30).

## Источники ТЗ

- Сценарии: 02 S-04 (кампания и единый пост), S-05 (предпросмотр), S-29 (правка после оформления — только контракт версии).
- Данные: 04 §4 (`media_file`, `media_derivative`, `campaign`, `post`, `post_version`, `post_version_media`), §1 (неизменяемые строки), §9 (перечисления).
- Состояния версии: 03 §1.1, §9; инварианты I-01, I-09.
- Интеграции: 05 §2.5 (форматирование и сериализация, сборка `final_text`), §2.6 (вложения: что уходит в канал).
- Маркировка: 06 §6.1 (сведения в посте), §6.2 (плашка).
- Требования: 07 §1 (`api` ходит только в хранилище), §2 (`build_media_derivative`: сразу, 5 попыток), §4 («Файлы», «Ввод»), §5 (журнал), §7, §8 (конфигурация).
- Реестр: D-03, D-04, D-05, D-10, D-16, D-17, D-18, D-20, D-22, D-23, D-30, A-21, A-23, A-38, A-39, Q-05, Q-06, Q-24, Q-30.
- Архитектура: `.ai-factory/ARCHITECTURE.md` — модуль `post` («медиатека, кампании, пост, версии, сериализация в MAX»), библиотеки `golang.org/x/image` и `disintegration/imaging`, хранилище `minio-go`.
- Существующий код: `internal/platform/storage` (`ObjectStorage`, `NewKey`, `storagefake`), `internal/account/app/organizations.go` (`UploadTaxDocument`: проверка типа по содержимому, загрузка до транзакции, ограничение частоты), `internal/account/httpapi/handlers.go` (`readFilePart`), `internal/bootstrap/server.go` (`BodyLimit`), `internal/platform/httpx` (`RequireCabinet`), `internal/platform/job` (`Register`, `RegisterPeriodic`, `DailyAt`, `dedupe_key`), `internal/channel/*` (образец устройства модуля), `mockapi/internal/store/post.go` (`Serialize`, `CheckDraft`, `FinalText` тестового сервера — образец поведения, не код для переноса).

## Границы этапа

Входит:
- Медиатека: `uploadMedia`, `listMedia`, `deleteMedia`; оригинал в закрытой области хранилища.
- Производные `preview`, `plain`, `labeled` (плашка по 06 §6.2) и задание `build_media_derivative`.
- Публичная область хранилища и публикация производных, вошедших в версию поста (07 §4).
- Кампании: `listCampaigns`, `createCampaign`, `getCampaign`, `updateCampaign` (R-1).
- Черновик поста: `getCampaignPost`, `saveCampaignPost`; документ редактора, его проверка и нормализация, распознавание ссылок (D-22).
- Сериализатор в формат `html` MAX, подсчёт длины, UTM-метки (A-21), сборка `final_text` со строкой маркировки и erid (05 §2.5).
- Предпросмотр `previewCampaignPost` (S-05) с автоматическими проверками.
- Версии поста: создание снимка из черновика, переходы `pending_moderation → approved | rejected`, `approved → ready`, неизменяемость `ready` (D-03, I-09) — как сервис для следующих этапов.

Не входит (следующие этапы):
- `checkoutCampaign`, `createOrderPostVersion`, состояния кампании `ordered` и `archived`, проверка полноты кампании при оплате (A-39 целиком): этап «Заказ и размещения». Он вызовет `CreateVersion` из этого этапа в транзакции оформления.
- Модерация поста администратором (S-17): этап «API администратора». Здесь — только переходы версии в `domain`/`app`.
- Получение erid и вызов `MarkReady`: этап «Маркировка через Яндекс ОРД».
- Загрузка вложений в MAX (`POST /uploads`) и публикация: этап «Автопостинг и контроль».
- Видео, аудио, файлы (A-23).

## Решения этапа, требующие подтверждения

Документы этих мест не определяют или противоречат друг другу. План реализует указанный вариант. После подтверждения задача 16 вносит его в документы. При отказе меняется только отмеченная задача.

| # | Вопрос | Решение в плане | Задача |
|---|---|---|---|
| R-1 | Пост живёт в кампании (D-16), а кампании записаны в этап «Заказ и размещения» | Таблица `campaign` и операции `listCampaigns`, `createCampaign`, `getCampaign`, `updateCampaign` — в этом этапе, в модуле `post`. Создание кампании создаёт пустой пост. Состояния `ordered` и `archived`, поле `order_id` и `checkoutCampaign` — следующий этап | 1, 2, 7, 12, 14 |
| R-2 | Правила загрузки | Тип — по содержимому (`http.DetectContentType`), он должен совпасть с форматом из `image.DecodeConfig`: только JPEG, PNG, WebP. Размер — не больше `MEDIA_MAX_BYTES` (50 МБ). Каждая сторона — от `MEDIA_MIN_SIDE` (160) до 7680 пикселей, с учётом поворота EXIF. Анимированные PNG (`acTL`) и WebP (флаг анимации в `VP8X`) отклоняются. Причины `422`: `empty`, `too_large`, `unsupported_type`, `animated`, `too_small`, `too_big_dimensions`, `malformed`. Предел тела `POST /media` — 51 МБ. Одновременных загрузок на экземпляр `api` — не больше `MEDIA_UPLOAD_CONCURRENCY` (4), сверх предела — `429`. Частота — `MEDIA_UPLOADS_PER_HOUR` (60) на пользователя. Повтор того же файла (`sha256`) в организации возвращает существующую запись. Имя от пользователя очищается от управляющих символов и пути, обрезается до 255 символов, пустое заменяется на `image` | 1, 6, 10, 13 |
| R-3 | Поворот и метаданные | `api` полностью картинку не декодирует, только `DecodeConfig` и чтение EXIF-ориентации. В `width`/`height` записываются размеры после поворота. Оригинал хранится байт в байт в закрытой области. Производные строятся после поворота и без метаданных (07 §4) | 6, 8, 10 |
| R-4 | Форматы производных; в `x/image` нет кодировщика WebP | `preview`: JPEG качества 80, длинная сторона `MEDIA_PREVIEW_SIDE` (640), прозрачность заливается белым. `plain` и `labeled`: JPEG → JPEG качества 90, PNG → PNG. WebP → PNG, если есть прозрачность, иначе JPEG качества 90. Это отступление от 06 §6.2 («тот же формат»). Производная больше 50 МБ перекодируется в JPEG качества 90; если и так не помещается — ошибка без повторов и ERROR | 8, 16 |
| R-5 | Шаблон плашки (06 §6.2) | Шрифт Go Bold (`golang.org/x/image/font/gofont/gobold`, лицензия BSD, кириллица есть) встроен в бинарник. Полоса — чёрная с непрозрачностью 60%, белый текст, поля по 2% ширины. Начальный кегль — 55% высоты полосы, нижний предел — 60% начального. Затем название обрезается с «…», ИНН остаётся целым. Версия шаблона — константа `PlateTemplateVersion = 1` в коде, а не в конфигурации: она меняется вместе с кодом. `params_hash = sha256(kind, версия шаблона, название, ИНН)`; для `plain` и `preview` в хеш идёт только `kind` и версия построения. Плашка ставится на каждое изображение альбома (A-38, рабочий ответ на Q-24) | 6, 8 |
| R-6 | Когда строятся производные | `preview` ставится заданием при загрузке. `plain` или `labeled` (по `token_carrier`) — при каждом сохранении черновика, для каждого изображения, с текущими реквизитами. Задание `build_media_derivative` идемпотентно по `(media_file_id, kind, params_hash)`, `dedupe_key = media:{id}:{kind}:{hash}`. Пока `preview` не готова, `MediaFile.preview_url` — подписанная ссылка на оригинал. Пока нужная производная не готова, предпросмотр отдаёт проблему `media_processing`. Все ссылки кабинета — подписанные, со сроком `MEDIA_URL_TTL` (1 ч) | 1, 10, 11 |
| R-7 | Публичная область (07 §1, §4: публичны только производные, вошедшие в версию) | Отдельная корзина `S3_BUCKET_PUBLIC` с анонимным чтением, без перечисления объектов. Ключ — `pub/<uuidv7>-<32 hex случайных>.<ext>`. Копирование на стороне хранилища (`CopyObject`) делает задание `publish_media_derivative` после создания версии. `public_url = S3_PUBLIC_BASE_URL + ключ`. Опубликованное не удаляется: это образец для ОРД | 3, 12, 14 |
| R-8 | Проверка и нормализация документа редактора | Блоков — не больше 300, текстовых узлов — не больше 3000, JSON — не больше 128 КБ. CRLF и CR заменяются на LF, табуляция — на пробел, остальные управляющие символы удаляются. `\n` внутри текста — перенос строки. Повторы меток в узле убираются. Пустые узлы удаляются, соседние узлы с одинаковыми метками сливаются. Пустой абзац сохраняется (пустая строка). `href` — только `https://`, есть хост, нет `userinfo` и пробелов, не длиннее 2048. Нарушение — `422` с путём узла. Сохранённый документ возвращается клиенту уже нормализованным | 4, 11 |
| R-9 | Распознавание адресов (D-22) | Выполняется при сохранении над нормализованным документом. Фрагмент `https://` до пробела или конца строки внутри узла без метки `link` становится ссылкой, остальные метки узла сохраняются. В адрес не входят завершающие `.,;:!?»"'`, а также `)`, если внутри адреса нет парной `(`. Распознанный адрес проходит те же проверки R-8. Если он их не проходит, остаётся текстом | 4, 11 |
| R-10 | Теги MAX и вложенность (05 §2.5, Q-06) | Теги сверяются со страницей dev.max.ru о форматировании во время задачи 5 и записываются в 05 §2.5. Рабочий набор: `<b>`, `<i>`, `<u>`, `<s>`, `<code>`, `<a href>`, `<mark>`, `<h1>`, `<blockquote>`. Блоки разделяются `\n`. Экранируются `& < > "`. Вложенность разрешена, порядок фиксированный: `link` снаружи, затем `bold`, `italic`, `underline`, `strike`, `highlight`, `code` (рабочий ответ на Q-06). Проверка на тестовом канале — этап «Проверки и доступы» | 5, 16 |
| R-11 | Как считать длину | Длина — в кодовых единицах UTF-16 сериализованной строки вместе с разметкой и UTM-метками. Для кириллицы и эмодзи это не мягче подсчёта по символам. Пределы: `POST_BODY_LIMIT` (3800) и `POST_FINAL_TEXT_LIMIT` (4000), A-38. Черновик длиннее 3800 сохраняется с проблемой `body_too_long`, чтобы работа не терялась. Сохранение длиннее 8000 отклоняется (`422`, `too_long`) | 5, 7, 11 |
| R-12 | UTM-метки (A-21) | Метки ставятся при создании версии, с настоящим номером заказа. В предпросмотре вместо номера — заглушка `order_0000000000` фиксированной ширины, чтобы длина не была занижена. Недостающие из `utm_source`, `utm_medium`, `utm_campaign` дописываются в конец строки запроса, уже заданные не меняются. При `token_position = link` к каждой ссылке при переходе в `ready` дописывается `erid` | 5, 11, 12 |
| R-13 | Место токена (A-38, D-30) | Если поля не переданы: `token_carrier = media` при наличии изображений, иначе `text`; `token_position = below`. Недоступный выбор не отклоняется при сохранении, а даёт проблему в предпросмотре: `media_required_for_token`, новая `link_required_for_token` (нет ссылок при `link`). Новые коды: `media_processing` (R-6), `requisites_missing` (у организации нет названия или ИНН) | 1, 7, 11 |
| R-14 | Откуда название и ИНН рекламодателя | Порт `post/domain.AdvertiserRequisites` возвращает `organization.name` и `organization.inn` текущей организации. Реализация — переходник в `bootstrap` поверх `account/app`. Снимок попадает в версию (04 §4); черновик реквизиты не хранит, плашка строится по текущим | 7, 11, 14 |
| R-15 | Изображения в черновике | Файлы — своей организации, не удалённые, без повторов, не больше `MEDIA_ALBUM_LIMIT` (10, A-23). Иначе `422` с `media_not_found` или `duplicate_media`. `deleteMedia` в той же транзакции убирает файл из черновиков всех кампаний организации. Версии не меняются (04 §4) | 7, 10, 11 |
| R-16 | Кто правит | Любой участник организации с открытым кабинетом рекламодателя (`httpx.RequireCabinet(ctx, "advertiser")`), иначе `403`. Чужая кампания или файл — `404` | 13 |
| R-17 | Поля кампании (A-39 — только форма, полнота — при оплате) | `name` — от 1 до 200 символов после обрезки пробелов. `date_from <= date_to`, длина периода — от 1 до 14 суток, `date_from` не раньше сегодняшнего дня по Москве. `time_from` и `time_to` задаются только вместе, `time_from < time_to`, разница — не меньше 1 часа. `extra_requirements` — не длиннее 2000. В `PATCH` отсутствующее поле не меняется; очистить поле нельзя (ограничение схемы, записывается в спецификацию). Период и требования меняются только в `draft` (`409`), название — всегда. `listCampaigns`: новые сверху, курсор по `(created_at, id)` | 1, 7, 12 |
| R-18 | Контракт версии для следующих этапов | `post/app.CreateVersion(ctx, orgID, campaignID, userID, orderNumber)` выполняется в транзакции вызывающего. Условия: в черновике нет проблем и все нужные производные готовы, иначе `409` `post_not_ready` с кодами проблем. `number` — следующий. Сохраняются снимок реквизитов, `content_doc`, `body` с UTM, место токена, строки `post_version_media` с производной по `token_carrier`. Ставится `publish_media_derivative` на каждую производную. `Approve`, `Reject(reason)`, `MarkReady(erid)` — методы `domain` с сервисными методами `app`. `MarkReady` собирает `final_text`; длиннее 4000 — ошибка. Неизменяемость `ready` закрепляет триггер базы: запрет `UPDATE` и `DELETE` строки `post_version` в `ready` и её `post_version_media` (04 §1) | 7, 9, 12 |
| R-19 | Новые виды заданий и очистка | `build_media_derivative` уже есть в 07 §2 (сразу, 5 попыток, `dedupe_key` по R-6). Новые: `publish_media_derivative` (сразу, 10 попыток, `dedupe_key media_publish:{derivative_id}`) и `purge_media` (раз в сутки, 04:30 МСК, 1 попытка). `purge_media` удаляет из хранилища оригиналы, удалённые больше `MEDIA_RETENTION_DELETED` (30 суток) назад и не входящие ни в одну версию, а также производные, которые не входят ни в одну версию и старше 30 суток. Строке ставится `purged_at`. Удаление из хранилища — через задание, а не через `cleanup`, потому что это внешний вызов | 2, 10, 12, 14 |
| R-20 | Память при построении | `worker` строит не больше `MEDIA_BUILD_CONCURRENCY` (2) изображений одновременно на экземпляр: одно 7680×7680 RGBA занимает около 236 МБ. Сверх предела задание ждёт, а не падает | 8, 14 |
| R-21 | Список медиатеки | Новые сверху, без удалённых, курсор по `(created_at, id)`, `limit` по общей схеме спецификации | 9, 10 |

## Commit Plan
- **Commit 1** (после задач 1–3): `feat(post): api contract, media and post schema, public storage`
- **Commit 2** (после задач 4–7): `feat(post): editor document, serializer, plate layout and post domain`
- **Commit 3** (после задач 8–9): `feat(post): image derivatives and post storage`
- **Commit 4** (после задач 10–12): `feat(post): media library, campaigns, draft, preview and versions`
- **Commit 5** (после задач 13–14): `feat(post): cabinet API and wiring`
- **Commit 6** (после задач 15–16): `test(post): media and post e2e, docs sync`

## Tasks

### Phase 1: Договор, данные, хранилище

- [x] **Task 1: Спецификация медиатеки, кампаний и поста (R-1, R-2, R-6, R-13, R-17)**
  - `api/openapi.yaml`:
    - `uploadMedia`: описать R-2 (пределы, причины `422`, повтор того же файла возвращает существующую запись), ответ `429`. `MediaFile.preview_url` — подписанная ссылка со сроком; пока превью строится, ведёт на оригинал (R-6).
    - `PostProblem.code`: добавить `link_required_for_token`, `media_processing`, `requisites_missing` (R-13). `PostPreview.media_urls` — подписанные ссылки со сроком.
    - `saveCampaignPost`: `422` с причинами `media_not_found`, `duplicate_media`, `too_long`, `invalid_doc` и путём узла в `details`. Описать нормализацию и распознавание ссылок: ответ содержит документ после них (R-8, R-9).
    - `updateCampaign`: `409` для полей периода вне `draft`, `422` с причинами `period_too_long`, `period_in_past`, `period_order`, `time_window`, `time_pair`. Отметить, что очистить поле нельзя (R-17).
    - `createCampaign`: `422` для тех же причин.
  - `go generate ./internal/apigen`; `mockapi` собирается из той же спецификации: добавить новые коды проблем в `mockapi/internal/store/post.go` (`link_required_for_token`), чтобы контрактный тест `mockapi` проходил.
  - Журнал: не применимо.
  - Тесты: существующие тесты `bootstrap` и `mockapi` (`contract_test.go`) проходят. Операции этапа отвечают `501` до задачи 13.
  - Файлы: `api/openapi.yaml`, `internal/apigen/api.gen.go`, `mockapi/internal/api/api.gen.go`, `mockapi/internal/store/post.go`.

- [x] **Task 2: Миграция медиатеки и поста (R-1, R-18, R-19)**
  - `migrations/00006_media_post.sql` по 04 §4:
    - `media_file` (`id uuid`, поля 04 §4, `created_by`, `created_at`, `purged_at`, `deleted_at`). `check` на `mime`, `size_bytes <= 52428800`, `width`/`height` от 1 до 7680. Уникальны `storage_key` и `(organization_id, sha256) WHERE deleted_at IS NULL`. Индекс `(organization_id, created_at DESC, id DESC) WHERE deleted_at IS NULL`.
    - `media_derivative` (`id uuid`, поля 04 §4, `mime`, `status text check (pending, ready)`, `public_storage_key`, `published_at`, `created_at`, `purged_at`). Уникальна тройка `(media_file_id, kind, params_hash)`. `check`: `public_url` обязателен при заданном `published_at`.
    - `campaign` (поля 04 §4, `created_by`, `created_at`, `updated_at`, `status check (draft, ordered, archived)`, `check (date_from <= date_to)`, `check (time_from < time_to)`).
    - `post` (поля 04 §4, уникален `campaign_id`, `updated_at`).
    - `post_version` (поля 04 §4, `created_at`, уникальна пара `(post_id, number)`, `check` на `status`, `token_carrier`, `token_position`, `char_length(body) <= 3800` как страховка, `final_text` обязателен в `ready`).
    - `post_version_media` (поля 04 §4, уникальна пара `(post_version_id, position)`, `check position between 0 and 9`).
    - Триггер `post_version_ready_immutable` (R-18): запрет `UPDATE` и `DELETE` строки `post_version` с `status = 'ready'` в старом значении и строк `post_version_media` такой версии.
  - Права роли — через `ALTER DEFAULT PRIVILEGES` из `00002`; проверить, что новые таблицы покрыты.
  - Журнал: не применимо (INFO `migrations applied` пишет `migrate`).
  - Тесты: `up` → `down` → `up`. Роль приложения читает и пишет новые таблицы. Изменение версии в `ready` и удаление её `post_version_media` не проходят. Второй живой файл с тем же `sha256` в организации не вставляется, после мягкого удаления — вставляется. Вторая версия с тем же номером не вставляется.
  - Файлы: `migrations/00006_media_post.sql`.

- [x] **Task 3: Публичная область хранилища и чтение объектов (R-7)**
  - `internal/platform/storage/storage.go`: в `ObjectStorage` добавить `Get(ctx, key) (io.ReadCloser, int64, error)` (закрытая область) и `CopyToPublic(ctx, privateKey, publicKey, contentType) error`. Функции `PublicURL(key) string` (база из `S3_PUBLIC_BASE_URL`) и `NewPublicKey(ext)` по R-7 (`crypto/rand`, 16 байт). `S3Options` получает `PublicBucket`, `PublicBaseURL`; `NewS3` проверяет обе корзины.
  - `storagefake`: `Get`, `CopyToPublic`, доступ к публичным объектам в тестах, `FailWith(op, err)`.
  - `compose.dev.yml`: служба инициализации MinIO (`mc`) создаёт корзины `maxim-private`, `maxim-public` и ставит анонимное чтение без перечисления только на публичную.
  - Журнал: DEBUG `object read` (`key`, `size`), DEBUG `object published` (`key`, `public_key`), ERROR `object storage failed` (уже есть, `op` — `Storage.Get` / `Storage.CopyToPublic`).
  - Тесты: `storagefake` — чтение отсутствующего ключа → `ErrNotFound`; копирование сохраняет тип; `NewPublicKey` даёт разные ключи нужной формы. `S3` — тест с меткой `integration` на MinIO из `testcontainers-go`, пропускается с `-short`.
  - Файлы: `internal/platform/storage/{storage.go,storage_test.go,public.go}`, `internal/platform/storage/storagefake/fake.go`, `compose.dev.yml`.
<!-- Commit checkpoint: tasks 1-3 -->

### Phase 2: Предметные правила

- [x] **Task 4: `post/domain` — документ редактора, нормализация, распознавание ссылок (R-8, R-9)** (depends on 1)
  - `internal/post/domain/doc.go`: собственные типы `Doc`, `Block` (`paragraph`, `heading`, `blockquote`), `Text`, `Mark` (`bold`, `italic`, `underline`, `strike`, `code`, `link`, `highlight`) с `Href`. Типы `apigen` в `domain` не попадают: преобразование делает `httpapi`. Метод `Normalize(doc, limits) (Doc, error)` по R-8; ошибка `DocError{Path, Reason}`.
  - `internal/post/domain/autolink.go`: `Autolink(doc) Doc` по R-9. Делит узлы, переносит метки, проверяет адрес функцией `ValidHref` из R-8.
  - `HasLinks(doc) bool`, `PlainText(doc) string` (проверка на пустоту).
  - Журнал: не применимо (`domain` не пишет журнал).
  - Тесты (табличные): каждый запрет R-8 с путём узла; CRLF, табуляция, управляющие символы; слияние соседних узлов, удаление пустых, повторы меток; пустой абзац сохраняется. Распознавание: адрес в начале, середине и конце узла; завершающие `.`, `,`, `)`, `»`; скобки внутри адреса (`https://ru.wikipedia.org/wiki/A_(B)`); `http://` не распознаётся; адрес внутри узла с `bold` сохраняет `bold`; адрес уже со ссылкой не трогается; два адреса в одном узле; адрес с пробелом после `https://` — не адрес; адрес длиннее 2048 остаётся текстом. Повторное применение `Normalize` и `Autolink` ничего не меняет.
  - Файлы: `internal/post/domain/{doc.go,autolink.go,doc_test.go,autolink_test.go}`.

- [x] **Task 5: `post/domain` — сериализатор, длина, UTM, итоговый текст (R-10 … R-12)** (depends on 4)
  - Перед кодом — сверить теги со страницей `https://dev.max.ru/docs-api/use-cases/sending-messages/text-formatting` и схемой `max-messenger/api-schema`. Расхождение с рабочим набором R-10 записать в задачу 16.
  - `internal/post/domain/serialize.go`: `Serialize(doc, utm *UTM) string` — `html` MAX по R-10, порядок вложения меток, экранирование. `UTM{Campaign string}`: к адресам ссылок добавляются недостающие метки (R-12) с сохранением исходного порядка параметров.
  - `Length(s string) int` — кодовые единицы UTF-16 (R-11).
  - `internal/post/domain/final_text.go`: `FinalText(body string, carrier, position, name, inn, erid string) (string, error)` по 05 §2.5: строка маркировки, её место, `erid` к каждой ссылке при `link`, ошибка `ErrFinalTextTooLong` больше `POST_FINAL_TEXT_LIMIT`. `MarkingLine(...)` отдельно для предпросмотра.
  - Журнал: не применимо.
  - Тесты (табличные и эталонные): каждый элемент отдельно; все метки сразу (порядок вложения); `<`, `&`, `"` в тексте и в адресе; заголовок и цитата с метками; пустой абзац; `\n` внутри текста; UTM: адрес без запроса, с запросом, с частью меток, с `#фрагментом`; длина: кириллица, эмодзи (2 единицы), разметка учитывается. `FinalText` — все 6 сочетаний места пометки и erid × есть или нет ссылок; erid к адресу с запросом и без; пометка на плашке + erid в ссылке → без строки маркировки; 3800 + строка маркировки ≤ 4000 при названии в 200 символов; превышение → ошибка. Эталоны — `testdata/serialize/*.golden`.
  - Файлы: `internal/post/domain/{serialize.go,final_text.go,serialize_test.go,final_text_test.go}`, `internal/post/domain/testdata/serialize/*`.

- [x] **Task 6: `post/domain` — правила файла и раскладка плашки (R-2, R-3, R-5)**
  - `internal/post/domain/media.go`: `MediaFile`, `Derivative`, `DerivativeKind` (`preview`, `plain`, `labeled`), `CheckUpload(info UploadInfo, limits) error` по R-2 (`UploadInfo` — тип по содержимому, формат декодера, размеры после поворота, признак анимации, размер). `CleanOriginalName(s) string`. `ParamsHash(kind, templateVersion, name, inn) string` (R-5). `NeededKind(carrier) DerivativeKind`.
  - `internal/post/domain/plate.go`: `PlateText(name, inn)`, `PlateLayout(width, height int, name, inn string, m TextMeasurer) Layout` — чистая раскладка по 06 §6.2 и R-5: высота полосы `max(8% высоты, 40)`, кегль от 55% высоты полосы до 60% от него, затем обрезка названия с «…» с сохранением ИНН. `TextMeasurer` — интерфейс (ширина строки при кегле); реализация на шрифте — в `post/imaging`.
  - Журнал: не применимо.
  - Тесты (табличные): каждая причина отказа `CheckUpload` на границах (159/160, 7680/7681 по каждой стороне, ровно 50 МБ и на байт больше, поворот 90° меняет стороны местами); `CleanOriginalName` — путь `../../a.png`, управляющие символы, 300 символов, пустое. `PlateLayout` с поддельным измерителем: короткое название — начальный кегль; длинное — уменьшение; очень длинное — обрезка с «…», ИНН целый; высота 300 → полоса 40; высота 1000 → 80. `ParamsHash` меняется при смене названия, ИНН или версии шаблона.
  - Файлы: `internal/post/domain/{media.go,plate.go,media_test.go,plate_test.go}`.

- [x] **Task 7: `post/domain` — кампания, черновик, проблемы, версия (R-1, R-11, R-13 … R-15, R-17, R-18)** (depends on 5, 6)
  - `internal/post/domain/campaign.go`: `Campaign`, `NewCampaign(orgID, userID, input, today)`, `(*Campaign).Update(input, today) error` по R-17. Ошибки с причинами из задачи 1, `ErrNotDraft`.
  - `internal/post/domain/draft.go`: `Draft{Doc, MediaIDs, TokenCarrier, TokenPosition}`, `(*Post).SaveDraft(input, now) error`: значения места токена по умолчанию (R-13), проверка изображений (R-15 — повторы и предел; принадлежность проверяет `app`), предел сохранения 8000 (R-11). `RemoveMedia(id) bool`.
  - `internal/post/domain/problems.go`: `Problems(draft, body string, ctx ProblemContext) []Problem` — `empty_text`, `body_too_long`, `too_many_media`, `link_not_https` (страховка), `media_required_for_token`, `link_required_for_token`, `media_processing`, `requisites_missing`. `ProblemContext` — готовность производных и наличие реквизитов.
  - `internal/post/domain/version.go`: `Version`, `VersionMedia`, `NewVersion(post, number, requisites, body, media []VersionMedia, userID, now) (Version, error)`; переходы `Approve(adminID, now)`, `Reject(adminID, reason, now)` (причина от 1 до 1000 символов), `MarkReady(erid, now)` (собирает `final_text`) — только из допустимых состояний 03 §1.1, иначе `apperr.InvalidTransition`. `VersionStatus` и значения — по 04 §9.
  - Порты в `ports.go`: `MediaFiles`, `Derivatives`, `Campaigns`, `Posts`, `Versions`, `AdvertiserRequisites` (R-14), `Renderer` (построение производной: оригинал → байты, тип, размеры), `ObjectStore` (нужное подмножество `platform/storage`).
  - Журнал: не применимо.
  - Тесты (табличные): каждая причина R-17 на границах (1 и 14 суток, 15 суток, вчера и сегодня по Москве в 23:59 и 00:00, окно ровно 1 ч и 59 мин); правка периода вне `draft` → `ErrNotDraft`, правка названия — можно. Место токена по умолчанию с изображениями и без; каждый код проблемы в одиночку; чистый черновик — без проблем. Переходы версии из каждого состояния × каждый переход; `MarkReady` с длинным `final_text` → ошибка, состояние не меняется.
  - Файлы: `internal/post/domain/{campaign.go,draft.go,problems.go,version.go,ports.go,campaign_test.go,draft_test.go,problems_test.go,version_test.go}`.
<!-- Commit checkpoint: tasks 4-7 -->

### Phase 3: Построение изображений и хранение

- [x] **Task 8: `post/imaging` — превью, копия, плашка (R-3 … R-5, R-20)** (depends on 6)
  - `go get golang.org/x/image github.com/disintegration/imaging` (версии зафиксировать в `go.mod`).
  - `internal/post/imaging`: реализует `domain.Renderer` и `domain.TextMeasurer`. Декодирование с поворотом по EXIF (`imaging.Decode(r, imaging.AutoOrientation(true))`), WebP — через `golang.org/x/image/webp`. `Preview`, `Plain`, `Labeled(name, inn)` по R-4 и R-5. Шрифт Go Bold через `opentype`, кегль и обрезка — по `domain.PlateLayout`. Кодирование без метаданных. Проверка признака анимации APNG и WebP (`Probe(data) (domain.UploadInfo, error)` без полного декодирования — её вызывает `api`). Семафор `MEDIA_BUILD_CONCURRENCY` (R-20) с ожиданием по `context`.
  - Журнал: DEBUG `media derivative rendered` (`kind`, `width`, `height`, `size_bytes`, `duration_ms`), WARN `media derivative reencoded` (`reason = too_large`), DEBUG `media render waiting` (`in_use`).
  - Тесты: эталонные картинки `testdata/` (JPEG с EXIF-ориентацией 6, PNG с прозрачностью, WebP с прозрачностью и без, анимированные PNG и WebP, 160×160). Плашка: полоса нужной высоты внизу, пиксели над ней не меняются, ИНН целиком на длинном названии (сверка с эталоном с допуском); `plain` без EXIF; повёрнутый JPEG — размеры после поворота; WebP → PNG/JPEG по R-4; `Probe` распознаёт анимацию; семафор не пускает третий одновременный вызов.
  - Файлы: `internal/post/imaging/{imaging.go,probe.go,plate.go,font.go,imaging_test.go,probe_test.go}`, `internal/post/imaging/testdata/*`, `go.mod`, `go.sum`.

- [x] **Task 9: `post/postgres` — файлы, производные, кампании, черновики, версии (R-18, R-21)** (depends on 2, 7)
  - `sqlc.yaml`: запись модуля `post`. Запросы `queries/*.sql`:
    - файлы: вставка, получение своего, поиск по `sha256` в организации, список с курсором R-21, мягкое удаление, проверка набора идентификаторов организации (живые), выборка для `purge_media`;
    - производные: `upsert` в `pending` по тройке, отметка `ready` с ключом и размерами, получение по тройке, набор готовых производных для списка файлов, отметка публикации, выборка для `purge_media`;
    - кампании: вставка, получение своей с `FOR UPDATE`, список с курсором, обновление;
    - посты: вставка, получение по кампании с `FOR UPDATE`, сохранение черновика, удаление файла из черновиков организации (`jsonb`);
    - версии: следующий номер, вставка версии и строк медиа, получение с `FOR UPDATE`, смена состояния, получение по посту.
  - Репозитории реализуют порты `domain`; `draft` хранится как `jsonb` своего формата (`doc`, `media_ids`, `token_carrier`, `token_position`) с версией формата.
  - Журнал: DEBUG `post draft saved` (`post_id`, `media_count`, `doc_bytes`), DEBUG `media files listed` (`count`).
  - Тесты (PostgreSQL): курсор обходит файлы ровно по разу при одинаковом `created_at`; удалённые файлы не видны; удаление файла убирает его из черновиков двух кампаний организации и не трогает чужую организацию; `upsert` производной идемпотентен; номер версии растёт, два параллельных `CreateVersion` одного поста не дают одинаковых номеров (блокировка поста); версия в `ready` не меняется (ошибка триггера проходит как `apperr`).
  - Файлы: `sqlc.yaml`, `internal/post/postgres/{store.go,media.go,derivatives.go,campaigns.go,posts.go,versions.go,*_test.go}`, `internal/post/postgres/queries/{media,derivatives,campaigns,posts,versions}.sql`, `internal/post/postgres/sqlc/*`.
<!-- Commit checkpoint: tasks 8-9 -->

### Phase 4: Сценарии

- [x] **Task 10: Медиатека и построение производных (R-2, R-3, R-6, R-15, R-19, R-21)** (depends on 8, 9)
  - `internal/post/app/media.go`:
    - `Upload(ctx, orgID, userID, name string, data []byte)`: ограничение частоты (R-2, по `audit_log` или счётчику как в `UploadTaxDocument`), `Probe`, `domain.CheckUpload`, поиск дубликата по `sha256`. Затем `PutPrivate` до транзакции (ключ `media/<uuidv7><ext>`). В транзакции: вставка `media_file`, `upsert` производной `preview`, `Enqueue(build_media_derivative)`, `audit_log` `media.uploaded`. Если транзакция не прошла — удалить объект (WARN при неудаче).
    - `List`, `Delete(ctx, orgID, userID, id)`: мягкое удаление и удаление из черновиков в одной транзакции (R-15), `audit_log` `media.deleted`.
    - Ссылки: `PreviewURL(file)` — подписанная ссылка на готовое превью или на оригинал (R-6).
  - `internal/post/app/derivatives.go`: `BuildDerivative(ctx, payload)` для задания: выйти, если производная уже `ready` или файл очищен; прочитать оригинал (`Get`), построить (`Renderer`), `PutPrivate` (ключ `derivative/<uuidv7><ext>`), затем в транзакции отметить `ready`. Ошибка декодирования → `job.Permanent` и ERROR.
  - `internal/post/jobs/jobs.go`: `Register(build_media_derivative, 5 попыток)`.
  - Журнал: INFO `media uploaded` (`media_file_id`, `mime`, `size_bytes`, `width`, `height`; без имени файла), INFO `media upload deduplicated` (`media_file_id`), WARN `media upload rejected` (`reason`, `content_type`, `size_bytes`), WARN `media upload rate limited`, INFO `media deleted` (`media_file_id`, `drafts_updated`), INFO `media derivative built` (`media_file_id`, `kind`, `derivative_id`, `duration_ms`), INFO `media derivative skipped` (`reason`), ERROR `media derivative failed` (`media_file_id`, `kind`, `error`), WARN `media orphan object delete failed` (`key`).
  - Тесты (PostgreSQL, `storagefake`, `clock.Manual`): загрузка JPEG → запись, объект в закрытой области, задание `preview` поставлено; повтор того же файла → та же запись, второго объекта нет; PNG с расширением `.jpg` в имени → тип по содержимому; текст под видом картинки → `422 unsupported_type`; 61-я загрузка за час → `429`; ошибка хранилища → `5xx`, записи нет; задание строит `preview`, повтор задания ничего не меняет; удаление файла из черновика двух кампаний; список не содержит удалённых; чужой файл → `404`.
  - Файлы: `internal/post/app/{service.go,media.go,derivatives.go,media_test.go,derivatives_test.go}`, `internal/post/jobs/jobs.go`.

- [x] **Task 11: Кампании, черновик и предпросмотр (R-1, R-6, R-8, R-9, R-11 … R-15, R-17)** (depends on 9)
  - `internal/post/app/campaigns.go`: `CreateCampaign` (кампания и пустой пост в одной транзакции, `audit_log` `campaign.created`), `ListCampaigns`, `GetCampaign`, `UpdateCampaign` (`FOR UPDATE`, `audit_log` `campaign.updated` с `audit.Diff` без `extra_requirements` в значениях — только признак изменения). «Сегодня» — по Москве из `clock`.
  - `internal/post/app/draft.go`: `GetDraft`, `SaveDraft(ctx, orgID, userID, campaignID, input)`: `Normalize` → `Autolink` → проверка файлов организации (R-15) → `SaveDraft`. В той же транзакции для каждого изображения — `upsert` нужной производной с текущими реквизитами (R-14) и `Enqueue(build_media_derivative)` (R-6). `audit_log` `post.draft_saved` без содержимого.
  - `internal/post/app/preview.go`: `Preview(ctx, orgID, campaignID)`: `Serialize` с заглушкой UTM (R-12), `Length`, проблемы (`domain.Problems`), подписанные ссылки готовых производных, `final_text_sample` через `FinalText` с условным erid `erid_sample` (как в `mockapi`), `body_limit` из конфигурации.
  - Журнал: INFO `campaign created` (`campaign_id`), INFO `campaign updated` (`campaign_id`, `fields`), WARN `campaign update rejected` (`campaign_id`, `reason`), INFO `post draft saved` (`post_id`, `media_count`, `body_length`, `links_autolinked`), WARN `post draft rejected` (`post_id`, `reason`, `path`), DEBUG `post preview built` (`post_id`, `body_length`, `problems` — только коды). Текст, адреса и реквизиты не пишутся.
  - Тесты (PostgreSQL, `storagefake`, `clock.Manual`): создание кампании даёт пустой черновик с `token_carrier = text`; сохранение с адресом в тексте → в ответе ссылка; повторное сохранение того же документа ничего не меняет; чужой файл → `422 media_not_found`; 11 файлов → `422` (схема или `domain`); `media` без изображений → проблема `media_required_for_token`; `link` без ссылок → `link_required_for_token`; пока производные строятся → `media_processing`, после выполнения заданий — нет; длина с UTM-заглушкой больше длины без неё; 3801 единица → `body_too_long`, черновик сохранён; смена названия организации → новая производная `labeled` с другим `params_hash`; правка периода кампании в прошлом → `422`.
  - Файлы: `internal/post/app/{campaigns.go,draft.go,preview.go,campaigns_test.go,draft_test.go,preview_test.go}`.

- [x] **Task 12: Версии поста, публикация производных, очистка (R-7, R-12, R-18, R-19)** (depends on 10, 11)
  - `internal/post/app/versions.go`:
    - `CreateVersion(ctx, orgID, campaignID, userID, orderNumber int64) (domain.Version, error)` по R-18: работает в транзакции из `context` (открывает свою, если её нет). Блокировка поста, проблемы черновика, готовность производных, снимок реквизитов, `Serialize` с настоящим UTM, вставка версии и медиа, `Enqueue(publish_media_derivative)` для каждой производной, `audit_log` `post.version_created`.
    - `ApproveVersion`, `RejectVersion`, `MarkVersionReady(ctx, versionID, erid)` — блокировка версии, переход `domain`, сохранение, `audit_log` (`post.version_approved`, `post.version_rejected`, `post.version_ready`). Проверку роли администратора и вызовы делают следующие этапы.
    - `GetVersion(ctx, versionID)` с публичными ссылками изображений (для ОРД и публикации).
  - `internal/post/app/publish.go`: `PublishDerivative(ctx, derivativeID)` — выйти, если уже опубликована; `CopyToPublic` с ключом R-7 вне транзакции; затем отметить `public_url`, `published_at`.
  - `internal/post/app/purge.go`: `PurgeMedia(ctx)` по R-19 порциями по 100: удаление объекта, затем `purged_at`. Объект, который уже отсутствует, считается удалённым.
  - `internal/post/jobs/jobs.go`: `Register(publish_media_derivative, 10)`, `RegisterPeriodic(purge_media, job.DailyAt(4, 30), …)`.
  - Журнал: INFO `post version created` (`post_id`, `version_id`, `number`, `media_count`, `body_length`), WARN `post version blocked` (`post_id`, `problems`), INFO `post version status changed` (`version_id`, `from`, `to`), WARN `post version transition rejected` (`version_id`, `transition`, `status`), ERROR `post final text too long` (`version_id`, `length`), INFO `media derivative published` (`derivative_id`), INFO `media purged` (`files`, `derivatives`), WARN `media purge object failed` (`key`).
  - Тесты (PostgreSQL, `storagefake`, `clock.Manual`): версия 1 из готового черновика — снимок реквизитов, `body` с `utm_campaign=order_42`, `post_version_media` с `labeled` при `media` и с `plain` при `text`; черновик с проблемой → `409 post_not_ready`; не готова производная → `409` с `media_processing`; вторая версия — номер 2, первая не изменилась; правка черновика после версии не меняет версию; задание публикации даёт `public_url`, повтор не копирует второй раз; переходы `approve` → `ready` с erid под постом и в ссылках; `reject` после `ready` → ошибка; очистка: удалённый файл старше 30 суток без версий — объект удалён; файл в версии — остаётся; производная устаревшего `params_hash` без версий — удалена.
  - Файлы: `internal/post/app/{versions.go,publish.go,purge.go,versions_test.go,publish_test.go,purge_test.go}`, `internal/post/jobs/jobs.go`.
<!-- Commit checkpoint: tasks 10-12 -->

### Phase 5: HTTP и сборка

- [x] **Task 13: Операции кабинета: медиатека, кампании, пост (R-2, R-16)** (depends on 10, 11)
  - `internal/post/httpapi`: `UploadMedia` (чтение части `file` по образцу `readFilePart` с пределом `MEDIA_MAX_BYTES + 1`, имя из `part.FileName()`; семафор `MEDIA_UPLOAD_CONCURRENCY` → `429`), `ListMedia`, `DeleteMedia`, `ListCampaigns`, `CreateCampaign`, `GetCampaign`, `UpdateCampaign`, `GetCampaignPost`, `SaveCampaignPost`, `PreviewCampaignPost`. Везде `httpx.RequireCabinet(ctx, "advertiser")`. Преобразование `apigen.EditorDoc` ↔ `domain.Doc`, `MediaFile`, `Campaign`, `PostDraft`, `PostPreview`.
  - `internal/bootstrap/server.go`: предел тела `POST /api/v1/media` — 51 МБ (R-2).
  - Ошибки через `apperr`: `404`, `409`, `422` с `details.reason` и `details.path`, `429`.
  - Журнал: WARN `403` пишет `httpx`; WARN `media upload concurrency exceeded` (`limit`); остальное — в `app`.
  - Тесты (`httptest` поверх собранного сервера): без кабинета рекламодателя → `403`; загрузка `multipart` JPEG → `201`, ответ проходит проверку схемой, `preview_url` есть; 51 МБ → `422 too_large` (или `413` от предела тела — зафиксировать одно поведение и описать в спецификации); CSRF обязателен для `POST`/`PUT`/`PATCH`/`DELETE`; `saveCampaignPost` возвращает нормализованный документ; неизвестный тип блока → `422` от проверки запроса до обработчика.
  - Файлы: `internal/post/httpapi/{handlers.go,media.go,campaigns.go,post.go,convert.go,handlers_test.go}`, `internal/bootstrap/server.go`.

- [x] **Task 14: Конфигурация и сборка (R-7, R-14, R-19, R-20)** (depends on 12, 13)
  - `internal/bootstrap/config.go`: `S3_BUCKET_PUBLIC`, `S3_PUBLIC_BASE_URL` (https в продакшене), `MEDIA_MAX_BYTES` (52428800, не больше 50 МБ — предел MAX), `MEDIA_MIN_SIDE` (160), `MEDIA_PREVIEW_SIDE` (640), `MEDIA_UPLOADS_PER_HOUR` (60), `MEDIA_UPLOAD_CONCURRENCY` (4), `MEDIA_BUILD_CONCURRENCY` (2), `MEDIA_ALBUM_LIMIT` (10, не больше 10 до ответа на Q-05), `MEDIA_URL_TTL` (1h), `MEDIA_RETENTION_DELETED` (720h), `POST_BODY_LIMIT` (3800), `POST_FINAL_TEXT_LIMIT` (4000, `POST_BODY_LIMIT` меньше его). Проверка диапазонов при запуске. Хранилище нужно и `worker` (построение производных).
  - `internal/bootstrap/post.go`: сборка `post/app.Service`. Переходник `AdvertiserRequisites` поверх `account/app` (R-14), `post/imaging`, хранилище или `storagefake` в `APP_ENV=test`. `modules.go`, `worker.go`, `server.go`: обработчики `post/httpapi`, регистрация `build_media_derivative`, `publish_media_derivative`, `purge_media`. `unimplemented.go`: убрать заглушки реализованных операций (`checkoutCampaign` и `createOrderPostVersion` остаются).
  - `.env.example`, `compose.dev.yml`: новые переменные, публичная корзина (задача 3). В заметку для `/aif-docs`: как загрузить картинку через Swagger или `curl` и открыть производную с плашкой.
  - Журнал: INFO `worker started` с новыми видами заданий; конфигурация пишется маскированной (как сейчас).
  - Тесты: проверки конфигурации (`MEDIA_MAX_BYTES` больше 50 МБ, `POST_BODY_LIMIT >= POST_FINAL_TEXT_LIMIT`, пустая публичная корзина вне `test`). `wiring_test.go`: три вида заданий зарегистрированы, операции этапа больше не отвечают `501`, `checkoutCampaign` — всё ещё `501`.
  - Файлы: `internal/bootstrap/{config.go,config_test.go,post.go,modules.go,worker.go,server.go,unimplemented.go,wiring_test.go}`, `.env.example`, `compose.dev.yml`.
<!-- Commit checkpoint: tasks 13-14 -->

### Phase 6: Проверка и документы

- [x] **Task 15: Сквозные тесты медиатеки и поста** (depends on 14)
  - `internal/bootstrap/post_e2e_test.go` на собранных `api` и `worker` (PostgreSQL, `clock.Manual`, `storagefake`, `mailfake`):
    - S-04 → S-05: рекламодатель с реквизитами загружает два изображения (JPEG с EXIF-поворотом и PNG с прозрачностью), создаёт кампанию, сохраняет пост с заголовком, жирным, адресом в тексте и альбомом. Предпросмотр сначала даёт `media_processing`, после выполнения заданий — без проблем: в `media_urls` — производные с плашкой, в `final_text_sample` — erid под постом, ссылка с UTM-заглушкой.
    - Смена места токена на `text` + `above` → предпросмотр без плашки, строка маркировки сверху.
    - Удаление файла из медиатеки → он пропал из черновика.
    - Версия (вызов `CreateVersion` из теста как будущий этап): `body` с номером заказа, задания публикации дают публичные ссылки; `approve` → `MarkReady` с erid → `final_text` по 05 §2.5; правка черновика после версии не меняет версию.
  - Проверка журнала сборщиком записей: обязательные события 07 §5.3 этапа (действие пользователя, задание выполнено, смена состояния версии) присутствуют. Текст поста, адреса ссылок, название, ИНН и имя файла не встречаются.
  - Файлы: `internal/bootstrap/post_e2e_test.go`, `internal/bootstrap/testdata/media/*`.

- [x] **Task 16: Синхронизация документов с решениями этапа** (depends on 14)
  - После подтверждения R-1 … R-21 внести их в документы:
    - 02 S-04 — проблемы предпросмотра вместо запрета сохранения (R-11, R-13), распознавание ссылок (R-9), удаление файла из черновиков (R-15); S-05 — заглушка номера заказа (R-12), `media_processing` (R-6).
    - 04 §4 — новые поля (`id`, `created_at`, `status` и публичный ключ производной, `purged_at`), триггер неизменяемости (R-18), уникальность `sha256` (R-2).
    - 05 §2.5 — сверенные теги MAX (R-10), правила распознавания (R-9), подсчёт длины (R-11), место UTM-меток (R-12).
    - 06 §6.2 — формат производных из WebP (R-4), шаблон плашки (R-5).
    - 07 §2 — `publish_media_derivative`, `purge_media`, ключ `build_media_derivative` (R-6, R-19); §4 «Файлы» — пределы и анимация (R-2), поворот (R-3), публичная корзина (R-7); §8 — новые параметры (задача 14).
    - `docs/open-questions.md` — Q-06 (рабочий ответ реализован), Q-24 (каждое изображение), Q-05 (предел в конфигурации).
    - `.ai-factory/ARCHITECTURE.md` — пакет `post/imaging` как адаптер порта `Renderer`, порт `AdvertiserRequisites`, публичная область в `platform/storage`, кампании в модуле `post` (R-1). `.ai-factory/ROADMAP.md` — кампании перенесены из этапа «Заказ и размещения» (R-1).
  - Журнал: не применимо.
  - Файлы: `docs/02-scenarios.md`, `docs/04-domain-model.md`, `docs/05-integrations.md`, `docs/06-finance-legal.md`, `docs/07-non-functional.md`, `docs/open-questions.md`, `.ai-factory/ARCHITECTURE.md`, `.ai-factory/ROADMAP.md`.
<!-- Commit checkpoint: tasks 15-16 -->

## Критерии готовности этапа

- `go build ./...`, `go vet ./...`, `go test -race ./...` проходят локально и в CI. `sqlc generate` и `go generate ./...` не дают изменений. `mockapi` собирается и проходит свой контрактный тест.
- `docker compose -f compose.dev.yml up`: загрузка картинки, создание кампании, сохранение поста и предпросмотр показывают производную с плашкой из MinIO.
- Каждый элемент форматирования, каждое сочетание места токена и каждая причина отказа загрузки, документа и кампании покрыты табличными тестами `domain`.
- Оригинал не меняется и не доступен по публичной ссылке. Публичны только производные, вошедшие в версию (07 §4).
- Версия в `ready` и её изображения не меняются ни кодом, ни прямым запросом к базе (I-09).
- Повтор заданий `build_media_derivative`, `publish_media_derivative`, `purge_media` идемпотентен (07 §9).
- Текст поста, адреса ссылок, название и ИНН рекламодателя, имя файла отсутствуют в журнале.
- Решения R-1 … R-21 подтверждены и внесены в документы (задача 16).
