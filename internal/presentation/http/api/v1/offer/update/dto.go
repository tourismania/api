// Package updateofferhttp is the HTTP boundary for the UpdateOffer
// command — request decoding, validation and response shaping.
package updateofferhttp

import (
	"time"

	"github.com/google/uuid"
)

// UpdateOfferRequest is the public request body schema. Every field is
// optional: only non-nil fields are applied (partial update). Flights
// follows the same nil-vs-present convention: absent means untouched,
// present (including []) fully replaces the offer's flights.
type UpdateOfferRequest struct {
	Title       *string        `json:"title"       validate:"omitempty,max=200"`
	Description *string        `json:"description" validate:"omitempty,max=5000"`
	Status      *string        `json:"status"      validate:"omitempty,oneof=draft ready published"`
	Flights     *[]FlightInput `json:"flights"     validate:"omitempty,dive"`
}

// FlightInput is one flight of an UpdateOfferRequest: an ordered list of
// nonstop segments from the flight's departure airport to its arrival
// airport, possibly via layovers.
type FlightInput struct {
	Segments []FlightSegmentInput `json:"segments" validate:"required,min=1,dive"`
}

// FlightSegmentInput is one nonstop leg of a FlightInput.
type FlightSegmentInput struct {
	DepartureAirportICAO string    `json:"departure_airport_icao" validate:"required,len=4"`
	ArrivalAirportICAO   string    `json:"arrival_airport_icao"   validate:"required,len=4"`
	DepartureAt          time.Time `json:"departure_at"           validate:"required"`
	ArrivalAt            time.Time `json:"arrival_at"             validate:"required"`
}

// UpdateOfferResponse is the response envelope.
type UpdateOfferResponse struct {
	ID   int       `json:"id"`
	UUID uuid.UUID `json:"uuid"`
}
