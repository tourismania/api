package integration_test

import (
	"context"
	"testing"
	"time"

	"api/internal/domain/entity"
	"api/internal/domain/enum"
	domainrepo "api/internal/domain/repository"
	"api/internal/infrastructure/persistence/postgres"
	"api/internal/infrastructure/persistence/postgres/db"
	pgrepo "api/internal/infrastructure/persistence/postgres/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newOfferRepo(t *testing.T) (*pgrepo.OfferRepository, *pgrepo.AgencyRepository, *pgrepo.UserRepository) {
	t.Helper()
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, testDB(t))
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	queries := db.New(pool)
	return pgrepo.NewOfferRepository(queries), pgrepo.NewAgencyRepository(queries), pgrepo.NewUserRepository(queries)
}

// seedAgencyAndUser creates a fresh active agency + user under it, so each
// test operates on isolated, unambiguous fixtures.
func seedAgencyAndUser(t *testing.T, agencyRepo *pgrepo.AgencyRepository, userRepo *pgrepo.UserRepository) (agencyID, userID int) {
	t.Helper()
	ctx := context.Background()

	agencyID, err := agencyRepo.Store(ctx, entity.Agency{
		UUID:      uuid.New(),
		Name:      "Offer Test Agency " + uuid.NewString(),
		Status:    enum.AgencyStatusActive,
		CreatedAt: time.Now().Truncate(time.Second),
	})
	require.NoError(t, err)

	idPtr, err := userRepo.Store(ctx, entity.User{
		FirstName: "Offer",
		LastName:  "Tester",
		Email:     "offer+" + uuid.NewString() + "@example.com",
		AgencyID:  agencyID,
	}, "hashed:secret")
	require.NoError(t, err)
	require.NotNil(t, idPtr)

	return agencyID, *idPtr
}

