// Package airport is the airport reference aggregate: the Airport
// entity with its City/Country reference data, geographic Location and
// the read/write repository contracts used by airport search and the
// sync-airports command.
package airport

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
	Location Location
	City     City
	Country  Country
}
