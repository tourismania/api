package getairline

import "time"

// CountryResult — страна авиакомпании.
type CountryResult struct {
	ISO2 string
	Name string
}

// Result — авиакомпания с признаком Hidden: дубль или пропавшая из
// источника, для нового сегмента не выбирается. Остальные поля — как у
// элемента поиска; коды IATA не отдаются.
type Result struct {
	ID          int
	ICAO        *string
	Name        string
	NameRU      string
	Country     *CountryResult
	DissolvedOn *time.Time
	Hidden      bool
}
