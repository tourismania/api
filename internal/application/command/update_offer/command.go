// Package updateoffer holds the UpdateOffer command, its handler and
// result.
package updateoffer

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

// Command represents the intent to partially update an existing offer.
// Only non-nil fields are applied. The caller is identified only by
// CurrentUserUUID; the handler resolves agency_id/role from the DB.
type Command struct {
	UUID        uuid.UUID
	Title       *string
	Description *string
	Status      *offer.Status
	// Flights — указатель на срез перелётов, как у Title/Description/Status:
	// nil — ключа "flights" в запросе не было, сохранённые перелёты не
	// трогаются; не-nil (в том числе пустой срез) — новый полный набор
	// перелётов оффера. flight.Manager сравнивает его с сохранённым и, если
	// изменилась только цена, обновляет только её.
	Flights *[]FlightInput

	CurrentUserUUID uuid.UUID
}
