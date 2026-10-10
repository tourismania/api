package unit_test

import (
	"context"
	"testing"
	"time"

	"api/internal/application/apperror"
	updateoffer "api/internal/application/command/update_offer"
	"api/internal/domain/money"
	"api/internal/domain/offer"
	"api/internal/domain/offer/flight"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateOffer_FlightsKeyAbsent_LeavesFlightsUntouched(t *testing.T) {
	existing := &offer.Offer{UUID: uuid.New(), AgencyID: 5, Title: "Old", Status: offer.StatusDraft}
	offers := &mockOfferRepo{findByUUIDOffer: existing}
	mgr := offer.NewManager(offers, &mockAgencyRepo{})
	flightRepo := &mockOfferFlightRepo{}
	flightMgr := flight.NewManager(flightRepo, &mockAirportRepo{})
	h := updateoffer.NewHandler(mgr, flightMgr, agentUserFinder(5), noopTxManager{})

	newTitle := "New title"
	_, err := h.Handle(context.Background(), updateoffer.Command{
		UUID:            existing.UUID,
		Title:           &newTitle,
		CurrentUserUUID: uuid.New(),
	})

	require.NoError(t, err)
	assert.False(t, flightRepo.replaceCalled, "flights key absent from the request must leave existing flights untouched")
}

func TestUpdateOffer_FlightsEmptySlice_ClearsFlights(t *testing.T) {
	existing := &offer.Offer{UUID: uuid.New(), AgencyID: 5, Status: offer.StatusDraft}
	offers := &mockOfferRepo{findByUUIDOffer: existing}
	mgr := offer.NewManager(offers, &mockAgencyRepo{})
	flightRepo := &mockOfferFlightRepo{findByOfferIDFlights: []flight.Flight{sampleFlight("UUEE", "LFPG", time.Now())}}
	flightMgr := flight.NewManager(flightRepo, &mockAirportRepo{})
	h := updateoffer.NewHandler(mgr, flightMgr, agentUserFinder(5), noopTxManager{})

	empty := []updateoffer.FlightInput{}
	_, err := h.Handle(context.Background(), updateoffer.Command{
		UUID:            existing.UUID,
		Flights:         &empty,
		CurrentUserUUID: uuid.New(),
	})

	require.NoError(t, err)
	assert.True(t, flightRepo.replaceCalled)
	assert.Empty(t, flightRepo.replacedFlights)
}

func TestUpdateOffer_FlightsChanged_Replaces(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	existing := &offer.Offer{ID: 7, UUID: uuid.New(), AgencyID: 5, Status: offer.StatusDraft}
	offers := &mockOfferRepo{findByUUIDOffer: existing}
	mgr := offer.NewManager(offers, &mockAgencyRepo{})
	flightRepo := &mockOfferFlightRepo{findByOfferIDFlights: []flight.Flight{sampleFlight("UUEE", "LFPG", base)}}
	flightMgr := flight.NewManager(flightRepo, &mockAirportRepo{})
	h := updateoffer.NewHandler(mgr, flightMgr, agentUserFinder(5), noopTxManager{})

	newFlights := []updateoffer.FlightInput{
		{Price: "15000", Currency: "RUB", Segments: []updateoffer.FlightSegmentInput{{DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "EDDF", DepartureAt: base, ArrivalAt: base.Add(3 * time.Hour)}}},
	}
	_, err := h.Handle(context.Background(), updateoffer.Command{
		UUID:            existing.UUID,
		Flights:         &newFlights,
		CurrentUserUUID: uuid.New(),
	})

	require.NoError(t, err)
	assert.True(t, flightRepo.replaceCalled)
	assert.Equal(t, 7, flightRepo.replacedOfferID)
}

func TestUpdateOffer_FlightsIdenticalToStored_IsNoOp(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	existing := &offer.Offer{ID: 7, UUID: uuid.New(), AgencyID: 5, Status: offer.StatusDraft}
	offers := &mockOfferRepo{findByUUIDOffer: existing}
	mgr := offer.NewManager(offers, &mockAgencyRepo{})
	flightRepo := &mockOfferFlightRepo{findByOfferIDFlights: []flight.Flight{sampleFlight("UUEE", "LFPG", base)}}
	flightMgr := flight.NewManager(flightRepo, &mockAirportRepo{})
	h := updateoffer.NewHandler(mgr, flightMgr, agentUserFinder(5), noopTxManager{})

	sameFlights := []updateoffer.FlightInput{
		{Price: "15000", Currency: "RUB", Segments: []updateoffer.FlightSegmentInput{{DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", DepartureAt: base, ArrivalAt: base.Add(2 * time.Hour)}}},
	}
	_, err := h.Handle(context.Background(), updateoffer.Command{
		UUID:            existing.UUID,
		Flights:         &sameFlights,
		CurrentUserUUID: uuid.New(),
	})

	require.NoError(t, err)
	assert.False(t, flightRepo.replaceCalled, "identical flights must not touch the database")
}

func TestUpdateOffer_InvalidFlightStructure_ReturnsValidationError(t *testing.T) {
	existing := &offer.Offer{UUID: uuid.New(), AgencyID: 5, Status: offer.StatusDraft}
	offers := &mockOfferRepo{findByUUIDOffer: existing}
	mgr := offer.NewManager(offers, &mockAgencyRepo{})
	flightMgr := flight.NewManager(&mockOfferFlightRepo{}, &mockAirportRepo{})
	h := updateoffer.NewHandler(mgr, flightMgr, agentUserFinder(5), noopTxManager{})

	invalid := []updateoffer.FlightInput{{Price: "15000", Currency: "RUB"}}
	_, err := h.Handle(context.Background(), updateoffer.Command{
		UUID:            existing.UUID,
		Flights:         &invalid,
		CurrentUserUUID: uuid.New(),
	})

	assert.ErrorIs(t, err, apperror.ErrValidation)
}

func TestUpdateOffer_OnlyPriceChanged_UpdatesPricesKeepsFlights(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	existing := &offer.Offer{ID: 7, UUID: uuid.New(), AgencyID: 5, Status: offer.StatusDraft}
	offers := &mockOfferRepo{findByUUIDOffer: existing}
	mgr := offer.NewManager(offers, &mockAgencyRepo{})
	stored := sampleFlight("UUEE", "LFPG", base)
	stored.ID = 31
	flightRepo := &mockOfferFlightRepo{findByOfferIDFlights: []flight.Flight{stored}}
	flightMgr := flight.NewManager(flightRepo, &mockAirportRepo{})
	h := updateoffer.NewHandler(mgr, flightMgr, agentUserFinder(5), noopTxManager{})

	repriced := []updateoffer.FlightInput{
		{Price: "17500", Currency: "RUB", Segments: []updateoffer.FlightSegmentInput{{DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", DepartureAt: base, ArrivalAt: base.Add(2 * time.Hour)}}},
	}
	_, err := h.Handle(context.Background(), updateoffer.Command{
		UUID:            existing.UUID,
		Flights:         &repriced,
		CurrentUserUUID: uuid.New(),
	})

	require.NoError(t, err)
	assert.False(t, flightRepo.replaceCalled, "price-only change must not replace flights")
	require.True(t, flightRepo.updatePricesCalled)
	assert.Equal(t, 7, flightRepo.updatePricesOfferID)
	assert.Equal(t, []flight.PriceUpdate{{FlightID: 31, Price: mustMoney("17500", money.CodeRUB)}}, flightRepo.updatedPrices)
}
