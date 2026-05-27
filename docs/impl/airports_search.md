# Техническое задание на реализацию: `GET /api/v1/airports`

> **Статус:** Ready for implementation  
> **Endpoint:** `GET /api/v1/airports`  
> **Назначение:** Автодополнение и поиск аэропортов по одной строке (название, IATA, ICAO, город).

---

## 1. Контекст

Endpoint доступен если передается корректный JWT-токен. Данные берутся из трёх read-only таблиц в схеме `tourismania`. Источники данных обновляются раз в месяц, поэтому ответы кэшируются на стороне клиента (`Cache-Control: public, max-age=3600`).

Направление зависимостей сохраняется: `Presentation → Application → Domain ← Infrastructure`.

---

## 2. Полный список создаваемых файлов

```
migrations/postgres/
  006_enable_unaccent.up.sql
  006_enable_unaccent.down.sql
  007_enable_pg_trgm.up.sql
  007_enable_pg_trgm.down.sql
  008_create_countries.up.sql
  008_create_countries.down.sql
  009_create_cities.up.sql
  009_create_cities.down.sql
  010_create_airports.up.sql
  010_create_airports.down.sql

internal/
  domain/
    entity/
      airport.go                                    # Airport + City + Country entities
    valueobject/
      location.go                                   # Location (lat, lon, elevation)
    repository/
      airport_repository.go                         # AirportRepository interface

  application/
    query/
      search_airports/
        query.go                                    # SearchAirportsQuery
        result.go                                   # SearchAirportsResult + AirportResult
        handler.go                                  # Handler + UseCase interface

  infrastructure/
    persistence/
      postgres/
        model/
          airport.go                                # DB-модели Airport, City, Country
        queries/
          airports.sql                              # sqlc-запрос SearchAirports
        repository/
          airport_repository.go                     # Реализация AirportRepository

  presentation/
    http/
      api/
        v1/
          airport/
            search/
              handler.go                            # HTTP handler
              dto.go                                # Request params + Response types
      httpx/
        error.go                                    # Расширение: WriteStructuredError

tests/
  unit/
    airport_search_params_test.go                   # Валидация параметров запроса
  integration/
    airport_repository_test.go                      # Репозиторий против реального Postgres
  application/
    airports_search_http_test.go                    # E2E HTTP-тест
```

**Изменяемые файлы:**
```
config/config.go                           # + RateLimitConfig
config/container.go                        # Restructure: Http{} и App{} вложенные группы
internal/presentation/http/router.go       # + GET /api/v1/airports
CLAUDE.md                                  # + правило миграций
go.mod / go.sum                            # + github.com/go-chi/httprate
```

---

## 3. Правило миграций (добавить в CLAUDE.md)

Добавить в раздел **General Rules** в `CLAUDE.md`:

```
- Принцип миграций: 1 действие = 1 миграция. Каждая миграция содержит одно
  логическое изменение схемы: создание одной таблицы, добавление одного индекса,
  включение одного расширения. Не объединять несвязанные изменения в одном файле.
```

---

## 4. База данных

Таблицы создаются в схеме по умолчанию (`public`), без отдельной схемы `aviation`. Это соответствует существующей структуре проекта (таблица `users` находится в `public`).

### Миграции (006–014)

Каждый файл выполняет **одно** действие согласно правилу выше.

**006** — расширение `unaccent`:
```sql
-- 006_enable_unaccent.up.sql
CREATE EXTENSION IF NOT EXISTS unaccent;

-- 006_enable_unaccent.down.sql
DROP EXTENSION IF EXISTS unaccent;
```

**007** — расширение `pg_trgm`:
```sql
-- 007_enable_pg_trgm.up.sql
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- 007_enable_pg_trgm.down.sql
DROP EXTENSION IF EXISTS pg_trgm;
```

**008** — таблица `countries`:
```sql
-- 008_create_countries.up.sql
CREATE TABLE countries (
    iso2 char(2)       PRIMARY KEY,
    name varchar(100)  NOT NULL
);

-- 008_create_countries.down.sql
DROP TABLE IF EXISTS countries;
```

