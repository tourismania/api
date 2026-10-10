package application_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	getpublishedoffer "api/internal/application/query/get_published_offer"
	"api/internal/domain/offer"
	"api/internal/domain/user"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// priceTestEnv — роутер с агентом своего агентства и офером, который
// находится по любому uuid: достаточно для сценариев POST → GET и PATCH → GET.
type priceTestEnv struct {
	router  http.Handler
	token   string
	offerID uuid.UUID
	flights *stubOfferFlightRepo
}

const priceTestAgencyID = 3

func newPriceTestEnv(t *testing.T) priceTestEnv {
	t.Helper()
	jwtSvc := newTestJWTService(t)
	userUUID := uuid.New()
	token, err := jwtSvc.Issue(userUUID)
	require.NoError(t, err)

	users := stubUserFinder{record: &user.Record{
		ID: 1, Uuid: userUUID, Roles: []string{string(user.RoleAgent)}, AgencyID: priceTestAgencyID,
	}}
	offerUUID := uuid.New()
	offers := &stubOfferRepo{storeID: 9, found: &offer.Offer{
		ID: 9, UUID: offerUUID, AgencyID: priceTestAgencyID, Title: "Test", Status: offer.StatusDraft,
	}}
	flights := &stubOfferFlightRepo{}
	return priceTestEnv{
		router:  newOffersTestRouter(jwtSvc, offers, users, &stubGetPublishedOfferUseCase{}, flights),
		token:   token,
		offerID: offerUUID,
		flights: flights,
	}
}

func (e priceTestEnv) do(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+e.token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	e.router.ServeHTTP(rr, req)
	return rr
}

// flightJSON — тело одного перелёта с прямым сегментом UUEE→LFPG; priceJSON
// вставляется как есть (`"price":"100","currency":"RUB"` и т. п.).
func flightJSON(priceJSON string) string {
	sep := ""
	if priceJSON != "" {
		sep = ","
	}
	return `{` + priceJSON + sep + `"segments":[
		{"departure_airport_icao":"UUEE","arrival_airport_icao":"LFPG","departure_at":"2026-08-01T10:00:00Z","arrival_at":"2026-08-01T14:00:00Z"}
	]}`
}

func createBody(flights ...string) string {
	return `{"title":"Test","description":"d","status":"draft","flights":[` + strings.Join(flights, ",") + `]}`
}

func patchBody(flights ...string) string {
	return `{"flights":[` + strings.Join(flights, ",") + `]}`
}

// errorBody — разобранный ответ об ошибке.
type errorBody struct {
	Error string         `json:"error"`
	Meta  map[string]any `json:"meta"`
}

func decodeError(t *testing.T, rr *httptest.ResponseRecorder) errorBody {
	t.Helper()
	var body errorBody
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body), rr.Body.String())
	return body
}

