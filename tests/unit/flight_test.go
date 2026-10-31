package unit_test

import (
	"api/internal/domain/airport"
	"api/internal/domain/offer/flight"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlight_TotalDuration_NonstopSingleSegment(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	f := flight.Flight{Segments: []flight.Segment{
		{AirlineID: sampleAirlineID, DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", DepartureAt: base, ArrivalAt: base.Add(4 * time.Hour)},
	}}

	assert.Equal(t, 4*time.Hour, f.TotalDuration(nil))
	assert.Empty(t, f.Layovers(nil))
	assert.Equal(t, "UUEE", f.DepartureAirportICAO())
	assert.Equal(t, "LFPG", f.ArrivalAirportICAO())
}

func TestFlight_TotalDuration_OneLayover(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	f := flight.Flight{Segments: []flight.Segment{
		{AirlineID: sampleAirlineID, DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "UUDD", DepartureAt: base, ArrivalAt: base.Add(2 * time.Hour)},
		{AirlineID: sampleAirlineID, DepartureAirportICAO: "UUDD", ArrivalAirportICAO: "LFPG", DepartureAt: base.Add(4 * time.Hour), ArrivalAt: base.Add(7 * time.Hour)},
	}}

	assert.Equal(t, 7*time.Hour, f.TotalDuration(nil))
	require := assert.New(t)
	layovers := f.Layovers(nil)
	require.Len(layovers, 1)
	require.Equal("UUDD", layovers[0].AirportICAO)
	require.Equal(2*time.Hour, layovers[0].Duration)
	require.Equal("UUEE", f.DepartureAirportICAO())
	require.Equal("LFPG", f.ArrivalAirportICAO())
}

func TestFlight_TotalDuration_TwoLayovers(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	f := flight.Flight{Segments: []flight.Segment{
		{AirlineID: sampleAirlineID, DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "UUDD", DepartureAt: base, ArrivalAt: base.Add(2 * time.Hour)},
		{AirlineID: sampleAirlineID, DepartureAirportICAO: "UUDD", ArrivalAirportICAO: "EDDF", DepartureAt: base.Add(3 * time.Hour), ArrivalAt: base.Add(5 * time.Hour)},
		{AirlineID: sampleAirlineID, DepartureAirportICAO: "EDDF", ArrivalAirportICAO: "LFPG", DepartureAt: base.Add(6 * time.Hour), ArrivalAt: base.Add(7 * time.Hour)},
	}}

	assert.Equal(t, 7*time.Hour, f.TotalDuration(nil))
	layovers := f.Layovers(nil)
	if assert.Len(t, layovers, 2) {
		assert.Equal(t, "UUDD", layovers[0].AirportICAO)
		assert.Equal(t, time.Hour, layovers[0].Duration)
		assert.Equal(t, "EDDF", layovers[1].AirportICAO)
		assert.Equal(t, time.Hour, layovers[1].Duration)
	}
}

func TestFlightSegment_Duration(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	s := flight.Segment{DepartureAt: base, ArrivalAt: base.Add(90 * time.Minute)}
	assert.Equal(t, 90*time.Minute, s.Duration(nil))
}

func TestFlightSegment_CabinClassZeroValue_IsUnknown(t *testing.T) {
	var seg flight.Segment

	assert.Equal(t, flight.CabinClassUnknown, seg.CabinClass)
}

// testZones — таймзоны аэропортов из сценариев спеки; у NOTZ таймзона не указана,
// у BADZ — неизвестная.
func testZones() airport.Zones {
	return airport.NewZones([]airport.Airport{
		{ICAO: "USSS", City: airport.City{Timezone: "Asia/Yekaterinburg"}},
		{ICAO: "UUEE", City: airport.City{Timezone: "Europe/Moscow"}},
		{ICAO: "EDDB", City: airport.City{Timezone: "Europe/Berlin"}},
		{ICAO: "EDDF", City: airport.City{Timezone: "Europe/Berlin"}},
		{ICAO: "LTFM", City: airport.City{Timezone: "Europe/Istanbul"}},
		{ICAO: "NOTZ"},
		{ICAO: "BADZ", City: airport.City{Timezone: "Mars/Olympus"}},
	})
}

// wall — настенные часы сегмента, как они приходят из запроса и из БД.
func wall(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse("2006-01-02T15:04:05", value)
	require.NoError(t, err)
	return parsed
}

// localSegment — сегмент с настенными часами.
func localSegment(t *testing.T, dep, depAt, arr, arrAt string) flight.Segment {
	t.Helper()
	return flight.Segment{
		AirlineID: sampleAirlineID, DepartureAirportICAO: dep, ArrivalAirportICAO: arr,
		DepartureAt: wall(t, depAt), ArrivalAt: wall(t, arrAt),
	}
}

func TestSegmentDuration_LocalTimeAcrossZones_ComputedFromZones(t *testing.T) {
	cases := map[string]struct {
		seg  flight.Segment
		want time.Duration
	}{
		"Екатеринбург → Москва": {
			localSegment(t, "USSS", "2026-05-14T10:00:00", "UUEE", "2026-05-14T11:00:00"), 3 * time.Hour,
		},
		"Берлин → Стамбул летом": {
			localSegment(t, "EDDB", "2026-07-15T10:00:00", "LTFM", "2026-07-15T14:00:00"), 3 * time.Hour,
		},
		"Берлин → Стамбул зимой": {
			localSegment(t, "EDDB", "2026-01-15T10:00:00", "LTFM", "2026-01-15T14:00:00"), 2 * time.Hour,
		},
		"перевод часов в полёте": {
			localSegment(t, "EDDB", "2026-03-28T23:00:00", "EDDF", "2026-03-29T04:00:00"), 4 * time.Hour,
		},
		"прилёт раньше вылета по местным часам": {
			localSegment(t, "USSS", "2026-05-14T10:00:00", "UUEE", "2026-05-14T09:00:00"), time.Hour,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.seg.Duration(testZones()))
		})
	}
}

