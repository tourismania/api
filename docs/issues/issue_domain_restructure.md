# Issue №15 — Реструктуризация domain-слоя: type-first → resource-first

- **GitHub Issue:** №15 «Обсудить реструктуризацию domain-слоя: type-first vs resource-first»
- **Поднято в:** review PR №13
- **Дата решения:** 2026-09-16
- **Статус:** реализовано (ветка `feature/15`)

## Контекст

Изначальная структура `internal/domain/` была type-first: пакеты `entity/`, `enum/`, `event/`, `factory/`, `repository/`, `service/`, `valueobject/`, внутри которых лежали файлы всех агрегатов вперемешку. В review PR №13 предложена альтернатива — resource-first: один агрегат/bounded context = один пакет.

## Решение

**Переходим на resource-first.** Причины:

1. **Масштабирование по агрегатам.** На момент решения в домене уже 4 агрегата (User, Agency, Airport, Offer c Flight), и Offer успел зайти (PR №22). При type-first каждый новый агрегат размазывается по 7 пакетам; при resource-first — добавляется один пакет.
2. **Соответствие best practices Go.** Идиоматичный Go группирует код по функциональной связности (bounded context), а не по техническому виду типа. Пакеты `entity`/`service` — классический анти-паттерн «пакеты-свалки» (аналог `utils`).
3. **Screaming architecture / vertical slice.** Список каталогов домена теперь читается как список бизнес-понятий, а не как учебник по DDD-терминологии.
4. **Устранение stutter.** `repository.UserRepository` → `user.Repository`, `service.AgencyManager` → `agency.Manager` — короче и идиоматичнее.
5. **Инвариант «домен не импортирует внешнее» не страдает.** Весь слой по-прежнему живёт под `internal/domain/` — аудит одним `grep` по каталогу, как и раньше.

Вариант с подкаталогами внутри ресурса (`domain/user/entity/`, `domain/user/services/`) отклонён: в Go это создаёт множество одноимённых микропакетов (`entity` в каждом ресурсе), возвращает stutter и усложняет импорты. Внутри пакета агрегата — плоские файлы.

Миграция выполнена **одним PR** (после мержа Offer): изменение чисто механическое (перемещение + переименование, поведение не меняется), покрыто существующими тестами; тянуть её поэтапно означало бы жить с двумя конвенциями одновременно.

## Целевая структура

```
internal/domain/
  user/     # User, Record, Role, Actor, RightsDescribe(+Factory, Describer),
            # Repository, Creator, Finder, PasswordHasher, событие Registered
  agency/   # Agency, Status, Repository, Manager
  airport/  # Airport, City, Country, Location, Repository, CityRepository, CountryRepository
  offer/    # Offer, Status, Flight/FlightSegment/Layover (+NewFlight),
            # Repository, FlightRepository, Manager, FlightManager
  event/    # Общее ядро: интерфейсы DomainEvent и Bus
```

Зависимости между доменными пакетами (циклы запрещены):

```
user    → agency, event
offer   → agency, user, airport
agency  → (ничего)
airport → (ничего)
event   → (ничего)
```

## Карта переименований (старое → новое)

| Было | Стало |
|---|---|
| `entity.User` / `entity.UserRecord` | `user.User` / `user.Record` |
| `enum.Role`, `enum.Role*` | `user.Role`, `user.Role*` |
| `valueobject.Actor` | `user.Actor` |
| `valueobject.RightsDescribe` (+factory, +service) | `user.RightsDescribe` (+`user.RightsDescribeFactory`, `user.RightsDescriber`) |
| `repository.UserRepository` | `user.Repository` |
| `service.UserCreator` / `service.UserFinder` | `user.Creator` / `user.Finder` |
| `service.PasswordHasher` | `user.PasswordHasher` |
| `service.ErrUserNotPersisted` / `service.ErrActorNotFound` | `user.ErrNotPersisted` / `user.ErrActorNotFound` |
| `event.UserRegistered` | `user.Registered` (код события `user_registered` не менялся) |
| `entity.Agency`, `enum.AgencyStatus*` | `agency.Agency`, `agency.Status*` |
| `repository.AgencyRepository`, `service.AgencyManager` | `agency.Repository`, `agency.Manager` |
| `service.ErrAgencyNotFound` / `ErrAgencyInactive` | `agency.ErrNotFound` / `agency.ErrInactive` |
| `entity.Airport/City/Country`, `valueobject.Location` | `airport.Airport/City/Country/Location` |
| `repository.AirportRepository/CityRepository/CountryRepository` | `airport.Repository/CityRepository/CountryRepository` |
| `repository.AirportFilter` / `AirportSearchResult` | `airport.Filter` / `airport.SearchResult` |
| `entity.Offer`, `enum.OfferStatus*`, `entity.OfferTitleMaxLength` | `offer.Offer`, `offer.Status*`, `offer.TitleMaxLength` |
| `entity.Flight/FlightSegment/Layover`, `factory.NewFlight` | `offer.Flight/FlightSegment/Layover`, `offer.NewFlight` |
| `repository.OfferRepository/OfferFlightRepository` | `offer.Repository` / `offer.FlightRepository` |
| `repository.OfferFilter` / `OfferListResult` | `offer.Filter` / `offer.ListResult` |
| `service.OfferManager` / `OfferFlightManager` | `offer.Manager` / `offer.FlightManager` |
| `service.ErrOffer*`, `service.ErrInsufficientRole`, `service.ErrFlightAirportNotFound`, `factory.ErrFlight*` | `offer.Err*` |

## Negative constraints (соблюдены)

- Поведение не меняется: ни один публичный контракт (HTTP, CLI, Kafka-события, SQL) не затронут; код события `user_registered` и все строковые значения enum'ов сохранены.
- Миграция — отдельный PR, не смешана с feature-работой.

## Связанные обсуждения

- Issue №14 — структура тестов (`tests/unit|integration|application` vs рядом с кодом) — решается отдельно; текущая структура тестов в этом PR не менялась.
