# Техническое задание на реализацию: `GET /api/v1/airports`

> **Статус:** Реализован  
> **Endpoint:** `GET /api/v1/airports`  
> **Назначение:** Автодополнение и поиск аэропортов по одной строке (название, IATA, ICAO, город).

---

## 1. Контекст

Endpoint доступен если передаётся корректный JWT-токен. Данные берутся из трёх read-only таблиц (`countries`, `cities`, `airports`). Источники данных обновляются раз в месяц через команду CLI `sync-airports`. Ответы кэшируются на стороне клиента (`Cache-Control: private, max-age=3600`).

Направление зависимостей сохраняется: `Presentation → Application → Domain ← Infrastructure`.

---

## 2. Полный список файлов

### Созданные файлы

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
  011_add_cities_unique_index.up.sql
  011_add_cities_unique_index.down.sql

internal/
  domain/
    entity/
      airport.go                                    # Airport + City + Country entities
    valueobject/
      location.go                                   # Location (lat, lon, elevation)
    repository/
      airport_repository.go                         # AirportRepository interface (Search + Upsert)
      city_repository.go                            # CityRepository interface (Upsert)
      country_repository.go                         # CountryRepository interface (Upsert)

  application/
    query/
      search_airports/
        query.go                                    # SearchAirportsQuery
        result.go                                   # SearchAirportsResult + AirportResult
        handler.go                                  # Handler + UseCase interface
    command/
      sync_airports/
        command.go                                  # Command (DryRun, Progress)
        handler.go                                  # Handler: оркестрирует sync из mwgg + Wikidata
        result.go                                   # Result (Countries, Cities, Airports counts)

  infrastructure/
    geo/
      mwgg/
        client.go                                   # HTTP-клиент mwgg/Airports с exponential-backoff
      wikidata/
        client.go                                   # SPARQL-клиент Wikidata (RU имена) с paging + backoff
      static/
        countries_ru.go                             # Статическая карта ISO-2 → русское название страны
    persistence/
      postgres/
        model/
          airport.go                                # DB-модели Airport, City, Country (документационные)
        mapper/
          airport_mapper.go                         # sqlc row → entity.Airport (отдельный пакет)
        queries/
          airports.sql                              # sqlc-запрос SearchAirports
        repository/
          airport_repository.go                     # AirportRepository: Search (sqlc) + Upsert (raw pgx)
          city_repository.go                        # CityRepository: Upsert (raw pgx)
          country_repository.go                     # CountryRepository: Upsert (raw pgx)

  presentation/
    http/
      api/
        v1/
          airport/
            search/
              handler.go                            # HTTP handler
              dto.go                                # SearchParams + Response types
      httpx/
        error.go                                    # StructuredErrorBody + WriteStructuredError
    cli/
      sync_airports.go                              # cobra-команда sync-airports [--dry-run]

tests/
  unit/
    airport_search_params_test.go
  integration/
    airport_repository_test.go
  application/
    airports_search_http_test.go
```

### Изменённые файлы

```
config/config.go          # + RateLimitConfig
config/container.go       # + App.SyncAirports, Http{} и App{} вложенные группы
presentation/http/
  router.go               # + GET /api/v1/airports; rate limiter на весь /api/v1
  httpx/response.go       # + WriteValidationError
```

---

## 3. База данных

Таблицы создаются в схеме `public`. Каждая миграция выполняет одно логическое действие.

### Миграции 006–011

**006** — расширение `unaccent`:
```sql
-- 006_enable_unaccent.up.sql
CREATE EXTENSION IF NOT EXISTS unaccent;
```

**007** — расширение `pg_trgm`:
```sql
-- 007_enable_pg_trgm.up.sql
CREATE EXTENSION IF NOT EXISTS pg_trgm;
```

**008** — таблица `countries`:
```sql
-- 008_create_countries.up.sql
CREATE TABLE countries (
    iso2 char(2)       PRIMARY KEY,
    name varchar(100)  NOT NULL
);
```

**009** — таблица `cities` + GIN-индекс:
```sql
-- 009_create_cities.up.sql
CREATE TABLE cities (
    id           serial       PRIMARY KEY,
    name         varchar(100) NOT NULL,
    state        varchar(100),
    timezone     varchar(50),
    country_iso2 char(2)      NOT NULL REFERENCES countries(iso2)
);

