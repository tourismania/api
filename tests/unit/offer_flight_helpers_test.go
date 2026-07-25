package unit_test

import (
	"context"

	"api/internal/domain/entity"
	"api/internal/domain/repository"
	"api/internal/domain/service"
)

// noopTxManager is a hand-written test double for txmanager.TxManager
// that runs fn directly against the given context — unit tests never
// need a real database transaction, only the same call shape the
// command handlers depend on.
type noopTxManager struct{}

func (noopTxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

// mockOfferFlightRepo is a hand-written test double for
// repository.OfferFlightRepository.
type mockOfferFlightRepo struct {
	findByOfferIDFlights []entity.Flight
	findByOfferIDErr     error

	replaceErr      error
	replaceCalled   bool
	replacedOfferID int
	replacedFlights []entity.Flight
}

func (m *mockOfferFlightRepo) FindByOfferID(_ context.Context, _ int) ([]entity.Flight, error) {
	return m.findByOfferIDFlights, m.findByOfferIDErr
}

func (m *mockOfferFlightRepo) ReplaceForOffer(_ context.Context, offerID int, flights []entity.Flight) error {
	m.replaceCalled = true
	m.replacedOfferID = offerID
	m.replacedFlights = flights
	return m.replaceErr
}

// mockAirportRepo is a hand-written test double for
// repository.AirportRepository. FindByICAOs defaults to reporting every
// requested icao as existing unless findByICAOsAirports/findByICAOsErr
// is set, so tests that don't care about airport existence don't need
// to configure it.
type mockAirportRepo struct {
	findByICAOsAirports []entity.Airport
	findByICAOsErr      error
}

func (m *mockAirportRepo) Search(_ context.Context, _ repository.AirportFilter) (repository.AirportSearchResult, error) {
	return repository.AirportSearchResult{}, nil
}

func (m *mockAirportRepo) Upsert(_ context.Context, _ string, _ *string, _ string, _, _ float64, _ *int, _ int) error {
	return nil
}

func (m *mockAirportRepo) FindByICAOs(_ context.Context, icaos []string) ([]entity.Airport, error) {
	if m.findByICAOsErr != nil {
		return nil, m.findByICAOsErr
	}
	if m.findByICAOsAirports != nil {
		return m.findByICAOsAirports, nil
	}
	airports := make([]entity.Airport, 0, len(icaos))
	for _, icao := range icaos {
		airports = append(airports, entity.Airport{ICAO: icao})
	}
	return airports, nil
}

// noFlightManager wires an OfferFlightManager over empty/permissive
// mocks — every icao "exists", nothing was stored before.
func noFlightManager() *service.OfferFlightManager {
	return service.NewOfferFlightManager(&mockOfferFlightRepo{}, &mockAirportRepo{})
}

// stubFlightFinder is a hand-written test double shared by get_offer's
// and get_published_offer's structurally identical FlightFinder ports.
type stubFlightFinder struct {
	flights []entity.Flight
	err     error
}

func (s stubFlightFinder) FindByOfferID(_ context.Context, _ int) ([]entity.Flight, error) {
	return s.flights, s.err
}
