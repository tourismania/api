package unit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"api/internal/application/apperror"
	"api/internal/application/helpers"
	getoffer "api/internal/application/query/get_offer"
	getpublishedoffer "api/internal/application/query/get_published_offer"
	"api/internal/domain/airport"
	"api/internal/domain/offer"
	"api/internal/domain/offer/flight"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// airportCardAirports — справочник: Екатеринбург с IATA, Москва без него.
func airportCardAirports() *mockAirportRepo {
	svx := "SVX"
	return &mockAirportRepo{findByICAOsAirports: []airport.Airport{
		{
			ICAO: "USSS", IATA: &svx, Name: "Кольцово",
			City:    airport.City{Name: "Екатеринбург", Timezone: "Asia/Yekaterinburg"},
			Country: airport.Country{ISO2: "RU", Name: "Россия"},
		},
		{
			ICAO: "UUEE", Name: "Шереметьево",
			City:    airport.City{Name: "Москва", Timezone: "Europe/Moscow"},
			Country: airport.Country{ISO2: "RU", Name: "Россия"},
		},
	}}
}

// airportCardFlights — Екатеринбург → Москва и обратно, в 3 аэропорта-записи на
// 2 разных кода: запрос к справочнику всё равно один.
func airportCardFlights() []flight.Flight {
	wall := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	return []flight.Flight{
		{Price: samplePrice(), Segments: []flight.Segment{
			{AirlineID: sampleAirlineID, DepartureAirportICAO: "USSS", ArrivalAirportICAO: "UUEE", DepartureAt: wall, ArrivalAt: wall.Add(time.Hour)},
		}},
		{Price: samplePrice(), Segments: []flight.Segment{
			{AirlineID: sampleAirlineID, DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "USSS", DepartureAt: wall.Add(24 * time.Hour), ArrivalAt: wall.Add(30 * time.Hour)},
		}},
	}
}

func TestFindSegmentAirports_NoSegments_NoQuery(t *testing.T) {
	airports := &mockAirportRepo{}

	got, err := helpers.FindSegmentAirports(context.Background(), airports, nil)

	require.NoError(t, err)
	assert.Empty(t, got)
	assert.Zero(t, airports.findCalls)
}

func TestFindSegmentAirports_OneQueryUniqueCodes(t *testing.T) {
	airports := airportCardAirports()

	got, err := helpers.FindSegmentAirports(context.Background(), airports, airportCardFlights())

	require.NoError(t, err)
	assert.Equal(t, 1, airports.findCalls)
	assert.Equal(t, []string{"USSS", "UUEE"}, airports.lastICAOs)
	assert.Equal(t, "Кольцово", got["USSS"].Name)
	assert.Equal(t, "Шереметьево", got["UUEE"].Name)
}

func TestFindSegmentAirports_CodeMissing_ErrorWithCode_NotAppError(t *testing.T) {
	airports := &mockAirportRepo{findByICAOsAirports: []airport.Airport{{ICAO: "USSS"}}}

	_, err := helpers.FindSegmentAirports(context.Background(), airports, airportCardFlights())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "UUEE")
	assert.NotErrorIs(t, err, apperror.ErrValidation)
	assert.NotErrorIs(t, err, apperror.ErrNotFound)
}

func TestFindSegmentAirports_RepositoryError_Wrapped(t *testing.T) {
	boom := errors.New("boom")
	airports := &mockAirportRepo{findByICAOsErr: boom}

	_, err := helpers.FindSegmentAirports(context.Background(), airports, airportCardFlights())

	assert.ErrorIs(t, err, boom)
}

func TestGetOffer_FlightsIncludeAirportsAndZonedDurations(t *testing.T) {
	o := offer.Offer{ID: 1, UUID: uuid.New(), AgencyID: 7, Status: offer.StatusDraft}
	mgr := offer.NewManager(&mockOfferRepo{findByUUIDOffer: &o}, &mockAgencyRepo{})
	airports := airportCardAirports()
	h := getoffer.NewHandler(mgr, stubFlightFinder{flights: airportCardFlights()}, &mockAirlineRepo{}, airports, userRecordWithAgency(7))

	res, err := h.Handle(context.Background(), getoffer.Query{UUID: o.UUID, CurrentUserUUID: uuid.New()})

	require.NoError(t, err)
	assert.Equal(t, 1, airports.findCalls, "один запрос на карточку")
	seg := res.Flights[0].Segments[0]
	assert.Equal(t, "Кольцово", seg.DepartureAirport.Name)
	require.NotNil(t, seg.DepartureAirport.IATA)
	assert.Equal(t, "SVX", *seg.DepartureAirport.IATA)
	assert.Equal(t, "Asia/Yekaterinburg", seg.DepartureAirport.City.Timezone)
	assert.Equal(t, "RU", seg.DepartureAirport.Country.ISO2)
	assert.Equal(t, "Шереметьево", seg.ArrivalAirport.Name)
	assert.Nil(t, seg.ArrivalAirport.IATA)
	// 10:00 по Екатеринбургу → 11:00 по Москве: 3 часа, а не 1.
	assert.EqualValues(t, 10800, seg.DurationSeconds)
	assert.EqualValues(t, 10800, res.Flights[0].TotalDurationSeconds)
}

func TestGetOffer_AirportMissing_ReturnsUnexpectedError(t *testing.T) {
	o := offer.Offer{ID: 1, UUID: uuid.New(), AgencyID: 7, Status: offer.StatusDraft}
	mgr := offer.NewManager(&mockOfferRepo{findByUUIDOffer: &o}, &mockAgencyRepo{})
	airports := &mockAirportRepo{findByICAOsAirports: []airport.Airport{}}
	h := getoffer.NewHandler(mgr, stubFlightFinder{flights: airportCardFlights()}, &mockAirlineRepo{}, airports, userRecordWithAgency(7))

	_, err := h.Handle(context.Background(), getoffer.Query{UUID: o.UUID, CurrentUserUUID: uuid.New()})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in the reference table")
}

func TestGetPublishedOffer_FlightsIncludeAirportsAndZonedDurations(t *testing.T) {
	o := &offer.Offer{ID: 1, UUID: uuid.New(), AgencyID: 7, Status: offer.StatusPublished}
	airports := airportCardAirports()
	h := getpublishedoffer.NewHandler(stubOfferFinder{offer: o}, stubFlightFinder{flights: airportCardFlights()}, &mockAirlineRepo{}, airports)

	res, err := h.Handle(context.Background(), getpublishedoffer.Query{UUID: o.UUID})

	require.NoError(t, err)
	assert.Equal(t, 1, airports.findCalls, "один запрос на карточку")
	seg := res.Flights[0].Segments[0]
	assert.Equal(t, "Кольцово", seg.DepartureAirport.Name)
	assert.Equal(t, "Шереметьево", seg.ArrivalAirport.Name)
	assert.EqualValues(t, 10800, seg.DurationSeconds)
}