**009** — таблица `cities`:
```sql
-- 009_create_cities.up.sql
CREATE TABLE cities (
    id           serial      PRIMARY KEY,
    name         varchar(100) NOT NULL,
    state        varchar(100),
    timezone     varchar(50),
    country_iso2 char(2)     NOT NULL REFERENCES countries(iso2)
);

-- 009_create_cities.down.sql
DROP TABLE IF EXISTS cities;
```

**010** — таблица `airports`:
```sql
-- 010_create_airports.up.sql
CREATE TABLE airports (
    icao         char(4)      PRIMARY KEY,
    iata         char(3)      UNIQUE,          -- nullable: малые аэропорты без IATA
    name         varchar(200) NOT NULL,
    location     float8[],                     -- [longitude, latitude], индекс от 1
    elevation_ft int,
    city_id      int          NOT NULL REFERENCES cities(id)
);

-- 010_create_airports.down.sql
DROP TABLE IF EXISTS airports;
```

---

## 5. sqlc-запрос

**Файл:** `internal/infrastructure/persistence/postgres/queries/airports.sql`

```sql
-- name: SearchAirports :many
WITH q AS (
    SELECT
        a.icao,
        a.iata,
        a.name                  AS airport_name,
        a.location[1]           AS lon,
        a.location[2]           AS lat,
        a.elevation_ft,
        c.id                    AS city_id,
        c.name                  AS city_name,
        c.state                 AS city_state,
        c.timezone              AS city_timezone,
        co.iso2                 AS country_iso2,
        co.name                 AS country_name,
        CASE
            WHEN upper(a.iata) = upper(@search::text)  THEN 1
            WHEN upper(a.icao) = upper(@search::text)  THEN 2
            WHEN lower(unaccent(a.name)) LIKE lower(unaccent(@search_prefix::text))
              OR lower(unaccent(c.name)) LIKE lower(unaccent(@search_prefix::text)) THEN 3
            ELSE 4
        END                     AS rank
    FROM airports a
    JOIN cities    c  ON c.id   = a.city_id
    JOIN countries co ON co.iso2 = c.country_iso2
    WHERE (
        upper(a.iata)               = upper(@search::text)
        OR upper(a.icao)            = upper(@search::text)
        OR lower(unaccent(a.name))  LIKE lower(unaccent(@search_like::text))
        OR lower(unaccent(c.name))  LIKE lower(unaccent(@search_like::text))
    )
    AND (@country_filter::text IS NULL OR co.iso2 = upper(@country_filter::text))
)
SELECT *, COUNT(*) OVER() AS total_count
FROM q
ORDER BY rank ASC, airport_name ASC
LIMIT @limit_val::int
OFFSET @offset_val::int;
```

> **Примечание по индексации массива:** PostgreSQL индексирует массивы начиная с 1. Поле `location[1]` — долгота (lon), `location[2]` — широта (lat).

> **Особенность sqlc:** Из-за оконной функции `COUNT(*) OVER()` и CASE-выражения sqlc может сгенерировать неточные типы для `total_count` и `rank`. После `make sqlc` проверить `db/airports.sql.go` — при несоответствии добавить `overrides` в `sqlc.yaml`. Файлы в `db/` вручную не редактировать.

---

## 6. Доменный слой

### 6.1 Value Object — `internal/domain/valueobject/location.go`

```go
package valueobject

// Location represents the geographic position of an airport.
type Location struct {
    Latitude    float64
    Longitude   float64
    ElevationFt *int
}
```

### 6.2 Entities — `internal/domain/entity/airport.go`

