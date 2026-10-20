package application_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	getpublishedoffer "api/internal/application/query/get_published_offer"
	"api/internal/domain/offer"
	"api/internal/domain/offer/flight"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Сценарии класса обслуживания сегмента перелёта (дельта offers/flights,
// изменение add-flight-segment-class).

// Сегменты UUEE→LFPG и LFPG→KJFK одного перелёта с пересадкой в LFPG.
const (
	firstSegmentFields  = `"airline_id":1,"departure_airport_icao":"UUEE","arrival_airport_icao":"LFPG","departure_at":"2026-08-01T10:00:00Z","arrival_at":"2026-08-01T14:00:00Z"`
	secondSegmentFields = `"airline_id":1,"departure_airport_icao":"LFPG","arrival_airport_icao":"KJFK","departure_at":"2026-08-01T16:00:00Z","arrival_at":"2026-08-02T00:00:00Z"`
)

// segmentWithClass — сегмент из полей fields и cabin_class со значением
// classJSON (уже JSON: строка в кавычках, null, число и т. п.); пустой
// classJSON — поле не передаётся.
func segmentWithClass(fields, classJSON string) string {
	if classJSON == "" {
		return `{` + fields + `}`
	}
	return `{` + fields + `,"cabin_class":` + classJSON + `}`
}

// classedFlightJSON — перелёт по цене priced из переданных сегментов.
func classedFlightJSON(segments ...string) string {
	body := `{` + priced + `,"segments":[`
	for i, s := range segments {
		if i > 0 {
			body += ","
		}
		body += s
	}
	return body + `]}`
}

// nonstopWithClass — прямой перелёт UUEE→LFPG с классом classJSON.
func nonstopWithClass(classJSON string) string {
	return classedFlightJSON(segmentWithClass(firstSegmentFields, classJSON))
}

// cardSegments читает сегменты единственного перелёта из карточки.
func (e priceTestEnv) cardSegments(t *testing.T) []any {
	t.Helper()
	flights := e.cardFlights(t)
	require.Len(t, flights, 1)
	segments, ok := flights[0]["segments"].([]any)
	require.True(t, ok)
	return segments
}

// cardCabinClasses возвращает cabin_class каждого сегмента единственного
// перелёта карточки; поле обязано присутствовать всегда.
func (e priceTestEnv) cardCabinClasses(t *testing.T) []any {
	t.Helper()
	var classes []any
	for _, s := range e.cardSegments(t) {
		seg, ok := s.(map[string]any)
		require.True(t, ok)
		class, present := seg["cabin_class"]
		require.True(t, present, "cabin_class должен присутствовать всегда")
		classes = append(classes, class)
	}
	return classes
}

func TestOffersHTTP_CreateFlight_EachCabinClass_CardReturnsClass(t *testing.T) {
	for _, class := range []string{"economy", "comfort", "comfort_plus", "business"} {
		t.Run(class, func(t *testing.T) {
			e := newPriceTestEnv(t)

			rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(nonstopWithClass(`"`+class+`"`)))

			require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
			assert.Equal(t, []any{class}, e.cardCabinClasses(t))
		})
	}
}

func TestOffersHTTP_CreateFlight_NoCabinClass_CardReturnsEmptyString(t *testing.T) {
	for name, classJSON := range map[string]string{
		"поле отсутствует": "",
		"null":             `null`,
		"пустая строка":    `""`,
	} {
		t.Run(name, func(t *testing.T) {
			e := newPriceTestEnv(t)

			rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(nonstopWithClass(classJSON)))

			require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
			assert.Equal(t, []any{""}, e.cardCabinClasses(t), "класс за клиента не подставляется")
		})
	}
}

