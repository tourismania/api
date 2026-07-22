package unit_test

import (
	"context"
	"testing"

	updateoffer "api/internal/application/command/update_offer"
	"api/internal/domain/entity"
	"api/internal/domain/enum"
	"api/internal/domain/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateOffer_RoleAgent_OwnAgency_AppliesChanges(t *testing.T) {
	existing := &entity.Offer{UUID: uuid.New(), AgencyID: 5, Title: "Old", Status: enum.OfferStatusDraft}
	offers := &mockOfferRepo{findByUUIDOffer: existing}
	mgr := service.NewOfferManager(offers, &mockAgencyRepo{})
	users := stubUserFinder{record: &entity.UserRecord{ID: 9, AgencyID: 5, Roles: []string{string(enum.RoleAgent)}}}
	h := updateoffer.NewHandler(mgr, users)

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
	existing := &entity.Offer{UUID: uuid.New(), AgencyID: 1, Title: "Old", Status: enum.OfferStatusDraft}
	mgr := service.NewOfferManager(&mockOfferRepo{findByUUIDOffer: existing}, &mockAgencyRepo{})
	users := stubUserFinder{record: &entity.UserRecord{ID: 9, AgencyID: 5, Roles: []string{string(enum.RoleAgent)}}}
	h := updateoffer.NewHandler(mgr, users)

	newTitle := "New title"
	_, err := h.Handle(context.Background(), updateoffer.Command{
		UUID:            existing.UUID,
		Title:           &newTitle,
		CurrentUserUUID: uuid.New(),
	})

	assert.ErrorIs(t, err, service.ErrOfferNotFound)
}

func TestUpdateOffer_RoleUser_ReturnsInsufficientRole(t *testing.T) {
	existing := &entity.Offer{UUID: uuid.New(), AgencyID: 5, Status: enum.OfferStatusDraft}
	mgr := service.NewOfferManager(&mockOfferRepo{findByUUIDOffer: existing}, &mockAgencyRepo{})
	users := stubUserFinder{record: &entity.UserRecord{ID: 9, AgencyID: 5, Roles: []string{string(enum.RoleUser)}}}
	h := updateoffer.NewHandler(mgr, users)

	newTitle := "New title"
	_, err := h.Handle(context.Background(), updateoffer.Command{
		UUID:            existing.UUID,
		Title:           &newTitle,
		CurrentUserUUID: uuid.New(),
	})

	assert.ErrorIs(t, err, service.ErrInsufficientRole)
}
