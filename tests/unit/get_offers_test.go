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

func TestGetOffers_SuperAdmin_NoForcedFilters(t *testing.T) {
	lister := &stubOfferLister{}
	h := getoffers.NewHandler(lister)

	requestedAgency := 5
	_, err := h.Handle(context.Background(), getoffers.Query{
		AgencyID:        &requestedAgency,
		CurrentAgencyID: intPtr(1),
		CurrentRoles:    []enum.Role{enum.RoleSuperAdmin},
	})

	require.NoError(t, err)
	require.NotNil(t, lister.gotFilter.AgencyID)
	assert.Equal(t, requestedAgency, *lister.gotFilter.AgencyID, "super admin's requested filter must pass through untouched")
	assert.Nil(t, lister.gotFilter.Status, "super admin is not restricted to published offers")
}

func TestGetOffers_Agent_AgencyFilterForcedToOwnAgency(t *testing.T) {
	lister := &stubOfferLister{}
	h := getoffers.NewHandler(lister)

	requestedAgency := 999 // attempt to query another agency
	_, err := h.Handle(context.Background(), getoffers.Query{
		AgencyID:        &requestedAgency,
		CurrentAgencyID: intPtr(1),
		CurrentRoles:    []enum.Role{enum.RoleAgent},
	})

	require.NoError(t, err)
	require.NotNil(t, lister.gotFilter.AgencyID)
	assert.Equal(t, 1, *lister.gotFilter.AgencyID, "agent must never see another agency's offers, regardless of requested filter")
	assert.Nil(t, lister.gotFilter.Status, "agent sees any status within their own agency")
}

func TestGetOffers_PlainUser_StatusForcedToPublished(t *testing.T) {
	lister := &stubOfferLister{}
	h := getoffers.NewHandler(lister)

	_, err := h.Handle(context.Background(), getoffers.Query{
		CurrentAgencyID: intPtr(1),
		CurrentRoles:    []enum.Role{enum.RoleUser},
	})

	require.NoError(t, err)
	require.NotNil(t, lister.gotFilter.Status)
	assert.Equal(t, enum.OfferStatusPublished, *lister.gotFilter.Status)
}

func TestGetOffers_AgentWithoutAgency_FallsBackToPublishedOnly(t *testing.T) {
	lister := &stubOfferLister{}
	h := getoffers.NewHandler(lister)

	_, err := h.Handle(context.Background(), getoffers.Query{
		CurrentAgencyID: nil,
		CurrentRoles:    []enum.Role{enum.RoleAgent},
	})

	require.NoError(t, err)
	require.NotNil(t, lister.gotFilter.Status)
	assert.Equal(t, enum.OfferStatusPublished, *lister.gotFilter.Status)
	assert.Nil(t, lister.gotFilter.AgencyID)
}

func TestGetOffers_MapsResultToOfferResults(t *testing.T) {
	lister := &stubOfferLister{result: repository.OfferListResult{TotalCount: 1}}
	h := getoffers.NewHandler(lister)

	res, err := h.Handle(context.Background(), getoffers.Query{
		CurrentRoles: []enum.Role{enum.RoleSuperAdmin},
	})

	require.NoError(t, err)
	assert.EqualValues(t, 1, res.TotalCount)
	assert.Empty(t, res.Offers)
}