func TestOffersHTTP_CreateFlight_NoCabinClass_ResponseHasEmptyStringNotNull(t *testing.T) {
	e := newPriceTestEnv(t)
	require.Equal(t, http.StatusCreated, e.do(t, http.MethodPost, "/api/v1/offers", createBody(nonstopWithClass(""))).Code)

	rr := e.do(t, http.MethodGet, "/api/v1/offers/"+e.offerID.String(), "")

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"cabin_class":""`)
}

func TestOffersHTTP_CreateFlight_DifferentClassesPerSegment_KeptInOrder(t *testing.T) {
	e := newPriceTestEnv(t)

	rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(classedFlightJSON(
		segmentWithClass(firstSegmentFields, `"economy"`),
		segmentWithClass(secondSegmentFields, `"comfort"`),
	)))

	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	assert.Equal(t, []any{"economy", "comfort"}, e.cardCabinClasses(t))
}

func TestOffersHTTP_CreateFlight_UnknownCabinClass_Returns400Oneof(t *testing.T) {
	for _, class := range []string{"first", "premium_economy", "comfort+", "Business", " economy"} {
		t.Run(class, func(t *testing.T) {
			e := newPriceTestEnv(t)

			rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(nonstopWithClass(jsonString(t, class))))

			require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
			body := decodeError(t, rr)
			assert.Equal(t, "validation failed", body.Error)
			assert.Equal(t, map[string]any{"CabinClass": "oneof"}, body.Meta)
			assert.False(t, e.offers.storeCalled, "оффер не должен создаваться")
			assert.False(t, e.flights.replaceCalled)
		})
	}
}

func TestOffersHTTP_CreateFlight_CabinClassNotString_Returns400WrongType(t *testing.T) {
	for _, classJSON := range []string{`1`, `{"name":"economy"}`, `["economy"]`} {
		e := newPriceTestEnv(t)

		rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(nonstopWithClass(classJSON)))

		require.Equal(t, http.StatusBadRequest, rr.Code, classJSON)
		assert.Contains(t, decodeError(t, rr).Error, "wrong type", classJSON)
		assert.False(t, e.offers.storeCalled, classJSON)
	}
}

func TestOffersHTTP_CreateFlight_CabinClassOnFlight_Returns400UnknownField(t *testing.T) {
	e := newPriceTestEnv(t)

	rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(flightJSON(priced+`,"cabin_class":"economy"`)))

	require.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, decodeError(t, rr).Error, "unknown field")
	assert.False(t, e.offers.storeCalled)
}

func TestOffersHTTP_CreateFlight_CamelCaseFlightClass_Returns400UnknownField(t *testing.T) {
	e := newPriceTestEnv(t)

	rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(classedFlightJSON(`{`+firstSegmentFields+`,"flightClass":"economy"}`)))

	require.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, decodeError(t, rr).Error, "unknown field")
	assert.False(t, e.offers.storeCalled)
}

func TestOffersHTTP_CreateFlight_LegacySegmentsWithoutClass_Returns201(t *testing.T) {
	e := newPriceTestEnv(t)

	rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(flightJSON(priced)))

	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	require.Len(t, e.flights.replacedFlights, 1)
	assert.Equal(t, flight.CabinClassUnknown, e.flights.replacedFlights[0].Segments[0].CabinClass)
	assert.Equal(t, []any{""}, e.cardCabinClasses(t))
}

func TestOffersHTTP_PatchFlights_SameSetAbsentVsEmptyClass_IsNoOp(t *testing.T) {
	e := newPriceTestEnv(t)
	require.Equal(t, http.StatusCreated, e.do(t, http.MethodPost, "/api/v1/offers",
		createBody(nonstopWithClass(""))).Code)
	before := e.cardFlights(t)
	e.flights.replaceCalled = false

	for _, classJSON := range []string{`""`, `null`, ""} {
		rr := e.do(t, http.MethodPatch, "/api/v1/offers/"+e.offerID.String(), patchBody(nonstopWithClass(classJSON)))

		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		assert.False(t, e.flights.replaceCalled, classJSON)
		assert.False(t, e.flights.updateDetailsCalled, classJSON)
	}
	assert.Equal(t, before[0]["id"], e.cardFlights(t)[0]["id"])
}

func TestOffersHTTP_PatchFlights_SameClasses_IsNoOp(t *testing.T) {
	e := newPriceTestEnv(t)
	set := classedFlightJSON(
		segmentWithClass(firstSegmentFields, `"economy"`),
		segmentWithClass(secondSegmentFields, `"business"`),
	)
	require.Equal(t, http.StatusCreated, e.do(t, http.MethodPost, "/api/v1/offers", createBody(set)).Code)
	e.flights.replaceCalled = false

	rr := e.do(t, http.MethodPatch, "/api/v1/offers/"+e.offerID.String(), patchBody(set))

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.False(t, e.flights.replaceCalled)
	assert.False(t, e.flights.updateDetailsCalled)
}

func TestOffersHTTP_PatchFlights_OnlyCabinClassChanged_KeepsFlightIDs(t *testing.T) {
	e := newPriceTestEnv(t)
	require.Equal(t, http.StatusCreated, e.do(t, http.MethodPost, "/api/v1/offers", createBody(classedFlightJSON(
		segmentWithClass(firstSegmentFields, `"economy"`),
		segmentWithClass(secondSegmentFields, `"economy"`),
	))).Code)
	before := e.cardFlights(t)
	e.flights.replaceCalled = false

	rr := e.do(t, http.MethodPatch, "/api/v1/offers/"+e.offerID.String(), patchBody(classedFlightJSON(
		segmentWithClass(firstSegmentFields, `"economy"`),
		segmentWithClass(secondSegmentFields, `"comfort_plus"`),
	)))

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.False(t, e.flights.replaceCalled, "class-only change must not replace flights")
	assert.True(t, e.flights.updateDetailsCalled)
	after := e.cardFlights(t)
	require.Len(t, after, 1)
	assert.Equal(t, before[0]["id"], after[0]["id"])
	assert.Equal(t, before[0]["segments"].([]any)[0].(map[string]any)["departure_at"],
		after[0]["segments"].([]any)[0].(map[string]any)["departure_at"])
	assert.Equal(t, []any{"economy", "comfort_plus"}, e.cardCabinClasses(t))
	assert.Equal(t, "100.00", after[0]["price"])
}

func TestOffersHTTP_PatchFlights_CabinClassRemoved_KeepsFlightIDs(t *testing.T) {
	for name, classJSON := range map[string]string{
		"поле отсутствует": "",
		"null":             `null`,
		"пустая строка":    `""`,
	} {
		t.Run(name, func(t *testing.T) {
			e := newPriceTestEnv(t)
			require.Equal(t, http.StatusCreated, e.do(t, http.MethodPost, "/api/v1/offers",
				createBody(nonstopWithClass(`"business"`))).Code)
			before := e.cardFlights(t)
			e.flights.replaceCalled = false

			rr := e.do(t, http.MethodPatch, "/api/v1/offers/"+e.offerID.String(), patchBody(nonstopWithClass(classJSON)))

			require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
			assert.False(t, e.flights.replaceCalled)
			after := e.cardFlights(t)
			require.Len(t, after, 1)
			assert.Equal(t, before[0]["id"], after[0]["id"])
			assert.Equal(t, []any{""}, e.cardCabinClasses(t))
		})
	}
}

func TestOffersHTTP_PatchFlights_UnknownCabinClass_Returns400_NothingChanged(t *testing.T) {
	e := newPriceTestEnv(t)
	require.Equal(t, http.StatusCreated, e.do(t, http.MethodPost, "/api/v1/offers",
		createBody(nonstopWithClass(`"economy"`))).Code)
	e.flights.replaceCalled = false

	body := `{"title":"Новое название","flights":[` + nonstopWithClass(`"first"`) + `]}`
	rr := e.do(t, http.MethodPatch, "/api/v1/offers/"+e.offerID.String(), body)

	require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
	assert.Equal(t, map[string]any{"CabinClass": "oneof"}, decodeError(t, rr).Meta)
	assert.False(t, e.offers.updateCalled, "базовые поля не должны меняться")
	assert.False(t, e.flights.replaceCalled)
	assert.False(t, e.flights.updateDetailsCalled)
	assert.Equal(t, []any{"economy"}, e.cardCabinClasses(t))
}

func TestOffersHTTP_GetPublicOffer_WithFlights_IncludesCabinClass(t *testing.T) {
	e := newPriceTestEnv(t)
	require.Equal(t, http.StatusCreated, e.do(t, http.MethodPost, "/api/v1/offers", createBody(classedFlightJSON(
		segmentWithClass(firstSegmentFields, `"business"`),
		segmentWithClass(secondSegmentFields, ""),
	))).Code)
	// Публичная карточка через настоящий use case поверх тех же хранилищ.
	published := *e.offers.found
	published.Status = offer.StatusPublished
	publicUC := getpublishedoffer.NewHandler(&stubOfferRepo{found: &published}, e.flights, &stubAirlineRepo{})
	r := newOffersTestRouter(newTestJWTService(t), e.offers, stubUserFinder{}, publicUC, e.flights)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/offers/"+e.offerID.String(), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var card struct {
		Flights []struct {
			Segments []map[string]any `json:"segments"`
		} `json:"flights"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &card))
	require.Len(t, card.Flights, 1)
	require.Len(t, card.Flights[0].Segments, 2)
	assert.Equal(t, "business", card.Flights[0].Segments[0]["cabin_class"])
	assert.Equal(t, "", card.Flights[0].Segments[1]["cabin_class"])
}
