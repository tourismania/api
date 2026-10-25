package helpers

import (
	"context"
	"fmt"

	"api/internal/domain/airline"
	"api/internal/domain/offer/flight"
)

// AirlineFinder — порт чтения авиакомпаний по id (включая дубли и пропавшие):
// карточке оффера нужны данные авиакомпаний сегментов.
type AirlineFinder interface {
	FindByIDs(ctx context.Context, ids []int) ([]airline.Airline, error)
}

// FindSegmentAirlines одним запросом загружает авиакомпании всех сегментов
// flights (без перелётов — без запроса) и возвращает их по id. Id без записи
// невозможен из-за внешнего ключа; если всё же — это порча данных, а не ошибка
// клиента: ошибка с id (500), не apperror.
func FindSegmentAirlines(ctx context.Context, finder AirlineFinder, flights []flight.Flight) (map[int]airline.Airline, error) {
	ids := flight.AirlineIDs(flights)
	if len(ids) == 0 {
		return map[int]airline.Airline{}, nil
	}

	found, err := finder.FindByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("find segment airlines: %w", err)
	}
	byID := make(map[int]airline.Airline, len(found))
	for _, a := range found {
		byID[a.ID] = a
	}
	for _, id := range ids {
		if _, ok := byID[id]; !ok {
			return nil, fmt.Errorf("flight segment references airline %d that is not in the reference table", id)
		}
	}
	return byID, nil
}
