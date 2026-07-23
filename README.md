# Tourismania API (Go)

REST API сервис управления пользователями на Go 1.26, со строгим разделением слоёв (Clean Architecture / DDD).

## Стек

- **Go 1.26+**
- **chi v5** — HTTP роутер
- **pgx/v5 + sqlc** — PostgreSQL 17
- **golang-migrate** — миграции 
- **golang-jwt v5** — JWT (RS256)
- **go-playground/validator v10** — валидация
- **segmentio/kafka-go** — публикация доменных событий
- **swaggo/swag** — OpenAPI/Swagger
- **spf13/cobra** — CLI
- **golang.org/x/crypto/bcrypt** — хеш паролей

## Структура проекта

```
cmd/
  server/     # HTTP-сервер
  cli/        # CLI (cobra)
internal/
  domain/         # Доменный слой (entity, enum, event, factory, repository, service, valueobject)
  application/    # Use cases (command/query, command/query bus)
  infrastructure/ # Реализации интерфейсов домена (postgres, kafka, jwt, bcrypt)
  presentation/   # HTTP, CLI, DTO
migrations/       # SQL up/down миграции
config/           # config.go + JWT-ключи + container
tests/            # unit / integration / application
```

Направление зависимостей: `Presentation → Application → Domain ← Infrastructure`.

## Быстрый старт

### Через docker-compose

```bash
cp .env.example .env

# Сгенерировать JWT-ключи (если ещё не сделано):
make jwt-keys

# Поднять всё одной командой.
# Миграции запускаются автоматически внутри app-контейнера (entrypoint.sh)
# до старта сервера. Если они упали — контейнер рестартует.
docker compose up
```

## CLI

**Конвенция именования:** все команды следуют паттерну `<ресурс> <действие>` (например `user create`, `agency create`) — единый стандарт для всех CLI-команд проекта, вместо разрозненного `<действие>-<ресурс>`.

```bash
# локально
go run ./cmd/cli user create "Ada" "Lovelace" ada@example.com secret --agency-id 1
# → User successfully generated! id=1

# production
docker compose exec tourismania_app /app/cli user create "first_name" "last_name" email@example.com password --agency-id 1
```

### airports

