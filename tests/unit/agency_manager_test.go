package unit_test

import (
	"context"
	"testing"
	"time"

	"api/internal/domain/entity"
	"api/internal/domain/enum"
	"api/internal/domain/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAgencyRepo is a hand-written test double for repository.AgencyRepository.
type mockAgencyRepo struct {
	storeID      int
	storeErr     error
	storedAgency entity.Agency

	existsVal bool
	existsErr error

	setStatusErr error
	setStatusID  int
	setStatusTo  enum.AgencyStatus
}

func (m *mockAgencyRepo) Store(_ context.Context, a entity.Agency) (int, error) {
	m.storedAgency = a
	return m.storeID, m.storeErr
}

func (m *mockAgencyRepo) FindByID(_ context.Context, _ int) (*entity.Agency, error) {
	return nil, nil
}

func (m *mockAgencyRepo) SetStatus(_ context.Context, id int, status enum.AgencyStatus) error {
	m.setStatusID = id
	m.setStatusTo = status
	return m.setStatusErr
}

func (m *mockAgencyRepo) Exists(_ context.Context, _ int) (bool, error) {
	return m.existsVal, m.existsErr
}

func TestAgencyManager_Create_GeneratesUUIDAndActiveStatus(t *testing.T) {
	repo := &mockAgencyRepo{storeID: 7}
	mgr := service.NewAgencyManager(repo)

	agency, err := mgr.Create(context.Background(), "Acme Travel")

	require.NoError(t, err)
	assert.Equal(t, 7, agency.ID)
	assert.Equal(t, "Acme Travel", agency.Name)
	assert.Equal(t, enum.AgencyStatusActive, agency.Status)
	assert.NotEqual(t, uuid.Nil, agency.UUID)
	assert.WithinDuration(t, time.Now(), agency.CreatedAt, time.Second)
	assert.Equal(t, agency.UUID, repo.storedAgency.UUID, "the persisted row should carry the same UUID returned to the caller")
}

func TestAgencyManager_Create_EmptyName_ReturnsError(t *testing.T) {
	mgr := service.NewAgencyManager(&mockAgencyRepo{})

	_, err := mgr.Create(context.Background(), "")

	assert.Error(t, err)
}

func TestAgencyManager_Deactivate_NotFound_ReturnsError(t *testing.T) {
	repo := &mockAgencyRepo{existsVal: false}
	mgr := service.NewAgencyManager(repo)

	err := mgr.Deactivate(context.Background(), 5)

	assert.ErrorIs(t, err, service.ErrAgencyNotFound)
}

func TestAgencyManager_Deactivate_Exists_SetsInactiveStatus(t *testing.T) {
	repo := &mockAgencyRepo{existsVal: true}
	mgr := service.NewAgencyManager(repo)

	err := mgr.Deactivate(context.Background(), 5)

	require.NoError(t, err)
	assert.Equal(t, 5, repo.setStatusID)
	assert.Equal(t, enum.AgencyStatusInactive, repo.setStatusTo)
}