CREATE INDEX cities_name_trgm_idx
    ON cities USING gin (lower(name) gin_trgm_ops);
```

**010** — таблица `airports` + индексы:
```sql
-- 010_create_airports.up.sql
CREATE TABLE airports (
    icao         char(4)      PRIMARY KEY,
    iata         char(3)      UNIQUE,          -- nullable: малые аэропорты без IATA
    name         varchar(200) NOT NULL,
    location     float8[],                     -- [latitude, longitude], индекс от 1
    elevation_ft int,
    city_id      int          NOT NULL REFERENCES cities(id)
);

CREATE INDEX airports_name_trgm_idx
    ON airports USING gin (lower(name) gin_trgm_ops);

CREATE INDEX airports_iata_upper_idx ON airports (upper(iata));
CREATE INDEX airports_icao_upper_idx ON airports (upper(icao));
```

> **Порядок элементов массива:** `location[1]` — широта (lat), `location[2]` — долгота (lon).
> Это инвертировано от традиционного географического порядка [lon, lat] — важно учитывать в Upsert и маппере.

**011** — уникальный индекс на `cities` (для idempotent Upsert):
```sql
-- 011_add_cities_unique_index.up.sql
CREATE UNIQUE INDEX cities_uniq_name_state_country
    ON cities (lower(name), COALESCE(lower(state), ''), country_iso2);
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
        a.location[1]           AS lat,
        a.location[2]           AS lon,
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
    JOIN cities    c  ON c.id       = a.city_id
    JOIN countries co ON co.iso2    = c.country_iso2
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

> **Параметры:** `search`, `search_prefix`, `search_like`, `country_filter` (всегда NULL — фильтр по стране не передаётся из приложения), `limit_val`, `offset_val`.
>
> **Порядок колонок:** `location[1] = lat`, `location[2] = lon`.
>
> **Особенность sqlc:** Из-за оконной функции `COUNT(*) OVER()` и CASE-выражения sqlc может сгенерировать неточные типы для `total_count` и `rank`. После `make sqlc` проверить `db/airports.sql.go` — при несоответствии добавить `overrides` в `sqlc.yaml`.

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

// Airport is the core aggregate for the airport search feature.
// IATA may be nil for small airports without an IATA code.
type Airport struct {
    ICAO     string
    IATA     *string
    Name     string
    Location valueobject.Location
    City     City
    Country  Country
}
```

### 5.3 Repository interfaces

**`internal/domain/repository/airport_repository.go`**

```go
package repository

import (
    "context"
    "api/internal/domain/entity"
)

// AirportFilter carries the normalised search parameters.
type AirportFilter struct {
    Search string // trimmed and space-collapsed
    Limit  int
    Offset int
}

// AirportSearchResult carries a page of airports plus the total count.
type AirportSearchResult struct {
    Airports   []entity.Airport
    TotalCount int64
}

// AirportRepository is the read/write-port for airports.
// The concrete implementation lives in infrastructure/persistence.
type AirportRepository interface {
    Search(ctx context.Context, f AirportFilter) (AirportSearchResult, error)
    Upsert(ctx context.Context, icao string, iata *string, name string, lat, lon float64, elevationFt *int, cityID int) error
}
```

> **Важно:** `AirportFilter` не содержит поле `Country` — фильтрация по стране убрана из query-side API. Параметр `@country_filter` в SQL всегда получает NULL.

**`internal/domain/repository/city_repository.go`**

```go
// CityRepository is the write-port for the airport-sync command.
type CityRepository interface {
    Upsert(ctx context.Context, name string, state *string, timezone, countryISO2 string) (int, error)
}
```

**`internal/domain/repository/country_repository.go`**

```go
// CountryRepository is the write-port for the airport-sync command.
type CountryRepository interface {
    Upsert(ctx context.Context, iso2, name string) error
}
```

---

## 6. Application слой

### 6.1 Query — `internal/application/query/search_airports/`

**`query.go`**
```go
package searchairports

