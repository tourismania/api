package unit_test

import (
	"context"
	"testing"

	"api/internal/application/apperror"
	createoffer "api/internal/application/command/create_offer"
	"api/internal/domain/entity"
	"api/internal/domain/enum"
	"api/internal/domain/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateOffer_RoleAgent_CreatesUnderCallerAgency(t *testing.T) {
	offers := &mockOfferRepo{storeID: 1}
	agencies := &mockAgencyRepo{findByIDAgency: activeAgency(5)}
	mgr := service.NewOfferManager(offers, agencies)
	users := service.NewUserFinder(stubUserFinder{record: &entity.UserRecord{ID: 9, AgencyID: 5, Roles: []string{string(enum.RoleAgent)}}})
	h := createoffer.NewHandler(mgr, users)

	res, err := h.Handle(context.Background(), createoffer.Command{
		Title:           "Title",
		Description:     "desc",
		Status:          enum.OfferStatusDraft,
		CurrentUserUUID: uuid.New(),
	})

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, res.UUID)
	assert.Equal(t, 5, offers.storedOffer.AgencyID)
	assert.Equal(t, 9, offers.storedOffer.CreatedBy)
}

func TestCreateOffer_RoleUser_ReturnsInsufficientRole(t *testing.T) {
	offers := &mockOfferRepo{}
	mgr := service.NewOfferManager(offers, &mockAgencyRepo{findByIDAgency: activeAgency(5)})
	users := service.NewUserFinder(stubUserFinder{record: &entity.UserRecord{ID: 9, AgencyID: 5, Roles: []string{string(enum.RoleUser)}}})
	h := createoffer.NewHandler(mgr, users)

	_, err := h.Handle(context.Background(), createoffer.Command{
		Title:           "Title",
		Description:     "desc",
		Status:          enum.OfferStatusDraft,
		CurrentUserUUID: uuid.New(),
	})

	assert.ErrorIs(t, err, apperror.ErrForbidden)
}

func TestCreateOffer_ActorNotFound_ReturnsUnauthenticated(t *testing.T) {
	mgr := service.NewOfferManager(&mockOfferRepo{}, &mockAgencyRepo{})
	h := createoffer.NewHandler(mgr, noUserFound())

	_, err := h.Handle(context.Background(), createoffer.Command{
		Title:           "Title",
		Description:     "desc",
		Status:          enum.OfferStatusDraft,
		CurrentUserUUID: uuid.New(),
	})

	assert.ErrorIs(t, err, apperror.ErrUnauthenticated)
}
