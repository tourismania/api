package getpublishedoffer

import (
	"api/internal/application/helpers"
	"api/internal/domain/airline"
	"api/internal/domain/airport"
	"api/internal/domain/offer"
	"api/internal/domain/offer/flight"
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/google/uuid"
)

// UseCase is the port the presentation layer depends on.
type UseCase interface {
	Handle(ctx context.Context, q Query) (Result, error)
}

// OfferFinder is the read-port consumed by this use-case. The concrete
// implementation lives in the infrastructure layer.
type OfferFinder interface {
	FindByUUID(ctx context.Context, id uuid.UUID) (*offer.Offer, error)
}

// FlightFinder is the read-port for an offer's flights.
type FlightFinder interface {
	FindByOfferID(ctx context.Context, offerID int) ([]flight.Flight, error)
}

// AirlineFinder — порт чтения авиакомпаний сегментов по id.
type AirlineFinder = helpers.AirlineFinder

// AirportFinder — порт чтения аэропортов сегментов по кодам ICAO.
type AirportFinder = helpers.AirportFinder

// Handler fetches a single offer and only ever returns it if published —
// draft/ready offers are reported as not found, regardless of agency.
type Handler struct {
	offers   OfferFinder
	flights  FlightFinder
	airlines AirlineFinder
	airports AirportFinder
}

// NewHandler constructs the handler.
func NewHandler(offers OfferFinder, flights FlightFinder, airlines AirlineFinder, airports AirportFinder) *Handler {
	return &Handler{offers: offers, flights: flights, airlines: airlines, airports: airports}
}

// Handle satisfies UseCase.
func (h *Handler) Handle(ctx context.Context, q Query) (Result, error) {
	found, err := h.offers.FindByUUID(ctx, q.UUID)
	if err != nil {
		return Result{}, fmt.Errorf("find offer: %w", err)
	}
	if found == nil || !found.IsPublished() {
		return Result{}, offer.ErrNotFound
	}

	flights, err := h.flights.FindByOfferID(ctx, found.ID)
	if err != nil {
		return Result{}, fmt.Errorf("find offer flights: %w", err)
	}
	airlines, err := helpers.FindSegmentAirlines(ctx, h.airlines, flights)
	if err != nil {
		return Result{}, err
	}
	airports, err := helpers.FindSegmentAirports(ctx, h.airports, flights)
	if err != nil {
		return Result{}, err
	}

	return Result{
		ID:          found.ID,
		UUID:        found.UUID,
		Title:       found.Title,
		Description: found.Description,
		AgencyID:    found.AgencyID,
		CreatedAt:   found.CreatedAt,
		UpdatedAt:   found.UpdatedAt,
		Flights:     toFlightResults(flights, airlines, airports),
	}, nil
}

// toFlightResults считает суммарную длительность, длительность каждого
// сегмента и пересадки через доменные методы flight.Flight (они нигде
// не хранятся, а вычисляются на лету) и превращает их в плоский
// FlightResult. Это единственное место, где вызывается доменное
// поведение полёта — presentation получает уже готовые числа. Данные
// авиакомпаний сегментов берутся из airlines (уже загружены по всем id), данные
// аэропортов и их таймзоны для длительностей — из airports (уже загружены по
// всем ICAO).
func toFlightResults(flights []flight.Flight, airlines map[int]airline.Airline, airports map[string]airport.Airport) []FlightResult {
	zones := airport.NewZones(slices.Collect(maps.Values(airports)))
	out := make([]FlightResult, 0, len(flights))
	for _, f := range flights {
		segments := make([]FlightSegmentResult, 0, len(f.Segments))
		for _, s := range f.Segments {
			segments = append(segments, FlightSegmentResult{
				DepartureAirportICAO: s.DepartureAirportICAO,
				ArrivalAirportICAO:   s.ArrivalAirportICAO,
				DepartureAirport:     helpers.ToAirportResult(airports[s.DepartureAirportICAO]),
				ArrivalAirport:       helpers.ToAirportResult(airports[s.ArrivalAirportICAO]),
				DepartureAt:          s.DepartureAt,
				ArrivalAt:            s.ArrivalAt,
				DurationSeconds:      int64(s.Duration(zones).Seconds()),
				CabinClass:           string(s.CabinClass),
				FlightNumber:         s.FlightNumber,
				AirlineID:            s.AirlineID,
				Airline:              toAirlineResult(airlines[s.AirlineID]),
			})
		}

		layoverEntities := f.Layovers(zones)
		layovers := make([]LayoverResult, 0, len(layoverEntities))
		for _, l := range layoverEntities {
			layovers = append(layovers, LayoverResult{
				AirportICAO:     l.AirportICAO,
				DurationSeconds: int64(l.Duration.Seconds()),
			})
		}

		out = append(out, FlightResult{
			ID:                   f.ID,
			Price:                f.Price.String(),
			Currency:             f.Price.Currency.Code,
			ManagerComment:       f.ManagerComment,
			DepartureAirportICAO: f.DepartureAirportICAO(),
			ArrivalAirportICAO:   f.ArrivalAirportICAO(),
			TotalDurationSeconds: int64(f.TotalDuration(zones).Seconds()),
			Segments:             segments,
			Layovers:             layovers,
		})
	}
	return out
}

// toAirlineResult — проекция авиакомпании сегмента.
func toAirlineResult(a airline.Airline) AirlineResult {
	return AirlineResult{
		ID:          a.ID,
		ICAO:        a.ICAO,
		Name:        a.Name,
		NameRU:      a.NameRU,
		DissolvedOn: a.DissolvedOn,
	}
}
