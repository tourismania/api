package unit_test

import (
	"context"
	"testing"

	getpublishedoffer "api/internal/application/query/get_published_offer"
	"api/internal/domain/entity"
	"api/internal/domain/enum"
	"api/internal/domain/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPublishedOffer_Published_Visible(t *testing.T) {
	offer := &entity.Offer{UUID: uuid.New(), AgencyID: 7, Status: enum.OfferStatusPublished}
	h := getpublishedoffer.NewHandler(stubOfferFinder{offer: offer})

	res, err := h.Handle(context.Background(), getpublishedoffer.Query{UUID: offer.UUID})

	require.NoError(t, err)
	assert.Equal(t, offer.UUID, res.UUID)
	assert.Equal(t, offer.AgencyID, res.AgencyID)
}

func TestGetPublishedOffer_Draft_NotFound(t *testing.T) {
	offer := &entity.Offer{UUID: uuid.New(), AgencyID: 7, Status: enum.OfferStatusDraft}
	h := getpublishedoffer.NewHandler(stubOfferFinder{offer: offer})

	_, err := h.Handle(context.Background(), getpublishedoffer.Query{UUID: offer.UUID})

	assert.ErrorIs(t, err, service.ErrOfferNotFound)
}

func TestGetPublishedOffer_Ready_NotFound(t *testing.T) {
	offer := &entity.Offer{UUID: uuid.New(), AgencyID: 7, Status: enum.OfferStatusReady}
	h := getpublishedoffer.NewHandler(stubOfferFinder{offer: offer})

	_, err := h.Handle(context.Background(), getpublishedoffer.Query{UUID: offer.UUID})

	assert.ErrorIs(t, err, service.ErrOfferNotFound)
}

func TestGetPublishedOffer_NotFound_ReturnsErrOfferNotFound(t *testing.T) {
	h := getpublishedoffer.NewHandler(stubOfferFinder{offer: nil})

	_, err := h.Handle(context.Background(), getpublishedoffer.Query{UUID: uuid.New()})

	assert.ErrorIs(t, err, service.ErrOfferNotFound)
}
