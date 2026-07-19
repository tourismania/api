# [feat] Сущность Agency: справочник агентств, привязка пользователя, CLI

> **GitHub issue:** [#11](https://github.com/tourismania/api/issues/11)
> **Блокирует:** [#12](https://github.com/tourismania/api/issues/12) (Offer)

## Контекст

Фундамент для витрины предложений (offer): турагент (`ROLE_AGENT`) принадлежит агентству по правилу **1 пользователь = 1 агентство**, и предложения принадлежат агентству. Эта задача вводит сущность агентства, привязку пользователей к агентствам, управление агентствами через CLI и доработку регистрации.

Полное ТЗ: `docs/specs/offer_crud_spec.md` (§3.3–3.5, §4.2, §5, §6.3, §8, §9).

> Блокирует issue «Сущность Offer (CRUD)».

## Scope

- Доменные сущности/enum:
  - `internal/domain/entity/agency.go` — `Agency{ ID, UUID, Name, Status, CreatedAt, DeletedAt }`.
  - `internal/domain/enum/agency_status.go` — `AgencyStatus` (`active` / `inactive`).
  - `internal/domain/enum/role.go` — добавить `RoleAgent = "ROLE_AGENT"`.
  - `internal/domain/entity/user.go` — добавить `AgencyID int` (1 пользователь = 1 агентство, обязательное поле).
- Репозиторий (1 сущность = 1 репозиторий):
  - `internal/domain/repository/agency_repository.go` — интерфейс `Store`, `FindByID`, `SetStatus`, `Exists`.
  - Реализация в `internal/infrastructure/persistence/postgres/repository/agency_repository.go` + `queries/agencies.sql` (`make sqlc`) + `mapper/agency_mapper.go`.
- Доменный сервис:
  - `internal/domain/service/agency_manager.go` — `AgencyManager` с `Create`, `Deactivate`, `Activate`.
- CLI (cobra), по аналогии с `internal/presentation/cli`:
  - `agency create --name "<name>"` → `AgencyManager.Create` (генерирует `uuid`, `created_at`, `status=active`), печатает `id`/`uuid`.
  - `agency deactivate --id <id>` → `AgencyManager.Deactivate` (`SetStatus(id, inactive)`).
  - `agency activate --id <id>` → `AgencyManager.Activate` (`SetStatus(id, active)`).
  - Единый стандарт именования команд `<ресурс> <действие>` (см. review [#4730807556](https://github.com/tourismania/api/pull/13#pullrequestreview-4730807556)): бывший `create-user` переименован в `user create --agency-id <id>`, зафиксировано в `CLAUDE.md`/`README.md`.
- Регистрация пользователя (§6.3): `create_user` принимает `agency_id`:
  - `.../user/create/dto.go` — поле `agency_id` (`int`, `validate:"required,gt=0"`).
  - `application/command/create_user/{command,handler}.go` — прокинуть `AgencyID` (обязательное поле).
  - `domain/service/user_creator.go` — сохранять `AgencyID`; всегда проверять существование и активность агентства через `AgencyRepository`.
  - `repository/user_repository.go` + `queries/users.sql` — писать/читать `agency_id`.
- Миграции (1 действие = 1 миграция; таблица + её индексы вместе):
  - `012_create_agencies` — таблица `agencies (id, uuid UNIQUE, name, status DEFAULT 'active', created_at, deleted_at NULL)`.
  - `013_seed_agencies` — сеет два агентства: `ДЕМО` и `ТУРИЗМАНИЯ`. Выполняется **до** `014`, чтобы `id` агентства `ТУРИЗМАНИЯ` уже существовал на момент backfill'а.
  - `014_add_users_agency` — `ALTER TABLE users ADD COLUMN agency_id INT NULL REFERENCES agencies(id)`, затем `UPDATE users SET agency_id = (SELECT id FROM agencies WHERE name = 'ТУРИЗМАНИЯ') WHERE agency_id IS NULL` (id ищется по имени, а не хардкодится и не проставляется через DB-level `DEFAULT`), затем `ALTER COLUMN agency_id SET NOT NULL`. Колонка **не** остаётся `NULL`-able — см. пересмотр в «Открытый вопрос» ниже.
- Тесты: unit (`AgencyManager`, инварианты `Agency`/`AgencyStatus`), integration (`AgencyRepository`: Store/SetStatus/FindByID), application (регистрация с `agency_id`).
- Docs: `README.md` — раздел **CLI** (новые команды); при вопросах «зачем/почему» — `FAQ.md`; новые ENV (если появятся) — `.env.example`.

## Acceptance criteria

- [x] Миграции 012–014 применяются и откатываются (`make migrate-up` / `migrate-down`) — проверено локально через `migrate up`/`down 3` против реального Postgres 17.
- [x] `Agency` имеет `uuid`, `status` (active/inactive), `created_at`, `deleted_at`; чтения фильтруют `deleted_at IS NULL`.
- [x] Роль `ROLE_AGENT` добавлена в `enum.Role`.
- [x] У `users` есть `agency_id`; регистрация (`create_user`) принимает и сохраняет обязательный `agency_id`.
- [x] CLI `agency create`, `agency deactivate` и `agency activate` работают через `AgencyManager` — проверено вручную против реального Postgres.
- [x] Seed-агентства применяются.
- [x] Публичные функции домена покрыты тестами (unit: `Agency.IsActive`, `AgencyManager` (`Create`/`Deactivate`/`Activate`); integration: `AgencyRepository`, `UserCreator` + agency).
- [x] `README.md` (CLI) обновлён; `go test ./...`, `go build ./...` — зелёные. `golangci-lint` недоступен в текущем окружении (не установлен) — не запускался.

## Реализация (заметки)

- `db/` (sqlc-shaped код) в этом проекте **не генерируется** реальным `sqlc generate` — файлы поддерживаются вручную в стиле, который sqlc произвёл бы (см. комментарий в `db.go`). `agencies.sql.go` и правки `users.sql.go` сделаны тем же способом; `queries/*.sql` остаются источником истины для схемы запроса.
- HTTP-хендлер `create_user` возвращает `400` (не `500`) на `ErrAgencyNotFound`/`ErrAgencyInactive` — это ошибки валидации входных данных, а не внутреннего состояния.
- Kafka недоступна в среде разработки для сквозной проверки (`bitnami/kafka:3.7` снят с Docker Hub) — путь `agency_id` в `UserCreator` проверен integration-тестами с in-memory `event.Bus`, а не полным docker-compose стеком.
- По итогам review [#4730807556](https://github.com/tourismania/api/pull/13#pullrequestreview-4730807556): `GET /api/v1/users/me` теперь возвращает `agency` (`id`, `uuid`, `name`) — `getme.Handler` получил порт `AgencyFinder` (переиспользует `AgencyRepository.FindByID`), `Result`/`GetMeResponse` расширены полем `Agency`.
- Заодно унифицирован под `<ресурс> <действие>` и CLI `sync-airports`, пропущенный в первом проходе переименования: теперь `airports sync` (`internal/presentation/cli/airports.go`).

## Открытый вопрос

- ~~Обязателен ли `agency_id` при регистрации для всех, или только для агентов (клиент `ROLE_USER` — без агентства)?~~
  **Решено (пересмотрено по итогам review [#4698705993](https://github.com/tourismania/api/pull/13#pullrequestreview-4698705993)):** `agency_id` обязателен для каждого пользователя вне зависимости от роли — `int`, не `*int`, `validate:"required,gt=0"` в DTO. `UserCreator.Create` всегда проверяет существование и активность агентства (`ErrAgencyNotFound` / `ErrAgencyInactive`). Роли при регистрации сейчас не выбираются — `create_user` всегда назначает `ROLE_USER`; выбор роли `ROLE_AGENT` остаётся вне scope этой задачи и будет закреплён в будущей задаче по регистрации агентов.
- ~~Оставлять ли `users.agency_id` `NULL`-able на уровне БД?~~
  **Решено (пересмотрено по итогам review [#4730807556](https://github.com/tourismania/api/pull/13#pullrequestreview-4730807556)):** нет, колонка `NOT NULL`. Миграции `013_seed_agencies`/`014_add_users_agency` сеют агентства `ДЕМО` и `ТУРИЗМАНИЯ`, backfill'ят все существующие строки `users` на `ТУРИЗМАНИЯ` (поиск `id` по имени, без хардкода и без DB-level `DEFAULT`) и затем ставят `SET NOT NULL`. Ранее заведённая запись в тех.долге про nullable-колонку снята как реализованная — см. `docs/tech_debt/tasks.md`.

## Negative constraints (чего НЕ делаем)

- Нет HTTP-CRUD агентств (только CLI) — вынесено в отдельный будущий issue.
- Нет Kafka-событий по агентству.
- Домен без внешних импортов (`pgx`/`chi`/`kafka`); никаких `log.Fatal`/`os.Exit` вне `main()`; DI только в `config/container.go`.
- Файлы в `db/` и `docs/` вручную не редактируются (генерируются).
