package airport

import "context"

// Filter carries the normalised search parameters.
type Filter struct {
	Search string // trimmed and space-collapsed
	Limit  int
	Offset int
}

// SearchResult carries a page of airports plus the total count.
type SearchResult struct {
	Airports   []Airport
	TotalCount int64
}

// Repository is the read-port for airport search.
// The concrete implementation lives in infrastructure/persistence.
type Repository interface {
	Search(ctx context.Context, f Filter) (SearchResult, error)
	Upsert(ctx context.Context, icao string, iata *string, name string, lat, lon float64, elevationFt *int, cityID int) error
	// FindByICAOs returns the airports matching any of the given icaos.
	// Unknown icaos are simply absent from the result — callers compare
	// len(result) against len(icaos) (or index by ICAO) to detect them.
	FindByICAOs(ctx context.Context, icaos []string) ([]Airport, error)
}
