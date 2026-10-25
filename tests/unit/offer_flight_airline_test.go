package unit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"api/internal/application/apperror"
	createoffer "api/internal/application/command/create_offer"
	updateoffer "api/internal/application/command/update_offer"
	"api/internal/application/helpers"
	getoffer "api/internal/application/query/get_offer"
	getpublishedoffer "api/internal/application/query/get_published_offer"
	"api/internal/domain/airline"
	"api/internal/domain/offer"
	"api/internal/domain/offer/flight"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Авиакомпания сегмента перелёта (дельта offers/flights, изменение
// add-flight-segment-airline): фабрика, Manager, перевод ошибок, сборка из
// DTO и чтение карточки.

var airlineBase = time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)

// withAirlines возвращает копию перелёта с ID id и авиакомпаниями сегментов
// (по порядку); сегменты копируются.
func withAirlines(f flight.Flight, id int, airlineIDs ...int) flight.Flight {
	f.ID = id
	f.Segments = append([]flight.Segment(nil), f.Segments...)
	for i, a := range airlineIDs {
		f.Segments[i].AirlineID = a
	}
	return f
}

// airlinesDirectory — справочник mockAirlineRepo с перечисленными записями.
func airlinesDirectory(airlines ...airline.Airline) *mockAirlineRepo {
	m := &mockAirlineRepo{airlines: map[int]airline.Airline{}}
	for _, a := range airlines {
		m.airlines[a.ID] = a
	}
	return m
}

func icaoPtr(s string) *string { return &s }

func intPtr(i int) *int { return &i }

// Записи справочника для тестов: 42 — «Аэрофлот», 57 — «Азимут», 61 —
// прекратившая работу, 501 — дубль 502, 503 — пропавшая из источника.
var (
	aeroflot     = airline.Airline{ID: 42, ICAO: icaoPtr("AFL"), Name: "Aeroflot", NameRU: "Аэрофлот", Country: &airline.Country{ISO2: "RU"}}
	azimuth      = airline.Airline{ID: 57, ICAO: icaoPtr("AZO"), Name: "Azimuth", NameRU: "Азимут"}
	transaero    = airline.Airline{ID: 61, ICAO: icaoPtr("TSO"), Name: "Transaero", DissolvedOn: date(2015, 10, 26)}
	azimuthDup   = airline.Airline{ID: 501, ICAO: icaoPtr("AZO"), Name: "Azimuth", MergedIntoID: intPtr(502)}
	azimuthLive  = airline.Airline{ID: 502, ICAO: icaoPtr("AZO"), Name: "Azimuth Airlines"}
	goneAirline  = airline.Airline{ID: 503, ICAO: icaoPtr("GON"), Name: "Gone Air", MissingSince: date(2026, 10, 1)}
	unknownEntry = airline.Airline{ID: airline.UnknownID, Name: "Unknown", NameRU: "Неизвестная"}
)

func testDirectory() *mockAirlineRepo {
	return airlinesDirectory(unknownEntry, aeroflot, azimuth, transaero, azimuthDup, azimuthLive, goneAirline)
}

// ---------------------------------------------------------------------------
// Фабрика
// ---------------------------------------------------------------------------

