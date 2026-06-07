package repository

import "context"

// CountryRepository is the write-port for the airport-sync command.
// Implementations live in infrastructure/persistence.
type CountryRepository interface {

	// Upsert inserts or updates a country by ISO-2 code.
	Upsert(ctx context.Context, iso2, name string) error
}
