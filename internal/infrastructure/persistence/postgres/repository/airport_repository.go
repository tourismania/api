package repository

import (
	"context"
	"errors"
	"fmt"

	"api/internal/domain/entity"
	domainrepo "api/internal/domain/repository"
	"api/internal/domain/valueobject"
	"api/internal/infrastructure/persistence/postgres/db"

	"github.com/jackc/pgx/v5"
)

// Compile-time interface check.
var _ domainrepo.AirportRepository = (*AirportRepository)(nil)

// AirportRepository persists domain airport aggregates via pgx/sqlc.
type AirportRepository struct {
	queries *db.Queries
}

// NewAirportRepository wires the queries layer.
func NewAirportRepository(queries *db.Queries) *AirportRepository {
	return &AirportRepository{queries: queries}
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
		airports = append(airports, mapRowToAirport(row))
	}

	return domainrepo.AirportSearchResult{Airports: airports, TotalCount: total}, nil
}

func mapRowToAirport(row db.SearchAirportsRow) entity.Airport {
	elevFt := ptrInt32ToInt(row.ElevationFt)

	var lon, lat float64
	if row.Lon != nil {
		lon = *row.Lon
	}
	if row.Lat != nil {
		lat = *row.Lat
	}

	timezone := ""
	if row.CityTimezone != nil {
		timezone = *row.CityTimezone
	}

	return entity.Airport{
		ICAO: row.Icao,
		IATA: row.Iata,
		Name: row.AirportName,
		Location: valueobject.Location{
			Latitude:    lat,
			Longitude:   lon,
			ElevationFt: elevFt,
		},
		City: entity.City{
			ID:       int(row.CityID),
			Name:     row.CityName,
			State:    row.CityState,
			Timezone: timezone,
		},
		Country: entity.Country{
			ISO2: row.CountryIso2,
			Name: row.CountryName,
		},
	}
}

func ptrInt32ToInt(v *int32) *int {
	if v == nil {
		return nil
	}
	i := int(*v)
	return &i
}
