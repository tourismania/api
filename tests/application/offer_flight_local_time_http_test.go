package application_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"api/internal/domain/airport"
	"api/internal/domain/money"
	"api/internal/domain/offer/flight"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Сценарии требования «Api SHALL хранить местное время сегмента и определять
// момент по таймзоне аэропорта» и «Неверный формат времени» (дельта
// offers/flights, изменение connect-web-flights-to-api).

// zonedAirports — справочник сценариев спеки: город с таймзоной у всех, кроме NOTZ.
func zonedAirports() map[string]airport.Airport {
	zoned := func(icao, timezone string) airport.Airport {
		return airport.Airport{ICAO: icao, City: airport.City{Name: icao, Timezone: timezone}}
	}
	return map[string]airport.Airport{
		"USSS": zoned("USSS", "Asia/Yekaterinburg"),
		"UUEE": zoned("UUEE", "Europe/Moscow"),
		"EDDB": zoned("EDDB", "Europe/Berlin"),
		"EDDF": zoned("EDDF", "Europe/Berlin"),
		"LTFM": zoned("LTFM", "Europe/Istanbul"),
		"NOTZ": zoned("NOTZ", ""),
	}
}

func newZonedTestEnv(t *testing.T) priceTestEnv {
	t.Helper()
	return newPriceTestEnvWithAirports(t, stubAirportRepo{airports: zonedAirports()})
}

// timeSegment — сегмент с заданными аэропортами и местным временем.
func timeSegment(dep, depAt, arr, arrAt string) string {
	return fmt.Sprintf(
		`{"airline_id":1,"departure_airport_icao":%q,"arrival_airport_icao":%q,"departure_at":%q,"arrival_at":%q}`,
		dep, arr, depAt, arrAt,
	)
}

// timeFlight — перелёт из сегментов; цена не важна для этих сценариев.
func timeFlight(segments ...string) string {
	joined := segments[0]
	for _, s := range segments[1:] {
		joined += "," + s
	}
	return `{"price":"100","currency":"RUB","segments":[` + joined + `]}`
}

