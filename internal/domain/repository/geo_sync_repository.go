package repository

import "context"

// GeoSyncRepository is the write-port for the airport-sync command.
// Implementations live in infrastructure/persistence.
type GeoSyncRepository interface {
	// UpsertCountry inserts or updates a country by ISO-2 code.
	UpsertCountry(ctx context.Context, iso2, name string) error

	// UpsertCity inserts the city if no matching row exists (matched by
	// lower(name) + state + country_iso2), otherwise returns the existing ID.
	// Returns the database id of the city.
	UpsertCity(ctx context.Context, name string, state *string, timezone, countryISO2 string) (int, error)

	// UpsertAirport inserts or updates an airport by ICAO code.
	UpsertAirport(ctx context.Context, icao string, iata *string, name string, lat, lon float64, elevationFt *int, cityID int) error
}
