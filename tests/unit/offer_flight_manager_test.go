package unit_test

import (
	"api/internal/domain/airport"
	"api/internal/domain/money"
	"api/internal/domain/offer/flight"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleFlight(dep, arr string, base time.Time) flight.Flight {
	return flight.Flight{Price: samplePrice(), Segments: []flight.Segment{
		{DepartureAirportICAO: dep, ArrivalAirportICAO: arr, DepartureAt: base, ArrivalAt: base.Add(2 * time.Hour)},
	}}
}

// withPrice возвращает копию перелёта с другой ценой и ID.
func withPrice(f flight.Flight, id int, price money.Money) flight.Flight {
	f.ID = id
	f.Price = price
	return f
}

func TestOfferFlightManager_ReplaceForOffer_IdenticalContent_IsNoOp(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	existing := []flight.Flight{sampleFlight("UUEE", "LFPG", base)}
	flights := &mockOfferFlightRepo{findByOfferIDFlights: existing}
	mgr := flight.NewManager(flights, &mockAirportRepo{})

	err := mgr.ReplaceForOffer(context.Background(), 1, []flight.Flight{sampleFlight("UUEE", "LFPG", base)})

	require.NoError(t, err)
	assert.False(t, flights.replaceCalled, "identical content must not touch the database")
	assert.False(t, flights.updateDetailsCalled, "identical prices must not touch the database")
}

func TestOfferFlightManager_ReplaceForOffer_SamePriceDifferentNotation_IsNoOp(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	stored := withPrice(sampleFlight("UUEE", "LFPG", base), 7, mustMoney("100.00", money.CodeRUB))
	flights := &mockOfferFlightRepo{findByOfferIDFlights: []flight.Flight{stored}}
	mgr := flight.NewManager(flights, &mockAirportRepo{})

	incoming := withPrice(sampleFlight("UUEE", "LFPG", base), 0, mustMoney("100", money.CodeRUB))
	err := mgr.ReplaceForOffer(context.Background(), 1, []flight.Flight{incoming})

	require.NoError(t, err)
	assert.False(t, flights.replaceCalled)
	assert.False(t, flights.updateDetailsCalled)
}

func TestOfferFlightManager_ReplaceForOffer_OnlyPriceChanged_UpdatesPricesKeepsIDs(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	existing := []flight.Flight{
		withPrice(sampleFlight("UUEE", "LFPG", base), 7, mustMoney("100", money.CodeRUB)),
		withPrice(sampleFlight("LFPG", "UUEE", base.Add(48*time.Hour)), 8, mustMoney("200", money.CodeRUB)),
	}
	flights := &mockOfferFlightRepo{findByOfferIDFlights: existing}
	mgr := flight.NewManager(flights, &mockAirportRepo{})

	// Маршрут тот же, у второго перелёта другая цена.
	incoming := []flight.Flight{
		withPrice(sampleFlight("UUEE", "LFPG", base), 0, mustMoney("100", money.CodeRUB)),
		withPrice(sampleFlight("LFPG", "UUEE", base.Add(48*time.Hour)), 0, mustMoney("250.50", money.CodeRUB)),
	}
	err := mgr.ReplaceForOffer(context.Background(), 1, incoming)

	require.NoError(t, err)
	assert.False(t, flights.replaceCalled, "price-only change must not replace the set")
	require.True(t, flights.updateDetailsCalled)
	assert.Equal(t, 1, flights.updateDetailsOfferID)
	assert.Equal(t, []flight.DetailsUpdate{{FlightID: 8, Price: mustMoney("250.50", money.CodeRUB)}}, flights.updatedDetails)
}

func TestOfferFlightManager_ReplaceForOffer_OnlyCurrencyChanged_UpdatesPrices(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	existing := []flight.Flight{withPrice(sampleFlight("UUEE", "LFPG", base), 7, mustMoney("100", money.CodeRUB))}
	flights := &mockOfferFlightRepo{findByOfferIDFlights: existing}
	mgr := flight.NewManager(flights, &mockAirportRepo{})

	incoming := []flight.Flight{withPrice(sampleFlight("UUEE", "LFPG", base), 0, mustMoney("100", money.CodeEUR))}
	err := mgr.ReplaceForOffer(context.Background(), 1, incoming)

	require.NoError(t, err)
	assert.False(t, flights.replaceCalled)
	assert.Equal(t, []flight.DetailsUpdate{{FlightID: 7, Price: mustMoney("100", money.CodeEUR)}}, flights.updatedDetails)
}

// withComment возвращает копию перелёта с другим комментарием менеджера и ID.
func withComment(f flight.Flight, id int, comment string) flight.Flight {
	f.ID = id
	f.ManagerComment = comment
	return f
}

func TestOfferFlightManager_ReplaceForOffer_SameRoutePriceAndComment_IsNoOp(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	stored := withComment(sampleFlight("UUEE", "LFPG", base), 7, "Багаж оплачивается отдельно")
	flights := &mockOfferFlightRepo{findByOfferIDFlights: []flight.Flight{stored}}
	mgr := flight.NewManager(flights, &mockAirportRepo{})

	incoming := withComment(sampleFlight("UUEE", "LFPG", base), 0, "Багаж оплачивается отдельно")
	err := mgr.ReplaceForOffer(context.Background(), 1, []flight.Flight{incoming})

	require.NoError(t, err)
	assert.False(t, flights.replaceCalled)
	assert.False(t, flights.updateDetailsCalled)
}

func TestOfferFlightManager_ReplaceForOffer_OnlyCommentChanged_UpdatesDetailsKeepsIDs(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	existing := []flight.Flight{
		withComment(sampleFlight("UUEE", "LFPG", base), 7, "туда"),
		withComment(sampleFlight("LFPG", "UUEE", base.Add(48*time.Hour)), 8, ""),
	}
	flights := &mockOfferFlightRepo{findByOfferIDFlights: existing}
	mgr := flight.NewManager(flights, &mockAirportRepo{})

	// Маршрут и цены те же, у второго перелёта появился комментарий.
	incoming := []flight.Flight{
		withComment(sampleFlight("UUEE", "LFPG", base), 0, "туда"),
		withComment(sampleFlight("LFPG", "UUEE", base.Add(48*time.Hour)), 0, "Ночная пересадка"),
	}
	err := mgr.ReplaceForOffer(context.Background(), 1, incoming)

	require.NoError(t, err)
	assert.False(t, flights.replaceCalled, "comment-only change must not replace the set")
	require.True(t, flights.updateDetailsCalled)
	assert.Equal(t, 1, flights.updateDetailsOfferID)
	assert.Equal(t, []flight.DetailsUpdate{
		{FlightID: 8, Price: samplePrice(), ManagerComment: "Ночная пересадка"},
	}, flights.updatedDetails)
}

func TestOfferFlightManager_ReplaceForOffer_CommentRemoved_UpdatesDetails(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	stored := withComment(sampleFlight("UUEE", "LFPG", base), 7, "Багаж оплачивается отдельно")
	flights := &mockOfferFlightRepo{findByOfferIDFlights: []flight.Flight{stored}}
	mgr := flight.NewManager(flights, &mockAirportRepo{})

	err := mgr.ReplaceForOffer(context.Background(), 1, []flight.Flight{sampleFlight("UUEE", "LFPG", base)})

	require.NoError(t, err)
	assert.False(t, flights.replaceCalled)
	assert.Equal(t, []flight.DetailsUpdate{{FlightID: 7, Price: samplePrice(), ManagerComment: ""}}, flights.updatedDetails)
}

func TestOfferFlightManager_ReplaceForOffer_PriceAndCommentChanged_OneUpdate(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	stored := withComment(withPrice(sampleFlight("UUEE", "LFPG", base), 7, mustMoney("100", money.CodeRUB)), 7, "старый")
	flights := &mockOfferFlightRepo{findByOfferIDFlights: []flight.Flight{stored}}
	mgr := flight.NewManager(flights, &mockAirportRepo{})

	incoming := withComment(withPrice(sampleFlight("UUEE", "LFPG", base), 0, mustMoney("200", money.CodeEUR)), 0, "новый")
	err := mgr.ReplaceForOffer(context.Background(), 1, []flight.Flight{incoming})

	require.NoError(t, err)
	assert.False(t, flights.replaceCalled)
	assert.Equal(t, []flight.DetailsUpdate{
		{FlightID: 7, Price: mustMoney("200", money.CodeEUR), ManagerComment: "новый"},
	}, flights.updatedDetails, "price and comment of one flight must be a single update")
}

func TestOfferFlightManager_ReplaceForOffer_TrailingSpaceInComment_IsAChange(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	stored := withComment(sampleFlight("UUEE", "LFPG", base), 7, "a")
	flights := &mockOfferFlightRepo{findByOfferIDFlights: []flight.Flight{stored}}
	mgr := flight.NewManager(flights, &mockAirportRepo{})

	incoming := withComment(sampleFlight("UUEE", "LFPG", base), 0, "a ")
	err := mgr.ReplaceForOffer(context.Background(), 1, []flight.Flight{incoming})

	require.NoError(t, err)
	assert.False(t, flights.replaceCalled)
	assert.Equal(t, []flight.DetailsUpdate{{FlightID: 7, Price: samplePrice(), ManagerComment: "a "}}, flights.updatedDetails)
}

func TestOfferFlightManager_ReplaceForOffer_RouteAndCommentChanged_Replaces(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	stored := withComment(sampleFlight("UUEE", "LFPG", base), 7, "старый")
	flights := &mockOfferFlightRepo{findByOfferIDFlights: []flight.Flight{stored}}
	mgr := flight.NewManager(flights, &mockAirportRepo{})

	incoming := []flight.Flight{withComment(sampleFlight("UUEE", "EDDF", base), 0, "новый")}
	err := mgr.ReplaceForOffer(context.Background(), 1, incoming)

	require.NoError(t, err)
	assert.True(t, flights.replaceCalled)
	assert.Equal(t, incoming, flights.replacedFlights)
	assert.False(t, flights.updateDetailsCalled)
}

func TestOfferFlightManager_ReplaceForOffer_RouteAndPriceChanged_Replaces(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	existing := []flight.Flight{withPrice(sampleFlight("UUEE", "LFPG", base), 7, mustMoney("100", money.CodeRUB))}
	flights := &mockOfferFlightRepo{findByOfferIDFlights: existing}
	mgr := flight.NewManager(flights, &mockAirportRepo{})

	incoming := []flight.Flight{withPrice(sampleFlight("UUEE", "EDDF", base), 0, mustMoney("300", money.CodeRUB))}
	err := mgr.ReplaceForOffer(context.Background(), 1, incoming)

	require.NoError(t, err)
	assert.True(t, flights.replaceCalled)
	assert.False(t, flights.updateDetailsCalled)
}

func TestOfferFlightManager_ReplaceForOffer_DifferentContent_Replaces(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	existing := []flight.Flight{sampleFlight("UUEE", "LFPG", base)}
	flights := &mockOfferFlightRepo{findByOfferIDFlights: existing}
	mgr := flight.NewManager(flights, &mockAirportRepo{})

	newSet := []flight.Flight{sampleFlight("UUEE", "EDDF", base)}
	err := mgr.ReplaceForOffer(context.Background(), 1, newSet)

	require.NoError(t, err)
	assert.True(t, flights.replaceCalled)
	assert.Equal(t, 1, flights.replacedOfferID)
	assert.Equal(t, newSet, flights.replacedFlights)
}

func TestOfferFlightManager_ReplaceForOffer_EmptyReplacesNonEmpty(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	existing := []flight.Flight{sampleFlight("UUEE", "LFPG", base)}
	flights := &mockOfferFlightRepo{findByOfferIDFlights: existing}
	mgr := flight.NewManager(flights, &mockAirportRepo{})

	err := mgr.ReplaceForOffer(context.Background(), 1, nil)

	require.NoError(t, err)
	assert.True(t, flights.replaceCalled, "clearing flights must still hit the database")
}

func TestOfferFlightManager_ReplaceForOffer_BothEmpty_IsNoOp(t *testing.T) {
	flights := &mockOfferFlightRepo{}
	mgr := flight.NewManager(flights, &mockAirportRepo{})

	err := mgr.ReplaceForOffer(context.Background(), 1, nil)

	require.NoError(t, err)
	assert.False(t, flights.replaceCalled)
}

func TestOfferFlightManager_ReplaceForOffer_DifferentOrder_IsNotEqual(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	existing := []flight.Flight{
		sampleFlight("UUEE", "LFPG", base),
		sampleFlight("EDDF", "LFPG", base.Add(10*time.Hour)),
	}
	flights := &mockOfferFlightRepo{findByOfferIDFlights: existing}
	mgr := flight.NewManager(flights, &mockAirportRepo{})

	// Same two flights, reversed order — order is significant.
	reordered := []flight.Flight{existing[1], existing[0]}
	err := mgr.ReplaceForOffer(context.Background(), 1, reordered)

	require.NoError(t, err)
	assert.True(t, flights.replaceCalled)
}

func TestOfferFlightManager_ReplaceForOffer_UnknownAirport_ReturnsErr(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	flights := &mockOfferFlightRepo{}
	airports := &mockAirportRepo{findByICAOsAirports: []airport.Airport{{ICAO: "UUEE"}}}
	mgr := flight.NewManager(flights, airports)

	err := mgr.ReplaceForOffer(context.Background(), 1, []flight.Flight{sampleFlight("UUEE", "LFPG", base)})

	assert.ErrorIs(t, err, flight.ErrAirportNotFound)
	assert.False(t, flights.replaceCalled, "must not write when an airport is missing")
}
