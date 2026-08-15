package getoffer

import (
	"time"

	"api/internal/domain/enum"

	"github.com/google/uuid"
)

// Result is the application-layer view of a single offer returned to
// the presentation layer.
type Result struct {
	ID          int
	UUID        uuid.UUID
	Title       string
	Description string
	AgencyID    int
	CreatedBy   int
	Status      enum.OfferStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Flights     []FlightResult
}

// FlightResult — уже вычисленная проекция одного перелёта offer'а:
// суммарная длительность и пересадки посчитаны здесь, в Application-
// слое (через доменные методы entity.Flight), а не в presentation.
// Presentation лишь копирует поля в свой wire-DTO, не обращаясь к
// domain-типам и не вызывая доменное поведение напрямую.
type FlightResult struct {
	ID                   int
	DepartureAirportICAO string
	ArrivalAirportICAO   string
	TotalDurationSeconds int64
	Segments             []FlightSegmentResult
	Layovers             []LayoverResult
}

// FlightSegmentResult — один беспосадочный участок FlightResult.
type FlightSegmentResult struct {
	DepartureAirportICAO string
	ArrivalAirportICAO   string
	DepartureAt          time.Time
	ArrivalAt            time.Time
	DurationSeconds      int64
}

// LayoverResult — пересадка между двумя соседними сегментами FlightResult.
type LayoverResult struct {
	AirportICAO     string
	DurationSeconds int64
}
