package entity

import "api/internal/domain/valueobject"

// Country holds ISO-2 country data.
type Country struct {
	ISO2 string
	Name string
}

// City is the municipality an airport belongs to.
type City struct {
	ID       int
	Name     string
	State    *string
	Timezone string
}

// Airport is the core aggregate for the airport search feature.
// IATA may be nil for small airports without an IATA code.
type Airport struct {
	ICAO     string
	IATA     *string
	Name     string
	Location valueobject.Location
	City     City
	Country  Country
}
