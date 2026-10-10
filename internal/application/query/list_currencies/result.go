package listcurrencies

// CurrencyResult — одна валюта справочника.
type CurrencyResult struct {
	Code     string
	Exponent int
}

// Result — все валюты справочника в его порядке.
type Result struct {
	Currencies []CurrencyResult
}
