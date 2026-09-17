package unit_test

import (
	"api/internal/domain/airport"
	"api/internal/domain/offer/flight"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleFlight(dep, arr string, base time.Time) flight.Flight {
	return flight.Flight{Segments: []flight.Segment{
		{DepartureAirportICAO: dep, ArrivalAirportICAO: arr, DepartureAt: base, ArrivalAt: base.Add(2 * time.Hour)},
	}}
}

func TestOfferFlightManager_ReplaceForOffer_IdenticalContent_IsNoOp(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	existing := []flight.Flight{sampleFlight("UUEE", "LFPG", base)}
	flights := &mockOfferFlightRepo{findByOfferIDFlights: existing}
	mgr := flight.NewManager(flights, &mockAirportRepo{})

	err := mgr.ReplaceForOffer(context.Background(), 1, []flight.Flight{sampleFlight("UUEE", "LFPG", base)})

	require.NoError(t, err)
	assert.False(t, flights.replaceCalled, "identical content must not touch the database")
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
