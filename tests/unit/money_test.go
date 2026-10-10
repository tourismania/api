package unit_test

import (
	"api/internal/domain/money"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookupCurrency_SupportedCodes_ReturnTwoDigitExponent(t *testing.T) {
	for _, code := range []string{"RUB", "USD", "EUR", "TRY"} {
		cur, err := money.LookupCurrency(code)
		require.NoError(t, err, code)
		assert.Equal(t, code, cur.Code)
		assert.Equal(t, 2, cur.Exponent, code)
	}
}

func TestLookupCurrency_UnknownOrLowercase_ReturnsErr(t *testing.T) {
	for _, code := range []string{"GBP", "rub", "", "RUBL"} {
		_, err := money.LookupCurrency(code)
		assert.ErrorIs(t, err, money.ErrCurrencyUnsupported, code)
	}
}

func TestParse_ValidAmounts_RoundTrip(t *testing.T) {
	cases := []struct {
		amount    string
		code      string
		wantMinor int64
		wantStr   string
	}{
		{"0", "RUB", 0, "0.00"},
		{"12345.5", "EUR", 1234550, "12345.50"},
		{"12345.50", "RUB", 1234550, "12345.50"},
		{"100", "USD", 10000, "100.00"},
		{"0.05", "TRY", 5, "0.05"},
		{"9999999999.99", "RUB", 999999999999, "9999999999.99"},
	}
	for _, tc := range cases {
		m, err := money.Parse(tc.amount, tc.code)
		require.NoError(t, err, tc.amount)
		assert.Equal(t, tc.wantMinor, m.AmountMinor, tc.amount)
		assert.Equal(t, tc.code, m.Currency.Code)
		assert.Equal(t, tc.wantStr, m.String(), tc.amount)
	}
}

func TestParse_SameValueDifferentNotation_Equal(t *testing.T) {
	a, err := money.Parse("100", "RUB")
	require.NoError(t, err)
	b, err := money.Parse("100.00", "RUB")
	require.NoError(t, err)
	assert.Equal(t, a, b)
}

func TestParse_InvalidAmounts_ReturnsErr(t *testing.T) {
	for _, amount := range []string{
		"-1", "+1", "1e3", "12,50", "abc", "", ".5", "1.", " 1", "1.2.3",
		"10.125",      // у USD только 2 знака после точки
		"12345678901", // 11 цифр в целой части
	} {
		_, err := money.Parse(amount, "USD")
		assert.ErrorIs(t, err, money.ErrAmountInvalid, amount)
	}
}

func TestParse_UnsupportedCurrency_ReturnsErr(t *testing.T) {
	_, err := money.Parse("100", "GBP")
	assert.ErrorIs(t, err, money.ErrCurrencyUnsupported)
}

// Валюты с 3 и 0 знаками после точки в справочнике пока нет — проверяем,
// что разбор и форматирование берут масштаб из Exponent, а не зашитые 2.
func TestFromDecimal_NonTwoDigitExponents_UseCurrencyScale(t *testing.T) {
	kwd := money.Currency{Code: "KWD", Exponent: 3}
	jpy := money.Currency{Code: "JPY", Exponent: 0}

	cases := []struct {
		amount    string
		cur       money.Currency
		wantMinor int64
		wantStr   string
	}{
		{"12.345", kwd, 12345, "12.345"},
		{"12.3", kwd, 12300, "12.300"},
		{"0.001", kwd, 1, "0.001"},
		{"500", jpy, 500, "500"},
	}
	for _, tc := range cases {
		m, err := money.FromDecimal(tc.amount, tc.cur)
		require.NoError(t, err, tc.amount)
		assert.Equal(t, tc.wantMinor, m.AmountMinor, tc.amount)
		assert.Equal(t, tc.wantStr, m.String(), tc.amount)
	}

	_, err := money.FromDecimal("12.3456", kwd)
	assert.ErrorIs(t, err, money.ErrAmountInvalid)
	_, err = money.FromDecimal("500.5", jpy)
	assert.ErrorIs(t, err, money.ErrAmountInvalid)
	_, err = money.FromDecimal("500.", jpy)
	assert.ErrorIs(t, err, money.ErrAmountInvalid)
}

func TestFromMinor_ValidAndInvalid(t *testing.T) {
	m, err := money.FromMinor(1500000, "RUB")
	require.NoError(t, err)
	assert.Equal(t, "15000.00", m.String())

	_, err = money.FromMinor(-1, "RUB")
	assert.ErrorIs(t, err, money.ErrAmountInvalid)

	_, err = money.FromMinor(100, "XXX")
	assert.ErrorIs(t, err, money.ErrCurrencyUnsupported)
}
