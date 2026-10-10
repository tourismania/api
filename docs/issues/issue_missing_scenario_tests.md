# [test] Недостающие тесты сценариев перелётов и справочника валют

> **GitHub issue:** [№28](https://github.com/tourismania/api/issues/28)
> **Источник требований:** изменение SDD `add-flight-price-currency` (`tourismania/sdd`, `openspec/changes/add-flight-price-currency/`), принято в [sdd PR №12](https://github.com/tourismania/sdd/pull/12) и [sdd PR №13](https://github.com/tourismania/sdd/pull/13); находки `/opsx:verify`.
> **Зависит от:** [№26](https://github.com/tourismania/api/issues/26) (цена и валюта перелёта, v2.1.0)

## Контекст

`/opsx:verify` нашёл сценарии спек без тестов. Поведение реализовано и не меняется — тесты его фиксируют.

## Что покрываем (`tests/application/offer_flight_scenarios_http_test.go`)

| Спека | Сценарий | Проверка |
|---|---|---|
| `offers/flights` | Неверный формат времени | `departure_at` не RFC3339 → `400 invalid date/time value … expected RFC3339`, перелёты не пишутся |
| `offers/flights` | Оффер без перелётов | в карточке `flights` — `[]`, не `null` |
| `offers/flights` | Список офферов | элементы `GET /api/v1/offers` без ключа `flights` |
| `travel-services/currencies` | Список совпадает с правилами записи цены | для каждой валюты из `GET /api/v1/currencies`: цена с `exponent` знаками → `201`, с `exponent + 1` → `400`, `meta: {"Price":"money"}` |

## Non-goals

Изменения кода и контракта.
