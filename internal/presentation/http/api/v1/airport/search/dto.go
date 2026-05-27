package searchairporthttp

// SearchParams contains the query parameters for the airport search endpoint.
// Validation uses go-playground/validator; business rules are applied in the handler.
type SearchParams struct {
	Search  string `schema:"search"  validate:"required"`
	Limit   int    `schema:"limit"   validate:"omitempty,min=1,max=100"`
	Offset  int    `schema:"offset"  validate:"omitempty,min=0,max=10000"`
	Country string `schema:"country" validate:"omitempty,len=2"`
	Lang    string `schema:"lang"    validate:"omitempty"`
}

// LocationResponse is the location projection in the JSON response.
type LocationResponse struct {
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	ElevationFt *int    `json:"elevation_ft"`
}

// CityResponse is the city projection in the JSON response.
type CityResponse struct {
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	State    *string `json:"state"`
	Timezone string  `json:"timezone"`
}

// CountryResponse is the country projection in the JSON response.
type CountryResponse struct {
	ISO2 string `json:"iso2"`
	Name string `json:"name"`
}

// AirportResponse is a single airport item in the JSON response.
type AirportResponse struct {
	ICAO     string           `json:"icao"`
	IATA     *string          `json:"iata"`
	Name     string           `json:"name"`
	Location LocationResponse `json:"location"`
	City     CityResponse     `json:"city"`
	Country  CountryResponse  `json:"country"`
}

// MetaResponse carries pagination metadata.
type MetaResponse struct {
	Total  int64  `json:"total"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
	Search string `json:"search"`
}

// SearchResponse is the top-level JSON envelope for a successful response.
type SearchResponse struct {
	Data []AirportResponse `json:"data"`
	Meta MetaResponse      `json:"meta"`
}
