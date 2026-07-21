package unit_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"api/internal/domain/entity"
	"api/internal/domain/enum"
	"api/internal/domain/repository"
	"api/internal/domain/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockOfferRepo is a hand-written test double for repository.OfferRepository.
type mockOfferRepo struct {
	storeID     int
	storeErr    error
	storedOffer entity.Offer

	findByUUIDOffer *entity.Offer
	findByUUIDErr   error

	updateErr    error
	updatedOffer entity.Offer

	softDeleteErr error
	softDeletedID uuid.UUID
}

func (m *mockOfferRepo) Store(_ context.Context, o entity.Offer) (int, error) {
	m.storedOffer = o
	return m.storeID, m.storeErr
}

func (m *mockOfferRepo) FindByUUID(_ context.Context, _ uuid.UUID) (*entity.Offer, error) {
	return m.findByUUIDOffer, m.findByUUIDErr
}

func (m *mockOfferRepo) List(_ context.Context, _ repository.OfferFilter) (repository.OfferListResult, error) {
	return repository.OfferListResult{}, nil
}

func (m *mockOfferRepo) Update(_ context.Context, o entity.Offer) error {
	m.updatedOffer = o
	return m.updateErr
}

func (m *mockOfferRepo) SoftDelete(_ context.Context, id uuid.UUID) error {
	m.softDeletedID = id
	return m.softDeleteErr
}

func activeAgency(id int) *entity.Agency {
	return &entity.Agency{ID: id, Status: enum.AgencyStatusActive}
}

func inactiveAgency(id int) *entity.Agency {
	return &entity.Agency{ID: id, Status: enum.AgencyStatusInactive}
}

func TestOfferManager_Insert_ValidInput_DerivesAgencyFromActor(t *testing.T) {
	offers := &mockOfferRepo{storeID: 9}
	agencies := &mockAgencyRepo{findByIDAgency: activeAgency(3)}
	mgr := service.NewOfferManager(offers, agencies)

	offer, err := mgr.Insert(context.Background(), "Sochi package", "5 nights", enum.OfferStatusDraft, service.Actor{
		UserID:   42,
		AgencyID: 3,
	})

	require.NoError(t, err)
	assert.Equal(t, 9, offer.ID)
	assert.Equal(t, 3, offer.AgencyID)
	assert.Equal(t, 42, offer.CreatedBy)
	assert.Equal(t, enum.OfferStatusDraft, offer.Status)
	assert.NotEqual(t, uuid.Nil, offer.UUID)
	assert.WithinDuration(t, time.Now(), offer.CreatedAt, time.Second)
	assert.Equal(t, 3, offers.storedOffer.AgencyID, "AgencyID must be derived from the actor, never from caller input")
}

func TestOfferManager_Insert_EmptyTitle_ReturnsError(t *testing.T) {
	mgr := service.NewOfferManager(&mockOfferRepo{}, &mockAgencyRepo{findByIDAgency: activeAgency(1)})

	_, err := mgr.Insert(context.Background(), "", "desc", enum.OfferStatusDraft, service.Actor{
		AgencyID: 1,
	})

	assert.ErrorIs(t, err, service.ErrOfferTitleInvalid)
}

func TestOfferManager_Insert_TitleTooLong_ReturnsError(t *testing.T) {
	mgr := service.NewOfferManager(&mockOfferRepo{}, &mockAgencyRepo{findByIDAgency: activeAgency(1)})

	_, err := mgr.Insert(context.Background(), strings.Repeat("a", entity.OfferTitleMaxLength+1), "desc", enum.OfferStatusDraft, service.Actor{
		AgencyID: 1,
	})

	assert.ErrorIs(t, err, service.ErrOfferTitleInvalid)
}

func TestOfferManager_Insert_InvalidStatus_ReturnsError(t *testing.T) {
	mgr := service.NewOfferManager(&mockOfferRepo{}, &mockAgencyRepo{findByIDAgency: activeAgency(1)})

	_, err := mgr.Insert(context.Background(), "Title", "desc", enum.OfferStatus("archived"), service.Actor{
		AgencyID: 1,
	})

	assert.ErrorIs(t, err, service.ErrOfferStatusInvalid)
}

func TestOfferManager_Insert_AgencyNotFound_ReturnsError(t *testing.T) {
	mgr := service.NewOfferManager(&mockOfferRepo{}, &mockAgencyRepo{findByIDAgency: nil})

	_, err := mgr.Insert(context.Background(), "Title", "desc", enum.OfferStatusDraft, service.Actor{
		AgencyID: 1,
	})

	assert.ErrorIs(t, err, service.ErrAgencyNotFound)
}

func TestOfferManager_Insert_AgencyInactive_ReturnsError(t *testing.T) {
	mgr := service.NewOfferManager(&mockOfferRepo{}, &mockAgencyRepo{findByIDAgency: inactiveAgency(1)})

	_, err := mgr.Insert(context.Background(), "Title", "desc", enum.OfferStatusDraft, service.Actor{
		AgencyID: 1,
	})

	assert.ErrorIs(t, err, service.ErrAgencyInactive)
}

