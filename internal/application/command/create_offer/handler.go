package createoffer

import (
	"context"

	"api/internal/application/apperror"
	"api/internal/domain/service"
)

// UseCase is the port the presentation layer depends on.
type UseCase interface {
	Handle(ctx context.Context, cmd Command) (Result, error)
}

// Handler executes the CreateOffer command by delegating to the domain
// OfferManager service. Keeping the handler thin preserves DDD: business
// invariants and ownership rules stay in the domain layer. Resolving
// the acting principal (agency_id, roles) from its uuid is delegated to
// the domain service.UserFinder, not to presentation-layer middleware,
// so it always reflects the latest DB state. Every domain error is
// translated to apperror before it leaves this handler — presentation
// never sees a domain/service sentinel directly.
type Handler struct {
	offerManager *service.OfferManager
	userFinder   *service.UserFinder
}

// NewHandler constructs the handler.
func NewHandler(offerManager *service.OfferManager, userFinder *service.UserFinder) *Handler {
	return &Handler{offerManager: offerManager, userFinder: userFinder}
}

// Handle satisfies UseCase.
func (h *Handler) Handle(ctx context.Context, cmd Command) (Result, error) {
	actor, err := h.userFinder.Resolve(ctx, cmd.CurrentUserUUID)
	if err != nil {
		return Result{}, apperror.FromDomainError(err)
	}

	offer, err := h.offerManager.Insert(ctx, cmd.Title, cmd.Description, cmd.Status, actor)
	if err != nil {
		return Result{}, apperror.FromDomainError(err)
	}
	return Result{ID: offer.ID, UUID: offer.UUID}, nil
}
