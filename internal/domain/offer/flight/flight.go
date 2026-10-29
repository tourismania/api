// Package flight is the Flight child aggregate of an Offer: the Flight
// entity with its Segments and Layovers, the structural factory (New),
// the repository contract and the replace-set manager. It lives as a
// subpackage of domain/offer because a flight never exists outside an
// offer (every contract is scoped by offerID); future offer children
// (hotels, transfers, rents) follow the same layout — one subpackage
// per child aggregate, so the parent offer package stays the aggregate
// root only. Children may import the parent package; the parent never
// imports its children — composition happens in the application layer.
package flight

import (
	"time"

	"api/internal/domain/money"
)

// Segment is one nonstop leg of a Flight. CabinClass — класс обслуживания
// сегмента (у сегментов одного перелёта классы могут различаться);
// CabinClassUnknown — класс неизвестен. AirlineID — id авиакомпании в
// справочнике airline, обязателен (перевозчик неизвестен — явно
// airline.UnknownID); сегмент ссылается именно на эту запись, какие бы коды
// ни были у неё и у других авиакомпаний. FlightNumber — номер рейса сегмента
// в том виде, в каком передан (формат не проверяется, регистр, пробелы внутри
// и разделители не меняются); "" — номер неизвестен. Обрезка пробелов по
// краям и длина номера — правила формы запроса (длина — как у комментария
// менеджера): New их не применяет. Класс,
// авиакомпания и номер рейса в маршрут не входят: их изменение обновляет
// сегмент на месте (см. Manager.ReplaceForOffer).
type Segment struct {
	DepartureAirportICAO string
	ArrivalAirportICAO   string
	DepartureAt          time.Time
	ArrivalAt            time.Time
	CabinClass           CabinClass
	AirlineID            int
	FlightNumber         string
}

// Duration is the segment's own flight time.
func (s Segment) Duration() time.Duration {
	return s.ArrivalAt.Sub(s.DepartureAt)
}

// Layover is the wait between the arrival of one segment and the
// departure of the next.
type Layover struct {
	AirportICAO string
	Duration    time.Duration
}

// Flight — путь из аэропорта вылета первого сегмента в аэропорт прилёта
// последнего, из одного или нескольких беспосадочных сегментов: 2 и более
// сегмента — это одна или несколько пересадок. Price — цена перелёта
// целиком (не сегмента) в валюте из справочника money. ManagerComment —
// комментарий менеджера к перелёту целиком, текст для клиента; "" —
// комментария нет. Структурные
// инварианты (есть сегменты, хронология, непрерывность маршрута,
// положительные пересадки) проверяет New, а не этот тип: он всегда несёт
// уже проверенные данные. TotalDuration и Layovers всегда вычисляются из
// Segments на лету и не хранятся.
type Flight struct {
	ID             int
	OfferID        int
	Price          money.Money
	ManagerComment string
	Segments       []Segment
}

// DepartureAirportICAO is the airport of the flight's first segment.
func (f Flight) DepartureAirportICAO() string {
	return f.Segments[0].DepartureAirportICAO
}

// ArrivalAirportICAO is the airport of the flight's last segment.
func (f Flight) ArrivalAirportICAO() string {
	return f.Segments[len(f.Segments)-1].ArrivalAirportICAO
}

// TotalDuration is the elapsed time from the first segment's departure
// to the last segment's arrival, including every layover in between.
func (f Flight) TotalDuration() time.Duration {
	first := f.Segments[0]
	last := f.Segments[len(f.Segments)-1]
	return last.ArrivalAt.Sub(first.DepartureAt)
}

// Layovers returns one entry per stop between consecutive segments, in
// order. A nonstop flight (single segment) returns nil.
func (f Flight) Layovers() []Layover {
	if len(f.Segments) < 2 {
		return nil
	}
	layovers := make([]Layover, 0, len(f.Segments)-1)
	for i := 1; i < len(f.Segments); i++ {
		prev, next := f.Segments[i-1], f.Segments[i]
		layovers = append(layovers, Layover{
			AirportICAO: prev.ArrivalAirportICAO,
			Duration:    next.DepartureAt.Sub(prev.ArrivalAt),
		})
	}
	return layovers
}
