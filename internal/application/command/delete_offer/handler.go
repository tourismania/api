package deleteoffer

import (
	"context"

	"api/internal/domain/service"
)

// UseCase is the port the presentation layer depends on.
type UseCase interface {
	Handle(ctx context.Context, cmd Command) (Result, error)
}

// Handler executes the DeleteOffer command by delegating to the domain
// OfferManager service, which enforces agency ownership.
type Handler struct {
	offerManager *service.OfferManager
}

// NewHandler constructs the handler.
func NewHandler(offerManager *service.OfferManager) *Handler {
	return &Handler{offerManager: offerManager}
}

// Handle satisfies UseCase.
func (h *Handler) Handle(ctx context.Context, cmd Command) (Result, error) {
	if err := h.offerManager.Delete(ctx, cmd.UUID, service.Actor{
		UserID:   cmd.CurrentUserID,
		AgencyID: cmd.AgencyID,
	}); err != nil {
		return Result{}, err
	}
	return Result{}, nil
}
