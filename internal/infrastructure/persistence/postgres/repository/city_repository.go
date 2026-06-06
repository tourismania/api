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

// Upsert inserts a new city or returns the id of the existing matching row.
// Matching is case-insensitive on name and state; exact on country_iso2.
// Requires the unique index cities_uniq_name_state_country (migration 011).
func (r *CityRepository) Upsert(ctx context.Context, name string, state *string, timezone, countryISO2 string) (int, error) {
	const q = `
INSERT INTO cities (name, state, timezone, country_iso2)
VALUES ($1, $2, $3, $4)
ON CONFLICT (lower(name), COALESCE(lower(state), ''), country_iso2)
DO UPDATE SET timezone = EXCLUDED.timezone
RETURNING id`

	var id int
	if err := r.pool.QueryRow(ctx, q, name, state, timezone, countryISO2).Scan(&id); err != nil {
		return 0, fmt.Errorf("upsert city: %w", err)
	}
	return id, nil
}
