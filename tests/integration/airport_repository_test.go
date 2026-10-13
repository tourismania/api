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
	repo, f, _ := newSeededAirportRepoWithPool(t)
	return repo, f
}

// newSeededAirportRepoWithPool — как newSeededAirportRepo, но отдаёт и пул
// для проверочных запросов в обход репозитория.
func newSeededAirportRepoWithPool(t *testing.T) (*pgrepo.AirportRepository, airportFixture, *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, testDB(t))
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return newAirportRepoOn(pool), seedAirportFixture(t, pool), pool
}

// countLiteralNameMatches — сколько аэропортов содержат s буквально (без
// подстановочных символов) в названии аэропорта или города. Нужен, чтобы
// точно проверять выдачу, даже если в БД загружен реальный справочник.
func countLiteralNameMatches(t *testing.T, pool *pgxpool.Pool, s string) int64 {
	t.Helper()
	var n int64
	err := pool.QueryRow(context.Background(), `
SELECT count(*)
FROM airports a
JOIN cities c ON c.id = a.city_id
WHERE strpos(lower(unaccent(a.name)), lower(unaccent($1))) > 0
   OR strpos(lower(unaccent(c.name)), lower(unaccent($1))) > 0`, s).Scan(&n)
	require.NoError(t, err)
	return n
}

// fixtureICAOs — коды всех аэропортов сида.
func fixtureICAOs(f airportFixture) []string {
	return []string{
		f.Alpha.ICAO, f.Aaron.ICAO, f.Bravo.ICAO, f.Delta.ICAO,
		f.Zurich.ICAO, f.Special.ICAO, f.SpecialAaron.ICAO,
	}
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

func TestAirportRepository_Search_PercentOnly_NoWildcard(t *testing.T) {
	repo, f, pool := newSeededAirportRepoWithPool(t)

	// «%%» — не «все аэропорты»: ищется буквально, а в названиях сида такой
	// последовательности нет (в «… 50%» процент один).
	res := searchAirports(t, repo, "%%", 100, 0)

	assert.Equal(t, countLiteralNameMatches(t, pool, "%%"), res.TotalCount)
	for _, icao := range fixtureICAOs(f) {
		assert.NotContains(t, icaosOf(res.Airports), icao)
	}
}

func TestAirportRepository_Search_Underscore_NoWildcard(t *testing.T) {
	repo, f := newSeededAirportRepo(t)

	// «_» не заменяет «p» в «Alpha <token>».
	res := searchAirports(t, repo, "Al_ha "+f.Token, 10, 0)

	assert.Empty(t, res.Airports)
	assert.Equal(t, int64(0), res.TotalCount)
}

func TestAirportRepository_Search_Backslash_Literal(t *testing.T) {
	repo, f := newSeededAirportRepo(t)

	// «\» ищется как обычный символ, а не экранирует следующий.
	res := searchAirports(t, repo, `Al\pha `+f.Token, 10, 0)

	assert.Empty(t, res.Airports)
	assert.Equal(t, int64(0), res.TotalCount)
}

func TestAirportRepository_Search_SpecialCharsInName_PrefixRank(t *testing.T) {
	repo, f := newSeededAirportRepo(t)

	// Название Special начинается со строки поиска, SpecialAaron содержит её
	// в середине и по алфавиту раньше — Special должен быть первым.
	res := searchAirports(t, repo, "test_field "+f.Token+" 50%", 10, 0)

	assert.Equal(t, []string{f.Special.ICAO, f.SpecialAaron.ICAO}, icaosOf(res.Airports))
}
