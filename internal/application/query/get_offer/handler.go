package getoffer

import (
	"context"
	"fmt"

	"api/internal/application/apperror"
	"api/internal/domain/entity"
	"api/internal/domain/service"
)

// UseCase is the port the presentation layer depends on.
type UseCase interface {
	Handle(ctx context.Context, q Query) (Result, error)
}

// FlightFinder is the read-port for an offer's flights. Defined here to
// invert the dependency: infrastructure implements this.
type FlightFinder interface {
	FindByOfferID(ctx context.Context, offerID int) ([]entity.Flight, error)
}

// Handler fetches a single offer for its own agency's staff/users: the
// caller sees it regardless of status as long as it belongs to their
// agency; any other agency's offer is reported as not found. Published
// offers of other agencies are served separately by get_published_offer.
// The caller's own agency is resolved from its uuid via
// service.UserFinder, not presentation-layer middleware. Ownership is
// checked by the domain OfferManager.FindOwned — the same method the
// write use-cases use — so the comparison exists in exactly one place.
type Handler struct {
	offerManager *service.OfferManager
	flights      FlightFinder
	userFinder   *service.UserFinder
}

// NewHandler constructs the handler.
func NewHandler(offerManager *service.OfferManager, flights FlightFinder, userFinder *service.UserFinder) *Handler {
	return &Handler{offerManager: offerManager, flights: flights, userFinder: userFinder}
}

// Handle satisfies UseCase.
func (h *Handler) Handle(ctx context.Context, q Query) (Result, error) {
	actor, err := h.userFinder.Resolve(ctx, q.CurrentUserUUID)
	if err != nil {
		return Result{}, apperror.FromDomainError(err)
	}

	offer, err := h.offerManager.FindOwned(ctx, q.UUID, actor)
	if err != nil {
		return Result{}, apperror.FromDomainError(err)
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
		CreatedBy:   offer.CreatedBy,
		Status:      offer.Status,
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
