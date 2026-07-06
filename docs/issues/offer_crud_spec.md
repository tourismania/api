# ТЗ: CRUD предложений по турпутёвкам (Offer)

**Статус:** Draft (v2, согласовано с заказчиком)
**Дата:** 2026-07-05
**Автор:** —
**Связанные документы:** `CLAUDE.md`, `README.md`

---

## 1. Контекст и цель

В платформе Tourismania турагент публикует **предложения по турпутёвкам (offer)**. Основная цель сущности — витрина: клиент знакомится с предложением (направление, условия, состав) и принимает решение о покупке.

Итоговая **цена offer не хранится в самой сущности** — она будет вычисляться из дочерних сущностей (перелёты, отели, поездки), которые реализуются отдельными ТЗ (см. §10). В рамках текущего ТЗ поле цены отсутствует полностью.

Реализуем полный **CRUD** для offer по архитектуре проекта (Clean Architecture + DDD + CQRS), сохраняя направление зависимостей `Presentation → Application → Domain ← Infrastructure`.

### Границы (scope)

**В scope (единый рабочий блок, один issue):**

- Доменная сущность `Offer`.
- Справочник `agencies` (турагентство) + признак активности; управление агентствами — через **CLI-команды** (создание / деактивация), см. §8.4 и §10.
- Привязка пользователя к агентству: **1 пользователь = 1 агентство** (поле `agency_id` у `users`).
- Новая доменная роль `ROLE_AGENT` и ролевая авторизация на границе HTTP + общий resolver текущего пользователя.
- CQRS use-cases: `create_offer`, `update_offer`, `delete_offer`, `get_offer`, `get_offers`.
- HTTP-эндпоинты `/api/v1/offers*`, Swagger, тесты.

**Вне scope (отдельные ТЗ/issue, см. §10):**

- Дочерние таблицы `offer_flights`, `offer_hotels`, `offer_trips` и вычисление цены.
- Полноценный HTTP-CRUD агентств (в этой итерации — только CLI create/deactivate).
- Kafka-события, нечёткий поиск, бронирование/покупка — **не делаем**.

---

## 2. Решения по требованиям (согласовано)

| Вопрос | Решение |
|--------|---------|
| Объём операций | Полный CRUD: `create/update/delete/get/get_offers` |
| Кто пишет | Роль турагента `ROLE_AGENT` (+ `ROLE_SUPER_ADMIN`) |
| Кто читает | Любой аутентифицированный (клиенты — `ROLE_USER`) |
| **Владение** | **Агент управляет всеми offer в рамках своего агентства.** 1 пользователь = 1 агентство. Супер-админ — любыми |
| Цена | **Отсутствует**; будет считаться из дочерних сущностей позже |
| Kafka-события | Не публикуются |
| Список | Пагинация + фильтры (без нечёткого поиска) |
| Удаление | **Soft delete** (`deleted_at`) |
| Статусы | Нужны: `draft` / `published`. Клиентам видны только `published` |
| Управление агентствами | Через CLI (create + deactivate), не через HTTP |
| Объём поставки | Один issue, полностью рабочий блок |

---

## 3. Доменная модель

### 3.1. Сущность `Offer`

`internal/domain/entity/offer.go`

| Поле | Тип (Go) | Описание |
|------|----------|----------|
| `ID` | `int` | Внутренний идентификатор (SERIAL). |
| `UUID` | `uuid.UUID` | Публичный неизменяемый идентификатор (как у `User`). |
| `Title` | `string` | Название путёвки. |
| `Description` | `string` | Описание (в БД — `text`). |
| `AgencyID` | `int` | FK → `agencies.id`. Агентство-владелец. Определяет права доступа. |
| `CreatedBy` | `int` | FK → `users.id`. Автор (аудит; на права не влияет). |
| `Status` | `enum.OfferStatus` | `draft` / `published`. |
| `CreatedAt` | `time.Time` | Дата создания. |
| `UpdatedAt` | `time.Time` | Дата обновления. |
| `DeletedAt` | `*time.Time` | Soft delete; `nil` — активна. |

Доменные инварианты:

- `Title` — непустой, длина ≤ 200.
- `Status` — из допустимого множества.
- `AgencyID`, `CreatedBy` — заданы (> 0).

Все публичные типы/поля — с Go-doc комментариями.

> Поля цены (`Money` VO, `price_*` колонки) в этой версии **исключены** намеренно.

### 3.2. Enum `OfferStatus`

`internal/domain/enum/offer_status.go`

```go
type OfferStatus string
const (
    OfferStatusDraft     OfferStatus = "draft"
    OfferStatusPublished OfferStatus = "published"
)
```

