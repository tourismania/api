package integration_test

import (
	"context"
	"testing"
	"time"

	"api/internal/domain/money"
	"api/internal/domain/offer"
	"api/internal/domain/offer/flight"
	"api/internal/infrastructure/persistence/postgres"
	"api/internal/infrastructure/persistence/postgres/db"
	pgrepo "api/internal/infrastructure/persistence/postgres/repository"
	pgtxmanager "api/internal/infrastructure/persistence/postgres/txmanager"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testFlightPrice — валидная цена по умолчанию для тестовых перелётов.
func testFlightPrice(t *testing.T) money.Money {
	t.Helper()
	m, err := money.Parse("15000", money.CodeRUB)
	require.NoError(t, err)
	return m
}

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
	o := newTestOffer(agencyID, userID, offer.StatusDraft)
	id, err := offerRepo.Store(ctx, o)
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

	flights := []flight.Flight{
		{Price: testFlightPrice(t), Segments: []flight.Segment{
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

	flights := []flight.Flight{
		{Price: testFlightPrice(t), Segments: []flight.Segment{
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

	first := []flight.Flight{{Price: testFlightPrice(t), Segments: []flight.Segment{
		{DepartureAirportICAO: dep, ArrivalAirportICAO: arr, DepartureAt: base, ArrivalAt: base.Add(time.Hour)},
	}}}
	require.NoError(t, repo.ReplaceForOffer(context.Background(), offerID, first))

	second := []flight.Flight{{Price: testFlightPrice(t), Segments: []flight.Segment{
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

	require.NoError(t, repo.ReplaceForOffer(context.Background(), offerID, []flight.Flight{{Price: testFlightPrice(t), Segments: []flight.Segment{
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

	flights := []flight.Flight{{Price: testFlightPrice(t), Segments: []flight.Segment{
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
		flights := []flight.Flight{{Price: testFlightPrice(t), Segments: []flight.Segment{
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
		flights := []flight.Flight{{Price: testFlightPrice(t), Segments: []flight.Segment{
			{DepartureAirportICAO: dep, ArrivalAirportICAO: arr, DepartureAt: base, ArrivalAt: base.Add(time.Hour)},
		}}}
		return repo.ReplaceForOffer(txCtx, offerID, flights)
	})
	require.NoError(t, err)

	found, findErr := repo.FindByOfferID(context.Background(), offerID)
	require.NoError(t, findErr)
	require.Len(t, found, 1)
}

func TestOfferFlightRepository_ReplaceForOffer_StoresAndReadsPrice(t *testing.T) {
	pool := newOfferFlightTestPool(t)
	repo := pgrepo.NewOfferFlightRepository(db.New(pool))
	offerID := seedTestOfferID(t, pool)
	dep, arr := seedTestAirport(t, pool), seedTestAirport(t, pool)
	base := time.Now().UTC().Truncate(time.Second)

	price, err := money.Parse("12345.5", money.CodeEUR)
	require.NoError(t, err)
	require.NoError(t, repo.ReplaceForOffer(context.Background(), offerID, []flight.Flight{
		{Price: price, Segments: []flight.Segment{
			{DepartureAirportICAO: dep, ArrivalAirportICAO: arr, DepartureAt: base, ArrivalAt: base.Add(3 * time.Hour)},
		}},
	}))

	found, err := repo.FindByOfferID(context.Background(), offerID)
	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.Equal(t, price, found[0].Price)
	assert.Equal(t, "12345.50", found[0].Price.String())
}

func TestOfferFlightRepository_UpdateDetails_KeepsIDsAndSegments(t *testing.T) {
	pool := newOfferFlightTestPool(t)
	repo := pgrepo.NewOfferFlightRepository(db.New(pool))
	offerID := seedTestOfferID(t, pool)
	dep, arr := seedTestAirport(t, pool), seedTestAirport(t, pool)
	base := time.Now().UTC().Truncate(time.Second)

	require.NoError(t, repo.ReplaceForOffer(context.Background(), offerID, []flight.Flight{
		{Price: testFlightPrice(t), Segments: []flight.Segment{
			{DepartureAirportICAO: dep, ArrivalAirportICAO: arr, DepartureAt: base, ArrivalAt: base.Add(3 * time.Hour)},
		}},
		{Price: testFlightPrice(t), Segments: []flight.Segment{
			{DepartureAirportICAO: arr, ArrivalAirportICAO: dep, DepartureAt: base.Add(48 * time.Hour), ArrivalAt: base.Add(51 * time.Hour)},
		}},
	}))
	before, err := repo.FindByOfferID(context.Background(), offerID)
	require.NoError(t, err)
	require.Len(t, before, 2)

	newPrice, err := money.Parse("99.99", money.CodeUSD)
	require.NoError(t, err)
	require.NoError(t, repo.UpdateDetails(context.Background(), offerID, []flight.DetailsUpdate{
		{FlightID: before[1].ID, Price: newPrice},
	}))

	after, err := repo.FindByOfferID(context.Background(), offerID)
	require.NoError(t, err)
	require.Len(t, after, 2)
	assert.Equal(t, before[0].ID, after[0].ID)
	assert.Equal(t, before[1].ID, after[1].ID)
	assert.Equal(t, before[0].Price, after[0].Price, "цена другого перелёта не меняется")
	assert.Equal(t, newPrice, after[1].Price)
	assert.Equal(t, before[1].Segments, after[1].Segments)
}

func TestOfferFlightRepository_ManagerComment_StoredAndReadAsIs(t *testing.T) {
	pool := newOfferFlightTestPool(t)
	repo := pgrepo.NewOfferFlightRepository(db.New(pool))
	offerID := seedTestOfferID(t, pool)
	dep, arr := seedTestAirport(t, pool), seedTestAirport(t, pool)
	base := time.Now().UTC().Truncate(time.Second)

	comments := []string{
		"Багаж оплачивается отдельно",
		"Строка 1\nСтрока 2\r\n\tс отступом ",
		`Rock'n'roll \ "VIP"'; DROP TABLE offers; --`,
		"",
	}
	flights := make([]flight.Flight, 0, len(comments))
	for i, c := range comments {
		at := base.Add(time.Duration(i) * 24 * time.Hour)
		flights = append(flights, flight.Flight{Price: testFlightPrice(t), ManagerComment: c, Segments: []flight.Segment{
			{DepartureAirportICAO: dep, ArrivalAirportICAO: arr, DepartureAt: at, ArrivalAt: at.Add(3 * time.Hour)},
		}})
	}
	require.NoError(t, repo.ReplaceForOffer(context.Background(), offerID, flights))

	found, err := repo.FindByOfferID(context.Background(), offerID)
	require.NoError(t, err)
	require.Len(t, found, len(comments))
	for i, c := range comments {
		assert.Equal(t, c, found[i].ManagerComment)
	}
}

func TestOfferFlightRepository_UpdateDetails_ChangesPriceAndComment_KeepsIDsAndSegments(t *testing.T) {
	pool := newOfferFlightTestPool(t)
	repo := pgrepo.NewOfferFlightRepository(db.New(pool))
	offerID := seedTestOfferID(t, pool)
	dep, arr := seedTestAirport(t, pool), seedTestAirport(t, pool)
	base := time.Now().UTC().Truncate(time.Second)

	require.NoError(t, repo.ReplaceForOffer(context.Background(), offerID, []flight.Flight{
		{Price: testFlightPrice(t), ManagerComment: "первый", Segments: []flight.Segment{
			{DepartureAirportICAO: dep, ArrivalAirportICAO: arr, DepartureAt: base, ArrivalAt: base.Add(3 * time.Hour)},
		}},
		{Price: testFlightPrice(t), ManagerComment: "второй", Segments: []flight.Segment{
			{DepartureAirportICAO: arr, ArrivalAirportICAO: dep, DepartureAt: base.Add(48 * time.Hour), ArrivalAt: base.Add(51 * time.Hour)},
		}},
	}))
	before, err := repo.FindByOfferID(context.Background(), offerID)
	require.NoError(t, err)
	require.Len(t, before, 2)

	newPrice, err := money.Parse("99.99", money.CodeUSD)
	require.NoError(t, err)
	require.NoError(t, repo.UpdateDetails(context.Background(), offerID, []flight.DetailsUpdate{
		{FlightID: before[0].ID, Price: before[0].Price, ManagerComment: ""},
		{FlightID: before[1].ID, Price: newPrice, ManagerComment: "Ночная пересадка"},
	}))

	after, err := repo.FindByOfferID(context.Background(), offerID)
	require.NoError(t, err)
	require.Len(t, after, 2)
	assert.Equal(t, before[0].ID, after[0].ID)
	assert.Equal(t, before[1].ID, after[1].ID)
	assert.Equal(t, "", after[0].ManagerComment, "комментарий удалён")
	assert.Equal(t, before[0].Price, after[0].Price)
	assert.Equal(t, newPrice, after[1].Price)
	assert.Equal(t, "Ночная пересадка", after[1].ManagerComment)
	assert.Equal(t, before[0].Segments, after[0].Segments)
	assert.Equal(t, before[1].Segments, after[1].Segments)
}

func TestOfferFlightRepository_UpdateDetails_OtherOfferFlight_NotChanged(t *testing.T) {
	pool := newOfferFlightTestPool(t)
	repo := pgrepo.NewOfferFlightRepository(db.New(pool))
	ownerID := seedTestOfferID(t, pool)
	otherID := seedTestOfferID(t, pool)
	dep, arr := seedTestAirport(t, pool), seedTestAirport(t, pool)
	base := time.Now().UTC().Truncate(time.Second)

	require.NoError(t, repo.ReplaceForOffer(context.Background(), ownerID, []flight.Flight{
		{Price: testFlightPrice(t), Segments: []flight.Segment{
			{DepartureAirportICAO: dep, ArrivalAirportICAO: arr, DepartureAt: base, ArrivalAt: base.Add(3 * time.Hour)},
		}},
	}))
	owned, err := repo.FindByOfferID(context.Background(), ownerID)
	require.NoError(t, err)
	require.Len(t, owned, 1)

	newPrice, err := money.Parse("1", money.CodeUSD)
	require.NoError(t, err)
	// Чужой offerID: перелёт ownerID и классы его сегментов обновляться не должны.
	require.NoError(t, repo.UpdateDetails(context.Background(), otherID, []flight.DetailsUpdate{
		{FlightID: owned[0].ID, Price: newPrice, SegmentCabinClasses: []flight.CabinClass{flight.CabinClassBusiness}},
	}))

	after, err := repo.FindByOfferID(context.Background(), ownerID)
	require.NoError(t, err)
	assert.Equal(t, owned[0].Price, after[0].Price)
	assert.Equal(t, flight.CabinClassUnknown, after[0].Segments[0].CabinClass)
}

func TestOfferFlightRepository_FindByOfferID_UnknownStoredCurrency_ReturnsNonValidationErr(t *testing.T) {
	pool := newOfferFlightTestPool(t)
	repo := pgrepo.NewOfferFlightRepository(db.New(pool))
	offerID := seedTestOfferID(t, pool)

	// CHECK на валюту в БД нет — запись в обход api может оставить
	// неизвестный код; это порча данных, а не ошибка клиента.
	_, err := pool.Exec(context.Background(),
		`INSERT INTO offer_flights (offer_id, price_minor, currency) VALUES ($1, 100, 'GBP')`, offerID)
	require.NoError(t, err)

	_, err = repo.FindByOfferID(context.Background(), offerID)
	require.Error(t, err)
	assert.NotErrorIs(t, err, money.ErrCurrencyUnsupported, "не должна превращаться в 400")
}

func TestOfferFlightRepository_CabinClass_StoredAndReadPerSegment(t *testing.T) {
	pool := newOfferFlightTestPool(t)
	repo := pgrepo.NewOfferFlightRepository(db.New(pool))
	offerID := seedTestOfferID(t, pool)
	a, b, c := seedTestAirport(t, pool), seedTestAirport(t, pool), seedTestAirport(t, pool)
	base := time.Now().UTC().Truncate(time.Second)

	require.NoError(t, repo.ReplaceForOffer(context.Background(), offerID, []flight.Flight{
		{Price: testFlightPrice(t), Segments: []flight.Segment{
			{DepartureAirportICAO: a, ArrivalAirportICAO: b, DepartureAt: base, ArrivalAt: base.Add(3 * time.Hour), CabinClass: flight.CabinClassComfortPlus},
			{DepartureAirportICAO: b, ArrivalAirportICAO: c, DepartureAt: base.Add(5 * time.Hour), ArrivalAt: base.Add(8 * time.Hour)},
			{DepartureAirportICAO: c, ArrivalAirportICAO: a, DepartureAt: base.Add(10 * time.Hour), ArrivalAt: base.Add(13 * time.Hour), CabinClass: flight.CabinClassBusiness},
		}},
	}))

	got, err := repo.FindByOfferID(context.Background(), offerID)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Len(t, got[0].Segments, 3)
	assert.Equal(t, flight.CabinClassComfortPlus, got[0].Segments[0].CabinClass)
	assert.Equal(t, flight.CabinClassUnknown, got[0].Segments[1].CabinClass)
	assert.Equal(t, flight.CabinClassBusiness, got[0].Segments[2].CabinClass)
}

func TestOfferFlightRepository_UpdateDetails_ChangesCabinClasses_KeepsIDsAirportsAndTimes(t *testing.T) {
	pool := newOfferFlightTestPool(t)
	repo := pgrepo.NewOfferFlightRepository(db.New(pool))
	offerID := seedTestOfferID(t, pool)
	dep, mid, arr := seedTestAirport(t, pool), seedTestAirport(t, pool), seedTestAirport(t, pool)
	base := time.Now().UTC().Truncate(time.Second)

	require.NoError(t, repo.ReplaceForOffer(context.Background(), offerID, []flight.Flight{
		{Price: testFlightPrice(t), Segments: []flight.Segment{
			{DepartureAirportICAO: dep, ArrivalAirportICAO: mid, DepartureAt: base, ArrivalAt: base.Add(3 * time.Hour), CabinClass: flight.CabinClassBusiness},
			{DepartureAirportICAO: mid, ArrivalAirportICAO: arr, DepartureAt: base.Add(5 * time.Hour), ArrivalAt: base.Add(8 * time.Hour), CabinClass: flight.CabinClassEconomy},
		}},
		{Price: testFlightPrice(t), Segments: []flight.Segment{
			{DepartureAirportICAO: arr, ArrivalAirportICAO: dep, DepartureAt: base.Add(48 * time.Hour), ArrivalAt: base.Add(51 * time.Hour), CabinClass: flight.CabinClassComfort},
		}},
	}))
	before, err := repo.FindByOfferID(context.Background(), offerID)
	require.NoError(t, err)
	require.Len(t, before, 2)

	// Классы первого перелёта: бизнес → неизвестен, эконом → комфорт плюс.
	require.NoError(t, repo.UpdateDetails(context.Background(), offerID, []flight.DetailsUpdate{
		{FlightID: before[0].ID, Price: before[0].Price, SegmentCabinClasses: []flight.CabinClass{flight.CabinClassUnknown, flight.CabinClassComfortPlus}},
	}))

	after, err := repo.FindByOfferID(context.Background(), offerID)
	require.NoError(t, err)
	require.Len(t, after, 2)
	assert.Equal(t, before[0].ID, after[0].ID)
	assert.Equal(t, before[1].ID, after[1].ID)
	require.Len(t, after[0].Segments, 2)
	assert.Equal(t, flight.CabinClassUnknown, after[0].Segments[0].CabinClass)
	assert.Equal(t, flight.CabinClassComfortPlus, after[0].Segments[1].CabinClass)
	for i := range before[0].Segments {
		want, got := before[0].Segments[i], after[0].Segments[i]
		assert.Equal(t, want.DepartureAirportICAO, got.DepartureAirportICAO)
		assert.Equal(t, want.ArrivalAirportICAO, got.ArrivalAirportICAO)
		assert.True(t, want.DepartureAt.Equal(got.DepartureAt))
		assert.True(t, want.ArrivalAt.Equal(got.ArrivalAt))
	}
	assert.Equal(t, before[1].Segments, after[1].Segments, "другой перелёт не меняется")
}
