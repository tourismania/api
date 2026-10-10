package flight

import (
	"errors"

	"api/internal/domain/money"
)

// ErrSegmentsEmpty is returned when New is given no segments.
// A Flight is 1 or more segments — there is no such thing as an empty
// flight.
var ErrSegmentsEmpty = errors.New("flight must have at least one segment")

// ErrSegmentChronologyInvalid is returned when a segment's
// arrival is not strictly after its own departure.
var ErrSegmentChronologyInvalid = errors.New("flight segment arrival must be strictly after departure")

// ErrSegmentDiscontinuous is returned when the arrival airport of
// one segment does not match the departure airport of the next —
// "airport A" and "airport B" of a Flight are derived from the ends of
// a continuous chain, not independent fields.
var ErrSegmentDiscontinuous = errors.New("flight segments must form a continuous route")

// ErrLayoverNonPositive is returned when the next segment departs
// at or before the previous segment arrives — a layover of zero or
// negative duration is not physically possible.
var ErrLayoverNonPositive = errors.New("flight layover between segments must be strictly positive")

// New собирает Flight с ценой price из сегментов, уже упорядоченных от
// первого к последнему. Цена должна быть в валюте из справочника money и
// неотрицательной (нулевое значение money.Money отклоняется) — иначе
// money.ErrCurrencyUnsupported / money.ErrAmountInvalid. Затем проверяются
// все структурные инварианты, не требующие I/O: список сегментов не пуст,
// хронология внутри каждого сегмента, непрерывность маршрута между
// соседними сегментами и строго положительная пересадка на каждой
// остановке. Существование аэропортов New не проверяет — для этого нужен
// запрос к БД, это делает Manager.
func New(price money.Money, segments []Segment) (Flight, error) {
	if _, err := money.LookupCurrency(price.Currency.Code); err != nil {
		return Flight{}, err
	}
	if price.AmountMinor < 0 {
		return Flight{}, money.ErrAmountInvalid
	}
	if len(segments) == 0 {
		return Flight{}, ErrSegmentsEmpty
	}

	for i, seg := range segments {
		if !seg.ArrivalAt.After(seg.DepartureAt) {
			return Flight{}, ErrSegmentChronologyInvalid
		}
		if i == 0 {
			continue
		}
		prev := segments[i-1]
		if prev.ArrivalAirportICAO != seg.DepartureAirportICAO {
			return Flight{}, ErrSegmentDiscontinuous
		}
		if !seg.DepartureAt.After(prev.ArrivalAt) {
			return Flight{}, ErrLayoverNonPositive
		}
	}

	return Flight{Price: price, Segments: segments}, nil
}