### 3.3. Роль `ROLE_AGENT`

`internal/domain/enum/role.go` — добавить:

```go
RoleAgent Role = "ROLE_AGENT"
```

### 3.4. Сущность `Agency`

`internal/domain/entity/agency.go`

```go
// Agency is the travel agency that owns offers.
type Agency struct {
    ID        int
    UUID      uuid.UUID
    Name      string
    Status    enum.AgencyStatus // active / inactive
    CreatedAt time.Time
    DeletedAt *time.Time        // soft delete
}
```

Enum `internal/domain/enum/agency_status.go`:

```go
type AgencyStatus string
const (
    AgencyStatusActive   AgencyStatus = "active"
    AgencyStatusInactive AgencyStatus = "inactive"
)
```

- Деактивация (CLI) переводит `Status` в `inactive` (не soft-delete).
- `DeletedAt` — для будущего soft-delete агентства; чтения фильтруют `deleted_at IS NULL`.
- offer можно создавать только под агентством со `Status = active`.

### 3.5. Привязка пользователя к агентству

**Инвариант: 1 пользователь = 1 агентство.** У `entity.User` появляется `AgencyID *int` (nullable: у клиентов `ROLE_USER` агентства может не быть; у `ROLE_AGENT` — обязательно). Значение `AgencyID` турагента — источник `offer.AgencyID` при создании.

**Регистрация пользователя учитывает агентство.** Существующий флоу регистрации (`create_user`) дополняется передачей `agency_id` при создании — см. §6.3.

---

## 4. Репозитории (порты + реализация)

Конвенция: **1 сущность = 1 репозиторий**. Интерфейс — в `internal/domain/repository/`, реализация — в `internal/infrastructure/persistence/postgres/repository/`. sqlc-запросы — в `queries/*.sql`, генерация `make sqlc`; код в `db/` вручную не редактируется. Модель БД ≠ entity, маппинг — в `mapper/`.

### 4.1. `OfferRepository`

`internal/domain/repository/offer_repository.go`

```go
type OfferFilter struct {
    AgencyID  *int              // фильтр по агентству
    Status    *enum.OfferStatus // фильтр по статусу
    CreatedBy *int              // опционально
    Limit     int
    Offset    int
}

type OfferListResult struct {
    Offers     []entity.Offer
    TotalCount int64
}

type OfferRepository interface {
    Store(ctx context.Context, o entity.Offer) (int, error)
    FindByUUID(ctx context.Context, id uuid.UUID) (*entity.Offer, error)
    List(ctx context.Context, f OfferFilter) (OfferListResult, error)
    Update(ctx context.Context, o entity.Offer) error
    SoftDelete(ctx context.Context, id uuid.UUID) error
}
```

- Все чтения фильтруют `deleted_at IS NULL`.
- Метод `List` — внутреннее имя репозитория (порт чтения); публичный use-case называется `get_offers`.

### 4.2. `AgencyRepository`

`internal/domain/repository/agency_repository.go`

```go
type AgencyRepository interface {
    Store(ctx context.Context, a entity.Agency) (int, error)               // CLI create
    FindByID(ctx context.Context, id int) (*entity.Agency, error)
    SetStatus(ctx context.Context, id int, status enum.AgencyStatus) error // CLI deactivate
    Exists(ctx context.Context, id int) (bool, error)
}
```

---

## 5. Доменные сервисы

`internal/domain/service/offer_manager.go` — **единый `OfferManager`** с методами `Insert`, `Update`, `Delete` (по договорённости; при росте логики дробим позже).

Ответственность:

- Проверка инвариантов `Offer` при создании/обновлении.
- Проверка существования и активности (`Status = active`) агентства через `AgencyRepository`.
- **Проверка владения по агентству:** при `Update`/`Delete` сверять `offer.AgencyID == currentAgencyID`; исключение — `ROLE_SUPER_ADMIN` (доступ к любым).
- При `Insert`: `offer.AgencyID` берётся из агентства текущего агента (не из тела запроса), `CreatedBy` = текущий пользователь.
- Sentinel-ошибки: `ErrOfferNotFound`, `ErrOfferForbidden`, `ErrAgencyNotFound`, `ErrAgencyInactive`, `ErrOfferNotPersisted`.
- Оборачивание ошибок `fmt.Errorf("context: %w", err)`; без `log.Fatal`/`os.Exit`; без внешних импортов (`pgx`, `chi`, `kafka`).

