package unit_test

import (
	"api/internal/domain/airline"
	"api/internal/domain/airport"
	"api/internal/domain/money"
	"api/internal/domain/offer/flight"
	"context"
	"fmt"
	"time"
)

// mustMoney разбирает сумму для тестов и паникует на некорректном вводе —
// в тестах это всегда ошибка самого теста.
func mustMoney(amount, code string) money.Money {
	m, err := money.Parse(amount, code)
	if err != nil {
		panic(err)
	}
	return m
}

// samplePrice — валидная цена по умолчанию для тестовых перелётов.
func samplePrice() money.Money {
	return mustMoney("15000", money.CodeRUB)
}

// noopTxManager is a hand-written test double for txmanager.TxManager
// that runs fn directly against the given context — unit tests never
// need a real database transaction, only the same call shape the
// command handlers depend on.
type noopTxManager struct{}

func (noopTxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

// mockOfferFlightRepo is a hand-written test double for
// flight.Repository.
type mockOfferFlightRepo struct {
	findByOfferIDFlights []flight.Flight
	findByOfferIDErr     error

	replaceErr      error
	replaceCalled   bool
	replacedOfferID int
	replacedFlights []flight.Flight

	updateDetailsErr     error
	updateDetailsCalled  bool
	updateDetailsOfferID int
	updatedDetails       []flight.DetailsUpdate
}

func (m *mockOfferFlightRepo) FindByOfferID(_ context.Context, _ int) ([]flight.Flight, error) {
	return m.findByOfferIDFlights, m.findByOfferIDErr
}

func (m *mockOfferFlightRepo) ReplaceForOffer(_ context.Context, offerID int, flights []flight.Flight) error {
	m.replaceCalled = true
	m.replacedOfferID = offerID
	m.replacedFlights = flights
	return m.replaceErr
}

func (m *mockOfferFlightRepo) UpdateDetails(_ context.Context, offerID int, updates []flight.DetailsUpdate) error {
	m.updateDetailsCalled = true
	m.updateDetailsOfferID = offerID
	m.updatedDetails = updates
	return m.updateDetailsErr
}

// mockAirportRepo is a hand-written test double for
// airport.Repository. FindByICAOs defaults to reporting every
// requested icao as existing unless findByICAOsAirports/findByICAOsErr
// is set, so tests that don't care about airport existence don't need
// to configure it.
type mockAirportRepo struct {
	findByICAOsAirports []airport.Airport
	findByICAOsErr      error
}

func (m *mockAirportRepo) Search(_ context.Context, _ airport.Filter) (airport.SearchResult, error) {
	return airport.SearchResult{}, nil
}

func (m *mockAirportRepo) Upsert(_ context.Context, _ string, _ *string, _ string, _, _ float64, _ *int, _ int) error {
	return nil
}

func (m *mockAirportRepo) FindByICAOs(_ context.Context, icaos []string) ([]airport.Airport, error) {
	if m.findByICAOsErr != nil {
		return nil, m.findByICAOsErr
	}
	if m.findByICAOsAirports != nil {
		return m.findByICAOsAirports, nil
	}
	airports := make([]airport.Airport, 0, len(icaos))
	for _, icao := range icaos {
		airports = append(airports, airport.Airport{ICAO: icao})
	}
	return airports, nil
}

// sampleAirlineID — id авиакомпании по умолчанию в тестовых сегментах;
// mockAirlineRepo без настройки считает его существующим.
const sampleAirlineID = 42

// mockAirlineRepo — ручной двойник airline.Repository. FindByIDs без
// настройки (airlines == nil) считает существующим каждый id ≥ 1 (не
// скрытым), чтобы тестам, которым авиакомпании не важны, ничего не
// настраивать; с airlines — возвращает только записи из неё. Запросы
// FindByIDs запоминаются.
type mockAirlineRepo struct {
	airlines     map[int]airline.Airline
	findByIDsErr error
	findCalls    [][]int
}

func (m *mockAirlineRepo) Search(_ context.Context, _ airline.Filter) (airline.SearchResult, error) {
	return airline.SearchResult{}, nil
}

func (m *mockAirlineRepo) FindByIDs(_ context.Context, ids []int) ([]airline.Airline, error) {
	m.findCalls = append(m.findCalls, ids)
	if m.findByIDsErr != nil {
		return nil, m.findByIDsErr
	}
	out := make([]airline.Airline, 0, len(ids))
	for _, id := range ids {
		if m.airlines == nil {
			if id >= 1 {
				out = append(out, airline.Airline{ID: id, Name: fmt.Sprintf("Airline %d", id)})
			}
			continue
		}
		if a, ok := m.airlines[id]; ok {
			out = append(out, a)
		}
	}
	return out, nil
}

func (m *mockAirlineRepo) Insert(_ context.Context, _ string, _ airline.Airline) (int, error) {
	return 0, nil
}

func (m *mockAirlineRepo) Update(_ context.Context, _ int, _ airline.Airline) error { return nil }

func (m *mockAirlineRepo) ListSourceIDs(_ context.Context) (map[string][]airline.SourceRecord, error) {
	return nil, nil
}

func (m *mockAirlineRepo) MarkMerged(_ context.Context, _, _ int) error { return nil }

func (m *mockAirlineRepo) MarkMissing(_ context.Context, _ []int, _ time.Time) error { return nil }

func (m *mockAirlineRepo) ClearMissing(_ context.Context, _ []int) error { return nil }

// withFlightNumbers возвращает копию перелёта с другими номерами рейсов
// сегментов (по порядку) и ID; сегменты копируются, исходный перелёт не
// меняется.
func withFlightNumbers(f flight.Flight, id int, numbers ...string) flight.Flight {
	f.ID = id
	f.Segments = append([]flight.Segment(nil), f.Segments...)
	for i, n := range numbers {
		f.Segments[i].FlightNumber = n
	}
	return f
}

// noFlightManager wires an flight.Manager over empty/permissive
// mocks — every icao and airline "exists", nothing was stored before.
func noFlightManager() *flight.Manager {
	return flight.NewManager(&mockOfferFlightRepo{}, &mockAirportRepo{}, &mockAirlineRepo{})
}

// stubFlightFinder is a hand-written test double shared by get_offer's
// and get_published_offer's structurally identical FlightFinder ports.
type stubFlightFinder struct {
	flights []flight.Flight
	err     error
}

func (s stubFlightFinder) FindByOfferID(_ context.Context, _ int) ([]flight.Flight, error) {
	return s.flights, s.err
}
