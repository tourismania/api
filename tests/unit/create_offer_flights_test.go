package unit_test

import (
	"context"
	"testing"
	"time"

	"api/internal/application/apperror"
	createoffer "api/internal/application/command/create_offer"
	"api/internal/domain/airport"
	"api/internal/domain/money"
	"api/internal/domain/offer"
	"api/internal/domain/offer/flight"
	"api/internal/domain/user"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func agentUserFinder(agencyID int) *user.Finder {
	return user.NewFinder(stubUserFinder{record: &user.Record{ID: 9, AgencyID: agencyID, Roles: []string{string(user.RoleAgent)}}})
}

func TestCreateOffer_WithValidFlights_ReplacesFlightsForNewOffer(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	offers := &mockOfferRepo{storeID: 42}
	agencies := &mockAgencyRepo{findByIDAgency: activeAgency(5)}
	mgr := offer.NewManager(offers, agencies)
	flightRepo := &mockOfferFlightRepo{}
	flightMgr := flight.NewManager(flightRepo, &mockAirportRepo{})
	h := createoffer.NewHandler(mgr, flightMgr, agentUserFinder(5), noopTxManager{})

	res, err := h.Handle(context.Background(), createoffer.Command{
		Title:       "Title",
		Description: "desc",
		Status:      offer.StatusDraft,
		Flights: []createoffer.FlightInput{
			{Price: "15000", Currency: "RUB", Segments: []createoffer.FlightSegmentInput{{DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", DepartureAt: base, ArrivalAt: base.Add(4 * time.Hour)}}},
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
	mgr := offer.NewManager(offers, &mockAgencyRepo{findByIDAgency: activeAgency(5)})
	flightRepo := &mockOfferFlightRepo{}
	flightMgr := flight.NewManager(flightRepo, &mockAirportRepo{})
	h := createoffer.NewHandler(mgr, flightMgr, agentUserFinder(5), noopTxManager{})

	_, err := h.Handle(context.Background(), createoffer.Command{
		Title:           "Title",
		Description:     "desc",
		Status:          offer.StatusDraft,
		CurrentUserUUID: uuid.New(),
	})

	require.NoError(t, err)
	assert.False(t, flightRepo.replaceCalled)
}

func TestCreateOffer_InvalidFlightStructure_ReturnsValidationError_NeverStoresOffer(t *testing.T) {
	offers := &mockOfferRepo{storeID: 1}
	mgr := offer.NewManager(offers, &mockAgencyRepo{findByIDAgency: activeAgency(5)})
	flightMgr := flight.NewManager(&mockOfferFlightRepo{}, &mockAirportRepo{})
	h := createoffer.NewHandler(mgr, flightMgr, agentUserFinder(5), noopTxManager{})

	_, err := h.Handle(context.Background(), createoffer.Command{
		Title:       "Title",
		Description: "desc",
		Status:      offer.StatusDraft,
		// Empty segments list violates flight.New's structural invariant.
		Flights:         []createoffer.FlightInput{{Price: "15000", Currency: "RUB"}},
		CurrentUserUUID: uuid.New(),
	})

	assert.ErrorIs(t, err, apperror.ErrValidation)
	assert.Equal(t, offer.Offer{}, offers.storedOffer, "structural validation must fail before any transaction opens")
}

func TestCreateOffer_UnknownAirport_ReturnsValidationError(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	offers := &mockOfferRepo{storeID: 1}
	mgr := offer.NewManager(offers, &mockAgencyRepo{findByIDAgency: activeAgency(5)})
	airports := &mockAirportRepo{findByICAOsAirports: []airport.Airport{}}
	flightMgr := flight.NewManager(&mockOfferFlightRepo{}, airports)
	h := createoffer.NewHandler(mgr, flightMgr, agentUserFinder(5), noopTxManager{})

	_, err := h.Handle(context.Background(), createoffer.Command{
		Title:       "Title",
		Description: "desc",
		Status:      offer.StatusDraft,
		Flights: []createoffer.FlightInput{
			{Price: "15000", Currency: "RUB", Segments: []createoffer.FlightSegmentInput{{DepartureAirportICAO: "ZZZZ", ArrivalAirportICAO: "LFPG", DepartureAt: base, ArrivalAt: base.Add(4 * time.Hour)}}},
		},
		CurrentUserUUID: uuid.New(),
	})

	assert.ErrorIs(t, err, apperror.ErrValidation)
}

func TestCreateOffer_WithValidFlights_StoresParsedPrice(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	offers := &mockOfferRepo{storeID: 42}
	mgr := offer.NewManager(offers, &mockAgencyRepo{findByIDAgency: activeAgency(5)})
	flightRepo := &mockOfferFlightRepo{}
	flightMgr := flight.NewManager(flightRepo, &mockAirportRepo{})
	h := createoffer.NewHandler(mgr, flightMgr, agentUserFinder(5), noopTxManager{})

	_, err := h.Handle(context.Background(), createoffer.Command{
		Title:  "Title",
		Status: offer.StatusDraft,
		Flights: []createoffer.FlightInput{
			{Price: "12345.5", Currency: "EUR", Segments: []createoffer.FlightSegmentInput{{DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", DepartureAt: base, ArrivalAt: base.Add(4 * time.Hour)}}},
		},
		CurrentUserUUID: uuid.New(),
	})

	require.NoError(t, err)
	require.Len(t, flightRepo.replacedFlights, 1)
	assert.Equal(t, mustMoney("12345.50", money.CodeEUR), flightRepo.replacedFlights[0].Price)
}

func TestCreateOffer_WithManagerComment_StoresComment(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	offers := &mockOfferRepo{storeID: 42}
	mgr := offer.NewManager(offers, &mockAgencyRepo{findByIDAgency: activeAgency(5)})
	flightRepo := &mockOfferFlightRepo{}
	flightMgr := flight.NewManager(flightRepo, &mockAirportRepo{})
	h := createoffer.NewHandler(mgr, flightMgr, agentUserFinder(5), noopTxManager{})

	_, err := h.Handle(context.Background(), createoffer.Command{
		Title:  "Title",
		Status: offer.StatusDraft,
		Flights: []createoffer.FlightInput{
			{Price: "100", Currency: "RUB", ManagerComment: "Багаж оплачивается отдельно", Segments: []createoffer.FlightSegmentInput{{DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", DepartureAt: base, ArrivalAt: base.Add(4 * time.Hour)}}},
		},
		CurrentUserUUID: uuid.New(),
	})

	require.NoError(t, err)
	require.Len(t, flightRepo.replacedFlights, 1)
	assert.Equal(t, "Багаж оплачивается отдельно", flightRepo.replacedFlights[0].ManagerComment)
}

func TestCreateOffer_ManagerCommentControlChar_ReturnsFieldError_NeverStoresOffer(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	offers := &mockOfferRepo{storeID: 1}
	mgr := offer.NewManager(offers, &mockAgencyRepo{findByIDAgency: activeAgency(5)})
	flightRepo := &mockOfferFlightRepo{}
	flightMgr := flight.NewManager(flightRepo, &mockAirportRepo{})
	h := createoffer.NewHandler(mgr, flightMgr, agentUserFinder(5), noopTxManager{})

	_, err := h.Handle(context.Background(), createoffer.Command{
		Title:  "Title",
		Status: offer.StatusDraft,
		Flights: []createoffer.FlightInput{
			{Price: "100", Currency: "RUB", ManagerComment: "до\u0000после", Segments: []createoffer.FlightSegmentInput{{DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", DepartureAt: base, ArrivalAt: base.Add(4 * time.Hour)}}},
		},
		CurrentUserUUID: uuid.New(),
	})

	assert.ErrorIs(t, err, apperror.ErrValidation)
	fe, ok := apperror.AsFieldError(err)
	require.True(t, ok)
	assert.Equal(t, "ManagerComment", fe.Field)
	assert.Equal(t, offer.Offer{}, offers.storedOffer)
	assert.False(t, flightRepo.replaceCalled)
}

func TestCreateOffer_InvalidPriceOrCurrency_ReturnsValidationError_NeverStoresOffer(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	for _, tc := range []struct{ price, currency string }{
		{"10.125", "USD"},
		{"-1", "RUB"},
		{"100", "GBP"},
		{"", "RUB"},
	} {
		offers := &mockOfferRepo{storeID: 1}
		mgr := offer.NewManager(offers, &mockAgencyRepo{findByIDAgency: activeAgency(5)})
		flightMgr := flight.NewManager(&mockOfferFlightRepo{}, &mockAirportRepo{})
		h := createoffer.NewHandler(mgr, flightMgr, agentUserFinder(5), noopTxManager{})

		_, err := h.Handle(context.Background(), createoffer.Command{
			Title:  "Title",
			Status: offer.StatusDraft,
			Flights: []createoffer.FlightInput{
				{Price: tc.price, Currency: tc.currency, Segments: []createoffer.FlightSegmentInput{{DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", DepartureAt: base, ArrivalAt: base.Add(4 * time.Hour)}}},
			},
			CurrentUserUUID: uuid.New(),
		})

		assert.ErrorIs(t, err, apperror.ErrValidation, tc)
		assert.Equal(t, offer.Offer{}, offers.storedOffer, tc)
	}
}

func TestCreateOffer_WithCabinClasses_StoresClassesEmptyStaysEmpty(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	offers := &mockOfferRepo{storeID: 42}
	mgr := offer.NewManager(offers, &mockAgencyRepo{findByIDAgency: activeAgency(5)})
	flightRepo := &mockOfferFlightRepo{}
	flightMgr := flight.NewManager(flightRepo, &mockAirportRepo{})
	h := createoffer.NewHandler(mgr, flightMgr, agentUserFinder(5), noopTxManager{})

	_, err := h.Handle(context.Background(), createoffer.Command{
		Title:  "Title",
		Status: offer.StatusDraft,
		Flights: []createoffer.FlightInput{
			{Price: "100", Currency: "RUB", Segments: []createoffer.FlightSegmentInput{
				{DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", DepartureAt: base, ArrivalAt: base.Add(4 * time.Hour), CabinClass: "comfort"},
				{DepartureAirportICAO: "LFPG", ArrivalAirportICAO: "KJFK", DepartureAt: base.Add(6 * time.Hour), ArrivalAt: base.Add(14 * time.Hour)},
			}},
		},
		CurrentUserUUID: uuid.New(),
	})

	require.NoError(t, err)
	require.Len(t, flightRepo.replacedFlights, 1)
	require.Len(t, flightRepo.replacedFlights[0].Segments, 2)
	assert.Equal(t, flight.CabinClassComfort, flightRepo.replacedFlights[0].Segments[0].CabinClass)
	assert.Equal(t, flight.CabinClassUnknown, flightRepo.replacedFlights[0].Segments[1].CabinClass, "no default class")
}

func TestCreateOffer_CabinClassInvalid_ReturnsFieldError_NeverStoresOffer(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	offers := &mockOfferRepo{storeID: 1}
	mgr := offer.NewManager(offers, &mockAgencyRepo{findByIDAgency: activeAgency(5)})
	flightRepo := &mockOfferFlightRepo{}
	flightMgr := flight.NewManager(flightRepo, &mockAirportRepo{})
	h := createoffer.NewHandler(mgr, flightMgr, agentUserFinder(5), noopTxManager{})

	_, err := h.Handle(context.Background(), createoffer.Command{
		Title:  "Title",
		Status: offer.StatusDraft,
		Flights: []createoffer.FlightInput{
			{Price: "100", Currency: "RUB", Segments: []createoffer.FlightSegmentInput{{DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", DepartureAt: base, ArrivalAt: base.Add(4 * time.Hour), CabinClass: "first"}}},
		},
		CurrentUserUUID: uuid.New(),
	})

	assert.ErrorIs(t, err, apperror.ErrValidation)
	fe, ok := apperror.AsFieldError(err)
	require.True(t, ok)
	assert.Equal(t, "CabinClass", fe.Field)
	assert.Equal(t, "oneof", fe.Rule)
	assert.Equal(t, offer.Offer{}, offers.storedOffer)
	assert.False(t, flightRepo.replaceCalled)
}
