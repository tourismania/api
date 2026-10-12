# Классы обслуживания: как добавить, переименовать, удалить

Класс обслуживания (класс салона) — атрибут **сегмента** перелёта: `cabin_class` у
`flights[].segments[]` в `POST`/`PATCH`/`GET .../offers`. Справочник классов — системные данные в
коде, а не в БД: `internal/domain/offer/flight/cabin_class.go`, тип `flight.CabinClass`.

| Константа | Значение | Смысл |
|---|---|---|
| `CabinClassUnknown` | `""` | класс неизвестен (не передан) — не значение по умолчанию |
| `CabinClassEconomy` | `economy` | эконом |
| `CabinClassComfort` | `comfort` | комфорт (у российских перевозчиков — аналог premium economy) |
| `CabinClassComfortPlus` | `comfort_plus` | комфорт плюс |
| `CabinClassBusiness` | `business` | бизнес |

`flight.CabinClasses()` — известные классы без `Unknown`; это **единственный источник правды**:

- `flight.New` отклоняет сегмент с классом не из списка и не пустым → `flight.ErrCabinClassInvalid`,
  `helpers.BuildFlights` переводит её в `400 validation failed`, `meta` `{"CabinClass":"oneof"}`;
- presentation значения **не перечисляет** (`oneof` в DTO нет) — правило «валидация по слоям»
  (`CLAUDE.md`): справочник — бизнес-правило, а не форма запроса;
- тег `enums` у `cabin_class` в четырёх DTO (`create`/`update` — `FlightSegmentInput`,
  `get`/`get_public` — `FlightSegmentResponse`) — только документация swagger; unit-тест
  `tests/unit/cabin_class_swagger_enums_test.go` сверяет его с `flight.CabinClasses()`;
- будущий `GET /config/offers` (строка бэклога sdd) возьмёт список из `flight.CabinClasses()`.

В БД класс хранится как есть: `offer_flight_segments.cabin_class VARCHAR(16) NOT NULL DEFAULT ''`
(миграция `020`), **без** `CHECK` и PostgreSQL `ENUM` — дубль списка в БД требовал бы синхронной
миграции при каждом расширении набора и превращал рассинхрон в `500`.

## Формат значения

Строчные латинские буквы и `_` (snake_case): `comfort_plus`, а не `comfort+` или `Comfort Plus`.

- `+` в query нужно кодировать (`comfort%2B`), а незакодированный `+` читается как пробел;
- значения ложатся в имена констант и enum клиентов (TypeScript, Swift, Kotlin) без экранирования;
- так устроен отраслевой набор (`premium_economy`).

Длина — не больше 16 символов (`VARCHAR(16)`); более длинное значение потребует миграции столбца.

---

## Добавить класс

Миграция БД и правка presentation **не нужны**.

1. В `internal/domain/offer/flight/cabin_class.go` добавьте константу и вставьте её в
   `CabinClasses()` на место по старшинству (от эконома к бизнесу):
   ```go
   CabinClassFirst CabinClass = "first"
   ...
   return []CabinClass{..., CabinClassBusiness, CabinClassFirst}
   ```
2. Тег `enums` у `cabin_class` в четырёх DTO (см. выше) — добавьте значение в том же порядке, что в
   `CabinClasses()` (первым идёт пустое значение: `enums:",economy,…,first"`).
3. Тесты:
   - `tests/unit/flight_factory_test.go` — новый класс в `TestNewFlight_CabinClassValid_KeptAsIs`,
     обновите `TestCabinClasses_ReturnsFourKnownClasses`;
   - `tests/application/offer_flight_cabin_class_http_test.go` — новый класс в
     `TestOffersHTTP_CreateFlight_EachCabinClass_CardReturnsClass`;
   - тесты используют `first` и `premium_economy` как примеры **недопустимых** значений. Если
     добавляете одно из них — найдите его в тестах и замените другим недопустимым значением:
     ```bash
     grep -rn '"first"\|premium_economy' tests/
     ```
4. `README.md`, абзац «Класс обслуживания сегмента перелёта» — список значений.
5. Прогоните сборку, тесты и линтер (см. `CLAUDE.md`, Validation Gates), затем `make swag` и проверьте,
   что у `cabin_class` в `docs/swagger/swagger.json` новое значение в `enum`.
6. Вне api:
   - SDD (`tourismania/sdd`): список классов — часть спеки `offers/flights`; заведите change
     (`/opsx:propose`) до правки api;
   - web: тип `FlightClass` и подписи классов в редакторе перелёта — иначе web не даст выбрать класс и
     не покажет его.

## Переименовать класс

Опаснее добавления: в БД уже есть сегменты со старым значением. Чтение их не ломается (на чтении класс
не проверяется), но `PATCH` с тем же набором перелётов получит `400` от `flight.New`. Для клиентов это
**ломающее изменение контракта** — согласуйте его в SDD и с web.

1. Проверьте, есть ли данные:
   ```sql
   SELECT count(*) FROM offer_flight_segments WHERE cabin_class = '<старое>';
   ```
2. Если строки есть — миграция данных `make migrate-new name=rename_cabin_class_<старое>`
   (одно действие — одна миграция): up — `UPDATE offer_flight_segments SET cabin_class = '<новое>'
   WHERE cabin_class = '<старое>'`, down — обратный `UPDATE`.
3. Поменяйте значение константы (имя константы — тоже, если оно отражает значение), тег `enums`,
   тесты и README — как при добавлении.
4. Выкатывайте миграцию **вместе** с кодом: старый код отклонит новое значение при `PATCH`, новый —
   старое. Проверьте миграцию на копии данных: `make migrate-up`, `make migrate-down`, `make migrate-up`.

## Удалить класс

Тоже ломающее изменение контракта, по тем же причинам, что и переименование.

1. Проверьте данные тем же запросом, что при переименовании.
2. Если строки есть — решите с продуктом, во что их перевести: в `''` (класс неизвестен) или в другой
   класс, — и сделайте это миграцией данных (down — с комментарием, что исходное значение не
   восстанавливается, если перевод был в `''`).
3. Удалите константу из `CabinClasses()` и сам тип-константу, значение из тега `enums`, обновите
   тесты (значение переходит в список недопустимых), README, SDD-спеку и web — как при добавлении.
