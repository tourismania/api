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

// Сценарии комментария менеджера к перелёту (дельта offers/flights,
// изменение add-flight-manager-comment).

// managerCommentMaxLen — лимит длины комментария в символах из контракта.
const managerCommentMaxLen = 2000

// priced — цена и валюта перелёта по умолчанию для тел запросов.
const priced = `"price":"100","currency":"RUB"`

// withComment дописывает к полям перелёта manager_comment со значением
// commentJSON (уже JSON: строка в кавычках, null, число и т. п.).
func withComment(commentJSON string) string {
	return priced + `,"manager_comment":` + commentJSON
}

// jsonString кодирует s как JSON-строку.
func jsonString(t *testing.T, s string) string {
	t.Helper()
	b, err := json.Marshal(s)
	require.NoError(t, err)
	return string(b)
}

// createAndReadComment создаёт оффер с одним перелётом и возвращает
// manager_comment перелёта из карточки.
func (e priceTestEnv) createAndReadComment(t *testing.T, flightFields string) any {
	t.Helper()
	rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(flightJSON(flightFields)))
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	flights := e.cardFlights(t)
	require.Len(t, flights, 1)
	comment, ok := flights[0]["manager_comment"]
	require.True(t, ok, "manager_comment должен присутствовать всегда")
	return comment
}

func TestOffersHTTP_CreateFlight_WithManagerComment_CardReturnsComment(t *testing.T) {
	e := newPriceTestEnv(t)

	comment := e.createAndReadComment(t, withComment(`"Багаж оплачивается отдельно"`))

	assert.Equal(t, "Багаж оплачивается отдельно", comment)
}

func TestOffersHTTP_CreateFlight_NoComment_CardReturnsEmptyString(t *testing.T) {
	for name, fields := range map[string]string{
		"поле отсутствует": priced,
		"null":             withComment(`null`),
		"пустая строка":    withComment(`""`),
	} {
		t.Run(name, func(t *testing.T) {
			e := newPriceTestEnv(t)

			assert.Equal(t, "", e.createAndReadComment(t, fields))
		})
	}
}

func TestOffersHTTP_CreateFlight_NoComment_ResponseHasEmptyStringNotNull(t *testing.T) {
	e := newPriceTestEnv(t)
	require.Equal(t, http.StatusCreated, e.do(t, http.MethodPost, "/api/v1/offers", createBody(flightJSON(priced))).Code)

	rr := e.do(t, http.MethodGet, "/api/v1/offers/"+e.offerID.String(), "")

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"manager_comment":""`)
}

func TestOffersHTTP_CreateFlight_CommentKeptAsIs(t *testing.T) {
	for name, text := range map[string]string{
		"переводы строк и табуляция": "Строка 1\nСтрока 2\r\n\tотступ",
		"пробелы по краям":           "  Багаж оплачивается отдельно  ",
		"разметка":                   "<b>A & B</b> <script>alert(1)</script>",
		"эмодзи":                     "Отличный рейс ✈️🧳",
	} {
		t.Run(name, func(t *testing.T) {
			e := newPriceTestEnv(t)

			assert.Equal(t, text, e.createAndReadComment(t, withComment(jsonString(t, text))))
		})
	}
}

func TestOffersHTTP_CreateFlight_SQLLikeComment_ReturnedUnchanged(t *testing.T) {
	e := newPriceTestEnv(t)
	text := `Rock'n'roll \ "VIP"'; DROP TABLE offers; --`

	assert.Equal(t, text, e.createAndReadComment(t, withComment(jsonString(t, text))))

	// Оффер и его перелёты по-прежнему читаются.
	flights := e.cardFlights(t)
	require.Len(t, flights, 1)
	assert.Equal(t, text, flights[0]["manager_comment"])
	assert.Equal(t, "100.00", flights[0]["price"])
}

