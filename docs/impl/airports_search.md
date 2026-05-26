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
  006_create_aviation_schema.up.sql
  006_create_aviation_schema.down.sql

internal/
  domain/
    entity/
      airport.go                                   # Airport + City + Country entities
    valueobject/
      location.go                                  # Location (lat, lon, elevation)
    repository/
      airport_repository.go                        # AirportRepository interface

  application/
    query/
      search_airports/
        query.go                                   # SearchAirportsQuery
        result.go                                  # SearchAirportsResult + AirportResult
        handler.go                                 # Handler + UseCase interface

  infrastructure/
    persistence/
      postgres/
        queries/
          airports.sql                             # sqlc-запрос SearchAirports
        repository/
          airport_repository.go                   # Реализация AirportRepository

  presentation/
    http/
      api/
        v1/
          airport/
            search/
              handler.go                           # HTTP handler
              dto.go                               # Request params + Response types
      middleware/
        rate_limit.go                              # Per-IP rate limiter (60 req/min)
      httpx/
        error.go                                   # Расширение: WriteStructuredError

tests/
  unit/
    airport_search_params_test.go                 # Валидация параметров запроса
  integration/
    airport_repository_test.go                    # Репозиторий против реального Postgres
  application/
    airports_search_http_test.go                  # E2E HTTP-тест
```

**Изменяемые файлы:**
```
config/config.go          # + RateLimitConfig
config/container.go       # + AirportRepo, SearchAirportsApp, AirportsHandler
internal/presentation/http/router.go  # + GET /api/v1/airports
```

---

## 3. База данных

### 3.1 Миграция `006_create_aviation_schema.up.sql`

```sql
-- Расширения для регистронезависимого поиска с диакритикой и trigram-индексов
CREATE EXTENSION IF NOT EXISTS unaccent;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE aviation.countries (
    iso2        char(2)      PRIMARY KEY,
    name        varchar(100) NOT NULL
);

CREATE TABLE aviation.cities (
    id           serial       PRIMARY KEY,
    name         varchar(100) NOT NULL,
    state        varchar(100),
    timezone     varchar(50),
    country_iso2 char(2)      NOT NULL REFERENCES aviation.countries(iso2)
);

CREATE TABLE aviation.airports (
    icao         char(4)       PRIMARY KEY,
    iata         char(3)       UNIQUE,           -- nullable: малые аэропорты без IATA
    name         varchar(200)  NOT NULL,
    location     float8[],                        -- [longitude, latitude]
    elevation_ft int,
    city_id      int           NOT NULL REFERENCES aviation.cities(id)
);

-- =================== Индексы ===================

-- Trigram-индексы для подстрочного ILIKE.
-- Без них поиск делает seq scan по 29k строкам.
CREATE INDEX airports_name_trgm_idx
    ON aviation.airports USING gin (lower(unaccent(name)) gin_trgm_ops);

CREATE INDEX cities_name_trgm_idx
    ON aviation.cities USING gin (lower(unaccent(name)) gin_trgm_ops);

-- Upper-регистровые btree-индексы для точного совпадения IATA/ICAO.
CREATE INDEX airports_iata_upper_idx ON aviation.airports (upper(iata));
CREATE INDEX airports_icao_upper_idx ON aviation.airports (upper(icao));
```

### 3.2 Миграция `006_create_aviation_schema.down.sql`

```sql
DROP EXTENSION IF EXISTS pg_trgm;
DROP EXTENSION IF EXISTS unaccent;
DROP TABLE IF EXISTS countries;
DROP TABLE IF EXISTS cities;
DROP TABLE IF EXISTS airports;
```

---

## 4. sqlc-запрос

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
    FROM aviation.airports a
    JOIN aviation.cities    c  ON c.id       = a.city_id
    JOIN aviation.countries co ON co.iso2    = c.country_iso2
    WHERE (
        upper(a.iata)                    = upper(@search::text)
        OR upper(a.icao)                 = upper(@search::text)
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

> **Примечание:** PostgreSQL индексирует массивы начиная с 1. В ТЗ `location[0]` и `location[1]` используют 0-based нотацию — адаптируем на `location[1]` (lon) и `location[2]` (lat).

> **Особенность sqlc:** Из-за оконной функции `COUNT(*) OVER()` sqlc может сгенерировать неточную модель. После `make sqlc` **обязательно** проверить `db/airports.sql.go` и при необходимости скорректировать типы вручную (только в рамках типа для `total_count int64`, остальное генерируется корректно). Файл в `db/` вручную не редактировать — выполнить `make sqlc` заново после правки запроса.

---

## 5. Доменный слой

### 5.1 Value Object — `internal/domain/valueobject/location.go`

```go
package valueobject

