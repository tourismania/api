package unit_test

import (
	"context"
	"testing"
	"time"

	"api/internal/application/apperror"
	updateoffer "api/internal/application/command/update_offer"
	"api/internal/domain/entity"
	"api/internal/domain/enum"
	"api/internal/domain/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateOffer_FlightsKeyAbsent_LeavesFlightsUntouched(t *testing.T) {
	existing := &entity.Offer{UUID: uuid.New(), AgencyID: 5, Title: "Old", Status: enum.OfferStatusDraft}
	offers := &mockOfferRepo{findByUUIDOffer: existing}
	mgr := service.NewOfferManager(offers, &mockAgencyRepo{})
	flightRepo := &mockOfferFlightRepo{}
	flightMgr := service.NewOfferFlightManager(flightRepo, &mockAirportRepo{})
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
	existing := &entity.Offer{UUID: uuid.New(), AgencyID: 5, Status: enum.OfferStatusDraft}
	offers := &mockOfferRepo{findByUUIDOffer: existing}
	mgr := service.NewOfferManager(offers, &mockAgencyRepo{})
	flightRepo := &mockOfferFlightRepo{findByOfferIDFlights: []entity.Flight{sampleFlight("UUEE", "LFPG", time.Now())}}
	flightMgr := service.NewOfferFlightManager(flightRepo, &mockAirportRepo{})
	h := updateoffer.NewHandler(mgr, flightMgr, agentUserFinder(5), noopTxManager{})

	empty := [][]entity.FlightSegment{}
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
	existing := &entity.Offer{ID: 7, UUID: uuid.New(), AgencyID: 5, Status: enum.OfferStatusDraft}
	offers := &mockOfferRepo{findByUUIDOffer: existing}
	mgr := service.NewOfferManager(offers, &mockAgencyRepo{})
	flightRepo := &mockOfferFlightRepo{findByOfferIDFlights: []entity.Flight{sampleFlight("UUEE", "LFPG", base)}}
	flightMgr := service.NewOfferFlightManager(flightRepo, &mockAirportRepo{})
	h := updateoffer.NewHandler(mgr, flightMgr, agentUserFinder(5), noopTxManager{})

	newFlights := [][]entity.FlightSegment{
		{{DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "EDDF", DepartureAt: base, ArrivalAt: base.Add(3 * time.Hour)}},
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
	existing := &entity.Offer{ID: 7, UUID: uuid.New(), AgencyID: 5, Status: enum.OfferStatusDraft}
	offers := &mockOfferRepo{findByUUIDOffer: existing}
	mgr := service.NewOfferManager(offers, &mockAgencyRepo{})
	flightRepo := &mockOfferFlightRepo{findByOfferIDFlights: []entity.Flight{sampleFlight("UUEE", "LFPG", base)}}
	flightMgr := service.NewOfferFlightManager(flightRepo, &mockAirportRepo{})
	h := updateoffer.NewHandler(mgr, flightMgr, agentUserFinder(5), noopTxManager{})

	sameFlights := [][]entity.FlightSegment{
		{{DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", DepartureAt: base, ArrivalAt: base.Add(2 * time.Hour)}},
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
	existing := &entity.Offer{UUID: uuid.New(), AgencyID: 5, Status: enum.OfferStatusDraft}
	offers := &mockOfferRepo{findByUUIDOffer: existing}
	mgr := service.NewOfferManager(offers, &mockAgencyRepo{})
	flightMgr := service.NewOfferFlightManager(&mockOfferFlightRepo{}, &mockAirportRepo{})
	h := updateoffer.NewHandler(mgr, flightMgr, agentUserFinder(5), noopTxManager{})

	invalid := [][]entity.FlightSegment{{}}
	_, err := h.Handle(context.Background(), updateoffer.Command{
		UUID:            existing.UUID,
		Flights:         &invalid,
		CurrentUserUUID: uuid.New(),
	})

	assert.ErrorIs(t, err, apperror.ErrValidation)
}