```go
package entity

import "api/internal/domain/valueobject"

// Country holds ISO-2 country data.
type Country struct {
    ISO2 string
    Name string
}

// City is the municipality an airport belongs to.
type City struct {
    ID       int
    Name     string
    State    *string
    Timezone string
}

// Airport is the core aggregate for the search feature.
// IATA может быть nil у малых аэропортов без IATA-кода.
type Airport struct {
    ICAO     string
    IATA     *string
    Name     string
    Location valueobject.Location
    City     City
    Country  Country
}
```

### 6.3 Repository interface — `internal/domain/repository/airport_repository.go`

```go
package repository

import (
    "context"
    "api/internal/domain/entity"
)

// AirportFilter carries the normalised search parameters.
type AirportFilter struct {
    Search  string  // normalised (trimmed, lower)
    Country *string // ISO-2, upper; nil means no filter
    Limit   int
    Offset  int
}

// AirportSearchResult carries a page of airports plus the total count.
type AirportSearchResult struct {
    Airports   []entity.Airport
    TotalCount int64
}

// AirportRepository is the read-port for airport search.
// The concrete implementation lives in infrastructure/persistence.
type AirportRepository interface {
    Search(ctx context.Context, f AirportFilter) (AirportSearchResult, error)
}
```

---

## 7. Application слой

### 7.1 `internal/application/query/search_airports/query.go`

```go
package searchairports

// Query is the input for the SearchAirports use-case.
type Query struct {
    Search  string
    Country *string // nil — без фильтра
    Limit   int
    Offset  int
}
```

### 7.2 `internal/application/query/search_airports/result.go`

```go
package searchairports

// LocationResult is the coordinate projection returned by the use-case.
type LocationResult struct {
    Latitude    float64
    Longitude   float64
    ElevationFt *int
}

// CityResult is the city projection returned by the use-case.
type CityResult struct {
    ID       int
    Name     string
    State    *string
    Timezone string
}

// CountryResult is the country projection returned by the use-case.
type CountryResult struct {
    ISO2 string
    Name string
}

// AirportResult is a single airport in the use-case response.
type AirportResult struct {
    ICAO     string
    IATA     *string
    Name     string
    Location LocationResult
    City     CityResult
    Country  CountryResult
}

// Result is what the Handler returns to the presentation layer.
type Result struct {
    Airports   []AirportResult
    TotalCount int64
}
```

### 7.3 `internal/application/query/search_airports/handler.go`

```go
package searchairports

import (
    "context"
    "fmt"

    "api/internal/domain/repository"
)

// UseCase is the port the presentation layer depends on.
type UseCase interface {
    Handle(ctx context.Context, q Query) (Result, error)
}

// AirportSearcher is the read-port consumed by this use-case.
// Defined here to invert the dependency: infrastructure implements this.
type AirportSearcher interface {
    Search(ctx context.Context, f repository.AirportFilter) (repository.AirportSearchResult, error)
}

// Handler orchestrates the airport search use-case.
type Handler struct {
    airports AirportSearcher
}

// NewHandler constructs the use-case handler.
func NewHandler(airports AirportSearcher) *Handler {
    return &Handler{airports: airports}
}

// Handle satisfies UseCase.
func (h *Handler) Handle(ctx context.Context, q Query) (Result, error) {
    res, err := h.airports.Search(ctx, repository.AirportFilter{
        Search:  q.Search,
        Country: q.Country,
        Limit:   q.Limit,
        Offset:  q.Offset,
    })
    if err != nil {
        return Result{}, fmt.Errorf("search airports: %w", err)
    }

    out := make([]AirportResult, 0, len(res.Airports))
    for _, a := range res.Airports {
        out = append(out, AirportResult{
            ICAO: a.ICAO,
            IATA: a.IATA,
            Name: a.Name,
            Location: LocationResult{
                Latitude:    a.Location.Latitude,
                Longitude:   a.Location.Longitude,
                ElevationFt: a.Location.ElevationFt,
            },
            City: CityResult{
                ID:       a.City.ID,
                Name:     a.City.Name,
                State:    a.City.State,
                Timezone: a.City.Timezone,
            },
            Country: CountryResult{
                ISO2: a.Country.ISO2,
                Name: a.Country.Name,
            },
        })
    }

    return Result{Airports: out, TotalCount: res.TotalCount}, nil
}
```

