package mapper

import (
	"api/internal/domain/airport"
	"api/internal/infrastructure/persistence/postgres/db"
)

// ToAirportDomain converts a sqlc row to a domain entity.
func ToAirportDomain(row db.SearchAirportsRow) airport.Airport {
	elevFt := ptrInt32ToInt(row.ElevationFt)

	var lon, lat float64
	if row.Lon != nil {
		lon = *row.Lon
	}
	if row.Lat != nil {
		lat = *row.Lat
	}

	timezone := ""
	if row.CityTimezone != nil {
		timezone = *row.CityTimezone
	}

	return airport.Airport{
		ICAO: row.Icao,
		IATA: row.Iata,
		Name: row.AirportName,
		Location: airport.Location{
			Latitude:    lat,
			Longitude:   lon,
			ElevationFt: elevFt,
		},
		City: airport.City{
			ID:       int(row.CityID),
			Name:     row.CityName,
			State:    row.CityState,
			Timezone: timezone,
		},
		Country: airport.Country{
			ISO2: row.CountryIso2,
			Name: row.CountryName,
		},
	}
}

// ToAirportDomainFromICAORow converts a FindAirportsByICAOs row to a
// domain entity.
func ToAirportDomainFromICAORow(row db.FindAirportsByICAOsRow) airport.Airport {
	elevFt := ptrInt32ToInt(row.ElevationFt)

	var lon, lat float64
	if row.Lon != nil {
		lon = *row.Lon
	}
	if row.Lat != nil {
		lat = *row.Lat
	}

	timezone := ""
	if row.CityTimezone != nil {
		timezone = *row.CityTimezone
	}

	return airport.Airport{
		ICAO: row.Icao,
		IATA: row.Iata,
		Name: row.AirportName,
		Location: airport.Location{
			Latitude:    lat,
			Longitude:   lon,
			ElevationFt: elevFt,
		},
		City: airport.City{
			ID:       int(row.CityID),
			Name:     row.CityName,
			State:    row.CityState,
			Timezone: timezone,
		},
		Country: airport.Country{
			ISO2: row.CountryIso2,
			Name: row.CountryName,
		},
	}
}

func ptrInt32ToInt(v *int32) *int {
	if v == nil {
		return nil
	}
	i := int(*v)
	return &i
}
