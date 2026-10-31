package application_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	getpublishedoffer "api/internal/application/query/get_published_offer"
	"api/internal/domain/airline"
	"api/internal/domain/offer"
	"api/internal/domain/user"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Сценарии авиакомпании сегмента перелёта (дельта offers/flights, изменение
// add-flight-segment-airline). Справочник — stubAirlineRepo с записями ниже;
// он же хранит изменения «синхронизации» (переименование, слияние).

const (
	aeroflotID   = 42
	azimuthID    = 57
	transaeroID  = 61
	turkishID    = 70
	dupID        = 501 // дубль 502
	liveID       = 502
	goneID       = 503 // пропала из источника
	oldCodeID    = 80  // A — закрывшаяся с кодом XAA
	newCodeID    = 81  // B — получила код XAA
	unknownAirID = 999_999
)

func icao(s string) *string { return &s }

// liveIDValue и goneSince — адресуемые значения для записей справочника:
// 501 — дубль 502, 503 — пропала из источника.
var (
	liveIDValue = liveID
	goneSince   = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
)

func airlineDirectory() *stubAirlineRepo {
	dissolved := time.Date(2015, 10, 26, 0, 0, 0, 0, time.UTC)
	return &stubAirlineRepo{airlines: map[int]airline.Airline{
		airline.UnknownID: {ID: airline.UnknownID, Name: "Unknown", NameRU: "Неизвестная"},
		aeroflotID:        {ID: aeroflotID, ICAO: icao("AFL"), Name: "Aeroflot", NameRU: "Аэрофлот"},
		azimuthID:         {ID: azimuthID, ICAO: icao("AZO"), Name: "Azimuth", NameRU: "Азимут"},
		transaeroID:       {ID: transaeroID, ICAO: icao("TSO"), Name: "Transaero", NameRU: "Трансаэро", DissolvedOn: &dissolved},
		turkishID:         {ID: turkishID, ICAO: icao("THY"), Name: "Turkish Airlines"},
		dupID:             {ID: dupID, ICAO: icao("AZM"), Name: "Azimuth", MergedIntoID: &liveIDValue},
		liveID:            {ID: liveID, ICAO: icao("AZM"), Name: "Azimuth Airlines"},
		goneID:            {ID: goneID, ICAO: icao("GON"), Name: "Gone Air", MissingSince: &goneSince},
		oldCodeID:         {ID: oldCodeID, ICAO: icao("XAA"), Name: "Old Air"},
	}}
}

// airlineTestEnv — priceTestEnv со справочником авиакомпаний.
type airlineTestEnv struct {
	priceTestEnv
	airlines *stubAirlineRepo
}

func newAirlineTestEnv(t *testing.T) airlineTestEnv {
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
	airlines := airlineDirectory()
	return airlineTestEnv{
		priceTestEnv: priceTestEnv{
			router:  newOffersTestRouterWithAirlines(jwtSvc, offers, users, &stubGetPublishedOfferUseCase{}, flights, airlines),
			token:   token,
			offerID: offerUUID,
			offers:  offers,
			flights: flights,
		},
		airlines: airlines,
	}
}

// segmentWithAirline — сегмент из полей fields с airline_id = airlineJSON
// (уже JSON); пустой airlineJSON — поле не передаётся.
func segmentWithAirline(fields, airlineJSON string) string {
	if airlineJSON == "" {
		return `{` + fields + `}`
	}
	return `{` + fields + `,"airline_id":` + airlineJSON + `}`
}

// Сегменты без airline_id (firstSegmentFields в файле класса уже с ним).
const (
	firstLegFields  = `"departure_airport_icao":"UUEE","arrival_airport_icao":"LFPG","departure_at":"2026-08-01T10:00:00","arrival_at":"2026-08-01T14:00:00"`
	secondLegFields = `"departure_airport_icao":"LFPG","arrival_airport_icao":"KJFK","departure_at":"2026-08-01T16:00:00","arrival_at":"2026-08-02T00:00:00"`
)

// nonstopWithAirline — прямой перелёт UUEE→LFPG с airline_id = airlineJSON
// и ценой priceJSON (по умолчанию priced).
func nonstopWithAirline(airlineJSON string) string {
	return classedFlightJSON(segmentWithAirline(firstLegFields, airlineJSON))
}

func nonstopWithAirlinePriced(airlineJSON, priceJSON string) string {
	return `{` + priceJSON + `,"segments":[` + segmentWithAirline(firstLegFields, airlineJSON) + `]}`
}

// cardAirlines — airline_id и airline каждого сегмента единственного
// перелёта карточки; оба поля обязаны присутствовать.
func (e airlineTestEnv) cardAirlines(t *testing.T) ([]any, []map[string]any) {
	t.Helper()
	var ids []any
	var objects []map[string]any
	for _, s := range e.cardSegments(t) {
		seg, ok := s.(map[string]any)
		require.True(t, ok)
		id, present := seg["airline_id"]
		require.True(t, present, "airline_id присутствует всегда")
		obj, ok := seg["airline"].(map[string]any)
		require.True(t, ok, "airline — всегда объект: %v", seg["airline"])
		ids = append(ids, id)
		objects = append(objects, obj)
	}
	return ids, objects
}

func (e airlineTestEnv) flightIDs() []int {
	ids := make([]int, 0, len(e.flights.stored))
	for _, f := range e.flights.stored {
		ids = append(ids, f.ID)
	}
	return ids
}

func (e airlineTestEnv) create(t *testing.T, flights ...string) {
	t.Helper()
	rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(flights...))
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
}

