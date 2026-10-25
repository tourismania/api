package mapper

import (
	"strings"

	"api/internal/domain/airline"
	"api/internal/infrastructure/persistence/postgres/db"
)

// ToAirlineDomainFromSearchRow собирает авиакомпанию из строки поиска.
// Поиск скрытые записи не возвращает, коды IATA не читает.
func ToAirlineDomainFromSearchRow(row db.SearchAirlinesRow) airline.Airline {
	return airline.Airline{
		ID:          int(row.ID),
		ICAO:        row.Icao,
		Name:        row.Name,
		NameRU:      row.NameRu,
		Country:     toAirlineCountry(row.CountryIso2, row.CountryName),
		DissolvedOn: row.DissolvedOn,
	}
}

// ToAirlineDomainFromIDRow собирает авиакомпанию из строки ListAirlinesByIDs,
// с кодами IATA и состоянием в источнике (дубль, пропавшая).
func ToAirlineDomainFromIDRow(row db.ListAirlinesByIDsRow) airline.Airline {
	codes := make([]string, 0, len(row.IataCodes))
	for _, c := range row.IataCodes {
		// char(2) дополняется пробелами только у более коротких значений —
		// у кодов IATA их нет, но обрезка дешевле сюрприза.
		codes = append(codes, strings.TrimSpace(c))
	}
	return airline.Airline{
		ID:           int(row.ID),
		ICAO:         row.Icao,
		Name:         row.Name,
		NameRU:       row.NameRu,
		Country:      toAirlineCountry(row.CountryIso2, row.CountryName),
		DissolvedOn:  row.DissolvedOn,
		IATACodes:    codes,
		MergedIntoID: toIntPtr(row.MergedIntoID),
		MissingSince: row.MissingSince,
	}
}

// toIntPtr переводит nullable int32 столбца в *int домена.
func toIntPtr(v *int32) *int {
	if v == nil {
		return nil
	}
	i := int(*v)
	return &i
}

// toAirlineCountry — страна авиакомпании или nil, если её нет (LEFT JOIN).
func toAirlineCountry(iso2, name *string) *airline.Country {
	if iso2 == nil {
		return nil
	}
	c := airline.Country{ISO2: *iso2}
	if name != nil {
		c.Name = *name
	}
	return &c
}