---

## 8. Infrastructure слой

### 8.1 DB-модели — `internal/infrastructure/persistence/postgres/model/airport.go`

Модели отражают строки БД (≠ доменные entity), аналогично существующей `model/user.go`.

```go
// Package model contains PostgreSQL row representations.
// These are NOT domain entities — they mirror the exact DB column layout.
package model

// Country is the DB row for the countries table.
type Country struct {
    ISO2 string // char(2) PRIMARY KEY
    Name string // varchar(100) NOT NULL
}

// City is the DB row for the cities table.
type City struct {
    ID          int     // serial PRIMARY KEY
    Name        string  // varchar(100) NOT NULL
    State       *string // varchar(100) nullable
    Timezone    string  // varchar(50)
    CountryISO2 string  // char(2) FK -> countries.iso2
}

// Airport is the DB row for the airports table.
type Airport struct {
    ICAO        string   // char(4) PRIMARY KEY
    IATA        *string  // char(3) UNIQUE nullable
    Name        string   // varchar(200) NOT NULL
    Location    []float64 // float8[] — [longitude, latitude]
    ElevationFt *int     // int nullable
    CityID      int      // int FK -> cities.id
}
```

> Модели используются только внутри `infrastructure/persistence` для явного документирования структуры таблиц. В репозитории маппинг происходит из `db.SearchAirportsRow` (sqlc) → `entity.Airport` (домен) напрямую, без промежуточного model — если модель не нужна как промежуточный шаг, её роль сугубо документационная.

### 8.2 Repository — `internal/infrastructure/persistence/postgres/repository/airport_repository.go`

Ответственности:
1. Принимает `repository.AirportFilter`.
2. Формирует параметры sqlc-запроса: `search_like = "%" + search + "%"`, `search_prefix = search + "%"`.
3. Маппит строки `db.SearchAirportsRow` → `entity.Airport`.
4. При `pgx.ErrNoRows` возвращает пустой результат, не ошибку.

```go
// Compile-time interface check
var _ domainrepo.AirportRepository = (*AirportRepository)(nil)

type AirportRepository struct {
    queries *db.Queries
}

func NewAirportRepository(queries *db.Queries) *AirportRepository {
    return &AirportRepository{queries: queries}
}

func (r *AirportRepository) Search(ctx context.Context, f repository.AirportFilter) (repository.AirportSearchResult, error) {
    searchLike   := "%" + f.Search + "%"
    searchPrefix := f.Search + "%"

    rows, err := r.queries.SearchAirports(ctx, db.SearchAirportsParams{
        Search:        f.Search,
        SearchLike:    searchLike,
        SearchPrefix:  searchPrefix,
        CountryFilter: f.Country,       // *string, nil = no filter
        LimitVal:      int32(f.Limit),
        OffsetVal:     int32(f.Offset),
    })
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return repository.AirportSearchResult{}, nil
        }
        return repository.AirportSearchResult{}, fmt.Errorf("search airports: %w", err)
    }

    airports := make([]entity.Airport, 0, len(rows))
    var total int64
    for _, row := range rows {
        total = row.TotalCount
        airports = append(airports, mapRowToAirport(row))
    }

    return repository.AirportSearchResult{Airports: airports, TotalCount: total}, nil
}
```

**Маппинг** `mapRowToAirport` из `db.SearchAirportsRow`:
- `row.Iata` — sqlc генерирует `pgtype.Text` для nullable UNIQUE; конвертируем: если `.Valid` → `&row.Iata.String`, иначе `nil`
- `row.Lon`, `row.Lat` → `float64` (прямой cast или через `pgtype.Float8`)
- `row.ElevationFt` — nullable int; аналогично через `.Valid`

> Точные имена полей `db.SearchAirportsRow` уточняются после `make sqlc`. При несоответствии типов — добавить `overrides` в `sqlc.yaml`.

