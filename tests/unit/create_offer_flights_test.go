package unit_test

import (
	"context"
	"testing"
	"time"

	"api/internal/application/apperror"
	createoffer "api/internal/application/command/create_offer"
	"api/internal/domain/entity"
	"api/internal/domain/enum"
	"api/internal/domain/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func agentUserFinder(agencyID int) *service.UserFinder {
	return service.NewUserFinder(stubUserFinder{record: &entity.UserRecord{ID: 9, AgencyID: agencyID, Roles: []string{string(enum.RoleAgent)}}})
}

func TestCreateOffer_WithValidFlights_ReplacesFlightsForNewOffer(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	offers := &mockOfferRepo{storeID: 42}
	agencies := &mockAgencyRepo{findByIDAgency: activeAgency(5)}
	mgr := service.NewOfferManager(offers, agencies)
	flightRepo := &mockOfferFlightRepo{}
	flightMgr := service.NewOfferFlightManager(flightRepo, &mockAirportRepo{})
	h := createoffer.NewHandler(mgr, flightMgr, agentUserFinder(5), noopTxManager{})

	res, err := h.Handle(context.Background(), createoffer.Command{
		Title:       "Title",
		Description: "desc",
		Status:      enum.OfferStatusDraft,
		Flights: [][]createoffer.FlightSegmentInput{
			{{DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", DepartureAt: base, ArrivalAt: base.Add(4 * time.Hour)}},
		},
		CurrentUserUUID: uuid.New(),
	})

	require.NoError(t, err)
	assert.Equal(t, 42, res.ID)
	assert.True(t, flightRepo.replaceCalled)
	assert.Equal(t, 42, flightRepo.replacedOfferID)
	require.Len(t, flightRepo.replacedFlights, 1)
	assert.Equal(t, "UUEE", flightRepo.replacedFlights[0].DepartureAirportICAO())
}

func TestCreateOffer_NoFlights_DoesNotCallReplace(t *testing.T) {
	offers := &mockOfferRepo{storeID: 1}
	mgr := service.NewOfferManager(offers, &mockAgencyRepo{findByIDAgency: activeAgency(5)})
	flightRepo := &mockOfferFlightRepo{}
	flightMgr := service.NewOfferFlightManager(flightRepo, &mockAirportRepo{})
	h := createoffer.NewHandler(mgr, flightMgr, agentUserFinder(5), noopTxManager{})

	_, err := h.Handle(context.Background(), createoffer.Command{
		Title:           "Title",
		Description:     "desc",
		Status:          enum.OfferStatusDraft,
		CurrentUserUUID: uuid.New(),
	})

	require.NoError(t, err)
	assert.False(t, flightRepo.replaceCalled)
}

func TestCreateOffer_InvalidFlightStructure_ReturnsValidationError_NeverStoresOffer(t *testing.T) {
	offers := &mockOfferRepo{storeID: 1}
	mgr := service.NewOfferManager(offers, &mockAgencyRepo{findByIDAgency: activeAgency(5)})
	flightMgr := service.NewOfferFlightManager(&mockOfferFlightRepo{}, &mockAirportRepo{})
	h := createoffer.NewHandler(mgr, flightMgr, agentUserFinder(5), noopTxManager{})

	_, err := h.Handle(context.Background(), createoffer.Command{
		Title:       "Title",
		Description: "desc",
		Status:      enum.OfferStatusDraft,
		// Empty segments list violates factory.NewFlight's structural invariant.
		Flights:         [][]createoffer.FlightSegmentInput{{}},
		CurrentUserUUID: uuid.New(),
	})

	assert.ErrorIs(t, err, apperror.ErrValidation)
	assert.Equal(t, entity.Offer{}, offers.storedOffer, "structural validation must fail before any transaction opens")
}

func TestCreateOffer_UnknownAirport_ReturnsValidationError(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	offers := &mockOfferRepo{storeID: 1}
	mgr := service.NewOfferManager(offers, &mockAgencyRepo{findByIDAgency: activeAgency(5)})
	airports := &mockAirportRepo{findByICAOsAirports: []entity.Airport{}}
	flightMgr := service.NewOfferFlightManager(&mockOfferFlightRepo{}, airports)
	h := createoffer.NewHandler(mgr, flightMgr, agentUserFinder(5), noopTxManager{})

	_, err := h.Handle(context.Background(), createoffer.Command{
		Title:       "Title",
		Description: "desc",
		Status:      enum.OfferStatusDraft,
		Flights: [][]createoffer.FlightSegmentInput{
			{{DepartureAirportICAO: "ZZZZ", ArrivalAirportICAO: "LFPG", DepartureAt: base, ArrivalAt: base.Add(4 * time.Hour)}},
		},
		CurrentUserUUID: uuid.New(),
	})

	assert.ErrorIs(t, err, apperror.ErrValidation)
}
