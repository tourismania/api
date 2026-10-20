-- name: SearchAirlines :many
-- Поиск выбираемых авиакомпаний: дубли (merged_into_id) и пропавшие из
-- источника (missing_since) не возвращаются никогда, в т. ч. по ids.
-- search_like / search_prefix — уже экранированные (persistence/helpers.EscapeLike)
-- шаблоны '%s%' и 's%'; пустая строка поиска даёт '%%' — без отбора по
-- названию (только вместе с ids). rank: 0 — название начинается со строки,
-- 1 — подстрока названия, 2 — совпало только название страны.
WITH q AS (
    SELECT
        a.id,
        a.icao,
        a.name,
        a.name_ru,
        a.dissolved_on,
        co.iso2 AS country_iso2,
        co.name AS country_name,
        CASE
            WHEN lower(unaccent(a.name))    LIKE lower(unaccent(@search_prefix::text))
              OR lower(unaccent(a.name_ru)) LIKE lower(unaccent(@search_prefix::text)) THEN 0
            WHEN lower(unaccent(a.name))    LIKE lower(unaccent(@search_like::text))
              OR lower(unaccent(a.name_ru)) LIKE lower(unaccent(@search_like::text)) THEN 1
            ELSE 2
        END AS rank
    FROM airlines a
    LEFT JOIN countries co ON co.iso2 = a.country_iso2
    WHERE a.merged_into_id IS NULL
      AND a.missing_since IS NULL
      AND (
          lower(unaccent(a.name))    LIKE lower(unaccent(@search_like::text))
          OR lower(unaccent(a.name_ru)) LIKE lower(unaccent(@search_like::text))
          OR lower(unaccent(co.name))   LIKE lower(unaccent(@search_like::text))
      )
      AND (@country::text = '' OR a.country_iso2 = @country::text)
      AND (cardinality(@ids::int[]) = 0 OR a.id = ANY(@ids::int[]))
)
SELECT id, icao, name, name_ru, dissolved_on, country_iso2, country_name, rank,
       COUNT(*) OVER () AS total_count
FROM q
ORDER BY (dissolved_on IS NOT NULL), rank, name, id
LIMIT @limit_val::int
OFFSET @offset_val::int;

-- name: ListAirlinesByIDs :many
-- Все записи с перечисленными id, включая скрытые (hidden).
SELECT a.id, a.icao, a.name, a.name_ru, a.dissolved_on, a.iata_codes,
       co.iso2 AS country_iso2, co.name AS country_name,
       (a.merged_into_id IS NOT NULL OR a.missing_since IS NOT NULL) AS hidden
FROM airlines a
LEFT JOIN countries co ON co.iso2 = a.country_iso2
WHERE a.id = ANY(@ids::int[]);

-- name: InsertAirline :one
INSERT INTO airlines (source_id, icao, name, name_ru, country_iso2, dissolved_on, iata_codes)
VALUES (@source_id, @icao, @name, @name_ru, @country_iso2, @dissolved_on, @iata_codes)
RETURNING id;

-- name: UpdateAirline :exec
-- Только записи из источника: служебную запись синхронизация не меняет.
UPDATE airlines
SET icao = @icao, name = @name, name_ru = @name_ru, country_iso2 = @country_iso2,
    dissolved_on = @dissolved_on, iata_codes = @iata_codes
WHERE id = @id AND source_id IS NOT NULL;

-- name: ListAirlineSourceIDs :many
SELECT id, source_id, merged_into_id IS NOT NULL AS merged, missing_since IS NOT NULL AS missing
FROM airlines
WHERE source_id IS NOT NULL
ORDER BY id;

-- name: MergeAirline :exec
-- from_id становится дублем to_id; записи, уже слитые в from_id, переводятся
-- на to_id, чтобы цепочка не росла. Сегменты офферов не трогаются.
UPDATE airlines
SET merged_into_id = @to_id
WHERE (id = @from_id OR merged_into_id = @from_id)
  AND id <> @to_id
  AND source_id IS NOT NULL;

-- name: MarkAirlinesMissing :exec
UPDATE airlines
SET missing_since = @since
WHERE id = ANY(@ids::int[]) AND missing_since IS NULL AND source_id IS NOT NULL;

-- name: ClearAirlinesMissing :exec
UPDATE airlines
SET missing_since = NULL
WHERE id = ANY(@ids::int[]) AND missing_since IS NOT NULL;
