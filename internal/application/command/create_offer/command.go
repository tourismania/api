// Package createoffer holds the CreateOffer command, its handler and
// result.
package createoffer

import (
	"api/internal/application/flightinput"
	"api/internal/domain/offer"

	"github.com/google/uuid"
)

// FlightInput — DTO одного перелёта (цена, валюта, сегменты); общий для
// create_offer и update_offer, см. flightinput.Flight.
type FlightInput = flightinput.Flight

// FlightSegmentInput — DTO одного сегмента перелёта, см. flightinput.Segment.
type FlightSegmentInput = flightinput.Segment

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
	// Flights — перелёты оффера в порядке сохранения. nil или пустой срез —
	// оффер создаётся без перелётов.
	Flights []FlightInput

	CurrentUserUUID uuid.UUID
}
