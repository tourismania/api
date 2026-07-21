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
- **`403`** — авторизован, но роль без права записи (`ROLE_USER` на `POST`/`PATCH`/`DELETE`) — отдаёт `RequireRole`.
- **`404`** — offer не найден **или** принадлежит не тому агентству, что у пользователя (существование чужого offer не раскрываем). То же для не-`published` на публичной ручке.

## Scope

### Домен (`internal/domain/`)

- `entity/offer.go` — `Offer{ ID, UUID, Title, Description, AgencyID, CreatedBy, Status, CreatedAt, UpdatedAt, DeletedAt }`. Метод `IsPublished()` (учитывает только `published`).
- `enum/offer_status.go` — `OfferStatus` (`draft` / `ready` / `published`).
- `repository/offer_repository.go` — интерфейс: `Store`, `FindByUUID`, `List(OfferFilter)`, `Update`, `SoftDelete`. `OfferFilter{ AgencyID, Status, CreatedBy, Limit, Offset }` (`AgencyID` заполняется слоями выше из identity, наружу как query-параметр не выставляется).
- `service/offer_manager.go` — единый `OfferManager{ Insert, Update, Delete }`: инварианты, проверка активности агентства, владение по агентству (строгое равенство `offer.AgencyID == actor.AgencyID`, **без ветки для `ROLE_SUPER_ADMIN`**). При несовпадении агентства → `ErrOfferNotFound` (не `Forbidden`: чужой offer для пользователя не существует). Sentinel-ошибки: `ErrOfferNotFound`, `ErrAgencyInactive`, `ErrOfferInvalid`, …
  - `service.Actor{ UserID int, AgencyID int }` — `AgencyID` обязательный `int` (не `*int`), т.к. `agency_id` пользователя `NOT NULL`. Роль на write-ownership не влияет (гейт по роли — в presentation через `RequireRole`), поэтому `Roles` в `Actor` для write не нужны. Метода `IsSuperAdmin()` нет.

### Infrastructure (`internal/infrastructure/persistence/postgres/`)

- `repository/offer_repository.go` — реализация `domain/repository`.
- `queries/offers.sql` (+ `make sqlc`) — все чтения фильтруют `deleted_at IS NULL`.
- `mapper/offer_mapper.go` — маппинг `model.Offer ↔ entity.Offer`.

### Application (CQRS, `internal/application/`)

**command:**

- `create_offer`, `update_offer`, `delete_offer`. Identity write-side передаётся явно: `CurrentUserID int`, `AgencyID int` (обязательный, заполняется presentation-слоем **строго** из `CurrentUser.AgencyID`, другого источника нет). Поле ролей в команде не нужно.

**query:**

- `get_offers` — список для авторизованного пользователя. `Query{ AgencyID int, Status *OfferStatus, CreatedBy *int, Limit, Offset }`; результат — элементы + `TotalCount`. Всегда скоуп `agency_id = AgencyID`, любые статусы. Роль на выборку не влияет (`ROLE_USER` и агенты видят одинаковый список своего агентства).
- `get_offer` — приватное получение одного offer по `uuid`. `Query{ UUID, AgencyID int }`. Возвращает offer любого статуса, если `offer.AgencyID == AgencyID`; иначе `ErrOfferNotFound`.
- `get_published_offer` — **(новая)** публичное получение одного offer по `uuid`. `Query{ UUID }`, без identity. Возвращает offer только если `published`; иначе `ErrOfferNotFound`.

### Presentation (HTTP, `internal/presentation/http/`)

Пакеты `api/v1/offers/*`.

- Тело `POST`/`PATCH`: только `title`, `description`, `status` (`agency_id` выводится из агентства текущего пользователя на сервере, в теле не принимается).
- Query-параметры списка (N2): `status` (`oneof=draft ready published`), `created_by`, `limit`, `offset`. Параметра `agency_id` нет — скоуп навязывается из `CurrentUser.AgencyID`.
- Регистрация маршрутов в `router.go`; роутер разделён на:
  - **публичную группу** (без auth) — `GET /api/v1/public/offers/{uuid}` (N4);
  - **приватную группу** (`JWT` + `CurrentUser`) — `GET /offers` (N2), `GET /offers/{uuid}` (N3);
  - внутри приватной — **write-подгруппа** с `RequireRole(ROLE_AGENT, ROLE_SUPER_ADMIN)`: `POST`/`PATCH`/`DELETE` (N1, N5, N6).
