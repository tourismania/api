# Agent Context — Tourismania API

---

## Project Purpose

REST API сервис управления пользователями для платформы Tourismania, построенный на Go с жёстким разделением слоёв (Clean Architecture + DDD + CQRS). Обрабатывает регистрацию, аутентификацию и профили пользователей. Надёжность и консистентность данных важнее скорости итераций.

**Primary language:** Go 1.26
**Framework:** chi v5 (HTTP router), spf13/cobra (CLI)  
**Key dependencies:** PostgreSQL 17 (pgx/v5 + sqlc), Kafka (segmentio/kafka-go), JWT RS256 (golang-jwt v5), golang-migrate, swaggo/swag, go-playground/validator v10, bcrypt

---

## Repository Structure

```
cmd/
  server/             # Entrypoint HTTP-сервера
  cli/                # Entrypoint CLI (cobra)
config/
  config.go           # Загрузка конфигурации из .env
  container.go        # Composition root (DI), единственное место сборки зависимостей
  jwt/                # RSA-ключи для JWT (private.pem / public.pem)
internal/
  domain/             # ЯДРО — не зависит ни от чего внешнего
    entity/           # Доменные сущности (User и т.д.)
    enum/             # Перечисления (Role и т.д.)
    event/            # Доменные события и интерфейс Bus
    factory/          # Фабрики для создания сложных объектов
    repository/       # Интерфейсы репозиториев
    service/          # Доменные сервисы (UserCreator, PasswordHasher и т.д.)
    valueobject/      # Value objects
  application/        # Use cases (тонкий слой оркестрации)
    command/          # Write-side: Command + Handler + Result
    query/            # Read-side: Query + Handler + Result
  infrastructure/     # Реализации доменных интерфейсов
    auth/             # JWT-сервис
    broker/kafka/     # Kafka Producer (реализация event.Bus)
    persistence/
      postgres/
        db/           # sqlc-генерированный код (НЕ РЕДАКТИРОВАТЬ вручную)
        model/        # Модели БД (≠ доменные entity)
        repository/   # Реализации domain/repository
  presentation/       # Точки входа (НЕ содержит бизнес-логику)
    http/
      api/            # HTTP-хендлеры (login, v1/user/*)
      middleware/     # Middleware (auth, logging и т.д.)
      httpx/          # Вспомогательные утилиты HTTP
      router.go       # Регистрация всех маршрутов
    cli/              # CLI-команды (cobra)
migrations/
  postgres/           # SQL up/down миграции (нумерованные: NNN_name.up/down.sql)
docs/                 # Swagger/OpenAPI (генерируется через swag, НЕ РЕДАКТИРОВАТЬ вручную)
tests/
  unit/               # Чистые unit-тесты (без I/O)
  integration/        # Тесты с реальной БД
  application/        # End-to-end HTTP-тесты
```

**Направление зависимостей:** `Presentation → Application → Domain ← Infrastructure`

Доменный слой не знает ни об HTTP, ни о Postgres, ни о Kafka.

---

## General Rules

- Always use Context7 when I need library/API documentation, code generation, setup or configuration steps without me having to explicitly ask.
- use skill: golang-design-patterns
- Для работы с техническим заданием подключай агента - engineering-technical-writer
- Если создаётся новая CLI-команда — добавь её в `README.md` в раздел **CLI**.
- Если создаётся новый route — добавь его в `README.md` в раздел **Endpoints**.
- Файлы в `docs/` и `internal/infrastructure/persistence/postgres/db/` генерируются автоматически. Не редактируй их вручную.
- При создании новой переменной окружения, которая появляется в проекте, добавляй ее в `.env.example`

---

## Critical Constraints

- **Never use `log.Fatal` or `os.Exit` outside of `main()`.**  
  Return errors. We instrument error rates via middleware and `os.Exit` bypasses it entirely.
- **No global state.** DI собирается исключительно в `config/container.go`.
- **Domain layer has zero external imports.** Никаких `pgx`, `chi`, `kafka` в `internal/domain/`.

---

## Coding Conventions

### Architecture

- Доменная сущность ≠ ORM-модель: `domain/entity.User` vs `infrastructure/persistence/postgres/model.User`.
- Репозиторий — интерфейс в домене, реализация в `infrastructure/`.
- CQRS: команды в `application/command/`, запросы в `application/query/`.
- Доменные события публикуются через интерфейс `domain/event.Bus` (Kafka — одна из реализаций).
- Все бизнес-эндпоинты — под `/api/v1/`.

### General

- Все публичные функции и типы — с Go-doc комментариями.
- Нет магических чисел — только именованные константы.
- Предпочитать композицию наследованию (embedding только если оправдано).
- Единый формат ответа API: `{ data, error, metadata }` (или как определено в `httpx/`).
- Входные данные валидируются на границе presentation-слоя (`go-playground/validator`), внутрь домена попадают уже чистые данные.

### Error Handling

