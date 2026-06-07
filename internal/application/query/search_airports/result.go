package searchairports

// LocationResult is the coordinate projection returned by the use-case.
type LocationResult struct {
	Latitude    float64
	Longitude   float64
	ElevationFt *int
}

// CityResult is the city projection returned by the use-case.
type CityResult struct {
	ID       int
	Name     string
	State    *string
	Timezone string
}

// CountryResult is the country projection returned by the use-case.
type CountryResult struct {
	ISO2 string
	Name string
}

// AirportResult is a single airport in the use-case response.
type AirportResult struct {
	ICAO     string
	IATA     *string
	Name     string
	Location LocationResult
	City     CityResult
	Country  CountryResult
}

// Result is what the Handler returns to the presentation layer.
type Result struct {
	Airports   []AirportResult
	TotalCount int64
}
