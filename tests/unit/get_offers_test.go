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

func TestGetOffers_StaffAgency_AgencyFilterForcedToOwnAgency(t *testing.T) {
	lister := &stubOfferLister{}
	h := getoffers.NewHandler(lister)

	requestedAgency := 999 // attempt to query another agency
	_, err := h.Handle(context.Background(), getoffers.Query{
		AgencyID:        &requestedAgency,
		CurrentAgencyID: intPtr(1),
	})

	require.NoError(t, err)
	require.NotNil(t, lister.gotFilter.AgencyID)
	assert.Equal(t, 1, *lister.gotFilter.AgencyID, "an authenticated staff member must never see another agency's offers, regardless of requested filter")
	assert.Nil(t, lister.gotFilter.Status, "staff sees any status within their own agency")
}

func TestGetOffers_Anonymous_StatusForcedToPublished(t *testing.T) {
	lister := &stubOfferLister{}
	h := getoffers.NewHandler(lister)

	requestedAgency := 5
	_, err := h.Handle(context.Background(), getoffers.Query{
		AgencyID:        &requestedAgency,
		CurrentAgencyID: nil,
	})

	require.NoError(t, err)
	require.NotNil(t, lister.gotFilter.Status)
	assert.Equal(t, enum.OfferStatusPublished, *lister.gotFilter.Status)
}

func TestGetOffers_MapsResultToOfferResults(t *testing.T) {
	lister := &stubOfferLister{result: repository.OfferListResult{TotalCount: 1}}
	h := getoffers.NewHandler(lister)

	res, err := h.Handle(context.Background(), getoffers.Query{})

	require.NoError(t, err)
	assert.EqualValues(t, 1, res.TotalCount)
	assert.Empty(t, res.Offers)
}