---

## 9. Presentation слой

### 9.1 DTO — `internal/presentation/http/api/v1/airport/search/dto.go`

```go
package searchairporthttp

// SearchParams содержит query-параметры запроса.
type SearchParams struct {
    Search  string `schema:"search"  validate:"required"`
    Limit   int    `schema:"limit"   validate:"omitempty,min=1,max=100"`
    Offset  int    `schema:"offset"  validate:"omitempty,min=0,max=10000"`
    Country string `schema:"country" validate:"omitempty,len=2"`
    Lang    string `schema:"lang"    validate:"omitempty"`
}

// LocationResponse is the location projection in the JSON response.
type LocationResponse struct {
    Latitude    float64 `json:"latitude"`
    Longitude   float64 `json:"longitude"`
    ElevationFt *int    `json:"elevation_ft"`
}

// CityResponse is the city projection in the JSON response.
type CityResponse struct {
    ID       int     `json:"id"`
    Name     string  `json:"name"`
    State    *string `json:"state"`
    Timezone string  `json:"timezone"`
}

// CountryResponse is the country projection in the JSON response.
type CountryResponse struct {
    ISO2 string `json:"iso2"`
    Name string `json:"name"`
}

// AirportResponse is a single airport item in the JSON response.
type AirportResponse struct {
    ICAO     string           `json:"icao"`
    IATA     *string          `json:"iata"`
    Name     string           `json:"name"`
    Location LocationResponse `json:"location"`
    City     CityResponse     `json:"city"`
    Country  CountryResponse  `json:"country"`
}

// MetaResponse carries pagination metadata.
type MetaResponse struct {
    Total  int64  `json:"total"`
    Limit  int    `json:"limit"`
    Offset int    `json:"offset"`
    Search string `json:"search"`
}

// SearchResponse is the top-level JSON envelope for a successful response.
type SearchResponse struct {
    Data []AirportResponse `json:"data"`
    Meta MetaResponse      `json:"meta"`
}
```

### 9.2 HTTP Handler — `internal/presentation/http/api/v1/airport/search/handler.go`

**Логика хендлера:**

1. Декодировать query-параметры через `r.URL.Query()`.
2. Нормализовать `search`: `strings.TrimSpace()` → свернуть пробелы → проверить длину ≥ 2.
3. Если длина < 2 → `httpx.WriteStructuredError(w, 400, "INVALID_SEARCH", "Parameter 'search' must be at least 2 characters long", "search")`.
4. Нормализовать `country` → `strings.ToUpper()`; если пусто → `nil`.
5. Применить дефолты: `limit = 20`, `offset = 0`.
6. Провалидировать диапазоны через `validator`.
7. Вызвать `useCase.Handle(r.Context(), query)`.
8. Установить заголовок `Cache-Control: public, max-age=3600`.
9. Вернуть `200` с `SearchResponse`.

**Swag-аннотации:**
```go
//	@Summary      Search airports
//	@Description  Full-text search over airports and cities with ranking and pagination.
//	@Tags         Airports
//	@Produce      json
//	@Param        search  query     string  true   "Search string (min 2 chars)"
//	@Param        limit   query     int     false  "Max results (1–100, default 20)"
//	@Param        offset  query     int     false  "Pagination offset (0–10000)"
//	@Param        country query     string  false  "ISO-2 country filter (e.g. RU)"
//	@Success      200     {object}  SearchResponse
//	@Failure      400     {object}  httpx.StructuredErrorBody
//	@Failure      429     {object}  httpx.StructuredErrorBody
//	@Failure      500     {object}  httpx.StructuredErrorBody
//	@Security     Bearer
//	@Router       /api/v1/airports [get]
```

### 9.3 Расширение httpx — `internal/presentation/http/httpx/error.go`

ТЗ требует формата ошибок, отличающегося от существующего `ErrorBody`. Добавить отдельный файл в пакет `httpx`:

