// Package updateofferhttp is the HTTP boundary for the UpdateOffer
// command — request decoding, validation and response shaping.
package updateofferhttp

import (
	"time"

	"api/internal/domain/offer/flight"

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

// FlightInput — один перелёт UpdateOfferRequest: цена (десятичная строка, знаков
// после точки не больше, чем у валюты), валюта (код ISO 4217 из системного
// справочника), необязательный комментарий менеджера (текст для клиента,
// до 2000 символов; отсутствие поля, null и "" — комментария нет) и
// сегменты от аэропорта вылета до аэропорта прилёта, по порядку, с
// пересадками. Здесь проверяется только форма запроса (обязательность
// полей, длина комментария); формат цены, валюту по справочнику и
// управляющие символы в комментарии проверяет application-слой и
// возвращает apperror.FieldError.
type FlightInput struct {
	Price          string               `json:"price"    validate:"required" example:"12345.50"`
	Currency       string               `json:"currency" validate:"required" example:"RUB"`
	ManagerComment string               `json:"manager_comment" validate:"max=2000" example:"Багаж оплачивается отдельно"`
	Segments       []FlightSegmentInput `json:"segments" validate:"required,min=1,dive"`
}

// FlightSegmentInput is one nonstop leg of a FlightInput.
//
// CabinClass имеет доменный тип flight.CabinClass: swag строит список значений
// (enum) из его констант сам, поэтому в DTO значения не перечисляются ни в
// oneof, ни в enums — единственный источник — домен, проверку допустимости
// делает домен. См. docs/cabin-classes.md.
type FlightSegmentInput struct {
	DepartureAirportICAO string    `json:"departure_airport_icao" validate:"required,len=4"`
	ArrivalAirportICAO   string    `json:"arrival_airport_icao"   validate:"required,len=4"`
	DepartureAt          time.Time `json:"departure_at"           validate:"required"`
	ArrivalAt            time.Time `json:"arrival_at"             validate:"required"`
	// CabinClass — класс обслуживания сегмента; необязателен (отсутствие поля, null и "" — класс неизвестен). Допустимые значения — enum схемы flight.CabinClass (справочник классов обслуживания); недопустимое значение — 400 с meta {"CabinClass":"oneof"}.
	CabinClass flight.CabinClass `json:"cabin_class" example:"economy"`
	// AirlineID — id авиакомпании из поиска GET /api/v1/airlines; обязателен (нет поля, null или 0 — 400 с meta {"AirlineID":"required"}). Перевозчик неизвестен — 1 («Неизвестная»). Нет в справочнике или скрытая из поиска запись, на которую оффер ещё не ссылался, — 400 «flight segment references an unknown airline».
	AirlineID int `json:"airline_id" validate:"required" example:"1"`
}

// UpdateOfferResponse is the response envelope.
type UpdateOfferResponse struct {
	ID   int       `json:"id"`
	UUID uuid.UUID `json:"uuid"`
}
