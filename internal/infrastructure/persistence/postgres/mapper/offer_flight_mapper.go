package mapper

import (
	"api/internal/domain/offer"
	"api/internal/infrastructure/persistence/postgres/db"
)

// ToFlightsDomain groups flight rows with their segments into domain
// entities. flightRows must already be ordered by ascending id and
// segRows by ascending (flight_id, sequence) — both ListOfferFlightsByOfferID
// and ListOfferFlightSegmentsByFlightIDs guarantee this ordering, so no
// sort happens here.
func ToFlightsDomain(flightRows []db.OfferFlight, segRows []db.OfferFlightSegment) []offer.Flight {
	if len(flightRows) == 0 {
		return nil
	}

	segsByFlight := make(map[int32][]offer.FlightSegment, len(flightRows))
	for _, s := range segRows {
		segsByFlight[s.FlightID] = append(segsByFlight[s.FlightID], offer.FlightSegment{
			DepartureAirportICAO: s.DepartureAirportICAO,
			ArrivalAirportICAO:   s.ArrivalAirportICAO,
			DepartureAt:          s.DepartureAt,
			ArrivalAt:            s.ArrivalAt,
		})
	}

	flights := make([]offer.Flight, 0, len(flightRows))
	for _, f := range flightRows {
		flights = append(flights, offer.Flight{
			ID:       int(f.ID),
			OfferID:  int(f.OfferID),
			Segments: segsByFlight[f.ID],
		})
	}
	return flights
}
