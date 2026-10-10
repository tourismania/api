// Package money — системный справочник валют и денежная сумма. Общий для
// всех позиций оффера (перелёты, будущие отели и т. д.), поэтому лежит
// отдельным пакетом, а не внутри domain/offer/flight. Пакет ни от кого
// не зависит.
package money

import "errors"

// ErrCurrencyUnsupported — валюты с таким кодом нет в справочнике.
var ErrCurrencyUnsupported = errors.New("currency is not supported")

// Currency — валюта из справочника: код ISO 4217 и число знаков после
// точки (minor unit по ISO 4217), то есть во сколько раз минорная единица
// меньше основной: 2 — копейки/центы, 3 — филсы, 0 — дробной части нет.
type Currency struct {
	Code     string
	Exponent int
}

// Коды валют справочника.
const (
	CodeRUB = "RUB"
	CodeUSD = "USD"
	CodeEUR = "EUR"
	CodeTRY = "TRY"
)

// exponentTwoDecimals — две цифры после точки. Константа названа по числу
// знаков, а не по названию минорной единицы: у RUB это копейки, у USD и
// EUR — центы, у TRY — куруши.
const exponentTwoDecimals = 2

// currencies — системный справочник поддерживаемых валют. Таблица
// неизменяемая: пакет её только читает.
//
// Как добавить валюту, изменить число знаков после точки или удалить
// валюту — см. docs/currencies.md. Exponent валюты, которая уже
// использована в сохранённых ценах, нельзя менять без миграции данных:
// суммы хранятся в минорных единицах, и старые значения поменяют смысл.
var currencies = [...]Currency{
	{Code: CodeRUB, Exponent: exponentTwoDecimals},
	{Code: CodeUSD, Exponent: exponentTwoDecimals},
	{Code: CodeEUR, Exponent: exponentTwoDecimals},
	{Code: CodeTRY, Exponent: exponentTwoDecimals},
}

// Currencies возвращает все валюты справочника в его порядке. Возвращается
// копия: изменить справочник через результат нельзя.
func Currencies() []Currency {
	out := make([]Currency, len(currencies))
	copy(out, currencies[:])
	return out
}

// LookupCurrency ищет валюту в справочнике по коду ISO 4217. Сравнение
// точное, с учётом регистра: "rub" — не RUB.
func LookupCurrency(code string) (Currency, error) {
	for _, c := range currencies {
		if c.Code == code {
			return c, nil
		}
	}
	return Currency{}, ErrCurrencyUnsupported
}
