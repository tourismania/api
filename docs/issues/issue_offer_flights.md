# [feat] Перелёты (Flight) в составе Offer: хранение, пересадки, вычисление длительности

> **GitHub issue:** [№20](https://github.com/tourismania/api/issues/20)
> **Зависит от:** [№12](https://github.com/tourismania/api/issues/12) (Offer)


## Контекст

Агент, создавая или редактируя предложение (`offer`), должен иметь возможность указать один или несколько перелётов: из аэропорта A в аэропорт B, возможно с пересадками через промежуточные аэропорты. Аэропорт — уже существующая сущность `Airport` (справочник, PK — `icao`). Каждый перелёт — упорядоченная последовательность сегментов (беспосадочных перегонов); клиенту важно видеть не только сырые данные, но и производные величины: общее время в пути и время каждой пересадки.

Перелёты сохраняются **в рамках существующих `POST`/`PATCH /api/v1/offers`** — новых HTTP-маршрутов не добавляется, только новый ключ `flights` в теле запроса и в детальном ответе. Это продолжение issue №12 (`Offer`) — расширяет ту же сущность дочерним агрегатом, как и было заложено в комментарии `entity.Offer` ("child entities (flights/hotels/trips) added in a later iteration").

**Инварианты:**

- **1 перелёт = 1 или более сегментов**, сегменты хранятся в отдельной таблице `offer_flight_segments`, привязанной к `offer_flights.id` (не напрямую к `offer_id`) — сегмент без родительского перелёта не существует.
- **Аэропорт сегмента — существующий `Airport`**, идентифицируется по `icao` (не `int id`: у `Airport` нет суррогатного id, PK — `icao char(4)`, см. `internal/domain/entity/airport.go` и миграцию `010_create_airports`).
- **Непрерывность маршрута**: аэропорт прилёта сегмента N обязан совпадать с аэропортом вылета сегмента N+1. "Аэропорт A" и "аэропорт B" перелёта — это не отдельные поля, а производные: вылет первого сегмента и прилёт последнего.
- **Хронология строгая**: `departure_at < arrival_at` внутри каждого сегмента; `departure_at` сегмента N+1 строго больше `arrival_at` сегмента N (пересадка не может быть нулевой или отрицательной длительности) → иначе `400`.
- **`OfferFlightManager` не проверяет роль/владение офером повторно** — вызывается только из `create_offer`/`update_offer` **после** успешного `OfferManager.Insert`/`Update`, которые уже проверили роль (`ROLE_AGENT`/`ROLE_SUPER_ADMIN`) и агентство. Дублирования проверки владения здесь нет.
- **Вычисление общего времени полёта и времени пересадок — только on-the-fly** (методы на `entity.Flight`), в БД не материализуется и не кэшируется. Пересчитывается при каждом чтении из уже загруженных `departure_at`/`arrival_at` сегментов.

## Модель данных

### Домен (`internal/domain/`)

- `entity/flight.go`:
  ```go
  // FlightSegment — один беспосадочный перегон.
  type FlightSegment struct {
      DepartureAirportICAO string
      ArrivalAirportICAO   string
      DepartureAt          time.Time
      ArrivalAt            time.Time
  }
  func (s FlightSegment) Duration() time.Duration

  // Layover — время между прилётом одного сегмента и вылетом следующего.
  type Layover struct {
      AirportICAO string
      Duration    time.Duration
  }

  // Flight — перелёт из первого DepartureAirportICAO в последний
  // ArrivalAirportICAO, состоящий из одного или более сегментов.
  type Flight struct {
      ID       int
      OfferID  int
      Segments []FlightSegment // len >= 1, гарантируется фабрикой
  }
  func (f Flight) DepartureAirportICAO() string // Segments[0].DepartureAirportICAO
  func (f Flight) ArrivalAirportICAO() string   // Segments[last].ArrivalAirportICAO
  func (f Flight) TotalDuration() time.Duration // Segments[last].ArrivalAt - Segments[0].DepartureAt
  func (f Flight) Layovers() []Layover          // по одной записи на стык сегментов
  ```
- `factory/flight_factory.go` — `NewFlight(segments []entity.FlightSegment) (entity.Flight, error)`. Чистая (без I/O) проверка структурных инвариантов: непустой список, хронология внутри сегмента, непрерывность маршрута, строго положительная пересадка. Ошибки о несуществующем `icao` сюда не входят — это требует похода в БД, см. `OfferFlightManager` ниже.
- Новые доменные ошибки (`internal/domain/service`, рядом с существующими `Err*` в `offer_manager.go`, либо отдельный файл `flight_errors.go`):
  - `ErrFlightSegmentsEmpty`
  - `ErrFlightSegmentChronologyInvalid` (`arrival_at <= departure_at`)
  - `ErrFlightSegmentDiscontinuous` (стык сегментов не совпадает по аэропорту)
  - `ErrFlightLayoverNonPositive`
  - `ErrFlightAirportNotFound` (`icao` не существует в `airports`)
- `repository/offer_flight_repository.go`:
  ```go
  type OfferFlightRepository interface {
      // FindByOfferID возвращает перелёты офера в порядке сохранения
      // (flights по возрастанию id, сегменты внутри — по sequence).
      FindByOfferID(ctx context.Context, offerID int) ([]entity.Flight, error)
      // ReplaceForOffer атомарно удаляет все существующие flights/segments
      // офера и вставляет переданный набор. Вызывается только внутри
      // TxManager.WithinTx вместе с записью в offers.
      ReplaceForOffer(ctx context.Context, offerID int, flights []entity.Flight) error
  }
  ```
- `repository/airport_repository.go` — расширить существующий интерфейс методом массовой проверки существования, например `FindByICAOs(ctx context.Context, icaos []string) ([]entity.Airport, error)` (сейчас есть только `Search`/`Upsert`, точечного лукапа по `icao` нет).
- `service/offer_flight_manager.go`:
  ```go
  type OfferFlightManager struct {
      flights  repository.OfferFlightRepository
      airports repository.AirportRepository
  }
  func NewOfferFlightManager(flights repository.OfferFlightRepository, airports repository.AirportRepository) *OfferFlightManager

  // ReplaceForOffer проверяет существование всех icao (батчем через
  // FindByICAOs), сравнивает переданный набор flights с уже сохранённым
  // (FindByOfferID) и, ТОЛЬКО если они отличаются, вызывает
  // flights.ReplaceForOffer. При совпадении — no-op, БД не трогается.
  // Транзакцию (WithinTx) открывает вызывающий Handler, а не этот метод,
  // — так офер и flights создаются/обновляются в одной транзакции.
  func (m *OfferFlightManager) ReplaceForOffer(ctx context.Context, offerID int, flights []entity.Flight) error
  ```
  Сравнение — по содержимому (аэропорты + таймстемпы), без учёта `ID` (у входящих flights его нет), с учётом порядка (порядок flights и порядок сегментов внутри — значимы).

### Infrastructure (`internal/infrastructure/persistence/postgres/`)

- `repository/offer_flight_repository.go` — реализация `domain/repository.OfferFlightRepository`.
- `queries/offer_flights.sql` (+ `make sqlc`) — insert/select для `offer_flights`, `queries/offer_flight_segments.sql` — insert/select для `offer_flight_segments` (упорядочены по `sequence`).
- `mapper/offer_flight_mapper.go` — маппинг `model.OfferFlight`/`model.OfferFlightSegment` ↔ `entity.Flight`/`entity.FlightSegment`.
- `txmanager/` — реализация порта `application/txmanager.TxManager` (см. Application ниже) на `pgxpool.Pool.BeginTx`; активная `pgx.Tx` кладётся в `context.Context` (приватный ключ пакета). Репозитории (`OfferRepository`, `OfferFlightRepository`) при выполнении sqlc-запросов достают tx из контекста, если он там есть, иначе используют пул напрямую — sqlc уже генерирует `Queries` через интерфейс `DBTX` (`Exec`/`Query`/`QueryRow`), которому удовлетворяют и `pgxpool.Pool`, и `pgx.Tx`, так что переключение прозрачно.

### Application (CQRS, `internal/application/`)

- **Новый порт `application/txmanager`** (по образцу уже существующего `application/apperror` — небольшой самостоятельный пакет верхнего уровня в Application, не внутри конкретного `command/*`): `type TxManager interface { WithinTx(ctx context.Context, fn func(ctx context.Context) error) error }`. Живёт в Application, а не в домене — атомарность между `OfferRepository.Store`/`Update` и `OfferFlightRepository.ReplaceForOffer` нужна только на уровне оркестрации use case'а (`create_offer`/`update_offer`); ни `OfferManager`, ни `OfferFlightManager` сами `WithinTx` не вызывают и о его существовании не знают — транзакцию открывает Handler. Реализация — `internal/infrastructure/persistence/postgres/txmanager/` (см. Infrastructure выше). Переиспользуется будущими multi-repository записями (`hotels`, `trips` по тому же `Offer`), поэтому не привязывается к одним offer'ам.
- `command/create_offer`: `Command.Flights []FlightInput` (или сырые `[]entity.FlightSegment`-группы — решается на этапе реализации). Пустой/отсутствующий список — офер создаётся без перелётов. `Handler.Handle` оборачивает **и** `offerManager.Insert`, **и** (если `Flights` непуст) `offerFlightManager.ReplaceForOffer` в один `txManager.WithinTx` — иначе офер может быть создан без перелётов при сбое второго шага.
- `command/update_offer`: `Command.Flights *[]FlightInput` — **указатель на слайс**, как и `Title`/`Description`/`Status`: `nil` = ключ `flights` в PATCH отсутствовал → не трогаем; ненулевой (в т.ч. `&[]FlightInput{}`) → полная замена *после* сравнения с текущим состоянием (если идентично — no-op, даже транзакция не открывается). `Handler.Handle` оборачивает `offerManager.Update` и (при `Flights != nil`) `offerFlightManager.ReplaceForOffer` в общий `txManager.WithinTx`.
- `query/get_offer`, `query/get_published_offer` — `Result` дополняется `Flights []entity.Flight` (подгружаются через `offerFlightRepository.FindByOfferID` после `FindOwned`/поиска офера). **`query/get_offers` (список) не меняется** — flights в список не подмешиваются, чтобы не тянуть сегменты на каждый офер страницы (N+1); для просмотра перелётов клиент идёт в детальную ручку.
- Ответ `create_offer`/`update_offer` (`Result{ID, UUID}`) **не меняется** — минимальный, как сейчас для `title`/`description`/`status`; подтверждение сохранённых flights клиент получает через `GET /offers/{uuid}`.

### Presentation (HTTP, `internal/presentation/http/`)

- Тело `POST`/`PATCH /api/v1/offers`, новый необязательный ключ:
  ```json
  {
    "flights": [
      {
        "segments": [
          {
            "departure_airport_icao": "UUEE",
            "arrival_airport_icao": "UUDD",
            "departure_at": "2026-08-01T10:00:00Z",
            "arrival_at": "2026-08-01T12:00:00Z"
          }
        ]
      }
    ]
  }
  ```
  Валидация (`go-playground/validator`): `segments` — `required,min=1,dive`; `departure_airport_icao`/`arrival_airport_icao` — `required,len=4`; таймстемпы — `required`. Непрерывность/хронология/пересадка/существование `icao` — доменные ошибки (`400` через `apperror.ErrValidation`), не validator-теги.
- Ответ детальных ручек (`GET /offers/{uuid}`, `GET /public/offers/{uuid}`) — новый ключ `flights`, с вычисленными длительностями:
  ```json
  {
    "flights": [
      {
        "id": 1,
        "departure_airport_icao": "UUEE",
        "arrival_airport_icao": "LFPG",
        "total_duration_seconds": 25200,
        "segments": [
          {"departure_airport_icao": "UUEE", "arrival_airport_icao": "UUDD", "departure_at": "...", "arrival_at": "...", "duration_seconds": 7200},
          {"departure_airport_icao": "UUDD", "arrival_airport_icao": "LFPG", "departure_at": "...", "arrival_at": "...", "duration_seconds": 10800}
        ],
        "layovers": [
          {"airport_icao": "UUDD", "duration_seconds": 7200}
        ]
      }
    ]
  }
  ```
- Новых сентинелов `apperror` не требуется — все ошибки валидации flight переводятся в существующий `apperror.ErrValidation` (`400`) через `apperror.FromDomainError`.
- Сборка зависимостей (`OfferFlightRepository`, `OfferFlightManager`, `TxManager`) — в `config/container.go`.

### Миграции

- `016_create_offer_flights` — таблица `offer_flights (id SERIAL PK, offer_id INT NOT NULL REFERENCES offers(id) ON DELETE CASCADE)`. Индекс `offer_flights_offer_id_idx`.
- `017_create_offer_flight_segments` — таблица `offer_flight_segments (id SERIAL PK, flight_id INT NOT NULL REFERENCES offer_flights(id) ON DELETE CASCADE, sequence SMALLINT NOT NULL, departure_airport_icao CHAR(4) NOT NULL REFERENCES airports(icao), arrival_airport_icao CHAR(4) NOT NULL REFERENCES airports(icao), departure_at TIMESTAMP NOT NULL, arrival_at TIMESTAMP NOT NULL, UNIQUE(flight_id, sequence))`. Индекс `offer_flight_segments_flight_id_idx`.
- Две отдельные миграции — по принципу "1 действие = 1 миграция" (создание каждой таблицы отдельно).

### Прочее

- Swagger — `make swag` (новые поля запроса/ответа offers).
- `README.md` — раздел **Endpoints**: у существующих `POST`/`PATCH /offers` и `GET /offers/{uuid}`/`GET /public/offers/{uuid}` дописать упоминание ключа `flights` (маршрутов не добавляется, поэтому таблица эндпоинтов не меняется по составу, только по описанию).

## Acceptance criteria

- [ ] Миграции `016_create_offer_flights`, `017_create_offer_flight_segments` применяются и откатываются.
- [ ] `POST /offers` с `flights` создаёт офер и перелёты атомарно (одна транзакция); при ошибке валидации flights офер не создаётся вообще (ни офер, ни flights не сохраняются).
- [ ] `PATCH /offers/{uuid}` без ключа `flights` в теле не трогает существующие перелёты офера.
- [ ] `PATCH /offers/{uuid}` с `flights`, идентичными уже сохранённым (тот же порядок, те же аэропорты и таймстемпы) — не производит запись в `offer_flights`/`offer_flight_segments` (no-op).
- [ ] `PATCH /offers/{uuid}` с `flights: []` очищает все перелёты офера (если они были).
- [ ] `PATCH /offers/{uuid}` с изменённым набором `flights` — старые перелёты полностью заменяются новыми в одной транзакции.
- [ ] Валидация: пустой список сегментов, `arrival_at <= departure_at`, разрыв маршрута (аэропорт стыка не совпадает), пересадка `<= 0`, несуществующий `icao` — все → `400`.
- [ ] `GET /offers/{uuid}` и `GET /public/offers/{uuid}` возвращают `flights` с сегментами, `total_duration_seconds` и `layovers` (`airport_icao` + `duration_seconds`), корректно посчитанными для перелётов с 0, 1 и 2+ пересадками.
- [ ] `GET /offers` (список) не изменяет текущий формат ответа — `flights` там нет.
- [ ] Ответы `POST`/`PATCH /offers` не меняют текущий контракт (`{id, uuid}`).
- [ ] Swagger сгенерирован (`make swag`); `README.md` обновлён.
- [ ] Критический путь покрыт: unit — `factory.NewFlight` (все инварианты), `OfferFlightManager.ReplaceForOffer` (no-op при совпадении, замена при отличии, мок-репозитории), `entity.Flight.TotalDuration`/`Layovers` (0/1/2+ пересадок); integration — `OfferFlightRepository` на реальном Postgres (в т.ч. `ReplaceForOffer` внутри реальной транзакции, FK на `airports`); application e2e — `create_offer`/`update_offer` с `flights` (успех, откат транзакции при доменной ошибке), `get_offer`/`get_published_offer` с непустыми `flights`. `go test ./...`, `go build ./...`, `golangci-lint run` — зелёные.

## Negative constraints (чего НЕ делаем)

- Не вводим отдельные HTTP-маршруты для flights — только ключ `flights` в существующих `offers`-ручках.
- Не материализуем/не кэшируем `total_duration`/`layover` в БД — только on-the-fly вычисление при чтении.
- Не поддерживаем частичное обновление отдельного flight по id внутри `PATCH` — только полная замена всего списка `flights` офера (после сравнения на отличия).
- `flights` не добавляется в ответ `GET /offers` (список) и в ответы `POST`/`PATCH /offers` — только в детальные `GET /offers/{uuid}` / `GET /public/offers/{uuid}`.
- `OfferFlightManager` не проверяет роль/владение — это исключительно ответственность `OfferManager`, вызываемого раньше в том же Handler.
- Домен без внешних импортов (`pgx` — только в infrastructure); `TxManager` — интерфейс в Application (`application/txmanager`), не в домене — домен о существовании транзакций не знает. Никаких `log.Fatal`/`os.Exit` вне `main()`; DI только в `config/container.go`.
- Файлы в `db/` и `docs/swagger` (генерируемые) вручную не редактируются.
