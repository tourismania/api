package httpx

import (
	"errors"

	"api/internal/domain/money"

	"github.com/go-playground/validator/v10"
)

// Имена полей и теги, под которыми ошибки цены попадают в meta ответа
// 400 «validation failed».
const (
	priceFieldName    = "Price"
	currencyFieldName = "Currency"
	priceTag          = "money"
	currencyTag       = "currency"
)

// PricedInput — request-DTO с ценой (десятичная строка) и кодом валюты.
type PricedInput interface {
	PriceAndCurrency() (amount, currency string)
}

// PriceParser проверяет сумму в валюте и возвращает money.ErrAmountInvalid
// или money.ErrCurrencyUnsupported. Реализацию (разбор по справочнику
// валют) подставляет composition root: presentation-слой не вызывает
// доменную логику сам, а только сопоставляет её sentinel-ошибки полям.
type PriceParser func(amount, currency string) error

// PriceValidation возвращает struct-level проверку для DTO с ценой:
// валюта должна быть в справочнике, а сумма — допустимой для неё
// десятичной строкой. Ошибки попадают в meta как Currency и Price.
// Пустые значения пропускаются — их уже отклоняет тег required.
func PriceValidation(parse PriceParser) validator.StructLevelFunc {
	return func(sl validator.StructLevel) {
		in, ok := sl.Current().Interface().(PricedInput)
		if !ok {
			return
		}
		amount, currency := in.PriceAndCurrency()
		if amount == "" || currency == "" {
			return
		}
		switch err := parse(amount, currency); {
		case errors.Is(err, money.ErrCurrencyUnsupported):
			sl.ReportError(currency, currencyFieldName, currencyFieldName, currencyTag, "")
		case errors.Is(err, money.ErrAmountInvalid):
			sl.ReportError(amount, priceFieldName, priceFieldName, priceTag, "")
		}
	}
}
