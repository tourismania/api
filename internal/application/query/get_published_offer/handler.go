package getpublishedoffer

import (
	"context"
	"fmt"

	"api/internal/domain/entity"
	"api/internal/domain/service"

	"github.com/google/uuid"
)

// UseCase is the port the presentation layer depends on.
type UseCase interface {
	Handle(ctx context.Context, q Query) (Result, error)
}

// OfferFinder is the read-port consumed by this use-case. The concrete
// implementation lives in the infrastructure layer.
type OfferFinder interface {
	FindByUUID(ctx context.Context, id uuid.UUID) (*entity.Offer, error)
}

// FlightFinder is the read-port for an offer's flights.
type FlightFinder interface {
	FindByOfferID(ctx context.Context, offerID int) ([]entity.Flight, error)
}

// Handler fetches a single offer and only ever returns it if published —
// draft/ready offers are reported as not found, regardless of agency.
type Handler struct {
	offers  OfferFinder
	flights FlightFinder
}

// NewHandler constructs the handler.
func NewHandler(offers OfferFinder, flights FlightFinder) *Handler {
	return &Handler{offers: offers, flights: flights}
}

// Handle satisfies UseCase.
func (h *Handler) Handle(ctx context.Context, q Query) (Result, error) {
	offer, err := h.offers.FindByUUID(ctx, q.UUID)
	if err != nil {
		return Result{}, fmt.Errorf("find offer: %w", err)
	}
	if offer == nil || !offer.IsPublished() {
		return Result{}, service.ErrOfferNotFound
	}

	flights, err := h.flights.FindByOfferID(ctx, offer.ID)
	if err != nil {
		return Result{}, fmt.Errorf("find offer flights: %w", err)
	}

	return Result{
		ID:          offer.ID,
		UUID:        offer.UUID,
		Title:       offer.Title,
		Description: offer.Description,
		AgencyID:    offer.AgencyID,
		CreatedAt:   offer.CreatedAt,
		UpdatedAt:   offer.UpdatedAt,
		Flights:     toFlightResults(flights),
	}, nil
}

// toFlightResults считает суммарную длительность, длительность каждого
// сегмента и пересадки через доменные методы entity.Flight (они нигде
// не хранятся, а вычисляются на лету) и превращает их в плоский
// FlightResult. Это единственное место, где вызывается доменное
// поведение полёта — presentation получает уже готовые числа.
func toFlightResults(flights []entity.Flight) []FlightResult {
	out := make([]FlightResult, 0, len(flights))
	for _, f := range flights {
		segments := make([]FlightSegmentResult, 0, len(f.Segments))
		for _, s := range f.Segments {
			segments = append(segments, FlightSegmentResult{
				DepartureAirportICAO: s.DepartureAirportICAO,
				ArrivalAirportICAO:   s.ArrivalAirportICAO,
				DepartureAt:          s.DepartureAt,
				ArrivalAt:            s.ArrivalAt,
				DurationSeconds:      int64(s.Duration().Seconds()),
			})
		}

		layoverEntities := f.Layovers()
		layovers := make([]LayoverResult, 0, len(layoverEntities))
		for _, l := range layoverEntities {
			layovers = append(layovers, LayoverResult{
				AirportICAO:     l.AirportICAO,
				DurationSeconds: int64(l.Duration.Seconds()),
			})
		}

		out = append(out, FlightResult{
			ID:                   f.ID,
			DepartureAirportICAO: f.DepartureAirportICAO(),
			ArrivalAirportICAO:   f.ArrivalAirportICAO(),
			TotalDurationSeconds: int64(f.TotalDuration().Seconds()),
			Segments:             segments,
			Layovers:             layovers,
		})
	}
	return out
}
