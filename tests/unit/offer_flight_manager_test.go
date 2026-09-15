package unit_test

import (
	"context"
	"testing"
	"time"

	"api/internal/domain/entity"
	"api/internal/domain/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleFlight(dep, arr string, base time.Time) entity.Flight {
	return entity.Flight{Segments: []entity.FlightSegment{
		{DepartureAirportICAO: dep, ArrivalAirportICAO: arr, DepartureAt: base, ArrivalAt: base.Add(2 * time.Hour)},
	}}
}

func TestOfferFlightManager_ReplaceForOffer_IdenticalContent_IsNoOp(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	existing := []entity.Flight{sampleFlight("UUEE", "LFPG", base)}
	flights := &mockOfferFlightRepo{findByOfferIDFlights: existing}
	mgr := service.NewOfferFlightManager(flights, &mockAirportRepo{})

	err := mgr.ReplaceForOffer(context.Background(), 1, []entity.Flight{sampleFlight("UUEE", "LFPG", base)})

	require.NoError(t, err)
	assert.False(t, flights.replaceCalled, "identical content must not touch the database")
}

func TestOfferFlightManager_ReplaceForOffer_DifferentContent_Replaces(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	existing := []entity.Flight{sampleFlight("UUEE", "LFPG", base)}
	flights := &mockOfferFlightRepo{findByOfferIDFlights: existing}
	mgr := service.NewOfferFlightManager(flights, &mockAirportRepo{})

	newSet := []entity.Flight{sampleFlight("UUEE", "EDDF", base)}
	err := mgr.ReplaceForOffer(context.Background(), 1, newSet)

	require.NoError(t, err)
	assert.True(t, flights.replaceCalled)
	assert.Equal(t, 1, flights.replacedOfferID)
	assert.Equal(t, newSet, flights.replacedFlights)
}

func TestOfferFlightManager_ReplaceForOffer_EmptyReplacesNonEmpty(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	existing := []entity.Flight{sampleFlight("UUEE", "LFPG", base)}
	flights := &mockOfferFlightRepo{findByOfferIDFlights: existing}
	mgr := service.NewOfferFlightManager(flights, &mockAirportRepo{})

	err := mgr.ReplaceForOffer(context.Background(), 1, nil)

	require.NoError(t, err)
	assert.True(t, flights.replaceCalled, "clearing flights must still hit the database")
}

func TestOfferFlightManager_ReplaceForOffer_BothEmpty_IsNoOp(t *testing.T) {
	flights := &mockOfferFlightRepo{}
	mgr := service.NewOfferFlightManager(flights, &mockAirportRepo{})

	err := mgr.ReplaceForOffer(context.Background(), 1, nil)

	require.NoError(t, err)
	assert.False(t, flights.replaceCalled)
}

func TestOfferFlightManager_ReplaceForOffer_DifferentOrder_IsNotEqual(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	existing := []entity.Flight{
		sampleFlight("UUEE", "LFPG", base),
		sampleFlight("EDDF", "LFPG", base.Add(10*time.Hour)),
	}
	flights := &mockOfferFlightRepo{findByOfferIDFlights: existing}
	mgr := service.NewOfferFlightManager(flights, &mockAirportRepo{})

	// Same two flights, reversed order — order is significant.
	reordered := []entity.Flight{existing[1], existing[0]}
	err := mgr.ReplaceForOffer(context.Background(), 1, reordered)

	require.NoError(t, err)
	assert.True(t, flights.replaceCalled)
}

func TestOfferFlightManager_ReplaceForOffer_UnknownAirport_ReturnsErr(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	flights := &mockOfferFlightRepo{}
	airports := &mockAirportRepo{findByICAOsAirports: []entity.Airport{{ICAO: "UUEE"}}}
	mgr := service.NewOfferFlightManager(flights, airports)

	err := mgr.ReplaceForOffer(context.Background(), 1, []entity.Flight{sampleFlight("UUEE", "LFPG", base)})

	assert.ErrorIs(t, err, service.ErrFlightAirportNotFound)
	assert.False(t, flights.replaceCalled, "must not write when an airport is missing")
}
