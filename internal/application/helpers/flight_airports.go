package helpers

import (
	"context"
	"fmt"

	"api/internal/domain/airport"
	"api/internal/domain/offer/flight"
)

// AirportFinder — порт чтения аэропортов по кодам ICAO: карточке оффера нужны
// данные аэропортов сегментов и их таймзоны.
type AirportFinder interface {
	FindByICAOs(ctx context.Context, icaos []string) ([]airport.Airport, error)
}

// AirportResult — аэропорт сегмента в карточке оффера: IATA nil — кода нет;
// City.Timezone "" — таймзона города не указана.
type AirportResult struct {
	ICAO    string
	IATA    *string
	Name    string
	City    AirportCityResult
	Country AirportCountryResult
}

// AirportCityResult — город аэропорта сегмента.
type AirportCityResult struct {
	Name     string
	Timezone string
}

// AirportCountryResult — страна аэропорта сегмента.
type AirportCountryResult struct {
	ISO2 string
	Name string
}

// FindSegmentAirports одним запросом загружает аэропорты всех сегментов flights
// (без сегментов — без запроса) и возвращает их по ICAO. Код без записи
// невозможен из-за внешнего ключа; если всё же — это порча данных, а не ошибка
// клиента: ошибка с кодом (500), не apperror.
func FindSegmentAirports(ctx context.Context, finder AirportFinder, flights []flight.Flight) (map[string]airport.Airport, error) {
	icaos := flight.AirportICAOs(flights)
	if len(icaos) == 0 {
		return map[string]airport.Airport{}, nil
	}

	found, err := finder.FindByICAOs(ctx, icaos)
	if err != nil {
		return nil, fmt.Errorf("find segment airports: %w", err)
	}
	byICAO := make(map[string]airport.Airport, len(found))
	for _, a := range found {
		byICAO[a.ICAO] = a
	}
	for _, icao := range icaos {
		if _, ok := byICAO[icao]; !ok {
			return nil, fmt.Errorf("flight segment references airport %s that is not in the reference table", icao)
		}
	}
	return byICAO, nil
}

// ToAirportResult — проекция аэропорта сегмента.
func ToAirportResult(a airport.Airport) AirportResult {
	return AirportResult{
		ICAO:    a.ICAO,
		IATA:    a.IATA,
		Name:    a.Name,
		City:    AirportCityResult{Name: a.City.Name, Timezone: a.City.Timezone},
		Country: AirportCountryResult{ISO2: a.Country.ISO2, Name: a.Country.Name},
	}
}
