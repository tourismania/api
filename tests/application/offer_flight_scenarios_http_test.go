package application_test

// Сценарии спек offers/flights и travel-services/currencies, которые
// /opsx:verify нашёл без тестов (issue №28). Поведение не менялось —
// тесты его фиксируют.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"api/internal/domain/offer"
	"api/internal/domain/user"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// offers/flights, «Неверный формат времени».
func TestOffersHTTP_CreateFlight_NonRFC3339Time_Returns400(t *testing.T) {
	e := newPriceTestEnv(t)

	body := createBody(`{"price":"100","currency":"RUB","segments":[
		{"airline_id":1,"departure_airport_icao":"UUEE","arrival_airport_icao":"LFPG","departure_at":"2026-08-01 10:00","arrival_at":"2026-08-01T14:00:00Z"}
	]}`)
	rr := e.do(t, http.MethodPost, "/api/v1/offers", body)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	msg := decodeError(t, rr).Error
	assert.Contains(t, msg, "invalid date/time value")
	assert.Contains(t, msg, "expected RFC3339")
	assert.False(t, e.flights.replaceCalled, "оффер и перелёты не создаются")
}

// offers/flights, «Оффер без перелётов»: flights — пустой массив, а не null.
func TestOffersHTTP_GetOffer_NoFlights_ReturnsEmptyArray(t *testing.T) {
	e := newPriceTestEnv(t)

	rr := e.do(t, http.MethodGet, "/api/v1/offers/"+e.offerID.String(), "")

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var card map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &card))
	require.Contains(t, card, "flights")
	assert.JSONEq(t, `[]`, string(card["flights"]))
}

// offers/flights, «Список офферов»: элементы списка не содержат flights.
func TestOffersHTTP_ListOffers_ItemsHaveNoFlights(t *testing.T) {
	jwtSvc := newTestJWTService(t)
	userUUID := uuid.New()
	token, err := jwtSvc.Issue(userUUID)
	require.NoError(t, err)

	now := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	offers := &stubOfferRepo{listed: []offer.Offer{
		{ID: 1, UUID: uuid.New(), AgencyID: priceTestAgencyID, Title: "A", Status: offer.StatusDraft, CreatedAt: now, UpdatedAt: now},
		{ID: 2, UUID: uuid.New(), AgencyID: priceTestAgencyID, Title: "B", Status: offer.StatusPublished, CreatedAt: now, UpdatedAt: now},
	}}
	users := stubUserFinder{record: &user.Record{
		ID: 1, Uuid: userUUID, Roles: []string{string(user.RoleAgent)}, AgencyID: priceTestAgencyID,
	}}
	r := newOffersTestRouter(jwtSvc, offers, users, &stubGetPublishedOfferUseCase{}, &stubOfferFlightRepo{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/offers", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var list struct {
		Data []map[string]json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &list))
	require.Len(t, list.Data, 2)
	for _, item := range list.Data {
		assert.NotContains(t, item, "flights")
	}
}

// travel-services/currencies, «Список совпадает с правилами записи цены»:
// клиент берёт code и exponent из GET /api/v1/currencies; цена ровно с
// exponent знаками после точки принимается, с exponent+1 — отклоняется.
func TestCurrenciesHTTP_ListMatchesPriceRules(t *testing.T) {
	jwtSvc := newTestJWTService(t)
	token, err := jwtSvc.Issue(uuid.New())
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/currencies", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	newCurrenciesTestRouter(jwtSvc).ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	var list struct {
		Data []struct {
			Code     string `json:"code"`
			Exponent int    `json:"exponent"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &list))
	require.NotEmpty(t, list.Data)

	for _, c := range list.Data {
		e := newPriceTestEnv(t)
		ok := e.do(t, http.MethodPost, "/api/v1/offers",
			createBody(flightJSON(`"price":"`+priceWithDecimals(c.Exponent)+`","currency":"`+c.Code+`"`)))
		assert.Equal(t, http.StatusCreated, ok.Code, "%s: %d знаков должны приниматься: %s", c.Code, c.Exponent, ok.Body.String())

		e = newPriceTestEnv(t)
		bad := e.do(t, http.MethodPost, "/api/v1/offers",
			createBody(flightJSON(`"price":"`+priceWithDecimals(c.Exponent+1)+`","currency":"`+c.Code+`"`)))
		require.Equal(t, http.StatusBadRequest, bad.Code, "%s: %d знаков должны отклоняться", c.Code, c.Exponent+1)
		assert.Equal(t, map[string]any{"Price": "money"}, decodeError(t, bad).Meta, c.Code)
	}
}

// priceWithDecimals — сумма «100» с n знаками после точки («100», «100.5», «100.55», …).
func priceWithDecimals(n int) string {
	if n == 0 {
		return "100"
	}
	return "100." + strings.Repeat("5", n)
}