// ---------------------------------------------------------------------------
// Запись и чтение
// ---------------------------------------------------------------------------

func TestOffersHTTP_CreateFlight_Airline_CardReturnsAirlineObject(t *testing.T) {
	e := newAirlineTestEnv(t)
	e.create(t, nonstopWithAirline(fmt.Sprint(aeroflotID)))

	ids, objects := e.cardAirlines(t)

	assert.Equal(t, []any{float64(aeroflotID)}, ids)
	assert.Equal(t, map[string]any{
		"id": float64(aeroflotID), "icao": "AFL", "name": "Aeroflot", "name_ru": "Аэрофлот", "dissolved_on": nil,
	}, objects[0])
}

func TestOffersHTTP_CreateFlight_UnknownAirline_CardReturnsServiceRecord(t *testing.T) {
	e := newAirlineTestEnv(t)
	e.create(t, nonstopWithAirline("1"))

	rr := e.do(t, http.MethodGet, "/api/v1/offers/"+e.offerID.String(), "")

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(),
		`"airline_id":1,"airline":{"id":1,"icao":null,"name":"Unknown","name_ru":"Неизвестная","dissolved_on":null}`)
}

func TestOffersHTTP_CreateFlight_DissolvedAirline_DissolvedOnReturned(t *testing.T) {
	e := newAirlineTestEnv(t)
	e.create(t, nonstopWithAirline(fmt.Sprint(transaeroID)))

	_, objects := e.cardAirlines(t)

	assert.Equal(t, "2015-10-26", objects[0]["dissolved_on"])
}

func TestOffersHTTP_CreateFlight_ForeignAirline_NameRUEmptyString(t *testing.T) {
	e := newAirlineTestEnv(t)
	e.create(t, nonstopWithAirline(fmt.Sprint(turkishID)))

	_, objects := e.cardAirlines(t)

	assert.Equal(t, "", objects[0]["name_ru"])
}

func TestOffersHTTP_CreateFlight_DifferentAirlinesPerSegment_KeptInOrder(t *testing.T) {
	e := newAirlineTestEnv(t)
	e.create(t, classedFlightJSON(
		segmentWithAirline(firstLegFields, fmt.Sprint(aeroflotID)),
		segmentWithAirline(secondLegFields, fmt.Sprint(azimuthID)),
	))

	ids, objects := e.cardAirlines(t)

	assert.Equal(t, []any{float64(aeroflotID), float64(azimuthID)}, ids)
	assert.Equal(t, "Azimuth", objects[1]["name"])
}

func TestOffersHTTP_CreateFlight_NoAirline_Returns400Required(t *testing.T) {
	for name, airlineJSON := range map[string]string{"поле отсутствует": "", "null": "null", "0": "0"} {
		t.Run(name, func(t *testing.T) {
			e := newAirlineTestEnv(t)

			rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(nonstopWithAirline(airlineJSON)))

			require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
			body := decodeError(t, rr)
			assert.Equal(t, "validation failed", body.Error)
			assert.Equal(t, map[string]any{"AirlineID": "required"}, body.Meta)
			assert.False(t, e.offers.storeCalled, "оффер не создан")
			assert.False(t, e.flights.replaceCalled, "авиакомпания за клиента не подставляется")
		})
	}
}

func TestOffersHTTP_CreateFlight_SegmentsInOldFormat_Returns400Required(t *testing.T) {
	e := newAirlineTestEnv(t)

	rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(classedFlightJSON(`{`+firstLegFields+`}`, `{`+secondLegFields+`}`)))

	require.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, map[string]any{"AirlineID": "required"}, decodeError(t, rr).Meta)
}

