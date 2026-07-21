package unit_test

import (
	"context"
	"testing"

	getoffers "api/internal/application/query/get_offers"
	"api/internal/domain/enum"
	"api/internal/domain/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubOfferLister is a hand-written test double for the getoffers.OfferLister port.
type stubOfferLister struct {
	gotFilter repository.OfferFilter
	result    repository.OfferListResult
	err       error
}

func (s *stubOfferLister) List(_ context.Context, f repository.OfferFilter) (repository.OfferListResult, error) {
	s.gotFilter = f
	return s.result, s.err
}

func TestGetOffers_AlwaysScopedToCallerOwnAgency(t *testing.T) {
	lister := &stubOfferLister{}
	h := getoffers.NewHandler(lister)

	_, err := h.Handle(context.Background(), getoffers.Query{AgencyID: 1})

	require.NoError(t, err)
	require.NotNil(t, lister.gotFilter.AgencyID)
	assert.Equal(t, 1, *lister.gotFilter.AgencyID)
	assert.Nil(t, lister.gotFilter.Status, "role has no bearing on visibility: no status filter means any status")
}

func TestGetOffers_StatusFilterPassedThrough(t *testing.T) {
	lister := &stubOfferLister{}
	h := getoffers.NewHandler(lister)

	published := enum.OfferStatusPublished
	_, err := h.Handle(context.Background(), getoffers.Query{
		AgencyID: 1,
		Status:   &published,
	})

	require.NoError(t, err)
	require.NotNil(t, lister.gotFilter.Status)
	assert.Equal(t, enum.OfferStatusPublished, *lister.gotFilter.Status)
}

func TestGetOffers_CreatedByFilterPassedThrough(t *testing.T) {
	lister := &stubOfferLister{}
	h := getoffers.NewHandler(lister)

	createdBy := 42
	_, err := h.Handle(context.Background(), getoffers.Query{
		AgencyID:  1,
		CreatedBy: &createdBy,
	})

	require.NoError(t, err)
	require.NotNil(t, lister.gotFilter.CreatedBy)
	assert.Equal(t, 42, *lister.gotFilter.CreatedBy)
}

func TestGetOffers_MapsResultToOfferResults(t *testing.T) {
	lister := &stubOfferLister{result: repository.OfferListResult{TotalCount: 1}}
	h := getoffers.NewHandler(lister)

	res, err := h.Handle(context.Background(), getoffers.Query{AgencyID: 1})

	require.NoError(t, err)
	assert.EqualValues(t, 1, res.TotalCount)
	assert.Empty(t, res.Offers)
}