func TestSegmentDuration_NoOrUnknownZone_TreatedAsUTC(t *testing.T) {
	for _, icao := range []string{"NOTZ", "BADZ", "NONE"} {
		t.Run(icao, func(t *testing.T) {
			// Москва 10:00 — это 07:00 UTC.
			seg := localSegment(t, "UUEE", "2026-05-14T10:00:00", icao, "2026-05-14T09:00:00")

			assert.Equal(t, 2*time.Hour, seg.Duration(testZones()))
		})
	}
}

func TestSegmentDuration_NilZones_AllUTC(t *testing.T) {
	seg := localSegment(t, "UUEE", "2026-05-14T10:00:00", "USSS", "2026-05-14T11:00:00")

	assert.Equal(t, time.Hour, seg.Duration(nil))
}

func TestFlightLayovers_ClockChangeDuringLayover_RealDuration(t *testing.T) {
	f := flight.Flight{Segments: []flight.Segment{
		localSegment(t, "UUEE", "2026-03-28T22:00:00", "EDDF", "2026-03-29T01:30:00"),
		localSegment(t, "EDDF", "2026-03-29T03:30:00", "LTFM", "2026-03-29T08:00:00"),
	}}

	layovers := f.Layovers(testZones())

	require.Len(t, layovers, 1)
	assert.Equal(t, "EDDF", layovers[0].AirportICAO)
	assert.Equal(t, time.Hour, layovers[0].Duration)
}

func TestFlightTotalDuration_AcrossZones_IncludesLayover(t *testing.T) {
	f := flight.Flight{Segments: []flight.Segment{
		localSegment(t, "USSS", "2026-05-14T10:00:00", "UUEE", "2026-05-14T11:00:00"),
		localSegment(t, "UUEE", "2026-05-14T13:00:00", "LTFM", "2026-05-14T16:00:00"),
	}}

	// 05:00Z → 13:00Z.
	assert.Equal(t, 8*time.Hour, f.TotalDuration(testZones()))
}

// Неоднозначное (было дважды) и несуществующее (не было) местное время не падает,
// а длительность от чтения к чтению не меняется.
func TestSegmentDuration_AmbiguousOrMissingLocalTime_Stable(t *testing.T) {
	cases := map[string]flight.Segment{
		"неоднозначное":  localSegment(t, "EDDB", "2026-10-25T02:30:00", "LTFM", "2026-10-25T08:00:00"),
		"несуществующее": localSegment(t, "EDDB", "2026-03-29T02:30:00", "LTFM", "2026-03-29T08:00:00"),
	}
	for name, seg := range cases {
		t.Run(name, func(t *testing.T) {
			first := seg.Duration(testZones())

			assert.Positive(t, first)
			assert.Equal(t, first, seg.Duration(testZones()))
		})
	}
}

func TestAirportICAOs_RepeatsAndOrder(t *testing.T) {
	flights := []flight.Flight{
		{Segments: []flight.Segment{
			{DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG"},
			{DepartureAirportICAO: "LFPG", ArrivalAirportICAO: "KJFK"},
		}},
		{Segments: []flight.Segment{{DepartureAirportICAO: "KJFK", ArrivalAirportICAO: "UUEE"}}},
	}

	assert.Equal(t, []string{"UUEE", "LFPG", "KJFK"}, flight.AirportICAOs(flights))
}

func TestAirportICAOs_EmptySet_Empty(t *testing.T) {
	assert.Empty(t, flight.AirportICAOs(nil))
	assert.Empty(t, flight.AirportICAOs([]flight.Flight{{}}))
}
