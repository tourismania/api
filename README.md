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

### Через docker-compose (не тестировался)

```bash
cp .env.example .env

# Сгенерировать JWT-ключи (если ещё не сделано):
openssl genpkey -algorithm RSA -out config/jwt/private.pem -pkeyopt rsa_keygen_bits:2048
openssl rsa -pubout -in config/jwt/private.pem -out config/jwt/public.pem

docker-compose up -d database kafka
# Применить миграции:
docker run --rm -v "$(pwd)/migrations:/migrations" --network host \
  migrate/migrate -path=/migrations \
  -database "postgres://root:qwerty123@localhost:5432/tourismania?sslmode=disable" up

docker-compose up app
```

## CLI

```bash
go run ./cmd/cli create-user "Ada" "Lovelace" ada@example.com secret
# → User successfully generated! id=1
```

## Endpoints

| Метод | Путь             | Доступ | Описание                         |
| ----- |------------------| ------ | -------------------------------- |
| POST  | /api/login       | public | Логин, возвращает JWT            |
| POST  | /api/v1/users    | JWT    | Создание пользователя            |
| GET   | /api/v1/users/me | JWT    | Профиль текущего пользователя    |
| GET   | /api/doc         | public | Swagger UI                       |
| GET   | /healthz         | public | Healthcheck                      |

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
