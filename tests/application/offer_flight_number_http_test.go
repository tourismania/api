package application_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	getpublishedoffer "api/internal/application/query/get_published_offer"
	"api/internal/domain/offer"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Сценарии номера рейса сегмента перелёта (дельта offers/flights, изменение
// add-flight-segment-number). Сегменты — из offer_flight_cabin_class_http_test.go,
// все с airline_id.

// segmentWithNumber — сегмент из полей fields и flight_number со значением
// numberJSON (уже JSON: строка в кавычках, null, число и т. п.); пустой
// numberJSON — поле не передаётся.
func segmentWithNumber(fields, numberJSON string) string {
	if numberJSON == "" {
		return `{` + fields + `}`
	}
	return `{` + fields + `,"flight_number":` + numberJSON + `}`
}

// nonstopWithNumber — прямой перелёт UUEE→LFPG с номером numberJSON.
func nonstopWithNumber(numberJSON string) string {
	return classedFlightJSON(segmentWithNumber(firstSegmentFields, numberJSON))
}

// cardFlightNumbers возвращает flight_number каждого сегмента единственного
// перелёта карточки; поле обязано присутствовать всегда.
func (e priceTestEnv) cardFlightNumbers(t *testing.T) []any {
	t.Helper()
	var numbers []any
	for _, s := range e.cardSegments(t) {
		seg, ok := s.(map[string]any)
		require.True(t, ok)
		number, present := seg["flight_number"]
		require.True(t, present, "flight_number должен присутствовать всегда")
		numbers = append(numbers, number)
	}
	return numbers
}

func TestOffersHTTP_CreateFlight_WithFlightNumber_CardReturnsNumber(t *testing.T) {
	e := newPriceTestEnv(t)

	rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(nonstopWithNumber(`"SU1234"`)))

	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	assert.Equal(t, []any{"SU1234"}, e.cardFlightNumbers(t))
}

func TestOffersHTTP_CreateFlight_FlightNumber_ReturnedExactlyAsSent(t *testing.T) {
	for _, number := range []string{"su 1234", "U6-773", "ЮТ-773"} {
		t.Run(number, func(t *testing.T) {
			e := newPriceTestEnv(t)

			rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(nonstopWithNumber(jsonString(t, number))))

			require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
			assert.Equal(t, []any{number}, e.cardFlightNumbers(t), "регистр, пробелы внутри и разделители не меняются")
		})
	}
}

func TestOffersHTTP_CreateFlight_FlightNumberEdgeSpaces_Trimmed(t *testing.T) {
	for sent, want := range map[string]string{
		" U6-773 ":      "U6-773",
		"\tSU1234\n":    "SU1234",
		"  su 1234  ":   "su 1234",
		"\u00a0ЮТ-773 ": "ЮТ-773",
	} {
		t.Run(want, func(t *testing.T) {
			e := newPriceTestEnv(t)

			rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(nonstopWithNumber(jsonString(t, sent))))

			require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
			require.Len(t, e.flights.replacedFlights, 1)
			assert.Equal(t, want, e.flights.replacedFlights[0].Segments[0].FlightNumber)
			assert.Equal(t, []any{want}, e.cardFlightNumbers(t), "пробелы по краям обрезаются, внутри — нет")
		})
	}
}

func TestOffersHTTP_CreateFlight_NoFlightNumber_CardReturnsEmptyString(t *testing.T) {
	for name, numberJSON := range map[string]string{
		"поле отсутствует": "",
		"null":             `null`,
		"пустая строка":    `""`,
		"только пробелы":   `"   "`,
	} {
		t.Run(name, func(t *testing.T) {
			e := newPriceTestEnv(t)

			rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(nonstopWithNumber(numberJSON)))

			require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
			assert.Equal(t, []any{""}, e.cardFlightNumbers(t), "номер за клиента не подставляется")
		})
	}
}

