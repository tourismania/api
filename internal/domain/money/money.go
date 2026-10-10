package money

import (
	"errors"
	"strconv"
	"strings"
)

// ErrAmountInvalid — сумма не неотрицательная десятичная запись в
// допустимых пределах: есть знак, экспонента или запятая, больше
// maxIntegerDigits цифр в целой части или больше знаков после точки, чем
// у валюты.
var ErrAmountInvalid = errors.New("amount must be a non-negative decimal with at most 10 integer digits and no more fractional digits than the currency allows")

// maxIntegerDigits — предел цифр в целой части суммы. При Exponent до 3
// максимум минорных единиц ~10^13 — в int64 (~9,2·10^18) с большим запасом.
const maxIntegerDigits = 10

// decimalBase — основание десятичной записи суммы.
const decimalBase = 10

// decimalPoint — разделитель целой и дробной части на проводе.
const decimalPoint = "."

// Money — неотрицательная сумма в валюте из справочника. Хранится целым
// числом минорных единиц (копеек, центов), а не float: так сложение и
// сравнение точные, а "100" и "100.00" дают одно и то же значение.
type Money struct {
	AmountMinor int64
	Currency    Currency
}

// Parse разбирает десятичную строку amount в валюте с кодом code из
// справочника: "12345.5" RUB → 1234550 минорных единиц.
func Parse(amount, code string) (Money, error) {
	cur, err := LookupCurrency(code)
	if err != nil {
		return Money{}, err
	}
	return FromDecimal(amount, cur)
}

// FromDecimal разбирает десятичную строку amount в валюте cur. Знаков
// после точки — не больше cur.Exponent; при Exponent 0 точки быть не
// должно.
func FromDecimal(amount string, cur Currency) (Money, error) {
	intPart, fracPart, hasPoint := strings.Cut(amount, decimalPoint)
	if !isDigits(intPart) || len(intPart) > maxIntegerDigits {
		return Money{}, ErrAmountInvalid
	}
	if hasPoint && (!isDigits(fracPart) || len(fracPart) > cur.Exponent) {
		return Money{}, ErrAmountInvalid
	}

	// Дополняем дробную часть нулями до Exponent: "12345" + "5" → "1234550".
	digits := intPart + fracPart + strings.Repeat("0", cur.Exponent-len(fracPart))
	minor, err := strconv.ParseInt(digits, decimalBase, 64)
	if err != nil {
		return Money{}, ErrAmountInvalid
	}
	return Money{AmountMinor: minor, Currency: cur}, nil
}

// FromMinor собирает сумму из минорных единиц и кода валюты — например,
// при чтении из хранилища.
func FromMinor(amountMinor int64, code string) (Money, error) {
	cur, err := LookupCurrency(code)
	if err != nil {
		return Money{}, err
	}
	if amountMinor < 0 {
		return Money{}, ErrAmountInvalid
	}
	return Money{AmountMinor: amountMinor, Currency: cur}, nil
}

// String форматирует сумму десятичной строкой ровно с Exponent знаками
// после точки: 1234550 RUB → "12345.50", 12345 при Exponent 3 → "12.345",
// 500 при Exponent 0 → "500".
func (m Money) String() string {
	digits := strconv.FormatInt(m.AmountMinor, decimalBase)
	exp := m.Currency.Exponent
	if exp == 0 {
		return digits
	}
	// Дополняем ведущими нулями, чтобы целая часть была не пустой: 5 → "0.05".
	if pad := exp + 1 - len(digits); pad > 0 {
		digits = strings.Repeat("0", pad) + digits
	}
	split := len(digits) - exp
	return digits[:split] + decimalPoint + digits[split:]
}

// isDigits — непустая строка только из ASCII-цифр.
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
