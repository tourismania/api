package unit_test

import (
	"context"
	"testing"

	getoffer "api/internal/application/query/get_offer"
	"api/internal/domain/entity"
	"api/internal/domain/enum"
	"api/internal/domain/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubOfferFinder is a hand-written test double for the getoffer.OfferFinder port.
type stubOfferFinder struct {
	offer *entity.Offer
	err   error
}

func (s stubOfferFinder) FindByUUID(_ context.Context, _ uuid.UUID) (*entity.Offer, error) {
	return s.offer, s.err
}

func TestGetOffer_SuperAdmin_SeesDraftOfferOfAnyAgency(t *testing.T) {
	offer := &entity.Offer{UUID: uuid.New(), AgencyID: 7, Status: enum.OfferStatusDraft}
	h := getoffer.NewHandler(stubOfferFinder{offer: offer})

	res, err := h.Handle(context.Background(), getoffer.Query{
		UUID:            offer.UUID,
		CurrentAgencyID: intPtr(1),
		CurrentRoles:    []enum.Role{enum.RoleSuperAdmin},
	})

	require.NoError(t, err)
	assert.Equal(t, offer.UUID, res.UUID)
}

func TestGetOffer_Agent_SeesOwnAgencyDraftOffer(t *testing.T) {
	offer := &entity.Offer{UUID: uuid.New(), AgencyID: 7, Status: enum.OfferStatusDraft}
	h := getoffer.NewHandler(stubOfferFinder{offer: offer})

	res, err := h.Handle(context.Background(), getoffer.Query{
		UUID:            offer.UUID,
		CurrentAgencyID: intPtr(7),
		CurrentRoles:    []enum.Role{enum.RoleAgent},
	})

	require.NoError(t, err)
	assert.Equal(t, offer.UUID, res.UUID)
}

func TestGetOffer_Agent_OtherAgencyDraftOffer_NotFound(t *testing.T) {
	offer := &entity.Offer{UUID: uuid.New(), AgencyID: 7, Status: enum.OfferStatusDraft}
	h := getoffer.NewHandler(stubOfferFinder{offer: offer})

	_, err := h.Handle(context.Background(), getoffer.Query{
		UUID:            offer.UUID,
		CurrentAgencyID: intPtr(1),
		CurrentRoles:    []enum.Role{enum.RoleAgent},
	})

	assert.ErrorIs(t, err, service.ErrOfferNotFound)
}

func TestGetOffer_Agent_OtherAgencyPublishedOffer_Visible(t *testing.T) {
	offer := &entity.Offer{UUID: uuid.New(), AgencyID: 7, Status: enum.OfferStatusPublished}
	h := getoffer.NewHandler(stubOfferFinder{offer: offer})

	res, err := h.Handle(context.Background(), getoffer.Query{
		UUID:            offer.UUID,
		CurrentAgencyID: intPtr(1),
		CurrentRoles:    []enum.Role{enum.RoleAgent},
	})

	require.NoError(t, err)
	assert.Equal(t, offer.UUID, res.UUID)
}

func TestGetOffer_PlainUser_DraftOffer_NotFound(t *testing.T) {
	offer := &entity.Offer{UUID: uuid.New(), AgencyID: 7, Status: enum.OfferStatusDraft}
	h := getoffer.NewHandler(stubOfferFinder{offer: offer})

	_, err := h.Handle(context.Background(), getoffer.Query{
		UUID:            offer.UUID,
		CurrentAgencyID: intPtr(7),
		CurrentRoles:    []enum.Role{enum.RoleUser},
	})

	assert.ErrorIs(t, err, service.ErrOfferNotFound)
}

func TestGetOffer_PlainUser_PublishedOffer_Visible(t *testing.T) {
	offer := &entity.Offer{UUID: uuid.New(), AgencyID: 7, Status: enum.OfferStatusPublished}
	h := getoffer.NewHandler(stubOfferFinder{offer: offer})

	res, err := h.Handle(context.Background(), getoffer.Query{
		UUID:            offer.UUID,
		CurrentAgencyID: intPtr(1),
		CurrentRoles:    []enum.Role{enum.RoleUser},
	})

	require.NoError(t, err)
	assert.Equal(t, offer.UUID, res.UUID)
}

func TestGetOffer_NotFound_ReturnsErrOfferNotFound(t *testing.T) {
	h := getoffer.NewHandler(stubOfferFinder{offer: nil})

	_, err := h.Handle(context.Background(), getoffer.Query{
		UUID:         uuid.New(),
		CurrentRoles: []enum.Role{enum.RoleSuperAdmin},
	})

	assert.ErrorIs(t, err, service.ErrOfferNotFound)
}

func intPtr(v int) *int { return &v }