// cardFlights читает перелёты из GET /api/v1/offers/{uuid}.
func (e priceTestEnv) cardFlights(t *testing.T) []map[string]any {
	t.Helper()
	rr := e.do(t, http.MethodGet, "/api/v1/offers/"+e.offerID.String(), "")
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var card struct {
		Flights []map[string]any `json:"flights"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &card))
	return card.Flights
}

func TestOffersHTTP_CreateFlight_WithPrice_CardReturnsNormalizedPrice(t *testing.T) {
	e := newPriceTestEnv(t)

	rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(flightJSON(`"price":"12345.5","currency":"EUR"`)))
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	flights := e.cardFlights(t)
	require.Len(t, flights, 1)
	assert.Equal(t, "12345.50", flights[0]["price"])
	assert.Equal(t, "EUR", flights[0]["currency"])
}

func TestOffersHTTP_CreateFlight_ZeroPrice_CardReturnsZeroWithCents(t *testing.T) {
	e := newPriceTestEnv(t)

	rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(flightJSON(`"price":"0","currency":"RUB"`)))
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	flights := e.cardFlights(t)
	require.Len(t, flights, 1)
	assert.Equal(t, "0.00", flights[0]["price"])
}

func TestOffersHTTP_CreateFlight_WithoutPriceAndCurrency_Returns400WithBothFields(t *testing.T) {
	e := newPriceTestEnv(t)

	rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(flightJSON("")))

	require.Equal(t, http.StatusBadRequest, rr.Code)
	body := decodeError(t, rr)
	assert.Equal(t, "validation failed", body.Error)
	assert.Contains(t, body.Meta, "Price")
	assert.Contains(t, body.Meta, "Currency")
	assert.False(t, e.flights.replaceCalled)
}

func TestOffersHTTP_CreateFlight_EmptyPrice_Returns400(t *testing.T) {
	e := newPriceTestEnv(t)

	rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(flightJSON(`"price":"","currency":"RUB"`)))

	require.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, decodeError(t, rr).Meta, "Price")
}

func TestOffersHTTP_CreateFlight_MoreDecimalsThanCurrency_Returns400Price(t *testing.T) {
	e := newPriceTestEnv(t)

	rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(flightJSON(`"price":"10.125","currency":"USD"`)))

	require.Equal(t, http.StatusBadRequest, rr.Code)
	body := decodeError(t, rr)
	assert.Equal(t, "validation failed", body.Error)
	assert.Equal(t, map[string]any{"Price": "money"}, body.Meta)
}

func TestOffersHTTP_CreateFlight_InvalidPriceNotation_Returns400Price(t *testing.T) {
	for _, price := range []string{"-1", "1e3", "12,50", "abc", "12345678901"} {
		e := newPriceTestEnv(t)

		rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(flightJSON(`"price":"`+price+`","currency":"RUB"`)))

		require.Equal(t, http.StatusBadRequest, rr.Code, price)
		assert.Equal(t, map[string]any{"Price": "money"}, decodeError(t, rr).Meta, price)
	}
}

func TestOffersHTTP_CreateFlight_PriceAsJSONNumber_Returns400WrongType(t *testing.T) {
	e := newPriceTestEnv(t)

	rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(flightJSON(`"price":100,"currency":"RUB"`)))

	require.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, decodeError(t, rr).Error, "wrong type")
}

func TestOffersHTTP_CreateFlight_UnsupportedCurrency_Returns400Currency(t *testing.T) {
	for _, currency := range []string{"GBP", "rub"} {
		e := newPriceTestEnv(t)

		rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(flightJSON(`"price":"100","currency":"`+currency+`"`)))

		require.Equal(t, http.StatusBadRequest, rr.Code, currency)
		body := decodeError(t, rr)
		assert.Equal(t, "validation failed", body.Error)
		assert.Equal(t, map[string]any{"Currency": "currency"}, body.Meta, currency)
	}
}

func TestOffersHTTP_CreateFlight_PriceInsideSegment_Returns400UnknownField(t *testing.T) {
	e := newPriceTestEnv(t)

	body := createBody(`{"price":"100","currency":"RUB","segments":[
		{"departure_airport_icao":"UUEE","arrival_airport_icao":"LFPG","departure_at":"2026-08-01T10:00:00Z","arrival_at":"2026-08-01T14:00:00Z","price":"100"}
	]}`)
	rr := e.do(t, http.MethodPost, "/api/v1/offers", body)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, decodeError(t, rr).Error, "unknown field")
}

func TestOffersHTTP_PatchFlights_SameSetDifferentNotation_IsNoOp(t *testing.T) {
	e := newPriceTestEnv(t)
	require.Equal(t, http.StatusCreated, e.do(t, http.MethodPost, "/api/v1/offers",
		createBody(flightJSON(`"price":"100.00","currency":"RUB"`))).Code)
	e.flights.replaceCalled = false

	rr := e.do(t, http.MethodPatch, "/api/v1/offers/"+e.offerID.String(), patchBody(flightJSON(`"price":"100","currency":"RUB"`)))

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.False(t, e.flights.replaceCalled)
	assert.False(t, e.flights.updatePricesCalled)
}

func TestOffersHTTP_PatchFlights_OnlyPriceChanged_KeepsFlightIDs(t *testing.T) {
	e := newPriceTestEnv(t)
	require.Equal(t, http.StatusCreated, e.do(t, http.MethodPost, "/api/v1/offers",
		createBody(flightJSON(`"price":"100","currency":"RUB"`))).Code)
	before := e.cardFlights(t)
	require.Len(t, before, 1)
	e.flights.replaceCalled = false

	rr := e.do(t, http.MethodPatch, "/api/v1/offers/"+e.offerID.String(), patchBody(flightJSON(`"price":"175.5","currency":"EUR"`)))

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.False(t, e.flights.replaceCalled, "price-only change must not replace flights")
	assert.True(t, e.flights.updatePricesCalled)
	after := e.cardFlights(t)
	require.Len(t, after, 1)
	assert.Equal(t, before[0]["id"], after[0]["id"])
	assert.Equal(t, "175.50", after[0]["price"])
	assert.Equal(t, "EUR", after[0]["currency"])
}

func TestOffersHTTP_PatchFlights_InvalidPrice_Returns400(t *testing.T) {
	e := newPriceTestEnv(t)

	rr := e.do(t, http.MethodPatch, "/api/v1/offers/"+e.offerID.String(), patchBody(flightJSON(`"price":"10.125","currency":"USD"`)))

	require.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, map[string]any{"Price": "money"}, decodeError(t, rr).Meta)
}

func TestOffersHTTP_GetPublicOffer_WithFlights_IncludesPriceAndCurrency(t *testing.T) {
	jwtSvc := newTestJWTService(t)
	id := uuid.New()
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	publicUC := &stubGetPublishedOfferUseCase{result: getpublishedoffer.Result{
		ID: 1, UUID: id, AgencyID: 5,
		Flights: []getpublishedoffer.FlightResult{{
			ID: 1, Price: "15000.00", Currency: "RUB",
			DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", TotalDurationSeconds: 7200,
			Segments: []getpublishedoffer.FlightSegmentResult{
				{DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", DepartureAt: base, ArrivalAt: base.Add(2 * time.Hour), DurationSeconds: 7200},
			},
		}},
	}}
	r := newOffersTestRouter(jwtSvc, &stubOfferRepo{}, stubUserFinder{}, publicUC, &stubOfferFlightRepo{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/offers/"+id.String(), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"price":"15000.00"`)
	assert.Contains(t, rr.Body.String(), `"currency":"RUB"`)
}
