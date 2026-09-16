package unit_test

import (
	"api/internal/domain/airport"
	"api/internal/domain/offer"
	"context"
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
// offer.FlightRepository.
type mockOfferFlightRepo struct {
	findByOfferIDFlights []offer.Flight
	findByOfferIDErr     error

	replaceErr      error
	replaceCalled   bool
	replacedOfferID int
	replacedFlights []offer.Flight
}

func (m *mockOfferFlightRepo) FindByOfferID(_ context.Context, _ int) ([]offer.Flight, error) {
	return m.findByOfferIDFlights, m.findByOfferIDErr
}

func (m *mockOfferFlightRepo) ReplaceForOffer(_ context.Context, offerID int, flights []offer.Flight) error {
	m.replaceCalled = true
	m.replacedOfferID = offerID
	m.replacedFlights = flights
	return m.replaceErr
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

// noFlightManager wires an offer.FlightManager over empty/permissive
// mocks — every icao "exists", nothing was stored before.
func noFlightManager() *offer.FlightManager {
	return offer.NewFlightManager(&mockOfferFlightRepo{}, &mockAirportRepo{})
}

// stubFlightFinder is a hand-written test double shared by get_offer's
// and get_published_offer's structurally identical FlightFinder ports.
type stubFlightFinder struct {
	flights []offer.Flight
	err     error
}

func (s stubFlightFinder) FindByOfferID(_ context.Context, _ int) ([]offer.Flight, error) {
	return s.flights, s.err
}
