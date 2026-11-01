package application_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	getpublishedoffer "api/internal/application/query/get_published_offer"
	"api/internal/domain/airport"
	"api/internal/domain/offer"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Сценарии объектов аэропортов сегмента в карточке (дельта offers/flights,
// изменение connect-web-flights-to-api).

const (
	sheremetyevoJSON = `{"icao":"UUEE","iata":"SVO","name":"Шереметьево",
		"city":{"name":"Москва","timezone":"Europe/Moscow"},"country":{"iso2":"RU","name":"Россия"}}`
	istanbulJSON = `{"icao":"LTFM","iata":"IST","name":"Стамбул",
		"city":{"name":"Стамбул","timezone":"Europe/Istanbul"},"country":{"iso2":"TR","name":"Турция"}}`
)

// directoryAirports — справочник: UUEE и LTFM с полными данными, XXXX без IATA и
// без таймзоны у города.
func directoryAirports() map[string]airport.Airport {
	svo, ist := "SVO", "IST"
	return map[string]airport.Airport{
		"UUEE": {
			ICAO: "UUEE", IATA: &svo, Name: "Шереметьево",
			City:    airport.City{Name: "Москва", Timezone: "Europe/Moscow"},
			Country: airport.Country{ISO2: "RU", Name: "Россия"},
		},
		"LTFM": {
			ICAO: "LTFM", IATA: &ist, Name: "Стамбул",
			City:    airport.City{Name: "Стамбул", Timezone: "Europe/Istanbul"},
			Country: airport.Country{ISO2: "TR", Name: "Турция"},
		},
		"XXXX": {
			ICAO: "XXXX", Name: "Малый", City: airport.City{Name: "Посёлок"},
			Country: airport.Country{ISO2: "RU", Name: "Россия"},
		},
	}
}

// rawJSON — значение поля как текст JSON.
func rawJSON(t *testing.T, value any) string {
	t.Helper()
	b, err := json.Marshal(value)
	require.NoError(t, err)
	return string(b)
}

func TestOffersHTTP_GetOffer_SegmentAirportObjects(t *testing.T) {
	e := newPriceTestEnvWithAirports(t, stubAirportRepo{airports: directoryAirports()})

	e.createFlight(t, timeFlight(timeSegment("UUEE", "2026-05-14T10:00:00", "LTFM", "2026-05-14T14:00:00")))

	seg := segmentAt(t, e.cardFlight(t), 0)
	assert.Equal(t, "UUEE", seg["departure_airport_icao"], "поле с кодом остаётся")
	assert.Equal(t, "LTFM", seg["arrival_airport_icao"], "поле с кодом остаётся")
	assert.JSONEq(t, sheremetyevoJSON, rawJSON(t, seg["departure_airport"]))
	assert.JSONEq(t, istanbulJSON, rawJSON(t, seg["arrival_airport"]))
}

func TestOffersHTTP_GetOffer_AirportWithoutIATAAndTimezone_NullsNotOmitted(t *testing.T) {
	e := newPriceTestEnvWithAirports(t, stubAirportRepo{airports: directoryAirports()})

	e.createFlight(t, timeFlight(timeSegment("XXXX", "2026-05-14T10:00:00", "UUEE", "2026-05-14T14:00:00")))

	assert.JSONEq(t, `{"icao":"XXXX","iata":null,"name":"Малый",
		"city":{"name":"Посёлок","timezone":null},"country":{"iso2":"RU","name":"Россия"}}`,
		rawJSON(t, segmentAt(t, e.cardFlight(t), 0)["departure_airport"]))
}

func TestOffersHTTP_GetOffer_AirportUpdatedInReference_CurrentDataReturned(t *testing.T) {
	airports := directoryAirports()
	e := newPriceTestEnvWithAirports(t, stubAirportRepo{airports: airports})
	e.createFlight(t, timeFlight(timeSegment("UUEE", "2026-05-14T10:00:00", "LTFM", "2026-05-14T14:00:00")))

	updated := airports["UUEE"]
	updated.Name = "Шереметьево им. А. С. Пушкина"
	airports["UUEE"] = updated

	departure, ok := segmentAt(t, e.cardFlight(t), 0)["departure_airport"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Шереметьево им. А. С. Пушкина", departure["name"])
}

func TestOffersHTTP_GetPublicOffer_SameAirportObjects(t *testing.T) {
	airports := stubAirportRepo{airports: directoryAirports()}
	e := newPriceTestEnvWithAirports(t, airports)
	e.createFlight(t, timeFlight(timeSegment("UUEE", "2026-05-14T10:00:00", "LTFM", "2026-05-14T14:00:00")))
	published := *e.offers.found
	published.Status = offer.StatusPublished
	publicUC := getpublishedoffer.NewHandler(&stubOfferRepo{found: &published}, e.flights, &stubAirlineRepo{}, airports)
	r := newOffersTestRouter(newTestJWTService(t), e.offers, stubUserFinder{}, publicUC, e.flights)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/offers/"+e.offerID.String(), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var card struct {
		Flights []map[string]any `json:"flights"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &card))
	require.Len(t, card.Flights, 1)
	seg := segmentAt(t, card.Flights[0], 0)
	assert.Equal(t, "UUEE", seg["departure_airport_icao"])
	assert.Equal(t, "2026-05-14T10:00:00", seg["departure_at"])
	assert.JSONEq(t, sheremetyevoJSON, rawJSON(t, seg["departure_airport"]))
	assert.JSONEq(t, istanbulJSON, rawJSON(t, seg["arrival_airport"]))
	// Часы Москвы и Стамбула совпадают, длительность — как на циферблате.
	assert.EqualValues(t, 14400, seg["duration_seconds"])
}