func newTestOffer(agencyID, createdBy int, status enum.OfferStatus) entity.Offer {
	now := time.Now().Truncate(time.Second)
	return entity.Offer{
		UUID:        uuid.New(),
		Title:       "Test offer " + uuid.NewString(),
		Description: "A short description",
		AgencyID:    agencyID,
		CreatedBy:   createdBy,
		Status:      status,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func TestOfferRepository_StoreThenFindByUUID_ReturnsStoredOffer(t *testing.T) {
	offerRepo, agencyRepo, userRepo := newOfferRepo(t)
	agencyID, userID := seedAgencyAndUser(t, agencyRepo, userRepo)
	ctx := context.Background()

	offer := newTestOffer(agencyID, userID, enum.OfferStatusDraft)
	id, err := offerRepo.Store(ctx, offer)
	require.NoError(t, err)
	require.Greater(t, id, 0)

	found, err := offerRepo.FindByUUID(ctx, offer.UUID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, offer.Title, found.Title)
	assert.Equal(t, offer.Description, found.Description)
	assert.Equal(t, agencyID, found.AgencyID)
	assert.Equal(t, userID, found.CreatedBy)
	assert.Equal(t, enum.OfferStatusDraft, found.Status)
	assert.Nil(t, found.DeletedAt)
}

func TestOfferRepository_FindByUUID_ReturnsNilForUnknownUUID(t *testing.T) {
	offerRepo, _, _ := newOfferRepo(t)

	found, err := offerRepo.FindByUUID(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestOfferRepository_Update_PersistsChanges(t *testing.T) {
	offerRepo, agencyRepo, userRepo := newOfferRepo(t)
	agencyID, userID := seedAgencyAndUser(t, agencyRepo, userRepo)
	ctx := context.Background()

	offer := newTestOffer(agencyID, userID, enum.OfferStatusDraft)
	_, err := offerRepo.Store(ctx, offer)
	require.NoError(t, err)

	offer.Title = "Updated title"
	offer.Status = enum.OfferStatusPublished
	offer.UpdatedAt = time.Now().Truncate(time.Second)
	require.NoError(t, offerRepo.Update(ctx, offer))

	found, err := offerRepo.FindByUUID(ctx, offer.UUID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "Updated title", found.Title)
	assert.Equal(t, enum.OfferStatusPublished, found.Status)
}

func TestOfferRepository_SoftDelete_ExcludesFromFindByUUID(t *testing.T) {
	offerRepo, agencyRepo, userRepo := newOfferRepo(t)
	agencyID, userID := seedAgencyAndUser(t, agencyRepo, userRepo)
	ctx := context.Background()

	offer := newTestOffer(agencyID, userID, enum.OfferStatusDraft)
	_, err := offerRepo.Store(ctx, offer)
	require.NoError(t, err)

	require.NoError(t, offerRepo.SoftDelete(ctx, offer.UUID))

	found, err := offerRepo.FindByUUID(ctx, offer.UUID)
	require.NoError(t, err)
	assert.Nil(t, found, "soft-deleted offer must not be returned by reads")
}

func TestOfferRepository_List_FiltersByAgencyAndStatus(t *testing.T) {
	offerRepo, agencyRepo, userRepo := newOfferRepo(t)
	agencyID, userID := seedAgencyAndUser(t, agencyRepo, userRepo)
	otherAgencyID, otherUserID := seedAgencyAndUser(t, agencyRepo, userRepo)
	ctx := context.Background()

	published := newTestOffer(agencyID, userID, enum.OfferStatusPublished)
	draft := newTestOffer(agencyID, userID, enum.OfferStatusDraft)
	otherAgencyOffer := newTestOffer(otherAgencyID, otherUserID, enum.OfferStatusPublished)

	for _, o := range []entity.Offer{published, draft, otherAgencyOffer} {
		_, err := offerRepo.Store(ctx, o)
		require.NoError(t, err)
	}

	status := enum.OfferStatusPublished
	res, err := offerRepo.List(ctx, domainrepo.OfferFilter{
		AgencyID: &agencyID,
		Status:   &status,
		Limit:    10,
		Offset:   0,
	})
	require.NoError(t, err)
	require.Len(t, res.Offers, 1)
	assert.Equal(t, published.UUID, res.Offers[0].UUID)
	assert.EqualValues(t, 1, res.TotalCount)
}

func TestOfferRepository_List_ExcludesSoftDeleted(t *testing.T) {
	offerRepo, agencyRepo, userRepo := newOfferRepo(t)
	agencyID, userID := seedAgencyAndUser(t, agencyRepo, userRepo)
	ctx := context.Background()

	kept := newTestOffer(agencyID, userID, enum.OfferStatusDraft)
	deleted := newTestOffer(agencyID, userID, enum.OfferStatusDraft)

	_, err := offerRepo.Store(ctx, kept)
	require.NoError(t, err)
	_, err = offerRepo.Store(ctx, deleted)
	require.NoError(t, err)
	require.NoError(t, offerRepo.SoftDelete(ctx, deleted.UUID))

	res, err := offerRepo.List(ctx, domainrepo.OfferFilter{
		AgencyID: &agencyID,
		Limit:    10,
		Offset:   0,
	})
	require.NoError(t, err)
	require.Len(t, res.Offers, 1)
	assert.Equal(t, kept.UUID, res.Offers[0].UUID)
}

func TestOfferRepository_List_Pagination_LimitsAndOffsets(t *testing.T) {
	offerRepo, agencyRepo, userRepo := newOfferRepo(t)
	agencyID, userID := seedAgencyAndUser(t, agencyRepo, userRepo)
	ctx := context.Background()

	const total = 3
	for i := 0; i < total; i++ {
		_, err := offerRepo.Store(ctx, newTestOffer(agencyID, userID, enum.OfferStatusDraft))
		require.NoError(t, err)
	}

	page1, err := offerRepo.List(ctx, domainrepo.OfferFilter{AgencyID: &agencyID, Limit: 2, Offset: 0})
	require.NoError(t, err)
	assert.Len(t, page1.Offers, 2)
	assert.EqualValues(t, total, page1.TotalCount)

	page2, err := offerRepo.List(ctx, domainrepo.OfferFilter{AgencyID: &agencyID, Limit: 2, Offset: 2})
	require.NoError(t, err)
	assert.Len(t, page2.Offers, 1)
	assert.EqualValues(t, total, page2.TotalCount)
}
