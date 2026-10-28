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
  domain/         # Доменный слой, resource-first: 1 агрегат = 1 пакет (user, agency, airport, offer, money + ядро event); дочерние агрегаты — подпакетами (offer/flight)
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

### airlines

Синхронизирует справочник авиакомпаний (и их страны) из Wikidata: все элементы с кодом ICAO
авиакомпании — действующие и прекратившие работу — с кодами ICAO и IATA, английским названием,
русским (только у российских), страной и датой прекращения деятельности. Запись — «вставить или
обновить» по QID элемента, id записей сохраняются; пропавшие из Wikidata не удаляются, а скрываются
из поиска (дубли, слитые в источнике, и пропавшие). Неоднозначности источника пишутся JSON-журналом в
stderr. Ожидаемое время выполнения: ~30 секунд для ~3 400 авиакомпаний.

Подробно (запрос к Wikidata, правила разбора, дубли и пропавшие, тайминги) — [docs/cli/sync_airlines.md](docs/cli/sync_airlines.md); события журнала аномалий и нормальные значения счётчиков — раздел [«Мониторинг»](docs/cli/sync_airlines.md#5-мониторинг).

```bash
# Preview без записи в БД (запросы, разбор и журнал аномалий выполняются)
go run ./cmd/cli airlines sync --dry-run

# Полная синхронизация (запускать раз в месяц и сразу после выкладки миграций 021/022)
go run ./cmd/cli airlines sync

# Журнал аномалий — отдельно от прогресса
go run ./cmd/cli airlines sync 2> airlines-sync.jsonl

# production
docker compose exec tourismania_app /app/cli airlines sync
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
| GET   | /api/v1/airlines | JWT (любая роль) | Поиск авиакомпаний: `search` (от 3 символов) — подстрока английского/русского названия или названия страны; `country` (ISO-2); `ids` (до 100 id через запятую); `limit`/`offset`. Только выбираемые записи (без дублей и пропавших), действующие выше закрывшихся; по кодам не ищет. `id` из выдачи — `airline_id` сегмента перелёта |
| GET   | /api/v1/airlines/{id} | JWT (любая роль) | Авиакомпания по id, в т. ч. дубль или пропавшая из источника, которых поиск не возвращает (поля — как у элемента поиска); нет записи → `404 airline not found` |
| GET   | /api/v1/currencies | JWT (любая роль) | Список доступных валют из системного справочника: `{"data":[{"code":"RUB","exponent":2},…]}` — код ISO 4217 и число знаков после точки; `Cache-Control: private, max-age=3600` |
| POST  | /api/v1/offers   | JWT, ROLE_AGENT/ROLE_SUPER_ADMIN, своё агентство | Создать offer (agency_id выводится из агентства текущего пользователя); опциональный ключ `flights` создаёт перелёты атомарно вместе с offer |
| GET   | /api/v1/offers   | JWT (любая роль) | Список offer своего агентства (пагинация + фильтры, любой статус); `flights` в ответ не подмешивается |
| GET   | /api/v1/offers/{uuid} | JWT (любая роль) | Получить один offer своего агентства (любой статус) вместе с `flights`; чужое агентство → `404` |
| GET   | /api/v1/public/offers/{uuid} | public | Получить опубликованный offer по `uuid` без авторизации, вместе с `flights`; не-`published` → `404` |
| PATCH | /api/v1/offers/{uuid} | JWT, ROLE_AGENT/ROLE_SUPER_ADMIN, своё агентство | Частичное обновление offer (title/description/status/flights) |
| DELETE| /api/v1/offers/{uuid} | JWT, ROLE_AGENT/ROLE_SUPER_ADMIN, своё агентство | Soft delete offer |
| GET   | /api/docs        | public | Swagger UI                       |
| GET   | /healthz         | public | Healthcheck                      |

**Статусы offer:** `draft` (черновик, редактируется) → `ready` (заполнен и сохранён, но агент ещё не решил публиковать) → `published` (виден всем). `draft` и `ready` видны только пользователям своего агентства (любая роль); переход между статусами свободный, делается через `PATCH .../offers/{uuid}` (`status`).

**Видимость offer (read-side):** `GET /api/v1/offers` и `GET /api/v1/offers/{uuid}` — приватные эндпоинты (нужен JWT), список/offer скоупится строго на агентство текущего пользователя, любой статус; роль на видимость не влияет — `ROLE_USER` видит то же, что и `ROLE_AGENT`/`ROLE_SUPER_ADMIN` в пределах своего агентства. Offer другого агентства для приватных ручек не существует (`404`) — 1 пользователь = 1 агентство, кросс-агентского доступа нет даже у `ROLE_SUPER_ADMIN`. `GET /api/v1/public/offers/{uuid}` — отдельная, полностью анонимная ручка («ссылка, которой делятся с клиентом»): без токена, видит только `published` (независимо от агентства), иначе `404`.

**Владение по агентству (write-side):** создавать/изменять/удалять offer может только `ROLE_AGENT` или `ROLE_SUPER_ADMIN`, принадлежащий **тому же** агентству, что и offer (`offer.agency_id == user.agency_id`); чужое агентство → `404` (не раскрываем существование чужого offer), недостаточная роль (`ROLE_USER`) → `403`. Особого режима для `ROLE_SUPER_ADMIN` нет — 1 пользователь = 1 агентство касается всех ролей одинаково.

**Перелёты (`flights`) offer (issue №20):** новых маршрутов не добавляется — перелёты сохраняются и читаются через существующие `POST`/`PATCH`/`GET .../offers`. Каждый flight — упорядоченный список сегментов (`departure_airport_icao`/`arrival_airport_icao`/`departure_at`/`arrival_at`); аэропорт прилёта сегмента N обязан совпадать с аэропортом вылета сегмента N+1, пересадка между сегментами — строго `> 0`. В `POST` ключ `flights` опционален (список groups сегментов); offer и его flights создаются в одной транзакции — при ошибке валидации flights offer не создаётся. В `PATCH` ключ `flights` — указатель на список, как `title`/`description`/`status`: отсутствует в теле → перелёты не трогаются; `flights: []` → все перелёты офера удаляются; непустой список → полная замена (diff-then-replace: идентичный набор — no-op, БД не трогается).

**Цена перелёта (issue №26):** у каждого flight обязательны `price` и `currency` (на уровне перелёта, не сегмента): `{"price":"12345.50","currency":"RUB","segments":[...]}`. `currency` — код ISO 4217 из системного справочника валют (`internal/domain/money`, сейчас `RUB`, `USD`, `EUR`, `TRY`); `price` — неотрицательная десятичная **строка** (не число JSON), до 10 цифр в целой части и не больше знаков после точки, чем у валюты. Ошибки → `400 validation failed` с `meta` `Price`/`Currency`. В ответе `price` — ровно с числом знаков валюты (`"12345.5"` → `"12345.50"`). Хранится целым числом минорных единиц (`offer_flights.price_minor` + `currency`), без float и без `CHECK` в БД. В `PATCH`, если маршрут и время перелётов не изменились, а изменились только цена/валюта, обновляется только цена — `id` перелётов сохраняются. Миграция `018` удаляет перелёты, сохранённые до появления цены. Как добавить валюту или изменить её число знаков — [`docs/currencies.md`](docs/currencies.md), почему так — [`FAQ.md`](FAQ.md) №2.

**Комментарий менеджера к перелёту (issue №31):** у flight необязательное поле `manager_comment` — текст для клиента на уровне перелёта, не сегмента; виден и в публичной карточке (`/public/offers/{uuid}`). Запрос: `{"price":"12345.50","currency":"RUB","manager_comment":"Багаж оплачивается отдельно","segments":[...]}`; отсутствие поля, `null` и `""` — «комментария нет». В ответе `GET` поле есть всегда: `{"id":1,"price":"12345.50","currency":"RUB","manager_comment":"Багаж оплачивается отдельно","departure_airport_icao":"UUEE",...}`, без комментария — `"manager_comment":""`. До 2000 символов (символов, не байт), длиннее → `400 validation failed`, `meta` `{"ManagerComment":"max"}`. Текст хранится и отдаётся как есть — без обрезки пробелов, экранирования и фильтрации (кавычки, `\`, `;`, `<>&`, эмодзи допустимы; экранирует клиент при выводе). Защита от SQL-инъекций — только параметры запроса (`$n`), а не фильтрация текста. Управляющие символы, кроме `\t`, `\n`, `\r` (в т. ч. `\u0000`, который PostgreSQL не хранит), → `400`, `meta` `{"ManagerComment":"text"}`. В `PATCH`, если маршрут и время перелётов не изменились, а изменились цена/валюта и/или комментарий, они обновляются на месте — `id` перелётов сохраняются. Миграция `019` добавляет `offer_flights.manager_comment TEXT NOT NULL DEFAULT ''` (уже сохранённые перелёты получают `""`).

**Класс обслуживания сегмента перелёта (sdd `add-flight-segment-class`):** у каждого сегмента необязательное поле `cabin_class` — класс салона на уровне сегмента, не перелёта (у сегментов одного перелёта классы могут различаться); виден и в публичной карточке (`/public/offers/{uuid}`). Допустимые значения — `economy`, `comfort`, `comfort_plus`, `business` (строчными, точное совпадение) и `""` — класс неизвестен. Значения по умолчанию нет: отсутствие поля, `null` и `""` — «класс неизвестен», хранится и отдаётся как `""`. Запрос: `{"price":"12345.50","currency":"RUB","segments":[{"departure_airport_icao":"UUEE","arrival_airport_icao":"LFPG","departure_at":"2026-08-01T10:00:00Z","arrival_at":"2026-08-01T14:00:00Z","cabin_class":"business"}]}`. В ответе `GET` поле есть у каждого сегмента всегда: `"segments":[{"departure_airport_icao":"UUEE",...,"duration_seconds":14400,"cabin_class":"business"}]`, без класса — `"cabin_class":""`. Другое значение (`first`, `premium_economy`, `comfort+`, `Business`, `" economy"`) → `400 validation failed`, `meta` `{"CabinClass":"oneof"}`; не строка → `400 field "…" has the wrong type`. Список значений живёт только в домене (`flight.CabinClasses()`), в presentation он не дублируется (ни `oneof`, ни `enums`): поле DTO имеет тип `flight.CabinClass`, и swag строит `enum` схемы `flight.CabinClass` из констант домена. Как добавить, переименовать или удалить класс — [`docs/cabin-classes.md`](docs/cabin-classes.md). В `PATCH`, если маршрут и время перелётов не изменились, а изменились классы сегментов (вместе с ценой/валютой/комментарием или без них), они обновляются на месте — `id` перелётов сохраняются. Миграция `020` добавляет `offer_flight_segments.cabin_class VARCHAR(16) NOT NULL DEFAULT ''` (уже сохранённые сегменты получают `""`).

**Авиакомпания сегмента перелёта (sdd `add-flight-segment-airline`) — ломающее изменение для клиентов `flights`:** у каждого сегмента **обязательное** поле `airline_id` — id авиакомпании из поиска `GET /api/v1/airlines` (на уровне сегмента: сегменты одного перелёта могут выполнять разные авиакомпании). Перевозчик ещё неизвестен — `1` (служебная запись «Неизвестная»); api ничего не подставляет. Запрос: `{"price":"12345.50","currency":"RUB","segments":[{"departure_airport_icao":"UUEE","arrival_airport_icao":"LFPG","departure_at":"2026-08-01T10:00:00Z","arrival_at":"2026-08-01T14:00:00Z","airline_id":1}]}`. Ссылка — по id записи справочника, а не по коду: коды ICAO не уникальны и переназначаются после закрытия авиакомпаний, а id в разных окружениях (локально, prod) разные — клиент всегда выбирает авиакомпанию поиском; фиксирован только `1`. Сегмент ссылается именно на выбранную запись: переход кода к другой авиакомпании и синхронизация справочника его не меняют. В ответе `GET` у сегмента всегда `airline_id` и `airline` — объект (не `null`) с текущими данными справочника: `"airline_id":42,"airline":{"id":42,"icao":"AFL","name":"Aeroflot","name_ru":"Аэрофлот","dissolved_on":null}`, для неизвестной — `"airline_id":1,"airline":{"id":1,"icao":null,"name":"Unknown","name_ru":"Неизвестная","dissolved_on":null}`; `name_ru` — `""` у нероссийских; кодов IATA нет; видно и в публичной карточке. Ошибки: нет поля / `null` / `0` → `400 validation failed`, `meta` `{"AirlineID":"required"}`; не целое число (`"AFL"`, `"42"`, `1.5`) → `400 field "…" has the wrong type`; id нет в справочнике (в т. ч. отрицательный) → `400 validation failed: flight segment references an unknown airline`; `airline`/`airline_icao` в сегменте или `airline_id` у перелёта → `400` unknown field. Закрывшуюся авиакомпанию указать можно. Дубль или пропавшую из Wikidata запись (поиск их не возвращает) — только если оффер уже ссылался на неё хотя бы в одном сегменте: старые офферы сохраняются без ошибок, новых ссылок на скрытые нет. В `PATCH`, если маршрут и время перелётов не изменились, а изменились только авиакомпании сегментов (вместе с классом/ценой/комментарием или без них), они обновляются на месте — `id` перелётов сохраняются. Миграция `021` создаёт справочник `airlines` с записью «Неизвестная» (`id = 1`), `022` добавляет `offer_flight_segments.airline_id int NOT NULL DEFAULT 1 REFERENCES airlines(id)` — уже сохранённые сегменты получают «Неизвестная». После выкладки: `airlines sync --dry-run`, затем `airlines sync` (до первой синхронизации в справочнике только «Неизвестная»).

**Номер рейса сегмента перелёта (sdd `add-flight-segment-number`):** у каждого сегмента необязательное поле `flight_number` — номер рейса на уровне сегмента, не перелёта (у каждого беспосадочного сегмента свой рейс); виден и в публичной карточке (`/public/offers/{uuid}`). Строка до 10 символов (символов, не байт: `ЮТЮТЮТЮТЮТ` допустим), хранится и отдаётся ровно как передана — формат не проверяется, регистр, пробелы и разделители не меняются (`"su 1234"`, `" U6-773 "`, `"ЮТ-773"` возвращаются как есть). Значения по умолчанию нет: отсутствие поля, `null` и `""` — «номер неизвестен», хранится и отдаётся как `""`. Запрос: `{"price":"12345.50","currency":"RUB","segments":[{"departure_airport_icao":"UUEE","arrival_airport_icao":"LFPG","departure_at":"2026-08-01T10:00:00Z","arrival_at":"2026-08-01T14:00:00Z","airline_id":1,"flight_number":"SU1234"}]}`. В ответе `GET` поле есть у каждого сегмента всегда: `"segments":[{"departure_airport_icao":"UUEE",...,"cabin_class":"","flight_number":"SU1234","airline_id":1,...}]`, без номера — `"flight_number":""`. Длиннее 10 символов → `400 validation failed`, `meta` `{"FlightNumber":"max"}`; не строка → `400 field "…" has the wrong type`; `flight_number` у перелёта или `flightNumber` в сегменте → `400` unknown field. Номер с авиакомпанией сегмента не сопоставляется, уникальность не проверяется. В `PATCH` номера сравниваются точно (`"SU1234"` и `"su 1234"` — разные); если маршрут и время перелётов не изменились, а изменились только номера рейсов (вместе с классом/авиакомпанией/ценой/комментарием или без них), они обновляются на месте — `id` перелётов сохраняются. Миграция `023` добавляет `offer_flight_segments.flight_number VARCHAR(10) NOT NULL DEFAULT ''` (уже сохранённые сегменты получают `""`).

В детальных `GET .../offers/{uuid}` ответ содержит вычисленные `total_duration_seconds` (на весь flight) и `layovers` (по одному на каждую пересадку, с `duration_seconds`) — эти величины не хранятся в БД, а пересчитываются на лету при каждом чтении.

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

Роль хранится в `users.roles` (массив) и резолвится на каждый запрос заново, а не из самого токена — так отозванные/изменённые права применяются немедленно, без переиздания токена. JWT несёт только неизменяемый `uuid` (`Claims.Subject`); `agency_id`/`roles` из БД резолвит доменный сервис `domain/user.Finder` по этому `uuid` — presentation-слой (в т.ч. мидлвари) в БД не ходит.

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
- И роль (для записи), и владение по агентству проверяет доменный `offer.Manager` (`FindOwned` — единая точка проверки владения, используется и чтением, и записью) — не HTTP-мидлварь. Presentation-хендлер читает `uuid` из контекста (`custommw.CurrentUserUUIDFromContext`, без параллельного парсинга и без собственной 401-ветки) и передаёт его в Command/Query; application-хендлер резолвит `agency_id`/`roles` через доменный `user.Finder.Resolve` и строит `user.Actor`, который уже идёт в домен. Ошибки домена (sentinel-ошибки доменных пакетов) presentation-слой не видит напрямую — application-хендлер переводит их в `application/apperror` (`ErrUnauthenticated`/`ErrForbidden`/`ErrNotFound`/`ErrValidation`), и только эти сентинелы определяют HTTP-код.

## Тесты

```bash
go test ./tests/unit/...
go test ./tests/integration/...
go test ./tests/application/...
```

Интеграционным тестам нужен Postgres с применёнными миграциями: `TEST_DATABASE_URL=postgres://…`, без неё они пропускаются. Данные тесты засевают сами и удаляют после себя — загружать справочник аэропортов (`airports sync`) не нужно. Сиды поиска аэропортов — `tests/integration/airport_fixtures_test.go`: коды из цифр (реальные IATA/ICAO — только буквы) и уникальный токен в названиях, поэтому тесты не трогают реальные записи и безопасны даже на базе с загруженным справочником.

## Swagger

При наличии установленного `swag` CLI:

```bash
go install github.com/swaggo/swag/cmd/swag@latest
swag init -g cmd/server/main.go -o docs/swagger
# или make swag
```

После генерации Swagger UI будет доступен на `/api/docs`.

### Тестирование через Postman/Insomnia

- **Актуальный контракт целиком:** `docs/swagger/swagger.json` импортируется
  напрямую (`File → Import`) и в Postman, и в Insomnia — оба поддерживают
  OpenAPI/Swagger 2.0 нативно, без конвертации. Пересобирается через `make
  swag` вместе с кодом, так что коллекцию достаточно переимпортировать после
  правки хендлера — расхождений с реальным API не возникает.
- Чек-лист сценариев по доменам — [`docs/test_cases/test_cases.md`](docs/test_cases/test_cases.md).

## Ключевые архитектурные принципы

1. Доменная сущность ≠ ORM-модель (`domain/user.User` vs `infrastructure/persistence/postgres/model.User`).
2. Репозиторий — интерфейс в домене, реализация в infrastructure.
3. CQRS через `CommandBus` и `QueryBus` (in-memory routing).
4. Доменные события публикуются через интерфейс `event.Bus` (kafka — реализация).
5. DI собирается явно в `internal/app/container.go` — никаких глобалов.
6. Все бизнес-эндпоинты под `/api/v1/`.
