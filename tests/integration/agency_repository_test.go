package integration_test

import (
	"context"
	"testing"
	"time"

	"api/internal/domain/agency"
	"api/internal/infrastructure/persistence/postgres"
	"api/internal/infrastructure/persistence/postgres/db"
	pgrepo "api/internal/infrastructure/persistence/postgres/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAgencyRepo(t *testing.T) *pgrepo.AgencyRepository {
	t.Helper()
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, testDB(t))
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pgrepo.NewAgencyRepository(db.New(pool))
}

func TestAgencyRepository_StoreThenFindByID_ReturnsStoredAgency(t *testing.T) {
	repo := newAgencyRepo(t)
	ctx := context.Background()

	a := agency.Agency{
		UUID:      uuid.New(),
		Name:      "Test Agency " + uuid.NewString(),
		Status:    agency.StatusActive,
		CreatedAt: time.Now().Truncate(time.Second),
	}

	id, err := repo.Store(ctx, a)
	require.NoError(t, err)
	require.Greater(t, id, 0)

	found, err := repo.FindByID(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, a.UUID, found.UUID)
	assert.Equal(t, a.Name, found.Name)
	assert.Equal(t, agency.StatusActive, found.Status)
	assert.Nil(t, found.DeletedAt)
}

func TestAgencyRepository_Exists_TrueForStoredAgency(t *testing.T) {
	repo := newAgencyRepo(t)
	ctx := context.Background()

	id, err := repo.Store(ctx, agency.Agency{
		UUID:      uuid.New(),
		Name:      "Existence Check Agency " + uuid.NewString(),
		Status:    agency.StatusActive,
		CreatedAt: time.Now(),
	})
	require.NoError(t, err)

	exists, err := repo.Exists(ctx, id)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestAgencyRepository_Exists_FalseForUnknownID(t *testing.T) {
	repo := newAgencyRepo(t)

	exists, err := repo.Exists(context.Background(), -1)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestAgencyRepository_SetStatus_UpdatesStoredStatus(t *testing.T) {
	repo := newAgencyRepo(t)
	ctx := context.Background()

	id, err := repo.Store(ctx, agency.Agency{
		UUID:      uuid.New(),
		Name:      "Deactivation Check Agency " + uuid.NewString(),
		Status:    agency.StatusActive,
		CreatedAt: time.Now(),
	})
	require.NoError(t, err)

	require.NoError(t, repo.SetStatus(ctx, id, agency.StatusInactive))

	found, err := repo.FindByID(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, agency.StatusInactive, found.Status)
}

func TestAgencyRepository_FindByID_ReturnsNilForUnknownID(t *testing.T) {
	repo := newAgencyRepo(t)

	found, err := repo.FindByID(context.Background(), -1)
	require.NoError(t, err)
	assert.Nil(t, found)
}
