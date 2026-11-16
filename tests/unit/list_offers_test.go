package unit_test

import (
	"context"
	"testing"

	"api/internal/application/apperror"
	listoffers "api/internal/application/query/list_offers"
	"api/internal/domain/offer"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubOfferLister is a hand-written test double for the listoffers.OfferLister port.
type stubOfferLister struct {
	gotFilter offer.Filter
	result    offer.ListResult
	err       error
}

func (s *stubOfferLister) List(_ context.Context, f offer.Filter) (offer.ListResult, error) {
	s.gotFilter = f
	return s.result, s.err
}

func TestListOffers_AlwaysScopedToCallerOwnAgency(t *testing.T) {
	lister := &stubOfferLister{}
	h := listoffers.NewHandler(lister, userRecordWithAgency(1))

	_, err := h.Handle(context.Background(), listoffers.Query{})

	require.NoError(t, err)
	require.NotNil(t, lister.gotFilter.AgencyID)
	assert.Equal(t, 1, *lister.gotFilter.AgencyID)
	assert.Nil(t, lister.gotFilter.Status, "role has no bearing on visibility: no status filter means any status")
}

func TestListOffers_StatusFilterPassedThrough(t *testing.T) {
	lister := &stubOfferLister{}
	h := listoffers.NewHandler(lister, userRecordWithAgency(1))

	published := offer.StatusPublished
	_, err := h.Handle(context.Background(), listoffers.Query{
		Status: &published,
	})

	require.NoError(t, err)
	require.NotNil(t, lister.gotFilter.Status)
	assert.Equal(t, offer.StatusPublished, *lister.gotFilter.Status)
}

func TestListOffers_CreatedByFilterPassedThrough(t *testing.T) {
	lister := &stubOfferLister{}
	h := listoffers.NewHandler(lister, userRecordWithAgency(1))

	createdBy := 42
	_, err := h.Handle(context.Background(), listoffers.Query{
		CreatedBy: &createdBy,
	})

	require.NoError(t, err)
	require.NotNil(t, lister.gotFilter.CreatedBy)
	assert.Equal(t, 42, *lister.gotFilter.CreatedBy)
}

func TestListOffers_MapsResultToOfferResults(t *testing.T) {
	lister := &stubOfferLister{result: offer.ListResult{TotalCount: 1}}
	h := listoffers.NewHandler(lister, userRecordWithAgency(1))

	res, err := h.Handle(context.Background(), listoffers.Query{})

	require.NoError(t, err)
	assert.EqualValues(t, 1, res.TotalCount)
	assert.Empty(t, res.Offers)
}

func TestListOffers_ActorNotFound_ReturnsUnauthenticated(t *testing.T) {
	lister := &stubOfferLister{}
	h := listoffers.NewHandler(lister, noUserFound())

	_, err := h.Handle(context.Background(), listoffers.Query{})

	assert.ErrorIs(t, err, apperror.ErrUnauthenticated)
}
