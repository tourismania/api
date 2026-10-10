// Package createofferhttp is the HTTP boundary for the CreateOffer
// command — request decoding, validation and response shaping.
package createofferhttp

import (
	"time"

	"github.com/google/uuid"
)

// CreateOfferRequest is the public request body schema. agency_id is
// deliberately absent: it is always derived server-side from the
// caller's own agency.
type CreateOfferRequest struct {
	Title       string        `json:"title"       validate:"required,max=200"`
	Description string        `json:"description" validate:"max=5000"`
	Status      string        `json:"status"      validate:"required,oneof=draft ready published"`
	Flights     []FlightInput `json:"flights"     validate:"omitempty,dive"`
}

// FlightInput — один перелёт CreateOfferRequest: цена (десятичная строка, знаков
// после точки не больше, чем у валюты), валюта (код ISO 4217 из системного
// справочника) и сегменты от аэропорта вылета до аэропорта прилёта, по
// порядку, с пересадками. Здесь проверяется только форма запроса
// (обязательность полей); формат цены и валюту по справочнику проверяет
// application-слой и возвращает apperror.FieldError.
type FlightInput struct {
	Price    string               `json:"price"    validate:"required" example:"12345.50"`
	Currency string               `json:"currency" validate:"required" example:"RUB"`
	Segments []FlightSegmentInput `json:"segments" validate:"required,min=1,dive"`
}

// FlightSegmentInput is one nonstop leg of a FlightInput.
type FlightSegmentInput struct {
	DepartureAirportICAO string    `json:"departure_airport_icao" validate:"required,len=4"`
	ArrivalAirportICAO   string    `json:"arrival_airport_icao"   validate:"required,len=4"`
	DepartureAt          time.Time `json:"departure_at"           validate:"required"`
	ArrivalAt            time.Time `json:"arrival_at"             validate:"required"`
}

// CreateOfferResponse is the response envelope.
type CreateOfferResponse struct {
	ID   int       `json:"id"`
	UUID uuid.UUID `json:"uuid"`
}
