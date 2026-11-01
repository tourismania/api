package flight

import (
	"errors"

	"api/internal/domain/airport"
	"api/internal/domain/money"
)

// ErrSegmentsEmpty is returned when New is given no segments.
// A Flight is 1 or more segments — there is no such thing as an empty
// flight.
var ErrSegmentsEmpty = errors.New("flight must have at least one segment")

// ErrSegmentChronologyInvalid возвращается CheckChronology, если реальный
// момент прилёта сегмента не строго позже реального момента его вылета.
var ErrSegmentChronologyInvalid = errors.New("flight segment arrival must be strictly after departure")

// ErrSegmentDiscontinuous is returned when the arrival airport of
// one segment does not match the departure airport of the next —
// "airport A" and "airport B" of a Flight are derived from the ends of
// a continuous chain, not independent fields.
var ErrSegmentDiscontinuous = errors.New("flight segments must form a continuous route")

// ErrLayoverNonPositive возвращается CheckChronology, если следующий сегмент
// реально вылетает в момент прилёта предыдущего или раньше: пересадка нулевой
// или отрицательной длительности физически невозможна.
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
// структурные инварианты, не требующие I/O: список сегментов не пуст и маршрут
// между соседними сегментами непрерывен. Хронологию и пересадки New не
// проверяет — для них нужны таймзоны аэропортов (см. CheckChronology).
// Существование аэропортов и авиакомпаний New тоже не проверяет — для этого
// нужен запрос к БД, это делает Manager.
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
		if i > 0 && segments[i-1].ArrivalAirportICAO != seg.DepartureAirportICAO {
			return Flight{}, ErrSegmentDiscontinuous
		}
	}

	return Flight{Price: price, ManagerComment: managerComment, Segments: segments}, nil
}

// CheckChronology проверяет хронологию перелёта по реальным моментам: прилёт
// каждого сегмента строго позже его вылета (ErrSegmentChronologyInvalid), а
// вылет следующего сегмента строго позже прилёта предыдущего
// (ErrLayoverNonPositive). Настенные часы могут идти и в обратном порядке:
// вылет из Екатеринбурга в 10:00 и прилёт в Москву в 09:00 — это час в полёте.
func CheckChronology(f Flight, zones airport.Zones) error {
	for i, seg := range f.Segments {
		if !seg.arrivalMoment(zones).After(seg.departureMoment(zones)) {
			return ErrSegmentChronologyInvalid
		}
		if i > 0 && !seg.departureMoment(zones).After(f.Segments[i-1].arrivalMoment(zones)) {
			return ErrLayoverNonPositive
		}
	}
	return nil
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
