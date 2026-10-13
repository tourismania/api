# [feat] Класс обслуживания сегмента перелёта (Flight) в составе Offer

> **GitHub issue:** [№N](https://github.com/tourismania/api/issues/N)
> **Зависит от:** [№31](https://github.com/tourismania/api/issues/31) (комментарий менеджера, обновление на месте `DetailsUpdate`)
> **Источник требований:** изменение SDD `add-flight-segment-class` в `tourismania/sdd`
> (`openspec/changes/add-flight-segment-class/`: proposal, дельта спеки `offers/flights`, design, tasks),
> принято в [sdd PR №23](https://github.com/tourismania/sdd/pull/23).
> **Реализация:** [api PR №35](https://github.com/tourismania/api/pull/35).
> Этот файл — краткая выжимка для api; при расхождении прав SDD.

## Контекст

Класс обслуживания (эконом / комфорт / бизнес) — то, что клиент сравнивает рядом с ценой. Сейчас он
есть только у сегмента в редакторе web и теряется при сохранении: api его не хранит. Без него web не
может перейти на перелёты из api без потери данных.

**Инварианты:**

- **Класс — у сегмента, не у перелёта.** У сегментов одного перелёта классы могут различаться
  (пересадка в другом классе). Поле — `cabin_class` (класс салона; не `booking_class`/`fare_class` —
  это буквенный класс тарифа).
- **Допустимые значения** — ровно `economy`, `comfort`, `comfort_plus`, `business` (строчные,
  snake_case, без `+` и пробелов) и `""` — «класс неизвестен». Сравнение точное: `Business`,
  `" economy"`, `comfort+`, `first`, `premium_economy` → `400`.
- **Без значения по умолчанию.** Отсутствие поля, `null` и `""` — класс неизвестен; хранится и
  отдаётся как `""`. Класс за клиента не подставляется.
- **Список значений — только в домене** (`flight.CabinClasses()`), это справочник, а не форма запроса:
  presentation значения не перечисляет (`oneof` в DTO нет), ошибку поля отдаёт `helpers.BuildFlights`
  (`apperror.FieldError` `CabinClass`/`oneof`). Swagger-список тоже не дублируется (ревью api PR №35):
  поле DTO имеет доменный тип `flight.CabinClass`, swag строит `enum` из его констант; тег `enums`
  не используется, unit-тест следит за типом поля и отсутствием `enums`/`oneof`.
- **Виден везде, где виден перелёт**, включая публичную ссылку; в ответе поле есть всегда.
- **Изменились только классы (и/или цена, комментарий) — обновляются на месте.** Класс в маршрут не
  входит: при том же маршруте `PATCH` пишет классы сегментов через `flight.DetailsUpdate`
  (`SegmentCabinClasses`), `id` перелётов сохраняются.

## Эндпоинты

Новых маршрутов нет.

| Метод | Путь | Что меняется |
|---|---|---|
| `POST`  | `/api/v1/offers` | у `flights[].segments[]` необязательное `cabin_class` (строка, допускает `null`) |
| `PATCH` | `/api/v1/offers/{uuid}` | то же; правка только классов сохраняет `id` перелётов |
| `GET`   | `/api/v1/offers/{uuid}` | у `flights[].segments[]` всегда `cabin_class` (строка, `""` — неизвестен) |
| `GET`   | `/api/v1/public/offers/{uuid}` | то же |

## Ошибки

| Условие | Ответ |
|---|---|
| Значение не из справочника | `400 validation failed`, `meta` `{"CabinClass":"oneof"}` |
| Не строка (число, объект, массив) | `400 field "…" has the wrong type` |
| `cabin_class` у перелёта, `flightClass` в сегменте | `400` unknown field |

## Хранение

Миграция `020_add_offer_flight_segments_cabin_class`:
`offer_flight_segments.cabin_class VARCHAR(16) NOT NULL DEFAULT ''`; уже сохранённые сегменты
получают `''`. Без `CHECK`/`ENUM` — набор классов доменное правило. Совместима со старым кодом —
можно накатить до выкладки. Down — `DROP COLUMN` (теряются только классы).

Обновление класса на месте — `UpdateOfferFlightSegmentCabinClass` по `flight_id` и `sequence` в
пределах `offer_id` (как `UpdateOfferFlightDetails`): сегменты перелёта чужого оффера не меняются.

## Acceptance criteria

- [ ] Сценарии из дельты `offers/flights` покрыты HTTP-тестами (`tests/application/offer_flight_cabin_class_http_test.go`)
- [ ] Доменный инвариант и обновление на месте покрыты unit-тестами; запись/чтение — интеграционными
- [ ] Swagger: `cabin_class` в запросе и ответе ссылается на схему `flight.CabinClass` с `enum`, построенным из констант домена; в DTO нет `enums`/`oneof` (unit-тест)
- [ ] README, `docs/test_cases/test_cases.md` обновлены; инструкция `docs/cabin-classes.md`

## Non-goals

- Изменения web; эндпоинт `/config` со списком классов; классы `premium_economy`/`first`;
  класс на уровне перелёта целиком; влияние класса на цену, багаж и тарифы; авиакомпания и номер рейса.