// Location represents the geographic position of an airport.
type Location struct {
    Latitude    float64
    Longitude   float64
    ElevationFt *int
}
```

### 5.2 Entities — `internal/domain/entity/airport.go`

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
// iata может быть nil у малых аэропортов без IATA-кода.
type Airport struct {
    ICAO     string
    IATA     *string
    Name     string
    Location valueobject.Location
    City     City
    Country  Country
}
```

### 5.3 Repository interface — `internal/domain/repository/airport_repository.go`

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

> **Важно:** домен не знает о `pgx`, `sqlc` или схеме `aviation` — он работает исключительно с этим интерфейсом.

---

## 6. Application слой

### 6.1 `internal/application/query/search_airports/query.go`

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

### 6.2 `internal/application/query/search_airports/result.go`

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

### 6.3 `internal/application/query/search_airports/handler.go`

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

## 7. Infrastructure слой

### `internal/infrastructure/persistence/postgres/repository/airport_repository.go`

Ответственности:
1. Принимает `repository.AirportFilter`.
2. Формирует параметры sqlc-запроса `SearchAirports` (нормализация строк: `search_like = "%" + search + "%"`, `search_prefix = search + "%"`).
3. Маппит строки sqlc-модели в `[]entity.Airport` и возвращает `AirportSearchResult`.
4. Обрабатывает `pgx.ErrNoRows` → возвращает пустой результат (не ошибку).

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
        CountryFilter: f.Country,    // *string, nil = no filter
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

**Маппинг** `mapRowToAirport` извлекает из `db.SearchAirportsRow`:
- `row.Iata` → `*string` (sqlc генерирует как `pgtype.Text` для nullable UNIQUE — конвертируем через `.String` + nil-check)
- `row.Lon`, `row.Lat` → `float64`
- `row.ElevationFt` → `*int` (nullable int)
- остальные поля — прямой маппинг

> Точные названия полей генерируемой структуры `db.SearchAirportsRow` уточняются после `make sqlc`. При несоответствии типов — добавить `overrides` в `sqlc.yaml`.

---

## 8. Presentation слой

### 8.1 DTO — `internal/presentation/http/api/v1/airport/search/dto.go`

```go
package searchairporthttp

// SearchParams содержит query-параметры запроса.
// Валидация — go-playground/validator; бизнес-правила — в хендлере.
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

### 8.2 HTTP Handler — `internal/presentation/http/api/v1/airport/search/handler.go`

**Логика хендлера:**

1. Декодировать query-параметры через `schema.NewDecoder()` (или вручную через `r.URL.Query()`).
2. Нормализовать `search`:
   - `strings.TrimSpace()`
   - Свернуть множественные пробелы: `regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")`
   - Если длина после trim < 2 → вернуть `400` с `httpx.WriteStructuredError(w, 400, "INVALID_SEARCH", "Parameter 'search' must be at least 2 characters long", "search")`.
3. Нормализовать `country` → `strings.ToUpper()`.
4. Применить дефолты: `limit = 20`, `offset = 0`.
5. Провалидировать через `validator`.
6. Вызвать `useCase.Handle(r.Context(), query)`.
7. Установить заголовок `Cache-Control: public, max-age=3600`.
8. Вернуть `200` с `SearchResponse`.

**Swag-аннотации (шаблон):**
```go
// Handle is the http.HandlerFunc.
//
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
//	@Router       /api/v1/airports [get]
```

### 8.3 Расширение httpx — `internal/presentation/http/httpx/error.go`

