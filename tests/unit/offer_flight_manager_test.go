package unit_test

import (
	"api/internal/domain/airport"
	"api/internal/domain/money"
	"api/internal/domain/offer/flight"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleFlight(dep, arr string, base time.Time) flight.Flight {
	return flight.Flight{Price: samplePrice(), Segments: []flight.Segment{
		{DepartureAirportICAO: dep, ArrivalAirportICAO: arr, DepartureAt: base, ArrivalAt: base.Add(2 * time.Hour)},
	}}
}

// withPrice возвращает копию перелёта с другой ценой и ID.
func withPrice(f flight.Flight, id int, price money.Money) flight.Flight {
	f.ID = id
	f.Price = price
	return f
}

func TestOfferFlightManager_ReplaceForOffer_IdenticalContent_IsNoOp(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	existing := []flight.Flight{sampleFlight("UUEE", "LFPG", base)}
	flights := &mockOfferFlightRepo{findByOfferIDFlights: existing}
	mgr := flight.NewManager(flights, &mockAirportRepo{})

	err := mgr.ReplaceForOffer(context.Background(), 1, []flight.Flight{sampleFlight("UUEE", "LFPG", base)})

	require.NoError(t, err)
	assert.False(t, flights.replaceCalled, "identical content must not touch the database")
	assert.False(t, flights.updatePricesCalled, "identical prices must not touch the database")
}

func TestOfferFlightManager_ReplaceForOffer_SamePriceDifferentNotation_IsNoOp(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	stored := withPrice(sampleFlight("UUEE", "LFPG", base), 7, mustMoney("100.00", money.CodeRUB))
	flights := &mockOfferFlightRepo{findByOfferIDFlights: []flight.Flight{stored}}
	mgr := flight.NewManager(flights, &mockAirportRepo{})

	incoming := withPrice(sampleFlight("UUEE", "LFPG", base), 0, mustMoney("100", money.CodeRUB))
	err := mgr.ReplaceForOffer(context.Background(), 1, []flight.Flight{incoming})

	require.NoError(t, err)
	assert.False(t, flights.replaceCalled)
	assert.False(t, flights.updatePricesCalled)
}

func TestOfferFlightManager_ReplaceForOffer_OnlyPriceChanged_UpdatesPricesKeepsIDs(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	existing := []flight.Flight{
		withPrice(sampleFlight("UUEE", "LFPG", base), 7, mustMoney("100", money.CodeRUB)),
		withPrice(sampleFlight("LFPG", "UUEE", base.Add(48*time.Hour)), 8, mustMoney("200", money.CodeRUB)),
	}
	flights := &mockOfferFlightRepo{findByOfferIDFlights: existing}
	mgr := flight.NewManager(flights, &mockAirportRepo{})

	// Маршрут тот же, у второго перелёта другая цена.
	incoming := []flight.Flight{
		withPrice(sampleFlight("UUEE", "LFPG", base), 0, mustMoney("100", money.CodeRUB)),
		withPrice(sampleFlight("LFPG", "UUEE", base.Add(48*time.Hour)), 0, mustMoney("250.50", money.CodeRUB)),
	}
	err := mgr.ReplaceForOffer(context.Background(), 1, incoming)

	require.NoError(t, err)
	assert.False(t, flights.replaceCalled, "price-only change must not replace the set")
	require.True(t, flights.updatePricesCalled)
	assert.Equal(t, 1, flights.updatePricesOfferID)
	assert.Equal(t, []flight.PriceUpdate{{FlightID: 8, Price: mustMoney("250.50", money.CodeRUB)}}, flights.updatedPrices)
}

func TestOfferFlightManager_ReplaceForOffer_OnlyCurrencyChanged_UpdatesPrices(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	existing := []flight.Flight{withPrice(sampleFlight("UUEE", "LFPG", base), 7, mustMoney("100", money.CodeRUB))}
	flights := &mockOfferFlightRepo{findByOfferIDFlights: existing}
	mgr := flight.NewManager(flights, &mockAirportRepo{})

	incoming := []flight.Flight{withPrice(sampleFlight("UUEE", "LFPG", base), 0, mustMoney("100", money.CodeEUR))}
	err := mgr.ReplaceForOffer(context.Background(), 1, incoming)

	require.NoError(t, err)
	assert.False(t, flights.replaceCalled)
	assert.Equal(t, []flight.PriceUpdate{{FlightID: 7, Price: mustMoney("100", money.CodeEUR)}}, flights.updatedPrices)
}

func TestOfferFlightManager_ReplaceForOffer_RouteAndPriceChanged_Replaces(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	existing := []flight.Flight{withPrice(sampleFlight("UUEE", "LFPG", base), 7, mustMoney("100", money.CodeRUB))}
	flights := &mockOfferFlightRepo{findByOfferIDFlights: existing}
	mgr := flight.NewManager(flights, &mockAirportRepo{})

	incoming := []flight.Flight{withPrice(sampleFlight("UUEE", "EDDF", base), 0, mustMoney("300", money.CodeRUB))}
	err := mgr.ReplaceForOffer(context.Background(), 1, incoming)

	require.NoError(t, err)
	assert.True(t, flights.replaceCalled)
	assert.False(t, flights.updatePricesCalled)
}

func TestOfferFlightManager_ReplaceForOffer_DifferentContent_Replaces(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	existing := []flight.Flight{sampleFlight("UUEE", "LFPG", base)}
	flights := &mockOfferFlightRepo{findByOfferIDFlights: existing}
	mgr := flight.NewManager(flights, &mockAirportRepo{})

	newSet := []flight.Flight{sampleFlight("UUEE", "EDDF", base)}
	err := mgr.ReplaceForOffer(context.Background(), 1, newSet)

	require.NoError(t, err)
	assert.True(t, flights.replaceCalled)
	assert.Equal(t, 1, flights.replacedOfferID)
	assert.Equal(t, newSet, flights.replacedFlights)
}

func TestOfferFlightManager_ReplaceForOffer_EmptyReplacesNonEmpty(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	existing := []flight.Flight{sampleFlight("UUEE", "LFPG", base)}
	flights := &mockOfferFlightRepo{findByOfferIDFlights: existing}
	mgr := flight.NewManager(flights, &mockAirportRepo{})

	err := mgr.ReplaceForOffer(context.Background(), 1, nil)

	require.NoError(t, err)
	assert.True(t, flights.replaceCalled, "clearing flights must still hit the database")
}

func TestOfferFlightManager_ReplaceForOffer_BothEmpty_IsNoOp(t *testing.T) {
	flights := &mockOfferFlightRepo{}
	mgr := flight.NewManager(flights, &mockAirportRepo{})

	err := mgr.ReplaceForOffer(context.Background(), 1, nil)

	require.NoError(t, err)
	assert.False(t, flights.replaceCalled)
}

func TestOfferFlightManager_ReplaceForOffer_DifferentOrder_IsNotEqual(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	existing := []flight.Flight{
		sampleFlight("UUEE", "LFPG", base),
		sampleFlight("EDDF", "LFPG", base.Add(10*time.Hour)),
	}
	flights := &mockOfferFlightRepo{findByOfferIDFlights: existing}
	mgr := flight.NewManager(flights, &mockAirportRepo{})

	// Same two flights, reversed order — order is significant.
	reordered := []flight.Flight{existing[1], existing[0]}
	err := mgr.ReplaceForOffer(context.Background(), 1, reordered)

	require.NoError(t, err)
	assert.True(t, flights.replaceCalled)
}

func TestOfferFlightManager_ReplaceForOffer_UnknownAirport_ReturnsErr(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	flights := &mockOfferFlightRepo{}
	airports := &mockAirportRepo{findByICAOsAirports: []airport.Airport{{ICAO: "UUEE"}}}
	mgr := flight.NewManager(flights, airports)

	err := mgr.ReplaceForOffer(context.Background(), 1, []flight.Flight{sampleFlight("UUEE", "LFPG", base)})

	assert.ErrorIs(t, err, flight.ErrAirportNotFound)
	assert.False(t, flights.replaceCalled, "must not write when an airport is missing")
}