```go
// StructuredErrorBody is the error envelope for public endpoints
// that require machine-readable error codes.
type StructuredErrorBody struct {
    Error StructuredError `json:"error"`
}

// StructuredError carries a machine-readable code alongside the human message.
type StructuredError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
    Field   string `json:"field,omitempty"`
}

// WriteStructuredError sends a structured error response.
func WriteStructuredError(w http.ResponseWriter, status int, code, message, field string) {
    WriteJSON(w, status, StructuredErrorBody{
        Error: StructuredError{Code: code, Message: message, Field: field},
    })
}
```

---

## 10. Rate Limiting Middleware

**Почему `go-chi/httprate`, а не кастомная реализация:**
Для chi-приложений `go-chi/httprate` является идиоматическим выбором. Он battle-tested, поддерживает per-key (IP) лимит из коробки, не требует ручного управления `sync.Map` и TTL, и выражается в одну строку. Кастомная реализация на `golang.org/x/time/rate` избыточна.

**Установка:**
```bash
go get github.com/go-chi/httprate
```

**Использование в router.go:**
```go
import "github.com/go-chi/httprate"

// В Server.Build():
r.Group(func(pub chi.Router) {
    pub.Use(s.JWT)  // endpoint за JWT
    pub.Use(httprate.LimitByIP(60, time.Minute))
    pub.Get("/api/v1/airports", s.Airports.Handle)
})
```

**При превышении лимита** `go-chi/httprate` автоматически возвращает `429`. Чтобы дополнить тело ответа структурированной ошибкой — передать кастомный `LimitHandler`:

```go
httprate.NewRateLimiter(60, time.Minute, httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Retry-After", "60")
    httpx.WriteStructuredError(w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many requests", "")
}))
```

**Конфигурация** — добавить в `config.go`:
```go
// RateLimitConfig controls per-IP request throttling for public endpoints.
type RateLimitConfig struct {
    RequestsPerMinute int // RATE_LIMIT_RPM, default 60
}
```

Переменная окружения: `RATE_LIMIT_RPM=60`. Добавить в `.env.example`.

> **Область применения:** rate limiter навешивается только на маршрут `/api/v1/airports`. Глобально не применять — у авторизованных маршрутов другая семантика лимитирования.

---

## 11. Изменения в `config/config.go`

```go
// RateLimitConfig controls per-IP request throttling for public endpoints.
type RateLimitConfig struct {
    RequestsPerMinute int // RATE_LIMIT_RPM, default 60
}
```

В `Load()` добавить поле `Config.RateLimit RateLimitConfig` и читать `RATE_LIMIT_RPM` (дефолт: 60).

---

## 12. Изменения в `config/container.go`

Текущая структура `Container` держит все хендлеры на верхнем уровне вместе с инфраструктурой. Необходимо сгруппировать по слоям:

```go
// Container holds every wired collaborator the entrypoints need.
type Container struct {
    Cfg *Config

    // Infrastructure
    Pool    *pgxpool.Pool
    Queries *db.Queries
    Kafka   *kafka.Producer
    JWT     *auth.Service

    // App — application layer use-case handlers.
    App struct {
        CreateUser     *createusercmd.Handler
        GetMe          *getmeq.Handler
        SearchAirports *searchairports.Handler
    }

    // Http — presentation layer HTTP handlers.
    Http struct {
        Login        *loginhttp.Handler
        CreateUser   *createuserhttp.Handler
        GetMe        *getmehttp.Handler
        Airports     *searchairporthttp.Handler
    }

    Validate *validator.Validate
}
```

В `Build()` обновить присвоения:
```go
// Application
c.App.CreateUser     = createusercmd.NewHandler(userCreator)
c.App.GetMe          = getmeq.NewHandler(userRepo, rightsDescriber)
c.App.SearchAirports = searchairports.NewHandler(airportRepo)

// HTTP
c.Http.Login      = loginhttp.NewHandler(queries, hasher, jwtSvc, validate)
c.Http.CreateUser = createuserhttp.NewHandler(c.App.CreateUser, validate)
c.Http.GetMe      = getmehttp.NewHandler(c.App.GetMe, getmehttp.NewResolver())
c.Http.Airports   = searchairporthttp.NewHandler(c.App.SearchAirports, validate)
```

