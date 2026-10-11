# [fix] Поиск аэропортов: спецсимволы в `search` ищутся буквально

> **GitHub issue:** [№33](https://github.com/tourismania/api/issues/33)
> **Источник требований:** изменение SDD `fix-airport-search-wildcards` (`tourismania/sdd`, `openspec/changes/fix-airport-search-wildcards/`), принято в [sdd PR №20](https://github.com/tourismania/sdd/pull/20).

## Контекст

`GET /api/v1/airports` подставляет `search` в шаблон `LIKE` как есть: `%` и `_` работают как подстановка, `\` экранирует следующий символ. `search=%%` проходит проверку длины и находит все аэропорты справочника, `Al_ha` находит `Alpha`.

- **Экранирование — в репозитории**, `queries/airports.sql` и `db/` не меняются: работает за счёт экранирующего `\` по умолчанию у `LIKE` в PostgreSQL.
- **Коды IATA/ICAO** сравниваются через `=` и передаются без экранирования.

## Acceptance criteria

- Все символы `search`, включая `%`, `_`, `\`, сравниваются с названиями аэропорта и города буквально — и в поиске подстроки, и в ранжировании «название начинается со строки».
- `search=%25%25` → `200`, `meta.search` = `%%`, аэропорты без `%%` в названии не находятся.
- Аэропорт `Test_Field 50%` находится по `search=test_field 50%` в группе «начинается со строки».
- Сравнение IATA/ICAO, нормализация, минимальная длина, ранжирование, пагинация и формат ответа не меняются.

## Тесты

| Файл | Тест | Проверка |
|---|---|---|
| `tests/integration/airport_repository_test.go` | `..._PercentOnly_NoWildcard` | `%%` — ни одного аэропорта сида, `TotalCount` = число буквальных совпадений в БД |
| | `..._Underscore_NoWildcard` | `Al_ha <token>` не находит `Alpha <token> International` |
| | `..._Backslash_Literal` | `Al\pha <token>` не находит Alpha |
| | `..._SpecialCharsInName_PrefixRank` | `test_field <token> 50%` → `Test_Field …` первым, `Aaron Test_Field …` (подстрока) — следом |
| `tests/application/airports_search_http_test.go` | `TestSearchAirports_PercentOnly_NoWildcard` | полный стек на БД: `search=%25%25` → `200`, `meta.search` = `%%`, аэропорт сида не найден |

## Non-goals

- Изменение `queries/airports.sql`, `db/`, swagger, миграций.
- Запрет спецсимволов в `search` / новая ошибка валидации.
- Изменения web.
