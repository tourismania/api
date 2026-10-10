package unit_test

import (
	"testing"
	"time"

	"api/internal/application/apperror"
	"api/internal/application/flightinput"
	"api/internal/domain/money"
	"api/internal/domain/offer/flight"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validSegments() []flightinput.Segment {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	return []flightinput.Segment{
		{DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", DepartureAt: base, ArrivalAt: base.Add(4 * time.Hour)},
	}
}

func TestFlightInputBuild_Valid_ParsesPrice(t *testing.T) {
	flights, err := flightinput.Build([]flightinput.Flight{{Price: "12345.5", Currency: "EUR", Segments: validSegments()}})

	require.NoError(t, err)
	require.Len(t, flights, 1)
	assert.Equal(t, mustMoney("12345.50", money.CodeEUR), flights[0].Price)
}

func TestFlightInputBuild_Empty_ReturnsNil(t *testing.T) {
	flights, err := flightinput.Build(nil)

	require.NoError(t, err)
	assert.Nil(t, flights)
}

func TestFlightInputBuild_InvalidPrice_ReturnsPriceFieldError(t *testing.T) {
	_, err := flightinput.Build([]flightinput.Flight{{Price: "10.125", Currency: "USD", Segments: validSegments()}})

	fe, ok := apperror.AsFieldError(err)
	require.True(t, ok)
	assert.Equal(t, "Price", fe.Field)
	assert.Equal(t, "money", fe.Rule)
	assert.ErrorIs(t, err, money.ErrAmountInvalid)
}

func TestFlightInputBuild_UnsupportedCurrency_ReturnsCurrencyFieldError(t *testing.T) {
	_, err := flightinput.Build([]flightinput.Flight{{Price: "100", Currency: "GBP", Segments: validSegments()}})

	fe, ok := apperror.AsFieldError(err)
	require.True(t, ok)
	assert.Equal(t, "Currency", fe.Field)
	assert.Equal(t, "currency", fe.Rule)
}

func TestFlightInputBuild_StructuralError_IsNotFieldError(t *testing.T) {
	_, err := flightinput.Build([]flightinput.Flight{{Price: "100", Currency: "RUB"}})

	assert.ErrorIs(t, err, flight.ErrSegmentsEmpty)
	_, ok := apperror.AsFieldError(err)
	assert.False(t, ok)
}
