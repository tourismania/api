package integration_test

import (
	"context"
	"os"
	"testing"

	domainrepo "api/internal/domain/repository"
	"api/internal/infrastructure/persistence/postgres"
	"api/internal/infrastructure/persistence/postgres/db"
	pgrepo "api/internal/infrastructure/persistence/postgres/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testDB returns a database URL from the environment.
// Integration tests require a running Postgres with migrations applied.
func testDB(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration tests")
	}
	return dsn
}

func newAirportRepo(t *testing.T) *pgrepo.AirportRepository {
	t.Helper()
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, testDB(t))
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pgrepo.NewAirportRepository(db.New(pool), pool)
}

func TestAirportRepository_Search_ByName(t *testing.T) {
	repo := newAirportRepo(t)
	res, err := repo.Search(context.Background(), domainrepo.AirportFilter{
		Search: "Moscow",
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(res.Airports), 1)
}

func TestAirportRepository_Search_ByIATA_ExactFirst(t *testing.T) {
	repo := newAirportRepo(t)
	res, err := repo.Search(context.Background(), domainrepo.AirportFilter{
		Search: "SVO",
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err)
	require.NotEmpty(t, res.Airports)
	assert.NotNil(t, res.Airports[0].IATA)
	assert.Equal(t, "SVO", *res.Airports[0].IATA)
}

func TestAirportRepository_Search_ByICAO_ExactFirst(t *testing.T) {
	repo := newAirportRepo(t)
	res, err := repo.Search(context.Background(), domainrepo.AirportFilter{
		Search: "UUEE",
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err)
	require.NotEmpty(t, res.Airports)
	assert.Equal(t, "UUEE", res.Airports[0].ICAO)
}

func TestAirportRepository_Search_Pagination(t *testing.T) {
	repo := newAirportRepo(t)
	res, err := repo.Search(context.Background(), domainrepo.AirportFilter{
		Search: "Moscow",
		Limit:  2,
		Offset: 2,
	})
	require.NoError(t, err)
	assert.LessOrEqual(t, len(res.Airports), 2)
	assert.GreaterOrEqual(t, res.TotalCount, int64(4))
}

func TestAirportRepository_Search_Unaccent(t *testing.T) {
	repo := newAirportRepo(t)
	res, err := repo.Search(context.Background(), domainrepo.AirportFilter{
		Search: "Zurich",
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err)
	require.NotEmpty(t, res.Airports, "expected to find Zürich Airport via unaccent search")

	found := false
	for _, a := range res.Airports {
		if a.ICAO == "LSZH" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected LSZH (Zürich) in results")
}
