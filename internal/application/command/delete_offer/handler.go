package deleteoffer

import (
	"context"

	"api/internal/application/apperror"
	"api/internal/domain/service"
)

// UseCase is the port the presentation layer depends on.
type UseCase interface {
	Handle(ctx context.Context, cmd Command) (Result, error)
}

// Handler executes the DeleteOffer command by delegating to the domain
// OfferManager service, which enforces agency ownership and the write
// role. The acting principal is resolved from its uuid via
// service.UserFinder, not presentation-layer middleware. Every domain
// error is translated to apperror before it leaves this handler.
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

	if err := h.offerManager.Delete(ctx, cmd.UUID, actor); err != nil {
		return Result{}, apperror.FromDomainError(err)
	}
	return Result{}, nil
}