**`AgencyManager`** (`internal/domain/service/agency_manager.go`) — отдельный доменный сервис для агентств (методы `Create`, `Deactivate`), используется CLI-командами (§9).

---

## 6. Application слой (CQRS)

### Write-side (`internal/application/command/`)

- `create_offer/` — `command.go`, `handler.go`, `result.go`. Handler → `OfferManager.Insert`, возвращает `Result{UUID}`.
- `update_offer/` — UUID + изменяемые поля (`Title`, `Description`, `Status`) + identity вызывающего.
- `delete_offer/` — UUID + identity вызывающего.

### Read-side (`internal/application/query/`)

- `get_offer/` — `query.go` (UUID + identity для проверки видимости), `handler.go`, `result.go`.
- `get_offers/` — `query.go` (фильтры + `Limit`/`Offset` + identity), `handler.go`, `result.go` (список + `TotalCount`).

**Identity вызывающего** передаётся из presentation в use-case явными полями (домен не знает про HTTP/JWT):

```go
CurrentUserID   int
CurrentAgencyID *int          // nil у клиентов без агентства
CurrentRoles    []enum.Role
```

Правила видимости в read-side:

- `ROLE_SUPER_ADMIN` — все offer.
- `ROLE_AGENT` — все offer своего агентства (любой статус).
- `ROLE_USER` — только `status = published`.

Хендлеры тонкие — только оркестрация, реализуют порт `UseCase interface { Handle(ctx, ...) (..., error) }`.

### 6.3. Изменение флоу регистрации (`create_user`)

Регистрация пользователя должна привязывать его к агентству (1 пользователь = 1 агентство).

Затрагиваются существующие файлы:

- `internal/presentation/http/api/v1/user/create/dto.go` — добавить в `CreateUserRequest` поле `agency_id` (`*int`, JSON `agency_id`). Для будущей регистрации агента — обязательно; для клиента — опционально (правило подтвердить, см. §14).
- `internal/application/command/create_user/command.go` — добавить `AgencyID *int`.
- `internal/application/command/create_user/handler.go` — прокинуть `AgencyID` в `entity.User`.
- `internal/domain/service/user_creator.go` — `UserCreator.Create` сохраняет `AgencyID`; при заданном `AgencyID` проверять существование и активность агентства через `AgencyRepository` (новая зависимость сервиса).
- `internal/infrastructure/persistence/postgres/repository/user_repository.go` + `queries/users.sql` — `Store` пишет `agency_id`; выборки возвращают `agency_id`.

> Kafka-событие `UserRegistered` остаётся как есть. Изменение регистрации входит в текущий рабочий блок (один issue), т.к. без `agency_id` у агента offer не создать.

---

## 7. Presentation слой (HTTP)

### 7.1. Эндпоинты

Все под `/api/v1/`, за JWT-middleware. Новые routes добавить в `README.md` → **Endpoints**.

| Метод | Путь | Назначение | Доступ |
|-------|------|-----------|--------|
| `POST` | `/api/v1/offers` | Создать offer | `ROLE_AGENT`, `ROLE_SUPER_ADMIN` |
| `GET` | `/api/v1/offers` | Список (пагинация + фильтры) | аутентифицированный (видимость по роли) |
| `GET` | `/api/v1/offers/{uuid}` | Получить один offer | аутентифицированный (видимость по роли) |
| `PATCH` | `/api/v1/offers/{uuid}` | Обновить offer | агент того же агентства или `ROLE_SUPER_ADMIN` |
| `DELETE` | `/api/v1/offers/{uuid}` | Удалить (soft) | агент того же агентства или `ROLE_SUPER_ADMIN` |

### 7.2. Структура пакетов

```
internal/presentation/http/api/v1/offer/
  create/   handler.go, dto.go
  get_list/ handler.go, dto.go   # GET /offers   → use-case get_offers
  get/      handler.go, dto.go   # GET /offers/{uuid} → use-case get_offer
  update/   handler.go, dto.go
  delete/   handler.go, dto.go
```

- DTO валидируются `go-playground/validator` на границе.
- Тело `POST`/`PATCH` содержит только `title`, `description`, `status` — **без `agency_id` и без цены**. `agency_id` выводится из агентства текущего агента на сервере.
- Ответы — через `httpx` (`WriteJSON` / `ErrorBody`).
- Регистрация в `internal/presentation/http/router.go` (`Server` + `Build`); сборка зависимостей — в `config/container.go` (единственный composition root).

### 7.3. Авторизация: общий resolver текущего пользователя

**Контекст проблемы:** JWT несёт только `Subject` (uuid пользователя); роли и агентство берутся из БД на каждый запрос (комментарий в `auth.Claims`).

