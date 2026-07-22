# [feat] Сущность Offer: CRUD предложений по турпутёвкам

> **GitHub issue:** [#12](https://github.com/tourismania/api/issues/12)
> **Зависит от:** [#11](https://github.com/tourismania/api/issues/11) (Agency)
> **Ревизия ТЗ:** переработанный текст после разбора нестыковок. Заменяет тело исходного issue и `offer_crud_spec.md` как источник истины.

## Контекст

Витрина предложений по турпутёвкам. Авторизованный турагент создаёт, редактирует и удаляет предложения (`offer`). Управлять предложением может любой сотрудник **того же агентства** (владение — по агентству, не по автору). Чтобы поделиться предложением с клиентом (неавторизованным пользователем), агент публикует его: опубликованный offer доступен по `uuid` без авторизации.

Базовые правила модели:

- **`1 пользователь = 1 агентство`**, поле `agency_id` у пользователя `NOT NULL` (миграция `014_add_users_agency`). Действует для **всех** ролей одинаково, включая `ROLE_SUPER_ADMIN`.
- **У авторизованного пользователя нет понятия «чужое агентство»** — он всегда работает только в пределах агентства, к которому принадлежит. Offer другого агентства для него на приватных ручках просто не существует (`404`), а не «запрещён».
- **Публичный доступ** (без токена) — агентства не касается вовсе: гостю виден любой `published` offer по `uuid`.
- **`ROLE_USER` — только чтение.** Сотрудник агентства без прав записи: видит все offer своего агентства (любой статус), но не может создавать/редактировать/удалять.
- **Запись (`POST`/`PATCH`/`DELETE`) — только `ROLE_AGENT` и `ROLE_SUPER_ADMIN`** в пределах своего агентства.
- **Смена статуса — свободная**, через тело запроса; машины состояний нет.

## Статусы (`OfferStatus`)

- `draft` — черновик.
- `ready` — заполнен и сохранён, но агент ещё не решил публиковать; видимость идентична `draft` (только внутри агентства).
- `published` — публично видим по `uuid`.

Переходы свободные: допустимы любые (создание сразу в `published`, снятие с публикации `published → draft/ready`). Валидация значения — `oneof=draft ready published`; недопустимое значение → `400`.

## Эндпоинты (что реализуем)

| # | Метод | Путь | Доступ | Успех |
|---|---|---|---|---|
| 1 | `POST` | `/api/v1/offers` | `ROLE_AGENT`/`ROLE_SUPER_ADMIN`, своё агентство | `201` |
| 2 | `GET` | `/api/v1/offers` | любой авторизованный (своё агентство) | `200` |
| 3 | `GET` | `/api/v1/offers/{uuid}` | любой авторизованный (своё агентство, любой статус) | `200` |
| 4 | `GET` | `/api/v1/public/offers/{uuid}` | публичный (без токена), только `published` | `200` |
| 5 | `PATCH` | `/api/v1/offers/{uuid}` | `ROLE_AGENT`/`ROLE_SUPER_ADMIN`, своё агентство | `200` |
| 6 | `DELETE` | `/api/v1/offers/{uuid}` | `ROLE_AGENT`/`ROLE_SUPER_ADMIN`, своё агентство | `204` |

Получение offer по `uuid` разделено на **две отдельные ручки** (N3 приватная и N4 публичная) с разными обработчиками, представлениями и query. Публичная ручка (N4) — это «ссылка, которой делятся с клиентом».

> Путь публичной ручки `/api/v1/public/offers/{uuid}` выбран под разделение роутера на публичную/приватную группы; при желании легко заменить (например `/api/v1/offers/{uuid}/public`).

## Матрица видимости

| Принципал | `POST` / `PATCH` / `DELETE` | `GET /offers` (список) | `GET /offers/{uuid}` (приватный) | `GET /public/offers/{uuid}` |
|---|---|---|---|---|
| Гость (без токена) | `401` | `401` | `401` | `published` → `200`, иначе `404` |
| `ROLE_USER` (своё агентство) | `403` | offer своего агентства, **все статусы** | offer своего агентства, любой статус; иначе `404` | `published` → `200`, иначе `404` |
| `ROLE_AGENT` / `ROLE_SUPER_ADMIN` (своё агентство) | `201`/`200`/`204` (только своё агентство) | offer своего агентства, **все статусы** | offer своего агентства, любой статус; иначе `404` | `published` → `200`, иначе `404` |

Представление: приватные ручки (N2, N3) → детальное DTO; публичная (N4) → публичное DTO.

Коды ошибок:

- **`401`** — нет/невалидный токен на приватной ручке.
- **`403`** — авторизован, но роль без права записи (`ROLE_USER` на `POST`/`PATCH`/`DELETE`) — отдаёт доменный `OfferManager` (`ErrInsufficientRole`), не HTTP-мидлварь.
- **`404`** — offer не найден **или** принадлежит не тому агентству, что у пользователя (существование чужого offer не раскрываем). То же для не-`published` на публичной ручке.

## Scope

### Домен (`internal/domain/`)

- `entity/offer.go` — `Offer{ ID, UUID, Title, Description, AgencyID, CreatedBy, Status, CreatedAt, UpdatedAt, DeletedAt }`. Метод `IsPublished()` (учитывает только `published`).
- `enum/offer_status.go` — `OfferStatus` (`draft` / `ready` / `published`).
- `repository/offer_repository.go` — интерфейс: `Store`, `FindByUUID`, `List(OfferFilter)`, `Update`, `SoftDelete`. `OfferFilter{ AgencyID, Status, CreatedBy, Limit, Offset }` (`AgencyID` заполняется слоями выше из identity, наружу как query-параметр не выставляется).
- `service/offer_manager.go` — единый `OfferManager{ Insert, Update, Delete }`: инварианты, проверка активности агентства, роль на запись (`ROLE_AGENT`/`ROLE_SUPER_ADMIN`, иначе `ErrInsufficientRole` — проверяется первым, до похода в репозиторий, т.к. не зависит от конкретного offer), владение по агентству (строгое равенство `offer.AgencyID == actor.AgencyID`, **без ветки для `ROLE_SUPER_ADMIN`**). При несовпадении агентства → `ErrOfferNotFound` (не `Forbidden`: чужой offer для пользователя не существует). Sentinel-ошибки: `ErrOfferNotFound`, `ErrInsufficientRole`, `ErrActorNotFound`, `ErrAgencyInactive`, `ErrOfferInvalid`, …
  - `valueobject/actor.go` — `Actor{ UserID int, AgencyID int, Roles []enum.Role }` вынесен из `domain/service` в `domain/valueobject`: это идентичность `entity.User`, а не деталь одного менеджера — переиспользуется любым доменным сервисом, которому нужно знать «кто выполняет операцию» (не только offers). `AgencyID` обязательный `int` (не `*int`), т.к. `agency_id` пользователя `NOT NULL`. `HasRole(role)` — метод value object'а.

### Infrastructure (`internal/infrastructure/persistence/postgres/`)

- `repository/offer_repository.go` — реализация `domain/repository`.
- `queries/offers.sql` (+ `make sqlc`) — все чтения фильтруют `deleted_at IS NULL`.
- `mapper/offer_mapper.go` — маппинг `model.Offer ↔ entity.Offer`.

### Application (CQRS, `internal/application/`)

Идентичность (`agency_id`, `roles`) больше не приходит из presentation готовой — Command/Query несут только неизменяемый `CurrentUserUUID uuid.UUID` (из `Claims.Subject`), а сам handler резолвит `Actor`/`agency_id` из БД через общий порт `application/identity.UserFinder` (`application/identity.Resolve`, переиспользуется всеми ниже + `get_me`). Не найден пользователь по `uuid` (аккаунт удалён после выдачи токена) → `service.ErrActorNotFound` (`401`).

**command:**

- `create_offer`, `update_offer`, `delete_offer`. `Command{ ..., CurrentUserUUID uuid.UUID }`. Handler резолвит `Actor` через `identity.Resolve` и передаёt его в `OfferManager` — роль и владение проверяет домен, не presentation.

**query:**

- `get_offers` — список для авторизованного пользователя. `Query{ CurrentUserUUID uuid.UUID, Status *OfferStatus, CreatedBy *int, Limit, Offset }`; результат — элементы + `TotalCount`. Handler резолвит `agency_id` из `CurrentUserUUID` и скоупит `agency_id = actor.AgencyID`, любые статусы. Роль на выборку не влияет (`ROLE_USER` и агенты видят одинаковый список своего агентства).
- `get_offer` — приватное получение одного offer по `uuid`. `Query{ UUID, CurrentUserUUID uuid.UUID }`. Возвращает offer любого статуса, если `offer.AgencyID == actor.AgencyID`; иначе `ErrOfferNotFound`.
- `get_published_offer` — **(новая)** публичное получение одного offer по `uuid`. `Query{ UUID }`, без identity вообще (даже uuid не нужен). Возвращает offer только если `published`; иначе `ErrOfferNotFound`.

### Presentation (HTTP, `internal/presentation/http/`)

Пакеты `api/v1/offers/*`.

- Тело `POST`/`PATCH`: только `title`, `description`, `status` (`agency_id` выводится из агентства текущего пользователя на сервере, в теле не принимается).
- Query-параметры списка (N2): `status` (`oneof=draft ready published`), `created_by`, `limit`, `offset`. Параметра `agency_id` нет — скоуп навязывается application-слоем из резолвнутого `Actor.AgencyID`.
- Регистрация маршрутов в `router.go`; роутер разделён на:
  - **публичную группу** (без auth) — `GET /api/v1/public/offers/{uuid}` (N4);
  - **приватную группу** (только `JWT`) — `GET /offers` (N2), `GET /offers/{uuid}` (N3), а также `POST`/`PATCH`/`DELETE` (N1, N5, N6). Нет отдельной write-подгруппы с ролевым guard'ом на уровне роутера — роль на запись проверяет доменный `OfferManager` (см. Домен выше).
- Хендлер достаёт из HTTP-запроса только `uuid` (`custommw.CurrentUserUUID(ctx)`, чистое чтение JWT claims, без похода в БД) и передаёт его в Command/Query; резолвинг `agency_id`/`roles` — задача application-слоя.
- Сборка зависимостей — в `config/container.go`.

### Авторизация (middleware)

- **Никакой мидлвари, резолвящей пользователя из БД, нет.** `custommw.JWT` валидирует токен и кладёт `Claims` (только `Subject` — uuid) в контекст; `custommw.CurrentUserUUID(ctx)` — чистое извлечение uuid из claims, без похода в БД. Резолвинг `agency_id`/`roles` из уже это uuid — обязанность application-слоя (`application/identity.Resolve`, см. Application выше), переиспользуется также в `get_me`.
- Роль на запись (`ROLE_AGENT`/`ROLE_SUPER_ADMIN`) проверяет не HTTP-guard, а доменный `OfferManager` — нет отдельного `RequireRole`-мидлваря для offers.
- Публичная ручка (N4) auth-middleware не использует (полностью анонимна). Отдельный `OptionalJWT`/`OptionalCurrentUser` не нужен — публичное и приватное чтение разнесены по разным эндпоинтам.
- `entity.UserRecord` дополняется полем `ID int` (внутренний numeric id) — нужно для `Actor.UserID` в identity.

### Миграция

- `015_create_offers` — одна миграция: таблица `offers` + индексы (`agency_id`, `status`, частичный `WHERE deleted_at IS NULL`) + FK `agency_id → agencies(id)` и FK `created_by → users(id)`. Создание таблицы со своими constraints/индексами — одно логическое изменение.

### Прочее

- Swagger — `make swag`.
- `README.md` — раздел **Endpoints** (6 ручек) + раздел **Роли и права** (матрица видимости).

## Acceptance criteria

- [x] Миграция 015 применяется и откатывается (проверено вживую: `migrate up`/`down 1`/`up` на Postgres 17 в Docker).
- [x] Эндпоинты N1–N6 работают с кодами `201`/`200`/`204`/`400`/`401`/`403`/`404`.
- [x] Гость: `POST/PATCH/DELETE` и `GET /offers`, `GET /offers/{uuid}` → `401`; `GET /public/offers/{uuid}` → `published` виден, не-`published` → `404`.
- [x] `ROLE_USER`: `GET /offers` и `GET /offers/{uuid}` — offer только своего агентства, все статусы; `POST/PATCH/DELETE` → `403`.
- [x] Агент/супер-админ: управляет и видит offer только своего агентства (все статусы); offer с `uuid` другого агентства на приватных ручках → `404` (без исключений по роли — `1 пользователь = 1 агентство`).
- [x] Получение по `uuid` разделено на приватную (N3) и публичную (N4) ручки с отдельными query (`get_offer` / `get_published_offer`).
- [x] Добавлены описание ролей в документацию
- [x] Добавлены описание статусов offer в документацию
- [x] Смена `status` свободная через тело; создание сразу в `published` и снятие с публикации работают; недопустимое значение статуса → `400`.
- [x] Soft delete: удалённые offer исключены из всех чтений (проверено integration-тестом `TestOfferRepository_SoftDelete_ExcludesFromFindByUUID` вживую на Postgres).
- [x] Идентичность резолвится application-слоем (`application/identity.Resolve`) по `uuid` из JWT — переиспользуется `create_offer`/`update_offer`/`delete_offer`/`get_offer`/`get_offers`/`get_me`; никакой мидлвари, ходящей в БД, нет.
- [x] Swagger сгенерирован (`make swag`); `README.md` (Endpoints + «Роли и права») обновлён.
- [x] Критический путь покрыт: unit `OfferManager`/`create_offer`/`update_offer`/`delete_offer`/`get_offer`/`get_offers`/`get_published_offer` (мок-репозитории, включая `ErrInsufficientRole`/`ErrActorNotFound`) + application e2e с реальными handler'ами (`401`/`403`/`201`/публичный `200`+`404`). `go test ./...` (включая `integration`, живой Postgres в Docker) — зелёные; `migrate up/down` для `015_create_offers` проверен вживую; `go build ./...`, `go vet ./...` — зелёные; `golangci-lint run ./...` без новых замечаний (1 не связанная с этим issue преэкзистирующая находка — deprecated `jwt.ParseRSAPrivateKeyFromPEMWithPassword`).

## Negative constraints (чего НЕ делаем)

- Нет Kafka-событий по offer, нет нечёткого поиска, нет бронирования/покупки.
- Нет машины состояний статусов (переходы свободные).
- Нет маркетплейс-витрины между агентствами: авторизованный пользователь видит только своё агентство; кросс-агентский доступ — только через публичную ссылку на `published`.
- Домен без внешних импортов; никаких `log.Fatal`/`os.Exit` вне `main()`; DI только в `config/container.go`.
- Файлы в `db/` и `docs/` (генерируемые) вручную не редактируются.
