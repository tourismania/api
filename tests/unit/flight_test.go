package unit_test

import (
	"testing"
	"time"

	"api/internal/domain/entity"

	"github.com/stretchr/testify/assert"
)

func TestFlight_TotalDuration_NonstopSingleSegment(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	f := entity.Flight{Segments: []entity.FlightSegment{
		{DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", DepartureAt: base, ArrivalAt: base.Add(4 * time.Hour)},
	}}

	assert.Equal(t, 4*time.Hour, f.TotalDuration())
	assert.Empty(t, f.Layovers())
	assert.Equal(t, "UUEE", f.DepartureAirportICAO())
	assert.Equal(t, "LFPG", f.ArrivalAirportICAO())
}

func TestFlight_TotalDuration_OneLayover(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	f := entity.Flight{Segments: []entity.FlightSegment{
		{DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "UUDD", DepartureAt: base, ArrivalAt: base.Add(2 * time.Hour)},
		{DepartureAirportICAO: "UUDD", ArrivalAirportICAO: "LFPG", DepartureAt: base.Add(4 * time.Hour), ArrivalAt: base.Add(7 * time.Hour)},
	}}

	assert.Equal(t, 7*time.Hour, f.TotalDuration())
	require := assert.New(t)
	layovers := f.Layovers()
	require.Len(layovers, 1)
	require.Equal("UUDD", layovers[0].AirportICAO)
	require.Equal(2*time.Hour, layovers[0].Duration)
	require.Equal("UUEE", f.DepartureAirportICAO())
	require.Equal("LFPG", f.ArrivalAirportICAO())
}

func TestFlight_TotalDuration_TwoLayovers(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	f := entity.Flight{Segments: []entity.FlightSegment{
		{DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "UUDD", DepartureAt: base, ArrivalAt: base.Add(2 * time.Hour)},
		{DepartureAirportICAO: "UUDD", ArrivalAirportICAO: "EDDF", DepartureAt: base.Add(3 * time.Hour), ArrivalAt: base.Add(5 * time.Hour)},
		{DepartureAirportICAO: "EDDF", ArrivalAirportICAO: "LFPG", DepartureAt: base.Add(6 * time.Hour), ArrivalAt: base.Add(7 * time.Hour)},
	}}

	assert.Equal(t, 7*time.Hour, f.TotalDuration())
	layovers := f.Layovers()
	if assert.Len(t, layovers, 2) {
		assert.Equal(t, "UUDD", layovers[0].AirportICAO)
		assert.Equal(t, time.Hour, layovers[0].Duration)
		assert.Equal(t, "EDDF", layovers[1].AirportICAO)
		assert.Equal(t, time.Hour, layovers[1].Duration)
	}
}

func TestFlightSegment_Duration(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	s := entity.FlightSegment{DepartureAt: base, ArrivalAt: base.Add(90 * time.Minute)}
	assert.Equal(t, 90*time.Minute, s.Duration())
}
