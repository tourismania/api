# [feat] Цена и валюта перелёта (Flight) в составе Offer

> **GitHub issue:** [№26](https://github.com/tourismania/api/issues/26)
> **Зависит от:** [№20](https://github.com/tourismania/api/issues/20) (перелёты в составе Offer)
> **Источник требований:** изменение SDD `add-flight-price-currency` в `tourismania/sdd`
> (`openspec/changes/add-flight-price-currency/`: proposal, дельта спеки `offers/flights`, design, tasks),
> принято в [sdd PR №12](https://github.com/tourismania/sdd/pull/12).
> **Реализация:** [api PR №27](https://github.com/tourismania/api/pull/27).
> Этот файл — краткая выжимка для api; при расхождении прав SDD.

## Контекст

У перелёта оффера в api не было цены: цену и валюту знал только редактор web, и при сохранении они
терялись. Без цены в api web не может перейти на перелёты из api, а итог оффера нельзя посчитать по
данным api.

**Инварианты:**

- **Цена — у перелёта целиком, не у сегмента.** `price` и `currency` обязательны для каждого перелёта.
- **Валюта — из системного справочника в коде** (`internal/domain/money`): код ISO 4217 + число знаков
  после точки (`Exponent`). Сейчас `RUB`, `USD`, `EUR`, `TRY` (по 2 знака). Справочника в БД и `CHECK` нет.
- **Сумма — без float.** Хранится целым числом минорных единиц (`price_minor BIGINT`), на проводе —
  десятичной строкой; знаков после точки не больше `Exponent` валюты, в ответе — ровно `Exponent`.
- **Изменилась только цена — обновляется только цена.** Если в `PATCH` маршрут и время перелётов совпали
  с сохранёнными, перелёты не пересоздаются, `id` сохраняются. Сопоставление — по позиции в массиве.

## Эндпоинты

Один новый маршрут — список валют (добавлен по ревью sdd PR №13); остальное — в существующих маршрутах оффера.

| Метод | Путь | Что меняется |
|---|---|---|
| `POST` | `/api/v1/offers` | `flights[]` — обязательные `price` (строка), `currency` |
| `PATCH` | `/api/v1/offers/{uuid}` | то же; обновление только цены без смены `id` |
| `GET` | `/api/v1/offers/{uuid}` | у перелёта `price`, `currency` |
| `GET` | `/api/v1/public/offers/{uuid}` | у перелёта `price`, `currency` |
| `GET` | `/api/v1/currencies` | **новый** (ревью sdd PR №13): список валют справочника `{"data":[{"code","exponent"}]}`, JWT, `Cache-Control: private, max-age=3600` |

**BREAKING:** клиенты, пишущие `flights` напрямую, должны передавать `price` и `currency`.

## Модель данных

Миграция `018_add_offer_flights_price`: `offer_flights.price_minor BIGINT NOT NULL`,
`offer_flights.currency CHAR(3) NOT NULL`. Перелёты, сохранённые без цены, удаляются (цену взять неоткуда;
заполнять нулём — выдумывать данные). Down удаляет столбцы, удалённые перелёты не возвращает.

## Ошибки

Обязательность `price`/`currency` проверяет валидатор presentation-слоя (форма запроса). Валюту по справочнику
и формат суммы проверяет application-слой (`flightinput.Build`) и возвращает `apperror.FieldError` — presentation
отдаёт его в `meta` так же, как ошибки валидатора (ревью PR №27: бизнес-правила не в валидаторе и не в `config/`).

| Случай | Ответ |
|---|---|
| нет `price`/`currency` | `400 validation failed`, `meta` `Price`/`Currency` = `required` |
| валюта не из справочника (`GBP`, `rub`) | `400 validation failed`, `meta: {"Currency":"currency"}` |
| лишние знаки после точки, знак, экспонента, запятая, >10 цифр | `400 validation failed`, `meta: {"Price":"money"}` |
| `price` числом JSON | `400 field "…" has the wrong type` |
| неизвестная валюта в БД (порча данных) | `500` |

## Документация

- `docs/currencies.md` — как добавить валюту, изменить число знаков после точки, удалить валюту.
- `FAQ.md` №2 — почему без float, почему минорные единицы, почему справочник в коде и без `CHECK`.

## Non-goals

Подключение web к перелётам api; авиакомпания, номер рейса, класс, комментарий; расширение списка валют;
переход web на список валют из api.
