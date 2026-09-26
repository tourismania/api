package unit_test

import (
	"context"
	"testing"

	getpublishedoffer "api/internal/application/query/get_published_offer"
	"api/internal/domain/offer"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubOfferFinder is a hand-written test double for the
// getpublishedoffer.OfferFinder port.
type stubOfferFinder struct {
	offer *offer.Offer
	err   error
}

func (s stubOfferFinder) FindByUUID(_ context.Context, _ uuid.UUID) (*offer.Offer, error) {
	return s.offer, s.err
}

func TestGetPublishedOffer_Published_Visible(t *testing.T) {
	o := &offer.Offer{UUID: uuid.New(), AgencyID: 7, Status: offer.StatusPublished}
	h := getpublishedoffer.NewHandler(stubOfferFinder{offer: o}, stubFlightFinder{})

	res, err := h.Handle(context.Background(), getpublishedoffer.Query{UUID: o.UUID})

	require.NoError(t, err)
	assert.Equal(t, o.UUID, res.UUID)
	assert.Equal(t, o.AgencyID, res.AgencyID)
}

func TestGetPublishedOffer_Draft_NotFound(t *testing.T) {
	o := &offer.Offer{UUID: uuid.New(), AgencyID: 7, Status: offer.StatusDraft}
	h := getpublishedoffer.NewHandler(stubOfferFinder{offer: o}, stubFlightFinder{})

	_, err := h.Handle(context.Background(), getpublishedoffer.Query{UUID: o.UUID})

	assert.ErrorIs(t, err, offer.ErrNotFound)
}

func TestGetPublishedOffer_Ready_NotFound(t *testing.T) {
	o := &offer.Offer{UUID: uuid.New(), AgencyID: 7, Status: offer.StatusReady}
	h := getpublishedoffer.NewHandler(stubOfferFinder{offer: o}, stubFlightFinder{})

	_, err := h.Handle(context.Background(), getpublishedoffer.Query{UUID: o.UUID})

	assert.ErrorIs(t, err, offer.ErrNotFound)
}

func TestGetPublishedOffer_NotFound_ReturnsErrOfferNotFound(t *testing.T) {
	h := getpublishedoffer.NewHandler(stubOfferFinder{offer: nil}, stubFlightFinder{})

	_, err := h.Handle(context.Background(), getpublishedoffer.Query{UUID: uuid.New()})

	assert.ErrorIs(t, err, offer.ErrNotFound)
}
