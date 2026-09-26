package unit_test

import (
	"api/internal/domain/agency"
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAgencyRepo is a hand-written test double for agency.Repository.
// Reused by offer_manager_test.go: offer.Manager consults AgencyRepository too.
type mockAgencyRepo struct {
	storeID      int
	storeErr     error
	storedAgency agency.Agency

	existsVal bool
	existsErr error

	setStatusErr error
	setStatusID  int
	setStatusTo  agency.Status

	// findByIDAgency/findByIDErr let callers other than agency.Manager
	// (e.g. offer.Manager) stub a specific FindByID response. Zero value
	// preserves the original "not found" behaviour.
	findByIDAgency *agency.Agency
	findByIDErr    error
}

func (m *mockAgencyRepo) Store(_ context.Context, a agency.Agency) (int, error) {
	m.storedAgency = a
	return m.storeID, m.storeErr
}

func (m *mockAgencyRepo) FindByID(_ context.Context, _ int) (*agency.Agency, error) {
	return m.findByIDAgency, m.findByIDErr
}

func (m *mockAgencyRepo) SetStatus(_ context.Context, id int, status agency.Status) error {
	m.setStatusID = id
	m.setStatusTo = status
	return m.setStatusErr
}

func (m *mockAgencyRepo) Exists(_ context.Context, _ int) (bool, error) {
	return m.existsVal, m.existsErr
}

func TestAgencyManager_Create_GeneratesUUIDAndActiveStatus(t *testing.T) {
	repo := &mockAgencyRepo{storeID: 7}
	mgr := agency.NewManager(repo)

	created, err := mgr.Create(context.Background(), "Acme Travel")

	require.NoError(t, err)
	assert.Equal(t, 7, created.ID)
	assert.Equal(t, "Acme Travel", created.Name)
	assert.Equal(t, agency.StatusActive, created.Status)
	assert.NotEqual(t, uuid.Nil, created.UUID)
	assert.WithinDuration(t, time.Now(), created.CreatedAt, time.Second)
	assert.Equal(t, created.UUID, repo.storedAgency.UUID, "the persisted row should carry the same UUID returned to the caller")
}

func TestAgencyManager_Create_EmptyName_ReturnsError(t *testing.T) {
	mgr := agency.NewManager(&mockAgencyRepo{})

	_, err := mgr.Create(context.Background(), "")

	assert.Error(t, err)
}

func TestAgencyManager_Deactivate_NotFound_ReturnsError(t *testing.T) {
	repo := &mockAgencyRepo{existsVal: false}
	mgr := agency.NewManager(repo)

	err := mgr.Deactivate(context.Background(), 5)

	assert.ErrorIs(t, err, agency.ErrNotFound)
}

func TestAgencyManager_Deactivate_Exists_SetsInactiveStatus(t *testing.T) {
	repo := &mockAgencyRepo{existsVal: true}
	mgr := agency.NewManager(repo)

	err := mgr.Deactivate(context.Background(), 5)

	require.NoError(t, err)
	assert.Equal(t, 5, repo.setStatusID)
	assert.Equal(t, agency.StatusInactive, repo.setStatusTo)
}

func TestAgencyManager_Activate_NotFound_ReturnsError(t *testing.T) {
	repo := &mockAgencyRepo{existsVal: false}
	mgr := agency.NewManager(repo)

	err := mgr.Activate(context.Background(), 5)

	assert.ErrorIs(t, err, agency.ErrNotFound)
}

func TestAgencyManager_Activate_Exists_SetsActiveStatus(t *testing.T) {
	repo := &mockAgencyRepo{existsVal: true}
	mgr := agency.NewManager(repo)

	err := mgr.Activate(context.Background(), 5)

	require.NoError(t, err)
	assert.Equal(t, 5, repo.setStatusID)
	assert.Equal(t, agency.StatusActive, repo.setStatusTo)
}
