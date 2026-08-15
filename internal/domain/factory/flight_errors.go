package factory

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