> Обратить внимание: все вызывающие места (`cmd/server/main.go`, `cmd/cli/main.go`), которые используют `c.LoginHandler`, `c.GetMeHandler` и т.д., необходимо обновить на `c.Http.Login`, `c.Http.GetMe` и т.д.

---

## 13. Изменения в `internal/presentation/http/router.go`

Обновить `Server` с учётом новой структуры `Container.Http`:

```go
type Server struct {
    Login      *loginhttp.Handler
    CreateUser *createuserhttp.Handler
    GetMe      *getmehttp.Handler
    Airports   *searchairporthttp.Handler
    JWT        *auth.Service
    RateLimit  int // requests per minute, from config

    CORSAllowedOrigins []string
}
```

В `Build()` добавить к JWT-группе:
```go
r.Route("/api/v1", func(api chi.Router) {
    api.Use(custommw.JWT(s.JWT))
    api.Post("/users", s.CreateUser.Handle)
    api.Get("/users/me", s.GetMe.Handle)

    // Airports search — публичный в продуктовом смысле, но за JWT на MVP.
    api.With(
        httprate.NewRateLimiter(s.RateLimit, time.Minute,
            httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
                w.Header().Set("Retry-After", "60")
                httpx.WriteStructuredError(w, http.StatusTooManyRequests,
                    "RATE_LIMITED", "Too many requests", "")
            }),
        ).Handler,
    ).Get("/airports", s.Airports.Handle)
})
```

---

## 14. Тесты

### 14.1 Unit-тесты — `tests/unit/airport_search_params_test.go`

| Тест | Описание |
|------|----------|
| `TestNormalizeSearch_Trim` | Обрезает пробелы по краям |
| `TestNormalizeSearch_CollapseSpaces` | Сворачивает множественные пробелы |
| `TestNormalizeSearch_TooShort_ReturnsError` | Строка < 2 символов → ошибка |
| `TestNormalizeSearch_ExactlyTwo_OK` | Строка из 2 символов — OK |
| `TestDefaultLimit` | При отсутствии limit → 20 |
| `TestDefaultOffset` | При отсутствии offset → 0 |
| `TestCountryUppercase` | `ru` → `RU` |

### 14.2 Integration-тесты — `tests/integration/airport_repository_test.go`

Требуют запущенного Postgres с миграциями и seed-данными.

| Тест | Seed-данные | Проверка |
|------|-------------|----------|
| `TestAirportRepository_Search_ByName` | SVO, DME, VKO, ZIA | Возвращает ≥ 1 результат |
| `TestAirportRepository_Search_ByIATA_ExactFirst` | SVO | SVO первым в результате |
| `TestAirportRepository_Search_ByICAO_ExactFirst` | UUEE | UUEE первым |
| `TestAirportRepository_Search_CountryFilter` | SVO (RU), JFK (US) | Фильтр `RU` — только SVO |
| `TestAirportRepository_Search_Pagination` | 4 московских аэропорта | `offset=2, limit=2` → 2 записи, `TotalCount=4` |
| `TestAirportRepository_Search_Unaccent` | ZRH `Zürich Airport` | Запрос `Zurich` находит аэропорт |

### 14.3 Application (E2E) тесты — `tests/application/airports_search_http_test.go`

Паттерн: поднять `Server.Build()` с тестовой БД, выполнить HTTP-запрос через `httptest`.

