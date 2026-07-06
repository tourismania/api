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
  - `internal/domain/entity/user.go` — добавить `AgencyID *int` (1 пользователь = 1 агентство).
- Репозиторий (1 сущность = 1 репозиторий):
  - `internal/domain/repository/agency_repository.go` — интерфейс `Store`, `FindByID`, `SetStatus`, `Exists`.
  - Реализация в `internal/infrastructure/persistence/postgres/repository/agency_repository.go` + `queries/agencies.sql` (`make sqlc`) + `mapper/agency_mapper.go`.
- Доменный сервис:
  - `internal/domain/service/agency_manager.go` — `AgencyManager` с `Create`, `Deactivate`.
- CLI (cobra), по аналогии с `internal/presentation/cli`:
  - `agency create --name "<name>"` → `AgencyManager.Create` (генерирует `uuid`, `created_at`, `status=active`), печатает `id`/`uuid`.
  - `agency deactivate --id <id>` → `AgencyManager.Deactivate` (`SetStatus(id, inactive)`).
- Регистрация пользователя (§6.3): `create_user` принимает `agency_id`:
  - `.../user/create/dto.go` — поле `agency_id` (`*int`).
  - `application/command/create_user/{command,handler}.go` — прокинуть `AgencyID`.
  - `domain/service/user_creator.go` — сохранять `AgencyID`; при заданном — проверять существование и активность агентства через `AgencyRepository`.
  - `repository/user_repository.go` + `queries/users.sql` — писать/читать `agency_id`.
- Миграции (1 действие = 1 миграция; таблица + её индексы вместе):
  - `012_create_agencies` — таблица `agencies (id, uuid UNIQUE, name, status DEFAULT 'active', created_at, deleted_at NULL)`.
  - `013_add_users_agency` — `ALTER TABLE users ADD COLUMN agency_id INT NULL REFERENCES agencies(id)`.
  - `014_seed_agencies` — 1–2 демо-агентства (seed, отдельной миграцией).
- Тесты: unit (`AgencyManager`, инварианты `Agency`/`AgencyStatus`), integration (`AgencyRepository`: Store/SetStatus/FindByID), application (регистрация с `agency_id`).
- Docs: `README.md` — раздел **CLI** (новые команды); при вопросах «зачем/почему» — `FAQ.md`; новые ENV (если появятся) — `.env.example`.

## Acceptance criteria

- [ ] Миграции 012–014 применяются и откатываются (`make migrate-up` / `migrate-down`).
- [ ] `Agency` имеет `uuid`, `status` (active/inactive), `created_at`, `deleted_at`; чтения фильтруют `deleted_at IS NULL`.
- [ ] Роль `ROLE_AGENT` добавлена в `enum.Role`.
- [ ] У `users` есть `agency_id`; регистрация (`create_user`) принимает и сохраняет `agency_id`.
- [ ] CLI `agency create` и `agency deactivate` работают через `AgencyManager`.
- [ ] Seed-агентства применяются.
- [ ] Публичные функции домена покрыты тестами; критический путь ≥ 90%.
- [ ] `README.md` (CLI) обновлён; `go test ./...`, `golangci-lint run`, `go build ./...` — зелёные.

## Negative constraints (чего НЕ делаем)

- Нет HTTP-CRUD агентств (только CLI) — вынесено в отдельный будущий issue.
- Нет Kafka-событий по агентству.
- Домен без внешних импортов (`pgx`/`chi`/`kafka`); никаких `log.Fatal`/`os.Exit` вне `main()`; DI только в `config/container.go`.
- Файлы в `db/` и `docs/` вручную не редактируются (генерируются).

## Открытый вопрос

- Обязателен ли `agency_id` при регистрации для всех, или только для агентов (клиент `ROLE_USER` — без агентства)?
