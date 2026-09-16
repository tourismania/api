package unit_test

import (
	"context"
	"testing"

	"api/internal/application/apperror"
	updateoffer "api/internal/application/command/update_offer"
	"api/internal/domain/offer"
	"api/internal/domain/user"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateOffer_RoleAgent_OwnAgency_AppliesChanges(t *testing.T) {
	existing := &offer.Offer{UUID: uuid.New(), AgencyID: 5, Title: "Old", Status: offer.StatusDraft}
	offers := &mockOfferRepo{findByUUIDOffer: existing}
	mgr := offer.NewManager(offers, &mockAgencyRepo{})
	users := user.NewFinder(stubUserFinder{record: &user.Record{ID: 9, AgencyID: 5, Roles: []string{string(user.RoleAgent)}}})
	h := updateoffer.NewHandler(mgr, noFlightManager(), users, noopTxManager{})

	newTitle := "New title"
	_, err := h.Handle(context.Background(), updateoffer.Command{
		UUID:            existing.UUID,
		Title:           &newTitle,
		CurrentUserUUID: uuid.New(),
	})

	require.NoError(t, err)
	assert.Equal(t, "New title", offers.updatedOffer.Title)
}

func TestUpdateOffer_DifferentAgency_ReturnsNotFound(t *testing.T) {
	existing := &offer.Offer{UUID: uuid.New(), AgencyID: 1, Title: "Old", Status: offer.StatusDraft}
	mgr := offer.NewManager(&mockOfferRepo{findByUUIDOffer: existing}, &mockAgencyRepo{})
	users := user.NewFinder(stubUserFinder{record: &user.Record{ID: 9, AgencyID: 5, Roles: []string{string(user.RoleAgent)}}})
	h := updateoffer.NewHandler(mgr, noFlightManager(), users, noopTxManager{})

	newTitle := "New title"
	_, err := h.Handle(context.Background(), updateoffer.Command{
		UUID:            existing.UUID,
		Title:           &newTitle,
		CurrentUserUUID: uuid.New(),
	})

	assert.ErrorIs(t, err, apperror.ErrNotFound)
}

func TestUpdateOffer_RoleUser_ReturnsInsufficientRole(t *testing.T) {
	existing := &offer.Offer{UUID: uuid.New(), AgencyID: 5, Status: offer.StatusDraft}
	mgr := offer.NewManager(&mockOfferRepo{findByUUIDOffer: existing}, &mockAgencyRepo{})
	users := user.NewFinder(stubUserFinder{record: &user.Record{ID: 9, AgencyID: 5, Roles: []string{string(user.RoleUser)}}})
	h := updateoffer.NewHandler(mgr, noFlightManager(), users, noopTxManager{})

	newTitle := "New title"
	_, err := h.Handle(context.Background(), updateoffer.Command{
		UUID:            existing.UUID,
		Title:           &newTitle,
		CurrentUserUUID: uuid.New(),
	})

	assert.ErrorIs(t, err, apperror.ErrForbidden)
}