| Тест | Запрос | Ожидаемый ответ |
|------|--------|-----------------|
| `TestSearchAirports_Moscow` | `?search=Moscow` | 200, Москва первая |
| `TestSearchAirports_IATA_SVO` | `?search=SVO` | 200, `data[0].iata = "SVO"` |
| `TestSearchAirports_ICAO_UUEE` | `?search=UUEE` | 200, `data[0].icao = "UUEE"` |
| `TestSearchAirports_CountryFilter` | `?search=mos&country=RU` | 200, все `country.iso2 = "RU"` |
| `TestSearchAirports_TooShort` | `?search=a` | 400, `error.code = "INVALID_SEARCH"` |
| `TestSearchAirports_Pagination` | `?search=Moscow&limit=2&offset=2` | 200, `len(data) ≤ 2`, `meta.total = 4` |
| `TestSearchAirports_Unaccent` | `?search=Zurich` | 200, содержит аэропорт Цюриха |
| `TestSearchAirports_NoSearch` | (без search) | 400, `error.code = "INVALID_SEARCH"` |

---

## 15. Порядок реализации

```
1.  Миграции 006–010 + make sqlc
2.  domain/valueobject/location.go
3.  domain/entity/airport.go
4.  domain/repository/airport_repository.go
5.  infrastructure/persistence/postgres/model/airport.go
6.  infrastructure/persistence/postgres/repository/airport_repository.go
7.  application/query/search_airports/ (query, result, handler)
8.  httpx/error.go (WriteStructuredError)
9.  presentation/http/api/v1/airport/search/ (dto, handler)
10. go get github.com/go-chi/httprate
11. config/config.go (+RateLimitConfig)
12. config/container.go (реструктуризация Http{} / App{})
13. presentation/http/router.go (новый маршрут + rate limiter)
14. Обновить cmd/server/main.go и cmd/cli/main.go (новые пути к хендлерам)
15. Тесты (unit → integration → application)
16. make swag
17. README.md (+endpoint)
18. .env.example (+RATE_LIMIT_RPM)
19. CLAUDE.md (+правило миграций)
```

---

## 16. Acceptance Criteria

| # | Критерий | Тест-кейс |
|---|----------|-----------|
| 1 | `?search=Moscow` → SVO, DME, VKO, ZIA в первых строках | `TestSearchAirports_Moscow` |
| 2 | `?search=SVO` → Шереметьево первым | `TestSearchAirports_IATA_SVO` |
| 3 | `?search=UUEE` → Шереметьево первым | `TestSearchAirports_ICAO_UUEE` |
| 4 | `?search=mos&country=RU` → только Россия | `TestSearchAirports_CountryFilter` |
| 5 | `?search=a` → `400 INVALID_SEARCH` | `TestSearchAirports_TooShort` |
| 6 | `?search=Zurich` находит Zürich Airport | `TestSearchAirports_Unaccent` |
| 7 | `?search=Moscow&limit=2&offset=2` → 2 записи, `meta.total=4` | `TestSearchAirports_Pagination` |
| 8 | p95 ≤ 100 мс при 100 RPS | Нагрузочный тест (вне scope) |

---

## 17. Нефункциональные требования (Checklist)

- [ ] Заголовок `Cache-Control: public, max-age=3600` устанавливается в хендлере
- [ ] Rate limit: `go-chi/httprate`, 60 req/min на IP, `429 RATE_LIMITED`, `Retry-After: 60`
- [ ] CORS: наследуется из глобального middleware `router.go`
- [ ] Логирование: `search`, `limit`, `offset`, `duration`, `count` через `slog.InfoContext`. Без PII
- [ ] Endpoint за JWT (`custommw.JWT`)
- [ ] `make sqlc` не падает
- [ ] `golangci-lint run` чист
- [ ] `go build ./...` проходит
- [ ] `go test ./...` проходит
- [ ] `README.md` обновлён: раздел **Endpoints**
- [ ] `.env.example` обновлён: `RATE_LIMIT_RPM`
- [ ] `CLAUDE.md` обновлён: правило миграций
- [ ] `config/container.go` реструктурирован: `Http{}`, `App{}`
- [ ] Все вызывающие места обновлены под новую структуру `Container`
