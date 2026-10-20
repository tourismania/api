package searchairlines

import (
	"context"
	"fmt"

	"api/internal/domain/airline"
)

// UseCase — порт, от которого зависит presentation-слой.
type UseCase interface {
	Handle(ctx context.Context, q Query) (Result, error)
}

// AirlineSearcher — порт поиска; реализует infrastructure.
type AirlineSearcher interface {
	Search(ctx context.Context, f airline.Filter) (airline.SearchResult, error)
}

// Handler выполняет поиск авиакомпаний.
type Handler struct {
	airlines AirlineSearcher
}

// NewHandler собирает обработчик.
func NewHandler(airlines AirlineSearcher) *Handler {
	return &Handler{airlines: airlines}
}

// Handle возвращает выбираемые авиакомпании (без дублей и пропавших).
func (h *Handler) Handle(ctx context.Context, q Query) (Result, error) {
	res, err := h.airlines.Search(ctx, airline.Filter{
		Search:      q.Search,
		CountryISO2: q.CountryISO2,
		IDs:         q.IDs,
		Limit:       q.Limit,
		Offset:      q.Offset,
	})
	if err != nil {
		return Result{}, fmt.Errorf("search airlines: %w", err)
	}

	out := make([]AirlineResult, 0, len(res.Airlines))
	for _, a := range res.Airlines {
		r := AirlineResult{
			ID:          a.ID,
			ICAO:        a.ICAO,
			Name:        a.Name,
			NameRU:      a.NameRU,
			DissolvedOn: a.DissolvedOn,
		}
		if a.Country != nil {
			r.Country = &CountryResult{ISO2: a.Country.ISO2, Name: a.Country.Name}
		}
		out = append(out, r)
	}
	return Result{Airlines: out, TotalCount: res.TotalCount}, nil
}

// Compile-time check.
var _ UseCase = (*Handler)(nil)
