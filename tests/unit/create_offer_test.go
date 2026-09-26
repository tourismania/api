package unit_test

import (
	"context"
	"testing"

	"api/internal/application/apperror"
	createoffer "api/internal/application/command/create_offer"
	"api/internal/domain/offer"
	"api/internal/domain/user"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateOffer_RoleAgent_CreatesUnderCallerAgency(t *testing.T) {
	offers := &mockOfferRepo{storeID: 1}
	agencies := &mockAgencyRepo{findByIDAgency: activeAgency(5)}
	mgr := offer.NewManager(offers, agencies)
	users := user.NewFinder(stubUserFinder{record: &user.Record{ID: 9, AgencyID: 5, Roles: []string{string(user.RoleAgent)}}})
	h := createoffer.NewHandler(mgr, noFlightManager(), users, noopTxManager{})

	res, err := h.Handle(context.Background(), createoffer.Command{
		Title:           "Title",
		Description:     "desc",
		Status:          offer.StatusDraft,
		CurrentUserUUID: uuid.New(),
	})

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, res.UUID)
	assert.Equal(t, 5, offers.storedOffer.AgencyID)
	assert.Equal(t, 9, offers.storedOffer.CreatedBy)
}

func TestCreateOffer_RoleUser_ReturnsInsufficientRole(t *testing.T) {
	offers := &mockOfferRepo{}
	mgr := offer.NewManager(offers, &mockAgencyRepo{findByIDAgency: activeAgency(5)})
	users := user.NewFinder(stubUserFinder{record: &user.Record{ID: 9, AgencyID: 5, Roles: []string{string(user.RoleUser)}}})
	h := createoffer.NewHandler(mgr, noFlightManager(), users, noopTxManager{})

	_, err := h.Handle(context.Background(), createoffer.Command{
		Title:           "Title",
		Description:     "desc",
		Status:          offer.StatusDraft,
		CurrentUserUUID: uuid.New(),
	})

	assert.ErrorIs(t, err, apperror.ErrForbidden)
}

func TestCreateOffer_ActorNotFound_ReturnsUnauthenticated(t *testing.T) {
	mgr := offer.NewManager(&mockOfferRepo{}, &mockAgencyRepo{})
	h := createoffer.NewHandler(mgr, noFlightManager(), noUserFound(), noopTxManager{})

	_, err := h.Handle(context.Background(), createoffer.Command{
		Title:           "Title",
		Description:     "desc",
		Status:          offer.StatusDraft,
		CurrentUserUUID: uuid.New(),
	})

	assert.ErrorIs(t, err, apperror.ErrUnauthenticated)
}
