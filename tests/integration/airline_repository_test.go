package integration_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"api/internal/domain/airline"
	"api/internal/domain/offer/flight"
	"api/internal/infrastructure/persistence/postgres"
	"api/internal/infrastructure/persistence/postgres/db"
	pgrepo "api/internal/infrastructure/persistence/postgres/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Сиды справочника авиакомпаний изолированы от реальных данных: названия и
// source_id содержат уникальный для прогона токен, страна — тестовая
// airlineFixtureCountryISO2. Поэтому тесты не зависят от того, выполнялась
// ли `airlines sync`, и проверяют точный состав выдачи. После теста записи
// удаляются.

// airlineFixtureCountryISO2 — тестовая страна авиакомпаний (не ZZ аэропортов:
// название страны здесь своё, с токеном).
const airlineFixtureCountryISO2 = "ZY"

// airlineFixture — засеянные авиакомпании одного теста (id записей).
type airlineFixture struct {
	Token string
	// CountryName — название тестовой страны: «Ctry<token>».
	CountryName string
	// Alpha — действующая «Alpha <token> Air», код ZQA, два IATA, страна ZY.
	Alpha int
	// Aaron — «Aaron Alpha <token>»: «alpha <token>» не в начале, по алфавиту
	// раньше Alpha.
	Aaron int
	// OldAlpha — закрывшаяся «Alpha <token> Old» с тем же кодом ZQA, страна ZY.
	OldAlpha int
	// Rus — «Rusline <token>» с русским названием «Руслайн <token>».
	Rus int
	// Zurich — «Zürich <token>» для проверки диакритики.
	Zurich int
	// Special — «Al_ha <token> 50%\» со спецсимволами LIKE.
	Special int
}

func newAirlineTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := postgres.NewPool(context.Background(), testDB(t))
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

