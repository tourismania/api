package unit_test

import (
	"context"
	"testing"

	listcurrencies "api/internal/application/query/list_currencies"
	"api/internal/domain/money"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCurrencies_ReturnsDirectoryInOrder(t *testing.T) {
	got := money.Currencies()

	assert.Equal(t, []money.Currency{
		{Code: "RUB", Exponent: 2},
		{Code: "USD", Exponent: 2},
		{Code: "EUR", Exponent: 2},
		{Code: "TRY", Exponent: 2},
	}, got)
}

func TestCurrencies_ReturnsCopy_DirectoryCannotBeChanged(t *testing.T) {
	got := money.Currencies()
	got[0].Exponent = 3
	got[0].Code = "XXX"

	cur, err := money.LookupCurrency("RUB")
	require.NoError(t, err)
	assert.Equal(t, 2, cur.Exponent)
	assert.Equal(t, "RUB", money.Currencies()[0].Code)
}

func TestListCurrencies_ReturnsCodeAndExponent(t *testing.T) {
	res, err := listcurrencies.NewHandler().Handle(context.Background(), listcurrencies.Query{})

	require.NoError(t, err)
	require.Len(t, res.Currencies, 4)
	assert.Equal(t, listcurrencies.CurrencyResult{Code: "RUB", Exponent: 2}, res.Currencies[0])
	assert.Equal(t, listcurrencies.CurrencyResult{Code: "TRY", Exponent: 2}, res.Currencies[3])
}