// Query is the input for the SearchAirports use-case.
type Query struct {
    Search string
    Limit  int
    Offset int
}
```

> `Country` поле отсутствует — фильтрация по стране не поддерживается на уровне use-case.

**`result.go`** — LocationResult, CityResult, CountryResult, AirportResult, Result (аналогично исходному спеку).

**`handler.go`** — транслирует Query → AirportFilter → AirportSearchResult → Result. Маппинг entity → use-case result — inline.

### 6.2 Command — `internal/application/command/sync_airports/`

**`command.go`**
```go
package syncairports

import "io"

// Command carries the parameters for the sync-airports use-case.
type Command struct {
    DryRun   bool      // не записывает в БД
    Progress io.Writer // если non-nil, получает строки прогресса
}
```

**`result.go`**
```go
type Result struct {
    Countries int
    Cities    int
    Airports  int
}
```

**`handler.go`** — оркестрирует sync:
1. Fetch аэропортов из `AirportSource` (mwgg)
2. Fetch RU-имён аэропортов из `TranslationSource` (Wikidata)
3. Fetch RU-имён городов из `TranslationSource` (Wikidata)
4. Upsert стран через `CountryRepository`
5. Upsert городов через `CityRepository` (dedup по `cityKey{name, state, country}`)
6. Upsert аэропортов через `AirportRepository`

Интерфейсы источников данных определены в этом же пакете:

```go
type AirportSource interface {
    Fetch(ctx context.Context) ([]AirportRecord, error)
}

type TranslationSource interface {
    FetchAirportNamesRU(ctx context.Context) (map[string]string, error)
    FetchCityNamesRU(ctx context.Context) (map[string]string, error)
}

type CountryNameSource interface {
    NameRU(iso2 string) string
}
```

---

## 7. Infrastructure слой

### 7.1 DB-модели — `internal/infrastructure/persistence/postgres/model/airport.go`

Документационные структуры, зеркалящие схему БД. В репозитории не используются напрямую.

### 7.2 Mapper — `internal/infrastructure/persistence/postgres/mapper/airport_mapper.go`

```go
package mapper

// ToAirportDomain converts a sqlc row to a domain entity.
func ToAirportDomain(row db.SearchAirportsRow) entity.Airport {
    elevFt := ptrInt32ToInt(row.ElevationFt)

    var lon, lat float64
    if row.Lon != nil { lon = *row.Lon }
    if row.Lat != nil { lat = *row.Lat }

    timezone := ""
    if row.CityTimezone != nil { timezone = *row.CityTimezone }

    return entity.Airport{
        ICAO: row.Icao,
        IATA: row.Iata,
        Name: row.AirportName,
        Location: valueobject.Location{
            Latitude:    lat,
            Longitude:   lon,
            ElevationFt: elevFt,
        },
        City: entity.City{
            ID:       int(row.CityID),
            Name:     row.CityName,
            State:    row.CityState,
            Timezone: timezone,
        },
        Country: entity.Country{
            ISO2: row.CountryIso2,
            Name: row.CountryName,
        },
    }
}
```

> `row.Iata` — тип `*string` (sqlc генерирует nullable pointer). `row.Lat`/`row.Lon` — `*float64`.

### 7.3 AirportRepository — `internal/infrastructure/persistence/postgres/repository/airport_repository.go`

```go
type AirportRepository struct {
    queries *db.Queries
    pool    *pgxpool.Pool
}

