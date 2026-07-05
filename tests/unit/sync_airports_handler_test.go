package unit_test

import (
	"context"
	"testing"

	syncairports "api/internal/application/command/sync_airports"
	domainrepo "api/internal/domain/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAirportSource struct{ records []syncairports.AirportRecord }

func (f fakeAirportSource) Fetch(context.Context) ([]syncairports.AirportRecord, error) {
	return f.records, nil
}

type fakeTranslations struct {
	cityNamesRU    map[syncairports.CityRef]string
	receivedCities []syncairports.CityRef
}

func (f *fakeTranslations) FetchAirportNamesRU(context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}

func (f *fakeTranslations) FetchCityNamesRU(_ context.Context, cities []syncairports.CityRef) (map[syncairports.CityRef]string, error) {
	f.receivedCities = cities
	return f.cityNamesRU, nil
}

type fakeCountryNames struct{}

func (fakeCountryNames) NameRU(string) string { return "" }

type fakeCountryRepo struct{}

func (*fakeCountryRepo) Upsert(context.Context, string, string) error { return nil }

type cityUpsertCall struct {
	name, state, countryISO2 string
}

type fakeCityRepo struct {
	calls  []cityUpsertCall
	nextID int
}

func (f *fakeCityRepo) Upsert(_ context.Context, name string, state *string, _ string, countryISO2 string) (int, error) {
	s := ""
	if state != nil {
		s = *state
	}
	f.calls = append(f.calls, cityUpsertCall{name: name, state: s, countryISO2: countryISO2})
	f.nextID++
	return f.nextID, nil
}

type airportUpsertCall struct {
	icao   string
	cityID int
}

type fakeAirportRepo struct{ calls []airportUpsertCall }

func (*fakeAirportRepo) Search(context.Context, domainrepo.AirportFilter) (domainrepo.AirportSearchResult, error) {
	return domainrepo.AirportSearchResult{}, nil
}

func (f *fakeAirportRepo) Upsert(_ context.Context, icao string, _ *string, _ string, _, _ float64, _ *int, cityID int) error {
	f.calls = append(f.calls, airportUpsertCall{icao: icao, cityID: cityID})
	return nil
}

// TestHandler_SharedCityGroup_UsesOwnCityTranslationNotAnotherAirports is a
// regression test for the bug where the Domodedovo/Sheremetyevo group of
// Moscow-Oblast airports ended up saved with the city "Быково" (Bykovo) —
// the translation for a *different* airport in the same dedup group that
// happened to sort first alphabetically by ICAO. Translations must be
// looked up by the city's own name, never borrowed from another airport.
func TestHandler_SharedCityGroup_UsesOwnCityTranslationNotAnotherAirports(t *testing.T) {
	records := []syncairports.AirportRecord{
		{ICAO: "UUBB", Name: "Bykovo Airport", City: "Bykovo", State: "Moscow-Oblast", CountryISO2: "RU"},
		{ICAO: "UUDD", Name: "Domodedovo International Airport", City: "Moscow", State: "Moscow-Oblast", CountryISO2: "RU"},
		{ICAO: "UUEE", Name: "Sheremetyevo International Airport", City: "Moscow", State: "Moscow-Oblast", CountryISO2: "RU"},
	}
	translations := &fakeTranslations{
		cityNamesRU: map[syncairports.CityRef]string{
			{Name: "Moscow", CountryISO2: "RU"}: "Москва",
			{Name: "Bykovo", CountryISO2: "RU"}: "Быково",
		},
	}
	cityRepo := &fakeCityRepo{}
	airportRepo := &fakeAirportRepo{}

	h := syncairports.NewHandler(airportRepo, &fakeCountryRepo{}, cityRepo, fakeAirportSource{records: records}, translations, fakeCountryNames{})

	res, err := h.Handle(context.Background(), syncairports.Command{})
	require.NoError(t, err)

	// Only two distinct cities should have been upserted: Москва (shared by
	// UUDD/UUEE) and Быково (UUBB's own city) — never a third, mixed-up one.
	assert.Equal(t, 2, res.Cities)
	require.Len(t, cityRepo.calls, 2)

	cityIDByName := map[string]int{}
	for i, c := range cityRepo.calls {
		cityIDByName[c.name] = i + 1
	}
	require.Contains(t, cityIDByName, "Москва")
	require.Contains(t, cityIDByName, "Быково")
	assert.NotContains(t, cityIDByName, "Moscow", "city name should be translated, not left in English")

	moscowID := cityIDByName["Москва"]
	bykovoID := cityIDByName["Быково"]

	for _, a := range airportRepo.calls {
		switch a.icao {
		case "UUDD", "UUEE":
			assert.Equal(t, moscowID, a.cityID, "airport %s must be linked to the Moscow city row, not Bykovo's", a.icao)
		case "UUBB":
			assert.Equal(t, bykovoID, a.cityID, "UUBB must keep its own Bykovo city row")
		}
	}

	// Translations must be requested once per distinct city, not once per airport.
	assert.Len(t, translations.receivedCities, 2)
}
