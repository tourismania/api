// Package getofferhttp is the HTTP boundary for the GetOffer query.
package getofferhttp

import (
	"time"

	"api/internal/domain/offer/flight"
	offerhttp "api/internal/presentation/http/api/v1/offer"

	"github.com/google/uuid"
)

// OfferResponse is the public projection of a single offer.
type OfferResponse struct {
	ID          int              `json:"id"`
	UUID        uuid.UUID        `json:"uuid"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	AgencyID    int              `json:"agency_id"`
	CreatedBy   int              `json:"created_by"`
	Status      string           `json:"status"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
	Flights     []FlightResponse `json:"flights"`
}

// FlightResponse is a computed projection of one of the offer's flights.
type FlightResponse struct {
	ID int `json:"id"`
	// Price — цена перелёта строкой ровно с тем числом знаков после
	// точки, что у валюты.
	Price    string `json:"price"    example:"12345.50"`
	Currency string `json:"currency" example:"RUB"`
	// ManagerComment — комментарий менеджера к перелёту как есть; поле
	// присутствует всегда, "" — комментария нет.
	ManagerComment       string                  `json:"manager_comment" example:"Багаж оплачивается отдельно"`
	DepartureAirportICAO string                  `json:"departure_airport_icao"`
	ArrivalAirportICAO   string                  `json:"arrival_airport_icao"`
	TotalDurationSeconds int64                   `json:"total_duration_seconds"`
	Segments             []FlightSegmentResponse `json:"segments"`
	Layovers             []LayoverResponse       `json:"layovers"`
}

// FlightSegmentResponse is one nonstop leg of a FlightResponse.
//
// CabinClass имеет доменный тип flight.CabinClass: swag строит список значений
// (enum) из его констант сам, поэтому в DTO значения не перечисляются ни в
// oneof, ни в enums — единственный источник — домен, проверку допустимости
// делает домен. См. docs/cabin-classes.md.
type FlightSegmentResponse struct {
	DepartureAirportICAO string `json:"departure_airport_icao"`
	ArrivalAirportICAO   string `json:"arrival_airport_icao"`
	// DepartureAirport — текущие данные аэропорта вылета из справочника; всегда объект.
	DepartureAirport offerhttp.AirportResponse `json:"departure_airport"`
	// ArrivalAirport — текущие данные аэропорта прилёта из справочника; всегда объект.
	ArrivalAirport offerhttp.AirportResponse `json:"arrival_airport"`
	// DepartureAt — местные дата и время аэропорта вылета в том виде, в каком сохранены, без смещения.
	DepartureAt offerhttp.LocalDateTime `json:"departure_at" swaggertype:"string" example:"2026-05-14T10:00:00"`
	// ArrivalAt — местные дата и время аэропорта прилёта в том виде, в каком сохранены, без смещения.
	ArrivalAt offerhttp.LocalDateTime `json:"arrival_at" swaggertype:"string" example:"2026-05-14T11:00:00"`
	// DurationSeconds — реальное время полёта сегмента по таймзонам аэропортов, без пересадок.
	DurationSeconds int64 `json:"duration_seconds"`
	// CabinClass — класс обслуживания сегмента; поле присутствует всегда, "" — класс неизвестен. Возможные значения — enum схемы flight.CabinClass (справочник классов обслуживания).
	CabinClass flight.CabinClass `json:"cabin_class" example:"economy"`
	// FlightNumber — номер рейса сегмента в том виде, как передан (без пробелов по краям); поле присутствует всегда, "" — номер неизвестен.
	FlightNumber string `json:"flight_number" example:"SU1234"`
	// AirlineID — id авиакомпании сегмента в справочнике.
	AirlineID int `json:"airline_id" example:"1"`
	// Airline — текущие данные авиакомпании сегмента из справочника; всегда объект.
	Airline AirlineResponse `json:"airline"`
}

// airlineDateLayout — формат даты прекращения деятельности авиакомпании.
const airlineDateLayout = "2006-01-02"

// AirlineResponse — авиакомпания сегмента. Коды IATA не отдаются.
type AirlineResponse struct {
	ID int `json:"id" example:"1"`
	// ICAO — код ICAO; null только у служебной записи «Неизвестная» (id 1).
	ICAO *string `json:"icao" example:"AFL"`
	// Name — английское название.
	Name string `json:"name" example:"Aeroflot"`
	// NameRU — русское название, только у авиакомпаний России; поле присутствует всегда, "" — нет.
	NameRU string `json:"name_ru" example:"Аэрофлот"`
	// DissolvedOn — дата прекращения деятельности YYYY-MM-DD; null — действует.
	DissolvedOn *string `json:"dissolved_on" example:"2015-10-26"`
}

// LayoverResponse is the wait between two consecutive segments of a
// FlightResponse.
type LayoverResponse struct {
	AirportICAO     string `json:"airport_icao"`
	DurationSeconds int64  `json:"duration_seconds"`
}
