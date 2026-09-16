package offer

import "errors"

// ErrFlightSegmentsEmpty is returned when NewFlight is given no segments.
// A Flight is 1 or more segments — there is no such thing as an empty
// flight.
var ErrFlightSegmentsEmpty = errors.New("flight must have at least one segment")

// ErrFlightSegmentChronologyInvalid is returned when a segment's
// arrival is not strictly after its own departure.
var ErrFlightSegmentChronologyInvalid = errors.New("flight segment arrival must be strictly after departure")

// ErrFlightSegmentDiscontinuous is returned when the arrival airport of
// one segment does not match the departure airport of the next —
// "airport A" and "airport B" of a Flight are derived from the ends of
// a continuous chain, not independent fields.
var ErrFlightSegmentDiscontinuous = errors.New("flight segments must form a continuous route")

// ErrFlightLayoverNonPositive is returned when the next segment departs
// at or before the previous segment arrives — a layover of zero or
// negative duration is not physically possible.
var ErrFlightLayoverNonPositive = errors.New("flight layover between segments must be strictly positive")

// NewFlight builds a Flight from segments already ordered
// first-to-last, checking every structural invariant that does not
// require I/O: non-empty segment list, chronology within each segment,
// route continuity between consecutive segments, and a strictly
// positive layover at every stop. It does not check that the referenced
// airports exist — that requires a database lookup and is the
// responsibility of FlightManager, which calls NewFlight only after
// (or alongside) that check.
func NewFlight(segments []FlightSegment) (Flight, error) {
	if len(segments) == 0 {
		return Flight{}, ErrFlightSegmentsEmpty
	}

	for i, seg := range segments {
		if !seg.ArrivalAt.After(seg.DepartureAt) {
			return Flight{}, ErrFlightSegmentChronologyInvalid
		}
		if i == 0 {
			continue
		}
		prev := segments[i-1]
		if prev.ArrivalAirportICAO != seg.DepartureAirportICAO {
			return Flight{}, ErrFlightSegmentDiscontinuous
		}
		if !seg.DepartureAt.After(prev.ArrivalAt) {
			return Flight{}, ErrFlightLayoverNonPositive
		}
	}

	return Flight{Segments: segments}, nil
}
