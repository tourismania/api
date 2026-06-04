package mapper

import (
	"api/internal/domain/entity"
	"api/internal/domain/valueobject"
	"api/internal/infrastructure/persistence/postgres/db"
)

// ToAirportDomain converts a sqlc row to a domain entity.
func ToAirportDomain(row db.SearchAirportsRow) entity.Airport {
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

	return entity.Airport{
		ICAO: row.Icao,
		IATA: row.Iata,
		Name: row.AirportName,
		Location: valueobject.Location{
			Latitude:    lat,
			Longitude:   lon,
			ElevationFt: elevFt,
		},
		City: entity.City{
			ID:       int(row.CityID),
			Name:     row.CityName,
			State:    row.CityState,
			Timezone: timezone,
		},
		Country: entity.Country{
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