- Использовать `fmt.Errorf("context: %w", err)` для оборачивания ошибок с сохранением цепочки.
- Никогда не поглощать ошибки молча.
- Логировать ошибки на границе (middleware), не на каждом уровне.
- Возвращать осмысленные HTTP-статусы с деталями ошибки.
- Типизированные ошибки (sentinel errors или кастомные типы) для ошибок, на которые надо реагировать по-разному.

### Logging

- Структурированное логирование (JSON-формат).
- Включать request ID в каждую запись.
- Уровни: `debug` — для разработки, `info` — для операций, `error` — для сбоев.

### Security

- Никогда не коммитить секреты или credentials.
- Вся конфигурация — через переменные окружения (`.env` + `godotenv`).
- Валидировать и санировать весь внешний ввод.
- JWT-ключи хранятся в `config/jwt/` и исключены из git (`.gitignore`).

---

## Testing Strategy

### Framework and Tools

- **Test framework:** `go test` (стандартная библиотека)
- **Assertion library:** `github.com/stretchr/testify`
- **Mocking:** интерфейсы + ручные моки или `gomock`
- **Coverage tool:** `go test -cover`

### Test Organization

```
tests/
  unit/           # Тестируют одну функцию/метод без I/O (быстрые)
  integration/    # Тестируют реальную БД (требуют запущенного Postgres)
  application/    # End-to-end HTTP-тесты (поднимают полный роутер)
```

### Test Naming

- `TestFunctionName_Scenario_ExpectedResult` (например, `TestUserCreator_DuplicateEmail_ReturnsError`)
- Описывать поведение, а не реализацию.

### Coverage Expectations

- Все публичные функции доменного слоя — минимум один тест.
- Критические пути (auth, создание пользователя) — 90%+.
- `internal/infrastructure/persistence/postgres/db/` — не тестируется напрямую (генерированный код).

### Running Tests

```bash
# Unit тесты (без внешних зависимостей)
go test ./tests/unit/...

# Integration тесты (нужен запущенный Postgres)
go test ./tests/integration/...

# Application тесты (полный стек)
go test ./tests/application/...

# Все тесты
go test ./...

# С покрытием
go test -cover ./...
```

---

## Development Process

### Workflow

```
Plan → Issue → Implement → Review → Merge → Docs
```

| Phase | Description |
|-------|-------------|
| **Plan** | Определить scope, зависимости, владельцев файлов. |
| **Issue** | Создать GitHub Issue с acceptance criteria и negative constraints. |
| **Implement** | Feature branch. Следовать конвенциям. Писать тесты. |
| **Review** | PR. Проверка корректности, покрытия, соответствия конвенциям. |
| **Merge** | После апрува — merge в `main`. |
| **Docs** | Обновить `README.md` (CLI/Endpoints), закрыть issue. |

### Branch Strategy

- **`main`** — стабильный production-ready код
- **Feature branches** — одна ветка на issue, от `main`
  - Формат: `[type]/[short-description]` (например, `feat/user-profile`, `fix/jwt-expiry`)

### Issues-First Rule

- Каждый запрос на работу — сначала GitHub Issue, потом реализация.
- Оригинальный промпт сохраняется в описании issue.
- Если промпт содержит несколько задач — создавать отдельные issues.

---

## Build and Run

```bash
# Загрузить зависимости
go mod download

# Сборка сервера
go build ./cmd/server

# Запуск сервера локально
go run ./cmd/server

# Горячая перезагрузка (dev)
air

# Сборка CLI
go build ./cmd/cli

# Запуск CLI
go run ./cmd/cli -- <command>

# Линтер
golangci-lint run

# Генерация Swagger
make swag

# Генерация sqlc
make sqlc

# Миграции
make migrate-up
make migrate-down
make migrate-new name=<migration_name>

# Генерация JWT-ключей
make jwt-keys

# Docker
docker-compose up -d database kafka
docker-compose up app
```

---

## Documentation Maintenance

| Документ       | Обновлять при                                                                                       |
|----------------|-----------------------------------------------------------------------------------------------------|
| `README.md`    | Добавление/изменение CLI-команд или endpoints                                                       |
| `CLAUDE.md`    | Изменение процессов, конвенций, структуры                                                           |
| `docs/swagger` | Автоматически через `make swag` при изменении хендлеров                                             |
| Inline godoc   | При изменении публичного API функций/типов                                                          |
| `STYLE.md `    | Изменились соглашения о стиле                                                                       |
| `FAQ.md `      | Если в процессе работы были заданы вопросы "Зачем?", "Почему?", "Для чего?". |

---

## Validation Gates

Перед тем как PR считается готовым к merge:

- [ ] Все тесты проходят (`go test ./...`)
- [ ] Линтер чист (`golangci-lint run`)
- [ ] Типы корректны (`go build ./...`)
- [ ] `README.md` обновлён (новые CLI-команды / endpoints)
- [ ] PR ограничен scope issue — нет несвязанных изменений
- [ ] Acceptance criteria из issue выполнены
- [ ] Как минимум один review с разрешёнными замечаниями

---

## References

TODO: дополнить ссылками на внутреннюю документацию по мере появления.
