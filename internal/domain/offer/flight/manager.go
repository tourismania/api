package flight

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"api/internal/domain/airline"
	"api/internal/domain/airport"
)

// ErrAirportNotFound is returned by Manager when a segment references
// an icao that does not exist in the airports reference table. Unlike
// the structural errors checked by New, this requires a database
// lookup, so it is raised here rather than by the factory.
var ErrAirportNotFound = errors.New("flight segment references an unknown airport icao")

// ErrAirlineNotFound возвращается Manager, если сегмент ссылается на id,
// которого нет в справочнике авиакомпаний (в т. ч. отрицательный), или на
// скрытую из поиска запись (дубль, пропавшая из источника), на которую оффер
// до этой записи не ссылался.
var ErrAirlineNotFound = errors.New("flight segment references an unknown airline")

// Manager orchestrates replacing the full set of flights attached to
// an Offer. It does not check role/agency ownership: it is only ever
// called from create_offer/update_offer handlers after
// offer.Manager.Insert/Update has already enforced that, within the
// same command and (for the write path) the same transaction.
type Manager struct {
	flights  Repository
	airports airport.Repository
	airlines airline.Repository
}

// NewManager wires the collaborators.
func NewManager(flights Repository, airports airport.Repository, airlines airline.Repository) *Manager {
	return &Manager{flights: flights, airports: airports, airlines: airlines}
}

// ReplaceForOffer проверяет, что все коды icao из flights есть в
// справочнике аэропортов, а авиакомпании сегментов — в справочнике
// авиакомпаний (скрытая запись допустима, только если на неё уже ссылается
// сохранённый сегмент оффера), и сравнивает flights с сохранёнными для
// offerID — сначала маршрут, потом цены, комментарии, классы и авиакомпании
// сегментов:
//   - маршрут совпал (тот же порядок перелётов, те же аэропорты и
//     моменты времени сегментов; ID, назначенные сервером, не учитываются),
//     цены, комментарии, классы и авиакомпании совпали — no-op, БД не трогается;
//   - маршрут совпал, у части перелётов другая цена, валюта, комментарий,
//     класс или авиакомпания хотя бы одного сегмента — у этих перелётов
//     обновляются только цена, комментарий, классы и авиакомпании сегментов;
//     аэропорты и время сегментов и идентификаторы сохраняются;
//   - маршрут отличается — набор заменяется целиком.
//
// Перелёты сопоставляются по позиции: у входящих перелётов нет ID.
func (m *Manager) ReplaceForOffer(ctx context.Context, offerID int, flights []Flight) error {
	if err := m.checkAirportsExist(ctx, flights); err != nil {
		return err
	}

	// Сохранённые перелёты нужны и для сравнения, и для проверки скрытых
	// авиакомпаний, поэтому загружаются до неё.
	existing, err := m.flights.FindByOfferID(ctx, offerID)
	if err != nil {
		return fmt.Errorf("find existing flights: %w", err)
	}
	if err := m.checkAirlinesExist(ctx, flights, existing); err != nil {
		return err
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

// checkAirlinesExist одним запросом проверяет авиакомпании сегментов flights:
// id должен быть в справочнике, а невыбираемая запись (дубль или пропавшая из
// источника, см. airline.Airline.Selectable) допустима, только если её id уже
// есть у сохранённых сегментов оффера existing — тогда сохранение старого
// оффера не ломается из-за правки в источнике, а новых ссылок на такие записи
// не появляется. Сравнение — по всему офферу, а не по позиции: набор
// передаётся целиком, и сегменты при правке могут сдвигаться.
func (m *Manager) checkAirlinesExist(ctx context.Context, flights, existing []Flight) error {
	ids := AirlineIDs(flights)
	if len(ids) == 0 {
		return nil
	}

	found, err := m.airlines.FindByIDs(ctx, ids)
	if err != nil {
		return fmt.Errorf("find airlines by id: %w", err)
	}
	byID := make(map[int]airline.Airline, len(found))
	for _, a := range found {
		byID[a.ID] = a
	}
	stored := AirlineIDs(existing)
	for _, id := range ids {
		a, ok := byID[id]
		if !ok {
			return ErrAirlineNotFound
		}
		if _, referenced := slices.BinarySearch(stored, id); !a.Selectable() && !referenced {
			return ErrAirlineNotFound
		}
	}
	return nil
}

// AirlineIDs возвращает id авиакомпаний всех сегментов flights без повторов,
// по возрастанию.
func AirlineIDs(flights []Flight) []int {
	var ids []int
	for _, f := range flights {
		for _, seg := range f.Segments {
			ids = append(ids, seg.AirlineID)
		}
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}

// routesEqual сравнивает два набора перелётов только по маршруту: порядок
// перелётов и порядок сегментов внутри перелёта значимы, ID не учитываются
// — у входящих перелётов их нет. Цена, комментарий, классы и авиакомпании
// сегментов в сравнение маршрута не входят — их сравнивает detailsUpdates.
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

// detailsUpdates возвращает новые цену, комментарий, классы и авиакомпании
// сегментов для перелётов, у которых цена, валюта, комментарий, класс или
// авиакомпания хотя бы одного сегмента отличается от сохранённых; в
// обновление идут классы и авиакомпании всех сегментов перелёта. Комментарии и классы сравниваются точно, без
// нормализации ("a" и "a " различны): api хранит значения как есть. existing и incoming — наборы с уже совпавшим
// маршрутом (routesEqual), сопоставленные по позиции; ID берётся у
// сохранённого перелёта.
func detailsUpdates(existing, incoming []Flight) []DetailsUpdate {
	var updates []DetailsUpdate
	for i := range existing {
		if existing[i].Price != incoming[i].Price ||
			existing[i].ManagerComment != incoming[i].ManagerComment ||
			!segmentDetailsEqual(existing[i].Segments, incoming[i].Segments) {
			updates = append(updates, DetailsUpdate{
				FlightID:            existing[i].ID,
				Price:               incoming[i].Price,
				ManagerComment:      incoming[i].ManagerComment,
				SegmentCabinClasses: segmentCabinClasses(incoming[i].Segments),
				SegmentAirlineIDs:   segmentAirlineIDs(incoming[i].Segments),
			})
		}
	}
	return updates
}

// segmentDetailsEqual сравнивает классы и авиакомпании сегментов двух
// перелётов по позиции. Вызывается только для перелётов с совпавшим
// маршрутом, поэтому число сегментов одинаково.
func segmentDetailsEqual(a, b []Segment) bool {
	for i := range a {
		if a[i].CabinClass != b[i].CabinClass || a[i].AirlineID != b[i].AirlineID {
			return false
		}
	}
	return true
}

// segmentCabinClasses возвращает классы сегментов по порядку.
func segmentCabinClasses(segs []Segment) []CabinClass {
	classes := make([]CabinClass, 0, len(segs))
	for _, s := range segs {
		classes = append(classes, s.CabinClass)
	}
	return classes
}

// segmentAirlineIDs возвращает id авиакомпаний сегментов по порядку.
func segmentAirlineIDs(segs []Segment) []int {
	ids := make([]int, 0, len(segs))
	for _, s := range segs {
		ids = append(ids, s.AirlineID)
	}
	return ids
}
