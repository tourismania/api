// Package createoffer holds the CreateOffer command, its handler and
// result.
package createoffer

import (
	"api/internal/domain/offer"
	"time"

	"github.com/google/uuid"
)

// FlightInput — DTO одного перелёта на границе Application-слоя: цена
// десятичной строкой, код валюты и сегменты от первого к последнему.
// Разбор цены по справочнику валют (money.Parse) и сборка flight.Flight
// происходят внутри Handler.
type FlightInput struct {
	Price    string
	Currency string
	Segments []FlightSegmentInput
}

// FlightSegmentInput — DTO одного перелётного сегмента на границе
// Application-слоя. Презентационный слой конвертирует свой собственный
// DTO в этот тип и ничего не знает про domain/offer: сборка
// flight.Segment/flight.Flight и их валидация происходят уже
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
	// Flights — перелёты оффера в порядке сохранения. A nil/empty slice
	// means the offer is created without flights.
	Flights []FlightInput

	CurrentUserUUID uuid.UUID
}
