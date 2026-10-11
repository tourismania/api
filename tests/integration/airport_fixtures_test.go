package integration_test

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"

	pgrepo "api/internal/infrastructure/persistence/postgres/repository"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// Сиды справочника аэропортов для тестов поиска. Данные изолированы от
// реальных: коды состоят из цифр (настоящие IATA/ICAO — только буквы), а
// названия содержат уникальный для прогона токен. Поэтому тесты не зависят
// от того, загружен ли справочник командой `airports sync`, не трогают
// реальные записи и проверяют точное число результатов. После теста
// записи удаляются.

// fixtureCountryISO2 — тестовая страна (та же, что в seedTestAirport).
const fixtureCountryISO2 = "ZZ"

// fixtureTokenLen — длина уникального токена в названиях.
const fixtureTokenLen = 8

// codeAttempts — сколько раз пробовать подобрать свободный код.
const codeAttempts = 50

// fixtureAirport — один аэропорт сида.
type fixtureAirport struct {
	ICAO string
	IATA string
	Name string
}

// airportFixture — засеянные данные одного теста.
type airportFixture struct {
	// Token — уникальная для прогона строка в названиях городов и аэропортов.
	Token string
	// CityName — город с четырьмя аэропортами (Alpha, Aaron, Bravo, Delta).
	CityName string
	// Alpha — аэропорт, название которого начинается с «Alpha <token>».
	Alpha fixtureAirport
	// Aaron — содержит «Alpha <token>» не в начале названия и по алфавиту
	// стоит раньше Alpha: проверка, что совпадение по началу выше подстроки.
	Aaron fixtureAirport
	// Bravo — обычный аэропорт города.
	Bravo fixtureAirport
	// Delta — в названии есть IATA-код Alpha: проверка, что точный IATA
	// выше совпадения по названию.
	Delta fixtureAirport
	// ZurichCity, Zurich — город и аэропорт с «ü» для проверки unaccent.
	ZurichCity string
	Zurich     fixtureAirport
	// SpecialCity, Special — город и аэропорт «Test_Field <token> 50%» со
	// спецсимволами LIKE в названии: проверка, что `%` и `_` ищутся буквально.
	SpecialCity string
	Special     fixtureAirport
	// SpecialAaron — содержит название Special не в начале и по алфавиту
	// стоит раньше: проверка, что Special попадает в группу «начинается со строки».
	SpecialAaron fixtureAirport
}

// seedAirportFixture засевает тестовые города и аэропорты и регистрирует
// их удаление после теста.
func seedAirportFixture(t *testing.T, pool *pgxpool.Pool) airportFixture {
	t.Helper()
	ctx := context.Background()

	token := randomLetters(fixtureTokenLen)
	codes := freeCodes(t, pool, 7)

	f := airportFixture{
		Token:       token,
		CityName:    "Тестоград-" + token,
		ZurichCity:  "Zürich-" + token,
		SpecialCity: "Спецград-" + token,
	}
	f.Alpha = fixtureAirport{ICAO: codes[0].icao, IATA: codes[0].iata, Name: "Alpha " + token + " International"}
	f.Aaron = fixtureAirport{ICAO: codes[1].icao, IATA: codes[1].iata, Name: "Aaron Alpha " + token + " Field"}
	f.Bravo = fixtureAirport{ICAO: codes[2].icao, IATA: codes[2].iata, Name: "Bravo " + token + " Field"}
	f.Delta = fixtureAirport{ICAO: codes[3].icao, IATA: codes[3].iata, Name: "Delta " + codes[0].iata + " " + token}
	f.Zurich = fixtureAirport{ICAO: codes[4].icao, IATA: codes[4].iata, Name: "Zürich-" + token + " Flughafen"}
	f.Special = fixtureAirport{ICAO: codes[5].icao, IATA: codes[5].iata, Name: "Test_Field " + token + " 50%"}
	f.SpecialAaron = fixtureAirport{ICAO: codes[6].icao, IATA: codes[6].iata, Name: "Aaron Test_Field " + token + " 50%"}

	countryRepo := pgrepo.NewCountryRepository(pool)
	require.NoError(t, countryRepo.Upsert(ctx, fixtureCountryISO2, "Test Country"))

	cityRepo := pgrepo.NewCityRepository(pool)
	cityID, err := cityRepo.Upsert(ctx, f.CityName, nil, "UTC", fixtureCountryISO2)
	require.NoError(t, err)
	zurichCityID, err := cityRepo.Upsert(ctx, f.ZurichCity, nil, "Europe/Zurich", fixtureCountryISO2)
	require.NoError(t, err)
	specialCityID, err := cityRepo.Upsert(ctx, f.SpecialCity, nil, "UTC", fixtureCountryISO2)
	require.NoError(t, err)

	airportRepo := newAirportRepoOn(pool)
	byCity := map[int][]fixtureAirport{
		cityID:        {f.Alpha, f.Aaron, f.Bravo, f.Delta},
		zurichCityID:  {f.Zurich},
		specialCityID: {f.Special, f.SpecialAaron},
	}
	var icaos []string
	for city, airports := range byCity {
		for _, a := range airports {
			iata := a.IATA
			require.NoError(t, airportRepo.Upsert(ctx, a.ICAO, &iata, a.Name, 0, 0, nil, city))
			icaos = append(icaos, a.ICAO)
		}
	}

	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM airports WHERE icao = ANY($1::char(4)[])`, icaos)
		_, _ = pool.Exec(ctx, `DELETE FROM cities WHERE id = ANY($1::int[])`, []int{cityID, zurichCityID, specialCityID})
	})
	return f
}

// fixtureCode — пара свободных кодов ICAO/IATA.
type fixtureCode struct {
	icao string
	iata string
}

// freeCodes подбирает n различных пар кодов из цифр, которых ещё нет в
// справочнике (на случай остатков от прерванного прогона).
func freeCodes(t *testing.T, pool *pgxpool.Pool, n int) []fixtureCode {
	t.Helper()
	ctx := context.Background()
	seen := make(map[string]struct{})
	codes := make([]fixtureCode, 0, n)
	for attempt := 0; len(codes) < n && attempt < codeAttempts*n; attempt++ {
		num := rand.IntN(1000)
		c := fixtureCode{icao: fmt.Sprintf("9%03d", num), iata: fmt.Sprintf("%03d", num)}
		if _, dup := seen[c.icao]; dup {
			continue
		}
		var exists bool
		err := pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM airports WHERE icao = $1 OR iata = $2)`, c.icao, c.iata,
		).Scan(&exists)
		require.NoError(t, err)
		if exists {
			continue
		}
		seen[c.icao] = struct{}{}
		codes = append(codes, c)
	}
	require.Len(t, codes, n, "не удалось подобрать свободные коды аэропортов")
	return codes
}

// randomLetters — строка из n строчных латинских букв.
func randomLetters(n int) string {
	var b strings.Builder
	for range n {
		b.WriteByte(byte('a' + rand.IntN(26)))
	}
	return b.String()
}
