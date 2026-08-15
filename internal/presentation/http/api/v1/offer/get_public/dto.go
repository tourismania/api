// Package getpublicofferhttp is the HTTP boundary for the
// GetPublishedOffer query (GET /api/v1/public/offers/{uuid} — the fully
// anonymous "share link" endpoint).
package getpublicofferhttp

import (
	"time"

	"github.com/google/uuid"
)

// OfferResponse is the public projection of a published offer. It is
// deliberately narrower than the private OfferResponse: no created_by
// (audit-only, internal) and no status (this endpoint only ever returns
// published offers).
type OfferResponse struct {
	ID          int              `json:"id"`
	UUID        uuid.UUID        `json:"uuid"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	AgencyID    int              `json:"agency_id"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
	Flights     []FlightResponse `json:"flights"`
}

// FlightResponse is a computed projection of one of the offer's flights.
type FlightResponse struct {
	ID                   int                     `json:"id"`
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
}

// LayoverResponse is the wait between two consecutive segments of a
// FlightResponse.
type LayoverResponse struct {
	AirportICAO     string `json:"airport_icao"`
	DurationSeconds int64  `json:"duration_seconds"`
}