func TestOffersHTTP_CreateFlight_ControlCharInComment_Returns400Text(t *testing.T) {
	for name, commentJSON := range map[string]string{
		"NUL": `"до\u0000после"`,
		"ESC": `"\u001b[31mкрасный"`,
	} {
		t.Run(name, func(t *testing.T) {
			e := newPriceTestEnv(t)

			rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(flightJSON(withComment(commentJSON))))

			require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
			body := decodeError(t, rr)
			assert.Equal(t, "validation failed", body.Error)
			assert.Equal(t, map[string]any{"ManagerComment": "text"}, body.Meta)
			assert.False(t, e.offers.storeCalled, "оффер не должен создаваться")
			assert.False(t, e.flights.replaceCalled)
		})
	}
}

func TestOffersHTTP_CreateFlight_CommentExactlyMaxCyrillic_Returns201(t *testing.T) {
	e := newPriceTestEnv(t)
	text := strings.Repeat("ж", managerCommentMaxLen)
	require.Greater(t, len(text), managerCommentMaxLen, "в байтах длиннее лимита")

	assert.Equal(t, text, e.createAndReadComment(t, withComment(jsonString(t, text))))
}

func TestOffersHTTP_CreateFlight_CommentTooLong_Returns400Max(t *testing.T) {
	e := newPriceTestEnv(t)
	text := strings.Repeat("ж", managerCommentMaxLen+1)

	rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(flightJSON(withComment(jsonString(t, text)))))

	require.Equal(t, http.StatusBadRequest, rr.Code)
	body := decodeError(t, rr)
	assert.Equal(t, "validation failed", body.Error)
	assert.Equal(t, map[string]any{"ManagerComment": "max"}, body.Meta)
	assert.False(t, e.offers.storeCalled, "оффер не должен создаваться")
}

func TestOffersHTTP_CreateFlight_CommentNotString_Returns400WrongType(t *testing.T) {
	for _, commentJSON := range []string{`42`, `{"text":"a"}`, `["a"]`} {
		e := newPriceTestEnv(t)

		rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(flightJSON(withComment(commentJSON))))

		require.Equal(t, http.StatusBadRequest, rr.Code, commentJSON)
		assert.Contains(t, decodeError(t, rr).Error, "wrong type", commentJSON)
		assert.False(t, e.offers.storeCalled, commentJSON)
	}
}

func TestOffersHTTP_CreateFlight_CommentInsideSegment_Returns400UnknownField(t *testing.T) {
	e := newPriceTestEnv(t)

	body := createBody(`{"price":"100","currency":"RUB","segments":[
		{"airline_id":1,"departure_airport_icao":"UUEE","arrival_airport_icao":"LFPG","departure_at":"2026-08-01T10:00:00Z","arrival_at":"2026-08-01T14:00:00Z","manager_comment":"a"}
	]}`)
	rr := e.do(t, http.MethodPost, "/api/v1/offers", body)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, decodeError(t, rr).Error, "unknown field")
}

func TestOffersHTTP_CreateFlight_CamelCaseComment_Returns400UnknownField(t *testing.T) {
	e := newPriceTestEnv(t)

	rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(flightJSON(priced+`,"managerComment":"a"`)))

	require.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, decodeError(t, rr).Error, "unknown field")
}

func TestOffersHTTP_CreateFlight_LegacyFormatWithoutComment_Returns201(t *testing.T) {
	e := newPriceTestEnv(t)

	rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(flightJSON(priced)))

	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	require.Len(t, e.flights.replacedFlights, 1)
	assert.Equal(t, "", e.flights.replacedFlights[0].ManagerComment)
}

func TestOffersHTTP_PatchFlights_SameSetNullVsEmptyComment_IsNoOp(t *testing.T) {
	e := newPriceTestEnv(t)
	require.Equal(t, http.StatusCreated, e.do(t, http.MethodPost, "/api/v1/offers",
		createBody(flightJSON(withComment(`""`)))).Code)
	before := e.cardFlights(t)
	e.flights.replaceCalled = false

	for _, fields := range []string{withComment(`null`), priced, withComment(`""`)} {
		rr := e.do(t, http.MethodPatch, "/api/v1/offers/"+e.offerID.String(), patchBody(flightJSON(fields)))

		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		assert.False(t, e.flights.replaceCalled, fields)
		assert.False(t, e.flights.updateDetailsCalled, fields)
	}
	assert.Equal(t, before[0]["id"], e.cardFlights(t)[0]["id"])
}

