// Package updateoffer holds the UpdateOffer command, its handler and
// result.
package updateoffer

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

// Command represents the intent to partially update an existing offer.
// Only non-nil fields are applied. The caller is identified only by
// CurrentUserUUID; the handler resolves agency_id/role from the DB.
type Command struct {
	UUID        uuid.UUID
	Title       *string
	Description *string
	Status      *offer.Status
	// Flights is a pointer to a slice of flights, mirroring
	// Title/Description/Status: nil means the "flights" key was absent
	// from the request and existing flights are left untouched; a non-nil
	// value (including an empty slice) is the offer's new full set —
	// flight.Manager сравнивает его с сохранённым и, если изменилась
	// только цена, обновляет только её.
	Flights *[]FlightInput

	CurrentUserUUID uuid.UUID
}
