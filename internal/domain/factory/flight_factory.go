package factory

import "api/internal/domain/entity"

// NewFlight builds an entity.Flight from segments already ordered
// first-to-last, checking every structural invariant that does not
// require I/O: non-empty segment list, chronology within each segment,
// route continuity between consecutive segments, and a strictly
// positive layover at every stop. It does not check that the referenced
// airports exist — that requires a database lookup and is the
// responsibility of service.OfferFlightManager, which calls NewFlight
// only after (or alongside) that check.
func NewFlight(segments []entity.FlightSegment) (entity.Flight, error) {
	if len(segments) == 0 {
		return entity.Flight{}, ErrFlightSegmentsEmpty
	}

	for i, seg := range segments {
		if !seg.ArrivalAt.After(seg.DepartureAt) {
			return entity.Flight{}, ErrFlightSegmentChronologyInvalid
		}
		if i == 0 {
			continue
		}
		prev := segments[i-1]
		if prev.ArrivalAirportICAO != seg.DepartureAirportICAO {
			return entity.Flight{}, ErrFlightSegmentDiscontinuous
		}
		if !seg.DepartureAt.After(prev.ArrivalAt) {
			return entity.Flight{}, ErrFlightLayoverNonPositive
		}
	}

	return entity.Flight{Segments: segments}, nil
}
