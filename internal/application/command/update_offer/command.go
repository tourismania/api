// Package updateoffer holds the UpdateOffer command, its handler and
// result.
package updateoffer

import (
	"time"

	"api/internal/domain/enum"

	"github.com/google/uuid"
)

// FlightSegmentInput — DTO одного перелётного сегмента на границе
// Application-слоя. Презентационный слой конвертирует свой собственный
// DTO в этот тип и ничего не знает про domain/entity: сборка
// entity.FlightSegment/entity.Flight и их валидация происходят уже
// внутри Handler.
type FlightSegmentInput struct {
	DepartureAirportICAO string
	ArrivalAirportICAO   string
	DepartureAt          time.Time
	ArrivalAt            time.Time
}

// Command represents the intent to partially update an existing offer.
// Only non-nil fields are applied. The caller is identified only by
// CurrentUserUUID; the handler resolves agency_id/role from the DB.
type Command struct {
	UUID        uuid.UUID
	Title       *string
	Description *string
	Status      *enum.OfferStatus
	// Flights is a pointer to a slice of segment groups (one group per
	// flight), mirroring Title/Description/Status: nil means the
	// "flights" key was absent from the request and existing flights are
	// left untouched; a non-nil value (including an empty slice) fully
	// replaces the offer's flights after a content diff against what is
	// already stored.
	Flights *[][]FlightSegmentInput

	CurrentUserUUID uuid.UUID
}
