package db

import (
	"context"
)

// searchAirportsSQL is the hand-maintained positional-param version of
// queries/airports.sql (named params @x → positional $x).
// Parameter order: $1=search, $2=search_prefix, $3=search_like,
// $4=limit, $5=offset.
const searchAirportsSQL = `WITH q AS (
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
            WHEN upper(a.iata) = upper($1)  THEN 1
            WHEN upper(a.icao) = upper($1)  THEN 2
            WHEN lower(unaccent(a.name)) LIKE lower(unaccent($2))
              OR lower(unaccent(c.name)) LIKE lower(unaccent($2)) THEN 3
            ELSE 4
        END                     AS rank
    FROM airports a
    JOIN cities    c  ON c.id       = a.city_id
    JOIN countries co ON co.iso2    = c.country_iso2
    WHERE (
        upper(a.iata)                    = upper($1)
        OR upper(a.icao)                 = upper($1)
        OR lower(unaccent(a.name))  LIKE lower(unaccent($3))
        OR lower(unaccent(c.name))  LIKE lower(unaccent($3))
    )
)
SELECT icao, iata, airport_name, lat, lon, elevation_ft,
       city_id, city_name, city_state, city_timezone,
       country_iso2, country_name, rank,
       COUNT(*) OVER() AS total_count
FROM q
ORDER BY rank ASC, airport_name ASC
LIMIT $4
OFFSET $5`

// SearchAirportsParams carries the bound parameters for SearchAirports.
type SearchAirportsParams struct {
	Search       string
	SearchPrefix string
	SearchLike   string
	LimitVal     int32
	OffsetVal    int32
}

// SearchAirportsRow is one row returned by SearchAirports.
type SearchAirportsRow struct {
	Icao         string
	Iata         *string
	AirportName  string
	Lon          *float64
	Lat          *float64
	ElevationFt  *int32
	CityID       int32
	CityName     string
	CityState    *string
	CityTimezone *string
	CountryIso2  string
	CountryName  string
	Rank         int32
	TotalCount   int64
}

// SearchAirports executes the airport full-text search query.
func (q *Queries) SearchAirports(ctx context.Context, arg SearchAirportsParams) ([]SearchAirportsRow, error) {
	rows, err := q.db.Query(ctx, searchAirportsSQL,
		arg.Search,
		arg.SearchPrefix,
		arg.SearchLike,
		arg.LimitVal,
		arg.OffsetVal,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []SearchAirportsRow
	for rows.Next() {
		var row SearchAirportsRow
		if err := rows.Scan(
			&row.Icao,
			&row.Iata,
			&row.AirportName,
			&row.Lat,
			&row.Lon,
			&row.ElevationFt,
			&row.CityID,
			&row.CityName,
			&row.CityState,
			&row.CityTimezone,
			&row.CountryIso2,
			&row.CountryName,
			&row.Rank,
			&row.TotalCount,
		); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
