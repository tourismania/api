package repository

import (
	"context"

	"api/internal/domain/entity"
)

// AirportFilter carries the normalised search parameters.
type AirportFilter struct {
	Search  string  // trimmed and space-collapsed
	Country *string // nil = no country filter; ISO-2 code otherwise
	Limit   int
	Offset  int
}

// AirportSearchResult carries a page of airports plus the total count.
type AirportSearchResult struct {
	Airports   []entity.Airport
	TotalCount int64
}

// AirportRepository is the read-port for airport search.
// The concrete implementation lives in infrastructure/persistence.
type AirportRepository interface {
	Search(ctx context.Context, f AirportFilter) (AirportSearchResult, error)
}
