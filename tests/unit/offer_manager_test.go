package unit_test

import (
	"api/internal/domain/agency"
	"api/internal/domain/offer"
	"api/internal/domain/user"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockOfferRepo is a hand-written test double for offer.Repository.
type mockOfferRepo struct {
	storeID     int
	storeErr    error
	storedOffer offer.Offer

	findByUUIDOffer *offer.Offer
	findByUUIDErr   error

	updateErr    error
	updatedOffer offer.Offer

	softDeleteErr error
	softDeletedID uuid.UUID
}

func (m *mockOfferRepo) Store(_ context.Context, o offer.Offer) (int, error) {
	m.storedOffer = o
	return m.storeID, m.storeErr
}

func (m *mockOfferRepo) FindByUUID(_ context.Context, _ uuid.UUID) (*offer.Offer, error) {
	return m.findByUUIDOffer, m.findByUUIDErr
}

func (m *mockOfferRepo) List(_ context.Context, _ offer.Filter) (offer.ListResult, error) {
	return offer.ListResult{}, nil
}

func (m *mockOfferRepo) Update(_ context.Context, o offer.Offer) error {
	m.updatedOffer = o
	return m.updateErr
}

func (m *mockOfferRepo) SoftDelete(_ context.Context, id uuid.UUID) error {
	m.softDeletedID = id
	return m.softDeleteErr
}

func activeAgency(id int) *agency.Agency {
	return &agency.Agency{ID: id, Status: agency.StatusActive}
}

func inactiveAgency(id int) *agency.Agency {
	return &agency.Agency{ID: id, Status: agency.StatusInactive}
}

// agentActor is the default actor for tests exercising invariants other
// than the write-role gate itself: an authenticated ROLE_AGENT belonging
// to agencyID.
func agentActor(agencyID int) user.Actor {
	return user.Actor{AgencyID: agencyID, Roles: []user.Role{user.RoleAgent}}
}

func TestOfferManager_Insert_ValidInput_DerivesAgencyFromActor(t *testing.T) {
	offers := &mockOfferRepo{storeID: 9}
	agencies := &mockAgencyRepo{findByIDAgency: activeAgency(3)}
	mgr := offer.NewManager(offers, agencies)

	created, err := mgr.Insert(context.Background(), "Sochi package", "5 nights", offer.StatusDraft, user.Actor{
		UserID:   42,
		AgencyID: 3,
		Roles:    []user.Role{user.RoleAgent},
	})

	require.NoError(t, err)
	assert.Equal(t, 9, created.ID)
	assert.Equal(t, 3, created.AgencyID)
	assert.Equal(t, 42, created.CreatedBy)
	assert.Equal(t, offer.StatusDraft, created.Status)
	assert.NotEqual(t, uuid.Nil, created.UUID)
	assert.WithinDuration(t, time.Now(), created.CreatedAt, time.Second)
	assert.Equal(t, 3, offers.storedOffer.AgencyID, "AgencyID must be derived from the actor, never from caller input")
}

func TestOfferManager_Insert_SuperAdminRole_Allowed(t *testing.T) {
	offers := &mockOfferRepo{storeID: 1}
	agencies := &mockAgencyRepo{findByIDAgency: activeAgency(1)}
	mgr := offer.NewManager(offers, agencies)

	_, err := mgr.Insert(context.Background(), "Title", "desc", offer.StatusDraft, user.Actor{
		AgencyID: 1,
		Roles:    []user.Role{user.RoleSuperAdmin},
	})

	require.NoError(t, err)
}

func TestOfferManager_Insert_RoleUser_ReturnsInsufficientRole(t *testing.T) {
	mgr := offer.NewManager(&mockOfferRepo{}, &mockAgencyRepo{findByIDAgency: activeAgency(1)})

	_, err := mgr.Insert(context.Background(), "Title", "desc", offer.StatusDraft, user.Actor{
		AgencyID: 1,
		Roles:    []user.Role{user.RoleUser},
	})

	assert.ErrorIs(t, err, offer.ErrInsufficientRole)
}

func TestOfferManager_Insert_NoRoles_ReturnsInsufficientRole(t *testing.T) {
	mgr := offer.NewManager(&mockOfferRepo{}, &mockAgencyRepo{findByIDAgency: activeAgency(1)})

	_, err := mgr.Insert(context.Background(), "Title", "desc", offer.StatusDraft, user.Actor{
		AgencyID: 1,
	})

	assert.ErrorIs(t, err, offer.ErrInsufficientRole)
}

func TestOfferManager_Insert_EmptyTitle_ReturnsError(t *testing.T) {
	mgr := offer.NewManager(&mockOfferRepo{}, &mockAgencyRepo{findByIDAgency: activeAgency(1)})

	_, err := mgr.Insert(context.Background(), "", "desc", offer.StatusDraft, agentActor(1))

	assert.ErrorIs(t, err, offer.ErrTitleInvalid)
}

func TestOfferManager_Insert_TitleTooLong_ReturnsError(t *testing.T) {
	mgr := offer.NewManager(&mockOfferRepo{}, &mockAgencyRepo{findByIDAgency: activeAgency(1)})

	_, err := mgr.Insert(context.Background(), strings.Repeat("a", offer.TitleMaxLength+1), "desc", offer.StatusDraft, agentActor(1))

	assert.ErrorIs(t, err, offer.ErrTitleInvalid)
}

func TestOfferManager_Insert_InvalidStatus_ReturnsError(t *testing.T) {
	mgr := offer.NewManager(&mockOfferRepo{}, &mockAgencyRepo{findByIDAgency: activeAgency(1)})

	_, err := mgr.Insert(context.Background(), "Title", "desc", offer.Status("archived"), agentActor(1))

	assert.ErrorIs(t, err, offer.ErrStatusInvalid)
}

func TestOfferManager_Insert_AgencyNotFound_ReturnsError(t *testing.T) {
	mgr := offer.NewManager(&mockOfferRepo{}, &mockAgencyRepo{findByIDAgency: nil})

	_, err := mgr.Insert(context.Background(), "Title", "desc", offer.StatusDraft, agentActor(1))

	assert.ErrorIs(t, err, agency.ErrNotFound)
}

func TestOfferManager_Insert_AgencyInactive_ReturnsError(t *testing.T) {
	mgr := offer.NewManager(&mockOfferRepo{}, &mockAgencyRepo{findByIDAgency: inactiveAgency(1)})

	_, err := mgr.Insert(context.Background(), "Title", "desc", offer.StatusDraft, agentActor(1))

	assert.ErrorIs(t, err, agency.ErrInactive)
}

func TestOfferManager_Update_RoleUser_ReturnsInsufficientRole(t *testing.T) {
	existing := &offer.Offer{UUID: uuid.New(), AgencyID: 1, Title: "Old", Status: offer.StatusDraft}
	mgr := offer.NewManager(&mockOfferRepo{findByUUIDOffer: existing}, &mockAgencyRepo{})

	newTitle := "New title"
	_, err := mgr.Update(context.Background(), existing.UUID, &newTitle, nil, nil, user.Actor{
		AgencyID: 1,
		Roles:    []user.Role{user.RoleUser},
	})

	assert.ErrorIs(t, err, offer.ErrInsufficientRole)
}

func TestOfferManager_Update_NotFound_ReturnsError(t *testing.T) {
	mgr := offer.NewManager(&mockOfferRepo{findByUUIDOffer: nil}, &mockAgencyRepo{})

	_, err := mgr.Update(context.Background(), uuid.New(), nil, nil, nil, agentActor(1))

	assert.ErrorIs(t, err, offer.ErrNotFound)
}

func TestOfferManager_Update_DifferentAgency_ReturnsNotFound(t *testing.T) {
	existing := &offer.Offer{UUID: uuid.New(), AgencyID: 1, Title: "Old", Status: offer.StatusDraft}
	mgr := offer.NewManager(&mockOfferRepo{findByUUIDOffer: existing}, &mockAgencyRepo{})

	_, err := mgr.Update(context.Background(), existing.UUID, nil, nil, nil, agentActor(2))

	assert.ErrorIs(t, err, offer.ErrNotFound)
}

func TestOfferManager_Update_SameAgency_AppliesPartialChanges(t *testing.T) {
	existing := &offer.Offer{UUID: uuid.New(), AgencyID: 1, Title: "Old", Description: "Old desc", Status: offer.StatusDraft}
	offers := &mockOfferRepo{findByUUIDOffer: existing}
	mgr := offer.NewManager(offers, &mockAgencyRepo{})

	newTitle := "New title"
	updated, err := mgr.Update(context.Background(), existing.UUID, &newTitle, nil, nil, agentActor(1))

	require.NoError(t, err)
	assert.Equal(t, "New title", updated.Title)
	assert.Equal(t, "Old desc", updated.Description, "description was not supplied, must stay unchanged")
	assert.Equal(t, "New title", offers.updatedOffer.Title)
}

func TestOfferManager_Update_DifferentAgency_SuperAdminStillNotFound(t *testing.T) {
	// 1 user = 1 agency: there is no role-based bypass, not even for
	// ROLE_SUPER_ADMIN — ownership is strict agency equality, and a
	// mismatch is reported as not-found, not forbidden.
	existing := &offer.Offer{UUID: uuid.New(), AgencyID: 1, Title: "Old", Status: offer.StatusDraft}
	mgr := offer.NewManager(&mockOfferRepo{findByUUIDOffer: existing}, &mockAgencyRepo{})

	newTitle := "Admin edit"
	_, err := mgr.Update(context.Background(), existing.UUID, &newTitle, nil, nil, user.Actor{
		AgencyID: 999,
		Roles:    []user.Role{user.RoleSuperAdmin},
	})

	assert.ErrorIs(t, err, offer.ErrNotFound)
}

func TestOfferManager_Update_InvalidStatus_ReturnsError(t *testing.T) {
	existing := &offer.Offer{UUID: uuid.New(), AgencyID: 1, Title: "Old", Status: offer.StatusDraft}
	mgr := offer.NewManager(&mockOfferRepo{findByUUIDOffer: existing}, &mockAgencyRepo{})

	badStatus := offer.Status("archived")
	_, err := mgr.Update(context.Background(), existing.UUID, nil, nil, &badStatus, agentActor(1))

	assert.ErrorIs(t, err, offer.ErrStatusInvalid)
}

func TestOfferManager_Delete_RoleUser_ReturnsInsufficientRole(t *testing.T) {
	existing := &offer.Offer{UUID: uuid.New(), AgencyID: 1}
	mgr := offer.NewManager(&mockOfferRepo{findByUUIDOffer: existing}, &mockAgencyRepo{})

	err := mgr.Delete(context.Background(), existing.UUID, user.Actor{
		AgencyID: 1,
		Roles:    []user.Role{user.RoleUser},
	})

	assert.ErrorIs(t, err, offer.ErrInsufficientRole)
}

func TestOfferManager_Delete_NotFound_ReturnsError(t *testing.T) {
	mgr := offer.NewManager(&mockOfferRepo{findByUUIDOffer: nil}, &mockAgencyRepo{})

	err := mgr.Delete(context.Background(), uuid.New(), agentActor(1))

	assert.ErrorIs(t, err, offer.ErrNotFound)
}

func TestOfferManager_Delete_DifferentAgency_ReturnsNotFound(t *testing.T) {
	existing := &offer.Offer{UUID: uuid.New(), AgencyID: 1}
	mgr := offer.NewManager(&mockOfferRepo{findByUUIDOffer: existing}, &mockAgencyRepo{})

	err := mgr.Delete(context.Background(), existing.UUID, agentActor(2))

	assert.ErrorIs(t, err, offer.ErrNotFound)
}

func TestOfferManager_Delete_SameAgency_SoftDeletes(t *testing.T) {
	existing := &offer.Offer{UUID: uuid.New(), AgencyID: 1}
	offers := &mockOfferRepo{findByUUIDOffer: existing}
	mgr := offer.NewManager(offers, &mockAgencyRepo{})

	err := mgr.Delete(context.Background(), existing.UUID, agentActor(1))

	require.NoError(t, err)
	assert.Equal(t, existing.UUID, offers.softDeletedID)
}