ТЗ требует формата ошибок:
```json
{"error": {"code": "INVALID_SEARCH", "message": "...", "field": "search"}}
```

Это отличается от существующего `ErrorBody`. Добавить в пакет `httpx` новую функцию:

```go
// StructuredErrorBody is the error envelope used by public endpoints
// that return machine-readable error codes (e.g. airport search).
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

## 9. Rate Limiting Middleware

**Файл:** `internal/presentation/http/middleware/rate_limit.go`

**Зависимость:** `golang.org/x/time/rate` (уже является транзитивной зависимостью Go-экосистемы; если нет — `go get golang.org/x/time/rate`).

**Подход:** per-IP sliding window с `sync.Map` для хранения limiter'ов.

```go
// RateLimit returns a middleware that limits each IP to `rps` requests per second
// over a burst of `burst` requests. Превышение → 429 с заголовком Retry-After.
func RateLimit(rps rate.Limit, burst int) func(http.Handler) http.Handler
```

**Параметры для конфигурации:** 60 req/min → `rps = rate.Every(time.Second), burst = 60` (или `rate.Limit(1.0), burst = 60`).

Конфигурация подтягивается через `RateLimitConfig` в `config.go`:
```go
type RateLimitConfig struct {
    RequestsPerMinute int // default: 60
    Burst             int // default: 60
}
```
Переменные окружения: `RATE_LIMIT_RPM`, `RATE_LIMIT_BURST`.

**Заголовок при 429:**
```
Retry-After: 60
```
Тело ошибки: `httpx.WriteStructuredError(w, 429, "RATE_LIMITED", "Too many requests", "")`.

> **Область применения:** middleware применяется **только** к маршруту `/api/v1/airports` (или к группе публичных API-эндпоинтов). Не вешать глобально, чтобы не задеть авторизованные маршруты с другими лимитами.

---

## 10. Изменения в `config/config.go`

Добавить структуру и загрузку:

```go
// RateLimitConfig controls per-IP request throttling.
type RateLimitConfig struct {
    RequestsPerMinute int // RATE_LIMIT_RPM, default 60
    Burst             int // RATE_LIMIT_BURST, default 60
}
```

В `Load()` дополнить поле `Config.RateLimit RateLimitConfig` и читать `RATE_LIMIT_RPM` / `RATE_LIMIT_BURST`.

Добавить в `.env.example`:
```env
RATE_LIMIT_RPM=60
RATE_LIMIT_BURST=60
```

---

## 11. Изменения в `config/container.go`

Добавить в `Container`:
```go
AirportRepo        *pgrepo.AirportRepository
SearchAirportsApp  *searchairports.Handler
AirportsHandler    *searchairporthttp.Handler
```

В `Build()` добавить после создания `userRepo`:
```go
airportRepo       := pgrepo.NewAirportRepository(queries)
searchAirportsApp := searchairports.NewHandler(airportRepo)
airportsH         := searchairporthttp.NewHandler(searchAirportsApp, validate)
```

---

## 12. Изменения в `internal/presentation/http/router.go`

Добавить в `Server`:
```go
Airports    *searchairporthttp.Handler
RateLimiter func(http.Handler) http.Handler  // построен из конфига в Build()
```

В методе `Build()` добавить публичный маршрут **вне** JWT-группы:

```go
// Публичные API-эндпоинты с rate limiting.
r.Group(func(pub chi.Router) {
    pub.Use(s.RateLimiter)
    pub.Get("/api/v1/airports", s.Airports.Handle)
})
```

---

## 13. Тесты

### 13.1 Unit-тесты — `tests/unit/airport_search_params_test.go`

| Тест | Описание |
|------|----------|
| `TestNormalizeSearch_Trim` | Обрезает пробелы по краям |
| `TestNormalizeSearch_CollapseSpaces` | Сворачивает множественные пробелы |
| `TestNormalizeSearch_TooShort_ReturnsError` | Строка < 2 символов → ошибка |
| `TestNormalizeSearch_ExactlyTwo_OK` | Строка из 2 символов — OK |
| `TestDefaultLimit` | При отсутствии limit → 20 |
| `TestDefaultOffset` | При отсутствии offset → 0 |
| `TestCountryUppercase` | `ru` → `RU` |

### 13.2 Integration-тесты — `tests/integration/airport_repository_test.go`

Требуют запущенного Postgres с миграциями.

| Тест | Seed-данные | Проверка |
|------|-------------|----------|
| `TestAirportRepository_Search_ByName` | SVO, DME, VKO, ZIA | Возвращает ≥ 1 результат |
| `TestAirportRepository_Search_ByIATA_ExactFirst` | SVO | SVO первым в результате |
| `TestAirportRepository_Search_ByICAO_ExactFirst` | UUEE | UUEE первым |
| `TestAirportRepository_Search_CountryFilter` | SVO (RU), JFK (US) | Фильтр `RU` — только SVO |
| `TestAirportRepository_Search_Pagination` | 4 московских аэропорта | `offset=2, limit=2` → 2 записи, `TotalCount=4` |
| `TestAirportRepository_Search_Unaccent` | ZRH с именем `Zürich Airport` | Запрос `Zurich` находит аэропорт |

### 13.3 Application (E2E) тесты — `tests/application/airports_search_http_test.go`

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

## 14. Порядок реализации

Рекомендуемый порядок для чистого прохождения CI на каждом шаге:

```
1. Миграция + make sqlc (БД + автогенерация)
2. domain/valueobject/location.go
3. domain/entity/airport.go
4. domain/repository/airport_repository.go
5. application/query/search_airports/ (query, result, handler)
6. infrastructure/persistence/postgres/repository/airport_repository.go
7. httpx/error.go (WriteStructuredError)
8. presentation/http/api/v1/airport/search/ (dto, handler)
9. middleware/rate_limit.go
10. config/config.go (+RateLimitConfig)
11. config/container.go (wiring)
12. presentation/http/router.go (новый маршрут)
13. Тесты (unit → integration → application)
14. make swag (генерация swagger)
15. README.md (добавить endpoint в раздел Endpoints)
16. .env.example (RATE_LIMIT_RPM, RATE_LIMIT_BURST)
```

---

## 15. Acceptance Criteria (из ТЗ → с указанием тест-кейса)

| # | Критерий | Покрывается тестом |
|---|----------|--------------------|
| 1 | `?search=Moscow` → SVO, DME, VKO, ZIA в первых строках | `TestSearchAirports_Moscow` |
| 2 | `?search=SVO` → Шереметьево первым | `TestSearchAirports_IATA_SVO` |
| 3 | `?search=UUEE` → Шереметьево первым | `TestSearchAirports_ICAO_UUEE` |
| 4 | `?search=mos&country=RU` → только Россия | `TestSearchAirports_CountryFilter` |
| 5 | `?search=a` → `400 INVALID_SEARCH` | `TestSearchAirports_TooShort` |
| 6 | `?search=Zurich` находит Zürich Airport | `TestSearchAirports_Unaccent` |
| 7 | `?search=Moscow&limit=2&offset=2` → 2 записи, `meta.total=4` | `TestSearchAirports_Pagination` |
| 8 | p95 ≤ 100 мс при 100 RPS | Нагрузочный тест (вне scope unit/int) |

---

## 16. Нефункциональные требования (Checklist)

- [ ] Заголовок `Cache-Control: public, max-age=3600` устанавливается в хендлере
- [ ] Rate limit middleware: 60 req/min на IP, `429 RATE_LIMITED`, `Retry-After: 60`
- [ ] CORS: наследуется из глобального middleware `router.go` (уже настроен)
- [ ] Логирование в хендлере: `search`, `limit`, `offset`, `duration`, `count` (через `slog.InfoContext`). Без PII
- [ ] Публичный endpoint: **не** оборачивать в `custommw.JWT`
- [ ] `make sqlc` не падает после добавления `airports.sql`
- [ ] `golangci-lint run` чист
- [ ] `go build ./...` проходит
- [ ] `go test ./...` проходит (unit без БД, integration с БД)
- [ ] `README.md` обновлён: раздел **Endpoints**
- [ ] `.env.example` обновлён: `RATE_LIMIT_RPM`, `RATE_LIMIT_BURST`
