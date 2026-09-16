// Package createoffer holds the CreateOffer command, its handler and
// result.
package createoffer

import (
	"api/internal/domain/offer"
	"time"

	"github.com/google/uuid"
)

// FlightSegmentInput — DTO одного перелётного сегмента на границе
// Application-слоя. Презентационный слой конвертирует свой собственный
// DTO в этот тип и ничего не знает про domain/offer: сборка
// offer.FlightSegment/offer.Flight и их валидация происходят уже
// внутри Handler.
type FlightSegmentInput struct {
	DepartureAirportICAO string
	ArrivalAirportICAO   string
	DepartureAt          time.Time
	ArrivalAt            time.Time
}

// Command represents the intent to publish a new offer under the
// caller's own agency. The request body never carries agency_id: the
// caller is identified only by CurrentUserUUID (the immutable uuid
// carried in the JWT); the handler resolves agency_id and role from the
// DB itself, so they always reflect the latest state rather than a
// value trusted from presentation.
type Command struct {
	Title       string
	Description string
	Status      offer.Status
	// Flights is one group of segments per flight, first-to-last within
	// each group. A nil/empty slice means the offer is created without
	// flights.
	Flights [][]FlightSegmentInput

	CurrentUserUUID uuid.UUID
}
