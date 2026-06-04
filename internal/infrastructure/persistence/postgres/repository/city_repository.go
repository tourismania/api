package repository

import (
	"context"
	"fmt"

	domainrepo "api/internal/domain/repository"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Compile-time interface check.
var _ domainrepo.CityRepository = (*CityRepository)(nil)

// CityRepository writes countries, cities, and airports to Postgres using
// raw pgx queries (no sqlc generation required for admin sync operations).
type CityRepository struct {
	pool *pgxpool.Pool
}

// NewCityRepository constructs a GeoSyncRepository backed by the given pool.
func NewCityRepository(pool *pgxpool.Pool) *CityRepository {
	return &CityRepository{pool: pool}
}

// Upsert looks up a matching city row; if none exists, inserts one.
// Matching is case-insensitive on name and exact on (state, country_iso2).
// Returns the city's database id.
func (r *CityRepository) Upsert(ctx context.Context, name string, state *string, timezone, countryISO2 string) (int, error) {
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
