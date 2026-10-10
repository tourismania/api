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

// exponentCents — две цифры после точки (копейки, центы, куруши).
const exponentCents = 2

// currencies — системный справочник поддерживаемых валют. Таблица
// неизменяемая: пакет её только читает.
//
// Как добавить валюту, изменить число знаков после точки или удалить
// валюту — см. docs/currencies.md. Exponent валюты, которая уже
// использована в сохранённых ценах, нельзя менять без миграции данных:
// суммы хранятся в минорных единицах, и старые значения поменяют смысл.
var currencies = [...]Currency{
	{Code: CodeRUB, Exponent: exponentCents},
	{Code: CodeUSD, Exponent: exponentCents},
	{Code: CodeEUR, Exponent: exponentCents},
	{Code: CodeTRY, Exponent: exponentCents},
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
