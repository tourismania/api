package integration_test

import (
	"context"
	"testing"
	"time"

	"api/internal/domain/entity"
	"api/internal/domain/enum"
	"api/internal/infrastructure/persistence/postgres"
	"api/internal/infrastructure/persistence/postgres/db"
	pgrepo "api/internal/infrastructure/persistence/postgres/repository"
	pgtxmanager "api/internal/infrastructure/persistence/postgres/txmanager"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newOfferFlightTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, testDB(t))
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

// seedTestAirport creates a fresh country/city/airport chain and returns
// the new airport's icao, so each test operates on isolated fixtures
// regardless of whatever real airport data the target database already
// has (or doesn't have).
func seedTestAirport(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx := context.Background()

	countryRepo := pgrepo.NewCountryRepository(pool)
	require.NoError(t, countryRepo.Upsert(ctx, "ZZ", "Test Country"))

	cityRepo := pgrepo.NewCityRepository(pool)
	cityID, err := cityRepo.Upsert(ctx, "Test City "+uuid.NewString(), nil, "UTC", "ZZ")
	require.NoError(t, err)

	icao := "T" + uuid.NewString()[:3]
	airportRepo := pgrepo.NewAirportRepository(db.New(pool), pool)
	require.NoError(t, airportRepo.Upsert(ctx, icao, nil, "Test Airport", 0, 0, nil, cityID))

	return icao
}

// seedTestOffer creates a fresh agency/user/offer chain and returns the
// offer's database id, for tests that need an owning offer to attach
// flights to.
func seedTestOfferID(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	ctx := context.Background()

	queries := db.New(pool)
	offerRepo := pgrepo.NewOfferRepository(queries)
	agencyRepo := pgrepo.NewAgencyRepository(queries)
	userRepo := pgrepo.NewUserRepository(queries)

	agencyID, userID := seedAgencyAndUser(t, agencyRepo, userRepo)
	offer := newTestOffer(agencyID, userID, enum.OfferStatusDraft)
	id, err := offerRepo.Store(ctx, offer)
	require.NoError(t, err)
	return id
}

func TestOfferFlightRepository_FindByOfferID_NoFlights_ReturnsNil(t *testing.T) {
	pool := newOfferFlightTestPool(t)
	repo := pgrepo.NewOfferFlightRepository(db.New(pool))
	offerID := seedTestOfferID(t, pool)

	flights, err := repo.FindByOfferID(context.Background(), offerID)

	require.NoError(t, err)
	assert.Nil(t, flights)
}

func TestOfferFlightRepository_ReplaceForOffer_StoreThenFindByOfferID_ReturnsStoredFlights(t *testing.T) {
	pool := newOfferFlightTestPool(t)
	repo := pgrepo.NewOfferFlightRepository(db.New(pool))
	offerID := seedTestOfferID(t, pool)

	dep := seedTestAirport(t, pool)
	arr := seedTestAirport(t, pool)
	base := time.Now().UTC().Truncate(time.Second)

	flights := []entity.Flight{
		{Segments: []entity.FlightSegment{
			{DepartureAirportICAO: dep, ArrivalAirportICAO: arr, DepartureAt: base, ArrivalAt: base.Add(3 * time.Hour)},
		}},
	}

	require.NoError(t, repo.ReplaceForOffer(context.Background(), offerID, flights))

	found, err := repo.FindByOfferID(context.Background(), offerID)
	require.NoError(t, err)
	require.Len(t, found, 1)
	require.Len(t, found[0].Segments, 1)
	assert.Equal(t, dep, found[0].Segments[0].DepartureAirportICAO)
	assert.Equal(t, arr, found[0].Segments[0].ArrivalAirportICAO)
	assert.WithinDuration(t, base, found[0].Segments[0].DepartureAt, time.Second)
	assert.Equal(t, offerID, found[0].OfferID)
}

func TestOfferFlightRepository_ReplaceForOffer_PreservesSegmentOrder(t *testing.T) {
	pool := newOfferFlightTestPool(t)
	repo := pgrepo.NewOfferFlightRepository(db.New(pool))
	offerID := seedTestOfferID(t, pool)

	a := seedTestAirport(t, pool)
	b := seedTestAirport(t, pool)
	c := seedTestAirport(t, pool)
	base := time.Now().UTC().Truncate(time.Second)

	flights := []entity.Flight{
		{Segments: []entity.FlightSegment{
			{DepartureAirportICAO: a, ArrivalAirportICAO: b, DepartureAt: base, ArrivalAt: base.Add(2 * time.Hour)},
			{DepartureAirportICAO: b, ArrivalAirportICAO: c, DepartureAt: base.Add(3 * time.Hour), ArrivalAt: base.Add(5 * time.Hour)},
		}},
	}
	require.NoError(t, repo.ReplaceForOffer(context.Background(), offerID, flights))

	found, err := repo.FindByOfferID(context.Background(), offerID)
	require.NoError(t, err)
	require.Len(t, found, 1)
	require.Len(t, found[0].Segments, 2)
	assert.Equal(t, a, found[0].Segments[0].DepartureAirportICAO)
	assert.Equal(t, b, found[0].Segments[0].ArrivalAirportICAO)
	assert.Equal(t, b, found[0].Segments[1].DepartureAirportICAO)
	assert.Equal(t, c, found[0].Segments[1].ArrivalAirportICAO)
}

