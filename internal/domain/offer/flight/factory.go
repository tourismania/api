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

// ErrManagerCommentInvalid возвращается New, если комментарий менеджера
// содержит управляющий символ C0 (U+0000–U+001F), кроме табуляции, перевода
// строки и возврата каретки, или U+007F. U+0000 PostgreSQL не хранит в TEXT,
// остальные в тексте для клиента бессмысленны и ломают вывод.
var ErrManagerCommentInvalid = errors.New("flight manager comment contains a forbidden control character")

// ErrAirlineRequired возвращается New, если у сегмента не указана
// авиакомпания (AirlineID = 0). Авиакомпания обязательна: если перевозчик ещё
// неизвестен, клиент явно передаёт airline.UnknownID, api сам ничего не
// подставляет. Существование авиакомпании проверяет Manager.
var ErrAirlineRequired = errors.New("flight segment airline is required")

// Границы запрещённых в комментарии менеджера управляющих символов.
const (
	lastC0Control = '\u001f'
	deleteControl = '\u007f'
)

// New собирает Flight с ценой price и комментарием менеджера managerComment
// из сегментов, уже упорядоченных от первого к последнему. Цена должна быть
// в валюте из справочника money и неотрицательной (нулевое значение
// money.Money отклоняется) — иначе money.ErrCurrencyUnsupported /
// money.ErrAmountInvalid. Комментарий сохраняется как есть (без обрезки и
// экранирования), пустая строка — «комментария нет»; управляющие символы,
// кроме \t, \n, \r, — ErrManagerCommentInvalid. Длину комментария New не
// проверяет — это правило формы запроса. Класс обслуживания каждого
// сегмента должен быть из CabinClasses или пустым (класс неизвестен) —
// иначе ErrCabinClassInvalid. Авиакомпания каждого сегмента обязательна:
// AirlineID = 0 — ErrAirlineRequired (отрицательный id проходит дальше и
// отклоняется Manager как несуществующий). Номер рейса сегмента New не
// проверяет — доменных правил у него нет. Затем проверяются
// все структурные инварианты, не требующие I/O: список сегментов не пуст,
// хронология внутри каждого сегмента, непрерывность маршрута между
// соседними сегментами и строго положительная пересадка на каждой
// остановке. Существование аэропортов и авиакомпаний New не проверяет — для
// этого нужен запрос к БД, это делает Manager.
func New(price money.Money, managerComment string, segments []Segment) (Flight, error) {
	if _, err := money.LookupCurrency(price.Currency.Code); err != nil {
		return Flight{}, err
	}
	if price.AmountMinor < 0 {
		return Flight{}, money.ErrAmountInvalid
	}
	if !managerCommentValid(managerComment) {
		return Flight{}, ErrManagerCommentInvalid
	}
	if len(segments) == 0 {
		return Flight{}, ErrSegmentsEmpty
	}

	for i, seg := range segments {
		if !seg.CabinClass.IsValid() {
			return Flight{}, ErrCabinClassInvalid
		}
		if seg.AirlineID == 0 {
			return Flight{}, ErrAirlineRequired
		}
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

	return Flight{Price: price, ManagerComment: managerComment, Segments: segments}, nil
}

// managerCommentValid сообщает, что в комментарии нет запрещённых
// управляющих символов: C0, кроме \t, \n, \r, и U+007F.
func managerCommentValid(comment string) bool {
	for _, r := range comment {
		switch {
		case r == '\t', r == '\n', r == '\r':
			continue
		case r <= lastC0Control, r == deleteControl:
			return false
		}
	}
	return true
}
