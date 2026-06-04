package repository

import "context"

// CityRepository is the write-port for the airport-sync command.
// Implementations live in infrastructure/persistence.
type CityRepository interface {

	// Upsert UpsertCity inserts the city if no matching row exists (matched by
	// lower(name) + state + country_iso2), otherwise returns the existing ID.
	// Returns the database id of the city.
	Upsert(ctx context.Context, name string, state *string, timezone, countryISO2 string) (int, error)
}
