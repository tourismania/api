package valueobject

// Location represents the geographic position of an airport.
type Location struct {
	Latitude    float64
	Longitude   float64
	ElevationFt *int
}
