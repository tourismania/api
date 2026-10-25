package getairline

import (
	"context"
	"fmt"

	"api/internal/application/apperror"
	"api/internal/domain/airline"
)

// UseCase — порт, от которого зависит presentation-слой.
type UseCase interface {
	Handle(ctx context.Context, q Query) (Result, error)
}

// AirlineFinder — порт чтения авиакомпаний по id (включая дубли и пропавшие).
type AirlineFinder interface {
	FindByIDs(ctx context.Context, ids []int) ([]airline.Airline, error)
}

// Handler отдаёт одну авиакомпанию по id.
type Handler struct {
	airlines AirlineFinder
}

// NewHandler собирает обработчик.
func NewHandler(airlines AirlineFinder) *Handler {
	return &Handler{airlines: airlines}
}

// Handle возвращает авиакомпанию q.ID; нет записи — apperror.ErrNotFound.
func (h *Handler) Handle(ctx context.Context, q Query) (Result, error) {
	found, err := h.airlines.FindByIDs(ctx, []int{q.ID})
	if err != nil {
		return Result{}, fmt.Errorf("find airline %d: %w", q.ID, err)
	}
	if len(found) == 0 {
		return Result{}, fmt.Errorf("%w: airline %d", apperror.ErrNotFound, q.ID)
	}

	a := found[0]
	r := Result{
		ID:          a.ID,
		ICAO:        a.ICAO,
		Name:        a.Name,
		NameRU:      a.NameRU,
		DissolvedOn: a.DissolvedOn,
	}
	if a.Country != nil {
		r.Country = &CountryResult{ISO2: a.Country.ISO2, Name: a.Country.Name}
	}
	return r, nil
}

// Compile-time check.
var _ UseCase = (*Handler)(nil)
