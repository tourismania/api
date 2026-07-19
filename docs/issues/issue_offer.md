# [feat] Сущность Offer: CRUD предложений по турпутёвкам

> **GitHub issue:** [#12](https://github.com/tourismania/api/issues/12)
> **Зависит от:** [#11](https://github.com/tourismania/api/issues/11) (Agency)

## Контекст

Витрина предложений: турагент публикует offer, клиент знакомится и решает о покупке. Итоговая цена в offer **не хранится** — будет считаться из дочерних сущностей (перелёты/отели/поездки) в отдельных issue. Владение — **по агентству**: агент управляет всеми offer своего агентства; супер-админ — всеми; клиент видит только `published`.

Полное ТЗ: `docs/specs/offer_crud_spec.md`.

> **Зависит от issue «Сущность Agency»** (нужны `agencies`, `users.agency_id`, `ROLE_AGENT`, `AgencyRepository`).

## Scope

- Домен:
  - `internal/domain/entity/offer.go` — `Offer{ ID, UUID, Title, Description, AgencyID, CreatedBy, Status, CreatedAt, UpdatedAt, DeletedAt }` (без цены).
  - `internal/domain/enum/offer_status.go` — `OfferStatus` (`draft` / `published`).
  - `internal/domain/repository/offer_repository.go` — `Store`, `FindByUUID`, `List(OfferFilter)`, `Update`, `SoftDelete`; `OfferFilter{ AgencyID, Status, CreatedBy, Limit, Offset }`.
  - `internal/domain/service/offer_manager.go` — единый `OfferManager{ Insert, Update, Delete }`: инварианты, проверка активности агентства, **владение по агентству** (`offer.AgencyID == currentAgencyID`, кроме `ROLE_SUPER_ADMIN`). Sentinel-ошибки `ErrOfferNotFound`, `ErrOfferForbidden`, `ErrAgencyInactive`, …
- Infrastructure:
  - `repository/offer_repository.go` + `queries/offers.sql` (`make sqlc`) + `mapper/offer_mapper.go`. Чтения фильтруют `deleted_at IS NULL`.
- Application (CQRS):
  - command: `create_offer`, `update_offer`, `delete_offer`.
  - query: `get_offer`, `get_offers` (пагинация + фильтры + `TotalCount`).
  - Identity вызывающего передаётся явно: `CurrentUserID int`, `CurrentAgencyID *int`, `CurrentRoles []enum.Role`.
- Presentation (HTTP), пакеты `internal/presentation/http/api/v1/offer/{create,get,get_list,update,delete}`:
  - `POST /api/v1/offers` (ROLE_AGENT, ROLE_SUPER_ADMIN)
  - `GET /api/v1/offers` (аутентиф.; видимость по роли)
  - `GET /api/v1/offers/{uuid}` (аутентиф.; видимость по роли)
  - `PATCH /api/v1/offers/{uuid}` (агент того же агентства или супер-админ)
  - `DELETE /api/v1/offers/{uuid}` (soft; агент того же агентства или супер-админ)
  - Тело `POST`/`PATCH`: только `title`, `description`, `status` (`agency_id` выводится из агентства агента на сервере).
  - Регистрация в `router.go`; сборка в `config/container.go`.
- Авторизация: общий переиспользуемый middleware **`CurrentUser`** (по `Claims.Subject` достаёт `User{ ID, Roles, AgencyID }` из БД, кладёт в контекст; переиспользуется и в `get_me`) + guard `RequireRole(...)`.
- Миграция: `015_create_offers` — таблица `offers` + индексы (`agency_id`, `status`, частичный `WHERE deleted_at IS NULL`) в одной миграции.
- Swagger (`make swag`), тесты, `README.md` (Endpoints).

## Видимость (read-side)

- `ROLE_SUPER_ADMIN` — все offer.
- `ROLE_AGENT` — все offer своего агентства (любой статус).
- `ROLE_USER` — только `status = published`.

## Acceptance criteria

- [x] Миграция 015 применяется/откатывается.
- [x] Эндпоинты `POST/GET/GET{uuid}/PATCH/DELETE /api/v1/offers` работают с кодами 201/200/204/400/401/403/404.
- [x] Владение по агентству: агент управляет всеми offer своего агентства; чужое агентство → 403; супер-админ → все.
- [x] Клиент (`ROLE_USER`) видит только `published`.
- [x] Soft delete: удалённые offer исключены из чтений.
- [x] Общий `CurrentUser` resolver-middleware внедрён и переиспользуется (также в `get_me`).
- [x] Swagger сгенерирован (`make swag`); `README.md` (Endpoints + новый раздел «Роли и права») обновлён.
- [x] Критический путь (создание + авторизация по агентству) покрыт unit-тестами `OfferManager`/`get_offer`/`get_offers` (мокированные репозитории) + integration-тестами `OfferRepository` + application e2e (401/403/201); `go test ./...`, `go build ./...` — зелёные. `golangci-lint run` не выявил замечаний в новом коде (6 предсуществующих замечаний в несвязанных файлах).

## Заметки по реализации

- Видимость `ROLE_USER` реализована **без** ограничения по агентству (клиент видит `published` offer любого агентства — витрина-маркетплейс), в соответствии с полным ТЗ (`docs/issues/offer_crud_spec.md`, §6). Тело исходного GitHub issue содержало формулировку «своего агенства» для `ROLE_USER`, которая противоречит полному ТЗ; выбран вариант из полного ТЗ как источник истины.
- `entity.UserRecord` дополнен полем `ID int` (внутренний numeric id) — требовалось для `CurrentUserID` в identity, которую резолвит `CurrentUser` middleware.
- Файлы в `internal/infrastructure/persistence/postgres/db/` в этом репозитории **хэнд-мейд, имитирующие вывод sqlc** (реальный `sqlc generate` даёт другую типизацию — `pgtype.*` вместо `uuid.UUID`/`*string`); `offers.sql.go` написан вручную в том же стиле, что и `agencies.sql.go`.

## Negative constraints (чего НЕ делаем)

- Нет цены в offer и дочерних сущностей (перелёты/отели/поездки) — отдельные issue.
- Нет Kafka-событий по offer, нет нечёткого поиска, нет бронирования/покупки.
- Домен без внешних импортов; никаких `log.Fatal`/`os.Exit` вне `main()`; DI только в `config/container.go`.
- Файлы в `db/` и `docs/` вручную не редактируются.

## Roadmap (отдельные будущие issue)

- Дочерние сущности `OfferFlight`, `OfferHotel`, `OfferTrip` (нормализованно, не JSON).
- Вычисление итоговой цены offer из дочерних сущностей.
- Полный HTTP-CRUD агентств.
