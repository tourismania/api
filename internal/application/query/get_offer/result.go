package getoffer

import (
	"api/internal/domain/offer"
	"time"

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
	Status      offer.Status
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Flights     []FlightResult
}

// FlightResult — уже вычисленная проекция одного перелёта offer'а:
// суммарная длительность и пересадки посчитаны здесь, в Application-
// слое (через доменные методы flight.Flight), а не в presentation.
// Presentation лишь копирует поля в свой wire-DTO, не обращаясь к
// domain-типам и не вызывая доменное поведение напрямую.
type FlightResult struct {
	ID int
	// Price — цена перелёта десятичной строкой ровно с тем числом знаков
	// после точки, что у валюты (money.Money.String()).
	Price    string
	Currency string
	// ManagerComment — комментарий менеджера к перелёту как есть; "" —
	// комментария нет.
	ManagerComment       string
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
	// CabinClass — класс обслуживания сегмента; "" — класс неизвестен.
	CabinClass string
	// FlightNumber — номер рейса сегмента в том виде, как передан (без пробелов по краям); "" — номер неизвестен.
	FlightNumber string
	// AirlineID — id авиакомпании сегмента в справочнике.
	AirlineID int
	// Airline — текущие данные этой авиакомпании из справочника.
	Airline AirlineResult
}

// AirlineResult — авиакомпания сегмента: ICAO nil — у служебной записи
// «Неизвестная»; NameRU "" — русского названия нет; DissolvedOn nil —
// действует. Коды IATA не отдаются.
type AirlineResult struct {
	ID          int
	ICAO        *string
	Name        string
	NameRU      string
	DissolvedOn *time.Time
}

// LayoverResult — пересадка между двумя соседними сегментами FlightResult.
type LayoverResult struct {
	AirportICAO     string
	DurationSeconds int64
}
