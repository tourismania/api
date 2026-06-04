package repository

import (
	"context"
	"fmt"

	domainrepo "api/internal/domain/repository"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Compile-time interface check.
var _ domainrepo.CountryRepository = (*CountryRepository)(nil)

// CountryRepository writes countries, cities, and airports to Postgres using
// raw pgx queries (no sqlc generation required for admin sync operations).
type CountryRepository struct {
	pool *pgxpool.Pool
}

// NewCountryRepository CountryRepository constructs a CountryRepository backed by the given pool.
func NewCountryRepository(pool *pgxpool.Pool) *CountryRepository {
	return &CountryRepository{pool: pool}
}

// Upsert inserts or updates a country row.
func (r *CountryRepository) Upsert(ctx context.Context, iso2, name string) error {
	const q = `
INSERT INTO countries (iso2, name)
VALUES ($1, $2)
ON CONFLICT (iso2) DO UPDATE SET name = EXCLUDED.name`

	if _, err := r.pool.Exec(ctx, q, iso2, name); err != nil {
		return fmt.Errorf("upsert country: %w", err)
	}
	return nil
}
