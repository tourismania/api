package mapper

import (
	"fmt"

	"api/internal/domain/money"
	"api/internal/domain/offer/flight"
	"api/internal/infrastructure/persistence/postgres/db"
)

// ToFlightsDomain собирает строки перелётов и их сегментов в доменные
// сущности. flightRows должны быть упорядочены по возрастанию id, а
// segRows — по (flight_id, sequence); ListOfferFlightsByOfferID и
// ListOfferFlightSegmentsByFlightIDs это гарантируют, поэтому здесь
// ничего не сортируется.
//
// Цена, которую нельзя собрать из сохранённых столбцов (валюты нет в
// справочнике, отрицательная сумма), — это порча данных, а не ошибка
// клиента, поэтому исходная ошибка money встраивается через %v, а не %w:
// apperror не должен превратить её в 400.
func ToFlightsDomain(flightRows []db.OfferFlight, segRows []db.OfferFlightSegment) ([]flight.Flight, error) {
	if len(flightRows) == 0 {
		return nil, nil
	}

	segsByFlight := make(map[int32][]flight.Segment, len(flightRows))
	for _, s := range segRows {
		segsByFlight[s.FlightID] = append(segsByFlight[s.FlightID], flight.Segment{
			DepartureAirportICAO: s.DepartureAirportICAO,
			ArrivalAirportICAO:   s.ArrivalAirportICAO,
			DepartureAt:          s.DepartureAt,
			ArrivalAt:            s.ArrivalAt,
			CabinClass:           flight.CabinClass(s.CabinClass),
		})
	}

	flights := make([]flight.Flight, 0, len(flightRows))
	for _, f := range flightRows {
		price, err := money.FromMinor(f.PriceMinor, f.Currency)
		if err != nil {
			return nil, fmt.Errorf("offer flight %d has an invalid stored price: %v", f.ID, err)
		}
		flights = append(flights, flight.Flight{
			ID:             int(f.ID),
			OfferID:        int(f.OfferID),
			Price:          price,
			ManagerComment: f.ManagerComment,
			Segments:       segsByFlight[f.ID],
		})
	}
	return flights, nil
}
