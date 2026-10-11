package flight

import (
	"context"
	"errors"
	"fmt"

	"api/internal/domain/airport"
)

// ErrAirportNotFound is returned by Manager when a segment references
// an icao that does not exist in the airports reference table. Unlike
// the structural errors checked by New, this requires a database
// lookup, so it is raised here rather than by the factory.
var ErrAirportNotFound = errors.New("flight segment references an unknown airport icao")

// Manager orchestrates replacing the full set of flights attached to
// an Offer. It does not check role/agency ownership: it is only ever
// called from create_offer/update_offer handlers after
// offer.Manager.Insert/Update has already enforced that, within the
// same command and (for the write path) the same transaction.
type Manager struct {
	flights  Repository
	airports airport.Repository
}

// NewManager wires the collaborators.
func NewManager(flights Repository, airports airport.Repository) *Manager {
	return &Manager{flights: flights, airports: airports}
}

// ReplaceForOffer проверяет, что все коды icao из flights есть в
// справочнике аэропортов, и сравнивает flights с сохранёнными для offerID —
// сначала маршрут, потом цены и комментарии:
//   - маршрут совпал (тот же порядок перелётов, те же аэропорты и
//     моменты времени сегментов; ID, назначенные сервером, не учитываются),
//     цены и комментарии совпали — no-op, БД не трогается;
//   - маршрут совпал, у части перелётов другая цена, валюта или
//     комментарий — у этих перелётов обновляются только цена и комментарий,
//     сегменты и идентификаторы сохраняются;
//   - маршрут отличается — набор заменяется целиком.
//
// Перелёты сопоставляются по позиции: у входящих перелётов нет ID.
func (m *Manager) ReplaceForOffer(ctx context.Context, offerID int, flights []Flight) error {
	if err := m.checkAirportsExist(ctx, flights); err != nil {
		return err
	}

	existing, err := m.flights.FindByOfferID(ctx, offerID)
	if err != nil {
		return fmt.Errorf("find existing flights: %w", err)
	}
	if routesEqual(existing, flights) {
		updates := detailsUpdates(existing, flights)
		if len(updates) == 0 {
			return nil
		}
		if err := m.flights.UpdateDetails(ctx, offerID, updates); err != nil {
			return fmt.Errorf("update flight details: %w", err)
		}
		return nil
	}

	if err := m.flights.ReplaceForOffer(ctx, offerID, flights); err != nil {
		return fmt.Errorf("replace flights: %w", err)
	}
	return nil
}

func (m *Manager) checkAirportsExist(ctx context.Context, flights []Flight) error {
	seen := make(map[string]struct{})
	var icaos []string
	for _, f := range flights {
		for _, seg := range f.Segments {
			for _, icao := range [2]string{seg.DepartureAirportICAO, seg.ArrivalAirportICAO} {
				if _, ok := seen[icao]; ok {
					continue
				}
				seen[icao] = struct{}{}
				icaos = append(icaos, icao)
			}
		}
	}
	if len(icaos) == 0 {
		return nil
	}

	found, err := m.airports.FindByICAOs(ctx, icaos)
	if err != nil {
		return fmt.Errorf("find airports by icao: %w", err)
	}
	foundSet := make(map[string]struct{}, len(found))
	for _, a := range found {
		foundSet[a.ICAO] = struct{}{}
	}
	for _, icao := range icaos {
		if _, ok := foundSet[icao]; !ok {
			return ErrAirportNotFound
		}
	}
	return nil
}

// routesEqual сравнивает два набора перелётов только по маршруту: порядок
// перелётов и порядок сегментов внутри перелёта значимы, ID не учитываются
// — у входящих перелётов их нет. Цена и комментарий в сравнение маршрута не
// входят — их сравнивает detailsUpdates.
func routesEqual(a, b []Flight) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !segmentsEqual(a[i].Segments, b[i].Segments) {
			return false
		}
	}
	return true
}

func segmentsEqual(a, b []Segment) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].DepartureAirportICAO != b[i].DepartureAirportICAO ||
			a[i].ArrivalAirportICAO != b[i].ArrivalAirportICAO ||
			!a[i].DepartureAt.Equal(b[i].DepartureAt) ||
			!a[i].ArrivalAt.Equal(b[i].ArrivalAt) {
			return false
		}
	}
	return true
}

// detailsUpdates возвращает новые цену и комментарий для перелётов, у
// которых цена, валюта или комментарий отличается от сохранённых.
// Комментарии сравниваются точно, без нормализации ("a" и "a " различны):
// api хранит текст как есть. existing и incoming — наборы с уже совпавшим
// маршрутом (routesEqual), сопоставленные по позиции; ID берётся у
// сохранённого перелёта.
func detailsUpdates(existing, incoming []Flight) []DetailsUpdate {
	var updates []DetailsUpdate
	for i := range existing {
		if existing[i].Price != incoming[i].Price || existing[i].ManagerComment != incoming[i].ManagerComment {
			updates = append(updates, DetailsUpdate{
				FlightID:       existing[i].ID,
				Price:          incoming[i].Price,
				ManagerComment: incoming[i].ManagerComment,
			})
		}
	}
	return updates
}