func TestOffersHTTP_PatchFlights_OnlyCommentChanged_KeepsFlightIDs(t *testing.T) {
	e := newPriceTestEnv(t)
	require.Equal(t, http.StatusCreated, e.do(t, http.MethodPost, "/api/v1/offers",
		createBody(flightJSON(withComment(`"старый"`)))).Code)
	before := e.cardFlights(t)
	e.flights.replaceCalled = false

	rr := e.do(t, http.MethodPatch, "/api/v1/offers/"+e.offerID.String(), patchBody(flightJSON(withComment(`"Ночная пересадка"`))))

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.False(t, e.flights.replaceCalled, "comment-only change must not replace flights")
	assert.True(t, e.flights.updateDetailsCalled)
	after := e.cardFlights(t)
	require.Len(t, after, 1)
	assert.Equal(t, before[0]["id"], after[0]["id"])
	assert.Equal(t, "Ночная пересадка", after[0]["manager_comment"])
	assert.Equal(t, "100.00", after[0]["price"])
}

func TestOffersHTTP_PatchFlights_CommentRemoved_KeepsFlightIDs(t *testing.T) {
	for name, fields := range map[string]string{
		"поле отсутствует": priced,
		"null":             withComment(`null`),
		"пустая строка":    withComment(`""`),
	} {
		t.Run(name, func(t *testing.T) {
			e := newPriceTestEnv(t)
			require.Equal(t, http.StatusCreated, e.do(t, http.MethodPost, "/api/v1/offers",
				createBody(flightJSON(withComment(`"Багаж оплачивается отдельно"`)))).Code)
			before := e.cardFlights(t)
			e.flights.replaceCalled = false

			rr := e.do(t, http.MethodPatch, "/api/v1/offers/"+e.offerID.String(), patchBody(flightJSON(fields)))

			require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
			assert.False(t, e.flights.replaceCalled)
			after := e.cardFlights(t)
			require.Len(t, after, 1)
			assert.Equal(t, before[0]["id"], after[0]["id"])
			assert.Equal(t, "", after[0]["manager_comment"])
		})
	}
}

func TestOffersHTTP_PatchFlights_ControlCharInComment_Returns400Text(t *testing.T) {
	e := newPriceTestEnv(t)

	rr := e.do(t, http.MethodPatch, "/api/v1/offers/"+e.offerID.String(), patchBody(flightJSON(withComment(`"\u0000"`))))

	require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
	assert.Equal(t, map[string]any{"ManagerComment": "text"}, decodeError(t, rr).Meta)
	assert.False(t, e.flights.replaceCalled)
	assert.False(t, e.flights.updateDetailsCalled)
}

func TestOffersHTTP_PatchFlights_CommentTooLong_Returns400Max(t *testing.T) {
	e := newPriceTestEnv(t)
	text := strings.Repeat("a", managerCommentMaxLen+1)

	rr := e.do(t, http.MethodPatch, "/api/v1/offers/"+e.offerID.String(), patchBody(flightJSON(withComment(jsonString(t, text)))))

	require.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, map[string]any{"ManagerComment": "max"}, decodeError(t, rr).Meta)
	assert.False(t, e.flights.replaceCalled)
	assert.False(t, e.flights.updateDetailsCalled)
}

func TestOffersHTTP_GetPublicOffer_WithFlights_IncludesManagerComment(t *testing.T) {
	e := newPriceTestEnv(t)
	require.Equal(t, http.StatusCreated, e.do(t, http.MethodPost, "/api/v1/offers",
		createBody(flightJSON(withComment(`"Багаж оплачивается отдельно"`)), flightJSON(priced))).Code)
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
		Flights []map[string]any `json:"flights"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &card))
	require.Len(t, card.Flights, 2)
	assert.Equal(t, "Багаж оплачивается отдельно", card.Flights[0]["manager_comment"])
	assert.Equal(t, "", card.Flights[1]["manager_comment"])
}
