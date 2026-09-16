package unit_test

import (
	"context"
	"testing"

	"api/internal/application/apperror"
	deleteoffer "api/internal/application/command/delete_offer"
	"api/internal/domain/offer"
	"api/internal/domain/user"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteOffer_RoleAgent_OwnAgency_SoftDeletes(t *testing.T) {
	existing := &offer.Offer{UUID: uuid.New(), AgencyID: 5}
	offers := &mockOfferRepo{findByUUIDOffer: existing}
	mgr := offer.NewManager(offers, &mockAgencyRepo{})
	users := user.NewFinder(stubUserFinder{record: &user.Record{ID: 9, AgencyID: 5, Roles: []string{string(user.RoleAgent)}}})
	h := deleteoffer.NewHandler(mgr, users)

	_, err := h.Handle(context.Background(), deleteoffer.Command{
		UUID:            existing.UUID,
		CurrentUserUUID: uuid.New(),
	})

	require.NoError(t, err)
	assert.Equal(t, existing.UUID, offers.softDeletedID)
}

func TestDeleteOffer_DifferentAgency_ReturnsNotFound(t *testing.T) {
	existing := &offer.Offer{UUID: uuid.New(), AgencyID: 1}
	mgr := offer.NewManager(&mockOfferRepo{findByUUIDOffer: existing}, &mockAgencyRepo{})
	users := user.NewFinder(stubUserFinder{record: &user.Record{ID: 9, AgencyID: 5, Roles: []string{string(user.RoleAgent)}}})
	h := deleteoffer.NewHandler(mgr, users)

	_, err := h.Handle(context.Background(), deleteoffer.Command{
		UUID:            existing.UUID,
		CurrentUserUUID: uuid.New(),
	})

	assert.ErrorIs(t, err, apperror.ErrNotFound)
}

func TestDeleteOffer_RoleUser_ReturnsInsufficientRole(t *testing.T) {
	existing := &offer.Offer{UUID: uuid.New(), AgencyID: 5}
	mgr := offer.NewManager(&mockOfferRepo{findByUUIDOffer: existing}, &mockAgencyRepo{})
	users := user.NewFinder(stubUserFinder{record: &user.Record{ID: 9, AgencyID: 5, Roles: []string{string(user.RoleUser)}}})
	h := deleteoffer.NewHandler(mgr, users)

	_, err := h.Handle(context.Background(), deleteoffer.Command{
		UUID:            existing.UUID,
		CurrentUserUUID: uuid.New(),
	})

	assert.ErrorIs(t, err, apperror.ErrForbidden)
}
