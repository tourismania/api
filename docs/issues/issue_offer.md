# [feat] Сущность Offer: CRUD предложений по турпутёвкам

> **GitHub issue:** [#12](https://github.com/tourismania/api/issues/12)
> **Зависит от:** [#11](https://github.com/tourismania/api/issues/11) (Agency)

## Контекст

Витрина предложений: турагент публикует offer, клиент знакомится и решает о покупке. Итоговая цена в offer **не хранится** — будет считаться из дочерних сущностей (перелёты/отели/поездки) в отдельных issue. Владение — **по агентству**: агент (или супер-админ) управляет только offer **своего** агентства — 1 пользователь = 1 агентство касается всех ролей одинаково, кросс-агентского доступа нет ни у кого. Опубликованный (`published`) offer виден любому, включая неавторизованных пользователей.

Полное ТЗ: `docs/specs/offer_crud_spec.md`.

> **Зависит от issue «Сущность Agency»** (нужны `agencies`, `users.agency_id`, `ROLE_AGENT`, `AgencyRepository`).

## Scope

- Домен:
  - `internal/domain/entity/offer.go` — `Offer{ ID, UUID, Title, Description, AgencyID, CreatedBy, Status, CreatedAt, UpdatedAt, DeletedAt }` (без цены).
  - `internal/domain/enum/offer_status.go` — `OfferStatus` (`draft` / `published`).
  - `internal/domain/repository/offer_repository.go` — `Store`, `FindByUUID`, `List(OfferFilter)`, `Update`, `SoftDelete`; `OfferFilter{ AgencyID, Status, CreatedBy, Limit, Offset }`.
  - `internal/domain/service/offer_manager.go` — единый `OfferManager{ Insert, Update, Delete }`: инварианты, проверка активности агентства, **владение по агентству** (`offer.AgencyID == actor.AgencyID`, строгое равенство, **без исключений для `ROLE_SUPER_ADMIN`** — 1 пользователь = 1 агентство). Sentinel-ошибки `ErrOfferNotFound`, `ErrOfferForbidden`, `ErrAgencyInactive`, …
- Infrastructure:
  - `repository/offer_repository.go` + `queries/offers.sql` (`make sqlc`) + `mapper/offer_mapper.go`. Чтения фильтруют `deleted_at IS NULL`.
- Application (CQRS):
  - command: `create_offer`, `update_offer`, `delete_offer`.
  - query: `get_offer`, `get_offers` (пагинация + фильтры + `TotalCount`).
  - Identity вызывающего передаётся явно: write-side — `CurrentUserID int`, `AgencyID int` (обязательный, берётся из `CurrentUser`, без отдельного "Current"-префикса); read-side — `CurrentAgencyID *int` (`nil` = гость/не сотрудник агентства — используется для публичных GET).
- Presentation (HTTP), пакеты `internal/presentation/http/api/v1/offer/{create,get,get_list,update,delete}`:
  - `POST /api/v1/offers` (ROLE_AGENT, ROLE_SUPER_ADMIN; только своё агентство)
  - `GET /api/v1/offers` (**публичный**, JWT опционален; видимость — см. ниже)
  - `GET /api/v1/offers/{uuid}` (**публичный**, JWT опционален; видимость — см. ниже)
  - `PATCH /api/v1/offers/{uuid}` (агент/супер-админ того же агентства)
  - `DELETE /api/v1/offers/{uuid}` (soft; агент/супер-админ того же агентства)
  - Тело `POST`/`PATCH`: только `title`, `description`, `status` (`agency_id` выводится из агентства текущего пользователя на сервере).
  - Регистрация в `router.go`; сборка в `config/container.go`.
- Авторизация: общий переиспользуемый middleware **`CurrentUser`** (по `Claims.Subject` достаёт `User{ ID, Roles, AgencyID }` из БД, кладёт в контекст; переиспользуется и в `get_me`) + guard `RequireRole(...)` для write-эндпоинтов + **`OptionalJWT`/`OptionalCurrentUser`** для публичных read-эндпоинтов offer (резолвят принципала, если токен есть, иначе продолжают как гость).
- Миграция: `015_create_offers` — таблица `offers` + индексы (`agency_id`, `status`, частичный `WHERE deleted_at IS NULL`) в одной миграции.
- Swagger (`make swag`), тесты, `README.md` (Endpoints).

## Видимость (read-side)

`GET /api/v1/offers*` — публичные эндпоинты (JWT не обязателен):

- Опубликованный (`status=published`) offer виден **любому**, включая неавторизованных пользователей.
- Аутентифицированный сотрудник агентства (`ROLE_AGENT` или `ROLE_SUPER_ADMIN`) дополнительно видит offer **своего** агентства в любом статусе; фильтр `agency_id` в этом случае принудительно выставляется в собственное агентство — кросс-агентского доступа нет даже у `ROLE_SUPER_ADMIN`.
- `ROLE_USER`-клиенты видят только `published`, независимо от своего `agency_id` (наличие `agency_id` у клиента — административный атрибут регистрации, не даёт видимости чужих черновиков).

## Acceptance criteria

- [x] Миграция 015 применяется/откатывается.
- [x] Эндпоинты `POST/GET/GET{uuid}/PATCH/DELETE /api/v1/offers` работают с кодами 201/200/204/400/401/403/404.
- [x] Владение по агентству: агент/супер-админ управляет только offer своего агентства; чужое агентство → 403 (без исключений по роли — 1 пользователь = 1 агентство).
- [x] Клиент (`ROLE_USER`) и неавторизованный пользователь видят только `published`; `GET /api/v1/offers*` — публичные эндпоинты.
- [x] Soft delete: удалённые offer исключены из чтений.
- [x] Общий `CurrentUser` resolver-middleware внедрён и переиспользуется (также в `get_me`).
- [x] Swagger сгенерирован (`make swag`); `README.md` (Endpoints + новый раздел «Роли и права») обновлён.
- [x] Критический путь (создание + авторизация по агентству) покрыт unit-тестами `OfferManager`/`get_offer`/`get_offers` (мокированные репозитории) + integration-тестами `OfferRepository` + application e2e (401/403/201); `go test ./...`, `go build ./...` — зелёные. `golangci-lint run` не выявил замечаний в новом коде (6 предсуществующих замечаний в несвязанных файлах).

## Заметки по реализации

- Видимость `ROLE_USER` реализована **без** ограничения по агентству (клиент видит `published` offer любого агентства — витрина-маркетплейс), в соответствии с полным ТЗ (`docs/issues/offer_crud_spec.md`, §6). Тело исходного GitHub issue содержало формулировку «своего агенства» для `ROLE_USER`, которая противоречит полному ТЗ; выбран вариант из полного ТЗ как источник истины.
- `entity.UserRecord` дополнен полем `ID int` (внутренний numeric id) — требовалось для `CurrentUserID` в identity, которую резолвит `CurrentUser` middleware.
- Файлы в `internal/infrastructure/persistence/postgres/db/` в этом репозитории **хэнд-мейд, имитирующие вывод sqlc** (реальный `sqlc generate` даёт другую типизацию — `pgtype.*` вместо `uuid.UUID`/`*string`); `offers.sql.go` написан вручную в том же стиле, что и `agencies.sql.go`.

### Правки по итогам code review PR #16

1. **`GET /api/v1/offers*` сделаны публичными.** Изначально обе ручки чтения требовали JWT — ревьюер указал, что `published` offer должен быть виден любому неавторизованному пользователю. Добавлены `middleware.OptionalJWT`/`middleware.OptionalCurrentUser` (парсят токен, если он есть, иначе продолжают как гость); роутер разделён на публичную группу (offer-чтения) и приватную (всё остальное, включая offer-записи).
2. **Убран кросс-агентский bypass для `ROLE_SUPER_ADMIN`.** `1 пользователь = 1 агентство` касается всех ролей одинаково — супер-админ не может управлять/видеть offer чужих агентств "просто потому что супер-админ". Метод `Actor.IsSuperAdmin()` и связанная ветка в `OfferManager.findOwned` удалены; владение — строгое равенство `offer.AgencyID == actor.AgencyID`. На read-side элевированную видимость (черновики своего агентства) сохраняет только принадлежность к агентству + роль `ROLE_AGENT`/`ROLE_SUPER_ADMIN` (иначе customer с тем же `agency_id` видел бы чужие черновики).
3. **`AgencyID` в `Actor`/командах стал обязательным `int`** (было `*int`) — ревьюер указал, что `agency_id` пользователя NOT NULL в БД (миграция `014_add_users_agency`), поэтому моделировать его как опциональный было ошибкой. Затронуты: `service.Actor`, `create_offer/update_offer/delete_offer.Command` (поле переименовано `CurrentAgencyID` → `AgencyID`, `CurrentRoles` убрано как избыточное — write-side ownership больше не завязан на роли), `middleware.CurrentUser.AgencyID`. Read-side (`get_offer/get_offers.Query.CurrentAgencyID`) сознательно остался `*int`: `nil` здесь означает не "нет агентства", а "гостевой запрос без сотрудника агентства" — легитимная семантика для публичных ручек.
4. **Command больше не принимает отдельный `CurrentAgencyID`, отличный от авторизованного пользователя** — `AgencyID` в команде теперь заполняется presentation-слоем строго из `CurrentUser.AgencyID` (`middleware.CurrentUserFromContext`), другого источника нет.
5. **Дополнительный статус offer между `draft` и `published`** — запрошен ревьюером ("не черновик, результат сохранён, но не готов к публикации"); требует уточнения названия/семантики, вынесено на отдельное обсуждение с автором задачи перед реализацией.

## Negative constraints (чего НЕ делаем)

- Нет цены в offer и дочерних сущностей (перелёты/отели/поездки) — отдельные issue.
- Нет Kafka-событий по offer, нет нечёткого поиска, нет бронирования/покупки.
- Домен без внешних импортов; никаких `log.Fatal`/`os.Exit` вне `main()`; DI только в `config/container.go`.
- Файлы в `db/` и `docs/` вручную не редактируются.

## Roadmap (отдельные будущие issue)

- Дочерние сущности `OfferFlight`, `OfferHotel`, `OfferTrip` (нормализованно, не JSON).
- Вычисление итоговой цены offer из дочерних сущностей.
- Полный HTTP-CRUD агентств.