func NewAirportRepository(queries *db.Queries, pool *pgxpool.Pool) *AirportRepository
```

- **`Search`**: параметры sqlc — `Search`, `SearchPrefix`, `SearchLike`, `LimitVal`, `OffsetVal`. `CountryFilter` **не передаётся** (поле отсутствует в `SearchAirportsParams`). Маппинг через `mapper.ToAirportDomain`.
- **`Upsert`**: raw pgx-запрос `INSERT … ON CONFLICT (icao) DO UPDATE`. Сохраняет `ARRAY[lat, lon]::float8[]`.

### 7.4 CityRepository

Upsert через raw pgx, `ON CONFLICT (lower(name), COALESCE(lower(state),''), country_iso2) DO UPDATE SET timezone = EXCLUDED.timezone`. Требует миграции 011.

### 7.5 CountryRepository

Upsert через raw pgx, `ON CONFLICT (iso2) DO UPDATE SET name = EXCLUDED.name`.

### 7.6 Geo-клиенты

| Пакет | Интерфейс | Описание |
|-------|-----------|----------|
| `geo/mwgg` | `AirportSource` | HTTP GET `github.com/mwgg/Airports/master/airports.json`, retry 4× с backoff 3s→6s→12s |
| `geo/wikidata` | `TranslationSource` | SPARQL endpoint, paging по 10 000 записей, retry 4× с backoff |
| `geo/static` | `CountryNameSource` | Статическая карта ISO-2 → русское название (~250 стран) |

---

## 8. Presentation слой

### 8.1 DTO — `internal/presentation/http/api/v1/airport/search/dto.go`

```go
// SearchParams содержит query-параметры запроса.
type SearchParams struct {
    Search string `schema:"search"  validate:"required"`
    Limit  int    `schema:"limit"   validate:"omitempty,min=1,max=100"`
    Offset int    `schema:"offset"  validate:"omitempty,min=0,max=10000"`
    Lang   string `schema:"lang"    validate:"omitempty"`
}
```

> Параметр `country` **удалён** из DTO. Параметр `lang` присутствует, но не используется в текущей реализации.

Типы ответа (`LocationResponse`, `CityResponse`, `CountryResponse`, `AirportResponse`, `MetaResponse`, `SearchResponse`) аналогичны исходному спеку.

### 8.2 HTTP Handler — `internal/presentation/http/api/v1/airport/search/handler.go`

```go
const DefaultLimit = 20
const DefaultOffset = 0

