// Package model contains PostgreSQL row representations.
// These are NOT domain entities — they mirror the exact DB column layout.
package model

// Country is the DB row for the countries table.
type Country struct {
	ISO2 string // char(2) PRIMARY KEY
	Name string // varchar(100) NOT NULL
}

// City is the DB row for the cities table.
type City struct {
	ID          int     // serial PRIMARY KEY
	Name        string  // varchar(100) NOT NULL
	State       *string // varchar(100) nullable
	Timezone    string  // varchar(50)
	CountryISO2 string  // char(2) FK -> countries.iso2
}

// Airport is the DB row for the airports table.
type Airport struct {
	ICAO        string    // char(4) PRIMARY KEY
	IATA        *string   // char(3) UNIQUE nullable
	Name        string    // varchar(200) NOT NULL
	Location    []float64 // float8[] — [longitude, latitude]
	ElevationFt *int      // int nullable
	CityID      int       // int FK -> cities.id
}