// newAirlineToken — уникальный токен из латинских букв (без цифр, чтобы
// подстрока не совпала с кодом).
func newAirlineToken() string {
	hex := strings.ReplaceAll(uuid.NewString(), "-", "")[:10]
	var b strings.Builder
	for _, r := range hex {
		if r >= '0' && r <= '9' {
			r = 'g' + (r - '0')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// seedAirlines засевает сид и регистрирует очистку записей с source_id
// «test-<token>-…».
func seedAirlines(t *testing.T, pool *pgxpool.Pool, repo *pgrepo.AirlineRepository) airlineFixture {
	t.Helper()
	ctx := context.Background()
	token := newAirlineToken()
	f := airlineFixture{Token: token, CountryName: "Ctry" + token}

	require.NoError(t, pgrepo.NewCountryRepository(pool).Upsert(ctx, airlineFixtureCountryISO2, f.CountryName))
	t.Cleanup(func() {
		_, err := pool.Exec(context.Background(), `DELETE FROM airlines WHERE source_id LIKE $1`, "test-"+token+"-%")
		assert.NoError(t, err)
	})

	country := &airline.Country{ISO2: airlineFixtureCountryISO2}
	dissolved := time.Date(2015, 10, 26, 0, 0, 0, 0, time.UTC)
	f.Alpha = insertAirline(t, repo, token, "alpha", airline.Airline{ICAO: strPtr("ZQA"), Name: "Alpha " + token + " Air", Country: country, IATACodes: []string{"Z1", "Z2"}})
	f.Aaron = insertAirline(t, repo, token, "aaron", airline.Airline{ICAO: strPtr("ZQB"), Name: "Aaron Alpha " + token})
	f.OldAlpha = insertAirline(t, repo, token, "old-alpha", airline.Airline{ICAO: strPtr("ZQA"), Name: "Alpha " + token + " Old", Country: country, DissolvedOn: &dissolved})
	f.Rus = insertAirline(t, repo, token, "rus", airline.Airline{ICAO: strPtr("ZQR"), Name: "Rusline " + token, NameRU: "Руслайн " + token})
	f.Zurich = insertAirline(t, repo, token, "zurich", airline.Airline{ICAO: strPtr("ZQZ"), Name: "Zürich " + token})
	f.Special = insertAirline(t, repo, token, "special", airline.Airline{ICAO: strPtr("ZQS"), Name: "Al_ha " + token + ` 50%\`})
	return f
}

func insertAirline(t *testing.T, repo *pgrepo.AirlineRepository, token, key string, a airline.Airline) int {
	t.Helper()
	id, err := repo.Insert(context.Background(), "test-"+token+"-"+key, a)
	require.NoError(t, err)
	return id
}

func strPtr(s string) *string { return &s }

func newSeededAirlineRepo(t *testing.T) (*pgrepo.AirlineRepository, airlineFixture, *pgxpool.Pool) {
	t.Helper()
	pool := newAirlineTestPool(t)
	repo := pgrepo.NewAirlineRepository(db.New(pool))
	return repo, seedAirlines(t, pool, repo), pool
}

// searchAirlineIDs — id найденных записей в порядке выдачи.
func searchAirlineIDs(t *testing.T, repo *pgrepo.AirlineRepository, f airline.Filter) ([]int, int64) {
	t.Helper()
	if f.Limit == 0 {
		f.Limit = 100
	}
	res, err := repo.Search(context.Background(), f)
	require.NoError(t, err)
	ids := make([]int, 0, len(res.Airlines))
	for _, a := range res.Airlines {
		ids = append(ids, a.ID)
	}
	return ids, res.TotalCount
}

func findAirline(t *testing.T, repo *pgrepo.AirlineRepository, id int) airline.Airline {
	t.Helper()
	found, err := repo.FindByIDs(context.Background(), []int{id})
	require.NoError(t, err)
	require.Len(t, found, 1)
	return found[0]
}

func TestAirlineRepository_InsertThenUpdate_KeepsIDAndChangesData(t *testing.T) {
	repo, f, _ := newSeededAirlineRepo(t)

	a := findAirline(t, repo, f.Alpha)
	assert.Equal(t, "ZQA", *a.ICAO)
	assert.Equal(t, "", a.NameRU, "name_ru по умолчанию — пустая строка")
	assert.Equal(t, []string{"Z1", "Z2"}, a.IATACodes, "оба кода IATA сохраняются")
	require.NotNil(t, a.Country)
	assert.Equal(t, f.CountryName, a.Country.Name)

	dissolved := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, repo.Update(context.Background(), f.Alpha, airline.Airline{
		ICAO: strPtr("ZQX"), Name: "Alpha " + f.Token + " Renamed", NameRU: "Альфа", DissolvedOn: &dissolved,
	}))

	got := findAirline(t, repo, f.Alpha)
	assert.Equal(t, f.Alpha, got.ID)
	assert.Equal(t, "ZQX", *got.ICAO, "код ICAO меняется, id — нет")
	assert.Equal(t, "Alpha "+f.Token+" Renamed", got.Name)
	assert.Equal(t, "Альфа", got.NameRU)
	assert.Nil(t, got.Country)
	require.NotNil(t, got.DissolvedOn)
	assert.True(t, dissolved.Equal(*got.DissolvedOn))
	assert.Empty(t, got.IATACodes)
}

func TestAirlineRepository_SameICAOAndSourceID_RecordsCoexist(t *testing.T) {
	repo, f, _ := newSeededAirlineRepo(t)

	ids, _ := searchAirlineIDs(t, repo, airline.Filter{Search: "alpha " + f.Token})
	assert.Contains(t, ids, f.Alpha)
	assert.Contains(t, ids, f.OldAlpha, "запись с тем же ICAO находится отдельно")

	first := insertAirline(t, repo, f.Token, "same", airline.Airline{ICAO: strPtr("ZQQ"), Name: "Same " + f.Token})
	second := insertAirline(t, repo, f.Token, "same", airline.Airline{ICAO: strPtr("ZQQ"), Name: "Same " + f.Token})
	assert.NotEqual(t, first, second)

	sources, err := repo.ListSourceIDs(context.Background())
	require.NoError(t, err)
	assert.Len(t, sources["test-"+f.Token+"-same"], 2, "source_id не уникален в схеме")
}

func TestAirlineRepository_Search_ByNamesAndCountry_CaseAndDiacriticInsensitive(t *testing.T) {
	repo, f, _ := newSeededAirlineRepo(t)

	for name, tc := range map[string]struct {
		search string
		want   []int
	}{
		"английское название": {"RUSLINE " + f.Token, []int{f.Rus}},
		"русское название":    {"руслайн " + f.Token, []int{f.Rus}},
		"без диакритики":      {"zurich " + f.Token, []int{f.Zurich}},
		"с диакритикой":       {"ZÜRICH " + f.Token, []int{f.Zurich}},
		"название страны":     {strings.ToUpper(f.CountryName), []int{f.Alpha, f.OldAlpha}},
	} {
		t.Run(name, func(t *testing.T) {
			ids, _ := searchAirlineIDs(t, repo, airline.Filter{Search: tc.search})
			assert.Equal(t, tc.want, ids)
		})
	}
}

func TestAirlineRepository_Search_LikeWildcardsAreLiteral(t *testing.T) {
	repo, f, _ := newSeededAirlineRepo(t)

	ids, _ := searchAirlineIDs(t, repo, airline.Filter{Search: "al_ha " + f.Token})
	assert.Equal(t, []int{f.Special}, ids, "_ не подстановка: Alpha не находится")

	ids, _ = searchAirlineIDs(t, repo, airline.Filter{Search: f.Token + " 50%"})
	assert.Equal(t, []int{f.Special}, ids)

	ids, _ = searchAirlineIDs(t, repo, airline.Filter{Search: "50%\\"})
	assert.Contains(t, ids, f.Special)

	ids, _ = searchAirlineIDs(t, repo, airline.Filter{Search: "alpha%" + f.Token})
	assert.Empty(t, ids, "% не подстановка")
}

func TestAirlineRepository_Search_CodesNotSearched(t *testing.T) {
	repo, f, _ := newSeededAirlineRepo(t)

	ids, _ := searchAirlineIDs(t, repo, airline.Filter{Search: "ZQA", CountryISO2: airlineFixtureCountryISO2})
	assert.Empty(t, ids, "по ICAO не ищется")
	ids, _ = searchAirlineIDs(t, repo, airline.Filter{Search: "Z1", IDs: []int{f.Alpha}})
	assert.Empty(t, ids, "по IATA не ищется")
}

func TestAirlineRepository_Search_CountryFilter(t *testing.T) {
	repo, f, _ := newSeededAirlineRepo(t)

	ids, _ := searchAirlineIDs(t, repo, airline.Filter{Search: f.Token, CountryISO2: airlineFixtureCountryISO2})
	assert.Equal(t, []int{f.Alpha, f.OldAlpha}, ids)
}

func TestAirlineRepository_Search_RankingActiveFirstThenGroups(t *testing.T) {
	repo, f, _ := newSeededAirlineRepo(t)

	// Alpha — действующая, начало названия; Aaron — действующая, подстрока;
	// OldAlpha — начало названия, но закрывшаяся — ниже всех действующих.
	ids, _ := searchAirlineIDs(t, repo, airline.Filter{Search: "alpha " + f.Token})
	assert.Equal(t, []int{f.Alpha, f.Aaron, f.OldAlpha}, ids)

	// Совпадение по названию выше совпадения только по стране.
	countryOnly := insertAirline(t, repo, f.Token, "country-only", airline.Airline{
		ICAO: strPtr("ZQC"), Name: "Zeta " + f.Token, Country: &airline.Country{ISO2: airlineFixtureCountryISO2},
	})
	named := insertAirline(t, repo, f.Token, "named", airline.Airline{ICAO: strPtr("ZQD"), Name: "Omega " + f.CountryName})
	ids, _ = searchAirlineIDs(t, repo, airline.Filter{Search: f.CountryName})
	assert.Equal(t, []int{named, f.Alpha, countryOnly, f.OldAlpha}, ids)
}

func TestAirlineRepository_Search_TotalCountIgnoresPagination(t *testing.T) {
	repo, f, _ := newSeededAirlineRepo(t)

	ids, total := searchAirlineIDs(t, repo, airline.Filter{Search: "alpha " + f.Token, Limit: 1, Offset: 1})
	assert.Equal(t, []int{f.Aaron}, ids)
	assert.Equal(t, int64(3), total)
}

func TestAirlineRepository_Search_UnknownRecordFoundByBothNames(t *testing.T) {
	repo := pgrepo.NewAirlineRepository(db.New(newAirlineTestPool(t)))

	for _, search := range []string{"неизв", "unknown"} {
		ids, _ := searchAirlineIDs(t, repo, airline.Filter{Search: search})
		assert.Contains(t, ids, airline.UnknownID, search)
	}
}

func TestAirlineRepository_HiddenRecords_NotSearchedUntilBack(t *testing.T) {
	repo, f, _ := newSeededAirlineRepo(t)
	ctx := context.Background()
	dup := insertAirline(t, repo, f.Token, "dup", airline.Airline{ICAO: strPtr("ZQA"), Name: "Alpha " + f.Token + " Dup"})
	gone := insertAirline(t, repo, f.Token, "gone", airline.Airline{ICAO: strPtr("ZQG"), Name: "Alpha " + f.Token + " Gone"})

	require.NoError(t, repo.MarkMerged(ctx, dup, f.Alpha))
	require.NoError(t, repo.MarkMissing(ctx, []int{gone}, time.Now()))

	ids, _ := searchAirlineIDs(t, repo, airline.Filter{Search: "alpha " + f.Token})
	assert.NotContains(t, ids, dup, "дубль не ищется")
	assert.NotContains(t, ids, gone, "пропавшая не ищется")

	ids, _ = searchAirlineIDs(t, repo, airline.Filter{IDs: []int{f.Alpha, dup, gone}})
	assert.Equal(t, []int{f.Alpha}, ids, "по ids скрытые тоже не находятся")

	found, err := repo.FindByIDs(ctx, []int{dup, gone, f.Alpha, 999_999_999})
	require.NoError(t, err, "отсутствующий id не ошибка")
	hidden := map[int]bool{}
	for _, a := range found {
		hidden[a.ID] = a.Hidden
	}
	assert.Equal(t, map[int]bool{dup: true, gone: true, f.Alpha: false}, hidden)

	require.NoError(t, repo.ClearMissing(ctx, []int{gone}))
	ids, _ = searchAirlineIDs(t, repo, airline.Filter{Search: "alpha " + f.Token})
	assert.Contains(t, ids, gone, "вернувшаяся снова находится")
}

func TestAirlineRepository_Search_ByIDs(t *testing.T) {
	repo, f, _ := newSeededAirlineRepo(t)

	ids, total := searchAirlineIDs(t, repo, airline.Filter{IDs: []int{f.Rus, f.Alpha}})
	assert.Equal(t, []int{f.Alpha, f.Rus}, ids, "в порядке ранжирования")
	assert.Equal(t, int64(2), total)

	ids, _ = searchAirlineIDs(t, repo, airline.Filter{IDs: []int{999_999_999}})
	assert.Empty(t, ids)

	ids, _ = searchAirlineIDs(t, repo, airline.Filter{Search: "rusline", IDs: []int{f.Rus, f.Alpha}})
	assert.Equal(t, []int{f.Rus}, ids, "search и ids вместе")
}

func TestAirlineRepository_MarkMerged_ChainCollapsesAndSegmentsUntouched(t *testing.T) {
	repo, f, pool := newSeededAirlineRepo(t)
	ctx := context.Background()
	a := insertAirline(t, repo, f.Token, "chain-a", airline.Airline{ICAO: strPtr("ZQE"), Name: "Chain A " + f.Token})
	b := insertAirline(t, repo, f.Token, "chain-b", airline.Airline{ICAO: strPtr("ZQE"), Name: "Chain B " + f.Token})
	c := insertAirline(t, repo, f.Token, "chain-c", airline.Airline{ICAO: strPtr("ZQE"), Name: "Chain C " + f.Token})

	// Сегмент оффера ссылается на A.
	flights := pgrepo.NewOfferFlightRepository(db.New(pool))
	offerID := seedTestOfferID(t, pool)
	dep, arr := seedTestAirport(t, pool), seedTestAirport(t, pool)
	base := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, flights.ReplaceForOffer(ctx, offerID, []flight.Flight{{Price: testFlightPrice(t), Segments: []flight.Segment{
		{DepartureAirportICAO: dep, ArrivalAirportICAO: arr, DepartureAt: base, ArrivalAt: base.Add(time.Hour), AirlineID: a},
	}}}))
	t.Cleanup(func() { assert.NoError(t, flights.ReplaceForOffer(context.Background(), offerID, nil)) })

	require.NoError(t, repo.MarkMerged(ctx, a, b))
	require.NoError(t, repo.MarkMerged(ctx, b, c))

	assert.Equal(t, c, mergedInto(t, pool, a), "A, слитая в B, переходит на C")
	assert.Equal(t, c, mergedInto(t, pool, b))
	assert.Equal(t, 0, mergedInto(t, pool, c))

	stored, err := flights.FindByOfferID(ctx, offerID)
	require.NoError(t, err)
	require.Len(t, stored, 1)
	assert.Equal(t, a, stored[0].Segments[0].AirlineID, "слияние не меняет сегменты")
}

// mergedInto — merged_into_id записи (0 — не дубль).
func mergedInto(t *testing.T, pool *pgxpool.Pool, id int) int {
	t.Helper()
	var into *int32
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT merged_into_id FROM airlines WHERE id = $1`, id).Scan(&into))
	if into == nil {
		return 0
	}
	return int(*into)
}

func TestAirlineRepository_UnknownRecord_FixedAndUntouchedBySync(t *testing.T) {
	repo := pgrepo.NewAirlineRepository(db.New(newAirlineTestPool(t)))
	ctx := context.Background()

	// Update и пометки служебную запись не меняют (у неё нет source_id).
	require.NoError(t, repo.Update(ctx, airline.UnknownID, airline.Airline{Name: "Changed"}))
	require.NoError(t, repo.MarkMissing(ctx, []int{airline.UnknownID}, time.Now()))

	got := findAirline(t, repo, airline.UnknownID)
	assert.Nil(t, got.ICAO)
	assert.Equal(t, "Unknown", got.Name)
	assert.Equal(t, "Неизвестная", got.NameRU)
	assert.Nil(t, got.Country)
	assert.Nil(t, got.DissolvedOn)
	assert.False(t, got.Hidden)
}
