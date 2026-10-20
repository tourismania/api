# [feat] Справочник авиакомпаний и авиакомпания сегмента перелёта (Flight) в составе Offer

> **Зависит от:** sdd `add-flight-segment-class` (обновление сегментов на месте `DetailsUpdate`, api PR №35)
> **Источник требований:** изменение SDD `add-flight-segment-airline` в `tourismania/sdd`
> (`openspec/changes/add-flight-segment-airline/`: proposal, дельты спеков `travel-services/airlines` и
> `offers/flights`, design, tasks), принято в [sdd PR №27](https://github.com/tourismania/sdd/pull/27).
> **Реализация:** [api PR №36](https://github.com/tourismania/api/pull/36).
> Этот файл — краткая выжимка для api; при расхождении прав SDD.

## Контекст

Авиакомпания — то, по чему клиент узнаёт перелёт, а api её не хранил: в редакторе web она — свободный
текст и теряется при сохранении. Свободный текст не подходит (одна авиакомпания пишется по-разному),
поэтому нужен справочник авиакомпаний: наполняется из Wikidata командой CLI, менеджер выбирает
авиакомпанию поиском, сегмент ссылается на неё по id.

**Инварианты:**

- **Ключ авиакомпании — id записи справочника**, коды ICAO и IATA — атрибуты без уникальности: код ICAO
  переназначается после закрытия авиакомпании (~13% кодов Wikidata встречаются у нескольких
  авиакомпаний). Сегмент ссылается на запись, выбранную в поиске, и ссылка дальше не вычисляется.
- **Авиакомпания сегмента обязательна.** Перевозчик неизвестен — явно `1` (служебная запись
  «Неизвестная», `Unknown`); api ничего не подставляет. `0`/`null`/нет поля → `400 required`.
- **`name_ru` — только у российских**, у остальных `""`; IATA в ответах не отдаются.
- **Дубли и пропавшие из Wikidata скрываются из поиска**, но остаются для сохранённых сегментов;
  синхронизация сегменты не меняет. Скрытый id в записи оффера допустим, только если оффер уже
  ссылался на него (сравнение по всему офферу).
- **Авиакомпания в маршрут не входит**: её изменение (вместе с классом/ценой/комментарием или без них)
  обновляет сегмент на месте, `id` перелётов сохраняются.

## Эндпоинты

| Метод | Путь | Что меняется |
|---|---|---|
| `GET`   | `/api/v1/airlines` | **новый**: поиск по названиям и стране, `country`, `ids`, `limit`/`offset`; только выбираемые записи |
| `GET`   | `/api/v1/airlines/{id}` | **новый**: запись по id, в т. ч. скрытая (`hidden`) |
| `POST`  | `/api/v1/offers` | у `flights[].segments[]` **обязательный** `airline_id` (integer) |
| `PATCH` | `/api/v1/offers/{uuid}` | то же; правка только авиакомпаний сохраняет `id` перелётов |
| `GET`   | `/api/v1/offers/{uuid}` | у сегментов `airline_id` и `airline{id, icao, name, name_ru, dissolved_on}` |
| `GET`   | `/api/v1/public/offers/{uuid}` | то же |

CLI: `airlines sync [--dry-run]` — [docs/cli/sync_airlines.md](../cli/sync_airlines.md) (журнал аномалий —
раздел «Мониторинг»).

## Ошибки

| Условие | Ответ |
|---|---|
| `airline_id` нет / `null` / `0` | `400 validation failed`, `meta` `{"AirlineID":"required"}` |
| Не целое число (`"AFL"`, `"42"`, `1.5`) | `400 field "…" has the wrong type` |
| id нет в справочнике (в т. ч. отрицательный) или скрытый, на который оффер не ссылался | `400 validation failed: flight segment references an unknown airline` |
| `airline_id` у перелёта, `airline`/`airline_icao` в сегменте | `400` unknown field |
| Поиск: `search` короче 3 символов | `400 INVALID_SEARCH` |
| Поиск: `country` не ISO-2 / `ids` не целые ≥ 1 / > 100 / `limit` > 100 / `offset` > 10000 | `400 validation failed`, `meta` `{"Country":"iso2"}` / `{"IDs":"min=1"}` / `{"IDs":"max=100"}` / `{"Limit":"max=100"}` / `{"Offset":"max=10000"}` |
| `GET /airlines/{id}`: нет записи или id не целое ≥ 1 | `404 airline not found` |

## Хранение

- Миграция `021_create_airlines`: таблица `airlines` (`id serial`, `source_id` без `UNIQUE`, `icao`,
  `name`, `name_ru NOT NULL DEFAULT ''`, `country_iso2 → countries`, `dissolved_on`, `iata_codes char(2)[]`,
  `merged_into_id → airlines`, `missing_since`), запись `(1, 'Unknown', 'Неизвестная')`, `setval` на `1`.
- Миграция `022_add_offer_flight_segments_airline_id`: `airline_id int NOT NULL DEFAULT 1 REFERENCES
  airlines(id)` — уже сохранённые сегменты получают «Неизвестная»; `DEFAULT` — backfill и
  совместимость при выкладке, не умолчание контракта.
- Выкладка: миграции → api → `airlines sync --dry-run` → `airlines sync`.
