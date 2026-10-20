package searchairlines

import "time"

// CountryResult — страна авиакомпании.
type CountryResult struct {
	ISO2 string
	Name string
}

// AirlineResult — одна авиакомпания выдачи. ICAO nil — у служебной записи;
// NameRU "" — русского названия нет; Country nil — страна неизвестна;
// DissolvedOn nil — авиакомпания действует. Коды IATA не отдаются.
type AirlineResult struct {
	ID          int
	ICAO        *string
	Name        string
	NameRU      string
	Country     *CountryResult
	DissolvedOn *time.Time
}

// Result — страница выдачи и общее число найденных.
type Result struct {
	Airlines   []AirlineResult
	TotalCount int64
}