func TestOffersHTTP_CreateFlight_NoFlightNumber_ResponseHasEmptyStringNotNull(t *testing.T) {
	e := newPriceTestEnv(t)
	require.Equal(t, http.StatusCreated, e.do(t, http.MethodPost, "/api/v1/offers", createBody(nonstopWithNumber(""))).Code)

	rr := e.do(t, http.MethodGet, "/api/v1/offers/"+e.offerID.String(), "")

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"flight_number":""`)
}

func TestOffersHTTP_CreateFlight_FlightNumberTenCyrillicChars_Returns201(t *testing.T) {
	e := newPriceTestEnv(t)
	number := strings.Repeat("Ю", 10) // 10 символов, 20 байт в UTF-8

	rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(nonstopWithNumber(jsonString(t, number))))

	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	assert.Equal(t, []any{number}, e.cardFlightNumbers(t))
}

func TestOffersHTTP_CreateFlight_FlightNumberTenCharsWithEdgeSpaces_Returns201Trimmed(t *testing.T) {
	e := newPriceTestEnv(t)
	number := "SU12345678" // 10 символов; с пробелами по краям — 14

	rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(nonstopWithNumber(jsonString(t, "  "+number+"  "))))

	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	assert.Equal(t, []any{number}, e.cardFlightNumbers(t), "длина проверяется после обрезки")
}

func TestOffersHTTP_CreateFlight_FlightNumberTooLong_Returns400Max(t *testing.T) {
	for _, number := range []string{"SU12345678X", strings.Repeat("Ю", 11), " SU12345678X "} {
		t.Run(number, func(t *testing.T) {
			e := newPriceTestEnv(t)

			rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(nonstopWithNumber(jsonString(t, number))))

			require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
			body := decodeError(t, rr)
			assert.Equal(t, "validation failed", body.Error)
			assert.Equal(t, map[string]any{"FlightNumber": "max"}, body.Meta)
			assert.False(t, e.offers.storeCalled, "оффер не должен создаваться")
			assert.False(t, e.flights.replaceCalled)
		})
	}
}

func TestOffersHTTP_CreateFlight_FlightNumberNotString_Returns400WrongType(t *testing.T) {
	for _, numberJSON := range []string{`1234`, `{"code":"SU"}`, `["SU1234"]`} {
		e := newPriceTestEnv(t)

		rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(nonstopWithNumber(numberJSON)))

		require.Equal(t, http.StatusBadRequest, rr.Code, numberJSON)
		assert.Contains(t, decodeError(t, rr).Error, "wrong type", numberJSON)
		assert.False(t, e.offers.storeCalled, numberJSON)
	}
}

func TestOffersHTTP_CreateFlight_DifferentFlightNumbersPerSegment_KeptInOrder(t *testing.T) {
	e := newPriceTestEnv(t)

	rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(classedFlightJSON(
		segmentWithNumber(firstSegmentFields, `"SU1234"`),
		segmentWithNumber(secondSegmentFields, `"SU5678"`),
	)))

	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	assert.Equal(t, []any{"SU1234", "SU5678"}, e.cardFlightNumbers(t))
}

func TestOffersHTTP_CreateFlight_FlightNumberOnFlight_Returns400UnknownField(t *testing.T) {
	e := newPriceTestEnv(t)

	rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(flightJSON(priced+`,"flight_number":"SU1234"`)))

	require.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, decodeError(t, rr).Error, "unknown field")
	assert.False(t, e.offers.storeCalled)
}

func TestOffersHTTP_CreateFlight_UnknownSegmentField_Returns400UnknownField(t *testing.T) {
	for _, field := range []string{`"flightNumber":"SU1234"`, `"baggage":"23kg"`} {
		t.Run(field, func(t *testing.T) {
			e := newPriceTestEnv(t)

			rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(classedFlightJSON(`{`+firstSegmentFields+`,`+field+`}`)))

			require.Equal(t, http.StatusBadRequest, rr.Code)
			assert.Contains(t, decodeError(t, rr).Error, "unknown field")
			assert.False(t, e.offers.storeCalled)
		})
	}
}

func TestOffersHTTP_CreateFlight_LegacySegmentsWithoutFlightNumber_Returns201(t *testing.T) {
	e := newPriceTestEnv(t)

	rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(flightJSON(priced)))

	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	require.Len(t, e.flights.replacedFlights, 1)
	assert.Equal(t, "", e.flights.replacedFlights[0].Segments[0].FlightNumber)
	assert.Equal(t, []any{""}, e.cardFlightNumbers(t))
}

func TestOffersHTTP_PatchFlights_SameSetAbsentVsEmptyFlightNumber_IsNoOp(t *testing.T) {
	e := newPriceTestEnv(t)
	require.Equal(t, http.StatusCreated, e.do(t, http.MethodPost, "/api/v1/offers",
		createBody(nonstopWithNumber(`""`))).Code)
	before := e.cardFlights(t)
	e.flights.replaceCalled = false

	for _, numberJSON := range []string{"", `null`, `""`, `"  "`} {
		rr := e.do(t, http.MethodPatch, "/api/v1/offers/"+e.offerID.String(), patchBody(nonstopWithNumber(numberJSON)))

		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		assert.False(t, e.flights.replaceCalled, numberJSON)
		assert.False(t, e.flights.updateDetailsCalled, numberJSON)
	}
	assert.Equal(t, before[0]["id"], e.cardFlights(t)[0]["id"])
}

func TestOffersHTTP_PatchFlights_OnlyFlightNumberChanged_KeepsFlightIDs(t *testing.T) {
	for name, tc := range map[string]struct{ from, to string }{
		"исправлена опечатка": {from: `"TK2412"`, to: `"TK2414"`},
		"другое написание":    {from: `"SU1234"`, to: `"su 1234"`},
		"пробел внутри":       {from: `"SU1234"`, to: `"SU 1234"`},
		"номер стал известен": {from: "", to: `"SU1234"`},
	} {
		t.Run(name, func(t *testing.T) {
			e := newPriceTestEnv(t)
			require.Equal(t, http.StatusCreated, e.do(t, http.MethodPost, "/api/v1/offers", createBody(classedFlightJSON(
				segmentWithNumber(firstSegmentFields, `"SU2130"`),
				segmentWithNumber(secondSegmentFields, tc.from),
			))).Code)
			before := e.cardFlights(t)
			e.flights.replaceCalled = false

			rr := e.do(t, http.MethodPatch, "/api/v1/offers/"+e.offerID.String(), patchBody(classedFlightJSON(
				segmentWithNumber(firstSegmentFields, `"SU2130"`),
				segmentWithNumber(secondSegmentFields, tc.to),
			)))

			require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
			assert.False(t, e.flights.replaceCalled, "number-only change must not replace flights")
			assert.True(t, e.flights.updateDetailsCalled)
			after := e.cardFlights(t)
			require.Len(t, after, 1)
			assert.Equal(t, before[0]["id"], after[0]["id"])
			var want string
			require.NoError(t, json.Unmarshal([]byte(tc.to), &want))
			assert.Equal(t, []any{"SU2130", want}, e.cardFlightNumbers(t))
		})
	}
}

func TestOffersHTTP_PatchFlights_FlightNumberDiffersOnlyByEdgeSpaces_IsNoOp(t *testing.T) {
	e := newPriceTestEnv(t)
	require.Equal(t, http.StatusCreated, e.do(t, http.MethodPost, "/api/v1/offers",
		createBody(nonstopWithNumber(`"SU1234"`))).Code)
	before := e.cardFlights(t)
	e.flights.replaceCalled = false

	rr := e.do(t, http.MethodPatch, "/api/v1/offers/"+e.offerID.String(), patchBody(nonstopWithNumber(`" SU1234 "`)))

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.False(t, e.flights.replaceCalled)
	assert.False(t, e.flights.updateDetailsCalled, "после обрезки номер совпадает с сохранённым")
	assert.Equal(t, before[0]["id"], e.cardFlights(t)[0]["id"])
	assert.Equal(t, []any{"SU1234"}, e.cardFlightNumbers(t))
}

func TestOffersHTTP_PatchFlights_FlightNumberRemoved_KeepsFlightIDs(t *testing.T) {
	for name, numberJSON := range map[string]string{
		"поле отсутствует": "",
		"null":             `null`,
		"пустая строка":    `""`,
		"только пробелы":   `" "`,
	} {
		t.Run(name, func(t *testing.T) {
			e := newPriceTestEnv(t)
			require.Equal(t, http.StatusCreated, e.do(t, http.MethodPost, "/api/v1/offers",
				createBody(nonstopWithNumber(`"SU1234"`))).Code)
			before := e.cardFlights(t)
			e.flights.replaceCalled = false

			rr := e.do(t, http.MethodPatch, "/api/v1/offers/"+e.offerID.String(), patchBody(nonstopWithNumber(numberJSON)))

			require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
			assert.False(t, e.flights.replaceCalled)
			after := e.cardFlights(t)
			require.Len(t, after, 1)
			assert.Equal(t, before[0]["id"], after[0]["id"])
			assert.Equal(t, []any{""}, e.cardFlightNumbers(t))
		})
	}
}

func TestOffersHTTP_PatchFlights_FlightNumberTooLong_Returns400_NothingChanged(t *testing.T) {
	e := newPriceTestEnv(t)
	require.Equal(t, http.StatusCreated, e.do(t, http.MethodPost, "/api/v1/offers",
		createBody(nonstopWithNumber(`"SU1234"`))).Code)
	e.flights.replaceCalled = false

	body := `{"title":"Новое название","flights":[` + nonstopWithNumber(`"SU12345678X"`) + `]}`
	rr := e.do(t, http.MethodPatch, "/api/v1/offers/"+e.offerID.String(), body)

	require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
	assert.Equal(t, map[string]any{"FlightNumber": "max"}, decodeError(t, rr).Meta)
	assert.False(t, e.offers.updateCalled, "базовые поля не должны меняться")
	assert.False(t, e.flights.replaceCalled)
	assert.False(t, e.flights.updateDetailsCalled)
	assert.Equal(t, []any{"SU1234"}, e.cardFlightNumbers(t))
}

func TestOffersHTTP_GetPublicOffer_WithFlights_IncludesFlightNumber(t *testing.T) {
	e := newPriceTestEnv(t)
	require.Equal(t, http.StatusCreated, e.do(t, http.MethodPost, "/api/v1/offers", createBody(classedFlightJSON(
		segmentWithNumber(firstSegmentFields, `"SU1234"`),
		segmentWithNumber(secondSegmentFields, ""),
	))).Code)
	// Публичная карточка через настоящий use case поверх тех же хранилищ.
	published := *e.offers.found
	published.Status = offer.StatusPublished
	publicUC := getpublishedoffer.NewHandler(&stubOfferRepo{found: &published}, e.flights, &stubAirlineRepo{}, stubAirportRepo{})
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
	assert.Equal(t, "SU1234", card.Flights[0].Segments[0]["flight_number"])
	assert.Equal(t, "", card.Flights[0].Segments[1]["flight_number"])
}