func TestOfferManager_Update_NotFound_ReturnsError(t *testing.T) {
	mgr := service.NewOfferManager(&mockOfferRepo{findByUUIDOffer: nil}, &mockAgencyRepo{})

	_, err := mgr.Update(context.Background(), uuid.New(), nil, nil, nil, service.Actor{
		AgencyID: 1,
	})

	assert.ErrorIs(t, err, service.ErrOfferNotFound)
}

func TestOfferManager_Update_DifferentAgency_ReturnsNotFound(t *testing.T) {
	existing := &entity.Offer{UUID: uuid.New(), AgencyID: 1, Title: "Old", Status: enum.OfferStatusDraft}
	mgr := service.NewOfferManager(&mockOfferRepo{findByUUIDOffer: existing}, &mockAgencyRepo{})

	_, err := mgr.Update(context.Background(), existing.UUID, nil, nil, nil, service.Actor{
		AgencyID: 2,
	})

	assert.ErrorIs(t, err, service.ErrOfferNotFound)
}

func TestOfferManager_Update_SameAgency_AppliesPartialChanges(t *testing.T) {
	existing := &entity.Offer{UUID: uuid.New(), AgencyID: 1, Title: "Old", Description: "Old desc", Status: enum.OfferStatusDraft}
	offers := &mockOfferRepo{findByUUIDOffer: existing}
	mgr := service.NewOfferManager(offers, &mockAgencyRepo{})

	newTitle := "New title"
	updated, err := mgr.Update(context.Background(), existing.UUID, &newTitle, nil, nil, service.Actor{
		AgencyID: 1,
	})

	require.NoError(t, err)
	assert.Equal(t, "New title", updated.Title)
	assert.Equal(t, "Old desc", updated.Description, "description was not supplied, must stay unchanged")
	assert.Equal(t, "New title", offers.updatedOffer.Title)
}

func TestOfferManager_Update_DifferentAgency_SuperAdminStillNotFound(t *testing.T) {
	// 1 user = 1 agency: there is no role-based bypass, not even for
	// ROLE_SUPER_ADMIN — ownership is strict agency equality, and a
	// mismatch is reported as not-found, not forbidden.
	existing := &entity.Offer{UUID: uuid.New(), AgencyID: 1, Title: "Old", Status: enum.OfferStatusDraft}
	mgr := service.NewOfferManager(&mockOfferRepo{findByUUIDOffer: existing}, &mockAgencyRepo{})

	newTitle := "Admin edit"
	_, err := mgr.Update(context.Background(), existing.UUID, &newTitle, nil, nil, service.Actor{
		AgencyID: 999,
	})

	assert.ErrorIs(t, err, service.ErrOfferNotFound)
}

func TestOfferManager_Update_InvalidStatus_ReturnsError(t *testing.T) {
	existing := &entity.Offer{UUID: uuid.New(), AgencyID: 1, Title: "Old", Status: enum.OfferStatusDraft}
	mgr := service.NewOfferManager(&mockOfferRepo{findByUUIDOffer: existing}, &mockAgencyRepo{})

	badStatus := enum.OfferStatus("archived")
	_, err := mgr.Update(context.Background(), existing.UUID, nil, nil, &badStatus, service.Actor{
		AgencyID: 1,
	})

	assert.ErrorIs(t, err, service.ErrOfferStatusInvalid)
}

func TestOfferManager_Delete_NotFound_ReturnsError(t *testing.T) {
	mgr := service.NewOfferManager(&mockOfferRepo{findByUUIDOffer: nil}, &mockAgencyRepo{})

	err := mgr.Delete(context.Background(), uuid.New(), service.Actor{AgencyID: 1})

	assert.ErrorIs(t, err, service.ErrOfferNotFound)
}

func TestOfferManager_Delete_DifferentAgency_ReturnsNotFound(t *testing.T) {
	existing := &entity.Offer{UUID: uuid.New(), AgencyID: 1}
	mgr := service.NewOfferManager(&mockOfferRepo{findByUUIDOffer: existing}, &mockAgencyRepo{})

	err := mgr.Delete(context.Background(), existing.UUID, service.Actor{
		AgencyID: 2,
	})

	assert.ErrorIs(t, err, service.ErrOfferNotFound)
}

func TestOfferManager_Delete_SameAgency_SoftDeletes(t *testing.T) {
	existing := &entity.Offer{UUID: uuid.New(), AgencyID: 1}
	offers := &mockOfferRepo{findByUUIDOffer: existing}
	mgr := service.NewOfferManager(offers, &mockAgencyRepo{})

	err := mgr.Delete(context.Background(), existing.UUID, service.Actor{
		AgencyID: 1,
	})

	require.NoError(t, err)
	assert.Equal(t, existing.UUID, offers.softDeletedID)
}
