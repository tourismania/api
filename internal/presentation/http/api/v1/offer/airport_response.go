package offerhttp

import "api/internal/application/helpers"

// AirportResponse — аэропорт сегмента в карточке оффера и публичной ссылке:
// текущие данные справочника. Все поля присутствуют всегда.
type AirportResponse struct {
	ICAO string `json:"icao" example:"UUEE"`
	// IATA — код IATA; null — кода нет.
	IATA    *string                `json:"iata" example:"SVO" extensions:"x-nullable"`
	Name    string                 `json:"name" example:"Шереметьево"`
	City    AirportCityResponse    `json:"city"`
	Country AirportCountryResponse `json:"country"`
}

// AirportCityResponse — город аэропорта сегмента.
type AirportCityResponse struct {
	Name string `json:"name" example:"Москва"`
	// Timezone — таймзона города (IANA); null — не указана, время аэропорта считается UTC.
	Timezone *string `json:"timezone" example:"Europe/Moscow" extensions:"x-nullable"`
}

// AirportCountryResponse — страна аэропорта сегмента.
type AirportCountryResponse struct {
	ISO2 string `json:"iso2" example:"RU"`
	Name string `json:"name" example:"Россия"`
}

// NewAirportResponse — аэропорт в формате ответа; пустая таймзона — null.
func NewAirportResponse(a helpers.AirportResult) AirportResponse {
	var timezone *string
	if a.City.Timezone != "" {
		timezone = &a.City.Timezone
	}
	return AirportResponse{
		ICAO:    a.ICAO,
		IATA:    a.IATA,
		Name:    a.Name,
		City:    AirportCityResponse{Name: a.City.Name, Timezone: timezone},
		Country: AirportCountryResponse{ISO2: a.Country.ISO2, Name: a.Country.Name},
	}
}
