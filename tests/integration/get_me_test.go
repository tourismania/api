package integration_test

import (
	"context"
	"testing"
	"time"

	getme "api/internal/application/query/get_me"
	"api/internal/domain/entity"
	"api/internal/domain/enum"
	"api/internal/domain/factory"
	"api/internal/domain/service"
	"api/internal/infrastructure/persistence/postgres"
	"api/internal/infrastructure/persistence/postgres/db"
	pgrepo "api/internal/infrastructure/persistence/postgres/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestGetMe_ReturnsLinkedAgency(t *testing.T) {
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, testDB(t))
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	queries := db.New(pool)

	agencyRepo := pgrepo.NewAgencyRepository(queries)
	userRepo := pgrepo.NewUserRepository(queries)

	agencyID, err := agencyRepo.Store(ctx, entity.Agency{
		UUID:      uuid.New(),
		Name:      "GetMe Test Agency " + uuid.NewString(),
		Status:    enum.AgencyStatusActive,
		CreatedAt: time.Now().Truncate(time.Second),
	})
	require.NoError(t, err)
	agency, err := agencyRepo.FindByID(ctx, agencyID)
	require.NoError(t, err)
	require.NotNil(t, agency)

	email := "getme+" + uuid.NewString() + "@example.com"
	_, err = userRepo.Store(ctx, entity.User{
		FirstName: "GetMe",
		LastName:  "Test",
		Email:     email,
		AgencyID:  agencyID,
	}, "hashed:secret")
	require.NoError(t, err)

	stored, err := queries.GetUserByEmail(ctx, email)
	require.NoError(t, err)

	rightsDescriber := service.NewRightsDescriber(factory.NewRightsDescribeFactory())
	handler := getme.NewHandler(userRepo, agencyRepo, rightsDescriber)

	res, err := handler.Handle(ctx, getme.Query{Uuid: stored.Uuid})
	require.NoError(t, err)
	require.Equal(t, agencyID, res.Agency.ID)
	require.Equal(t, agency.UUID, res.Agency.UUID)
	require.Equal(t, agency.Name, res.Agency.Name)
}
