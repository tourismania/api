package integration_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"api/internal/domain/airport"
	"api/internal/infrastructure/persistence/postgres"
	"api/internal/infrastructure/persistence/postgres/db"
	pgrepo "api/internal/infrastructure/persistence/postgres/repository"

	"github.com/jackc/pgx/v5/pgxpool"
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

// newAirportRepoOn создаёт репозиторий аэропортов поверх пула.
func newAirportRepoOn(pool *pgxpool.Pool) *pgrepo.AirportRepository {
	return pgrepo.NewAirportRepository(db.New(pool), pool)
}

// newSeededAirportRepo открывает пул, засевает справочник тестовыми
// аэропортами (см. seedAirportFixture) и возвращает репозиторий и сид.
func newSeededAirportRepo(t *testing.T) (*pgrepo.AirportRepository, airportFixture) {
	t.Helper()
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, testDB(t))
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return newAirportRepoOn(pool), seedAirportFixture(t, pool)
}

func searchAirports(t *testing.T, repo *pgrepo.AirportRepository, search string, limit, offset int) airport.SearchResult {
	t.Helper()
	res, err := repo.Search(context.Background(), airport.Filter{Search: search, Limit: limit, Offset: offset})
	require.NoError(t, err)
	return res
}

func icaosOf(airports []airport.Airport) []string {
	out := make([]string, 0, len(airports))
	for _, a := range airports {
		out = append(out, a.ICAO)
	}
	return out
}

func TestAirportRepository_Search_ByCity_ReturnsAllCityAirportsByName(t *testing.T) {
	repo, f := newSeededAirportRepo(t)

	res := searchAirports(t, repo, f.CityName, 10, 0)

	assert.Equal(t, int64(4), res.TotalCount)
	// Все совпали по началу названия города (один ранг) — порядок по названию аэропорта.
	assert.Equal(t, []string{f.Aaron.ICAO, f.Alpha.ICAO, f.Bravo.ICAO, f.Delta.ICAO}, icaosOf(res.Airports))
	assert.Equal(t, f.CityName, res.Airports[0].City.Name)
}

func TestAirportRepository_Search_ByName_CaseInsensitive(t *testing.T) {
	repo, f := newSeededAirportRepo(t)

	res := searchAirports(t, repo, strings.ToUpper("bravo "+f.Token), 10, 0)

	assert.Equal(t, []string{f.Bravo.ICAO}, icaosOf(res.Airports))
}

func TestAirportRepository_Search_PrefixRanksAboveSubstring(t *testing.T) {
	repo, f := newSeededAirportRepo(t)

	// «Alpha <token>» — начало названия Alpha и середина названия Aaron;
	// по алфавиту Aaron раньше, но совпадение по началу ранжируется выше.
	res := searchAirports(t, repo, "Alpha "+f.Token, 10, 0)

	assert.Equal(t, []string{f.Alpha.ICAO, f.Aaron.ICAO}, icaosOf(res.Airports))
}

func TestAirportRepository_Search_ByIATA_ExactFirst(t *testing.T) {
	repo, f := newSeededAirportRepo(t)

	// IATA Alpha совпадает точно, а у Delta тот же код есть в названии.
	res := searchAirports(t, repo, f.Alpha.IATA, 10, 0)

	require.NotEmpty(t, res.Airports)
	require.NotNil(t, res.Airports[0].IATA)
	assert.Equal(t, f.Alpha.IATA, *res.Airports[0].IATA)
	assert.Contains(t, icaosOf(res.Airports), f.Delta.ICAO, "совпадение по названию тоже в выдаче, но ниже")
}

func TestAirportRepository_Search_ByICAO_ExactFirst(t *testing.T) {
	repo, f := newSeededAirportRepo(t)

	res := searchAirports(t, repo, f.Bravo.ICAO, 10, 0)

	require.NotEmpty(t, res.Airports)
	assert.Equal(t, f.Bravo.ICAO, res.Airports[0].ICAO)
}

func TestAirportRepository_Search_Pagination(t *testing.T) {
	repo, f := newSeededAirportRepo(t)

	res := searchAirports(t, repo, f.CityName, 2, 2)

	assert.Equal(t, int64(4), res.TotalCount, "total считается без учёта пагинации")
	assert.Equal(t, []string{f.Bravo.ICAO, f.Delta.ICAO}, icaosOf(res.Airports))
}

func TestAirportRepository_Search_Unaccent(t *testing.T) {
	repo, f := newSeededAirportRepo(t)

	// Ищем без «ü» — находим «Zürich-…» по городу и названию аэропорта.
	res := searchAirports(t, repo, "Zurich-"+f.Token, 10, 0)

	assert.Equal(t, []string{f.Zurich.ICAO}, icaosOf(res.Airports))
	assert.Equal(t, f.ZurichCity, res.Airports[0].City.Name)
}

func TestAirportRepository_Search_NoMatch_ReturnsEmpty(t *testing.T) {
	repo, f := newSeededAirportRepo(t)

	res := searchAirports(t, repo, "nothing-"+f.Token, 10, 0)

	assert.Empty(t, res.Airports)
	assert.Equal(t, int64(0), res.TotalCount)
}
