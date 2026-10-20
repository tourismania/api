package integration_test

import (
	"context"
	"testing"
	"time"

	"api/internal/domain/airline"
	"api/internal/domain/offer/flight"
	"api/internal/infrastructure/persistence/postgres/db"
	pgrepo "api/internal/infrastructure/persistence/postgres/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Авиакомпания сегмента перелёта в хранилище (изменение add-flight-segment-airline).

func TestOfferFlightRepository_SegmentAirlines_StoredAndReadPerSegment(t *testing.T) {
	pool := newOfferFlightTestPool(t)
	airlines := pgrepo.NewAirlineRepository(db.New(pool))
	f := seedAirlines(t, pool, airlines)
	repo := pgrepo.NewOfferFlightRepository(db.New(pool))
	offerID := seedTestOfferID(t, pool)
	t.Cleanup(func() { assert.NoError(t, repo.ReplaceForOffer(context.Background(), offerID, nil)) })
	a, b, c := seedTestAirport(t, pool), seedTestAirport(t, pool), seedTestAirport(t, pool)
	base := time.Now().UTC().Truncate(time.Second)

	require.NoError(t, repo.ReplaceForOffer(context.Background(), offerID, []flight.Flight{{Price: testFlightPrice(t), Segments: []flight.Segment{
		{DepartureAirportICAO: a, ArrivalAirportICAO: b, DepartureAt: base, ArrivalAt: base.Add(3 * time.Hour), AirlineID: f.Alpha},
		{DepartureAirportICAO: b, ArrivalAirportICAO: c, DepartureAt: base.Add(5 * time.Hour), ArrivalAt: base.Add(8 * time.Hour), AirlineID: airline.UnknownID},
	}}}))

	found, err := repo.FindByOfferID(context.Background(), offerID)
	require.NoError(t, err)
	require.Len(t, found, 1)
	require.Len(t, found[0].Segments, 2)
	assert.Equal(t, f.Alpha, found[0].Segments[0].AirlineID)
	assert.Equal(t, airline.UnknownID, found[0].Segments[1].AirlineID)
}

func TestOfferFlightRepository_UpdateDetails_ChangesAirline_KeepsIDsAirportsTimesAndClass(t *testing.T) {
	pool := newOfferFlightTestPool(t)
	airlines := pgrepo.NewAirlineRepository(db.New(pool))
	f := seedAirlines(t, pool, airlines)
	repo := pgrepo.NewOfferFlightRepository(db.New(pool))
	offerID := seedTestOfferID(t, pool)
	t.Cleanup(func() { assert.NoError(t, repo.ReplaceForOffer(context.Background(), offerID, nil)) })
	dep, arr := seedTestAirport(t, pool), seedTestAirport(t, pool)
	base := time.Now().UTC().Truncate(time.Second)

	require.NoError(t, repo.ReplaceForOffer(context.Background(), offerID, []flight.Flight{{Price: testFlightPrice(t), Segments: []flight.Segment{
		{DepartureAirportICAO: dep, ArrivalAirportICAO: arr, DepartureAt: base, ArrivalAt: base.Add(3 * time.Hour), CabinClass: flight.CabinClassBusiness, AirlineID: f.Alpha},
	}}}))
	before, err := repo.FindByOfferID(context.Background(), offerID)
	require.NoError(t, err)

	require.NoError(t, repo.UpdateDetails(context.Background(), offerID, []flight.DetailsUpdate{{
		FlightID:            before[0].ID,
		Price:               before[0].Price,
		SegmentCabinClasses: []flight.CabinClass{flight.CabinClassBusiness},
		SegmentAirlineIDs:   []int{airline.UnknownID},
	}}))

	after, err := repo.FindByOfferID(context.Background(), offerID)
	require.NoError(t, err)
	require.Len(t, after, 1)
	assert.Equal(t, before[0].ID, after[0].ID)
	want := before[0].Segments[0]
	got := after[0].Segments[0]
	assert.Equal(t, airline.UnknownID, got.AirlineID)
	assert.Equal(t, want.DepartureAirportICAO, got.DepartureAirportICAO)
	assert.Equal(t, want.ArrivalAirportICAO, got.ArrivalAirportICAO)
	assert.True(t, want.DepartureAt.Equal(got.DepartureAt))
	assert.True(t, want.ArrivalAt.Equal(got.ArrivalAt))
	assert.Equal(t, flight.CabinClassBusiness, got.CabinClass)
}

func TestOfferFlightRepository_UnknownAirlineID_ReturnsFKError(t *testing.T) {
	pool := newOfferFlightTestPool(t)
	repo := pgrepo.NewOfferFlightRepository(db.New(pool))
	offerID := seedTestOfferID(t, pool)
	dep, arr := seedTestAirport(t, pool), seedTestAirport(t, pool)
	base := time.Now().UTC().Truncate(time.Second)

	err := repo.ReplaceForOffer(context.Background(), offerID, []flight.Flight{{Price: testFlightPrice(t), Segments: []flight.Segment{
		{DepartureAirportICAO: dep, ArrivalAirportICAO: arr, DepartureAt: base, ArrivalAt: base.Add(time.Hour), AirlineID: 999_999_999},
	}}})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "offer_flight_segments_airline_id_fkey")
}

func TestOfferFlightRepository_UpdateDetails_MismatchedSegmentLists_Error(t *testing.T) {
	pool := newOfferFlightTestPool(t)
	repo := pgrepo.NewOfferFlightRepository(db.New(pool))

	err := repo.UpdateDetails(context.Background(), 1, []flight.DetailsUpdate{{
		FlightID: 1, SegmentCabinClasses: []flight.CabinClass{flight.CabinClassUnknown},
	}})

	require.Error(t, err)
}
