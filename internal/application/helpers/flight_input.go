// Package helpers — общие помощники application-слоя, которые нужны
// нескольким use case'ам, но не являются ни use case'ом, ни законченным
// сервисом или маппером. Сейчас здесь DTO перелёта и его сборка в доменный
// flight.Flight, общие для команд create_offer и update_offer: обе
// принимают перелёты в одном виде, поэтому разбор цены и сборка живут в
// одном экземпляре, а пакеты команд объявляют на эти типы псевдонимы
// (createoffer.FlightInput и т. д.).
package helpers

import (
	"errors"
	"time"

	"api/internal/application/apperror"
	"api/internal/domain/money"
	"api/internal/domain/offer/flight"
)

// Имена полей и правил, под которыми ошибки цены уходят в meta ответа
// «validation failed» — так же, как ошибки валидатора presentation-слоя.
const (
	priceField    = "Price"
	priceRule     = "money"
	currencyField = "Currency"
	currencyRule  = "currency"
	// managerCommentField / managerCommentRule — комментарий менеджера с
	// запрещёнными управляющими символами.
	managerCommentField = "ManagerComment"
	managerCommentRule  = "text"
	// cabinClassField / cabinClassRule — класс обслуживания сегмента не из
	// справочника flight.CabinClasses. Правило "oneof" — как у перечислений
	// валидатора presentation, хотя проверяет его домен.
	cabinClassField = "CabinClass"
	cabinClassRule  = "oneof"
)

// FlightInput — DTO одного перелёта на границе Application-слоя: цена
// десятичной строкой, код валюты, комментарий менеджера ("" — комментария
// нет) и сегменты от первого к последнему.
// Presentation заполняет его структурно, ничего не зная про domain/offer.
type FlightInput struct {
	Price          string
	Currency       string
	ManagerComment string
	Segments       []FlightSegmentInput
}

// FlightSegmentInput — DTO одного беспосадочного сегмента перелёта.
// CabinClass — класс обслуживания как пришёл ("" — класс неизвестен);
// значение по умолчанию не подставляется, допустимость проверяет домен.
type FlightSegmentInput struct {
	DepartureAirportICAO string
	ArrivalAirportICAO   string
	DepartureAt          time.Time
	ArrivalAt            time.Time
	CabinClass           string
}

// BuildFlights разбирает цену каждого перелёта по справочнику валют и проверяет
// структурные инварианты через flight.New до любой записи в БД, поэтому
// некорректный перелёт даже не открывает транзакцию. Ошибка цены, валюты,
// комментария или класса сегмента возвращается как *apperror.FieldError
// (поле Price / Currency / ManagerComment / CabinClass), остальные доменные
// ошибки — как есть.
func BuildFlights(inputs []FlightInput) ([]flight.Flight, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	flights := make([]flight.Flight, 0, len(inputs))
	for _, in := range inputs {
		price, err := money.Parse(in.Price, in.Currency)
		if err != nil {
			return nil, priceFieldError(err)
		}
		f, err := flight.New(price, in.ManagerComment, toDomainSegments(in.Segments))
		if errors.Is(err, flight.ErrManagerCommentInvalid) {
			return nil, apperror.NewFieldError(managerCommentField, managerCommentRule, err)
		}
		if errors.Is(err, flight.ErrCabinClassInvalid) {
			return nil, apperror.NewFieldError(cabinClassField, cabinClassRule, err)
		}
		if err != nil {
			return nil, err
		}
		flights = append(flights, f)
	}
	return flights, nil
}

// priceFieldError сопоставляет ошибку разбора цены полю запроса.
func priceFieldError(err error) error {
	if errors.Is(err, money.ErrCurrencyUnsupported) {
		return apperror.NewFieldError(currencyField, currencyRule, err)
	}
	return apperror.NewFieldError(priceField, priceRule, err)
}

// toDomainSegments конвертирует DTO сегментов в доменный flight.Segment.
func toDomainSegments(in []FlightSegmentInput) []flight.Segment {
	segs := make([]flight.Segment, 0, len(in))
	for _, s := range in {
		segs = append(segs, flight.Segment{
			DepartureAirportICAO: s.DepartureAirportICAO,
			ArrivalAirportICAO:   s.ArrivalAirportICAO,
			DepartureAt:          s.DepartureAt,
			ArrivalAt:            s.ArrivalAt,
			CabinClass:           flight.CabinClass(s.CabinClass),
		})
	}
	return segs
}
