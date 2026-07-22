package updateoffer

import (
	"context"

	"api/internal/application/identity"
	"api/internal/domain/service"
)

// UseCase is the port the presentation layer depends on.
type UseCase interface {
	Handle(ctx context.Context, cmd Command) (Result, error)
}

// Handler executes the UpdateOffer command by delegating to the domain
// OfferManager service, which enforces agency ownership and the write
// role. The acting principal is resolved from its uuid via
// application/identity, not presentation-layer middleware.
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

	offer, err := h.offerManager.Update(ctx, cmd.UUID, cmd.Title, cmd.Description, cmd.Status, actor)
	if err != nil {
		return Result{}, err
	}
	return Result{ID: offer.ID, UUID: offer.UUID}, nil
}
