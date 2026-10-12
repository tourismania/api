// Package getofferhttp is the HTTP boundary for the GetOffer query.
package getofferhttp

import (
	"time"

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
type FlightSegmentResponse struct {
	DepartureAirportICAO string    `json:"departure_airport_icao"`
	ArrivalAirportICAO   string    `json:"arrival_airport_icao"`
	DepartureAt          time.Time `json:"departure_at"`
	ArrivalAt            time.Time `json:"arrival_at"`
	DurationSeconds      int64     `json:"duration_seconds"`
	// CabinClass — класс обслуживания сегмента; поле присутствует всегда,
	// "" — класс неизвестен.
	CabinClass string `json:"cabin_class" example:"economy" enums:",economy,comfort,comfort_plus,business"`
}

// LayoverResponse is the wait between two consecutive segments of a
// FlightResponse.
type LayoverResponse struct {
	AirportICAO     string `json:"airport_icao"`
	DurationSeconds int64  `json:"duration_seconds"`
}
