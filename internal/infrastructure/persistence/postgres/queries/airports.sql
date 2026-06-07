-- name: SearchAirports :many
WITH q AS (
    SELECT
        a.icao,
        a.iata,
        a.name                  AS airport_name,
        a.location[1]           AS lat,
        a.location[2]           AS lon,
        a.elevation_ft,
        c.id                    AS city_id,
        c.name                  AS city_name,
        c.state                 AS city_state,
        c.timezone              AS city_timezone,
        co.iso2                 AS country_iso2,
        co.name                 AS country_name,
        CASE
            WHEN upper(a.iata) = upper(@search::text)  THEN 1
            WHEN upper(a.icao) = upper(@search::text)  THEN 2
            WHEN lower(unaccent(a.name)) LIKE lower(unaccent(@search_prefix::text))
              OR lower(unaccent(c.name)) LIKE lower(unaccent(@search_prefix::text)) THEN 3
            ELSE 4
        END                     AS rank
    FROM airports a
    JOIN cities    c  ON c.id       = a.city_id
    JOIN countries co ON co.iso2    = c.country_iso2
    WHERE (
        upper(a.iata)                    = upper(@search::text)
        OR upper(a.icao)                 = upper(@search::text)
        OR lower(unaccent(a.name))  LIKE lower(unaccent(@search_like::text))
        OR lower(unaccent(c.name))  LIKE lower(unaccent(@search_like::text))
    )
    AND (@country_filter::text IS NULL OR co.iso2 = upper(@country_filter::text))
)
SELECT *, COUNT(*) OVER() AS total_count
FROM q
ORDER BY rank ASC, airport_name ASC
LIMIT @limit_val::int
OFFSET @offset_val::int;