Синхронизирует аэропорты, города и страны из внешних источников в БД.
Загружает данные из [mwgg/Airports](https://github.com/mwgg/Airports) (GitHub JSON)
и обогащает их русскоязычными названиями через Wikidata SPARQL.
Ожидаемое время выполнения: ~3–4 минуты для ~40 000 аэропортов.

Подробное описание алгоритма (источники данных, устройство запросов к Wikidata, тайминги, известные ограничения): [docs/cli/sync_airports.md](docs/cli/sync_airports.md).

```bash
# Preview без записи в БД
go run ./cmd/cli airports sync --dry-run

# Полная синхронизация (запускать раз в месяц)
go run ./cmd/cli airports sync

# production
docker compose exec tourismania_app /app/cli airports sync
```

### agency

Управление справочником агентств (только через CLI — HTTP CRUD агентств вне scope текущей итерации).

```bash
# Создать агентство (генерирует uuid, status=active, created_at)
go run ./cmd/cli agency create --name "Acme Travel"
# → Agency successfully created! id=1 uuid=...

# Деактивировать агентство (status → inactive)
go run ./cmd/cli agency deactivate --id 1
# → Agency successfully deactivated! id=1

# Активировать агентство обратно (status → active)
go run ./cmd/cli agency activate --id 1
# → Agency successfully activated! id=1

# production
docker compose exec tourismania_app /app/cli agency create --name "Acme Travel"
docker compose exec tourismania_app /app/cli agency deactivate --id 1
docker compose exec tourismania_app /app/cli agency activate --id 1
```

## Endpoints

| Метод | Путь             | Доступ | Описание                         |
| ----- |------------------|--------| -------------------------------- |
| POST  | /api/login       | public | Логин, возвращает JWT            |
| POST  | /api/v1/users    | JWT    | Создание пользователя (обязательный `agency_id` — привязка к агентству) |
| GET   | /api/v1/users/me | JWT    | Профиль текущего пользователя    |
| GET   | /api/v1/airports | JWT    | Поиск аэропортов по названию, IATA, ICAO, городу |
| POST  | /api/v1/offers   | JWT, ROLE_AGENT/ROLE_SUPER_ADMIN, своё агентство | Создать offer (agency_id выводится из агентства текущего пользователя) |
| GET   | /api/v1/offers   | JWT (любая роль) | Список offer своего агентства (пагинация + фильтры, любой статус) |
| GET   | /api/v1/offers/{uuid} | JWT (любая роль) | Получить один offer своего агентства (любой статус); чужое агентство → `404` |
| GET   | /api/v1/public/offers/{uuid} | public | Получить опубликованный offer по `uuid` без авторизации; не-`published` → `404` |
| PATCH | /api/v1/offers/{uuid} | JWT, ROLE_AGENT/ROLE_SUPER_ADMIN, своё агентство | Частичное обновление offer (title/description/status) |
| DELETE| /api/v1/offers/{uuid} | JWT, ROLE_AGENT/ROLE_SUPER_ADMIN, своё агентство | Soft delete offer |
| GET   | /api/doc         | public | Swagger UI                       |
| GET   | /healthz         | public | Healthcheck                      |

**Статусы offer:** `draft` (черновик, редактируется) → `ready` (заполнен и сохранён, но агент ещё не решил публиковать) → `published` (виден всем). `draft` и `ready` видны только пользователям своего агентства (любая роль); переход между статусами свободный, делается через `PATCH .../offers/{uuid}` (`status`).

**Видимость offer (read-side):** `GET /api/v1/offers` и `GET /api/v1/offers/{uuid}` — приватные эндпоинты (нужен JWT), список/offer скоупится строго на агентство текущего пользователя, любой статус; роль на видимость не влияет — `ROLE_USER` видит то же, что и `ROLE_AGENT`/`ROLE_SUPER_ADMIN` в пределах своего агентства. Offer другого агентства для приватных ручек не существует (`404`) — 1 пользователь = 1 агентство, кросс-агентского доступа нет даже у `ROLE_SUPER_ADMIN`. `GET /api/v1/public/offers/{uuid}` — отдельная, полностью анонимная ручка («ссылка, которой делятся с клиентом»): без токена, видит только `published` (независимо от агентства), иначе `404`.

**Владение по агентству (write-side):** создавать/изменять/удалять offer может только `ROLE_AGENT` или `ROLE_SUPER_ADMIN`, принадлежащий **тому же** агентству, что и offer (`offer.agency_id == user.agency_id`); чужое агентство → `404` (не раскрываем существование чужого offer), недостаточная роль (`ROLE_USER`) → `403`. Особого режима для `ROLE_SUPER_ADMIN` нет — 1 пользователь = 1 агентство касается всех ролей одинаково.

## Отладка (Debugger)

### HTTP-запросы (Remote Delve через Docker)

`dlv` уже установлен в dev-образе, порт `2345` проброшен в `docker-compose.override.yml`.

**1. В проекте уже настроен режим отладки в `.air.toml`** — в секции `[build]`:

```toml
[build]
  # -gcflags отключает оптимизации и инлайнинг, иначе точки остановки прыгают
  cmd      = "go build -gcflags='all=-N -l' -o ./tmp/server ./cmd/server"
  bin      = "./tmp/server"
  # full_bin: air запускает бинарник через dlv вместо прямого вызова
  full_bin = "dlv exec --headless --listen=:2345 --api-version=2 --accept-multiclient --continue ./tmp/server"
```

**2. Поднимаем контейнер с приложением**

**3. Подключиться из IDE GoLand:**

**3.1. Подключение из Goland**

`Run → Edit Configurations → + → Go Remote`

| Поле | Значение |
|------|----------|
| Host | `localhost` |
| Port | `2345` |
| On disconnect | `Leave it running` |

Нажать **Debug** — GoLand подключается к dlv. Точки остановки в хендлерах срабатывают при каждом запросе. При изменении кода air пересобирает бинарник; нужно переподключиться из GoLand (кнопка Debug снова).

> **Точка в `main.go`** выполняется один раз при старте. Чтобы поймать её, убери флаг `--continue` из `full_bin` — dlv будет ждать attach до запуска процесса.

**3.2. Подключение из Visual Studio**

```json
{
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Golang: Attach to docker (server)",
            "type": "go",
            "request": "attach",
            "mode": "remote",
            "remotePath": "/app",
            "port": 2345,
            "host": "127.0.0.1",
            "cwd": "${workspaceFolder}"
        },
        {
            "name": "Golang: Attach to docker (CLI)",
            "type": "go",
            "request": "attach",
            "mode": "remote",
            "remotePath": "/app",
            "port": 2346,
            "host": "127.0.0.1",
            "cwd": "${workspaceFolder}"
        }
    ]
}
```

Как отлаживать CLI:

1. docker compose up -d — поднять dev-стек (server через air на 2345, БД, Kafka).
2. В отдельном терминале: make debug-cli cmd="airports sync --dry-run" — зайдёт в уже запущенный контейнер app и стартует headless dlv для ./cmd/cli на порту 2346, дождётся подключения.
3. В VS Code: Run and Debug → выбрать "Golang: Attach to docker (CLI)" → F5. Брейкпоинты в internal/presentation/cli/airports.go (или user.go) должны сработать.
4. Завершить сессию — Ctrl+C в терминале с make debug-cli, порт 2346 освободится для следующего запуска.

---

### CLI-команды (локальный dlv)

CLI (`cmd/cli`) запускается локально, без Docker.

**Через GoLand (рекомендуется):**

`Run → Edit Configurations → + → Go Build`

| Поле | Значение |
|------|----------|
| Run kind | `File` |
| Files | `cmd/cli/main.go` |
| Program arguments | `user create "Ada" "Lovelace" ada@example.com secret --agency-id 1` |

Поставить точку остановки → нажать **Debug**.

**Через терминал:**

```bash
# dlv сам компилирует с нужными флагами и запускает
dlv debug ./cmd/cli -- user create "Ada" "Lovelace" ada@example.com secret --agency-id 1
```

Внутри интерактивной сессии dlv:

```
(dlv) break internal/presentation/cli/create_user.go:25
(dlv) continue
```

---

## Миграции

Использовать команды описанные в `Makefile`

```
make migrate-up
make migrate-down
make migrate-new
```

```bash
migrate -path=./migrations -database "postgres://root:qwerty123@localhost:5432/tourismania?sslmode=disable" up
```

## Авторизация

```bash
make jwt-keys # генерация ключей
```

## Роли и права

Роль хранится в `users.roles` (массив) и резолвится на каждый запрос заново, а не из самого токена — так отозванные/изменённые права применяются немедленно, без переиздания токена. JWT несёт только неизменяемый `uuid` (`Claims.Subject`); `agency_id`/`roles` из БД резолвит доменный сервис `domain/service.UserFinder` по этому `uuid` — presentation-слой (в т.ч. мидлвари) в БД не ходит.

| Роль / гость       | Offers: создание/изменение/удаление | Offers: чтение (приватное, `/offers*`) | Offers: чтение (публичное, `/public/offers/{uuid}`) |
|--------------------|--------------------------------------|-----------------------------------------|--------------------------------------------------------|
| `ROLE_SUPER_ADMIN` | Только offer своего агентства (`offer.agency_id == user.agency_id`); чужое агентство → `404` — особого режима у супер-админа нет, 1 пользователь = 1 агентство касается всех ролей | Любой offer своего агентства (любой статус); чужое агентство → `404` | `published` → `200`, иначе `404` |
| `ROLE_AGENT`       | Только offer своего агентства (`offer.agency_id == user.agency_id`); чужое агентство → `404` | Любой offer своего агентства (любой статус); чужое агентство → `404` | `published` → `200`, иначе `404` |
| `ROLE_USER`        | Недоступно (`403` на write-эндпоинты) | Любой offer своего агентства (любой статус) — так же, как у агента/супер-админа; чужое агентство → `404` | `published` → `200`, иначе `404` |
| Неавторизованный   | Недоступно (`401` на write-эндпоинты) | Недоступно (`401`) | `published` → `200`, иначе `404` |

- **1 пользователь = 1 агентство касается всех ролей одинаково — кросс-агентского доступа нет даже у `ROLE_SUPER_ADMIN`.** Владение по агентству жёстко привязано к `AgencyID` пользователя, не к `CreatedBy` (тот — только аудит). Offer другого агентства на приватных ручках не раскрывается — `404`, а не `403`.
- `agency_id` при создании offer никогда не берётся из тела запроса — только из агентства текущего аутентифицированного пользователя.
- Роль не влияет на **видимость чтения** — только на право записи. `ROLE_USER` читает офферы своего агентства наравне с `ROLE_AGENT`/`ROLE_SUPER_ADMIN`, но не может создавать/изменять/удалять.
- `GET /api/v1/offers` и `GET /api/v1/offers/{uuid}` требуют JWT + `CurrentUserUUID` (обе — настоящие мидлвари, `custommw.JWT` затем `custommw.CurrentUserUUID`): первая валидирует токен и кладёт claims в контекст, вторая один раз извлекает из них `uuid` — ни одна в БД не ходит. `GET /api/v1/public/offers/{uuid}` — отдельный, полностью анонимный обработчик без auth-мидлварей вообще.
- И роль (для записи), и владение по агентству проверяет доменный `OfferManager` (`FindOwned` — единая точка проверки владения, используется и чтением, и записью) — не HTTP-мидлварь. Presentation-хендлер читает `uuid` из контекста (`custommw.CurrentUserUUIDFromContext`, без параллельного парсинга и без собственной 401-ветки) и передаёт его в Command/Query; application-хендлер резолвит `agency_id`/`roles` через доменный `service.UserFinder.Resolve` и строит `valueobject.Actor`, который уже идёт в домен. Ошибки домена (`service.Err*`) presentation-слой не видит напрямую — application-хендлер переводит их в `application/apperror` (`ErrUnauthenticated`/`ErrForbidden`/`ErrNotFound`/`ErrValidation`), и только эти сентинелы определяют HTTP-код.

## Тесты

```bash
go test ./tests/unit/...
go test ./tests/integration/...
go test ./tests/application/...
```

## Swagger

При наличии установленного `swag` CLI:

```bash
go install github.com/swaggo/swag/cmd/swag@latest
swag init -g cmd/server/main.go -o docs
```

После генерации Swagger UI будет доступен на `/api/doc`.

## Ключевые архитектурные принципы

1. Доменная сущность ≠ ORM-модель (`domain/entity.User` vs `infrastructure/persistence/postgres/model.User`).
2. Репозиторий — интерфейс в домене, реализация в infrastructure.
3. CQRS через `CommandBus` и `QueryBus` (in-memory routing).
4. Доменные события публикуются через интерфейс `event.Bus` (kafka — реализация).
5. DI собирается явно в `internal/app/container.go` — никаких глобалов.
6. Все бизнес-эндпоинты под `/api/v1/`.