// cardFlight — единственный перелёт карточки как разобранный JSON.
func (e priceTestEnv) cardFlight(t *testing.T) map[string]any {
	t.Helper()
	rr := e.do(t, http.MethodGet, "/api/v1/offers/"+e.offerID.String(), "")
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var card struct {
		Flights []map[string]any `json:"flights"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &card))
	require.Len(t, card.Flights, 1)
	return card.Flights[0]
}

func (e priceTestEnv) createFlight(t *testing.T, flightJSON string) {
	t.Helper()
	rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(flightJSON))
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
}

// segmentAt — сегмент index перелёта из карточки.
func segmentAt(t *testing.T, f map[string]any, index int) map[string]any {
	t.Helper()
	segments, ok := f["segments"].([]any)
	require.True(t, ok)
	seg, ok := segments[index].(map[string]any)
	require.True(t, ok)
	return seg
}

func TestOffersHTTP_FlightAcrossZones_LocalTimeKept_DurationReal(t *testing.T) {
	e := newZonedTestEnv(t)

	e.createFlight(t, timeFlight(timeSegment("USSS", "2026-05-14T10:00:00", "UUEE", "2026-05-14T11:00:00")))

	f := e.cardFlight(t)
	seg := segmentAt(t, f, 0)
	assert.Equal(t, "2026-05-14T10:00:00", seg["departure_at"], "без Z и без сдвига")
	assert.Equal(t, "2026-05-14T11:00:00", seg["arrival_at"], "без Z и без сдвига")
	assert.EqualValues(t, 10800, seg["duration_seconds"])
	assert.EqualValues(t, 10800, f["total_duration_seconds"])
}

func TestOffersHTTP_FlightDuration_OffsetTakenOnFlightDate(t *testing.T) {
	cases := map[string]struct {
		date string
		want int
	}{
		"лето": {"2026-07-15", 10800},
		"зима": {"2026-01-15", 7200},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := newZonedTestEnv(t)

			e.createFlight(t, timeFlight(timeSegment("EDDB", tc.date+"T10:00:00", "LTFM", tc.date+"T14:00:00")))

			assert.EqualValues(t, tc.want, segmentAt(t, e.cardFlight(t), 0)["duration_seconds"])
		})
	}
}

func TestOffersHTTP_FlightDuration_ClockChangeInFlight(t *testing.T) {
	e := newZonedTestEnv(t)

	e.createFlight(t, timeFlight(timeSegment("EDDB", "2026-03-28T23:00:00", "EDDF", "2026-03-29T04:00:00")))

	assert.EqualValues(t, 14400, segmentAt(t, e.cardFlight(t), 0)["duration_seconds"])
}

func TestOffersHTTP_FlightLayover_ClockChangeDuringLayover(t *testing.T) {
	e := newZonedTestEnv(t)

	e.createFlight(t, timeFlight(
		timeSegment("UUEE", "2026-03-28T22:00:00", "EDDF", "2026-03-29T01:30:00"),
		timeSegment("EDDF", "2026-03-29T03:30:00", "LTFM", "2026-03-29T08:00:00"),
	))

	layovers, ok := e.cardFlight(t)["layovers"].([]any)
	require.True(t, ok)
	require.Len(t, layovers, 1)
	layover, ok := layovers[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "EDDF", layover["airport_icao"])
	assert.EqualValues(t, 3600, layover["duration_seconds"])
}

func TestOffersHTTP_Flight_ArrivalEarlierByClocksOnly_Saved(t *testing.T) {
	e := newZonedTestEnv(t)

	e.createFlight(t, timeFlight(timeSegment("USSS", "2026-05-14T10:00:00", "UUEE", "2026-05-14T09:00:00")))

	assert.EqualValues(t, 3600, segmentAt(t, e.cardFlight(t), 0)["duration_seconds"])
}

func TestOffersHTTP_Flight_ArrivalLaterByClocksOnly_Returns400(t *testing.T) {
	e := newZonedTestEnv(t)

	rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(timeFlight(
		timeSegment("UUEE", "2026-05-14T10:00:00", "USSS", "2026-05-14T11:00:00"),
	)))

	require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
	assert.Contains(t, decodeError(t, rr).Error, "flight segment arrival must be strictly after departure")
	assert.False(t, e.flights.replaceCalled)
}

func TestOffersHTTP_FlightLayover_NonPositiveByRealMoments_Returns400(t *testing.T) {
	e := newZonedTestEnv(t)

	// Прилёт в Москву и вылет оттуда в одну минуту — ноль на пересадке.
	rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(timeFlight(
		timeSegment("USSS", "2026-05-14T10:00:00", "UUEE", "2026-05-14T11:00:00"),
		timeSegment("UUEE", "2026-05-14T11:00:00", "LTFM", "2026-05-14T15:00:00"),
	)))

	require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
	assert.Contains(t, decodeError(t, rr).Error, "flight layover between segments must be strictly positive")
}

func TestOffersHTTP_Flight_AmbiguousOrMissingLocalTime_AcceptedAndStable(t *testing.T) {
	for name, depAt := range map[string]string{
		"было дважды": "2026-10-25T02:30:00",
		"не было":     "2026-03-29T02:30:00",
	} {
		t.Run(name, func(t *testing.T) {
			e := newZonedTestEnv(t)
			arrAt := depAt[:len("2026-10-25")] + "T09:00:00"

			e.createFlight(t, timeFlight(timeSegment("EDDB", depAt, "LTFM", arrAt)))

			first := segmentAt(t, e.cardFlight(t), 0)
			assert.Equal(t, depAt, first["departure_at"])
			assert.Equal(t, first["duration_seconds"], segmentAt(t, e.cardFlight(t), 0)["duration_seconds"])
		})
	}
}

func TestOffersHTTP_Flight_AirportWithoutTimezone_TreatedAsUTC(t *testing.T) {
	e := newZonedTestEnv(t)

	// Москва 10:00 — 07:00 UTC, а время NOTZ считается UTC.
	e.createFlight(t, timeFlight(timeSegment("UUEE", "2026-05-14T10:00:00", "NOTZ", "2026-05-14T09:00:00")))

	assert.EqualValues(t, 7200, segmentAt(t, e.cardFlight(t), 0)["duration_seconds"])
}

func TestOffersHTTP_Flight_TimezoneUpdatedInReference_LocalTimeKept_DurationRecomputed(t *testing.T) {
	airports := zonedAirports()
	e := newPriceTestEnvWithAirports(t, stubAirportRepo{airports: airports})
	e.createFlight(t, timeFlight(timeSegment("USSS", "2026-05-14T10:00:00", "UUEE", "2026-05-14T11:00:00")))

	updated := airports["USSS"]
	updated.City.Timezone = "Europe/Moscow"
	airports["USSS"] = updated

	seg := segmentAt(t, e.cardFlight(t), 0)
	assert.Equal(t, "2026-05-14T10:00:00", seg["departure_at"])
	assert.EqualValues(t, 3600, seg["duration_seconds"])
}

// Сегменты, сохранённые, когда время принималось в RFC3339: их часы считаются местными.
func TestOffersHTTP_Flight_SavedBeforeLocalTime_ClocksAreLocal(t *testing.T) {
	e := newZonedTestEnv(t)
	price, err := money.Parse("100", money.CodeRUB)
	require.NoError(t, err)
	dep := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	e.flights.stored = []flight.Flight{{ID: 1, Price: price, Segments: []flight.Segment{{
		AirlineID: 1, DepartureAirportICAO: "USSS", ArrivalAirportICAO: "UUEE", DepartureAt: dep, ArrivalAt: dep.Add(time.Hour),
	}}}}

	seg := segmentAt(t, e.cardFlight(t), 0)

	assert.Equal(t, "2026-08-01T10:00:00", seg["departure_at"])
	assert.EqualValues(t, 10800, seg["duration_seconds"])
}

func TestOffersHTTP_CreateFlight_NotLocalTimeFormat_Returns400(t *testing.T) {
	for name, value := range map[string]string{
		"Z":              `"2026-05-14T10:00:00Z"`,
		"смещение":       `"2026-05-14T10:00:00+05:00"`,
		"без секунд":     `"2026-05-14T10:00"`,
		"пробел":         `"2026-05-14 10:00:00"`,
		"дробные":        `"2026-05-14T10:00:00.5"`,
		"такой даты нет": `"2026-02-30T10:00:00"`,
		"мусор":          `"завтра"`,
		"число":          `1747216800`,
	} {
		t.Run(name, func(t *testing.T) {
			e := newZonedTestEnv(t)

			rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(`{"price":"100","currency":"RUB","segments":[
				{"airline_id":1,"departure_airport_icao":"UUEE","arrival_airport_icao":"LTFM","departure_at":`+value+`,"arrival_at":"2026-05-14T14:00:00"}
			]}`))

			require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
			msg := decodeError(t, rr).Error
			assert.Contains(t, msg, "invalid date/time value")
			assert.Contains(t, msg, "(expected local YYYY-MM-DDTHH:MM:SS without timezone)")
			assert.False(t, e.offers.storeCalled, "оффер не создаётся")
			assert.False(t, e.flights.replaceCalled, "перелёты не создаются")
		})
	}
}

func TestOffersHTTP_PatchFlight_WithOffset_Returns400_NothingChanged(t *testing.T) {
	e := newZonedTestEnv(t)

	rr := e.do(t, http.MethodPatch, "/api/v1/offers/"+e.offerID.String(), patchBody(timeFlight(
		timeSegment("UUEE", "2026-05-14T10:00:00+03:00", "LTFM", "2026-05-14T14:00:00"),
	)))

	require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
	assert.Contains(t, decodeError(t, rr).Error, "expected local YYYY-MM-DDTHH:MM:SS without timezone")
	assert.False(t, e.offers.updateCalled)
	assert.False(t, e.flights.replaceCalled)
}

func TestOffersHTTP_CreateFlight_TimeMissingOrNull_ReturnsRequired(t *testing.T) {
	for name, field := range map[string]string{
		"нет поля": ``,
		"null":     `"departure_at":null,`,
	} {
		t.Run(name, func(t *testing.T) {
			e := newZonedTestEnv(t)

			rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(`{"price":"100","currency":"RUB","segments":[
				{"airline_id":1,"departure_airport_icao":"UUEE","arrival_airport_icao":"LTFM",`+field+`"arrival_at":"2026-05-14T14:00:00"}
			]}`))

			require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
			assert.Equal(t, map[string]any{"DepartureAt": "required"}, decodeError(t, rr).Meta)
		})
	}
}

func TestOffersHTTP_PatchFlight_SameLocalTime_KeepsFlightIDs(t *testing.T) {
	e := newZonedTestEnv(t)
	body := timeFlight(timeSegment("USSS", "2026-05-14T10:00:00", "UUEE", "2026-05-14T11:00:00"))
	e.createFlight(t, body)
	id := e.cardFlight(t)["id"]
	e.flights.replaceCalled = false

	rr := e.do(t, http.MethodPatch, "/api/v1/offers/"+e.offerID.String(), patchBody(body))

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.False(t, e.flights.replaceCalled, "тот же набор не заменяется")
	assert.Equal(t, id, e.cardFlight(t)["id"])
}
