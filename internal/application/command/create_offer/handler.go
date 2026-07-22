package createoffer

import (
	"context"

	"api/internal/application/identity"
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
// application/identity, not to presentation-layer middleware, so it
// always reflects the latest DB state.
type Handler struct {
	offerManager *service.OfferManager
	users        identity.UserFinder
}

// NewHandler constructs the handler.
func NewHandler(offerManager *service.OfferManager, users identity.UserFinder) *Handler {
	return &Handler{offerManager: offerManager, users: users}
}

// Handle satisfies UseCase.
func (h *Handler) Handle(ctx context.Context, cmd Command) (Result, error) {
	actor, err := identity.Resolve(ctx, h.users, cmd.CurrentUserUUID)
	if err != nil {
		return Result{}, err
	}

	offer, err := h.offerManager.Insert(ctx, cmd.Title, cmd.Description, cmd.Status, actor)
	if err != nil {
		return Result{}, err
	}
	return Result{ID: offer.ID, UUID: offer.UUID}, nil
}