// NormalizeSearch — экспортированная функция (для unit-тестирования).
func NormalizeSearch(raw string) (string, error)
```

**Логика хендлера:**
1. `NormalizeSearch(q.Get("search"))` — TrimSpace + collapse пробелов + проверка `len([]rune) >= 2`.
2. При ошибке → `WriteStructuredError(400, "INVALID_SEARCH", …)`.
3. `parseIntDefault` для `limit`/`offset`.
4. Валидация через `validator.Struct(SearchParams{…})` → `WriteValidationError` при ошибке.
5. `useCase.Handle(ctx, Query{Search, Limit, Offset})` (без Country).
6. Логирование `search`, `limit`, `offset`, `count`, `duration_ms` через `slog.InfoContext`.
7. `Cache-Control: private, max-age=3600`.
8. `200` с `SearchResponse`.

**Swag-аннотации:**
```go
//	@Summary      Search airports
//	@Description  Full-text search over airports and cities with ranking and pagination.
//	@Tags         Airports
//	@Produce      json
//	@Param        search  query     string  true   "Search string (min 2 chars)"
//	@Param        limit   query     int     false  "Max results (1–100, default 20)"
//	@Param        offset  query     int     false  "Pagination offset (0–10000)"
//	@Success      200     {object}  SearchResponse
//	@Failure      400     {object}  httpx.StructuredErrorBody
//	@Failure      429     {object}  httpx.StructuredErrorBody
//	@Failure      500     {object}  httpx.StructuredErrorBody
//	@Security     Bearer
//	@Router       /api/v1/airports [get]
```

### 8.3 httpx — error utilities

**`httpx/error.go`** — `StructuredErrorBody`, `StructuredError`, `WriteStructuredError` — для машиночитаемых кодов ошибок.

**`httpx/response.go`** — `WriteJSON`, `WriteError`, `WriteValidationError` — `WriteValidationError` возвращает `ErrorBody{Error: "validation failed", Code: 400, Meta: field→tag}`.

### 8.4 CLI — `internal/presentation/cli/sync_airports.go`

```go
// NewSyncAirportsCommand returns the `sync-airports` cobra command.
// Usage: app sync-airports [--dry-run]
func NewSyncAirportsCommand(uc syncairports.UseCase) *cobra.Command
```

Флаг `--dry-run` — выводит статистику без записи в БД.

---

## 9. Rate Limiting

Rate limiter применяется ко **всему** `/api/v1/*` (не только к `/airports`):

```go
limiter := httprate.NewRateLimiter(
    s.RateLimit,   // cfg.RateLimit.RequestsPerMinute, default 60
    time.Minute,
    httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Retry-After", "60")
        httpx.WriteStructuredError(w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many requests", "")
    }),
)

r.With(limiter.Handler).Route("/api/v1", func(api chi.Router) { … })
```

Переменная окружения `RATE_LIMIT_RPM=60`. При `<= 0` принудительно устанавливается 60.

---

## 10. Конфигурация

### `config/config.go`

```go
// RateLimitConfig controls per-IP request throttling for public endpoints.
type RateLimitConfig struct {
    RequestsPerMinute int // RATE_LIMIT_RPM, default 60
}
```

### `config/container.go`

```go
type Container struct {
    Cfg *Config

    // Infrastructure
    Pool     *pgxpool.Pool
    Queries  *db.Queries
    Kafka    *kafka.Producer
    JWT      *auth.Service
    Validate *validator.Validate

    // App — application layer use-case handlers.
    App struct {
        CreateUser     *createusercmd.Handler
        GetMe          *getmeq.Handler
        SearchAirports *searchairports.Handler
        SyncAirports   *syncairportscmd.Handler
    }

    // Http — presentation layer HTTP handlers.
    Http struct {
        Login      *loginhttp.Handler
        CreateUser *createuserhttp.Handler
        GetMe      *getmehttp.Handler
        Airports   *searchairporthttp.Handler
    }
}
```

Airport-wiring в `Build()`:
```go
airportRepo    := pgrepo.NewAirportRepository(queries, pool)
countryRepo    := pgrepo.NewCountryRepository(pool)
cityRepo       := pgrepo.NewCityRepository(pool)

searchAirports := searchairports.NewHandler(airportRepo)
syncAirports   := syncairportscmd.NewHandler(
    airportRepo, countryRepo, cityRepo,
    mwgg.New(), wikidata.New(), static.CountryNames{},
)
```

---

## 11. Тесты

### 11.1 Unit-тесты — `tests/unit/airport_search_params_test.go`

| Тест | Описание |
|------|----------|
| `TestNormalizeSearch_Trim` | Обрезает пробелы по краям |
| `TestNormalizeSearch_CollapseSpaces` | Сворачивает множественные пробелы |
| `TestNormalizeSearch_TooShort_ReturnsError` | Строка < 2 символов → ошибка |
| `TestNormalizeSearch_Empty_ReturnsError` | Пустая строка → ошибка |
| `TestNormalizeSearch_ExactlyTwo_OK` | Строка из 2 символов — OK |
| `TestDefaultLimit` | DefaultLimit == 20 |
| `TestDefaultOffset` | DefaultOffset == 0 |

### 11.2 Integration-тесты — `tests/integration/airport_repository_test.go`

Требуют `TEST_DATABASE_URL` с запущенным Postgres + миграциями + seed-данными. При отсутствии переменной — `t.Skip`.

| Тест | Проверка |
|------|----------|
| `TestAirportRepository_Search_ByName` | `search=Moscow` → ≥ 1 результат |
| `TestAirportRepository_Search_ByIATA_ExactFirst` | `search=SVO` → первый результат IATA="SVO" |
| `TestAirportRepository_Search_ByICAO_ExactFirst` | `search=UUEE` → первый результат ICAO="UUEE" |
| `TestAirportRepository_Search_Pagination` | `limit=2,offset=2` → ≤ 2 записи, TotalCount ≥ 4 |
| `TestAirportRepository_Search_Unaccent` | `search=Zurich` → LSZH в результатах |

### 11.3 Application (E2E) тесты — `tests/application/airports_search_http_test.go`

Используют `fakeUseCase` (stub), не требуют реальной БД. Поднимают `chi.Router` с реальным HTTP Handler.

| Тест | Запрос | Ожидаемый ответ |
|------|--------|-----------------|
| `TestSearchAirports_Moscow` | `?search=Moscow` | 200, ≥ 1 результат |
| `TestSearchAirports_IATA_SVO` | `?search=SVO` | 200, `data[0].iata = "SVO"` |
| `TestSearchAirports_ICAO_UUEE` | `?search=UUEE` | 200, `data[0].icao = "UUEE"` |
| `TestSearchAirports_TooShort` | `?search=a` | 400, `error.code = "INVALID_SEARCH"` |
| `TestSearchAirports_NoSearch` | (без search) | 400, `error.code = "INVALID_SEARCH"` |
| `TestSearchAirports_Pagination` | `?search=Moscow&limit=2&offset=2` | 200, `len(data) ≤ 2`, `meta.total = 4` |
| `TestSearchAirports_CacheControlHeader` | `?search=Moscow` | Header `Cache-Control: private, max-age=3600` |

---

## 12. Acceptance Criteria

| # | Критерий | Тест-кейс |
|---|----------|-----------|
| 1 | `?search=Moscow` → SVO, DME, VKO, ZIA в первых строках | `TestSearchAirports_Moscow` |
| 2 | `?search=SVO` → Шереметьево первым | `TestSearchAirports_IATA_SVO` |
| 3 | `?search=UUEE` → Шереметьево первым | `TestSearchAirports_ICAO_UUEE` |
| 4 | `?search=a` → `400 INVALID_SEARCH` | `TestSearchAirports_TooShort` |
| 5 | `?search=Zurich` находит Zürich Airport (LSZH) | `TestAirportRepository_Search_Unaccent` |
| 6 | `?search=Moscow&limit=2&offset=2` → 2 записи, `meta.total ≥ 4` | `TestSearchAirports_Pagination` |
| 7 | `sync-airports --dry-run` печатает статистику, не пишет в БД | Ручное тестирование |
| 8 | p95 ≤ 100 мс при 100 RPS | Нагрузочный тест (вне scope) |

---

## 13. Нефункциональные требования (Checklist)

- [x] Заголовок `Cache-Control: private, max-age=3600` устанавливается в хендлере
- [x] Rate limit: `go-chi/httprate`, 60 req/min на IP (по всему `/api/v1`), `429 RATE_LIMITED`, `Retry-After: 60`
- [x] CORS: наследуется из глобального middleware `router.go`
- [x] Логирование: `search`, `limit`, `offset`, `duration_ms`, `count` через `slog.InfoContext`. Без PII
- [x] Endpoint за JWT (`custommw.JWT`)
- [x] `make sqlc` не падает
- [x] `golangci-lint run` чист
- [x] `go build ./...` проходит
- [x] `go test ./...` проходит
- [x] `README.md` обновлён: разделы **Endpoints** и **CLI**
- [x] `.env.example` обновлён: `RATE_LIMIT_RPM`
- [x] `config/container.go` реструктурирован: `Http{}`, `App{}`
- [x] Миграция 011 (unique index на cities) необходима для корректной работы `sync-airports`

---

## 14. Известные расхождения с исходным ТЗ

| # | Что изменилось | Причина |
|---|----------------|---------|
| 1 | `Cache-Control: private` вместо `public` | Ответ зависит от JWT-пользователя → не должен кэшироваться публично |
| 2 | `country` фильтр удалён из DTO, Query, AirportFilter | Упрощение MVP; SQL поддерживает параметр (всегда NULL) |
| 3 | `location[1] = lat`, `[2] = lon` (не lon/lat) | Порядок при Upsert — `ARRAY[lat, lon]` |
| 4 | Rate limiter на весь `/api/v1`, не только `/airports` | Единая точка применения в `router.go` |
| 5 | Добавлен mapper-пакет (`mapper/airport_mapper.go`) | Изолирует sqlc-зависимость от логики репозитория |
| 6 | Добавлен `sync-airports` command (mwgg + Wikidata + static) | Необходим для наполнения данных |
| 7 | Добавлены `CityRepository`, `CountryRepository` | Нужны для sync-airports |
| 8 | Миграция 011 (unique index на cities) | Нужна для idempotent Upsert городов |
