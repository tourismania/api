package service

import "errors"

// ErrFlightAirportNotFound is returned by OfferFlightManager when a
// segment references an icao that does not exist in the airports
// reference table. Unlike the structural errors in domain/factory, this
// requires a database lookup, so it is raised here rather than by
// factory.NewFlight.
var ErrFlightAirportNotFound = errors.New("flight segment references an unknown airport icao")