func TestFlightNew_NoAirline_ReturnsErrAirlineRequired(t *testing.T) {
	_, err := flight.New(samplePrice(), "", []flight.Segment{
		{DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", DepartureAt: airlineBase, ArrivalAt: airlineBase.Add(time.Hour)},
	})

	assert.ErrorIs(t, err, flight.ErrAirlineRequired)
}

func TestFlightNew_UnknownAirline_Accepted(t *testing.T) {
	f, err := flight.New(samplePrice(), "", []flight.Segment{
		{AirlineID: airline.UnknownID, DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", DepartureAt: airlineBase, ArrivalAt: airlineBase.Add(time.Hour)},
	})

	require.NoError(t, err)
	assert.Equal(t, airline.UnknownID, f.Segments[0].AirlineID)
}

func TestFlightNew_NegativeAirline_PassesToManager(t *testing.T) {
	_, err := flight.New(samplePrice(), "", []flight.Segment{
		{AirlineID: -1, DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", DepartureAt: airlineBase, ArrivalAt: airlineBase.Add(time.Hour)},
	})

	assert.NoError(t, err, "отрицательный id отклоняет Manager как несуществующий")
}

// ---------------------------------------------------------------------------
// Manager: проверка ссылок
// ---------------------------------------------------------------------------

func replaceWithAirlines(t *testing.T, airlines *mockAirlineRepo, stored []flight.Flight, incoming ...flight.Flight) (*mockOfferFlightRepo, error) {
	t.Helper()
	flights := &mockOfferFlightRepo{findByOfferIDFlights: stored}
	mgr := flight.NewManager(flights, &mockAirportRepo{}, airlines)
	return flights, mgr.ReplaceForOffer(context.Background(), 1, incoming)
}

func TestOfferFlightManager_UnknownOrNegativeAirline_ErrAirlineNotFound(t *testing.T) {
	for name, id := range map[string]int{"нет в справочнике": 999_999, "отрицательный": -1} {
		t.Run(name, func(t *testing.T) {
			flights, err := replaceWithAirlines(t, testDirectory(), nil,
				withAirlines(sampleFlight("UUEE", "LFPG", airlineBase), 0, id))

			assert.ErrorIs(t, err, flight.ErrAirlineNotFound)
			assert.False(t, flights.replaceCalled)
		})
	}
}

func TestOfferFlightManager_NotSelectableAirlineNotReferencedBefore_ErrAirlineNotFound(t *testing.T) {
	for name, id := range map[string]int{"дубль": azimuthDup.ID, "пропавшая": goneAirline.ID} {
		t.Run(name+" при создании", func(t *testing.T) {
			flights, err := replaceWithAirlines(t, testDirectory(), nil,
				withAirlines(sampleFlight("UUEE", "LFPG", airlineBase), 0, id))

			assert.ErrorIs(t, err, flight.ErrAirlineNotFound)
			assert.False(t, flights.replaceCalled)
			assert.False(t, flights.updateDetailsCalled)
		})
		t.Run(name+" в PATCH оффера без него", func(t *testing.T) {
			stored := []flight.Flight{withAirlines(sampleFlight("UUEE", "LFPG", airlineBase), 7, aeroflot.ID)}
			flights, err := replaceWithAirlines(t, testDirectory(), stored,
				withAirlines(sampleFlight("UUEE", "LFPG", airlineBase), 0, id))

			assert.ErrorIs(t, err, flight.ErrAirlineNotFound)
			assert.False(t, flights.replaceCalled)
			assert.False(t, flights.updateDetailsCalled)
		})
	}
}

func TestOfferFlightManager_NotSelectableAirlineAlreadyInOffer_Accepted(t *testing.T) {
	// Сохранён прямой перелёт с дублем 501 и перелёт с 42; в PATCH 501 стоит
	// у другого (нового) перелёта — сравнение по всему офферу, не по позиции.
	stored := []flight.Flight{
		withAirlines(sampleFlight("UUEE", "LFPG", airlineBase), 7, azimuthDup.ID),
		withAirlines(sampleFlight("LFPG", "UUEE", airlineBase.Add(48*time.Hour)), 8, aeroflot.ID),
	}
	flights, err := replaceWithAirlines(t, testDirectory(), stored,
		withAirlines(sampleFlight("UUEE", "KJFK", airlineBase), 0, aeroflot.ID),
		withAirlines(sampleFlight("KJFK", "UUEE", airlineBase.Add(72*time.Hour)), 0, azimuthDup.ID),
	)

	require.NoError(t, err)
	assert.True(t, flights.replaceCalled)
}

func TestOfferFlightManager_DissolvedAirline_Accepted(t *testing.T) {
	_, err := replaceWithAirlines(t, testDirectory(), nil,
		withAirlines(sampleFlight("UUEE", "LFPG", airlineBase), 0, transaero.ID))

	assert.NoError(t, err)
}

func TestOfferFlightManager_SeveralSegmentsSameAirline_OneQuery(t *testing.T) {
	airlines := testDirectory()
	_, err := replaceWithAirlines(t, airlines, nil,
		withAirlines(twoSegmentFlight(airlineBase), 0, aeroflot.ID, aeroflot.ID),
		withAirlines(sampleFlight("KJFK", "UUEE", airlineBase.Add(72*time.Hour)), 0, azimuth.ID),
	)

	require.NoError(t, err)
	require.Len(t, airlines.findCalls, 1, "один запрос на весь набор")
	assert.ElementsMatch(t, []int{aeroflot.ID, azimuth.ID}, airlines.findCalls[0], "id без повторов")
}

func TestOfferFlightManager_NoFlights_NoAirlineQuery(t *testing.T) {
	airlines := testDirectory()
	_, err := replaceWithAirlines(t, airlines, nil)

	require.NoError(t, err)
	assert.Empty(t, airlines.findCalls)
}

// ---------------------------------------------------------------------------
// Manager: сравнение и обновление на месте
// ---------------------------------------------------------------------------

func TestOfferFlightManager_SameAirlines_IsNoOp(t *testing.T) {
	stored := []flight.Flight{withAirlines(twoSegmentFlight(airlineBase), 7, aeroflot.ID, azimuth.ID)}
	flights, err := replaceWithAirlines(t, testDirectory(), stored,
		withAirlines(twoSegmentFlight(airlineBase), 0, aeroflot.ID, azimuth.ID))

	require.NoError(t, err)
	assert.False(t, flights.replaceCalled)
	assert.False(t, flights.updateDetailsCalled)
}

func TestOfferFlightManager_OnlyOneSegmentAirlineChanged_UpdatesThatFlightOnly(t *testing.T) {
	stored := []flight.Flight{
		withAirlines(twoSegmentFlight(airlineBase), 7, aeroflot.ID, aeroflot.ID),
		withAirlines(sampleFlight("KJFK", "UUEE", airlineBase.Add(72*time.Hour)), 8, aeroflot.ID),
	}
	flights, err := replaceWithAirlines(t, testDirectory(), stored,
		withAirlines(twoSegmentFlight(airlineBase), 0, aeroflot.ID, azimuth.ID),
		withAirlines(sampleFlight("KJFK", "UUEE", airlineBase.Add(72*time.Hour)), 0, aeroflot.ID),
	)

	require.NoError(t, err)
	assert.False(t, flights.replaceCalled, "маршрут тот же — без замены")
	assert.Equal(t, []flight.DetailsUpdate{{
		FlightID:            7,
		Price:               samplePrice(),
		SegmentCabinClasses: []flight.CabinClass{flight.CabinClassUnknown, flight.CabinClassUnknown},
		SegmentAirlineIDs:   []int{aeroflot.ID, azimuth.ID},
	}}, flights.updatedDetails)
}

func TestOfferFlightManager_AirlineToUnknown_UpdatedInPlace(t *testing.T) {
	stored := []flight.Flight{withAirlines(sampleFlight("UUEE", "LFPG", airlineBase), 7, aeroflot.ID)}
	flights, err := replaceWithAirlines(t, testDirectory(), stored,
		withAirlines(sampleFlight("UUEE", "LFPG", airlineBase), 0, airline.UnknownID))

	require.NoError(t, err)
	assert.False(t, flights.replaceCalled)
	require.Len(t, flights.updatedDetails, 1)
	assert.Equal(t, 7, flights.updatedDetails[0].FlightID)
	assert.Equal(t, []int{airline.UnknownID}, flights.updatedDetails[0].SegmentAirlineIDs)
}

func TestOfferFlightManager_AirlineAndCabinClassChanged_OneUpdate(t *testing.T) {
	stored := []flight.Flight{withAirlines(sampleFlight("UUEE", "LFPG", airlineBase), 7, aeroflot.ID)}
	incoming := withCabinClasses(withAirlines(sampleFlight("UUEE", "LFPG", airlineBase), 0, azimuth.ID), 0, flight.CabinClassBusiness)
	flights, err := replaceWithAirlines(t, testDirectory(), stored, incoming)

	require.NoError(t, err)
	assert.Equal(t, []flight.DetailsUpdate{{
		FlightID:            7,
		Price:               samplePrice(),
		SegmentCabinClasses: []flight.CabinClass{flight.CabinClassBusiness},
		SegmentAirlineIDs:   []int{azimuth.ID},
	}}, flights.updatedDetails)
}

// ---------------------------------------------------------------------------
// Перевод ошибок и сборка из DTO
// ---------------------------------------------------------------------------

func TestFromDomainError_AirlineErrors_AreValidation(t *testing.T) {
	for _, err := range []error{flight.ErrAirlineRequired, flight.ErrAirlineNotFound} {
		got := apperror.FromDomainError(err)
		assert.ErrorIs(t, got, apperror.ErrValidation)
		assert.Contains(t, got.Error(), err.Error())
	}
}

func TestBuildFlights_AirlineIDPassedAsIs(t *testing.T) {
	segs := validSegments()
	segs[0].AirlineID = transaero.ID

	flights, err := helpers.BuildFlights([]helpers.FlightInput{{Price: "100", Currency: "RUB", Segments: segs}})

	require.NoError(t, err)
	assert.Equal(t, transaero.ID, flights[0].Segments[0].AirlineID)
}

func TestBuildFlights_NoAirline_ReturnsAirlineIDRequiredFieldError(t *testing.T) {
	segs := validSegments()
	segs[0].AirlineID = 0

	_, err := helpers.BuildFlights([]helpers.FlightInput{{Price: "100", Currency: "RUB", Segments: segs}})

	fe, ok := apperror.AsFieldError(err)
	require.True(t, ok, err)
	assert.Equal(t, "AirlineID", fe.Field)
	assert.Equal(t, "required", fe.Rule)
	assert.ErrorIs(t, err, flight.ErrAirlineRequired)
}

func TestCreateOffer_UnknownAirline_ValidationErrorNoWrite(t *testing.T) {
	offers := &mockOfferRepo{storeID: 42}
	mgr := offer.NewManager(offers, &mockAgencyRepo{findByIDAgency: activeAgency(5)})
	flightRepo := &mockOfferFlightRepo{}
	h := createoffer.NewHandler(mgr, flight.NewManager(flightRepo, &mockAirportRepo{}, testDirectory()), agentUserFinder(5), noopTxManager{})

	_, err := h.Handle(context.Background(), createoffer.Command{
		Title: "Title", Status: offer.StatusDraft,
		Flights: []createoffer.FlightInput{{Price: "15000", Currency: "RUB", Segments: []createoffer.FlightSegmentInput{
			{AirlineID: azimuthDup.ID, DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", DepartureAt: airlineBase, ArrivalAt: airlineBase.Add(4 * time.Hour)},
		}}},
		CurrentUserUUID: uuid.New(),
	})

	assert.ErrorIs(t, err, apperror.ErrValidation)
	assert.Contains(t, err.Error(), "flight segment references an unknown airline")
	assert.False(t, flightRepo.replaceCalled)
}

func TestUpdateOffer_OnlyAirlineChanged_UpdatesDetailsKeepsFlights(t *testing.T) {
	o := &offer.Offer{ID: 9, UUID: uuid.New(), AgencyID: 5, Status: offer.StatusDraft}
	mgr := offer.NewManager(&mockOfferRepo{findByUUIDOffer: o}, &mockAgencyRepo{findByIDAgency: activeAgency(5)})
	stored := withAirlines(sampleFlight("UUEE", "LFPG", airlineBase), 31, aeroflot.ID)
	flightRepo := &mockOfferFlightRepo{findByOfferIDFlights: []flight.Flight{stored}}
	h := updateoffer.NewHandler(mgr, flight.NewManager(flightRepo, &mockAirportRepo{}, testDirectory()), agentUserFinder(5), noopTxManager{})

	flights := []updateoffer.FlightInput{{Price: "15000", Currency: "RUB", Segments: []updateoffer.FlightSegmentInput{
		{AirlineID: airline.UnknownID, DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", DepartureAt: airlineBase, ArrivalAt: airlineBase.Add(2 * time.Hour)},
	}}}
	_, err := h.Handle(context.Background(), updateoffer.Command{UUID: o.UUID, Flights: &flights, CurrentUserUUID: uuid.New()})

	require.NoError(t, err)
	assert.False(t, flightRepo.replaceCalled, "id перелётов сохраняются")
	require.Len(t, flightRepo.updatedDetails, 1)
	assert.Equal(t, []int{airline.UnknownID}, flightRepo.updatedDetails[0].SegmentAirlineIDs)
}

// ---------------------------------------------------------------------------
// Чтение карточки
// ---------------------------------------------------------------------------

func airlineCardFlights() []flight.Flight {
	return []flight.Flight{
		withAirlines(twoSegmentFlight(airlineBase), 11, aeroflot.ID, azimuth.ID),
		withAirlines(sampleFlight("KJFK", "UUEE", airlineBase.Add(72*time.Hour)), 12, aeroflot.ID),
		withAirlines(sampleFlight("UUEE", "LFPG", airlineBase.Add(96*time.Hour)), 13, transaero.ID),
		withAirlines(sampleFlight("LFPG", "UUEE", airlineBase.Add(120*time.Hour)), 14, airline.UnknownID),
	}
}

func TestGetOffer_SegmentAirlines_OneQueryAndFullData(t *testing.T) {
	o := &offer.Offer{UUID: uuid.New(), AgencyID: 7, Status: offer.StatusDraft}
	mgr := offer.NewManager(&mockOfferRepo{findByUUIDOffer: o}, &mockAgencyRepo{})
	airlines := testDirectory()
	h := getoffer.NewHandler(mgr, stubFlightFinder{flights: airlineCardFlights()}, airlines, userRecordWithAgency(7))

	res, err := h.Handle(context.Background(), getoffer.Query{UUID: o.UUID})

	require.NoError(t, err)
	require.Len(t, airlines.findCalls, 1, "один запрос на оффер")
	assert.ElementsMatch(t, []int{aeroflot.ID, azimuth.ID, transaero.ID, airline.UnknownID}, airlines.findCalls[0])

	seg := res.Flights[0].Segments
	assert.Equal(t, aeroflot.ID, seg[0].AirlineID)
	assert.Equal(t, getoffer.AirlineResult{ID: 42, ICAO: icaoPtr("AFL"), Name: "Aeroflot", NameRU: "Аэрофлот"}, seg[0].Airline)
	assert.Equal(t, "Азимут", seg[1].Airline.NameRU, "у сегментов одного перелёта свои авиакомпании")
	assert.Equal(t, transaero.DissolvedOn, res.Flights[2].Segments[0].Airline.DissolvedOn)
	assert.Equal(t, "", res.Flights[2].Segments[0].Airline.NameRU, "русского названия нет — пустая строка")
	unknown := res.Flights[3].Segments[0].Airline
	assert.Equal(t, getoffer.AirlineResult{ID: 1, Name: "Unknown", NameRU: "Неизвестная"}, unknown)
	assert.Nil(t, unknown.ICAO)
}

func TestGetOffer_NoFlights_NoAirlineQuery(t *testing.T) {
	o := &offer.Offer{UUID: uuid.New(), AgencyID: 7, Status: offer.StatusDraft}
	mgr := offer.NewManager(&mockOfferRepo{findByUUIDOffer: o}, &mockAgencyRepo{})
	airlines := testDirectory()
	h := getoffer.NewHandler(mgr, stubFlightFinder{}, airlines, userRecordWithAgency(7))

	_, err := h.Handle(context.Background(), getoffer.Query{UUID: o.UUID})

	require.NoError(t, err)
	assert.Empty(t, airlines.findCalls)
}

func TestGetOffer_SegmentAirlineMissingInDirectory_NonValidationError(t *testing.T) {
	o := &offer.Offer{UUID: uuid.New(), AgencyID: 7, Status: offer.StatusDraft}
	mgr := offer.NewManager(&mockOfferRepo{findByUUIDOffer: o}, &mockAgencyRepo{})
	stored := []flight.Flight{withAirlines(sampleFlight("UUEE", "LFPG", airlineBase), 11, 777)}
	h := getoffer.NewHandler(mgr, stubFlightFinder{flights: stored}, testDirectory(), userRecordWithAgency(7))

	_, err := h.Handle(context.Background(), getoffer.Query{UUID: o.UUID})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "777")
	assert.False(t, errors.Is(err, apperror.ErrValidation), "порча данных — 500, не 400")
}

func TestGetPublishedOffer_SegmentAirlines_Returned(t *testing.T) {
	o := &offer.Offer{UUID: uuid.New(), AgencyID: 7, Status: offer.StatusPublished}
	airlines := testDirectory()
	h := getpublishedoffer.NewHandler(stubOfferFinder{offer: o}, stubFlightFinder{flights: airlineCardFlights()}, airlines)

	res, err := h.Handle(context.Background(), getpublishedoffer.Query{UUID: o.UUID})

	require.NoError(t, err)
	require.Len(t, airlines.findCalls, 1)
	assert.Equal(t, getpublishedoffer.AirlineResult{ID: 42, ICAO: icaoPtr("AFL"), Name: "Aeroflot", NameRU: "Аэрофлот"}, res.Flights[0].Segments[0].Airline)
	assert.Equal(t, "Unknown", res.Flights[3].Segments[0].Airline.Name)
	assert.Equal(t, transaero.DissolvedOn, res.Flights[2].Segments[0].Airline.DissolvedOn)
}

func TestGetPublishedOffer_NoFlights_NoAirlineQuery(t *testing.T) {
	o := &offer.Offer{UUID: uuid.New(), AgencyID: 7, Status: offer.StatusPublished}
	airlines := testDirectory()
	h := getpublishedoffer.NewHandler(stubOfferFinder{offer: o}, stubFlightFinder{}, airlines)

	_, err := h.Handle(context.Background(), getpublishedoffer.Query{UUID: o.UUID})

	require.NoError(t, err)
	assert.Empty(t, airlines.findCalls)
}