func TestOffersHTTP_CreateFlight_UnknownOrNotSelectableAirline_Returns400(t *testing.T) {
	for name, id := range map[string]int{
		"нет в справочнике": unknownAirID,
		"отрицательный":     -1,
		"дубль":             dupID,
		"пропавшая":         goneID,
	} {
		t.Run(name, func(t *testing.T) {
			e := newAirlineTestEnv(t)

			rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(nonstopWithAirline(fmt.Sprint(id))))

			require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
			assert.Contains(t, decodeError(t, rr).Error, "flight segment references an unknown airline")
			assert.False(t, e.flights.replaceCalled)
		})
	}
}

func TestOffersHTTP_CreateFlight_AirlineIDNotInteger_Returns400WrongType(t *testing.T) {
	for _, airlineJSON := range []string{`"AFL"`, `"42"`, `1.5`, `{}`, `[42]`} {
		t.Run(airlineJSON, func(t *testing.T) {
			e := newAirlineTestEnv(t)

			rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(nonstopWithAirline(airlineJSON)))

			require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
			assert.Contains(t, decodeError(t, rr).Error, "has the wrong type")
		})
	}
}

func TestOffersHTTP_CreateFlight_AirlineFieldMisplaced_Returns400UnknownField(t *testing.T) {
	for name, flightJSON := range map[string]string{
		"airline_id у перелёта": `{` + priced + `,"airline_id":42,"segments":[` + segmentWithAirline(firstLegFields, "42") + `]}`,
		"airline в сегменте":    classedFlightJSON(`{` + firstLegFields + `,"airline_id":42,"airline":"Аэрофлот"}`),
		"airline_icao вместо":   classedFlightJSON(`{` + firstLegFields + `,"airline_icao":"AFL"}`),
	} {
		t.Run(name, func(t *testing.T) {
			e := newAirlineTestEnv(t)

			rr := e.do(t, http.MethodPost, "/api/v1/offers", createBody(flightJSON))

			require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
			assert.Contains(t, decodeError(t, rr).Error, "unknown field")
		})
	}
}

func TestOffersHTTP_DirectoryRenamed_CardShowsCurrentData(t *testing.T) {
	e := newAirlineTestEnv(t)
	e.create(t, nonstopWithAirline(fmt.Sprint(aeroflotID)))

	renamed := e.airlines.airlines[aeroflotID]
	renamed.Name = "Aeroflot Russian Airlines"
	e.airlines.airlines[aeroflotID] = renamed

	_, objects := e.cardAirlines(t)
	assert.Equal(t, "Aeroflot Russian Airlines", objects[0]["name"])
}

func TestOffersHTTP_PublicCard_ContainsAirline(t *testing.T) {
	e := newAirlineTestEnv(t)
	e.create(t, nonstopWithAirline(fmt.Sprint(aeroflotID)))
	published := *e.offers.found
	published.Status = offer.StatusPublished
	publicUC := getpublishedoffer.NewHandler(&stubOfferRepo{found: &published}, e.flights, e.airlines, stubAirportRepo{})
	r := newOffersTestRouterWithAirlines(newTestJWTService(t), e.offers, stubUserFinder{}, publicUC, e.flights, e.airlines)

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
	seg := card.Flights[0].Segments[0]
	assert.Equal(t, float64(aeroflotID), seg["airline_id"])
	assert.Equal(t, "Аэрофлот", seg["airline"].(map[string]any)["name_ru"])
}

// ---------------------------------------------------------------------------
// PATCH: обновление на месте, переход кода, дубли, пропавшие
// ---------------------------------------------------------------------------

func TestOffersHTTP_PatchAirlineToUnknown_KeepsFlightIDs(t *testing.T) {
	e := newAirlineTestEnv(t)
	e.create(t, nonstopWithAirline(fmt.Sprint(aeroflotID)))
	before := e.flightIDs()

	rr := e.do(t, http.MethodPatch, "/api/v1/offers/"+e.offerID.String(), patchBody(nonstopWithAirline("1")))

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, before, e.flightIDs(), "id перелётов сохраняются")
	ids, _ := e.cardAirlines(t)
	assert.Equal(t, []any{float64(1)}, ids)
}

func TestOffersHTTP_PatchWithoutAirline_Returns400NothingChanged(t *testing.T) {
	e := newAirlineTestEnv(t)
	e.create(t, nonstopWithAirline(fmt.Sprint(aeroflotID)))
	e.offers.updateCalled = false

	rr := e.do(t, http.MethodPatch, "/api/v1/offers/"+e.offerID.String(),
		`{"title":"Changed","flights":[`+nonstopWithAirline("")+`]}`)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, map[string]any{"AirlineID": "required"}, decodeError(t, rr).Meta)
	assert.False(t, e.offers.updateCalled, "базовые поля не меняются")
	ids, _ := e.cardAirlines(t)
	assert.Equal(t, []any{float64(aeroflotID)}, ids)
}