- Сборка зависимостей — в `config/container.go`.

### Авторизация (middleware)

- **`CurrentUser`** — переиспользуемый resolver: по `Claims.Subject` достаёт `User{ ID, Roles, AgencyID }` из БД **через query из слоя Application**, кладёт в контекст. Переиспользуется также в `get_me`.
- **`RequireRole(...)`** — guard для write-эндпоинтов.
- Публичная ручка (N4) auth-middleware не использует (полностью анонимна). Отдельный `OptionalJWT`/`OptionalCurrentUser` не нужен — публичное и приватное чтение разнесены по разным эндпоинтам.
- `entity.UserRecord` дополняется полем `ID int` (внутренний numeric id) — нужно для `CurrentUserID` в identity.

### Миграция

- `015_create_offers` — одна миграция: таблица `offers` + индексы (`agency_id`, `status`, частичный `WHERE deleted_at IS NULL`) + FK `agency_id → agencies(id)` и FK `created_by → users(id)`. Создание таблицы со своими constraints/индексами — одно логическое изменение.

### Прочее

- Swagger — `make swag`.
- `README.md` — раздел **Endpoints** (6 ручек) + раздел **Роли и права** (матрица видимости).

## Acceptance criteria

- [ ] Миграция 015 применяется и откатывается.
- [ ] Эндпоинты N1–N6 работают с кодами `201`/`200`/`204`/`400`/`401`/`403`/`404`.
- [ ] Гость: `POST/PATCH/DELETE` и `GET /offers`, `GET /offers/{uuid}` → `401`; `GET /public/offers/{uuid}` → `published` виден, не-`published` → `404`.
- [ ] `ROLE_USER`: `GET /offers` и `GET /offers/{uuid}` — offer только своего агентства, все статусы; `POST/PATCH/DELETE` → `403`.
- [ ] Агент/супер-админ: управляет и видит offer только своего агентства (все статусы); offer с `uuid` другого агентства на приватных ручках → `404` (без исключений по роли — `1 пользователь = 1 агентство`).
- [ ] Получение по `uuid` разделено на приватную (N3) и публичную (N4) ручки с отдельными query (`get_offer` / `get_published_offer`).
- [ ] Добавлены описание ролей в документацию
- [ ] Добавлены описание статусов offer в документацию
- [ ] Смена `status` свободная через тело; создание сразу в `published` и снятие с публикации работают; недопустимое значение статуса → `400`.
- [ ] Soft delete: удалённые offer исключены из всех чтений.
- [ ] Общий `CurrentUser` resolver-middleware внедрён и переиспользуется (в т.ч. в `get_me`).
- [ ] Swagger сгенерирован (`make swag`); `README.md` (Endpoints + «Роли и права») обновлён.
- [ ] Критический путь покрыт: unit `OfferManager`/`get_offer`/`get_offers`/`get_published_offer` (мок-репозитории) + integration `OfferRepository` + application e2e (`401`/`403`/`201`/публичный `200`+`404`). `go test ./...`, `go build ./...` — зелёные; `golangci-lint run` без новых замечаний.

## Negative constraints (чего НЕ делаем)

- Нет Kafka-событий по offer, нет нечёткого поиска, нет бронирования/покупки.
- Нет машины состояний статусов (переходы свободные).
- Нет маркетплейс-витрины между агентствами: авторизованный пользователь видит только своё агентство; кросс-агентский доступ — только через публичную ссылку на `published`.
- Домен без внешних импортов; никаких `log.Fatal`/`os.Exit` вне `main()`; DI только в `config/container.go`.
- Файлы в `db/` и `docs/` (генерируемые) вручную не редактируются.
