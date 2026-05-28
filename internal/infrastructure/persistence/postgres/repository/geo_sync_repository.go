package repository

import (
	"context"
	"fmt"

	domainrepo "api/internal/domain/repository"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Compile-time interface check.
var _ domainrepo.GeoSyncRepository = (*GeoSyncRepository)(nil)

// GeoSyncRepository writes countries, cities, and airports to Postgres using
// raw pgx queries (no sqlc generation required for admin sync operations).
type GeoSyncRepository struct {
	pool *pgxpool.Pool
}

// NewGeoSyncRepository constructs a GeoSyncRepository backed by the given pool.
func NewGeoSyncRepository(pool *pgxpool.Pool) *GeoSyncRepository {
	return &GeoSyncRepository{pool: pool}
}

// UpsertCountry inserts or updates a country row.
func (r *GeoSyncRepository) UpsertCountry(ctx context.Context, iso2, name string) error {
	const q = `
INSERT INTO countries (iso2, name)
VALUES ($1, $2)
ON CONFLICT (iso2) DO UPDATE SET name = EXCLUDED.name`

	if _, err := r.pool.Exec(ctx, q, iso2, name); err != nil {
		return fmt.Errorf("upsert country: %w", err)
	}
	return nil
}

// UpsertCity looks up a matching city row; if none exists, inserts one.
// Matching is case-insensitive on name and exact on (state, country_iso2).
// Returns the city's database id.
func (r *GeoSyncRepository) UpsertCity(ctx context.Context, name string, state *string, timezone, countryISO2 string) (int, error) {
	const selectQ = `
SELECT id FROM cities
WHERE lower(name) = lower($1)
  AND country_iso2 = $2
  AND (
    ($3::text IS NULL AND state IS NULL)
    OR lower(state) = lower($3::text)
  )
LIMIT 1`

	var id int
	err := r.pool.QueryRow(ctx, selectQ, name, countryISO2, state).Scan(&id)
	if err == nil {
		return id, nil
	}

	const insertQ = `
INSERT INTO cities (name, state, timezone, country_iso2)
VALUES ($1, $2, $3, $4)
RETURNING id`

	err = r.pool.QueryRow(ctx, insertQ, name, state, timezone, countryISO2).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert city: %w", err)
	}
	return id, nil
}

// UpsertAirport inserts or updates an airport row by ICAO primary key.
func (r *GeoSyncRepository) UpsertAirport(
	ctx context.Context,
	icao string,
	iata *string,
	name string,
	lat, lon float64,
	elevationFt *int,
	cityID int,
) error {
	const q = `
INSERT INTO airports (icao, iata, name, location, elevation_ft, city_id)
VALUES ($1, $2, $3, ARRAY[$4, $5]::float8[], $6, $7)
ON CONFLICT (icao) DO UPDATE SET
    iata         = EXCLUDED.iata,
    name         = EXCLUDED.name,
    location     = EXCLUDED.location,
    elevation_ft = EXCLUDED.elevation_ft,
    city_id      = EXCLUDED.city_id`

	if _, err := r.pool.Exec(ctx, q, icao, iata, name, lon, lat, elevationFt, cityID); err != nil {
		return fmt.Errorf("upsert airport: %w", err)
	}
	return nil
}
