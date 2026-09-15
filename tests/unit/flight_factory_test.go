package unit_test

import (
	"testing"
	"time"

	"api/internal/domain/entity"
	"api/internal/domain/factory"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFlight_EmptySegments_ReturnsErr(t *testing.T) {
	_, err := factory.NewFlight(nil)
	assert.ErrorIs(t, err, factory.ErrFlightSegmentsEmpty)
}

func TestNewFlight_ArrivalNotAfterDeparture_ReturnsErr(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	_, err := factory.NewFlight([]entity.FlightSegment{
		{DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", DepartureAt: base, ArrivalAt: base},
	})
	assert.ErrorIs(t, err, factory.ErrFlightSegmentChronologyInvalid)
}

func TestNewFlight_ArrivalBeforeDeparture_ReturnsErr(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	_, err := factory.NewFlight([]entity.FlightSegment{
		{DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", DepartureAt: base, ArrivalAt: base.Add(-time.Hour)},
	})
	assert.ErrorIs(t, err, factory.ErrFlightSegmentChronologyInvalid)
}

func TestNewFlight_DiscontinuousRoute_ReturnsErr(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	_, err := factory.NewFlight([]entity.FlightSegment{
		{DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "UUDD", DepartureAt: base, ArrivalAt: base.Add(2 * time.Hour)},
		// Departs from EDDF, not UUDD (the previous segment's arrival) — the route breaks.
		{DepartureAirportICAO: "EDDF", ArrivalAirportICAO: "LFPG", DepartureAt: base.Add(4 * time.Hour), ArrivalAt: base.Add(6 * time.Hour)},
	})
	assert.ErrorIs(t, err, factory.ErrFlightSegmentDiscontinuous)
}

func TestNewFlight_ZeroLayover_ReturnsErr(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	_, err := factory.NewFlight([]entity.FlightSegment{
		{DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "UUDD", DepartureAt: base, ArrivalAt: base.Add(2 * time.Hour)},
		// Departs exactly when the previous segment arrives: zero layover.
		{DepartureAirportICAO: "UUDD", ArrivalAirportICAO: "LFPG", DepartureAt: base.Add(2 * time.Hour), ArrivalAt: base.Add(4 * time.Hour)},
	})
	assert.ErrorIs(t, err, factory.ErrFlightLayoverNonPositive)
}

func TestNewFlight_NegativeLayover_ReturnsErr(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	_, err := factory.NewFlight([]entity.FlightSegment{
		{DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "UUDD", DepartureAt: base, ArrivalAt: base.Add(2 * time.Hour)},
		// Departs before the previous segment even arrives.
		{DepartureAirportICAO: "UUDD", ArrivalAirportICAO: "LFPG", DepartureAt: base.Add(time.Hour), ArrivalAt: base.Add(4 * time.Hour)},
	})
	assert.ErrorIs(t, err, factory.ErrFlightLayoverNonPositive)
}

func TestNewFlight_ValidNonstop_ReturnsFlight(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	f, err := factory.NewFlight([]entity.FlightSegment{
		{DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", DepartureAt: base, ArrivalAt: base.Add(4 * time.Hour)},
	})
	require.NoError(t, err)
	assert.Len(t, f.Segments, 1)
}

func TestNewFlight_ValidWithLayovers_ReturnsFlight(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	f, err := factory.NewFlight([]entity.FlightSegment{
		{DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "UUDD", DepartureAt: base, ArrivalAt: base.Add(2 * time.Hour)},
		{DepartureAirportICAO: "UUDD", ArrivalAirportICAO: "EDDF", DepartureAt: base.Add(3 * time.Hour), ArrivalAt: base.Add(5 * time.Hour)},
		{DepartureAirportICAO: "EDDF", ArrivalAirportICAO: "LFPG", DepartureAt: base.Add(6 * time.Hour), ArrivalAt: base.Add(7 * time.Hour)},
	})
	require.NoError(t, err)
	assert.Len(t, f.Segments, 3)
}