Реализуем **общий переиспользуемый middleware/resolver `CurrentUser`**:

1. По `Claims.Subject` (uuid) достаёт `entity.User` (`ID`, `Roles`, `AgencyID`) через `UserRepository` и кладёт в контекст (`CurrentUserFromContext(ctx)`).
2. Переиспользуется существующими фичами (напр. `get_me`) и новыми offer-хендлерами.
3. Дополнительный guard `RequireRole(roles...)` для write-эндпоинтов (403, если роли нет).
4. Из контекста хендлеры берут `CurrentUserID`, `CurrentAgencyID`, `CurrentRoles` и прокидывают в use-case; ролевую/владельческую проверку по агентству выполняет доменный `OfferManager`.

> Термин «Actor» из v1 заменён на явные поля `CurrentUserID` / `CurrentAgencyID` / `CurrentRoles` — это identity аутентифицированного пользователя, выполняющего запрос.

### 7.4. Коды ответов

| Ситуация | HTTP |
|----------|------|
| Создано | 201 + `{uuid}` |
| Успех чтения/обновления | 200 |
| Удаление | 204 |
| Ошибка валидации | 400 |
| Нет/невалидный токен | 401 |
| Нет прав / чужое агентство | 403 |
| offer/agency не найден | 404 |

---

## 8. База данных (миграции)

Нумерация продолжает существующую (последняя — `011`). Каждая — `.up.sql` + `.down.sql`, создаётся `make migrate-new name=...`. По договорённости **создание таблицы и её индексов — в одной миграции** (логически связаны).

### 8.1. `012_create_agencies`

```sql
CREATE TABLE agencies (
    id         SERIAL       PRIMARY KEY,
    uuid       UUID         NOT NULL UNIQUE,
    name       VARCHAR(200) NOT NULL,
    status     VARCHAR(20)  NOT NULL DEFAULT 'active', -- active / inactive
    created_at TIMESTAMP    NOT NULL,
    deleted_at TIMESTAMP    NULL
);
```

### 8.2. `013_add_users_agency`

```sql
ALTER TABLE "users"
    ADD COLUMN agency_id INT NULL REFERENCES agencies(id);
```

(1 пользователь = 1 агентство; nullable — клиенты без агентства.)

### 8.3. `014_create_offers`

```sql
CREATE TABLE offers (
    id          SERIAL       PRIMARY KEY,
    uuid        UUID         NOT NULL UNIQUE,
    title       VARCHAR(200) NOT NULL,
    description TEXT         NOT NULL DEFAULT '',
    agency_id   INT          NOT NULL REFERENCES agencies(id),
    created_by  INT          NOT NULL REFERENCES users(id),
    status      VARCHAR(20)  NOT NULL DEFAULT 'draft',
    created_at  TIMESTAMP    NOT NULL,
    updated_at  TIMESTAMP    NOT NULL,
    deleted_at  TIMESTAMP    NULL
);

CREATE INDEX offers_agency_id_idx ON offers (agency_id);
CREATE INDEX offers_status_idx    ON offers (status);
CREATE INDEX offers_active_idx    ON offers (id) WHERE deleted_at IS NULL;
```

### 8.4. `015_seed_agencies` (seed)

Стартовые данные агентств (отдельной миграцией, не смешивая со схемой): 1–2 демо-агентства (`uuid`, `name`, `status=active`, `created_at`) для локальной разработки и тестов.

sqlc-запросы: `queries/offers.sql`, `queries/agencies.sql`, дополнить `queries/users.sql` (выборка `agency_id`). Генерация — `make sqlc`.

---

## 9. CLI (управление агентствами)

Агентства управляются через CLI (cobra), по аналогии с `cmd/cli` / `internal/presentation/cli`. Логика — в доменном сервисе **`AgencyManager`** (`internal/domain/service/agency_manager.go`) с методами `Create` и `Deactivate`. Новые команды добавить в `README.md` → **CLI**.

| Команда | Действие |
|---------|----------|
| `agency create --name "<name>"` | `AgencyManager.Create` → генерирует `uuid`, `created_at`, `status=active`, `Store`. Печатает `id`/`uuid`. |
| `agency deactivate --id <id>` | `AgencyManager.Deactivate` → `SetStatus(id, inactive)`. |

CLI-команды не содержат бизнес-логики — делегируют `AgencyManager`. Никаких `log.Fatal`/`os.Exit` вне `main()`.

---

## 10. Задачи на будущее (описания для GitHub Issues)

Ниже — готовые описания отдельных issue (создаются позже, вне текущего блока).