func TestOffersHTTP_CodeMovedToAnotherAirline_SegmentKeepsOriginal(t *testing.T) {
	e := newAirlineTestEnv(t)
	e.create(t, nonstopWithAirline(fmt.Sprint(oldCodeID)))
	before := e.flightIDs()

	// Синхронизация: у A появилась дата закрытия, B получила тот же код.
	dissolved := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	a := e.airlines.airlines[oldCodeID]
	a.DissolvedOn = &dissolved
	e.airlines.airlines[oldCodeID] = a
	e.airlines.airlines[newCodeID] = airline.Airline{ID: newCodeID, ICAO: icao("XAA"), Name: "New Air"}

	_, objects := e.cardAirlines(t)
	assert.Equal(t, float64(oldCodeID), objects[0]["id"])
	assert.Equal(t, "Old Air", objects[0]["name"])
	assert.Equal(t, "2024-01-01", objects[0]["dissolved_on"])

	rr := e.do(t, http.MethodPatch, "/api/v1/offers/"+e.offerID.String(), patchBody(nonstopWithAirline(fmt.Sprint(oldCodeID))))
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, before, e.flightIDs())
	ids, _ := e.cardAirlines(t)
	assert.Equal(t, []any{float64(oldCodeID)}, ids)
}

func TestOffersHTTP_SegmentAirlineBecameDuplicate_OldOfferStillSaves(t *testing.T) {
	e := newAirlineTestEnv(t)
	// До слияния 501 был выбираемым.
	visible := e.airlines.airlines[dupID]
	visible.MergedIntoID = nil
	e.airlines.airlines[dupID] = visible
	e.create(t, nonstopWithAirline(fmt.Sprint(dupID)))

	// Синхронизация признала 501 дублем 502.
	visible.MergedIntoID = &liveIDValue
	e.airlines.airlines[dupID] = visible

	ids, objects := e.cardAirlines(t)
	assert.Equal(t, []any{float64(dupID)}, ids)
	assert.Equal(t, "Azimuth", objects[0]["name"])

	patch := func(flight string) *httptest.ResponseRecorder {
		return e.do(t, http.MethodPatch, "/api/v1/offers/"+e.offerID.String(), patchBody(flight))
	}
	require.Equal(t, http.StatusOK, patch(nonstopWithAirline(fmt.Sprint(dupID))).Code, "тот же набор")
	rr := patch(nonstopWithAirlinePriced(fmt.Sprint(dupID), `"price":"999","currency":"RUB"`))
	require.Equal(t, http.StatusOK, rr.Code, "другая цена, тот же дубль: %s", rr.Body.String())

	rr = patch(nonstopWithAirline(fmt.Sprint(liveID)))
	require.Equal(t, http.StatusOK, rr.Code, "замена на оставшуюся запись: %s", rr.Body.String())
	ids, _ = e.cardAirlines(t)
	assert.Equal(t, []any{float64(liveID)}, ids)

	rr = patch(nonstopWithAirline(fmt.Sprint(dupID)))
	require.Equal(t, http.StatusBadRequest, rr.Code, "после замены дубль — новая ссылка")
	assert.Contains(t, decodeError(t, rr).Error, "flight segment references an unknown airline")
}

func TestOffersHTTP_SegmentAirlineGoneFromSource_SameOfferSavesOtherRejected(t *testing.T) {
	e := newAirlineTestEnv(t)
	visible := e.airlines.airlines[goneID]
	visible.MissingSince = nil
	e.airlines.airlines[goneID] = visible
	e.create(t, nonstopWithAirline(fmt.Sprint(goneID)))
	visible.MissingSince = &goneSince
	e.airlines.airlines[goneID] = visible

	rr := e.do(t, http.MethodPatch, "/api/v1/offers/"+e.offerID.String(), patchBody(nonstopWithAirline(fmt.Sprint(goneID))))
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	other := newAirlineTestEnv(t)
	rr = other.do(t, http.MethodPatch, "/api/v1/offers/"+other.offerID.String(), patchBody(nonstopWithAirline(fmt.Sprint(goneID))))
	require.Equal(t, http.StatusBadRequest, rr.Code, "в другом оффере — новая ссылка")
	assert.True(t, strings.Contains(decodeError(t, rr).Error, "unknown airline"))
}
