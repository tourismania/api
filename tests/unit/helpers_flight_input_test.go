package unit_test

import (
	"testing"
	"time"

	"api/internal/application/apperror"
	"api/internal/application/helpers"
	"api/internal/domain/money"
	"api/internal/domain/offer/flight"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validSegments() []helpers.FlightSegmentInput {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	return []helpers.FlightSegmentInput{
		{AirlineID: sampleAirlineID, DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", DepartureAt: base, ArrivalAt: base.Add(4 * time.Hour)},
	}
}

func TestBuildFlights_Valid_ParsesPrice(t *testing.T) {
	flights, err := helpers.BuildFlights([]helpers.FlightInput{{Price: "12345.5", Currency: "EUR", Segments: validSegments()}})

	require.NoError(t, err)
	require.Len(t, flights, 1)
	assert.Equal(t, mustMoney("12345.50", money.CodeEUR), flights[0].Price)
}

func TestBuildFlights_Empty_ReturnsNil(t *testing.T) {
	flights, err := helpers.BuildFlights(nil)

	require.NoError(t, err)
	assert.Nil(t, flights)
}

func TestBuildFlights_InvalidPrice_ReturnsPriceFieldError(t *testing.T) {
	_, err := helpers.BuildFlights([]helpers.FlightInput{{Price: "10.125", Currency: "USD", Segments: validSegments()}})

	fe, ok := apperror.AsFieldError(err)
	require.True(t, ok)
	assert.Equal(t, "Price", fe.Field)
	assert.Equal(t, "money", fe.Rule)
	assert.ErrorIs(t, err, money.ErrAmountInvalid)
}

func TestBuildFlights_UnsupportedCurrency_ReturnsCurrencyFieldError(t *testing.T) {
	_, err := helpers.BuildFlights([]helpers.FlightInput{{Price: "100", Currency: "GBP", Segments: validSegments()}})

	fe, ok := apperror.AsFieldError(err)
	require.True(t, ok)
	assert.Equal(t, "Currency", fe.Field)
	assert.Equal(t, "currency", fe.Rule)
}

func TestBuildFlights_ManagerComment_PassedAsIs(t *testing.T) {
	comment := " Багаж оплачивается отдельно\nRock'n'roll \\ \"VIP\" "
	flights, err := helpers.BuildFlights([]helpers.FlightInput{
		{Price: "100", Currency: "RUB", ManagerComment: comment, Segments: validSegments()},
		{Price: "100", Currency: "RUB", Segments: validSegments()},
	})

	require.NoError(t, err)
	require.Len(t, flights, 2)
	assert.Equal(t, comment, flights[0].ManagerComment)
	assert.Empty(t, flights[1].ManagerComment)
}

func TestBuildFlights_ManagerCommentControlChar_ReturnsManagerCommentFieldError(t *testing.T) {
	for _, comment := range []string{"\u0000", "\u001b[0m", "\u007f"} {
		_, err := helpers.BuildFlights([]helpers.FlightInput{
			{Price: "100", Currency: "RUB", ManagerComment: comment, Segments: validSegments()},
		})

		fe, ok := apperror.AsFieldError(err)
		require.True(t, ok, "%q", comment)
		assert.Equal(t, "ManagerComment", fe.Field)
		assert.Equal(t, "text", fe.Rule)
		assert.ErrorIs(t, err, apperror.ErrValidation)
		assert.ErrorIs(t, err, flight.ErrManagerCommentInvalid)
	}
}

func TestBuildFlights_StructuralError_IsNotFieldError(t *testing.T) {
	_, err := helpers.BuildFlights([]helpers.FlightInput{{Price: "100", Currency: "RUB"}})

	assert.ErrorIs(t, err, flight.ErrSegmentsEmpty)
	_, ok := apperror.AsFieldError(err)
	assert.False(t, ok)
}

func TestBuildFlights_CabinClass_PassedAsIsWithoutDefault(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	flights, err := helpers.BuildFlights([]helpers.FlightInput{{Price: "100", Currency: "RUB", Segments: []helpers.FlightSegmentInput{
		{AirlineID: sampleAirlineID, DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", DepartureAt: base, ArrivalAt: base.Add(4 * time.Hour), CabinClass: "comfort_plus"},
		{AirlineID: sampleAirlineID, DepartureAirportICAO: "LFPG", ArrivalAirportICAO: "KJFK", DepartureAt: base.Add(6 * time.Hour), ArrivalAt: base.Add(14 * time.Hour)},
	}}})

	require.NoError(t, err)
	require.Len(t, flights, 1)
	assert.Equal(t, flight.CabinClassComfortPlus, flights[0].Segments[0].CabinClass)
	assert.Equal(t, flight.CabinClassUnknown, flights[0].Segments[1].CabinClass, "empty class must stay empty")
}

func TestBuildFlights_CabinClassInvalid_ReturnsCabinClassFieldError(t *testing.T) {
	for _, class := range []string{"first", "premium_economy", "comfort+", "Business", " economy"} {
		segs := validSegments()
		segs[0].CabinClass = class
		_, err := helpers.BuildFlights([]helpers.FlightInput{{Price: "100", Currency: "RUB", Segments: segs}})

		fe, ok := apperror.AsFieldError(err)
		require.True(t, ok, "%q", class)
		assert.Equal(t, "CabinClass", fe.Field)
		assert.Equal(t, "oneof", fe.Rule)
		assert.ErrorIs(t, err, apperror.ErrValidation)
		assert.ErrorIs(t, err, flight.ErrCabinClassInvalid)
	}
}