func TestOfferFlightRepository_ReplaceForOffer_ReplacesPreviousSet(t *testing.T) {
	pool := newOfferFlightTestPool(t)
	repo := pgrepo.NewOfferFlightRepository(db.New(pool))
	offerID := seedTestOfferID(t, pool)

	dep := seedTestAirport(t, pool)
	arr := seedTestAirport(t, pool)
	newArr := seedTestAirport(t, pool)
	base := time.Now().UTC().Truncate(time.Second)

	first := []entity.Flight{{Segments: []entity.FlightSegment{
		{DepartureAirportICAO: dep, ArrivalAirportICAO: arr, DepartureAt: base, ArrivalAt: base.Add(time.Hour)},
	}}}
	require.NoError(t, repo.ReplaceForOffer(context.Background(), offerID, first))

	second := []entity.Flight{{Segments: []entity.FlightSegment{
		{DepartureAirportICAO: dep, ArrivalAirportICAO: newArr, DepartureAt: base, ArrivalAt: base.Add(2 * time.Hour)},
	}}}
	require.NoError(t, repo.ReplaceForOffer(context.Background(), offerID, second))

	found, err := repo.FindByOfferID(context.Background(), offerID)
	require.NoError(t, err)
	require.Len(t, found, 1)
	require.Len(t, found[0].Segments, 1)
	assert.Equal(t, newArr, found[0].Segments[0].ArrivalAirportICAO, "the old flight must be fully replaced, not appended to")
}

func TestOfferFlightRepository_ReplaceForOffer_EmptySet_ClearsFlights(t *testing.T) {
	pool := newOfferFlightTestPool(t)
	repo := pgrepo.NewOfferFlightRepository(db.New(pool))
	offerID := seedTestOfferID(t, pool)

	dep := seedTestAirport(t, pool)
	arr := seedTestAirport(t, pool)
	base := time.Now().UTC().Truncate(time.Second)

	require.NoError(t, repo.ReplaceForOffer(context.Background(), offerID, []entity.Flight{{Segments: []entity.FlightSegment{
		{DepartureAirportICAO: dep, ArrivalAirportICAO: arr, DepartureAt: base, ArrivalAt: base.Add(time.Hour)},
	}}}))

	require.NoError(t, repo.ReplaceForOffer(context.Background(), offerID, nil))

	found, err := repo.FindByOfferID(context.Background(), offerID)
	require.NoError(t, err)
	assert.Empty(t, found)
}

func TestOfferFlightRepository_ReplaceForOffer_UnknownAirportICAO_ReturnsFKError(t *testing.T) {
	pool := newOfferFlightTestPool(t)
	repo := pgrepo.NewOfferFlightRepository(db.New(pool))
	offerID := seedTestOfferID(t, pool)
	base := time.Now().UTC().Truncate(time.Second)

	flights := []entity.Flight{{Segments: []entity.FlightSegment{
		{DepartureAirportICAO: "ZZZZ", ArrivalAirportICAO: "YYYY", DepartureAt: base, ArrivalAt: base.Add(time.Hour)},
	}}}

	err := repo.ReplaceForOffer(context.Background(), offerID, flights)
	assert.Error(t, err)
}

// TestOfferFlightRepository_WithinRealTransaction_RollsBackOnError
// exercises the actual atomicity guarantee ReplaceForOffer relies on:
// when the calling handler's txmanager.Manager.WithinTx fails after the
// flights write, the flights insert is rolled back along with
// everything else in that unit of work.
func TestOfferFlightRepository_WithinRealTransaction_RollsBackOnError(t *testing.T) {
	pool := newOfferFlightTestPool(t)
	repo := pgrepo.NewOfferFlightRepository(db.New(pool))
	offerID := seedTestOfferID(t, pool)
	dep := seedTestAirport(t, pool)
	arr := seedTestAirport(t, pool)
	base := time.Now().UTC().Truncate(time.Second)

	txMgr := pgtxmanager.New(pool)
	sentinelErr := assert.AnError

	err := txMgr.WithinTx(context.Background(), func(txCtx context.Context) error {
		flights := []entity.Flight{{Segments: []entity.FlightSegment{
			{DepartureAirportICAO: dep, ArrivalAirportICAO: arr, DepartureAt: base, ArrivalAt: base.Add(time.Hour)},
		}}}
		if err := repo.ReplaceForOffer(txCtx, offerID, flights); err != nil {
			return err
		}
		return sentinelErr
	})

	assert.ErrorIs(t, err, sentinelErr)

	found, findErr := repo.FindByOfferID(context.Background(), offerID)
	require.NoError(t, findErr)
	assert.Empty(t, found, "the flights insert must have been rolled back with the rest of the transaction")
}

func TestOfferFlightRepository_WithinRealTransaction_CommitsOnSuccess(t *testing.T) {
	pool := newOfferFlightTestPool(t)
	repo := pgrepo.NewOfferFlightRepository(db.New(pool))
	offerID := seedTestOfferID(t, pool)
	dep := seedTestAirport(t, pool)
	arr := seedTestAirport(t, pool)
	base := time.Now().UTC().Truncate(time.Second)

	txMgr := pgtxmanager.New(pool)

	err := txMgr.WithinTx(context.Background(), func(txCtx context.Context) error {
		flights := []entity.Flight{{Segments: []entity.FlightSegment{
			{DepartureAirportICAO: dep, ArrivalAirportICAO: arr, DepartureAt: base, ArrivalAt: base.Add(time.Hour)},
		}}}
		return repo.ReplaceForOffer(txCtx, offerID, flights)
	})
	require.NoError(t, err)

	found, findErr := repo.FindByOfferID(context.Background(), offerID)
	require.NoError(t, findErr)
	require.Len(t, found, 1)
}
