package repository

import (
	"api/internal/infrastructure/persistence/postgres/mapper"
	"context"
	"errors"
	"fmt"

	"api/internal/domain/entity"
	domainrepo "api/internal/domain/repository"
	"api/internal/infrastructure/persistence/postgres/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Compile-time interface check.
var _ domainrepo.AirportRepository = (*AirportRepository)(nil)

// AirportRepository persists domain airport aggregates via pgx/sqlc.
type AirportRepository struct {
	queries *db.Queries
	pool    *pgxpool.Pool
}

// NewAirportRepository wires the queries layer.
func NewAirportRepository(queries *db.Queries, pool *pgxpool.Pool) *AirportRepository {
	return &AirportRepository{
		queries: queries,
		pool:    pool,
	}
}

// Search executes the airport full-text search and maps results to domain entities.
func (r *AirportRepository) Search(ctx context.Context, f domainrepo.AirportFilter) (domainrepo.AirportSearchResult, error) {
	searchLike := "%" + f.Search + "%"
	searchPrefix := f.Search + "%"

	rows, err := r.queries.SearchAirports(ctx, db.SearchAirportsParams{
		Search:       f.Search,
		SearchPrefix: searchPrefix,
		SearchLike:   searchLike,
		LimitVal:     int32(f.Limit),
		OffsetVal:    int32(f.Offset),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainrepo.AirportSearchResult{}, nil
		}
		return domainrepo.AirportSearchResult{}, fmt.Errorf("search airports: %w", err)
	}

	airports := make([]entity.Airport, 0, len(rows))
	var total int64
	for _, row := range rows {
		total = row.TotalCount
		airports = append(airports, mapper.ToAirportDomain(row))
	}

	return domainrepo.AirportSearchResult{Airports: airports, TotalCount: total}, nil
}

// Upsert inserts or updates an airport row by ICAO primary key.
func (r *AirportRepository) Upsert(
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
