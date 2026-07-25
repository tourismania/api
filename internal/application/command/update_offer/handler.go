package updateoffer

import (
	"context"

	"api/internal/application/apperror"
	"api/internal/application/txmanager"
	"api/internal/domain/entity"
	"api/internal/domain/factory"
	"api/internal/domain/service"
)

// UseCase is the port the presentation layer depends on.
type UseCase interface {
	Handle(ctx context.Context, cmd Command) (Result, error)
}

// Handler executes the UpdateOffer command by delegating to the domain
// OfferManager service, which enforces agency ownership and the write
// role. The acting principal is resolved from its uuid via
// service.UserFinder, not presentation-layer middleware. Every domain
// error is translated to apperror before it leaves this handler.
// Updating the offer and (if the flights key was present) replacing its
// flights is wrapped in a single txManager.WithinTx.
type Handler struct {
	offerManager       *service.OfferManager
	offerFlightManager *service.OfferFlightManager
	userFinder         *service.UserFinder
	txManager          txmanager.TxManager
}

// NewHandler constructs the handler.
func NewHandler(offerManager *service.OfferManager, offerFlightManager *service.OfferFlightManager, userFinder *service.UserFinder, txManager txmanager.TxManager) *Handler {
	return &Handler{
		offerManager:       offerManager,
		offerFlightManager: offerFlightManager,
		userFinder:         userFinder,
		txManager:          txManager,
	}
}

// Handle satisfies UseCase.
func (h *Handler) Handle(ctx context.Context, cmd Command) (Result, error) {
	actor, err := h.userFinder.Resolve(ctx, cmd.CurrentUserUUID)
	if err != nil {
		return Result{}, apperror.FromDomainError(err)
	}

	var flights []entity.Flight
	if cmd.Flights != nil {
		flights, err = buildFlights(*cmd.Flights)
		if err != nil {
			return Result{}, apperror.FromDomainError(err)
		}
	}

	var result Result
	err = h.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		offer, err := h.offerManager.Update(txCtx, cmd.UUID, cmd.Title, cmd.Description, cmd.Status, actor)
		if err != nil {
			return err
		}
		if cmd.Flights != nil {
			if err := h.offerFlightManager.ReplaceForOffer(txCtx, offer.ID, flights); err != nil {
				return err
			}
		}
		result = Result{ID: offer.ID, UUID: offer.UUID}
		return nil
	})
	if err != nil {
		return Result{}, apperror.FromDomainError(err)
	}
	return result, nil
}

// buildFlights validates each segment group's structural invariants via
// factory.NewFlight before any database write happens.
func buildFlights(groups [][]entity.FlightSegment) ([]entity.Flight, error) {
	if len(groups) == 0 {
		return nil, nil
	}
	flights := make([]entity.Flight, 0, len(groups))
	for _, segs := range groups {
		f, err := factory.NewFlight(segs)
		if err != nil {
			return nil, err
		}
		flights = append(flights, f)
	}
	return flights, nil
}
