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

### sync-airports

Синхронизирует аэропорты, города и страны из внешних источников в БД.
Загружает данные из [mwgg/Airports](https://github.com/mwgg/Airports) (GitHub JSON)
и обогащает их русскоязычными названиями через Wikidata SPARQL.
Ожидаемое время выполнения: ~3–4 минуты для ~40 000 аэропортов.

Подробное описание алгоритма (источники данных, устройство запросов к Wikidata, тайминги, известные ограничения): [docs/cli/sync_airports.md](docs/cli/sync_airports.md).

```bash
# Preview без записи в БД
go run ./cmd/cli sync-airports --dry-run

# Полная синхронизация (запускать раз в месяц)
go run ./cmd/cli sync-airports

# production
docker compose exec tourismania_app /app/cli sync-airports
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
| GET   | /api/doc         | public | Swagger UI                       |
| GET   | /healthz         | public | Healthcheck                      |

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
2. В отдельном терминале: make debug-cli cmd="sync-airports --dry-run" — зайдёт в уже запущенный контейнер app и стартует headless dlv для ./cmd/cli на порту 2346, дождётся подключения.
3. В VS Code: Run and Debug → выбрать "Golang: Attach to docker (CLI)" → F5. Брейкпоинты в internal/presentation/cli/sync_airports.go (или user.go) должны сработать.
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
