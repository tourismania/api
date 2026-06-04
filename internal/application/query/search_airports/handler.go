package searchairports

import (
	"context"
	"fmt"

	"api/internal/domain/repository"
)

// UseCase is the port the presentation layer depends on.
type UseCase interface {
	Handle(ctx context.Context, q Query) (Result, error)
}

// AirportSearcher is the read-port consumed by this use-case.
// Defined here to invert the dependency: infrastructure implements this.
type AirportSearcher interface {
	Search(ctx context.Context, f repository.AirportFilter) (repository.AirportSearchResult, error)
}

// Handler orchestrates the airport search use-case.
type Handler struct {
	airports AirportSearcher
}

// NewHandler constructs the use-case handler.
func NewHandler(airports AirportSearcher) *Handler {
	return &Handler{airports: airports}
}

// Handle satisfies UseCase.
func (h *Handler) Handle(ctx context.Context, q Query) (Result, error) {
	res, err := h.airports.Search(ctx, repository.AirportFilter{
		Search: q.Search,
		Limit:  q.Limit,
		Offset: q.Offset,
	})
	if err != nil {
		return Result{}, fmt.Errorf("search airports: %w", err)
	}

	out := make([]AirportResult, 0, len(res.Airports))
	for _, a := range res.Airports {
		out = append(out, AirportResult{
			ICAO: a.ICAO,
			IATA: a.IATA,
			Name: a.Name,
			Location: LocationResult{
				Latitude:    a.Location.Latitude,
				Longitude:   a.Location.Longitude,
				ElevationFt: a.Location.ElevationFt,
			},
			City: CityResult{
				ID:       a.City.ID,
				Name:     a.City.Name,
				State:    a.City.State,
				Timezone: a.City.Timezone,
			},
			Country: CountryResult{
				ISO2: a.Country.ISO2,
				Name: a.Country.Name,
			},
		})
	}

	return Result{Airports: out, TotalCount: res.TotalCount}, nil
}