### Issue: Дочерняя сущность `OfferFlight` (перелёты)

Реализовать сущность и таблицу `offer_flights` со связью FK на `offers(id)`, отдельным репозиторием (1 сущность = 1 репозиторий). Поля перелёта (черновик, уточнить): направление (from/to — связь с `airports`), даты вылета/прилёта, номер рейса, цена сегмента. CRUD в рамках offer. Данные хранить нормализованно, не в JSON. Учесть soft-delete/каскад согласно политике offer.

### Issue: Дочерняя сущность `OfferHotel` (отели)

Таблица `offer_hotels` (FK → `offers`), отдельный репозиторий. Поля (черновик): название отеля, категория/звёзды, тип питания, даты заезда/выезда, цена проживания. Нормализованное хранение.

### Issue: Дочерняя сущность `OfferTrip` (поездки/экскурсии)

Таблица `offer_trips` (FK → `offers`), отдельный репозиторий. Поля (черновик): название, описание, дата, цена. Нормализованное хранение.

### Issue: Вычисление итоговой цены Offer

После появления дочерних сущностей — агрегировать итоговую цену offer из сумм перелётов/отелей/поездок (доменный сервис/расчёт на read-side). Определить валюту и правила суммирования.

### Issue: Полный HTTP-CRUD агентств

Расширить управление агентствами до полноценного HTTP-CRUD (`/api/v1/agencies`) поверх существующего справочника и CLI: list/get/update/soft-delete, роли и права.

> **Не делаем (по решению заказчика):** бронирование/покупка, Kafka-события offer, нечёткий поиск.

---

## 11. Процесс

- **Один issue** на весь рабочий блок текущего ТЗ (миграции + домен + репозитории + use-cases + HTTP + CLI + тесты) — закрываем полностью рабочий кусок.
- Оригинальный промпт сохранить в описании issue.
- Именование ветки — определим позже.
- **Docs**: обновить `README.md` (Endpoints + CLI); при вопросах «зачем/почему» — `FAQ.md`.
- Новые ENV (если появятся) — в `.env.example`.
- **Validation Gates** перед merge: `go test ./...`, `golangci-lint run`, `go build ./...`, обновлён `README.md`, есть review, acceptance criteria выполнены.

---

## 12. Тестирование

Согласно `CLAUDE.md`. Именование: `TestFunctionName_Scenario_ExpectedResult`.

- **unit** (`tests/unit/`): инварианты `Offer`, `OfferStatus`; логика `OfferManager` (владение по агентству, супер-админ, активность агентства) с моками репозиториев. Все публичные функции домена — ≥1 тест.
- **integration** (`tests/integration/`): `OfferRepository` (Store/FindByUUID/List с фильтрами и пагинацией/Update/SoftDelete, `deleted_at IS NULL`); `AgencyRepository` (Store/SetActive/FindByID).
- **application** (`tests/application/`): e2e HTTP CRUD — коды 201/200/204/400/401/403/404; ролевой доступ; владение по агентству (агент чужого агентства → 403); видимость `published` для `ROLE_USER`; регистрация с `agency_id` (пользователь привязан к агентству).
- Критический путь (создание + авторизация по агентству) — покрытие ≥ 90%.
- Сгенерированный код `db/` напрямую не тестируется.

---

## 13. Acceptance criteria (для issue)

1. Миграции 012–015 применяются и откатываются (`make migrate-up` / `migrate-down`).
2. Роль `ROLE_AGENT` добавлена; у `users` есть `agency_id` (1:1 с агентством).
3. Доступны эндпоинты `POST/GET/GET{uuid}/PATCH/DELETE /api/v1/offers` с корректными кодами.
4. Владение по агентству работает: агент управляет всеми offer своего агентства; чужие → 403; супер-админ → все.
5. Клиенты (`ROLE_USER`) видят только `published`.
6. Soft delete: удалённые offer исключены из чтений.
7. CLI `agency create` / `agency deactivate` (через `AgencyManager`) работают; агентство имеет `uuid`, `status`, `created_at`, `deleted_at`.
8. Seed-агентства применяются.
9. Общий `CurrentUser` resolver-middleware внедрён и переиспользуется.
10. Регистрация (`create_user`) принимает `agency_id` и сохраняет привязку пользователя к агентству.
11. `README.md` обновлён (Endpoints + CLI); тесты и линтер зелёные.

---

## 14. Открытые вопросы (минимальны)

1. Обязателен ли `agency_id` при регистрации для всех, или только для будущих агентов (клиент `ROLE_USER` может регистрироваться без агентства)?
