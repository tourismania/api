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
type FlightSegmentInput struct {
	DepartureAirportICAO string    `json:"departure_airport_icao" validate:"required,len=4"`
	ArrivalAirportICAO   string    `json:"arrival_airport_icao"   validate:"required,len=4"`
	DepartureAt          time.Time `json:"departure_at"           validate:"required"`
	ArrivalAt            time.Time `json:"arrival_at"             validate:"required"`
	// CabinClass — класс обслуживания сегмента; необязателен (отсутствие
	// поля, null и "" — класс неизвестен). Допустимые значения проверяет
	// домен (справочник flight.CabinClasses), а не валидатор: oneof здесь
	// дублировал бы список и расходился с доменом при его расширении.
	CabinClass string `json:"cabin_class" example:"economy" enums:",economy,comfort,comfort_plus,business"`
}

// UpdateOfferResponse is the response envelope.
type UpdateOfferResponse struct {
	ID   int       `json:"id"`
	UUID uuid.UUID `json:"uuid"`
}
