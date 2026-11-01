// Package createofferhttp is the HTTP boundary for the CreateOffer
// command — request decoding, validation and response shaping.
package createofferhttp

import (
	"strings"

	"api/internal/domain/offer/flight"
	offerhttp "api/internal/presentation/http/api/v1/offer"

	"github.com/google/uuid"
)

// CreateOfferRequest is the public request body schema. agency_id is
// deliberately absent: it is always derived server-side from the
// caller's own agency.
type CreateOfferRequest struct {
	Title       string        `json:"title"       validate:"required,max=200"`
	Description string        `json:"description" validate:"max=5000"`
	Status      string        `json:"status"      validate:"required,oneof=draft ready published"`
	Flights     []FlightInput `json:"flights"     validate:"omitempty,dive"`
}

// FlightInput — один перелёт CreateOfferRequest: цена (десятичная строка, знаков
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
	DepartureAirportICAO string `json:"departure_airport_icao" validate:"required,len=4"`
	ArrivalAirportICAO   string `json:"arrival_airport_icao"   validate:"required,len=4"`
	// DepartureAt — местные дата и время аэропорта вылета, YYYY-MM-DDTHH:MM:SS без смещения и суффикса Z; реальный момент api определяет по таймзоне аэропорта.
	DepartureAt offerhttp.LocalDateTime `json:"departure_at" validate:"required" swaggertype:"string" example:"2026-05-14T10:00:00"`
	// ArrivalAt — местные дата и время аэропорта прилёта, YYYY-MM-DDTHH:MM:SS без смещения и суффикса Z; реальный момент api определяет по таймзоне аэропорта.
	ArrivalAt offerhttp.LocalDateTime `json:"arrival_at" validate:"required" swaggertype:"string" example:"2026-05-14T11:00:00"`
	// CabinClass — класс обслуживания сегмента; необязателен (отсутствие поля, null и "" — класс неизвестен). Допустимые значения — enum схемы flight.CabinClass (справочник классов обслуживания); недопустимое значение — 400 с meta {"CabinClass":"oneof"}.
	CabinClass flight.CabinClass `json:"cabin_class" example:"economy"`
	// AirlineID — id авиакомпании из поиска GET /api/v1/airlines; обязателен (нет поля, null или 0 — 400 с meta {"AirlineID":"required"}). Перевозчик неизвестен — 1 («Неизвестная»). Нет в справочнике или скрытая из поиска запись, на которую оффер ещё не ссылался, — 400 «flight segment references an unknown airline».
	AirlineID int `json:"airline_id" validate:"required" example:"1"`
	// FlightNumber — номер рейса сегмента, строка до 10 символов; пробелы по краям обрезаются до проверки длины, в остальном хранится как передан (формат не проверяется, регистр, пробелы внутри и разделители не меняются). Необязателен: отсутствие поля, null, "" и строка из одних пробелов — номер неизвестен. Длиннее 10 символов после обрезки — 400 с meta {"FlightNumber":"max"}.
	FlightNumber string `json:"flight_number" validate:"max=10" example:"SU1234"`
}

// Normalize обрезает пробелы по краям номеров рейсов сегментов; вызывается
// httpx.DecodeJSON до валидации, поэтому длина проверяется у обрезанного номера.
func (r *CreateOfferRequest) Normalize() {
	for i := range r.Flights {
		segs := r.Flights[i].Segments
		for j := range segs {
			segs[j].FlightNumber = strings.TrimSpace(segs[j].FlightNumber)
		}
	}
}

// CreateOfferResponse is the response envelope.
type CreateOfferResponse struct {
	ID   int       `json:"id"`
	UUID uuid.UUID `json:"uuid"`
}
