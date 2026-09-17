package mapper

import (
	"api/internal/domain/offer/flight"
	"api/internal/infrastructure/persistence/postgres/db"
)

// ToFlightsDomain groups flight rows with their segments into domain
// entities. flightRows must already be ordered by ascending id and
// segRows by ascending (flight_id, sequence) — both ListOfferFlightsByOfferID
// and ListOfferFlightSegmentsByFlightIDs guarantee this ordering, so no
// sort happens here.
func ToFlightsDomain(flightRows []db.OfferFlight, segRows []db.OfferFlightSegment) []flight.Flight {
	if len(flightRows) == 0 {
		return nil
	}

	segsByFlight := make(map[int32][]flight.Segment, len(flightRows))
	for _, s := range segRows {
		segsByFlight[s.FlightID] = append(segsByFlight[s.FlightID], flight.Segment{
			DepartureAirportICAO: s.DepartureAirportICAO,
			ArrivalAirportICAO:   s.ArrivalAirportICAO,
			DepartureAt:          s.DepartureAt,
			ArrivalAt:            s.ArrivalAt,
		})
	}

	flights := make([]flight.Flight, 0, len(flightRows))
	for _, f := range flightRows {
		flights = append(flights, flight.Flight{
			ID:       int(f.ID),
			OfferID:  int(f.OfferID),
			Segments: segsByFlight[f.ID],
		})
	}
	return flights
}
